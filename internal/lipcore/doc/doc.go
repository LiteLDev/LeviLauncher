// Package doc holds the JSON value model lip 0.38 works on and the file
// formats that carry it: strict JSON (PackageFileParser.ParseJson), YAML files
// (protocol/README.md "YAML file", src/Lip.Core/Json/YamlJson.cs) and the
// System.Text.Json text lip writes for journal records
// (Mcmod/Install/WorkspaceTransaction.cs, JsonObject.ToJsonString).
//
// A value is one of
//
//	nil          JSON null
//	bool         JSON true / false
//	json.Number  a JSON number, as its JSON text
//	string       a JSON string
//	[]any        a JSON array
//	*Object      a JSON object, which keeps its members in insertion order
//
// The YAML reader is a port of the YamlDotNet 16.3.0 scanner and parser (the
// library lip uses) rather than a wrapper of gopkg.in/yaml.v3: yaml.v3's
// public API drops the non-specific tag "!" (so "! 5" would read as a number),
// rejects the surrogate pair escapes YamlDotNet accepts (a \u escape of a high
// surrogate followed by one of a low surrogate) and reports syntax errors
// without a column, so it cannot match lip.
package doc

import (
	"encoding/json"
	"fmt"
)

// Object is a JSON object whose members keep the order they were added in,
// as System.Text.Json's JsonObject does. Keys are compared ordinally. The zero
// value is an empty object ready to use.
type Object struct {
	keys  []string
	index map[string]int
	vals  []any
}

// NewObject returns an empty object.
func NewObject() *Object { return &Object{} }

// Keys returns the member names in order. The caller must not modify it.
func (o *Object) Keys() []string {
	if o == nil {
		return nil
	}
	return o.keys
}

// Len is the number of members.
func (o *Object) Len() int {
	if o == nil {
		return 0
	}
	return len(o.keys)
}

// Get returns the value of member k.
func (o *Object) Get(k string) (any, bool) {
	if o == nil {
		return nil, false
	}
	i, ok := o.index[k]
	if !ok {
		return nil, false
	}
	return o.vals[i], true
}

// Has reports whether member k exists.
func (o *Object) Has(k string) bool {
	_, ok := o.Get(k)
	return ok
}

// Set sets member k to v: an existing member keeps its position, a new one is
// added at the end. v must be one of the value types of this package (nil,
// bool, json.Number, string, []any, *Object); Set panics otherwise.
func (o *Object) Set(k string, v any) {
	switch v.(type) {
	case nil, bool, json.Number, string, []any, *Object:
	default:
		panic(fmt.Sprintf("doc: Object.Set(%q): %T is not a JSON value", k, v))
	}
	if i, ok := o.index[k]; ok {
		o.vals[i] = v
		return
	}
	if o.index == nil {
		o.index = make(map[string]int)
	}
	o.index[k] = len(o.keys)
	o.keys = append(o.keys, k)
	o.vals = append(o.vals, v)
}

// Delete removes member k, keeping the order of the others. It reports
// whether the member existed.
func (o *Object) Delete(k string) bool {
	i, ok := o.index[k]
	if !ok {
		return false
	}
	// New backing arrays, so a Keys() slice taken before stays intact.
	o.keys = append(o.keys[:i:i], o.keys[i+1:]...)
	o.vals = append(o.vals[:i:i], o.vals[i+1:]...)
	delete(o.index, k)
	for j := i; j < len(o.keys); j++ {
		o.index[o.keys[j]] = j
	}
	return true
}

// Clone returns a deep copy of the object.
func (o *Object) Clone() *Object {
	if o == nil {
		return nil
	}
	c := &Object{
		keys:  append([]string(nil), o.keys...),
		vals:  make([]any, len(o.vals)),
		index: make(map[string]int, len(o.keys)),
	}
	for i, v := range o.vals {
		c.vals[i] = Clone(v)
	}
	for k, i := range o.index {
		c.index[k] = i
	}
	return c
}

// Clone returns a deep copy of a value.
func Clone(v any) any {
	switch t := v.(type) {
	case *Object:
		return t.Clone()
	case []any:
		if t == nil {
			return []any(nil)
		}
		c := make([]any, len(t))
		for i, e := range t {
			c[i] = Clone(e)
		}
		return c
	default:
		return v
	}
}

// Equal reports whether two values are the same JSON value: objects with the
// same members (in any order) and equal values, arrays with equal items in
// order, and numbers with the same JSON text.
func Equal(a, b any) bool {
	switch x := a.(type) {
	case *Object:
		y, ok := b.(*Object)
		if !ok || x.Len() != y.Len() {
			return false
		}
		for i, k := range x.Keys() {
			w, ok := y.Get(k)
			if !ok || !Equal(x.vals[i], w) {
				return false
			}
		}
		return true
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !Equal(x[i], y[i]) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}
