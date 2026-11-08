package archive

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// DirectorySource is a directory archive (protocol/package.md §1;
// Sources/DirectorySource.cs): the files below a directory of the local file
// system, keyed by their '/'-separated path relative to it, in ordinal order.
// Links and other reparse points are skipped, not followed; an inaccessible
// directory fails the listing rather than being left out. Files are read
// where they are, when opened.
type DirectorySource struct {
	root  string
	keys  []string
	sizes map[string]int64
}

// NewDirectorySource lists the files below dir.
func NewDirectorySource(dir string) (*DirectorySource, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, &FileError{Message: listError(dir), Err: err}
	}
	files, err := listFiles(root)
	if err != nil {
		return nil, &FileError{Message: listError(root), Err: err}
	}
	// Outside Windows '\' is not a separator, so a name may contain it; such
	// a directory is not a valid archive (protocol/package.md §1).
	if filepath.Separator != '\\' {
		for _, f := range files {
			if i := strings.IndexByte(f.key, '\\'); i >= 0 {
				name := f.key
				if end := strings.IndexByte(f.key[i:], '/'); end >= 0 {
					name = f.key[:i+end]
				}
				return nil, &UnavailableError{Message: "The package directory " + root + " is not a valid archive: '" +
					filepath.Join(root, filepath.FromSlash(name)) + "' has a '\\' in its name, which lip does not" +
					" install. Rename it."}
			}
		}
	}
	s := &DirectorySource{root: root, sizes: make(map[string]int64, len(files))}
	for _, f := range files {
		s.keys = append(s.keys, f.key)
		s.sizes[f.key] = f.size
	}
	slices.SortFunc(s.keys, compareOrdinal)
	return s, nil
}

func listError(root string) string {
	return "Could not list the files of the package directory " + root + ". Check that lip may read" +
		" it and every directory below it."
}

// Root is the absolute path of the directory.
func (s *DirectorySource) Root() string { return s.root }

func (s *DirectorySource) Keys() []string { return append([]string(nil), s.keys...) }

// Size is the size of a file when it was listed.
func (s *DirectorySource) Size(key string) (int64, bool) {
	size, ok := s.sizes[key]
	return size, ok
}

func (s *DirectorySource) Open(key string) (io.ReadCloser, error) {
	if _, ok := s.sizes[key]; !ok {
		return nil, &noFileError{"The directory " + s.root + " has no file '" + key + "'."}
	}
	path := filepath.Join(s.root, filepath.FromSlash(key))
	f, err := os.Open(path)
	if err != nil {
		return nil, &FileError{
			Message: "Could not read the file " + path + " of the package directory " + s.root + ".",
			Err:     err,
		}
	}
	return f, nil
}

type listedFile struct {
	key  string // '/'-separated, relative to the root
	path string
	size int64
}

// listFiles lists every file below root recursively, as .NET's
// Directory.EnumerateFiles with AttributesToSkip = ReparsePoint and
// IgnoreInaccessible = false: entries that are reparse points (links,
// junctions) are neither listed nor descended into, hidden and system files
// are listed, and a directory that cannot be read is an error.
func listFiles(root string) ([]listedFile, error) {
	var files []listedFile
	var walk func(dir, prefix string) error
	walk = func(dir, prefix string) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				if os.IsNotExist(err) {
					continue // removed while listing
				}
				return err
			}
			if isReparsePoint(info) {
				continue
			}
			path := filepath.Join(dir, e.Name())
			if info.IsDir() {
				if err := walk(path, prefix+e.Name()+"/"); err != nil {
					return err
				}
				continue
			}
			// Special files (FIFOs, sockets, device nodes) are no files of an
			// archive; reading a FIFO would wait for a writer forever.
			if !info.Mode().IsRegular() && isSpecialFile(path) {
				continue
			}
			files = append(files, listedFile{key: prefix + e.Name(), path: path, size: info.Size()})
		}
		return nil
	}
	if err := walk(root, ""); err != nil {
		return nil, err
	}
	return files, nil
}
