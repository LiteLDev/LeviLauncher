//go:build darwin || linux

package httpx

import "testing"

func TestMacProxySettings(t *testing.T) {
	settings := map[string]string{"HTTPEnable": "1", "HTTPProxy": "127.0.0.1", "HTTPPort": "7897", "HTTPSEnable": "1", "HTTPSProxy": "::1", "HTTPSPort": "7898"}
	if p := proxyFromMacSettings("http", settings); p == nil || p.String() != "http://127.0.0.1:7897" {
		t.Fatalf("HTTP proxy: %v", p)
	}
	if p := proxyFromMacSettings("https", settings); p == nil || p.String() != "http://[::1]:7898" {
		t.Fatalf("HTTPS proxy: %v", p)
	}
	settings["HTTPSPort"] = "invalid"
	if p := proxyFromMacSettings("https", settings); p != nil {
		t.Fatalf("invalid port accepted: %v", p)
	}
	settings["HTTPEnable"] = "0"
	if p := proxyFromMacSettings("http", settings); p != nil {
		t.Fatalf("disabled proxy accepted: %v", p)
	}
}
