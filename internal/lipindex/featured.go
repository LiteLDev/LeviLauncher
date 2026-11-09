package lipindex

import "strings"

// Tags as the catalogue shows and filters them (the shared contract's A4,
// ported from levimc.org mods/src/data/featured-tags.ts and mirrored by
// frontend/src/utils/lipCatalog.ts). The catalogue carries a mod's tags as
// its manifest has them; only listings normalise them, hide the loader and
// kind tags, and group a few common ones under the featured tags.

// FeaturedTag is a tag the catalogue lists first, with the other tags that
// count as it.
type FeaturedTag struct {
	// ID is the tag itself and the filter value that selects the group.
	ID string
	// Aliases are other (normalised) tags that count as this one.
	Aliases []string
	// Labels are its names in the site's languages (en, zh-CN, zh-Hant);
	// the search finds the tag by any of them.
	Labels [3]string
}

// FeaturedTags are the featured tags in the order filters list them.
var FeaturedTags = []FeaturedTag{
	{"library", []string{"api", "modapi", "nativeapi", "event", "export", "form", "pland:library"}, [3]string{"Library / API", "前置库 / API", "前置庫 / API"}},
	{"utility", []string{"tools", "essential", "inventory", "waila", "info-display", "tooltip", "recipe-viewer", "item-list"}, [3]string{"Utility", "实用工具", "實用工具"}},
	{"management", []string{"backup", "ban", "whitelist", "login", "command", "land", "checkbag"}, [3]string{"Server management", "服务器管理", "伺服器管理"}},
	{"game-mechanics", []string{"gameplay", "vanilla-plus", "achievement"}, [3]string{"Game mechanics", "游戏机制", "遊戲機制"}},
	{"adventure", []string{"worldgen", "terrain", "biome"}, [3]string{"Content & adventure", "内容与冒险", "內容與冒險"}},
	{"visual", []string{"hud", "sidebar", "shader", "replay", "recording"}, [3]string{"Visual & HUD", "视觉与界面", "視覺與介面"}},
	{"economy", []string{"money"}, [3]string{"Economy", "经济", "經濟"}},
	{"teleport", []string{"tp", "waystones"}, [3]string{"Teleport", "传送", "傳送"}},
	{"anticheat", []string{"antixray"}, [3]string{"Anti-cheat", "反作弊", "反作弊"}},
	{"optimization", []string{"performance", "cleaner"}, [3]string{"Optimization", "性能优化", "效能優化"}},
	{"bugfix", []string{"fix"}, [3]string{"Bug fixes", "错误修复", "錯誤修復"}},
	{"technology", []string{"redstone", "piston", "automation"}, [3]string{"Technical & redstone", "技术与红石", "技術與紅石"}},
	{"protocol", []string{"multiversion"}, [3]string{"Protocol & multi-version", "协议与多版本", "協議與多版本"}},
}

// featuredGroup maps a featured id or alias to its featured tag.
var featuredGroup = func() map[string]*FeaturedTag {
	m := map[string]*FeaturedTag{}
	for i := range FeaturedTags {
		f := &FeaturedTags[i]
		m[f.ID] = f
		for _, a := range f.Aliases {
			m[a] = f
		}
	}
	return m
}()

// hiddenTags are tags nearly every mod carries, or that say what the side
// and the platform already do: never shown or offered. lse, quickjs and
// nodejs stay, as LSE plugins rarely declare their dependency on LSE.
var hiddenTags = map[string]bool{"levilamina": true, "ll": true, "mod": true, "plugin": true, "native": true,
	"client": true, "server": true, "minecraft-bedrock": true}

var hiddenTagPrefixes = []string{"platform:", "type:"}

// NormalizeTag is a tag as the listing knows it: lower case, without a
// "category:" prefix ("category:Utility" -> "utility").
func NormalizeTag(tag string) string {
	l := strings.ToLower(strings.TrimSpace(tag))
	return strings.TrimPrefix(l, "category:")
}

// IsHiddenTag reports whether the listing never shows a normalised tag (an
// empty one included).
func IsHiddenTag(tag string) bool {
	if tag == "" || hiddenTags[tag] {
		return true
	}
	for _, p := range hiddenTagPrefixes {
		if strings.HasPrefix(tag, p) {
			return true
		}
	}
	return false
}

// VisibleTags are a mod's tags as the listing shows them: normalised,
// hidden ones dropped, each once, in the manifest's order.
func VisibleTags(tags []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, t := range tags {
		n := NormalizeTag(t)
		if seen[n] {
			continue
		}
		seen[n] = true
		if !IsHiddenTag(n) {
			out = append(out, n)
		}
	}
	return out
}

// FeaturedOf is the featured tag a tag (normalised or not) is or is an
// alias of.
func FeaturedOf(tag string) (*FeaturedTag, bool) {
	f, ok := featuredGroup[NormalizeTag(tag)]
	return f, ok
}

// TagKey is the value a tag filters by: its featured tag's id when it is in
// a featured group ("money" -> "economy"), else the normalised tag; "" for
// a hidden one.
func TagKey(tag string) string {
	n := NormalizeTag(tag)
	if IsHiddenTag(n) {
		return ""
	}
	if f, ok := featuredGroup[n]; ok {
		return f.ID
	}
	return n
}

// TagKeys are the filter values a mod's tags give, each once, in the
// manifest's order.
func TagKeys(tags []string) []string {
	out := []string{}
	for _, t := range tags {
		if k := TagKey(t); k != "" && !contains(out, k) {
			out = append(out, k)
		}
	}
	return out
}

// FeaturedTagOf is a featured tag of a mod with the raw tags that put it
// there (shown in grey beside its name: "Teleport tp").
type FeaturedTagOf struct {
	Tag *FeaturedTag
	Raw []string
}

// FeaturedTagsOf are the featured tags of a mod, in the table's order.
func FeaturedTagsOf(tags []string) []FeaturedTagOf {
	raw := map[*FeaturedTag][]string{}
	for _, t := range VisibleTags(tags) {
		if f, ok := featuredGroup[t]; ok {
			raw[f] = append(raw[f], t)
		}
	}
	out := []FeaturedTagOf{}
	for i := range FeaturedTags {
		f := &FeaturedTags[i]
		if r, ok := raw[f]; ok {
			out = append(out, FeaturedTagOf{Tag: f, Raw: r})
		}
	}
	return out
}

// PlainTagsOf are a mod's visible tags in no featured group, in the
// manifest's order.
func PlainTagsOf(tags []string) []string {
	out := []string{}
	for _, t := range VisibleTags(tags) {
		if _, ok := featuredGroup[t]; !ok {
			out = append(out, t)
		}
	}
	return out
}

// IsFeaturedTag reports whether id is the id of a featured tag.
func IsFeaturedTag(id string) bool {
	f, ok := featuredGroup[id]
	return ok && f.ID == id
}
