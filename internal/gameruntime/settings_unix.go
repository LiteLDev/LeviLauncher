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

func settingsPath() string { return filepath.Join(apppath.ConfigDir(), "wine.json") }

func LoadSettings() (types.WineSettings, error) { return loadSettings(settingsPath()) }

func loadSettings(path string) (types.WineSettings, error) {
	var s types.WineSettings
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return s, err
	}
	if err == nil {
		if err := json.Unmarshal(b, &s); err != nil {
			return s, fmt.Errorf("invalid Wine settings: %w", err)
		}
	}
	return resolveSettings(s)
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
			s.App = filepath.Join(home, "Applications", "Minecraft Wine", "Minecraft Wine.app")
		}
	}
	if s.Executable == "" {
		if s.App != "" {
			s.Executable = filepath.Join(s.App, "Contents/SharedSupport/wine/bin/wine")
		} else {
			s.Executable, _ = exec.LookPath("wine")
		}
	}
	if s.Prefix == "" {
		s.Prefix = os.Getenv("LEVI_WINEPREFIX")
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
	if s.Sync == "" {
		s.Sync = "auto"
	}
	for _, field := range []*string{&s.App, &s.Executable, &s.Prefix} {
		*field = strings.TrimSpace(*field)
		if strings.HasPrefix(*field, "~/") {
			*field = filepath.Join(home, (*field)[2:])
		}
		if *field != "" {
			*field = filepath.Clean(*field)
		}
	}
	return s, nil
}

func profileFromSettings(s types.WineSettings) (Profile, error) {
	s, err := resolveSettings(s)
	p := Profile{App: s.App, Wine: s.Executable, Prefix: s.Prefix, Graphics: s.Graphics, Sync: s.Sync}
	if err != nil {
		return p, err
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
		if s.Executable != filepath.Join(s.App, "Contents/SharedSupport/wine/bin/wine") {
			return p, errors.New("Wine executable does not belong to the selected app; clear the app path to use a custom executable")
		}
	}
	info, err := os.Stat(s.Executable)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
		return p, errors.New("Wine executable is missing or is not executable")
	}
	info, err = os.Stat(filepath.Join(s.Prefix, "drive_c"))
	if err != nil || !info.IsDir() {
		return p, errors.New("Wine prefix is not initialized (drive_c is missing)")
	}
	if p.crossOverRoot() != "" {
		if _, err := os.Stat(filepath.Join(s.Prefix, "cxbottle.conf")); err != nil {
			return p, errors.New("select a CrossOver bottle directory")
		}
	}
	switch s.Graphics {
	case "auto", "wined3d":
	case "d3dmetal":
		if root := p.crossOverRoot(); root != "" {
			if _, err := os.Stat(filepath.Join(root, "lib64/apple_gptk")); err != nil {
				return p, errors.New("D3DMetal is missing from the selected Wine runtime")
			}
			break
		}
		if s.App == "" {
			return p, errors.New("D3DMetal requires a compatible Wine app bundle")
		}
		if _, err := os.Stat(filepath.Join(s.App, "Contents/SharedSupport/renderer/d3dmetal/wine")); err != nil {
			return p, errors.New("D3DMetal is missing from the selected Wine app")
		}
	default:
		return p, errors.New("unknown Wine graphics backend")
	}
	switch s.Sync {
	case "auto", "msync", "esync", "none":
	default:
		return p, errors.New("unknown Wine synchronization mode")
	}
	return p, nil
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
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	output, err := p.Command(ctx, "--version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Wine version check failed: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}
