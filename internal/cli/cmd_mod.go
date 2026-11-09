package cli

// Shared parts of the mod commands: the backend seams, reading an
// instance's mods without side effects, the catalogue, mapping mods to it
// and resolving mod references. The commands are in cmd_mod_*.go.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/explorer"
	"github.com/liteldev/LeviLauncher/internal/fsx"
	"github.com/liteldev/LeviLauncher/internal/lip"
	"github.com/liteldev/LeviLauncher/internal/lipindex"
	"github.com/liteldev/LeviLauncher/internal/mcservice"
	"github.com/liteldev/LeviLauncher/internal/mods"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

// modBackend is every backend call the mod commands make; tests replace it.
type modBackend struct {
	catalog          func(ctx context.Context, refresh bool) (*lipindex.Index, time.Time, bool, error)
	fetchDetail      func(ctx context.Context, identifier string) (*lipindex.Detail, error)
	loaderDB         func() (map[string][]string, error)
	installStates    func(ctx context.Context, inst Instance, ids []string) []types.LIPPackageInstallStateEntry
	listPackages     func(ctx context.Context, inst Instance) (map[string]lip.PackageInstallState, error)
	installPackage   func(ctx context.Context, inst Instance, id, version string) string
	uninstallPackage func(ctx context.Context, inst Instance, id string) string
	installLoader    func(ctx context.Context, inst Instance, gameVersion, version string) string
	readme           func(projectURL string) (string, error)
	enable           func(inst Instance, folder string) string
	disable          func(inst Instance, folder string) string
	remove           func(inst Instance, folder string) string
	setManifest      func(inst Instance, folder, name, version, typ, entry, author string) string
	importZip        func(inst Instance, data []byte, overwrite bool) string
	importDll        func(inst Instance, fileName string, data []byte, name, typ, version string, overwrite bool) string
	openFolder       func(dir string) error
}

// catalogMaxAge is how long the cached catalogue is used before it is
// fetched again (a full fetch takes many seconds).
const catalogMaxAge = 30 * time.Minute

var modOps = defaultModBackend()

func defaultModBackend() modBackend {
	client := lipindex.NewClient()
	return modBackend{
		catalog: func(ctx context.Context, refresh bool) (*lipindex.Index, time.Time, bool, error) {
			src := &lipindex.Source{Client: client, MaxAge: catalogMaxAge,
				Cache: &lipindex.Cache{Path: filepath.Join(apppath.CacheDir(), "lip-catalog.v4.json")}}
			return src.Load(ctx, refresh)
		},
		fetchDetail: client.FetchDetail,
		loaderDB:    mcservice.FetchLeviLaminaVersionDB,
		installStates: func(ctx context.Context, inst Instance, ids []string) []types.LIPPackageInstallStateEntry {
			return mcservice.GetLIPPackageInstallStates(ctx, inst.Name, ids)
		},
		listPackages: func(ctx context.Context, inst Instance) (map[string]lip.PackageInstallState, error) {
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			return lip.ListPackageStates(ctx, inst.Dir)
		},
		installPackage: func(ctx context.Context, inst Instance, id, version string) string {
			return mcservice.InstallLIPPackage(ctx, inst.Name, id, version)
		},
		uninstallPackage: func(ctx context.Context, inst Instance, id string) string {
			return mcservice.UninstallLIPPackage(ctx, inst.Name, id)
		},
		installLoader: func(ctx context.Context, inst Instance, gameVersion, version string) string {
			return mcservice.InstallLeviLamina(ctx, gameVersion, inst.Name, version)
		},
		readme:  lip.FetchPackageReadme,
		enable:  func(inst Instance, folder string) string { return mods.EnableMod(inst.Name, folder) },
		disable: func(inst Instance, folder string) string { return mods.DisableMod(inst.Name, folder) },
		remove:  func(inst Instance, folder string) string { return mods.DeleteMod(inst.Name, folder) },
		setManifest: func(inst Instance, folder, name, version, typ, entry, author string) string {
			return mods.UpdateModManifest(inst.Name, folder, name, version, typ, entry, author)
		},
		importZip: func(inst Instance, data []byte, overwrite bool) string {
			return mods.ImportZipToMods(inst.Name, data, overwrite)
		},
		importDll: func(inst Instance, fileName string, data []byte, name, typ, version string, overwrite bool) string {
			return mods.ImportDllToMods(inst.Name, fileName, data, name, typ, version, overwrite)
		},
		openFolder: explorer.OpenPath,
	}
}

func init() {
	RegisterBackendErrors(map[string]BackendError{
		"ERR_LIP_INDEX_TIMEOUT": {Exit: ExitNetwork, Message: "the mod catalogue did not respond in time",
			Hint: "check the network connection and run the command again"},
		"ERR_LIP_INDEX_UNAVAILABLE": {Exit: ExitNetwork, Message: "cannot reach the mod catalogue (levimc.org)",
			Hint: "check the network connection and run the command again"},
		"ERR_LIP_PACKAGE_NOT_FOUND": {Exit: ExitNotFound, Message: "the mod catalogue has no such mod",
			Hint: ProgramName + " mod search <words>"},
	})
}

// modErrors are the messages of backend codes as the mod commands mean
// them. They are applied here rather than registered globally because
// other commands get some of the same codes for packs or worlds.
var modErrors = map[string]BackendError{
	"ERR_UWP_UNSUPPORTED_FEATURE": {Exit: ExitConflict, Message: "mods need the GDK version of Minecraft; this instance is a UWP install"},
	"ERR_TARGET_NOT_FOUND":        {Exit: ExitNotFound, Message: "the instance folder is missing"},
	"ERR_LIP_PACKAGE_INVALID_IDENTIFIER": {Exit: ExitUsage, Message: "not a valid mod package reference",
		Hint: "use owner/repo, for example LiteLDev/LegacyMoney"},
	"ERR_LIP_PACKAGE_VERSION_REQUIRED": {Exit: ExitUsage, Message: "a version is needed"},
	"ERR_LIP_PACKAGE_INSTALL_FAILED": {Exit: ExitFailure, Message: "the package manager could not install the mod",
		Hint: "run the command again with -v to see the package manager's log"},
	"ERR_LIP_PACKAGE_UNINSTALL_FAILED": {Exit: ExitFailure, Message: "the package manager could not remove the mod",
		Hint: "run the command again with -v to see the package manager's log"},
	"ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS": {Exit: ExitConflict, Message: "other installed mods need this mod",
		Hint: "remove the mods that depend on it first"},
	"ERR_LIP_PACKAGE_QUERY_FAILED": {Exit: ExitFailure, Message: "cannot read which packages are installed in the instance"},
	"ERR_LIP_INSTALL_FAILED": {Exit: ExitFailure, Message: "the package manager could not install LeviLamina",
		Hint: "run the command again with -v to see the package manager's log"},
	"ERR_FETCH_LL_DB": {Exit: ExitNetwork, Message: "cannot download the list of LeviLamina versions",
		Hint: "check the network connection and run the command again"},
	"ERR_LL_NOT_SUPPORTED":         {Exit: ExitUsage, Message: "LeviLamina does not support this game version"},
	"ERR_LL_VERSION_UNSUPPORTED":   {Exit: ExitNotFound, Message: "that LeviLamina version does not support this game version"},
	"ERR_INVALID_VERSION_FORMAT":   {Exit: ExitFailure, Message: "the instance's game version is not a full version number"},
	"ERR_OPEN_ZIP":                 {Exit: ExitFailure, Message: "not a valid mod package (cannot open the archive)"},
	"ERR_INVALID_PACKAGE":          {Exit: ExitFailure, Message: "the mod package or folder is not valid"},
	"ERR_INVALID_MANIFEST":         {Exit: ExitFailure, Message: "the mod's manifest.json is invalid (it needs name, entry, version and type)"},
	"ERR_MANIFEST_NOT_FOUND":       {Exit: ExitFailure, Message: "the mod has no manifest.json"},
	"ERR_MANIFEST_ENTRY_NOT_FOUND": {Exit: ExitFailure, Message: "the file named as the mod's entry is missing from the package"},
	"ERR_READ_ZIP_ENTRY":           {Exit: ExitFailure, Message: "cannot read the mod package"},
	"ERR_CREATE_TARGET_DIR":        {Exit: ExitFailure, Message: "cannot create the mod folder"},
	"ERR_DUPLICATE_FOLDER": {Exit: ExitConflict, Message: "a mod with that folder name is already installed",
		Hint: "add --overwrite to replace it or --skip-existing to keep it"},
	"ERR_WRITE_FILE": {Exit: ExitFailure, Message: "cannot write the mod files"},
}

// modBackendError converts a backend result of a mod operation.
func modBackendError(result string) *Error {
	e := Backend(result)
	if e == nil {
		return nil
	}
	if info, ok := modErrors[e.Code]; ok {
		if info.Exit != 0 {
			e.Exit = info.Exit
		}
		if info.Message != "" {
			if e.Detail == "" && e.Message != "" && e.Message != info.Message && !strings.HasPrefix(e.Message, "the operation failed") {
				e.Detail = e.Message
			}
			e.Message = info.Message
		}
		if info.Hint != "" {
			e.Hint = info.Hint
		}
	}
	return e
}

// catalogError converts a failure to load the catalogue.
func modCatalogError(err error) *Error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return FromError(err)
	}
	e := FromError(err)
	if !strings.HasPrefix(e.Code, "ERR_LIP_") {
		e = Backend("ERR_LIP_INDEX_UNAVAILABLE").WithDetail(err.Error())
	}
	return e
}

// lipShortID is a catalogue id without the "github.com/" prefix, as users
// type it.
func lipShortID(id string) string {
	id = strings.TrimSpace(id)
	if l := strings.ToLower(id); strings.HasPrefix(l, "github.com/") {
		return id[len("github.com/"):]
	}
	return id
}

// installedMod is a mod folder of an instance.
type installedMod struct {
	types.ModInfo
	Enabled bool
}

// readMods lists the mods of an instance without changing anything (the
// backend's GetMods creates the mods folder). Mods whose manifest cannot
// be read are skipped, as the window does.
func modReadAll(inst Instance) ([]installedMod, error) {
	dir := filepath.Join(inst.Dir, "mods")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []installedMod{}, nil
		}
		return nil, Errorf(ExitFailure, "ERR_ACCESS_MODS_DIR", "cannot read the mods folder of %s: %v", inst.Name, err)
	}
	out := []installedMod{}
	for _, e := range entries {
		if !e.IsDir() || fsx.IsTempName(e.Name()) {
			continue
		}
		manifest := filepath.Join(dir, e.Name(), "manifest.json")
		closed := manifest + ".close"
		path := manifest
		switch {
		case utils.FileExists(manifest):
		case utils.FileExists(closed):
			path = closed
		default:
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var m types.ModManifestJson
		if json.Unmarshal(utils.JsonCompatBytes(b), &m) != nil {
			continue
		}
		out = append(out, installedMod{
			ModInfo: types.ModInfo{Name: m.Name, Entry: m.Entry, Version: m.Version, Type: m.Type, Author: m.Author, Folder: e.Name()},
			Enabled: utils.FileExists(manifest) && !utils.FileExists(closed),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

// modsDir is the mods folder of an instance.
func modsDir(inst Instance) string { return filepath.Join(inst.Dir, "mods") }

// modView is an instance's mods mapped to the catalogue, as the mods page
// shows them.
type modView struct {
	inst     Instance
	all      []installedMod // including LeviLamina
	visible  []installedMod // without LeviLamina
	enabled  map[string]bool
	index    *lipindex.Index
	mapped   map[string]lipindex.ModState
	states   map[string]lipindex.InstallState // by lookup key
	loader   lipindex.InstallState
	listing  lipindex.Listing
	catErr   *Error // the catalogue could not be loaded
	stateErr string // the workspace could not be queried
}

func modInfos(list []installedMod) []types.ModInfo {
	out := make([]types.ModInfo, len(list))
	for i, m := range list {
		out[i] = m.ModInfo
	}
	return out
}

// loadModView reads the mods of inst. With catalog, it also loads the
// catalogue (fetching it when the cache is stale or refresh is set) and
// maps the mods to it; a catalogue failure is kept in catErr and the mods
// are shown as local ones, as the window does.
func modLoadView(c *Ctx, inst Instance, catalog, refresh bool) (*modView, error) {
	all, err := modReadAll(inst)
	if err != nil {
		return nil, err
	}
	v := &modView{inst: inst, all: all, enabled: map[string]bool{}, states: map[string]lipindex.InstallState{}}
	for _, m := range all {
		v.enabled[lipindex.ModFolder(m.ModInfo)] = m.Enabled
		if !lipindex.IsLeviLaminaMod(m.ModInfo) {
			v.visible = append(v.visible, m)
		}
	}
	if catalog {
		idx, err := modLoadCatalog(c, refresh)
		if err != nil {
			if e := FromError(err); e.Exit == ExitInterrupted {
				return nil, err
			}
			v.catErr = modCatalogError(err)
		}
		v.index = idx
	}
	v.remap(c)
	return v, nil
}

// remap maps the mods to the catalogue and queries the workspace.
func (v *modView) remap(c *Ctx) {
	infos := modInfos(v.visible)
	var mapper *lipindex.Mapper
	if v.index != nil {
		mapper = lipindex.NewMapper(v.index)
	}
	v.mapped = mapper.Map(infos)
	ids := []string{lipindex.LeviLaminaID}
	if v.index != nil {
		ids = append(ids, lipindex.Candidates(v.mapped)...)
	}
	v.states, v.stateErr = modQueryStates(c, v.inst, ids)
	v.loader = v.states[lipindex.LookupKey(lipindex.LeviLaminaID)]
	if v.index == nil || v.stateErr != "" {
		v.listing = lipindex.BuildListing(infos, modNoMapping(infos), nil, v.enabled)
		return
	}
	v.listing = lipindex.BuildListing(infos, v.mapped, v.states, v.enabled)
}

// noMapping maps every mod to no package (catalogue unavailable).
func modNoMapping(mods []types.ModInfo) map[string]lipindex.ModState {
	out := map[string]lipindex.ModState{}
	for _, m := range mods {
		out[lipindex.ModFolder(m)] = lipindex.ModState{Source: lipindex.SourceNone, Mapping: lipindex.MappingNone}
	}
	return out
}

// queryInstallStates asks the instance's package workspace for the states
// of ids, keyed by lookup key. The first per-package error is returned as
// a code; UWP instances have no workspace.
func modQueryStates(c *Ctx, inst Instance, ids []string) (map[string]lipindex.InstallState, string) {
	out := map[string]lipindex.InstallState{}
	if inst.IsUWP() {
		return out, "ERR_UWP_UNSUPPORTED_FEATURE"
	}
	firstErr := ""
	for _, e := range modOps.installStates(c.Context(), inst, ids) {
		k := lipindex.LookupKey(e.IdentifierKey)
		if k == "" {
			continue
		}
		if _, dup := out[k]; dup {
			continue
		}
		st := lipindex.InstallState{Installed: e.State.Installed, Explicit: e.State.ExplicitInstalled,
			InstalledVersion: strings.TrimSpace(e.State.InstalledVersion), Error: strings.TrimSpace(e.State.Error)}
		if st.Error != "" && firstErr == "" {
			firstErr = st.Error
		}
		out[k] = st
	}
	for _, id := range ids {
		k := lipindex.LookupKey(id)
		if _, ok := out[k]; !ok && k != "" {
			out[k] = lipindex.InstallState{Error: "ERR_LIP_PACKAGE_QUERY_FAILED"}
			if firstErr == "" {
				firstErr = "ERR_LIP_PACKAGE_QUERY_FAILED"
			}
		}
	}
	return out, firstErr
}

// loadCatalog loads the catalogue with a progress line. A stale cached
// copy is used with a warning when fetching fails.
func modLoadCatalog(c *Ctx, refresh bool) (*lipindex.Index, error) {
	p := c.Progress()
	p.Phase("Loading the mod catalogue")
	idx, at, stale, err := modOps.catalog(c.Context(), refresh)
	p.Done()
	if err != nil && idx == nil {
		return nil, err
	}
	if stale {
		c.Warn("cannot reach the mod catalogue; using the copy from %s", Ago(at))
	} else if !at.IsZero() {
		c.Verbosef("Mod catalogue from %s", Ago(at))
	}
	return idx, nil
}

// fetchPackage loads one catalogue package, fresh: directly by its id,
// then through the (cached) catalogue for other spellings. The READMEs come
// with a direct fetch only.
func modFetchPackage(c *Ctx, ref lipindex.Reference) (*lipindex.Package, []lipindex.Readme, error) {
	p := c.Progress()
	p.Phase("Looking up %s", lipShortID(ref.ID))
	d, err := modOps.fetchDetail(c.Context(), ref.ID)
	p.Done()
	if err == nil {
		if pkg, ok := lipindex.ParsePackage(d); ok {
			return pkg, d.Readmes, nil
		}
		return nil, nil, modNotClientMod(ref.ID)
	}
	if !errors.Is(err, lipindex.ErrPackageNotFound) {
		return nil, nil, modCatalogError(err)
	}
	// The catalogue's paths are case-sensitive; find the exact spelling.
	idx, cerr := modLoadCatalog(c, false)
	if cerr != nil {
		return nil, nil, modCatalogError(cerr)
	}
	pkg, ok := idx.Find(ref.ID)
	if !ok {
		return nil, nil, modUnknownPackage(ref.ID, idx)
	}
	if d, err := modOps.fetchDetail(c.Context(), pkg.Identifier); err == nil {
		if fresh, ok := lipindex.ParsePackage(d); ok {
			return fresh, d.Readmes, nil
		}
	}
	return pkg, nil, nil
}

func modFirstNonEmpty(s ...string) string {
	for _, x := range s {
		if strings.TrimSpace(x) != "" {
			return x
		}
	}
	return ""
}

func modNotClientMod(id string) *Error {
	return Errorf(ExitNotFound, "ERR_LIP_PACKAGE_NOT_FOUND", "%s has no build for the client on %s (no variant has an entry with side client and platform %s)",
		lipShortID(id), lipindex.TargetPlatform, lipindex.TargetPlatform).
		WithHint("%s mod search <words>", ProgramName)
}

func modUnknownPackage(id string, idx *lipindex.Index) *Error {
	e := Errorf(ExitNotFound, "ERR_LIP_PACKAGE_NOT_FOUND", "the mod catalogue has no mod %s", lipShortID(id))
	if idx != nil {
		var names []string
		for _, p := range idx.Packages {
			names = append(names, lipShortID(p.Identifier))
		}
		if s := suggest(lipShortID(id), names); s != "" {
			return e.WithHint("did you mean %q?", s)
		}
		repo := id[strings.LastIndex(id, "/")+1:]
		return e.WithHint("%s mod search %s", ProgramName, quoteArg(repo))
	}
	return e.WithHint("%s mod search <words>", ProgramName)
}

// requireGDK refuses UWP instances for package operations.
func modRequireGDK(inst Instance) error {
	if inst.IsUWP() {
		return modBackendError("ERR_UWP_UNSUPPORTED_FEATURE").
			WithHint("install a GDK build: %s install <version>", ProgramName)
	}
	return nil
}

// modTarget is a mod reference resolved to folders of an instance.
type modTarget struct {
	ref     string
	folders []string
	mods    []installedMod
	// group is set when the reference names a catalogue package installed
	// in the instance, or a folder that belongs to one.
	group *lipindex.Group
	// packageID is set for a package reference ("github.com/owner/repo").
	packageID string
	// item is the mapping of a single local mod.
	item *lipindex.Item
}

func (t modTarget) label() string {
	if t.group != nil {
		return fmt.Sprintf("%s (%s)", t.group.PackageName, lipShortID(t.group.Identifier))
	}
	if len(t.mods) == 1 {
		m := t.mods[0]
		if !strings.EqualFold(m.Name, m.Folder) && m.Name != "" {
			return fmt.Sprintf("%s (folder %s)", m.Name, m.Folder)
		}
		return m.Folder
	}
	if t.packageID != "" {
		return lipShortID(t.packageID)
	}
	return t.ref
}

func (v *modView) modByFolder(folder string) (installedMod, bool) {
	for _, m := range v.all {
		if strings.EqualFold(m.Folder, folder) {
			return m, true
		}
	}
	return installedMod{}, false
}

func (v *modView) groupOf(folder string) *lipindex.Group {
	for i := range v.listing.Groups {
		for _, f := range v.listing.Groups[i].Folders {
			if strings.EqualFold(f, folder) {
				return &v.listing.Groups[i]
			}
		}
	}
	return nil
}

func (v *modView) itemOf(folder string) *lipindex.Item {
	for i := range v.listing.Items {
		if strings.EqualFold(v.listing.Items[i].Folder, folder) {
			return &v.listing.Items[i]
		}
	}
	return nil
}

// errLoaderMod is returned for LeviLamina itself, which the loader
// commands manage.
func modErrLoader() *Error {
	return Errorf(ExitUsage, "ERR_LL_MANAGED_IN_VERSION_SETTINGS", "LeviLamina is managed with the loader commands, not as a mod").
		WithHint("%s loader --help", ProgramName)
}

// resolveMod resolves a reference: a folder, then a unique manifest name
// (both ignoring case), then a catalogue package. needCatalog reports a
// package-shaped reference that the view cannot resolve without the
// catalogue.
func (v *modView) resolveMod(ref string) (t modTarget, needCatalog bool, err error) {
	ref = strings.TrimSpace(ref)
	t.ref = ref
	if ref == "" {
		return t, false, UsageError("empty mod reference")
	}
	single := func(m installedMod) (modTarget, bool, error) {
		if lipindex.IsLeviLaminaMod(m.ModInfo) {
			return t, false, modErrLoader()
		}
		t.folders, t.mods = []string{m.Folder}, []installedMod{m}
		t.group = v.groupOf(m.Folder)
		t.item = v.itemOf(m.Folder)
		return t, false, nil
	}
	if m, ok := v.modByFolder(ref); ok {
		return single(m)
	}
	var byName []installedMod
	for _, m := range v.all {
		if strings.EqualFold(strings.TrimSpace(m.Name), ref) {
			byName = append(byName, m)
		}
	}
	switch len(byName) {
	case 1:
		return single(byName[0])
	case 0:
	default:
		folders := make([]string, len(byName))
		for i, m := range byName {
			folders[i] = m.Folder
		}
		return t, false, UsageError("several mods are named %q: folders %s", ref, strings.Join(folders, ", ")).
			WithHint("name the folder instead").withCode("ERR_AMBIGUOUS_MOD")
	}
	pref, isPackage := lipindex.ParseReference(ref)
	if isPackage {
		if lipindex.IsLeviLamina(pref.ID) {
			return t, false, modErrLoader()
		}
		t.packageID = pref.ID
		key := lipindex.LookupKey(pref.ID)
		for i := range v.listing.Groups {
			g := &v.listing.Groups[i]
			if lipindex.LookupKey(g.Identifier) == key {
				t.group = g
				t.folders = append([]string(nil), g.Folders...)
				for _, f := range g.Folders {
					if m, ok := v.modByFolder(f); ok {
						t.mods = append(t.mods, m)
					}
				}
				return t, false, nil
			}
		}
		// Local mods mapped to the package but not installed through it.
		for i := range v.listing.Items {
			it := &v.listing.Items[i]
			if it.State.Source == lipindex.SourceUnique && lipindex.LookupKey(it.State.Identifier) == key {
				t.folders = append(t.folders, it.Folder)
				if m, ok := v.modByFolder(it.Folder); ok {
					t.mods = append(t.mods, m)
				}
				if t.item == nil {
					t.item = it
				}
			}
		}
		if len(t.folders) > 0 {
			return t, false, nil
		}
		if v.index == nil {
			return t, true, nil
		}
	}
	names := []string{}
	for _, m := range v.visible {
		names = append(names, m.Folder)
		if m.Name != "" && !strings.EqualFold(m.Name, m.Folder) {
			names = append(names, m.Name)
		}
	}
	for _, g := range v.listing.Groups {
		names = append(names, lipShortID(g.Identifier))
	}
	e := Errorf(ExitNotFound, "ERR_MOD_NOT_FOUND", "no mod %q in instance %s", ref, v.inst.Name)
	if s := suggest(ref, names); s != "" {
		return t, false, e.WithHint("did you mean %q?", s)
	}
	return t, false, e.WithHint("%s mod list -i %s", ProgramName, quoteArg(v.inst.Name))
}

// resolveMods resolves references, loading the catalogue when one of them
// is a package that only the catalogue can resolve.
func (v *modView) resolveMods(c *Ctx, refs []string) ([]modTarget, error) {
	out := make([]modTarget, 0, len(refs))
	for _, ref := range refs {
		t, needCatalog, err := v.resolveMod(ref)
		if err == nil && needCatalog {
			if v.index == nil && v.catErr == nil {
				idx, lerr := modLoadCatalog(c, false)
				if lerr != nil {
					return nil, modCatalogError(lerr)
				}
				v.index = idx
				v.remap(c)
			} else if v.catErr != nil {
				return nil, v.catErr
			}
			t, needCatalog, err = v.resolveMod(ref)
			if err == nil && needCatalog {
				err = Errorf(ExitNotFound, "ERR_MOD_NOT_FOUND", "no mod %q in instance %s", ref, v.inst.Name).
					WithHint("%s mod list -i %s", ProgramName, quoteArg(v.inst.Name))
			}
		}
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// batch runs one step per item: results go to stdout as they finish,
// failures to stderr, and a summary closes a batch with failures (exit 7,
// or 1 when everything failed). A single item's failure is returned as is.
type modBatch struct {
	c       *Ctx
	total   int
	failed  int
	lastErr error
	results []modBatchResult
	ids     []string
}

// modBatchResult is one element of a batch command's --json output.
type modBatchResult struct {
	Item    string        `json:"item"`
	Status  string        `json:"status"`
	From    string        `json:"from,omitempty"`
	To      string        `json:"to,omitempty"`
	Message string        `json:"message,omitempty"`
	Error   *modJSONError `json:"error,omitempty"`
}

type modJSONError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

func newModBatch(c *Ctx, total int) *modBatch {
	return &modBatch{c: c, total: total, results: []modBatchResult{}}
}

// ok records a success; line is the human result ("enabled Foo").
func (b *modBatch) ok(r modBatchResult, id, line string) {
	b.results = append(b.results, r)
	if id != "" {
		b.ids = append(b.ids, id)
	}
	if !b.c.G.JSON && !b.c.G.Quiet && line != "" {
		b.c.Progress().Done()
		b.c.Printf("%s\n", line)
	}
}

// fail records a failure of item.
func (b *modBatch) fail(item string, err error) {
	e := FromError(err)
	b.failed++
	b.lastErr = e
	b.results = append(b.results, modBatchResult{Item: item, Status: "failed",
		Error: &modJSONError{Code: e.Code, Message: e.Message, Hint: e.Hint}})
	if b.total > 1 && !b.c.G.JSON {
		msg := e.Message
		if e.Hint != "" {
			msg += " (hint: " + e.Hint + ")"
		}
		b.c.errf("error: %s: %s [%s]\n", item, msg, e.Code)
	}
}

// interrupted reports whether the command was cancelled (Ctrl-C), after
// which the remaining items are skipped.
func (b *modBatch) interrupted() bool { return b.c.Context().Err() != nil }

// done finishes the batch.
func (b *modBatch) done() error {
	if b.total == 1 && b.failed == 1 {
		return b.lastErr
	}
	if b.c.G.JSON {
		if err := b.c.JSON(b.results); err != nil {
			return err
		}
	} else if b.c.G.Quiet {
		for _, id := range b.ids {
			b.c.Printf("%s\n", id)
		}
	}
	if b.failed == 0 {
		return nil
	}
	if b.c.Context().Err() != nil {
		return b.c.Context().Err()
	}
	if !b.c.G.JSON {
		b.c.errf("%d of %d failed\n", b.failed, b.total)
	}
	if b.failed == b.total {
		return ExitCode(ExitFailure)
	}
	return ExitCode(ExitPartial)
}

// modSoftInstance resolves the instance when one is named or there is a
// default, without failing when there is none (for commands that work
// without an instance).
func modSoftInstance(c *Ctx) (Instance, bool, error) {
	if strings.TrimSpace(c.G.Instance) != "" {
		inst, err := c.ResolveInstance("")
		return inst, err == nil, err
	}
	name, _, err := c.DefaultInstance()
	if err != nil || name == "" {
		return Instance{}, false, nil
	}
	if _, err := LookupInstance(name); err != nil {
		return Instance{}, false, nil
	}
	inst, err := c.ResolveInstance("")
	if err != nil {
		return Instance{}, false, nil
	}
	return inst, true, nil
}

// loadLoaderDB fetches and normalises the LeviLamina version database.
func modLoaderDB(c *Ctx) (lipindex.LoaderDB, error) {
	p := c.Progress()
	p.Phase("Loading the LeviLamina version list")
	raw, err := modOps.loaderDB()
	p.Done()
	if err != nil {
		return nil, modBackendError("ERR_FETCH_LL_DB").WithDetail(err.Error())
	}
	return lipindex.NormalizeLoaderDB(raw), nil
}
