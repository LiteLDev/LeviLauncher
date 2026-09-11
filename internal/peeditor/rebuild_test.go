package peeditor

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func patchTestPE(t *testing.T, original []byte) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.exe")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	if added, err := EnsureImportedDLL(path, "LeviLauncher.dll", "LeviLauncherEntry"); err != nil || !added {
		t.Fatalf("patch: added=%v err=%v", added, err)
	}
	patched, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return patched
}

func TestRebuildPreservesFilePointersAndOverlay(t *testing.T) {
	original := crowdedPE()
	oldEnd := uint32(len(original))
	original = append(original, bytes.Repeat([]byte{0x5a}, 131)...)
	put32 := func(off int, value uint32) { binary.LittleEndian.PutUint32(original[off:], value) }
	const directories = 0xf8 + 112
	put32(directories+4*8, oldEnd+32) // Certificate file offset, not RVA.
	put32(directories+4*8+4, 16)
	put32(0xe4+8, oldEnd+64)  // COFF symbol table
	put32(488+24, oldEnd+96)  // Section relocations
	put32(488+28, oldEnd+108) // Section line numbers
	put32(directories+6*8, 0x3100)
	put32(directories+6*8+4, 56)
	clear(original[0x900:0x938])
	put32(0x900+16, 8)
	put32(0x900+20, 0x3180)
	put32(0x900+24, 0x980) // Debug payload inside a section.
	put32(0x91c+16, 12)
	put32(0x91c+24, oldEnd+16) // Debug payload in overlay.
	patched := patchTestPE(t, original)
	get32 := func(off int) uint32 { return binary.LittleEndian.Uint32(patched[off:]) }
	newEnd := oldEnd + 0x400 // Header expansion plus new import section.
	for field, want := range map[int]uint32{
		directories + 4*8: newEnd + 32,
		0xe4 + 8:          newEnd + 64,
		488 + 24:          newEnd + 96,
		488 + 28:          newEnd + 108,
		0xb00 + 20:        0x3180, // RVA stays unchanged.
		0xb00 + 24:        0xb80,
		0xb1c + 24:        newEnd + 16,
	} {
		if got := get32(field); got != want {
			t.Errorf("field at 0x%x: got 0x%x, want 0x%x", field, got, want)
		}
	}
	if !bytes.Equal(patched[newEnd:], original[oldEnd:]) {
		t.Fatal("overlay bytes were lost or changed")
	}
	if !bytes.Equal(patched[0xb80:0xb88], original[0x980:0x988]) {
		t.Fatal("debug section payload changed")
	}
}

func TestRebuildChecksumMatchesWindows(t *testing.T) {
	proc := windows.NewLazySystemDLL("imagehlp.dll").NewProc("CheckSumMappedFile")
	for _, tail := range []int{0, 1, 7, 8} {
		t.Run(string(rune('0'+tail)), func(t *testing.T) {
			original := append(crowdedPE(), bytes.Repeat([]byte{0xf7}, tail)...)
			binary.LittleEndian.PutUint32(original[0xf8+64:], 0xdeadbeef)
			patched := patchTestPE(t, original)
			var stored, calculated uint32
			ntHeader, _, err := proc.Call(uintptr(unsafe.Pointer(&patched[0])), uintptr(len(patched)),
				uintptr(unsafe.Pointer(&stored)), uintptr(unsafe.Pointer(&calculated)))
			if ntHeader == 0 {
				t.Fatalf("CheckSumMappedFile: %v", err)
			}
			if stored != calculated {
				t.Fatalf("checksum: got 0x%x, Windows calculated 0x%x", stored, calculated)
			}
		})
	}
}

func TestRebuiltImageLoadsWithWindows(t *testing.T) {
	// A DLL with no entry point executes no fixture code. Let the actual Windows
	// loader validate the expanded headers, section layout and both import tables.
	original := crowdedPE()
	binary.LittleEndian.PutUint16(original[0xf6:], 0x2022)
	binary.LittleEndian.PutUint32(original[0xf8+16:], 0)
	path := filepath.Join(t.TempDir(), "fixture.dll")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	if added, err := EnsureImportedDLL(path, "VERSION.dll", "GetFileVersionInfoSizeW"); err != nil || !added {
		t.Fatalf("patch native fixture: added=%v err=%v", added, err)
	}
	handle, err := windows.LoadLibraryEx(path, 0, windows.LOAD_LIBRARY_SEARCH_SYSTEM32)
	if err != nil {
		t.Fatalf("Windows rejected the rebuilt image: %v", err)
	}
	if err := windows.FreeLibrary(handle); err != nil {
		t.Fatal(err)
	}
}

func TestRebuildWithHeaderRoomAndUninitializedSection(t *testing.T) {
	original := crowdedPE()
	// Remove the last raw section; its old bytes become an overlay. The section
	// table now has 56 spare bytes, so adding a header requires no expansion.
	binary.LittleEndian.PutUint16(original[0xe6:], 12)
	clear(original[488+12*40 : 1024])
	const bss = 488 + 11*40
	binary.LittleEndian.PutUint32(original[bss+16:], 0)
	binary.LittleEndian.PutUint32(original[bss+20:], 0)
	patched := patchTestPE(t, original)
	if got := binary.LittleEndian.Uint32(patched[0xf8+60:]); got != 1024 {
		t.Fatalf("unexpected header expansion: %d", got)
	}
	if binary.LittleEndian.Uint32(patched[bss+16:]) != 0 || binary.LittleEndian.Uint32(patched[bss+20:]) != 0 {
		t.Fatal("uninitialized section gained raw data")
	}
	if !bytes.Equal(patched[0x400:0x400+11*0x200], original[0x400:0x400+11*0x200]) {
		t.Fatal("existing initialized sections changed")
	}
	if !bytes.Equal(patched[len(patched)-1024:], original[len(original)-1024:]) {
		t.Fatal("trailing bytes were lost")
	}
}

func TestRebuildRejectsInvalidLayoutsWithoutWriting(t *testing.T) {
	put32 := func(b []byte, off int, value uint32) { binary.LittleEndian.PutUint32(b[off:], value) }
	for _, tc := range []struct {
		name, want string
		mutate     func([]byte)
	}{
		{"headers collide with RVA", "overlap section", func(b []byte) { put32(b, 488+12, 0x500) }},
		{"raw data past EOF", "outside the file body", func(b []byte) { put32(b, 488+12*40+20, uint32(len(b))+8) }},
		{"overlapping sections", "overlaps another section", func(b []byte) { put32(b, 488+12*40+20, 0x400) }},
		{"header payload", "overlaps existing header data", func(b []byte) { b[1008] = 1 }},
		{"directories outside optional header", "data directories exceed", func(b []byte) { put32(b, 0xf8+108, 17) }},
		{"missing import slot", "no import directory slot", func(b []byte) { put32(b, 0xf8+108, 1) }},
		{"non power of two alignment", "invalid alignment", func(b []byte) { put32(b, 0xf8+36, 513) }},
		{"file alignment exceeds section alignment", "invalid alignment", func(b []byte) { put32(b, 0xf8+36, 0x2000) }},
		{"low alignment would change mapping", "low-alignment section", func(b []byte) { put32(b, 0xf8+32, 0x200) }},
		{"RVA overflow", "32-bit size limits", func(b []byte) { put32(b, 0xf8+56, 0xffffffff) }},
		{"imports in virtual padding", "not mapped", func(b []byte) {
			put32(b, 488+40+8, 0x400)
			put32(b, 0xf8+120, 0x2300)
		}},
		{"truncated debug directory", "invalid PE debug directory", func(b []byte) {
			put32(b, 0xf8+112+6*8, 0x3100)
			put32(b, 0xf8+112+6*8+4, 27)
		}},
		{"unmapped certificate", "cannot relocate PE file range", func(b []byte) {
			put32(b, 0xf8+112+4*8, uint32(len(b))+8)
			put32(b, 0xf8+112+4*8+4, 16)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := crowdedPE()
			tc.mutate(original)
			path := filepath.Join(t.TempDir(), "fixture.exe")
			if err := os.WriteFile(path, original, 0644); err != nil {
				t.Fatal(err)
			}
			added, err := EnsureImportedDLL(path, "LeviLauncher.dll", "LeviLauncherEntry")
			if added || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got added=%v err=%v; want %q", added, err, tc.want)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(after, original) {
				t.Fatalf("failed rebuild changed executable: %v", err)
			}
			if _, err := os.Stat(path + ".orig"); !os.IsNotExist(err) {
				t.Fatalf("failed rebuild created a backup: %v", err)
			}
			assertNoStagedImages(t, filepath.Dir(path))
		})
	}
}
