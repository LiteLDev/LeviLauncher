//go:build !darwin

package cli

import (
	"context"
	"errors"

	"github.com/liteldev/LeviLauncher/internal/types"
)

func darwinNeedsRosetta(context.Context) bool { return false }

func darwinCheckWine(context.Context) (string, error) {
	return "", errors.New("Wine runtime is only used on macOS")
}

func darwinInstallManagedWine(context.Context, func(types.WineInstallProgress)) error {
	return errors.New("managed Wine installation is only supported on macOS")
}

func describeDarwinWineError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
