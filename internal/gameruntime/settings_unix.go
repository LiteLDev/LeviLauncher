//go:build darwin

package gameruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/fsx"
	"github.com/liteldev/LeviLauncher/internal/types"
)

var (
	ErrExecutableMissing   = errors.New("Wine executable is missing or is not executable")
	ErrPrefixUninitialized = errors.New("Wine prefix is not initialized (drive_c is missing)")
	ErrD3DMetalUnavailable = errors.New("D3DMetal is missing from the selected Wine runtime")
)

func settingsPath() string { return filepath.Join(apppath.ConfigDir(), "wine.json") }

func LoadSettings() (types.WineSettings, error) { return loadSettings(settingsPath()) }

func loadSettings(path string) (types.WineSettings, error) {
	s, err := readSettings(path)
	if err != nil && !os.IsNotExist(err) {
		return s, err
	}
	return resolveSettings(s)
}

func readSettings(path string) (types.WineSettings, error) {
	var s types.WineSettings
	b, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("invalid Wine settings: %w", err)
	}
	return s, nil
}

func resolveSettings(s types.WineSettings) (types.WineSettings, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return s, err
	}
	if s.App == "" && s.Executable == "" {
		s.App = os.Getenv("LEVI_WINE_APP")
		s.Executable = os.Getenv("LEVI_WINE_EXECUTABLE")
		if s.App == "" && s.Executable == "" {
			// The managed installer selects MetalSharp after Wine and its
			// dependencies are ready. A staged bundle alone is not a selection.
			s.App = filepath.Join(home, "Applications", "Minecraft Wine", "Minecraft Wine.app")
		}
	}

	for _, field := range []*string{&s.App, &s.Executable, &s.Prefix} {
		*field = normalizeWinePath(home, *field)
	}
	metalSharp := metalSharpForSettings(s)
	s.Runtime = ""
	s.D3DMetalVersions = nil
	if metalSharp.app != "" {
		s.Runtime = "metalsharp"
		s.D3DMetalVersions = metalSharp.availableD3DMetalVersions()
	}
	if s.App == "" && metalSharp.app != "" {
		s.App = metalSharp.app
		s.Executable = appWineExecutable(s.App)
	}
	if s.Executable == "" {
		if s.App != "" {
			s.Executable = appWineExecutable(s.App)
		} else {
			s.Executable, _ = exec.LookPath("wine")
		}
	}
	if s.Prefix == "" {
		s.Prefix = os.Getenv("LEVI_WINEPREFIX")
		if s.Prefix != "" {
			s.Prefix = normalizeWinePath(home, s.Prefix)
		}
		if s.Prefix == "" && metalSharp.app != "" {
			s.Prefix = metalSharpPrefix(metalSharp.app)
		}
		if s.Prefix == "" && s.App != "" {
			s.Prefix = filepath.Join(s.App, "Contents/SharedSupport/prefix")
		}
		if s.Prefix == "" {
			s.Prefix = filepath.Join(home, ".wine")
		}
	}
	if s.Graphics == "" {
		s.Graphics = "auto"
	}
	if s.D3DMetalVersion == "" {
		s.D3DMetalVersion = "auto"
	}
	if s.Sync == "" {
		s.Sync = "auto"
	}
	if s.ThreadQoS == "" {
		s.ThreadQoS = "balanced"
	}
	if s.VSync == "" {
		s.VSync = "auto"
	}
	if s.MetalFXScale == "" {
		if s.EnableMetalFX {
			if s.RetinaMode {
				s.MetalFXScale = "performance"
			} else {
				s.MetalFXScale = "balanced"
			}
		} else {
			s.MetalFXScale = "off"
		}
	}
	s.EnableMetalFX = s.MetalFXScale != "off"
	if s.MetalFXSharpness == "" {
		s.MetalFXSharpness = "auto"
	}

	return s, nil
}

func normalizeWinePath(home, path string) string {
	path = strings.TrimSpace(path)
	if strings.HasPrefix(path, "~/") {
		path = filepath.Join(home, path[2:])
	}
	if path != "" {
		return filepath.Clean(path)
	}
	return ""
}

func profileFromSettings(s types.WineSettings) (Profile, error) {
	s, err := resolveSettings(s)
	p := Profile{
		App:               s.App,
		Wine:              s.Executable,
		Prefix:            s.Prefix,
		Graphics:          s.Graphics,
		D3DMetalVersion:   s.D3DMetalVersion,
		EnableMetal4:      s.EnableMetal4,
		Sync:              s.Sync,
		ThreadQoS:         s.ThreadQoS,
		DisableRosettaAVX: s.DisableRosettaAVX,
		LeftCommandIsCtrl: s.LeftCommandIsCtrl,
		RetinaMode:        s.RetinaMode,
		MetalHUD:          s.MetalHUD,
		VSync:             s.VSync,
		MaxFPS:            s.MaxFPS,
		EnableMetalFX:     s.EnableMetalFX,
		MetalFXScale:      s.MetalFXScale,
		MetalFXSharpness:  s.MetalFXSharpness,
		CompatDebugLog:    s.CompatDebugLog,
		metalSharp:        metalSharpForSettings(s),
	}
	if err != nil {
		return p, err
	}
	if s.Executable == "" {
		return p, ErrExecutableMissing
	}
	for _, path := range []string{s.Executable, s.Prefix} {
		if !filepath.IsAbs(path) {
			return p, errors.New("Wine executable and prefix must be absolute paths")
		}
	}
	if s.App != "" {
		if !filepath.IsAbs(s.App) {
			return p, errors.New("Wine app bundle must be an absolute path")
		}
		if expected := appWineExecutable(s.App); s.Executable != expected && (p.metalSharp.app == "" || !sameRuntimePath(s.Executable, expected)) {
			return p, errors.New("Wine executable does not belong to the selected app; clear the app path to use a custom executable")
		}
	}
	info, err := os.Stat(s.Executable)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
		return p, ErrExecutableMissing
	}
	if !winePrefixReady(s.Prefix, p.metalSharp.app != "") {
		return p, ErrPrefixUninitialized
	}
	if p.crossOverRoot() != "" {
		if _, err := os.Stat(filepath.Join(s.Prefix, "cxbottle.conf")); err != nil {
			return p, errors.New("select a CrossOver bottle directory")
		}
	}
	if s.MaxFPS < 0 {
		return p, errors.New("Wine frame-rate limit cannot be negative")
	}
	if p.metalSharp.app != "" {
		if err := p.metalSharp.validate(p); err != nil {
			return p, err
		}
	} else {
		switch s.Graphics {
		case "auto", "wined3d":
		case "d3dmetal":
			if root := p.crossOverRoot(); root != "" {
				if _, err := os.Stat(filepath.Join(root, "lib64/apple_gptk")); err != nil {
					return p, ErrD3DMetalUnavailable
				}
				break
			}
			if s.App == "" {
				return p, ErrD3DMetalUnavailable
			}
			if _, err := os.Stat(filepath.Join(s.App, "Contents/SharedSupport/renderer/d3dmetal/wine")); err != nil {
				return p, ErrD3DMetalUnavailable
			}
		default:
			return p, errors.New("unknown Wine graphics backend")
		}
	}
	switch s.Sync {
	case "auto", "msync", "esync", "none":
	default:
		return p, errors.New("unknown Wine synchronization mode")
	}
	switch s.D3DMetalVersion {
	case "auto", "gptk4", "gptk3":
	default:
		return p, errors.New("unknown D3DMetal version")
	}
	switch s.ThreadQoS {
	case "default", "balanced", "aggressive":
	default:
		return p, errors.New("unknown Wine thread QoS mode")
	}
	switch s.VSync {
	case "auto", "on", "off":
	default:
		return p, errors.New("unknown Wine VSync mode")
	}
	switch s.MetalFXScale {
	case "off", "quality", "balanced", "performance", "ultra_performance", "extreme", "max":
	default:
		return p, errors.New("unknown MetalFX upscaling mode")
	}
	switch s.MetalFXSharpness {
	case "", "auto", "0", "10", "15", "25", "35":
	default:
		return p, errors.New("unknown MetalFX sharpening level")
	}
	return p, nil
}

func winePrefixReady(prefix string, metalSharp bool) bool {
	if metalSharp {
		return initializedMetalSharpPrefix(prefix)
	}
	info, err := os.Stat(filepath.Join(prefix, "drive_c"))
	return err == nil && info.IsDir()
}

func SaveSettings(s types.WineSettings) error { return saveSettings(settingsPath(), s) }

func saveSettings(path string, s types.WineSettings) error {
	s, err := resolveSettings(s)
	if err != nil {
		return err
	}
	if _, err := profileFromSettings(s); err != nil {
		return err
	}
	s.Runtime = ""
	s.D3DMetalVersions = nil
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return fsx.WriteFileAtomic(path, b, 0600)
}

// CheckSettings checks the selected paths and engine version without creating
// or modifying a prefix. It does not claim a game can render successfully.
func CheckSettings(ctx context.Context, s types.WineSettings) (string, error) {
	p, err := profileFromSettings(s)
	if err != nil {
		return "", err
	}
	return p.CheckVersion(ctx)
}

// CheckVersion probes the engine of a validated profile.
func (p Profile) CheckVersion(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	output, err := p.Command(ctx, "--version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Wine version check failed: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}
