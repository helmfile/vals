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

	require.NoError(t, os.WriteFile(filepath.Join(dir, "input.yaml"), data, 0o644))

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

	expected := []string{
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

	expected := []string{
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

func TestGetExecProvider(t *testing.T) {
	testCases := []struct {
		code     string
		expected string
	}{
		{"ref+exec://echo/hello", "hello"},
		{"ref+exec://echo/hello world", "hello world"},
		{"ref+exec://printf/hello?trim=false", "hello"},
	}

	for _, tc := range testCases {
		t.Run(tc.code, func(t *testing.T) {
			t.Parallel()
			got, err := Get(tc.code, Options{})
			require.NoError(t, err)
			require.Equal(t, tc.expected, got)
		})
	}
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

func TestFlatten(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain text with refs",
			input:    "The secret is ref+echo://mysecret\n",
			expected: "The secret is mysecret\n",
		},
		{
			name:     "JSONC-like with comments",
			input:    "{\n  \"key\": \"ref+echo://hello\"\n  // this is a comment\n}\n",
			expected: "{\n  \"key\": \"hello\"\n  // this is a comment\n}\n",
		},
		{
			name:     "JSON5-like with trailing comma and unquoted keys",
			input:    "{\n  key: \"ref+echo://value\",\n  other: 123,\n}\n",
			expected: "{\n  key: \"value\",\n  other: 123,\n}\n",
		},
		{
			name:     "multiple refs on different lines",
			input:    "line1: ref+echo://first\nline2: ref+echo://second\nline3: ref+echo://third\n",
			expected: "line1: first\nline2: second\nline3: third\n",
		},
		{
			name:     "no refs passthrough",
			input:    "just some plain text\nwith multiple lines\nand no refs at all\n",
			expected: "just some plain text\nwith multiple lines\nand no refs at all\n",
		},
		{
			name:     "ref embedded in larger string",
			input:    "prefix-ref+echo://middle-suffix\n",
			expected: "prefix-middle-suffix\n",
		},
		{
			name:     "ref with fragment",
			input:    "value is ref+echo://foo/bar#/foo\n",
			expected: "value is bar\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Integration test: write to file, read via TextInput, resolve via Get
			dir := t.TempDir()
			path := filepath.Join(dir, "input.txt")
			require.NoError(t, os.WriteFile(path, []byte(tc.input), 0o644))

			text, err := TextInput(path)
			require.NoError(t, err)
			require.Equal(t, tc.input, text)

			got, err := Get(text, Options{})
			require.NoError(t, err)
			require.Equal(t, tc.expected, got)
		})
	}
}

func TestGetNested(t *testing.T) {
	testCases := []struct {
		envVars  map[string]string
		name     string
		code     string
		expected string
	}{
		{nil, "nested echo", "ref+echo://ref+echo://inner/value", "inner/value"},
		{nil, "triple nested echo", "ref+echo://ref+echo://ref+echo://deep/value", "deep/value"},
		{nil, "non-nested unchanged", "ref+echo://simple/value", "simple/value"},
		{map[string]string{"TEST_NESTED_VAR": "resolved"}, "nested envsubst in echo", "ref+echo://ref+envsubst://$TEST_NESTED_VAR/path", "resolved/path"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}
			got, err := Get(tc.code, Options{})
			require.NoError(t, err)
			require.Equal(t, tc.expected, got)
		})
	}
}

func TestTextInput(t *testing.T) {
	t.Run("read file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.txt")
		content := "hello ref+echo://world\n// comment\n"
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		got, err := TextInput(path)
		require.NoError(t, err)
		require.Equal(t, content, got)
	})

	t.Run("read from stdin", func(t *testing.T) {
		content := "line1=ref+echo://value1\nline2=ref+echo://value2\n"

		r, w, err := os.Pipe()
		require.NoError(t, err)

		origStdin := os.Stdin
		os.Stdin = r
		defer func() { os.Stdin = origStdin }()

		_, err = w.WriteString(content)
		require.NoError(t, err)
		w.Close()

		got, err := TextInput("-")
		require.NoError(t, err)
		require.Equal(t, content, got)
	})

	t.Run("directory returns error", func(t *testing.T) {
		dir := t.TempDir()
		_, err := TextInput(dir)
		require.Error(t, err)
		require.Contains(t, err.Error(), "does not support directories")
	})

	t.Run("empty string returns error", func(t *testing.T) {
		_, err := TextInput("")
		require.Error(t, err)
		require.Contains(t, err.Error(), "No file specified")
	})

	t.Run("nonexistent file returns error", func(t *testing.T) {
		_, err := TextInput("/nonexistent/path/file.txt")
		require.Error(t, err)
	})
}

func TestEvalNested(t *testing.T) {
	t.Setenv("TEST_NESTED_VAR", "resolved")

	template := map[string]interface{}{
		"key1": "ref+echo://ref+echo://inner/value",
		"key2": "ref+echo://simple/value",
		"key3": "ref+echo://ref+envsubst://$TEST_NESTED_VAR/path",
	}

	got, err := Eval(template)
	require.NoError(t, err)
	require.Equal(t, "inner/value", got["key1"])
	require.Equal(t, "simple/value", got["key2"])
	require.Equal(t, "resolved/path", got["key3"])
}

func TestEvalNodesWithDictionaries(t *testing.T) {
	yamlDocs := `- entry: first
  username: ref+echo://secrets.enc.yaml
- entry: second
  username: ref+echo://secrets.enc.yaml
`

	expected := `- entry: first
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
	yamlDocs := `
date: 2025-01-01
datet_in_list:
  - from: 2025-01-01
datetime: 2025-01-01T12:34:56Z
datetime_millis: 2025-01-01T12:34:56.789Z
datetime_offset: 2025-01-01T12:34:56+01:00
`

	expected := `date: "2025-01-01"
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

func TestEvalNodesTypes(t *testing.T) {
	tmpDir := t.TempDir()

	createTmpFile := func(t *testing.T, dir, name, content string) string {
		tmpFilePath := filepath.Join(dir, name)
		err := os.WriteFile(tmpFilePath, []byte(content), 0o600)
		require.NoError(t, err)
		return tmpFilePath
	}

	secretYaml := `
bool: true
int: 42
string: "It's a string"
`
	secretsFile := createTmpFile(t, tmpDir, "secrets.yaml", secretYaml)

	replacer := strings.NewReplacer("{file-ref}", "ref+file://"+secretsFile)
	inputYaml := replacer.Replace(`
bool_value: {file-ref}#/bool
int_value: {file-ref}#/int
string_value: {file-ref}#/string
`)
	inputFile := createTmpFile(t, tmpDir, "input.yaml", inputYaml)

	expected := `bool_value: true
int_value: 42
string_value: It's a string
`

	input, err := Inputs(inputFile)
	require.NoError(t, err)

	nodes, err := EvalNodes(input, Options{})
	require.NoError(t, err)
	buf := new(strings.Builder)

	err = Output(buf, "", nodes)
	require.NoError(t, err)

	require.Equal(t, expected, buf.String())
}
