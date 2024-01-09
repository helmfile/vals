package vals

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestValues_k8s(t *testing.T) {
	// Setup:
	// create a namespace:
	//   kubectl create namespace test-namespace
	// create a secret:
	//   kubectl create secret generic mysecret -n test-namespace --from-literal=key=p4ssw0rd
	// create a configmap:
	//   kubectl create configmap myconfigmap -n test-namespace --from-literal=key=configValue

	type testcase struct {
		template map[string]interface{}
		want     map[string]interface{}
		wantErr  string
	}

	homeDir, _ := os.UserHomeDir()

	testcases := []testcase{
		// (secret) valid Secret is specified, uses current context
		{
			template: map[string]interface{}{
				"test_key": "secretref+k8s://v1/Secret/test-namespace/mysecret/key",
			},
			want: map[string]interface{}{
				"test_key": "p4ssw0rd",
			},
			wantErr: "",
		},
		// (secret) valid Secret is specified, with specific kube context
		{
			template: map[string]interface{}{
				"test_key": "secretref+k8s://v1/Secret/test-namespace/mysecret/key?kubeContext=kind-cluster",
			},
			want: map[string]interface{}{
				"test_key": "p4ssw0rd",
			},
			wantErr: "",
		},
		// (secret) valid Secret is specified, with specific kube context and kube config path
		{
			template: map[string]interface{}{
				"test_key": fmt.Sprintf("secretref+k8s://v1/Secret/test-namespace/mysecret/key?kubeContext=kind-cluster&kubeConfigPath=%s/.kube/config", homeDir),
			},
			want: map[string]interface{}{
				"test_key": "p4ssw0rd",
			},
			wantErr: "",
		},
		// (secret) valid Secret is specified, with a kube config path, no specific kube context (uses current)
		{
			template: map[string]interface{}{
				"test_key": fmt.Sprintf("ref+k8s://v1/Secret/test-namespace/mysecret/key?kubeConfigPath=%s/.kube/config", homeDir),
			},
			want: map[string]interface{}{
				"test_key": "p4ssw0rd",
			},
			wantErr: "",
		},
		// (secret) non-existent Secret
		{
			template: map[string]interface{}{
				"test_key": "ref+k8s://v1/Secret/test-namespace/non-existent-secret/key",
			},
			want:    nil,
			wantErr: "expand k8s://v1/Secret/test-namespace/non-existent-secret/key: Unable to get Secret test-namespace/non-existent-secret: Unable to get the Secret object from Kubernetes: secrets \"non-existent-secret\" not found",
		},
		// (configmap) valid ConfigMap is specified, using current context
		{
			template: map[string]interface{}{
				"test_key": "ref+k8s://v1/ConfigMap/test-namespace/myconfigmap/key",
			},
			want: map[string]interface{}{
				"test_key": "configValue",
			},
			wantErr: "",
		},
		// (configmap) valid Secret is specified, with specific kube context
		{
			template: map[string]interface{}{
				"test_key": "ref+k8s://v1/ConfigMap/test-namespace/myconfigmap/key?kubeContext=kind-cluster",
			},
			want: map[string]interface{}{
				"test_key": "configValue",
			},
			wantErr: "",
		},
		// (configmap) valid ConfigMap is specified, with specific kube context and kube config path
		{
			template: map[string]interface{}{
				"test_key": fmt.Sprintf("ref+k8s://v1/ConfigMap/test-namespace/myconfigmap/key?kubeContext=kind-cluster&kubeConfigPath=%s/.kube/config", homeDir),
			},
			want: map[string]interface{}{
				"test_key": "configValue",
			},
		},
		// (configmap) non-existent ConfigMap
		{
			template: map[string]interface{}{
				"test_key": "ref+k8s://v1/ConfigMap/test-namespace/non-existent-configmap/key",
			},
			want:    nil,
			wantErr: "expand k8s://v1/ConfigMap/test-namespace/non-existent-configmap/key: Unable to get ConfigMap test-namespace/non-existent-configmap: Unable to get the ConfigMap object from Kubernetes: configmaps \"non-existent-configmap\" not found",
		},
		// unsupported kind
		{
			template: map[string]interface{}{
				"test_key": "ref+k8s://v1/UnsupportedKind/test-namespace/myconfigmap/key",
			},
			want:    nil,
			wantErr: "expand k8s://v1/UnsupportedKind/test-namespace/myconfigmap/key: Unable to get UnsupportedKind test-namespace/myconfigmap: The specified kind is not valid. Valid kinds: Secret, ConfigMap",
		},
		// unsupported apiVersion
		{
			template: map[string]interface{}{
				"test_key": "ref+k8s://v2/ConfigMap/test-namespace/myconfigmap/key",
			},
			want:    nil,
			wantErr: "expand k8s://v2/ConfigMap/test-namespace/myconfigmap/key: Invalid apiVersion v2. Only apiVersion v1 is supported at this time.",
		},
		// invalid apiVersion
		{
			template: map[string]interface{}{
				"test_key": "ref+k8s://invalidApiVersion/ConfigMap/test-namespace/myconfigmap/key",
			},
			want:    nil,
			wantErr: "expand k8s://invalidApiVersion/ConfigMap/test-namespace/myconfigmap/key: Invalid apiVersion invalidApiVersion. Only apiVersion v1 is supported at this time.",
		},
		// non-existent namespace
		{
			template: map[string]interface{}{
				"test_key": "ref+k8s://v1/ConfigMap/non-existent-namespace/myconfigmap/key",
			},
			want:    nil,
			wantErr: "expand k8s://v1/ConfigMap/non-existent-namespace/myconfigmap/key: Unable to get ConfigMap non-existent-namespace/myconfigmap: Unable to get the ConfigMap object from Kubernetes: configmaps \"myconfigmap\" not found",
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
