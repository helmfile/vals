//go:build conjur || all_providers || !custom_providers

package stringprovider

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/conjur"
	"github.com/helmfile/vals/pkg/providers/registry"
)

func init() {
	registry.RegisterStringProvider("conjur", func(l *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringProvider, error) {
		return conjur.New(l, provider), nil
	})
}
