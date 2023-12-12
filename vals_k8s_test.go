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
		expected map[string]interface{}
	}

	namespace := "test-namespace"
	key := "key"
	homeDir, _ := os.UserHomeDir()

	testcases := []testcase{
		{
			template: map[string]interface{}{
				"test_key": fmt.Sprintf("secretref+k8s://%s/%s/%s", namespace, "mysecret", key),
			},
			expected: map[string]interface{}{
				"test_key": "p4ssw0rd",
			},
		},
		{
			template: map[string]interface{}{
				"test_key": fmt.Sprintf("secretref+k8s://%s/%s/%s?kubeContext=minikube", namespace, "mysecret", key),
			},
			expected: map[string]interface{}{
				"test_key": "p4ssw0rd",
			},
		},
		{
			template: map[string]interface{}{
				"test_key": fmt.Sprintf("secretref+k8s://%s/%s/%s?kubeContext=minikube&kubeConfigPath=%s/.kube/config", namespace, "mysecret", key, homeDir),
			},
			expected: map[string]interface{}{
				"test_key": "p4ssw0rd",
			},
		},
	}

	for i := range testcases {
		tc := testcases[i]
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			vals, err := Eval(tc.template)
			if err != nil {
				t.Fatalf("%v", err)
			}
			diff := cmp.Diff(tc.expected, vals)
			if diff != "" {
				t.Errorf("unexpected diff: %s", diff)
			}
		})
	}
}
