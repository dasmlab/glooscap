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

// TranslationJobSpec defines the desired state of TranslationJob
type TranslationJobSpec struct {
	// Source identifies the wiki target and page to translate.
	// +kubebuilder:validation:Required
	Source TranslationSourceSpec `json:"source"`

	// Destination indicates where translated content should be published.
	// +optional
	Destination *TranslationDestinationSpec `json:"destination,omitempty"`

	// Pipeline selects the execution mode for the translation.
	// +kubebuilder:validation:Enum=InlineLLM;TektonJob
	// +kubebuilder:default=TektonJob
	Pipeline TranslationPipelineMode `json:"pipeline,omitempty"`

	// Parameters includes optional overrides for translation prompts or throttling.
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`
}

// TranslationJobStatus defines the observed state of TranslationJob.
type TranslationJobStatus struct {
	// State reflects the high-level lifecycle phase.
	// +kubebuilder:validation:Enum=Queued;Validating;AwaitingApproval;Dispatching;Running;Publishing;Completed;Failed
	// +optional
	State TranslationJobState `json:"state,omitempty"`

	// Message contains human-readable details about the current state.
	// +optional
	Message string `json:"message,omitempty"`

	// StartedAt records when processing began.
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// FinishedAt records when processing completed.
	// +optional
	FinishedAt *metav1.Time `json:"finishedAt,omitempty"`

	// AuditRef references an immutable audit log entry.
	// +optional
	AuditRef string `json:"auditRef,omitempty"`

	// Conditions provide granular status updates.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// DuplicateInfo contains information about a duplicate page found at destination.
	// +optional
	DuplicateInfo *DuplicateInfo `json:"duplicateInfo,omitempty"`
}

// DuplicateInfo describes a duplicate page found at the destination.
type DuplicateInfo struct {
	// PageID is the ID of the duplicate page at the destination.
	PageID string `json:"pageId"`
	// PageTitle is the title of the duplicate page.
	PageTitle string `json:"pageTitle"`
	// PageURI is the URI of the duplicate page.
	PageURI string `json:"pageUri"`
	// Message is a human-readable message about the duplicate.
	Message string `json:"message"`
}

// TranslationSourceSpec identifies source wiki content.
type TranslationSourceSpec struct {
	// TargetRef refers to the WikiTarget resource name.
	// +kubebuilder:validation:Required
	TargetRef string `json:"targetRef"`

	// PageID is the identifier of the Outline page to translate.
	// +kubebuilder:validation:Required
	PageID string `json:"pageId"`

	// Revision allows locking translation to a specific revision.
	// +optional
	Revision string `json:"revision,omitempty"`
}

// TranslationDestinationSpec configures where to publish translated content.
type TranslationDestinationSpec struct {
	// TargetRef overrides the target wiki; defaults to source target.
	// +optional
	TargetRef string `json:"targetRef,omitempty"`

	// PathPrefix ensures translated pages use a specific prefix (e.g., language code).
	// +optional
	PathPrefix string `json:"pathPrefix,omitempty"`

	// LanguageTag sets the desired language annotation.
	// +optional
	LanguageTag string `json:"languageTag,omitempty"`
}

// TranslationPipelineMode sets the execution backend.
type TranslationPipelineMode string

const (
	TranslationPipelineModeInlineLLM TranslationPipelineMode = "InlineLLM"
	TranslationPipelineModeTektonJob TranslationPipelineMode = "TektonJob"
)

// TranslationJobState enumerates job lifecycle states.
type TranslationJobState string

const (
	TranslationJobStateQueued           TranslationJobState = "Queued"
	TranslationJobStateValidating       TranslationJobState = "Validating"
	TranslationJobStateAwaitingApproval TranslationJobState = "AwaitingApproval"
	TranslationJobStateDispatching      TranslationJobState = "Dispatching"
	TranslationJobStateRunning          TranslationJobState = "Running"
	TranslationJobStatePublishing       TranslationJobState = "Publishing"
	TranslationJobStateCompleted        TranslationJobState = "Completed"
	TranslationJobStateFailed           TranslationJobState = "Failed"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// TranslationJob is the Schema for the translationjobs API
type TranslationJob struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of TranslationJob
	// +required
	Spec TranslationJobSpec `json:"spec"`

	// status defines the observed state of TranslationJob
	// +optional
	Status TranslationJobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TranslationJobList contains a list of TranslationJob
type TranslationJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TranslationJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TranslationJob{}, &TranslationJobList{})
}
