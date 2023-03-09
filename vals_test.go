package vals

import (
	"bytes"
	"os"
	"path/filepath"
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
