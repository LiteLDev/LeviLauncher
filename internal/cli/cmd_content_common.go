package cli

// Helpers shared by the content commands: world, pack, screenshot, server
// and player. Their identifiers start with "content" so they cannot clash
// with other command files of this package.

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/contentmgr"
	"github.com/liteldev/LeviLauncher/internal/mcservice"
	"github.com/liteldev/LeviLauncher/internal/packages"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

// Seams replaced by tests: the content folders of an instance by name.
var contentRootsByName = mcservice.GetContentRoots

func init() {
	RegisterBackendErrorsFor([]string{"world", "pack", "screenshot", "server", "player"}, map[string]BackendError{
		"ERR_OPEN_ZIP":           {Exit: ExitFailure, Message: "cannot read the file as a ZIP archive"},
		"ERR_MANIFEST_NOT_FOUND": {Exit: ExitFailure, Message: "the file contains no manifest.json, so it is not a pack"},
		"ERR_DUPLICATE_UUID": {Exit: ExitConflict, Message: "a pack with the same UUID is already installed",
			Hint: "add --overwrite to replace it, or --skip-existing to keep it"},
		"ERR_DUPLICATE_FOLDER": {Exit: ExitConflict, Message: "a pack with the same folder is already installed",
			Hint: "add --overwrite to replace it, or --skip-existing to keep it"},
		"ERR_NO_PLAYER": {Exit: ExitUsage, Message: "skin packs of this instance are stored per player",
			Hint: "pass -p <id|gamertag>"},
		"ERR_INVALID_PACKAGE":   {Exit: ExitFailure, Message: "the file contains no resource, behavior or skin pack"},
		"ERR_CREATE_TARGET_DIR": {Exit: ExitFailure, Message: "cannot create the target folder"},
		"ERR_READ_ZIP_ENTRY":    {Exit: ExitFailure, Message: "cannot read a file inside the archive"},
		"ERR_WRITE_FILE":        {Exit: ExitFailure, Message: "cannot write the files (check permissions and free disk space)"},
		"ERR_INVALID_PATH":      {Exit: ExitFailure, Message: "the item is not inside this instance's content folders"},
		"ERR_INVALID_WORLD_DIR": {Exit: ExitNotFound, Message: "the world folder does not exist"},
	})
}

// contentManager returns the backend content manager, wired like the
// window's but without a window.
func contentManager() *contentmgr.Manager {
	return contentmgr.New(contentmgr.Deps{
		PackLoader:      packages.PackManager{},
		GetContentRoots: func(name string) types.ContentRoots { return contentRootsByName(name) },
		GetVersionGameInfo: func(name string) string {
			inst, err := LookupInstance(name)
			if err != nil {
				return ""
			}
			return inst.Meta.GameVersion
		},
		ListDir: mcservice.ListDir,
	})
}

// contentRoots returns the content folders of inst.
func contentRoots(inst Instance) types.ContentRoots { return contentRootsByName(inst.Name) }

func contentIsUWP(roots types.ContentRoots) bool {
	return strings.EqualFold(roots.PackageType, versions.PackageTypeUWP)
}

// contentValidPlayer reports whether id is usable as a single path segment.
func contentValidPlayer(id string) bool {
	id = strings.TrimSpace(id)
	return id != "" && id != "." && id != ".." && !strings.ContainsAny(id, `/\:`)
}

// contentPlayerDir is the folder of one kind of per-player content
// (minecraftWorlds, skin_packs, Screenshots, minecraftpe): the shared
// com.mojang folder on UWP, Users\<player>\games\com.mojang\<kind> on GDK.
// "" when it cannot be resolved (GDK without a valid player).
func contentPlayerDir(roots types.ContentRoots, player, kind string) string {
	if contentIsUWP(roots) {
		d := ""
		switch kind {
		case "minecraftWorlds":
			d = roots.Worlds
		case "skin_packs":
			d = roots.SkinPacks
		case "Screenshots":
			d = roots.Screenshots
		}
		if d == "" && roots.ComMojangRoot != "" {
			d = filepath.Join(roots.ComMojangRoot, kind)
		}
		return d
	}
	if strings.TrimSpace(roots.UsersRoot) == "" || !contentValidPlayer(player) {
		return ""
	}
	return filepath.Join(roots.UsersRoot, player, "games", "com.mojang", kind)
}

// contentSharedSkins reports whether skin packs of inst are shared by all
// players (UWP, or a game newer than 1.26.0) rather than stored per player.
func contentSharedSkins(inst Instance, roots types.ContentRoots) bool {
	if contentIsUWP(roots) || inst.IsUWP() {
		return true
	}
	return versions.CompareGameVersions(inst.Meta.GameVersion, "1.26.0.0") > 0
}

// contentRunningPIDs lists the Minecraft processes that use inst's data: the
// instance's own processes when it has its own data, otherwise every
// Minecraft process except those of instances with their own data.
func contentRunningPIDs(inst Instance) []int {
	procs := minecraftProcesses()
	if len(procs) == 0 {
		return nil
	}
	isolated := map[string]bool{}
	if all, err := listInstances(); err == nil {
		for _, i := range all {
			if i.Meta.EnableIsolation {
				isolated[strings.ToLower(i.Name)] = true
			}
		}
	}
	var pids []int
	for _, p := range procs {
		own := p.IsLauncher && strings.EqualFold(p.VersionName, inst.Name)
		if inst.Meta.EnableIsolation {
			if own {
				pids = append(pids, p.Pid)
			}
			continue
		}
		if !own && p.IsLauncher && isolated[strings.ToLower(p.VersionName)] {
			continue
		}
		pids = append(pids, p.Pid)
	}
	return pids
}

// contentEnsureStopped fails with exit 4 when Minecraft is running on the
// data of inst. what names the thing about to change ("world data").
func contentEnsureStopped(inst Instance, what string) error {
	pids := contentRunningPIDs(inst)
	if len(pids) == 0 {
		return nil
	}
	e := Errorf(ExitConflict, "ERR_GAME_RUNNING", "Minecraft is running (PID %s); close it before changing %s", joinInts(pids), what)
	if len(pids) == 1 {
		for _, p := range minecraftProcesses() {
			if p.Pid == pids[0] && !strings.EqualFold(p.VersionName, inst.Name) {
				return e.WithHint("%s stop %d", ProgramName, pids[0])
			}
		}
	}
	return e.WithHint("%s stop %s", ProgramName, quoteArg(inst.Name))
}

// contentCheckTransferTarget ports the window's rule for where content may
// be copied: into an instance with its own data, or into a UWP instance that
// does not share the source's data folder.
func contentCheckTransferTarget(src Instance, srcRoots types.ContentRoots, dst Instance) error {
	if strings.EqualFold(src.Name, dst.Name) {
		return UsageError("cannot copy instance %s into itself", src.Name).withCode("ERR_INVALID_TARGET").
			WithHint("pass another instance with --to <instance>")
	}
	if dst.Meta.EnableIsolation {
		return nil
	}
	if !dst.IsUWP() {
		return UsageError("instance %s uses the shared Minecraft data folder; content can only be copied into instances with their own data, or into UWP instances", dst.Name).
			withCode("ERR_INVALID_TARGET").
			WithHint("%s instance set %s --own-data", ProgramName, quoteArg(dst.Name))
	}
	if !contentIsUWP(srcRoots) || srcRoots.IsIsolation || dst.IsPreview() != srcRoots.IsPreview {
		return nil
	}
	return UsageError("instances %s and %s share the same data folder, so there is nothing to copy", src.Name, dst.Name).
		withCode("ERR_INVALID_TARGET")
}

// contentLookupTarget resolves the --to instance of a copy command.
func contentLookupTarget(c *Ctx) (Instance, error) {
	to := strings.TrimSpace(c.String("to"))
	if to == "" {
		return Instance{}, UsageError("missing --to <instance>").withCode("ERR_TARGET_REQUIRED").
			WithHint("%s instance list", ProgramName)
	}
	return LookupInstance(to)
}

var contentFormatCodes = regexp.MustCompile(`§.?`)

// contentPlain strips Minecraft § formatting codes.
func contentPlain(s string) string {
	return strings.TrimSpace(contentFormatCodes.ReplaceAllString(s, ""))
}

// contentFileExt returns the lower-case extension of path.
func contentFileExt(path string) string { return strings.ToLower(filepath.Ext(path)) }

// contentCheckFile fails with exit 3 when path is not an existing file.
func contentCheckFile(path string) error {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return Errorf(ExitNotFound, "ERR_FILE_NOT_FOUND", "no file %s", path)
	}
	return nil
}

// contentWrongFileError is the usage error for a file the command does not
// import, with a hint naming the command that does.
func contentWrongFileError(path, expected string) *Error {
	e := UsageError("%s is not %s", filepath.Base(path), expected).withCode("ERR_UNSUPPORTED_FILE")
	switch contentFileExt(path) {
	case ".mcworld":
		return e.WithHint("%s world import %s", ProgramName, quoteArg(path))
	case ".mcpack", ".mcaddon":
		return e.WithHint("%s pack import %s", ProgramName, quoteArg(path))
	case ".mcmod", ".zip", ".dll":
		return e.WithHint("%s mod import %s", ProgramName, quoteArg(path))
	}
	return e
}

// contentSamePath compares two paths, ignoring case and separators.
func contentSamePath(a, b string) bool {
	ca, err1 := filepath.Abs(strings.TrimSpace(a))
	cb, err2 := filepath.Abs(strings.TrimSpace(b))
	if err1 != nil || err2 != nil {
		return false
	}
	return strings.EqualFold(filepath.Clean(ca), filepath.Clean(cb))
}

// contentUnder reports whether path is strictly inside root.
func contentUnder(path, root string) bool {
	if strings.TrimSpace(root) == "" {
		return false
	}
	p, err1 := filepath.Abs(strings.TrimSpace(path))
	r, err2 := filepath.Abs(strings.TrimSpace(root))
	if err1 != nil || err2 != nil {
		return false
	}
	p, r = strings.ToLower(filepath.Clean(p)), strings.ToLower(filepath.Clean(r))
	return p != r && strings.HasPrefix(p, r+string(os.PathSeparator))
}

// contentLenientPlayer resolves the player when the command can do without
// one: an explicit -p or $LEVILAUNCHER_PLAYER that does not match is still
// an error, but "no profile" or "several profiles" give the zero Player.
func contentLenientPlayer(c *Ctx, inst Instance) (Player, error) {
	p, err := c.ResolvePlayer(inst)
	if err == nil {
		return p, nil
	}
	if e := FromError(err); e.Code == "ERR_PLAYER_REQUIRED" || e.Code == "ERR_NO_PLAYERS" {
		return Player{}, nil
	}
	return Player{}, err
}

// contentAmbiguous is the exit-2 error for a reference matching several
// items.
func contentAmbiguous(noun, ref string, matches []string) *Error {
	return UsageError("%q matches several %ss: %s", ref, noun, strings.Join(matches, ", ")).
		withCode("ERR_AMBIGUOUS_"+strings.ToUpper(noun)).
		WithHint("refer to the %s by its ID", noun)
}

// contentItem is one result of a batch command (import, remove, copy,
// export), also its --json element.
type contentItem struct {
	Item   string            `json:"item"`
	Status string            `json:"status"`
	ID     string            `json:"id,omitempty"`
	Name   string            `json:"name,omitempty"`
	Kind   string            `json:"kind,omitempty"`
	Path   string            `json:"path,omitempty"`
	Target string            `json:"target,omitempty"`
	Error  *contentItemError `json:"error,omitempty"`

	err  *Error // failure, nil on success
	line string // human result line on success
	qid  string // identifier printed with -q
}

type contentItemError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

// contentBatch runs the items of a batch command in order and reports
// them: one line per item as it completes (failures on stderr), a summary,
// and exit 7 when some items failed or 1 when all did. A batch of one
// item that fails returns that item's error instead.
type contentBatch struct {
	c     *Ctx
	total int
	items []contentItem
	verb  string // past tense for the summary: "removed"
}

func newContentBatch(c *Ctx, total int, verb string) *contentBatch {
	return &contentBatch{c: c, total: total, verb: verb, items: []contentItem{}}
}

// add records a finished item and prints its line.
func (b *contentBatch) add(it contentItem) {
	if it.err != nil {
		it.Status = "failed"
		it.Error = &contentItemError{Code: it.err.Code, Message: it.err.Message, Hint: it.err.Hint}
	}
	b.items = append(b.items, it)
	c := b.c
	if c.G.JSON {
		return
	}
	if it.err != nil {
		if b.total > 1 {
			c.errf("failed %s: %s\n", it.Item, it.err.Message)
		}
		return
	}
	if c.G.Quiet {
		if it.qid != "" {
			fmt.Fprintln(c.Stdout, it.qid)
		}
		return
	}
	if it.line != "" {
		fmt.Fprintln(c.Stdout, it.line)
	}
}

// finish prints the --json document and the summary and returns the exit.
func (b *contentBatch) finish() error {
	c := b.c
	failed := 0
	counts := map[string]int{}
	var order []string
	for _, it := range b.items {
		if it.err != nil {
			failed++
		}
		if counts[it.Status] == 0 {
			order = append(order, it.Status)
		}
		counts[it.Status]++
	}
	if len(b.items) == 1 && b.total == 1 && b.items[0].err != nil {
		return b.items[0].err
	}
	if c.G.JSON {
		if err := c.JSON(b.items); err != nil {
			return err
		}
	}
	if len(b.items) > 1 {
		parts := make([]string, 0, len(order))
		for _, s := range order {
			parts = append(parts, fmt.Sprintf("%d %s", counts[s], s))
		}
		c.Notice("%s", strings.Join(parts, ", "))
		if !c.G.JSON {
			seen := map[string]bool{}
			for _, it := range b.items {
				if it.err != nil && it.err.Hint != "" && !seen[it.err.Hint] {
					seen[it.err.Hint] = true
					c.errf("hint: %s\n", it.err.Hint)
				}
			}
		}
	}
	switch {
	case failed == 0:
		return nil
	case failed == len(b.items):
		return ExitCode(ExitFailure)
	}
	return ExitCode(ExitPartial)
}

// contentDuplicates decides what happens to an item that is already
// installed: --overwrite, --skip-existing, --yes (overwrite), a prompt on a
// console (with "all" and "none" for the rest of the batch), otherwise a
// conflict error for that item.
type contentDuplicates struct {
	mode string // "", "overwrite" or "skip"
}

func newContentDuplicates(c *Ctx) (*contentDuplicates, error) {
	ow, skip := c.Bool("overwrite"), c.Bool("skip-existing")
	switch {
	case ow && skip:
		return nil, UsageError("--overwrite and --skip-existing cannot be used together")
	case ow:
		return &contentDuplicates{mode: "overwrite"}, nil
	case skip:
		return &contentDuplicates{mode: "skip"}, nil
	}
	return &contentDuplicates{}, nil
}

// decide returns true to overwrite, false to skip, or the item's error.
func (d *contentDuplicates) decide(c *Ctx, label, code string) (bool, error) {
	switch d.mode {
	case "overwrite":
		return true, nil
	case "skip":
		return false, nil
	}
	if c.G.Yes {
		return true, nil
	}
	if !c.CanPrompt() {
		e := Backend(code)
		e.Message = label + " is already installed"
		return false, e
	}
	c.errf("%s is already installed. Replace it? [y]es/[N]o/[a]ll/n[o]ne: ", label)
	if c.in == nil {
		c.in = bufio.NewReader(c.Stdin)
	}
	line, _ := c.in.ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	case "a", "all":
		d.mode = "overwrite"
		return true, nil
	case "o", "none":
		d.mode = "skip"
		return false, nil
	}
	return false, nil
}

// contentIsDuplicate reports whether a backend result means "already
// installed".
func contentIsDuplicate(code string) bool {
	return code == "ERR_DUPLICATE_UUID" || code == "ERR_DUPLICATE_FOLDER"
}

// contentDirNames lists the sub-folder names of dir.
func contentDirNames(dir string) map[string]bool {
	out := map[string]bool{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if e.IsDir() {
			out[e.Name()] = true
		}
	}
	return out
}

// contentNewDirs returns the folders of dir that are not in before.
func contentNewDirs(dir string, before map[string]bool) []string {
	var out []string
	for name := range contentDirNames(dir) {
		if !before[name] {
			out = append(out, name)
		}
	}
	return out
}

// contentOverwriteFlags are the duplicate-policy flags of import and copy.
func contentOverwriteFlags() []*Flag {
	return []*Flag{
		BoolFlag("overwrite", "", "replace an installed pack with the same UUID"),
		BoolFlag("skip-existing", "", "keep an installed pack with the same UUID"),
	}
}
