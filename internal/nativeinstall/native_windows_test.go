//go:build windows

package nativeinstall

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"golang.org/x/crypto/xts"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"
)

func caught(f func()) (err error) {
	defer func() {
		if v := recover(); v != nil {
			if e, ok := v.(*Error); ok {
				err = e
			} else {
				panic(v)
			}
		}
	}()
	f()
	return nil
}
func unhex(s string) []byte {
	b, e := hex.DecodeString(s)
	if e != nil {
		panic(e)
	}
	return b
}
func TestRFC3394KnownAnswerAndTamper(t *testing.T) {
	kek := unhex("000102030405060708090a0b0c0d0e0f")
	wrapped := unhex("1fa68b0a8112b447aef34bd8fb5a7b829d3e862371d2cfe5")
	want := unhex("00112233445566778899aabbccddeeff")
	if got := unwrapKey(kek, wrapped); !bytes.Equal(got, want) {
		t.Fatal("RFC 3394 section 4.1 mismatch")
	}
	wrapped[11] ^= 1
	if caught(func() { unwrapKey(kek, wrapped) }) == nil {
		t.Fatal("tampered key wrap accepted")
	}
}
func TestXTSAgainstIndependentImplementation(t *testing.T) {
	// x/crypto/xts uses data-key then tweak-key; the MSIXVC key has the reverse order.
	key := make([]byte, 32)
	plain := make([]byte, 4096)
	for i := range key {
		key[i] = byte(i + 1)
	}
	for i := range plain {
		plain[i] = byte(i*17 + 3)
	}
	oracle, e := xts.NewCipher(aes.NewCipher, append(append([]byte{}, key[16:]...), key[:16]...))
	if e != nil {
		t.Fatal(e)
	}
	ciphertext := make([]byte, len(plain))
	oracle.Encrypt(ciphertext, plain, 0x12345678)
	tweak := make([]byte, 16)
	binary.LittleEndian.PutUint64(tweak, 0x12345678)
	decryptPage(ciphertext, key, tweak)
	if !bytes.Equal(ciphertext, plain) {
		t.Fatal("XTS key order, tweak, or GF arithmetic mismatch")
	}
}
func TestSPLicenseRejectsMalformedTLV(t *testing.T) {
	for _, input := range [][]byte{nil, make([]byte, 7), append(make([]byte, 8), 1), append(make([]byte, 8), []byte{1, 0, 0, 0, 255, 255, 255, 255}...), append(make([]byte, 8), []byte{1, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0}...)} {
		if caught(func() { parseSP(input) }) == nil {
			t.Fatalf("accepted malformed %d-byte TLV", len(input))
		}
	}
}
func TestOutputPathValidation(t *testing.T) {
	for _, path := range []string{"../save", "a/../b", "/absolute", "C:\\escape", "\\\\server\\share", "a:stream", "nul.txt", "COM1", "LPT9.log", "CONIN$", "COM¹.txt", "a.", "a ", "a//b", "a\x01b"} {
		if caught(func() { safePath(path) }) == nil {
			t.Errorf("accepted unsafe path %q", path)
		}
	}
	if got := safePath("data\\资源包\\texture.png"); got != filepath.Join("data", "资源包", "texture.png") {
		t.Fatal(got)
	}
}
func TestIdentifiersUseVDUIDNotXvcContentID(t *testing.T) {
	b := make([]byte, 20480)
	copy(b[512:], "msft-xvd")
	binary.LittleEndian.PutUint32(b[0x208:], 65)
	binary.LittleEndian.PutUint32(b[0x20c:], 3)
	binary.LittleEndian.PutUint64(b[0x218:], 4096)
	binary.LittleEndian.PutUint32(b[0x290:], 0xda8)
	for i := 0; i < 16; i++ {
		b[0x220+i] = 0xad
		b[16384+i] = 0xe2
		b[16400+i] = 0xba
	}
	binary.LittleEndian.PutUint16(b[16384+0xd1e:], 1)
	file := filepath.Join(t.TempDir(), "synthetic.msixvc")
	if e := os.WriteFile(file, b, 0600); e != nil {
		t.Fatal(e)
	}
	f, e := os.Open(file)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	content, key := readIdentifiers(f)
	if content != formatGUID(bytes.Repeat([]byte{0xad}, 16)) || key != formatGUID(bytes.Repeat([]byte{0xba}, 16)) {
		t.Fatal("wrong package identity fields")
	}
}
func TestCanceledInstallAndExistingOutputArePreserved(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "existing")
	if e := os.Mkdir(out, 0700); e != nil {
		t.Fatal(e)
	}
	marker := filepath.Join(out, "save.txt")
	if e := os.WriteFile(marker, []byte("preserve"), 0600); e != nil {
		t.Fatal(e)
	}
	_, err := Install(context.Background(), "nonexistent.msixvc", out, Options{})
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != "ERR_TARGET_EXISTS" {
		t.Fatalf("expected target-exists, got %v", err)
	}
	b, e := os.ReadFile(marker)
	if e != nil || string(b) != "preserve" {
		t.Fatal("existing output changed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	canceled := filepath.Join(dir, "must-not-exist")
	_, err = Install(ctx, "nonexistent.msixvc", canceled, Options{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if _, e := os.Stat(canceled); !os.IsNotExist(e) {
		t.Fatal("canceled install published output")
	}
}

func syntheticPackage(t *testing.T) (string, string) {
	t.Helper()
	const userOff = 16384
	const xvcOff = 20480
	const dataOff = 24576
	b := make([]byte, 36864)
	copy(b[512:], "msft-xvd")
	binary.LittleEndian.PutUint32(b[0x208:], 65)
	binary.LittleEndian.PutUint32(b[0x20c:], 3)
	binary.LittleEndian.PutUint64(b[0x218:], 12288)
	binary.LittleEndian.PutUint32(b[0x28c:], 4096)
	binary.LittleEndian.PutUint32(b[0x290:], 4096)
	for i := 0; i < 16; i++ {
		b[0x220+i] = 0xad
		b[xvcOff+16+i] = 0xba
	}
	u := b[userOff:xvcOff]
	binary.LittleEndian.PutUint32(u, 16)
	binary.LittleEndian.PutUint32(u[16+524:], 1)
	entry := u[16+528:]
	putText := func(dst []byte, s string) {
		for i, r := range utf16.Encode([]rune(s)) {
			binary.LittleEndian.PutUint16(dst[i*2:], r)
		}
	}
	putText(entry, "SegmentMetadata.bin")
	sm := u[1072:]
	binary.LittleEndian.PutUint32(entry[524:], 1056)
	binary.LittleEndian.PutUint32(sm[12:], 100)
	binary.LittleEndian.PutUint32(sm[16:], 3)
	names := []string{"MicrosoftGame.Config", "Minecraft.Windows.exe", "data/empty.txt"}
	lengths := []uint64{7, 1024, 0}
	pathOffset := 0
	for i, name := range names {
		p := 100 + i*16
		chars := utf16.Encode([]rune(name))
		binary.LittleEndian.PutUint16(sm[p+2:], uint16(len(chars)))
		binary.LittleEndian.PutUint32(sm[p+4:], uint32(pathOffset))
		binary.LittleEndian.PutUint64(sm[p+8:], lengths[i])
		putText(sm[148+pathOffset:], name)
		pathOffset += len(chars) * 2
	}
	binary.LittleEndian.PutUint32(entry[520:], uint32(148+pathOffset))
	x := b[xvcOff:dataOff]
	binary.LittleEndian.PutUint32(x[0xd10:], 2)
	binary.LittleEndian.PutUint32(x[0xd14:], 1)
	binary.LittleEndian.PutUint16(x[0xd1e:], 1)
	binary.LittleEndian.PutUint32(x[0xd3c:], 3)
	region := x[0xda8:]
	binary.LittleEndian.PutUint32(region, 1)
	binary.LittleEndian.PutUint16(region[4:], 65535)
	binary.LittleEndian.PutUint64(region[80:], dataOff)
	binary.LittleEndian.PutUint64(region[88:], 12288)
	for i := 0; i < 3; i++ {
		binary.LittleEndian.PutUint32(x[0xda8+128+i*12:], uint32(dataOff/4096+i))
	}
	copy(b[dataOff:], "<Game/>")
	executable := b[dataOff+4096 : dataOff+8192]
	copy(executable, "MZ")
	binary.LittleEndian.PutUint32(executable[0x3c:], 128)
	copy(executable[128:], "PE\x00\x00")
	coff := executable[132:]
	binary.LittleEndian.PutUint16(coff, 0x8664)
	binary.LittleEndian.PutUint16(coff[2:], 1)
	binary.LittleEndian.PutUint16(coff[16:], 240)
	optional := coff[20:]
	binary.LittleEndian.PutUint16(optional, 0x20b)
	binary.LittleEndian.PutUint32(optional[108:], 16)
	section := optional[240:]
	copy(section, ".text")
	binary.LittleEndian.PutUint32(section[8:], 1)
	binary.LittleEndian.PutUint32(section[12:], 4096)
	binary.LittleEndian.PutUint32(section[16:], 512)
	binary.LittleEndian.PutUint32(section[20:], 512)
	executable[512] = 0xc3
	tree := b[12288:16384]
	for i := 0; i < 5; i++ {
		digest := sha256.Sum256(b[userOff+i*4096 : userOff+(i+1)*4096])
		copy(tree[i*24:], digest[:20])
	}
	root := sha256.Sum256(tree)
	copy(b[0x240:], root[:])
	path := filepath.Join(t.TempDir(), "synthetic.msixvc")
	if e := os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
	return path, formatGUID(bytes.Repeat([]byte{0xba}, 16))
}

func TestExtractionTransactionAndEmptySegment(t *testing.T) {
	for _, mode := range []string{"success", "prepare-failure", "canceled", "corrupt-page"} {
		t.Run(mode, func(t *testing.T) {
			source, keyID := syntheticPackage(t)
			if mode == "corrupt-page" {
				f, e := os.OpenFile(source, os.O_WRONLY, 0)
				if e != nil {
					t.Fatal(e)
				}
				_, e = f.WriteAt([]byte{1}, 24576+4095)
				f.Close()
				if e != nil {
					t.Fatal(e)
				}
			}
			f, e := os.Open(source)
			if e != nil {
				t.Fatal(e)
			}
			defer f.Close()
			parent := t.TempDir()
			out := filepath.Join(parent, "result")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			active = &installation{ctx: ctx, source: f, out: out}
			defer func() { active = nil }()
			if mode == "prepare-failure" {
				active.options.Prepare = func(string) error { return errors.New("synthetic failure") }
			}
			if mode == "canceled" {
				active.options.Prepare = func(string) error { cancel(); return nil }
			}
			err := caught(func() { fullExtract(source, make([]byte, 32), keyID) })
			if mode == "success" {
				if err != nil {
					t.Fatal(err)
				}
				info, e := os.Stat(filepath.Join(out, "data", "empty.txt"))
				if e != nil || info.Size() != 0 {
					t.Fatal("empty segment was not extracted")
				}
				if active.report.FileCount != 3 || active.report.Bytes != 1031 {
					t.Fatal("incorrect manifest totals")
				}
			} else {
				if err == nil {
					t.Fatal("failure scenario unexpectedly succeeded")
				}
				if _, e := os.Stat(out); !os.IsNotExist(e) {
					t.Fatal("failed extraction published output")
				}
				children, e := os.ReadDir(parent)
				if e != nil || len(children) != 0 {
					t.Fatal("failed extraction left staging data")
				}
			}
		})
	}
}

func FuzzSPLicense(f *testing.F) {
	f.Add([]byte{})
	f.Add(make([]byte, 8))
	f.Add(append(make([]byte, 8), []byte{1, 0, 0, 0, 1, 0, 0, 0, 42}...))
	f.Fuzz(func(t *testing.T, data []byte) { _ = caught(func() { parseSP(data) }) })
}
