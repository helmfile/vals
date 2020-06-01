# vals

`vals` is a tool for managing configuration values and secrets.

It supports various backends including:

- Vault
- AWS SSM Parameter Store
- AWS Secrets Manager
- GCP Secrets Manager
- [SOPS](https://github.com/mozilla/sops)-encrypted files
- Terraform outputs(Coming soon)
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

`vals` has a collection of providers that each an be referred with a URI scheme looks `vals+<TYPE>`.

For this example, use the [Vault](https://www.terraform.io/docs/providers/vault/index.html) provider.

Let's start by writing some secret value to `Vault`:

```console
$ vault kv put secret/foo mykey=myvalue
```

Now input the template of your YAML and refer to `vals`' Vault provider by using `vals+vault` in the URI scheme:

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
- [GCP Secrets Manager](#gcp-secrets-manager)
- [SOPS](#sops) powered by [sops](https://github.com/mozilla/sops))
- [Terraform (tfstate)](#terraform-tfstate) powered by [tfstate-lookup](https://github.com/fujiwara/tfstate-lookup)
- [Echo](#echo)
- [File](#file)

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

### AWS SSM Parameter Store

- `ref+awsssm://PATH/TO/PARAM[?region=REGION]`
- `ref+awsssm://PREFIX/TO/PARAMS[?region=REGION]#/PATH/TO/PARAM`

In the latter case, `vals` uses `GetParametersByPath(/PREFIX/TO/PARAMS)` caches the result per prefix rather than each single path to reduce number of API calls

Examples:

- `ref+awsssm://myteam/mykey`
- `ref+awsssm://myteam/mydoc#/foo/bar`
- `ref+awsssm://myteam/mykey?region=us-west-2`

### AWS Secrets Manager

- `ref+awssec://PATH/TO/SECRET[?region=REGION&version_stage=STAGE&version_id=ID]`
- `ref+awssec://PATH/TO/SECRET[?region=REGION&version_stage=STAGE&version_id=ID]#/yaml_or_json_key/in/secret`

Examples:

- `ref+awssec://myteam/mykey`
- `ref+awssec://myteam/mydoc#/foo/bar`
- `ref+awssec://myteam/mykey?region=us-west-2`

### GCP Secrets Manager

- `ref+gcpsecrets://PROJECT/SECRET[?version=VERSION]`
- `ref+gcpsecrets://PROJECT/SECRET[?version=VERSION]#/yaml_or_json_key/in/secret`

Examples:

- `ref+gcpsecrets://myproject/mysecret`
- `ref+gcpsecrets://myproject/mysecret?version=3`
- `ref+gcpsecrets://myproject/mysecret?version=3#/yaml_or_json_key/in/secret`

### Terraform (tfstate)

- `ref+tfstate://path/to/some.tfstate/RESOURCE_NAME`

Examples:

- `ref+tfstate://path/to/some.tfstate/aws_vpc.main.id`
- `ref+tfstate://path/to/some.tfstate/module.mymodule.aws_vpc.main.id`
- `ref+tfstate://path/to/some.tfstate/data.thetype.name.foo.bar`

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

- `ref+file://path/to/file[#/path/to/the/value]`

Examples:

- `ref+file://foo/bar` loads the file at `foo/bar`
- `ref+file://some.yaml#/foo/bar` loads the YAML file at `some.yaml` and reads the value for the path `$.foo.bar`.
  Let's say `some.yaml` contains `{"foo":{"bar":"BAR"}}`, `key1: ref+file://some.yaml#/foo/bar` results in `key1: BAR`.

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
mysecretvalue: secretref+awssec://mysecret/value
```

To leverage `GitOps` most by allowing you to review the content of `ref+awsssm://myconfig/value` only, you run `vals eval --exclude-secretref` to generate the following:

```yaml
myconfigvalue: MYCONFIG_VALUE
mysecretvalue: secretref+awssec://mysecret/value
```

This is safe to be committed into git because, as you've told to `vals`, `awsssm://myconfig/value` is a config value that can be shared publicly.

## Non-Goals

### String-Interpolation / Template Functions

In the early days of this project, the original author has investigated if it was a good idea to introduce string interpolation like feature to vals:

```
foo: xx${{ref "vals+vault://127.0.0.1:8200/mykv/foo?proto=http#/mykey" }}
bar:
  baz: yy${{ref "vals+vault://127.0.0.1:8200/mykv/foo?proto=http#/mykey" }}
```

But the idea had abandoned due to that it seemed to drive the momentum to vals being a full-fledged YAML templating engine. What if some users started wanting to use `vals` for transforming values with functions?
That's not the business of vals.

Instead, use vals solely for composing sets of values that are then input to another templating engine or data manipulation language like Jsonnet and CUE.

### Merge

Merging YAMLs is out of the scope of `vals`. There're better alternatives like Jsonnet, Sprig, and CUE for the job.
