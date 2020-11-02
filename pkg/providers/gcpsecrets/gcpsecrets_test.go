package gcpsecrets

import (
	config2 "github.com/variantdev/vals/pkg/config"
	"testing"
)

func Test_New(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]interface{}
		want    provider
	}{
		{"latest", map[string]interface{}{"version": "latest"}, provider{version: "latest"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = config2.Map(tt.options)
		})
	}
}
