// Package lip adapts the native lip engine (internal/lipcore) to the
// launcher: it runs package operations per instance directory, forwards
// engine reports as lip_task_* events and keeps the status API the frontend
// used when lip was an external lipd.exe.
package lip

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	EventLipInstallStatus   = "lip_install_status"
	EventLipInstallProgress = "lip_install_progress"
	EventLipInstallDone     = "lip_install_done"
	EventLipInstallError    = "lip_install_error"

	// NativeVersion is reported as both the installed and the latest lip
	// version: lip is built into the launcher, so it is always up to date.
	NativeVersion = "0.37.0-native"

	defaultLipGithubProxy = "https://github.bibk.top"
)

// Status mirrors the runtime status the frontend reads via GetLipStatus.
type Status struct {
	Path           string `json:"path"`
	Installed      bool   `json:"installed"`
	UpToDate       bool   `json:"upToDate"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	Error          string `json:"error"`
}

// emitEvent sends an application event; replaced in tests.
var emitEvent = func(event string, data any) {
	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit(event, data)
}

// Proxy configuration sources; replaced in tests.
var (
	githubProxyOverride = config.GetLipGithubProxyOverride
	mirrorEnabled       = utils.IsMirrorEnabled
)

// GithubProxy returns the GitHub proxy shown in the launcher settings: the
// user's override, otherwise the built-in default.
func GithubProxy() string {
	if proxy := strings.TrimSpace(githubProxyOverride()); proxy != "" {
		return proxy
	}
	return defaultLipGithubProxy
}

// EffectiveGithubProxy returns the proxy the engine uses for GitHub URLs.
// An explicitly saved proxy always applies; otherwise the built-in default
// is used only when mirrors are enabled, so users elsewhere reach GitHub
// directly (lip's default behavior).
func EffectiveGithubProxy() string {
	if proxy := strings.TrimSpace(githubProxyOverride()); proxy != "" {
		return proxy
	}
	if mirrorEnabled() {
		return defaultLipGithubProxy
	}
	return ""
}

func normalizeGithubProxy(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultLipGithubProxy, nil
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return "", fmt.Errorf("ERR_LIP_INVALID_GITHUB_PROXY")
	}
	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return "", fmt.Errorf("ERR_LIP_INVALID_GITHUB_PROXY")
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = ""
	return parsed.String(), nil
}

// SetGithubProxy validates and persists the GitHub proxy. It applies to the
// next package operation.
func SetGithubProxy(raw string) string {
	normalized, err := normalizeGithubProxy(raw)
	if err != nil {
		return extractErrCode(err)
	}
	if err := config.Update(func(c *config.AppConfig) { c.LipGithubProxy = normalized }); err != nil {
		return "ERR_WRITE_FILE"
	}
	return ""
}

func extractErrCode(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return "ERR_LIP_INSTALL_FAILED"
	}
	if idx := strings.Index(msg, ":"); idx > 0 {
		head := strings.TrimSpace(msg[:idx])
		if strings.HasPrefix(head, "ERR_") {
			return head
		}
	}
	if strings.HasPrefix(msg, "ERR_") {
		return msg
	}
	return "ERR_LIP_INSTALL_FAILED"
}

// ErrorCode extracts the leading "ERR_X" code of err, defaulting to
// ERR_LIP_INSTALL_FAILED.
func ErrorCode(err error) string {
	return extractErrCode(err)
}

// IsInstalled reports whether lip is available. It is built in, so always.
func IsInstalled() bool { return true }

// EnsureInstalledWithError is kept for compatibility; lip is built in.
func EnsureInstalledWithError() error { return nil }

// GetVersion returns the built-in lip version.
func GetVersion() string { return NativeVersion }

// GetLatestVersion returns the built-in lip version.
func GetLatestVersion() (string, error) { return NativeVersion, nil }

// CheckStatus reports the built-in lip as installed and up to date.
func CheckStatus() Status {
	return Status{
		Installed:      true,
		UpToDate:       true,
		CurrentVersion: NativeVersion,
		LatestVersion:  NativeVersion,
	}
}

// Install is a no-op kept for the frontend's install flow: lip is built in,
// so it reports success immediately.
func Install() string {
	emitEvent(EventLipInstallStatus, "done")
	emitEvent(EventLipInstallProgress, map[string]interface{}{
		"percentage": float64(100),
		"current":    int64(0),
		"total":      int64(0),
	})
	emitEvent(EventLipInstallDone, NativeVersion)
	return ""
}
