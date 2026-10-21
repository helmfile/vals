//go:build aws || all_providers || !custom_providers

package stringprovider

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/awskms"
	"github.com/helmfile/vals/pkg/providers/awssecrets"
	"github.com/helmfile/vals/pkg/providers/registry"
	"github.com/helmfile/vals/pkg/providers/s3"
	"github.com/helmfile/vals/pkg/providers/ssm"
)

func init() {
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
}
