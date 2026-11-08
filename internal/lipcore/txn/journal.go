package txn

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

// op is JournalOp; its names are the "op" of the records (protocol/workspace.md
// §7.3).
type op int

const (
	opBegin op = iota
	opCreateDirectory
	opCreateFile
	opBackup
	opDeleteDirectoryOnCommit
	opCommit
)

var opNames = [...]string{"Begin", "CreateDirectory", "CreateFile", "Backup",
	"DeleteDirectoryOnCommit", "Commit"}

func (o op) String() string { return opNames[o] }

// entry is JournalEntry: to is the backup of a Backup, or the start time of a
// Begin; hidden is whether the file of a Backup was hidden. In memory paths
// are absolute; in the journal they are relative to the root, '/'-separated.
type entry struct {
	op     op
	path   string
	to     string
	hasTo  bool
	hidden bool
}

// hasPaths: whether path, and to of a Backup, are workspace paths.
func hasPaths(o op) bool { return o != opBegin && o != opCommit }

func hasDestination(o op) bool { return o == opBackup }

// startTime is DateTimeOffset.UtcNow.ToString("O").
func startTime(now time.Time) string {
	return now.UTC().Format("2006-01-02T15:04:05.0000000") + "+00:00"
}

// toJournal is WorkspaceTransaction.ToJournal.
func toJournal(root string, e entry) entry {
	if !hasPaths(e.op) {
		return e
	}
	e.path = relativePath(root, e.path)
	if hasDestination(e.op) {
		e.to = relativePath(root, e.to)
	}
	return e
}

// fromJournal is WorkspaceTransaction.FromJournal.
func fromJournal(root string, e entry) entry {
	if !hasPaths(e.op) {
		return e
	}
	e.path = absolutePath(root, e.path)
	if hasDestination(e.op) && e.hasTo {
		e.to = absolutePath(root, e.to)
	}
	return e
}

// relativePath is Path.GetRelativePath(root, path) with '/' separators. Only
// the separator becomes '/': outside Windows '\' is a character of a name.
func relativePath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	return filepath.ToSlash(rel)
}

// absolutePath is Path.GetFullPath(Path.Combine(root, path)).
func absolutePath(root, path string) string {
	p := filepath.FromSlash(path)
	if !rooted(p) {
		p = filepath.Join(root, p)
	}
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return filepath.Clean(p)
}

// rooted is Path.IsPathRooted.
func rooted(p string) bool {
	return filepath.IsAbs(p) || filepath.VolumeName(p) != "" ||
		(len(p) > 0 && (p[0] == '/' || p[0] == filepath.Separator))
}

// serialize is WorkspaceTransaction.Serialize: a record with only the
// members its op has, as JsonObject.ToJsonString writes it.
func serialize(e entry) string {
	var b strings.Builder
	b.WriteString(`{"op":`)
	b.WriteString(jsonString(e.op.String()))
	if e.op != opCommit {
		b.WriteString(`,"path":`)
		b.WriteString(jsonString(e.path))
	}
	if hasDestination(e.op) || (e.op == opBegin && e.hasTo) {
		b.WriteString(`,"to":`)
		if e.hasTo {
			b.WriteString(jsonString(e.to))
		} else {
			b.WriteString("null")
		}
	}
	if e.op == opBackup {
		if e.hidden {
			b.WriteString(`,"hidden":true`)
		} else {
			b.WriteString(`,"hidden":false`)
		}
	}
	b.WriteByte('}')
	return b.String()
}

// parse is WorkspaceTransaction.Parse: a line of the journal as a record, or
// false when it is none.
func parse(line []byte) (entry, bool) {
	// JsonDocument rejects text that is not UTF-8.
	if !utf8.Valid(line) {
		return entry{}, false
	}
	// Duplicate members: the last one counts, as for JsonElement.TryGetProperty.
	var root map[string]json.RawMessage
	if err := json.Unmarshal(line, &root); err != nil || root == nil {
		return entry{}, false
	}
	var name string
	if raw, ok := root["op"]; !ok || !isString(raw) || json.Unmarshal(raw, &name) != nil {
		return entry{}, false
	}
	kind := -1
	for i, n := range opNames {
		if n == name {
			kind = i
		}
	}
	if kind < 0 {
		return entry{}, false
	}
	e := entry{op: op(kind)}
	if e.op == opCommit {
		return e, true
	}
	to, hasTo := root["to"]
	hidden, hasHidden := root["hidden"]
	path, hasPath := root["path"]
	if !hasPath || !isString(path) ||
		((hasDestination(e.op) || (e.op == opBegin && hasTo)) && (!hasTo || !isString(to))) ||
		(e.op == opBackup && hasHidden && !isBool(hidden)) {
		return entry{}, false
	}
	if json.Unmarshal(path, &e.path) != nil {
		return entry{}, false
	}
	if hasTo && isString(to) {
		if json.Unmarshal(to, &e.to) != nil {
			return entry{}, false
		}
		e.hasTo = true
	}
	e.hidden = e.op == opBackup && hasHidden && string(bytes.TrimSpace(hidden)) == "true"
	return e, true
}

func isString(raw json.RawMessage) bool {
	t := bytes.TrimSpace(raw)
	return len(t) > 0 && t[0] == '"'
}

func isBool(raw json.RawMessage) bool {
	t := string(bytes.TrimSpace(raw))
	return t == "true" || t == "false"
}

// parseJournal splits a journal into its records: a record is a line ended by
// LF, so what follows the last LF (the cut-off line of a killed process) is
// none. Returns the records and the number of lines that are none.
func parseJournal(data []byte) ([]entry, int) {
	var entries []entry
	ignored := 0
	for start := 0; ; {
		end := bytes.IndexByte(data[start:], '\n')
		if end < 0 {
			break
		}
		if e, ok := parse(data[start : start+end]); ok {
			entries = append(entries, e)
		} else {
			ignored++
		}
		start += end + 1
	}
	return entries, ignored
}

const hexDigits = "0123456789ABCDEF"

// jsonString writes a JSON string as System.Text.Json does with the default
// encoder: '"', '&', '\”, '+', '<', '>', '`', DEL, control characters and
// every non-ASCII character as \uXXXX (UTF-16 code units, uppercase hex);
// '\\' as "\\\\"; \b \t \n \f \r in their short forms; invalid UTF-8 as
// �.
func jsonString(s string) string {
	b := make([]byte, 0, len(s)+2)
	appendU := func(r uint16) {
		b = append(b, '\\', 'u', hexDigits[r>>12&0xF], hexDigits[r>>8&0xF],
			hexDigits[r>>4&0xF], hexDigits[r&0xF])
	}
	b = append(b, '"')
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			i++
			switch c {
			case '\\':
				b = append(b, '\\', '\\')
			case '\b':
				b = append(b, '\\', 'b')
			case '\t':
				b = append(b, '\\', 't')
			case '\n':
				b = append(b, '\\', 'n')
			case '\f':
				b = append(b, '\\', 'f')
			case '\r':
				b = append(b, '\\', 'r')
			case '"', '&', '\'', '+', '<', '>', '`', 0x7F:
				appendU(uint16(c))
			default:
				if c < 0x20 {
					appendU(uint16(c))
				} else {
					b = append(b, c)
				}
			}
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		if r >= 0x10000 {
			r1, r2 := utf16.EncodeRune(r)
			appendU(uint16(r1))
			appendU(uint16(r2))
		} else {
			appendU(uint16(r))
		}
	}
	return string(append(b, '"'))
}
