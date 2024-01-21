# vals

`vals` is a tool for managing configuration values and secrets.

It supports various backends including:

- Vault
- AWS SSM Parameter Store
- AWS Secrets Manager
- AWS S3
- GCP Secrets Manager
- GCP KMS
- [Google Sheets](#google-sheets)
- [SOPS](https://github.com/getsops/sops)-encrypted files
- Terraform State
- 1Password Connect
- [Doppler](https://doppler.com/)
- CredHub(Coming soon)
- Pulumi State
- Kubernetes
- Conjur

- Use `vals eval -f refs.yaml` to replace all the `ref`s in the file to actual values and secrets.
- Use `vals exec -f env.yaml -- <COMMAND>` to populate envvars and execute the command.
- Use `vals env -f env.yaml` to render envvars that are consumable by `eval` or a tool like `direnv`

ToC:

- [Usage](#usage)
  - [CLI](#cli)
  - [Helm](#helm)
  - [Go](#go)
- [Expression Syntax](#expression-syntax)
- [Supported Backends](#supported-backends)

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
  eval          Evaluate a JSON/YAML document and replace any template expressions in it and prints the result
  exec          Populates the environment variables and executes the command
  env           Renders environment variables to be consumed by eval or a tool like direnv
  get           Evaluate a string value passed as the first argument and replace any expressiosn in it and prints the result
  ksdecode      Decode YAML document(s) by converting Secret resources' "data" to "stringData" for use with "vals eval"
  version       Print vals version

Use "vals [command] --help" for more information about a comman
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
  mysql-password: ref+vault://secret/data/foo#/mykey
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
import "github.com/helmfile/vals"

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

## Expression Syntax

`vals` finds and replaces every occurrence of `ref+BACKEND://PATH[?PARAMS][#FRAGMENT][+]` URI-like expression within the string at the value position with the retrieved secret value.

`BACKEND` is the identifier of one of the [supported backends](#supported-backends).

`PATH` is the backend-specific path for the secret to be retried.

`PARAMS` is key-value pairs where the key and the value are combined using the intermediate "=" character while key-value pairs are combined using "&" characters. It's supposed to be the "query" component of the URI as defined in [RFC3986](https://www.rfc-editor.org/rfc/rfc3986).

`FRAGMENT` is a path-like expression that is used to extract a single value within the secret. When a fragment is specified, `vals` parse the secret value denoted by the `PATH` into a YAML or JSON object, and traverses the object following the fragment, and uses the value at the path as the final secret value. It's supposed to be the "fragment" componet of the URI as defined in [RFC3986](https://www.rfc-editor.org/rfc/rfc3986).

Finally, the optional trailing `+` is the explicit "end" of the expression. You usually don't need it, as if omitted, it treats anything after `ref+` and before the new-line or the end-of-line as an expression to be evaluated. An explicit `+` is handy when you want to do a simple string interpolation. That is, `foo ref+SECRET1+ ref+SECRET2+ bar` evaluates to `foo SECRET1_VALUE SECRET2_VALUE bar`.

Although we mention the RFC for the sake of explanation, `PARAMS` and `FRAGMENT` might not be fully RFC-compliant as, under the hood, we use a simple regexp that seemed to work for most of use-cases.

The regexp is defined as [DefaultRefRegexp](#https://github.com/helmfile/vals/blob/86bccbee4d5f430b7d24b2e3af781336767c0d35/pkg/expansion/expand_match.go#L15) in our code base.

Please see the [relevant unit test cases](https://github.com/helmfile/vals/blob/main/pkg/expansion/expand_match_test.go) for exactly which patterns are supposed to work with `vals`.

## Supported Backends

- [Vault](#vault)
- [AWS SSM Parameter Store](#aws-ssm-parameter-store)
- [AWS Secrets Manager](#aws-secrets-manager)
- [AWS S3](#aws-s3)
- [GCP Secrets Manager](#gcp-secrets-manager)
- [GCP KMS](#gcp-kms)
- [Google Sheets](#google-sheets)
- [Google GCS](#google-gcs)
- [SOPS](#sops) powered by [sops](https://github.com/getsops/sops)
- [Terraform (tfstate)](#terraform-tfstate) powered by [tfstate-lookup](https://github.com/fujiwara/tfstate-lookup)
- [Echo](#echo)
- [File](#file)
- [Azure Key Vault](#azure-key-vault)
- [EnvSubst](#envsubst)
- [GitLab](#gitlab)
- [1Password Connect](#1password-connect)
- [Doppler](#doppler)
- [Pulumi State](#pulumi-state)
- [Kubernetes](#kubernetes)
- [Conjur](#conjur)

Please see [pkg/providers](https://github.com/helmfile/vals/tree/master/pkg/providers) for the implementations of all the providers. The package names corresponds to the URI schemes.

### Vault

- `ref+vault://PATH/TO/KVBACKEND[?address=VAULT_ADDR:PORT&token_file=PATH/TO/FILE&token_env=VAULT_TOKEN&namespace=VAULT_NAMESPACE]#/fieldkey`
- `ref+vault://PATH/TO/KVBACKEND[?address=VAULT_ADDR:PORT&auth_method=approle&role_id=ce5e571a-f7d4-4c73-93dd-fd6922119839&secret_id=5c9194b9-585e-4539-a865-f45604bd6f56]#/fieldkey`
- `ref+vault://PATH/TO/KVBACKEND[?address=VAULT_ADDR:PORT&auth_method=kubernetes&role_id=K8S-ROLE`

* `address` defaults to the value of the `VAULT_ADDR` envvar.
* `namespace` defaults to the value of the `VAULT_NAMESPACE` envvar.
* `auth_method` default to `token` and can also be set to the value of the `VAULT_AUTH_METHOD` envar.
* `role_id` defaults to the value of the `VAULT_ROLE_ID` envvar.
* `secret_id` defaults to the value of the `VAULT_SECRET_ID` envvar.
* `version` is the specific version of the secret to be obtained. Used when you want to get a previous content of the secret.

### Authentication

The `auth_method` or `VAULT_AUTH_METHOD` envar configures how `vals` authenticates to HashiCorp Vault. Currently only these options are supported:

* [approle](https://www.vaultproject.io/docs/auth/approle#via-the-api): it requires you pass on a `role_id` together with a `secret_id`.
* [token](https://www.vaultproject.io/docs/auth/token): you just need creating and passing on a `VAULT_TOKEN`. If `VAULT_TOKEN` isn't set, token can be retrieved from `VAULT_TOKEN_FILE` env or `~/.vault-token` file.
* [kubernetes](https://www.vaultproject.io/docs/auth/kubernetes): if you're running inside a Kubernetes cluster, you can use this option. It requires you [configure](https://www.vaultproject.io/docs/auth/kubernetes#configuration) a policy, a Kubernetes role, a service account and a JWT token. The login path can also be set using the environment variable `VAULT_KUBERNETES_MOUNT_POINT` (default is `/kubernetes`). You must also set `role_id` or `VAULT_ROLE_ID` envar to the Kubernetes role.

Examples:

- `ref+vault://mykv/foo?address=https://vault1.example.com:8200#/bar` reads the value for the field `bar` in the kv `foo` on Vault listening on `https://vault1.example.com` with the Vault token read from **the envvar `VAULT_TOKEN`, or the file `~/.vault_token` when the envvar is not set**
- `ref+vault://mykv/foo?token_env=VAULT_TOKEN_VAULT1&namespace=ns1&address=https://vault1.example.com:8200#/bar` reads the value for the field `bar` from namespace `ns1` in the kv `foo` on Vault listening on `https://vault1.example.com` with the Vault token read from **the envvar `VAULT_TOKEN_VAULT1`**
- `ref+vault://mykv/foo?token_file=~/.vault_token_vault1&address=https://vault1.example.com:8200#/bar` reads the value for the field `bar` in the kv `foo` on Vault listening on `https://vault1.example.com` with the Vault token read from **the file `~/.vault_token_vault1`**
- `ref+vault://mykv/foo?role_id=my-kube-role#/bar` using the Kubernetes role to log in to Vault

### AWS

There are four providers for AWS:

- SSM Parameter Store
- Secrets Manager
- S3
- KMS

Both provider have support for specifying AWS region and profile via envvars or options:

- AWS profile can be specified via an option `profile=AWS_PROFILE_NAME` or envvar `AWS_PROFILE`
- AWS region can be specified via an option `region=AWS_REGION_NAME` or envvar `AWS_DEFAULT_REGION`

#### AWS SSM Parameter Store

- `ref+awsssm://PATH/TO/PARAM[?region=REGION&role_arn=ASSUMED_ROLE_ARN]`
- `ref+awsssm://PREFIX/TO/PARAMS[?region=REGION&role_arn=ASSUMED_ROLE_ARN&mode=MODE&version=VERSION]#/PATH/TO/PARAM`

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

- `ref+awssecrets://PATH/TO/SECRET[?region=REGION&role_arn=ASSUMED_ROLE_ARN&version_stage=STAGE&version_id=ID]`
- `ref+awssecrets://PATH/TO/SECRET[?region=REGION&role_arn=ASSUMED_ROLE_ARN&version_stage=STAGE&version_id=ID]#/yaml_or_json_key/in/secret`
- `ref+awssecrets://ACCOUNT:ARN:secret:/PATH/TO/PARAM[?region=REGION&role_arn=ASSUMED_ROLE_ARN]`

The third form allows you to reference a secret in another AWS account (if your cross-account secret permissions are configured).

Examples:

- `ref+awssecrets://myteam/mykey`
- `ref+awssecrets://myteam/mydoc#/foo/bar`
- `ref+awssecrets://myteam/mykey?region=us-west-2`
- `ref+awssecrets://arn:aws:secretsmanager:<REGION>:<ACCOUNT_ID>:secret:/myteam/mydoc/?region=ap-southeast-2#/secret/key`

#### AWS S3

- `ref+s3://BUCKET/KEY/OF/OBJECT[?region=REGION&profile=AWS_PROFILE&role_arn=ASSUMED_ROLE_ARN&version_id=ID]`
- `ref+s3://BUCKET/KEY/OF/OBJECT[?region=REGION&profile=AWS_PROFILE&role_arn=ASSUMED_ROLE_ARN&version_id=ID]#/yaml_or_json_key/in/secret`

Examples:

- `ref+s3://mybucket/mykey`
- `ref+s3://mybucket/myjsonobj#/foo/bar`
- `ref+s3://mybucket/myyamlobj#/foo/bar`
- `ref+s3://mybucket/mykey?region=us-west-2`
- `ref+s3://mybucket/mykey?profile=prod`

#### AWS KMS

- `ref+awskms://BASE64CIPHERTEXT[?region=REGION&profile=AWS_PROFILE&role_arn=ASSUMED_ROLE_ARN&alg=ENCRYPTION_ALGORITHM&key=KEY_ID&context=URL_ENCODED_JSON]`
- `ref+awskms://BASE64CIPHERTEXT[?region=REGION&profile=AWS_PROFILE&role_arn=ASSUMED_ROLE_ARN&alg=ENCRYPTION_ALGORITHM&key=KEY_ID&context=URL_ENCODED_JSON]#/yaml_or_json_key/in/secret`

Decrypts the URL-safe base64-encoded ciphertext using AWS KMS. Note that URL-safe base64 encoding is
the same as "traditional" base64 encoding, except it uses `_` and `-` in place of `/` and `+`, respectively.
For example, to get a URL-safe base64-encoded ciphertext using the AWS CLI, you might run
```
aws kms encrypt \
  --key-id alias/example \
  --plaintext $(echo -n "hello, world" | base64 -w0) \
  --query CiphertextBlob \
  --output text |
  tr '/+' '_-'
```

Valid values for `alg` include:
* `SYMMETRIC_DEFAULT` (the default)
* `RSAES_OAEP_SHA_1`
* `RSAES_OAEP_SHA_256`

Valid value formats for `key` include:
* A key id `1234abcd-12ab-34cd-56ef-1234567890ab`
* A URL-encoded key id ARN: `arn%3Aaws%3Akms%3Aus-east-2%3A111122223333%3Akey%2F1234abcd-12ab-34cd-56ef-1234567890ab`
* A URL-encoded key alias: `alias%2FExampleAlias`
* A URL-encoded key alias ARN: `arn%3Aaws%3Akms%3Aus-east-2%3A111122223333%3Aalias%2FExampleAlias`

For ciphertext encrypted with a symmetric key, the `key` field may be omitted. For ciphertext
encrypted with a key in your own account, a plain key id or alias can be used. If the encryption
key is from another AWS account, you must use the key or alias ARN.

Use the `context` parameter to optionally specify the encryption context used when encrypting the
ciphertext. Format it by URL-encoding the JSON representation of the encryption context. For example,
if the encryption context is `{"foo":"bar","hello":"world"}`, then you would represent that as
`context=%7B%22foo%22%3A%22bar%22%2C%22hello%22%2C%22world%22%7D`.

Examples:
- `ref+awskms://AQICAHhy_i8hQoGLOE46PVJyinH...WwHKT0i3H0znHRHwfyC7AGZ8ek=`
- `ref+awskms://AQICAHhy...nHRHwfyC7AGZ8ek=#/foo/bar`
- `ref+awskms://AQICAHhy...WwHKT0i3AGZ8ek=?context=%7B%22foo%22%3A%22bar%22%2C%22hello%22%2C%22world%22%7D`
- `ref+awskms://AQICAVJyinH...WwHKT0iC7AGZ8ek=?alg=RSAES_OAEP_SHA1&key=alias%2FExampleAlias`
- `ref+awskms://AQICA...fyC7AGZ8ek=?alg=RSAES_OAEP_SHA256&key=arn%3Aaws%3Akms%3Aus-east-2%3A111122223333%3Akey%2F1234abcd-12ab-34cd-56ef-1234567890ab&context=%7B%22foo%22%3A%22bar%22%2C%22hello%22%2C%22world%22%7D`

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

### GCP KMS

- `ref+gkms://BASE64CIPHERTEXT?project=myproject&location=global&keyring=mykeyring&crypto_key=mykey`
- `ref+gkms://BASE64CIPHERTEXT?project=myproject&location=global&keyring=mykeyring&crypto_key=mykey#/yaml_or_json_key/in/secret`

Decrypts the URL-safe base64-encoded ciphertext using GCP KMS. Note that URL-safe base64 encoding is the same as "traditional" base64 encoding, except it uses _ and - in place of / and +, respectively. For example, to get a URL-safe base64-encoded ciphertext using the GCP CLI, you might run
```
echo test | gcloud kms encrypt \
  --project myproject \
  --location global \
  --keyring mykeyring \
  --key mykey \
  --plaintext-file - \
  --ciphertext-file - \
  | base64 -w0 \
  | tr '/+' '_-'
```

### Google Sheets

- `ref+googlesheets://SPREADSHEET_ID?credentials_file=credentials.json#/KEY`

Examples:

- `ref+googlesheets://foobarbaz?credentials_file=credentials.json#/MYENV1` authenticates Google Sheets API using the credentials.json file, retrieve KVs from the sheet wit the spreadsheet ID "foobarbaz", and retrieves the value for the key "MYENV1". The `credentials.json` can be either a serviceaccount json key file, or client credentials file. In case it's a client credentials file, vals initiates a WebAuth flow and prints the URL. You open the URL with a browser, authenticate yourself there, copy the resulting auth code, input the auth code to vals.

### Terraform (tfstate)

- `ref+tfstate://relative/path/to/some.tfstate/RESOURCE_NAME[?aws_profile=AWS_PROFILE]`
- `ref+tfstate:///absolute/path/to/some.tfstate/RESOURCE_NAME[?aws_profile=AWS_PROFILE]`
- `ref+tfstate://relative/path/to/some.tfstate/RESOURCE_NAME[?az_subscription_id=AZ_SUBSCRIPTION_ID]`
- `ref+tfstate:///absolute/path/to/some.tfstate/RESOURCE_NAME[?az_subscription_id=AZ_SUBSCRIPTION_ID]`

Options:

`aws_profile`: If non-empty, `vals` tries to let tfstate-lookup to use the specified AWS profile defined in the well-known `~/.credentials` file.
`az_subscription_id`: If non-empty, `vals` tries to let tfstate-lookup to use the specified Azure Subscription ID.

Examples:

- `ref+tfstate://path/to/some.tfstate/aws_vpc.main.id`
- `ref+tfstate://path/to/some.tfstate/module.mymodule.aws_vpc.main.id`
- `ref+tfstate://path/to/some.tfstate/output.OUTPUT_NAME`
- `ref+tfstate://path/to/some.tfstate/data.thetype.name.foo.bar`

When you're using [terraform-aws-vpc](https://github.com/terraform-aws-modules/terraform-aws-vpc) to define a `module "vpc"` resource and you wanted to grab the first vpc ARN created by the module:

```
$ tfstate-lookup -s ./terraform.tfstate module.vpc.aws_vpc.this[0].arn
arn:aws:ec2:us-east-2:ACCOUNT_ID:vpc/vpc-0cb48a12e4df7ad4c

$ echo 'foo: ref+tfstate://terraform.tfstate/module.vpc.aws_vpc.this[0].arn' | vals eval -f -
foo: arn:aws:ec2:us-east-2:ACCOUNT_ID:vpc/vpc-0cb48a12e4df7ad4c
```

You can also grab a Terraform output by using `output.OUTPUT_NAME` like:

```
$ tfstate-lookup -s ./terraform.tfstate output.mystack_apply
```

which is equivalent to the following input for `vals`:

```
$ echo 'foo: ref+tfstate://terraform.tfstate/output.mystack_apply' | vals eval -f -
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

- `ref+tfstateazurerm://{resource_group_name}/{storage_account_name}/{container_name}/{blob_name}.tfstate/RESOURCE_NAME[?az_subscription_id=SUBSCRIPTION_ID]`

Examples:

- `ref+tfstateazurerm://my_rg/my_storage_account/terraform-backend/unique.terraform.tfstate/output.virtual_network.name`
- `ref+tfstateazurerm://my_rg/my_storage_account/terraform-backend/unique.terraform.tfstate/output.virtual_network.name?az_subscription_id=abcd-efgh-ijlk-mnop`

It allows to use Terraform state stored in Azure Blob storage given the resource group, storage account, container name and blob name. You can try to read the state with command:

```
$ tfstate-lookup -s azurerm://my_rg/my_storage_account/terraform-backend/unique.terraform.tfstate output.virtual_network.name
```

which is equivalent to the following input for `vals`:

```
$ echo 'foo: ref+tfstateazurerm://my_rg/my_storage_account/terraform-backend/unique.terraform.tfstate/output.virtual_network.name' | vals eval -f -
```

### Terraform in Terraform Cloud / Terraform Enterprise (tfstateremote)

- `ref+tfstateremote://app.terraform.io/{org}/{myworkspace}/RESOURCE_NAME`

Examples:

- `ref+tfstateremote://app.terraform.io/myorg/myworkspace/output.virtual_network.name`

It allows to use Terraform state stored in Terraform Cloud / Terraform Enterprise given the resource group, the organization and the workspace. You can try to read the state with command (with exported variable `TFE_TOKEN`):

```
$ tfstate-lookup -s remote://app.terraform.io/myorg/myworkspace output.virtual_network.name
```

which is equivalent to the following input for `vals`:

```
$ echo 'foo: ref+tfstateremote://app.terraform.io/myorg/myworkspace/output.virtual_network.name' | vals eval -f -
```

### SOPS

- The whole content of a SOPS-encrypted file: `ref+sops://base64_data_or_path_to_file?key_type=[filepath|base64]&format=[binary|dotenv|yaml]`
- The value for the specific path in an encrypted YAML/JSON document: `ref+sops://base64_data_or_path_to_file#/json_or_yaml_key/in/the_encrypted_doc`

Note: When using an inline base64-encoded sops "file", be sure to use URL-safe Base64 encoding.
URL-safe base64 encoding is the same as "traditional" base64 encoding, except it uses `_` and `-` in
place of `/` and `+`, respectively. For example, you might use the following command:
`sops -e <(echo "foo") | base64 -w0 | tr '/+' '_-'`

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
- `ref+file://foo/bar?encode=base64` loads the file at `foo/bar` and encodes its content to a base64 string
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

### EnvSubst

Environment variables substitution.

- `ref+envsubst://$VAR1`

Examples:

- `ref+envsubst://$VAR1` loads environment variables `$VAR1`

### GitLab Secrets

For this provider to work you require an [access token](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html) exported as the environment variable `GITLAB_TOKEN`.


- `ref+gitlab://my-gitlab-server.com/project_id/secret_name?[ssl_verify=false&scheme=https&api_version=v4]`

Examples:

- `ref+gitlab://gitlab.com/11111/password`
- `ref+gitlab://my-gitlab.org/11111/password?ssl_verify=true&scheme=https`

### 1Password Connect

For this provider to work you require a working and accessible [1Password connect server](https://developer.1password.com/docs/connect).
The following env vars have to be configured:
- `OP_CONNECT_HOST`
- `OP_CONNET_TOKEN`

1Password is organized in vaults and items.
An item can have multiple fields with or without a section. Labels can be set on fields and sections.
Vaults, items, sections and labels can be accessed by ID or by label/name (and IDs and labels can be mixed and matched in one URL).

If a section does not have a label the field is only accessible via the section ID. This does not hold true for some default fields which may have no section at all (e.g.username and password for a `Login` item).

*Caution: vals-expressions are parsed as URIs. For the 1Password connect provider the host component of the URI identifies the vault (by ID or name). Therefore vaults containing certain characters not allowed in the host component (e.g. whitespaces, see [RFC-3986](https://www.rfc-editor.org/rfc/rfc3986#section-3.2.2) for details) can only be accessed by ID.*

Examples:

- `ref+onepasswordconnect://VAULT_ID/ITEM_ID#/[SECTION_ID.]FIELD_ID`
- `ref+onepasswordconnect://VAULT_LABEL/ITEM_LABEL#/[SECTION_LABEL.]FIELD_LABEL`
- `ref+onepasswordconnect://VAULT_LABEL/ITEM_ID#/[SECTION_LABEL.]FIELD_ID`

### Doppler

- `ref+doppler://PROJECT/ENVIRONMENT/SECRET_KEY[?token=dp.XX.XXXXXX&address=https://api.doppler.com&no_verify_tls=false&include_doppler_defaults=false]`

* `PROJECT` can be absent if the Token is a `Service Token` for that project. It can be set via `DOPPLER_PROJECT` envvar. See [Doppler docs](https://docs.doppler.com/docs/enclave-service-tokens) for more information.
* `ENVIRONMENT` (aka: "Config") can be absent if the Token is a `Service Token` for that project. It can be set via `DOPPLER_ENVIRONMENT` envvar. See [Doppler docs](https://docs.doppler.com/docs/enclave-service-tokens) for more information.
* `SECRET_KEY` can be absent and it will fetch all secrets for the project/environment.
* `token` defaults to the value of the `DOPPLER_TOKEN` envvar.
* `address` defaults to the value of the `DOPPLER_API_ADDR` envvar, if unset: `https://api.doppler.com`.
* `no_verify_tls` default `false`.
* `include_doppler_defaults` defaults to `false`, if set to `true` it will include the Doppler defaults for the project/environment (DOPPLER_ENVIRONMENT, DOPPLER_PROJECT and DOPPLER_CONFIG). It only works when `SECRET_KEY` is absent.

Examples:

(DOPPLER_TOKEN set as environment variable)

- `ref+doppler:////` fetches all secrets for the project/environment when using a Service Token.
- `ref+doppler:////FOO` fetches the value of secret with name `FOO` for the project/environment when using a Service Token.
- `ref+doppler://#FOO` fetches the value of secret with name `FOO` for the project/environment when using a Service Token.
- `ref+doppler://MyProject/development/DB_PASSWORD` fetches the value of secret with name `DB_PASSWORD` for the project named `MyProject` and environment named `development`.
- `ref+doppler://MyProject/development/#DB_PASSWORD` fetches the value of secret with name `DB_PASSWORD` for the project named `MyProject` and environment named `development`.

### Pulumi State

Obtain value in state pulled from Pulumi Cloud REST API:

- `ref+pulumistateapi://RESOURCE_TYPE/RESOURCE_LOGICAL_NAME/ATTRIBUTE_TYPE/ATTRIBUTE_KEY_PATH?project=PROJECT&stack=STACK`

* `RESOURCE_TYPE` is a Pulumi [resource type](https://www.pulumi.com/docs/concepts/resources/names/#types) of the form `<package>:<module>:<type>`, where forward slashes (`/`) are replaced by a double underscore (`__`) and colons (`:`) are replaced by a single underscore (`_`). For example `aws:s3:Bucket` would be encoded as `aws__s3__Bucket` and `kubernetes:storage.k8s.io/v1:StorageClass` would be encoded as `kubernetes_storage.k8s.io__v1_StorageClass`.
* `RESOURCE_LOGICAL_NAME` is the [logical name](https://www.pulumi.com/docs/concepts/resources/names/#logicalname) of the resource in the Pulumi program.
* `ATTRIBUTE_TYPE` is either `outputs` or `inputs`.
* `ATTRIBUTE_KEY_PATH` is a [GJSON](https://github.com/tidwall/gjson/blob/master/SYNTAX.md) expression that selects the desired attribute from the resource's inputs or outputs per the chosen `ATTRIBUTE_TYPE` value. You must encode any characters that would otherwise not comply with URI syntax, for example `#` becomes `%23`.
* `project` is the Pulumi project name.
* `stack` is the Pulumi stack name.

Environment variables:

- `PULUMI_API_ENDPOINT_URL` is the Pulumi API endpoint URL. Defaults to `https://api.pulumi.com`. You may also provide this as the `pulumi_api_endpoint_url` query parameter.
- `PULUMI_ACCESS_TOKEN` is the Pulumi access token to use for authentication.
- `PULUMI_ORGANIZATION` is the Pulumi organization to use for authentication. You may also provide this as an `organization` query parameter.

Examples:

- `ref+pulumistateapi://aws-native_s3_Bucket/my-bucket/outputs/bucketName?project=my-project&stack=my-stack`
- `ref+pulumistateapi://aws-native_s3_Bucket/my-bucket/outputs/tags.%23(key==SomeKey).value?project=my-project&stack=my-stack`
- `ref+pulumistateapi://kubernetes_storage.k8s.io__v1_StorageClass/gp2-encrypted/inputs/metadata.name?project=my-project&stack=my-stack`

### Kubernetes

Fetch value from Kubernetes:

- `ref+k8s://API_VERSION/KIND/NAMESPACE/NAME/KEY[?kubeConfigPath=<path_to_kubeconfig>&kubeContext=<kubernetes context name>]`

Authentication to the Kubernetes cluster is done by referencing the local kubeconfig file.
The path to the kubeconfig can be specified as a URI parameter, read from the `KUBECONFIG` environment variable or the provider will attempt to read `$HOME/.kube/config`.
The Kubernetes context can be specified as a URI parameteter.

Environment variables:

- `KUBECONFIG` contains the path to the Kubeconfig that will be used to fetch the secret.

Examples:

- `ref+k8s://v1/Secret/mynamespace/mysecret/foo`
- `ref+k8s://v1/ConfigMap/mynamespace/myconfigmap/foo`
- `ref+k8s://v1/Secret/mynamespace/mysecret/bar?kubeConfigPath=/home/user/kubeconfig`
- `secretref+k8s://v1/Secret/mynamespace/mysecret/baz`
- `secretref+k8s://v1/Secret/mynamespace/mysecret/baz?kubeContext=minikube`

> NOTE: This provider only supports kind "Secret" or "ConfigMap" in apiVersion "v1" at this time.

### Conjur

This provider retrieves the value of secrets stored in [Conjur](https://www.conjur.org/).
It's based on the https://github.com/cyberark/conjur-api-go lib.

The following env vars have to be configured:
- `CONJUR_APPLIANCE_URL`
- `CONJUR_ACCOUNT`
- `CONJUR_AUTHN_LOGIN`
- `CONJUR_AUTHN_API_KEY`

- `ref+conjur://PATH/TO/VARIABLE[?address=CONJUR_APPLIANCE_URL&account=CONJUR_ACCOUNT&login=CONJUR_AUTHN_LOGIN&apikey=CONJUR_AUTHN_API_KEY]/CONJUR_SECRET_ID`

Example:

- `ref+conjur://branch/variable_name`

## Advanced Usages

### Discriminating config and secrets

`vals` has an advanced feature that helps you to do GitOps.

`GitOps` is a good practice that helps you to review how your change would affect the production environment.

To best leverage GitOps, it is important to remove dynamic aspects of your config before reviewing.

On the other hand, `vals`'s primary purpose is to defer retrieval of values until the time of deployment, so that we won't accidentally git-commit secrets. The flip-side of this is, obviously, that you can't review the values themselves.

Using `ref+<value uri>` and `secretref+<value uri>` in combination with `vals eval --exclude-secret` helps it.

By using the `secretref+<uri>` notation, you tell `vals` that it is a secret and regular `ref+<uri>` instances are for config values.

```yaml
myconfigvalue: ref+awsssm://myconfig/value
mysecretvalue: secretref+awssecrets://mysecret/value
```

To leverage `GitOps` most by allowing you to review the content of `ref+awsssm://myconfig/value` only, you run `vals eval --exclude-secret` to generate the following:

```yaml
myconfigvalue: MYCONFIG_VALUE
mysecretvalue: secretref+awssecrets://mysecret/value
```

This is safe to be committed into git because, as you've told to `vals`, `awsssm://myconfig/value` is a config value that can be shared publicly.

## Non-Goals

### Complex String-Interpolation / Template Functions

In the early days of this project, the original author has investigated if it was a good idea to introduce string interpolation like feature to vals:

```
foo: xx${{ref "ref+vault://127.0.0.1:8200/mykv/foo?proto=http#/mykey" }}
bar:
  baz: yy${{ref "ref+vault://127.0.0.1:8200/mykv/foo?proto=http#/mykey" }}
```

But the idea had abandoned due to that it seemed to drive the momentum to vals being a full-fledged YAML templating engine. What if some users started wanting to use `vals` for transforming values with functions?
That's not the business of vals.

Instead, use vals solely for composing sets of values that are then input to another templating engine or data manipulation language like Jsonnet and CUE.

Note though, `vals` does have support for simple string interpolation like usage. See [Expression Syntax](#expression-syntax) for more information.

### Merge

Merging YAMLs is out of the scope of `vals`. There're better alternatives like Jsonnet, Sprig, and CUE for the job.
