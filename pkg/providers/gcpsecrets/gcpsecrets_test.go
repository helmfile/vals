package gcpsecrets

import (
	"testing"

	"github.com/variantdev/vals"
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
			config := vals.Map(tt.options)

		})
	}
}
