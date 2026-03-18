import * as types from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import type {
  LIPPackageBasicInfo,
  LIPSelfVariantRelation,
} from "@/utils/content";

const LEVILAMINA_NORMALIZED = "levilamina";

type MappingKind = "none" | "name" | "self_variant";

export type ModLIPState = {
  sourceType: "none" | "ambiguous" | "unique" | "levilamina";
  identifier: string;
  identifierKey: string;
  targetVersion: string;
  canUpdate: boolean;
  packageName: string;
  mappingKind: MappingKind;
  matchedAlias: string;
};

export type ModListItem = {
  kind: "mod";
  key: string;
  folder: string;
  mod: types.ModInfo;
  lipState: ModLIPState;
};

export type LipGroupItem = {
  kind: "lip";
  key: string;
  identifier: string;
  identifierKey: string;
  displayIdentifier: string;
  packageName: string;
  installedVersion: string;
  explicitInstalled: boolean;
  folders: string[];
  mods: types.ModInfo[];
  childLabels: string[];
  childPreview: string;
  extraChildrenCount: number;
  allEnabled: boolean;
  anyEnabled: boolean;
  lipState: ModLIPState;
};

export type LIPPackageInstallState = {
  installed: boolean;
  explicitInstalled: boolean;
  installedVersion: string;
  error: string;
};

type LIPAliasCandidate = {
  identifier: string;
  identifierKey: string;
  lookupKey: string;
  packageName: string;
  alias: string;
  matchTokens: string[];
  packageHints: string[];
};

const normalizeName = (value: string): string =>
  String(value || "")
    .trim()
    .toLowerCase();

const normalizeIdentifier = (value: string): string =>
  String(value || "").trim();

const normalizeIdentifierLookupKey = (value: string): string =>
  String(value || "")
    .trim()
    .toLowerCase();

const normalizeVersionForCompare = (value: string): string =>
  String(value || "")
    .trim()
    .replace(/^v/i, "")
    .toLowerCase();

const normalizeMatchValue = (value: string): string =>
  String(value || "")
    .trim()
    .toLowerCase()
    .replace(/\s+/g, "");

const slugifyText = (value: string): string =>
  String(value || "")
    .replace(/([a-z0-9])([A-Z])/g, "$1-$2")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");

const isStableVersion = (value: string): boolean =>
  !String(value || "")
    .trim()
    .includes("-");

const ALIAS_GENERIC_TOKENS = new Set([
  "client",
  "server",
  "mod",
  "mods",
  "plugin",
  "plugins",
  "pack",
  "package",
]);

const buildAliasMatchTokens = (alias: string): string[] => {
  const normalized = normalizeMatchValue(alias);
  if (!normalized) return [];

  const set = new Set<string>();
  const add = (value: string) => {
    const token = normalizeMatchValue(value);
    if (!token || token.length < 3) return;
    if (ALIAS_GENERIC_TOKENS.has(token)) return;
    set.add(token);
  };

  add(normalized);
  add(normalized.replace(/_/g, "-"));
  add(normalized.replace(/-/g, "_"));

  const stripped = normalized.replace(
    /^(client|server|mod|mods|plugin|plugins|pack|package)[_-]/,
    "",
  );
  if (stripped && stripped !== normalized) {
    add(stripped);
    add(stripped.replace(/_/g, "-"));
    add(stripped.replace(/-/g, "_"));
  }

  for (const part of normalized.split(/[_-]+/)) {
    add(part);
  }

  const parts = normalized.split(/[_-]+/).filter(Boolean);
  if (parts.length > 1) {
    add(parts[parts.length - 1]);
  }

  return Array.from(set);
};

const buildPackageHints = (
  packageName: string,
  identifier: string,
): string[] => {
  const set = new Set<string>();
  const add = (value: string) => {
    const slug = slugifyText(value);
    if (!slug || slug.length < 4) return;
    set.add(slug);
    set.add(slug.replace(/-/g, ""));
  };

  add(packageName);

  const normalizedIdentifier =
    normalizeIdentifierLookupKey(identifier).split("#")[0];
  const parts = normalizedIdentifier.split("/").filter(Boolean);
  if (parts.length > 0) {
    add(parts[parts.length - 1]);
  }
  if (parts.length > 1) {
    add(`${parts[parts.length - 2]}-${parts[parts.length - 1]}`);
  }

  return Array.from(set);
};

const isTokenMatchedInValue = (token: string, rawValue: string): boolean => {
  const value = normalizeMatchValue(rawValue);
  if (!token || !value) return false;

  const tokenVariants = [
    token,
    token.replace(/_/g, "-"),
    token.replace(/-/g, "_"),
  ];
  const valueVariants = [
    value,
    value.replace(/_/g, "-"),
    value.replace(/-/g, "_"),
  ];

  for (const tokenVariant of tokenVariants) {
    for (const valueVariant of valueVariants) {
      if (valueVariant === tokenVariant) return true;
      if (valueVariant.endsWith(`-${tokenVariant}`)) return true;
      if (valueVariant.endsWith(`_${tokenVariant}`)) return true;
      if (valueVariant.split(/[_-]+/).includes(tokenVariant)) return true;
    }
  }

  return false;
};

const isPackageHintMatched = (hint: string, rawValue: string): boolean => {
  const value = normalizeMatchValue(rawValue);
  if (!hint || !value) return false;
  if (value.includes(hint)) return true;
  const normalizedHint = hint.replace(/-/g, "");
  if (normalizedHint && value.includes(normalizedHint)) return true;
  return false;
};

const isSelfVariantCandidateFuzzyMatched = (
  candidate: LIPAliasCandidate,
  values: string[],
): boolean => {
  const hasAliasTokenMatch = candidate.matchTokens.some((token) =>
    values.some((value) => isTokenMatchedInValue(token, value)),
  );
  if (!hasAliasTokenMatch) return false;

  if (candidate.packageHints.length === 0) return true;
  return candidate.packageHints.some((hint) =>
    values.some((value) => isPackageHintMatched(hint, value)),
  );
};

const pickTargetVersion = (versions: string[]): string => {
  const list = Array.isArray(versions)
    ? versions.map((item) => String(item || "").trim()).filter(Boolean)
    : [];
  if (list.length === 0) return "";
  const stable = list.find((version) => isStableVersion(version));
  return stable || list[0];
};

const formatChildPreview = (
  labels: string[],
): { preview: string; extra: number; deduped: string[] } => {
  const deduped = Array.from(
    new Set(labels.map((item) => String(item || "").trim()).filter(Boolean)),
  );
  deduped.sort((a, b) =>
    a.localeCompare(b, undefined, { sensitivity: "base" }),
  );
  const previewItems = deduped.slice(0, 2);
  return {
    preview: previewItems.join(", "),
    extra: Math.max(0, deduped.length - previewItems.length),
    deduped,
  };
};

export const buildDefaultModLIPState = (): ModLIPState => ({
  sourceType: "none",
  identifier: "",
  identifierKey: "",
  targetVersion: "",
  canUpdate: false,
  packageName: "",
  mappingKind: "none",
  matchedAlias: "",
});

export const resolveModFolder = (mod: types.ModInfo): string => {
  const folder = String((mod as any)?.folder || "").trim();
  if (folder) return folder;
  return String(mod?.name || "").trim();
};

export const isLeviLaminaMod = (mod: types.ModInfo): boolean =>
  normalizeName(mod?.name || "") === LEVILAMINA_NORMALIZED;

export const filterVisibleMods = (
  mods: types.ModInfo[] | null | undefined,
): types.ModInfo[] =>
  Array.isArray(mods) ? mods.filter((mod) => !isLeviLaminaMod(mod)) : [];

export const parseLIPPackageInstallState = (
  value: unknown,
): LIPPackageInstallState => {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return {
      installed: false,
      explicitInstalled: false,
      installedVersion: "",
      error: "",
    };
  }
  const record = value as Record<string, unknown>;
  return {
    installed: Boolean(record.installed),
    explicitInstalled: Boolean(record.explicitInstalled),
    installedVersion: String(record.installedVersion ?? "").trim(),
    error: String(record.error ?? "").trim(),
  };
};

export const buildSelfVariantCandidates = (
  relations: LIPSelfVariantRelation[],
): LIPAliasCandidate[] => {
  const dedup = new Map<string, LIPAliasCandidate>();

  for (const relation of relations || []) {
    const identifier = String(relation.identifier || "").trim();
    const identifierKey = normalizeIdentifier(identifier);
    const lookupKey = normalizeIdentifierLookupKey(identifierKey);
    if (!identifier || !identifierKey || !lookupKey) continue;
    const packageName = String(relation.packageName || "").trim() || identifier;
    const packageHints = buildPackageHints(packageName, identifier);

    for (const rawAlias of relation.aliases || []) {
      const alias = normalizeName(rawAlias || "");
      if (!alias) continue;

      const key = `${lookupKey}::${alias}`;
      if (dedup.has(key)) continue;

      dedup.set(key, {
        identifier,
        identifierKey,
        lookupKey,
        packageName,
        alias,
        matchTokens: buildAliasMatchTokens(alias),
        packageHints,
      });
    }
  }

  return Array.from(dedup.values());
};

export const buildModLIPStateByFolder = (args: {
  modsInfo: types.ModInfo[];
  lipSourceLoaded: boolean;
  selfVariantCandidates: LIPAliasCandidate[];
  lipPackagesByName: Record<string, LIPPackageBasicInfo[]>;
  lipPackageByIdentifier: Record<string, LIPPackageBasicInfo>;
}): Map<string, ModLIPState> => {
  const map = new Map<string, ModLIPState>();
  const {
    modsInfo,
    lipSourceLoaded,
    selfVariantCandidates,
    lipPackagesByName,
    lipPackageByIdentifier,
  } = args;

  const selfVariantCandidatesByAlias = new Map<string, LIPAliasCandidate[]>();
  for (const candidate of selfVariantCandidates) {
    const list = selfVariantCandidatesByAlias.get(candidate.alias) || [];
    list.push(candidate);
    selfVariantCandidatesByAlias.set(candidate.alias, list);
  }

  for (const mod of modsInfo) {
    const folder = resolveModFolder(mod);
    if (!folder) continue;

    if (isLeviLaminaMod(mod)) {
      map.set(folder, {
        sourceType: "levilamina",
        identifier: "",
        identifierKey: "",
        targetVersion: "",
        canUpdate: false,
        packageName: "",
        mappingKind: "none",
        matchedAlias: "",
      });
      continue;
    }

    if (!lipSourceLoaded) {
      map.set(folder, buildDefaultModLIPState());
      continue;
    }

    const aliasesToCheck = [
      normalizeName(mod.name || ""),
      normalizeName(folder),
    ]
      .filter(Boolean)
      .filter((value, index, arr) => arr.indexOf(value) === index);

    const aliasMatchesMap = new Map<string, LIPAliasCandidate>();
    for (const alias of aliasesToCheck) {
      const candidates = selfVariantCandidatesByAlias.get(alias) || [];
      for (const candidate of candidates) {
        aliasMatchesMap.set(candidate.lookupKey, candidate);
      }
    }

    if (aliasMatchesMap.size === 0) {
      const valuesToMatch = [
        normalizeMatchValue(mod.name || ""),
        normalizeMatchValue(folder),
      ]
        .filter(Boolean)
        .filter((value, index, arr) => arr.indexOf(value) === index);

      for (const candidate of selfVariantCandidates) {
        if (!isSelfVariantCandidateFuzzyMatched(candidate, valuesToMatch)) {
          continue;
        }
        aliasMatchesMap.set(candidate.lookupKey, candidate);
      }
    }

    const aliasMatches = Array.from(aliasMatchesMap.values());
    if (aliasMatches.length > 1) {
      map.set(folder, {
        sourceType: "ambiguous",
        identifier: "",
        identifierKey: "",
        targetVersion: "",
        canUpdate: false,
        packageName: "",
        mappingKind: "self_variant",
        matchedAlias: "",
      });
      continue;
    }

    if (aliasMatches.length === 1) {
      const matched = aliasMatches[0];
      const matchedPackage = lipPackageByIdentifier[matched.lookupKey];
      const targetVersion = pickTargetVersion(matchedPackage?.versions || []);
      const canUpdate =
        Boolean(targetVersion) &&
        normalizeVersionForCompare(mod.version || "") !==
          normalizeVersionForCompare(targetVersion);
      const resolvedIdentifier = String(
        matchedPackage?.identifier || matched.identifier,
      ).trim();
      const resolvedIdentifierKey = normalizeIdentifier(
        resolvedIdentifier || matched.identifierKey,
      );

      map.set(folder, {
        sourceType: matchedPackage ? "unique" : "none",
        identifier: resolvedIdentifier,
        identifierKey: resolvedIdentifierKey,
        targetVersion,
        canUpdate,
        packageName:
          String(matchedPackage?.name || "").trim() || matched.packageName,
        mappingKind: "self_variant",
        matchedAlias: matched.alias,
      });
      continue;
    }

    const normalizedModName = normalizeName(mod.name || "");
    const nameCandidates = normalizedModName
      ? lipPackagesByName[normalizedModName] || []
      : [];

    if (nameCandidates.length === 0) {
      map.set(folder, buildDefaultModLIPState());
      continue;
    }

    if (nameCandidates.length > 1) {
      map.set(folder, {
        sourceType: "ambiguous",
        identifier: "",
        identifierKey: "",
        targetVersion: "",
        canUpdate: false,
        packageName: "",
        mappingKind: "name",
        matchedAlias: "",
      });
      continue;
    }

    const matched = nameCandidates[0];
    const targetVersion = pickTargetVersion(matched.versions || []);
    const canUpdate =
      Boolean(targetVersion) &&
      normalizeVersionForCompare(mod.version || "") !==
        normalizeVersionForCompare(targetVersion);
    const resolvedIdentifier = String(matched.identifier || "").trim();
    const resolvedIdentifierKey = normalizeIdentifier(resolvedIdentifier);

    map.set(folder, {
      sourceType: "unique",
      identifier: resolvedIdentifier,
      identifierKey: resolvedIdentifierKey,
      targetVersion,
      canUpdate,
      packageName: matched.name || "",
      mappingKind: "name",
      matchedAlias: "",
    });
  }

  return map;
};

export type CandidateLIPIdentifier = {
  identifier: string;
  identifierKey: string;
};

export const collectCandidateLIPIdentifiers = (
  modLIPStateByFolder: Map<string, ModLIPState>,
): CandidateLIPIdentifier[] => {
  const identifiers = new Map<string, CandidateLIPIdentifier>();
  for (const state of modLIPStateByFolder.values()) {
    if (state.sourceType !== "unique") continue;
    const identifier = String(state.identifier || "").trim();
    const identifierKey = normalizeIdentifier(
      state.identifierKey || state.identifier,
    );
    const lookupKey = normalizeIdentifierLookupKey(identifierKey || identifier);
    if (!identifier || !identifierKey || !lookupKey) continue;
    if (!identifiers.has(lookupKey)) {
      identifiers.set(lookupKey, { identifier, identifierKey });
    }
  }
  return Array.from(identifiers.values()).sort((a, b) =>
    a.identifierKey.localeCompare(b.identifierKey, undefined, {
      sensitivity: "base",
    }),
  );
};

export const buildListItems = (args: {
  modsInfo: types.ModInfo[];
  modLIPStateByFolder: Map<string, ModLIPState>;
  lipInstallStateByIdentifier: Map<string, LIPPackageInstallState>;
  enabledByFolder: Map<string, boolean>;
  sortDirection?: "asc" | "desc";
}): { lipGroupItems: LipGroupItem[]; normalItems: ModListItem[] } => {
  const {
    modsInfo,
    modLIPStateByFolder,
    lipInstallStateByIdentifier,
    enabledByFolder,
    sortDirection = "asc",
  } = args;

  const grouped = new Map<
    string,
    {
      identifier: string;
      identifierKey: string;
      packageName: string;
      mods: types.ModInfo[];
      folders: string[];
      childLabels: string[];
      targetVersion: string;
      hasSelfVariantMapping: boolean;
    }
  >();

  for (const mod of modsInfo) {
    const folder = resolveModFolder(mod);
    if (!folder) continue;

    const state = modLIPStateByFolder.get(folder);
    if (
      !state ||
      state.sourceType !== "unique" ||
      !state.identifier ||
      !state.identifierKey
    ) {
      continue;
    }

    const lookupKey = normalizeIdentifierLookupKey(
      state.identifierKey || state.identifier,
    );
    if (!lookupKey) {
      continue;
    }

    if (!grouped.has(lookupKey)) {
      grouped.set(lookupKey, {
        identifier: state.identifier,
        identifierKey: state.identifierKey,
        packageName: state.packageName || mod.name || state.identifier,
        mods: [],
        folders: [],
        childLabels: [],
        targetVersion: state.targetVersion,
        hasSelfVariantMapping: false,
      });
    }

    const current = grouped.get(lookupKey)!;
    current.mods.push(mod);
    current.folders.push(folder);
    current.hasSelfVariantMapping =
      current.hasSelfVariantMapping || state.mappingKind === "self_variant";

    const childLabel =
      state.mappingKind === "self_variant" && state.matchedAlias
        ? state.matchedAlias
        : normalizeName(mod.name || "") || normalizeName(folder);
    if (childLabel) current.childLabels.push(childLabel);
  }

  const lipGroupItems: LipGroupItem[] = [];
  for (const group of grouped.values()) {
    const installState = lipInstallStateByIdentifier.get(group.identifierKey);
    const installedByLIP = Boolean(installState?.installed);
    const queryError = String(installState?.error || "").trim();
    const treatedAsLip =
      installedByLIP ||
      (group.hasSelfVariantMapping &&
        Boolean(queryError) &&
        queryError !== "ERR_LIP_NOT_INSTALLED");
    if (!treatedAsLip) continue;

    const uniqueFolders = Array.from(new Set(group.folders));
    const allEnabled =
      uniqueFolders.length > 0 &&
      uniqueFolders.every((folder) => Boolean(enabledByFolder.get(folder)));
    const anyEnabled = uniqueFolders.some((folder) =>
      Boolean(enabledByFolder.get(folder)),
    );

    const targetVersion = group.targetVersion || "";
    const installedVersionFromState = String(
      installState?.installedVersion || "",
    ).trim();
    const uniqueModVersions = Array.from(
      new Set(
        group.mods
          .map((mod) => String(mod.version || "").trim())
          .filter(Boolean),
      ),
    );
    const installedVersion =
      installedVersionFromState ||
      (uniqueModVersions.length === 1 ? uniqueModVersions[0] : "");
    const explicitInstalled = Boolean(installState?.explicitInstalled);
    const canUpdate =
      Boolean(targetVersion) &&
      group.mods.some(
        (mod) =>
          normalizeVersionForCompare(mod.version || "") !==
          normalizeVersionForCompare(targetVersion),
      );

    const previewData = formatChildPreview(group.childLabels);

    lipGroupItems.push({
      kind: "lip",
      key: `lip:${group.identifierKey}`,
      identifier: group.identifier,
      identifierKey: group.identifierKey,
      displayIdentifier: group.identifier,
      packageName: group.packageName || group.identifier,
      installedVersion,
      explicitInstalled,
      folders: uniqueFolders,
      mods: group.mods,
      childLabels: previewData.deduped,
      childPreview: previewData.preview,
      extraChildrenCount: previewData.extra,
      allEnabled,
      anyEnabled,
      lipState: {
        sourceType: "unique",
        identifier: group.identifier,
        identifierKey: group.identifierKey,
        targetVersion,
        canUpdate,
        packageName: group.packageName || group.identifier,
        mappingKind: group.hasSelfVariantMapping ? "self_variant" : "name",
        matchedAlias: "",
      },
    });
  }

  lipGroupItems.sort((a, b) => {
    const nameA = String(a.packageName || "").toLowerCase();
    const nameB = String(b.packageName || "").toLowerCase();
    if (nameA < nameB) return sortDirection === "asc" ? -1 : 1;
    if (nameA > nameB) return sortDirection === "asc" ? 1 : -1;
    return 0;
  });

  const lipManagedFolderSet = new Set<string>();
  for (const group of lipGroupItems) {
    for (const folder of group.folders) {
      lipManagedFolderSet.add(folder);
    }
  }

  const normalMods = modsInfo.filter((mod) => {
    const folder = resolveModFolder(mod);
    return folder ? !lipManagedFolderSet.has(folder) : false;
  });

  normalMods.sort((a, b) => {
    const nameA = String(a.name || "").toLowerCase();
    const nameB = String(b.name || "").toLowerCase();
    if (nameA < nameB) return sortDirection === "asc" ? -1 : 1;
    if (nameA > nameB) return sortDirection === "asc" ? 1 : -1;
    return 0;
  });

  const normalItems: ModListItem[] = normalMods.map((mod) => {
    const folder = resolveModFolder(mod);
    return {
      kind: "mod",
      key: `mod:${folder}`,
      folder,
      mod,
      lipState: modLIPStateByFolder.get(folder) || buildDefaultModLIPState(),
    };
  });

  return { lipGroupItems, normalItems };
};
