//go:build linux

package extractor

import (
    "bytes"
    "crypto/sha256"
    "io"
    "os"
    "os/exec"
    "path/filepath"
    "strings"

    "github.com/liteldev/LeviLauncher/internal/utils"
)

func Init() {}

func ensureEmbeddedDLL() string {
    if len(embeddedLauncherCoreDLL) == 0 {
        return ""
    }
    base := utils.BaseRoot()
    dir := filepath.Join(base, "bin")
    _ = os.MkdirAll(dir, 0755)
    target := filepath.Join(dir, "launcher_core.dll")
    needWrite := false
    if fi, err := os.Stat(target); err != nil || fi.Size() == 0 {
        needWrite = true
    } else {
        if fh, err := fileSHA256(target); err != nil {
            needWrite = true
        } else {
            eh := bytesSHA256(embeddedLauncherCoreDLL)
            if !bytes.Equal(fh, eh) { needWrite = true }
        }
    }
    if needWrite {
        tmp := target + ".tmp"
        if err := os.WriteFile(tmp, embeddedLauncherCoreDLL, 0644); err == nil {
            if err := os.Rename(tmp, target); err == nil { return target }
            _ = os.Remove(tmp)
        }
    }
    if fi, err := os.Stat(target); err == nil && fi.Size() > 0 { return target }
    return ""
}

func ensureEmbeddedWrapper() string {
    if len(embeddedLauncherCoreCLI) == 0 {
        return ""
    }
    base := utils.BaseRoot()
    dir := filepath.Join(base, "bin")
    _ = os.MkdirAll(dir, 0755)
    target := filepath.Join(dir, "launcher_core_cli.exe")
    needWrite := false
    if fi, err := os.Stat(target); err != nil || fi.Size() == 0 {
        needWrite = true
    } else {
        if fh, err := fileSHA256(target); err != nil {
            needWrite = true
        } else {
            eh := bytesSHA256(embeddedLauncherCoreCLI)
            if !bytes.Equal(fh, eh) { needWrite = true }
        }
    }
    if needWrite {
        tmp := target + ".tmp"
        if err := os.WriteFile(tmp, embeddedLauncherCoreCLI, 0644); err == nil {
            if err := os.Rename(tmp, target); err == nil { return target }
            _ = os.Remove(tmp)
        }
    }
    if fi, err := os.Stat(target); err == nil && fi.Size() > 0 { return target }
    return ""
}

func MiHoYo(msixvcPath string, outDir string) (int, string) {
	if strings.TrimSpace(msixvcPath) == "" || strings.TrimSpace(outDir) == "" {
		return 1, "ERR_ARGS"
	}
	if _, err := os.Stat(msixvcPath); err != nil {
		return 1, "ERR_MSIXVC_NOT_FOUND"
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return 1, "ERR_CREATE_TARGET_DIR"
	}

    dll := ""
    if p := ensureEmbeddedDLL(); p != "" {
        dll = p
    } else {
        base := utils.BaseRoot()
        cand := filepath.Join(base, "bin", "launcher_core.dll")
        if _, err := os.Stat(cand); err == nil {
            dll = cand
        }
    }
    if strings.TrimSpace(dll) == "" {
        return 1, "ERR_DLL_NOT_AVAILABLE"
    }
    if _, err := os.Stat(dll); err != nil {
        return 1, "ERR_DLL_NOT_AVAILABLE"
    }
    wrapper := strings.TrimSpace(os.Getenv("LAUNCHER_CORE_WRAPPER"))
    if wrapper == "" {
        if p := ensureEmbeddedWrapper(); p != "" {
            wrapper = p
        } else {
            base := filepath.Dir(dll)
            cand := filepath.Join(base, "launcher_core_cli.exe")
            if _, err := os.Stat(cand); err == nil {
                wrapper = cand
            }
        }
    }
    if strings.TrimSpace(wrapper) == "" {
        return 1, "ERR_WRAPPER_NOT_FOUND"
    }
    if _, err := os.Stat(wrapper); err != nil {
        return 1, "ERR_WRAPPER_NOT_FOUND"
    }
    // Prefer Wine built from WineGDK
    wine := filepath.Join(utils.BaseRoot(), "wine", "files", "bin", "wine")
    if _, err := os.Stat(wine); err != nil {
        wow := filepath.Join(utils.BaseRoot(), "wine", "files", "bin-wow64", "wine")
        if _, er2 := os.Stat(wow); er2 == nil {
            wine = wow
        } else {
            alt := filepath.Join(utils.BaseRoot(), "wine", "files", "bin", "wine64")
            if _, er3 := os.Stat(alt); er3 == nil { wine = alt } else { return 1, "ERR_WINE_NOT_AVAILABLE" }
        }
    }
    pf := filepath.Join(utils.BaseRoot(), "prefix")
    _ = os.MkdirAll(pf, 0755)
    cmd := exec.Command(wine, wrapper, msixvcPath, outDir, dll)
    cmd.Env = append(os.Environ(), "WINEPREFIX="+pf)
    if err := cmd.Run(); err != nil {
        return 1, "ERR_APPX_INSTALL_FAILED"
    }
    return 0, ""
}

func fileSHA256(p string) ([]byte, error) {
    f, err := os.Open(p)
    if err != nil { return nil, err }
    defer f.Close()
    h := sha256.New()
    if _, err := io.Copy(h, f); err != nil { return nil, err }
    return h.Sum(nil), nil
}

func bytesSHA256(b []byte) []byte {
    h := sha256.Sum256(b)
    return h[:]
}
