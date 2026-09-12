package peeditor

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	imageScnCntInitializedData = 0x00000040
	imageScnMemRead            = 0x40000000
	imageScnMemWrite           = 0x80000000

	peMagicPE32Plus = 0x20b
)

func align32(n, a uint32) uint32 {
	if a == 0 {
		return n
	}
	return (n + a - 1) &^ (a - 1)
}

func isZeroBytes(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}

// peSection mirrors the fields of a section header needed for RVA mapping.
type peSection struct {
	vaddr, vsize, rawPtr, rawSize uint32
}

// peView holds the parsed offsets of a PE32+ image required for import editing.
type peView struct {
	coff           int
	opt            int
	sizeOfOptional int
	numSections    int
	sectionAlign   uint32
	fileAlign      uint32
	sizeOfImage    uint32
	sizeOfHeaders  uint32
	numRvaAndSizes uint32
	dataDir        int
	secs           []peSection
}

// parsePE parses the headers of a PE32+ image. It returns an error for any
// image that is not a well-formed 64-bit PE.
func parsePE(data []byte) (*peView, error) {
	if len(data) < 64 || data[0] != 'M' || data[1] != 'Z' {
		return nil, fmt.Errorf("not a PE file")
	}
	peOff := int(binary.LittleEndian.Uint32(data[60:64]))
	if peOff <= 0 || peOff+24 > len(data) {
		return nil, fmt.Errorf("bad PE header offset")
	}
	if data[peOff] != 'P' || data[peOff+1] != 'E' || data[peOff+2] != 0 || data[peOff+3] != 0 {
		return nil, fmt.Errorf("bad PE signature")
	}
	coff := peOff + 4
	numSections := int(binary.LittleEndian.Uint16(data[coff+2 : coff+4]))
	sizeOfOptional := int(binary.LittleEndian.Uint16(data[coff+16 : coff+18]))
	opt := coff + 20
	if sizeOfOptional < 112 || opt+sizeOfOptional > len(data) {
		return nil, fmt.Errorf("truncated optional header")
	}
	magic := binary.LittleEndian.Uint16(data[opt : opt+2])
	if magic != peMagicPE32Plus {
		return nil, fmt.Errorf("unsupported PE optional-header magic 0x%x (need PE32+)", magic)
	}
	if opt+112 > len(data) {
		return nil, fmt.Errorf("truncated PE32+ optional header")
	}

	v := &peView{
		coff:           coff,
		opt:            opt,
		sizeOfOptional: sizeOfOptional,
		numSections:    numSections,
		sectionAlign:   binary.LittleEndian.Uint32(data[opt+32 : opt+36]),
		fileAlign:      binary.LittleEndian.Uint32(data[opt+36 : opt+40]),
		sizeOfImage:    binary.LittleEndian.Uint32(data[opt+56 : opt+60]),
		sizeOfHeaders:  binary.LittleEndian.Uint32(data[opt+60 : opt+64]),
		numRvaAndSizes: binary.LittleEndian.Uint32(data[opt+108 : opt+112]),
		dataDir:        opt + 112,
	}
	if v.fileAlign == 0 || v.sectionAlign < v.fileAlign || v.fileAlign&(v.fileAlign-1) != 0 || v.sectionAlign&(v.sectionAlign-1) != 0 {
		return nil, fmt.Errorf("invalid alignment (file=%d section=%d)", v.fileAlign, v.sectionAlign)
	}
	if v.numRvaAndSizes > uint32((sizeOfOptional-112)/8) {
		return nil, fmt.Errorf("data directories exceed optional header")
	}

	secTableOff := opt + sizeOfOptional
	if numSections == 0 || numSections > 96 || uint64(v.sizeOfHeaders) > uint64(len(data)) || uint64(secTableOff+numSections*40) > uint64(v.sizeOfHeaders) {
		return nil, fmt.Errorf("invalid PE headers or section count")
	}
	for i := 0; i < numSections; i++ {
		h := secTableOff + i*40
		if h+40 > len(data) {
			return nil, fmt.Errorf("truncated section header %d", i)
		}
		v.secs = append(v.secs, peSection{
			vsize:   binary.LittleEndian.Uint32(data[h+8 : h+12]),
			vaddr:   binary.LittleEndian.Uint32(data[h+12 : h+16]),
			rawSize: binary.LittleEndian.Uint32(data[h+16 : h+20]),
			rawPtr:  binary.LittleEndian.Uint32(data[h+20 : h+24]),
		})
	}
	return v, nil
}

func (v *peView) sectionHeaderTableOffset() int { return v.opt + v.sizeOfOptional }

func (v *peView) rvaToOffset(rva uint32) (int, bool) {
	if rva < v.sizeOfHeaders {
		return int(rva), true
	}
	for _, s := range v.secs {
		// Only initialized, file-backed bytes have a file offset.
		if s.rawSize != 0 && rva >= s.vaddr && uint64(rva)-uint64(s.vaddr) < uint64(s.rawSize) {
			return int(uint64(s.rawPtr) + uint64(rva-s.vaddr)), true
		}
	}
	return 0, false
}

func readCString(data []byte, off int) string {
	if off < 0 || off >= len(data) {
		return ""
	}
	end := off
	for end < len(data) && data[end] != 0 {
		end++
	}
	return string(data[off:end])
}

// dllIsImported reports whether the image already imports dllName (by walking
// the import descriptor table to its null terminator).
func dllIsImported(data []byte, dllName string) (bool, error) {
	v, err := parsePE(data)
	if err != nil {
		return false, err
	}
	if v.numRvaAndSizes <= 1 {
		return false, nil
	}
	importRVA := binary.LittleEndian.Uint32(data[v.dataDir+8 : v.dataDir+12])
	if importRVA == 0 {
		return false, nil
	}
	off, ok := v.rvaToOffset(importRVA)
	if !ok {
		return false, fmt.Errorf("import directory RVA 0x%x not mapped", importRVA)
	}
	for {
		if off+20 > len(data) {
			return false, fmt.Errorf("truncated import descriptor")
		}
		desc := data[off : off+20]
		if isZeroBytes(desc) {
			return false, nil
		}
		nameRVA := binary.LittleEndian.Uint32(desc[12:16])
		if nameRVA != 0 {
			if nOff, ok := v.rvaToOffset(nameRVA); ok {
				if strings.EqualFold(readCString(data, nOff), dllName) {
					return true, nil
				}
			}
		}
		off += 20
	}
}

// EnsureImportedDLL makes exePath import funcName from dllName so the Windows
// loader loads dllName (and runs its DllMain) while snapping the executable's
// imports. It works by appending a new writable section that carries a rebuilt
// copy of the import descriptor table plus the thunk/name data for the new
// entry, then repointing the import data directory at the copy. Like PeEditor's
// pe_bliss rebuild_pe flow, it rebuilds the file layout and checksum, growing
// SizeOfHeaders and relocating raw section data while keeping existing RVAs.
//
// The operation is idempotent: if dllName is already imported it returns
// (false, nil) and leaves the file untouched. Only PE32+ (x64) images are
// supported.
func EnsureImportedDLL(exePath, dllName, funcName string) (bool, error) {
	data, err := os.ReadFile(exePath)
	if err != nil {
		return false, err
	}

	if present, err := dllIsImported(data, dllName); err != nil {
		return false, err
	} else if present {
		return false, nil
	}

	v, err := parsePE(data)
	if err != nil {
		return false, err
	}
	if v.numRvaAndSizes <= 1 {
		return false, fmt.Errorf("optional header has no import directory slot")
	}

	// Collect the existing descriptors verbatim; their OriginalFirstThunk,
	// Name and FirstThunk RVAs keep pointing at the original sections.
	var descriptors []byte
	nExisting := 0
	if v.numRvaAndSizes > 1 {
		if importRVA := binary.LittleEndian.Uint32(data[v.dataDir+8 : v.dataDir+12]); importRVA != 0 {
			off, ok := v.rvaToOffset(importRVA)
			if !ok {
				return false, fmt.Errorf("import directory RVA 0x%x not mapped", importRVA)
			}
			for {
				if off+20 > len(data) {
					return false, fmt.Errorf("truncated import descriptor")
				}
				desc := data[off : off+20]
				if isZeroBytes(desc) {
					break
				}
				descriptors = append(descriptors, desc...)
				nExisting++
				off += 20
			}
		}
	}

	// Lay out the new section:
	//   [existing descriptors][new descriptor][null descriptor][ILT][IAT][hint+name][dll name]
	newVA, err := nextSectionRVA(v)
	if err != nil {
		return false, err
	}

	descArraySize := uint32((nExisting + 2) * 20)
	iltOff := align32(descArraySize, 8)
	iatOff := iltOff + 16 // one 8-byte thunk + 8-byte null terminator
	hintOff := iatOff + 16

	hintName := make([]byte, 2) // hint = 0
	hintName = append(hintName, []byte(funcName)...)
	hintName = append(hintName, 0)
	if len(hintName)%2 != 0 {
		hintName = append(hintName, 0)
	}
	dllNameOff := hintOff + uint32(len(hintName))
	dllNameBytes := append([]byte(dllName), 0)
	if len(dllNameBytes)%2 != 0 {
		dllNameBytes = append(dllNameBytes, 0)
	}
	blobSize := dllNameOff + uint32(len(dllNameBytes))

	blob := make([]byte, blobSize)
	copy(blob[0:], descriptors)
	nd := nExisting * 20
	binary.LittleEndian.PutUint32(blob[nd+0:], newVA+iltOff)      // OriginalFirstThunk
	binary.LittleEndian.PutUint32(blob[nd+4:], 0)                 // TimeDateStamp
	binary.LittleEndian.PutUint32(blob[nd+8:], 0)                 // ForwarderChain
	binary.LittleEndian.PutUint32(blob[nd+12:], newVA+dllNameOff) // Name
	binary.LittleEndian.PutUint32(blob[nd+16:], newVA+iatOff)     // FirstThunk
	// The null descriptor after it stays zero.
	binary.LittleEndian.PutUint64(blob[iltOff:], uint64(newVA+hintOff))
	binary.LittleEndian.PutUint64(blob[iatOff:], uint64(newVA+hintOff))
	copy(blob[hintOff:], hintName)
	copy(blob[dllNameOff:], dllNameBytes)

	data, err = rebuildWithImportSection(data, v, blob, newVA)
	if err != nil {
		return false, err
	}

	// Patch header fields.
	binary.LittleEndian.PutUint32(data[v.dataDir+8:], newVA)                     // import dir RVA
	binary.LittleEndian.PutUint32(data[v.dataDir+12:], uint32((nExisting+2)*20)) // import dir size
	if v.numRvaAndSizes > 11 {                                                   // zero the bound-import directory
		binary.LittleEndian.PutUint32(data[v.dataDir+11*8:], 0)
		binary.LittleEndian.PutUint32(data[v.dataDir+11*8+4:], 0)
	}
	binary.LittleEndian.PutUint32(data[v.opt+64:], peChecksum(data, v.opt+64))

	if ok, err := dllIsImported(data, dllName); err != nil {
		return false, fmt.Errorf("post-patch verification failed to parse: %w", err)
	} else if !ok {
		return false, fmt.Errorf("post-patch verification did not find %s in import table", dllName)
	}

	if err := writePatchedExecutable(exePath, data); err != nil {
		return false, err
	}
	return true, nil
}

var procReplaceFile = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReplaceFileW")

// writePatchedExecutable stages the complete image before replacing the game.
// ReplaceFileW preserves the original file's ACLs and saves it as .orig.
func writePatchedExecutable(exePath string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(exePath), ".leviloader-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	original, err := windows.UTF16PtrFromString(exePath)
	if err != nil {
		return err
	}
	replacement, err := windows.UTF16PtrFromString(tmp.Name())
	if err != nil {
		return err
	}
	backup, err := windows.UTF16PtrFromString(exePath + ".orig")
	if err != nil {
		return err
	}
	ok, _, err := procReplaceFile.Call(
		uintptr(unsafe.Pointer(original)),
		uintptr(unsafe.Pointer(replacement)),
		uintptr(unsafe.Pointer(backup)),
		0, 0, 0,
	)
	if ok == 0 {
		return fmt.Errorf("replace %s (backup: %s.orig): %w", exePath, exePath, err)
	}
	return nil
}
