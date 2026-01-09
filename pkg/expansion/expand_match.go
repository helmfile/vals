package expansion

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

type ExpandRegexMatch struct {
	Target *regexp.Regexp
	Lookup func(string) (interface{}, error)
	Only   []string
}

var DefaultRefRegexp = regexp.MustCompile(`((secret)?ref)\+([^\+:]*:\/\/[^\+\n ]+[^\+\n ",])\+?`)

func (e *ExpandRegexMatch) shouldExpand(kind string) bool {
	return len(e.Only) == 0 || slices.Contains(e.Only, kind)
}

func (e *ExpandRegexMatch) InString(s string) (string, error) {
	var sb strings.Builder
	for {
		ixs := e.Target.FindStringSubmatchIndex(s)
		if ixs == nil {
			sb.WriteString(s)
			return sb.String(), nil
		}

		kind := s[ixs[2]:ixs[3]]
		if !e.shouldExpand(kind) {
			sb.WriteString(s)
			// FIXME: this skips the rest of the string, is this intended?
			return sb.String(), nil
		}

		ref := s[ixs[6]:ixs[7]]
		val, err := e.Lookup(ref)
		if err != nil {
			return "", fmt.Errorf("expand %s: %v", ref, err)
		}
		sb.WriteString(s[:ixs[0]])
		fmt.Fprintf(&sb, "%v", val)
		s = s[ixs[1]:]
	}
}

// InValue expands matches in the given string value.
// If the entire string matches the regex, it expands and preserves the type.
// If only part of the string matches, it expands as a string.
func (e *ExpandRegexMatch) InValue(s string) (interface{}, error) {
	ixs := e.Target.FindStringSubmatchIndex(s)
	switch {
	// No match, return as is
	case ixs == nil:
		return s, nil
	// Full match, expand preserving type
	case ixs[0] == 0 && ixs[1] == len(s):
		kind := s[ixs[2]:ixs[3]]
		ref := s[ixs[6]:ixs[7]]
		if !e.shouldExpand(kind) {
			return s, nil
		}
		val, err := e.Lookup(ref)
		if err != nil {
			return nil, fmt.Errorf("expand %s: %v", ref, err)
		}
		return val, nil
	// Partial match, expand as string
	default:
		return e.InString(s)
	}
}

func (e *ExpandRegexMatch) InMap(target map[string]interface{}) (map[string]interface{}, error) {
	ret, err := ModifyStringValues(target, func(p string) (interface{}, error) {
		ret, err := e.InValue(p)
		if err != nil {
			return nil, err
		}
		return ret, nil
	})
	if err != nil {
		return nil, err
	}

	switch ret := ret.(type) {
	case map[string]interface{}:
		return ret, nil
	default:
		return nil, fmt.Errorf("unexpected type: %v: %T", ret, ret)
	}
}
