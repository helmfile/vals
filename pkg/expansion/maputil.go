package expansion

import (
	"fmt"
	"maps"

	"gopkg.in/yaml.v3"
)

func ModifyStringValues(v any, f func(path string) (any, error)) (any, error) {
	merge := func(strmap map[string]any, k string, opts any) (bool, error) {
		switch opts.(type) {
		case map[string]any, map[any]any:
		default:
			return false, nil
		}

		k2, err := ModifyStringValues(k, f)
		if err != nil {
			return false, err
		}
		switch yamlOrJson := k2.(type) {
		case string:
			if yamlOrJson == k {
				break
			}
			m := map[string]any{}
			if err := yaml.Unmarshal([]byte(yamlOrJson), &m); err != nil {
				return false, err
			}
			// opts := v
			maps.Copy(strmap, m)
			return true, nil
		}
		return false, nil
	}

	var casted_v any
	switch typed_v := v.(type) {
	case string:
		return f(typed_v)
	case map[any]any:
		strmap := map[string]any{}
		for k, v := range typed_v {
			strmap[fmt.Sprintf("%v", k)] = v
		}
		extends := map[string]any{}
		var deleted []string
		for k, v := range strmap {
			ok, err := merge(extends, k, v)
			if ok {
				deleted = append(deleted, k)
				continue
			}
			if err != nil {
				return nil, err
			}

			v2, err := ModifyStringValues(v, f)
			if err != nil {
				return nil, err
			}
			strmap[k] = v2
		}
		for _, k := range deleted {
			delete(strmap, k)
		}
		maps.Copy(strmap, extends)
		return strmap, nil
	case map[string]any:
		extends := map[string]any{}
		var deleted []string
		for k, v := range typed_v {
			ok, err := merge(extends, k, v)
			if err != nil {
				return nil, err
			}
			if ok {
				deleted = append(deleted, k)
				continue
			}

			v2, err := ModifyStringValues(v, f)
			if err != nil {
				return nil, err
			}
			typed_v[k] = v2
		}
		for _, k := range deleted {
			delete(typed_v, k)
		}
		maps.Copy(typed_v, extends)
		return typed_v, nil
	case []any:
		a := []any{}
		for i := range typed_v {
			res, err := ModifyStringValues(typed_v[i], f)
			if err != nil {
				return nil, err
			}
			a = append(a, res)
		}
		casted_v = a
	case []string:
		a := []any{}
		for i := range typed_v {
			res, err := f(typed_v[i])
			if err != nil {
				return nil, err
			}
			a = append(a, res)
		}
		casted_v = a
	default:
		casted_v = typed_v
	}
	return casted_v, nil
}
