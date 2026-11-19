package app

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/diagnose"
	"github.com/liteldev/LeviLauncher/internal/gameinput"
	"github.com/liteldev/LeviLauncher/internal/httpx"
	"github.com/liteldev/LeviLauncher/internal/lip"
	"github.com/liteldev/LeviLauncher/internal/mcservice"
	"github.com/liteldev/LeviLauncher/internal/nativeinstall"
	"github.com/liteldev/LeviLauncher/internal/processinfo"
	"github.com/liteldev/LeviLauncher/internal/registry"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/liteldev/LeviLauncher/internal/uwp"
	"github.com/liteldev/LeviLauncher/internal/vcruntime"
	"github.com/liteldev/LeviLauncher/internal/versions"
	"github.com/liteldev/LeviLauncher/internal/webview2runtime"
	"github.com/liteldev/LeviLauncher/internal/xbox"
)

// DiagnosticsService runs the quick checks behind "Run diagnostics".
type DiagnosticsService struct{}

func NewDiagnosticsService() *DiagnosticsService { return &DiagnosticsService{} }

// Run checks what the failure of topic may come from; instance is the
// instance folder name or "", channel narrows license checks ("release",
// "preview" or ""). It returns within about a minute.
func (s *DiagnosticsService) Run(ctx context.Context, topic string, instance string, channel string) diagnose.Report {
	rep := diagnose.Run(ctx, newDiagnoseEnv(), diagnose.Request{Topic: topic, Instance: instance, Channel: channel})
	log.Printf("diagnostics: topic=%s instance=%q took %dms: %v", rep.Topic, rep.Instance, rep.DurationMs, diagnose.Summary(rep))
	for _, c := range rep.Checks {
		if c.Status == diagnose.StatusFail || c.Status == diagnose.StatusWarn {
			log.Printf("diagnostics: %s %s %v %s", c.Status, c.ID, c.Params, c.Detail)
		}
	}
	return rep
}

// IsDeveloperModeEnabled lets the UI check Developer Mode before a UWP
// registration fails on it.
func (s *DiagnosticsService) IsDeveloperModeEnabled() bool { return registry.IsDevModeEnabled() }

// EnableDeveloperMode asks for administrator approval and turns Developer
// Mode on; false when it was declined or failed.
func (s *DiagnosticsService) EnableDeveloperMode() bool { return registry.TryEnableDevMode() }

func newDiagnoseEnv() *diagnose.Env {
	client := &http.Client{Transport: httpx.NewBlockingProxyTransport()}
	return &diagnose.Env{
		Instance:       diagnoseInstance,
		VCRuntime:      vcruntime.IsInstalled,
		GameInput:      gameinput.IsInstalled,
		GamingServices: func() bool { _, err := registry.GetAppxInfo("Microsoft.GamingServices"); return err == nil },
		Service:        serviceInfo,
		DevMode:        registry.IsDevModeEnabled,
		WebView2:       webview2runtime.IsInstalled,
		Elevated:       processinfo.IsElevated,
		DataRoot:       apppath.BaseRoot,
		CanWrite:       utils.CanWriteDir,
		FreeBytes:      freeBytes,
		BedrockDataDirs: func() []string {
			root := apppath.AppData()
			return []string{filepath.Join(root, "Minecraft Bedrock"), filepath.Join(root, "Minecraft Bedrock Preview")}
		},
		IsLink: isLinkOrJunction,
		RunningExes: func() []string {
			var out []string
			for _, p := range mcservice.ListMinecraftProcesses() {
				if !p.IsLauncher && p.ExePath != "" {
					out = append(out, p.ExePath)
				}
			}
			return out
		},
		Registered: func(channel string) string {
			release, preview := mcservice.RegisteredPackageLocations()
			if channel == "preview" {
				return preview
			}
			return release
		},
		SamePath: func(a, b string) bool {
			ca, cb := utils.CanonicalWindowsPath(a), utils.CanonicalWindowsPath(b)
			return ca != "" && strings.EqualFold(ca, cb)
		},
		XUID: func() string {
			id, err := xbox.GetLocalUserId()
			if err != nil || id == 0 {
				return ""
			}
			return strconv.FormatUint(id, 10)
		},
		Licenses: func(ctx context.Context, xuid string) (string, string) {
			l := nativeinstall.RefreshGameLicenses(ctx, apppath.AccountsDir(), xuid)
			return l.Release, l.Preview
		},
		Proxy: func() (diagnose.ProxyInfo, error) {
			ps, err := httpx.SystemProxySettings()
			info := diagnose.ProxyInfo{Env: ps.Env, AutoDetect: ps.AutoDetect, PAC: ps.AutoConfigURL}
			if u := ps.ManualProxyFor("https://github.com"); u != nil {
				info.Manual = u.Host
			}
			return info, err
		},
		Dial: func(ctx context.Context, address string) error {
			var d net.Dialer
			c, err := d.DialContext(ctx, "tcp", address)
			if err == nil {
				c.Close()
			}
			return err
		},
		Get: func(ctx context.Context, url string) diagnose.ProbeResult {
			return probe(ctx, client, url)
		},
		Mirror:            utils.IsMirrorEnabled,
		GithubProxy:       lip.EffectiveGithubProxy,
		GithubProxyCustom: func() bool { return strings.TrimSpace(config.GetLipGithubProxyOverride()) != "" },
		LLVersions: func(ctx context.Context) (map[string][]string, error) {
			type result struct {
				db  map[string][]string
				err error
			}
			ch := make(chan result, 1)
			go func() { db, err := mcservice.FetchLeviLaminaVersionDB(); ch <- result{db, err} }()
			select {
			case r := <-ch:
				return r.db, r.err
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}
}

func diagnoseInstance(name string) (diagnose.Instance, bool) {
	root, err := apppath.InstancesDir()
	if err != nil || strings.TrimSpace(name) == "" {
		return diagnose.Instance{}, false
	}
	dir := filepath.Join(root, strings.TrimSpace(name))
	meta, err := versions.ReadMeta(dir)
	if err != nil {
		if _, statErr := os.Stat(dir); statErr != nil {
			// Report the missing folder through instance_files.
			return diagnose.Instance{Name: name, Dir: dir, Channel: "release", PackageType: versions.PackageTypeGDK}, true
		}
	}
	channel := "release"
	if strings.EqualFold(strings.TrimSpace(meta.Type), "preview") {
		channel = "preview"
	}
	inst := diagnose.Instance{
		Name:        name,
		Dir:         dir,
		GameVersion: strings.TrimSpace(meta.GameVersion),
		Channel:     channel,
		PackageType: versions.DetectPackageType(dir, meta),
		Isolation:   meta.EnableIsolation,
	}
	if inst.PackageType == versions.PackageTypeUWP {
		if m, err := uwp.ReadManifest(dir); err == nil && len(m.Applications) > 0 {
			inst.Executable = m.Applications[0].Executable
		}
	}
	return inst, true
}

func probe(ctx context.Context, client *http.Client, url string) diagnose.ProbeResult {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return diagnose.ProbeResult{Err: err}
	}
	resp, err := client.Do(req)
	res := diagnose.ProbeResult{Latency: time.Since(start), Err: err}
	if err != nil {
		return res
	}
	defer resp.Body.Close()
	_, _ = io.CopyN(io.Discard, resp.Body, 64<<10)
	res.Status = resp.StatusCode
	if d, err := http.ParseTime(resp.Header.Get("Date")); err == nil {
		res.Date = d
	}
	return res
}

func freeBytes(path string) (uint64, bool) {
	p := strings.TrimSpace(path)
	for p != "" {
		if _, err := os.Stat(p); err == nil {
			break
		}
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		p = parent
	}
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
