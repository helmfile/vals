package vutil

import (
	"reflect"
	"strings"
	"testing"
)

func TestEvaluateStringInterpolationLikeGoTemplateValues(t *testing.T) {
	type testcase struct {
		input map[string]interface{}
		expected map[string]interface{}
	}

	testcases := []testcase{
		{
			input: map[string]interface{}{
				"foo": "A${{upper \"b\"}}C",
			},
			expected: map[string]interface{}{
				"foo": "ABC",
			},
		},
	}

	for _, tc := range testcases {
		funcmap := map[string]interface{}{
			"upper": strings.ToUpper,
		}
		actual, err := EvaluateStringInterpolationLikeGoTemplateValues(tc.input, funcmap)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(tc.expected, actual) {
			t.Errorf("unexpected result: expected:\n%v\ngot:%v\n", tc.expected, actual)
		}
	}
}
