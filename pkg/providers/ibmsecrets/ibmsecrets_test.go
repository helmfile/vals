package ibmsecrets

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_parseKey(t *testing.T) {
	testcases := []struct {
		key     string
		want    secretSpec
		wantErr string
	}{
		{
			key: "secret_groups/mygroup/secret_types/arbitrary/secrets/mysecret",
			want: secretSpec{
				groupName:    "mygroup",
				secretType:   "arbitrary",
				secretName:   "mysecret",
				fragmentPath: "",
			},
		},
		{
			// leading slash is tolerated
			key: "/secret_groups/mygroup/secret_types/kv/secrets/mysecret",
			want: secretSpec{
				groupName:    "mygroup",
				secretType:   "kv",
				secretName:   "mysecret",
				fragmentPath: "",
			},
		},
		{
			// fragment path
			key: "secret_groups/mygroup/secret_types/kv/secrets/mysecret#/some/nested/key",
			want: secretSpec{
				groupName:    "mygroup",
				secretType:   "kv",
				secretName:   "mysecret",
				fragmentPath: "some/nested/key",
			},
		},
		{
			// service_credentials with fragment
			key: "secret_groups/prod/secret_types/service_credentials/secrets/mysc#/apikey",
			want: secretSpec{
				groupName:    "prod",
				secretType:   "service_credentials",
				secretName:   "mysc",
				fragmentPath: "apikey",
			},
		},
		{
			// too few path segments
			key:     "secret_groups/mygroup/secret_types/kv",
			want:    secretSpec{},
			wantErr: `ibmsecrets: invalid key "secret_groups/mygroup/secret_types/kv": expected secret_groups/<group>/secret_types/<type>/secrets/<name>`,
		},
		{
			// wrong segment labels
			key:     "groups/mygroup/types/kv/secrets/mysecret",
			want:    secretSpec{},
			wantErr: `ibmsecrets: invalid key "groups/mygroup/types/kv/secrets/mysecret": expected secret_groups/<group>/secret_types/<type>/secrets/<name>`,
		},
		{
			// empty group name
			key:     "secret_groups/ /secret_types/kv/secrets/mysecret",
			want:    secretSpec{},
			wantErr: `ibmsecrets: missing group name in key "secret_groups/ /secret_types/kv/secrets/mysecret"`,
		},
		{
			// empty secret name
			key:     "secret_groups/mygroup/secret_types/kv/secrets/ ",
			want:    secretSpec{},
			wantErr: `ibmsecrets: missing secret name in key "secret_groups/mygroup/secret_types/kv/secrets/ "`,
		},
	}

	for i, tc := range testcases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			got, err := parseKey(tc.key)
			if err != nil {
				if err.Error() != tc.wantErr {
					t.Fatalf("unexpected error: want %q, got %q", tc.wantErr, err.Error())
				}
				return
			}
			if tc.wantErr != "" {
				t.Fatalf("expected error %q, got none", tc.wantErr)
			}
			if diff := cmp.Diff(tc.want, got, cmp.AllowUnexported(secretSpec{})); diff != "" {
				t.Errorf("unexpected result (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_getValueByPath(t *testing.T) {
	obj := map[string]interface{}{
		"apikey": "the-key",
		"nested": map[string]interface{}{
			"inner": "inner-value",
		},
	}

	testcases := []struct {
		path    string
		want    string
		wantErr string
	}{
		{path: "apikey", want: "the-key"},
		{path: "nested/inner", want: "inner-value"},
		{path: "missing", wantErr: `ibmsecrets: key "missing" not found in secret data (path "missing")`},
	}

	for i, tc := range testcases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			got, err := getValueByPath(obj, tc.path)
			if err != nil {
				if err.Error() != tc.wantErr {
					t.Fatalf("unexpected error: want %q, got %q", tc.wantErr, err.Error())
				}
				return
			}
			if tc.wantErr != "" {
				t.Fatalf("expected error %q, got none", tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("want %q, got %q", tc.want, got)
			}
		})
	}
}
