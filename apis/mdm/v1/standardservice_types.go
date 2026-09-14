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

// ServiceURL represents a service URL configuration.
type ServiceURL struct {
	// URL is the service endpoint URL.
	// +kubebuilder:validation:Required
	URL string `json:"url"`

	// SortOrder determines the priority of this URL.
	// +optional
	SortOrder *int `json:"sortOrder,omitempty"`

	// AuthenticationMethodID references an authentication method for this URL.
	// +optional
	// +crossplane:generate:reference:type=AuthenticationMethod
	// +crossplane:generate:reference:refFieldName=AuthenticationMethodRef
	// +crossplane:generate:reference:selectorFieldName=AuthenticationMethodSelector
	AuthenticationMethodID *string `json:"authenticationMethodId,omitempty"`

	// AuthenticationMethodRef references an MDM AuthenticationMethod to populate authenticationMethodId.
	// +optional
	AuthenticationMethodRef *xpv1.NamespacedReference `json:"authenticationMethodRef,omitempty"`

	// AuthenticationMethodSelector selects an MDM AuthenticationMethod to populate authenticationMethodId.
	// +optional
	AuthenticationMethodSelector *xpv1.NamespacedSelector `json:"authenticationMethodSelector,omitempty"`
}

// StandardServiceParameters are the configurable fields of an MDM StandardService.
type StandardServiceParameters struct {
	// Name of the standard service. Required.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Description of the standard service.
	// +optional
	Description *string `json:"description,omitempty"`

	// Trusted indicates if this is a trusted service.
	// +optional
	Trusted *bool `json:"trusted,omitempty"`

	// Tags for the service. At least one tag is required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=1
	Tags []string `json:"tags"`

	// ServiceURLs are the endpoints for this service.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=5
	ServiceURLs []ServiceURL `json:"serviceUrls"`

	// OrganizationID is the IAM organization GUID (optional).
	// +optional
	OrganizationID *string `json:"organizationId,omitempty"`
}

// StandardServiceObservation are the observable fields of an MDM StandardService.
type StandardServiceObservation struct {
	// ID is the MDM ID of the standard service.
	ID *string `json:"id,omitempty"`

	// Description as returned by DIP.
	Description *string `json:"description,omitempty"`

	// Trusted indicates if this is a trusted service, as returned by DIP.
	Trusted *bool `json:"trusted,omitempty"`
}

// StandardServiceSpec defines the desired state of an MDM StandardService.
type StandardServiceSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              StandardServiceParameters `json:"forProvider"`
}

// StandardServiceStatus represents the observed state of an MDM StandardService.
type StandardServiceStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider                 StandardServiceObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// StandardService is the Schema for the MDM StandardService API.
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,hsdp}
type StandardService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              StandardServiceSpec   `json:"spec"`
	Status            StandardServiceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StandardServiceList contains a list of StandardService.
type StandardServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StandardService `json:"items"`
}

// StandardService type metadata.
var (
	StandardServiceKind             = reflect.TypeOf(StandardService{}).Name()
	StandardServiceGroupKind        = schema.GroupKind{Group: SchemeGroupVersion.Group, Kind: StandardServiceKind}.String()
	StandardServiceKindAPIVersion   = StandardServiceKind + "." + SchemeGroupVersion.String()
	StandardServiceGroupVersionKind = SchemeGroupVersion.WithKind(StandardServiceKind)
)

func init() {
	SchemeBuilder.Register(&StandardService{}, &StandardServiceList{})
}
