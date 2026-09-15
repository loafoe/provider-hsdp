/*
Copyright 2025 The Crossplane Authors.

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

package v1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// PropositionParameters are the configurable fields of an MDM Proposition.
type PropositionParameters struct {
	// Name of the proposition. Required.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Description of the proposition.
	// +optional
	Description *string `json:"description,omitempty"`

	// OrganizationID is the IAM organization GUID.
	// +kubebuilder:validation:Required
	// +crossplane:generate:reference:type=github.com/loafoe/provider-hsdp/apis/iam/v1.Organization
	// +crossplane:generate:reference:refFieldName=OrganizationRef
	// +crossplane:generate:reference:selectorFieldName=OrganizationSelector
	OrganizationID string `json:"organizationId"`

	// OrganizationRef references an IAM Organization.
	// +optional
	OrganizationRef *xpv1.NamespacedReference `json:"organizationRef,omitempty"`

	// OrganizationSelector selects an IAM Organization.
	// +optional
	OrganizationSelector *xpv1.NamespacedSelector `json:"organizationSelector,omitempty"`

	// PropositionGUID is the IAM proposition GUID (optional, for linking).
	// +optional
	PropositionGUID *string `json:"propositionGuid,omitempty"`

	// GlobalReferenceID is a globally unique reference identifier.
	// +kubebuilder:validation:Required
	GlobalReferenceID string `json:"globalReferenceId"`

	// Status of the proposition. Either DRAFT or ACTIVE.
	// +kubebuilder:validation:Enum=DRAFT;ACTIVE
	// +kubebuilder:default=ACTIVE
	// +optional
	Status *string `json:"status,omitempty"`
}

// PropositionObservation are the observable fields of an MDM Proposition.
type PropositionObservation struct {
	// ID is the MDM ID of the proposition.
	ID *string `json:"id,omitempty"`

	// Description as returned by DIP.
	Description *string `json:"description,omitempty"`

	// GlobalReferenceID as returned by DIP.
	GlobalReferenceID *string `json:"globalReferenceId,omitempty"`

	// Status of the proposition (DRAFT or ACTIVE) as returned by DIP.
	Status *string `json:"status,omitempty"`
}

// PropositionSpec defines the desired state of an MDM Proposition.
type PropositionSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              PropositionParameters `json:"forProvider"`
}

// PropositionStatus represents the observed state of an MDM Proposition.
type PropositionStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider                 PropositionObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// Proposition is the Schema for the MDM Proposition API.
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,hsdp}
type Proposition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              PropositionSpec   `json:"spec"`
	Status            PropositionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PropositionList contains a list of Proposition.
type PropositionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Proposition `json:"items"`
}

// Proposition type metadata.
var (
	PropositionKind             = reflect.TypeOf(Proposition{}).Name()
	PropositionGroupKind        = schema.GroupKind{Group: SchemeGroupVersion.Group, Kind: PropositionKind}.String()
	PropositionKindAPIVersion   = PropositionKind + "." + SchemeGroupVersion.String()
	PropositionGroupVersionKind = SchemeGroupVersion.WithKind(PropositionKind)
)

func init() {
	SchemeBuilder.Register(&Proposition{}, &PropositionList{})
}
