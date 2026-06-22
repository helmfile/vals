//go:build onepassword || all_providers || !custom_providers

package stringprovider

import (
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
	"github.com/helmfile/vals/pkg/providers/onepassword"
	"github.com/helmfile/vals/pkg/providers/onepasswordconnect"
	"github.com/helmfile/vals/pkg/providers/registry"
)

func init() {
	registry.RegisterStringProvider("onepassword", func(_ *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringProvider, error) {
		return onepassword.New(provider), nil
	})
	registry.RegisterStringProvider("onepasswordconnect", func(_ *log.Logger, provider api.StaticConfig, _ string) (api.LazyLoadedStringProvider, error) {
		return onepasswordconnect.New(provider), nil
	})
}
