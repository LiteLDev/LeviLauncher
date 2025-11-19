package icons

import (
    "bytes"
    "os"
    "path/filepath"
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
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(outPath, b.Bytes(), 0644)
}

func EnsureVersionIcon(versionDir string, _ bool) string {
    srcPng := filepath.Join(versionDir, "LargeLogo.png")
    icoPath := filepath.Join(versionDir, "LargeLogo.ico")
    if fi, err := os.Stat(srcPng); err == nil && !fi.IsDir() {
        need := false
        if si, e := os.Stat(icoPath); e != nil || si.Size() == 0 || fi.ModTime().After(si.ModTime()) {
            need = true
        }
        if need {
            if data, er := os.ReadFile(srcPng); er == nil {
                _ = writeICOFromPNG(data, icoPath)
            }
        }
        if _, e := os.Stat(icoPath); e == nil {
            return icoPath
        }
    }
    return ""
}
