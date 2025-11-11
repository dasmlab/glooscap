package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

// Start launches the API server and blocks until the context is cancelled.
func Start(ctx context.Context, opts Options) error {
	if opts.Addr == "" {
		opts.Addr = ":3000"
	}

	router := chi.NewRouter()
	router.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	router.Get("/api/v1/catalogue", func(w http.ResponseWriter, r *http.Request) {
		target := r.URL.Query().Get("target")
		var pages []catalog.Page
		if opts.Catalogue != nil {
			pages = opts.Catalogue.List(target)
		}
		writeJSON(w, pages)
	})

	router.Get("/api/v1/targets", func(w http.ResponseWriter, _ *http.Request) {
		var targets []catalog.Target
		if opts.Catalogue != nil {
			targets = opts.Catalogue.Targets()
		}
		writeJSON(w, targets)
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

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
