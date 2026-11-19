package diagnose

import (
	"context"
	"sync"
	"time"
)

// Instance is what the checks need to know about one game instance.
type Instance struct {
	Name        string
	Dir         string
	GameVersion string
	Channel     string // "release" or "preview"
	PackageType string // "gdk" or "uwp"
	Isolation   bool
	// Executable is the game executable a UWP manifest names, "" when the
	// manifest is missing or unreadable (or for GDK instances).
	Executable string
}

// ServiceInfo is the state of a Windows service.
type ServiceInfo struct {
	Exists    bool
	Running   bool
	StartType string // "auto", "manual", "disabled" or ""
}

// ProxyInfo is the proxy configuration requests follow.
type ProxyInfo struct {
	Env        string // proxy from environment variables
	AutoDetect bool
	PAC        string
	// Manual is the manual proxy address (host:port) used for HTTPS requests
	// to GitHub, "" when none applies.
	Manual string
}

// ProbeResult is the outcome of one HTTP request.
type ProbeResult struct {
	Status  int // HTTP status, 0 when no response
	Err     error
	Date    time.Time // the response's Date header, zero when absent
	Latency time.Duration
}

// Env is the system the checks look at. DefaultEnv returns the real one;
// tests build their own. Every function must be safe for concurrent use.
type Env struct {
	Now            func() time.Time
	Instance       func(name string) (Instance, bool)
	VCRuntime      func() bool
	GameInput      func() bool
	GamingServices func() bool
	Service        func(name string) ServiceInfo
	DevMode        func() bool
	WebView2       func() bool
	Elevated       func() bool
	DataRoot       func() string
	CanWrite       func(dir string) bool
	FreeBytes      func(path string) (uint64, bool)
	// BedrockDataDirs are the shared Minecraft data folders under %APPDATA%.
	BedrockDataDirs func() []string
	IsLink          func(path string) bool
	// RunningExes lists the executable paths of running Minecraft processes.
	RunningExes func() []string
	// Registered returns the install folder of the Minecraft package
	// registered with Windows for channel, "" when none is.
	Registered func(channel string) string
	SamePath   func(a, b string) bool
	// XUID is the signed-in account; unreachable tells a network failure
	// confirming it from being signed out.
	XUID     func() (xuid string, unreachable bool)
	Licenses func(ctx context.Context, xuid string) (release, preview string)
	Proxy    func() (ProxyInfo, error)
	Dial     func(ctx context.Context, address string) error
	Get      func(ctx context.Context, url string) ProbeResult
	Mirror   func() bool
	// GithubProxy is the GitHub proxy mods are installed through, "" for
	// direct; GithubProxyCustom tells whether the user set it.
	GithubProxy       func() string
	GithubProxyCustom func() bool
	LLVersions        func(ctx context.Context) (map[string][]string, error)

	shared *sharedState
}

func (e *Env) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

// sharedState memoizes work several checks need (the same endpoint probe,
// the LeviLamina version list) within one run.
type sharedState struct {
	env    *Env
	mu     sync.Mutex
	probes map[string]*probeOnce
	llOnce sync.Once
	llDB   map[string][]string
	llErr  error
	llDur  time.Duration
}

type probeOnce struct {
	once sync.Once
	res  ProbeResult
}

func newSharedState(env *Env) *sharedState {
	return &sharedState{env: env, probes: map[string]*probeOnce{}}
}

func (s *sharedState) probe(ctx context.Context, url string) ProbeResult {
	s.mu.Lock()
	p, ok := s.probes[url]
	if !ok {
		p = &probeOnce{}
		s.probes[url] = p
	}
	s.mu.Unlock()
	p.once.Do(func() {
		pctx, cancel := context.WithTimeout(ctx, ProbeTimeout)
		defer cancel()
		began := s.env.now()
		p.res = s.env.Get(pctx, url)
		if p.res.Latency == 0 {
			p.res.Latency = s.env.now().Sub(began)
		}
	})
	return p.res
}

func (s *sharedState) llVersions(ctx context.Context) (map[string][]string, error, time.Duration) {
	s.llOnce.Do(func() {
		lctx, cancel := context.WithTimeout(ctx, 2*ProbeTimeout)
		defer cancel()
		began := s.env.now()
		s.llDB, s.llErr = s.env.LLVersions(lctx)
		s.llDur = s.env.now().Sub(began)
	})
	return s.llDB, s.llErr, s.llDur
}
