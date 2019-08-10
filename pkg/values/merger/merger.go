package merger

import (
	"fmt"
	"github.com/mumoshu/values/pkg/values/api"
	"github.com/mumoshu/values/pkg/values/providers/sprucemerge"
)

func New(tpe string, provider api.StaticConfig) (api.Merger, error) {
	switch tpe {
	case "spruce":
		return sprucemerge.New(provider), nil
	}

	return nil, fmt.Errorf("failed initializing merger from config: %v", provider)
}
