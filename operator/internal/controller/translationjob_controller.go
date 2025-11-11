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
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	wikiv1alpha1 "github.com/dasmlab/glooscap-operator/api/v1alpha1"
)

// TranslationJobReconciler reconciles a TranslationJob object
type TranslationJobReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=wiki.glooscap.dasmlab.org,resources=translationjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=wiki.glooscap.dasmlab.org,resources=translationjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=wiki.glooscap.dasmlab.org,resources=translationjobs/finalizers,verbs=update
// +kubebuilder:rbac:groups=wiki.glooscap.dasmlab.org,resources=wikitargets,verbs=get;list;watch

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
	}

	// Validate referenced WikiTarget exists to surface immediate feedback.
	if job.Spec.Source.TargetRef != "" {
		var target wikiv1alpha1.WikiTarget
		if err := r.Get(ctx, client.ObjectKey{Namespace: job.Namespace, Name: job.Spec.Source.TargetRef}, &target); err != nil {
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
			} else {
				return ctrl.Result{}, err
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

	requeue := ctrl.Result{}
	if job.Status.State == wikiv1alpha1.TranslationJobStateQueued {
		requeue.RequeueAfter = time.Minute
	}

	return requeue, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TranslationJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&wikiv1alpha1.TranslationJob{}).
		Named("translationjob").
		Complete(r)
}

func jobStatusChanged(previous *wikiv1alpha1.TranslationJobStatus, updated *wikiv1alpha1.TranslationJobStatus) bool {
	return !equality.Semantic.DeepEqual(previous, updated)
}
