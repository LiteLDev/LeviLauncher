package doc

// This file ports YamlDotNet 16.3.0's Parser (YamlDotNet/Core/Parser.cs),
// which turns the scanner's tokens into events, together with the parts of
// ParserExtensions.cs, TagName.cs and Constants.cs that YamlJson relies on.
//
// Copyright (c) Antoine Aubry and contributors (YamlDotNet, MIT license).

type eventKind int

const (
	evStreamStart eventKind = iota
	evStreamEnd
	evDocumentStart
	evDocumentEnd
	evAlias
	evScalar
	evSequenceStart
	evSequenceEnd
	evMappingStart
	evMappingEnd
)

// event is a ParsingEvent. anchor and tag are "" when empty (YamlDotNet
// never makes an empty anchor or tag name).
type event struct {
	kind       eventKind
	start, end mark
	anchor     string
	tag        string
	value      string
	style      scalarStyle
}

type parserState int

const (
	psStreamStart parserState = iota
	psStreamEnd
	psImplicitDocumentStart
	psDocumentStart
	psDocumentContent
	psDocumentEnd
	psBlockNode
	psBlockNodeOrIndentlessSequence
	psFlowNode
	psBlockSequenceFirstEntry
	psBlockSequenceEntry
	psIndentlessSequenceEntry
	psBlockMappingFirstKey
	psBlockMappingKey
	psBlockMappingValue
	psFlowSequenceFirstEntry
	psFlowSequenceEntry
	psFlowSequenceEntryMappingKey
	psFlowSequenceEntryMappingValue
	psFlowSequenceEntryMappingEnd
	psFlowMappingFirstKey
	psFlowMappingKey
	psFlowMappingValue
	psFlowMappingEmptyValue
)

type tagDirective struct{ handle, prefix string }

// defaultTagDirectives is Constants.DefaultTagDirectives.
var defaultTagDirectives = []tagDirective{{"!", "!"}, {"!!", "tag:yaml.org,2002:"}}

// tagDirectives is a TagDirectiveCollection: keyed by handle, in order.
type tagDirectives []tagDirective

func (c tagDirectives) find(handle string) (tagDirective, bool) {
	for _, d := range c {
		if d.handle == handle {
			return d, true
		}
	}
	return tagDirective{}, false
}

func (c *tagDirectives) addAll(src []tagDirective) {
	for _, d := range src {
		if _, ok := c.find(d.handle); !ok {
			*c = append(*c, d)
		}
	}
}

type parser struct {
	scanner      *scanner
	states       []parserState
	tagDirs      tagDirectives
	state        parserState
	currentToken *token
	hasVersion   bool
	version      [2]int
	pending      []*event

	// Current is the current event; nil before the first MoveNext and after
	// the end.
	current *event
}

func newParser(text []uint16) *parser {
	return &parser{scanner: newScanner(text)}
}

func (p *parser) getCurrentToken() *token {
	if p.currentToken == nil {
		// The scanner skips comments, so the first token is the one.
		if p.scanner.moveNextWithoutConsuming() {
			p.currentToken = p.scanner.current
		}
	}
	return p.currentToken
}

func (p *parser) moveNext() bool {
	if p.state == psStreamEnd {
		p.current = nil
		return false
	} else if len(p.pending) == 0 {
		p.pending = append(p.pending, p.stateMachine())
	}
	p.current = p.pending[0]
	p.pending = p.pending[1:]
	return true
}

func (p *parser) popState() parserState {
	if len(p.states) == 0 {
		throwCrash("Stack empty.") // Stack<T>.Pop's InvalidOperationException
	}
	s := p.states[len(p.states)-1]
	p.states = p.states[:len(p.states)-1]
	return s
}

func (p *parser) pushState(s parserState) { p.states = append(p.states, s) }

func (p *parser) stateMachine() *event {
	switch p.state {
	case psStreamStart:
		return p.parseStreamStart()
	case psImplicitDocumentStart:
		return p.parseDocumentStart(true)
	case psDocumentStart:
		return p.parseDocumentStart(false)
	case psDocumentContent:
		return p.parseDocumentContent()
	case psDocumentEnd:
		return p.parseDocumentEnd()
	case psBlockNode:
		return p.parseNode(true, false)
	case psBlockNodeOrIndentlessSequence:
		return p.parseNode(true, true)
	case psFlowNode:
		return p.parseNode(false, false)
	case psBlockSequenceFirstEntry:
		return p.parseBlockSequenceEntry(true)
	case psBlockSequenceEntry:
		return p.parseBlockSequenceEntry(false)
	case psIndentlessSequenceEntry:
		return p.parseIndentlessSequenceEntry()
	case psBlockMappingFirstKey:
		return p.parseBlockMappingKey(true)
	case psBlockMappingKey:
		return p.parseBlockMappingKey(false)
	case psBlockMappingValue:
		return p.parseBlockMappingValue()
	case psFlowSequenceFirstEntry:
		return p.parseFlowSequenceEntry(true)
	case psFlowSequenceEntry:
		return p.parseFlowSequenceEntry(false)
	case psFlowSequenceEntryMappingKey:
		return p.parseFlowSequenceEntryMappingKey()
	case psFlowSequenceEntryMappingValue:
		return p.parseFlowSequenceEntryMappingValue()
	case psFlowSequenceEntryMappingEnd:
		return p.parseFlowSequenceEntryMappingEnd()
	case psFlowMappingFirstKey:
		return p.parseFlowMappingKey(true)
	case psFlowMappingKey:
		return p.parseFlowMappingKey(false)
	case psFlowMappingValue:
		return p.parseFlowMappingValue(false)
	case psFlowMappingEmptyValue:
		return p.parseFlowMappingValue(true)
	}
	throwCrash("Invalid state")
	return nil
}

func (p *parser) skip() {
	if p.currentToken != nil {
		p.currentToken = nil
		p.scanner.consumeCurrent()
	}
}

func tokStart(t *token) mark {
	if t == nil {
		return emptyMark
	}
	return t.start
}

func tokEnd(t *token) mark {
	if t == nil {
		return emptyMark
	}
	return t.end
}

func (p *parser) parseStreamStart() *event {
	current := p.getCurrentToken()
	if !isKind(current, tokStreamStart) {
		throwYAML(tokStart(current), "Did not find expected <stream-start>.")
	}
	p.skip()
	p.state = psImplicitDocumentStart
	return &event{kind: evStreamStart, start: current.start, end: current.end}
}

func (p *parser) parseDocumentStart(isImplicit bool) *event {
	if isKind(p.currentToken, tokVersionDirective) {
		throwYAML(emptyMark, "While parsing a document start node, could not find document end marker before version directive.")
	}
	current := p.getCurrentToken()
	if !isImplicit {
		for isKind(current, tokDocumentEnd) {
			p.skip()
			current = p.getCurrentToken()
		}
	}
	if current == nil {
		throwYAML(emptyMark, "Reached the end of the stream while parsing a document start.")
	}
	if current.kind == tokScalar && (p.state == psImplicitDocumentStart || p.state == psDocumentStart) {
		isImplicit = true
	}
	if isImplicit && !isKind(current, tokVersionDirective, tokTagDirective, tokDocumentStart, tokStreamEnd, tokDocumentEnd) || current.kind == tokBlockMappingStart {
		p.processDirectives()
		p.pushState(psDocumentEnd)
		p.state = psBlockNode
		return &event{kind: evDocumentStart, start: current.start, end: current.end}
	} else if !isKind(current, tokStreamEnd, tokDocumentEnd) {
		start := current.start
		p.processDirectives()
		current = p.getCurrentToken()
		if current == nil {
			throwYAML(emptyMark, "Reached the end of the stream while parsing a document start")
		}
		if current.kind != tokDocumentStart {
			throwYAML(current.start, "Did not find expected <document start>.")
		}
		p.pushState(psDocumentEnd)
		p.state = psDocumentContent
		end := current.end
		p.skip()
		return &event{kind: evDocumentStart, start: start, end: end}
	}
	if current.kind == tokDocumentEnd {
		p.skip()
	}
	p.state = psStreamEnd
	current = p.getCurrentToken()
	if current == nil {
		throwYAML(emptyMark, "Reached the end of the stream while parsing a document start")
	}
	evt := &event{kind: evStreamEnd, start: current.start, end: current.end}
	if p.scanner.moveNextWithoutConsuming() {
		throwCrash("The scanner should contain no more tokens.")
	}
	return evt
}

func (p *parser) processDirectives() {
	hasOwnDirectives := false
	var tags tagDirectives
	for {
		t := p.getCurrentToken()
		if isKind(t, tokVersionDirective) {
			if p.hasVersion {
				throwYAML(t.start, "Found duplicate %YAML directive.")
			}
			if t.major != 1 || t.minor > 3 {
				throwYAML(t.start, "Found incompatible YAML document.")
			}
			p.hasVersion = true
			p.version = [2]int{t.major, t.minor}
			hasOwnDirectives = true
		} else if isKind(t, tokTagDirective) {
			if _, ok := tags.find(t.handle); ok {
				throwYAML(t.start, "Found duplicate %TAG directive.")
			}
			// TagDirective's constructor checks the handle.
			if !validTagHandle(t.handle) {
				throwCrash("Tag handle must start and end with '!' and contain alphanumerical characters only.")
			}
			tags = append(tags, tagDirective{t.handle, t.value})
			hasOwnDirectives = true
		} else if isKind(t, tokDocumentStart) && (!p.hasVersion || (p.version[0] == 1 && p.version[1] > 1)) {
			if !p.hasVersion {
				p.hasVersion = true
				p.version = [2]int{1, 2}
			}
			hasOwnDirectives = true
			break
		} else {
			break
		}
		p.skip()
	}
	tags.addAll(defaultTagDirectives)
	if hasOwnDirectives {
		p.tagDirs = nil
	}
	p.tagDirs.addAll(tags)
}

// validTagHandle is TagDirective's ^!([0-9A-Za-z_\-]*!)?$.
func validTagHandle(h string) bool {
	if h == "" || h[0] != '!' {
		return false
	}
	if len(h) == 1 {
		return true
	}
	if h[len(h)-1] != '!' {
		return false
	}
	for i := 1; i < len(h)-1; i++ {
		if !isAlphaNumericDashOrUnderscore(uint16(h[i])) {
			return false
		}
	}
	return true
}

func (p *parser) parseDocumentContent() *event {
	if isKind(p.getCurrentToken(), tokVersionDirective, tokTagDirective, tokDocumentStart, tokDocumentEnd, tokStreamEnd) {
		p.state = p.popState()
		return emptyScalar(p.scanner.cursorMark())
	}
	return p.parseNode(true, false)
}

func emptyScalar(position mark) *event {
	return &event{kind: evScalar, style: stylePlain, start: position, end: position}
}

func (p *parser) parseNode(isBlock, isIndentlessSequence bool) *event {
	if t := p.getCurrentToken(); isKind(t, tokError) {
		throwYAML(t.start, t.value)
	}
	current := p.getCurrentToken()
	if current == nil {
		throwYAML(emptyMark, "Reached the end of the stream while parsing a node")
	}
	if current.kind == tokAlias {
		p.state = p.popState()
		evt := &event{kind: evAlias, anchor: current.value, start: current.start, end: current.end}
		p.skip()
		return evt
	}
	start := current.start
	anchorName, tagName := "", ""
	var lastAnchor, lastTag *token
	for {
		if anchorName == "" && current.kind == tokAnchor {
			lastAnchor = current
			anchorName = current.value
			p.skip()
		} else if tagName == "" && current.kind == tokTag {
			lastTag = current
			if current.handle == "" {
				tagName = newTagName(current.value)
			} else if d, ok := p.tagDirs.find(current.handle); ok {
				tagName = newTagName(d.prefix + current.value)
			} else {
				throwYAML(current.start, "While parsing a node, found undefined tag handle.")
			}
			p.skip()
		} else if current.kind == tokAnchor {
			throwYAML(current.start, "While parsing a node, found more than one anchor.")
		} else if current.kind == tokAlias {
			throwYAML(current.start, "While parsing a node, did not find expected token.")
		} else if current.kind == tokError {
			if lastTag != nil && lastAnchor != nil && anchorName != "" {
				return &event{kind: evScalar, anchor: anchorName, style: styleAny, start: lastAnchor.start, end: lastAnchor.end}
			}
			throwYAML(current.start, current.value)
		} else {
			break
		}
		current = p.getCurrentToken()
		if current == nil {
			throwYAML(emptyMark, "Reached the end of the stream while parsing a node")
		}
	}
	if isIndentlessSequence && isKind(p.getCurrentToken(), tokBlockEntry) {
		p.state = psIndentlessSequenceEntry
		return &event{kind: evSequenceStart, anchor: anchorName, tag: tagName, start: start, end: current.end}
	}
	if current.kind == tokScalar {
		p.state = p.popState()
		p.skip()
		evt := &event{kind: evScalar, anchor: anchorName, tag: tagName, value: current.value, style: current.style, start: start, end: current.end}
		if anchorName != "" && p.scanner.moveNextWithoutConsuming() {
			p.currentToken = p.scanner.current
			if isKind(p.currentToken, tokError) {
				throwYAML(p.currentToken.start, p.currentToken.value)
			}
		}
		if p.state == psFlowMappingKey && !isKind(p.scanner.current, tokFlowMappingEnd) && p.scanner.moveNextWithoutConsuming() {
			p.currentToken = p.scanner.current
			if p.currentToken != nil && !isKind(p.currentToken, tokFlowEntry, tokFlowMappingEnd) {
				throwYAML(p.currentToken.start, "While parsing a flow mapping, did not find expected ',' or '}'.")
			}
		}
		return evt
	}
	if current.kind == tokFlowSequenceStart {
		p.state = psFlowSequenceFirstEntry
		return &event{kind: evSequenceStart, anchor: anchorName, tag: tagName, start: start, end: current.end}
	}
	if current.kind == tokFlowMappingStart {
		p.state = psFlowMappingFirstKey
		return &event{kind: evMappingStart, anchor: anchorName, tag: tagName, start: start, end: current.end}
	}
	if isBlock {
		if current.kind == tokBlockSequenceStart {
			p.state = psBlockSequenceFirstEntry
			return &event{kind: evSequenceStart, anchor: anchorName, tag: tagName, start: start, end: current.end}
		}
		if current.kind == tokBlockMappingStart {
			p.state = psBlockMappingFirstKey
			return &event{kind: evMappingStart, anchor: anchorName, tag: tagName, start: start, end: current.end}
		}
	}
	if anchorName != "" || tagName != "" {
		p.state = p.popState()
		return &event{kind: evScalar, anchor: anchorName, tag: tagName, style: stylePlain, start: start, end: current.end}
	}
	throwYAML(current.start, "While parsing a node, did not find expected node content.")
	return nil
}

// newTagName is TagName's constructor: an empty tag is an ArgumentException.
// (A global tag must also be a well-formed URI; YamlJson rejects every tag
// that is not "!" or a core tag anyway, so that check is not repeated.)
func newTagName(v string) string {
	if v == "" {
		throwCrash("Tag value must not be empty.")
	}
	return v
}

func (p *parser) parseDocumentEnd() *event {
	current := p.getCurrentToken()
	if current == nil {
		throwYAML(emptyMark, "Reached the end of the stream while parsing a document end")
	}
	start := current.start
	end := start
	if current.kind == tokDocumentEnd {
		end = current.end
		p.skip()
	} else if !(isKind(p.currentToken, tokStreamEnd, tokDocumentStart, tokFlowSequenceEnd, tokVersionDirective) ||
		(p.current != nil && p.current.kind == evScalar && isKind(p.currentToken, tokError))) {
		throwYAML(start, "Did not find expected <document end>.")
	}
	if p.hasVersion && p.version[0] == 1 && p.version[1] > 1 {
		p.hasVersion = false
	}
	p.state = psDocumentStart
	return &event{kind: evDocumentEnd, start: start, end: end}
}

func (p *parser) parseBlockSequenceEntry(isFirst bool) *event {
	if isFirst {
		p.getCurrentToken()
		p.skip()
	}
	current := p.getCurrentToken()
	if isKind(current, tokBlockEntry) {
		m := current.end
		p.skip()
		current = p.getCurrentToken()
		if !isKind(current, tokBlockEntry, tokBlockEnd) {
			p.pushState(psBlockSequenceEntry)
			return p.parseNode(true, false)
		}
		p.state = psBlockSequenceEntry
		return emptyScalar(m)
	} else if isKind(current, tokBlockEnd) {
		p.state = p.popState()
		evt := &event{kind: evSequenceEnd, start: current.start, end: current.end}
		p.skip()
		return evt
	}
	throwYAML(tokStart(current), "While parsing a block collection, did not find expected '-' indicator.")
	return nil
}

func (p *parser) parseIndentlessSequenceEntry() *event {
	current := p.getCurrentToken()
	if isKind(current, tokBlockEntry) {
		m := current.end
		p.skip()
		current = p.getCurrentToken()
		if !isKind(current, tokBlockEntry, tokKey, tokValue, tokBlockEnd) {
			p.pushState(psIndentlessSequenceEntry)
			return p.parseNode(true, false)
		}
		p.state = psIndentlessSequenceEntry
		return emptyScalar(m)
	}
	p.state = p.popState()
	return &event{kind: evSequenceEnd, start: tokStart(current), end: tokEnd(current)}
}

func (p *parser) parseBlockMappingKey(isFirst bool) *event {
	if isFirst {
		p.getCurrentToken()
		p.skip()
	}
	current := p.getCurrentToken()
	switch {
	case isKind(current, tokKey):
		m := current.end
		p.skip()
		current = p.getCurrentToken()
		if !isKind(current, tokKey, tokValue, tokBlockEnd) {
			p.pushState(psBlockMappingValue)
			return p.parseNode(true, true)
		}
		p.state = psBlockMappingValue
		return emptyScalar(m)
	case isKind(current, tokValue):
		p.skip()
		return emptyScalar(current.end)
	case isKind(current, tokAlias):
		p.skip()
		return &event{kind: evAlias, anchor: current.value, start: current.start, end: current.end}
	case isKind(current, tokBlockEnd):
		p.state = p.popState()
		evt := &event{kind: evMappingEnd, start: current.start, end: current.end}
		p.skip()
		return evt
	case isKind(p.getCurrentToken(), tokError):
		throwYAML(current.start, current.value)
	}
	throwYAML(tokStart(current), "While parsing a block mapping, did not find expected key.")
	return nil
}

func (p *parser) parseBlockMappingValue() *event {
	current := p.getCurrentToken()
	if isKind(current, tokValue) {
		m := current.end
		p.skip()
		current = p.getCurrentToken()
		if !isKind(current, tokKey, tokValue, tokBlockEnd) {
			p.pushState(psBlockMappingKey)
			return p.parseNode(true, true)
		}
		p.state = psBlockMappingKey
		return emptyScalar(m)
	} else if isKind(current, tokError) {
		throwYAML(current.start, current.value)
	}
	p.state = psBlockMappingKey
	return emptyScalar(tokStart(current))
}

func (p *parser) parseFlowSequenceEntry(isFirst bool) *event {
	if isFirst {
		p.getCurrentToken()
		p.skip()
	}
	current := p.getCurrentToken()
	if !isKind(current, tokFlowSequenceEnd) {
		if !isFirst {
			if isKind(current, tokFlowEntry) {
				p.skip()
				current = p.getCurrentToken()
			} else {
				throwYAML(tokStart(current), "While parsing a flow sequence, did not find expected ',' or ']'.")
			}
		}
		if isKind(current, tokKey) {
			p.state = psFlowSequenceEntryMappingKey
			evt := &event{kind: evMappingStart, start: emptyMark, end: emptyMark}
			p.skip()
			return evt
		} else if !isKind(current, tokFlowSequenceEnd) {
			p.pushState(psFlowSequenceEntry)
			return p.parseNode(false, false)
		}
	}
	p.state = p.popState()
	evt := &event{kind: evSequenceEnd, start: tokStart(current), end: tokEnd(current)}
	p.skip()
	return evt
}

func (p *parser) parseFlowSequenceEntryMappingKey() *event {
	current := p.getCurrentToken()
	if !isKind(current, tokValue, tokFlowEntry, tokFlowSequenceEnd) {
		p.pushState(psFlowSequenceEntryMappingValue)
		return p.parseNode(false, false)
	}
	m := tokEnd(current)
	p.skip()
	p.state = psFlowSequenceEntryMappingValue
	return emptyScalar(m)
}

func (p *parser) parseFlowSequenceEntryMappingValue() *event {
	current := p.getCurrentToken()
	if isKind(current, tokValue) {
		p.skip()
		current = p.getCurrentToken()
		if !isKind(current, tokFlowEntry, tokFlowSequenceEnd) {
			p.pushState(psFlowSequenceEntryMappingEnd)
			return p.parseNode(false, false)
		}
	}
	p.state = psFlowSequenceEntryMappingEnd
	return emptyScalar(tokStart(current))
}

func (p *parser) parseFlowSequenceEntryMappingEnd() *event {
	p.state = psFlowSequenceEntry
	current := p.getCurrentToken()
	return &event{kind: evMappingEnd, start: tokStart(current), end: tokEnd(current)}
}

func (p *parser) parseFlowMappingKey(isFirst bool) *event {
	if isFirst {
		p.getCurrentToken()
		p.skip()
	}
	current := p.getCurrentToken()
	if !isKind(current, tokFlowMappingEnd) {
		if !isFirst {
			if isKind(current, tokFlowEntry) {
				p.skip()
				current = p.getCurrentToken()
			} else if !isKind(current, tokScalar) {
				throwYAML(tokStart(current), "While parsing a flow mapping,  did not find expected ',' or '}'.")
			}
		}
		if isKind(current, tokKey) {
			p.skip()
			current = p.getCurrentToken()
			if !isKind(current, tokValue, tokFlowEntry, tokFlowMappingEnd) {
				p.pushState(psFlowMappingValue)
				return p.parseNode(false, false)
			}
			p.state = psFlowMappingValue
			return emptyScalar(tokStart(current))
		} else if isKind(current, tokScalar) {
			p.pushState(psFlowMappingValue)
			return p.parseNode(false, false)
		} else if !isKind(current, tokFlowMappingEnd) {
			p.pushState(psFlowMappingEmptyValue)
			return p.parseNode(false, false)
		}
	}
	p.state = p.popState()
	p.skip()
	return &event{kind: evMappingEnd, start: tokStart(current), end: tokEnd(current)}
}

func (p *parser) parseFlowMappingValue(isEmpty bool) *event {
	current := p.getCurrentToken()
	if !isEmpty && isKind(current, tokValue) {
		p.skip()
		current = p.getCurrentToken()
		if !isKind(current, tokFlowEntry, tokFlowMappingEnd) {
			p.pushState(psFlowMappingKey)
			return p.parseNode(false, false)
		}
	}
	p.state = psFlowMappingKey
	if !isEmpty && isKind(current, tokScalar) {
		p.skip()
		return &event{kind: evScalar, value: current.value, style: current.style, start: current.start, end: current.end}
	}
	return emptyScalar(tokStart(current))
}
