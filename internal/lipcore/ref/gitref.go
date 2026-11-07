package ref

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/lipcore/semver"
)

// GitRefKind is the kind of a GitRef (Ref.cs GitRefKind).
type GitRefKind int

const (
	// GitRefVersion is a SemVer 2.0.0 version V, selecting the tag "v" + V.
	GitRefVersion GitRefKind = iota
	GitRefBranch
	GitRefTag
	GitRefRev
)

func (k GitRefKind) String() string {
	switch k {
	case GitRefVersion:
		return "version"
	case GitRefBranch:
		return "branch"
	case GitRefTag:
		return "tag"
	default:
		return "rev"
	}
}

func (k GitRefKind) prefix() string {
	switch k {
	case GitRefVersion:
		return ""
	case GitRefBranch:
		return "branch:"
	case GitRefTag:
		return "tag:"
	default:
		return "rev:"
	}
}

// GitRef is the ref of a remote repository (protocol/package.md §2.1): a
// version, "branch:" name, "tag:" name or "rev:" commit id. It ports GitRef
// of src/Lip.Core/Mcmod/Ref.cs (v0.38.0-beta.1).
type GitRef struct {
	Kind GitRefKind
	// Name is the version, the branch or tag name, or the full lowercase
	// commit id of a rev.
	Name string
}

func (g GitRef) String() string { return g.Kind.prefix() + g.Name }

// GitRefError is the FormatException of GitRef.Parse.
type GitRefError struct {
	Input  string
	Reason string
}

func (e *GitRefError) Error() string {
	return fmt.Sprintf("The ref '%s' of a remote repository is not valid: %s.", e.Input, e.Reason)
}

var commitIDRegex = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

// TryParseGitRef ports GitRef.TryParse: the ref, or the reason it is not
// valid.
func TryParseGitRef(s string) (GitRef, string, bool) {
	for _, kind := range []GitRefKind{GitRefBranch, GitRefTag, GitRefRev} {
		prefix := kind.prefix()
		if !strings.HasPrefix(s, prefix) {
			continue
		}

		name := s[len(prefix):]
		if kind == GitRefRev {
			if !commitIDRegex.MatchString(name) {
				return GitRef{}, fmt.Sprintf("'%s' is not a full commit id: write all 40"+
					" hexadecimal digits, not an abbreviation", name), false
			}
			name = strings.ToLower(name)
		} else if !IsValidRefName(name) {
			what := kind.String()
			if name == "" {
				return GitRef{}, fmt.Sprintf("the %s name after \"%s\" is missing", what, prefix), false
			}
			return GitRef{}, fmt.Sprintf("'%s' is not a valid git %s name (see 'git"+
				" check-ref-format')", name, what), false
		}
		return GitRef{Kind: kind, Name: name}, "", true
	}

	if isStrictSemver(s) {
		return GitRef{Kind: GitRefVersion, Name: s}, "", true
	}

	if len(s) > 1 && (s[0] == 'v' || s[0] == 'V') && isStrictSemver(s[1:]) {
		return GitRef{}, fmt.Sprintf("write the version without the leading 'v', as '%s'", s[1:]), false
	}
	return GitRef{}, "a ref is a SemVer version such as 1.2.3 (without a leading 'v')," +
		" branch:<name>, tag:<name> or rev:<full commit id>", false
}

// ParseGitRef ports GitRef.Parse; the error is a *GitRefError.
func ParseGitRef(s string) (GitRef, error) {
	g, reason, ok := TryParseGitRef(s)
	if !ok {
		return GitRef{}, &GitRefError{Input: s, Reason: reason}
	}
	return g, nil
}

// RevGitRef ports GitRef.Rev.
func RevGitRef(commit string) (GitRef, error) { return ParseGitRef("rev:" + commit) }

// IsValidRefName reports whether a branch or tag name is valid as git
// check-ref-format defines it for one ref name below refs/heads/ or
// refs/tags/ (GitRef.IsValidRefName).
func IsValidRefName(name string) bool {
	if name == "" || name == "@" || strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") ||
		strings.HasSuffix(name, ".") || strings.Contains(name, "//") ||
		strings.Contains(name, "..") || strings.Contains(name, "@{") {
		return false
	}
	for i := 0; i < len(name); i++ {
		switch c := name[i]; {
		case c < 0x20, c == 0x7f:
			return false
		case c == ' ', c == '~', c == '^', c == ':', c == '?', c == '*', c == '[', c == '\\':
			return false
		}
	}
	for _, comp := range strings.Split(name, "/") {
		if strings.HasPrefix(comp, ".") || strings.HasSuffix(comp, ".lock") {
			return false
		}
	}
	return true
}

// isStrictSemver reports whether SemVersion.TryParse(s, Strict) succeeds.
func isStrictSemver(s string) bool {
	_, err := semver.ParseStrict(s)
	return err == nil
}
