// Package gdkversions supplements the historical GDK catalog with Xbox package
// updates while preserving the catalog format consumed by the download page.
package gdkversions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	githubURL   = "https://raw.githubusercontent.com/LiteLDev/minecraft-windows-gdk-version-db/refs/heads/main/historical_versions.json"
	proxyURL    = "https://github.bibk.top/LiteLDev/minecraft-windows-gdk-version-db/raw/refs/heads/main/historical_versions.json"
	packagesURL = "https://packagespc.xboxlive.com/GetBasePackage/"
	releaseID   = "7792d9ce-355a-493c-afbd-768f4a77c3b0"
	previewID   = "98bd2335-9b01-4e4c-bd05-ccc01614078b"
)

type Version struct {
	Version   string   `json:"version"`
	URLs      []string `json:"urls"`
	Timestamp int64    `json:"timestamp,omitempty"`
	MD5       string   `json:"md5,omitempty"`
}

type Catalog struct {
	FileVersion     int       `json:"file_version"`
	ReleaseVersions []Version `json:"releaseVersions"`
	PreviewVersions []Version `json:"previewVersions"`
	Source          string    `json:"_source,omitempty"`
}

type Client struct {
	http         *http.Client
	authorize    func(context.Context) (string, error)
	historyURLs  []string
	packagesURL  string
	storeTimeout time.Duration
	cacheMu      sync.Mutex
}

func NewClient(client *http.Client, authorize func(context.Context) (string, error)) *Client {
	return &Client{
		http: client, authorize: authorize,
		historyURLs: []string{githubURL, proxyURL}, packagesURL: packagesURL,
		storeTimeout: 10 * time.Second,
	}
}

// Fetch queries both sources concurrently. Each source and each Store channel
// can fail independently. Pending Store versions survive refreshes and restarts
// until a successful historical catalog contains them; no credentials are saved.
func (c *Client) Fetch(ctx context.Context, preferCN bool, cachePath string) Catalog {
	live := make(chan Catalog, 1)
	go func() {
		storeCtx, cancel := context.WithTimeout(ctx, c.storeTimeout)
		defer cancel()
		live <- c.fetchLatest(storeCtx)
	}()
	history := c.fetchHistory(ctx, preferCN)
	latest := <-live

	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	pending := readPending(cachePath)
	history.ReleaseVersions, pending.ReleaseVersions = mergeVersions(history.ReleaseVersions, pending.ReleaseVersions, latest.ReleaseVersions)
	history.PreviewVersions, pending.PreviewVersions = mergeVersions(history.PreviewVersions, pending.PreviewVersions, latest.PreviewVersions)
	writePending(cachePath, pending)
	if history.Source == "" && (len(history.ReleaseVersions) > 0 || len(history.PreviewVersions) > 0) {
		history.Source = packagesURL
	}
	return history
}

func (c *Client) fetchHistory(ctx context.Context, preferCN bool) Catalog {
	urls := append([]string(nil), c.historyURLs...)
	if preferCN && len(urls) == 2 {
		urls[0], urls[1] = urls[1], urls[0]
	}
	var lastErr error
	for _, url := range urls {
		for attempt := 0; attempt < 2; attempt++ {
			requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			var catalog Catalog
			err := c.getJSON(requestCtx, url, "", &catalog)
			cancel()
			if err == nil && (catalog.ReleaseVersions != nil || catalog.PreviewVersions != nil) {
				catalog.Source = url
				return catalog
			}
			if err == nil {
				err = fmt.Errorf("empty historical catalog")
			}
			lastErr = err
			if ctx.Err() != nil {
				break
			}
		}
		if ctx.Err() != nil {
			break
		}
	}
	log.Printf("GDK historical catalog unavailable: %v", lastErr)
	return Catalog{FileVersion: 1}
}

func (c *Client) fetchLatest(ctx context.Context) Catalog {
	var catalog Catalog
	auth, err := c.authorize(ctx)
	if err != nil {
		log.Printf("GDK Store discovery unavailable: %v", err)
		return catalog
	}
	if auth == "" {
		return catalog
	}
	var wg sync.WaitGroup
	for _, channel := range []struct {
		name, id string
		versions *[]Version
	}{
		{"Release", releaseID, &catalog.ReleaseVersions},
		{"Preview", previewID, &catalog.PreviewVersions},
	} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var response packageResponse
			if err := c.getJSON(ctx, c.packagesURL+channel.id, auth, &response); err != nil {
				log.Printf("GDK Store %s discovery failed: %v", channel.name, err)
				return
			}
			version, err := response.version(channel.name)
			if err != nil {
				log.Printf("GDK Store %s package invalid: %v", channel.name, err)
				return
			}
			*channel.versions = []Version{version}
		}()
	}
	wg.Wait()
	return catalog
}

func (c *Client) getJSON(ctx context.Context, url, auth string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cache-Control", "no-cache")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s -> HTTP %d", req.URL.Host, resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out)
}
