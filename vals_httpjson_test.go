package vals

import (
	"os"
	"testing"

	config2 "github.com/helmfile/vals/pkg/config"
)

func createProvider(providerPath string, inlineValue string, floatAsInt string) config2.MapConfig {
	config := map[string]interface{}{
		"provider": map[string]interface{}{
			"name":       "httpjson",
			"path":       providerPath,
			"floatAsInt": floatAsInt,
		},
		"inline": map[string]interface{}{
			"value": inlineValue,
		},
	}
	return config2.Map(config)
}

// nolint
func Test_HttpJson(t *testing.T) {
	if os.Getenv("SKIP_TESTS") != "" {
		t.Skip("Skipping tests")
	}

	t.Run("Get name from first array item", func(t *testing.T) {
		config := createProvider("httpjson://api.github.com/users/helmfile/repos?mode=singleparam#", "//*[1]/name", "false")
		vals, err := Load(config)
		if err != nil {
			t.Fatalf("%v", err)
		}
		expected := "chartify"
		actual := vals["value"]
		if actual != expected {
			t.Errorf("unepected value for key %q: expected=%q, got=%q", "value", expected, actual)
		}
	})

	t.Run("Get name from second array item", func(t *testing.T) {
		config := createProvider("httpjson://api.github.com/users/helmfile/repos?mode=singleparam#", "//*[2]/name", "false")
		vals, err := Load(config)
		if err != nil {
			t.Fatalf("%v", err)
		}
		expected := "go-yaml"
		actual := vals["value"]
		if actual != expected {
			t.Errorf("unepected value for key %q: expected=%q, got=%q", "value", expected, actual)
		}
	})

	t.Run("Error getting document from location jsonquery.LoadURL", func(t *testing.T) {
		config := createProvider("httpjson://boom.github.com/users/helmfile/repos?mode=singleparam#", "//owner", "false")
		_, err := Load(config)
		if err != nil {
			expected := "error fetching json document at https://boom.github.com/users/helmfile/repos: invalid character '<' looking for beginning of value"
			actual := err.Error()
			if actual != expected {
				t.Errorf("unepected value for key %q: expected=%q, got=%q", "value", expected, actual)
			}
		}
	})

	t.Run("Error running json.Query", func(t *testing.T) {
		config := createProvider("httpjson://api.github.com/users/helmfile/repos?mode=singleparam#", "/boom", "false")
		_, err := Load(config)
		if err != nil {
			expected := "unable to query doc for value with xpath query using httpjson://api.github.com/users/helmfile/repos?mode=singleparam#//boom"
			actual := err.Error()
			if actual != expected {
				t.Errorf("unepected value for key %q: expected=%q, got=%q", "value", expected, actual)
			}
		}
	})

	t.Run("Get Avatar URL with child nodes causing error", func(t *testing.T) {
		config := createProvider("httpjson://api.github.com/users/helmfile/repos?mode=singleparam#", "//owner", "false")
		_, err := Load(config)
		if err != nil {
			expected := "location //owner has child nodes at https://api.github.com/users/helmfile/repos, please use a more granular query"
			actual := err.Error()
			if actual != expected {
				t.Errorf("unepected value for key %q: expected=%q, got=%q", "value", expected, actual)
			}
		}
	})

	t.Run("Test floatAsInt Success", func(t *testing.T) {
		config := createProvider("httpjson://api.github.com/users/helmfile/repos?mode=singleparam#", "//*[1]/id", "true")
		vals, err := Load(config)
		if err != nil {
			t.Fatalf("%v", err)
		}
		expected := "251296379"
		actual := vals["value"]
		if actual != expected {
			t.Errorf("unepected value for key %q: expected=%q, got=%q", "value", expected, actual)
		}
	})

	t.Run("Test floatAsInt failure", func(t *testing.T) {
		config := createProvider("httpjson://api.github.com/users/helmfile/repos?mode=singleparam#", "//*[1]/name", "true")
		_, err := Load(config)
		if err != nil {
			expected := "unable to convert possible float to int for value: chartify"
			actual := err.Error()
			if actual != expected {
				t.Errorf("unepected value for key %q: expected=%q, got=%q", "value", expected, actual)
			}
		}
	})
}
