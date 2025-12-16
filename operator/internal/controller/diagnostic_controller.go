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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/go-logr/logr"
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
	
	logger.Info("starting diagnostic job creator (creates jobs every 2 minutes)")
	
	ticker := time.NewTicker(2 * time.Minute)
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
	
	// Create 2-3 test TranslationJobs with different configurations
	testJobs := []struct {
		name        string
		pageTitle   string
		languageTag string
		pageID      string
	}{
		{
			name:        fmt.Sprintf("diagnostic-test-1-%d", time.Now().Unix()),
			pageTitle:   "PAGE2-TEST",
			languageTag: "fr-CA",
			pageID:      pageID,
		},
		{
			name:        fmt.Sprintf("diagnostic-test-2-%d", time.Now().Unix()),
			pageTitle:   "Test Page",
			languageTag: "fr-CA",
			pageID:      pageID,
		},
		{
			name:        fmt.Sprintf("diagnostic-test-3-%d", time.Now().Unix()),
			pageTitle:   "Diagnostic Translation",
			languageTag: "fr-CA",
			pageID:      pageID,
		},
	}

	created := 0
	for _, testJob := range testJobs {
		// Check if job already exists
		var existing wikiv1alpha1.TranslationJob
		if err := r.Client.Get(ctx, client.ObjectKey{Namespace: "glooscap-system", Name: testJob.name}, &existing); err == nil {
			// Job exists, skip
			continue
		}

		// Create new TranslationJob
		job := &wikiv1alpha1.TranslationJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testJob.name,
				Namespace: "glooscap-system",
				Labels: map[string]string{
					"app.kubernetes.io/managed-by":  "diagnostic-controller",
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
				Parameters: map[string]string{
					"pageTitle": testJob.pageTitle,
					"pipeline":  "TektonJob",
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

