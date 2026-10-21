package awsclicompat

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
)

func TestParseAWSLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected aws.ClientLogMode
	}{
		{
			name:     "empty environment variable defaults to no logging (secure default)",
			envValue: "",
			expected: LogModeOff,
		},
		{
			name:     "off disables all logging",
			envValue: "off",
			expected: LogModeOff,
		},
		{
			name:     "OFF (case insensitive) disables all logging",
			envValue: "OFF",
			expected: LogModeOff,
		},
		{
			name:     "retries only",
			envValue: "retries",
			expected: aws.LogRetries,
		},
		{
			name:     "request only",
			envValue: "request",
			expected: aws.LogRequest,
		},
		{
			name:     "request_with_body only",
			envValue: "request_with_body",
			expected: aws.LogRequestWithBody,
		},
		{
			name:     "response only",
			envValue: "response",
			expected: aws.LogResponse,
		},
		{
			name:     "response_with_body only",
			envValue: "response_with_body",
			expected: aws.LogResponseWithBody,
		},
		{
			name:     "signing only",
			envValue: "signing",
			expected: aws.LogSigning,
		},
		{
			name:     "retries and request (comma separated)",
			envValue: "retries,request",
			expected: aws.LogRetries | aws.LogRequest,
		},
		{
			name:     "request and response (comma separated)",
			envValue: "request,response",
			expected: aws.LogRequest | aws.LogResponse,
		},
		{
			name:     "all options (comma separated)",
			envValue: "retries,request,request_with_body,response,response_with_body,signing",
			expected: aws.LogRetries | aws.LogRequest | aws.LogRequestWithBody | aws.LogResponse | aws.LogResponseWithBody | aws.LogSigning,
		},
		{
			name:     "spaces in comma separated values",
			envValue: " retries , request ",
			expected: aws.LogRetries | aws.LogRequest,
		},
		{
			name:     "case insensitive",
			envValue: "RETRIES,REQUEST",
			expected: aws.LogRetries | aws.LogRequest,
		},
		{
			name:     "invalid values default to no logging (secure)",
			envValue: "invalid,unknown",
			expected: LogModeOff,
		},
		{
			name:     "mixed valid and invalid values use only valid ones",
			envValue: "retries,invalid,request",
			expected: aws.LogRetries | aws.LogRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original environment variable
			originalValue := os.Getenv("AWS_SDK_GO_LOG_LEVEL")
			defer func() {
				if originalValue == "" {
					os.Unsetenv("AWS_SDK_GO_LOG_LEVEL")
				} else {
					os.Setenv("AWS_SDK_GO_LOG_LEVEL", originalValue)
				}
			}()

			// Set test environment variable
			if tt.envValue == "" {
				os.Unsetenv("AWS_SDK_GO_LOG_LEVEL")
			} else {
				os.Setenv("AWS_SDK_GO_LOG_LEVEL", tt.envValue)
			}

			result := parseAWSLogLevel("")
			if result != tt.expected {
				t.Errorf("parseAWSLogLevel(\"\") = %d, want %d", result, tt.expected)
			}
		})
	}
}

// TestParseAWSLogLevelIndividualFlags tests that individual log mode flags work correctly
func TestParseAWSLogLevelIndividualFlags(t *testing.T) {
	// Test that LogRetries has the expected value
	os.Setenv("AWS_SDK_GO_LOG_LEVEL", "retries")
	defer os.Unsetenv("AWS_SDK_GO_LOG_LEVEL")

	result := parseAWSLogLevel("")
	if !result.IsRetries() {
		t.Errorf("Expected retries logging to be enabled")
	}
	if result.IsRequest() {
		t.Errorf("Expected request logging to be disabled")
	}
}

// TestDefaultSecureBehavior ensures the default prevents credential leakage
func TestDefaultSecureBehavior(t *testing.T) {
	// Ensure no AWS_SDK_GO_LOG_LEVEL is set
	os.Unsetenv("AWS_SDK_GO_LOG_LEVEL")

	result := parseAWSLogLevel("")
	expected := LogModeOff // No logging by default for security

	if result != expected {
		t.Errorf("Default behavior should be secure (no logging)! parseAWSLogLevel(\"\") = %d, want %d", result, expected)
	}
}

// TestPresetLevels tests the new preset log levels
func TestPresetLevels(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected aws.ClientLogMode
	}{
		{"minimal", "minimal", aws.LogRetries},
		{"standard", "standard", aws.LogRetries | aws.LogRequest},
		{"verbose", "verbose", aws.LogRetries | aws.LogRequest | aws.LogRequestWithBody | aws.LogResponse | aws.LogResponseWithBody | aws.LogSigning},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("AWS_SDK_GO_LOG_LEVEL")
			result := parseAWSLogLevel(tt.level)
			if result != tt.expected {
				t.Errorf("parseAWSLogLevel(%q) = %d, want %d", tt.level, result, tt.expected)
			}
		})
	}
}

// TestNewConfigProfileNotFoundFallback verifies that when a specified AWS profile
// does not exist in the shared config, newConfig falls back to the default
// credential chain instead of returning an error.
//
// The test simulates a realistic developer setup: ~/.aws/config has a [default]
// profile but not the profile specified in vals, and AWS_PROFILE is not set.
func TestNewConfigProfileNotFoundFallback(t *testing.T) {
	// Use a profile name that is guaranteed not to exist in the temp config files.
	const nonExistentProfile = "vals-test-profile-does-not-exist-12345"

	// Create temp AWS config and credentials files with only a [default] section.
	// This reflects the realistic scenario where a user has ~/.aws/config with a
	// default profile but not the one specified in vals.
	configFile, err := os.CreateTemp(t.TempDir(), "aws-config-*")
	if err != nil {
		t.Fatalf("creating temp AWS config file: %v", err)
	}
	if _, err := configFile.WriteString("[default]\n"); err != nil {
		t.Fatalf("writing temp AWS config file: %v", err)
	}
	configFile.Close()

	credFile, err := os.CreateTemp(t.TempDir(), "aws-credentials-*")
	if err != nil {
		t.Fatalf("creating temp AWS credentials file: %v", err)
	}
	if _, err := credFile.WriteString("[default]\n"); err != nil {
		t.Fatalf("writing temp AWS credentials file: %v", err)
	}
	credFile.Close()

	// Override environment so LoadDefaultConfig only reads the temp files.
	t.Setenv("AWS_CONFIG_FILE", configFile.Name())
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", credFile.Name())
	t.Setenv("AWS_DEFAULT_REGION", "us-east-1")
	// Clear variables that could interfere with profile or credential resolution.
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("FORCE_AWS_PROFILE", "")
	t.Setenv("AWS_SDK_LOAD_CONFIG", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_SESSION_TOKEN", "")

	ctx := context.Background()

	// First, confirm that loading directly with the non-existent profile does produce
	// SharedConfigProfileNotExistError — this ensures the fallback is actually exercised.
	_, directErr := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithSharedConfigFiles([]string{configFile.Name()}),
		awsconfig.WithSharedCredentialsFiles([]string{credFile.Name()}),
		awsconfig.WithSharedConfigProfile(nonExistentProfile),
	)
	var profileNotExist awsconfig.SharedConfigProfileNotExistError
	if !errors.As(directErr, &profileNotExist) {
		t.Fatalf("expected SharedConfigProfileNotExistError from direct load, got: %v", directErr)
	}

	// Now verify that newConfig falls back rather than propagating that error.
	_, fallbackErr := newConfig(ctx, "us-east-1", nonExistentProfile, "")
	if fallbackErr != nil {
		t.Fatalf("newConfig with non-existent profile should fall back to default credentials, got error: %v", fallbackErr)
	}
}

// TestNewConfigProfileNotFoundFallbackNoConfigFile verifies that the fallback works
// when no ~/.aws/config exists at all (e.g., EC2 instances relying on instance profiles).
// This is the primary scenario from https://github.com/helmfile/vals/issues/1094.
func TestNewConfigProfileNotFoundFallbackNoConfigFile(t *testing.T) {
	const nonExistentProfile = "vals-test-profile-does-not-exist-12345"

	// Point config files at paths that do not exist so there is no shared config
	// at all — this mirrors the EC2 case where no ~/.aws/config is present.
	nonExistentPath := filepath.Join(t.TempDir(), "does-not-exist")

	t.Setenv("AWS_CONFIG_FILE", nonExistentPath)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", nonExistentPath)
	t.Setenv("AWS_DEFAULT_REGION", "us-east-1")
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("FORCE_AWS_PROFILE", "")
	t.Setenv("AWS_SDK_LOAD_CONFIG", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_SESSION_TOKEN", "")

	ctx := context.Background()
	_, fallbackErr := newConfig(ctx, "us-east-1", nonExistentProfile, "")
	if fallbackErr != nil {
		t.Fatalf("newConfig should fall back to default credentials when profile is not found and no config file exists, got error: %v", fallbackErr)
	}
}

// TestNewConfigProfileNotFoundFallbackWithAWSProfileEnv verifies that the fallback
// still succeeds when AWS_PROFILE is set to the same missing profile.
// This is the regression guard for https://github.com/helmfile/vals/issues/1094 in
// environments where both `profile=dev` is passed to vals and AWS_PROFILE=dev is set
// in the shell.
func TestNewConfigProfileNotFoundFallbackWithAWSProfileEnv(t *testing.T) {
	const nonExistentProfile = "vals-test-profile-does-not-exist-12345"

	// Use empty (but existing) config files to ensure there really is no matching profile.
	configFile, err := os.CreateTemp(t.TempDir(), "aws-config-*")
	if err != nil {
		t.Fatalf("creating temp AWS config file: %v", err)
	}
	configFile.Close()

	credFile, err := os.CreateTemp(t.TempDir(), "aws-credentials-*")
	if err != nil {
		t.Fatalf("creating temp AWS credentials file: %v", err)
	}
	credFile.Close()

	t.Setenv("AWS_CONFIG_FILE", configFile.Name())
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", credFile.Name())
	t.Setenv("AWS_DEFAULT_REGION", "us-east-1")
	// Set AWS_PROFILE to the same missing profile — this is the regression scenario.
	t.Setenv("AWS_PROFILE", nonExistentProfile)
	t.Setenv("FORCE_AWS_PROFILE", "")
	t.Setenv("AWS_SDK_LOAD_CONFIG", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_SESSION_TOKEN", "")

	ctx := context.Background()
	_, fallbackErr := newConfig(ctx, "us-east-1", nonExistentProfile, "")
	if fallbackErr != nil {
		t.Fatalf("newConfig should fall back to default credentials when AWS_PROFILE is set to the same missing profile, got error: %v", fallbackErr)
	}
}
