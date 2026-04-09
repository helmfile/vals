package awsclicompat

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// profileEnvMu guards the temporary clearing of AWS_PROFILE / AWS_DEFAULT_PROFILE
// that is needed in the profile-not-found fallback path. The AWS SDK v2 reads these
// environment variables directly via os.Getenv inside LoadDefaultConfig, so we must
// serialize callers that need to temporarily suppress them.
var profileEnvMu sync.Mutex

const (
	// LogModeOff represents no AWS SDK logging (secure default)
	// This is more readable than using the literal value 0
	LogModeOff aws.ClientLogMode = 0
)

// parseAWSLogLevel parses the AWS SDK log level from environment variable or provided default.
// Priority: AWS_SDK_GO_LOG_LEVEL env var > paramDefault parameter
//
// Supported values (case-insensitive, comma-separated):
// - "off" or empty: No logging
// - "minimal": Log retries only
// - "standard": Log retries and requests (previous default behavior)
// - "verbose": Log everything (requests, responses, bodies, signing)
// - "retries", "request", "request_with_body", "response", "response_with_body", "signing": Individual flags
//
// Default behavior (secure-by-default):
// - Empty/unset input: No logging to prevent sensitive information leakage
// - Invalid/unrecognized values: No logging to prevent accidental credential exposure
func parseAWSLogLevel(paramDefault string) aws.ClientLogMode {
	// Environment variable takes precedence (highest priority)
	logLevel := strings.TrimSpace(os.Getenv("AWS_SDK_GO_LOG_LEVEL"))

	// If env var not set, use parameter default
	if logLevel == "" {
		logLevel = paramDefault
	}

	// If still empty, default to no logging for security
	// See: https://github.com/helmfile/helmfile/issues/2270
	if logLevel == "" {
		return LogModeOff
	}

	// Handle preset levels (including "off")
	logLevelLower := strings.ToLower(logLevel)
	switch logLevelLower {
	case "off":
		return LogModeOff
	case "minimal":
		return aws.LogRetries
	case "standard":
		return aws.LogRetries | aws.LogRequest
	case "verbose":
		return aws.LogRetries | aws.LogRequest | aws.LogRequestWithBody |
			aws.LogResponse | aws.LogResponseWithBody | aws.LogSigning
	}

	// Parse individual flags (comma-separated)
	var mode aws.ClientLogMode
	levels := strings.Split(logLevel, ",")

	for _, level := range levels {
		level = strings.ToLower(strings.TrimSpace(level))
		switch level {
		case "retries":
			mode |= aws.LogRetries
		case "request":
			mode |= aws.LogRequest
		case "request_with_body":
			mode |= aws.LogRequestWithBody
		case "response":
			mode |= aws.LogResponse
		case "response_with_body":
			mode |= aws.LogResponseWithBody
		case "signing":
			mode |= aws.LogSigning
		}
	}

	// Secure-by-default: If no valid log levels were specified, default to no logging
	// This prevents accidental credential exposure from typos or invalid values
	if mode == 0 {
		return LogModeOff
	}

	return mode
}

// NewConfig enhances newConfig by adding support for assuming a role
// not specified in the AWS profile.
// The third parameter is the ARN of the role to assume.
// Optional: accepts a variadic logLevel parameter for AWS SDK logging configuration
//
// Both the config creation and the assumed role credentials use the
// specified region and the profile.
//
// If we need to use separate regions and profiles for each config,
// we might need to enhance this function further.
// That's another story though...
func NewConfig(ctx context.Context, region string, profile string, roleARN string, logLevel ...string) (aws.Config, error) {
	var level string
	if len(logLevel) > 0 {
		level = logLevel[0]
	}

	cfg, err := newConfig(ctx, region, profile, level)
	if err != nil {
		return aws.Config{}, err
	}

	if roleARN != "" {
		stsSvc := sts.NewFromConfig(cfg)
		cfg.Credentials = stscreds.NewAssumeRoleProvider(stsSvc, roleARN)
	}

	return cfg, nil
}

// newConfig creates a new AWS config for the given AWS region and AWS PROFILE.
//
// The following credential sources are supported:
//
// 1. static credentials (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
// 2. static credentials loaded from profiles (AWS_PROFILE, when AWS_SDK_LOAD_CONFIG=true)
// 3. dynamic credentials obtained by assuming the role using static credentials loaded from the profile (AWS_PROFILE, when AWS_SDK_LOAD_CONFIG=true)
// 4. dynamic credentials obtained by assuming the role using static credentials loaded from the env (FORCE_AWS_PROFILE=true w/ credential_source=Environment)
//
// The fourth option of using FORCE_AWS_PROFILE=true and AWS_PROFILE=yourprofile is equivalent to `aws --profile ${AWS_PROFILE}`.
// See https://github.com/helmfile/vals/issues/19#issuecomment-600437486 for more details and why and when this is needed.
//
// If the explicit profile parameter is specified but not found in the shared config, the
// function falls back to the default credential chain (e.g. EC2 instance profile,
// environment variables, etc.) without specifying any profile. This allows the fallback to
// work even when no ~/.aws/config exists at all (e.g., EC2 instances relying on IAM roles).
// Note: this fallback does not apply when the profile is selected via
// FORCE_AWS_PROFILE/AWS_PROFILE env vars (without an explicit profile param).
func newConfig(ctx context.Context, region string, profile string, logLevel string) (aws.Config, error) {
	// Build base options shared between initial load and fallback
	baseOpts := buildBaseOpts(region, logLevel)

	// Determine the effective profile: explicit param takes priority, then FORCE_AWS_PROFILE env path.
	effectiveProfile := profile
	if effectiveProfile == "" && os.Getenv("FORCE_AWS_PROFILE") == "true" {
		effectiveProfile = os.Getenv("AWS_PROFILE")
	}

	// Build full options including profile selection
	opts := baseOpts
	if effectiveProfile != "" {
		opts = append(opts, config.WithSharedConfigProfile(effectiveProfile))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		// If the explicit profile parameter doesn't exist, fall back to the default
		// credential chain (e.g. EC2 instance profile, environment variables, etc.).
		//
		// The AWS SDK v2 checks AWS_PROFILE (and AWS_DEFAULT_PROFILE) via os.Getenv
		// inside LoadDefaultConfig. If either is set to the same missing profile, the
		// fallback call would also fail with SharedConfigProfileNotExistError. We
		// therefore temporarily clear both variables under a mutex so that the SDK
		// uses loadSharedConfigIgnoreNotExist (its lenient path) instead.
		//
		// Do NOT use WithSharedConfigProfile("default") here: that still requires the
		// profile to exist in ~/.aws/config or ~/.aws/credentials. EC2 users may have
		// no AWS config files at all, relying entirely on the instance's IAM role.
		// Without an explicit profile option the SDK falls through its default
		// credential chain (env vars, shared credentials, EC2 IMDS, etc.).
		var profileNotExist config.SharedConfigProfileNotExistError
		if profile != "" && errors.As(err, &profileNotExist) {
			return loadDefaultConfigWithoutProfileEnv(ctx, baseOpts)
		}
		return aws.Config{}, err
	}

	return cfg, nil
}

// loadDefaultConfigWithoutProfileEnv loads the AWS default config while
// suppressing the AWS_PROFILE and AWS_DEFAULT_PROFILE environment variables.
//
// The AWS SDK v2 reads these variables directly inside LoadDefaultConfig
// (see resolveConfigLoaders in the SDK source). When either is set to a
// profile that does not exist in the shared config files, the SDK returns
// SharedConfigProfileNotExistError even when no explicit profile option is
// passed. Temporarily clearing the variables causes the SDK to use its
// lenient loader (loadSharedConfigIgnoreNotExist) and fall through to other
// credential sources (env key/secret, EC2 IMDS, etc.).
//
// profileEnvMu serializes concurrent callers so they do not observe each
// other's temporary env changes. The cleared values are always restored
// before the function returns.
func loadDefaultConfigWithoutProfileEnv(ctx context.Context, baseOpts []func(*config.LoadOptions) error) (aws.Config, error) {
	profileEnvMu.Lock()
	defer profileEnvMu.Unlock()

	// Save and clear AWS_PROFILE / AWS_DEFAULT_PROFILE.
	savedProfile := os.Getenv("AWS_PROFILE")
	savedDefaultProfile := os.Getenv("AWS_DEFAULT_PROFILE")
	if savedProfile != "" {
		if err := os.Unsetenv("AWS_PROFILE"); err != nil {
			return aws.Config{}, err
		}
	}
	if savedDefaultProfile != "" {
		if err := os.Unsetenv("AWS_DEFAULT_PROFILE"); err != nil {
			// Best-effort restore before returning.
			if savedProfile != "" {
				_ = os.Setenv("AWS_PROFILE", savedProfile)
			}
			return aws.Config{}, err
		}
	}

	cfg, err := config.LoadDefaultConfig(ctx, baseOpts...)

	// Restore the original values unconditionally.
	if savedProfile != "" {
		_ = os.Setenv("AWS_PROFILE", savedProfile)
	}
	if savedDefaultProfile != "" {
		_ = os.Setenv("AWS_DEFAULT_PROFILE", savedDefaultProfile)
	}

	return cfg, err
}

// buildBaseOpts constructs the common AWS config options (region, endpoint, log level)
// that are used both in the initial config load and in the profile-not-found fallback.
func buildBaseOpts(region string, logLevel string) []func(*config.LoadOptions) error {
	var opts []func(*config.LoadOptions) error

	// Set region if provided
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}

	// AWS_ENDPOINT_URL
	//
	// Whenever AWS gets around to having their Golang libraries
	// reach parity with their Python libraries and CLI, this
	// workaround can go away. In the meantime, this level of
	// configurability is useful for integrating with non-AWS
	// infrastructure like Localstack and Moto for testing and
	// development.
	//
	// https://github.com/aws/aws-sdk-go/issues/4942
	if endpointUrl := os.Getenv("AWS_ENDPOINT_URL"); endpointUrl != "" {
		// nolint:staticcheck // This deprecated API is needed for AWS_ENDPOINT_URL support
		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			// nolint:staticcheck // This deprecated API is needed for AWS_ENDPOINT_URL support
			return aws.Endpoint{
				URL:           endpointUrl,
				SigningRegion: region,
			}, nil
		})
		// nolint:staticcheck // This deprecated API is needed for AWS_ENDPOINT_URL support
		opts = append(opts, config.WithEndpointResolverWithOptions(customResolver))
	}

	// Configure client log mode based on AWS_SDK_GO_LOG_LEVEL environment variable or provided logLevel
	// Default to no logging for security (prevents credential leakage)
	opts = append(opts, config.WithClientLogMode(parseAWSLogLevel(logLevel)))

	return opts
}

// NewSession provides backwards compatibility for existing code
// Optional: accepts a variadic logLevel parameter for AWS SDK logging configuration
// Deprecated: Use NewConfig instead
func NewSession(region string, profile string, roleARN string, logLevel ...string) aws.Config {
	ctx := context.Background()
	cfg, err := NewConfig(ctx, region, profile, roleARN, logLevel...)
	if err != nil {
		panic(err) // This matches the old session.Must behavior
	}
	return cfg
}
