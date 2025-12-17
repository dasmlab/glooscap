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
	"sync"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	manager "sigs.k8s.io/controller-runtime/pkg/manager"

	wikiv1alpha1 "github.com/dasmlab/glooscap-operator/api/v1alpha1"
)

// DiagnosticRunnable creates test TranslationJobs periodically for diagnostic purposes.
// This runs in the background and creates a StarWars test TranslationJob every 30 seconds
// to test the translation pipeline end-to-end.
type DiagnosticRunnable struct {
	Client client.Client
	// Track last failure time per job type to implement cooldown
	lastFailureTime map[string]time.Time
	lastFailureMu   sync.Mutex
}

// Cooldown period after a failed diagnostic job before trying again
const diagnosticCooldownPeriod = 45 * time.Second

// Start implements manager.Runnable
func (r *DiagnosticRunnable) Start(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("diagnostic")

	logger.Info("starting background translation test job creator (runs every 30 seconds)")

	// Create initial test job immediately
	r.createTestJob(ctx, logger)

	// Then run every 30 seconds
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// Create test job - failures are ok, just log and continue
			r.createTestJob(ctx, logger)
		}
	}
}

// createTestJob creates a single StarWars test translation job
func (r *DiagnosticRunnable) createTestJob(ctx context.Context, logger logr.Logger) {
	// Failures are ok - just log and continue
	defer func() {
		if r := recover(); r != nil {
			logger.Error(fmt.Errorf("panic in createTestJob: %v", r), "test job creation panicked, continuing")
		}
	}()

	// For diagnostic tests, we don't need WikiTargets - we just test the translation service
	// Use dummy target references that won't be resolved (test will use embedded content)
	var sourceTargetName string = "diagnostic-source"
	var destTargetName string = "diagnostic-dest"
	
	// Try to find real targets if available (for better integration testing)
	var targets wikiv1alpha1.WikiTargetList
	if err := r.Client.List(ctx, &targets, client.InNamespace("glooscap-system")); err == nil && len(targets.Items) > 0 {
		// Use real targets if available
	for i := range targets.Items {
		target := &targets.Items[i]
		if target.Spec.Mode != wikiv1alpha1.WikiTargetModeReadOnly {
				if destTargetName == "diagnostic-dest" {
					destTargetName = target.Name
			}
		}
			if sourceTargetName == "diagnostic-source" {
				sourceTargetName = target.Name
		}
	}
		logger.V(1).Info("using real WikiTargets for diagnostic test", "source", sourceTargetName, "dest", destTargetName)
	} else {
		logger.Info("no WikiTargets found, using dummy targets for translation service test only")
	}

	// Use a fixed test page ID for StarWars test
	pageID := "998e669e-a2fe-496a-92d3-a265cb27a362"

	// StarWars test content (from iskoces test suite)
	starWarsContent := `A long time ago in a galaxy far, far away...

It is a period of civil war. Rebel spaceships, striking from a hidden base, have won their first victory against the evil Galactic Empire.

During the battle, Rebel spies managed to steal secret plans to the Empire's ultimate weapon, the DEATH STAR, an armored space station with enough power to destroy an entire planet.

Pursued by the Empire's sinister agents, Princess Leia races home aboard her starship, custodian of the stolen plans that can save her people and restore freedom to the galaxy...`

	// Generate unique job name with timestamp
	jobName := fmt.Sprintf("test-starwars-%d", time.Now().Unix())

	// Check if a recent test job already exists and is still processing
	var existingJobs wikiv1alpha1.TranslationJobList
	if err := r.Client.List(ctx, &existingJobs,
		client.InNamespace("glooscap-system"),
		client.MatchingLabels{"glooscap.dasmlab.org/diagnostic": "true"}); err == nil {
		// Check if there's a recent job (within last 2 minutes) that's still processing
		for _, job := range existingJobs.Items {
			if strings.HasPrefix(job.Name, "test-starwars-") {
				// If job has no state and is older than 1 minute, it's likely stuck - allow new job
				if job.Status.State == "" {
					age := time.Since(job.CreationTimestamp.Time)
					if age > 1*time.Minute {
						logger.V(1).Info("test job has no state and is old, likely stuck - will create new one", "job", job.Name, "age", age)
						// Delete the stuck job
						if err := r.Client.Delete(ctx, &job); err == nil {
							logger.V(1).Info("deleted stuck test job with no state", "name", job.Name)
						}
						continue
					}
					// Job is new and has no state yet, skip creating new one (wait for reconciliation)
					logger.V(1).Info("test job has no state yet, waiting for reconciliation", "job", job.Name, "age", age)
					return
				}
				// Check if job is still in progress (not completed or failed)
				if job.Status.State != wikiv1alpha1.TranslationJobStateCompleted &&
					job.Status.State != wikiv1alpha1.TranslationJobStateFailed {
					// Job still processing, skip creating new one
					logger.V(1).Info("test job still processing, skipping creation", "job", job.Name, "state", job.Status.State)
					return
				}
				// If job completed or failed more than 2 minutes ago, we can create a new one
				if job.Status.FinishedAt != nil {
					age := time.Since(job.Status.FinishedAt.Time)
					if age < 2*time.Minute {
						logger.V(1).Info("recent test job exists, skipping creation", "job", job.Name, "age", age)
						return
					}
				}
			}
		}

		// Clean up old completed/failed test jobs (keep only last 3)
		testJobs := []wikiv1alpha1.TranslationJob{}
		for _, job := range existingJobs.Items {
			if strings.HasPrefix(job.Name, "test-starwars-") {
				testJobs = append(testJobs, job)
					}
				}
		if len(testJobs) > 3 {
				// Sort by creation time (oldest first)
			sort.Slice(testJobs, func(i, j int) bool {
				return testJobs[i].CreationTimestamp.Before(&testJobs[j].CreationTimestamp)
				})
			// Delete oldest ones (keep only the last 3)
			toDelete := len(testJobs) - 3
				for i := 0; i < toDelete; i++ {
				if err := r.Client.Delete(ctx, &testJobs[i]); err == nil {
					logger.V(1).Info("deleted old test job", "name", testJobs[i].Name, "state", testJobs[i].Status.State)
				}
			}
		}
		}

	// Check if this specific job already exists
		var existing wikiv1alpha1.TranslationJob
	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: "glooscap-system", Name: jobName}, &existing); err == nil {
		// Job exists, skip
		logger.V(1).Info("test job already exists", "name", jobName)
			return
		}

		// Create new TranslationJob
		job := &wikiv1alpha1.TranslationJob{
			ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
				Namespace: "glooscap-system",
				Labels: map[string]string{
					"app.kubernetes.io/managed-by":    "diagnostic-controller",
					"glooscap.dasmlab.org/diagnostic": "true",
				},
			},
			Spec: wikiv1alpha1.TranslationJobSpec{
				Source: wikiv1alpha1.TranslationSourceSpec{
				TargetRef: sourceTargetName,
				PageID:    pageID,
				},
				Destination: &wikiv1alpha1.TranslationDestinationSpec{
				TargetRef:   destTargetName,
				LanguageTag: "fr-CA",
				},
				Pipeline: wikiv1alpha1.TranslationPipelineModeTektonJob, // Use TektonJob pipeline to dispatch to runner
				Parameters: map[string]string{
				"pageTitle":   "Star Wars Opening",
				"testContent": starWarsContent, // Embedded test content
				"diagnostic":  "true",          // Mark as diagnostic job
				"prefix":      "AUTODIAG",     // Use AUTODIAG prefix
				},
			},
		}

		if err := r.Client.Create(ctx, job); err != nil {
		// Failures are ok - just log and continue
		logger.V(1).Info("failed to create test TranslationJob (may already exist)", "name", jobName, "error", err)
		return
		}

	logger.Info("created test TranslationJob",
		"name", jobName,
		"source", sourceTargetName,
		"destination", destTargetName,
		"language", "fr-CA",
		"pageID", pageID,
		"note", "This job tests translation service connectivity - results will be logged but not posted to wiki")
}

// createDiagnosticJobs creates multiple diagnostic jobs (kept for backward compatibility, but not used by default)
func (r *DiagnosticRunnable) createDiagnosticJobs(ctx context.Context, logger logr.Logger) {
	// This is the old method that creates multiple jobs - kept for reference
	// The new createTestJob method is used instead
	r.createTestJob(ctx, logger)
}

// SetupDiagnosticRunnable sets up the diagnostic runnable with the Manager.
func SetupDiagnosticRunnable(mgr manager.Manager) error {
	runnable := &DiagnosticRunnable{
		Client: mgr.GetClient(),
	}
	return mgr.Add(runnable)
}
