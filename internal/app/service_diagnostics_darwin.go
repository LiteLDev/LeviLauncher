package app

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/liteldev/LeviLauncher/internal/diagnose"
	"github.com/liteldev/LeviLauncher/internal/gameinput"
	"github.com/liteldev/LeviLauncher/internal/gameruntime"
	"github.com/liteldev/LeviLauncher/internal/httpx"
	"github.com/liteldev/LeviLauncher/internal/mcservice"
	"github.com/liteldev/LeviLauncher/internal/vcruntime"
)

func configureDiagnostics(env *diagnose.Env) {
	env.RuntimeChecks = wineDiagnosticChecks()
	env.RunningExes = func() []string {
		var paths []string
		for _, process := range gameruntime.Processes() {
			paths = append(paths, process.Executable)
		}
		return paths
	}
	env.SkipCheck = func(id string) bool {
		switch id {
		case "vcruntime", "gameinput", "gaming_services", "xbox_services", "developer_mode", "webview2", "registration", "appdata_link", "elevation":
			return true
		default:
			return false
		}
	}
	env.FreeBytes = func(path string) (uint64, bool) {
		stats := mcservice.GetDriveStats(existingDiagnosticPath(path))
		return stats["free"], stats["total"] != 0
	}
	env.SamePath = func(a, b string) bool {
		if filepath.Clean(a) == filepath.Clean(b) {
			return a != "" && b != ""
		}
		left, leftErr := os.Stat(a)
		right, rightErr := os.Stat(b)
		return leftErr == nil && rightErr == nil && os.SameFile(left, right)
	}
	env.Proxy = func() (diagnose.ProxyInfo, error) {
		req, err := http.NewRequest(http.MethodGet, "https://github.com", nil)
		if err != nil {
			return diagnose.ProxyInfo{}, err
		}
		proxy, err := httpx.ProxyForRequest(req)
		if err != nil || proxy == nil {
			return diagnose.ProxyInfo{}, err
		}
		if httpx.UsesSystemProxy(req) {
			return diagnose.ProxyInfo{Manual: proxy.Host}, nil
		}
		return diagnose.ProxyInfo{Env: proxy.Host}, nil
	}
}

func wineDiagnosticChecks() []diagnose.RuntimeCheck {
	profile := sync.OnceValues(gameruntime.Default)
	var rosettaOnce sync.Once
	var rosettaMissing bool
	needsRosetta := func(ctx context.Context) bool {
		rosettaOnce.Do(func() {
			probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			rosettaMissing = gameruntime.NeedsRosetta(probeCtx)
		})
		return rosettaMissing
	}

	var checks []diagnose.RuntimeCheck
	if runtime.GOARCH == "arm64" {
		checks = append(checks, diagnose.RuntimeCheck{ID: "wine_rosetta", Run: func(ctx context.Context) diagnose.Check {
			if needsRosetta(ctx) {
				return diagnose.Check{Status: diagnose.StatusFail}
			}
			return diagnose.Check{Status: diagnose.StatusOK}
		}})
	}
	checks = append(checks, diagnose.RuntimeCheck{ID: "wine_runtime", Run: func(ctx context.Context) diagnose.Check {
		p, err := profile()
		if errors.Is(err, gameruntime.ErrExecutableMissing) {
			if requirement := gameruntime.CheckManagedWineRequirements(ctx); requirement != nil {
				err = requirement
			}
		}
		if err != nil {
			return diagnose.Check{Status: diagnose.StatusFail, Params: map[string]string{"reason": wineDiagnosticReason(err)}}
		}
		if needsRosetta(ctx) {
			return diagnose.Check{Status: diagnose.StatusSkip, Params: map[string]string{"reason": "rosetta"}}
		}
		version, err := p.CheckVersion(ctx)
		if err != nil || version == "" {
			return diagnose.Check{Status: diagnose.StatusFail, Params: map[string]string{"reason": "unavailable"}}
		}
		return diagnose.Check{Status: diagnose.StatusOK, Params: map[string]string{"version": version}}
	}})

	withProfile := func(id string, run func(gameruntime.Profile) diagnose.Check) diagnose.RuntimeCheck {
		return diagnose.RuntimeCheck{ID: id, Run: func(context.Context) diagnose.Check {
			p, err := profile()
			if err != nil {
				return diagnose.Check{Status: diagnose.StatusSkip}
			}
			return run(p)
		}}
	}
	return append(checks,
		withProfile("wine_graphics", func(p gameruntime.Profile) diagnose.Check {
			return diagnose.Check{Status: diagnose.StatusInfo, Params: map[string]string{"backend": p.GraphicsBackend()}}
		}),
		withProfile("wine_vcruntime", func(p gameruntime.Profile) diagnose.Check {
			if vcruntime.IsInstalledIn(p) {
				return diagnose.Check{Status: diagnose.StatusOK}
			}
			return diagnose.Check{Status: diagnose.StatusInfo}
		}),
		withProfile("wine_gameinput", func(p gameruntime.Profile) diagnose.Check {
			if gameinput.IsInstalledIn(p) {
				return diagnose.Check{Status: diagnose.StatusOK}
			}
			return diagnose.Check{Status: diagnose.StatusInfo}
		}),
	)
}

func wineDiagnosticReason(err error) string {
	switch {
	case errors.Is(err, gameruntime.ErrExecutableMissing):
		return "missing"
	case errors.Is(err, gameruntime.ErrPrefixUninitialized):
		return "prefix"
	case errors.Is(err, gameruntime.ErrUnsupportedArch):
		return "unsupported_arch"
	case errors.Is(err, gameruntime.ErrUnsupportedOS):
		return "unsupported_os"
	case errors.Is(err, gameruntime.ErrD3DMetalUnavailable), errors.Is(err, gameruntime.ErrGraphicsBackendUnavailable):
		return "graphics"
	default:
		return "invalid"
	}
}
