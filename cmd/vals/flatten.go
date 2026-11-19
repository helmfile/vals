package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/helmfile/vals"
)

// runFlatten implements the flatten command.
func runFlatten(args []string) {
	flattenCmd := flag.NewFlagSet("flatten", flag.ExitOnError)
	f := flattenCmd.String("f", "-", "Text file to be flattened, or a directory when -output-dir is set. When set to \"-\", vals reads from STDIN")
	silent := flattenCmd.Bool("s", false, "Silent mode")
	e := flattenCmd.Bool("exclude-secret", false, "Leave secretref+<uri> as-is and only replace ref+<uri>")
	outputDir := flattenCmd.String("output-dir", "", "Render every file under the directory given by -f to the same relative path under this directory, which must be missing or empty")
	err := flattenCmd.Parse(args)
	if err != nil {
		fatal("%v", err)
	}

	var logOut io.Writer = os.Stderr
	if *silent {
		logOut = io.Discard
	}
	opts := vals.Options{
		ExcludeSecret: *e,
		LogOutput:     logOut,
	}

	if *outputDir != "" {
		if *f == "-" {
			fatal("-output-dir requires -f to be a directory")
		}
		if err := flattenDir(*f, *outputDir, opts); err != nil {
			fatal("%v", err)
		}
		return
	}

	text, err := vals.TextInput(*f)
	if err != nil {
		fatal("%v", err)
	}

	result, err := vals.Get(text, opts)
	if err != nil {
		fatal("%v", err)
	}

	_, err = os.Stdout.WriteString(result)
	if err != nil {
		fatal("%v", err)
	}
}

// flattenDir renders every regular file under src to the same relative path
// under dst, which must be missing or empty and outside of src. One runtime is
// used for the whole tree, so files share its cache of resolved refs.
func flattenDir(src, dst string, opts vals.Options) error {
	root, err := realDir(src)
	if err != nil {
		return err
	}
	out, err := realDir(dst)
	if errors.Is(err, fs.ErrNotExist) {
		parent, perr := realDir(filepath.Dir(dst))
		if perr != nil {
			return perr
		}
		out = filepath.Join(parent, filepath.Base(dst))
	} else if err != nil {
		return err
	}
	sep := string(filepath.Separator)
	if rel, err := filepath.Rel(root, out); err == nil && !strings.HasPrefix(rel+sep, ".."+sep) {
		return fmt.Errorf("output directory %s must be outside of the input directory %s", dst, src)
	}
	if err := os.Mkdir(out, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	entries, err := os.ReadDir(out)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("output directory %s must be empty", dst)
	}

	rt, err := vals.New(opts)
	if err != nil {
		return err
	}

	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		target := filepath.Join(out, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		res, err := rt.Get(string(data))
		if err != nil {
			return fmt.Errorf("%s: %w", filepath.Join(src, rel), err)
		}
		return os.WriteFile(target, []byte(res), 0o600)
	})
}

// realDir returns the absolute path of dir with symlinks resolved.
func realDir(dir string) (string, error) {
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", dir)
	}
	return filepath.Abs(resolved)
}
