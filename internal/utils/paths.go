package utils

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/fsx"
)

func CanWriteDir(p string) bool {
	v := strings.TrimSpace(p)
	if v == "" {
		return false
	}
	if !filepath.IsAbs(v) {
		return false
	}

	isWritable := func(dir string) bool {
		f, err := fsx.CreateTemp(dir, "levilauncher-write-test")
		if err != nil {
			return false
		}
		tf := f.Name()
		_, werr := f.Write([]byte("ok"))
		cerr := f.Close()
		_ = os.Remove(tf)
		return werr == nil && cerr == nil
	}

	curr := v
	for {
		info, err := os.Stat(curr)
		if err == nil {
			if info.IsDir() {
				return isWritable(curr)
			}
			return false
		}
		if !os.IsNotExist(err) {
			return false
		}

		parent := filepath.Dir(curr)
		if parent == curr {
			return false
		}
		curr = parent
	}
}
