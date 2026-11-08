// Package gitpack decodes git packfiles and walks trees, porting lip
// 0.38.0-beta.1 src/Lip.Core/Git/GitPackFile.cs, GitObject.cs,
// GitProtocolException.cs and ZlibInflater.cs, plus the tree walk of
// GitRemoteClient.WalkTree.
package gitpack

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ErrNotImplemented is returned by skeleton stubs.
var ErrNotImplemented = errors.New("gitpack: not implemented")

// ErrCorrupt is wrapped by every packfile/zlib decoding error (a
// *ProtocolError whose message is lip's "Corrupt packfile: <reason>.").
var ErrCorrupt = errors.New("corrupt packfile")

// ProtocolError is lip's GitProtocolException: a git remote misbehaved,
// reported an error or sent malformed data. Err is the inner error, if any.
type ProtocolError struct {
	Message string
	Err     error
}

func (e *ProtocolError) Error() string { return e.Message }

func (e *ProtocolError) Unwrap() error { return e.Err }

// UnusableRepositoryError is lip's UnusableRepositoryException: the content
// of a repository is something lip cannot handle (a file name that is not
// valid, an object that is too large, an object of the wrong type). Trying
// again or using another server does not help. Reason is in plain words,
// without a final period.
type UnusableRepositoryError struct{ Reason string }

func (e *UnusableRepositoryError) Error() string { return e.Reason }

// tooLarge is the UnusableRepositoryError for an object over MaxObjectSize.
func tooLarge(size int64) error {
	return &UnusableRepositoryError{Reason: fmt.Sprintf(
		"it contains an object of %d bytes, more than the %d bytes lip can handle", size, MaxObjectSize)}
}

// TypeName is lip's GitObject.GetTypeName.
func TypeName(t ObjectType) string { return t.String() }

// MaxObjectSize bounds a single object's size, guarding against
// decompression bombs (lip GitPackFile.MaxObjectSize).
const MaxObjectSize = 512 * 1024 * 1024

const (
	objOfsDelta = 6
	objRefDelta = 7
)

type ObjectType int

const (
	ObjCommit ObjectType = 1
	ObjTree   ObjectType = 2
	ObjBlob   ObjectType = 3
	ObjTag    ObjectType = 4
)

func (t ObjectType) String() string {
	switch t {
	case ObjCommit:
		return "commit"
	case ObjTree:
		return "tree"
	case ObjBlob:
		return "blob"
	case ObjTag:
		return "tag"
	}
	return "unknown"
}

// Object is a fully resolved (undeltified) object.
type Object struct {
	Type ObjectType
	Data []byte
}

// Store holds decoded objects keyed by lowercase hex SHA-1.
type Store struct {
	objects map[string]Object
}

// NewStore returns an empty store.
func NewStore() *Store { return &Store{objects: map[string]Object{}} }

// Len returns the number of objects in s.
func (s *Store) Len() int { return len(s.objects) }

// corrupt is lip's GitPackFile.Corrupt.
func corrupt(format string, args ...any) error {
	return &ProtocolError{Message: "Corrupt packfile: " + fmt.Sprintf(format, args...) + ".", Err: ErrCorrupt}
}

// Decode reads every object of a version 2/3 packfile into s (see Open for
// the checks made). Unlike lip, which only looks inside the pack, REF_DELTA
// bases may also come from objects already in s (thin packs). On error s may
// hold some of the pack's objects.
func (s *Store) Decode(pack []byte) error {
	p, err := OpenBytes(pack)
	if err != nil {
		return err
	}
	defer p.Close()
	p.external = func(oid string) (Object, bool) {
		o, ok := s.objects[oid]
		return o, ok
	}
	return p.ReadObjects(func(oid string, t ObjectType, data []byte) error {
		s.Add(oid, Object{Type: t, Data: data})
		return nil
	})
}

// ApplyDelta applies a git delta to a base object.
func ApplyDelta(base, delta []byte) ([]byte, error) {
	pos := 0
	readByte := func() (byte, error) {
		if pos >= len(delta) {
			return 0, corrupt("truncated delta")
		}
		b := delta[pos]
		pos++
		return b, nil
	}
	readSize := func() (int64, error) {
		var size int64
		shift := 0
		for {
			if shift > 56 {
				return 0, corrupt("delta size too large")
			}
			c, err := readByte()
			if err != nil {
				return 0, err
			}
			size |= int64(c&0x7f) << shift
			shift += 7
			if c&0x80 == 0 {
				return size, nil
			}
		}
	}
	srcSize, err := readSize()
	if err != nil {
		return nil, err
	}
	dstSize, err := readSize()
	if err != nil {
		return nil, err
	}
	if srcSize != int64(len(base)) {
		return nil, corrupt("delta base size mismatch")
	}
	if dstSize > MaxObjectSize {
		return nil, tooLarge(dstSize)
	}
	out := make([]byte, dstSize)
	o := 0
	for pos < len(delta) {
		op := delta[pos]
		pos++
		switch {
		case op&0x80 != 0:
			var off, sz int64
			for i := 0; i < 4; i++ {
				if op&(1<<i) != 0 {
					b, err := readByte()
					if err != nil {
						return nil, err
					}
					off |= int64(b) << (8 * i)
				}
			}
			for i := 0; i < 3; i++ {
				if op&(0x10<<i) != 0 {
					b, err := readByte()
					if err != nil {
						return nil, err
					}
					sz |= int64(b) << (8 * i)
				}
			}
			if sz == 0 {
				sz = 0x10000
			}
			if off+sz > int64(len(base)) || int64(o)+sz > int64(len(out)) {
				return nil, corrupt("delta copy out of range")
			}
			o += copy(out[o:], base[off:off+sz])
		case op != 0:
			n := int(op)
			if pos+n > len(delta) || o+n > len(out) {
				return nil, corrupt("delta insert out of range")
			}
			o += copy(out[o:], delta[pos:pos+n])
			pos += n
		default:
			return nil, corrupt("invalid delta opcode")
		}
	}
	if o != len(out) {
		return nil, corrupt("delta result size mismatch")
	}
	return out, nil
}

// Get returns the object with id oid.
func (s *Store) Get(oid string) (Object, bool) {
	o, ok := s.objects[oid]
	return o, ok
}

// Put adds an object and returns its id.
func (s *Store) Put(t ObjectType, data []byte) string {
	oid := HashObject(t, data)
	s.objects[oid] = Object{Type: t, Data: data}
	return oid
}

// Add stores o under oid, which the caller has already computed (e.g. from
// Pack.ReadObjects).
func (s *Store) Add(oid string, o Object) { s.objects[oid] = o }

// IsObjectID reports whether v is 40 ASCII hex digits (any case), like lip's
// GitObject.IsObjectId.
func IsObjectID(v string) bool {
	if len(v) != 40 {
		return false
	}
	for i := 0; i < len(v); i++ {
		c := v[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

// readHeader returns the trimmed value of the first "<key> " line in the
// header block (up to the first empty line) of a commit or tag.
func readHeader(content []byte, key string) (string, bool) {
	if end := bytes.Index(content, []byte("\n\n")); end >= 0 {
		content = content[:end]
	}
	for _, line := range strings.Split(decodeUTF8(content), "\n") {
		if strings.HasPrefix(line, key+" ") {
			return strings.TrimFunc(line[len(key)+1:], isDotNetWhiteSpace), true
		}
	}
	return "", false
}

// ParseCommitTree returns the tree id of raw commit data.
func ParseCommitTree(data []byte) (string, error) {
	tree, ok := readHeader(data, "tree")
	if !ok || !IsObjectID(tree) {
		return "", &ProtocolError{Message: "Malformed commit object: it has no valid tree header."}
	}
	return tree, nil
}

// ParseTagTarget returns the target id of raw annotated tag data.
func ParseTagTarget(data []byte) (string, error) {
	target, ok := readHeader(data, "object")
	if !ok || !IsObjectID(target) {
		return "", &ProtocolError{Message: "Malformed tag object: it has no valid object header."}
	}
	return target, nil
}

// missingObject is WalkTree's error for an object the server did not send.
func missingObject(oid string) error {
	return &ProtocolError{Message: fmt.Sprintf("The server did not send object %s.", oid)}
}

// CommitTree returns the root tree id of a commit object in s.
func (s *Store) CommitTree(commitOID string) (string, error) {
	o, ok := s.objects[commitOID]
	if !ok {
		return "", missingObject(commitOID)
	}
	if o.Type != ObjCommit {
		return "", notACommit(commitOID, o.Type)
	}
	return ParseCommitTree(o.Data)
}

func notACommit(oid string, t ObjectType) error {
	return &UnusableRepositoryError{Reason: fmt.Sprintf(
		"the object %s is a %s, not a commit or a tag of one", oid, TypeName(t))}
}

// PeelCommit follows annotated tags from oid and returns the commit id they
// point to, as lip's WalkTree does.
func (s *Store) PeelCommit(oid string) (string, error) {
	cur := oid
	// Object ids are hashes of the content, so tags cannot form a cycle;
	// the bound only guards against a corrupt store.
	for range len(s.objects) + 1 {
		o, ok := s.objects[cur]
		if !ok {
			return "", missingObject(cur)
		}
		switch o.Type {
		case ObjTag:
			t, err := ParseTagTarget(o.Data)
			if err != nil {
				return "", err
			}
			cur = t
		case ObjCommit:
			return cur, nil
		default:
			return "", notACommit(oid, o.Type)
		}
	}
	return "", notACommit(oid, ObjTag)
}

// TreeEntry is one raw tree entry.
type TreeEntry struct {
	// Mode is the octal mode digits read as a decimal number and formatted
	// back, as lip does (e.g. "100644", "40000"; "040000" becomes "40000").
	Mode string
	Name string
	OID  string
}

// ParseTree parses raw tree object data. Modes must be 1-7 octal digits;
// names are decoded as UTF-8 (invalid bytes become U+FFFD).
func ParseTree(data []byte) ([]TreeEntry, error) {
	var out []TreeEntry
	pos := 0
	for pos < len(data) {
		sp := bytes.IndexByte(data[pos:], ' ')
		nul := -1
		if sp >= 0 {
			sp += pos
			if n := bytes.IndexByte(data[sp+1:], 0); n >= 0 {
				nul = sp + 1 + n
			}
		}
		if sp < 0 || nul < 0 || nul+21 > len(data) {
			return nil, &ProtocolError{Message: fmt.Sprintf("Malformed tree object: the entry at byte %d is truncated.", pos)}
		}
		modeText := ASCIIString(data[pos:sp])
		if len(modeText) == 0 || len(modeText) > 7 || strings.Trim(modeText, "01234567") != "" {
			return nil, &ProtocolError{Message: fmt.Sprintf("Malformed tree entry mode '%s'.", modeText)}
		}
		mode, _ := strconv.Atoi(modeText)
		out = append(out, TreeEntry{
			Mode: strconv.Itoa(mode),
			Name: decodeUTF8(data[sp+1 : nul]),
			OID:  hex.EncodeToString(data[nul+1 : nul+21]),
		})
		pos = nul + 21
	}
	return out, nil
}

// File is a non-tree, non-gitlink entry reachable from a tree (lip's
// GitTreeEntry).
type File struct {
	// Path is relative, with "/" separators.
	Path string
	OID  string
	// Mode is the octal mode digits read as a decimal number, as git prints
	// them, e.g. "100644", "100755" or "120000".
	Mode string
}

// IsRegularFile reports whether mode is 100644 or 100755 (lip's
// GitTreeEntry.IsRegularFile): a regular, possibly executable, file rather
// than e.g. a symlink.
func IsRegularFile(mode string) bool {
	return mode == "100644" || mode == "100755"
}

// IsRegularFile reports whether f is a regular (possibly executable) file
// rather than e.g. a symlink.
func (f File) IsRegularFile() bool { return IsRegularFile(f.Mode) }

// WalkTree lists every non-tree, non-gitlink entry under treeOID
// recursively, exactly like lip's GitRemoteClient.WalkTree: symlinks (120000)
// and any other non-40000/160000 modes are included. An entry named "", ".",
// ".." or containing "/" is an UnusableRepositoryError (protocol/package.md
// section 1); a missing or non-tree subtree is a ProtocolError.
func (s *Store) WalkTree(treeOID string) ([]File, error) {
	var out []File
	var walk func(prefix, oid string) error
	walk = func(prefix, oid string) error {
		o, ok := s.objects[oid]
		if !ok {
			return missingObject(oid)
		}
		if o.Type != ObjTree {
			return &ProtocolError{Message: fmt.Sprintf("Object %s at '%s' is a %s, not a tree.", oid, prefix, TypeName(o.Type))}
		}
		entries, err := ParseTree(o.Data)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if !isSafeEntryName(e.Name) {
				return &UnusableRepositoryError{Reason: fmt.Sprintf(
					"it has an entry named '%s%s' (in tree %s), which is not a valid file name", prefix, e.Name, oid)}
			}
			p := prefix + e.Name
			switch e.Mode {
			case "40000":
				if err := walk(p+"/", e.OID); err != nil {
					return err
				}
			case "160000":
			default:
				out = append(out, File{Path: p, OID: e.OID, Mode: e.Mode})
			}
		}
		return nil
	}
	if err := walk("", treeOID); err != nil {
		return nil, err
	}
	return out, nil
}

// ListTree peels commitish (a commit or annotated tag id) to a commit and
// walks its root tree with WalkTree, like lip's GitRemoteClient.WalkTree.
func (s *Store) ListTree(commitish string) ([]File, error) {
	commit, err := s.PeelCommit(commitish)
	if err != nil {
		return nil, err
	}
	tree, err := s.CommitTree(commit)
	if err != nil {
		return nil, err
	}
	return s.WalkTree(tree)
}

// WalkFiles lists the regular files (modes 100644, 100755) under treeOID
// recursively; gitlinks, symlinks and other modes are skipped. Names are
// checked and subtrees required as by WalkTree.
func (s *Store) WalkFiles(treeOID string) ([]File, error) {
	all, err := s.WalkTree(treeOID)
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, f := range all {
		if f.IsRegularFile() {
			out = append(out, f)
		}
	}
	return out, nil
}

// isSafeEntryName rejects names a git archive cannot hold (lip's
// IsSafeEntryName, protocol/package.md section 1).
func isSafeEntryName(name string) bool {
	switch name {
	case "", ".", "..":
		return false
	}
	return !strings.ContainsAny(name, "/\\")
}

// HashObject returns the hex SHA-1 of "<type> <len>\x00<data>".
func HashObject(t ObjectType, data []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "%s %d\x00", t, len(data))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyBlob reports an error if data does not hash to oid as a blob. As in
// lip, oid is compared case-insensitively and a 64-character oid is checked
// with SHA-256.
func VerifyBlob(oid string, data []byte) error {
	var h hash.Hash
	if len(oid) == 64 {
		h = sha256.New()
	} else {
		h = sha1.New()
	}
	fmt.Fprintf(h, "blob %d\x00", len(data))
	h.Write(data)
	actual := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actual, oid) {
		return fmt.Errorf("blob content hash %s does not match object id %s", actual, oid)
	}
	return nil
}

// DecodeUTF8 decodes b like .NET Encoding.UTF8.GetString (invalid
// sequences become U+FFFD).
func DecodeUTF8(b []byte) string { return decodeUTF8(b) }

// TrimSpace trims leading and trailing white space like .NET String.Trim().
func TrimSpace(s string) string { return strings.TrimFunc(s, isDotNetWhiteSpace) }

// ASCIIString decodes b like .NET Encoding.ASCII.GetString: bytes above
// 0x7F become '?'.
func ASCIIString(b []byte) string {
	out := make([]byte, len(b))
	for i, c := range b {
		if c > 0x7f {
			c = '?'
		}
		out[i] = c
	}
	return string(out)
}

// decodeUTF8 decodes b like .NET Encoding.UTF8.GetString: each maximal
// subpart of an ill-formed sequence (a lead byte followed by the longest run
// of continuation bytes still forming a valid prefix, per Unicode Table 3-7)
// becomes a single U+FFFD.
func decodeUTF8(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	var sb strings.Builder
	for len(b) > 0 {
		r, n := utf8.DecodeRune(b)
		if r == utf8.RuneError && n == 1 {
			n = maximalSubpart(b)
		}
		sb.WriteRune(r)
		b = b[n:]
	}
	return sb.String()
}

// maximalSubpart returns the length (at least 1) of the ill-formed sequence
// at the start of b.
func maximalSubpart(b []byte) int {
	lo, hi := byte(0x80), byte(0xbf)
	var need int
	switch c := b[0]; {
	case c >= 0xc2 && c <= 0xdf:
		need = 1
	case c == 0xe0:
		need, lo = 2, 0xa0
	case c == 0xed:
		need, hi = 2, 0x9f
	case c >= 0xe1 && c <= 0xef:
		need = 2
	case c == 0xf0:
		need, lo = 3, 0x90
	case c == 0xf4:
		need, hi = 3, 0x8f
	case c >= 0xf1 && c <= 0xf3:
		need = 3
	default:
		return 1
	}
	n := 1
	for n <= need && n < len(b) && b[n] >= lo && b[n] <= hi {
		n++
		lo, hi = 0x80, 0xbf
	}
	return n
}

// isDotNetWhiteSpace matches char.IsWhiteSpace, used by string.Trim().
func isDotNetWhiteSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\v', '\f', '\r', 0x85, 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}
