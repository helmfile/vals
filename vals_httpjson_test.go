package vals

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	config2 "github.com/helmfile/vals/pkg/config"
)

var server *httptest.Server

func setup() {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Define the JSON data
		data := []map[string]interface{}{
			{
				"name": "chartify",
				"id":   251296379,
				"status": map[string]interface{}{
					"database": map[string]interface{}{
						"DBNodes": []string{
							"chartify.database1.io",
							"chartify.database2.io",
							"chartify.database3.io",
							"chartify.database4.io",
							"chartify.database5.io",
						},
					},
				},
				"owner": map[string]interface{}{
					"login": "helmfile",
					"id":    8319146,
				},
			},
			{
				"name": "go-yaml",
				"id":   597918420,
				"status": map[string]interface{}{
					"database": map[string]interface{}{
						"DBNodes": []string{
							"go-yaml.database1.io",
							"go-yaml.database2.io",
							"go-yaml.database3.io",
							"go-yaml.database4.io",
							"go-yaml.database5.io",
						},
					},
				},
				"owner": map[string]interface{}{
					"login": "helmfile",
					"id":    83191469,
				},
			},
		}

		// Encode the JSON data
		jsonData, err := json.Marshal(data)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Set the Content-Type header
		w.Header().Set("Content-Type", "application/json")

		// Write the JSON response
		w.Write(jsonData)
	})

	// Create a test server (using any free port)
	server = httptest.NewServer(handler)
}

func teardown() {
	// Close the test server
	server.Close()
}

func createProvider(providerPath string, inlineValue string, floatAsInt string) config2.MapConfig {
	// Construct the configuration map with the provided values
	config := map[string]interface{}{
		"provider": map[string]interface{}{
			"name":       "httpjson",
			"path":       providerPath,
			"floatAsInt": floatAsInt,
			"insecure":   "true",
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

	// Initialize a web server to serve JSON data for testing purposes
	setup()

	// Teardown web server once testing is complete
	defer teardown()

	// Get the server URL without the protocol
	serverURLWithoutProtocol := strings.TrimPrefix(server.URL, "http://")

	t.Run("Get name from first array item", func(t *testing.T) {
		config := createProvider("httpjson://"+serverURLWithoutProtocol+"?insecure=true&mode=singleparam#", "//*[1]/name", "false")
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
		config := createProvider("httpjson://"+serverURLWithoutProtocol+"?insecure=true&mode=singleparam#", "//*[2]/name", "false")
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
		config := createProvider("httpjson://boom.github.com/users/helmfile/repos?insecure=true&mode=singleparam#", "//owner", "false")
		_, err := Load(config)
		if err != nil {
			expected := "error fetching json document at http://boom.github.com/users/helmfile/repos: invalid character '<' looking for beginning of value"
			actual := err.Error()
			if actual != expected {
				t.Errorf("unepected value for key %q: expected=%q, got=%q", "value", expected, actual)
			}
		}
	})

	t.Run("Error running json.Query", func(t *testing.T) {
		uri := "httpjson://" + serverURLWithoutProtocol + "?insecure=true&mode=singleparam#"
		config := createProvider(uri, "/boom", "false")
		_, err := Load(config)
		if err != nil {
			expected := "unable to query doc for value with xpath query using " + uri + "//boom"
			actual := err.Error()
			if actual != expected {
				t.Errorf("unepected value for key %q: expected=%q, got=%q", "value", expected, actual)
			}
		}
	})

	t.Run("Query list for comma separated string", func(t *testing.T) {
		uri := "httpjson://" + serverURLWithoutProtocol + "?insecure=true&mode=singleparam#"
		config := createProvider(uri, "/boom", "false")
		_, err := Load(config)
		if err != nil {
			expected := "unable to query doc for value with xpath query using " + uri + "//boom"
			actual := err.Error()
			if actual != expected {
				t.Errorf("unepected value for key %q: expected=%q, got=%q", "value", expected, actual)
			}
		}
	})

	t.Run("Get Avatar URL with child nodes causing error", func(t *testing.T) {
		config := createProvider("httpjson://"+serverURLWithoutProtocol+"?insecure=true&mode=singleparam#", "//owner", "false")
		_, err := Load(config)
		if err != nil {
			expected := "location //owner has child nodes at " + server.URL + ", please use a more granular query"
			actual := err.Error()
			if actual != expected {
				t.Errorf("unepected value for key %q: expected=%q, got=%q", "value", expected, actual)
			}
		}
	})

	t.Run("Test floatAsInt Success", func(t *testing.T) {
		config := createProvider("httpjson://"+serverURLWithoutProtocol+"?insecure=true&floatAsInt=true&mode=singleparam#", "//*[1]/id", "true")
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
		config := createProvider("httpjson://"+serverURLWithoutProtocol+"?insecure=true&mode=singleparam#", "//*[1]/name", "false")
		_, err := Load(config)
		if err != nil {
			expected := "unable to convert possible float to int for value: chartify"
			actual := err.Error()
			if actual != expected {
				t.Errorf("unepected value for key %q: expected=%q, got=%q", "value", expected, actual)
			}
		}
	})

	t.Run("Test list returned as string", func(t *testing.T) {
		config := createProvider("httpjson://"+serverURLWithoutProtocol+"?insecure=true&mode=singleparam#", "//*[1]/DBNodes", "false")
		vals, err := Load(config)
		if err != nil {
			t.Fatalf("%v", err)
		}
		expected := "chartify.database1.io,chartify.database2.io,chartify.database3.io,chartify.database4.io,chartify.database5.io"
		actual := vals["value"]
		if actual != expected {
			t.Errorf("unepected value for key %q: expected=%q, got=%q", "value", expected, actual)
		}
	})
}
