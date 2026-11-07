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
// holding config.yaml and liprc.json with the given contents (none when
// empty). The real %APPDATA%\lip is never touched.
func setLipConfig(t *testing.T, configYAML, liprc string) string {
	t.Helper()
	dir := t.TempDir()
	if configYAML != "" {
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(configYAML), 0o644); err != nil {
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

const lipConfigWithProxy = "format: 4\ngithub_proxy: https://myproxy.example\n"

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
		{name: "trimmed", config: "format: 4\ngithub_proxy: '  https://myproxy.example  '\n", want: "https://myproxy.example"},
		{name: "no proxy", config: "format: 4\n"},
		{name: "empty proxy", config: "format: 4\ngithub_proxy: ''\n"},
		{name: "missing format", config: "github_proxy: https://myproxy.example\n"},
		{name: "wrong format", config: "format: 3\ngithub_proxy: https://myproxy.example\n"},
		{name: "not a string", config: "format: 4\ngithub_proxy: [1]\n"},
		{name: "not a mapping", config: "- 1\n"},
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

// A lip v0.x liprc.json is migrated to config.yaml on first read, as lip
// 0.38 does.
func TestLipConfigMigratesLipRC(t *testing.T) {
	dir := setLipConfig(t, "", lipRCWithProxy)
	if got := lipConfigGithubProxy(); got != "https://legacy.example" {
		t.Fatalf("lipConfigGithubProxy() = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "liprc.json")); !os.IsNotExist(err) {
		t.Fatalf("liprc.json should be removed after migration: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil || !strings.Contains(string(data), "https://legacy.example") {
		t.Fatalf("config.yaml = %q, %v", data, err)
	}
	if got := lipConfigGithubProxy(); got != "https://legacy.example" {
		t.Fatalf("after migration lipConfigGithubProxy() = %q", got)
	}
}

// config.yaml wins over a leftover liprc.json, which is then kept.
func TestLipConfigIgnoresLipRCBesideConfig(t *testing.T) {
	dir := setLipConfig(t, "format: 4\n", lipRCWithProxy)
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
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); !os.IsNotExist(err) {
		t.Fatalf("config.yaml should not be written: %v", err)
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
