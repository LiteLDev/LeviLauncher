package migrate

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/mod/module"
)

// This file ports lip 0.37.0
// src/Lip.Core/Migration/PackageManifests/PackageManifestMigration.cs and the
// PackageManifestV1*/PackageManifestV2* DTOs. The DTOs are bound with
// System.Text.Json default options, so binding here follows the same rules:
// case-sensitive property names, unknown properties ignored, the last of
// duplicate properties wins, `required` properties must be present (null is
// accepted), and a value of the wrong JSON type is an error. Nullable
// reference-type members are modelled as pointers / nil slices so that the
// C# code's null handling (including the paths that throw) is preserved.

// --- binding helpers ---

func bindError(path, format string, args ...any) error {
	return fmt.Errorf("migrate: %s: %s", path, fmt.Sprintf(format, args...))
}

// requireProps mirrors the JsonRequired check for `required` members.
func requireProps(obj *jvalue, path, typ string, names ...string) error {
	var missing []string
	for _, n := range names {
		if _, ok := obj.get(n); !ok {
			missing = append(missing, "'"+n+"'")
		}
	}
	if len(missing) > 0 {
		return bindError(path, "%s is missing required properties %s", typ, strings.Join(missing, ", "))
	}
	return nil
}

// bindObject returns nil for an absent or null value.
func bindObject(v *jvalue, path, typ string) (*jvalue, error) {
	if v == nil || v.kind == jNull {
		return nil, nil
	}
	if v.kind != jObject {
		return nil, bindError(path, "the JSON value could not be converted to %s", typ)
	}
	return v, nil
}

func bindString(v *jvalue, path string) (*string, error) {
	if v == nil || v.kind == jNull {
		return nil, nil
	}
	if v.kind != jString {
		return nil, bindError(path, "the JSON value could not be converted to System.String")
	}
	s := v.str
	return &s, nil
}

func prop(obj *jvalue, key string) *jvalue {
	v, _ := obj.get(key)
	return v
}

// bindStringList binds List<string>?: nil for absent/null, otherwise a
// non-nil slice whose elements are nil for JSON null.
func bindStringList(v *jvalue, path string) ([]*string, error) {
	if v == nil || v.kind == jNull {
		return nil, nil
	}
	if v.kind != jArray {
		return nil, bindError(path, "the JSON value could not be converted to List<string>")
	}
	out := make([]*string, 0, len(v.arr))
	for i, e := range v.arr {
		s, err := bindString(e, fmt.Sprintf("%s[%d]", path, i))
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

type dictEntry[T any] struct {
	key string
	val T
}

// bindDict binds Dictionary<string, T>?: nil for absent/null. Duplicate keys
// keep the first position and the last value, like the indexer assignment
// System.Text.Json uses.
func bindDict[T any](v *jvalue, path string, bind func(*jvalue, string) (T, error)) ([]dictEntry[T], error) {
	if v == nil || v.kind == jNull {
		return nil, nil
	}
	if v.kind != jObject {
		return nil, bindError(path, "the JSON value could not be converted to a dictionary")
	}
	out := []dictEntry[T]{}
	idx := map[string]int{}
	// Every occurrence is bound (and type-checked), as System.Text.Json
	// reads them all.
	for _, m := range v.obj {
		val, err := bind(m.val, path+"['"+m.key+"']")
		if err != nil {
			return nil, err
		}
		if i, ok := idx[m.key]; ok {
			out[i].val = val
			continue
		}
		idx[m.key] = len(out)
		out = append(out, dictEntry[T]{key: m.key, val: val})
	}
	return out, nil
}

// --- format 2 DTOs ---

type v2Manifest struct {
	tooth, version *string
	info           *v2Info
	assetURL       *string
	commands       *v2Commands
	dependencies   []dictEntry[*string] // nil: null
	files          *v2Files
	platforms      []*v2Platform // nil: null; nil elements: JSON null
}

type v2Info struct {
	name, description *string
	tags              []*string // nil: null
	avatarURL         *string
}

type v2Commands struct {
	preInstall, postInstall, preUninstall, postUninstall []*string
}

func (c *v2Commands) hasCommands() bool {
	return c.preInstall != nil || c.postInstall != nil || c.preUninstall != nil || c.postUninstall != nil
}

type v2Files struct {
	place            []*v2Place // nil: null; nil elements: JSON null
	preserve, remove []*string
}

type v2Place struct{ src, dest *string }

type v2Platform struct {
	goos, goarch, assetURL *string
	commands               *v2Commands
	dependencies           []dictEntry[*string]
	files                  *v2Files
}

func bindV2(root *jvalue) (*v2Manifest, error) {
	const p = "$"
	if err := requireProps(root, p, "PackageManifestV2", "format_version", "tooth", "version", "info"); err != nil {
		return nil, err
	}
	m := &v2Manifest{}
	var err error
	if m.tooth, err = bindString(prop(root, "tooth"), p+".tooth"); err != nil {
		return nil, err
	}
	if m.version, err = bindString(prop(root, "version"), p+".version"); err != nil {
		return nil, err
	}
	if m.info, err = bindV2Info(prop(root, "info"), p+".info"); err != nil {
		return nil, err
	}
	if m.assetURL, err = bindString(prop(root, "asset_url"), p+".asset_url"); err != nil {
		return nil, err
	}
	if m.commands, err = bindV2Commands(prop(root, "commands"), p+".commands"); err != nil {
		return nil, err
	}
	if m.dependencies, err = bindDict(prop(root, "dependencies"), p+".dependencies", bindString); err != nil {
		return nil, err
	}
	if _, err = bindDict(prop(root, "prerequisites"), p+".prerequisites", bindString); err != nil {
		return nil, err
	}
	if m.files, err = bindV2Files(prop(root, "files"), p+".files"); err != nil {
		return nil, err
	}
	if v := prop(root, "platforms"); v != nil && v.kind != jNull {
		if v.kind != jArray {
			return nil, bindError(p+".platforms", "the JSON value could not be converted to List<PackageManifestV2Platform>")
		}
		m.platforms = make([]*v2Platform, 0, len(v.arr))
		for i, e := range v.arr {
			pl, err := bindV2Platform(e, fmt.Sprintf("%s.platforms[%d]", p, i))
			if err != nil {
				return nil, err
			}
			m.platforms = append(m.platforms, pl)
		}
	}
	return m, nil
}

func bindV2Info(v *jvalue, path string) (*v2Info, error) {
	obj, err := bindObject(v, path, "PackageManifestV2Info")
	if obj == nil || err != nil {
		return nil, err
	}
	if err := requireProps(obj, path, "PackageManifestV2Info", "name", "description", "author", "tags"); err != nil {
		return nil, err
	}
	info := &v2Info{}
	if info.name, err = bindString(prop(obj, "name"), path+".name"); err != nil {
		return nil, err
	}
	if info.description, err = bindString(prop(obj, "description"), path+".description"); err != nil {
		return nil, err
	}
	if _, err = bindString(prop(obj, "author"), path+".author"); err != nil {
		return nil, err
	}
	if info.tags, err = bindStringList(prop(obj, "tags"), path+".tags"); err != nil {
		return nil, err
	}
	if info.avatarURL, err = bindString(prop(obj, "avatar_url"), path+".avatar_url"); err != nil {
		return nil, err
	}
	return info, nil
}

func bindV2Commands(v *jvalue, path string) (*v2Commands, error) {
	obj, err := bindObject(v, path, "PackageManifestV2Commands")
	if obj == nil || err != nil {
		return nil, err
	}
	c := &v2Commands{}
	for _, f := range []struct {
		key string
		dst *[]*string
	}{
		{"pre_install", &c.preInstall},
		{"post_install", &c.postInstall},
		{"pre_uninstall", &c.preUninstall},
		{"post_uninstall", &c.postUninstall},
	} {
		if *f.dst, err = bindStringList(prop(obj, f.key), path+"."+f.key); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func bindV2Files(v *jvalue, path string) (*v2Files, error) {
	obj, err := bindObject(v, path, "PackageManifestV2Files")
	if obj == nil || err != nil {
		return nil, err
	}
	f := &v2Files{}
	if pv := prop(obj, "place"); pv != nil && pv.kind != jNull {
		if pv.kind != jArray {
			return nil, bindError(path+".place", "the JSON value could not be converted to List<PackageManifestV2Place>")
		}
		f.place = make([]*v2Place, 0, len(pv.arr))
		for i, e := range pv.arr {
			ep := fmt.Sprintf("%s.place[%d]", path, i)
			eo, err := bindObject(e, ep, "PackageManifestV2Place")
			if err != nil {
				return nil, err
			}
			if eo == nil {
				f.place = append(f.place, nil)
				continue
			}
			if err := requireProps(eo, ep, "PackageManifestV2Place", "src", "dest"); err != nil {
				return nil, err
			}
			pl := &v2Place{}
			if pl.src, err = bindString(prop(eo, "src"), ep+".src"); err != nil {
				return nil, err
			}
			if pl.dest, err = bindString(prop(eo, "dest"), ep+".dest"); err != nil {
				return nil, err
			}
			f.place = append(f.place, pl)
		}
	}
	if f.preserve, err = bindStringList(prop(obj, "preserve"), path+".preserve"); err != nil {
		return nil, err
	}
	if f.remove, err = bindStringList(prop(obj, "remove"), path+".remove"); err != nil {
		return nil, err
	}
	return f, nil
}

func bindV2Platform(v *jvalue, path string) (*v2Platform, error) {
	obj, err := bindObject(v, path, "PackageManifestV2Platform")
	if obj == nil || err != nil {
		return nil, err
	}
	if err := requireProps(obj, path, "PackageManifestV2Platform", "goos"); err != nil {
		return nil, err
	}
	p := &v2Platform{}
	if p.goarch, err = bindString(prop(obj, "goarch"), path+".goarch"); err != nil {
		return nil, err
	}
	if p.goos, err = bindString(prop(obj, "goos"), path+".goos"); err != nil {
		return nil, err
	}
	if p.assetURL, err = bindString(prop(obj, "asset_url"), path+".asset_url"); err != nil {
		return nil, err
	}
	if p.commands, err = bindV2Commands(prop(obj, "commands"), path+".commands"); err != nil {
		return nil, err
	}
	if p.dependencies, err = bindDict(prop(obj, "dependencies"), path+".dependencies", bindString); err != nil {
		return nil, err
	}
	if _, err = bindDict(prop(obj, "prerequisites"), path+".prerequisites", bindString); err != nil {
		return nil, err
	}
	if p.files, err = bindV2Files(prop(obj, "files"), path+".files"); err != nil {
		return nil, err
	}
	return p, nil
}

// --- format 1 DTOs ---

type v1Manifest struct {
	tooth, version *string
	dependencies   []dictEntry[[][]*string] // nil: null; val nil: null; inner nil: null
	information    *jvalue                  // JsonExtensionData object, nil when null/absent
	placement      []*v1Placement
	commands       []*v1Command
}

// v1Placement omits GOOS/GOARCH: they are bound (type-checked) but the
// migration ignores them, so placements always apply to every platform.
type v1Placement struct{ source, destination *string }

type v1Command struct {
	typ, goos, goarch *string
	commands          []*string
}

func bindV1(root *jvalue) (*v1Manifest, error) {
	const p = "$"
	if err := requireProps(root, p, "PackageManifestV1", "format_version", "tooth", "version"); err != nil {
		return nil, err
	}
	m := &v1Manifest{}
	var err error
	if m.tooth, err = bindString(prop(root, "tooth"), p+".tooth"); err != nil {
		return nil, err
	}
	if m.version, err = bindString(prop(root, "version"), p+".version"); err != nil {
		return nil, err
	}
	bindGroups := func(v *jvalue, path string) ([][]*string, error) {
		if v == nil || v.kind == jNull {
			return nil, nil
		}
		if v.kind != jArray {
			return nil, bindError(path, "the JSON value could not be converted to List<List<string>>")
		}
		out := make([][]*string, 0, len(v.arr))
		for i, e := range v.arr {
			g, err := bindStringList(e, fmt.Sprintf("%s[%d]", path, i))
			if err != nil {
				return nil, err
			}
			out = append(out, g)
		}
		return out, nil
	}
	if m.dependencies, err = bindDict(prop(root, "dependencies"), p+".dependencies", bindGroups); err != nil {
		return nil, err
	}
	if m.information, err = bindObject(prop(root, "information"), p+".information", "PackageManifestV1Information"); err != nil {
		return nil, err
	}
	if v := prop(root, "placement"); v != nil && v.kind != jNull {
		if v.kind != jArray {
			return nil, bindError(p+".placement", "the JSON value could not be converted to List<PackageManifestV1Placement>")
		}
		m.placement = make([]*v1Placement, 0, len(v.arr))
		for i, e := range v.arr {
			ep := fmt.Sprintf("%s.placement[%d]", p, i)
			eo, err := bindObject(e, ep, "PackageManifestV1Placement")
			if err != nil {
				return nil, err
			}
			if eo == nil {
				m.placement = append(m.placement, nil)
				continue
			}
			if err := requireProps(eo, ep, "PackageManifestV1Placement", "source", "destination"); err != nil {
				return nil, err
			}
			pl := &v1Placement{}
			var ignored *string
			for _, f := range []struct {
				key string
				dst **string
			}{{"source", &pl.source}, {"destination", &pl.destination}, {"GOOS", &ignored}, {"GOARCH", &ignored}} {
				if *f.dst, err = bindString(prop(eo, f.key), ep+"."+f.key); err != nil {
					return nil, err
				}
			}
			m.placement = append(m.placement, pl)
		}
	}
	if _, err = bindStringList(prop(root, "possession"), p+".possession"); err != nil {
		return nil, err
	}
	if v := prop(root, "commands"); v != nil && v.kind != jNull {
		if v.kind != jArray {
			return nil, bindError(p+".commands", "the JSON value could not be converted to List<PackageManifestV1Command>")
		}
		m.commands = make([]*v1Command, 0, len(v.arr))
		for i, e := range v.arr {
			ep := fmt.Sprintf("%s.commands[%d]", p, i)
			eo, err := bindObject(e, ep, "PackageManifestV1Command")
			if err != nil {
				return nil, err
			}
			if eo == nil {
				m.commands = append(m.commands, nil)
				continue
			}
			if err := requireProps(eo, ep, "PackageManifestV1Command", "type", "commands"); err != nil {
				return nil, err
			}
			c := &v1Command{}
			if c.typ, err = bindString(prop(eo, "type"), ep+".type"); err != nil {
				return nil, err
			}
			if c.commands, err = bindStringList(prop(eo, "commands"), ep+".commands"); err != nil {
				return nil, err
			}
			if c.goos, err = bindString(prop(eo, "GOOS"), ep+".GOOS"); err != nil {
				return nil, err
			}
			if c.goarch, err = bindString(prop(eo, "GOARCH"), ep+".GOARCH"); err != nil {
				return nil, err
			}
			m.commands = append(m.commands, c)
		}
	}
	return m, nil
}

// --- MigrateV1ToV2 ---

func strPtrEq(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// equalOrdinalIgnoreCase approximates StringComparison.OrdinalIgnoreCase
// (per-character simple upper-casing).
func equalOrdinalIgnoreCase(a, b string) bool {
	ra, rb := []rune(a), []rune(b)
	if len(ra) != len(rb) {
		return false
	}
	for i := range ra {
		if unicode.ToUpper(ra[i]) != unicode.ToUpper(rb[i]) {
			return false
		}
	}
	return true
}

func migrateV1ToV2(m *v1Manifest) (*v2Manifest, error) {
	infoString := func(key string) (*string, error) {
		empty := ""
		if m.information == nil {
			return &empty, nil
		}
		v, ok := m.information.get(key)
		if !ok || v.kind == jNull {
			return &empty, nil
		}
		if v.kind != jString {
			return nil, fmt.Errorf("migrate: information.%s: the requested operation requires an element of type 'String', but the target element has type '%s'", key, v.kind)
		}
		s := v.str
		return &s, nil
	}
	info := &v2Info{tags: []*string{}}
	var err error
	if info.name, err = infoString("name"); err != nil {
		return nil, err
	}
	if info.description, err = infoString("description"); err != nil {
		return nil, err
	}
	if _, err = infoString("author"); err != nil {
		return nil, err
	}
	if m.information != nil {
		if t, ok := m.information.get("tags"); ok && t.kind == jArray {
			for _, e := range t.arr {
				s := ""
				switch e.kind {
				case jNull:
				case jString:
					s = e.str
				default:
					return nil, fmt.Errorf("migrate: information.tags: the requested operation requires an element of type 'String', but the target element has type '%s'", e.kind)
				}
				info.tags = append(info.tags, &s)
			}
		}
	}

	var platforms []*v2Platform
	getOrCreatePlatform := func(goos string, goarch *string) *v2Platform {
		for _, p := range platforms {
			if *p.goos == goos && strPtrEq(p.goarch, goarch) {
				return p
			}
		}
		g := goos
		p := &v2Platform{goos: &g, goarch: goarch}
		platforms = append(platforms, p)
		return p
	}

	var commands *v2Commands
	if m.commands != nil {
		root := &v2Commands{}
		for i, cmd := range m.commands {
			// lip v0.38 checks each command before it is read, whatever its
			// type.
			if cmd == nil {
				return nil, fmt.Errorf("migrate: commands[%d] is null", i)
			}
			if cmd.typ == nil {
				return nil, fmt.Errorf("migrate: commands[%d].type is null", i)
			}
			if cmd.commands == nil {
				return nil, fmt.Errorf("migrate: commands[%d].commands is null", i)
			}
			var target *v2Commands
			if cmd.goos == nil || *cmd.goos == "" {
				target = root
			} else {
				p := getOrCreatePlatform(*cmd.goos, cmd.goarch)
				if p.commands == nil {
					p.commands = &v2Commands{}
				}
				target = p.commands
			}
			var list *[]*string
			switch {
			case equalOrdinalIgnoreCase(*cmd.typ, "install"):
				list = &target.postInstall
			case equalOrdinalIgnoreCase(*cmd.typ, "uninstall"):
				list = &target.preUninstall
			default:
				continue
			}
			if *list == nil {
				*list = []*string{}
			}
			*list = append(*list, cmd.commands...)
		}
		if root.hasCommands() {
			commands = root
		}
	}

	var deps []dictEntry[*string]
	if m.dependencies != nil {
		deps = make([]dictEntry[*string], 0, len(m.dependencies))
		for _, d := range m.dependencies {
			if d.val == nil {
				return nil, fmt.Errorf("migrate: dependencies[%q] is null", d.key)
			}
			groups := make([]string, 0, len(d.val))
			for _, g := range d.val {
				if g == nil {
					return nil, fmt.Errorf("migrate: dependencies[%q] contains null", d.key)
				}
				parts := make([]string, 0, len(g))
				for _, s := range g {
					if s != nil {
						parts = append(parts, *s)
					} else {
						parts = append(parts, "")
					}
				}
				groups = append(groups, strings.Join(parts, " && "))
			}
			s := strings.Join(groups, " || ")
			deps = append(deps, dictEntry[*string]{key: d.key, val: &s})
		}
	}

	var files *v2Files
	if m.placement != nil {
		files = &v2Files{place: make([]*v2Place, 0, len(m.placement))}
		for i, p := range m.placement {
			if p == nil {
				return nil, fmt.Errorf("migrate: placement[%d] is null", i)
			}
			if p.source == nil || p.destination == nil {
				return nil, fmt.Errorf("migrate: placement[%d] needs the strings source and destination", i)
			}
			dest := strings.TrimRight(*p.destination, "*")
			files.place = append(files.place, &v2Place{src: p.source, dest: &dest})
		}
	}

	return &v2Manifest{
		tooth:        m.tooth,
		version:      m.version,
		info:         info,
		commands:     commands,
		dependencies: deps,
		files:        files,
		platforms:    platforms,
	}, nil
}

// --- MigrateV2ToV3 ---

var templateVariableRe = regexp.MustCompile(`\$\(([^)]+?)\)`)

var goosPrefix = map[string]string{
	"windows": "win",
	"darwin":  "osx",
	"linux":   "linux",
	"ios":     "ios",
	"android": "android",
}

var goarchSuffix = map[string]string{
	"amd64":   "x64",
	"386":     "x86",
	"arm64":   "arm64",
	"arm":     "arm",
	"loong64": "loongarch64",
}

// variantRe is PackageId's VariantRegex. .NET's "$" also matches before a
// final "\n", which the optional "\n" reproduces.
var variantRe = regexp.MustCompile(`^([a-z0-9_]+(/[a-z0-9_]+)?)?\n?$`)

// packageIDRe is PackageId's SelfRegex (.NET's end-of-line "$" quirk cannot
// change the captures here because both groups are greedy up to the end).
var packageIDRe = regexp.MustCompile(`^([^#]+)(?:#([^#]*))?$`)

// parsePackageID ports PackageId.Parse and returns PackageId.ToString().
func parsePackageID(s string) (string, bool) {
	m := packageIDRe.FindStringSubmatch(s)
	if m == nil {
		return "", false
	}
	path, variant := m[1], m[2]
	if module.CheckPath(path) != nil || !variantRe.MatchString(variant) {
		return "", false
	}
	if variant == "" {
		return path, true
	}
	return path + "#" + variant, true
}

func assetType(url string) string {
	ext := url
	if i := strings.LastIndexByte(url, '.'); i >= 0 {
		ext = url[i+1:]
	}
	switch ext {
	case "tar":
		return "tar"
	case "tgz", "gz":
		return "tgz"
	case "zip":
		return "zip"
	}
	return "uncompressed"
}

type v2Converter struct {
	m *v2Manifest
}

func (c *v2Converter) scope(name string) (*jvalue, bool) {
	switch name {
	case "version":
		return jstrp(c.m.version), true
	case "path":
		return jstrp(c.m.tooth), true
	}
	return nil, false
}

// render is RenderTemplate: null -> "", $(var) -> {{var}}, then Scriban
// with model {version, path}.
func (c *v2Converter) render(s *string) (string, error) {
	if s == nil {
		return "", nil
	}
	return renderTemplate(templateVariableRe.ReplaceAllString(*s, "{{${1}}}"), c.scope)
}

func (c *v2Converter) renderList(list []*string) (*jvalue, error) {
	out := jarr()
	for _, s := range list {
		r, err := c.render(s)
		if err != nil {
			return nil, err
		}
		out.arr = append(out.arr, jstr(r))
	}
	return out, nil
}

// renderGlobs renders preserve/remove patterns; Glob.Parse rejects "".
func (c *v2Converter) renderGlobs(list []*string) (*jvalue, error) {
	out, err := c.renderList(list)
	if err != nil {
		return nil, err
	}
	for _, e := range out.arr {
		if e.str == "" {
			return nil, fmt.Errorf("migrate: empty glob pattern")
		}
	}
	return out, nil
}

func (c *v2Converter) dependencies(entries []dictEntry[*string], npm bool) *jvalue {
	out := newObj()
	idx := map[string]int{}
	for _, e := range entries {
		id, ok := parsePackageID(e.key)
		if !ok || e.val == nil {
			continue // invalid dependencies are ignored
		}
		r, err := NormalizeRange(*e.val, npm)
		if err != nil {
			continue
		}
		if i, ok := idx[id]; ok {
			out.v.obj[i].val = jstr(r)
			continue
		}
		idx[id] = len(out.v.obj)
		out.set(id, jstr(r))
	}
	return out.v
}

func (c *v2Converter) placements(places []*v2Place) (*jvalue, error) {
	out := jarr()
	for i, pl := range places {
		if pl == nil {
			return nil, fmt.Errorf("migrate: files.place[%d] is null", i)
		}
		src, err := c.render(pl.src)
		if err != nil {
			return nil, err
		}
		dst, err := c.render(pl.dest)
		if err != nil {
			return nil, err
		}
		if pl.src == nil {
			return nil, fmt.Errorf("migrate: files.place[%d].src is null", i)
		}
		typ := "file"
		if strings.HasSuffix(*pl.src, "*") {
			typ = "dir"
		}
		out.arr = append(out.arr, newObj().
			set("type", jstr(typ)).
			set("src", jstr(strings.TrimRight(src, "*"))).
			set("dest", jstr(dst)).v)
	}
	return out, nil
}

func (c *v2Converter) scripts(cmds *v2Commands) (*jvalue, error) {
	var lists [4][]*string
	if cmds != nil {
		lists = [4][]*string{cmds.preInstall, cmds.postInstall, cmds.preUninstall, cmds.postUninstall}
	}
	var rendered [4]*jvalue
	for i, l := range lists {
		r, err := c.renderList(l)
		if err != nil {
			return nil, err
		}
		rendered[i] = r
	}
	return scriptsObj(rendered[0], rendered[1], rendered[2], rendered[3]), nil
}

func scriptsObj(preInstall, postInstall, preUninstall, postUninstall *jvalue) *jvalue {
	return newObj().
		set("pre_install", preInstall).
		set("install", jarr()).
		set("post_install", postInstall).
		set("pre_uninstall", preUninstall).
		set("uninstall", jarr()).
		set("post_uninstall", postUninstall).v
}

func asset(typ string, urls, placements *jvalue) *jvalue {
	return newObj().set("type", jstr(typ)).set("urls", urls).set("placements", placements).v
}

func variant(platform string, deps, assets, preserve, remove, scripts *jvalue) *jvalue {
	return newObj().
		set("label", jstr("")).
		set("platform", jstr(platform)).
		set("dependencies", deps).
		set("assets", assets).
		set("preserve_files", preserve).
		set("remove_files", remove).
		set("scripts", scripts).v
}

func emptyVariant(platform string) *jvalue {
	return variant(platform, newObj().v, jarr(), jarr(), jarr(), scriptsObj(jarr(), jarr(), jarr(), jarr()))
}

// migrateV2ToV3 ports MigrateV2ToV3 and returns the format 3 document.
// Version is emitted as written (package manifest normalizes it); tooth
// path, tags and placement destinations are validated by manifest.Parse.
func migrateV2ToV3(m *v2Manifest) (*jvalue, error) {
	c := &v2Converter{m: m}
	if m.tooth == nil {
		return nil, fmt.Errorf("migrate: invalid package path: null")
	}
	if m.version == nil {
		return nil, fmt.Errorf("migrate: version is null")
	}
	if m.info == nil {
		return nil, fmt.Errorf("migrate: info is null")
	}
	// Since lip v0.38 the tags of format 3 may be null (absent from the
	// derived info); a null tag fails the tag check.
	tags := jnull()
	if m.info.tags != nil {
		list := make([]string, 0, len(m.info.tags))
		for _, t := range m.info.tags {
			if t == nil {
				return nil, fmt.Errorf("migrate: info.tags contains null")
			}
			list = append(list, *t)
		}
		tags = jstrs(list)
	}
	avatar, err := c.render(m.info.avatarURL)
	if err != nil {
		return nil, err
	}

	// Without platforms, the root fields make one variant without a
	// platform, whose ranges are read as SemVer ranges and whose places,
	// without an asset_url, are of a self asset; a platform uses its own
	// fields and else the root ones, with ranges read as npm ranges. A
	// platform that is JSON null is read like the root (lip v0.38 iterates
	// the platforms as PackageManifestV2Platform? and tests for null).
	platforms := m.platforms
	if platforms == nil {
		platforms = []*v2Platform{nil}
	}
	var variants []*jvalue
	for _, p := range platforms {
		var pFiles *v2Files
		var pDeps []dictEntry[*string]
		var pAssetURL *string
		var pCommands *v2Commands
		if p != nil {
			pFiles, pDeps, pAssetURL, pCommands = p.files, p.dependencies, p.assetURL, p.commands
		}

		depSrc := pDeps
		if depSrc == nil {
			depSrc = m.dependencies
		}
		deps := c.dependencies(depSrc, p != nil)

		var places []*v2Place
		if pFiles != nil && pFiles.place != nil {
			places = pFiles.place
		} else if m.files != nil {
			places = m.files.place
		}
		assets := jarr()
		assetURL := pAssetURL
		if assetURL == nil {
			assetURL = m.assetURL
		}
		if assetURL != nil {
			url, err := c.render(assetURL)
			if err != nil {
				return nil, err
			}
			pls, err := c.placements(places)
			if err != nil {
				return nil, err
			}
			assets.arr = append(assets.arr, asset(assetType(url), jstrs([]string{url}), pls))
		} else if p == nil && places != nil {
			pls, err := c.placements(places)
			if err != nil {
				return nil, err
			}
			assets.arr = append(assets.arr, asset("self", jarr(), pls))
		}

		// lip reads the platform after the asset, before the files and
		// scripts.
		platform := ""
		if p != nil {
			var err error
			if platform, err = platformOf(p); err != nil {
				return nil, err
			}
		}

		cmds := pCommands
		if cmds == nil {
			cmds = m.commands
		}
		scripts, err := c.scripts(cmds)
		if err != nil {
			return nil, err
		}

		pick := func(get func(*v2Files) []*string) []*string {
			if pFiles != nil {
				if l := get(pFiles); l != nil {
					return l
				}
			}
			if m.files != nil {
				return get(m.files)
			}
			return nil
		}
		preserve, err := c.renderGlobs(pick(func(f *v2Files) []*string { return f.preserve }))
		if err != nil {
			return nil, err
		}
		remove, err := c.renderGlobs(pick(func(f *v2Files) []*string { return f.remove }))
		if err != nil {
			return nil, err
		}

		variants = append(variants, variant(platform, deps, assets, preserve, remove, scripts))
	}

	// lip v0.37.0 added an empty variant for its own runtime when a platform
	// is a pattern (e.g. "win-*" from a missing goarch), so that label ""
	// has a variant for every runtime; lip v0.38 inserts an empty variant
	// without a platform, which, merged for the runtime being read, does the
	// same without depending on the machine lip runs on.
	for _, v := range variants {
		if pl, _ := v.get("platform"); strings.Contains(pl.str, "*") {
			variants = append([]*jvalue{emptyVariant("")}, variants...)
			break
		}
	}

	return newObj().
		set("format_version", jnum(3)).
		set("format_uuid", jstr(formatUUID)).
		set("tooth", jstr(*m.tooth)).
		set("version", jstr(*m.version)).
		set("info", newObj().
			set("name", jstrp(m.info.name)).
			set("description", jstrp(m.info.description)).
			set("tags", tags).
			set("avatar_url", jstr(avatar)).v).
		set("variants", jarr(variants...)).v, nil
}

// platformOf is PlatformOf: the runtime identifier pattern of a platform.
func platformOf(p *v2Platform) (string, error) {
	var goos string
	if p.goos != nil {
		goos = *p.goos
	}
	prefix, ok := goosPrefix[goos]
	if p.goos == nil || !ok {
		return "", fmt.Errorf("migrate: unsupported GOOS: %s", goos)
	}
	suffix := "*"
	if p.goarch != nil {
		if suffix, ok = goarchSuffix[*p.goarch]; !ok {
			return "", fmt.Errorf("migrate: unsupported GOARCH: %s", *p.goarch)
		}
	}
	return prefix + "-" + suffix, nil
}
