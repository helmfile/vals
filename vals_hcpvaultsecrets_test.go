package vals

import (
	"fmt"
	"os"
	"testing"

	config2 "github.com/helmfile/vals/pkg/config"
)

func TestValues_HCPVaultSecrets_String(t *testing.T) {
	// TODO
	// Pre-requisite:
	// 1. Create a Vault Secrets in HCP
	// 2. Configure the HCP Vault Secrets using a Service Principal https://developer.hashicorp.com/vault/tutorials/hcp-vault-secrets-get-started/hcp-vault-secrets-install-cli#configure-the-hcp-vault-secrets-cli
	// 3. Set the following environment variables:
	//    - HCP_CLIENT_ID
	//    - HCP_CLIENT_SECRET
	// 4. Run `vlt config init`
	// 5. Create an APP by running `echo y | vlt apps create vals-test`
	// 6. Create a simple secret by running `vlt secrets create --app-name vals-test fooKey="myValue"`

	if os.Getenv("SKIP_TESTS") != "" {
		t.Skip("Skipping tests")
	}

	type testcase struct {
		config map[string]interface{}
	}

	commonInline := map[string]interface{}{
		"vals-test": "fooKey",
	}

	testcases := []testcase{
		{
			config: map[string]interface{}{
				"provider": map[string]interface{}{
					"name": "hcpvaultsecrets",
					"type": "string",
					"path": "vals-test",
				},
				"inline": commonInline,
			},
		},
		{
			config: map[string]interface{}{
				"provider": map[string]interface{}{
					"name": "hcpvaultsecrets",
					// implies type=string
					"path": "vals-test",
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
				expected := "myValue"
				key := "vals-test"
				actual := vals[key]
				if actual != expected {
					t.Errorf("unexpected value for key %q: expected=%q, got=%q", key, expected, actual)
				}
			}
		})
	}
}
