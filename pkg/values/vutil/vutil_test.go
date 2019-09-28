package vutil

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestModifyMapValues(t *testing.T) {
	type testcase struct {
		input    interface{}
		conv func(map[string]interface{}) (interface{}, bool, error)
		expected interface{}
	}
	uppercase := evalUnaryExpr("ref", func(v string) (string, error) {
		return strings.ToUpper(v), nil
	})
	lowercase := evalUnaryExpr("ref", func(v string) (string, error) {
		return strings.ToLower(v), nil
	})
	uppercaseWithTypes := EvalUnaryExprWithTypes("ref", func(v string) (string, error) {
		return strings.ToUpper(v), nil
	})
	lowercaseWithTypes := EvalUnaryExprWithTypes("ref", func(v string) (string, error) {
		return strings.ToLower(v), nil
	})
	testcases := []testcase{
		{
			input: map[string]interface{}{
				"foo": map[string]interface{}{"$ref": "foo"},
				"bar": "BAR",
			},
			conv: uppercase,
			expected: map[string]interface{}{
				"foo": "FOO",
				"bar": "BAR",
			},
		},
		{
			input: map[string]interface{}{
				"foo": map[string]interface{}{"$ref": "FOO"},
				"bar": "BAR",
			},
			conv: lowercase,
			expected: map[string]interface{}{
				"foo": "foo",
				"bar": "BAR",
			},
		},
		{
			input: map[string]interface{}{
				"foo": map[string]interface{}{"$ref": "foo"},
				"bar": "BAR",
			},
			conv: uppercaseWithTypes,
			expected: map[string]interface{}{
				"foo": "FOO",
				"bar": "BAR",
			},
		},
		{
			input: map[string]interface{}{
				"foo": map[string]interface{}{"$ref": "FOO"},
				"bar": "BAR",
			},
			conv: lowercaseWithTypes,
			expected: map[string]interface{}{
				"foo": "foo",
				"bar": "BAR",
			},
		},
		{
			input: map[string]interface{}{
				"$types": map[string]interface{}{
					"v": "fo",
				},
				"foo": map[string]interface{}{"$v": "o"},
				"bar": "BAR",
			},
			conv: uppercaseWithTypes,
			expected: map[string]interface{}{
				"foo": "FOO",
				"bar": "BAR",
			},
		},
		{
			input: map[string]interface{}{
				"$types": map[string]interface{}{
					"v": "FO",
				},
				"foo": map[string]interface{}{"$v": "O"},
				"bar": "BAR",
			},
			conv: lowercaseWithTypes,
			expected: map[string]interface{}{
				"foo": "foo",
				"bar": "BAR",
			},
		},
	}

	for i := range testcases {
		name := fmt.Sprintf("case_%d", i)
		tc := testcases[i]

		t.Run(name, func(t *testing.T) {
			actual, err := ModifyMapValues(tc.input, tc.conv)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(actual, tc.expected) {
				t.Errorf("unexpected result: expected=%v, got=%v", tc.expected, actual)
			}
		})
	}
}
