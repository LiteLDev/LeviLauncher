package mcservice

import (
    "os"
    "path/filepath"
    "strings"

    "github.com/liteldev/LeviLauncher/internal/types"
    "github.com/liteldev/LeviLauncher/internal/utils"
)

// ListDrives is implemented per-OS

func ListDir(path string) []types.FileEntry {
	list := []types.FileEntry{}
	if path == "" {
		return list
	}
	ents, err := os.ReadDir(path)
	if err != nil {
		return list
	}
	for _, e := range ents {
		p := filepath.Join(path, e.Name())
		list = append(list, types.FileEntry{
			Name:  e.Name(),
			Path:  p,
			IsDir: e.IsDir(),
			Size:  0,
		})
	}
	return list
}

func GetPathSize(path string) int64 {
	p := strings.TrimSpace(path)
	if p == "" {
		return 0
	}
	fi, err := os.Stat(p)
	if err != nil {
		return 0
	}
	if fi.IsDir() {
		return utils.DirSize(p)
	}
	return fi.Size()
}

func GetPathModTime(path string) int64 {
	p := strings.TrimSpace(path)
	if p == "" {
		return 0
	}
	fi, err := os.Stat(p)
	if err != nil {
		return 0
	}
	return fi.ModTime().Unix()
}

func CreateFolder(parent string, name string) string {
	p := strings.TrimSpace(parent)
	n := strings.TrimSpace(name)
	if p == "" || n == "" {
		return "ERR_NAME_REQUIRED"
	}
	fi, err := os.Stat(p)
	if err != nil || !fi.IsDir() {
		return "ERR_NOT_FOUND_OLD"
	}
	safe := utils.SanitizeFilename(n)
	if strings.TrimSpace(safe) == "" {
		safe = "new_folder"
	}
	full := filepath.Join(p, safe)
	if utils.DirExists(full) {
		return "ERR_NAME_EXISTS"
	}
	if err := utils.CreateDir(full); err != nil {
		return "ERR_CREATE_TARGET_DIR"
	}
	return ""
}

// GetDriveStats is implemented per-OS
