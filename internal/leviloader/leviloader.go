package leviloader

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/peeditor"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	// LoaderDLLName is the injected loader dropped next to Minecraft.Windows.exe.
	LoaderDLLName = "LeviLauncher.dll"
	// LoaderEntryName is the anchor export referenced by the import descriptor
	// added to Minecraft.Windows.exe.
	LoaderEntryName = "LeviLauncherEntry"

	legacyProxyName  = "vcruntime140_1.dll"
	minecraftExeName = "Minecraft.Windows.exe"

	EventMigrateStart    = "leviloader.migrate.start"
	EventMigrateProgress = "leviloader.migrate.progress"
	EventMigrateDone     = "leviloader.migrate.done"
	EventMigrateError    = "leviloader.migrate.error"
)

//go:embed LeviLauncher.dll
var embeddedLoader []byte

type MigrateProgress struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

func bytesSHA256(b []byte) []byte { h := sha256.Sum256(b); return h[:] }

func fileSHA256(p string) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func emit(name string, data any) {
	if app := application.Get(); app != nil {
		app.Event.Emit(name, data)
	}
}

// EnsureForVersion writes LeviLauncher.dll into versionDir when it is missing
// or stale. Removal of the legacy vcruntime140_1.dll proxy is deferred to
// PatchAndActivate, which
// only clears it once the import-table loader is confirmed active.
func EnsureForVersion(ctx context.Context, versionDir string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dir := strings.TrimSpace(versionDir)
	if dir == "" {
		return fmt.Errorf("version directory is empty")
	}

	dest := filepath.Join(dir, LoaderDLLName)
	needWrite := true
	if fi, err := os.Stat(dest); err == nil && fi.Size() > 0 {
		if fh, err := fileSHA256(dest); err == nil && bytes.Equal(fh, bytesSHA256(embeddedLoader)) {
			needWrite = false
		}
	}
	if needWrite {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		tmp := dest + ".tmp"
		if err := os.WriteFile(tmp, embeddedLoader, 0644); err != nil {
			_ = os.Remove(tmp)
			return err
		}
		if err := os.Rename(tmp, dest); err != nil {
			_ = os.Remove(tmp)
			return err
		}
	}
	return nil
}

func removeLegacyProxy(dir string) error {
	legacy := filepath.Join(dir, legacyProxyName)
	if err := os.Remove(legacy); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// PatchAndActivate deploys LeviLauncher.dll and adds its import to the version's
// Minecraft.Windows.exe and, once that import is confirmed present, removes the
// legacy vcruntime140_1.dll proxy so only one loader initializes. When patching
// fails the proxy is left untouched as a fallback. It is idempotent.
func PatchAndActivate(ctx context.Context, versionDir string) (bool, error) {
	if err := EnsureForVersion(ctx, versionDir); err != nil {
		return false, fmt.Errorf("deploy %s: %w", LoaderDLLName, err)
	}
	exe := filepath.Join(versionDir, minecraftExeName)
	added, err := peeditor.EnsureImportedDLL(exe, LoaderDLLName, LoaderEntryName)
	if err != nil {
		return false, err
	}
	if err := removeLegacyProxy(versionDir); err != nil {
		return added, err
	}
	return added, nil
}

// NeedsMigration reports whether the one-time migration from the legacy proxy
// mechanism to the import-table loader has yet to run.
func NeedsMigration() bool {
	c, err := config.Load()
	if err != nil {
		return false
	}
	return !c.LoaderMigratedV1
}

func markMigrated() error {
	c, err := config.Load()
	if err != nil {
		return err
	}
	c.LoaderMigratedV1 = true
	return config.Save(c)
}

// RunMigration migrates every installed version to the import-table loader:
// installs LeviLauncher.dll, patches the game executable's import table, and
// removes the legacy vcruntime140_1.dll proxy. It marks the migration complete
// on success and is safe to call more than once.
func RunMigration(ctx context.Context) (migrated int, err error) {
	defer func() {
		if err != nil {
			emit(EventMigrateError, err.Error())
		}
	}()
	root, err := apppath.VersionsDir()
	if err != nil {
		return 0, err
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return 0, err
	}
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if fileExists(filepath.Join(dir, minecraftExeName)) {
			dirs = append(dirs, dir)
		}
	}

	total := len(dirs)
	emit(EventMigrateStart, struct{}{})
	emit(EventMigrateProgress, MigrateProgress{Done: 0, Total: total})

	for i, dir := range dirs {
		if _, err := PatchAndActivate(ctx, dir); err != nil {
			return migrated, fmt.Errorf("migrate %s: %w", filepath.Base(dir), err)
		}
		migrated++
		emit(EventMigrateProgress, MigrateProgress{Done: i + 1, Total: total})
	}

	if err := markMigrated(); err != nil {
		return migrated, err
	}
	emit(EventMigrateDone, migrated)
	return migrated, nil
}
