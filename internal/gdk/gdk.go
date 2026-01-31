package gdk

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/downloader"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

const (
	EventDownloadStatus   = "gdk_download_status"
	EventDownloadProgress = "gdk_download_progress"
	EventDownloadDone     = "gdk_download_done"
	EventDownloadError    = "gdk_download_error"
	EventInstallStart     = "gdk_install_start"
	EventInstallDone      = "gdk_install_done"
	EventInstallError     = "gdk_install_error"
)

type DownloadProgress struct {
	Downloaded int64
	Total      int64
	Dest       string
}

var gdkMgr = downloader.NewManager(
	downloader.Events{
		Status:   EventDownloadStatus,
		Progress: EventDownloadProgress,
		Done:     EventDownloadDone,
		Error:    EventDownloadError,
		ProgressFactory: func(p downloader.DownloadProgress) any {
			return DownloadProgress{Downloaded: p.Downloaded, Total: p.Total, Dest: p.Dest}
		},
	},
	downloader.Options{Throttle: 250 * time.Millisecond, Resume: false, RemoveOnCancel: true},
)

func IsInstalled() bool {
	p := `C:\\Program Files (x86)\\Microsoft GDK\\bin\\wdapp.exe`
	return utils.FileExists(p)
}

func StartDownload(ctx context.Context, url string) string {
	dir, err := apppath.InstallersDir()
	if err != nil {
		application.Get().Event.Emit(EventDownloadError, err.Error())
		return ""
	}
	name := deriveFilename(url)
	dest := filepath.Join(dir, name)
	return gdkMgr.Start(ctx, stripFilenameParam(url), dest)
}

func downloadFile(ctx context.Context, src string, dest string) error {
	return fmt.Errorf("deprecated")
}

func CancelDownload() { gdkMgr.Cancel() }

func InstallFromZip(ctx context.Context, zipPath string) string {
	defer func() {
		if strings.TrimSpace(zipPath) != "" {
			_ = os.Remove(zipPath)
		}
	}()
	if strings.TrimSpace(zipPath) == "" || !utils.FileExists(zipPath) {
		return "ERR_ZIP_NOT_FOUND"
	}
	tmpDir, err := os.MkdirTemp("", "gdk_zip_")
	if err != nil {
		return "ERR_CREATE_TEMP"
	}
	defer os.RemoveAll(tmpDir)
	if err := unzip(zipPath, tmpDir); err != nil {
		return "ERR_UNZIP"
	}
	var msi string
	candidate := filepath.Join(tmpDir, "Installers", "Microsoft GDK x86 Common-x86_en-us.msi")
	if utils.FileExists(candidate) {
		msi = candidate
	} else {
		_ = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".msi") && strings.Contains(strings.ToLower(info.Name()), "gdk") {
				msi = path
				return io.EOF
			}
			return nil
		})
		if msi == "" {
			return "ERR_MSI_NOT_FOUND"
		}
	}
	application.Get().Event.Emit(EventInstallStart, msi)
	cmd := exec.Command("msiexec", "/i", msi)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Run(); err != nil {
		application.Get().Event.Emit(EventInstallError, err.Error())
		return "ERR_START_MSI"
	}
	application.Get().Event.Emit(EventInstallDone, "ok")
	return ""
}

func unzip(src string, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		fp := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fp, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(fp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, f.Mode())
		if err != nil {
			_ = rc.Close()
			return err
		}
		if _, err = io.Copy(out, rc); err != nil {
			_ = out.Close()
			_ = rc.Close()
			return err
		}
		_ = out.Close()
		_ = rc.Close()
	}
	return nil
}

func deriveFilename(raw string) string {
	n := "GDK.zip"
	parts := strings.Split(raw, "/")
	if len(parts) > 0 {
		last := parts[len(parts)-1]
		if strings.TrimSpace(last) != "" {
			n = last
		}
	}
	if !strings.HasSuffix(strings.ToLower(n), ".zip") {
		n += ".zip"
	}
	return n
}

func stripFilenameParam(raw string) string {
	if idx := strings.Index(raw, "?"); idx != -1 {
		return raw[:idx]
	}
	return raw
}
