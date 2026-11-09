import { openDirectory, openModsDirectory } from "@/utils/explorer";
import { toast, useOverlayState } from "@heroui/react";
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
import { ROUTES, routeTo } from "@/constants/routes";
import { getPathBaseName, normalizeDroppedFiles } from "@/utils/fs";
import { explainErrorDetail, splitErrorResult } from "@/utils/errorDetail";

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
  const [errorMsg, setErrorMsg] = useState("");
  const [errorFile, setErrorFile] = useState("");
  const [resultSuccess, setResultSuccess] = useState<string[]>([]);
  const [resultFailed, setResultFailed] = useState<
    Array<{ name: string; err: string }>
  >([]);
  const [currentFile, setCurrentFile] = useState("");
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
  const [lipInstallStateByIdentifier, setLipInstallStateByIdentifier] =
    useState<Map<string, LIPPackageInstallState>>(new Map());

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
    open: errOnOpen,
    setOpen: errOnOpenChange,
    close: errOnClose,
  } = useOverlayState();
  const {
    isOpen: delOpen,
    open: delOnOpen,
    setOpen: delOnOpenChange,
    close: delOnClose,
  } = useOverlayState();
  const {
    isOpen: dllOpen,
    open: dllOnOpen,
    setOpen: setDllOpen,
    close: dllOnClose,
  } = useOverlayState();
  const {
    isOpen: dupOpen,
    open: dupOnOpen,
    setOpen: setDupOpen,
    close: dupOnClose,
  } = useOverlayState();
  // Escape or a backdrop click closes these prompts through onOpenChange
  // without calling onCancel. Settle the pending import prompt as a cancel so
  // doImportFromPaths can move on (and release importInProgressRef). After a
  // confirm/cancel the promise is already settled, so this is a no-op.
  const dllOnOpenChange = React.useCallback(
    (open: boolean) => {
      setDllOpen(open);
      if (open) return;
      const resolve = dllResolveRef.current;
      dllResolveRef.current = null;
      resolve?.(false);
    },
    [setDllOpen],
  );
  const dupOnOpenChange = React.useCallback(
    (open: boolean) => {
      setDupOpen(open);
      if (open) return;
      const resolve = dupResolveRef.current;
      dupResolveRef.current = null;
      resolve?.(false);
    },
    [setDupOpen],
  );
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
    isOpen: demotedWarningOpen,
    open: demotedWarningOnOpen,
    close: demotedWarningOnClose,
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

  // Updates and promotions name the package without a variant ("<id>", not
  // "<id>#" or "<id>#<variant>"): the backend changes the installed package
  // in place whatever its variant, and only for a package that is not
  // installed picks the variant itself (the catalogue's rule A2: "client",
  // then the default "", then the others by name, among those with a
  // client entry for this platform).
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

  const showActionToast = (
    type: "success" | "error",
    actionLabel: string,
    name: string,
    err?: unknown,
  ) => {
    if (type === "success") {
      toast(t("common.success"), {
        variant: "success",
        description: `${actionLabel}: ${name}`,
        timeout: 2000,
      });
      return;
    }

    // The backend appends the technical detail after the code; explain the
    // likeliest cause in words so the toast says why, not only that it failed.
    const { detail } = splitErrorResult(err);
    const cause = explainErrorDetail(detail)[0];
    const message = `${actionLabel}: ${resolveErrorText(t, err)}`;
    toast(t("common.error"), {
      variant: "danger",
      description: cause ? `${message}
${t(`errors.explain.${cause}`)}` : message,
      timeout: cause ? 8000 : 2000,
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
      toast(t("common.error"), {
        variant: "danger",
        description: t("mods.err_invalid_name"),
        timeout: 2000,
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
        showActionToast("error", t("common.save"), mod.name, err);
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
      showActionToast("success", t("common.save"), nextMod.name);
      editOnClose();
      resetEditForm();
    } catch (error) {
      showActionToast(
        "error",
        t("common.save"),
        mod.name,
        parseErrorCode(error) || "ERR_WRITE_FILE",
      );
    } finally {
      setEditSaving(false);
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
    if (importInProgressRef.current) return;
    importInProgressRef.current = true;
    try {
      const normalizedPaths = normalizeDroppedFiles(paths);
      if (!normalizedPaths.length) return;
      const name = activeVersionName;
      if (!name) {
        setErrorMsg(t("launcherpage.currentVersion_none") as string);
        return;
      }
      let started = false;
      const startImporting = () => {
        if (started) return;
        setImporting(true);
        started = true;
      };
      const succFiles: string[] = [];
      const errPairs: Array<{ name: string; err: string }> = [];
      const total = normalizedPaths.length;
      let done = 0;
      setImportProgress({ done, total });

      const askOverwrite = async (label: string) => {
        dupNameRef.current = label;
        await new Promise<void>((r) => setTimeout(r, 0));
        dupOnOpen();
        return new Promise<boolean>((resolve) => {
          dupResolveRef.current = resolve;
        });
      };

      const importPackage = async (p: string, base: string) => {
        startImporting();
        setCurrentFile(base);
        let err = await ImportModZipPath(name, p, false);
        if (err && String(err) === "ERR_DUPLICATE_FOLDER") {
          if (!(await askOverwrite(base))) return;
          err = await ImportModZipPath(name, p, true);
        }
        if (err) {
          // ERR_OPEN_ZIP is shared with content imports; name the mod package.
          errPairs.push({
            name: base,
            err: String(err) === "ERR_OPEN_ZIP" ? ERR_OPEN_MOD_PACKAGE : err,
          });
          return;
        }
        succFiles.push(base);
      };

      const importDll = async (p: string, base: string) => {
        const baseNoExt = base.replace(/\.[^/.]+$/, "");
        setDllName(baseNoExt);
        setDllType("preload-native");
        setDllVersion("0.0.0");
        await new Promise<void>((resolve) => setTimeout(resolve, 0));
        dllOnOpen();
        const ok = await new Promise<boolean>((resolve) => {
          dllResolveRef.current = resolve;
        });
        if (!ok) return;
        startImporting();
        setCurrentFile(base);
        const vals = dllConfirmRef.current || {
          name: dllName,
          type: dllType,
          version: dllVersion,
        };
        dllConfirmRef.current = null;
        const importWith = (overwrite: boolean) =>
          ImportModDllPath(
            name,
            p,
            vals.name,
            vals.type || "preload-native",
            vals.version || "0.0.0",
            overwrite,
          );
        let err = await importWith(false);
        if (err && String(err) === "ERR_DUPLICATE_FOLDER") {
          if (!(await askOverwrite(base))) return;
          err = await importWith(true);
        }
        if (err) {
          errPairs.push({ name: base, err });
          return;
        }
        succFiles.push(base);
      };

      for (const p of normalizedPaths) {
        const base = getPathBaseName(p) || p;
        const kind = resolveModImportKind(p);
        try {
          if (kind === "package") {
            await importPackage(p, base);
          } else if (kind === "dll") {
            await importDll(p, base);
          } else {
            errPairs.push({ name: base, err: ERR_UNSUPPORTED_MOD_FILE });
          }
        } catch (e) {
          errPairs.push({ name: base, err: parseErrorCode(e) || "IMPORT_ERROR" });
        }
        done += 1;
        setImportProgress({ done, total });
      }
      if (started) {
        await refreshInstance(name, "mods-import-complete");
      }
      setResultSuccess(succFiles);
      setResultFailed(errPairs);
      if (succFiles.length > 0 || errPairs.length > 0) {
        errOnOpen();
      }
    } catch (e: any) {
      setErrorMsg(String(e?.message || e || "IMPORT_ERROR"));
    } finally {
      importInProgressRef.current = false;
      dllResolveRef.current = null;
      dupResolveRef.current = null;
      setImporting(false);
      setCurrentFile("");
      setImportProgress({ done: 0, total: 0 });
    }
  };

  const doImportRef = useRef(doImportFromPaths);
  doImportRef.current = doImportFromPaths;

  useEffect(() => {
    if (!activeVersionName) {
      navigate(ROUTES.instances, { replace: true });
    }
  }, [activeVersionName, navigate]);

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
      toast(t("common.error"), {
        variant: "danger",
        description: t("launcherpage.currentVersion_none"),
        timeout: 2000,
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
      toast(t("common.error"), {
        variant: "danger",
        description: t("launcherpage.currentVersion_none"),
        timeout: 2000,
      });
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
      batchUpdateOnClose();
      if (failed.length > 0) {
        showActionResult(success, failed);
      } else if (hasSuccess) {
        toast(t("common.success"), {
          variant: "success",
          description: `${t("mods.action_update")} x${success.length}`,
          timeout: 2000,
        });
      }
    } finally {
      setBatchUpdating(false);
    }
  };

  const handleBatchUninstall = async () => {
    const name = activeVersionName;
    if (!name) {
      toast(t("common.error"), {
        variant: "danger",
        description: t("launcherpage.currentVersion_none"),
        timeout: 2000,
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
      batchUninstallOnClose();
      if (failed.length > 0) {
        showActionResult(success, failed);
      } else if (success.length > 0) {
        toast(t("common.success"), {
          variant: "success",
          description: `${t("mods.action_uninstall")} x${success.length}`,
          timeout: 2000,
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
      toast(t("common.error"), {
        variant: "danger",
        description: t("launcherpage.currentVersion_none"),
        timeout: 2000,
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
          feedbackMode: "progress",
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
      toast(t("common.error"), {
        variant: "danger",
        description: t("launcherpage.currentVersion_none"),
        timeout: 2000,
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
          feedbackMode: "progress",
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
      toast(t("common.error"), {
        variant: "danger",
        description: t("launcherpage.currentVersion_none"),
        timeout: 2000,
      });
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
      showActionToast(
        "error",
        t("mods.action_promote_install"),
        group.packageName,
        parseErrorCode(error) || "ERR_LIP_PACKAGE_INSTALL_FAILED",
      );
      return;
    }

    await refreshAll();
    toast(t("common.success"), {
      variant: "success",
      description: t("mods.action_promote_install_success", {
        name: group.packageName,
      }),
      timeout: 2000,
    });
  };

  const handlePromoteLipGroup = async (group: LipGroupItem) => {
    const name = activeVersionName;
    if (!name) {
      toast(t("common.error"), {
        variant: "danger",
        description: t("launcherpage.currentVersion_none"),
        timeout: 2000,
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
      toast(t("common.error"), {
        variant: "danger",
        description: t("launcherpage.currentVersion_none"),
        timeout: 2000,
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
      toast(t("common.error"), {
        variant: "danger",
        description: t("launcherpage.currentVersion_none"),
        timeout: 2000,
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
    lipSyncStatus,
    lipSyncError,
    lipInfoPending,
    lipInfoWarning,
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
