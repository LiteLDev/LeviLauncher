package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/content"
	"github.com/liteldev/LeviLauncher/internal/mcservice"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

// Seams replaced by tests: the window's world backup functions, which write
// to the launcher's world-backups folder.
var (
	worldBackupWithVersion = mcservice.BackupWorldWithVersion
	worldBackupPlain       = mcservice.BackupWorld
	worldNow               = time.Now
)

func init() {
	Register(&Command{
		Path:    "world list",
		Summary: "list the worlds of an instance and player",
		Args:    "[<filter>]",
		Description: `The ID (the world's folder name) is the stable reference: display names
need not be unique. Every world command accepts the ID or a unique display
name. <filter> matches part of the name or ID, ignoring case.`,
		Examples: []string{"world list", "world list -i Survival -p Steve --size", "world ls -q"},
		Flags:    []*Flag{BoolFlag("size", "", "compute the size of each world (slower)")},
		MaxArgs:  1,
		Run:      runWorldList,
	})
	Register(&Command{
		Path:    "world import",
		Summary: "import worlds from .mcworld files",
		Args:    "<file.mcworld>...",
		Description: `Each file becomes a new world, so nothing is overwritten. On a GDK instance
the world goes to the player chosen with -p (or the only/signed-in profile).
Prints the new world's ID.`,
		Examples: []string{`world import "D:\Downloads\Skyblock.mcworld"`, "world import a.mcworld b.mcworld -i Survival -p Steve"},
		MinArgs:  1, MaxArgs: Unlimited,
		Run: runWorldImport,
	})
	Register(&Command{
		Path:    "world export",
		Summary: "export (back up) worlds as .mcworld files",
		Args:    "<world>...",
		Description: `Without -o the archive goes to the launcher's world-backups folder, named
with the date and time. -o names a .mcworld file (one world) or a folder (any
number of worlds; created when missing). Prints the path of each archive.`,
		Examples: []string{"world export \"My Survival\"", `world export abc123= -o D:\Backups`, `world export abc123= -o D:\Backups\survival.mcworld`},
		Flags:    []*Flag{StringFlag("output", "o", "<file|dir>", "where to write the archive")},
		MinArgs:  1, MaxArgs: Unlimited,
		Run: runWorldExport,
	})
	Register(&Command{
		Path:        "world remove",
		Summary:     "permanently delete worlds",
		Args:        "<world>...",
		Description: "Asks for confirmation; export a world first if you may want it back.",
		Examples:    []string{"world remove \"Old Test\"", "world rm abc123= def456= --yes"},
		MinArgs:     1, MaxArgs: Unlimited,
		Run: runWorldRemove,
	})
	Register(&Command{
		Path:        "world rename",
		Summary:     "change a world's display name",
		Args:        "<world> <new-name>",
		Description: "Updates both levelname.txt and the LevelName field of level.dat, so the game shows the new name.",
		Examples:    []string{`world rename abc123= "Skyblock"`, `world rename "New World" "Creative Build" -i Creative`},
		MinArgs:     2, MaxArgs: 2,
		Run: runWorldRename,
	})
	Register(&Command{
		Path:    "world copy",
		Summary: "copy worlds to another instance",
		Args:    "<world>...",
		Description: `Nothing is overwritten: a folder name already used in the target gets a _1
suffix. The target must have its own data (or be a UWP instance that does not
share the source's data). On a GDK target the world goes to --to-player, else
the profile with the same ID, else the target's first profile.`,
		Examples: []string{"world copy \"My Survival\" --to 1.21.50-preview", "world copy abc123= --to Survival --to-player Steve"},
		Flags: []*Flag{
			StringFlag("to", "", "<instance>", "target instance (required)"),
			StringFlag("to-player", "", "<id|gamertag>", "player profile in the target instance"),
		},
		MinArgs: 1, MaxArgs: Unlimited,
		Run: runWorldCopy,
	})
	Register(&Command{
		Path:    "world get",
		Summary: "read world settings from level.dat",
		Args:    "<world> [<key>[/<sub>...]]",
		Description: `Without a key, lists every top-level setting (game mode, difficulty,
cheats, game rules, ...). A compound key such as "abilities" lists its
children; nested keys use '/' ("abilities/mayfly"). A single value is printed
bare, so it can be used in scripts.`,
		Examples: []string{"world get \"My Survival\"", "world get abc123= GameType", "world get abc123= abilities --json"},
		MinArgs:  1, MaxArgs: 2,
		Run: runWorldGet,
	})
	Register(&Command{
		Path:    "world set",
		Summary: "change world settings in level.dat",
		Args:    "<world> <key>[/<sub>...]=<value>...",
		Description: `The value's type comes from the existing setting. Known settings accept
names (GameType=creative, Difficulty=hard); byte and int switches accept
true/false. Lists and compounds take JSON. --type is only needed to add a
setting that does not exist yet. The first change keeps a one-time backup
(level.dat_leviold). Refused while Minecraft is running on this data.`,
		Examples: []string{"world set \"My Survival\" GameType=creative Difficulty=hard", "world set abc123= commandsEnabled=true abilities/mayfly=1", "world set abc123= myFlag=1 --type byte"},
		Flags: []*Flag{
			EnumFlag("type", "", []string{"byte", "short", "int", "long", "float", "double", "string", "list", "compound"},
				"type of a new setting"),
		},
		MinArgs: 2, MaxArgs: Unlimited,
		Run: runWorldSet,
	})
}

// worldEntry is a world folder.
type worldEntry struct {
	ID       string
	Name     string
	Path     string
	Modified time.Time
}

// worldScope is the instance, player and worlds folder a world command
// acts on.
type worldScope struct {
	inst   Instance
	player Player
	roots  types.ContentRoots
	dir    string
}

func worldScopeOf(c *Ctx) (worldScope, error) {
	inst, err := c.ResolveInstance("")
	if err != nil {
		return worldScope{}, err
	}
	player, err := c.ResolvePlayer(inst)
	if err != nil {
		return worldScope{}, err
	}
	roots := contentRoots(inst)
	dir := contentPlayerDir(roots, player.ID, "minecraftWorlds")
	if dir == "" {
		return worldScope{}, Errorf(ExitFailure, "ERR_NO_WORLDS_FOLDER", "cannot locate the worlds folder of instance %s", inst.Name)
	}
	return worldScope{inst: inst, player: player, roots: roots, dir: dir}, nil
}

// worldScan lists the worlds in dir (empty when it does not exist).
func worldScan(dir string) []worldEntry {
	out := []worldEntry{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		w := worldEntry{ID: e.Name(), Path: p, Name: mcservice.GetWorldLevelName(p)}
		if w.Name == "" {
			w.Name = e.Name()
		}
		if fi, err := e.Info(); err == nil {
			w.Modified = fi.ModTime()
		}
		out = append(out, w)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if a != b {
			return a < b
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// worldFind resolves a world reference: its ID (folder name), else its
// display name, both exact and ignoring case.
func worldFind(worlds []worldEntry, ref string, inst string) (worldEntry, error) {
	ref = strings.TrimSpace(ref)
	for _, w := range worlds {
		if w.ID == ref {
			return w, nil
		}
	}
	var byID, byName []worldEntry
	for _, w := range worlds {
		if strings.EqualFold(w.ID, ref) {
			byID = append(byID, w)
		}
		if strings.EqualFold(w.Name, ref) {
			byName = append(byName, w)
		}
	}
	if len(byID) == 1 {
		return byID[0], nil
	}
	switch len(byName) {
	case 1:
		return byName[0], nil
	case 0:
	default:
		ids := make([]string, len(byName))
		for i, w := range byName {
			ids[i] = w.ID
		}
		return worldEntry{}, contentAmbiguous("world", ref, ids)
	}
	e := Errorf(ExitNotFound, "ERR_WORLD_NOT_FOUND", "no world %q in instance %s", ref, inst)
	var cands []string
	for _, w := range worlds {
		cands = append(cands, w.Name, w.ID)
	}
	if s := suggest(ref, cands); s != "" {
		return worldEntry{}, e.WithHint("did you mean %q?", s)
	}
	return worldEntry{}, e.WithHint("%s world list -i %s", ProgramName, quoteArg(inst))
}

// worldFindAll resolves every reference before anything changes, dropping
// repeats.
func worldFindAll(worlds []worldEntry, refs []string, inst string) ([]worldEntry, error) {
	var out []worldEntry
	seen := map[string]bool{}
	for _, r := range refs {
		w, err := worldFind(worlds, r, inst)
		if err != nil {
			return nil, err
		}
		if !seen[w.ID] {
			seen[w.ID] = true
			out = append(out, w)
		}
	}
	return out, nil
}

func worldLabel(w worldEntry) string {
	if w.Name == w.ID {
		return fmt.Sprintf("%q", w.Name)
	}
	return fmt.Sprintf("%q (%s)", w.Name, w.ID)
}

// --- world list ---

type worldListItem struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Modified time.Time `json:"modified"`
	Size     *int64    `json:"size,omitempty"`
	IconPath string    `json:"iconPath,omitempty"`
}

func runWorldList(c *Ctx) error {
	s, err := worldScopeOf(c)
	if err != nil {
		return err
	}
	filter := strings.ToLower(strings.TrimSpace(c.Arg(0)))
	withSize := c.Bool("size")
	items := []worldListItem{}
	for _, w := range worldScan(s.dir) {
		if filter != "" && !strings.Contains(strings.ToLower(w.Name+"\x00"+w.ID), filter) {
			continue
		}
		it := worldListItem{ID: w.ID, Name: w.Name, Path: w.Path, Modified: w.Modified}
		if withSize {
			n := utils.DirSize(w.Path)
			it.Size = &n
		}
		if icon := filepath.Join(w.Path, "world_icon.jpeg"); utils.FileExists(icon) {
			it.IconPath = icon
		}
		items = append(items, it)
	}
	if len(items) == 0 {
		if filter != "" {
			c.Notice("No world matches.")
		} else {
			c.Notice("No worlds in %s.", s.dir)
		}
	}
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	return c.Render(items, ids, func(w io.Writer) error {
		if len(items) == 0 {
			return nil
		}
		header := []string{"ID", "NAME", "MODIFIED"}
		if withSize {
			header = append(header, "SIZE")
		}
		rows := make([][]string, len(items))
		for i, it := range items {
			rows[i] = []string{it.ID, it.Name, Ago(it.Modified)}
			if it.Size != nil {
				rows[i] = append(rows[i], Size(*it.Size))
			}
		}
		return Table(w, header, rows)
	})
}

// --- world import ---

func runWorldImport(c *Ctx) error {
	for _, f := range c.Args {
		if contentFileExt(f) != ".mcworld" {
			return contentWrongFileError(f, "a .mcworld file")
		}
		if err := contentCheckFile(f); err != nil {
			return err
		}
	}
	s, err := worldScopeOf(c)
	if err != nil {
		return err
	}
	mgr := contentManager()
	b := newContentBatch(c, len(c.Args), "imported")
	p := c.Progress()
	for _, f := range c.Args {
		if err := c.Context().Err(); err != nil {
			return err
		}
		p.Phase("Importing %s", filepath.Base(f))
		_ = os.MkdirAll(s.dir, 0o755)
		before := contentDirNames(s.dir)
		res := mgr.ImportMcworldPath(s.inst.Name, s.player.ID, f, false)
		created := contentNewDirs(s.dir, before)
		it := contentItem{Item: f}
		if res != "" {
			for _, d := range created {
				_ = os.RemoveAll(filepath.Join(s.dir, d))
			}
			it.err = worldImportError(res, f)
			b.add(it)
			continue
		}
		if len(created) == 1 {
			path := filepath.Join(s.dir, created[0])
			it.ID, it.Path = created[0], path
			it.Name = mcservice.GetWorldLevelName(path)
		}
		it.Status, it.qid = "imported", it.ID
		it.line = fmt.Sprintf("imported %s as %s", filepath.Base(f), worldLabel(worldEntry{ID: it.ID, Name: contentFirstNonEmpty(it.Name, it.ID)}))
		b.add(it)
	}
	p.Done()
	return b.finish()
}

func worldImportError(code, file string) *Error {
	e := Backend(code)
	switch code {
	case "ERR_OPEN_ZIP":
		e.Message = fmt.Sprintf("cannot read %s as a world archive", filepath.Base(file))
	case "ERR_ACCESS_VERSIONS_DIR":
		e.Message = "cannot locate the worlds folder of this instance and player"
	}
	return e
}

func contentFirstNonEmpty(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// --- world export ---

func runWorldExport(c *Ctx) error {
	s, err := worldScopeOf(c)
	if err != nil {
		return err
	}
	worlds, err := worldFindAll(worldScan(s.dir), c.Args, s.inst.Name)
	if err != nil {
		return err
	}
	out := strings.TrimSpace(c.String("output"))
	dirMode := false
	if out != "" {
		fi, statErr := os.Stat(out)
		// A path naming a .mcworld file is the archive itself; anything else
		// is a folder, created when missing.
		isFile := statErr == nil && !fi.IsDir() ||
			statErr != nil && contentFileExt(out) == ".mcworld" && !strings.HasSuffix(out, `\`) && !strings.HasSuffix(out, "/")
		switch {
		case isFile && len(worlds) > 1:
			return UsageError("-o must be a folder when exporting several worlds")
		case isFile && statErr == nil:
			return Errorf(ExitConflict, "ERR_FILE_EXISTS", "%s already exists", out).
				WithHint("choose another file with -o, or remove that one first")
		}
		dirMode = !isFile
	}
	b := newContentBatch(c, len(worlds), "exported")
	p := c.Progress()
	for _, w := range worlds {
		if err := c.Context().Err(); err != nil {
			return err
		}
		p.Phase("Exporting %s", w.Name)
		it := contentItem{Item: w.ID, ID: w.ID, Name: w.Name}
		dest, e := worldExportOne(w, s.inst.Name, out, dirMode)
		if e != nil {
			it.err = e
		} else {
			it.Status, it.Path, it.qid, it.line = "exported", dest, dest, dest
		}
		b.add(it)
	}
	p.Done()
	return b.finish()
}

// worldExportOne zips one world: to the backups folder (out ""), into the
// folder out (dirMode), or to the file out.
func worldExportOne(w worldEntry, inst, out string, dirMode bool) (string, *Error) {
	fail := func(err error) *Error {
		return Errorf(ExitFailure, "ERR_WORLD_EXPORT_FAILED", "cannot export world %s: %v", worldLabel(w), err)
	}
	if out == "" {
		dest := worldBackupWithVersion(w.Path, inst)
		if dest == "" {
			dest = worldBackupPlain(w.Path)
		}
		if dest == "" {
			return "", Errorf(ExitFailure, "ERR_WORLD_EXPORT_FAILED", "cannot export world %s", worldLabel(w)).
				WithHint("pass -o <folder> to write the archive somewhere else")
		}
		return dest, nil
	}
	dest := out
	if dirMode {
		if err := os.MkdirAll(out, 0o755); err != nil {
			return "", fail(err)
		}
		base := utils.SanitizeFilename(w.Name) + "_" + worldNow().Format("20060102-150405")
		dest = filepath.Join(out, base+".mcworld")
		for i := 1; utils.FileExists(dest); i++ {
			dest = filepath.Join(out, fmt.Sprintf("%s_%d.mcworld", base, i))
		}
	} else if dir := filepath.Dir(out); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fail(err)
		}
	}
	if err := utils.ZipDir(w.Path, dest); err != nil {
		return "", fail(err)
	}
	if abs, err := filepath.Abs(dest); err == nil {
		dest = abs
	}
	return dest, nil
}

// --- world remove ---

func runWorldRemove(c *Ctx) error {
	s, err := worldScopeOf(c)
	if err != nil {
		return err
	}
	worlds, err := worldFindAll(worldScan(s.dir), c.Args, s.inst.Name)
	if err != nil {
		return err
	}
	if err := contentEnsureStopped(s.inst, "worlds"); err != nil {
		return err
	}
	var q string
	ids := make([]string, len(worlds))
	for i, w := range worlds {
		ids[i] = quoteArg(w.ID)
	}
	if len(worlds) == 1 {
		q = fmt.Sprintf("Permanently delete world %s of instance %s?", worldLabel(worlds[0]), s.inst.Name)
	} else {
		names := make([]string, len(worlds))
		for i, w := range worlds {
			names[i] = worldLabel(w)
		}
		q = fmt.Sprintf("Permanently delete %d worlds of instance %s (%s)?", len(worlds), s.inst.Name, strings.Join(names, ", "))
	}
	if !c.G.Yes && c.CanPrompt() {
		c.Notice("To keep a copy first: %s world export %s -i %s", ProgramName, strings.Join(ids, " "), quoteArg(s.inst.Name))
	}
	if err := c.Confirm(q); err != nil {
		return err
	}
	mgr := contentManager()
	b := newContentBatch(c, len(worlds), "removed")
	for _, w := range worlds {
		it := contentItem{Item: w.ID, ID: w.ID, Name: w.Name, Path: w.Path}
		if res := mgr.DeleteWorld(s.inst.Name, w.Path); res != "" {
			it.err = Backend(res)
			if res == "ERR_WRITE_FILE" {
				it.err.Message = fmt.Sprintf("cannot delete world %s (a file may be in use)", worldLabel(w))
			}
		} else {
			it.Status, it.qid, it.line = "removed", w.ID, "removed world "+worldLabel(w)
		}
		b.add(it)
	}
	return b.finish()
}

// --- world rename ---

type worldRenameResult struct {
	ID      string `json:"id"`
	OldName string `json:"oldName"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Changed bool   `json:"changed"`
}

func runWorldRename(c *Ctx) error {
	newName := strings.TrimSpace(c.Arg(1))
	if newName == "" {
		return UsageError("the new name must not be empty")
	}
	if strings.ContainsAny(newName, "\r\n") {
		return UsageError("the new name must be a single line")
	}
	s, err := worldScopeOf(c)
	if err != nil {
		return err
	}
	w, err := worldFind(worldScan(s.dir), c.Arg(0), s.inst.Name)
	if err != nil {
		return err
	}
	txt := mcservice.GetWorldLevelName(w.Path)
	nbtName, hasNBT, ver := worldLevelNameField(w.Path)
	res := worldRenameResult{ID: w.ID, OldName: w.Name, Name: newName, Path: w.Path}
	if txt == newName && (!hasNBT || nbtName == newName) {
		c.Notice("World %s is already named %q.", w.ID, newName)
		return c.Render(res, []string{w.ID}, func(io.Writer) error { return nil })
	}
	if err := contentEnsureStopped(s.inst, "worlds"); err != nil {
		return err
	}
	if e := Backend(mcservice.SetWorldLevelName(w.Path, newName)); e != nil {
		return e
	}
	if hasNBT && nbtName != newName {
		f := types.LevelDatField{Name: "LevelName", Tag: "string", ValueString: newName}
		if err := content.WriteLevelDatFields(w.Path, []types.LevelDatField{f}, ver); err != nil {
			return Errorf(ExitFailure, "ERR_WRITE_FILE", "renamed levelname.txt but cannot update level.dat: %v", err)
		}
	}
	res.Changed = true
	return c.Render(res, []string{w.ID}, func(out io.Writer) error {
		_, err := fmt.Fprintf(out, "renamed world %s to %q\n", worldLabel(w), newName)
		return err
	})
}

// worldLevelNameField reads the LevelName string of level.dat.
func worldLevelNameField(dir string) (name string, ok bool, version int32) {
	fields, ver, err := content.ReadLevelDatFieldsAt(dir, nil)
	if err != nil {
		return "", false, 0
	}
	for _, f := range fields {
		if f.Name == "LevelName" && f.Tag == "string" {
			return f.ValueString, true, ver
		}
	}
	return "", false, ver
}

// --- world copy ---

func runWorldCopy(c *Ctx) error {
	s, err := worldScopeOf(c)
	if err != nil {
		return err
	}
	worlds, err := worldFindAll(worldScan(s.dir), c.Args, s.inst.Name)
	if err != nil {
		return err
	}
	dst, err := contentLookupTarget(c)
	if err != nil {
		return err
	}
	if err := contentCheckTransferTarget(s.inst, s.roots, dst); err != nil {
		return err
	}
	dstRoots := contentRoots(dst)
	dstPlayer, err := worldTargetPlayer(c, dst, dstRoots, s.player.ID)
	if err != nil {
		return err
	}
	dstDir := contentPlayerDir(dstRoots, dstPlayer.ID, "minecraftWorlds")
	if dstDir == "" {
		return Errorf(ExitFailure, "ERR_NO_WORLDS_FOLDER", "cannot locate the worlds folder of instance %s", dst.Name)
	}
	targetLabel := dst.Name
	if dstPlayer.ID != "" {
		targetLabel += " (player " + dstPlayer.label() + ")"
	}
	mgr := contentManager()
	b := newContentBatch(c, len(worlds), "copied")
	p := c.Progress()
	for _, w := range worlds {
		if err := c.Context().Err(); err != nil {
			return err
		}
		p.Phase("Copying %s", w.Name)
		it := contentItem{Item: w.ID, Name: w.Name, Target: dst.Name}
		before := contentDirNames(dstDir)
		if res := mgr.TransferWorldToVersion(s.inst.Name, s.player.ID, w.Path, dst.Name, dstPlayer.ID); res != "" {
			it.err = Backend(res)
			if res == "ERR_ACCESS_VERSIONS_DIR" {
				it.err.Message = "cannot locate the worlds folder of the source or target"
			}
			b.add(it)
			continue
		}
		newID := w.ID
		if created := contentNewDirs(dstDir, before); len(created) == 1 {
			newID = created[0]
		}
		it.Status, it.ID, it.Path, it.qid = "copied", newID, filepath.Join(dstDir, newID), newID
		it.line = fmt.Sprintf("copied %s to %s as %s", worldLabel(w), targetLabel, newID)
		b.add(it)
	}
	p.Done()
	return b.finish()
}

// worldTargetPlayer picks the player profile of a GDK copy target:
// --to-player, else the source's profile ID when the target has it, else
// the target's first profile. UWP targets have none.
func worldTargetPlayer(c *Ctx, dst Instance, dstRoots types.ContentRoots, srcPlayer string) (Player, error) {
	if contentIsUWP(dstRoots) {
		if strings.TrimSpace(c.String("to-player")) != "" {
			c.Notice("--to-player is ignored: UWP instances have no player profiles")
		}
		return Player{}, nil
	}
	players, err := ListPlayers(dst)
	if err != nil {
		return Player{}, Errorf(ExitFailure, "ERR_ACCESS_PLAYERS", "cannot read the player profiles of %s: %v", dst.Name, err)
	}
	if want := strings.TrimSpace(c.String("to-player")); want != "" {
		for _, p := range players {
			if strings.EqualFold(p.ID, want) || (p.Gamertag != "" && strings.EqualFold(p.Gamertag, want)) {
				return p, nil
			}
		}
		return Player{}, Errorf(ExitNotFound, "ERR_PLAYER_NOT_FOUND", "no player %q in instance %s", want, dst.Name).
			WithHint("%s player list -i %s", ProgramName, quoteArg(dst.Name))
	}
	if len(players) == 0 {
		return Player{}, Errorf(ExitNotFound, "ERR_NO_PLAYERS", "instance %s has no player profiles yet", dst.Name).
			WithHint("start the game once and sign in: %s play %s", ProgramName, quoteArg(dst.Name))
	}
	for _, p := range players {
		if strings.EqualFold(p.ID, srcPlayer) {
			return p, nil
		}
	}
	c.Notice("Copying to player %s of %s (first profile; choose another with --to-player)", players[0].label(), dst.Name)
	return players[0], nil
}
