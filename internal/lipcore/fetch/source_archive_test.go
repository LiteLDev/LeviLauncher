package fetch

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"encoding/hex"
	"hash/crc32"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The expectations below were checked against SharpCompress 0.50.5
// (ArchiveFactory.OpenArchive, as lip's ArchiveSource uses it) on Windows.

func TestNetNameMaximalSubpart(t *testing.T) {
	const r = "\uFFFD"
	for in, want := range map[string]string{
		"plain/ünï.txt":             "plain/ünï.txt",
		"\xE4\xA1\xD6\xD0.txt":      r + r + r + ".txt", // E4 A1 is one subpart
		"\xB2\xE5\xBC\xFE":          r + r + r,          // GBK 插件: E5 BC is one subpart
		"\xE0\x80":                  r + r,              // E0 needs A0..BF next
		"\xED\xA0\x80":              r + r + r,          // surrogate
		"\xF0\x90\x80":              r,                  // truncated 4-byte sequence
		"\xF4\x90\x80\x80":          r + r + r + r,      // above U+10FFFF
		"\xC0\xAF":                  r + r,              // overlong
		"\xF5x":                     r + "x",
		"a\xEF\xBF\xBD\xC3":         "a" + r + r,
		"\xF1\x80\x80\xE1\x80x\x80": r + r + "x" + r,
	} {
		if got := netName(in); got != want {
			t.Errorf("netName(%q) = %q, want %q", in, got, want)
		}
	}
}

// zipWith writes a zip whose entries are given as headers with raw
// (already compressed) data.
func zipWith(t *testing.T, entries []*zip.FileHeader, data [][]byte) string {
	t.Helper()
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for i, h := range entries {
		w, err := zw.CreateRaw(h)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(data[i])
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return writeTemp(t, "a.zip", b.Bytes())
}

func rawHeader(name string, method uint16, content, compressed []byte) *zip.FileHeader {
	return &zip.FileHeader{
		Name: name, Method: method, CRC32: crc32.ChecksumIEEE(content),
		CompressedSize64: uint64(len(compressed)), UncompressedSize64: uint64(len(content)),
	}
}

func unicodePathExtra(raw, name string) []byte {
	field := []byte{1}
	field = binary.LittleEndian.AppendUint32(field, crc32.ChecksumIEEE([]byte(raw)))
	field = append(field, name...)
	extra := binary.LittleEndian.AppendUint16(nil, zipUnicodePathExtra)
	extra = binary.LittleEndian.AppendUint16(extra, uint16(len(field)))
	return append(extra, field...)
}

func TestZipUnicodePathExtraField(t *testing.T) {
	gbk := "\xD6\xD0.txt" // 中.txt in GBK, without the UTF-8 flag
	h1 := rawHeader(gbk, zip.Store, []byte("A"), []byte("A"))
	h1.NonUTF8 = true
	// An unrelated extra field first, then the Unicode Path field.
	h1.Extra = append([]byte{0xfe, 0xca, 2, 0, 9, 9}, unicodePathExtra(gbk, "中.txt")...)
	h2 := rawHeader("\xD6\xD0.bin", zip.Store, []byte("B"), []byte("B"))
	h2.NonUTF8 = true
	s, err := OpenArchive("zip", zipWith(t, []*zip.FileHeader{h1, h2}, [][]byte{[]byte("A"), []byte("B")}))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	checkSource(t, s, []struct{ key, content string }{
		{"中.txt", "A"},
		{"\uFFFD\uFFFD.bin", "B"}, // D6 and D0 are separate maximal subparts
	})
}

func TestZipBackslashDirectoryEntry(t *testing.T) {
	// .NET Framework 4.5 ZipFile.CreateFromDirectory writes directories as
	// empty entries ending in '\'; SharpCompress treats those as
	// directories, but not a '\' entry with (compressed) data.
	empty := rawHeader("dir\\", zip.Store, nil, nil)
	file := rawHeader("dir\\a.txt", zip.Store, []byte("a"), []byte("a"))
	deflated := rawHeader("d2\\", zip.Deflate, nil, []byte{3, 0})
	nonEmpty := rawHeader("x\\", zip.Store, []byte("x"), []byte("x"))
	s, err := OpenArchive("zip", zipWith(t,
		[]*zip.FileHeader{empty, file, deflated, nonEmpty},
		[][]byte{nil, []byte("a"), {3, 0}, []byte("x")}))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	checkSource(t, s, []struct{ key, content string }{
		{"dir\\a.txt", "a"},
		{"d2\\", ""},
		{"x\\", "x"},
	})
}

// lzmaZipData compresses content as a zip LZMA entry (with or without an end
// marker).
func lzmaZipData(t *testing.T, content []byte, eos bool) []byte {
	t.Helper()
	var b bytes.Buffer
	cfg := lzma.WriterConfig{EOSMarker: eos, Size: int64(len(content))}
	if eos {
		cfg.Size = -1
	}
	w, err := cfg.NewWriter(&b)
	if err != nil {
		t.Fatal(err)
	}
	w.Write(content)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	alone := b.Bytes() // properties, dictionary size, size, stream
	return append([]byte{16, 2, 5, 0}, append(alone[:5:5], alone[13:]...)...)
}

func TestZipCompressionMethods(t *testing.T) {
	content := bytes.Repeat([]byte("hello lip "), 500)
	var zs bytes.Buffer
	zw, _ := zstd.NewWriter(&zs)
	zw.Write(content)
	zw.Close()
	var xs bytes.Buffer
	xw, _ := xz.NewWriter(&xs)
	xw.Write(content)
	xw.Close()
	d64, d64Content := deflate64Fixed()
	bz := mustHex(t, "425a6839314159265359555a44f70000021980400010001264c0102000220069ea100305d3b62183c5dc914e14241556913dc0")

	lzEOS := rawHeader("lzma-eos.txt", zipLZMA, content, lzmaZipData(t, content, true))
	lzEOS.Flags |= 0x2
	entries := []*zip.FileHeader{
		rawHeader("bzip2.txt", zipBZip2, []byte("hello bzip2"), bz),
		lzEOS,
		rawHeader("lzma.txt", zipLZMA, content, lzmaZipData(t, content, false)),
		rawHeader("lzma-empty.txt", zipLZMA, nil, nil),
		rawHeader("zstd.txt", zipZstd, content, zs.Bytes()),
		rawHeader("xz.txt", zipXZ, content, xs.Bytes()),
		rawHeader("deflate64.txt", zipDeflate64, d64Content, d64),
	}
	data := [][]byte{bz, lzmaZipData(t, content, true), lzmaZipData(t, content, false), nil, zs.Bytes(), xs.Bytes(), d64}
	s, err := OpenArchive("zip", zipWith(t, entries, data))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	checkSource(t, s, []struct{ key, content string }{
		{"bzip2.txt", "hello bzip2"},
		{"lzma-eos.txt", string(content)},
		{"lzma.txt", string(content)},
		{"lzma-empty.txt", ""},
		{"zstd.txt", string(content)},
		{"xz.txt", string(content)},
		{"deflate64.txt", string(d64Content)},
	})
}

func TestZipPythonLZMA(t *testing.T) {
	// Python zipfile.ZIP_LZMA (end marker, flag bit 1) holding a.txt.
	p := writeTemp(t, "lz.zip", mustHex(t, "504b03043f0002000e006367665da7f3eaba250000004c04000005000000612e747874090405005d0000800000341949ee8de91487c42512434ed503858d64e8e1835fffe0958000504b01023f003f0002000e006367665da7f3eaba250000004c040000050000000000000000000000800100000000612e747874504b0506000000000100010033000000480000000000"))
	s, err := OpenArchive("zip", p)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	checkSource(t, s, []struct{ key, content string }{{"a.txt", strings.Repeat("hello lip ", 110)}})
}

func TestZipUnsupportedMethods(t *testing.T) {
	var entries []*zip.FileHeader
	var data [][]byte
	for _, m := range []uint16{1, 2, 6, 98, 77} {
		entries = append(entries, rawHeader(string(rune('a'+m%26)), m, []byte("x"), []byte("junk")))
		data = append(data, []byte("junk"))
	}
	s, err := OpenArchive("zip", zipWith(t, entries, data))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for i, want := range []string{"method 1 (Shrink)", "method 2 (Reduce)", "method 6 (Implode)", "method 98 (PPMd)", "unsupported compression algorithm (method 77)"} {
		_, err := s.Open(entries[i].Name)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Open(method %d) err = %v, want %q", entries[i].Method, err, want)
		}
	}
}

func TestZipChecksumVerified(t *testing.T) {
	var zs bytes.Buffer
	zw, _ := zstd.NewWriter(&zs)
	zw.Write([]byte("data"))
	zw.Close()
	bad := rawHeader("crc.txt", zipZstd, []byte("data"), zs.Bytes())
	bad.CRC32++
	short := rawHeader("size.txt", zipZstd, []byte("data!"), zs.Bytes())
	short.CRC32 = crc32.ChecksumIEEE([]byte("data"))
	s, err := OpenArchive("zip", zipWith(t, []*zip.FileHeader{bad, short}, [][]byte{zs.Bytes(), zs.Bytes()}))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for key, want := range map[string]error{"crc.txt": zip.ErrChecksum, "size.txt": io.ErrUnexpectedEOF} {
		rc, err := s.Open(key)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.ReadAll(rc); err != want {
			t.Errorf("read %s: err = %v, want %v", key, err, want)
		}
		rc.Close()
	}
}

// gzipRaw builds a gzip member by hand; fname nil omits FNAME.
func gzipRaw(t *testing.T, fname []byte, data []byte) []byte {
	t.Helper()
	b := []byte{0x1f, 0x8b, 8, 0, 0, 0, 0, 0, 0, 0xff}
	b[3] |= 1 << 2 // FEXTRA, skipped when reading the name
	b = append(b, 3, 0, 'a', 'b', 'c')
	if fname != nil {
		b[3] |= 1 << 3
		b = append(append(b, fname...), 0)
	}
	var c bytes.Buffer
	fw, _ := flate.NewWriter(&c, flate.DefaultCompression)
	fw.Write(data)
	fw.Close()
	b = append(b, c.Bytes()...)
	b = binary.LittleEndian.AppendUint32(b, crc32.ChecksumIEEE(data))
	return binary.LittleEndian.AppendUint32(b, uint32(len(data)))
}

func TestPlainGzipFileName(t *testing.T) {
	text := "not a tar"
	for _, tc := range []struct {
		fname []byte
		want  []string
	}{
		{nil, nil},               // no FNAME: SharpCompress key is null
		{[]byte{}, []string{""}}, // empty FNAME
		{[]byte("caf\xe9.txt"), []string{"caf\uFFFD.txt"}}, // decoded as UTF-8, not Latin-1
		{[]byte("café.txt"), []string{"café.txt"}},
	} {
		s, err := OpenArchive("tgz", writeTemp(t, "p.gz", gzipRaw(t, tc.fname, []byte(text))))
		if err != nil {
			t.Fatal(err)
		}
		if got := s.Keys(); !equal(got, tc.want) {
			t.Errorf("FNAME %q: keys = %q, want %q", tc.fname, got, tc.want)
		}
		for _, k := range tc.want {
			rc, err := s.Open(k)
			if err != nil {
				t.Fatal(err)
			}
			b, _ := io.ReadAll(rc)
			rc.Close()
			if string(b) != text {
				t.Errorf("content = %q", b)
			}
		}
		if tc.want == nil {
			if _, err := s.Open(""); err == nil {
				t.Error(`nameless gzip: Open("") succeeded`)
			}
		}
		s.Close()
	}
}

func TestTarSparseEntryStreamed(t *testing.T) {
	// A PAX 0.1 sparse file with a 256 MiB logical size and one data byte;
	// opening it must not expand it into memory.
	const size = 256 << 20
	var b bytes.Buffer
	pax := paxRecord("GNU.sparse.name", "big") + paxRecord("GNU.sparse.size", "268435456") +
		paxRecord("GNU.sparse.numblocks", "1") + paxRecord("GNU.sparse.map", "0,1")
	b.Write(rawTarHeader("PaxHeaders/big", 'x', len(pax)))
	b.Write(padBlock([]byte(pax)))
	b.Write(rawTarHeader("GNUSparseFile.0/big", '0', 1))
	b.Write(padBlock([]byte("Z")))
	b.Write(make([]byte, 1024))
	s, err := OpenArchive("tar", writeTemp(t, "big.tar", b.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	rc, err := s.Open("big")
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	head := make([]byte, 4)
	if _, err := io.ReadFull(rc, head); err != nil || string(head) != "Z\x00\x00\x00" {
		t.Fatalf("head = %q, %v", head, err)
	}
	runtime.ReadMemStats(&after)
	if grew := after.TotalAlloc - before.TotalAlloc; grew > 16<<20 {
		t.Errorf("Open allocated %d bytes", grew)
	}
	n, err := io.Copy(io.Discard, rc)
	if err != nil || n != size-4 {
		t.Errorf("rest = %d, %v", n, err)
	}
}

func TestSevenZipSource(t *testing.T) {
	// bodgit/sevenzip testdata t1.7z (LZMA2) and empty.7z (directories 01-05
	// and empty files 06-10).
	t1 := mustHex(t, "377abcaf271c000453a5f0c862000000000000002000000000000000c0cc85cc6261720a666f6f0a0000813307ae31986a96454d75138f0cdcb4c684fb5a0fa9dd2ecd99971c9ea3e1007be2f602a60f6aecab6e8dbde8277872e16e77f16ec96f9be091061505212a7b500232c12b21e923cad82f85387b832e9c8e91d07ec0000017060801095a00070b01000123030101055d001000000c660a01dd91f3f10000")
	empty := mustHex(t, "377abcaf271c0004515d7d0b4f000000000000002100000000000000261813a20000817c0c067e03564556f37dca71ca9c2a43892593262c51a30e4b1b533d0fa800da8a61513aff980bb59072f21a87cbebb0a3706c9afaa3b129ec05ff047a12eb9b607186b9ecb60d7b6000000017060001094f00070b01000123030101055d001000000c80ce0a012cdb8b270000")
	for name, tc := range map[string]struct {
		data []byte
		want []struct{ key, content string }
	}{
		"t1.7z":    {t1, []struct{ key, content string }{{"bar", "bar\n"}, {"foo", "foo\n"}}},
		"empty.7z": {empty, []struct{ key, content string }{{"06", ""}, {"07", ""}, {"08", ""}, {"09", ""}, {"10", ""}}},
	} {
		s, err := OpenArchive("zip", writeTemp(t, name, tc.data))
		if err != nil {
			t.Fatal(err)
		}
		checkSource(t, s, tc.want)
		s.Close()
	}
}

// rar5Archive builds a RAR 5 archive of stored entries by hand.
func rar5Archive(entries []struct {
	name string
	dir  bool
	data string
}) []byte {
	vint := func(b []byte, v uint64) []byte {
		for v >= 0x80 {
			b = append(b, byte(v)|0x80)
			v >>= 7
		}
		return append(b, byte(v))
	}
	block := func(body []byte, data string) []byte {
		h := vint(nil, uint64(len(body)))
		h = append(h, body...)
		out := binary.LittleEndian.AppendUint32(nil, crc32.ChecksumIEEE(h))
		return append(append(out, h...), data...)
	}
	out := []byte("Rar!\x1a\x07\x01\x00")
	out = append(out, block([]byte{1, 0, 0}, "")...) // main header, no flags
	for _, e := range entries {
		body := vint(nil, 2) // file header
		body = vint(body, 2) // has data area
		body = vint(body, uint64(len(e.data)))
		flags := uint64(4) // CRC32 present
		if e.dir {
			flags |= 1
		}
		body = vint(body, flags)
		body = vint(body, uint64(len(e.data)))
		body = vint(body, 0x20) // attributes
		body = binary.LittleEndian.AppendUint32(body, crc32.ChecksumIEEE([]byte(e.data)))
		body = vint(body, 0) // stored
		body = vint(body, 0) // Windows
		body = vint(body, uint64(len(e.name)))
		body = append(body, e.name...)
		out = append(out, block(body, e.data)...)
	}
	return append(out, block([]byte{5, 0, 0}, "")...) // end of archive
}

// rar4Archive builds a RAR 4 archive of stored entries by hand.
func rar4Archive(entries []struct {
	name string
	dir  bool
	data string
}) []byte {
	block := func(typ byte, flags uint16, rest []byte, data string) []byte {
		h := []byte{typ}
		h = binary.LittleEndian.AppendUint16(h, flags)
		h = binary.LittleEndian.AppendUint16(h, uint16(7+len(rest)))
		h = append(h, rest...)
		out := binary.LittleEndian.AppendUint16(nil, uint16(crc32.ChecksumIEEE(h)))
		return append(append(out, h...), data...)
	}
	out := []byte("Rar!\x1a\x07\x00")
	out = append(out, block(0x73, 0, make([]byte, 6), "")...)
	for _, e := range entries {
		var rest []byte
		rest = binary.LittleEndian.AppendUint32(rest, uint32(len(e.data))) // packed
		rest = binary.LittleEndian.AppendUint32(rest, uint32(len(e.data))) // unpacked
		rest = append(rest, 2)                                             // Windows
		rest = binary.LittleEndian.AppendUint32(rest, crc32.ChecksumIEEE([]byte(e.data)))
		rest = binary.LittleEndian.AppendUint32(rest, 0x21) // DOS time
		rest = append(rest, 20, 0x30)                       // version, stored
		rest = binary.LittleEndian.AppendUint16(rest, uint16(len(e.name)))
		rest = binary.LittleEndian.AppendUint32(rest, 0x20)
		rest = append(rest, e.name...)
		flags := uint16(0x8000) // has data
		if e.dir {
			flags |= 0xe0
		}
		if strings.Contains(e.name, ";") {
			flags |= 0x0800 // file version appended to the name
		}
		out = append(out, block(0x74, flags, rest, e.data)...)
	}
	return append(out, block(0x7b, 0x4000, nil, "")...)
}

func TestRarSource(t *testing.T) {
	type entry = struct {
		name string
		dir  bool
		data string
	}
	sep := func(s string) string {
		if os.PathSeparator == '\\' {
			return strings.ReplaceAll(s, "/", "\\")
		}
		return s
	}
	rar5 := rar5Archive([]entry{
		{"d", true, ""}, {"d/a.txt", false, "A"}, {"b.txt", false, "B"},
		{"x\\y.txt", false, "Y"}, {"\xE4\xA1.txt", false, "G"}, {"b.txt", false, "B2"},
	})
	back := "x\\y.txt" // a '\' in a RAR 5 name is not a separator
	if os.PathSeparator == '\\' {
		back = "x-y.txt"
	}
	rar4 := rar4Archive([]entry{{"d", true, ""}, {"d\\a.txt", false, "A"}, {"b.txt", false, "B"}, {"v.txt;2", false, "V"}})
	for name, tc := range map[string]struct {
		data []byte
		want []struct{ key, content string }
	}{
		"5.rar": {rar5, []struct{ key, content string }{
			{sep("d/a.txt"), "A"}, {"b.txt", "B"}, {back, "Y"}, {"\uFFFD.txt", "G"}, {"b.txt", "B"},
		}},
		"4.rar": {rar4, []struct{ key, content string }{{sep("d/a.txt"), "A"}, {"b.txt", "B"}, {"v.txt;2", "V"}}},
	} {
		s, err := OpenArchive("zip", writeTemp(t, name, tc.data))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		checkSource(t, s, tc.want)
		s.Close()
	}
}

func TestSevenZipAndRarDetected(t *testing.T) {
	for _, tc := range []struct {
		head string
		kind archiveKind
	}{
		{"7z\xbc\xaf\x27\x1c\x00\x04", kind7z},
		{"Rar!\x1a\x07\x00", kindRar4},
		{"Rar!\x1a\x07\x01\x00", kindRar5},
	} {
		f, err := os.Open(writeTemp(t, "x", []byte(tc.head)))
		if err != nil {
			t.Fatal(err)
		}
		kind, err := detect(f)
		f.Close()
		if err != nil || kind != tc.kind {
			t.Errorf("detect(%q) = %v, %v", tc.head, kind, err)
		}
	}
}
