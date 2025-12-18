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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	manager "sigs.k8s.io/controller-runtime/pkg/manager"

	wikiv1alpha1 "github.com/dasmlab/glooscap-operator/api/v1alpha1"
	"github.com/dasmlab/glooscap-operator/pkg/outline"
)

// WikiTargetDiagnosticRunnable tests write access to WikiTargets in readWrite mode.
// This runs independently and creates/updates a "GLOODIAG TEST" page in drafts
// to verify that we can always write to the target wiki.
type WikiTargetDiagnosticRunnable struct {
	Client        client.Client
	OutlineClient OutlineClientFactory
	// Track master keys and last page IDs per target (in-memory cache)
	masterKeys   map[string]string // target name -> master key (e.g., "GLOODIAG TEST abc123")
	lastPageIDs  map[string]string // target name -> last page ID
	keysMu       sync.RWMutex
}

const (
	// Diagnostic page title prefix
	diagnosticPageTitlePrefix = "GLOODIAG TEST"
	// How often to run the diagnostic (every 30 seconds)
	diagnosticInterval = 30 * time.Second
	// Annotation keys for storing master key and last page ID
	annotationMasterKey  = "glooscap.dasmlab.org/diagnostic-master-key"
	annotationLastPageID = "glooscap.dasmlab.org/diagnostic-last-page-id"
)

// Start implements manager.Runnable
func (r *WikiTargetDiagnosticRunnable) Start(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("wikitarget-diagnostic")

	logger.Info("starting WikiTarget write diagnostic (runs every 30 seconds)")

	// Initialize maps if not already initialized
	if r.masterKeys == nil {
		r.masterKeys = make(map[string]string)
	}
	if r.lastPageIDs == nil {
		r.lastPageIDs = make(map[string]string)
	}

	// Run initial diagnostic immediately (this will also clean up old pages on startup)
	r.runDiagnostic(ctx, logger)

	// Then run every 30 seconds
	ticker := time.NewTicker(diagnosticInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			r.runDiagnostic(ctx, logger)
		}
	}
}

// isDiagnosticEnabled checks if write diagnostic is enabled via ConfigMap
func (r *WikiTargetDiagnosticRunnable) isDiagnosticEnabled(ctx context.Context, logger logr.Logger) bool {
	configMapName := "glooscap-config"
	namespace := "glooscap-system"

	var cm corev1.ConfigMap
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: configMapName}, &cm)
	if err != nil {
		if errors.IsNotFound(err) {
			// ConfigMap doesn't exist, default to enabled
			return true
		}
		logger.V(1).Info("failed to get config map, defaulting to enabled", "error", err)
		return true // Default to enabled on error
	}

	// Check the diagnostic-write-enabled key
	if val, exists := cm.Data["diagnostic-write-enabled"]; exists {
		return val == "true"
	}

	// Key doesn't exist, default to enabled
	return true
}

// runDiagnostic checks all readWrite WikiTargets and creates/updates diagnostic pages
func (r *WikiTargetDiagnosticRunnable) runDiagnostic(ctx context.Context, logger logr.Logger) {
	// Failures are ok - just log and continue
	defer func() {
		if r := recover(); r != nil {
			logger.Error(fmt.Errorf("panic in runDiagnostic: %v", r), "diagnostic panicked, continuing")
		}
	}()

	// Check if diagnostic is enabled
	if !r.isDiagnosticEnabled(ctx, logger) {
		logger.V(1).Info("write diagnostic is disabled, skipping")
		return
	}

	// Get all WikiTargets
	var targets wikiv1alpha1.WikiTargetList
	if err := r.Client.List(ctx, &targets, client.InNamespace("glooscap-system")); err != nil {
		logger.Error(err, "failed to list WikiTargets (diagnostic will skip this cycle)")
		return
	}

	// Filter for readWrite mode targets
	readWriteTargets := []wikiv1alpha1.WikiTarget{}
	for _, target := range targets.Items {
		if target.Spec.Mode == wikiv1alpha1.WikiTargetModeReadWrite {
			readWriteTargets = append(readWriteTargets, target)
		}
	}

	if len(readWriteTargets) == 0 {
		logger.V(1).Info("no readWrite WikiTargets found, skipping write diagnostic")
		return
	}

	logger.Info("running write diagnostic", "targetCount", len(readWriteTargets))

	// Process each readWrite target
	for i := range readWriteTargets {
		target := &readWriteTargets[i]
		r.testTargetWrite(ctx, logger, target)
	}
}

// generateRandomSuffix generates a random 6-character suffix for the master key
func generateRandomSuffix() (string, error) {
	bytes := make([]byte, 3) // 3 bytes = 6 hex characters
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// getOrCreateMasterKey gets the master key for a target from annotations, or creates a new one
func (r *WikiTargetDiagnosticRunnable) getOrCreateMasterKey(ctx context.Context, target *wikiv1alpha1.WikiTarget, logger logr.Logger) (string, error) {
	// Check in-memory cache first
	r.keysMu.RLock()
	if key, exists := r.masterKeys[target.Name]; exists {
		r.keysMu.RUnlock()
		return key, nil
	}
	r.keysMu.RUnlock()

	// Check annotations
	if target.Annotations != nil {
		if key, exists := target.Annotations[annotationMasterKey]; exists && key != "" {
			// Store in cache
			r.keysMu.Lock()
			r.masterKeys[target.Name] = key
			r.keysMu.Unlock()
			return key, nil
		}
	}

	// Generate new master key
	suffix, err := generateRandomSuffix()
	if err != nil {
		return "", fmt.Errorf("failed to generate random suffix: %w", err)
	}
	masterKey := fmt.Sprintf("%s %s", diagnosticPageTitlePrefix, suffix)

	// Store in annotations
	if target.Annotations == nil {
		target.Annotations = make(map[string]string)
	}
	target.Annotations[annotationMasterKey] = masterKey

	// Update the target
	if err := r.Client.Update(ctx, target); err != nil {
		return "", fmt.Errorf("failed to update target annotations: %w", err)
	}

	// Store in cache
	r.keysMu.Lock()
	r.masterKeys[target.Name] = masterKey
	r.keysMu.Unlock()

	logger.Info("generated new master key for diagnostic", "target", target.Name, "masterKey", masterKey)
	return masterKey, nil
}

// testTargetWrite creates or updates the diagnostic page for a specific target
func (r *WikiTargetDiagnosticRunnable) testTargetWrite(ctx context.Context, logger logr.Logger, target *wikiv1alpha1.WikiTarget) {
	targetLogger := logger.WithValues("wikitarget", target.Name, "uri", target.Spec.URI)

	// Create Outline client
	if r.OutlineClient == nil {
		targetLogger.Error(nil, "outline client factory not configured")
		return
	}

	client, err := r.OutlineClient.New(ctx, r.Client, target)
	if err != nil {
		targetLogger.Error(err, "failed to create outline client for diagnostic")
		return
	}

	// Get or create master key for this target
	masterKey, err := r.getOrCreateMasterKey(ctx, target, targetLogger)
	if err != nil {
		targetLogger.Error(err, "failed to get or create master key")
		return
	}

	// On startup, clean up all old "GLOODIAG TEST *" pages (any with the prefix)
	// This ensures we start fresh
	targetLogger.Info("checking for old diagnostic pages to clean up", "masterKey", masterKey)

	// List all pages to find any matching "GLOODIAG TEST *" pattern
	var pages []outline.PageSummary
	if target.Status.CollectionID != "" {
		pages, err = client.ListPages(ctx, target.Status.CollectionID)
		if err != nil {
			targetLogger.V(1).Info("failed to list pages with collection ID, trying all pages", "collectionID", target.Status.CollectionID, "error", err)
			pages, err = client.ListPages(ctx)
		}
	} else {
		pages, err = client.ListPages(ctx)
	}

	if err != nil {
		targetLogger.V(1).Info("failed to list pages, skipping diagnostic this cycle", "error", err)
		return
	}

	// Find all pages matching "GLOODIAG TEST *" pattern (any diagnostic pages)
	var oldDiagnosticPages []outline.PageSummary
	for _, page := range pages {
		if strings.HasPrefix(page.Title, diagnosticPageTitlePrefix) {
			// Only delete if it's not our current master key (we'll handle that separately)
			if page.Title != masterKey {
				oldDiagnosticPages = append(oldDiagnosticPages, page)
			}
		}
	}

	// Delete all old diagnostic pages (cleanup on startup and ongoing)
	if len(oldDiagnosticPages) > 0 {
		targetLogger.Info("found old diagnostic pages to clean up", "count", len(oldDiagnosticPages))
		for _, page := range oldDiagnosticPages {
			targetLogger.Info("deleting old diagnostic page", "pageID", page.ID, "title", page.Title, "isDraft", page.IsDraft)
			if err := client.DeletePage(ctx, page.ID); err != nil {
				targetLogger.Error(err, "failed to delete old diagnostic page", "pageID", page.ID)
				// Continue deleting others even if one fails
			} else {
				targetLogger.Info("deleted old diagnostic page", "pageID", page.ID, "title", page.Title)
			}
		}
	}

	// Get last page ID from annotations or cache
	var lastPageID string
	r.keysMu.RLock()
	if id, exists := r.lastPageIDs[target.Name]; exists {
		lastPageID = id
	}
	r.keysMu.RUnlock()

	// Also check annotations (in case cache was cleared)
	if lastPageID == "" && target.Annotations != nil {
		if id, exists := target.Annotations[annotationLastPageID]; exists {
			lastPageID = id
		}
	}

	// If we have a last page ID, delete it first (from previous run)
	if lastPageID != "" {
		targetLogger.Info("deleting previous diagnostic page", "pageID", lastPageID)
		if err := client.DeletePage(ctx, lastPageID); err != nil {
			// Page might already be deleted, that's ok
			targetLogger.V(1).Info("failed to delete previous page (may already be gone)", "pageID", lastPageID, "error", err)
		} else {
			targetLogger.Info("deleted previous diagnostic page", "pageID", lastPageID)

			// Verify it's gone by trying to get it (optional verification)
			// We'll just proceed - if it still exists, we'll handle it in the next cycle
		}
	}

	// Generate diagnostic content with timestamp and UUID
	now := time.Now()
	diagUUID := uuid.New().String()
	content := fmt.Sprintf(`# %s

This is an automated diagnostic page created by Glooscap to verify write access to this wiki.

## Diagnostic Information

- **Timestamp**: %s
- **UUID**: %s
- **WikiTarget**: %s
- **Namespace**: %s
- **Last Updated**: %s

This page is automatically updated every 30 seconds to verify that Glooscap can write to drafts in this wiki.

---
*This page can be safely ignored or deleted. It is used only for connectivity testing.*
`, masterKey, now.Format(time.RFC3339), diagUUID, target.Name, target.Namespace, now.Format(time.RFC3339))

	// Create new page with master key as title
	targetLogger.Info("creating new diagnostic page", "masterKey", masterKey)
	createReq := outline.CreatePageRequest{
		Title: masterKey,
		Text:  content,
		// Don't specify collection - will be created in drafts
	}
	createResp, err := client.CreatePage(ctx, createReq)
	if err != nil {
		targetLogger.Error(err, "failed to create diagnostic page")
		return
	}

	newPageID := createResp.Data.ID
	targetLogger.Info("diagnostic page created successfully",
		"pageID", newPageID,
		"slug", createResp.Data.Slug,
		"uuid", diagUUID,
		"masterKey", masterKey)

	// Store the new page ID in annotations and cache for next run
	if target.Annotations == nil {
		target.Annotations = make(map[string]string)
	}
	target.Annotations[annotationLastPageID] = newPageID
	if err := r.Client.Update(ctx, target); err != nil {
		targetLogger.Error(err, "failed to update target with last page ID")
		// Continue anyway - cache will help
	}

	// Update cache
	r.keysMu.Lock()
	r.lastPageIDs[target.Name] = newPageID
	r.keysMu.Unlock()
}

// SetupWikiTargetDiagnosticRunnable sets up the WikiTarget diagnostic runnable with the Manager.
func SetupWikiTargetDiagnosticRunnable(mgr manager.Manager, outlineClient OutlineClientFactory) error {
	runnable := &WikiTargetDiagnosticRunnable{
		Client:        mgr.GetClient(),
		OutlineClient: outlineClient,
	}
	return mgr.Add(runnable)
}

