package gdkversions

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func packageFixture(channel, version string) map[string]any {
	identity := "Microsoft.MinecraftUWP"
	if channel == "Preview" {
		identity = "Microsoft.MinecraftWindowsBeta"
	}
	filename := identity + "_" + version + "_x64__8wekyb3d8bbwe.msixvc"
	return map[string]any{
		"PackageFound": true, "AvailabilityDate": "2026-09-15T10:00:00Z",
		"PackageFiles": []map[string]any{{
			"FileName":     filename,
			"FileHash":     "not-an-md5",
			"CdnRootPaths": []string{"https://assets1.xboxlive.com/prefix/", "https://assets2.xboxlive.com/prefix", "https://assets1.xboxlive.com/prefix/"},
			"RelativeUrl":  "content/" + filename,
		}},
	}
}

func TestStoreCatalogLifecycle(t *testing.T) {
	historical := Version{Version: "Release 1.26.40.01", URLs: []string{"https://history.example/old.msixvc"}, Timestamp: 1, MD5: "old-checksum"}
	history := Catalog{FileVersion: 1, ReleaseVersions: []Version{historical}, PreviewVersions: []Version{}}
	release := "1.26.4501.0"
	storeFailed, historyFailed := false, false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cache-Control") != "no-cache" {
			t.Error("catalog requests must bypass caches")
		}
		if r.URL.Path == "/history" {
			if r.Header.Get("Authorization") != "" {
				t.Error("Xbox credentials leaked to historical catalog")
			}
			if historyFailed {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_ = json.NewEncoder(w).Encode(history)
			return
		}
		if r.Header.Get("Authorization") != "test-auth" {
			t.Error("Store request missing authorization")
		}
		if storeFailed {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if r.URL.Path == "/packages/"+releaseID {
			_ = json.NewEncoder(w).Encode(packageFixture("Release", release))
		} else if r.URL.Path == "/packages/"+previewID {
			_ = json.NewEncoder(w).Encode(packageFixture("Preview", "1.26.6022.0"))
		} else {
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	newClient := func() *Client {
		c := NewClient(server.Client(), func(context.Context) (string, error) { return "test-auth", nil })
		c.historyURLs = []string{server.URL + "/history"}
		c.packagesURL = server.URL + "/packages/"
		return c
	}
	cache := filepath.Join(t.TempDir(), "pending.json")
	c := newClient()
	discoveredAfter := time.Now().Unix()
	got := c.Fetch(context.Background(), false, cache)
	if len(got.ReleaseVersions) != 2 || len(got.PreviewVersions) != 1 {
		t.Fatalf("Store versions not added: %+v", got)
	}
	if !reflect.DeepEqual(got.ReleaseVersions[0], historical) || got.ReleaseVersions[1].Version != "Release 1.26.45.01" || got.ReleaseVersions[1].MD5 != "" {
		t.Fatalf("historical data changed or package conversion incorrect: %+v", got.ReleaseVersions)
	}
	if len(got.ReleaseVersions[1].URLs) != 2 || !strings.Contains(got.ReleaseVersions[1].URLs[0], "/prefix/content/") {
		t.Fatal("CDN roots were not deduplicated or lost their prefix")
	}
	firstSeen := got.ReleaseVersions[1].Timestamp
	if firstSeen < discoveredAfter || firstSeen > time.Now().Unix() {
		t.Fatal("Store discovery time must match historical catalog timestamp semantics")
	}
	storeFailed = true
	c = newClient() // Simulate restarting the launcher, with only the disk cache.
	got = c.Fetch(context.Background(), false, cache)
	if len(got.ReleaseVersions) != 2 || got.ReleaseVersions[1].Timestamp != firstSeen || len(got.PreviewVersions) != 1 {
		t.Fatalf("pending versions lost after restart/Store failure: %+v", got)
	}
	storeFailed = false
	release = "1.26.4602.0"
	got = c.Fetch(context.Background(), false, cache)
	if len(got.ReleaseVersions) != 3 {
		t.Fatalf("new Store release displaced a still-pending version: %+v", got)
	}
	// JSON catches up with the older pending release. Numeric spelling differs.
	completed := Version{Version: "Release 1.26.45.1", URLs: []string{"https://history.example/new.msixvc"}, Timestamp: 2, MD5: "verified-checksum"}
	history.ReleaseVersions = append(history.ReleaseVersions, completed)
	got = c.Fetch(context.Background(), false, cache)
	if len(got.ReleaseVersions) != 3 || !reflect.DeepEqual(got.ReleaseVersions[1], completed) {
		t.Fatalf("JSON did not replace the matching pending record: %+v", got.ReleaseVersions)
	}
	pending := readPending(cache)
	if len(pending.ReleaseVersions) != 1 || pending.ReleaseVersions[0].Version != "Release 1.26.46.02" {
		t.Fatalf("matched release not pruned: %+v", pending)
	}
	history.ReleaseVersions = append(history.ReleaseVersions, got.ReleaseVersions[2])
	history.PreviewVersions = got.PreviewVersions
	got = c.Fetch(context.Background(), false, cache)
	if pending = readPending(cache); len(pending.ReleaseVersions)+len(pending.PreviewVersions) != 0 {
		t.Fatalf("pending catalog not cleared after JSON caught up: %+v", pending)
	}
	if len(got.ReleaseVersions) != 3 || len(got.PreviewVersions) != 1 {
		t.Fatal("duplicate versions after JSON caught up")
	}
	// Store remains useful even when both historical URLs would be unavailable.
	historyFailed = true
	got = c.Fetch(context.Background(), false, cache)
	if len(got.ReleaseVersions) != 1 || len(got.PreviewVersions) != 1 || got.Source != packagesURL {
		t.Fatalf("history failure hid Store results: %+v", got)
	}
	contents, err := os.ReadFile(cache)
	if err != nil || strings.Contains(string(contents), "test-auth") {
		t.Fatalf("cache contains credentials or could not be read: %v", err)
	}
}

func TestSourceFallbacksAndTimeout(t *testing.T) {
	for _, scenario := range []string{"signed out", "auth timeout", "preview failed", "package timeout", "mirror fallback"} {
		t.Run(scenario, func(t *testing.T) {
			var paths []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/history") {
					paths = append(paths, r.URL.Path)
					if scenario == "mirror fallback" && r.URL.Path == "/history-cn" {
						_, _ = w.Write([]byte(`{"unexpected":true}`))
						return
					}
					_, _ = w.Write([]byte(`{"file_version":1,"releaseVersions":[{"version":"Release 1.26.40.01","urls":["https://history.example/old.msixvc"]}],"previewVersions":[]}`))
					return
				}
				if scenario == "signed out" || scenario == "auth timeout" {
					t.Error("unauthenticated request reached Store")
				}
				if scenario == "package timeout" {
					<-r.Context().Done()
					return
				}
				if r.URL.Path == "/packages/"+previewID {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				_ = json.NewEncoder(w).Encode(packageFixture("Release", "1.26.4501.0"))
			}))
			defer server.Close()
			c := NewClient(server.Client(), func(ctx context.Context) (string, error) {
				if scenario == "signed out" {
					return "", errors.New("interaction required")
				}
				if scenario == "auth timeout" {
					<-ctx.Done()
					return "", ctx.Err()
				}
				return "test-auth", nil
			})
			c.storeTimeout = 100 * time.Millisecond
			c.historyURLs = []string{server.URL + "/history", server.URL + "/history-cn"}
			c.packagesURL = server.URL + "/packages/"
			started := time.Now()
			got := c.Fetch(context.Background(), scenario == "mirror fallback", filepath.Join(t.TempDir(), "pending.json"))
			if time.Since(started) > 2*time.Second {
				t.Fatal("Store timeout blocked historical results")
			}
			want := 1
			if scenario == "preview failed" || scenario == "mirror fallback" {
				want = 2
			}
			if len(got.ReleaseVersions) != want || len(got.PreviewVersions) != 0 {
				t.Fatalf("source failure contaminated working source: %+v", got)
			}
			if scenario == "mirror fallback" && !reflect.DeepEqual(paths, []string{"/history-cn", "/history-cn", "/history"}) {
				t.Fatalf("mirror preference/retries lost: %v", paths)
			}
		})
	}
}

func TestPackageValidation(t *testing.T) {
	for _, scenario := range []string{"not found", "wrong channel", "wrong arch", "not msixvc", "bad version", "no CDN", "unsafe CDN", "absolute relative URL", "mismatched filename"} {
		t.Run(scenario, func(t *testing.T) {
			fixture := packageFixture("Release", "1.26.4501.0")
			file := fixture["PackageFiles"].([]map[string]any)[0]
			switch scenario {
			case "not found":
				fixture["PackageFound"] = false
			case "wrong channel":
				file["FileName"] = strings.ReplaceAll(file["FileName"].(string), "MinecraftUWP", "MinecraftWindowsBeta")
			case "wrong arch":
				file["FileName"] = strings.ReplaceAll(file["FileName"].(string), "x64", "arm64")
			case "not msixvc":
				file["FileName"] = "dependency.appx"
			case "bad version":
				file["FileName"] = strings.ReplaceAll(file["FileName"].(string), "1.26.4501.0", "1.26.invalid.0")
			case "no CDN":
				file["CdnRootPaths"] = []string{}
			case "unsafe CDN":
				file["CdnRootPaths"] = []string{"file:///C:/", "javascript:alert(1)", "https://user:password@example.com/"}
			case "absolute relative URL":
				file["RelativeUrl"] = "https://other.example/" + file["FileName"].(string)
			case "mismatched filename":
				file["RelativeUrl"] = "content/other.msixvc"
			}
			raw, _ := json.Marshal(fixture)
			var response packageResponse
			if err := json.Unmarshal(raw, &response); err != nil {
				t.Fatal(err)
			}
			if _, err := response.version("Release"); err == nil {
				t.Fatal("invalid package accepted")
			}
		})
	}
}

func TestMergeKeepsChannelsDistinctAndTimestampStable(t *testing.T) {
	pending := []Version{{Version: "Release 1.26.0.01", URLs: []string{"https://old.example/package.msixvc"}, Timestamp: 10}}
	latest := []Version{{Version: "Release 1.26.0.1", URLs: []string{"https://fresh.example/package.msixvc"}, Timestamp: 20}, {Version: "Preview 1.26.0.01"}}
	merged, remaining := mergeVersions(nil, pending, latest)
	if len(merged) != 2 || len(remaining) != 2 || merged[0].Timestamp != 10 || merged[0].URLs[0] != latest[0].URLs[0] {
		t.Fatalf("incorrect merge: %+v, %+v", merged, remaining)
	}
}
