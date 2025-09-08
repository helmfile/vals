package awsclicompat

import (
	"context"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// parseAWSLogLevel parses the AWS_SDK_GO_LOG_LEVEL environment variable
// and returns the corresponding aws.ClientLogMode.
// Supported values (case-insensitive, comma-separated):
// - "off" or empty: No logging
// - "retries": Log retries only
// - "request": Log requests only
// - "request_with_body": Log requests with body
// - "response": Log responses only
// - "response_with_body": Log responses with body
// - "signing": Log signing
// Default (when not set): "retries,request" for backward compatibility
func parseAWSLogLevel() aws.ClientLogMode {
	logLevel := strings.TrimSpace(os.Getenv("AWS_SDK_GO_LOG_LEVEL"))

	// Default to retries and request logging for backward compatibility
	if logLevel == "" {
		return aws.LogRetries | aws.LogRequest
	}

	// Handle "off" explicitly
	if strings.ToLower(logLevel) == "off" {
		return 0 // LogOff equivalent
	}

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

	// If no valid log levels were specified, default to retries and request
	if mode == 0 && strings.ToLower(logLevel) != "off" {
		return aws.LogRetries | aws.LogRequest
	}

	return mode
}

// NewConfig enhances newConfig by adding support for assuming a role
// not specified in the AWS profile.
// The third parameter is the ARN of the role to assume.
//
// Both the config creation and the assumed role credentials use the
// specified region and the profile.
//
// If we need to use separate regions and profiles for each config,
// we might need to enhance this function further.
// That's another story though...
func NewConfig(ctx context.Context, region string, profile string, roleARN string) (aws.Config, error) {
	cfg, err := newConfig(ctx, region, profile)
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
func newConfig(ctx context.Context, region string, profile string) (aws.Config, error) {
	var opts []func(*config.LoadOptions) error

	// Set region if provided
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}

	// Handle profile selection
	switch {
	case profile != "":
		opts = append(opts, config.WithSharedConfigProfile(profile))
	case os.Getenv("FORCE_AWS_PROFILE") == "true":
		if awsProfile := os.Getenv("AWS_PROFILE"); awsProfile != "" {
			opts = append(opts, config.WithSharedConfigProfile(awsProfile))
		}
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

	// Configure client log mode based on AWS_SDK_GO_LOG_LEVEL environment variable
	opts = append(opts, config.WithClientLogMode(parseAWSLogLevel()))

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, err
	}

	return cfg, nil
}

// NewSession provides backwards compatibility for existing code
// Deprecated: Use NewConfig instead
func NewSession(region string, profile string, roleARN string) aws.Config {
	ctx := context.Background()
	cfg, err := NewConfig(ctx, region, profile, roleARN)
	if err != nil {
		panic(err) // This matches the old session.Must behavior
	}
	return cfg
}
