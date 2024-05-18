package vals

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestValues_OnePassword_EvalTemplate(t *testing.T) {
	// TODO
	// 1. Create vault and item for testing and a service account
	//  op vault create vals-test
	//  op item create --vault vals-test --title=vals-test username=foo@bar.org password=secret --category=login
	//  op service-account create "Vals Test Service Account" --expires-in 24h --vault vals-test:read_items

	// 2. Set up the new service account token as environment variable:
	//  export OP_SERVICE_ACCOUNT_TOKEN=ops_xxxxxxxxx
	if os.Getenv("SKIP_TESTS") != "" {
		t.Skip("Skipping tests")
	}

	type testcase struct {
		template map[string]interface{}
		expected map[string]interface{}
	}
	vaultName := "vals-test"
	itemName := "vals-test"

	testcases := []testcase{
		{
			template: map[string]interface{}{
				"foo":      "FOO",
				"username": fmt.Sprintf("ref+op://%s/%s/username", vaultName, itemName),
				"password": fmt.Sprintf("ref+op://%s/%s/password", vaultName, itemName),
			},
			expected: map[string]interface{}{
				"foo":      "FOO",
				"username": "foo@bar.org",
				"password": "secret",
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
				t.Errorf("unxpected diff: %s", diff)
			}
		})
	}
}
