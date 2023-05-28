package vals

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/redis/go-redis/v9"

	config2 "github.com/helmfile/vals/pkg/config"
)

func StartRedis(t *testing.T, mountPath, mountInputType string) (string, func()) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())

	redis := exec.CommandContext(ctx, "redis-server")
	redis.Stdout = os.Stdout
	redis.Stderr = os.Stderr

	errs := make(chan error, 1)

	go func() {
		errs <- redis.Run()
	}()

	return "localhost:6379", func() {
		cancel()
		if err := <-errs; err != nil {
			t.Logf("stopping redis: %v", err)
		}
	}
}

func SetupRedisKV(t *testing.T, writes map[string]map[string]interface{}) (string, func()) {
	addr, stop := StartRedis(t, "my", "kv")

	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	for path, data := range writes {
		sec, err := client.HSet(context.Background(), path, data).Result()
		if err != nil {
			t.Logf("%v", sec)
			t.Fatalf("%v", err)
		}
	}

	return addr, stop
}

func TestValues_Redis_EvalTemplate(t *testing.T) {
	addr, stop := SetupRedisKV(
		t,
		map[string]map[string]interface{}{
			"my/foo": {
				"mykey": "myvalue",
			},
			"my/objs": {
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
				"foo": fmt.Sprintf("ref+redis://my/foo?address=%s#/mykey", addr),
				"bar": map[string]interface{}{
					"baz": fmt.Sprintf("ref+redis://my/foo?address=%s#/mykey", addr),
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
				fmt.Sprintf("ref+redis://my/objs?address=%s#/myyaml", addr): map[string]interface{}{},
				fmt.Sprintf("ref+redis://my/objs?address=%s#/myjson", addr): map[string]interface{}{},
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
				fmt.Sprintf("ref+redis://my/objs?address=%s#/myyaml", addr): map[interface{}]interface{}{},
				fmt.Sprintf("ref+redis://my/objs?address=%s#/myjson", addr): map[interface{}]interface{}{},
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

func TestValues_Redis_String(t *testing.T) {
	// TODO
	// Pre-requisite: redis-cli set my/foo/bar 'bar: { baz: myvalue }'

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
					"name":    "redis",
					"type":    "string",
					"path":    "my/foo",
					"address": "localhost:6379",
				},
				"inline": commonInline,
			},
		},
		{
			config: map[string]interface{}{
				"provider": map[string]interface{}{
					"name": "redis",
					// implies type=string
					"path":    "my/foo",
					"address": "localhost:6379",
				},
				"inline": commonInline,
			},
		},
		{
			config: map[string]interface{}{
				// implies name=redis and type=string
				"redis": map[string]interface{}{
					"path":    "my/foo",
					"address": "localhost:6379",
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

func TestValues_Redis_Map(t *testing.T) {
	// TODO
	// Pre-requisite: redis-cli set my/foo/bar 'bar: { baz: myvalue }'

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
					"name":    "redis",
					"type":    "map",
					"path":    "my",
					"address": "localhost:6379",
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
					"name":    "redis",
					"type":    "map",
					"format":  "raw",
					"path":    "my",
					"address": "localhost:6379",
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
					"name": "redis",
					// implies type:map format:raw
					"prefix":  "my",
					"address": "localhost:6379",
				},
			},
		},
		{
			name: "setForKey1",
			config: map[string]interface{}{
				"redis": map[string]interface{}{
					// implies type:map format:raw
					"prefix":     "my/foo",
					"address":    "localhost:6379",
					"setForKeys": []string{"foo", "bar.baz"},
				},
			},
		},
		{
			name: "setForKey2",
			config: map[string]interface{}{
				"redis": map[string]interface{}{
					// implies type:map format:raw
					"paths":      []string{"my/foo/mykey"},
					"address":    "localhost:6379",
					"setForKeys": []string{"foo", "bar.baz"},
				},
			},
		},
		{
			name: "setForKey3",
			config: map[string]interface{}{
				"redis": map[string]interface{}{
					// implies type:map format:raw
					"prefix":     "my/foo/",
					"keys":       []string{"mykey"},
					"address":    "localhost:6379",
					"setForKeys": []string{"foo", "bar.baz"},
				},
			},
		},
		{
			name: "set1",
			config: map[string]interface{}{
				"redis": map[string]interface{}{
					// implies type:map format:raw
					"prefix":  "my",
					"address": "localhost:6379",
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
				"redis": map[string]interface{}{
					// implies type:map format:raw
					"prefix":  "my",
					"address": "localhost:6379",
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

func TestValues_Redis_Map_Raw(t *testing.T) {
	// TODO
	// Pre-requisite: redis-cli set my/foo/bar 'bar: { baz: myvalue }'

	type testcase struct {
		provider map[string]interface{}
	}
	testcases := []testcase{
		{
			provider: map[string]interface{}{
				"name":    "redis",
				"type":    "map",
				"path":    "my",
				"address": "localhost:6379",
				"format":  "raw",
			},
		},
		{
			provider: map[string]interface{}{
				"name": "redis",
				// implies
				//"type":    "map",
				//"format":  "raw",
				"prefix":  "my",
				"address": "localhost:6379",
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

func TestValues_Redis_Map_YAML(t *testing.T) {
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
	// redis-cli set my/yamltest/myyaml "$(cat myyaml.yaml)"
	// redis-cli set my/yamltest/myjson "$(cat myjson.json)"

	yamlContent, err := os.ReadFile("myyaml.yaml")
	if err != nil {
		t.Fatalf("%v", err)
	}

	jsonContent, err := os.ReadFile("myjson.json")
	if err != nil {
		t.Fatalf("%v", err)
	}

	addr, stop := SetupRedisKV(
		t,
		map[string]map[string]interface{}{
			"my/yamltest": {
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
		"name":    "redis",
		"type":    "map",
		"path":    "my/yamltest",
		"address": addr,
		"format":  "yaml",
	}

	provider2 := map[string]interface{}{
		"name": "redis",
		// implies `type: map`
		"path":    "my/yamltest",
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

func TestValues_Redis_Map_YAML_Root(t *testing.T) {
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
	// redis-cli set my/yamltest/myyaml "$(cat myyaml.yaml)"
	// redis-cli set my/yamltest/myjson "$(cat myjson.json)"

	type provider struct {
		config map[string]interface{}
	}
	testcases := []provider{
		{
			config: map[string]interface{}{
				"provider": map[string]interface{}{
					"name":    "redis",
					"type":    "map",
					"path":    "my/yamltest/myyaml",
					"address": "localhost:6379",
					"format":  "yaml",
				},
			},
		},
		{
			config: map[string]interface{}{
				"provider": map[string]interface{}{
					"name":    "redis",
					"type":    "map",
					"path":    "my/yamltest/myjson",
					"address": "localhost:6379",
					"format":  "yaml",
				},
			},
		},
		{
			config: map[string]interface{}{
				"provider": map[string]interface{}{
					"name": "redis",
					// implies format:yaml and type:map
					"path":    "my/yamltest/myyaml",
					"address": "localhost:6379",
				},
			},
		},
		{
			config: map[string]interface{}{
				"provider": map[string]interface{}{
					"name": "redis",
					// implies format:yaml and type:map
					"path":    "my/yamltest/myjson",
					"address": "localhost:6379",
				},
			},
		},
		{
			config: map[string]interface{}{
				// implies name:redis
				"redis": map[string]interface{}{
					// implies format:yaml and type:map
					"path":    "my/yamltest/myjson",
					"address": "localhost:6379",
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

func TestValues_Redis_Map_Raw_Root(t *testing.T) {
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
	// redis-cli set my/yamltest/myyaml "$(cat myyaml.yaml)"
	// redis-cli set my/yamltest/myjson "$(cat myjson.json)"
	// redis-cli set /my/foo/mykey myvalue

	type testcase struct {
		config map[string]interface{}
	}
	provider1 := map[string]interface{}{
		"name":    "redis",
		"type":    "map",
		"path":    "my/foo",
		"address": "localhost:6379",
		"format":  "raw",
	}

	provider2 := map[string]interface{}{
		"name": "redis",
		// implies format:raw
		"prefix":  "my/foo",
		"address": "localhost:6379",
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
				// implies name:redis
				"redis": map[string]interface{}{
					// implies format:raw
					"prefix":  "my/foo",
					"address": "localhost:6379",
				},
			},
		},
		{
			config: map[string]interface{}{
				// implies name:ssm
				"redis": map[string]interface{}{
					// implies format:raw
					"prefix":  "/my/foo",
					"keys":    []string{"mykey"},
					"address": "localhost:6379",
				},
			},
		},
		{
			config: map[string]interface{}{
				// implies name:ssm
				"redis": map[string]interface{}{
					// implies format:raw
					"paths":   []string{"/my/foo/mykey"},
					"address": "localhost:6379",
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
