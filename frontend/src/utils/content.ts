import { listDirectories } from "./fs";
import { compareVersions } from "./version";
import { GetUserGamertagMap } from "bindings/github.com/liteldev/LeviLauncher/userservice";

export async function listPlayers(usersRoot: string): Promise<string[]> {
  if (!usersRoot) return [];
  const entries = await listDirectories(usersRoot);
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

type UnknownRecord = Record<string, unknown>;

const LIP_INDEX_URL = "https://lipr.levimc.org/levilauncher.json";
const LL_DEPENDENCY_KEYS = new Set([
  "liteldev/levilamina",
  "liteldev/levilamina#client",
  "github.com/liteldev/levilamina",
  "github.com/liteldev/levilamina#client",
]);
const LEVI_LAMINA_IDENTIFIER = "liteldev/levilamina";

const normalizeLIPIdentifier = (value: string): string => {
  return String(value || "")
    .trim()
    .toLowerCase()
    .replace(/^https?:\/\//, "")
    .replace(/^\/+/, "")
    .replace(/\/+$/, "");
};

const getIdentifierOwnerRepo = (value: string): string => {
  const normalized = normalizeLIPIdentifier(value);
  if (!normalized) return "";

  const withoutVariant = normalized.split("#")[0];
  const parts = withoutVariant.split("/").filter(Boolean);
  if (parts.length < 2) {
    return withoutVariant;
  }
  return `${parts[parts.length - 2]}/${parts[parts.length - 1]}`;
};

const isLeviLaminaIdentifier = (value: string): boolean => {
  const normalized = normalizeLIPIdentifier(value);
  if (!normalized) return false;

  const withoutVariant = normalized.split("#")[0];
  if (withoutVariant === LEVI_LAMINA_IDENTIFIER) return true;
  return getIdentifierOwnerRepo(withoutVariant) === LEVI_LAMINA_IDENTIFIER;
};

const isLeviLaminaDependencyKey = (value: string): boolean => {
  const normalized = normalizeLIPIdentifier(value);
  if (!normalized) return false;

  if (LL_DEPENDENCY_KEYS.has(normalized)) return true;
  return isLeviLaminaIdentifier(normalized);
};

export interface LIPPackageBasicInfo {
  identifier: string;
  name: string;
  description: string;
  author: string;
  tags: string[];
  avatarUrl: string;
  projectUrl: string;
  hotness: number;
  updated: string;
  versions: string[];
  llDependencyRanges: string[];
  variants: LIPPackageVariantOption[];
  preferredVariantKey: string;
}

export interface LIPPackageVariantOption {
  key: string;
  label: string;
  packageIdentifier: string;
}

export interface LIPPackageFileInfo {
  version: string;
  dependencies: Record<string, string>;
  llDependencyRanges: string[];
  otherDependencies: Record<string, string>;
}

export interface LIPPackageVariantDetail extends LIPPackageVariantOption {
  files: LIPPackageFileInfo[];
  versions: string[];
  llDependencyRanges: string[];
}

export interface LIPPackageDetail extends LIPPackageBasicInfo {
  files: LIPPackageFileInfo[];
  variantDetails: LIPPackageVariantDetail[];
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

const isRecord = (value: unknown): value is UnknownRecord =>
  Boolean(value) && typeof value === "object" && !Array.isArray(value);

const toSafeString = (value: unknown): string =>
  typeof value === "string" ? value : "";

const toSafeNumber = (value: unknown): number => {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : 0;
};

const toOptionalSafeNumber = (value: unknown): number | null => {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === "string" && value.trim() === "") {
    return null;
  }
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : null;
};

const toStringArray = (value: unknown): string[] => {
  if (!Array.isArray(value)) return [];
  return value
    .filter((item): item is string => typeof item === "string")
    .map((item) => item.trim())
    .filter(Boolean);
};

type ParsedSemver = {
  major: number;
  minor: number;
  patch: number;
  pre: string[];
};

const normalizeSemverInput = (value: string): string =>
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

const compareSemverLike = (a: string, b: string): number => {
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

const buildVariantPackageIdentifier = (
  identifier: string,
  variantKey: string,
): string => {
  const normalizedIdentifier =
    String(identifier || "")
      .trim()
      .split("#")[0] || "";
  const normalizedVariantKey = String(variantKey || "").trim();
  if (!normalizedIdentifier) return "";
  return normalizedVariantKey
    ? `${normalizedIdentifier}#${normalizedVariantKey}`
    : `${normalizedIdentifier}#`;
};

const getVariantSortRank = (variantKey: string): number => {
  const normalizedVariantKey = String(variantKey || "")
    .trim()
    .toLowerCase();
  if (normalizedVariantKey === "client") return 0;
  if (!normalizedVariantKey) return 1;
  return 2;
};

const resolvePreferredVariantKey = (
  variants: LIPPackageVariantDetail[],
): string => {
  const clientVariant = variants.find(
    (variant) =>
      String(variant.key || "")
        .trim()
        .toLowerCase() === "client",
  );
  if (clientVariant) return clientVariant.key;
  return "";
};

const parsePackageVersionsRecord = (
  versionsValue: UnknownRecord | null,
): {
  files: LIPPackageFileInfo[];
  versions: string[];
  llDependencyRanges: string[];
} => {
  if (!versionsValue) {
    return { files: [], versions: [], llDependencyRanges: [] };
  }

  const files: LIPPackageFileInfo[] = [];
  const packageRangeSet = new Set<string>();

  for (const [version, packageMeta] of Object.entries(versionsValue)) {
    const normalizedVersion = String(version || "").trim();
    if (!normalizedVersion) continue;

    const dependencyMap: Record<string, string> = {};
    const otherDependencies: Record<string, string> = {};
    const llRangeSet = new Set<string>();

    if (isRecord(packageMeta)) {
      const dependencies = packageMeta["dependencies"];
      if (isRecord(dependencies)) {
        for (const [depKey, depRange] of Object.entries(dependencies)) {
          const normalizedKey = String(depKey || "").trim();
          if (!normalizedKey) continue;
          const normalizedRange = toSafeString(depRange).trim();
          if (!normalizedRange) continue;

          dependencyMap[normalizedKey] = normalizedRange;

          if (isLeviLaminaDependencyKey(normalizedKey)) {
            llRangeSet.add(normalizedRange);
            packageRangeSet.add(normalizedRange);
            continue;
          }
          otherDependencies[normalizedKey] = normalizedRange;
        }
      }
    }

    files.push({
      version: normalizedVersion,
      dependencies: dependencyMap,
      llDependencyRanges: Array.from(llRangeSet),
      otherDependencies,
    });
  }

  files.sort((a, b) => compareSemverLike(b.version, a.version));
  const versions = files.map((file) => file.version);
  return {
    files,
    versions,
    llDependencyRanges: Array.from(packageRangeSet),
  };
};

const parsePackageVariants = (
  value: unknown,
  identifier: string,
): LIPPackageVariantDetail[] => {
  if (!isRecord(value)) return [];

  const variants: LIPPackageVariantDetail[] = [];
  for (const [variantKey, variantMeta] of Object.entries(value)) {
    if (!isRecord(variantMeta)) continue;

    const parsed = parsePackageVersionsRecord(
      isRecord(variantMeta["versions"])
        ? (variantMeta["versions"] as UnknownRecord)
        : null,
    );
    if (parsed.versions.length === 0) continue;

    const normalizedVariantKey = String(variantKey || "").trim();
    variants.push({
      key: normalizedVariantKey,
      label: toSafeString(variantMeta["label"]).trim(),
      packageIdentifier: buildVariantPackageIdentifier(
        identifier,
        normalizedVariantKey,
      ),
      files: parsed.files,
      versions: parsed.versions,
      llDependencyRanges: parsed.llDependencyRanges,
    });
  }

  return variants.sort((a, b) => {
    const rankDiff = getVariantSortRank(a.key) - getVariantSortRank(b.key);
    if (rankDiff !== 0) return rankDiff;

    return a.packageIdentifier.localeCompare(b.packageIdentifier, undefined, {
      sensitivity: "base",
    });
  });
};

const normalizeHttpUrl = (value: string): string => {
  const raw = value.trim();
  if (!raw) return "";
  try {
    const parsed = new URL(raw);
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:") return "";
    return parsed.toString();
  } catch {
    return "";
  }
};

const inferAuthorFromIdentifier = (identifier: string): string => {
  const parts = identifier.split("/").filter(Boolean);
  if (parts.length >= 3 && parts[0].includes(".")) {
    return parts[1] || "";
  }
  if (parts.length >= 2) {
    return parts[parts.length - 2] || "";
  }
  return "";
};

const inferProjectUrlFromIdentifier = (identifier: string): string => {
  const direct = normalizeHttpUrl(identifier);
  if (direct) return direct;

  const normalized = identifier.trim().replace(/^\/+|\/+$/g, "");
  if (!normalized || !normalized.includes("/")) return "";

  const parts = normalized.split("/").filter(Boolean);
  if (
    parts.length === 2 &&
    !parts[0].includes(".") &&
    !parts[0].includes(":")
  ) {
    return normalizeHttpUrl(`https://github.com/${parts[0]}/${parts[1]}`);
  }

  return normalizeHttpUrl(`https://${normalized}`);
};

type ParsedLIPPackage = {
  basic: LIPPackageBasicInfo;
  detail: LIPPackageDetail;
};

const extractSelfVariantAliasesFromPackage = (
  value: unknown,
  identifier: string,
): string[] => {
  if (!isRecord(value)) return [];

  const normalizedIdentifier = normalizeLIPIdentifier(identifier).split("#")[0];
  if (!normalizedIdentifier) return [];

  const variants = value["variants"];
  const variantItems: unknown[] = Array.isArray(variants)
    ? variants
    : isRecord(variants)
      ? Object.values(variants)
      : [];
  if (variantItems.length === 0) return [];

  const aliasSet = new Set<string>();
  const collectDependencyKeys = (meta: unknown): string[] => {
    if (!isRecord(meta)) return [];
    const dependencies = meta["dependencies"];
    if (!isRecord(dependencies)) return [];
    return Object.keys(dependencies);
  };

  for (const variant of variantItems) {
    if (!isRecord(variant)) continue;
    const depKeys: string[] = [];

    depKeys.push(...collectDependencyKeys(variant));

    const versions = variant["versions"];
    if (isRecord(versions)) {
      for (const versionMeta of Object.values(versions)) {
        depKeys.push(...collectDependencyKeys(versionMeta));
      }
    }

    for (const depKey of depKeys) {
      const normalizedDep = normalizeLIPIdentifier(depKey);
      if (!normalizedDep) continue;
      const depBase = normalizedDep.split("#")[0];
      if (depBase !== normalizedIdentifier) continue;

      const hashIndex = normalizedDep.indexOf("#");
      if (hashIndex < 0) continue;
      const alias = normalizedDep
        .slice(hashIndex + 1)
        .trim()
        .toLowerCase();
      if (!alias) continue;
      aliasSet.add(alias);
    }
  }

  return Array.from(aliasSet).sort((a, b) => a.localeCompare(b));
};

const parseLIPPackage = (
  value: unknown,
  identifier: string,
): ParsedLIPPackage | null => {
  if (!isRecord(value)) return null;

  const normalizedIdentifier = toSafeString(identifier).trim();
  if (!normalizedIdentifier) return null;

  const info = isRecord(value["info"]) ? value["info"] : {};

  const name = toSafeString(info["name"]).trim() || normalizedIdentifier;
  const description = toSafeString(info["description"]).trim();
  const tags = toStringArray(info["tags"]);
  const avatarUrl = normalizeHttpUrl(toSafeString(info["avatar_url"]));
  const updated = toSafeString(value["updated_at"]).trim();
  const variantDetails = parsePackageVariants(
    value["variants"],
    normalizedIdentifier,
  );
  if (variantDetails.length === 0) return null;

  const preferredVariantKey = resolvePreferredVariantKey(variantDetails);
  if (!preferredVariantKey) return null;
  const preferredVariant =
    variantDetails.find((variant) => variant.key === preferredVariantKey) ||
    variantDetails[0];
  if (!preferredVariant) return null;
  const hotness =
    toOptionalSafeNumber(value["stargazer_count"]) ??
    toSafeNumber(value["stars"]);

  const basic: LIPPackageBasicInfo = {
    identifier: normalizedIdentifier,
    name,
    description,
    author: inferAuthorFromIdentifier(normalizedIdentifier),
    tags,
    avatarUrl,
    projectUrl: inferProjectUrlFromIdentifier(normalizedIdentifier),
    hotness,
    updated,
    versions: preferredVariant.versions,
    llDependencyRanges: preferredVariant.llDependencyRanges,
    variants: variantDetails.map((variant) => ({
      key: variant.key,
      label: variant.label,
      packageIdentifier: variant.packageIdentifier,
    })),
    preferredVariantKey: preferredVariant.key,
  };

  const detail: LIPPackageDetail = {
    ...basic,
    files: preferredVariant.files,
    variantDetails,
  };

  return { basic, detail };
};

const parseLIPIndexResult = (json: unknown): ParsedLIPIndexResult => {
  const empty: ParsedLIPIndexResult = {
    packages: [],
    detailsByIdentifier: {},
    selfVariantRelations: [],
  };
  if (!isRecord(json)) return empty;

  const packagesValue = json["packages"];
  if (!isRecord(packagesValue)) return empty;

  const deduplicatedPackages = new Map<string, LIPPackageBasicInfo>();
  const detailsByIdentifier: Record<string, LIPPackageDetail> = {};
  const selfVariantRelationsByIdentifier = new Map<
    string,
    LIPSelfVariantRelation
  >();

  for (const [identifier, item] of Object.entries(packagesValue)) {
    const parsed = parseLIPPackage(item, identifier);
    if (parsed && !deduplicatedPackages.has(parsed.basic.identifier)) {
      deduplicatedPackages.set(parsed.basic.identifier, parsed.basic);
      detailsByIdentifier[parsed.detail.identifier] = parsed.detail;
    }

    const selfAliases = extractSelfVariantAliasesFromPackage(item, identifier);
    if (selfAliases.length > 0) {
      const relationIdentifier =
        parsed?.basic.identifier || String(identifier || "").trim();
      if (relationIdentifier) {
        const existing =
          selfVariantRelationsByIdentifier.get(relationIdentifier);
        if (!existing) {
          selfVariantRelationsByIdentifier.set(relationIdentifier, {
            identifier: relationIdentifier,
            packageName: parsed?.basic.name || relationIdentifier,
            aliases: selfAliases,
          });
        } else {
          const merged = new Set([...existing.aliases, ...selfAliases]);
          existing.aliases = Array.from(merged).sort((a, b) =>
            a.localeCompare(b),
          );
          if (!existing.packageName && parsed?.basic.name) {
            existing.packageName = parsed.basic.name;
          }
        }
      }
    }

  }

  const selfVariantRelations = Array.from(
    selfVariantRelationsByIdentifier.values(),
  ).sort((a, b) =>
    (a.packageName || a.identifier).localeCompare(
      b.packageName || b.identifier,
      undefined,
      { sensitivity: "base" },
    ),
  );

  return {
    packages: Array.from(deduplicatedPackages.values()),
    detailsByIdentifier,
    selfVariantRelations,
  };
};

let cachedLIPIndexResult: ParsedLIPIndexResult | null = null;
let cachedLIPIndexResultPromise: Promise<ParsedLIPIndexResult> | null = null;
const LIP_INDEX_FETCH_TIMEOUT_MS = 8_000;

const fetchLIPIndexResult = async (options?: {
  forceRefresh?: boolean;
  signal?: AbortSignal;
}): Promise<ParsedLIPIndexResult> => {
  const forceRefresh = options?.forceRefresh === true;

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
      const response = await fetch(LIP_INDEX_URL, {
        method: "GET",
        cache: "no-store",
        signal: controller.signal,
      });

      if (!response.ok) {
        throw new Error(
          `Failed to fetch lip package index: ${response.status}`,
        );
      }

      const json: unknown = await response.json();
      cachedLIPIndexResult = parseLIPIndexResult(json);
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

  const lowerTarget = target.toLowerCase();
  const lowerTargetBase = targetBase.toLowerCase();
  for (const [candidateIdentifier, detail] of Object.entries(
    detailsByIdentifier,
  )) {
    const lowerCandidateIdentifier = candidateIdentifier.toLowerCase();
    if (
      lowerCandidateIdentifier === lowerTarget ||
      lowerCandidateIdentifier === lowerTargetBase
    ) {
      return detail;
    }
  }
  return null;
};

export async function fetchLIPPackageDetail(
  identifier: string,
  options?: {
    forceRefresh?: boolean;
    signal?: AbortSignal;
  },
): Promise<LIPPackageDetail | null> {
  const result = await fetchLIPIndexResult(options);
  return findLIPPackageDetail(result.detailsByIdentifier, identifier);
}
