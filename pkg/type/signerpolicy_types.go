package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&SignerPolicy{}, &SignerPolicyList{})
}

// SignerPolicySpec is a spec of SignerPolicy
type SignerPolicySpec struct {
	// Signers are the list of desired signers of images to be allowed in the namespace
	Signers []string `json:"signers"`
}

// +kubebuilder:object:root=true

// SignerPolicy contains the list of valid signer in a namespace
// +kubebuilder:resource:path=signerpolicies,scope=Namespaced,shortName=sp
type SignerPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SignerPolicySpec `json:"spec"`
}

// +kubebuilder:object:root=true

// SignerPolicyList contains the list of SignerPolicy resources
type SignerPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SignerPolicy `json:"items"`
}
