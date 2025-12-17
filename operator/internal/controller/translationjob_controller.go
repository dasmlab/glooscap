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

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	wikiv1alpha1 "github.com/dasmlab/glooscap-operator/api/v1alpha1"
	"github.com/dasmlab/glooscap-operator/pkg/catalog"
	"github.com/dasmlab/glooscap-operator/pkg/nanabush"
	"github.com/dasmlab/glooscap-operator/pkg/outline"
	"github.com/dasmlab/glooscap-operator/pkg/vllm"
)

// TranslationJobEvent represents a translation job event for SSE broadcasting
// This type is also defined in internal/server/http.go - they must match
type TranslationJobEvent struct {
	Type      string `json:"type"`                // "processing_translation" or "translation_complete"
	JobName   string `json:"jobName"`             // TranslationJob name (e.g., "translation-xxxx")
	PageURL   string `json:"pageUrl,omitempty"`   // URL to the translated page (for completion events)
	PageID    string `json:"pageId,omitempty"`    // Page ID of the translated page
	PageTitle string `json:"pageTitle,omitempty"` // Title of the translated page
	State     string `json:"state,omitempty"`     // Job state (e.g., "Completed", "Failed")
	Message   string `json:"message,omitempty"`   // Optional message
}

// TranslationJobReconciler reconciles a TranslationJob object
type TranslationJobReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Recorder record.EventRecorder

	Dispatcher    vllm.Dispatcher
	Jobs          *catalog.JobStore
	Catalogue     *catalog.Store
	OutlineClient OutlineClientFactory
	Nanabush      *nanabush.Client // Direct reference (for backward compatibility)
	// GetNanabushClient is a function that returns the current nanabush client (for runtime updates)
	GetNanabushClient func() *nanabush.Client
	// TranslationJobEventCh is a channel to send TranslationJob events for SSE broadcasting
	TranslationJobEventCh chan<- TranslationJobEvent
}

// +kubebuilder:rbac:groups=wiki.glooscap.dasmlab.org,resources=translationjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=wiki.glooscap.dasmlab.org,resources=translationjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=wiki.glooscap.dasmlab.org,resources=translationjobs/finalizers,verbs=update
// +kubebuilder:rbac:groups=wiki.glooscap.dasmlab.org,resources=wikitargets,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=create;delete;get;list;patch;update;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the TranslationJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *TranslationJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("translationjob", req.NamespacedName)

	var job wikiv1alpha1.TranslationJob
	if err := r.Get(ctx, req.NamespacedName, &job); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	now := metav1.Now()
	updated := job.Status.DeepCopy()

	if updated.State == "" {
		updated.State = wikiv1alpha1.TranslationJobStateQueued
		updated.StartedAt = &now
		meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "AwaitingDispatch",
			Message:            "Translation job is queued for dispatch",
			LastTransitionTime: now,
		})

		// Send processing_translation SSE event when job is first created (submitted)
		if r.TranslationJobEventCh != nil {
			select {
			case r.TranslationJobEventCh <- TranslationJobEvent{
				Type:    "processing_translation",
				JobName: job.Name,
				State:   string(updated.State),
				Message: updated.Message,
			}:
			default:
				// Channel full, skip (non-blocking)
			}
		}
	}

	// Validation phase: check template, destination, and duplicates
	// Only validate if we're in Queued state (first time through)
	if updated.State == wikiv1alpha1.TranslationJobStateQueued {
		updated.State = wikiv1alpha1.TranslationJobStateValidating
		meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "Validating",
			Message:            "Validating translation request",
			LastTransitionTime: now,
		})
		// Update status immediately to transition to Validating
		if !jobStatusChanged(&job.Status, updated) {
			return ctrl.Result{}, nil
		}
		job.Status = *updated
		if err := r.Status().Update(ctx, &job); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Check if this is a diagnostic job - diagnostic jobs skip WikiTarget validation
	isDiagnostic := job.Labels["glooscap.dasmlab.org/diagnostic"] == "true" ||
		job.Spec.Parameters["diagnostic"] == "true"

	// Get source target for use in validation and dispatch (skip for diagnostic jobs)
	var sourceTarget wikiv1alpha1.WikiTarget
	if !isDiagnostic {
		// Only validate WikiTarget for non-diagnostic jobs
	if job.Spec.Source.TargetRef != "" {
		if err := r.Get(ctx, client.ObjectKey{Namespace: job.Namespace, Name: job.Spec.Source.TargetRef}, &sourceTarget); err != nil {
			if errors.IsNotFound(err) {
				meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
					Type:               "Ready",
					Status:             metav1.ConditionFalse,
					Reason:             "TargetMissing",
					Message:            "Referenced WikiTarget does not exist",
					LastTransitionTime: now,
				})
				updated.State = wikiv1alpha1.TranslationJobStateFailed
				updated.Message = "WikiTarget not found"
				updated.FinishedAt = &now
				job.Status = *updated
				if err := r.Status().Update(ctx, &job); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}
	} else {
		updated.State = wikiv1alpha1.TranslationJobStateFailed
		updated.Message = "Source TargetRef is required"
		updated.FinishedAt = &now
		job.Status = *updated
		if err := r.Status().Update(ctx, &job); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
		}
	} else {
		logger.Info("diagnostic job: skipping WikiTarget validation, will use embedded test content", "job", job.Name)
	}

	// Run validation only if we're in Validating state
	if updated.State == wikiv1alpha1.TranslationJobStateValidating {
		logger.Info("validating translation job", "job", job.Name)
		// Perform validation checks

		// Check if page is a template (should not be translated)
		if r.Catalogue != nil {
			targetID := fmt.Sprintf("%s/%s", sourceTarget.Namespace, sourceTarget.Name)
			pages := r.Catalogue.List(targetID)
			for _, page := range pages {
				if page.ID == job.Spec.Source.PageID {
					if page.IsTemplate {
						logger.Info("validation failed: page is a template", "pageID", job.Spec.Source.PageID, "title", page.Title)
						meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
							Type:               "Ready",
							Status:             metav1.ConditionFalse,
							Reason:             "TemplateRejected",
							Message:            "Templates cannot be translated",
							LastTransitionTime: now,
						})
						updated.State = wikiv1alpha1.TranslationJobStateFailed
						updated.Message = "Page is a template and cannot be translated"
						updated.FinishedAt = &now
						job.Status = *updated
						if err := r.Status().Update(ctx, &job); err != nil {
							return ctrl.Result{}, err
						}
						return ctrl.Result{}, nil
					}
					break
				}
			}
		}

		// Validate destination (skip for diagnostic jobs)
		if !isDiagnostic {
		destTargetRef := job.Spec.Source.TargetRef
		if job.Spec.Destination != nil && job.Spec.Destination.TargetRef != "" {
			destTargetRef = job.Spec.Destination.TargetRef
		}

		var destTarget wikiv1alpha1.WikiTarget
		if err := r.Get(ctx, client.ObjectKey{Namespace: job.Namespace, Name: destTargetRef}, &destTarget); err != nil {
			if errors.IsNotFound(err) {
				// For diagnostic jobs, skip destination WikiTarget validation
				if isDiagnostic {
					logger.Info("diagnostic job: skipping destination WikiTarget validation, using dummy target", "targetRef", destTargetRef)
					// Create a dummy destTarget for diagnostic jobs
					destTarget = wikiv1alpha1.WikiTarget{
						ObjectMeta: metav1.ObjectMeta{
							Name:      destTargetRef,
							Namespace: job.Namespace,
						},
						Spec: wikiv1alpha1.WikiTargetSpec{
							URI:  "diagnostic://test",
							Mode: wikiv1alpha1.WikiTargetModeReadWrite,
						},
					}
				} else {
				meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
					Type:               "Ready",
					Status:             metav1.ConditionFalse,
					Reason:             "DestinationMissing",
					Message:            "Destination WikiTarget does not exist",
					LastTransitionTime: now,
				})
				updated.State = wikiv1alpha1.TranslationJobStateFailed
				updated.Message = "Destination WikiTarget not found"
				updated.FinishedAt = &now
				job.Status = *updated
				if err := r.Status().Update(ctx, &job); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}
			} else {
			return ctrl.Result{}, err
			}
		}

		// Check if destination allows writes (skip for diagnostic jobs)
		if !isDiagnostic && destTarget.Spec.Mode == wikiv1alpha1.WikiTargetModeReadOnly {
			meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				Reason:             "DestinationReadOnly",
				Message:            "Destination WikiTarget is read-only",
				LastTransitionTime: now,
			})
			updated.State = wikiv1alpha1.TranslationJobStateFailed
			updated.Message = "Destination WikiTarget is read-only and cannot accept translations"
			updated.FinishedAt = &now
			job.Status = *updated
			if err := r.Status().Update(ctx, &job); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		// Check for duplicate page at destination (skip for diagnostic jobs)
		if !isDiagnostic && r.OutlineClient != nil && r.Catalogue != nil {
			destClient, err := r.OutlineClient.New(ctx, r.Client, &destTarget)
			if err == nil {
					// Use collection constraint from destination WikiTarget if available
					var destPages []outline.PageSummary
					if destTarget.Status.CollectionID != "" {
						destPages, err = destClient.ListPages(ctx, destTarget.Status.CollectionID)
						logger.V(1).Info("checking duplicates in destination collection", "collectionID", destTarget.Status.CollectionID, "collectionName", destTarget.Status.CollectionName)
					} else {
						destPages, err = destClient.ListPages(ctx)
						logger.V(1).Info("checking duplicates in all destination pages (no collection constraint)")
					}
				if err == nil {
					// Get source page title from catalog
					sourcePageTitle := ""
					targetID := fmt.Sprintf("%s/%s", sourceTarget.Namespace, sourceTarget.Name)
					sourcePages := r.Catalogue.List(targetID)
					for _, page := range sourcePages {
						if page.ID == job.Spec.Source.PageID {
							sourcePageTitle = page.Title
							break
						}
					}

					// Check for existing page with AUTOTRANSLATED prefix
					// We NEVER overwrite existing pages - if one exists, we'll create a unique one
					existingTranslatedPage := ""
					for _, destPage := range destPages {
						// Check if this is an AUTOTRANSLATED page for our source
						if strings.HasPrefix(destPage.Title, "AUTOTRANSLATED--> ") {
							// Extract source title from AUTOTRANSLATED page
							extractedSource := strings.TrimPrefix(destPage.Title, "AUTOTRANSLATED--> ")
							if extractedSource == sourcePageTitle {
								existingTranslatedPage = destPage.ID
								logger.Info("found existing AUTOTRANSLATED page for source",
									"source_title", sourcePageTitle,
									"existing_page_id", destPage.ID,
									"existing_page_title", destPage.Title)
								break
							}
						}
					}

					// If we found an existing translated page, we'll create a unique one
					// Store this info for later use in publishing
					if existingTranslatedPage != "" {
						logger.Info("existing AUTOTRANSLATED page found - will create unique page",
							"source_title", sourcePageTitle,
							"existing_page_id", existingTranslatedPage)
					}
				}
			}
			}
		} else {
			logger.Info("diagnostic job: skipping destination WikiTarget validation", "job", job.Name)
		}

		// If we reach here, validation passed - transition to Queued
		logger.Info("validation passed, transitioning to Queued", "job", job.Name)
		updated.State = wikiv1alpha1.TranslationJobStateQueued
		meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "ValidationPassed",
			Message:            "Validation passed, ready for dispatch",
			LastTransitionTime: now,
		})
		// Don't return here - continue to dispatch logic below
		// We'll update status after dispatch
	}
	// Handle approval for duplicates or draft publishing (check if user approved via annotation or publish job)
	if updated.State == wikiv1alpha1.TranslationJobStateAwaitingApproval {
		// Check if this is a duplicate approval
		if approved, ok := job.Annotations["glooscap.dasmlab.org/duplicate-approved"]; ok && approved == "true" {
			// User approved, clear duplicate info and proceed
			updated.DuplicateInfo = nil
			updated.State = wikiv1alpha1.TranslationJobStateQueued
			meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				Reason:             "Approved",
				Message:            "Duplicate overwrite approved by user",
				LastTransitionTime: now,
			})
		} else if publishJobName, ok := job.Annotations["glooscap.dasmlab.org/publish-job"]; ok && publishJobName != "" {
			// Check if publish job has completed successfully
			var publishJob wikiv1alpha1.TranslationJob
			if err := r.Get(ctx, client.ObjectKey{Namespace: job.Namespace, Name: publishJobName}, &publishJob); err == nil {
				if publishJob.Status.State == wikiv1alpha1.TranslationJobStateCompleted {
					// Publish job completed, mark original job as completed
					updated.State = wikiv1alpha1.TranslationJobStateCompleted
					updated.FinishedAt = &now
					updated.Message = "Translation published successfully"
					if job.Annotations != nil {
						job.Annotations["glooscap.dasmlab.org/is-draft"] = "false"
					}
					meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
						Type:               "Ready",
						Status:             metav1.ConditionTrue,
						Reason:             "Published",
						Message:            "Translation has been published",
						LastTransitionTime: now,
					})
					// Send translation_complete SSE event
					if r.TranslationJobEventCh != nil {
						pageURL := ""
						if job.Annotations != nil {
							pageURL = job.Annotations["glooscap.dasmlab.org/published-page-url"]
						}
						select {
						case r.TranslationJobEventCh <- TranslationJobEvent{
							Type:      "translation_complete",
							JobName:   job.Name,
							PageURL:   pageURL,
							PageID:    job.Annotations["glooscap.dasmlab.org/published-page-id"],
							PageTitle: job.Annotations["glooscap.dasmlab.org/published-page-title"],
							State:     string(updated.State),
							Message:   updated.Message,
						}:
						default:
							// Channel full, skip (non-blocking)
						}
					}
				} else if publishJob.Status.State == wikiv1alpha1.TranslationJobStateFailed {
					// Publish job failed
					updated.State = wikiv1alpha1.TranslationJobStateFailed
					updated.FinishedAt = &now
					updated.Message = fmt.Sprintf("Publish job failed: %s", publishJob.Status.Message)
					meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
						Type:               "Ready",
						Status:             metav1.ConditionFalse,
						Reason:             "PublishFailed",
						Message:            updated.Message,
						LastTransitionTime: now,
					})
				} else {
					// Publish job still running, wait
					return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
				}
			}
		} else {
			// Still awaiting approval (draft or duplicate), requeue
			if !jobStatusChanged(&job.Status, updated) {
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}
			job.Status = *updated
			if err := r.Status().Update(ctx, &job); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
	}

	// Check Kubernetes Job status if we're in Dispatching state (for TektonJob pipeline)
	if updated.State == wikiv1alpha1.TranslationJobStateDispatching {
		logger.Info("checking Kubernetes Job status for dispatched job", "job", job.Name)
		// Look for the Kubernetes Job created by the dispatcher
		// Job name format: translation-{TranslationJob.Name}
		k8sJobName := fmt.Sprintf("translation-%s", job.Name)
		var k8sJob batchv1.Job
		if err := r.Get(ctx, types.NamespacedName{Namespace: job.Namespace, Name: k8sJobName}, &k8sJob); err != nil {
			if errors.IsNotFound(err) {
				// Job not found yet, might still be creating - requeue
				logger.Info("Kubernetes Job not found yet, waiting", "k8sJob", k8sJobName, "job", job.Name)
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}
			logger.Error(err, "failed to get Kubernetes Job", "k8sJob", k8sJobName, "job", job.Name)
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}

		// Check Job status
		if k8sJob.Status.Succeeded > 0 {
			// Job completed successfully
			logger.Info("Kubernetes Job completed successfully", "k8sJob", k8sJobName, "job", job.Name)
			updated.State = wikiv1alpha1.TranslationJobStateCompleted
			updated.FinishedAt = &now
			updated.Message = "Translation job completed successfully"
			meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
				Type:               "Ready",
				Status:             metav1.ConditionTrue,
				Reason:             "Completed",
				Message:            "Translation job completed successfully",
				LastTransitionTime: now,
			})
		} else if k8sJob.Status.Failed > 0 {
			// Job failed - check pod status for more details
			logger.Info("Kubernetes Job failed", "k8sJob", k8sJobName, "job", job.Name, "failed", k8sJob.Status.Failed)
			
			// Get pods for this job to check for ImagePullBackOff or other pod-level errors
			var pods corev1.PodList
			if err := r.List(ctx, &pods, client.InNamespace(job.Namespace), client.MatchingLabels{"job-name": k8sJobName}); err == nil {
				for _, pod := range pods.Items {
					// Check pod container statuses for errors
					for _, containerStatus := range pod.Status.ContainerStatuses {
						if containerStatus.State.Waiting != nil {
							reason := containerStatus.State.Waiting.Reason
							message := containerStatus.State.Waiting.Message
							if reason == "ImagePullBackOff" || reason == "ErrImagePull" {
								logger.Error(nil, "Pod failed to pull image", "pod", pod.Name, "reason", reason, "message", message)
							}
						}
						if containerStatus.State.Terminated != nil && containerStatus.State.Terminated.ExitCode != 0 {
							logger.Info("Pod container terminated with error", "pod", pod.Name, "exitCode", containerStatus.State.Terminated.ExitCode, "reason", containerStatus.State.Terminated.Reason, "message", containerStatus.State.Terminated.Message)
						}
					}
					// Also check pod phase
					if pod.Status.Phase == corev1.PodFailed {
						logger.Info("Pod in Failed phase", "pod", pod.Name, "reason", pod.Status.Reason, "message", pod.Status.Message)
					}
				}
			}
			
			// Try to get failure message from job conditions
			failureMessage := "Translation job failed"
			for _, condition := range k8sJob.Status.Conditions {
				if condition.Type == batchv1.JobFailed && condition.Status == "True" {
					failureMessage = condition.Message
					if failureMessage == "" {
						failureMessage = condition.Reason
					}
					break
				}
			}
			updated.State = wikiv1alpha1.TranslationJobStateFailed
			updated.FinishedAt = &now
			updated.Message = failureMessage
			meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				Reason:             "JobFailed",
				Message:            failureMessage,
				LastTransitionTime: now,
			})
		} else {
			// Job still running - but check if pods are stuck (e.g., ImagePullBackOff)
			// This helps detect issues even before the job is marked as failed
			var pods corev1.PodList
			if err := r.List(ctx, &pods, client.InNamespace(job.Namespace), client.MatchingLabels{"job-name": k8sJobName}); err == nil {
				for _, pod := range pods.Items {
					for _, containerStatus := range pod.Status.ContainerStatuses {
						if containerStatus.State.Waiting != nil {
							reason := containerStatus.State.Waiting.Reason
							if reason == "ImagePullBackOff" || reason == "ErrImagePull" {
								// Pod is stuck trying to pull image - mark job as failed
								logger.Error(nil, "Pod stuck in ImagePullBackOff, marking job as failed", "pod", pod.Name, "reason", reason, "message", containerStatus.State.Waiting.Message)
								updated.State = wikiv1alpha1.TranslationJobStateFailed
								updated.FinishedAt = &now
								updated.Message = fmt.Sprintf("Failed to pull image: %s - %s", reason, containerStatus.State.Waiting.Message)
								meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
									Type:               "Ready",
									Status:             metav1.ConditionFalse,
									Reason:             "ImagePullFailed",
									Message:            updated.Message,
									LastTransitionTime: now,
								})
								// Break out of loops and continue to status update
								break
							}
						}
					}
					// If we set updated.State to Failed above, break out of pod loop
					if updated.State == wikiv1alpha1.TranslationJobStateFailed {
						break
					}
				}
			}
			
			// If we detected ImagePullBackOff and marked job as failed, continue to status update
			if updated.State == wikiv1alpha1.TranslationJobStateFailed {
				// Status already set above, continue to update
			} else {
				// Job still running normally, requeue to check again
				logger.V(1).Info("Kubernetes Job still running", "k8sJob", k8sJobName, "job", job.Name,
					"active", k8sJob.Status.Active, "succeeded", k8sJob.Status.Succeeded, "failed", k8sJob.Status.Failed)
				return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
			}
		}
	}

	// Check if job is in Queued state (check both updated and current status)
	// After validation, we update status to Queued, so check current status first
	currentState := job.Status.State
	if updated.State != "" {
		currentState = updated.State
	}
	
	if currentState == wikiv1alpha1.TranslationJobStateQueued {
		// Check if this is a diagnostic job - diagnostic jobs always use dispatcher (runner)
		isDiagnostic := job.Labels["glooscap.dasmlab.org/diagnostic"] == "true" ||
			job.Spec.Parameters["diagnostic"] == "true"

		// Check if job explicitly requests TektonJob pipeline
		useDispatcher := job.Spec.Pipeline == wikiv1alpha1.TranslationPipelineModeTektonJob || isDiagnostic

		// Get current nanabush client (supports runtime reconfiguration)
		var currentNanabush *nanabush.Client
		if r.GetNanabushClient != nil {
			currentNanabush = r.GetNanabushClient()
		} else {
			currentNanabush = r.Nanabush // Fallback to direct reference
		}

		// Use dispatcher if requested, otherwise use gRPC to Nanabush if available
		if useDispatcher && r.Dispatcher != nil {
			logger.Info("dispatching translation job to runner", "job", job.Name, "mode", job.Spec.Pipeline)
			// Use dispatcher (runner) for TektonJob pipeline or diagnostic jobs
			mode := vllm.ModeFromString(string(job.Spec.Pipeline))
			if mode == "" {
				mode = vllm.ModeTektonJob
			}
			dispatchErr := r.Dispatcher.Dispatch(ctx, vllm.Request{
				JobName:      job.Name,
				Namespace:    job.Namespace,
				PageID:       job.Spec.Source.PageID,
				LanguageTag:  languageTagForJob(&job),
				SourceTarget: job.Spec.Source.TargetRef,
				Mode:         mode,
			})
			if dispatchErr != nil {
				logger.Error(dispatchErr, "failed to dispatch translation job", "job", job.Name)
				meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
					Type:               "Ready",
					Status:             metav1.ConditionFalse,
					Reason:             "DispatchFailed",
					Message:            dispatchErr.Error(),
					LastTransitionTime: now,
				})
				updated.State = wikiv1alpha1.TranslationJobStateFailed
				updated.Message = dispatchErr.Error()
				updated.FinishedAt = &now
			} else {
				logger.Info("translation job dispatched successfully", "job", job.Name, "k8sJob", fmt.Sprintf("translation-%s", job.Name))
				updated.State = wikiv1alpha1.TranslationJobStateDispatching
				meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
					Type:               "Ready",
					Status:             metav1.ConditionFalse,
					Reason:             "Dispatching",
					Message:            "Translation dispatched to runner",
					LastTransitionTime: now,
				})
				updated.Message = "Dispatch accepted by translation runner"
			}
		} else if currentNanabush != nil {
			// Get source page content on-the-fly
			var sourcePage *catalog.Page
			var sourceClient *outline.Client
			if r.Catalogue != nil && r.OutlineClient != nil {
				targetID := fmt.Sprintf("%s/%s", sourceTarget.Namespace, sourceTarget.Name)
				pages := r.Catalogue.List(targetID)
				for _, page := range pages {
					if page.ID == job.Spec.Source.PageID {
						sourcePage = page
						break
					}
				}

				// Create Outline client for source target
				client, err := r.OutlineClient.New(ctx, r.Client, &sourceTarget)
				if err != nil {
					logger.Error(err, "failed to create Outline client for source")
				} else {
					sourceClient = client
				}
			}

			// Pre-flight: Check title only first
			if sourcePage != nil && currentNanabush != nil {
				checkResp, err := currentNanabush.CheckTitle(ctx, nanabush.CheckTitleRequest{
					Title:          sourcePage.Title,
					LanguageTag:    languageTagForJob(&job),
					SourceLanguage: sourcePage.Language,
				})
				if err != nil {
					logger.Error(err, "title check failed", "title", sourcePage.Title)
					meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
						Type:               "Ready",
						Status:             metav1.ConditionFalse,
						Reason:             "PreflightFailed",
						Message:            fmt.Sprintf("Title check failed: %v", err),
						LastTransitionTime: now,
					})
					updated.State = wikiv1alpha1.TranslationJobStateFailed
					updated.Message = fmt.Sprintf("Pre-flight check failed: %v", err)
					updated.FinishedAt = &now
				} else if !checkResp.Ready {
					meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
						Type:               "Ready",
						Status:             metav1.ConditionFalse,
						Reason:             "NotReady",
						Message:            checkResp.Message,
						LastTransitionTime: now,
					})
					updated.State = wikiv1alpha1.TranslationJobStateFailed
					updated.Message = checkResp.Message
					updated.FinishedAt = &now
				} else {
					// Pre-flight passed, now fetch full content and translate
					logger.Info("pre-flight check passed", "estimated_time", checkResp.EstimatedTimeSeconds)

					// Fetch page content on-the-fly
					var pageContent *outline.PageContent
					var templateContent *outline.PageContent
					if sourceClient != nil {
						content, err := sourceClient.GetPageContent(ctx, job.Spec.Source.PageID)
						if err != nil {
							logger.Error(err, "failed to fetch page content")
							meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
								Type:               "Ready",
								Status:             metav1.ConditionFalse,
								Reason:             "ContentFetchFailed",
								Message:            fmt.Sprintf("Failed to fetch page content: %v", err),
								LastTransitionTime: now,
							})
							updated.State = wikiv1alpha1.TranslationJobStateFailed
							updated.Message = fmt.Sprintf("Failed to fetch page content: %v", err)
							updated.FinishedAt = &now
						} else {
							pageContent = content

							// Fetch template if available
							if sourcePage.Template != "" {
								template, err := sourceClient.GetTemplate(ctx, sourcePage.Template)
								if err == nil {
									templateContent = template
								}
							}
						}
					}

					if pageContent != nil {
						// Build gRPC request
						grpcReq := nanabush.TranslateRequest{
							JobID:     job.Name,
							Namespace: job.Namespace,
							Primitive: "doc-translate",
							Document: &nanabush.DocumentContent{
								Title:    pageContent.Title,
								Markdown: pageContent.Markdown,
								Slug:     pageContent.Slug,
								Metadata: map[string]string{
									"collection": sourcePage.Collection,
									"template":   sourcePage.Template,
								},
							},
							SourceLanguage: sourcePage.Language,
							TargetLanguage: languageTagForJob(&job),
							SourceWikiURI:  sourceTarget.Spec.URI,
							PageID:         job.Spec.Source.PageID,
							PageSlug:       sourcePage.Slug,
						}

						if templateContent != nil {
							grpcReq.TemplateHelper = &nanabush.DocumentContent{
								Title:    templateContent.Title,
								Markdown: templateContent.Markdown,
								Slug:     templateContent.Slug,
							}
						}

						// Call translation service via gRPC (supports both Nanabush and Iskoces)
						updated.State = wikiv1alpha1.TranslationJobStateDispatching
						meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
							Type:               "Ready",
							Status:             metav1.ConditionFalse,
							Reason:             "Translating",
							Message:            "Translation in progress",
							LastTransitionTime: now,
						})

						// TODO: This should be async, but for now we'll do it synchronously
						// In production, this should dispatch to a Tekton job or async worker
						// Use a longer timeout for translation (5 minutes) to handle large documents
						translateCtx, translateCancel := context.WithTimeout(ctx, 5*time.Minute)
						defer translateCancel()
						translateResp, err := currentNanabush.Translate(translateCtx, grpcReq)
						if err != nil {
							logger.Error(err, "translation failed")
							meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
								Type:               "Ready",
								Status:             metav1.ConditionFalse,
								Reason:             "TranslationFailed",
								Message:            fmt.Sprintf("Translation failed: %v", err),
								LastTransitionTime: now,
							})
							updated.State = wikiv1alpha1.TranslationJobStateFailed
							updated.Message = fmt.Sprintf("Translation failed: %v", err)
							updated.FinishedAt = &now
						} else if !translateResp.Success {
							meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
								Type:               "Ready",
								Status:             metav1.ConditionFalse,
								Reason:             "TranslationFailed",
								Message:            translateResp.ErrorMessage,
								LastTransitionTime: now,
							})
							updated.State = wikiv1alpha1.TranslationJobStateFailed
							updated.Message = translateResp.ErrorMessage
							updated.FinishedAt = &now
						} else {
							// Translation succeeded - update status
							updated.State = wikiv1alpha1.TranslationJobStatePublishing
							meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
								Type:               "Ready",
								Status:             metav1.ConditionFalse,
								Reason:             "TranslationComplete",
								Message:            "Translation completed, publishing to destination",
								LastTransitionTime: now,
							})
							updated.Message = fmt.Sprintf("Translation completed (tokens: %d, time: %.2fs)", translateResp.TokensUsed, translateResp.InferenceTimeSeconds)
							logger.Info("translation completed", "tokens", translateResp.TokensUsed, "time", translateResp.InferenceTimeSeconds)

							// Publish translated content to destination wiki
							// SAFETY CHECKS:
							// 1. NEVER overwrite existing pages - create unique pages if needed
							// 2. Always prefix with "AUTOTRANSLATED--> <SOURCE TITLE>"
							// 3. Create at same level as source (same collection/parent)
							// 4. NEVER modify source pages

							// Get destination target (re-fetch to ensure we have it)
							destTargetRef := job.Spec.Source.TargetRef
							if job.Spec.Destination != nil && job.Spec.Destination.TargetRef != "" {
								destTargetRef = job.Spec.Destination.TargetRef
							}
							var destTarget wikiv1alpha1.WikiTarget
							if err := r.Get(ctx, client.ObjectKey{Namespace: job.Namespace, Name: destTargetRef}, &destTarget); err != nil {
								logger.Error(err, "failed to get destination target")
								meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
									Type:               "Ready",
									Status:             metav1.ConditionFalse,
									Reason:             "PublishFailed",
									Message:            fmt.Sprintf("Failed to get destination target: %v", err),
									LastTransitionTime: now,
								})
								updated.State = wikiv1alpha1.TranslationJobStateFailed
								updated.Message = fmt.Sprintf("Failed to get destination target: %v", err)
								updated.FinishedAt = &now
							} else {
								// Get destination client
								destClient, err := r.OutlineClient.New(ctx, r.Client, &destTarget)
								if err != nil {
									logger.Error(err, "failed to create destination client")
									meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
										Type:               "Ready",
										Status:             metav1.ConditionFalse,
										Reason:             "PublishFailed",
										Message:            fmt.Sprintf("Failed to create destination client: %v", err),
										LastTransitionTime: now,
									})
									updated.State = wikiv1alpha1.TranslationJobStateFailed
									updated.Message = fmt.Sprintf("Failed to create destination client: %v", err)
									updated.FinishedAt = &now
								} else {
									// Get source page info to determine collection/parent
									var sourceCollectionID string
									sourcePageTitle := ""
									if sourcePage != nil {
										sourcePageTitle = sourcePage.Title
										// Use collection ID from source target status if available
										// The sourcePage.Collection is the collection name, we need the ID
										if sourceTarget.Status.CollectionID != "" && sourceTarget.Status.CollectionName != "" {
											// If the source page's collection name matches the cached collection name, use the cached ID
											if sourcePage.Collection == sourceTarget.Status.CollectionName {
												sourceCollectionID = sourceTarget.Status.CollectionID
												logger.V(1).Info("using cached collection ID for source page", "collectionID", sourceCollectionID, "collectionName", sourceTarget.Status.CollectionName)
											}
										}
										// If we still don't have a collection ID, try to find it from Outline
										// This should be rare - only if the page is in a different collection than the cached one
										if sourceCollectionID == "" && sourceClient != nil {
											// Use collection constraint from source WikiTarget if available to limit search
											var sourcePages []outline.PageSummary
											if sourceTarget.Status.CollectionID != "" {
												sourcePages, err = sourceClient.ListPages(ctx, sourceTarget.Status.CollectionID)
											} else {
												sourcePages, err = sourceClient.ListPages(ctx)
											}
											if err == nil {
												for _, sp := range sourcePages {
													if sp.ID == job.Spec.Source.PageID {
														// The page's Collection field is the name, not the ID
														// We'd need to look up the ID, but for now, if it matches our cached collection, use that
														if sp.Collection == sourceTarget.Status.CollectionName && sourceTarget.Status.CollectionID != "" {
															sourceCollectionID = sourceTarget.Status.CollectionID
														}
														break
													}
												}
											}
										}
									}

									// Build page title with AUTOTRANSLATED prefix
									baseTitle := sourcePageTitle
									if baseTitle == "" {
										baseTitle = "Untitled Page"
									}
									translatedTitle := fmt.Sprintf("AUTOTRANSLATED--> %s", baseTitle)

									// Check if a page with this exact title already exists
									// Use collection constraint from destination WikiTarget if available
									var destPages []outline.PageSummary
									if destTarget.Status.CollectionID != "" {
										destPages, err = destClient.ListPages(ctx, destTarget.Status.CollectionID)
										logger.V(1).Info("checking title uniqueness in destination collection", "collectionID", destTarget.Status.CollectionID, "collectionName", destTarget.Status.CollectionName)
									} else {
										destPages, err = destClient.ListPages(ctx)
										logger.V(1).Info("checking title uniqueness in all destination pages (no collection constraint)")
									}
									uniqueTitle := translatedTitle
									counter := 1
									if err == nil {
										for {
											titleExists := false
											for _, dp := range destPages {
												if dp.Title == uniqueTitle {
													titleExists = true
													break
												}
											}
											if !titleExists {
												break
											}
											// Title exists - make it unique
											uniqueTitle = fmt.Sprintf("AUTOTRANSLATED--> %s (%d)", baseTitle, counter)
											counter++
											if counter > 100 {
												// Safety limit
												logger.Error(nil, "unable to generate unique title after 100 attempts")
												break
											}
										}
									}

									if uniqueTitle != translatedTitle {
										logger.Info("using unique title to avoid overwrite",
											"original", translatedTitle,
											"unique", uniqueTitle)
									}

									// Create the page - NEVER overwrite, always create new
									createReq := outline.CreatePageRequest{
										Title:        uniqueTitle,
										Text:         translateResp.TranslatedMarkdown,
										CollectionID: sourceCollectionID, // Same collection as source
									}

									createResp, err := destClient.CreatePage(ctx, createReq)
									if err != nil {
										logger.Error(err, "failed to create translated page",
											"title", uniqueTitle)
										meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
											Type:               "Ready",
											Status:             metav1.ConditionFalse,
											Reason:             "PublishFailed",
											Message:            fmt.Sprintf("Failed to create page: %v", err),
											LastTransitionTime: now,
										})
										updated.State = wikiv1alpha1.TranslationJobStateFailed
										updated.Message = fmt.Sprintf("Failed to create translated page: %v", err)
										updated.FinishedAt = &now
									} else {
										logger.Info("translated page created successfully",
											"page_id", createResp.Data.ID,
											"title", uniqueTitle,
											"slug", createResp.Data.Slug)
										updated.State = wikiv1alpha1.TranslationJobStateCompleted
										updated.FinishedAt = &now
										updated.Message = fmt.Sprintf("Translation completed and published (page: %s)", createResp.Data.Slug)
										meta.SetStatusCondition(&updated.Conditions, metav1.Condition{
											Type:               "Ready",
											Status:             metav1.ConditionTrue,
											Reason:             "Completed",
											Message:            fmt.Sprintf("Translation published as: %s", uniqueTitle),
											LastTransitionTime: now,
										})

										// Build page URL from destination target
										pageURL := ""
										if destTarget.Spec.URI != "" {
											// Construct URL: baseURI/doc/slug
											pageURL = fmt.Sprintf("%s/doc/%s", strings.TrimSuffix(destTarget.Spec.URI, "/"), createResp.Data.Slug)
										}

										// Send translation_complete SSE event
										if r.TranslationJobEventCh != nil {
											select {
											case r.TranslationJobEventCh <- TranslationJobEvent{
												Type:      "translation_complete",
												JobName:   job.Name,
												PageURL:   pageURL,
												PageID:    createResp.Data.ID,
												PageTitle: uniqueTitle,
												State:     string(updated.State),
												Message:   updated.Message,
											}:
											default:
												// Channel full, skip (non-blocking)
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	if !jobStatusChanged(&job.Status, updated) {
		return ctrl.Result{}, nil
	}

	job.Status = *updated
	if err := r.Status().Update(ctx, &job); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("updated translation job status", "state", job.Status.State)
	r.Recorder.Event(&job, "Normal", string(job.Status.State), job.Status.Message)

	if r.Jobs != nil {
		r.Jobs.Update(&job)
	}

	// Do NOT requeue failed jobs - they will just create more pods and fail again
	// Only requeue dispatching jobs to check Kubernetes Job status
	requeue := ctrl.Result{}
	if updated.State == wikiv1alpha1.TranslationJobStateDispatching {
		// Dispatching jobs should be requeued to check Kubernetes Job status
		logger.V(1).Info("job dispatching, requeuing to check status", "job", job.Name)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	} else if updated.State == wikiv1alpha1.TranslationJobStateFailed {
		// Failed jobs should NOT be requeued - they will just fail again and create more pods
		// Return empty result to stop reconciliation
		logger.Info("job failed, not requeuing to prevent pod accumulation", "state", job.Status.State, "message", job.Status.Message)
		return ctrl.Result{}, nil
	} else if updated.State == wikiv1alpha1.TranslationJobStateCompleted {
		// Completed jobs should NOT be requeued
		logger.Info("job completed, not requeuing", "job", job.Name)
		return ctrl.Result{}, nil
	}

	return requeue, nil
}

func languageTagForJob(job *wikiv1alpha1.TranslationJob) string {
	if job.Spec.Destination != nil && job.Spec.Destination.LanguageTag != "" {
		return job.Spec.Destination.LanguageTag
	}
	if lang, ok := job.Spec.Parameters["languageTag"]; ok && lang != "" {
		return lang
	}
	return "fr-CA"
}

// SetupWithManager sets up the controller with the Manager.
func (r *TranslationJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&wikiv1alpha1.TranslationJob{}).
		Named("translationjob").
		// Limit concurrent reconciles to prevent overwhelming the translation service
		// This helps when many jobs are queued after a restart
		WithOptions(controller.Options{MaxConcurrentReconciles: 3}).
		Complete(r)
}

func jobStatusChanged(previous *wikiv1alpha1.TranslationJobStatus, updated *wikiv1alpha1.TranslationJobStatus) bool {
	return !equality.Semantic.DeepEqual(previous, updated)
}
