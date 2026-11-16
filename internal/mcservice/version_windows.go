package mcservice

import (
	"fmt"

	_ "image/gif"
	_ "image/jpeg"

	"strings"

	"unsafe"

	"github.com/liteldev/LeviLauncher/internal/utils"

	"golang.org/x/sys/windows"
)

func readExeFileVersion(exePath string) (string, bool) {
	p := strings.TrimSpace(exePath)
	if p == "" || !utils.FileExists(p) {
		return "", false
	}
	p16, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return "", false
	}
	var handle uint32
	r1, _, _ := procGetFileVersionInfoSizeW.Call(
		uintptr(unsafe.Pointer(p16)),
		uintptr(unsafe.Pointer(&handle)),
	)
	if r1 == 0 {
		return "", false
	}
	sz := uint32(r1)
	buf := make([]byte, sz)
	r1, _, _ = procGetFileVersionInfoW.Call(
		uintptr(unsafe.Pointer(p16)),
		0,
		uintptr(sz),
		uintptr(unsafe.Pointer(&buf[0])),
	)
	if r1 == 0 {
		return "", false
	}
	root16, _ := windows.UTF16PtrFromString(`\`)
	var block unsafe.Pointer
	var blockLen uint32
	r1, _, _ = procVerQueryValueW.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(root16)),
		uintptr(unsafe.Pointer(&block)),
		uintptr(unsafe.Pointer(&blockLen)),
	)
	if r1 == 0 || block == nil || blockLen < uint32(unsafe.Sizeof(vsFixedFileInfo{})) {
		return "", false
	}
	info := (*vsFixedFileInfo)(block)
	if info.Signature != 0xFEEF04BD {
		return "", false
	}
	major := int(info.FileVersionMS >> 16)
	minor := int(info.FileVersionMS & 0xFFFF)
	build := int(info.FileVersionLS >> 16)
	rev := int(info.FileVersionLS & 0xFFFF)
	return fmt.Sprintf("%d.%d.%d.%d", major, minor, build, rev), true
}

func IsProcessRunningAtPath(exePath string) bool {
	p := canonicalPath(exePath)
	if p == "" {
		return false
	}
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(snap)
	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	if err := windows.Process32First(snap, &pe); err != nil {
		return false
	}
	for {
		pid := pe.ProcessID
		h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
		if err == nil {
			buf := make([]uint16, 1024)
			size := uint32(len(buf))
			if e := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); e == nil && size > 0 {
				path := windows.UTF16ToString(buf[:size])
				_ = windows.CloseHandle(h)
				// The OS already reports a resolved image path here.
				if normalizePath(path) == p {
					return true
				}
			} else {
				_ = windows.CloseHandle(h)
			}
		}
		if err := windows.Process32Next(snap, &pe); err != nil {
			break
		}
	}
	return false
}

var (
	versionDLL                  = windows.NewLazySystemDLL("version.dll")
	procGetFileVersionInfoSizeW = versionDLL.NewProc("GetFileVersionInfoSizeW")
	procGetFileVersionInfoW     = versionDLL.NewProc("GetFileVersionInfoW")
	procVerQueryValueW          = versionDLL.NewProc("VerQueryValueW")
)
