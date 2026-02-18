package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
)

// Setup:
// create a local Kubernetes cluster using minikube:
//   minikube start
// create a namespace:
//   kubectl create namespace test-namespace
// create a secret:
//   kubectl create secret generic mysecret -n test-namespace --from-literal=key=p4ssw0rd
// create a configmap:
//   kubectl create configmap myconfigmap -n test-namespace --from-literal=key=configValue

func Test_getObject(t *testing.T) {
	homeDir, _ := os.UserHomeDir()
	testcases := []struct {
		want           map[string]string
		namespace      string
		kind           string
		name           string
		kubeConfigPath string
		wantErr        string
		inCluster      bool
	}{
		// (secret) valid kubeConfigPath is specified
		{
			namespace:      "test-namespace",
			kind:           "Secret",
			name:           "mysecret",
			kubeConfigPath: fmt.Sprintf("%s/.kube/config", homeDir),
			want:           map[string]string{"key": "p4ssw0rd"},
			wantErr:        "",
		},
		// (secret) kubeConfigPath does not exist
		{
			namespace:      "test-namespace",
			kind:           "Secret",
			name:           "mysecret",
			kubeConfigPath: "/tmp/does-not-exist",
			want:           nil,
			wantErr:        "Unable to build config from vals configuration: stat /tmp/does-not-exist: no such file or directory",
		},
		// (secret) namespace does not exist
		{
			namespace:      "non-existent-namespace",
			kind:           "Secret",
			name:           "mysecret",
			kubeConfigPath: fmt.Sprintf("%s/.kube/config", homeDir),
			want:           nil,
			wantErr:        "Unable to get the Secret object from Kubernetes: secrets \"mysecret\" not found",
		},
		// (secret) secret does not exist
		{
			namespace:      "test-namespace",
			kind:           "Secret",
			name:           "non-existent-secret",
			kubeConfigPath: fmt.Sprintf("%s/.kube/config", homeDir),
			want:           nil,
			wantErr:        "Unable to get the Secret object from Kubernetes: secrets \"non-existent-secret\" not found",
		},
		// (configmap) valid kubeConfigPath is specified
		{
			namespace:      "test-namespace",
			kind:           "ConfigMap",
			name:           "myconfigmap",
			kubeConfigPath: fmt.Sprintf("%s/.kube/config", homeDir),
			want:           map[string]string{"key": "configValue"},
			wantErr:        "",
		},
		// (configmap) kubeConfigPath does not exist
		{
			namespace:      "test-namespace",
			kind:           "ConfigMap",
			name:           "myconfigmap",
			kubeConfigPath: "/tmp/does-not-exist",
			want:           nil,
			wantErr:        "Unable to build config from vals configuration: stat /tmp/does-not-exist: no such file or directory",
		},
		// (configmap) namespace does not exist
		{
			namespace:      "non-existent-namespace",
			kind:           "ConfigMap",
			name:           "myconfigmap",
			kubeConfigPath: fmt.Sprintf("%s/.kube/config", homeDir),
			want:           nil,
			wantErr:        "Unable to get the ConfigMap object from Kubernetes: configmaps \"myconfigmap\" not found",
		},
		// (configmap) configmap does not exist
		{
			namespace:      "test-namespace",
			kind:           "ConfigMap",
			name:           "non-existent-configmap",
			kubeConfigPath: fmt.Sprintf("%s/.kube/config", homeDir),
			want:           nil,
			wantErr:        "Unable to get the ConfigMap object from Kubernetes: configmaps \"non-existent-configmap\" not found",
		},
		// unsupported kind
		{
			namespace:      "test-namespace",
			kind:           "UnsupportedKind",
			name:           "myconfigmap",
			kubeConfigPath: fmt.Sprintf("%s/.kube/config", homeDir),
			want:           nil,
			wantErr:        "The specified kind is not valid. Valid kinds: Secret, ConfigMap",
		},
	}

	for i := range testcases {
		tc := testcases[i]
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			got, err := getObject(tc.kind, tc.namespace, tc.name, tc.kubeConfigPath, "", false, context.Background())
			if err != nil {
				if err.Error() != tc.wantErr {
					t.Fatalf("unexpected error: want %q, got %q", tc.wantErr, err.Error())
				}
			} else {
				if tc.wantErr != "" {
					t.Fatalf("expected error did not occur: want %q, got none", tc.wantErr)
				}
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("unexpected result: -(want), +(got)\n%s", diff)
			}
		})
	}
}

func Test_getObject_InCluster(t *testing.T) {
	testcases := []struct {
		want           map[string]string
		namespace      string
		kind           string
		name           string
		kubeConfigPath string
		wantErr        string
		inCluster      bool
	}{
		// (secret) Running outside a cluster
		{
			namespace:      "test-namespace",
			kind:           "Secret",
			name:           "mysecret",
			kubeConfigPath: "",
			inCluster:      true,
			want:           nil,
			wantErr:        "Unable to build config from vals configuration: unable to load in-cluster configuration, KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT must be defined",
		},
	}
	for i := range testcases {
		tc := testcases[i]
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			got, err := getObject(tc.kind, tc.namespace, tc.name, "", "", true, context.Background())
			if err != nil {
				if err.Error() != tc.wantErr {
					t.Fatalf("unexpected error: want %q, got %q", tc.wantErr, err.Error())
				}
			} else {
				if tc.wantErr != "" {
					t.Fatalf("expected error did not occur: want %q, got none", tc.wantErr)
				}
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("unexpected result: -(want), +(got)\n%s", diff)
			}
		})
	}
}

func Test_getKubeConfigPath(t *testing.T) {
	homeDir, _ := os.UserHomeDir()
	testcases := []struct {
		config           config.MapConfig
		kubeConfigEnvVar string
		want             string
		wantErr          string
	}{
		// kubeConfigPath is set
		{
			config: config.MapConfig{
				M: map[string]interface{}{
					"kubeConfigPath": fmt.Sprintf("%s/.kube/config", homeDir),
				},
			},
			want:    fmt.Sprintf("%s/.kube/config", homeDir),
			wantErr: "",
		},
		// kubeConfigPath does not exist
		{
			config: config.MapConfig{
				M: map[string]interface{}{"kubeConfigPath": "/tmp/does-not-exist"},
			},
			want:    "",
			wantErr: "kubeConfigPath URI parameter is set but path /tmp/does-not-exist does not exist.",
		},
		// KUBECONFIG specified path is set
		{
			config: config.MapConfig{
				M: map[string]interface{}{},
			},
			kubeConfigEnvVar: fmt.Sprintf("%s/.kube/config", homeDir),
			want:             fmt.Sprintf("%s/.kube/config", homeDir),
			wantErr:          "",
		},
		// KUBECONFIG specified path does not exist
		{
			config: config.MapConfig{
				M: map[string]interface{}{},
			},
			kubeConfigEnvVar: "/tmp/does-not-exist",
			want:             "",
			wantErr:          "KUBECONFIG environment variable is set but path /tmp/does-not-exist does not exist.",
		},
		// defaultPath exists
		{
			config: config.MapConfig{
				M: map[string]interface{}{},
			},
			kubeConfigEnvVar: "",
			want:             fmt.Sprintf("%s/.kube/config", homeDir),
			wantErr:          "",
		},
	}

	for i := range testcases {
		tc := testcases[i]
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			os.Unsetenv("KUBECONFIG")
			if tc.kubeConfigEnvVar != "" {
				os.Setenv("KUBECONFIG", tc.kubeConfigEnvVar)
			}
			got, err := getKubeConfigPath(tc.config)
			if err != nil {
				if err.Error() != tc.wantErr {
					t.Fatalf("unexpected error: want %q, got %q", tc.wantErr, err.Error())
				}
			} else {
				if tc.wantErr != "" {
					t.Fatalf("expected error did not occur: want %q, got none", tc.wantErr)
				}
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("unexpected result: -(want), +(got)\n%s", diff)
			}
		})
	}
}

func Test_getKubeContext(t *testing.T) {
	testcases := []struct {
		config config.MapConfig
		want   string
	}{
		// Valid kubeContext is specified
		{
			config: config.MapConfig{
				M: map[string]interface{}{
					"kubeContext": "minikube",
				},
			},
			want: "minikube",
		},
		// kubeContext is not specified, should return empty
		{
			config: config.MapConfig{
				M: map[string]interface{}{"kubeConfigPath": ""},
			},
			want: "",
		},
	}

	for i := range testcases {
		tc := testcases[i]
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			got := getKubeContext(tc.config)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("unexpected result: -(want), +(got)\n%s", diff)
			}
		})
	}
}

func Test_GetString(t *testing.T) {
	logger := log.New(log.Config{Output: os.Stderr})
	tests := []struct {
		path    string
		want    string
		wantErr string
	}{
		// (secret) Valid path is specified
		{
			path:    "v1/Secret/test-namespace/mysecret/key",
			want:    "p4ssw0rd",
			wantErr: "",
		},
		// (configmap) Valid path is specified
		{
			path:    "v1/ConfigMap/test-namespace/myconfigmap/key",
			want:    "configValue",
			wantErr: "",
		},
		// (secret) Invalid path is specified
		{
			path:    "v1/Secret/test-namespace/mysecret/key/more/path",
			want:    "",
			wantErr: "Invalid path v1/Secret/test-namespace/mysecret/key/more/path. Path must be in the format <apiVersion>/<kind>/<namespace>/<name>[/<key>]",
		},
		// (configmap) Invalid path is specified
		{
			path:    "v1/ConfigMap/test-namespace/myconfigmap/key/more/path",
			want:    "",
			wantErr: "Invalid path v1/ConfigMap/test-namespace/myconfigmap/key/more/path. Path must be in the format <apiVersion>/<kind>/<namespace>/<name>[/<key>]",
		},
		// (secret) Non-existent namespace is specified
		{
			path:    "v1/Secret/badnamespace/secret/key",
			want:    "",
			wantErr: "Unable to get Secret badnamespace/secret: Unable to get the Secret object from Kubernetes: secrets \"secret\" not found",
		},
		// (configmap) Non-existent secret is specified
		{
			path:    "v1/Secret/test-namespace/badsecret/key",
			want:    "",
			wantErr: "Unable to get Secret test-namespace/badsecret: Unable to get the Secret object from Kubernetes: secrets \"badsecret\" not found",
		},
		// (secret) Non-existent key is requested
		{
			path:    "v1/Secret/test-namespace/mysecret/non-existent-key",
			want:    "",
			wantErr: "Key non-existent-key does not exist in test-namespace/mysecret",
		},
		// (configmap) Non-existent key is requested
		{
			path:    "v1/ConfigMap/test-namespace/myconfigmap/non-existent-key",
			want:    "",
			wantErr: "Key non-existent-key does not exist in test-namespace/myconfigmap",
		},
		// (secret) Invalid apiVersion specified
		{
			path:    "v2/Secret/test-namespace/mysecret/non-existent-key",
			want:    "",
			wantErr: "Invalid apiVersion v2. Only apiVersion v1 is supported at this time.",
		},
		// (configmap) Invalid apiVersion specified
		{
			path:    "v2/ConfigMap/test-namespace/myconfigmap/non-existent-key",
			want:    "",
			wantErr: "Invalid apiVersion v2. Only apiVersion v1 is supported at this time.",
		},
		// Incorrect path is specified
		{
			path:    "bad/data/path",
			want:    "",
			wantErr: "Invalid path bad/data/path. Path must be in the format <apiVersion>/<kind>/<namespace>/<name>[/<key>]",
		},
		// Unsupported kind is specified
		{
			path:    "v1/UnsupportedKind/test-namespace/myconfigmap/key",
			want:    "",
			wantErr: "Unable to get UnsupportedKind test-namespace/myconfigmap: The specified kind is not valid. Valid kinds: Secret, ConfigMap",
		},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			// Create provider with mock
			homeDir, _ := os.UserHomeDir()
			conf := map[string]interface{}{}
			conf["kubeConfigPath"] = fmt.Sprintf("%s/.kube/config", homeDir)
			conf["kubeContext"] = "kind-cluster"
			p, err := New(logger, config.MapConfig{M: conf})
			require.NoErrorf(t, err, "unexpected error creating provider: %v", err)

			got, err := p.GetString(tc.path)
			if err != nil {
				if err.Error() != tc.wantErr {
					t.Fatalf("unexpected error: want %q, got %q", tc.wantErr, err.Error())
				}
			} else {
				if tc.wantErr != "" {
					t.Fatalf("expected error did not occur: want %q, got none", tc.wantErr)
				}
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("unexpected result: -(want), +(got)\n%s", diff)
			}
		})
	}
}

func Test_GetStringMap_Validation(t *testing.T) {
	// These tests validate path parsing and don't require a kubeconfig or cluster.
	logger := log.New(log.Config{Output: os.Stderr})
	p := &provider{log: logger}

	tests := []struct {
		path    string
		wantErr string
	}{
		// Invalid path - too few parts
		{
			path:    "v1/Secret/test-namespace",
			wantErr: "Invalid path v1/Secret/test-namespace. Path must be in the format <apiVersion>/<kind>/<namespace>/<name>",
		},
		// Invalid path - too many parts
		{
			path:    "v1/Secret/test-namespace/mysecret/key",
			wantErr: "Invalid path v1/Secret/test-namespace/mysecret/key. Path must be in the format <apiVersion>/<kind>/<namespace>/<name>",
		},
		// Invalid apiVersion
		{
			path:    "v2/Secret/test-namespace/mysecret",
			wantErr: "Invalid apiVersion v2. Only apiVersion v1 is supported at this time.",
		},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got, err := p.GetStringMap(tc.path)
			require.EqualError(t, err, tc.wantErr)
			require.Nil(t, got)
		})
	}
}

func Test_GetStringMap(t *testing.T) {
	logger := log.New(log.Config{Output: os.Stderr})
	tests := []struct {
		path    string
		want    map[string]interface{}
		wantErr string
	}{
		// (secret) Valid 4-part path returns all keys
		{
			path:    "v1/Secret/test-namespace/mysecret",
			want:    map[string]interface{}{"key": "p4ssw0rd"},
			wantErr: "",
		},
		// (configmap) Valid 4-part path returns all keys
		{
			path:    "v1/ConfigMap/test-namespace/myconfigmap",
			want:    map[string]interface{}{"key": "configValue"},
			wantErr: "",
		},
		// Unsupported kind
		{
			path:    "v1/UnsupportedKind/test-namespace/myresource",
			want:    nil,
			wantErr: "Unable to get UnsupportedKind test-namespace/myresource: The specified kind is not valid. Valid kinds: Secret, ConfigMap",
		},
		// (secret) Non-existent namespace
		{
			path:    "v1/Secret/badnamespace/mysecret",
			want:    nil,
			wantErr: "Unable to get Secret badnamespace/mysecret: Unable to get the Secret object from Kubernetes: secrets \"mysecret\" not found",
		},
		// (secret) Non-existent secret
		{
			path:    "v1/Secret/test-namespace/badsecret",
			want:    nil,
			wantErr: "Unable to get Secret test-namespace/badsecret: Unable to get the Secret object from Kubernetes: secrets \"badsecret\" not found",
		},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			homeDir, err := os.UserHomeDir()
			require.NoError(t, err, "failed to determine user home directory")
			conf := map[string]interface{}{}
			conf["kubeConfigPath"] = fmt.Sprintf("%s/.kube/config", homeDir)
			conf["kubeContext"] = "kind-cluster"
			p, err := New(logger, config.MapConfig{M: conf})
			require.NoErrorf(t, err, "unexpected error creating provider: %v", err)

			got, err := p.GetStringMap(tc.path)
			if err != nil {
				if tc.wantErr == "" {
					t.Fatalf("unexpected error: %v", err)
				}
				if err.Error() != tc.wantErr {
					t.Fatalf("unexpected error: want %q, got %q", tc.wantErr, err.Error())
				}
			} else if tc.wantErr != "" {
				t.Fatalf("expected error did not occur: want %q, got none", tc.wantErr)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("unexpected result: -(want), +(got)\n%s", diff)
			}
		})
	}
}

func Test_GetString_AllKeys_Validation(t *testing.T) {
	// These tests validate path parsing and don't require a kubeconfig or cluster.
	logger := log.New(log.Config{Output: os.Stderr})
	p := &provider{log: logger}

	tests := []struct {
		path    string
		wantErr string
	}{
		// Invalid apiVersion with 4-part path
		{
			path:    "v2/Secret/test-namespace/mysecret",
			wantErr: "Invalid apiVersion v2. Only apiVersion v1 is supported at this time.",
		},
		// Invalid path - too few parts
		{
			path:    "bad/data/path",
			wantErr: "Invalid path bad/data/path. Path must be in the format <apiVersion>/<kind>/<namespace>/<name>[/<key>]",
		},
		// Invalid path - too many parts (6+)
		{
			path:    "v1/Secret/test-namespace/mysecret/key/extra/path",
			wantErr: "Invalid path v1/Secret/test-namespace/mysecret/key/extra/path. Path must be in the format <apiVersion>/<kind>/<namespace>/<name>[/<key>]",
		},
		// Empty key (trailing slash)
		{
			path:    "v1/Secret/test-namespace/mysecret/",
			wantErr: "Invalid path v1/Secret/test-namespace/mysecret/. Key must not be empty in the format <apiVersion>/<kind>/<namespace>/<name>/<key>",
		},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got, err := p.GetString(tc.path)
			require.EqualError(t, err, tc.wantErr)
			require.Empty(t, got)
		})
	}
}

func Test_GetString_AllKeys(t *testing.T) {
	logger := log.New(log.Config{Output: os.Stderr})
	tests := []struct {
		path    string
		want    map[string]interface{}
		wantErr string
	}{
		// (secret) 4-part path returns JSON of all keys
		{
			path:    "v1/Secret/test-namespace/mysecret",
			want:    map[string]interface{}{"key": "p4ssw0rd"},
			wantErr: "",
		},
		// (configmap) 4-part path returns JSON of all keys
		{
			path:    "v1/ConfigMap/test-namespace/myconfigmap",
			want:    map[string]interface{}{"key": "configValue"},
			wantErr: "",
		},
		// Unsupported kind with 4-part path
		{
			path:    "v1/UnsupportedKind/test-namespace/myresource",
			want:    nil,
			wantErr: "Unable to get UnsupportedKind test-namespace/myresource: The specified kind is not valid. Valid kinds: Secret, ConfigMap",
		},
		// (secret) Non-existent namespace with 4-part path
		{
			path:    "v1/Secret/badnamespace/mysecret",
			want:    nil,
			wantErr: "Unable to get Secret badnamespace/mysecret: Unable to get the Secret object from Kubernetes: secrets \"mysecret\" not found",
		},
		// (secret) Non-existent secret with 4-part path
		{
			path:    "v1/Secret/test-namespace/badsecret",
			want:    nil,
			wantErr: "Unable to get Secret test-namespace/badsecret: Unable to get the Secret object from Kubernetes: secrets \"badsecret\" not found",
		},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			homeDir, err := os.UserHomeDir()
			require.NoError(t, err, "failed to determine user home directory")
			conf := map[string]interface{}{}
			conf["kubeConfigPath"] = fmt.Sprintf("%s/.kube/config", homeDir)
			conf["kubeContext"] = "kind-cluster"
			p, err := New(logger, config.MapConfig{M: conf})
			require.NoErrorf(t, err, "unexpected error creating provider: %v", err)

			got, err := p.GetString(tc.path)
			if err != nil {
				if tc.wantErr == "" {
					t.Fatalf("unexpected error: %v", err)
				}
				if err.Error() != tc.wantErr {
					t.Fatalf("unexpected error: want %q, got %q", tc.wantErr, err.Error())
				}
			} else if tc.wantErr != "" {
				t.Fatalf("expected error did not occur: want %q, got none", tc.wantErr)
			}

			if tc.want != nil {
				var gotMap map[string]interface{}
				require.NoErrorf(t, json.Unmarshal([]byte(got), &gotMap), "failed to unmarshal JSON: %s", got)
				if diff := cmp.Diff(tc.want, gotMap); diff != "" {
					t.Errorf("unexpected result: -(want), +(got)\n%s", diff)
				}
			}
		})
	}
}
