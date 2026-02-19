import React from "react";
import { useDisclosure } from "@heroui/react";
import { Events } from "@wailsio/runtime";
import { useLocation, useNavigate } from "react-router-dom";
import { GetVersionMeta } from "bindings/github.com/liteldev/LeviLauncher/versionservice";
import { GetLocalUserGamertag } from "bindings/github.com/liteldev/LeviLauncher/userservice";
import {
  GetContentRoots,
} from "bindings/github.com/liteldev/LeviLauncher/contentservice";
import * as types from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import { readCurrentVersionName } from "@/utils/currentVersion";
import { compareVersions } from "@/utils/version";
import { countDirectories } from "@/utils/fs";
import {
  getPlayerGamertagMap,
  listPlayers,
} from "@/utils/content";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import * as contentService from "bindings/github.com/liteldev/LeviLauncher/contentservice";

type TFunc = (key: string, opts?: Record<string, unknown>) => string;

export const useContentPage = (t: TFunc) => {
  const navigate = useNavigate();
  const location = useLocation() as any;
  const hasBackend = minecraft !== undefined && contentService !== undefined;

  // --- State ---
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
  const [isSharedMode, setIsSharedMode] = React.useState<boolean>(false);
  const [playerGamertagMap, setPlayerGamertagMap] = React.useState<
    Record<string, string>
  >({});
  const [worldsCount, setWorldsCount] = React.useState<number>(0);
  const [resCount, setResCount] = React.useState<number>(0);
  const [bpCount, setBpCount] = React.useState<number>(0);
  const [skinCount, setSkinCount] = React.useState<number>(0);
  const [serversCount, setServersCount] = React.useState<number>(0);
  const [screenshotsCount, setScreenshotsCount] = React.useState<number>(0);
  const [importing, setImporting] = React.useState(false);
  const [errorMsg, setErrorMsg] = React.useState("");
  const [currentFile, setCurrentFile] = React.useState("");
  const [resultSuccess, setResultSuccess] = React.useState<string[]>([]);
  const [resultFailed, setResultFailed] = React.useState<
    Array<{ name: string; err: string }>
  >([]);

  // --- Refs ---
  const dupResolveRef = React.useRef<((overwrite: boolean) => void) | null>(
    null,
  );
  const dupNameRef = React.useRef<string>("");
  const playerSelectResolveRef = React.useRef<
    ((player: string) => void) | null
  >(null);
  const pendingImportPathsRef = React.useRef<string[]>([]);

  // --- Disclosures ---
  const {
    isOpen: errOpen,
    onOpen: errOnOpen,
    onClose: errOnClose,
    onOpenChange: errOnOpenChange,
  } = useDisclosure();
  const {
    isOpen: dupOpen,
    onOpen: dupOnOpen,
    onClose: dupOnClose,
    onOpenChange: dupOnOpenChange,
  } = useDisclosure();
  const {
    isOpen: playerSelectOpen,
    onOpen: playerSelectOnOpen,
    onClose: playerSelectOnClose,
    onOpenChange: playerSelectOnOpenChange,
  } = useDisclosure();

  // --- Handlers ---
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
        setScreenshotsCount(0);
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

        let isShared = false;
        try {
          const meta: any = await GetVersionMeta(name);
          isShared =
            meta.gameVersion && compareVersions(meta.gameVersion, "1.26.0") > 0;
        } catch {}
        setIsSharedMode(isShared);

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
              if (isShared) {
                if (safe.resourcePacks) {
                  const dir = safe.resourcePacks.replace(
                    /[\\/]resource_packs[\\/]?$/,
                    "",
                  );
                  const sep = safe.resourcePacks.includes("/") ? "/" : "\\";
                  const sp = `${dir}${sep}skin_packs`;
                  setSkinCount(await countDirectories(sp));
                } else {
                  setSkinCount(0);
                }
              } else {
                const sp = `${safe.usersRoot}\\${player}\\games\\com.mojang\\skin_packs`;
                setSkinCount(await countDirectories(sp));
              }
              const srvs = await (minecraft as any)?.ListServers?.(
                name,
                player,
              );
              setServersCount(srvs?.length || 0);
              try {
                const shots = await (contentService as any)?.ListScreenshots?.(
                  name,
                  player,
                );
                setScreenshotsCount(Array.isArray(shots) ? shots.length : 0);
              } catch {
                setScreenshotsCount(0);
              }
            } else {
              setWorldsCount(0);
              setSkinCount(0);
              setServersCount(0);
              setScreenshotsCount(0);
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
                const tag = await GetLocalUserGamertag();
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
          setScreenshotsCount(0);
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

  const onChangePlayer = async (player: string) => {
    setLoading(true);
    setSelectedPlayer(player);
    try {
      if (!hasBackend || !roots.usersRoot || !player) {
        setWorldsCount(0);
        setSkinCount(0);
        setServersCount(0);
        setScreenshotsCount(0);
        return;
      }
      const wp = `${roots.usersRoot}\\${player}\\games\\com.mojang\\minecraftWorlds`;
      setWorldsCount(await countDirectories(wp));
      if (isSharedMode) {
        if (roots.resourcePacks) {
          const dir = roots.resourcePacks.replace(
            /[\\/]resource_packs[\\/]?$/,
            "",
          );
          const sep = roots.resourcePacks.includes("/") ? "/" : "\\";
          const sp = `${dir}${sep}skin_packs`;
          setSkinCount(await countDirectories(sp));
        } else {
          setSkinCount(0);
        }
      } else {
        const sp = `${roots.usersRoot}\\${player}\\games\\com.mojang\\skin_packs`;
        setSkinCount(await countDirectories(sp));
      }
      const srvs = await (minecraft as any)?.ListServers?.(
        currentVersionName || readCurrentVersionName(),
        player,
      );
      setServersCount(srvs?.length || 0);
      try {
        const shots = await (contentService as any)?.ListScreenshots?.(
          currentVersionName || readCurrentVersionName(),
          player,
        );
        setScreenshotsCount(Array.isArray(shots) ? shots.length : 0);
      } catch {
        setScreenshotsCount(0);
      }
    } finally {
      setLoading(false);
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
          const isSkin = await (contentService as any)?.IsMcpackSkinPackPath?.(p);
          if (isSkin) {
            hasSkin = true;
            break;
          }
        }
      }
      let chosenPlayer = "";

      const needsPlayer = hasWorld || (hasSkin && !isSharedMode);

      if (needsPlayer) {
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
          const isSkin = await (contentService as any)?.IsMcpackSkinPackPath?.(p);
          const effectivePlayer = isSkin && isSharedMode ? "" : playerToUse;

          if (
            effectivePlayer &&
            typeof (contentService as any)?.ImportMcpackPathWithPlayer === "function"
          ) {
            err = await (contentService as any)?.ImportMcpackPathWithPlayer?.(
              name,
              effectivePlayer,
              p,
              false,
            );
          } else {
            err = await (contentService as any)?.ImportMcpackPath?.(name, p, false);
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
                  effectivePlayer &&
                  typeof (contentService as any)?.ImportMcpackPathWithPlayer ===
                    "function"
                ) {
                  err = await (contentService as any)?.ImportMcpackPathWithPlayer?.(
                    name,
                    effectivePlayer,
                    p,
                    true,
                  );
                } else {
                  err = await (contentService as any)?.ImportMcpackPath?.(
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
            typeof (contentService as any)?.ImportMcaddonPathWithPlayer ===
              "function"
          ) {
            err = await (contentService as any)?.ImportMcaddonPathWithPlayer?.(
              name,
              playerToUse,
              p,
              false,
            );
          } else {
            err = await (contentService as any)?.ImportMcaddonPath?.(name, p, false);
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
                  typeof (contentService as any)?.ImportMcaddonPathWithPlayer ===
                    "function"
                ) {
                  err = await (contentService as any)?.ImportMcaddonPathWithPlayer?.(
                    name,
                    playerToUse,
                    p,
                    true,
                  );
                } else {
                  err = await (contentService as any)?.ImportMcaddonPath?.(
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
          let err = await (contentService as any)?.ImportMcworldPath?.(
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
                err = await (contentService as any)?.ImportMcworldPath?.(
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
      if (succFiles.length > 0 || errPairs.length > 0) {
        await refreshAll(playerToUse);
        setResultSuccess(succFiles);
        setResultFailed(errPairs);
        errOnOpen();
      }
    } catch (e: any) {
      setErrorMsg(String(e?.message || e || "IMPORT_ERROR"));
    } finally {
      setImporting(false);
      setCurrentFile("");
    }
  };

  // --- Effects ---
  const doImportRef = React.useRef(doImportFromPaths);
  doImportRef.current = doImportFromPaths;

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

  React.useEffect(() => {
    return Events.On("files-dropped", (event) => {
      const data = (event.data as { files: string[] }) || {};
      if (data.files && data.files.length > 0) {
        void doImportRef.current(data.files);
      }
    });
  }, []);

  return {
    // state
    loading,
    error,
    currentVersionName,
    roots,
    players,
    selectedPlayer,
    isSharedMode,
    playerGamertagMap,
    worldsCount,
    resCount,
    bpCount,
    skinCount,
    serversCount,
    screenshotsCount,
    importing,
    errorMsg,
    currentFile,
    resultSuccess,
    resultFailed,
    hasBackend,

    // setters needed by JSX callbacks
    setErrorMsg,
    setResultSuccess,
    setResultFailed,

    // refs (for modal JSX)
    dupResolveRef,
    dupNameRef,
    playerSelectResolveRef,

    // disclosures
    errOpen,
    errOnOpen,
    errOnClose,
    errOnOpenChange,
    dupOpen,
    dupOnOpen,
    dupOnClose,
    dupOnOpenChange,
    playerSelectOpen,
    playerSelectOnOpen,
    playerSelectOnClose,
    playerSelectOnOpenChange,

    // handlers
    refreshAll,
    onChangePlayer,
    doImportFromPaths,

    // navigation
    navigate,
  };
};

