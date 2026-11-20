package diagnose

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeEnv is a healthy system: every check passes.
func fakeEnv() *Env {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	return &Env{
		Now: func() time.Time { return now },
		Instance: func(name string) (Instance, bool) {
			return Instance{Name: name, Dir: `C:\nonexistent-diagnose-test\` + name, GameVersion: "1.21.120", Channel: "release", PackageType: "gdk"}, true
		},
		VCRuntime:       func() bool { return true },
		GameInput:       func() bool { return true },
		GamingServices:  func() bool { return true },
		Service:         func(string) ServiceInfo { return ServiceInfo{Exists: true, Running: true, StartType: "auto"} },
		DevMode:         func() bool { return true },
		WebView2:        func() bool { return true },
		Elevated:        func() bool { return false },
		DataRoot:        func() string { return `C:\Games\LeviLauncher` },
		CanWrite:        func(string) bool { return true },
		FreeBytes:       func(string) (uint64, bool) { return 100 << 30, true },
		BedrockDataDirs: func() []string { return []string{`C:\AppData\Minecraft Bedrock`} },
		IsLink:          func(string) bool { return false },
		RunningExes:     func() []string { return nil },
		Registered:      func(string) string { return "" },
		SamePath:        func(a, b string) bool { return strings.EqualFold(a, b) },
		XUID:            func() (string, bool) { return "2535400000000000", false },
		Licenses: func(context.Context, string) (string, string) {
			return "authorized", "authorized"
		},
		Proxy: func() (ProxyInfo, error) { return ProxyInfo{}, nil },
		Dial:  func(context.Context, string) error { return nil },
		Get: func(context.Context, string) ProbeResult {
			return ProbeResult{Status: 200, Date: now, Latency: 100 * time.Millisecond}
		},
		Mirror:      func() bool { return false },
		GithubProxy: func() string { return "" },
		LLVersions: func(context.Context) (map[string][]string, error) {
			return map[string][]string{"1.21.120": {"1.6.0", "1.6.1"}}, nil
		},
	}
}

func find(t *testing.T, r Report, id string) Check {
	t.Helper()
	for _, c := range r.Checks {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("check %q not in report %+v", id, r.Checks)
	return Check{}
}

func has(r Report, id string) bool {
	for _, c := range r.Checks {
		if c.ID == id {
			return true
		}
	}
	return false
}

func TestHealthySystemHasNoFailures(t *testing.T) {
	for _, topic := range []string{TopicFull, TopicInstall, TopicDownload, TopicLicense} {
		r := Run(context.Background(), fakeEnv(), Request{Topic: topic})
		for _, c := range r.Checks {
			if c.Status == StatusFail || c.Status == StatusWarn {
				t.Errorf("%s: %s = %s %v", topic, c.ID, c.Status, c.Params)
			}
		}
	}
}

func TestPlatformChecksCanBeExcluded(t *testing.T) {
	env := fakeEnv()
	env.GamingServices = nil
	env.SkipCheck = func(id string) bool { return id == "gaming_services" }
	report := Run(context.Background(), env, Request{Topic: TopicLaunch})
	if has(report, "gaming_services") || !has(report, "vcruntime") {
		t.Fatalf("unexpected checks: %+v", report.Checks)
	}
}

func TestRuntimeChecksMatchTopic(t *testing.T) {
	env := fakeEnv()
	env.RuntimeChecks = []RuntimeCheck{
		{ID: "wine_runtime", Run: func(context.Context) Check { return Check{Status: StatusOK} }},
	}
	for _, topic := range []string{"", TopicFull, TopicInstall, TopicLaunch, TopicLicense, TopicDownload, TopicUWP, TopicLIP} {
		report := Run(context.Background(), env, Request{Topic: topic})
		want := topic == "" || topic == TopicFull || topic == TopicInstall || topic == TopicLaunch
		if has(report, "wine_runtime") != want {
			t.Errorf("topic %q: expected Wine check presence %v", topic, want)
		}
	}
	if report := Run(context.Background(), fakeEnv(), Request{Topic: TopicFull}); has(report, "wine_runtime") {
		t.Fatal("runtime check ran without a platform implementation")
	}
}

func TestInstanceChecksNeedAnInstance(t *testing.T) {
	r := Run(context.Background(), fakeEnv(), Request{Topic: TopicLaunch})
	for _, id := range []string{"instance_files", "registration", "instance_running", "instance_path"} {
		if has(r, id) {
			t.Errorf("%s ran without an instance", id)
		}
	}
	if !has(r, "vcruntime") {
		t.Error("vcruntime missing from the launch topic")
	}
	r = Run(context.Background(), fakeEnv(), Request{Topic: TopicLaunch, Instance: "Survival"})
	if c := find(t, r, "instance_files"); c.Status != StatusFail || c.Params["reason"] != "dir_missing" {
		t.Errorf("instance_files = %+v", c)
	}
}

func TestGDKPrerequisitesDependOnInstanceType(t *testing.T) {
	env := fakeEnv()
	env.GameInput = func() bool { return false }
	env.GamingServices = func() bool { return false }
	env.DevMode = func() bool { return false }

	r := Run(context.Background(), env, Request{Topic: TopicLaunch, Instance: "gdk"})
	if c := find(t, r, "gameinput"); c.Status != StatusFail {
		t.Errorf("GDK gameinput = %s", c.Status)
	}
	if c := find(t, r, "developer_mode"); c.Status != StatusSkip {
		t.Errorf("GDK developer_mode = %s", c.Status)
	}

	env.Instance = func(name string) (Instance, bool) {
		return Instance{Name: name, Dir: `C:\x\` + name, Channel: "release", PackageType: "uwp"}, true
	}
	r = Run(context.Background(), env, Request{Topic: TopicLaunch, Instance: "uwp"})
	if c := find(t, r, "gameinput"); c.Status != StatusSkip {
		t.Errorf("UWP gameinput = %s", c.Status)
	}
	if c := find(t, r, "developer_mode"); c.Status != StatusFail {
		t.Errorf("UWP developer_mode = %s", c.Status)
	}

	r = Run(context.Background(), env, Request{Topic: TopicFull})
	if c := find(t, r, "gaming_services"); c.Status != StatusWarn {
		t.Errorf("no-instance gaming_services = %s", c.Status)
	}
}

func TestGamingServicesDisabled(t *testing.T) {
	env := fakeEnv()
	env.Service = func(name string) ServiceInfo {
		return ServiceInfo{Exists: true, StartType: "disabled"}
	}
	r := Run(context.Background(), env, Request{Topic: TopicLaunch, Instance: "a"})
	if c := find(t, r, "gaming_services"); c.Status != StatusFail || c.Params["reason"] != "disabled" {
		t.Errorf("gaming_services = %+v", c)
	}
	if c := find(t, r, "xbox_services"); c.Status != StatusWarn || !strings.Contains(c.Params["services"], "XblAuthManager") {
		t.Errorf("xbox_services = %+v", c)
	}
}

func TestLicenseReleaseMissingPreviewOwned(t *testing.T) {
	env := fakeEnv()
	env.Licenses = func(context.Context, string) (string, string) { return "not_entitled", "authorized" }

	r := Run(context.Background(), env, Request{Topic: TopicLicense})
	c := find(t, r, "license")
	if c.Status != StatusWarn || c.Params["release"] != "not_entitled" || c.Params["preview"] != "authorized" {
		t.Errorf("both channels: %+v", c)
	}
	r = Run(context.Background(), env, Request{Topic: TopicInstall, Channel: "release"})
	if c := find(t, r, "license"); c.Status != StatusFail {
		t.Errorf("release install: %+v", c)
	}
	r = Run(context.Background(), env, Request{Topic: TopicInstall, Channel: "preview"})
	if c := find(t, r, "license"); c.Status != StatusOK {
		t.Errorf("preview install: %+v", c)
	}
}

func TestSignedOutSkipsLicense(t *testing.T) {
	env := fakeEnv()
	env.XUID = func() (string, bool) { return "", false }
	r := Run(context.Background(), env, Request{Topic: TopicLicense})
	if c := find(t, r, "account"); c.Status != StatusFail {
		t.Errorf("account = %s", c.Status)
	}
	if c := find(t, r, "license"); c.Status != StatusSkip {
		t.Errorf("license = %s", c.Status)
	}
}

func TestUnreachableAccountIsNotSignedOut(t *testing.T) {
	env := fakeEnv()
	env.XUID = func() (string, bool) { return "", true }
	r := Run(context.Background(), env, Request{Topic: TopicFull})
	if c := find(t, r, "account"); c.Status != StatusFail || c.Params["reason"] != "unreachable" {
		t.Errorf("account = %+v", c)
	}
	if c := find(t, r, "license"); c.Status != StatusSkip || c.Params["reason"] != "unreachable" {
		t.Errorf("license = %+v", c)
	}
}

func TestStaleLocalProxy(t *testing.T) {
	env := fakeEnv()
	env.Proxy = func() (ProxyInfo, error) { return ProxyInfo{Manual: "127.0.0.1:7890"}, nil }
	env.Dial = func(context.Context, string) error { return errors.New("connection refused") }
	r := Run(context.Background(), env, Request{Topic: TopicLIP})
	c := find(t, r, "proxy")
	if c.Status != StatusFail || c.Params["reason"] != "unreachable" || c.Params["local"] != "true" || c.Params["address"] != "127.0.0.1:7890" {
		t.Errorf("proxy = %+v", c)
	}
}

func TestProxyHostPort(t *testing.T) {
	for in, want := range map[string]string{
		"127.0.0.1:7890":          "127.0.0.1:7890",
		"http://proxy.local:8080": "proxy.local:8080",
		"https://proxy.local":     "proxy.local:443",
		"proxy.local":             "proxy.local:80",
	} {
		if got := proxyHostPort(in); got != want {
			t.Errorf("proxyHostPort(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEndpointFailuresAreClassified(t *testing.T) {
	env := fakeEnv()
	env.Get = func(_ context.Context, url string) ProbeResult {
		switch {
		case strings.Contains(url, "githubusercontent"):
			return ProbeResult{Err: &net.DNSError{Err: "no such host", Name: "raw.githubusercontent.com"}}
		case strings.Contains(url, "gh-proxy"):
			return ProbeResult{Status: 200, Latency: time.Second}
		}
		return ProbeResult{Status: 200}
	}
	r := Run(context.Background(), env, Request{Topic: TopicDownload})
	if c := find(t, r, "ep_version_db"); c.Status != StatusFail || c.Params["reason"] != "dns" {
		t.Errorf("ep_version_db = %+v", c)
	}
	if c := find(t, r, "mirror"); c.Status != StatusWarn || c.Params["reason"] != "github_unreachable" {
		t.Errorf("mirror = %+v", c)
	}
	if c := find(t, r, "ep_github_proxy"); c.Status != StatusOK || c.Params["active"] != "false" {
		t.Errorf("ep_github_proxy = %+v", c)
	}
}

func TestClockSkew(t *testing.T) {
	env := fakeEnv()
	env.Get = func(context.Context, string) ProbeResult {
		return ProbeResult{Status: 200, Date: env.Now().Add(-10 * time.Minute)}
	}
	r := Run(context.Background(), env, Request{Topic: TopicLicense})
	if c := find(t, r, "clock"); c.Status != StatusFail || c.Params["skew"] != "600" {
		t.Errorf("clock = %+v", c)
	}
}

func TestLeviLaminaSupport(t *testing.T) {
	env := fakeEnv()
	r := Run(context.Background(), env, Request{Topic: TopicLIP, Instance: "a"})
	if c := find(t, r, "ll_support"); c.Status != StatusOK || c.Params["latest"] != "1.6.1" {
		t.Errorf("ll_support = %+v", c)
	}
	env.Instance = func(name string) (Instance, bool) {
		return Instance{Name: name, Dir: `C:\x`, GameVersion: "1.26.99", Channel: "release", PackageType: "gdk"}, true
	}
	r = Run(context.Background(), env, Request{Topic: TopicLIP, Instance: "a"})
	if c := find(t, r, "ll_support"); c.Status != StatusWarn || c.Params["reason"] != "unsupported" {
		t.Errorf("ll_support = %+v", c)
	}
}

func TestSharedProbesRunOnce(t *testing.T) {
	env := fakeEnv()
	var calls atomic.Int32
	env.Get = func(_ context.Context, url string) ProbeResult {
		if strings.Contains(url, "githubusercontent") {
			calls.Add(1)
		}
		return ProbeResult{Status: 200}
	}
	Run(context.Background(), env, Request{Topic: TopicFull})
	if n := calls.Load(); n != 1 {
		t.Errorf("version DB probed %d times, want 1", n)
	}
}

func TestRegistration(t *testing.T) {
	env := fakeEnv()
	env.Instance = func(name string) (Instance, bool) {
		return Instance{Name: name, Dir: `C:\i\` + name, Channel: "preview", PackageType: "uwp"}, true
	}
	env.Registered = func(ch string) string {
		if ch == "preview" {
			return `C:\Program Files\WindowsApps\Microsoft.MinecraftWindowsBeta`
		}
		return ""
	}
	r := Run(context.Background(), env, Request{Topic: TopicUWP, Instance: "p"})
	if c := find(t, r, "registration"); c.Status != StatusWarn || c.Params["state"] != "other" {
		t.Errorf("registration = %+v", c)
	}
}

func TestPlanOrderIsKept(t *testing.T) {
	r := Run(context.Background(), fakeEnv(), Request{Topic: TopicLicense})
	var ids []string
	for _, c := range r.Checks {
		ids = append(ids, c.ID)
	}
	want := "account,license,elevation,proxy,clock,ep_microsoft_auth,ep_xbox_store"
	if got := strings.Join(ids, ","); got != want {
		t.Errorf("order = %s, want %s", got, want)
	}
}
