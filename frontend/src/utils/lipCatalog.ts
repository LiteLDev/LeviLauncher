// The mod catalogue of levimc.org/mods as the launcher reads, searches and
// filters it: a port of the site's pure logic (the shared contract's A1-A5:
// mods/src/data/{featured-tags,licenses,sides}.ts, mods/src/lib/{search,
// catalog-query}.ts and preferredVariant of mods/src/lib/manifest.ts), so the
// launcher finds and counts exactly what the site does. The launcher forces
// the side ("client") and the platform (its RID), so it shows no Side or
// Platform filter; everything else matches the site.
//
// This module has no imports and no side effects: scripts/lipCatalog.test.mjs
// runs it under node --test against the shared fixture in
// internal/lipindex/testdata (the same bytes the Go lipindex tests read).

// ------------------------------------------------------------------ JSON

/** Where an entry of a package file runs: the dedicated server or the client. */
export type LipSide = "server" | "client";

/** The sides, server first, as the JSON lists them. */
export const LIP_SIDES: readonly LipSide[] = ["server", "client"];

/** The side the launcher installs for. */
export const LAUNCHER_SIDE: LipSide = "client";

/** The platform (.NET RID) the launcher installs for (the Go engine's CurrentPlatform on Windows x64). */
export const LAUNCHER_PLATFORM = "win-x64";

/** A mod of `/mods/index.json` (`mods[]`). */
export interface LipCatalogEntry {
  /** `github.com/<owner>/<repo>`. */
  id: string;
  name: string;
  description: string;
  /** The repository owner: the author, linked to `https://github.com/<owner>`. */
  owner: string;
  /** The mod's logo, else the owner's avatar. */
  logo: string;
  /** Raw manifest tags. */
  tags: string[];
  side: LipSide[];
  /** RIDs any entry of the latest version runs on. */
  platform: string[];
  /** Minecraft versions any version runs on, newest first. */
  gameVersions: string[];
  /** What the latest version depends on: bare id → range. */
  deps: Record<string, string>;
  stars: number;
  /** SPDX id, as GitHub reports it. */
  license?: string;
  updated: string;
  created?: string;
  url: string;
  json: string;
  requiredBy: number;
  /** Position in the index (its default order: "relevance" without a search). */
  order: number;
}

/** An entry of a variant of a version. */
export interface LipEntry {
  side: LipSide;
  platform: string;
  /** Dependency key (`<id>[#<variant>]`) → version range. */
  deps: Record<string, string>;
}

/** A version of a mod's detail (`versions[]`, newest first). */
export interface LipVersion {
  version: string;
  prerelease: boolean;
  released?: string;
  releaseUrl: string;
  manifestUrl: string;
  info: { name: string; description: string; tags: string[]; logo?: string };
  /** Variant name (`""`: the default) → its entries. */
  variants: Record<string, LipEntry[]>;
}

/** A mod another one refers to. */
export interface LipModRef {
  id: string;
  name: string;
  owner: string;
  url?: string;
}

/** `/mods/<id>/index.json`. */
export interface LipModDetail {
  url: string;
  id: string;
  canonical?: string;
  name: string;
  description: string;
  owner: string;
  logo: string;
  repository: string;
  issues?: string;
  releases: string;
  stars: number;
  updated: string;
  published?: string;
  created?: string;
  license?: string;
  side: LipSide[];
  platform: string[];
  gameVersions: string[];
  tags: string[];
  latest: { variant: string; version: string; install: string };
  readmes: { file: string; lang: string | null; text: string }[];
  versions: LipVersion[];
  /** By lower-case id. */
  dependencies: Record<string, LipModRef>;
  requiredBy: LipModRef[];
}

type JsonRecord = Record<string, unknown>;

const isRecord = (value: unknown): value is JsonRecord =>
  Boolean(value) && typeof value === "object" && !Array.isArray(value);

const str = (value: unknown): string => (typeof value === "string" ? value.trim() : "");

const optStr = (value: unknown): string | undefined => str(value) || undefined;

const strings = (value: unknown): string[] =>
  Array.isArray(value) ? value.map(str).filter(Boolean) : [];

const num = (value: unknown): number => {
  const n = typeof value === "number" ? value : Number(value);
  return Number.isFinite(n) ? n : 0;
};

const isSide = (value: string): value is LipSide => value === "server" || value === "client";

const sides = (value: unknown): LipSide[] => {
  const list = strings(value).map((s) => s.toLowerCase());
  return LIP_SIDES.filter((s) => list.includes(s));
};

const rangeMap = (value: unknown): Record<string, string> => {
  const out: Record<string, string> = {};
  if (!isRecord(value)) return out;
  for (const [key, range] of Object.entries(value)) {
    const k = key.trim();
    const r = str(range);
    if (k && r) out[k] = r;
  }
  return out;
};

/** The mods of `/mods/index.json`, in its order; an entry without an id is dropped. */
export function parseLipCatalog(json: unknown): LipCatalogEntry[] {
  if (!isRecord(json) || !Array.isArray(json.mods)) return [];
  const out: LipCatalogEntry[] = [];
  for (const mod of json.mods) {
    if (!isRecord(mod)) continue;
    const id = str(mod.id);
    if (!id) continue;
    out.push({
      id,
      name: str(mod.name) || id.split("/").pop() || id,
      description: str(mod.description),
      owner: str(mod.owner) || ownerOf(id),
      logo: str(mod.logo),
      tags: strings(mod.tags),
      side: sides(mod.side),
      platform: strings(mod.platform),
      gameVersions: strings(mod.gameVersions),
      deps: rangeMap(mod.deps),
      stars: num(mod.stars),
      license: optStr(mod.license),
      updated: str(mod.updated),
      created: optStr(mod.created),
      url: str(mod.url),
      json: str(mod.json),
      requiredBy: num(mod.requiredBy),
      order: out.length,
    });
  }
  return out;
}

const parseEntry = (value: unknown): LipEntry | null => {
  if (!isRecord(value)) return null;
  const side = str(value.side).toLowerCase();
  const platform = str(value.platform);
  if (!isSide(side) || !platform) return null;
  return { side, platform, deps: rangeMap(value.deps) };
};

const parseVariants = (value: unknown): Record<string, LipEntry[]> => {
  const out: Record<string, LipEntry[]> = {};
  if (!isRecord(value)) return out;
  for (const [name, entries] of Object.entries(value)) {
    if (!Array.isArray(entries)) continue;
    out[name.trim()] = entries.map(parseEntry).filter((e): e is LipEntry => e !== null);
  }
  return out;
};

const parseRef = (value: unknown): LipModRef | null => {
  if (!isRecord(value)) return null;
  const id = str(value.id);
  if (!id) return null;
  return { id, name: str(value.name) || id, owner: str(value.owner), url: optStr(value.url) };
};

/**
 * A mod's detail (`/mods/<id>/index.json`), or null when it is not one (no
 * id, or the shape of the site before lip v0.40: `tooth` instead of `id`).
 */
export function parseLipModDetail(json: unknown): LipModDetail | null {
  if (!isRecord(json)) return null;
  const id = str(json.id);
  if (!id || !Array.isArray(json.versions)) return null;
  const versions: LipVersion[] = [];
  for (const v of json.versions) {
    if (!isRecord(v)) continue;
    const version = str(v.version);
    if (!version) continue;
    const info = isRecord(v.info) ? v.info : {};
    versions.push({
      version,
      prerelease: v.prerelease === true,
      released: optStr(v.released),
      releaseUrl: str(v.releaseUrl),
      manifestUrl: str(v.manifestUrl),
      info: { name: str(info.name), description: str(info.description), tags: strings(info.tags), logo: optStr(info.logo) },
      variants: parseVariants(v.variants),
    });
  }
  const latest = isRecord(json.latest) ? json.latest : {};
  const dependencies: Record<string, LipModRef> = {};
  if (isRecord(json.dependencies)) {
    for (const [key, value] of Object.entries(json.dependencies)) {
      const ref = parseRef(value);
      if (ref) dependencies[key.toLowerCase()] = ref;
    }
  }
  return {
    url: str(json.url),
    id,
    canonical: optStr(json.canonical),
    name: str(json.name) || id.split("/").pop() || id,
    description: str(json.description),
    owner: str(json.owner) || ownerOf(id),
    logo: str(json.logo),
    repository: str(json.repository),
    issues: optStr(json.issues),
    releases: str(json.releases),
    stars: num(json.stars),
    updated: str(json.updated),
    published: optStr(json.published),
    created: optStr(json.created),
    license: optStr(json.license),
    side: sides(json.side),
    platform: strings(json.platform),
    gameVersions: strings(json.gameVersions),
    tags: strings(json.tags),
    latest: { variant: typeof latest.variant === "string" ? latest.variant : "", version: str(latest.version), install: str(latest.install) },
    readmes: Array.isArray(json.readmes)
      ? json.readmes.filter(isRecord).map((r) => ({ file: str(r.file), lang: typeof r.lang === "string" ? r.lang : null, text: typeof r.text === "string" ? r.text : "" }))
      : [],
    versions,
    dependencies,
    requiredBy: Array.isArray(json.requiredBy) ? json.requiredBy.map(parseRef).filter((r): r is LipModRef => r !== null) : [],
  };
}

/** The owner segment of an id (`github.com/<owner>/<repo>`). */
export function ownerOf(id: string): string {
  const parts = id.split("/").filter(Boolean);
  return parts.length >= 3 ? parts[1] : parts.length === 2 ? parts[0] : "";
}

/** The author's GitHub page (decision 2: the author is always the repository owner). */
export const ownerUrl = (owner: string): string => (owner ? `https://github.com/${encodeURIComponent(owner)}` : "");

// ------------------------------------------------------------------ dependencies, variants

export const LEVILAMINA_ID = "github.com/LiteLDev/LeviLamina";

/** A dependency key's bare id (`github.com/o/r#client` → `github.com/o/r`) and variant. */
export function splitDependency(key: string): { id: string; variant: string } {
  const hash = key.indexOf("#");
  return hash < 0 ? { id: key, variant: "" } : { id: key.slice(0, hash), variant: key.slice(hash + 1) };
}

/** The dependency key names LeviLamina (any variant, any case). */
export const isLeviLaminaKey = (key: string): boolean =>
  splitDependency(key.trim()).id.toLowerCase() === LEVILAMINA_ID.toLowerCase();

/** An entry runs on the side and the platform. */
export const entryMatches = (entry: LipEntry, side: LipSide, platform: string): boolean =>
  entry.side === side && entry.platform.toLowerCase() === platform.toLowerCase();

const variantRank = (name: string) => (name === "client" ? 0 : name === "" ? 1 : 2);

/** Variant names in the order rule A2 prefers them: `client`, the default `""`, then the others alphabetically. */
export const compareVariantNames = (a: string, b: string): number =>
  variantRank(a) - variantRank(b) || (a < b ? -1 : a > b ? 1 : 0);

/**
 * The variants with an entry for the side (and the platform, when given),
 * in the order of {@link compareVariantNames}.
 */
export function qualifyingVariants(variants: Record<string, readonly LipEntry[]>, side: LipSide, platform?: string): string[] {
  return Object.keys(variants)
    .filter((name) => variants[name].some((e) => e.side === side && (platform === undefined || e.platform.toLowerCase() === platform.toLowerCase())))
    .sort(compareVariantNames);
}

/**
 * The variant to install for a side and platform (rule A2): among those with
 * an entry for them, `client`, then `""`, then the others alphabetically.
 * `undefined`: none has one.
 */
export function preferredVariant(variants: Record<string, readonly LipEntry[]>, side: LipSide, platform?: string): string | undefined {
  return qualifyingVariants(variants, side, platform)[0];
}

/** The entries of a variant of a version that run on the side and platform. */
export function variantEntries(version: LipVersion, variant: string, side: LipSide, platform: string): LipEntry[] {
  return (version.variants[variant] ?? []).filter((e) => entryMatches(e, side, platform));
}

/** `<id>#<variant>`: what installs a variant (`<id>#` for the default variant). */
export const variantIdentifier = (id: string, variant: string): string => `${splitDependency(id).id}#${variant}`;

/** One version of a variant, for the side and platform: its dependencies, and the LeviLamina ranges among them. */
export interface LipVariantFile {
  version: string;
  prerelease: boolean;
  released?: string;
  /** Dependency key → range, over the matching entries. */
  deps: Record<string, string>;
  /** The LeviLamina ranges of {@link deps}. */
  llRanges: string[];
  /** The other dependencies. */
  otherDeps: Record<string, string>;
}

/** The versions (newest first, as the detail lists them) of a variant that has an entry for the side and platform. */
export function variantFiles(detail: Pick<LipModDetail, "versions">, variant: string, side: LipSide, platform: string): LipVariantFile[] {
  const out: LipVariantFile[] = [];
  for (const v of detail.versions) {
    const entries = variantEntries(v, variant, side, platform);
    if (!entries.length) continue;
    const deps: Record<string, string> = {};
    const otherDeps: Record<string, string> = {};
    const llRanges: string[] = [];
    for (const e of entries) {
      for (const [key, range] of Object.entries(e.deps)) {
        deps[key] = range;
        if (isLeviLaminaKey(key)) {
          if (!llRanges.includes(range)) llRanges.push(range);
        } else otherDeps[key] = range;
      }
    }
    out.push({ version: v.version, prerelease: v.prerelease, released: v.released, deps, llRanges, otherDeps });
  }
  return out;
}

/**
 * The `#alias` variants a mod's own dependencies name
 * (`LegacyScriptEngine#client_lua`), lower case and sorted: installed mods
 * whose folder or name is such an alias belong to the package.
 */
export function selfVariantAliases(detail: Pick<LipModDetail, "id" | "versions">): string[] {
  const base = detail.id.toLowerCase();
  const aliases = new Set<string>();
  for (const v of detail.versions) {
    for (const entries of Object.values(v.variants)) {
      for (const e of entries) {
        for (const key of Object.keys(e.deps)) {
          const { id, variant } = splitDependency(key.trim().toLowerCase());
          const alias = variant.trim();
          if (id === base && key.includes("#") && alias) aliases.add(alias);
        }
      }
    }
  }
  return [...aliases].sort((a, b) => (a < b ? -1 : a > b ? 1 : 0));
}

/** The catalogue entries the launcher can install: an entry for its side and platform in the latest version. */
export function launcherEntries(entries: readonly LipCatalogEntry[], side: LipSide = LAUNCHER_SIDE, platform: string = LAUNCHER_PLATFORM): LipCatalogEntry[] {
  const p = platform.toLowerCase();
  return entries.filter((e) => e.side.includes(side) && e.platform.some((x) => x.toLowerCase() === p));
}

/**
 * Whether a detail has a variant the launcher can install: one with an entry
 * for the side and platform in some version. The index's `side` and
 * `platform` are unions over the latest version's entries, so a mod with
 * client entries only for Android and server entries only for Windows passes
 * {@link launcherEntries} for Windows without being installable there; Go
 * (lipindex ParsePackageFor) and LeviLauncher-Android (LipIndex.parsePackage) leave
 * such a mod out, as they do one whose detail did not load.
 */
export function hasLauncherVariant(
  detail: Pick<LipModDetail, "versions"> | null | undefined,
  side: LipSide = LAUNCHER_SIDE,
  platform: string = LAUNCHER_PLATFORM,
): boolean {
  return !!detail && detail.versions.some((v) => qualifyingVariants(v.variants, side, platform).length > 0);
}

// ------------------------------------------------------------------ platforms, game versions

/** The platforms launchers force (and the site lists first, in this order). */
export const FORCED_PLATFORMS: readonly string[] = ["win-x64", "win-arm64", "linux-x64", "linux-arm64", "android-arm64"];

/** The order pages list platforms in: {@link FORCED_PLATFORMS} first, in that order, then the others by name. */
export function comparePlatforms(a: string, b: string): number {
  const rank = (p: string) => {
    const i = FORCED_PLATFORMS.indexOf(p);
    return i < 0 ? FORCED_PLATFORMS.length : i;
  };
  return rank(a) - rank(b) || (a < b ? -1 : a > b ? 1 : 0);
}

function parseVersion(version: string): { nums: number[]; pre: string } | null {
  const m = /^v?(\d+(?:\.\d+)*)(?:-([0-9A-Za-z.-]+))?(?:\+.*)?$/.exec(version.trim());
  if (!m) return null;
  return { nums: m[1].split(".").map(Number), pre: m[2] ?? "" };
}

function comparePre(a: string, b: string): number {
  if (a === b) return 0;
  if (!a) return 1;
  if (!b) return -1;
  const pa = a.split(".");
  const pb = b.split(".");
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    const x = pa[i];
    const y = pb[i];
    if (x === undefined) return -1;
    if (y === undefined) return 1;
    const nx = /^\d+$/.test(x) ? Number(x) : NaN;
    const ny = /^\d+$/.test(y) ? Number(y) : NaN;
    const d = !Number.isNaN(nx) && !Number.isNaN(ny) ? nx - ny : x < y ? -1 : x > y ? 1 : 0;
    if (d) return d;
  }
  return 0;
}

/** Semver-like order of game versions (the site's); unparsable versions sort first. */
export function compareGameVersions(a: string, b: string): number {
  const va = parseVersion(a);
  const vb = parseVersion(b);
  if (!va || !vb) return va ? 1 : vb ? -1 : a.localeCompare(b);
  for (let i = 0; i < Math.max(va.nums.length, vb.nums.length); i++) {
    const d = (va.nums[i] ?? 0) - (vb.nums[i] ?? 0);
    if (d) return d;
  }
  return comparePre(va.pre, vb.pre);
}

/** A game version as the catalogue spells it: the first three numbers (`1.21.50.07` → `1.21.50`). */
export function catalogGameVersion(version: string): string {
  const m = /^v?(\d+)\.(\d+)\.(\d+)/.exec(version.trim());
  return m ? `${Number(m[1])}.${Number(m[2])}.${Number(m[3])}` : version.trim();
}

// ------------------------------------------------------------------ tags (A4)

/** Labels in the site's languages (search finds featured tags by any of them). */
export interface LipLabels {
  en: string;
  "zh-CN": string;
  "zh-Hant": string;
}

/** A kit sprite of a featured tag (`kit/assets/sprites.mjs` key). */
export type LipTagSprite =
  | "bookQuill"
  | "pickaxe"
  | "apps"
  | "diamond"
  | "compass"
  | "map"
  | "emerald"
  | "arrowRight"
  | "tnt"
  | "potion"
  | "bug"
  | "torch"
  | "globe";

export interface FeaturedTag {
  /** The tag itself, and the filter value that selects the group (`lip.tags.<id>` names it in the UI). */
  id: string;
  /** Other (normalised) tags that count as this one. */
  aliases: readonly string[];
  label: LipLabels;
  icon: LipTagSprite;
}

/** In the order filters and chips list them. */
export const FEATURED_TAGS: readonly FeaturedTag[] = [
  {
    id: "library",
    aliases: ["api", "modapi", "nativeapi", "event", "export", "form", "pland:library"],
    label: { en: "Library / API", "zh-CN": "前置库 / API", "zh-Hant": "前置庫 / API" },
    icon: "bookQuill",
  },
  {
    id: "utility",
    aliases: ["tools", "essential", "inventory", "waila", "info-display", "tooltip", "recipe-viewer", "item-list"],
    label: { en: "Utility", "zh-CN": "实用工具", "zh-Hant": "實用工具" },
    icon: "pickaxe",
  },
  {
    id: "management",
    aliases: ["backup", "ban", "whitelist", "login", "command", "land", "checkbag"],
    label: { en: "Server management", "zh-CN": "服务器管理", "zh-Hant": "伺服器管理" },
    icon: "apps",
  },
  {
    id: "game-mechanics",
    aliases: ["gameplay", "vanilla-plus", "achievement"],
    label: { en: "Game mechanics", "zh-CN": "游戏机制", "zh-Hant": "遊戲機制" },
    icon: "diamond",
  },
  {
    id: "adventure",
    aliases: ["worldgen", "terrain", "biome"],
    label: { en: "Content & adventure", "zh-CN": "内容与冒险", "zh-Hant": "內容與冒險" },
    icon: "compass",
  },
  {
    id: "visual",
    aliases: ["hud", "sidebar", "shader", "replay", "recording"],
    label: { en: "Visual & HUD", "zh-CN": "视觉与界面", "zh-Hant": "視覺與介面" },
    icon: "map",
  },
  {
    id: "economy",
    aliases: ["money"],
    label: { en: "Economy", "zh-CN": "经济", "zh-Hant": "經濟" },
    icon: "emerald",
  },
  {
    id: "teleport",
    aliases: ["tp", "waystones"],
    label: { en: "Teleport", "zh-CN": "传送", "zh-Hant": "傳送" },
    icon: "arrowRight",
  },
  {
    id: "anticheat",
    aliases: ["antixray"],
    label: { en: "Anti-cheat", "zh-CN": "反作弊", "zh-Hant": "反作弊" },
    icon: "tnt",
  },
  {
    id: "optimization",
    aliases: ["performance", "cleaner"],
    label: { en: "Optimization", "zh-CN": "性能优化", "zh-Hant": "效能優化" },
    icon: "potion",
  },
  {
    id: "bugfix",
    aliases: ["fix"],
    label: { en: "Bug fixes", "zh-CN": "错误修复", "zh-Hant": "錯誤修復" },
    icon: "bug",
  },
  {
    id: "technology",
    aliases: ["redstone", "piston", "automation"],
    label: { en: "Technical & redstone", "zh-CN": "技术与红石", "zh-Hant": "技術與紅石" },
    icon: "torch",
  },
  {
    id: "protocol",
    aliases: ["multiversion"],
    label: { en: "Protocol & multi-version", "zh-CN": "协议与多版本", "zh-Hant": "協議與多版本" },
    icon: "globe",
  },
];

export const FEATURED_BY_ID: ReadonlyMap<string, FeaturedTag> = new Map(FEATURED_TAGS.map((f) => [f.id, f]));

/** Featured id and alias → the featured tag. */
const GROUP = new Map<string, FeaturedTag>();
for (const f of FEATURED_TAGS) for (const tag of [f.id, ...f.aliases]) GROUP.set(tag, f);

/**
 * Tags nearly every mod carries, or that say what the side and the platform
 * already do (loader, kind, side): never shown or offered. `lse`, `quickjs`
 * and `nodejs` stay, as LSE plugins rarely declare their dependency on LSE.
 */
const HIDDEN = new Set(["levilamina", "ll", "mod", "plugin", "native", "client", "server", "minecraft-bedrock"]);
const HIDDEN_PREFIXES = ["platform:", "type:"];

/** A tag as the listing knows it: lower case, without a `category:` prefix (`category:Utility` → `utility`). */
export function normalizeTag(tag: string): string {
  const lower = tag.trim().toLowerCase();
  return lower.startsWith("category:") ? lower.slice("category:".length) : lower;
}

/** A normalised tag the listing never shows; an empty one too. */
export const isHiddenTag = (tag: string): boolean =>
  !tag || HIDDEN.has(tag) || HIDDEN_PREFIXES.some((prefix) => tag.startsWith(prefix));

/** A mod's tags as the listing shows them: normalised, hidden ones dropped, each once, in the manifest's order. */
export function visibleTags(tags: readonly string[]): string[] {
  return [...new Set(tags.map(normalizeTag))].filter((tag) => !isHiddenTag(tag));
}

/** The featured tag a tag (normalised or not) is or is an alias of. */
export const featuredOf = (tag: string): FeaturedTag | undefined => GROUP.get(normalizeTag(tag));

/**
 * The value a tag filters by: its featured tag's id when it is in a featured
 * group (`money` → `economy`), else the normalised tag; `null` for a hidden one.
 */
export function tagKey(tag: string): string | null {
  const normalized = normalizeTag(tag);
  if (isHiddenTag(normalized)) return null;
  return GROUP.get(normalized)?.id ?? normalized;
}

/** The filter values a mod's tags give, each once, in the manifest's order. */
export function tagKeys(tags: readonly string[]): string[] {
  return [...new Set(tags.map(tagKey).filter((key): key is string => key !== null))];
}

/** The featured tags of a mod, in the table's order, each with the raw tags that put it there (`Teleport [tp]`). */
export function featuredTagsOf(tags: readonly string[]): { tag: FeaturedTag; raw: string[] }[] {
  const raw = new Map<FeaturedTag, string[]>();
  for (const tag of visibleTags(tags)) {
    const f = GROUP.get(tag);
    if (f) raw.set(f, [...(raw.get(f) ?? []), tag]);
  }
  return FEATURED_TAGS.filter((f) => raw.has(f)).map((f) => ({ tag: f, raw: raw.get(f)! }));
}

/** A mod's visible tags that are in no featured group, in the manifest's order. */
export const plainTagsOf = (tags: readonly string[]): string[] => visibleTags(tags).filter((tag) => !GROUP.has(tag));

// ------------------------------------------------------------------ licenses (A5)

export type LicenseFamilyId = "permissive" | "weak-copyleft" | "copyleft" | "other" | "none";

export interface LicenseFamily {
  id: LicenseFamilyId;
  label: LipLabels;
}

/** In the order filters list them (`lip.license.<id>` names them in the UI). */
export const LICENSE_FAMILIES: readonly LicenseFamily[] = [
  { id: "permissive", label: { en: "Permissive", "zh-CN": "宽松", "zh-Hant": "寬鬆" } },
  { id: "weak-copyleft", label: { en: "Weak copyleft", "zh-CN": "弱著佐权", "zh-Hant": "弱著佐權" } },
  { id: "copyleft", label: { en: "Copyleft", "zh-CN": "著佐权", "zh-Hant": "著佐權" } },
  { id: "other", label: { en: "Other", "zh-CN": "其他", "zh-Hant": "其他" } },
  { id: "none", label: { en: "No license", "zh-CN": "无许可证", "zh-Hant": "無授權條款" } },
];

export const isLicenseFamily = (value: string): value is LicenseFamilyId => LICENSE_FAMILIES.some((f) => f.id === value);

/** Licenses GitHub cannot tell (it reports NOASSERTION), by lower-case `owner/repo`. */
export const LICENSE_OVERRIDES: Readonly<Record<string, string>> = {
  "copper-lamp/bschat": "AGPL-3.0",
  "ieckseyr/hologramlib": "MIT",
};

const FAMILY_IDS: readonly [LicenseFamilyId, RegExp][] = [
  ["permissive", /^(?:MIT|APACHE-2\.0|BSD-.+|ISC|ZLIB|CC0-1\.0|UNLICENSE|0BSD)$/],
  ["weak-copyleft", /^(?:LGPL-.+|MPL-2\.0|EPL-2\.0)$/],
  ["copyleft", /^(?:A?GPL-.+)$/],
];

/** The family of an SPDX id (any case); `none` without one. */
export function licenseFamily(spdx: string | undefined): LicenseFamilyId {
  if (!spdx) return "none";
  const id = spdx.toUpperCase();
  return FAMILY_IDS.find(([, re]) => re.test(id))?.[0] ?? "other";
}

const repoKey = (id: string) => id.toLowerCase().split("/").slice(-2).join("/");

/** A mod's license as the listing shows it: the override for its repository, else the SPDX id GitHub reports. */
export function licenseOf(mod: { id: string; license?: string | undefined }): string | undefined {
  return LICENSE_OVERRIDES[repoKey(mod.id)] ?? (mod.license || undefined);
}

/**
 * License filter values as the site's `license=` reads them: a family in any
 * case becomes its id; an SPDX id takes the spelling some mod has, or goes
 * when none has it (kept as typed while the ids are not known yet).
 * Duplicates in any case go.
 */
export function licenseFilterValues(values: readonly string[], spdxIds?: readonly string[]): string[] {
  const out: string[] = [];
  for (const v of values) {
    const lower = v.toLowerCase();
    const value = isLicenseFamily(lower) ? lower : spdxIds ? spdxIds.find((id) => id.toLowerCase() === lower) : v;
    if (value !== undefined && !out.some((x) => x.toLowerCase() === value.toLowerCase())) out.push(value);
  }
  return out;
}

// ------------------------------------------------------------------ search (A3)

/**
 * The text a mod is found by: name ×3; repository path and owner ×2; its
 * featured tags' labels in every language and its visible tags ×2;
 * description ×1. Lower case.
 */
export function searchFields(entry: Pick<LipCatalogEntry, "id" | "name" | "owner" | "tags" | "description">): [string, number][] {
  const labels = featuredTagsOf(entry.tags).flatMap(({ tag }) => Object.values(tag.label));
  const repo = entry.id.split("/").slice(1).join(" ");
  const fields: [string, number][] = [
    [entry.name, 3],
    [`${repo} ${entry.owner}`, 2],
    [[...new Set([...labels, ...visibleTags(entry.tags)])].join(" "), 2],
    [entry.description, 1],
  ];
  return fields.map(([text, weight]) => [text.toLowerCase(), weight]);
}

const WORD = /[\p{L}\p{N}]/u;

/** How well `token` matches `text`: 3 a whole word, 2 the start of a word, 1 anywhere, 0 not at all. */
function matchQuality(text: string, token: string): number {
  let best = 0;
  for (let at = text.indexOf(token); at >= 0; at = text.indexOf(token, at + 1)) {
    const start = at === 0 || !WORD.test(text[at - 1]);
    const end = at + token.length >= text.length || !WORD.test(text[at + token.length]);
    best = Math.max(best, start ? (end ? 3 : 2) : 1);
    if (best === 3) break;
  }
  return best;
}

interface SearchDoc {
  id: string;
  fields: [string, number][];
}

let searchCache: { entries: readonly LipCatalogEntry[]; docs: SearchDoc[] } | null = null;

function docsFor(entries: readonly LipCatalogEntry[]): SearchDoc[] {
  if (searchCache?.entries !== entries) searchCache = { entries, docs: entries.map((e) => ({ id: e.id, fields: searchFields(e) })) };
  return searchCache.docs;
}

/**
 * Relevance scores of the mods matching `query` (higher is better), or
 * `null` for an empty query. Every word of the query has to match; each adds
 * the weight of the fields it matches times how well it matches, and an
 * exact name scores highest.
 */
export function searchCatalog(entries: readonly LipCatalogEntry[], query: string): Map<string, number> | null {
  const q = query.trim().toLowerCase();
  if (!q) return null;
  const tokens = [...new Set(q.split(/[\s,，、]+/u).filter(Boolean))];
  const scores = new Map<string, number>();
  for (const doc of docsFor(entries)) {
    let total = 0;
    let all = true;
    for (const token of tokens) {
      let score = 0;
      for (const [text, weight] of doc.fields) score += weight * matchQuality(text, token);
      if (!score) {
        all = false;
        break;
      }
      total += score;
    }
    if (!all) continue;
    if (doc.fields[0][0] === q) total += 100;
    scores.set(doc.id, total);
  }
  return scores;
}

// ------------------------------------------------------------------ filters and sorting (A3)

/** The sorts, exactly the site's: relevance is the search score with a search, else the index order. */
export const LIP_SORTS = ["relevance", "stars", "updated", "newest"] as const;
export type LipSortKey = (typeof LIP_SORTS)[number];

export const isLipSortKey = (value: string): value is LipSortKey => (LIP_SORTS as readonly string[]).includes(value);

/** Values a filter group includes, and values it excludes. */
export interface LipFacet {
  inc: string[];
  exc: string[];
}

export const EMPTY_FACET: LipFacet = { inc: [], exc: [] };

export interface LipQueryState {
  q: string;
  /** Game versions: any of them (OR). */
  gv: string[];
  /** Sides: any of them (OR); the launcher forces its side instead. */
  side: LipSide[];
  /** Platforms (RIDs): any of them (OR); the launcher forces its platform instead. */
  platform: string[];
  /** Tag keys: every included one (AND), none of the excluded. */
  tags: LipFacet;
  /** License families and SPDX ids: any of them (OR). */
  license: string[];
  /** Bare ids the latest version depends on, any case: every one (AND). */
  dep: string[];
  /** Owners, any case (OR). */
  author: string[];
  sort: LipSortKey;
}

export const DEFAULT_QUERY_STATE: LipQueryState = {
  q: "",
  gv: [],
  side: [],
  platform: [],
  tags: EMPTY_FACET,
  license: [],
  dep: [],
  author: [],
  sort: "relevance",
};

/**
 * Tag filter values as the site reads them: `-tag` excludes; a tag both
 * included and excluded is excluded; hidden tags are dropped and aliases
 * become their featured tag.
 */
export function tagFacet(values: readonly string[]): LipFacet {
  const inc: string[] = [];
  const exc: string[] = [];
  for (const raw of values) {
    const excluded = raw.startsWith("-");
    const key = tagKey(excluded ? raw.slice(1) : raw);
    if (!key) continue;
    const into = excluded ? exc : inc;
    if (!into.includes(key)) into.push(key);
  }
  return { inc: inc.filter((key) => !exc.includes(key)), exc };
}

const toggleIn = (list: readonly string[], id: string) => (list.includes(id) ? list.filter((x) => x !== id) : [...list, id]);

/** Includes a tag, or takes it off when it is included (the site's filter groups). An excluded tag becomes included. */
export const includeTag = (f: LipFacet, id: string): LipFacet => ({ inc: toggleIn(f.inc, id), exc: f.exc.filter((x) => x !== id) });

/** Excludes a tag, or takes it off when it is excluded. An included tag becomes excluded. */
export const excludeTag = (f: LipFacet, id: string): LipFacet => ({ inc: f.inc.filter((x) => x !== id), exc: toggleIn(f.exc, id) });

/** Toggles a value of an OR/AND list, compared in any case. */
export function toggleValue(values: readonly string[], value: string): string[] {
  const l = value.toLowerCase();
  return values.some((v) => v.toLowerCase() === l) ? values.filter((v) => v.toLowerCase() !== l) : [...values, value];
}

/** How many filters are on: search and sort do not count. */
export function filterCount(state: LipQueryState): number {
  return (
    state.gv.length +
    state.side.length +
    state.platform.length +
    state.tags.inc.length +
    state.tags.exc.length +
    state.license.length +
    state.dep.length +
    state.author.length
  );
}

/** The state without any filter: search and sort stay. */
export function clearFilters(state: LipQueryState): LipQueryState {
  return { ...DEFAULT_QUERY_STATE, q: state.q, sort: state.sort };
}

interface Facts {
  tags: Set<string>;
  license: string | undefined;
  family: LicenseFamilyId;
  deps: Set<string>;
  platform: Set<string>;
  owner: string;
}

const factsCache = new WeakMap<LipCatalogEntry, Facts>();

function factsOf(e: LipCatalogEntry): Facts {
  let facts = factsCache.get(e);
  if (!facts) {
    const license = licenseOf(e);
    facts = {
      tags: new Set(tagKeys(e.tags)),
      license: license?.toLowerCase(),
      family: licenseFamily(license),
      deps: new Set(Object.keys(e.deps).map((d) => d.toLowerCase())),
      platform: new Set(e.platform.map((p) => p.toLowerCase())),
      owner: e.owner.toLowerCase(),
    };
    factsCache.set(e, facts);
  }
  return facts;
}

type Filters = Pick<LipQueryState, "gv" | "side" | "platform" | "tags" | "license" | "dep" | "author">;

/** Whether a mod passes the filters (the search is applied apart, see {@link searchCatalog}). */
export function matchesFilters(e: LipCatalogEntry, f: Filters): boolean {
  const facts = factsOf(e);
  return (
    (!f.gv.length || f.gv.some((v) => e.gameVersions.includes(v))) &&
    (!f.side.length || f.side.some((s) => e.side.includes(s))) &&
    (!f.platform.length || f.platform.some((p) => facts.platform.has(p.toLowerCase()))) &&
    f.tags.inc.every((key) => facts.tags.has(key)) &&
    !f.tags.exc.some((key) => facts.tags.has(key)) &&
    (!f.license.length || f.license.some((v) => v === facts.family || v.toLowerCase() === facts.license)) &&
    f.dep.every((d) => facts.deps.has(d.toLowerCase())) &&
    (!f.author.length || f.author.some((a) => a.toLowerCase() === facts.owner))
  );
}

const time = (iso: string | undefined) => {
  const t = iso ? Date.parse(iso) : NaN;
  return Number.isNaN(t) ? 0 : t;
};

/**
 * The order of the results. Relevance is the search score or, without a
 * search, the index's order (the site's default order), which also breaks
 * search ties.
 */
export function compareEntries(sort: LipSortKey, scores: Map<string, number> | null) {
  const byStars = (a: LipCatalogEntry, b: LipCatalogEntry) => b.stars - a.stars || time(b.updated) - time(a.updated);
  const byName = (a: LipCatalogEntry, b: LipCatalogEntry) =>
    a.id.toLowerCase() < b.id.toLowerCase() ? -1 : a.id.toLowerCase() > b.id.toLowerCase() ? 1 : 0;
  return (a: LipCatalogEntry, b: LipCatalogEntry): number => {
    let d: number;
    if (sort === "relevance") d = (scores ? (scores.get(b.id) ?? 0) - (scores.get(a.id) ?? 0) : 0) || a.order - b.order;
    else if (sort === "stars") d = byStars(a, b);
    else if (sort === "updated") d = time(b.updated) - time(a.updated) || b.stars - a.stars;
    else d = time(b.created) - time(a.created) || b.stars - a.stars;
    return d || byName(a, b);
  };
}

/** A license family and the SPDX ids under it. */
export interface LicenseOption {
  family: LicenseFamilyId;
  /** By how many mods have them (none for `none`). */
  ids: string[];
}

export interface LipFacetOptions {
  /** Newest first. */
  gv: string[];
  side: LipSide[];
  platform: string[];
  /** Tag keys: the featured tags some mod has, in their table's order, then every other by how many mods carry it. */
  tags: string[];
  license: LicenseOption[];
  /** Bare ids as the index spells them, by how many mods depend on them. */
  dep: string[];
  /** Owners, by how many mods they have. */
  author: string[];
}

const byFrequency = (values: Iterable<string>) => {
  const n = new Map<string, number>();
  for (const v of values) n.set(v, (n.get(v) ?? 0) + 1);
  return [...n.keys()].sort((a, b) => n.get(b)! - n.get(a)! || a.localeCompare(b));
};

const optionsCache = new WeakMap<readonly LipCatalogEntry[], LipFacetOptions>();

/** The values every filter group offers, from the whole catalogue. */
export function facetOptions(entries: readonly LipCatalogEntry[]): LipFacetOptions {
  const cached = optionsCache.get(entries);
  if (cached) return cached;
  const gv = [...new Set(entries.flatMap((e) => e.gameVersions))].sort((a, b) => compareGameVersions(b, a));
  const platform = [...new Set(entries.flatMap((e) => e.platform))].sort(comparePlatforms);
  const tagCounts = byFrequency(entries.flatMap((e) => [...factsOf(e).tags]));
  const featured = FEATURED_TAGS.map((f) => f.id).filter((id) => tagCounts.includes(id));
  const licenses = entries.map((e) => licenseOf(e));
  const license = LICENSE_FAMILIES.map((f) => f.id)
    .filter((family) => licenses.some((l) => licenseFamily(l) === family))
    .map((family) => ({
      family,
      ids: byFrequency(licenses.filter((l): l is string => !!l && licenseFamily(l) === family)),
    }));
  const options: LipFacetOptions = {
    gv,
    side: LIP_SIDES.filter((id) => entries.some((e) => e.side.includes(id))),
    platform,
    tags: [...featured, ...tagCounts.filter((key) => !featured.includes(key))],
    license,
    dep: byFrequency(entries.flatMap((e) => Object.keys(e.deps))),
    author: byFrequency(entries.map((e) => e.owner)),
  };
  optionsCache.set(entries, options);
  return options;
}

export interface LipFacetCounts {
  gv: Record<string, number>;
  side: Record<string, number>;
  platform: Record<string, number>;
  tags: Record<string, number>;
  license: Record<string, number>;
  dep: Record<string, number>;
  author: Record<string, number>;
}

export interface LipQueryResult<T extends LipCatalogEntry = LipCatalogEntry> {
  /** Every mod matching the query, in order. */
  matched: T[];
  /**
   * How many results each filter option would give: an OR group's option
   * (game version, side, platform, license, author) as if it were the
   * group's only value; an AND group's (tags, dependencies) as if added to
   * the group (an excluded tag as if included instead).
   */
  counts: LipFacetCounts;
}

const withExtra = (options: readonly string[], selected: readonly string[]) => [
  ...options,
  ...selected.filter((v) => !options.some((o) => o.toLowerCase() === v.toLowerCase())),
];

/**
 * Applies a state to the catalogue. `scores` are the search's
 * ({@link searchCatalog}), `null` without a query. `extra` is a further
 * filter that every result and count respects (the launcher's LeviLamina
 * version).
 */
export function runQuery<T extends LipCatalogEntry>(
  entries: readonly T[],
  state: LipQueryState,
  scores: Map<string, number> | null,
  extra?: (entry: T) => boolean,
): LipQueryResult<T> {
  const searched = scores ? entries.filter((e) => scores.has(e.id)) : entries;
  const base = extra ? searched.filter(extra) : searched;
  const filters: Filters = {
    gv: state.gv,
    side: state.side,
    platform: state.platform,
    tags: state.tags,
    license: state.license,
    dep: state.dep,
    author: state.author,
  };
  const matched = base.filter((e) => matchesFilters(e, filters)).sort(compareEntries(state.sort, scores));

  const count = (patch: Partial<Filters>) => {
    const f = { ...filters, ...patch };
    return base.reduce((n, e) => n + (matchesFilters(e, f) ? 1 : 0), 0);
  };
  const has = (values: readonly string[], v: string) => values.some((x) => x.toLowerCase() === v.toLowerCase());
  const options = facetOptions(entries);
  const counts: LipFacetCounts = { gv: {}, side: {}, platform: {}, tags: {}, license: {}, dep: {}, author: {} };
  for (const v of withExtra(options.gv, state.gv)) counts.gv[v] = count({ gv: [v] });
  for (const s of LIP_SIDES) counts.side[s] = count({ side: [s] });
  for (const p of withExtra(options.platform, state.platform)) counts.platform[p] = count({ platform: [p] });
  for (const key of withExtra(options.tags, [...state.tags.inc, ...state.tags.exc])) {
    const inc = state.tags.inc.includes(key) ? state.tags.inc : [...state.tags.inc, key];
    counts.tags[key] = count({ tags: { inc, exc: state.tags.exc.filter((x) => x !== key) } });
  }
  const licenseValues = [...LICENSE_FAMILIES.map((f) => f.id), ...options.license.flatMap((o) => o.ids)];
  for (const v of withExtra(licenseValues, state.license)) counts.license[v] = count({ license: [v] });
  for (const d of withExtra(options.dep, state.dep)) {
    counts.dep[d] = count({ dep: has(state.dep, d) ? state.dep : [...state.dep, d] });
  }
  for (const a of withExtra(options.author, state.author)) counts.author[a] = count({ author: [a] });
  return { matched, counts };
}
