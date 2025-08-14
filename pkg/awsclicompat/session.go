package awsclicompat

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// NewSession enhances newSession by adding support for assuming a role
// not specified in the AWS profile.
// The third parameter is the ARN of the role to assume.
//
// Both the session calls the STS API to assume the role
// and the session uses the assumed role credentials use the
// specified region and the profile.
//
// If we need to use separate regions and profiles for each session,
// we might need to enhance this function further.
// That's another story though...
func NewSession(region string, profile string, roleARN string) aws.Config {
	cfg := newConfig(region, profile)

	if roleARN != "" {
		// Create an STS client using the base config
		stsClient := sts.NewFromConfig(cfg)
		// Create credentials that assume the role
		cfg.Credentials = stscreds.NewAssumeRoleProvider(stsClient, roleARN)
	}

	return cfg
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
func newConfig(region string, profile string) aws.Config {
	configOptions := []func(*config.LoadOptions) error{}

	if region != "" {
		configOptions = append(configOptions, config.WithRegion(region))
	}

	if profile != "" {
		configOptions = append(configOptions, config.WithSharedConfigProfile(profile))
	} else if os.Getenv("FORCE_AWS_PROFILE") == "true" {
		awsProfile := os.Getenv("AWS_PROFILE")
		if awsProfile != "" {
			configOptions = append(configOptions, config.WithSharedConfigProfile(awsProfile))
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
	endpointUrl := os.Getenv("AWS_ENDPOINT_URL")
	if endpointUrl != "" {
		configOptions = append(configOptions, config.WithCustomCABundle(nil))
		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL: endpointUrl,
			}, nil
		})
		configOptions = append(configOptions, config.WithEndpointResolverWithOptions(customResolver))
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(), configOptions...)
	if err != nil {
		// In v1, session.Must was used which panics on error
		// We maintain similar behavior here
		panic(err)
	}

	return cfg
}
