package vals

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	secret "github.com/scaleway/scaleway-sdk-go/api/secret/v1beta1"
	"github.com/scaleway/scaleway-sdk-go/scw"
)

func TestValues_SCW_String(t *testing.T) {
	if os.Getenv("SKIP_TESTS") != "" {
		t.Skip("Skipping tests")
	}

	projectID := os.Getenv("SCW_PROJECT_ID")
	if projectID == "" {
		projectID = os.Getenv("SCW_DEFAULT_PROJECT_ID")
	}
	projectOpts := scw.WithDefaultProjectID(projectID)

	region := os.Getenv("SCW_REGION")
	if region == "" {
		region = os.Getenv("SCW_DEFAULT_REGION")
	}
	regionOpts := scw.WithDefaultRegion(scw.Region(region))

	accessKey := os.Getenv("SCW_ACCESS_KEY")
	secretKey := os.Getenv("SCW_SECRET_KEY")
	authOpts := scw.WithAuth(accessKey, secretKey)

	client, err := scw.NewClient(
		projectOpts,
		authOpts,
		regionOpts,
	)

	if err != nil {
		t.Fatalf("Error creating scaleway client: %v", err)
	}

	path := "/confidential"

	api := secret.NewAPI(client)

	secretCreateRequest := &secret.CreateSecretRequest{
		Region:    scw.Region(region),
		ProjectID: projectID,
		Path:      &path,
		Name:      "foo",
		Type:      "Opaque",
	}

	secretResponse, err := api.CreateSecret(secretCreateRequest)

	if err != nil {
		t.Fatalf("Error creating secret: %v", err)
	}

	request := &secret.CreateSecretVersionRequest{
		SecretID: secretResponse.ID,
		Data:     []byte("myvalue"),
	}

	updatedSecret, err := api.CreateSecretVersion(request)

	if err != nil {
		t.Fatalf("Error creating secret version: %v", err)
	}

	defer func() {
		deleteSecretRequest := &secret.DeleteSecretRequest{
			SecretID: updatedSecret.SecretID,
			Region:   scw.Region(region),
		}
		if err := api.DeleteSecret(deleteSecretRequest); err != nil {
			t.Fatalf("Error deleting secret: %v", err)
		}
	}()

	type testcase struct {
		template map[string]interface{}
		expected map[string]interface{}
	}

	testcases := []testcase{
		{
			template: map[string]interface{}{
				"test_key": "ref+scw:///confidential/foo",
			},
			expected: map[string]interface{}{
				"test_key": "myvalue",
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

func TestValues_SCW_Json(t *testing.T) {
	if os.Getenv("SKIP_TESTS") != "" {
		t.Skip("Skipping tests")
	}

	projectID := os.Getenv("SCW_PROJECT_ID")
	if projectID == "" {
		projectID = os.Getenv("SCW_DEFAULT_PROJECT_ID")
	}
	projectOpts := scw.WithDefaultProjectID(projectID)

	region := os.Getenv("SCW_REGION")
	if region == "" {
		region = os.Getenv("SCW_DEFAULT_REGION")
	}
	regionOpts := scw.WithDefaultRegion(scw.Region(region))

	accessKey := os.Getenv("SCW_ACCESS_KEY")
	secretKey := os.Getenv("SCW_SECRET_KEY")
	authOpts := scw.WithAuth(accessKey, secretKey)

	client, err := scw.NewClient(
		projectOpts,
		authOpts,
		regionOpts,
	)

	if err != nil {
		t.Fatalf("Error creating scaleway client: %v", err)
	}

	path := "/confidential"

	api := secret.NewAPI(client)

	secretCreateRequest := &secret.CreateSecretRequest{
		Region:    scw.Region(region),
		ProjectID: projectID,
		Path:      &path,
		Name:      "bar",
		Type:      "key_value",
	}

	secretResponse, err := api.CreateSecret(secretCreateRequest)

	if err != nil {
		t.Fatalf("Error creating secret: %v", err)
	}

	request := &secret.CreateSecretVersionRequest{
		SecretID: secretResponse.ID,
		Data:     []byte(`{"mykey":"myvalue"}`),
	}

	updatedSecret, err := api.CreateSecretVersion(request)

	if err != nil {
		t.Fatalf("Error creating secret version: %v", err)
	}

	defer func() {
		deleteSecretRequest := &secret.DeleteSecretRequest{
			SecretID: updatedSecret.SecretID,
			Region:   scw.Region(region),
		}
		if err := api.DeleteSecret(deleteSecretRequest); err != nil {
			t.Fatalf("Error deleting secret: %v", err)
		}
	}()

	type testcase struct {
		template map[string]interface{}
		expected map[string]interface{}
	}

	testcases := []testcase{
		{
			template: map[string]interface{}{
				"test_key": "ref+scw:///confidential/bar",
			},
			expected: map[string]interface{}{
				"test_key": `{"mykey":"myvalue"}`,
			},
		},
		{
			template: map[string]interface{}{
				"test_key": "ref+scw:///confidential/bar#mykey",
			},
			expected: map[string]interface{}{
				"test_key": "myvalue",
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
