/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	wikiv1alpha1 "github.com/dasmlab/glooscap-operator/api/v1alpha1"
	"github.com/dasmlab/glooscap-operator/pkg/catalog"
	"github.com/dasmlab/glooscap-operator/pkg/outline"
)

const (
	// DefaultRefreshInterval is the default time between catalog refreshes
	DefaultRefreshInterval = 15 * time.Second
)

// WikiTargetReconciler reconciles a WikiTarget object
type WikiTargetReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Recorder record.EventRecorder

	Catalogue     *catalog.Store
	OutlineClient OutlineClientFactory
}

// +kubebuilder:rbac:groups=wiki.glooscap.dasmlab.org,resources=wikitargets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=wiki.glooscap.dasmlab.org,resources=wikitargets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=wiki.glooscap.dasmlab.org,resources=wikitargets/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the WikiTarget object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *WikiTargetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("wikitarget", req.NamespacedName)

	var target wikiv1alpha1.WikiTarget
	if err := r.Get(ctx, req.NamespacedName, &target); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	status := target.Status.DeepCopy()
	now := metav1.Now()

	// Ensure InsecureSkipTLSVerify is set to true by default (for now, to handle self-signed certs)
	// Update if it's false (default for bool is false, so this catches unset values)
	if !target.Spec.InsecureSkipTLSVerify {
		logger.Info("Setting InsecureSkipTLSVerify=true for WikiTarget (default for self-signed certs)")
		target.Spec.InsecureSkipTLSVerify = true
		if err := r.Update(ctx, &target); err != nil {
			logger.Error(err, "failed to update WikiTarget with InsecureSkipTLSVerify=true")
			// Continue anyway - will try again next reconcile
		} else {
			logger.Info("Updated WikiTarget with InsecureSkipTLSVerify=true")
			// Re-fetch to get updated version
			if err := r.Get(ctx, req.NamespacedName, &target); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// Handle paused state
	if target.Spec.IsPaused {
		status.Paused = true
		meta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "Paused",
			Message:            "WikiTarget reconciliation is paused",
			LastTransitionTime: now,
		})
		logger.Info("WikiTarget is paused, skipping reconciliation")

		if !statusChanged(&target.Status, status) {
			return ctrl.Result{RequeueAfter: DefaultRefreshInterval}, nil
		}
		target.Status = *status
		if err := r.Status().Update(ctx, &target); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: DefaultRefreshInterval}, nil
	}
	status.Paused = false

	// Check if we should refresh (either first time, or Ready for more than 15 seconds, or force-refresh annotation)
	shouldRefresh := false
	refreshReason := ""

	// Check for force-refresh annotation
	if target.Annotations != nil {
		if _, hasForceRefresh := target.Annotations["glooscap.dasmlab.org/force-refresh"]; hasForceRefresh {
			shouldRefresh = true
			refreshReason = "force refresh requested"
			// Remove the annotation after processing
			delete(target.Annotations, "glooscap.dasmlab.org/force-refresh")
			if err := r.Update(ctx, &target); err != nil {
				logger.Error(err, "failed to remove force-refresh annotation")
				return ctrl.Result{}, err
			}
			logger.Info("force-refresh annotation processed and removed")

		}
	}

	if !shouldRefresh {
		if !status.Ready || status.LastSyncTime == nil {
			// First discovery - always refresh
			shouldRefresh = true
			refreshReason = "initial discovery"
		} else if status.Ready {
			// Check if we've been ready for more than 15 seconds
			timeSinceLastSync := now.Time.Sub(status.LastSyncTime.Time)
			if timeSinceLastSync >= DefaultRefreshInterval {
				shouldRefresh = true
				refreshReason = "periodic refresh"
			}
		}
	}

	if !shouldRefresh {
		// Not time to refresh yet, requeue for the remaining time
		timeSinceLastSync := now.Time.Sub(status.LastSyncTime.Time)
		requeueAfter := DefaultRefreshInterval - timeSinceLastSync
		if requeueAfter < time.Second {
			requeueAfter = time.Second
		}
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}

	// Set status to "Refreshing Catalog" if we were previously Ready
	if status.Ready {
		meta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "RefreshingCatalog",
			Message:            "Refreshing catalog",
			LastTransitionTime: now,
		})
	} else {
		meta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "DiscoveryPending",
			Message:            "Discovery worker initialising",
			LastTransitionTime: now,
		})
	}

	if status.CatalogRevision == 0 {
		status.CatalogRevision = 1
	}

	logger.Info("refreshing catalogue", "reason", refreshReason)

	if err := r.refreshCatalogue(ctx, &target, status); err != nil {
		logger.Error(err, "failed to refresh catalogue", "uri", target.Spec.URI)
		status.Ready = false
		meta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "DiscoveryFailed",
			Message:            err.Error(),
			LastTransitionTime: now,
		})
	} else {
		status.Ready = true
		status.LastSyncTime = &now
		logger.Info("successfully refreshed catalogue", "uri", target.Spec.URI, "pages", status.CatalogRevision)
	}

	if !statusChanged(&target.Status, status) {
		return ctrl.Result{RequeueAfter: DefaultRefreshInterval}, nil
	}

	target.Status = *status
	if err := r.Status().Update(ctx, &target); err != nil {
		return ctrl.Result{}, err
	}

	r.Recorder.Event(&target, "Normal", "DiscoverySync", "WikiTarget discovery refreshed")
	logger.Info("refreshed WikiTarget status")

	return ctrl.Result{RequeueAfter: DefaultRefreshInterval}, nil
}

func (r *WikiTargetReconciler) refreshCatalogue(ctx context.Context, target *wikiv1alpha1.WikiTarget, status *wikiv1alpha1.WikiTargetStatus) error {
	logger := log.FromContext(ctx).WithValues("wikitarget", fmt.Sprintf("%s/%s", target.Namespace, target.Name))

	if r.OutlineClient == nil {
		return fmt.Errorf("outline client factory not configured")
	}

	logger.Info("creating outline client", "uri", target.Spec.URI)
	client, err := r.OutlineClient.New(ctx, r.Client, target)
	if err != nil {
		logger.Error(err, "failed to create outline client")
		return fmt.Errorf("create outline client: %w", err)
	}

	logger.Info("fetching pages from outline", "uri", target.Spec.URI, "InsecureSkipTLSVerify", target.Spec.InsecureSkipTLSVerify)
	
	// MVP: Prefer "Maurice PDG Collection" if it exists, otherwise get all pages
	var pages []outline.PageSummary
	var collectionID string
	
	// First, try to find "Maurice PDG Collection"
	collections, collErr := client.ListCollections(ctx)
	if collErr == nil {
		for _, coll := range collections {
			if coll.Name == "Maurice PDG Collection" {
				collectionID = coll.ID
				logger.Info("Found 'Maurice PDG Collection', constraining search to this collection", "collectionID", collectionID)
				break
			}
		}
	} else {
		logger.Info("Failed to list collections, will search all pages", "error", collErr)
	}
	
	// Fetch pages (with collection filter if found)
	if collectionID != "" {
		pages, err = client.ListPages(ctx, collectionID)
	} else {
		logger.Info("'Maurice PDG Collection' not found, fetching all pages")
		pages, err = client.ListPages(ctx)
	}
	if err != nil {
		// Check if this is a TLS certificate error and we haven't enabled skip verification yet
		// Check both the error string and unwrap to check for underlying TLS errors
		errStr := err.Error()
		errStrLower := strings.ToLower(errStr)
		isCertError := strings.Contains(errStrLower, "certificate") || 
			strings.Contains(errStrLower, "x509") || 
			strings.Contains(errStrLower, "unknown authority") ||
			strings.Contains(errStrLower, "certificate signed by unknown") ||
			strings.Contains(errStrLower, "failed to verify certificate") ||
			strings.Contains(errStrLower, "tls:") ||
			strings.Contains(errStrLower, "tls handshake")
		
		logger.Info("ListPages error detected", "error", errStr, "isCertError", isCertError, "InsecureSkipTLSVerify", target.Spec.InsecureSkipTLSVerify)
		
		if isCertError && !target.Spec.InsecureSkipTLSVerify {
			logger.Info("TLS certificate error detected, automatically enabling InsecureSkipTLSVerify and retrying",
				"error", errStr)
			
			// Update the WikiTarget to enable TLS skip verification
			target.Spec.InsecureSkipTLSVerify = true
			if updateErr := r.Client.Update(ctx, &target); updateErr != nil {
				logger.Error(updateErr, "failed to update WikiTarget with InsecureSkipTLSVerify")
				return fmt.Errorf("list pages: %w (failed to enable TLS skip: %v)", err, updateErr)
			}
			
			logger.Info("WikiTarget updated with InsecureSkipTLSVerify=true, creating new client")
			
			// Refresh the target from API server to ensure we have the latest version
			var updatedTarget wikiv1alpha1.WikiTarget
			key := types.NamespacedName{Namespace: target.Namespace, Name: target.Name}
			if refreshErr := r.Client.Get(ctx, key, &updatedTarget); refreshErr != nil {
				logger.Error(refreshErr, "failed to refresh WikiTarget after update")
				// Continue with the updated target we have in memory
				updatedTarget = *target
			} else {
				*target = updatedTarget
			}
			
			// Create a new client with TLS skip enabled
			client, retryErr := r.OutlineClient.New(ctx, r.Client, target)
			if retryErr != nil {
				logger.Error(retryErr, "failed to create outline client with TLS skip")
				return fmt.Errorf("create outline client with TLS skip: %w", retryErr)
			}
			
			logger.Info("Retrying ListPages with TLS skip verification enabled")
			// Retry ListPages
			pages, retryErr = client.ListPages(ctx)
			if retryErr != nil {
				logger.Error(retryErr, "failed to list pages from outline even with TLS skip enabled")
				return fmt.Errorf("list pages (with TLS skip): %w", retryErr)
			}
			
			logger.Info("successfully fetched pages after enabling TLS skip verification", "count", len(pages))
		} else if isCertError && target.Spec.InsecureSkipTLSVerify {
			// Already has TLS skip enabled but still failing - this is unexpected
			logger.Error(err, "TLS certificate error even with InsecureSkipTLSVerify enabled - check client configuration")
			return fmt.Errorf("list pages (TLS skip already enabled): %w", err)
		} else {
			logger.Error(err, "failed to list pages from outline")
			return fmt.Errorf("list pages: %w", err)
		}
	}
	logger.Info("fetched pages from outline", "count", len(pages))

	if r.Catalogue != nil {
		targetID := fmt.Sprintf("%s/%s", target.Namespace, target.Name)
		baseURI := strings.TrimSuffix(target.Spec.URI, "/")
		catalogPages := make([]catalog.Page, 0, len(pages))

		for i, page := range pages {
			// Build full URI for the page
			pageURI := fmt.Sprintf("%s/doc/%s", baseURI, page.Slug)

			// Always log discovered pages with URI
			logger.Info("discovered page",
				"index", i+1,
				"title", page.Title,
				"id", page.ID,
				"slug", page.Slug,
				"uri", pageURI,
				"updatedAt", page.UpdatedAt.Format(time.RFC3339),
			)

			// Default language to EN if not provided by Outline
			language := page.Language
			if language == "" {
				language = "EN"
			}

			catalogPages = append(catalogPages, catalog.Page{
				ID:         page.ID,
				Title:      page.Title,
				Slug:       page.Slug,
				URI:        pageURI,
				UpdatedAt:  page.UpdatedAt,
				Language:   language,
				HasAssets:  page.HasAssets,
				Collection: page.Collection,
				Template:   page.Template,
				IsTemplate: page.IsTemplate,
			})
		}

		r.Catalogue.Update(targetID, catalog.Target{
			ID:        targetID,
			Namespace: target.Namespace,
			Name:      target.Name,
			Mode:      string(target.Spec.Mode),
			URI:       target.Spec.URI,
		}, catalogPages)
	}

	status.CatalogRevision++
	meta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "DiscoverySucceeded",
		Message:            fmt.Sprintf("Discovered %d pages", len(pages)),
		LastTransitionTime: metav1.Now(),
	})
	return nil
}

func statusChanged(oldStatus *wikiv1alpha1.WikiTargetStatus, newStatus *wikiv1alpha1.WikiTargetStatus) bool {
	return !equality.Semantic.DeepEqual(oldStatus, newStatus)
}

// SetupWithManager sets up the controller with the Manager.
func (r *WikiTargetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&wikiv1alpha1.WikiTarget{}).
		Named("wikitarget").
		Complete(r)
}
