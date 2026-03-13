//go:build aws || all_providers || !custom_providers

package stringmapprovider

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/awskms"
	"github.com/helmfile/vals/pkg/providers/awssecrets"
	"github.com/helmfile/vals/pkg/providers/registry"
	"github.com/helmfile/vals/pkg/providers/ssm"
)

func init() {
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
