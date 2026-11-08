package uwp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// zipEntry is a central directory record. Unlike archive/zip it exposes the
// local header offset, so file data can be addressed without reading every
// local header over the network.
type zipEntry struct {
	Name             string
	Method           uint16
	Flags            uint16
	CRC32            uint32
	CompressedSize   int64
	UncompressedSize int64
	HeaderOffset     int64
}

// zipDirectory is a parsed central directory. Raw holds the bytes from the
// start of the central directory to the end of the archive, which is what
// the AXCD digest of an Appx signature covers.
type zipDirectory struct {
	Entries  []zipEntry
	CDOffset int64
	Raw      []byte
}

const (
	sigEOCD        = 0x06054b50
	sigZip64EOCD   = 0x06064b50
	sigZip64Locate = 0x07064b50
	sigCentral     = 0x02014b50
	sigLocal       = 0x04034b50
	maxDirectory   = 64 << 20
)

func readZipDirectory(r io.ReaderAt, size int64) (*zipDirectory, error) {
	tailLen := int64(22 + 65535 + 20)
	if tailLen > size {
		tailLen = size
	}
	tail := make([]byte, tailLen)
	if _, err := r.ReadAt(tail, size-tailLen); err != nil && err != io.EOF {
		return nil, err
	}
	eocd := -1
	for i := len(tail) - 22; i >= 0; i-- {
		if binary.LittleEndian.Uint32(tail[i:]) == sigEOCD {
			eocd = i
			break
		}
	}
	if eocd < 0 {
		return nil, errors.New("zip end of central directory not found")
	}
	e := tail[eocd:]
	entries := int64(binary.LittleEndian.Uint16(e[10:]))
	cdSize := int64(binary.LittleEndian.Uint32(e[12:]))
	cdOffset := int64(binary.LittleEndian.Uint32(e[16:]))
	if entries == 0xFFFF || cdSize == 0xFFFFFFFF || cdOffset == 0xFFFFFFFF {
		if eocd < 20 || binary.LittleEndian.Uint32(tail[eocd-20:]) != sigZip64Locate {
			return nil, errors.New("zip64 locator not found")
		}
		z64Offset := int64(binary.LittleEndian.Uint64(tail[eocd-20+8:]))
		rec := make([]byte, 56)
		if _, err := r.ReadAt(rec, z64Offset); err != nil {
			return nil, err
		}
		if binary.LittleEndian.Uint32(rec) != sigZip64EOCD {
			return nil, errors.New("zip64 end of central directory not found")
		}
		entries = int64(binary.LittleEndian.Uint64(rec[32:]))
		cdSize = int64(binary.LittleEndian.Uint64(rec[40:]))
		cdOffset = int64(binary.LittleEndian.Uint64(rec[48:]))
	}
	if cdOffset < 0 || cdSize < 0 || cdOffset+cdSize > size || size-cdOffset > maxDirectory || entries > 200000 {
		return nil, errors.New("zip central directory out of range")
	}
	raw := make([]byte, size-cdOffset)
	if _, err := r.ReadAt(raw, cdOffset); err != nil && err != io.EOF {
		return nil, err
	}
	dir := &zipDirectory{CDOffset: cdOffset, Raw: raw}
	b := raw[:cdSize]
	for len(b) > 0 {
		if len(b) < 46 || binary.LittleEndian.Uint32(b) != sigCentral {
			return nil, errors.New("bad central directory record")
		}
		ent := zipEntry{
			Flags:            binary.LittleEndian.Uint16(b[8:]),
			Method:           binary.LittleEndian.Uint16(b[10:]),
			CRC32:            binary.LittleEndian.Uint32(b[16:]),
			CompressedSize:   int64(binary.LittleEndian.Uint32(b[20:])),
			UncompressedSize: int64(binary.LittleEndian.Uint32(b[24:])),
			HeaderOffset:     int64(binary.LittleEndian.Uint32(b[42:])),
		}
		nameLen := int(binary.LittleEndian.Uint16(b[28:]))
		extraLen := int(binary.LittleEndian.Uint16(b[30:]))
		commentLen := int(binary.LittleEndian.Uint16(b[32:]))
		end := 46 + nameLen + extraLen + commentLen
		if end > len(b) {
			return nil, errors.New("truncated central directory record")
		}
		ent.Name = string(b[46 : 46+nameLen])
		extra := b[46+nameLen : 46+nameLen+extraLen]
		for len(extra) >= 4 {
			id := binary.LittleEndian.Uint16(extra)
			n := int(binary.LittleEndian.Uint16(extra[2:]))
			if 4+n > len(extra) {
				break
			}
			if id == 0x0001 {
				f := extra[4 : 4+n]
				next := func() (int64, bool) {
					if len(f) < 8 {
						return 0, false
					}
					v := int64(binary.LittleEndian.Uint64(f))
					f = f[8:]
					return v, true
				}
				if ent.UncompressedSize == 0xFFFFFFFF {
					if v, ok := next(); ok {
						ent.UncompressedSize = v
					}
				}
				if ent.CompressedSize == 0xFFFFFFFF {
					if v, ok := next(); ok {
						ent.CompressedSize = v
					}
				}
				if ent.HeaderOffset == 0xFFFFFFFF {
					if v, ok := next(); ok {
						ent.HeaderOffset = v
					}
				}
			}
			extra = extra[4+n:]
		}
		if ent.HeaderOffset < 0 || ent.HeaderOffset >= cdOffset || ent.CompressedSize < 0 || ent.UncompressedSize < 0 {
			return nil, fmt.Errorf("zip entry %q out of range", ent.Name)
		}
		dir.Entries = append(dir.Entries, ent)
		b = b[end:]
	}
	if int64(len(dir.Entries)) != entries {
		return nil, errors.New("zip entry count mismatch")
	}
	return dir, nil
}

// dataOffset reads the entry's local header to find where its data starts.
func (e zipEntry) dataOffset(r io.ReaderAt) (int64, error) {
	hdr := make([]byte, 30)
	if _, err := r.ReadAt(hdr, e.HeaderOffset); err != nil {
		return 0, err
	}
	if binary.LittleEndian.Uint32(hdr) != sigLocal {
		return 0, fmt.Errorf("bad local header for %q", e.Name)
	}
	return e.HeaderOffset + 30 + int64(binary.LittleEndian.Uint16(hdr[26:])) + int64(binary.LittleEndian.Uint16(hdr[28:])), nil
}
