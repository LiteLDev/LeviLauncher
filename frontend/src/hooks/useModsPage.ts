import React, { useEffect, useMemo, useRef, useState } from "react";
import { useDisclosure } from "@heroui/react";
import { useLocation, useNavigate, useBlocker } from "react-router-dom";
import { Events } from "@wailsio/runtime";
import {
  ListDir,
  OpenPathDir,
  UninstallLeviLamina,
} from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import {
  GetVersionMeta,
  GetVersionsDir,
} from "bindings/github.com/liteldev/LeviLauncher/versionservice";
import {
  OpenModsExplorer,
  GetMods,
  DeleteMod,
  EnableMod,
  DisableMod,
  IsModEnabled,
  ImportModZipPath,
  ImportModDllPath,
} from "bindings/github.com/liteldev/LeviLauncher/modsservice";
import * as types from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";

const readCurrentVersionName = (): string => {
  try {
    return localStorage.getItem("ll.currentVersionName") || "";
  } catch {
    return "";
  }
};

type TFunc = (key: string, opts?: Record<string, unknown>) => string;

export const useModsPage = (
  t: TFunc,
  scrollRef: React.RefObject<HTMLDivElement | null>,
) => {
  const navigate = useNavigate();
  const location = useLocation() as any;

  // --- State ---
  const [currentVersionName, setCurrentVersionName] = useState<string>("");
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
  const [deleting, setDeleting] = useState<boolean>(false);
  const [enabledByName, setEnabledByName] = useState<Map<string, boolean>>(
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
  const [gameVersion, setGameVersion] = useState("");

  // --- Refs ---
  const dllResolveRef = useRef<((ok: boolean) => void) | null>(null);
  const dllConfirmRef = useRef<{
    name: string;
    type: string;
    version: string;
  } | null>(null);
  const dupResolveRef = useRef<((overwrite: boolean) => void) | null>(null);
  const dupNameRef = useRef<string>("");
  const lastScrollTopRef = useRef<number>(0);
  const restorePendingRef = useRef<boolean>(false);

  // --- Disclosures ---
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

  // --- Navigation blocking ---
  useBlocker(() => importing);

  useEffect(() => {
    const lock = importing;
    window.dispatchEvent(
      new CustomEvent("ll-nav-lock-changed", { detail: { lock } }),
    );
    return () => {
      if (lock) {
        window.dispatchEvent(
          new CustomEvent("ll-nav-lock-changed", { detail: { lock: false } }),
        );
      }
    };
  }, [importing]);

  // --- Game version fetch ---
  useEffect(() => {
    const fetchVersion = async () => {
      if (!currentVersionName) return;
      try {
        const meta = await GetVersionMeta(currentVersionName);
        if (meta && meta.gameVersion) {
          setGameVersion(meta.gameVersion);
        } else {
          setGameVersion(currentVersionName);
        }
      } catch {
        setGameVersion(currentVersionName);
      }
    };
    fetchVersion();
  }, [currentVersionName]);

  // --- Refresh helpers ---
  const refreshEnabledStates = async (name: string) => {
    try {
      const list = await GetMods(name);
      const m = new Map<string, boolean>();
      for (const it of list || []) {
        const ok = await (IsModEnabled as any)?.(name, it.name);
        m.set(it.name, !!ok);
      }
      setEnabledByName(m);
    } catch {
      setEnabledByName(new Map());
    }
  };

  const refreshModsAndStates = async (name: string) => {
    for (let i = 0; i < 4; i++) {
      try {
        const data = await GetMods(name);
        setModsInfo(data || []);
        await refreshEnabledStates(name);
      } catch {
        setModsInfo([]);
      }
      await new Promise((r) => setTimeout(r, 250));
    }
  };

  const refreshAll = async () => {
    setLoading(true);
    const name = currentVersionName || readCurrentVersionName();
    if (name) {
      try {
        const data = await GetMods(name);
        setModsInfo(data || []);
        await refreshEnabledStates(name);
      } catch {
        setModsInfo([]);
      }
    }
    setLoading(false);
  };

  // --- Import ---
  const doImportFromPaths = async (paths: string[]) => {
    try {
      if (!paths?.length) return;
      const name = currentVersionName || readCurrentVersionName();
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
              const ok = await new Promise<boolean>((resolve) => {
                dupResolveRef.current = resolve;
              });
              if (ok) {
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
      const data = await GetMods(name);
      setModsInfo(data || []);
      void refreshEnabledStates(name);
      setResultSuccess(succFiles);
      setResultFailed(errPairs);
      if (succFiles.length > 0 || errPairs.length > 0) {
        errOnOpen();
      }
      await refreshEnabledStates(currentVersionName);
    } catch (e: any) {
      setErrorMsg(String(e?.message || e || "IMPORT_ERROR"));
    } finally {
      setImporting(false);
      setCurrentFile("");
    }
  };

  const doImportRef = useRef(doImportFromPaths);
  doImportRef.current = doImportFromPaths;

  // --- Initial load ---
  useEffect(() => {
    const name = readCurrentVersionName();
    if (!name) {
      navigate("/versions", { replace: true });
      return;
    }
    setCurrentVersionName(name);
    GetMods(name)
      .then((data) => setModsInfo(data || []))
      .catch(() => setModsInfo([]));
    void refreshEnabledStates(name);
  }, []);

  // --- Version polling ---
  useEffect(() => {
    const id = window.setInterval(() => {
      const name = readCurrentVersionName();
      if (name !== currentVersionName) {
        setCurrentVersionName(name);
        if (name) {
          GetMods(name)
            .then((data) => setModsInfo(data || []))
            .catch(() => setModsInfo([]));
          void refreshEnabledStates(name);
        } else {
          setModsInfo([]);
          setEnabledByName(new Map());
        }
      }
    }, 1000);
    return () => window.clearInterval(id);
  }, [currentVersionName]);

  // --- File drop events ---
  useEffect(() => {
    return Events.On("files-dropped", (event) => {
      const data = (event.data as { files: string[] }) || {};
      if (data.files && data.files.length > 0) {
        void doImportRef.current(data.files);
      }
    });
  }, []);

  // --- Scroll restoration after mod list change ---
  useEffect(() => {
    if (!restorePendingRef.current) return;
    requestAnimationFrame(() => {
      try {
        if (scrollRef.current)
          scrollRef.current.scrollTop = lastScrollTopRef.current;
      } catch {}
    });
    restorePendingRef.current = false;
  }, [modsInfo]);

  // --- Filtering and sorting ---
  const filtered = useMemo(() => {
    let list = modsInfo || [];
    if (onlyEnabled) {
      const has = enabledByName && enabledByName.size > 0;
      if (has) list = list.filter((m) => !!enabledByName.get(m.name));
    }
    const q = query.trim().toLowerCase();
    if (!q) return list;
    return list.filter(
      (m) =>
        `${m.name}`.toLowerCase().includes(q) ||
        `${m.version}`.toLowerCase().includes(q),
    );
  }, [modsInfo, query, onlyEnabled, enabledByName]);

  const sortedItems = useMemo(() => {
    const items = [...filtered];
    if (sortConfig.key === "name") {
      items.sort((a, b) => {
        const nameA = a.name.toLowerCase();
        const nameB = b.name.toLowerCase();
        if (nameA < nameB) return sortConfig.direction === "asc" ? -1 : 1;
        if (nameA > nameB) return sortConfig.direction === "asc" ? 1 : -1;
        return 0;
      });
    }
    return items;
  }, [filtered, sortConfig]);

  const handleSort = (key: string) => {
    let direction: "asc" | "desc" = "asc";
    if (sortConfig.key === key && sortConfig.direction === "asc") {
      direction = "desc";
    }
    setSortConfig({ key, direction });
  };

  // --- Selection ---
  const onSelectionChange = (name: string) => {
    setSelectedKeys((prev) => {
      const newSet = new Set(prev);
      if (newSet.has(name)) newSet.delete(name);
      else newSet.add(name);
      return newSet;
    });
  };

  const onSelectAll = (val: boolean) => {
    if (val) {
      setSelectedKeys(new Set(sortedItems.map((i) => i.name)));
    } else {
      setSelectedKeys(new Set());
    }
  };

  // --- Batch operations ---
  const handleBatchEnable = async () => {
    const name = currentVersionName || readCurrentVersionName();
    if (!name) return;
    for (const modName of selectedKeys) {
      try {
        await (EnableMod as any)?.(name, modName);
      } catch {}
    }
    await refreshEnabledStates(name);
  };

  const handleBatchDisable = async () => {
    const name = currentVersionName || readCurrentVersionName();
    if (!name) return;
    for (const modName of selectedKeys) {
      try {
        await (DisableMod as any)?.(name, modName);
      } catch {}
    }
    await refreshEnabledStates(name);
  };

  const handleBatchRemove = async () => {
    const name = currentVersionName || readCurrentVersionName();
    if (!name) return;
    for (const modName of selectedKeys) {
      try {
        if (modName === "LeviLamina") continue;
        await (DeleteMod as any)?.(name, modName);
      } catch {}
    }
    await refreshAll();
    setSelectedKeys(new Set());
  };

  // --- Mod actions ---
  const openDetails = (m: types.ModInfo) => {
    setActiveMod(m);
    infoOnOpen();
  };

  const handleDeleteMod = async () => {
    if (!activeMod) return;
    const name = currentVersionName || readCurrentVersionName();
    if (!name) {
      setErrorMsg(t("launcherpage.currentVersion_none") as string);
      errOnOpen();
      return;
    }
    const pos = scrollRef.current?.scrollTop || 0;
    setDeleting(true);
    lastScrollTopRef.current = pos;
    restorePendingRef.current = true;
    let err = "";
    if (activeMod.name === "LeviLamina") {
      err = await (UninstallLeviLamina as any)?.(name);
    } else {
      err = await (DeleteMod as any)?.(name, activeMod.name);
    }
    if (err) {
      setResultSuccess([]);
      setResultFailed([{ name: activeMod.name, err }]);
      delOnOpen();
      setDeleting(false);
      return;
    }
    const data = await GetMods(name);
    setModsInfo(data || []);
    await refreshEnabledStates(name);
    void refreshEnabledStates(name);
    requestAnimationFrame(() => {
      try {
        if (scrollRef.current) scrollRef.current.scrollTop = pos;
      } catch {}
    });
    setResultSuccess([activeMod.name]);
    setResultFailed([]);
    infoOnClose();
    delOnOpen();
    setDeleting(false);
  };

  const openFolder = () => {
    const name = currentVersionName;
    if (!name) {
      navigate("/versions");
      return;
    }
    OpenModsExplorer(name);
  };

  const openModFolder = async (mod: types.ModInfo) => {
    const name = currentVersionName;
    if (!name) return;

    try {
      const versionsDir = await GetVersionsDir();
      const sep = versionsDir.includes("\\") ? "\\" : "/";
      const modPath = `${versionsDir}${sep}${name}${sep}mods${sep}${mod.name}`;

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

  // --- Toggle mod enabled state (used inline in JSX) ---
  const toggleModEnabled = async (modName: string, val: boolean) => {
    const name = currentVersionName || readCurrentVersionName();
    if (!name) return;
    try {
      if (val) {
        const err = await (EnableMod as any)?.(name, modName);
        if (err) return;
      } else {
        const err = await (DisableMod as any)?.(name, modName);
        if (err) return;
      }
      const ok = await (IsModEnabled as any)?.(name, modName);
      setEnabledByName((prev) => {
        const nm = new Map(prev);
        nm.set(modName, !!ok);
        return nm;
      });
    } catch {}
  };

  return {
    // state
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
    deleting,
    enabledByName,
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
    gameVersion,

    // refs
    dllResolveRef,
    dllConfirmRef,
    dupResolveRef,
    dupNameRef,
    lastScrollTopRef,
    restorePendingRef,

    // disclosures
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

    // derived data
    filtered,
    sortedItems,

    // handlers
    refreshAll,
    refreshEnabledStates,
    refreshModsAndStates,
    doImportFromPaths,
    handleSort,
    onSelectionChange,
    onSelectAll,
    handleBatchEnable,
    handleBatchDisable,
    handleBatchRemove,
    openDetails,
    handleDeleteMod,
    openFolder,
    openModFolder,
    toggleModEnabled,

    // navigation
    navigate,
  };
};
