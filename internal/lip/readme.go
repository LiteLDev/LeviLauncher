package lip

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/httpx"
)

const readmeFetchTimeout = 20 * time.Second

// FetchPackageReadme downloads the README of a GitHub project URL through
// the GitHub proxy. It returns "" for non-GitHub URLs.
func FetchPackageReadme(projectURL string) (string, error) {
	trimmedURL := strings.TrimSpace(projectURL)
	if trimmedURL == "" {
		return "", nil
	}

	parsed, err := url.Parse(trimmedURL)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(parsed.Hostname(), "github.com") && !strings.EqualFold(parsed.Hostname(), "www.github.com") {
		return "", nil
	}

	parts := strings.Split(parsed.Path, "/")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			segments = append(segments, trimmed)
		}
	}
	if len(segments) < 2 {
		return "", nil
	}

	owner := segments[0]
	repo := strings.TrimSuffix(segments[1], ".git")
	if owner == "" || repo == "" {
		return "", nil
	}

	candidates := []string{
		fmt.Sprintf("https://github.bibk.top/%s/%s/raw/refs/heads/main/README.md", owner, repo),
		fmt.Sprintf("https://github.bibk.top/%s/%s/raw/refs/heads/master/README.md", owner, repo),
		fmt.Sprintf("https://github.bibk.top/%s/%s/raw/refs/heads/main/readme.md", owner, repo),
		fmt.Sprintf("https://github.bibk.top/%s/%s/raw/refs/heads/master/readme.md", owner, repo),
	}

	ctx, cancel := context.WithTimeout(context.Background(), readmeFetchTimeout)
	defer cancel()

	var lastErr error
	for _, candidate := range candidates {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate, nil)
		if err != nil {
			lastErr = err
			continue
		}
		httpx.ApplyDefaultHeaders(req)

		resp, err := httpx.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		func() {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
				return
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				lastErr = err
				return
			}
			lastErr = nil
			trimmedBody := strings.TrimSpace(string(body))
			if trimmedBody != "" {
				trimmedURL = string(body)
			}
		}()
		if lastErr == nil {
			return trimmedURL, nil
		}
	}

	if lastErr != nil {
		return "", lastErr
	}
	return "", nil
}
