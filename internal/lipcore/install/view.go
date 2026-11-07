package install

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/liteldev/LeviLauncher/internal/lipcore/txn"
)

// View is WorkspaceView.cs: the workspace as it will be when the steps
// planned so far have run, for planning a whole operation before changing
// anything (protocol/workspace.md §6): the file system, overlaid with the
// files the planned steps write and delete and with the layout the operation
// establishes first.
//
// Paths are absolute. A View is not safe for concurrent use.
type View struct {
	files       map[string]bool // by pathKey
	directories map[string]bool // by pathKey

	// The paths of files and directories by the pathKey of their parent
	// directory, each by its own pathKey.
	changed map[string]map[string]string

	// The entries of directories as they are now; the workspace does not
	// change while an operation is planned.
	listings map[string][]Entry
}

// Entry is WorkspaceView.Entry: an entry of a directory, its absolute path,
// and whether it is a directory or a link (a reparse point on Windows).
type Entry struct {
	FullName    string
	IsDirectory bool
	IsLink      bool
}

// NewView is the workspace as it is, with the layout the plan establishes
// (none when layout is nil).
func NewView(layout *LayoutPlan) *View {
	v := &View{
		files:       map[string]bool{},
		directories: map[string]bool{},
		changed:     map[string]map[string]string{},
		listings:    map[string][]Entry{},
	}
	if layout != nil && layout.CreateMods {
		v.setDirectory(trimEndingSeparator(layout.Mods), true)
	}
	return v
}

// FileExists is File.Exists over the view.
func (v *View) FileExists(path string) bool {
	path = trimEndingSeparator(path)
	if exists, ok := v.files[pathKey(path)]; ok {
		return exists
	}
	return fileExists(path)
}

// DirectoryExists is Directory.Exists over the view.
func (v *View) DirectoryExists(path string) bool {
	path = trimEndingSeparator(path)
	if exists, ok := v.directories[pathKey(path)]; ok {
		return exists
	}
	return dirExists(path)
}

// Enumerate returns the entries of a directory (none when it does not
// exist), in ordinal order of their paths. Listing a directory of the file
// system fails with a *txn.Error.
func (v *View) Enumerate(directory string) ([]Entry, error) {
	directory = trimEndingSeparator(directory)
	if !v.DirectoryExists(directory) {
		return nil, nil
	}

	listing, err := v.listing(directory)
	if err != nil {
		return nil, err
	}
	entries := map[string]Entry{}
	for _, e := range listing {
		path := filepath.Join(directory, filepath.Base(e.FullName))
		e.FullName = path
		entries[pathKey(path)] = e
	}

	for key, path := range v.changed[pathKey(directory)] {
		if exists, ok := v.directories[key]; ok {
			if exists {
				if _, present := entries[key]; !present {
					entries[key] = Entry{FullName: path, IsDirectory: true}
				}
			} else {
				delete(entries, key)
			}
		}
		if exists, ok := v.files[key]; ok {
			if exists {
				entries[key] = Entry{FullName: path}
			} else {
				delete(entries, key)
			}
		}
	}

	result := make([]Entry, 0, len(entries))
	for _, e := range entries {
		result = append(result, e)
	}
	sort.Slice(result, func(i, j int) bool { return compareOrdinal(result[i].FullName, result[j].FullName) < 0 })
	return result, nil
}

// IsEmpty reports whether a directory has no entries in the view.
// It is !Enumerate(directory).Any() without building the listing: an entry
// is present unless the plan deleted it, and a planned entry is present
// unless the plan deleted it again (a planned file wins over a planned
// directory of the same path, as in Enumerate).
func (v *View) IsEmpty(directory string) (bool, error) {
	directory = trimEndingSeparator(directory)
	if !v.DirectoryExists(directory) {
		return true, nil
	}
	listing, err := v.listing(directory)
	if err != nil {
		return false, err
	}
	present := func(key string, listed bool) bool {
		p := listed
		if exists, ok := v.directories[key]; ok {
			p = exists
		}
		if exists, ok := v.files[key]; ok {
			p = exists
		}
		return p
	}
	for _, e := range listing {
		if present(pathKey(filepath.Join(directory, filepath.Base(e.FullName))), true) {
			return false, nil
		}
	}
	for key := range v.changed[pathKey(directory)] {
		if present(key, false) {
			return false, nil
		}
	}
	return true, nil
}

// AddFile records a planned write: the file exists, and so do its
// ancestors.
func (v *View) AddFile(path string) {
	path = trimEndingSeparator(path)
	v.setFile(path, true)
	for dir, ok := parentDir(path); ok && !v.DirectoryExists(dir); dir, ok = parentDir(dir) {
		v.setDirectory(dir, true)
	}
}

// DeleteFile records a planned deletion of a file.
func (v *View) DeleteFile(path string) { v.setFile(trimEndingSeparator(path), false) }

// DeleteDirectory records a planned deletion of an empty directory.
func (v *View) DeleteDirectory(path string) { v.setDirectory(trimEndingSeparator(path), false) }

func (v *View) setFile(path string, exists bool) {
	v.files[pathKey(path)] = exists
	v.addChanged(path)
}

func (v *View) setDirectory(path string, exists bool) {
	v.directories[pathKey(path)] = exists
	v.addChanged(path)
}

func (v *View) addChanged(path string) {
	parent, ok := parentDir(path)
	if !ok {
		return
	}
	children := v.changed[pathKey(parent)]
	if children == nil {
		children = map[string]string{}
		v.changed[pathKey(parent)] = children
	}
	if _, seen := children[pathKey(path)]; !seen {
		children[pathKey(path)] = path
	}
}

func (v *View) listing(directory string) ([]Entry, error) {
	key := pathKey(directory)
	if listing, ok := v.listings[key]; ok {
		return listing, nil
	}
	var listing []Entry
	if dirExists(directory) {
		names, err := readDirNames(directory)
		if err != nil {
			return nil, &txn.Error{
				Msg: fmt.Sprintf("Cannot list the directory '%s' to find the files to delete."+
					" Check that you may read it.", directory),
				Err: err,
			}
		}
		for _, name := range names {
			path := filepath.Join(directory, name)
			isDir, isLink, err := entryKind(path)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, &txn.Error{
					Msg: fmt.Sprintf("Cannot list the directory '%s' to find the files to delete."+
						" Check that you may read it.", directory),
					Err: err,
				}
			}
			listing = append(listing, Entry{FullName: path, IsDirectory: isDir, IsLink: isLink})
		}
	}
	v.listings[key] = listing
	return listing, nil
}

func readDirNames(dir string) ([]string, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Readdirnames(-1)
}

// fileExists is File.Exists.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// dirExists is Directory.Exists.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
