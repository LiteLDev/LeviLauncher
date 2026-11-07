package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/config"
	"github.com/liteldev/LeviLauncher/internal/mcservice"
	"github.com/liteldev/LeviLauncher/internal/nativeinstall"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/liteldev/LeviLauncher/internal/versions"
	"github.com/liteldev/LeviLauncher/internal/xbox"
)

// Environment variables naming the default instance and player of a shell.
const (
	EnvInstance = "LEVILAUNCHER_INSTANCE"
	EnvPlayer   = "LEVILAUNCHER_PLAYER"
)

// Instance is an installed instance.
type Instance struct {
	Name        string               // folder name: the instance's identity
	Dir         string               // absolute folder
	Meta        versions.VersionMeta // settings (effective values, see versions.ReadMeta)
	PackageType string               // versions.PackageTypeGDK or PackageTypeUWP
	Channel     string               // release, beta or preview
}

// IsUWP reports whether the instance is a UWP (appx) install.
func (i Instance) IsUWP() bool { return i.PackageType == versions.PackageTypeUWP }

// IsPreview reports whether the instance is on the preview channel.
func (i Instance) IsPreview() bool { return strings.EqualFold(i.Channel, "preview") }

// Seams replaced by tests.
var (
	listInstances      = scanInstances
	defaultInstanceCfg = config.GetCurrentInstance
	usersRootOf        = func(inst Instance) string { return mcservice.GetContentRoots(inst.Name).UsersRoot }
	gamertagCache      = readGamertagCache
	signedInGamertag   = lookupSignedInGamertag
)

// ListInstances returns the installed instances sorted by name
// (case-insensitive).
func ListInstances() ([]Instance, error) { return listInstances() }

func scanInstances() ([]Instance, error) {
	root, err := apppath.InstancesDir()
	if err != nil {
		return nil, Errorf(ExitFailure, "ERR_ACCESS_VERSIONS_DIR", "cannot access the instances folder: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, Errorf(ExitFailure, "ERR_ACCESS_VERSIONS_DIR", "cannot read the instances folder: %v", err)
	}
	var out []Instance
	for _, e := range entries {
		dir := filepath.Join(root, e.Name())
		// A junction to an external game folder is not reported as a dir.
		if !e.IsDir() && !utils.ResolvesToDir(dir) {
			continue
		}
		if !versions.HasMeta(dir) {
			continue
		}
		m, err := versions.ReadMeta(dir)
		if err != nil {
			continue
		}
		out = append(out, Instance{
			Name: e.Name(), Dir: dir, Meta: m,
			PackageType: versions.DetectPackageType(dir, m),
			Channel:     strings.ToLower(strings.TrimSpace(m.Type)),
		})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

// LookupInstance finds an instance by exact name, ignoring case. A miss is
// ERR_INSTANCE_NOT_FOUND (exit 3) with the nearest names as a hint.
func LookupInstance(name string) (Instance, error) {
	all, err := listInstances()
	if err != nil {
		return Instance{}, err
	}
	return lookupIn(all, name)
}

func lookupIn(all []Instance, name string) (Instance, error) {
	name = strings.TrimSpace(name)
	for _, inst := range all {
		if strings.EqualFold(inst.Name, name) {
			return inst, nil
		}
	}
	e := Errorf(ExitNotFound, "ERR_INSTANCE_NOT_FOUND", "no instance named %q", name)
	names := instanceNames(all)
	if s := suggest(name, names); s != "" {
		return Instance{}, e.WithHint("did you mean %q?", s)
	}
	if len(names) == 0 {
		return Instance{}, e.WithHint("%s install latest", ProgramName)
	}
	return Instance{}, e.WithHint("%s instance list", ProgramName)
}

func instanceNames(all []Instance) []string {
	out := make([]string, len(all))
	for i, inst := range all {
		out[i] = inst.Name
	}
	return out
}

// Instance sources, as echoed and reported by DefaultInstance.
const (
	SourceArgument = "argument"
	SourceFlag     = "--instance"
	SourceEnv      = EnvInstance
	SourceSetting  = "default instance"
	SourceOnly     = "only instance"
)

// DefaultInstance returns the instance commands use when none is named,
// with its source (SourceEnv, SourceSetting, SourceOnly), or "" and "" when
// there is none. It does not check that the name exists.
func (c *Ctx) DefaultInstance() (name, source string, err error) {
	if v := strings.TrimSpace(c.Getenv(EnvInstance)); v != "" {
		return v, SourceEnv, nil
	}
	if v := strings.TrimSpace(defaultInstanceCfg()); v != "" {
		return v, SourceSetting, nil
	}
	all, err := listInstances()
	if err != nil {
		return "", "", err
	}
	if len(all) == 1 {
		return all[0].Name, SourceOnly, nil
	}
	return "", "", nil
}

// ResolveInstance picks the instance a command acts on: arg (a positional
// argument, "" when the command has none or it was omitted), else -i, else
// $LEVILAUNCHER_INSTANCE, else the default instance setting, else the only
// installed instance. An implicit choice is echoed on stderr. Names match
// exactly, ignoring case.
//
// Errors: a name that does not exist is exit 3; no instance installed is
// exit 3 with an install hint; several instances and no default is exit 2
// listing them with the hint "LeviLauncher.exe use <name>"; a positional
// argument contradicting -i is exit 2.
func (c *Ctx) ResolveInstance(arg string) (Instance, error) {
	arg = strings.TrimSpace(arg)
	flag := strings.TrimSpace(c.G.Instance)
	if arg != "" && flag != "" && !strings.EqualFold(arg, flag) {
		return Instance{}, UsageError("instance %q given twice: as an argument and as --instance %q", arg, flag)
	}
	all, err := listInstances()
	if err != nil {
		return Instance{}, err
	}
	switch {
	case arg != "":
		return lookupIn(all, arg)
	case flag != "":
		return lookupIn(all, flag)
	}
	if len(all) == 0 {
		return Instance{}, Errorf(ExitNotFound, "ERR_NO_INSTANCES", "no instance is installed").
			WithHint("%s install latest", ProgramName)
	}
	name, source, err := c.DefaultInstance()
	if err != nil {
		return Instance{}, err
	}
	if name == "" {
		return Instance{}, UsageError("several instances are installed and none is the default: %s",
			strings.Join(instanceNames(all), ", ")).
			WithHint("%s use <name>, or pass -i <name>", ProgramName).
			withCode("ERR_INSTANCE_REQUIRED")
	}
	inst, err := lookupIn(all, name)
	if err != nil {
		e := FromError(err)
		switch source {
		case SourceEnv:
			e.Message = fmt.Sprintf("%s names %q, which is not installed", EnvInstance, name)
		case SourceSetting:
			e.Message = fmt.Sprintf("the default instance %q is no longer installed", name)
			e.Hint = ProgramName + " use <name>"
		}
		return Instance{}, e
	}
	c.Notice("Using instance %s (%s)", inst.Name, source)
	return inst, nil
}

// Player is a player profile folder of a GDK instance.
type Player struct {
	ID       string // folder name under the instance's Users folder
	Gamertag string // cached gamertag, "" when unknown
	Dir      string // absolute folder
}

// excludedPlayerDirs are folders under Users that are not players.
var excludedPlayerDirs = map[string]bool{"shared": true, "9556213259376595538": true}

// ListPlayers returns the player profiles of a GDK instance, sorted by
// gamertag then ID. UWP instances have none.
func ListPlayers(inst Instance) ([]Player, error) {
	if inst.IsUWP() {
		return nil, nil
	}
	root := usersRootOf(inst)
	if root == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	tags := gamertagCache()
	var out []Player
	for _, e := range entries {
		if !e.IsDir() || excludedPlayerDirs[strings.ToLower(e.Name())] {
			continue
		}
		out = append(out, Player{ID: e.Name(), Gamertag: tags[e.Name()], Dir: filepath.Join(root, e.Name())})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := strings.ToLower(out[i].Gamertag), strings.ToLower(out[j].Gamertag)
		if a != b {
			return a < b
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// ResolvePlayer picks the player profile for per-player content of a GDK
// instance: -p (profile ID or cached gamertag, exact, ignoring case), else
// $LEVILAUNCHER_PLAYER, else the only profile, else the profile of the
// signed-in Microsoft account (from the cache, waiting at most 3 seconds).
// An implicit choice is echoed on stderr.
//
// UWP instances have no players: the result is the zero Player and nil,
// with a notice when -p was given. Errors: an unknown player is exit 3; no
// profile is exit 3; several profiles and no way to choose is exit 2 with
// the list.
func (c *Ctx) ResolvePlayer(inst Instance) (Player, error) {
	want := strings.TrimSpace(c.G.Player)
	source := "--player"
	if want == "" {
		if v := strings.TrimSpace(c.Getenv(EnvPlayer)); v != "" {
			want, source = v, EnvPlayer
		}
	}
	if inst.IsUWP() {
		if strings.TrimSpace(c.G.Player) != "" {
			c.Notice("--player is ignored: UWP instances have no player profiles")
		}
		return Player{}, nil
	}
	players, err := ListPlayers(inst)
	if err != nil {
		return Player{}, Errorf(ExitFailure, "ERR_ACCESS_PLAYERS", "cannot read the player profiles of %s: %v", inst.Name, err)
	}
	if want != "" {
		for _, p := range players {
			if strings.EqualFold(p.ID, want) || (p.Gamertag != "" && strings.EqualFold(p.Gamertag, want)) {
				if source != "--player" {
					c.Notice("Using player %s (%s)", p.label(), source)
				}
				return p, nil
			}
		}
		e := Errorf(ExitNotFound, "ERR_PLAYER_NOT_FOUND", "no player %q in instance %s", want, inst.Name)
		return Player{}, e.WithHint("%s player list -i %s", ProgramName, quoteArg(inst.Name))
	}
	switch len(players) {
	case 0:
		return Player{}, Errorf(ExitNotFound, "ERR_NO_PLAYERS", "instance %s has no player profiles yet", inst.Name).
			WithHint("start the game once and sign in: %s play %s", ProgramName, quoteArg(inst.Name))
	case 1:
		c.Notice("Using player %s (only profile)", players[0].label())
		return players[0], nil
	}
	if tag := signedInGamertag(c.Context()); tag != "" {
		for _, p := range players {
			if strings.EqualFold(p.Gamertag, tag) {
				c.Notice("Using player %s (signed-in account)", p.label())
				return p, nil
			}
		}
	}
	labels := make([]string, len(players))
	for i, p := range players {
		labels[i] = p.label()
	}
	return Player{}, UsageError("instance %s has several player profiles: %s", inst.Name, strings.Join(labels, ", ")).
		WithHint("pass -p <id|gamertag>").withCode("ERR_PLAYER_REQUIRED")
}

func (p Player) label() string {
	if p.Gamertag != "" {
		return p.Gamertag + " (" + p.ID + ")"
	}
	return p.ID
}

// quoteArg quotes a command-line argument for a hint when needed.
func quoteArg(s string) string {
	if s == "" || strings.ContainsAny(s, " \t\"&|<>^") {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}

// readGamertagCache reads the launcher's profile-folder -> gamertag cache.
func readGamertagCache() map[string]string {
	out := map[string]string{}
	b, err := os.ReadFile(apppath.GamertagsPath())
	if err != nil {
		return out
	}
	var m map[string]string
	if json.Unmarshal(b, &m) != nil {
		return out
	}
	for k, v := range m {
		if k, v = strings.TrimSpace(k), strings.TrimSpace(v); k != "" && v != "" {
			out[k] = v
		}
	}
	return out
}

// lookupSignedInGamertag returns the gamertag of the launcher's signed-in
// account, or "" when signed out or when it takes more than 3 seconds.
func lookupSignedInGamertag(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	result := make(chan string, 1)
	go func() {
		if err := RestoreAccount(ctx); err != nil {
			result <- ""
			return
		}
		tag, err := xbox.GetLocalUserGamertag()
		if err != nil {
			tag = ""
		}
		result <- strings.TrimSpace(tag)
	}()
	select {
	case tag := <-result:
		return tag
	case <-ctx.Done():
		return ""
	}
}

// RestoreAccount loads the launcher's saved Microsoft account selection
// (no network). Account and licence commands call it before using xbox or
// nativeinstall functions; it is cheap to call more than once.
func RestoreAccount(ctx context.Context) error {
	return nativeinstall.RestoreSharedAccount(ctx, apppath.AccountsDir())
}
