//go:build !windows

package lockfile

import "os"

func replaceRename(tmp, dst string) error { return os.Rename(tmp, dst) }
