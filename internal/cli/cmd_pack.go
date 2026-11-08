package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/packages"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

// Pack kinds as the user names them.
const (
	packKindResource = "resource"
	packKindBehavior = "behavior"
	packKindSkin     = "skin"
)

var packKinds = []string{packKindResource, packKindBehavior, packKindSkin}

func init() {
	Register(&Command{
		Path:    "pack list",
		Summary: "list resource, behavior and skin packs",
		Args:    "[<filter>]",
		Description: `Every pack command accepts a pack's ID (its folder name), its UUID, its
path or its unique name. Skin packs are stored per player on GDK instances of
game version 1.26 and older; -p chooses the player there, and without one
only resource and behavior packs are listed. <filter> matches part of the
name, ID or UUID, ignoring case.`,
		Examples: []string{"pack list", "pack list --kind resource -i Survival", "pack ls --kind skin -p Steve --json"},
		Flags: []*Flag{
			EnumsFlag("kind", "", packKinds, "only packs of this kind"),
			BoolFlag("size", "", "compute the size of each pack (slower)"),
		},
		MaxArgs: 1,
		Run:     runPackList,
	})
	Register(&Command{
		Path:    "pack import",
		Summary: "install packs and add-ons from .mcpack and .mcaddon files",
		Args:    "<file.mcpack|file.mcaddon>...",
		Description: `The kind of each pack (resource, behavior, skin) comes from its manifest.
A pack whose UUID is already installed is replaced with --overwrite, kept with
--skip-existing, and otherwise asked about on a console (and fails without
one). Skin packs that are stored per player go to the player chosen with -p.`,
		Examples: []string{`pack import "D:\Downloads\Faithful.mcpack"`, "pack import addon.mcaddon -i Survival --overwrite", "pack import a.mcpack b.mcpack --skip-existing"},
		Flags:    contentOverwriteFlags(),
		MinArgs:  1, MaxArgs: Unlimited,
		Run: runPackImport,
	})
	Register(&Command{
		Path:        "pack remove",
		Summary:     "delete installed packs",
		Args:        "<pack>...",
		Description: "Asks for confirmation. When the pack's install folder is left empty it is removed too.",
		Examples:    []string{"pack remove \"Faithful 32x\"", "pack rm 0d6f8c2a-91b4-4c5e-a1f0-3b2c9d8e7f61 --yes"},
		MinArgs:     1, MaxArgs: Unlimited,
		Run: runPackRemove,
	})
	Register(&Command{
		Path:    "pack copy",
		Summary: "copy packs to another instance",
		Args:    "<pack>... | --all",
		Description: `--all copies every pack of the chosen kinds (default: resource and
behavior). The target must have its own data (or be a UWP instance that does
not share the source's data). Skin packs stored per player cannot be copied,
nor copied into an instance that stores them per player: import the .mcpack
there with -p instead.`,
		Examples: []string{"pack copy --all --to 1.21.50-preview", "pack copy \"Faithful 32x\" --to Survival --overwrite"},
		Flags: append([]*Flag{
			StringFlag("to", "", "<instance>", "target instance (required)"),
			BoolFlag("all", "", "copy every pack of the chosen kinds"),
			EnumsFlag("kind", "", packKinds, "only packs of this kind"),
		}, contentOverwriteFlags()...),
		MaxArgs: Unlimited,
		Run:     runPackCopy,
	})
}

// packEntry is an installed pack.
type packEntry struct {
	ID               string    `json:"id"`
	Kind             string    `json:"kind"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	Version          string    `json:"version"`
	MinEngineVersion string    `json:"minEngineVersion"`
	UUID             string    `json:"uuid"`
	Path             string    `json:"path"`
	Modified         time.Time `json:"modified"`
	Size             *int64    `json:"size,omitempty"`

	top string // the install folder (root\ID)
}

func packKindOf(t packages.PackType) string {
	switch t {
	case packages.PackTypeResources:
		return packKindResource
	case packages.PackTypeBehavior:
		return packKindBehavior
	case packages.PackTypeSkins:
		return packKindSkin
	}
	return ""
}

func packKindLabel(kind string) string {
	if kind == "" {
		return "pack"
	}
	return kind + " pack"
}

// packRootsOf lists the folders packs are installed in, as the backend scans
// them: resource, behavior, shared skins, then the player's skins.
func packRootsOf(roots types.ContentRoots, player string) []string {
	out := []string{roots.ResourcePacks, roots.BehaviorPacks}
	if roots.ResourcePacks != "" {
		out = append(out, filepath.Join(filepath.Dir(roots.ResourcePacks), "skin_packs"))
	}
	if contentValidPlayer(player) && roots.UsersRoot != "" {
		pr := filepath.Join(roots.UsersRoot, player)
		out = append(out, filepath.Join(pr, "games", "com.mojang", "skin_packs"), filepath.Join(pr, "skin_packs"))
	}
	return out
}

// packScan lists the packs of inst (with player's skin packs when player is
// set), sorted by kind and name.
func packScan(inst Instance, player string) []packEntry {
	roots := contentRoots(inst)
	installRoots := packRootsOf(roots, player)
	out := []packEntry{}
	for _, p := range contentManager().ListPacksForVersion(inst.Name, player) {
		kind := packKindOf(p.Manifest.PackType)
		if kind == "" || strings.TrimSpace(p.Path) == "" {
			continue
		}
		e := packEntry{Kind: kind, Path: p.Path, UUID: p.Manifest.Identity.UUID,
			Name: contentPlain(p.Manifest.Name), Description: contentPlain(p.Manifest.Description)}
		e.Version, e.MinEngineVersion = packManifestVersions(p.Path)
		for _, r := range installRoots {
			if contentUnder(p.Path, r) {
				rel, err := filepath.Rel(r, p.Path)
				if err == nil {
					e.ID = strings.Split(filepath.ToSlash(rel), "/")[0]
					e.top = filepath.Join(r, e.ID)
				}
				break
			}
		}
		if e.ID == "" {
			e.ID = filepath.Base(p.Path)
		}
		if e.Name == "" {
			e.Name = e.ID
		}
		if fi, err := os.Stat(p.Path); err == nil {
			e.Modified = fi.ModTime()
		}
		out = append(out, e)
	}
	rank := map[string]int{packKindResource: 0, packKindBehavior: 1, packKindSkin: 2}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Kind != b.Kind {
			return rank[a.Kind] < rank[b.Kind]
		}
		if x, y := strings.ToLower(a.Name), strings.ToLower(b.Name); x != y {
			return x < y
		}
		return a.Path < b.Path
	})
	return out
}

// packManifestVersions reads the version and minimum engine version of the
// manifest in dir as dotted strings.
func packManifestVersions(dir string) (version, minEngine string) {
	b, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return "", ""
	}
	var raw packages.RawManifest
	if json.Unmarshal(utils.JsonCompatBytes(b), &raw) != nil {
		return "", ""
	}
	join := func(v any) string {
		parts := packages.ParseVersion(v)
		s := make([]string, len(parts))
		for i, n := range parts {
			s[i] = strconv.Itoa(n)
		}
		return strings.Join(s, ".")
	}
	return join(raw.Header.Version), join(raw.Header.MinEngineVersion)
}

func packFilterKinds(packs []packEntry, kinds []string) []packEntry {
	if len(kinds) == 0 {
		return packs
	}
	out := []packEntry{}
	for _, p := range packs {
		if containsFold(kinds, p.Kind) {
			out = append(out, p)
		}
	}
	return out
}

// packFind resolves a pack reference: its path, ID (folder name), UUID or
// unique name, exact and ignoring case.
func packFind(packs []packEntry, ref, inst string, playerKnown bool) (packEntry, error) {
	ref = strings.TrimSpace(ref)
	steps := []func(packEntry) bool{
		func(p packEntry) bool {
			return contentSamePath(p.Path, ref) || (p.top != "" && contentSamePath(p.top, ref))
		},
		func(p packEntry) bool { return strings.EqualFold(p.ID, ref) },
		func(p packEntry) bool { return p.UUID != "" && strings.EqualFold(p.UUID, ref) },
		func(p packEntry) bool { return strings.EqualFold(p.Name, ref) },
	}
	for _, match := range steps {
		var hits []packEntry
		for _, p := range packs {
			if match(p) {
				hits = append(hits, p)
			}
		}
		switch len(hits) {
		case 0:
			continue
		case 1:
			return hits[0], nil
		}
		desc := make([]string, len(hits))
		for i, p := range hits {
			desc[i] = p.Kind + " " + p.ID + " (" + p.Path + ")"
		}
		return packEntry{}, contentAmbiguous("pack", ref, desc).WithHint("refer to the pack by its path")
	}
	e := Errorf(ExitNotFound, "ERR_PACK_NOT_FOUND", "no pack %q in instance %s", ref, inst)
	var cands []string
	for _, p := range packs {
		cands = append(cands, p.Name, p.ID)
	}
	if s := suggest(ref, cands); s != "" {
		return packEntry{}, e.WithHint("did you mean %q?", s)
	}
	if !playerKnown {
		return packEntry{}, e.WithHint("for a skin pack stored per player, pass -p <player>")
	}
	return packEntry{}, e.WithHint("%s pack list -i %s", ProgramName, quoteArg(inst))
}

func packFindAll(packs []packEntry, refs []string, inst string, playerKnown bool) ([]packEntry, error) {
	var out []packEntry
	seen := map[string]bool{}
	for _, r := range refs {
		p, err := packFind(packs, r, inst, playerKnown)
		if err != nil {
			return nil, err
		}
		if k := strings.ToLower(p.Path); !seen[k] {
			seen[k] = true
			out = append(out, p)
		}
	}
	return out, nil
}

func packLabel(p packEntry) string {
	return fmt.Sprintf("%s %q", packKindLabel(p.Kind), p.Name)
}

// packScope resolves the instance and, when skin packs are per player, the
// player. strict makes a missing player an error.
func packScope(c *Ctx, strict bool) (Instance, Player, bool, error) {
	inst, err := c.ResolveInstance("")
	if err != nil {
		return Instance{}, Player{}, false, err
	}
	if contentSharedSkins(inst, contentRoots(inst)) {
		if strings.TrimSpace(c.G.Player) != "" && !inst.IsUWP() {
			c.Verbosef("skin packs of %s are shared by all players; --player is not needed", inst.Name)
		}
		return inst, Player{}, true, nil
	}
	var p Player
	if strict {
		p, err = c.ResolvePlayer(inst)
	} else {
		p, err = contentLenientPlayer(c, inst)
	}
	if err != nil {
		return Instance{}, Player{}, false, err
	}
	return inst, p, p.ID != "", nil
}

// --- pack list ---

func runPackList(c *Ctx) error {
	kinds := c.Strings("kind")
	wantSkins := len(kinds) == 0 || containsFold(kinds, packKindSkin)
	explicitSkins := containsFold(kinds, packKindSkin)
	inst, err := c.ResolveInstance("")
	if err != nil {
		return err
	}
	player := Player{}
	if wantSkins && !contentSharedSkins(inst, contentRoots(inst)) {
		if explicitSkins {
			player, err = c.ResolvePlayer(inst)
		} else {
			player, err = contentLenientPlayer(c, inst)
			if err == nil && player.ID == "" {
				c.Notice("Skin packs of %s are stored per player; pass -p <player> to list them.", inst.Name)
			}
		}
		if err != nil {
			return err
		}
	}
	filter := strings.ToLower(strings.TrimSpace(c.Arg(0)))
	withSize := c.Bool("size")
	items := []packEntry{}
	for _, p := range packFilterKinds(packScan(inst, player.ID), kinds) {
		if filter != "" && !strings.Contains(strings.ToLower(p.Name+"\x00"+p.ID+"\x00"+p.UUID), filter) {
			continue
		}
		if withSize {
			n := utils.DirSize(p.Path)
			p.Size = &n
		}
		items = append(items, p)
	}
	if len(items) == 0 {
		c.Notice("No packs.")
	}
	ids := make([]string, len(items))
	for i, p := range items {
		ids[i] = p.ID
	}
	return c.Render(items, ids, func(w io.Writer) error {
		if len(items) == 0 {
			return nil
		}
		header := []string{"ID", "KIND", "NAME", "VERSION", "UUID", "MODIFIED"}
		if withSize {
			header = append(header, "SIZE")
		}
		rows := make([][]string, len(items))
		for i, p := range items {
			rows[i] = []string{p.ID, p.Kind, p.Name, Dash(p.Version), Dash(p.UUID), Ago(p.Modified)}
			if p.Size != nil {
				rows[i] = append(rows[i], Size(*p.Size))
			}
		}
		return Table(w, header, rows)
	})
}

// --- pack import ---

// packImporter is the backend import API (contentmgr.Manager), replaced by
// tests.
type packImporter interface {
	IsMcpackSkinPackPath(path string) bool
	ImportMcpackPath(name, path string, overwrite bool) string
	ImportMcpackPathWithPlayer(name, player, path string, overwrite bool) string
	ImportMcaddonPath(name, path string, overwrite bool) string
	ImportMcaddonPathWithPlayer(name, player, path string, overwrite bool) string
}

var newPackImporter = func() packImporter { return contentManager() }

func runPackImport(c *Ctx) error {
	for _, f := range c.Args {
		if ext := contentFileExt(f); ext != ".mcpack" && ext != ".mcaddon" {
			return contentWrongFileError(f, "a .mcpack or .mcaddon file")
		}
		if err := contentCheckFile(f); err != nil {
			return err
		}
	}
	dups, err := newContentDuplicates(c)
	if err != nil {
		return err
	}
	inst, err := c.ResolveInstance("")
	if err != nil {
		return err
	}
	imp := newPackImporter()
	shared := contentSharedSkins(inst, contentRoots(inst))
	// A skin .mcpack stored per player needs the player; an add-on uses it
	// when one can be found.
	needPlayer, wantPlayer := false, false
	skins := map[string]bool{}
	for _, f := range c.Args {
		switch contentFileExt(f) {
		case ".mcpack":
			skins[f] = imp.IsMcpackSkinPackPath(f)
			needPlayer = needPlayer || (skins[f] && !shared)
		case ".mcaddon":
			wantPlayer = wantPlayer || !shared
		}
	}
	player := Player{}
	switch {
	case needPlayer:
		if player, err = c.ResolvePlayer(inst); err != nil {
			return err
		}
	case wantPlayer:
		if player, err = contentLenientPlayer(c, inst); err != nil {
			return err
		}
	}
	b := newContentBatch(c, len(c.Args), "imported")
	p := c.Progress()
	for _, f := range c.Args {
		if err := c.Context().Err(); err != nil {
			return err
		}
		p.Phase("Importing %s", filepath.Base(f))
		pl := player.ID
		if shared || (contentFileExt(f) == ".mcpack" && !skins[f]) {
			pl = ""
		}
		run := func(overwrite bool) string {
			if contentFileExt(f) == ".mcaddon" {
				if pl != "" {
					return imp.ImportMcaddonPathWithPlayer(inst.Name, pl, f, overwrite)
				}
				return imp.ImportMcaddonPath(inst.Name, f, overwrite)
			}
			if pl != "" {
				return imp.ImportMcpackPathWithPlayer(inst.Name, pl, f, overwrite)
			}
			return imp.ImportMcpackPath(inst.Name, f, overwrite)
		}
		it := contentItem{Item: f, qid: f}
		if skins[f] {
			it.Kind = packKindSkin
		}
		label := filepath.Base(f)
		res := run(false)
		status := "imported"
		if contentIsDuplicate(res) {
			p.Done()
			overwrite, derr := dups.decide(c, label, res)
			switch {
			case derr != nil:
				it.err = FromError(derr)
				b.add(it)
				continue
			case !overwrite:
				it.Status, it.line = "skipped", "skipped "+label+" (already installed)"
				b.add(it)
				continue
			}
			p.Phase("Importing %s", label)
			res, status = run(true), "overwritten"
		}
		if res != "" {
			it.err = packImportError(res, f, inst)
		} else {
			it.Status, it.line = status, status+" "+label
		}
		b.add(it)
	}
	p.Done()
	return b.finish()
}

func packImportError(code, file string, inst Instance) *Error {
	e := Backend(code)
	switch code {
	case "ERR_OPEN_ZIP":
		e.Message = fmt.Sprintf("cannot read %s as a pack archive", filepath.Base(file))
	case "ERR_NO_PLAYER":
		e.Message = fmt.Sprintf("%s contains a skin pack, and skin packs of %s are stored per player", filepath.Base(file), inst.Name)
		e.Hint = fmt.Sprintf("%s pack import %s -i %s -p <player>", ProgramName, quoteArg(file), quoteArg(inst.Name))
	case "ERR_ACCESS_VERSIONS_DIR":
		e.Message = fmt.Sprintf("cannot locate the pack folders of instance %s", inst.Name)
	}
	return e
}

// --- pack remove ---

var packDelete = func(inst, path string) string { return contentManager().DeletePack(inst, path) }

func runPackRemove(c *Ctx) error {
	inst, player, playerKnown, err := packScope(c, false)
	if err != nil {
		return err
	}
	packs, err := packFindAll(packScan(inst, player.ID), c.Args, inst.Name, playerKnown)
	if err != nil {
		return err
	}
	var q string
	if len(packs) == 1 {
		q = fmt.Sprintf("Permanently delete %s (%s) from instance %s?", packLabel(packs[0]), packs[0].ID, inst.Name)
	} else {
		names := make([]string, len(packs))
		for i, p := range packs {
			names[i] = fmt.Sprintf("%q", p.Name)
		}
		q = fmt.Sprintf("Permanently delete %d packs from instance %s: %s?", len(packs), inst.Name, strings.Join(names, ", "))
	}
	if err := c.Confirm(q); err != nil {
		return err
	}
	b := newContentBatch(c, len(packs), "removed")
	for _, p := range packs {
		it := contentItem{Item: p.ID, ID: p.ID, Name: p.Name, Kind: p.Kind, Path: p.Path}
		if res := packDelete(inst.Name, p.Path); res != "" {
			it.err = Backend(res)
			switch res {
			case "ERR_INVALID_PACKAGE", "ERR_INVALID_PATH":
				it.err.Message = fmt.Sprintf("%s is outside the pack folders the launcher manages and cannot be removed", packLabel(p))
				it.err.Detail = p.Path
			case "ERR_WRITE_FILE":
				it.err.Message = fmt.Sprintf("cannot delete %s (a file may be in use)", packLabel(p))
			}
		} else {
			packRemoveEmptyParents(p)
			it.Status, it.qid, it.line = "removed", p.ID, "removed "+packLabel(p)
		}
		b.add(it)
	}
	return b.finish()
}

// packRemoveEmptyParents removes the folders between a deleted pack and
// its install folder that the deletion left empty.
func packRemoveEmptyParents(p packEntry) {
	if p.top == "" || contentSamePath(p.top, p.Path) {
		return
	}
	for dir := filepath.Dir(p.Path); contentSamePath(dir, p.top) || contentUnder(dir, p.top); dir = filepath.Dir(dir) {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 || os.Remove(dir) != nil || contentSamePath(dir, p.top) {
			return
		}
	}
}

// --- pack copy ---

var packTransfer = func(src, path, dst string, overwrite bool) string {
	return contentManager().TransferPackToVersion(src, path, dst, overwrite)
}

func runPackCopy(c *Ctx) error {
	all := c.Bool("all")
	switch {
	case all && len(c.Args) > 0:
		return UsageError("give pack names or --all, not both")
	case !all && len(c.Args) == 0:
		return UsageError("name the packs to copy, or pass --all").WithHint("%s pack copy --all --to <instance>", ProgramName)
	}
	dups, err := newContentDuplicates(c)
	if err != nil {
		return err
	}
	kinds := c.Strings("kind")
	var src Instance
	var player Player
	var playerKnown bool
	if all {
		if src, err = c.ResolveInstance(""); err != nil {
			return err
		}
		if len(kinds) == 0 {
			kinds = []string{packKindResource, packKindBehavior}
		}
	} else if src, player, playerKnown, err = packScope(c, false); err != nil {
		return err
	}
	dst, err := contentLookupTarget(c)
	if err != nil {
		return err
	}
	srcRoots := contentRoots(src)
	if err := contentCheckTransferTarget(src, srcRoots, dst); err != nil {
		return err
	}
	available := packFilterKinds(packScan(src, player.ID), kinds)
	var packs []packEntry
	if all {
		packs = available
	} else if packs, err = packFindAll(available, c.Args, src.Name, playerKnown); err != nil {
		return err
	}
	if len(packs) == 0 {
		c.Notice("No packs to copy.")
		return c.Render([]contentItem{}, nil, nil)
	}
	sharedSkinDir := ""
	if srcRoots.ResourcePacks != "" {
		sharedSkinDir = filepath.Join(filepath.Dir(srcRoots.ResourcePacks), "skin_packs")
	}
	dstSharedSkins := contentSharedSkins(dst, contentRoots(dst))
	b := newContentBatch(c, len(packs), "copied")
	p := c.Progress()
	for i, pk := range packs {
		if err := c.Context().Err(); err != nil {
			return err
		}
		p.Phase("Copying %s", pk.Name)
		p.Count(int64(i), int64(len(packs)), "packs")
		it := contentItem{Item: pk.ID, ID: pk.ID, Name: pk.Name, Kind: pk.Kind, Target: dst.Name, qid: pk.ID}
		if pk.Kind == packKindSkin {
			switch {
			case !contentUnder(pk.Path, sharedSkinDir):
				it.err = Errorf(ExitConflict, "ERR_SKIN_PACK_PER_PLAYER",
					"%s is stored per player and cannot be copied; import its .mcpack into %s with -p instead", packLabel(pk), dst.Name)
			case !dstSharedSkins:
				it.err = Errorf(ExitConflict, "ERR_SKIN_PACK_PER_PLAYER",
					"instance %s stores skin packs per player (game 1.26 or older); import the .mcpack there with -p instead", dst.Name)
			}
			if it.err != nil {
				b.add(it)
				continue
			}
		}
		res := packTransfer(src.Name, pk.Path, dst.Name, false)
		status := "copied"
		if contentIsDuplicate(res) {
			p.Done()
			overwrite, derr := dups.decide(c, fmt.Sprintf("%s in %s", packLabel(pk), dst.Name), res)
			switch {
			case derr != nil:
				it.err = FromError(derr)
				b.add(it)
				continue
			case !overwrite:
				it.Status, it.line = "skipped", fmt.Sprintf("skipped %s (already in %s)", packLabel(pk), dst.Name)
				b.add(it)
				continue
			}
			res, status = packTransfer(src.Name, pk.Path, dst.Name, true), "overwritten"
		}
		if res != "" {
			it.err = Backend(res)
			switch res {
			case "ERR_ACCESS_VERSIONS_DIR":
				it.err.Message = fmt.Sprintf("cannot locate the pack folders of instance %s", dst.Name)
			case "ERR_INVALID_PACKAGE", "ERR_INVALID_PATH":
				it.err.Message = fmt.Sprintf("%s is outside the pack folders the launcher manages and cannot be copied", packLabel(pk))
			}
		} else {
			verb := "copied"
			if status == "overwritten" {
				verb = "copied (replaced)"
			}
			it.Status, it.line = status, fmt.Sprintf("%s %s to %s", verb, packLabel(pk), dst.Name)
		}
		b.add(it)
	}
	p.Done()
	return b.finish()
}
