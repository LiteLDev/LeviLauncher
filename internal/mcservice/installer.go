package mcservice

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/extractor"
	"github.com/liteldev/LeviLauncher/internal/msixvc"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/liteldev/LeviLauncher/internal/vcruntime"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type VersionStatus struct {
	Version      string `json:"version"`
	IsInstalled  bool   `json:"isInstalled"`
	IsDownloaded bool   `json:"isDownloaded"`
	Type         string `json:"type"`
}

func StartMsixvcDownload(ctx context.Context, url string, md5sum string) string {
	return msixvc.StartDownload(ctx, url, md5sum)
}
func ResumeMsixvcDownload() { msixvc.Resume() }
func CancelMsixvcDownload() { msixvc.Cancel() }
func CancelMsixvcDownloadTask(dest string) { msixvc.CancelTask(dest) }

func InstallExtractMsixvc(ctx context.Context, name string, folderName string, isPreview bool) string {
	n := strings.TrimSpace(name)
	if n == "" {
		return "ERR_MSIXVC_NOT_SPECIFIED"
	}
	inPath := n
	if !filepath.IsAbs(inPath) {
		if dir, err := apppath.InstallersDir(); err == nil && dir != "" {
			inPath += ".msixvc"
			inPath = filepath.Join(dir, inPath)
		}
	}
	if !utils.FileExists(inPath) {
		return "ERR_MSIXVC_NOT_FOUND"
	}
	vdir, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	outDir := filepath.Join(vdir, strings.TrimSpace(folderName))
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "ERR_CREATE_TARGET_DIR"
	}
	pipeName := fmt.Sprintf(`\\.\pipe\levi_msixvc_progress_%d`, time.Now().UnixNano())
	ln, err := winio.ListenPipe(pipeName, nil)
	if err != nil {
		return "ERR_CREATE_PIPE"
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			text := scanner.Text()
			text = strings.ReplaceAll(text, "\\", "/")
			var p struct {
				File          string `json:"file"`
				Current       int64  `json:"current"`
				Total         int64  `json:"total"`
				GlobalCurrent int64  `json:"global_current"`
				GlobalTotal   int64  `json:"global_total"`
			}
			if err := json.Unmarshal([]byte(text), &p); err == nil {
				application.Get().Event.Emit(EventExtractProgress, types.ExtractProgress{
					Dir:           outDir,
					Bytes:         p.Current,
					TotalBytes:    p.Total,
					GlobalCurrent: p.GlobalCurrent,
					GlobalTotal:   p.GlobalTotal,
					CurrentFile:   p.File,
					Ts:            time.Now().UnixMilli(),
				})
			}
		}
	}()

	rc, msg := extractor.GetWithPipe(inPath, outDir, pipeName)
	if rc != 0 {
		application.Get().Event.Emit(EventExtractError, msg)
		if strings.TrimSpace(msg) == "" {
			msg = "ERR_APPX_INSTALL_FAILED"
		}
		_ = os.RemoveAll(outDir)
		return msg
	}
	_ = vcruntime.EnsureForVersion(ctx, outDir)
	application.Get().Event.Emit(EventExtractDone, outDir)
	return ""
}

func ResolveDownloadedMsixvc(version string, versionType string) string {
	dir, err := apppath.InstallersDir()
	if err != nil || strings.TrimSpace(dir) == "" {
		return ""
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".msixvc") {
			continue
		}
		ext := filepath.Ext(name)
		if strings.ToLower(ext) == ".msixvc" {
			name = name[:len(name)-len(ext)]
		} else {
			name = strings.TrimSuffix(name, ".msixvc")
		}
		b := strings.TrimSpace(name)
		v := strings.TrimSpace(version)
		bl := strings.ToLower(b)
		vl := strings.ToLower(v)
		if vl == bl {
			return name
		}
	}
	return ""
}

func DeleteDownloadedMsixvc(version string, versionType string) string {
	name := strings.TrimSpace(ResolveDownloadedMsixvc(version, versionType))
	if name == "" {
		return "ERR_MSIXVC_NOT_FOUND"
	}
	dir, err := apppath.InstallersDir()
	if err != nil || strings.TrimSpace(dir) == "" {
		return "ERR_ACCESS_INSTALLERS_DIR"
	}
	path := filepath.Join(dir, name+".msixvc")
	if !utils.FileExists(path) {
		alt := filepath.Join(dir, name)
		if utils.FileExists(alt) {
			path = alt
		}
	}
	if !utils.FileExists(path) {
		return "ERR_MSIXVC_NOT_FOUND"
	}
	if err := os.Remove(path); err != nil {
		return "ERR_WRITE_TARGET"
	}
	return ""
}

func GetInstallerDir() string {
	dir, err := apppath.InstallersDir()
	if err != nil {
		return ""
	}
	return dir
}

func GetVersionsDir() string {
	dir, err := apppath.VersionsDir()
	if err != nil {
		return ""
	}
	return dir
}

func GetVersionStatus(version string, versionType string) VersionStatus {
	status := VersionStatus{Version: version, Type: versionType, IsInstalled: false, IsDownloaded: false}
	if name := ResolveDownloadedMsixvc(version, versionType); strings.TrimSpace(name) != "" {
		status.IsDownloaded = true
	}
	return status
}

func GetAllVersionsStatus(versionsList []map[string]interface{}) []VersionStatus {
	var results []VersionStatus
	for _, versionData := range versionsList {
		version, ok := versionData["version"].(string)
		if !ok {
			version, ok = versionData["short"].(string)
			if !ok {
				continue
			}
		}
		versionType, ok := versionData["type"].(string)
		if !ok {
			versionType = "release"
		}
		status := GetVersionStatus(version, versionType)
		results = append(results, status)
	}
	return results
}
