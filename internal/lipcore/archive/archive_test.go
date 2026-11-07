package archive

import (
	"archive/zip"
	"bytes"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

type zipEntrySpec struct {
	name  string
	data  string
	attrs uint32
	dir   bool // no content written
}

// makeZip builds a ZIP archive as the upstream tests' Zip helper does.
func makeZip(t *testing.T, entries ...zipEntrySpec) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: e.name, Method: zip.Deflate, ExternalAttrs: e.attrs})
		if err != nil {
			t.Fatal(err)
		}
		if !e.dir {
			if _, err := w.Write([]byte(e.data)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func file(name, data string) zipEntrySpec { return zipEntrySpec{name: name, data: data} }

func readKey(t *testing.T, s Source, key string) string {
	t.Helper()
	rc, err := s.Open(key)
	if err != nil {
		t.Fatalf("Open(%q): %v", key, err)
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read %q: %v", key, err)
	}
	return string(b)
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// --- ZipValidator ---

func TestValidateZip_NotAZip_GuessesWhatItIs(t *testing.T) {
	tests := []struct {
		content, guess string
	}{
		{"", " (it is empty)"},
		{"<html>404</html>", " (it is a web page, which servers send when the file is missing or access is denied)"},
		{"\x1f\x8b\x08\x00rest", " (it is a gzip file)"},
		{"7z\xbc\xaf\x27\x1c", " (it is a 7z archive)"},
		{"Rar!\x1a\x07\x00", " (it is a RAR archive)"},
		{"not a zip", ""},
		{"<", " (it is a web page, which servers send when the file is missing or access is denied)"},
		{"Ra", ""},
	}
	for _, tt := range tests {
		path := filepath.Join(t.TempDir(), "pkg.zip")
		writeFile(t, path, []byte(tt.content))
		err := ValidateZip(path)
		var ue *UnavailableError
		if !errors.As(err, &ue) {
			t.Fatalf("%q: got %v, want UnavailableError", tt.content, err)
		}
		want := "The local file " + path + " is not a ZIP archive" + tt.guess + ". lip reads packages and" +
			" assets only as ZIP files (not as .tar.gz, .7z or .rar)."
		if ue.Error() != want {
			t.Errorf("%q:\n got %s\nwant %s", tt.content, ue.Error(), want)
		}
	}
}

func TestValidateZip_InvalidEntry_IsRejected(t *testing.T) {
	tests := []struct {
		name   string
		attrs  uint32
		reason string
	}{
		{"../evil.txt", 0, "contains a '..' segment"},
		{"a/../../evil.txt", 0, "contains a '..' segment"},
		{"a/..", 0, "contains a '..' segment"},
		{"/abs.txt", 0, "starts with '/'"},
		{`a\b.txt`, 0, `contains '\'`},
		{`\abs.txt`, 0, `contains '\'`},
		{"", 0, "has an empty path"},
		{"link", 0xA1FF0000, "is neither a regular file nor a directory (e.g. a symbolic link)"},
		{"fifo", 0x11A40000, "is neither a regular file nor a directory (e.g. a symbolic link)"},
	}
	for _, tt := range tests {
		b := makeZip(t, file("lip.yaml", "{}"), zipEntrySpec{name: tt.name, data: "x", attrs: tt.attrs})
		err := ValidateZipReader(bytes.NewReader(b), int64(len(b)), "The local file p.zip")
		want := "The local file p.zip is not a valid archive: its entry '" + tt.name + "' " + tt.reason +
			", which lip does not install for safety. Ask the package author to create the archive with" +
			" only regular files and directories at relative '/'-separated paths."
		if err == nil || err.Error() != want {
			t.Errorf("%q:\n got %v\nwant %s", tt.name, err, want)
		}
		if _, err := OpenZipBytes(b); err == nil {
			t.Errorf("%q: OpenZipBytes accepted it", tt.name)
		}
	}
}

func TestValidateZip_OnlyProtocolForbiddenPaths_AreRejected(t *testing.T) {
	// protocol/package.md §1 forbids only a leading '/', '..' segments and '\'.
	for _, name := range []string{"C:x", "./a", "a//b", ".", "a/./b", "...", "..a"} {
		b := makeZip(t, file("lip.yaml", "{}"), file(name, "x"))
		if err := ValidateZipReader(bytes.NewReader(b), int64(len(b)), "x"); err != nil {
			t.Errorf("%q: %v", name, err)
		}
	}
}

func TestOpenZip_DriveLetterEntry_KeysInEntryOrder(t *testing.T) {
	s, err := OpenZipBytes(makeZip(t, file("lip.yaml", "{}"), file("C:x", "x")))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Keys(); !slices.Equal(got, []string{"lip.yaml", "C:x"}) {
		t.Fatalf("keys %q", got)
	}
}

func TestOpenZip_UnixRegularFilesAndDirectories_Succeed(t *testing.T) {
	b := makeZip(t,
		zipEntrySpec{name: "lip.yaml", data: "{}", attrs: 0x81A40000},
		zipEntrySpec{name: "dir/", attrs: 0x41ED0000, dir: true})
	s, err := OpenZipBytes(b)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Keys(); !slices.Equal(got, []string{"lip.yaml"}) {
		t.Fatalf("keys %q", got)
	}
}

// --- ZipSource ---

func TestZipSource_ReadsFilesAndSizes(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.CreateHeader(&zip.FileHeader{Name: "stored.txt", Method: zip.Store})
	w.Write([]byte("stored"))
	w, _ = zw.CreateHeader(&zip.FileHeader{Name: "sub/deflated.txt", Method: zip.Deflate})
	w.Write([]byte(strings.Repeat("deflated", 100)))
	zw.Close()

	s, err := OpenZipBytes(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if got := readKey(t, s, "stored.txt"); got != "stored" {
		t.Errorf("stored: %q", got)
	}
	if got := readKey(t, s, "sub/deflated.txt"); got != strings.Repeat("deflated", 100) {
		t.Errorf("deflated: %q", got)
	}
	if n, ok := s.Size("sub/deflated.txt"); !ok || n != 800 {
		t.Errorf("Size = %d, %v", n, ok)
	}
	if _, ok := s.Size("missing"); ok {
		t.Error("Size of a missing key")
	}
}

func TestZipSource_DuplicateNames_FirstWins(t *testing.T) {
	s, err := OpenZipBytes(makeZip(t, file("a", "first"), file("b", "b"), file("a", "second"), zipEntrySpec{name: "d/", dir: true}))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Keys(); !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("keys %q", got)
	}
	if got := readKey(t, s, "a"); got != "first" {
		t.Fatalf("a = %q", got)
	}
}

func TestZipSource_MissingKey_ErrNoFile(t *testing.T) {
	s, err := OpenZipBytes(makeZip(t, file("a", "a"), zipEntrySpec{name: "d/", dir: true}))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"b", "d/"} {
		_, err := s.Open(key)
		if !errors.Is(err, ErrNoFile) || err.Error() != "The ZIP archive has no file '"+key+"'." {
			t.Errorf("%q: %v", key, err)
		}
	}
}

func rawZip(t *testing.T, name string, method uint16, compressed []byte, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateRaw(&zip.FileHeader{
		Name:               name,
		Method:             method,
		CRC32:              crc32.ChecksumIEEE(content),
		CompressedSize64:   uint64(len(compressed)),
		UncompressedSize64: uint64(len(content)),
	})
	if err != nil {
		t.Fatal(err)
	}
	w.Write(compressed)
	zw.Close()
	return buf.Bytes()
}

func TestZipSource_Deflate64_IsRead(t *testing.T) {
	stream, content := deflate64Fixed()
	s, err := OpenZipBytes(rawZip(t, "big.bin", zipDeflate64, stream, content))
	if err != nil {
		t.Fatal(err)
	}
	if got := readKey(t, s, "big.bin"); got != string(content) {
		t.Fatalf("decoded %d bytes, want %d", len(got), len(content))
	}
}

func TestZipSource_UnsupportedMethod_IsDamaged(t *testing.T) {
	s, err := OpenZipBytes(rawZip(t, "a.bin", 12 /* BZip2 */, []byte("BZh9"), []byte("x")))
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Open("a.bin")
	var de *DamagedError
	if !errors.As(err, &de) {
		t.Fatalf("got %v", err)
	}
	want := "Could not extract 'a.bin' from the ZIP archive: the archive is damaged, or the file is" +
		" compressed with a method lip does not support (only Deflate, Deflate64 and no compression are" +
		" supported). Ask the package author to create the archive with a standard ZIP tool."
	if err.Error() != want {
		t.Fatalf("got  %s\nwant %s", err, want)
	}
}

func TestZipSource_CorruptDeflate_IsDamagedWhenRead(t *testing.T) {
	s, err := OpenZipBytes(rawZip(t, "a.txt", zip.Deflate, []byte{0xff, 0xff, 0xff, 0xff}, []byte("abcd")))
	if err != nil {
		t.Fatal(err)
	}
	rc, err := s.Open("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	_, err = io.ReadAll(rc)
	var de *DamagedError
	if !errors.As(err, &de) {
		t.Fatalf("got %v", err)
	}
}

func TestOpenZipFile_ReadsAndValidates(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "pkg.bin")
	writeFile(t, good, makeZip(t, file("lip.yaml", "local"), file("sub/a.txt", "sub")))
	s, err := OpenZipFile(good)
	if err != nil {
		t.Fatal(err)
	}
	if got := readKey(t, s, "sub/a.txt"); got != "sub" {
		t.Fatalf("got %q", got)
	}
	_, err = s.Open("x")
	if err == nil || err.Error() != "The ZIP archive "+good+" has no file 'x'." {
		t.Fatalf("got %v", err)
	}

	bad := filepath.Join(dir, "bad.zip")
	writeFile(t, bad, makeZip(t, file("../evil.txt", "x")))
	if _, err := OpenZipFile(bad); err == nil || !strings.Contains(err.Error(), "not a valid archive") {
		t.Fatalf("got %v", err)
	}
}

func TestNewZipFileSource_DamagedCachedFile_AdvisesCacheClean(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cached")
	writeFile(t, path, []byte("garbage"))
	_, err := NewZipFileSource(path, "the download of https://x/a.zip ("+path+")")
	var de *DamagedError
	if !errors.As(err, &de) {
		t.Fatalf("got %v", err)
	}
	want := "The download of https://x/a.zip (" + path + ") is not a valid ZIP archive or is damaged." +
		" The download may have been damaged after it was stored: run 'lip cache clean' and try again." +
		" If this keeps happening, ask the package author to create the archive with a standard ZIP tool."
	if err.Error() != want {
		t.Fatalf("got  %s\nwant %s", err, want)
	}
}

// ArchiveProviderTests: local files (protocol/package.md §2.3).

func TestOpenLocalZip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pkg.zip")
	writeFile(t, path, makeZip(t, file("lip.yaml", "local"), file("sub/a.txt", "sub")))
	s, err := OpenLocalZip(path)
	if err != nil {
		t.Fatal(err)
	}
	// The bytes were read once: changing the file does not change the source.
	writeFile(t, path, []byte("changed"))
	if got := readKey(t, s, "lip.yaml"); got != "local" {
		t.Fatalf("got %q", got)
	}

	missing := filepath.Join(dir, "missing.zip")
	_, err = OpenLocalZip(missing)
	var ue *UnavailableError
	if !errors.As(err, &ue) || err.Error() != "There is no file or directory "+missing+". Check the path;"+
		" a relative path is relative to the current directory." {
		t.Fatalf("got %v", err)
	}

	notZip := filepath.Join(dir, "not.zip")
	writeFile(t, notZip, []byte("not a zip"))
	if _, err := OpenLocalZip(notZip); err == nil || !strings.Contains(err.Error(), "not a ZIP archive") {
		t.Fatalf("got %v", err)
	}

	for _, e := range []zipEntrySpec{{name: "../evil.txt"}, {name: `a\b.txt`}, {name: "link", attrs: 0xA1FF0000}} {
		invalid := filepath.Join(dir, "invalid.zip")
		writeFile(t, invalid, makeZip(t, file("lip.yaml", "{}"), e))
		if _, err := OpenLocalZip(invalid); !errors.As(err, &ue) || !strings.Contains(err.Error(), "not a valid archive") {
			t.Fatalf("%q: got %v", e.name, err)
		}
	}
}

// --- DirectorySource ---

func TestDirectorySource_KeysOrdinalAndReadWhereItIs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "lip.yaml"), []byte("v1"))
	writeFile(t, filepath.Join(dir, "lib", "a.dll"), []byte("a"))
	writeFile(t, filepath.Join(dir, "B.txt"), []byte("b"))
	writeFile(t, filepath.Join(dir, "lib", "deep", ".hidden"), []byte("h"))
	if err := os.MkdirAll(filepath.Join(dir, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := NewDirectorySource(dir + string(filepath.Separator))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := s.Keys(), []string{"B.txt", "lib/a.dll", "lib/deep/.hidden", "lip.yaml"}; !slices.Equal(got, want) {
		t.Fatalf("keys %q, want %q", got, want)
	}
	if n, ok := s.Size("lib/a.dll"); !ok || n != 1 {
		t.Fatalf("Size = %d, %v", n, ok)
	}
	writeFile(t, filepath.Join(dir, "lip.yaml"), []byte("v2"))
	if got := readKey(t, s, "lip.yaml"); got != "v2" {
		t.Fatalf("got %q", got)
	}
	if _, err := s.Open("lib"); !errors.Is(err, ErrNoFile) {
		t.Fatalf("got %v", err)
	}
}

func TestDirectorySource_LinksAreSkipped(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.txt"), []byte("s"))
	writeFile(t, filepath.Join(dir, "a.txt"), []byte("a"))
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(dir, "link.txt")); err != nil {
		t.Skipf("cannot create symbolic links: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "linkdir")); err != nil {
		t.Skipf("cannot create symbolic links: %v", err)
	}
	s, err := NewDirectorySource(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Keys(); !slices.Equal(got, []string{"a.txt"}) {
		t.Fatalf("keys %q", got)
	}
}

func TestCompareOrdinal_UTF16Order(t *testing.T) {
	// U+FF21 (one UTF-16 unit 0xFF21) sorts after U+1F600 (0xD83D 0xDE00).
	keys := []string{"\uFF21", "\U0001F600", "a", "ab", "B"}
	slices.SortFunc(keys, compareOrdinal)
	if want := []string{"B", "a", "ab", "\U0001F600", "\uFF21"}; !slices.Equal(keys, want) {
		t.Fatalf("got %q", keys)
	}
}

// --- MountSource ---

type fakeSource struct{ keys []string }

func (f fakeSource) Keys() []string { return f.keys }
func (f fakeSource) Open(key string) (io.ReadCloser, error) {
	if !slices.Contains(f.keys, key) {
		return nil, &noFileError{"no " + key}
	}
	return io.NopCloser(strings.NewReader("<" + key + ">")), nil
}
func (f fakeSource) Size(key string) (int64, bool) {
	return int64(len(key) + 2), slices.Contains(f.keys, key)
}

func TestMount_KeysUnderTheirMountsInOrder(t *testing.T) {
	m := Mount(fakeSource{[]string{"b", "a/c"}}, fakeSource{[]string{"d"}})
	if got := m.Keys(); !slices.Equal(got, []string{"0/b", "0/a/c", "1/d"}) {
		t.Fatalf("keys %q", got)
	}
}

func TestMount_OpenReadsFromItsSource(t *testing.T) {
	m := Mount(fakeSource{[]string{"a/c"}}, fakeSource{[]string{"d"}})
	if got := readKey(t, m, "0/a/c"); got != "<a/c>" {
		t.Fatalf("got %q", got)
	}
	if got := readKey(t, m, "1/d"); got != "<d>" {
		t.Fatalf("got %q", got)
	}
	if n, ok := m.Size("1/d"); !ok || n != 3 {
		t.Fatalf("Size = %d, %v", n, ok)
	}
	for _, key := range []string{"2/x", "1"} {
		_, err := m.Open(key)
		if !errors.Is(err, ErrNoFile) || err.Error() != "No asset of the package has the file '"+key+"'." {
			t.Fatalf("%q: %v", key, err)
		}
	}
}

// --- DeferredSource ---

func TestDefer_OpensOnceOnFirstUse(t *testing.T) {
	var calls atomic.Int32
	d := Defer(func() (Source, error) {
		calls.Add(1)
		return fakeSource{[]string{"a"}}, nil
	})
	if calls.Load() != 0 {
		t.Fatal("opened before use")
	}
	done := make(chan struct{})
	for range 8 {
		go func() {
			d.Keys()
			done <- struct{}{}
		}()
	}
	for range 8 {
		<-done
	}
	if got := readKey(t, d, "a"); got != "<a>" {
		t.Fatalf("got %q", got)
	}
	if calls.Load() != 1 {
		t.Fatalf("opened %d times", calls.Load())
	}
}

func TestDefer_FailureIsKept(t *testing.T) {
	var calls int
	boom := errors.New("boom")
	d := Defer(func() (Source, error) {
		calls++
		return nil, boom
	})
	if _, err := d.Open("a"); !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
	if keys := d.Keys(); keys != nil {
		t.Fatalf("keys %q", keys)
	}
	if _, _, err := ReadPackageFile(d); !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
	if _, _, err := ReadPackageFile(Mount(d)); !errors.Is(err, boom) {
		t.Fatalf("mount: got %v", err)
	}
	if calls != 1 {
		t.Fatalf("opened %d times", calls)
	}
}

// --- ArchiveReader ---

func TestReadPackageFile(t *testing.T) {
	tests := []struct {
		keys     []string
		name     string
		errorMsg string
	}{
		{[]string{"lip.yaml", "manifest.json"}, "lip.yaml", ""},
		{[]string{"tooth.json", "lip.yaml"}, "lip.yaml", ""},
		{[]string{"tooth.json"}, "tooth.json", ""},
		{[]string{"manifest.json"}, "", "pkg is not a lip package: it has no lip.yaml at its root."},
		{[]string{"a/b/lip.yaml"}, "", "pkg is not a lip package: it has no lip.yaml at its root."},
		{[]string{"a/b/lip.yaml", "sub/lip.yaml"}, "", "pkg is not a lip package: it has no lip.yaml at its root." +
			" There is one at 'sub/lip.yaml': the files of a package must be at the root of the archive, not in a folder."},
		{[]string{"repo-main/tooth.json", "x/lip.yaml"}, "", "pkg is not a lip package: it has no lip.yaml at its root." +
			" There is one at 'repo-main/tooth.json': the files of a package must be at the root of the archive, not in a folder."},
		{[]string{"sub/lip.yamlx", "/lip.yaml"}, "", "pkg is not a lip package: it has no lip.yaml at its root." +
			" There is one at '/lip.yaml': the files of a package must be at the root of the archive, not in a folder."},
	}
	for _, tt := range tests {
		name, data, err := ReadPackageFileOf(fakeSource{tt.keys}, "pkg")
		if tt.errorMsg != "" {
			var np *NotPackageError
			var ue *UnavailableError
			if !errors.As(err, &np) || !errors.As(err, &ue) || err.Error() != tt.errorMsg {
				t.Errorf("%q:\n got %v\nwant %s", tt.keys, err, tt.errorMsg)
			}
			continue
		}
		if err != nil || name != tt.name || string(data) != "<"+tt.name+">" {
			t.Errorf("%q: %q %q %v", tt.keys, name, data, err)
		}
	}
	if _, _, err := ReadPackageFile(fakeSource{nil}); err == nil ||
		err.Error() != "The archive is not a lip package: it has no lip.yaml at its root." {
		t.Fatalf("got %v", err)
	}
}

// --- Packer ---

func zipContents(t *testing.T, path string) map[string]string {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	out := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		out[f.Name] = string(b)
	}
	return out
}

func sortedKeys(m map[string]string) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func TestPack_PacksEveryFileWithoutAddingAManifest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "LegacyMoney.dll"), []byte("dll"))
	writeFile(t, filepath.Join(root, "lang", "en_US.json"), []byte("{}"))
	output := filepath.Join(root, "out.mcmod")
	if err := Pack(root, output); err != nil {
		t.Fatal(err)
	}
	got := zipContents(t, output)
	if keys := sortedKeys(got); !slices.Equal(keys, []string{"LegacyMoney.dll", "lang/en_US.json"}) {
		t.Fatalf("keys %q", keys)
	}
	if got["LegacyMoney.dll"] != "dll" || got["lang/en_US.json"] != "{}" {
		t.Fatalf("contents %q", got)
	}
	// The archive is valid for lip, in ordinal order.
	s, err := OpenZipFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if keys := s.Keys(); !slices.Equal(keys, []string{"LegacyMoney.dll", "lang/en_US.json"}) {
		t.Fatalf("entry order %q", keys)
	}
}

func TestPack_ExcludesOutputAndGitAndLipAtTheRootOnly(t *testing.T) {
	root := t.TempDir()
	for path, data := range map[string]string{
		"a.txt": "a", ".git/HEAD": "ref", ".lip/workspace.json": "{}", "sub/.git/HEAD": "nested",
		"sub/.lip/x": "nested", ".gitignore": "*.zip", "out.mcmod": "previous", ".gitx/y": "y",
	} {
		writeFile(t, filepath.Join(root, filepath.FromSlash(path)), []byte(data))
	}
	output := filepath.Join(root, "out.mcmod")
	if err := Pack(root, output); err != nil {
		t.Fatal(err)
	}
	if keys := sortedKeys(zipContents(t, output)); !slices.Equal(keys,
		[]string{".gitignore", ".gitx/y", "a.txt", "sub/.git/HEAD", "sub/.lip/x"}) {
		t.Fatalf("keys %q", keys)
	}
}

func TestPack_FileNamedLikeATemporaryOutput_IsPackedAndKept(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "out.mcmod")
	writeFile(t, output+".tmp", []byte("mine"))
	if err := Pack(root, output); err != nil {
		t.Fatal(err)
	}
	got := zipContents(t, output)
	if len(got) != 1 || got["out.mcmod.tmp"] != "mine" {
		t.Fatalf("got %q", got)
	}
}

func TestPack_OutputOutsideTheDirectory_ReplacesIt(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "build")
	writeFile(t, filepath.Join(root, "a.txt"), []byte("a"))
	output := filepath.Join(base, "dist", "out.zip")
	writeFile(t, output, []byte("previous"))
	if err := Pack(root, output); err != nil {
		t.Fatal(err)
	}
	if keys := sortedKeys(zipContents(t, output)); !slices.Equal(keys, []string{"a.txt"}) {
		t.Fatalf("keys %q", keys)
	}
}

func TestPack_NotADirectory_Fails(t *testing.T) {
	base := t.TempDir()
	missing := filepath.Join(base, "build")
	err := Pack(missing, filepath.Join(missing, "out.zip"))
	if err == nil || err.Error() != "Cannot pack "+missing+": there is no such directory. Check the path." {
		t.Fatalf("got %v", err)
	}
	f := filepath.Join(base, "file")
	writeFile(t, f, nil)
	err = Pack(f, filepath.Join(base, "out.zip"))
	if err == nil || err.Error() != "Cannot pack "+f+": it is a file, not a directory. Pass the directory of the package." {
		t.Fatalf("got %v", err)
	}
}

func TestZipSource_FileChanged_IsReadAgain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cached.zip")
	writeFile(t, path, makeZip(t, file("a", "old")))
	s, err := NewZipFileSource(path, "the download")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, makeZip(t, file("pad", strings.Repeat("p", 100)), file("a", "new content"), file("b", "b")))
	if got := readKey(t, s, "a"); got != "new content" {
		t.Fatalf("a = %q", got)
	}
	if got := s.Keys(); !slices.Equal(got, []string{"pad", "a", "b"}) {
		t.Fatalf("keys %q", got)
	}
}

func TestListKeys_ReportsTheFailureToOpen(t *testing.T) {
	boom := errors.New("boom")
	failing := Defer(func() (Source, error) { return nil, boom })
	if _, err := ListKeys(Mount(fakeSource{[]string{"a"}}, failing)); !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
	keys, err := ListKeys(Mount(fakeSource{[]string{"a"}}, Defer(func() (Source, error) { return fakeSource{[]string{"b"}}, nil })))
	if err != nil || !slices.Equal(keys, []string{"0/a", "1/b"}) {
		t.Fatalf("%q %v", keys, err)
	}
}

type unavailableSource struct{ fakeSource }

func (unavailableSource) Open(string) (io.ReadCloser, error) {
	return nil, &UnavailableError{Message: "specific"}
}

func TestReadPackageFile_UnavailableErrorPassesThrough(t *testing.T) {
	_, _, err := ReadPackageFileOf(unavailableSource{fakeSource{[]string{"lip.yaml"}}}, "pkg")
	if err == nil || err.Error() != "specific" {
		t.Fatalf("got %v", err)
	}
}
