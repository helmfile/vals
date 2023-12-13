package k8s

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"

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

func Test_getObject(t *testing.T) {
	homeDir, _ := os.UserHomeDir()
	testcases := []struct {
		namespace      string
		name           string
		kubeConfigPath string
		want           map[string][]uint8
		wantErr        string
	}{
		{
			namespace:      "test-namespace",
			name:           "mysecret",
			kubeConfigPath: fmt.Sprintf("%s/.kube/config", homeDir),
			want:           map[string][]uint8{"key": []uint8("p4ssw0rd")},
			wantErr:        "",
		},
		// kubeConfigPath does not exist
		{
			namespace:      "test-namespace",
			name:           "mysecret",
			kubeConfigPath: "/tmp/does-not-exist",
			want:           nil,
			wantErr:        "Unable to build Kubeconfig from vals configuration: stat /tmp/does-not-exist: no such file or directory",
		},
		// namespace does not exist
		{
			namespace:      "non-existent-namespace",
			name:           "mysecret",
			kubeConfigPath: fmt.Sprintf("%s/.kube/config", homeDir),
			want:           nil,
			wantErr:        "Unable to get the object from Kubernetes: secrets \"mysecret\" not found",
		},
		// secret does not exist
		{
			namespace:      "test-namespace",
			name:           "non-existent-secret",
			kubeConfigPath: fmt.Sprintf("%s/.kube/config", homeDir),
			want:           nil,
			wantErr:        "Unable to get the object from Kubernetes: secrets \"non-existent-secret\" not found",
		},
	}

	for i := range testcases {
		tc := testcases[i]
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			got, err := getObject(tc.namespace, tc.name, tc.kubeConfigPath, "", context.Background())
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

func Test_getKubeConfig(t *testing.T) {
	homeDir, _ := os.UserHomeDir()
	testcases := []struct {
		config  config.MapConfig
		want    string
		wantErr string
	}{
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
	}

	for i := range testcases {
		tc := testcases[i]
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			got, err := getKubeConfig(tc.config)
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
		{
			path:    "v1/Secret/test-namespace/mysecret/key",
			want:    "p4ssw0rd",
			wantErr: "",
		},
		{
			path:    "v1/Secret/test-namespace/mysecret/non-existent-key",
			want:    "",
			wantErr: "Key non-existent-key does not exist in test-namespace/mysecret",
		},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			// Create provider with mock
			homeDir, _ := os.UserHomeDir()
			conf := map[string]interface{}{}
			conf["kubeConfigPath"] = fmt.Sprintf("%s/.kube/config", homeDir)
			conf["kubeContext"] = "minikube"
			p, _ := New(logger, config.MapConfig{M: conf})

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
