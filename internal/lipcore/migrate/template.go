package migrate

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// decimalMaxMantissa is System.Decimal's 96-bit mantissa limit.
var decimalMaxMantissa = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 96), big.NewInt(1))

// ErrUnsupportedTemplate is returned for Scriban constructs outside the
// subset implemented here. lip renders manifest strings with the full
// Scriban 7.5 language; real manifests only use variable references, so the
// rest is rejected loudly instead of being rendered differently.
var ErrUnsupportedTemplate = errors.New("migrate: unsupported Scriban template construct")

// errTemplate reports template errors that Scriban also reports (parse
// errors and runtime errors such as member access on null).
var errTemplate = errors.New("migrate: template error")

// templateScope resolves a top-level variable name; ok=false when the model
// has no such member.
type templateScope func(name string) (v *jvalue, ok bool)

// renderTemplate renders src like Scriban.Template.Parse(src).Render(model)
// for the subset of Scriban that tooth.json manifests use:
//
//   - text outside code blocks is copied verbatim ("}}" and single braces
//     are plain text);
//   - "{{ ... }}" code blocks (an unterminated "{{" runs to the end of the
//     string, as in Scriban) with whitespace control "{{-"/"-}}" (all
//     whitespace incl. newlines) and "{{~"/"~}}" (spaces/tabs; "~}}" also
//     eats one newline);
//   - "{%{ ... }%}" raw blocks;
//   - statements separated by newlines or ";", "#" comments;
//   - expressions: a variable ("name", "$local" which is always null),
//     "true"/"false"/"null", string literals ("..", '..', `..`), integer
//     literals, followed by any number of ".member" or "[index]" accessors.
//
// Semantics follow Scriban 7.5 with a Dictionary<string, JsonElement>/
// Dictionary<string, object> model (verified against Scriban 7.5.0):
// variable and member lookups are case-sensitive; unknown variables and
// missing members render as ""; ".size" on strings/arrays is the length;
// arrays accept negative indexes; member/index access on null is an error;
// nested objects/arrays render as Scriban literals (`{name: "N", x: [1]}`);
// numbers render the way .NET decimal does (`1.50`, `1e3` -> `1000`).
//
// Anything else (operators, pipes, function calls, control flow, builtin
// objects such as "string") returns ErrUnsupportedTemplate.
func renderTemplate(src string, scope templateScope) (string, error) {
	var out strings.Builder
	i := 0
	rightTrim := byte(0) // whitespace control of the previous tag: 0, '-', '~'
	for {
		next, raw := findTag(src, i)
		end := next
		if next < 0 {
			end = len(src)
		}
		text := src[i:end]
		text = trimLeft(text, rightTrim)
		rightTrim = 0
		if next >= 0 && !raw && next+2 < len(src) && (src[next+2] == '-' || src[next+2] == '~') {
			text = trimRight(text, src[next+2])
		}
		out.WriteString(text)
		if next < 0 {
			return out.String(), nil
		}
		if raw {
			start := next + 3
			k := strings.Index(src[start:], "}%}")
			if k < 0 {
				out.WriteString(src[start:])
				return out.String(), nil
			}
			out.WriteString(src[start : start+k])
			i = start + k + 3
			continue
		}
		start := next + 2
		if start < len(src) && (src[start] == '-' || src[start] == '~') {
			start++
		}
		closeAt := findCodeEnd(src, start)
		var code string
		if closeAt < 0 {
			code = src[start:]
			i = len(src)
		} else {
			code = src[start:closeAt]
			i = closeAt + 2
			if n := len(code); n > 0 && (code[n-1] == '-' || code[n-1] == '~') {
				rightTrim = code[n-1]
				code = code[:n-1]
			}
		}
		if err := evalCode(&out, code, scope); err != nil {
			return "", err
		}
		if closeAt < 0 {
			return out.String(), nil
		}
	}
}

// findTag returns the index of the next "{{" or "{%{" at or after i.
func findTag(s string, i int) (int, bool) {
	for j := i; j+1 < len(s); j++ {
		if s[j] != '{' {
			continue
		}
		if s[j+1] == '{' {
			return j, false
		}
		if s[j+1] == '%' && j+2 < len(s) && s[j+2] == '{' {
			return j, true
		}
	}
	return -1, false
}

// findCodeEnd returns the index of the "}}" closing the code block starting
// at i, skipping string literals and comments, or -1.
func findCodeEnd(s string, i int) int {
	for j := i; j < len(s); j++ {
		switch c := s[j]; c {
		case '"', '\'', '`':
			j = skipString(s, j)
			if j < 0 {
				return -1
			}
		case '#':
			for j+1 < len(s) && s[j+1] != '\n' && !(s[j+1] == '}' && j+2 < len(s) && s[j+2] == '}') {
				j++
			}
		case '}':
			if j+1 < len(s) && s[j+1] == '}' {
				return j
			}
		}
	}
	return -1
}

// skipString returns the index of the closing quote of the literal starting
// at s[i], or -1.
func skipString(s string, i int) int {
	q := s[i]
	for j := i + 1; j < len(s); j++ {
		if s[j] == '\\' && q != '`' {
			j++
			continue
		}
		if s[j] == q {
			return j
		}
	}
	return -1
}

func isInlineSpace(r rune) bool { return r != '\n' && r != '\r' && unicode.IsSpace(r) }

func trimLeft(text string, mode byte) string {
	switch mode {
	case '-':
		return strings.TrimLeftFunc(text, unicode.IsSpace)
	case '~':
		text = strings.TrimLeftFunc(text, isInlineSpace)
		if strings.HasPrefix(text, "\r\n") {
			return text[2:]
		}
		if strings.HasPrefix(text, "\n") {
			return text[1:]
		}
	}
	return text
}

func trimRight(text string, mode byte) string {
	switch mode {
	case '-':
		return strings.TrimRightFunc(text, unicode.IsSpace)
	case '~':
		return strings.TrimRightFunc(text, isInlineSpace)
	}
	return text
}

// --- lexer ---

type tokKind uint8

const (
	tkIdent tokKind = iota
	tkLocal         // $name
	tkString
	tkInt
	tkDot
	tkLBrack
	tkRBrack
	tkMinus
	tkSep // newline or ';'
	tkOther
)

type token struct {
	kind tokKind
	text string // identifier name, decoded string, integer literal or raw text
}

func lexCode(code string) ([]token, error) {
	var toks []token
	for i := 0; i < len(code); {
		c := code[i]
		switch {
		case c == '\n' || c == ';':
			toks = append(toks, token{kind: tkSep})
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '#':
			for i < len(code) && code[i] != '\n' {
				i++
			}
		case c == '.':
			toks = append(toks, token{kind: tkDot})
			i++
		case c == '[':
			toks = append(toks, token{kind: tkLBrack})
			i++
		case c == ']':
			toks = append(toks, token{kind: tkRBrack})
			i++
		case c == '-':
			toks = append(toks, token{kind: tkMinus})
			i++
		case c == '"' || c == '\'' || c == '`':
			end := skipString(code, i)
			if end < 0 {
				return nil, fmt.Errorf("%w: unterminated string literal", errTemplate)
			}
			s, err := unquote(code[i+1:end], c)
			if err != nil {
				return nil, err
			}
			toks = append(toks, token{kind: tkString, text: s})
			i = end + 1
		case isIdentStart(c):
			j := i + 1
			for j < len(code) && isIdentPart(code[j]) {
				j++
			}
			toks = append(toks, token{kind: tkIdent, text: code[i:j]})
			i = j
		case c == '$' && i+1 < len(code) && isIdentStart(code[i+1]):
			j := i + 2
			for j < len(code) && isIdentPart(code[j]) {
				j++
			}
			toks = append(toks, token{kind: tkLocal, text: code[i+1 : j]})
			i = j
		case c >= '0' && c <= '9':
			j := i + 1
			for j < len(code) && code[j] >= '0' && code[j] <= '9' {
				j++
			}
			if j < len(code) && (code[j] == '.' && j+1 < len(code) && code[j+1] >= '0' && code[j+1] <= '9' || isIdentStart(code[j])) {
				return nil, fmt.Errorf("%w: number literal %q", ErrUnsupportedTemplate, code[i:])
			}
			toks = append(toks, token{kind: tkInt, text: code[i:j]})
			i = j
		default:
			_, size := utf8.DecodeRuneInString(code[i:])
			toks = append(toks, token{kind: tkOther, text: code[i : i+size]})
			i += size
		}
	}
	return toks, nil
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentPart(c byte) bool { return isIdentStart(c) || (c >= '0' && c <= '9') }

// unquote decodes a Scriban string literal body. Backtick strings are
// verbatim; the others support the usual backslash escapes.
func unquote(body string, q byte) (string, error) {
	if q == '`' {
		return body, nil
	}
	var b strings.Builder
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c != '\\' {
			b.WriteByte(c)
			continue
		}
		i++
		if i >= len(body) {
			return "", fmt.Errorf("%w: bad escape", errTemplate)
		}
		switch e := body[i]; e {
		case '\'', '"', '\\':
			b.WriteByte(e)
		case '0':
			b.WriteByte(0)
		case 'a':
			b.WriteByte('\a')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'v':
			b.WriteByte('\v')
		case 'x', 'u':
			n := 2
			if e == 'u' {
				n = 4
			}
			if i+1+n > len(body) {
				return "", fmt.Errorf("%w: bad escape", errTemplate)
			}
			cp, err := strconv.ParseUint(body[i+1:i+1+n], 16, 32)
			if err != nil {
				return "", fmt.Errorf("%w: bad escape", errTemplate)
			}
			i += n
			r := rune(cp)
			// Join a \uD800-\uDBFF \uDC00-\uDFFF pair into one code point
			// (C# strings are UTF-16); lone surrogates become U+FFFD.
			if utf16.IsSurrogate(r) && r < 0xdc00 && i+7 <= len(body) && body[i+1] == '\\' && body[i+2] == 'u' {
				if lo, err := strconv.ParseUint(body[i+3:i+7], 16, 32); err == nil {
					if dec := utf16.DecodeRune(r, rune(lo)); dec != utf8.RuneError {
						r = dec
						i += 6
					}
				}
			}
			b.WriteRune(r)
		default:
			return "", fmt.Errorf("%w: bad escape \\%c", errTemplate, e)
		}
	}
	return b.String(), nil
}

// controlKeywords are Scriban statement keywords; this subset does not
// implement them.
var controlKeywords = map[string]bool{
	"if": true, "else": true, "end": true, "for": true, "in": true,
	"while": true, "func": true, "ret": true, "break": true,
	"continue": true, "case": true, "when": true, "with": true,
	"wrap": true, "capture": true, "readonly": true, "import": true,
	"tablerow": true, "this": true, "do": true,
}

// builtinObjects are Scriban's builtin function objects; a model member of
// the same name shadows them, otherwise rendering one is unsupported.
var builtinObjects = map[string]bool{
	"array": true, "date": true, "html": true, "include": true,
	"math": true, "object": true, "regex": true, "string": true,
	"timespan": true,
}

// --- evaluator ---

func evalCode(out *strings.Builder, code string, scope templateScope) error {
	toks, err := lexCode(code)
	if err != nil {
		return err
	}
	for len(toks) > 0 {
		n := 0
		for n < len(toks) && toks[n].kind != tkSep {
			n++
		}
		stmt := toks[:n]
		if n < len(toks) {
			toks = toks[n+1:]
		} else {
			toks = nil
		}
		if len(stmt) == 0 {
			continue
		}
		v, err := evalExpr(stmt, scope)
		if err != nil {
			return err
		}
		s, err := renderValue(v)
		if err != nil {
			return err
		}
		out.WriteString(s)
	}
	return nil
}

// evalExpr evaluates "primary accessor*". A nil result is Scriban's null.
func evalExpr(toks []token, scope templateScope) (*jvalue, error) {
	var (
		v    *jvalue
		name string
	)
	switch t := toks[0]; t.kind {
	case tkIdent:
		name = t.text
		switch {
		case name == "true" || name == "false":
			v = &jvalue{kind: jBool, b: name == "true"}
		case name == "null":
			v = nil
		case controlKeywords[name]:
			return nil, fmt.Errorf("%w: keyword %q", ErrUnsupportedTemplate, name)
		default:
			mv, ok := scope(name)
			switch {
			case ok:
				v = nullToNil(mv)
			case builtinObjects[name]:
				return nil, fmt.Errorf("%w: builtin %q", ErrUnsupportedTemplate, name)
			}
		}
	case tkLocal:
		name = "$" + t.text
	case tkString:
		v = jstr(t.text)
	case tkInt:
		v = &jvalue{kind: jNumber, str: t.text}
	default:
		return nil, fmt.Errorf("%w: unexpected %q", ErrUnsupportedTemplate, t.text)
	}
	toks = toks[1:]
	for len(toks) > 0 {
		switch toks[0].kind {
		case tkDot:
			if len(toks) < 2 || toks[1].kind != tkIdent {
				return nil, fmt.Errorf("%w: bad member access", ErrUnsupportedTemplate)
			}
			member := toks[1].text
			toks = toks[2:]
			if v == nil {
				return nil, fmt.Errorf("%w: cannot get the member %s.%s for a null object", errTemplate, name, member)
			}
			name += "." + member
			v = getMember(v, member)
		case tkLBrack:
			var (
				idx   token
				neg   bool
				close int
			)
			switch {
			case len(toks) >= 3 && (toks[1].kind == tkInt || toks[1].kind == tkString) && toks[2].kind == tkRBrack:
				idx, close = toks[1], 2
			case len(toks) >= 4 && toks[1].kind == tkMinus && toks[2].kind == tkInt && toks[3].kind == tkRBrack:
				idx, neg, close = toks[2], true, 3
			default:
				return nil, fmt.Errorf("%w: unsupported indexer", ErrUnsupportedTemplate)
			}
			toks = toks[close+1:]
			if v == nil {
				return nil, fmt.Errorf("%w: object %s is null, cannot access indexer", errTemplate, name)
			}
			name += "[]"
			var err error
			if v, err = getIndex(v, idx, neg); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("%w: unexpected %q", ErrUnsupportedTemplate, toks[0].text)
		}
	}
	return v, nil
}

func nullToNil(v *jvalue) *jvalue {
	if v == nil || v.kind == jNull {
		return nil
	}
	return v
}

func getMember(v *jvalue, member string) *jvalue {
	switch v.kind {
	case jObject:
		m, _ := v.get(member)
		return nullToNil(m)
	case jArray:
		if member == "size" {
			return &jvalue{kind: jNumber, str: strconv.Itoa(len(v.arr))}
		}
	case jString:
		if member == "size" {
			return &jvalue{kind: jNumber, str: strconv.Itoa(len(utf16.Encode([]rune(v.str))))}
		}
	}
	return nil
}

func getIndex(v *jvalue, idx token, neg bool) (*jvalue, error) {
	switch v.kind {
	case jObject:
		if idx.kind == tkString {
			m, _ := v.get(idx.text)
			return nullToNil(m), nil
		}
		return nil, nil
	case jArray:
		if idx.kind != tkInt {
			return nil, nil
		}
		n, err := strconv.Atoi(idx.text)
		if err != nil {
			return nil, nil
		}
		if neg {
			if n != 0 {
				n = len(v.arr) - n
			}
		}
		if n < 0 || n >= len(v.arr) {
			return nil, nil
		}
		return nullToNil(v.arr[n]), nil
	case jString:
		if idx.kind != tkInt {
			return nil, fmt.Errorf("%w: string indexer", ErrUnsupportedTemplate)
		}
		units := utf16.Encode([]rune(v.str))
		n, err := strconv.Atoi(idx.text)
		if err != nil {
			return nil, fmt.Errorf("%w: index out of range", errTemplate)
		}
		if neg {
			if n != 0 {
				n = len(units) - n
			}
		}
		if n < 0 || n >= len(units) {
			return nil, fmt.Errorf("%w: index out of range", errTemplate)
		}
		return jstr(string(utf16.Decode(units[n : n+1]))), nil
	}
	return nil, fmt.Errorf("%w: indexer on %s", ErrUnsupportedTemplate, v.kind)
}

// renderValue converts a value to text like Scriban's TemplateContext.
func renderValue(v *jvalue) (string, error) {
	if v == nil {
		return "", nil
	}
	switch v.kind {
	case jNull:
		return "", nil
	case jString:
		return v.str, nil
	case jBool:
		return strconv.FormatBool(v.b), nil
	case jNumber:
		return formatNumber(v.str)
	}
	var b strings.Builder
	if err := writeLiteral(&b, v); err != nil {
		return "", err
	}
	return b.String(), nil
}

// writeLiteral formats nested values the way Scriban renders arrays and
// objects: `[1, "s", null]`, `{key: value, "not ident": 1}`.
func writeLiteral(b *strings.Builder, v *jvalue) error {
	switch v.kind {
	case jNull:
		b.WriteString("null")
	case jBool:
		b.WriteString(strconv.FormatBool(v.b))
	case jNumber:
		s, err := formatNumber(v.str)
		if err != nil {
			return err
		}
		b.WriteString(s)
	case jString:
		writeScribanString(b, v.str)
	case jArray:
		b.WriteByte('[')
		for i, e := range v.arr {
			if i > 0 {
				b.WriteString(", ")
			}
			if err := writeLiteral(b, e); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case jObject:
		b.WriteByte('{')
		for i, m := range v.obj {
			if i > 0 {
				b.WriteString(", ")
			}
			if isLiteralKey(m.key) {
				b.WriteString(m.key)
			} else {
				writeScribanString(b, m.key)
			}
			b.WriteString(": ")
			if err := writeLiteral(b, m.val); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	}
	return nil
}

func isLiteralKey(k string) bool {
	if k == "" {
		return false
	}
	for i, r := range k {
		if r == '_' || unicode.IsLetter(r) || (i > 0 && unicode.IsDigit(r)) {
			continue
		}
		return false
	}
	return true
}

func writeScribanString(b *strings.Builder, s string) {
	const hex = "0123456789abcdef"
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\a':
			b.WriteString(`\a`)
		case '\b':
			b.WriteString(`\b`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\v':
			b.WriteString(`\v`)
		case '\f':
			b.WriteString(`\f`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if c < 0x20 {
				b.WriteString(`\x`)
				b.WriteByte(hex[c>>4])
				b.WriteByte(hex[c&0xf])
			} else {
				b.WriteByte(c)
			}
		}
	}
	b.WriteByte('"')
}

// formatNumber renders a JSON number literal the way Scriban renders a
// JsonElement number: integers that fit in a long print as such; anything
// else is converted to System.Decimal, which keeps the literal's scale
// ("1.50", "1e3" -> "1000", "100e-2" -> "1.00", "-0.0" -> "0.0"). Literals
// that System.Decimal cannot hold exactly (a mantissa above 2^96-1 or a
// scale above 28, where .NET rounds or fails) are reported as unsupported.
func formatNumber(lit string) (string, error) {
	if n, err := strconv.ParseInt(lit, 10, 64); err == nil {
		return strconv.FormatInt(n, 10), nil
	}
	s := lit
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	mant, exp := s, 0
	if k := strings.IndexAny(s, "eE"); k >= 0 {
		mant = s[:k]
		e, err := strconv.Atoi(strings.TrimPrefix(s[k+1:], "+"))
		if err != nil || e > 1000 || e < -1000 {
			return "", fmt.Errorf("%w: number %s", ErrUnsupportedTemplate, lit)
		}
		exp = e
	}
	intPart, frac := mant, ""
	if k := strings.IndexByte(mant, '.'); k >= 0 {
		intPart, frac = mant[:k], mant[k+1:]
	}
	digits := intPart + frac
	scale := len(frac) - exp
	if scale < 0 {
		digits += strings.Repeat("0", -scale)
		scale = 0
	}
	if m, ok := new(big.Int).SetString(digits, 10); !ok || m.Cmp(decimalMaxMantissa) > 0 || scale > 28 {
		return "", fmt.Errorf("%w: number %s", ErrUnsupportedTemplate, lit)
	}
	if len(digits) < scale+1 {
		digits = strings.Repeat("0", scale+1-len(digits)) + digits
	}
	ip := strings.TrimLeft(digits[:len(digits)-scale], "0")
	if ip == "" {
		ip = "0"
	}
	out := ip
	if scale > 0 {
		out += "." + digits[len(digits)-scale:]
	}
	if neg && strings.Trim(digits, "0") != "" {
		out = "-" + out
	}
	return out, nil
}
