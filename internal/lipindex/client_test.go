package lipindex

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func catalogueServer(t *testing.T, delay time.Duration) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	docs := map[string]string{
		"/github.com/MarmieQi/LHolo/index.json":              fixtureHolo,
		"/github.com/LiteLDev/LegacyMoney/index.json":        fixtureMoney,
		"/github.com/LiteLDev/LegacyScriptEngine/index.json": fixtureLSE,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if delay > 0 {
			time.Sleep(delay)
		}
		if r.URL.Path == "/index.json" {
			w.Write([]byte(`{"mods":[
				{"id":"github.com/MarmieQi/LHolo","description":"Schematic mod, as the index cuts it…","side":["client"],"platform":["win-x64"]},
				{"id":"github.com/LiteLDev/LegacyMoney","side":["server","client"],"platform":["win-x64"]},
				{"id":"github.com/ShrBox/BackupHelper","side":["server"],"platform":["win-x64"]},
				{"id":"github.com/Some/AndroidOnly","side":["client"],"platform":["android-arm64"]},
				{"id":"github.com/LiteLDev/LegacyScriptEngine","side":["server","client"],"platform":["win-x64"]},
				{"id":"github.com/Broken/Detail","side":["client"],"platform":["win-x64"]}]}`))
			return
		}
		if d, ok := docs[r.URL.Path]; ok {
			w.Write([]byte(d))
			return
		}
		if strings.Contains(r.URL.Path, "Broken") {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestFetchDetails(t *testing.T) {
	srv, hits := catalogueServer(t, 0)
	c := &Client{BaseURL: srv.URL + "/", HTTP: srv.Client(), Workers: 2, Timeout: 5 * time.Second}
	details, err := c.FetchDetails(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(details) != 3 {
		t.Fatalf("details = %d, want 3 (server-only and Android-only skipped, broken ignored)", len(details))
	}
	if n := hits.Load(); n != 5 {
		t.Fatalf("requests = %d, want index + 4 details", n)
	}
	var order []string
	for _, d := range details {
		order = append(order, d.ID)
	}
	if want := []string{"github.com/MarmieQi/LHolo", "github.com/LiteLDev/LegacyMoney", "github.com/LiteLDev/LegacyScriptEngine"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("details in %v, want the index's order", order)
	}
	idx := Build(details)
	holo, ok := idx.Find("MarmieQi/LHolo")
	if !ok {
		t.Fatal("LHolo missing")
	}
	// The search reads the index's description (what the site and the
	// window search); the detail's whole one is kept for `mod show`.
	if holo.Description != "Schematic mod, as the index cuts it…" || holo.FullDescription != "Schematic mod" {
		t.Fatalf("LHolo description = %q / %q", holo.Description, holo.FullDescription)
	}
	if money, _ := idx.Find("LiteLDev/LegacyMoney"); money == nil || money.Description != "Economy for servers and clients" {
		t.Fatalf("without an index description, the detail's is used: %+v", money)
	}
	if _, err := c.FetchDetail(context.Background(), "github.com/no/such"); !errors.Is(err, ErrPackageNotFound) {
		t.Fatalf("FetchDetail(missing) = %v", err)
	}
	if _, err := c.FetchDetail(context.Background(), "github.com/Broken/Detail"); !errors.Is(err, ErrIndexUnavailable) {
		t.Fatalf("FetchDetail(broken) = %v", err)
	}
}

func TestFetchDetailsTimeout(t *testing.T) {
	srv, _ := catalogueServer(t, 300*time.Millisecond)
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client(), Workers: 1, Timeout: 500 * time.Millisecond}
	if _, err := c.FetchDetails(context.Background(), nil); !errors.Is(err, ErrIndexTimeout) {
		t.Fatalf("err = %v, want ErrIndexTimeout", err)
	}
	if !strings.HasPrefix(ErrIndexTimeout.Error(), "ERR_LIP_INDEX_TIMEOUT") {
		t.Fatal("the timeout error must carry its code")
	}
}

func TestSourceCaches(t *testing.T) {
	srv, hits := catalogueServer(t, 0)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	src := &Source{
		Client: &Client{BaseURL: srv.URL, HTTP: srv.Client(), Timeout: 5 * time.Second},
		Cache:  &Cache{Path: filepath.Join(t.TempDir(), "cache", "lip.json")},
		MaxAge: 30 * time.Minute,
		Now:    func() time.Time { return now },
	}
	idx, at, stale, err := src.Load(context.Background(), false)
	if err != nil || stale || !at.Equal(now) || len(idx.Packages) != 3 {
		t.Fatalf("first load = %d packages, %v, %v, %v", len(idx.Packages), at, stale, err)
	}
	first := hits.Load()
	now = now.Add(10 * time.Minute)
	if _, _, _, err := src.Load(context.Background(), false); err != nil || hits.Load() != first {
		t.Fatalf("a fresh cache must not fetch (hits %d -> %d, %v)", first, hits.Load(), err)
	}
	if _, _, _, err := src.Load(context.Background(), true); err != nil || hits.Load() == first {
		t.Fatal("refresh must fetch")
	}
	// Offline: the stale copy is used and reported.
	srv.Close()
	now = now.Add(time.Hour)
	idx, _, stale, err = src.Load(context.Background(), false)
	if idx == nil || !stale || err == nil {
		t.Fatalf("offline load = %v, stale %v, err %v", idx, stale, err)
	}
	if d, _, _ := src.Cache.Load(); len(d) != 3 || d[0].Readmes != nil {
		t.Fatal("the cache keeps the details without READMEs")
	}
}
