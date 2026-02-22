package resourcerules

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/httpx"
)

const (
	releaseAPI = "https://api.github.com/repos/dreamguxiang/resource_pack_rules/releases/latest"
	binName    = "resource_pack_rules.bin"
)

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}

type releaseResponse struct {
	Assets []releaseAsset `json:"assets"`
	Digest string         `json:"digest"`
}

type Status struct {
	Path       string `json:"path"`
	Installed  bool   `json:"installed"`
	UpToDate   bool   `json:"upToDate"`
	LocalSHA   string `json:"localSHA256"`
	RemoteSHA  string `json:"remoteSHA256"`
	CanCompare bool   `json:"canCompare"`
	Error      string `json:"error"`
}

func binDir() string {
	return filepath.Join(apppath.AppData(), "levilauncher.exe", "bin")
}

func localBinPath() string {
	return filepath.Join(binDir(), binName)
}

func fileSHA256Hex(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func parseDigest(digest string) string {
	if strings.HasPrefix(digest, "sha256:") {
		return strings.TrimPrefix(digest, "sha256:")
	}
	return digest
}

func releaseAPIs() []string {
	return []string{
		releaseAPI,
		"https://cdn.gh-proxy.org/" + releaseAPI,
		"https://edgeone.gh-proxy.org/" + releaseAPI,
		"https://gh-proxy.org/" + releaseAPI,
		"https://hk.gh-proxy.org/" + releaseAPI,
		"https://ghproxy.vip/" + releaseAPI,
	}
}

func fetchRelease(ctx context.Context) (downloadURL, remoteSHA256 string, err error) {
	for _, api := range releaseAPIs() {
		req, err := http.NewRequestWithContext(ctx, "GET", api, nil)
		if err != nil {
			continue
		}
		httpx.ApplyDefaultHeaders(req)
		req.Header.Set("Accept", "application/vnd.github+json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			continue
		}

		var rel releaseResponse
		if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		for _, a := range rel.Assets {
			if a.Name == binName {
				digest := a.Digest
				if digest == "" {
					digest = rel.Digest
				}
				return a.BrowserDownloadURL, parseDigest(digest), nil
			}
		}
	}
	return "", "", fmt.Errorf("asset %s not found in release APIs", binName)
}

func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	httpx.ApplyDefaultHeaders(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return err
	}
	f.Close()

	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func EnsureLatest(ctx context.Context) {
	if err := EnsureLatestWithError(ctx); err != nil {
		log.Printf("resourcerules: %v", err)
	}
}

func EnsureLatestWithError(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	dlURL, remoteSHA, err := fetchRelease(ctx)
	if err != nil {
		return fmt.Errorf("fetch release: %w", err)
	}

	if remoteSHA != "" {
		localSHA, err := fileSHA256Hex(localBinPath())
		if err == nil && strings.EqualFold(localSHA, remoteSHA) {
			return nil
		}
	}

	dir := binDir()
	_ = os.MkdirAll(dir, 0o755)

	if err := downloadFile(ctx, dlURL, localBinPath()); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	log.Printf("resourcerules: updated %s", binName)
	return nil
}

func CheckStatus(ctx context.Context) Status {
	out := Status{
		Path: localBinPath(),
	}

	localSHA, err := fileSHA256Hex(localBinPath())
	if err == nil {
		out.Installed = true
		out.LocalSHA = strings.ToLower(localSHA)
	} else if !os.IsNotExist(err) {
		out.Error = fmt.Sprintf("local sha256: %v", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_, remoteSHA, err := fetchRelease(ctx)
	if err != nil {
		if out.Error == "" {
			out.Error = fmt.Sprintf("fetch release: %v", err)
		} else {
			out.Error += "; " + fmt.Sprintf("fetch release: %v", err)
		}
		return out
	}

	remoteSHA = strings.ToLower(strings.TrimSpace(remoteSHA))
	if remoteSHA == "" {
		if out.Error == "" {
			out.Error = "remote sha256 is empty"
		} else {
			out.Error += "; remote sha256 is empty"
		}
		return out
	}

	out.RemoteSHA = remoteSHA
	out.CanCompare = true
	if out.Installed {
		out.UpToDate = strings.EqualFold(out.LocalSHA, out.RemoteSHA)
	}
	return out
}
