package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/fsx"
	"github.com/liteldev/LeviLauncher/internal/gdkversions"
	"github.com/liteldev/LeviLauncher/internal/uwpdownload"
)

// Source says where Load reads the catalogue from.
type Source struct {
	// FetchGDK fetches the GDK history merged with the Store's newest builds
	// (gdkversions.Client.Fetch). It reports failure by an empty Source.
	FetchGDK func(ctx context.Context) gdkversions.Catalog
	// LoadUWP returns the UWP catalogue (embedded, never stale).
	LoadUWP func() ([]uwpdownload.Version, error)
	// CachePath is where the last good GDK history is kept; "" disables
	// the cache.
	CachePath string
	// Now is the clock; nil is time.Now.
	Now func() time.Time
}

// Loaded is a catalogue read by Load.
type Loaded struct {
	Builds []Build // sorted with Sort
	// GDKStale is true when the live GDK history could not be fetched and
	// the cached one (from CachedAt) was used.
	GDKStale bool
	CachedAt time.Time
	// GDKMissing is true when GDK builds were wanted but neither the live
	// history nor a cache was available (Store builds may still be listed).
	GDKMissing bool
}

// ErrUnavailable reports that no catalogue could be read.
var ErrUnavailable = errors.New("the Minecraft build catalogue is unreachable")

type cachedGDK struct {
	SavedAt time.Time           `json:"savedAt"`
	Catalog gdkversions.Catalog `json:"catalog"`
}

// liveHistory reports whether c came from the historical catalogue (and not
// only from Store discovery or nothing).
func liveHistory(c gdkversions.Catalog) bool {
	return strings.Contains(c.Source, "historical_versions")
}

// Load reads the catalogue of the wanted package types. A GDK history fetched
// live is saved to the cache; when the live fetch fails the cache is used
// and the newest Store builds found live are merged into it. Load fails with
// ErrUnavailable only when nothing at all could be read.
func Load(ctx context.Context, src Source, wantGDK, wantUWP bool) (Loaded, error) {
	now := time.Now
	if src.Now != nil {
		now = src.Now
	}
	var out Loaded
	if wantGDK && src.FetchGDK != nil {
		live := src.FetchGDK(ctx)
		if err := ctx.Err(); err != nil {
			return Loaded{}, err
		}
		catalog := live
		if liveHistory(live) {
			if src.CachePath != "" {
				if b, err := json.Marshal(cachedGDK{SavedAt: now(), Catalog: live}); err == nil {
					_ = fsx.WriteFileAtomic(src.CachePath, b, 0o644)
				}
			}
		} else if cached, ok := readCache(src.CachePath); ok {
			catalog = mergeGDK(cached.Catalog, live)
			out.GDKStale, out.CachedAt = true, cached.SavedAt
		} else {
			out.GDKMissing = true
		}
		out.Builds = append(out.Builds, FromGDK(catalog)...)
	}
	if wantUWP && src.LoadUWP != nil {
		list, err := src.LoadUWP()
		if err != nil {
			if len(out.Builds) == 0 {
				return Loaded{}, err
			}
		} else {
			out.Builds = append(out.Builds, FromUWP(list)...)
		}
	}
	if len(out.Builds) == 0 && (wantGDK || wantUWP) {
		return out, ErrUnavailable
	}
	Sort(out.Builds)
	return out, nil
}

func readCache(path string) (cachedGDK, bool) {
	if path == "" {
		return cachedGDK{}, false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return cachedGDK{}, false
	}
	var c cachedGDK
	if json.Unmarshal(b, &c) != nil || (len(c.Catalog.ReleaseVersions) == 0 && len(c.Catalog.PreviewVersions) == 0) {
		return cachedGDK{}, false
	}
	return c, true
}

// mergeGDK adds the live entries (Store discoveries) the cached history
// lacks.
func mergeGDK(cached, live gdkversions.Catalog) gdkversions.Catalog {
	merge := func(base, extra []gdkversions.Version) []gdkversions.Version {
		seen := map[string]bool{}
		for _, v := range base {
			seen[strings.ToLower(ShortVersion(v.Version))] = true
		}
		out := append([]gdkversions.Version(nil), base...)
		for _, v := range extra {
			if k := strings.ToLower(ShortVersion(v.Version)); !seen[k] {
				seen[k] = true
				out = append(out, v)
			}
		}
		return out
	}
	cached.ReleaseVersions = merge(cached.ReleaseVersions, live.ReleaseVersions)
	cached.PreviewVersions = merge(cached.PreviewVersions, live.PreviewVersions)
	return cached
}
