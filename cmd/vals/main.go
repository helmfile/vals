package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/helmfile/vals"
)

var (
	version string
	commit  string
)

func flagUsage() {
	text := `vals is a Helm-like configuration "Values" loader with support for various sources and merge strategies

Usage:
  vals [command]

Available Commands:
  eval		Evaluate a JSON/YAML document and replace any template expressions in it and prints the result
  exec		Populates the environment variables and executes the command
  env		Renders environment variables to be consumed by eval or a tool like direnv
  get		Evaluate a string value passed as the first argument and replace any expressions in it and prints the result
  ksdecode	Decode YAML document(s) by converting Secret resources' "data" to "stringData" for use with "vals eval"
  version	Print vals version

Use "vals [command] --help" for more information about a command
`

	fmt.Fprintf(os.Stderr, "%s\n", text)
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func readNodesOrFail(f *string) []yaml.Node {
	nodes, err := vals.Inputs(*f)
	if err != nil {
		fatal("%v", err)
	}
	return nodes
}

func readOrFail(f *string) map[string]interface{} {
	nodes := readNodesOrFail(f)
	if len(nodes) == 0 {
		fatal("no document found")
	}
	var nodeValue map[string]interface{}
	err := nodes[0].Decode(&nodeValue)
	if err != nil {
		fatal("%v", err)
	}
	return nodeValue
}

func writeOrFail(o *string, nodes []yaml.Node) {
	err := vals.Output(os.Stdout, *o, nodes)
	if err != nil {
		fatal("%v", err)
	}
}

func main() {
	flag.Usage = flagUsage

	CmdEval := "eval"
	CmdGet := "get"
	CmdExec := "exec"
	CmdEnv := "env"
	CmdKsDecode := "ksdecode"
	CmdVersion := "version"

	if len(os.Args) == 1 {
		flag.Usage()
		return
	}

	switch os.Args[1] {
	case CmdEval:
		evalCmd := flag.NewFlagSet(CmdEval, flag.ExitOnError)
		f := evalCmd.String("f", "-", "YAML/JSON file to be evaluated. When set to \"-\", vals reads from STDIN")
		o := evalCmd.String("o", "yaml", "Output type which is either \"yaml\" or \"json\"")
		silent := evalCmd.Bool("s", false, "Silent mode")
		e := evalCmd.Bool("exclude-secret", false, "Leave secretref+<uri> as-is and only replace ref+<uri>")
		failOnMissingKeyInMap := evalCmd.Bool("fail-on-missing-key-in-map", true, "When set to false, the vals-eval command exits with code 0 even when the key denoted by the #/key/for/value/in/the/json/or/yaml does not exist in the decoded map")
		err := evalCmd.Parse(os.Args[2:])
		if err != nil {
			fatal("%v", err)
		}

		var logOut io.Writer = os.Stderr
		if *silent {
			logOut = io.Discard
		}

		nodes := readNodesOrFail(f)

		res, err := vals.EvalNodes(nodes, vals.Options{
			ExcludeSecret:         *e,
			LogOutput:             logOut,
			FailOnMissingKeyInMap: *failOnMissingKeyInMap,
		})
		if err != nil {
			fatal("%v", err)
		}

		writeOrFail(o, res)
	case CmdGet:
		getCmd := flag.NewFlagSet(CmdGet, flag.ExitOnError)
		silent := getCmd.Bool("s", false, "Silent mode")
		err := getCmd.Parse(os.Args[2:])
		if err != nil {
			fatal("%v", err)
		}

		code := getCmd.Arg(0)
		if code == "" {
			fatal("The first argument of the get command is required")
		}

		var logOut io.Writer = os.Stderr
		if *silent {
			logOut = io.Discard
		}

		v, err := vals.Get(code, vals.Options{LogOutput: logOut})
		if err != nil {
			fatal("%v", err)
		}

		_, err = os.Stdout.WriteString(v)
		if err != nil {
			fatal("%v", err)
		}
	case CmdExec:
		execCmd := flag.NewFlagSet(CmdExec, flag.ExitOnError)
		f := execCmd.String("f", "", "YAML/JSON file to be loaded to set envvars")
		inheritEnv := execCmd.Bool("i", false, "Inherit environment variables")
		silent := execCmd.Bool("s", false, "Silent mode")
		streamYAML := execCmd.String("stream-yaml", "", `Reads the specific YAML file or all the YAML files
stored within the specific directory, evaluate each YAML file,
joining all the YAML files with "---" lines, and stream the
result into the stdin of the executed command.
This is handy when you want to use vals to preprocess
Kubernetes manifests to kubectl-apply, without writing
the vals-eval outputs onto the disk, for security reasons.`)
		err := execCmd.Parse(os.Args[2:])
		if err != nil {
			fatal("%v", err)
		}

		var m map[string]interface{}

		if *f != "" {
			m = readOrFail(f)
		} else {
			m = map[string]interface{}{}
		}

		var logOut io.Writer = os.Stderr
		if *silent {
			logOut = io.Discard
		}

		err = vals.Exec(m, execCmd.Args(), vals.ExecConfig{
			InheritEnv: *inheritEnv,
			Options:    vals.Options{LogOutput: logOut},
			StreamYAML: *streamYAML,
		})
		if err != nil {
			fatal("%v", err)
		}
	case CmdEnv:
		execEnv := flag.NewFlagSet(CmdEnv, flag.ExitOnError)
		f := execEnv.String("f", "", "YAML/JSON file to be loaded to set envvars")
		export := execEnv.Bool("export", false, "Prepend 'export' to each line")
		err := execEnv.Parse(os.Args[2:])
		if err != nil {
			fatal("%v", err)
		}

		m := readOrFail(f)

		env, err := vals.QuotedEnv(m)
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
		err := evalCmd.Parse(os.Args[2:])
		if err != nil {
			fatal("%v", err)
		}

		nodes := readNodesOrFail(f)

		var res []yaml.Node
		for _, node := range nodes {
			n, err := KsDecode(node)
			if err != nil {
				fatal("%v", err)
			}
			res = append(res, *n)
		}

		writeOrFail(o, res)
	case CmdVersion:
		if len(version) == 0 {
			fmt.Println("Version: dev")
		} else {
			fmt.Println("Version:", version)
		}
		fmt.Println("Git Commit:", commit)
	default:
		flag.Usage()
	}
}

func KsDecode(node yaml.Node) (*yaml.Node, error) {
	if node.Kind != yaml.DocumentNode {
		return nil, fmt.Errorf("unexpected kind of node: expected %d, got %d", yaml.DocumentNode, node.Kind)
	}

	var res yaml.Node = node

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

	if isSecret && !kk.IsZero() {
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
