//go:build darwin || linux

package httpx

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var systemProxyOnce sync.Once
var unixSystemProxy map[string]string

func newBaseTransport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = proxyFromUnixSystem
	return t
}
func newBlockingBaseTransport() *http.Transport { return newBaseTransport() }

func proxyFromUnixSystem(req *http.Request) (*url.URL, error) {
	host := req.URL.Hostname()
	if host == "localhost" || net.ParseIP(host).IsLoopback() {
		return nil, nil
	}
	if proxy, err := http.ProxyFromEnvironment(req); proxy != nil || err != nil {
		return proxy, err
	}
	systemProxyOnce.Do(func() {
		unixSystemProxy = map[string]string{}
		if runtime.GOOS != "darwin" {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		output, err := exec.CommandContext(ctx, "/usr/sbin/scutil", "--proxy").Output()
		if err != nil {
			return
		}
		for _, line := range strings.Split(string(output), "\n") {
			key, value, ok := strings.Cut(strings.TrimSpace(line), " : ")
			if ok {
				unixSystemProxy[key] = strings.TrimSpace(value)
			}
		}
	})
	return proxyFromMacSettings(req.URL.Scheme, unixSystemProxy), nil
}
func proxyFromMacSettings(scheme string, settings map[string]string) *url.URL {
	name := "HTTP"
	if scheme == "https" {
		name = "HTTPS"
	}
	if settings[name+"Enable"] != "1" {
		return nil
	}
	host := settings[name+"Proxy"]
	port, err := strconv.Atoi(settings[name+"Port"])
	if host == "" || err != nil || port < 1 || port > 65535 {
		return nil
	}
	return &url.URL{Scheme: "http", Host: net.JoinHostPort(host, strconv.Itoa(port))}
}
