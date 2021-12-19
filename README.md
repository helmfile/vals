# vals

`vals` is a tool for managing configuration values and secrets.

It supports various backends including:

- Vault
- AWS SSM Parameter Store
- AWS Secrets Manager
- AWS S3
- GCP Secrets Manager
- [SOPS](https://github.com/mozilla/sops)-encrypted files
- Terraform State
- CredHub(Coming soon)

- Use `vals eval -f refs.yaml` to replace all the `ref`s in the file to actual values and secrets.
- Use `vals exec -f env.yaml -- <COMMAND>` to populate envvars and execute the command.
- Use `vals env -f env.yaml` to render envvars that are consumable by `eval` or a tool like `direnv`

## Usage

- [CLI](#cli)
- [Helm](#helm)
- [Go](#go)

# CLI

```
vals is a Helm-like configuration "Values" loader with support for various sources and merge strategies

Usage:
  vals [command]

Available Commands:
  eval		Evaluate a JSON/YAML document and replace any template expressions in it and prints the result
  exec		Populates the environment variables and executes the command
  env		Renders environment variables to be consumed by eval or a tool like direnv
  ksdecode	Decode YAML document(s) by converting Secret resources' "data" to "stringData" for use with "vals eval"

Use "vals [command] --help" for more infomation about a command
```

`vals` has a collection of providers that each an be referred with a URI scheme looks `ref+<TYPE>`.

For this example, use the [Vault](https://www.terraform.io/docs/providers/vault/index.html) provider.

Let's start by writing some secret value to `Vault`:

```console
$ vault kv put secret/foo mykey=myvalue
```

Now input the template of your YAML and refer to `vals`' Vault provider by using `ref+vault` in the URI scheme:

```console
$ VAULT_TOKEN=yourtoken VAULT_ADDR=http://127.0.0.1:8200/ \
  echo "foo: ref+vault://secret/data/foo?proto=http#/mykey" | vals eval -f -
```

Voila! `vals`, replacing every reference to your secret value in Vault, produces the output looks like:

```yaml
foo: myvalue
```

Which is equivalent to that of the following shell script:

```bash
VAULT_TOKEN=yourtoken  VAULT_ADDR=http://127.0.0.1:8200/ cat <<EOF
foo: $(vault kv get -format json secret/foo | jq -r .data.data.mykey)
EOF
```

Save the YAML content to `x.vals.yaml` and running `vals eval -f x.vals.yaml` does produce output equivalent to the previous one:

```yaml
foo: myvalue
```

### Helm

Use value references as Helm Chart values, so that you can feed the `helm template` output to `vals -f -` for transforming the refs to secrets.

```console
$ helm template mysql-1.3.2.tgz --set mysqlPassword='ref+vault://secret/data/foo#/mykey' | vals ksdecode -o yaml -f - | tee manifests.yaml
apiVersion: v1
kind: Secret
metadata:
  labels:
    app: release-name-mysql
    chart: mysql-1.3.2
    heritage: Tiller
    release: release-name
  name: release-name-mysql
  namespace: default
stringData:
  mysql-password: refs+vault://secret/data/foo#/mykey
  mysql-root-password: vZQmqdGw3z
type: Opaque
```

This manifest is safe to be committed into your version-control system(GitOps!) as it doesn't contain actual secrets.

When you finally deploy the manifests, run `vals eval` to replace all the refs to actual secrets:

```console
$ cat manifests.yaml | ~/p/values/bin/vals eval -f - | tee all.yaml
apiVersion: v1
kind: Secret
metadata:
    labels:
        app: release-name-mysql
        chart: mysql-1.3.2
        heritage: Tiller
        release: release-name
    name: release-name-mysql
    namespace: default
stringData:
    mysql-password: myvalue
    mysql-root-password: 0A8V1SER9t
type: Opaque
```

Finally run `kubectl apply` to apply manifests:

```console
$ kubectl apply -f all.yaml
```

This gives you a solid foundation for building a secure CD system as you need to allow access to a secrets store like Vault only from servers or containers that pulls safe manifests and runs deployments.

In other words, you can safely omit access from the CI to the secrets store.

### Go

```go
import "github.com/variantdev/vals"

secretsToCache := 256 // how many secrets to keep in LRU cache
runtime, err := vals.New(secretsToCache)
if err != nil {
  return nil, err
}

valsRendered, err := runtime.Eval(map[string]interface{}{
    "inline": map[string]interface{}{
        "foo": "ref+vault://127.0.0.1:8200/mykv/foo?proto=http#/mykey",
        "bar": map[string]interface{}{
            "baz": "ref+vault://127.0.0.1:8200/mykv/foo?proto=http#/mykey",
        },
    },
})
```

Now, `vals` contains a `map[string]interface{}` representation of the below:

```console
cat <<EOF
foo: $(vault read mykv/foo -o json | jq -r .mykey)
  bar:
    baz: $(vault read mykv/foo -o json | jq -r .mykey)
EOF
```

## Suported Backends

- [Vault](#vault)
- [AWS SSM Parameter Store](#aws-ssm-parameter-store)
- [AWS Secrets Manager](#aws-secrets-manager)
- [AWS S3](#aws-s3)
- [GCP Secrets Manager](#gcp-secrets-manager)
- [Google GCS](#google-gcs)
- [SOPS](#sops) powered by [sops](https://github.com/mozilla/sops)
- [Terraform (tfstate)](#terraform-tfstate) powered by [tfstate-lookup](https://github.com/fujiwara/tfstate-lookup)
- [Echo](#echo)
- [File](#file)
- [Azure Key Vault](#azure-key-vault)

Please see [pkg/providers](https://github.com/variantdev/vals/tree/master/pkg/providers) for the implementations of all the providers. The package names corresponds to the URI schemes.

### Vault

- `ref+vault://PATH/TO/KVBACKEND[?address=VAULT_ADDR:PORT&token_file=PATH/TO/FILE&token_env=VAULT_TOKEN]#/fieldkey`
- `ref+vault://PATH/TO/KVBACKEND[?address=VAULT_ADDR:PORT&auth_method=approle&role_id=ce5e571a-f7d4-4c73-93dd-fd6922119839&secret_id=5c9194b9-585e-4539-a865-f45604bd6f56]#/fieldkey`

`address` defaults to the value of the `VAULT_ADDR` envvar.
`auth_method` default to `token` and can also be set to the value of the `VAULT_AUTH_METHOD` envar.
`role_id` defaults to the value of the `VAULT_ROLE_ID` envvar.
`secret_id` defaults to the value of the `VAULT_SECRET_ID` envvar.
`version` is the specific version of the secret to be obtained. Used when you want to get a previous content of the secret.

Examples:

- `ref+vault://mykv/foo#/bar?address=https://vault1.example.com:8200` reads the value for the field `bar` in the kv `foo` on Vault listening on `https://vault1.example.com` with the Vault token read from **the envvar `VAULT_TOKEN`, or the file `~/.vault_token` when the envvar is not set**
- `ref+vault://mykv/foo#/bar?token_env=VAULT_TOKEN_VAULT1&address=https://vault1.example.com:8200` reads the value for the field `bar` in the kv `foo` on Vault listening on `https://vault1.example.com` with the Vault token read from **the envvar `VAULT_TOKEN_VAULT1`**
- `ref+vault://mykv/foo#/bar?token_file=~/.vault_token_vault1&address=https://vault1.example.com:8200` reads the value for the field `bar` in the kv `foo` on Vault listening on `https://vault1.example.com` with the Vault token read from **the file `~/.vault_token_vault1`**

### AWS

There are two providers for AWS:

- SSM Parameter Store
- Secrets Manager

Both provider have support for specifying AWS region and profile via envvars or options:

- AWS profile can be specified via an option `profile=AWS_PROFILE_NAME` or envvar `AWS_PROFILE`
- AWS region can be specified via an option `region=AWS_REGION_NAME` or envvar `AWS_DEFAULT_REGION`

#### AWS SSM Parameter Store

- `ref+awsssm://PATH/TO/PARAM[?region=REGION]`
- `ref+awsssm://PREFIX/TO/PARAMS[?region=REGION&mode=MODE&version=VERSION]#/PATH/TO/PARAM`

The first form result in a `GetParameter` call and result in the reference to be replaced with the value of the parameter.

The second form is handy but fairly complex.

- If `mode` is not set, `vals` uses `GetParametersByPath(/PREFIX/TO/PARAMS)` caches the result per prefix rather than each single path to reduce number of API calls
- If `mode` is `singleparam`, `vals` uses `GetParameter` to obtain the value parameter for key `/PREFIX/TO/PARAMS`, parse the value as a YAML hash, extract the value at the yaml path `PATH.TO.PARAM`.
  - When `version` is set, `vals` uses `GetParameterHistoryPages` instead of `GetParameter`.

For the second form, you can optionally specify `recursive=true` to enable the recursive option of the GetParametersByPath API.

Let's say you had a number of parameters like:

```
NAME        VALUE
/foo/bar    {"BAR":"VALUE"}
/foo/bar/a  A
/foo/bar/b  B
```

- `ref+awsssm://foo/bar` and `ref+awsssm://foo#/bar` results in `{"BAR":"VALUE"}`
- `ref+awsssm://foo/bar/a`, `ref+awsssm://foo/bar?#/a`, and `ref+awsssm://foo?recursive=true#/bar/a` results in `A`
- `ref+awsssm://foo/bar?mode=singleparam#/BAR` results in `VALUE`.

On the other hand,

- `ref+awsssm://foo/bar#/BAR` fails because `/foo/bar` evaluates to `{"a":"A","b":"B"}`.
- `ref+awsssm://foo?recursive=true#/bar` fails because `/foo?recursive=true` internal evaluates to `{"foo":{"a":"A","b":"B"}}`

#### AWS Secrets Manager

- `ref+awssecrets://PATH/TO/SECRET[?region=REGION&version_stage=STAGE&version_id=ID]`
- `ref+awssecrets://PATH/TO/SECRET[?region=REGION&version_stage=STAGE&version_id=ID]#/yaml_or_json_key/in/secret`
- `ref+awssecrets://ACCOUNT:ARN:secret:/PATH/TO/PARAM[?region=REGION]`

The third form allows you to reference a secret in another AWS account (if your cross-account secret permissions are configured).

Examples:

- `ref+awssecrets://myteam/mykey`
- `ref+awssecrets://myteam/mydoc#/foo/bar`
- `ref+awssecrets://myteam/mykey?region=us-west-2`
- `ref+awssecrets:///arn:aws:secretsmanager:<REGION>:<ACCOUNT_ID>:secret:/myteam/mydoc/?region=ap-southeast-2#/secret/key`

#### AWS S3

- `ref+s3://BUCKET/KEY/OF/OBJECT[?region=REGION&profile=AWS_PROFILE&version_id=ID]`
- `ref+s3://BUCKET/KEY/OF/OBJECT[?region=REGION&profile=AWS_PROFILE&version_id=ID]#/yaml_or_json_key/in/secret`

Examples:

- `ref+s3://mybucket/mykey`
- `ref+s3://mybucket/myjsonobj#/foo/bar`
- `ref+s3://mybucket/myyamlobj#/foo/bar`
- `ref+s3://mybucket/mykey?region=us-west-2`
- `ref+s3://mybucket/mykey?profile=prod`

#### Google GCS
- `ref+gcs://BUCKET/KEY/OF/OBJECT[?generation=ID]`
- `ref+gcs://BUCKET/KEY/OF/OBJECT[?generation=ID]#/yaml_or_json_key/in/secret`

Examples:

- `ref+gcs://mybucket/mykey`
- `ref+gcs://mybucket/myjsonobj#/foo/bar`
- `ref+gcs://mybucket/myyamlobj#/foo/bar`
- `ref+gcs://mybucket/mykey?generation=1639567476974625`

### GCP Secrets Manager

- `ref+gcpsecrets://PROJECT/SECRET[?version=VERSION]`
- `ref+gcpsecrets://PROJECT/SECRET[?version=VERSION]#/yaml_or_json_key/in/secret`

Examples:

- `ref+gcpsecrets://myproject/mysecret`
- `ref+gcpsecrets://myproject/mysecret?version=3`
- `ref+gcpsecrets://myproject/mysecret?version=3#/yaml_or_json_key/in/secret`

> NOTE: Got an error like `expand gcpsecrets://project/secret-name?version=1: failed to get secret: rpc error: code = PermissionDenied desc = Request had insufficient authentication scopes.`?
>
> In some cases like you need to use an alternative credentials or project,
> you'll likely need to set `GOOGLE_APPLICATION_CREDENTIALS` and/or `GCP_PROJECT` envvars.

### Terraform (tfstate)

- `ref+tfstate://relative/path/to/some.tfstate/RESOURCE_NAME`
- `ref+tfstate:///absolute/path/to/some.tfstate/RESOURCE_NAME`

Examples:

- `ref+tfstate://path/to/some.tfstate/aws_vpc.main.id`
- `ref+tfstate://path/to/some.tfstate/module.mymodule.aws_vpc.main.id`
- `ref+tfstate://path/to/some.tfstate/output.OUTPUT_NAME.value`
- `ref+tfstate://path/to/some.tfstate/data.thetype.name.foo.bar`

When you're using [terraform-aws-vpc](https://github.com/terraform-aws-modules/terraform-aws-vpc) to define a `module "vpc"` resource and you wanted to grab the first vpc ARN created by the module:

```
$ tfstate-lookup -s ./terraform.tfstate module.vpc.aws_vpc.this[0].arn
arn:aws:ec2:us-east-2:ACCOUNT_ID:vpc/vpc-0cb48a12e4df7ad4c

$ echo 'foo: ref+tfstate://terraform.tfstate/module.vpc.aws_vpc.this[0].arn' | vals eval -f -
foo: arn:aws:ec2:us-east-2:ACCOUNT_ID:vpc/vpc-0cb48a12e4df7ad4c
```

You can also grab a Terraform output by using `output.OUTPUT_NAME.value` like:

```
$ tfstate-lookup -s ./terraform.tfstate output.mystack_apply.value
```

which is equivalent to the following input for `vals`:

```
$ echo 'foo: ref+tfstate://terraform.tfstate/output.mystack_apply.value' | vals eval -f -
```

Remote backends like S3, GCS and AzureRM blob store are also supported. When a remote backend is used in your terraform workspace, there should be a local file at `./terraform/terraform.tfstate` that contains the reference to the backend:

```
{
    "version": 3,
    "serial": 1,
    "lineage": "f1ad69de-68b8-9fe5-7e87-0cb70d8572c8",
    "backend": {
        "type": "s3",
        "config": {
            "access_key": null,
            "acl": null,
            "assume_role_policy": null,
            "bucket": "yourbucketnname",
```

Just specify the path to that file, so that `vals` is able to transparently make the remote state contents available for you.

### Terraform in GCS bucket (tfstategs)

- `ref+tfstategs://bucket/path/to/some.tfstate/RESOURCE_NAME`

Examples:

- `ref+tfstategs://bucket/path/to/some.tfstate/google_compute_disk.instance.id`

It allows to use Terraform state stored in GCS bucket with the direct URL to it. You can try to read the state with command: 

```
$ tfstate-lookup -s gs://bucket-with-terraform-state/terraform.tfstate google_compute_disk.instance.source_image_id
5449927740744213880
```

which is equivalent to the following input for `vals`:

```
$ echo 'foo: ref+tfstategs://bucket-with-terraform-state/terraform.tfstate/google_compute_disk.instance.source_image_id' | vals eval -f -
```

### Terraform in S3 bucket (tfstates3)

- `ref+tfstates3://bucket/path/to/some.tfstate/RESOURCE_NAME`

Examples:

- `ref+tfstates3://bucket/path/to/some.tfstate/aws_vpc.main.id`

It allows to use Terraform state stored in AWS S3 bucket with the direct URL to it. You can try to read the state with command: 

```
$ tfstate-lookup -s s3://bucket-with-terraform-state/terraform.tfstate module.vpc.aws_vpc.this[0].arn
arn:aws:ec2:us-east-2:ACCOUNT_ID:vpc/vpc-0cb48a12e4df7ad4c
```

which is equivalent to the following input for `vals`:

```
$ echo 'foo: ref+tfstates3://bucket-with-terraform-state/terraform.tfstate/module.vpc.aws_vpc.this[0].arn' | vals eval -f -
```
### Terraform in AzureRM Blob storage (tfstateazurerm)

- `ref+tfstateazurerm://{resource_group_name}/{storage_account_name}/{container_name}/{blob_name}.tfstate/RESOURCE_NAME`

Examples:

- `ref+tfstateazurerm://my_rg/my_storage_account/terraform-backend/unique.terraform.tfstate/output.virtual_network.name`

It allows to use Terraform state stored in Azure Blob storage given the resource group, storage account, container name and blob name. You can try to read the state with command:

```
$ tfstate-lookup -s azurerm://my_rg/my_storage_account/terraform-backend/unique.terraform.tfstate output.virtual_network.name
```

which is equivalent to the following input for `vals`:

```
$ echo 'foo: ref+tfstateazurerm://my_rg/my_storage_account/terraform-backend/unique.terraform.tfstate/output.virtual_network.name' | vals eval -f -
```
### SOPS

- The whole content of a SOPS-encrypted file: `ref+sops://base64_data_or_path_to_file?key_type=[filepath|base64]&format=[binary|dotenv|yaml]`
- The value for the specific path in an encrypted YAML/JSON document: `ref+sops://base64_data_or_path_to_file#/json_or_yaml_key/in/the_encrypted_doc`

Examples:

- `ref+sops://path/to/file` reads `path/to/file` as `binary` input
- `ref+sops://<base64>?key_type=base64` reads `<base64>` as the base64-encoded data to be decrypted by sops as `binary`
- `ref+sops://path/to/file#/foo/bar` reads `path/to/file` as a `yaml` file and returns the value at `foo.bar`.
- `ref+sops://path/to/file?format=json#/foo/bar` reads `path/to/file` as a `json` file and returns the value at `foo.bar`.

### Echo

Echo provider echoes the string for testing purpose. Please read [the original proposal](https://github.com/roboll/helmfile/pull/920#issuecomment-548213738) to get why we might need this.

- `ref+echo://KEY1/KEY2/VALUE[#/path/to/the/value]`

Examples:

- `ref+echo://foo/bar` generates `foo/bar`
- `ref+echo://foo/bar/baz#/foo/bar` generates `baz`. This works by the host and the path part `foo/bar/baz` generating an object `{"foo":{"bar":"baz"}}` and the fragment part `#/foo/bar` results in digging the object to obtain the value at `$.foo.bar`.


### File

File provider reads a local text file, or the value for the specific path in a YAML/JSON file.

- `ref+file://relative/path/to/file[#/path/to/the/value]`
- `ref+file:///absolute/path/to/file[#/path/to/the/value]`

Examples:

- `ref+file://foo/bar` loads the file at `foo/bar`
- `ref+file:///home/foo/bar` loads the file at `/home/foo/bar`
- `ref+file://some.yaml#/foo/bar` loads the YAML file at `some.yaml` and reads the value for the path `$.foo.bar`.
  Let's say `some.yaml` contains `{"foo":{"bar":"BAR"}}`, `key1: ref+file://some.yaml#/foo/bar` results in `key1: BAR`.

### Azure Key Vault

Retrieve secrets from Azure Key Vault. Path is used to specify the vault and secret name. Optionally a specific secret version can be retrieved.

- `ref+azurekeyvault://VAULT-NAME/SECRET-NAME[/VERSION]`

VAULT-NAME is either a simple name if operating in AzureCloud (vault.azure.net) or the full endpoint dns name when operating against non-default azure clouds (US Gov Cloud, China Cloud, German Cloud).
Examples:
- `ref+azurekeyvault://my-vault/secret-a`
- `ref+azurekeyvault://my-vault/secret-a/ba4f196b15f644cd9e949896a21bab0d`
- `ref+azurekeyvault://gov-cloud-test.vault.usgovcloudapi.net/secret-b`

#### Authentication

Vals aquires Azure credentials though Azure CLI or from environment variables. The easiest way is to run `az login`. Vals can then aquire the current credentials from `az` without further set up. 

Other authentication methods require information to be passed in environment variables. See [Azure SDK docs](https://docs.microsoft.com/en-us/azure/developer/go/azure-sdk-authorization#use-environment-based-authentication) and [auth.go](https://godoc.org/github.com/Azure/go-autorest/autorest/azure/auth#NewAuthorizerFromEnvironment) for the full list of supported environment variables.

For example, if using client credentials the required env vars are `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`, `AZURE_TENANT_ID` and possibly `AZURE_ENVIRONMENT` in case of accessing an Azure GovCloud.

The order in which authentication methods are checked is:
1. Client credentials
2. Client certificate
3. Username/Password
4. Azure CLI or Managed identity (set environment `AZURE_USE_MSI=true` to enabled MSI)


## Advanced Usages

### Discriminating config and secrets

`vals` has an advanced feature that helps you to do GitOps.

`GitOps` is a good practice that helps you to review how your change would affect the production environment.

To best leverage GitOps, it is important to remove dynamic aspects of your config before reviewing.

On the other hand, `vals`'s primary purpose is to defer retrieval of values until the time of deployment, so that we won't accidentally git-commit secrets. The flip-side of this is, obviously, that you can't review the values themselves.

Using `ref+<value uri>` and `secretref+<value uri>` in combination with `vals eval --exclude-secretref` helps it.

By using the `secretref+<uri>` notation, you tell `vals` that it is a secret and regular `ref+<uri>` instances are for config values.

```yaml
myconfigvalue: ref+awsssm://myconfig/value
mysecretvalue: secretref+awssecrets://mysecret/value
```

To leverage `GitOps` most by allowing you to review the content of `ref+awsssm://myconfig/value` only, you run `vals eval --exclude-secretref` to generate the following:

```yaml
myconfigvalue: MYCONFIG_VALUE
mysecretvalue: secretref+awssecrets://mysecret/value
```

This is safe to be committed into git because, as you've told to `vals`, `awsssm://myconfig/value` is a config value that can be shared publicly.

## Non-Goals

### String-Interpolation / Template Functions

In the early days of this project, the original author has investigated if it was a good idea to introduce string interpolation like feature to vals:

```
foo: xx${{ref "ref+vault://127.0.0.1:8200/mykv/foo?proto=http#/mykey" }}
bar:
  baz: yy${{ref "ref+vault://127.0.0.1:8200/mykv/foo?proto=http#/mykey" }}
```

But the idea had abandoned due to that it seemed to drive the momentum to vals being a full-fledged YAML templating engine. What if some users started wanting to use `vals` for transforming values with functions?
That's not the business of vals.

Instead, use vals solely for composing sets of values that are then input to another templating engine or data manipulation language like Jsonnet and CUE.

### Merge

Merging YAMLs is out of the scope of `vals`. There're better alternatives like Jsonnet, Sprig, and CUE for the job.
