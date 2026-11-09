package pkgfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
	"github.com/liteldev/LeviLauncher/internal/lipcore/ref"
	"github.com/liteldev/LeviLauncher/internal/lipcore/txn"
)

// This file ports LockStore (src/Lip.Core/Mcmod/Lock/LockStore.cs): the
// state file <root>/.lip/state.json (protocol/workspace.md §7.1,
// state.schema.json), a JSON file read and written through package doc.
// A workspace without the state file has no installed packages, unless
// it has a tooth_lock.json (protocol/compatibility.md §3), which the first
// state file written replaces (CommitStaged).
//
// Writing goes through a staged file: StageWorkspace writes
// .lip/state.json.tmp, CommitStaged renames it over .lip/state.json
// (the commit point of an operation), DiscardStaged deletes it. Inside a
// transaction, pass StagedHooks to txn.Options and RecoverHooks to
// txn.RecoverWith, so that recovery commits the staged file as lip does,
// tooth_lock.json deletion included.

// ToothLockFile is the tooth_lock.json of lip v0.x at the root of a workspace
// (ToothLockMigration.FileName).
const ToothLockFile = "tooth_lock.json"

// LockFormatError is LockFormatException: a state file exists but cannot
// be read. Message is lip's message exactly; Err is what is wrong.
// OtherFormat marks a state file written by a different version of lip:
// its "format" is an integer other than 4, or its top level uses the keys
// of lip before v0.40.
type LockFormatError struct {
	Message     string
	OtherFormat bool
	Err         error
}

func (e *LockFormatError) Error() string { return e.Message }
func (e *LockFormatError) Unwrap() error { return e.Err }

// ErrInvalidWorkspace is the cause of the error StageWorkspace returns for a
// state file that is not valid: a bug in the caller (lip's
// ArgumentException).
var ErrInvalidWorkspace = errors.New("pkgfile: invalid state file")

// invalidWorkspaceError is the ArgumentException of LockStore.Stage.
type invalidWorkspaceError struct{ err error }

func (e *invalidWorkspaceError) Error() string {
	return "lip was about to write an invalid state file, and did not write it. This is a bug in" +
		" lip. (Parameter 'lockFile')"
}
func (e *invalidWorkspaceError) Unwrap() []error { return []error{e.err, ErrInvalidWorkspace} }

var commitRegex = regexp.MustCompile(`^[0-9a-f]{40}$`)

// WorkspaceStore reads and writes state files on a file system. The zero
// value uses the operating system.
type WorkspaceStore struct {
	// FS is the file system (txn.OS when nil).
	FS txn.FS
}

func (s WorkspaceStore) fs() txn.FS {
	if s.FS == nil {
		return txn.OS
	}
	return s.FS
}

func (s WorkspaceStore) fileExists(path string) bool {
	info, err := s.fs().Stat(path)
	return err == nil && !info.IsDir()
}

// normalizeRoot is LockStore.NormalizeRoot: the full path without a trailing
// separator.
func normalizeRoot(root string) (string, error) { return filepath.Abs(root) }

// WorkspacePath is the path of the state file, <root>/.lip/state.json.
func WorkspacePath(root string) (string, error) {
	r, err := normalizeRoot(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(r, txn.LipDirectory, txn.WorkspaceFile), nil
}

// LoadWorkspace is LoadWorkspace on the operating system.
func LoadWorkspace(root string) (*pkgmodel.LockFile, bool, error) {
	return WorkspaceStore{}.LoadWorkspace(root)
}

// SaveWorkspace is SaveWorkspace on the operating system.
func SaveWorkspace(root string, lf *pkgmodel.LockFile) error {
	return WorkspaceStore{}.SaveWorkspace(root, lf)
}

// StageWorkspace is StageWorkspace on the operating system.
func StageWorkspace(root string, lf *pkgmodel.LockFile) error {
	return WorkspaceStore{}.StageWorkspace(root, lf)
}

// CommitStaged is CommitStaged on the operating system.
func CommitStaged(root string) error { return WorkspaceStore{}.CommitStaged(root) }

// DiscardStaged is DiscardStaged on the operating system.
func DiscardStaged(root string) error { return WorkspaceStore{}.DiscardStaged(root) }

// RecoverHooks is RecoverHooks on the operating system.
func RecoverHooks(root string) txn.RecoverOptions { return WorkspaceStore{}.RecoverHooks(root) }

// LoadWorkspace is LockStore.Load: the state file of root, and whether it
// exists (nil, false, nil when it does not). A state file that exists but
// cannot be read is a *txn.Error; one that is not valid a *LockFormatError.
// Manifests are kept as read (*doc.Object).
func (s WorkspaceStore) LoadWorkspace(root string) (*pkgmodel.LockFile, bool, error) {
	path, err := WorkspacePath(root)
	if err != nil {
		return nil, false, err
	}
	if !s.fileExists(path) {
		return nil, false, nil
	}
	data, err := s.fs().ReadFile(path)
	if err != nil {
		return nil, false, &txn.Error{Msg: fmt.Sprintf("Cannot read the state file '%s'.", path), Err: err}
	}
	lf, err := ParseWorkspace(data, path)
	if err != nil {
		return nil, false, err
	}
	return lf, true, nil
}

// ParseWorkspace is LockStore.Parse: reads the bytes of a state file;
// path names it in messages. The error is a *LockFormatError.
func ParseWorkspace(data []byte, path string) (*pkgmodel.LockFile, error) {
	otherFormat := false
	lf, err := func() (*pkgmodel.LockFile, error) {
		v, err := doc.ParseJSONFile(data)
		if err != nil {
			return nil, err
		}
		root, ok := v.(*doc.Object)
		if !ok {
			return nil, lockErr("The file is not a JSON object.", nil)
		}
		format, ok := Format(root)
		if !ok || format != pkgmodel.Format {
			otherFormat = ok
			is := "missing or not a number"
			if ok {
				is = fmt.Sprint(format)
			}
			return nil, lockErr(fmt.Sprintf("\"%s\" is %s, but this version of lip reads only %d. If another"+
				" version of lip wrote the file, use that version.", pkgmodel.FormatKey, is, pkgmodel.Format), nil)
		}
		if UsesOldKeys(root) {
			otherFormat = true
			return nil, lockErr("The file "+OldKeysMessage+".", nil)
		}

		// "commit" is omitted, not null, for a package that is not from a remote
		// repository.
		if packages, ok := root.Get("packages"); ok {
			if arr, ok := packages.([]any); ok {
				for i, p := range arr {
					o, ok := p.(*doc.Object)
					if !ok {
						continue
					}
					if c, ok := o.Get("commit"); ok {
						if _, ok := c.(string); !ok {
							return nil, lockErr(fmt.Sprintf("packages[%d].commit is %s, but it must be a string of"+
								" 40 lowercase hexadecimal digits, or be left out.", i, valueKind(c)), nil)
						}
					}
				}
			}
		}

		raw, err := deserializeLock(root)
		if err != nil {
			return nil, err
		}
		if err := validateRead(raw); err != nil {
			return nil, err
		}
		return raw.toModel(), nil
	}()
	if err == nil {
		return lf, nil
	}
	if otherFormat {
		return nil, &LockFormatError{OtherFormat: true, Err: err, Message: fmt.Sprintf("The state file"+
			" '%s' was written by a different version of lip, so this lip cannot tell which packages"+
			" are installed.", path)}
	}
	return nil, &LockFormatError{Err: err, Message: fmt.Sprintf("The state file '%s' is invalid, so lip"+
		" cannot tell which packages are installed. If you edited it by hand, undo the change; if you"+
		" have a copy of the file, restore it.", path)}
}

func lockErr(msg string, inner error) error { return &LockFormatError{Message: msg, Err: inner} }

// valueKind is JsonValueKind.ToString().ToLowerInvariant().
func valueKind(v any) string {
	switch t := v.(type) {
	case *doc.Object:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case json.Number:
		return "number"
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return "null"
	}
}

func (lf *lockFile) toModel() *pkgmodel.LockFile {
	out := &pkgmodel.LockFile{Format: pkgmodel.Format, Side: lf.side, Platform: lf.platform,
		Packages: make([]pkgmodel.LockedPackage, len(lf.packages))}
	for i, p := range lf.packages {
		files := make([]string, len(p.files))
		for j, f := range p.files {
			files[j] = *f
		}
		lp := pkgmodel.LockedPackage{Manifest: p.manifest, Variant: p.variant, Explicit: p.explicit,
			Ref: p.ref, Files: files}
		if p.commit != nil {
			lp.Commit = *p.commit
		}
		out.Packages[i] = lp
	}
	return out
}

// validateRead is LockStore.Validate on a deserialized state file, which
// may hold null packages and null files.
func validateRead(lf *lockFile) error {
	if err := validateEnvironment(lf.side, lf.platform); err != nil {
		return err
	}
	for i, p := range lf.packages {
		if p == nil {
			return lockErr(fmt.Sprintf("packages[%d] is null, but it must be an object.", i), nil)
		}
		at := fmt.Sprintf("packages[%d]", i)
		for j, f := range p.files {
			if f == nil {
				return lockErr(fmt.Sprintf("%s.files[%d] is null, but it must be a workspace-relative path.", at, j), nil)
			}
		}
		if err := validatePackage(at, p.manifest, p.variant, p.commit); err != nil {
			return err
		}
	}
	return nil
}

func validateEnvironment(side, platform string) error {
	if !IsSide(side) {
		return lockErr(fmt.Sprintf("\"side\" is \"%s\", but it must be \"server\" or \"client\".", side), nil)
	}
	if platform == "" {
		return lockErr("\"platform\" is missing or empty, but it must be a runtime identifier such as"+
			" \"win-x64\".", nil)
	}
	return nil
}

func validatePackage(at string, manifest any, variant string, commit *string) error {
	if variant != "" && !ref.IsValidVariant(variant) {
		return lockErr(fmt.Sprintf("%s.variant is \"%s\", which is not a valid variant name.", at, variant), nil)
	}
	if _, ok := manifest.(*doc.Object); !ok {
		return lockErr(fmt.Sprintf("%s.manifest is %s, but it must be the package file of the package, a"+
			" mapping.", at, valueKind(manifest)), nil)
	}
	if _, err := Parse(manifest); err != nil {
		return lockErr(fmt.Sprintf("%s.manifest, the package file of %s, is not a valid package file.", at,
			describePackage(manifest, variant)), err)
	}
	if commit != nil && !commitRegex.MatchString(*commit) {
		return lockErr(fmt.Sprintf("%s.commit is \"%s\", but it must be 40 lowercase hexadecimal digits.", at,
			*commit), nil)
	}
	return nil
}

// ValidateWorkspace is LockStore.Validate: checks what state.schema.json
// requires of a state file beyond its shape. The error is a
// *LockFormatError. A nil Packages or Files is an empty array; a Commit of ""
// is an absent one.
func ValidateWorkspace(lf *pkgmodel.LockFile) error {
	if lf.Format != pkgmodel.Format {
		return lockErr(fmt.Sprintf("\"%s\" is %d, but it must be %d.", pkgmodel.FormatKey, lf.Format,
			pkgmodel.Format), nil)
	}
	if err := validateEnvironment(lf.Side, lf.Platform); err != nil {
		return err
	}
	for i := range lf.Packages {
		p := &lf.Packages[i]
		var commit *string
		if p.Commit != "" {
			commit = &p.Commit
		}
		if err := validatePackage(fmt.Sprintf("packages[%d]", i), p.Manifest, p.Variant, commit); err != nil {
			return err
		}
	}
	return nil
}

// stringMember is LockedPackage.StringMember.
func stringMember(manifest any, name string) string {
	if o, ok := manifest.(*doc.Object); ok {
		if v, ok := o.Get(name); ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

// describePackage is LockStore.DescribePackage: the identity and version of a
// package for messages, as far as its manifest tells.
func describePackage(manifest any, variant string) string {
	id := stringMember(manifest, "id")
	if id == "" {
		return "a package"
	}
	return strings.TrimRightFunc(ref.Identity(id, variant)+" "+stringMember(manifest, "version"), unicode.IsSpace)
}

// MarshalWorkspace is the bytes LockStore.Stage writes for a state file:
// format, side, platform and packages; for each package its manifest (as
// it is), variant, explicit, ref, commit (left out when "") and files.
func MarshalWorkspace(lf *pkgmodel.LockFile) []byte {
	root := doc.NewObject()
	root.Set(pkgmodel.FormatKey, json.Number(fmt.Sprint(lf.Format)))
	root.Set("side", lf.Side)
	root.Set("platform", lf.Platform)
	packages := make([]any, len(lf.Packages))
	for i, p := range lf.Packages {
		o := doc.NewObject()
		o.Set("manifest", p.Manifest)
		o.Set("variant", p.Variant)
		o.Set("explicit", p.Explicit)
		o.Set("ref", p.Ref)
		if p.Commit != "" {
			o.Set("commit", p.Commit)
		}
		files := make([]any, len(p.Files))
		for j, f := range p.Files {
			files[j] = f
		}
		o.Set("files", files)
		packages[i] = o
	}
	root.Set("packages", packages)
	return doc.MarshalJSONFile(root)
}

// SaveWorkspace is LockStore.Save: StageWorkspace, then CommitStaged. It is
// for writes outside a transaction.
func (s WorkspaceStore) SaveWorkspace(root string, lf *pkgmodel.LockFile) error {
	if err := s.StageWorkspace(root, lf); err != nil {
		return err
	}
	return s.CommitStaged(root)
}

// StageWorkspace is LockStore.Stage: writes the state file to
// <root>/.lip/state.json.tmp, replacing an earlier staged one, creating
// .lip (hidden) when needed. A state file that is not valid is not
// written: the error is a bug of the caller (errors.Is ErrInvalidWorkspace).
func (s WorkspaceStore) StageWorkspace(root string, lf *pkgmodel.LockFile) error {
	if err := ValidateWorkspace(lf); err != nil {
		return &invalidWorkspaceError{err}
	}
	r, err := normalizeRoot(root)
	if err != nil {
		return err
	}
	if err := txn.EnsureLipDirFS(s.fs(), r); err != nil {
		return err
	}
	temp := filepath.Join(r, txn.LipDirectory, txn.StagedFile)
	if err := s.fs().WriteFile(temp, MarshalWorkspace(lf)); err != nil {
		if s.fileExists(temp) {
			// A staged state file is never read, and the next save replaces it.
			_ = s.fs().Remove(temp)
		}
		return &txn.Error{Msg: fmt.Sprintf("Cannot write the new state file '%s'.", temp), Err: err}
	}
	return nil
}

// CommitStaged is LockStore.CommitStaged: moves the staged state file into
// place in one rename, the point at which an operation commits. It does
// nothing when none is staged, and fails only when the file was not moved.
//
// The first state file of a workspace whose state file was derived
// from its tooth_lock.json replaces it: when no state file existed and
// tooth_lock.json is one protocol/compatibility.md §3 reads, tooth_lock.json
// is deleted (best effort) once the state file is in place.
func (s WorkspaceStore) CommitStaged(root string) error {
	r, err := normalizeRoot(root)
	if err != nil {
		return err
	}
	path := filepath.Join(r, txn.LipDirectory, txn.WorkspaceFile)
	temp := filepath.Join(r, txn.LipDirectory, txn.StagedFile)
	if !s.fileExists(temp) {
		return nil
	}
	migrated := !s.fileExists(path) && s.toothLockReadable(r)
	if err := txn.CommitStaged(s.fs(), r); err != nil {
		return err
	}
	if migrated {
		// The state file is in place; what follows must not fail the
		// operation. A tooth_lock.json is not read once the state file
		// exists.
		_ = s.fs().Remove(filepath.Join(r, ToothLockFile))
	}
	return nil
}

// DiscardStaged is LockStore.DiscardStaged: deletes the staged workspace
// file, if any.
func (s WorkspaceStore) DiscardStaged(root string) error {
	r, err := normalizeRoot(root)
	if err != nil {
		return err
	}
	return txn.DiscardStaged(s.fs(), r)
}

// StagedHooks are CommitStaged and DiscardStaged of root, for
// txn.Options.CommitStaged and txn.Options.DiscardStaged.
func (s WorkspaceStore) StagedHooks(root string) (commit, discard func() error) {
	return func() error { return s.CommitStaged(root) }, func() error { return s.DiscardStaged(root) }
}

// RecoverHooks are the txn.RecoverOptions with which recovery commits or
// discards the staged state file as lip does (CommitStaged, with its
// tooth_lock.json deletion, and DiscardStaged), on the store's file system.
func (s WorkspaceStore) RecoverHooks(root string) txn.RecoverOptions {
	commit, discard := s.StagedHooks(root)
	return txn.RecoverOptions{FS: s.FS, CommitLock: commit, DiscardLock: discard}
}

// toothLockReadable is ToothLockMigration.Read(fs, root) is not null
// (src/Lip.Core/Mcmod/Compat/ToothLockMigration.cs): <root>/tooth_lock.json
// is strict JSON, an object with format_version 3 and an array "packages" of
// objects with "manifest" (an object with format_version 1, 2 or 3),
// "variant" (a string), "locked" (a boolean) and "files" (an array of
// strings). A missing or unreadable file is not.
func (s WorkspaceStore) toothLockReadable(root string) bool {
	path := filepath.Join(root, ToothLockFile)
	if !s.fileExists(path) {
		return false
	}
	data, err := s.fs().ReadFile(path)
	if err != nil {
		return false
	}
	v, err := doc.ParseJSON(data, ToothLockFile)
	if err != nil {
		return false
	}
	if f, ok := IntegerMember(v, "format_version"); !ok || f != 3 {
		return false
	}
	packages, ok := v.(*doc.Object).Get("packages")
	if !ok {
		return false
	}
	arr, ok := packages.([]any)
	if !ok {
		return false
	}
	for _, p := range arr {
		o, ok := p.(*doc.Object)
		if !ok {
			return false
		}
		manifest, ok := o.Get("manifest")
		if !ok {
			return false
		}
		if f, ok := IntegerMember(manifest, "format_version"); !ok || f < 1 || f > 3 {
			return false
		}
		if variant, ok := o.Get("variant"); !ok {
			return false
		} else if _, ok := variant.(string); !ok {
			return false
		}
		if locked, ok := o.Get("locked"); !ok {
			return false
		} else if _, ok := locked.(bool); !ok {
			return false
		}
		files, ok := o.Get("files")
		if !ok {
			return false
		}
		list, ok := files.([]any)
		if !ok {
			return false
		}
		for _, f := range list {
			if _, ok := f.(string); !ok {
				return false
			}
		}
	}
	return true
}
