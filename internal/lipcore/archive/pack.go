package archive

import (
	"archive/zip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Pack writes a ZIP archive of a directory to output, replacing it (lip pack,
// protocol/cli.md §5.11; Packer.cs): every regular file below dir at its
// path relative to it, except output itself (whatever path reaches it) and
// everything below .git/ and .lip/ at its root. Files are written in ordinal
// order of their paths; links are not followed, special files (FIFOs,
// sockets, device nodes) are left out, and an inaccessible directory, or a
// '\' in the name of a packed file or of a directory above it, fails before
// output is touched.
func Pack(dir, output string) error {
	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		if err == nil {
			return errors.New("Cannot pack " + root + ": it is a file, not a directory. Pass the directory" +
				" of the package.")
		}
		return errors.New("Cannot pack " + root + ": there is no such directory. Check the path.")
	}

	outputPath, err := filepath.Abs(output)
	if err != nil {
		return err
	}

	listed, err := listFiles(root)
	if err != nil {
		return &FileError{
			Message: "Cannot pack " + root + ": could not list the files in it. lip packs every file below" +
				" the directory, so it must be able to read every directory below it.",
			Err: err,
		}
	}
	// The same file can be reached by other paths (links, junctions, case,
	// 8.3 short names), and packing output would read it while it is being
	// written.
	outputIdentity, hasOutputIdentity := identityOf(outputPath)
	// Whether the entry is below the directory name at the root, also when it
	// is spelled in another case on a file system that ignores case: then
	// both name the same directory.
	isBelow := func(key, name string) bool {
		first := ""
		if slash := strings.IndexByte(key, '/'); slash >= 0 {
			first = key[:slash]
		}
		if first == name {
			return true
		}
		if !strings.EqualFold(first, name) {
			return false
		}
		a, ok := identityOf(filepath.Join(root, name))
		if !ok {
			return false
		}
		b, ok := identityOf(filepath.Join(root, first))
		return ok && a == b
	}
	var files []listedFile
	for _, f := range listed {
		if samePath(f.path, outputPath) || isBelow(f.key, ".git") || isBelow(f.key, ".lip") {
			continue
		}
		if hasOutputIdentity {
			if id, ok := identityOf(f.path); ok && id == outputIdentity {
				continue
			}
		}
		// Outside Windows '\' is not a separator, so a name may contain it; a
		// ZIP entry cannot (protocol/package.md §1), so the file cannot be
		// packed at its own path.
		if strings.ContainsRune(f.key, '\\') {
			return errors.New("Cannot pack " + root + ": '" + f.path + `' has a '\' in its path, which a ZIP` +
				" archive cannot hold. Rename it.")
		}
		files = append(files, f)
	}
	slices.SortFunc(files, func(a, b listedFile) int { return compareOrdinal(a.key, b.key) })

	out, err := os.Create(outputPath)
	if err != nil {
		return &FileError{
			Message: "Could not create the archive " + outputPath + ". Check that its directory exists and" +
				" that no other program has it open.",
			Err: err,
		}
	}
	writeErr := func(err error) error {
		return &FileError{Message: "Could not write the archive " + outputPath + ".", Err: err}
	}

	zw := zip.NewWriter(out)
	now := time.Now()
	for _, f := range files {
		if err := addFile(zw, f, outputPath, now); err != nil {
			zw.Close()
			out.Close()
			var fe *FileError
			if errors.As(err, &fe) {
				return err
			}
			return writeErr(err)
		}
	}
	// Closing the archive writes its central directory, which can fail too
	// (e.g. disk full).
	if err := zw.Close(); err != nil {
		out.Close()
		return writeErr(err)
	}
	if err := out.Close(); err != nil {
		return writeErr(err)
	}
	return nil
}

// addFile adds a file as .NET's ZipArchive.CreateEntry(name,
// CompressionLevel.Optimal) does: Deflate, the current local time as the
// MS-DOS modification time and no other attributes. A failure to read the
// file is a *FileError naming it.
func addFile(zw *zip.Writer, f listedFile, outputPath string, now time.Time) error {
	in, err := os.Open(f.path)
	if err != nil {
		return &FileError{Message: "Could not add " + f.path + " to the archive " + outputPath + ".", Err: err}
	}
	defer in.Close()
	date, tm := dosTime(now)
	w, err := zw.CreateHeader(&zip.FileHeader{
		Name:         f.key,
		Method:       zip.Deflate,
		ModifiedDate: date,
		ModifiedTime: tm,
	})
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, readerOnly{in}); err != nil {
		var pe *os.PathError
		if errors.As(err, &pe) && pe.Path == f.path {
			return &FileError{Message: "Could not add " + f.path + " to the archive " + outputPath + ".", Err: err}
		}
		return err
	}
	return nil
}

// readerOnly hides os.File's WriterTo, so that io.Copy's errors from reading
// stay *os.PathError.
type readerOnly struct{ r io.Reader }

func (r readerOnly) Read(p []byte) (int, error) { return r.r.Read(p) }

// dosTime is the MS-DOS date and time of t, as ZIP stores it.
func dosTime(t time.Time) (date, tm uint16) {
	if t.Year() < 1980 {
		t = time.Date(1980, 1, 1, 0, 0, 0, 0, time.Local)
	}
	date = uint16(t.Day() + int(t.Month())<<5 + (t.Year()-1980)<<9)
	tm = uint16(t.Second()/2 + t.Minute()<<5 + t.Hour()<<11)
	return date, tm
}

// samePath compares full paths as Packer does: ignoring case on Windows
// (OrdinalIgnoreCase), exactly elsewhere.
func samePath(a, b string) bool {
	if runtime.GOOS != "windows" {
		return a == b
	}
	for a != "" && b != "" {
		ra, na := utf8.DecodeRuneInString(a)
		rb, nb := utf8.DecodeRuneInString(b)
		if ra != rb && unicode.ToUpper(ra) != unicode.ToUpper(rb) {
			return false
		}
		a, b = a[na:], b[nb:]
	}
	return a == b
}
