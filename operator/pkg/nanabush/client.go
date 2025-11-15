package nanabush

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
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
	clientID   string
	clientName string
	clientVersion string
	namespace  string
	metadata   map[string]string
	
	// Heartbeat
	heartbeatInterval time.Duration
	heartbeatStop     chan struct{}
	heartbeatWg       sync.WaitGroup
	
	// Connection state
	mu       sync.RWMutex
	registered bool
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

	conn, err := grpc.DialContext(ctx, cfg.Address, opts...)
	if err != nil {
		return nil, fmt.Errorf("nanabush: dial %s: %w", cfg.Address, err)
	}

	// Initialize generated client stub
	client := nanabushv1.NewTranslationServiceClient(conn)

	c := &Client{
		conn:          conn,
		addr:          cfg.Address,
		secure:        cfg.Secure,
		client:        client,
		clientName:    cfg.ClientName,
		clientVersion: cfg.ClientVersion,
		namespace:     cfg.Namespace,
		metadata:      cfg.Metadata,
		heartbeatInterval: 60 * time.Second, // Default: 60 seconds
		heartbeatStop: make(chan struct{}),
	}
	
	// Register with server
	if err := c.register(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("nanabush: register: %w", err)
	}
	
	// Start heartbeat goroutine
	c.startHeartbeat()
	
	return c, nil
}

// register registers the client with the server.
func (c *Client) register(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	resp, err := c.client.RegisterClient(ctx, &nanabushv1.RegisterClientRequest{
		ClientName:    c.clientName,
		ClientVersion: c.clientVersion,
		Namespace:     c.namespace,
		Metadata:      c.metadata,
		RegisteredAt:  timestamppb.Now(),
	})
	if err != nil {
		return fmt.Errorf("register client: %w", err)
	}
	
	if !resp.Success {
		return fmt.Errorf("registration failed: %s", resp.Message)
	}
	
	c.clientID = resp.ClientId
	c.registered = true
	
	// Update heartbeat interval from server response
	if resp.HeartbeatIntervalSeconds > 0 {
		c.heartbeatInterval = time.Duration(resp.HeartbeatIntervalSeconds) * time.Second
	}
	
	return nil
}

// startHeartbeat starts the heartbeat goroutine.
func (c *Client) startHeartbeat() {
	c.heartbeatWg.Add(1)
	go func() {
		defer c.heartbeatWg.Done()
		
		ticker := time.NewTicker(c.heartbeatInterval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				c.sendHeartbeat()
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
		return
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	resp, err := c.client.Heartbeat(ctx, &nanabushv1.HeartbeatRequest{
		ClientId:  clientID,
		ClientName: clientName,
		SentAt:    timestamppb.Now(),
		Metadata:  c.metadata,
	})
	if err != nil {
		// Connection error - need to re-register
		c.mu.Lock()
		c.registered = false
		c.mu.Unlock()
		
		// Try to reconnect and re-register
		go c.reconnectAndRegister()
		return
	}
	
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

// Close closes the gRPC connection and stops the heartbeat.
func (c *Client) Close() error {
	// Stop heartbeat
	close(c.heartbeatStop)
	c.heartbeatWg.Wait()
	
	// Close connection
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
		JobID:              resp.JobId,
		Success:            resp.Success,
		TranslatedTitle:    resp.TranslatedTitle,
		TranslatedMarkdown: resp.TranslatedMarkdown,
		ErrorMessage:       resp.ErrorMessage,
		TokensUsed:         resp.TokensUsed,
		InferenceTimeSeconds: resp.InferenceTimeSeconds,
		CompletedAt:        completedAt,
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
