package peeditor

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

// The section table ends at 1008, leaving only 16 bytes in a 1024-byte header.
// This reproduces the Minecraft 1.21.124.02 migration failure without a game binary.
func crowdedPE() []byte {
	const sections = 13
	data := make([]byte, 0x400+sections*0x200)
	put16 := func(off int, value uint16) { binary.LittleEndian.PutUint16(data[off:], value) }
	put32 := func(off int, value uint32) { binary.LittleEndian.PutUint32(data[off:], value) }
	copy(data, "MZ")
	put32(0x3c, 0xe0)
	copy(data[0xe0:], "PE\x00\x00")
	put16(0xe4, 0x8664)
	put16(0xe6, sections)
	put16(0xf4, 240)
	put16(0xf6, 0x22)
	const opt = 0xf8
	put16(opt, 0x20b)
	put32(opt+4, 0x200)
	put32(opt+8, (sections-1)*0x200)
	put32(opt+16, 0x1000)
	put32(opt+20, 0x1000)
	binary.LittleEndian.PutUint64(data[opt+24:], 0x140000000)
	put32(opt+32, 0x1000)
	put32(opt+36, 0x200)
	put32(opt+56, (sections+1)*0x1000)
	put32(opt+60, 0x400)
	put16(opt+68, 3)
	put32(opt+108, 16)
	put32(opt+120, 0x2000)
	put32(opt+124, 40)
	for i := 0; i < sections; i++ {
		h := 488 + i*40
		copy(data[h:], ".data")
		put32(h+8, 0x200)
		put32(h+12, uint32(i+1)*0x1000)
		put32(h+16, 0x200)
		put32(h+20, 0x400+uint32(i)*0x200)
		put32(h+36, 0xc0000040)
		for j := 0; j < 0x200; j++ {
			data[0x400+i*0x200+j] = byte(i + 1)
		}
	}
	clear(data[0x600:0x800])
	put32(0x600, 0x2060) // OriginalFirstThunk
	put32(0x60c, 0x2040) // Name
	put32(0x610, 0x2070) // FirstThunk
	copy(data[0x640:], "KERNEL32.dll\x00")
	binary.LittleEndian.PutUint64(data[0x660:], 0x2080)
	binary.LittleEndian.PutUint64(data[0x670:], 0x2080)
	copy(data[0x682:], "ExitProcess\x00")
	return data
}

func TestEnsureImportedDLLRebuildsCrowdedHeaders(t *testing.T) {
	original := crowdedPE()
	path := filepath.Join(t.TempDir(), "Minecraft.Windows.exe")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	added, err := EnsureImportedDLL(path, "LeviLauncher.dll", "LeviLauncherEntry")
	if err != nil || !added {
		t.Fatalf("patch crowded headers: added=%v err=%v", added, err)
	}
	patched, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f, err := pe.NewFile(bytes.NewReader(patched))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if h := f.OptionalHeader.(*pe.OptionalHeader64); h.SizeOfHeaders != 0x600 || h.SizeOfImage != 0xf000 {
		t.Fatalf("unexpected rebuilt headers: %+v", h)
	}
	if len(f.Sections) != 14 || f.Sections[13].Name != ".levildr" {
		t.Fatal("missing appended loader section")
	}
	for i, s := range f.Sections[:13] {
		if s.VirtualAddress != uint32(i+1)*0x1000 || s.Offset != 0x600+uint32(i)*0x200 {
			t.Fatalf("section %d has incorrect RVA or file offset: %+v", i, s.SectionHeader)
		}
		got, err := s.Data()
		if err != nil || !bytes.Equal(got, original[0x400+i*0x200:0x600+i*0x200]) {
			t.Fatalf("section %d content changed: %v", i, err)
		}
	}
	// debug/pe.ImportedSymbols assumes every import lives in the same section.
	// Read via its section metadata instead: old IAT/name RVAs intentionally stay
	// in .data while the rebuilt descriptors and new import live in .levildr.
	imports := peDataAtRVA(t, f, f.OptionalHeader.(*pe.OptionalHeader64).DataDirectory[1].VirtualAddress)
	if !bytes.Equal(imports[:20], original[0x600:0x614]) || !bytes.Equal(imports[40:60], make([]byte, 20)) {
		t.Fatal("original descriptor or import terminator changed")
	}
	name := peDataAtRVA(t, f, binary.LittleEndian.Uint32(imports[32:]))
	if !bytes.HasPrefix(name, []byte("LeviLauncher.dll\x00")) {
		t.Fatal("new DLL name is not mapped")
	}
	for _, field := range []int{20, 36} {
		thunks := peDataAtRVA(t, f, binary.LittleEndian.Uint32(imports[field:]))
		hintName := peDataAtRVA(t, f, uint32(binary.LittleEndian.Uint64(thunks)))
		if !bytes.HasPrefix(hintName, []byte("\x00\x00LeviLauncherEntry\x00")) || binary.LittleEndian.Uint64(thunks[8:]) != 0 {
			t.Fatal("new import lookup/address table is invalid")
		}
	}
	backup, err := os.ReadFile(path + ".orig")
	if err != nil || !bytes.Equal(backup, original) {
		t.Fatalf("original backup changed: %v", err)
	}
	added, err = EnsureImportedDLL(path, "LeviLauncher.dll", "LeviLauncherEntry")
	if err != nil || added {
		t.Fatalf("repeat patch: added=%v err=%v", added, err)
	}
	again, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(again, patched) {
		t.Fatalf("repeat patch changed executable: %v", err)
	}
	backup, err = os.ReadFile(path + ".orig")
	if err != nil || !bytes.Equal(backup, original) {
		t.Fatalf("repeat patch replaced original backup: %v", err)
	}
}

func peDataAtRVA(t *testing.T, f *pe.File, rva uint32) []byte {
	t.Helper()
	for _, s := range f.Sections {
		if rva >= s.VirtualAddress && rva-s.VirtualAddress < s.Size {
			data, err := s.Data()
			if err != nil {
				t.Fatal(err)
			}
			return data[rva-s.VirtualAddress:]
		}
	}
	t.Fatalf("RVA 0x%x does not map to section data", rva)
	return nil
}

func TestWritePatchedExecutablePreservesOriginal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Minecraft.Windows.exe")
	original := []byte("original image")
	patched := []byte("complete patched image")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	if err := writePatchedExecutable(path, patched); err != nil {
		t.Fatal(err)
	}
	for name, expected := range map[string][]byte{path: patched, path + ".orig": original} {
		got, err := os.ReadFile(name)
		if err != nil || !bytes.Equal(got, expected) {
			t.Fatalf("%s: got %q, want %q (err=%v)", name, got, expected, err)
		}
	}
	assertNoStagedImages(t, filepath.Dir(path))
}

func TestWritePatchedExecutableLeavesLockedOriginalIntact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Minecraft.Windows.exe")
	original := []byte("original image")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	// Allow reads and writes but deny replacement, as an in-use image can do.
	handle, err := windows.CreateFile(name, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil,
		windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	err = writePatchedExecutable(path, []byte("patched image"))
	if !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		t.Fatalf("expected sharing violation, got %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatalf("failed replacement changed the original: %q (err=%v)", got, err)
	}
	assertNoStagedImages(t, filepath.Dir(path))
}

func assertNoStagedImages(t *testing.T, dir string) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, ".leviloader-*.tmp"))
	if err != nil || len(files) != 0 {
		t.Fatalf("staged files were not cleaned up: %v (err=%v)", files, err)
	}
}
