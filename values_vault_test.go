package values

import (
	"fmt"
	"testing"
)

func TestValues_Vault_SpruceMerge(t *testing.T) {
	// TODO
	// Pre-requisite: vault write mykv/foo mykey=myvalue

	type testcase struct {
		config map[string]interface{}
	}

	testcases := []testcase{
		{
			config: map[string]interface{}{
				"spruce": map[string]interface{}{
					"appendByDefault": false,
					"valuesFrom": []interface{}{
						map[string]interface{}{
							// implies name=vault and type=string
							"vault": map[string]interface{}{
								"path":    "mykv/foo",
								"address": "http://127.0.0.1:8200",
								"set": map[string]interface{}{
									"foo": "mykey",
									"bar": []string{
										"mykey",
									},
								},
							},
						},
						map[string]interface{}{
							// implies name=vault and type=string
							"vault": map[string]interface{}{
								"path":    "mykv/foo",
								"address": "http://127.0.0.1:8200",
								"set": map[string]interface{}{
									"foo": "mykey",
									"bar": []string{
										"(( prepend ))",
										"mykey",
									},
									"baz": "mykey",
								},
							},
						},
					},
				},
			},
		},
	}

	for i := range testcases {
		tc := testcases[i]
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			config := Map(tc.config)

			vals, err := New(config)
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
				case []interface{}:
					expected := "myvalue"
					if len(bar) != 2 {
						t.Fatalf("unexpected length: expected=2, got=%d", len(bar))
					}
					for i := range bar {
						if bar[i] != expected {
							t.Errorf("bar[%d] != %v: expected %v, got %v", i, expected, expected, bar[i])
						}
					}
				default:
					t.Fatalf("unexpected type of bar: value=%v, type=%T", bar, bar)
				}
			}

			{
				switch actual := vals["baz"].(type) {
				case string:
					expected := "myvalue"
					if actual != expected {
						t.Errorf("unepected value for key %q: expected=%q, got=%q", "baz", expected, actual)
					}
				default:
					t.Fatalf("unexpected type of baz: value=%v, type=%T", actual, actual)
				}
			}
		})
	}
}

func TestValues_Vault_String(t *testing.T) {
	// TODO
	// Pre-requisite: vault write mykv/foo mykey=myvalue

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
			config := Map(tc.config)

			vals, err := New(config)
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
			config := Map(tc.config)

			vals, err := New(config)
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
			config := Map(map[string]interface{}{
				"provider": tc.provider,
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

	type testcase struct {
		provider map[string]interface{}
		dataKey  string
	}
	provider1 := map[string]interface{}{
		"name":    "vault",
		"type":    "map",
		"path":    "mykv/yamltest",
		"address": "http://127.0.0.1:8200",
		"format":  "yaml",
	}

	provider2 := map[string]interface{}{
		"name": "vault",
		// implies `type: map`
		"path":    "mykv/yamltest",
		"address": "http://127.0.0.1:8200",
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
			config := Map(map[string]interface{}{
				"provider": tc.provider,
				"inline": map[string]interface{}{
					"bar": tc.dataKey,
				},
			})

			vals, err := New(config)
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
			config := Map(tc.config)

			vals, err := New(config)
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
			config := Map(tc.config)

			vals, err := New(config)
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
