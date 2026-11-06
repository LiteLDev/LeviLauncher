// Package gitpack decodes git packfiles and walks trees, porting lip 0.37.0
// src/Lip.Core/Git/GitPackFile.cs, GitObject.cs and ZlibInflater.cs, plus the
// tree walk of GitRemoteClient.WalkTree.
package gitpack

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ErrNotImplemented is returned by skeleton stubs.
var ErrNotImplemented = errors.New("gitpack: not implemented")

// ErrCorrupt is wrapped by every packfile/zlib decoding error.
var ErrCorrupt = errors.New("corrupt packfile")

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

func corrupt(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrCorrupt, fmt.Sprintf(format, args...))
}

type packEntry struct {
	typ      int
	data     []byte // inflated object or delta data
	baseIdx  int    // OFS_DELTA base entry index
	baseOID  string // REF_DELTA base id
	resolved bool
	rtype    ObjectType
	rdata    []byte
}

// Decode parses a version 2/3 packfile (header, entries, trailing SHA-1
// checksum) into s, resolving OFS_DELTA and REF_DELTA entries (ref bases
// may be objects already in s). Object ids are computed with HashObject.
//
// As in lip, the trailing checksum is verified before any entry is read,
// every entry must inflate to exactly its declared size, OFS_DELTA bases
// must start exactly at an earlier entry, and the last entry must end
// exactly at the trailer. Unlike lip, which only looks inside the pack,
// REF_DELTA bases may also come from objects already in s. On error s may
// hold some of the pack's objects.
func (s *Store) Decode(pack []byte) error {
	if len(pack) < 32 {
		return corrupt("too short")
	}
	if !bytes.Equal(pack[:4], []byte("PACK")) {
		return corrupt("bad signature")
	}
	version := binary.BigEndian.Uint32(pack[4:8])
	if version != 2 && version != 3 {
		return corrupt("unsupported version %d", version)
	}
	count := binary.BigEndian.Uint32(pack[8:12])

	end := len(pack) - 20
	sum := sha1.Sum(pack[:end])
	if !bytes.Equal(sum[:], pack[end:]) {
		return corrupt("checksum mismatch")
	}

	entries := make([]*packEntry, 0, min(int(count), end/2))
	indexByOffset := map[int]int{}
	pos := 12
	var z inflater
	readByte := func() (int, error) {
		if pos >= len(pack) {
			return 0, corrupt("unexpected end")
		}
		b := pack[pos]
		pos++
		return int(b), nil
	}

	for i := uint32(0); i < count; i++ {
		offset := pos
		if offset >= end {
			return corrupt("object count exceeds data")
		}
		c, err := readByte()
		if err != nil {
			return err
		}
		typ := (c >> 4) & 7
		size := int64(c & 0x0f)
		shift := 4
		for c&0x80 != 0 {
			if shift > 56 {
				return corrupt("object size too large")
			}
			if c, err = readByte(); err != nil {
				return err
			}
			size |= int64(c&0x7f) << shift
			shift += 7
		}

		e := &packEntry{typ: typ, baseIdx: -1}
		switch {
		case typ == objOfsDelta:
			if c, err = readByte(); err != nil {
				return err
			}
			distance := int64(c & 0x7f)
			for c&0x80 != 0 {
				if distance > (1<<63-1)>>8 {
					return corrupt("delta offset too large")
				}
				if c, err = readByte(); err != nil {
					return err
				}
				distance = ((distance + 1) << 7) | int64(c&0x7f)
			}
			baseIdx, ok := -1, false
			if distance != 0 && distance <= int64(offset) {
				baseIdx, ok = indexByOffset[offset-int(distance)]
			}
			if !ok {
				return corrupt("invalid delta base offset")
			}
			e.baseIdx = baseIdx
		case typ == objRefDelta:
			if pos+20 > len(pack) {
				return corrupt("unexpected end")
			}
			e.baseOID = hex.EncodeToString(pack[pos : pos+20])
			pos += 20
		case typ < 1 || typ > 4:
			return corrupt("invalid object type %d", typ)
		}

		if size > MaxObjectSize {
			return corrupt("object of %d bytes exceeds the %d byte limit", size, MaxObjectSize)
		}

		data, n, err := z.inflate(pack[pos:end], size)
		if err != nil {
			return err
		}
		pos += n
		e.data = data

		indexByOffset[offset] = len(entries)
		entries = append(entries, e)
	}
	if pos != end {
		return corrupt("unexpected data after last object")
	}

	// canResolve reports whether every REF_DELTA base in the entry's delta
	// chain is available.
	canResolve := func(i int) (bool, error) {
		for steps := 0; steps <= len(entries); steps++ {
			e := entries[i]
			if e.resolved {
				return true, nil
			}
			switch e.typ {
			case objOfsDelta:
				i = e.baseIdx
			case objRefDelta:
				_, ok := s.objects[e.baseOID]
				return ok, nil
			default:
				return true, nil
			}
		}
		return false, corrupt("delta cycle")
	}

	// Resolve in pack order; REF_DELTA bases may appear after their deltas,
	// so unresolvable entries are retried until no progress is made.
	deferred := make([]int, len(entries))
	for i := range deferred {
		deferred[i] = i
	}
	for len(deferred) > 0 {
		var still []int
		for _, i := range deferred {
			ok, err := canResolve(i)
			if err != nil {
				return err
			}
			if !ok {
				still = append(still, i)
				continue
			}
			if err := s.resolve(entries, i); err != nil {
				return err
			}
		}
		if len(still) == len(deferred) {
			return corrupt("missing delta base")
		}
		deferred = still
	}
	return nil
}

// resolve undeltifies entry i (whose chain is resolvable) and puts it in s.
func (s *Store) resolve(entries []*packEntry, i int) error {
	var chain []int
	cur := i
	var t ObjectType
	var data []byte
	for {
		e := entries[cur]
		if e.resolved {
			t, data = e.rtype, e.rdata
			break
		}
		if e.typ != objOfsDelta && e.typ != objRefDelta {
			t, data = ObjectType(e.typ), e.data
			break
		}
		chain = append(chain, cur)
		if len(chain) > len(entries) {
			return corrupt("delta cycle")
		}
		if e.typ == objOfsDelta {
			cur = e.baseIdx
			continue
		}
		base, ok := s.objects[e.baseOID]
		if !ok {
			return corrupt("missing delta base %s", e.baseOID)
		}
		t, data = base.Type, base.Data
		break
	}
	for k := len(chain) - 1; k >= 0; k-- {
		e := entries[chain[k]]
		out, err := ApplyDelta(data, e.data)
		if err != nil {
			return err
		}
		data = out
		e.resolved, e.rtype, e.rdata, e.data = true, t, data, nil
	}
	e := entries[i]
	if !e.resolved {
		e.resolved, e.rtype, e.rdata, e.data = true, t, data, nil
	}
	s.Put(e.rtype, e.rdata)
	return nil
}

type inflater struct {
	r  bytes.Reader
	zr io.ReadCloser
}

// inflate decompresses one zlib stream at the start of src that must expand
// to exactly size bytes, returning the data and the number of compressed
// bytes consumed (including the Adler-32 trailer).
// The decompressor is reused across calls to avoid reallocating its window.
func (z *inflater) inflate(src []byte, size int64) ([]byte, int, error) {
	// bytes.Reader implements io.ByteReader, so compress/flate reads exactly
	// the bytes of the stream and no more.
	r := &z.r
	r.Reset(src)
	var err error
	if z.zr == nil {
		z.zr, err = zlib.NewReader(r)
	} else {
		err = z.zr.(zlib.Resetter).Reset(r, nil)
	}
	if err != nil {
		z.zr = nil
		return nil, 0, corrupt("invalid zlib header: %v", err)
	}
	zr := z.zr
	// Large buffers grow as data arrives so a bogus header size cannot force
	// a huge allocation up front.
	var out []byte
	if size <= 1<<20 {
		out = make([]byte, size)
		if _, err := io.ReadFull(zr, out); err != nil {
			return nil, 0, corrupt("zlib stream: expected %d bytes: %v", size, err)
		}
	} else {
		var err error
		if out, err = io.ReadAll(io.LimitReader(zr, size)); err != nil {
			return nil, 0, corrupt("zlib stream: %v", err)
		}
		if int64(len(out)) != size {
			return nil, 0, corrupt("zlib stream: expected %d bytes but got %d", size, len(out))
		}
	}
	var one [1]byte
	n, err := zr.Read(one[:])
	for n == 0 && err == nil {
		n, err = zr.Read(one[:])
	}
	if n > 0 {
		return nil, 0, corrupt("zlib stream: output exceeds declared size %d", size)
	}
	if err != io.EOF {
		return nil, 0, corrupt("zlib stream: %v", err)
	}
	return out, len(src) - r.Len(), nil
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
		return nil, corrupt("delta result too large")
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
		return "", errors.New("malformed commit object: missing tree")
	}
	return tree, nil
}

// ParseTagTarget returns the target id of raw annotated tag data.
func ParseTagTarget(data []byte) (string, error) {
	target, ok := readHeader(data, "object")
	if !ok || !IsObjectID(target) {
		return "", errors.New("malformed tag object: missing target")
	}
	return target, nil
}

// CommitTree returns the root tree id of a commit object in s.
func (s *Store) CommitTree(commitOID string) (string, error) {
	o, ok := s.objects[commitOID]
	if !ok {
		return "", fmt.Errorf("remote did not send object %s", commitOID)
	}
	if o.Type != ObjCommit {
		return "", fmt.Errorf("object %s is not a commit", commitOID)
	}
	return ParseCommitTree(o.Data)
}

// PeelCommit follows annotated tags from oid and returns the commit id they
// point to, as lip's WalkTree does.
func (s *Store) PeelCommit(oid string) (string, error) {
	cur := oid
	for range len(s.objects) + 1 {
		o, ok := s.objects[cur]
		if !ok {
			return "", fmt.Errorf("remote did not send object %s", cur)
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
			return "", fmt.Errorf("object %s does not point to a commit", oid)
		}
	}
	return "", fmt.Errorf("object %s does not point to a commit", oid)
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
			return nil, errors.New("malformed tree object")
		}
		modeText := string(data[pos:sp])
		if len(modeText) == 0 || len(modeText) > 7 || strings.Trim(modeText, "01234567") != "" {
			return nil, fmt.Errorf("malformed tree entry mode '%s'", modeText)
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

// File is a regular file reachable from a tree.
type File struct {
	// Path is relative, with "/" separators.
	Path string
	OID  string
	Mode string
}

// IsRegularFile reports whether mode is 100644, 100755 or 100664.
func IsRegularFile(mode string) bool {
	return mode == "100644" || mode == "100755" || mode == "100664"
}

// IsRegularFile reports whether f is a regular (possibly executable) file
// rather than e.g. a symlink.
func (f File) IsRegularFile() bool { return IsRegularFile(f.Mode) }

// WalkTree lists every non-tree, non-gitlink entry under treeOID
// recursively, exactly like lip's GitRemoteClient.WalkTree: symlinks (120000)
// and any other non-40000/160000 modes are included. Names "", ".", "..",
// ".git" (case-insensitive) or containing "/", "\" or NUL are errors, as is a
// missing or non-tree subtree.
func (s *Store) WalkTree(treeOID string) ([]File, error) {
	var out []File
	var walk func(prefix, oid string) error
	walk = func(prefix, oid string) error {
		o, ok := s.objects[oid]
		if !ok {
			return fmt.Errorf("remote did not send object %s", oid)
		}
		if o.Type != ObjTree {
			return fmt.Errorf("object %s is not a tree", oid)
		}
		entries, err := ParseTree(o.Data)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if !isSafeEntryName(e.Name) {
				return fmt.Errorf("unsafe tree entry name '%s%s'", prefix, e.Name)
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
// walks its root tree with WalkTree, like lip's GitRemoteClient.ListTree
// after the fetch.
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

// WalkFiles lists all regular files (modes 100644, 100755, 100664) under
// treeOID recursively; gitlinks and symlinks are skipped. Entry names ".",
// "..", ".git" or containing "/", "\" or NUL are errors. Every subtree must
// be present in s.
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

// isSafeEntryName rejects names git itself refuses to check out.
func isSafeEntryName(name string) bool {
	switch name {
	case "", ".", "..":
		return false
	}
	return !strings.EqualFold(name, ".git") && !strings.ContainsAny(name, "/\\\x00")
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

// decodeUTF8 decodes b like .NET Encoding.UTF8.GetString, replacing invalid
// sequences with U+FFFD.
func decodeUTF8(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	var sb strings.Builder
	for len(b) > 0 {
		r, n := utf8.DecodeRune(b)
		sb.WriteRune(r)
		b = b[n:]
	}
	return sb.String()
}

// isDotNetWhiteSpace matches char.IsWhiteSpace, used by string.Trim().
func isDotNetWhiteSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\v', '\f', '\r', 0x85, 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}
