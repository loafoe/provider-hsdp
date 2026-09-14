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

// DeviceTypeParameters are the configurable fields of an MDM DeviceType.
type DeviceTypeParameters struct {
	// Name of the device type. Required.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Description of the device type.
	// +optional
	Description *string `json:"description,omitempty"`

	// CTN is the Commercial Type Number. Required.
	// +kubebuilder:validation:Required
	CTN string `json:"ctn"`

	// DeviceGroupID is the MDM device group ID this device type belongs to.
	// +optional
	// +crossplane:generate:reference:type=DeviceGroup
	// +crossplane:generate:reference:refFieldName=DeviceGroupRef
	// +crossplane:generate:reference:selectorFieldName=DeviceGroupSelector
	DeviceGroupID *string `json:"deviceGroupId,omitempty"`

	// DeviceGroupRef references an MDM DeviceGroup to populate deviceGroupId.
	// +optional
	DeviceGroupRef *xpv1.NamespacedReference `json:"deviceGroupRef,omitempty"`

	// DeviceGroupSelector selects an MDM DeviceGroup to populate deviceGroupId.
	// +optional
	DeviceGroupSelector *xpv1.NamespacedSelector `json:"deviceGroupSelector,omitempty"`

	// DefaultGroupGUID is the default IAM group GUID.
	// +optional
	DefaultGroupGUID *string `json:"defaultGroupGuid,omitempty"`

	// CustomTypeAttributes are custom attributes for the device type (JSON).
	// +optional
	CustomTypeAttributes *string `json:"customTypeAttributes,omitempty"`
}

// DeviceTypeObservation are the observable fields of an MDM DeviceType.
type DeviceTypeObservation struct {
	// ID is the MDM ID of the device type.
	ID *string `json:"id,omitempty"`

	// Description as returned by DIP.
	Description *string `json:"description,omitempty"`

	// CTN (Commercial Type Number) as returned by DIP.
	CTN *string `json:"ctn,omitempty"`
}

// DeviceTypeSpec defines the desired state of an MDM DeviceType.
type DeviceTypeSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              DeviceTypeParameters `json:"forProvider"`
}

// DeviceTypeStatus represents the observed state of an MDM DeviceType.
type DeviceTypeStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider                 DeviceTypeObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// DeviceType is the Schema for the MDM DeviceType API.
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,hsdp}
type DeviceType struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DeviceTypeSpec   `json:"spec"`
	Status            DeviceTypeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DeviceTypeList contains a list of DeviceType.
type DeviceTypeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeviceType `json:"items"`
}

// DeviceType type metadata.
var (
	DeviceTypeKind             = reflect.TypeOf(DeviceType{}).Name()
	DeviceTypeGroupKind        = schema.GroupKind{Group: SchemeGroupVersion.Group, Kind: DeviceTypeKind}.String()
	DeviceTypeKindAPIVersion   = DeviceTypeKind + "." + SchemeGroupVersion.String()
	DeviceTypeGroupVersionKind = SchemeGroupVersion.WithKind(DeviceTypeKind)
)

func init() {
	SchemeBuilder.Register(&DeviceType{}, &DeviceTypeList{})
}
