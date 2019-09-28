package values

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
)

func Input(f string) (map[string]interface{}, error) {
	m := map[string]interface{}{}
	var input []byte
	var err error
	if f == "-" {
		input, err = ioutil.ReadAll(os.Stdin)
	} else if f != "" {
		input, err = ioutil.ReadFile(f)
	} else {
		return nil, fmt.Errorf("Nothing to eval: No file specified")
	}
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(input, &m); err != nil {
		return nil, err
	}

	return m, nil
}

func Output(o string, res map[string]interface{}) (*string, error) {
	var err error
	var out []byte
	switch o {
	case "yaml":
		out, err = yaml.Marshal(res)
	case "json":
		out, err = json.Marshal(res)
	default:
		return nil, fmt.Errorf("Unknown output type: %s", o)
	}
	if err != nil {
		return nil, fmt.Errorf("Failed marshalling into %s: %v", o, err)
	}
	str := string(out)
	return &str, nil
}
