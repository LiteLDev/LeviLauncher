import { openDirectory, openModsDirectory } from "@/utils/explorer";
import { useOverlayState } from "@heroui/react";
import React, { useEffect, useMemo, useRef, useState } from "react";

import { useNavigate, useBlocker } from "react-router-dom";
import { Events } from "@wailsio/runtime";
import {
  InstallLIPPackage,
  ListDir,
  UninstallLIPPackage,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import {
  GetVersionMeta,
  GetVersionsDir,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/versionservice";
import {
  DeleteMod,
  EnableMod,
  DisableMod,
  ImportModZipPath,
  ImportModDllPath,
  UpdateModManifest,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/modsservice";
import * as types from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import {
  buildDefaultModLIPState,
  isLeviLaminaMod,
  normalizeIdentifierLookupKey,
  resolveModFolder,
  type LipGroupItem,
  type LIPPackageInstallState,
  type ModLIPState,
  type ModListItem,
} from "@/utils/modIntelligenceResolver";
import { readCurrentVersionName } from "@/utils/currentVersion";
import { useCurrentVersion } from "@/utils/CurrentVersionContext";
import { useModIntelligence } from "@/utils/ModIntelligenceContext";
import { useLipTaskConsole } from "@/utils/LipTaskConsoleContext";
import { setNavLockReason } from "@/hooks/useAppNavigation";
import { routeTo } from "@/constants/routes";
import { getPathBaseName, normalizeDroppedFiles } from "@/utils/fs";
import { resolveImportError } from "@/utils/importError";
import {
  appendTaskLog,
  finishActivityTask,
  notify,
  setTaskProgress,
  startActivityTask,
} from "@/utils/activity";

const ERR_LL_MANAGED_IN_VERSION_SETTINGS = "ERR_LL_MANAGED_IN_VERSION_SETTINGS";
const ERR_NO_UPDATE_SOURCE = "ERR_NO_UPDATE_SOURCE";
const ERR_ALREADY_LATEST = "ERR_ALREADY_LATEST";
const ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS =
  "ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS";
const ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY =
  "ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY";
const ERR_UNSUPPORTED_MOD_FILE = "ERR_UNSUPPORTED_MOD_FILE";
const ERR_OPEN_MOD_PACKAGE = "ERR_OPEN_MOD_PACKAGE";

// .mcmod is the mod package format; .zip (same layout) and .dll stay
// importable for files picked through "All files".
const resolveModImportKind = (path: string): "package" | "dll" | "unsupported" => {
  const lower = path.toLowerCase();
  if (lower.endsWith(".mcmod") || lower.endsWith(".zip")) return "package";
  if (lower.endsWith(".dll")) return "dll";
  return "unsupported";
};

type TFunc = (key: string, opts?: Record<string, unknown>) => string;
export type TabKey = "all" | "normal" | "lip";
export type VisibleItem = ModListItem | LipGroupItem;

type TaskLogLevel = "info" | "success" | "warning" | "error";
type TaskLogger = (level: TaskLogLevel, message: string) => void;

// One line per imported file. Lines are appended as files start and keep
// their final result, so the whole run stays readable after it finishes.
export type ImportLogStatus =
  | "waiting"
  | "importing"
  | "imported"
  | "overwritten"
  | "skipped"
  | "failed";
export type ImportLogEntry = {
  id: number;
  name: string;
  status: ImportLogStatus;
  reason?: string;
};
// A question the import needs answered before it can continue; shown inside
// the import dialog instead of a separate modal.
export type ImportPrompt = { kind: "dll" | "duplicate"; file: string };

export type BatchResult = {
  action: "update" | "uninstall";
  success: string[];
  failed: Array<{ name: string; err: string }>;
  demoted: string[];
};

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
  const [importOpen, setImportOpen] = useState(false);
  const [importLog, setImportLog] = useState<ImportLogEntry[]>([]);
  const [importPrompt, setImportPrompt] = useState<ImportPrompt | null>(null);
  const [importError, setImportError] = useState("");
  const [dupApplyAll, setDupApplyAll] = useState(false);
  const [importProgress, setImportProgress] = useState({ done: 0, total: 0 });
  const importInProgressRef = useRef(false);
  const [activeMod, setActiveMod] = useState<types.ModInfo | null>(null);
  const [editMod, setEditMod] = useState<types.ModInfo | null>(null);
  const [editName, setEditName] = useState("");
  const [editVersion, setEditVersion] = useState("");
  const [editType, setEditType] = useState("");
  const [editEntry, setEditEntry] = useState("");
  const [editAuthor, setEditAuthor] = useState("");
  const [editSaving, setEditSaving] = useState(false);
  const [activeLipIdentifier, setActiveLipIdentifier] = useState("");
  const [deleting, setDeleting] = useState<boolean>(false);
  const [batchUpdating, setBatchUpdating] = useState<boolean>(false);
  const [batchUninstalling, setBatchUninstalling] = useState<boolean>(false);
  const [batchResult, setBatchResult] = useState<BatchResult | null>(null);
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
  const [lipInstallStateByIdentifier, setLipInstallStateByIdentifier] =
    useState<Map<string, LIPPackageInstallState>>(new Map());

  const promptResolveRef = useRef<((answer: boolean) => void) | null>(null);
  const dllConfirmRef = useRef<{
    name: string;
    type: string;
    version: string;
  } | null>(null);
  // "Do the same for remaining duplicates" for the running import.
  const dupPolicyRef = useRef<"ask" | "overwrite" | "skip">("ask");
  const actionConfirmRunnerRef = useRef<(() => Promise<void>) | null>(null);
  const lastScrollTopRef = useRef<number>(0);
  const restorePendingRef = useRef<boolean>(false);

  const {
    isOpen: delCfmOpen,
    open: delCfmOnOpen,
    setOpen: delCfmOnOpenChange,
    close: delCfmOnClose,
  } = useOverlayState();
  const {
    isOpen: infoOpen,
    open: infoOnOpen,
    setOpen: infoOnOpenChange,
    close: infoOnClose,
  } = useOverlayState();
  const {
    isOpen: editOpen,
    open: editOnOpen,
    close: editOnClose,
  } = useOverlayState();
  const {
    isOpen: batchUpdateOpen,
    open: batchUpdateOnOpen,
    setOpen: batchUpdateOnOpenChange,
    close: batchUpdateOnClose,
  } = useOverlayState();
  const {
    isOpen: batchUninstallOpen,
    open: batchUninstallOnOpen,
    setOpen: batchUninstallOnOpenChange,
    close: batchUninstallOnClose,
  } = useOverlayState();
  const {
    isOpen: actionConfirmOpen,
    open: actionConfirmOnOpen,
    close: actionConfirmOnClose,
  } = useOverlayState();

  useBlocker(() => importing);

  useEffect(() => {
    setNavLockReason("mods-import", importing);
    return () => {
      if (importing) {
        setNavLockReason("mods-import", false);
      }
    };
  }, [importing]);

  const resolveLIPIdentifierForRequest = (
    identifier: string,
    identifierKey?: string,
  ): string => {
    return String(identifier || identifierKey || "").trim();
  };

  const activeVersionName = useMemo(
    () => currentVersionName || readCurrentVersionName(),
    [currentVersionName],
  );
  const currentSnapshot = useMemo(
    () => (activeVersionName ? getInstanceSnapshot(activeVersionName) : null),
    [activeVersionName, getInstanceSnapshot, snapshotRevision],
  );
  const lipSyncStatus = currentSnapshot?.lipSyncStatus || "idle";
  const lipSyncError = String(currentSnapshot?.lipSyncError || "").trim();
  const lipSourceLoaded = lipSyncStatus !== "idle";

  useEffect(() => {
    if (!activeVersionName) {
      setModsInfo([]);
      setEnabledByFolder(new Map());
      setLipInstallStateByIdentifier(new Map());
      return;
    }
    void ensureInstanceHydrated(activeVersionName, {
      background: true,
      reason: "mods-page-enter",
    });
  }, [activeVersionName, ensureInstanceHydrated]);

  useEffect(() => {
    if (!activeVersionName) return;
    if (!currentSnapshot) {
      setModsInfo([]);
      setEnabledByFolder(new Map());
      setLipInstallStateByIdentifier(new Map());
      return;
    }

    setModsInfo(
      Array.isArray(currentSnapshot.modsInfo) ? currentSnapshot.modsInfo : [],
    );
    setEnabledByFolder(
      currentSnapshot.enabledByFolder instanceof Map
        ? new Map(currentSnapshot.enabledByFolder)
        : new Map(),
    );
    setLipInstallStateByIdentifier(
      currentSnapshot.lipInstallStateByIdentifier instanceof Map
        ? new Map(currentSnapshot.lipInstallStateByIdentifier)
        : new Map(),
    );
  }, [activeVersionName, currentSnapshot]);

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

  const modLIPStateByFolder = useMemo(
    () =>
      currentSnapshot?.modLIPStateByFolder instanceof Map
        ? new Map(currentSnapshot.modLIPStateByFolder)
        : new Map<string, ModLIPState>(),
    [currentSnapshot],
  );

  const baseLipGroupItems = useMemo(
    () =>
      Array.isArray(currentSnapshot?.lipGroupItems)
        ? currentSnapshot.lipGroupItems
        : [],
    [currentSnapshot],
  );

  const baseNormalItems = useMemo(
    () =>
      Array.isArray(currentSnapshot?.normalItems)
        ? currentSnapshot.normalItems
        : [],
    [currentSnapshot],
  );

  const modsByFolder = useMemo(() => {
    const map = new Map<string, types.ModInfo>();
    for (const mod of modsInfo) {
      const folder = resolveModFolder(mod);
      if (!folder) continue;
      map.set(folder, mod);
    }
    return map;
  }, [modsInfo]);

  const allLipGroupItems = useMemo(() => {
    return baseLipGroupItems.map((group) => {
      const installState = lipInstallStateByIdentifier.get(
        normalizeIdentifierLookupKey(group.identifierKey),
      );
      if (!installState) return group;

      const uniqueModVersions = Array.from(
        new Set(
          group.mods
            .map((mod) => String(mod.version || "").trim())
            .filter(Boolean),
        ),
      );
      const installedVersion =
        String(installState.installedVersion || "").trim() ||
        (uniqueModVersions.length === 1
          ? uniqueModVersions[0]
          : group.installedVersion);

      return {
        ...group,
        installedVersion,
        explicitInstalled: Boolean(installState.explicitInstalled),
      };
    });
  }, [baseLipGroupItems, lipInstallStateByIdentifier]);

  const lipGroupByIdentifier = useMemo(() => {
    const map = new Map<string, LipGroupItem>();
    for (const group of allLipGroupItems) {
      map.set(group.identifierKey, group);
    }
    return map;
  }, [allLipGroupItems]);

  const lipManagedFolderSet = useMemo(() => {
    const set = new Set<string>();
    for (const group of allLipGroupItems) {
      for (const folder of group.folders) {
        set.add(folder);
      }
    }
    return set;
  }, [allLipGroupItems]);

  const normalMods = useMemo(() => {
    return baseNormalItems.map((item) => item.mod);
  }, [baseNormalItems]);

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

  const baseNormalItemsByFolder = useMemo(() => {
    const map = new Map<string, ModListItem>();
    for (const item of baseNormalItems) {
      map.set(item.folder, item);
    }
    return map;
  }, [baseNormalItems]);

  const normalItems = useMemo<ModListItem[]>(() => {
    return sortedNormalMods.map((mod) => {
      const folder = resolveModFolder(mod);
      const baseItem = baseNormalItemsByFolder.get(folder);
      const lipState =
        (folder && modLIPStateByFolder.get(folder)) ||
        baseItem?.lipState ||
        buildDefaultModLIPState();
      return {
        kind: "mod",
        key: baseItem?.key || `mod:${folder}`,
        folder,
        mod,
        lipState,
      };
    });
  }, [baseNormalItemsByFolder, modLIPStateByFolder, sortedNormalMods]);

  const filteredLipItems = useMemo(() => {
    let list = allLipGroupItems;
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
        String(text || "")
          .toLowerCase()
          .includes(q),
      );
    });
  }, [allLipGroupItems, onlyEnabled, query]);

  const lipGroupItems = useMemo(() => {
    const items = [...filteredLipItems];
    items.sort((a, b) => {
      const nameA = String(a.packageName || "").toLowerCase();
      const nameB = String(b.packageName || "").toLowerCase();
      if (nameA < nameB) return sortConfig.direction === "asc" ? -1 : 1;
      if (nameA > nameB) return sortConfig.direction === "asc" ? 1 : -1;
      return 0;
    });
    return items;
  }, [filteredLipItems, sortConfig.direction]);

  const visibleItems = useMemo<VisibleItem[]>(() => {
    if (tabKey === "normal") return normalItems;
    if (tabKey === "lip") return lipGroupItems;
    return [...normalItems, ...lipGroupItems];
  }, [lipGroupItems, normalItems, tabKey]);

  const listHydrating =
    Boolean(activeVersionName) &&
    (!currentSnapshot || currentSnapshot.status === "loading");

  const lipInfoPending =
    Boolean(activeVersionName) &&
    currentSnapshot?.status === "ready" &&
    lipSyncStatus === "loading";

  const lipInfoWarning = lipSyncStatus === "error" ? lipSyncError : "";

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
    return (
      lipInstallStateByIdentifier.get(
        normalizeIdentifierLookupKey(activeLipGroup.identifierKey),
      ) || null
    );
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
    const lookupKey = normalizeIdentifierLookupKey(identifierKey);
    if (!lookupKey) return;
    setLipInstallStateByIdentifier((prev) => {
      const next = new Map(prev);
      next.set(lookupKey, state);
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
      const state = await ensurePackageInstallState(
        name,
        group.identifierKey || group.identifier,
      );
      setLipInstallStateCache(group.identifierKey, state);
      return state;
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
    const installState = lipInstallStateByIdentifier.get(
      normalizeIdentifierLookupKey(group.identifierKey),
    );
    const installedVersion = String(
      installState?.installedVersion || "",
    ).trim();
    if (installedVersion) return installedVersion;
    return String(group.installedVersion || "").trim();
  };

  const isLipGroupPromotable = (group: LipGroupItem): boolean => {
    const installState = lipInstallStateByIdentifier.get(
      normalizeIdentifierLookupKey(group.identifierKey),
    );
    if (!installState) {
      return Boolean(group.installedVersion) && !group.explicitInstalled;
    }
    return Boolean(installState.installed) && !installState.explicitInstalled;
  };

  // Notices go through notify() so they stay in the activity history; errors
  // carry a title naming the action that failed.
  const notifyAction = (
    type: "success" | "error",
    actionLabel: string,
    name: string,
    err?: unknown,
  ) => {
    if (type === "success") {
      notify({
        variant: "success",
        title: t("mods.notify.action_done", { action: actionLabel }),
        description: name,
        source: "mods",
      });
      return;
    }

    notify({
      variant: "danger",
      title: t("mods.notify.action_failed", { action: actionLabel }),
      description: name
        ? `${name}: ${resolveErrorText(t, err)}`
        : resolveErrorText(t, err),
      source: "mods",
    });
  };

  const notifyNoInstance = () => {
    notify({
      variant: "warning",
      title: t("mods.no_instance.title"),
      description: t("mods.no_instance.body"),
      source: "mods",
    });
  };

  const populateEditForm = (mod: types.ModInfo) => {
    setEditName(String(mod.name || ""));
    setEditVersion(String(mod.version || ""));
    setEditType(String(mod.type || ""));
    setEditEntry(String(mod.entry || ""));
    setEditAuthor(String((mod as any).author || ""));
  };

  const openEditForMod = (mod: types.ModInfo) => {
    setActiveLipIdentifier("");
    setEditMod(mod);
    populateEditForm(mod);
    editOnOpen();
  };

  const resetEditForm = () => {
    setEditMod(null);
    setEditName("");
    setEditVersion("");
    setEditType("");
    setEditEntry("");
    setEditAuthor("");
  };

  const closeEditModal = () => {
    if (editSaving) return;
    editOnClose();
    resetEditForm();
  };

  const editOnOpenChange = (nextOpen: boolean) => {
    if (nextOpen) {
      editOnOpen();
      return;
    }
    closeEditModal();
  };

  const editFormValid = editName.trim().length > 0 && !editSaving;

  const handleSaveModEdit = async () => {
    const name = activeVersionName;
    const mod = editMod;
    const folder = mod ? resolveModFolder(mod) : "";
    const nextName = editName.trim();

    if (!name || !mod || !folder || !nextName) {
      notify({
        variant: "danger",
        title: t("mods.notify.action_failed", { action: t("common.save") }),
        description: t("mods.err_invalid_name"),
        source: "mods",
      });
      return;
    }

    const nextMod = {
      ...mod,
      name: nextName,
      version: editVersion.trim(),
      type: editType.trim(),
      entry: editEntry.trim(),
      author: editAuthor.trim(),
      folder,
    } as types.ModInfo;

    const pos = scrollRef.current?.scrollTop || 0;
    lastScrollTopRef.current = pos;
    restorePendingRef.current = true;
    setEditSaving(true);

    try {
      const err = await UpdateModManifest(
        name,
        folder,
        nextMod.name,
        nextMod.version,
        nextMod.type,
        nextMod.entry,
        String((nextMod as any).author || ""),
      );

      if (err) {
        notifyAction("error", t("common.save"), mod.name, err);
        return;
      }

      setModsInfo((prev) =>
        prev.map((item) =>
          resolveModFolder(item) === folder ? nextMod : item,
        ),
      );
      setActiveMod((current) =>
        current && resolveModFolder(current) === folder ? nextMod : current,
      );
      setEditMod(nextMod);
      await refreshInstance(name, "mods-edit");
      notifyAction("success", t("common.save"), nextMod.name);
      editOnClose();
      resetEditForm();
    } catch (error) {
      notifyAction(
        "error",
        t("common.save"),
        mod.name,
        parseErrorCode(error) || "ERR_WRITE_FILE",
      );
    } finally {
      setEditSaving(false);
    }
  };

  const askImportPrompt = async (prompt: ImportPrompt) => {
    setImportPrompt(prompt);
    return new Promise<boolean>((resolve) => {
      promptResolveRef.current = resolve;
    });
  };

  // Answers the pending inline prompt. For a DLL, `accept` imports it with the
  // form values; for a duplicate, `accept` overwrites the existing mod.
  // `dismissed` (Escape/backdrop) skips only this file, never the rest.
  const answerImportPrompt = (accept: boolean, dismissed = false) => {
    const resolve = promptResolveRef.current;
    if (!resolve) return;
    if (importPrompt?.kind === "dll") {
      const nm = dllName.trim();
      if (accept && !nm) return;
      dllConfirmRef.current = accept
        ? {
            name: nm,
            type: dllType.trim() || "preload-native",
            version: dllVersion.trim() || "0.0.0",
          }
        : null;
    } else if (importPrompt?.kind === "duplicate" && dupApplyAll && !dismissed) {
      dupPolicyRef.current = accept ? "overwrite" : "skip";
    }
    promptResolveRef.current = null;
    setImportPrompt(null);
    resolve(accept);
  };

  const closeImportDialog = () => {
    if (importInProgressRef.current) return;
    setImportOpen(false);
  };

  // Escape or a backdrop click while a prompt is pending skips that file so
  // the import can move on; once finished, it closes the dialog.
  const importOnOpenChange = (open: boolean) => {
    if (open) return;
    if (promptResolveRef.current) {
      answerImportPrompt(false, true);
      return;
    }
    closeImportDialog();
  };

  const doImportFromPaths = async (paths: string[]) => {
    if (importInProgressRef.current) {
      if (normalizeDroppedFiles(paths).length > 0) {
        notify({
          variant: "warning",
          title: t("mods.notify.import_busy_title"),
          description: t("mods.notify.import_busy_body"),
          source: "mods",
        });
      }
      return;
    }
    importInProgressRef.current = true;
    let taskId = "";
    let aborted = false;
    const counts: Record<ImportLogStatus, number> = {
      waiting: 0,
      importing: 0,
      imported: 0,
      overwritten: 0,
      skipped: 0,
      failed: 0,
    };
    try {
      const normalizedPaths = normalizeDroppedFiles(paths);
      if (!normalizedPaths.length) return;
      const name = activeVersionName;
      if (!name) {
        notifyNoInstance();
        return;
      }
      const total = normalizedPaths.length;
      let done = 0;
      let changed = false;
      let seq = 0;
      dupPolicyRef.current = "ask";
      setDupApplyAll(false);
      setImportLog([]);
      setImportError("");
      setImportProgress({ done, total });
      setImporting(true);
      setImportOpen(true);
      taskId = startActivityTask({
        title: t("mods.importing_title"),
        description: name,
        source: "mods",
      });

      const begin = (file: string, status: ImportLogStatus) => {
        seq += 1;
        const id = seq;
        setImportLog((prev) => [...prev, { id, name: file, status }]);
        return id;
      };
      const mark = (id: number, status: ImportLogStatus) => {
        setImportLog((prev) =>
          prev.map((entry) => (entry.id === id ? { ...entry, status } : entry)),
        );
      };
      const settle = (
        id: number,
        file: string,
        status: ImportLogStatus,
        reason?: string,
      ) => {
        counts[status] += 1;
        setImportLog((prev) =>
          prev.map((entry) =>
            entry.id === id ? { ...entry, status, reason } : entry,
          ),
        );
        appendTaskLog(
          taskId,
          `${file}: ${t(`mods.import_log.${status}`)}${reason ? ` (${reason})` : ""}`,
          status === "failed"
            ? "error"
            : status === "skipped"
              ? "warning"
              : "success",
        );
      };
      const failReason = (err: unknown) =>
        resolveImportError(parseErrorCode(err) || "IMPORT_ERROR", t as any);
      const shouldOverwrite = async (file: string) => {
        if (dupPolicyRef.current !== "ask") {
          return dupPolicyRef.current === "overwrite";
        }
        return askImportPrompt({ kind: "duplicate", file });
      };

      // Runs one import call and resolves a duplicate folder inline.
      const runImport = async (
        id: number,
        base: string,
        importWith: (overwrite: boolean) => Promise<string>,
        mapErr: (err: string) => string = (err) => err,
      ) => {
        let err = String((await importWith(false)) || "");
        let overwritten = false;
        if (err === "ERR_DUPLICATE_FOLDER") {
          mark(id, "waiting");
          if (!(await shouldOverwrite(base))) {
            settle(id, base, "skipped", t("mods.import_log.reason_exists"));
            return;
          }
          mark(id, "importing");
          overwritten = true;
          err = String((await importWith(true)) || "");
        }
        if (err) {
          settle(id, base, "failed", failReason(mapErr(err)));
          return;
        }
        changed = true;
        settle(id, base, overwritten ? "overwritten" : "imported");
      };

      const importPackage = (p: string, base: string, id: number) =>
        runImport(
          id,
          base,
          (overwrite) => ImportModZipPath(name, p, overwrite),
          // ERR_OPEN_ZIP is shared with content imports; name the mod package.
          (err) => (err === "ERR_OPEN_ZIP" ? ERR_OPEN_MOD_PACKAGE : err),
        );

      const importDll = async (p: string, base: string, id: number) => {
        setDllName(base.replace(/\.[^/.]+$/, ""));
        setDllType("preload-native");
        setDllVersion("0.0.0");
        dllConfirmRef.current = null;
        const ok = await askImportPrompt({ kind: "dll", file: base });
        const vals = dllConfirmRef.current as {
          name: string;
          type: string;
          version: string;
        } | null;
        dllConfirmRef.current = null;
        if (!ok || !vals) {
          settle(id, base, "skipped", t("mods.import_log.reason_cancelled"));
          return;
        }
        mark(id, "importing");
        await runImport(id, base, (overwrite) =>
          ImportModDllPath(
            name,
            p,
            vals.name,
            vals.type,
            vals.version,
            overwrite,
          ),
        );
      };

      for (const p of normalizedPaths) {
        const base = getPathBaseName(p) || p;
        const kind = resolveModImportKind(p);
        const id = begin(base, kind === "dll" ? "waiting" : "importing");
        try {
          if (kind === "package") {
            await importPackage(p, base, id);
          } else if (kind === "dll") {
            await importDll(p, base, id);
          } else {
            settle(id, base, "failed", failReason(ERR_UNSUPPORTED_MOD_FILE));
          }
        } catch (e) {
          settle(id, base, "failed", failReason(e));
        }
        done += 1;
        setImportProgress({ done, total });
        setTaskProgress(taskId, (done / total) * 100);
      }
      if (changed) {
        await refreshInstance(name, "mods-import-complete");
      }
    } catch (e: any) {
      const message = String(e?.message || e || "IMPORT_ERROR");
      aborted = true;
      // Shown in the import dialog itself (which marks the run failed); the
      // silent notice keeps a copy in the activity history.
      setImportError(message);
      notify({
        silent: true,
        variant: "danger",
        title: t("mods.notify.import_failed"),
        description: message,
        source: "mods",
      });
      if (taskId) appendTaskLog(taskId, message, "error");
    } finally {
      if (taskId) {
        const succeeded = counts.imported + counts.overwritten;
        finishActivityTask(
          taskId,
          aborted || (counts.failed > 0 && succeeded === 0)
            ? "error"
            : succeeded === 0 && counts.failed === 0
              ? "cancelled"
              : "success",
          t("mods.import_summary", {
            imported: succeeded,
            skipped: counts.skipped,
            failed: counts.failed,
          }),
        );
      }
      importInProgressRef.current = false;
      promptResolveRef.current = null;
      setImportPrompt(null);
      setImporting(false);
    }
  };

  const doImportRef = useRef(doImportFromPaths);
  doImportRef.current = doImportFromPaths;

  // Switching instance in place starts from a clean selection.
  useEffect(() => {
    setSelectedKeys(new Set());
  }, [activeVersionName]);

  useEffect(() => {
    return Events.On("files-dropped", (event) => {
      const data = (event.data as { files: string[] }) || {};
      const files = normalizeDroppedFiles(data.files);
      if (files.length > 0) {
        void doImportRef.current(files);
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
      const err = await InstallLIPPackage(
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

  // Whether uninstallModWithFallback runs a lip task for this mod (the other
  // mods are deleted directly, with nothing for a progress bar to measure).
  const uninstallRunsLipTask = (mod: types.ModInfo): boolean => {
    if (isLeviLaminaMod(mod)) return false;
    const folder = resolveModFolder(mod);
    if (!folder || !lipManagedFolderSet.has(folder)) return false;
    const state = modLIPStateByFolder.get(folder);
    return state?.sourceType === "unique" && !!state.identifier;
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
        const uninstallErr = await UninstallLIPPackage(
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
      const uninstallErr = await UninstallLIPPackage(
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
      notifyNoInstance();
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
    setBatchResult(null);
    batchUpdateOnOpen();
  };

  const openBatchUninstallConfirm = () => {
    if (selectedItems.length === 0) return;
    setBatchResult(null);
    batchUninstallOnOpen();
  };

  // A clean batch closes its confirm dialog with a notice. Anything the user
  // should read (failures, packages kept as dependencies) turns the same
  // dialog into the result view instead of stacking another modal.
  const finishBatch = (
    action: BatchResult["action"],
    success: string[],
    failed: BatchResult["failed"],
    demoted: string[],
    close: () => void,
  ) => {
    const actionLabel = t(
      action === "update" ? "mods.action_update" : "mods.action_uninstall",
    );
    if (failed.length > 0 || demoted.length > 0) {
      // Store readable reasons so the dialog and the history read the same.
      setBatchResult({
        action,
        success,
        failed: failed.map((item) => ({
          name: item.name,
          err: resolveErrorText(t, item.err),
        })),
        demoted,
      });
      notify({
        silent: true,
        variant: failed.length > 0 ? "danger" : "warning",
        title: t(
          failed.length > 0
            ? "mods.notify.action_failed"
            : "mods.notify.action_done",
          { action: actionLabel },
        ),
        description: [
          ...failed.map(
            (item) => `${item.name}: ${resolveErrorText(t, item.err)}`,
          ),
          ...demoted.map(
            (item) => `${item}: ${t("mods.batch_result.demoted")}`,
          ),
        ].join("\n"),
        source: "mods",
      });
      return;
    }
    close();
    if (success.length > 0) {
      notify({
        variant: "success",
        title: t("mods.notify.action_done", { action: actionLabel }),
        description: success.join(", "),
        source: "mods",
      });
    }
  };

  const handleBatchUpdate = async () => {
    const name = activeVersionName;
    if (!name) {
      notifyNoInstance();
      return;
    }

    const updatableItems = selectedItems.filter(
      (item) =>
        item.lipState.sourceType === "unique" && item.lipState.canUpdate,
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
            feedbackMode: "progress",
            errorConsole: false,
          },
          async ({ addLog, setOuter }) => {
            for (const [index, item] of updatableItems.entries()) {
              setOuter(
                index,
                updatableItems.length,
                item.kind === "mod" ? item.mod.name : item.packageName,
              );
              if (item.kind === "mod") {
                const identifierKey = item.lipState.identifierKey;
                if (identifierKey && handledIdentifiers.has(identifierKey))
                  continue;

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

              if (
                item.identifierKey &&
                handledIdentifiers.has(item.identifierKey)
              ) {
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
            setOuter(updatableItems.length, updatableItems.length);

            if (failed.length > 0) {
              throw new Error(
                failed[0].err || "ERR_LIP_PACKAGE_INSTALL_FAILED",
              );
            }
          },
        );
      } catch {}

      if (hasSuccess) await refreshAll();
      setSelectedKeys(new Set());
      finishBatch("update", success, failed, [], batchUpdateOnClose);
    } finally {
      setBatchUpdating(false);
    }
  };

  const handleBatchUninstall = async () => {
    const name = activeVersionName;
    if (!name) {
      notifyNoInstance();
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
            feedbackMode: "progress",
            errorConsole: false,
          },
          async ({ addLog, setOuter }) => {
            for (const [index, item] of selectedItems.entries()) {
              setOuter(
                index,
                selectedItems.length,
                item.kind === "mod" ? item.mod.name : item.packageName,
              );
              if (item.kind === "mod") {
                addLog(
                  "info",
                  `${t("mods.action_uninstall")}: ${item.mod.name}`,
                );
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

              if (
                item.identifierKey &&
                handledIdentifiers.has(item.identifierKey)
              ) {
                continue;
              }

              const latestState = await queryLipGroupInstallState(item);
              if (latestState.installed && !latestState.explicitInstalled) {
                const blockedErr = ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS;
                addLog("warning", `${item.packageName}: ${blockedErr}`);
                failed.push({ name: item.packageName, err: blockedErr });
                continue;
              }

              addLog(
                "info",
                `${t("mods.action_uninstall")}: ${item.packageName}`,
              );
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
            setOuter(selectedItems.length, selectedItems.length);

            if (failed.length > 0) {
              throw new Error(
                failed[0].err || "ERR_LIP_PACKAGE_UNINSTALL_FAILED",
              );
            }
          },
        );
      } catch {}

      if (hasStateChanged) await refreshAll();
      setSelectedKeys(new Set());
      finishBatch("uninstall", success, failed, demoted, batchUninstallOnClose);
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
    navigate(routeTo.lipPackage(target));
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
      notifyNoInstance();
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
          feedbackMode: "progress",
        },
        async ({ addLog }) => {
          addLog("info", `${t("mods.action_update")}: ${mod.name}`);
          const err = await installModByLIP(mod, addLog);
          if (err) throw new Error(err);
        },
      );
    } catch (error) {
      notifyAction(
        "error",
        t("mods.action_update"),
        mod.name,
        parseErrorCode(error) || "ERR_LIP_PACKAGE_INSTALL_FAILED",
      );
      return;
    }

    await refreshAll();
    notifyAction("success", t("mods.action_update"), mod.name);
  };

  const handleUpdateMod = async (mod: types.ModInfo) => {
    const lipState = getModLIPState(mod);
    if (lipState.sourceType !== "unique") {
      notifyAction(
        "error",
        t("mods.action_update"),
        mod.name,
        ERR_NO_UPDATE_SOURCE,
      );
      return;
    }
    if (!lipState.canUpdate) {
      notifyAction(
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
      notifyNoInstance();
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
          feedbackMode: "progress",
        },
        async ({ addLog }) => {
          addLog("info", `${t("mods.action_update")}: ${group.packageName}`);
          const err = await installLipGroupByLIP(group, addLog);
          if (err) throw new Error(err);
        },
      );
    } catch (error) {
      notifyAction(
        "error",
        t("mods.action_update"),
        group.packageName,
        parseErrorCode(error) || "ERR_LIP_PACKAGE_INSTALL_FAILED",
      );
      return;
    }

    await refreshAll();
    notifyAction("success", t("mods.action_update"), group.packageName);
  };

  const handleUpdateLipGroup = async (group: LipGroupItem) => {
    if (group.lipState.sourceType !== "unique") {
      notifyAction(
        "error",
        t("mods.action_update"),
        group.packageName,
        ERR_NO_UPDATE_SOURCE,
      );
      return;
    }
    if (!group.lipState.canUpdate) {
      notifyAction(
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
      notifyNoInstance();
      return;
    }

    try {
      await runWithLipTask(
        {
          action: "install",
          target: name,
          methods: ["Install"],
          feedbackMode: "progress",
        },
        async ({ addLog }) => {
          addLog(
            "info",
            `${t("mods.action_promote_install")}: ${group.packageName}@${version}`,
          );
          const err = await InstallLIPPackage(
            name,
            requestIdentifier,
            version,
          );
          if (err) throw new Error(String(err));
        },
      );
    } catch (error) {
      notifyAction(
        "error",
        t("mods.action_promote_install"),
        group.packageName,
        parseErrorCode(error) || "ERR_LIP_PACKAGE_INSTALL_FAILED",
      );
      return;
    }

    await refreshAll();
    notify({
      variant: "success",
      title: t("mods.notify.action_done", {
        action: t("mods.action_promote_install"),
      }),
      description: t("mods.action_promote_install_success", {
        name: group.packageName,
      }),
      source: "mods",
    });
  };

  const handlePromoteLipGroup = async (group: LipGroupItem) => {
    const name = activeVersionName;
    if (!name) {
      notifyNoInstance();
      return;
    }

    const requestIdentifier = resolveLIPIdentifierForRequest(
      group.identifier,
      group.identifierKey,
    );
    if (!requestIdentifier) {
      notifyAction(
        "error",
        t("mods.action_promote_install"),
        group.packageName,
        "ERR_LIP_PACKAGE_INVALID_IDENTIFIER",
      );
      return;
    }

    const installState = await queryLipGroupInstallState(group);
    if (installState.error) {
      notifyAction(
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
      notifyAction(
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

  // Uninstall errors are thrown so the confirm dialog shows them inline and
  // stays open; the activity history keeps a copy.
  const failUninstall = (name: string, err: unknown): never => {
    const text = resolveErrorText(t, err);
    notify({
      silent: true,
      variant: "danger",
      title: t("mods.notify.action_failed", {
        action: t("mods.action_uninstall"),
      }),
      description: `${name}: ${text}`,
      source: "mods",
    });
    throw new Error(text);
  };

  // A package other mods still need is kept as a dependency; say so in a
  // notice that also lands in the activity history.
  const notifyDemoted = (names: string[]) => {
    notify({
      variant: "warning",
      title: t("mods.uninstall.demoted_title"),
      description: `${names.join(", ")}: ${t("errors.ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY")}`,
      source: "mods",
    });
  };

  const handleDeleteMod = async () => {
    if (!activeMod) return;
    const name = activeVersionName;
    if (!name) {
      notifyNoInstance();
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
          feedbackMode: uninstallRunsLipTask(activeMod)
            ? "progress"
            : "on_error",
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
      setDeleting(false);
      failUninstall(
        activeMod.name,
        parseErrorCode(error) || "ERR_LIP_PACKAGE_UNINSTALL_FAILED",
      );
    }

    await refreshAll();
    requestAnimationFrame(() => {
      try {
        if (scrollRef.current) scrollRef.current.scrollTop = pos;
      } catch {}
    });
    if (demotedToDependency) {
      notifyDemoted([activeMod.name]);
      infoOnClose();
      setDeleting(false);
      return;
    }
    notifyAction("success", t("mods.action_uninstall"), activeMod.name);
    infoOnClose();
    setDeleting(false);
  };

  const handleDeleteLipGroup = async () => {
    if (!activeLipGroup) return;

    const name = activeVersionName;
    if (!name) {
      notifyNoInstance();
      return;
    }

    const latestState = await queryLipGroupInstallState(activeLipGroup);
    if (latestState.installed && !latestState.explicitInstalled) {
      failUninstall(
        activeLipGroup.packageName,
        ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS,
      );
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
          feedbackMode: "progress",
        },
        async ({ addLog }) => {
          addLog(
            "info",
            `${t("mods.action_uninstall")}: ${activeLipGroup.packageName}`,
          );
          const err = await uninstallLipGroupWithFallback(
            activeLipGroup,
            addLog,
          );
          if (err === ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY) {
            demotedToDependency = true;
            addLog("warning", `${activeLipGroup.packageName}: ${err}`);
            return;
          }
          if (err) throw new Error(err);
        },
      );
    } catch (error) {
      setDeleting(false);
      failUninstall(
        activeLipGroup.packageName,
        parseErrorCode(error) || "ERR_LIP_PACKAGE_UNINSTALL_FAILED",
      );
    }

    await refreshAll();
    requestAnimationFrame(() => {
      try {
        if (scrollRef.current) scrollRef.current.scrollTop = pos;
      } catch {}
    });
    if (demotedToDependency) {
      notifyDemoted([activeLipGroup.packageName]);
      infoOnClose();
      setDeleting(false);
      return;
    }
    notifyAction(
      "success",
      t("mods.action_uninstall"),
      activeLipGroup.packageName,
    );
    setDeleting(false);
  };

  const handleDeleteCurrentTarget = async () => {
    if (activeLipIdentifier) {
      if (activeDeleteBlocked && activeLipGroup) {
        failUninstall(
          activeLipGroup.packageName,
          ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS,
        );
      }
      await handleDeleteLipGroup();
      return;
    }
    await handleDeleteMod();
  };

  const openFolder = () => {
    const name = activeVersionName;
    if (!name) {
      notifyNoInstance();
      return;
    }
    void openModsDirectory(name);
  };

  const openModFolder = async (mod: types.ModInfo) => {
    const name = activeVersionName;
    if (!name) return;

    const folder = resolveModFolder(mod);
    if (!folder) return;

    let modPath: string;
    try {
      const versionsDir = await GetVersionsDir();
      const sep = versionsDir.includes("\\") ? "\\" : "/";
      modPath = `${versionsDir}${sep}${name}${sep}mods${sep}${folder}`;
      await ListDir(modPath);
    } catch {
      await openModsDirectory(name);
      return;
    }
    await openDirectory(modPath);
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
    await refreshInstance(
      name,
      val ? "mods-group-enable" : "mods-group-disable",
    );
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

  // LIP-managed uninstalls may keep the package as a dependency; the confirm
  // dialog says so up front instead of a follow-up warning modal.
  const activeDeleteMayDemote = activeLipGroup
    ? true
    : activeMod
      ? uninstallRunsLipTask(activeMod)
      : false;

  const selectedLipManagedCount = selectedItems.filter((item) =>
    item.kind === "lip" ? true : uninstallRunsLipTask(item.mod),
  ).length;

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
    importOpen,
    importOnOpenChange,
    closeImportDialog,
    importLog,
    importPrompt,
    importError,
    answerImportPrompt,
    dupApplyAll,
    setDupApplyAll,
    importProgress,
    activeMod,
    setActiveMod,
    editMod,
    editName,
    setEditName,
    editVersion,
    setEditVersion,
    editType,
    setEditType,
    editEntry,
    setEditEntry,
    editAuthor,
    setEditAuthor,
    editSaving,
    editFormValid,
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
    batchResult,
    activeDeleteMayDemote,
    selectedLipManagedCount,
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
    lipSyncStatus,
    lipSyncError,
    lipInfoPending,
    lipInfoWarning,
    listHydrating,
    selectedMods,
    selectedItems,
    selectedUpdatableCount,

    delCfmOpen,
    delCfmOnOpen,
    delCfmOnClose,
    delCfmOnOpenChange,
    infoOpen,
    infoOnOpen,
    infoOnClose,
    infoOnOpenChange,
    editOpen,
    editOnOpen,
    editOnClose: closeEditModal,
    editOnOpenChange,
    batchUpdateOpen,
    batchUpdateOnOpen,
    batchUpdateOnClose,
    batchUpdateOnOpenChange,
    batchUninstallOpen,
    batchUninstallOnOpen,
    batchUninstallOnClose,
    batchUninstallOnOpenChange,
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
    hasInstalledItems: baseNormalItems.length > 0 || allLipGroupItems.length > 0,
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
    openEditForMod,
    handleSaveModEdit,
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
