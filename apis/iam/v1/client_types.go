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

// ClientParameters are the configurable fields of a Client.
type ClientParameters struct {
	// Name of the client. Immutable after creation.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name"`

	// Type of the client. Either "Public" or "Confidential". Immutable after creation.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Public;Confidential
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="type is immutable"
	Type string `json:"type"`

	// ClientID is the OAuth2 client_id. Must be 5-20 characters. Immutable after creation.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=5
	// +kubebuilder:validation:MaxLength=20
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="clientId is immutable"
	ClientID string `json:"clientId"`

	// PasswordSecretRef references a Secret containing the client password.
	// Password must be 8-16 chars with at least one capital, number, special char.
	// Immutable after creation.
	// +kubebuilder:validation:Required
	PasswordSecretRef xpv1.SecretKeySelector `json:"passwordSecretRef"`

	// Description of the client.
	// +kubebuilder:validation:Required
	Description string `json:"description"`

	// ApplicationID is the application GUID this client belongs to.
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

	// GlobalReferenceID is a global unique identifier. Immutable after creation.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="globalReferenceId is immutable"
	GlobalReferenceID string `json:"globalReferenceId"`

	// RedirectionURIs for OAuth2 redirects.
	// +optional
	RedirectionURIs []string `json:"redirectionURIs,omitempty"`

	// ResponseTypes (e.g., code, token).
	// +optional
	ResponseTypes []string `json:"responseTypes,omitempty"`

	// Scopes available to this client.
	// +optional
	Scopes []string `json:"scopes,omitempty"`

	// DefaultScopes applied when none requested.
	// +optional
	DefaultScopes []string `json:"defaultScopes,omitempty"`

	// ConsentImplied skips user consent screen.
	// +optional
	ConsentImplied *bool `json:"consentImplied,omitempty"`

	// AccessTokenLifetime in seconds.
	// +optional
	AccessTokenLifetime *int64 `json:"accessTokenLifetime,omitempty"`

	// RefreshTokenLifetime in seconds.
	// +optional
	RefreshTokenLifetime *int64 `json:"refreshTokenLifetime,omitempty"`

	// IDTokenLifetime in seconds.
	// +optional
	IDTokenLifetime *int64 `json:"idTokenLifetime,omitempty"`
}

// ClientObservation are the observable fields of a Client.
type ClientObservation struct {
	// ID is the GUID of the client.
	ID *string `json:"id,omitempty"`

	// ClientID is the OAuth2 client_id.
	ClientID *string `json:"clientId,omitempty"`

	// Disabled indicates if the client is disabled.
	Disabled *bool `json:"disabled,omitempty"`

	// Type of the client (Public or Confidential) as returned by DIP.
	Type *string `json:"type,omitempty"`

	// Description as returned by DIP.
	Description *string `json:"description,omitempty"`
}

// ClientSpec defines the desired state of a Client.
type ClientSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              ClientParameters `json:"forProvider"`
}

// ClientStatus represents the observed state of a Client.
type ClientStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider                 ClientObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// Client is the Schema for the Client API.
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,hsdp}
type Client struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ClientSpec   `json:"spec"`
	Status            ClientStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClientList contains a list of Client.
type ClientList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Client `json:"items"`
}

// Client type metadata.
var (
	ClientKind             = reflect.TypeOf(Client{}).Name()
	ClientGroupKind        = schema.GroupKind{Group: SchemeGroupVersion.Group, Kind: ClientKind}.String()
	ClientKindAPIVersion   = ClientKind + "." + SchemeGroupVersion.String()
	ClientGroupVersionKind = SchemeGroupVersion.WithKind(ClientKind)
)

func init() {
	SchemeBuilder.Register(&Client{}, &ClientList{})
}
