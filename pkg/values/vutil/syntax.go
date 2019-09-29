package vutil

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

func transformUnaryExpr(tpe, toType string, f func(string) (string, error)) (func(map[string]interface{}) (interface{}, bool, error)) {
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

		return map[string]interface{}{"$" + toType: res}, true, nil
	}
}

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

func EvaluateStringInterpolationLikeGoTemplateValues(v interface{}, funcs template.FuncMap) (interface{}, error) {
	return ModifyStringValues(v, func(p string) (interface{}, error) {
		var res string
		for {
			markStart := "{{"
			markEnd := "}}"
			controlMark := "$"
			start := strings.Index(p, controlMark+markStart)
			if start < 0 {
				res += p
				break
			}
			res += p[0:start]
			p = p[start+len(controlMark):]
			exprEnd := strings.Index(p, markEnd)
			if exprEnd < 0 {
				return nil, fmt.Errorf("missing closing }} in %q", p)
			}

			expr := p[0:exprEnd + len(markEnd)]

			tmpl := template.New("expr")
			tmpl = tmpl.Funcs(funcs)

			tmpl, err := tmpl.Parse(expr)
			if err != nil {
				return nil, err
			}

			buf := &bytes.Buffer{}
			if err := tmpl.Execute(buf, nil); err != nil {
				return nil, err
			}

			res += buf.String()

			p = p[exprEnd + len(markEnd):]
		}
		return res, nil
	})
}

func evalStrInterpolationExpr(funcs template.FuncMap) (func(map[string]interface{}) (interface{}, bool, error)) {
	return func(v map[string]interface{}) (interface{}, bool, error) {
		res, err := EvaluateStringInterpolationLikeGoTemplateValues(v, funcs)

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

func FlattenTypes(tpe string) func(map[string]interface{}) (interface{}, bool, error) {
	return func(obj map[string]interface{}) (interface{}, bool, error) {
		var eval func(map[string]interface{}) (interface{}, bool, error)

		evalRef := transformUnaryExpr(tpe, tpe, func(p string) (string, error) {
			return p, nil
		})

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
				cand := transformUnaryExpr(k, tpe, func(suffix string) (string, error) {
					return prefix + suffix, nil
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
