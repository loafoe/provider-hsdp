package v1

import (
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProviderConfigStatus defines the status of a Provider.
type ProviderConfigStatus struct {
	xpv1.ProviderConfigStatus `json:",inline"`
}

// ProviderCredentials required to authenticate.
type ProviderCredentials struct {
	// Source of the provider credentials.
	// +kubebuilder:validation:Enum=Secret
	// +kubebuilder:default=Secret
	Source xpv1.CredentialsSource `json:"source"`

	// SecretRef references a secret containing service_id and service_private_key.
	// The secret may also contain region and environment to override spec values.
	// +optional
	SecretRef *xpv1.SecretKeySelector `json:"secretRef,omitempty"`
}

// ProviderConfigSpec defines the configuration for connecting to DIP.
type ProviderConfigSpec struct {
	// Region is the DIP IAM region. Can be overridden by secret.
	// +optional
	Region string `json:"region,omitempty"`

	// Environment is the DIP IAM environment. Can be overridden by secret.
	// +optional
	Environment string `json:"environment,omitempty"`

	// Credentials for service identity authentication.
	// +kubebuilder:validation:Required
	Credentials ProviderCredentials `json:"credentials"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="REGION",type="string",JSONPath=".spec.region"
// +kubebuilder:resource:scope=Cluster,categories={crossplane,provider,hsdp}

// ClusterProviderConfig configures the DIP provider cluster-wide. This is the
// default ProviderConfig kind referenced by namespace-scoped managed
// resources, per the Crossplane v2 convention. For a namespace-scoped
// override, see ProviderConfig in the hsdp.m.crossplane.io group.
type ClusterProviderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProviderConfigSpec   `json:"spec"`
	Status ProviderConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterProviderConfigList contains a list of ClusterProviderConfig.
type ClusterProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterProviderConfig `json:"items"`
}
