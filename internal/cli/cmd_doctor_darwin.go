//go:build darwin

package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/liteldev/LeviLauncher/internal/gameruntime"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/versionlaunch"
)

func darwinNeedsRosetta(ctx context.Context) bool {
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return gameruntime.NeedsRosetta(probeCtx)
}

func darwinCheckWine(ctx context.Context) (version string, err error) {
	p, err := gameruntime.Default()
	if errors.Is(err, gameruntime.ErrExecutableMissing) {
		if requirement := gameruntime.CheckManagedWineRequirements(ctx); requirement != nil {
			return "", requirement
		}
	}
	if err != nil {
		return "", err
	}
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	version, err = p.CheckVersion(probeCtx)
	if err == nil && version != "" {
		version += " · " + p.GraphicsBackend()
	}
	return version, err
}

func darwinInstallManagedWine(ctx context.Context, progress func(types.WineInstallProgress)) error {
	_, err := gameruntime.InstallManagedWine(ctx, true, progress, versionlaunch.PrepareWine)
	return err
}

func describeDarwinWineError(err error) string {
	switch {
	case errors.Is(err, gameruntime.ErrExecutableMissing):
		return "not installed or executable missing"
	case errors.Is(err, gameruntime.ErrPrefixUninitialized):
		return "Wine prefix is not initialized"
	case errors.Is(err, gameruntime.ErrUnsupportedArch):
		return "MetalSharp Wine requires Apple Silicon"
	case errors.Is(err, gameruntime.ErrUnsupportedOS):
		return "MetalSharp Wine requires macOS 15 or later"
	case errors.Is(err, gameruntime.ErrGraphicsBackendUnavailable), errors.Is(err, gameruntime.ErrD3DMetalUnavailable):
		return err.Error()
	default:
		return fmt.Sprintf("unavailable (%v)", err)
	}
}
