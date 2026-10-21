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

const maxNestingDepth = 10

// refPrefixRegexp detects ref+ or secretref+ prefixes to identify potential nesting.
var refPrefixRegexp = regexp.MustCompile(`(secret)?ref\+`)

func (e *ExpandRegexMatch) shouldExpand(kind string) bool {
	return len(e.Only) == 0 || slices.Contains(e.Only, kind)
}

// resolveInnerRefs finds nested ref+ expressions and resolves them inside-out.
// For example, ref+echo://ref+envsubst://$VAR/path will first resolve the inner
// ref+envsubst expression, then the outer ref+echo expression.
func (e *ExpandRegexMatch) resolveInnerRefs(s string, depth int) (string, error) {
	if depth >= maxNestingDepth {
		return "", fmt.Errorf("maximum nesting depth (%d) exceeded", maxNestingDepth)
	}

	positions := refPrefixRegexp.FindAllStringIndex(s, -1)
	if len(positions) <= 1 {
		return s, nil
	}

	// Find the rightmost ref that is actually nested (no separator between it
	// and the preceding ref+ prefix). Work from innermost outward.
	for i := len(positions) - 1; i >= 1; i-- {
		between := s[positions[i-1][1]:positions[i][0]]
		if strings.ContainsAny(between, " \n\r\t\",") {
			continue // Independent ref, not nested
		}

		// This ref is nested — resolve it
		start := positions[i][0]
		substring := s[start:]
		ixs := e.Target.FindStringSubmatchIndex(substring)
		if ixs == nil {
			continue
		}
		kind := substring[ixs[2]:ixs[3]]
		if !e.shouldExpand(kind) {
			continue
		}

		ref := substring[ixs[6]:ixs[7]]
		val, err := e.Lookup(ref)
		if err != nil {
			return "", fmt.Errorf("expand %s: %v", ref, err)
		}

		// Nested refs become part of an outer URI, so they must resolve to scalar values.
		if val == nil {
			return "", fmt.Errorf("nested ref %s resolved to nil", ref)
		}
		switch val.(type) {
		case string, bool,
			int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64,
			float32, float64:
			// Scalar types that format cleanly into a URI.
		default:
			return "", fmt.Errorf("nested ref %s resolved to %T; nested refs must resolve to scalar values", ref, val)
		}

		replaceStart := start + ixs[0]
		replaceEnd := start + ixs[1]
		result := s[:replaceStart] + fmt.Sprintf("%v", val) + s[replaceEnd:]
		return e.resolveInnerRefs(result, depth+1)
	}

	return s, nil
}

func (e *ExpandRegexMatch) InString(s string) (string, error) {
	s, err := e.resolveInnerRefs(s, 0)
	if err != nil {
		return "", err
	}

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
	resolved, err := e.resolveInnerRefs(s, 0)
	if err != nil {
		return nil, err
	}
	s = resolved

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
