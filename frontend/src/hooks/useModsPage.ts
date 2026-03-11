import React, { useEffect, useMemo, useRef, useState } from "react";
import { addToast, useDisclosure } from "@heroui/react";
import { useNavigate, useBlocker } from "react-router-dom";
import { Call, Events } from "@wailsio/runtime";
import { ListDir, OpenPathDir } from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import {
  GetVersionMeta,
  GetVersionsDir,
} from "bindings/github.com/liteldev/LeviLauncher/versionservice";
import {
  OpenModsExplorer,
  DeleteMod,
  EnableMod,
  DisableMod,
  ImportModZipPath,
  ImportModDllPath,
} from "bindings/github.com/liteldev/LeviLauncher/modsservice";
import * as types from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import {
  fetchLIPPackagesIndex,
  fetchLIPSelfVariantRelations,
  type LIPPackageBasicInfo,
  type LIPSelfVariantRelation,
} from "@/utils/content";
import { readCurrentVersionName } from "@/utils/currentVersion";
import { useCurrentVersion } from "@/utils/CurrentVersionContext";
import { useModIntelligence } from "@/utils/ModIntelligenceContext";
import { useLipTaskConsole } from "@/utils/LipTaskConsoleContext";
import { setNavLockReason } from "@/hooks/useAppNavigation";
import { ROUTES } from "@/constants/routes";

const LEVILAMINA_NORMALIZED = "levilamina";
const ERR_LL_MANAGED_IN_VERSION_SETTINGS =
  "ERR_LL_MANAGED_IN_VERSION_SETTINGS";
const ERR_NO_UPDATE_SOURCE = "ERR_NO_UPDATE_SOURCE";
const ERR_ALREADY_LATEST = "ERR_ALREADY_LATEST";
const ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS =
  "ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS";
const ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY =
  "ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY";

type TFunc = (key: string, opts?: Record<string, unknown>) => string;
export type TabKey = "all" | "normal" | "lip";
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

export type VisibleItem = ModListItem | LipGroupItem;

type TaskLogLevel = "info" | "success" | "warning" | "error";
type TaskLogger = (level: TaskLogLevel, message: string) => void;

type LIPPackageInstallState = {
  installed: boolean;
  explicitInstalled: boolean;
  installedVersion: string;
  error: string;
};

type LIPAliasCandidate = {
  identifier: string;
  identifierKey: string;
  packageName: string;
  alias: string;
  matchTokens: string[];
  packageHints: string[];
};

type CandidateLIPIdentifier = {
  identifier: string;
  identifierKey: string;
};

const normalizeName = (value: string): string =>
  String(value || "").trim().toLowerCase();

const normalizeIdentifier = (value: string): string =>
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

const buildPackageHints = (packageName: string, identifier: string): string[] => {
  const set = new Set<string>();
  const add = (value: string) => {
    const slug = slugifyText(value);
    if (!slug || slug.length < 4) return;
    set.add(slug);
    set.add(slug.replace(/-/g, ""));
  };

  add(packageName);

  const normalizedIdentifier = normalizeIdentifier(identifier).split("#")[0];
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

  const tokenVariants = [token, token.replace(/_/g, "-"), token.replace(/-/g, "_")];
  const valueVariants = [value, value.replace(/_/g, "-"), value.replace(/-/g, "_")];

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

const isStableVersion = (value: string): boolean =>
  !String(value || "").trim().includes("-");

export const resolveModFolder = (mod: types.ModInfo): string => {
  const folder = String((mod as any)?.folder || "").trim();
  if (folder) return folder;
  return String(mod?.name || "").trim();
};

export const isLeviLaminaMod = (mod: types.ModInfo): boolean =>
  normalizeName(mod?.name || "") === LEVILAMINA_NORMALIZED;

const filterVisibleMods = (
  mods: types.ModInfo[] | null | undefined,
): types.ModInfo[] =>
  Array.isArray(mods) ? mods.filter((mod) => !isLeviLaminaMod(mod)) : [];

const parseErrorCode = (value: unknown): string => {
  const source =
    typeof value === "string"
      ? value
      : value instanceof Error
        ? value.message
        : String(value || "");
  const trimmed = source.trim();
  if (!trimmed) return "";
  const match = trimmed.match(/ERR_[A-Z0-9_]+/);
  return match ? match[0] : trimmed;
};

const resolveErrorText = (t: TFunc, value: unknown): string => {
  const code = parseErrorCode(value);
  if (!code) return t("common.error");

  const direct = t(code);
  if (direct !== code) {
    return direct;
  }

  const byErrorKey = t(`errors.${code}`);
  if (byErrorKey !== `errors.${code}`) {
    return byErrorKey;
  }

  return code;
};

const pickTargetVersion = (versions: string[]): string => {
  const list = Array.isArray(versions)
    ? versions.map((item) => String(item || "").trim()).filter(Boolean)
    : [];
  if (list.length === 0) return "";
  const stable = list.find((version) => isStableVersion(version));
  return stable || list[0];
};

const buildDefaultModLIPState = (): ModLIPState => ({
  sourceType: "none",
  identifier: "",
  identifierKey: "",
  targetVersion: "",
  canUpdate: false,
  packageName: "",
  mappingKind: "none",
  matchedAlias: "",
});

const formatChildPreview = (
  labels: string[],
): { preview: string; extra: number; deduped: string[] } => {
  const deduped = Array.from(
    new Set(labels.map((item) => String(item || "").trim()).filter(Boolean)),
  );
  deduped.sort((a, b) => a.localeCompare(b, undefined, { sensitivity: "base" }));
  const previewItems = deduped.slice(0, 2);
  return {
    preview: previewItems.join(", "),
    extra: Math.max(0, deduped.length - previewItems.length),
    deduped,
  };
};

export const useModsPage = (
  t: TFunc,
  scrollRef: React.RefObject<HTMLDivElement | null>,
) => {
  const navigate = useNavigate();
  const { runWithLipTask } = useLipTaskConsole();
  const { currentVersionName } = useCurrentVersion();
  const {
    ensureInstanceHydrated,
    getInstanceSnapshot,
    ensurePackageInstallState,
    refreshInstance,
    snapshotRevision,
  } = useModIntelligence();

  const [modsInfo, setModsInfo] = useState<Array<types.ModInfo>>([]);
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(false);
  const [importing, setImporting] = useState(false);
  const [errorMsg, setErrorMsg] = useState("");
  const [errorFile, setErrorFile] = useState("");
  const [resultSuccess, setResultSuccess] = useState<string[]>([]);
  const [resultFailed, setResultFailed] = useState<
    Array<{ name: string; err: string }>
  >([]);
  const [currentFile, setCurrentFile] = useState("");
  const [activeMod, setActiveMod] = useState<types.ModInfo | null>(null);
  const [activeLipIdentifier, setActiveLipIdentifier] = useState("");
  const [deleting, setDeleting] = useState<boolean>(false);
  const [batchUpdating, setBatchUpdating] = useState<boolean>(false);
  const [batchUninstalling, setBatchUninstalling] = useState<boolean>(false);
  const [demotedWarningNames, setDemotedWarningNames] = useState<string[]>([]);
  const [actionConfirmTitle, setActionConfirmTitle] = useState("");
  const [actionConfirmBody, setActionConfirmBody] = useState("");
  const [actionConfirming, setActionConfirming] = useState<boolean>(false);
  const [enabledByFolder, setEnabledByFolder] = useState<Map<string, boolean>>(
    new Map(),
  );
  const [onlyEnabled, setOnlyEnabled] = useState<boolean>(false);
  const [dllName, setDllName] = useState("");
  const [dllType, setDllType] = useState("preload-native");
  const [dllVersion, setDllVersion] = useState("0.0.0");
  const [selectedKeys, setSelectedKeys] = useState<Set<string>>(new Set());
  const [sortConfig, setSortConfig] = useState<{
    key: string;
    direction: "asc" | "desc";
  }>({ key: "name", direction: "asc" });
  const [tabKey, setTabKey] = useState<TabKey>("all");
  const [gameVersion, setGameVersion] = useState("");
  const [lipPackagesByName, setLIPPackagesByName] = useState<
    Record<string, LIPPackageBasicInfo[]>
  >({});
  const [lipPackageByIdentifier, setLIPPackageByIdentifier] = useState<
    Record<string, LIPPackageBasicInfo>
  >({});
  const [lipSelfVariantRelations, setLipSelfVariantRelations] = useState<
    LIPSelfVariantRelation[]
  >([]);
  const [lipInstallStateByIdentifier, setLipInstallStateByIdentifier] = useState<
    Map<string, LIPPackageInstallState>
  >(new Map());
  const [lipSourceLoaded, setLIPSourceLoaded] = useState(false);
  const [installStateLoaded, setInstallStateLoaded] = useState(false);

  const dllResolveRef = useRef<((ok: boolean) => void) | null>(null);
  const dllConfirmRef = useRef<{
    name: string;
    type: string;
    version: string;
  } | null>(null);
  const dupResolveRef = useRef<((overwrite: boolean) => void) | null>(null);
  const dupNameRef = useRef<string>("");
  const actionConfirmRunnerRef = useRef<(() => Promise<void>) | null>(null);
  const lastScrollTopRef = useRef<number>(0);
  const restorePendingRef = useRef<boolean>(false);

  const {
    isOpen: errOpen,
    onOpen: errOnOpen,
    onOpenChange: errOnOpenChange,
    onClose: errOnClose,
  } = useDisclosure();
  const {
    isOpen: delOpen,
    onOpen: delOnOpen,
    onOpenChange: delOnOpenChange,
    onClose: delOnClose,
  } = useDisclosure();
  const {
    isOpen: dllOpen,
    onOpen: dllOnOpen,
    onOpenChange: dllOnOpenChange,
    onClose: dllOnClose,
  } = useDisclosure();
  const {
    isOpen: dupOpen,
    onOpen: dupOnOpen,
    onOpenChange: dupOnOpenChange,
    onClose: dupOnClose,
  } = useDisclosure();
  const {
    isOpen: delCfmOpen,
    onOpen: delCfmOnOpen,
    onOpenChange: delCfmOnOpenChange,
    onClose: delCfmOnClose,
  } = useDisclosure();
  const {
    isOpen: infoOpen,
    onOpen: infoOnOpen,
    onOpenChange: infoOnOpenChange,
    onClose: infoOnClose,
  } = useDisclosure();
  const {
    isOpen: batchUpdateOpen,
    onOpen: batchUpdateOnOpen,
    onOpenChange: batchUpdateOnOpenChange,
    onClose: batchUpdateOnClose,
  } = useDisclosure();
  const {
    isOpen: batchUninstallOpen,
    onOpen: batchUninstallOnOpen,
    onOpenChange: batchUninstallOnOpenChange,
    onClose: batchUninstallOnClose,
  } = useDisclosure();
  const {
    isOpen: demotedWarningOpen,
    onOpen: demotedWarningOnOpen,
    onClose: demotedWarningOnClose,
  } = useDisclosure();
  const {
    isOpen: actionConfirmOpen,
    onOpen: actionConfirmOnOpen,
    onClose: actionConfirmOnClose,
  } = useDisclosure();

  useBlocker(() => importing);

  useEffect(() => {
    setNavLockReason("mods-import", importing);
    return () => {
      if (importing) {
        setNavLockReason("mods-import", false);
      }
    };
  }, [importing]);

  const callMinecraftByName = async <T,>(
    method: string,
    ...args: unknown[]
  ): Promise<T> => {
    return (await Call.ByName(`main.Minecraft.${method}`, ...args)) as T;
  };

  const resolveLIPIdentifierForRequest = (
    identifier: string,
    identifierKey?: string,
  ): string => {
    const rawIdentifier = String(identifier || "").trim();
    const normalized = normalizeIdentifier(identifierKey || rawIdentifier);
    if (!normalized) return rawIdentifier;
    const fromIndex = String(
      lipPackageByIdentifier[normalized]?.identifier || "",
    ).trim();
    if (fromIndex) return fromIndex;
    return rawIdentifier;
  };

  const activeVersionName = useMemo(
    () => currentVersionName || readCurrentVersionName(),
    [currentVersionName],
  );

  useEffect(() => {
    if (!activeVersionName) {
      setModsInfo([]);
      setEnabledByFolder(new Map());
      setLipInstallStateByIdentifier(new Map());
      setInstallStateLoaded(false);
      return;
    }
    void ensureInstanceHydrated(activeVersionName, {
      background: true,
      reason: "mods-page-enter",
    });
  }, [activeVersionName, ensureInstanceHydrated]);

  useEffect(() => {
    if (!activeVersionName) return;
    const snapshot = getInstanceSnapshot(activeVersionName);
    if (!snapshot) return;

    setModsInfo(Array.isArray(snapshot.modsInfo) ? snapshot.modsInfo : []);
    setEnabledByFolder(
      snapshot.enabledByFolder instanceof Map
        ? new Map(snapshot.enabledByFolder)
        : new Map(),
    );
    setLipInstallStateByIdentifier(
      snapshot.lipInstallStateByIdentifier instanceof Map
        ? new Map(snapshot.lipInstallStateByIdentifier)
        : new Map(),
    );
    if (snapshot.status !== "loading") {
      setInstallStateLoaded(true);
    }
  }, [activeVersionName, getInstanceSnapshot, snapshotRevision]);

  useEffect(() => {
    let cancelled = false;
    const loadLIPPackages = async () => {
      try {
        const [packages, relations] = await Promise.all([
          fetchLIPPackagesIndex(),
          fetchLIPSelfVariantRelations(),
        ]);
        if (cancelled) return;

        const groupedByName: Record<string, LIPPackageBasicInfo[]> = {};
        const groupedByIdentifier: Record<string, LIPPackageBasicInfo> = {};

        for (const pkg of packages) {
          const normalizedName = normalizeName(pkg.name || "");
          if (normalizedName) {
            if (!Array.isArray(groupedByName[normalizedName])) {
              groupedByName[normalizedName] = [];
            }
            groupedByName[normalizedName].push(pkg);
          }

          const normalizedIdentifier = normalizeIdentifier(pkg.identifier || "");
          if (normalizedIdentifier && !groupedByIdentifier[normalizedIdentifier]) {
            groupedByIdentifier[normalizedIdentifier] = pkg;
          }
        }

        setLIPPackagesByName(groupedByName);
        setLIPPackageByIdentifier(groupedByIdentifier);
        setLipSelfVariantRelations(Array.isArray(relations) ? relations : []);
      } catch {
        if (cancelled) return;
        setLIPPackagesByName({});
        setLIPPackageByIdentifier({});
        setLipSelfVariantRelations([]);
      } finally {
        if (!cancelled) setLIPSourceLoaded(true);
      }
    };

    void loadLIPPackages();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    const fetchVersion = async () => {
      if (!activeVersionName) return;
      try {
        const meta = await GetVersionMeta(activeVersionName);
        if (meta && meta.gameVersion) {
          setGameVersion(meta.gameVersion);
        } else {
          setGameVersion(activeVersionName);
        }
      } catch {
        setGameVersion(activeVersionName);
      }
    };
    void fetchVersion();
  }, [activeVersionName]);

  const modsByFolder = useMemo(() => {
    const map = new Map<string, types.ModInfo>();
    for (const mod of modsInfo) {
      const folder = resolveModFolder(mod);
      if (!folder) continue;
      map.set(folder, mod);
    }
    return map;
  }, [modsInfo]);

  const selfVariantCandidates = useMemo(() => {
    const dedup = new Map<string, LIPAliasCandidate>();

    for (const relation of lipSelfVariantRelations) {
      const identifier = String(relation.identifier || "").trim();
      const identifierKey = normalizeIdentifier(identifier);
      if (!identifier || !identifierKey) continue;
      const packageName =
        String(relation.packageName || "").trim() || identifier;
      const packageHints = buildPackageHints(packageName, identifier);

      for (const rawAlias of relation.aliases || []) {
        const alias = normalizeName(rawAlias || "");
        if (!alias) continue;

        const key = `${identifierKey}::${alias}`;
        if (dedup.has(key)) continue;

        dedup.set(key, {
          identifier,
          identifierKey,
          packageName,
          alias,
          matchTokens: buildAliasMatchTokens(alias),
          packageHints,
        });
      }
    }

    return Array.from(dedup.values());
  }, [lipSelfVariantRelations]);

  const selfVariantCandidatesByAlias = useMemo(() => {
    const map = new Map<string, LIPAliasCandidate[]>();
    for (const candidate of selfVariantCandidates) {
      const list = map.get(candidate.alias) || [];
      list.push(candidate);
      map.set(candidate.alias, list);
    }
    return map;
  }, [selfVariantCandidates]);

  const modLIPStateByFolder = useMemo(() => {
    const map = new Map<string, ModLIPState>();

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

      const aliasesToCheck = [normalizeName(mod.name || ""), normalizeName(folder)]
        .filter(Boolean)
        .filter((value, index, arr) => arr.indexOf(value) === index);

      const aliasMatchesMap = new Map<string, LIPAliasCandidate>();
      for (const alias of aliasesToCheck) {
        const candidates = selfVariantCandidatesByAlias.get(alias) || [];
        for (const candidate of candidates) {
          aliasMatchesMap.set(candidate.identifierKey, candidate);
        }
      }

      if (aliasMatchesMap.size === 0) {
        const valuesToMatch = [normalizeMatchValue(mod.name || ""), normalizeMatchValue(folder)]
          .filter(Boolean)
          .filter((value, index, arr) => arr.indexOf(value) === index);

        for (const candidate of selfVariantCandidates) {
          if (!isSelfVariantCandidateFuzzyMatched(candidate, valuesToMatch)) {
            continue;
          }
          aliasMatchesMap.set(candidate.identifierKey, candidate);
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
        const matchedPackage = lipPackageByIdentifier[matched.identifierKey];
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
  }, [
    modsInfo,
    lipSourceLoaded,
    selfVariantCandidates,
    selfVariantCandidatesByAlias,
    lipPackagesByName,
    lipPackageByIdentifier,
  ]);

  const candidateLIPIdentifiers = useMemo<CandidateLIPIdentifier[]>(() => {
    const identifiers = new Map<string, string>();
    for (const state of modLIPStateByFolder.values()) {
      if (state.sourceType !== "unique") continue;
      const identifier = String(state.identifier || "").trim();
      const identifierKey = normalizeIdentifier(
        state.identifierKey || state.identifier,
      );
      if (!identifier || !identifierKey) continue;
      if (!identifiers.has(identifierKey)) {
        identifiers.set(identifierKey, identifier);
      }
    }
    return Array.from(identifiers.entries())
      .map(([identifierKey, identifier]) => ({ identifier, identifierKey }))
      .sort((a, b) =>
        a.identifierKey.localeCompare(b.identifierKey, undefined, {
          sensitivity: "base",
        }),
      );
  }, [modLIPStateByFolder]);

  useEffect(() => {
    let cancelled = false;

    const loadInstallState = async () => {
      const name = activeVersionName;
      if (!name) {
        setLipInstallStateByIdentifier(new Map());
        setInstallStateLoaded(false);
        return;
      }
      if (candidateLIPIdentifiers.length === 0) {
        setLipInstallStateByIdentifier(new Map());
        setInstallStateLoaded(true);
        return;
      }

      const snapshot = getInstanceSnapshot(name);
      if (snapshot?.lipInstallStateByIdentifier instanceof Map) {
        const hasAll = candidateLIPIdentifiers.every((item) =>
          snapshot.lipInstallStateByIdentifier.has(item.identifierKey),
        );
        if (hasAll) {
          setLipInstallStateByIdentifier(
            new Map(snapshot.lipInstallStateByIdentifier),
          );
          setInstallStateLoaded(true);
          return;
        }
      }

      setInstallStateLoaded(false);

      const entries = await Promise.all(
        candidateLIPIdentifiers.map(async (item) => {
          try {
            const sharedState = await ensurePackageInstallState(
              name,
              item.identifier,
            );
            return [
              item.identifierKey,
              {
                installed: sharedState.installed,
                explicitInstalled: sharedState.explicitInstalled,
                installedVersion: String(sharedState.installedVersion || "").trim(),
                error: String(sharedState.error || "").trim(),
              },
            ] as const;
          } catch (error) {
            return [
              item.identifierKey,
              {
                installed: false,
                explicitInstalled: false,
                installedVersion: "",
                error: parseErrorCode(error) || "ERR_LIP_PACKAGE_QUERY_FAILED",
              },
            ] as const;
          }
        }),
      );

      if (cancelled) return;
      setLipInstallStateByIdentifier(new Map(entries));
      setInstallStateLoaded(true);
    };

    void loadInstallState();
    return () => {
      cancelled = true;
    };
  }, [
    activeVersionName,
    candidateLIPIdentifiers,
    ensurePackageInstallState,
    getInstanceSnapshot,
  ]);

  const lipGroupItems = useMemo(() => {
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

      if (!grouped.has(state.identifierKey)) {
        grouped.set(state.identifierKey, {
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

      const current = grouped.get(state.identifierKey)!;
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

    const result: LipGroupItem[] = [];

    for (const group of grouped.values()) {
      const installState = lipInstallStateByIdentifier.get(group.identifierKey);
      const installedByLIP = Boolean(installState?.installed);
      const queryError = String(installState?.error || "").trim();
      const treatedAsLip =
        installedByLIP || (group.hasSelfVariantMapping && Boolean(queryError));
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
      const displayIdentifier = String(
        lipPackageByIdentifier[group.identifierKey]?.identifier || group.identifier,
      ).trim();

      result.push({
        kind: "lip",
        key: `lip:${group.identifierKey}`,
        identifier: group.identifier,
        identifierKey: group.identifierKey,
        displayIdentifier: displayIdentifier || group.identifier,
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

    result.sort((a, b) => {
      const nameA = String(a.packageName || "").toLowerCase();
      const nameB = String(b.packageName || "").toLowerCase();
      if (nameA < nameB) return sortConfig.direction === "asc" ? -1 : 1;
      if (nameA > nameB) return sortConfig.direction === "asc" ? 1 : -1;
      return 0;
    });

    return result;
  }, [
    modsInfo,
    modLIPStateByFolder,
    lipInstallStateByIdentifier,
    lipPackageByIdentifier,
    enabledByFolder,
    sortConfig.direction,
  ]);

  const lipGroupByIdentifier = useMemo(() => {
    const map = new Map<string, LipGroupItem>();
    for (const group of lipGroupItems) {
      map.set(group.identifierKey, group);
    }
    return map;
  }, [lipGroupItems]);

  const lipManagedFolderSet = useMemo(() => {
    const set = new Set<string>();
    for (const group of lipGroupItems) {
      for (const folder of group.folders) {
        set.add(folder);
      }
    }
    return set;
  }, [lipGroupItems]);

  const normalMods = useMemo(() => {
    return modsInfo.filter((mod) => {
      const folder = resolveModFolder(mod);
      return folder ? !lipManagedFolderSet.has(folder) : false;
    });
  }, [modsInfo, lipManagedFolderSet]);

  const filteredNormalMods = useMemo(() => {
    let list = normalMods;
    if (onlyEnabled) {
      list = list.filter((mod) => {
        const folder = resolveModFolder(mod);
        return folder ? Boolean(enabledByFolder.get(folder)) : false;
      });
    }

    const q = query.trim().toLowerCase();
    if (!q) return list;

    return list.filter(
      (mod) =>
        `${mod.name}`.toLowerCase().includes(q) ||
        `${mod.version}`.toLowerCase().includes(q),
    );
  }, [normalMods, onlyEnabled, enabledByFolder, query]);

  const sortedNormalMods = useMemo(() => {
    const items = [...filteredNormalMods];
    if (sortConfig.key === "name") {
      items.sort((a, b) => {
        const nameA = String(a.name || "").toLowerCase();
        const nameB = String(b.name || "").toLowerCase();
        if (nameA < nameB) return sortConfig.direction === "asc" ? -1 : 1;
        if (nameA > nameB) return sortConfig.direction === "asc" ? 1 : -1;
        return 0;
      });
    }
    return items;
  }, [filteredNormalMods, sortConfig]);

  const normalItems = useMemo<ModListItem[]>(() => {
    return sortedNormalMods.map((mod) => {
      const folder = resolveModFolder(mod);
      const resolvedState = modLIPStateByFolder.get(folder);
      const lipState =
        folder && lipManagedFolderSet.has(folder) && resolvedState
          ? resolvedState
          : buildDefaultModLIPState();
      return {
        kind: "mod",
        key: `mod:${folder}`,
        folder,
        mod,
        lipState,
      };
    });
  }, [sortedNormalMods, modLIPStateByFolder, lipManagedFolderSet]);

  const filteredLipItems = useMemo(() => {
    let list = lipGroupItems;
    if (onlyEnabled) {
      list = list.filter((item) => item.anyEnabled);
    }

    const q = query.trim().toLowerCase();
    if (!q) return list;

    return list.filter((item) => {
      const haystacks = [
        item.packageName,
        item.identifier,
        item.childLabels.join(" "),
      ];
      return haystacks.some((text) =>
        String(text || "").toLowerCase().includes(q),
      );
    });
  }, [lipGroupItems, onlyEnabled, query]);

  const visibleItems = useMemo<VisibleItem[]>(() => {
    if (!lipSourceLoaded) return [];
    if (tabKey === "normal") return normalItems;
    if (tabKey === "lip") return filteredLipItems;
    return [...normalItems, ...filteredLipItems];
  }, [lipSourceLoaded, tabKey, normalItems, filteredLipItems]);

  const currentSnapshot = useMemo(
    () => (activeVersionName ? getInstanceSnapshot(activeVersionName) : null),
    [activeVersionName, getInstanceSnapshot, snapshotRevision],
  );

  const listHydrating =
    Boolean(activeVersionName) &&
    (!lipSourceLoaded ||
      !installStateLoaded ||
      !currentSnapshot ||
      (currentSnapshot.status === "loading" && currentSnapshot.updatedAt <= 0));

  const visibleItemMap = useMemo(() => {
    const map = new Map<string, VisibleItem>();
    for (const item of visibleItems) {
      map.set(item.key, item);
    }
    return map;
  }, [visibleItems]);

  const selectedItems = useMemo(
    () =>
      Array.from(selectedKeys)
        .map((key) => visibleItemMap.get(key))
        .filter((item): item is VisibleItem => Boolean(item)),
    [selectedKeys, visibleItemMap],
  );

  const selectedMods = useMemo(
    () =>
      selectedItems
        .filter((item): item is ModListItem => item.kind === "mod")
        .map((item) => item.mod),
    [selectedItems],
  );

  const selectedUpdatableCount = useMemo(
    () => selectedItems.filter((item) => item.lipState.canUpdate).length,
    [selectedItems],
  );

  const activeLipGroup = useMemo(() => {
    if (!activeLipIdentifier) return null;
    return lipGroupByIdentifier.get(activeLipIdentifier) || null;
  }, [activeLipIdentifier, lipGroupByIdentifier]);

  const activeLipInstallState = useMemo(() => {
    if (!activeLipGroup) return null;
    return lipInstallStateByIdentifier.get(activeLipGroup.identifierKey) || null;
  }, [activeLipGroup, lipInstallStateByIdentifier]);

  const activeDeleteBlocked = useMemo(() => {
    if (!activeLipGroup || !activeLipInstallState) return false;
    return (
      Boolean(activeLipInstallState.installed) &&
      !Boolean(activeLipInstallState.explicitInstalled)
    );
  }, [activeLipGroup, activeLipInstallState]);

  const activeDeleteWarning = useMemo(() => {
    if (!activeDeleteBlocked) return "";
    return resolveErrorText(t, ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS);
  }, [activeDeleteBlocked, t]);

  useEffect(() => {
    setSelectedKeys(new Set());
  }, [tabKey]);

  useEffect(() => {
    setSelectedKeys((prev) => {
      const next = new Set<string>();
      for (const key of prev) {
        if (visibleItemMap.has(key)) next.add(key);
      }
      return next;
    });
  }, [visibleItemMap]);

  const refreshEnabledStates = async (name: string) => {
    const normalized = String(name || "").trim();
    if (!normalized) return;
    await refreshInstance(normalized, "mods-refresh-enabled");
  };

  const refreshModsAndStates = async (name: string) => {
    const normalized = String(name || "").trim();
    if (!normalized) return;
    await refreshInstance(normalized, "mods-refresh-mods-and-states");
  };

  const refreshAll = async () => {
    setLoading(true);
    const name = activeVersionName;
    if (name) {
      try {
        await refreshInstance(name, "mods-refresh-all");
      } catch {}
    }
    setLoading(false);
  };

  const setLipInstallStateCache = (
    identifierKey: string,
    state: LIPPackageInstallState,
  ) => {
    const normalizedKey = normalizeIdentifier(identifierKey);
    if (!normalizedKey) return;
    setLipInstallStateByIdentifier((prev) => {
      const next = new Map(prev);
      next.set(normalizedKey, state);
      return next;
    });
  };

  const queryLipGroupInstallState = async (
    group: LipGroupItem,
  ): Promise<LIPPackageInstallState> => {
    const name = activeVersionName;
    if (!name) {
      return {
        installed: false,
        explicitInstalled: false,
        installedVersion: "",
        error: "ERR_INVALID_NAME",
      };
    }

    try {
      const state = await ensurePackageInstallState(name, group.identifier);
      const normalizedState: LIPPackageInstallState = {
        installed: Boolean(state.installed),
        explicitInstalled: Boolean(state.explicitInstalled),
        installedVersion: String(state.installedVersion || "").trim(),
        error: String(state.error || "").trim(),
      };
      setLipInstallStateCache(group.identifierKey, normalizedState);
      return normalizedState;
    } catch (error) {
      return {
        installed: false,
        explicitInstalled: false,
        installedVersion: "",
        error: parseErrorCode(error) || "ERR_LIP_PACKAGE_QUERY_FAILED",
      };
    }
  };

  const resolvePromoteInstallVersion = (group: LipGroupItem): string => {
    const installState = lipInstallStateByIdentifier.get(group.identifierKey);
    const installedVersion = String(installState?.installedVersion || "").trim();
    if (installedVersion) return installedVersion;
    return String(group.installedVersion || "").trim();
  };

  const isLipGroupPromotable = (group: LipGroupItem): boolean => {
    const installState = lipInstallStateByIdentifier.get(group.identifierKey);
    if (!installState) {
      return Boolean(group.installedVersion) && !group.explicitInstalled;
    }
    return Boolean(installState.installed) && !installState.explicitInstalled;
  };

  const showActionToast = (
    type: "success" | "error",
    actionLabel: string,
    name: string,
    err?: unknown,
  ) => {
    if (type === "success") {
      addToast({
        color: "success",
        title: t("common.success"),
        description: `${actionLabel}: ${name}`,
      });
      return;
    }

    addToast({
      color: "danger",
      title: t("common.error"),
      description: `${actionLabel}: ${resolveErrorText(t, err)}`,
    });
  };

  const showActionResult = (
    success: string[],
    failed: Array<{ name: string; err: string }>,
  ) => {
    setResultSuccess(success);
    setResultFailed(failed);
    if (success.length > 0 || failed.length > 0) {
      delOnOpen();
    }
  };

  const openDemotedWarning = (names: string[]) => {
    const uniqueNames = Array.from(
      new Set(names.map((item) => String(item || "").trim()).filter(Boolean)),
    );
    if (uniqueNames.length === 0) return;
    setDemotedWarningNames(uniqueNames);
    demotedWarningOnOpen();
  };

  const closeDemotedWarning = () => {
    setDemotedWarningNames([]);
    demotedWarningOnClose();
  };

  const demotedWarningOnOpenChange = (nextOpen: boolean) => {
    if (nextOpen) {
      demotedWarningOnOpen();
      return;
    }
    closeDemotedWarning();
  };

  const doImportFromPaths = async (paths: string[]) => {
    try {
      if (!paths?.length) return;
      const name = activeVersionName;
      if (!name) {
        setErrorMsg(t("launcherpage.currentVersion_none") as string);
        return;
      }
      let started = false;
      const succFiles: string[] = [];
      const errPairs: Array<{ name: string; err: string }> = [];
      for (const p of paths) {
        const lower = p.toLowerCase();
        if (lower.endsWith(".zip")) {
          if (!started) {
            setImporting(true);
            started = true;
          }
          const base = p.replace(/\\/g, "/").split("/").pop() || p;
          setCurrentFile(base);
          let err = await ImportModZipPath(name, p, false);
          if (err) {
            if (String(err) === "ERR_DUPLICATE_FOLDER") {
              dupNameRef.current = base;
              await new Promise<void>((r) => setTimeout(r, 0));
              dupOnOpen();
              const ok = await new Promise<boolean>((resolve) => {
                dupResolveRef.current = resolve;
              });
              if (ok) {
                err = await ImportModZipPath(name, p, true);
                if (!err) {
                  succFiles.push(base);
                  continue;
                }
              } else {
                continue;
              }
            }
            errPairs.push({ name: base, err });
            continue;
          }
          succFiles.push(base);
        } else if (lower.endsWith(".dll")) {
          const base = p.replace(/\\/g, "/").split("/").pop() || "";
          const baseNoExt = base.replace(/\.[^/.]+$/, "");
          setDllName(baseNoExt);
          setDllType("preload-native");
          setDllVersion("0.0.0");
          await new Promise<void>((resolve) => setTimeout(resolve, 0));
          dllOnOpen();
          const ok = await new Promise<boolean>((resolve) => {
            dllResolveRef.current = resolve;
          });
          if (!ok) {
            continue;
          }
          if (!started) {
            setImporting(true);
            started = true;
          }
          setCurrentFile(base || p);
          const vals = dllConfirmRef.current || {
            name: dllName,
            type: dllType,
            version: dllVersion,
          };
          dllConfirmRef.current = null;
          let err = await ImportModDllPath(
            name,
            p,
            vals.name,
            vals.type || "preload-native",
            vals.version || "0.0.0",
            false,
          );
          if (err) {
            if (String(err) === "ERR_DUPLICATE_FOLDER") {
              dupNameRef.current = base || p;
              await new Promise<void>((r) => setTimeout(r, 0));
              dupOnOpen();
              const overwrite = await new Promise<boolean>((resolve) => {
                dupResolveRef.current = resolve;
              });
              if (overwrite) {
                err = await ImportModDllPath(
                  name,
                  p,
                  vals.name,
                  vals.type || "preload-native",
                  vals.version || "0.0.0",
                  true,
                );
                if (!err) {
                  succFiles.push(base || p);
                  continue;
                }
              } else {
                continue;
              }
            }
            errPairs.push({ name: base || p, err });
            continue;
          }
          succFiles.push(base || p);
        }
      }
      await refreshInstance(name, "mods-import-complete");
      setResultSuccess(succFiles);
      setResultFailed(errPairs);
      if (succFiles.length > 0 || errPairs.length > 0) {
        errOnOpen();
      }
    } catch (e: any) {
      setErrorMsg(String(e?.message || e || "IMPORT_ERROR"));
    } finally {
      setImporting(false);
      setCurrentFile("");
    }
  };

  const doImportRef = useRef(doImportFromPaths);
  doImportRef.current = doImportFromPaths;

  useEffect(() => {
    if (!activeVersionName) {
      navigate(ROUTES.instances, { replace: true });
      return;
    }
    const snapshot = getInstanceSnapshot(activeVersionName);
    if (!snapshot) {
      setInstallStateLoaded(false);
      return;
    }
    setInstallStateLoaded(snapshot.status !== "loading");
  }, [activeVersionName, getInstanceSnapshot, navigate, snapshotRevision]);

  useEffect(() => {
    return Events.On("files-dropped", (event) => {
      const data = (event.data as { files: string[] }) || {};
      if (data.files && data.files.length > 0) {
        void doImportRef.current(data.files);
      }
    });
  }, []);

  useEffect(() => {
    if (!restorePendingRef.current) return;
    requestAnimationFrame(() => {
      try {
        if (scrollRef.current)
          scrollRef.current.scrollTop = lastScrollTopRef.current;
      } catch {}
    });
    restorePendingRef.current = false;
  }, [modsInfo, scrollRef]);

  const filtered = filteredNormalMods;
  const sortedItems = sortedNormalMods;

  const handleSort = (key: string) => {
    let direction: "asc" | "desc" = "asc";
    if (sortConfig.key === key && sortConfig.direction === "asc") {
      direction = "desc";
    }
    setSortConfig({ key, direction });
  };

  const onSelectionChange = (itemKey: string) => {
    setSelectedKeys((prev) => {
      const next = new Set(prev);
      if (next.has(itemKey)) next.delete(itemKey);
      else next.add(itemKey);
      return next;
    });
  };

  const onSelectAll = (checked: boolean) => {
    if (checked) {
      setSelectedKeys(new Set(visibleItems.map((item) => item.key)));
      return;
    }
    setSelectedKeys(new Set());
  };

  const installLIPIdentifier = async (
    identifier: string,
    identifierKey: string,
    targetVersion: string,
    onLog?: TaskLogger,
  ): Promise<string> => {
    const name = activeVersionName;
    const requestIdentifier = resolveLIPIdentifierForRequest(
      identifier,
      identifierKey,
    );
    if (!name) return "ERR_INVALID_NAME";
    if (!requestIdentifier || !targetVersion) return ERR_NO_UPDATE_SOURCE;

    try {
      onLog?.("info", t("lip.task_console.stage_install_pkg"));
      const err = await callMinecraftByName<string>(
        "InstallLIPPackage",
        name,
        requestIdentifier,
        targetVersion,
      );
      return String(err || "").trim();
    } catch (error) {
      return parseErrorCode(error) || "ERR_LIP_PACKAGE_INSTALL_FAILED";
    }
  };

  const installModByLIP = async (
    mod: types.ModInfo,
    onLog?: TaskLogger,
  ): Promise<string> => {
    if (isLeviLaminaMod(mod)) {
      return ERR_LL_MANAGED_IN_VERSION_SETTINGS;
    }

    const folder = resolveModFolder(mod);
    const state = modLIPStateByFolder.get(folder);
    if (!state || state.sourceType !== "unique") {
      return ERR_NO_UPDATE_SOURCE;
    }
    if (!state.targetVersion) {
      return ERR_NO_UPDATE_SOURCE;
    }
    if (!state.canUpdate) {
      return ERR_ALREADY_LATEST;
    }

    return installLIPIdentifier(
      state.identifier,
      state.identifierKey,
      state.targetVersion,
      onLog,
    );
  };

  const installLipGroupByLIP = async (
    group: LipGroupItem,
    onLog?: TaskLogger,
  ): Promise<string> => {
    const state = group.lipState;
    if (state.sourceType !== "unique") return ERR_NO_UPDATE_SOURCE;
    if (!state.targetVersion) return ERR_NO_UPDATE_SOURCE;
    if (!state.canUpdate) return ERR_ALREADY_LATEST;
    return installLIPIdentifier(
      state.identifier,
      state.identifierKey,
      state.targetVersion,
      onLog,
    );
  };

  const uninstallModWithFallback = async (
    mod: types.ModInfo,
    onLog?: TaskLogger,
  ): Promise<string> => {
    const name = activeVersionName;
    if (!name) return "ERR_INVALID_NAME";

    if (isLeviLaminaMod(mod)) {
      return ERR_LL_MANAGED_IN_VERSION_SETTINGS;
    }

    const folder = resolveModFolder(mod);
    if (!folder) return "ERR_INVALID_PACKAGE";

    const state = modLIPStateByFolder.get(folder);
    const isLipManagedMod = lipManagedFolderSet.has(folder);
    if (isLipManagedMod && state?.sourceType === "unique" && state.identifier) {
      const requestIdentifier = resolveLIPIdentifierForRequest(
        state.identifier,
        state.identifierKey,
      );
      if (!requestIdentifier) return "ERR_LIP_PACKAGE_INVALID_IDENTIFIER";
      try {
        onLog?.("info", t("mods.action_uninstall"));
        const uninstallErr = await callMinecraftByName<string>(
          "UninstallLIPPackage",
          name,
          requestIdentifier,
        );
        return String(uninstallErr || "").trim();
      } catch (error) {
        return parseErrorCode(error) || "ERR_LIP_PACKAGE_UNINSTALL_FAILED";
      }
    }

    try {
      onLog?.("warning", t("lip.task_console.stage_fallback_delete"));
      const deleteErr = await (DeleteMod as any)?.(name, folder);
      return String(deleteErr || "").trim();
    } catch (error) {
      return parseErrorCode(error) || "ERR_WRITE_FILE";
    }
  };

  const uninstallLipGroupWithFallback = async (
    group: LipGroupItem,
    onLog?: TaskLogger,
  ): Promise<string> => {
    const name = activeVersionName;
    if (!name) return "ERR_INVALID_NAME";
    const requestIdentifier = resolveLIPIdentifierForRequest(
      group.identifier,
      group.identifierKey,
    );
    if (!requestIdentifier) return "ERR_LIP_PACKAGE_INVALID_IDENTIFIER";

    try {
      onLog?.("info", t("mods.action_uninstall"));
      const uninstallErr = await callMinecraftByName<string>(
        "UninstallLIPPackage",
        name,
        requestIdentifier,
      );
      return String(uninstallErr || "").trim();
    } catch (error) {
      return parseErrorCode(error) || "ERR_LIP_PACKAGE_UNINSTALL_FAILED";
    }
  };

  const resetActionConfirm = () => {
    actionConfirmRunnerRef.current = null;
    setActionConfirmTitle("");
    setActionConfirmBody("");
  };

  const openInstallActionConfirm = (
    packageName: string,
    version: string,
    actionLabel: string,
    runner: () => Promise<void>,
  ) => {
    const name = activeVersionName;
    if (!name) {
      addToast({
        color: "danger",
        title: t("common.error"),
        description: t("launcherpage.currentVersion_none"),
      });
      return;
    }

    actionConfirmRunnerRef.current = runner;
    setActionConfirmTitle(t("lip.package.confirm_install_title"));
    setActionConfirmBody(
      t("lip.package.confirm_install_body", {
        package: packageName,
        version: String(version || "").trim() || "-",
        instance: name,
        action: actionLabel,
      }),
    );
    actionConfirmOnOpen();
  };

  const confirmPendingAction = async () => {
    const runner = actionConfirmRunnerRef.current;
    if (!runner) {
      actionConfirmOnClose();
      resetActionConfirm();
      return;
    }

    setActionConfirming(true);
    try {
      await runner();
      actionConfirmOnClose();
      resetActionConfirm();
    } finally {
      setActionConfirming(false);
    }
  };

  const handleActionConfirmOpenChange = (nextOpen: boolean) => {
    if (!nextOpen && actionConfirming) return;
    if (nextOpen) {
      actionConfirmOnOpen();
      return;
    }
    actionConfirmOnClose();
    resetActionConfirm();
  };

  const closeActionConfirm = () => {
    if (actionConfirming) return;
    actionConfirmOnClose();
    resetActionConfirm();
  };

  const handleBatchEnable = async () => {
    const name = activeVersionName;
    if (!name) return;

    const folders = new Set<string>();
    for (const item of selectedItems) {
      if (item.kind === "mod") {
        folders.add(item.folder);
      } else {
        for (const folder of item.folders) folders.add(folder);
      }
    }

    for (const folder of folders) {
      try {
        await (EnableMod as any)?.(name, folder);
      } catch {}
    }
    await refreshEnabledStates(name);
  };

  const handleBatchDisable = async () => {
    const name = activeVersionName;
    if (!name) return;

    const folders = new Set<string>();
    for (const item of selectedItems) {
      if (item.kind === "mod") {
        folders.add(item.folder);
      } else {
        for (const folder of item.folders) folders.add(folder);
      }
    }

    for (const folder of folders) {
      try {
        await (DisableMod as any)?.(name, folder);
      } catch {}
    }
    await refreshEnabledStates(name);
  };

  const openBatchUpdateConfirm = () => {
    if (selectedItems.length === 0) return;
    batchUpdateOnOpen();
  };

  const openBatchUninstallConfirm = () => {
    if (selectedItems.length === 0) return;
    batchUninstallOnOpen();
  };

  const handleBatchUpdate = async () => {
    const name = activeVersionName;
    if (!name) {
      addToast({
        color: "danger",
        title: t("common.error"),
        description: t("launcherpage.currentVersion_none"),
      });
      return;
    }

    const updatableItems = selectedItems.filter(
      (item) => item.lipState.sourceType === "unique" && item.lipState.canUpdate,
    );
    if (updatableItems.length === 0) {
      batchUpdateOnClose();
      return;
    }

    setBatchUpdating(true);
    const success: string[] = [];
    const failed: Array<{ name: string; err: string }> = [];
    const handledIdentifiers = new Set<string>();
    let hasSuccess = false;

    try {
      try {
        await runWithLipTask(
          {
            action: "update",
            target: name,
            methods: ["Install", "Update"],
            feedbackMode: "never",
          },
          async ({ addLog }) => {
            for (const item of updatableItems) {
              if (item.kind === "mod") {
                const identifierKey = item.lipState.identifierKey;
                if (identifierKey && handledIdentifiers.has(identifierKey)) continue;

                addLog("info", `${t("mods.action_update")}: ${item.mod.name}`);
                const err = await installModByLIP(item.mod, addLog);
                if (err) {
                  addLog("error", `${item.mod.name}: ${err}`);
                  failed.push({ name: item.mod.name, err });
                  continue;
                }
                if (identifierKey) handledIdentifiers.add(identifierKey);
                hasSuccess = true;
                success.push(item.mod.name);
                addLog("success", `${item.mod.name}: ok`);
                continue;
              }

              if (item.identifierKey && handledIdentifiers.has(item.identifierKey)) {
                continue;
              }

              addLog("info", `${t("mods.action_update")}: ${item.packageName}`);
              const err = await installLipGroupByLIP(item, addLog);
              if (err) {
                addLog("error", `${item.packageName}: ${err}`);
                failed.push({ name: item.packageName, err });
                continue;
              }
              handledIdentifiers.add(item.identifierKey);
              hasSuccess = true;
              success.push(item.packageName);
              addLog("success", `${item.packageName}: ok`);
            }

            if (failed.length > 0) {
              throw new Error(failed[0].err || "ERR_LIP_PACKAGE_INSTALL_FAILED");
            }
          },
        );
      } catch {}

      if (hasSuccess) await refreshAll();
      setSelectedKeys(new Set());
      batchUpdateOnClose();
      if (failed.length > 0) {
        showActionResult(success, failed);
      } else if (hasSuccess) {
        addToast({
          color: "success",
          title: t("common.success"),
          description: `${t("mods.action_update")} x${success.length}`,
        });
      }
    } finally {
      setBatchUpdating(false);
    }
  };

  const handleBatchUninstall = async () => {
    const name = activeVersionName;
    if (!name) {
      addToast({
        color: "danger",
        title: t("common.error"),
        description: t("launcherpage.currentVersion_none"),
      });
      return;
    }
    if (selectedItems.length === 0) return;

    setBatchUninstalling(true);
    const success: string[] = [];
    const failed: Array<{ name: string; err: string }> = [];
    const demoted: string[] = [];
    const handledIdentifiers = new Set<string>();
    let hasStateChanged = false;

    try {
      try {
        await runWithLipTask(
          {
            action: "uninstall",
            target: name,
            methods: ["Uninstall"],
            feedbackMode: "never",
          },
          async ({ addLog }) => {
            for (const item of selectedItems) {
              if (item.kind === "mod") {
                addLog("info", `${t("mods.action_uninstall")}: ${item.mod.name}`);
                const err = await uninstallModWithFallback(item.mod, addLog);
                if (err) {
                  if (err === ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY) {
                    addLog("warning", `${item.mod.name}: ${err}`);
                    demoted.push(item.mod.name);
                    hasStateChanged = true;
                  } else {
                    addLog("error", `${item.mod.name}: ${err}`);
                    failed.push({ name: item.mod.name, err });
                  }
                  continue;
                }
                hasStateChanged = true;
                success.push(item.mod.name);
                addLog("success", `${item.mod.name}: ok`);
                continue;
              }

              if (item.identifierKey && handledIdentifiers.has(item.identifierKey)) {
                continue;
              }

              const latestState = await queryLipGroupInstallState(item);
              if (latestState.installed && !latestState.explicitInstalled) {
                const blockedErr = ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS;
                addLog("warning", `${item.packageName}: ${blockedErr}`);
                failed.push({ name: item.packageName, err: blockedErr });
                continue;
              }

              addLog("info", `${t("mods.action_uninstall")}: ${item.packageName}`);
              const err = await uninstallLipGroupWithFallback(item, addLog);
              if (err) {
                if (err === ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY) {
                  addLog("warning", `${item.packageName}: ${err}`);
                  demoted.push(item.packageName);
                  hasStateChanged = true;
                } else {
                  addLog("error", `${item.packageName}: ${err}`);
                  failed.push({ name: item.packageName, err });
                }
                continue;
              }

              handledIdentifiers.add(item.identifierKey);
              hasStateChanged = true;
              success.push(item.packageName);
              addLog("success", `${item.packageName}: ok`);
            }

            if (failed.length > 0) {
              throw new Error(failed[0].err || "ERR_LIP_PACKAGE_UNINSTALL_FAILED");
            }
          },
        );
      } catch {}

      if (hasStateChanged) await refreshAll();
      setSelectedKeys(new Set());
      batchUninstallOnClose();
      if (failed.length > 0) {
        showActionResult(success, failed);
      } else if (success.length > 0) {
        addToast({
          color: "success",
          title: t("common.success"),
          description: `${t("mods.action_uninstall")} x${success.length}`,
        });
      }
      if (demoted.length > 0) {
        openDemotedWarning(demoted);
      }
    } finally {
      setBatchUninstalling(false);
    }
  };

  const openDetails = (mod: types.ModInfo) => {
    setActiveLipIdentifier("");
    setActiveMod(mod);
    infoOnOpen();
  };

  const openLIPPackageDetails = (identifier: string) => {
    const target = String(identifier || "").trim();
    if (!target) return;
    navigate(`/lip/package/${encodeURIComponent(target)}`);
  };

  const openDeleteForMod = (mod: types.ModInfo) => {
    setActiveLipIdentifier("");
    setActiveMod(mod);
    delCfmOnOpen();
  };

  const openDeleteForLipGroup = async (group: LipGroupItem) => {
    setActiveMod(null);
    setActiveLipIdentifier(group.identifierKey);
    try {
      await queryLipGroupInstallState(group);
    } catch {}
    delCfmOnOpen();
  };

  const executeUpdateMod = async (mod: types.ModInfo) => {
    const name = activeVersionName;
    if (!name) {
      addToast({
        color: "danger",
        title: t("common.error"),
        description: t("launcherpage.currentVersion_none"),
      });
      return;
    }

    const pos = scrollRef.current?.scrollTop || 0;
    lastScrollTopRef.current = pos;
    restorePendingRef.current = true;

    try {
      await runWithLipTask(
        {
          action: "update",
          target: name,
          methods: ["Install", "Update"],
          feedbackMode: "on_error",
        },
        async ({ addLog }) => {
          addLog("info", `${t("mods.action_update")}: ${mod.name}`);
          const err = await installModByLIP(mod, addLog);
          if (err) throw new Error(err);
        },
      );
    } catch (error) {
      showActionToast(
        "error",
        t("mods.action_update"),
        mod.name,
        parseErrorCode(error) || "ERR_LIP_PACKAGE_INSTALL_FAILED",
      );
      return;
    }

    await refreshAll();
    showActionToast("success", t("mods.action_update"), mod.name);
  };

  const handleUpdateMod = async (mod: types.ModInfo) => {
    const lipState = getModLIPState(mod);
    if (lipState.sourceType !== "unique") {
      showActionToast(
        "error",
        t("mods.action_update"),
        mod.name,
        ERR_NO_UPDATE_SOURCE,
      );
      return;
    }
    if (!lipState.canUpdate) {
      showActionToast(
        "error",
        t("mods.action_update"),
        mod.name,
        ERR_ALREADY_LATEST,
      );
      return;
    }

    openInstallActionConfirm(
      mod.name,
      lipState.targetVersion || mod.version || "-",
      t("mods.action_update"),
      async () => {
        await executeUpdateMod(mod);
      },
    );
  };

  const executeUpdateLipGroup = async (group: LipGroupItem) => {
    const name = activeVersionName;
    if (!name) {
      addToast({
        color: "danger",
        title: t("common.error"),
        description: t("launcherpage.currentVersion_none"),
      });
      return;
    }

    const pos = scrollRef.current?.scrollTop || 0;
    lastScrollTopRef.current = pos;
    restorePendingRef.current = true;

    try {
      await runWithLipTask(
        {
          action: "update",
          target: name,
          methods: ["Install", "Update"],
          feedbackMode: "on_error",
        },
        async ({ addLog }) => {
          addLog("info", `${t("mods.action_update")}: ${group.packageName}`);
          const err = await installLipGroupByLIP(group, addLog);
          if (err) throw new Error(err);
        },
      );
    } catch (error) {
      showActionToast(
        "error",
        t("mods.action_update"),
        group.packageName,
        parseErrorCode(error) || "ERR_LIP_PACKAGE_INSTALL_FAILED",
      );
      return;
    }

    await refreshAll();
    showActionToast("success", t("mods.action_update"), group.packageName);
  };

  const handleUpdateLipGroup = async (group: LipGroupItem) => {
    if (group.lipState.sourceType !== "unique") {
      showActionToast(
        "error",
        t("mods.action_update"),
        group.packageName,
        ERR_NO_UPDATE_SOURCE,
      );
      return;
    }
    if (!group.lipState.canUpdate) {
      showActionToast(
        "error",
        t("mods.action_update"),
        group.packageName,
        ERR_ALREADY_LATEST,
      );
      return;
    }

    openInstallActionConfirm(
      group.packageName,
      group.lipState.targetVersion || group.installedVersion || "-",
      t("mods.action_update"),
      async () => {
        await executeUpdateLipGroup(group);
      },
    );
  };

  const executePromoteLipGroup = async (
    group: LipGroupItem,
    requestIdentifier: string,
    version: string,
  ) => {
    const name = activeVersionName;
    if (!name) {
      addToast({
        color: "danger",
        title: t("common.error"),
        description: t("launcherpage.currentVersion_none"),
      });
      return;
    }

    try {
      await runWithLipTask(
        {
          action: "install",
          target: name,
          methods: ["Install"],
          feedbackMode: "on_error",
        },
        async ({ addLog }) => {
          addLog(
            "info",
            `${t("mods.action_promote_install")}: ${group.packageName}@${version}`,
          );
          const err = await callMinecraftByName<string>(
            "InstallLIPPackage",
            name,
            requestIdentifier,
            version,
          );
          if (err) throw new Error(String(err));
        },
      );
    } catch (error) {
      showActionToast(
        "error",
        t("mods.action_promote_install"),
        group.packageName,
        parseErrorCode(error) || "ERR_LIP_PACKAGE_INSTALL_FAILED",
      );
      return;
    }

    await refreshAll();
    addToast({
      color: "success",
      title: t("common.success"),
      description: t("mods.action_promote_install_success", {
        name: group.packageName,
      }),
    });
  };

  const handlePromoteLipGroup = async (group: LipGroupItem) => {
    const name = activeVersionName;
    if (!name) {
      addToast({
        color: "danger",
        title: t("common.error"),
        description: t("launcherpage.currentVersion_none"),
      });
      return;
    }

    const requestIdentifier = resolveLIPIdentifierForRequest(
      group.identifier,
      group.identifierKey,
    );
    if (!requestIdentifier) {
      showActionToast(
        "error",
        t("mods.action_promote_install"),
        group.packageName,
        "ERR_LIP_PACKAGE_INVALID_IDENTIFIER",
      );
      return;
    }

    const installState = await queryLipGroupInstallState(group);
    if (installState.error) {
      showActionToast(
        "error",
        t("mods.action_promote_install"),
        group.packageName,
        installState.error,
      );
      return;
    }
    const version = resolvePromoteInstallVersion({
      ...group,
      installedVersion: installState.installedVersion || group.installedVersion,
    });
    if (!version) {
      showActionToast(
        "error",
        t("mods.action_promote_install"),
        group.packageName,
        "mods.action_promote_install_failed_no_version",
      );
      return;
    }

    openInstallActionConfirm(
      group.packageName,
      version,
      t("mods.action_promote_install"),
      async () => {
        await executePromoteLipGroup(group, requestIdentifier, version);
      },
    );
  };

  const handleDeleteMod = async () => {
    if (!activeMod) return;
    const name = activeVersionName;
    if (!name) {
      addToast({
        color: "danger",
        title: t("common.error"),
        description: t("launcherpage.currentVersion_none"),
      });
      return;
    }

    const pos = scrollRef.current?.scrollTop || 0;
    setDeleting(true);
    lastScrollTopRef.current = pos;
    restorePendingRef.current = true;
    let demotedToDependency = false;

    try {
      await runWithLipTask(
        {
          action: "uninstall",
          target: name,
          methods: ["Uninstall"],
          feedbackMode: "on_error",
        },
        async ({ addLog }) => {
          addLog("info", `${t("mods.action_uninstall")}: ${activeMod.name}`);
          const err = await uninstallModWithFallback(activeMod, addLog);
          if (err === ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY) {
            demotedToDependency = true;
            addLog("warning", `${activeMod.name}: ${err}`);
            return;
          }
          if (err) throw new Error(err);
        },
      );
    } catch (error) {
      showActionToast(
        "error",
        t("mods.action_uninstall"),
        activeMod.name,
        parseErrorCode(error) || "ERR_LIP_PACKAGE_UNINSTALL_FAILED",
      );
      setDeleting(false);
      return;
    }

    await refreshAll();
    requestAnimationFrame(() => {
      try {
        if (scrollRef.current) scrollRef.current.scrollTop = pos;
      } catch {}
    });
    if (demotedToDependency) {
      openDemotedWarning([activeMod.name]);
      infoOnClose();
      setDeleting(false);
      return;
    }
    showActionToast("success", t("mods.action_uninstall"), activeMod.name);
    infoOnClose();
    setDeleting(false);
  };

  const handleDeleteLipGroup = async () => {
    if (!activeLipGroup) return;

    const name = activeVersionName;
    if (!name) {
      addToast({
        color: "danger",
        title: t("common.error"),
        description: t("launcherpage.currentVersion_none"),
      });
      return;
    }

    const latestState = await queryLipGroupInstallState(activeLipGroup);
    if (latestState.installed && !latestState.explicitInstalled) {
      showActionToast(
        "error",
        t("mods.action_uninstall"),
        activeLipGroup.packageName,
        ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS,
      );
      return;
    }

    const pos = scrollRef.current?.scrollTop || 0;
    setDeleting(true);
    lastScrollTopRef.current = pos;
    restorePendingRef.current = true;
    let demotedToDependency = false;

    try {
      await runWithLipTask(
        {
          action: "uninstall",
          target: name,
          methods: ["Uninstall"],
          feedbackMode: "on_error",
        },
        async ({ addLog }) => {
          addLog(
            "info",
            `${t("mods.action_uninstall")}: ${activeLipGroup.packageName}`,
          );
          const err = await uninstallLipGroupWithFallback(activeLipGroup, addLog);
          if (err === ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY) {
            demotedToDependency = true;
            addLog("warning", `${activeLipGroup.packageName}: ${err}`);
            return;
          }
          if (err) throw new Error(err);
        },
      );
    } catch (error) {
      showActionToast(
        "error",
        t("mods.action_uninstall"),
        activeLipGroup.packageName,
        parseErrorCode(error) || "ERR_LIP_PACKAGE_UNINSTALL_FAILED",
      );
      setDeleting(false);
      return;
    }

    await refreshAll();
    requestAnimationFrame(() => {
      try {
        if (scrollRef.current) scrollRef.current.scrollTop = pos;
      } catch {}
    });
    if (demotedToDependency) {
      openDemotedWarning([activeLipGroup.packageName]);
      setDeleting(false);
      return;
    }
    showActionToast(
      "success",
      t("mods.action_uninstall"),
      activeLipGroup.packageName,
    );
    setDeleting(false);
  };

  const handleDeleteCurrentTarget = async () => {
    if (activeLipIdentifier) {
      if (activeDeleteBlocked && activeLipGroup) {
        showActionToast(
          "error",
          t("mods.action_uninstall"),
          activeLipGroup.packageName,
          ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS,
        );
        return;
      }
      await handleDeleteLipGroup();
      return;
    }
    await handleDeleteMod();
  };

  const openFolder = () => {
    const name = activeVersionName;
    if (!name) {
      navigate(ROUTES.instances);
      return;
    }
    OpenModsExplorer(name);
  };

  const openModFolder = async (mod: types.ModInfo) => {
    const name = activeVersionName;
    if (!name) return;

    const folder = resolveModFolder(mod);
    if (!folder) return;

    try {
      const versionsDir = await GetVersionsDir();
      const sep = versionsDir.includes("\\") ? "\\" : "/";
      const modPath = `${versionsDir}${sep}${name}${sep}mods${sep}${folder}`;

      try {
        await ListDir(modPath);
        await OpenPathDir(modPath);
      } catch {
        OpenModsExplorer(name);
      }
    } catch {
      OpenModsExplorer(name);
    }
  };

  const toggleModEnabled = async (modFolder: string, val: boolean) => {
    const name = activeVersionName;
    if (!name || !modFolder) return;
    try {
      if (val) {
        const err = await (EnableMod as any)?.(name, modFolder);
        if (err) return;
      } else {
        const err = await (DisableMod as any)?.(name, modFolder);
        if (err) return;
      }
      await refreshInstance(name, val ? "mods-enable" : "mods-disable");
    } catch {}
  };

  const toggleLipGroupEnabled = async (group: LipGroupItem, val: boolean) => {
    const name = activeVersionName;
    if (!name) return;

    for (const folder of group.folders) {
      try {
        if (val) {
          await (EnableMod as any)?.(name, folder);
        } else {
          await (DisableMod as any)?.(name, folder);
        }
      } catch {}
    }
    await refreshInstance(name, val ? "mods-group-enable" : "mods-group-disable");
  };

  const getModLIPState = (mod: types.ModInfo): ModLIPState => {
    const folder = resolveModFolder(mod);
    if (!folder || !lipManagedFolderSet.has(folder)) {
      return buildDefaultModLIPState();
    }
    return modLIPStateByFolder.get(folder) || buildDefaultModLIPState();
  };

  const activeDeleteName = activeLipGroup
    ? activeLipGroup.packageName
    : activeMod?.name || "";

  const activeDeleteKind: "mod" | "lip" | "none" = activeLipGroup
    ? "lip"
    : activeMod
      ? "mod"
      : "none";

  return {
    currentVersionName,
    modsInfo,
    query,
    setQuery,
    loading,
    importing,
    errorMsg,
    setErrorMsg,
    errorFile,
    setErrorFile,
    resultSuccess,
    setResultSuccess,
    resultFailed,
    setResultFailed,
    currentFile,
    activeMod,
    setActiveMod,
    activeLipIdentifier,
    setActiveLipIdentifier,
    activeLipGroup,
    activeDeleteName,
    activeDeleteKind,
    activeDeleteBlocked,
    activeDeleteWarning,
    deleting,
    batchUpdating,
    batchUninstalling,
    demotedWarningNames,
    actionConfirmTitle,
    actionConfirmBody,
    actionConfirming,
    enabledByFolder,
    onlyEnabled,
    setOnlyEnabled,
    dllName,
    setDllName,
    dllType,
    setDllType,
    dllVersion,
    setDllVersion,
    selectedKeys,
    sortConfig,
    tabKey,
    setTabKey,
    gameVersion,
    lipSourceLoaded,
    listHydrating,
    selectedMods,
    selectedItems,
    selectedUpdatableCount,

    dllResolveRef,
    dllConfirmRef,
    dupResolveRef,
    dupNameRef,
    lastScrollTopRef,
    restorePendingRef,

    errOpen,
    errOnOpen,
    errOnClose,
    errOnOpenChange,
    delOpen,
    delOnOpen,
    delOnClose,
    delOnOpenChange,
    dllOpen,
    dllOnOpen,
    dllOnClose,
    dllOnOpenChange,
    dupOpen,
    dupOnOpen,
    dupOnClose,
    dupOnOpenChange,
    delCfmOpen,
    delCfmOnOpen,
    delCfmOnClose,
    delCfmOnOpenChange,
    infoOpen,
    infoOnOpen,
    infoOnClose,
    infoOnOpenChange,
    batchUpdateOpen,
    batchUpdateOnOpen,
    batchUpdateOnClose,
    batchUpdateOnOpenChange,
    batchUninstallOpen,
    batchUninstallOnOpen,
    batchUninstallOnClose,
    batchUninstallOnOpenChange,
    demotedWarningOpen,
    demotedWarningOnOpenChange,
    closeDemotedWarning,
    actionConfirmOpen,
    actionConfirmOnClose: closeActionConfirm,
    actionConfirmOnOpenChange: handleActionConfirmOpenChange,

    ERR_LL_MANAGED_IN_VERSION_SETTINGS,
    ERR_NO_UPDATE_SOURCE,
    ERR_ALREADY_LATEST,

    filtered,
    sortedItems,
    normalItems,
    lipGroupItems,
    visibleItems,
    modsByFolder,

    refreshAll,
    refreshEnabledStates,
    refreshModsAndStates,
    doImportFromPaths,
    handleSort,
    onSelectionChange,
    onSelectAll,
    handleBatchEnable,
    handleBatchDisable,
    openBatchUpdateConfirm,
    openBatchUninstallConfirm,
    handleBatchUpdate,
    handleBatchUninstall,
    openDetails,
    openLIPPackageDetails,
    openDeleteForMod,
    openDeleteForLipGroup,
    handleUpdateMod,
    handleUpdateLipGroup,
    handlePromoteLipGroup,
    confirmPendingAction,
    handleDeleteMod,
    handleDeleteLipGroup,
    handleDeleteCurrentTarget,
    openFolder,
    openModFolder,
    toggleModEnabled,
    toggleLipGroupEnabled,
    getModLIPState,
    resolveModFolder,
    isLeviLaminaMod,
    isLipGroupPromotable,

    navigate,
  };
};


