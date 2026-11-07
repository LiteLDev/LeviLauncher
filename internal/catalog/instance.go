package catalog

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FreeName returns name, or name followed by "-2", "-3", ... when taken
// reports the name in use. It gives up after 999 tries and returns "".
func FreeName(name string, taken func(string) bool) string {
	name = strings.TrimSpace(name)
	if !taken(name) {
		return name
	}
	for i := 2; i < 1000; i++ {
		n := fmt.Sprintf("%s-%d", name, i)
		if len(n) > 64 {
			return ""
		}
		if !taken(n) {
			return n
		}
	}
	return ""
}

// InheritSource is an instance whose own game data a new instance can copy.
type InheritSource struct {
	Name        string
	GameVersion string
	Channel     string
	PackageType string
	Isolated    bool
}

// InheritCandidates returns the instances a new GDK instance of channel can
// copy game data from, as the install page offers them: GDK instances with
// their own data on the same channel that also appear in eligible (the
// backend's list of instances whose data folder is not empty).
func InheritCandidates(all []InheritSource, eligible []string, channel string) []InheritSource {
	ok := map[string]bool{}
	for _, n := range eligible {
		ok[n] = true
	}
	channel = NormalizeChannel(channel)
	out := []InheritSource{}
	for _, s := range all {
		if NormalizeType(s.PackageType) != TypeGDK || !s.Isolated ||
			!strings.EqualFold(strings.TrimSpace(s.Channel), channel) || !ok[s.Name] {
			continue
		}
		out = append(out, s)
	}
	return out
}

// GDKIdentity is the identity of an extracted GDK game folder, from its
// MicrosoftGame.config.
type GDKIdentity struct {
	Name           string // Microsoft.MinecraftUWP or Microsoft.MinecraftWindowsBeta
	PackageVersion string // 1.21.12001.0
	GameVersion    string // 1.21.120.01
	Channel        string // release or preview
}

type microsoftGameConfig struct {
	Identity struct {
		Name    string `xml:"Name,attr"`
		Version string `xml:"Version,attr"`
	} `xml:"Identity"`
}

// ReadGDKIdentity reads dir\MicrosoftGame.config. ok is false when it is
// missing, unreadable or not a Minecraft package.
func ReadGDKIdentity(dir string) (GDKIdentity, bool) {
	b, err := os.ReadFile(filepath.Join(dir, "MicrosoftGame.config"))
	if err != nil {
		return GDKIdentity{}, false
	}
	return ParseGDKIdentity(b)
}

// ParseGDKIdentity parses the contents of a MicrosoftGame.config.
func ParseGDKIdentity(b []byte) (GDKIdentity, bool) {
	var cfg microsoftGameConfig
	if err := xml.Unmarshal(b, &cfg); err != nil {
		return GDKIdentity{}, false
	}
	id := GDKIdentity{Name: strings.TrimSpace(cfg.Identity.Name), PackageVersion: strings.TrimSpace(cfg.Identity.Version)}
	switch {
	case strings.EqualFold(id.Name, "Microsoft.MinecraftUWP"):
		id.Channel = ChannelRelease
	case strings.EqualFold(id.Name, "Microsoft.MinecraftWindowsBeta"):
		id.Channel = ChannelPreview
	default:
		return GDKIdentity{}, false
	}
	gv, ok := PackageToGameVersion(id.PackageVersion)
	if !ok {
		return GDKIdentity{}, false
	}
	id.GameVersion = gv
	return id, true
}
