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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// WikiTargetSpec defines the desired state of WikiTarget
type WikiTargetSpec struct {
	// URI is the base URL of the Outline wiki to synchronise.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Format=uri
	// +kubebuilder:validation:MaxLength=512
	URI string `json:"uri"`

	// ServiceAccountSecretRef references the Kubernetes secret containing API credentials.
	// +kubebuilder:validation:Required
	ServiceAccountSecretRef SecretKeyRef `json:"serviceAccountSecretRef"`

	// Mode determines how this target will be used during publication.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=ReadOnly;ReadWrite;PushOnly
	Mode WikiTargetMode `json:"mode"`

	// Sync configures the cadence of page discovery.
	// +optional
	Sync *WikiTargetSyncSpec `json:"sync,omitempty"`

	// TranslationDefaults specifies default destination parameters when creating TranslationJobs.
	// +optional
	TranslationDefaults *TranslationDefaults `json:"translationDefaults,omitempty"`
}

// WikiTargetStatus defines the observed state of WikiTarget.
type WikiTargetStatus struct {
	// LastSyncTime records the most recent successful discovery run.
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// CatalogRevision increments each time the catalogue is refreshed.
	// +optional
	CatalogRevision int64 `json:"catalogRevision,omitempty"`

	// Conditions represent the latest available observations of a target's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// WikiTargetMode enumerates supported publication modes.
type WikiTargetMode string

const (
	WikiTargetModeReadOnly WikiTargetMode = "ReadOnly"
	WikiTargetModeReadWrite WikiTargetMode = "ReadWrite"
	WikiTargetModePushOnly  WikiTargetMode = "PushOnly"
)

// WikiTargetSyncSpec controls discovery scheduling.
type WikiTargetSyncSpec struct {
	// Interval represents how often discovery should run.
	// +optional
	Interval *metav1.Duration `json:"interval,omitempty"`

	// FullRefreshInterval ensures a complete rescan at the provided cadence.
	// +optional
	FullRefreshInterval *metav1.Duration `json:"fullRefreshInterval,omitempty"`
}

// TranslationDefaults specifies default translation destinations.
type TranslationDefaults struct {
	// DestinationTarget optionally overrides the target wiki for translated content.
	// +optional
	DestinationTarget string `json:"destinationTarget,omitempty"`

	// DestinationPathPrefix adds a prefix (e.g., language code) for translated pages.
	// +optional
	DestinationPathPrefix string `json:"destinationPathPrefix,omitempty"`

	// LanguageTag sets the BCP 47 language code for translations.
	// +optional
	LanguageTag string `json:"languageTag,omitempty"`
}

// SecretKeyRef identifies a secret and optional key.
type SecretKeyRef struct {
	// Name of the secret.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Key within the secret data map. Defaults to "token".
	// +optional
	Key string `json:"key,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// WikiTarget is the Schema for the wikitargets API
type WikiTarget struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of WikiTarget
	// +required
	Spec WikiTargetSpec `json:"spec"`

	// status defines the observed state of WikiTarget
	// +optional
	Status WikiTargetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WikiTargetList contains a list of WikiTarget
type WikiTargetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WikiTarget `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WikiTarget{}, &WikiTargetList{})
}
