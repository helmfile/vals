package sprucemerge

import (
	"github.com/geofffranks/yaml"
	"testing"
)

func TestMerge(t *testing.T) {
	a := map[string]interface{}{}
	b := map[string]interface{}{}
	yamlA := []byte(`some_data: this will be overwritten later
a_random_map:
  key1: some data
heres_an_array:
- first element
`)
	yamlB := []byte(`some_data: 42
a_random_map:
  key2: adding more data
heres_an_array:
- (( prepend ))
- zeroth element
more_data: 84
`)
	if err := yaml.Unmarshal(yamlA, &a); err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(yamlB, &b); err != nil {
		t.Fatal(err)
	}
	p := &provider{}
	res, err := p.Merge(a, b)
	if err != nil {
		t.Fatal(err)
	}
	println(res)
}
