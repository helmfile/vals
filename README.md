# values

Helm-like configuration "Values" loader with support for various backends including:

- Vault
- AWS SSM Parameter Store
- AWS Secrets Manager
- [SOPS](https://github.com/mozilla/sops)-encrypted files
- Merge with Spruce (e.g. Append/Prepend Arrays In Hash)
- Terraform outputs(Coming soon)
- CredHub(Coming soon)

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
  flatten	Loads a vals template and replaces every instances of custom types to plain $ref's
  ksdecode	Decode YAML document(s) by converting Secret resources' "data" to "stringData" for use with "vals eval"

Use "vals [command] --help" for more infomation about a command
```

`vals -t yaml -e <YAML>` takes any valid YAML and evaluates [JSO Reference](https://json-spec.readthedocs.io/reference.html).

`vals` has its own provider which can be reffered with a URI scheme looks `vals+<TYPE>`.

For this example, use the [Vault](https://www.terraform.io/docs/providers/vault/index.html) provider.
 
Let's start by writing some secret value to `Vault`:

```console
$ vault write mykv/foo mykey=myvalue
```

Now input the template of your YAML and refer to `vals`' Vault provider by using `vals+vault` in the URI scheme:

```console
$ vals eval -e '
foo: {"$ref":"vals+vault://127.0.0.1:8200/mykv/foo?proto=http#/mykey"}
bar:
  baz: {"$ref":"vals+vault://127.0.0.1:8200/mykv/foo?proto=http#/mykey"}
```

Voila! `vals`, replacing every reference to your secret value in Vault, produces the output looks like:

```yaml
foo: FOO
bar:
  baz: FOO
```

Which is equivalent to that of the following shell script:

```bash
VAULT_TOKEN=yourtoken  VAULT_ADDR=http://127.0.0.1:8200/ cat <<EOF
foo: $(vault read mykv/foo -o json | jq -r .mykey)
  bar:
    baz: $(vault read mykv/foo -o json | jq -r .mykey)
EOF
```

An another form of the previous usage is to use `$types` for reducing code repetition.

`$types` allows you to define reusable template of `$ref`s as a named `type` which can then be referred from your template by `$name`:

`x.vals.yaml`:

```yaml
$types:
  v: vals+vault://127.0.0.1:8200/mykv/foo?proto=http

foo: {"$v":"/mykey"}
bar:
  baz: {"$v":"/mykey"}
```

Running `vals eval -f x.vals.yaml` does produce output equivalent to the previous one:

```yaml
foo: FOO
bar:
  baz: FOO
```

Lastly, `vals flatten` can be used to replace all the custom type refs to plain JSON Reference `$refs`:

```console
$ vals flatten -f x.vals.yaml
```

```yaml
foo: {"$ref":"vals+vault://127.0.0.1:8200/mykv/foo?proto=http#/mykey"}
bar:
  baz: {"$ref":"vals+vault://127.0.0.1:8200/mykv/foo?proto=http#/mykey"}
```

There's also a shorter syntax:

```
foo: $ref vals+vault://127.0.0.1:8200/mykv/foo?proto=http#/mykey
bar:
  baz: $ref vals+vault://127.0.0.1:8200/mykv/foo?proto=http#/mykey

```

And with string interpolation:

```
foo: xx${{ref "vals+vault://127.0.0.1:8200/mykv/foo?proto=http#/mykey" }}
bar:
  baz: yy${{ref "vals+vault://127.0.0.1:8200/mykv/foo?proto=http#/mykey" }}
```

### Helm

When you're using a `vals` template as a values file, `helm` usually fail rendering the release manifests as you can't inject YAML objects like `{"$ref":"vals+vault://..."}` to where `string` values are expected(e.g. `data` and `stringData` kvs of `Secret` resources).

To deal with it, use `vals flatten -c` to use the compact format so that JSON references are transformed to vals' own `string` representations, which is safe to be used as values.

```console
$ vals flatten -f x.vals.yaml -c
```

```yaml
foo: "$ref vals+vault://127.0.0.1:8200/mykv/foo?proto=http#/mykey"
bar:
  baz: "$ref vals+vault://127.0.0.1:8200/mykv/foo?proto=http#/mykey"
```

You should use `helm template` and compact-`$ref`s as values to inject references to secrets like:

```console
$ helm template mysql-1.3.2.tgz --set mysqlPassword='$ref vals+vault://127.0.0.1:8200/mykv/foo#/mykey' | vals ksdecode -o yaml -f - | tee manifests.yaml
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
  mysql-password: $ref vals+vault://127.0.0.1:8200/mykv/foo#/mykey
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
import "github.com/mumoshu/values"

config := Map(map[string]interface{}{
    "provider": map[string]interface{}{
        "name":     "vault",
        "type":     "map",
        "path":     "mykv",
        "address":  "http://127.0.0.1:8200",
        "strategy": "raw",
    },
    "inline": map[string]interface{}{
        "foo": "foo",
        "bar": map[string]interface{}{
            "baz": "foo",
        },
    },
})

vals, err := values.Load(config)
if err != nil {
    t.Fatalf("%v", err)
}
```

Now, `vals` contains a `map[string]interface{}` representation of the below:

```console
cat <<EOF
foo: $(vault read mykv/foo -o json | jq -r .mykey)
  bar:
    baz: $(vault read mykv/foo -o json | jq -r .mykey)
EOF
```
