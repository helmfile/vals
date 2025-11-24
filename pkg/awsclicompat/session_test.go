package awsclicompat

import (
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
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
