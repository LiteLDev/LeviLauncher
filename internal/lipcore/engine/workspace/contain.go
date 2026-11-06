package workspace

import (
	"errors"
	"fmt"
	"strings"
)

// This file keeps package files inside the workspace directory. lip writes
// and deletes files wherever an asset key, a placement dest or a
// tooth_lock.json entry points; LeviLauncher deliberately refuses paths
// that leave Dir instead.

// ErrUnsafePath is wrapped by the errors reporting a package path that
// would be written or deleted outside the workspace directory.
var ErrUnsafePath = errors.New("unsafe package path")

type unsafePathError struct{ reason string }

func (e *unsafePathError) Error() string { return e.reason }
func (e *unsafePathError) Unwrap() error { return ErrUnsafePath }

func unsafePath(format string, args ...any) error {
	return &unsafePathError{reason: fmt.Sprintf(format, args...)}
}

// reservedDeviceNames are the Win32 device names that open a device instead
// of a file in any directory, also with an extension ("NUL.txt") or
// trailing spaces ("NUL .txt").
var reservedDeviceNames = func() map[string]bool {
	m := map[string]bool{"CON": true, "PRN": true, "AUX": true, "NUL": true, "CONIN$": true, "CONOUT$": true}
	for _, d := range []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "¹", "²", "³"} {
		m["COM"+d] = true
		m["LPT"+d] = true
	}
	return m
}()

func isReservedDeviceName(segment string) bool {
	stem, _, _ := strings.Cut(segment, ".")
	return reservedDeviceNames[strings.ToUpper(strings.TrimRight(stem, " "))]
}

// containment checks paths against one workspace directory, remembering
// the existing paths whose links were already checked, so a package's
// files sharing directories cost one probe per directory.
type containment struct {
	root, prefix string
	// realRoot is root with links resolved; computed on the first link.
	realRoot string
	checked  map[string]bool
}

func (w *Workspace) containment() *containment {
	root := getFullPath(w.Dir, ".")
	prefix := root
	if !strings.HasSuffix(prefix, dirSep()) {
		prefix += dirSep()
	}
	return &containment{root: root, prefix: prefix, checked: map[string]bool{}}
}

// check returns an error wrapping ErrUnsafePath when full (a path from
// FullPath) must not be written (followLast) or deleted by a package:
//   - it is Dir itself or lexically outside Dir (UNC and \\?\ or \\.\
//     device paths never match Dir's prefix); the prefix test ignores case
//     where paths do;
//   - it contains a NUL character or, on Windows, a "/": FullPath could
//     not normalize it (GetFullPathNameW fails on NUL and beyond 32767
//     characters and then returns its input unchanged), so its "." and
//     ".." segments are not collapsed and os.MkdirAll would still resolve
//     them lexically;
//   - on Windows, a segment names an alternate data stream ("file:stream")
//     or a reserved device (CON, NUL, COM1, ...), or ends in a dot or
//     space;
//   - an existing symbolic link or junction on the way (and, when
//     followLast, full itself, since creating a file follows it) resolves
//     outside Dir or cannot be resolved.
//
// FullPath removes every trailing dot and space of the last segment, and a
// single trailing dot of the others (as lip does), so most such names are
// written at the checked path. What it keeps ("plugins /x.dll" or
// "plugins../x.dll") Win32 could not create as a directory either, so
// rejecting it loses no working package.
func (c *containment) check(full string, followLast bool) error {
	if strings.ContainsRune(full, 0) {
		return unsafePath("it contains a NUL character")
	}
	if isWindows && strings.ContainsRune(full, '/') {
		return unsafePath("it cannot be normalized (too long, or a device path)")
	}
	if samePath(full, c.root) {
		return unsafePath("it is the workspace directory '%s' itself", c.root)
	}
	rest, ok := cutPathPrefix(full, c.prefix)
	if !ok {
		// FullPath turns a device name into \\.\NUL in any directory.
		if dev, isDev := strings.CutPrefix(full, `\\.\`); isWindows && isDev && isReservedDeviceName(dev) {
			return unsafePath("'%s' is a reserved device name", dev)
		}
		return unsafePath("it is outside the workspace directory '%s'", c.root)
	}
	// rest is not empty: samePath caught Dir with trailing separators.
	// "/" was rejected above on Windows; splitting on it too is only
	// defence in depth.
	segments := strings.FieldsFunc(rest, isDirSep)
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
			if isReservedDeviceName(s) {
				return unsafePath("'%s' is a reserved device name", s)
			}
			// What FullPath left: see above. This includes names made of
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
			if c.realRoot == "" {
				c.realRoot = c.root
				if r, err := realPath(c.root); err == nil {
					c.realRoot = r
				}
			}
			if !pathWithin(target, c.realRoot) {
				return unsafePath("it goes through the link '%s', which points outside the workspace directory (to '%s')", p, target)
			}
		}
		c.checked[p] = true
	}
	return nil
}

// cutPathPrefix is strings.CutPrefix, ignoring case where paths do.
func cutPathPrefix(s, prefix string) (string, bool) {
	if pathIgnoreCase {
		return cutPrefixFoldOrdinal(s, prefix)
	}
	return strings.CutPrefix(s, prefix)
}

// samePath reports whether the full paths a and b are equal, ignoring
// trailing separators and, where paths do, case.
func samePath(a, b string) bool {
	a = strings.TrimRight(a, dirSep())
	b = strings.TrimRight(b, dirSep())
	return a == b || pathIgnoreCase && equalFoldOrdinal(a, b)
}

// pathWithin reports whether the full path p is root or inside it.
func pathWithin(p, root string) bool {
	if samePath(p, root) {
		return true
	}
	_, ok := cutPathPrefix(p, strings.TrimRight(root, dirSep())+dirSep())
	return ok
}
