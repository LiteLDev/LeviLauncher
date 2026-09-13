package mcservice

import (
	"errors"
	"os"
	"path"
	"path/filepath"

	"github.com/liteldev/LeviLauncher/internal/utils"
)

func collectUWPSafeGameDataEntries(mojangRoot string) ([]instanceBackupSourceEntry, int64, error) {
	var entries []instanceBackupSourceEntry
	var total int64
	for _, rootName := range instanceBackupSafeBedrockRoots {
		source := filepath.Join(mojangRoot, rootName)
		info, err := os.Stat(source)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, 0, wrapInstanceBackupReadError(err)
		}
		if info.IsDir() {
			hasPayload, err := instanceBackupDirContainsPayload(source)
			if err != nil {
				return nil, 0, err
			}
			if !hasPayload {
				continue
			}
			total += utils.DirSize(source)
		} else {
			total += info.Size()
		}
		entries = append(entries, instanceBackupSourceEntry{
			ArchivePath: path.Join("gameData", "games", "com.mojang", rootName),
			SourcePath:  source, IsDir: info.IsDir(),
		})
	}
	return entries, total, nil
}
