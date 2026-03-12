package httpx

import (
	"net/http"
	"net/url"
	"sync"
	"syscall"
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
)

func newBaseTransport() *http.Transport {
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.Proxy = proxyFromWindowsSystem
	return base
}

func proxyFromWindowsSystem(req *http.Request) (*url.URL, error) {
	if req == nil || req.URL == nil {
		return nil, nil
	}
	if isImplicitDirectHost(req.URL.Hostname()) {
		return nil, nil
	}

	if proxyURL, err := http.ProxyFromEnvironment(req); proxyURL != nil || err != nil {
		return proxyURL, err
	}

	cfg, err := getCurrentUserIEProxyConfig()
	if err != nil {
		return nil, nil
	}

	if cfg.autoDetect || cfg.autoConfig != "" {
		proxyURL, resolved, autoErr := proxyURLFromAutoConfig(req.URL, cfg)
		if autoErr == nil && resolved {
			return proxyURL, nil
		}
	}

	return newManualProxyConfig(cfg.proxy, cfg.proxyBypass).proxyForURL(req.URL)
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
