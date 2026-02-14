import React, { useEffect, useMemo, useRef, useState } from "react";
import {
  Button,
  Card,
  CardBody,
  Chip,
  Input,
  useDisclosure,
  Progress,
  Switch,
  Checkbox,
  Dropdown,
  DropdownTrigger,
  DropdownMenu,
  DropdownItem,
} from "@heroui/react";
import { useLocation, useNavigate, useBlocker } from "react-router-dom";
import { DeleteConfirmModal } from "@/components/DeleteConfirmModal";
import { ImportResultModal } from "@/components/ImportResultModal";
import { UnifiedModal } from "@/components/UnifiedModal";
import { useTranslation } from "react-i18next";
import {
  OpenModsExplorer,
  GetMods,
  DeleteMod,
  EnableMod,
  DisableMod,
  IsModEnabled,
  UninstallLeviLamina,
} from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { Dialogs, Events } from "@wailsio/runtime";
import * as types from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import {
  FaPuzzlePiece,
  FaTrash,
  FaEllipsisVertical,
  FaChevronUp,
  FaChevronDown,
  FaBan,
  FaCheck,
} from "react-icons/fa6";
import {
  FaSync,
  FaFilter,
  FaTimes,
  FaFolderOpen,
  FaInfoCircle,
} from "react-icons/fa";
import { FiUploadCloud, FiAlertTriangle } from "react-icons/fi";
import { AnimatePresence, motion } from "framer-motion";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { FileDropOverlay } from "@/components/FileDropOverlay";
import { useFileDrag } from "@/hooks/useFileDrag";
import { PageHeader } from "@/components/PageHeader";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { cn } from "@/utils/cn";
import { COMPONENT_STYLES } from "@/constants/componentStyles";

const readCurrentVersionName = (): string => {
  try {
    return localStorage.getItem("ll.currentVersionName") || "";
  } catch {
    return "";
  }
};

export const ModsPage: React.FC = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation() as any;
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
  const {} = useDisclosure();
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
  const dllResolveRef = useRef<((ok: boolean) => void) | null>(null);
  const dllConfirmRef = useRef<{
    name: string;
    type: string;
    version: string;
  } | null>(null);
  const dupResolveRef = useRef<((overwrite: boolean) => void) | null>(null);
  const dupNameRef = useRef<string>("");
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const isDragActive = useFileDrag(scrollRef as React.RefObject<HTMLElement>);
  const lastScrollTopRef = useRef<number>(0);
  const restorePendingRef = useRef<boolean>(false);

  const [gameVersion, setGameVersion] = useState("");

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

  useEffect(() => {
    const fetchVersion = async () => {
      if (!currentVersionName) return;
      try {
        const meta = await (minecraft as any)?.GetVersionMeta(
          currentVersionName,
        );
        if (meta && meta.GameVersion) {
          setGameVersion(meta.GameVersion);
        } else {
          setGameVersion(currentVersionName);
        }
      } catch {
        setGameVersion(currentVersionName);
      }
    };
    fetchVersion();
  }, [currentVersionName]);

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
          let err = await minecraft?.ImportModZipPath?.(name, p, false);
          if (err) {
            if (String(err) === "ERR_DUPLICATE_FOLDER") {
              dupNameRef.current = base;
              await new Promise<void>((r) => setTimeout(r, 0));
              dupOnOpen();
              const ok = await new Promise<boolean>((resolve) => {
                dupResolveRef.current = resolve;
              });
              if (ok) {
                err = await minecraft?.ImportModZipPath?.(name, p, true);
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
          let err = await minecraft?.ImportModDllPath?.(
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
                err = await minecraft?.ImportModDllPath?.(
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

  useEffect(() => {
    return Events.On("files-dropped", (event) => {
      const data = (event.data as { files: string[] }) || {};
      if (data.files && data.files.length > 0) {
        void doImportRef.current(data.files);
      }
    });
  }, []);

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
        if (modName === "LeviLamina") continue; // Skip LL
        await (DeleteMod as any)?.(name, modName);
      } catch {}
    }
    await refreshAll();
    setSelectedKeys(new Set());
  };

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
      const versionsDir = await (minecraft as any).GetVersionsDir();
      const sep = versionsDir.includes("\\") ? "\\" : "/";
      const modPath = `${versionsDir}${sep}${name}${sep}mods${sep}${mod.name}`;

      try {
        await (minecraft as any).ListDir(modPath);
        await (minecraft as any).OpenPathDir(modPath);
      } catch {
        OpenModsExplorer(name);
      }
    } catch {
      OpenModsExplorer(name);
    }
  };

  return (
    <PageContainer
      ref={scrollRef}
      id="mods-drop-zone"
      {...({ "data-file-drop-target": true } as any)}
      className="relative"
    >
      {/* Drag Overlay */}
      <FileDropOverlay isDragActive={isDragActive} text={t("mods.drop_hint")} />

      <UnifiedModal
        isOpen={importing && !dllOpen}
        onOpenChange={() => {}}
        title={t("mods.importing_title")}
        type="primary"
        icon={<FiUploadCloud className="w-6 h-6 text-primary-500" />}
        hideCloseButton
        isDismissable={false}
        showConfirmButton={false}
      >
        <div className="py-1">
          <Progress isIndeterminate aria-label="importing" className="w-full" />
        </div>
        <div className="text-default-600 dark:text-zinc-300 text-sm mt-2">
          {t("mods.importing_body")}
        </div>
        {currentFile ? (
          <div className="mt-2 rounded-md bg-default-100/60 dark:bg-zinc-800/60 border border-default-200 dark:border-zinc-700 px-3 py-2 text-default-800 dark:text-zinc-100 text-sm wrap-break-word whitespace-pre-wrap font-mono">
            {currentFile}
          </div>
        ) : null}
      </UnifiedModal>

      <ImportResultModal
        isOpen={errOpen}
        onOpenChange={errOnOpenChange}
        results={{ success: resultSuccess, failed: resultFailed }}
        onConfirm={() => {
          setErrorMsg("");
          setErrorFile("");
          setResultSuccess([]);
          setResultFailed([]);
          errOnClose();
        }}
      />

      <ImportResultModal
        isOpen={delOpen}
        onOpenChange={delOnOpenChange}
        results={{ success: resultSuccess, failed: resultFailed }}
        onConfirm={() => {
          setErrorMsg("");
          setErrorFile("");
          setResultSuccess([]);
          setResultFailed([]);
          delOnClose();
        }}
      />

      <UnifiedModal
        isOpen={dllOpen}
        onOpenChange={dllOnOpenChange}
        title={t("mods.dll_modal_title")}
        type="primary"
        icon={<FaPuzzlePiece className="w-6 h-6 text-primary-500" />}
        hideCloseButton
        onConfirm={() => {
          const nm = dllName.trim();
          if (!nm) return;
          const tp = (dllType || "").trim() || "preload-native";
          const ver = (dllVersion || "").trim() || "0.0.0";
          dllConfirmRef.current = {
            name: nm,
            type: tp,
            version: ver,
          };
          try {
            dllResolveRef.current && dllResolveRef.current(true);
          } finally {
            dllOnClose();
          }
        }}
        onCancel={() => {
          try {
            dllConfirmRef.current = null;
            dllResolveRef.current && dllResolveRef.current(false);
          } finally {
            dllOnClose();
          }
        }}
        showCancelButton
        confirmText={t("common.confirm")}
        cancelText={t("common.cancel")}
      >
        <div className="flex flex-col gap-3">
          <Input
            label={t("mods.dll_name") as string}
            value={dllName}
            onValueChange={setDllName}
            autoFocus
            size="sm"
            classNames={COMPONENT_STYLES.input}
          />
          <Input
            label={t("mods.dll_type") as string}
            value={dllType}
            onValueChange={setDllType}
            size="sm"
            classNames={COMPONENT_STYLES.input}
          />
          <Input
            label={t("mods.dll_version") as string}
            value={dllVersion}
            onValueChange={setDllVersion}
            size="sm"
            classNames={COMPONENT_STYLES.input}
          />
        </div>
      </UnifiedModal>

      <UnifiedModal
        isOpen={dupOpen}
        onOpenChange={dupOnOpenChange}
        title={t("mods.overwrite_modal_title")}
        type="warning"
        hideCloseButton
        onConfirm={() => {
          try {
            dupResolveRef.current && dupResolveRef.current(true);
          } finally {
            dupOnClose();
          }
        }}
        onCancel={() => {
          try {
            dupResolveRef.current && dupResolveRef.current(false);
          } finally {
            dupOnClose();
          }
        }}
        showCancelButton
        confirmText={t("common.confirm")}
        cancelText={t("common.cancel")}
      >
        <div className="text-sm text-default-700 dark:text-zinc-300">
          {t("mods.overwrite_modal_body")}
        </div>
        {dupNameRef.current ? (
          <div className="mt-2 rounded-md bg-default-100/60 dark:bg-zinc-800/60 border border-default-200 dark:border-zinc-700 px-3 py-2 text-default-800 dark:text-zinc-100 text-sm wrap-break-word whitespace-pre-wrap">
            {dupNameRef.current}
          </div>
        ) : null}
      </UnifiedModal>

      {/* Drag Overlay */}
      <Card className={cn("shrink-0", LAYOUT.GLASS_CARD.BASE)}>
        <CardBody className="p-6 flex flex-col gap-6">
          <PageHeader
            title={t("moddedcard.title")}
            description={
              <div className="flex items-center gap-2">
                <span>{currentVersionName || "No Version Selected"}</span>
                {modsInfo.length > 0 && (
                  <Chip
                    size="sm"
                    variant="flat"
                    className="h-5 text-xs bg-default-100 dark:bg-zinc-800"
                  >
                    {modsInfo.length}
                  </Chip>
                )}
              </div>
            }
            endContent={
              <>
                <Button
                  color="primary"
                  variant="shadow"
                  className="bg-primary-600 hover:bg-primary-500 text-white shadow-lg shadow-primary-900/20"
                  startContent={<FiUploadCloud />}
                  onPress={async () => {
                    try {
                      const result = await Dialogs.OpenFile({
                        Filters: [
                          { DisplayName: "Mod Files", Pattern: "*.zip;*.dll" },
                        ],
                        AllowsMultipleSelection: true,
                        Title: t("mods.import_button"),
                      });
                      if (result && result.length > 0) {
                        void doImportFromPaths(result);
                      }
                    } catch (e) {
                      console.error(e);
                    }
                  }}
                  isDisabled={importing}
                >
                  {t("mods.import_button")}
                </Button>
                <Button
                  variant="flat"
                  className="bg-default-100 dark:bg-zinc-800"
                  onPress={openFolder}
                >
                  {t("downloadmodal.open_folder")}
                </Button>
              </>
            }
          />
        </CardBody>
      </Card>

      <div className="flex flex-col sm:flex-row items-center gap-3 px-2">
        <Input
          placeholder={t("common.search_placeholder")}
          value={query}
          onValueChange={setQuery}
          startContent={<FaFilter className="text-default-400" />}
          endContent={
            query && (
              <button onClick={() => setQuery("")}>
                <FaTimes className="text-default-400 hover:text-default-600" />
              </button>
            )
          }
          radius="full"
          variant="flat"
          className="w-full sm:max-w-xs"
          classNames={COMPONENT_STYLES.input}
        />
        <div className="w-px h-6 bg-default-200 dark:bg-white/10 hidden sm:block" />
        <Checkbox
          size="sm"
          isSelected={onlyEnabled}
          onValueChange={setOnlyEnabled}
          classNames={{
            base: "m-0",
            label: "text-default-500 dark:text-zinc-400",
          }}
        >
          {t("mods.only_enabled") as string}
        </Checkbox>
      </div>

      <div className="flex-1 overflow-hidden flex flex-col">
        <div className="flex flex-row items-center px-5 py-2 text-sm text-default-500 dark:text-zinc-400 font-semibold">
          <div className="w-10 flex justify-center">
            <Checkbox
              size="sm"
              classNames={{ wrapper: "after:bg-primary-500" }}
              isSelected={
                sortedItems.length > 0 &&
                selectedKeys.size === sortedItems.length
              }
              isIndeterminate={
                selectedKeys.size > 0 && selectedKeys.size < sortedItems.length
              }
              onValueChange={onSelectAll}
            />
          </div>
          {selectedKeys.size > 0 ? (
            <div className="flex-1 flex items-center gap-3 animate-in fade-in slide-in-from-left-2 duration-200">
              <Button
                size="sm"
                variant="flat"
                className="bg-default-100 dark:bg-zinc-800 h-8 min-w-0 px-3"
                startContent={<FaCheck />}
                onPress={handleBatchEnable}
              >
                {t("common.enable")}
              </Button>
              <Button
                size="sm"
                variant="flat"
                className="bg-default-100 dark:bg-zinc-800 h-8 min-w-0 px-3"
                startContent={<FaBan />}
                onPress={handleBatchDisable}
              >
                {t("common.disable")}
              </Button>
              <Button
                size="sm"
                color="danger"
                variant="flat"
                className="bg-danger-500/10 text-danger h-8 min-w-0 px-3"
                startContent={<FaTrash />}
                onPress={handleBatchRemove}
              >
                {t("common.remove")}
              </Button>
            </div>
          ) : (
            <>
              <div
                className="flex-1 cursor-pointer flex items-center gap-1 hover:text-default-700 dark:hover:text-zinc-300 transition-colors select-none"
                onClick={() => handleSort("name")}
              >
                {t("mods.field_name")}{" "}
                {sortConfig.key === "name" &&
                  (sortConfig.direction === "asc" ? (
                    <FaChevronUp className="text-xs" />
                  ) : (
                    <FaChevronDown className="text-xs" />
                  ))}
              </div>
              <div className="flex-[0.6]">{t("common.version")}</div>
              <div className="flex items-center gap-6 text-xs">
                <button
                  className="flex items-center gap-1.5 hover:text-default-700 dark:hover:text-zinc-300 transition-colors"
                  onClick={() => refreshAll()}
                  disabled={loading}
                >
                  <FaSync className={loading ? "animate-spin" : ""} />
                  {t("common.refresh")}
                </button>
              </div>
            </>
          )}
        </div>

        <div className="flex-1 overflow-y-auto p-2 custom-scrollbar flex flex-col gap-2">
          {!currentVersionName ? (
            <div className="flex flex-col items-center justify-center h-full text-default-400 gap-2">
              <FiAlertTriangle className="w-8 h-8 opacity-50" />
              <p>{t("launcherpage.currentVersion_none")}</p>
            </div>
          ) : sortedItems.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full text-default-400 gap-2">
              <FaPuzzlePiece className="w-8 h-8 opacity-50" />
              <p>{t("moddedcard.content.none")}</p>
            </div>
          ) : (
            sortedItems.map((m, idx) => {
              return (
                <div
                  key={`${m.name}-${m.version}-${idx}`}
                  className={`flex flex-row items-center p-3 rounded-2xl bg-white/60 dark:bg-zinc-800/40 hover:bg-white/80 dark:hover:bg-zinc-800/80 border transition-all gap-4 ${
                    selectedKeys.has(m.name)
                      ? "border-primary-500/50 dark:border-primary-500/30 bg-primary-50/50 dark:bg-primary-900/10"
                      : "border-white/40 dark:border-white/5"
                  }`}
                >
                  <div
                    className="w-10 flex justify-center"
                    onClick={(e) => e.stopPropagation()}
                  >
                    <Checkbox
                      size="sm"
                      color="success"
                      isSelected={selectedKeys.has(m.name)}
                      onValueChange={() => onSelectionChange(m.name)}
                    />
                  </div>

                  <div className="w-12 h-12 rounded-xl bg-default-100 dark:bg-zinc-900 flex items-center justify-center text-default-500 dark:text-zinc-400 shrink-0">
                    <FaPuzzlePiece className="w-6 h-6" />
                  </div>

                  <div className="flex-1 min-w-0 flex flex-col justify-center">
                    <div className="font-bold text-default-900 dark:text-zinc-100 truncate text-base">
                      {m.name}
                    </div>
                    <div className="text-xs text-default-500 dark:text-zinc-400 truncate">
                      by {m.author || "Unknown"}
                    </div>
                  </div>

                  <div className="flex-[0.6] min-w-0 flex flex-col justify-center">
                    <div className="text-default-700 dark:text-zinc-300 truncate text-sm">
                      {m.version || "-"}
                    </div>
                    <div className="text-xs text-default-500 dark:text-zinc-400 truncate font-mono opacity-70">
                      {m.entry || m.type}
                    </div>
                  </div>

                  <div
                    className="flex items-center gap-3 pr-2"
                    onClick={(e) => e.stopPropagation()}
                  >
                    <Switch
                      size="sm"
                      isSelected={!!enabledByName.get(m.name)}
                      classNames={{
                        wrapper: "group-data-[selected=true]:bg-primary-500",
                      }}
                      onValueChange={async (val) => {
                        const name =
                          currentVersionName || readCurrentVersionName();
                        if (!name) return;
                        try {
                          if (val) {
                            const err = await (EnableMod as any)?.(
                              name,
                              m.name,
                            );
                            if (err) return;
                          } else {
                            const err = await (DisableMod as any)?.(
                              name,
                              m.name,
                            );
                            if (err) return;
                          }
                          const ok = await (IsModEnabled as any)?.(
                            name,
                            m.name,
                          );
                          setEnabledByName((prev) => {
                            const nm = new Map(prev);
                            nm.set(m.name, !!ok);
                            return nm;
                          });
                        } catch {}
                      }}
                      aria-label={t("mods.toggle_label") as string}
                    />

                    <Button
                      isIconOnly
                      size="sm"
                      variant="light"
                      className="text-default-400 hover:text-danger transition-opacity"
                      onPress={() => {
                        setActiveMod(m);
                        delCfmOnOpen();
                      }}
                    >
                      <FaTrash />
                    </Button>

                    <Dropdown classNames={COMPONENT_STYLES.dropdown}>
                      <DropdownTrigger>
                        <Button
                          isIconOnly
                          size="sm"
                          variant="light"
                          className="text-default-400"
                        >
                          <FaEllipsisVertical />
                        </Button>
                      </DropdownTrigger>
                      <DropdownMenu aria-label="Mod Actions">
                        <DropdownItem
                          key="folder"
                          startContent={<FaFolderOpen />}
                          onPress={() => openModFolder(m)}
                        >
                          {t("common.open_folder")}
                        </DropdownItem>
                        <DropdownItem
                          key="details"
                          startContent={<FaInfoCircle />}
                          onPress={() => openDetails(m)}
                        >
                          {t("common.details")}
                        </DropdownItem>
                      </DropdownMenu>
                    </Dropdown>
                  </div>
                </div>
              );
            })
          )}
        </div>
      </div>

      <UnifiedModal
        isOpen={infoOpen}
        onOpenChange={infoOnOpenChange}
        title={t("mods.details_title")}
        type="primary"
        icon={<FaPuzzlePiece className="w-6 h-6 text-primary-500" />}
        hideCloseButton
        showConfirmButton={false}
        showCancelButton
        cancelText={t("common.close")}
        onCancel={() => infoOnClose()}
        footer={
          <>
            <Button variant="light" onPress={() => infoOnClose()}>
              {t("common.cancel")}
            </Button>
            <Button color="danger" onPress={delCfmOnOpen}>
              {t("common.delete")}
            </Button>
          </>
        }
      >
        {activeMod ? (
          <div className="space-y-2 text-sm dark:text-zinc-200">
            <div>
              <span className="text-default-500 dark:text-zinc-400">
                {t("mods.field_name")}：
              </span>
              {activeMod.name}
            </div>
            <div>
              <span className="text-default-500 dark:text-zinc-400">
                {t("mods.field_version")}：
              </span>
              {activeMod.version || "-"}
            </div>
            <div>
              <span className="text-default-500 dark:text-zinc-400">
                {t("mods.field_type")}：
              </span>
              {activeMod.type || "-"}
            </div>
            <div>
              <span className="text-default-500 dark:text-zinc-400">
                {t("mods.field_entry")}：
              </span>
              {activeMod.entry || "-"}
            </div>
            {activeMod.author ? (
              <div>
                <span className="text-default-500 dark:text-zinc-400">
                  {t("mods.field_author")}：
                </span>
                {activeMod.author}
              </div>
            ) : null}
            <div className="pt-2">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="text-default-500 dark:text-zinc-400">
                    {t("mods.toggle_label")}
                  </span>
                  <Chip
                    size="sm"
                    variant="flat"
                    color={
                      enabledByName.get(activeMod.name) ? "success" : "warning"
                    }
                  >
                    {enabledByName.get(activeMod.name)
                      ? (t("mods.toggle_on") as string)
                      : (t("mods.toggle_off") as string)}
                  </Chip>
                </div>
                <Switch
                  isSelected={!!enabledByName.get(activeMod.name)}
                  classNames={{
                    wrapper: "group-data-[selected=true]:bg-primary-500",
                  }}
                  onValueChange={async (val) => {
                    const name = currentVersionName || readCurrentVersionName();
                    if (!name) return;
                    try {
                      if (val) {
                        const err = await (EnableMod as any)?.(
                          name,
                          activeMod.name,
                        );
                        if (err) return;
                      } else {
                        const err = await (DisableMod as any)?.(
                          name,
                          activeMod.name,
                        );
                        if (err) return;
                      }
                      const ok = await (IsModEnabled as any)?.(
                        name,
                        activeMod.name,
                      );
                      setEnabledByName((prev) => {
                        const nm = new Map(prev);
                        nm.set(activeMod.name, !!ok);
                        return nm;
                      });
                    } catch {}
                  }}
                  aria-label={t("mods.toggle_label") as string}
                />
              </div>
              <div className="text-default-500 dark:text-zinc-400 text-xs mt-1">
                {enabledByName.get(activeMod.name)
                  ? (t("mods.toggle_desc_on") as string)
                  : (t("mods.toggle_desc_off") as string)}
              </div>
            </div>
          </div>
        ) : null}
      </UnifiedModal>
      <DeleteConfirmModal
        isOpen={delCfmOpen}
        onOpenChange={delCfmOnOpenChange}
        onConfirm={handleDeleteMod}
        title={t("mods.confirm_delete_title")}
        description={t("mods.confirm_delete_body", {
          type: t("moddedcard.title"),
        })}
        itemName={activeMod?.name}
        isPending={deleting}
      />
    </PageContainer>
  );
};

export default ModsPage;
