package lip

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setProxyConfig(t *testing.T, override string, mirror bool) {
	t.Helper()
	oldOverride, oldMirror := githubProxyOverride, mirrorEnabled
	githubProxyOverride = func() string { return override }
	mirrorEnabled = func() bool { return mirror }
	t.Cleanup(func() {
		githubProxyOverride = oldOverride
		mirrorEnabled = oldMirror
	})
	setLipConfig(t, "", "")
}

// setLipConfig points lip's configuration directory at a temp directory
// holding config.toml and liprc.json with the given contents (none when
// empty). The real %APPDATA%\lip is never touched.
func setLipConfig(t *testing.T, configTOML, liprc string) string {
	t.Helper()
	dir := t.TempDir()
	if configTOML != "" {
		if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(configTOML), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if liprc != "" {
		if err := os.WriteFile(filepath.Join(dir, "liprc.json"), []byte(liprc), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old := lipConfigDir
	lipConfigDir = func() string { return dir }
	t.Cleanup(func() { lipConfigDir = old })
	return dir
}

const lipConfigWithProxy = "format = 4\ngithub_proxy = \"https://myproxy.example\"\n"

func TestProxyFromLipConfig(t *testing.T) {
	tests := []struct {
		name          string
		override      string
		mirror        bool
		wantEffective string
		wantDisplayed string
	}{
		{name: "config applies without mirror", wantEffective: "https://myproxy.example", wantDisplayed: "https://myproxy.example"},
		{name: "config beats mirror default", mirror: true, wantEffective: "https://myproxy.example", wantDisplayed: "https://myproxy.example"},
		{name: "override beats config", override: "https://override.example", wantEffective: "https://override.example", wantDisplayed: "https://override.example"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setProxyConfig(t, tt.override, tt.mirror)
			setLipConfig(t, lipConfigWithProxy, "")
			if got := EffectiveGithubProxy(); got != tt.wantEffective {
				t.Fatalf("EffectiveGithubProxy() = %q, want %q", got, tt.wantEffective)
			}
			if got := GithubProxy(); got != tt.wantDisplayed {
				t.Fatalf("GithubProxy() = %q, want %q", got, tt.wantDisplayed)
			}
		})
	}
}

func TestLipConfigGithubProxyFollowsLipParsing(t *testing.T) {
	tests := []struct {
		name   string
		config string
		want   string
	}{
		{name: "valid", config: lipConfigWithProxy, want: "https://myproxy.example"},
		{name: "trimmed", config: "format = 4\ngithub_proxy = '  https://myproxy.example  '\n", want: "https://myproxy.example"},
		{name: "no proxy", config: "format = 4\n"},
		{name: "empty proxy", config: "format = 4\ngithub_proxy = ''\n"},
		{name: "missing format", config: "github_proxy = \"https://myproxy.example\"\n"},
		{name: "wrong format", config: "format = 3\ngithub_proxy = \"https://myproxy.example\"\n"},
		{name: "not a string", config: "format = 4\ngithub_proxy = [1]\n"},
		{name: "not TOML", config: "format: 4\ngithub_proxy: https://myproxy.example\n"},
		{name: "missing file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setLipConfig(t, tt.config, "")
			if got := lipConfigGithubProxy(); got != tt.want {
				t.Fatalf("lipConfigGithubProxy() = %q, want %q", got, tt.want)
			}
		})
	}
}

const lipRCWithProxy = `{
  "format_version": 3,
  "format_uuid": "289f771f-2c9a-4d73-9f3f-8492495a924d",
  "github_proxy": "https://legacy.example"
}`

// A lip v0.x liprc.json is migrated to config.toml on first read, as lip
// does.
func TestLipConfigMigratesLipRC(t *testing.T) {
	dir := setLipConfig(t, "", lipRCWithProxy)
	if got := lipConfigGithubProxy(); got != "https://legacy.example" {
		t.Fatalf("lipConfigGithubProxy() = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "liprc.json")); !os.IsNotExist(err) {
		t.Fatalf("liprc.json should be removed after migration: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.toml"))
	if err != nil || !strings.Contains(string(data), "https://legacy.example") {
		t.Fatalf("config.toml = %q, %v", data, err)
	}
	if got := lipConfigGithubProxy(); got != "https://legacy.example" {
		t.Fatalf("after migration lipConfigGithubProxy() = %q", got)
	}
}

// config.toml wins over a leftover liprc.json, which is then kept.
func TestLipConfigIgnoresLipRCBesideConfig(t *testing.T) {
	dir := setLipConfig(t, "format = 4\n", lipRCWithProxy)
	if got := lipConfigGithubProxy(); got != "" {
		t.Fatalf("lipConfigGithubProxy() = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "liprc.json")); err != nil {
		t.Fatalf("liprc.json should be kept: %v", err)
	}
}

func TestLipConfigKeepsInvalidLipRC(t *testing.T) {
	dir := setLipConfig(t, "", `{"format_version":2,"github_proxy":"https://legacy.example"}`)
	if got := lipConfigGithubProxy(); got != "" {
		t.Fatalf("lipConfigGithubProxy() = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.toml")); !os.IsNotExist(err) {
		t.Fatalf("config.toml should not be written: %v", err)
	}
}

func TestProxySelection(t *testing.T) {
	tests := []struct {
		name          string
		override      string
		mirror        bool
		wantEffective string
		wantDisplayed string
	}{
		{name: "override wins outside China", override: "https://proxy.example", mirror: false, wantEffective: "https://proxy.example", wantDisplayed: "https://proxy.example"},
		{name: "override wins with mirror", override: " https://proxy.example ", mirror: true, wantEffective: "https://proxy.example", wantDisplayed: "https://proxy.example"},
		{name: "mirror uses default", override: "", mirror: true, wantEffective: defaultLipGithubProxy, wantDisplayed: defaultLipGithubProxy},
		{name: "no mirror goes direct", override: "", mirror: false, wantEffective: "", wantDisplayed: defaultLipGithubProxy},
		{name: "saved bare gh-proxy completed", override: "https://gh-proxy.org", mirror: true, wantEffective: "https://gh-proxy.org/https://github.com", wantDisplayed: "https://gh-proxy.org/https://github.com"},
		{name: "saved bare ghproxy.vip completed", override: "https://ghproxy.vip/", mirror: false, wantEffective: "https://ghproxy.vip/https://github.com", wantDisplayed: "https://ghproxy.vip/https://github.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setProxyConfig(t, tt.override, tt.mirror)
			if got := EffectiveGithubProxy(); got != tt.wantEffective {
				t.Fatalf("EffectiveGithubProxy() = %q, want %q", got, tt.wantEffective)
			}
			if got := GithubProxy(); got != tt.wantDisplayed {
				t.Fatalf("GithubProxy() = %q, want %q", got, tt.wantDisplayed)
			}
		})
	}
}

func TestNormalizeGithubProxy(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty restores default", input: "  ", want: defaultLipGithubProxy},
		{name: "https url", input: "https://github.bibk.top", want: "https://github.bibk.top"},
		{name: "trailing slashes trimmed", input: "https://example.com/proxy///", want: "https://example.com/proxy"},
		{name: "http url allowed", input: "http://127.0.0.1:8080", want: "http://127.0.0.1:8080"},
		{name: "missing scheme", input: "github.bibk.top", wantErr: true},
		{name: "unsupported scheme", input: "ftp://example.com", wantErr: true},
		{name: "missing host", input: "https://", wantErr: true},
		{name: "bare gh-proxy completed", input: "https://gh-proxy.org", want: "https://gh-proxy.org/https://github.com"},
		{name: "bare gh-proxy subdomain completed", input: "https://v4.gh-proxy.org/", want: "https://v4.gh-proxy.org/https://github.com"},
		{name: "complete gh-proxy kept", input: "https://hk.gh-proxy.org/https://github.com/", want: "https://hk.gh-proxy.org/https://github.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeGithubProxy(tt.input)
			if tt.wantErr {
				if code := extractErrCode(err); err == nil || code != "ERR_LIP_INVALID_GITHUB_PROXY" {
					t.Fatalf("normalizeGithubProxy(%q) = %q, %v; want ERR_LIP_INVALID_GITHUB_PROXY", tt.input, got, err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("normalizeGithubProxy(%q) = %q, %v; want %q", tt.input, got, err, tt.want)
			}
		})
	}
}

func TestSetGithubProxyRejectsInvalid(t *testing.T) {
	if code := SetGithubProxy("ftp://x"); code != "ERR_LIP_INVALID_GITHUB_PROXY" {
		t.Fatalf("got %q", code)
	}
}

func TestErrorCode(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{nil, ""},
		{fmt.Errorf("ERR_LIP_CACHE_CLEAN_FAILED: %w", errors.New("boom")), "ERR_LIP_CACHE_CLEAN_FAILED"},
		{errors.New("ERR_LIP_NOT_INSTALLED"), "ERR_LIP_NOT_INSTALLED"},
		{errors.New("Package 'x' is already explicitly installed"), "ERR_LIP_INSTALL_FAILED"},
		{errors.New(""), "ERR_LIP_INSTALL_FAILED"},
	}
	for _, tt := range tests {
		if got := ErrorCode(tt.err); got != tt.want {
			t.Fatalf("ErrorCode(%v) = %q, want %q", tt.err, got, tt.want)
		}
	}
}

func TestErrorResultCarriesTheEngineMessage(t *testing.T) {
	if got := ErrorResult("ERR_X", nil); got != "ERR_X" {
		t.Fatalf("nil error: got %q", got)
	}
	got := ErrorResult("ERR_X", errors.New("network down"))
	if !strings.HasPrefix(got, "ERR_X: ") || !strings.Contains(got, "network down") {
		t.Fatalf("got %q", got)
	}
}

func TestCompleteGithubProxy(t *testing.T) {
	tests := []struct{ in, want string }{
		{"https://gh-proxy.org", "https://gh-proxy.org/https://github.com"},
		{"https://gh-proxy.org/", "https://gh-proxy.org/https://github.com"},
		{"https://V4.GH-Proxy.org", "https://V4.GH-Proxy.org/https://github.com"},
		{"https://ghproxy.vip", "https://ghproxy.vip/https://github.com"},
		{"https://gh-proxy.org/https://github.com", "https://gh-proxy.org/https://github.com"},
		{"https://gh-proxy.org/custom", "https://gh-proxy.org/custom"},
		{"https://github.bibk.top", "https://github.bibk.top"},
		{"https://notgh-proxy.org", "https://notgh-proxy.org"},
		{"https://gh-proxy.org.evil.example", "https://gh-proxy.org.evil.example"},
		{"https://gh.bibk.top", "https://gh.bibk.top/https://github.com"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := completeGithubProxy(tt.in); got != tt.want {
			t.Errorf("completeGithubProxy(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
