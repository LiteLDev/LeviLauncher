package nativeinstall

import (
	"encoding/json"
	"fmt"

	"golang.org/x/sys/windows"

	"os"
	"path/filepath"

	"unsafe"

	"github.com/liteldev/LeviLauncher/internal/fsx"
)

func loadProtected(path string, out any) error {
	info, e := os.Stat(path)
	if e != nil {
		return e
	}
	if info.Size() == 0 || info.Size() > 1<<20 {
		return fmt.Errorf("invalid device cache size")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	if len(b) == 0 {
		return fmt.Errorf("empty protected cache")
	}
	in := windows.DataBlob{Size: uint32(len(b)), Data: &b[0]}
	var plain windows.DataBlob
	e = windows.CryptUnprotectData(&in, nil, nil, 0, nil, 1, &plain)
	if e != nil {
		return e
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(plain.Data))))
	p := unsafe.Slice(plain.Data, plain.Size)
	defer clear(p)
	return json.Unmarshal(p, out)
}

func saveProtected(path string, v any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	defer clear(b)
	in := windows.DataBlob{Size: uint32(len(b)), Data: &b[0]}
	var out windows.DataBlob
	e = windows.CryptProtectData(&in, nil, nil, 0, nil, 1, &out)
	if e != nil {
		return e
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(out.Data))))
	temp, e := fsx.CreateTemp(filepath.Dir(path), path)
	if e != nil {
		return e
	}
	name := temp.Name()
	defer os.Remove(name)
	if _, e = temp.Write(unsafe.Slice(out.Data, out.Size)); e != nil {
		temp.Close()
		return e
	}
	if e = temp.Sync(); e != nil {
		temp.Close()
		return e
	}
	if e = temp.Close(); e != nil {
		return e
	}
	return os.Rename(name, path)
}
