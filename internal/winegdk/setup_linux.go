//go:build linux

package winegdk

import (
    "archive/tar"
    "bufio"
    "compress/gzip"
    "context"
    "io"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "strings"

    "github.com/liteldev/LeviLauncher/internal/utils"
    "github.com/wailsapp/wails/v3/pkg/application"
)

const (
	EventSetupStatus   = "winegdk.setup.status"
	EventSetupProgress = "winegdk.setup.progress"
	EventSetupError    = "winegdk.setup.error"
	EventSetupDone     = "winegdk.setup.done"
)

func Setup(ctx context.Context) string {
    base := utils.BaseRoot()
    if strings.TrimSpace(base) == "" {
        application.Get().Event.Emit(EventSetupError, "ERR_BASE_ROOT")
        return "ERR_BASE_ROOT"
    }
    application.Get().Event.Emit(EventSetupStatus, "start")
    wineDir := filepath.Join(base, "wine")
    _ = os.MkdirAll(wineDir, 0755)
    url := "https://github.com/Weather-OS/GDK-Proton/releases/download/release/GE-Proton10-25.tar.gz"
    tmp := filepath.Join(wineDir, "GE-Proton10-25.tar.gz")
    application.Get().Event.Emit(EventSetupStatus, "download_start")
    if err := downloadWithProgress(url, tmp, "download"); err != nil {
        application.Get().Event.Emit(EventSetupError, "ERR_DOWNLOAD")
        return "ERR_DOWNLOAD"
    }
    application.Get().Event.Emit(EventSetupStatus, "extract_start")
    f, err := os.Open(tmp)
    if err != nil {
        application.Get().Event.Emit(EventSetupError, "ERR_OPEN_TAR")
        return "ERR_OPEN_TAR"
    }
    defer f.Close()
    gz, err := gzip.NewReader(f)
    if err != nil {
        application.Get().Event.Emit(EventSetupError, "ERR_OPEN_GZ")
        return "ERR_OPEN_GZ"
    }
    defer gz.Close()
    tr := tar.NewReader(gz)
    for {
        hdr, er := tr.Next()
        if er == io.EOF { break }
        if er != nil { application.Get().Event.Emit(EventSetupError, "ERR_EXTRACT"); return "ERR_EXTRACT" }
        name := strings.TrimSpace(hdr.Name)
        if name == "" { continue }
        parts := strings.SplitN(name, "/", 2)
        if len(parts) < 2 { continue }
        rel := parts[1]
        if strings.TrimSpace(rel) == "" { continue }
        outPath := filepath.Join(wineDir, rel)
        switch hdr.Typeflag {
        case tar.TypeDir:
            _ = os.MkdirAll(outPath, os.FileMode(hdr.Mode))
        case tar.TypeReg:
            if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil { application.Get().Event.Emit(EventSetupError, "ERR_WRITE_FILE"); return "ERR_WRITE_FILE" }
            of, e2 := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
            if e2 != nil { application.Get().Event.Emit(EventSetupError, "ERR_WRITE_FILE"); return "ERR_WRITE_FILE" }
            if _, e3 := io.Copy(of, tr); e3 != nil { _ = of.Close(); application.Get().Event.Emit(EventSetupError, "ERR_WRITE_FILE"); return "ERR_WRITE_FILE" }
            _ = of.Close()
        default:
        }
        application.Get().Event.Emit(EventSetupProgress, map[string]interface{}{"phase": "extract", "line": rel})
    }
    _ = os.Remove(tmp)
    wineBin := filepath.Join(wineDir, "files", "bin", "wine")
    if _, err := os.Stat(wineBin); err != nil {
        wow := filepath.Join(wineDir, "files", "bin-wow64", "wine")
        if _, er2 := os.Stat(wow); er2 == nil {
            wineBin = wow
        } else {
            alt := filepath.Join(wineDir, "files", "bin", "wine64")
            if _, er3 := os.Stat(alt); er3 == nil { wineBin = alt } else { application.Get().Event.Emit(EventSetupError, "ERR_WINE_NOT_AVAILABLE"); return "ERR_WINE_NOT_AVAILABLE" }
        }
    }
    pf := filepath.Join(base, "prefix")
    _ = os.MkdirAll(pf, 0755)
    application.Get().Event.Emit(EventSetupStatus, "winetricks")
    wt := exec.Command("bash", "-c", "WINE='"+wineBin+"' WINEPREFIX='"+pf+"' winetricks vkd3d dxvk dxvk_nvapi0061")
    if err := streamCmd(wt, "winetricks"); err != nil {
        application.Get().Event.Emit(EventSetupError, "ERR_WINETRICKS")
        return "ERR_WINETRICKS"
    }
    application.Get().Event.Emit(EventSetupDone, struct{}{})
    return ""
}

func downloadWithProgress(url string, dest string, phase string) error {
    resp, err := http.Get(url)
    if err != nil { return err }
    defer resp.Body.Close()
    if resp.StatusCode != 200 { return io.ErrUnexpectedEOF }
    if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil { return err }
    f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
    if err != nil { return err }
    defer f.Close()
    buf := make([]byte, 256*1024)
    var total int64
    for {
        n, e := resp.Body.Read(buf)
        if n > 0 {
            if _, werr := f.Write(buf[:n]); werr != nil { return werr }
            total += int64(n)
            application.Get().Event.Emit(EventSetupProgress, map[string]interface{}{"phase": phase, "line": total})
        }
        if e != nil { if e == io.EOF { break } else { return e } }
    }
    return nil
}

func streamCmd(cmd *exec.Cmd, phase string) error {
    stdout, _ := cmd.StdoutPipe()
    stderr, _ := cmd.StderrPipe()
    if err := cmd.Start(); err != nil {
        return err
    }
    go func() {
        sc := bufio.NewScanner(stdout)
        for sc.Scan() {
            application.Get().Event.Emit(EventSetupProgress, map[string]interface{}{"phase": phase, "stream": "stdout", "line": sc.Text()})
        }
    }()
    go func() {
        sc := bufio.NewScanner(stderr)
        for sc.Scan() {
            application.Get().Event.Emit(EventSetupProgress, map[string]interface{}{"phase": phase, "stream": "stderr", "line": sc.Text()})
        }
    }()
    return cmd.Wait()
}
