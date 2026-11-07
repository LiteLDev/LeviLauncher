// Package catalog is the Minecraft build catalogue the launcher installs
// from: the GDK (msixvc) history merged with the Store's newest builds, and
// the UWP (appx) builds of Windows Update. It holds the rules the download
// and install pages used to keep in TypeScript (normalisation, sorting,
// version matching, mirror choice, installer file names, LeviLamina version
// order, instance naming) so the command line and the window can share them.
//
// The package does no I/O of its own except where a function says so
// (Load, ProbeMirrors, ScanInstallers, ReadGDKIdentity).
package catalog

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/gdkversions"
	"github.com/liteldev/LeviLauncher/internal/lipindex"
	"github.com/liteldev/LeviLauncher/internal/uwpdownload"
)

// Package types and channels, as stored in instance settings.
const (
	TypeGDK = "gdk"
	TypeUWP = "uwp"

	ChannelRelease = "release"
	ChannelBeta    = "beta"
	ChannelPreview = "preview"
)

// Channels lists the channels in display order.
var Channels = []string{ChannelRelease, ChannelBeta, ChannelPreview}

// Build is one installable Minecraft build.
type Build struct {
	// Version is the game version: "1.21.50.07" (GDK) or "1.21.114.1" (UWP).
	Version string
	// Channel is release, beta (UWP only) or preview.
	Channel string
	// Type is gdk or uwp.
	Type string
	// URLs are the GDK download mirrors, xboxlive.cn hosts first.
	URLs []string
	// MD5 is the GDK installer digest; empty for builds found in the Store
	// that the history does not list yet.
	MD5 string
	// UUID is the Windows Update ID of a UWP build.
	UUID string
	// Timestamp is when the GDK build was catalogued (Unix seconds); 0 for
	// UWP builds.
	Timestamp int64
}

// Label is the build's channel as a title: "Release", "Beta", "Preview".
func (b Build) Label() string { return ChannelLabel(b.Channel) }

// Key identifies a build across catalogues: "gdk:release:1.21.50.07".
func (b Build) Key() string { return b.Type + ":" + b.Channel + ":" + b.Version }

// ChannelLabel returns "Release", "Beta" or "Preview" for a channel; an
// unknown channel is "Release", as the window treats it.
func ChannelLabel(channel string) string {
	switch NormalizeChannel(channel) {
	case ChannelPreview:
		return "Preview"
	case ChannelBeta:
		return "Beta"
	}
	return "Release"
}

// NormalizeChannel maps any spelling of a channel to release, beta or
// preview; anything else is release.
func NormalizeChannel(channel string) string {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case ChannelPreview:
		return ChannelPreview
	case ChannelBeta:
		return ChannelBeta
	}
	return ChannelRelease
}

// NormalizeType maps a package type to gdk or uwp (anything but uwp is gdk).
func NormalizeType(t string) string {
	if strings.EqualFold(strings.TrimSpace(t), TypeUWP) {
		return TypeUWP
	}
	return TypeGDK
}

var channelPrefix = regexp.MustCompile(`(?i)^(?:Preview|Release)\s*`)

// ShortVersion strips the channel word of a GDK history entry:
// "Release 1.21.50.07" is "1.21.50.07".
func ShortVersion(v string) string {
	return strings.TrimSpace(channelPrefix.ReplaceAllString(strings.TrimSpace(v), ""))
}

// NormalizeURLs trims the mirror URLs of a history entry, strips stray
// backticks, drops empty ones and moves xboxlive.cn hosts first (stable).
func NormalizeURLs(raw []string) []string {
	out := make([]string, 0, len(raw))
	for _, u := range raw {
		u = strings.TrimSpace(u)
		u = strings.TrimPrefix(u, "`")
		u = strings.TrimSuffix(u, "`")
		if u != "" {
			out = append(out, u)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.Contains(out[i], "xboxlive.cn") && !strings.Contains(out[j], "xboxlive.cn")
	})
	return out
}

// FromGDK turns a GDK catalogue into builds, previews first as the history
// lists them.
func FromGDK(c gdkversions.Catalog) []Build {
	out := make([]Build, 0, len(c.ReleaseVersions)+len(c.PreviewVersions))
	for _, list := range []struct {
		channel  string
		versions []gdkversions.Version
	}{{ChannelPreview, c.PreviewVersions}, {ChannelRelease, c.ReleaseVersions}} {
		for _, v := range list.versions {
			short := ShortVersion(v.Version)
			if short == "" {
				continue
			}
			out = append(out, Build{
				Version: short, Channel: list.channel, Type: TypeGDK,
				URLs: NormalizeURLs(v.URLs), MD5: strings.TrimSpace(v.MD5), Timestamp: v.Timestamp,
			})
		}
	}
	return out
}

// FromUWP turns the UWP catalogue into builds.
func FromUWP(list []uwpdownload.Version) []Build {
	out := make([]Build, 0, len(list))
	for _, v := range list {
		out = append(out, Build{
			Version: strings.TrimSpace(v.Version), Channel: NormalizeChannel(v.Type), Type: TypeUWP,
			UUID: strings.TrimSpace(v.UUID), URLs: []string{},
		})
	}
	return out
}

// CompareVersions orders dotted versions numerically segment by segment
// (missing or non-numeric segments count as 0): -1, 0 or 1.
func CompareVersions(a, b string) int {
	pa, pb := strings.Split(strings.TrimSpace(a), "."), strings.Split(strings.TrimSpace(b), ".")
	n := max(len(pa), len(pb))
	for i := 0; i < n; i++ {
		var va, vb int
		if i < len(pa) {
			va, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			vb, _ = strconv.Atoi(pb[i])
		}
		if va != vb {
			if va < vb {
				return -1
			}
			return 1
		}
	}
	return 0
}

// Sort orders builds as the download page does: newest catalogue timestamp
// first (UWP builds have none and come last), then newest version first.
func Sort(builds []Build) {
	sort.SliceStable(builds, func(i, j int) bool {
		if builds[i].Timestamp != builds[j].Timestamp {
			return builds[i].Timestamp > builds[j].Timestamp
		}
		return CompareVersions(builds[i].Version, builds[j].Version) > 0
	})
}

// Filter narrows a catalogue. Empty fields match everything.
type Filter struct {
	Channels []string // release, beta, preview
	Type     string   // gdk or uwp
	// Query matches part of the version, channel or type, ignoring case.
	Query string
}

// Match reports whether b passes f.
func (f Filter) Match(b Build) bool {
	if t := strings.TrimSpace(f.Type); t != "" && !strings.EqualFold(t, b.Type) {
		return false
	}
	if len(f.Channels) > 0 {
		ok := false
		for _, c := range f.Channels {
			if NormalizeChannel(c) == b.Channel {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if q := strings.ToLower(strings.TrimSpace(f.Query)); q != "" {
		return strings.Contains(strings.ToLower(b.Version), q) ||
			strings.Contains(strings.ToLower(b.Label()+" "+b.Version), q) ||
			strings.Contains(b.Channel, q) || strings.Contains(b.Type, q)
	}
	return true
}

// Select returns the builds passing f, in order.
func Select(builds []Build, f Filter) []Build {
	out := []Build{}
	for _, b := range builds {
		if f.Match(b) {
			out = append(out, b)
		}
	}
	return out
}

// Latest is the version spec naming the newest build.
const Latest = "latest"

// VersionEqual reports whether two versions are the same build number,
// segment by segment, so "1.21.50.7" equals "1.21.50.07".
func VersionEqual(a, b string) bool { return lipindex.VersionEqual(a, b) }

// HasVersionPrefix reports whether prefix names version on dotted segment
// boundaries: "1.21.50" matches "1.21.50.07", "1.21.5" does not, and a
// version is a prefix of itself.
func HasVersionPrefix(version, prefix string) bool { return lipindex.HasVersionPrefix(version, prefix) }

// Resolution errors. Resolve wraps them with the spec.
var (
	ErrNoBuild   = errors.New("no build matches")
	ErrBadSpec   = errors.New("not a version")
	specCharsRef = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)*$`)
)

// ValidSpec reports whether spec is "latest" or a dotted number.
func ValidSpec(spec string) bool {
	spec = strings.TrimSpace(spec)
	return strings.EqualFold(spec, Latest) || specCharsRef.MatchString(spec)
}

// Resolve picks the build a version spec names among builds:
//
//   - "latest": the newest build (release unless f.Channels says otherwise);
//   - an exact version ("1.21.50.07", also "1.21.50.7");
//   - a dotted prefix on segment boundaries ("1.21.50" is the newest
//     1.21.50.x; "1.21.5" does not match it). Without f.Channels a prefix
//     prefers release builds when any matches.
//
// When the same version exists as GDK and UWP, GDK wins unless f.Type says
// otherwise. f.Query is ignored.
func Resolve(builds []Build, spec string, f Filter) (Build, error) {
	spec = strings.TrimSpace(spec)
	if !ValidSpec(spec) {
		return Build{}, fmt.Errorf("%w: %q", ErrBadSpec, spec)
	}
	f.Query = ""
	pool := Select(builds, f)
	latest := strings.EqualFold(spec, Latest)
	var cands []Build
	if !latest {
		for _, b := range pool {
			if VersionEqual(b.Version, spec) {
				cands = append(cands, b)
			}
		}
	}
	if len(cands) == 0 {
		for _, b := range pool {
			if latest || HasVersionPrefix(b.Version, spec) {
				cands = append(cands, b)
			}
		}
		if len(f.Channels) == 0 {
			var rel []Build
			for _, b := range cands {
				if b.Channel == ChannelRelease {
					rel = append(rel, b)
				}
			}
			if len(rel) > 0 || latest {
				cands = rel
			}
		}
	}
	if len(cands) == 0 {
		return Build{}, fmt.Errorf("%w %q", ErrNoBuild, spec)
	}
	best := cands[0]
	for _, b := range cands[1:] {
		if better(b, best) {
			best = b
		}
	}
	return best, nil
}

var channelRank = map[string]int{ChannelRelease: 0, ChannelPreview: 1, ChannelBeta: 2}

func better(a, b Build) bool {
	if c := CompareVersions(a.Version, b.Version); c != 0 {
		return c > 0
	}
	if a.Type != b.Type {
		return a.Type == TypeGDK
	}
	return channelRank[a.Channel] < channelRank[b.Channel]
}

// Nearest returns up to n catalogue versions closest to spec, for a
// did-you-mean hint: those sharing the longest dotted prefix, newest first.
func Nearest(builds []Build, spec string, n int) []string {
	sp := strings.Split(strings.TrimSpace(spec), ".")
	out := []string{}
	seen := map[string]bool{}
	for k := len(sp) - 1; k >= 1 && len(out) == 0; k-- {
		prefix := strings.Join(sp[:k], ".")
		for _, b := range builds {
			if HasVersionPrefix(b.Version, prefix) && !seen[b.Version] {
				seen[b.Version] = true
				out = append(out, b.Version)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return CompareVersions(out[i], out[j]) > 0 })
	if len(out) > n {
		out = out[:n]
	}
	return out
}
