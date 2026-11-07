package doc

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"runtime/debug"
	"strconv"
	"unicode/utf16"
)

// YAMLError is the error ParseYAML returns: a YamlFormatException of lip's
// YamlJson. Message is that exception's message exactly (it does not name the
// file; lip's callers wrap it, e.g. "The lip.yaml is not a valid YAML file of
// the kind lip reads."). File is the name ParseYAML was given.
type YAMLError struct {
	File    string
	Message string
	Err     error // the YamlDotNet exception behind it, if any
}

func (e *YAMLError) Error() string {
	if e.File == "" {
		return e.Message
	}
	return e.File + ": " + e.Message
}

func (e *YAMLError) Unwrap() error { return e.Err }

// formatError is a YamlFormatException thrown while reading.
type formatError struct{ msg string }

func throwFormat(format string, args ...any) { panic(&formatError{fmt.Sprintf(format, args...)}) }

const (
	coreTagPrefix = "tag:yaml.org,2002:"
	// maxDepth is the nesting depth of collections, as JsonDocument allows
	// by default.
	maxDepth = 64
	// maxNodes is the number of nodes a document may expand to through
	// aliases.
	maxNodes = 1_000_000
)

var (
	nullRegex        = regexp.MustCompile(`^(?:null|Null|NULL|~)?$`)
	boolRegex        = regexp.MustCompile(`^(?:true|True|TRUE|false|False|FALSE)$`)
	yaml11BoolRegex  = regexp.MustCompile(`^(?:y|Y|yes|Yes|YES|n|N|no|No|NO|on|On|ON|off|Off|OFF)$`)
	decimalRegex     = regexp.MustCompile(`^[-+]?[0-9]+$`)
	octalRegex       = regexp.MustCompile(`^0o[0-7]+$`)
	hexadecimalRegex = regexp.MustCompile(`^0x[0-9a-fA-F]+$`)
	floatRegex       = regexp.MustCompile(`^[-+]?(?:\.[0-9]+|[0-9]+(?:\.[0-9]*)?)(?:[eE][-+]?[0-9]+)?$`)
	infinityRegex    = regexp.MustCompile(`^[-+]?(?:\.inf|\.Inf|\.INF)$`)
	notANumberRegex  = regexp.MustCompile(`^(?:\.nan|\.NaN|\.NAN)$`)
)

// ParseYAML reads a YAML file as the JSON value it represents, as lip's
// YamlJson.Parse(byte[]) (src/Lip.Core/Json/YamlJson.cs) does: protocol/
// README.md "YAML file", a YAML stream of one document read with the YAML 1.2
// core schema.
//
// The bytes are decoded as .NET's StreamReader does with a strict UTF-8
// encoding and byte order mark detection: a UTF-8, UTF-16 (LE/BE) or UTF-32
// (LE/BE) byte order mark selects that encoding (decoding with U+FFFD for
// invalid input, as .NET's built-in encodings do), and text without one must
// be valid UTF-8. A quoted or block scalar, or one tagged "!" or "!!str", is a
// string; a plain scalar is null, a boolean, an integer (decimal, 0o octal,
// 0x hexadecimal; any size), a float or a string by the core schema; aliases
// are expanded (at most 1,000,000 nodes); collections nest at most 64 deep;
// mapping keys must be unique strings. Errors are *YAMLError.
func ParseYAML(data []byte, file string) (v any, err error) {
	text, derr := decodeText(data)
	if derr != nil {
		return nil, &YAMLError{File: file, Message: "The file is not valid UTF-8 text. Save it with the UTF-8 encoding.",
			Err: derr}
	}
	return parseYAMLText(text, file)
}

func parseYAMLText(text []uint16, file string) (v any, err error) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		var start mark
		var inner error
		switch e := r.(type) {
		case *formatError:
			v, err = nil, &YAMLError{File: file, Message: e.msg}
			return
		case *yamlError:
			start, inner = e.start, fmt.Errorf("%s", e.msg)
		case *yamlCrash:
			// lip does not catch these; report the file as not valid YAML.
			v, err = nil, &YAMLError{File: file, Message: "The file is not valid YAML.", Err: fmt.Errorf("%s", e.msg)}
			return
		default:
			// A bug in this port, not a property of the file: say so rather
			// than blame the file, and keep the launcher running.
			v, err = nil, &YAMLError{File: file, Message: fmt.Sprintf("internal error reading YAML: %v", e),
				Err: fmt.Errorf("%v\n%s", e, debug.Stack())}
			return
		}
		msg := "The file is not valid YAML."
		if start.line > 0 {
			msg = fmt.Sprintf("The file is not valid YAML at line %d, column %d.", start.line, start.column)
		}
		v, err = nil, &YAMLError{File: file, Message: msg, Err: inner}
	}()
	r := &reader{parser: newParser(text), anchors: map[string]anchored{}}
	value, depth := r.readStream()
	// YamlJson writes the value and reads it again as a JsonDocument, which
	// allows 64 levels of nesting.
	if depth > maxJSONDepth {
		throwFormat("The file holds a value that is not a JSON value, such as a string with an unpaired" +
			" surrogate escape like \"\\uD800\".")
	}
	return value, nil
}

// decodeText decodes bytes as new StreamReader(stream, strictUtf8,
// detectEncodingFromByteOrderMarks: true).ReadToEnd() does, into UTF-16;
// the error is the DecoderFallbackException it throws for text without a
// byte order mark that is not UTF-8.
func decodeText(b []byte) ([]uint16, *DecoderError) {
	switch {
	case len(b) >= 2 && b[0] == 0xFE && b[1] == 0xFF:
		return decodeUTF16(b[2:], true), nil
	case len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE:
		if len(b) < 4 || b[2] != 0 || b[3] != 0 {
			return decodeUTF16(b[2:], false), nil
		}
		return decodeUTF32(b[4:], false), nil
	case len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF:
		// Encoding.UTF8, which replaces invalid bytes.
		return decodeUTF8Replacing(b[3:]), nil
	case len(b) >= 4 && b[0] == 0 && b[1] == 0 && b[2] == 0xFE && b[3] == 0xFF:
		return decodeUTF32(b[4:], true), nil
	}
	if err := streamReaderUTF8Error(b); err != nil {
		return nil, err
	}
	return utf16.Encode([]rune(string(b))), nil
}

// decodeUTF16 decodes as Encoding.Unicode / BigEndianUnicode: unpaired
// surrogates and a final odd byte become U+FFFD.
func decodeUTF16(b []byte, bigEndian bool) []uint16 {
	units := make([]uint16, 0, len(b)/2+1)
	for i := 0; i+1 < len(b); i += 2 {
		if bigEndian {
			units = append(units, uint16(b[i])<<8|uint16(b[i+1]))
		} else {
			units = append(units, uint16(b[i+1])<<8|uint16(b[i]))
		}
	}
	out := make([]uint16, 0, len(units)+1)
	for i := 0; i < len(units); i++ {
		u := units[i]
		switch {
		case u >= 0xD800 && u <= 0xDBFF:
			if i+1 < len(units) && units[i+1] >= 0xDC00 && units[i+1] <= 0xDFFF {
				out = append(out, u, units[i+1])
				i++
			} else {
				out = append(out, 0xFFFD)
			}
		case u >= 0xDC00 && u <= 0xDFFF:
			out = append(out, 0xFFFD)
		default:
			out = append(out, u)
		}
	}
	if len(b)%2 != 0 {
		out = append(out, 0xFFFD)
	}
	return out
}

// decodeUTF32 decodes as Encoding.UTF32 / UTF32Encoding(bigEndian): invalid
// code points and trailing bytes become U+FFFD.
func decodeUTF32(b []byte, bigEndian bool) []uint16 {
	var out []uint16
	i := 0
	for ; i+3 < len(b); i += 4 {
		var c uint32
		if bigEndian {
			c = uint32(b[i])<<24 | uint32(b[i+1])<<16 | uint32(b[i+2])<<8 | uint32(b[i+3])
		} else {
			c = uint32(b[i+3])<<24 | uint32(b[i+2])<<16 | uint32(b[i+1])<<8 | uint32(b[i])
		}
		if c > 0x10FFFF || (c >= 0xD800 && c <= 0xDFFF) {
			out = append(out, 0xFFFD)
		} else {
			out = utf16.AppendRune(out, rune(c))
		}
	}
	if i < len(b) {
		out = append(out, 0xFFFD)
	}
	return out
}

type anchored struct {
	node  any
	size  int
	depth int // nested containers in node
}

// reader is YamlJson.Reader: it reads the events of a YAML stream into a
// JSON value.
type reader struct {
	parser  *parser
	anchors map[string]anchored
	nodes   int
}

// at is YamlJson's At: " at line L, column C" for where an event starts.
func at(e *event) string {
	if e == nil || e.start.line <= 0 {
		return ""
	}
	return fmt.Sprintf(" at line %d, column %d", e.start.line, e.start.column)
}

var eventTypeNames = map[eventKind]string{
	evStreamStart: "StreamStart", evStreamEnd: "StreamEnd", evDocumentStart: "DocumentStart",
	evDocumentEnd: "DocumentEnd", evAlias: "AnchorAlias", evScalar: "Scalar",
	evSequenceStart: "SequenceStart", evSequenceEnd: "SequenceEnd", evMappingStart: "MappingStart",
	evMappingEnd: "MappingEnd",
}

// consume is ParserExtensions.Consume<T>.
func (r *reader) consume(kind eventKind) {
	r.require(kind)
	r.parser.moveNext()
}

// tryConsume is ParserExtensions.TryConsume<T>.
func (r *reader) tryConsume(kind eventKind) bool {
	if r.accept(kind) {
		r.parser.moveNext()
		return true
	}
	return false
}

func (r *reader) accept(kind eventKind) bool {
	if r.parser.current == nil {
		if !r.parser.moveNext() {
			throwCrash("EndOfStreamException")
		}
	}
	return r.parser.current.kind == kind
}

func (r *reader) require(kind eventKind) {
	if !r.accept(kind) {
		e := r.parser.current
		throwYAML(e.start, fmt.Sprintf("Expected '%s', got '%s'.", eventTypeNames[kind], eventTypeNames[e.kind]))
	}
}

func (r *reader) readStream() (any, int) {
	r.consume(evStreamStart)
	if !r.tryConsume(evDocumentStart) {
		throwFormat("The file holds no YAML document: it is empty or holds only comments.")
	}
	value, _, depth := r.read(0)
	r.consume(evDocumentEnd)
	if r.parser.current == nil || r.parser.current.kind != evStreamEnd {
		throwFormat("The file holds more than one YAML document; the second one starts%s. Keep only one"+
			" document: remove the '---' or '...' line that separates them.", at(r.parser.current))
	}
	r.consume(evStreamEnd)
	return value, depth
}

// read reads a node and returns it, with the number of nodes it holds and
// the number of nested containers in it.
func (r *reader) read(depth int) (any, int, int) {
	p := r.parser
	if depth > maxDepth {
		throwFormat("The file nests collections deeper than %d levels%s.", maxDepth, at(p.current))
	}
	if p.current != nil && p.current.kind == evAlias {
		alias := p.current
		p.moveNext()
		a, ok := r.anchors[alias.anchor]
		if !ok {
			throwFormat("The alias *%s%s names no earlier anchor. Define the anchor &%s before the alias uses it.",
				alias.anchor, at(alias), alias.anchor)
		}
		r.count(a.size)
		return Clone(a.node), a.size, a.depth
	}
	start := p.current
	if start == nil || (start.kind != evScalar && start.kind != evSequenceStart && start.kind != evMappingStart) {
		throwFormat("The file has an unexpected YAML element%s (%s).", at(start), eventString(start))
	}
	var node any
	var size, nested int
	switch start.kind {
	case evScalar:
		p.moveNext()
		node = readScalar(start)
		size = 1
		r.count(1)
	case evSequenceStart:
		p.moveNext()
		checkCollectionTag(start, "seq")
		array := []any{}
		size = 1
		r.count(1)
		for !r.tryConsume(evSequenceEnd) {
			item, itemSize, itemNested := r.read(depth + 1)
			array = append(array, item)
			size += itemSize
			nested = max(nested, itemNested)
		}
		nested++
		node = array
	case evMappingStart:
		p.moveNext()
		checkCollectionTag(start, "map")
		mapping := NewObject()
		size = 1
		r.count(1)
		for !r.tryConsume(evMappingEnd) {
			keyStart := p.current
			key, _, _ := r.read(depth + 1)
			name, ok := key.(string)
			if !ok {
				keyText := "null"
				if key != nil {
					keyText = string(MarshalJSONCompact(key))
				}
				throwFormat("The mapping key %s%s is not a string. Keys must be text; quote the key if it is"+
					" meant as text.", keyText, at(keyStart))
			}
			if mapping.Has(name) {
				throwFormat("The mapping key '%s'%s appears twice in the same mapping. Remove one of them.",
					name, at(keyStart))
			}
			value, valueSize, valueNested := r.read(depth + 1)
			mapping.Set(name, value)
			size += valueSize
			nested = max(nested, valueNested)
		}
		nested++
		node = mapping
	}
	if start.anchor != "" {
		r.anchors[start.anchor] = anchored{node, size, nested}
	}
	return node, size, nested
}

func eventString(e *event) string {
	if e == nil {
		return ""
	}
	return eventTypeNames[e.kind]
}

func (r *reader) count(nodes int) {
	r.nodes += nodes
	if r.nodes > maxNodes {
		throwFormat("The file expands to more than %d values through aliases. Use fewer aliases, or copy"+
			" fewer values with each.", maxNodes)
	}
}

func isNonSpecific(tag string) bool { return tag == "!" || tag == "?" }

func readScalar(s *event) any {
	value := s.value
	if s.tag == "" {
		if s.style == stylePlain {
			return resolvePlain(value)
		}
		return value
	}
	if isNonSpecific(s.tag) {
		return value
	}
	resolved := resolvePlain(value)
	switch s.tag {
	case coreTagPrefix + "str":
		return value
	case coreTagPrefix + "null":
		if resolved == nil {
			return nil
		}
	case coreTagPrefix + "bool":
		if _, ok := resolved.(bool); ok {
			return resolved
		}
	case coreTagPrefix + "int":
		if decimalRegex.MatchString(value) || octalRegex.MatchString(value) || hexadecimalRegex.MatchString(value) {
			return resolved
		}
	case coreTagPrefix + "float":
		if _, ok := resolved.(json.Number); ok {
			return resolved
		}
	}
	throwFormat("The value '%s'%s tagged %s is not valid. Remove the tag, or write a value the tag allows.",
		value, at(s), shortTag(s.tag))
	return nil
}

func checkCollectionTag(start *event, kind string) {
	tag := start.tag
	if tag != "" && !isNonSpecific(tag) && tag != coreTagPrefix+kind {
		what := "mapping"
		if kind == "seq" {
			what = "sequence"
		}
		throwFormat("The tag %s%s is not valid here, on a %s. Remove the tag.", shortTag(tag), at(start), what)
	}
}

// shortTag is a tag of the core schema in its usual short form, like !!int.
func shortTag(tag string) string {
	if len(tag) >= len(coreTagPrefix) && tag[:len(coreTagPrefix)] == coreTagPrefix {
		return "!!" + tag[len(coreTagPrefix):]
	}
	return tag
}

// resolvePlain is YamlJson.ResolvePlain: the value of a plain scalar by the
// YAML 1.2 core schema. Numbers are json.Number holding the text
// System.Text.Json writes for them. It panics with a *formatError for .inf,
// .nan and floats out of range.
func resolvePlain(text string) any {
	if nullRegex.MatchString(text) {
		return nil
	}
	if boolRegex.MatchString(text) {
		return text[0] == 't' || text[0] == 'T'
	}
	if decimalRegex.MatchString(text) {
		n, _ := new(big.Int).SetString(text, 10)
		return json.Number(n.String())
	}
	if octalRegex.MatchString(text) {
		n, _ := new(big.Int).SetString(text[2:], 8)
		return json.Number(n.String())
	}
	if hexadecimalRegex.MatchString(text) {
		n, _ := new(big.Int).SetString(text[2:], 16)
		return json.Number(n.String())
	}
	if floatRegex.MatchString(text) {
		f, _ := strconv.ParseFloat(text, 64)
		if math.IsInf(f, 0) || math.IsNaN(f) {
			throwFormat("The number %s is out of range. Quote it (\"%s\") if it is meant as text.", text, text)
		}
		return json.Number(formatDouble(f))
	}
	if infinityRegex.MatchString(text) || notANumberRegex.MatchString(text) {
		throwFormat("The value %s cannot be represented in JSON. Quote it (\"%s\") if it is meant as text.", text, text)
	}
	return text
}

// readsBackAsString is YamlJson.ReadsBackAsString.
func readsBackAsString(value string) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			if _, isFormat := r.(*formatError); !isFormat {
				panic(r)
			}
			ok = false
		}
	}()
	_, ok = resolvePlain(value).(string)
	return ok
}

// formatDouble formats a finite double as Utf8JsonWriter.WriteNumberValue
// (double.TryFormat with the default format) does: the shortest text that
// round-trips, in fixed notation when the decimal exponent is from -4 to 16
// (0.0001, 10000000000000000), else as d.dddE+XX (1E-05, 1E+17).
func formatDouble(f float64) string {
	if f == 0 {
		if math.Signbit(f) {
			return "-0"
		}
		return "0"
	}
	s := strconv.FormatFloat(f, 'e', -1, 64) // -d.ddde±XX
	neg := s[0] == '-'
	if neg {
		s = s[1:]
	}
	ePos := 0
	for i := range len(s) {
		if s[i] == 'e' {
			ePos = i
			break
		}
	}
	mant, expText := s[:ePos], s[ePos+1:]
	exp, _ := strconv.Atoi(expText)
	digits := mant[:1]
	if len(mant) > 2 {
		digits += mant[2:]
	}
	scale := exp + 1 // value = 0.digits × 10^scale
	var out []byte
	if neg {
		out = append(out, '-')
	}
	if scale > 17 || scale < -3 {
		out = append(out, digits[0])
		if len(digits) > 1 {
			out = append(out, '.')
			out = append(out, digits[1:]...)
		}
		out = append(out, 'E')
		e := scale - 1
		if e < 0 {
			out = append(out, '-')
			e = -e
		} else {
			out = append(out, '+')
		}
		if e < 10 {
			out = append(out, '0')
		}
		out = strconv.AppendInt(out, int64(e), 10)
		return string(out)
	}
	if scale <= 0 {
		out = append(out, '0', '.')
		for range -scale {
			out = append(out, '0')
		}
		out = append(out, digits...)
		return string(out)
	}
	if len(digits) <= scale {
		out = append(out, digits...)
		for range scale - len(digits) {
			out = append(out, '0')
		}
		return string(out)
	}
	out = append(out, digits[:scale]...)
	out = append(out, '.')
	out = append(out, digits[scale:]...)
	return string(out)
}
