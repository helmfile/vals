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
		only     []string
		input    string
		expected string
	}{
		{
			name:     "ref",
			regex:    DefaultRefRegexp,
			input:    "ref+vault://srv/foo/bar",
			expected: "vault-srv-/foo/bar",
		},
		{
			name:     "ref + only ref",
			regex:    DefaultRefRegexp,
			only:     []string{"ref"},
			input:    "ref+vault://srv/foo/bar",
			expected: "vault-srv-/foo/bar",
		},
		{
			name:     "ref + only ref and secretref",
			regex:    DefaultRefRegexp,
			only:     []string{"ref", "secretref"},
			input:    "ref+vault://srv/foo/bar",
			expected: "vault-srv-/foo/bar",
		},
		{
			name:     "secretref",
			regex:    DefaultRefRegexp,
			input:    "secretref+vault://srv/foo/bar",
			expected: "vault-srv-/foo/bar",
		},
		{
			name:     "secretref + only ref",
			regex:    DefaultRefRegexp,
			only:     []string{"ref"},
			input:    "secretref+vault://srv/foo/bar",
			expected: "secretref+vault://srv/foo/bar",
		},
		{
			name:     "secretref",
			regex:    DefaultRefRegexp,
			only:     []string{"ref", "secretref"},
			input:    "secretref+vault://srv/foo/bar",
			expected: "vault-srv-/foo/bar",
		},
		{
			// two or more refs
			name:     "multi refs",
			regex:    DefaultRefRegexp,
			only:     []string{"ref", "secretref"},
			input:    "secretref+vault://srv/foo/bar+, secretref+vault://srv/foo/bar",
			expected: "vault-srv-/foo/bar, vault-srv-/foo/bar",
		},
		{
			// two or more refs ending with +
			name:     "multi refs",
			regex:    DefaultRefRegexp,
			only:     []string{"ref", "secretref"},
			input:    "secretref+vault://srv/foo/bar+, secretref+vault://srv/foo/bar+ ",
			expected: "vault-srv-/foo/bar, vault-srv-/foo/bar ",
		},
		{
			// one ref with trailing string containing +
			name:     "multi refs",
			regex:    DefaultRefRegexp,
			only:     []string{"ref", "secretref"},
			input:    "secretref+vault://srv/foo/bar+ + + ",
			expected: "vault-srv-/foo/bar + + ",
		},
		{
			// see https://github.com/roboll/helmfile/issues/973
			name:     "this shouldn't be expanded",
			regex:    DefaultRefRegexp,
			only:     []string{"ref", "secretref"},
			input:    "\"no-referrer\" always;\nreturn 301 $scheme://$host:$server_port/remote.php/dav;",
			expected: "\"no-referrer\" always;\nreturn 301 $scheme://$host:$server_port/remote.php/dav;",
		},
		{
			// see https://github.com/helmfile/vals/issues/57
			name:     "it should skip newline after fragment",
			regex:    DefaultRefRegexp,
			only:     []string{"ref", "secretref"},
			input:    "ref+vault://srv/foo/bar#certificate\n",
			expected: "vault-srv-/foo/bar\n",
		},
		{
			// see https://github.com/helmfile/vals/issues/57
			name:     "it should skip newline after path",
			regex:    DefaultRefRegexp,
			only:     []string{"ref", "secretref"},
			input:    "ref+vault://srv/foo/bar\n",
			expected: "vault-srv-/foo/bar\n",
		},
		{
			name:     "it should not match closing quotes when using query params",
			regex:    DefaultRefRegexp,
			only:     []string{"ref", "secretref"},
			input:    "\"ref+awsssm://srv/foo/bar?mode=singleparam\"",
			expected: "\"awsssm-srv-/foo/bar\"",
		},
		{
			name:     "it should not match closing quotes and prevent greedy matches like it occurs in json",
			regex:    DefaultRefRegexp,
			only:     []string{"ref", "secretref"},
			input:    "\"ref+awsssm://srv/foo/bar?mode=singleparam\",\n\"ref+awsssm://srv2/foo/bar?mode=singleparam\"",
			expected: "\"awsssm-srv-/foo/bar\",\n\"awsssm-srv2-/foo/bar\"",
		},
		{
			name:     "it should match greedily upto a space when using query params",
			regex:    DefaultRefRegexp,
			only:     []string{"ref", "secretref"},
			input:    "ref+awsssm://srv/foo/bar?mode=singleparam some-string",
			expected: "awsssm-srv-/foo/bar some-string",
		},
		{
			name:     "it should handle multiple refs when using query params",
			regex:    DefaultRefRegexp,
			only:     []string{"ref", "secretref"},
			input:    "ref+awsssm://srv/foo/bar?mode=singleparam some-string ref+awsssm://srv/foo/bar?mode=singleparam",
			expected: "awsssm-srv-/foo/bar some-string awsssm-srv-/foo/bar",
		},
		{
			name:     "it should handle quoted values in query",
			regex:    DefaultRefRegexp,
			only:     []string{"ref", "secretref"},
			input:    "ref+tfstategs://foo/bar.tfstate/state[\"value\"].value",
			expected: "tfstategs-foo-/bar.tfstate/state[\"value\"].value",
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
				Only:   tc.only,
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
