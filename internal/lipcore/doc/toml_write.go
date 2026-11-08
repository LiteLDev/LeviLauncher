package doc

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// MarshalTOML writes a JSON object as lip writes TOML files
// (TomlJson.SerializeToUtf8Bytes, src/Lip.Core/Json/TomlJson.cs): a TOML
// document that reads back as the same value, ending with a line break, in
// UTF-8 without a byte order mark.
//
// Members that are scalars, arrays without tables, or empty objects come
// first as "key = value"; then each non-empty object member as a [table]
// (without a header of its own when all its members have one); then each
// array of objects, at least one of which holds a non-empty object or an
// array with an object, as an [[array of tables]]. Any other object is
// written inline. Keys are bare when they can be, otherwise basic strings.
//
// It fails when v is not an *Object, or holds null.
func MarshalTOML(v any) ([]byte, error) {
	o, ok := v.(*Object)
	if !ok {
		return nil, errors.New("A TOML document is a table. (Parameter 'value')")
	}
	var b strings.Builder
	if err := writeTOMLTable(&b, o, nil); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

func tomlKey(k string) string {
	if k == "" {
		return tomlQuote(k)
	}
	for i := 0; i < len(k); i++ {
		if !isBareKeyChar(rune(k[i])) {
			return tomlQuote(k)
		}
	}
	return k
}

func tomlPath(path []string) string {
	keys := make([]string, len(path))
	for i, k := range path {
		keys[i] = tomlKey(k)
	}
	return strings.Join(keys, ".")
}

// isTOMLTable is IsTable: a non-empty object.
func isTOMLTable(v any) bool {
	o, ok := v.(*Object)
	return ok && o.Len() > 0
}

// isTOMLTableArray is IsTableArray: a non-empty array of objects only, at
// least one of which holds a non-empty object or an array with an object.
func isTOMLTableArray(v any) bool {
	a, ok := v.([]any)
	if !ok || len(a) == 0 {
		return false
	}
	for _, item := range a {
		if _, ok := item.(*Object); !ok {
			return false
		}
	}
	for _, item := range a {
		o := item.(*Object)
		for _, k := range o.Keys() {
			m, _ := o.Get(k)
			if isTOMLTable(m) {
				return true
			}
			if arr, ok := m.([]any); ok {
				for _, e := range arr {
					if _, ok := e.(*Object); ok {
						return true
					}
				}
			}
		}
	}
	return false
}

func isTOMLHeaded(v any) bool { return isTOMLTable(v) || isTOMLTableArray(v) }

// separate writes a blank line before a header, except at the start.
func separate(b *strings.Builder) {
	if b.Len() > 0 {
		b.WriteByte('\n')
	}
}

func writeTOMLTable(b *strings.Builder, table *Object, path []string) error {
	var tables, tableArrays []string
	for _, k := range table.Keys() {
		v, _ := table.Get(k)
		switch {
		case isTOMLTable(v):
			tables = append(tables, k)
		case isTOMLTableArray(v):
			tableArrays = append(tableArrays, k)
		default:
			b.WriteString(tomlKey(k))
			b.WriteString(" = ")
			if err := writeTOMLInline(b, v); err != nil {
				return err
			}
			b.WriteByte('\n')
		}
	}
	for _, k := range tables {
		v, _ := table.Get(k)
		sub := v.(*Object)
		subPath := append(append([]string(nil), path...), k)
		headed := true
		for _, m := range sub.Keys() {
			mv, _ := sub.Get(m)
			if !isTOMLHeaded(mv) {
				headed = false
				break
			}
		}
		if !headed {
			separate(b)
			b.WriteString("[" + tomlPath(subPath) + "]\n")
		}
		if err := writeTOMLTable(b, sub, subPath); err != nil {
			return err
		}
	}
	for _, k := range tableArrays {
		v, _ := table.Get(k)
		subPath := append(append([]string(nil), path...), k)
		for _, item := range v.([]any) {
			separate(b)
			b.WriteString("[[" + tomlPath(subPath) + "]]\n")
			if err := writeTOMLTable(b, item.(*Object), subPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeTOMLInline(b *strings.Builder, v any) error {
	switch t := v.(type) {
	case *Object:
		if t.Len() == 0 {
			b.WriteString("{}")
			return nil
		}
		b.WriteString("{ ")
		for i, k := range t.Keys() {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(tomlKey(k))
			b.WriteString(" = ")
			m, _ := t.Get(k)
			if err := writeTOMLInline(b, m); err != nil {
				return err
			}
		}
		b.WriteString(" }")
	case []any:
		b.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				b.WriteString(", ")
			}
			if err := writeTOMLInline(b, e); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case string:
		b.WriteString(tomlQuote(t))
	case json.Number:
		b.WriteString(tomlNumber(string(t)))
	case bool:
		b.WriteString(strconv.FormatBool(t))
	case nil:
		return errors.New("TOML has no null. (Parameter 'value')")
	default:
		panic(fmt.Sprintf("doc: %T is not a JSON value", v))
	}
	return nil
}

// tomlNumber is Number: a JSON number as a TOML integer or float of the
// same value. One that JsonElement.TryGetInt64 reads is written as that
// integer; one with '.', 'e' or 'E' as written; an integer beyond 64 bits
// as double.ToString("R") writes it, with ".0" when that has neither 'E'
// nor '.'.
func tomlNumber(text string) string {
	if !strings.ContainsAny(text, ".eE") {
		if i, err := strconv.ParseInt(text, 10, 64); err == nil {
			return strconv.FormatInt(i, 10)
		}
	} else {
		return text
	}
	d, _ := strconv.ParseFloat(text, 64)
	s := formatDoubleR(d)
	if strings.ContainsAny(s, "E.") {
		return s
	}
	return s + ".0"
}

// formatDoubleR formats a double as double.ToString("R") does on .NET Core
// 3.0 and later: the shortest text that round-trips, in fixed notation when
// the decimal exponent is from -5 to 14, else as d.dddE+XX.
func formatDoubleR(f float64) string {
	if math.IsInf(f, 1) {
		return "∞"
	}
	if math.IsInf(f, -1) {
		return "-∞"
	}
	if math.IsNaN(f) {
		return "NaN"
	}
	if f == 0 {
		if math.Signbit(f) {
			return "-0"
		}
		return "0"
	}
	s := strconv.FormatFloat(f, 'e', -1, 64)
	mant, expText, _ := strings.Cut(s, "e")
	exp, _ := strconv.Atoi(expText)
	if exp >= -5 && exp < 15 {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	sign := "+"
	if exp < 0 {
		sign = "-"
		exp = -exp
	}
	e := strconv.Itoa(exp)
	if len(e) < 2 {
		e = "0" + e
	}
	return mant + "E" + sign + e
}

// tomlQuote is Quote: a TOML basic string, with ", \ and the control
// characters (U+0000 to U+001F and U+007F to U+009F) escaped.
func tomlQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, c := range s {
		switch c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\f':
			b.WriteString(`\f`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if c < 0x20 || (c >= 0x7F && c <= 0x9F) {
				fmt.Fprintf(&b, `\u%04X`, c)
			} else {
				b.WriteRune(c)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
