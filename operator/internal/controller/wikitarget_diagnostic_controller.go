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
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
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
}

const (
	// Diagnostic page title
	diagnosticPageTitle = "GLOODIAG TEST"
	// How often to run the diagnostic (every 30 seconds)
	diagnosticInterval = 30 * time.Second
)

// Start implements manager.Runnable
func (r *WikiTargetDiagnosticRunnable) Start(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("wikitarget-diagnostic")

	logger.Info("starting WikiTarget write diagnostic (runs every 30 seconds)")

	// Run initial diagnostic immediately
	r.runDiagnostic(ctx, logger)

	// Then run every 5 minutes
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

// runDiagnostic checks all readWrite WikiTargets and creates/updates diagnostic pages
func (r *WikiTargetDiagnosticRunnable) runDiagnostic(ctx context.Context, logger logr.Logger) {
	// Failures are ok - just log and continue
	defer func() {
		if r := recover(); r != nil {
			logger.Error(fmt.Errorf("panic in runDiagnostic: %v", r), "diagnostic panicked, continuing")
		}
	}()

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
`, diagnosticPageTitle, now.Format(time.RFC3339), diagUUID, target.Name, target.Namespace, now.Format(time.RFC3339))

	// Check if diagnostic page already exists
	// Use collection ID if available to constrain search, but also search all pages since drafts might not be in a collection
	var existingPageID string
	var existingPageIsDraft bool
	var pages []outline.PageSummary
	
	// Try to list pages - use collection ID if available, but we also need to check drafts which might not be in collections
	// First try with collection ID if available
	if target.Status.CollectionID != "" {
		pages, err = client.ListPages(ctx, target.Status.CollectionID)
		if err != nil {
			targetLogger.V(1).Info("failed to list pages with collection ID, trying all pages", "collectionID", target.Status.CollectionID, "error", err)
			// Fallback to listing all pages
			pages, err = client.ListPages(ctx)
		}
	} else {
		pages, err = client.ListPages(ctx)
	}
	
	if err != nil {
		targetLogger.V(1).Info("failed to list pages, skipping diagnostic this cycle", "error", err)
		// Don't try to create if we can't list - we might create duplicates
		return
	}
	
	// Find all pages with the diagnostic title
	// Track the last link to draft (prefer draft pages, then most recent)
	var allDiagnosticPages []outline.PageSummary
	for _, page := range pages {
		if page.Title == diagnosticPageTitle {
			allDiagnosticPages = append(allDiagnosticPages, page)
		}
	}

	// If we found diagnostic pages, keep the best one and delete the rest
	if len(allDiagnosticPages) > 0 {
		// Find the best page to keep: prefer drafts, then most recent
		var bestPage *outline.PageSummary
		var bestIndex int
		for i := range allDiagnosticPages {
			page := &allDiagnosticPages[i]
			if bestPage == nil {
				bestPage = page
				bestIndex = i
				continue
			}
			// Prefer draft pages
			if page.IsDraft && !bestPage.IsDraft {
				bestPage = page
				bestIndex = i
				continue
			}
			// If both are drafts or both are not drafts, prefer most recent
			if page.IsDraft == bestPage.IsDraft {
				if page.UpdatedAt.After(bestPage.UpdatedAt) {
					bestPage = page
					bestIndex = i
				}
			}
		}

		existingPageID = bestPage.ID
		existingPageIsDraft = bestPage.IsDraft
		targetLogger.Info("found diagnostic pages", "total", len(allDiagnosticPages), "keeping", existingPageID, "isDraft", existingPageIsDraft)

		// Delete all other diagnostic pages (loop through and delete duplicates)
		for i, page := range allDiagnosticPages {
			if i != bestIndex {
				targetLogger.Info("deleting duplicate diagnostic page", "pageID", page.ID, "isDraft", page.IsDraft)
				if err := client.DeletePage(ctx, page.ID); err != nil {
					targetLogger.Error(err, "failed to delete duplicate diagnostic page", "pageID", page.ID)
					// Continue deleting others even if one fails
				} else {
					targetLogger.Info("deleted duplicate diagnostic page", "pageID", page.ID)
				}
			}
		}

		// Update the kept page
		targetLogger.Info("updating existing diagnostic page", "pageID", existingPageID)
		updateReq := outline.UpdatePageRequest{
			ID:   existingPageID,
			Text: content,
		}
		updateResp, err := client.UpdatePage(ctx, updateReq)
		if err != nil {
			targetLogger.Error(err, "failed to update diagnostic page")
			// If update fails, try creating a new one (maybe the page was deleted)
			targetLogger.Info("update failed, attempting to create new page instead")
			existingPageID = "" // Clear so we try create below
		} else {
			targetLogger.Info("diagnostic page updated successfully",
				"pageID", updateResp.Data.ID,
				"slug", updateResp.Data.Slug,
				"uuid", diagUUID)
			return // Success, we're done
		}
	}

	// Create new page (either doesn't exist, or update failed and we're retrying)
	if existingPageID == "" {
		targetLogger.Info("creating new diagnostic page")
		createReq := outline.CreatePageRequest{
			Title: diagnosticPageTitle,
			Text:  content,
			// Don't specify collection - will be created in drafts
		}
		createResp, err := client.CreatePage(ctx, createReq)
		if err != nil {
			targetLogger.Error(err, "failed to create diagnostic page")
			return
		}
		targetLogger.Info("diagnostic page created successfully",
			"pageID", createResp.Data.ID,
			"slug", createResp.Data.Slug,
			"uuid", diagUUID)
	}
}

// SetupWikiTargetDiagnosticRunnable sets up the WikiTarget diagnostic runnable with the Manager.
func SetupWikiTargetDiagnosticRunnable(mgr manager.Manager, outlineClient OutlineClientFactory) error {
	runnable := &WikiTargetDiagnosticRunnable{
		Client:        mgr.GetClient(),
		OutlineClient: outlineClient,
	}
	return mgr.Add(runnable)
}

