package vals

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestValues_k8s(t *testing.T) {
	// Setup:
	// create a local Kubernetes cluster using minikube:
	//   minikube start
	// create a namespace:
	//   kubectl create namespace test-namespace
	// create a secret:
	//   kubectl create secret generic mysecret -n test-namespace --from-literal=key=p4ssw0rd

	type testcase struct {
		template map[string]interface{}
		want     map[string]interface{}
		wantErr  string
	}

	apiVersion := "v1"
	kind := "Secret"
	namespace := "test-namespace"
	key := "key"
	homeDir, _ := os.UserHomeDir()

	testcases := []testcase{
		{
			template: map[string]interface{}{
				"test_key": fmt.Sprintf("secretref+k8s://%s/%s/%s/%s/%s", apiVersion, kind, namespace, "mysecret", key),
			},
			want: map[string]interface{}{
				"test_key": "p4ssw0rd",
			},
			wantErr: "",
		},
		{
			template: map[string]interface{}{
				"test_key": fmt.Sprintf("secretref+k8s://%s/%s/%s/%s/%s?kubeContext=minikube", apiVersion, kind, namespace, "mysecret", key),
			},
			want: map[string]interface{}{
				"test_key": "p4ssw0rd",
			},
			wantErr: "",
		},
		{
			template: map[string]interface{}{
				"test_key": fmt.Sprintf("secretref+k8s://%s/%s/%s/%s/%s?kubeContext=minikube&kubeConfigPath=%s/.kube/config", apiVersion, kind, namespace, "mysecret", key, homeDir),
			},
			want: map[string]interface{}{
				"test_key": "p4ssw0rd",
			},
			wantErr: "",
		},
		{
			template: map[string]interface{}{
				"test_key": fmt.Sprintf("secretref+k8s://%s/%s/%s/%s/%s?kubeContext=minikube&kubeConfigPath=%s/.kube/config", "v2", kind, namespace, "mysecret", key, homeDir),
			},
			want:    nil,
			wantErr: fmt.Sprintf("expand k8s://v2/Secret/test-namespace/mysecret/key?kubeContext=minikube&kubeConfigPath=%s/.kube/config: Invalid apiVersion v2. Only apiVersion v1 is supported at this time.", homeDir),
		},
	}

	for i := range testcases {
		tc := testcases[i]
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			vals, err := Eval(tc.template)
			if err != nil {
				if err.Error() != tc.wantErr {
					t.Fatalf("unexpected error: want %q, got %q", tc.wantErr, err.Error())
				}
			} else {
				if tc.wantErr != "" {
					t.Fatalf("expected error did not occur: want %q, got none", tc.wantErr)
				}
			}
			diff := cmp.Diff(tc.want, vals)
			if diff != "" {
				t.Errorf("unexpected diff: %s", diff)
			}
		})
	}
}
