import { EMPTY_CONTENT_ROOTS, isContentTransferTarget } from "@/utils/content";
import { useOverlayState } from "@heroui/react";
import React from "react";

import { Events } from "@wailsio/runtime";
import { useLocation, useNavigate } from "react-router-dom";
import {
  GetVersionMeta,
  ListVersionMetas,
  GetVersionLogoDataUrl,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/versionservice";
import { GetLocalUserGamertag } from "bindings/github.com/liteldev/LeviLauncher/internal/app/userservice";
import { GetContentRoots } from "bindings/github.com/liteldev/LeviLauncher/internal/app/contentservice";
import * as types from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import { useCurrentVersion } from "@/utils/CurrentVersionContext";
import { compareVersions } from "@/utils/version";
import {
  countDirectories,
  getPathBaseName,
  normalizeDroppedFiles,
} from "@/utils/fs";
import { getPlayerGamertagMap, listPlayers } from "@/utils/content";
import { notify } from "@/utils/activity";
import { useContentTask, type ContentTask } from "@/components/ContentTaskDialog";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import * as contentService from "bindings/github.com/liteldev/LeviLauncher/internal/app/contentservice";

type TFunc = (key: string, opts?: Record<string, unknown>) => string;
type TransferTargetVersion = {
  name: string;
  gameVersion: string;
  type: string;
  icon?: string;
};

const isDuplicateError = (err: string) =>
  err === "ERR_DUPLICATE_FOLDER" || err === "ERR_DUPLICATE_UUID";

export const useContentPage = (t: TFunc) => {
  const navigate = useNavigate();
  const location = useLocation() as any;
  const { currentVersionName } = useCurrentVersion();
  const versionRef = React.useRef(currentVersionName);
  versionRef.current = currentVersionName;

  // --- State ---
  const [loading, setLoading] = React.useState<boolean>(true);
  const [error, setError] = React.useState<string>("");
  const [roots, setRoots] = React.useState<types.ContentRoots>(EMPTY_CONTENT_ROOTS);
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
  const [transferTargets, setTransferTargets] = React.useState<
    TransferTargetVersion[]
  >([]);
  const [selectedTransferTargets, setSelectedTransferTargets] = React.useState<
    string[]
  >([]);

  // --- Tasks ---
  // Import and transfer each run in one dialog with a growing per-item log.
  const importTask = useContentTask("import");
  const transferTask = useContentTask("transfer");
  const busyRef = React.useRef(false);
  const refreshGeneration = React.useRef(0);

  // --- Disclosures ---
  const {
    isOpen: transferTargetOpen,
    open: transferTargetOnOpen,
    close: transferTargetOnClose,
    setOpen: transferTargetOnOpenChange,
  } = useOverlayState();

  // --- Handlers ---
  const refreshAll = async (playerToRefresh?: string) => {
    // A newer refresh (e.g. after switching instance) wins over this one.
    const generation = ++refreshGeneration.current;
    const stale = () => generation !== refreshGeneration.current;
    setLoading(true);
    setError("");
    const name = versionRef.current;
    try {
      if (!name) {
        setRoots(EMPTY_CONTENT_ROOTS);
        setPlayers([]);
        setSelectedPlayer("");
        setPlayerGamertagMap({});
        setWorldsCount(0);
        setResCount(0);
        setBpCount(0);
        setSkinCount(0);
        setServersCount(0);
        setScreenshotsCount(0);
      } else {
        const r = await GetContentRoots(name);
        if (stale()) return;
        const safe = r || EMPTY_CONTENT_ROOTS;
        setRoots(safe);

        let isShared = false;
        try {
          const meta: any = await GetVersionMeta(name);
          isShared =
            meta.gameVersion && compareVersions(meta.gameVersion, "1.26.0") > 0;
        } catch {}
        if (stale()) return;
        isShared = isShared || safe.packageType === "uwp";
        setIsSharedMode(isShared);

        if (safe.packageType === "uwp") {
          setPlayers([]);
          setSelectedPlayer("");
          setPlayerGamertagMap({});
          const [worlds, skins, servers, screenshots] = await Promise.all([
            countDirectories(safe.worlds || ""),
            countDirectories(safe.skinPacks || ""),
            minecraft.ListServers(name, ""),
            contentService.ListScreenshots(name, ""),
          ]);
          if (stale()) return;
          setWorldsCount(worlds);
          setSkinCount(skins);
          setServersCount(servers?.length || 0);
          setScreenshotsCount(screenshots?.length || 0);
        } else if (safe.usersRoot) {
          const names = await listPlayers(safe.usersRoot);
          if (stale()) return;
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
            if (stale()) return;
            if (!player) {
              setWorldsCount(0);
              setSkinCount(0);
              setServersCount(0);
              setScreenshotsCount(0);
              return;
            }
            const wp = `${safe.usersRoot}\\${player}\\games\\com.mojang\\minecraftWorlds`;
            let skinsPath = `${safe.usersRoot}\\${player}\\games\\com.mojang\\skin_packs`;
            if (isShared) {
              skinsPath = "";
              if (safe.resourcePacks) {
                const dir = safe.resourcePacks.replace(
                  /[\\/]resource_packs[\\/]?$/,
                  "",
                );
                const sep = safe.resourcePacks.includes("/") ? "/" : "\\";
                skinsPath = `${dir}${sep}skin_packs`;
              }
            }
            const [worlds, skins, srvs, shots] = await Promise.all([
              countDirectories(wp),
              skinsPath ? countDirectories(skinsPath) : Promise.resolve(0),
              (minecraft as any)?.ListServers?.(name, player),
              Promise.resolve(
                (contentService as any)?.ListScreenshots?.(name, player),
              ).catch(() => []),
            ]);
            // Drop results that arrive after a newer refresh started.
            if (stale()) return;
            setWorldsCount(worlds);
            setSkinCount(skins);
            setServersCount(srvs?.length || 0);
            setScreenshotsCount(Array.isArray(shots) ? shots.length : 0);
          };

          await loadCounts(
            names.includes(currentPlayer) ? currentPlayer : nextPlayer,
          );

          (async () => {
            try {
              const map = await getPlayerGamertagMap(safe.usersRoot);
              if (stale()) return;
              setPlayerGamertagMap(map);

              if (
                playerToRefresh === undefined &&
                !names.includes(currentPlayer)
              ) {
                const tag = await GetLocalUserGamertag();
                if (tag && !stale()) {
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
        const [res, bp] = await Promise.all([
          countDirectories(safe.resourcePacks),
          countDirectories(safe.behaviorPacks),
        ]);
        if (stale()) return;
        setResCount(res);
        setBpCount(bp);
      }
    } catch (e) {
      if (!stale()) setError(t("contentpage.error_resolve_paths") as string);
    } finally {
      if (!stale()) setLoading(false);
    }
  };

  const onChangePlayer = async (player: string) => {
    setLoading(true);
    setSelectedPlayer(player);
    try {
      if (!roots.usersRoot || !player) {
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
        currentVersionName,
        player,
      );
      setServersCount(srvs?.length || 0);
      try {
        const shots = await (contentService as any)?.ListScreenshots?.(
          currentVersionName,
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

  const importOne = async (
    name: string,
    path: string,
    player: string,
    overwrite: boolean,
  ): Promise<string> => {
    const lower = path.toLowerCase();
    const svc = contentService as any;
    if (lower.endsWith(".mcworld")) {
      return await svc?.ImportMcworldPath?.(name, player, path, overwrite);
    }
    const isAddon = lower.endsWith(".mcaddon");
    const withPlayer = isAddon
      ? svc?.ImportMcaddonPathWithPlayer
      : svc?.ImportMcpackPathWithPlayer;
    if (player && typeof withPlayer === "function") {
      return await withPlayer(name, player, path, overwrite);
    }
    const plain = isAddon ? svc?.ImportMcaddonPath : svc?.ImportMcpackPath;
    return await plain?.(name, path, overwrite);
  };

  // Runs one item, asking about duplicates inline, and logs its outcome.
  const runItem = async (
    task: ContentTask,
    label: string,
    attempt: (overwrite: boolean) => Promise<string>,
  ) => {
    const id = task.begin(label);
    const run = async (overwrite: boolean) => {
      try {
        return String((await attempt(overwrite)) || "");
      } catch (e: any) {
        return String(e?.message || e || "ERROR");
      }
    };
    let err = await run(false);
    if (isDuplicateError(err)) {
      if (!(await task.askOverwrite(label))) {
        task.end(id, "skipped");
        return;
      }
      err = await run(true);
      if (!err) {
        task.end(id, "success", undefined, true);
        return;
      }
    }
    if (err) task.end(id, "failed", err);
    else task.end(id, "success");
  };

  const notifyNoInstance = () =>
    notify({
      variant: "warning",
      title: t("content_task.no_instance_title"),
      description: t("content_task.no_instance_body"),
      source: "content",
    });

  const doImportFromPaths = async (paths: string[]) => {
    // File-drop events can arrive again while a player/overwrite prompt is open.
    // Keep the active batch and its pending prompt resolver intact.
    if (busyRef.current) return;
    busyRef.current = true;
    try {
      const normalizedPaths = normalizeDroppedFiles(paths).filter((p) =>
        /\.(mcpack|mcaddon|mcworld)$/i.test(p),
      );
      if (!normalizedPaths.length) return;
      const name = currentVersionName;
      if (!name) {
        notifyNoInstance();
        return;
      }
      importTask.open(normalizedPaths.length);
      try {
        const hasWorld = normalizedPaths.some((p) =>
          p.toLowerCase().endsWith(".mcworld"),
        );
        let hasSkin = false;
        for (const p of normalizedPaths) {
          if (p.toLowerCase().endsWith(".mcpack")) {
            const isSkin = await (
              contentService as any
            )?.IsMcpackSkinPackPath?.(p);
            if (isSkin) {
              hasSkin = true;
              break;
            }
          }
        }
        let chosenPlayer = "";
        const needsPlayer =
          roots.packageType !== "uwp" &&
          (hasWorld || (hasSkin && !isSharedMode));
        if (needsPlayer) {
          chosenPlayer = await importTask.askPlayer();
          if (!chosenPlayer) return;
          setSelectedPlayer(chosenPlayer);
        }
        const playerToUse = chosenPlayer || selectedPlayer || "";
        let started = false;
        for (const p of normalizedPaths) {
          const lower = p.toLowerCase();
          const base = getPathBaseName(p);
          if (
            lower.endsWith(".mcworld") &&
            !playerToUse &&
            roots.packageType !== "uwp"
          ) {
            importTask.end(importTask.begin(base), "failed", "ERR_NO_PLAYER");
            continue;
          }
          let effectivePlayer = playerToUse;
          if (lower.endsWith(".mcpack") && isSharedMode) {
            const isSkin = await (
              contentService as any
            )?.IsMcpackSkinPackPath?.(p);
            if (isSkin) effectivePlayer = "";
          }
          started = true;
          await runItem(importTask, base, (overwrite) =>
            importOne(name, p, effectivePlayer, overwrite),
          );
        }
        // Refresh once after importing, including a player change where every
        // duplicate was skipped, so the displayed counts match the selected player.
        if (started || chosenPlayer) {
          await refreshAll(playerToUse);
        }
        importTask.finish();
      } catch (e) {
        importTask.fail(e);
      }
    } finally {
      busyRef.current = false;
    }
  };

  const openResourceTransferModal = async () => {
    if (busyRef.current) return;
    const name = currentVersionName;
    if (!name) {
      notifyNoInstance();
      return;
    }
    try {
      const list = await (ListVersionMetas as any)?.();
      const metas = Array.isArray(list) ? list : [];
      const targets: TransferTargetVersion[] = metas
        .filter(
          (m: any) =>
            m &&
            typeof m.name === "string" &&
            m.name &&
            isContentTransferTarget(roots, m) &&
            m.name !== name,
        )
        .sort((a: any, b: any) => {
          const byVersion = compareVersions(
            String(b.gameVersion || "0"),
            String(a.gameVersion || "0"),
          );
          if (byVersion !== 0) return byVersion;
          return String(a.name || "").localeCompare(String(b.name || ""));
        })
        .map((m: any) => ({
          name: String(m.name || ""),
          gameVersion: String(m.gameVersion || ""),
          type: String(m.type || ""),
        }));

      // Fetch icons for targets
      await Promise.all(
        targets.map(async (t) => {
          try {
            const getter = GetVersionLogoDataUrl as any;
            if (typeof getter === "function") {
              const url = await getter(t.name);
              if (url) {
                t.icon = url;
              }
            }
          } catch (e) {
            console.warn("Failed to fetch logo for", t.name, e);
          }
        }),
      );

      setTransferTargets(targets);
      setSelectedTransferTargets(targets.length > 0 ? [targets[0].name] : []);
      transferTargetOnOpen();
    } catch (e: any) {
      notify({
        variant: "danger",
        title: t("content_task.load_targets_failed"),
        description: String(e?.message || e || ""),
        source: "content",
      });
    }
  };

  const transferResourcesToTargets = async () => {
    const sourceVersionName = currentVersionName;
    if (!sourceVersionName) {
      notifyNoInstance();
      return;
    }
    const targetNames = selectedTransferTargets.filter(Boolean);
    if (targetNames.length === 0 || busyRef.current) return;
    busyRef.current = true;

    transferTargetOnClose();
    transferTask.open();

    try {
      const transferFn = (contentService as any)?.TransferPackToVersion;
      if (typeof transferFn !== "function") {
        const id = transferTask.begin(t("contentpage.resource_packs"));
        transferTask.end(id, "failed", t("common.feature_unavailable"));
        transferTask.finish();
        return;
      }

      const allPacks = await (contentService as any)?.ListPacksForVersion?.(
        sourceVersionName,
        "",
      );
      const packs = (Array.isArray(allPacks) ? allPacks : []).filter(
        (p: any) => {
          const packType = Number(p?.manifest?.pack_type || 0);
          const path = String(p?.path || "").trim();
          return path && (packType === 6 || packType === 4);
        },
      );

      if (packs.length === 0) {
        const id = transferTask.begin(sourceVersionName);
        transferTask.end(id, "failed", t("contentpage.no_resource_packs"));
        transferTask.finish();
        return;
      }

      transferTask.setTotal(targetNames.length * packs.length);
      for (const targetName of targetNames) {
        for (const pack of packs) {
          const packPath = String(pack?.path || "").trim();
          if (!packPath) continue;
          const packName = String(
            pack?.manifest?.name || getPathBaseName(packPath),
          );
          await runItem(
            transferTask,
            `${packName} -> ${targetName}`,
            (overwrite) =>
              transferFn(sourceVersionName, packPath, targetName, overwrite),
          );
        }
      }
      transferTask.finish();
    } catch (e) {
      transferTask.fail(e);
    } finally {
      busyRef.current = false;
    }
  };

  // --- Effects ---
  const doImportRef = React.useRef(doImportFromPaths);
  doImportRef.current = doImportFromPaths;

  // The player handed over on entry applies to the instance shown on entry;
  // switching the instance in place (header picker) reloads with its players.
  const initialRef = React.useRef<{ version: string; player: string } | null>(
    null,
  );
  if (!initialRef.current) {
    initialRef.current = {
      version: currentVersionName,
      player:
        location?.state?.player ||
        localStorage.getItem("content.selectedPlayer") ||
        "",
    };
  }
  React.useEffect(() => {
    const initial = initialRef.current!;
    if (currentVersionName !== initial.version) initial.player = "";
    if (initial.player) {
      refreshAll(initial.player);
    } else {
      refreshAll();
    }
    if (location?.state?.player) {
      navigate(location.pathname, {
        replace: true,
        state: { ...(location.state || {}), player: undefined },
      });
    }
  }, [currentVersionName]);

  React.useEffect(() => {
    if (selectedPlayer) {
      localStorage.setItem("content.selectedPlayer", selectedPlayer);
    }
  }, [selectedPlayer]);

  React.useEffect(() => {
    return Events.On("files-dropped", (event) => {
      const data = (event.data as { files: string[] }) || {};
      const files = normalizeDroppedFiles(data.files);
      if (files.length > 0) {
        void doImportRef.current(files);
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
    importing: importTask.isBusy || transferTask.isBusy,
    transferTargets,
    selectedTransferTargets,

    // setters needed by JSX callbacks
    setSelectedTransferTargets,

    // tasks (rendered by ContentTaskDialog)
    importTask,
    transferTask,

    // disclosures
    transferTargetOpen,
    transferTargetOnOpen,
    transferTargetOnClose,
    transferTargetOnOpenChange,

    // handlers
    refreshAll,
    onChangePlayer,
    doImportFromPaths,
    openResourceTransferModal,
    transferResourcesToTargets,

    // navigation
    navigate,
  };
};
