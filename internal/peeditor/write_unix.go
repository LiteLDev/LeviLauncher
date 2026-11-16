//go:build darwin || linux

package peeditor

import (
	"github.com/liteldev/LeviLauncher/internal/fsx"
	"os"
)

func writePatchedExecutable(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if _, err = os.Stat(path + ".orig"); os.IsNotExist(err) {
		if err = fsx.WriteFileAtomic(path+".orig", original, info.Mode().Perm()); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return fsx.WriteFileAtomic(path, data, info.Mode().Perm())
}
