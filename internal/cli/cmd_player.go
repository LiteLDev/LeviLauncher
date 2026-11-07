package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/fsx"
	"github.com/liteldev/LeviLauncher/internal/xbox"
)

// Seams replaced by tests: online gamertag lookup and its cache file.
var (
	playerGamertagByXUID = func(ctx context.Context, xuid uint64) (string, error) {
		if err := RestoreAccount(ctx); err != nil {
			return "", err
		}
		return xbox.GetGamertagByXuid(xuid)
	}
	playerGamertagCachePath = apppath.GamertagsPath
)

func init() {
	Register(&Command{
		Path:    "player list",
		Summary: "list the player profiles of a GDK instance",
		Description: `These are the values -p accepts: the profile ID (folder name) or its
gamertag. Gamertags come from the launcher's cache; --resolve-names looks up
the missing ones online with the signed-in Microsoft account. The profile of
the signed-in account is marked with '*'. UWP instances have no profiles.`,
		Examples: []string{"player list", "player list -i Survival --resolve-names", "player ls -q"},
		Flags:    []*Flag{BoolFlag("resolve-names", "", "look up unknown gamertags online (needs a signed-in account)")},
		Run:      runPlayerList,
	})
}

type playerItem struct {
	ID       string `json:"id"`
	Gamertag string `json:"gamertag"`
	SignedIn bool   `json:"signedIn"`
	Worlds   int    `json:"worlds"`
	Path     string `json:"path"`
}

func runPlayerList(c *Ctx) error {
	inst, err := c.ResolveInstance("")
	if err != nil {
		return err
	}
	items := []playerItem{}
	if inst.IsUWP() {
		c.Notice("UWP instances do not separate players.")
		return c.Render(items, nil, nil)
	}
	if c.Bool("resolve-names") {
		if err := playerResolveNames(c, usersRootOf(inst)); err != nil {
			return err
		}
	}
	players, err := ListPlayers(inst)
	if err != nil {
		return Errorf(ExitFailure, "ERR_ACCESS_PLAYERS", "cannot read the player profiles of %s: %v", inst.Name, err)
	}
	signedIn := ""
	if len(players) > 0 {
		signedIn = signedInGamertag(c.Context())
	}
	for _, p := range players {
		it := playerItem{ID: p.ID, Gamertag: p.Gamertag, Path: p.Dir,
			SignedIn: signedIn != "" && strings.EqualFold(p.Gamertag, signedIn)}
		it.Worlds = len(contentDirNames(filepath.Join(p.Dir, "games", "com.mojang", "minecraftWorlds")))
		items = append(items, it)
	}
	if len(items) == 0 {
		c.Notice("Instance %s has no player profiles yet; they appear after the game is started and signed in.", inst.Name)
	}
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	return c.Render(items, ids, func(w io.Writer) error {
		if len(items) == 0 {
			return nil
		}
		rows := make([][]string, len(items))
		for i, it := range items {
			mark := " "
			if it.SignedIn {
				mark = "*"
			}
			rows[i] = []string{mark + " " + it.ID, Dash(it.Gamertag), strconv.Itoa(it.Worlds)}
		}
		return Table(w, []string{"  ID", "GAMERTAG", "WORLDS"}, rows)
	})
}

// playerResolveNames looks up the gamertags of profiles missing from the
// cache and saves them, as the window does: a profile's XUID is the name of
// the first folder in its Screenshots folder. A failed lookup is a warning;
// not being signed in stops the lookups.
func playerResolveNames(c *Ctx, usersRoot string) error {
	usersRoot = strings.TrimSpace(usersRoot)
	if usersRoot == "" {
		return nil
	}
	entries, err := os.ReadDir(usersRoot)
	if err != nil {
		return nil
	}
	cache := playerReadCache()
	changed := false
	for _, e := range entries {
		id := strings.TrimSpace(e.Name())
		if !e.IsDir() || excludedPlayerDirs[strings.ToLower(id)] || strings.TrimSpace(cache[id]) != "" {
			continue
		}
		xuid := playerXUID(filepath.Join(usersRoot, id))
		if xuid == 0 {
			c.Verbosef("no XUID known for profile %s", id)
			continue
		}
		if err := c.Context().Err(); err != nil {
			return err
		}
		raw, err := playerGamertagByXUID(c.Context(), xuid)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			if errors.Is(err, xbox.ErrInteractionRequired) {
				c.Warn("not signed in to a Microsoft account; showing cached gamertags only (sign in: %s account login)", ProgramName)
				break
			}
			c.Warn("cannot look up the gamertag of profile %s: %v", id, err)
			continue
		}
		parts := strings.SplitN(strings.TrimSpace(raw), "|", 2)
		tag := strings.TrimSpace(parts[0])
		if tag == "" {
			continue
		}
		key := id
		if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
			key = strings.TrimSpace(parts[1])
		}
		if cache[key] != tag {
			cache[key] = tag
			changed = true
		}
	}
	if changed {
		b, err := json.Marshal(cache)
		if err == nil {
			err = fsx.WriteFileAtomic(playerGamertagCachePath(), b, 0o644)
		}
		if err != nil {
			c.Warn("cannot save the gamertag cache: %v", err)
		}
	}
	return nil
}

// playerXUID reads a profile's XUID from its Screenshots folder, 0 if none.
func playerXUID(profileDir string) uint64 {
	entries, err := os.ReadDir(filepath.Join(profileDir, "games", "com.mojang", "Screenshots"))
	if err != nil {
		return 0
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if n, err := strconv.ParseUint(strings.TrimSpace(e.Name()), 10, 64); err == nil {
			return n
		}
		return 0
	}
	return 0
}

// playerReadCache reads the whole gamertag cache file, keeping its entries
// so that saving it does not drop any.
func playerReadCache() map[string]string {
	out := map[string]string{}
	b, err := os.ReadFile(playerGamertagCachePath())
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
