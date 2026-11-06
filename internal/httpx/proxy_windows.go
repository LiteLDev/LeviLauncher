package httpx

import (
	"net/http"
	"net/url"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	winHTTPAccessTypeNoProxy  = 1
	winHTTPAccessTypeNamed    = 3
	winHTTPAutoProxyDetect    = 0x00000001
	winHTTPAutoProxyConfigURL = 0x00000002
	winHTTPDetectTypeDHCP     = 0x00000001
	winHTTPDetectTypeDNSA     = 0x00000002

	autoProxyLookupTimeout = 750 * time.Millisecond
	autoProxyRetryDelay    = 30 * time.Second
)

type winHTTPCurrentUserIEProxyConfig struct {
	autoDetect  uint32
	autoConfig  *uint16
	proxy       *uint16
	proxyBypass *uint16
}

type winHTTPAutoProxyOptions struct {
	flags                 uint32
	autoDetectFlags       uint32
	autoConfigURL         *uint16
	reserved              uintptr
	reservedLength        uint32
	autoLogonIfChallenged uint32
}

type winHTTPProxyInfo struct {
	accessType  uint32
	proxy       *uint16
	proxyBypass *uint16
}

type ieProxyConfig struct {
	autoDetect  bool
	autoConfig  string
	proxy       string
	proxyBypass string
}

var (
	winHTTPDLL                             = windows.NewLazySystemDLL("winhttp.dll")
	kernel32DLL                            = windows.NewLazySystemDLL("kernel32.dll")
	procWinHTTPOpen                        = winHTTPDLL.NewProc("WinHttpOpen")
	procWinHTTPGetIEProxyConfigCurrentUser = winHTTPDLL.NewProc("WinHttpGetIEProxyConfigForCurrentUser")
	procWinHTTPGetProxyForURL              = winHTTPDLL.NewProc("WinHttpGetProxyForUrl")
	procGlobalFree                         = kernel32DLL.NewProc("GlobalFree")

	winHTTPSessionOnce sync.Once
	winHTTPSession     uintptr
	winHTTPSessionErr  error

	autoProxyLookupMu           sync.Mutex
	autoProxyLookupActive       bool
	autoProxyLookupTimedOut     bool
	autoProxyLookupDisabledTill time.Time
)

// Lookups behind the system proxy resolvers; replaced in tests.
var (
	envProxy           = http.ProxyFromEnvironment
	loadIEProxyConfig  = getCurrentUserIEProxyConfig
	lookupAutoProxyURL = proxyURLFromAutoConfig
)

func newBaseTransport() *http.Transport {
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.Proxy = proxyFromWindowsSystem
	return base
}

func newBlockingBaseTransport() *http.Transport {
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.Proxy = proxyFromWindowsSystemBlocking
	return base
}

// autoProxyResolver resolves the PAC/WPAD proxy for req. resolved=false
// falls back to the manual proxy; an error fails the request.
type autoProxyResolver func(req *http.Request, cfg ieProxyConfig) (proxyURL *url.URL, resolved bool, err error)

// proxyFromWindowsSystem keeps UI requests responsive: PAC/WPAD lookups get
// a soft time budget and are skipped while another one runs or after a
// failure.
func proxyFromWindowsSystem(req *http.Request) (*url.URL, error) {
	return systemProxyForRequest(req, func(req *http.Request, cfg ieProxyConfig) (*url.URL, bool, error) {
		proxyURL, resolved := proxyURLFromAutoConfigWithBudget(req.URL, cfg)
		return proxyURL, resolved, nil
	})
}

// proxyFromWindowsSystemBlocking waits for the PAC/WPAD result like .NET's
// HttpClient.DefaultProxy, falling back to the manual proxy only when
// WinHTTP reports a failure. It gives up when the request's context ends.
func proxyFromWindowsSystemBlocking(req *http.Request) (*url.URL, error) {
	return systemProxyForRequest(req, func(req *http.Request, cfg ieProxyConfig) (*url.URL, bool, error) {
		type result struct {
			proxyURL *url.URL
			resolved bool
			err      error
		}
		// WinHttpGetProxyForUrl cannot be cancelled; a lookup abandoned
		// with its request finishes in the background.
		resultCh := make(chan result, 1)
		go func() {
			proxyURL, resolved, err := lookupAutoProxyURL(req.URL, cfg)
			resultCh <- result{proxyURL, resolved, err}
		}()
		select {
		case r := <-resultCh:
			if r.err != nil {
				return nil, false, nil
			}
			return r.proxyURL, r.resolved, nil
		case <-req.Context().Done():
			return nil, false, req.Context().Err()
		}
	})
}

func systemProxyForRequest(req *http.Request, resolveAuto autoProxyResolver) (*url.URL, error) {
	if req == nil || req.URL == nil {
		return nil, nil
	}
	if isImplicitDirectHost(req.URL.Hostname()) {
		return nil, nil
	}

	if proxyURL, err := envProxy(req); proxyURL != nil || err != nil {
		return proxyURL, err
	}

	cfg, err := loadIEProxyConfig()
	if err != nil {
		return nil, nil
	}

	manual := newManualProxyConfig(cfg.proxy, cfg.proxyBypass)

	if cfg.autoDetect || cfg.autoConfig != "" {
		proxyURL, resolved, err := resolveAuto(req, cfg)
		if err != nil {
			return nil, err
		}
		if resolved {
			return proxyURL, nil
		}
	}

	return manual.proxyForURL(req.URL)
}

func proxyURLFromAutoConfigWithBudget(reqURL *url.URL, cfg ieProxyConfig) (*url.URL, bool) {
	if reqURL == nil || !beginAutoProxyLookup() {
		return nil, false
	}

	proxyURL, resolved, err, timedOut := runWithSoftTimeout(
		autoProxyLookupTimeout,
		func() (*url.URL, bool, error) {
			return lookupAutoProxyURL(reqURL, cfg)
		},
		finishAutoProxyLookup,
		markAutoProxyLookupTimeout,
	)
	if timedOut || err != nil {
		return nil, false
	}
	return proxyURL, resolved
}

func beginAutoProxyLookup() bool {
	autoProxyLookupMu.Lock()
	defer autoProxyLookupMu.Unlock()

	now := time.Now()
	if autoProxyLookupActive {
		return false
	}
	if now.Before(autoProxyLookupDisabledTill) {
		return false
	}

	autoProxyLookupActive = true
	autoProxyLookupTimedOut = false
	return true
}

func finishAutoProxyLookup(err error) {
	autoProxyLookupMu.Lock()
	defer autoProxyLookupMu.Unlock()

	autoProxyLookupActive = false
	timedOut := autoProxyLookupTimedOut
	autoProxyLookupTimedOut = false
	if err != nil {
		until := time.Now().Add(autoProxyRetryDelay)
		if until.After(autoProxyLookupDisabledTill) {
			autoProxyLookupDisabledTill = until
		}
		return
	}
	if timedOut {
		return
	}

	autoProxyLookupDisabledTill = time.Time{}
}

func markAutoProxyLookupTimeout() {
	autoProxyLookupMu.Lock()
	defer autoProxyLookupMu.Unlock()

	autoProxyLookupTimedOut = true
	until := time.Now().Add(autoProxyRetryDelay)
	if until.After(autoProxyLookupDisabledTill) {
		autoProxyLookupDisabledTill = until
	}
}

func getCurrentUserIEProxyConfig() (ieProxyConfig, error) {
	var cfg winHTTPCurrentUserIEProxyConfig
	if err := callWinHTTPBool(procWinHTTPGetIEProxyConfigCurrentUser.Call(uintptr(unsafe.Pointer(&cfg)))); err != nil {
		return ieProxyConfig{}, err
	}
	defer freeGlobalString(cfg.autoConfig)
	defer freeGlobalString(cfg.proxy)
	defer freeGlobalString(cfg.proxyBypass)

	return ieProxyConfig{
		autoDetect:  cfg.autoDetect != 0,
		autoConfig:  windows.UTF16PtrToString(cfg.autoConfig),
		proxy:       windows.UTF16PtrToString(cfg.proxy),
		proxyBypass: windows.UTF16PtrToString(cfg.proxyBypass),
	}, nil
}

func proxyURLFromAutoConfig(reqURL *url.URL, cfg ieProxyConfig) (*url.URL, bool, error) {
	if reqURL == nil {
		return nil, false, nil
	}

	session, err := getWinHTTPSession()
	if err != nil {
		return nil, false, err
	}

	var options winHTTPAutoProxyOptions
	if cfg.autoDetect {
		options.flags |= winHTTPAutoProxyDetect
		options.autoDetectFlags = winHTTPDetectTypeDHCP | winHTTPDetectTypeDNSA
	}
	if cfg.autoConfig != "" {
		autoConfigURL, err := windows.UTF16PtrFromString(cfg.autoConfig)
		if err != nil {
			return nil, false, err
		}
		options.flags |= winHTTPAutoProxyConfigURL
		options.autoConfigURL = autoConfigURL
	}
	if options.flags == 0 {
		return nil, false, nil
	}
	options.autoLogonIfChallenged = 1

	requestURL, err := windows.UTF16PtrFromString(reqURL.String())
	if err != nil {
		return nil, false, err
	}

	var proxyInfo winHTTPProxyInfo
	if err := callWinHTTPBool(procWinHTTPGetProxyForURL.Call(
		session,
		uintptr(unsafe.Pointer(requestURL)),
		uintptr(unsafe.Pointer(&options)),
		uintptr(unsafe.Pointer(&proxyInfo)),
	)); err != nil {
		return nil, false, err
	}
	defer freeGlobalString(proxyInfo.proxy)
	defer freeGlobalString(proxyInfo.proxyBypass)

	if proxyInfo.accessType != winHTTPAccessTypeNamed {
		return nil, true, nil
	}

	proxyURL, err := parseAutoProxyURL(windows.UTF16PtrToString(proxyInfo.proxy))
	if err != nil {
		return nil, false, err
	}
	return proxyURL, true, nil
}

func getWinHTTPSession() (uintptr, error) {
	winHTTPSessionOnce.Do(func() {
		userAgent, err := windows.UTF16PtrFromString(UserAgent())
		if err != nil {
			winHTTPSessionErr = err
			return
		}

		handle, _, callErr := procWinHTTPOpen.Call(
			uintptr(unsafe.Pointer(userAgent)),
			winHTTPAccessTypeNoProxy,
			0,
			0,
			0,
		)
		if handle == 0 {
			winHTTPSessionErr = normalizeCallError(callErr)
			return
		}
		winHTTPSession = handle
	})
	return winHTTPSession, winHTTPSessionErr
}

func callWinHTTPBool(r1 uintptr, _ uintptr, callErr error) error {
	if r1 != 0 {
		return nil
	}
	return normalizeCallError(callErr)
}

func normalizeCallError(callErr error) error {
	if errno, ok := callErr.(syscall.Errno); ok && errno != 0 {
		return errno
	}
	return windows.GetLastError()
}

func freeGlobalString(ptr *uint16) {
	if ptr == nil {
		return
	}
	_, _, _ = procGlobalFree.Call(uintptr(unsafe.Pointer(ptr)))
}
