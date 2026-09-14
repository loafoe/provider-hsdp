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

// UserParameters are the configurable fields of a User.
type UserParameters struct {
	// LoginID is the user's login identifier. Immutable after creation.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="loginId is immutable"
	LoginID string `json:"loginId"`

	// Email address of the user.
	// +kubebuilder:validation:Required
	Email string `json:"email"`

	// FirstName of the user.
	// +optional
	FirstName *string `json:"firstName,omitempty"`

	// LastName of the user.
	// +optional
	LastName *string `json:"lastName,omitempty"`

	// OrganizationID is the organization GUID this user belongs to.
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

	// PreferredLanguage (e.g., en-US).
	// +optional
	PreferredLanguage *string `json:"preferredLanguage,omitempty"`

	// PreferredCommunicationChannel (e.g., email, sms).
	// +optional
	PreferredCommunicationChannel *string `json:"preferredCommunicationChannel,omitempty"`

	// IsAgeValidated indicates if user's age has been validated.
	// +optional
	IsAgeValidated *bool `json:"isAgeValidated,omitempty"`

	// PasswordSecretRef references a secret containing the initial password.
	// +optional
	PasswordSecretRef *xpv1.SecretKeySelector `json:"passwordSecretRef,omitempty"`
}

// UserObservation are the observable fields of a User.
type UserObservation struct {
	// ID is the GUID of the user.
	ID *string `json:"id,omitempty"`

	// AccountStatus (e.g., ACTIVE, INACTIVE).
	AccountStatus *string `json:"accountStatus,omitempty"`

	// EmailVerified indicates if email has been verified.
	EmailVerified *bool `json:"emailVerified,omitempty"`

	// LastLoginTime as returned by DIP.
	LastLoginTime *string `json:"lastLoginTime,omitempty"`

	// MFAStatus as returned by DIP.
	MFAStatus *string `json:"mfaStatus,omitempty"`

	// PhoneVerified indicates if the phone number has been verified.
	PhoneVerified *bool `json:"phoneVerified,omitempty"`

	// MustChangePassword indicates if the user must change their password.
	MustChangePassword *bool `json:"mustChangePassword,omitempty"`
}

// UserSpec defines the desired state of a User.
type UserSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              UserParameters `json:"forProvider"`
}

// UserStatus represents the observed state of a User.
type UserStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider                 UserObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// User is the Schema for the User API.
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,hsdp}
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              UserSpec   `json:"spec"`
	Status            UserStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UserList contains a list of User.
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

// User type metadata.
var (
	UserKind             = reflect.TypeOf(User{}).Name()
	UserGroupKind        = schema.GroupKind{Group: SchemeGroupVersion.Group, Kind: UserKind}.String()
	UserKindAPIVersion   = UserKind + "." + SchemeGroupVersion.String()
	UserGroupVersionKind = SchemeGroupVersion.WithKind(UserKind)
)

func init() {
	SchemeBuilder.Register(&User{}, &UserList{})
}
