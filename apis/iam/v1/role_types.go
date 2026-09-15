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

// RoleParameters are the configurable fields of a Role.
type RoleParameters struct {
	// Name of the role. Immutable after creation.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name"`

	// Description of the role.
	// +optional
	Description *string `json:"description,omitempty"`

	// ManagingOrganizationID is the organization GUID that manages this role.
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

	// Permissions is the list of permission names assigned to this role.
	// +optional
	Permissions []string `json:"permissions,omitempty"`

	// SharingPolicies share this role with other Organizations.
	// +optional
	SharingPolicies []RoleSharingPolicyParameters `json:"sharingPolicies,omitempty"`
}

// RoleSharingPolicyParameters describe a sharing policy that grants this
// role to another Organization.
type RoleSharingPolicyParameters struct {
	// TargetOrganizationID is the GUID of the Organization to share this
	// role with.
	// +optional
	// +crossplane:generate:reference:type=Organization
	// +crossplane:generate:reference:refFieldName=TargetOrganizationRef
	// +crossplane:generate:reference:selectorFieldName=TargetOrganizationSelector
	TargetOrganizationID *string `json:"targetOrganizationId,omitempty"`

	// TargetOrganizationRef references the target Organization.
	// +optional
	TargetOrganizationRef *xpv1.NamespacedReference `json:"targetOrganizationRef,omitempty"`

	// TargetOrganizationSelector selects the target Organization.
	// +optional
	TargetOrganizationSelector *xpv1.NamespacedSelector `json:"targetOrganizationSelector,omitempty"`

	// SharingPolicy is the sharing mode, e.g. "ALL_USERS".
	// +kubebuilder:validation:Required
	SharingPolicy string `json:"sharingPolicy"`

	// Purpose describes why this role is being shared.
	// +optional
	Purpose *string `json:"purpose,omitempty"`
}

// RoleSharingPolicyStatus is the observed state of a RoleSharingPolicy.
type RoleSharingPolicyStatus struct {
	// TargetOrganizationID is the GUID of the Organization this role is
	// shared with.
	TargetOrganizationID string `json:"targetOrganizationId,omitempty"`

	// SharingPolicy is the sharing mode currently in effect.
	SharingPolicy string `json:"sharingPolicy,omitempty"`

	// Purpose as returned by DIP.
	Purpose string `json:"purpose,omitempty"`
}

// RoleObservation are the observable fields of a Role.
type RoleObservation struct {
	// ID is the GUID of the role.
	ID *string `json:"id,omitempty"`

	// Description as returned by DIP.
	Description *string `json:"description,omitempty"`

	// SharingPolicies are the sharing policies currently in effect for this
	// role, as observed from DIP.
	SharingPolicies []RoleSharingPolicyStatus `json:"sharingPolicies,omitempty"`
}

// RoleSpec defines the desired state of a Role.
type RoleSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              RoleParameters `json:"forProvider"`
}

// RoleStatus represents the observed state of a Role.
type RoleStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider                 RoleObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// Role is the Schema for the Role API.
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,hsdp}
type Role struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RoleSpec   `json:"spec"`
	Status            RoleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RoleList contains a list of Role.
type RoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Role `json:"items"`
}

// Role type metadata.
var (
	RoleKind             = reflect.TypeOf(Role{}).Name()
	RoleGroupKind        = schema.GroupKind{Group: SchemeGroupVersion.Group, Kind: RoleKind}.String()
	RoleKindAPIVersion   = RoleKind + "." + SchemeGroupVersion.String()
	RoleGroupVersionKind = SchemeGroupVersion.WithKind(RoleKind)
)

func init() {
	SchemeBuilder.Register(&Role{}, &RoleList{})
}
