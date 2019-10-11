package vals

import (
	"fmt"
	"testing"
)

func TestValues_SSM_String(t *testing.T) {
	// TODO
	// Pre-requisite: aws ssm put-parameter --name /mykv/foo/mykey --value myvalue --type String

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
					"name":   "ssm",
					"type":   "string",
					"path":   "/mykv/foo",
					"region": "ap-northeast-1",
				},
				"inline": commonInline,
			},
		},
		{
			config: map[string]interface{}{
				"provider": map[string]interface{}{
					"name": "ssm",
					// implies type=string
					"path":   "/mykv/foo",
					"region": "ap-northeast-1",
				},
				"inline": commonInline,
			},
		},
		{
			config: map[string]interface{}{
				// implies name=ssm and type=string
				"ssm": map[string]interface{}{
					"path":   "/mykv/foo",
					"region": "ap-northeast-1",
				},
				"inline": commonInline,
			},
		},
	}

	for i := range testcases {
		tc := testcases[i]
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			config := Map(tc.config)

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

func TestValues_SSM_Map(t *testing.T) {
	// TODO
	// Pre-requisite: aws ssm put-parameter --name /mykv/foo/mykey --value myvalue --type String

	type testcase struct {
		provider map[string]interface{}
	}

	testcases := []testcase{
		{
			provider: map[string]interface{}{
				"name":   "ssm",
				"type":   "map",
				"path":   "/mykv",
				"region": "ap-northeast-1",
			},
		},
		{
			provider: map[string]interface{}{
				"name":   "ssm",
				"type":   "map",
				"format": "raw",
				"path":   "/mykv",
				"region": "ap-northeast-1",
			},
		},
		{
			provider: map[string]interface{}{
				"name": "ssm",
				// implies type:map format:raw
				"prefix": "/mykv",
				"region": "ap-northeast-1",
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

func TestValues_SSM_Map_Raw(t *testing.T) {
	// TODO
	// Pre-requisite: aws ssm put-parameter --name /mykv/foo/mykey --value myvalue --type String

	type testcase struct {
		provider map[string]interface{}
	}

	testcases := []testcase{
		{
			provider: map[string]interface{}{
				"name":    "ssm",
				"type":    "map",
				"path":    "/mykv",
				"address": "http://127.0.0.1:8200",
				"format":  "raw",
			},
		},
		{
			provider: map[string]interface{}{
				"name": "ssm",
				// implies
				//"type":    "map",
				//"format":  "raw",
				"prefix":  "/mykv",
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

func TestValues_SSM_Map_YAML(t *testing.T) {
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
	// Pre-requisite:
	//   aws ssm put-parameter --name /mykv/yamltest/myyaml --value "$(cat myyaml.yaml)" --type String
	//   aws ssm put-parameter --name /mykv/yamltest/myjson --value "$(cat myjson.json)" --type String

	type testcase struct {
		provider map[string]interface{}
		dataKey  string
	}

	provider1 := map[string]interface{}{
		"name":   "ssm",
		"type":   "map",
		"path":   "/mykv/yamltest",
		"region": "ap-northeast-1",
		"format": "yaml",
	}

	provider2 := map[string]interface{}{
		"name": "ssm",
		// implies `type: map`
		"path":   "/mykv/yamltest",
		"region": "ap-northeast-1",
		"format": "yaml",
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

func TestValues_SSM_Map_YAML_Root(t *testing.T) {
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
	// Pre-requisite:
	//   aws ssm put-parameter --name /mykv/yamltest/myyaml --value "$(cat myyaml.yaml)" --type String
	//   aws ssm put-parameter --name /mykv/yamltest/myjson --value "$(cat myjson.json)" --type String

	type provider struct {
		config map[string]interface{}
	}
	testcases := []provider{
		{
			config: map[string]interface{}{
				"provider": map[string]interface{}{
					"name":   "ssm",
					"type":   "map",
					"path":   "/mykv/yamltest/myyaml",
					"region": "ap-northeast-1",
					"format": "yaml",
				},
			},
		},
		{
			config: map[string]interface{}{
				"provider": map[string]interface{}{
					"name":   "ssm",
					"type":   "map",
					"path":   "/mykv/yamltest/myjson",
					"region": "ap-northeast-1",
					"format": "yaml",
				},
			},
		},
		{
			config: map[string]interface{}{
				"provider": map[string]interface{}{
					"name": "ssm",
					// implies format:yaml and type:map
					"path":   "/mykv/yamltest/myyaml",
					"region": "ap-northeast-1",
				},
			},
		},
		{
			config: map[string]interface{}{
				"provider": map[string]interface{}{
					"name": "ssm",
					// implies format:yaml and type:map
					"path":   "/mykv/yamltest/myjson",
					"region": "ap-northeast-1",
				},
			},
		},
		{
			config: map[string]interface{}{
				// implies name:vault
				"ssm": map[string]interface{}{
					// implies format:yaml and type:map
					"path":   "/mykv/yamltest/myjson",
					"region": "ap-northeast-1",
				},
			},
		},
	}

	for i := range testcases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			tc := testcases[i]
			config := Map(tc.config)

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

func TestValues_SSM_Map_Raw_Root(t *testing.T) {
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
	// Pre-requisite:
	//   aws ssm put-parameter --name /mykv/yamltest/myyaml --value "$(cat myyaml.yaml)" --type String
	//   aws ssm put-parameter --name /mykv/yamltest/myjson --value "$(cat myjson.json)" --type String

	type testcase struct {
		config map[string]interface{}
	}

	provider1 := map[string]interface{}{
		"name":   "ssm",
		"type":   "map",
		"path":   "/mykv/foo",
		"region": "ap-northeast-1",
		"format": "raw",
	}

	provider2 := map[string]interface{}{
		"name": "ssm",
		// implies format:raw
		"prefix": "/mykv/foo",
		"region": "ap-northeast-1",
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
				// implies name:ssm
				"ssm": map[string]interface{}{
					// implies format:raw
					"prefix":  "/mykv/foo",
					"address": "http://127.0.0.1:8200",
				},
			},
		},
		{
			config: map[string]interface{}{
				// implies name:ssm
				"ssm": map[string]interface{}{
					// implies format:raw
					"prefix": "/mykv/foo",
					"keys":   []string{"mykey"},
					"region": "ap-northeast-1",
				},
			},
		},
		{
			config: map[string]interface{}{
				// implies name:ssm
				"ssm": map[string]interface{}{
					// implies format:raw
					"paths":  []string{"/mykv/foo/mykey"},
					"region": "ap-northeast-1",
				},
			},
		},
	}

	for i := range testcases {
		tc := testcases[i]

		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			config := Map(tc.config)

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
