package main

import (
	"flag"
	"fmt"
	"github.com/mumoshu/values"
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

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format + "\n", args...)
	os.Exit(1)
}


func readOrFail(f *string) map[string]interface{} {
	m, err := values.Input(*f)
	if err != nil {
		fatal("%v", err)
	}
	return m
}

func writeOrFail(o *string, res map[string]interface{}) {
	out, err := values.Output(*o, res)
	if err != nil {
		fatal("%v", err)
	}
	fmt.Printf("%s", *out)
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

		m := readOrFail(f)

		res, err := values.Eval(m)
		if err != nil {
			fatal("%v", err)
		}

		writeOrFail(o, res)
	case CmdFlatten:
		evalCmd := flag.NewFlagSet(CmdFlatten, flag.ExitOnError)
		f := evalCmd.String("f", "", "YAML/JSON file to be flattened")
		o := evalCmd.String("o", "yaml", "Output type which is either \"yaml\" or \"json\"")
		c := evalCmd.Bool("c", false, "Use vals' own compact format of $ref")
		evalCmd.Parse(os.Args[2:])

		m := readOrFail(f)

		res, err := values.Flatten(m, *c)
		if err != nil {
			fatal("%v", err)
		}

		writeOrFail(o, res)
	default:
		flag.Usage()
	}
}
