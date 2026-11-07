package manifest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/lipcore/stjson"
)

// This file reproduces how System.Text.Json deserializes lip's records:
// property names are case-sensitive, unknown properties are ignored, the
// last duplicate property wins, C# "required" properties must be present,
// and init-time validation (regexes, dest safety, path checks) applies.
// Where lip would keep a JSON null in a list/string property, defaults are
// used instead (null lists -> empty, null optional strings -> "").

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// TrimBOM strips a leading UTF-8 byte order mark, which System.Text.Json
// skips when deserializing from a stream.
func TrimBOM(data []byte) []byte { return bytes.TrimPrefix(data, utf8BOM) }

// Parse decodes a format 3 manifest, fills defaults (empty strings/lists,
// as lip's records do), normalizes Version, dependency ids/ranges and asset
// URLs, and validates it (Validate).
func Parse(data []byte) (*Manifest, error) {
	m, err := decodeManifest(TrimBOM(data))
	if err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

// JSONObject is a decoded JSON object with case-sensitive property lookup.
type JSONObject map[string]json.RawMessage

func isNull(raw json.RawMessage) bool { return string(bytes.TrimSpace(raw)) == "null" }

// DecodeObject decodes raw as a JSON object (null is an error).
func DecodeObject(raw json.RawMessage, what string) (JSONObject, error) {
	if isNull(raw) {
		return nil, fmt.Errorf("%s: expected object, got null", what)
	}
	var o JSONObject
	if err := json.Unmarshal(raw, &o); err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}
	return o, nil
}

func missing(what, key string) error {
	return fmt.Errorf("%s: missing required property '%s'", what, key)
}

// String reads a string property. Absent or null yields "" unless
// required, in which case both are errors.
func (o JSONObject) String(key, what string, required bool) (string, error) {
	raw, ok := o[key]
	if !ok {
		if required {
			return "", missing(what, key)
		}
		return "", nil
	}
	if isNull(raw) {
		if required {
			return "", fmt.Errorf("%s.%s: expected string, got null", what, key)
		}
		return "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", fmt.Errorf("%s.%s: %w", what, key, err)
	}
	return s, nil
}

// Int reads an Int32 property. ok is false when the property is absent.
func (o JSONObject) Int(key, what string) (n int, ok bool, err error) {
	raw, ok := o[key]
	if !ok {
		return 0, false, nil
	}
	var v int32
	if isNull(raw) {
		return 0, true, fmt.Errorf("%s.%s: expected number, got null", what, key)
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, true, fmt.Errorf("%s.%s: %w", what, key, err)
	}
	return int(v), true, nil
}

// Bool reads a required boolean property.
func (o JSONObject) Bool(key, what string) (bool, error) {
	raw, ok := o[key]
	if !ok {
		return false, missing(what, key)
	}
	if isNull(raw) {
		return false, fmt.Errorf("%s.%s: expected boolean, got null", what, key)
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err != nil {
		return false, fmt.Errorf("%s.%s: %w", what, key, err)
	}
	return b, nil
}

// StringList reads an array of strings. Absent or null yields an empty
// (non-nil) list; null elements are errors. dropEmpty skips "" entries
// (lip's GlobListJsonConverter/UrlListJsonConverter do).
func (o JSONObject) StringList(key, what string, required, dropEmpty bool) ([]string, error) {
	raw, ok := o[key]
	if !ok && required {
		return nil, missing(what, key)
	}
	out := []string{}
	if !ok || isNull(raw) {
		return out, nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("%s.%s: %w", what, key, err)
	}
	for i, item := range items {
		if isNull(item) {
			return nil, fmt.Errorf("%s.%s[%d]: expected string, got null", what, key, i)
		}
		var s string
		if err := json.Unmarshal(item, &s); err != nil {
			return nil, fmt.Errorf("%s.%s[%d]: %w", what, key, i, err)
		}
		if dropEmpty && s == "" {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

// List reads an array of raw elements; absent or null yields nil.
func (o JSONObject) List(key, what string) ([]json.RawMessage, error) {
	raw, ok := o[key]
	if !ok || isNull(raw) {
		return nil, nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("%s.%s: %w", what, key, err)
	}
	return items, nil
}

func decodeManifest(data []byte) (*Manifest, error) {
	const what = "manifest"
	o, err := DecodeObject(data, what)
	if err != nil {
		return nil, err
	}
	m := &Manifest{}

	fv, ok, err := o.Int("format_version", what)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, missing(what, "format_version")
	}
	if fv != FormatVersion {
		return nil, fmt.Errorf("unsupported format version: %d", fv)
	}
	m.FormatVersion = fv

	if m.FormatUUID, err = o.String("format_uuid", what, true); err != nil {
		return nil, err
	}
	if m.FormatUUID != FormatUUID {
		return nil, fmt.Errorf("unsupported format UUID: %s", m.FormatUUID)
	}

	if m.Tooth, err = o.String("tooth", what, true); err != nil {
		return nil, err
	}
	if err := checkPath(m.Tooth); err != nil {
		return nil, fmt.Errorf("invalid package path: %s: %w", m.Tooth, err)
	}

	version, err := o.String("version", what, true)
	if err != nil {
		return nil, err
	}
	if m.Version, err = NormalizeVersion(version); err != nil {
		return nil, fmt.Errorf("invalid package version: %s: %w", version, err)
	}

	m.Info = Info{Tags: []string{}}
	if raw, ok := o["info"]; ok && !isNull(raw) {
		if m.Info, err = decodeInfo(raw); err != nil {
			return nil, err
		}
	}

	m.Variants = []Variant{}
	variants, err := o.List("variants", what)
	if err != nil {
		return nil, err
	}
	for i, raw := range variants {
		v, err := decodeVariant(raw, fmt.Sprintf("variants[%d]", i))
		if err != nil {
			return nil, err
		}
		m.Variants = append(m.Variants, v)
	}
	return m, nil
}

func decodeInfo(raw json.RawMessage) (Info, error) {
	const what = "info"
	var info Info
	o, err := DecodeObject(raw, what)
	if err != nil {
		return info, err
	}
	if info.Name, err = o.String("name", what, false); err != nil {
		return info, err
	}
	if info.Description, err = o.String("description", what, false); err != nil {
		return info, err
	}
	if info.Tags, err = o.StringList("tags", what, false, false); err != nil {
		return info, err
	}
	for _, tag := range info.Tags {
		if !IsValidTag(tag) {
			return info, fmt.Errorf("invalid package tag: %s", tag)
		}
	}
	if info.AvatarURL, err = o.String("avatar_url", what, false); err != nil {
		return info, err
	}
	return info, nil
}

func decodeVariant(raw json.RawMessage, what string) (Variant, error) {
	v := Variant{}
	o, err := DecodeObject(raw, what)
	if err != nil {
		return v, err
	}
	if v.Label, err = o.String("label", what, false); err != nil {
		return v, err
	}
	if !IsValidVariant(v.Label) {
		return v, fmt.Errorf("invalid label: %s", v.Label)
	}
	if v.Platform, err = o.String("platform", what, false); err != nil {
		return v, err
	}

	v.Dependencies = Dependencies{}
	if raw, ok := o["dependencies"]; ok && !isNull(raw) {
		if err := v.Dependencies.UnmarshalJSON(raw); err != nil {
			return v, fmt.Errorf("%s.dependencies: %w", what, err)
		}
	}

	v.Assets = []Asset{}
	assets, err := o.List("assets", what)
	if err != nil {
		return v, err
	}
	for i, raw := range assets {
		a, err := decodeAsset(raw, fmt.Sprintf("%s.assets[%d]", what, i))
		if err != nil {
			return v, err
		}
		v.Assets = append(v.Assets, a)
	}

	if v.PreserveFiles, err = o.StringList("preserve_files", what, false, true); err != nil {
		return v, err
	}
	if v.RemoveFiles, err = o.StringList("remove_files", what, false, true); err != nil {
		return v, err
	}
	for _, list := range [][]string{v.PreserveFiles, v.RemoveFiles} {
		for i, pattern := range list {
			formatted, err := NormalizeGlob(pattern)
			if err != nil {
				return v, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
			}
			list[i] = formatted
		}
	}

	v.Scripts = emptyScripts()
	if raw, ok := o["scripts"]; ok && !isNull(raw) {
		if v.Scripts, err = decodeScripts(raw, what+".scripts"); err != nil {
			return v, err
		}
	}
	return v, nil
}

func emptyScripts() Scripts {
	return Scripts{
		PreInstall:    []string{},
		Install:       []string{},
		PostInstall:   []string{},
		PreUninstall:  []string{},
		Uninstall:     []string{},
		PostUninstall: []string{},
	}
}

func decodeScripts(raw json.RawMessage, what string) (Scripts, error) {
	s := Scripts{}
	o, err := DecodeObject(raw, what)
	if err != nil {
		return s, err
	}
	for _, f := range []struct {
		key string
		dst *[]string
	}{
		{"pre_install", &s.PreInstall},
		{"install", &s.Install},
		{"post_install", &s.PostInstall},
		{"pre_uninstall", &s.PreUninstall},
		{"uninstall", &s.Uninstall},
		{"post_uninstall", &s.PostUninstall},
	} {
		if *f.dst, err = o.StringList(f.key, what, false, false); err != nil {
			return s, err
		}
	}
	return s, nil
}

func decodeAsset(raw json.RawMessage, what string) (Asset, error) {
	a := Asset{}
	o, err := DecodeObject(raw, what)
	if err != nil {
		return a, err
	}
	typ, ok := o["type"]
	if !ok {
		return a, missing(what, "type")
	}
	name, err := decodeEnum(typ, assetTypeNames())
	if err != nil {
		return a, fmt.Errorf("%s.type: %w", what, err)
	}
	a.Type = AssetType(name)

	if a.URLs, err = o.StringList("urls", what, false, true); err != nil {
		return a, err
	}
	for i, u := range a.URLs {
		a.URLs[i] = NormalizeURL(u)
	}

	a.Placements = []Placement{}
	placements, err := o.List("placements", what)
	if err != nil {
		return a, err
	}
	for i, raw := range placements {
		p, err := decodePlacement(raw, fmt.Sprintf("%s.placements[%d]", what, i))
		if err != nil {
			return a, err
		}
		a.Placements = append(a.Placements, p)
	}
	return a, nil
}

func decodePlacement(raw json.RawMessage, what string) (Placement, error) {
	p := Placement{}
	o, err := DecodeObject(raw, what)
	if err != nil {
		return p, err
	}
	typ, ok := o["type"]
	if !ok {
		return p, missing(what, "type")
	}
	name, err := decodeEnum(typ, placementTypeNames())
	if err != nil {
		return p, fmt.Errorf("%s.type: %w", what, err)
	}
	p.Type = PlacementType(name)
	if p.Src, err = o.String("src", what, true); err != nil {
		return p, err
	}
	if p.Dest, err = o.String("dest", what, true); err != nil {
		return p, err
	}
	if !IsValidDest(p.Dest) {
		return p, fmt.Errorf("invalid destination path: %s", p.Dest)
	}
	return p, nil
}

func assetTypeNames() []string {
	names := make([]string, len(assetTypes))
	for i, t := range assetTypes {
		names[i] = string(t)
	}
	return names
}

func placementTypeNames() []string {
	names := make([]string, len(placementTypes))
	for i, t := range placementTypes {
		names[i] = string(t)
	}
	return names
}

// decodeEnum mimics JsonStringEnumConverter (with JsonStringEnumMemberName)
// for non-flags enums: member names match case-sensitively after trimming
// surrounding spaces, and integer numbers or numeric strings select a
// member by its declaration index. Values without a named member (which
// lip would keep as bare numbers) are rejected.
func decodeEnum(raw json.RawMessage, names []string) (string, error) {
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil && !isNull(raw) {
		if n >= 0 && n < int64(len(names)) {
			return names[n], nil
		}
		return "", fmt.Errorf("unsupported enum value %d", n)
	}
	var s string
	if isNull(raw) || json.Unmarshal(raw, &s) != nil {
		return "", fmt.Errorf("invalid enum value %s", strings.TrimSpace(string(raw)))
	}
	t := strings.TrimSpace(s)
	for _, name := range names {
		if t == name {
			return name, nil
		}
	}
	if n, err := strconv.ParseInt(t, 10, 32); err == nil && n >= 0 && n < int64(len(names)) {
		return names[n], nil
	}
	return "", fmt.Errorf("invalid enum value %q", s)
}

// MarshalJSON encodes d as a JSON object in insertion order.
func (d Dependencies) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, dep := range d {
		if i > 0 {
			b.WriteByte(',')
		}
		k, err := json.Marshal(dep.ID)
		if err != nil {
			return nil, err
		}
		v, err := json.Marshal(dep.Range)
		if err != nil {
			return nil, err
		}
		b.Write(k)
		b.WriteByte(':')
		b.Write(v)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// UnmarshalJSON decodes a JSON object of "<package id>": "<range>" like
// lip's PackageIdToSemVersionRangeDictionary: entries keep document order,
// ids are parsed and stored in PackageId.ToString form, ranges are
// normalized with NormalizeRange, and duplicate ids are an error.
func (d *Dependencies) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return errors.New("dependencies: expected object")
	}
	out := Dependencies{}
	seen := map[PackageID]bool{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		key := tok.(string)
		tok, err = dec.Token()
		if err != nil {
			return err
		}
		value, ok := tok.(string)
		if !ok {
			return fmt.Errorf("dependencies: value of %q must be a string", key)
		}
		id, err := ParsePackageID(key)
		if err != nil {
			return err
		}
		if seen[id] {
			return fmt.Errorf("dependencies: an item with the same key has already been added. Key: %s", id)
		}
		seen[id] = true
		r, err := NormalizeRange(value)
		if err != nil {
			return fmt.Errorf("dependencies: invalid version range for %s: %s: %w", key, value, err)
		}
		out = append(out, Dependency{ID: id.String(), Range: r})
	}
	if _, err := dec.Token(); err != nil {
		return err
	}
	*d = out
	return nil
}

// MarshalSTJSON writes d as an ordered object.
func (d Dependencies) MarshalSTJSON(w *stjson.Writer) error {
	w.BeginObject()
	for _, dep := range d {
		w.Key(dep.ID)
		w.String(dep.Range)
	}
	w.EndObject()
	return nil
}
