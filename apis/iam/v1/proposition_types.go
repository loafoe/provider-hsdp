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

// PropositionParameters are the configurable fields of a Proposition.
type PropositionParameters struct {
	// Name of the proposition. Immutable after creation.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name"`

	// Description of the proposition.
	// +optional
	Description *string `json:"description,omitempty"`

	// OrganizationID is the organization GUID this proposition belongs to.
	// +optional
	// +crossplane:generate:reference:type=Organization
	// +crossplane:generate:reference:refFieldName=OrganizationRef
	// +crossplane:generate:reference:selectorFieldName=OrganizationSelector
	OrganizationID *string `json:"organizationId,omitempty"`

	// OrganizationRef references an Organization.
	// +optional
	OrganizationRef *xpv1.NamespacedReference `json:"organizationRef,omitempty"`

	// OrganizationSelector selects an Organization.
	// +optional
	OrganizationSelector *xpv1.NamespacedSelector `json:"organizationSelector,omitempty"`

	// GlobalReferenceID is a global unique identifier. Immutable after creation.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="globalReferenceId is immutable"
	GlobalReferenceID string `json:"globalReferenceId"`
}

// PropositionObservation are the observable fields of a Proposition.
type PropositionObservation struct {
	// ID is the GUID of the proposition.
	ID *string `json:"id,omitempty"`

	// Description as returned by DIP.
	Description *string `json:"description,omitempty"`

	// GlobalReferenceID as returned by DIP.
	GlobalReferenceID *string `json:"globalReferenceId,omitempty"`
}

// PropositionSpec defines the desired state of a Proposition.
type PropositionSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              PropositionParameters `json:"forProvider"`
}

// PropositionStatus represents the observed state of a Proposition.
type PropositionStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider                 PropositionObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// Proposition is the Schema for the Proposition API.
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
