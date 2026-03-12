//go:build aws || all_providers || !custom_providers

package vals

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/awskms"
	"github.com/helmfile/vals/pkg/providers/awssecrets"
	"github.com/helmfile/vals/pkg/providers/registry"
	"github.com/helmfile/vals/pkg/providers/s3"
	"github.com/helmfile/vals/pkg/providers/ssm"
)

func init() {
	// vals.go createProvider
	registry.RegisterProvider(ProviderS3, func(l *log.Logger, conf config.MapConfig, awsLogLevel string) (api.Provider, error) {
		return s3.New(l, conf, awsLogLevel), nil
	})
	registry.RegisterProvider(ProviderSSM, func(l *log.Logger, conf config.MapConfig, awsLogLevel string) (api.Provider, error) {
		return ssm.New(l, conf, awsLogLevel), nil
	})
	registry.RegisterProvider(ProviderSecretsManager, func(l *log.Logger, conf config.MapConfig, awsLogLevel string) (api.Provider, error) {
		return awssecrets.New(l, conf, awsLogLevel), nil
	})
	registry.RegisterProvider(ProviderKms, func(_ *log.Logger, conf config.MapConfig, awsLogLevel string) (api.Provider, error) {
		return awskms.New(conf, awsLogLevel), nil
	})

	// stringprovider
	registry.RegisterStringProvider("s3", func(l *log.Logger, provider api.StaticConfig, awsLogLevel string) (api.LazyLoadedStringProvider, error) {
		return s3.New(l, provider, awsLogLevel), nil
	})
	registry.RegisterStringProvider("ssm", func(l *log.Logger, provider api.StaticConfig, awsLogLevel string) (api.LazyLoadedStringProvider, error) {
		return ssm.New(l, provider, awsLogLevel), nil
	})
	registry.RegisterStringProvider("awssecrets", func(l *log.Logger, provider api.StaticConfig, awsLogLevel string) (api.LazyLoadedStringProvider, error) {
		return awssecrets.New(l, provider, awsLogLevel), nil
	})
	registry.RegisterStringProvider("awskms", func(_ *log.Logger, provider api.StaticConfig, awsLogLevel string) (api.LazyLoadedStringProvider, error) {
		return awskms.New(provider, awsLogLevel), nil
	})

	// stringmapprovider
	registry.RegisterStringMapProvider("s3", func(l *log.Logger, provider api.StaticConfig, awsLogLevel string) (api.LazyLoadedStringMapProvider, error) {
		return ssm.New(l, provider, awsLogLevel), nil
	})
	registry.RegisterStringMapProvider("ssm", func(l *log.Logger, provider api.StaticConfig, awsLogLevel string) (api.LazyLoadedStringMapProvider, error) {
		return ssm.New(l, provider, awsLogLevel), nil
	})
	registry.RegisterStringMapProvider("awssecrets", func(l *log.Logger, provider api.StaticConfig, awsLogLevel string) (api.LazyLoadedStringMapProvider, error) {
		return awssecrets.New(l, provider, awsLogLevel), nil
	})
	registry.RegisterStringMapProvider("awskms", func(_ *log.Logger, provider api.StaticConfig, awsLogLevel string) (api.LazyLoadedStringMapProvider, error) {
		return awskms.New(provider, awsLogLevel), nil
	})
}
