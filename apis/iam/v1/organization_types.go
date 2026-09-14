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

// OrganizationParameters are the configurable fields of an Organization.
type OrganizationParameters struct {
	// Name of the organization. Immutable after creation.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name"`

	// DisplayName for the organization.
	// +optional
	DisplayName *string `json:"displayName,omitempty"`

	// Description of the organization.
	// +optional
	Description *string `json:"description,omitempty"`

	// ParentOrgID is the parent organization GUID.
	// +optional
	// +crossplane:generate:reference:type=Organization
	// +crossplane:generate:reference:refFieldName=ParentOrgRef
	// +crossplane:generate:reference:selectorFieldName=ParentOrgSelector
	ParentOrgID *string `json:"parentOrgId,omitempty"`

	// ParentOrgRef references an Organization to populate parentOrgId.
	// +optional
	ParentOrgRef *xpv1.NamespacedReference `json:"parentOrgRef,omitempty"`

	// ParentOrgSelector selects an Organization to populate parentOrgId.
	// +optional
	ParentOrgSelector *xpv1.NamespacedSelector `json:"parentOrgSelector,omitempty"`

	// Type of the organization (e.g., Hospital).
	// +optional
	Type *string `json:"type,omitempty"`

	// ExternalID is a client-defined identifier.
	// +optional
	ExternalID *string `json:"externalId,omitempty"`
}

// OrganizationObservation are the observable fields of an Organization.
type OrganizationObservation struct {
	// ID is the GUID of the organization.
	ID *string `json:"id,omitempty"`

	// Active indicates if the organization is active.
	Active *bool `json:"active,omitempty"`

	// Name of the organization as returned by DIP.
	Name *string `json:"name,omitempty"`

	// DisplayName of the organization as returned by DIP.
	DisplayName *string `json:"displayName,omitempty"`

	// Description of the organization as returned by DIP.
	Description *string `json:"description,omitempty"`

	// Type of the organization (e.g., Hospital) as returned by DIP.
	Type *string `json:"type,omitempty"`

	// ExternalID as returned by DIP.
	ExternalID *string `json:"externalId,omitempty"`

	// ParentOrgID is the parent organization GUID as returned by DIP.
	ParentOrgID *string `json:"parentOrgId,omitempty"`
}

// OrganizationSpec defines the desired state of an Organization.
type OrganizationSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              OrganizationParameters `json:"forProvider"`
}

// OrganizationStatus represents the observed state of an Organization.
type OrganizationStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider                 OrganizationObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// Organization is the Schema for the Organization API.
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,hsdp}
type Organization struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              OrganizationSpec   `json:"spec"`
	Status            OrganizationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OrganizationList contains a list of Organization.
type OrganizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Organization `json:"items"`
}

// Organization type metadata.
var (
	OrganizationKind             = reflect.TypeOf(Organization{}).Name()
	OrganizationGroupKind        = schema.GroupKind{Group: SchemeGroupVersion.Group, Kind: OrganizationKind}.String()
	OrganizationKindAPIVersion   = OrganizationKind + "." + SchemeGroupVersion.String()
	OrganizationGroupVersionKind = SchemeGroupVersion.WithKind(OrganizationKind)
)

func init() {
	SchemeBuilder.Register(&Organization{}, &OrganizationList{})
}
