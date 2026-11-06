package lip

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	setLipRC(t, "")
}

// setLipRC points liprc.json at a temp file holding content (none when
// empty).
func setLipRC(t *testing.T, content string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "liprc.json")
	if content != "" {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old := lipRCPath
	lipRCPath = func() string { return path }
	t.Cleanup(func() { lipRCPath = old })
}

const lipRCWithProxy = `{
  "format_version": 3,
  "format_uuid": "289f771f-2c9a-4d73-9f3f-8492495a924d",
  "github_proxy": "https://myproxy.example"
}`

func TestProxyFromLipRC(t *testing.T) {
	tests := []struct {
		name          string
		override      string
		mirror        bool
		wantEffective string
		wantDisplayed string
	}{
		{name: "liprc applies without mirror", wantEffective: "https://myproxy.example", wantDisplayed: "https://myproxy.example"},
		{name: "liprc beats mirror default", mirror: true, wantEffective: "https://myproxy.example", wantDisplayed: "https://myproxy.example"},
		{name: "override beats liprc", override: "https://override.example", wantEffective: "https://override.example", wantDisplayed: "https://override.example"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setProxyConfig(t, tt.override, tt.mirror)
			setLipRC(t, lipRCWithProxy)
			if got := EffectiveGithubProxy(); got != tt.wantEffective {
				t.Fatalf("EffectiveGithubProxy() = %q, want %q", got, tt.wantEffective)
			}
			if got := GithubProxy(); got != tt.wantDisplayed {
				t.Fatalf("GithubProxy() = %q, want %q", got, tt.wantDisplayed)
			}
		})
	}
}

func TestLipRCGithubProxyFollowsLipParsing(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "valid", content: lipRCWithProxy, want: "https://myproxy.example"},
		{name: "bom", content: string([]byte{0xef, 0xbb, 0xbf}) + lipRCWithProxy, want: "https://myproxy.example"},
		{name: "null proxy", content: `{"format_version":3,"format_uuid":"289f771f-2c9a-4d73-9f3f-8492495a924d","github_proxy":null}`},
		{name: "missing format", content: `{"github_proxy":"https://myproxy.example"}`},
		{name: "wrong version", content: `{"format_version":2,"format_uuid":"289f771f-2c9a-4d73-9f3f-8492495a924d","github_proxy":"https://myproxy.example"}`},
		{name: "wrong case key", content: `{"format_version":3,"format_uuid":"289f771f-2c9a-4d73-9f3f-8492495a924d","GitHub_Proxy":"https://myproxy.example"}`},
		{name: "not a string", content: `{"format_version":3,"format_uuid":"289f771f-2c9a-4d73-9f3f-8492495a924d","github_proxy":1}`},
		{name: "malformed", content: `{`},
		{name: "missing file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setLipRC(t, tt.content)
			if got := lipRCGithubProxy(); got != tt.want {
				t.Fatalf("lipRCGithubProxy() = %q, want %q", got, tt.want)
			}
		})
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

