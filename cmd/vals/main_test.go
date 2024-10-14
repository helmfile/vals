package main

import (
	"bytes"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestKsDecode(t *testing.T) {
	in := `data:
  foo: Rk9P
stringData:
  bar: BAR
kind: Secret
`
	outExpected := `stringData:
  bar: BAR
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

func TestKsEncode(t *testing.T) {
	in := `stringData:
  foo: FOO
kind: Secret
`
	outExpected := `data:
  foo: Rk9P
kind: Secret
`
	var inNode yaml.Node
	if err := yaml.Unmarshal([]byte(in), &inNode); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	outNode, err := KsEncode(inNode)
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
