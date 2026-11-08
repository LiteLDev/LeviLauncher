package migrate

import (
	"encoding/json"
	"fmt"

	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
)

// RenderDocument renders every string of a package file document (the value
// model of package doc) as a Scriban template, as lip's PackageTemplates does
// (protocol/package.md §3): object keys are not rendered, the variables are
// the top-level members as written, so a template in a member is never
// rendered before another template reads it. The input is not modified.
//
// Only the subset of Scriban described at renderTemplate is supported; any
// other construct, and any template error, is returned as an error.
func RenderDocument(root any) (any, error) {
	obj, ok := root.(*doc.Object)
	if !ok {
		return root, nil
	}
	model := fromDoc(obj)
	scope := func(name string) (*jvalue, bool) { return model.get(name) }
	out, err := renderWith(model, scope)
	if err != nil {
		return nil, err
	}
	return toDoc(out), nil
}

func renderWith(v *jvalue, scope templateScope) (*jvalue, error) {
	switch v.kind {
	case jString:
		s, err := renderTemplate(v.str, scope)
		if err != nil {
			return nil, err
		}
		return jstr(s), nil
	case jArray:
		out := &jvalue{kind: jArray, arr: make([]*jvalue, len(v.arr))}
		for i, e := range v.arr {
			r, err := renderWith(e, scope)
			if err != nil {
				return nil, err
			}
			out.arr[i] = r
		}
		return out, nil
	case jObject:
		out := &jvalue{kind: jObject, obj: make([]jmember, len(v.obj))}
		for i, m := range v.obj {
			r, err := renderWith(m.val, scope)
			if err != nil {
				return nil, err
			}
			out.obj[i] = jmember{key: m.key, val: r}
		}
		return out, nil
	}
	return v, nil
}

func fromDoc(v any) *jvalue {
	switch t := v.(type) {
	case nil:
		return jnull()
	case bool:
		return &jvalue{kind: jBool, b: t}
	case json.Number:
		return &jvalue{kind: jNumber, str: string(t)}
	case string:
		return jstr(t)
	case []any:
		out := &jvalue{kind: jArray, arr: make([]*jvalue, len(t))}
		for i, e := range t {
			out.arr[i] = fromDoc(e)
		}
		return out
	case *doc.Object:
		out := &jvalue{kind: jObject, obj: make([]jmember, 0, t.Len())}
		for _, k := range t.Keys() {
			m, _ := t.Get(k)
			out.obj = append(out.obj, jmember{key: k, val: fromDoc(m)})
		}
		return out
	}
	panic(fmt.Sprintf("migrate: unexpected document value %T", v))
}

func toDoc(v *jvalue) any {
	switch v.kind {
	case jBool:
		return v.b
	case jNumber:
		return json.Number(v.str)
	case jString:
		return v.str
	case jArray:
		out := make([]any, len(v.arr))
		for i, e := range v.arr {
			out[i] = toDoc(e)
		}
		return out
	case jObject:
		o := doc.NewObject()
		for _, m := range v.obj {
			o.Set(m.key, toDoc(m.val))
		}
		return o
	}
	return nil
}
