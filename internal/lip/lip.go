// Package lip adapts the native lip engine (internal/lipcore) to the
// launcher: it runs package operations per instance directory, forwards
// engine reports as lip_task_* events and exposes package states keyed by
// package id. Workspaces follow lip (lip.toml packages, .lip/state.json) on
// the client side.
package lip

import (
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/eventsink"
	lipconfig "github.com/liteldev/LeviLauncher/internal/lipcore/config"
	"github.com/liteldev/LeviLauncher/internal/lipcore/lipdirs"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

const defaultLipGithubProxy = "https://gh-proxy.org/https://github.com"

// emitEvent sends an application event; replaced in tests.
var emitEvent = func(event string, data any) {
	eventsink.Emit(event, data)
}

// Proxy configuration sources; replaced in tests.
var (
	githubProxyOverride = config.GetLipGithubProxyOverride
	mirrorEnabled       = utils.IsMirrorEnabled
)

// lipConfigDir is lip's configuration directory (%APPDATA%\lip), holding
// config.toml; replaced in tests.
var lipConfigDir = lipdirs.ConfigDir

// lipConfigGithubProxy returns github_proxy from lip's config.toml, as set
// with `lip config set github_proxy`. Like lip, a missing config.toml
// is first migrated from a lip v0.x liprc.json, and a missing or invalid
// file means no proxy (lip reports it and continues).
func lipConfigGithubProxy() string {
	c, msgs, err := lipconfig.LoadIn(lipConfigDir())
	for _, m := range msgs {
		log.Printf("lip config: %s", m.Text)
	}
	if err != nil {
		log.Printf("lip config: %v", err)
		return ""
	}
	if !c.HasGithubProxy {
		return ""
	}
	return strings.TrimSpace(c.GithubProxy)
}

// configuredGithubProxy is the proxy the user chose: the launcher override,
// otherwise github_proxy from lip's config.toml.
func configuredGithubProxy() string {
	if proxy := strings.TrimSpace(githubProxyOverride()); proxy != "" {
		return completeGithubProxy(proxy)
	}
	return completeGithubProxy(lipConfigGithubProxy())
}

// fullURLProxyHosts are GitHub proxies that take the whole GitHub URL after
// their own (https://gh-proxy.org/https://github.com/owner/repo). lip only
// appends a request's path to the proxy, so these must be configured with
// "/https://github.com"; without it they answer 404 (gh-proxy.org) or 400
// (ghproxy.vip).
var fullURLProxyHosts = []string{"gh-proxy.org", "ghproxy.vip", "gh.bibk.top"}

const githubOrigin = "https://github.com"

// completeGithubProxy adds "/https://github.com" to a bare full-URL proxy
// (such as "https://gh-proxy.org", the form earlier launcher versions
// offered and saved) so that lip's path rewrite reaches GitHub through it.
// Any other proxy is returned unchanged.
func completeGithubProxy(proxy string) string {
	parsed, err := url.Parse(proxy)
	if err != nil || parsed.Host == "" || strings.Trim(parsed.Path, "/") != "" ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return proxy
	}
	host := strings.ToLower(parsed.Hostname())
	for _, h := range fullURLProxyHosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			return strings.TrimRight(proxy, "/") + "/" + githubOrigin
		}
	}
	return proxy
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
// the launcher override, then github_proxy from config.toml (what lip itself
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
	return completeGithubProxy(parsed.String()), nil
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

// ErrorResult is the result string handed to the GUI for a failed package
// operation: code, then lip's full message for err (the failure, its causes
// and its "hint:" lines, as lip prints them), so the launcher can explain
// the failure instead of showing a bare code. Without a message it is just
// code.
func ErrorResult(code string, err error) string {
	msg := strings.TrimSpace(formatEngineError(err))
	if msg == "" {
		return code
	}
	return code + ": " + msg
}
