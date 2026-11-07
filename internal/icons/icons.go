package icons

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/fsx"
)

func writeICOFromPNG(pngData []byte, outPath string) error {
	b := &bytes.Buffer{}
	b.Write([]byte{0x00, 0x00})
	b.Write([]byte{0x01, 0x00})
	b.Write([]byte{0x01, 0x00})
	b.Write([]byte{0x00})
	b.Write([]byte{0x00})
	b.Write([]byte{0x00})
	b.Write([]byte{0x00})
	b.Write([]byte{0x00, 0x00})
	b.Write([]byte{0x00, 0x00})
	sz := uint32(len(pngData))
	b.Write([]byte{byte(sz), byte(sz >> 8), byte(sz >> 16), byte(sz >> 24)})
	b.Write([]byte{0x16, 0x00, 0x00, 0x00})
	b.Write(pngData)
	return fsx.WriteFileAtomic(outPath, b.Bytes(), 0o644)
}

// EnsureVersionIcon converts the instance's LargeLogo.png, a file of the
// game package, to <instance>\.levilauncher\icon.ico and returns its path,
// or "" when the instance has no logo.
func EnsureVersionIcon(versionDir string, _ bool) string {
	srcPng := filepath.Join(versionDir, "LargeLogo.png")
	icoPath := filepath.Join(apppath.InstanceStateDir(versionDir), apppath.InstanceIconFileName)
	fi, err := os.Stat(srcPng)
	if err != nil || fi.IsDir() {
		return ""
	}
	if si, e := os.Stat(icoPath); e != nil || si.Size() == 0 || fi.ModTime().After(si.ModTime()) {
		if data, er := os.ReadFile(srcPng); er == nil {
			if writeICOFromPNG(data, icoPath) == nil {
				fsx.SetHidden(filepath.Dir(icoPath))
			}
		}
	}
	if _, e := os.Stat(icoPath); e != nil {
		return ""
	}
	return icoPath
}
