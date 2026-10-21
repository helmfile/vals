//go:build doppler || all_providers || !custom_providers

package stringprovider

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/doppler"
	"github.com/helmfile/vals/pkg/providers/registry"
)

func init() {
	registry.RegisterStringProvider("doppler", func(l *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringProvider, error) {
		return doppler.New(l, provider), nil
	})
}
