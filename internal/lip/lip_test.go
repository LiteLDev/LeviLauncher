package lip

import (
	"errors"
	"fmt"
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

func TestStatusShims(t *testing.T) {
	if !IsInstalled() {
		t.Fatal("IsInstalled must be true")
	}
	if err := EnsureInstalledWithError(); err != nil {
		t.Fatalf("EnsureInstalledWithError = %v", err)
	}
	if GetVersion() != NativeVersion {
		t.Fatalf("GetVersion = %q", GetVersion())
	}
	if v, err := GetLatestVersion(); err != nil || v != NativeVersion {
		t.Fatalf("GetLatestVersion = %q, %v", v, err)
	}
	st := CheckStatus()
	if !st.Installed || !st.UpToDate || st.CurrentVersion != NativeVersion || st.LatestVersion != NativeVersion || st.Error != "" {
		t.Fatalf("unexpected status: %+v", st)
	}
}

func TestInstallShimEmitsDone(t *testing.T) {
	rec := recordEvents(t)
	if code := Install(); code != "" {
		t.Fatalf("Install() = %q", code)
	}
	events := rec.snapshot()
	last := events[len(events)-1]
	if last.name != EventLipInstallDone || last.data != NativeVersion {
		t.Fatalf("unexpected events: %+v", events)
	}
	for _, ev := range events {
		if ev.name == EventLipInstallError {
			t.Fatalf("unexpected error event: %+v", ev)
		}
	}
}
