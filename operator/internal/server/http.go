package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	wikiv1alpha1 "github.com/dasmlab/glooscap-operator/api/v1alpha1"
	"github.com/dasmlab/glooscap-operator/pkg/catalog"
)

// Options controls the API server.
type Options struct {
	Addr      string
	Catalogue *catalog.Store
	Jobs      *catalog.JobStore
	Client    client.Client
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
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
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
		w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering
		
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

// buildStateResponse constructs the full state response with WikiTargets and pages.
func buildStateResponse(opts Options) map[string]any {
	result := map[string]any{
		"wikitargets": []map[string]any{},
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
