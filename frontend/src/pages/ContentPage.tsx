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
import { Dialogs } from "@wailsio/runtime";
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
  FaCogs,
  FaFolderOpen,
  FaGlobe,
  FaImage,
  FaSync,
  FaUserTag,
  FaServer,
} from "react-icons/fa";
import { readCurrentVersionName } from "@/utils/currentVersion";
import { countDirectories } from "@/utils/fs";
import {
  getPlayerGamertagMap,
  listPlayers,
  resolvePlayerDisplayName,
} from "@/utils/content";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { FiUploadCloud, FiAlertTriangle } from "react-icons/fi";
import { PageHeader } from "@/components/PageHeader";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { cn } from "@/utils/cn";

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
  const [playerGamertagMap, setPlayerGamertagMap] = React.useState<
    Record<string, string>
  >({});
  const [worldsCount, setWorldsCount] = React.useState<number>(0);
  const [resCount, setResCount] = React.useState<number>(0);
  const [bpCount, setBpCount] = React.useState<number>(0);
  const [skinCount, setSkinCount] = React.useState<number>(0);
  const [serversCount, setServersCount] = React.useState<number>(0);
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
        setPlayerGamertagMap({});
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

          let nextPlayer = names[0] || "";
          const currentPlayer =
            playerToRefresh !== undefined ? playerToRefresh : selectedPlayer;

          if (names.includes(currentPlayer)) {
            nextPlayer = currentPlayer;
          }

          if (playerToRefresh !== undefined) {
            setSelectedPlayer(playerToRefresh);
          } else if (!names.includes(currentPlayer)) {
            setSelectedPlayer(nextPlayer);
          }

          const loadCounts = async (player: string) => {
            if (player) {
              const wp = `${safe.usersRoot}\\${player}\\games\\com.mojang\\minecraftWorlds`;
              setWorldsCount(await countDirectories(wp));
              const sp = `${safe.usersRoot}\\${player}\\games\\com.mojang\\skin_packs`;
              setSkinCount(await countDirectories(sp));
              const srvs = await (minecraft as any)?.ListServers?.(
                name,
                player,
              );
              setServersCount(srvs?.length || 0);
            } else {
              setWorldsCount(0);
              setSkinCount(0);
              setServersCount(0);
            }
          };

          await loadCounts(
            names.includes(currentPlayer) ? currentPlayer : nextPlayer,
          );

          (async () => {
            try {
              const map = await getPlayerGamertagMap(safe.usersRoot);
              setPlayerGamertagMap(map);

              if (
                playerToRefresh === undefined &&
                !names.includes(currentPlayer)
              ) {
                const tag = await (minecraft as any)?.GetLocalUserGamertag?.();
                if (tag) {
                  let matched = "";
                  for (const p of names) {
                    if (map[p] === tag) {
                      matched = p;
                      break;
                    }
                  }
                  if (matched && matched !== nextPlayer) {
                    setSelectedPlayer(matched);
                    await loadCounts(matched);
                  }
                }
              }
            } catch {}
          })();
        } else {
          setPlayers([]);
          setSelectedPlayer("");
          setPlayerGamertagMap({});
          setWorldsCount(0);
          setSkinCount(0);
          setServersCount(0);
        }
        setResCount(await countDirectories(safe.resourcePacks));
        setBpCount(await countDirectories(safe.behaviorPacks));
      }
    } catch (e) {
      setError(t("contentpage.error_resolve_paths") as string);
    } finally {
      setLoading(false);
    }
  };

  React.useEffect(() => {
    let passedPlayer = location?.state?.player;
    if (!passedPlayer) {
      passedPlayer = localStorage.getItem("content.selectedPlayer") || "";
    }

    if (passedPlayer) {
      refreshAll(passedPlayer);
      if (location?.state?.player) {
        navigate(location.pathname, {
          replace: true,
          state: { ...(location.state || {}), player: undefined },
        });
      }
    } else {
      refreshAll();
    }
  }, []);

  React.useEffect(() => {
    if (selectedPlayer) {
      localStorage.setItem("content.selectedPlayer", selectedPlayer);
    }
  }, [selectedPlayer]);

  const onChangePlayer = async (player: string) => {
    setLoading(true);
    setSelectedPlayer(player);
    try {
      if (!hasBackend || !roots.usersRoot || !player) {
        setWorldsCount(0);
        setSkinCount(0);
        setServersCount(0);
        return;
      }
      const wp = `${roots.usersRoot}\\${player}\\games\\com.mojang\\minecraftWorlds`;
      setWorldsCount(await countDirectories(wp));
      const sp = `${roots.usersRoot}\\${player}\\games\\com.mojang\\skin_packs`;
      setSkinCount(await countDirectories(sp));
      const srvs = await (minecraft as any)?.ListServers?.(
        currentVersionName || readCurrentVersionName(),
        player,
      );
      setServersCount(srvs?.length || 0);
    } finally {
      setLoading(false);
    }
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
        return t("contentpage.no_player_selected") as string;
      case "ERR_INVALID_NAME":
        return t("mods.err_invalid_name") as string;
      case "ERR_ACCESS_VERSIONS_DIR":
        return t("mods.err_access_versions_dir") as string;
      case "ERR_CREATE_TARGET_DIR":
        return t("mods.err_create_target_dir") as string;
      case "ERR_OPEN_ZIP":
        return t("mods.err_open_zip") as string;
      case "ERR_MANIFEST_NOT_FOUND":
        return t("mods.err_manifest_not_found") as string;
      case "ERR_INVALID_PACKAGE":
        return t("mods.err_invalid_package") as string;
      case "ERR_DUPLICATE_FOLDER":
        return t("mods.err_duplicate_folder") as string;
      case "ERR_READ_ZIP_ENTRY":
        return t("mods.err_read_zip_entry") as string;
      case "ERR_WRITE_FILE":
        return t("mods.err_write_file") as string;
      default:
        return code || (t("mods.err_unknown") as string);
    }
  };

  const doImportFromPaths = async (paths: string[]) => {
    try {
      if (!paths?.length) return;
      const name = currentVersionName || readCurrentVersionName();
      if (!name) {
        setErrorMsg(t("launcherpage.currentVersion_none") as string);
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
      setErrorMsg(t("launcherpage.currentVersion_none") as string);
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
              err = "ERR_NOT_IMPLEMENTED";
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
    <PageContainer className={`relative ${dragActive ? "cursor-copy" : ""}`}>
      <AnimatePresence>
        {dragActive ? (
          <motion.div
            className="pointer-events-none fixed inset-0 z-40 flex items-center justify-center bg-black/20 backdrop-blur-sm"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.2 }}
          >
            <div
              className={cn(
                "p-8 flex flex-col items-center gap-4 border border-white/20",
                LAYOUT.GLASS_CARD.BASE,
                "bg-white/90 dark:bg-zinc-900/90",
              )}
            >
              <FiUploadCloud className="w-16 h-16 text-primary-500" />
              <div className="text-xl font-bold bg-linear-to-r from-primary-500 to-secondary-500 bg-clip-text text-transparent">
                {t("contentpage.drop_hint")}
              </div>
            </div>
          </motion.div>
        ) : null}
      </AnimatePresence>

      {/* Header Card */}
      <Card className={LAYOUT.GLASS_CARD.BASE}>
        <CardBody className="p-6">
          <div className="flex flex-col gap-6">
            <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
              <div>
                <div className="flex items-center gap-3">
                  <PageHeader
                    title={t("launcherpage.content_manage")}
                    titleClassName="pb-1"
                  />
                </div>
                <div className="mt-2 text-default-500 text-sm flex flex-wrap items-center gap-2">
                  <span>{t("contentpage.current_version")}:</span>
                  <span className="font-medium text-default-700 bg-default-100 px-2 py-0.5 rounded-md">
                    {currentVersionName || t("contentpage.none")}
                  </span>
                  <span className="text-default-300">|</span>
                  <span>{t("contentpage.isolation")}:</span>
                  <span
                    className={`font-medium px-2 py-0.5 rounded-md ${
                      roots.isIsolation
                        ? "bg-success-50 text-success-600"
                        : "bg-default-100 text-default-700"
                    }`}
                  >
                    {roots.isIsolation ? t("common.yes") : t("common.no")}
                  </span>
                  <span className="text-default-300">|</span>
                  <span>{t("contentpage.select_player")}:</span>
                  <Dropdown>
                    <DropdownTrigger>
                      <Button
                        size="sm"
                        variant="light"
                        className="h-6 min-w-0 px-2 text-small font-medium text-default-700 bg-default-100 rounded-md"
                      >
                        {selectedPlayer
                          ? resolvePlayerDisplayName(
                              selectedPlayer,
                              playerGamertagMap,
                            )
                          : t("contentpage.no_players")}
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
                          <DropdownItem
                            key={p}
                            textValue={resolvePlayerDisplayName(
                              p,
                              playerGamertagMap,
                            )}
                          >
                            {resolvePlayerDisplayName(p, playerGamertagMap)}
                          </DropdownItem>
                        ))
                      ) : (
                        <DropdownItem key="none" isDisabled>
                          {t("contentpage.no_players")}
                        </DropdownItem>
                      )}
                    </DropdownMenu>
                  </Dropdown>
                  {!selectedPlayer && (
                    <span className="text-danger-500 text-xs">
                      ({t("contentpage.require_player_for_world_import")})
                    </span>
                  )}
                </div>
              </div>
              <div className="flex items-center gap-2">
                <Button
                  radius="full"
                  className="bg-emerald-600 text-white font-medium shadow-sm"
                  startContent={<FiUploadCloud />}
                  onPress={async () => {
                    try {
                      const paths = await Dialogs.OpenFile({
                        Title: t("contentpage.import_button"),
                        Filters: [
                          {
                            DisplayName: "Content Files",
                            Pattern: "*.mcworld;*.mcpack;*.mcaddon",
                          },
                        ],
                        AllowsMultipleSelection: true,
                      });
                      if (paths && Array.isArray(paths) && paths.length > 0) {
                        doImportFromPaths(paths);
                      }
                    } catch (e) {
                      console.error(e);
                    }
                  }}
                  isDisabled={importing}
                >
                  {t("contentpage.import_button")}
                </Button>
                <Tooltip
                  content={t("contentpage.open_users_dir") as unknown as string}
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
                    {t("common.open")}
                  </Button>
                </Tooltip>
                <Tooltip content={t("common.refresh") as unknown as string}>
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
            {!!error && <div className="text-danger-500 text-sm">{error}</div>}
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
          className={cn("h-full", LAYOUT.GLASS_CARD.BASE)}
        >
          <CardBody className="p-6">
            <div className="flex items-center justify-between w-full">
              <div className="flex items-center gap-3">
                <div className="p-2 rounded-xl bg-blue-50 dark:bg-blue-900/20 text-blue-500">
                  <FaGlobe className="w-6 h-6" />
                </div>
                <span className="text-lg font-medium text-default-700">
                  {t("contentpage.worlds")}
                </span>
              </div>
              <AnimatePresence mode="wait">
                {loading ? (
                  <motion.div
                    key="spinner"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                  >
                    <Spinner size="sm" />
                  </motion.div>
                ) : (
                  <motion.span
                    key="count"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                    className="text-2xl font-bold text-default-900"
                  >
                    {worldsCount}
                  </motion.span>
                )}
              </AnimatePresence>
            </div>
          </CardBody>
        </Card>

        <Card
          isPressable
          onPress={() => navigate("/content/resourcePacks")}
          className={cn("h-full", LAYOUT.GLASS_CARD.BASE)}
        >
          <CardBody className="p-6">
            <div className="flex items-center justify-between w-full">
              <div className="flex items-center gap-3">
                <div className="p-2 rounded-xl bg-purple-50 dark:bg-purple-900/20 text-purple-500">
                  <FaImage className="w-6 h-6" />
                </div>
                <span className="text-lg font-medium text-default-700">
                  {t("contentpage.resource_packs")}
                </span>
              </div>
              <AnimatePresence mode="wait">
                {loading ? (
                  <motion.div
                    key="spinner"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                  >
                    <Spinner size="sm" />
                  </motion.div>
                ) : (
                  <motion.span
                    key="count"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                    className="text-2xl font-bold text-default-900"
                  >
                    {resCount}
                  </motion.span>
                )}
              </AnimatePresence>
            </div>
          </CardBody>
        </Card>

        <Card
          isPressable
          onPress={() => navigate("/content/behaviorPacks")}
          className={cn("h-full", LAYOUT.GLASS_CARD.BASE)}
        >
          <CardBody className="p-6">
            <div className="flex items-center justify-between w-full">
              <div className="flex items-center gap-3">
                <div className="p-2 rounded-xl bg-orange-50 dark:bg-orange-900/20 text-orange-500">
                  <FaCogs className="w-6 h-6" />
                </div>
                <span className="text-lg font-medium text-default-700">
                  {t("contentpage.behavior_packs")}
                </span>
              </div>
              <AnimatePresence mode="wait">
                {loading ? (
                  <motion.div
                    key="spinner"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                  >
                    <Spinner size="sm" />
                  </motion.div>
                ) : (
                  <motion.span
                    key="count"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                    className="text-2xl font-bold text-default-900"
                  >
                    {bpCount}
                  </motion.span>
                )}
              </AnimatePresence>
            </div>
          </CardBody>
        </Card>

        <Card
          isPressable
          onPress={() =>
            navigate("/content/skinPacks", {
              state: { player: selectedPlayer },
            })
          }
          className={cn("h-full", LAYOUT.GLASS_CARD.BASE)}
        >
          <CardBody className="p-6">
            <div className="flex items-center justify-between w-full">
              <div className="flex items-center gap-3">
                <div className="p-2 rounded-xl bg-pink-50 dark:bg-pink-900/20 text-pink-500">
                  <FaUserTag className="w-6 h-6" />
                </div>
                <span className="text-lg font-medium text-default-700">
                  {t("contentpage.skin_packs")}
                </span>
              </div>
              <AnimatePresence mode="wait">
                {loading ? (
                  <motion.div
                    key="spinner"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                  >
                    <Spinner size="sm" />
                  </motion.div>
                ) : (
                  <motion.span
                    key="count"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                    className="text-2xl font-bold text-default-900"
                  >
                    {skinCount}
                  </motion.span>
                )}
              </AnimatePresence>
            </div>
          </CardBody>
        </Card>

        <Card
          isPressable
          onPress={() =>
            navigate("/content/servers", {
              state: { player: selectedPlayer },
            })
          }
          className={cn("h-full", LAYOUT.GLASS_CARD.BASE)}
        >
          <CardBody className="p-6">
            <div className="flex items-center justify-between w-full">
              <div className="flex items-center gap-3">
                <div className="p-2 rounded-xl bg-indigo-50 dark:bg-indigo-900/20 text-indigo-500">
                  <FaServer className="w-6 h-6" />
                </div>
                <span className="text-lg font-medium text-default-700">
                  {t("contentpage.servers")}
                </span>
              </div>
              <AnimatePresence mode="wait">
                {loading ? (
                  <motion.div
                    key="spinner"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                  >
                    <Spinner size="sm" />
                  </motion.div>
                ) : (
                  <motion.span
                    key="count"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                    className="text-2xl font-bold text-default-900"
                  >
                    {serversCount}
                  </motion.span>
                )}
              </AnimatePresence>
            </div>
          </CardBody>
        </Card>
      </div>

      <input
        ref={fileInputRef}
        type="file"
        accept=".mcworld,.mcpack,.mcaddon"
        multiple
        className="hidden"
        onChange={handleFilePick}
      />

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
                <span>{t("mods.importing_title")}</span>
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
                  {t("mods.importing_body")}
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
                    ? t("mods.summary_title_partial")
                    : t("mods.summary_title_done")}
                </span>
              </BaseModalHeader>
              <BaseModalBody>
                {resultSuccess.length ? (
                  <div className="mb-2">
                    <div className="text-sm font-semibold text-success">
                      {t("mods.summary_success")} ({resultSuccess.length})
                    </div>
                    <div className="mt-1 rounded-md bg-success/5 border border-success/30 px-3 py-2 text-success-700 text-sm wrap-break-word whitespace-pre-wrap">
                      {resultSuccess.join("\n")}
                    </div>
                  </div>
                ) : null}
                {resultFailed.length ? (
                  <div>
                    <div className="text-sm font-semibold text-danger">
                      {t("mods.summary_failed")} ({resultFailed.length})
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
                  {t("common.confirm")}
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
                {t("mods.overwrite_modal_title")}
              </BaseModalHeader>
              <BaseModalBody>
                <div className="text-sm text-default-700">
                  {t("mods.overwrite_modal_body")}
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
                  {t("common.cancel")}
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
                  {t("common.confirm")}
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
                {t("contentpage.select_player_title")}
              </BaseModalHeader>
              <BaseModalBody>
                <div className="text-sm text-default-700">
                  {t("contentpage.select_player_for_import")}
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
                        {resolvePlayerDisplayName(p, playerGamertagMap)}
                      </Button>
                    ))
                  ) : (
                    <div className="text-sm text-default-500">
                      {t("contentpage.no_players")}
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
                  {t("common.cancel")}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>
    </PageContainer>
  );
}
