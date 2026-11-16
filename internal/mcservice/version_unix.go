//go:build darwin || linux

package mcservice

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"github.com/liteldev/LeviLauncher/internal/gameruntime"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

func readExeFileVersion(path string) (string, bool) {
	f, err := pe.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	for _, s := range f.Sections {
		if s.Name != ".rsrc" || s.Size > 64<<20 {
			continue
		}
		data, err := s.Data()
		if err != nil {
			continue
		}
		at := bytes.Index(data, []byte{0xbd, 0x04, 0xef, 0xfe})
		if at < 0 || len(data)-at < 52 {
			continue
		}
		ms := binary.LittleEndian.Uint32(data[at+8:])
		ls := binary.LittleEndian.Uint32(data[at+12:])
		return fmt.Sprintf("%d.%d.%d.%d", ms>>16, ms&0xffff, ls>>16, ls&0xffff), true
	}
	return "", false
}
func IsProcessRunningAtPath(path string) bool {
	for _, p := range gameruntime.Processes() {
		if utils.CanonicalWindowsPath(p.Executable) == utils.CanonicalWindowsPath(path) {
			return true
		}
	}
	return false
}
