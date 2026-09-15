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

// DeviceParameters are the configurable fields of a Device.
type DeviceParameters struct {
	// LoginID is the unique login/username for the device. Must be 5-50
	// characters. Immutable after creation.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=5
	// +kubebuilder:validation:MaxLength=50
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="loginId is immutable"
	LoginID string `json:"loginId"`

	// PasswordSecretRef references a Secret containing the device password.
	// Immutable after creation.
	// +kubebuilder:validation:Required
	PasswordSecretRef xpv1.SecretKeySelector `json:"passwordSecretRef"`

	// DeviceExtIDSystem identifies the coding system of the external
	// device identifier.
	// +kubebuilder:validation:Required
	DeviceExtIDSystem string `json:"deviceExtIdSystem"`

	// DeviceExtIDValue is the external device identifier value.
	// +kubebuilder:validation:Required
	DeviceExtIDValue string `json:"deviceExtIdValue"`

	// Type of the device.
	// +kubebuilder:validation:Required
	Type string `json:"type"`

	// GlobalReferenceID is a globally unique identifier for the device.
	// Must be 3-50 characters.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=3
	// +kubebuilder:validation:MaxLength=50
	GlobalReferenceID string `json:"globalReferenceId"`

	// Text is a free-text description of the device.
	// +optional
	Text *string `json:"text,omitempty"`

	// ForTest indicates whether this is a test device.
	// +optional
	ForTest *bool `json:"forTest,omitempty"`

	// OrganizationID is the organization GUID that owns this device.
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

	// ApplicationID is the application GUID this device belongs to.
	// +optional
	// +crossplane:generate:reference:type=Application
	// +crossplane:generate:reference:refFieldName=ApplicationRef
	// +crossplane:generate:reference:selectorFieldName=ApplicationSelector
	ApplicationID *string `json:"applicationId,omitempty"`

	// ApplicationRef references an Application.
	// +optional
	ApplicationRef *xpv1.NamespacedReference `json:"applicationRef,omitempty"`

	// ApplicationSelector selects an Application.
	// +optional
	ApplicationSelector *xpv1.NamespacedSelector `json:"applicationSelector,omitempty"`
}

// DeviceObservation are the observable fields of a Device.
type DeviceObservation struct {
	// ID is the GUID of the device.
	ID *string `json:"id,omitempty"`

	// IsActive as returned by DIP.
	IsActive *bool `json:"isActive,omitempty"`
}

// DeviceSpec defines the desired state of a Device.
type DeviceSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              DeviceParameters `json:"forProvider"`
}

// DeviceStatus represents the observed state of a Device.
type DeviceStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider                 DeviceObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// Device is the Schema for the Device API.
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,hsdp}
type Device struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DeviceSpec   `json:"spec"`
	Status            DeviceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DeviceList contains a list of Device.
type DeviceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Device `json:"items"`
}

// Device type metadata.
var (
	DeviceKind             = reflect.TypeOf(Device{}).Name()
	DeviceGroupKind        = schema.GroupKind{Group: APIGroup, Kind: DeviceKind}.String()
	DeviceKindAPIVersion   = DeviceKind + "." + SchemeGroupVersion.String()
	DeviceGroupVersionKind = SchemeGroupVersion.WithKind(DeviceKind)
)

func init() {
	SchemeBuilder.Register(&Device{}, &DeviceList{})
}
