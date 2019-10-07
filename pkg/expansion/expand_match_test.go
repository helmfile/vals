package expansion

import (
	"net/url"
	"reflect"
	"regexp"
	"testing"
)

func TestExpandRegexpMatchInString(t *testing.T) {
	testcases := []struct {
		name     string
		regex    *regexp.Regexp
		input    string
		expected string
	}{
		{
			name:     "basic",
			regex:    DefaultRefRegexp,
			input:    "ref+vault://srv/foo/bar",
			expected: "vault-srv-/foo/bar",
		},
	}

	for i := range testcases {
		tc := testcases[i]

		t.Run(tc.name, func(t *testing.T) {
			lookup := func(m string) (string, error) {
				parsed, err := url.Parse(m)
				if err != nil {
					return "", err
				}

				return parsed.Scheme + "-" + parsed.Host + "-" + parsed.Path, nil
			}

			expand := ExpandRegexMatch{
				Target: tc.regex,
				Lookup: lookup,
			}

			actual, err := expand.InString(tc.input)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(tc.expected, actual) {
				t.Errorf("unexpected result: expected:\n%v\ngot:%v\n", tc.expected, actual)
			}
		})
	}
}

func TestExpandRegexpMatchInMap(t *testing.T) {
	testcases := []struct {
		name     string
		regex    *regexp.Regexp
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name:     "string",
			regex:    DefaultRefRegexp,
			input:    map[string]interface{}{"k": "ref+vault://srv/foo/bar"},
			expected: map[string]interface{}{"k": "vault-srv-/foo/bar"},
		},
		{
			name:     "string-slice",
			regex:    DefaultRefRegexp,
			input:    map[string]interface{}{"k": []string{"ref+vault://srv/foo/bar"}},
			expected: map[string]interface{}{"k": []interface{}{"vault-srv-/foo/bar"}},
		},
		{
			name:     "interface-slice",
			regex:    DefaultRefRegexp,
			input:    map[string]interface{}{"k": []interface{}{"ref+vault://srv/foo/bar"}},
			expected: map[string]interface{}{"k": []interface{}{"vault-srv-/foo/bar"}},
		},
		{
			name:     "interface-slice-in-interface-map",
			regex:    DefaultRefRegexp,
			input:    map[string]interface{}{"k": map[interface{}]interface{}{"k2": []interface{}{"ref+vault://srv/foo/bar"}}},
			expected: map[string]interface{}{"k": map[string]interface{}{"k2": []interface{}{"vault-srv-/foo/bar"}}},
		},
		{
			name:     "interface-slice-in-string-map",
			regex:    DefaultRefRegexp,
			input:    map[string]interface{}{"k": map[string]interface{}{"k2": []interface{}{"ref+vault://srv/foo/bar"}}},
			expected: map[string]interface{}{"k": map[string]interface{}{"k2": []interface{}{"vault-srv-/foo/bar"}}},
		},
		{
			name:     "string-in-interface-map",
			regex:    DefaultRefRegexp,
			input:    map[string]interface{}{"k": map[interface{}]interface{}{"k2": "ref+vault://srv/foo/bar"}},
			expected: map[string]interface{}{"k": map[string]interface{}{"k2": "vault-srv-/foo/bar"}},
		},
		{
			name:     "string-in-string-map",
			regex:    DefaultRefRegexp,
			input:    map[string]interface{}{"k": map[string]interface{}{"k2": "ref+vault://srv/foo/bar"}},
			expected: map[string]interface{}{"k": map[string]interface{}{"k2": "vault-srv-/foo/bar"}},
		},
	}

	for i := range testcases {
		tc := testcases[i]

		t.Run(tc.name, func(t *testing.T) {
			lookup := func(m string) (string, error) {
				parsed, err := url.Parse(m)
				if err != nil {
					return "", err
				}

				return parsed.Scheme + "-" + parsed.Host + "-" + parsed.Path, nil
			}

			expand := ExpandRegexMatch{
				Target: tc.regex,
				Lookup: lookup,
			}

			actual, err := expand.InMap(tc.input)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(tc.expected, actual) {
				t.Errorf("unexpected result: expected:\n%v\ngot:%v\n", tc.expected, actual)
			}
		})
	}
}
