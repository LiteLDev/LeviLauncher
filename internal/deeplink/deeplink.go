// Package deeplink parses levilauncher:// links, the links levimc.org and
// lip's lip:// links share a grammar with:
//
//	levilauncher://install/<module-path>[?version=<semver>][&variant=<variant>]
//
// for example levilauncher://install/github.com/LiteLDev/Arke?version=0.1.6.
//
// A link only ever names a catalogue mod, a version and a variant. It never
// installs anything by itself and never names a path: the launcher opens the
// mod's page and the user picks the instance and confirms.
package deeplink

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/mod/module"

	"github.com/liteldev/LeviLauncher/internal/lipcore/semver"
)

// Scheme is the URI scheme the launcher registers.
const Scheme = "levilauncher"

// ActionInstall is the only action: open a mod's install view.
const ActionInstall = "install"

// EventOpen is emitted to the frontend when a link is waiting to be taken
// with DeepLinkService.TakePending.
const EventOpen = "deeplink.open"

// MaxLength bounds the raw link. Real links are well under 300 bytes; the
// limit keeps a crafted argument or pipe line from being worked on at all.
const MaxLength = 2048

var variantRe = regexp.MustCompile(`^[a-z0-9_]+(/[a-z0-9_]+)?$`)

// Request is a parsed, validated link.
type Request struct {
	// Action is ActionInstall.
	Action string `json:"action"`
	// ModulePath is the mod's catalogue id, a Go module path with its case
	// kept, such as github.com/LiteLDev/Arke.
	ModulePath string `json:"modulePath"`
	// Version is a strict SemVer version without a leading "v", or "" for
	// the latest.
	Version string `json:"version"`
	// Variant is the package variant, or "" for the one the launcher prefers.
	Variant string `json:"variant"`
}

// String is the canonical form of the link, which Parse reads back as r.
func (r Request) String() string {
	var b strings.Builder
	b.WriteString(Scheme)
	b.WriteString("://")
	b.WriteString(r.Action)
	b.WriteByte('/')
	b.WriteString(r.ModulePath)
	q := url.Values{}
	if r.Version != "" {
		q.Set("version", r.Version)
	}
	if r.Variant != "" {
		q.Set("variant", r.Variant)
	}
	if len(q) > 0 {
		b.WriteByte('?')
		b.WriteString(q.Encode())
	}
	return b.String()
}

// ErrNotDeepLink is returned by Parse for text that is not a levilauncher: link.
var ErrNotDeepLink = errors.New("not a levilauncher:// link")

// IsDeepLink reports whether arg is meant as a levilauncher: link, valid or
// not: it starts with the scheme, case-insensitively.
func IsDeepLink(arg string) bool {
	a := strings.TrimSpace(arg)
	return len(a) > len(Scheme) && strings.EqualFold(a[:len(Scheme)+1], Scheme+":")
}

// FromArgs returns the first argument that is a levilauncher: link, as
// Windows passes it ("%1"), and whether there was one.
func FromArgs(args []string) (string, bool) {
	for _, a := range args {
		if IsDeepLink(a) {
			return strings.TrimSpace(a), true
		}
	}
	return "", false
}

// Parse parses and validates a levilauncher:// link.
func Parse(raw string) (Request, error) {
	raw = strings.TrimSpace(raw)
	if !IsDeepLink(raw) {
		return Request{}, ErrNotDeepLink
	}
	if len(raw) > MaxLength {
		return Request{}, fmt.Errorf("link is longer than %d bytes", MaxLength)
	}
	for _, c := range raw {
		// Control characters and spaces never belong in a link; rejecting them
		// up front also keeps the canonical form a single pipe line.
		if c <= ' ' || c == 0x7f {
			return Request{}, errors.New("link contains a space or control character")
		}
		if c == '\\' {
			return Request{}, errors.New("link contains a backslash")
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return Request{}, fmt.Errorf("malformed link: %w", err)
	}
	if !strings.EqualFold(u.Scheme, Scheme) {
		return Request{}, ErrNotDeepLink
	}
	if u.Opaque != "" {
		return Request{}, errors.New("link must start with levilauncher://")
	}
	if u.User != nil || u.Port() != "" || strings.Contains(u.Host, ":") {
		return Request{}, errors.New("link has a user or port")
	}
	if u.Fragment != "" || strings.Contains(raw, "#") {
		return Request{}, errors.New("link has a fragment; name the variant with ?variant=")
	}
	action := strings.ToLower(u.Host)
	if action != ActionInstall {
		if action == "" {
			return Request{}, errors.New("link has no action")
		}
		return Request{}, fmt.Errorf("unknown action %q", u.Host)
	}

	path := strings.TrimPrefix(u.Path, "/")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return Request{}, errors.New("link names no mod")
	}
	if err := module.CheckPath(path); err != nil {
		return Request{}, fmt.Errorf("invalid mod id: %w", err)
	}

	q, err := parseQuery(u.RawQuery)
	if err != nil {
		return Request{}, fmt.Errorf("malformed query: %w", err)
	}
	version, err := single(q, "version")
	if err != nil {
		return Request{}, err
	}
	if version != "" {
		if _, err := semver.Parse(version); err != nil {
			return Request{}, fmt.Errorf("version %q is not a SemVer version such as 1.2.3", version)
		}
	}
	variant, err := single(q, "variant")
	if err != nil {
		return Request{}, err
	}
	if variant != "" && !variantRe.MatchString(variant) {
		return Request{}, fmt.Errorf("invalid variant %q", variant)
	}
	return Request{Action: ActionInstall, ModulePath: path, Version: version, Variant: variant}, nil
}

// single returns the one value of key, "" when it is absent. A key given
// twice is ambiguous and rejected.
func single(q url.Values, key string) (string, error) {
	values := q[key]
	switch len(values) {
	case 0:
		return "", nil
	case 1:
		return values[0], nil
	default:
		return "", fmt.Errorf("%s is given more than once", key)
	}
}

// parseQuery splits a query like url.ParseQuery, except that '+' stays '+'
// (url.ParseQuery reads it as a space): a SemVer version's build metadata
// uses '+', and lip reads lip:// links the same way.
func parseQuery(raw string) (url.Values, error) {
	q := url.Values{}
	for _, part := range strings.Split(raw, "&") {
		if part == "" {
			continue
		}
		k, v, _ := strings.Cut(part, "=")
		key, err := url.PathUnescape(k)
		if err != nil {
			return nil, err
		}
		value, err := url.PathUnescape(v)
		if err != nil {
			return nil, err
		}
		q[key] = append(q[key], value)
	}
	return q, nil
}
