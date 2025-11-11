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

// WikiTargetReconciler reconciles a WikiTarget object
type WikiTargetReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=wiki.glooscap.dasmlab.org,resources=wikitargets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=wiki.glooscap.dasmlab.org,resources=wikitargets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=wiki.glooscap.dasmlab.org,resources=wikitargets/finalizers,verbs=update

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

	meta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "DiscoveryPending",
		Message:            "Discovery worker initialising",
		LastTransitionTime: now,
	})

	if status.CatalogRevision == 0 {
		status.CatalogRevision = 1
	}

	if status.LastSyncTime == nil {
		status.LastSyncTime = &now
	}

	if !statusChanged(&target.Status, status) {
		return ctrl.Result{}, nil
	}

	target.Status = *status
	if err := r.Status().Update(ctx, &target); err != nil {
		return ctrl.Result{}, err
	}

	r.Recorder.Event(&target, "Normal", "DiscoveryPending", "WikiTarget queued for discovery scheduling")
	logger.Info("initialised WikiTarget status")

	return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
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
