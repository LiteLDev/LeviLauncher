// Package ref ports lip's sources (protocol/package.md §2): Ref and GitRef of
// src/Lip.Core/Mcmod/Ref.cs, package arguments (PackageArguments.cs,
// protocol/cli.md §3), Go module path checking (src/Golang.Org.X.Mod/Module.cs)
// and package name comparison (PackageNames in Packages.cs), all as of lip
// v0.38.0-beta.1.
package ref

import (
	"fmt"
	"regexp"
	"strings"
)

// Kind is the form of a source (RefKind).
type Kind int

const (
	// KindURL is "https://..." naming a ZIP archive.
	KindURL Kind = iota
	// KindSpec is a Go module path naming a git repository (a remote
	// repository); its version tags are the versions, unless a ref pins it.
	KindSpec
	// KindFile is "file:///..." naming a local ZIP file or local directory.
	// Requests have local paths as the file: URI of their absolute path.
	KindFile
)

func (k Kind) String() string {
	switch k {
	case KindURL:
		return "Url"
	case KindSpec:
		return "Spec"
	default:
		return "File"
	}
}

// Ref is a source: target [ "#" variant ], where a remote repository may be
// followed by [ "@" ref ] after the variant.
type Ref struct {
	Kind Kind
	// Target is the location without the variant and the git ref: the URL
	// exactly as written, the module path of a spec ref, or the absolute file
	// URI of a file ref.
	Target string
	// Variant is the part after the "#", or "" when there is none.
	Variant string
	// GitRef pins a spec ref, or is nil when the spec ref selects by tag
	// version. Being a pointer, it makes == compare identity: compare Refs
	// with Equal, and key maps by String().
	GitRef *GitRef
}

// VariantRule says what a variant looks like, for messages.
const VariantRule = "a variant is a name, or two names separated by '/', of lowercase letters," +
	" digits and '_' (e.g. client or my_variant/sub)"

var variantRegex = regexp.MustCompile(`^[a-z0-9_]+(/[a-z0-9_]+)?$`)

// IsValidVariant reports whether variant is a valid non-empty variant.
func IsValidVariant(variant string) bool { return variantRegex.MatchString(variant) }

func (r Ref) String() string {
	s := r.Target
	if r.Variant != "" {
		s += "#" + r.Variant
	}
	if r.GitRef != nil {
		s += "@" + r.GitRef.String()
	}
	return s
}

// Equal reports whether r and o are the same source (record equality).
func (r Ref) Equal(o Ref) bool {
	if r.Kind != o.Kind || r.Target != o.Target || r.Variant != o.Variant {
		return false
	}
	if r.GitRef == nil || o.GitRef == nil {
		return r.GitRef == nil && o.GitRef == nil
	}
	return *r.GitRef == *o.GitRef
}

// LocalPath is, for a file ref, the absolute local path (new
// Uri(Target).LocalPath). It fails for other kinds, and, like .NET, for the
// rare target that does not parse again as a file URI.
func (r Ref) LocalPath() (string, error) {
	if r.Kind != KindFile {
		return "", fmt.Errorf("The source '%s' is not a local path, so it has no local file path.", r)
	}
	return fileURILocalPath(r.Target)
}

// hasFileScheme is s.StartsWith("file:", StringComparison.OrdinalIgnoreCase),
// which folds no non-ASCII char into "file:" (U+0131 'ı' does not match).
func hasFileScheme(s string) bool {
	return len(s) >= len("file:") && asciiEqualFold(s[:len("file:")], "file:")
}

func asciiEqualFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if x >= 'A' && x <= 'Z' {
			x |= 0x20
		}
		if y >= 'A' && y <= 'Z' {
			y |= 0x20
		}
		if x != y {
			return false
		}
	}
	return true
}

// WithVariant returns r with another variant ("" for none).
func (r Ref) WithVariant(variant string) (Ref, error) {
	if variant != "" && !IsValidVariant(variant) {
		return Ref{}, fmt.Errorf("The variant '%s' of '%s' is not valid: %s.", variant, r.Target,
			VariantRule)
	}
	r.Variant = variant
	return r, nil
}

// uriSchemeRegex matches a URI scheme that is no DOS drive letter.
var uriSchemeRegex = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]+:`)

// FromFile returns the file ref of an absolute local path (new
// Uri(absolutePath, UriKind.Absolute), which also takes a file: URI).
func FromFile(absolutePath, variant string) (Ref, error) {
	var uri string
	switch s := trimLeadingLWS(absolutePath); {
	case hasFileScheme(s):
		u, err := parseFileURI(s)
		if err != nil {
			return Ref{}, err
		}
		uri = u.absoluteURI()
	case uriSchemeRegex.MatchString(s):
		return Ref{}, fmt.Errorf("'%s' is not an absolute local path.", absolutePath)
	default:
		var err error
		if uri, err = implicitFileURI(absolutePath); err != nil {
			return Ref{}, err
		}
	}
	return Ref{Kind: KindFile, Target: uri}.WithVariant(variant)
}

// Error is the FormatException of Ref.Parse.
type Error struct {
	Source string
	// Reason is the error of Ref.TryParse.
	Reason string
}

func (e *Error) Error() string {
	return fmt.Sprintf("The source '%s' is not valid: %s.", e.Source, e.Reason)
}

// Parse parses a source; allowFile also accepts local paths as file: URIs.
// The error is an *Error.
func Parse(s string, allowFile bool) (Ref, error) {
	r, reason, ok := TryParse(s, allowFile)
	if !ok {
		return Ref{}, &Error{Source: s, Reason: reason}
	}
	return r, nil
}

// TryParse ports Ref.TryParse: the source, or why it is not valid.
func TryParse(s string, allowFile bool) (Ref, string, bool) {
	isURL := strings.HasPrefix(s, "https://")
	isFile := hasFileScheme(s)

	// Module paths, variants and URLs before the fragment contain no '@' that
	// starts a git ref; everything after the first '@' of a spec ref is its
	// git ref, which may contain '#'.
	head := s
	var gitRef *GitRef
	if !isURL && !isFile {
		if at := strings.IndexByte(s, '@'); at >= 0 {
			head = s[:at]
			g, gitErr, ok := TryParseGitRef(s[at+1:])
			if !ok {
				return Ref{}, fmt.Sprintf("the ref '%s' after '@' is not valid: %s", s[at+1:], gitErr), false
			}
			gitRef = &g
		}
	}

	target, variant := head, ""
	hash := strings.IndexByte(head, '#')
	if hash >= 0 {
		target, variant = head[:hash], head[hash+1:]
		if !IsValidVariant(variant) {
			if variant == "" {
				return Ref{}, "the variant after '#' is missing; " + VariantRule, false
			}
			return Ref{}, fmt.Sprintf("the variant '%s' is not valid; %s", variant, VariantRule), false
		}
	}

	switch {
	case isURL:
		if !isURIText(target) {
			return Ref{}, fmt.Sprintf("'%s' is not a valid absolute https:// URL (characters other"+
				" than those a URL allows must be percent-encoded)", target), false
		}
		if !validHTTPSURL(target) {
			return Ref{}, fmt.Sprintf("'%s' is not a valid absolute https:// URL", target), false
		}
		return Ref{Kind: KindURL, Target: target, Variant: variant}, "", true
	case isFile:
		if !allowFile {
			return Ref{}, "local paths are only allowed in package arguments on the command line," +
				" not as dependencies", false
		}
		abs, ok := fileURIAbsolute(target)
		if !ok {
			return Ref{}, fmt.Sprintf("'%s' is not an absolute file: URI", target), false
		}
		return Ref{Kind: KindFile, Target: abs, Variant: variant}, "", true
	default:
		if err := CheckModulePath(target); err != nil {
			// The reason alone; the outer message names the path again.
			reason := err.(*ModulePathError).Reason
			return Ref{}, fmt.Sprintf("'%s' is not a valid repository path: %s; a remote"+
				" repository is written like github.com/owner/repo", target, reason), false
		}
		return Ref{Kind: KindSpec, Target: target, Variant: variant, GitRef: gitRef}, "", true
	}
}
