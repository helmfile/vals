package vals

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/vault/api"

	config2 "github.com/helmfile/vals/pkg/config"
)

type Conn struct {
	Client *api.Client
	Token  string
}

func StartVault(t *testing.T, mountPath, mountInputType string) (Conn, func()) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())

	port := 8200
	devRootTokenID := "root"
	vaultAddr := fmt.Sprintf("127.0.0.1:%v", port)
	vault := exec.CommandContext(ctx, "vault", "server",
		"-dev",
		"-dev-root-token-id="+devRootTokenID,
		fmt.Sprintf("-dev-listen-address=%s", vaultAddr),
	)
	vault.Stdout = os.Stdout
	vault.Stderr = os.Stderr

	errs := make(chan error, 1)

	go func() {
		errs <- vault.Run()
	}()

	config := &api.Config{
		Address: "http://" + vaultAddr,
	}
	client, err := api.NewClient(config)
	if err != nil {
		t.Fatalf("Failed creating vault client: %v", err)
	}

	client.SetToken(devRootTokenID)

	client.Sys().Mount(mountPath, &api.MountInput{
		Type: mountInputType,
	})

	return Conn{Client: client, Token: devRootTokenID}, func() {
		cancel()

		if err := <-errs; err != nil {
			t.Logf("stopping vault: %v", err)
		}
	}
}

func SetupVaultKV(t *testing.T, writes map[string]map[string]interface{}) (string, func()) {
	// TODO v2 api support where mountInputType should be "kv-v2" rather than "kv"
	conn, stop := StartVault(t, "mykv", "kv")

	client := conn.Client
	addr := conn.Client.Address()
	for path, data := range writes {
		sec, err := client.Logical().Write(path, data)
		if err != nil {
			t.Logf("%v", sec)
			t.Fatalf("%v", err)
		}
	}
	// TODO Mock os.Getenv so that this won't result in data race when multiple tests are run concurrently
	os.Setenv("VAULT_TOKEN", conn.Token)

	return addr, stop
}

func TestValues_Vault_EvalTemplate(t *testing.T) {
	// Pre-requisite:
	//   vault secrets enable -path=mykv kv
	//   vault write mykv/foo mykey=myvalue
	//   vault read mykv/foo
	if os.Getenv("SKIP_TESTS") != "" {
		t.Skip("Skipping tests")
	}

	addr, stop := SetupVaultKV(
		t,
		map[string]map[string]interface{}{
			"mykv/foo": {
				"mykey": "myvalue",
			},
			"mykv/objs": {
				"myyaml": `yamlkey1: yamlval1
`,
				"myjson": `{"jsonkey1":"jsonval1"}
`,
			},
		},
	)
	defer stop()

	type testcase struct {
		config   map[string]interface{}
		expected map[string]interface{}
	}

	testcases := []testcase{
		{
			config: map[string]interface{}{
				"foo": fmt.Sprintf("ref+vault://mykv/foo?address=%s#/mykey", addr),
				"bar": map[string]interface{}{
					"baz": fmt.Sprintf("ref+vault://mykv/foo?address=%s#/mykey", addr),
				},
			},
			expected: map[string]interface{}{
				"foo": "myvalue",
				"bar": map[string]interface{}{
					"baz": "myvalue",
				},
			},
		},
		{
			config: map[string]interface{}{
				"foo": "FOO",
				fmt.Sprintf("ref+vault://mykv/objs?address=%s#/myyaml", addr): map[string]interface{}{},
				fmt.Sprintf("ref+vault://mykv/objs?address=%s#/myjson", addr): map[string]interface{}{},
			},
			expected: map[string]interface{}{
				"foo":      "FOO",
				"yamlkey1": "yamlval1",
				"jsonkey1": "jsonval1",
			},
		},
		{
			config: map[string]interface{}{
				"foo": "FOO",
				// See https://github.com/roboll/helmfile/issues/990#issuecomment-557753645
				fmt.Sprintf("ref+vault://mykv/objs?address=%s#/myyaml", addr): map[interface{}]interface{}{},
				fmt.Sprintf("ref+vault://mykv/objs?address=%s#/myjson", addr): map[interface{}]interface{}{},
			},
			expected: map[string]interface{}{
				"foo":      "FOO",
				"yamlkey1": "yamlval1",
				"jsonkey1": "jsonval1",
			},
		},
	}

	for i := range testcases {
		tc := testcases[i]
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			vals, err := Eval(tc.config)
			if err != nil {
				t.Fatalf("%v", err)
			}

			diff := cmp.Diff(tc.expected, vals)
			if diff != "" {
				t.Errorf("unxpected diff: %s", diff)
			}
		})
	}
}

func TestValues_Vault_String(t *testing.T) {
	// TODO
	// Pre-requisite: vault write mykv/foo mykey=myvalue
	if os.Getenv("SKIP_TESTS") != "" {
		t.Skip("Skipping tests")
	}

	type testcase struct {
		config map[string]interface{}
	}
	commonInline := map[string]interface{}{
		"foo": "mykey",
		"bar": map[string]interface{}{
			"baz": "mykey",
		},
	}

	testcases := []testcase{
		{
			config: map[string]interface{}{
				"provider": map[string]interface{}{
					"name":    "vault",
					"type":    "string",
					"path":    "mykv/foo",
					"address": "http://127.0.0.1:8200",
				},
				"inline": commonInline,
			},
		},
		{
			config: map[string]interface{}{
				"provider": map[string]interface{}{
					"name": "vault",
					// implies type=string
					"path":    "mykv/foo",
					"address": "http://127.0.0.1:8200",
				},
				"inline": commonInline,
			},
		},
		{
			config: map[string]interface{}{
				// implies name=vault and type=string
				"vault": map[string]interface{}{
					"path":    "mykv/foo",
					"address": "http://127.0.0.1:8200",
				},
				"inline": commonInline,
			},
		},
	}

	for i := range testcases {
		tc := testcases[i]
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			config := config2.Map(tc.config)

			vals, err := Load(config)
			if err != nil {
				t.Fatalf("%v", err)
			}

			{
				expected := "myvalue"
				key := "foo"
				actual := vals[key]
				if actual != expected {
					t.Errorf("unepected value for key %q: expected=%q, got=%q", key, expected, actual)
				}
			}

			{
				switch bar := vals["bar"].(type) {
				case map[string]interface{}:
					expected := "myvalue"
					key := "baz"
					actual := bar[key]
					if actual != expected {
						t.Errorf("unepected value for key %q: expected=%q, got=%q", key, expected, actual)
					}
				default:
					t.Fatalf("unexpected type of bar: value=%v, type=%T", bar, bar)
				}
			}
		})
	}
}

func TestValues_Vault_Map(t *testing.T) {
	// TODO
	// Pre-requisite: vault write mykv/foo mykey=myvalue
	if os.Getenv("SKIP_TESTS") != "" {
		t.Skip("Skipping tests")
	}

	type testcase struct {
		name   string
		config map[string]interface{}
	}
	testcases := []testcase{
		{
			config: map[string]interface{}{
				"inline": map[string]interface{}{
					"foo": "foo",
					"bar": map[string]interface{}{
						"baz": "foo",
					},
				},
				"provider": map[string]interface{}{
					"name":    "vault",
					"type":    "map",
					"path":    "mykv",
					"address": "http://127.0.0.1:8200",
				},
			},
		},
		{
			config: map[string]interface{}{
				"inline": map[string]interface{}{
					"foo": "foo",
					"bar": map[string]interface{}{
						"baz": "foo",
					},
				},
				"provider": map[string]interface{}{
					"name":    "vault",
					"type":    "map",
					"format":  "raw",
					"path":    "mykv",
					"address": "http://127.0.0.1:8200",
				},
			},
		},
		{
			config: map[string]interface{}{
				"inline": map[string]interface{}{
					"foo": "foo",
					"bar": map[string]interface{}{
						"baz": "foo",
					},
				},
				"provider": map[string]interface{}{
					"name": "vault",
					// implies type:map format:raw
					"prefix":  "mykv",
					"address": "http://127.0.0.1:8200",
				},
			},
		},
		{
			name: "setForKey1",
			config: map[string]interface{}{
				"vault": map[string]interface{}{
					// implies type:map format:raw
					"prefix":     "mykv/foo",
					"address":    "http://127.0.0.1:8200",
					"setForKeys": []string{"foo", "bar.baz"},
				},
			},
		},
		{
			name: "setForKey2",
			config: map[string]interface{}{
				"vault": map[string]interface{}{
					// implies type:map format:raw
					"paths":      []string{"mykv/foo/mykey"},
					"address":    "http://127.0.0.1:8200",
					"setForKeys": []string{"foo", "bar.baz"},
				},
			},
		},
		{
			name: "setForKey3",
			config: map[string]interface{}{
				"vault": map[string]interface{}{
					// implies type:map format:raw
					"prefix":     "mykv/foo/",
					"keys":       []string{"mykey"},
					"address":    "http://127.0.0.1:8200",
					"setForKeys": []string{"foo", "bar.baz"},
				},
			},
		},
		{
			name: "set1",
			config: map[string]interface{}{
				"vault": map[string]interface{}{
					// implies type:map format:raw
					"prefix":  "mykv",
					"address": "http://127.0.0.1:8200",
					"set": map[string]interface{}{
						"foo": "foo",
						"bar": map[string]interface{}{
							"baz": "foo",
						},
					},
				},
			},
		},
		{
			name: "set2",
			config: map[string]interface{}{
				"vault": map[string]interface{}{
					// implies type:map format:raw
					"prefix":  "mykv",
					"address": "http://127.0.0.1:8200",
					"set": map[string]interface{}{
						"foo": "foo",
						"bar": map[string]interface{}{
							"baz": "foo",
						},
					},
				},
			},
		},
	}

	for i := range testcases {
		tc := testcases[i]
		tcname := fmt.Sprintf("%d", i)
		if tc.name != "" {
			tcname = tc.name
		}
		t.Run(tcname, func(t *testing.T) {
			config := config2.Map(tc.config)

			vals, err := Load(config)
			if err != nil {
				t.Fatalf("%v", err)
			}

			{
				switch foo := vals["foo"].(type) {
				case map[string]interface{}:
					key := "mykey"
					actual, ok := foo[key]
					if !ok {
						t.Fatalf("%q does not exist", key)
					}
					expected := "myvalue"
					if actual != expected {
						t.Errorf("unepected value for key %q: expected=%q, got=%q", key, expected, actual)
					}
				default:
					t.Fatalf("unexpected type of foo: value=%v, type=%T", foo, foo)
				}
			}

			{
				switch bar := vals["bar"].(type) {
				case map[string]interface{}:
					switch baz := bar["baz"].(type) {
					case map[string]interface{}:
						key := "mykey"
						actual, ok := baz[key]
						if !ok {
							t.Fatalf("%q does not exist", key)
						}
						expected := "myvalue"
						if actual != expected {
							t.Errorf("unepected value for key %q: expected=%q, got=%q", key, expected, actual)
						}
					default:
						t.Fatalf("unexpected type of baz: value=%v, type=%T", baz, baz)
					}
				default:
					t.Fatalf("unexpected type of bar: value=%v, type=%T", bar, bar)
				}
			}
		})
	}
}

func TestValues_Vault_Map_Raw(t *testing.T) {
	// TODO
	// Pre-requisite: vault write mykv/foo mykey=myvalue
	if os.Getenv("SKIP_TESTS") != "" {
		t.Skip("Skipping tests")
	}

	type testcase struct {
		provider map[string]interface{}
	}
	testcases := []testcase{
		{
			provider: map[string]interface{}{
				"name":    "vault",
				"type":    "map",
				"path":    "mykv",
				"address": "http://127.0.0.1:8200",
				"format":  "raw",
			},
		},
		{
			provider: map[string]interface{}{
				"name": "vault",
				// implies
				//"type":    "map",
				//"format":  "raw",
				"prefix":  "mykv",
				"address": "http://127.0.0.1:8200",
			},
		},
	}

	for i := range testcases {
		tc := testcases[i]

		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			config := config2.Map(map[string]interface{}{
				"provider": tc.provider,
				"inline": map[string]interface{}{
					"foo": "foo",
					"bar": map[string]interface{}{
						"baz": "foo",
					},
				},
			})

			vals, err := Load(config)
			if err != nil {
				t.Fatalf("%v", err)
			}

			{
				switch foo := vals["foo"].(type) {
				case map[string]interface{}:
					key := "mykey"
					actual, ok := foo[key]
					if !ok {
						t.Fatalf("%q does not exist", key)
					}
					expected := "myvalue"
					if actual != expected {
						t.Errorf("unepected value for key %q: expected=%q, got=%q", key, expected, actual)
					}
				default:
					t.Fatalf("unexpected type of foo: value=%v, type=%T", foo, foo)
				}
			}

			{
				switch bar := vals["bar"].(type) {
				case map[string]interface{}:
					switch baz := bar["baz"].(type) {
					case map[string]interface{}:
						key := "mykey"
						actual, ok := baz[key]
						if !ok {
							t.Fatalf("%q does not exist", key)
						}
						expected := "myvalue"
						if actual != expected {
							t.Errorf("unepected value for key %q: expected=%q, got=%q", key, expected, actual)
						}
					default:
						t.Fatalf("unexpected type of baz: value=%v, type=%T", baz, baz)
					}
				default:
					t.Fatalf("unexpected type of bar: value=%v, type=%T", bar, bar)
				}
			}
		})
	}
}

func TestValues_Vault_Map_YAML(t *testing.T) {
	// TODO
	// cat <<EOF > myyaml.yaml
	// baz:
	//   mykey: myvalue
	// EOF
	//
	// cat <<EOF > myjson.json
	// {"baz": {"mykey": "myvalue"}}
	// EOF
	//
	// vault write mykv/yamltest myyaml="$(cat myyaml.yaml)" myjson="$(cat myjson.json)"
	if os.Getenv("SKIP_TESTS") != "" {
		t.Skip("Skipping tests")
	}

	yamlContent, err := os.ReadFile("myyaml.yaml")
	if err != nil {
		t.Fatalf("%v", err)
	}

	jsonContent, err := os.ReadFile("myjson.json")
	if err != nil {
		t.Fatalf("%v", err)
	}

	addr, stop := SetupVaultKV(
		t,
		map[string]map[string]interface{}{
			"mykv/yamltest": {
				"myyaml": string(yamlContent),
				"myjson": string(jsonContent),
			},
		},
	)
	defer stop()

	type testcase struct {
		provider map[string]interface{}
		dataKey  string
	}
	provider1 := map[string]interface{}{
		"name":    "vault",
		"type":    "map",
		"path":    "mykv/yamltest",
		"address": addr,
		"format":  "yaml",
	}

	provider2 := map[string]interface{}{
		"name": "vault",
		// implies `type: map`
		"path":    "mykv/yamltest",
		"address": addr,
		"format":  "yaml",
	}

	testcases := []testcase{
		{
			provider: provider1,
			dataKey:  "myyaml",
		},
		{
			provider: provider1,
			dataKey:  "myjson",
		},
		{
			provider: provider2,
			dataKey:  "myyaml",
		},
		{
			provider: provider2,
			dataKey:  "myjson",
		},
	}

	for i := range testcases {
		tc := testcases[i]
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			config := config2.Map(map[string]interface{}{
				"provider": tc.provider,
				"inline": map[string]interface{}{
					"bar": tc.dataKey,
				},
			})

			vals, err := Load(config)
			if err != nil {
				t.Fatalf("%v", err)
			}

			{
				switch bar := vals["bar"].(type) {
				case map[string]interface{}:
					switch baz := bar["baz"].(type) {
					case map[string]interface{}:
						key := "mykey"
						actual, ok := baz[key]
						if !ok {
							t.Fatalf("%q does not exist", key)
						}
						expected := "myvalue"
						if actual != expected {
							t.Errorf("unepected value for key %q: expected=%q, got=%q", key, expected, actual)
						}
					default:
						t.Fatalf("unexpected type of baz: value=%v, type=%T", baz, baz)
					}
				default:
					t.Fatalf("unexpected type of bar: value=%v, type=%T", bar, bar)
				}
			}
		})
	}
}

func TestValues_Vault_Map_YAML_Root(t *testing.T) {
	// TODO
	// cat <<EOF > myyaml.yaml
	// baz:
	//   mykey: myvalue
	// EOF
	//
	// cat <<EOF > myjson.json
	// {"baz": {"mykey": "myvalue"}}
	// EOF
	//
	// vault write mykv/yamltest myyaml="$(cat myyaml.yaml)" myjson="$(cat myjson.json)"
	if os.Getenv("SKIP_TESTS") != "" {
		t.Skip("Skipping tests")
	}

	type provider struct {
		config map[string]interface{}
	}
	testcases := []provider{
		{
			config: map[string]interface{}{
				"provider": map[string]interface{}{
					"name":    "vault",
					"type":    "map",
					"path":    "mykv/yamltest/myyaml",
					"address": "http://127.0.0.1:8200",
					"format":  "yaml",
				},
			},
		},
		{
			config: map[string]interface{}{
				"provider": map[string]interface{}{
					"name":    "vault",
					"type":    "map",
					"path":    "mykv/yamltest/myjson",
					"address": "http://127.0.0.1:8200",
					"format":  "yaml",
				},
			},
		},
		{
			config: map[string]interface{}{
				"provider": map[string]interface{}{
					"name": "vault",
					// implies format:yaml and type:map
					"path":    "mykv/yamltest/myyaml",
					"address": "http://127.0.0.1:8200",
				},
			},
		},
		{
			config: map[string]interface{}{
				"provider": map[string]interface{}{
					"name": "vault",
					// implies format:yaml and type:map
					"path":    "mykv/yamltest/myjson",
					"address": "http://127.0.0.1:8200",
				},
			},
		},
		{
			config: map[string]interface{}{
				// implies name:vault
				"vault": map[string]interface{}{
					// implies format:yaml and type:map
					"path":    "mykv/yamltest/myjson",
					"address": "http://127.0.0.1:8200",
				},
			},
		},
	}

	for i := range testcases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			tc := testcases[i]
			config := config2.Map(tc.config)

			vals, err := Load(config)
			if err != nil {
				t.Fatalf("%v", err)
			}

			{
				switch baz := vals["baz"].(type) {
				case map[string]interface{}:
					key := "mykey"
					actual, ok := baz[key]
					if !ok {
						t.Fatalf("%q does not exist", key)
					}
					expected := "myvalue"
					if actual != expected {
						t.Errorf("unepected value for key %q: expected=%q, got=%q", key, expected, actual)
					}
				default:
					t.Fatalf("unexpected type of baz: value=%v, type=%T", baz, baz)
				}
			}
		})
	}
}

func TestValues_Vault_Map_Raw_Root(t *testing.T) {
	// TODO
	// cat <<EOF > myyaml.yaml
	// baz:
	//   mykey: myvalue
	// EOF
	//
	// cat <<EOF > myjson.json
	// {"baz": {"mykey": "myvalue"}}
	// EOF
	//
	// vault write mykv/yamltest myyaml="$(cat myyaml.yaml)" myjson="$(cat myjson.json)"
	if os.Getenv("SKIP_TESTS") != "" {
		t.Skip("Skipping tests")
	}

	type testcase struct {
		config map[string]interface{}
	}
	provider1 := map[string]interface{}{
		"name":    "vault",
		"type":    "map",
		"path":    "mykv/foo",
		"address": "http://127.0.0.1:8200",
		"format":  "raw",
	}

	provider2 := map[string]interface{}{
		"name": "vault",
		// implies format:raw
		"prefix":  "mykv/foo",
		"address": "http://127.0.0.1:8200",
	}

	testcases := []testcase{
		{
			config: map[string]interface{}{
				"provider": provider1,
			},
		},
		{
			config: map[string]interface{}{
				"provider": provider2,
			},
		},
		{
			config: map[string]interface{}{
				// implies name:vault
				"vault": map[string]interface{}{
					// implies format:raw
					"prefix":  "mykv/foo",
					"address": "http://127.0.0.1:8200",
				},
			},
		},
		{
			config: map[string]interface{}{
				// implies name:ssm
				"vault": map[string]interface{}{
					// implies format:raw
					"prefix":  "/mykv/foo",
					"keys":    []string{"mykey"},
					"address": "http://127.0.0.1:8200",
				},
			},
		},
		{
			config: map[string]interface{}{
				// implies name:ssm
				"vault": map[string]interface{}{
					// implies format:raw
					"paths":   []string{"/mykv/foo/mykey"},
					"address": "http://127.0.0.1:8200",
				},
			},
		},
	}

	for i := range testcases {
		tc := testcases[i]

		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			config := config2.Map(tc.config)

			vals, err := Load(config)
			if err != nil {
				t.Fatalf("%v", err)
			}

			{
				key := "mykey"
				actual, ok := vals[key]
				if !ok {
					t.Fatalf("%q does not exist", key)
				}
				expected := "myvalue"
				if actual != expected {
					t.Errorf("unepected value for key %q: expected=%q, got=%q", key, expected, actual)
				}
			}
		})
	}
}
