// SPDX-License-Identifier: GPL-3.0-only
package nativeinstall

import (
	"bytes"
	"crypto/aes"
	"crypto/hmac"
	"crypto/sha256"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf16"
)

func unwrapKey(kek, wrapped []byte) []byte {
	if len(wrapped) < 24 || len(wrapped)%8 != 0 {
		fail("invalid RFC3394 size")
	}
	a, e := aes.NewCipher(kek)
	must(e)
	out := append([]byte(nil), wrapped...)
	n := len(out)/8 - 1
	var block [16]byte
	for j := 5; j >= 0; j-- {
		for i := n; i >= 1; i-- {
			binary.BigEndian.PutUint64(block[:8], binary.BigEndian.Uint64(out[:8])^uint64(n*j+i))
			copy(block[8:], out[i*8:(i+1)*8])
			a.Decrypt(block[:], block[:])
			copy(out[:8], block[:8])
			copy(out[i*8:(i+1)*8], block[8:])
		}
	}
	if !hmac.Equal(out[:8], bytes.Repeat([]byte{0xa6}, 8)) {
		clear(out)
		fail("content key wrap authentication failed")
	}
	key := append([]byte(nil), out[8:]...)
	clear(out)
	return key
}
func formatGUID(b []byte) string {
	if len(b) < 16 {
		fail("short GUID")
	}
	return fmt.Sprintf("%08x-%04x-%04x-%x-%x", binary.LittleEndian.Uint32(b), binary.LittleEndian.Uint16(b[4:]), binary.LittleEndian.Uint16(b[6:]), b[8:10], b[10:16])
}
func readRange(f *os.File, pos int64, n int) []byte {
	if n < 0 || n > 64<<20 {
		fail("invalid read size")
	}
	b := make([]byte, n)
	_, e := f.ReadAt(b, pos)
	must(e)
	return b
}
func decryptPage(page, key, tweak []byte) {
	t, e := aes.NewCipher(key[:16])
	must(e)
	d, e := aes.NewCipher(key[16:])
	must(e)
	var tw [16]byte
	t.Encrypt(tw[:], tweak)
	for p := 0; p < len(page); p += 16 {
		b := page[p : p+16]
		for i := range b {
			b[i] ^= tw[i]
		}
		d.Decrypt(b, b)
		for i := range b {
			b[i] ^= tw[i]
		}
		carry := byte(0)
		for i := range tw {
			next := tw[i] >> 7
			tw[i] = (tw[i] << 1) | carry
			carry = next
		}
		if carry != 0 {
			tw[0] ^= 0x87
		}
	}
}
func useLicense(blob []byte, contentID, licenseType string) {
	blocks := parseSP(blob)
	if len(ownDeviceID) != 8 || !hmac.Equal(blocks[0xd2], ownDeviceID) {
		failCode("ERR_LICENSE_DEVICE_MISMATCH", "content license is bound to another device")
	}
	keys := blocks[0xca]
	for pos := 0; pos < len(keys); {
		if len(keys)-pos < 4 {
			fail("short content key header")
		}
		idlen, keylen := int(binary.LittleEndian.Uint16(keys[pos:])), int(binary.LittleEndian.Uint16(keys[pos+2:]))
		pos += 4
		if idlen < 16 || keylen != 40 || idlen+keylen > len(keys)-pos {
			fail("invalid packed content key")
		}
		id := formatGUID(keys[pos:])
		if strings.EqualFold(id, active.report.KeyID) {
			if contentKey != nil {
				fail("ambiguous duplicate content key")
			}
			key := unwrapKey(ownKey, keys[pos+idlen:pos+idlen+keylen])
			if len(key) != 32 {
				clear(key)
				fail("invalid content key length")
			}
			contentKey = key
			active.report.LicenseType = licenseType
			emit(map[string]any{"online_content_id": contentID, "online_key_id": id, "license_type": licenseType, "device_binding_matches": true, "key_wrap_integrity_verified": true})
		}
		pos += idlen + keylen
	}
}

type segmentPlan struct {
	name      string
	size      int64
	start     int64
	region    uint32
	encrypted bool
	flags     uint16
}

func pages(n uint64) uint64 { return (n + 4095) / 4096 }
func u16text(b []byte) string {
	if len(b)%2 != 0 {
		fail("odd UTF16")
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	s := string(utf16.Decode(u))
	return strings.TrimRight(s, "\x00")
}
func safePath(s string) string {
	for _, r := range s {
		if r < 32 {
			fail("control character in output path")
		}
	}
	if s == "" || strings.ContainsAny(s, "\x00<>:\"|?*") {
		fail("unsafe output path")
	}
	s = strings.ReplaceAll(s, "\\", "/")
	if strings.HasPrefix(s, "/") {
		fail("absolute output path")
	}
	for _, part := range strings.Split(s, "/") {
		if part == "" || part == "." || part == ".." || strings.TrimRight(part, ". ") != part {
			fail("unsafe path component")
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CONIN$" || base == "CONOUT$" || base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || regexp.MustCompile("^(COM|LPT)[1-9¹²³]$").MatchString(base) {
			fail("reserved Windows path")
		}
	}
	return filepath.FromSlash(s)
}
func fullExtract(path string, key []byte, keyID string) {
	f := active.source
	stat, e := f.Stat()
	must(e)
	h := readRange(f, 0, 4096)
	if string(h[512:520]) != "msft-xvd" || binary.LittleEndian.Uint32(h[0x208:]) != 65 || binary.LittleEndian.Uint32(h[0x280:]) != 0 {
		fail("unsupported XVD layout")
	}
	udLen := uint64(binary.LittleEndian.Uint32(h[0x28c:]))
	xvcLen := uint64(binary.LittleEndian.Uint32(h[0x290:]))
	dynLen := uint64(binary.LittleEndian.Uint32(h[0x294:]))
	if dynLen != 0 || udLen < 544 || udLen > 32<<20 || xvcLen > 32<<20 {
		fail("unsupported metadata size")
	}
	hashed := pages(binary.LittleEndian.Uint64(h[0x218:])) + pages(udLen) + pages(xvcLen)
	counts := []uint64{}
	n := hashed
	for {
		n = (n + 169) / 170
		counts = append(counts, n)
		if n == 1 {
			break
		}
		if len(counts) > 4 {
			fail("oversized hash tree")
		}
	}
	totalHashes := uint64(0)
	for _, n := range counts {
		totalHashes += n
	}
	treeOff := uint64(12288) + pages(uint64(binary.LittleEndian.Uint32(h[0x288:])))*4096 + uint64(h[0x470])*4096
	userOff := treeOff + totalHashes*4096
	xvcOff := userOff + pages(udLen)*4096
	if userOff+hashed*4096 > uint64(stat.Size()) {
		fail("truncated package")
	}
	tree := readRange(f, int64(treeOff), int(totalHashes*4096))
	top := sha256.Sum256(tree[:4096])
	if !hmac.Equal(top[:], h[0x240:0x260]) {
		fail("root hash mismatch")
	}
	parentStart := uint64(0)
	childStart := uint64(1)
	for level := len(counts) - 2; level >= 0; level-- {
		for i := uint64(0); i < counts[level]; i++ {
			child := tree[(childStart+i)*4096 : (childStart+i+1)*4096]
			want := (parentStart+i/170)*4096 + (i%170)*24
			sum := sha256.Sum256(child)
			if !hmac.Equal(sum[:20], tree[want:want+20]) {
				fail("hash tree branch mismatch")
			}
		}
		parentStart = childStart
		childStart += counts[level]
	}
	leafBase := totalHashes - counts[0]
	entry := func(pos int64) []byte {
		if pos < int64(userOff) || pos%4096 != 0 {
			fail("invalid hashed page offset")
		}
		index := (uint64(pos) - userOff) / 4096
		if index >= hashed {
			fail("page outside hashed area")
		}
		off := (leafBase+index/170)*4096 + index%170*24
		return tree[off : off+24]
	}
	checkedPage := func(pos int64) []byte {
		b := readRange(f, pos, 4096)
		want := entry(pos)
		sum := sha256.Sum256(b)
		if !hmac.Equal(sum[:20], want[:20]) {
			fail(fmt.Sprintf("data page digest mismatch at %d", pos))
		}
		return b
	}
	for pos := userOff; pos < xvcOff+pages(xvcLen)*4096; pos += 4096 {
		checkedPage(int64(pos))
	}
	ud := readRange(f, int64(userOff), int(udLen))
	uh := int(binary.LittleEndian.Uint32(ud))
	if uh < 16 || uh > len(ud)-528 {
		fail("bad user metadata header")
	}
	count := int(binary.LittleEndian.Uint32(ud[uh+524:]))
	if count < 1 || count > 1000 || uh+528+count*528 > len(ud) {
		fail("bad user file table")
	}
	var segmeta []byte
	for i := 0; i < count; i++ {
		p := uh + 528 + i*528
		name := u16text(ud[p : p+520])
		size, off := int(binary.LittleEndian.Uint32(ud[p+520:])), int(binary.LittleEndian.Uint32(ud[p+524:]))
		off += uh
		if off < 0 || off > len(ud) || size > len(ud)-off {
			fail("bad user file bounds")
		}
		if name == "SegmentMetadata.bin" {
			segmeta = ud[off : off+size]
		}
	}
	if len(segmeta) < 100 {
		fail("missing segment metadata")
	}
	segCount := int(binary.LittleEndian.Uint32(segmeta[16:]))
	headerLen := int(binary.LittleEndian.Uint32(segmeta[12:]))
	if headerLen != 100 || segCount < 1 || segCount > 1000000 || headerLen+segCount*16 > len(segmeta) {
		fail("bad segment table")
	}
	plans := make([]segmentPlan, segCount)
	pathBase := headerLen + segCount*16
	names := map[string]bool{}
	for i := range plans {
		p := headerLen + i*16
		pn := int(binary.LittleEndian.Uint16(segmeta[p+2:])) * 2
		po := pathBase + int(binary.LittleEndian.Uint32(segmeta[p+4:]))
		size := binary.LittleEndian.Uint64(segmeta[p+8:])
		if po < pathBase || po > len(segmeta) || pn > len(segmeta)-po || size > uint64(stat.Size()) {
			fail("bad segment bounds")
		}
		name := safePath(u16text(segmeta[po : po+pn]))
		lower := strings.ToLower(name)
		if names[lower] {
			fail("case-colliding output paths")
		}
		names[lower] = true
		plans[i] = segmentPlan{name: name, size: int64(size), start: -1, flags: binary.LittleEndian.Uint16(segmeta[p:])}
	}
	for name := range names {
		for parent := filepath.Dir(name); parent != "."; parent = filepath.Dir(parent) {
			if names[parent] {
				fail("file/directory path collision")
			}
		}
	}
	x := readRange(f, int64(xvcOff), int(xvcLen))
	if binary.LittleEndian.Uint16(x[0xd1e:]) != 1 || !strings.EqualFold(formatGUID(x[16:32]), keyID) {
		fail("missing/ambiguous key mapping")
	}
	regionCount := int(binary.LittleEndian.Uint32(x[0xd14:]))
	updateCount := int(binary.LittleEndian.Uint32(x[0xd3c:]))
	updateBase := 0xda8 + regionCount*128
	if regionCount < 1 || regionCount > 10000 || updateCount < 1 || updateBase+updateCount*12 > len(x) {
		fail("bad region table")
	}
	firstSegmentOff := int64(binary.LittleEndian.Uint32(x[updateBase:])) * 4096
	covered := 0
	for i := 0; i < regionCount; i++ {
		p := 0xda8 + i*128
		start := int(binary.LittleEndian.Uint32(x[p+12:]))
		off, length := int64(binary.LittleEndian.Uint64(x[p+80:])), int64(binary.LittleEndian.Uint64(x[p+88:]))
		if start == 0 && off != firstSegmentOff {
			continue
		}
		if off < 0 || length < 0 || off%4096 != 0 || length%4096 != 0 || off > stat.Size() || length > stat.Size()-off || start >= len(plans) {
			fail("invalid extraction region")
		}
		id := binary.LittleEndian.Uint32(x[p:])
		ki := binary.LittleEndian.Uint16(x[p+4:])
		if ki != 0 && ki != 65535 {
			fail("unsupported region KeyID")
		}
		pos := off
		for idx := start; pos < off+length; idx++ {
			if idx == len(plans) && i == regionCount-1 && off+length-pos <= 65536 {
				for padding := pos; padding < off+length; padding += 4096 {
					checkedPage(padding)
				}
				emit(map[string]any{"verified_final_region_padding_bytes": off + length - pos})
				break
			}
			if idx >= len(plans) || plans[idx].start != -1 {
				fail(fmt.Sprintf("overlapping/incomplete region segments: region=%x idx=%d pos=%d end=%d count=%d", id, idx, pos, off+length, len(plans)))
			}
			consume := int64(pages(uint64(plans[idx].size))) * 4096
			if consume == 0 {
				consume = 4096
			}
			if consume > off+length-pos {
				fail("segment exceeds region")
			}
			if idx >= updateCount || int64(binary.LittleEndian.Uint32(x[updateBase+idx*12:]))*4096 != pos {
				fail("update segment offset mismatch")
			}
			plans[idx].start = pos
			plans[idx].region = id
			plans[idx].encrypted = ki != 65535
			covered++
			pos += consume
		}
	}
	if covered != len(plans) {
		fail(fmt.Sprintf("incomplete manifest: %d/%d", covered, len(plans)))
	}
	if !names["minecraft.windows.exe"] || !names["microsoftgame.config"] {
		fail("required game files absent")
	}
	emit(map[string]any{"manifest_files": len(plans), "hash_tree_validated": true, "metadata_pages_validated": true})
	staging, e := os.MkdirTemp(filepath.Dir(active.out), ".msixvc-staging-")
	must(e)
	ok := false
	var output *os.Root
	defer func() {
		if output != nil {
			_ = output.Close()
		}
		if !ok {
			_ = os.RemoveAll(staging)
		}
	}()
	output, e = os.OpenRoot(staging)
	must(e)
	total := int64(0)
	for _, s := range plans {
		total += s.size
	}
	current := int64(0)
	last := time.Now()
	for _, s := range plans {
		checkCanceled()
		must(output.MkdirAll(filepath.Dir(s.name), 0700))
		out, e := output.OpenFile(s.name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		must(e)
		func() {
			defer out.Close()
			remaining := s.size
			for pos := s.start; remaining > 0 || pos == s.start; pos += 4096 {
				checkCanceled()
				page := checkedPage(pos)
				if s.encrypted {
					tweak := make([]byte, 16)
					copy(tweak[:4], entry(pos)[20:])
					binary.LittleEndian.PutUint32(tweak[4:], s.region)
					copy(tweak[8:], h[0x220:0x228])
					decryptPage(page, key, tweak)
				}
				n := int64(4096)
				if remaining < n {
					n = remaining
				}
				_, e := out.Write(page[:n])
				if e != nil {
					out.Close()
					fail(e)
				}
				remaining -= n
				current += n
				if remaining == 0 {
					break
				}
			}
			must(out.Close())
		}()
		if time.Since(last) > 150*time.Millisecond {
			emit(map[string]any{"extract_bytes": current, "extract_total": total, "file": s.name, "file_current": s.size, "file_total": s.size})
			last = time.Now()
		}
	}
	if current != total {
		fail("extraction byte count mismatch")
	}
	pe, e := pe.Open(filepath.Join(staging, "Minecraft.Windows.exe"))
	must(e)
	if pe.FileHeader.Machine != 0x8664 {
		pe.Close()
		fail("unexpected PE architecture")
	}
	defer pe.Close()
	imports, e := pe.ImportedSymbols()
	must(e)
	sections := len(pe.Sections)
	must(pe.Close())
	if active.options.Prepare != nil {
		must(active.options.Prepare(staging))
	}
	checkCanceled()
	must(output.Close())
	final := active.out
	if _, e := os.Lstat(final); !os.IsNotExist(e) {
		fail("final output already exists")
	}
	must(os.Rename(staging, final))
	ok = true
	active.report.OutputDir = final
	active.report.FileCount = len(plans)
	active.report.Bytes = current
	emit(map[string]any{"extraction_complete": true, "output_directory": final, "files": len(plans), "bytes": current, "exe_pe_sections": sections, "exe_import_symbol_count": len(imports), "input_unchanged": func() bool {
		end, e := f.Stat()
		return e == nil && end.Size() == stat.Size() && end.ModTime() == stat.ModTime()
	}()})
}
