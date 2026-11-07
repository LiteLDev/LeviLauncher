package archive

import (
	"archive/zip"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"unicode/utf8"
)

// ZipValidator.cs: a file is a valid ZIP archive in the sense of
// protocol/package.md §1 when it parses as ZIP (decided by content only) and
// every entry path is '/'-separated, not absolute, without '..' segments or
// '\', and only regular file and directory entries are present.

const (
	unixFileTypeMask = 0xF000
	unixRegularFile  = 0x8000
	unixDirectory    = 0x4000
)

// ValidateZip validates the ZIP archive at path, naming it "The local file
// <path>" in the error.
func ValidateZip(path string) error {
	return ValidateZipFile(path, "The local file "+path)
}

// ValidateZipFile validates the ZIP archive at path. subject names the file
// as the subject of a sentence, e.g. "The file downloaded from
// https://example.com/a.zip". An invalid archive is an *UnavailableError; a
// failure to read the file is returned as it is.
func ValidateZipFile(path, subject string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	return ValidateZipReader(f, info.Size(), subject)
}

// ValidateZipReader validates the ZIP archive in r.
func ValidateZipReader(r io.ReaderAt, size int64, subject string) error {
	zr, err := newZipReader(r, size)
	if err != nil {
		if !isZipFormatError(err) {
			return err
		}
		return notZipError(subject, r, size, err)
	}
	return validateEntries(zr, subject)
}

func notZipError(subject string, r io.ReaderAt, size int64, err error) error {
	return &UnavailableError{
		Message: subject + " is not a ZIP archive" + guess(r, size) + ". lip reads packages and" +
			" assets only as ZIP files (not as .tar.gz, .7z or .rar).",
		Err: err,
	}
}

func validateEntries(zr *zip.Reader, subject string) error {
	for _, zf := range zr.File {
		name := netName(zf.Name)
		if reason := invalidReason(name, zf.ExternalAttrs); reason != "" {
			return &UnavailableError{Message: subject + " is not a valid archive: its entry '" + name +
				"' " + reason + ", which lip does not install for safety. Ask the package author to" +
				" create the archive with only regular files and directories at relative" +
				" '/'-separated paths."}
		}
	}
	return nil
}

// newZipReader parses the central directory. ErrInsecurePath (reported only
// with GODEBUG=zipinsecurepath=0) is not an error here: lip checks the paths
// itself.
func newZipReader(r io.ReaderAt, size int64) (*zip.Reader, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
		return nil, err
	}
	return zr, nil
}

// isZipFormatError reports whether err is a failure to parse the archive
// (.NET's InvalidDataException) rather than to read it.
func isZipFormatError(err error) bool {
	return errors.Is(err, zip.ErrFormat) || errors.Is(err, zip.ErrAlgorithm) ||
		errors.Is(err, zip.ErrChecksum) || errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF)
}

// guess says what the content is instead, when it is recognisable: e.g. a
// web page, which servers and proxies send instead of a file they do not
// have.
func guess(r io.ReaderAt, size int64) string {
	head := make([]byte, min(size, 4))
	n, err := r.ReadAt(head, 0)
	if err != nil && n < len(head) {
		return ""
	}
	head = head[:n]
	switch {
	case n == 0:
		return " (it is empty)"
	case head[0] == '<':
		return " (it is a web page, which servers send when the file is missing or access is denied)"
	case n >= 2 && head[0] == 0x1F && head[1] == 0x8B:
		return " (it is a gzip file)"
	case n >= 2 && head[0] == '7' && head[1] == 'z':
		return " (it is a 7z archive)"
	case n >= 4 && string(head) == "Rar!":
		return " (it is a RAR archive)"
	}
	return ""
}

func invalidReason(path string, externalAttrs uint32) string {
	switch {
	case path == "":
		return "has an empty path"
	case strings.Contains(path, `\`):
		return `contains '\'`
	case strings.HasPrefix(path, "/"):
		return "starts with '/'"
	case slices.Contains(strings.Split(path, "/"), ".."):
		return "contains a '..' segment"
	}
	// The upper 16 bits hold the Unix mode when the entry was made on Unix;
	// they are 0 otherwise.
	switch (externalAttrs >> 16) & unixFileTypeMask {
	case 0, unixRegularFile, unixDirectory:
		return ""
	}
	return "is neither a regular file nor a directory (e.g. a symbolic link)"
}

// netName converts archive entry name bytes the way .NET's UTF-8 decoder
// (Encoding.UTF8.GetString) does: each maximal subpart of an ill-formed
// sequence becomes one U+FFFD.
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
