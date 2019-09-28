package main

import (
	"bytes"
	"gopkg.in/yaml.v3"
	"testing"
)

func TestKsDecode(t *testing.T) {
	in := `data:
  foo: Rk9P
kind: Secret
`
	outExpected := `stringData:
  foo: FOO
kind: Secret
`
	var inNode yaml.Node
	if err := yaml.Unmarshal([]byte(in), &inNode); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	outNode, err := KsDecode(inNode)
	if err != nil {
		t.Fatalf("ksdecode: %v", err)
	}

	buf := &bytes.Buffer{}
	encoder := yaml.NewEncoder(buf)
	encoder.SetIndent(2)

	if err := encoder.Encode(outNode); err != nil {
		t.Fatalf("marshal: %v", err)
	}

	outActual := buf.String()

	if outActual != outExpected {
		t.Errorf("unexpected out: expected=%s, got=%s", outExpected, outActual)
	}
}
