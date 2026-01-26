package mcservice

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	json "github.com/goccy/go-json"

	"github.com/corpix/uarand"
)

type KnownFolder struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func FetchHistoricalVersions(preferCN bool) map[string]interface{} {
	const githubURL = "https://raw.githubusercontent.com/LiteLDev/minecraft-windows-gdk-version-db/refs/heads/main/historical_versions.json"
	const proxyURL = "https://github.bibk.top/LiteLDev/minecraft-windows-gdk-version-db/raw/refs/heads/main/historical_versions.json"
	const gitcodeURL = "https://raw.gitcode.com/dreamguxiang/minecraft-windows-gdk-version-db/raw/main/historical_versions.json"

	urls := []string{githubURL, proxyURL, gitcodeURL}
	if preferCN {
		urls = []string{gitcodeURL, proxyURL, githubURL}
	}

	client := &http.Client{Timeout: 5 * time.Second}
	var lastErr error
	for _, u := range urls {
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("User-Agent", uarand.GetRandom())

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		var obj map[string]interface{}
		func() {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				lastErr = fmt.Errorf("status %d", resp.StatusCode)
				return
			}
			if derr := json.NewDecoder(resp.Body).Decode(&obj); derr != nil {
				lastErr = derr
				return
			}
		}()

		if lastErr == nil && obj != nil {
			obj["_source"] = u
			return obj
		}
	}
	if lastErr != nil {
		log.Println("FetchHistoricalVersions error:", lastErr)
	}
	return map[string]interface{}{}
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
	client := &http.Client{Timeout: time.Duration(timeoutMs) * time.Millisecond}
	results := make([]map[string]interface{}, 0, len(urls))
	for _, u := range urls {
		start := time.Now()
		ok := false
		req, err := http.NewRequest("HEAD", strings.TrimSpace(u), nil)
		if err == nil {
			req.Header.Set("User-Agent", uarand.GetRandom())
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