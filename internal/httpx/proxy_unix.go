//go:build darwin

package httpx

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http/httpproxy"
)

type unixProxyCache struct {
	mu       sync.Mutex
	settings map[string]string
	until    time.Time
	load     func() (map[string]string, error)
	now      func() time.Time
}

var systemProxy = unixProxyCache{load: loadUnixProxySettings, now: time.Now}

func (c *unixProxyCache) get() (map[string]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if c.settings != nil && now.Before(c.until) {
		return c.settings, nil
	}
	settings, err := c.load()
	if err != nil {
		c.until = now.Add(time.Second)
		if c.settings != nil {
			return c.settings, nil
		}
		return nil, err
	}
	c.settings = settings
	c.until = now.Add(5 * time.Second)
	return settings, nil
}

func newBaseTransport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = proxyFromUnixSystem
	return t
}
func newBlockingBaseTransport() *http.Transport { return newBaseTransport() }

// NewDirectClient is used only as a fallback for native Xbox token exchanges.
// It retains the normal certificate checks and headers, and has its own pool.
func NewDirectClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &http.Client{Transport: &defaultHeaderTransport{base: transport}, Timeout: timeout}
}

// UsesSystemProxy distinguishes automatic system routing from an explicit
// environment proxy. A caller must not bypass an explicitly configured proxy.
func UsesSystemProxy(req *http.Request) bool {
	env := httpproxy.FromEnvironment()
	explicit := env.HTTPProxy
	if req.URL.Scheme == "https" {
		explicit = env.HTTPSProxy
	}
	if explicit != "" || os.Getenv("ALL_PROXY") != "" || os.Getenv("all_proxy") != "" {
		return false
	}
	proxy, err := proxyFromUnixSystem(req)
	return err == nil && proxy != nil
}

// ProxyForRequest reports the same effective proxy used by the macOS transport.
func ProxyForRequest(req *http.Request) (*url.URL, error) {
	return proxyFromUnixSystem(req)
}

func proxyFromUnixSystem(req *http.Request) (*url.URL, error) {
	if isImplicitDirectHost(strings.ToLower(req.URL.Hostname())) {
		return nil, nil
	}
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	settings, err := systemProxy.get()
	env := httpproxy.FromEnvironment()
	// Fill the system fallback before evaluating NO_PROXY, so an explicit
	// bypass remains a direct connection rather than falling through to macOS.
	schemeProxy := env.HTTPProxy
	if req.URL.Scheme == "https" {
		schemeProxy = env.HTTPSProxy
	}
	if schemeProxy == "" {
		schemeProxy = os.Getenv("ALL_PROXY")
		if schemeProxy == "" {
			schemeProxy = os.Getenv("all_proxy")
		}
	}
	if err != nil && schemeProxy == "" {
		return nil, err
	}
	return proxyFromUnixSettings(req, settings, env)
}

func proxyFromUnixSettings(req *http.Request, settings map[string]string, env *httpproxy.Config) (*url.URL, error) {
	cfg := *env
	all := os.Getenv("ALL_PROXY")
	if all == "" {
		all = os.Getenv("all_proxy")
	}
	if cfg.HTTPProxy == "" {
		cfg.HTTPProxy = all
	}
	if cfg.HTTPSProxy == "" {
		cfg.HTTPSProxy = all
	}
	if cfg.HTTPProxy == "" {
		if proxy := proxyFromMacSettings("http", settings); proxy != nil {
			cfg.HTTPProxy = proxy.String()
		}
	}
	if cfg.HTTPSProxy == "" {
		if proxy := proxyFromMacSettings("https", settings); proxy != nil {
			cfg.HTTPSProxy = proxy.String()
		}
	}
	if settings["ExcludeSimpleHostnames"] == "1" && isLocalHostName(req.URL.Hostname()) {
		return nil, nil
	}
	cfg.NoProxy = strings.Trim(strings.Join([]string{cfg.NoProxy, settings["ExceptionsList"]}, ","), ",")
	return cfg.ProxyFunc()(req.URL)
}

func loadUnixProxySettings() (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "/usr/sbin/scutil", "--proxy").Output()
	if err != nil {
		return nil, fmt.Errorf("read macOS proxy settings: %w", err)
	}
	return parseUnixProxySettings(string(output)), nil
}

func parseUnixProxySettings(output string) map[string]string {
	settings := map[string]string{}
	depth := 0
	exceptions := false
	var bypass []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "}" {
			if depth == 2 {
				exceptions = false
			}
			depth--
			continue
		}
		key, value, ok := strings.Cut(line, " : ")
		if strings.HasSuffix(line, "{") {
			depth++
			if depth == 2 && key == "ExceptionsList" {
				exceptions = true
			}
			continue
		}
		if !ok {
			continue
		}
		if depth == 1 {
			settings[key] = strings.TrimSpace(value)
		}
		if depth == 2 && exceptions {
			bypass = append(bypass, strings.TrimSpace(value))
		}
	}
	settings["ExceptionsList"] = strings.Join(bypass, ",")
	return settings
}

func proxyFromMacSettings(scheme string, settings map[string]string) *url.URL {
	name := "HTTP"
	if scheme == "https" {
		name = "HTTPS"
	}
	proxyScheme := "http"
	if settings[name+"Enable"] != "1" {
		if settings["SOCKSEnable"] != "1" {
			return nil
		}
		name = "SOCKS"
		proxyScheme = "socks5"
	}
	host := settings[name+"Proxy"]
	port, err := strconv.Atoi(settings[name+"Port"])
	if host == "" || err != nil || port < 1 || port > 65535 {
		return nil
	}
	return &url.URL{Scheme: proxyScheme, Host: net.JoinHostPort(host, strconv.Itoa(port))}
}
