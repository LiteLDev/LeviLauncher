package mcservice

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "strings"
    "time"

    "github.com/corpix/uarand"
)

type KnownFolder struct {
    Name string `json:"name"`
    Path string `json:"path"`
}

func FetchHistoricalVersions(preferCN bool) map[string]interface{} {
    const githubURL = "https://raw.githubusercontent.com/LiteLDev/minecraft-windows-gdk-version-db/refs/heads/main/historical_versions.json"
    const gitcodeURL = "https://github.bibk.top/LiteLDev/minecraft-windows-gdk-version-db/raw/refs/heads/main/historical_versions.json"
    urls := []string{githubURL, gitcodeURL}
    if preferCN {
        urls = []string{gitcodeURL, githubURL}
    }
    client := &http.Client{Timeout: 8 * time.Second}
    var lastErr error
    for _, u := range urls {
        req, err := http.NewRequest(http.MethodGet, u, nil)
        if err != nil {
            lastErr = err
            continue
        }
        req.Header.Set("Accept", "application/json")
        req.Header.Set("Cache-Control", "no-cache")
        resp, err := client.Do(req)
        if err != nil {
            lastErr = err
            continue
        }
        func() {
            defer resp.Body.Close()
            if resp.StatusCode != http.StatusOK {
                lastErr = fmt.Errorf("status %d", resp.StatusCode)
                return
            }
            dec := json.NewDecoder(resp.Body)
            var obj map[string]interface{}
            if derr := dec.Decode(&obj); derr != nil {
                lastErr = derr
                return
            }
        }()
        if lastErr == nil {
            resp2, err2 := client.Get(u)
            if err2 != nil {
                lastErr = err2
                continue
            }
            defer resp2.Body.Close()
            var obj2 map[string]interface{}
            if derr2 := json.NewDecoder(resp2.Body).Decode(&obj2); derr2 != nil {
                lastErr = derr2
                continue
            }
            obj2["_source"] = u
            return obj2
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

// GetLicenseInfo removed per project preference to keep license in README only