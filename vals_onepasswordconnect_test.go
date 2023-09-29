package vals

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestValues_OnePasswordConnect_EvalTemplate(t *testing.T) {
	// TODO
	// create vault and item for testing
	//  op vault create vals-test
	//  op item create --vault vals-test --title=vals-test username=foo@bar.org password=secret --category=login

	// Pre-requisite:
	//  Setup 1Password connect service with access to `vals-test` vault: https://developer.1password.com/docs/connect/

	// set up service principal credentials in the environment:
	//  "OP_CONNECT_TOKEN":  "...",
	//  "OP_CONNECT_HOST": "...",

	type testcase struct {
		template map[string]interface{}
		expected map[string]interface{}
	}
	vaultLabel := "vals-test"
	itemLabel := "vals-test"

	testcases := []testcase{
		{
			template: map[string]interface{}{
				"foo":      "FOO",
				"username": fmt.Sprintf("ref+onepasswordconnect://%s/%s#/username", vaultLabel, itemLabel),
				"password": fmt.Sprintf("ref+onepasswordconnect://%s/%s#/password", vaultLabel, itemLabel),
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
