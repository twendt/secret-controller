package keyvault

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/twendt/secret-controller/pkg/secretstore/keyvault/auth"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/2016-10-01/keyvault"
)

const (
	azureJSONPath string = "/etc/kubernetes/azure.json"
	userAgent     string = "secrets-initializer"
)

type Client struct {
	keyvaultClient *keyvault.BaseClient
	url            string
}

type AzureJSON struct {
	Cloud                        string  `json:"cloud"`
	TenantID                     string  `json:"tenantId"`
	SubscriptionID               string  `json:"subscriptionId"`
	AadClientID                  string  `json:"aadClientId"`
	AadClientSecret              string  `json:"aadClientSecret"`
	ResourceGroup                string  `json:"resourceGroup"`
	Location                     string  `json:"location"`
	VMType                       string  `json:"vmType"`
	SubnetName                   string  `json:"subnetName"`
	SecurityGroupName            string  `json:"securityGroupName"`
	VnetName                     string  `json:"vnetName"`
	VnetResourceGroup            string  `json:"vnetResourceGroup"`
	RouteTableName               string  `json:"routeTableName"`
	PrimaryAvailabilitySetName   string  `json:"primaryAvailabilitySetName"`
	PrimaryScaleSetName          string  `json:"primaryScaleSetName"`
	CloudProviderBackoff         bool    `json:"cloudProviderBackoff"`
	CloudProviderBackoffRetries  int64   `json:"cloudProviderBackoffRetries"`
	CloudProviderBackoffExponent float64 `json:"cloudProviderBackoffExponent"`
	CloudProviderBackoffDuration int64   `json:"cloudProviderBackoffDuration"`
	CloudProviderBackoffJitter   int64   `json:"cloudProviderBackoffJitter"`
	CloudProviderRatelimit       bool    `json:"cloudProviderRatelimit"`
	CloudProviderRateLimitQPS    int64   `json:"cloudProviderRateLimitQPS"`
	CloudProviderRateLimitBucket int64   `json:"cloudProviderRateLimitBucket"`
	UseManagedIdentityExtension  bool    `json:"useManagedIdentityExtension"`
	UserAssignedIdentityID       string  `json:"userAssignedIdentityID"`
	UseInstanceMetadata          bool    `json:"useInstanceMetadata"`
	LoadBalancerSku              string  `json:"loadBalancerSku"`
	ExcludeMasterFromStandardLB  bool    `json:"excludeMasterFromStandardLB"`
	ProviderVaultName            string  `json:"providerVaultName"`
	ProviderKeyName              string  `json:"providerKeyName"`
	ProviderKeyVersion           string  `json:"providerKeyVersion"`
}

func NewVaultClient(name string) (Client, error) {
	if name == "" {
		return Client{}, fmt.Errorf("No Vault Name set")
	}

	client := Client{
		url: "https://" + name + ".vault.azure.net/",
	}

	tenantID := os.Getenv("KEYVAULT_TENANT_ID")
	clientID := os.Getenv("KEYVAULT_CLIENT_ID")
	clientSecret := os.Getenv("KEYVAULT_CLIENT_SECRET")

	var keyvaultClient *keyvault.BaseClient
	if tenantID != "" && clientID != "" && clientSecret != "" {
		var err error
		keyvaultClient, err = getVaultClient(tenantID, clientID, clientSecret)
		if err != nil {
			return client, err
		}
	} else {
		var err error
		keyvaultClient, err = getVaultClientFromAzureJSON()
		if err != nil {
			return client, err
		}
	}
	client.keyvaultClient = keyvaultClient
	return client, nil
}

func (c Client) GetSecretValue(name string) (string, error) {
	return c.GetSecretValueForVersion(name, "")
}

func (c Client) GetSecretValueForVersion(name, version string) (string, error) {
	ctx := context.Background()
	secret, err := c.keyvaultClient.GetSecret(ctx, c.url, name, version)
	if err != nil {
		return "", err
	}

	return *secret.Value, nil
}

func getVaultClient(tenantID, clientID, clientSecret string) (*keyvault.BaseClient, error) {
	vaultClient := keyvault.New()
	a, err := auth.GetKeyvaultAuthorizer(tenantID, clientID, clientSecret)
	if err != nil {
		return nil, err
	}

	vaultClient.Authorizer = a
	if err = vaultClient.AddToUserAgent(userAgent); err != nil {
		return nil, err
	}

	return &vaultClient, nil
}

func getVaultClientFromAzureJSON() (*keyvault.BaseClient, error) {
	jsonFile, err := os.Open(azureJSONPath)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	jsonBytes, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}
	jsonFile.Close()

	var azureJSON AzureJSON
	if err = json.Unmarshal(jsonBytes, &azureJSON); err != nil {
		return nil, err
	}

	vaultClient, err := getVaultClient(azureJSON.TenantID, azureJSON.AadClientID, azureJSON.AadClientSecret)
	if err != nil {
		return nil, err
	}

	return vaultClient, nil
}
