package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	wikiv1alpha1 "github.com/dasmlab/glooscap-operator/api/v1alpha1"
	"github.com/dasmlab/glooscap-operator/pkg/catalog"
	"github.com/dasmlab/glooscap-operator/pkg/nanabush"
	"github.com/dasmlab/glooscap-operator/internal/controller"
)

// Options controls the API server.
type Options struct {
	Addr      string
	Catalogue *catalog.Store
	Jobs      *catalog.JobStore
	Client    client.Client
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
			}
		}
	}()

	router := chi.NewRouter()

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
	router.Get("/api/v1/status/nanabush", func(w http.ResponseWriter, _ *http.Request) {
		if opts.Nanabush == nil {
			writeJSON(w, nanabush.Status{
				Connected:  false,
				Registered: false,
				Status:     "error",
			})
			return
		}
		status := opts.Nanabush.Status()
		writeJSON(w, status)
	})

	// Generic translation service status endpoint (alias for backward compatibility)
	router.Get("/api/v1/status/translation", func(w http.ResponseWriter, _ *http.Request) {
		// Use getter function if available (for runtime updates), otherwise use direct reference
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

		translateResp, err := nanabushClient.Translate(ctx, grpcReq)
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
			"success":           true,
			"originalTitle":     pageContent.Title,
			"translatedTitle":   translateResp.TranslatedTitle,
			"translatedMarkdown": translateResp.TranslatedMarkdown,
			"tokensUsed":        translateResp.TokensUsed,
			"inferenceTime":     translateResp.InferenceTimeSeconds,
			"message":           "Translation completed. Page creation coming soon.",
		})
	})

	// Translation Service Configuration CRUD endpoints
	router.Get("/api/v1/translation-service", func(w http.ResponseWriter, r *http.Request) {
		if opts.ConfigStore == nil {
			http.Error(w, "configuration store not available", http.StatusServiceUnavailable)
			return
		}
		config := opts.ConfigStore.GetTranslationServiceConfig()
		if config == nil {
			// Return empty config if not set
			writeJSON(w, TranslationServiceConfig{})
			return
		}
		writeJSON(w, config)
	})

	router.Post("/api/v1/translation-service", func(w http.ResponseWriter, r *http.Request) {
		if opts.ConfigStore == nil {
			http.Error(w, "configuration store not available", http.StatusServiceUnavailable)
			return
		}
		if opts.ReconfigureTranslationService == nil {
			http.Error(w, "translation service reconfiguration not available", http.StatusServiceUnavailable)
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
			// Default to nanabush if not specified
			config.Type = "nanabush"
		}

		// Store configuration first (so UI can read it back immediately)
		opts.ConfigStore.SetTranslationServiceConfig(&config)

		// Reconfigure the translation service client (async - returns immediately)
		// The actual connection/registration happens in the background
		if err := opts.ReconfigureTranslationService(config); err != nil {
			http.Error(w, fmt.Sprintf("failed to initiate translation service reconfiguration: %v", err), http.StatusInternalServerError)
			return
		}

		// Return success immediately - reconfiguration is happening in background
		writeJSON(w, map[string]string{
			"status":  "reconfiguration_initiated",
			"address": config.Address,
			"type":    config.Type,
			"message": "Translation service reconfiguration started. Connection will be established in the background.",
		})
	})

	router.Put("/api/v1/translation-service", func(w http.ResponseWriter, r *http.Request) {
		// PUT is same as POST for this resource - reuse POST handler logic
		if opts.ConfigStore == nil {
			http.Error(w, "configuration store not available", http.StatusServiceUnavailable)
			return
		}
		if opts.ReconfigureTranslationService == nil {
			http.Error(w, "translation service reconfiguration not available", http.StatusServiceUnavailable)
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
			// Default to nanabush if not specified
			config.Type = "nanabush"
		}

		// Store configuration first (so UI can read it back immediately)
		opts.ConfigStore.SetTranslationServiceConfig(&config)

		// Reconfigure the translation service client (async - returns immediately)
		if err := opts.ReconfigureTranslationService(config); err != nil {
			http.Error(w, fmt.Sprintf("failed to initiate translation service reconfiguration: %v", err), http.StatusInternalServerError)
			return
		}

		// Return success immediately - reconfiguration is happening in background
		writeJSON(w, map[string]string{
			"status":  "reconfiguration_initiated",
			"address": config.Address,
			"type":    config.Type,
			"message": "Translation service reconfiguration started. Connection will be established in the background.",
		})
	})

	router.Delete("/api/v1/translation-service", func(w http.ResponseWriter, r *http.Request) {
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

	// WikiTarget CRUD endpoints (POST, PUT, DELETE)
	router.Post("/api/v1/wikitargets", func(w http.ResponseWriter, r *http.Request) {
		if opts.Client == nil {
			http.Error(w, "kubernetes client not configured", http.StatusServiceUnavailable)
			return
		}

		var target wikiv1alpha1.WikiTarget
		if err := json.NewDecoder(r.Body).Decode(&target); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Set default namespace if not provided
		if target.Namespace == "" {
			target.Namespace = "glooscap-system"
		}

		// Validate required fields
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

		if err := opts.Client.Create(r.Context(), &target); err != nil {
			if errors.IsAlreadyExists(err) {
				http.Error(w, "WikiTarget already exists", http.StatusConflict)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
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

	// Add nanabush status if client is available
	// Use getter function if available (for runtime updates), otherwise use direct reference
	var nanabushClient *nanabush.Client
	if opts.GetNanabushClient != nil {
		nanabushClient = opts.GetNanabushClient()
	} else if opts.Nanabush != nil {
		nanabushClient = opts.Nanabush
	}

	if nanabushClient != nil {
		status := nanabushClient.Status()
		result["nanabush"] = map[string]any{
			"connected":                status.Connected,
			"registered":               status.Registered,
			"clientId":                 status.ClientID,
			"lastHeartbeat":            status.LastHeartbeat,
			"missedHeartbeats":         status.MissedHeartbeats,
			"heartbeatIntervalSeconds": status.HeartbeatInterval,
			"status":                   status.Status,
		}
	} else {
		// No nanabush client configured
		result["nanabush"] = map[string]any{
			"connected":  false,
			"registered": false,
			"status":     "error",
		}
	}

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
