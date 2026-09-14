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

// AuthenticationMethodParameters are the configurable fields of an MDM AuthenticationMethod.
type AuthenticationMethodParameters struct {
	// Name of the authentication method. Required. Max 20 chars.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=20
	Name string `json:"name"`

	// Description of the authentication method.
	// +optional
	Description *string `json:"description,omitempty"`

	// LoginName for authentication. Required. Max 78 chars.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=78
	LoginName string `json:"loginName"`

	// PasswordSecretRef references a Secret containing the password.
	// +kubebuilder:validation:Required
	PasswordSecretRef xpv1.SecretKeySelector `json:"passwordSecretRef"`

	// ClientID for OAuth2 authentication. Required. Max 78 chars.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=78
	ClientID string `json:"clientId"`

	// ClientSecretRef references a Secret containing the client secret.
	// +kubebuilder:validation:Required
	ClientSecretRef xpv1.SecretKeySelector `json:"clientSecretRef"`

	// AuthURL is the authentication URL (optional).
	// +optional
	AuthURL *string `json:"authUrl,omitempty"`

	// AuthMethod is the authentication method type (optional).
	// +optional
	AuthMethod *string `json:"authMethod,omitempty"`

	// APIVersion for the authentication API (optional).
	// +optional
	APIVersion *string `json:"apiVersion,omitempty"`

	// OrganizationID is the IAM organization GUID (optional).
	// +optional
	OrganizationID *string `json:"organizationId,omitempty"`
}

// AuthenticationMethodObservation are the observable fields of an MDM AuthenticationMethod.
type AuthenticationMethodObservation struct {
	// ID is the MDM ID of the authentication method.
	ID *string `json:"id,omitempty"`

	// Description as returned by DIP.
	Description *string `json:"description,omitempty"`

	// AuthURL as returned by DIP.
	AuthURL *string `json:"authUrl,omitempty"`

	// AuthMethod as returned by DIP.
	AuthMethod *string `json:"authMethod,omitempty"`
}

// AuthenticationMethodSpec defines the desired state of an MDM AuthenticationMethod.
type AuthenticationMethodSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              AuthenticationMethodParameters `json:"forProvider"`
}

// AuthenticationMethodStatus represents the observed state of an MDM AuthenticationMethod.
type AuthenticationMethodStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider                 AuthenticationMethodObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// AuthenticationMethod is the Schema for the MDM AuthenticationMethod API.
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,hsdp}
type AuthenticationMethod struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              AuthenticationMethodSpec   `json:"spec"`
	Status            AuthenticationMethodStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AuthenticationMethodList contains a list of AuthenticationMethod.
type AuthenticationMethodList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AuthenticationMethod `json:"items"`
}

// AuthenticationMethod type metadata.
var (
	AuthenticationMethodKind             = reflect.TypeOf(AuthenticationMethod{}).Name()
	AuthenticationMethodGroupKind        = schema.GroupKind{Group: SchemeGroupVersion.Group, Kind: AuthenticationMethodKind}.String()
	AuthenticationMethodKindAPIVersion   = AuthenticationMethodKind + "." + SchemeGroupVersion.String()
	AuthenticationMethodGroupVersionKind = SchemeGroupVersion.WithKind(AuthenticationMethodKind)
)

func init() {
	SchemeBuilder.Register(&AuthenticationMethod{}, &AuthenticationMethodList{})
}
