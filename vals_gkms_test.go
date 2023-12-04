package vals

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestValues_GKMS(t *testing.T) {
	// TODO
	// create gkms and encrypt test value
	//  gcloud kms keyrings create "test" --location "global"
	//  gcloud kms keys create "default" --location "global" --keyring "test" --purpose "encryption"
	//  echo -n "test_value" \
	//    | gcloud kms encrypt \
	//      --location "global" \
	//      --keyring "test" \
	//      --key "default" \
	//      --plaintext-file - \
	//      --ciphertext-file - \
	//    | base64 -w0 \
	//    | tr '/+' '_-'
	//
	// run with:
	//
	//	go test -run '^(TestValues_GKMS)$'

	type testcase struct {
		template map[string]interface{}
		expected map[string]interface{}
	}

	plain_value := "test_value"
	encrypted_value := "CiQAmPqoGAKT97oUK0DdiI_cLDm3j6iPDK4-TJ3yQII-snFHCckSMwAkTpnEoD5wOeRaZrt3eC1ewFMuw617fqqjTStrsar9ciGERzk5t6uMgA0HKzSxGMdjHQ=="

	project := "test-project"
	location := "global"
	keyring := "test"
	crypto_key := "default"

	testcases := []testcase{
		{
			template: map[string]interface{}{
				"test_key": fmt.Sprintf("ref+gkms://%s?project=%s&location=%s&keyring=%s&crypto_key=%s", encrypted_value, project, location, keyring, crypto_key),
			},
			expected: map[string]interface{}{
				"test_key": plain_value,
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
