package uwpdownload

import (
	"context"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/httpx"
)

// This probe fetches metadata, a signed URL and four archive-header bytes. It
// never downloads the full game or installs it. Normal tests stay offline.
func TestLiveCatalogAndResolution(t *testing.T) {
	if os.Getenv("LEVILAUNCHER_UWP_NETWORK_TEST") != "1" {
		t.Skip("set LEVILAUNCHER_UWP_NETWORK_TEST=1 to query the catalog and Windows Update")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	entries, err := FetchCatalog(ctx)
	if err != nil || len(entries) == 0 {
		t.Fatalf("catalog: count=%d error=%v", len(entries), err)
	}
	for _, entry := range entries {
		if entry.Type != "release" {
			continue
		}
		resolved, err := ResolveURL(ctx, entry.UUID)
		if err != nil || resolved == "" {
			t.Fatalf("resolve %s: %v", entry.Version, err)
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, resolved, nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Range", "bytes=0-3")
		response, err := httpx.NewClient(20 * time.Second).Do(request)
		if err != nil {
			t.Fatalf("CDN range request: %v", err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
			t.Fatalf("CDN status %d", response.StatusCode)
		}
		var header [4]byte
		if _, err := io.ReadFull(response.Body, header[:]); err != nil || string(header[:]) != "PK\x03\x04" {
			t.Fatalf("archive header: %x %v", header, err)
		}
		t.Logf("catalog entries=%d; resolved release=%s; CDN returned ZIP header", len(entries), entry.Version)
		return
	}
	t.Fatal("catalog contains no release")
}
