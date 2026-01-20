package vals

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	testCases := []struct {
		yamlDocs string
		expected string
	}{
		{`apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
---
apiVersion: v1
data:
  username: ref+echo://secrets.enc.yaml
kind: Secret
`, `apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
---
apiVersion: v1
data:
  username: secrets.enc.yaml
kind: Secret
`},
		{`foo: ref+echo://foo/bar
bar: ref+echo://foo/bar#/foo
`, `bar: bar
foo: foo/bar
`},
		{`bar: ref+echo://foo/bar#/foo
foo: ref+echo://foo/bar`, `bar: bar
foo: foo/bar
`},
	}

	for idx, testCase := range testCases {
		t.Run(strconv.Itoa(idx), func(t *testing.T) {
			t.Parallel()
			tmpFile, err := os.CreateTemp("", "secrets.yaml")
			defer os.Remove(tmpFile.Name())
			require.NoError(t, err)
			_, err = tmpFile.WriteString(testCase.yamlDocs)
			require.NoError(t, err)

			input, err := Inputs(tmpFile.Name())
			require.NoError(t, err)
			nodes, err := EvalNodes(input, Options{})
			require.NoError(t, err)
			buf := new(strings.Builder)

			err = Output(buf, "", nodes)
			require.NoError(t, err)

			require.Equal(t, testCase.expected, buf.String())
		})
	}
}

func TestGet(t *testing.T) {
	testCases := []struct {
		code     string
		expected string
	}{
		{"ref+echo://foo/bar", "foo/bar"},
		{"ref+echo://foo/bar#/foo", "bar"},
		{"ref+echo://foo/bar ref+echo://foo/bar#/foo", "foo/bar bar"},
		{"ref+echo://foo/bar#/foo ref+echo://foo/bar", "bar foo/bar"},
	}

	for idx, testCase := range testCases {
		t.Run(strconv.Itoa(idx), func(t *testing.T) {
			t.Parallel()
			got, err := Get(testCase.code, Options{})
			require.NoError(t, err)
			require.Equal(t, testCase.expected, got)
		})
	}
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

func TestEvalIntegerBool(t *testing.T) {
	var yamlDocs = `
---
Integer: 1
ResultInteger: ref+file://secrets.yaml#/Integer
Bool: true
ResultBool: ref+file://secrets.yaml#/Bool
`
	var expected = `Bool: true
Integer: 1
ResultBool: "true"
ResultInteger: "1"
`

	tmpFile, err := os.Create("secrets.yaml")
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
