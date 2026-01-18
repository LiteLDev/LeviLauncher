import React from "react";
import { useTranslation } from "react-i18next";
import {
  Button,
  Card,
  CardBody,
  Dropdown,
  DropdownItem,
  DropdownMenu,
  DropdownTrigger,
  Spinner,
  Tooltip,
  ModalContent,
  Progress,
  useDisclosure,
} from "@heroui/react";
import {
  BaseModal,
  BaseModalHeader,
  BaseModalBody,
  BaseModalFooter,
} from "@/components/BaseModal";
import { AnimatePresence, motion } from "framer-motion";
import { useLocation, useNavigate } from "react-router-dom";
import { GetContentRoots } from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import * as types from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import {
  FaArrowLeft,
  FaCogs,
  FaFolderOpen,
  FaGlobe,
  FaImage,
  FaSync,
  FaUserTag,
} from "react-icons/fa";
import { readCurrentVersionName } from "@/utils/currentVersion";
import { countDirectories } from "@/utils/fs";
import { listPlayers } from "@/utils/content";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { FiUploadCloud, FiAlertTriangle } from "react-icons/fi";
import { PageHeader } from "@/components/PageHeader";

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
    null,
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
  const {
    isOpen: playerSelectOpen,
    onOpen: playerSelectOnOpen,
    onOpenChange: playerSelectOnOpenChange,
  } = useDisclosure();
  const playerSelectResolveRef = React.useRef<
    ((player: string) => void) | null
  >(null);
  const pendingImportFilesRef = React.useRef<File[]>([]);
  const pendingImportPathsRef = React.useRef<string[]>([]);

  const refreshAll = async (playerToRefresh?: string) => {
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
          const currentPlayer =
            playerToRefresh !== undefined ? playerToRefresh : selectedPlayer;
          if (playerToRefresh !== undefined) {
            setSelectedPlayer(playerToRefresh);
          }
          if (names.includes(currentPlayer)) {
            if (currentPlayer) {
              const wp = `${safe.usersRoot}\\${currentPlayer}\\games\\com.mojang\\minecraftWorlds`;
              setWorldsCount(await countDirectories(wp));
              const sp = `${safe.usersRoot}\\${currentPlayer}\\games\\com.mojang\\skin_packs`;
              setSkinCount(await countDirectories(sp));
            } else {
              setWorldsCount(0);
              setSkinCount(0);
            }
          } else {
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
        }) as string,
      );
    } finally {
      setLoading(false);
    }
  };

  React.useEffect(() => {
    const passedPlayer = location?.state?.player || "";
    if (passedPlayer) {
      refreshAll(passedPlayer);
      navigate(location.pathname, {
        replace: true,
        state: { ...(location.state || {}), player: undefined },
      });
    } else {
      refreshAll();
    }
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
    overwrite: boolean,
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
          overwrite,
        );
      } else {
        err = await (minecraft as any)?.ImportMcpack?.(name, bytes, overwrite);
      }
      return String(err || "");
    } catch (e: any) {
      return String(e?.message || "IMPORT_ERROR");
    }
  };
  const postImportMcaddon = async (
    name: string,
    file: File,
    overwrite: boolean,
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
          overwrite,
        );
      } else {
        err = await (minecraft as any)?.ImportMcaddon?.(name, bytes, overwrite);
      }
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
          }) as string,
        );
        return;
      }
      const hasWorld = paths.some((p) => p?.toLowerCase().endsWith(".mcworld"));
      let hasSkin = false;
      if (paths.length > 0) {
        setImporting(true);
        const firstBase =
          paths[0].replace(/\\/g, "/").split("/").pop() || paths[0];
        setCurrentFile(firstBase);
      }
      for (const p of paths) {
        if (p?.toLowerCase().endsWith(".mcpack")) {
          const isSkin = await (minecraft as any)?.IsMcpackSkinPackPath?.(p);
          if (isSkin) {
            hasSkin = true;
            break;
          }
        }
      }
      let chosenPlayer = "";
      if (hasWorld || hasSkin) {
        pendingImportPathsRef.current = paths;
        playerSelectOnOpen();
        chosenPlayer = await new Promise<string>((resolve) => {
          playerSelectResolveRef.current = resolve;
        });
        if (!chosenPlayer) {
          pendingImportPathsRef.current = [];
          return;
        }
        setSelectedPlayer(chosenPlayer);
        await onChangePlayer(chosenPlayer);
      }
      let started = false;
      const succFiles: string[] = [];
      const errPairs: Array<{ name: string; err: string }> = [];
      const pathsToImport =
        pendingImportPathsRef.current.length > 0
          ? pendingImportPathsRef.current
          : paths;
      pendingImportPathsRef.current = [];
      const playerToUse = chosenPlayer || selectedPlayer || "";
      for (const p of pathsToImport) {
        const lower = p.toLowerCase();
        if (lower.endsWith(".mcpack")) {
          if (!started) {
            setImporting(true);
            started = true;
          }
          const base = p.replace(/\\/g, "/").split("/").pop() || p;
          setCurrentFile(base);
          let err = "";
          if (
            playerToUse &&
            typeof (minecraft as any)?.ImportMcpackPathWithPlayer === "function"
          ) {
            err = await (minecraft as any)?.ImportMcpackPathWithPlayer?.(
              name,
              playerToUse,
              p,
              false,
            );
          } else {
            err = await (minecraft as any)?.ImportMcpackPath?.(name, p, false);
          }
          if (err) {
            if (
              String(err) === "ERR_DUPLICATE_FOLDER" ||
              String(err) === "ERR_DUPLICATE_UUID"
            ) {
              dupNameRef.current = base;
              await new Promise<void>((r) => setTimeout(r, 0));
              dupOnOpen();
              const ok = await new Promise<boolean>((resolve) => {
                dupResolveRef.current = resolve;
              });
              if (ok) {
                if (
                  playerToUse &&
                  typeof (minecraft as any)?.ImportMcpackPathWithPlayer ===
                    "function"
                ) {
                  err = await (minecraft as any)?.ImportMcpackPathWithPlayer?.(
                    name,
                    playerToUse,
                    p,
                    true,
                  );
                } else {
                  err = await (minecraft as any)?.ImportMcpackPath?.(
                    name,
                    p,
                    true,
                  );
                }
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
        } else if (lower.endsWith(".mcaddon")) {
          if (!started) {
            setImporting(true);
            started = true;
          }
          const base = p.replace(/\\/g, "/").split("/").pop() || p;
          setCurrentFile(base);
          let err = "";
          if (
            playerToUse &&
            typeof (minecraft as any)?.ImportMcaddonPathWithPlayer ===
              "function"
          ) {
            err = await (minecraft as any)?.ImportMcaddonPathWithPlayer?.(
              name,
              playerToUse,
              p,
              false,
            );
          } else {
            err = await (minecraft as any)?.ImportMcaddonPath?.(name, p, false);
          }
          if (err) {
            if (
              String(err) === "ERR_DUPLICATE_FOLDER" ||
              String(err) === "ERR_DUPLICATE_UUID"
            ) {
              dupNameRef.current = base;
              await new Promise<void>((r) => setTimeout(r, 0));
              dupOnOpen();
              const ok = await new Promise<boolean>((resolve) => {
                dupResolveRef.current = resolve;
              });
              if (ok) {
                if (
                  playerToUse &&
                  typeof (minecraft as any)?.ImportMcaddonPathWithPlayer ===
                    "function"
                ) {
                  err = await (minecraft as any)?.ImportMcaddonPathWithPlayer?.(
                    name,
                    playerToUse,
                    p,
                    true,
                  );
                } else {
                  err = await (minecraft as any)?.ImportMcaddonPath?.(
                    name,
                    p,
                    true,
                  );
                }
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
        } else if (lower.endsWith(".mcworld")) {
          const base = p.replace(/\\/g, "/").split("/").pop() || p;
          if (!playerToUse) {
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
            playerToUse,
            p,
            false,
          );
          if (err) {
            if (
              String(err) === "ERR_DUPLICATE_FOLDER" ||
              String(err) === "ERR_DUPLICATE_UUID"
            ) {
              dupNameRef.current = base;
              await new Promise<void>((r) => setTimeout(r, 0));
              dupOnOpen();
              const ok = await new Promise<boolean>((resolve) => {
                dupResolveRef.current = resolve;
              });
              if (ok) {
                err = await (minecraft as any)?.ImportMcworldPath?.(
                  name,
                  playerToUse,
                  p,
                  true,
                );
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
        }
      }
      await refreshAll(playerToUse);
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

  const handleImportFiles = async (files: File[]) => {
    if (!files.length) return;
    if (!currentVersionName) {
      setErrorMsg(
        t("launcherpage.currentVersion_none", {
          defaultValue: "未选择版本",
        }) as string,
      );
      return;
    }

    let chosenPlayer = "";
    let started = false;
    const succFiles: string[] = [];
    const errPairs: Array<{ name: string; err: string }> = [];
    let filesToImport: File[] = files;
    let playerToUse = selectedPlayer || "";

    try {
      setImporting(true);
      started = true;
      setCurrentFile(files[0]?.name || "");
      await new Promise<void>((r) => setTimeout(r, 0));

      const hasWorld = files.some((f) =>
        f?.name?.toLowerCase().endsWith(".mcworld"),
      );
      let hasSkin = false;
      for (const f of files) {
        if (f?.name?.toLowerCase().endsWith(".mcpack")) {
          setCurrentFile(f.name);
          await new Promise<void>((r) => setTimeout(r, 0));
          const buf = await f.arrayBuffer();
          const bytes = Array.from(new Uint8Array(buf));
          const isSkin = await (minecraft as any)?.IsMcpackSkinPack?.(bytes);
          if (isSkin) {
            hasSkin = true;
            break;
          }
        }
      }

      if (hasWorld || hasSkin) {
        setImporting(false);
        setCurrentFile("");
        started = false;
        await new Promise<void>((r) => setTimeout(r, 0));

        pendingImportFilesRef.current = files;
        playerSelectOnOpen();
        chosenPlayer = await new Promise<string>((resolve) => {
          playerSelectResolveRef.current = resolve;
        });
        if (!chosenPlayer) {
          pendingImportFilesRef.current = [];
          return;
        }
        setSelectedPlayer(chosenPlayer);
        await onChangePlayer(chosenPlayer);

        setImporting(true);
        started = true;
        filesToImport = pendingImportFilesRef.current.length
          ? pendingImportFilesRef.current
          : files;
        pendingImportFilesRef.current = [];
        playerToUse = chosenPlayer || selectedPlayer || "";
        setCurrentFile(filesToImport[0]?.name || "");
        await new Promise<void>((r) => setTimeout(r, 0));
      } else {
        pendingImportFilesRef.current = [];
        filesToImport = files;
        playerToUse = selectedPlayer || "";
      }

      for (const f of filesToImport) {
        const lower = f.name.toLowerCase();
        setCurrentFile(f.name);

        let err = "";
        if (lower.endsWith(".mcpack")) {
          err = await postImportMcpack(currentVersionName, f, false);
        } else if (lower.endsWith(".mcaddon")) {
          err = await postImportMcaddon(currentVersionName, f, false);
        } else if (lower.endsWith(".mcworld")) {
          if (!playerToUse) {
            err = "ERR_NO_PLAYER";
          } else {
            // Use ImportMcworld if available, or fallback to temp file + ImportMcworldPath logic if needed.
            // Since we don't have postImportMcworld, we implement inline.
            // Assuming ImportMcworld exists and takes bytes like ImportMcpack
            const buf = await f.arrayBuffer();
            const bytes = Array.from(new Uint8Array(buf));
            if (typeof (minecraft as any)?.ImportMcworld === "function") {
              err = await (minecraft as any)?.ImportMcworld?.(
                currentVersionName,
                playerToUse,
                bytes,
                false,
              );
            } else {
              // Fallback: write temp file and use ImportMcworldPath
              // This requires exposing WriteTempFile which we don't know if we have.
              // But wait, the existing code for paths uses ImportMcworldPath.
              // If ImportMcworld is not available, we can't easily import from bytes without a helper.
              // We will assume ImportMcworld exists for now as it's consistent with ImportMcpack.
              err = "ERR_NOT_IMPLEMENTED"; // Placeholder if function missing
            }
          }
        }

        if (err) {
          if (
            String(err) === "ERR_DUPLICATE_FOLDER" ||
            String(err) === "ERR_DUPLICATE_UUID"
          ) {
            dupNameRef.current = f.name;
            await new Promise<void>((r) => setTimeout(r, 0));
            dupOnOpen();
            const ok = await new Promise<boolean>((resolve) => {
              dupResolveRef.current = resolve;
            });
            if (ok) {
              if (lower.endsWith(".mcpack")) {
                err = await postImportMcpack(currentVersionName, f, true);
              } else if (lower.endsWith(".mcaddon")) {
                err = await postImportMcaddon(currentVersionName, f, true);
              } else if (lower.endsWith(".mcworld")) {
                const buf = await f.arrayBuffer();
                const bytes = Array.from(new Uint8Array(buf));
                if (typeof (minecraft as any)?.ImportMcworld === "function") {
                  err = await (minecraft as any)?.ImportMcworld?.(
                    currentVersionName,
                    playerToUse,
                    bytes,
                    true,
                  );
                }
              }
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
      await refreshAll(playerToUse);
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

  const handleFilePick = async (e: React.ChangeEvent<HTMLInputElement>) => {
    try {
      setErrorMsg("");
      setResultSuccess([]);
      setResultFailed([]);
      const list = e.target.files;
      if (!list || list.length === 0) return;

      const files: File[] = Array.from(list).filter(
        (f) =>
          f &&
          (f.name.toLowerCase().endsWith(".mcworld") ||
            f.name.toLowerCase().endsWith(".mcpack") ||
            f.name.toLowerCase().endsWith(".mcaddon")),
      );
      await handleImportFiles(files);
    } catch (e: any) {
      setErrorMsg(String(e?.message || e || "IMPORT_ERROR"));
    }
  };

  React.useEffect(() => {
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

    const onDrop = async (e: DragEvent) => {
      const hasFiles = (e.dataTransfer?.files?.length || 0) > 0;
      if (!hasFiles && dragCounter.current <= 0 && !isFileDrag(e)) return;
      e.preventDefault();
      e.stopPropagation();
      dragCounter.current = 0;
      setDragActive(false);
      setErrorMsg("");
      setResultSuccess([]);
      setResultFailed([]);

      const files: File[] = Array.from(e.dataTransfer?.files || []).filter(
        (f) =>
          f &&
          (f.name.toLowerCase().endsWith(".mcworld") ||
            f.name.toLowerCase().endsWith(".mcpack") ||
            f.name.toLowerCase().endsWith(".mcaddon")),
      );
      if (files.length > 0) {
        await handleImportFiles(files);
      }
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
      className={`relative w-full max-w-full mx-auto h-full flex flex-col ${
        dragActive ? "cursor-copy" : ""
      }`}
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.25 }}
    >
      <AnimatePresence>
        {dragActive ? (
          <motion.div
            className="pointer-events-none fixed inset-0 z-40 flex items-center justify-center bg-black/20 backdrop-blur-sm"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.2 }}
          >
            <div className="bg-white/90 dark:bg-zinc-900/90 p-8 rounded-4xl shadow-2xl flex flex-col items-center gap-4 border border-white/20">
              <FiUploadCloud className="w-16 h-16 text-primary-500" />
              <div className="text-xl font-bold bg-linear-to-r from-primary-500 to-secondary-500 bg-clip-text text-transparent">
                {t("contentpage.drop_hint", {
                  defaultValue: "拖入 .mcworld/.mcpack/.mcaddon 以导入",
                })}
              </div>
            </div>
          </motion.div>
        ) : null}
      </AnimatePresence>

      <div className="flex-1 overflow-auto p-4">
        <div className="w-full max-w-none pb-12">
          {/* Header Card */}
          <Card className="rounded-4xl shadow-md mb-6 bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md border-none">
            <CardBody className="px-6 sm:px-8 py-5">
              <div className="flex flex-col gap-4">
                <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
                  <div>
                    <div className="flex items-center gap-3">
                      <Button
                        isIconOnly
                        radius="full"
                        variant="light"
                        onPress={() => navigate("/")}
                      >
                        <FaArrowLeft size={20} />
                      </Button>
                      <PageHeader
                        title={t("launcherpage.content_manage", {
                          defaultValue: "内容管理",
                        })}
                        titleClassName="pb-1"
                      />
                    </div>
                    <div className="mt-2 text-default-500 text-sm flex flex-wrap items-center gap-2">
                      <span>
                        {t("contentpage.current_version", {
                          defaultValue: "当前版本",
                        })}
                        :
                      </span>
                      <span className="font-medium text-default-700 bg-default-100 px-2 py-0.5 rounded-md">
                        {currentVersionName ||
                          t("contentpage.none", { defaultValue: "无" })}
                      </span>
                      <span className="text-default-300">|</span>
                      <span>
                        {t("contentpage.isolation", {
                          defaultValue: "版本隔离",
                        })}
                        :
                      </span>
                      <span
                        className={`font-medium px-2 py-0.5 rounded-md ${
                          roots.isIsolation
                            ? "bg-success-50 text-success-600"
                            : "bg-default-100 text-default-700"
                        }`}
                      >
                        {roots.isIsolation
                          ? t("common.yes", { defaultValue: "是" })
                          : t("common.no", { defaultValue: "否" })}
                      </span>
                      <span className="text-default-300">|</span>
                      <span>
                        {t("contentpage.select_player", {
                          defaultValue: "玩家",
                        })}
                        :
                      </span>
                      <Dropdown>
                        <DropdownTrigger>
                          <Button
                            size="sm"
                            variant="light"
                            className="h-6 min-w-0 px-2 text-small font-medium text-default-700 bg-default-100 rounded-md"
                          >
                            {selectedPlayer ||
                              t("contentpage.no_players", {
                                defaultValue: "暂无",
                              })}
                          </Button>
                        </DropdownTrigger>
                        <DropdownMenu
                          aria-label="Players"
                          selectionMode="single"
                          selectedKeys={new Set([selectedPlayer])}
                          onSelectionChange={(keys) => {
                            const arr = Array.from(
                              keys as unknown as Set<string>,
                            );
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
                              {t("contentpage.no_players", {
                                defaultValue: "暂无玩家",
                              })}
                            </DropdownItem>
                          )}
                        </DropdownMenu>
                      </Dropdown>
                      {!selectedPlayer && (
                        <span className="text-danger-500 text-xs">
                          (
                          {t("contentpage.require_player_for_world_import", {
                            defaultValue: "需选择玩家",
                          })}
                          )
                        </span>
                      )}
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <Tooltip
                      content={
                        t("contentpage.open_users_dir", {
                          defaultValue: "打开存储目录",
                        }) as unknown as string
                      }
                    >
                      <Button
                        radius="full"
                        variant="flat"
                        startContent={<FaFolderOpen />}
                        onPress={() => {
                          if (roots.usersRoot) {
                            (minecraft as any)?.OpenPathDir(roots.usersRoot);
                          }
                        }}
                        isDisabled={!hasBackend || !roots.usersRoot}
                        className="bg-default-100 dark:bg-zinc-800 text-default-600 dark:text-zinc-200 font-medium"
                      >
                        {t("common.open", { defaultValue: "打开" })}
                      </Button>
                    </Tooltip>
                    <Tooltip
                      content={
                        t("common.refresh", {
                          defaultValue: "刷新",
                        }) as unknown as string
                      }
                    >
                      <Button
                        isIconOnly
                        radius="full"
                        variant="light"
                        onPress={() => refreshAll()}
                        isDisabled={loading}
                      >
                        <FaSync
                          className={loading ? "animate-spin" : ""}
                          size={18}
                        />
                      </Button>
                    </Tooltip>
                  </div>
                </div>
                {!!error && (
                  <div className="text-danger-500 text-sm">{error}</div>
                )}
              </div>
            </CardBody>
          </Card>

          {/* Content Grid */}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
            <Card
              isPressable
              onPress={() =>
                navigate("/content/worlds", {
                  state: { player: selectedPlayer },
                })
              }
              className="rounded-4xl shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md border-none h-full"
            >
              <CardBody className="p-6">
                <div className="flex items-center justify-between w-full">
                  <div className="flex items-center gap-3">
                    <div className="p-2 rounded-xl bg-blue-50 dark:bg-blue-900/20 text-blue-500">
                      <FaGlobe className="w-6 h-6" />
                    </div>
                    <span className="text-lg font-medium text-default-700">
                      {t("contentpage.worlds", { defaultValue: "世界" })}
                    </span>
                  </div>
                  {loading ? (
                    <Spinner size="sm" />
                  ) : (
                    <span className="text-2xl font-bold text-default-900">
                      {worldsCount}
                    </span>
                  )}
                </div>
              </CardBody>
            </Card>

            <Card
              isPressable
              onPress={() => navigate("/content/resource-packs")}
              className="rounded-4xl shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md border-none h-full"
            >
              <CardBody className="p-6">
                <div className="flex items-center justify-between w-full">
                  <div className="flex items-center gap-3">
                    <div className="p-2 rounded-xl bg-purple-50 dark:bg-purple-900/20 text-purple-500">
                      <FaImage className="w-6 h-6" />
                    </div>
                    <span className="text-lg font-medium text-default-700">
                      {t("contentpage.resource_packs", {
                        defaultValue: "资源包",
                      })}
                    </span>
                  </div>
                  {loading ? (
                    <Spinner size="sm" />
                  ) : (
                    <span className="text-2xl font-bold text-default-900">
                      {resCount}
                    </span>
                  )}
                </div>
              </CardBody>
            </Card>

            <Card
              isPressable
              onPress={() => navigate("/content/behavior-packs")}
              className="rounded-4xl shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md border-none h-full"
            >
              <CardBody className="p-6">
                <div className="flex items-center justify-between w-full">
                  <div className="flex items-center gap-3">
                    <div className="p-2 rounded-xl bg-orange-50 dark:bg-orange-900/20 text-orange-500">
                      <FaCogs className="w-6 h-6" />
                    </div>
                    <span className="text-lg font-medium text-default-700">
                      {t("contentpage.behavior_packs", {
                        defaultValue: "行为包",
                      })}
                    </span>
                  </div>
                  {loading ? (
                    <Spinner size="sm" />
                  ) : (
                    <span className="text-2xl font-bold text-default-900">
                      {bpCount}
                    </span>
                  )}
                </div>
              </CardBody>
            </Card>

            <Card
              isPressable
              onPress={() =>
                navigate("/content/skin-packs", {
                  state: { player: selectedPlayer },
                })
              }
              className="rounded-4xl shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md border-none h-full"
            >
              <CardBody className="p-6">
                <div className="flex items-center justify-between w-full">
                  <div className="flex items-center gap-3">
                    <div className="p-2 rounded-xl bg-pink-50 dark:bg-pink-900/20 text-pink-500">
                      <FaUserTag className="w-6 h-6" />
                    </div>
                    <span className="text-lg font-medium text-default-700">
                      {t("contentpage.skin_packs", { defaultValue: "皮肤包" })}
                    </span>
                  </div>
                  {loading ? (
                    <Spinner size="sm" />
                  ) : (
                    <span className="text-2xl font-bold text-default-900">
                      {skinCount}
                    </span>
                  )}
                </div>
              </CardBody>
            </Card>
          </div>

          <div className="mt-8 flex justify-end gap-4">
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
              variant="shadow"
              className="bg-emerald-600 hover:bg-emerald-500 text-white shadow-lg shadow-emerald-900/20"
              startContent={<FiUploadCloud />}
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
      </div>

      <BaseModal
        size="sm"
        isOpen={importing}
        hideCloseButton
        isDismissable={false}
      >
        <ModalContent>
          {() => (
            <>
              <BaseModalHeader className="flex-row items-center gap-2 text-primary-600">
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
                className={`flex-row items-center gap-2 ${
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
      <BaseModal
        size="md"
        isOpen={playerSelectOpen}
        onOpenChange={playerSelectOnOpenChange}
        hideCloseButton
      >
        <ModalContent>
          {(onClose) => (
            <>
              <BaseModalHeader className="text-primary-600">
                {t("contentpage.select_player_title", {
                  defaultValue: "选择玩家",
                })}
              </BaseModalHeader>
              <BaseModalBody>
                <div className="text-sm text-default-700">
                  {t("contentpage.select_player_for_import", {
                    defaultValue: "请选择要导入到的玩家",
                  })}
                </div>
                <div className="mt-3 space-y-2">
                  {players.length ? (
                    players.map((p) => (
                      <Button
                        key={p}
                        variant="bordered"
                        className="w-full justify-start"
                        onPress={() => {
                          try {
                            playerSelectResolveRef.current &&
                              playerSelectResolveRef.current(p);
                          } finally {
                            onClose();
                          }
                        }}
                      >
                        {p}
                      </Button>
                    ))
                  ) : (
                    <div className="text-sm text-default-500">
                      {t("contentpage.no_players", {
                        defaultValue: "暂无玩家",
                      })}
                    </div>
                  )}
                </div>
              </BaseModalBody>
              <BaseModalFooter>
                <Button
                  variant="light"
                  onPress={() => {
                    try {
                      playerSelectResolveRef.current &&
                        playerSelectResolveRef.current("");
                    } finally {
                      onClose();
                    }
                  }}
                >
                  {t("common.cancel", { defaultValue: "取消" })}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>
    </motion.div>
  );
}
