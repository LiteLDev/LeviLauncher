// Package lip adapts the native lip engine (internal/lipcore) to the
// launcher: it runs package operations per instance directory, forwards
// engine reports as lip_task_* events and keeps the status API the frontend
// used when lip was an external lipd.exe.
package lip

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const defaultLipGithubProxy = "https://github.bibk.top"

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

// lipRCPath is lip's runtime config (ConfigService: %APPDATA%\lip\liprc.json);
// replaced in tests.
var lipRCPath = func() string {
	return filepath.Join(apppath.AppData(), "lip", "liprc.json")
}

// lipRCFormatUUID is RuntimeConfig's format_uuid (format_version 3).
const lipRCFormatUUID = "289f771f-2c9a-4d73-9f3f-8492495a924d"

// lipRCGithubProxy returns github_proxy from liprc.json, as set with
// `lip config set github_proxy` or written by older launchers. Like lip's
// ConfigService, a missing or invalid file (including a wrong
// format_version/format_uuid) means no proxy.
func lipRCGithubProxy() string {
	data, err := os.ReadFile(lipRCPath())
	if err != nil {
		return ""
	}
	// A map keeps System.Text.Json's case-sensitive property names.
	var rc map[string]json.RawMessage
	if err := json.Unmarshal(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")), &rc); err != nil {
		return ""
	}
	var version int
	var uuid, proxy string
	if json.Unmarshal(rc["format_version"], &version) != nil || version != 3 ||
		json.Unmarshal(rc["format_uuid"], &uuid) != nil || uuid != lipRCFormatUUID {
		return ""
	}
	if raw, ok := rc["github_proxy"]; !ok || json.Unmarshal(raw, &proxy) != nil {
		return ""
	}
	return strings.TrimSpace(proxy)
}

// configuredGithubProxy is the proxy the user chose: the launcher override,
// otherwise github_proxy from lip's liprc.json.
func configuredGithubProxy() string {
	if proxy := strings.TrimSpace(githubProxyOverride()); proxy != "" {
		return proxy
	}
	return lipRCGithubProxy()
}

// GithubProxy returns the GitHub proxy shown in the launcher settings (only
// while mirrors are enabled): the configured proxy, otherwise the built-in
// default. It matches EffectiveGithubProxy whenever mirrors are enabled.
func GithubProxy() string {
	if proxy := configuredGithubProxy(); proxy != "" {
		return proxy
	}
	return defaultLipGithubProxy
}

// EffectiveGithubProxy returns the proxy the engine uses for GitHub URLs:
// the launcher override, then github_proxy from liprc.json (what lip itself
// reads), then the built-in default when mirrors are enabled. Otherwise
// GitHub is reached directly (lip's default behavior).
func EffectiveGithubProxy() string {
	if proxy := configuredGithubProxy(); proxy != "" {
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

