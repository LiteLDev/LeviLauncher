//go:build darwin

package cli

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/liteldev/LeviLauncher/internal/gameruntime"
	"github.com/liteldev/LeviLauncher/internal/types"
)

func darwinNeedsRosetta(ctx context.Context) bool {
	if runtime.GOARCH != "arm64" {
		return false
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return gameruntime.NeedsRosetta(probeCtx)
}

func darwinCheckWine(ctx context.Context) (version string, err error) {
	p, err := gameruntime.Default()
	if err != nil {
		return "", err
	}
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return p.CheckVersion(probeCtx)
}

func darwinInstallManagedWine(ctx context.Context, progress func(types.WineInstallProgress)) error {
	_, err := gameruntime.InstallManagedWine(ctx, true, progress)
	return err
}

func describeDarwinWineError(err error) string {
	switch {
	case errors.Is(err, gameruntime.ErrExecutableMissing):
		return "not installed or executable missing"
	case errors.Is(err, gameruntime.ErrPrefixUninitialized):
		return "Wine prefix is not initialized"
	case errors.Is(err, gameruntime.ErrD3DMetalUnavailable):
		return "D3DMetal graphics backend unavailable"
	default:
		return fmt.Sprintf("unavailable (%v)", err)
	}
}
