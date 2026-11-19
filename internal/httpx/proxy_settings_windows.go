package httpx

import (
	"net/url"
	"os"
	"strings"
)

// ProxySettings describes how this process reaches the internet, for
// diagnostics: the proxy environment variables, then the current user's
// Windows (WinINet) proxy configuration that every request follows.
type ProxySettings struct {
	// Env is the proxy taken from HTTPS_PROXY/HTTP_PROXY/ALL_PROXY, if any.
	Env string
	// AutoDetect is "Automatically detect settings" (WPAD).
	AutoDetect bool
	// AutoConfigURL is the PAC script address ("Use setup script").
	AutoConfigURL string
	// Manual is the raw manual proxy value ("host:port" or
	// "http=host:port;https=host:port").
	Manual string
	// Bypass is the raw bypass list.
	Bypass string
}

// SystemProxySettings reads the proxy configuration requests use.
func SystemProxySettings() (ProxySettings, error) {
	var s ProxySettings
	for _, k := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			s.Env = v
			break
		}
	}
	cfg, err := loadIEProxyConfig()
	if err != nil {
		return s, err
	}
	s.AutoDetect = cfg.autoDetect
	s.AutoConfigURL = cfg.autoConfig
	s.Manual = cfg.proxy
	s.Bypass = cfg.proxyBypass
	return s, nil
}

// ManualProxyFor returns the manual proxy that applies to target (honoring
// the bypass list), or nil when the request goes direct.
func (s ProxySettings) ManualProxyFor(target string) *url.URL {
	u, err := url.Parse(target)
	if err != nil {
		return nil
	}
	p, err := newManualProxyConfig(s.Manual, s.Bypass).proxyForURL(u)
	if err != nil {
		return nil
	}
	return p
}
