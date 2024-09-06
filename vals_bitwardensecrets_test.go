package vals

import (
	"fmt"
	"os"
	"testing"

	config2 "github.com/helmfile/vals/pkg/config"
)

func TestValues_BitwardenSecrets_String(t *testing.T) {
	// TODO
	// Pre-requisite:
	// 1. Create a Project in Bitwarden Secrets Manager called "vals-test"
	// 2. Create a "Machine account", grant "Can read" permission on the "vals-test" project, and generate an access token
	// 3. Get the Organization ID from the Bitwarden Secrets Manager URL. For example, if the URL is https://vault.bitwarden.com/#/sm/00000000-0000-0000-0000-000000000000, the Organization ID is 00000000-0000-0000-0000-000000000000.
	// 4. Export the following environment variables:
	//    - BWS_ACCESS_TOKEN
	//    - BWS_ORGANIZATION_ID
	// 5. Create a secret in the "vals-test" project with name "fooKey" and value "myValue"

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
					"name": "bws",
					"type": "string",
					"path": "vals-test",
				},
				"inline": commonInline,
			},
		},
		{
			config: map[string]interface{}{
				"provider": map[string]interface{}{
					"name": "bws",
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
				t.Fatalf("failed to load config for testcase %d: %v", i, err)
			}

			{
				expected := "myValue"
				key := "vals-test"
				actual := vals[key]
				if actual != expected {
					t.Errorf("unepected value for key %q: expected=%q, got=%q", key, expected, actual)
				}
			}
		})
	}
}
