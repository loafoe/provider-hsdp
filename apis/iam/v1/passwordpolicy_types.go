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

// ChallengePolicyParameters configures security challenge questions.
type ChallengePolicyParameters struct {
	// DefaultQuestions available for users.
	// +optional
	DefaultQuestions []string `json:"defaultQuestions,omitempty"`

	// MinAnswerLength for challenge answers.
	// +optional
	MinAnswerLength *int `json:"minAnswerLength,omitempty"`

	// MaxIncorrectAttempts before lockout.
	// +optional
	MaxIncorrectAttempts *int `json:"maxIncorrectAttempts,omitempty"`
}

// PasswordPolicyParameters are the configurable fields of a PasswordPolicy.
type PasswordPolicyParameters struct {
	// ManagingOrganizationID is the organization GUID that manages this policy.
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

	// ExpiryPeriodInDays before password must be changed.
	// +optional
	ExpiryPeriodInDays *int `json:"expiryPeriodInDays,omitempty"`

	// HistoryCount of previous passwords that cannot be reused.
	// +optional
	HistoryCount *int `json:"historyCount,omitempty"`

	// MinLength of password.
	// +optional
	MinLength *int `json:"minLength,omitempty"`

	// MaxLength of password.
	// +optional
	MaxLength *int `json:"maxLength,omitempty"`

	// MinLowercase characters required.
	// +optional
	MinLowercase *int `json:"minLowercase,omitempty"`

	// MinUppercase characters required.
	// +optional
	MinUppercase *int `json:"minUppercase,omitempty"`

	// MinNumeric characters required.
	// +optional
	MinNumeric *int `json:"minNumeric,omitempty"`

	// MinSpecialChars required.
	// +optional
	MinSpecialChars *int `json:"minSpecialChars,omitempty"`

	// ChallengesEnabled enables security challenge questions.
	// +optional
	ChallengesEnabled *bool `json:"challengesEnabled,omitempty"`

	// ChallengePolicy configures security challenge questions.
	// +optional
	ChallengePolicy *ChallengePolicyParameters `json:"challengePolicy,omitempty"`
}

// PasswordPolicyObservation are the observable fields of a PasswordPolicy.
type PasswordPolicyObservation struct {
	// ID is the GUID of the password policy.
	ID *string `json:"id,omitempty"`

	// ExpiryPeriodInDays as returned by DIP.
	ExpiryPeriodInDays *int `json:"expiryPeriodInDays,omitempty"`

	// HistoryCount as returned by DIP.
	HistoryCount *int `json:"historyCount,omitempty"`
}

// PasswordPolicySpec defines the desired state of a PasswordPolicy.
type PasswordPolicySpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              PasswordPolicyParameters `json:"forProvider"`
}

// PasswordPolicyStatus represents the observed state of a PasswordPolicy.
type PasswordPolicyStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider                 PasswordPolicyObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// PasswordPolicy is the Schema for the PasswordPolicy API.
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,hsdp}
type PasswordPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              PasswordPolicySpec   `json:"spec"`
	Status            PasswordPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PasswordPolicyList contains a list of PasswordPolicy.
type PasswordPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PasswordPolicy `json:"items"`
}

// PasswordPolicy type metadata.
var (
	PasswordPolicyKind             = reflect.TypeOf(PasswordPolicy{}).Name()
	PasswordPolicyGroupKind        = schema.GroupKind{Group: SchemeGroupVersion.Group, Kind: PasswordPolicyKind}.String()
	PasswordPolicyKindAPIVersion   = PasswordPolicyKind + "." + SchemeGroupVersion.String()
	PasswordPolicyGroupVersionKind = SchemeGroupVersion.WithKind(PasswordPolicyKind)
)

func init() {
	SchemeBuilder.Register(&PasswordPolicy{}, &PasswordPolicyList{})
}
