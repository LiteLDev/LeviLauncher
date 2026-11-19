// Package diagnose runs the quick checks behind the GUI's "Run diagnostics"
// buttons: given what failed (a topic) and optionally the instance it failed
// for, it looks at the runtimes, Windows settings, network paths, account
// licenses and instance files that the failure may come from.
//
// Checks report machine-readable results only (an id, a status and named
// parameters); the frontend turns them into localized explanations and fix
// actions. A run is bounded (RunTimeout) and its checks run concurrently, so
// a diagnosis stays well under a minute even when every endpoint times out.
package diagnose

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

// Statuses of a check, from best to worst. "info" carries a fact that is
// neither good nor bad; "skip" means the check did not apply or could not run
// because an earlier one failed.
const (
	StatusOK   = "ok"
	StatusInfo = "info"
	StatusSkip = "skip"
	StatusWarn = "warn"
	StatusFail = "fail"
)

// Topics: what the user was doing when the failure happened.
const (
	TopicLaunch   = "launch"   // starting an instance
	TopicInstall  = "install"  // downloading and installing a game version
	TopicDownload = "download" // downloading files (game packages, updates)
	TopicLIP      = "lip"      // installing LeviLamina or mods
	TopicLicense  = "license"  // account ownership and licenses
	TopicUWP      = "uwp"      // registering / launching UWP instances
	TopicFull     = "full"     // everything that does not need an instance
)

// RunTimeout bounds a whole run; individual network probes use ProbeTimeout.
const (
	RunTimeout     = 50 * time.Second
	ProbeTimeout   = 8 * time.Second
	LicenseTimeout = 40 * time.Second
)

// Check is one result line.
type Check struct {
	ID     string            `json:"id"`
	Status string            `json:"status"`
	Params map[string]string `json:"params,omitempty"`
	// Detail is the raw technical reason (an error message), untranslated.
	Detail     string `json:"detail,omitempty"`
	DurationMs int64  `json:"durationMs"`
}

// Report is the result of one run.
type Report struct {
	Topic      string  `json:"topic"`
	Instance   string  `json:"instance,omitempty"`
	Checks     []Check `json:"checks"`
	DurationMs int64   `json:"durationMs"`
}

// Request selects what to check.
type Request struct {
	Topic    string
	Instance string // instance folder name, "" for none
	// Channel narrows license checks for install failures: "release",
	// "preview" or "" for both.
	Channel string
}

// checkFunc fills in one check. It must honor ctx.
type checkFunc func(ctx context.Context, env *Env, req Request, inst *Instance) Check

type plannedCheck struct {
	id  string
	run checkFunc
}

// plan returns the checks of a topic in display order.
func plan(req Request, inst *Instance, skipCheck func(string) bool) []plannedCheck {
	var ids []string
	hasInst := inst != nil
	switch req.Topic {
	case TopicLaunch:
		ids = []string{"instance_files", "vcruntime", "gameinput", "gaming_services", "xbox_services", "developer_mode", "registration", "instance_running", "instance_path", "appdata_link", "disk_space"}
	case TopicUWP:
		ids = []string{"developer_mode", "registration", "instance_files", "instance_running", "instance_path"}
	case TopicInstall:
		ids = []string{"account", "license", "vcruntime", "data_root", "disk_space", "proxy", "clock", "ep_microsoft_auth", "ep_xbox_store", "ep_xbox_packages", "ep_version_db"}
	case TopicDownload:
		ids = []string{"disk_space", "data_root", "proxy", "mirror", "clock", "ep_xbox_packages", "ep_version_db", "ep_github", "ep_github_proxy"}
	case TopicLIP:
		ids = []string{"proxy", "mirror", "lip_proxy", "clock", "ep_github", "ep_github_proxy", "ep_levimc", "ep_ll_db", "ll_support", "instance_path", "instance_running"}
	case TopicLicense:
		ids = []string{"account", "license", "elevation", "proxy", "clock", "ep_microsoft_auth", "ep_xbox_store"}
	default: // TopicFull
		ids = []string{"vcruntime", "gameinput", "gaming_services", "xbox_services", "developer_mode", "webview2", "data_root", "disk_space", "appdata_link", "elevation", "account", "license", "proxy", "mirror", "lip_proxy", "clock", "ep_microsoft_auth", "ep_xbox_store", "ep_xbox_packages", "ep_version_db", "ep_github", "ep_github_proxy", "ep_levimc"}
	}
	out := make([]plannedCheck, 0, len(ids))
	for _, id := range ids {
		if (instanceOnly[id] && !hasInst) || (skipCheck != nil && skipCheck(id)) {
			continue
		}
		if fn, ok := registry[id]; ok {
			out = append(out, plannedCheck{id: id, run: fn})
		}
	}
	return out
}

// instanceOnly checks are dropped when no instance was given.
var instanceOnly = map[string]bool{
	"instance_files": true, "registration": true, "instance_running": true,
	"instance_path": true, "ll_support": true,
}

// Run executes the checks of req concurrently and returns them in plan
// order. It never takes longer than RunTimeout (plus a short grace period
// for checks to notice cancellation); a check still running at the deadline
// is reported as a failure with the "timeout" reason.
func Run(ctx context.Context, env *Env, req Request) Report {
	req.Topic = strings.ToLower(strings.TrimSpace(req.Topic))
	req.Instance = strings.TrimSpace(req.Instance)
	req.Channel = strings.ToLower(strings.TrimSpace(req.Channel))
	start := env.now()
	ctx, cancel := context.WithTimeout(ctx, RunTimeout)
	defer cancel()

	var inst *Instance
	if req.Instance != "" {
		if i, ok := env.Instance(req.Instance); ok {
			inst = &i
			if req.Channel == "" {
				req.Channel = i.Channel
			}
		}
	}
	planned := plan(req, inst, env.SkipCheck)
	results := make([]Check, len(planned))
	done := make([]bool, len(planned))
	var mu sync.Mutex
	var wg sync.WaitGroup
	shared := newSharedState(env)
	runEnv := *env
	runEnv.shared = shared
	for i, pc := range planned {
		wg.Add(1)
		go func(i int, pc plannedCheck) {
			defer wg.Done()
			began := env.now()
			c := pc.run(ctx, &runEnv, req, inst)
			if c.ID == "" {
				c.ID = pc.id
			}
			c.DurationMs = env.now().Sub(began).Milliseconds()
			mu.Lock()
			results[i], done[i] = c, true
			mu.Unlock()
		}(i, pc)
	}
	finished := make(chan struct{})
	go func() { wg.Wait(); close(finished) }()
	select {
	case <-finished:
	case <-ctx.Done():
		// Give checks a moment to return their own cancellation result.
		select {
		case <-finished:
		case <-time.After(1500 * time.Millisecond):
		}
	}
	mu.Lock()
	out := make([]Check, len(planned))
	for i, pc := range planned {
		if done[i] {
			out[i] = results[i]
		} else {
			out[i] = Check{ID: pc.id, Status: StatusFail, Params: map[string]string{"reason": "timeout"}}
		}
	}
	mu.Unlock()
	rep := Report{Topic: req.Topic, Instance: req.Instance, Checks: out}
	if rep.Topic == "" {
		rep.Topic = TopicFull
	}
	rep.DurationMs = env.now().Sub(start).Milliseconds()
	return rep
}

// Summary counts statuses; used by tests and logging.
func Summary(r Report) map[string]int {
	m := map[string]int{}
	for _, c := range r.Checks {
		m[c.Status]++
	}
	return m
}

// worst returns the most severe of the given statuses.
func worst(statuses ...string) string {
	rank := map[string]int{StatusOK: 0, StatusInfo: 1, StatusSkip: 2, StatusWarn: 3, StatusFail: 4}
	sort.SliceStable(statuses, func(i, j int) bool { return rank[statuses[i]] > rank[statuses[j]] })
	if len(statuses) == 0 {
		return StatusOK
	}
	return statuses[0]
}
