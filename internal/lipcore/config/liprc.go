package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// This file reads liprc.json as ConfigService.MigrateLiprc does, with
// JsonDocument.Parse(byte[]) and the default JsonDocumentOptions: strict
// JSON syntax (no comments, no trailing commas, no byte order mark), at most
// 64 levels of nesting, and duplicate member names allowed, the last one
// winning in TryGetProperty. String contents are not checked while parsing;
// JsonElement.GetString checks the one string lip reads, github_proxy.
//
// The messages of syntax errors follow System.Text.Json's for the common
// cases ("... LineNumber: 0 | BytePositionInLine: 3."); rarer ones are
// worded generically.

// liprcString is a JSON string as written between its quotes, escapes
// included; decoded only when read (GetString).
type liprcString []byte

// liprcObject keeps the last value of each member name.
type liprcObject map[string]any

type liprcParser struct {
	data      []byte
	pos       int
	line      int
	lineStart int
}

func (p *liprcParser) fail(format string, args ...any) error {
	return fmt.Errorf(format+" LineNumber: %d | BytePositionInLine: %d.",
		append(args, p.line, p.pos-p.lineStart)...)
}

func (p *liprcParser) skipWS() {
	for p.pos < len(p.data) {
		switch p.data[p.pos] {
		case '\n':
			p.line++
			p.lineStart = p.pos + 1
		case ' ', '\t', '\r':
		default:
			return
		}
		p.pos++
	}
}

// show is how System.Text.Json names a byte in messages.
func show(c byte) string {
	if c < 0x20 || c >= 0x7F {
		return fmt.Sprintf("0x%02X", c)
	}
	return string(c)
}

func parseLiprcJSON(data []byte) (any, error) {
	p := &liprcParser{data: data}
	p.skipWS()
	if p.pos >= len(p.data) {
		return nil, p.fail("The input does not contain any JSON tokens. Expected the input to start with a" +
			" valid JSON token, when isFinalBlock is true.")
	}
	v, err := p.value(0)
	if err != nil {
		return nil, err
	}
	p.skipWS()
	if p.pos < len(p.data) {
		return nil, p.fail("'%s' is invalid after a single JSON value. Expected end of data.", show(p.data[p.pos]))
	}
	return v, nil
}

func (p *liprcParser) value(depth int) (any, error) {
	if p.pos >= len(p.data) {
		return nil, p.fail("Expected depth to be zero at the end of the JSON payload. There is an open JSON" +
			" object or array that should be closed.")
	}
	switch c := p.data[p.pos]; {
	case c == '{' || c == '[':
		if depth >= 64 {
			return nil, p.fail("The maximum configured depth of 64 has been exceeded. Cannot read next JSON %s.",
				map[bool]string{true: "object", false: "array"}[c == '{'])
		}
		p.pos++
		if c == '{' {
			return p.object(depth + 1)
		}
		return p.array(depth + 1)
	case c == '"':
		return p.str()
	case c == '-' || (c >= '0' && c <= '9'):
		return p.number()
	case c == 't':
		return true, p.literal("true")
	case c == 'f':
		return false, p.literal("false")
	case c == 'n':
		return nil, p.literal("null")
	default:
		return nil, p.fail("'%s' is an invalid start of a value.", show(c))
	}
}

func (p *liprcParser) object(depth int) (any, error) {
	o := liprcObject{}
	p.skipWS()
	if p.pos < len(p.data) && p.data[p.pos] == '}' {
		p.pos++
		return o, nil
	}
	for {
		p.skipWS()
		if p.pos >= len(p.data) {
			return nil, p.fail("Expected depth to be zero at the end of the JSON payload. There is an open JSON" +
				" object or array that should be closed.")
		}
		if c := p.data[p.pos]; c != '"' {
			if c == '}' {
				return nil, p.fail("The JSON object contains a trailing comma at the end which is not supported" +
					" in this mode. Change the reader options.")
			}
			return nil, p.fail("'%s' is an invalid start of a property name. Expected a '\"'.", show(c))
		}
		k, err := p.str()
		if err != nil {
			return nil, err
		}
		p.skipWS()
		if p.pos >= len(p.data) || p.data[p.pos] != ':' {
			if p.pos >= len(p.data) {
				return nil, p.fail("Expected depth to be zero at the end of the JSON payload. There is an open" +
					" JSON object or array that should be closed.")
			}
			return nil, p.fail("'%s' is invalid after a property name. Expected a ':'.", show(p.data[p.pos]))
		}
		p.pos++
		p.skipWS()
		v, err := p.value(depth)
		if err != nil {
			return nil, err
		}
		// Names compare as their unescaped text; one that does not decode
		// cannot be looked up as github_proxy or format_version anyway.
		name, _ := decodeJSONString(k.(liprcString))
		o[name] = v
		p.skipWS()
		if p.pos >= len(p.data) {
			return nil, p.fail("Expected depth to be zero at the end of the JSON payload. There is an open JSON" +
				" object or array that should be closed.")
		}
		switch c := p.data[p.pos]; c {
		case ',':
			p.pos++
		case '}':
			p.pos++
			return o, nil
		default:
			return nil, p.fail("'%s' is invalid after a value. Expected either ',', '}', or ']'.", show(c))
		}
	}
}

func (p *liprcParser) array(depth int) (any, error) {
	var a []any
	p.skipWS()
	if p.pos < len(p.data) && p.data[p.pos] == ']' {
		p.pos++
		return []any{}, nil
	}
	for {
		p.skipWS()
		if p.pos < len(p.data) && p.data[p.pos] == ']' {
			return nil, p.fail("The JSON array contains a trailing comma at the end which is not supported in" +
				" this mode. Change the reader options.")
		}
		v, err := p.value(depth)
		if err != nil {
			return nil, err
		}
		a = append(a, v)
		p.skipWS()
		if p.pos >= len(p.data) {
			return nil, p.fail("Expected depth to be zero at the end of the JSON payload. There is an open JSON" +
				" object or array that should be closed.")
		}
		switch c := p.data[p.pos]; c {
		case ',':
			p.pos++
		case ']':
			p.pos++
			return a, nil
		default:
			return nil, p.fail("'%s' is invalid after a value. Expected either ',', '}', or ']'.", show(c))
		}
	}
}

// literal consumes true, false or null. On a mismatch System.Text.Json shows
// the rest of the data, at the first byte that differs (or the end).
func (p *liprcParser) literal(word string) error {
	rest := p.data[p.pos:]
	if len(rest) >= len(word) && string(rest[:len(word)]) == word {
		p.pos += len(word)
		return nil
	}
	i := 1
	for i < len(word) && i < len(rest) && rest[i] == word[i] {
		i++
	}
	p.pos += i
	return p.fail("'%s' is an invalid JSON literal. Expected the literal '%s'.", string(rest), word)
}

func (p *liprcParser) number() (any, error) {
	start := p.pos
	digits := func() int {
		n := 0
		for p.pos < len(p.data) && p.data[p.pos] >= '0' && p.data[p.pos] <= '9' {
			p.pos++
			n++
		}
		return n
	}
	afterSign := func() (any, error) {
		return nil, p.fail("'%s' is invalid within a number, immediately after a sign character ('+' or"+
			" '-'). Expected a digit ('0'-'9').", showAt(p.data, p.pos))
	}
	if p.data[p.pos] == '-' {
		p.pos++
	}
	if p.pos < len(p.data) && p.data[p.pos] == '0' {
		p.pos++
		if p.pos < len(p.data) && p.data[p.pos] >= '0' && p.data[p.pos] <= '9' {
			return nil, p.fail("Invalid leading zero before '%s'.", show(p.data[p.pos]))
		}
	} else if digits() == 0 {
		return afterSign()
	}
	if p.pos < len(p.data) && p.data[p.pos] == '.' {
		p.pos++
		if digits() == 0 {
			return nil, p.fail("'%s' is invalid within a number, immediately after a decimal point ('.')."+
				" Expected a digit ('0'-'9').", showAt(p.data, p.pos))
		}
	}
	if p.pos < len(p.data) && (p.data[p.pos] == 'e' || p.data[p.pos] == 'E') {
		p.pos++
		if p.pos < len(p.data) && (p.data[p.pos] == '+' || p.data[p.pos] == '-') {
			p.pos++
		}
		if digits() == 0 {
			return afterSign()
		}
	}
	if p.pos < len(p.data) {
		switch c := p.data[p.pos]; c {
		case ' ', '\t', '\r', '\n', ',', '}', ']':
		default:
			return nil, p.fail("'%s' is an invalid end of a number. Expected a delimiter.", show(c))
		}
	}
	return json.Number(p.data[start:p.pos]), nil
}

func showAt(b []byte, i int) string {
	if i >= len(b) {
		return "end of data"
	}
	return show(b[i])
}

// str scans a string token: no control characters, valid escapes. Its
// content is not decoded.
func (p *liprcParser) str() (any, error) {
	p.pos++
	start := p.pos
	for p.pos < len(p.data) {
		c := p.data[p.pos]
		switch {
		case c == '"':
			s := liprcString(p.data[start:p.pos])
			p.pos++
			return s, nil
		case c < 0x20:
			return nil, p.fail("'%s' is invalid within a JSON string. The string should be correctly escaped.", show(c))
		case c == '\\':
			p.pos++
			if p.pos >= len(p.data) {
				break
			}
			switch e := p.data[p.pos]; e {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
				p.pos++
			case 'u':
				p.pos++
				for i := 0; i < 4; i++ {
					if p.pos >= len(p.data) || !isHex(p.data[p.pos]) {
						return nil, p.fail("'%s' is not a hex digit following '\\u' within a JSON string. The string"+
							" should be correctly escaped.", showAt(p.data, p.pos))
					}
					p.pos++
				}
			default:
				return nil, p.fail("'%s' is an invalid escapable character within a JSON string. The string"+
					" should be correctly escaped.", show(e))
			}
		default:
			p.pos++
		}
	}
	return nil, p.fail("Expected end of string, but instead reached end of data.")
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// decodeJSONString is JsonElement.GetString on a string token: the bytes
// must be valid UTF-8 and every \u escape of a surrogate must be paired.
func decodeJSONString(raw liprcString) (string, error) {
	if !utf8.Valid(raw) {
		return "", errors.New("Cannot transcode invalid UTF-8 JSON text to UTF-16 string.")
	}
	out := make([]byte, 0, len(raw))
	for i := 0; i < len(raw); {
		c := raw[i]
		if c != '\\' {
			out = append(out, c)
			i++
			continue
		}
		e := raw[i+1]
		i += 2
		switch e {
		case 'b':
			out = append(out, '\b')
		case 'f':
			out = append(out, '\f')
		case 'n':
			out = append(out, '\n')
		case 'r':
			out = append(out, '\r')
		case 't':
			out = append(out, '\t')
		case 'u':
			r := hex4(raw[i:])
			i += 4
			if utf16.IsSurrogate(r) {
				if r >= 0xDC00 {
					return "", fmt.Errorf("Cannot read invalid UTF-16 JSON text as string. Invalid surrogate"+
						" value: '0x%04X'.", r)
				}
				if i+6 > len(raw) || raw[i] != '\\' || raw[i+1] != 'u' {
					return "", errors.New("Cannot read incomplete UTF-16 JSON text as string with missing low surrogate.")
				}
				r2 := hex4(raw[i+2:])
				if r2 < 0xDC00 || r2 > 0xDFFF {
					return "", errors.New("Cannot read incomplete UTF-16 JSON text as string with missing low surrogate.")
				}
				r = utf16.DecodeRune(r, r2)
				i += 6
			}
			out = utf8.AppendRune(out, r)
		default: // '"', '\\', '/'
			out = append(out, e)
		}
	}
	return string(out), nil
}

func hex4(b []byte) rune {
	n, _ := strconv.ParseUint(string(b[:4]), 16, 32)
	return rune(n)
}
