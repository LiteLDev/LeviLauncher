package cli

// world get / world set: the world settings stored in level.dat.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/content"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

type worldEnumOption struct{ Value, Label string }

// worldEnumOptions names the values of well-known level.dat settings, as the
// window's world editor does.
var worldEnumOptions = func() map[string][]worldEnumOption {
	broadcast := []worldEnumOption{{"0", "NoMultiPlay"}, {"1", "InviteOnly"}, {"2", "FriendsOnly"}, {"3", "FriendsOfFriends"}, {"4", "Public"}}
	generator := []worldEnumOption{{"0", "Legacy"}, {"1", "Overworld"}, {"2", "Flat"}, {"3", "Nether"}, {"4", "TheEnd"}, {"5", "Void"}}
	return map[string][]worldEnumOption{
		"XBLBroadcastIntent":      broadcast,
		"GamePublishSetting":      broadcast,
		"PlatformBroadcastIntent": broadcast,
		"WorldVersion":            {{"0", "Pre_1_18"}, {"1", "Post_1_18"}},
		"StorageVersion": {{"0", "Unknown"}, {"1", "OldV1"}, {"2", "OldV2"}, {"3", "OldV3"}, {"4", "LevelDB1"},
			{"5", "LevelDBSubChunks"}, {"6", "LevelDBSubChunkRawZip"}, {"7", "LevelDBPaletted1"},
			{"8", "LevelDBPalettedMultiBlockStorage"}, {"9", "LevelDataUpgradedSeed"}, {"10", "LevelDataStrictSize"}},
		"Generator":     generator,
		"GeneratorType": generator,
		"GameType": {{"-1", "Undefined"}, {"0", "Survival/WorldDefault"}, {"1", "Creative"}, {"2", "Adventure"},
			{"5", "Default"}, {"6", "Spectator"}},
		"Difficulty":             {{"0", "Peaceful"}, {"1", "Easy"}, {"2", "Normal"}, {"3", "Hard"}},
		"editorWorldType":        {{"0", "NonEditor"}, {"1", "EditorProject"}, {"2", "EditorTestLevel"}, {"3", "EditorRealmsUpload"}},
		"eduOffer":               {{"0", "None"}, {"1", "RestOfWorld"}, {"2", "China_Deprecated"}},
		"permissionsLevel":       {{"0", "Any"}, {"1", "GameDirectors"}, {"2", "Admin"}, {"3", "Host"}, {"4", "Owner"}, {"5", "Internal"}},
		"playerPermissionsLevel": {{"0", "Visitor"}, {"1", "Member"}, {"2", "Operator"}, {"3", "Custom"}},
		"daylightCycle":          {{"0", "Normal"}, {"1", "AlwaysDay"}, {"2", "LockTime"}},
	}
}()

// worldEnumLabel returns the name of value for setting name, "" if none.
func worldEnumLabel(name, value string) string {
	for _, o := range worldEnumOptions[name] {
		if o.Value == strings.TrimSpace(value) {
			return o.Label
		}
	}
	return ""
}

// worldEnumValue maps a name ("creative", "survival") of setting name to its
// number.
func worldEnumValue(name, raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	for _, o := range worldEnumOptions[name] {
		if strings.EqualFold(o.Label, raw) {
			return o.Value, true
		}
		for _, part := range strings.Split(o.Label, "/") {
			if strings.EqualFold(part, raw) {
				return o.Value, true
			}
		}
	}
	return "", false
}

func worldEnumNames(name string) string {
	var out []string
	for _, o := range worldEnumOptions[name] {
		out = append(out, strings.ToLower(strings.Split(o.Label, "/")[0]))
	}
	return strings.Join(out, ", ")
}

// worldSetting is one level.dat field in --json output.
type worldSetting struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value any    `json:"value"`
	Label string `json:"label,omitempty"`

	display string // human form of the value
	raw     string // bare value for a single-key get
}

type worldSettingsDoc struct {
	World   string         `json:"world"`
	Path    string         `json:"path,omitempty"`
	Version int32          `json:"version"`
	Fields  []worldSetting `json:"fields"`
}

// worldLevelDat is a decoded level.dat.
type worldLevelDat struct {
	dir     string
	root    map[string]any // the Data compound, or the root when there is none
	inData  bool
	version int32
}

func worldLoadLevelDat(w worldEntry) (*worldLevelDat, error) {
	if !utils.FileExists(filepath.Join(w.Path, "level.dat")) {
		return nil, Errorf(ExitNotFound, "ERR_LEVEL_DAT_NOT_FOUND", "world %s has no level.dat", worldLabel(w))
	}
	root, ver, err := content.DecodeLevelDat(w.Path)
	if err != nil {
		return nil, Errorf(ExitFailure, "ERR_LEVEL_DAT_INVALID", "cannot read level.dat of world %s: %v", worldLabel(w), err)
	}
	ld := &worldLevelDat{dir: w.Path, root: root, version: ver}
	if data, ok := root["Data"].(map[string]any); ok {
		ld.root, ld.inData = data, true
	}
	return ld, nil
}

// compound returns the compound at path, nil when there is none.
func (ld *worldLevelDat) compound(path []string) map[string]any {
	cur := ld.root
	for _, seg := range path {
		m, ok := cur[seg].(map[string]any)
		if !ok {
			return nil
		}
		cur = m
	}
	return cur
}

// fields returns the typed fields of the compound at path, in the order the
// game stores them.
func (ld *worldLevelDat) fields(path []string) ([]types.LevelDatField, error) {
	fields, _, err := content.ReadLevelDatFieldsAt(ld.dir, path)
	if err != nil {
		return nil, err
	}
	var order []string
	if len(path) == 0 {
		if o, v, err := content.ReadLevelDatOrder(ld.dir); err == nil && v == ld.version {
			order = o
		}
	} else {
		p := path
		if ld.inData {
			p = append([]string{"Data"}, path...)
		}
		if o, v, err := content.ReadLevelDatOrderAt(ld.dir, p); err == nil && v == ld.version {
			order = o
		}
	}
	worldSortFields(fields, order)
	return fields, nil
}

// worldSortFields orders fields as listed in order, the rest after them
// upper-case names first, as the window does.
func worldSortFields(fields []types.LevelDatField, order []string) {
	rank := map[string]int{}
	for i, k := range order {
		if _, ok := rank[k]; !ok {
			rank[k] = i
		}
	}
	group := func(s string) int {
		if s == "" {
			return 2
		}
		switch c := s[0]; {
		case c >= 'A' && c <= 'Z':
			return 0
		case c >= 'a' && c <= 'z':
			return 1
		}
		return 2
	}
	sort.SliceStable(fields, func(i, j int) bool {
		a, b := fields[i].Name, fields[j].Name
		ra, oka := rank[a]
		rb, okb := rank[b]
		switch {
		case oka && okb:
			return ra < rb
		case oka != okb:
			return oka
		}
		if ga, gb := group(a), group(b); ga != gb {
			return ga < gb
		}
		return a < b
	})
}

// worldFindField finds name among fields: exact, else the only match
// ignoring case.
func worldFindField(fields []types.LevelDatField, name string) (types.LevelDatField, bool) {
	var fold []types.LevelDatField
	for _, f := range fields {
		if f.Name == name {
			return f, true
		}
		if strings.EqualFold(f.Name, name) {
			fold = append(fold, f)
		}
	}
	if len(fold) == 1 {
		return fold[0], true
	}
	return types.LevelDatField{}, false
}

// worldKeyPath splits "abilities/mayfly" into its segments.
func worldKeyPath(key string) ([]string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, UsageError("empty setting name")
	}
	segs := strings.Split(key, "/")
	for i, s := range segs {
		segs[i] = strings.TrimSpace(s)
		if segs[i] == "" {
			return nil, UsageError("invalid setting name %q: empty part between '/'", key)
		}
	}
	return segs, nil
}

func worldKeyNotFound(key string, fields []types.LevelDatField) *Error {
	e := Errorf(ExitNotFound, "ERR_KEY_NOT_FOUND", "no setting %q in level.dat", key)
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.Name
	}
	last := key
	if i := strings.LastIndex(key, "/"); i >= 0 {
		last = key[i+1:]
	}
	if s := suggest(last, names); s != "" {
		return e.WithHint("did you mean %q?", s)
	}
	return e
}

// worldToSetting converts a field for output; compound values come from the
// decoded compound cur.
func worldToSetting(f types.LevelDatField, cur map[string]any, console bool) worldSetting {
	s := worldSetting{Name: f.Name, Type: f.Tag}
	switch f.Tag {
	case "byte", "short", "int", "long":
		n, err := strconv.ParseInt(strings.TrimSpace(f.ValueString), 10, 64)
		if err == nil {
			s.Value = n
		} else {
			s.Value = f.ValueString
		}
		s.raw, s.display = f.ValueString, f.ValueString
		if l := worldEnumLabel(f.Name, f.ValueString); l != "" {
			s.Label = l
			s.display += " (" + l + ")"
		}
	case "float", "double":
		if v, err := strconv.ParseFloat(strings.TrimSpace(f.ValueString), 64); err == nil && !math.IsInf(v, 0) && !math.IsNaN(v) {
			s.Value = v
		} else {
			s.Value = f.ValueString
		}
		s.raw, s.display = f.ValueString, f.ValueString
	case "string":
		s.Value, s.raw, s.display = f.ValueString, f.ValueString, f.ValueString
	case "compound":
		m, _ := cur[f.Name].(map[string]any)
		b, err := json.Marshal(m)
		if err == nil {
			s.Value = json.RawMessage(b)
			s.raw = string(b)
		}
		s.display = fmt.Sprintf("{%d settings}", len(m))
	default: // list and unknown
		v := strings.TrimSpace(f.ValueJSON)
		if v != "" && json.Valid([]byte(v)) {
			s.Value = json.RawMessage(v)
			s.raw = v
		} else {
			s.Value, s.raw = f.ValueString, f.ValueString
		}
		s.display = s.raw
		if console && len([]rune(s.display)) > 60 {
			s.display = string([]rune(s.display)[:57]) + "..."
		}
	}
	return s
}

// --- world get ---

func runWorldGet(c *Ctx) error {
	s, err := worldScopeOf(c)
	if err != nil {
		return err
	}
	w, err := worldFind(worldScan(s.dir), c.Arg(0), s.inst.Name)
	if err != nil {
		return err
	}
	ld, err := worldLoadLevelDat(w)
	if err != nil {
		return err
	}
	var path []string
	if key := strings.TrimSpace(c.Arg(1)); key != "" {
		segs, err := worldKeyPath(key)
		if err != nil {
			return err
		}
		parent := segs[:len(segs)-1]
		fields, err := ld.fields(parent)
		if err != nil {
			return Errorf(ExitNotFound, "ERR_KEY_NOT_FOUND", "no setting %q in level.dat", strings.Join(parent, "/"))
		}
		f, ok := worldFindField(fields, segs[len(segs)-1])
		if !ok {
			return worldKeyNotFound(key, fields)
		}
		if f.Tag != "compound" {
			one := worldToSetting(f, ld.compound(parent), false)
			doc := worldSettingsDoc{World: w.ID, Path: strings.Join(append(append([]string{}, parent...), f.Name), "/"),
				Version: ld.version, Fields: []worldSetting{one}}
			return c.Render(doc, []string{one.raw}, func(out io.Writer) error {
				_, err := fmt.Fprintln(out, one.raw)
				return err
			})
		}
		path = append(append([]string{}, parent...), f.Name)
	}
	fields, err := ld.fields(path)
	if err != nil {
		return Errorf(ExitFailure, "ERR_LEVEL_DAT_INVALID", "cannot read level.dat of world %s: %v", worldLabel(w), err)
	}
	cur := ld.compound(path)
	doc := worldSettingsDoc{World: w.ID, Path: strings.Join(path, "/"), Version: ld.version, Fields: []worldSetting{}}
	for _, f := range fields {
		doc.Fields = append(doc.Fields, worldToSetting(f, cur, c.StdoutIsConsole()))
	}
	ids := make([]string, len(doc.Fields))
	for i, f := range doc.Fields {
		ids[i] = f.Name
	}
	return c.Render(doc, ids, func(out io.Writer) error {
		rows := make([][]string, len(doc.Fields))
		for i, f := range doc.Fields {
			rows[i] = []string{f.Name, f.Type, f.display}
		}
		return Table(out, []string{"KEY", "TYPE", "VALUE"}, rows)
	})
}

// --- world set ---

type worldChange struct {
	Key   string `json:"key"`
	Type  string `json:"type"`
	Old   any    `json:"old"`
	New   any    `json:"new"`
	Added bool   `json:"added,omitempty"`

	oldDisplay, newDisplay string
}

type worldSetResult struct {
	World   string        `json:"world"`
	Changes []worldChange `json:"changes"`
}

// worldAssignment is one <key>=<value> argument.
type worldAssignment struct {
	key    string
	parent []string
	name   string
	value  string
}

func worldParseAssignments(args []string) ([]worldAssignment, error) {
	var out []worldAssignment
	for _, a := range args {
		i := strings.Index(a, "=")
		if i <= 0 {
			return nil, UsageError("expected <key>=<value>, got %q", a).WithHint("%s world set <world> GameType=creative", ProgramName)
		}
		segs, err := worldKeyPath(a[:i])
		if err != nil {
			return nil, err
		}
		out = append(out, worldAssignment{key: strings.Join(segs, "/"), parent: segs[:len(segs)-1], name: segs[len(segs)-1], value: a[i+1:]})
	}
	return out, nil
}

// worldParseValue turns the text of an assignment into a field of type tag.
func worldParseValue(name, tag, raw string) (types.LevelDatField, error) {
	f := types.LevelDatField{Name: name, Tag: tag}
	bad := func(format string, a ...any) error {
		return UsageError("invalid value %q for %s (%s): %s", raw, name, tag, fmt.Sprintf(format, a...)).withCode("ERR_INVALID_VALUE")
	}
	v := strings.TrimSpace(raw)
	integer := func(lo, hi int64) error {
		if ev, ok := worldEnumValue(name, v); ok {
			v = ev
		}
		switch strings.ToLower(v) {
		case "true", "on", "yes":
			v = "1"
		case "false", "off", "no":
			v = "0"
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			if names := worldEnumNames(name); names != "" {
				return bad("expected a number or one of %s", names)
			}
			return bad("expected a whole number")
		}
		if n < lo || n > hi {
			return bad("out of range %d..%d", lo, hi)
		}
		f.ValueString = strconv.FormatInt(n, 10)
		return nil
	}
	switch tag {
	case "byte":
		return f, integer(math.MinInt8, math.MaxUint8)
	case "short":
		return f, integer(math.MinInt16, math.MaxInt16)
	case "int":
		return f, integer(math.MinInt32, math.MaxInt32)
	case "long":
		return f, integer(math.MinInt64, math.MaxInt64)
	case "float", "double":
		bits := 64
		if tag == "float" {
			bits = 32
		}
		x, err := strconv.ParseFloat(v, bits)
		if err != nil || math.IsInf(x, 0) || math.IsNaN(x) {
			return f, bad("expected a number")
		}
		f.ValueString = strconv.FormatFloat(x, 'g', -1, bits)
		return f, nil
	case "string":
		f.ValueString = raw
		return f, nil
	case "list", "compound":
		if v == "" {
			v = map[string]string{"list": "[]", "compound": "{}"}[tag]
		}
		var x any
		if err := json.Unmarshal([]byte(v), &x); err != nil {
			return f, bad("expected JSON")
		}
		if _, ok := x.([]any); tag == "list" && !ok {
			return f, bad("expected a JSON array")
		}
		if _, ok := x.(map[string]any); tag == "compound" && !ok {
			return f, bad("expected a JSON object")
		}
		var buf bytes.Buffer
		if err := json.Compact(&buf, []byte(v)); err != nil {
			return f, bad("expected JSON")
		}
		f.ValueJSON = buf.String()
		return f, nil
	}
	return f, UsageError("setting %s has a type (%s) that cannot be changed", name, tag).withCode("ERR_UNSUPPORTED_TYPE")
}

// worldSameValue reports whether a parsed value equals the stored one.
func worldSameValue(old, nw types.LevelDatField) bool {
	switch nw.Tag {
	case "float", "double":
		a, e1 := strconv.ParseFloat(strings.TrimSpace(old.ValueString), 64)
		b, e2 := strconv.ParseFloat(strings.TrimSpace(nw.ValueString), 64)
		if nw.Tag == "float" {
			return e1 == nil && e2 == nil && float32(a) == float32(b)
		}
		return e1 == nil && e2 == nil && a == b
	case "list", "compound":
		var a, b any
		if json.Unmarshal([]byte(old.ValueJSON), &a) != nil || json.Unmarshal([]byte(nw.ValueJSON), &b) != nil {
			return false
		}
		ja, _ := json.Marshal(a)
		jb, _ := json.Marshal(b)
		return bytes.Equal(ja, jb)
	}
	return strings.TrimSpace(old.ValueString) == strings.TrimSpace(nw.ValueString)
}

// worldPlanSet works out the changes of the assignments against ld. typ is
// --type, for new settings. Unchanged values are dropped.
func worldPlanSet(ld *worldLevelDat, as []worldAssignment, typ string) (map[string][]types.LevelDatField, []string, []worldChange, error) {
	byParent := map[string][]types.LevelDatField{}
	var parents []string
	var changes []worldChange
	cache := map[string][]types.LevelDatField{}
	for _, a := range as {
		pk := strings.Join(a.parent, "/")
		fields, ok := cache[pk]
		if !ok {
			if len(a.parent) > 0 && ld.compound(a.parent) == nil {
				return nil, nil, nil, Errorf(ExitNotFound, "ERR_KEY_NOT_FOUND", "no compound setting %q in level.dat", pk)
			}
			var err error
			fields, err = ld.fields(a.parent)
			if err != nil {
				return nil, nil, nil, Errorf(ExitNotFound, "ERR_KEY_NOT_FOUND", "no compound setting %q in level.dat", pk)
			}
			cache[pk] = fields
		}
		old, exists := worldFindField(fields, a.name)
		tag := old.Tag
		name := a.name
		if exists {
			name = old.Name
			if typ != "" && typ != old.Tag {
				return nil, nil, nil, UsageError("%s is a %s setting; --type only applies to new settings", a.key, old.Tag).withCode("ERR_INVALID_VALUE")
			}
			if old.Tag == "compound" {
				if b, err := json.Marshal(ld.compound(append(append([]string{}, a.parent...), old.Name))); err == nil {
					old.ValueJSON = string(b)
				}
			}
		} else {
			if typ == "" {
				e := UsageError("there is no setting %q in level.dat; to add it, pass --type <type>", a.key).withCode("ERR_TYPE_REQUIRED")
				if s := suggest(a.name, worldFieldNames(fields)); s != "" {
					return nil, nil, nil, e.WithHint("did you mean %q?", s)
				}
				return nil, nil, nil, e.WithHint("%s world set <world> %s=<value> --type int", ProgramName, a.key)
			}
			tag = typ
		}
		nf, err := worldParseValue(name, tag, a.value)
		if err != nil {
			return nil, nil, nil, err
		}
		key := strings.Join(append(append([]string{}, a.parent...), name), "/")
		ch := worldChange{Key: key, Type: tag, Added: !exists}
		cur := ld.compound(a.parent)
		newSetting := worldToSetting(nf, nil, false)
		if nf.Tag == "compound" {
			newSetting.Value, newSetting.display = json.RawMessage(nf.ValueJSON), nf.ValueJSON
		}
		ch.New, ch.newDisplay = newSetting.Value, newSetting.display
		if exists {
			if worldSameValue(old, nf) {
				continue
			}
			prev := worldToSetting(old, cur, false)
			if old.Tag == "compound" {
				prev.display = old.ValueJSON
			}
			ch.Old, ch.oldDisplay = prev.Value, prev.display
		}
		// A later assignment of the same key wins.
		list := byParent[pk]
		for i := range list {
			if list[i].Name == nf.Name {
				list = append(list[:i], list[i+1:]...)
				break
			}
		}
		for i := range changes {
			if changes[i].Key == key {
				changes = append(changes[:i], changes[i+1:]...)
				break
			}
		}
		if _, seen := byParent[pk]; !seen {
			parents = append(parents, pk)
		}
		byParent[pk] = append(list, nf)
		changes = append(changes, ch)
	}
	// Top-level changes are saved first, then each compound, as the window
	// does.
	sort.SliceStable(parents, func(i, j int) bool { return parents[i] == "" && parents[j] != "" })
	return byParent, parents, changes, nil
}

func worldFieldNames(fields []types.LevelDatField) []string {
	out := make([]string, len(fields))
	for i, f := range fields {
		out[i] = f.Name
	}
	return out
}

func runWorldSet(c *Ctx) error {
	as, err := worldParseAssignments(c.Args[1:])
	if err != nil {
		return err
	}
	s, err := worldScopeOf(c)
	if err != nil {
		return err
	}
	w, err := worldFind(worldScan(s.dir), c.Arg(0), s.inst.Name)
	if err != nil {
		return err
	}
	ld, err := worldLoadLevelDat(w)
	if err != nil {
		return err
	}
	byParent, parents, changes, err := worldPlanSet(ld, as, c.String("type"))
	if err != nil {
		return err
	}
	res := worldSetResult{World: w.ID, Changes: []worldChange{}}
	if len(changes) == 0 {
		c.Notice("Nothing to change: world %s already has these values.", worldLabel(w))
		return c.Render(res, nil, nil)
	}
	if err := contentEnsureStopped(s.inst, "world settings"); err != nil {
		return err
	}
	for _, pk := range parents {
		var path []string
		if pk != "" {
			path = strings.Split(pk, "/")
		}
		if err := content.WriteLevelDatFieldsAt(w.Path, path, byParent[pk], ld.version); err != nil {
			return Errorf(ExitFailure, "ERR_WRITE_FILE", "cannot write level.dat of world %s: %v", worldLabel(w), err)
		}
	}
	res.Changes = changes
	ids := make([]string, len(changes))
	for i, ch := range changes {
		ids[i] = ch.Key
	}
	return c.Render(res, ids, func(out io.Writer) error {
		for _, ch := range changes {
			if ch.Added {
				fmt.Fprintf(out, "%s: added (%s) %s\n", ch.Key, ch.Type, ch.newDisplay)
				continue
			}
			fmt.Fprintf(out, "%s: %s -> %s\n", ch.Key, ch.oldDisplay, ch.newDisplay)
		}
		return nil
	})
}
