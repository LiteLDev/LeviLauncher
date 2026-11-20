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

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/diagnose"
	"github.com/liteldev/LeviLauncher/internal/gameinput"
	"github.com/liteldev/LeviLauncher/internal/httpx"
	"github.com/liteldev/LeviLauncher/internal/lip"
	"github.com/liteldev/LeviLauncher/internal/mcservice"
	"github.com/liteldev/LeviLauncher/internal/nativeinstall"
	"github.com/liteldev/LeviLauncher/internal/registry"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/liteldev/LeviLauncher/internal/uwp"
	"github.com/liteldev/LeviLauncher/internal/vcruntime"
	"github.com/liteldev/LeviLauncher/internal/versions"
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
	env := &diagnose.Env{
		Instance:  diagnoseInstance,
		VCRuntime: vcruntime.IsInstalled,
		GameInput: gameinput.IsInstalled,
		DataRoot:  apppath.BaseRoot,
		CanWrite:  utils.CanWriteDir,
		RunningExes: func() []string {
			var out []string
			for _, p := range mcservice.ListMinecraftProcesses() {
				if !p.IsLauncher && p.ExePath != "" {
					out = append(out, p.ExePath)
				}
			}
			return out
		},
		XUID: func() (string, bool) {
			id, err := xbox.GetLocalUserId()
			if err != nil || id == 0 {
				return "", xbox.IsUnreachable(err)
			}
			return strconv.FormatUint(id, 10), false
		},
		Licenses: func(ctx context.Context, xuid string) (string, string) {
			l := nativeinstall.RefreshGameLicenses(ctx, apppath.AccountsDir(), xuid)
			return l.Release, l.Preview
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
	configureDiagnostics(env)
	return env
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

func existingDiagnosticPath(path string) string {
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
	return p
}
