package expansion

import (
	"fmt"
	"regexp"
	"strings"
)

type ExpandRegexMatch struct {
	Target *regexp.Regexp
	Lookup func(string) (string, error)
	Only   []string
}

var DefaultRefRegexp = regexp.MustCompile(`((secret)?ref)\+([^\+:]*://[^\+]+)\+?`)

func (e *ExpandRegexMatch) InString(s string) (string, error) {
	var sb strings.Builder
	for {
		ixs := e.Target.FindStringSubmatchIndex(s)
		if ixs == nil {
			sb.WriteString(s)
			return sb.String(), nil
		}
		kind := s[ixs[2]:ixs[3]]
		if len(e.Only) > 0 {
			var shouldExpand bool
			for _, k := range e.Only {
				if k == kind {
					shouldExpand = true
					break
				}
			}
			if !shouldExpand {
				sb.WriteString(s)
				return sb.String(), nil
			}
		}
		ref := s[ixs[6]:ixs[7]]
		val, err := e.Lookup(ref)
		if err != nil {
			return "", fmt.Errorf("expand %s: %v", ref, err)
		}
		sb.WriteString(s[:ixs[0]])
		sb.WriteString(val)
		s = s[ixs[1]:]
	}
}

func (e *ExpandRegexMatch) InMap(target map[string]interface{}) (map[string]interface{}, error) {
	ret, err := ModifyStringValues(target, func(p string) (interface{}, error) {
		ret, err := e.InString(p)
		if err != nil {
			return nil, err
		}
		return ret, nil
	})

	if err != nil {
		return nil, err
	}

	switch ret.(type) {
	case map[string]interface{}:
		return ret.(map[string]interface{}), nil
	default:
		return nil, fmt.Errorf("unexpected type: %v: %T", ret, ret)
	}
}
