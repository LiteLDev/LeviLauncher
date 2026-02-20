import { listDirectories } from "./fs";
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

const LIP_INDEX_URL = "https://lipr.levimc.org/index.json";

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
}

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

const toStringArray = (value: unknown): string[] => {
  if (!Array.isArray(value)) return [];
  return value
    .filter((item): item is string => typeof item === "string")
    .map((item) => item.trim())
    .filter(Boolean);
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
  if (!identifier.includes("/")) return "";

  const candidate = normalizeHttpUrl(`https://${identifier}`);
  return candidate;
};

const parseLIPPackage = (value: unknown): LIPPackageBasicInfo | null => {
  if (!isRecord(value)) return null;

  const identifier = toSafeString(value["tooth"]).trim();
  if (!identifier) return null;

  const info = isRecord(value["info"]) ? value["info"] : {};

  const name = toSafeString(info["name"]).trim() || identifier;
  const description = toSafeString(info["description"]).trim();
  const tags = toStringArray(info["tags"]);
  const avatarUrl = normalizeHttpUrl(toSafeString(info["avatar_url"]));
  const updated = toSafeString(value["updated"]).trim();

  return {
    identifier,
    name,
    description,
    author: inferAuthorFromIdentifier(identifier),
    tags,
    avatarUrl,
    projectUrl: inferProjectUrlFromIdentifier(identifier),
    hotness: toSafeNumber(value["stars"]),
    updated,
    versions: toStringArray(value["versions"]),
  };
};

let cachedLIPPackages: LIPPackageBasicInfo[] | null = null;
let cachedLIPPackagesPromise: Promise<LIPPackageBasicInfo[]> | null = null;

export async function fetchLIPPackagesIndex(options?: {
  forceRefresh?: boolean;
  signal?: AbortSignal;
}): Promise<LIPPackageBasicInfo[]> {
  const forceRefresh = options?.forceRefresh === true;

  if (!forceRefresh && cachedLIPPackages) {
    return cachedLIPPackages;
  }

  if (!forceRefresh && cachedLIPPackagesPromise) {
    return cachedLIPPackagesPromise;
  }

  cachedLIPPackagesPromise = (async () => {
    const response = await fetch(LIP_INDEX_URL, {
      method: "GET",
      cache: "no-store",
      signal: options?.signal,
    });

    if (!response.ok) {
      throw new Error(`Failed to fetch LIP package index: ${response.status}`);
    }

    const json: unknown = await response.json();
    if (!isRecord(json)) {
      cachedLIPPackages = [];
      return cachedLIPPackages;
    }

    const rawPackages = Array.isArray(json["packages"]) ? json["packages"] : [];
    const deduplicated = new Map<string, LIPPackageBasicInfo>();

    for (const item of rawPackages) {
      const parsed = parseLIPPackage(item);
      if (!parsed || deduplicated.has(parsed.identifier)) continue;
      deduplicated.set(parsed.identifier, parsed);
    }

    cachedLIPPackages = Array.from(deduplicated.values());
    return cachedLIPPackages;
  })();

  try {
    return await cachedLIPPackagesPromise;
  } catch (error) {
    if (cachedLIPPackages) {
      return cachedLIPPackages;
    }
    throw error;
  } finally {
    cachedLIPPackagesPromise = null;
  }
}
