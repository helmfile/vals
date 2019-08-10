# values

Helm-like configuration "Values" loader with support for various backends including:

- Vault
- AWS SSM Parameter Store
- AWS Secrets Manager
- Merge with Spruce (e.g. Append/Prepend Arrays In Hash)
- Terraform outputs(Coming soon)
- CredHub(Coming soon)

## Usage:

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

Now, `vals` contains a map[string]interface{} representation of the below:

```console
cat <<EOF
foo: $(vault read mykv/foo -o json | jq -r .mykey)
  bar:
    baz: $(vault read mykv/foo -o json | jq -r .mykey)
EOF
```