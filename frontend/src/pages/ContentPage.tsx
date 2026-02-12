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
  Progress,
  useDisclosure,
} from "@heroui/react";
import { Dialogs, Events } from "@wailsio/runtime";
import { UnifiedModal } from "@/components/UnifiedModal";
import { ImportResultModal } from "@/components/ImportResultModal";
import { AnimatePresence, motion } from "framer-motion";
import { useLocation, useNavigate } from "react-router-dom";
import {
  GetContentRoots,
  GetVersionMeta,
} from "bindings/github.com/liteldev/LeviLauncher/minecraft";
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
import { compareVersions } from "@/utils/version";
import { countDirectories } from "@/utils/fs";
import {
  getPlayerGamertagMap,
  listPlayers,
  resolvePlayerDisplayName,
} from "@/utils/content";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { FiUploadCloud } from "react-icons/fi";
import { PageHeader } from "@/components/PageHeader";
import { PageContainer } from "@/components/PageContainer";
import { FileDropOverlay } from "@/components/FileDropOverlay";
import { useFileDrag } from "@/hooks/useFileDrag";
import { LAYOUT } from "@/constants/layout";
import { cn } from "@/utils/cn";

export default function ContentPage() {
  const { t } = useTranslation();
  const scrollRef = React.useRef<HTMLDivElement>(null);
  const isDragActive = useFileDrag(scrollRef as React.RefObject<HTMLElement>);
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
  const [isSharedMode, setIsSharedMode] = React.useState<boolean>(false);
  const [playerGamertagMap, setPlayerGamertagMap] = React.useState<
    Record<string, string>
  >({});
  const [worldsCount, setWorldsCount] = React.useState<number>(0);
  const [resCount, setResCount] = React.useState<number>(0);
  const [bpCount, setBpCount] = React.useState<number>(0);
  const [skinCount, setSkinCount] = React.useState<number>(0);
  const [serversCount, setServersCount] = React.useState<number>(0);
  const [importing, setImporting] = React.useState(false);
  const [errorMsg, setErrorMsg] = React.useState("");
  const [currentFile, setCurrentFile] = React.useState("");
  const [resultSuccess, setResultSuccess] = React.useState<string[]>([]);
  const [resultFailed, setResultFailed] = React.useState<
    Array<{ name: string; err: string }>
  >([]);
  const dupResolveRef = React.useRef<((overwrite: boolean) => void) | null>(
    null,
  );
  const dupNameRef = React.useRef<string>("");
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
  const playerSelectResolveRef = React.useRef<
    ((player: string) => void) | null
  >(null);
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
          const isSkin = await (minecraft as any)?.IsMcpackSkinPackPath?.(p);
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
          const isSkin = await (minecraft as any)?.IsMcpackSkinPackPath?.(p);
          const effectivePlayer = isSkin && isSharedMode ? "" : playerToUse;

          if (
            effectivePlayer &&
            typeof (minecraft as any)?.ImportMcpackPathWithPlayer === "function"
          ) {
            err = await (minecraft as any)?.ImportMcpackPathWithPlayer?.(
              name,
              effectivePlayer,
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
                  effectivePlayer &&
                  typeof (minecraft as any)?.ImportMcpackPathWithPlayer ===
                    "function"
                ) {
                  err = await (minecraft as any)?.ImportMcpackPathWithPlayer?.(
                    name,
                    effectivePlayer,
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

  const doImportRef = React.useRef(doImportFromPaths);
  doImportRef.current = doImportFromPaths;

  React.useEffect(() => {
    return Events.On("files-dropped", (event) => {
      const data = (event.data as { files: string[] }) || {};
      // Ignore target check to ensure stability, as we are inside the component
      if (data.files && data.files.length > 0) {
        void doImportRef.current(data.files);
      }
    });
  }, []);

  return (
    <PageContainer
      ref={scrollRef}
      id="content-drop-zone"
      {...({ "data-file-drop-target": true } as any)}
      className="relative"
      animate={false}
    >
      {/* Drag Overlay */}
      <FileDropOverlay
        isDragActive={isDragActive}
        text={t("contentpage.drop_hint")}
      />

      {/* Header Card */}
      <motion.div
        initial={{ opacity: 0, y: -20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4 }}
      >
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
                  <div className="mt-2 text-default-500 dark:text-zinc-400 text-sm flex flex-wrap items-center gap-2">
                    <span>{t("contentpage.current_version")}:</span>
                    <span className="font-medium text-default-700 dark:text-zinc-200 bg-default-100 dark:bg-zinc-800 px-2 py-0.5 rounded-md">
                      {currentVersionName || t("contentpage.none")}
                    </span>
                    <span className="text-default-300 dark:text-zinc-600">
                      |
                    </span>
                    <span>{t("contentpage.isolation")}:</span>
                    <span
                      className={`font-medium px-2 py-0.5 rounded-md ${
                        roots.isIsolation
                          ? "bg-success-50 text-success-600 dark:bg-success-900/20 dark:text-success-400"
                          : "bg-default-100 dark:bg-zinc-800 text-default-700 dark:text-zinc-200"
                      }`}
                    >
                      {roots.isIsolation ? t("common.yes") : t("common.no")}
                    </span>
                    <span className="text-default-300 dark:text-zinc-600">
                      |
                    </span>
                    <span>{t("contentpage.select_player")}:</span>
                    <Dropdown>
                      <DropdownTrigger>
                        <Button
                          size="sm"
                          variant="light"
                          className="h-6 min-w-0 px-2 text-small font-medium text-default-700 dark:text-zinc-200 bg-default-100 dark:bg-zinc-800 rounded-md"
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
                          const arr = Array.from(
                            keys as unknown as Set<string>,
                          );
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
                    className="bg-primary-600 text-white font-medium shadow-sm"
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
                    content={
                      t("contentpage.open_users_dir") as unknown as string
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
              {!!error && (
                <div className="text-danger-500 text-sm">{error}</div>
              )}
            </div>
          </CardBody>
        </Card>
      </motion.div>

      {/* Content Grid */}
      <motion.div
        className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6"
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4, delay: 0.1 }}
      >
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
                <span className="text-lg font-medium text-default-700 dark:text-zinc-200">
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
                    className="text-2xl font-bold text-default-900 dark:text-zinc-100"
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
                <span className="text-lg font-medium text-default-700 dark:text-zinc-200">
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
                    className="text-2xl font-bold text-default-900 dark:text-zinc-100"
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
                <span className="text-lg font-medium text-default-700 dark:text-zinc-200">
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
                    className="text-2xl font-bold text-default-900 dark:text-zinc-100"
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
                <span className="text-lg font-medium text-default-700 dark:text-zinc-200">
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
                    className="text-2xl font-bold text-default-900 dark:text-zinc-100"
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
                <span className="text-lg font-medium text-default-700 dark:text-zinc-200">
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
                    className="text-2xl font-bold text-default-900 dark:text-zinc-100"
                  >
                    {serversCount}
                  </motion.span>
                )}
              </AnimatePresence>
            </div>
          </CardBody>
        </Card>
      </motion.div>

      <UnifiedModal
        isOpen={importing}
        onOpenChange={() => {}}
        type="primary"
        title={t("mods.importing_title")}
        icon={<FiUploadCloud className="w-6 h-6 text-primary-500" />}
        hideCloseButton
        isDismissable={false}
        showConfirmButton={false}
        showCancelButton={false}
      >
        <div className="flex flex-col gap-4">
          <Progress
            isIndeterminate
            aria-label="importing"
            className="w-full"
            size="sm"
            color="primary"
          />
          <div className="text-default-600 dark:text-zinc-300 text-sm">
            {t("mods.importing_body")}
          </div>
          {currentFile ? (
            <div className="p-3 bg-default-100/50 dark:bg-zinc-800 rounded-xl border border-default-200/50 text-small font-mono text-default-800 dark:text-zinc-200 break-all">
              {currentFile}
            </div>
          ) : null}
        </div>
      </UnifiedModal>
      <ImportResultModal
        isOpen={errOpen}
        onOpenChange={errOnOpenChange}
        results={{ success: resultSuccess, failed: resultFailed }}
        onConfirm={() => {
          setErrorMsg("");
          setResultSuccess([]);
          setResultFailed([]);
        }}
      />
      <UnifiedModal
        isOpen={dupOpen}
        onOpenChange={(open) => {
          if (!open) {
            dupOnClose();
            dupResolveRef.current?.(false);
          }
        }}
        type="warning"
        title={t("mods.overwrite_modal_title")}
        confirmText={t("common.confirm")}
        cancelText={t("common.cancel")}
        showCancelButton
        onConfirm={() => {
          dupResolveRef.current?.(true);
          dupOnClose();
        }}
        onCancel={() => {
          dupResolveRef.current?.(false);
          dupOnClose();
        }}
      >
        <div className="flex flex-col gap-4">
          <div className="text-sm text-default-700 dark:text-zinc-300">
            {t("mods.overwrite_modal_body")}
          </div>
          {dupNameRef.current ? (
            <div className="p-3 bg-default-100/50 dark:bg-zinc-800 rounded-xl border border-default-200/50 text-small font-mono text-default-800 dark:text-zinc-200 break-all">
              {dupNameRef.current}
            </div>
          ) : null}
        </div>
      </UnifiedModal>
      <UnifiedModal
        isOpen={playerSelectOpen}
        onOpenChange={(open) => {
          if (!open) {
            playerSelectOnClose();
            playerSelectResolveRef.current?.("");
          }
        }}
        type="primary"
        title={t("contentpage.select_player_title")}
        cancelText={t("common.cancel")}
        showCancelButton
        showConfirmButton={false}
        onCancel={() => {
          playerSelectResolveRef.current?.("");
          playerSelectOnClose();
        }}
      >
        <div className="flex flex-col gap-4">
          <div className="text-sm text-default-700 dark:text-zinc-300">
            {t("contentpage.select_player_for_import")}
          </div>
          <div className="flex flex-col gap-2 max-h-[300px] overflow-y-auto custom-scrollbar p-1">
            {players.length ? (
              players.map((p) => (
                <Button
                  key={p}
                  variant="flat"
                  className="w-full justify-start bg-default-100 dark:bg-zinc-800 text-default-700 dark:text-zinc-200"
                  onPress={() => {
                    playerSelectResolveRef.current?.(p);
                    playerSelectOnClose();
                  }}
                >
                  {resolvePlayerDisplayName(p, playerGamertagMap)}
                </Button>
              ))
            ) : (
              <div className="text-sm text-default-500 dark:text-zinc-400">
                {t("contentpage.no_players")}
              </div>
            )}
          </div>
        </div>
      </UnifiedModal>
    </PageContainer>
  );
}
