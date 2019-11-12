package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/variantdev/vals"
	"gopkg.in/yaml.v3"
	"os"
)

func flagUsage() {
	text := `vals is a Helm-like configuration "Values" loader with support for various sources and merge strategies

Usage:
  vals [command]

Available Commands:
  eval		Evaluate a JSON/YAML document and replace any template expressions in it and prints the result
  exec		Populates the environment variables and executes the command
  env		Renders environment variables to be consumed by eval or a tool like direnv
  ksdecode	Decode YAML document(s) by converting Secret resources' "data" to "stringData" for use with "vals eval"

Use "vals [command] --help" for more infomation about a command
`

	fmt.Fprintf(os.Stderr, "%s\n", text)
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func readOrFail(f *string) map[string]interface{} {
	m, err := vals.Input(*f)
	if err != nil {
		fatal("%v", err)
	}
	return m
}

func writeOrFail(o *string, res interface{}) {
	out, err := vals.Output(*o, res)
	if err != nil {
		fatal("%v", err)
	}
	fmt.Printf("%s", *out)
}

func main() {
	flag.Usage = flagUsage

	CmdEval := "eval"
	CmdExec := "exec"
	CmdEnv := "env"
	CmdKsDecode := "ksdecode"

	if len(os.Args) == 1 {
		flag.Usage()
		return
	}

	switch os.Args[1] {
	case CmdEval:
		evalCmd := flag.NewFlagSet(CmdEval, flag.ExitOnError)
		f := evalCmd.String("f", "-", "YAML/JSON file to be evaluated. When set to \"-\", vals reads from STDIN")
		o := evalCmd.String("o", "yaml", "Output type which is either \"yaml\" or \"json\"")
		e := evalCmd.Bool("exclude-secret", false, "Leave secretref+<uri> as-is and only replace ref+<uri>")
		evalCmd.Parse(os.Args[2:])

		m := readOrFail(f)

		res, err := vals.Eval(m, vals.Options{ExcludeSecret: *e})
		if err != nil {
			fatal("%v", err)
		}

		writeOrFail(o, res)
	case CmdExec:
		execCmd := flag.NewFlagSet(CmdExec, flag.ExitOnError)
		f := execCmd.String("f", "", "YAML/JSON file to be loaded to set envvars")
		execCmd.Parse(os.Args[2:])

		m := readOrFail(f)

		err := vals.Exec(m, execCmd.Args())
		if err != nil {
			fatal("%v", err)
		}
	case CmdEnv:
		execEnv := flag.NewFlagSet(CmdEnv, flag.ExitOnError)
		f := execEnv.String("f", "", "YAML/JSON file to be loaded to set envvars")
		export := execEnv.Bool("export", false, "Prepend `export`s to each line")
		execEnv.Parse(os.Args[2:])

		m := readOrFail(f)

		env, err := vals.Env(m)
		if err != nil {
			fatal("%v", err)
		}
		for _, l := range env {
			if *export {
				l = "export " + l
			}
			fmt.Fprintln(os.Stdout, l)
		}
	case CmdKsDecode:
		evalCmd := flag.NewFlagSet(CmdKsDecode, flag.ExitOnError)
		f := evalCmd.String("f", "", "YAML/JSON file to be decoded")
		o := evalCmd.String("o", "yaml", "Output type which is either \"yaml\" or \"json\"")
		evalCmd.Parse(os.Args[2:])

		nodes, err := vals.Inputs(*f)
		if err != nil {
			fatal("%v", err)
		}

		var res []yaml.Node
		for _, node := range nodes {
			n, err := KsDecode(node)
			if err != nil {
				fatal("%v", err)
			}
			res = append(res, *n)
		}

		for i, node := range res {
			buf := &bytes.Buffer{}
			encoder := yaml.NewEncoder(buf)
			encoder.SetIndent(2)

			if err := encoder.Encode(&node); err != nil {
				fatal("%v", err)
			}
			if *o == "json" {
				var v interface{}
				if err := json.Unmarshal(buf.Bytes(), &v); err != nil {
					fatal("%v", err)
				}
				bs, err := json.Marshal(v)
				if err != nil {
					fatal("%v", err)
				}
				print(string(bs))
			} else {
				print(buf.String())
			}
			if i != len(res)-1 {
				fmt.Println("---")
			}
		}
	default:
		flag.Usage()
	}
}

func KsDecode(node yaml.Node) (*yaml.Node, error) {
	if node.Kind != yaml.DocumentNode {
		return nil, fmt.Errorf("unexpected kind of node: expected %d, got %d", yaml.DocumentNode, node.Kind)
	}

	var res yaml.Node
	res = node

	var kk yaml.Node
	var vv yaml.Node
	var ii int

	isSecret := false
	mappings := node.Content[0].Content
	for i := 0; i < len(mappings); i += 2 {
		j := i + 1
		k := mappings[i]
		v := mappings[j]

		if k.Value == "kind" && v.Value == "Secret" {
			isSecret = true
		}

		if k.Value == "data" {
			ii = i
			kk = *k
			vv = *v
		}
	}

	if isSecret {
		kk.Value = "stringData"

		v := vv
		nestedMappings := v.Content
		v.Content = make([]*yaml.Node, len(v.Content))
		for i := 0; i < len(nestedMappings); i += 2 {
			b64 := nestedMappings[i+1].Value
			decoded, err := base64.StdEncoding.DecodeString(b64)
			if err != nil {
				return nil, err
			}
			nestedMappings[i+1].Value = string(decoded)

			v.Content[i] = nestedMappings[i]
			v.Content[i+1] = nestedMappings[i+1]
		}

		res.Content[0].Content[ii] = &kk
		res.Content[0].Content[ii+1] = &v
	}

	return &res, nil
}
