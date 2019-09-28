# values

Helm-like configuration "Values" loader with support for various backends including:

- Vault
- AWS SSM Parameter Store
- AWS Secrets Manager
- [SOPS](https://github.com/mozilla/sops)-encrypted files
- Merge with Spruce (e.g. Append/Prepend Arrays In Hash)
- Terraform outputs(Coming soon)
- CredHub(Coming soon)

## Usage:

# CLI

`vals -t yaml -e <YAML>` takes any valid YAML and evaluates [JSO Reference](https://json-spec.readthedocs.io/reference.html).

`vals` has its own provider which can be reffered with a URI scheme looks `vals+<TYPE>`.

For this example, use the [Vault](https://www.terraform.io/docs/providers/vault/index.html) provider.
 
Let's start by writing some secret value to `Vault`:

```console
$ vault write mykv/foo mykey=myvalue
```

Now input the template of your YAML and refer to `vals`' Vault provider by using `vals+vault` in the URI scheme:

```
$ vals -t yaml -e '
foo: {"$ref":"vals+vault://127.0.0.1:8200/mykv/foo?proto=http#/mykey"
bar:
  baz: {"$ref":"vals+vault://127.0.0.1:8200/mykv/foo?proto=http#/myket"
```

Voila! `vals`, replacing every reference to your secret value in Vault, produces the output looks like:

```
foo: FOO
bar:
  baz: FOO
```

Which is equivalent to that of the following shell script:

```
VAULT_TOKEN=yourtoken  VAULT_ADDR=http://127.0.0.1:8200/ cat <<EOF
foo: $(vault read mykv/foo -o json | jq -r .mykey)
  bar:
    baz: $(vault read mykv/foo -o json | jq -r .mykey)
EOF
```

### Go

```go
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

vals, err := New(config)
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
