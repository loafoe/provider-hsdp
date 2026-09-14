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

// GroupParameters are the configurable fields of a Group.
type GroupParameters struct {
	// Name of the group. Immutable after creation.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name"`

	// Description of the group.
	// +optional
	Description *string `json:"description,omitempty"`

	// ManagingOrganizationID is the organization GUID that manages this group.
	// +optional
	// +crossplane:generate:reference:type=Organization
	// +crossplane:generate:reference:refFieldName=ManagingOrganizationRef
	// +crossplane:generate:reference:selectorFieldName=ManagingOrganizationSelector
	ManagingOrganizationID *string `json:"managingOrganizationId,omitempty"`

	// ManagingOrganizationRef references an Organization.
	// +optional
	ManagingOrganizationRef *xpv1.NamespacedReference `json:"managingOrganizationRef,omitempty"`

	// ManagingOrganizationSelector selects an Organization.
	// +optional
	ManagingOrganizationSelector *xpv1.NamespacedSelector `json:"managingOrganizationSelector,omitempty"`
}

// GroupObservation are the observable fields of a Group.
type GroupObservation struct {
	// ID is the GUID of the group.
	ID *string `json:"id,omitempty"`

	// Description as returned by DIP.
	Description *string `json:"description,omitempty"`
}

// GroupSpec defines the desired state of a Group.
type GroupSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              GroupParameters `json:"forProvider"`
}

// GroupStatus represents the observed state of a Group.
type GroupStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider                 GroupObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// Group is the Schema for the Group API.
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,hsdp}
type Group struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              GroupSpec   `json:"spec"`
	Status            GroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GroupList contains a list of Group.
type GroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Group `json:"items"`
}

// Group type metadata.
var (
	GroupKind             = reflect.TypeOf(Group{}).Name()
	GroupGroupKind        = schema.GroupKind{Group: APIGroup, Kind: GroupKind}.String()
	GroupKindAPIVersion   = GroupKind + "." + SchemeGroupVersion.String()
	GroupGroupVersionKind = SchemeGroupVersion.WithKind(GroupKind)
)

func init() {
	SchemeBuilder.Register(&Group{}, &GroupList{})
}
