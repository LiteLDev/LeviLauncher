package launchercore

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/liteldev/LeviLauncher/internal/vcruntime"
	"golang.org/x/sys/windows"
)

var (
	coreOnce                           sync.Once
	coreErr                            error
	apiOnce                            sync.Once
	apiErr                             error
	miProc                             *windows.LazyProc
	pGetWithPipe                       *windows.LazyProc
	useWide                            bool
	k32                                *windows.LazyDLL
	pWideCharToMultiByte               *windows.LazyProc
	pMultiByteToWideChar               *windows.LazyProc
	apiDLL                             *windows.LazyDLL
	pGetGamertagByXuid                 *windows.LazyProc
	pGetLocalUserId                    *windows.LazyProc
	pGetLocalUserGamertag              *windows.LazyProc
	pGetLocalUserGamerPictureSize      *windows.LazyProc
	pGetLocalUserGamerPicture          *windows.LazyProc
	pResetSession                      *windows.LazyProc
	pXUserGetState                     *windows.LazyProc
	pGetAggregatedUserStatisticsByXuid *windows.LazyProc
)

//go:embed launcher_core.dll
var embeddedLauncherCoreDLL []byte

//go:embed launcher_api.dll
var embeddedLauncherAPIDLL []byte

//go:embed libHttpClient.dll
var embeddedLibHttpClientDLL []byte

func prepareDLL() (string, error) {
	if len(embeddedLauncherCoreDLL) == 0 {
		return "", nil
	}

	base := os.Getenv("APPDATA")
	if base == "" {
		if d, err := os.UserCacheDir(); err == nil {
			base = d
		}
	}
	if base == "" {
		return "", nil
	}
	exeName := "levilauncher.exe"
	if exe, err := os.Executable(); err == nil {
		if b := filepath.Base(exe); b != "" {
			exeName = strings.ToLower(b)
		}
	}
	dir := filepath.Join(base, exeName, "bin")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	writeIfChanged := func(name string, data []byte) (string, error) {
		if len(data) == 0 {
			return "", nil
		}
		target := filepath.Join(dir, name)
		needWrite := false
		if fi, err := os.Stat(target); err != nil || fi.Size() == 0 {
			needWrite = true
		} else {
			if fh, err := fileSHA256(target); err != nil {
				needWrite = true
			} else {
				eh := bytesSHA256(data)
				if !bytes.Equal(fh, eh) {
					needWrite = true
				}
			}
		}
		if needWrite {
			tmp := target + ".tmp"
			if err := os.WriteFile(tmp, data, 0644); err != nil {
				return "", err
			}
			if err := os.Rename(tmp, target); err != nil {
				_ = os.Remove(tmp)
				return "", err
			}
		}
		return target, nil
	}

	corePath, err := writeIfChanged("launcher_core.dll", embeddedLauncherCoreDLL)
	if err != nil {
		return "", err
	}
	if _, err := writeIfChanged("launcher_api.dll", embeddedLauncherAPIDLL); err != nil {
		return "", err
	}
	if _, err := writeIfChanged("libHttpClient.dll", embeddedLibHttpClientDLL); err != nil {
		return "", err
	}
	_ = vcruntime.EnsureForVersion(context.Background(), dir)
	return corePath, nil
}

func fileSHA256(p string) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	sum := h.Sum(nil)
	return sum, nil
}

func bytesSHA256(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}

func EnsureCoreLoaded() error {
	coreOnce.Do(func() {
		name := os.Getenv("LAUNCHER_CORE_DLL")
		if name == "" {
			if p, err := prepareDLL(); err == nil && p != "" {
				_ = os.Setenv("LAUNCHER_CORE_DLL", p)
				name = p
				if os.Getenv("LAUNCHER_API_DLL") == "" {
					_ = os.Setenv("LAUNCHER_API_DLL", filepath.Join(filepath.Dir(p), "launcher_api.dll"))
				}
			}
		}
		if name == "" {
			name = "launcher_core.dll"
		}

		nameToLoad := name
		var dllDir string
		if abs, err := filepath.Abs(name); err == nil {
			nameToLoad = abs
			dllDir = filepath.Dir(abs)
		}

		if strings.TrimSpace(dllDir) != "" {
			for _, dep := range []string{"vcruntime140_1.dll", "libHttpClient.dll"} {
				depPath := filepath.Join(dllDir, dep)
				if _, err := os.Stat(depPath); err == nil {
					_, _ = windows.LoadLibrary(depPath)
				}
			}
			_ = windows.SetDllDirectory(dllDir)
			defer func() { _ = windows.SetDllDirectory("") }()
		}

		dll := windows.NewLazyDLL(nameToLoad)
		wide := dll.NewProc("GetW")
		ansi := dll.NewProc("Get")

		if err := dll.Load(); err != nil {
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "module could not be found") {
				coreErr = fmt.Errorf("ERR_DLL_DEPENDENCY_MISSING")
			} else {
				coreErr = err
			}
			return
		}
		if err := wide.Find(); err == nil {
			miProc = wide
			useWide = true
		} else if err := ansi.Find(); err == nil {
			miProc = ansi
			useWide = false
		} else {
			coreErr = ansi.Find()
			return
		}

		k32 = windows.NewLazyDLL("kernel32.dll")
		pWideCharToMultiByte = k32.NewProc("WideCharToMultiByte")
		pMultiByteToWideChar = k32.NewProc("MultiByteToWideChar")
		_ = k32.Load()
		pGetWithPipe = dll.NewProc("GetWithPipe")
	})
	return coreErr
}

func EnsureApiLoaded() error {
	apiOnce.Do(func() {
		if err := EnsureCoreLoaded(); err != nil {
			apiErr = err
			return
		}

		apiName := os.Getenv("LAUNCHER_API_DLL")
		if apiName == "" {
			coreName := os.Getenv("LAUNCHER_CORE_DLL")
			if coreName == "" {
				coreName = "launcher_core.dll"
			}
			if abs, err := filepath.Abs(coreName); err == nil {
				apiName = filepath.Join(filepath.Dir(abs), "launcher_api.dll")
			}
		}
		if apiName == "" {
			apiName = "launcher_api.dll"
		}

		nameToLoad := apiName
		var dllDir string
		if abs, err := filepath.Abs(apiName); err == nil {
			nameToLoad = abs
			dllDir = filepath.Dir(abs)
		}

		if strings.TrimSpace(dllDir) != "" {
			for _, dep := range []string{"vcruntime140_1.dll", "libHttpClient.dll"} {
				depPath := filepath.Join(dllDir, dep)
				if _, err := os.Stat(depPath); err == nil {
					_, _ = windows.LoadLibrary(depPath)
				}
			}
			_ = windows.SetDllDirectory(dllDir)
			defer func() { _ = windows.SetDllDirectory("") }()
		}

		apiDLL = windows.NewLazyDLL(nameToLoad)
		pGetGamertagByXuid = apiDLL.NewProc("nh_8b3de51a")
		pGetLocalUserId = apiDLL.NewProc("nh_04d8c8d4")
		pGetLocalUserGamertag = apiDLL.NewProc("nh_f53b7b5e")
		pGetLocalUserGamerPictureSize = apiDLL.NewProc("nh_1f2d9a0c")
		pGetLocalUserGamerPicture = apiDLL.NewProc("nh_aa347be1")
		pResetSession = apiDLL.NewProc("nh_5c8e2c0b")
		pXUserGetState = apiDLL.NewProc("nh_3e61c2af")
		pGetAggregatedUserStatisticsByXuid = apiDLL.NewProc("nh_2b8f3d6a")

		if err := apiDLL.Load(); err != nil {
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "module could not be found") {
				apiErr = fmt.Errorf("ERR_DLL_DEPENDENCY_MISSING")
			} else {
				apiErr = err
			}
			return
		}
	})
	return apiErr
}

func Init() {
	_ = EnsureCoreLoaded()
}

func Extract(msixvcPath string, outDir string) (int, string) {
	if err := EnsureCoreLoaded(); err != nil {
		return 1, err.Error()
	}

	var r1 uintptr
	if useWide {
		cMsix16, err := windows.UTF16PtrFromString(msixvcPath)
		if err != nil {
			return 1, err.Error()
		}
		cOut16, err := windows.UTF16PtrFromString(outDir)
		if err != nil {
			return 1, err.Error()
		}
		r1, _, _ = miProc.Call(
			uintptr(unsafe.Pointer(cMsix16)),
			uintptr(unsafe.Pointer(cOut16)),
		)
	} else {
		bMsix, err := utf8ToACP(msixvcPath)
		if err != nil {
			return 1, err.Error()
		}
		bOut, err := utf8ToACP(outDir)
		if err != nil {
			return 1, err.Error()
		}
		r1, _, _ = miProc.Call(
			uintptr(unsafe.Pointer(&bMsix[0])),
			uintptr(unsafe.Pointer(&bOut[0])),
		)
	}

	rc := int(r1)
	if rc == 0 {
		return 0, ""
	}
	if rc == 3 {
		return 3, "ERR_MC_NOT_AUTHORIZED"
	}
	return 1, "ERR_APPX_INSTALL_FAILED"
}

func ExtractWithPipe(msixvcPath, outDir, pipeName string) (int, string) {
	if err := EnsureCoreLoaded(); err != nil {
		return 1, err.Error()
	}

	bMsix, err := utf8ToACP(msixvcPath)
	if err != nil {
		return 1, err.Error()
	}
	bOut, err := utf8ToACP(outDir)
	if err != nil {
		return 1, err.Error()
	}
	bPipe, err := utf8ToACP(pipeName)
	if err != nil {
		return 1, err.Error()
	}

	r1, _, _ := pGetWithPipe.Call(
		uintptr(unsafe.Pointer(&bMsix[0])),
		uintptr(unsafe.Pointer(&bOut[0])),
		uintptr(unsafe.Pointer(&bPipe[0])),
	)

	rc := int(r1)
	if rc == 0 {
		return 0, ""
	}
	if rc == 3 {
		return 3, "ERR_MC_NOT_AUTHORIZED"
	}
	return 1, "ERR_APPX_INSTALL_FAILED"
}

func utf8ToACP(s string) ([]byte, error) {
	u16, err := windows.UTF16FromString(s)
	if err != nil {
		return nil, err
	}
	if len(u16) == 0 {
		return []byte{0}, nil
	}
	const CP_ACP = 0
	r0, _, e1 := pWideCharToMultiByte.Call(
		uintptr(uint32(CP_ACP)),
		0,
		uintptr(unsafe.Pointer(&u16[0])),
		uintptr(len(u16)-1),
		0,
		0,
		0,
		0,
	)
	if r0 == 0 {
		if e1 != nil {
			return nil, e1
		}
		return nil, syscall.EINVAL
	}
	buf := make([]byte, r0+1)
	r1, _, e2 := pWideCharToMultiByte.Call(
		uintptr(uint32(CP_ACP)),
		0,
		uintptr(unsafe.Pointer(&u16[0])),
		uintptr(len(u16)-1),
		uintptr(unsafe.Pointer(&buf[0])),
		r0,
		0,
		0,
	)
	if r1 == 0 {
		if e2 != nil {
			return nil, e2
		}
		return nil, syscall.EINVAL
	}
	buf[r0] = 0
	return buf, nil
}

func AcpToString(b []byte) (string, error) {
	if len(b) == 0 {
		return "", nil
	}
	const CP_ACP = 0
	r0, _, e1 := pMultiByteToWideChar.Call(
		uintptr(uint32(CP_ACP)),
		0,
		uintptr(unsafe.Pointer(&b[0])),
		uintptr(len(b)),
		0,
		0,
	)
	if r0 == 0 {
		if e1 != nil && e1 != syscall.Errno(0) {
			return "", e1
		}
		return "", nil
	}

	u16 := make([]uint16, r0)
	r1, _, e2 := pMultiByteToWideChar.Call(
		uintptr(uint32(CP_ACP)),
		0,
		uintptr(unsafe.Pointer(&b[0])),
		uintptr(len(b)),
		uintptr(unsafe.Pointer(&u16[0])),
		r0,
	)
	if r1 == 0 {
		return "", e2
	}
	return syscall.UTF16ToString(u16), nil
}

func GetAggregatedUserStatisticsByXuid(xuid uint64) (minutesPlayed int64, blockBroken int64, mobKilled int64, distanceTravelled float64, err error) {
	if err := EnsureApiLoaded(); err != nil {
		return 0, 0, 0, 0, err
	}

	var mp int64
	var bb int64
	var mk int64
	var dt float64

	r1, _, e1 := pGetAggregatedUserStatisticsByXuid.Call(
		uintptr(xuid),
		uintptr(unsafe.Pointer(&mp)),
		uintptr(unsafe.Pointer(&bb)),
		uintptr(unsafe.Pointer(&mk)),
		uintptr(unsafe.Pointer(&dt)),
	)

	if int32(r1) != 0 {
		if e1 != nil && e1 != syscall.Errno(0) {
			return 0, 0, 0, 0, fmt.Errorf("call failed with ret %d: %w", r1, e1)
		}
		return 0, 0, 0, 0, fmt.Errorf("call failed with ret %d", r1)
	}

	return mp, bb, mk, dt, nil
}
