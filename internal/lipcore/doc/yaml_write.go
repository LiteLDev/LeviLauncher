package doc

// MarshalYAML and the emitter below port lip's YamlJson.Serialize
// (src/Lip.Core/Json/YamlJson.cs) together with the parts of YamlDotNet
// 16.3.0's Emitter (YamlDotNet/Core/Emitter.cs) it reaches: block mappings
// and sequences, empty collections in flow style, and plain, single-quoted,
// double-quoted and folded scalars, with Emitter(writer, bestIndent: 2,
// bestWidth: int.MaxValue, isCanonical: false) and the default settings
// (sequences not indented inside mappings, simple keys up to 1024 UTF-16
// code units, no UTF-16 surrogate pair escapes).
//
// Copyright (c) Antoine Aubry and contributors (YamlDotNet, MIT license).

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf16"
)

const (
	bestIndent         = 2
	maxSimpleKeyLength = 1024
)

// MarshalYAML writes v as lip writes YAML files (YamlJson.Serialize, then
// UTF-8 without a byte order mark): block style indented by two spaces,
// object members in order, empty objects and arrays as {} and [], numbers as
// their JSON text, true/false/null plain, and every string in the style
// YamlDotNet picks for ScalarStyle.Any, except that a string a plain scalar
// would not read back as (such as "", "1.0", "true", "null"), a YAML 1.1
// boolean (yes, no, on, off, ...) or a string with a control character is
// double-quoted. Line breaks are "\n" and the text ends with one. v must hold
// only the value types of this package; it panics otherwise.
func MarshalYAML(v any) []byte {
	e := &emitter{indent: -1, isWhitespace: true, isIndentation: true}
	// DocumentStart (implicit): CheckEmptyDocument writes "---" when the
	// document is an empty scalar.
	if s, ok := v.(string); ok && s == "" {
		e.writeIndent()
		e.writeIndicator("---", true, false, false)
	}
	e.emitNode(v, false, false)
	// DocumentEnd (implicit).
	e.writeIndent()
	return replaceLineEndings(e.out)
}

type scalarData struct {
	value               []uint16
	isMultiline         bool
	isFlowPlainAllowed  bool
	isBlockPlainAllowed bool
	isSingleQuotedAllow bool
	isBlockAllowed      bool
	hasSingleQuotes     bool
	style               scalarStyle
}

type emitter struct {
	out []uint16

	indents            []int
	indent             int
	flowLevel          int
	isMappingContext   bool
	isSimpleKeyContext bool
	column             int
	isWhitespace       bool
	isIndentation      bool
}

// emitNode is Emitter.EmitNode for a whole value: the events of v, from its
// start event to its end event.
func (e *emitter) emitNode(v any, isMapping, isSimpleKey bool) {
	e.isMappingContext = isMapping
	e.isSimpleKeyContext = isSimpleKey
	switch t := v.(type) {
	case *Object:
		if t.Len() == 0 {
			e.emitEmptyFlow('{', '}')
			return
		}
		// BlockMappingFirstKey .. MappingEnd.
		e.increaseIndent(false, false)
		for i, k := range t.Keys() {
			key := e.stringScalar(k)
			e.writeIndent()
			if e.checkSimpleKey(key) {
				e.emitScalar(key, true, true)
				e.writeIndicator(":", false, false, false)
				e.emitNode(t.vals[i], true, false)
			} else {
				e.writeIndicator("?", true, false, true)
				e.emitScalar(key, true, false)
				e.writeIndent()
				e.writeIndicator(":", true, false, true)
				e.emitNode(t.vals[i], true, false)
			}
		}
		e.popIndent()
	case []any:
		if len(t) == 0 {
			e.emitEmptyFlow('[', ']')
			return
		}
		// BlockSequenceFirstItem .. SequenceEnd.
		e.increaseIndent(false, e.isMappingContext && !e.isIndentation)
		for _, item := range t {
			e.writeIndent()
			e.writeIndicator("-", true, false, true)
			e.emitNode(item, false, false)
		}
		e.popIndent()
	case string:
		e.emitScalar(e.stringScalar(t), isMapping, isSimpleKey)
	case json.Number:
		e.emitScalar(e.plainScalar(string(t)), isMapping, isSimpleKey)
	case bool:
		text := "false"
		if t {
			text = "true"
		}
		e.emitScalar(e.plainScalar(text), isMapping, isSimpleKey)
	case nil:
		e.emitScalar(e.plainScalar("null"), isMapping, isSimpleKey)
	default:
		panic(fmt.Sprintf("doc: %T is not a JSON value", v))
	}
}

// emitEmptyFlow writes an empty collection in flow style
// (EmitFlowSequenceItem / EmitFlowMappingKey, first item and end; the
// indent and flow level they raise are restored before anything else is
// written, so only the indicators remain).
func (e *emitter) emitEmptyFlow(open, close byte) {
	e.writeIndicator(string(open), true, true, false)
	e.writeIndicator(string(close), false, false, false)
}

// stringScalar is YamlJson.EmitString: ScalarStyle.Any when the plain form
// reads back as the string, it is no YAML 1.1 boolean and it has no control
// character; else ScalarStyle.DoubleQuoted.
func (e *emitter) stringScalar(value string) *scalarData {
	style := styleDoubleQuoted
	if readsBackAsString(value) && !yaml11BoolRegex.MatchString(value) && !hasControl(value) {
		style = styleAny
	}
	sd := analyzeScalar(utf16.Encode([]rune(value)))
	sd.style = style
	return sd
}

// plainScalar is YamlJson.EmitPlain.
func (e *emitter) plainScalar(text string) *scalarData {
	sd := analyzeScalar(utf16.Encode([]rune(text)))
	sd.style = stylePlain
	return sd
}

// hasControl is value.Any(char.IsControl): U+0000..U+001F, U+007F..U+009F.
func hasControl(s string) bool {
	for _, r := range s {
		if r <= 0x1F || (r >= 0x7F && r <= 0x9F) {
			return true
		}
	}
	return false
}

func (e *emitter) checkSimpleKey(key *scalarData) bool {
	if key.isMultiline {
		return false
	}
	return len(key.value) <= maxSimpleKeyLength
}

func (e *emitter) emitScalar(sd *scalarData, isMapping, isSimpleKey bool) {
	e.isMappingContext = isMapping
	e.isSimpleKeyContext = isSimpleKey
	e.selectScalarStyle(sd)
	e.increaseIndent(true, false)
	switch sd.style {
	case stylePlain:
		e.writePlainScalar(sd.value)
	case styleSingleQuoted:
		e.writeSingleQuotedScalar(sd.value)
	case styleDoubleQuoted:
		e.writeDoubleQuotedScalar(sd.value)
	case styleFolded:
		e.writeFoldedScalar(sd.value)
	default:
		panic("doc: unexpected scalar style")
	}
	e.popIndent()
}

func (e *emitter) selectScalarStyle(sd *scalarData) {
	style := sd.style
	if style == styleAny {
		if sd.isMultiline {
			style = styleFolded
		} else {
			style = stylePlain
		}
	}
	if e.isSimpleKeyContext && sd.isMultiline {
		style = styleDoubleQuoted
	}
	if style == stylePlain {
		if (e.flowLevel != 0 && !sd.isFlowPlainAllowed) || (e.flowLevel == 0 && !sd.isBlockPlainAllowed) {
			if sd.isSingleQuotedAllow && !sd.hasSingleQuotes {
				style = styleSingleQuoted
			} else {
				style = styleDoubleQuoted
			}
		}
		if len(sd.value) == 0 && (e.flowLevel != 0 || e.isSimpleKeyContext) {
			style = styleSingleQuoted
		}
	}
	if style == styleSingleQuoted && !sd.isSingleQuotedAllow {
		style = styleDoubleQuoted
	}
	if style == styleLiteral || style == styleFolded {
		if !sd.isBlockAllowed || e.flowLevel != 0 || e.isSimpleKeyContext {
			style = styleDoubleQuoted
		}
	}
	sd.style = style
}

func isPrintable(c uint16) bool {
	return c == 0x9 || c == 0xA || c == 0xD || (c >= 0x20 && c <= 0x7E) || c == 0x85 ||
		(c >= 0xA0 && c <= 0xD7FF) || (c >= 0xE000 && c <= 0xFFFD)
}

func isWhiteBreakOrZeroAt(v []uint16, i int) bool {
	var c uint16
	if i < len(v) {
		c = v[i]
	}
	return c == ' ' || c == '\t' || isBreakChar(c) || c == 0
}

func inSet(c uint16, set string) bool {
	for i := 0; i < len(set); i++ {
		if uint16(set[i]) == c {
			return true
		}
	}
	return false
}

// analyzeScalar is Emitter.AnalyzeScalar for an untagged scalar written to
// a Unicode TextWriter.
func analyzeScalar(value []uint16) *scalarData {
	sd := &scalarData{value: value}
	if len(value) == 0 {
		sd.isSingleQuotedAllow = true
		return sd
	}
	var flowIndicators, blockIndicators bool
	if hasPrefix16(value, "---") || hasPrefix16(value, "...") {
		flowIndicators = true
		blockIndicators = true
	}
	preceededByWhitespace := true
	followedByWhitespace := isWhiteBreakOrZeroAt(value, 1)
	var leadingSpace, leadingBreak, trailingSpace, trailingBreak, leadingQuote, breakSpace, spaceBreak,
		previousSpace, previousBreak, lineOfSpaces, lineBreaks, specialCharacters, singleQuotes, linesOfSpaces bool
	isFirst := true
	for pos := 0; pos < len(value); pos++ {
		c := value[pos]
		if isFirst {
			if inSet(c, "#,[]{}&*!|>\"%@`'") {
				flowIndicators = true
				blockIndicators = true
				leadingQuote = c == '\''
				singleQuotes = singleQuotes || c == '\''
			}
			if inSet(c, "?:") {
				flowIndicators = true
				if followedByWhitespace {
					blockIndicators = true
				}
			}
			if c == '-' && followedByWhitespace {
				flowIndicators = true
				blockIndicators = true
			}
		} else {
			if inSet(c, ",?[]{}") {
				flowIndicators = true
			}
			if c == ':' {
				flowIndicators = true
				if followedByWhitespace {
					blockIndicators = true
				}
			}
			if c == '#' && preceededByWhitespace {
				flowIndicators = true
				blockIndicators = true
			}
			singleQuotes = singleQuotes || c == '\''
		}
		if !specialCharacters && !isPrintable(c) {
			specialCharacters = true
		}
		if isBreakChar(c) {
			lineBreaks = true
		}
		if c == ' ' {
			if isFirst {
				leadingSpace = true
			}
			if pos >= len(value)-1 {
				trailingSpace = true
			}
			if previousBreak {
				breakSpace = true
				lineOfSpaces = true
			}
			previousSpace = true
			previousBreak = false
		} else if isBreakChar(c) {
			if isFirst {
				leadingBreak = true
			}
			if pos >= len(value)-1 {
				trailingBreak = true
			}
			if previousSpace {
				spaceBreak = true
			}
			if lineOfSpaces {
				linesOfSpaces = true
			}
			previousSpace = false
			previousBreak = true
		} else {
			previousSpace = false
			previousBreak = false
			lineOfSpaces = false
		}
		preceededByWhitespace = isWhiteBreakOrZeroAt(value, pos)
		if pos+1 < len(value) {
			followedByWhitespace = isWhiteBreakOrZeroAt(value, pos+2)
		}
		isFirst = false
	}
	sd.isFlowPlainAllowed = true
	sd.isBlockPlainAllowed = true
	sd.isSingleQuotedAllow = true
	sd.isBlockAllowed = true
	if leadingSpace || leadingBreak || trailingSpace || trailingBreak || leadingQuote {
		sd.isFlowPlainAllowed = false
		sd.isBlockPlainAllowed = false
	}
	if trailingSpace {
		sd.isBlockAllowed = false
	}
	if breakSpace || spaceBreak || specialCharacters {
		sd.isFlowPlainAllowed = false
		sd.isBlockPlainAllowed = false
		sd.isSingleQuotedAllow = false
	}
	if linesOfSpaces {
		sd.isBlockAllowed = false
	}
	sd.isMultiline = lineBreaks
	if lineBreaks {
		sd.isFlowPlainAllowed = false
		sd.isBlockPlainAllowed = false
	}
	if flowIndicators {
		sd.isFlowPlainAllowed = false
	}
	if blockIndicators {
		sd.isBlockPlainAllowed = false
	}
	sd.hasSingleQuotes = singleQuotes
	return sd
}

func hasPrefix16(v []uint16, p string) bool {
	if len(v) < len(p) {
		return false
	}
	for i := 0; i < len(p); i++ {
		if v[i] != uint16(p[i]) {
			return false
		}
	}
	return true
}

func (e *emitter) increaseIndent(isFlow, isIndentless bool) {
	e.indents = append(e.indents, e.indent)
	if e.indent < 0 {
		if isFlow {
			e.indent = bestIndent
		} else {
			e.indent = 0
		}
	} else if !isIndentless {
		// forceIndentLess is set (IndentSequences is false), so only
		// indentless sequences keep the indent.
		e.indent += bestIndent
	}
}

func (e *emitter) popIndent() {
	e.indent = e.indents[len(e.indents)-1]
	e.indents = e.indents[:len(e.indents)-1]
}

func (e *emitter) write(c uint16) {
	e.out = append(e.out, c)
	e.column++
}

func (e *emitter) writeString(s string) {
	u := utf16.Encode([]rune(s))
	e.out = append(e.out, u...)
	e.column += len(u)
}

func (e *emitter) writeBreak(c uint16) {
	e.out = append(e.out, c)
	e.column = 0
}

func (e *emitter) writeIndicator(indicator string, needWhitespace, whitespace, indentation bool) {
	if needWhitespace && !e.isWhitespace {
		e.write(' ')
	}
	e.writeString(indicator)
	e.isWhitespace = whitespace
	e.isIndentation = e.isIndentation && indentation
}

func (e *emitter) writeIndent() {
	currentIndent := max(e.indent, 0)
	if !e.isIndentation || e.column > currentIndent || (e.column == currentIndent && !e.isWhitespace) {
		e.writeBreak('\n')
	}
	for e.column < currentIndent {
		e.write(' ')
	}
	e.isWhitespace = true
	e.isIndentation = true
}

// writePlainScalar writes a plain scalar, which never holds a line break
// (that makes it multiline, which is never plain) and is never folded
// (bestWidth is int.MaxValue).
func (e *emitter) writePlainScalar(value []uint16) {
	if !e.isWhitespace {
		e.write(' ')
	}
	for _, c := range value {
		e.write(c)
	}
	e.isWhitespace = false
	e.isIndentation = false
}

func (e *emitter) writeSingleQuotedScalar(value []uint16) {
	e.writeIndicator("'", true, false, false)
	for _, c := range value {
		if c == '\'' {
			e.write(c)
		}
		e.write(c)
	}
	e.writeIndicator("'", false, false, false)
	e.isWhitespace = false
	e.isIndentation = false
}

func (e *emitter) writeHex(code uint32, digits int) {
	const hex = "0123456789ABCDEF"
	for i := digits - 1; i >= 0; i-- {
		e.write(uint16(hex[(code>>(4*uint(i)))&0xF]))
	}
}

func (e *emitter) writeDoubleQuotedScalar(value []uint16) {
	e.writeIndicator("\"", true, false, false)
	for i := 0; i < len(value); i++ {
		c := value[i]
		if !isPrintable(c) || isBreakChar(c) || c == '"' || c == '\\' {
			e.write('\\')
			switch c {
			case 0:
				e.write('0')
			case 0x7:
				e.write('a')
			case 0x8:
				e.write('b')
			case 0x9:
				e.write('t')
			case 0xA:
				e.write('n')
			case 0xB:
				e.write('v')
			case 0xC:
				e.write('f')
			case 0xD:
				e.write('r')
			case 0x1B:
				e.write('e')
			case '"':
				e.write('"')
			case '\\':
				e.write('\\')
			case 0x85:
				e.write('N')
			case 0xA0:
				e.write('_')
			case 0x2028:
				e.write('L')
			case 0x2029:
				e.write('P')
			default:
				switch {
				case c <= 0xFF:
					e.write('x')
					e.writeHex(uint32(c), 2)
				case c >= 0xD800 && c <= 0xDBFF:
					if i+1 < len(value) && value[i+1] >= 0xDC00 && value[i+1] <= 0xDFFF {
						e.write('U')
						e.writeHex(uint32(utf16.DecodeRune(rune(c), rune(value[i+1]))), 8)
						i++
					} else {
						panic("doc: orphaned high surrogate")
					}
				default:
					e.write('u')
					e.writeHex(uint32(c), 4)
				}
			}
		} else {
			e.write(c)
		}
	}
	e.writeIndicator("\"", false, false, false)
	e.isWhitespace = false
	e.isIndentation = false
}

func isBlank(c uint16) bool { return c == ' ' || c == '\t' }

// emitterBreak is Emitter.IsBreak: CR, LF and NEL are written as a line
// break, LS and PS as themselves.
func emitterBreak(c uint16) (uint16, bool) {
	switch c {
	case '\r', '\n', 0x85:
		return '\n', true
	case 0x2028, 0x2029:
		return c, true
	}
	return 0, false
}

func (e *emitter) writeBlockScalarHints(value []uint16) {
	if len(value) > 0 && (value[0] == ' ' || isBreakChar(value[0])) {
		e.writeIndicator(fmt.Sprint(bestIndent), false, false, false)
	}
	chompHint := ""
	if len(value) == 0 || !isBreakChar(value[len(value)-1]) {
		chompHint = "-"
	} else if len(value) >= 2 && isBreakChar(value[len(value)-2]) {
		chompHint = "+"
	}
	if chompHint != "" {
		e.writeIndicator(chompHint, false, false, false)
	}
}

func (e *emitter) writeFoldedScalar(value []uint16) {
	previousBreak := true
	leadingSpaces := true
	e.writeIndicator(">", true, false, false)
	e.writeBlockScalarHints(value)
	e.writeBreak('\n')
	e.isIndentation = true
	e.isWhitespace = true
	for i := 0; i < len(value); i++ {
		c := value[i]
		if breakCharacter, ok := emitterBreak(c); ok {
			if c == '\r' && i+1 < len(value) && value[i+1] == '\n' {
				continue
			}
			if !previousBreak && !leadingSpaces && breakCharacter == '\n' {
				k := 0
				for i+k < len(value) {
					if _, ok := emitterBreak(value[i+k]); !ok {
						break
					}
					k++
				}
				if i+k < len(value) {
					_, isBrk := emitterBreak(value[i+k])
					if !(isBlank(value[i+k]) || isBrk) {
						e.writeBreak('\n')
					}
				}
			}
			e.writeBreak(breakCharacter)
			e.isIndentation = true
			previousBreak = true
		} else {
			if previousBreak {
				e.writeIndent()
				leadingSpaces = isBlank(c)
			}
			e.write(c)
			e.isIndentation = false
			previousBreak = false
		}
	}
}

// replaceLineEndings is string.ReplaceLineEndings("\n") (CR LF, CR, LF, NEL,
// LS, FF and PS each become LF), encoded as UTF-8.
func replaceLineEndings(u []uint16) []byte {
	var b strings.Builder
	runes := utf16.Decode(u)
	for i := 0; i < len(runes); i++ {
		switch r := runes[i]; r {
		case '\r':
			if i+1 < len(runes) && runes[i+1] == '\n' {
				i++
			}
			b.WriteByte('\n')
		case '\n', 0x85, 0x2028, 0x0C, 0x2029:
			b.WriteByte('\n')
		default:
			b.WriteRune(r)
		}
	}
	return []byte(b.String())
}
