package peeditor

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"

	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

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
