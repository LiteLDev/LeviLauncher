import React from "react";
import { useTranslation } from "react-i18next";
import {
  Button,
  Dropdown,
  DropdownItem,
  DropdownMenu,
  DropdownTrigger,
  Spinner,
  Tooltip,
  Modal,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  Progress,
  useDisclosure,
} from "@heroui/react";
import { AnimatePresence, motion } from "framer-motion";
import { useLocation, useNavigate } from "react-router-dom";
import { GetContentRoots } from "../../bindings/github.com/liteldev/LeviLauncher/minecraft";
import * as types from "../../bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import { FaGlobe, FaImage, FaCogs } from "react-icons/fa";
import { readCurrentVersionName } from "../utils/currentVersion";
import { countDirectories } from "../utils/fs";
import { listPlayers } from "../utils/content";
import * as minecraft from "../../bindings/github.com/liteldev/LeviLauncher/minecraft";
import { FiUploadCloud, FiAlertTriangle } from "react-icons/fi";

export default function ContentPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation() as any;
  const hasBackend = minecraft !== undefined;
  const [loading, setLoading] = React.useState<boolean>(true);
  const [error, setError] = React.useState<string>("");
  const [currentVersionName, setCurrentVersionName] =
    React.useState<string>("");
  const [roots, setRoots] = React.useState<types.ContentRoots>({
    base: "",
    usersRoot: "",
    resourcePacks: "",
    behaviorPacks: "",
    isIsolation: false,
    isPreview: false,
  });
  const [players, setPlayers] = React.useState<string[]>([]);
  const [selectedPlayer, setSelectedPlayer] = React.useState<string>("");
  const [worldsCount, setWorldsCount] = React.useState<number>(0);
  const [resCount, setResCount] = React.useState<number>(0);
  const [bpCount, setBpCount] = React.useState<number>(0);
  const [skinCount, setSkinCount] = React.useState<number>(0);
  const [dragActive, setDragActive] = React.useState(false);
  const dragCounter = React.useRef(0);
  const [importing, setImporting] = React.useState(false);
  const [errorMsg, setErrorMsg] = React.useState("");
  const [currentFile, setCurrentFile] = React.useState("");
  const [resultSuccess, setResultSuccess] = React.useState<string[]>([]);
  const [resultFailed, setResultFailed] = React.useState<
    Array<{ name: string; err: string }>
  >([]);
  const fileInputRef = React.useRef<HTMLInputElement | null>(null);
  const fmProcessedRef = React.useRef<string | null>(null);
  const dupResolveRef = React.useRef<((overwrite: boolean) => void) | null>(
    null
  );
  const dupNameRef = React.useRef<string>("");
  const {
    isOpen: errOpen,
    onOpen: errOnOpen,
    onOpenChange: errOnOpenChange,
  } = useDisclosure();
  const {
    isOpen: dupOpen,
    onOpen: dupOnOpen,
    onOpenChange: dupOnOpenChange,
  } = useDisclosure();

  const refreshAll = React.useCallback(async () => {
    setLoading(true);
    setError("");
    const name = readCurrentVersionName();
    setCurrentVersionName(name);
    try {
      if (!hasBackend || !name) {
        setRoots({
          base: "",
          usersRoot: "",
          resourcePacks: "",
          behaviorPacks: "",
          isIsolation: false,
          isPreview: false,
        });
        setPlayers([]);
        setSelectedPlayer("");
        setWorldsCount(0);
        setResCount(0);
        setBpCount(0);
      } else {
        const r = await GetContentRoots(name);
        const safe = r || {
          base: "",
          usersRoot: "",
          resourcePacks: "",
          behaviorPacks: "",
          isIsolation: false,
          isPreview: false,
        };
        setRoots(safe);
        if (safe.usersRoot) {
          const names = await listPlayers(safe.usersRoot);
          setPlayers(names);
          const nextPlayer = names[0] || "";
          setSelectedPlayer(nextPlayer);
          if (nextPlayer) {
            const wp = `${safe.usersRoot}\\${nextPlayer}\\games\\com.mojang\\minecraftWorlds`;
            setWorldsCount(await countDirectories(wp));
            const sp = `${safe.usersRoot}\\${nextPlayer}\\games\\com.mojang\\skin_packs`;
            setSkinCount(await countDirectories(sp));
          } else {
            setWorldsCount(0);
            setSkinCount(0);
          }
        } else {
          setPlayers([]);
          setSelectedPlayer("");
          setWorldsCount(0);
          setSkinCount(0);
        }
        setResCount(await countDirectories(safe.resourcePacks));
        setBpCount(await countDirectories(safe.behaviorPacks));
      }
    } catch (e) {
      setError(
        t("contentpage.error_resolve_paths", {
          defaultValue: "无法解析内容路径。",
        }) as string
      );
    } finally {
      setLoading(false);
    }
  }, [hasBackend, t]);

  React.useEffect(() => {
    refreshAll();
  }, []);

  const onChangePlayer = async (player: string) => {
    setSelectedPlayer(player);
    if (!hasBackend || !roots.usersRoot || !player) {
      setWorldsCount(0);
      setSkinCount(0);
      return;
    }
    const wp = `${roots.usersRoot}\\${player}\\games\\com.mojang\\minecraftWorlds`;
    setWorldsCount(await countDirectories(wp));
    const sp = `${roots.usersRoot}\\${player}\\games\\com.mojang\\skin_packs`;
    setSkinCount(await countDirectories(sp));
  };
  const postImportMcpack = async (
    name: string,
    file: File,
    overwrite: boolean
  ): Promise<string> => {
    try {
      const buf = await file.arrayBuffer();
      const bytes = Array.from(new Uint8Array(buf));
      let err = "";
      if (
        selectedPlayer &&
        typeof (minecraft as any)?.ImportMcpackWithPlayer === "function"
      ) {
        err = await (minecraft as any)?.ImportMcpackWithPlayer?.(
          name,
          selectedPlayer,
          file.name,
          bytes,
          overwrite
        );
      } else {
        err = await (minecraft as any)?.ImportMcpack?.(
          name,
          bytes,
          overwrite
        );
      }
      return String(err || "");
    } catch (e: any) {
      return String(e?.message || "IMPORT_ERROR");
    }
  };
  const postImportMcaddon = async (
    name: string,
    file: File,
    overwrite: boolean
  ): Promise<string> => {
    try {
      const buf = await file.arrayBuffer();
      const bytes = Array.from(new Uint8Array(buf));
      let err = "";
      if (
        selectedPlayer &&
        typeof (minecraft as any)?.ImportMcaddonWithPlayer === "function"
      ) {
        err = await (minecraft as any)?.ImportMcaddonWithPlayer?.(
          name,
          selectedPlayer,
          bytes,
          overwrite
        );
      } else {
        err = await (minecraft as any)?.ImportMcaddon?.(
          name,
          bytes,
          overwrite
        );
      }
      return String(err || "");
    } catch (e: any) {
      return String(e?.message || "IMPORT_ERROR");
    }
  };
  const postImportMcworld = async (
    name: string,
    player: string,
    file: File,
    overwrite: boolean
  ): Promise<string> => {
    try {
      const buf = await file.arrayBuffer();
      const bytes = Array.from(new Uint8Array(buf));
      const err = await (minecraft as any)?.ImportMcworld?.(
        name,
        player,
        file.name,
        bytes,
        overwrite
      );
      return String(err || "");
    } catch (e: any) {
      return String(e?.message || "IMPORT_ERROR");
    }
  };

  const resolveImportError = (err: string): string => {
    const code = String(err || "").trim();
    switch (code) {
      case "ERR_NO_PLAYER":
        return t("contentpage.no_player_selected", {
          defaultValue: "未选择玩家",
        }) as string;
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

  React.useEffect(() => {
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
          }) as string
        );
        return;
      }
      let started = false;
      const succFiles: string[] = [];
      const errPairs: Array<{ name: string; err: string }> = [];
      for (const p of paths) {
        const lower = p.toLowerCase();
        if (lower.endsWith(".mcpack")) {
          if (!started) {
            setImporting(true);
            started = true;
          }
          const base = p.replace(/\\/g, "/").split("/").pop() || p;
          setCurrentFile(base);
          let err = "";
          let usePlayer = selectedPlayer;
          if (!usePlayer) {
            try {
              const ver = currentVersionName || readCurrentVersionName();
              if (ver) {
                const r = await GetContentRoots(ver);
                const ur = r?.usersRoot || "";
                if (ur) {
                  const names = await listPlayers(ur);
                  usePlayer = names[0] || "";
                  if (usePlayer) setSelectedPlayer(usePlayer);
                }
              }
            } catch {}
          }
          if (
            usePlayer &&
            typeof (minecraft as any)?.ImportMcpackPathWithPlayer === "function"
          ) {
            err = await (minecraft as any)?.ImportMcpackPathWithPlayer?.(
              name,
              usePlayer,
              p,
              false
            );
          } else {
            err = await (minecraft as any)?.ImportMcpackPath?.(name, p, false);
          }
          if (err) {
            if (String(err) === "ERR_DUPLICATE_FOLDER") {
              dupNameRef.current = base;
              await new Promise<void>((r) => setTimeout(r, 0));
              dupOnOpen();
              const ok = await new Promise<boolean>((resolve) => {
                dupResolveRef.current = resolve;
              });
              if (ok) {
                if (
                  usePlayer &&
                  typeof (minecraft as any)?.ImportMcpackPathWithPlayer ===
                    "function"
                ) {
                  err = await (minecraft as any)?.ImportMcpackPathWithPlayer?.(
                    name,
                    usePlayer,
                    p,
                    true
                  );
                } else {
                  err = await (minecraft as any)?.ImportMcpackPath?.(
                    name,
                    p,
                    true
                  );
                }
                if (!err) {
                  succFiles.push(base);
                  continue;
                }
              }
            }
            errPairs.push({ name: base, err });
            continue;
          }
          succFiles.push(base);
        } else if (lower.endsWith(".mcaddon")) {
          if (!started) {
            setImporting(true);
            started = true;
          }
          const base = p.replace(/\\/g, "/").split("/").pop() || p;
          setCurrentFile(base);
          let err = "";
          let usePlayer2 = selectedPlayer;
          if (!usePlayer2) {
            try {
              const ver = currentVersionName || readCurrentVersionName();
              if (ver) {
                const r = await GetContentRoots(ver);
                const ur = r?.usersRoot || "";
                if (ur) {
                  const names = await listPlayers(ur);
                  usePlayer2 = names[0] || "";
                  if (usePlayer2) setSelectedPlayer(usePlayer2);
                }
              }
            } catch {}
          }
          if (
            usePlayer2 &&
            typeof (minecraft as any)?.ImportMcaddonPathWithPlayer ===
              "function"
          ) {
            err = await (minecraft as any)?.ImportMcaddonPathWithPlayer?.(
              name,
              usePlayer2,
              p,
              false
            );
          } else {
            err = await (minecraft as any)?.ImportMcaddonPath?.(name, p, false);
          }
          if (err) {
            if (String(err) === "ERR_DUPLICATE_FOLDER") {
              dupNameRef.current = base;
              await new Promise<void>((r) => setTimeout(r, 0));
              dupOnOpen();
              const ok = await new Promise<boolean>((resolve) => {
                dupResolveRef.current = resolve;
              });
              if (ok) {
                if (
                  usePlayer2 &&
                  typeof (minecraft as any)?.ImportMcaddonPathWithPlayer ===
                    "function"
                ) {
                  err = await (minecraft as any)?.ImportMcaddonPathWithPlayer?.(
                    name,
                    usePlayer2,
                    p,
                    true
                  );
                } else {
                  err = await (minecraft as any)?.ImportMcaddonPath?.(
                    name,
                    p,
                    true
                  );
                }
                if (!err) {
                  succFiles.push(base);
                  continue;
                }
              }
            }
            errPairs.push({ name: base, err });
            continue;
          }
          succFiles.push(base);
        } else if (lower.endsWith(".mcworld")) {
          const base = p.replace(/\\/g, "/").split("/").pop() || p;
          let usePlayer = selectedPlayer;
          if (!usePlayer) {
            try {
              const ver = currentVersionName || readCurrentVersionName();
              if (ver) {
                const r = await GetContentRoots(ver);
                const ur = r?.usersRoot || "";
                if (ur) {
                  const names = await listPlayers(ur);
                  usePlayer = names[0] || "";
                  if (usePlayer) setSelectedPlayer(usePlayer);
                }
              }
            } catch {}
          }
          if (!usePlayer) {
            errPairs.push({ name: base, err: "ERR_NO_PLAYER" });
            continue;
          }
          if (!started) {
            setImporting(true);
            started = true;
          }
          setCurrentFile(base);
          let err = await (minecraft as any)?.ImportMcworldPath?.(
            name,
            usePlayer,
            p,
            false
          );
          if (err) {
            if (String(err) === "ERR_DUPLICATE_FOLDER") {
              dupNameRef.current = base;
              await new Promise<void>((r) => setTimeout(r, 0));
              dupOnOpen();
              const ok = await new Promise<boolean>((resolve) => {
                dupResolveRef.current = resolve;
              });
              if (ok) {
                err = await (minecraft as any)?.ImportMcworldPath?.(
                  name,
                  usePlayer,
                  p,
                  true
                );
                if (!err) {
                  succFiles.push(base);
                  continue;
                }
              }
            }
            errPairs.push({ name: base, err });
            continue;
          }
          succFiles.push(base);
        }
      }
      await refreshAll();
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

  const handleFilePick = async (e: React.ChangeEvent<HTMLInputElement>) => {
    try {
      setErrorMsg("");
      setResultSuccess([]);
      setResultFailed([]);
      const list = e.target.files;
      if (!list || list.length === 0) return;
      if (!currentVersionName) {
        setErrorMsg(
          t("launcherpage.currentVersion_none", {
            defaultValue: "未选择版本",
          }) as string
        );
        return;
      }
      let started = false;
      const succFiles: string[] = [];
      const errPairs: Array<{ name: string; err: string }> = [];
      const files: File[] = Array.from(list).filter(
        (f) =>
          f &&
          (f.name.toLowerCase().endsWith(".mcworld") ||
            f.name.toLowerCase().endsWith(".mcpack") ||
            f.name.toLowerCase().endsWith(".mcaddon"))
      );
      const hasWorld = files.some((f) =>
        f?.name?.toLowerCase().endsWith(".mcworld")
      );
      if (hasWorld && !selectedPlayer) {
        setErrorMsg(
          t("contentpage.require_player_for_world_import", {
            defaultValue: "导入世界 (.mcworld) 前请先选择玩家",
          }) as string
        );
        return;
      }
      for (const f of files) {
        const lower = f.name.toLowerCase();
        if (lower.endsWith(".mcpack")) {
          if (!started) {
            setImporting(true);
            started = true;
          }
          setCurrentFile(f.name);
          let err = await postImportMcpack(currentVersionName, f, false);
          if (err) {
            if (String(err) === "ERR_DUPLICATE_FOLDER") {
              dupNameRef.current = f.name;
              await new Promise<void>((r) => setTimeout(r, 0));
              dupOnOpen();
              const ok = await new Promise<boolean>((resolve) => {
                dupResolveRef.current = resolve;
              });
              if (ok) {
                err = await postImportMcpack(currentVersionName, f, true);
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
        } else if (lower.endsWith(".mcaddon")) {
          if (!started) {
            setImporting(true);
            started = true;
          }
          setCurrentFile(f.name);
          let err = await postImportMcaddon(currentVersionName, f, false);
          if (err) {
            if (String(err) === "ERR_DUPLICATE_FOLDER") {
              dupNameRef.current = f.name;
              await new Promise<void>((r) => setTimeout(r, 0));
              dupOnOpen();
              const ok = await new Promise<boolean>((resolve) => {
                dupResolveRef.current = resolve;
              });
              if (ok) {
                err = await postImportMcaddon(currentVersionName, f, true);
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
        } else if (lower.endsWith(".mcworld")) {
          if (!selectedPlayer) {
            errPairs.push({ name: f.name, err: "ERR_NO_PLAYER" });
            continue;
          }
          if (!started) {
            setImporting(true);
            started = true;
          }
          setCurrentFile(f.name);
          let err = await postImportMcworld(
            currentVersionName,
            selectedPlayer,
            f,
            false
          );
          if (err) {
            if (String(err) === "ERR_DUPLICATE_FOLDER") {
              dupNameRef.current = f.name;
              await new Promise<void>((r) => setTimeout(r, 0));
              dupOnOpen();
              const ok = await new Promise<boolean>((resolve) => {
                dupResolveRef.current = resolve;
              });
              if (ok) {
                err = await postImportMcworld(
                  currentVersionName,
                  selectedPlayer,
                  f,
                  true
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
      await refreshAll();
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

  return (
    <motion.div
      className={`relative w-full h-full p-3 sm:p-4 lg:p-6 ${
        dragActive ? "cursor-copy" : ""
      }`}
      onDragOver={(e) => {
        e.preventDefault();
        e.stopPropagation();
        try {
          e.dataTransfer.dropEffect = "copy";
        } catch {}
      }}
      onDragEnter={(e) => {
        e.preventDefault();
        e.stopPropagation();
        try {
          e.dataTransfer.dropEffect = "copy";
        } catch {}
        dragCounter.current++;
        setDragActive(true);
      }}
      onDragLeave={(e) => {
        e.preventDefault();
        e.stopPropagation();
        dragCounter.current = Math.max(0, dragCounter.current - 1);
        if (dragCounter.current === 0) setDragActive(false);
      }}
      onDrop={async (e) => {
        e.preventDefault();
        e.stopPropagation();
        dragCounter.current = 0;
        setDragActive(false);
        setErrorMsg("");
        setResultSuccess([]);
        setResultFailed([]);
        const files: File[] = Array.from(e.dataTransfer.files || []).filter(
          (f) =>
            f &&
            (f.name.toLowerCase().endsWith(".mcworld") ||
              f.name.toLowerCase().endsWith(".mcpack") ||
              f.name.toLowerCase().endsWith(".mcaddon"))
        );
        const hasWorld = files.some((f) =>
          f?.name?.toLowerCase().endsWith(".mcworld")
        );
        if (hasWorld && !selectedPlayer) {
          setErrorMsg(
            t("contentpage.require_player_for_world_import", {
              defaultValue: "导入世界 (.mcworld) 前请先选择玩家",
            }) as string
          );
          return;
        }
        if (!files.length) return;
        let started = false;
        const succFiles: string[] = [];
        const errPairs: Array<{ name: string; err: string }> = [];
        try {
          for (const f of files) {
            const lower = f.name.toLowerCase();
            if (lower.endsWith(".mcpack")) {
              if (!started) {
                setImporting(true);
                started = true;
              }
              setCurrentFile(f.name);
              let err = await postImportMcpack(currentVersionName, f, false);
              if (err) {
                if (String(err) === "ERR_DUPLICATE_FOLDER") {
                  dupNameRef.current = f.name;
                  await new Promise<void>((r) => setTimeout(r, 0));
                  dupOnOpen();
                  const ok = await new Promise<boolean>((resolve) => {
                    dupResolveRef.current = resolve;
                  });
                  if (ok) {
                    err = await postImportMcpack(currentVersionName, f, true);
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
            } else if (lower.endsWith(".mcaddon")) {
              if (!started) {
                setImporting(true);
                started = true;
              }
              setCurrentFile(f.name);
              let err = await postImportMcaddon(currentVersionName, f, false);
              if (err) {
                if (String(err) === "ERR_DUPLICATE_FOLDER") {
                  dupNameRef.current = f.name;
                  await new Promise<void>((r) => setTimeout(r, 0));
                  dupOnOpen();
                  const ok = await new Promise<boolean>((resolve) => {
                    dupResolveRef.current = resolve;
                  });
                  if (ok) {
                    err = await postImportMcaddon(currentVersionName, f, true);
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
            } else if (lower.endsWith(".mcworld")) {
            if (!selectedPlayer) {
              errPairs.push({ name: f.name, err: "ERR_NO_PLAYER" });
              continue;
            }
            if (!started) {
              setImporting(true);
              started = true;
            }
            setCurrentFile(f.name);
              let err = await postImportMcworld(
                currentVersionName,
                selectedPlayer,
                f,
                false
              );
              if (err) {
                if (String(err) === "ERR_DUPLICATE_FOLDER") {
                  dupNameRef.current = f.name;
                  await new Promise<void>((r) => setTimeout(r, 0));
                  dupOnOpen();
                  const ok = await new Promise<boolean>((resolve) => {
                    dupResolveRef.current = resolve;
                  });
                  if (ok) {
                    err = await postImportMcworld(
                      currentVersionName,
                      selectedPlayer,
                      f,
                      true
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
          await refreshAll();
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
      }}
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.25 }}
    >
      <Modal size="sm" isOpen={importing} hideCloseButton isDismissable={false}>
        <ModalContent>
          {() => (
            <>
              <ModalHeader className="flex items-center gap-2 text-primary-600">
                <FiUploadCloud className="w-5 h-5" />
                <span>
                  {t("mods.importing_title", { defaultValue: "正在导入..." })}
                </span>
              </ModalHeader>
              <ModalBody>
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
                  <div className="mt-1 rounded-md bg-default-100/60 border border-default-200 px-3 py-2 text-default-800 text-sm break-words whitespace-pre-wrap">
                    {currentFile}
                  </div>
                ) : null}
              </ModalBody>
            </>
          )}
        </ModalContent>
      </Modal>
      <Modal
        size="md"
        isOpen={errOpen}
        onOpenChange={errOnOpenChange}
        hideCloseButton
      >
        <ModalContent>
          {(onClose) => (
            <>
              <ModalHeader
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
              </ModalHeader>
              <ModalBody>
                {resultSuccess.length ? (
                  <div className="mb-2">
                    <div className="text-sm font-semibold text-success">
                      {t("mods.summary_success", { defaultValue: "成功" })} (
                      {resultSuccess.length})
                    </div>
                    <div className="mt-1 rounded-md bg-success/5 border border-success/30 px-3 py-2 text-success-700 text-sm break-words whitespace-pre-wrap">
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
                    <div className="mt-1 rounded-md bg-danger/5 border border-danger/30 px-3 py-2 text-danger-700 text-sm break-words whitespace-pre-wrap">
                      {resultFailed
                        .map(
                          (it) => `${it.name} - ${resolveImportError(it.err)}`
                        )
                        .join("\n")}
                    </div>
                  </div>
                ) : null}
              </ModalBody>
              <ModalFooter>
                <Button
                  color="primary"
                  onPress={() => {
                    setErrorMsg("");
                    setResultSuccess([]);
                    setResultFailed([]);
                    onClose();
                  }}
                >
                  {t("common.confirm", { defaultValue: "确定" })}
                </Button>
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>
      <Modal
        size="md"
        isOpen={dupOpen}
        onOpenChange={dupOnOpenChange}
        hideCloseButton
      >
        <ModalContent>
          {(onClose) => (
            <>
              <ModalHeader className="text-primary-600">
                {t("mods.overwrite_modal_title", {
                  defaultValue: "检测到重复",
                })}
              </ModalHeader>
              <ModalBody>
                <div className="text-sm text-default-700">
                  {t("mods.overwrite_modal_body", {
                    defaultValue: "同名模块文件夹已存在，是否覆盖（更新）？",
                  })}
                </div>
                {dupNameRef.current ? (
                  <div className="mt-1 rounded-md bg-default-100/60 border border-default-200 px-3 py-2 text-default-800 text-sm break-words whitespace-pre-wrap">
                    {dupNameRef.current}
                  </div>
                ) : null}
              </ModalBody>
              <ModalFooter>
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
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>
      <motion.div
        initial={{ opacity: 0, y: 8 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.25 }}
        className={`relative overflow-hidden rounded-2xl border border-default-200 bg-white/60 dark:bg-neutral-900/60 backdrop-blur-md p-5 ${
          dragActive ? "border-2 border-dashed border-primary" : ""
        }`}
      >
        <AnimatePresence>
          {dragActive ? (
            <motion.div
              className="pointer-events-none absolute inset-0 z-40 flex items-center justify-center bg-black/10 rounded-2xl"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.2 }}
            >
              <div className="text-primary-600 text-xl font-semibold">
                {t("contentpage.drop_hint", {
                  defaultValue: "拖入 .mcworld/.mcpack/.mcaddon 以导入",
                })}
              </div>
            </motion.div>
          ) : null}
        </AnimatePresence>
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-bold">
            {t("launcherpage.content_manage", { defaultValue: "内容管理" })}
          </h1>
          <Tooltip
            content={
              t("common.refresh", { defaultValue: "刷新" }) as unknown as string
            }
          >
            <Button
              size="sm"
              variant="bordered"
              onPress={refreshAll}
              isDisabled={loading}
              className="rounded-full px-4"
            >
              {t("common.refresh", { defaultValue: "刷新" })}
            </Button>
          </Tooltip>
        </div>

        <div className="mt-2 text-default-500 text-sm">
          {t("contentpage.current_version", { defaultValue: "当前版本" })}:{" "}
          <span className="font-medium">
            {currentVersionName ||
              t("contentpage.none", { defaultValue: "无" })}
          </span>
          <span className="mx-2">·</span>
          {t("contentpage.isolation", { defaultValue: "版本隔离" })}:{" "}
          <span className="font-medium">
            {roots.isIsolation
              ? t("common.yes", { defaultValue: "是" })
              : t("common.no", { defaultValue: "否" })}
          </span>
        </div>
        {!!error && <div className="mt-2 text-danger-500 text-sm">{error}</div>}

        <div className="mt-4 rounded-xl border border-default-200 bg-white/50 dark:bg-neutral-800/40 shadow-sm backdrop-blur-sm px-3 py-2 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className="text-small text-default-600">
              {t("contentpage.select_player", { defaultValue: "选择玩家" })}
            </span>
            <span className="text-small text-default-700 font-medium">
              {selectedPlayer ||
                t("contentpage.no_players", { defaultValue: "暂无玩家" })}
            </span>
            {!selectedPlayer ? (
              <span className="ml-2 text-small text-danger-500">
                {t("contentpage.require_player_for_world_import", {
                  defaultValue: "导入世界需选择玩家",
                })}
              </span>
            ) : null}
          </div>
          <Dropdown>
            <DropdownTrigger>
              <Button size="sm" variant="light" className="rounded-full">
                {selectedPlayer ||
                  t("contentpage.select_player", { defaultValue: "选择玩家" })}
              </Button>
            </DropdownTrigger>
            <DropdownMenu
              aria-label="Players"
              selectionMode="single"
              selectedKeys={new Set([selectedPlayer])}
              onSelectionChange={(keys) => {
                const arr = Array.from(keys as unknown as Set<string>);
                const next = arr[0] || "";
                if (typeof next === "string") onChangePlayer(next);
              }}
            >
              {players.length ? (
                players.map((p) => (
                  <DropdownItem key={p} textValue={p}>
                    {p}
                  </DropdownItem>
                ))
              ) : (
                <DropdownItem key="none" isDisabled>
                  {t("contentpage.no_players", { defaultValue: "暂无玩家" })}
                </DropdownItem>
              )}
            </DropdownMenu>
          </Dropdown>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 mt-4">
          <div
            className="rounded-xl border border-default-200 bg-white/50 dark:bg-neutral-800/40 shadow-sm backdrop-blur-sm px-3 py-3 cursor-pointer transition hover:bg-white/70 dark:hover:bg-neutral-800/60"
            onClick={() => navigate("/content/worlds")}
            role="button"
            aria-label="worlds"
          >
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <FaGlobe className="text-default-500" />
                <span className="text-small text-default-600 truncate">
                  {t("contentpage.worlds", { defaultValue: "世界" })}
                </span>
              </div>
              {loading ? (
                <div className="flex items-center gap-2">
                  <Spinner size="sm" />{" "}
                  <span className="text-default-500">
                    {t("common.loading", { defaultValue: "加载中" })}
                  </span>
                </div>
              ) : (
                <span className="text-base font-semibold text-default-800">
                  {worldsCount}
                </span>
              )}
            </div>
          </div>
          <div
            className="rounded-xl border border-default-200 bg-white/50 dark:bg-neutral-800/40 shadow-sm backdrop-blur-sm px-3 py-3 cursor-pointer transition hover:bg-white/70 dark:hover:bg-neutral-800/60"
            onClick={() => navigate("/content/skin-packs")}
            role="button"
            aria-label="skin-packs"
          >
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <FaImage className="text-default-500" />
                <span className="text-small text-default-600 truncate">
                  {t("contentpage.skin_packs", { defaultValue: "皮肤包" })}
                </span>
              </div>
              {loading ? (
                <div className="flex items-center gap-2">
                  <Spinner size="sm" />{" "}
                  <span className="text-default-500">
                    {t("common.loading", { defaultValue: "加载中" })}
                  </span>
                </div>
              ) : (
                <span className="text-base font-semibold text-default-800">
                  {skinCount}
                </span>
              )}
            </div>
          </div>
          <div
            className="rounded-xl border border-default-200 bg-white/50 dark:bg-neutral-800/40 shadow-sm backdrop-blur-sm px-3 py-3 cursor-pointer transition hover:bg-white/70 dark:hover:bg-neutral-800/60"
            onClick={() => navigate("/content/resource-packs")}
            role="button"
            aria-label="resource-packs"
          >
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <FaImage className="text-default-500" />
                <span className="text-small text-default-600 truncate">
                  {t("contentpage.resource_packs", { defaultValue: "资源包" })}
                </span>
              </div>
              {loading ? (
                <div className="flex items-center gap-2">
                  <Spinner size="sm" />{" "}
                  <span className="text-default-500">
                    {t("common.loading", { defaultValue: "加载中" })}
                  </span>
                </div>
              ) : (
                <span className="text-base font-semibold text-default-800">
                  {resCount}
                </span>
              )}
            </div>
          </div>
          <div
            className="rounded-xl border border-default-200 bg-white/50 dark:bg-neutral-800/40 shadow-sm backdrop-blur-sm px-3 py-3 cursor-pointer transition hover:bg-white/70 dark:hover:bg-neutral-800/60"
            onClick={() => navigate("/content/behavior-packs")}
            role="button"
            aria-label="behavior-packs"
          >
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <FaCogs className="text-default-500" />
                <span className="text-small text-default-600 truncate">
                  {t("contentpage.behavior_packs", { defaultValue: "行为包" })}
                </span>
              </div>
              {loading ? (
                <div className="flex items-center gap-2">
                  <Spinner size="sm" />{" "}
                  <span className="text-default-500">
                    {t("common.loading", { defaultValue: "加载中" })}
                  </span>
                </div>
              ) : (
                <span className="text-base font-semibold text-default-800">
                  {bpCount}
                </span>
              )}
            </div>
          </div>
          <div className="sm:col-span-3 flex items-center justify-end gap-2">
            <input
              ref={fileInputRef}
              type="file"
              accept=".mcworld,.mcpack,.mcaddon"
              multiple
              className="hidden"
              onChange={handleFilePick}
            />
            <Button
              color="primary"
              variant="flat"
              onPress={() =>
                navigate("/filemanager", {
                  state: {
                    allowedExt: [".mcworld", ".mcpack", ".mcaddon"],
                    multi: true,
                    returnTo: "/content",
                  },
                })
              }
              isDisabled={importing}
            >
              {t("contentpage.import_button", {
                defaultValue: "导入 .mcworld/.mcpack/.mcaddon",
              })}
            </Button>
          </div>
        </div>
      </motion.div>
    </motion.div>
  );
}
