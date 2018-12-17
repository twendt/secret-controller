package main

import (
	"fmt"
	"reflect"
	"testing"

	keyvaultsecretv1alpha1 "github.com/twendt/secret-controller/pkg/apis/secretcontroller/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_newSecret(t *testing.T) {
	type args struct {
		keyvaultSecret *keyvaultsecretv1alpha1.KeyvaultSecret
	}
	type want struct {
		name      string
		namespace string
	}
	tests := []struct {
		name    string
		args    args
		want    want
		wantErr bool
	}{
		{
			"secretOK",
			args{
				keyvaultSecret: &keyvaultsecretv1alpha1.KeyvaultSecret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "test-namespace",
					},
				},
			},
			want{
				name:      "test-secret",
				namespace: "test-namespace",
			},
			false,
		},
		{
			"secretFail",
			args{
				keyvaultSecret: &keyvaultsecretv1alpha1.KeyvaultSecret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-secret",
					},
				},
			},
			want{
				name: "test-secret",
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := SecretConverter{
				keyvaultSecret: tt.args.keyvaultSecret,
			}
			got := converter.newSecret()
			if got.Name != tt.want.name {
				t.Errorf("newSecret() Name = %v, want %v", got.Name, tt.want.name)
			}
			if got.Namespace != tt.want.namespace {
				t.Errorf("newSecret() Namespace = %v, want %v", got.Namespace, tt.want.namespace)
			}
		})
	}
}

type testSecretStoreClient struct {
	GetSecretValueFunc func() (string, error)
}

func (s testSecretStoreClient) GetSecretValue(name string) (string, error) {
	return s.GetSecretValueForVersion(name, "")
}

func (s testSecretStoreClient) GetSecretValueForVersion(name, version string) (string, error) {
	return s.GetSecretValueFunc()
}

func Test_setSecretItems(t *testing.T) {
	type args struct {
		items  []keyvaultsecretv1alpha1.KeyvaultSecretEntry
		secret *corev1.Secret
	}
	type storeClientResult struct {
		value string
		err   error
	}
	tests := []struct {
		name              string
		args              args
		storeClientResult storeClientResult
		want              map[string][]byte
		wantErr           bool
	}{
		{
			name: "1 item with KeyvaultName",
			args: args{
				items: []keyvaultsecretv1alpha1.KeyvaultSecretEntry{
					{
						KubernetesName: "KubernetesName",
						KeyvaultName:   "KeyvaultName",
					},
				},
				secret: &corev1.Secret{},
			},
			storeClientResult: storeClientResult{"value", nil},
			want:              map[string][]byte{"KubernetesName": []byte("value")},
			wantErr:           false,
		},
		{
			name: "1 item with KeyvaultName with version",
			args: args{
				items: []keyvaultsecretv1alpha1.KeyvaultSecretEntry{
					{
						KubernetesName:  "KubernetesName",
						KeyvaultName:    "KeyvaultName",
						KeyvaultVersion: "versionx",
					},
				},
				secret: &corev1.Secret{},
			},
			storeClientResult: storeClientResult{"value", nil},
			want:              map[string][]byte{"KubernetesName": []byte("value")},
			wantErr:           false,
		},
		{
			name: "2 items with KeyvaultName",
			args: args{
				items: []keyvaultsecretv1alpha1.KeyvaultSecretEntry{
					{
						KubernetesName: "KubernetesName",
						KeyvaultName:   "KeyvaultName",
					},
					{
						KubernetesName: "KubernetesName1",
						KeyvaultName:   "KeyvaultName1",
					},
				},

				secret: &corev1.Secret{},
			},
			storeClientResult: storeClientResult{"value", nil},
			want: map[string][]byte{
				"KubernetesName":  []byte("value"),
				"KubernetesName1": []byte("value"),
			},
			wantErr: false,
		},
		{
			name: "1 item SecretTemplate",
			args: args{
				items: []keyvaultsecretv1alpha1.KeyvaultSecretEntry{
					{
						SecretTemplate: "[[ secretValue \"KubernetesName\" ]]",
						KubernetesName: "KubernetesName",
					},
				},

				secret: &corev1.Secret{},
			},
			storeClientResult: storeClientResult{"value", nil},
			want: map[string][]byte{
				"KubernetesName": []byte("value"),
			},
			wantErr: false,
		},
		{
			name: "1 item SecretTemplate with version",
			args: args{
				items: []keyvaultsecretv1alpha1.KeyvaultSecretEntry{
					{
						SecretTemplate: "[[ secretValueForVersion \"KubernetesName\" \"versionx\" ]]",
						KubernetesName: "KubernetesName",
					},
				},

				secret: &corev1.Secret{},
			},
			storeClientResult: storeClientResult{"value", nil},
			want: map[string][]byte{
				"KubernetesName": []byte("value"),
			},
			wantErr: false,
		},
		{
			name: "wrong template function",
			args: args{
				items: []keyvaultsecretv1alpha1.KeyvaultSecretEntry{
					{
						SecretTemplate: "[[ secretValue1 \"KubernetesName\" ]]",
						KubernetesName: "KubernetesName",
					},
				},

				secret: &corev1.Secret{},
			},
			storeClientResult: storeClientResult{"value", nil},
			want: map[string][]byte{
				"KubernetesName": []byte("value"),
			},
			wantErr: true,
		},
		{
			name: "error in GetSecretValue function in SecretTemplate",
			args: args{
				items: []keyvaultsecretv1alpha1.KeyvaultSecretEntry{
					{
						SecretTemplate: "[[ secretValue \"KubernetesName\" ]]",
						KubernetesName: "KubernetesName",
					},
				},

				secret: &corev1.Secret{},
			},
			storeClientResult: storeClientResult{"", fmt.Errorf("Secret not found")},
			want: map[string][]byte{
				"KubernetesName": []byte("Secret not found"),
			},
			wantErr: false,
		},
		{
			name: "error in GetSecretValue function in KeyvaultName",
			args: args{
				items: []keyvaultsecretv1alpha1.KeyvaultSecretEntry{
					{
						KeyvaultName:   "KeyvaultName",
						KubernetesName: "KubernetesName",
					},
				},

				secret: &corev1.Secret{},
			},
			storeClientResult: storeClientResult{"", fmt.Errorf("Secret not found")},
			want: map[string][]byte{
				"KubernetesName": []byte("Secret not found"),
			},
			wantErr: true,
		},
		{
			name: "missing KubernetesName",
			args: args{
				items: []keyvaultsecretv1alpha1.KeyvaultSecretEntry{
					{
						KeyvaultName: "KeyvaultName",
					},
				},

				secret: &corev1.Secret{},
			},
			storeClientResult: storeClientResult{"", nil},
			want:              map[string][]byte{},
			wantErr:           true,
		},
		{
			name: "missing KeyvaultName and SecretTmplate",
			args: args{
				items: []keyvaultsecretv1alpha1.KeyvaultSecretEntry{
					{
						KubernetesName: "KubernetesName",
					},
				},

				secret: &corev1.Secret{},
			},
			storeClientResult: storeClientResult{"", nil},
			want:              map[string][]byte{},
			wantErr:           true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := testSecretStoreClient{
				GetSecretValueFunc: func() (string, error) {
					return tt.storeClientResult.value, tt.storeClientResult.err
				},
			}
			keyvaultSecret := &keyvaultsecretv1alpha1.KeyvaultSecret{
				Spec: keyvaultsecretv1alpha1.KeyvaultSecretSpec{
					Items: tt.args.items,
				},
			}
			converter := SecretConverter{
				keyvaultSecret: keyvaultSecret,
				storeClient:    client,
			}
			secret, err := converter.getK8sSecret()
			if (err != nil) != tt.wantErr {
				t.Errorf("getK8sSecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(secret.Data, tt.want) {
				t.Errorf("getK8sSecret() = %v, want %v ", secret.Data, tt.want)
			}
		})
	}
}
