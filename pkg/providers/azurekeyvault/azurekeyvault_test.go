package azurekeyvault

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
			key:     "test-vault/a-secret",
			want:    secretSpec{"https://test-vault.vault.azure.net", "a-secret", ""},
			wantErr: "",
		},
		{
			// secret version
			key:     "test-vault/a-secret/v1",
			want:    secretSpec{"https://test-vault.vault.azure.net", "a-secret", "v1"},
			wantErr: "",
		},
		{
			// strips trailing slash
			key:     "test-vault/a-secret/",
			want:    secretSpec{"https://test-vault.vault.azure.net", "a-secret", ""},
			wantErr: "",
		},
		{
			// allows endpoint override
			key:     "test-vault.vault.usgovcloudapi.net/a-secret",
			want:    secretSpec{"https://test-vault.vault.usgovcloudapi.net", "a-secret", ""},
			wantErr: "",
		},
		{
			// illegal key
			key:     "too-short/",
			want:    secretSpec{},
			wantErr: `invalid secret specifier: "too-short/"`,
		},
		{
			// illegal key
			key:     "to/many/key/components",
			want:    secretSpec{},
			wantErr: `invalid secret specifier: "to/many/key/components"`,
		},
		{
			// missing vault name
			key:     "/secret",
			want:    secretSpec{},
			wantErr: `missing key vault name: "/secret"`,
		},
		{
			// missing secret name
			key:     "test-vault/ /",
			want:    secretSpec{},
			wantErr: `missing secret name: "test-vault/ /"`,
		},
	}

	for i := range testcases {
		tc := testcases[i]
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {

			got, err := parseKey(tc.key)
			if err != nil {
				if err.Error() != tc.wantErr {
					t.Fatalf("unexpected error: want %q, got %q", tc.wantErr, err.Error())
				}
			} else {
				if tc.wantErr != "" {
					t.Fatalf("expected error did not occur: want %q, got none", tc.wantErr)
				}
			}

			if diff := cmp.Diff(tc.want, got, cmp.AllowUnexported(secretSpec{})); diff != "" {
				t.Errorf("unexpected result: -(want), +(got)\n%s", diff)
			}
		})
	}
}
