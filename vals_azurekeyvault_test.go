package vals

import (
	"fmt"
	"os"
	"testing"

	config2 "github.com/helmfile/vals/pkg/config"
)

func TestValues_AzureKeyVault_String(t *testing.T) {
	// TODO
	// Pre-requisite:
	//  az group create --name vals-test-rg -l eastus2
	//  az keyvault create --name vals-test --resource-group vals-test-rg --location eastus2
	//  az keyvault secret set --name fooKey --vault-name vals-test --value myValue

	// create service principal for tests
	//  az ad sp create-for-rbac --name http://vals-test-sp --skip-assignment
	//  az keyvault set-policy --name vals-test --spn  http://vals-test-sp --secret-permissions get

	// set up service principal credentials in the environment:
	//  "AZURE_CLIENT_ID":  "...",
	//  "AZURE_CLIENT_SECRET": "...",
	//  "AZURE_TENANT_ID": "..."
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
					"name": "azurekeyvault",
					"type": "string",
					"path": "vals-test",
				},
				"inline": commonInline,
			},
		},
		{
			config: map[string]interface{}{
				"provider": map[string]interface{}{
					"name": "azurekeyvault",
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
