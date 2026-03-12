//go:build gitlab || all_providers || !custom_providers

package vals

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/gitlab"
	"github.com/helmfile/vals/pkg/providers/registry"
)

func init() {
	registry.RegisterProvider(ProviderGitLab, func(_ *log.Logger, conf config.MapConfig, _ string) (api.Provider, error) {
		return gitlab.New(conf), nil
	})
	registry.RegisterStringProvider("gitlab", func(_ *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringProvider, error) {
		return gitlab.New(provider), nil
	})
}
