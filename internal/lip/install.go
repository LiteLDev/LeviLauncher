package lip

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/corpix/uarand"
	json "github.com/goccy/go-json"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	EventLipInstallStatus   = "lip_install_status"
	EventLipInstallProgress = "lip_install_progress"
	EventLipInstallDone     = "lip_install_done"
	EventLipInstallError    = "lip_install_error"
)

func LipDir() string {
	return filepath.Join(config.ConfigDir(), "lip")
}

func LipExePath() string {
	exe := "lip"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	return filepath.Join(LipDir(), exe)
}

func IsInstalled() bool {
	return utils.FileExists(LipExePath())
}

func GetVersion() string {
	if !IsInstalled() {
		return ""
	}
	cmd := exec.Command(LipExePath(), "--version")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func GetLatestVersion() (string, error) {
	apis := []string{
		"https://api.github.com/repos/futrime/lip/releases/latest",
		"https://cdn.gh-proxy.org/https://api.github.com/repos/futrime/lip/releases/latest",
	}

	var payload struct {
		TagName string `json:"tag_name"`
	}

	for _, api := range apis {
		req, err := http.NewRequest("GET", api, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", uarand.GetRandom())
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			continue
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err == nil {
			resp.Body.Close()
			return payload.TagName, nil
		}
		resp.Body.Close()
	}
	return "", fmt.Errorf("failed to fetch latest version")
}

func Install() string {
	application.Get().Event.Emit(EventLipInstallStatus, "checking_update")
	tagName, err := GetLatestVersion()
	if err != nil {
		return "ERR_FETCH_RELEASE_FAILED"
	}

	filename := "lip-cli-win-x64-self-contained.zip"
	downloadUrl := fmt.Sprintf("https://github.com/futrime/lip/releases/download/%s/%s", tagName, filename)

	tmpDir, err := os.MkdirTemp("", "lip_install_")
	if err != nil {
		return "ERR_CREATE_TEMP"
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, filename)

	application.Get().Event.Emit(EventLipInstallStatus, "downloading")

	if err := downloadFile(downloadUrl, zipPath); err != nil {
		proxyUrl := "https://gh-proxy.org/" + downloadUrl
		if err := downloadFile(proxyUrl, zipPath); err != nil {
			return "ERR_DOWNLOAD_FAILED"
		}
	}

	application.Get().Event.Emit(EventLipInstallStatus, "extracting")
	targetDir := LipDir()

	os.RemoveAll(targetDir)

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "ERR_CREATE_TARGET_DIR"
	}

	if err := unzip(zipPath, targetDir); err != nil {
		return "ERR_UNZIP_FAILED"
	}

	application.Get().Event.Emit(EventLipInstallDone, strings.TrimPrefix(tagName, "v"))
	return ""
}

func downloadFile(url string, dest string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", uarand.GetRandom())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	total := resp.ContentLength
	counter := &progressWriter{
		total: float64(total),
		onProgress: func(p float64, current float64, total float64) {
			application.Get().Event.Emit(EventLipInstallProgress, map[string]interface{}{
				"percentage": p,
				"current":    current,
				"total":      total,
			})
		},
	}
	_, err = io.Copy(out, io.TeeReader(resp.Body, counter))
	return err
}

type progressWriter struct {
	total      float64
	current    float64
	onProgress func(float64, float64, float64)
	lastUpdate int64
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.current += float64(n)
	now := time.Now().UnixMilli()
	if now-pw.lastUpdate > 100 { 
		if pw.total > 0 {
			pw.onProgress((pw.current/pw.total)*100, pw.current, pw.total)
		}
		pw.lastUpdate = now
	}
	return n, nil
}

func unzip(src string, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
