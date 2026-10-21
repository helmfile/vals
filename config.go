package vals

import (
	"fmt"
)

func setValue(m map[string]any, v any, path ...string) error {
	var cur any

	cur = m

	for i, k := range path {
		if i == len(path)-1 {
			switch typed := cur.(type) {
			case map[string]any:
				typed[k] = v
			case map[any]any:
				typed[k] = v
			default:
				return fmt.Errorf("unexpected type: key=%v, value=%v, type=%T", path[:i+1], typed, typed)
			}
			return nil
		} else {
			switch typed := cur.(type) {
			case map[string]any:
				if _, ok := typed[k]; !ok {
					typed[k] = map[string]any{}
				}
				cur = typed[k]
			case map[any]any:
				if _, ok := typed[k]; !ok {
					typed[k] = map[string]any{}
				}
				cur = typed[k]
			default:
				return fmt.Errorf("unexpected type: key=%v, value=%v, type=%T", path[:i+1], typed, typed)
			}
		}
	}

	panic("invalid state")
}
