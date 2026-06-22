package stringmapprovider

import (
	"fmt"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
	execprovider "github.com/helmfile/vals/pkg/providers/exec"
	"github.com/helmfile/vals/pkg/providers/registry"
)

func New(l *log.Logger, provider api.StaticConfig, awsLogLevel string) (api.LazyLoadedStringMapProvider, error) {
	tpe := provider.String("name")

	switch tpe {
	case "exec":
		return execprovider.New(l, provider), nil
	default:
		if factory, ok := registry.GetStringMapProvider(tpe); ok {
			return factory(l, provider, awsLogLevel)
		}
	}

	return nil, fmt.Errorf("failed initializing string-map provider from config: %v", provider)
}
