package doc

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"
)

// TOMLError is the error ParseTOML returns: a TomlFormatException of lip's
// TomlJson (src/Lip.Core/Json/TomlJson.cs). Message says what is wrong with
// the file, and where when that is known; it does not name the file, which
// lip's callers do (e.g. "The lip.toml is not a valid TOML file of the kind
// lip reads.").
type TOMLError struct {
	Message string
	Err     error
}

func (e *TOMLError) Error() string { return e.Message }
func (e *TOMLError) Unwrap() error { return e.Err }

// tomlMaxNesting bounds the recursion of the parser itself; Convert then
// rejects anything nested deeper than maxJSONDepth.
const tomlMaxNesting = 1000

// ParseTOML reads a TOML file as the JSON value it represents, as lip's
// TomlJson.Parse(byte[]) does (protocol/README.md "TOML file"): a TOML
// document read by Tomlyn 2 (TOML 1.1: \e and \xHH escapes, inline tables
// over several lines with a trailing comma, times without seconds), where a
// table is an object, an array an array, a string a string, an integer or a
// float a number (written as Utf8JsonWriter writes a long or a double), and a
// boolean a boolean. A date-time, a date or a time, and the floats inf and
// nan, are not valid, nor is anything nested deeper than 64 levels. The value
// is always an *Object.
//
// The bytes are decoded as .NET's StreamReader does with a strict UTF-8
// encoding and byte order mark detection (decodeText). Errors are
// *TOMLError.
func ParseTOML(data []byte) (any, error) {
	text, derr := decodeText(data)
	if derr != nil {
		return nil, &TOMLError{Message: "The file is not valid UTF-8 text. Save it with the UTF-8 encoding.", Err: derr}
	}
	p := &tomlParser{s: []rune(string(utf16.Decode(text)))}
	p.root = &tomlTable{kind: tomlHeader}
	p.cur = p.root
	if err := p.document(); err != nil {
		return nil, &TOMLError{Message: "The file is not valid TOML: " + err.Error(), Err: err}
	}
	// Tomlyn fails on a table or an array nested 64 levels below the
	// document, before TomlJson looks at the values.
	if tomlDepth(p.root, 0) >= maxJSONDepth {
		return nil, &TOMLError{Message: fmt.Sprintf("The file is not valid TOML: The maximum depth of %d has been"+
			" exceeded", maxJSONDepth)}
	}
	return tomlConvert(p.root, "")
}

// tomlDepth is the deepest level of a table or an array at or below v,
// which is at level depth (the document is at level 0).
func tomlDepth(v any, depth int) int {
	deepest := -1
	switch t := v.(type) {
	case *tomlTable:
		deepest = depth
		for _, k := range t.keys {
			deepest = max(deepest, tomlDepth(t.vals[k], depth+1))
		}
	case *tomlArray:
		deepest = depth
		for _, item := range t.items {
			deepest = max(deepest, tomlDepth(item, depth+1))
		}
	}
	return deepest
}

// tomlKind is how a table came to be, which decides how it may be extended.
type tomlKind int

const (
	// tomlImplicit: created as a prefix of a [header] or [[header]]; a
	// [header] may define it once, and dotted keys may extend it.
	tomlImplicit tomlKind = iota
	// tomlHeader: defined by a [header] (or the root); not defined again.
	tomlHeader
	// tomlDotted: created by a dotted key; extended by dotted keys of the
	// same table, and holds sub-tables of headers, but no [header] of its own.
	tomlDotted
	// tomlInline: an inline table or a table inside one; never extended.
	tomlInline
	// tomlElement: an element of an array of tables.
	tomlElement
	// tomlInlineDotted: created by a dotted key of an inline table that is
	// being read; tomlInline once it is read.
	tomlInlineDotted
)

type tomlTable struct {
	keys []string
	vals map[string]any
	kind tomlKind
}

func (t *tomlTable) get(k string) (any, bool) {
	v, ok := t.vals[k]
	return v, ok
}

func (t *tomlTable) set(k string, v any) {
	if t.vals == nil {
		t.vals = map[string]any{}
	}
	t.keys = append(t.keys, k)
	t.vals[k] = v
}

// tomlArray is an array; tables is set for an array of tables ([[header]]).
type tomlArray struct {
	items  []any
	tables bool
}

// Scalars other than strings and booleans.
type (
	tomlInteger  int64
	tomlFloat    float64
	tomlDateTime struct{}
)

type tomlParser struct {
	s     []rune
	i     int
	root  *tomlTable
	cur   *tomlTable
	depth int
}

// tomlSyntaxError is a syntax or semantic error at a position of the text.
type tomlSyntaxError struct {
	line, col int
	msg       string
}

func (e *tomlSyntaxError) Error() string {
	return fmt.Sprintf("(%d,%d) : error : %s", e.line, e.col, e.msg)
}

func (p *tomlParser) errAt(pos int, format string, args ...any) error {
	line, col := 1, 1
	for j := 0; j < pos && j < len(p.s); j++ {
		if p.s[j] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return &tomlSyntaxError{line: line, col: col, msg: fmt.Sprintf(format, args...)}
}

func (p *tomlParser) errorf(format string, args ...any) error { return p.errAt(p.i, format, args...) }

func (p *tomlParser) eof() bool { return p.i >= len(p.s) }

func (p *tomlParser) peek() rune {
	if p.i < len(p.s) {
		return p.s[p.i]
	}
	return -1
}

func (p *tomlParser) peekAt(n int) rune {
	if p.i+n < len(p.s) {
		return p.s[p.i+n]
	}
	return -1
}

func (p *tomlParser) skipWS() {
	for p.i < len(p.s) && (p.s[p.i] == ' ' || p.s[p.i] == '\t') {
		p.i++
	}
}

// newline consumes a line break (LF or CRLF) and reports whether there was
// one. A CR without LF is an error.
func (p *tomlParser) newline() (bool, error) {
	switch p.peek() {
	case '\n':
		p.i++
		return true, nil
	case '\r':
		if p.peekAt(1) == '\n' {
			p.i += 2
			return true, nil
		}
		return false, p.errorf("Invalid \\r not followed by \\n")
	}
	return false, nil
}

// isControl reports a character TOML allows in no string or comment: the
// control characters other than tab, line feed and carriage return are
// never allowed; those two only as line breaks.
func isTOMLControl(c rune) bool {
	return (c < 0x20 && c != '\t') || c == 0x7F
}

// comment consumes a comment, if any, up to the line break.
func (p *tomlParser) comment() error {
	if p.peek() != '#' {
		return nil
	}
	p.i++
	for !p.eof() {
		c := p.s[p.i]
		if c == '\n' || (c == '\r' && p.peekAt(1) == '\n') {
			return nil
		}
		if isTOMLControl(c) {
			return p.errorf("Invalid control character U+%04X in comment", c)
		}
		p.i++
	}
	return nil
}

// endOfLine consumes whitespace, a comment and the line break that end a
// key/value pair or a header.
func (p *tomlParser) endOfLine() error {
	p.skipWS()
	if err := p.comment(); err != nil {
		return err
	}
	if p.eof() {
		return nil
	}
	ok, err := p.newline()
	if err != nil {
		return err
	}
	if !ok {
		return p.errorf("Unexpected token found `%c` while expecting token `newline`", p.peek())
	}
	return nil
}

func (p *tomlParser) document() error {
	// Tomlyn skips a byte order mark at the start of the text (one that
	// decoding left), and fails on U+FFFD anywhere.
	if p.peek() == 0xFEFF {
		p.i++
	}
	for j, c := range p.s {
		if c == 0xFFFD {
			return p.errAt(j, "Invalid UTF-16 surrogate")
		}
	}
	for {
		p.skipWS()
		if p.eof() {
			return nil
		}
		if ok, err := p.newline(); err != nil {
			return err
		} else if ok {
			continue
		}
		switch p.peek() {
		case '#':
			if err := p.comment(); err != nil {
				return err
			}
			continue
		case '[':
			if err := p.header(); err != nil {
				return err
			}
		default:
			if err := p.keyValue(p.cur); err != nil {
				return err
			}
		}
		if err := p.endOfLine(); err != nil {
			return err
		}
	}
}

func isBareKeyChar(c rune) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-'
}

// key reads a dotted key: simple keys separated by '.', with whitespace
// around the dots.
func (p *tomlParser) key() ([]string, error) {
	var parts []string
	for {
		p.skipWS()
		var k string
		switch c := p.peek(); {
		case c == '"':
			if p.peekAt(1) == '"' && p.peekAt(2) == '"' {
				return nil, p.errorf("A multi-line string cannot be a key")
			}
			s, err := p.basicString()
			if err != nil {
				return nil, err
			}
			k = s
		case c == '\'':
			if p.peekAt(1) == '\'' && p.peekAt(2) == '\'' {
				return nil, p.errorf("A multi-line string cannot be a key")
			}
			s, err := p.literalString()
			if err != nil {
				return nil, err
			}
			k = s
		case isBareKeyChar(c):
			start := p.i
			for !p.eof() && isBareKeyChar(p.s[p.i]) {
				p.i++
			}
			k = string(p.s[start:p.i])
		case c == -1:
			return nil, p.errorf("Expecting a key instead of <eof>")
		default:
			return nil, p.errorf("Unexpected token [%c] found", c)
		}
		parts = append(parts, k)
		p.skipWS()
		if p.peek() != '.' {
			return parts, nil
		}
		p.i++
	}
}

func dotted(parts []string) string {
	var b strings.Builder
	for i, k := range parts {
		if i > 0 {
			b.WriteByte('.')
		}
		b.WriteString(k)
	}
	return b.String()
}

// header reads a [table] or [[array of tables]] header and makes its table
// the current one.
func (p *tomlParser) header() error {
	start := p.i
	p.i++
	array := false
	if p.peek() == '[' {
		array = true
		p.i++
	}
	parts, err := p.key()
	if err != nil {
		return err
	}
	p.skipWS()
	if p.peek() != ']' {
		return p.errorf("Expecting `]` to close the table header")
	}
	p.i++
	if array {
		if p.peek() != ']' {
			return p.errorf("Expecting `]]` to close the array of tables header")
		}
		p.i++
	}
	redefined := func() error {
		return p.errAt(start, "The key `%s` is already defined and cannot be redefined", dotted(parts))
	}

	t := p.root
	for _, k := range parts[:len(parts)-1] {
		v, ok := t.get(k)
		if !ok {
			n := &tomlTable{kind: tomlImplicit}
			t.set(k, n)
			t = n
			continue
		}
		switch x := v.(type) {
		case *tomlTable:
			if x.kind == tomlInline {
				return redefined()
			}
			t = x
		case *tomlArray:
			if !x.tables {
				return redefined()
			}
			t = x.items[len(x.items)-1].(*tomlTable)
		default:
			return redefined()
		}
	}
	last := parts[len(parts)-1]
	v, exists := t.get(last)
	if array {
		elem := &tomlTable{kind: tomlElement}
		if !exists {
			t.set(last, &tomlArray{items: []any{elem}, tables: true})
		} else if a, ok := v.(*tomlArray); ok && a.tables {
			a.items = append(a.items, elem)
		} else {
			return redefined()
		}
		p.cur = elem
		return nil
	}
	if !exists {
		n := &tomlTable{kind: tomlHeader}
		t.set(last, n)
		p.cur = n
		return nil
	}
	if x, ok := v.(*tomlTable); ok && x.kind == tomlImplicit {
		x.kind = tomlHeader
		p.cur = x
		return nil
	}
	return redefined()
}

// keyValue reads a key/value pair into t.
func (p *tomlParser) keyValue(t *tomlTable) error {
	start := p.i
	parts, err := p.key()
	if err != nil {
		return err
	}
	p.skipWS()
	if p.peek() != '=' {
		if p.eof() {
			return p.errorf("Expecting `=` after a key instead of <eof>")
		}
		return p.errorf("Expecting `=` after a key instead of `%c`", p.peek())
	}
	p.i++
	p.skipWS()
	value, err := p.value()
	if err != nil {
		return err
	}
	redefined := func() error {
		return p.errAt(start, "The key `%s` is already defined and cannot be redefined", dotted(parts))
	}
	// Tomlyn lets a [header] define a table that dotted keys of an element
	// of an array of tables created, as if a header had.
	dottedKind := tomlDotted
	switch t.kind {
	case tomlElement:
		dottedKind = tomlImplicit
	case tomlInline, tomlInlineDotted:
		dottedKind = tomlInlineDotted
	}
	for _, k := range parts[:len(parts)-1] {
		v, ok := t.get(k)
		if !ok {
			n := &tomlTable{kind: dottedKind}
			t.set(k, n)
			t = n
			continue
		}
		x, ok := v.(*tomlTable)
		if !ok {
			return redefined()
		}
		switch x.kind {
		case tomlDotted:
		case tomlImplicit:
			x.kind = dottedKind
		case tomlInlineDotted:
		default:
			return redefined()
		}
		t = x
	}
	last := parts[len(parts)-1]
	if _, ok := t.get(last); ok {
		return redefined()
	}
	t.set(last, value)
	return nil
}

// skipBlank skips whitespace, line breaks and comments, as arrays and
// inline tables allow between their items.
func (p *tomlParser) skipBlank() error {
	for {
		p.skipWS()
		if err := p.comment(); err != nil {
			return err
		}
		ok, err := p.newline()
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	}
}

func (p *tomlParser) value() (any, error) {
	switch c := p.peek(); {
	case c == '"':
		if p.peekAt(1) == '"' && p.peekAt(2) == '"' {
			return p.multilineBasicString()
		}
		return p.basicString()
	case c == '\'':
		if p.peekAt(1) == '\'' && p.peekAt(2) == '\'' {
			return p.multilineLiteralString()
		}
		return p.literalString()
	case c == '[':
		return p.array()
	case c == '{':
		return p.inlineTable()
	case c == -1:
		return nil, p.errorf("Unexpected <eof> for a value")
	default:
		return p.scalar()
	}
}

func (p *tomlParser) enter() error {
	p.depth++
	if p.depth > tomlMaxNesting {
		return p.errorf("The document nests arrays or inline tables too deeply")
	}
	return nil
}

func (p *tomlParser) array() (any, error) {
	if err := p.enter(); err != nil {
		return nil, err
	}
	defer func() { p.depth-- }()
	p.i++
	a := &tomlArray{items: []any{}}
	for {
		if err := p.skipBlank(); err != nil {
			return nil, err
		}
		if p.peek() == ']' {
			p.i++
			return a, nil
		}
		if p.peek() == ',' {
			return nil, p.errorf("Unexpected token `,` for a value")
		}
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		a.items = append(a.items, v)
		if err := p.skipBlank(); err != nil {
			return nil, err
		}
		switch p.peek() {
		case ',':
			p.i++
		case ']':
			p.i++
			return a, nil
		case -1:
			return nil, p.errorf("Unexpected <eof> while parsing an array")
		default:
			return nil, p.errorf("Unexpected token `%c` while parsing an array. Expecting `,` or `]`", p.peek())
		}
	}
}

func (p *tomlParser) inlineTable() (any, error) {
	if err := p.enter(); err != nil {
		return nil, err
	}
	defer func() { p.depth-- }()
	p.i++
	t := &tomlTable{kind: tomlInline}
	for {
		if err := p.skipBlank(); err != nil {
			return nil, err
		}
		if p.peek() == '}' {
			p.i++
			freeze(t)
			return t, nil
		}
		if p.peek() == ',' {
			return nil, p.errorf("Unexpected token `Comma` while parsing inline table. Expecting a bare key or" +
				" string instead of `,`")
		}
		if err := p.keyValue(t); err != nil {
			return nil, err
		}
		if err := p.skipBlank(); err != nil {
			return nil, err
		}
		switch p.peek() {
		case ',':
			p.i++
		case '}':
			p.i++
			freeze(t)
			return t, nil
		case -1:
			return nil, p.errorf("Unexpected <eof> while parsing an inline table")
		default:
			return nil, p.errorf("Unexpected token `%c` while parsing an inline table. Expecting `,` or `}`",
				p.peek())
		}
	}
}

// freeze marks the tables that dotted keys of a read inline table created
// as part of it.
func freeze(t *tomlTable) {
	for _, k := range t.keys {
		if x, ok := t.vals[k].(*tomlTable); ok && x.kind == tomlInlineDotted {
			x.kind = tomlInline
			freeze(x)
		}
	}
}

// escape reads the escape sequence after a backslash in a basic string.
func (p *tomlParser) escape(b *strings.Builder) error {
	start := p.i - 1
	c := p.peek()
	p.i++
	switch c {
	case 'b':
		b.WriteByte('\b')
	case 't':
		b.WriteByte('\t')
	case 'n':
		b.WriteByte('\n')
	case 'f':
		b.WriteByte('\f')
	case 'r':
		b.WriteByte('\r')
	case 'e':
		b.WriteByte(0x1B)
	case '"':
		b.WriteByte('"')
	case '\\':
		b.WriteByte('\\')
	case 'x', 'u', 'U':
		n := map[rune]int{'x': 2, 'u': 4, 'U': 8}[c]
		if p.i+n > len(p.s) {
			return p.errAt(start, "Invalid End-Of-File found on string literal")
		}
		var v rune
		for j := 0; j < n; j++ {
			d, ok := hexVal(byteOf(p.s[p.i+j]))
			if !ok {
				return p.errAt(start, "Invalid hexadecimal digit `%c` in escape sequence", p.s[p.i+j])
			}
			v = v*16 + rune(d)
		}
		p.i += n
		if v > 0x10FFFF || (v >= 0xD800 && v <= 0xDFFF) {
			return p.errAt(start, "Invalid Unicode scalar value [%X]", v)
		}
		b.WriteRune(v)
	case -1:
		return p.errAt(start, "Invalid End-Of-File found on string literal")
	default:
		return p.errAt(start, "Unexpected escape character [%c] in string. Only b t n f r e \\ \" xHH u0000-uFFFF"+
			" U00000000-UFFFFFFFF are allowed", c)
	}
	return nil
}

func byteOf(r rune) byte {
	if r > 0x7F {
		return 0
	}
	return byte(r)
}

func (p *tomlParser) basicString() (string, error) {
	p.i++
	var b strings.Builder
	for {
		if p.eof() {
			return "", p.errorf("Invalid End-Of-File found on string literal")
		}
		c := p.s[p.i]
		switch {
		case c == '"':
			p.i++
			return b.String(), nil
		case c == '\\':
			p.i++
			if err := p.escape(&b); err != nil {
				return "", err
			}
		case c == '\n' || c == '\r':
			return "", p.errorf("Invalid newline in a single-line string")
		case isTOMLControl(c):
			return "", p.errorf("Invalid control character found in string")
		default:
			b.WriteRune(c)
			p.i++
		}
	}
}

func (p *tomlParser) literalString() (string, error) {
	p.i++
	start := p.i
	for {
		if p.eof() {
			return "", p.errorf("Invalid End-Of-File found on string literal")
		}
		c := p.s[p.i]
		switch {
		case c == '\'':
			s := string(p.s[start:p.i])
			p.i++
			return s, nil
		case c == '\n' || c == '\r':
			return "", p.errorf("Invalid newline in a single-line string")
		case isTOMLControl(c):
			return "", p.errorf("Invalid control character found in string")
		}
		p.i++
	}
}

// closingQuotes handles a run of quote characters q inside a multi-line
// string at p.i: three or more close the string, the ones beyond three (at
// most two) belonging to it. It reports whether the string ended, appending
// those quotes to b.
func (p *tomlParser) closingQuotes(q rune, b *strings.Builder) (bool, error) {
	n := 0
	for p.i+n < len(p.s) && p.s[p.i+n] == q {
		n++
	}
	if n < 3 {
		for range n {
			b.WriteRune(q)
		}
		p.i += n
		return false, nil
	}
	if n > 5 {
		return false, p.errAt(p.i+5, "Unexpected token found `%c` while expecting token `newline`", q)
	}
	for range n - 3 {
		b.WriteRune(q)
	}
	p.i += n
	return true, nil
}

// lineBreak appends a line break of a multi-line string at p.i, if any.
func (p *tomlParser) lineBreak(b *strings.Builder) (bool, error) {
	switch p.peek() {
	case '\n':
		b.WriteByte('\n')
		p.i++
		return true, nil
	case '\r':
		if p.peekAt(1) == '\n' {
			b.WriteString("\r\n")
			p.i += 2
			return true, nil
		}
		return false, p.errorf("Invalid \\r not followed by \\n")
	}
	return false, nil
}

// skipFirstNewline skips the line break right after the opening delimiter
// of a multi-line string.
func (p *tomlParser) skipFirstNewline() {
	if p.peek() == '\n' {
		p.i++
	} else if p.peek() == '\r' && p.peekAt(1) == '\n' {
		p.i += 2
	}
}

func (p *tomlParser) multilineBasicString() (string, error) {
	p.i += 3
	p.skipFirstNewline()
	var b strings.Builder
	for {
		if p.eof() {
			return "", p.errorf("Invalid End-Of-File found on string literal")
		}
		c := p.s[p.i]
		switch {
		case c == '"':
			done, err := p.closingQuotes('"', &b)
			if err != nil || done {
				return b.String(), err
			}
		case c == '\\':
			// A line-ending backslash: it removes the whitespace before the
			// line break and all whitespace and line breaks after it. As
			// Tomlyn reads it, a backslash followed by a space needs at
			// least one more whitespace character or line break after the
			// first line break.
			if after := p.peekAt(1); after == ' ' || after == '\t' || after == '\n' || after == '\r' {
				start := p.i
				p.i++
				p.skipWS()
				ok, err := p.newline()
				if err != nil {
					return "", err
				}
				if !ok || (after == ' ' && p.peek() != ' ' && p.peek() != '\t' && p.peek() != '\n' &&
					p.peek() != '\r') {
					return "", p.errAt(start, "Invalid escape `\\`. It must skip at least one line.")
				}
				for {
					p.skipWS()
					ok, err := p.newline()
					if err != nil {
						return "", err
					}
					if !ok {
						break
					}
				}
				continue
			}
			p.i++
			if err := p.escape(&b); err != nil {
				return "", err
			}
		case c == '\n' || c == '\r':
			if _, err := p.lineBreak(&b); err != nil {
				return "", err
			}
		case isTOMLControl(c):
			return "", p.errorf("Invalid control character found in string")
		default:
			b.WriteRune(c)
			p.i++
		}
	}
}

func (p *tomlParser) multilineLiteralString() (string, error) {
	p.i += 3
	p.skipFirstNewline()
	var b strings.Builder
	for {
		if p.eof() {
			return "", p.errorf("Invalid End-Of-File found on string literal")
		}
		c := p.s[p.i]
		switch {
		case c == '\'':
			done, err := p.closingQuotes('\'', &b)
			if err != nil || done {
				return b.String(), err
			}
		case c == '\n' || c == '\r':
			if _, err := p.lineBreak(&b); err != nil {
				return "", err
			}
		case isTOMLControl(c):
			return "", p.errorf("Invalid control character found in string")
		default:
			b.WriteRune(c)
			p.i++
		}
	}
}

func isScalarChar(c rune) bool {
	return isBareKeyChar(c) || c == '+' || c == '.' || c == ':'
}

// scalar reads a boolean, a number or a date-time.
func (p *tomlParser) scalar() (any, error) {
	start := p.i
	for !p.eof() && isScalarChar(p.s[p.i]) {
		p.i++
	}
	// A date and a time may be separated by a space.
	if p.i-start == 10 && p.peek() == ' ' && p.i+3 < len(p.s) && isDigit(byteOf(p.s[p.i+1])) &&
		isDigit(byteOf(p.s[p.i+2])) && p.s[p.i+3] == ':' && isLocalDate(string(p.s[start:p.i])) {
		p.i++
		for !p.eof() && isScalarChar(p.s[p.i]) {
			p.i++
		}
	}
	tok := string(p.s[start:p.i])
	if tok == "" {
		return nil, p.errorf("Unexpected token `%c` for a value", p.peek())
	}
	switch tok {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "inf", "+inf", "-inf":
		return tomlFloat(math.Inf(map[bool]int{true: -1, false: 1}[tok[0] == '-'])), nil
	case "nan", "+nan", "-nan":
		return tomlFloat(math.NaN()), nil
	}
	if v, ok, err := parseTOMLInteger(tok); ok {
		if err != nil {
			return nil, p.errAt(start, "%s", err.Error())
		}
		return v, nil
	}
	if isTOMLFloat(tok) {
		f, err := strconv.ParseFloat(strings.ReplaceAll(tok, "_", ""), 64)
		if err != nil && !math.IsInf(f, 0) {
			return nil, p.errAt(start, "Unable to parse float `%s`", tok)
		}
		return tomlFloat(f), nil
	}
	if isTOMLDateTime(tok) {
		return tomlDateTime{}, nil
	}
	return nil, p.errAt(start, "Unexpected token `%s` for a value", tok)
}

// digits reports whether s is digits of a base separated by single
// underscores (each underscore between two digits).
func underscoredDigits(s string, digit func(byte) bool) bool {
	if s == "" {
		return false
	}
	prevDigit := false
	for i := 0; i < len(s); i++ {
		switch {
		case digit(s[i]):
			prevDigit = true
		case s[i] == '_' && prevDigit && i+1 < len(s):
			prevDigit = false
		default:
			return false
		}
	}
	return prevDigit
}

func isHexDigit(c byte) bool {
	return isDigit(c) || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// parseTOMLInteger reads an integer token; ok is false when tok is not
// one. A token that is an integer out of range is ok with an error.
func parseTOMLInteger(tok string) (tomlInteger, bool, error) {
	if len(tok) > 2 && tok[0] == '0' && (tok[1] == 'x' || tok[1] == 'o' || tok[1] == 'b') {
		base, digit := 16, isHexDigit
		switch tok[1] {
		case 'o':
			base, digit = 8, func(c byte) bool { return c >= '0' && c <= '7' }
		case 'b':
			base, digit = 2, func(c byte) bool { return c == '0' || c == '1' }
		}
		if !underscoredDigits(tok[2:], digit) {
			return 0, false, nil
		}
		// Tomlyn limits the number of digits (to those of 64 bits) and
		// reads them as an unsigned 64-bit integer cast to a signed one.
		digits := strings.ReplaceAll(tok[2:], "_", "")
		if len(digits) > map[int]int{16: 16, 8: 21, 2: 64}[base] {
			return 0, true, fmt.Errorf("Invalid size of integer `%s`", tok)
		}
		u, err := strconv.ParseUint(digits, base, 64)
		if err != nil {
			return 0, true, fmt.Errorf("Unable to parse integer `%s`", tok)
		}
		return tomlInteger(int64(u)), true, nil
	}
	body := tok
	if body != "" && (body[0] == '+' || body[0] == '-') {
		body = body[1:]
	}
	if !underscoredDigits(body, isDigit) {
		return 0, false, nil
	}
	if len(body) > 1 && body[0] == '0' {
		return 0, true, fmt.Errorf("Unexpected leading zero (`0`) for integer `%s`", tok)
	}
	i, err := strconv.ParseInt(strings.ReplaceAll(tok, "_", ""), 10, 64)
	if err != nil {
		return 0, true, fmt.Errorf("Unable to parse integer `%s`", tok)
	}
	return tomlInteger(i), true, nil
}

// isTOMLFloat reports a float token other than inf and nan: an integer
// part, then a fractional part, an exponent, or both.
func isTOMLFloat(tok string) bool {
	s := tok
	if s != "" && (s[0] == '+' || s[0] == '-') {
		s = s[1:]
	}
	intEnd := strings.IndexAny(s, ".eE")
	if intEnd <= 0 {
		return false
	}
	intPart := s[:intEnd]
	if !underscoredDigits(intPart, isDigit) || (len(intPart) > 1 && intPart[0] == '0') {
		return false
	}
	rest := s[intEnd:]
	if rest[0] == '.' {
		end := strings.IndexAny(rest, "eE")
		if end < 0 {
			end = len(rest)
		}
		if !underscoredDigits(rest[1:end], isDigit) {
			return false
		}
		rest = rest[end:]
		if rest == "" {
			return true
		}
	}
	// An exponent: e or E, an optional sign, digits.
	exp := rest[1:]
	if exp != "" && (exp[0] == '+' || exp[0] == '-') {
		exp = exp[1:]
	}
	return underscoredDigits(exp, isDigit)
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return true
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// isLocalDate reports a valid YYYY-MM-DD.
func isLocalDate(s string) bool {
	if len(s) != 10 || s[4] != '-' || s[7] != '-' || !allDigits(s[:4]) || !allDigits(s[5:7]) ||
		!allDigits(s[8:]) {
		return false
	}
	year, month, day := atoi(s[:4]), atoi(s[5:7]), atoi(s[8:])
	if year < 1 || month < 1 || month > 12 || day < 1 {
		return false
	}
	days := [...]int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}[month-1]
	if month == 2 && year%4 == 0 && (year%100 != 0 || year%400 == 0) {
		days = 29
	}
	return day <= days
}

// isLocalTime reports a valid HH:MM, HH:MM:SS or HH:MM:SS.fraction.
func isLocalTime(s string) bool {
	if len(s) < 5 || s[2] != ':' || !allDigits(s[:2]) || !allDigits(s[3:5]) {
		return false
	}
	if atoi(s[:2]) > 23 || atoi(s[3:5]) > 59 {
		return false
	}
	rest := s[5:]
	if rest == "" {
		return true
	}
	if len(rest) < 3 || rest[0] != ':' || !allDigits(rest[1:3]) || atoi(rest[1:3]) > 59 {
		return false
	}
	rest = rest[3:]
	if rest == "" {
		return true
	}
	return rest[0] == '.' && allDigits(rest[1:])
}

// isTOMLDateTime reports an offset date-time, a local date-time, a local
// date or a local time.
func isTOMLDateTime(tok string) bool {
	if isLocalTime(tok) {
		return true
	}
	if len(tok) < 10 || !isLocalDate(tok[:10]) {
		return false
	}
	rest := tok[10:]
	if rest == "" {
		return true
	}
	if rest[0] != 'T' && rest[0] != 't' && rest[0] != ' ' {
		return false
	}
	rest = rest[1:]
	switch {
	case strings.HasSuffix(rest, "Z") || strings.HasSuffix(rest, "z"):
		rest = rest[:len(rest)-1]
	case len(rest) > 6 && (rest[len(rest)-6] == '+' || rest[len(rest)-6] == '-'):
		off := rest[len(rest)-5:]
		if off[2] != ':' || !allDigits(off[:2]) || !allDigits(off[3:]) || atoi(off[:2]) > 23 ||
			atoi(off[3:]) > 59 {
			return false
		}
		rest = rest[:len(rest)-6]
	}
	return isLocalTime(rest)
}

func tomlName(key string) string {
	if key == "" {
		return "the document"
	}
	return "'" + key + "'"
}

// tomlConvert is TomlJson.Convert: the JSON value of a TOML value; key
// names it in messages.
func tomlConvert(v any, key string) (any, error) {
	switch t := v.(type) {
	case *tomlTable:
		o := NewObject()
		for _, k := range t.keys {
			name := k
			if key != "" {
				name = key + "." + k
			}
			m, err := tomlConvert(t.vals[k], name)
			if err != nil {
				return nil, err
			}
			o.Set(k, m)
		}
		return o, nil
	case *tomlArray:
		a := make([]any, 0, len(t.items))
		for i, item := range t.items {
			m, err := tomlConvert(item, fmt.Sprintf("%s[%d]", key, i))
			if err != nil {
				return nil, err
			}
			a = append(a, m)
		}
		return a, nil
	case string, bool:
		return t, nil
	case tomlInteger:
		return json.Number(strconv.FormatInt(int64(t), 10)), nil
	case tomlFloat:
		f := float64(t)
		if math.IsInf(f, 0) || math.IsNaN(f) {
			return nil, &TOMLError{Message: fmt.Sprintf("The value of %s is inf or nan, which JSON cannot represent."+
				" Quote it if it is meant as text.", tomlName(key))}
		}
		return json.Number(formatDouble(f)), nil
	case tomlDateTime:
		return nil, &TOMLError{Message: fmt.Sprintf("The value of %s is a date or a time, which lip does not read."+
			" Quote it if it is meant as text.", tomlName(key))}
	default:
		panic(fmt.Sprintf("doc: unexpected TOML value %T", v))
	}
}
