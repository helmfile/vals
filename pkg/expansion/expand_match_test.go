package expansion

import (
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestExpandRegexpMatchInString(t *testing.T) {
	testcases := []struct {
		regex    *regexp.Regexp
		name     string
		input    string
		expected string
		only     []string
	}{
		{
			name:     "nested ref",
			regex:    DefaultRefRegexp,
			input:    "ref+echo://ref+echo://inner/value",
			expected: "echo-echo-inner--/value",
		},
		{
			name:     "triple nested ref",
			regex:    DefaultRefRegexp,
			input:    "ref+echo://ref+echo://ref+echo://deep/value",
			expected: "echo-echo-echo-deep---/value",
		},
		{
			name:     "nested ref with surrounding text",
			regex:    DefaultRefRegexp,
			input:    "prefix ref+echo://ref+echo://inner/value suffix",
			expected: "prefix echo-echo-inner--/value suffix",
		},
		{
			name:     "mixed nested and independent refs",
			regex:    DefaultRefRegexp,
			input:    "ref+echo://simple ref+echo://ref+echo://inner/value",
			expected: "echo-simple- echo-echo-inner--/value",
		},
		{
			name:     "nested secretref",
			regex:    DefaultRefRegexp,
			input:    "secretref+echo://secretref+echo://inner/value",
			expected: "echo-echo-inner--/value",
		},
		{
			name:     "mixed nested ref and secretref",
			regex:    DefaultRefRegexp,
			input:    "ref+echo://secretref+echo://inner/value",
			expected: "echo-echo-inner--/value",
		},
		{
			name:     "nested ref with only filter on inner",
			regex:    DefaultRefRegexp,
			only:     []string{"ref"},
			input:    "ref+echo://secretref+echo://inner/value",
			expected: "echo-secretref-echo://inner/value",
		},
		{
			name:     "nested ref with only filter on outer",
			regex:    DefaultRefRegexp,
			only:     []string{"secretref"},
			input:    "secretref+echo://ref+echo://inner/value",
			expected: "echo-ref-echo://inner/value",
		},
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
			lookup := func(m string) (interface{}, error) {
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
		regex    *regexp.Regexp
		input    map[string]interface{}
		expected map[string]interface{}
		name     string
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
			lookup := func(m string) (interface{}, error) {
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

func TestResolveInnerRefs(t *testing.T) {
	lookup := func(m string) (interface{}, error) {
		parsed, err := url.Parse(m)
		if err != nil {
			return "", err
		}
		return parsed.Scheme + "-" + parsed.Host + "-" + parsed.Path, nil
	}

	expand := ExpandRegexMatch{
		Target: DefaultRefRegexp,
		Lookup: lookup,
	}

	testcases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no ref prefix",
			input:    "just a plain string",
			expected: "just a plain string",
		},
		{
			name:     "single ref unchanged",
			input:    "ref+echo://simple/value",
			expected: "ref+echo://simple/value",
		},
		{
			name:     "single nesting resolved",
			input:    "ref+echo://ref+echo://inner/value",
			expected: "ref+echo://echo-inner-/value",
		},
		{
			name:     "double nesting resolved",
			input:    "ref+echo://ref+echo://ref+echo://deep/value",
			expected: "ref+echo://echo-echo-deep--/value",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := expand.resolveInnerRefs(tc.input, 0)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if actual != tc.expected {
				t.Errorf("expected: %s, got: %s", tc.expected, actual)
			}
		})
	}
}

func TestResolveInnerRefsNonScalarError(t *testing.T) {
	input := "ref+echo://ref+echo://trigger/value"

	t.Run("map value", func(t *testing.T) {
		expand := ExpandRegexMatch{
			Target: DefaultRefRegexp,
			Lookup: func(m string) (interface{}, error) {
				return map[string]interface{}{"key": "value"}, nil
			},
		}
		_, err := expand.resolveInnerRefs(input, 0)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "scalar") {
			t.Fatalf("expected scalar type error, got: %v", err)
		}
	})

	t.Run("nil value", func(t *testing.T) {
		expand := ExpandRegexMatch{
			Target: DefaultRefRegexp,
			Lookup: func(m string) (interface{}, error) {
				return nil, nil
			},
		}
		_, err := expand.resolveInnerRefs(input, 0)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "nil") {
			t.Fatalf("expected nil error, got: %v", err)
		}
	})

	t.Run("slice value", func(t *testing.T) {
		expand := ExpandRegexMatch{
			Target: DefaultRefRegexp,
			Lookup: func(m string) (interface{}, error) {
				return []string{"a", "b"}, nil
			},
		}
		_, err := expand.resolveInnerRefs(input, 0)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "scalar") {
			t.Fatalf("expected scalar type error, got: %v", err)
		}
	})
}

func TestResolveInnerRefsDepthLimit(t *testing.T) {
	lookup := func(m string) (interface{}, error) {
		// Always returns another nested ref to trigger infinite recursion
		return "ref+echo://nested/value", nil
	}

	expand := ExpandRegexMatch{
		Target: DefaultRefRegexp,
		Lookup: lookup,
	}

	input := "ref+echo://ref+echo://trigger/value"
	_, err := expand.resolveInnerRefs(input, 0)
	if err == nil {
		t.Fatal("expected depth limit error, got nil")
	}
	if !strings.Contains(err.Error(), "maximum nesting depth") {
		t.Fatalf("expected depth limit error, got: %v", err)
	}
}
