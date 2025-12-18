package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	wikiv1alpha1 "github.com/dasmlab/glooscap-operator/api/v1alpha1"
	"github.com/dasmlab/glooscap-operator/internal/controller"
	"github.com/dasmlab/glooscap-operator/pkg/catalog"
	"github.com/dasmlab/glooscap-operator/pkg/nanabush"
)

// Options controls the API server.
type Options struct {
	Addr      string
	Catalogue *catalog.Store
	Jobs      *catalog.JobStore
	Client    client.Client
	APIReader client.Reader // Uncached client for reading ConfigMaps (avoids cache watch requirements)
	Nanabush  *nanabush.Client
	// NanabushStatusCh is a channel that receives nanabush status updates to trigger SSE broadcasts
	NanabushStatusCh <-chan struct{}
	// GetNanabushClient is a function that returns the current nanabush client (for runtime updates)
	GetNanabushClient func() *nanabush.Client
	// ConfigStore manages runtime configuration
	ConfigStore *ConfigStore
	// ReconfigureTranslationService is a callback to reconfigure the translation service client
	ReconfigureTranslationService func(cfg TranslationServiceConfig) error
	// OutlineClientFactory creates Outline clients for WikiTargets
	OutlineClientFactory controller.OutlineClientFactory
	// TranslationJobEventCh is a channel that receives TranslationJob events to trigger SSE broadcasts
	TranslationJobEventCh <-chan controller.TranslationJobEvent
}

// eventBroadcaster manages SSE connections and broadcasts events.
type eventBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[chan []byte]struct{}
	trigger     chan struct{} // Channel to trigger immediate event send
}

func newEventBroadcaster() *eventBroadcaster {
	return &eventBroadcaster{
		subscribers: make(map[chan []byte]struct{}),
		trigger:     make(chan struct{}, 1),
	}
}

func (eb *eventBroadcaster) subscribe() chan []byte {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	ch := make(chan []byte, 10)
	eb.subscribers[ch] = struct{}{}
	return ch
}

func (eb *eventBroadcaster) unsubscribe(ch chan []byte) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	delete(eb.subscribers, ch)
	close(ch)
}

func (eb *eventBroadcaster) broadcast(data []byte) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	for ch := range eb.subscribers {
		select {
		case ch <- data:
		default:
			// Channel full, skip this subscriber
		}
	}
}

func (eb *eventBroadcaster) triggerBroadcast() {
	select {
	case eb.trigger <- struct{}{}:
	default:
		// Already triggered
	}
}

// Start launches the API server and blocks until the context is cancelled.
func Start(ctx context.Context, opts Options) error {
	if opts.Addr == "" {
		opts.Addr = ":3000"
	}

	broadcaster := newEventBroadcaster()

	// Start background goroutine to send periodic events and listen for store updates
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		var updateCh <-chan struct{}
		if opts.Catalogue != nil {
			updateCh = opts.Catalogue.NotifyUpdate()
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sendStateEvent(broadcaster, opts)
			case <-broadcaster.trigger:
				sendStateEvent(broadcaster, opts)
			case <-updateCh:
				// Store was updated, send event immediately
				sendStateEvent(broadcaster, opts)
			case <-opts.NanabushStatusCh:
				// Nanabush status changed, send event immediately
				sendStateEvent(broadcaster, opts)
			case jobEvent := <-opts.TranslationJobEventCh:
				// TranslationJob event received, send it immediately
				eventData := map[string]any{
					"event": "translation_job",
					"data":  jobEvent,
				}
				if data, err := json.Marshal(eventData); err == nil {
					broadcaster.broadcast(data)
				}
			}
		}
	}()

	router := chi.NewRouter()

	// Request logging middleware - log ALL requests
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(os.Stderr, "[http] %s %s %s\n", r.Method, r.URL.Path, r.RemoteAddr)
			fmt.Printf("[http] %s %s %s\n", r.Method, r.URL.Path, r.RemoteAddr)
			next.ServeHTTP(w, r)
		})
	})

	// CORS headers for UI
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Allow specific origins or all origins in development
			allowedOrigins := []string{
				"https://web-glooscap.apps.ocp-ai-sno-2.rh.dasmlab.org",
				"http://web-glooscap.apps.ocp-ai-sno-2.rh.dasmlab.org",
				"http://localhost:9000",
				"http://localhost:8080",
			}

			// When using credentials, we MUST use a specific origin, not "*"
			allowOrigin := ""
			if origin != "" {
				for _, allowed := range allowedOrigins {
					if origin == allowed {
						allowOrigin = origin
						break
					}
				}
				// If no match but we have an origin, allow it (for development flexibility)
				if allowOrigin == "" {
					allowOrigin = origin
				}
			}

			// If no origin header, default to wildcard (but can't use credentials then)
			if allowOrigin == "" {
				allowOrigin = "*"
				// Don't set credentials if using wildcard
			} else {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept")
			w.Header().Set("Access-Control-Expose-Headers", "Content-Type, Content-Length")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	router.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Status endpoint for translation service connection
	// Supports both Nanabush and Iskoces (backward compatible with /status/nanabush)
	router.Get("/api/v1/status/nanabush", func(w http.ResponseWriter, r *http.Request) {
		// Get client status first (most up-to-date)
		var nanabushClient *nanabush.Client
		if opts.GetNanabushClient != nil {
			nanabushClient = opts.GetNanabushClient()
		} else if opts.Nanabush != nil {
			nanabushClient = opts.Nanabush
		}

		var clientStatus nanabush.Status
		if nanabushClient != nil {
			clientStatus = nanabushClient.Status()
		} else {
			clientStatus = nanabush.Status{
				Connected:  false,
				Registered: false,
				Status:     "error",
			}
		}

		// Try to read from TranslationService CR status
		// Prefer client status if it shows connected/registered but CR doesn't (handles startup race condition)
		if opts.Client != nil {
			tsName := "glooscap-translation-service"
			var ts wikiv1alpha1.TranslationService
			err := opts.Client.Get(r.Context(), client.ObjectKey{Name: tsName}, &ts)
			if err == nil {
				// CR exists - check if status is populated
				if ts.Status.ClientID != "" || ts.Status.Status != "" {
					// CR status is populated - but prefer client status if it's more accurate
					// This handles the case where client is connected but CR hasn't been updated yet
					if clientStatus.Connected && clientStatus.Registered && (!ts.Status.Connected || !ts.Status.Registered) {
						// Client is connected but CR shows disconnected - prefer client status (more recent)
						writeJSON(w, clientStatus)
						return
					}
					// CR status is populated and matches client, or client is not connected - use CR status
					var lastHeartbeat time.Time
					if ts.Status.LastHeartbeat != nil {
						lastHeartbeat = ts.Status.LastHeartbeat.Time
					}
					writeJSON(w, nanabush.Status{
						ClientID:          ts.Status.ClientID,
						Connected:         ts.Status.Connected,
						Registered:        ts.Status.Registered,
						Status:            ts.Status.Status,
						MissedHeartbeats:  ts.Status.MissedHeartbeats,
						HeartbeatInterval: int64(ts.Status.HeartbeatIntervalSeconds), // Already in seconds
						LastHeartbeat:     lastHeartbeat,
					})
					return
				}
			}
		}

		// No CR or CR status not populated - use client status
		writeJSON(w, clientStatus)
	})

	// Generic translation service status endpoint (alias for backward compatibility)
	router.Get("/api/v1/status/translation", func(w http.ResponseWriter, r *http.Request) {
		// Try to read from TranslationService CR status first
		if opts.Client != nil {
			tsName := "glooscap-translation-service"
			var ts wikiv1alpha1.TranslationService
			err := opts.Client.Get(r.Context(), client.ObjectKey{Name: tsName}, &ts)
			if err == nil {
				// Return status from CR
				var lastHeartbeat time.Time
				if ts.Status.LastHeartbeat != nil {
					lastHeartbeat = ts.Status.LastHeartbeat.Time
				}
				writeJSON(w, nanabush.Status{
					ClientID:          ts.Status.ClientID,
					Connected:         ts.Status.Connected,
					Registered:        ts.Status.Registered,
					Status:            ts.Status.Status,
					MissedHeartbeats:  ts.Status.MissedHeartbeats,
					HeartbeatInterval: int64(ts.Status.HeartbeatIntervalSeconds), // Already in seconds
					LastHeartbeat:     lastHeartbeat,
				})
				return
			}
		}

		// Fallback to client status if CR doesn't exist
		var nanabushClient *nanabush.Client
		if opts.GetNanabushClient != nil {
			nanabushClient = opts.GetNanabushClient()
		} else if opts.Nanabush != nil {
			nanabushClient = opts.Nanabush
		}

		if nanabushClient == nil {
			writeJSON(w, nanabush.Status{
				Connected:  false,
				Registered: false,
				Status:     "error",
			})
			return
		}
		status := nanabushClient.Status()
		writeJSON(w, status)
	})

	router.Get("/api/v1/catalogue", func(w http.ResponseWriter, r *http.Request) {
		target := r.URL.Query().Get("target")
		var pages []*catalog.Page
		if opts.Catalogue != nil {
			pages = opts.Catalogue.List(target)
		}
		writeJSON(w, pages)
	})

	router.Get("/api/v1/targets", func(w http.ResponseWriter, r *http.Request) {
		var targets []catalog.Target
		if opts.Catalogue != nil {
			targets = opts.Catalogue.Targets()
		}
		writeJSON(w, targets)
	})

	router.Get("/api/v1/wikitargets", func(w http.ResponseWriter, r *http.Request) {
		if opts.Client == nil {
			http.Error(w, "kubernetes client not configured", http.StatusServiceUnavailable)
			return
		}
		namespace := r.URL.Query().Get("namespace")
		if namespace == "" {
			namespace = "glooscap-system"
		}

		var list wikiv1alpha1.WikiTargetList
		if err := opts.Client.List(r.Context(), &list, client.InNamespace(namespace)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		result := make([]map[string]any, 0, len(list.Items))
		for _, item := range list.Items {
			status := map[string]any{
				"catalogRevision": item.Status.CatalogRevision,
			}
			if item.Status.LastSyncTime != nil {
				status["lastSyncTime"] = item.Status.LastSyncTime.Time.Format(time.RFC3339)
			}
			conditions := make([]map[string]any, 0, len(item.Status.Conditions))
			for _, cond := range item.Status.Conditions {
				conditions = append(conditions, map[string]any{
					"type":               cond.Type,
					"status":             string(cond.Status),
					"reason":             cond.Reason,
					"message":            cond.Message,
					"lastTransitionTime": cond.LastTransitionTime.Time.Format(time.RFC3339),
				})
			}
			status["conditions"] = conditions

			result = append(result, map[string]any{
				"name":      item.Name,
				"namespace": item.Namespace,
				"uri":       item.Spec.URI,
				"mode":      string(item.Spec.Mode),
				"status":    status,
			})
		}
		writeJSON(w, map[string]any{"items": result})
	})

	router.Get("/api/v1/jobs", func(w http.ResponseWriter, _ *http.Request) {
		result := map[string]any{}
		if opts.Jobs != nil {
			result["items"] = opts.Jobs.List()
		} else {
			result["items"] = map[string]any{}
		}
		writeJSON(w, result)
	})

	// SSE endpoint for real-time catalogue updates
	// API endpoint to inspect DB state
	router.Get("/api/v1/db/state", func(w http.ResponseWriter, r *http.Request) {
		if opts.Catalogue == nil {
			writeJSON(w, map[string]any{"error": "catalogue not available"})
			return
		}

		state := buildStateResponse(opts)
		writeJSON(w, state)
	})

	// SSE endpoint for real-time WikiTarget and page state updates
	router.Get("/api/v1/events", func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers first
		origin := r.Header.Get("Origin")
		allowedOrigins := []string{
			"https://web-glooscap.apps.ocp-ai-sno-2.rh.dasmlab.org",
			"http://web-glooscap.apps.ocp-ai-sno-2.rh.dasmlab.org",
			"http://localhost:9000",
			"http://localhost:8080",
		}
		// When using credentials, we MUST use a specific origin, not "*"
		allowOrigin := ""
		if origin != "" {
			for _, allowed := range allowedOrigins {
				if origin == allowed {
					allowOrigin = origin
					break
				}
			}
			// If no match but we have an origin, allow it (for development flexibility)
			if allowOrigin == "" {
				allowOrigin = origin
			}
		}

		// If no origin header, default to wildcard (but can't use credentials then)
		if allowOrigin == "" {
			allowOrigin = "*"
		} else {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// Set up SSE headers - MUST be set before any writes
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Cache-Control")
		w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering
		// Additional headers to help with HTTP/2 SSE compatibility
		w.Header().Set("X-Content-Type-Options", "nosniff")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "SSE not supported", http.StatusInternalServerError)
			return
		}

		// Subscribe to events
		eventCh := broadcaster.subscribe()
		defer broadcaster.unsubscribe(eventCh)

		// Send initial state immediately
		initialState := buildStateResponse(opts)
		if data, err := json.Marshal(initialState); err == nil {
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}

		// Keepalive ticker to send periodic pings
		keepaliveTicker := time.NewTicker(15 * time.Second)
		defer keepaliveTicker.Stop()

		// Listen for events
		for {
			select {
			case <-r.Context().Done():
				return
			case <-keepaliveTicker.C:
				// Send keepalive comment
				fmt.Fprintf(w, ": keepalive\n\n")
				flusher.Flush()
			case data := <-eventCh:
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		}
	})

	// API endpoint to trigger immediate event broadcast
	router.Post("/api/v1/events/refresh", func(w http.ResponseWriter, r *http.Request) {
		broadcaster.triggerBroadcast()
		writeJSON(w, map[string]string{"status": "refresh triggered"})
	})

	// API endpoint to approve duplicate overwrite
	router.Patch("/api/v1/jobs/{namespace}/{jobId}/approve-duplicate", func(w http.ResponseWriter, r *http.Request) {
		if opts.Client == nil {
			http.Error(w, "job approval not configured", http.StatusServiceUnavailable)
			return
		}
		namespace := chi.URLParam(r, "namespace")
		jobId := chi.URLParam(r, "jobId")
		if namespace == "" || jobId == "" {
			http.Error(w, "namespace and jobId are required", http.StatusBadRequest)
			return
		}

		var job wikiv1alpha1.TranslationJob
		if err := opts.Client.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: jobId}, &job); err != nil {
			if errors.IsNotFound(err) {
				http.Error(w, "translation job not found", http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Add approval annotation
		if job.Annotations == nil {
			job.Annotations = make(map[string]string)
		}
		job.Annotations["glooscap.dasmlab.org/duplicate-approved"] = "true"

		if err := opts.Client.Update(r.Context(), &job); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]string{"status": "approved"})
	})

	router.Post("/api/v1/jobs", func(w http.ResponseWriter, r *http.Request) {
		if opts.Client == nil {
			http.Error(w, "job submission not configured", http.StatusServiceUnavailable)
			return
		}
		var req createJobRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := req.validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		job := &wikiv1alpha1.TranslationJob{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "translation-",
				Namespace:    req.Namespace,
			},
			Spec: wikiv1alpha1.TranslationJobSpec{
				Source: wikiv1alpha1.TranslationSourceSpec{
					TargetRef: req.TargetRef,
					PageID:    req.PageID,
				},
				Destination: &wikiv1alpha1.TranslationDestinationSpec{
					TargetRef:   req.TargetRef,
					LanguageTag: req.LanguageTag,
				},
				Pipeline: wikiv1alpha1.TranslationPipelineMode(req.Pipeline),
				Parameters: map[string]string{
					"pageTitle": req.PageTitle,
				},
			},
		}

		if err := opts.Client.Create(r.Context(), job); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"name": job.Name})
	})

	// Get page content endpoint (for analysis)
	router.Get("/api/v1/pages/{targetRef}/{pageId}/content", func(w http.ResponseWriter, r *http.Request) {
		if opts.Client == nil {
			http.Error(w, "page content retrieval not configured", http.StatusServiceUnavailable)
			return
		}
		if opts.OutlineClientFactory == nil {
			http.Error(w, "outline client factory not configured", http.StatusServiceUnavailable)
			return
		}

		targetRef := chi.URLParam(r, "targetRef")
		pageID := chi.URLParam(r, "pageId")
		namespace := r.URL.Query().Get("namespace")
		if namespace == "" {
			namespace = "glooscap-system"
		}

		if targetRef == "" || pageID == "" {
			http.Error(w, "targetRef and pageId are required", http.StatusBadRequest)
			return
		}

		ctx := r.Context()

		// Get WikiTarget
		var target wikiv1alpha1.WikiTarget
		if err := opts.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: targetRef}, &target); err != nil {
			if errors.IsNotFound(err) {
				http.Error(w, "WikiTarget not found", http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Create Outline client
		outlineClient, err := opts.OutlineClientFactory.New(ctx, opts.Client, &target)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to create outline client: %v", err), http.StatusInternalServerError)
			return
		}

		// Get page content
		pageContent, err := outlineClient.GetPageContent(ctx, pageID)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to fetch page content: %v", err), http.StatusInternalServerError)
			return
		}

		// Get page metadata from catalog if available
		var pageMetadata map[string]any
		if opts.Catalogue != nil {
			targetID := fmt.Sprintf("%s/%s", target.Namespace, target.Name)
			pages := opts.Catalogue.List(targetID)
			for _, p := range pages {
				if p.ID == pageID {
					pageMetadata = map[string]any{
						"id":         p.ID,
						"title":      p.Title,
						"slug":       p.Slug,
						"language":   p.Language,
						"collection": p.Collection,
						"template":   p.Template,
						"isTemplate": p.IsTemplate,
						"uri":        p.URI,
					}
					break
				}
			}
		}

		// Enrich page content with title and slug from metadata if available
		if pageMetadata != nil {
			if title, ok := pageMetadata["title"].(string); ok && title != "" && pageContent.Title == "" {
				pageContent.Title = title
			}
			if slug, ok := pageMetadata["slug"].(string); ok && slug != "" && pageContent.Slug == "" {
				pageContent.Slug = slug
			}
			if uri, ok := pageMetadata["uri"].(string); ok && uri != "" {
				// Use URI from metadata if available
			}
		}

		// Log the content length for debugging
		markdownLen := len(pageContent.Markdown)
		if markdownLen == 0 {
			// Log warning if markdown is empty
		}

		writeJSON(w, map[string]any{
			"pageId":    pageContent.ID,
			"title":     pageContent.Title,
			"slug":      pageContent.Slug,
			"markdown":  pageContent.Markdown,
			"metadata":  pageMetadata,
			"rawLength": markdownLen,
		})
	})

	// Approve/publish draft page endpoint - creates a publish job
	router.Post("/api/v1/approve-translation", func(w http.ResponseWriter, r *http.Request) {
		if opts.Client == nil {
			http.Error(w, "client not configured", http.StatusServiceUnavailable)
			return
		}

		var req struct {
			JobName   string `json:"jobName"`
			Namespace string `json:"namespace"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.JobName == "" || req.Namespace == "" {
			http.Error(w, "jobName and namespace are required", http.StatusBadRequest)
			return
		}

		ctx := r.Context()

		// Get TranslationJob
		var job wikiv1alpha1.TranslationJob
		if err := opts.Client.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: req.JobName}, &job); err != nil {
			if errors.IsNotFound(err) {
				http.Error(w, "TranslationJob not found", http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Verify job is in AwaitingApproval state
		if job.Status.State != wikiv1alpha1.TranslationJobStateAwaitingApproval {
			http.Error(w, fmt.Sprintf("job is not awaiting approval (current state: %s)", job.Status.State), http.StatusBadRequest)
			return
		}

		// Get page ID from annotations
		pageID := ""
		if job.Annotations != nil {
			if id, ok := job.Annotations["glooscap.dasmlab.org/published-page-id"]; ok {
				pageID = id
			}
		}

		if pageID == "" {
			http.Error(w, "no published page ID found in job annotations", http.StatusBadRequest)
			return
		}

		// Get destination WikiTarget
		destTargetRef := job.Spec.Source.TargetRef
		if job.Spec.Destination != nil && job.Spec.Destination.TargetRef != "" {
			destTargetRef = job.Spec.Destination.TargetRef
		}

		var destTarget wikiv1alpha1.WikiTarget
		if err := opts.Client.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: destTargetRef}, &destTarget); err != nil {
			http.Error(w, fmt.Sprintf("failed to get destination WikiTarget: %v", err), http.StatusInternalServerError)
			return
		}

		// Create a publish job (TranslationJob with Pipeline=Publish)
		// For now, we'll use a special parameter to indicate this is a publish job
		publishJobName := fmt.Sprintf("publish-%s", job.Name)
		publishJob := &wikiv1alpha1.TranslationJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      publishJobName,
				Namespace: req.Namespace,
				Labels: map[string]string{
					"glooscap.dasmlab.org/publish-job": "true",
					"glooscap.dasmlab.org/original-job": job.Name,
				},
			},
			Spec: wikiv1alpha1.TranslationJobSpec{
				Source: wikiv1alpha1.TranslationSourceSpec{
					TargetRef: destTargetRef,
					PageID:    pageID, // The draft page ID to publish
				},
				Pipeline: wikiv1alpha1.TranslationPipelineModeTektonJob,
				Parameters: map[string]string{
					"publish":      "true",
					"originalJob":  job.Name,
					"pageId":       pageID,
					"targetRef":    destTargetRef,
				},
			},
		}

		// Create the publish job
		if err := opts.Client.Create(ctx, publishJob); err != nil {
			if errors.IsAlreadyExists(err) {
				http.Error(w, "publish job already exists", http.StatusConflict)
				return
			}
			http.Error(w, fmt.Sprintf("failed to create publish job: %v", err), http.StatusInternalServerError)
			return
		}

		// Update original job to mark approval
		if job.Annotations == nil {
			job.Annotations = make(map[string]string)
		}
		job.Annotations["glooscap.dasmlab.org/approved-at"] = time.Now().Format(time.RFC3339)
		job.Annotations["glooscap.dasmlab.org/publish-job"] = publishJobName
		if err := opts.Client.Update(ctx, &job); err != nil {
			fmt.Printf("warning: failed to update job annotations: %v\n", err)
		}

		writeJSON(w, map[string]any{
			"success":      true,
			"publishJob":   publishJobName,
			"originalJob":  job.Name,
			"message":      "Publish job created successfully",
		})
	})

	// Direct translation endpoint (MVP)
	router.Post("/api/v1/translate", func(w http.ResponseWriter, r *http.Request) {
		if opts.Client == nil {
			http.Error(w, "translation not configured", http.StatusServiceUnavailable)
			return
		}
		// Use getter function if available (for runtime updates), otherwise use direct reference
		var nanabushClient *nanabush.Client
		if opts.GetNanabushClient != nil {
			nanabushClient = opts.GetNanabushClient()
		} else if opts.Nanabush != nil {
			nanabushClient = opts.Nanabush
		}

		if nanabushClient == nil {
			http.Error(w, "translation service not available", http.StatusServiceUnavailable)
			return
		}
		if opts.OutlineClientFactory == nil {
			http.Error(w, "outline client factory not configured", http.StatusServiceUnavailable)
			return
		}

		var req struct {
			TargetRef   string `json:"targetRef"`
			Namespace   string `json:"namespace"`
			PageID      string `json:"pageId"`
			PageTitle   string `json:"pageTitle"`
			LanguageTag string `json:"languageTag"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.TargetRef == "" || req.PageID == "" {
			http.Error(w, "targetRef and pageId are required", http.StatusBadRequest)
			return
		}

		ctx := r.Context()

		// Get WikiTarget
		var target wikiv1alpha1.WikiTarget
		namespace := req.Namespace
		if namespace == "" {
			namespace = "glooscap-system"
		}
		if err := opts.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: req.TargetRef}, &target); err != nil {
			if errors.IsNotFound(err) {
				http.Error(w, "WikiTarget not found", http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Create Outline client
		outlineClient, err := opts.OutlineClientFactory.New(ctx, opts.Client, &target)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to create outline client: %v", err), http.StatusInternalServerError)
			return
		}

		// Get page content
		pageContent, err := outlineClient.GetPageContent(ctx, req.PageID)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to fetch page content: %v", err), http.StatusInternalServerError)
			return
		}

		// Get page metadata from catalog if available
		var sourcePage *catalog.Page
		if opts.Catalogue != nil {
			targetID := fmt.Sprintf("%s/%s", target.Namespace, target.Name)
			pages := opts.Catalogue.List(targetID)
			for _, p := range pages {
				if p.ID == req.PageID {
					sourcePage = p
					break
				}
			}
		}

		// Enrich page content with title if available
		if pageContent.Title == "" {
			if sourcePage != nil {
				pageContent.Title = sourcePage.Title
			} else if req.PageTitle != "" {
				pageContent.Title = req.PageTitle
			}
		}

		// Determine source language
		sourceLang := "en"
		if sourcePage != nil && sourcePage.Language != "" {
			sourceLang = sourcePage.Language
		}

		// Determine target language
		targetLang := req.LanguageTag
		if targetLang == "" {
			targetLang = "fr-CA"
		}

		// Call translation service
		grpcReq := nanabush.TranslateRequest{
			JobID:     fmt.Sprintf("direct-%s", req.PageID),
			Namespace: namespace,
			Primitive: "doc-translate",
			Document: &nanabush.DocumentContent{
				Title:    pageContent.Title,
				Markdown: pageContent.Markdown,
				Slug:     pageContent.Slug,
			},
			SourceLanguage: sourceLang,
			TargetLanguage: targetLang,
			SourceWikiURI:  target.Spec.URI,
			PageID:         req.PageID,
			PageSlug:       pageContent.Slug,
		}

		// Use a longer timeout for translation (5 minutes) to handle large documents
		translateCtx, translateCancel := context.WithTimeout(ctx, 5*time.Minute)
		defer translateCancel()
		translateResp, err := nanabushClient.Translate(translateCtx, grpcReq)
		if err != nil {
			http.Error(w, fmt.Sprintf("translation failed: %v", err), http.StatusInternalServerError)
			return
		}

		if !translateResp.Success {
			http.Error(w, fmt.Sprintf("translation failed: %s", translateResp.ErrorMessage), http.StatusInternalServerError)
			return
		}

		// For MVP: Return the translated content
		// TODO: Create page in Outline with translated content and "TRANSLATED" prefix
		writeJSON(w, map[string]any{
			"success":            true,
			"originalTitle":      pageContent.Title,
			"translatedTitle":    translateResp.TranslatedTitle,
			"translatedMarkdown": translateResp.TranslatedMarkdown,
			"tokensUsed":         translateResp.TokensUsed,
			"inferenceTime":      translateResp.InferenceTimeSeconds,
			"message":            "Translation completed. Page creation coming soon.",
		})
	})

	// Translation Service Configuration CRUD endpoints
	router.Get("/api/v1/translation-service", func(w http.ResponseWriter, r *http.Request) {
		if opts.Client == nil {
			http.Error(w, "kubernetes client not configured", http.StatusServiceUnavailable)
			return
		}

		// Try to read from TranslationService CR first
		tsName := "glooscap-translation-service"
		var ts wikiv1alpha1.TranslationService
		err := opts.Client.Get(r.Context(), client.ObjectKey{Name: tsName}, &ts)
		if err == nil {
			// Return config from CR
			writeJSON(w, TranslationServiceConfig{
				Address: ts.Spec.Address,
				Type:    ts.Spec.Type,
				Secure:  ts.Spec.Secure,
			})
			return
		}

		// Fallback to ConfigStore for backward compatibility
		if opts.ConfigStore != nil {
			config := opts.ConfigStore.GetTranslationServiceConfig()
			if config != nil {
				writeJSON(w, config)
				return
			}
		}

		// Return empty config if not set
		writeJSON(w, TranslationServiceConfig{})
	})

	router.Post("/api/v1/translation-service", func(w http.ResponseWriter, r *http.Request) {
		if opts.Client == nil {
			http.Error(w, "kubernetes client not configured", http.StatusServiceUnavailable)
			return
		}

		var config TranslationServiceConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Validate required fields
		if config.Address == "" {
			http.Error(w, "address is required", http.StatusBadRequest)
			return
		}
		if config.Type == "" {
			// Default to iskoces if not specified
			config.Type = "iskoces"
		}

		// Create or update TranslationService CR
		// Use a fixed name since TranslationService is cluster-scoped
		tsName := "glooscap-translation-service"
		fmt.Printf("[http] POST /translation-service: Creating/updating TranslationService CR '%s' with address=%s, type=%s, secure=%v\n", tsName, config.Address, config.Type, config.Secure)
		var ts wikiv1alpha1.TranslationService
		err := opts.Client.Get(r.Context(), client.ObjectKey{Name: tsName}, &ts)
		if err != nil {
			if errors.IsNotFound(err) {
				// Create new TranslationService
				ts = wikiv1alpha1.TranslationService{
					ObjectMeta: metav1.ObjectMeta{
						Name: tsName,
					},
					Spec: wikiv1alpha1.TranslationServiceSpec{
						Address: config.Address,
						Type:    config.Type,
						Secure:  config.Secure,
					},
				}
				if err := opts.Client.Create(r.Context(), &ts); err != nil {
					fmt.Printf("[http] ERROR: Failed to create TranslationService CR '%s': %v (error type: %T)\n", tsName, err, err)
					http.Error(w, fmt.Sprintf("failed to create TranslationService: %v", err), http.StatusInternalServerError)
					return
				}
				fmt.Printf("[http] Successfully created TranslationService CR: %s\n", tsName)
			} else {
				fmt.Printf("[http] ERROR: Failed to get TranslationService CR '%s' (non-NotFound): %v (error type: %T)\n", tsName, err, err)
				http.Error(w, fmt.Sprintf("failed to get TranslationService: %v", err), http.StatusInternalServerError)
				return
			}
		} else {
			// Update existing TranslationService
			ts.Spec.Address = config.Address
			ts.Spec.Type = config.Type
			ts.Spec.Secure = config.Secure
			if err := opts.Client.Update(r.Context(), &ts); err != nil {
				fmt.Printf("[http] ERROR: Failed to update TranslationService CR '%s': %v (error type: %T)\n", tsName, err, err)
				http.Error(w, fmt.Sprintf("failed to update TranslationService: %v", err), http.StatusInternalServerError)
				return
			}
			fmt.Printf("[http] Successfully updated TranslationService CR: %s\n", tsName)
		}

		// Store configuration in config store for backward compatibility
		if opts.ConfigStore != nil {
			opts.ConfigStore.SetTranslationServiceConfig(&config)
		}

		// Return success - reconciliation will happen via controller
		writeJSON(w, map[string]string{
			"status":  "reconfiguration_initiated",
			"address": config.Address,
			"type":    config.Type,
			"message": "Translation service reconfiguration started. Connection will be established in the background.",
		})
	})

	router.Put("/api/v1/translation-service", func(w http.ResponseWriter, r *http.Request) {
		// PUT is same as POST for this resource - reuse POST handler logic
		if opts.Client == nil {
			http.Error(w, "kubernetes client not configured", http.StatusServiceUnavailable)
			return
		}

		var config TranslationServiceConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Validate required fields
		if config.Address == "" {
			http.Error(w, "address is required", http.StatusBadRequest)
			return
		}
		if config.Type == "" {
			// Default to iskoces if not specified
			config.Type = "iskoces"
		}

		// Create or update TranslationService CR
		tsName := "glooscap-translation-service"
		fmt.Printf("[http] PUT /translation-service: Creating/updating TranslationService CR '%s' with address=%s, type=%s, secure=%v\n", tsName, config.Address, config.Type, config.Secure)
		var ts wikiv1alpha1.TranslationService
		err := opts.Client.Get(r.Context(), client.ObjectKey{Name: tsName}, &ts)
		if err != nil {
			if errors.IsNotFound(err) {
				// Create new TranslationService
				ts = wikiv1alpha1.TranslationService{
					ObjectMeta: metav1.ObjectMeta{
						Name: tsName,
					},
					Spec: wikiv1alpha1.TranslationServiceSpec{
						Address: config.Address,
						Type:    config.Type,
						Secure:  config.Secure,
					},
				}
				if err := opts.Client.Create(r.Context(), &ts); err != nil {
					fmt.Printf("[http] ERROR: Failed to create TranslationService CR '%s': %v (error type: %T)\n", tsName, err, err)
					http.Error(w, fmt.Sprintf("failed to create TranslationService: %v", err), http.StatusInternalServerError)
					return
				}
				fmt.Printf("[http] Successfully created TranslationService CR: %s\n", tsName)
			} else {
				fmt.Printf("[http] ERROR: Failed to get TranslationService CR '%s' (non-NotFound): %v (error type: %T)\n", tsName, err, err)
				http.Error(w, fmt.Sprintf("failed to get TranslationService: %v", err), http.StatusInternalServerError)
				return
			}
		} else {
			// Update existing TranslationService
			ts.Spec.Address = config.Address
			ts.Spec.Type = config.Type
			ts.Spec.Secure = config.Secure
			if err := opts.Client.Update(r.Context(), &ts); err != nil {
				fmt.Printf("[http] ERROR: Failed to update TranslationService CR '%s': %v (error type: %T)\n", tsName, err, err)
				http.Error(w, fmt.Sprintf("failed to update TranslationService: %v", err), http.StatusInternalServerError)
				return
			}
			fmt.Printf("[http] Successfully updated TranslationService CR: %s\n", tsName)
		}

		// Store configuration in config store for backward compatibility
		if opts.ConfigStore != nil {
			opts.ConfigStore.SetTranslationServiceConfig(&config)
		}

		// Return success - reconciliation will happen via controller
		writeJSON(w, map[string]string{
			"status":  "reconfiguration_initiated",
			"address": config.Address,
			"type":    config.Type,
			"message": "Translation service reconfiguration started. Connection will be established in the background.",
		})
	})

	router.Delete("/api/v1/translation-service", func(w http.ResponseWriter, r *http.Request) {
		if opts.Client == nil {
			http.Error(w, "kubernetes client not configured", http.StatusServiceUnavailable)
			return
		}

		// Delete TranslationService CR
		tsName := "glooscap-translation-service"
		var ts wikiv1alpha1.TranslationService
		err := opts.Client.Get(r.Context(), client.ObjectKey{Name: tsName}, &ts)
		if err != nil {
			if errors.IsNotFound(err) {
				// Already deleted, return success
				writeJSON(w, map[string]string{
					"status":  "deleted",
					"message": "Translation service configuration already cleared",
				})
				return
			}
			http.Error(w, fmt.Sprintf("failed to get TranslationService: %v", err), http.StatusInternalServerError)
			return
		}

		// Delete the CR
		if err := opts.Client.Delete(r.Context(), &ts); err != nil {
			http.Error(w, fmt.Sprintf("failed to delete TranslationService: %v", err), http.StatusInternalServerError)
			return
		}

		// Clear config store for backward compatibility
		if opts.ConfigStore != nil {
			opts.ConfigStore.SetTranslationServiceConfig(nil)
		}

		writeJSON(w, map[string]string{
			"status":  "deleted",
			"message": "Translation service configuration cleared",
		})
	})

	router.Delete("/api/v1/translation-service-old", func(w http.ResponseWriter, r *http.Request) {
		if opts.ConfigStore == nil {
			http.Error(w, "configuration store not available", http.StatusServiceUnavailable)
			return
		}
		if opts.ReconfigureTranslationService == nil {
			http.Error(w, "translation service reconfiguration not available", http.StatusServiceUnavailable)
			return
		}

		// Clear configuration
		opts.ConfigStore.SetTranslationServiceConfig(nil)

		// Close existing client (by setting empty config)
		emptyConfig := TranslationServiceConfig{
			Address: "",
			Type:    "",
			Secure:  false,
		}
		if err := opts.ReconfigureTranslationService(emptyConfig); err != nil {
			// Log but don't fail - client might already be closed
			fmt.Printf("[http] Error clearing translation service: %v\n", err)
		}

		writeJSON(w, map[string]string{"status": "deleted"})
	})

	// Diagnostic write enabled flag endpoints
	router.Get("/api/v1/diagnostic/write-enabled", func(w http.ResponseWriter, r *http.Request) {
		if opts.Client == nil {
			http.Error(w, "kubernetes client not configured", http.StatusServiceUnavailable)
			return
		}

		ctx := r.Context()
		configMapName := "glooscap-config"
		namespace := "glooscap-system"

		var cm corev1.ConfigMap
		// Use APIReader (uncached client) to avoid requiring cluster-wide ConfigMap watch permissions
		reader := opts.APIReader
		if reader == nil {
			// Fallback to cached client if APIReader not set
			reader = opts.Client
		}
		err := reader.Get(ctx, client.ObjectKey{Namespace: namespace, Name: configMapName}, &cm)
		if err != nil {
			if errors.IsNotFound(err) {
				// ConfigMap doesn't exist, return default (enabled)
				writeJSON(w, map[string]bool{"enabled": true})
				return
			}
			http.Error(w, fmt.Sprintf("failed to get config: %v", err), http.StatusInternalServerError)
			return
		}

		// Check the diagnostic-write-enabled key
		enabled := true // Default to enabled
		if val, exists := cm.Data["diagnostic-write-enabled"]; exists {
			enabled = val == "true"
		}

		writeJSON(w, map[string]bool{"enabled": enabled})
	})

	router.Put("/api/v1/diagnostic/write-enabled", func(w http.ResponseWriter, r *http.Request) {
		if opts.Client == nil {
			http.Error(w, "kubernetes client not configured", http.StatusServiceUnavailable)
			return
		}

		var req map[string]bool
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		enabled, exists := req["enabled"]
		if !exists {
			http.Error(w, "enabled field is required", http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		configMapName := "glooscap-config"
		namespace := "glooscap-system"

		var cm corev1.ConfigMap
		// Use APIReader (uncached client) for reads to avoid requiring cluster-wide ConfigMap watch permissions
		// But use cached client for writes (Create/Update) as those don't trigger cache watches
		reader := opts.APIReader
		if reader == nil {
			reader = opts.Client
		}
		err := reader.Get(ctx, client.ObjectKey{Namespace: namespace, Name: configMapName}, &cm)
		if err != nil {
			if errors.IsNotFound(err) {
				// Create new ConfigMap
				cm = corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      configMapName,
						Namespace: namespace,
					},
					Data: map[string]string{
						"diagnostic-write-enabled": fmt.Sprintf("%v", enabled),
					},
				}
				if err := opts.Client.Create(ctx, &cm); err != nil {
					http.Error(w, fmt.Sprintf("failed to create config: %v", err), http.StatusInternalServerError)
					return
				}
			} else {
				http.Error(w, fmt.Sprintf("failed to get config: %v", err), http.StatusInternalServerError)
				return
			}
		} else {
			// Update existing ConfigMap
			if cm.Data == nil {
				cm.Data = make(map[string]string)
			}
			cm.Data["diagnostic-write-enabled"] = fmt.Sprintf("%v", enabled)
			if err := opts.Client.Update(ctx, &cm); err != nil {
				http.Error(w, fmt.Sprintf("failed to update config: %v", err), http.StatusInternalServerError)
				return
			}
		}

		writeJSON(w, map[string]bool{"enabled": enabled})
	})

	// WikiTarget CRUD endpoints (POST, PUT, DELETE)
	router.Post("/api/v1/wikitargets", func(w http.ResponseWriter, r *http.Request) {
		// Add panic recovery
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[http] PANIC in POST /wikitargets: %v\n", r)
				http.Error(w, fmt.Sprintf("internal server error: %v", r), http.StatusInternalServerError)
			}
		}()

		// Log immediately - this should always appear if request reaches handler
		fmt.Fprintf(os.Stderr, "[http] POST /api/v1/wikitargets received - Method: %s, URL: %s, Content-Type: %s\n", 
			r.Method, r.URL.String(), r.Header.Get("Content-Type"))
		fmt.Printf("[http] POST /api/v1/wikitargets received\n")
		if opts.Client == nil {
			fmt.Printf("[http] ERROR: kubernetes client not configured\n")
			http.Error(w, "kubernetes client not configured", http.StatusServiceUnavailable)
			return
		}

		// Decode request - UI sends {metadata: {name, namespace}, spec: {...}, secretToken: "..."}
		// First decode into a map to extract secretToken separately
		var requestData map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
			fmt.Printf("[http] ERROR: Failed to decode WikiTarget request: %v\n", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		fmt.Printf("[http] Decoded request data, has secretToken: %v, has metadata: %v, has spec: %v\n",
			requestData["secretToken"] != nil, requestData["metadata"] != nil, requestData["spec"] != nil)

		// Extract secretToken if provided
		var secretToken string
		if tokenVal, ok := requestData["secretToken"].(string); ok {
			secretToken = tokenVal
			fmt.Printf("[http] Extracted secretToken (length: %d)\n", len(secretToken))
		}
		// Remove secretToken from requestData before decoding into WikiTarget
		delete(requestData, "secretToken")

		// Decode the rest into WikiTarget (metadata and spec should be preserved)
		targetBytes, marshalErr := json.Marshal(requestData)
		if marshalErr != nil {
			fmt.Printf("[http] ERROR: Failed to marshal request data: %v\n", marshalErr)
			http.Error(w, fmt.Sprintf("failed to process request: %v", marshalErr), http.StatusBadRequest)
			return
		}
		previewLen := 200
		if len(targetBytes) < previewLen {
			previewLen = len(targetBytes)
		}
		fmt.Printf("[http] Marshaled request (length: %d): %s\n", len(targetBytes), string(targetBytes)[:previewLen])

		var target wikiv1alpha1.WikiTarget
		if err := json.Unmarshal(targetBytes, &target); err != nil {
			fmt.Printf("[http] ERROR: Failed to decode WikiTarget from request: %v\n", err)
			http.Error(w, fmt.Sprintf("failed to decode WikiTarget: %v", err), http.StatusBadRequest)
			return
		}
		fmt.Printf("[http] Decoded WikiTarget: name=%q, namespace=%q, uri=%q, secretName=%q\n",
			target.Name, target.Namespace, target.Spec.URI, target.Spec.ServiceAccountSecretRef.Name)

		// Set default namespace if not provided
		if target.Namespace == "" {
			target.Namespace = "glooscap-system"
		}

		// Validate required fields
		if target.Name == "" {
			http.Error(w, "metadata.name or name is required", http.StatusBadRequest)
			return
		}

		// Normalize name to RFC 1123 compliant format (lowercase, alphanumeric, dashes)
		normalizedName := normalizeRFC1123Name(target.Name)
		if normalizedName != target.Name {
			fmt.Printf("[http] Normalized WikiTarget name from %q to %q (RFC 1123 compliance)\n", target.Name, normalizedName)
			target.Name = normalizedName
		}
		if target.Spec.URI == "" {
			http.Error(w, "spec.uri is required", http.StatusBadRequest)
			return
		}
		if target.Spec.ServiceAccountSecretRef.Name == "" {
			http.Error(w, "spec.serviceAccountSecretRef.name is required", http.StatusBadRequest)
			return
		}
		if target.Spec.Mode == "" {
			http.Error(w, "spec.mode is required", http.StatusBadRequest)
			return
		}

		// Set default key if not provided
		if target.Spec.ServiceAccountSecretRef.Key == "" {
			target.Spec.ServiceAccountSecretRef.Key = "token"
		}

		// Set default InsecureSkipTLSVerify to true (for now, to handle self-signed certs)
		// Check if the request explicitly set this field
		_, hasInsecureSkipTLSVerify := getNestedBool(requestData, "spec", "insecureSkipTLSVerify")
		if !hasInsecureSkipTLSVerify {
			// Not explicitly set, default to true
			target.Spec.InsecureSkipTLSVerify = true
			fmt.Printf("[http] Setting InsecureSkipTLSVerify=true by default for WikiTarget '%s/%s'\n", target.Namespace, target.Name)
		}

		ctx := r.Context()
		fmt.Printf("[http] POST /wikitargets: Creating/updating WikiTarget '%s/%s' with URI=%s, secret=%s, mode=%s\n",
			target.Namespace, target.Name, target.Spec.URI, target.Spec.ServiceAccountSecretRef.Name, target.Spec.Mode)

		// Create or update the Secret if token is provided
		if secretToken != "" {
			secretKey := target.Spec.ServiceAccountSecretRef.Key
			if secretKey == "" {
				secretKey = "token"
			}
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      target.Spec.ServiceAccountSecretRef.Name,
					Namespace: target.Namespace,
				},
				Type: corev1.SecretTypeOpaque,
				StringData: map[string]string{
					secretKey: secretToken,
				},
			}

			// Check if secret exists
			var existingSecret corev1.Secret
			err := opts.Client.Get(ctx, client.ObjectKey{Namespace: target.Namespace, Name: secret.Name}, &existingSecret)
			if err != nil {
				if errors.IsNotFound(err) {
					// Create new secret
					fmt.Printf("[http] Creating Secret '%s/%s' for WikiTarget\n", target.Namespace, secret.Name)
					if err := opts.Client.Create(ctx, secret); err != nil {
						fmt.Printf("[http] ERROR: Failed to create Secret '%s/%s': %v\n", target.Namespace, secret.Name, err)
						http.Error(w, fmt.Sprintf("failed to create Secret: %v", err), http.StatusInternalServerError)
						return
					}
					fmt.Printf("[http] Successfully created Secret: %s/%s\n", target.Namespace, secret.Name)
				} else {
					fmt.Printf("[http] ERROR: Failed to get Secret '%s/%s': %v\n", target.Namespace, secret.Name, err)
					http.Error(w, fmt.Sprintf("failed to get Secret: %v", err), http.StatusInternalServerError)
					return
				}
			} else {
				// Update existing secret
				fmt.Printf("[http] Updating Secret '%s/%s' for WikiTarget\n", target.Namespace, secret.Name)
				// Update the secret data
				if existingSecret.Data == nil {
					existingSecret.Data = make(map[string][]byte)
				}
				existingSecret.Data[secretKey] = []byte(secretToken)
				if err := opts.Client.Update(ctx, &existingSecret); err != nil {
					fmt.Printf("[http] ERROR: Failed to update Secret '%s/%s': %v\n", target.Namespace, secret.Name, err)
					http.Error(w, fmt.Sprintf("failed to update Secret: %v", err), http.StatusInternalServerError)
					return
				}
				fmt.Printf("[http] Successfully updated Secret: %s/%s\n", target.Namespace, secret.Name)
			}
		}

		// Get existing WikiTarget (if any)
		var existing wikiv1alpha1.WikiTarget
		err := opts.Client.Get(ctx, client.ObjectKey{Namespace: target.Namespace, Name: target.Name}, &existing)
		if err != nil {
			if errors.IsNotFound(err) {
				// Create new WikiTarget
				fmt.Printf("[http] WikiTarget '%s/%s' not found, creating new one\n", target.Namespace, target.Name)
				if err := opts.Client.Create(ctx, &target); err != nil {
					fmt.Printf("[http] ERROR: Failed to create WikiTarget '%s/%s': %v (error type: %T)\n", target.Namespace, target.Name, err, err)
					http.Error(w, fmt.Sprintf("failed to create WikiTarget: %v", err), http.StatusInternalServerError)
					return
				}
				fmt.Printf("[http] Successfully created WikiTarget: %s/%s\n", target.Namespace, target.Name)
			} else {
				fmt.Printf("[http] ERROR: Failed to get WikiTarget '%s/%s' (non-NotFound): %v (error type: %T)\n", target.Namespace, target.Name, err, err)
				http.Error(w, fmt.Sprintf("failed to get WikiTarget: %v", err), http.StatusInternalServerError)
				return
			}
		} else {
			// Update existing WikiTarget
			fmt.Printf("[http] WikiTarget '%s/%s' exists, updating\n", target.Namespace, target.Name)
			existing.Spec = target.Spec
			if err := opts.Client.Update(ctx, &existing); err != nil {
				fmt.Printf("[http] ERROR: Failed to update WikiTarget '%s/%s': %v (error type: %T)\n", target.Namespace, target.Name, err, err)
				http.Error(w, fmt.Sprintf("failed to update WikiTarget: %v", err), http.StatusInternalServerError)
				return
			}
			fmt.Printf("[http] Successfully updated WikiTarget: %s/%s\n", target.Namespace, target.Name)
		}

		writeJSON(w, map[string]string{"name": target.Name, "namespace": target.Namespace})
	})

	router.Put("/api/v1/wikitargets/{namespace}/{name}", func(w http.ResponseWriter, r *http.Request) {
		if opts.Client == nil {
			http.Error(w, "kubernetes client not configured", http.StatusServiceUnavailable)
			return
		}

		namespace := chi.URLParam(r, "namespace")
		name := chi.URLParam(r, "name")
		if namespace == "" || name == "" {
			http.Error(w, "namespace and name are required", http.StatusBadRequest)
			return
		}

		var target wikiv1alpha1.WikiTarget
		if err := json.NewDecoder(r.Body).Decode(&target); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Ensure name and namespace match URL params
		target.Name = name
		target.Namespace = namespace

		// Get existing target to preserve metadata
		var existing wikiv1alpha1.WikiTarget
		if err := opts.Client.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: name}, &existing); err != nil {
			if errors.IsNotFound(err) {
				http.Error(w, "WikiTarget not found", http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Preserve resource version for optimistic concurrency
		target.ResourceVersion = existing.ResourceVersion

		if err := opts.Client.Update(r.Context(), &target); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]string{"name": target.Name, "namespace": target.Namespace})
	})

	router.Delete("/api/v1/wikitargets/{namespace}/{name}", func(w http.ResponseWriter, r *http.Request) {
		if opts.Client == nil {
			http.Error(w, "kubernetes client not configured", http.StatusServiceUnavailable)
			return
		}

		namespace := chi.URLParam(r, "namespace")
		name := chi.URLParam(r, "name")
		if namespace == "" || name == "" {
			http.Error(w, "namespace and name are required", http.StatusBadRequest)
			return
		}

		var target wikiv1alpha1.WikiTarget
		target.Name = name
		target.Namespace = namespace

		if err := opts.Client.Delete(r.Context(), &target); err != nil {
			if errors.IsNotFound(err) {
				http.Error(w, "WikiTarget not found", http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]string{"status": "deleted", "name": name, "namespace": namespace})
	})

	// POST endpoint to trigger a WikiTarget refresh by adding a force-refresh annotation
	router.Post("/api/v1/wikitargets/{namespace}/{name}/refresh", func(w http.ResponseWriter, r *http.Request) {
		if opts.Client == nil {
			http.Error(w, "kubernetes client not configured", http.StatusServiceUnavailable)
			return
		}

		namespace := chi.URLParam(r, "namespace")
		name := chi.URLParam(r, "name")
		if namespace == "" || name == "" {
			http.Error(w, "namespace and name are required", http.StatusBadRequest)
			return
		}

		var target wikiv1alpha1.WikiTarget
		if err := opts.Client.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: name}, &target); err != nil {
			if errors.IsNotFound(err) {
				http.Error(w, "WikiTarget not found", http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Add annotation to force refresh - controller will see this and immediately refresh
		if target.Annotations == nil {
			target.Annotations = make(map[string]string)
		}
		target.Annotations["glooscap.dasmlab.org/force-refresh"] = metav1.Now().Format(time.RFC3339)

		// Clear LastSyncTime to force immediate refresh
		target.Status.LastSyncTime = nil

		if err := opts.Client.Status().Update(r.Context(), &target); err != nil {
			http.Error(w, fmt.Sprintf("failed to update WikiTarget status: %v", err), http.StatusInternalServerError)
			return
		}

		// Also update the annotations
		if err := opts.Client.Update(r.Context(), &target); err != nil {
			http.Error(w, fmt.Sprintf("failed to update WikiTarget: %v", err), http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]string{"status": "refresh triggered", "name": name, "namespace": namespace})
	})

	server := &http.Server{
		Addr:              opts.Addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		_ = server.Shutdown(context.Background())
		return nil
	case err := <-errCh:
		return err
	}
}

type createJobRequest struct {
	Namespace   string `json:"namespace"`
	TargetRef   string `json:"targetRef"`
	PageID      string `json:"pageId"`
	LanguageTag string `json:"languageTag"`
	Pipeline    string `json:"pipeline"`
	PageTitle   string `json:"pageTitle"`
}

// normalizeRFC1123Name normalizes a string to be RFC 1123 compliant:
// - lowercase alphanumeric characters, '-' or '.'
// - must start and end with an alphanumeric character
func normalizeRFC1123Name(name string) string {
	// Convert to lowercase
	normalized := strings.ToLower(name)
	
	// Replace invalid characters with dashes
	var result strings.Builder
	for i, r := range normalized {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
		} else if r == '-' || r == '.' {
			// Only allow dash/dot if not at start/end
			if i > 0 && i < len(normalized)-1 {
				result.WriteRune(r)
			}
		} else {
			// Replace other characters with dash
			if i > 0 && i < len(normalized)-1 {
				result.WriteRune('-')
			}
		}
	}
	
	normalized = result.String()
	
	// Remove leading/trailing dashes and dots
	normalized = strings.Trim(normalized, "-.")
	
	// Ensure it starts and ends with alphanumeric
	if len(normalized) > 0 {
		first := normalized[0]
		last := normalized[len(normalized)-1]
		if !((first >= 'a' && first <= 'z') || (first >= '0' && first <= '9')) {
			normalized = "a" + normalized
		}
		if !((last >= 'a' && last <= 'z') || (last >= '0' && last <= '9')) {
			normalized = normalized + "a"
		}
	} else {
		normalized = "wikitarget"
	}
	
	return normalized
}

// getNestedBool safely extracts a boolean value from nested map structure
func getNestedBool(data map[string]interface{}, keys ...string) (bool, bool) {
	current := data
	for i, key := range keys {
		if i == len(keys)-1 {
			// Last key - return the bool value
			if val, ok := current[key]; ok {
				if boolVal, ok := val.(bool); ok {
					return boolVal, true
				}
			}
			return false, false
		}
		// Navigate deeper
		if val, ok := current[key]; ok {
			if nestedMap, ok := val.(map[string]interface{}); ok {
				current = nestedMap
			} else {
				return false, false
			}
		} else {
			return false, false
		}
	}
	return false, false
}

func (r *createJobRequest) validate() error {
	if r.Namespace == "" {
		r.Namespace = "glooscap-system"
	}
	if r.TargetRef == "" {
		return fmt.Errorf("targetRef is required")
	}
	if r.PageID == "" {
		return fmt.Errorf("pageId is required")
	}
	if r.LanguageTag == "" {
		r.LanguageTag = "fr-CA"
	}
	if r.Pipeline == "" {
		r.Pipeline = string(wikiv1alpha1.TranslationPipelineModeTektonJob)
	}
	return nil
}

// buildStateResponse constructs the full state response with WikiTargets, pages, and nanabush status.
func buildStateResponse(opts Options) map[string]any {
	result := map[string]any{
		"wikitargets": []map[string]any{},
	}

	// Get client status first (most up-to-date)
	var nanabushClient *nanabush.Client
	if opts.GetNanabushClient != nil {
		nanabushClient = opts.GetNanabushClient()
	} else if opts.Nanabush != nil {
		nanabushClient = opts.Nanabush
	}

	var clientStatus nanabush.Status
	if nanabushClient != nil {
		clientStatus = nanabushClient.Status()
	} else {
		clientStatus = nanabush.Status{
			Connected:  false,
			Registered: false,
			Status:     "error",
		}
	}

	// Try to read status from TranslationService CR
	// Prefer client status if it shows connected/registered but CR doesn't (handles startup race condition)
	var nanabushStatus map[string]any
	if opts.Client != nil {
		tsName := "glooscap-translation-service"
		var ts wikiv1alpha1.TranslationService
		ctx := context.Background() // Use background context for SSE
		err := opts.Client.Get(ctx, client.ObjectKey{Name: tsName}, &ts)
		if err == nil {
			// CR exists - check if status is populated
			if ts.Status.ClientID != "" || ts.Status.Status != "" {
				// CR status is populated - but prefer client status if it's more accurate
				// This handles the case where client is connected but CR hasn't been updated yet
				if clientStatus.Connected && clientStatus.Registered && (!ts.Status.Connected || !ts.Status.Registered) {
					// Client is connected but CR shows disconnected - prefer client status (more recent)
					var lastHeartbeatStr string
					if !clientStatus.LastHeartbeat.IsZero() {
						lastHeartbeatStr = clientStatus.LastHeartbeat.Format(time.RFC3339)
					}
					nanabushStatus = map[string]any{
						"connected":                clientStatus.Connected,
						"registered":               clientStatus.Registered,
						"clientId":                 clientStatus.ClientID,
						"lastHeartbeat":            lastHeartbeatStr,
						"missedHeartbeats":         clientStatus.MissedHeartbeats,
						"heartbeatIntervalSeconds": clientStatus.HeartbeatInterval,
						"status":                   clientStatus.Status,
					}
				} else {
					// CR status is populated and matches client, or client is not connected - use CR status
					var lastHeartbeatStr string
					if ts.Status.LastHeartbeat != nil {
						lastHeartbeatStr = ts.Status.LastHeartbeat.Format(time.RFC3339)
					}
					nanabushStatus = map[string]any{
						"connected":                ts.Status.Connected,
						"registered":               ts.Status.Registered,
						"clientId":                 ts.Status.ClientID,
						"lastHeartbeat":            lastHeartbeatStr,
						"missedHeartbeats":         ts.Status.MissedHeartbeats,
						"heartbeatIntervalSeconds": ts.Status.HeartbeatIntervalSeconds,
						"status":                   ts.Status.Status,
					}
				}
			}
		}
	}

	// Fallback to client status if CR doesn't exist or doesn't have status populated yet
	if nanabushStatus == nil {
		// Only return error status if we have a client but it's not registered after reasonable time
		// If clientId is empty but we just created the client, return "connecting" status
		if clientStatus.ClientID == "" && clientStatus.Status != "error" {
			// Client is still registering - return connecting status
			nanabushStatus = map[string]any{
				"connected":  false,
				"registered": false,
				"clientId":   "",
				"status":     "connecting",
			}
		} else {
			var lastHeartbeatStr string
			if !clientStatus.LastHeartbeat.IsZero() {
				lastHeartbeatStr = clientStatus.LastHeartbeat.Format(time.RFC3339)
			}
			nanabushStatus = map[string]any{
				"connected":                clientStatus.Connected,
				"registered":               clientStatus.Registered,
				"clientId":                 clientStatus.ClientID,
				"lastHeartbeat":            lastHeartbeatStr,
				"missedHeartbeats":         clientStatus.MissedHeartbeats,
				"heartbeatIntervalSeconds": clientStatus.HeartbeatInterval, // Already int64 in seconds
				"status":                   clientStatus.Status,
			}
		}
	}

	result["nanabush"] = nanabushStatus

	if opts.Catalogue == nil {
		return result
	}

	// Get all targets
	targets := opts.Catalogue.Targets()
	wikitargets := make([]map[string]any, 0, len(targets))

	for _, target := range targets {
		// Get pages for this target
		pages := opts.Catalogue.List(target.ID)
		pageList := make([]map[string]any, 0, len(pages))

		for _, page := range pages {
			pageList = append(pageList, map[string]any{
				"name":           page.Title,
				"slug":           page.Slug,
				"uri":            page.URI,
				"id":             page.ID,
				"state":          page.State,
				"lastChecked":    page.LastChecked.Format(time.RFC3339),
				"updatedAt":      page.UpdatedAt.Format(time.RFC3339),
				"autoTranslated": page.AutoTranslated,
				"translationURI": page.TranslationURI,
				"language":       page.Language,
				"hasAssets":      page.HasAssets,
				"collection":     page.Collection,
				"template":       page.Template,
				"isTemplate":     page.IsTemplate,
			})
		}

		wikitargets = append(wikitargets, map[string]any{
			"wikitarget": target.URI,
			"targetId":   target.ID,
			"name":       target.Name,
			"namespace":  target.Namespace,
			"mode":       target.Mode,
			"pages":      pageList,
		})
	}

	result["wikitargets"] = wikitargets

	// Add translation jobs to SSE response
	translationJobs := []map[string]any{}
	if opts.Client != nil {
		ctx := context.Background()
		var jobList wikiv1alpha1.TranslationJobList
		// List all TranslationJobs in glooscap-system namespace
		if err := opts.Client.List(ctx, &jobList, client.InNamespace("glooscap-system")); err == nil {
			for _, job := range jobList.Items {
				// Build source page URI if we have the page info
				sourceURI := ""
				sourcePageTitle := job.Spec.Parameters["pageTitle"]
				if opts.Catalogue != nil {
					// Try to find the page in the catalog to get its URI
					targetID := fmt.Sprintf("glooscap-system/%s", job.Spec.Source.TargetRef)
					pages := opts.Catalogue.List(targetID)
					for _, page := range pages {
						if page.ID == job.Spec.Source.PageID {
							sourceURI = page.URI
							if sourcePageTitle == "" {
								sourcePageTitle = page.Title
							}
							break
						}
					}
				}

				// Build destination info
				destTargetRef := job.Spec.Source.TargetRef
				if job.Spec.Destination != nil && job.Spec.Destination.TargetRef != "" {
					destTargetRef = job.Spec.Destination.TargetRef
				}
				languageTag := "fr-CA" // Default
				if job.Spec.Destination != nil && job.Spec.Destination.LanguageTag != "" {
					languageTag = job.Spec.Destination.LanguageTag
				}

				// Format timestamps
				var startedAt, finishedAt string
				if job.Status.StartedAt != nil {
					startedAt = job.Status.StartedAt.Format(time.RFC3339)
				}
				if job.Status.FinishedAt != nil {
					finishedAt = job.Status.FinishedAt.Format(time.RFC3339)
				}

				jobData := map[string]any{
					"uuid":       string(job.UID), // Use UID as UUID for UI hooking
					"name":       job.Name,
					"namespace":  job.Namespace,
					"state":      string(job.Status.State),
					"message":    job.Status.Message,
					"startedAt":  startedAt,
					"finishedAt": finishedAt,
					"source": map[string]any{
						"targetRef": job.Spec.Source.TargetRef,
						"pageId":    job.Spec.Source.PageID,
						"pageTitle": sourcePageTitle,
						"pageURI":   sourceURI, // Source page URI for UI hooking
					},
					"destination": map[string]any{
						"targetRef":   destTargetRef,
						"languageTag": languageTag,
					},
					"pipeline":     string(job.Spec.Pipeline),
					"isDiagnostic": job.Labels["glooscap.dasmlab.org/diagnostic"] == "true",
				}

				// Add translated page info if completed
				if job.Status.State == wikiv1alpha1.TranslationJobStateCompleted {
					// Get published page info from annotations
					var publishedPageID, publishedPageSlug, publishedPageURL string
					var isDraft bool = true

					if job.Annotations != nil {
						if pageID, ok := job.Annotations["glooscap.dasmlab.org/published-page-id"]; ok {
							publishedPageID = pageID
						}
						if pageSlug, ok := job.Annotations["glooscap.dasmlab.org/published-page-slug"]; ok {
							publishedPageSlug = pageSlug
						}
						if pageURL, ok := job.Annotations["glooscap.dasmlab.org/published-page-url"]; ok {
							publishedPageURL = pageURL
						}
						if draftFlag, ok := job.Annotations["glooscap.dasmlab.org/is-draft"]; ok {
							isDraft = (draftFlag == "true")
						}
					}

					jobData["translatedPage"] = map[string]any{
						"pageId":  publishedPageID,
						"slug":    publishedPageSlug,
						"url":     publishedPageURL,
						"isDraft": isDraft,
					}
				}

				translationJobs = append(translationJobs, jobData)
			}
		}
	} else if opts.Jobs != nil {
		// Fallback to JobStore if client not available
		jobs := opts.Jobs.List()
		for name, job := range jobs {
			jobData := map[string]any{
				"name":      name,
				"state":     string(job.Status.State),
				"message":   job.Status.Message,
				"pipeline":  job.Pipeline,
				"targetRef": job.TargetRef,
				"pageId":    job.PageID,
				"pageTitle": job.PageTitle,
			}
			translationJobs = append(translationJobs, jobData)
		}
	}

	result["translationJobs"] = translationJobs
	return result
}

// sendStateEvent builds and broadcasts the current state.
func sendStateEvent(broadcaster *eventBroadcaster, opts Options) {
	state := buildStateResponse(opts)
	if data, err := json.Marshal(state); err == nil {
		broadcaster.broadcast(data)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
