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

func TestGetRawText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain text with ref",
			input:    "plain text with ref+echo://hello-world embedded",
			expected: "plain text with hello-world embedded",
		},
		{
			name:     "JSONC with refs",
			input:    "{\n  // comment\n  \"key\": \"ref+echo://value\"\n}",
			expected: "{\n  // comment\n  \"key\": \"value\"\n}",
		},
		{
			name:     "config file",
			input:    "DATABASE_URL=ref+echo://postgres://localhost/db\nAPI_KEY=ref+echo://secret-123",
			expected: "DATABASE_URL=postgres://localhost/db\nAPI_KEY=secret-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Get(tt.input, Options{})
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}
