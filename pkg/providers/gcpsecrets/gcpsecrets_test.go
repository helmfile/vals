package gcpsecrets

import (
	"testing"

	config2 "github.com/helmfile/vals/pkg/config"
)

func Test_New(t *testing.T) {
	defaultVal := "default-value"

	tests := []struct {
		name    string
		options map[string]interface{}
		want    provider
	}{
		{"latest", map[string]interface{}{"version": "latest"}, provider{version: "latest", optional: false}},
		{"optional", map[string]interface{}{"version": "latest", "optional": true}, provider{version: "latest", optional: true}},
		{"latest", map[string]interface{}{"version": "latest"}, provider{version: "latest", fallback: nil}},
		{"fallback", map[string]interface{}{"version": "latest", "fallback_value": defaultVal}, provider{version: "latest", fallback: &defaultVal}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = config2.Map(tt.options)
		})
	}
}
