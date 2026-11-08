package uwp

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liteldev/LeviLauncher/internal/httpx"
	"github.com/liteldev/LeviLauncher/internal/uwpdownload"
)

// TestDeltaUpgradeLive installs a real base version from Windows Update and
// upgrades a copy of it with InstallDelta, then re-checks every block of the
// result against its block map. It downloads about 1 GB the first time:
//
//	LEVILAUNCHER_UWP_DELTA_E2E=<work dir> [BASE=1.21.113.1] [TARGET=1.21.114.1]
func TestDeltaUpgradeLive(t *testing.T) {
	work := os.Getenv("LEVILAUNCHER_UWP_DELTA_E2E")
	if work == "" {
		t.Skip("set LEVILAUNCHER_UWP_DELTA_E2E=<work dir> to run against Windows Update")
	}
	baseVersion := envOr("LEVILAUNCHER_UWP_DELTA_BASE", "1.21.113.1")
	targetVersion := envOr("LEVILAUNCHER_UWP_DELTA_TARGET", "1.21.114.1")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()
	catalog, err := uwpdownload.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	idOf := func(version string) string {
		for _, v := range catalog {
			if v.Version == version && v.Type == "release" {
				return v.UUID
			}
		}
		t.Fatalf("version %s not in catalog", version)
		return ""
	}

	baseDir := filepath.Join(work, "base-"+baseVersion)
	if _, err := os.Stat(baseDir); err != nil {
		pkg, err := uwpdownload.ResolvePackage(ctx, idOf(baseVersion))
		if err != nil {
			t.Fatal(err)
		}
		archive := filepath.Join(work, baseVersion+".appx")
		if pkg.Verify(archive) != nil {
			start := time.Now()
			if err := downloadFile(ctx, pkg.URL, archive); err != nil {
				t.Fatal(err)
			}
			if err := pkg.Verify(archive); err != nil {
				t.Fatal(err)
			}
			t.Logf("downloaded base %s in %s", baseVersion, time.Since(start))
		}
		if _, err := Install(ctx, archive, baseDir, Options{}); err != nil {
			t.Fatal(err)
		}
	}

	pkg, err := uwpdownload.ResolvePackage(ctx, idOf(targetVersion))
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(work, "delta-"+targetVersion)
	_ = os.RemoveAll(target)
	start := time.Now()
	var last DeltaStats
	phase := ""
	manifest, err := InstallDelta(ctx, pkg.URL, baseDir, target, DeltaOptions{
		Client: httpx.NewClient(2 * time.Minute),
		Stats: func(s DeltaStats) {
			last = s
			if s.Phase != phase {
				phase = s.Phase
				t.Logf("%6.1fs phase %s %+v", time.Since(start).Seconds(), s.Phase, s)
			}
		},
	})
	if err != nil {
		t.Fatalf("InstallDelta: %v", err)
	}
	t.Logf("delta %s -> %s in %s: version %s, reused %d of %d bytes, downloaded %d (planned %d)",
		baseVersion, targetVersion, time.Since(start), manifest.GameVersion(), last.Reused, last.Total, last.Downloaded, last.ToDownload)

	// Independent check of the published build.
	raw, err := os.ReadFile(filepath.Join(target, "AppxBlockMap.xml"))
	if err != nil {
		t.Fatal(err)
	}
	var bm blockMapXML
	if err := xml.Unmarshal(raw, &bm); err != nil {
		t.Fatal(err)
	}
	// Files keep their archive names, which MakeAppx percent-encodes, so map
	// them back to block map names the way InstallDelta does.
	onDisk := map[string]string{}
	err = filepath.WalkDir(target, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(target, p)
			onDisk[blockMapKey(filepath.ToSlash(rel))] = p
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, deltaBlockSize)
	checked := 0
	for _, f := range bm.Files {
		path, ok := onDisk[blockMapKey(f.Name)]
		if !ok {
			t.Fatalf("missing %s", f.Name)
		}
		fh, err := os.Open(path)
		if err != nil {
			t.Fatalf("missing %s: %v", f.Name, err)
		}
		info, _ := fh.Stat()
		if info.Size() != f.Size {
			t.Fatalf("%s size %d want %d", f.Name, info.Size(), f.Size)
		}
		for i, b := range f.Blocks {
			n, _ := fh.ReadAt(buf, int64(i)*deltaBlockSize)
			sum := sha256.Sum256(buf[:n])
			if base64.StdEncoding.EncodeToString(sum[:]) != b.Hash {
				if strings.EqualFold(f.Name, "Minecraft.Windows.exe") {
					continue // patched for the Xbox authentication key
				}
				t.Fatalf("%s block %d differs", f.Name, i)
			}
		}
		fh.Close()
		checked++
	}
	t.Logf("verified %d files block by block", checked)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := httpx.NewClient(time.Hour).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
