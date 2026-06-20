package vals

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestFragment_Issue114_FileProvider reproduces helmfile/vals#114 on the file
// provider (mirrors the s3 scenario without AWS): non-string values must be
// extractable via a #/key fragment.
func TestFragment_Issue114_FileProvider(t *testing.T) {
	dir := t.TempDir()
	jsonFile := filepath.Join(dir, "data.json")
	require.NoError(t, os.WriteFile(jsonFile, []byte(`{"key":123,"ratio":1.5,"obj":{"a":1}}`), 0o600))

	ref := "ref+file://" + jsonFile

	cases := []struct {
		want interface{}
		name string
		frag string
	}{
		{name: "int", frag: "#/key", want: 123},
		{name: "float", frag: "#/ratio", want: 1.5},
		{name: "object", frag: "#/obj", want: map[string]interface{}{"a": 1}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Eval(map[string]interface{}{"v": ref + tc.frag}, Options{})
			require.NoError(t, err)
			require.Equal(t, tc.want, res["v"])
		})
	}
}

// TestFragment_ScalarTypes covers non-string scalar leaf values (float and a
// uint64 above math.MaxInt64) that become extractable once isTerminalValue
// accepts all scalar kinds, not just bool/int/string.
func TestFragment_ScalarTypes(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "data.yaml")
	require.NoError(t, os.WriteFile(f, []byte(
		"ratio: 1.5\n"+
			"neg: -2.5\n"+
			"big: 9223372036854775808\n"+ // > math.MaxInt64 -> yaml decodes as uint64
			"n: 42\n"+
			"s: hello\n"+
			"flag: true\n"), 0o600))
	ref := "ref+file://" + f

	res, err := Eval(map[string]interface{}{
		"ratio": ref + "#/ratio",
		"neg":   ref + "#/neg",
		"big":   ref + "#/big",
		"n":     ref + "#/n",
		"s":     ref + "#/s",
		"flag":  ref + "#/flag",
	}, Options{})
	require.NoError(t, err)
	require.Equal(t, 1.5, res["ratio"])
	require.Equal(t, -2.5, res["neg"])
	require.Equal(t, uint64(9223372036854775808), res["big"])
	require.Equal(t, 42, res["n"])
	require.Equal(t, "hello", res["s"])
	require.Equal(t, true, res["flag"])
}

// TestFragment_NestedObjectAndArray verifies that a fragment pointing at a nested
// map or slice returns that structure as-is, and that descending to a deeper leaf
// keeps working.
func TestFragment_NestedObjectAndArray(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "data.yaml")
	require.NoError(t, os.WriteFile(f, []byte(
		"obj:\n"+
			"  a: 1\n"+
			"  b: two\n"+
			"list:\n"+
			"  - x\n"+
			"  - y\n"), 0o600))
	ref := "ref+file://" + f

	t.Run("object", func(t *testing.T) {
		res, err := Eval(map[string]interface{}{"v": ref + "#/obj"}, Options{})
		require.NoError(t, err)
		require.Equal(t, map[string]interface{}{"a": 1, "b": "two"}, res["v"])
	})
	t.Run("array", func(t *testing.T) {
		res, err := Eval(map[string]interface{}{"v": ref + "#/list"}, Options{})
		require.NoError(t, err)
		require.Equal(t, []interface{}{"x", "y"}, res["v"])
	})
	t.Run("nested-leaf", func(t *testing.T) {
		res, err := Eval(map[string]interface{}{"v": ref + "#/obj/a"}, Options{})
		require.NoError(t, err)
		require.Equal(t, 1, res["v"])
	})
}
