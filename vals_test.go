package vals

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExec(t *testing.T) {
	dir := t.TempDir()

	// should evaluate to "x: baz"
	data := []byte("x: ref+echo://foo/bar/baz#/foo/bar")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "input.yaml"), data, 0644))

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	c := ExecConfig{
		StreamYAML: dir,
		Stdout:     stdout,
		Stderr:     stderr,
	}

	err := Exec(map[string]interface{}{}, []string{"cat"}, c)
	require.NoError(t, err)

	require.Equal(t, "x: baz\n", stdout.String())
}

func TestEnv(t *testing.T) {
	input := make(map[string]interface{})

	input["var1"] = "ref+echo://value"
	input["var2"] = "ref+echo://val;ue"
	input["var3"] = "ref+echo://val'ue"
	input["var4"] = "ref+echo://'value"

	var expected = []string{
		"var1=value",
		"var2=val;ue",
		"var3=val'ue",
		"var4='value",
	}

	got, err := Env(input)
	require.NoError(t, err)

	sort.Strings(got)
	require.Equal(t, expected, got)
}

func TestQuotedEnv(t *testing.T) {
	input := make(map[string]interface{})

	input["var1"] = "ref+echo://value"
	input["var2"] = "ref+echo://val;ue"
	input["var3"] = "ref+echo://val'ue"
	input["var4"] = "ref+echo://'value"

	var expected = []string{
		"var1=value",
		`var2='val;ue'`,
		`var3='val'"'"'ue'`,
		`var4=''"'"'value'`,
	}

	got, err := QuotedEnv(input)
	require.NoError(t, err)

	sort.Strings(got)
	require.Equal(t, expected, got)
}

func TestEvalNodes(t *testing.T) {
	var yamlDocs = `apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
---
apiVersion: v1
data:
  username: ref+echo://secrets.enc.yaml
kind: Secret
`

	var expected = `apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
---
apiVersion: v1
data:
  username: secrets.enc.yaml
kind: Secret
`

	tmpFile, err := os.CreateTemp("", "secrets.yaml")
	defer os.Remove(tmpFile.Name())
	require.NoError(t, err)

	_, err = tmpFile.WriteString(yamlDocs)
	require.NoError(t, err)

	input, err := Inputs(tmpFile.Name())
	require.NoError(t, err)

	nodes, err := EvalNodes(input, Options{})
	require.NoError(t, err)
	buf := new(strings.Builder)

	err = Output(buf, "", nodes)
	require.NoError(t, err)

	require.Equal(t, expected, buf.String())
}

func TestEvalNodesWithDictionaries(t *testing.T) {
	var yamlDocs = `- entry: first
  username: ref+echo://secrets.enc.yaml
- entry: second
  username: ref+echo://secrets.enc.yaml
`

	var expected = `- entry: first
  username: secrets.enc.yaml
- entry: second
  username: secrets.enc.yaml
`

	tmpFile, err := os.CreateTemp("", "secrets.yaml")
	defer os.Remove(tmpFile.Name())
	require.NoError(t, err)

	_, err = tmpFile.WriteString(yamlDocs)
	require.NoError(t, err)

	input, err := Inputs(tmpFile.Name())
	require.NoError(t, err)

	nodes, err := EvalNodes(input, Options{})
	require.NoError(t, err)
	buf := new(strings.Builder)

	err = Output(buf, "", nodes)
	require.NoError(t, err)

	require.Equal(t, expected, buf.String())
}

func TestEvalNodesWithTime(t *testing.T) {
	var yamlDocs = `
date: 2025-01-01
datet_in_list: 
  - from: 2025-01-01
datetime: 2025-01-01T12:34:56Z
datetime_millis: 2025-01-01T12:34:56.789Z
datetime_offset: 2025-01-01T12:34:56+01:00
`

	var expected = `date: "2025-01-01"
datet_in_list:
  - from: "2025-01-01"
datetime: "2025-01-01T12:34:56Z"
datetime_millis: "2025-01-01T12:34:56.789Z"
datetime_offset: "2025-01-01T12:34:56+01:00"
`

	tmpFile, err := os.CreateTemp("", "secrets.yaml")
	defer os.Remove(tmpFile.Name())
	require.NoError(t, err)

	_, err = tmpFile.WriteString(yamlDocs)
	require.NoError(t, err)

	input, err := Inputs(tmpFile.Name())
	require.NoError(t, err)

	nodes, err := EvalNodes(input, Options{})
	require.NoError(t, err)
	buf := new(strings.Builder)

	err = Output(buf, "", nodes)
	require.NoError(t, err)

	require.Equal(t, expected, buf.String())
}

func TestNestedExpressions(t *testing.T) {
	// Set up test environment variables
	os.Setenv("TEST_VAR", "hello-world")
	os.Setenv("NESTED_VAR", "nested-value")
	defer func() {
		os.Unsetenv("TEST_VAR")
		os.Unsetenv("NESTED_VAR")
	}()

	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "echo with envsubst nested",
			input: map[string]interface{}{
				"test": "ref+echo://ref+envsubst://$TEST_VAR/foo",
			},
			expected: map[string]interface{}{
				"test": "hello-world/foo",
			},
		},
		{
			name: "envsubst with echo nested",
			input: map[string]interface{}{
				"test": "ref+envsubst://prefix-ref+echo://$NESTED_VAR-suffix",
			},
			expected: map[string]interface{}{
				"test": "prefix-nested-value-suffix",
			},
		},
		{
			name: "multiple nested expressions",
			input: map[string]interface{}{
				"test1": "ref+echo://ref+envsubst://$TEST_VAR/path",
				"test2": "ref+envsubst://ref+echo://$NESTED_VAR",
			},
			expected: map[string]interface{}{
				"test1": "hello-world/path",
				"test2": "nested-value",
			},
		},
		{
			name: "deeply nested expressions",
			input: map[string]interface{}{
				"test": "ref+echo://prefix/ref+envsubst://ref+echo://$NESTED_VAR/suffix",
			},
			expected: map[string]interface{}{
				"test": "prefix/nested-value/suffix",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Eval(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestNestedExpressionsWithGet(t *testing.T) {
	// Set up test environment variables
	os.Setenv("TEST_VAR", "hello-world")
	defer os.Unsetenv("TEST_VAR")

	runtime, err := New(Options{})
	require.NoError(t, err)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple nested expression",
			input:    "ref+echo://ref+envsubst://$TEST_VAR/foo",
			expected: "hello-world/foo",
		},
		{
			name:     "envsubst with echo nested",
			input:    "ref+envsubst://prefix-ref+echo://suffix",
			expected: "prefix-suffix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := runtime.Get(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestNestedExpressionsBackwardCompatibility(t *testing.T) {
	// Ensure that existing non-nested expressions still work
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "simple echo",
			input: map[string]interface{}{
				"test": "ref+echo://hello-world",
			},
			expected: map[string]interface{}{
				"test": "hello-world",
			},
		},
		{
			name: "echo with fragment",
			input: map[string]interface{}{
				"test": "ref+echo://foo/bar/baz#/foo/bar",
			},
			expected: map[string]interface{}{
				"test": "baz",
			},
		},
		{
			name: "file provider",
			input: map[string]interface{}{
				"test": "ref+file://./myjson.json#/baz/mykey",
			},
			expected: map[string]interface{}{
				"test": "myvalue",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Eval(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestNestedExpressionsEdgeCases(t *testing.T) {
	// Set up test environment variables
	os.Setenv("EDGE_VAR", "edge-value")
	defer os.Unsetenv("EDGE_VAR")

	tests := []struct {
		name      string
		input     string
		expected  string
		expectErr bool
	}{
		{
			name:     "nested expression with special characters",
			input:    "ref+echo://ref+envsubst://$EDGE_VAR-test_123",
			expected: "edge-value-test_123",
		},
	}

	runtime, err := New(Options{})
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := runtime.Get(tt.input)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, result)
			}
		})
	}
}
