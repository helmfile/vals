package registry

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
)

// ProviderFactory creates a provider for use in vals.go createProvider.
type ProviderFactory func(l *log.Logger, conf config.MapConfig, awsLogLevel string) (api.Provider, error)

// StringProviderFactory creates a string provider for use in stringprovider.New.
type StringProviderFactory func(l *log.Logger, provider api.StaticConfig, awsLogLevel string) (api.LazyLoadedStringProvider, error)

// StringMapProviderFactory creates a string-map provider for use in stringmapprovider.New.
type StringMapProviderFactory func(l *log.Logger, provider api.StaticConfig, awsLogLevel string) (api.LazyLoadedStringMapProvider, error)

var (
	providers          = map[string]ProviderFactory{}
	stringProviders    = map[string]StringProviderFactory{}
	stringMapProviders = map[string]StringMapProviderFactory{}
)

func RegisterProvider(scheme string, f ProviderFactory) {
	providers[scheme] = f
}

func GetProvider(scheme string) (ProviderFactory, bool) {
	f, ok := providers[scheme]
	return f, ok
}

func RegisterStringProvider(name string, f StringProviderFactory) {
	stringProviders[name] = f
}

func GetStringProvider(name string) (StringProviderFactory, bool) {
	f, ok := stringProviders[name]
	return f, ok
}

func RegisterStringMapProvider(name string, f StringMapProviderFactory) {
	stringMapProviders[name] = f
}

func GetStringMapProvider(name string) (StringMapProviderFactory, bool) {
	f, ok := stringMapProviders[name]
	return f, ok
}
