package vals

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

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
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
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
