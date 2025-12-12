package vals

import (
	"bytes"
	"crypto/md5"
	"fmt"
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

// mockProvider is a simple test double implementing api.Provider
type mockProvider struct {
	getStringFunc    func(string) (string, error)
	getStringMapFunc func(string) (map[string]interface{}, error)
}

func (m *mockProvider) GetString(key string) (string, error) {
	if m.getStringFunc != nil {
		return m.getStringFunc(key)
	}
	return "", nil
}

func (m *mockProvider) GetStringMap(key string) (map[string]interface{}, error) {
	if m.getStringMapFunc != nil {
		return m.getStringMapFunc(key)
	}
	return nil, nil
}

func TestARNFragmentExtractionWithMockProvider(t *testing.T) {
	r, err := New(Options{})
	require.NoError(t, err)

	// compute provider hash used by Runtime (scheme + query.Encode())
	hash := fmt.Sprintf("%x", md5.Sum([]byte("echo")))

	arn := "arn:aws:secretsmanager:us-east-1:123456789012:secret:/myteam/mydoc"

	// mock provider returns a nested map for the ARN key
	mock := &mockProvider{
		getStringMapFunc: func(key string) (map[string]interface{}, error) {
			if key != arn {
				t.Fatalf("unexpected key passed to provider.GetStringMap: %q", key)
			}
			return map[string]interface{}{
				"myteam": map[string]interface{}{
					"mydoc": "mydoc",
				},
			}, nil
		},
	}

	r.providers[hash] = mock

	res, err := r.Get("ref+echo://" + arn + "#/myteam/mydoc")
	require.NoError(t, err)
	require.Equal(t, "mydoc", res)
}

func TestARNBasedURIParsing(t *testing.T) {
	// Test that ARN-based URIs with colons are parsed correctly
	// This tests the fix for issue #909
	testCases := []struct {
		name        string
		input       string
		expected    string
		checkResult bool
	}{
		{
			name:        "Simple echo ARN format",
			input:       "ref+echo://arn:aws:secretsmanager:us-east-1:123456789012:secret:/demo/app/database",
			expected:    "arn:aws:secretsmanager:us-east-1:123456789012:secret:/demo/app/database",
			checkResult: true,
		},
		{
			name:        "ARN with query params",
			input:       "ref+echo://arn:aws:secretsmanager:us-east-1:123456789012:secret:/demo/app/database?region=us-east-1",
			expected:    "arn:aws:secretsmanager:us-east-1:123456789012:secret:/demo/app/database",
			checkResult: true,
		},
		{
			name:        "ARN with fragment - parse only",
			input:       "ref+echo://arn:aws:secretsmanager:us-east-1:123456789012:secret:/myteam/mydoc#/myteam/mydoc",
			checkResult: false,
		},
		{
			name:        "ARN with both query and fragment - parse only",
			input:       "ref+echo://arn:aws:secretsmanager:us-east-1:123456789012:secret:/demo/app/database?region=us-east-1#/demo/app/database",
			checkResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Get(tc.input, Options{})
			require.NoError(t, err, "Failed to parse ARN-based URI: %s", tc.input)
			if tc.checkResult {
				require.Equal(t, tc.expected, result)
			}
		})
	}
}
