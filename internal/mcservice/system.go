package mcservice

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/gdkversions"
	"github.com/liteldev/LeviLauncher/internal/httpx"
	"github.com/liteldev/LeviLauncher/internal/xbox"
)

type KnownFolder struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

var gdkVersionCatalog = gdkversions.NewClient(httpx.NewClient(5*time.Second), xbox.GetPackageUpdateAuthorization)

func FetchHistoricalVersions(preferCN bool) map[string]interface{} {
	catalog := gdkVersionCatalog.Fetch(context.Background(), preferCN, filepath.Join(apppath.ConfigDir(), "gdk_pending_versions.json"))
	return map[string]interface{}{
		"file_version":    catalog.FileVersion,
		"releaseVersions": catalog.ReleaseVersions,
		"previewVersions": catalog.PreviewVersions,
		"_source":         catalog.Source,
	}
}

func ListKnownFolders() []KnownFolder {
	out := []KnownFolder{}
	home, _ := os.UserHomeDir()
	if strings.TrimSpace(home) == "" {
		home = os.Getenv("USERPROFILE")
	}
	add := func(name, p string) {
		if strings.TrimSpace(p) == "" {
			return
		}
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			out = append(out, KnownFolder{Name: name, Path: p})
		}
	}
	add("Home", home)
	if home != "" {
		add("Desktop", filepath.Join(home, "Desktop"))
		add("Downloads", filepath.Join(home, "Downloads"))
	}
	return out
}

func TestMirrorLatencies(urls []string, timeoutMs int) []map[string]interface{} {
	if timeoutMs <= 0 {
		timeoutMs = 7000
	}
	client := httpx.NewClient(time.Duration(timeoutMs) * time.Millisecond)
	results := make([]map[string]interface{}, 0, len(urls))
	for _, u := range urls {
		start := time.Now()
		ok := false
		req, err := http.NewRequest("HEAD", strings.TrimSpace(u), nil)
		if err == nil {
			httpx.ApplyDefaultHeaders(req)
			if resp, er := client.Do(req); er == nil {
				_ = resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 400 {
					ok = true
				}
			}
		}
		elapsed := time.Since(start).Milliseconds()
		results = append(results, map[string]interface{}{"url": u, "latencyMs": elapsed, "ok": ok})
	}
	return results
}
