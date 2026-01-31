package utils

import (
	"os"
	"path/filepath"
	"strings"
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
		tf := filepath.Join(dir, ".ll_write_test.tmp")
		f, err := os.Create(tf)
		if err != nil {
			return false
		}
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
