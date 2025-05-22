package vals

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	config2 "github.com/helmfile/vals/pkg/config"
	"github.com/helmfile/vals/pkg/providers/httpjson"
)

const HttpJsonPrefix = "httpjson://"

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
	prefixAndPath := fmt.Sprintf("httpjson://%v", serverURLWithoutProtocol)

	t.Run("Get name from first array item", func(t *testing.T) {
		config := createProvider(prefixAndPath+"?insecure=true#", "//*[1]/name", "false")
		vals, err := Load(config)
		if err != nil {
			t.Fatalf("%v", err)
		}
		expected := "chartify"
		actual := vals["value"]
		if actual != expected {
			t.Errorf("unexpected value for key %q: expected=%q, got=%q", "value", expected, actual)
		}
	})

	t.Run("Get name from second array item", func(t *testing.T) {
		config := createProvider(prefixAndPath+"?insecure=true#", "//*[2]/name", "false")
		vals, err := Load(config)
		if err != nil {
			t.Fatalf("%v", err)
		}
		expected := "go-yaml"
		actual := vals["value"]
		if actual != expected {
			t.Errorf("unexpected value for key %q: expected=%q, got=%q", "value", expected, actual)
		}
	})

	t.Run("Error getting document from location jsonquery.LoadURL", func(t *testing.T) {
		config := createProvider("httpjson://boom.github.com/users/helmfile/repos?insecure=true#", "//owner", "false")
		_, err := Load(config)
		if err != nil {
			expected := "error fetching json document at http://boom.github.com/users/helmfile/repos: Get \"http://boom.github.com/users/helmfile/repos\": dial tcp: lookup boom.github.com: no such host"
			actual := err.Error()
			if actual != expected {
				t.Errorf("unexpected value for key %q: expected=%q, got=%q", "value", expected, actual)
			}
		}
	})

	t.Run("Error running json.Query", func(t *testing.T) {
		uri := prefixAndPath + "?insecure=true#"
		config := createProvider(uri, "/boom", "false")
		_, err := Load(config)
		if err != nil {
			expected := "unable to query doc for value with xpath query using " + uri + "//boom"
			actual := err.Error()
			if actual != expected {
				t.Errorf("unexpected value for key %q: expected=%q, got=%q", "value", expected, actual)
			}
		}
	})

	t.Run("Query list for comma separated string", func(t *testing.T) {
		uri := prefixAndPath + "?insecure=true&mode=singleparam#"
		config := createProvider(uri, "/boom", "false")
		_, err := Load(config)
		if err != nil {
			expected := "unable to query doc for value with xpath query using " + uri + "//boom"
			actual := err.Error()
			if actual != expected {
				t.Errorf("unexpected value for key %q: expected=%q, got=%q", "value", expected, actual)
			}
		}
	})

	t.Run("Get Avatar URL with child nodes causing error", func(t *testing.T) {
		config := createProvider(prefixAndPath+"?insecure=true#", "//owner", "false")
		_, err := Load(config)
		if err != nil {
			expected := "location //owner has child nodes at " + server.URL + ", please use a more granular query"
			actual := err.Error()
			if actual != expected {
				t.Errorf("unexpected value for key %q: expected=%q, got=%q", "value", expected, actual)
			}
		}
	})

	t.Run("Test floatAsInt Success", func(t *testing.T) {
		config := createProvider(prefixAndPath+"?insecure=true&floatAsInt=true#", "//*[1]/id", "true")
		vals, err := Load(config)
		if err != nil {
			t.Fatalf("%v", err)
		}
		expected := "251296379"
		actual := vals["value"]
		if actual != expected {
			t.Errorf("unexpected value for key %q: expected=%q, got=%q", "value", expected, actual)
		}
	})

	t.Run("Test floatAsInt failure", func(t *testing.T) {
		config := createProvider(prefixAndPath+"?insecure=true#", "//*[1]/name", "false")
		_, err := Load(config)
		if err != nil {
			expected := "unable to convert possible float to int for value: chartify"
			actual := err.Error()
			if actual != expected {
				t.Errorf("unexpected value for key %q: expected=%q, got=%q", "value", expected, actual)
			}
		}
	})

	t.Run("Test list returned as string", func(t *testing.T) {
		config := createProvider(prefixAndPath+"?insecure=true&mode=singleparam#", "//*[1]/DBNodes", "false")
		vals, err := Load(config)
		if err != nil {
			t.Fatalf("%v", err)
		}
		expected := "chartify.database1.io,chartify.database2.io,chartify.database3.io,chartify.database4.io,chartify.database5.io"
		actual := vals["value"]
		if actual != expected {
			t.Errorf("unexpected value for key %q: expected=%q, got=%q", "value", expected, actual)
		}
	})
}

func Test_HttpJson_UnitTests(t *testing.T) {
	if os.Getenv("SKIP_TESTS") != "" {
		t.Skip("Skipping tests")
	}

	// GetUrlFromUri
	t.Run("GetUrlFromUri: valid (http)", func(t *testing.T) {
		returnValue, err := httpjson.GetUrlFromUri("httpjson://boom.com/path?insecure=true#///*[1]/name", "http")
		if err != nil {
			t.Fatalf("%v", err)
		}
		expected := "http://boom.com/path"
		if returnValue != expected {
			t.Errorf("unexpected value for key %q: expected=%q, got=%q", "value", expected, returnValue)
		}
	})
	t.Run("GetUrlFromUri: valid (https)", func(t *testing.T) {
		returnValue, err := httpjson.GetUrlFromUri("httpjson://boom.com/path#///*[1]/name", "https")
		if err != nil {
			t.Fatalf("%v", err)
		}
		expected := "https://boom.com/path"
		if returnValue != expected {
			t.Errorf("unexpected value for key %q: expected=%q, got=%q", "value", expected, returnValue)
		}
	})
	t.Run("GetUrlFromUri: invalid character in host name (https)", func(t *testing.T) {
		_, err := httpjson.GetUrlFromUri("httpjson://supsupsup^boom#///*[1]/name", "https")
		expected := "invalid URL: https://supsupsup^boom, error: parse \"https://supsupsup^boom\": invalid character \"^\" in host name"

		if err == nil {
			t.Fatalf("expected an error %q, got nil", expected)
		}

		actual := err.Error()
		if actual != expected {
			t.Errorf("unexpected value for key %q: expected=%q, got=%q", "value", expected, actual)
		}
	})
	t.Run("GetUrlFromUri: no domain provided (http)", func(t *testing.T) {
		_, err := httpjson.GetUrlFromUri("httpjson://?insecure=true#///*[1]/name", "http")

		expected := "no domain found in uri: httpjson://?insecure=true#///*[1]/name"

		if err == nil {
			t.Fatalf("expected an error %q, got nil", expected)
		}

		actual := err.Error()
		if actual != expected {
			t.Errorf("unexpected value for key %q: expected=%q, got=%q", "value", expected, actual)
		}
	})
	t.Run("GetUrlFromUri: no domain provided (https)", func(t *testing.T) {
		_, err := httpjson.GetUrlFromUri("httpjson://#///*[1]/name", "https")

		expected := "no domain found in uri: httpjson://#///*[1]/name"

		if err == nil {
			t.Fatalf("expected an error %q, got nil", expected)
		}

		actual := err.Error()
		if actual != expected {
			t.Errorf("unexpected value for key %q: expected=%q, got=%q", "value", expected, actual)
		}
	})
	t.Run("GetUrlFromUri: query params are preserved", func(t *testing.T) {
		url, _ := httpjson.GetUrlFromUri("httpjson://domain.com/path/?param=value#///*[1]/name", "https")

		expected := "https://domain.com/path/?param=value"
		if expected != url {
			t.Errorf("unexpected url: expected=%q, got=%q", expected, url)
		}
	})
	t.Run("GetUrlFromUri: special query params are removed", func(t *testing.T) {
		url, _ := httpjson.GetUrlFromUri("httpjson://domain.com/path/?insecure=true&floatAsInt=false&param=value#///*[1]/name", "http")

		expected := "http://domain.com/path/?param=value"
		if expected != url {
			t.Errorf("unexpected url: expected=%q, got=%q", expected, url)
		}
	})

	// GetXpathFromUri
	t.Run("GetXpathFromUri: valid (http)", func(t *testing.T) {
		returnValue, err := httpjson.GetXpathFromUri("httpjson://blah.blah/blah?insecure=true#///*[1]/name")
		if err != nil {
			t.Fatalf("%v", err)
		}
		expected := "//*[1]/name"
		if returnValue != expected {
			t.Errorf("unexpected value for key %q: expected=%q, got=%q", "value", expected, returnValue)
		}
	})
	t.Run("GetXpathFromUri: valid (https)", func(t *testing.T) {
		returnValue, err := httpjson.GetXpathFromUri("httpjson://blah.blah/blah#///*[1]/name")
		if err != nil {
			t.Fatalf("%v", err)
		}
		expected := "//*[1]/name"
		if returnValue != expected {
			t.Errorf("unexpected value for key %q: expected=%q, got=%q", "value", expected, returnValue)
		}
	})
	t.Run("GetXpathFromUri: no xpath provided (http)", func(t *testing.T) {
		_, err := httpjson.GetXpathFromUri("httpjson://blah.blah/blah?insecure=true")
		if err != nil {
			expected := "no xpath expression found in uri: httpjson://blah.blah/blah?insecure=true"
			actual := err.Error()
			if actual != expected {
				t.Errorf("unexpected value for key %q: expected=%q, got=%q", "value", expected, actual)
			}
		}
	})
	t.Run("GetXpathFromUri: no xpath provided (https)", func(t *testing.T) {
		_, err := httpjson.GetXpathFromUri("httpjson://blah.blah/blah")
		if err != nil {
			expected := "no xpath expression found in uri: httpjson://blah.blah/blah"
			actual := err.Error()
			if actual != expected {
				t.Errorf("unexpected value for key %q: expected=%q, got=%q", "value", expected, actual)
			}
		}
	})
	t.Run("GetXpathFromUri: invalid xpath 1 (http)", func(t *testing.T) {
		_, err := httpjson.GetXpathFromUri("httpjson://blah.blah/blah?insecure=true#/")
		if err != nil {
			expected := "unable to compile xpath expression '' from uri: httpjson://blah.blah/blah?insecure=true#/"
			actual := err.Error()
			if actual != expected {
				t.Errorf("unexpected value for key %q: expected=%q, got=%q", "value", expected, actual)
			}
		}
	})
	t.Run("GetXpathFromUri: invalid xpath 1 (https)", func(t *testing.T) {
		_, err := httpjson.GetXpathFromUri("httpjson://blah.blah/blah#/")
		if err != nil {
			expected := "unable to compile xpath expression '' from uri: httpjson://blah.blah/blah#/"
			actual := err.Error()
			if actual != expected {
				t.Errorf("unexpected value for key %q: expected=%q, got=%q", "value", expected, actual)
			}
		}
	})
	t.Run("GetXpathFromUri: invalid xpath 2 (http)", func(t *testing.T) {
		_, err := httpjson.GetXpathFromUri("httpjson://blah.blah/blah?insecure=true#/hello^sup")
		if err != nil {
			expected := "unable to compile xpath expression '' from uri: httpjson://blah.blah/blah?insecure=true#/hello^sup"
			actual := err.Error()
			if actual != expected {
				t.Errorf("unexpected value for key %q: expected=%q, got=%q", "value", expected, actual)
			}
		}
	})
	t.Run("GetXpathFromUri: invalid xpath 2 (https)", func(t *testing.T) {
		_, err := httpjson.GetXpathFromUri("httpjson://blah.blah/blah#/hello^sup")
		if err != nil {
			expected := "unable to compile xpath expression '' from uri: httpjson://blah.blah/blah#/hello^sup"
			actual := err.Error()
			if actual != expected {
				t.Errorf("unexpected value for key %q: expected=%q, got=%q", "value", expected, actual)
			}
		}
	})
}
