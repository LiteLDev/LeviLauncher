package app

import (
	"os"
	"sync"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
)

func setupSponsorConfig(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { _, _ = config.Reload() })
	t.Setenv("APPDATA", t.TempDir())
	if _, err := config.Reload(); err != nil {
		t.Fatal(err)
	}
}

func TestSponsorPromptEveryHundredLauncherStarts(t *testing.T) {
	setupSponsorConfig(t)
	for count := uint64(1); count <= 301; count++ {
		// A new service represents a new launcher process.
		a := &Minecraft{}
		a.RecordLauncherStart()
		a.RecordLauncherStart()
		c, err := config.Reload()
		if err != nil || c.LauncherLaunchCount != count {
			t.Fatalf("persisted count = %d, want %d; error = %v", c.LauncherLaunchCount, count, err)
		}
		want := uint64(0)
		if count%100 == 0 {
			want = count
		}
		if got := a.TakeSponsorPrompt(); got != want {
			t.Fatalf("launch %d: prompt = %d, want %d", count, got, want)
		}
		if got := a.TakeSponsorPrompt(); got != 0 {
			t.Fatalf("launch %d: repeated prompt = %d", count, got)
		}
	}
}

func TestSponsorPromptConcurrentCallsOnlyCountAndConsumeOnce(t *testing.T) {
	setupSponsorConfig(t)
	if err := config.Update(func(c *config.AppConfig) { c.LauncherLaunchCount = 99 }); err != nil {
		t.Fatal(err)
	}
	a := &Minecraft{}
	var wg sync.WaitGroup
	results := make(chan uint64, 16)
	for i := 0; i < cap(results); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.RecordLauncherStart()
			results <- a.TakeSponsorPrompt()
		}()
	}
	wg.Wait()
	close(results)
	var total uint64
	for count := range results {
		total += count
	}
	c, err := config.Reload()
	if total != 100 || c.LauncherLaunchCount != 100 || err != nil {
		t.Fatalf("prompt total = %d, persisted count = %d; error = %v", total, c.LauncherLaunchCount, err)
	}
}

func TestSponsorPromptStorageFailureDoesNotShowReminder(t *testing.T) {
	setupSponsorConfig(t)
	if err := config.Update(func(c *config.AppConfig) { c.LauncherLaunchCount = 99 }); err != nil {
		t.Fatal(err)
	}
	// Replace the test config with a directory to make saving fail.
	p := apppath.ConfigPath()
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(p, 0o755); err != nil {
		t.Fatal(err)
	}
	a := &Minecraft{}
	a.RecordLauncherStart()
	if got := a.TakeSponsorPrompt(); got != 0 {
		t.Fatalf("prompt after failed save = %d", got)
	}
	c, err := config.Load()
	if err != nil || c.LauncherLaunchCount != 99 {
		t.Fatalf("failed save changed cached count: %+v, %v", c, err)
	}
}
