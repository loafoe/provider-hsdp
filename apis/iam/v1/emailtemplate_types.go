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

// EmailTemplateParameters are the configurable fields of an EmailTemplate.
type EmailTemplateParameters struct {
	// Type of email template (e.g., ACCOUNT_VERIFICATION, PASSWORD_RESET). Immutable.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="type is immutable"
	Type string `json:"type"`

	// ManagingOrganizationID is the organization GUID that manages this template.
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

	// Format of the email (HTML or TEXT).
	// +kubebuilder:validation:Enum=HTML;TEXT
	// +optional
	Format *string `json:"format,omitempty"`

	// Locale for the template (e.g., en-US).
	// +optional
	Locale *string `json:"locale,omitempty"`

	// Subject line of the email.
	// +optional
	Subject *string `json:"subject,omitempty"`

	// From address for the email.
	// +optional
	From *string `json:"from,omitempty"`

	// Message body content.
	// +optional
	Message *string `json:"message,omitempty"`

	// Link to include in the email.
	// +optional
	Link *string `json:"link,omitempty"`
}

// EmailTemplateObservation are the observable fields of an EmailTemplate.
type EmailTemplateObservation struct {
	// ID is the GUID of the email template.
	ID *string `json:"id,omitempty"`

	// Subject line as returned by DIP.
	Subject *string `json:"subject,omitempty"`

	// From address as returned by DIP.
	From *string `json:"from,omitempty"`

	// MessageBase64 is the base64-encoded message body actually sent to DIP.
	// DIP's API requires the message body to be base64-encoded on the wire
	// and doesn't return it on read, so this is only ever populated at
	// creation time, from the payload we sent.
	MessageBase64 *string `json:"messageBase64,omitempty"`
}

// EmailTemplateSpec defines the desired state of an EmailTemplate.
type EmailTemplateSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              EmailTemplateParameters `json:"forProvider"`
}

// EmailTemplateStatus represents the observed state of an EmailTemplate.
type EmailTemplateStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider                 EmailTemplateObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// EmailTemplate is the Schema for the EmailTemplate API.
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,hsdp}
type EmailTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              EmailTemplateSpec   `json:"spec"`
	Status            EmailTemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// EmailTemplateList contains a list of EmailTemplate.
type EmailTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EmailTemplate `json:"items"`
}

// EmailTemplate type metadata.
var (
	EmailTemplateKind             = reflect.TypeOf(EmailTemplate{}).Name()
	EmailTemplateGroupKind        = schema.GroupKind{Group: SchemeGroupVersion.Group, Kind: EmailTemplateKind}.String()
	EmailTemplateKindAPIVersion   = EmailTemplateKind + "." + SchemeGroupVersion.String()
	EmailTemplateGroupVersionKind = SchemeGroupVersion.WithKind(EmailTemplateKind)
)

func init() {
	SchemeBuilder.Register(&EmailTemplate{}, &EmailTemplateList{})
}
