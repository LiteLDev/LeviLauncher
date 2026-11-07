package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/update"
)

// selfUpdateDevVersion is the version of a build that was not stamped with
// one; the update package never offers it an update.
const selfUpdateDevVersion = "0.0.0-dev"

// selfUpdateResult is `self-update --json`.
type selfUpdateResult struct {
	Current         string `json:"current"`
	Latest          string `json:"latest"`
	UpdateAvailable bool   `json:"updateAvailable"`
	Notes           string `json:"notes"`
	Updated         bool   `json:"updated"`
}

// Seams replaced by tests.
var (
	suVersion    = update.GetAppVersion
	suCheck      = update.CheckUpdateErr
	suApply      = update.Apply
	suGUIRunning = GUIRunning
	// suGameRunning lists Minecraft processes started from an instance.
	suGameRunning = func() []string {
		var out []string
		for _, p := range minecraftProcesses() {
			if p.IsLauncher {
				out = append(out, p.VersionName)
			}
		}
		return out
	}
)

func init() {
	Register(&Command{
		Path:    "self-update",
		Summary: "update LeviLauncher itself to the newest release",
		Description: `Downloads the newest release and installs it over this program; the new version
runs from the next start. It refuses while the launcher window or a game
started from an instance is running. When the program folder is not writable,
run it from an elevated (administrator) terminal.

Pre-releases are offered with --beta, with 'config set beta-updates on', or
when this build is itself a pre-release.`,
		Examples: []string{"self-update --check", "self-update --yes", "self-update --beta --check --json"},
		Flags: []*Flag{
			BoolFlag("check", "", "only report whether an update exists, with its release notes"),
			BoolFlag("beta", "", "include pre-releases for this run"),
		},
		Group: GroupSystem,
		Run:   runSelfUpdate,
	})
}

func vPrefix(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}

func runSelfUpdate(c *Ctx) error {
	beta := c.Bool("beta")
	res := selfUpdateResult{Current: vPrefix(suVersion())}
	if strings.TrimPrefix(res.Current, "v") == selfUpdateDevVersion {
		c.Notice("This is a development build (%s); updates are not offered", res.Current)
		return renderSelfUpdate(c, res, c.Bool("check"))
	}

	prog := c.Progress()
	prog.Phase("Checking for updates")
	chk, err := suCheck(c.Context(), "", beta)
	prog.Done()
	if err != nil {
		return FromError(err)
	}
	if chk.IsUpdate {
		res.Latest, res.UpdateAvailable, res.Notes = vPrefix(chk.Version), true, chk.Body
	} else {
		res.Latest = res.Current
	}
	if c.Bool("check") || !res.UpdateAvailable {
		return renderSelfUpdate(c, res, true)
	}

	if suGUIRunning() {
		return ConflictError("the launcher window is running").
			WithHint("close LeviLauncher (also from the tray), then run '%s self-update' again", ProgramName).
			withCode("ERR_LAUNCHER_RUNNING")
	}
	if games := suGameRunning(); len(games) > 0 {
		return ConflictError("Minecraft is running (%s)", strings.Join(games, ", ")).
			WithHint("%s stop --all", ProgramName).withCode("ERR_GAME_RUNNING")
	}
	if err := c.Confirm(fmt.Sprintf("Update LeviLauncher from %s to %s?", res.Current, res.Latest)); err != nil {
		return err
	}
	applied, err := suApply(c.Context(), "", beta)
	c.Progress().Done()
	if err != nil {
		e := FromError(err)
		if e.Code == "ERR_UPDATE_NOT_AVAILABLE" {
			// Another process updated in the meantime.
			res.Latest, res.UpdateAvailable = res.Current, false
			return renderSelfUpdate(c, res, true)
		}
		return e
	}
	res.Updated = true
	if applied.To != "" {
		res.Latest = vPrefix(applied.To)
	}
	return renderSelfUpdate(c, res, false)
}

func renderSelfUpdate(c *Ctx, res selfUpdateResult, report bool) error {
	return c.Render(res, []string{res.Latest}, func(w io.Writer) error {
		switch {
		case res.Updated:
			fmt.Fprintf(w, "Updated %s -> %s; the new version runs from the next start\n", res.Current, res.Latest)
			return nil
		case !res.UpdateAvailable:
			fmt.Fprintf(w, "Up to date (%s)\n", Dash(res.Current))
			return nil
		}
		fmt.Fprintf(w, "Update available: %s -> %s\n", res.Current, res.Latest)
		if notes := strings.TrimSpace(res.Notes); notes != "" && report {
			lines := strings.Split(strings.ReplaceAll(notes, "\r\n", "\n"), "\n")
			if !c.G.Verbose && len(lines) > 20 {
				lines = append(lines[:20], fmt.Sprintf("... (%d more lines; -v shows all)", len(lines)-20))
			}
			fmt.Fprintf(w, "\n%s\n", strings.Join(lines, "\n"))
		}
		if report {
			c.Notice("Install it with: %s self-update", ProgramName)
		}
		return nil
	})
}
