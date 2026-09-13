package mcservice

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/msixvc"
	"github.com/liteldev/LeviLauncher/internal/uwpdownload"
)

func FetchUWPVersions(ctx context.Context) ([]uwpdownload.Version, error) {
	return uwpdownload.FetchCatalog(ctx)
}

func StartUWPDownload(ctx context.Context, version, updateID, channel string) (string, error) {
	filename, err := uwpdownload.Filename(version, channel)
	if err != nil {
		return "", err
	}
	source, err := uwpdownload.ResolvePackage(ctx, updateID)
	if err != nil {
		return "", fmt.Errorf("ERR_UWP_DOWNLOAD_URL: %w", err)
	}
	return msixvc.StartNamedDownload(ctx, source.URL, filename, source.Verify)
}

func ResolveDownloadedUWP(version, channel string) string {
	filename, err := uwpdownload.Filename(version, channel)
	if err != nil {
		return ""
	}
	dir, err := apppath.InstallersDir()
	if err != nil {
		return ""
	}
	path := filepath.Join(dir, filename)
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
		return path
	}
	return ""
}

func DeleteDownloadedUWP(version, channel string) string {
	path := ResolveDownloadedUWP(version, channel)
	if path == "" {
		return "ERR_UWP_PACKAGE_NOT_FOUND"
	}
	if err := os.Remove(path); err != nil {
		return "ERR_WRITE_TARGET"
	}
	return ""
}
