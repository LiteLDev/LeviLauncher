package app

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/liteldev/LeviLauncher/internal/diagnose"
	"github.com/liteldev/LeviLauncher/internal/httpx"
	"github.com/liteldev/LeviLauncher/internal/mcservice"
)

func configureDiagnostics(env *diagnose.Env) {
	env.SkipCheck = func(id string) bool {
		switch id {
		case "gaming_services", "xbox_services", "developer_mode", "webview2", "registration", "appdata_link", "elevation":
			return true
		default:
			return false
		}
	}
	env.FreeBytes = func(path string) (uint64, bool) {
		stats := mcservice.GetDriveStats(existingDiagnosticPath(path))
		return stats["free"], stats["total"] != 0
	}
	env.SamePath = func(a, b string) bool {
		if filepath.Clean(a) == filepath.Clean(b) {
			return a != "" && b != ""
		}
		left, leftErr := os.Stat(a)
		right, rightErr := os.Stat(b)
		return leftErr == nil && rightErr == nil && os.SameFile(left, right)
	}
	env.Proxy = func() (diagnose.ProxyInfo, error) {
		req, err := http.NewRequest(http.MethodGet, "https://github.com", nil)
		if err != nil {
			return diagnose.ProxyInfo{}, err
		}
		proxy, err := httpx.ProxyForRequest(req)
		if err != nil || proxy == nil {
			return diagnose.ProxyInfo{}, err
		}
		if httpx.UsesSystemProxy(req) {
			return diagnose.ProxyInfo{Manual: proxy.Host}, nil
		}
		return diagnose.ProxyInfo{Env: proxy.Host}, nil
	}
}
