package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/mumoshu/values"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
)

func flagUsage() {
	text := `vals is a Helm-like configuration "Values" loader with support for various sources and merge strategies

Usage:
  vals [command]

Available Commands:
  eval	Evaluate a JSON/YAML document and replace any template expressions in it and prints the result

Use "vals [command] --help" for more infomation about a command
`

	fmt.Fprintf(os.Stderr, "%s\n", text)
}

func input(f *string) map[string]interface{} {
	m := map[string]interface{}{}
	var input []byte
	var err error
	if *f == "-" {
		input, err = ioutil.ReadAll(os.Stdin)
	} else if *f != "" {
		input, err = ioutil.ReadFile(*f)
	} else {
		//evalCmd.Usage()
		fmt.Fprintf(os.Stderr, "Nothing to eval! Specify -f FILE\n")
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	if err := yaml.Unmarshal(input, &m); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	return m
}

func output(o *string, res map[string]interface{}) {
	var err error
	var out []byte
	switch *o {
	case "yaml":
		out, err = yaml.Marshal(res)
	case "json":
		out, err = json.Marshal(res)
	default:
		fmt.Fprintf(os.Stderr, "Unknown output type: %s", *o)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed marshalling into %s: %v\n", *o, err)
		os.Exit(1)
	}
	fmt.Printf("%s", string(out))
}

func main() {
	flag.Usage = flagUsage

	CmdEval := "eval"
	CmdFlatten := "flatten"

	if len(os.Args) == 1 {
		flag.Usage()
		return
	}

	switch os.Args[1] {
	case CmdEval:
		evalCmd := flag.NewFlagSet(CmdEval, flag.ExitOnError)
		f := evalCmd.String("f", "", "YAML/JSON file to be evaluated")
		o := evalCmd.String("o", "yaml", "Output type which is either \"yaml\" or \"json\"")
		evalCmd.Parse(os.Args[2:])

		m := input(f)

		res, err := values.Eval(m)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		output(o, res)
	case CmdFlatten:
		evalCmd := flag.NewFlagSet(CmdFlatten, flag.ExitOnError)
		f := evalCmd.String("f", "", "YAML/JSON file to be flattened")
		o := evalCmd.String("o", "yaml", "Output type which is either \"yaml\" or \"json\"")
		c := evalCmd.Bool("c", false, "Use vals' own compact format of $ref")
		evalCmd.Parse(os.Args[2:])

		m := input(f)

		res, err := values.Flatten(m, *c)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		output(o, res)
	default:
		flag.Usage()
	}
}
