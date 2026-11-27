package vals

import (
	"os"
	"reflect"
	"testing"

	"github.com/helmfile/vals/pkg/config"
)

// setup:
// gcloud parametermanager parameters create valstestparam --location=global --parameter-format=UNFORMATTED
// gcloud parametermanager parameters versions create v1 --parameter=valstestparam --location=global --payload-data='foo: bar'
// GCP_PROJECT=param-test-99234 go test -run '^(TestValues_GCPParameterManager)$'
func TestValues_GCPParameterManager(t *testing.T) {
	if os.Getenv("SKIP_TESTS") != "" {
		t.Skip("Skipping tests")
	}
	projectId := os.Getenv("GCP_PROJECT")
	if projectId == "" {
		t.Fatalf("gcpparams tests require GCP_PROJECT env var set correctly")
	}
	tests := []struct {
		config map[string]any
		want   map[string]any
		name   string
	}{
		{
			map[string]any{
				"provider": map[string]any{
					"name": "gcpparams",
					"type": "string",
					"path": projectId,
				},
				"inline": map[string]any{
					"valstestparam": "valstestparam",
				},
			},
			map[string]any{"valstestparam": "foo: bar"},
			"latest string",
		},
		{
			map[string]any{
				"provider": map[string]any{
					"name":     "gcpparams",
					"version":  "v1",
					"location": "global",
					"type":     "string",
					"path":     "",
				},
				"inline": map[string]any{
					"valstestparam": "valstestparam",
				},
			},
			map[string]any{"valstestparam": "foo: bar"},
			"v1 string with project from env",
		},
		{
			map[string]any{
				"provider": map[string]any{
					"name":    "gcpparams",
					"version": "v1",
					"render":  "false",
					"type":    "map",
					"path":    projectId,
				},
				"inline": map[string]any{
					"valstestparam": "valstestparam",
				},
			},
			map[string]any{
				"valstestparam": map[string]any{
					"foo": "bar",
				},
			},
			"v1 map not rendered",
		},
		{
			map[string]any{
				"provider": map[string]any{
					"name":           "gcpparams",
					"type":           "string",
					"path":           projectId,
					"fallback_value": "default-value",
				},
				"inline": map[string]any{
					"missingvar": "missingvar",
				},
			},
			map[string]any{"missingvar": "default-value"},
			"fallback string",
		},
		{
			map[string]any{
				"provider": map[string]any{
					"name":     "gcpparams",
					"type":     "string",
					"path":     projectId,
					"optional": true,
				},
				"inline": map[string]any{
					"missingvar": "",
				},
			},
			map[string]any{"missingvar": ""},
			"optional string",
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			vals, err := Load(config.Map(tt.config))
			if err != nil {
				t.Fatalf("%v", err)
			}

			if !reflect.DeepEqual(vals, tt.want) {
				t.Errorf("unexpected value for vals: want='%s', got='%s'", tt.want, vals)
			}
		})
	}
}
