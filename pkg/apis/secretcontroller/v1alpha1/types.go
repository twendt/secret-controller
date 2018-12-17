package v1alpha1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KeyvaultSecret is a specification for a KeyvaultSecret resource
type KeyvaultSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec KeyvaultSecretSpec `json:"spec"`
}

// KeyvaultSecretSpec is the spec for a KeyvaultSecret resource
type KeyvaultSecretSpec struct {
	SecretName string                `json:"secretName"`
	Items      []KeyvaultSecretEntry `json:"items"`
}

type KeyvaultSecretEntry struct {
	KeyvaultName    string `json:"keyvaultName"`
	KeyvaultVersion string `json:"keyvaultVersion"`
	KubernetesName  string `json:"kubernetesName"`
	SecretTemplate  string `json:"secretTemplate"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KeyvaultSecretList is a list of KeyvaultSecret resources
type KeyvaultSecretList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []KeyvaultSecret `json:"items"`
}

func (entry KeyvaultSecretEntry) IsTemplateEntry() bool {
	return entry.SecretTemplate != ""
}

func (entry KeyvaultSecretEntry) IsValid() (bool, error) {
	if entry.KubernetesName == "" || (entry.KeyvaultName == "" && entry.SecretTemplate == "") {
		return false, fmt.Errorf("NameKubernetes and one of NameKeyvault and SecretTemplate must be set")
	}
	return true, nil
}
