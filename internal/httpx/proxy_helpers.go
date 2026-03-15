package httpx

import (
	"net"
	"net/url"
	"path"
	"strings"
	"time"
)

type proxyTarget struct {
	address string
	scheme  string
}

type manualProxyConfig struct {
	defaultProxy proxyTarget
	httpProxy    proxyTarget
	httpsProxy   proxyTarget
	bypassRules  []string
	bypassLocal  bool
}

func newManualProxyConfig(proxyValue string, bypassValue string) manualProxyConfig {
	cfg := manualProxyConfig{}
	cfg.defaultProxy, cfg.httpProxy, cfg.httpsProxy = parseManualProxyTargets(proxyValue)
	cfg.bypassRules, cfg.bypassLocal = parseBypassRules(bypassValue)
	return cfg
}

func (cfg manualProxyConfig) proxyForURL(reqURL *url.URL) (*url.URL, error) {
	if reqURL == nil {
		return nil, nil
	}
	host := strings.ToLower(strings.TrimSpace(reqURL.Hostname()))
	if host == "" || shouldBypassHost(host, cfg.bypassRules, cfg.bypassLocal) {
		return nil, nil
	}

	target := cfg.defaultProxy
	switch strings.ToLower(strings.TrimSpace(reqURL.Scheme)) {
	case "https":
		if cfg.httpsProxy.address != "" {
			target = cfg.httpsProxy
		}
	case "http":
		if cfg.httpProxy.address != "" {
			target = cfg.httpProxy
		}
	default:
		if cfg.defaultProxy.address == "" && cfg.httpsProxy.address != "" {
			target = cfg.httpsProxy
		}
	}

	if target.address == "" {
		return nil, nil
	}
	return parseProxyURL(target.address, target.scheme)
}

func parseManualProxyTargets(raw string) (proxyTarget, proxyTarget, proxyTarget) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return proxyTarget{}, proxyTarget{}, proxyTarget{}
	}
	if !strings.Contains(trimmed, "=") {
		target := proxyTarget{address: trimmed, scheme: "http"}
		return target, proxyTarget{}, proxyTarget{}
	}

	var fallback proxyTarget
	var httpTarget proxyTarget
	var httpsTarget proxyTarget

	for _, entry := range splitProxyEntries(trimmed) {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		label := strings.ToLower(strings.TrimSpace(key))
		target := proxyTarget{
			address: strings.TrimSpace(value),
			scheme:  proxySchemeForManualEntry(label),
		}
		if target.address == "" {
			continue
		}
		switch label {
		case "http":
			httpTarget = target
		case "https":
			httpsTarget = target
		case "socks", "socks4", "socks5":
			if fallback.address == "" {
				fallback = target
			}
			if httpTarget.address == "" {
				httpTarget = target
			}
			if httpsTarget.address == "" {
				httpsTarget = target
			}
		default:
			if fallback.address == "" {
				fallback = target
			}
		}
	}

	if fallback.address == "" {
		if httpTarget.address != "" {
			fallback = httpTarget
		} else if httpsTarget.address != "" {
			fallback = httpsTarget
		}
	}

	return fallback, httpTarget, httpsTarget
}

func parseBypassRules(raw string) ([]string, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, false
	}

	replacer := strings.NewReplacer(";", ",", "\n", ",")
	parts := strings.Split(replacer.Replace(raw), ",")
	rules := make([]string, 0, len(parts))
	bypassLocal := false
	for _, part := range parts {
		rule := strings.ToLower(strings.TrimSpace(part))
		if rule == "" {
			continue
		}
		if rule == "<local>" {
			bypassLocal = true
			continue
		}
		rules = append(rules, rule)
	}
	return rules, bypassLocal
}

func shouldBypassHost(host string, rules []string, bypassLocal bool) bool {
	trimmedHost := strings.ToLower(strings.TrimSpace(host))
	if trimmedHost == "" {
		return true
	}
	if isImplicitDirectHost(trimmedHost) {
		return true
	}
	if bypassLocal && isLocalHostName(trimmedHost) {
		return true
	}
	for _, rule := range rules {
		if matchBypassRule(trimmedHost, rule) {
			return true
		}
	}
	return false
}

func isImplicitDirectHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isLocalHostName(host string) bool {
	return net.ParseIP(host) == nil && !strings.Contains(host, ".")
}

func matchBypassRule(host string, rule string) bool {
	pattern := strings.ToLower(strings.TrimSpace(rule))
	if pattern == "" {
		return false
	}
	if strings.ContainsAny(pattern, "*?") {
		ok, err := path.Match(pattern, host)
		return err == nil && ok
	}

	trimmedPattern := strings.TrimPrefix(pattern, ".")
	return host == trimmedPattern || strings.HasSuffix(host, "."+trimmedPattern)
}

func splitProxyEntries(raw string) []string {
	return strings.FieldsFunc(raw, func(r rune) bool {
		return r == ';' || r == ','
	})
}

func proxySchemeForManualEntry(label string) string {
	switch label {
	case "socks", "socks4", "socks5":
		return "socks5"
	default:
		return "http"
	}
}

func parseProxyURL(raw string, schemeHint string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	if strings.Contains(trimmed, "://") {
		return url.Parse(trimmed)
	}
	scheme := strings.TrimSpace(schemeHint)
	if scheme == "" {
		scheme = "http"
	}
	return url.Parse(scheme + "://" + trimmed)
}

func parseAutoProxyURL(raw string) (*url.URL, error) {
	for _, entry := range splitProxyEntries(raw) {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" || strings.EqualFold(trimmed, "DIRECT") {
			continue
		}
		fields := strings.Fields(trimmed)
		switch len(fields) {
		case 1:
			return parseProxyURL(fields[0], "http")
		default:
			scheme := "http"
			switch strings.ToUpper(strings.TrimSpace(fields[0])) {
			case "PROXY", "HTTP":
				scheme = "http"
			case "HTTPS":
				scheme = "https"
			case "SOCKS", "SOCKS4", "SOCKS5":
				scheme = "socks5"
			default:
				return parseProxyURL(trimmed, "http")
			}
			return parseProxyURL(fields[1], scheme)
		}
	}
	return nil, nil
}

type softTimeoutResult[T any] struct {
	value    T
	resolved bool
	err      error
}

func runWithSoftTimeout[T any](timeout time.Duration, fn func() (T, bool, error), onFinish func(error), onTimeout func()) (T, bool, error, bool) {
	var zero T
	if fn == nil {
		return zero, false, nil, false
	}
	if timeout <= 0 {
		value, resolved, err := fn()
		if onFinish != nil {
			onFinish(err)
		}
		return value, resolved, err, false
	}

	resultCh := make(chan softTimeoutResult[T], 1)
	go func() {
		value, resolved, err := fn()
		if onFinish != nil {
			onFinish(err)
		}
		resultCh <- softTimeoutResult[T]{
			value:    value,
			resolved: resolved,
			err:      err,
		}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case result := <-resultCh:
		return result.value, result.resolved, result.err, false
	case <-timer.C:
		if onTimeout != nil {
			onTimeout()
		}
		return zero, false, nil, true
	}
}
