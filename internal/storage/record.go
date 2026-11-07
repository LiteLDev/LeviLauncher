package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/fsx"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

var (
	recordMu     sync.Mutex
	recordLoaded bool
	recordCache  *Record
)

// LoadRecord returns storage-migration.json, or nil when no migration
// happened on this machine.
func LoadRecord() *Record {
	recordMu.Lock()
	defer recordMu.Unlock()
	if !recordLoaded {
		recordCache = readRecord()
		recordLoaded = true
	}
	return recordCache
}

func readRecord() *Record {
	b, err := os.ReadFile(apppath.MigrationPath())
	if err != nil {
		return nil
	}
	var r Record
	if json.Unmarshal(b, &r) != nil || r.Format != recordFormat {
		return nil
	}
	return &r
}

// TranslateLegacyPath maps a path of the legacy layout, such as the install
// location Windows recorded for a registered instance, to where the
// migration moved it. Any other path is returned unchanged.
func TranslateLegacyPath(p string) string {
	r := LoadRecord()
	if r == nil || strings.TrimSpace(p) == "" {
		return p
	}
	norm := utils.NormalizeWindowsPath(p)
	for _, m := range r.Moves {
		from := utils.NormalizeWindowsPath(m.From)
		if norm == from {
			return m.To
		}
		if strings.HasPrefix(norm, from+string(filepath.Separator)) {
			// Keep the case of the remainder when p is spelled plainly.
			rest := norm[len(from)+1:]
			if cp := filepath.Clean(p); len(cp) > len(from) && strings.EqualFold(cp[:len(from)], from) {
				rest = cp[len(from)+1:]
			}
			return filepath.Join(m.To, rest)
		}
	}
	return p
}

// IsMovedLegacyPath reports whether p is a path of the legacy layout that
// the migration moved.
func IsMovedLegacyPath(p string) bool {
	return !samePath(TranslateLegacyPath(p), p)
}

// MarkShortcutsRepaired records that desktop shortcuts were repaired.
func MarkShortcutsRepaired() error {
	recordMu.Lock()
	defer recordMu.Unlock()
	r := readRecord()
	if r == nil {
		return nil
	}
	r.ShortcutsRepaired = true
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := fsx.WriteFileAtomic(apppath.MigrationPath(), data, 0o644); err != nil {
		return err
	}
	recordCache = r
	recordLoaded = true
	return nil
}

// importMaxAge is how long a file stays in Cache\Downloads and Cache\Imports
// before CleanImports removes it.
const importMaxAge = 24 * time.Hour

// CleanImports removes files in Cache\Downloads and Cache\Imports that an
// import left behind.
func CleanImports() {
	now := time.Now()
	for _, dir := range []string{apppath.DownloadsDir(), apppath.ImportsDir()} {
		ents, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range ents {
			info, err := e.Info()
			if err != nil || now.Sub(info.ModTime()) < importMaxAge {
				continue
			}
			_ = os.RemoveAll(filepath.Join(dir, e.Name()))
		}
	}
}
