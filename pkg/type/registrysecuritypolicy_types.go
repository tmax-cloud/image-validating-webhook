package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&ClusterRegistrySecurityPolicy{}, &ClusterRegistrySecurityPolicyList{})
	SchemeBuilder.Register(&RegistrySecurityPolicy{}, &RegistrySecurityPolicyList{})
}

// RegistrySpec is a spec of Registries
type RegistrySpec struct {
	// Registry is URL of target registry
	Registry string `json:"registry"`
	// Notary is URL of registry's notary server
	Notary string `json:"notary,omitempty"`
	// SignCheck is a flag to decide to check sign data or not. If it is set false, sign check is skipped
	SignCheck bool `json:"signCheck"`
	// CosignKeyRef is key reference like secret resource or else that saved cosign key
	CosignKeyRef string `json:"cosignKeyRef,omitempty"`
	// Signers are the list of desired signers of images to be allowed
	Signer []string `json:"signer,omitempty"`
}

// ClusterRegistrySecurityPolicySpec is a spec of ClusterRegistrySecurityPolicy
type ClusterRegistrySecurityPolicySpec struct {
	// Registries are the list of registries allowed in the cluster
	Registries []RegistrySpec `json:"registries"`
}

// RegistrySecurityPolicySpec is a spec of RegistrySecurityPolicy
type RegistrySecurityPolicySpec struct {
	// Registries are the list of registries allowed in the namespace
	Registries []RegistrySpec `json:"registries"`
}

// +kubebuilder:object:root=true

// ClusterRegistrySecurityPolicy contains the list of valid registry in a cluster
// +kubebuilder:resource:path=clusterregistrysecuritypolicies,scope=Cluster,shortName=crsp
type ClusterRegistrySecurityPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ClusterRegistrySecurityPolicySpec `json:"spec"`
}

// +kubebuilder:object:root=true

// RegistrySecurityPolicy contains the list of valid registry in a namespace
// +kubebuilder:resource:path=registrysecuritypolicies,scope=Namespaced,shortName=rsp
type RegistrySecurityPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RegistrySecurityPolicySpec `json:"spec"`
}

// +kubebuilder:object:root=true

// ClusterRegistrySecurityPolicyList contains the list of ClusterRegistrySecurityPolicy resources
type ClusterRegistrySecurityPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterRegistrySecurityPolicy `json:"items"`
}

// +kubebuilder:object:root=true

// RegistrySecurityPolicyList contains the list of RegistrySecurityPolicy resources
type RegistrySecurityPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RegistrySecurityPolicy `json:"items"`
}
