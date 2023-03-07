package vals

import (
	"io"
)

func streamYAML(path string, w io.Writer) error {
	nodes, err := Inputs(path)
	if err != nil {
		return err
	}

	return Output(w, "yaml", nodes)
}
