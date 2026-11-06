package workspace

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// This file keeps package files inside the workspace directory. lip writes
// and deletes files wherever an asset key, a placement dest or a
// tooth_lock.json entry points; LeviLauncher deliberately refuses paths
// that leave Dir instead.
//
// No decision here folds case. Whether two names differing in case are the
// same file depends on the file system (NTFS directories can be marked
// case-sensitive, \\wsl$ shares are) and on its own case table, which is
// not Unicode's (unicode.ToUpper maps U+017F to 'S', NTFS does not). A
// path is inside Dir when it starts with Dir's full path exactly or, for a
// path read from tooth_lock.json or a link target, when the file system
// says that one of its existing ancestors is Dir.

// ErrUnsafePath is wrapped by the errors reporting a package path that
// would be written or deleted outside the workspace directory.
var ErrUnsafePath = errors.New("unsafe package path")

type unsafePathError struct{ reason string }

func (e *unsafePathError) Error() string { return e.reason }
func (e *unsafePathError) Unwrap() error { return ErrUnsafePath }

func unsafePath(format string, args ...any) error {
	return &unsafePathError{reason: fmt.Sprintf(format, args...)}
}

// containment checks paths against one workspace directory, remembering
// the existing paths whose links were already checked, so a package's
// files sharing directories cost one probe per directory.
type containment struct {
	// root is Dir's full path; prefix is root ending in a separator.
	root, prefix string
	// longPrefix is prefix in the form FullPath gives it in a path
	// containing '~': existing components expanded from 8.3 names and in
	// their on-disk case (GetLongPathNameW). It equals prefix elsewhere.
	longPrefix string
	// realRoot is root with links resolved; computed on the first link.
	realRoot string
	// rootInfo is Dir's file info and rootErr the error statting it;
	// computed on first use.
	rootInfo os.FileInfo
	rootErr  error
	// isRootDir caches, per path statted by belowRoot, whether it is Dir.
	isRootDir map[string]bool
	checked   map[string]bool
}

func (w *Workspace) containment() *containment {
	root := getFullPath(w.Dir, ".")
	if trimmed := strings.TrimRight(root, dirSep()); len(trimmed) > pathRootLength(root) {
		root = trimmed
	}
	return &containment{
		root:       root,
		prefix:     withSep(root),
		longPrefix: withSep(expandShortName(root)),
		checked:    map[string]bool{},
	}
}

func withSep(p string) string {
	if strings.HasSuffix(p, dirSep()) {
		return p
	}
	return p + dirSep()
}

// cut returns full without Dir's full path and the separators after it,
// comparing exactly (ordinal, case-sensitive). A target built from Dir
// keeps Dir's literal prefix, since FullPath does not change its case
// (except through longPrefix).
func (c *containment) cut(full string) (string, bool) {
	if c.isRoot(full) {
		return "", true
	}
	rest, ok := strings.CutPrefix(full, c.prefix)
	if !ok {
		rest, ok = strings.CutPrefix(full, c.longPrefix)
	}
	return strings.TrimLeft(rest, dirSep()), ok
}

// isRoot reports whether full is Dir itself, maybe with trailing
// separators, comparing exactly.
func (c *containment) isRoot(full string) bool {
	p := withSep(strings.TrimRight(full, dirSep()))
	return p == c.prefix || p == c.longPrefix
}

// checkNormalized rejects a path FullPath could not normalize: it contains
// a NUL character or, on Windows, a "/" (GetFullPathNameW fails on NUL and
// beyond 32767 characters and then returns its input unchanged), so its
// "." and ".." segments are not collapsed and os.MkdirAll would still
// resolve them lexically.
func checkNormalized(full string) error {
	if strings.ContainsRune(full, 0) {
		return unsafePath("it contains a NUL character")
	}
	if isWindows && strings.ContainsRune(full, '/') {
		return unsafePath("it cannot be normalized (too long, or a device path)")
	}
	return nil
}

// isDevicePath reports whether the full path p is in the \\.\ or \\?\
// device form.
func isDevicePath(p string) bool {
	return isWindows && (strings.HasPrefix(p, `\\.\`) || strings.HasPrefix(p, `\\?\`))
}

// check returns an error wrapping ErrUnsafePath when full (a path from
// FullPath) must not be written (followLast) or deleted by a package:
//   - FullPath could not normalize it (see checkNormalized);
//   - it is Dir itself or does not start with Dir's full path exactly.
//     UNC and device paths never do. Neither does a path that leaves Dir
//     and comes back in another case, which may name another directory;
//     refusing it loses no working package, since a target that stays in
//     Dir is Dir's path followed by the key or dest;
//   - on Windows, a segment names an alternate data stream ("file:stream")
//     or ends in a dot or space;
//   - an existing symbolic link or junction on the way (and, when
//     followLast, full itself, since creating a file follows it) resolves
//     outside Dir or cannot be resolved.
//
// A reserved device name (CON, NUL.txt, COM1, ...) is refused only where
// FullPath turns it into a device path (\\.\NUL, on Windows before 11),
// which fails the prefix test; Windows 11 opens an ordinary file by that
// name in a directory, as lip would install it.
//
// FullPath removes every trailing dot and space of the last segment, and a
// single trailing dot of the others (as lip does), so most such names are
// written at the checked path. What it keeps ("plugins /x.dll" or
// "plugins../x.dll") Win32 could not create as a directory either, so
// rejecting it loses no working package.
func (c *containment) check(full string, followLast bool) error {
	if err := checkNormalized(full); err != nil {
		return err
	}
	rest, ok := c.cut(full)
	if !ok {
		if isDevicePath(full) {
			return unsafePath("it is the device path '%s', outside the workspace directory '%s'", full, c.root)
		}
		return unsafePath("it is outside the workspace directory '%s'", c.root)
	}
	// checkRest rejects an empty rest: Dir itself.
	return c.checkRest(rest, followLast)
}

// checkRecorded is check for a path read from tooth_lock.json, to be
// deleted. lip records the FullPath of each file under its own working
// directory, whose case may differ from Dir's, so a path that does not
// start with Dir's full path exactly is also accepted when the file system
// says that one of its existing ancestors is Dir (os.SameFile). It returns
// the path to delete: Dir's full path followed by the rest of full.
func (c *containment) checkRecorded(full string) (string, error) {
	if err := checkNormalized(full); err != nil {
		return "", err
	}
	rest, ok := c.cut(full)
	if !ok {
		if rest, ok = c.belowRoot(full); !ok {
			return "", c.check(full, false)
		}
	}
	// checkRest rejects an empty rest: Dir itself.
	if err := c.checkRest(rest, false); err != nil {
		return "", err
	}
	return c.prefix + rest, nil
}

// belowRoot walks the existing ancestors of the full path p, p included,
// and when one of them is Dir according to the file system (os.SameFile)
// returns what follows it in p, without leading separators. Device paths
// are not walked (Win32 does not normalize them), nor are paths on another
// volume than Dir's: statting \\server\share would connect to that server.
// The volume test folds case only to decide whether to ask the file
// system; os.SameFile decides containment.
func (c *containment) belowRoot(p string) (string, bool) {
	if c.rootInfo == nil && c.rootErr == nil {
		c.rootInfo, c.rootErr = os.Stat(c.root)
		c.isRootDir = map[string]bool{}
	}
	rootLen := pathRootLength(p)
	if c.rootErr != nil || rootLen == 0 || isDevicePath(p) ||
		!strings.EqualFold(strings.TrimRight(p[:rootLen], dirSep()), strings.TrimRight(c.root[:pathRootLength(c.root)], dirSep())) {
		return "", false
	}
	for a := p; ; {
		isRoot, known := c.isRootDir[a]
		if !known {
			fi, err := os.Stat(a)
			isRoot = err == nil && os.SameFile(fi, c.rootInfo)
			c.isRootDir[a] = isRoot
		}
		if isRoot {
			return strings.TrimLeft(p[len(a):], dirSep()), true
		}
		if len(a) <= rootLen {
			return "", false
		}
		a = a[:max(strings.LastIndex(a, dirSep()), rootLen)]
	}
}

// checkRest checks rest, the part of a full path below Dir (see check).
func (c *containment) checkRest(rest string, followLast bool) error {
	// "/" was rejected on Windows; splitting on it too is only defence in
	// depth.
	segments := strings.FieldsFunc(rest, isDirSep)
	if len(segments) == 0 {
		return unsafePath("it is the workspace directory '%s' itself", c.root)
	}
	for _, s := range segments {
		// FullPath collapses these; guard against a path it could not
		// normalize.
		if s == "." || s == ".." {
			return unsafePath("it is outside the workspace directory '%s'", c.root)
		}
		if isWindows {
			if strings.ContainsRune(s, ':') {
				return unsafePath("'%s' names an alternate data stream", s)
			}
			// What FullPath left: see check. This includes names made of
			// only dots and spaces (".. ", "... ").
			if strings.HasSuffix(s, ".") || strings.HasSuffix(s, " ") {
				return unsafePath("'%s' ends in a dot or space", s)
			}
		}
	}

	if !followLast {
		segments = segments[:len(segments)-1]
	}
	for i := range segments {
		p := c.prefix + strings.Join(segments[:i+1], dirSep())
		if c.checked[p] {
			continue
		}
		link, exists := isLink(p)
		if !exists {
			// Nothing deeper exists either.
			return nil
		}
		if link {
			target, err := realPath(p)
			if err != nil {
				return unsafePath("it goes through the link '%s', which cannot be resolved: %v", p, err)
			}
			if !c.linkTargetWithin(target) {
				return unsafePath("it goes through the link '%s', which points outside the workspace directory (to '%s')", p, target)
			}
		}
		c.checked[p] = true
	}
	return nil
}

// linkTargetWithin reports whether target, an existing full path with its
// links resolved, is Dir or inside it: it starts with Dir's resolved path
// exactly, or the file system says that one of its ancestors is Dir.
func (c *containment) linkTargetWithin(target string) bool {
	if c.realRoot == "" {
		c.realRoot = c.root
		if r, err := realPath(c.root); err == nil {
			c.realRoot = r
		}
	}
	if strings.HasPrefix(withSep(target), withSep(c.realRoot)) {
		return true
	}
	rest, ok := c.belowRoot(target)
	if !ok {
		return false
	}
	for _, s := range strings.FieldsFunc(rest, isDirSep) {
		if s == "." || s == ".." {
			return false
		}
	}
	return true
}
