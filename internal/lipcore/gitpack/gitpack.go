// Package gitpack decodes git packfiles and walks trees, porting lip 0.37.0
// src/Lip.Core/Git/GitPackFile.cs, GitObject.cs and ZlibInflater.cs.
package gitpack

import "errors"

// ErrNotImplemented is returned by skeleton stubs.
var ErrNotImplemented = errors.New("gitpack: not implemented")

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

// Decode parses a version 2/3 packfile (header, entries, trailing SHA-1
// checksum) into s, resolving OFS_DELTA and REF_DELTA entries (ref bases
// may be objects already in s). Object ids are computed with HashObject.
func (s *Store) Decode(pack []byte) error {
	return ErrNotImplemented
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

// CommitTree returns the root tree id of a commit object in s.
func (s *Store) CommitTree(commitOID string) (string, error) {
	return "", ErrNotImplemented
}

// TreeEntry is one raw tree entry.
type TreeEntry struct {
	Mode string // e.g. "100644", "40000"
	Name string
	OID  string
}

// ParseTree parses raw tree object data.
func ParseTree(data []byte) ([]TreeEntry, error) {
	return nil, ErrNotImplemented
}

// File is a regular file reachable from a tree.
type File struct {
	// Path is relative, with "/" separators.
	Path string
	OID  string
	Mode string
}

// WalkFiles lists all regular files (modes 100644, 100755, 100664) under
// treeOID recursively; gitlinks and symlinks are skipped. Entry names ".",
// "..", ".git" or containing "/", "\" or NUL are errors. Every subtree must
// be present in s.
func (s *Store) WalkFiles(treeOID string) ([]File, error) {
	return nil, ErrNotImplemented
}

// HashObject returns the hex SHA-1 of "<type> <len>\x00<data>".
func HashObject(t ObjectType, data []byte) string {
	panic("gitpack: not implemented")
}

// VerifyBlob reports an error if data does not hash to oid as a blob.
func VerifyBlob(oid string, data []byte) error {
	return ErrNotImplemented
}
