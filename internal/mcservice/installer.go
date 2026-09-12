package mcservice

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/leviloader"
	"github.com/liteldev/LeviLauncher/internal/msixvc"
	"github.com/liteldev/LeviLauncher/internal/nativeinstall"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/liteldev/LeviLauncher/internal/versions"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type VersionStatus struct {
	Version      string `json:"version"`
	IsInstalled  bool   `json:"isInstalled"`
	IsDownloaded bool   `json:"isDownloaded"`
	Type         string `json:"type"`
	PackageType  string `json:"packageType"`
}

func StartMsixvcDownload(ctx context.Context, url string, md5sum string) string {
	return msixvc.StartDownload(ctx, url, md5sum)
}
func ResumeMsixvcDownload()                { msixvc.Resume() }
func CancelMsixvcDownload()                { msixvc.Cancel() }
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
	folder := strings.TrimSpace(folderName)
	if folder == "" || folder == "." || folder == ".." || filepath.Base(folder) != folder || strings.ContainsAny(folder, "<>:\"/\\|?*") {
		return "ERR_INVALID_FOLDER_NAME"
	}
	outDir := filepath.Join(vdir, folder)
	_, err = nativeinstall.Install(ctx, inPath, outDir, nativeinstall.Options{
		CacheDir:           filepath.Join(apppath.ConfigDir(), "microsoft-account"),
		RequireFullLicense: true,
		Prepare:            func(staging string) error { return leviloader.EnsureForVersion(ctx, staging) },
		Progress: func(p nativeinstall.Progress) {
			application.Get().Event.Emit(EventExtractProgress, types.ExtractProgress{
				Dir: outDir, Bytes: p.FileCurrent, TotalBytes: p.FileTotal,
				GlobalCurrent: p.Current, GlobalTotal: p.Total, CurrentFile: p.File, Ts: time.Now().UnixMilli(),
			})
		},
	})
	if err != nil {
		code := "ERR_NATIVE_MSIXVC"
		var failure *nativeinstall.Error
		if errors.As(err, &failure) {
			code = failure.Code
		}
		if errors.Is(err, context.Canceled) {
			code = "ERR_CANCELED"
		}
		application.Get().Event.Emit(EventExtractError, code)
		return code
	}
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
		channel := strings.ToLower(strings.TrimSpace(versionType))
		if vl == bl || ((channel == "release" || channel == "preview") && (bl == channel+" "+vl || bl == "minecraft-"+channel+"-"+vl)) {
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
	return GetVersionStatusForPackage(version, versionType, "gdk")
}

func GetVersionStatusForPackage(version, channel, packageType string) VersionStatus {
	return versionStatusForPackage(version, channel, packageType, ListVersionMetas())
}

func versionStatusForPackage(version, channel, packageType string, metas []versions.VersionMeta) VersionStatus {
	platform := versions.NormalizePackageType(packageType)
	status := VersionStatus{Version: version, Type: channel, PackageType: platform}
	if platform == "uwp" {
		status.IsDownloaded = ResolveDownloadedUWP(version, strings.ToLower(channel)) != ""
	} else {
		status.IsDownloaded = ResolveDownloadedMsixvc(version, channel) != ""
	}
	for _, meta := range metas {
		if meta.GameVersion == version && strings.EqualFold(meta.Type, channel) && versions.NormalizePackageType(meta.PackageType) == platform {
			status.IsInstalled = true
			break
		}
	}
	return status
}

func GetAllVersionsStatus(versionsList []map[string]interface{}) []VersionStatus {
	var results []VersionStatus
	metas := ListVersionMetas()
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
		packageType, _ := versionData["packageType"].(string)
		status := versionStatusForPackage(version, versionType, packageType, metas)
		results = append(results, status)
	}
	return results
}
