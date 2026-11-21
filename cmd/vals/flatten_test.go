package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/helmfile/vals"
)

var quiet = vals.Options{LogOutput: io.Discard}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFlattenDir(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "out")

	writeFile(t, filepath.Join(src, "query.sql"), "CREATE USER u IDENTIFIED BY 'ref+echo://alpha+';\n")
	writeFile(t, filepath.Join(src, "nested", "deep", "app.env"), "PASSWORD=ref+echo://beta\n")
	writeFile(t, filepath.Join(src, "plain.txt"), "no refs here\n")
	if err := os.Symlink("plain.txt", filepath.Join(src, "link.txt")); err != nil {
		t.Fatal(err)
	}

	if err := flattenDir(src, dst, quiet); err != nil {
		t.Fatalf("flattenDir: %v", err)
	}

	want := map[string]string{
		"query.sql":           "CREATE USER u IDENTIFIED BY 'alpha';\n",
		"nested/deep/app.env": "PASSWORD=beta\n",
		"plain.txt":           "no refs here\n",
	}
	for rel, content := range want {
		path := filepath.Join(dst, filepath.FromSlash(rel))
		got, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s: %v", rel, err)
			continue
		}
		if string(got) != content {
			t.Errorf("%s: expected %q, got %q", rel, content, got)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Errorf("%s: expected mode 0600, got %o", rel, mode)
		}
	}

	if _, err := os.Lstat(filepath.Join(dst, "link.txt")); !os.IsNotExist(err) {
		t.Errorf("symlink must be skipped, got err=%v", err)
	}
}

func TestFlattenDirReportsFileOnError(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "out")
	missing := filepath.Join(t.TempDir(), "missing.json")

	writeFile(t, filepath.Join(src, "bad.sql"), "ref+file://"+missing+"#/key+\n")

	err := flattenDir(src, dst, quiet)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), filepath.Join(src, "bad.sql")) {
		t.Errorf("error must name the file, got: %v", err)
	}
}

func TestFlattenDirRejectsBadPaths(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "a.txt"), "ref+echo://alpha\n")

	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(src, alias); err != nil {
		t.Fatal(err)
	}
	nonEmpty := t.TempDir()
	writeFile(t, filepath.Join(nonEmpty, "old.txt"), "old\n")

	cases := map[string]struct {
		src, dst, want string
	}{
		"output same as input":          {src, src, "must be outside"},
		"output inside input":           {src, filepath.Join(src, "out"), "must be outside"},
		"output inside input via alias": {src, filepath.Join(alias, "out"), "must be outside"},
		"output not empty":              {src, nonEmpty, "must be empty"},
		"input is a file":               {filepath.Join(src, "a.txt"), filepath.Join(t.TempDir(), "out"), "is not a directory"},
	}
	for name, c := range cases {
		err := flattenDir(c.src, c.dst, quiet)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: expected an error with %q, got: %v", name, c.want, err)
		}
	}

	if _, err := os.Lstat(filepath.Join(src, "out")); !os.IsNotExist(err) {
		t.Errorf("a rejected output directory must not be created, got err=%v", err)
	}
}
