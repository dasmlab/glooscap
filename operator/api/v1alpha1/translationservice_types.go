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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TranslationServiceSpec defines the desired state of TranslationService.
type TranslationServiceSpec struct {
	// Address is the gRPC address of the translation service (e.g., iskoces-service.iskoces.svc.cluster.local:50051)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=512
	Address string `json:"address"`

	// Type specifies the translation service type (e.g., "iskoces", "nanabush")
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=iskoces;nanabush
	Type string `json:"type"`

	// Secure enables TLS/mTLS for the connection
	// +optional
	// +kubebuilder:default=false
	Secure bool `json:"secure,omitempty"`
}

// TranslationServiceStatus defines the observed state of TranslationService.
type TranslationServiceStatus struct {
	// ClientID is the client identifier assigned by the translation service after registration
	// +optional
	ClientID string `json:"clientId,omitempty"`

	// Connected indicates whether the gRPC connection is established
	// +optional
	// +kubebuilder:default=false
	Connected bool `json:"connected,omitempty"`

	// Registered indicates whether the client has successfully registered with the service
	// +optional
	// +kubebuilder:default=false
	Registered bool `json:"registered,omitempty"`

	// Status is the overall connection status (e.g., "healthy", "warning", "error")
	// +optional
	Status string `json:"status,omitempty"`

	// LastHeartbeat records the timestamp of the last heartbeat received
	// +optional
	LastHeartbeat *metav1.Time `json:"lastHeartbeat,omitempty"`

	// MissedHeartbeats counts how many heartbeats have been missed
	// +optional
	MissedHeartbeats int `json:"missedHeartbeats,omitempty"`

	// HeartbeatIntervalSeconds is the interval between heartbeats in seconds
	// +optional
	HeartbeatIntervalSeconds int `json:"heartbeatIntervalSeconds,omitempty"`

	// Conditions represent the latest available observations of the service's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Address",type="string",JSONPath=".spec.address",description="Translation service address"
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type",description="Service type"
// +kubebuilder:printcolumn:name="Connected",type="boolean",JSONPath=".status.connected",description="Connection status"
// +kubebuilder:printcolumn:name="Registered",type="boolean",JSONPath=".status.registered",description="Registration status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.status",description="Overall status"
// +kubebuilder:printcolumn:name="ClientID",type="string",JSONPath=".status.clientId",description="Client ID"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status",description="Ready condition"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// TranslationService is the Schema for the translationservices API.
type TranslationService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of TranslationService
	Spec TranslationServiceSpec `json:"spec"`

	// Status defines the observed state of TranslationService
	// +optional
	Status TranslationServiceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TranslationServiceList contains a list of TranslationService.
type TranslationServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TranslationService `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TranslationService{}, &TranslationServiceList{})
}
