//go:build gitlab || all_providers || !custom_providers

package stringprovider

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/gitlab"
	"github.com/helmfile/vals/pkg/providers/registry"
)

func init() {
	registry.RegisterStringProvider("gitlab", func(_ *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringProvider, error) {
		return gitlab.New(provider), nil
	})
}
