package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/lip"
	"github.com/liteldev/LeviLauncher/internal/mcservice"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

// Sources of a setting's value, as `config list` and `config get --json`
// report them.
const (
	cfgSourceSet     = "set"     // saved in the launcher settings
	cfgSourceDefault = "default" // the built-in default
	cfgSourceAuto    = "auto"    // detected (mirror: region; GitHub proxy: mirrors on)
	cfgSourceLip     = "lip"     // github_proxy of lip's own config.toml
	cfgSourcePath    = "path"    // a read-only folder or file of the launcher
)

// cfgEntry is one setting in `config list --json` and `config get --json`.
type cfgEntry struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Source   string `json:"source"`
	ReadOnly bool   `json:"readOnly"`
}

// cfgChange is the result of `config set` and `config unset`.
type cfgChange struct {
	Key      string `json:"key"`
	Previous string `json:"previous"`
	Value    string `json:"value"`
	Source   string `json:"source"`
	Changed  bool   `json:"changed"`
}

// cfgKey describes one setting: how to read it, change it and reset it.
type cfgKey struct {
	name    string
	summary string
	values  string // allowed values, for help and errors
	// get returns the value in effect and its source.
	get func() (value, source string)
	// set validates and saves raw; it returns the value as stored. nil for a
	// read-only key.
	set func(c *Ctx, raw string) error
	// unset returns to the default; nil when the key cannot be unset.
	unset func(c *Ctx) error
}

func (k *cfgKey) readOnly() bool { return k.set == nil }

// Seams replaced by tests.
var (
	cfgGUIRunning  = GUIRunning
	cfgCanWriteDir = utils.CanWriteDir
	// cfgEffectiveProxy reads lip's own config.toml too.
	cfgEffectiveProxy = lip.EffectiveGithubProxy
)

// cfgKeys lists the settings in display order.
var cfgKeys = []*cfgKey{
	{
		name: "data-root", values: "<folder>",
		summary: "where instances, downloads and backups live (existing data is not moved)",
		get: func() (string, string) {
			if strings.TrimSpace(cfgLoad().BaseRoot) != "" {
				return apppath.BaseRoot(), cfgSourceSet
			}
			return apppath.BaseRoot(), cfgSourceDefault
		},
		set:   cfgSetDataRoot,
		unset: cfgUnsetDataRoot,
	},
	{
		name: "default-instance", values: "<instance>",
		summary: "the instance commands use when none is named (same as 'use')",
		get: func() (string, string) {
			if v := config.GetCurrentInstance(); v != "" {
				return v, cfgSourceSet
			}
			return "", cfgSourceDefault
		},
		set: func(c *Ctx, raw string) error {
			inst, err := LookupInstance(raw)
			if err != nil {
				return err
			}
			return cfgWrite(config.SetCurrentInstance(inst.Name))
		},
		unset: func(c *Ctx) error { return cfgWrite(config.SetCurrentInstance("")) },
	},
	{
		name: "mirror", values: "on|off",
		summary: "download from China-friendly mirrors (unset: detect by region)",
		get: func() (string, string) {
			if _, ok := config.GetMirrorSourceOverride(); ok {
				return onOff(utils.IsMirrorEnabled()), cfgSourceSet
			}
			return onOff(utils.IsMirrorEnabled()), cfgSourceAuto
		},
		set: func(c *Ctx, raw string) error {
			on, err := parseOnOff("mirror", raw)
			if err != nil {
				return err
			}
			return backendResult(mcservice.SetMirrorEnabled(on))
		},
		unset: func(c *Ctx) error {
			return cfgWrite(config.Update(func(a *config.AppConfig) { a.MirrorSourceEnabled = nil }))
		},
	},
	{
		name: "github-proxy", values: "<http(s) url>",
		summary: "GitHub proxy for mod downloads (used while mirrors are on unless set)",
		get: func() (string, string) {
			eff := cfgEffectiveProxy()
			switch {
			case config.GetLipGithubProxyOverride() != "":
				return eff, cfgSourceSet
			case eff == "":
				return "", cfgSourceDefault
			case utils.IsMirrorEnabled() && cfgIsDefaultProxy(eff):
				return eff, cfgSourceAuto
			}
			return eff, cfgSourceLip
		},
		set: func(c *Ctx, raw string) error {
			if strings.TrimSpace(raw) == "" {
				return UsageError("the GitHub proxy cannot be empty").
					WithHint("%s config unset github-proxy", ProgramName)
			}
			return backendResult(lip.SetGithubProxy(raw))
		},
		unset: func(c *Ctx) error {
			return cfgWrite(config.Update(func(a *config.AppConfig) { a.LipGithubProxy = "" }))
		},
	},
	{
		name: "beta-updates", values: "on|off",
		summary: "offer pre-release launcher updates",
		get: func() (string, string) {
			return cfgBoolValue(cfgLoad().EnableBetaUpdates, false)
		},
		set: func(c *Ctx, raw string) error {
			on, err := parseOnOff("beta-updates", raw)
			if err != nil {
				return err
			}
			return backendResult(mcservice.SetEnableBetaUpdates(on))
		},
		unset: func(c *Ctx) error { return backendResult(mcservice.SetEnableBetaUpdates(false)) },
	},
	{
		name: "discord", values: "on|off",
		summary: "show Discord Rich Presence while playing (launcher window only)",
		get: func() (string, string) {
			return cfgBoolValue(!cfgLoad().DisableDiscordRPC, true)
		},
		// Only the setting is saved: this process opens no Discord
		// connection (mcservice.SetDisableDiscordRPC would).
		set: func(c *Ctx, raw string) error {
			on, err := parseOnOff("discord", raw)
			if err != nil {
				return err
			}
			return cfgWrite(config.Update(func(a *config.AppConfig) { a.DisableDiscordRPC = !on }))
		},
		unset: func(c *Ctx) error {
			return cfgWrite(config.Update(func(a *config.AppConfig) { a.DisableDiscordRPC = false }))
		},
	},
	{
		name: "on-game-launch", values: "minimize|hide|close|keep",
		summary: "what the launcher window does when the game starts",
		get: func() (string, string) {
			return cfgEnumValue(cfgLoad().OnGameLaunch, config.GetOnGameLaunch())
		},
		set: func(c *Ctx, raw string) error {
			v, err := parseEnumValue("on-game-launch", raw, []string{
				config.OnGameLaunchMinimize, config.OnGameLaunchHide, config.OnGameLaunchClose, config.OnGameLaunchKeep})
			if err != nil {
				return err
			}
			return backendResult(mcservice.SetGameLaunchBehavior(v))
		},
		unset: func(c *Ctx) error {
			return cfgWrite(config.Update(func(a *config.AppConfig) { a.OnGameLaunch = "" }))
		},
	},
	{
		name: "on-game-exit", values: "reopen|keep|close",
		summary: "what the launcher window does when the game exits",
		get: func() (string, string) {
			return cfgEnumValue(cfgLoad().OnGameExit, config.GetOnGameExit())
		},
		set: func(c *Ctx, raw string) error {
			v, err := parseEnumValue("on-game-exit", raw, []string{
				config.OnGameExitReopen, config.OnGameExitKeep, config.OnGameExitClose})
			if err != nil {
				return err
			}
			return backendResult(mcservice.SetGameExitBehavior(v))
		},
		unset: func(c *Ctx) error {
			return cfgWrite(config.Update(func(a *config.AppConfig) { a.OnGameExit = "" }))
		},
	},
	{
		name: "close-to-tray", values: "on|off",
		summary: "closing the launcher window keeps it in the tray",
		get: func() (string, string) {
			return cfgBoolValue(cfgLoad().MinimizeToTray, false)
		},
		set: func(c *Ctx, raw string) error {
			on, err := parseOnOff("close-to-tray", raw)
			if err != nil {
				return err
			}
			return backendResult(mcservice.SetMinimizeToTray(on))
		},
		unset: func(c *Ctx) error { return backendResult(mcservice.SetMinimizeToTray(false)) },
	},
	// Read-only paths.
	{name: "instances-dir", summary: "folder of the installed instances", get: cfgPath(func() string {
		d, _ := apppath.InstancesDir()
		return d
	})},
	{name: "installers-dir", summary: "folder of downloaded game installers", get: cfgPath(func() string {
		d, _ := apppath.InstallersDir()
		return d
	})},
	{name: "backups-dir", summary: "folder of instance and world backups", get: cfgPath(apppath.BackupsDir)},
	{name: "cache-dir", summary: "folder of caches ('cache clean' empties parts of it)", get: cfgPath(apppath.CacheDir)},
	{name: "logs-dir", summary: "folder of the launcher logs", get: cfgPath(apppath.LogsDir)},
	{name: "config-file", summary: "the launcher settings file", get: cfgPath(apppath.ConfigPath)},
}

func cfgPath(f func() string) func() (string, string) {
	return func() (string, string) { return f(), cfgSourcePath }
}

func cfgLoad() config.AppConfig {
	c, _ := config.Load()
	return c
}

// cfgDefaultGithubProxy is lip's built-in proxy, used while mirrors are on
// and no proxy is configured (internal/lip defaultLipGithubProxy).
const cfgDefaultGithubProxy = "https://gh-proxy.org/https://github.com"

// cfgIsDefaultProxy reports whether the effective proxy is the built-in
// default rather than one from lip's config.toml.
func cfgIsDefaultProxy(effective string) bool {
	return strings.EqualFold(strings.TrimRight(effective, "/"), cfgDefaultGithubProxy)
}

func cfgBoolValue(v, def bool) (string, string) {
	if v == def {
		return onOff(v), cfgSourceDefault
	}
	return onOff(v), cfgSourceSet
}

func cfgEnumValue(saved, effective string) (string, string) {
	if strings.TrimSpace(saved) == "" {
		return effective, cfgSourceDefault
	}
	return effective, cfgSourceSet
}

// backendResult converts a backend result string into an error, nil for
// success (a nil *Error must not become a non-nil error interface).
func backendResult(result string) error {
	if e := Backend(result); e != nil {
		return e
	}
	return nil
}

func cfgWrite(err error) error {
	if err == nil {
		return nil
	}
	if e := FromError(err); e.Code != "ERR_FAILED" {
		return e
	}
	return Errorf(ExitFailure, "ERR_CONFIG_WRITE_FAILED", "cannot save the launcher settings: %v", err)
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// parseOnOff accepts on/off and the usual boolean spellings.
func parseOnOff(key, raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "on", "true", "yes", "1", "enable", "enabled":
		return true, nil
	case "off", "false", "no", "0", "disable", "disabled":
		return false, nil
	}
	return false, UsageError("invalid value %q for %s: use on or off", raw, key).withCode("ERR_INVALID_PARAM")
}

func parseEnumValue(key, raw string, allowed []string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	for _, a := range allowed {
		if v == a {
			return v, nil
		}
	}
	return "", UsageError("invalid value %q for %s: use %s", raw, key, strings.Join(allowed, ", ")).withCode("ERR_INVALID_PARAM")
}

func cfgSetDataRoot(c *Ctx, raw string) error {
	r := strings.TrimSpace(raw)
	if r == "" {
		return UsageError("the data root cannot be empty").WithHint("%s config unset data-root", ProgramName)
	}
	abs, err := filepath.Abs(r)
	if err != nil {
		return UsageError("invalid folder %q: %v", raw, err).withCode("ERR_INVALID_PATH")
	}
	if !cfgCanWriteDir(abs) {
		return Errorf(ExitConflict, "ERR_WRITE_TARGET", "cannot write to %s", abs).
			WithHint("choose a folder you can write to")
	}
	if err := Backend(mcservice.SetBaseRoot(abs)); err != nil {
		return err
	}
	return nil
}

func cfgUnsetDataRoot(c *Ctx) error {
	cur := apppath.BaseRoot()
	if err := c.Confirm(fmt.Sprintf("Reset the data root to the default (data in %s is not moved)?", cur)); err != nil {
		return err
	}
	return backendResult(mcservice.ResetBaseRoot())
}

func init() {
	keyHelp := cfgKeyHelp()
	Register(&Command{
		Path:        "config list",
		Summary:     "list launcher settings and folders",
		Description: "SOURCE is set (saved), default, auto (detected) or lip (lip's own config).\n\n" + keyHelp,
		Examples:    []string{"config list", "config list --json"},
		Run:         runConfigList,
	})
	Register(&Command{
		Path:        "config get",
		Summary:     "print the value of a setting",
		Args:        "<key>",
		Description: keyHelp,
		Examples:    []string{"config get data-root", "config get mirror --json"},
		MinArgs:     1, MaxArgs: 1,
		Run: runConfigGet,
	})
	Register(&Command{
		Path:    "config set",
		Summary: "change a setting",
		Args:    "<key> <value>",
		Description: "Setting a key to the value it already has changes nothing. Changing data-root\n" +
			"only points the launcher at another folder: existing instances, downloads and\n" +
			"backups are not moved.\n\n" + keyHelp,
		Examples: []string{"config set mirror on", "config set on-game-launch hide", `config set data-root D:\Games\LeviLauncher`},
		MinArgs:  2, MaxArgs: 2,
		Run: runConfigSet,
	})
	Register(&Command{
		Path:    "config unset",
		Summary: "return a setting to its default",
		Args:    "<key>",
		Description: "mirror returns to detection by region; data-root asks first (--yes skips).\n\n" +
			keyHelp,
		Examples: []string{"config unset github-proxy", "config unset data-root --yes", "config unset default-instance"},
		MinArgs:  1, MaxArgs: 1,
		Run: runConfigUnset,
	})
	RegisterBackendErrorsFor([]string{"config"}, map[string]BackendError{
		"ERR_INVALID_PATH":      {Exit: ExitUsage, Message: "invalid folder"},
		"ERR_CREATE_TARGET_DIR": {Exit: ExitFailure, Message: "cannot create the folder"},
		"ERR_WRITE_FILE":        {Exit: ExitFailure, Message: "cannot save the launcher settings"},
		"ERR_INVALID_PARAM":     {Exit: ExitUsage, Message: "invalid value"},
		"ERR_LIP_INVALID_GITHUB_PROXY": {Exit: ExitUsage, Message: "the GitHub proxy must be an http or https URL with a host",
			Hint: ProgramName + " config set github-proxy https://example.com"},
	})
}

// cfgKeyHelp lists the keys for the commands' help.
func cfgKeyHelp() string {
	var b strings.Builder
	b.WriteString("Settings:\n")
	w := 0
	for _, k := range cfgKeys {
		if n := len(k.name) + len(k.values) + 1; n > w {
			w = n
		}
	}
	for _, k := range cfgKeys {
		if k.readOnly() {
			continue
		}
		fmt.Fprintf(&b, "  %-*s  %s\n", w, k.name+" "+k.values, k.summary)
	}
	b.WriteString("\nRead-only (shown by list and get):\n")
	for _, k := range cfgKeys {
		if k.readOnly() {
			fmt.Fprintf(&b, "  %-*s  %s\n", w, k.name, k.summary)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func lookupCfgKey(name string) (*cfgKey, error) {
	n := strings.ToLower(strings.TrimSpace(name))
	names := make([]string, 0, len(cfgKeys))
	for _, k := range cfgKeys {
		if k.name == n {
			return k, nil
		}
		names = append(names, k.name)
	}
	e := Errorf(ExitUsage, "ERR_UNKNOWN_SETTING", "unknown setting %q", name)
	if s := suggest(n, names); s != "" {
		return nil, e.WithHint("did you mean %q?", s)
	}
	return nil, e.WithHint("%s config list", ProgramName)
}

func (k *cfgKey) entry() cfgEntry {
	v, src := k.get()
	return cfgEntry{Key: k.name, Value: v, Source: src, ReadOnly: k.readOnly()}
}

func runConfigList(c *Ctx) error {
	entries := make([]cfgEntry, 0, len(cfgKeys))
	ids := make([]string, 0, len(cfgKeys))
	for _, k := range cfgKeys {
		entries = append(entries, k.entry())
		ids = append(ids, k.name)
	}
	return c.Render(entries, ids, func(w io.Writer) error {
		var rows, paths [][]string
		for _, e := range entries {
			if e.ReadOnly {
				paths = append(paths, []string{e.Key, Dash(e.Value)})
				continue
			}
			rows = append(rows, []string{e.Key, Dash(e.Value), e.Source})
		}
		if err := Table(w, []string{"KEY", "VALUE", "SOURCE"}, rows); err != nil {
			return err
		}
		fmt.Fprintln(w)
		return Table(w, []string{"PATH", "LOCATION"}, paths)
	})
}

func runConfigGet(c *Ctx) error {
	k, err := lookupCfgKey(c.Arg(0))
	if err != nil {
		return err
	}
	e := k.entry()
	return c.Render(e, []string{e.Value}, func(w io.Writer) error {
		_, err := fmt.Fprintln(w, e.Value)
		return err
	})
}

func runConfigSet(c *Ctx) error {
	k, err := lookupCfgKey(c.Arg(0))
	if err != nil {
		return err
	}
	if k.readOnly() {
		return UsageError("%s is read-only", k.name).withCode("ERR_READ_ONLY_SETTING")
	}
	before, _ := k.get()
	if err := k.set(c, c.Arg(1)); err != nil {
		return err
	}
	return cfgReport(c, k, before)
}

func runConfigUnset(c *Ctx) error {
	k, err := lookupCfgKey(c.Arg(0))
	if err != nil {
		return err
	}
	if k.readOnly() {
		return UsageError("%s is read-only", k.name).withCode("ERR_READ_ONLY_SETTING")
	}
	before, src := k.get()
	if src != cfgSourceSet {
		// Nothing saved: already the default.
		return cfgReport(c, k, before)
	}
	if err := k.unset(c); err != nil {
		return err
	}
	return cfgReport(c, k, before)
}

// cfgReport prints "key: before -> after" and the notices of a change.
func cfgReport(c *Ctx, k *cfgKey, before string) error {
	after, src := k.get()
	ch := cfgChange{Key: k.name, Previous: before, Value: after, Source: src, Changed: before != after}
	if ch.Changed {
		if k.name == "data-root" {
			c.Warn("existing instances, downloads and backups in %s were not moved", before)
		}
		if cfgGUIRunning() {
			c.Notice("The launcher window is running; restart it to apply the change there")
		}
	}
	return c.Render(ch, []string{after}, func(w io.Writer) error {
		if !ch.Changed {
			_, err := fmt.Fprintf(w, "%s: %s (unchanged)\n", k.name, Dash(after))
			return err
		}
		_, err := fmt.Fprintf(w, "%s: %s -> %s\n", k.name, Dash(before), Dash(after))
		return err
	})
}
