import React, { useEffect, useMemo, useRef, useState } from "react";
import {
  Button,
  Card,
  CardBody,
  CardHeader,
  Chip,
  Input,
  useDisclosure,
  Progress,
  Switch,
  Tooltip,
  Checkbox,
  ModalContent,
} from "@heroui/react";
import { useLocation, useNavigate } from "react-router-dom";
import {
  BaseModal,
  BaseModalHeader,
  BaseModalBody,
  BaseModalFooter,
} from "@/components/BaseModal";
import { useTranslation } from "react-i18next";
import {
  OpenModsExplorer,
  GetMods,
  DeleteMod,
  EnableMod,
  DisableMod,
  IsModEnabled,
} from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import * as types from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import { FaPuzzlePiece } from "react-icons/fa6";
import { FaSync, FaFilter, FaTimes } from "react-icons/fa";
import { FiUploadCloud, FiAlertTriangle } from "react-icons/fi";
import { AnimatePresence, motion } from "framer-motion";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { PageHeader } from "@/components/PageHeader";

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
  const [dragActive, setDragActive] = useState(false);
  const dragCounter = useRef(0);
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
  const dllFileRef = useRef<File | null>(null);
  const dllBytesRef = useRef<number[] | null>(null);
  const dllResolveRef = useRef<((ok: boolean) => void) | null>(null);
  const dllConfirmRef = useRef<{
    name: string;
    type: string;
    version: string;
  } | null>(null);
  const dupResolveRef = useRef<((overwrite: boolean) => void) | null>(null);
  const dupNameRef = useRef<string>("");
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const fmProcessedRef = useRef<string | null>(null);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const lastScrollTopRef = useRef<number>(0);
  const restorePendingRef = useRef<boolean>(false);

  const postImportModZip = async (
    name: string,
    file: File,
    overwrite: boolean,
  ): Promise<string> => {
    try {
      const buf = await file.arrayBuffer();
      const bytes = Array.from(new Uint8Array(buf));
      const err = await (minecraft as any)?.ImportModZip?.(
        name,
        bytes,
        overwrite,
      );
      return String(err || "");
    } catch (e: any) {
      return String(e?.message || "IMPORT_ERROR");
    }
  };
  const postImportModDll = async (
    name: string,
    file: File,
    fileName: string,
    modName: string,
    modType: string,
    version: string,
    overwrite: boolean,
  ): Promise<string> => {
    try {
      const buf = await file.arrayBuffer();
      const bytes = Array.from(new Uint8Array(buf));
      const err = await (minecraft as any)?.ImportModDll?.(
        name,
        fileName,
        bytes,
        modName,
        modType,
        version,
        overwrite,
      );
      return String(err || "");
    } catch (e: any) {
      return String(e?.message || "IMPORT_ERROR");
    }
  };

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

  const resolveImportError = (err: string): string => {
    const code = String(err || "").trim();
    switch (code) {
      case "ERR_INVALID_NAME":
        return t("mods.err_invalid_name", {
          defaultValue: "无效的版本名或模块名",
        }) as string;
      case "ERR_ACCESS_VERSIONS_DIR":
        return t("mods.err_access_versions_dir", {
          defaultValue: "无法访问版本目录",
        }) as string;
      case "ERR_CREATE_TARGET_DIR":
        return t("mods.err_create_target_dir", {
          defaultValue: "创建目标目录失败",
        }) as string;
      case "ERR_OPEN_ZIP":
        return t("mods.err_open_zip", {
          defaultValue: "无法打开ZIP文件",
        }) as string;
      case "ERR_MANIFEST_NOT_FOUND":
        return t("mods.err_manifest_not_found", {
          defaultValue: "未找到 manifest.json",
        }) as string;
      case "ERR_INVALID_PACKAGE":
        return t("mods.err_invalid_package", {
          defaultValue: "无效的包结构",
        }) as string;
      case "ERR_DUPLICATE_FOLDER":
        return t("mods.err_duplicate_folder", {
          defaultValue: "同名模块文件夹已存在",
        }) as string;
      case "ERR_READ_ZIP_ENTRY":
        return t("mods.err_read_zip_entry", {
          defaultValue: "读取压缩包条目失败",
        }) as string;
      case "ERR_WRITE_FILE":
        return t("mods.err_write_file", {
          defaultValue: "写入文件失败",
        }) as string;
      default:
        return (
          code ||
          (t("mods.err_unknown", { defaultValue: "未知错误" }) as string)
        );
    }
  };

  useEffect(() => {
    const result: string[] | undefined = location?.state?.fileManagerResult;
    if (!result || !Array.isArray(result) || result.length === 0) return;
    const sig = result.join("|");
    if (fmProcessedRef.current === sig) return;
    fmProcessedRef.current = sig;
    navigate(location.pathname, {
      replace: true,
      state: { ...(location.state || {}), fileManagerResult: undefined },
    });
    void doImportFromPaths(result);
  }, [location?.state?.fileManagerResult]);

  const doImportFromPaths = async (paths: string[]) => {
    try {
      if (!paths?.length) return;
      const name = currentVersionName || readCurrentVersionName();
      if (!name) {
        setErrorMsg(
          t("launcherpage.currentVersion_none", {
            defaultValue: "未选择版本",
          }) as string,
        );
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

  const handleFilePick = async (e: React.ChangeEvent<HTMLInputElement>) => {
    try {
      setErrorMsg("");
      setErrorFile("");
      setResultSuccess([]);
      setResultFailed([]);
      const list = e.target.files;
      if (!list || list.length === 0) return;
      if (!currentVersionName) {
        setErrorMsg(
          t("launcherpage.currentVersion_none", {
            defaultValue: "未选择版本",
          }) as string,
        );
        return;
      }
      let started = false;
      const succFiles: string[] = [];
      const errPairs: Array<{ name: string; err: string }> = [];
      const files: File[] = Array.from(list).filter(
        (f) =>
          f &&
          (f.name.toLowerCase().endsWith(".zip") ||
            f.name.toLowerCase().endsWith(".dll")),
      );
      for (const f of files) {
        const lower = f.name.toLowerCase();
        if (lower.endsWith(".zip")) {
          if (!started) {
            setImporting(true);
            started = true;
          }
          setCurrentFile(f.name);
          await new Promise<void>((r) => setTimeout(r, 0));
          let err = await postImportModZip(currentVersionName, f, false);
          if (err) {
            if (String(err) === "ERR_DUPLICATE_FOLDER") {
              dupNameRef.current = f.name;
              await new Promise<void>((r) => setTimeout(r, 0));
              dupOnOpen();
              const ok = await new Promise<boolean>((resolve) => {
                dupResolveRef.current = resolve;
              });
              if (ok) {
                err = await postImportModZip(currentVersionName, f, true);
                if (!err) {
                  succFiles.push(f.name);
                  continue;
                }
              } else {
                continue;
              }
            }
            errPairs.push({ name: f.name, err });
            continue;
          }
          succFiles.push(f.name);
        } else if (lower.endsWith(".dll")) {
          const base = f.name.replace(/\.[^/.]+$/, "");
          setDllName(base);
          setDllType("preload-native");
          setDllVersion("0.0.0");
          dllFileRef.current = f;
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
          setCurrentFile(f.name);
          await new Promise<void>((r) => setTimeout(r, 0));
          const vals = dllConfirmRef.current || {
            name: dllName,
            type: dllType,
            version: dllVersion,
          };
          dllConfirmRef.current = null;
          let err = await postImportModDll(
            currentVersionName,
            dllFileRef.current || f,
            f.name,
            vals.name,
            vals.type || "preload-native",
            vals.version || "0.0.0",
            false,
          );
          if (err) {
            if (String(err) === "ERR_DUPLICATE_FOLDER") {
              dupNameRef.current = f.name;
              await new Promise<void>((r) => setTimeout(r, 0));
              dllOnClose();
              dupOnOpen();
              const ok = await new Promise<boolean>((resolve) => {
                dupResolveRef.current = resolve;
              });
              if (ok) {
                err = await postImportModDll(
                  currentVersionName,
                  dllFileRef.current || f,
                  f.name,
                  vals.name,
                  vals.type || "preload-native",
                  vals.version || "0.0.0",
                  true,
                );
                if (!err) {
                  succFiles.push(f.name);
                  continue;
                }
              } else {
                continue;
              }
            }
            errPairs.push({ name: f.name, err });
            continue;
          }
          succFiles.push(f.name);
        }
      }
      await refreshModsAndStates(currentVersionName);
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
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  };

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

  const openDetails = (m: types.ModInfo) => {
    setActiveMod(m);
    infoOnOpen();
  };

  const handleDeleteMod = async () => {
    if (!activeMod) return;
    const name = currentVersionName || readCurrentVersionName();
    if (!name) {
      setErrorMsg(
        t("launcherpage.currentVersion_none", {
          defaultValue: "未选择版本",
        }) as string,
      );
      errOnOpen();
      return;
    }
    const pos = scrollRef.current?.scrollTop || 0;
    setDeleting(true);
    lastScrollTopRef.current = pos;
    restorePendingRef.current = true;
    const err = await (DeleteMod as any)?.(name, activeMod.name);
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

  const handleDrop = async (e: DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    dragCounter.current = 0;
    setDragActive(false);
    setErrorMsg("");
    setErrorFile("");
    setResultSuccess([]);
    setResultFailed([]);
    const files: File[] = Array.from(e.dataTransfer?.files || []).filter(
      (f) =>
        f &&
        (f.name.toLowerCase().endsWith(".zip") ||
          f.name.toLowerCase().endsWith(".dll")),
    );
    if (!files.length) return;
    let started = false;
    const succFiles: string[] = [];
    const errPairs: Array<{ name: string; err: string }> = [];
    try {
      for (const f of files) {
        const lower = f.name.toLowerCase();
        if (lower.endsWith(".zip")) {
          if (!started) {
            setImporting(true);
            started = true;
          }
          setCurrentFile(f.name);
          await new Promise<void>((r) => setTimeout(r, 0));
          let err = await postImportModZip(currentVersionName, f, false);
          if (err) {
            if (String(err) === "ERR_DUPLICATE_FOLDER") {
              dupNameRef.current = f.name;
              await new Promise<void>((r) => setTimeout(r, 0));
              dupOnOpen();
              const ok = await new Promise<boolean>((resolve) => {
                dupResolveRef.current = resolve;
              });
              if (ok) {
                err = await postImportModZip(currentVersionName, f, true);
                if (!err) {
                  succFiles.push(f.name);
                  continue;
                }
              }
            }
            errPairs.push({ name: f.name, err });
            continue;
          }
          succFiles.push(f.name);
        } else if (lower.endsWith(".dll")) {
          const base = f.name.replace(/\.[^/.]+$/, "");
          setDllName(base);
          setDllType("preload-native");
          setDllVersion("0.0.0");
          dllFileRef.current = f;
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
          setCurrentFile(f.name);
          await new Promise<void>((r) => setTimeout(r, 0));
          const vals = dllConfirmRef.current || {
            name: dllName,
            type: dllType,
            version: dllVersion,
          };
          dllConfirmRef.current = null;
          let err = await postImportModDll(
            currentVersionName,
            dllFileRef.current || f,
            f.name,
            vals.name,
            vals.type || "preload-native",
            vals.version || "0.0.0",
            false,
          );
          if (err) {
            if (String(err) === "ERR_DUPLICATE_FOLDER") {
              dupNameRef.current = f.name;
              await new Promise<void>((r) => setTimeout(r, 0));
              dllOnClose();
              dupOnOpen();
              const ok = await new Promise<boolean>((resolve) => {
                dupResolveRef.current = resolve;
              });
              if (ok) {
                err = await postImportModDll(
                  currentVersionName,
                  dllFileRef.current || f,
                  f.name,
                  vals.name,
                  vals.type || "preload-native",
                  vals.version || "0.0.0",
                  true,
                );
                if (!err) {
                  succFiles.push(f.name);
                  continue;
                }
              }
            }
            errPairs.push({ name: f.name, err });
            continue;
          }
          succFiles.push(f.name);
        }
      }
      await refreshModsAndStates(currentVersionName);
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

  useEffect(() => {
    const isFileDrag = (e: DragEvent) => {
      try {
        const dt = e?.dataTransfer;
        if (!dt) return false;
        const types = dt.types ? Array.from(dt.types) : [];
        if (types.includes("Files")) return true;
        const items = dt.items ? Array.from(dt.items) : [];
        return items.some((it) => it?.kind === "file");
      } catch {
        return false;
      }
    };

    const onDragEnter = (e: DragEvent) => {
      if (!isFileDrag(e)) return;
      e.preventDefault();
      e.stopPropagation();
      dragCounter.current += 1;
      setDragActive(true);
    };

    const onDragLeave = (e: DragEvent) => {
      if (dragCounter.current <= 0) return;
      e.preventDefault();
      e.stopPropagation();
      dragCounter.current -= 1;
      if (dragCounter.current <= 0) {
        dragCounter.current = 0;
        setDragActive(false);
      }
    };

    const onDragOver = (e: DragEvent) => {
      if (!isFileDrag(e) && dragCounter.current <= 0) return;
      e.preventDefault();
      e.stopPropagation();
      try {
        (e.dataTransfer as any).dropEffect = "copy";
      } catch {}
    };

    const onDrop = (e: DragEvent) => {
      const hasFiles = (e.dataTransfer?.files?.length || 0) > 0;
      if (!hasFiles && dragCounter.current <= 0 && !isFileDrag(e)) return;
      handleDrop(e);
    };

    document.addEventListener("dragenter", onDragEnter, true);
    document.addEventListener("dragleave", onDragLeave, true);
    document.addEventListener("dragover", onDragOver, true);
    document.addEventListener("drop", onDrop, true);

    return () => {
      document.removeEventListener("dragenter", onDragEnter, true);
      document.removeEventListener("dragleave", onDragLeave, true);
      document.removeEventListener("dragover", onDragOver, true);
      document.removeEventListener("drop", onDrop, true);
    };
  });

  return (
    <motion.div
      ref={scrollRef}
      className={`fixed inset-0 z-40 w-full h-full flex flex-col pt-[84px] px-6 pb-6 overflow-hidden bg-default-50 dark:bg-black ${
        dragActive ? "cursor-copy" : ""
      }`}
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.25 }}
    >
      <BaseModal
        size="sm"
        isOpen={importing && !dllOpen}
        hideCloseButton
        isDismissable={false}
      >
        <ModalContent>
          {() => (
            <>
              <BaseModalHeader className="flex items-center gap-2 text-primary-600">
                <FiUploadCloud className="w-5 h-5" />
                <span>
                  {t("mods.importing_title", { defaultValue: "正在导入..." })}
                </span>
              </BaseModalHeader>
              <BaseModalBody>
                <div className="py-1">
                  <Progress
                    isIndeterminate
                    aria-label="importing"
                    className="w-full"
                  />
                </div>
                <div className="text-default-600 text-sm">
                  {t("mods.importing_body", {
                    defaultValue: "请稍候，正在处理所选文件。",
                  })}
                </div>
                {currentFile ? (
                  <div className="mt-1 rounded-md bg-default-100/60 border border-default-200 px-3 py-2 text-default-800 text-sm wrap-break-word whitespace-pre-wrap">
                    {currentFile}
                  </div>
                ) : null}
              </BaseModalBody>
            </>
          )}
        </ModalContent>
      </BaseModal>
      <BaseModal
        size="md"
        isOpen={errOpen}
        onOpenChange={errOnOpenChange}
        hideCloseButton
      >
        <ModalContent>
          {(onClose) => (
            <>
              <BaseModalHeader
                className={`flex items-center gap-2 ${
                  resultFailed.length ? "text-red-600" : "text-primary-600"
                }`}
              >
                {resultFailed.length ? (
                  <FiAlertTriangle className="w-5 h-5" />
                ) : (
                  <FiUploadCloud className="w-5 h-5" />
                )}
                <span>
                  {resultFailed.length
                    ? t("mods.summary_title_partial", {
                        defaultValue: "导入完成（部分失败）",
                      })
                    : t("mods.summary_title_done", {
                        defaultValue: "导入完成",
                      })}
                </span>
              </BaseModalHeader>
              <BaseModalBody>
                {resultSuccess.length ? (
                  <div className="mb-2">
                    <div className="text-sm font-semibold text-success">
                      {t("mods.summary_success", { defaultValue: "成功" })} (
                      {resultSuccess.length})
                    </div>
                    <div className="mt-1 rounded-md bg-success/5 border border-success/30 px-3 py-2 text-success-700 text-sm wrap-break-word whitespace-pre-wrap">
                      {resultSuccess.join("\n")}
                    </div>
                  </div>
                ) : null}
                {resultFailed.length ? (
                  <div>
                    <div className="text-sm font-semibold text-danger">
                      {t("mods.summary_failed", { defaultValue: "失败" })} (
                      {resultFailed.length})
                    </div>
                    <div className="mt-1 rounded-md bg-danger/5 border border-danger/30 px-3 py-2 text-danger-700 text-sm wrap-break-word whitespace-pre-wrap">
                      {resultFailed
                        .map(
                          (it) => `${it.name} - ${resolveImportError(it.err)}`,
                        )
                        .join("\n")}
                    </div>
                  </div>
                ) : null}
              </BaseModalBody>
              <BaseModalFooter>
                <Button
                  color="primary"
                  onPress={() => {
                    setErrorMsg("");
                    setErrorFile("");
                    setResultSuccess([]);
                    setResultFailed([]);
                    onClose();
                  }}
                >
                  {t("common.confirm", { defaultValue: "确定" })}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>

      <BaseModal
        size="md"
        isOpen={delOpen}
        onOpenChange={delOnOpenChange}
        hideCloseButton
      >
        <ModalContent>
          {(onClose) => (
            <>
              <BaseModalHeader
                className={`flex items-center gap-2 ${
                  resultFailed.length ? "text-red-600" : "text-primary-600"
                }`}
              >
                {resultFailed.length ? (
                  <FiAlertTriangle className="w-5 h-5" />
                ) : (
                  <FiUploadCloud className="w-5 h-5" />
                )}
                <span>
                  {resultFailed.length
                    ? t("mods.delete_summary_title_failed", {
                        defaultValue: "删除失败",
                      })
                    : t("mods.delete_summary_title_done", {
                        defaultValue: "删除完成",
                      })}
                </span>
              </BaseModalHeader>
              <BaseModalBody>
                {resultSuccess.length ? (
                  <div className="mb-2">
                    <div className="text-sm font-semibold text-success">
                      {t("mods.summary_deleted", { defaultValue: "已删除" })} (
                      {resultSuccess.length})
                    </div>
                    <div className="mt-1 rounded-md bg-success/5 border border-success/30 px-3 py-2 text-success-700 text-sm wrap-break-word whitespace-pre-wrap">
                      {resultSuccess.join("\n")}
                    </div>
                  </div>
                ) : null}
                {resultFailed.length ? (
                  <div>
                    <div className="text-sm font-semibold text-danger">
                      {t("mods.summary_failed", { defaultValue: "失败" })} (
                      {resultFailed.length})
                    </div>
                    <div className="mt-1 rounded-md bg-danger/5 border border-danger/30 px-3 py-2 text-danger-700 text-sm wrap-break-word whitespace-pre-wrap">
                      {resultFailed
                        .map(
                          (it) => `${it.name} - ${resolveImportError(it.err)}`,
                        )
                        .join("\n")}
                    </div>
                  </div>
                ) : null}
              </BaseModalBody>
              <BaseModalFooter>
                <Button
                  color="primary"
                  onPress={() => {
                    setErrorMsg("");
                    setErrorFile("");
                    setResultSuccess([]);
                    setResultFailed([]);
                    onClose();
                    delOnClose();
                  }}
                >
                  {t("common.confirm", { defaultValue: "确定" })}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>
      <BaseModal
        size="md"
        isOpen={dllOpen}
        onOpenChange={dllOnOpenChange}
        hideCloseButton
      >
        <ModalContent>
          {(onClose) => (
            <>
              <BaseModalHeader className="text-primary-600">
                {t("mods.dll_modal_title", { defaultValue: "导入 DLL 插件" })}
              </BaseModalHeader>
              <BaseModalBody>
                <div className="flex flex-col gap-3">
                  <Input
                    label={
                      t("mods.dll_name", { defaultValue: "插件名称" }) as string
                    }
                    value={dllName}
                    onValueChange={setDllName}
                    autoFocus
                    size="sm"
                  />
                  <Input
                    label={
                      t("mods.dll_type", { defaultValue: "类型" }) as string
                    }
                    value={dllType}
                    onValueChange={setDllType}
                    size="sm"
                  />
                  <Input
                    label={
                      t("mods.dll_version", { defaultValue: "版本" }) as string
                    }
                    value={dllVersion}
                    onValueChange={setDllVersion}
                    size="sm"
                  />
                </div>
              </BaseModalBody>
              <BaseModalFooter>
                <Button
                  variant="light"
                  onPress={() => {
                    try {
                      dllConfirmRef.current = null;
                      dllResolveRef.current && dllResolveRef.current(false);
                    } finally {
                      onClose();
                    }
                  }}
                >
                  {t("common.cancel", { defaultValue: "取消" })}
                </Button>
                <Button
                  color="primary"
                  onPress={() => {
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
                      onClose();
                    }
                  }}
                >
                  {t("common.confirm", { defaultValue: "确定" })}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>
      <BaseModal
        size="md"
        isOpen={dupOpen}
        onOpenChange={dupOnOpenChange}
        hideCloseButton
      >
        <ModalContent>
          {(onClose) => (
            <>
              <BaseModalHeader className="text-primary-600">
                {t("mods.overwrite_modal_title", {
                  defaultValue: "检测到重复",
                })}
              </BaseModalHeader>
              <BaseModalBody>
                <div className="text-sm text-default-700">
                  {t("mods.overwrite_modal_body", {
                    defaultValue: "同名模块文件夹已存在，是否覆盖（更新）？",
                  })}
                </div>
                {dupNameRef.current ? (
                  <div className="mt-1 rounded-md bg-default-100/60 border border-default-200 px-3 py-2 text-default-800 text-sm wrap-break-word whitespace-pre-wrap">
                    {dupNameRef.current}
                  </div>
                ) : null}
              </BaseModalBody>
              <BaseModalFooter>
                <Button
                  variant="light"
                  onPress={() => {
                    try {
                      dupResolveRef.current && dupResolveRef.current(false);
                    } finally {
                      onClose();
                    }
                  }}
                >
                  {t("common.cancel", { defaultValue: "取消" })}
                </Button>
                <Button
                  color="primary"
                  onPress={() => {
                    try {
                      dupResolveRef.current && dupResolveRef.current(true);
                    } finally {
                      onClose();
                    }
                  }}
                >
                  {t("common.confirm", { defaultValue: "确定" })}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>
      {/* Drag Overlay */}
      <AnimatePresence>
        {dragActive ? (
          <motion.div
            className="pointer-events-none fixed inset-0 z-[100] flex items-center justify-center bg-black/20 backdrop-blur-sm"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.2 }}
          >
            <div className="bg-white/90 dark:bg-zinc-900/90 p-8 rounded-4xl shadow-2xl flex flex-col items-center gap-4 border border-white/20">
              <FiUploadCloud className="w-16 h-16 text-primary-500" />
              <div className="text-xl font-bold bg-linear-to-r from-primary-500 to-secondary-500 bg-clip-text text-transparent">
                {t("mods.drop_hint", {
                  defaultValue: "拖入 .zip 或 .dll 以导入模组/插件",
                })}
              </div>
            </div>
          </motion.div>
        ) : null}
      </AnimatePresence>

      <div className="flex flex-col gap-4 mb-6 shrink-0 border-none shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md p-6 rounded-4xl">
        <PageHeader
          title={t("moddedcard.title", { defaultValue: "Mods" })}
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
              <input
                ref={fileInputRef}
                type="file"
                accept=".zip,.dll"
                multiple
                className="hidden"
                onChange={handleFilePick}
              />
              <Button
                color="primary"
                variant="shadow"
                className="bg-emerald-600 hover:bg-emerald-500 text-white shadow-lg shadow-emerald-900/20"
                startContent={<FiUploadCloud />}
                onPress={() =>
                  navigate("/filemanager", {
                    state: {
                      allowedExt: [".zip", ".dll"],
                      multi: true,
                      returnTo: "/mods",
                    },
                  })
                }
                isDisabled={importing}
              >
                {t("mods.import_button", { defaultValue: "导入 .zip/.dll" })}
              </Button>
              <Button
                variant="flat"
                className="bg-default-100 dark:bg-zinc-800"
                onPress={openFolder}
              >
                {t("downloadmodal.open_folder", { defaultValue: "打开目录" })}
              </Button>
            </>
          }
        />

        <div className="flex flex-col sm:flex-row items-center gap-3">
          <Input
            placeholder={t("common.search_placeholder", {
              defaultValue: "搜索...",
            })}
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
            classNames={{
              inputWrapper:
                "bg-default-100 dark:bg-default-50/50 hover:bg-default-200/70 transition-colors group-data-[focus=true]:bg-white dark:group-data-[focus=true]:bg-zinc-900 shadow-sm",
            }}
          />
          <div className="w-px h-6 bg-default-200 dark:bg-white/10 hidden sm:block" />
          <Checkbox
            size="sm"
            isSelected={onlyEnabled}
            onValueChange={setOnlyEnabled}
            classNames={{
              base: "m-0",
              label: "text-default-500",
            }}
          >
            {
              t("mods.only_enabled", {
                defaultValue: "仅显示已启用的模组",
              }) as string
            }
          </Checkbox>
          <div className="flex-1" />
          <Tooltip
            content={
              t("common.refresh", {
                defaultValue: "刷新",
              }) as unknown as string
            }
          >
            <Button
              size="sm"
              variant="light"
              isIconOnly
              radius="full"
              className="text-default-500"
              onPress={() => refreshAll()}
              isDisabled={loading}
            >
              <FaSync className={loading ? "animate-spin" : ""} size={16} />
            </Button>
          </Tooltip>
        </div>
      </div>

      <div className="flex-1 overflow-hidden rounded-4xl bg-white/40 dark:bg-zinc-900/30 backdrop-blur-sm border border-white/20 dark:border-white/5 p-1">
        <div className="h-full overflow-y-auto p-4 custom-scrollbar">
          {!currentVersionName ? (
            <div className="flex flex-col items-center justify-center h-full text-default-400 gap-2">
              <FiAlertTriangle className="w-8 h-8 opacity-50" />
              <p>
                {t("launcherpage.currentVersion_none", {
                  defaultValue: "未选择版本",
                })}
              </p>
            </div>
          ) : filtered.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full text-default-400 gap-2">
              <FaPuzzlePiece className="w-8 h-8 opacity-50" />
              <p>
                {t("moddedcard.content.none", { defaultValue: "未找到模组" })}
              </p>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
              {filtered.map((m, idx) => {
                const delay = Math.min(idx * 40, 400);
                return (
                  <motion.div
                    key={`${m.name}-${m.version}-${idx}`}
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ duration: 0.3, delay: delay / 1000 }}
                  >
                    <Card
                      isPressable
                      onPress={() => openDetails(m)}
                      className="w-full h-full bg-white/80 dark:bg-zinc-800/50 backdrop-blur-md border border-white/40 dark:border-white/5 hover:border-emerald-500/30 dark:hover:border-emerald-500/30 transition-all duration-200 group"
                      shadow="sm"
                    >
                      <CardBody className="p-4 flex flex-row items-center gap-4">
                        <div
                          className={`p-3 rounded-xl transition-colors ${
                            enabledByName.get(m.name)
                              ? "bg-emerald-100/50 dark:bg-emerald-900/20 text-emerald-600 dark:text-emerald-400"
                              : "bg-default-100 dark:bg-zinc-800 text-default-400"
                          }`}
                        >
                          <FaPuzzlePiece className="w-5 h-5" />
                        </div>
                        <div className="flex-1 min-w-0 text-left">
                          <div className="font-semibold text-default-900 truncate">
                            {m.name}
                          </div>
                          <div className="text-tiny text-default-500 truncate">
                            {m.version} {m.author ? `· ${m.author}` : ""}
                          </div>
                        </div>
                        <div
                          className="flex items-center"
                          onClick={(e) => e.stopPropagation()}
                        >
                          <Switch
                            size="sm"
                            color="success"
                            isSelected={!!enabledByName.get(m.name)}
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
                            aria-label={
                              t("mods.toggle_label", {
                                defaultValue: "启用模组",
                              }) as string
                            }
                            classNames={{
                              wrapper:
                                "group-hover:scale-110 transition-transform",
                            }}
                          />
                        </div>
                      </CardBody>
                    </Card>
                  </motion.div>
                );
              })}
            </div>
          )}
        </div>
      </div>

      <BaseModal
        size="md"
        isOpen={infoOpen}
        onOpenChange={infoOnOpenChange}
        hideCloseButton
      >
        <ModalContent>
          {(onClose) => (
            <>
              <BaseModalHeader className="flex items-center gap-2 text-primary-600">
                <FaPuzzlePiece className="w-5 h-5" />
                <span>
                  {t("mods.details_title", { defaultValue: "模组详情" })}
                </span>
              </BaseModalHeader>
              <BaseModalBody>
                {activeMod ? (
                  <div className="space-y-2 text-sm">
                    <div>
                      <span className="text-default-500">
                        {t("mods.field_name", { defaultValue: "名称" })}：
                      </span>
                      {activeMod.name}
                    </div>
                    <div>
                      <span className="text-default-500">
                        {t("mods.field_version", { defaultValue: "版本" })}：
                      </span>
                      {activeMod.version || "-"}
                    </div>
                    <div>
                      <span className="text-default-500">
                        {t("mods.field_type", { defaultValue: "类型" })}：
                      </span>
                      {activeMod.type || "-"}
                    </div>
                    <div>
                      <span className="text-default-500">
                        {t("mods.field_entry", { defaultValue: "入口" })}：
                      </span>
                      {activeMod.entry || "-"}
                    </div>
                    {activeMod.author ? (
                      <div>
                        <span className="text-default-500">
                          {t("mods.field_author", { defaultValue: "作者" })}：
                        </span>
                        {activeMod.author}
                      </div>
                    ) : null}
                    <div className="pt-2">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <span className="text-default-500">
                            {t("mods.toggle_label", {
                              defaultValue: "启用模组",
                            })}
                          </span>
                          <Chip
                            size="sm"
                            variant="flat"
                            color={
                              enabledByName.get(activeMod.name)
                                ? "success"
                                : "warning"
                            }
                          >
                            {enabledByName.get(activeMod.name)
                              ? (t("mods.toggle_on", {
                                  defaultValue: "已启用",
                                }) as string)
                              : (t("mods.toggle_off", {
                                  defaultValue: "已关闭",
                                }) as string)}
                          </Chip>
                        </div>
                        <Switch
                          isSelected={!!enabledByName.get(activeMod.name)}
                          onValueChange={async (val) => {
                            const name =
                              currentVersionName || readCurrentVersionName();
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
                          aria-label={
                            t("mods.toggle_label", {
                              defaultValue: "启用模组",
                            }) as string
                          }
                        />
                      </div>
                      <div className="text-default-500 text-xs mt-1">
                        {enabledByName.get(activeMod.name)
                          ? (t("mods.toggle_desc_on", {
                              defaultValue: "模组已启用，启动游戏时会加载。",
                            }) as string)
                          : (t("mods.toggle_desc_off", {
                              defaultValue: "模组已关闭，启动游戏时不会加载。",
                            }) as string)}
                      </div>
                    </div>
                  </div>
                ) : null}
              </BaseModalBody>
              <BaseModalFooter>
                <Button
                  variant="light"
                  onPress={() => {
                    onClose();
                  }}
                >
                  {t("common.cancel", { defaultValue: "取消" })}
                </Button>
                <Button color="danger" onPress={delCfmOnOpen}>
                  {t("common.delete", { defaultValue: "删除" })}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>
      <BaseModal
        size="sm"
        isOpen={delCfmOpen}
        onOpenChange={delCfmOnOpenChange}
        hideCloseButton
      >
        <ModalContent>
          {(onClose) => (
            <>
              <BaseModalHeader className="text-danger">
                {t("mods.confirm_delete_title", { defaultValue: "确认删除" })}
              </BaseModalHeader>
              <BaseModalBody>
                <div className="text-sm text-default-700 wrap-break-word whitespace-pre-wrap">
                  {t("mods.confirm_delete_body", {
                    type: t("moddedcard.title"),
                    defaultValue: "确定要删除此模组吗？此操作不可撤销。",
                  })}
                </div>
                {activeMod ? (
                  <div className="mt-1 rounded-md bg-default-100/60 border border-default-200 px-3 py-2 text-default-800 text-sm wrap-break-word whitespace-pre-wrap">
                    {activeMod.name}
                  </div>
                ) : null}
              </BaseModalBody>
              <BaseModalFooter>
                <Button
                  variant="light"
                  onPress={() => {
                    onClose();
                  }}
                >
                  {t("common.cancel", { defaultValue: "取消" })}
                </Button>
                <Button
                  color="danger"
                  isLoading={deleting}
                  onPress={async () => {
                    await handleDeleteMod();
                    onClose();
                  }}
                >
                  {t("common.confirm", { defaultValue: "确定" })}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>
    </motion.div>
  );
};

export default ModsPage;
