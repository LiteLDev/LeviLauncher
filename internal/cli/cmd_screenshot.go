package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/contentmgr"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

func init() {
	Register(&Command{
		Path:    "screenshot list",
		Summary: "list the screenshots of an instance and player, newest first",
		Description: `-q prints only the file paths, for copying the pictures elsewhere. To open
the folder, use 'instance path <instance> screenshots --open'.`,
		Examples: []string{"screenshot list", "screenshot ls -i Survival -p Steve -q", "screenshot list --json"},
		Run:      runScreenshotList,
	})
	Register(&Command{
		Path:        "screenshot remove",
		Summary:     "delete screenshots",
		Args:        "<name|path>... | --all",
		Description: "Deletes each picture with its sidecar files (.json, .mc). Asks for confirmation.",
		Examples:    []string{"screenshot remove 2024-05-01_14.03.22", "screenshot rm --all -p Steve --yes"},
		Flags:       []*Flag{BoolFlag("all", "", "every screenshot of the instance and player")},
		MaxArgs:     Unlimited,
		Run:         runScreenshotRemove,
	})
}

type screenshotItem struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	TakenAt time.Time `json:"takenAt"`
}

// screenshotScope resolves the instance and player of a screenshot command.
func screenshotScope(c *Ctx) (Instance, Player, error) {
	inst, err := c.ResolveInstance("")
	if err != nil {
		return Instance{}, Player{}, err
	}
	player, err := c.ResolvePlayer(inst)
	if err != nil {
		return Instance{}, Player{}, err
	}
	return inst, player, nil
}

func screenshotScan(inst Instance, player string) []screenshotItem {
	shots := contentManager().ListScreenshots(inst.Name, player)
	out := make([]screenshotItem, 0, len(shots))
	for _, s := range shots {
		out = append(out, screenshotFrom(s))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].TakenAt.Equal(out[j].TakenAt) {
			return out[i].TakenAt.After(out[j].TakenAt)
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func screenshotFrom(s contentmgr.ScreenshotInfo) screenshotItem {
	it := screenshotItem{Name: s.Name, Path: s.Path}
	if s.CaptureTime > 0 {
		it.TakenAt = time.Unix(s.CaptureTime, 0)
	}
	return it
}

func runScreenshotList(c *Ctx) error {
	inst, player, err := screenshotScope(c)
	if err != nil {
		return err
	}
	items := screenshotScan(inst, player.ID)
	if len(items) == 0 {
		c.Notice("No screenshots.")
	}
	ids := make([]string, len(items))
	for i, s := range items {
		ids[i] = s.Path
	}
	return c.Render(items, ids, func(w io.Writer) error {
		if len(items) == 0 {
			return nil
		}
		rows := make([][]string, len(items))
		for i, s := range items {
			rows[i] = []string{Ago(s.TakenAt), s.Name, s.Path}
		}
		return Table(w, []string{"TAKEN", "NAME", "PATH"}, rows)
	})
}

// screenshotFind resolves a screenshot by path, or by name when only one
// screenshot has it.
func screenshotFind(shots []screenshotItem, ref, inst string) (screenshotItem, error) {
	ref = strings.TrimSpace(ref)
	for _, s := range shots {
		if contentSamePath(s.Path, ref) {
			return s, nil
		}
	}
	var hits []screenshotItem
	for _, s := range shots {
		if strings.EqualFold(s.Name, ref) || strings.EqualFold(filepath.Base(s.Path), ref) {
			hits = append(hits, s)
		}
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		return screenshotItem{}, Errorf(ExitNotFound, "ERR_SCREENSHOT_NOT_FOUND", "no screenshot %q in instance %s", ref, inst).
			WithHint("%s screenshot list -i %s", ProgramName, quoteArg(inst))
	}
	paths := make([]string, len(hits))
	for i, s := range hits {
		paths[i] = s.Path
	}
	return screenshotItem{}, UsageError("%q matches several screenshots: %s", ref, strings.Join(paths, ", ")).
		withCode("ERR_AMBIGUOUS_SCREENSHOT").WithHint("refer to the screenshot by its path")
}

var screenshotDelete = func(inst, player, path string) string {
	return contentManager().DeleteScreenshot(inst, player, path)
}

func runScreenshotRemove(c *Ctx) error {
	all := c.Bool("all")
	switch {
	case all && len(c.Args) > 0:
		return UsageError("give screenshot names or --all, not both")
	case !all && len(c.Args) == 0:
		return UsageError("name the screenshots to delete, or pass --all").WithHint("%s screenshot list", ProgramName)
	}
	inst, player, err := screenshotScope(c)
	if err != nil {
		return err
	}
	shots := screenshotScan(inst, player.ID)
	var picked []screenshotItem
	if all {
		picked = shots
		if len(picked) == 0 {
			c.Notice("No screenshots to delete.")
			return c.Render([]contentItem{}, nil, nil)
		}
	} else {
		seen := map[string]bool{}
		for _, r := range c.Args {
			s, err := screenshotFind(shots, r, inst.Name)
			if err != nil {
				return err
			}
			if k := strings.ToLower(s.Path); !seen[k] {
				seen[k] = true
				picked = append(picked, s)
			}
		}
	}
	q := fmt.Sprintf("Permanently delete screenshot %s?", picked[0].Name)
	if len(picked) > 1 {
		q = fmt.Sprintf("Permanently delete %d screenshots of instance %s?", len(picked), inst.Name)
	}
	if err := c.Confirm(q); err != nil {
		return err
	}
	b := newContentBatch(c, len(picked), "removed")
	for _, s := range picked {
		it := contentItem{Item: s.Name, Name: s.Name, Path: s.Path}
		res := screenshotDelete(inst.Name, player.ID, s.Path)
		switch {
		case res != "":
			it.err = Backend(res)
		case utils.FileExists(s.Path):
			// The backend ignores removal errors; report them.
			it.err = Errorf(ExitFailure, "ERR_WRITE_FILE", "cannot delete screenshot %s (the file may be in use)", s.Name)
		default:
			it.Status, it.qid, it.line = "removed", s.Path, "removed screenshot "+s.Name
		}
		b.add(it)
	}
	return b.finish()
}
