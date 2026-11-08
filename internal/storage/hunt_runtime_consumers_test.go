package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// huntRCOldLauncherRuns simulates a pre-PR launcher started again after a
// migration: config.json renamed away, so it writes a fresh one, and it
// creates an instance in its default data root.
func huntRCOldLauncherRuns(t *testing.T, e env, instance, marker string) {
	t.Helper()
	write(t, filepath.Join(e.legacy, "config.json"), `{}`)
	write(t, filepath.Join(e.legacy, "versions", instance, "version.json"), `{"name":"`+instance+`","marker":"`+marker+`"}`)
	write(t, filepath.Join(e.legacy, "versions", instance, "Minecraft.Windows.exe"), "exe")
}

func mustMigrate(t *testing.T, step string) {
	t.Helper()
	resetRecordCache()
	if outcome, err := Prepare(); err != nil || outcome != Migrated {
		t.Fatalf("%s: Prepare = %v, %v", step, outcome, err)
	}
	resetRecordCache()
}

// Upgrade, downgrade (old launcher recreates X), upgrade (X -> X-legacy),
// the user deletes the current X, downgrade (old launcher recreates X and
// registers it), upgrade (X -> Instances\X, no conflict). The registration
// Windows holds is the latest one, <legacy>\versions\X, whose content is now
// Instances\X. TranslateLegacyPath must map it there, not to the older
// instance X-legacy: the stale, more specific move of the second migration
// must not outrank the third migration.
func TestHunt_runtime_consumers_StaleSpecificMoveOutranksLaterMigration(t *testing.T) {
	e := setup(t)
	huntRCOldLauncherRuns(t, e, "X", "v1")
	mustMigrate(t, "first upgrade")

	huntRCOldLauncherRuns(t, e, "X", "v2")
	mustMigrate(t, "second upgrade")
	local := filepath.Join(e.local, "LeviLauncher")
	if !strings.Contains(read(t, filepath.Join(local, "Instances", "X-legacy", "version.json")), "v2") {
		t.Fatal("setup: the second X was not kept as X-legacy")
	}

	// The user deletes the current X in the new launcher.
	if err := os.RemoveAll(filepath.Join(local, "Instances", "X")); err != nil {
		t.Fatal(err)
	}
	huntRCOldLauncherRuns(t, e, "X", "v3")
	mustMigrate(t, "third upgrade")
	if !strings.Contains(read(t, filepath.Join(local, "Instances", "X", "version.json")), "v3") {
		t.Fatal("setup: the third X did not move to Instances\\X")
	}

	registered := filepath.Join(e.legacy, "versions", "X")
	got := TranslateLegacyPath(registered)
	want := filepath.Join(local, "Instances", "X")
	if !samePath(got, want) {
		meta, _ := os.ReadFile(filepath.Join(got, "version.json"))
		t.Errorf("TranslateLegacyPath(%s) = %s (holds %s), want %s: the latest registration of X is mapped to an older instance", registered, got, meta, want)
	}
}

// After a first migration, an older launcher runs again, recreates
// instance Y and registers it, and the game runs from it: the next start
// cannot move <legacy>\versions\Y and defers. That path was never moved, it
// still exists, and it holds a different game than Instances\Y.
// TranslateLegacyPath / IsMovedLegacyPath must not report it as moved, or
// ReconcileRegisteredFlags marks Instances\Y registered and launching
// Instances\Y unregisters the running package (reregisterMovedGDK /
// uwp.LaunchWithPreparation).
func TestHunt_runtime_consumers_UnmovedPendingPathReportedMoved(t *testing.T) {
	e := setup(t)
	huntRCOldLauncherRuns(t, e, "Y", "v1")
	mustMigrate(t, "first upgrade")

	huntRCOldLauncherRuns(t, e, "Y", "v2")
	// The game runs from the recreated instance.
	f, err := os.Open(filepath.Join(e.legacy, "versions", "Y", "Minecraft.Windows.exe"))
	if err != nil {
		t.Fatal(err)
	}
	resetRecordCache()
	outcome, perr := Prepare()
	f.Close()
	if outcome != Deferred {
		t.Fatalf("Prepare = %v, %v; want Deferred", outcome, perr)
	}
	resetRecordCache()
	pending := filepath.Join(e.legacy, "versions", "Y")
	if !strings.Contains(read(t, filepath.Join(pending, "version.json")), "v2") {
		t.Fatal("setup: the pending instance is not in place")
	}
	if IsMovedLegacyPath(pending) {
		t.Errorf("IsMovedLegacyPath(%s) = true, TranslateLegacyPath = %s; the path was never moved and still exists", pending, TranslateLegacyPath(pending))
	}
}
