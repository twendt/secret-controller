package main

import (
	"bytes"
	"html/template"

	keyvaultsecretv1alpha1 "github.com/twendt/secret-controller/pkg/apis/secretcontroller/v1alpha1"
	"github.com/twendt/secret-controller/pkg/secretstore"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type SecretConverter struct {
	keyvaultSecret *keyvaultsecretv1alpha1.KeyvaultSecret
	storeClient    secretstore.Client
}

// newSecret creates a new Secret from a KeyvaultSecret resource
func (c *SecretConverter) newSecret() *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.keyvaultSecret.Name,
			Namespace: c.keyvaultSecret.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(c.keyvaultSecret, schema.GroupVersionKind{
					Group:   keyvaultsecretv1alpha1.SchemeGroupVersion.Group,
					Version: keyvaultsecretv1alpha1.SchemeGroupVersion.Version,
					Kind:    "KeyvaultSecret",
				}),
			},
		},
	}
	return secret
}

func (c *SecretConverter) getK8sSecret() (*corev1.Secret, error) {
	secret := c.newSecret()
	secret.Data = make(map[string][]byte)
	for _, item := range c.keyvaultSecret.Spec.Items {
		if ok, err := item.IsValid(); !ok {
			return secret, err
		}
		if item.IsTemplateEntry() {
			parsed, err := c.processTemplate(item)
			if err != nil {
				return secret, err
			}
			secret.Data[item.KubernetesName] = []byte(parsed)
			continue
		}

		secretValue, err := c.storeClient.GetSecretValueForVersion(item.KeyvaultName, item.KeyvaultVersion)
		if err != nil {
			return secret, err
		}
		secret.Data[item.KubernetesName] = []byte(secretValue)
	}
	return secret, nil
}

func (c *SecretConverter) processTemplate(item keyvaultsecretv1alpha1.KeyvaultSecretEntry) (string, error) {
	t, err := c.getTemplate(item)
	if err != nil {
		return "", err
	}
	var tpl bytes.Buffer
	if err := t.Execute(&tpl, nil); err != nil {
		return "", err
	}
	return tpl.String(), nil
}

func (c *SecretConverter) getTemplate(item keyvaultsecretv1alpha1.KeyvaultSecretEntry) (*template.Template, error) {
	return template.New(item.KubernetesName).Delims("[[", "]]").Funcs(template.FuncMap{
		"secretValue":           c.templateFuncSecretValue,
		"secretValueForVersion": c.templateFuncSecretValueForVersion,
	}).Parse(item.SecretTemplate)
}

func (c *SecretConverter) templateFuncSecretValue(name string) string {
	return c.templateFuncSecretValueForVersion(name, "")
}

func (c *SecretConverter) templateFuncSecretValueForVersion(name, version string) string {
	secretValue, err := c.storeClient.GetSecretValue(name)
	if err != nil {
		return err.Error()
	}
	return secretValue
}
