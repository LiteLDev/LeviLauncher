package cli

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/backupmods"
	"github.com/liteldev/LeviLauncher/internal/launch"
	"github.com/liteldev/LeviLauncher/internal/mcservice"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

// Backup scopes as the command line names them, and as the backend does.
const (
	bkScopeGameData = "game-data"
	bkScopeMods     = "mods"

	bkBackendGameData = "gameData"
	bkBackendMods     = "mods"

	bkModeFull = "full"

	bkChoiceBackup  = "backup"
	bkChoiceCurrent = "current"
)

// Seams replaced by tests.
var (
	bkInfo        = mcservice.GetInstanceBackupInfo
	bkCreate      = mcservice.BackupInstance
	bkInspect     = mcservice.InspectInstanceBackupArchive
	bkPreview     = mcservice.PreviewInstanceBackupRestoreConflicts
	bkRestore     = mcservice.RestoreInstanceBackup
	bkPackages    = backupmods.InstalledPackages
	bkScanMods    = backupmods.ScanMods
	bkGameRunning = func(dir string) bool { return len(launch.GameProcessIDs(dir)) > 0 }
)

func init() {
	Register(&Command{
		Path:    "backup create",
		Summary: "back up an instance's game data and mods",
		Args:    "[<instance>]",
		Description: `Writes a .zip archive to the launcher's backups folder and prints its path.
By default every scope that exists is backed up: game-data (worlds, packs and
settings; --full for all game data) and, on GDK instances, mods. Mods
installed from the LIP catalogue are recorded by name and version, so a
restore reinstalls them; other mods are stored as folders.`,
		Examples: []string{"backup create", "backup create Survival --scope game-data --full", "backup create -i Survival --json"},
		Flags: []*Flag{
			EnumsFlag("scope", "", []string{bkScopeGameData, bkScopeMods}, "what to back up (default: everything that exists)"),
			BoolFlag("full", "", "back up all game data, not only worlds, packs and settings"),
		},
		MaxArgs: 1,
		Run:     runBackupCreate,
	})
	Register(&Command{
		Path:    "backup list",
		Summary: "list instance and world backups",
		Args:    "[<instance>]",
		Description: `Lists instance backups (backup create) and world backups (.mcworld files saved
by the launcher window), newest first. Without an instance (argument or -i)
it lists the backups of every instance, including removed ones.`,
		Examples: []string{"backup list", "backup list Survival", "backup ls -q"},
		MaxArgs:  1,
		Run:      runBackupList,
	})
	Register(&Command{
		Path:    "backup restore",
		Summary: "restore an instance backup",
		Args:    "[<instance>] <archive.zip>",
		Description: `Restores the scopes of an archive (default: all it holds) into an instance
(default: the instance the archive was made from). Where the archive and the
instance both have a world, pack, mod or file, keep one of them: --prefer
decides for all, --resolve for one conflict (IDs from --dry-run), and on a
console the others are asked. Restoring mods reinstalls the recorded LIP
packages, which needs the network.

Exit status: 0 restored, 7 partly restored, 4 the game is running or the
archive is for another kind of instance (GDK/UWP).`,
		Examples: []string{
			"backup restore Survival_20260101-120000.zip --dry-run",
			"backup restore Survival Survival_20260101-120000.zip --prefer backup --yes",
			"backup restore Survival_20260101-120000.zip --prefer backup --resolve mods::mod_folder::LegacyMoney::LegacyMoney=current -y",
		},
		Flags: []*Flag{
			EnumsFlag("scope", "", []string{bkScopeGameData, bkScopeMods}, "what to restore (default: everything in the archive)"),
			EnumFlag("prefer", "", []string{bkChoiceBackup, bkChoiceCurrent}, "keep this side of every conflict not given with --resolve"),
			StringsFlag("resolve", "", "<id>=backup|current", "keep one side of one conflict"),
			BoolFlag("dry-run", "n", "list the conflicts and change nothing"),
		},
		MinArgs: 1, MaxArgs: 2,
		Run: runBackupRestore,
	})
	RegisterBackendErrorsFor([]string{"backup"}, map[string]BackendError{
		"ERR_INSTANCE_BACKUP_INSTANCE_NOT_FOUND":  {Exit: ExitNotFound, Message: "the instance does not exist"},
		"ERR_INSTANCE_BACKUP_INVALID_SCOPE":       {Exit: ExitUsage, Message: "the archive or instance has no such scope"},
		"ERR_INSTANCE_BACKUP_NO_SCOPE":            {Exit: ExitUsage, Message: "there is nothing to back up or restore"},
		"ERR_INSTANCE_BACKUP_INVALID_SCOPE_MODE":  {Exit: ExitUsage, Message: "this game data cannot be backed up that way"},
		"ERR_INSTANCE_BACKUP_GAME_DATA_NOT_FOUND": {Exit: ExitNotFound, Message: "the instance has no game data yet", Hint: "start the game once, then back it up"},
		"ERR_INSTANCE_BACKUP_CREATE_DIR":          {Exit: ExitFailure, Message: "cannot create the backups folder"},
		"ERR_INSTANCE_BACKUP_CREATE_ARCHIVE":      {Exit: ExitFailure, Message: "cannot create the backup archive"},
		"ERR_INSTANCE_BACKUP_WRITE_ARCHIVE":       {Exit: ExitFailure, Message: "writing the backup archive failed"},
		"ERR_INSTANCE_BACKUP_READ_SOURCE":         {Exit: ExitFailure, Message: "reading the instance's files failed (is the game running?)"},
		"ERR_INSTANCE_BACKUP_MODS_STATE_UNAVAILABLE": {Exit: ExitConflict, Message: "the installed mod packages cannot be read, so mods cannot be backed up",
			Hint: ProgramName + " backup create --scope game-data"},
		"ERR_INSTANCE_BACKUP_ARCHIVE_OPEN":                        {Exit: ExitFailure, Message: "cannot open the archive"},
		"ERR_INSTANCE_BACKUP_INVALID_ARCHIVE":                     {Exit: ExitFailure, Message: "the file is not a LeviLauncher instance backup"},
		"ERR_INSTANCE_BACKUP_PACKAGE_TYPE_MISMATCH":               {Exit: ExitConflict, Message: "the backup is for another kind of instance (GDK or UWP)"},
		"ERR_INSTANCE_BACKUP_RESTORE_EXTRACT_ARCHIVE":             {Exit: ExitFailure, Message: "extracting the archive failed"},
		"ERR_INSTANCE_BACKUP_RESTORE_WRITE_TARGET":                {Exit: ExitFailure, Message: "writing into the instance failed"},
		"ERR_INSTANCE_BACKUP_RESTORE_INVALID_CONFLICT_RESOLUTION": {Exit: ExitUsage, Message: "a conflict decision names no conflict of this restore"},
		"ERR_INSTANCE_BACKUP_RESTORE_CONFLICT_UNRESOLVED":         {Exit: ExitUsage, Message: "some conflicts have no decision", Hint: "add --prefer backup or --prefer current"},
		"ERR_INSTANCE_BACKUP_RESTORE_FULL_REQUIRES_EMPTY_TARGET":  {Exit: ExitConflict, Message: "a full game-data backup can only be restored into empty game data"},
		"ERR_INSTANCE_BACKUP_RESTORE_LIP_INSTALL_FAILED":          {Exit: ExitNetwork, Message: "reinstalling the recorded mod packages failed"},
		"ERR_INSTANCE_BACKUP_RESTORE_LIP_MISSING":                 {Exit: ExitFailure, Message: "some recorded mod packages are not installed after the restore"},
		"ERR_UWP_UNSUPPORTED_FEATURE":                             {Exit: ExitConflict, Message: "UWP instances have no mods"},
	})
}

// toBackendScopes maps game-data/mods to the backend's keys.
func toBackendScopes(scopes []string) []string {
	out := make([]string, 0, len(scopes))
	for _, s := range scopes {
		switch strings.ToLower(s) {
		case bkScopeGameData:
			out = append(out, bkBackendGameData)
		case bkScopeMods:
			out = append(out, bkBackendMods)
		}
	}
	return out
}

// cliScope maps a backend scope key to its command-line name.
func cliScope(key string) string {
	if strings.EqualFold(key, bkBackendGameData) {
		return bkScopeGameData
	}
	return strings.ToLower(key)
}

func cliScopes(keys []string) []string {
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, cliScope(k))
	}
	return out
}

// backupCreateResult is `backup create --json`.
type backupCreateResult struct {
	Instance       string   `json:"instance"`
	ArchivePath    string   `json:"archivePath"`
	BackupDir      string   `json:"backupDir"`
	IncludedScopes []string `json:"includedScopes"`
	Files          int      `json:"files"`
	Bytes          int64    `json:"bytes"`
	ModPackages    int      `json:"modPackages"`
}

// planBackupScopes picks the scopes and modes of a backup from the
// instance's backup info, as the window does: by default every scope that
// is selectable and exists.
func planBackupScopes(info types.InstanceBackupInfo, requested []string, full bool) ([]string, map[string]string, error) {
	byKey := map[string]types.InstanceBackupScope{}
	for _, s := range info.Scopes {
		byKey[s.Key] = s
	}
	var keys []string
	if len(requested) == 0 {
		for _, s := range info.Scopes {
			if s.Selectable && s.Exists {
				keys = append(keys, s.Key)
			}
		}
		if len(keys) == 0 {
			return nil, nil, Errorf(ExitNotFound, "ERR_INSTANCE_BACKUP_NO_SCOPE", "instance %s has no game data or mods to back up yet", info.Name)
		}
	} else {
		for _, k := range toBackendScopes(requested) {
			s, ok := byKey[k]
			if !ok || !s.Selectable || !s.Exists {
				if k == bkBackendMods {
					return nil, nil, ConflictError("instance %s has no mods to back up", info.Name).
						WithHint("UWP instances have no mods; back up the game data with --scope game-data").
						withCode("ERR_INSTANCE_BACKUP_INVALID_SCOPE")
				}
				return nil, nil, Backend("ERR_INSTANCE_BACKUP_GAME_DATA_NOT_FOUND")
			}
			keys = append(keys, k)
		}
	}
	modes := map[string]string{}
	hasGameData := false
	for _, k := range keys {
		s := byKey[k]
		if len(s.Modes) == 0 {
			continue
		}
		mode := strings.TrimSpace(s.DefaultMode)
		if mode == "" && len(s.Modes) > 0 {
			mode = s.Modes[0].Key
		}
		if k == bkBackendGameData {
			hasGameData = true
			if full {
				mode = bkModeFull
			}
		}
		modes[k] = mode
	}
	if full && !hasGameData {
		return nil, nil, UsageError("--full applies to game data; add --scope game-data")
	}
	return keys, modes, nil
}

func runBackupCreate(c *Ctx) error {
	inst, err := c.ResolveInstance(c.Arg(0))
	if err != nil {
		return err
	}
	info := bkInfo(inst.Name)
	if e := Backend(info.ErrorCode); e != nil {
		return e
	}
	keys, modes, err := planBackupScopes(info, c.Strings("scope"), c.Bool("full"))
	if err != nil {
		return err
	}
	req := types.InstanceBackupRequest{Scopes: keys, ScopeModes: modes, ModsLIPPackages: []types.InstanceBackupModsLIPPackage{}}
	for _, s := range info.Scopes {
		if s.Key == bkBackendGameData && s.Shared && containsFold(keys, bkBackendGameData) {
			c.Warn("instance %s shares its game data with other instances; the backup holds that shared data", inst.Name)
		}
	}
	if containsFold(keys, bkBackendMods) {
		c.Progress().Phase("Reading the installed mod packages")
		pkgs, err := bkPackages(c.Context(), inst.Dir)
		if err == nil {
			req.ModsLIPPackages, err = backupmods.Build(pkgs, bkScanMods(filepath.Join(inst.Dir, "mods")))
		}
		if err != nil {
			if c.Context().Err() != nil {
				return c.Context().Err()
			}
			return Backend("ERR_INSTANCE_BACKUP_MODS_STATE_UNAVAILABLE").WithDetail(err.Error())
		}
	}

	var last types.InstanceBackupCreateProgress
	prog := c.Progress()
	prog.Listen(func(_ string, data any) {
		if p, ok := data.(types.InstanceBackupCreateProgress); ok {
			last = p
		}
	})
	prog.Phase("Backing up %s", inst.Name)
	// The backend cannot be cancelled: Ctrl-C waits for a complete archive
	// (a second Ctrl-C ends the process, leaving no archive).
	res := bkCreate(inst.Name, req)
	prog.Done()
	if e := Backend(res.ErrorCode); e != nil {
		return e
	}
	out := backupCreateResult{
		Instance: inst.Name, ArchivePath: res.ArchivePath, BackupDir: res.BackupDir,
		IncludedScopes: cliScopes(res.IncludedScopes), Files: last.TotalFiles, Bytes: last.TotalBytes,
		ModPackages: len(req.ModsLIPPackages),
	}
	if fi, err := os.Stat(out.ArchivePath); err == nil && out.Bytes == 0 {
		out.Bytes = fi.Size()
	}
	return c.Render(out, []string{out.ArchivePath}, func(w io.Writer) error {
		c.Notice("Backed up %s (%s) to:", inst.Name, strings.Join(out.IncludedScopes, ", "))
		_, err := fmt.Fprintln(w, out.ArchivePath)
		return err
	})
}

// backupItem is an element of `backup list --json`.
type backupItem struct {
	Instance    string   `json:"instance"`
	Kind        string   `json:"kind"` // instance or world
	Created     string   `json:"created"`
	Size        int64    `json:"size"`
	Path        string   `json:"path"`
	GameVersion string   `json:"gameVersion,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
	World       string   `json:"world,omitempty"`

	created time.Time
}

func runBackupList(c *Ctx) error {
	name := strings.TrimSpace(c.Arg(0))
	flag := strings.TrimSpace(c.G.Instance)
	if name != "" && flag != "" && !strings.EqualFold(name, flag) {
		return UsageError("instance %q given twice: as an argument and as --instance %q", name, flag)
	}
	if name == "" {
		name = flag
	}
	items := append(listInstanceBackups(name), listWorldBackups(name)...)
	sort.SliceStable(items, func(i, j int) bool { return items[i].created.After(items[j].created) })
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.Path
	}
	return c.Render(items, ids, func(w io.Writer) error {
		if len(items) == 0 {
			if name != "" {
				c.Notice("No backups of %s", name)
			} else {
				c.Notice("No backups")
			}
			return nil
		}
		rows := make([][]string, 0, len(items))
		for _, it := range items {
			rows = append(rows, []string{Dash(it.Instance), it.Kind, Ago(it.created), Size(it.Size), it.Path})
		}
		return Table(w, []string{"INSTANCE", "KIND", "CREATED", "SIZE", "PATH"}, rows)
	})
}

// sanitizeBackupSegment names a folder as the backend names an instance's
// backup folder (mcservice.sanitizeInstanceBackupSegment).
func sanitizeBackupSegment(name string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(name) {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			b.WriteRune('_')
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimRight(b.String(), " .")
}

func listInstanceBackups(filter string) []backupItem {
	out := []backupItem{}
	root := apppath.InstanceBackupsDir()
	dirs, err := os.ReadDir(root)
	if err != nil {
		return out
	}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(root, d.Name()))
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.EqualFold(filepath.Ext(f.Name()), ".zip") {
				continue
			}
			p := filepath.Join(root, d.Name(), f.Name())
			fi, err := f.Info()
			if err != nil {
				continue
			}
			it := backupItem{Instance: d.Name(), Kind: "instance", Size: fi.Size(), Path: p, created: fi.ModTime()}
			if info := bkInspect(p); info.ErrorCode == "" {
				if info.Name != "" {
					it.Instance = info.Name
				}
				it.GameVersion = info.GameVersion
				it.Scopes = cliScopes(info.IncludedScopes)
				if t, err := time.Parse(time.RFC3339Nano, info.CreatedAt); err == nil {
					it.created = t
				}
			}
			if filter != "" && !strings.EqualFold(it.Instance, filter) && !strings.EqualFold(d.Name(), sanitizeBackupSegment(filter)) {
				continue
			}
			it.Created = it.created.UTC().Format(time.RFC3339)
			out = append(out, it)
		}
	}
	return out
}

// listWorldBackups lists the .mcworld files the window's world backup
// writes: <Worlds>\<instance>\<folder>_<world>\<world>_<time>.mcworld, and
// the older <Worlds>\<world>\<world>_<time>.mcworld without an instance.
func listWorldBackups(filter string) []backupItem {
	out := []backupItem{}
	root := apppath.WorldBackupsDir()
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(d.Name()), ".mcworld") {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		inst := ""
		if len(parts) == 3 {
			inst = parts[0]
		}
		if filter != "" && !strings.EqualFold(inst, utils.SanitizeFilename(filter)) {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		world := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		if i := strings.LastIndex(world, "_"); i > 0 {
			world = world[:i]
		}
		out = append(out, backupItem{Instance: inst, Kind: "world", Size: fi.Size(), Path: p, World: world,
			created: fi.ModTime(), Created: fi.ModTime().UTC().Format(time.RFC3339)})
		return nil
	})
	return out
}

// restoreConflict is a conflict of `backup restore --dry-run --json`.
type restoreConflict struct {
	ID       string            `json:"id"`
	Scope    string            `json:"scope"`
	Kind     string            `json:"kind"` // world, pack, mod or file
	Item     string            `json:"item"`
	Backup   string            `json:"backup"`
	Current  string            `json:"current"`
	Differs  map[string]string `json:"differs,omitempty"` // field: "backup -> current"
	Decision string            `json:"decision"`          // backup, current or "" (undecided)
}

// restorePlan is `backup restore --dry-run --json`.
type restorePlan struct {
	Instance    string            `json:"instance"`
	ArchivePath string            `json:"archivePath"`
	Scopes      []string          `json:"scopes"`
	Conflicts   []restoreConflict `json:"conflicts"`
	Resolutions map[string]string `json:"resolutions"`
}

// restoreResult is `backup restore --json`.
type restoreResult struct {
	Instance     string               `json:"instance"`
	ArchivePath  string               `json:"archivePath"`
	Status       string               `json:"status"` // success, partial or failed
	ScopeResults []restoreScopeResult `json:"scopeResults"`
}

type restoreScopeResult struct {
	Scope    string   `json:"scope"`
	Status   string   `json:"status"`
	Error    string   `json:"error,omitempty"`
	Warnings []string `json:"warnings"`
	Details  []string `json:"details"`
}

func conflictKind(identityKind string) string {
	switch identityKind {
	case "world_folder":
		return "world"
	case "pack_uuid":
		return "pack"
	case "mod_folder":
		return "mod"
	}
	return "file"
}

func toRestoreConflict(cf types.InstanceBackupRestoreConflict) restoreConflict {
	rc := restoreConflict{ID: cf.ID, Scope: cliScope(cf.ScopeKey), Kind: conflictKind(cf.IdentityKind),
		Item: cf.Path, Backup: cf.BackupSummary, Current: cf.CurrentSummary}
	for _, d := range cf.DiffFields {
		if rc.Differs == nil {
			rc.Differs = map[string]string{}
		}
		rc.Differs[d.Key] = Dash(d.BackupValue) + " -> " + Dash(d.CurrentValue)
	}
	return rc
}

// decideConflicts applies --resolve (id=choice) and then --prefer to the
// conflicts. An --resolve naming no conflict is a usage error.
func decideConflicts(conflicts []restoreConflict, resolves []string, prefer string) error {
	byID := map[string]int{}
	for i, cf := range conflicts {
		byID[cf.ID] = i
	}
	for _, r := range resolves {
		i := strings.LastIndex(r, "=")
		if i <= 0 {
			return UsageError("--resolve %q: use <id>=backup or <id>=current", r)
		}
		id, choice := strings.TrimSpace(r[:i]), strings.ToLower(strings.TrimSpace(r[i+1:]))
		if choice != bkChoiceBackup && choice != bkChoiceCurrent {
			return UsageError("--resolve %q: the choice must be backup or current", r)
		}
		idx, ok := byID[id]
		if !ok {
			return UsageError("--resolve names no conflict of this restore: %q", id).
				WithHint("list the conflicts with --dry-run").withCode("ERR_UNKNOWN_CONFLICT")
		}
		conflicts[idx].Decision = choice
	}
	if prefer != "" {
		for i := range conflicts {
			if conflicts[i].Decision == "" {
				conflicts[i].Decision = prefer
			}
		}
	}
	return nil
}

func undecided(conflicts []restoreConflict) []restoreConflict {
	var out []restoreConflict
	for _, cf := range conflicts {
		if cf.Decision == "" {
			out = append(out, cf)
		}
	}
	return out
}

// askConflicts asks for a decision on each undecided conflict on the
// console, offering to apply an answer to all remaining ones.
func askConflicts(c *Ctx, conflicts []restoreConflict) error {
	if c.in == nil {
		c.in = bufio.NewReader(c.Stdin)
	}
	open := len(undecided(conflicts))
	n := 0
	for i := range conflicts {
		if conflicts[i].Decision != "" {
			continue
		}
		n++
		cf := conflicts[i]
		c.errf("Conflict %d/%d (%s %s): %s\n  backup:  %s\n  current: %s\n", n, open, cf.Scope, cf.Kind, cf.Item, Dash(cf.Backup), Dash(cf.Current))
		keys := make([]string, 0, len(cf.Differs))
		for k := range cf.Differs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			c.errf("  %s: %s\n", k, cf.Differs[k])
		}
		for {
			c.errf("Keep [b]ackup, [c]urrent, [B]ackup for all, [C]urrent for all? ")
			line, err := c.in.ReadString('\n')
			answer := strings.TrimSpace(line)
			all := ""
			switch answer {
			case "b", "backup":
				conflicts[i].Decision = bkChoiceBackup
			case "c", "current":
				conflicts[i].Decision = bkChoiceCurrent
			case "B":
				all = bkChoiceBackup
			case "C":
				all = bkChoiceCurrent
			}
			if all != "" {
				for j := i; j < len(conflicts); j++ {
					if conflicts[j].Decision == "" {
						conflicts[j].Decision = all
					}
				}
				return nil
			}
			if conflicts[i].Decision != "" {
				break
			}
			if err != nil || c.Context().Err() != nil {
				return Errorf(ExitCancelled, "ERR_CANCELED", "cancelled")
			}
		}
	}
	return nil
}

// restoreTarget picks the instance of a restore: the one named (argument or
// -i), else the archive's own instance when it is installed.
func restoreTarget(c *Ctx, named string, info types.InstanceBackupArchiveInfo) (Instance, error) {
	if strings.TrimSpace(named) != "" || strings.TrimSpace(c.G.Instance) != "" {
		return c.ResolveInstance(named)
	}
	if n := strings.TrimSpace(info.Name); n != "" {
		if inst, err := LookupInstance(n); err == nil {
			c.Notice("Using instance %s (the backup's instance)", inst.Name)
			return inst, nil
		}
		return Instance{}, UsageError("the backup's instance %q is not installed", n).
			WithHint("name the instance to restore into: %s backup restore <instance> <archive.zip>", ProgramName).
			withCode("ERR_INSTANCE_REQUIRED")
	}
	return c.ResolveInstance("")
}

func runBackupRestore(c *Ctx) error {
	named, archive := "", c.Arg(0)
	if len(c.Args) == 2 {
		named, archive = c.Arg(0), c.Arg(1)
	}
	abs, err := filepath.Abs(strings.TrimSpace(archive))
	if err != nil {
		return UsageError("invalid path %q", archive)
	}
	if fi, err := os.Stat(abs); err != nil || fi.IsDir() {
		return NotFoundError("no backup archive at %s", abs).WithHint("%s backup list", ProgramName).withCode("ERR_ARCHIVE_NOT_FOUND")
	}
	info := bkInspect(abs)
	if e := Backend(info.ErrorCode); e != nil {
		return e
	}
	inst, err := restoreTarget(c, named, info)
	if err != nil {
		return err
	}
	scopes := toBackendScopes(c.Strings("scope"))
	for _, s := range scopes {
		if !containsFold(info.IncludedScopes, s) {
			return UsageError("the archive holds no %s (it holds: %s)", cliScope(s), strings.Join(cliScopes(info.IncludedScopes), ", ")).
				withCode("ERR_INSTANCE_BACKUP_INVALID_SCOPE")
		}
	}
	dry := c.Bool("dry-run")
	if !dry {
		restoring := scopes
		if len(restoring) == 0 {
			restoring = info.IncludedScopes
		}
		// Game data may be shared with other instances or the Store game,
		// so any Minecraft using it counts, not only this instance's own.
		if containsFold(restoring, bkBackendGameData) {
			if err := contentEnsureStopped(inst, "game data"); err != nil {
				return err
			}
		}
		if bkGameRunning(inst.Dir) {
			return ConflictError("Minecraft is running for instance %s", inst.Name).
				WithHint("%s stop %s", ProgramName, quoteArg(inst.Name)).withCode("ERR_GAME_RUNNING")
		}
	}

	prog := c.Progress()
	prog.Phase("Comparing the backup with %s", inst.Name)
	pv := bkPreview(inst.Name, types.InstanceBackupRestoreRequest{ArchivePath: abs, Scopes: scopes})
	prog.Done()
	if err := c.Context().Err(); err != nil {
		return err
	}
	if e := Backend(pv.ErrorCode); e != nil {
		return e
	}
	conflicts := make([]restoreConflict, 0, len(pv.Conflicts))
	for _, cf := range pv.Conflicts {
		conflicts = append(conflicts, toRestoreConflict(cf))
	}
	if err := decideConflicts(conflicts, c.Strings("resolve"), c.String("prefer")); err != nil {
		return err
	}
	plan := restorePlan{Instance: inst.Name, ArchivePath: abs, Scopes: cliScopes(pv.IncludedScopes),
		Conflicts: conflicts, Resolutions: map[string]string{}}
	for _, cf := range conflicts {
		plan.Resolutions[cf.ID] = cf.Decision
	}
	if dry {
		return renderRestorePlan(c, plan)
	}

	if open := undecided(conflicts); len(open) > 0 {
		if !c.CanPrompt() {
			ids := make([]string, 0, 3)
			for i, cf := range open {
				if i == 3 {
					ids = append(ids, "...")
					break
				}
				ids = append(ids, cf.ID)
			}
			return UsageError("%d conflict(s) need a decision: %s", len(open), strings.Join(ids, ", ")).
				WithHint("add --prefer backup or --prefer current, or --resolve <id>=backup|current (see --dry-run)").
				withCode("ERR_INSTANCE_BACKUP_RESTORE_CONFLICT_UNRESOLVED")
		}
		if err := askConflicts(c, conflicts); err != nil {
			return err
		}
	}
	keepBackup, keepCurrent := 0, 0
	resolutions := make([]types.InstanceBackupRestoreResolution, 0, len(conflicts))
	for _, cf := range conflicts {
		if cf.Decision == bkChoiceBackup {
			keepBackup++
		} else {
			keepCurrent++
		}
		resolutions = append(resolutions, types.InstanceBackupRestoreResolution{ConflictID: cf.ID, Choice: cf.Decision})
	}
	q := fmt.Sprintf("Restore %s from %s into instance %s", strings.Join(plan.Scopes, " and "), filepath.Base(abs), inst.Name)
	if len(conflicts) > 0 {
		q += fmt.Sprintf(" (replacing %d item(s) with the backup's, keeping %d)", keepBackup, keepCurrent)
	}
	if err := c.Confirm(q + "?"); err != nil {
		return err
	}

	prog.Phase("Restoring %s", inst.Name)
	// A restore is not stopped half-way: Ctrl-C only cancels the mod
	// package reinstall, which the result then reports.
	rr := bkRestore(c.Context(), inst.Name, types.InstanceBackupRestoreRequest{
		ArchivePath: abs, Scopes: pv.IncludedScopes, ConflictResolutions: resolutions})
	prog.Done()
	return finishRestore(c, inst.Name, abs, rr)
}

func renderRestorePlan(c *Ctx, plan restorePlan) error {
	ids := make([]string, len(plan.Conflicts))
	for i, cf := range plan.Conflicts {
		ids[i] = cf.ID
	}
	return c.Render(plan, ids, func(w io.Writer) error {
		if len(plan.Conflicts) == 0 {
			_, err := fmt.Fprintf(w, "No conflicts: restoring %s into %s replaces nothing\n", strings.Join(plan.Scopes, " and "), plan.Instance)
			return err
		}
		rows := make([][]string, 0, len(plan.Conflicts))
		for _, cf := range plan.Conflicts {
			rows = append(rows, []string{cf.ID, cf.Scope, cf.Kind, cf.Item, Dash(cf.Backup), Dash(cf.Current), Dash(cf.Decision)})
		}
		return Table(w, []string{"ID", "SCOPE", "KIND", "ITEM", "BACKUP", "CURRENT", "DECISION"}, rows)
	})
}

// restoreWarningText renders the backend's restore warnings.
var restoreWarningText = map[string]string{
	"ERR_LL_NOT_SUPPORTED":                             "LeviLamina does not support this game version; the recorded mod packages were not reinstalled",
	"ERR_INSTANCE_BACKUP_RESTORE_LIP_INSTALL_FAILED":   "some recorded mod packages could not be reinstalled",
	"ERR_INSTANCE_BACKUP_RESTORE_LIP_VERSION_MISMATCH": "some mod packages were reinstalled at another version",
	"ERR_INSTANCE_BACKUP_RESTORE_LIP_MISSING":          "some recorded mod packages are missing after the restore",
	"versions.edit.backup.shared_warning":              "the game data is shared with other instances",
	"versions.edit.backup.mode.full.warning":           "the backup holds all game data",
}

func warningText(w string) string {
	if t, ok := restoreWarningText[w]; ok {
		return t
	}
	if e := Backend(w); e != nil && strings.HasPrefix(w, "ERR_") {
		return e.Message
	}
	return w
}

func finishRestore(c *Ctx, instance, archive string, r types.InstanceBackupRestoreResult) error {
	out := restoreResult{Instance: instance, ArchivePath: archive, Status: r.Status, ScopeResults: []restoreScopeResult{}}
	for _, s := range r.ScopeResults {
		sr := restoreScopeResult{Scope: cliScope(s.Key), Status: s.Status, Warnings: []string{}, Details: append([]string{}, s.Details...)}
		if s.ErrorCode != "" {
			sr.Error = s.ErrorCode
		}
		for _, w := range s.Warnings {
			sr.Warnings = append(sr.Warnings, warningText(w))
		}
		out.ScopeResults = append(out.ScopeResults, sr)
	}
	if len(r.ScopeResults) == 0 && r.ErrorCode != "" {
		return Backend(r.ErrorCode)
	}
	err := c.Render(out, []string{out.Status}, func(w io.Writer) error {
		rows := make([][]string, 0, len(out.ScopeResults))
		for _, s := range out.ScopeResults {
			note := strings.Join(s.Warnings, "; ")
			if s.Error != "" && note == "" {
				note = warningText(s.Error)
			}
			rows = append(rows, []string{s.Scope, s.Status, Dash(note)})
		}
		if err := Table(w, []string{"SCOPE", "STATUS", "NOTE"}, rows); err != nil {
			return err
		}
		for _, s := range out.ScopeResults {
			for _, d := range s.Details {
				c.Verbosef("%s: %s", s.Scope, d)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	switch r.Status {
	case "success":
		return nil
	case "partial":
		if !c.G.JSON {
			c.Warn("the backup was only partly restored")
		}
		return ExitCode(ExitPartial)
	}
	code := r.ErrorCode
	if code == "" {
		code = "ERR_INSTANCE_BACKUP_RESTORE_WRITE_TARGET"
	}
	return Backend(code)
}
