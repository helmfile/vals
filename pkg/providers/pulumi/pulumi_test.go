package pulumi

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_parsePulumiResourceType(t *testing.T) {
	testcases := []struct {
		str  string
		want string
	}{
		{
			str:  "a_b_c",
			want: "a:b:c",
		},
		{
			str:  "a.b__c",
			want: "a.b/c",
		},
		{
			str:  "a.b__c.d",
			want: "a.b/c.d",
		},
		{
			str:  "a_b.c.d__e_f",
			want: "a:b.c.d/e:f",
		},
	}

	for i := range testcases {
		tc := testcases[i]
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			got := parsePulumiResourceType(tc.str)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("unexpected result: -(want), +(got)\n%s", diff)
			}
		})
	}
}

func Test_pulumiState_findResourceByLogicalName(t *testing.T) {
	testcases := []struct {
		want                *pulumiResource
		state               *pulumiState
		resourceType        string
		resourceLogicalName string
	}{
		{
			resourceType:        "provider:resource:TypeA",
			resourceLogicalName: "resource-name-b",
			want: &pulumiResource{
				URN:          "urn:pulumi:stack::project::provider:resource:Type::resource-name-b",
				Custom:       false,
				ID:           "b",
				ResourceType: "provider:resource:TypeA",
				Inputs:       json.RawMessage(`{"foo":"bar"}`),
				Outputs:      json.RawMessage(`{"baz":"qux"}`),
			},
			state: &pulumiState{
				Deployment: pulumiDeployment{
					Resources: []pulumiResource{
						{
							URN:          "urn:pulumi:stack::project::provider:resource:Type::resource-name-a",
							Custom:       false,
							ID:           "a",
							ResourceType: "provider:resource:TypeA",
							Inputs:       json.RawMessage(`{"foo":"bar"}`),
							Outputs:      json.RawMessage(`{"baz":"qux"}`),
						},
						{
							URN:          "urn:pulumi:stack::project::provider:resource:Type::resource-name-b",
							Custom:       false,
							ID:           "b",
							ResourceType: "provider:resource:TypeA",
							Inputs:       json.RawMessage(`{"foo":"bar"}`),
							Outputs:      json.RawMessage(`{"baz":"qux"}`),
						},
						{
							URN:          "urn:pulumi:stack::project::provider:resource:Type::resource-name-c",
							Custom:       false,
							ID:           "c",
							ResourceType: "provider:resource:TypeB",
							Inputs:       json.RawMessage(`{"foo":"bar"}`),
							Outputs:      json.RawMessage(`{"baz":"qux"}`),
						},
					},
				},
			},
		},
		{
			resourceType:        "provider:resource:TypeB",
			resourceLogicalName: "bogus-resource-name",
			want:                nil,
			state: &pulumiState{
				Deployment: pulumiDeployment{
					Resources: []pulumiResource{},
				},
			},
		},
	}

	for i := range testcases {
		tc := testcases[i]
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			got := tc.state.findResourceByLogicalName(tc.resourceType, tc.resourceLogicalName)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("unexpected result: -(want), +(got)\n%s", diff)
			}
		})
	}
}

func Test_pulumiResource_getAttributeValue(t *testing.T) {
	testcases := []struct {
		pulumiResource        *pulumiResource
		resourceAttribute     string
		resourceAttributePath string
		want                  string
	}{
		{
			resourceAttribute:     "outputs",
			resourceAttributePath: "baz.#(key==key2).value",
			want:                  "value2",
			pulumiResource: &pulumiResource{
				URN:          "urn:pulumi:stack::project::provider:resource:Type::resource-name-b",
				Custom:       false,
				ID:           "b",
				ResourceType: "provider:resource:TypeA",
				Inputs:       json.RawMessage(`{"foo":"bar"}`),
				Outputs:      json.RawMessage(`{"baz":[{"key":"key1","value":"value1"},{"key":"key2","value":"value2"},{"key":"key3","value":"value3"}]}`),
			},
		},
		{
			resourceAttribute:     "inputs",
			resourceAttributePath: `foo`,
			want:                  `bar`,
			pulumiResource: &pulumiResource{
				URN:          "urn:pulumi:stack::project::provider:resource:Type::resource-name-a",
				Custom:       false,
				ID:           "a",
				ResourceType: "provider:resource:TypeA",
				Inputs:       json.RawMessage(`{"foo":"bar"}`),
				Outputs:      json.RawMessage(`{"baz":"qux"}`),
			},
		},
	}

	for i := range testcases {
		tc := testcases[i]
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			got := tc.pulumiResource.getAttributeValue(tc.resourceAttribute, tc.resourceAttributePath)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("unexpected result: -(want), +(got)\n%s", diff)
			}
		})
	}
}
