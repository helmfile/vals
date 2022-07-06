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
