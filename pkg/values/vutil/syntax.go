package vutil

import "fmt"

func evalUnaryExpr(tpe string, f func(string) (string, error)) (func(map[string]interface{}) (interface{}, bool, error)) {
	return func(v map[string]interface{}) (interface{}, bool, error) {
		ref, ok := v["$"+tpe]
		if !ok {
			return nil, false, nil
		}

		str, ok := ref.(string)
		if !ok {
			return nil, false, fmt.Errorf("unexpected $ref type: expected string, got %T", ref)
		}

		if len(str) == 0 {
			return nil, false, fmt.Errorf("unexpected $ref value: expected non empty string, but got %q", ref)
		}

		res, err := f(str)
		if err != nil {
			return nil, false, err
		}

		return res, true, nil
	}
}

func returnFirstValid(fs ...func(map[string]interface{}) (interface{}, bool, error)) func(map[string]interface{}) (interface{}, bool, error) {
	return func(obj map[string]interface{}) (interface{}, bool, error) {
		for _, f := range fs {
			ret, ok, err := f(obj)
			if err != nil {
				return nil, false, err
			}

			if !ok {
				continue
			}

			return ret, true, nil
		}

		return nil, false, nil
	}
}

func EvalUnaryExprWithTypes(tpe string, f func(string) (string, error)) func(map[string]interface{}) (interface{}, bool, error) {
	return func(obj map[string]interface{}) (interface{}, bool, error) {
		var eval func(map[string]interface{}) (interface{}, bool, error)

		evalRef := evalUnaryExpr(tpe, f)

		_types, ok := obj["$types"]
		if !ok {
			eval = evalRef
		} else {
			types := make(map[string]string)
			switch t := _types.(type) {
			case map[interface{}]interface{}:
				for k, v := range t {
					types[fmt.Sprintf("%v", k)] = fmt.Sprintf("%v", v)
				}
			case map[string]interface{}:
				for k, v := range t {
					types[k] = fmt.Sprintf("%v", v)
				}
			default:
				return nil, false, fmt.Errorf("unexpected type of $types: expected map[string]interface{} or map[interface{}]interface{}, got %T", _types)
			}

			var candidates []func(map[string]interface{}) (interface{}, bool, error)
			for k, p := range types {
				prefix := p
				cand := evalUnaryExpr(k, func(suffix string) (string, error) {
					return f(prefix + suffix)
				})
				candidates = append(candidates, cand)
			}
			candidates = append(candidates, evalRef)

			eval = returnFirstValid(candidates...)
		}

		input := make(map[string]interface{})
		for k, v := range obj {
			if k != "$types" {
				input[k] = v
			}
		}

		ret, err := ModifyMapValues(input, eval)
		if err != nil {
			return nil, false, err
		}

		return ret, true, nil
	}
}
