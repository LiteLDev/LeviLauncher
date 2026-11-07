package storage

import (
	"path/filepath"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
)

func TestPrepareSharedNextToWindowUsesLegacyLayout(t *testing.T) {
	e := setup(t)
	seedLegacy(t, e.legacy, e.legacy)
	write(t, filepath.Join(e.legacy, "config.json"), `{}`)

	outcome, err := PrepareShared(func() bool { return true })
	if outcome != Legacy || err == nil {
		t.Fatalf("PrepareShared = %v, %v; want Legacy and an error", outcome, err)
	}
	if apppath.LegacyMode() != e.legacy {
		t.Fatalf("legacy mode = %q, want %q", apppath.LegacyMode(), e.legacy)
	}
	if !exists(filepath.Join(e.legacy, "config.json")) || exists(apppath.MigrationJournalPath()) {
		t.Fatal("PrepareShared must not migrate while a window runs")
	}
}

func TestPrepareSharedWithoutLegacyDataNextToWindow(t *testing.T) {
	setup(t)
	if outcome, err := PrepareShared(func() bool { return true }); outcome != Current || err != nil {
		t.Fatalf("PrepareShared = %v, %v; want Current, nil", outcome, err)
	}
}

func TestPrepareSharedMigratesAlone(t *testing.T) {
	e := setup(t)
	seedLegacy(t, e.legacy, e.legacy)
	write(t, filepath.Join(e.legacy, "config.json"), `{}`)
	if outcome, err := PrepareShared(func() bool { return false }); outcome != Migrated || err != nil {
		t.Fatalf("PrepareShared = %v, %v; want Migrated, nil", outcome, err)
	}
}
