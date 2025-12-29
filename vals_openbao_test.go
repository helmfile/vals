package vals

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openbao/openbao/api/v2"

	config2 "github.com/helmfile/vals/pkg/config"
)

const (
	baoTestKey   = "mykey"
	baoTestValue = "myvalue"
)

type OpenBaoConn struct {
	Client *api.Client
	Token  string
}

func StartOpenBao(t *testing.T, mountPath, mountInputType string) (OpenBaoConn, func()) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())

	port := 8210 // Use different port than Vault to avoid conflicts
	devRootTokenID := "root"
	baoAddr := fmt.Sprintf("127.0.0.1:%v", port)
	bao := exec.CommandContext(ctx, "bao", "server",
		"-dev",
		"-dev-root-token-id="+devRootTokenID,
		fmt.Sprintf("-dev-listen-address=%s", baoAddr),
	)
	bao.Stdout = os.Stdout
	bao.Stderr = os.Stderr

	errs := make(chan error, 1)

	go func() {
		errs <- bao.Run()
	}()

	config := &api.Config{
		Address: "http://" + baoAddr,
	}
	client, err := api.NewClient(config)
	if err != nil {
		t.Fatalf("Failed creating openbao client: %v", err)
	}

	client.SetToken(devRootTokenID)

	client.Sys().Mount(mountPath, &api.MountInput{
		Type: mountInputType,
	})

	return OpenBaoConn{Client: client, Token: devRootTokenID}, func() {
		cancel()

		if err := <-errs; err != nil {
			t.Logf("stopping openbao: %v", err)
		}
	}
}

func SetupOpenBaoKV(t *testing.T, writes map[string]map[string]interface{}) (string, func()) {
	// TODO v2 api support where mountInputType should be "kv-v2" rather than "kv"
	conn, stop := StartOpenBao(t, "mykv", "kv")

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
	os.Setenv("BAO_TOKEN", conn.Token)

	return addr, stop
}

func TestValues_OpenBao_EvalTemplate(t *testing.T) {
	// Pre-requisite:
	//   bao secrets enable -path=mykv kv
	//   bao write mykv/foo mykey=myvalue
	//   bao read mykv/foo
	if os.Getenv("SKIP_TESTS") != "" {
		t.Skip("Skipping tests")
	}

	addr, stop := SetupOpenBaoKV(
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
				"foo": fmt.Sprintf("ref+openbao://mykv/foo?address=%s#/mykey", addr),
				"bar": map[string]interface{}{
					"baz": fmt.Sprintf("ref+openbao://mykv/foo?address=%s#/mykey", addr),
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
				fmt.Sprintf("ref+openbao://mykv/objs?address=%s#/myyaml", addr): map[string]interface{}{},
				fmt.Sprintf("ref+openbao://mykv/objs?address=%s#/myjson", addr): map[string]interface{}{},
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
				fmt.Sprintf("ref+openbao://mykv/objs?address=%s#/myyaml", addr): map[interface{}]interface{}{},
				fmt.Sprintf("ref+openbao://mykv/objs?address=%s#/myjson", addr): map[interface{}]interface{}{},
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

func TestValues_OpenBao_String(t *testing.T) {
	// Pre-requisite: bao write mykv/foo mykey=myvalue
	if os.Getenv("SKIP_TESTS") != "" {
		t.Skip("Skipping tests")
	}

	type testcase struct {
		config map[string]interface{}
	}
	commonInline := map[string]interface{}{
		"foo": baoTestKey,
		"bar": map[string]interface{}{
			"baz": baoTestKey,
		},
	}

	testcases := []testcase{
		{
			config: map[string]interface{}{
				"provider": map[string]interface{}{
					"name":    "openbao",
					"type":    "string",
					"path":    "mykv/foo",
					"address": "http://127.0.0.1:8210",
				},
				"inline": commonInline,
			},
		},
		{
			config: map[string]interface{}{
				"provider": map[string]interface{}{
					"name": "openbao",
					// implies type=string
					"path":    "mykv/foo",
					"address": "http://127.0.0.1:8210",
				},
				"inline": commonInline,
			},
		},
		{
			config: map[string]interface{}{
				// implies name=openbao and type=string
				"openbao": map[string]interface{}{
					"path":    "mykv/foo",
					"address": "http://127.0.0.1:8210",
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
				expected := baoTestValue
				key := "foo"
				actual := vals[key]
				if actual != expected {
					t.Errorf("unexpected value for key %q: expected=%q, got=%q", key, expected, actual)
				}
			}

			{
				switch bar := vals["bar"].(type) {
				case map[string]interface{}:
					expected := baoTestValue
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

func TestValues_OpenBao_Map(t *testing.T) {
	// Pre-requisite: bao write mykv/foo mykey=myvalue
	if os.Getenv("SKIP_TESTS") != "" {
		t.Skip("Skipping tests")
	}

	type testcase struct {
		config map[string]interface{}
		name   string
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
					"name":    "openbao",
					"type":    "map",
					"path":    "mykv",
					"address": "http://127.0.0.1:8210",
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
					"name":    "openbao",
					"type":    "map",
					"format":  "raw",
					"path":    "mykv",
					"address": "http://127.0.0.1:8210",
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
					"name": "openbao",
					// implies type:map format:raw
					"prefix":  "mykv",
					"address": "http://127.0.0.1:8210",
				},
			},
		},
		{
			name: "setForKey1",
			config: map[string]interface{}{
				"openbao": map[string]interface{}{
					// implies type:map format:raw
					"prefix":     "mykv/foo",
					"address":    "http://127.0.0.1:8210",
					"setForKeys": []string{"foo", "bar.baz"},
				},
			},
		},
		{
			name: "setForKey2",
			config: map[string]interface{}{
				"openbao": map[string]interface{}{
					// implies type:map format:raw
					"paths":      []string{"mykv/foo/mykey"},
					"address":    "http://127.0.0.1:8210",
					"setForKeys": []string{"foo", "bar.baz"},
				},
			},
		},
		{
			name: "setForKey3",
			config: map[string]interface{}{
				"openbao": map[string]interface{}{
					// implies type:map format:raw
					"prefix":     "mykv/foo/",
					"keys":       []string{baoTestKey},
					"address":    "http://127.0.0.1:8210",
					"setForKeys": []string{"foo", "bar.baz"},
				},
			},
		},
		{
			name: "set1",
			config: map[string]interface{}{
				"openbao": map[string]interface{}{
					// implies type:map format:raw
					"prefix":  "mykv",
					"address": "http://127.0.0.1:8210",
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
				"openbao": map[string]interface{}{
					// implies type:map format:raw
					"prefix":  "mykv",
					"address": "http://127.0.0.1:8210",
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
					key := baoTestKey
					actual, ok := foo[key]
					if !ok {
						t.Fatalf("%q does not exist", key)
					}
					expected := baoTestValue
					if actual != expected {
						t.Errorf("unexpected value for key %q: expected=%q, got=%q", key, expected, actual)
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
						key := baoTestKey
						actual, ok := baz[key]
						if !ok {
							t.Fatalf("%q does not exist", key)
						}
						expected := baoTestValue
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

func TestValues_OpenBao_Map_Raw(t *testing.T) {
	// Pre-requisite: bao write mykv/foo mykey=myvalue
	if os.Getenv("SKIP_TESTS") != "" {
		t.Skip("Skipping tests")
	}

	type testcase struct {
		provider map[string]interface{}
	}
	testcases := []testcase{
		{
			provider: map[string]interface{}{
				"name":    "openbao",
				"type":    "map",
				"path":    "mykv",
				"address": "http://127.0.0.1:8210",
				"format":  "raw",
			},
		},
		{
			provider: map[string]interface{}{
				"name": "openbao",
				// implies
				//"type":    "map",
				//"format":  "raw",
				"prefix":  "mykv",
				"address": "http://127.0.0.1:8210",
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
					key := baoTestKey
					actual, ok := foo[key]
					if !ok {
						t.Fatalf("%q does not exist", key)
					}
					expected := baoTestValue
					if actual != expected {
						t.Errorf("unexpected value for key %q: expected=%q, got=%q", key, expected, actual)
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
						key := baoTestKey
						actual, ok := baz[key]
						if !ok {
							t.Fatalf("%q does not exist", key)
						}
						expected := baoTestValue
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

func TestValues_OpenBao_Map_Raw_Root(t *testing.T) {
	// Pre-requisite: bao write mykv/foo mykey=myvalue
	if os.Getenv("SKIP_TESTS") != "" {
		t.Skip("Skipping tests")
	}

	type testcase struct {
		config map[string]interface{}
	}
	provider1 := map[string]interface{}{
		"name":    "openbao",
		"type":    "map",
		"path":    "mykv/foo",
		"address": "http://127.0.0.1:8210",
		"format":  "raw",
	}

	provider2 := map[string]interface{}{
		"name": "openbao",
		// implies format:raw
		"prefix":  "mykv/foo",
		"address": "http://127.0.0.1:8210",
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
				// implies name:openbao
				"openbao": map[string]interface{}{
					// implies format:raw
					"prefix":  "mykv/foo",
					"address": "http://127.0.0.1:8210",
				},
			},
		},
		{
			config: map[string]interface{}{
				// implies name:openbao
				"openbao": map[string]interface{}{
					// implies format:raw
					"prefix":  "/mykv/foo",
					"keys":    []string{baoTestKey},
					"address": "http://127.0.0.1:8210",
				},
			},
		},
		{
			config: map[string]interface{}{
				// implies name:openbao
				"openbao": map[string]interface{}{
					// implies format:raw
					"paths":   []string{"/mykv/foo/mykey"},
					"address": "http://127.0.0.1:8210",
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
				key := baoTestKey
				actual, ok := vals[key]
				if !ok {
					t.Fatalf("%q does not exist", key)
				}
				expected := baoTestValue
				if actual != expected {
					t.Errorf("unexpected value for key %q: expected=%q, got=%q", key, expected, actual)
				}
			}
		})
	}
}
