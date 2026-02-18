package vals

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"
)

// TextInput reads a file (or stdin) as raw text and returns the contents as a string.
// It does not parse the contents — the file is treated as opaque text.
// Use f="-" to read from stdin, or pass a file path.
func TextInput(f string) (string, error) {
	var reader io.Reader
	if f == "-" {
		reader = os.Stdin
	} else if f != "" {
		fp, err := os.Open(f)
		if err != nil {
			return "", err
		}
		defer func() {
			_ = fp.Close()
		}()

		info, err := fp.Stat()
		if err != nil {
			return "", err
		}

		if info.IsDir() {
			return "", fmt.Errorf("text mode does not support directories: %s", f)
		}

		reader = fp
	} else {
		return "", fmt.Errorf("Nothing to read: No file specified")
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func Inputs(f string) ([]yaml.Node, error) {
	var reader io.Reader
	if f == "-" {
		reader = os.Stdin
	} else if f != "" {
		fp, err := os.Open(f)
		if err != nil {
			return nil, err
		}

		info, err := fp.Stat()
		if err != nil {
			return nil, err
		}

		if info.IsDir() {
			entries, err := fp.ReadDir(0)
			if err != nil {
				return nil, err
			}

			var nodes []yaml.Node

			for _, e := range entries {
				s := filepath.Join(f, e.Name())
				ns, err := Inputs(s)
				if err != nil {
					return nil, err
				}

				nodes = append(nodes, ns...)
			}

			return nodes, nil
		}

		reader = fp
		defer func() {
			_ = fp.Close()
		}()
	} else {
		return nil, fmt.Errorf("Nothing to eval: No file specified")
	}
	return nodesFromReader(reader)
}

func nodesFromReader(reader io.Reader) ([]yaml.Node, error) {
	nodes := []yaml.Node{}
	buf := bufio.NewReader(reader)
	decoder := yaml.NewDecoder(buf)
	for {
		node := yaml.Node{}
		if err := decoder.Decode(&node); err != nil {
			if err != io.EOF {
				return nil, err
			}
			break
		}
		if len(node.Content[0].Content) > 0 {
			replaceTimestamp(&node)
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

// A custom unmarshal is needed because go-yaml parse "YYYY-MM-DD" as a full
// timestamp, writing YYYY-MM-DD HH:MM:SS +0000 UTC when encoding, so we are
// going to treat timestamps as strings.
// See: https://github.com/go-yaml/yaml/issues/770

func replaceTimestamp(n *yaml.Node) {
	if len(n.Content) == 0 {
		return
	}
	for _, innerNode := range n.Content {
		if slices.Contains([]string{"!!map", "!!seq"}, innerNode.Tag) {
			replaceTimestamp(innerNode)
		}
		if innerNode.Tag == "!!timestamp" {
			innerNode.Tag = "!!str"
		}
	}
}

func Output(output io.Writer, format string, nodes []yaml.Node) error {
	for i, node := range nodes {
		var v interface{}
		if err := node.Decode(&v); err != nil {
			return err
		}
		if format == "json" {
			bs, err := json.Marshal(v)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(output, string(bs))
		} else {
			encoder := yaml.NewEncoder(output)
			encoder.SetIndent(2)

			if err := encoder.Encode(v); err != nil {
				return err
			}
		}
		if i != len(nodes)-1 {
			_, _ = fmt.Fprintln(output, "---")
		}
	}
	return nil
}
