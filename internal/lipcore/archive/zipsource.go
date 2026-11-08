package archive

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"
	"sync"
	"time"
)

// ZipSource is the files of a ZIP archive (ZipSource.cs), read strictly as
// ZIP with the same parser ValidateZip checks it with, so the entries that
// were validated are exactly the entries that are served. Its keys are the
// entries that do not end with '/', in entry order, each once; a duplicate
// name is served from its first entry. Only Stored, Deflate and Deflate64
// entries can be read, as with .NET's ZipArchive.
//
// A file archive is opened again for every use, so no handle is held in
// between; its central directory is read again when the file has changed
// (size or modification time), as lip reads it on every use.
type ZipSource struct {
	open           zipOpener
	description    string
	cachedDownload bool

	mu      sync.Mutex
	stamp   zipStamp
	keys    []string
	entries map[string]*zipEntry
	// prefetched are the files extracted by Prefetch, by key.
	prefetched map[string][]byte
}

// zipOpener opens the archive: its content, size, identity and a closer.
type zipOpener func() (r io.ReaderAt, size int64, stamp zipStamp, c io.Closer, err error)

// zipStamp tells whether a file archive changed since it was read.
type zipStamp struct {
	size    int64
	modTime time.Time
}

type zipEntry struct {
	method         uint16
	compressedSize uint64
	size           uint64
	offset         int64 // of the data, after the local header
	offsetErr      error
}

const (
	zipStored    = 0
	zipDeflate   = 8
	zipDeflate64 = 9
)

// OpenZipFile validates the ZIP archive at path ("The local file <path>")
// and opens it as "the ZIP archive <path>".
func OpenZipFile(path string) (*ZipSource, error) {
	return newZipSource(fileOpener(path), "the ZIP archive "+path, true, "The local file "+path)
}

// OpenZipBytes validates a ZIP archive held in memory and opens it.
func OpenZipBytes(b []byte) (*ZipSource, error) {
	return newZipSource(bytesOpener(b), "the ZIP archive", false, "The ZIP archive")
}

// OpenLocalZip opens a local ZIP file the way ArchiveProvider.OpenFile does:
// read once into memory, so that the files installed are exactly the bytes
// validated, even if the file changes meanwhile, then validated. Every
// failure is an *UnavailableError.
func OpenLocalZip(path string) (*ZipSource, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, &UnavailableError{
				Message: "There is no file or directory " + path + ". Check the path; a relative path" +
					" is relative to the current directory.",
				Err: err,
			}
		}
		return nil, &UnavailableError{Message: "Could not read the local file " + path + ".", Err: err}
	}
	return newZipSource(bytesOpener(content), "the local file "+path, false, "The local file "+path)
}

// NewZipFileSource opens the ZIP file at path without validating it: a file
// in the lip cache (validated when it was stored), whose errors advise 'lip
// cache clean'. description names it in errors, e.g. "the download of <URL>
// (<path>)".
func NewZipFileSource(path, description string) (*ZipSource, error) {
	return newZipSource(fileOpener(path), description, true, "")
}

// NewZipBytesSource opens a ZIP archive held in memory without validating
// it, e.g. one extracted from another archive.
func NewZipBytesSource(content []byte, description string) (*ZipSource, error) {
	return newZipSource(bytesOpener(content), description, false, "")
}

// NewCachedZipSource opens a download of the lip cache read into memory and
// validated there (ArchiveProvider.CachedZipSource), so that what is
// installed is exactly what was validated, even if another process deletes
// or replaces the cache entry meanwhile. Its errors advise 'lip cache clean'.
func NewCachedZipSource(content []byte, description string) (*ZipSource, error) {
	return newZipSource(bytesOpener(content), description, true, "")
}

// Prefetch is ZipSource.Prefetch: extracts the given files now, so that a
// damaged file or one compressed with a method lip does not support fails
// here rather than when it is read.
func (s *ZipSource) Prefetch(_ context.Context, keys []string) error {
	for _, key := range keys {
		s.mu.Lock()
		_, done := s.prefetched[key]
		s.mu.Unlock()
		if done {
			continue
		}
		r, err := s.open1(key)
		if err != nil {
			return err
		}
		content, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			return err
		}
		s.mu.Lock()
		if s.prefetched == nil {
			s.prefetched = map[string][]byte{}
		}
		s.prefetched[key] = content
		s.mu.Unlock()
	}
	return nil
}

func fileOpener(path string) zipOpener {
	return func() (io.ReaderAt, int64, zipStamp, io.Closer, error) {
		f, err := os.Open(path)
		if err != nil {
			return nil, 0, zipStamp{}, nil, err
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, 0, zipStamp{}, nil, err
		}
		return f, info.Size(), zipStamp{info.Size(), info.ModTime()}, f, nil
	}
}

func bytesOpener(content []byte) zipOpener {
	return func() (io.ReaderAt, int64, zipStamp, io.Closer, error) {
		return bytes.NewReader(content), int64(len(content)), zipStamp{}, io.NopCloser(nil), nil
	}
}

// newZipSource reads the archive. With a subject, the archive is validated
// first (ZipValidator.Validate) and its failures are *UnavailableError
// naming it by subject, or the error of reading the file as it is.
func newZipSource(open zipOpener, description string, cachedDownload bool, subject string) (*ZipSource, error) {
	s := &ZipSource{open: open, description: description, cachedDownload: cachedDownload}
	r, size, stamp, c, err := open()
	if err != nil {
		if subject != "" {
			return nil, err
		}
		return nil, s.openError(err)
	}
	defer c.Close()
	zr, err := newZipReader(r, size)
	if err != nil && subject != "" {
		if isZipFormatError(err) {
			return nil, notZipError(subject, r, size, err)
		}
		return nil, err
	}
	if err == nil && subject != "" {
		if err := validateEntries(zr, subject); err != nil {
			return nil, err
		}
	}
	if err := s.load(zr, err, stamp); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *ZipSource) openError(err error) error {
	return &FileError{Message: "Could not open " + s.description + "." + s.unreadableAdvice(), Err: err}
}

// load sets the entries from the central directory (parsed, or failed to
// parse with zerr). The local headers are read here, while the archive is
// open; a damaged one fails only the read of its entry, as in .NET.
func (s *ZipSource) load(zr *zip.Reader, zerr error, stamp zipStamp) error {
	if zerr != nil {
		if isZipFormatError(zerr) {
			return &DamagedError{
				Message: capitalize(s.description) + " is not a valid ZIP archive or is damaged." + s.damagedAdvice(),
				Err:     zerr,
			}
		}
		return &FileError{Message: "Could not read " + s.description + "." + s.unreadableAdvice(), Err: zerr}
	}
	var keys []string
	entries := map[string]*zipEntry{}
	for _, zf := range zr.File {
		name := netName(zf.Name)
		if strings.HasSuffix(name, "/") {
			continue
		}
		if _, dup := entries[name]; dup {
			continue
		}
		e := &zipEntry{
			method:         zf.Method,
			compressedSize: zf.CompressedSize64,
			size:           zf.UncompressedSize64,
		}
		e.offset, e.offsetErr = zf.DataOffset()
		keys = append(keys, name)
		entries[name] = e
	}
	s.mu.Lock()
	s.stamp, s.keys, s.entries = stamp, keys, entries
	s.mu.Unlock()
	return nil
}

// refresh reads the central directory again when the opened archive is not
// the one read before.
func (s *ZipSource) refresh(r io.ReaderAt, size int64, stamp zipStamp) error {
	s.mu.Lock()
	same := stamp == s.stamp
	s.mu.Unlock()
	if same {
		return nil
	}
	zr, err := newZipReader(r, size)
	return s.load(zr, err, stamp)
}

// Keys returns the file keys in entry order.
func (s *ZipSource) Keys() []string {
	if r, size, stamp, c, err := s.open(); err == nil {
		s.refresh(r, size, stamp) // on failure, the keys read before
		c.Close()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.keys...)
}

// Size returns the uncompressed size of a file.
func (s *ZipSource) Size(key string) (int64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[key]
	if !ok {
		return 0, false
	}
	return int64(e.size), true
}

// Open opens a file for reading. A damaged entry fails with *DamagedError,
// at Open or while reading; a failure to read the archive with *FileError.
// A file extracted by Prefetch is served as extracted.
func (s *ZipSource) Open(key string) (io.ReadCloser, error) {
	s.mu.Lock()
	content, ok := s.prefetched[key]
	s.mu.Unlock()
	if ok {
		return io.NopCloser(bytes.NewReader(content)), nil
	}
	return s.open1(key)
}

// open1 opens a file of the archive for reading.
func (s *ZipSource) open1(key string) (io.ReadCloser, error) {
	r, size, stamp, c, err := s.open()
	if err != nil {
		return nil, s.openError(err)
	}
	if err := s.refresh(r, size, stamp); err != nil {
		c.Close()
		return nil, err
	}
	s.mu.Lock()
	e, ok := s.entries[key]
	s.mu.Unlock()
	if !ok {
		c.Close()
		return nil, &noFileError{capitalize(s.description) + " has no file '" + key + "'."}
	}
	if e.offsetErr != nil {
		c.Close()
		return nil, s.readError(key, e.offsetErr)
	}
	if e.compressedSize > uint64(size) || uint64(e.offset) > uint64(size)-e.compressedSize {
		// ZipArchiveEntry.IsOpenable: "A local file header is corrupt."
		c.Close()
		return nil, s.readError(key, zip.ErrFormat)
	}
	// What ZipArchiveEntry.Open of .NET 10 serves, which lip relies on: the
	// encryption flags are ignored (the data is served as if it were not
	// encrypted), the CRC-32 is not checked, a stored entry is its
	// compressed size of data, and a compressed one is what the
	// decompressor yields, cut at the uncompressed size; compressed data
	// that ends early ends the file without an error.
	raw := io.NewSectionReader(r, e.offset, int64(e.compressedSize))
	z := &zipEntryReader{s: s, key: key, closer: c}
	switch e.method {
	case zipStored:
		z.r = raw
	case zipDeflate:
		z.r, z.limit, z.compressed = newDeflateReader(bufio.NewReader(raw)), e.size, true
	case zipDeflate64:
		z.r, z.limit, z.compressed = newDeflate64Reader(bufio.NewReader(raw)), e.size, true
		trial := bufio.NewReader(io.NewSectionReader(r, e.offset, int64(e.compressedSize)))
		if n, ok := dotnetDeflate64Length(trial, int64(e.compressedSize), e.size); ok && uint64(n) < z.limit {
			z.limit = uint64(n)
		}
	default:
		c.Close()
		return nil, s.readError(key, zip.ErrAlgorithm)
	}
	return z, nil
}

// readError names the archive and the entry in a failure to read it.
func (s *ZipSource) readError(key string, err error) error {
	if isDamaged(err) {
		return &DamagedError{
			Message: "Could not extract '" + key + "' from " + s.description + ": the archive is damaged," +
				" or the file is compressed with a method lip does not support (only Deflate," +
				" Deflate64 and no compression are supported)." + s.damagedAdvice(),
			Err: err,
		}
	}
	return &FileError{Message: "Could not read '" + key + "' from " + s.description + "." + s.unreadableAdvice(), Err: err}
}

func isDamaged(err error) bool {
	return isZipFormatError(err) || errors.Is(err, errDeflate64)
}

func (s *ZipSource) damagedAdvice() string {
	if s.cachedDownload {
		return " The download may have been damaged after it was stored: run 'lip cache clean' and try" +
			" again. If this keeps happening, ask the package author to create the archive with a" +
			" standard ZIP tool."
	}
	return " Ask the package author to create the archive with a standard ZIP tool."
}

func (s *ZipSource) unreadableAdvice() string {
	if s.cachedDownload {
		return " If another lip command is cleaning the cache, wait for it to finish; otherwise close" +
			" programs that may use the file, or run 'lip cache clean', and try again."
	}
	return ""
}

// zipEntryReader reads an entry as .NET's ZipArchiveEntry.Open stream does,
// naming the entry in errors.
type zipEntryReader struct {
	s      *ZipSource
	key    string
	r      io.Reader
	closer io.Closer
	// compressed: r is a decompressor, whose output DeflateStream cuts at
	// limit bytes (the uncompressed size), and whose running out of
	// compressed data is the end of the file.
	compressed bool
	limit      uint64
	n          uint64
	err        error
}

func (z *zipEntryReader) Read(p []byte) (int, error) {
	if z.err != nil {
		return 0, z.err
	}
	if z.compressed {
		if z.n >= z.limit {
			z.err = io.EOF
			return 0, io.EOF
		}
		if rest := z.limit - z.n; uint64(len(p)) > rest {
			p = p[:rest]
		}
	}
	n, err := z.r.Read(p)
	z.n += uint64(n)
	if z.compressed && errors.Is(err, io.ErrUnexpectedEOF) {
		err = io.EOF
	}
	if err == nil {
		return n, nil
	}
	if err != io.EOF {
		err = z.s.readError(z.key, err)
	}
	z.err = err
	return n, err
}

func (z *zipEntryReader) Close() error { return z.closer.Close() }
