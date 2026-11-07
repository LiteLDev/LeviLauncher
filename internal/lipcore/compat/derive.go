package compat

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/lipcore/doc"
	"github.com/liteldev/LeviLauncher/internal/lipcore/glob"
	"github.com/liteldev/LeviLauncher/internal/lipcore/manifest"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pathglob"
	"github.com/liteldev/LeviLauncher/internal/lipcore/pkgmodel"
	"github.com/liteldev/LeviLauncher/internal/lipcore/semver"
)

// Derive reads tooth.json bytes (ParseToothJSON) and derives the package file
// for the environment (Tooth.Derive): PackageDocument.ParseToothJson(...)
// .ReadFor(environment, version).
func Derive(tooth []byte, platform, runtime string, version *semver.Version) (*pkgmodel.Document, error) {
	t, err := ParseToothJSON(tooth)
	if err != nil {
		return nil, err
	}
	return t.Derive(platform, runtime, version)
}

// Derive is ToothCompat.Derive: the package file a tooth.json of format 3 is
// read as for the environment (platform "server" or "client", runtime a .NET
// runtime identifier), with version as the version of the package (its tag
// version), or, when nil, the version of the tooth.json. Its Raw is the
// derived document (a *doc.Object), and its entries are Derived, with Raw the
// entry within it.
//
// It fails with a *FormatError when the variants of a label cannot be merged
// (lip v0.37.0 failed on them too), or when a derived entry is not valid in a
// package file.
func (t *Tooth) Derive(platform, runtime string, version *semver.Version) (*pkgmodel.Document, error) {
	if platform != pkgmodel.PlatformServer && platform != pkgmodel.PlatformClient {
		return nil, fmt.Errorf("compat: platform %q is neither %q nor %q",
			platform, pkgmodel.PlatformServer, pkgmodel.PlatformClient)
	}
	if version == nil {
		if t.nullVersion {
			return nil, fmt.Errorf("the %s of %s has \"version\": null, which lip cannot read"+
				" without a tag version", ToothFileName, t.ID)
		}
		version = &t.Version
	}
	if t.nullVariants {
		return nil, fmt.Errorf("the %s of %s %s has \"variants\": null, which lip cannot read",
			ToothFileName, t.ID, version)
	}

	// derived maps each label, in the order first seen, to its entry or nil.
	derived := doc.NewObject()
	seen := map[string]bool{}
	for _, v := range t.m.Variants {
		label := v.Label
		if seen[label] {
			continue
		}
		seen[label] = true
		merged, err := t.getVariant(label, runtime)
		if err != nil {
			var mf *mergeFailure
			if errors.As(err, &mf) {
				return nil, formatErrorf(nil, "The %s of %s %s is not valid: its variants labeled"+
					" '%s' cannot be merged for %s, because %s. lip v0.37.0 could not install them"+
					" either; ask the package author to fix %s.",
					ToothFileName, t.ID, version, label, runtime, mf.reason, ToothFileName)
			}
			return nil, fmt.Errorf("the %s of %s %s cannot be read for %s: %w",
				ToothFileName, t.ID, version, runtime, err)
		}
		if merged == nil {
			continue // no variants for the runtime
		}
		entry, err := convertEntry(merged, platform, runtime)
		if err != nil {
			return nil, fmt.Errorf("the %s of %s %s cannot be read for %s: %w",
				ToothFileName, t.ID, version, runtime, err)
		}
		derived.Set(label, nilIfNone(entry))
	}

	if platform == pkgmodel.PlatformClient && seen[ClientLabel] {
		var entry any
		if client, ok := derived.Get(ClientLabel); ok && client != nil {
			entry = client.(*doc.Object).Clone()
		}
		derived.Set("", entry)
	}

	variants := doc.NewObject()
	for _, label := range derived.Keys() {
		if e, _ := derived.Get(label); e != nil {
			variants.Set(label, []any{e})
		}
	}

	document := doc.NewObject()
	document.Set(pkgmodel.FormatKey, json.Number(fmt.Sprint(pkgmodel.Format)))
	document.Set("id", t.ID)
	document.Set("version", version.String())
	var info *pkgmodel.Info
	if t.info != nil {
		o, i := convertInfo(t.info)
		document.Set("info", o)
		info = i
	}
	document.Set("variants", variants)

	result := &pkgmodel.Document{
		Format:  pkgmodel.Format,
		ID:      t.ID,
		Version: version.String(),
		Info:    info,
		Raw:     document,
	}
	for _, label := range variants.Keys() {
		v, _ := variants.Get(label)
		raw := v.([]any)[0].(*doc.Object)
		entry, err := parseEntry(raw)
		if err != nil {
			return nil, formatErrorf(err, "The %s of %s %s cannot be read for %s: its variant"+
				" '%s' does not convert to a valid variant of %s. Ask the package author to move"+
				" %s to %s with 'lip migrate'.", ToothFileName, t.ID, version, runtime, label,
				pkgmodel.PackageFileName, ToothFileName, pkgmodel.PackageFileName)
		}
		entry.Derived = true
		result.Variants = append(result.Variants,
			pkgmodel.VariantEntries{Name: label, Entries: []pkgmodel.Entry{*entry}})
	}
	return result, nil
}

func nilIfNone(o *doc.Object) any {
	if o == nil {
		return nil
	}
	return o
}

// mergeFailure is the ArgumentException or InvalidOperationException of
// PackageManifest.GetVariant, which Derive turns into a PackageFormatException.
type mergeFailure struct{ reason string }

func (e *mergeFailure) Error() string { return e.reason }

// mergeFailureReason is ToothCompat.MergeFailureReason.
const (
	reasonDependency = "more than one of them declares the same dependency"
	reasonScript     = "more than one of them sets the same kind of script" +
		" (pre_install, install, post_install, pre_uninstall, uninstall or post_uninstall)"
)

// mergedVariant is the PackageManifestVariant GetVariant returns, with the
// format 3 objects of its assets.
type mergedVariant struct {
	deps      manifest.Dependencies
	assets    []manifest.Asset
	rawAssets []*doc.Object
	preserve  []string
	remove    []string
	scripts   manifest.Scripts
}

// nullMember reports whether the format 3 object has member key set to null.
func nullMember(o *doc.Object, key string) bool {
	v, ok := o.Get(key)
	return ok && v == nil
}

// getVariant is PackageManifest.GetVariant(label, rid) of lip v0.38: the
// variants labeled label whose platform is "", rid, or a DotNet.Glob pattern
// matching rid, merged; nil when there are none (KeyNotFoundException). A
// *mergeFailure is an exception Derive reports as a PackageFormatException;
// any other error one lip does not expect (a JSON null it dereferences).
func (t *Tooth) getVariant(label, rid string) (*mergedVariant, error) {
	var sel []int
	for i := range t.m.Variants {
		v := &t.m.Variants[i]
		if v.Label != label {
			continue
		}
		if nullMember(t.raw[i], "platform") {
			// Glob.Parse(null) throws ArgumentNullException.
			return nil, &mergeFailure{reason: fmt.Sprintf("variants[%d] has \"platform\": null", i)}
		}
		if v.Platform != "" && v.Platform != rid {
			g, err := glob.Compile(v.Platform)
			if err != nil {
				return nil, &mergeFailure{reason: fmt.Sprintf(
					"the platform '%s' is not a valid pattern", v.Platform)}
			}
			if !g.Match(rid) {
				continue
			}
		}
		sel = append(sel, i)
	}
	if len(sel) == 0 {
		return nil, nil
	}

	m := &mergedVariant{}
	seen := map[string]bool{}
	for _, i := range sel {
		if nullMember(t.raw[i], "dependencies") {
			return nil, fmt.Errorf("variants[%d] has \"dependencies\": null", i)
		}
		for _, d := range t.m.Variants[i].Dependencies {
			if seen[d.ID] {
				return nil, &mergeFailure{reason: reasonDependency}
			}
			seen[d.ID] = true
			m.deps = append(m.deps, d)
		}
	}
	for _, i := range sel {
		if nullMember(t.raw[i], "assets") {
			return nil, fmt.Errorf("variants[%d] has \"assets\": null", i)
		}
		m.assets = append(m.assets, t.m.Variants[i].Assets...)
		if list, ok := t.raw[i].Get("assets"); ok {
			for _, a := range list.([]any) {
				m.rawAssets = append(m.rawAssets, a.(*doc.Object))
			}
		} else {
			for range t.m.Variants[i].Assets {
				m.rawAssets = append(m.rawAssets, doc.NewObject())
			}
		}
	}
	for _, f := range []struct {
		key string
		dst *[]string
		get func(*manifest.Variant) []string
	}{
		{"preserve_files", &m.preserve, func(v *manifest.Variant) []string { return v.PreserveFiles }},
		{"remove_files", &m.remove, func(v *manifest.Variant) []string { return v.RemoveFiles }},
	} {
		for _, i := range sel {
			if nullMember(t.raw[i], f.key) {
				return nil, fmt.Errorf("variants[%d] has \"%s\": null", i, f.key)
			}
			*f.dst = append(*f.dst, f.get(&t.m.Variants[i])...)
		}
	}

	sc := &m.scripts
	for _, f := range []struct {
		key string
		dst *[]string
		get func(*manifest.Scripts) []string
	}{
		{"pre_install", &sc.PreInstall, func(s *manifest.Scripts) []string { return s.PreInstall }},
		{"install", &sc.Install, func(s *manifest.Scripts) []string { return s.Install }},
		{"post_install", &sc.PostInstall, func(s *manifest.Scripts) []string { return s.PostInstall }},
		{"pre_uninstall", &sc.PreUninstall, func(s *manifest.Scripts) []string { return s.PreUninstall }},
		{"uninstall", &sc.Uninstall, func(s *manifest.Scripts) []string { return s.Uninstall }},
		{"post_uninstall", &sc.PostUninstall, func(s *manifest.Scripts) []string { return s.PostUninstall }},
	} {
		// SingleOrDefault(v => v.Scripts.<kind>.Count > 0): it fails on the
		// second match, and dereferences each variant until then.
		found := 0
		for _, i := range sel {
			if nullMember(t.raw[i], "scripts") {
				return nil, fmt.Errorf("variants[%d] has \"scripts\": null", i)
			}
			if s, ok := t.raw[i].Get("scripts"); ok && nullMember(s.(*doc.Object), f.key) {
				return nil, fmt.Errorf("variants[%d].scripts has \"%s\": null", i, f.key)
			}
			if list := f.get(&t.m.Variants[i].Scripts); len(list) > 0 {
				found++
				if found > 1 {
					return nil, &mergeFailure{reason: reasonScript}
				}
				*f.dst = list
			}
		}
	}
	return m, nil
}

// convertEntry is ToothCompat.ConvertEntry: the entry of a package file for
// the variants of a label merged for a runtime, for a platform and the
// runtime; nil when an asset has the type tar, tgz or uncompressed, which
// makes the variant absent. Placements and globs that are not valid in a
// package file are left out.
func convertEntry(m *mergedVariant, platform, runtime string) (*doc.Object, error) {
	entry := doc.NewObject()
	entry.Set("platform", platform)
	entry.Set("runtime", runtime)

	if len(m.deps) > 0 {
		deps := doc.NewObject()
		for _, d := range m.deps {
			deps.Set(d.ID, d.Range)
		}
		entry.Set("deps", deps)
	}

	var assets []any
	for i, a := range m.assets {
		if a.Type != manifest.AssetSelf && a.Type != manifest.AssetZip {
			return nil, nil
		}
		raw := m.rawAssets[i]
		converted := doc.NewObject()
		converted.Set("type", string(a.Type))
		if a.Type == manifest.AssetZip {
			if nullMember(raw, "urls") {
				return nil, fmt.Errorf("an asset of type zip has \"urls\": null")
			}
			urls := make([]any, len(a.URLs))
			for j, u := range a.URLs {
				urls[j] = u
			}
			converted.Set("urls", urls)
		}
		if nullMember(raw, "placements") {
			return nil, fmt.Errorf("an asset has \"placements\": null")
		}
		var place []any
		for _, p := range a.Placements {
			src, dest := ConvertPlacement(p)
			if IsValidPlacement(src, dest) {
				o := doc.NewObject()
				o.Set("src", src)
				o.Set("dest", dest)
				place = append(place, o)
			}
		}
		if len(place) > 0 {
			converted.Set("place", place)
		}
		assets = append(assets, converted)
	}
	if len(assets) > 0 {
		entry.Set("assets", assets)
	}

	addGlobs(entry, "preserve", PreserveFiles(m.preserve))
	addGlobs(entry, "purge", RemoveFiles(m.remove))

	h := ConvertScripts(m.scripts)
	hooks := doc.NewObject()
	addCommands(hooks, "pre_install", h.PreInstall)
	addCommands(hooks, "post_install", h.PostInstall)
	addCommands(hooks, "pre_uninstall", h.PreUninstall)
	addCommands(hooks, "post_uninstall", h.PostUninstall)
	if hooks.Len() > 0 {
		entry.Set("hooks", hooks)
	}
	return entry, nil
}

func addGlobs(entry *doc.Object, key string, globs []string) {
	var list []any
	for _, g := range globs {
		if TryParseGlob(g) == nil {
			continue
		}
		list = append(list, g)
	}
	if len(list) > 0 {
		entry.Set(key, list)
	}
}

func addCommands(hooks *doc.Object, key string, commands []string) {
	if len(commands) > 0 {
		list := make([]any, len(commands))
		for i, c := range commands {
			list[i] = c
		}
		hooks.Set(key, list)
	}
}

// ConvertPlacement is ToothCompat.ConvertPlacement: the member of place for a
// placement of format 3: '\' becomes '/', empty and "." segments are removed;
// a "dir" placement has src and dest each followed by '/' unless empty, a
// "file" placement whose src has '*', '?' or '[' has dest followed by '/'
// unless empty, any other "file" placement is the file form.
func ConvertPlacement(p manifest.Placement) (src, dest string) {
	src, dest = NormalizePath(p.Src), NormalizePath(p.Dest)
	dir := func(path string) string {
		if path == "" {
			return ""
		}
		return path + "/"
	}
	switch {
	case p.Type == manifest.PlacementDir:
		return dir(src), dir(dest)
	case pathglob.HasMeta(src):
		return src, dir(dest)
	default:
		return src, dest
	}
}

// NormalizePath is ToothCompat.NormalizePath: path with '\' replaced by '/'
// and its empty and "." segments removed.
func NormalizePath(path string) string {
	var segs []string
	for _, s := range strings.Split(strings.ReplaceAll(path, `\`, "/"), "/") {
		if s != "" && s != "." {
			segs = append(segs, s)
		}
	}
	return strings.Join(segs, "/")
}

// PreserveFiles is ToothCompat.PreserveFiles: the preserve of a package file
// for the patterns of preserve_files: '\' becomes '/', and a pattern without
// '/' is prefixed by "**/".
func PreserveFiles(patterns []string) []string {
	out := make([]string, 0, len(patterns))
	for _, p := range patterns {
		p = strings.ReplaceAll(p, `\`, "/")
		if !strings.Contains(p, "/") {
			p = "**/" + p
		}
		out = append(out, p)
	}
	return out
}

// RemoveFiles is ToothCompat.RemoveFiles: the purge of a package file for the
// patterns of remove_files: '\' becomes '/', and a pattern not ending with
// '/' is also given with a trailing '/'.
func RemoveFiles(patterns []string) []string {
	out := make([]string, 0, 2*len(patterns))
	for _, p := range patterns {
		p = strings.ReplaceAll(p, `\`, "/")
		if strings.HasSuffix(p, "/") {
			out = append(out, p)
		} else {
			out = append(out, p, p+"/")
		}
	}
	return out
}

// ConvertScripts is ToothCompat.ConvertScripts: "install" runs after the files
// were placed, together with "post_install", and "uninstall" before the files
// are removed, together with "pre_uninstall".
func ConvertScripts(s manifest.Scripts) pkgmodel.Hooks {
	cat := func(a, b []string) []string { return append(append([]string{}, a...), b...) }
	return pkgmodel.Hooks{
		PreInstall:    cat(s.PreInstall, nil),
		PostInstall:   cat(s.Install, s.PostInstall),
		PreUninstall:  cat(s.PreUninstall, s.Uninstall),
		PostUninstall: cat(s.PostUninstall, nil),
	}
}

// TryParseGlob is ToothCompat.TryParseGlob: the glob of a converted pattern,
// or nil for a pattern that is not a valid glob of a package file.
func TryParseGlob(pattern string) *pathglob.Glob {
	g, err := pathglob.Compile(pattern)
	if err != nil {
		return nil
	}
	return g
}

// IsLogo is ToothCompat.IsLogo: whether an "avatar_url" of format 3 is a
// "logo" of a package file: it starts with "https://".
func IsLogo(avatarURL string) bool { return strings.HasPrefix(avatarURL, "https://") }

// convertInfo is ToothCompat.Info: the info of a package file for the info of
// format 3 (members absent or null in it are absent), with "avatar_url" as
// "logo" when it starts with "https://" and dropped otherwise.
func convertInfo(in *doc.Object) (*doc.Object, *pkgmodel.Info) {
	out := doc.NewObject()
	info := &pkgmodel.Info{}
	if s, ok := stringMember(in, "name"); ok {
		out.Set("name", s)
		info.Name = s
	}
	if s, ok := stringMember(in, "description"); ok {
		out.Set("description", s)
		info.Description = s
	}
	if v, _ := in.Get("tags"); v != nil {
		list := v.([]any)
		tags := make([]any, len(list))
		for i, tag := range list {
			tags[i] = tag
			info.Tags = append(info.Tags, tag.(string))
		}
		out.Set("tags", tags)
	}
	if s, ok := stringMember(in, "avatar_url"); ok && IsLogo(s) {
		out.Set("logo", s)
		info.Logo = s
	}
	return out, info
}

func stringMember(o *doc.Object, key string) (string, bool) {
	v, _ := o.Get(key)
	s, ok := v.(string)
	return s, ok
}
