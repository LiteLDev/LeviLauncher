package fetch

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"io"
	"runtime"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// xzWithDictProp returns a single-block xz stream of content whose LZMA2
// dictionary size property is p (with the block header CRC fixed up).
func xzWithDictProp(t *testing.T, content []byte, p byte) []byte {
	t.Helper()
	var b bytes.Buffer
	w, err := xz.NewWriter(&b)
	if err != nil {
		t.Fatal(err)
	}
	w.Write(content)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	s := b.Bytes()
	h := s[12 : 12+(int(s[12])+1)*4]
	body := h[:len(h)-4]
	i := bytes.LastIndex(body, []byte{0x21, 1}) // LZMA2, 1-byte properties
	if i < 0 {
		t.Fatalf("no LZMA2 filter in block header %x", h)
	}
	body[i+2] = p
	binary.LittleEndian.PutUint32(h[len(h)-4:], crc32.ChecksumIEEE(body))
	return s
}

// readEntryAlloc opens and reads one entry, returning its content and the
// bytes allocated meanwhile.
func readEntryAlloc(t *testing.T, s AssetSource, key string) (string, uint64, error) {
	t.Helper()
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	rc, err := s.Open(key)
	if err != nil {
		return "", 0, err
	}
	defer rc.Close()
	got, err := io.ReadAll(rc)
	runtime.ReadMemStats(&after)
	return string(got), after.TotalAlloc - before.TotalAlloc, err
}

func TestZipDecoderDictionaryBounded(t *testing.T) {
	content := []byte("hello, small entry")
	xz4G := xzWithDictProp(t, content, 40) // 4 GiB - 1
	xz1G := xzWithDictProp(t, content, 36) // 1 GiB
	// A zstd frame header asking for a 512 MiB window, then one raw block.
	zs := []byte{0x28, 0xB5, 0x2F, 0xFD, 0x00, 19 << 3, 1 | 5<<3, 0, 0, 'h', 'e', 'l', 'l', 'o'}
	s, err := OpenArchive("zip", zipWith(t, []*zip.FileHeader{
		rawHeader("xz4g", zipXZ, content, xz4G),
		rawHeader("xz1g", zipXZ, content, xz1G),
		rawHeader("zstd", zipZstd, []byte("hello"), zs),
	}, [][]byte{xz4G, xz1G, zs}))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, c := range []struct{ key, want, err string }{
		// SharpCompress rejects property 40 ("invalid dictionary size").
		{"xz4g", "", "invalid LZMA2 dictionary size property 40"},
		{"xz1g", string(content), ""},
		{"zstd", "hello", ""},
	} {
		got, grew, err := readEntryAlloc(t, s, c.key)
		if c.err == "" && err != nil || c.err != "" && (err == nil || !strings.Contains(err.Error(), c.err)) {
			t.Errorf("%s: err = %v, want %q", c.key, err, c.err)
		} else if got != c.want {
			t.Errorf("%s: content = %q, want %q", c.key, got, c.want)
		}
		if grew > 16<<20 {
			t.Errorf("%s: reading allocated %d MiB", c.key, grew>>20)
		}
	}
}

func TestZipDecoderDictionaryAboveLimit(t *testing.T) {
	// Entries declared larger than the limit whose streams ask for more
	// than the limit are rejected instead of allocated.
	const size = 300 << 20
	lz := append([]byte{16, 2, 5, 0, 0x5D}, binary.LittleEndian.AppendUint32(nil, 512<<20)...)
	lz = append(lz, 0, 0, 0, 0)
	xs := xzWithDictProp(t, []byte("x"), 38) // 2 GiB
	window := []byte{0x28, 0xB5, 0x2F, 0xFD, 0x00, 19 << 3, 1 | 1<<3, 0, 0, 'x'}
	single := append([]byte{0x28, 0xB5, 0x2F, 0xFD, 0xE0}, binary.LittleEndian.AppendUint64(nil, size)...)
	single = append(single, 1|1<<3, 0, 0, 'x')
	var entries []*zip.FileHeader
	data := [][]byte{lz, xs, window, single}
	for i, m := range []uint16{zipLZMA, zipXZ, zipZstd, zipZstd} {
		h := rawHeader(string(rune('a'+i)), m, nil, data[i])
		h.UncompressedSize64 = size
		entries = append(entries, h)
	}
	s, err := OpenArchive("zip", zipWith(t, entries, data))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for i, want := range []string{
		"LZMA dictionary of 300 MiB exceeds the 256 MiB limit",
		"xz: LZMA2 dictionary of 384 MiB exceeds the 256 MiB limit",
		"zstd: frame window of 512 MiB exceeds the 256 MiB limit",
		"zstd: frame window of 300 MiB exceeds the 256 MiB limit",
	} {
		_, grew, err := readEntryAlloc(t, s, entries[i].Name)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("entry %d: err = %v, want %q", i, err, want)
		}
		if grew > 16<<20 {
			t.Errorf("entry %d: allocated %d MiB", i, grew>>20)
		}
	}
}

// bcjContent is the input of xzX86BCJ: an E8 (call) opcode with a small
// little-endian target every 16 bytes, so the x86 filter rewrites it.
func bcjContent() []byte {
	b := make([]byte, 4096)
	for i := range b {
		switch i % 16 {
		case 0:
			b[i] = 0xE8
		case 4:
			b[i] = 0
		default:
			b[i] = byte(i * 7 % 251)
		}
	}
	return b
}

// xzX86BCJ is bcjContent compressed by Python's lzma with filters
// [FILTER_X86, FILTER_LZMA2 preset 6] (an 8 MiB dictionary); the x86 filter
// changes 509 of its bytes.
const xzX86BCJ = "fd377a585a000004e6d6b44602010400210116000d86351fe00fff04e25d007402fdb36028b0a58387c24a1b7490d6d3283c8ecf9e2e6c3f97650089cf21acf4ac4c8d46c9869055140b3c4b352fa34a21d77720c3ba3f5dbd1908b76284b5816d87388c4d7dfb31406effb947394233d300ed6ef12a201f2382099b5693867d28f74e7ce691748aecb647952b4623da4e838a36107f1a046bf583272f5b74e26e65816b846a3fa21b052b8d780d0411287d6ffa9c257788d8929b010fcd211deb340e60b1f4c37e74196a7e6a6dcb3107d306d42d72bfcf95a82b5f73a8d079d4e100cdd0e45287e8b3eece6a399a09081cb0d64619540d71dd03686dc499ac214f7dde9994a283aa1357e7967ae65242e2840af9ac28b5d1a292d78e0cf6596018a46c4b9afd731b3af8f031add5fa055cb8c469245d6b911c55d50ca820439ce306c984def0fb5d4110d5d86b212da5d18ab284d1575e1339a9ac5c490828b374bc6e9f458044d6188aa05a17284b45f4d3d904642557862fc5d38f0cc071720324ae32eb4b74ed2233eafd03c6bbd7715353d1240c8a1e1e6c84266f770efb683e2306c264e1302fcf3385ee9b7154efc7eac928193a2e92e2e601bba9e7b53c2d1df5ce32a7113380b17fcc2536374604b5261870233fd284d2d90b785de833024dfe86d813091f9b0df6d317a2a79c3303b2e46dd967c4170f70f4d6032ae5784176ed9855ba149bbb2a431f78ec1da3caf14eb070a558ab2b4d3b4d099cb0c3557936587c6af2cb78645ee5a11713f63a8c86bd83e716db69214814190136b939681c1e5092cb26540ee9ac4e364f9dba4155ab4db6f5689fae6d4866c92a49fc4041a8a9d3dea9051482c097c4804506f98e8397d3a4dd5d3e0afe5f27f6bc92c1d9bd08c4fa6b29fc0e0d55074af97f099c2b37fe0976e31249ad87b29f50bbc65b8ca4185dd23e56dfd0e8eb05bfb90ffa13f431357a563183fd222c55c5c572843df16992020e153747d4bd2a000995dadbbab8357099b58862f7dbd2c4f03656f3369c05eedf1c9c11e14bd7a97703cb741306499e37d757025f6244579eb55cd20dbb729aa831f294002be5086c02e1dabcb70f2e0d908f82e673f11da226dc5aab215c879761a3d60ffdeb1a9832ff1bd972550fd825ea0d0d6817f216794d834078fa678332e621071ae7efb65064774496c91df8d99287b82da493db772325ec470f8ba51b0f270531c1f74f9c601711f1be81184009a0deb30f5e3fd4a8cdea072d20cb2525f4e4f7c1bd509e32ff0b562ef4ec3df679d20e5030605f6f363b3e8ccb16bcabb7e9ab0f7bc1b3c5a977770bf8473f573471de0969de407f3863a45c49160bd7437143416eaa889115dde3dc97dcbc437dcf3ce3223310746e63944cf4a86ab929d890a7355e9682dd2e6380c27c5acaf73a57a58515bf1872c67142776f997d211bdf8d12ca69f9de7e86ab231cea1d4e8c87174b11aec6045b77459a3e217768873433bd7aec57576de0d7b140c956a1724497091214f2ee0b37c87aea403517ee66bad30bd16ec08ffd858102c41493a1c1dfece885f393134010c78b361f9deaef921bf4a1633bd4340d255f582d62de0858ba19d9044424226f395e5fb404bf9852a59598eb1c9f8d6d009f203c8d0977709f8806b5397d9f46a94222452c330de652f956e840b241ec3a2e365e7fbdfdd35c7dc79f628fbd352ecdb1e05f794c5235cf5082c2fad7dadf7f61ac6dda234bf72eebeff96fa4b2b48980fdfc371e254268544ffec99dbef58846109f3d04000000e6c357a595cac40c0001fe09802000001e3ec934b1c467fb020000000004595a"

// xzDelta is "delta" compressed by Python's lzma with filters
// [FILTER_DELTA dist 1, FILTER_LZMA2], which SharpCompress does not decode.
const xzDelta = "fd377a585a0000016922de3602010301002101167920c4ee01000464010708ed00000000d9fe439600011905bce8eccb9042990d010000000001595a"

func TestZipXZFilters(t *testing.T) {
	content := bcjContent()
	bcj, delta := mustHex(t, xzX86BCJ), mustHex(t, xzDelta)
	s, err := OpenArchive("zip", zipWith(t, []*zip.FileHeader{
		rawHeader("a.dll", zipXZ, content, bcj),
		rawHeader("delta", zipXZ, []byte("delta"), delta),
	}, [][]byte{bcj, delta}))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got, _, err := readEntryAlloc(t, s, "a.dll"); err != nil || got != string(content) {
		t.Errorf("a.dll: err = %v, content equal = %v", err, got == string(content))
	}
	if _, _, err := readEntryAlloc(t, s, "delta"); err == nil || !strings.Contains(err.Error(), "Delta filter is not supported") {
		t.Errorf("delta: err = %v, want the Delta filter rejected", err)
	}
}

func TestZipMultiBlockStreamsRewritten(t *testing.T) {
	// Several xz blocks and streams (with stream padding), and several
	// zstd frames (with a skippable frame), each asking for a large
	// dictionary: every header must be rewritten, or the decoder's own
	// limit rejects the entry.
	part := bytes.Repeat([]byte("multi block "), 500)
	content := append(append([]byte(nil), part...), part...)
	var xs bytes.Buffer
	for range 2 {
		w, err := xz.WriterConfig{DictCap: 64 << 20, BlockSize: 1000}.NewWriter(&xs)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(part)
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		xs.Write(make([]byte, 8))
	}
	enc, err := zstd.NewWriter(nil, zstd.WithSingleSegment(false))
	if err != nil {
		t.Fatal(err)
	}
	frame := enc.EncodeAll(part, nil)
	enc.Close()
	if frame[4]&0x20 != 0 {
		t.Fatalf("frame header %x: want a window descriptor", frame[:6])
	}
	frame[5] = 13 << 3 // an 8 MiB window, as a streaming encoder writes
	zs := append(append(append([]byte(nil), frame...), 0x50, 0x2A, 0x4D, 0x18, 3, 0, 0, 0, 'a', 'b', 'c'), frame...)
	s, err := OpenArchive("zip", zipWith(t, []*zip.FileHeader{
		rawHeader("xz", zipXZ, content, xs.Bytes()),
		rawHeader("zstd", zipZstd, content, zs),
	}, [][]byte{xs.Bytes(), zs}))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, key := range []string{"xz", "zstd"} {
		got, grew, err := readEntryAlloc(t, s, key)
		if err != nil || got != string(content) {
			t.Errorf("%s: err = %v, content equal = %v", key, err, got == string(content))
		}
		if grew > 16<<20 {
			t.Errorf("%s: reading allocated %d MiB", key, grew>>20)
		}
	}
}

func TestZstdHugeDeclaredSize(t *testing.T) {
	// A declared size near 2^64 must not overflow the window computation.
	frame := []byte{0x28, 0xB5, 0x2F, 0xFD, 0x00, 19 << 3, 1 | 1<<3, 0, 0, 'x'}
	rc, err := newZipZstdReader(bytes.NewReader(frame), 1<<63+1)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	if _, err := io.ReadAll(rc); err == nil || !strings.Contains(err.Error(), "frame window of 512 MiB exceeds") {
		t.Errorf("err = %v, want the 512 MiB window rejected", err)
	}
}
