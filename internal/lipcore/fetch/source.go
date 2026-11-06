package fetch

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/bodgit/sevenzip"
	"github.com/nwaples/rardecode/v2"
	"github.com/ulikunitz/xz/lzma"
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
// zip, RAR, tar, gzip and 7z (a gzipped tar exposes the tar's entries; any
// other gzip exposes one entry named by the gzip header's file name, or no
// entry without one). Archive entries that are directories are excluded
// (zip: name ends with "/", or with "\" for an empty entry; tar: type '5';
// RAR and 7z: the directory flag/attribute); tar PAX global headers are
// skipped; symlinks, hard links and other special tar entries are exposed
// with empty content, as SharpCompress does. Zip entries may be stored or
// compressed with Deflate, Deflate64, BZip2, LZMA, Zstandard or XZ (with
// the x86, PowerPC, IA-64, ARM, ARM-Thumb and SPARC filters, but not Delta).
//
// Known differences from lip 0.37.0 / SharpCompress 0.50.5:
//   - SharpCompress 0.50.5 fails to open gzipped tars ("Cannot determine
//     compressed stream type"), so tgz assets never worked in lip 0.37.0;
//     here they are read as tar.
//   - pre-POSIX tar entries with type '\0' and a name ending in "/" are
//     treated as directories here (Go archive/tar); SharpCompress exposes
//     them as files.
//   - sparse tar entries are expanded here; SharpCompress exposes their
//     stored data under the raw header name.
//   - zip entries using the legacy Shrink, Reduce and Implode methods or
//     PPMd fail to open here (with an error naming the method).
//   - LZMA, XZ and Zstandard zip entries are decoded with a dictionary or
//     window of at most their declared size; entries larger than 256 MiB
//     whose stream asks for more than that fail to read here.
//   - 7z directories are recognised by their attributes rather than by the
//     empty-stream/empty-file flags, and 7z anti-items are exposed as files
//     by SharpCompress only.
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

// netName converts archive entry name bytes the way .NET's UTF-8 decoder
// (Encoding.UTF8.GetString) does: each maximal subpart of an ill-formed
// sequence becomes one U+FFFD (Unicode "U+FFFD substitution of maximal
// subparts"), so "\xE5\xBC" (a truncated 3-byte sequence) is one
// replacement, not two.
func netName(name string) string {
	if utf8.ValidString(name) {
		return name
	}
	var b strings.Builder
	for i := 0; i < len(name); {
		r, size := utf8.DecodeRuneInString(name[i:])
		if r != utf8.RuneError || size > 1 {
			b.WriteString(name[i : i+size])
			i += size
			continue
		}
		b.WriteRune(utf8.RuneError)
		i += maximalSubpart(name[i:])
	}
	return b.String()
}

// maximalSubpart returns the length of the longest prefix of s (which starts
// with an ill-formed sequence) that is a valid start of a UTF-8 sequence, or
// 1 if the first byte cannot start one (Unicode Table 3-7).
func maximalSubpart(s string) int {
	need, lo, hi := 0, byte(0x80), byte(0xBF)
	switch c := s[0]; {
	case 0xC2 <= c && c <= 0xDF:
		need = 1
	case c == 0xE0:
		need, lo = 2, 0xA0
	case c == 0xED:
		need, hi = 2, 0x9F
	case 0xE1 <= c && c <= 0xEF:
		need = 2
	case c == 0xF0:
		need, lo = 3, 0x90
	case c == 0xF4:
		need, hi = 3, 0x8F
	case 0xF1 <= c && c <= 0xF3:
		need = 3
	default:
		return 1
	}
	n := 1
	for n <= need && n < len(s) && lo <= s[n] && s[n] <= hi {
		n++
		lo, hi = 0x80, 0xBF
	}
	return n
}

type archiveKind int

const (
	kindZip archiveKind = iota
	kindTar
	kindTarGz
	kindGzip
	kind7z
	kindRar4
	kindRar5
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
	var src AssetSource
	switch kind {
	case kindZip, kind7z, kindRar4, kindRar5:
		fi, err := f.Stat()
		if err == nil {
			switch kind {
			case kindZip:
				src, err = openZip(f, fi.Size())
			case kind7z:
				src, err = open7z(f, fi.Size())
			default:
				src, err = openRar(f, kind == kindRar5)
			}
		}
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return src, nil
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
	case bytes.HasPrefix(head, []byte("Rar!\x1a\x07\x00")):
		return kindRar4, nil
	case bytes.HasPrefix(head, []byte("Rar!\x1a\x07\x01\x00")):
		return kindRar5, nil
	case bytes.HasPrefix(head, []byte("7z\xbc\xaf\x27\x1c")):
		return kind7z, nil
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
	return 0, errors.New("cannot determine archive type (supported: zip, rar, tar, gzip, 7z)")
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

func openZip(f *os.File, size int64) (*zipSource, error) {
	zr, err := zip.NewReader(f, size)
	if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
		return nil, err
	}
	s := &zipSource{f: f, files: map[string]*zip.File{}}
	for _, zf := range zr.File {
		name, err := zipEntryName(zf)
		if err != nil {
			return nil, err
		}
		// SharpCompress ZipFileEntry.IsDirectory.
		if strings.HasSuffix(name, "/") ||
			(strings.HasSuffix(name, "\\") && zf.CompressedSize64 == 0 && zf.UncompressedSize64 == 0) {
			continue
		}
		s.keys = append(s.keys, name)
		if _, dup := s.files[name]; !dup {
			s.files[name] = zf
		}
	}
	return s, nil
}

const zipUnicodePathExtra = 0x7075

// zipEntryName returns an entry's name as SharpCompress reads it from the
// central directory: the raw name decoded as UTF-8 (with or without the EFS
// flag, as .NET's default encoding is UTF-8), replaced by the first Info-ZIP
// Unicode Path extra field if there is one (its version and name CRC are
// not checked).
func zipEntryName(zf *zip.File) (string, error) {
	extra := zf.Extra
	for i := 0; i+4 <= len(extra); {
		tag := binary.LittleEndian.Uint16(extra[i:])
		size := int(binary.LittleEndian.Uint16(extra[i+2:]))
		if i+4+size > len(extra) {
			break
		}
		if tag == zipUnicodePathExtra {
			if size < 5 {
				// SharpCompress fails to open the archive.
				return "", fmt.Errorf("zip entry %q: invalid Unicode Path extra field", netName(zf.Name))
			}
			return netName(string(extra[i+4+5 : i+4+size])), nil
		}
		i += 4 + size
	}
	return netName(zf.Name), nil
}

func (s *zipSource) Keys() []string { return append([]string(nil), s.keys...) }

func (s *zipSource) Open(key string) (io.ReadCloser, error) {
	if zf, ok := s.files[key]; ok {
		return openZipEntry(zf)
	}
	return nil, keyNotFound(key)
}

// Zip compression methods SharpCompress decodes beyond Store and Deflate.
const (
	zipDeflate64 = 9
	zipBZip2     = 12
	zipLZMA      = 14
	zipZstd      = 93
	zipXZ        = 95
	zipPPMd      = 98
)

// zipLegacyMethods are decoded by SharpCompress but not supported here.
var zipLegacyMethods = map[uint16]string{
	1: "Shrink", 2: "Reduce", 3: "Reduce", 4: "Reduce", 5: "Reduce", 6: "Implode", zipPPMd: "PPMd",
}

// openZipEntry opens an entry for reading. Store and Deflate are decoded by
// archive/zip; the other methods SharpCompress supports are decoded here
// from the raw data, checking the size and CRC-32 like archive/zip does.
func openZipEntry(zf *zip.File) (io.ReadCloser, error) {
	if zf.Method == zip.Store || zf.Method == zip.Deflate {
		return zf.Open()
	}
	if name, ok := zipLegacyMethods[zf.Method]; ok {
		return nil, fmt.Errorf("zip entry %q: compression method %d (%s) is not supported", zf.Name, zf.Method, name)
	}
	raw, err := zf.OpenRaw()
	if err != nil {
		return nil, err
	}
	var rc io.ReadCloser
	switch zf.Method {
	case zipDeflate64:
		rc = io.NopCloser(newDeflate64Reader(raw))
	case zipBZip2:
		rc = io.NopCloser(bzip2.NewReader(raw))
	case zipLZMA:
		rc, err = newZipLZMAReader(raw, zf)
	case zipZstd:
		rc, err = newZipZstdReader(raw, zf.UncompressedSize64)
	case zipXZ:
		rc, err = newZipXZReader(raw, zf.UncompressedSize64)
	default:
		return nil, fmt.Errorf("zip entry %q: %w (method %d)", zf.Name, zip.ErrAlgorithm, zf.Method)
	}
	if err != nil {
		return nil, fmt.Errorf("zip entry %q: %w", zf.Name, err)
	}
	return &zipChecksumReader{rc: rc, size: zf.UncompressedSize64, crc: zf.CRC32}, nil
}

// newZipLZMAReader decodes a zip LZMA entry (APPNOTE 5.8.8): a 2-byte
// version, a 2-byte properties size and the properties, then the raw
// stream, ending with an end marker if general purpose flag bit 1 is set
// and after the entry's uncompressed size otherwise.
func newZipLZMAReader(raw io.Reader, zf *zip.File) (io.ReadCloser, error) {
	eos := zf.Flags&0x2 != 0
	if !eos && zf.UncompressedSize64 == 0 {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}
	var hdr [4]byte
	if _, err := io.ReadFull(raw, hdr[:]); err != nil {
		return nil, err
	}
	props := make([]byte, binary.LittleEndian.Uint16(hdr[2:]))
	if _, err := io.ReadFull(raw, props); err != nil {
		return nil, err
	}
	if len(props) != 5 {
		return nil, fmt.Errorf("unsupported LZMA properties size %d", len(props))
	}
	// Rebuild a .lzma header (properties, dictionary size, size). The
	// dictionary never needs to exceed the data, so a hostile dictionary
	// size cannot force a large allocation (see maxDecoderWindow).
	dict := uint64(binary.LittleEndian.Uint32(props[1:]))
	dict = min(dict, max(zf.UncompressedSize64, lzma.MinDictCap))
	if dict > maxDecoderWindow {
		return nil, errDecoderWindow("LZMA dictionary", dict)
	}
	alone := make([]byte, 13)
	alone[0] = props[0]
	binary.LittleEndian.PutUint32(alone[1:], uint32(dict))
	size := zf.UncompressedSize64
	if eos {
		size = math.MaxUint64 // unknown: up to the end marker
	}
	binary.LittleEndian.PutUint64(alone[5:], size)
	lr, err := lzma.NewReader(io.MultiReader(bytes.NewReader(alone), raw))
	if err != nil {
		return nil, err
	}
	return io.NopCloser(lr), nil
}

// zipChecksumReader verifies an entry's size and CRC-32 at EOF.
type zipChecksumReader struct {
	rc    io.ReadCloser
	size  uint64
	crc   uint32
	n     uint64
	hash  uint32
	ended error
}

func (r *zipChecksumReader) Read(p []byte) (int, error) {
	if r.ended != nil {
		return 0, r.ended
	}
	n, err := r.rc.Read(p)
	r.hash = crc32.Update(r.hash, crc32.IEEETable, p[:n])
	r.n += uint64(n)
	if r.n > r.size {
		r.ended = zip.ErrFormat
		return n, r.ended
	}
	if err == io.EOF {
		switch {
		case r.n != r.size:
			err = io.ErrUnexpectedEOF
		case r.crc != 0 && r.hash != r.crc:
			err = zip.ErrChecksum
		}
		r.ended = err
	} else if err != nil {
		r.ended = err
	}
	return n, err
}

func (r *zipChecksumReader) Close() error { return r.rc.Close() }

func (s *zipSource) Prefetch(context.Context, []string) error { return nil }
func (s *zipSource) Close() error                             { return s.f.Close() }

// --- 7z ---

type sevenZipSource struct {
	f     *os.File
	keys  []string
	files map[string]*sevenzip.File // first non-directory entry per key
}

// open7z indexes a 7z archive; keys are the stored names as-is.
func open7z(f *os.File, size int64) (*sevenZipSource, error) {
	zr, err := sevenzip.NewReader(f, size)
	if err != nil {
		return nil, err
	}
	s := &sevenZipSource{f: f, files: map[string]*sevenzip.File{}}
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		s.keys = append(s.keys, zf.Name)
		if _, dup := s.files[zf.Name]; !dup {
			s.files[zf.Name] = zf
		}
	}
	return s, nil
}

func (s *sevenZipSource) Keys() []string { return append([]string(nil), s.keys...) }

func (s *sevenZipSource) Open(key string) (io.ReadCloser, error) {
	if zf, ok := s.files[key]; ok {
		rc, err := zf.Open()
		if err != nil {
			return nil, err
		}
		// sevenzip yields no data (and no error) for encrypted entries
		// without a password; SharpCompress throws, so check size and CRC.
		return &zipChecksumReader{rc: rc, size: zf.UncompressedSize, crc: zf.CRC32}, nil
	}
	return nil, keyNotFound(key)
}

func (s *sevenZipSource) Prefetch(context.Context, []string) error { return nil }
func (s *sevenZipSource) Close() error                             { return s.f.Close() }

// --- RAR ---

// rarSource indexes a RAR archive once and rescans it to read an entry, as
// entries of a solid archive can only be decoded in order.
type rarSource struct {
	f    *os.File
	rar5 bool
	keys []string
}

func openRar(f *os.File, rar5 bool) (*rarSource, error) {
	s := &rarSource{f: f, rar5: rar5}
	err := s.scan(func(key string, _ *rardecode.Reader) bool {
		s.keys = append(s.keys, key)
		return false
	})
	if err != nil {
		return nil, err
	}
	return s, nil
}

// scan walks the non-directory entries in order, calling fn with each key
// until it returns true.
func (s *rarSource) scan(fn func(key string, r *rardecode.Reader) bool) error {
	r, err := rardecode.NewReader(io.NewSectionReader(s.f, 0, 1<<62))
	if err != nil {
		return err
	}
	for {
		hdr, err := r.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := hdr.Name
		if !s.rar5 && hdr.Version > 0 {
			// rardecode strips a RAR 4 file version (";N"); SharpCompress
			// keeps it. (A ";0" suffix cannot be told apart and stays
			// stripped.)
			name += ";" + strconv.Itoa(hdr.Version)
		}
		if !hdr.IsDir && fn(s.key(name), r) {
			return nil
		}
	}
}

// key converts a rardecode name ('/'-separated, raw bytes for RAR 5 and
// non-Unicode RAR 4 names) to the SharpCompress key: decoded as UTF-8, then
// FileHeader.ConvertPathV4/V5 for the current OS. On Windows that makes '\'
// the separator, and a '\' inside a RAR 5 name becomes '-'.
func (s *rarSource) key(name string) string {
	name = netName(name)
	if os.PathSeparator != '\\' {
		return name // RAR 4 '\' separators are already '/'
	}
	if s.rar5 {
		name = strings.ReplaceAll(name, "\\", "-")
	}
	return strings.ReplaceAll(name, "/", "\\")
}

func (s *rarSource) Keys() []string { return append([]string(nil), s.keys...) }

func (s *rarSource) Open(key string) (io.ReadCloser, error) {
	if !slices.Contains(s.keys, key) {
		return nil, keyNotFound(key)
	}
	var found *rardecode.Reader
	err := s.scan(func(k string, r *rardecode.Reader) bool {
		if k == key {
			found = r
		}
		return found != nil
	})
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, keyNotFound(key)
	}
	return io.NopCloser(found), nil
}

func (s *rarSource) Prefetch(context.Context, []string) error { return nil }
func (s *rarSource) Close() error                             { return s.f.Close() }

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
// The expanded data is streamed, as its size is only bounded by the header.
func (s *tarSource) readSparse(key string) (io.ReadCloser, error) {
	tr := tar.NewReader(bufio.NewReader(io.NewSectionReader(s.f, 0, 1<<62)))
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
		return io.NopCloser(tr), nil
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

// gzipSource exposes a plain gzip as one entry keyed by the header's file
// name (FNAME), decoded as UTF-8 like SharpCompress does; without FNAME the
// entry's key is null in SharpCompress, so lip exposes no keys.
type gzipSource struct {
	path    string
	name    string
	hasName bool
}

func newGzipSource(path string) (*gzipSource, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	br := bufio.NewReader(f)
	if _, err := gzip.NewReader(br); err != nil {
		return nil, err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	br.Reset(f)
	name, hasName, err := gzipFileName(br)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &gzipSource{path: path, name: name, hasName: hasName}, nil
}

// gzipFileName reads the raw FNAME field of a gzip header (RFC 1952), which
// compress/gzip would decode as Latin-1 and cannot tell apart from an empty
// name.
func gzipFileName(r *bufio.Reader) (string, bool, error) {
	var hdr [10]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return "", false, err
	}
	const fextra, fname = 1 << 2, 1 << 3
	if hdr[3]&fextra != 0 {
		var xlen [2]byte
		if _, err := io.ReadFull(r, xlen[:]); err != nil {
			return "", false, err
		}
		if _, err := r.Discard(int(binary.LittleEndian.Uint16(xlen[:]))); err != nil {
			return "", false, err
		}
	}
	if hdr[3]&fname == 0 {
		return "", false, nil
	}
	raw, err := r.ReadBytes(0)
	if err != nil {
		return "", false, err
	}
	return netName(string(raw[:len(raw)-1])), true, nil
}

func (s *gzipSource) Keys() []string {
	if !s.hasName {
		return nil
	}
	return []string{s.name}
}

func (s *gzipSource) Open(key string) (io.ReadCloser, error) {
	if !s.hasName || key != s.name {
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
