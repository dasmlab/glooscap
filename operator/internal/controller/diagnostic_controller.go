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
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	manager "sigs.k8s.io/controller-runtime/pkg/manager"

	wikiv1alpha1 "github.com/dasmlab/glooscap-operator/api/v1alpha1"
)

// DiagnosticRunnable creates test TranslationJobs periodically for diagnostic purposes.
// This runs in the background and creates TranslationJobs every 2 minutes
// to test the translation pipeline end-to-end.
type DiagnosticRunnable struct {
	Client client.Client
}

// Start implements manager.Runnable
func (r *DiagnosticRunnable) Start(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("diagnostic")

	logger.Info("starting diagnostic job creator (creates jobs every 30 seconds)")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Create initial batch immediately
	r.createDiagnosticJobs(ctx, logger)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			r.createDiagnosticJobs(ctx, logger)
		}
	}
}

func (r *DiagnosticRunnable) createDiagnosticJobs(ctx context.Context, logger logr.Logger) {
	// Get all WikiTargets to find source and destination
	var targets wikiv1alpha1.WikiTargetList
	if err := r.Client.List(ctx, &targets, client.InNamespace("glooscap-system")); err != nil {
		logger.Error(err, "failed to list WikiTargets")
		return
	}

	if len(targets.Items) < 1 {
		logger.Info("no WikiTargets found, skipping diagnostic job creation")
		return
	}

	// Find a source target (prefer one with pages)
	var sourceTarget *wikiv1alpha1.WikiTarget
	var destTarget *wikiv1alpha1.WikiTarget

	for i := range targets.Items {
		target := &targets.Items[i]
		if target.Spec.Mode != wikiv1alpha1.WikiTargetModeReadOnly {
			if destTarget == nil {
				destTarget = target
			}
		}
		if sourceTarget == nil {
			sourceTarget = target
		}
	}

	if sourceTarget == nil {
		logger.Info("no source target found")
		return
	}

	if destTarget == nil {
		// Use source as destination if no writable target found
		destTarget = sourceTarget
	}

	// Try to find a real page ID from the source target
	// We'll use a known test page ID, or try to find one from the catalog
	pageID := "998e669e-a2fe-496a-92d3-a265cb27a362" // Default test page ID

	// Create diagnostic TranslationJobs using iskoces test cases
	// These use embedded test content that matches iskoces/run-tests.sh
	testJobs := []struct {
		name        string
		pageTitle   string
		languageTag string
		pageID      string
		testContent string // Embedded test content from iskoces
	}{
		{
			name:        fmt.Sprintf("diagnostic-starwars-%d", time.Now().Unix()),
			pageTitle:   "Star Wars Opening",
			languageTag: "fr-CA",
			pageID:      pageID,
			testContent: `A long time ago in a galaxy far, far away...

It is a period of civil war. Rebel spaceships, striking from a hidden base, have won their first victory against the evil Galactic Empire.

During the battle, Rebel spies managed to steal secret plans to the Empire's ultimate weapon, the DEATH STAR, an armored space station with enough power to destroy an entire planet.

Pursued by the Empire's sinister agents, Princess Leia races home aboard her starship, custodian of the stolen plans that can save her people and restore freedom to the galaxy...`,
		},
		{
			name:        fmt.Sprintf("diagnostic-technical-%d", time.Now().Unix()),
			pageTitle:   "Technical Documentation",
			languageTag: "fr-CA",
			pageID:      pageID,
			testContent: `# Technical Documentation Translation Test

This document contains technical terminology and code examples that should be translated accurately.

## Key Concepts

- **Machine Translation**: The automatic translation of text from one language to another using computational methods.
- **Neural Networks**: Artificial intelligence systems inspired by biological neural networks.
- **API Endpoint**: A specific URL where an API can be accessed.

## Code Example

` + "```python" + `
def translate_text(text, source_lang, target_lang):
    """Translate text using the translation service."""
    response = client.translate(
        text=text,
        source=source_lang,
        target=target_lang
    )
    return response.translated_text
` + "```" + `

## Best Practices

1. Always validate input before translation
2. Handle errors gracefully
3. Cache translations when possible
4. Monitor translation quality`,
		},
		{
			name:        fmt.Sprintf("diagnostic-business-%d", time.Now().Unix()),
			pageTitle:   "Business Email",
			languageTag: "fr-CA",
			pageID:      pageID,
			testContent: `Subject: Quarterly Business Review Meeting

Dear Team,

I hope this message finds you well. I am writing to schedule our quarterly business review meeting for next month.

The meeting will cover:
- Revenue performance for Q3
- Strategic initiatives and their progress
- Upcoming product launches
- Budget allocation for next quarter

Please confirm your availability by Friday. The meeting will be held in the main conference room at 2:00 PM.

Best regards,
Management Team`,
		},
	}

	// Clean up old completed diagnostic jobs (keep only last 5 per type)
	logger.Info("cleaning up old diagnostic jobs (keeping last 5 per type)")
	var existingJobs wikiv1alpha1.TranslationJobList
	if err := r.Client.List(ctx, &existingJobs,
		client.InNamespace("glooscap-system"),
		client.MatchingLabels{"glooscap.dasmlab.org/diagnostic": "true"}); err != nil {
		logger.Error(err, "failed to list diagnostic jobs for cleanup")
	} else {
		// Group by test type (starwars, technical, business)
		jobsByType := make(map[string][]wikiv1alpha1.TranslationJob)
		for _, job := range existingJobs.Items {
			if job.Status.State == wikiv1alpha1.TranslationJobStateCompleted ||
				job.Status.State == wikiv1alpha1.TranslationJobStateFailed {
				// Extract type from name (e.g., "diagnostic-starwars-1765867004" -> "starwars")
				parts := strings.Split(job.Name, "-")
				if len(parts) >= 2 {
					jobType := parts[1] // "starwars", "technical", or "business"
					jobsByType[jobType] = append(jobsByType[jobType], job)
				}
			}
		}

		// Sort by creation timestamp and delete old ones (keep last 5)
		deletedCount := 0
		for jobType, jobs := range jobsByType {
			if len(jobs) > 5 {
				// Sort by creation time (oldest first)
				sort.Slice(jobs, func(i, j int) bool {
					return jobs[i].CreationTimestamp.Before(&jobs[j].CreationTimestamp)
				})
				// Delete oldest ones
				toDelete := len(jobs) - 5
				for i := 0; i < toDelete; i++ {
					if err := r.Client.Delete(ctx, &jobs[i]); err == nil {
						logger.Info("deleted old diagnostic job", "name", jobs[i].Name, "type", jobType, "state", jobs[i].Status.State)
						deletedCount++
					} else {
						logger.Error(err, "failed to delete old diagnostic job", "name", jobs[i].Name, "type", jobType)
					}
				}
			}
		}
		if deletedCount > 0 {
			logger.Info("diagnostic job cleanup complete", "deleted", deletedCount, "types", len(jobsByType))
		} else {
			logger.V(1).Info("no diagnostic jobs to clean up", "total_jobs", len(existingJobs.Items))
		}
	}

	created := 0
	for _, testJob := range testJobs {
		// Check if job already exists
		var existing wikiv1alpha1.TranslationJob
		if err := r.Client.Get(ctx, client.ObjectKey{Namespace: "glooscap-system", Name: testJob.name}, &existing); err == nil {
			// Job exists, skip
			continue
		}

		// Check if context is canceled (during shutdown)
		select {
		case <-ctx.Done():
			logger.Info("context canceled, stopping diagnostic job creation")
			return
		default:
		}

		// Create new TranslationJob
		job := &wikiv1alpha1.TranslationJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testJob.name,
				Namespace: "glooscap-system",
				Labels: map[string]string{
					"app.kubernetes.io/managed-by":    "diagnostic-controller",
					"glooscap.dasmlab.org/diagnostic": "true",
				},
			},
			Spec: wikiv1alpha1.TranslationJobSpec{
				Source: wikiv1alpha1.TranslationSourceSpec{
					TargetRef: sourceTarget.Name,
					PageID:    testJob.pageID,
				},
				Destination: &wikiv1alpha1.TranslationDestinationSpec{
					TargetRef:   destTarget.Name,
					LanguageTag: testJob.languageTag,
				},
				Pipeline: wikiv1alpha1.TranslationPipelineModeTektonJob, // Use TektonJob pipeline to dispatch to runner
				Parameters: map[string]string{
					"pageTitle":   testJob.pageTitle,
					"testContent": testJob.testContent, // Embedded test content
					"diagnostic":  "true",              // Mark as diagnostic job
					"prefix":      "AUTODIAG",          // Use AUTODIAG prefix
				},
			},
		}

		if err := r.Client.Create(ctx, job); err != nil {
			logger.Error(err, "failed to create diagnostic TranslationJob", "name", testJob.name)
			continue
		}

		logger.Info("created diagnostic TranslationJob",
			"name", testJob.name,
			"source", sourceTarget.Name,
			"destination", destTarget.Name,
			"language", testJob.languageTag,
			"pageID", testJob.pageID)
		created++
	}

	if created > 0 {
		logger.Info("created diagnostic TranslationJobs", "count", created)
	}
}

// SetupDiagnosticRunnable sets up the diagnostic runnable with the Manager.
func SetupDiagnosticRunnable(mgr manager.Manager) error {
	runnable := &DiagnosticRunnable{
		Client: mgr.GetClient(),
	}
	return mgr.Add(runnable)
}
