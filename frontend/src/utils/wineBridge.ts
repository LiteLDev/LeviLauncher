import {
  compareSemverLike,
  isLeviLaminaVersionCompatible,
  type LIPPackageDetail,
  type LIPPackageFileInfo,
} from "./content";

export const LEVI_WINE_BRIDGE_IDENTIFIER = "github.com/wzyyyyyyy/LeviWineBridge";
export const LEVI_WINE_BRIDGE_NAME = "LeviWineBridge";
const LEVI_WINE_BRIDGE_LOOKUP_KEY = "github.com/wzyyyyyyy/leviwinebridge";
const LEVI_WINE_BRIDGE_ENTRY = "leviwinebridge.dll";

export type WineBridgeInstallStatus = "not_installed" | "disabled" | "enabled";

export type WineBridgeInstanceModLike = {
  name?: string;
  entry?: string;
  version?: string;
  folder?: string;
};

export type WineBridgeInstanceState = {
  status: WineBridgeInstallStatus;
  installedVersion: string;
  folders: string[];
  disabledFolders: string[];
};

const normalizePackageLookupKey = (value: string): string => {
  let key = String(value || "")
    .trim()
    .toLowerCase()
    .replace(/^https?:\/\//, "");
  const versionIndex = key.indexOf("@");
  if (versionIndex >= 0) key = key.slice(0, versionIndex);
  const variantIndex = key.indexOf("#");
  if (variantIndex >= 0) key = key.slice(0, variantIndex);
  const parts = key.split("/").filter(Boolean);
  if (parts.length === 2 && !parts[0].includes(".")) {
    return `github.com/${parts.join("/")}`;
  }
  return parts.join("/");
};

const normalizeDisplayVersion = (value: string | undefined): string =>
  String(value || "")
    .trim()
    .replace(/^v/i, "");

export function detectWineBridgeInstanceState(params: {
  modsInfo?: WineBridgeInstanceModLike[] | null;
  enabledByFolder?: Map<string, boolean> | null;
  lipState?: { installed?: boolean; installedVersion?: string } | null;
  modLIPStateByFolder?: Map<
    string,
    { identifierKey?: string; identifier?: string }
  > | null;
}): WineBridgeInstanceState {
  const { modsInfo, enabledByFolder, lipState, modLIPStateByFolder } = params;
  const matchedFolders: string[] = [];
  const seenFolders = new Set<string>();
  let modVersion = "";

  if (Array.isArray(modsInfo)) {
    for (const mod of modsInfo) {
      const folder = String(mod?.folder || mod?.name || "").trim();
      const nameNorm = String(mod?.name || "").trim().toLowerCase();
      const entryNorm = String(mod?.entry || "").trim().toLowerCase();
      const folderNorm = folder.toLowerCase();
      const mapped = folder ? modLIPStateByFolder?.get(folder) : undefined;
      const mappedKey = mapped
        ? normalizePackageLookupKey(mapped.identifierKey || mapped.identifier || "")
        : "";

      const isMatch =
        nameNorm === LEVI_WINE_BRIDGE_NAME.toLowerCase() ||
        entryNorm === LEVI_WINE_BRIDGE_ENTRY ||
        folderNorm === LEVI_WINE_BRIDGE_NAME.toLowerCase() ||
        mappedKey === LEVI_WINE_BRIDGE_LOOKUP_KEY;

      if (!isMatch) continue;
      if (!modVersion) {
        modVersion = normalizeDisplayVersion(mod?.version);
      }
      if (folder && !seenFolders.has(folder)) {
        seenFolders.add(folder);
        matchedFolders.push(folder);
      }
    }
  }

  const lipInstalled = Boolean(lipState?.installed);
  const lipVersion = lipInstalled
    ? normalizeDisplayVersion(lipState?.installedVersion)
    : "";
  const installedVersion = lipVersion || modVersion;

  if (matchedFolders.length > 0) {
    const disabledFolders = matchedFolders.filter(
      (folder) => enabledByFolder?.get(folder) !== true,
    );
    return {
      status:
        disabledFolders.length < matchedFolders.length ? "enabled" : "disabled",
      installedVersion,
      folders: matchedFolders,
      disabledFolders,
    };
  }

  if (lipInstalled) {
    return {
      status: "enabled",
      installedVersion,
      folders: [],
      disabledFolders: [],
    };
  }

  return {
    status: "not_installed",
    installedVersion: "",
    folders: [],
    disabledFolders: [],
  };
}

export function hasWineBridgeUpdate(
  installedVersion: string,
  candidateVersion: string,
): boolean {
  const installed = normalizeDisplayVersion(installedVersion);
  const candidate = normalizeDisplayVersion(candidateVersion);
  if (!installed || !candidate) return false;
  return compareSemverLike(candidate, installed) > 0;
}

export type WineBridgeCatalogStatus = "idle" | "loading" | "ready" | "error";

export type WineBridgeCandidate = {
  identifier: string;
  version: string;
  prerelease: boolean;
};

export type WineBridgeOptionStatusKind =
  | "loading"
  | "compatible"
  | "incompatible"
  | "unavailable";

export type WineBridgeOptionState = {
  candidate: WineBridgeCandidate | null;
  compatible: boolean;
  checked: boolean;
  switchDisabled: boolean;
  statusKind: WineBridgeOptionStatusKind;
};

const pickPreferredVariant = (
  pkg: Pick<
    LIPPackageDetail,
    "identifier" | "preferredVariantKey" | "files" | "variantDetails"
  >,
): { packageIdentifier: string; files: LIPPackageFileInfo[] } => {
  const variants = Array.isArray(pkg.variantDetails) ? pkg.variantDetails : [];
  if (variants.length > 0) {
    const preferred =
      variants.find((variant) => variant.key === pkg.preferredVariantKey) ??
      variants[0];
    return {
      packageIdentifier:
        String(preferred.packageIdentifier || "").trim() ||
        String(pkg.identifier || "").trim() ||
        LEVI_WINE_BRIDGE_IDENTIFIER,
      files: Array.isArray(preferred.files) ? preferred.files : [],
    };
  }
  return {
    packageIdentifier:
      String(pkg.identifier || "").trim() || LEVI_WINE_BRIDGE_IDENTIFIER,
    files: Array.isArray(pkg.files) ? pkg.files : [],
  };
};

/**
 * Selects the best LeviWineBridge release compatible with `llVersion`.
 * Prefers the highest stable version; falls back to the highest compatible
 * prerelease when no stable release matches.
 */
export function selectCompatibleWineBridgeRelease(
  pkg:
    | Pick<
        LIPPackageDetail,
        "identifier" | "preferredVariantKey" | "files" | "variantDetails"
      >
    | null
    | undefined,
  llVersion: string,
): WineBridgeCandidate | null {
  const normalizedLL = String(llVersion || "").trim();
  if (!pkg || !normalizedLL) return null;

  const { packageIdentifier, files } = pickPreferredVariant(pkg);
  const compatibleFiles = files.filter((file) => {
    const version = String(file?.version || "").trim();
    if (!version) return false;
    const ranges = Array.isArray(file.llDependencyRanges)
      ? file.llDependencyRanges
      : [];
    if (ranges.length === 0) return true;
    return isLeviLaminaVersionCompatible(normalizedLL, ranges);
  });

  if (compatibleFiles.length === 0) return null;

  const sortNewestFirst = (list: LIPPackageFileInfo[]) =>
    [...list].sort((a, b) => compareSemverLike(b.version, a.version));

  const stable = sortNewestFirst(
    compatibleFiles.filter((file) => !file.prerelease),
  );
  const picked = stable[0] ?? sortNewestFirst(compatibleFiles)[0];
  if (!picked) return null;

  return {
    identifier: packageIdentifier,
    version: String(picked.version).trim(),
    prerelease: Boolean(picked.prerelease),
  };
}

export function resolveWineBridgeOptionState(params: {
  catalogStatus: WineBridgeCatalogStatus;
  pkg:
    | Pick<
        LIPPackageDetail,
        "identifier" | "preferredVariantKey" | "files" | "variantDetails"
      >
    | null
    | undefined;
  llVersion: string;
  userEnabled: boolean;
}): WineBridgeOptionState {
  const { catalogStatus, pkg, llVersion, userEnabled } = params;

  if (catalogStatus === "idle" || catalogStatus === "loading") {
    return {
      candidate: null,
      compatible: false,
      checked: false,
      switchDisabled: true,
      statusKind: "loading",
    };
  }

  if (catalogStatus === "error" || !pkg) {
    return {
      candidate: null,
      compatible: false,
      checked: false,
      switchDisabled: true,
      statusKind: "unavailable",
    };
  }

  const candidate = selectCompatibleWineBridgeRelease(pkg, llVersion);
  if (!candidate) {
    return {
      candidate: null,
      compatible: false,
      checked: false,
      switchDisabled: true,
      statusKind: "incompatible",
    };
  }

  return {
    candidate,
    compatible: true,
    checked: Boolean(userEnabled),
    switchDisabled: false,
    statusKind: "compatible",
  };
}
