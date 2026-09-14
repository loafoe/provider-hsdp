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

// ServiceParameters are the configurable fields of a Service.
type ServiceParameters struct {
	// Name of the service. Immutable after creation.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name"`

	// Description of the service.
	// +optional
	Description *string `json:"description,omitempty"`

	// ApplicationID is the application GUID this service belongs to.
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

	// PrivateKeySecretRef optionally references a secret containing a private key.
	// If not provided, a key pair will be generated.
	// +optional
	PrivateKeySecretRef *xpv1.SecretKeySelector `json:"privateKeySecretRef,omitempty"`

	// Scopes available to this service.
	// +optional
	Scopes []string `json:"scopes,omitempty"`

	// DefaultScopes applied when none requested.
	// +optional
	DefaultScopes []string `json:"defaultScopes,omitempty"`

	// AccessTokenLifetime in seconds.
	// +optional
	AccessTokenLifetime *int64 `json:"accessTokenLifetime,omitempty"`

	// RefreshTokenLifetime in seconds.
	// +optional
	RefreshTokenLifetime *int64 `json:"refreshTokenLifetime,omitempty"`

	// TokenEndpointAuthMethod (e.g., private_key_jwt).
	// +optional
	TokenEndpointAuthMethod *string `json:"tokenEndpointAuthMethod,omitempty"`
}

// ServiceObservation are the observable fields of a Service.
type ServiceObservation struct {
	// ID is the GUID of the service.
	ID *string `json:"id,omitempty"`

	// ServiceID is the service identifier used for authentication.
	ServiceID *string `json:"serviceId,omitempty"`

	// OrganizationID is the organization this service belongs to.
	OrganizationID *string `json:"organizationId,omitempty"`

	// ExpiresOn is when the service credentials expire.
	ExpiresOn *string `json:"expiresOn,omitempty"`

	// Description as returned by DIP.
	Description *string `json:"description,omitempty"`
}

// ServiceSpec defines the desired state of a Service.
type ServiceSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              ServiceParameters `json:"forProvider"`
}

// ServiceStatus represents the observed state of a Service.
type ServiceStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider                 ServiceObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// Service is the Schema for the Service API.
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,hsdp}
type Service struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ServiceSpec   `json:"spec"`
	Status            ServiceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ServiceList contains a list of Service.
type ServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Service `json:"items"`
}

// Service type metadata.
var (
	ServiceKind             = reflect.TypeOf(Service{}).Name()
	ServiceGroupKind        = schema.GroupKind{Group: SchemeGroupVersion.Group, Kind: ServiceKind}.String()
	ServiceKindAPIVersion   = ServiceKind + "." + SchemeGroupVersion.String()
	ServiceGroupVersionKind = SchemeGroupVersion.WithKind(ServiceKind)
)

func init() {
	SchemeBuilder.Register(&Service{}, &ServiceList{})
}
