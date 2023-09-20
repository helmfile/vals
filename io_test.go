package vals

import (
	"bytes"
	"strings"
	"testing"
)

func Test_InputOutput(t *testing.T) {
	baseDocument := `foo:
  bar:
    - baz
`

	tests := []struct {
		name     string
		input    string
		format   string
		expected string
	}{
		{
			name:     "single document yaml",
			input:    baseDocument,
			format:   "yaml",
			expected: "foo:\n  bar:\n    - baz\n",
		},
		{
			name:     "multi document yaml",
			input:    baseDocument + "---\nbar: baz\n",
			format:   "yaml",
			expected: "foo:\n  bar:\n    - baz\n---\nbar: baz\n",
		},
		{
			name:     "single document json",
			input:    baseDocument,
			format:   "json",
			expected: "{\"foo\":{\"bar\":[\"baz\"]}}\n",
		},
		{
			name:     "multi document json",
			input:    baseDocument + "---\nbar: baz\n",
			format:   "json",
			expected: "{\"foo\":{\"bar\":[\"baz\"]}}\n---\n{\"bar\":\"baz\"}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes, err := nodesFromReader(strings.NewReader(tt.input))
			if err != nil {
				t.Fatal(err)
			}
			buf := &bytes.Buffer{}
			err = Output(buf, tt.format, nodes)
			if err != nil {
				t.Fatal(err)
			}

			if buf.String() != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, buf.String())
			}

			nodesRoundTrip, err := nodesFromReader(buf)
			if err != nil {
				t.Fatal(err)
			}

			bufRoundTrip := &bytes.Buffer{}
			err = Output(bufRoundTrip, "yaml", nodesRoundTrip)
			if err != nil {
				t.Fatal(err)
			}

			if bufRoundTrip.String() != tt.input {
				t.Errorf("Expected %q, got %q", tt.input, bufRoundTrip.String())
			}
		})
	}
}

func Test_NodesFromReader(t *testing.T) {
	simpleDocument := "---\nfoo: bar\n"
	commentDocument := "---\n# comment\n"

	tests := []struct {
		name  string
		input string
		nodes int
	}{
		{
			name:  "single document",
			input: simpleDocument,
			nodes: 1,
		},
		{
			name:  "multi document",
			input: simpleDocument + simpleDocument,
			nodes: 2,
		},
		{
			name:  "single comment document",
			input: commentDocument,
			nodes: 0,
		},
		{
			name:  "multiple comment document",
			input: commentDocument + commentDocument,
			nodes: 0,
		},
		{
			name:  "mixed documents",
			input: simpleDocument + commentDocument,
			nodes: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes, err := nodesFromReader(strings.NewReader(tt.input))
			if err != nil {
				t.Fatal(err)
			}

			if len(nodes) != tt.nodes {
				t.Errorf("Expected %v nodes, got %v", tt.nodes, len(nodes))
			}
		})
	}
}
