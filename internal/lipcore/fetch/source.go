package fetch

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"
)

// AssetSource exposes the files of one asset by key (lip ISource). Keys are
// archive entry names used verbatim (no "./" stripping; directories
// excluded); an uncompressed asset has the single key "". Like lip, keys
// are not sanitized ("../x", absolute paths and backslashes are kept);
// callers placing files must validate them.
// It satisfies the engine's workspace.Source.
type AssetSource interface {
	Keys() []string
	Open(key string) (io.ReadCloser, error)
	// Prefetch hints that keys are about to be read; a no-op for the
	// sources in this package.
	Prefetch(ctx context.Context, keys []string) error
	Close() error
}

// ErrKeyNotFound is returned (wrapped) by Open for unknown keys.
var ErrKeyNotFound = errors.New("key not found")

func keyNotFound(key string) error {
	return fmt.Errorf("%w: '%s'", ErrKeyNotFound, key)
}

// OpenArchive opens a downloaded asset file of the given manifest asset type
// ("zip", "tar", "tgz" or "uncompressed").
//
// Like lip (SharpCompress ArchiveFactory.OpenArchive), the archive format of
// zip/tar/tgz assets is detected from the content, not from assetType:
// zip, tar, and gzip (a gzipped tar exposes the tar's entries; any other
// gzip exposes one entry named by the gzip header's file name). Archive
// entries that are directories are excluded (zip: name ends with "/"; tar:
// type '5'); tar PAX global headers are skipped; symlinks, hard links and
// other special tar entries are exposed with empty content, as
// SharpCompress does.
//
// Known differences from lip 0.37.0 / SharpCompress 0.50.5:
//   - SharpCompress 0.50.5 fails to open gzipped tars ("Cannot determine
//     compressed stream type"), so tgz assets never worked in lip 0.37.0;
//     here they are read as tar.
//   - pre-POSIX tar entries with type '\0' and a name ending in "/" are
//     treated as directories here (Go archive/tar); SharpCompress exposes
//     them as files.
func OpenArchive(assetType string, path string) (AssetSource, error) {
	switch assetType {
	case "uncompressed":
		return NewSingleFileSource(path), nil
	case "zip", "tar", "tgz":
		return openArchiveFile(path)
	default:
		return nil, fmt.Errorf("unsupported asset type %q", assetType)
	}
}

// netName converts an archive entry name the way .NET decoding it as UTF-8
// would: each invalid byte becomes U+FFFD.
func netName(name string) string {
	if utf8.ValidString(name) {
		return name
	}
	var b strings.Builder
	for i := 0; i < len(name); {
		r, size := utf8.DecodeRuneInString(name[i:])
		b.WriteRune(r)
		i += size
	}
	return b.String()
}

type archiveKind int

const (
	kindZip archiveKind = iota
	kindTar
	kindTarGz
	kindGzip
)

func openArchiveFile(path string) (AssetSource, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	kind, err := detect(f)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	switch kind {
	case kindZip:
		fi, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, err
		}
		zr, err := zip.NewReader(f, fi.Size())
		if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
			f.Close()
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return newZipSource(f, zr), nil
	case kindGzip:
		f.Close()
		return newGzipSource(path)
	default:
		f.Close()
		return newTarSource(path, kind == kindTarGz)
	}
}

// detect sniffs the archive format of f (rewinding it afterwards).
func detect(f *os.File) (archiveKind, error) {
	defer f.Seek(0, io.SeekStart)
	head := make([]byte, 512)
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return 0, err
	}
	head = head[:n]
	switch {
	case bytes.HasPrefix(head, []byte("PK\x03\x04")),
		bytes.HasPrefix(head, []byte("PK\x05\x06")),
		bytes.HasPrefix(head, []byte("PK\x07\x08")):
		return kindZip, nil
	case bytes.HasPrefix(head, []byte{0x1f, 0x8b}):
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return 0, err
		}
		gz, err := gzip.NewReader(f)
		if err != nil {
			return 0, err
		}
		inner := make([]byte, 512)
		m, _ := io.ReadFull(gz, inner)
		if isTarHeader(inner[:m]) {
			return kindTarGz, nil
		}
		return kindGzip, nil
	case isTarHeader(head):
		return kindTar, nil
	}
	return 0, errors.New("cannot determine archive type (supported: zip, tar, gzip)")
}

// isTarHeader reports whether block is a tar header with a valid checksum.
func isTarHeader(block []byte) bool {
	if len(block) < 512 {
		return false
	}
	chk := strings.TrimRight(strings.TrimLeft(string(block[148:156]), " "), " \x00")
	var want int64
	if chk == "" {
		return false
	}
	for _, c := range chk {
		if c < '0' || c > '7' {
			return false
		}
		want = want*8 + int64(c-'0')
	}
	var unsigned, signed int64
	for i, c := range block[:512] {
		if 148 <= i && i < 156 {
			c = ' '
		}
		unsigned += int64(c)
		signed += int64(int8(c))
	}
	return want == unsigned || want == signed
}

// --- zip ---

type zipSource struct {
	f     *os.File
	keys  []string
	files map[string]*zip.File // first non-directory entry per key
}

func newZipSource(f *os.File, zr *zip.Reader) *zipSource {
	s := &zipSource{f: f, files: map[string]*zip.File{}}
	for _, zf := range zr.File {
		name := netName(zf.Name)
		if strings.HasSuffix(name, "/") {
			continue
		}
		s.keys = append(s.keys, name)
		if _, dup := s.files[name]; !dup {
			s.files[name] = zf
		}
	}
	return s
}

func (s *zipSource) Keys() []string { return append([]string(nil), s.keys...) }

func (s *zipSource) Open(key string) (io.ReadCloser, error) {
	if zf, ok := s.files[key]; ok {
		return zf.Open()
	}
	return nil, keyNotFound(key)
}

func (s *zipSource) Prefetch(context.Context, []string) error { return nil }
func (s *zipSource) Close() error                             { return s.f.Close() }

// --- tar / tgz ---

// tarSource indexes a tar once (a gzipped tar is first decompressed to a
// temporary file) and serves entries by offset.
type tarSource struct {
	path    string // the tar file (the temporary file for tgz)
	tmp     bool   // path is a temporary file owned by the source
	f       *os.File
	keys    []string
	entries map[string]tarEntry // first non-directory entry per key
}

type tarEntry struct {
	off, size int64
	sparse    bool // GNU sparse data must be expanded by archive/tar
}

func newTarSource(path string, gz bool) (_ *tarSource, err error) {
	s := &tarSource{path: path, entries: map[string]tarEntry{}}
	if gz {
		if s.path, err = gunzipToTemp(path); err != nil {
			return nil, err
		}
		s.tmp = true
	}
	defer func() {
		if err != nil {
			s.Close()
		}
	}()
	if s.f, err = os.Open(s.path); err != nil {
		return nil, err
	}
	cr := &countingReader{r: bufio.NewReader(s.f)}
	tr := tar.NewReader(cr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil && !errors.Is(err, tar.ErrInsecurePath) {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if skipTarEntry(hdr) {
			continue
		}
		name := netName(hdr.Name)
		s.keys = append(s.keys, name)
		if _, dup := s.entries[name]; dup {
			continue
		}
		e := tarEntry{off: cr.n, size: hdr.Size, sparse: isSparse(hdr)}
		if headerOnly(hdr.Typeflag) {
			e.size = 0
		}
		s.entries[name] = e
	}
	return s, nil
}

func gunzipToTemp(path string) (string, error) {
	in, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer in.Close()
	gz, err := gzip.NewReader(bufio.NewReader(in))
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	out, err := os.CreateTemp("", "lip-tgz-*.tar")
	if err != nil {
		return "", err
	}
	if _, err = io.Copy(out, gz); err == nil {
		err = out.Close()
	} else {
		out.Close()
	}
	if err != nil {
		os.Remove(out.Name())
		return "", fmt.Errorf("%s: %w", path, err)
	}
	return out.Name(), nil
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

func skipTarEntry(hdr *tar.Header) bool {
	switch hdr.Typeflag {
	case tar.TypeDir, tar.TypeXGlobalHeader:
		return true
	}
	return false
}

// headerOnly reports entry types whose data archive/tar treats as empty.
func headerOnly(flag byte) bool {
	switch flag {
	case tar.TypeLink, tar.TypeSymlink, tar.TypeChar, tar.TypeBlock, tar.TypeDir, tar.TypeFifo:
		return true
	}
	return false
}

func isSparse(hdr *tar.Header) bool {
	if hdr.Typeflag == tar.TypeGNUSparse {
		return true
	}
	for k := range hdr.PAXRecords {
		if strings.HasPrefix(k, "GNU.sparse.") {
			return true
		}
	}
	return false
}

func (s *tarSource) Keys() []string { return append([]string(nil), s.keys...) }

func (s *tarSource) Open(key string) (io.ReadCloser, error) {
	e, ok := s.entries[key]
	if !ok {
		return nil, keyNotFound(key)
	}
	if !e.sparse {
		return io.NopCloser(io.NewSectionReader(s.f, e.off, e.size)), nil
	}
	return s.readSparse(key)
}

// readSparse rescans the archive to let archive/tar expand a sparse entry.
func (s *tarSource) readSparse(key string) (io.ReadCloser, error) {
	tr := tar.NewReader(io.NewSectionReader(s.f, 0, 1<<62))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, keyNotFound(key)
		}
		if err != nil && !errors.Is(err, tar.ErrInsecurePath) {
			return nil, err
		}
		if skipTarEntry(hdr) || netName(hdr.Name) != key {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		return io.NopCloser(bytes.NewReader(data)), nil
	}
}

func (s *tarSource) Prefetch(context.Context, []string) error { return nil }

// Close closes the archive and removes the decompressed temporary file.
func (s *tarSource) Close() error {
	var err error
	if s.f != nil {
		err = s.f.Close()
		s.f = nil
	}
	if s.tmp {
		if rerr := os.Remove(s.path); rerr != nil && !errors.Is(rerr, fs.ErrNotExist) && err == nil {
			err = rerr
		}
		s.tmp = false
	}
	return err
}

type multiCloser []io.Closer

func (m multiCloser) Close() error {
	var errs []error
	for _, c := range m {
		errs = append(errs, c.Close())
	}
	return errors.Join(errs...)
}

// --- plain gzip ---

type gzipSource struct {
	path string
	name string
}

func newGzipSource(path string) (*gzipSource, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	return &gzipSource{path: path, name: gz.Name}, nil
}

func (s *gzipSource) Keys() []string { return []string{s.name} }

func (s *gzipSource) Open(key string) (io.ReadCloser, error) {
	if key != s.name {
		return nil, keyNotFound(key)
	}
	f, err := os.Open(s.path)
	if err != nil {
		return nil, err
	}
	gz, err := gzip.NewReader(bufio.NewReader(f))
	if err != nil {
		f.Close()
		return nil, err
	}
	return struct {
		io.Reader
		io.Closer
	}{gz, multiCloser{gz, f}}, nil
}

func (s *gzipSource) Prefetch(context.Context, []string) error { return nil }
func (s *gzipSource) Close() error                             { return nil }

// --- single file ---

// SingleFileSource is lip's SingleFileSource: one file under the key "".
type SingleFileSource struct{ path string }

// NewSingleFileSource returns a source exposing path as the key "".
func NewSingleFileSource(path string) *SingleFileSource { return &SingleFileSource{path: path} }

func (s *SingleFileSource) Keys() []string { return []string{""} }

func (s *SingleFileSource) Open(key string) (io.ReadCloser, error) {
	if key != "" {
		return nil, keyNotFound(key)
	}
	return os.Open(s.path)
}

func (s *SingleFileSource) Prefetch(context.Context, []string) error { return nil }
func (s *SingleFileSource) Close() error                             { return nil }

// --- directory ---

const gitEntryName = ".git"

// DirectorySource is lip's DirectorySource: the files below a directory,
// keyed by their relative path with "/" separators, skipping ".git" files
// and directories (without descending into them).
type DirectorySource struct{ dir string }

// NewDirectorySource returns a source over the files below dir.
func NewDirectorySource(dir string) *DirectorySource { return &DirectorySource{dir: dir} }

// Keys lists files depth-first: a directory's files before its
// subdirectories, each in name order. Directory links are followed, except
// into a directory that is already being walked (a link cycle).
func (s *DirectorySource) Keys() []string {
	var keys []string
	var walking []os.FileInfo // directories on the current path
	var walk func(rel string)
	walk = func(rel string) {
		abs := filepath.Join(s.dir, filepath.FromSlash(rel))
		fi, err := os.Stat(abs)
		if err != nil {
			return
		}
		for _, a := range walking {
			if os.SameFile(a, fi) {
				return
			}
		}
		walking = append(walking, fi)
		defer func() { walking = walking[:len(walking)-1] }()
		entries, err := os.ReadDir(abs)
		if err != nil {
			return
		}
		var dirs []string
		for _, e := range entries {
			if e.Name() == gitEntryName {
				continue
			}
			k := e.Name()
			if rel != "" {
				k = rel + "/" + k
			}
			if isDirEntry(s.dir, k, e) {
				dirs = append(dirs, k)
			} else {
				keys = append(keys, k)
			}
		}
		for _, d := range dirs {
			walk(d)
		}
	}
	walk("")
	return keys
}

// isDirEntry follows symlinks and junctions like .NET
// EnumerateDirectories does.
func isDirEntry(root, key string, e fs.DirEntry) bool {
	if e.IsDir() {
		return true
	}
	if e.Type().IsRegular() {
		return false
	}
	fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(key)))
	return err == nil && fi.IsDir()
}

func (s *DirectorySource) Open(key string) (io.ReadCloser, error) {
	norm := strings.ReplaceAll(key, "\\", "/")
	for _, seg := range strings.Split(norm, "/") {
		if seg == "" || seg == "." || seg == ".." || seg == gitEntryName {
			return nil, keyNotFound(key)
		}
	}
	p := filepath.Join(s.dir, filepath.FromSlash(norm))
	fi, err := os.Stat(p)
	if err != nil || fi.IsDir() {
		return nil, keyNotFound(key)
	}
	return os.Open(p)
}

func (s *DirectorySource) Prefetch(context.Context, []string) error { return nil }
func (s *DirectorySource) Close() error                             { return nil }

// --- composite ---

// CompositeSource is lip's CompositeSource: the distinct keys of all
// sources in order; a key is read from the first source serving it. The
// key-to-source mapping is computed once, on first use.
type CompositeSource struct {
	sources []AssetSource

	once  sync.Once
	keys  []string
	owner map[string]int // key -> index of the first source serving it
}

// NewCompositeSource combines sources.
func NewCompositeSource(sources ...AssetSource) *CompositeSource {
	return &CompositeSource{sources: sources}
}

func (s *CompositeSource) index() {
	s.once.Do(func() {
		s.owner = map[string]int{}
		for i, src := range s.sources {
			for _, k := range src.Keys() {
				if _, ok := s.owner[k]; !ok {
					s.owner[k] = i
					s.keys = append(s.keys, k)
				}
			}
		}
	})
}

func (s *CompositeSource) Keys() []string {
	s.index()
	return append([]string(nil), s.keys...)
}

func (s *CompositeSource) Open(key string) (io.ReadCloser, error) {
	s.index()
	if i, ok := s.owner[key]; ok {
		return s.sources[i].Open(key)
	}
	return nil, keyNotFound(key)
}

// Prefetch forwards each key to the source Open would read it from.
func (s *CompositeSource) Prefetch(ctx context.Context, keys []string) error {
	s.index()
	served := make([][]string, len(s.sources))
	seen := map[string]bool{}
	for _, k := range keys {
		if i, ok := s.owner[k]; ok && !seen[k] {
			seen[k] = true
			served[i] = append(served[i], k)
		}
	}
	for i, ks := range served {
		if len(ks) > 0 {
			if err := s.sources[i].Prefetch(ctx, ks); err != nil {
				return err
			}
		}
	}
	return nil
}

// Close closes every source.
func (s *CompositeSource) Close() error {
	var errs []error
	for _, src := range s.sources {
		errs = append(errs, src.Close())
	}
	return errors.Join(errs...)
}
