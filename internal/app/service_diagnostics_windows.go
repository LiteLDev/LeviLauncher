package app

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/diagnose"
	"github.com/liteldev/LeviLauncher/internal/httpx"
	"github.com/liteldev/LeviLauncher/internal/mcservice"
	"github.com/liteldev/LeviLauncher/internal/processinfo"
	"github.com/liteldev/LeviLauncher/internal/registry"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/liteldev/LeviLauncher/internal/webview2runtime"
)

func configureDiagnostics(env *diagnose.Env) {
	env.GamingServices = func() bool {
		_, err := registry.GetAppxInfo("Microsoft.GamingServices")
		return err == nil
	}
	env.Service = serviceInfo
	env.DevMode = registry.IsDevModeEnabled
	env.WebView2 = webview2runtime.IsInstalled
	env.Elevated = processinfo.IsElevated
	env.FreeBytes = freeBytes
	env.BedrockDataDirs = func() []string {
		root := apppath.AppData()
		return []string{filepath.Join(root, "Minecraft Bedrock"), filepath.Join(root, "Minecraft Bedrock Preview")}
	}
	env.IsLink = isLinkOrJunction
	env.Registered = func(channel string) string {
		release, preview := mcservice.RegisteredPackageLocations()
		if channel == "preview" {
			return preview
		}
		return release
	}
	env.SamePath = func(a, b string) bool {
		ca, cb := utils.CanonicalWindowsPath(a), utils.CanonicalWindowsPath(b)
		return ca != "" && strings.EqualFold(ca, cb)
	}
	env.Proxy = func() (diagnose.ProxyInfo, error) {
		ps, err := httpx.SystemProxySettings()
		info := diagnose.ProxyInfo{Env: ps.Env, AutoDetect: ps.AutoDetect, PAC: ps.AutoConfigURL}
		if u := ps.ManualProxyFor("https://github.com"); u != nil {
			info.Manual = u.Host
		}
		return info, err
	}
}

func freeBytes(path string) (uint64, bool) {
	p := existingDiagnosticPath(path)
	if p == "" {
		return 0, false
	}
	ptr, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return 0, false
	}
	var free, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(ptr, &free, &total, &totalFree); err != nil {
		return 0, false
	}
	return free, true
}

func isLinkOrJunction(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil {
		return false
	}
	if fi.Mode()&os.ModeSymlink != 0 || fi.Mode()&os.ModeIrregular != 0 {
		return true
	}
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	attrs, err := windows.GetFileAttributes(ptr)
	return err == nil && attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}

// serviceInfo reads a service's state with query-only rights, which do not
// need administrator approval.
func serviceInfo(name string) diagnose.ServiceInfo {
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return diagnose.ServiceInfo{}
	}
	defer windows.CloseServiceHandle(scm)
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return diagnose.ServiceInfo{}
	}
	h, err := windows.OpenService(scm, namePtr, windows.SERVICE_QUERY_STATUS|windows.SERVICE_QUERY_CONFIG)
	if err != nil {
		return diagnose.ServiceInfo{}
	}
	svc := &mgr.Service{Name: name, Handle: h}
	defer svc.Close()
	info := diagnose.ServiceInfo{Exists: true}
	if st, err := svc.Query(); err == nil {
		info.Running = st.State == 4 // SERVICE_RUNNING
	}
	if cfg, err := svc.Config(); err == nil {
		switch cfg.StartType {
		case windows.SERVICE_AUTO_START, windows.SERVICE_BOOT_START, windows.SERVICE_SYSTEM_START:
			info.StartType = "auto"
		case windows.SERVICE_DEMAND_START:
			info.StartType = "manual"
		case windows.SERVICE_DISABLED:
			info.StartType = "disabled"
		}
	}
	return info
}
