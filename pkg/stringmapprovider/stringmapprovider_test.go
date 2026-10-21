package stringmapprovider

import (
	"os"
	"strings"
	"testing"

	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
)

// TestS3ProviderCallsS3API verifies that stringmapprovider with name "s3"
// actually uses the S3 provider (GetObject), not the SSM provider
// (GetParametersByPath).
//
// This is a regression test for a copy-paste bug (introduced in eafc4c0, 2020)
// where the "s3" case returned ssm.New() instead of s3.New().
// The bug was invisible at the s3 package level (s3_test.go tests the provider
// directly), so we need this integration-style test at the routing layer.
//
// We verify by inspecting the error message: the S3 provider returns errors
// mentioning "s3 object" while the SSM provider returns errors mentioning
// "ssm" or "GetParametersByPath".
func TestS3ProviderCallsS3API(t *testing.T) {
	// Use a non-routable endpoint so the request fails fast with a
	// provider-specific error message we can inspect.
	t.Setenv("AWS_ENDPOINT_URL", "http://192.0.2.1:1") // TEST-NET-1, guaranteed non-routable
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")

	l := log.New(log.Config{Output: os.Stderr})
	cfg := config.Map(map[string]interface{}{
		"name":   "s3",
		"region": "us-east-1",
	})

	p, err := New(l, cfg, "")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	_, err = p.GetStringMap("mybucket/mykey")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	errMsg := err.Error()
	t.Logf("error: %s", errMsg)

	// The S3 provider wraps errors as "getting s3 object: ..." and the
	// underlying SDK error mentions "S3: GetObject".
	// The SSM provider would produce errors mentioning "ssm" or
	// "GetParametersByPath".
	if strings.Contains(errMsg, "GetParametersByPath") || strings.Contains(errMsg, "SSM") {
		t.Errorf("s3 stringmapprovider used SSM provider instead of S3:\n  error: %s", errMsg)
	}
	if !strings.Contains(errMsg, "s3") && !strings.Contains(errMsg, "S3") {
		t.Errorf("expected error to mention S3, got: %s", errMsg)
	}
}
