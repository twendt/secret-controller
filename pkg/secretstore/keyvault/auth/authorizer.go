package auth

import (
	"net/url"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
)

var (
	armAuthorizer      autorest.Authorizer
	batchAuthorizer    autorest.Authorizer
	graphAuthorizer    autorest.Authorizer
	keyvaultAuthorizer autorest.Authorizer
)

const (
	vaultEndpoint               string = "https://vault.azure.net"
	activeDirectoryEndpoint     string = "https://login.microsoftonline.com/"
	keyVaultAuthorizeEndpoint   string = "https://login.windows.net/"
	keyVaultAuthorizePathSuffix string = "/oauth2/token"
)

// GetKeyvaultAuthorizer gets an OAuthTokenAuthorizer for use with Key Vault
// keys and secrets. Note that Key Vault *Vaults* are managed by Azure Resource
// Manager.
func GetKeyvaultAuthorizer(tenantID, clientID, clientSecret string) (autorest.Authorizer, error) {
	if keyvaultAuthorizer != nil {
		return keyvaultAuthorizer, nil
	}

	// BUG: default value for KeyVaultEndpoint is wrong
	// vaultEndpoint := "https://vault.azure.net"
	// vaultEndpoint := strings.TrimSuffix(config.Environment().KeyVaultEndpoint, "/")
	// BUG: alternateEndpoint replaces other endpoints in the configs below
	alternateEndpoint, _ := url.Parse(
		keyVaultAuthorizeEndpoint + tenantID + keyVaultAuthorizePathSuffix)

	var a autorest.Authorizer
	var err error

	oauthconfig, err := adal.NewOAuthConfig(
		activeDirectoryEndpoint, tenantID)
	if err != nil {
		return a, err
	}
	oauthconfig.AuthorizeEndpoint = *alternateEndpoint

	token, err := adal.NewServicePrincipalToken(
		*oauthconfig, clientID, clientSecret, vaultEndpoint)
	if err != nil {
		return a, err
	}

	a = autorest.NewBearerAuthorizer(token)

	if err == nil {
		keyvaultAuthorizer = a
	} else {
		keyvaultAuthorizer = nil
	}

	return keyvaultAuthorizer, err
}
