package diagnose

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Endpoints the launcher depends on.
const (
	urlMicrosoftAuth = "https://login.live.com/"
	urlXboxStore     = "https://displaycatalog.mp.microsoft.com/v7.0/products/9NBLGGH2JHXJ?market=US&languages=en-us"
	urlXboxPackages  = "https://packagespc.xboxlive.com/GetBasePackage/7792d9ce-355a-493c-afbd-768f4a77c3b0"
	urlVersionDB     = "https://raw.githubusercontent.com/LiteLDev/minecraft-windows-gdk-version-db/refs/heads/main/historical_versions.json"
	urlGitHub        = "https://github.com/LiteLDev/LeviLamina"
	urlLeviMC        = "https://levimc.org/"
	// DefaultGithubProxy is what "Use mirror" routes GitHub through.
	DefaultGithubProxy = "https://gh-proxy.org/https://github.com"
	// llDBPath is fetched through a GitHub proxy to test it end to end.
	llDBPath = "/LiteLDev/levilamina-client-version-db/raw/refs/heads/main/v2/version-db.json"
)

// Disk space thresholds: a GDK install needs its package plus the extracted
// game next to it.
const (
	diskFailBytes = 3 << 30
	diskWarnBytes = 10 << 30
	slowProbe     = 4 * time.Second
	clockFail     = 5 * time.Minute
	clockWarn     = 2 * time.Minute
)

var registry = map[string]checkFunc{
	"vcruntime":         checkVCRuntime,
	"gameinput":         checkGameInput,
	"gaming_services":   checkGamingServices,
	"xbox_services":     checkXboxServices,
	"developer_mode":    checkDeveloperMode,
	"webview2":          checkWebView2,
	"data_root":         checkDataRoot,
	"disk_space":        checkDiskSpace,
	"appdata_link":      checkAppDataLink,
	"elevation":         checkElevation,
	"account":           checkAccount,
	"license":           checkLicense,
	"proxy":             checkProxy,
	"mirror":            checkMirror,
	"clock":             checkClock,
	"ep_microsoft_auth": endpointCheck(urlMicrosoftAuth, false),
	"ep_xbox_store":     endpointCheck(urlXboxStore, true),
	"ep_xbox_packages":  endpointCheck(urlXboxPackages, false),
	"ep_version_db":     endpointCheck(urlVersionDB, true),
	"ep_github":         endpointCheck(urlGitHub, false),
	"ep_levimc":         endpointCheck(urlLeviMC, false),
	"ep_github_proxy":   checkGithubProxy,
	"ep_ll_db":          checkLLDB,
	"ll_support":        checkLLSupport,
	"instance_files":    checkInstanceFiles,
	"instance_path":     checkInstancePath,
	"instance_running":  checkInstanceRunning,
	"registration":      checkRegistration,
}

func p(kv ...string) map[string]string {
	m := make(map[string]string, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return m
}

func isGDK(inst *Instance) bool { return inst != nil && inst.PackageType != "uwp" }
func isUWP(inst *Instance) bool { return inst != nil && inst.PackageType == "uwp" }

// gdkNeed is how much a GDK prerequisite matters for this request: required
// for a GDK instance, irrelevant for a UWP one, advisory without an instance.
func gdkNeed(req Request, inst *Instance) (missing string, skip bool) {
	switch {
	case isUWP(inst):
		return "", true
	case isGDK(inst):
		return StatusFail, false
	}
	return StatusWarn, false
}

func checkVCRuntime(_ context.Context, env *Env, _ Request, _ *Instance) Check {
	if env.VCRuntime() {
		return Check{Status: StatusOK}
	}
	return Check{Status: StatusFail, Params: p("reason", "missing")}
}

func checkGameInput(_ context.Context, env *Env, req Request, inst *Instance) Check {
	missing, skip := gdkNeed(req, inst)
	if skip {
		return Check{Status: StatusSkip, Params: p("reason", "uwp")}
	}
	if env.GameInput() {
		return Check{Status: StatusOK}
	}
	return Check{Status: missing, Params: p("reason", "missing")}
}

func checkGamingServices(_ context.Context, env *Env, req Request, inst *Instance) Check {
	missing, skip := gdkNeed(req, inst)
	if skip {
		return Check{Status: StatusSkip, Params: p("reason", "uwp")}
	}
	if !env.GamingServices() {
		return Check{Status: missing, Params: p("reason", "missing")}
	}
	if env.Service != nil {
		svc := env.Service("GamingServices")
		if svc.Exists && svc.StartType == "disabled" {
			return Check{Status: missing, Params: p("reason", "disabled")}
		}
	}
	return Check{Status: StatusOK}
}

// checkXboxServices looks for the Xbox services LTSC and "optimized" Windows
// builds disable; Gaming Services and sign-in rely on them.
func checkXboxServices(_ context.Context, env *Env, req Request, inst *Instance) Check {
	if isUWP(inst) {
		return Check{Status: StatusSkip, Params: p("reason", "uwp")}
	}
	if env.Service == nil {
		return Check{Status: StatusSkip}
	}
	var disabled []string
	for _, name := range []string{"XblAuthManager", "XboxNetApiSvc", "XblGameSave"} {
		if s := env.Service(name); s.Exists && s.StartType == "disabled" {
			disabled = append(disabled, name)
		}
	}
	if len(disabled) == 0 {
		return Check{Status: StatusOK}
	}
	return Check{Status: StatusWarn, Params: p("reason", "disabled", "services", strings.Join(disabled, ", "))}
}

func checkDeveloperMode(_ context.Context, env *Env, req Request, inst *Instance) Check {
	if isGDK(inst) && req.Topic == TopicLaunch {
		return Check{Status: StatusSkip, Params: p("reason", "gdk")}
	}
	if env.DevMode() {
		return Check{Status: StatusOK}
	}
	if isUWP(inst) || req.Topic == TopicUWP {
		return Check{Status: StatusFail, Params: p("reason", "off")}
	}
	return Check{Status: StatusInfo, Params: p("reason", "off")}
}

func checkWebView2(_ context.Context, env *Env, _ Request, _ *Instance) Check {
	if env.WebView2() {
		return Check{Status: StatusOK}
	}
	return Check{Status: StatusWarn, Params: p("reason", "missing")}
}

// hasNonASCII reports characters that some game components and mods cannot
// handle in paths (the FAQ's "Chinese characters in the path" problem).
func hasNonASCII(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII {
			return true
		}
	}
	return false
}

func checkDataRoot(_ context.Context, env *Env, _ Request, _ *Instance) Check {
	root := env.DataRoot()
	if strings.TrimSpace(root) == "" {
		return Check{Status: StatusFail, Params: p("reason", "unknown")}
	}
	if !env.CanWrite(root) {
		return Check{Status: StatusFail, Params: p("reason", "not_writable", "path", root)}
	}
	if hasNonASCII(root) {
		return Check{Status: StatusWarn, Params: p("reason", "non_ascii", "path", root)}
	}
	return Check{Status: StatusOK, Params: p("path", root)}
}

func checkDiskSpace(_ context.Context, env *Env, _ Request, inst *Instance) Check {
	path := env.DataRoot()
	if inst != nil && inst.Dir != "" {
		path = inst.Dir
	}
	free, ok := env.FreeBytes(path)
	if !ok {
		return Check{Status: StatusSkip, Params: p("reason", "unknown", "path", path)}
	}
	params := p("path", path, "free", strconv.FormatUint(free, 10))
	switch {
	case free < diskFailBytes:
		params["reason"] = "low"
		return Check{Status: StatusFail, Params: params}
	case free < diskWarnBytes:
		params["reason"] = "low"
		return Check{Status: StatusWarn, Params: params}
	}
	return Check{Status: StatusOK, Params: params}
}

// checkAppDataLink finds shared data folders that other tools (BedrockBoot
// and similar) replaced with links; they break version isolation.
func checkAppDataLink(_ context.Context, env *Env, _ Request, _ *Instance) Check {
	for _, dir := range env.BedrockDataDirs() {
		if env.IsLink(dir) {
			return Check{Status: StatusWarn, Params: p("reason", "link", "path", dir)}
		}
	}
	return Check{Status: StatusOK}
}

func checkElevation(_ context.Context, env *Env, req Request, _ *Instance) Check {
	if !env.Elevated() {
		return Check{Status: StatusOK}
	}
	if req.Topic == TopicLicense {
		return Check{Status: StatusWarn, Params: p("reason", "elevated")}
	}
	return Check{Status: StatusInfo, Params: p("reason", "elevated")}
}

func checkAccount(_ context.Context, env *Env, req Request, _ *Instance) Check {
	xuid, unreachable := env.XUID()
	if strings.TrimSpace(xuid) != "" {
		return Check{Status: StatusOK}
	}
	if unreachable {
		// Signed in, but Xbox Live cannot be reached to confirm it.
		return Check{Status: StatusFail, Params: p("reason", "unreachable")}
	}
	if req.Topic == TopicFull {
		return Check{Status: StatusWarn, Params: p("reason", "signed_out")}
	}
	return Check{Status: StatusFail, Params: p("reason", "signed_out")}
}

// checkLicense asks Microsoft which channels the account owns. Params
// "release" and "preview" are authorized, trial, not_entitled or error.
func checkLicense(ctx context.Context, env *Env, req Request, _ *Instance) Check {
	xuid, unreachable := env.XUID()
	xuid = strings.TrimSpace(xuid)
	if xuid == "" {
		if unreachable {
			return Check{Status: StatusSkip, Params: p("reason", "unreachable")}
		}
		return Check{Status: StatusSkip, Params: p("reason", "signed_out")}
	}
	lctx, cancel := context.WithTimeout(ctx, LicenseTimeout)
	defer cancel()
	release, preview := env.Licenses(lctx, xuid)
	params := p("release", release, "preview", preview, "channel", req.Channel)
	channels := []string{"release", "preview"}
	switch req.Channel {
	case "release", "preview":
		channels = []string{req.Channel}
	}
	var statuses []string
	for _, ch := range channels {
		state := release
		if ch == "preview" {
			state = preview
		}
		switch state {
		case "authorized":
			statuses = append(statuses, StatusOK)
		case "trial", "not_entitled":
			if len(channels) == 1 {
				statuses = append(statuses, StatusFail)
			} else {
				statuses = append(statuses, StatusWarn)
			}
		default:
			statuses = append(statuses, StatusWarn)
		}
	}
	if lctx.Err() != nil && (release == "error" || preview == "error") {
		params["reason"] = "timeout"
	}
	return Check{Status: worst(statuses...), Params: params}
}

func checkProxy(ctx context.Context, env *Env, _ Request, _ *Instance) Check {
	info, err := env.Proxy()
	if err != nil {
		return Check{Status: StatusSkip, Params: p("reason", "unknown"), Detail: err.Error()}
	}
	addr, mode := "", "none"
	switch {
	case info.Env != "":
		addr, mode = info.Env, "env"
	case info.PAC != "":
		mode = "pac"
	case info.AutoDetect:
		mode = "auto"
	}
	if addr == "" && info.Manual != "" {
		addr = info.Manual
		if mode == "none" {
			mode = "manual"
		}
	}
	params := p("mode", mode)
	if info.PAC != "" {
		params["pac"] = info.PAC
	}
	if addr == "" {
		if mode == "none" {
			return Check{Status: StatusOK, Params: params}
		}
		return Check{Status: StatusInfo, Params: params}
	}
	hostport := proxyHostPort(addr)
	params["address"] = hostport
	if isLoopback(hostport) {
		params["local"] = "true"
	}
	dctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := env.Dial(dctx, hostport); err != nil {
		params["reason"] = "unreachable"
		return Check{Status: StatusFail, Params: params, Detail: err.Error()}
	}
	return Check{Status: StatusInfo, Params: params}
}

// proxyHostPort turns "http://host:port", "host:port" or "host" into
// host:port.
func proxyHostPort(addr string) string {
	a := strings.TrimSpace(addr)
	if !strings.Contains(a, "://") {
		a = "http://" + a
	}
	u, err := url.Parse(a)
	if err != nil || u.Host == "" {
		return strings.TrimSpace(addr)
	}
	if u.Port() == "" {
		port := "80"
		if u.Scheme == "https" {
			port = "443"
		}
		return net.JoinHostPort(u.Hostname(), port)
	}
	return u.Host
}

func isLoopback(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// checkMirror reports whether mirrors are on and recommends them when GitHub
// cannot be reached directly.
func checkMirror(ctx context.Context, env *Env, _ Request, _ *Instance) Check {
	on := env.Mirror()
	params := p("enabled", strconv.FormatBool(on))
	if on {
		return Check{Status: StatusInfo, Params: params}
	}
	res := env.shared.probe(ctx, urlVersionDB)
	if res.Err != nil || res.Status >= 400 || res.Latency > slowProbe {
		params["reason"] = "github_unreachable"
		return Check{Status: StatusWarn, Params: params}
	}
	return Check{Status: StatusInfo, Params: params}
}

// checkClock compares the local clock with a server's Date header; a wrong
// clock breaks TLS and Microsoft sign-in and licensing.
func checkClock(ctx context.Context, env *Env, _ Request, _ *Instance) Check {
	var date time.Time
	for _, u := range []string{urlXboxStore, urlMicrosoftAuth, urlVersionDB} {
		if r := env.shared.probe(ctx, u); !r.Date.IsZero() {
			date = r.Date.Add(r.Latency / 2)
			break
		}
	}
	if date.IsZero() {
		return Check{Status: StatusSkip, Params: p("reason", "no_server")}
	}
	skew := env.now().Sub(date)
	abs := skew
	if abs < 0 {
		abs = -abs
	}
	params := p("skew", strconv.FormatInt(int64(skew/time.Second), 10))
	switch {
	case abs >= clockFail:
		params["reason"] = "skew"
		return Check{Status: StatusFail, Params: params}
	case abs >= clockWarn:
		params["reason"] = "skew"
		return Check{Status: StatusWarn, Params: params}
	}
	return Check{Status: StatusOK, Params: params}
}

// endpointCheck probes target; strict endpoints must answer 2xx, others only
// need to answer at all (they reject requests without credentials).
func endpointCheck(target string, strict bool) checkFunc {
	return func(ctx context.Context, env *Env, _ Request, _ *Instance) Check {
		return probeCheck(env.shared.probe(ctx, target), target, strict)
	}
}

func probeCheck(r ProbeResult, target string, strict bool) Check {
	params := p("url", target, "latency", strconv.FormatInt(r.Latency.Milliseconds(), 10))
	if r.Err != nil {
		params["reason"] = classifyError(r.Err)
		return Check{Status: StatusFail, Params: params, Detail: r.Err.Error()}
	}
	params["http"] = strconv.Itoa(r.Status)
	switch {
	case r.Status == 429:
		params["reason"] = "rate_limited"
		return Check{Status: StatusWarn, Params: params}
	case r.Status >= 500:
		params["reason"] = "server_error"
		return Check{Status: StatusWarn, Params: params}
	case strict && r.Status >= 400:
		params["reason"] = "http_error"
		return Check{Status: StatusWarn, Params: params}
	case r.Latency > slowProbe:
		params["reason"] = "slow"
		return Check{Status: StatusWarn, Params: params}
	}
	return Check{Status: StatusOK, Params: params}
}

// classifyError names the kind of network failure.
func classifyError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "dns"
	}
	var nerr net.Error
	if errors.As(err, &nerr) && nerr.Timeout() {
		return "timeout"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "proxyconnect"):
		return "proxy"
	case strings.Contains(msg, "no such host") || strings.Contains(msg, "server misbehaving"):
		return "dns"
	case strings.Contains(msg, "x509") || strings.Contains(msg, "certificate") || strings.Contains(msg, "tls"):
		return "tls"
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline"):
		return "timeout"
	case strings.Contains(msg, "refused") || strings.Contains(msg, "actively refused"):
		return "refused"
	case strings.Contains(msg, "reset") || strings.Contains(msg, "forcibly closed") || strings.Contains(msg, "eof"):
		return "reset"
	}
	return "other"
}

// checkGithubProxy tests the GitHub proxy mods are installed through, or the
// default one when GitHub is reached directly (to tell whether turning on the
// mirror would help).
func checkGithubProxy(ctx context.Context, env *Env, _ Request, _ *Instance) Check {
	proxy := strings.TrimRight(strings.TrimSpace(env.GithubProxy()), "/")
	active := proxy != ""
	if !active {
		proxy = DefaultGithubProxy
	}
	target := proxy + llDBPath
	c := probeCheck(env.shared.probe(ctx, target), target, true)
	c.Params["proxy"] = proxy
	c.Params["active"] = strconv.FormatBool(active)
	c.Params["custom"] = strconv.FormatBool(active && env.GithubProxyCustom != nil && env.GithubProxyCustom())
	if !active && c.Status != StatusOK {
		// Not in use, so its failure only means it is no alternative.
		c.Status = StatusWarn
	}
	return c
}

func checkLLDB(ctx context.Context, env *Env, _ Request, _ *Instance) Check {
	db, err, dur := env.shared.llVersions(ctx)
	params := p("latency", strconv.FormatInt(dur.Milliseconds(), 10))
	if err != nil {
		params["reason"] = classifyError(err)
		return Check{Status: StatusFail, Params: params, Detail: err.Error()}
	}
	params["count"] = strconv.Itoa(len(db))
	return Check{Status: StatusOK, Params: params}
}

func checkLLSupport(ctx context.Context, env *Env, _ Request, inst *Instance) Check {
	if isUWP(inst) {
		return Check{Status: StatusSkip, Params: p("reason", "uwp")}
	}
	v := strings.TrimSpace(inst.GameVersion)
	if v == "" {
		return Check{Status: StatusSkip, Params: p("reason", "unknown_version")}
	}
	db, err, _ := env.shared.llVersions(ctx)
	if err != nil {
		return Check{Status: StatusSkip, Params: p("reason", "db_unavailable", "version", v)}
	}
	if list := db[v]; len(list) > 0 {
		return Check{Status: StatusOK, Params: p("version", v, "latest", list[len(list)-1], "count", strconv.Itoa(len(list)))}
	}
	return Check{Status: StatusWarn, Params: p("reason", "unsupported", "version", v)}
}

func checkInstanceFiles(_ context.Context, _ *Env, _ Request, inst *Instance) Check {
	if fi, err := os.Stat(inst.Dir); err != nil || !fi.IsDir() {
		return Check{Status: StatusFail, Params: p("reason", "dir_missing", "path", inst.Dir)}
	}
	exe := filepath.Join(inst.Dir, "Minecraft.Windows.exe")
	if isUWP(inst) {
		if inst.Executable == "" {
			return Check{Status: StatusFail, Params: p("reason", "manifest_missing", "path", inst.Dir)}
		}
		exe = filepath.Join(inst.Dir, inst.Executable)
	}
	if fi, err := os.Stat(exe); err != nil || fi.IsDir() {
		return Check{Status: StatusFail, Params: p("reason", "exe_missing", "path", exe)}
	}
	return Check{Status: StatusOK, Params: p("path", inst.Dir, "version", inst.GameVersion, "type", inst.PackageType)}
}

func checkInstancePath(_ context.Context, _ *Env, _ Request, inst *Instance) Check {
	if hasNonASCII(inst.Dir) {
		return Check{Status: StatusWarn, Params: p("reason", "non_ascii", "path", inst.Dir)}
	}
	if len(inst.Dir) > 160 {
		return Check{Status: StatusWarn, Params: p("reason", "long", "path", inst.Dir)}
	}
	return Check{Status: StatusOK, Params: p("path", inst.Dir)}
}

func checkInstanceRunning(_ context.Context, env *Env, _ Request, inst *Instance) Check {
	for _, exe := range env.RunningExes() {
		if env.SamePath(filepath.Dir(exe), inst.Dir) {
			return Check{Status: StatusWarn, Params: p("reason", "running")}
		}
	}
	return Check{Status: StatusOK}
}

// checkRegistration reports which folder Windows has registered as the
// channel's Minecraft app. UWP instances must be that folder to launch.
func checkRegistration(_ context.Context, env *Env, _ Request, inst *Instance) Check {
	loc := env.Registered(inst.Channel)
	params := p("channel", inst.Channel)
	switch {
	case loc == "":
		params["state"] = "none"
	case env.SamePath(loc, inst.Dir):
		params["state"] = "this"
		return Check{Status: StatusOK, Params: params}
	default:
		params["state"] = "other"
		params["path"] = loc
	}
	if !isUWP(inst) {
		return Check{Status: StatusInfo, Params: params}
	}
	if params["state"] == "other" {
		// Launching re-registers the instance, replacing the other
		// registration only when it belongs to the launcher.
		return Check{Status: StatusWarn, Params: params}
	}
	return Check{Status: StatusInfo, Params: params}
}
