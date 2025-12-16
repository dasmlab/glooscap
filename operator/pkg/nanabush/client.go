package nanabush

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/types/known/timestamppb"

	nanabushv1 "github.com/dasmlab/glooscap-operator/pkg/nanabush/proto/v1"
)

// Client is a gRPC client for communicating with the Nanabush translation service.
type Client struct {
	conn   *grpc.ClientConn
	addr   string
	secure bool
	client nanabushv1.TranslationServiceClient

	// Registration
	clientID      string
	clientName    string
	clientVersion string
	namespace     string
	metadata      map[string]string

	// Heartbeat
	heartbeatInterval time.Duration
	heartbeatStop     chan struct{}
	heartbeatWg       sync.WaitGroup
	lastHeartbeatTime time.Time
	missedHeartbeats  int

	// Connection state
	mu         sync.RWMutex
	registered bool

	// Status change callback (called when status changes)
	onStatusChange func(Status)
}

// Config contains configuration for the Nanabush client.
type Config struct {
	// Address is the gRPC server address (e.g., "nanabush-service.nanabush.svc:50051")
	Address string
	// Secure enables TLS/mTLS (default: false for now)
	Secure bool
	// TLSCertPath is the path to the client TLS certificate (for mTLS)
	TLSCertPath string
	// TLSKeyPath is the path to the client TLS private key
	TLSKeyPath string
	// TLSCAPath is the path to the CA certificate for server verification
	TLSCAPath string
	// Timeout is the connection timeout
	Timeout time.Duration

	// Client registration
	ClientName    string            // Name of the client (e.g., "glooscap")
	ClientVersion string            // Version of the client
	Namespace     string            // Kubernetes namespace
	Metadata      map[string]string // Additional metadata

	// OnStatusChange is called when the client status changes (connect, disconnect, heartbeat, etc.)
	OnStatusChange func(Status)
}

// NewClient creates a new Nanabush gRPC client and automatically registers with the server.
func NewClient(cfg Config) (*Client, error) {
	if cfg.Address == "" {
		return nil, fmt.Errorf("nanabush: address is required")
	}

	if cfg.ClientName == "" {
		cfg.ClientName = "glooscap" // Default client name
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	var opts []grpc.DialOption

	// Configure TLS/mTLS
	if cfg.Secure {
		// TODO: Load TLS credentials from cfg.TLSCertPath, cfg.TLSKeyPath, cfg.TLSCAPath
		// For now, use insecure for development
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Configure keepalive for connection health monitoring
	opts = append(opts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
		Time:                30 * time.Second,
		Timeout:             10 * time.Second,
		PermitWithoutStream: true,
	}))

	opts = append(opts, grpc.WithTimeout(timeout))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Log connection attempt
	fmt.Printf("[nanabush] Attempting gRPC connection to %s (secure=%v, timeout=%v)\n",
		cfg.Address, cfg.Secure, timeout)

	conn, err := grpc.DialContext(ctx, cfg.Address, opts...)
	if err != nil {
		fmt.Printf("[nanabush] Failed to dial %s: %v\n", cfg.Address, err)
		return nil, fmt.Errorf("nanabush: dial %s: %w", cfg.Address, err)
	}

	// Log connection state
	state := conn.GetState()
	fmt.Printf("[nanabush] gRPC connection established to %s (state: %s)\n", cfg.Address, state.String())

	// Wait for connection to be ready before proceeding
	// This ensures the connection is fully established before we try to register
	if state != connectivity.Ready {
		fmt.Printf("[nanabush] Connection not ready (state: %s), waiting for Ready state...\n", state.String())
		ctxReady, cancelReady := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelReady()

		// Wait for state to change from current state
		for {
			if !conn.WaitForStateChange(ctxReady, state) {
				// Timeout or context cancelled
				newState := conn.GetState()
				fmt.Printf("[nanabush] Connection state wait timeout/cancelled, current state: %s\n", newState.String())
				if newState == connectivity.Ready {
					break
				}
				// If not ready, we'll try anyway but log a warning
				fmt.Printf("[nanabush] Warning: Proceeding with registration despite connection not being Ready (state: %s)\n", newState.String())
				break
			}
			newState := conn.GetState()
			fmt.Printf("[nanabush] Connection state changed: %s -> %s\n", state.String(), newState.String())
			if newState == connectivity.Ready {
				fmt.Printf("[nanabush] Connection is now Ready!\n")
				break
			}
			if newState == connectivity.TransientFailure || newState == connectivity.Shutdown {
				fmt.Printf("[nanabush] Connection failed or shutdown (state: %s), registration will likely fail\n", newState.String())
				break
			}
			// Update state for next iteration
			state = newState
		}
	}

	// Initialize generated client stub
	client := nanabushv1.NewTranslationServiceClient(conn)

	c := &Client{
		conn:              conn,
		addr:              cfg.Address,
		secure:            cfg.Secure,
		client:            client,
		clientName:        cfg.ClientName,
		clientVersion:     cfg.ClientVersion,
		namespace:         cfg.Namespace,
		metadata:          cfg.Metadata,
		heartbeatInterval: 5 * time.Second, // Default: 5 seconds
		heartbeatStop:     make(chan struct{}),
		onStatusChange:    cfg.OnStatusChange,
	}

	// Register with server
	fmt.Printf("[nanabush] Registering client: name=%q, version=%q, namespace=%q\n",
		cfg.ClientName, cfg.ClientVersion, cfg.Namespace)
	if err := c.register(ctx); err != nil {
		conn.Close()
		fmt.Printf("[nanabush] Registration failed: %v\n", err)
		return nil, fmt.Errorf("nanabush: register: %w", err)
	}

	fmt.Printf("[nanabush] Client registered successfully: client_id=%q, heartbeat_interval=%v\n",
		c.clientID, c.heartbeatInterval)

	// Start heartbeat goroutine (interval may have been updated during registration)
	c.startHeartbeat()
	fmt.Printf("[nanabush] Heartbeat goroutine started (interval: %v)\n", c.heartbeatInterval)
	
	// Start watchdog goroutine to monitor for missed heartbeats
	c.startHeartbeatWatchdog()
	fmt.Printf("[nanabush] Heartbeat watchdog started\n")

	// Notify initial status after successful registration
	if c.onStatusChange != nil {
		c.onStatusChange(c.Status())
	}

	return c, nil
}

// register registers the client with the server.
func (c *Client) register(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	fmt.Printf("[nanabush] Calling RegisterClient RPC: name=%q, version=%q, namespace=%q\n",
		c.clientName, c.clientVersion, c.namespace)

	req := &nanabushv1.RegisterClientRequest{
		ClientName:    c.clientName,
		ClientVersion: c.clientVersion,
		Namespace:     c.namespace,
		Metadata:      c.metadata,
		RegisteredAt:  timestamppb.Now(),
	}

	// Check connection state before making RPC call
	connState := c.conn.GetState()
	fmt.Printf("[nanabush] Connection state before RegisterClient: %s\n", connState.String())

	if connState != connectivity.Ready && connState != connectivity.Idle {
		fmt.Printf("[nanabush] Warning: Connection not in Ready/Idle state (state: %s), RPC may fail\n", connState.String())
	}

	fmt.Printf("[nanabush] RegisterClient request sent, waiting for response...\n")
	startTime := time.Now()
	resp, err := c.client.RegisterClient(ctx, req)
	duration := time.Since(startTime)

	if err != nil {
		fmt.Printf("[nanabush] RegisterClient RPC failed after %v: %v\n", duration, err)
		fmt.Printf("[nanabush] Connection state after error: %s\n", c.conn.GetState().String())
		return fmt.Errorf("register client: %w", err)
	}

	fmt.Printf("[nanabush] RegisterClient response received after %v: success=%v, client_id=%q, message=%q, heartbeat_interval=%ds\n",
		duration, resp.Success, resp.ClientId, resp.Message, resp.HeartbeatIntervalSeconds)
	fmt.Printf("[nanabush] Connection state after successful response: %s\n", c.conn.GetState().String())

	if !resp.Success {
		fmt.Printf("[nanabush] Registration failed: %s\n", resp.Message)
		return fmt.Errorf("registration failed: %s", resp.Message)
	}

	c.clientID = resp.ClientId
	c.registered = true

	// Update heartbeat interval from server response
	if resp.HeartbeatIntervalSeconds > 0 {
		oldInterval := c.heartbeatInterval
		c.heartbeatInterval = time.Duration(resp.HeartbeatIntervalSeconds) * time.Second
		fmt.Printf("[nanabush] Heartbeat interval updated: %v -> %v\n", oldInterval, c.heartbeatInterval)
	}

	fmt.Printf("[nanabush] Client registration complete: client_id=%q, registered=%v\n", c.clientID, c.registered)

	// Notify status change
	if c.onStatusChange != nil {
		c.onStatusChange(c.Status())
	}

	return nil
}

// startHeartbeat starts the heartbeat goroutine.
func (c *Client) startHeartbeat() {
	c.heartbeatWg.Add(1)
	go func() {
		defer c.heartbeatWg.Done()

		// Get initial interval (should already be set from registration)
		c.mu.RLock()
		initialInterval := c.heartbeatInterval
		c.mu.RUnlock()
		
		fmt.Printf("[nanabush] Starting heartbeat goroutine with interval: %v\n", initialInterval)
		
		// Use a dynamic ticker that can be updated if interval changes
		ticker := time.NewTicker(initialInterval)
		defer ticker.Stop()

		// Track last tick time and current ticker interval for debugging
		lastTickTime := time.Now()
		tickCount := 0
		currentTickerInterval := initialInterval

		for {
			select {
			case <-ticker.C:
				tickCount++
				now := time.Now()
				timeSinceLastTick := now.Sub(lastTickTime)
				fmt.Printf("[nanabush] Heartbeat ticker fired (#%d): interval=%v, time_since_last_tick=%v\n",
					tickCount, currentTickerInterval, timeSinceLastTick.Round(time.Millisecond))
				lastTickTime = now
				
				c.sendHeartbeat()
				
				// Check if interval changed and recreate ticker if needed
				c.mu.RLock()
				desiredInterval := c.heartbeatInterval
				c.mu.RUnlock()
				if currentTickerInterval != desiredInterval {
					fmt.Printf("[nanabush] Heartbeat interval changed, recreating ticker: %v -> %v\n", currentTickerInterval, desiredInterval)
					ticker.Stop()
					ticker = time.NewTicker(desiredInterval)
					currentTickerInterval = desiredInterval
					lastTickTime = time.Now() // Reset tick time
				}
			case <-c.heartbeatStop:
				fmt.Printf("[nanabush] Heartbeat goroutine stopping (sent %d heartbeats)\n", tickCount)
				return
			}
		}
	}()
}

// startHeartbeatWatchdog starts a goroutine that monitors for missed heartbeats
func (c *Client) startHeartbeatWatchdog() {
	c.heartbeatWg.Add(1)
	go func() {
		defer c.heartbeatWg.Done()

		checkInterval := 5 * time.Second // Check every 5 seconds
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.mu.RLock()
				lastHeartbeat := c.lastHeartbeatTime
				interval := c.heartbeatInterval
				registered := c.registered
				clientID := c.clientID
				c.mu.RUnlock()

				if !registered || clientID == "" {
					continue // Not registered yet, skip check
				}

				now := time.Now()
				timeSinceLastHeartbeat := now.Sub(lastHeartbeat)
				threshold := interval * 2 // Alert if no heartbeat in 2x the interval

				if !lastHeartbeat.IsZero() && timeSinceLastHeartbeat > threshold {
					fmt.Printf("[nanabush] ⚠️  WARNING: No heartbeat received in %v (threshold: %v, last: %v)\n",
						timeSinceLastHeartbeat, threshold, lastHeartbeat.Format(time.RFC3339))
					// Increment missed heartbeats
					c.mu.Lock()
					c.missedHeartbeats++
					c.mu.Unlock()
					// Notify status change
					if c.onStatusChange != nil {
						c.onStatusChange(c.Status())
					}
				} else if !lastHeartbeat.IsZero() {
					fmt.Printf("[nanabush] ✓ Heartbeat OK: last received %v ago (threshold: %v)\n",
						timeSinceLastHeartbeat.Round(time.Second), threshold.Round(time.Second))
				}
			case <-c.heartbeatStop:
				return
			}
		}
	}()
}

// sendHeartbeat sends a heartbeat to the server.
func (c *Client) sendHeartbeat() {
	c.mu.RLock()
	clientID := c.clientID
	clientName := c.clientName
	registered := c.registered
	c.mu.RUnlock()

	if !registered || clientID == "" {
		fmt.Printf("[nanabush] Skipping heartbeat: registered=%v, client_id=%q\n", registered, clientID)
		return
	}

	fmt.Printf("[nanabush] 📤 Sending heartbeat: client_id=%q, client_name=%q, time=%v\n",
		clientID, clientName, time.Now().Format(time.RFC3339))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := c.client.Heartbeat(ctx, &nanabushv1.HeartbeatRequest{
		ClientId:   clientID,
		ClientName: clientName,
		SentAt:     timestamppb.Now(),
		Metadata:   c.metadata,
	})
	if err != nil {
		// Connection error - need to re-register
		fmt.Printf("[nanabush] Heartbeat failed: client_id=%q, error=%v\n", clientID, err)
		c.mu.Lock()
		c.registered = false
		c.missedHeartbeats++ // Increment missed heartbeats on error
		fmt.Printf("[nanabush] Missed heartbeats: %d\n", c.missedHeartbeats)
		c.mu.Unlock()

		// Notify status change on error
		if c.onStatusChange != nil {
			c.onStatusChange(c.Status())
		}

		// Try to reconnect and re-register
		fmt.Printf("[nanabush] Attempting to reconnect and re-register...\n")
		go c.reconnectAndRegister()
		return
	}

	fmt.Printf("[nanabush] Heartbeat acknowledged: client_id=%q, success=%v, message=%q\n",
		clientID, resp.Success, resp.Message)

	if resp.ReRegisterRequired {
		// Server requested re-registration
		c.mu.Lock()
		c.registered = false
		c.mu.Unlock()

		// Re-register
		if err := c.register(context.Background()); err != nil {
			// If registration fails, try to reconnect
			go c.reconnectAndRegister()
		}
		return
	}

	// Update heartbeat interval if server changed it
	if resp.HeartbeatIntervalSeconds > 0 {
		newInterval := time.Duration(resp.HeartbeatIntervalSeconds) * time.Second
		c.mu.Lock()
		if newInterval != c.heartbeatInterval {
			c.heartbeatInterval = newInterval
		}
		c.mu.Unlock()
	}

	// Mark heartbeat as successful
	c.mu.Lock()
	previousMissed := c.missedHeartbeats
	previousLastHeartbeat := c.lastHeartbeatTime
	c.lastHeartbeatTime = time.Now()
	c.missedHeartbeats = 0 // Reset missed heartbeats on success
	c.mu.Unlock()

	// Log heartbeat received
	if previousLastHeartbeat.IsZero() {
		fmt.Printf("[nanabush] ✓ First heartbeat received: client_id=%q, acknowledged at %v\n",
			clientID, time.Now().Format(time.RFC3339))
	} else {
		timeSinceLast := time.Since(previousLastHeartbeat)
		fmt.Printf("[nanabush] ✓ Heartbeat received: client_id=%q, time_since_last=%v, acknowledged at %v\n",
			clientID, timeSinceLast.Round(time.Millisecond), time.Now().Format(time.RFC3339))
	}

	if previousMissed > 0 {
		fmt.Printf("[nanabush] Heartbeat recovered: client_id=%q, missed_heartbeats_reset=0\n", clientID)
	}

	// Notify status change on successful heartbeat
	if c.onStatusChange != nil {
		c.onStatusChange(c.Status())
	}
}

// reconnectAndRegister attempts to reconnect and re-register with the server.
func (c *Client) reconnectAndRegister() {
	// Prevent multiple concurrent reconnection attempts using a sync flag
	// We'll use the registered flag to track connection state
	c.mu.Lock()
	isRegistered := c.registered
	c.mu.Unlock()

	// Only attempt reconnection if we were previously registered
	// (to avoid reconnection storms)
	if !isRegistered {
		// Not registered, skip reconnection attempt
		return
	}

	// Retry logic with exponential backoff
	maxRetries := 5
	backoff := 1 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Close old connection
		c.mu.Lock()
		oldConn := c.conn
		addr := c.addr
		secure := c.secure
		c.mu.Unlock()

		if oldConn != nil {
			oldConn.Close()
		}

		// Wait before retry (exponential backoff)
		if attempt > 0 {
			time.Sleep(backoff)
			backoff = backoff * 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}

		// Re-dial the server
		var opts []grpc.DialOption
		if secure {
			// TODO: Load TLS credentials
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		} else {
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		}

		opts = append(opts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}))

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		conn, err := grpc.DialContext(ctx, addr, opts...)
		cancel()

		if err != nil {
			// Log error and retry
			continue
		}

		// Initialize new client stub
		newClient := nanabushv1.NewTranslationServiceClient(conn)

		// Update connection
		c.mu.Lock()
		c.conn = conn
		c.client = newClient
		c.mu.Unlock()

		// Re-register with server
		ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
		err = c.register(ctx)
		cancel()

		if err != nil {
			conn.Close()
			// Log error and retry
			continue
		}

		// Restart heartbeat if needed
		c.mu.Lock()
		if c.registered {
			// Check if heartbeat is running
			select {
			case <-c.heartbeatStop:
				// Heartbeat stopped, restart it
				c.heartbeatStop = make(chan struct{})
				c.mu.Unlock()
				c.startHeartbeat()
			default:
				c.mu.Unlock()
			}
		} else {
			c.mu.Unlock()
		}

		// Success!
		return
	}

	// All retries failed - mark as unregistered
	c.mu.Lock()
	c.registered = false
	c.mu.Unlock()
}

// Reconfigure is not implemented - clients should be closed and recreated.
// This method exists for interface compatibility but should not be used.
// Instead, create a new client with NewClient() and replace the old one.
func (c *Client) Reconfigure(cfg Config) error {
	return fmt.Errorf("reconfigure not supported - close old client and create new one")
}

// Close closes the gRPC connection and stops the heartbeat.
func (c *Client) Close() error {
	// Stop heartbeat
	c.mu.Lock()
	if c.heartbeatStop != nil {
		close(c.heartbeatStop)
	}
	heartbeatWg := c.heartbeatWg
	c.mu.Unlock()

	heartbeatWg.Wait()

	// Close connection
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// IsRegistered returns whether the client is currently registered with the server.
func (c *Client) IsRegistered() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.registered
}

// ClientID returns the client ID assigned by the server.
func (c *Client) ClientID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.clientID
}

// Status returns the current connection status.
type Status struct {
	Connected         bool      `json:"connected"`
	Registered        bool      `json:"registered"`
	ClientID          string    `json:"clientId,omitempty"`
	LastHeartbeat     time.Time `json:"lastHeartbeat,omitempty"`
	MissedHeartbeats  int       `json:"missedHeartbeats"`
	HeartbeatInterval int64     `json:"heartbeatIntervalSeconds"`
	Status            string    `json:"status"` // "healthy", "warning", "error"
}

// Status returns the current connection status.
func (c *Client) Status() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	// Check connection state - consider connected if:
	// 1. Connection exists and is Ready, OR
	// 2. Client is registered and has recent heartbeat (connection might be in transient state)
	connState := connectivity.Idle
	if c.conn != nil {
		connState = c.conn.GetState()
	}
	connReady := connState == connectivity.Ready

	// Consider "connected" if registered with recent heartbeat, even if gRPC state isn't Ready
	// This handles transient connection states gracefully
	hasRecentHeartbeat := !c.lastHeartbeatTime.IsZero() && now.Sub(c.lastHeartbeatTime) < 3*c.heartbeatInterval
	effectivelyConnected := connReady || (c.registered && hasRecentHeartbeat)

	// Determine status based on registration and heartbeat state
	status := "error"
	if !c.registered {
		status = "error"
	} else if c.missedHeartbeats >= 3 {
		status = "error"
	} else if c.missedHeartbeats >= 1 {
		status = "warning"
	} else if hasRecentHeartbeat {
		// Has recent heartbeat - healthy
		status = "healthy"
	} else if c.lastHeartbeatTime.IsZero() {
		// Just registered, waiting for first heartbeat
		status = "warning"
	} else {
		// Haven't received heartbeat in too long
		status = "error"
	}

	return Status{
		Connected:         effectivelyConnected, // Use effective connection state
		Registered:        c.registered,
		ClientID:          c.clientID,
		LastHeartbeat:     c.lastHeartbeatTime,
		MissedHeartbeats:  c.missedHeartbeats,
		HeartbeatInterval: int64(c.heartbeatInterval.Seconds()),
		Status:            status,
	}
}

// CheckTitleRequest represents a title-only pre-flight check.
type CheckTitleRequest struct {
	Title          string
	LanguageTag    string // Target language (e.g., "fr-CA")
	SourceLanguage string // Source language (e.g., "EN")
}

// CheckTitleResponse indicates if Nanabush is ready.
type CheckTitleResponse struct {
	Ready                bool
	Message              string
	EstimatedTimeSeconds int32
}

// CheckTitle performs a lightweight pre-flight check with title only.
// This validates that Nanabush is ready and can handle the request.
func (c *Client) CheckTitle(ctx context.Context, req CheckTitleRequest) (*CheckTitleResponse, error) {
	if c.client == nil {
		return nil, fmt.Errorf("nanabush: client not initialized")
	}

	resp, err := c.client.CheckTitle(ctx, &nanabushv1.TitleCheckRequest{
		Title:          req.Title,
		LanguageTag:    req.LanguageTag,
		SourceLanguage: req.SourceLanguage,
	})
	if err != nil {
		return nil, fmt.Errorf("nanabush: CheckTitle: %w", err)
	}

	return &CheckTitleResponse{
		Ready:                resp.Ready,
		Message:              resp.Message,
		EstimatedTimeSeconds: resp.EstimatedTimeSeconds,
	}, nil
}

// DocumentContent represents document content and metadata.
type DocumentContent struct {
	Title    string
	Markdown string
	Slug     string
	Metadata map[string]string
}

// TranslateRequest contains the full translation request.
type TranslateRequest struct {
	JobID          string
	Namespace      string
	Primitive      string           // "title" or "doc-translate"
	Title          string           // For PRIMITIVE_TITLE
	Document       *DocumentContent // For PRIMITIVE_DOC_TRANSLATE
	TemplateHelper *DocumentContent // Optional template context
	SourceLanguage string
	TargetLanguage string
	SourceWikiURI  string
	PageID         string
	PageSlug       string
}

// TranslateResponse contains the translation result.
type TranslateResponse struct {
	JobID                string
	Success              bool
	TranslatedTitle      string
	TranslatedMarkdown   string
	ErrorMessage         string
	TokensUsed           int32
	InferenceTimeSeconds float64
	CompletedAt          time.Time
}

// Translate performs full document translation.
func (c *Client) Translate(ctx context.Context, req TranslateRequest) (*TranslateResponse, error) {
	if c.client == nil {
		return nil, fmt.Errorf("nanabush: client not initialized")
	}

	// Build the gRPC request
	grpcReq := &nanabushv1.TranslateRequest{
		JobId:          req.JobID,
		Namespace:      req.Namespace,
		SourceLanguage: req.SourceLanguage,
		TargetLanguage: req.TargetLanguage,
		SourceWikiUri:  req.SourceWikiURI,
		PageId:         req.PageID,
		PageSlug:       req.PageSlug,
		RequestedAt:    timestamppb.Now(),
	}

	// Handle primitive type and source content
	switch req.Primitive {
	case "title":
		grpcReq.Primitive = nanabushv1.PrimitiveType_PRIMITIVE_TITLE
		grpcReq.Source = &nanabushv1.TranslateRequest_Title{Title: req.Title}
	case "doc-translate":
		grpcReq.Primitive = nanabushv1.PrimitiveType_PRIMITIVE_DOC_TRANSLATE
		if req.Document == nil {
			return nil, fmt.Errorf("nanabush: Document is required for doc-translate primitive")
		}
		grpcReq.Source = &nanabushv1.TranslateRequest_Doc{
			Doc: &nanabushv1.DocumentContent{
				Title:    req.Document.Title,
				Markdown: req.Document.Markdown,
				Slug:     req.Document.Slug,
				Metadata: req.Document.Metadata,
			},
		}
	default:
		return nil, fmt.Errorf("nanabush: unsupported primitive type: %s", req.Primitive)
	}

	// Add template helper if provided
	if req.TemplateHelper != nil {
		grpcReq.TemplateHelper = &nanabushv1.DocumentContent{
			Title:    req.TemplateHelper.Title,
			Markdown: req.TemplateHelper.Markdown,
			Slug:     req.TemplateHelper.Slug,
			Metadata: req.TemplateHelper.Metadata,
		}
	}

	// Call the gRPC service
	resp, err := c.client.Translate(ctx, grpcReq)
	if err != nil {
		return nil, fmt.Errorf("nanabush: Translate: %w", err)
	}

	// Convert response
	var completedAt time.Time
	if resp.CompletedAt != nil {
		completedAt = resp.CompletedAt.AsTime()
	}

	return &TranslateResponse{
		JobID:                resp.JobId,
		Success:              resp.Success,
		TranslatedTitle:      resp.TranslatedTitle,
		TranslatedMarkdown:   resp.TranslatedMarkdown,
		ErrorMessage:         resp.ErrorMessage,
		TokensUsed:           resp.TokensUsed,
		InferenceTimeSeconds: resp.InferenceTimeSeconds,
		CompletedAt:          completedAt,
	}, nil
}

// Helper function to convert DocumentContent to proto
func documentContentToProto(doc *DocumentContent) *nanabushv1.DocumentContent {
	if doc == nil {
		return nil
	}
	return &nanabushv1.DocumentContent{
		Title:    doc.Title,
		Markdown: doc.Markdown,
		Slug:     doc.Slug,
		Metadata: doc.Metadata,
	}
}
