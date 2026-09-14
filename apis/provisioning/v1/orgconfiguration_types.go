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

// BootstrapSignatureConfig contains signature configuration options.
type BootstrapSignatureConfig struct {
	// Type is the signature type (RSA, ECC, DSA).
	// +kubebuilder:validation:Enum=RSA;ECC;DSA
	// +optional
	Type *string `json:"type,omitempty"`

	// Padding is the padding type for RSA signatures.
	// +kubebuilder:validation:Enum=RSA_PKCS1_PSS_PADDING
	// +optional
	Padding *string `json:"padding,omitempty"`

	// SaltLength is the salt length for PSS padding.
	// +kubebuilder:validation:Enum=RSA_PSS_SALTLEN_DIGEST;RSA_PSS_SALTLEN_MAX_SIGN;RSA_PSS_SALTLEN_AUTO
	// +optional
	SaltLength *string `json:"saltLength,omitempty"`
}

// BootstrapSignature contains the signature configuration for device bootstrapping.
type BootstrapSignature struct {
	// PublicKeySecretRef references a Secret containing the public key.
	// +kubebuilder:validation:Required
	PublicKeySecretRef xpv1.SecretKeySelector `json:"publicKeySecretRef"`

	// Algorithm is the signature algorithm.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=SHA256;SHA512;RSA-MD5;RSA-RIPEMD160;RSA-SHA1;RSA-SHA1-2;RSA-SHA224;RSA-SHA256;RSA-SHA3-224;RSA-SHA3-256;RSA-SHA3-384;RSA-SHA3-512;RSA-SHA384;RSA-SHA512
	Algorithm string `json:"algorithm"`

	// Config contains additional signature configuration.
	// +optional
	Config *BootstrapSignatureConfig `json:"config,omitempty"`
}

// ServiceAccount contains the service account credentials for provisioning.
type ServiceAccount struct {
	// ServiceAccountID is the service account identifier.
	// +kubebuilder:validation:Required
	ServiceAccountID string `json:"serviceAccountId"`

	// ServiceAccountKeySecretRef references a Secret containing the service account key.
	// +kubebuilder:validation:Required
	ServiceAccountKeySecretRef xpv1.SecretKeySelector `json:"serviceAccountKeySecretRef"`
}

// OrgConfigurationParameters are the configurable fields of an OrgConfiguration.
type OrgConfigurationParameters struct {
	// OrganizationID is the IAM organization GUID.
	// +kubebuilder:validation:Required
	OrganizationID string `json:"organizationId"`

	// ServiceAccount contains the service account credentials.
	// +kubebuilder:validation:Required
	ServiceAccount ServiceAccount `json:"serviceAccount"`

	// BootstrapSignature contains the bootstrap signature configuration.
	// +kubebuilder:validation:Required
	BootstrapSignature BootstrapSignature `json:"bootstrapSignature"`
}

// OrgConfigurationObservation are the observable fields of an OrgConfiguration.
type OrgConfigurationObservation struct {
	// ID is the provisioning service ID of the organization configuration.
	ID *string `json:"id,omitempty"`

	// VersionID is the version of the configuration.
	VersionID *int `json:"versionId,omitempty"`
}

// OrgConfigurationSpec defines the desired state of an OrgConfiguration.
type OrgConfigurationSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              OrgConfigurationParameters `json:"forProvider"`
}

// OrgConfigurationStatus represents the observed state of an OrgConfiguration.
type OrgConfigurationStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider                 OrgConfigurationObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// OrgConfiguration is the Schema for the Provisioning OrgConfiguration API.
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,hsdp}
type OrgConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              OrgConfigurationSpec   `json:"spec"`
	Status            OrgConfigurationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OrgConfigurationList contains a list of OrgConfiguration.
type OrgConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OrgConfiguration `json:"items"`
}

// OrgConfiguration type metadata.
var (
	OrgConfigurationKind             = reflect.TypeOf(OrgConfiguration{}).Name()
	OrgConfigurationGroupKind        = schema.GroupKind{Group: SchemeGroupVersion.Group, Kind: OrgConfigurationKind}.String()
	OrgConfigurationKindAPIVersion   = OrgConfigurationKind + "." + SchemeGroupVersion.String()
	OrgConfigurationGroupVersionKind = SchemeGroupVersion.WithKind(OrgConfigurationKind)
)

func init() {
	SchemeBuilder.Register(&OrgConfiguration{}, &OrgConfigurationList{})
}
