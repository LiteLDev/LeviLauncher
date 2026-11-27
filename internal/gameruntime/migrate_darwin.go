package gameruntime

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/types"
)

func legacyManagedWineDir() string {
	return filepath.Join(apppath.LocalDir(), "Runtimes", "Sikarugir")
}

func legacyManagedWineApp() string {
	return filepath.Join(legacyManagedWineDir(), "Levi Wine.app")
}

func IsLegacyManagedWine(s types.WineSettings) bool {
	old := legacyManagedWineApp()
	return s.App == old && s.Prefix == filepath.Join(old, "Contents/SharedSupport/prefix")
}

func pathInside(root, target string) bool {
	if root == "" || target == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func GetLegacyWineInfo() (types.LegacyWineInfo, error) {
	dir := legacyManagedWineDir()
	info := types.LegacyWineInfo{Path: dir}
	stat, err := os.Stat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return info, nil
	}
	if err != nil {
		return info, err
	}
	if !stat.IsDir() {
		return info, nil
	}
	current, err := readSettings(settingsPath())
	if err == nil {
		current, err = resolveSettings(current)
	}
	if err != nil {
		return info, fmt.Errorf("ERR_WINE_LEGACY_SETTINGS: %w", err)
	}
	info.Active = pathInside(dir, current.App) ||
		pathInside(dir, current.Executable) ||
		pathInside(dir, current.Prefix)
	info.Present = true
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if fi, err := entry.Info(); err == nil {
			total += fi.Size()
		}
		return nil
	})
	info.SizeBytes = total
	return info, nil
}

func RemoveLegacyWineRuntime() error {
	info, err := GetLegacyWineInfo()
	if err != nil {
		return err
	}
	if !info.Present {
		return nil
	}
	if info.Active {
		return errors.New("ERR_WINE_LEGACY_IN_USE")
	}
	return os.RemoveAll(info.Path)
}

func migrateLegacyWineAccount(previous types.WineSettings, prefix string) error {
	if !IsLegacyManagedWine(previous) {
		return nil
	}
	old, err := os.OpenRoot(previous.Prefix)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer old.Close()
	users := filepath.Join("drive_c", "users")
	next, err := os.OpenRoot(prefix)
	if err != nil {
		return err
	}
	defer next.Close()
	entries, err := os.ReadDir(filepath.Join(prefix, users))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "Public" {
			continue
		}
		dir := filepath.Join(users, entry.Name(), "AppData", "Roaming", "xgr")
		name := filepath.Join(dir, "auth.json")
		cache, err := old.Open(name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		err = copyWineAccountCache(cache, next, dir, name)
		cache.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func copyWineAccountCache(source *os.File, target *os.Root, dir, name string) error {
	info, err := source.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return errors.New("invalid Wine account cache")
	}
	data, err := io.ReadAll(io.LimitReader(source, (1<<20)+1))
	if err != nil {
		return err
	}
	// The old game can refresh its cache while the runtime is downloading.
	if len(data) > 1<<20 || !json.Valid(data) {
		return nil
	}
	if err := target.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file, err := target.OpenFile(name, os.O_WRONLY|os.O_EXCL|os.O_CREATE, 0600)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		_ = target.Remove(name)
		return err
	}
	return nil
}
