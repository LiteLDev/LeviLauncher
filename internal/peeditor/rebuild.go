package peeditor

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Rebuild the headers and raw layout, as in LiteLDev/PeEditor's pe_bliss flow:
// https://github.com/LiteLDev/pe_bliss/blob/733644d6ee7db7625a057f85569db766fe0c7478/src/pe_rebuilder.cpp
// Existing section RVAs (including IATs) stay fixed. File offsets must move.
// Unlike a section-only writer, retain the overlay and fix its file references.
type peFileRange struct {
	old, size, new uint32
}

func alignedPEValue(value uint64, alignment uint32) (uint32, error) {
	value = (value + uint64(alignment) - 1) &^ (uint64(alignment) - 1)
	if value > math.MaxUint32 {
		return 0, fmt.Errorf("rebuilt PE exceeds 32-bit size limits")
	}
	return uint32(value), nil
}

func nextSectionRVA(v *peView) (uint32, error) {
	end := uint64(v.sizeOfImage)
	for _, s := range v.secs {
		end = max(end, uint64(s.vaddr)+uint64(max(s.vsize, s.rawSize)))
	}
	return alignedPEValue(end, v.sectionAlign)
}

func rebuildWithImportSection(data []byte, v *peView, blob []byte, newVA uint32) ([]byte, error) {
	if v.numSections >= 96 {
		return nil, fmt.Errorf("maximum PE section count reached")
	}
	newHeader := v.sectionHeaderTableOffset() + v.numSections*40
	headers, err := alignedPEValue(max(uint64(newHeader+40), uint64(v.sizeOfHeaders)), v.fileAlign)
	if err != nil {
		return nil, err
	}
	// Do not overwrite header-resident data to make room for the section table.
	if !isZeroBytes(data[newHeader:min(newHeader+40, int(v.sizeOfHeaders))]) {
		return nil, fmt.Errorf("new section header overlaps existing header data")
	}
	for i := uint32(0); i < v.numRvaAndSizes; i++ {
		if i == 4 { // The certificate directory uses a file offset, not an RVA.
			continue
		}
		d := v.dataDir + int(i)*8
		rva, size := binary.LittleEndian.Uint32(data[d:]), binary.LittleEndian.Uint32(data[d+4:])
		if size != 0 && rva < uint32(newHeader+40) && uint64(rva)+uint64(size) > uint64(newHeader) {
			return nil, fmt.Errorf("new section header overlaps data directory %d", i)
		}
	}
	ranges := []peFileRange{{0, v.sizeOfHeaders, 0}}
	oldEnd, cursor := v.sizeOfHeaders, headers
	for i, s := range v.secs {
		if headers > s.vaddr {
			return nil, fmt.Errorf("rebuilt headers (%d bytes) overlap section %d RVA 0x%x", headers, i, s.vaddr)
		}
		if s.rawSize == 0 {
			continue
		}
		// Low-alignment images require identical file offsets and RVAs.
		if v.sectionAlign < 0x1000 && cursor != s.vaddr {
			return nil, fmt.Errorf("cannot relayout low-alignment section %d without changing its RVA", i)
		}
		end := uint64(s.rawPtr) + uint64(s.rawSize)
		if s.rawPtr < v.sizeOfHeaders || end > uint64(len(data)) {
			return nil, fmt.Errorf("section %d raw data is outside the file body", i)
		}
		for _, r := range ranges[1:] {
			if uint64(s.rawPtr) < uint64(r.old)+uint64(r.size) && end > uint64(r.old) {
				return nil, fmt.Errorf("section %d raw data overlaps another section", i)
			}
		}
		ranges = append(ranges, peFileRange{s.rawPtr, s.rawSize, cursor})
		oldEnd = max(oldEnd, uint32(end))
		cursor, err = alignedPEValue(uint64(cursor)+uint64(s.rawSize), v.fileAlign)
		if err != nil {
			return nil, err
		}
	}
	newRaw := cursor
	if v.sectionAlign < 0x1000 && newRaw != newVA {
		return nil, fmt.Errorf("cannot place new section in a low-alignment image")
	}
	newRawSize, err := alignedPEValue(uint64(len(blob)), v.fileAlign)
	if err != nil {
		return nil, err
	}
	cursor, err = alignedPEValue(uint64(cursor)+uint64(newRawSize), v.fileAlign)
	if err != nil {
		return nil, err
	}
	// Preserve trailing certificates/debug data after the new section, including
	// their original alignment modulo 8 when the old last section was unaligned.
	if uint64(oldEnd) < uint64(len(data)) {
		cursor, err = alignedPEValue(uint64(cursor), 8)
		if err != nil {
			return nil, err
		}
		start := uint64(cursor) + uint64(oldEnd%8)
		size := uint64(len(data)) - uint64(oldEnd)
		cursor, err = alignedPEValue(start+size, 1)
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, peFileRange{oldEnd, uint32(size), uint32(start)})
	}
	result := make([]byte, int(cursor))
	for _, r := range ranges {
		copy(result[int(r.new):], data[int(r.old):int(uint64(r.old)+uint64(r.size))])
	}
	moveRange := func(off, size uint32) (uint32, error) {
		for _, r := range ranges {
			if off >= r.old && uint64(off) < uint64(r.old)+uint64(r.size) && uint64(off)+uint64(size) <= uint64(r.old)+uint64(r.size) {
				if r.old == 0 && off < uint32(newHeader+40) && uint64(off)+uint64(size) > uint64(newHeader) {
					break
				}
				return r.new + (off - r.old), nil
			}
		}
		return 0, fmt.Errorf("cannot relocate PE file range 0x%x+0x%x", off, size)
	}
	fixPointer := func(field int, size uint32) error {
		old := binary.LittleEndian.Uint32(result[field:])
		if old == 0 {
			return nil
		}
		moved, err := moveRange(old, max(size, 1))
		if err == nil {
			binary.LittleEndian.PutUint32(result[field:], moved)
		}
		return err
	}
	for i, s := range v.secs {
		h := v.sectionHeaderTableOffset() + i*40
		if s.rawSize == 0 {
			binary.LittleEndian.PutUint32(result[h+20:], 0)
		} else if err := fixPointer(h+20, s.rawSize); err != nil {
			return nil, err
		}
		if err := fixPointer(h+24, uint32(binary.LittleEndian.Uint16(result[h+32:]))*10); err != nil {
			return nil, err
		}
		if err := fixPointer(h+28, uint32(binary.LittleEndian.Uint16(result[h+34:]))*6); err != nil {
			return nil, err
		}
	}
	if err := fixPointer(v.coff+8, 1); err != nil { // COFF symbol table
		return nil, err
	}
	if v.numRvaAndSizes > 4 {
		d := v.dataDir + 4*8
		if err := fixPointer(d, binary.LittleEndian.Uint32(result[d+4:])); err != nil {
			return nil, err
		}
	}
	if v.numRvaAndSizes > 6 {
		d := v.dataDir + 6*8
		rva, size := binary.LittleEndian.Uint32(data[d:]), binary.LittleEndian.Uint32(data[d+4:])
		if size != 0 {
			off, ok := v.rvaToOffset(rva)
			if !ok || size%28 != 0 || uint64(off)+uint64(size) > uint64(len(data)) {
				return nil, fmt.Errorf("invalid PE debug directory")
			}
			moved, err := moveRange(uint32(off), size)
			if err != nil {
				return nil, err
			}
			for n := 0; n < int(size); n += 28 {
				h := int(moved) + n
				if err := fixPointer(h+24, binary.LittleEndian.Uint32(result[h+16:])); err != nil {
					return nil, err
				}
			}
		}
	}
	copy(result[int(newRaw):], blob)
	clear(result[newHeader : newHeader+40])
	copy(result[newHeader:], ".levildr")
	binary.LittleEndian.PutUint32(result[newHeader+8:], uint32(len(blob)))
	binary.LittleEndian.PutUint32(result[newHeader+12:], newVA)
	binary.LittleEndian.PutUint32(result[newHeader+16:], newRawSize)
	binary.LittleEndian.PutUint32(result[newHeader+20:], newRaw)
	binary.LittleEndian.PutUint32(result[newHeader+36:], imageScnCntInitializedData|imageScnMemRead|imageScnMemWrite)
	imageSize, err := alignedPEValue(uint64(newVA)+uint64(len(blob)), v.sectionAlign)
	if err != nil {
		return nil, err
	}
	initializedSize, err := alignedPEValue(uint64(binary.LittleEndian.Uint32(data[v.opt+8:]))+uint64(newRawSize), 1)
	if err != nil {
		return nil, err
	}
	binary.LittleEndian.PutUint32(result[v.opt+8:], initializedSize)
	binary.LittleEndian.PutUint32(result[v.opt+56:], imageSize)
	binary.LittleEndian.PutUint32(result[v.opt+60:], headers)
	binary.LittleEndian.PutUint16(result[v.coff+2:], uint16(v.numSections+1))
	return result, nil
}

// PE checksum is the folded 16-bit sum, excluding CheckSum, plus file length.
func peChecksum(data []byte, checksumOffset int) uint32 {
	var sum uint32
	for i := 0; i < len(data); i += 2 {
		if i >= checksumOffset && i < checksumOffset+4 {
			continue
		}
		word := uint32(data[i])
		if i+1 < len(data) {
			word |= uint32(data[i+1]) << 8
		}
		sum += word
		sum = (sum & 0xffff) + (sum >> 16)
	}
	sum = (sum & 0xffff) + (sum >> 16)
	return sum + uint32(len(data))
}
