package mcservice

import (
	"context"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/uwp"
	"github.com/liteldev/LeviLauncher/internal/versions"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func InstallExtractAppx(ctx context.Context, archivePath, folderName, channel string) string {
	archivePath = strings.TrimSpace(archivePath)
	if archivePath == "" {
		return "ERR_UWP_PACKAGE_NOT_SPECIFIED"
	}
	if !filepath.IsAbs(archivePath) {
		if filepath.Base(archivePath) != archivePath {
			return "ERR_PATH_ESCAPE"
		}
		dir, err := apppath.InstallersDir()
		if err != nil {
			return "ERR_ACCESS_INSTALLERS_DIR"
		}
		archivePath = filepath.Join(dir, archivePath)
	}
	folderName = strings.TrimSpace(folderName)
	if versions.ValidateFolderName(folderName) != "" || folderName == "." || folderName == ".." {
		return "ERR_INVALID_FOLDER_NAME"
	}
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel != "release" && channel != "beta" && channel != "preview" {
		return "ERR_UWP_CHANNEL"
	}
	vdir, err := apppath.VersionsDir()
	if err != nil {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	target := filepath.Join(vdir, folderName)
	var lastProgress time.Time
	_, err = uwp.Install(ctx, archivePath, target, uwp.Options{
		Prepare: func(stage string, manifest uwp.Manifest) error {
			if manifest.IsPreview() {
				channel = "preview"
			} else if channel != "beta" {
				channel = "release"
			}
			return versions.WriteMeta(stage, versions.VersionMeta{Name: folderName, GameVersion: manifest.GameVersion(), Type: channel, PackageType: versions.PackageTypeUWP, CreatedAt: time.Now()})
		},
		Progress: func(p uwp.Progress) {
			if p.Current < p.Total && time.Since(lastProgress) < 100*time.Millisecond {
				return
			}
			lastProgress = time.Now()
			if app := application.Get(); app != nil {
				app.Event.Emit(EventExtractProgress, types.ExtractProgress{Dir: target, Bytes: p.FileCurrent, TotalBytes: p.FileTotal, GlobalCurrent: p.Current, GlobalTotal: p.Total, CurrentFile: p.File, Ts: time.Now().UnixMilli()})
			}
		},
	})
	if err != nil {
		code := uwp.ErrorCode(err)
		log.Printf("UWP install failed for %s: %v", folderName, err)
		if app := application.Get(); app != nil {
			app.Event.Emit(EventExtractError, code)
		}
		return code
	}
	if app := application.Get(); app != nil {
		app.Event.Emit(EventExtractDone, target)
	}
	return ""
}
