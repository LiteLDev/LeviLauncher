package mcservice

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/extractor"
	"github.com/liteldev/LeviLauncher/internal/msixvc"
	"github.com/liteldev/LeviLauncher/internal/peeditor"
	"github.com/liteldev/LeviLauncher/internal/preloader"
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

func StartMsixvcDownload(ctx context.Context, url string) string {
	return msixvc.StartDownload(ctx, url)
}
func ResumeMsixvcDownload() { msixvc.Resume() }
func CancelMsixvcDownload() { msixvc.Cancel() }

func InstallExtractMsixvc(ctx context.Context, name string, folderName string, isPreview bool) string {
	n := strings.TrimSpace(name)
	if n == "" {
		return "ERR_MSIXVC_NOT_SPECIFIED"
	}
	inPath := n
	if !filepath.IsAbs(inPath) {
		if dir, err := utils.GetInstallerDir(); err == nil && dir != "" {
			inPath += ".msixvc"
			inPath = filepath.Join(dir, inPath)
		}
	}
	if !utils.FileExists(inPath) {
		return "ERR_MSIXVC_NOT_FOUND"
	}
	vdir, err := utils.GetVersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	outDir := filepath.Join(vdir, strings.TrimSpace(folderName))
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "ERR_CREATE_TARGET_DIR"
	}
	stopCh := make(chan struct{})
	go func(dir string) {
		ticker := time.NewTicker(300 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				var totalBytes int64
				var files int64
				_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
					if err != nil {
						return nil
					}
					if d.IsDir() {
						return nil
					}
					if fi, e := os.Stat(path); e == nil {
						totalBytes += fi.Size()
						files++
					}
					return nil
				})
				application.Get().Event.Emit(EventExtractProgress, types.ExtractProgress{Dir: dir, Files: files, Bytes: totalBytes, Ts: time.Now().UnixMilli()})
			case <-stopCh:
				return
			}
		}
	}(outDir)

	rc, msg := extractor.MiHoYo(inPath, outDir)
	close(stopCh)
	if rc != 0 {
		application.Get().Event.Emit(EventExtractError, msg)
		if strings.TrimSpace(msg) == "" {
			msg = "ERR_APPX_INSTALL_FAILED"
		}
		return msg
	}
	_ = vcruntime.EnsureForVersion(ctx, outDir)
	_ = preloader.EnsureForVersion(ctx, outDir)
	_ = peeditor.EnsureForVersion(ctx, outDir)
	_ = peeditor.RunForVersion(ctx, outDir)
	application.Get().Event.Emit(EventExtractDone, outDir)
	return ""
}

func ResolveDownloadedMsixvc(version string, versionType string) string {
	dir, err := utils.GetInstallerDir()
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
	dir, err := utils.GetInstallerDir()
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
	dir, err := utils.GetInstallerDir()
	if err != nil {
		return ""
	}
	return dir
}

func GetVersionsDir() string {
	dir, err := utils.GetVersionsDir()
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
