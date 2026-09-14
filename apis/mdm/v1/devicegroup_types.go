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

// DeviceGroupParameters are the configurable fields of an MDM DeviceGroup.
type DeviceGroupParameters struct {
	// Name of the device group. Required.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Description of the device group.
	// +optional
	Description *string `json:"description,omitempty"`

	// ApplicationID is the MDM application ID this device group belongs to.
	// +optional
	// +crossplane:generate:reference:type=Application
	// +crossplane:generate:reference:refFieldName=ApplicationRef
	// +crossplane:generate:reference:selectorFieldName=ApplicationSelector
	ApplicationID *string `json:"applicationId,omitempty"`

	// ApplicationRef references an MDM Application to populate applicationId.
	// +optional
	ApplicationRef *xpv1.NamespacedReference `json:"applicationRef,omitempty"`

	// ApplicationSelector selects an MDM Application to populate applicationId.
	// +optional
	ApplicationSelector *xpv1.NamespacedSelector `json:"applicationSelector,omitempty"`

	// DefaultGroupGUID is the default IAM group GUID.
	// +optional
	DefaultGroupGUID *string `json:"defaultGroupGuid,omitempty"`
}

// DeviceGroupObservation are the observable fields of an MDM DeviceGroup.
type DeviceGroupObservation struct {
	// ID is the MDM ID of the device group.
	ID *string `json:"id,omitempty"`

	// Description as returned by DIP.
	Description *string `json:"description,omitempty"`
}

// DeviceGroupSpec defines the desired state of an MDM DeviceGroup.
type DeviceGroupSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              DeviceGroupParameters `json:"forProvider"`
}

// DeviceGroupStatus represents the observed state of an MDM DeviceGroup.
type DeviceGroupStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider                 DeviceGroupObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// DeviceGroup is the Schema for the MDM DeviceGroup API.
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,hsdp}
type DeviceGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DeviceGroupSpec   `json:"spec"`
	Status            DeviceGroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DeviceGroupList contains a list of DeviceGroup.
type DeviceGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeviceGroup `json:"items"`
}

// DeviceGroup type metadata.
var (
	DeviceGroupKind             = reflect.TypeOf(DeviceGroup{}).Name()
	DeviceGroupGroupKind        = schema.GroupKind{Group: SchemeGroupVersion.Group, Kind: DeviceGroupKind}.String()
	DeviceGroupKindAPIVersion   = DeviceGroupKind + "." + SchemeGroupVersion.String()
	DeviceGroupGroupVersionKind = SchemeGroupVersion.WithKind(DeviceGroupKind)
)

func init() {
	SchemeBuilder.Register(&DeviceGroup{}, &DeviceGroupList{})
}
