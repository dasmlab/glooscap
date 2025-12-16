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
	"sync"
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
	"github.com/dasmlab/glooscap-operator/pkg/nanabush"
)

// TranslationServiceReconciler reconciles a TranslationService object
type TranslationServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Recorder record.EventRecorder

	// NanabushClientMu protects access to the nanabush client
	NanabushClientMu *sync.RWMutex
	// NanabushClient is the shared nanabush client instance
	NanabushClient **nanabush.Client
	// NanabushStatusCh is a channel to trigger SSE broadcasts when status changes
	NanabushStatusCh chan<- struct{}
	// CreateTranslationServiceClient is a function to create a new translation service client
	CreateTranslationServiceClient func(address, serviceType string, secure bool) (*nanabush.Client, error)
}

// +kubebuilder:rbac:groups=wiki.glooscap.dasmlab.org,resources=translationservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=wiki.glooscap.dasmlab.org,resources=translationservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=wiki.glooscap.dasmlab.org,resources=translationservices/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop
func (r *TranslationServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("translationservice", req.NamespacedName)

	var ts wikiv1alpha1.TranslationService
	if err := r.Get(ctx, req.NamespacedName, &ts); err != nil {
		if errors.IsNotFound(err) {
			// TranslationService was deleted - close and clear the client
			logger.Info("TranslationService deleted, closing client")
			r.NanabushClientMu.Lock()
			if *r.NanabushClient != nil {
				if err := (*r.NanabushClient).Close(); err != nil {
					logger.Error(err, "error closing translation service client")
				}
				*r.NanabushClient = nil
			}
			r.NanabushClientMu.Unlock()
			
			// Trigger SSE broadcast
			select {
			case r.NanabushStatusCh <- struct{}{}:
			default:
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	status := ts.Status.DeepCopy()
	now := metav1.Now()

	// Check if we need to recreate the client
	// We'll track the last applied spec in an annotation to detect changes
	lastAppliedSpec := ""
	if ts.Annotations != nil {
		lastAppliedSpec = ts.Annotations["glooscap.dasmlab.org/last-applied-spec"]
	}
	currentSpec := fmt.Sprintf("%s|%s|%v", ts.Spec.Address, ts.Spec.Type, ts.Spec.Secure)
	
	specChanged := false
	r.NanabushClientMu.RLock()
	hasClient := *r.NanabushClient != nil
	r.NanabushClientMu.RUnlock()

	// Check if spec has changed or client doesn't exist
	if !hasClient || lastAppliedSpec != currentSpec {
		specChanged = true
		logger.Info("TranslationService spec changed or client missing, recreating client",
			"address", ts.Spec.Address,
			"type", ts.Spec.Type,
			"secure", ts.Spec.Secure,
			"has_client", hasClient,
			"last_spec", lastAppliedSpec,
			"current_spec", currentSpec)
	}

	if specChanged {
		// Close old client
		r.NanabushClientMu.Lock()
		oldClient := *r.NanabushClient
		*r.NanabushClient = nil
		r.NanabushClientMu.Unlock()

		if oldClient != nil {
			logger.Info("Closing old translation service client...")
			if err := oldClient.Close(); err != nil {
				logger.Error(err, "error closing old translation service client")
			}
		}

		// Create new client
		if ts.Spec.Address != "" {
			logger.Info("Creating new translation service client...",
				"address", ts.Spec.Address,
				"type", ts.Spec.Type,
				"secure", ts.Spec.Secure)

			client, err := r.CreateTranslationServiceClient(ts.Spec.Address, ts.Spec.Type, ts.Spec.Secure)
			if err != nil {
				logger.Error(err, "failed to create translation service client")
				meta.SetStatusCondition(&status.Conditions, metav1.Condition{
					Type:               "Ready",
					Status:             metav1.ConditionFalse,
					Reason:             "ClientCreationFailed",
					Message:            fmt.Sprintf("Failed to create client: %v", err),
					LastTransitionTime: now,
				})
				status.Status = "error"
				status.Connected = false
				status.Registered = false
				status.ClientID = ""

				if !translationServiceStatusChanged(&ts.Status, status) {
					return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
				}
				ts.Status = *status
				if err := r.Status().Update(ctx, &ts); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}

			// Update client atomically
			r.NanabushClientMu.Lock()
			*r.NanabushClient = client
			r.NanabushClientMu.Unlock()

			// Wait for registration to complete (up to 5 seconds)
			maxWait := 5 * time.Second
			checkInterval := 500 * time.Millisecond
			waited := time.Duration(0)
			var finalStatus nanabush.Status

			for waited < maxWait {
				time.Sleep(checkInterval)
				waited += checkInterval
				finalStatus = client.Status()
				if finalStatus.ClientID != "" {
					logger.Info("Client registered successfully",
						"client_id", finalStatus.ClientID,
						"connected", finalStatus.Connected,
						"registered", finalStatus.Registered,
						"waited_ms", waited.Milliseconds())
					break
				}
			}

			if finalStatus.ClientID == "" {
				logger.Info("Client registration still in progress after wait",
					"connected", finalStatus.Connected,
					"registered", finalStatus.Registered,
					"status", finalStatus.Status)
			}

			// Trigger SSE broadcast
			select {
			case r.NanabushStatusCh <- struct{}{}:
			default:
			}

			// Update annotation to track last applied spec
			if ts.Annotations == nil {
				ts.Annotations = make(map[string]string)
			}
			ts.Annotations["glooscap.dasmlab.org/last-applied-spec"] = currentSpec
			if err := r.Update(ctx, &ts); err != nil {
				logger.Error(err, "failed to update TranslationService annotation")
				// Continue anyway - annotation update failure is not critical
			}
		}
	}

	// Update status from current client
	r.NanabushClientMu.RLock()
	var clientStatus nanabush.Status
	if *r.NanabushClient != nil {
		clientStatus = (*r.NanabushClient).Status()
	} else {
		clientStatus = nanabush.Status{
			Connected:  false,
			Registered: false,
			Status:     "error",
		}
	}
	r.NanabushClientMu.RUnlock()

	// Update status fields
	status.ClientID = clientStatus.ClientID
	status.Connected = clientStatus.Connected
	status.Registered = clientStatus.Registered
	status.Status = clientStatus.Status
	status.MissedHeartbeats = clientStatus.MissedHeartbeats
	status.HeartbeatIntervalSeconds = int(clientStatus.HeartbeatInterval) // HeartbeatInterval is already int64 in seconds

	if !clientStatus.LastHeartbeat.IsZero() {
		lastHeartbeat := metav1.NewTime(clientStatus.LastHeartbeat)
		status.LastHeartbeat = &lastHeartbeat
	} else {
		status.LastHeartbeat = nil
	}

	// Update conditions
	if status.Connected && status.Registered {
		meta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             "Connected",
			Message:            fmt.Sprintf("Connected and registered with client ID: %s", status.ClientID),
			LastTransitionTime: now,
		})
	} else if status.Connected && !status.Registered {
		meta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "Connecting",
			Message:            "Connected but not yet registered",
			LastTransitionTime: now,
		})
	} else {
		meta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "Disconnected",
			Message:            "Not connected to translation service",
			LastTransitionTime: now,
		})
	}

	// Only update if status changed
	if !translationServiceStatusChanged(&ts.Status, status) {
		// Requeue periodically to update status from client
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	ts.Status = *status
	if err := r.Status().Update(ctx, &ts); err != nil {
		return ctrl.Result{}, err
	}

	// Trigger SSE broadcast on status update
	select {
	case r.NanabushStatusCh <- struct{}{}:
	default:
	}

	logger.Info("TranslationService status updated",
		"client_id", status.ClientID,
		"connected", status.Connected,
		"registered", status.Registered,
		"status", status.Status)

	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func translationServiceStatusChanged(oldStatus *wikiv1alpha1.TranslationServiceStatus, newStatus *wikiv1alpha1.TranslationServiceStatus) bool {
	return !equality.Semantic.DeepEqual(oldStatus, newStatus)
}

// SetupWithManager sets up the controller with the Manager.
func (r *TranslationServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&wikiv1alpha1.TranslationService{}).
		Named("translationservice").
		Complete(r)
}

