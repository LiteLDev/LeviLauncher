package vcruntime

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/corpix/uarand"
	"github.com/google/go-github/v30/github"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	EventEnsureStart    = "vcruntime.ensure.start"
	EventEnsureProgress = "vcruntime.ensure.progress"
	EventEnsureDone     = "vcruntime.ensure.done"

	EventDownloadStart    = "vcruntime.download.start"
	EventDownloadProgress = "vcruntime.download.progress"
	EventDownloadDone     = "vcruntime.download.done"
	EventDownloadError    = "vcruntime.download.error"
	EventMissing          = "vcruntime.missing"
)

type EnsureProgress struct {
	Downloaded int64
	Total      int64
}

var (
	mu       sync.Mutex
	ensuring bool
)

//go:embed vcruntime140_1.dll
var embeddedVcruntime []byte

func bytesSHA256(b []byte) []byte { h := sha256.Sum256(b); return h[:] }

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
	return h.Sum(nil), nil
}

func IsInstalled() bool {
	psCmd := `Get-ItemProperty HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*, HKLM:\SOFTWARE\Wow6432Node\Microsoft\Windows\CurrentVersion\Uninstall\* | Where-Object { $_.DisplayName -like "*Visual C++*" } | Select-Object -ExpandProperty DisplayName`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	output := string(out)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if strings.Contains(l, "Visual C++") && strings.Contains(l, "X64") || strings.Contains(l, "x64"){
			if strings.Contains(l, "2022") && strings.Contains(l, "Minimum Runtime") {		
				return true
			}
		}
	}
	return false
}

func EnsureInteractive(ctx context.Context) {
	mu.Lock()
	if ensuring {
		mu.Unlock()
		return
	}
	ensuring = true
	mu.Unlock()
	defer func() {
		mu.Lock()
		ensuring = false
		mu.Unlock()
	}()

	if IsInstalled() {
		return
	}

	application.Get().Event.Emit(EventEnsureStart, struct{}{})

	dir, _ := apppath.InstallersDir()
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "LeviLauncher", "Installers")
	}
	_ = os.MkdirAll(dir, 0755)
	dlPath := filepath.Join(dir, "vc_redist.x64.exe")
	tmpPath := dlPath + ".part"

	downloadURL := "https://aka.ms/vc14/vc_redist.x64.exe"

	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		application.Get().Event.Emit(EventDownloadError, err.Error())
		application.Get().Event.Emit(EventEnsureDone, false)
		return
	}
	req.Header.Set("User-Agent", uarand.GetRandom())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		application.Get().Event.Emit(EventDownloadError, err.Error())
		application.Get().Event.Emit(EventEnsureDone, false)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		application.Get().Event.Emit(EventDownloadError, resp.Status)
		application.Get().Event.Emit(EventEnsureDone, false)
		return
	}

	application.Get().Event.Emit(EventDownloadStart, resp.ContentLength)

	f, err := os.Create(tmpPath)
	if err != nil {
		application.Get().Event.Emit(EventDownloadError, err.Error())
		application.Get().Event.Emit(EventEnsureDone, false)
		return
	}

	var downloaded int64
	buf := make([]byte, 64*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				application.Get().Event.Emit(EventDownloadError, werr.Error())
				application.Get().Event.Emit(EventEnsureDone, false)
				return
			}
			downloaded += int64(n)
			application.Get().Event.Emit(EventDownloadProgress, EnsureProgress{Downloaded: downloaded, Total: resp.ContentLength})
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			application.Get().Event.Emit(EventDownloadError, rerr.Error())
			application.Get().Event.Emit(EventEnsureDone, false)
			return
		}
	}
	f.Close()

	if err := os.Rename(tmpPath, dlPath); err != nil {
		_ = os.Remove(dlPath) 
		if err := os.Rename(tmpPath, dlPath); err != nil {
			application.Get().Event.Emit(EventDownloadError, err.Error())
			application.Get().Event.Emit(EventEnsureDone, false)
			return
		}
	}

	application.Get().Event.Emit(EventDownloadDone, struct{}{})

	cmd := exec.Command(dlPath, "/install", "/quiet", "/norestart")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Run(); err != nil {
		log.Println("failed to run VC Runtime installer:", err)
	}

	installed := false
	for i := 0; i < 30; i++ {
		if IsInstalled() {
			installed = true
			break
		}
		time.Sleep(1 * time.Second)
	}
	log.Println("VC Runtime installed:", installed)
	application.Get().Event.Emit(EventEnsureDone, installed)
}

func EnsureForVersion(ctx context.Context, versionDir string) bool {
	if strings.TrimSpace(versionDir) == "" {
		return false
	}
	dest := filepath.Join(versionDir, "vcruntime140_1.dll")

	if len(embeddedVcruntime) > 0 {
		needWrite := true
		if fi, err := os.Stat(dest); err == nil && fi.Size() > 0 {
			if fh, err := fileSHA256(dest); err == nil {
				if bytes.Equal(fh, bytesSHA256(embeddedVcruntime)) {
					needWrite = false
				}
			}
		}
		if needWrite {
			_ = os.MkdirAll(versionDir, 0755)
			tmp := dest + ".tmp"
			if err := os.WriteFile(tmp, embeddedVcruntime, 0644); err != nil {
				_ = os.Remove(tmp)
				return false
			}
			if err := os.Rename(tmp, dest); err != nil {
				_ = os.Remove(tmp)
				return false
			}
		}
		return true
	}

	if _, err := os.Stat(dest); err == nil {
		return true
	}
	return false
}

func EnsureLatest(ctx context.Context, contentDir string) {
	if strings.TrimSpace(contentDir) == "" {
		return
	}
	dest := filepath.Join(contentDir, "vcruntime140_1.dll")
	tmp := dest + ".tmp"
	if _, err := os.Stat(dest); err == nil {
		application.Get().Event.Emit(EventEnsureStart, struct{}{})
		application.Get().Event.Emit(EventEnsureDone, true)

		return
	}
	if _, err := os.Stat(tmp); err == nil {
		if err := os.Rename(tmp, dest); err != nil {
			if in, e1 := os.Open(tmp); e1 == nil {
				defer in.Close()
				if out, e2 := os.Create(dest); e2 == nil {
					if _, e3 := io.Copy(out, in); e3 == nil {
						out.Close()
						_ = os.Remove(tmp)
						application.Get().Event.Emit(EventEnsureStart, struct{}{})
						application.Get().Event.Emit(EventEnsureDone, true)
						return
					}
					out.Close()
					_ = os.Remove(dest)
				}
			}
		} else {
			application.Get().Event.Emit(EventEnsureStart, struct{}{})
			application.Get().Event.Emit(EventEnsureDone, true)
			return
		}
	}
	application.Get().Event.Emit(EventEnsureStart, struct{}{})
	c, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var downloadURL string
	client := github.NewClient(nil)
	rel, _, err := client.Repositories.GetLatestRelease(c, "LiteLDev", "vcproxy")
	if err == nil && rel != nil {
		for _, asset := range rel.Assets {
			if strings.EqualFold(asset.GetName(), "vcruntime140_1.dll") {
				downloadURL = asset.GetBrowserDownloadURL()
				break
			}
		}
	} else if err != nil {
		log.Printf("vcruntime.EnsureLatest: 获取最新 release 失败: %v", err)
	}
	if downloadURL == "" {
		downloadURL = "https://github.com/LiteLDev/vcproxy/releases/download/v1.0.0/vcruntime140_1.dll"
	}
	req, err := http.NewRequestWithContext(c, "GET", downloadURL, nil)
	if err != nil {
		log.Printf("vcruntime.EnsureLatest: 构造请求失败: %v", err)
		application.Get().Event.Emit(EventEnsureDone, false)
		return
	}
	req.Header.Set("User-Agent", uarand.GetRandom())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("vcruntime.EnsureLatest: 请求失败: %v", err)
		application.Get().Event.Emit(EventEnsureDone, false)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("vcruntime.EnsureLatest: HTTP %s", resp.Status)
		application.Get().Event.Emit(EventEnsureDone, false)
		return
	}
	application.Get().Event.Emit(EventEnsureProgress, EnsureProgress{Downloaded: 0, Total: resp.ContentLength})
	_ = os.Remove(tmp)
	f, err := os.Create(tmp)
	if err != nil {
		log.Printf("vcruntime.EnsureLatest: 创建文件失败: %v", err)
		application.Get().Event.Emit(EventEnsureDone, false)
		return
	}
	defer f.Close()
	var downloaded int64
	buf := make([]byte, 64*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				log.Printf("vcruntime.EnsureLatest: 写入失败: %v", werr)
				application.Get().Event.Emit(EventEnsureDone, false)
				return
			}
			downloaded += int64(n)
			application.Get().Event.Emit(EventEnsureProgress, EnsureProgress{Downloaded: downloaded, Total: resp.ContentLength})
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			log.Printf("vcruntime.EnsureLatest: 读取失败: %v", rerr)
			application.Get().Event.Emit(EventEnsureDone, false)
			return
		}
	}
	if err := os.Rename(tmp, dest); err != nil {
		log.Printf("vcruntime.EnsureLatest: 移动到目标失败: %v", err)
		if in, e1 := os.Open(tmp); e1 == nil {
			defer in.Close()
			if out, e2 := os.Create(dest); e2 == nil {
				if _, e3 := io.Copy(out, in); e3 == nil {
					out.Close()
					_ = os.Remove(tmp)
					log.Printf("vcruntime.EnsureLatest: 复制回退成功: %s", dest)
					application.Get().Event.Emit(EventEnsureDone, true)
					return
				}
				out.Close()
				_ = os.Remove(dest)
			}
		}
		_ = os.Remove(tmp)
		application.Get().Event.Emit(EventEnsureDone, false)
		return
	}
	log.Printf("vcruntime.EnsureLatest: 已下载 vcruntime140_1.dll 到 %s", dest)
	application.Get().Event.Emit(EventEnsureDone, true)
}

func EnsureEmbedded(contentDir string, embedded []byte) {
	if strings.TrimSpace(contentDir) == "" {
		return
	}
	dest := filepath.Join(contentDir, "vcruntime140_1.dll")
	if _, err := os.Stat(dest); err == nil {
		return
	}
	if len(embedded) == 0 {
		return
	}
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, embedded, 0o644); err != nil {
		_ = os.Remove(tmp)
		return
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return
	}
}
