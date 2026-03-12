package registry

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
)

// ProviderFactory creates a provider given a logger and configuration.
type ProviderFactory func(l *log.Logger, conf config.MapConfig) (api.Provider, error)

var providers = map[string]ProviderFactory{}

// Register adds a provider factory under the given scheme name.
// It is intended to be called from init() functions in build-tag-guarded files.
func Register(scheme string, factory ProviderFactory) {
	providers[scheme] = factory
}

// Get returns the provider factory for the given scheme, if registered.
func Get(scheme string) (ProviderFactory, bool) {
	f, ok := providers[scheme]
	return f, ok
}
