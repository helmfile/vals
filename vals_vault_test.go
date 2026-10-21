package vals

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
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

func SetupVaultKV(t *testing.T, writes map[string]map[string]any) (string, func()) {
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
		map[string]map[string]any{
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
		config   map[string]any
		expected map[string]any
	}

	testcases := []testcase{
		{
			config: map[string]any{
				"foo": fmt.Sprintf("ref+vault://mykv/foo?address=%s#/mykey", addr),
				"bar": map[string]any{
					"baz": fmt.Sprintf("ref+vault://mykv/foo?address=%s#/mykey", addr),
				},
			},
			expected: map[string]any{
				"foo": "myvalue",
				"bar": map[string]any{
					"baz": "myvalue",
				},
			},
		},
		{
			config: map[string]any{
				"foo": "FOO",
				fmt.Sprintf("ref+vault://mykv/objs?address=%s#/myyaml", addr): map[string]any{},
				fmt.Sprintf("ref+vault://mykv/objs?address=%s#/myjson", addr): map[string]any{},
			},
			expected: map[string]any{
				"foo":      "FOO",
				"yamlkey1": "yamlval1",
				"jsonkey1": "jsonval1",
			},
		},
		{
			config: map[string]any{
				"foo": "FOO",
				// See https://github.com/roboll/helmfile/issues/990#issuecomment-557753645
				fmt.Sprintf("ref+vault://mykv/objs?address=%s#/myyaml", addr): map[any]any{},
				fmt.Sprintf("ref+vault://mykv/objs?address=%s#/myjson", addr): map[any]any{},
			},
			expected: map[string]any{
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

func TestValues_Vault_Decode_Base64(t *testing.T) {
	if os.Getenv("SKIP_TESTS") != "" {
		t.Skip("Skipping tests")
	}

	plain := "hello-binary-payload"
	encoded := base64.StdEncoding.EncodeToString([]byte(plain))

	addr, stop := SetupVaultKV(
		t,
		map[string]map[string]any{
			"mykv/bin": {
				"cert":  encoded,
				"plain": "not-base64-value",
			},
		},
	)
	defer stop()

	t.Run("decode=base64 decodes targeted key", func(t *testing.T) {
		config := map[string]any{
			"cert": fmt.Sprintf("ref+vault://mykv/bin?address=%s&decode=base64#/cert", addr),
		}
		got, err := Eval(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got["cert"] != plain {
			t.Errorf("expected decoded value %q, got %q", plain, got["cert"])
		}
	})

	t.Run("decode=base64 on non-base64 value returns error", func(t *testing.T) {
		config := map[string]any{
			"bad": fmt.Sprintf("ref+vault://mykv/bin?address=%s&decode=base64#/plain", addr),
		}
		_, err := Eval(config)
		if err == nil {
			t.Fatalf("expected base64 decode error, got nil")
		}
		if !strings.Contains(err.Error(), "base64 decode failed") {
			t.Errorf("expected base64 decode failure in error, got: %v", err)
		}
	})

	t.Run("mixed secret with #/field does not break sibling non-base64 keys", func(t *testing.T) {
		config := map[string]any{
			"cert": fmt.Sprintf("ref+vault://mykv/bin?address=%s&decode=base64#/cert", addr),
		}
		got, err := Eval(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got["cert"] != plain {
			t.Errorf("expected decoded value %q, got %q", plain, got["cert"])
		}
	})

	t.Run("no decode parameter returns raw value", func(t *testing.T) {
		config := map[string]any{
			"raw": fmt.Sprintf("ref+vault://mykv/bin?address=%s#/cert", addr),
		}
		got, err := Eval(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got["raw"] != encoded {
			t.Errorf("expected raw value %q, got %q", encoded, got["raw"])
		}
	})
}

func TestValues_Vault_String(t *testing.T) {
	// TODO
	// Pre-requisite: vault write mykv/foo mykey=myvalue
	if os.Getenv("SKIP_TESTS") != "" {
		t.Skip("Skipping tests")
	}

	type testcase struct {
		config map[string]any
	}
	commonInline := map[string]any{
		"foo": "mykey",
		"bar": map[string]any{
			"baz": "mykey",
		},
	}

	testcases := []testcase{
		{
			config: map[string]any{
				"provider": map[string]any{
					"name":    "vault",
					"type":    "string",
					"path":    "mykv/foo",
					"address": "http://127.0.0.1:8200",
				},
				"inline": commonInline,
			},
		},
		{
			config: map[string]any{
				"provider": map[string]any{
					"name": "vault",
					// implies type=string
					"path":    "mykv/foo",
					"address": "http://127.0.0.1:8200",
				},
				"inline": commonInline,
			},
		},
		{
			config: map[string]any{
				// implies name=vault and type=string
				"vault": map[string]any{
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
				expected := testValue
				key := "foo"
				actual := vals[key]
				if actual != expected {
					t.Errorf("unexpected value for key %q: expected=%q, got=%q", key, expected, actual)
				}
			}

			{
				switch bar := vals["bar"].(type) {
				case map[string]any:
					expected := testValue
					key := "baz"
					actual := bar[key]
					if actual != expected {
						t.Errorf("unexpected value for key %q: expected=%q, got=%q", key, expected, actual)
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
		config map[string]any
		name   string
	}
	testcases := []testcase{
		{
			config: map[string]any{
				"inline": map[string]any{
					"foo": "foo",
					"bar": map[string]any{
						"baz": "foo",
					},
				},
				"provider": map[string]any{
					"name":    "vault",
					"type":    "map",
					"path":    "mykv",
					"address": "http://127.0.0.1:8200",
				},
			},
		},
		{
			config: map[string]any{
				"inline": map[string]any{
					"foo": "foo",
					"bar": map[string]any{
						"baz": "foo",
					},
				},
				"provider": map[string]any{
					"name":    "vault",
					"type":    "map",
					"format":  "raw",
					"path":    "mykv",
					"address": "http://127.0.0.1:8200",
				},
			},
		},
		{
			config: map[string]any{
				"inline": map[string]any{
					"foo": "foo",
					"bar": map[string]any{
						"baz": "foo",
					},
				},
				"provider": map[string]any{
					"name": "vault",
					// implies type:map format:raw
					"prefix":  "mykv",
					"address": "http://127.0.0.1:8200",
				},
			},
		},
		{
			name: "setForKey1",
			config: map[string]any{
				"vault": map[string]any{
					// implies type:map format:raw
					"prefix":     "mykv/foo",
					"address":    "http://127.0.0.1:8200",
					"setForKeys": []string{"foo", "bar.baz"},
				},
			},
		},
		{
			name: "setForKey2",
			config: map[string]any{
				"vault": map[string]any{
					// implies type:map format:raw
					"paths":      []string{"mykv/foo/mykey"},
					"address":    "http://127.0.0.1:8200",
					"setForKeys": []string{"foo", "bar.baz"},
				},
			},
		},
		{
			name: "setForKey3",
			config: map[string]any{
				"vault": map[string]any{
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
			config: map[string]any{
				"vault": map[string]any{
					// implies type:map format:raw
					"prefix":  "mykv",
					"address": "http://127.0.0.1:8200",
					"set": map[string]any{
						"foo": "foo",
						"bar": map[string]any{
							"baz": "foo",
						},
					},
				},
			},
		},
		{
			name: "set2",
			config: map[string]any{
				"vault": map[string]any{
					// implies type:map format:raw
					"prefix":  "mykv",
					"address": "http://127.0.0.1:8200",
					"set": map[string]any{
						"foo": "foo",
						"bar": map[string]any{
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
				case map[string]any:
					key := "mykey"
					actual, ok := foo[key]
					if !ok {
						t.Fatalf("%q does not exist", key)
					}
					expected := testValue
					if actual != expected {
						t.Errorf("unexpected value for key %q: expected=%q, got=%q", key, expected, actual)
					}
				default:
					t.Fatalf("unexpected type of foo: value=%v, type=%T", foo, foo)
				}
			}

			{
				switch bar := vals["bar"].(type) {
				case map[string]any:
					switch baz := bar["baz"].(type) {
					case map[string]any:
						key := "mykey"
						actual, ok := baz[key]
						if !ok {
							t.Fatalf("%q does not exist", key)
						}
						expected := testValue
						if actual != expected {
							t.Errorf("unexpected value for key %q: expected=%q, got=%q", key, expected, actual)
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
		provider map[string]any
	}
	testcases := []testcase{
		{
			provider: map[string]any{
				"name":    "vault",
				"type":    "map",
				"path":    "mykv",
				"address": "http://127.0.0.1:8200",
				"format":  "raw",
			},
		},
		{
			provider: map[string]any{
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
			config := config2.Map(map[string]any{
				"provider": tc.provider,
				"inline": map[string]any{
					"foo": "foo",
					"bar": map[string]any{
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
				case map[string]any:
					key := "mykey"
					actual, ok := foo[key]
					if !ok {
						t.Fatalf("%q does not exist", key)
					}
					expected := testValue
					if actual != expected {
						t.Errorf("unexpected value for key %q: expected=%q, got=%q", key, expected, actual)
					}
				default:
					t.Fatalf("unexpected type of foo: value=%v, type=%T", foo, foo)
				}
			}

			{
				switch bar := vals["bar"].(type) {
				case map[string]any:
					switch baz := bar["baz"].(type) {
					case map[string]any:
						key := "mykey"
						actual, ok := baz[key]
						if !ok {
							t.Fatalf("%q does not exist", key)
						}
						expected := testValue
						if actual != expected {
							t.Errorf("unexpected value for key %q: expected=%q, got=%q", key, expected, actual)
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
		map[string]map[string]any{
			"mykv/yamltest": {
				"myyaml": string(yamlContent),
				"myjson": string(jsonContent),
			},
		},
	)
	defer stop()

	type testcase struct {
		provider map[string]any
		dataKey  string
	}
	provider1 := map[string]any{
		"name":    "vault",
		"type":    "map",
		"path":    "mykv/yamltest",
		"address": addr,
		"format":  "yaml",
	}

	provider2 := map[string]any{
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
			config := config2.Map(map[string]any{
				"provider": tc.provider,
				"inline": map[string]any{
					"bar": tc.dataKey,
				},
			})

			vals, err := Load(config)
			if err != nil {
				t.Fatalf("%v", err)
			}

			{
				switch bar := vals["bar"].(type) {
				case map[string]any:
					switch baz := bar["baz"].(type) {
					case map[string]any:
						key := "mykey"
						actual, ok := baz[key]
						if !ok {
							t.Fatalf("%q does not exist", key)
						}
						expected := testValue
						if actual != expected {
							t.Errorf("unexpected value for key %q: expected=%q, got=%q", key, expected, actual)
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
		config map[string]any
	}
	testcases := []provider{
		{
			config: map[string]any{
				"provider": map[string]any{
					"name":    "vault",
					"type":    "map",
					"path":    "mykv/yamltest/myyaml",
					"address": "http://127.0.0.1:8200",
					"format":  "yaml",
				},
			},
		},
		{
			config: map[string]any{
				"provider": map[string]any{
					"name":    "vault",
					"type":    "map",
					"path":    "mykv/yamltest/myjson",
					"address": "http://127.0.0.1:8200",
					"format":  "yaml",
				},
			},
		},
		{
			config: map[string]any{
				"provider": map[string]any{
					"name": "vault",
					// implies format:yaml and type:map
					"path":    "mykv/yamltest/myyaml",
					"address": "http://127.0.0.1:8200",
				},
			},
		},
		{
			config: map[string]any{
				"provider": map[string]any{
					"name": "vault",
					// implies format:yaml and type:map
					"path":    "mykv/yamltest/myjson",
					"address": "http://127.0.0.1:8200",
				},
			},
		},
		{
			config: map[string]any{
				// implies name:vault
				"vault": map[string]any{
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
				case map[string]any:
					key := "mykey"
					actual, ok := baz[key]
					if !ok {
						t.Fatalf("%q does not exist", key)
					}
					expected := testValue
					if actual != expected {
						t.Errorf("unexpected value for key %q: expected=%q, got=%q", key, expected, actual)
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
		config map[string]any
	}
	provider1 := map[string]any{
		"name":    "vault",
		"type":    "map",
		"path":    "mykv/foo",
		"address": "http://127.0.0.1:8200",
		"format":  "raw",
	}

	provider2 := map[string]any{
		"name": "vault",
		// implies format:raw
		"prefix":  "mykv/foo",
		"address": "http://127.0.0.1:8200",
	}

	testcases := []testcase{
		{
			config: map[string]any{
				"provider": provider1,
			},
		},
		{
			config: map[string]any{
				"provider": provider2,
			},
		},
		{
			config: map[string]any{
				// implies name:vault
				"vault": map[string]any{
					// implies format:raw
					"prefix":  "mykv/foo",
					"address": "http://127.0.0.1:8200",
				},
			},
		},
		{
			config: map[string]any{
				// implies name:ssm
				"vault": map[string]any{
					// implies format:raw
					"prefix":  "/mykv/foo",
					"keys":    []string{"mykey"},
					"address": "http://127.0.0.1:8200",
				},
			},
		},
		{
			config: map[string]any{
				// implies name:ssm
				"vault": map[string]any{
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
				expected := testValue
				if actual != expected {
					t.Errorf("unexpected value for key %q: expected=%q, got=%q", key, expected, actual)
				}
			}
		})
	}
}
