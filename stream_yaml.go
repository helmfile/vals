package vals

import (
	"io"
)

func streamYAML(path string, w, log io.Writer) error {
	nodes, err := Inputs(path)
	if err != nil {
		return err
	}

	nodes, err = EvalNodes(nodes, Options{LogOutput: log})
	if err != nil {
		return err
	}

	return Output(w, "yaml", nodes)
}
