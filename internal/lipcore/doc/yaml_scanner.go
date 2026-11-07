package doc

// This file ports YamlDotNet 16.3.0's Scanner (YamlDotNet/Core/Scanner.cs,
// with Cursor.cs, SimpleKey.cs, CharacterAnalyzer.cs and InsertionQueue.cs),
// the tokenizer lip's YamlJson reads YAML with. It works on UTF-16 code units
// as .NET does, so marks (line, column) match YamlDotNet's.
//
// Copyright (c) Antoine Aubry and contributors (YamlDotNet, MIT license).

import (
	"unicode/utf16"
	"unicode/utf8"
)

// mark is YamlDotNet's Mark: a position with a 1-based line and column.
type mark struct {
	index, line, column int64
}

// emptyMark is Mark.Empty.
var emptyMark = mark{0, 1, 1}

// yamlError is a YamlException (SyntaxErrorException,
// SemanticErrorException): it carries the mark where the problem starts.
type yamlError struct {
	start mark
	msg   string
}

// yamlCrash is any other exception YamlDotNet throws while reading
// (InvalidOperationException, ArgumentException, ...); lip does not catch
// those.
type yamlCrash struct {
	msg string
}

func throwYAML(start mark, msg string) { panic(&yamlError{start, msg}) }
func throwCrash(msg string)            { panic(&yamlCrash{msg}) }

type tokenKind int

const (
	tokStreamStart tokenKind = iota
	tokStreamEnd
	tokVersionDirective
	tokTagDirective
	tokDocumentStart
	tokDocumentEnd
	tokBlockSequenceStart
	tokBlockMappingStart
	tokBlockEnd
	tokFlowSequenceStart
	tokFlowSequenceEnd
	tokFlowMappingStart
	tokFlowMappingEnd
	tokBlockEntry
	tokFlowEntry
	tokKey
	tokValue
	tokAlias
	tokAnchor
	tokTag
	tokScalar
	tokError
)

type scalarStyle int

// ScalarStyle values of YamlDotNet.
const (
	styleAny scalarStyle = iota
	stylePlain
	styleSingleQuoted
	styleDoubleQuoted
	styleLiteral
	styleFolded
	styleForcePlain
)

type token struct {
	kind       tokenKind
	start, end mark
	// value is the scalar value, the anchor or alias name, the error message,
	// the tag suffix, or the %TAG prefix.
	value string
	// handle is the tag handle or the %TAG handle.
	handle       string
	style        scalarStyle
	isKey        bool
	major, minor int
}

type simpleKey struct {
	isPossible  bool
	isRequired  bool
	tokenNumber int
	index       int64
	line        int64
	lineOffset  int64
}

func (k *simpleKey) mark() mark { return mark{k.index, k.line, k.lineOffset + 1} }

const maxVersionNumberLength = 9

var simpleEscapeCodes = map[uint16]uint16{
	'0': 0, 'a': 0x07, 'b': 0x08, 't': 0x09, '\t': 0x09, 'n': 0x0A, 'v': 0x0B,
	'f': 0x0C, 'r': 0x0D, 'e': 0x1B, ' ': 0x20, '"': '"', '\\': '\\', '/': '/',
	'N': 0x85, '_': 0xA0, 'L': 0x2028, 'P': 0x2029,
}

type scanner struct {
	text []uint16
	pos  int

	// cursor
	cIndex, cLine, cLineOffset int64

	indents                      []int64
	tokens                       []*token
	simpleKeys                   []*simpleKey
	streamStartProduced          bool
	streamEndProduced            bool
	plainScalarFollowedByComment bool
	flowCollectionFetched        bool
	startFlowCollectionFetched   bool
	indent                       int64
	flowScalarFetched            bool
	simpleKeyAllowed             bool
	flowLevel                    int
	tokensParsed                 int
	tokenAvailable               bool
	previous                     *token
	previousAnchor               *token
	lastScalar                   *token
	maxKeySize                   int64

	current *token
}

func newScanner(text []uint16) *scanner {
	return &scanner{text: text, cLine: 1, indent: -1, maxKeySize: 1024}
}

// --- LookAheadBuffer / CharacterAnalyzer ---

func (s *scanner) endOfInput() bool { return s.pos >= len(s.text) }

func (s *scanner) peek(offset int) uint16 {
	if i := s.pos + offset; i < len(s.text) {
		return s.text[i]
	}
	return 0
}

func (s *scanner) check(c uint16, offset int) bool { return s.peek(offset) == c }

func (s *scanner) checkAny(chars string, offset int) bool {
	c := s.peek(offset)
	for i := 0; i < len(chars); i++ {
		if uint16(chars[i]) == c {
			return true
		}
	}
	return false
}

func isAlphaNumericDashOrUnderscore(c uint16) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_' || c == '-'
}

func (s *scanner) isAlphaNumericDashOrUnderscore() bool {
	return isAlphaNumericDashOrUnderscore(s.peek(0))
}

func (s *scanner) isDigit(offset int) bool { c := s.peek(offset); return c >= '0' && c <= '9' }
func (s *scanner) asDigit(offset int) int  { return int(s.peek(offset)) - '0' }

func (s *scanner) isHex(offset int) bool {
	c := s.peek(offset)
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')
}

func (s *scanner) asHex(offset int) int32 {
	c := s.peek(offset)
	if c <= '9' {
		return int32(c) - '0'
	}
	if c <= 'F' {
		return int32(c) - 'A' + 10
	}
	return int32(c) - 'a' + 10
}

func (s *scanner) isSpace(offset int) bool { return s.check(' ', offset) }
func (s *scanner) isZero(offset int) bool  { return s.check(0, offset) }
func (s *scanner) isTab(offset int) bool   { return s.check('\t', offset) }
func (s *scanner) isWhite(offset int) bool { return s.isSpace(offset) || s.isTab(offset) }

func isBreakChar(c uint16) bool {
	return c == '\r' || c == '\n' || c == 0x85 || c == 0x2028 || c == 0x2029
}

func (s *scanner) isBreak(offset int) bool       { return isBreakChar(s.peek(offset)) }
func (s *scanner) isCrLf(offset int) bool        { return s.check('\r', offset) && s.check('\n', offset+1) }
func (s *scanner) isBreakOrZero(offset int) bool { return s.isBreak(offset) || s.isZero(offset) }
func (s *scanner) isWhiteBreakOrZero(offset int) bool {
	return s.isWhite(offset) || s.isBreakOrZero(offset)
}

// --- Cursor ---

func (s *scanner) cursorMark() mark { return mark{s.cIndex, s.cLine, s.cLineOffset + 1} }

func (s *scanner) skip() {
	s.cIndex++
	s.cLineOffset++
	s.pos++
}

func (s *scanner) skipLine() {
	if s.isCrLf(0) {
		s.cIndex += 2
		s.cLine++
		s.cLineOffset = 0
		s.pos += 2
	} else if s.isBreak(0) {
		s.cIndex++
		s.cLine++
		s.cLineOffset = 0
		s.pos++
	} else if !s.isZero(0) {
		throwCrash("Not at a break.")
	}
}

func (s *scanner) readCurrentCharacter() uint16 {
	c := s.peek(0)
	s.skip()
	return c
}

func (s *scanner) readLine() uint16 {
	if s.checkAny("\r\n", 0) || s.check(0x85, 0) {
		s.skipLine()
		return '\n'
	}
	c := s.peek(0)
	s.skipLine()
	return c
}

// --- token queue (InsertionQueue) ---

func (s *scanner) enqueue(t *token) { s.tokens = append(s.tokens, t) }

func (s *scanner) insert(index int, t *token) {
	if index > len(s.tokens) || index < 0 {
		throwCrash("Cannot insert outside of the bounds of the queue")
	}
	s.tokens = append(s.tokens, nil)
	copy(s.tokens[index+1:], s.tokens[index:])
	s.tokens[index] = t
}

func (s *scanner) enqueueError(msg string, start, end mark) {
	s.enqueue(&token{kind: tokError, value: msg, start: start, end: end})
}

// --- IScanner ---

func (s *scanner) moveNextWithoutConsuming() bool {
	if !s.tokenAvailable && !s.streamEndProduced {
		s.fetchMoreTokens()
	}
	if len(s.tokens) > 0 {
		s.current = s.tokens[0]
		s.tokens = s.tokens[1:]
		s.tokenAvailable = false
		return true
	}
	s.current = nil
	return false
}

func (s *scanner) consumeCurrent() {
	s.tokensParsed++
	s.tokenAvailable = false
	s.previous = s.current
	s.current = nil
}

func (s *scanner) isDocumentStart() bool {
	return !s.endOfInput() && s.cLineOffset == 0 && s.check('-', 0) && s.check('-', 1) && s.check('-', 2) && s.isWhiteBreakOrZero(3)
}

func (s *scanner) isDocumentEnd() bool {
	return !s.endOfInput() && s.cLineOffset == 0 && s.check('.', 0) && s.check('.', 1) && s.check('.', 2) && s.isWhiteBreakOrZero(3)
}

func (s *scanner) isDocumentIndicator() bool { return s.isDocumentStart() || s.isDocumentEnd() }

func (s *scanner) topKey() *simpleKey { return s.simpleKeys[len(s.simpleKeys)-1] }

func (s *scanner) fetchMoreTokens() {
	for {
		needsMoreTokens := false
		if len(s.tokens) == 0 {
			needsMoreTokens = true
		} else {
			for i := len(s.simpleKeys) - 1; i >= 0; i-- {
				k := s.simpleKeys[i]
				if k.isPossible && k.tokenNumber == s.tokensParsed {
					needsMoreTokens = true
					break
				}
			}
		}
		if !needsMoreTokens {
			break
		}
		s.fetchNextToken()
	}
	s.tokenAvailable = true
}

func (s *scanner) staleSimpleKeys() {
	for i := len(s.simpleKeys) - 1; i >= 0; i-- {
		key := s.simpleKeys[i]
		if key.isPossible && (key.line < s.cLine || key.index+s.maxKeySize < s.cIndex) {
			if key.isRequired {
				m := s.cursorMark()
				s.enqueueError("While scanning a simple key, could not find expected ':'.", m, m)
			}
			key.isPossible = false
		}
	}
}

func (s *scanner) fetchNextToken() {
	if !s.streamStartProduced {
		s.fetchStreamStart()
		return
	}
	s.scanToNextToken()
	s.staleSimpleKeys()
	s.unrollIndent(s.cLineOffset)

	if s.endOfInput() {
		s.lastScalar = nil
		s.fetchStreamEnd()
	}
	if s.cLineOffset == 0 && s.check('%', 0) {
		s.lastScalar = nil
		s.fetchDirective()
		return
	}
	if s.isDocumentStart() {
		s.lastScalar = nil
		s.fetchDocumentIndicator(true)
		return
	}
	if s.isDocumentEnd() {
		s.lastScalar = nil
		s.fetchDocumentIndicator(false)
		return
	}
	if s.check('[', 0) {
		s.lastScalar = nil
		s.fetchFlowCollectionStart(true)
		return
	}
	if s.check('{', 0) {
		s.lastScalar = nil
		s.fetchFlowCollectionStart(false)
		return
	}
	if s.check(']', 0) {
		s.lastScalar = nil
		s.fetchFlowCollectionEnd(true)
		return
	}
	if s.check('}', 0) {
		s.lastScalar = nil
		s.fetchFlowCollectionEnd(false)
		return
	}
	if s.check(',', 0) {
		s.lastScalar = nil
		s.fetchFlowEntry()
		return
	}
	if s.check('-', 0) {
		if s.isWhiteBreakOrZero(1) {
			s.fetchBlockEntry()
			return
		} else if s.flowLevel > 0 && s.checkAny(",[]{}", 1) {
			m := s.cursorMark()
			s.enqueueError("Invalid key indicator format.", m, m)
		}
	}
	if s.check('?', 0) && (s.flowLevel > 0 || s.isWhiteBreakOrZero(1)) {
		if s.isWhiteBreakOrZero(1) {
			s.fetchKey()
			return
		}
	}
	if s.check(':', 0) &&
		(s.flowLevel > 0 || s.isWhiteBreakOrZero(1)) &&
		!(s.simpleKeyAllowed && s.flowLevel > 0) &&
		!(s.flowScalarFetched && s.check(':', 1)) {
		if s.isWhiteBreakOrZero(1) || s.check(',', 1) || s.flowScalarFetched || s.flowCollectionFetched || s.startFlowCollectionFetched {
			if s.lastScalar != nil {
				s.lastScalar.isKey = true
				s.lastScalar = nil
			}
			s.fetchValue()
			return
		}
	}
	if s.check('*', 0) {
		s.fetchAnchor(true)
		return
	}
	if s.check('&', 0) {
		s.fetchAnchor(false)
		return
	}
	if s.check('!', 0) {
		s.fetchTag()
		return
	}
	if s.check('|', 0) && s.flowLevel == 0 {
		s.fetchBlockScalar(true)
		return
	}
	if s.check('>', 0) && s.flowLevel == 0 {
		s.fetchBlockScalar(false)
		return
	}
	if s.check('\'', 0) {
		s.fetchQuotedScalar(true)
		return
	}
	if s.check('"', 0) {
		s.fetchQuotedScalar(false)
		return
	}
	isInvalidPlainScalarCharacter := s.isWhiteBreakOrZero(0) || s.checkAny("-?:,[]{}#&*!|>'\"%@`", 0)
	isPlainScalar := !isInvalidPlainScalarCharacter ||
		(s.check('-', 0) && !s.isWhite(1)) ||
		(s.checkAny("?:", 0) && !s.isWhiteBreakOrZero(1)) ||
		(s.simpleKeyAllowed && s.flowLevel > 0)
	if isPlainScalar {
		if s.plainScalarFollowedByComment {
			m := s.cursorMark()
			s.enqueueError("While scanning plain scalar, found a comment between adjacent scalars.", m, m)
		}
		if s.flowScalarFetched || s.flowCollectionFetched && !s.startFlowCollectionFetched {
			if s.check(':', 0) {
				s.skip()
			}
		}
		s.flowScalarFetched = false
		s.flowCollectionFetched = false
		s.startFlowCollectionFetched = false
		s.plainScalarFollowedByComment = false
		s.fetchPlainScalar()
		return
	}
	if s.simpleKeyAllowed && s.indent >= s.cLineOffset && s.isTab(0) {
		throwYAML(emptyMark, "While scanning a mapping, found invalid tab as indentation.")
	}
	if s.isWhiteBreakOrZero(0) {
		s.skip()
		return
	}
	start := s.cursorMark()
	s.skip()
	throwYAML(start, "While scanning for the next token, found character that cannot start any token.")
}

func (s *scanner) checkWhiteSpace() bool {
	return s.check(' ', 0) || ((s.flowLevel > 0 || !s.simpleKeyAllowed) && s.check('\t', 0))
}

func (s *scanner) scanToNextToken() {
	for {
		for s.checkWhiteSpace() {
			s.skip()
		}
		s.processComment()
		if s.isBreak(0) {
			s.skipLine()
			if s.flowLevel == 0 {
				s.simpleKeyAllowed = true
			}
		} else {
			break
		}
	}
}

// processComment skips a comment (the scanner skips comments).
func (s *scanner) processComment() {
	if s.check('#', 0) {
		s.skip()
		for s.isSpace(0) {
			s.skip()
		}
		for !s.isBreakOrZero(0) {
			s.skip()
		}
	}
}

func (s *scanner) fetchStreamStart() {
	s.simpleKeys = append(s.simpleKeys, &simpleKey{line: 1})
	s.simpleKeyAllowed = true
	s.streamStartProduced = true
	m := s.cursorMark()
	s.enqueue(&token{kind: tokStreamStart, start: m, end: m})
}

func (s *scanner) unrollIndent(column int64) {
	if s.flowLevel != 0 {
		return
	}
	for s.indent > column {
		m := s.cursorMark()
		s.enqueue(&token{kind: tokBlockEnd, start: m, end: m})
		s.indent = s.indents[len(s.indents)-1]
		s.indents = s.indents[:len(s.indents)-1]
	}
}

func (s *scanner) fetchStreamEnd() {
	if s.cLineOffset != 0 {
		s.cLine++
		s.cLineOffset = 0
	}
	s.unrollIndent(-1)
	s.removeSimpleKey()
	s.simpleKeyAllowed = false
	s.streamEndProduced = true
	m := s.cursorMark()
	s.enqueue(&token{kind: tokStreamEnd, start: m, end: m})
}

func (s *scanner) fetchDirective() {
	s.unrollIndent(-1)
	s.removeSimpleKey()
	s.simpleKeyAllowed = false
	if t := s.scanDirective(); t != nil {
		s.enqueue(t)
	}
}

func isKind(t *token, kinds ...tokenKind) bool {
	if t == nil {
		return false
	}
	for _, k := range kinds {
		if t.kind == k {
			return true
		}
	}
	return false
}

func (s *scanner) scanDirective() *token {
	start := s.cursorMark()
	s.skip()
	name := s.scanDirectiveName(start)
	var directive *token
	switch name {
	case "YAML":
		if isKind(s.previous, tokDocumentStart, tokStreamStart, tokDocumentEnd) {
			directive = s.scanVersionDirectiveValue(start)
		} else {
			throwYAML(start, "While scanning a version directive, did not find preceding <document end>.")
		}
	case "TAG":
		directive = s.scanTagDirectiveValue(start)
	default:
		for !s.endOfInput() && !s.check('#', 0) && !s.isBreak(0) {
			s.skip()
		}
		return nil
	}
	for s.isWhite(0) {
		s.skip()
	}
	s.processComment()
	if !s.isBreakOrZero(0) {
		throwYAML(start, "While scanning a directive, did not find expected comment or line break.")
	}
	if s.isBreak(0) {
		s.skipLine()
	}
	return directive
}

func (s *scanner) fetchDocumentIndicator(isStartToken bool) {
	s.unrollIndent(-1)
	s.removeSimpleKey()
	s.simpleKeyAllowed = false
	start := s.cursorMark()
	s.skip()
	s.skip()
	s.skip()
	if isStartToken {
		s.enqueue(&token{kind: tokDocumentStart, start: start, end: s.cursorMark()})
		return
	}
	var errorToken *token
	for !s.endOfInput() && !s.isBreak(0) && !s.check('#', 0) {
		if !s.isWhite(0) {
			errorToken = &token{kind: tokError, value: "While scanning a document end, found invalid content after '...' marker.", start: start, end: s.cursorMark()}
			break
		}
		s.skip()
	}
	s.enqueue(&token{kind: tokDocumentEnd, start: start, end: start})
	if errorToken != nil {
		s.enqueue(errorToken)
	}
}

func (s *scanner) fetchFlowCollectionStart(isSequence bool) {
	s.saveSimpleKey()
	s.simpleKeys = append(s.simpleKeys, &simpleKey{line: 1})
	s.flowLevel++
	s.simpleKeyAllowed = true
	start := s.cursorMark()
	s.skip()
	kind := tokFlowMappingStart
	if isSequence {
		kind = tokFlowSequenceStart
	}
	s.enqueue(&token{kind: kind, start: start, end: start})
	s.startFlowCollectionFetched = true
}

func (s *scanner) fetchFlowCollectionEnd(isSequence bool) {
	s.removeSimpleKey()
	if s.flowLevel > 0 {
		s.flowLevel--
		s.simpleKeys = s.simpleKeys[:len(s.simpleKeys)-1]
	}
	s.simpleKeyAllowed = false
	start := s.cursorMark()
	s.skip()
	var t, errorToken *token
	if isSequence {
		if s.check('#', 0) {
			errorToken = &token{kind: tokError, value: "While scanning a flow sequence end, found invalid comment after ']'.", start: start, end: start}
		}
		t = &token{kind: tokFlowSequenceEnd, start: start, end: start}
	} else {
		t = &token{kind: tokFlowMappingEnd, start: start, end: start}
	}
	s.enqueue(t)
	if errorToken != nil {
		s.enqueue(errorToken)
	}
	s.flowCollectionFetched = true
}

func (s *scanner) fetchFlowEntry() {
	s.removeSimpleKey()
	s.simpleKeyAllowed = true
	start := s.cursorMark()
	s.skip()
	end := s.cursorMark()
	if s.check('#', 0) {
		s.enqueueError("While scanning a flow entry, found invalid comment after comma.", start, end)
		return
	}
	s.enqueue(&token{kind: tokFlowEntry, start: start, end: end})
}

func (s *scanner) fetchBlockEntry() {
	if s.flowLevel == 0 {
		if !s.simpleKeyAllowed {
			if s.previousAnchor != nil && s.previousAnchor.end.line == s.cLine {
				throwYAML(s.previousAnchor.start, "Anchor before sequence entry on same line is not allowed.")
			}
			m := s.cursorMark()
			s.enqueueError("Block sequence entries are not allowed in this context.", m, m)
		}
		s.rollIndent(s.cLineOffset, -1, true, s.cursorMark())
	}
	s.removeSimpleKey()
	s.simpleKeyAllowed = true
	start := s.cursorMark()
	s.skip()
	s.enqueue(&token{kind: tokBlockEntry, start: start, end: s.cursorMark()})
}

func (s *scanner) fetchKey() {
	if s.flowLevel == 0 {
		if !s.simpleKeyAllowed {
			m := s.cursorMark()
			throwYAML(m, "Mapping keys are not allowed in this context.")
		}
		s.rollIndent(s.cLineOffset, -1, false, s.cursorMark())
	}
	s.removeSimpleKey()
	s.simpleKeyAllowed = s.flowLevel == 0
	start := s.cursorMark()
	s.skip()
	s.enqueue(&token{kind: tokKey, start: start, end: s.cursorMark()})
}

func (s *scanner) fetchValue() {
	key := s.topKey()
	if key.isPossible {
		m := key.mark()
		s.insert(key.tokenNumber-s.tokensParsed, &token{kind: tokKey, start: m, end: m})
		s.rollIndent(key.lineOffset, key.tokenNumber, false, m)
		key.isPossible = false
		s.simpleKeyAllowed = false
	} else {
		localSimpleKeyAllowed := s.flowLevel == 0
		if localSimpleKeyAllowed {
			if !s.simpleKeyAllowed {
				m := s.cursorMark()
				s.enqueueError("Mapping values are not allowed in this context.", m, m)
				return
			}
			s.rollIndent(s.cLineOffset, -1, false, s.cursorMark())
			if s.cLineOffset == 0 && key.lineOffset == 0 {
				m := key.mark()
				s.insert(len(s.tokens), &token{kind: tokKey, start: m, end: m})
				localSimpleKeyAllowed = false
			}
		}
		s.simpleKeyAllowed = localSimpleKeyAllowed
	}
	start := s.cursorMark()
	s.skip()
	s.enqueue(&token{kind: tokValue, start: start, end: s.cursorMark()})
}

func (s *scanner) rollIndent(column int64, number int, isSequence bool, position mark) {
	if s.flowLevel > 0 {
		return
	}
	if s.indent < column {
		s.indents = append(s.indents, s.indent)
		s.indent = column
		kind := tokBlockMappingStart
		if isSequence {
			kind = tokBlockSequenceStart
		}
		t := &token{kind: kind, start: position, end: position}
		if number == -1 {
			s.enqueue(t)
		} else {
			s.insert(number-s.tokensParsed, t)
		}
	}
}

func (s *scanner) fetchAnchor(isAlias bool) {
	s.saveSimpleKey()
	s.simpleKeyAllowed = false
	s.enqueue(s.scanAnchor(isAlias))
}

func (s *scanner) scanAnchor(isAlias bool) *token {
	start := s.cursorMark()
	s.skip()
	isAliasKey := false
	if isAlias {
		key := s.topKey()
		isAliasKey = key.isRequired && key.isPossible
	}
	var value []uint16
	for !s.isWhiteBreakOrZero(0) {
		if !s.checkAny("[]{},", 0) && !(isAliasKey && s.check(':', 0) && s.isWhiteBreakOrZero(1)) {
			value = append(value, s.readCurrentCharacter())
		} else {
			break
		}
	}
	if len(value) == 0 || !(s.isWhiteBreakOrZero(0) || s.checkAny("?:,]}%@`", 0)) {
		throwYAML(start, "While scanning an anchor or alias, found value containing disallowed: []{},")
	}
	t := &token{value: utf16String(value), start: start, end: s.cursorMark()}
	if isAlias {
		t.kind = tokAlias
		return t
	}
	t.kind = tokAnchor
	s.previousAnchor = t
	return t
}

func (s *scanner) fetchTag() {
	s.saveSimpleKey()
	s.simpleKeyAllowed = false
	s.enqueue(s.scanTag())
}

func (s *scanner) scanTag() *token {
	start := s.cursorMark()
	var handle, suffix string
	if s.check('<', 1) {
		handle = ""
		s.skip()
		s.skip()
		suffix = s.scanTagURI("", false, start)
		if !s.check('>', 0) {
			throwYAML(start, "While scanning a tag, did not find the expected '>'.")
		}
		s.skip()
	} else {
		firstPart := s.scanTagHandle(false, start)
		if len(firstPart) > 1 && firstPart[0] == '!' && firstPart[len(firstPart)-1] == '!' {
			handle = firstPart
			suffix = s.scanTagURI("", false, start)
		} else {
			suffix = s.scanTagURI(firstPart, true, start)
			handle = "!"
			if suffix == "" {
				suffix = handle
				handle = ""
			}
		}
	}
	if !s.isWhiteBreakOrZero(0) && !s.check(',', 0) {
		throwYAML(start, "While scanning a tag, did not find expected whitespace, comma or line break.")
	}
	return &token{kind: tokTag, handle: handle, value: suffix, start: start, end: s.cursorMark()}
}

func (s *scanner) fetchBlockScalar(isLiteral bool) {
	s.saveSimpleKey()
	s.simpleKeyAllowed = true
	s.enqueue(s.scanBlockScalar(isLiteral))
}

// boolPtr models C#'s bool? as 0 (null), 1 (true) and 2 (false).
type nullableBool int8

const (
	nbNull nullableBool = iota
	nbTrue
	nbFalse
)

func (s *scanner) scanBlockScalar(isLiteral bool) *token {
	var value, leadingBreak, trailingBreaks []uint16
	chomping := 0
	increment := int64(0)
	currentIndent := int64(0)
	leadingBlank := false
	isFirstLine := nbNull

	start := s.cursorMark()
	s.skip()
	if s.checkAny("+-", 0) {
		if s.check('+', 0) {
			chomping = 1
		} else {
			chomping = -1
		}
		s.skip()
		if s.isDigit(0) {
			if s.check('0', 0) {
				throwYAML(start, "While scanning a block scalar, found an indentation indicator equal to 0.")
			}
			increment = int64(s.asDigit(0))
			s.skip()
		}
	} else if s.isDigit(0) {
		if s.check('0', 0) {
			throwYAML(start, "While scanning a block scalar, found an indentation indicator equal to 0.")
		}
		increment = int64(s.asDigit(0))
		s.skip()
		if s.checkAny("+-", 0) {
			if s.check('+', 0) {
				chomping = 1
			} else {
				chomping = -1
			}
			s.skip()
		}
	}
	if s.check('#', 0) {
		throwYAML(start, "While scanning a block scalar, found a comment without whtespace after '>' indicator.")
	}
	for s.isWhite(0) {
		s.skip()
	}
	s.processComment()
	if !s.isBreakOrZero(0) {
		throwYAML(start, "While scanning a block scalar, did not find expected comment or line break.")
	}
	if s.isBreak(0) {
		s.skipLine()
		if isFirstLine == nbNull {
			isFirstLine = nbTrue
		} else if isFirstLine == nbTrue {
			isFirstLine = nbFalse
		}
	}
	end := s.cursorMark()
	if increment != 0 {
		if s.indent >= 0 {
			currentIndent = s.indent + increment
		} else {
			currentIndent = increment
		}
	}
	currentIndent = s.scanBlockScalarBreaks(currentIndent, &trailingBreaks, isLiteral, &end, &isFirstLine)
	isFirstLine = nbFalse

	for s.cLineOffset == currentIndent && !s.isZero(0) && !s.isDocumentEnd() {
		trailingBlank := s.isWhite(0)
		if !isLiteral && len(leadingBreak) > 0 && leadingBreak[0] == '\n' && !leadingBlank && !trailingBlank {
			if len(trailingBreaks) == 0 {
				value = append(value, ' ')
			}
			leadingBreak = leadingBreak[:0]
		} else {
			value = append(value, leadingBreak...)
			leadingBreak = leadingBreak[:0]
		}
		value = append(value, trailingBreaks...)
		trailingBreaks = trailingBreaks[:0]
		leadingBlank = s.isWhite(0)
		for !s.isBreakOrZero(0) {
			value = append(value, s.readCurrentCharacter())
		}
		if lineBreak := s.readLine(); lineBreak != 0 {
			leadingBreak = append(leadingBreak, lineBreak)
		}
		currentIndent = s.scanBlockScalarBreaks(currentIndent, &trailingBreaks, isLiteral, &end, &isFirstLine)
	}
	if chomping != -1 {
		value = append(value, leadingBreak...)
	}
	if chomping == 1 {
		value = append(value, trailingBreaks...)
	}
	style := styleFolded
	if isLiteral {
		style = styleLiteral
	}
	return &token{kind: tokScalar, value: utf16String(value), style: style, start: start, end: end}
}

func (s *scanner) scanBlockScalarBreaks(currentIndent int64, breaks *[]uint16, isLiteral bool, end *mark, isFirstLine *nullableBool) int64 {
	maxIndent := int64(0)
	indentOfFirstLine := int64(-1)
	*end = s.cursorMark()
	for {
		for (currentIndent == 0 || s.cLineOffset < currentIndent) && s.isSpace(0) {
			s.skip()
		}
		if s.cLineOffset > maxIndent {
			maxIndent = s.cLineOffset
		}
		if !s.isBreak(0) {
			if isLiteral && *isFirstLine == nbTrue {
				localIndent := s.cLineOffset
				i := 0
				for !s.isBreak(i) && s.isSpace(i) {
					i++
					localIndent++
				}
				if s.isBreak(i) && localIndent > s.cLineOffset {
					*isFirstLine = nbFalse
					indentOfFirstLine = localIndent
				}
			}
			break
		}
		if *isFirstLine == nbTrue {
			*isFirstLine = nbFalse
			indentOfFirstLine = s.cLineOffset
		}
		*breaks = append(*breaks, s.readLine())
		*end = s.cursorMark()
	}
	if isLiteral && indentOfFirstLine > 1 && currentIndent < indentOfFirstLine-1 {
		throwYAML(*end, "While scanning a literal block scalar, found extra spaces in first line.")
	}
	if !isLiteral && maxIndent > s.cLineOffset && indentOfFirstLine > -1 {
		throwYAML(*end, "While scanning a literal block scalar, found more spaces in lines above first content line.")
	}
	if currentIndent == 0 && (s.cLineOffset > 0 || s.indent > -1) {
		currentIndent = max(maxIndent, max(s.indent+1, 1))
	}
	return currentIndent
}

func (s *scanner) fetchQuotedScalar(isSingleQuoted bool) {
	s.saveSimpleKey()
	s.simpleKeyAllowed = false
	s.flowScalarFetched = s.flowLevel > 0
	scalar := s.scanFlowScalar(isSingleQuoted)
	s.enqueue(scalar)
	s.lastScalar = scalar
	if !isSingleQuoted && s.check('#', 0) {
		m := s.cursorMark()
		s.enqueueError("While scanning a flow sequence end, found invalid comment after double-quoted scalar.", m, m)
	}
}

func (s *scanner) scanFlowScalar(isSingleQuoted bool) *token {
	start := s.cursorMark()
	s.skip()
	var value, whitespaces, leadingBreak, trailingBreaks []uint16
	hasLeadingBlanks := false
	quote := uint16('"')
	if isSingleQuoted {
		quote = '\''
	}
	for {
		if s.isDocumentIndicator() {
			throwYAML(start, "While scanning a quoted scalar, found unexpected document indicator.")
		}
		if s.isZero(0) {
			throwYAML(start, "While scanning a quoted scalar, found unexpected end of stream.")
		}
		if hasLeadingBlanks && !isSingleQuoted && s.indent >= s.cLineOffset {
			throwYAML(start, "While scanning a multi-line double-quoted scalar, found wrong indentation.")
		}
		hasLeadingBlanks = false
		for !s.isWhiteBreakOrZero(0) {
			if isSingleQuoted && s.check('\'', 0) && s.check('\'', 1) {
				value = append(value, '\'')
				s.skip()
				s.skip()
			} else if s.check(quote, 0) {
				break
			} else if !isSingleQuoted && s.check('\\', 0) && s.isBreak(1) {
				s.skip()
				s.skipLine()
				hasLeadingBlanks = true
				break
			} else if !isSingleQuoted && s.check('\\', 0) {
				codeLength := 0
				escapeCharacter := s.peek(1)
				switch escapeCharacter {
				case 'x':
					codeLength = 2
				case 'u':
					codeLength = 4
				case 'U':
					codeLength = 8
				default:
					if u, ok := simpleEscapeCodes[escapeCharacter]; ok {
						value = append(value, u)
					} else {
						throwYAML(start, "While scanning a quoted scalar, found unknown escape character.")
					}
				}
				s.skip()
				s.skip()
				if codeLength > 0 {
					character := int32(0)
					for k := 0; k < codeLength; k++ {
						if !s.isHex(k) {
							throwYAML(start, "While scanning a quoted scalar, did not find expected hexadecimal number.")
						}
						character = (character << 4) + s.asHex(k)
					}
					if character >= 0xD800 && character <= 0xDFFF {
						for k := 0; k < codeLength; k++ {
							s.skip()
						}
						if s.peek(0) == '\\' && (s.peek(1) == 'u' || s.peek(1) == 'U') {
							s.skip()
							if s.peek(0) == 'u' {
								codeLength = 4
							} else {
								codeLength = 8
							}
							s.skip()
							lowSurrogate := int32(0)
							for k := 0; k < codeLength; k++ {
								if !s.isHex(0) {
									throwYAML(start, "While scanning a quoted scalar, did not find expected hexadecimal number.")
								}
								lowSurrogate = (lowSurrogate << 4) + s.asHex(k)
							}
							for k := 0; k < codeLength; k++ {
								s.skip()
							}
							// char.ConvertToUtf32((char)character, (char)lowSurrogate)
							hi, lo := uint16(character), uint16(lowSurrogate)
							if hi < 0xD800 || hi > 0xDBFF || lo < 0xDC00 || lo > 0xDFFF {
								throwCrash("ConvertToUtf32: not a valid surrogate pair")
							}
							character = int32(utf16.DecodeRune(rune(hi), rune(lo)))
						} else {
							throwYAML(start, "While scanning a quoted scalar, found invalid Unicode surrogates.")
						}
					} else if character > 0x10FFFF {
						throwYAML(start, "While scanning a quoted scalar, found invalid Unicode character escape code.")
					} else {
						for k := 0; k < codeLength; k++ {
							s.skip()
						}
					}
					// char.ConvertFromUtf32(character)
					if character < 0 || character > 0x10FFFF || (character >= 0xD800 && character <= 0xDFFF) {
						throwCrash("ConvertFromUtf32: not a valid code point")
					}
					value = utf16.AppendRune(value, rune(character))
				}
			} else {
				value = append(value, s.readCurrentCharacter())
			}
		}
		if s.check(quote, 0) {
			break
		}
		for s.isWhite(0) || s.isBreak(0) {
			if s.isWhite(0) {
				if !hasLeadingBlanks {
					whitespaces = append(whitespaces, s.readCurrentCharacter())
				} else {
					s.skip()
				}
			} else {
				if !hasLeadingBlanks {
					whitespaces = whitespaces[:0]
					leadingBreak = append(leadingBreak, s.readLine())
					hasLeadingBlanks = true
				} else {
					trailingBreaks = append(trailingBreaks, s.readLine())
				}
			}
		}
		if hasLeadingBlanks {
			if len(leadingBreak) > 0 && leadingBreak[0] == '\n' {
				if len(trailingBreaks) == 0 {
					value = append(value, ' ')
				} else {
					value = append(value, trailingBreaks...)
				}
			} else {
				value = append(value, leadingBreak...)
				value = append(value, trailingBreaks...)
			}
			leadingBreak = leadingBreak[:0]
			trailingBreaks = trailingBreaks[:0]
		} else {
			value = append(value, whitespaces...)
			whitespaces = whitespaces[:0]
		}
	}
	s.skip()
	style := styleDoubleQuoted
	if isSingleQuoted {
		style = styleSingleQuoted
	}
	return &token{kind: tokScalar, value: utf16String(value), style: style, start: start, end: s.cursorMark()}
}

func (s *scanner) fetchPlainScalar() {
	s.saveSimpleKey()
	s.simpleKeyAllowed = false
	isMultiline := false
	scalar := s.scanPlainScalar(&isMultiline)
	s.lastScalar = scalar
	if isMultiline && s.check(':', 0) && s.flowLevel == 0 && s.indent < s.cLineOffset {
		m := s.cursorMark()
		s.enqueueError("While scanning a multiline plain scalar, found invalid mapping.", m, m)
	}
	s.enqueue(scalar)
}

func (s *scanner) scanPlainScalar(isMultiline *bool) *token {
	var value, whitespaces, leadingBreak, trailingBreaks []uint16
	hasLeadingBlanks := false
	currentIndent := s.indent + 1
	start := s.cursorMark()
	end := start
	key := s.topKey()
	for {
		if s.isDocumentIndicator() {
			break
		}
		if s.check('#', 0) {
			if s.indent < 0 && s.flowLevel == 0 {
				s.plainScalarFollowedByComment = true
			}
			break
		}
		isAliasValue := s.check('*', 0) && !(key.isPossible && key.isRequired)
		for !s.isWhiteBreakOrZero(0) {
			if s.check(':', 0) && !isAliasValue && (s.isWhiteBreakOrZero(1) || (s.flowLevel > 0 && s.check(',', 1))) ||
				(s.flowLevel > 0 && s.checkAny(",[]{}", 0)) {
				if s.flowLevel == 0 && !key.isPossible {
					m := s.cursorMark()
					s.enqueueError("While scanning a plain scalar value, found invalid mapping.", m, m)
				}
				break
			}
			if hasLeadingBlanks || len(whitespaces) > 0 {
				if hasLeadingBlanks {
					if len(leadingBreak) > 0 && leadingBreak[0] == '\n' {
						if len(trailingBreaks) == 0 {
							value = append(value, ' ')
						} else {
							value = append(value, trailingBreaks...)
						}
					} else {
						value = append(value, leadingBreak...)
						value = append(value, trailingBreaks...)
					}
					leadingBreak = leadingBreak[:0]
					trailingBreaks = trailingBreaks[:0]
					hasLeadingBlanks = false
				} else {
					value = append(value, whitespaces...)
					whitespaces = whitespaces[:0]
				}
			}
			if s.flowLevel > 0 && s.cLineOffset < currentIndent {
				throwCrash("Operation is not valid due to the current state of the object.")
			}
			value = append(value, s.readCurrentCharacter())
			end = s.cursorMark()
		}
		if !(s.isWhite(0) || s.isBreak(0)) {
			break
		}
		for s.isWhite(0) || s.isBreak(0) {
			if s.isWhite(0) {
				if hasLeadingBlanks && s.cLineOffset < currentIndent && s.isTab(0) {
					throwYAML(start, "While scanning a plain scalar, found a tab character that violate indentation.")
				}
				if !hasLeadingBlanks {
					whitespaces = append(whitespaces, s.readCurrentCharacter())
				} else {
					s.skip()
				}
			} else {
				*isMultiline = true
				if !hasLeadingBlanks {
					whitespaces = whitespaces[:0]
					leadingBreak = append(leadingBreak, s.readLine())
					hasLeadingBlanks = true
				} else {
					trailingBreaks = append(trailingBreaks, s.readLine())
				}
			}
		}
		if s.flowLevel == 0 && s.cLineOffset < currentIndent {
			break
		}
	}
	if hasLeadingBlanks {
		s.simpleKeyAllowed = true
	}
	return &token{kind: tokScalar, value: utf16String(value), style: stylePlain, start: start, end: end}
}

func (s *scanner) removeSimpleKey() {
	key := s.topKey()
	if key.isPossible && key.isRequired {
		throwYAML(key.mark(), "While scanning a simple key, could not find expected ':'.")
	}
	key.isPossible = false
}

func (s *scanner) scanDirectiveName(start mark) string {
	var name []uint16
	for s.isAlphaNumericDashOrUnderscore() {
		name = append(name, s.readCurrentCharacter())
	}
	if len(name) == 0 {
		throwYAML(start, "While scanning a directive, could not find expected directive name.")
	}
	if s.endOfInput() {
		throwYAML(start, "While scanning a directive, found unexpected end of stream.")
	}
	if !s.isWhiteBreakOrZero(0) {
		throwYAML(start, "While scanning a directive, found unexpected non-alphabetical character.")
	}
	return utf16String(name)
}

func (s *scanner) skipWhitespaces() {
	for s.isWhite(0) {
		s.skip()
	}
}

func (s *scanner) scanVersionDirectiveValue(start mark) *token {
	s.skipWhitespaces()
	major := s.scanVersionDirectiveNumber(start)
	if !s.check('.', 0) {
		throwYAML(start, "While scanning a %YAML directive, did not find expected digit or '.' character.")
	}
	s.skip()
	minor := s.scanVersionDirectiveNumber(start)
	return &token{kind: tokVersionDirective, major: major, minor: minor, start: start, end: start}
}

func (s *scanner) scanTagDirectiveValue(start mark) *token {
	s.skipWhitespaces()
	handle := s.scanTagHandle(true, start)
	if !s.isWhite(0) {
		throwYAML(start, "While scanning a %TAG directive, did not find expected whitespace.")
	}
	s.skipWhitespaces()
	prefix := s.scanTagURI("", false, start)
	if !s.isWhiteBreakOrZero(0) {
		throwYAML(start, "While scanning a %TAG directive, did not find expected whitespace or line break.")
	}
	// TagDirective's constructor.
	if prefix == "" {
		throwCrash("Tag prefix must not be empty.")
	}
	return &token{kind: tokTagDirective, handle: handle, value: prefix, start: start, end: start}
}

// scanTagURI scans a tag URI; hasHead says head (the part scanned as a
// handle) is given, as C#'s non-null head.
func (s *scanner) scanTagURI(head string, hasHead bool, start mark) string {
	var tag []uint16
	if hasHead && len(head) > 1 {
		tag = append(tag, utf16.Encode([]rune(head[1:]))...)
	}
	for s.isAlphaNumericDashOrUnderscore() || s.checkAny(";/?:@&=+$.!~*'()[]%", 0) ||
		(s.check(',', 0) && !s.isBreak(1)) {
		if s.check('%', 0) {
			tag = append(tag, s.scanURIEscapes(start)...)
		} else if s.check('+', 0) {
			tag = append(tag, ' ')
			s.skip()
		} else {
			tag = append(tag, s.readCurrentCharacter())
		}
	}
	if len(tag) == 0 {
		return ""
	}
	if tag[len(tag)-1] == ',' {
		m := s.cursorMark()
		throwYAML(m, "Unexpected comma at end of tag")
	}
	return utf16String(tag)
}

func (s *scanner) scanURIEscapes(start mark) []uint16 {
	var charBytes []byte
	width := 0
	for {
		if !(s.check('%', 0) && s.isHex(1) && s.isHex(2)) {
			throwYAML(start, "While scanning a tag, did not find URI escaped octet.")
		}
		octet := (s.asHex(1) << 4) + s.asHex(2)
		if width == 0 {
			switch {
			case octet&0x80 == 0x00:
				width = 1
			case octet&0xE0 == 0xC0:
				width = 2
			case octet&0xF0 == 0xE0:
				width = 3
			case octet&0xF8 == 0xF0:
				width = 4
			}
			if width == 0 {
				throwYAML(start, "While scanning a tag, found an incorrect leading UTF-8 octet.")
			}
		} else if octet&0xC0 != 0x80 {
			throwYAML(start, "While scanning a tag, found an incorrect trailing UTF-8 octet.")
		}
		charBytes = append(charBytes, byte(octet))
		s.skip()
		s.skip()
		s.skip()
		width--
		if width <= 0 {
			break
		}
	}
	result := decodeUTF8Replacing(charBytes)
	if len(result) == 0 || len(result) > 2 {
		throwYAML(start, "While scanning a tag, found an incorrect UTF-8 sequence.")
	}
	return result
}

func (s *scanner) scanTagHandle(isDirective bool, start mark) string {
	if !s.check('!', 0) {
		throwYAML(start, "While scanning a tag, did not find expected '!'.")
	}
	var tagHandle []uint16
	tagHandle = append(tagHandle, s.readCurrentCharacter())
	for s.isAlphaNumericDashOrUnderscore() {
		tagHandle = append(tagHandle, s.readCurrentCharacter())
	}
	if s.check('!', 0) {
		tagHandle = append(tagHandle, s.readCurrentCharacter())
	} else if isDirective && (len(tagHandle) != 1 || tagHandle[0] != '!') {
		throwYAML(start, "While scanning a tag directive, did not find expected '!'.")
	}
	return utf16String(tagHandle)
}

func (s *scanner) scanVersionDirectiveNumber(start mark) int {
	value, length := 0, 0
	for s.isDigit(0) {
		length++
		if length > maxVersionNumberLength {
			throwYAML(start, "While scanning a %YAML directive, found extremely long version number.")
		}
		value = value*10 + s.asDigit(0)
		s.skip()
	}
	if length == 0 {
		throwYAML(start, "While scanning a %YAML directive, did not find expected version number.")
	}
	return value
}

func (s *scanner) saveSimpleKey() {
	isRequired := s.flowLevel == 0 && s.indent == s.cLineOffset
	if s.simpleKeyAllowed {
		key := &simpleKey{
			isPossible:  true,
			isRequired:  isRequired,
			tokenNumber: s.tokensParsed + len(s.tokens),
			index:       s.cIndex,
			line:        s.cLine,
			lineOffset:  s.cLineOffset,
		}
		s.removeSimpleKey()
		s.simpleKeys[len(s.simpleKeys)-1] = key
	}
}

// utf16String converts UTF-16 code units to a Go string; an unpaired
// surrogate (which the reader cannot produce) becomes U+FFFD.
func utf16String(u []uint16) string {
	return string(utf16.Decode(u))
}

// decodeUTF8Replacing decodes bytes as .NET's Encoding.UTF8 (replacement
// fallback) does, into UTF-16: each maximal invalid subsequence becomes one
// U+FFFD.
func decodeUTF8Replacing(b []byte) []uint16 {
	var out []uint16
	for len(b) > 0 {
		r, n := utf8.DecodeRune(b)
		if r == utf8.RuneError && n <= 1 {
			n = invalidUTF8Length(b)
		}
		out = utf16.AppendRune(out, r)
		b = b[n:]
	}
	return out
}

// invalidUTF8Length is the length of the maximal subpart of an ill-formed
// UTF-8 sequence at the start of b (Unicode 3.9, "U+FFFD substitution of
// maximal subparts"), at least 1.
func invalidUTF8Length(b []byte) int {
	c := b[0]
	var need int
	lo, hi := byte(0x80), byte(0xBF)
	switch {
	case c >= 0xC2 && c <= 0xDF:
		need = 1
	case c == 0xE0:
		need, lo = 2, 0xA0
	case c >= 0xE1 && c <= 0xEC, c == 0xEE, c == 0xEF:
		need = 2
	case c == 0xED:
		need, hi = 2, 0x9F
	case c == 0xF0:
		need, lo = 3, 0x90
	case c >= 0xF1 && c <= 0xF3:
		need = 3
	case c == 0xF4:
		need, hi = 3, 0x8F
	default:
		return 1
	}
	n := 1
	for i := 0; i < need && n < len(b); i++ {
		if b[n] < lo || b[n] > hi {
			break
		}
		lo, hi = 0x80, 0xBF
		n++
	}
	return n
}
