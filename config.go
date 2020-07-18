package vals

import (
	"fmt"
)

func setValue(m map[string]interface{}, v interface{}, path ...string) error {
	var cur interface{}

	cur = m

	for i, k := range path {
		if i == len(path)-1 {
			switch typed := cur.(type) {
			case map[string]interface{}:
				typed[k] = v
			case map[interface{}]interface{}:
				typed[k] = v
			default:
				return fmt.Errorf("unexpected type: key=%v, value=%v, type=%T", path[:i+1], typed, typed)
			}
			return nil
		} else {
			switch typed := cur.(type) {
			case map[string]interface{}:
				if _, ok := typed[k]; !ok {
					typed[k] = map[string]interface{}{}
				}
				cur = typed[k]
			case map[interface{}]interface{}:
				if _, ok := typed[k]; !ok {
					typed[k] = map[string]interface{}{}
				}
				cur = typed[k]
			default:
				return fmt.Errorf("unexpected type: key=%v, value=%v, type=%T", path[:i+1], typed, typed)
			}
		}
	}

	panic("invalid state")
}
