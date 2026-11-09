import { listDirectories } from "./fs";
import { compareVersions } from "./version";
import { GetUserGamertagMap } from "bindings/github.com/liteldev/LeviLauncher/internal/app/userservice";
import {
  compareVariantNames,
  hasLauncherVariant,
  LAUNCHER_PLATFORM,
  LAUNCHER_SIDE,
  launcherEntries,
  parseLipCatalog,
  parseLipModDetail,
  preferredVariant,
  qualifyingVariants,
  selfVariantAliases,
  splitDependency,
  variantFiles,
  variantIdentifier,
  type LipCatalogEntry,
  type LipModDetail,
  type LipVariantFile,
} from "./lipCatalog";

export const EMPTY_CONTENT_ROOTS = {
  base: "",
  packageType: "",
  comMojangRoot: "",
  worlds: "",
  skinPacks: "",
  screenshots: "",
  usersRoot: "",
  resourcePacks: "",
  behaviorPacks: "",
  isIsolation: false,
  isPreview: false,
};

type ContentPathRoots = {
  packageType?: string;
  usersRoot?: string;
  comMojangRoot?: string;
  worlds?: string;
  skinPacks?: string;
  screenshots?: string;
};

export function isContentTransferTarget(
  source: { packageType?: string; isIsolation?: boolean; isPreview?: boolean },
  target: { packageType?: string; enableIsolation?: boolean; type?: string },
): boolean {
  if (target.enableIsolation) return true;
  if (target.packageType !== "uwp") return false;
  // Release and Beta share LocalState only when both UWP instances are not isolated.
  return source.packageType !== "uwp" || !!source.isIsolation ||
    (String(target.type || "").trim().toLowerCase() === "preview") !== !!source.isPreview;
}

export function resolveContentPath(
  roots: ContentPathRoots,
  kind: "minecraftWorlds" | "skin_packs" | "Screenshots" | "minecraftpe",
  player = "",
): string {
  if (roots.packageType === "uwp") {
    const direct = kind === "minecraftWorlds" ? roots.worlds
      : kind === "skin_packs" ? roots.skinPacks
      : kind === "Screenshots" ? roots.screenshots : "";
    return direct || (roots.comMojangRoot ? `${roots.comMojangRoot}\\${kind}` : "");
  }
  if (!roots.usersRoot || !player || player === "." || player === ".." || /[\\/:]/.test(player)) return "";
  return `${roots.usersRoot}\\${player}\\games\\com.mojang\\${kind}`;
}

export async function listPlayers(usersRoot: string, throwOnError = false): Promise<string[]> {
  if (!usersRoot) return [];
  const entries = await listDirectories(usersRoot, throwOnError);
  return entries
    .map((e) => e.name)
    .filter((n) => n && n !== "9556213259376595538");
}

let cachedUsersRoot = "";
let cachedMap: Record<string, string> = {};
let cachedPromise: Promise<Record<string, string>> | null = null;

export async function getPlayerGamertagMap(
  usersRoot: string,
): Promise<Record<string, string>> {
  const root = String(usersRoot || "");
  if (!root) return {};
  if (root === cachedUsersRoot && cachedPromise) return cachedPromise;

  cachedUsersRoot = root;
  cachedMap = {};
  cachedPromise = (async () => {
    try {
      const fn = GetUserGamertagMap as any;
      if (typeof fn !== "function") return {};
      const res = await fn(root);
      if (res && typeof res === "object") {
        cachedMap = res as Record<string, string>;
        return cachedMap;
      }
    } catch {}
    return {};
  })();

  return cachedPromise;
}

export function resolvePlayerDisplayName(
  playerFolder: string,
  map: Record<string, string> | undefined,
): string {
  const key = String(playerFolder || "");
  const gt = map ? map[key] : undefined;
  return typeof gt === "string" && gt.trim() ? gt : key;
}

// ------------------------------------------------------------------ lip catalogue
//
// The catalogue is levimc.org/mods, read from its JSON at
// levimc.org/api/v4/mods/ (lip v0.40's keys; /mods/ itself keeps the JSON from
// before version 4 for older launchers): its index (`index.json`) lists every mod
// with what the filters need, and each mod's detail (`<id>/index.json`) its
// versions, each with variants of entries {side, platform, deps}. The
// launcher installs for the client on its platform (lipCatalog.ts
// LAUNCHER_SIDE / LAUNCHER_PLATFORM): it lists the mods whose latest version
// has such an entry, and offers the variants that have one (rule A2 picks
// the default). The pure logic lives in lipCatalog.ts.

const LIP_MODS_BASE_URL = "https://levimc.org/api/v4/mods/";
const LIP_INDEX_URL = `${LIP_MODS_BASE_URL}index.json`;
const LIP_DETAIL_FETCH_CONCURRENCY = 8;

/** A variant the launcher can install: `packageIdentifier` is `<id>#<variant>` (`<id>#` for the default `""`). */
export interface LIPPackageVariantOption {
  key: string;
  packageIdentifier: string;
}

/** One version of a variant, for the launcher's side and platform. */
export interface LIPPackageFileInfo {
  version: string;
  prerelease: boolean;
  released?: string;
  dependencies: Record<string, string>;
  llDependencyRanges: string[];
  otherDependencies: Record<string, string>;
}

export interface LIPPackageVariantDetail extends LIPPackageVariantOption {
  /** Newest first. */
  files: LIPPackageFileInfo[];
  versions: string[];
  llDependencyRanges: string[];
}

/**
 * A catalogue mod as the launcher lists it: the index's entry, and from its
 * detail the versions and LeviLamina ranges of the preferred variant.
 */
export interface LIPPackageBasicInfo extends LipCatalogEntry {
  /** The catalogue id (`github.com/<owner>/<repo>`), the same as `id`. */
  identifier: string;
  /** The repository owner (`owner`). */
  author: string;
  avatarUrl: string;
  projectUrl: string;
  /** The preferred variant's versions, newest first (empty until the detail is loaded). */
  versions: string[];
  /** LeviLamina ranges any version of the installable variants depends on. */
  llDependencyRanges: string[];
  /** The variants the launcher can install, preferred first. */
  variants: LIPPackageVariantOption[];
  /** Rule A2's choice; "" is the default variant too, so check `variants` for whether there is one. */
  preferredVariantKey: string;
}

export interface LIPPackageDetail extends LIPPackageBasicInfo {
  files: LIPPackageFileInfo[];
  variantDetails: LIPPackageVariantDetail[];
  /** The detail as the catalogue has it (license, side, platform, readmes, dependency names...). */
  detail: LipModDetail | null;
}

export interface LIPSelfVariantRelation {
  identifier: string;
  packageName: string;
  aliases: string[];
}

type ParsedLIPIndexResult = {
  packages: LIPPackageBasicInfo[];
  detailsByIdentifier: Record<string, LIPPackageDetail>;
  selfVariantRelations: LIPSelfVariantRelation[];
};

type ParsedSemver = {
  major: number;
  minor: number;
  patch: number;
  pre: string[];
};

export const normalizeSemverInput = (value: string): string =>
  String(value || "")
    .trim()
    .replace(/^v/i, "");

const parseSemver = (input: string): ParsedSemver | null => {
  const normalized = normalizeSemverInput(input);
  if (!normalized) return null;
  const match = normalized.match(
    /^(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$/,
  );
  if (!match) return null;
  return {
    major: Number(match[1]),
    minor: Number(match[2]),
    patch: Number(match[3]),
    pre: match[4] ? match[4].split(".") : [],
  };
};

const compareSemver = (a: ParsedSemver, b: ParsedSemver): number => {
  if (a.major !== b.major) return a.major > b.major ? 1 : -1;
  if (a.minor !== b.minor) return a.minor > b.minor ? 1 : -1;
  if (a.patch !== b.patch) return a.patch > b.patch ? 1 : -1;

  const aRelease = a.pre.length === 0;
  const bRelease = b.pre.length === 0;
  if (aRelease && bRelease) return 0;
  if (aRelease) return 1;
  if (bRelease) return -1;

  const length = Math.max(a.pre.length, b.pre.length);
  for (let i = 0; i < length; i += 1) {
    const aPart = a.pre[i];
    const bPart = b.pre[i];
    if (aPart === undefined) return -1;
    if (bPart === undefined) return 1;

    const aNumeric = /^[0-9]+$/.test(aPart);
    const bNumeric = /^[0-9]+$/.test(bPart);
    if (aNumeric && bNumeric) {
      const aNum = Number(aPart);
      const bNum = Number(bPart);
      if (aNum !== bNum) return aNum > bNum ? 1 : -1;
      continue;
    }
    if (aNumeric && !bNumeric) return -1;
    if (!aNumeric && bNumeric) return 1;
    if (aPart !== bPart) return aPart > bPart ? 1 : -1;
  }
  return 0;
};

const compareSemverCore = (a: ParsedSemver, b: ParsedSemver): number => {
  if (a.major !== b.major) return a.major > b.major ? 1 : -1;
  if (a.minor !== b.minor) return a.minor > b.minor ? 1 : -1;
  if (a.patch !== b.patch) return a.patch > b.patch ? 1 : -1;
  return 0;
};

export const compareSemverLike = (a: string, b: string): number => {
  const parsedA = parseSemver(a);
  const parsedB = parseSemver(b);
  if (parsedA && parsedB) return compareSemver(parsedA, parsedB);
  if (parsedA) return 1;
  if (parsedB) return -1;
  return normalizeSemverInput(a).localeCompare(normalizeSemverInput(b));
};

const isWildcardSegment = (value: string): boolean => {
  const normalized = value.trim().toLowerCase();
  return normalized === "*" || normalized === "x";
};

const parseWildcardRange = (
  token: string,
): { min: ParsedSemver; max: ParsedSemver } | "all" | null => {
  const normalized = normalizeSemverInput(token);
  if (!normalized) return null;
  if (isWildcardSegment(normalized)) return "all";

  const parts = normalized.split(".").map((part) => part.trim());
  if (parts.length === 0 || parts.length > 3) return null;

  const wildcardIndex = parts.findIndex((part) => isWildcardSegment(part));
  if (wildcardIndex < 0) return null;

  for (let i = 0; i < wildcardIndex; i += 1) {
    if (!/^\d+$/.test(parts[i])) return null;
  }
  for (let i = wildcardIndex; i < parts.length; i += 1) {
    if (!isWildcardSegment(parts[i])) return null;
  }

  if (wildcardIndex === 0) {
    return "all";
  }

  const major = Number(parts[0] || 0);
  const minor = wildcardIndex > 1 ? Number(parts[1] || 0) : 0;
  const min: ParsedSemver = { major, minor, patch: 0, pre: [] };

  if (wildcardIndex === 1) {
    return {
      min,
      max: { major: major + 1, minor: 0, patch: 0, pre: [] },
    };
  }

  if (wildcardIndex === 2) {
    return {
      min,
      max: { major, minor: minor + 1, patch: 0, pre: [] },
    };
  }

  return null;
};

const isRangeTokenMatched = (
  target: ParsedSemver,
  rawToken: string,
): boolean => {
  const token = String(rawToken || "").trim();
  if (!token) return false;

  const comparatorMatch = token.match(/^(>=|<=|>|<|=)\s*(.+)$/);
  if (comparatorMatch) {
    const comparator = comparatorMatch[1];
    const cmpTarget = parseSemver(comparatorMatch[2]);
    if (!cmpTarget) return false;
    const cmp = compareSemver(target, cmpTarget);
    if (comparator === ">=") return cmp >= 0;
    if (comparator === "<=") return cmp <= 0;
    if (comparator === ">") return cmp > 0;
    if (comparator === "<") return cmp < 0;
    return cmp === 0;
  }

  const wildcardRange = parseWildcardRange(token);
  if (wildcardRange === "all") return true;
  if (wildcardRange) {
    return (
      compareSemverCore(target, wildcardRange.min) >= 0 &&
      compareSemverCore(target, wildcardRange.max) < 0
    );
  }

  const exact = parseSemver(token);
  if (!exact) return false;
  return compareSemver(target, exact) === 0;
};

export const isLeviLaminaVersionInRange = (
  version: string,
  rangeExpression: string,
): boolean => {
  const target = parseSemver(version);
  if (!target) return false;

  const normalizedRange = String(rangeExpression || "").trim();
  if (!normalizedRange) return false;

  const tokens = normalizedRange.split(/\s+/).filter(Boolean);
  if (tokens.length === 0) return false;
  return tokens.every((token) => isRangeTokenMatched(target, token));
};

export const isLeviLaminaVersionCompatible = (
  version: string,
  ranges: string[],
): boolean => {
  if (!Array.isArray(ranges) || ranges.length === 0) return false;
  const normalizedVersion = String(version || "").trim();
  if (!normalizedVersion) return false;
  return ranges.some((range) =>
    isLeviLaminaVersionInRange(normalizedVersion, range),
  );
};

export const isLeviLaminaRangesCompatibleWithAnyVersion = (
  ranges: string[],
  llVersions: string[],
): boolean => {
  if (!Array.isArray(ranges) || ranges.length === 0) return false;
  if (!Array.isArray(llVersions) || llVersions.length === 0) return false;
  return llVersions.some((llVersion) =>
    isLeviLaminaVersionCompatible(llVersion, ranges),
  );
};

export const resolveSupportedGameVersionsByLLRanges = (
  ranges: string[],
  gameToLLVersions: Record<string, string[]>,
): string[] => {
  if (!Array.isArray(ranges) || ranges.length === 0) return [];
  if (!gameToLLVersions || typeof gameToLLVersions !== "object") return [];

  const matchedGameVersions: string[] = [];
  for (const [gameVersion, llVersions] of Object.entries(gameToLLVersions)) {
    const normalizedGameVersion = String(gameVersion || "").trim();
    if (!normalizedGameVersion) continue;
    if (
      !isLeviLaminaRangesCompatibleWithAnyVersion(
        ranges,
        Array.isArray(llVersions) ? llVersions : [],
      )
    ) {
      continue;
    }
    matchedGameVersions.push(normalizedGameVersion);
  }

  const unique = Array.from(new Set(matchedGameVersions));
  unique.sort((a, b) => compareVersions(b, a));
  return unique;
};

const normalizeHttpUrl = (value: string): string => {
  const raw = String(value || "").trim();
  if (!raw) return "";
  try {
    const parsed = new URL(raw);
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:") return "";
    return parsed.toString();
  } catch {
    return "";
  }
};

/** `https://github.com/<owner>/<repo>` for a catalogue id (`https://<id>` for other hosts). */
const projectUrlOf = (identifier: string): string => {
  const parts = identifier.trim().replace(/^https?:\/\//, "").split("/").filter(Boolean);
  if (parts.length === 2 && !parts[0].includes(".")) {
    return normalizeHttpUrl(`https://github.com/${parts[0]}/${parts[1]}`);
  }
  return parts.length >= 2 ? normalizeHttpUrl(`https://${parts.join("/")}`) : "";
};

/**
 * The index entry of a mod known only from its detail (a package page opened
 * directly, a dependency outside the launcher's list): `deps` is the union
 * over the latest version's entries keyed by bare id, the mod itself left
 * out, as the index has it.
 */
const entryFromDetail = (detail: LipModDetail): LipCatalogEntry => {
  const ranges = new Map<string, string[]>();
  for (const entries of Object.values(detail.versions[0]?.variants ?? {})) {
    for (const entry of entries) {
      for (const [key, range] of Object.entries(entry.deps)) {
        const { id } = splitDependency(key);
        if (id.toLowerCase() === detail.id.toLowerCase()) continue;
        const list = ranges.get(id) ?? [];
        if (!list.includes(range)) list.push(range);
        ranges.set(id, list);
      }
    }
  }
  return {
    id: detail.id,
    name: detail.name,
    description: detail.description,
    owner: detail.owner,
    logo: detail.logo,
    tags: detail.tags,
    side: detail.side,
    platform: detail.platform,
    gameVersions: detail.gameVersions,
    deps: Object.fromEntries(
      [...ranges]
        .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
        .map(([id, list]) => [id, list.join(" || ")]),
    ),
    stars: detail.stars,
    license: detail.license,
    updated: detail.updated,
    created: detail.created,
    url: detail.url,
    json: "",
    requiredBy: detail.requiredBy.length,
    order: Number.MAX_SAFE_INTEGER,
  };
};

const toFileInfo = (file: LipVariantFile): LIPPackageFileInfo => ({
  version: file.version,
  prerelease: file.prerelease,
  released: file.released,
  dependencies: file.deps,
  llDependencyRanges: file.llRanges,
  otherDependencies: file.otherDeps,
});

/**
 * A mod as the launcher shows it: its index entry and, from its detail, the
 * variants with an entry for the launcher's side and platform, preferred
 * (rule A2) first.
 */
const buildLIPPackageDetail = (
  entry: LipCatalogEntry,
  detail: LipModDetail | null,
): LIPPackageDetail => {
  const names = new Set<string>();
  for (const version of detail?.versions ?? []) {
    for (const name of qualifyingVariants(version.variants, LAUNCHER_SIDE, LAUNCHER_PLATFORM)) {
      names.add(name);
    }
  }
  const variantDetails: LIPPackageVariantDetail[] = detail
    ? [...names].sort(compareVariantNames).map((key) => {
        const files = variantFiles(detail, key, LAUNCHER_SIDE, LAUNCHER_PLATFORM).map(toFileInfo);
        return {
          key,
          packageIdentifier: variantIdentifier(entry.id, key),
          files,
          versions: files.map((file) => file.version),
          llDependencyRanges: [...new Set(files.flatMap((file) => file.llDependencyRanges))],
        };
      })
    : [];
  // Rule A2 over the newest version that has an installable variant.
  let preferredKey: string | undefined;
  for (const version of detail?.versions ?? []) {
    preferredKey = preferredVariant(version.variants, LAUNCHER_SIDE, LAUNCHER_PLATFORM);
    if (preferredKey !== undefined) break;
  }
  const preferred =
    variantDetails.find((variant) => variant.key === preferredKey) ?? variantDetails[0];
  return {
    ...entry,
    identifier: entry.id,
    author: entry.owner,
    avatarUrl: normalizeHttpUrl(entry.logo),
    projectUrl: normalizeHttpUrl(detail?.repository || "") || projectUrlOf(entry.id),
    versions: preferred?.versions ?? [],
    llDependencyRanges: preferred?.llDependencyRanges ?? [],
    variants: variantDetails.map(({ key, packageIdentifier }) => ({ key, packageIdentifier })),
    preferredVariantKey: preferred?.key ?? "",
    files: preferred?.files ?? [],
    variantDetails,
    detail,
  };
};

/** The detail document of a catalogue id (`levimc.org/api/v4/mods/<id>/index.json`). */
const buildLIPModDetailUrl = (identifier: string): string => {
  const path = String(identifier || "")
    .trim()
    .split("#")[0]
    .replace(/^https?:\/\//, "")
    .replace(/^\/+|\/+$/g, "");
  if (!path) return "";
  return `${LIP_MODS_BASE_URL}${path
    .split("/")
    .map((segment) => encodeURIComponent(segment))
    .join("/")}/index.json`;
};

const fetchLIPJson = async (
  url: string,
  signal: AbortSignal,
): Promise<unknown> => {
  const response = await fetch(url, {
    method: "GET",
    cache: "no-cache",
    signal,
  });
  if (!response.ok) {
    throw new Error(`Failed to fetch ${url}: ${response.status}`);
  }
  return response.json();
};

/** Details by id; one that fails or is not a detail of this shape is left out. */
const fetchLIPModDetails = async (
  identifiers: string[],
  signal: AbortSignal,
): Promise<Record<string, LipModDetail>> => {
  const details: Record<string, LipModDetail> = {};
  let nextIndex = 0;
  const worker = async () => {
    while (nextIndex < identifiers.length) {
      const identifier = identifiers[nextIndex++];
      const url = buildLIPModDetailUrl(identifier);
      if (!url) continue;
      try {
        const detail = parseLipModDetail(await fetchLIPJson(url, signal));
        if (detail) details[identifier] = detail;
      } catch (error) {
        if (signal.aborted) throw error;
      }
    }
  };
  await Promise.all(
    Array.from(
      { length: Math.min(LIP_DETAIL_FETCH_CONCURRENCY, identifiers.length) },
      worker,
    ),
  );
  return details;
};

/**
 * The `#alias` variants a mod's entries depend on of itself
 * (LegacyScriptEngine#client_lua): installed mods whose folder or name is
 * such an alias belong to the package.
 */
export const extractSelfVariantAliasesFromPackage = (
  detail: Pick<LipModDetail, "id" | "versions"> | null | undefined,
): string[] => (detail ? selfVariantAliases(detail) : []);

const buildLIPIndexResult = (
  entries: LipCatalogEntry[],
  details: Record<string, LipModDetail>,
): ParsedLIPIndexResult => {
  const packages: LIPPackageBasicInfo[] = [];
  const detailsByIdentifier: Record<string, LIPPackageDetail> = {};
  const relations: LIPSelfVariantRelation[] = [];
  for (const entry of entries) {
    if (detailsByIdentifier[entry.id]) continue;
    const detail = details[entry.id] ?? null;
    const parsed = buildLIPPackageDetail(entry, detail);
    detailsByIdentifier[entry.id] = parsed;
    const { files: _files, variantDetails: _variants, detail: _detail, ...basic } = parsed;
    // Listed only with a variant the launcher can install (as Go and
    // LeviLauncher-Android do): the index's side and platform are unions, so an
    // entry can pass launcherEntries with none, and one whose detail failed
    // cannot be installed either. Its detail stays available for lookups.
    if (hasLauncherVariant(detail)) packages.push(basic);
    const aliases = extractSelfVariantAliasesFromPackage(detail);
    if (aliases.length > 0) {
      relations.push({ identifier: entry.id, packageName: parsed.name || entry.id, aliases });
    }
  }
  relations.sort((a, b) =>
    (a.packageName || a.identifier).localeCompare(b.packageName || b.identifier, undefined, {
      sensitivity: "base",
    }),
  );
  return { packages, detailsByIdentifier, selfVariantRelations: relations };
};

let cachedLIPIndexResult: ParsedLIPIndexResult | null = null;
let cachedLIPIndexResultPromise: Promise<ParsedLIPIndexResult> | null = null;
const LIP_INDEX_FETCH_TIMEOUT_MS = 30_000;

const fetchLIPIndexResult = async (options?: {
  forceRefresh?: boolean;
  signal?: AbortSignal;
}): Promise<ParsedLIPIndexResult> => {
  const forceRefresh = options?.forceRefresh === true;
  const previous = cachedLIPIndexResult;

  if (!forceRefresh && cachedLIPIndexResult) {
    return cachedLIPIndexResult;
  }

  if (!forceRefresh && cachedLIPIndexResultPromise) {
    return cachedLIPIndexResultPromise;
  }

  cachedLIPIndexResultPromise = (async () => {
    const controller = new AbortController();
    let timeoutTriggered = false;
    const abort = () => controller.abort();
    const timeoutId = globalThis.setTimeout(() => {
      timeoutTriggered = true;
      controller.abort();
    }, LIP_INDEX_FETCH_TIMEOUT_MS);
    if (options?.signal) {
      if (options.signal.aborted) {
        controller.abort();
      } else {
        options.signal.addEventListener("abort", abort, { once: true });
      }
    }

    try {
      // Only mods whose latest version runs on the launcher's side and
      // platform are listed, and only theirs are fetched.
      const entries = launcherEntries(
        parseLipCatalog(await fetchLIPJson(LIP_INDEX_URL, controller.signal)),
      );
      const details = await fetchLIPModDetails(
        entries.map((entry) => entry.id),
        controller.signal,
      );
      if (entries.length > 0 && Object.keys(details).length === 0) {
        throw new Error("Failed to fetch lip package details");
      }
      // A detail that failed this time is reused from the previous load (as
      // Go's FetchDetails does); without one the mod is not listed.
      for (const entry of entries) {
        const old = previous?.detailsByIdentifier[entry.id]?.detail;
        if (!details[entry.id] && old) details[entry.id] = old;
      }
      cachedLIPIndexResult = buildLIPIndexResult(entries, details);
      return cachedLIPIndexResult;
    } catch (error) {
      if (timeoutTriggered) {
        throw new Error("ERR_LIP_INDEX_TIMEOUT");
      }
      throw error;
    } finally {
      globalThis.clearTimeout(timeoutId);
      options?.signal?.removeEventListener("abort", abort);
    }
  })();

  try {
    return await cachedLIPIndexResultPromise;
  } catch (error) {
    if (cachedLIPIndexResult) {
      return cachedLIPIndexResult;
    }
    throw error;
  } finally {
    cachedLIPIndexResultPromise = null;
  }
};

/** The mods the launcher lists, in the index's order. */
export async function fetchLIPPackagesIndex(options?: {
  forceRefresh?: boolean;
  signal?: AbortSignal;
}): Promise<LIPPackageBasicInfo[]> {
  const result = await fetchLIPIndexResult(options);
  return result.packages;
}

export async function fetchLIPSelfVariantRelations(options?: {
  forceRefresh?: boolean;
  signal?: AbortSignal;
}): Promise<LIPSelfVariantRelation[]> {
  const result = await fetchLIPIndexResult(options);
  return result.selfVariantRelations;
}

const findLIPPackageDetail = (
  detailsByIdentifier: Record<string, LIPPackageDetail>,
  identifier: string,
): LIPPackageDetail | null => {
  const target = String(identifier || "").trim();
  if (!target) return null;
  if (detailsByIdentifier[target]) return detailsByIdentifier[target];

  const targetBase = target.split("#")[0];
  if (targetBase && detailsByIdentifier[targetBase]) {
    return detailsByIdentifier[targetBase];
  }

  const lowerTargetBase = targetBase.toLowerCase();
  for (const [candidateIdentifier, detail] of Object.entries(
    detailsByIdentifier,
  )) {
    if (candidateIdentifier.toLowerCase() === lowerTargetBase) {
      return detail;
    }
  }
  return null;
};

/**
 * A mod's detail: from the loaded catalogue when it lists the mod (and its
 * detail loaded), else fetched directly. A mod with no variant for the
 * launcher's side and platform comes back with no `variantDetails`. Null
 * when the catalogue has no such mod.
 */
export async function fetchLIPPackageDetail(
  identifier: string,
  options?: {
    forceRefresh?: boolean;
    signal?: AbortSignal;
  },
): Promise<LIPPackageDetail | null> {
  const result = await fetchLIPIndexResult(options);
  const cached = findLIPPackageDetail(result.detailsByIdentifier, identifier);
  if (cached?.detail) return cached;

  const url = buildLIPModDetailUrl(identifier);
  if (!url || options?.signal?.aborted) return cached;
  const controller = new AbortController();
  const abort = () => controller.abort();
  options?.signal?.addEventListener("abort", abort, { once: true });
  const timeoutId = globalThis.setTimeout(abort, LIP_INDEX_FETCH_TIMEOUT_MS);
  try {
    const detail = parseLipModDetail(await fetchLIPJson(url, controller.signal));
    if (!detail) return cached;
    const parsed = buildLIPPackageDetail(cached ?? entryFromDetail(detail), detail);
    result.detailsByIdentifier[parsed.identifier] = parsed;
    return parsed;
  } catch {
    return cached;
  } finally {
    globalThis.clearTimeout(timeoutId);
    options?.signal?.removeEventListener("abort", abort);
  }
}
