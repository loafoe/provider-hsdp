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

// ApplicationParameters are the configurable fields of an MDM Application.
type ApplicationParameters struct {
	// Name of the application. Required.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Description of the application.
	// +optional
	Description *string `json:"description,omitempty"`

	// PropositionID is the MDM proposition ID this application belongs to.
	// +optional
	// +crossplane:generate:reference:type=Proposition
	// +crossplane:generate:reference:refFieldName=PropositionRef
	// +crossplane:generate:reference:selectorFieldName=PropositionSelector
	PropositionID *string `json:"propositionId,omitempty"`

	// PropositionRef references an MDM Proposition to populate propositionId.
	// +optional
	PropositionRef *xpv1.NamespacedReference `json:"propositionRef,omitempty"`

	// PropositionSelector selects an MDM Proposition to populate propositionId.
	// +optional
	PropositionSelector *xpv1.NamespacedSelector `json:"propositionSelector,omitempty"`

	// GlobalReferenceID is a globally unique reference identifier.
	// +kubebuilder:validation:Required
	GlobalReferenceID string `json:"globalReferenceId"`

	// ApplicationGUID is the IAM application GUID (optional, for linking).
	// +optional
	ApplicationGUID *string `json:"applicationGuid,omitempty"`

	// DefaultGroupGUID is the default IAM group GUID.
	// +optional
	DefaultGroupGUID *string `json:"defaultGroupGuid,omitempty"`
}

// ApplicationObservation are the observable fields of an MDM Application.
type ApplicationObservation struct {
	// ID is the MDM ID of the application.
	ID *string `json:"id,omitempty"`

	// Description as returned by DIP.
	Description *string `json:"description,omitempty"`

	// GlobalReferenceID as returned by DIP.
	GlobalReferenceID *string `json:"globalReferenceId,omitempty"`
}

// ApplicationSpec defines the desired state of an MDM Application.
type ApplicationSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              ApplicationParameters `json:"forProvider"`
}

// ApplicationStatus represents the observed state of an MDM Application.
type ApplicationStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider                 ApplicationObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// Application is the Schema for the MDM Application API.
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,hsdp}
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ApplicationSpec   `json:"spec"`
	Status            ApplicationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ApplicationList contains a list of Application.
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Application `json:"items"`
}

// Application type metadata.
var (
	ApplicationKind             = reflect.TypeOf(Application{}).Name()
	ApplicationGroupKind        = schema.GroupKind{Group: SchemeGroupVersion.Group, Kind: ApplicationKind}.String()
	ApplicationKindAPIVersion   = ApplicationKind + "." + SchemeGroupVersion.String()
	ApplicationGroupVersionKind = SchemeGroupVersion.WithKind(ApplicationKind)
)

func init() {
	SchemeBuilder.Register(&Application{}, &ApplicationList{})
}
