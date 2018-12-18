# Azure Key Vault Secret Controller

This repository implements a simple Kubernetes custom controller to load secrets from Azure Key Vault and store them as Kubernetes secrets.

The idea is to store all your secrets in Azure Key Vault and then deploy KeyvaultSecret resources into Kubernetes. The secret-controller will listen for those resources and create the corresponding Kubernetes secrets for them. Within your pods you can then use these secrets just like any other secret. Kubernetes will also make sure that the pods will not start before the required secrets are created.

Deleting the KeyvaultSecret will also delete the Kubernetes secret.

## Build

Clone the repo into your GOPATH

```
mkdir -p $GOPATH/src/github.com/twendt
cd $GOPATH/src/github.com/twendt
git clone https://github.com/twendt/secret-controller.git
cd secret-controller
```
### Local

```
go get
go build
```

### Docker

```
docker build -t secret-controller .
```

To speed up the builds there are also the file `Dockerfile.dependencies` and `Dockerfile.build`. Run them as follows so that Docker does not have to run `go get` on every build.

```
docker build -f Dockerfile.dependencies -t secret-controller-deps .

docker build -f Dockerfile.build -t secret-controller .
```

## Usage

### Deploy secret-controller

There is an example Helm chart in the `helm` directory. At least the following values have to be configured in `values.yaml`:

* repoBase This is the Docker registry. There is no pre-built image available in Dockerhub
* vaultName The name of the Azure Key vault to use

See `values.yaml` for further parameters.

### Run secret-controller locally

The secret-controller can also be run locally for testing purposes.

```
export KEYVAULT_TENANT_ID=<enter your tenant ID here>
export KEYVAULT_CLIENT_ID=<enter your client ID here>
export KEYVAULT_CLIENT_SECRET=<enter your client secret here>
./secret-controller --vault-name <name of Key Vault> --master <api server address> --kubeconfig <path to kubeconfig>
```

### Deploy secrets

The secret-controller provides a new resource `keyvaultsecrets.secretcontroller.twendt.de`. This resource can be used as follows:

```
apiVersion: secretcontroller.twendt.de/v1alpha1
kind: KeyvaultSecret
metadata:
  name: any-secret
spec:
  items:
    - keyvaultName: postgres
      kubernetesName: PG_USER
    - secretTemplate: 'postgresql://[[ secretValue "PG-USER" ]]:[[ secretValue "PG-PASSWORD" ]]@pghost:5432/dbname'
      kubernetesName: PG_DSN
```

The Kubernetes secret being created will get the same name as the KeyvaultSecret. This creates a 1:1 relationship between the 2. This also makes sure that it is not possible to define 2 KeyvaultSecrets that will create the same Kubernetes secret.

The items in the manifest define the entries within the secret that will be created.

As you can see there are 2 ways to define the items:

**keyvaultName and kubernetesName**

The `kubernetesName` defines the key that will be used within the Kubernetes secret.

`keyvaultName` simply defines the name of the secret in Key Vault and will be used as the value of the secret entry.

**secretTemplate and kubernetesName**

The `kubernetesName` defines the key that will be used within the Kubernetes secret.

The `secretTemplate` provides a very flexible way to define the value of the secret entry. You can use a combination of static text and Go templates. To be able to use this in Helm charts, the secret-controller uses `[[` and `]]` as template markers. This makes sure that Helm does not evaluate them. This also enables the possibility to use variables that Helm can replace so that you can for instance have Helm evaluate the hostname in the above example by using something like `{{ .Values.postgres.hostname }}` instead of pghost.

The secret-controller provides 2 template functions to retrieve the secret values from Azure Key Vault:
* secretValue This retrieves the latest version of the secret from Azure Key Vault
* secretValueForVersion This retrieves a specific version of the secret from Azure Key Vault. The version has to be passed as second string parameter
