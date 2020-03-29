package vals

import (
	"os"
	"reflect"
	"testing"
)

// setup:
// echo -n "foo: bar" | gcloud secrets create valstestvar --data-file=- --replication-policy=automatic
// GCP_PROJECT=secret-test-99234 go test -run '^(TestValues_GCPSecretsManager)$'
func TestValues_GCPSecretsManager(t *testing.T) {
	projectId := os.Getenv("GCP_PROJECT")
	if projectId == "" {
		t.Fatalf("gcpsecrets tests require GCP_PROJECT env var set correctly")
	}
	tests := []struct {
		name    string
		secrets map[string]string
		config  map[string]interface{}
		want    map[string]interface{}
	}{
		{
			"latest string",
			map[string]string{"valstestvar": "foo: bar"},
			map[string]interface{}{
				"provider": map[string]interface{}{
					"name":    "gcpsecrets",
					"version": "latest",
					"type":    "string",
					"path":    projectId,
				},
				"inline": map[string]interface{}{
					"valstestvar": "valstestvar",
				},
			},
			map[string]interface{}{"valstestvar": "foo: bar"},
		},
		{
			"v1 string",
			map[string]string{"valstestvar": "foo: bar"},
			map[string]interface{}{
				"provider": map[string]interface{}{
					"name":    "gcpsecrets",
					"version": "1",
					"type":    "string",
					"path":    projectId,
				},
				"inline": map[string]interface{}{
					"valstestvar": "valstestvar",
				},
			},
			map[string]interface{}{"valstestvar": "foo: bar"},
		},
		{
			"v1 map",
			map[string]string{"valstestvar": "foo: bar"},
			map[string]interface{}{
				"provider": map[string]interface{}{
					"name":    "gcpsecrets",
					"version": "1",
					"type":    "map",
					"path":    projectId,
				},
				"inline": map[string]interface{}{
					"valstestvar": "valstestvar",
				},
			},
			map[string]interface{}{
				"valstestvar": map[string]interface{}{
					"foo": "bar",
				},
			},
		},
		{
			"latest map",
			map[string]string{"valstestvar": "foo: bar"},
			map[string]interface{}{
				"provider": map[string]interface{}{
					"name":    "gcpsecrets",
					"version": "latest",
					"type":    "map",
					"path":    projectId,
				},
				"inline": map[string]interface{}{
					"valstestvar": "valstestvar",
				},
			},
			map[string]interface{}{
				"valstestvar": map[string]interface{}{
					"foo": "bar",
				},
			},
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			mapConfig := Map(tt.config)
			vals, err := Load(mapConfig)
			if err != nil {
				t.Fatalf("%v", err)
			}

			if !reflect.DeepEqual(vals, tt.want) {
				t.Errorf("unexpected value for vals: want='%s', got='%s'", tt.want, vals)
			}
		})
	}
}
