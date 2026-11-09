package lipindex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/liteldev/LeviLauncher/internal/fsx"
	"github.com/liteldev/LeviLauncher/internal/httpx"
)

// Catalogue errors. Their text starts with the launcher's error code.
var (
	// ErrIndexTimeout: the whole catalogue did not load within the timeout.
	ErrIndexTimeout = errors.New("ERR_LIP_INDEX_TIMEOUT: the mod catalogue did not respond in time")
	// ErrIndexUnavailable: the catalogue could not be fetched.
	ErrIndexUnavailable = errors.New("ERR_LIP_INDEX_UNAVAILABLE")
	// ErrPackageNotFound: the catalogue has no such package.
	ErrPackageNotFound = errors.New("ERR_LIP_PACKAGE_NOT_FOUND")
)

// DefaultBaseURL is the JSON of the catalogue of LeviLamina mods: version 4
// (lip v0.40's keys). levimc.org/mods/ keeps the JSON from before version 4
// for older launchers.
const DefaultBaseURL = "https://levimc.org/api/v4/mods/"

const (
	defaultWorkers = 8
	// DefaultTimeout bounds fetching the whole catalogue, as in the window.
	DefaultTimeout = 30 * time.Second
	requestTimeout = 20 * time.Second
)

// Client fetches the catalogue.
type Client struct {
	BaseURL string
	HTTP    *http.Client
	Workers int
	Timeout time.Duration
}

// NewClient returns a client for the public catalogue using the launcher's
// HTTP settings (system proxy, user agent).
func NewClient() *Client {
	return &Client{BaseURL: DefaultBaseURL, HTTP: httpx.NewClient(requestTimeout), Workers: defaultWorkers, Timeout: DefaultTimeout}
}

func (c *Client) base() string {
	if strings.TrimSpace(c.BaseURL) == "" {
		return DefaultBaseURL
	}
	return c.BaseURL
}

func (c *Client) getJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	httpx.ApplyDefaultHeaders(req)
	req.Header.Set("Cache-Control", "no-cache")
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		_, _ = io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("%w: %s", ErrPackageNotFound, url)
	}
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("fetch %s: status %d", url, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

type indexDoc struct {
	Mods []struct {
		ID          string   `json:"id"`
		Description string   `json:"description"`
		Side        []string `json:"side"`
		Platform    []string `json:"platform"`
	} `json:"mods"`
}

// descriptions maps each id of the index to the description it lists
// (cut to 200 characters: what the site and the window search).
func (doc *indexDoc) descriptions() map[string]string {
	out := map[string]string{}
	for _, m := range doc.Mods {
		id := strings.TrimSpace(m.ID)
		if _, seen := out[id]; id != "" && !seen {
			out[id] = m.Description
		}
	}
	return out
}

// installableIdentifiers lists, in the index's order, the catalogue entries
// whose latest version has an entry for side and platform (the launcher
// installs for the client on its own platform only).
func installableIdentifiers(doc *indexDoc, side, platform string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, m := range doc.Mods {
		id := strings.TrimSpace(m.ID)
		if id == "" || seen[id] {
			continue
		}
		hasSide, hasPlatform := false, false
		for _, s := range m.Side {
			hasSide = hasSide || strings.EqualFold(strings.TrimSpace(s), side)
		}
		for _, p := range m.Platform {
			hasPlatform = hasPlatform || strings.EqualFold(strings.TrimSpace(p), platform)
		}
		if hasSide && hasPlatform {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// FetchDetails fetches the catalogue: its index, then the detail of every
// package the launcher can install (an entry for TargetSide and
// TargetPlatform in its latest version) with a pool of workers. Each detail
// gets the index's description (CatalogDescription). The details
// come in the index's order. Details that fail are skipped (reused from
// previous when given); the fetch fails only when every detail failed. It
// times out with ErrIndexTimeout.
func (c *Client) FetchDetails(ctx context.Context, previous map[string]*Detail) ([]*Detail, error) {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx2, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	wrap := func(err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(ctx2.Err(), context.DeadlineExceeded) {
			return ErrIndexTimeout
		}
		return fmt.Errorf("%w: %v", ErrIndexUnavailable, err)
	}
	var doc indexDoc
	if err := c.getJSON(ctx2, strings.TrimRight(c.base(), "/")+"/index.json", &doc); err != nil {
		return nil, wrap(err)
	}
	ids := installableIdentifiers(&doc, TargetSide, TargetPlatform)
	descriptions := doc.descriptions()
	workers := c.Workers
	if workers <= 0 {
		workers = defaultWorkers
	}
	workers = min(workers, len(ids))
	var (
		mu      sync.Mutex
		next    int
		slots   = make([]*Detail, len(ids))
		fetched int
		wg      sync.WaitGroup
	)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				mu.Lock()
				if next >= len(ids) || ctx2.Err() != nil {
					mu.Unlock()
					return
				}
				i, id := next, ids[next]
				next++
				mu.Unlock()
				d, err := c.fetchDetail(ctx2, id)
				mu.Lock()
				switch {
				case err == nil:
					d.CatalogDescription = descriptions[id]
					slots[i] = d
					fetched++
				case previous[id] != nil:
					cp := *previous[id]
					cp.CatalogDescription = descriptions[id]
					slots[i] = &cp
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if ctx2.Err() != nil {
		return nil, wrap(ctx2.Err())
	}
	if len(ids) > 0 && fetched == 0 {
		return nil, wrap(errors.New("failed to fetch the package details"))
	}
	out := make([]*Detail, 0, len(slots))
	for _, d := range slots {
		if d != nil {
			out = append(out, d)
		}
	}
	return out, nil
}

func (c *Client) fetchDetail(ctx context.Context, identifier string) (*Detail, error) {
	u := DetailURL(c.base(), identifier)
	if u == "" {
		return nil, ErrPackageNotFound
	}
	var d Detail
	if err := c.getJSON(ctx, u, &d); err != nil {
		return nil, err
	}
	// A document without "id" is not a detail of this shape (the catalogue
	// before lip v0.40 had "tooth").
	if strings.TrimSpace(d.ID) == "" {
		return nil, fmt.Errorf("%w: %s has no package id", ErrPackageNotFound, u)
	}
	return &d, nil
}

// FetchDetail fetches one package's detail directly. A missing package is
// ErrPackageNotFound; other failures are ErrIndexUnavailable.
func (c *Client) FetchDetail(ctx context.Context, identifier string) (*Detail, error) {
	ctx2, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	d, err := c.fetchDetail(ctx2, identifier)
	switch {
	case err == nil:
		return d, nil
	case errors.Is(err, ErrPackageNotFound):
		return nil, err
	case ctx.Err() != nil:
		return nil, ctx.Err()
	}
	return nil, fmt.Errorf("%w: %v", ErrIndexUnavailable, err)
}

// Cache keeps the fetched catalogue on disk so that commands do not fetch
// every package each time (a full fetch takes many seconds). The details
// keep the index's order. A cache of another shape (from before lip v0.40)
// lives in another file (the CLI's lip-catalog.v4.json); one read anyway
// gives details without ids, which Build skips.
type Cache struct {
	Path string
}

type cacheDoc struct {
	FetchedAt time.Time `json:"fetchedAt"`
	Details   []*Detail `json:"details"`
}

// Load returns the cached details and when they were fetched.
func (c Cache) Load() ([]*Detail, time.Time, error) {
	b, err := os.ReadFile(c.Path)
	if err != nil {
		return nil, time.Time{}, err
	}
	var doc cacheDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, time.Time{}, err
	}
	return doc.Details, doc.FetchedAt, nil
}

// Save writes details (without READMEs) to the cache.
func (c Cache) Save(details []*Detail, at time.Time) error {
	slim := make([]*Detail, 0, len(details))
	for _, d := range details {
		if d == nil {
			continue
		}
		cp := *d
		cp.Readmes = nil
		slim = append(slim, &cp)
	}
	b, err := json.Marshal(cacheDoc{FetchedAt: at, Details: slim})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.Path), 0o755); err != nil {
		return err
	}
	return fsx.WriteFileAtomic(c.Path, b, 0o644)
}

// Source loads the catalogue from the cache while it is fresh, otherwise
// from the network (refreshing the cache).
type Source struct {
	Client *Client
	Cache  *Cache
	MaxAge time.Duration
	Now    func() time.Time
}

func (s *Source) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Load returns the catalogue. With refresh, or when the cache is older
// than MaxAge, it is fetched again; when that fails and a cached copy
// exists, the cached copy is returned with stale set.
func (s *Source) Load(ctx context.Context, refresh bool) (idx *Index, fetchedAt time.Time, stale bool, err error) {
	var cached []*Detail
	if s.Cache != nil {
		if d, at, e := s.Cache.Load(); e == nil {
			cached, fetchedAt = d, at
			if !refresh && s.MaxAge > 0 && s.now().Sub(at) < s.MaxAge && at.Before(s.now().Add(time.Minute)) {
				return Build(d), at, false, nil
			}
		}
	}
	prev := map[string]*Detail{}
	for _, d := range cached {
		if d != nil {
			prev[d.ID] = d
		}
	}
	client := s.Client
	if client == nil {
		client = NewClient()
	}
	details, ferr := client.FetchDetails(ctx, prev)
	if ferr != nil {
		if ctx.Err() == nil && len(cached) > 0 {
			return Build(cached), fetchedAt, true, ferr
		}
		return nil, time.Time{}, false, ferr
	}
	at := s.now()
	if s.Cache != nil {
		_ = s.Cache.Save(details, at)
	}
	return Build(details), at, false, nil
}
