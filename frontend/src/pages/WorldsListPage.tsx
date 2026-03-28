import React, { useState, useEffect, useCallback } from "react";
import { PageHeader } from "@/components/PageHeader";
import {
  Button,
  Input,
  Card,
  CardBody,
  Dropdown,
  DropdownTrigger,
  DropdownMenu,
  DropdownItem,
  Checkbox,
  Image,
  Spinner,
  Tooltip,
  useDisclosure,
  Pagination,
  addToast,
  Select,
  SelectItem,
  Progress,
} from "@heroui/react";
import {
  FaSortAmountDown,
  FaSortAmountUp,
  FaTrash,
  FaFolderOpen,
  FaSync,
  FaBox,
  FaArchive,
  FaFilter,
  FaUser,
  FaClock,
  FaEdit,
  FaCheckSquare,
  FaTimes,
  FaHdd,
  FaExchangeAlt,
} from "react-icons/fa";
import { useLocation, useNavigate } from "react-router-dom";
import {
  ListDir,
  GetWorldLevelName,
  GetWorldIconDataUrl,
  GetPathSize,
  GetPathModTime,
  BackupWorld,
  BackupWorldWithVersion,
  OpenPathDir,
} from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import {
  GetContentRoots,
  DeleteWorld,
  TransferWorldToVersion,
} from "bindings/github.com/liteldev/LeviLauncher/contentservice";
import {
  GetVersionLogoDataUrl,
  ListVersionMetas,
} from "bindings/github.com/liteldev/LeviLauncher/versionservice";
import { GetLocalUserGamertag } from "bindings/github.com/liteldev/LeviLauncher/userservice";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { readCurrentVersionName } from "@/utils/currentVersion";
import { DeleteConfirmModal } from "@/components/DeleteConfirmModal";
import { UnifiedModal } from "@/components/UnifiedModal";
import { ImportResultModal } from "@/components/ImportResultModal";
import { useTranslation } from "react-i18next";
import { motion, AnimatePresence } from "framer-motion";
import DefaultWorldPreview from "@/assets/images/world-preview-default.jpg";
import {
  getPlayerGamertagMap,
  listPlayers,
  resolvePlayerDisplayName,
} from "@/utils/content";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { cn } from "@/utils/cn";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { useScrollManager } from "@/hooks/useScrollManager";
import { useSelectionMode } from "@/hooks/useSelectionMode";
import { useContentSort } from "@/hooks/useContentSort";
import { formatBytes } from "@/utils/formatting";
import { compareVersions } from "@/utils/version";
import { getPathBaseName } from "@/utils/fs";

interface WorldInfo {
  Path: string;
  FolderName: string;
  IconBase64: string;
  Size: number;
  LastModified: number;
}

type TransferTargetVersion = {
  name: string;
  gameVersion: string;
  type: string;
  icon?: string;
};

import { SelectionBar } from "@/components/SelectionBar";

export default function WorldsListPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const currentVersionName =
    location.state?.versionName || readCurrentVersionName();

  const [selectedPlayer, setSelectedPlayer] = useState<string>(
    location.state?.player || "",
  );
  const [players, setPlayers] = useState<string[]>([]);
  const [playerGamertagMap, setPlayerGamertagMap] = useState<
    Record<string, string>
  >({});
  const [worlds, setWorlds] = useState<WorldInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [roots, setRoots] = useState<any>({});

  const [deletingOne, setDeletingOne] = useState<boolean>(false);
  const [deletingMany, setDeletingMany] = useState<boolean>(false);

  const [backingUp, setBackingUp] = useState("");
  const [activeWorld, setActiveWorld] = useState<WorldInfo | null>(null);

  const {
    isOpen: delOpen,
    onOpen: delOnOpen,
    onClose: delOnClose,
    onOpenChange: delOnOpenChange,
  } = useDisclosure();

  const {
    isOpen: delManyCfmOpen,
    onOpen: delManyCfmOnOpen,
    onClose: delManyCfmOnClose,
    onOpenChange: delManyCfmOnOpenChange,
  } = useDisclosure();
  const {
    isOpen: transferTargetOpen,
    onOpen: transferTargetOnOpen,
    onClose: transferTargetOnClose,
    onOpenChange: transferTargetOnOpenChange,
  } = useDisclosure();
  const {
    isOpen: transferResultOpen,
    onOpen: transferResultOnOpen,
    onOpenChange: transferResultOnOpenChange,
  } = useDisclosure();

  const [currentWorldsPath, setCurrentWorldsPath] = useState("");
  const [transferring, setTransferring] = useState<boolean>(false);
  const [currentTransferItem, setCurrentTransferItem] = useState<string>("");
  const [transferTargets, setTransferTargets] = useState<
    TransferTargetVersion[]
  >([]);
  const [selectedTransferTargets, setSelectedTransferTargets] = useState<
    string[]
  >([]);
  const [transferResultSuccess, setTransferResultSuccess] = useState<string[]>(
    [],
  );
  const [transferResultFailed, setTransferResultFailed] = useState<
    Array<{ name: string; err: string }>
  >([]);

  const scrollRef = React.useRef<HTMLDivElement | null>(null);
  const sort = useContentSort(
    "content.worlds.sort",
    worlds,
    (w: WorldInfo) => w.FolderName,
    (w: WorldInfo) => Number(w.LastModified || 0),
  );
  useScrollManager(scrollRef, [worlds], [sort.currentPage]);
  const selection = useSelectionMode(sort.filtered, (w: WorldInfo) => w.Path);

  useEffect(() => {
    const fetchPlayers = async () => {
      try {
        const r = await GetContentRoots(currentVersionName || "");
        setRoots(r);
        if (r.usersRoot) {
          const pList = await listPlayers(r.usersRoot);
          setPlayers(pList);

          let defaultP = "";
          if (!selectedPlayer && pList.length > 0) {
            defaultP = pList[0];
            setSelectedPlayer(defaultP);
          }

          (async () => {
            try {
              const map = await getPlayerGamertagMap(r.usersRoot);
              setPlayerGamertagMap(map);

              const tag = await GetLocalUserGamertag();
              if (tag) {
                for (const p of pList) {
                  if (map[p] === tag) {
                    if (
                      p !== defaultP &&
                      (!selectedPlayer || selectedPlayer === defaultP)
                    ) {
                      setSelectedPlayer(p);
                    }
                    break;
                  }
                }
              }
            } catch {}
          })();
        } else {
          setPlayers([]);
          setPlayerGamertagMap({});
        }
      } catch (e) {
        console.error("Failed to list players", e);
        setPlayers([]);
        setPlayerGamertagMap({});
      }
    };
    fetchPlayers();
  }, [currentVersionName]);

  const refreshAll = useCallback(() => {
    setLoading(true);

    const fetchWorlds = async () => {
      try {
        const r = await GetContentRoots(currentVersionName || "");
        setRoots(r);
        let worldsPath = "";
        if (r.usersRoot && selectedPlayer) {
          worldsPath = `${r.usersRoot}\\${selectedPlayer}\\games\\com.mojang\\minecraftWorlds`;
        } else {
          if (!selectedPlayer) {
            setWorlds([]);
            return;
          }
        }

        setCurrentWorldsPath(worldsPath);

        const entries = await ListDir(worldsPath);
        if (!entries) {
          setWorlds([]);
          return;
        }

        const list: WorldInfo[] = [];
        await Promise.all(
          entries.map(async (e) => {
            if (!e.isDir) return;
            try {
              const name = await GetWorldLevelName(e.path);
              const icon = await GetWorldIconDataUrl(e.path);
              const size = await GetPathSize(e.path);
              const time = await GetPathModTime(e.path);

              list.push({
                Path: e.path,
                FolderName: name || e.name,
                IconBase64: icon,
                Size: size,
                LastModified: time,
              });
            } catch (err) {
              console.error("Error reading world info", e.path, err);
            }
          }),
        );

        setWorlds(list);
      } catch (err: any) {
        console.error(err);
        addToast({ description: String(err), color: "danger" });
      } finally {
        setLoading(false);
      }
    };

    fetchWorlds();
  }, [selectedPlayer, currentVersionName]);

  useEffect(() => {
    setWorlds([]);
    refreshAll();
    if (selectedPlayer) {
      localStorage.setItem("content.selectedPlayer", selectedPlayer);
    }
  }, [selectedPlayer, refreshAll]);

  const handleDelete = async () => {
    if (!activeWorld) return;
    setDeletingOne(true);
    try {
      await DeleteWorld(currentVersionName || "", activeWorld.Path);
      addToast({ title: t("common.success"), color: "success" });
      refreshAll();
      delOnClose();
    } catch (e) {
      addToast({ description: String(e), color: "danger" });
    } finally {
      setDeletingOne(false);
    }
  };

  const handleBatchDelete = async () => {
    const paths = selection.getSelectedKeys();
    if (paths.length === 0) return;

    setDeletingMany(true);
    try {
      let successCount = 0;
      for (const p of paths) {
        try {
          await DeleteWorld(currentVersionName || "", p);
          successCount++;
        } catch (e) {
          console.error(e);
        }
      }
      addToast({
        title: t("contentpage.deleted_count", { count: successCount }),
        color: "success",
      });
      selection.clearSelection();
      refreshAll();
      delManyCfmOnClose();
    } finally {
      setDeletingMany(false);
    }
  };

  const handleBackup = async (w: WorldInfo) => {
    setBackingUp(w.Path);
    try {
      let dest = "";
      try {
        dest = await BackupWorldWithVersion(w.Path, currentVersionName || "");
      } catch {
        dest = await BackupWorld(w.Path);
      }

      if (dest) {
        addToast({
          title: t("contentpage.backup_success"),
          color: "success",
        });
      } else {
        addToast({
          description: t("contentpage.backup_failed"),
          color: "danger",
        });
      }
    } catch (e) {
      addToast({
        description: t("contentpage.backup_failed") + ": " + String(e),
        color: "danger",
      });
    } finally {
      setBackingUp("");
    }
  };

  const openTransferTargetModal = useCallback(async () => {
    if (transferring || selection.selectedCount === 0) return;

    const sourceVersionName = currentVersionName || readCurrentVersionName();
    if (!sourceVersionName) {
      addToast({
        title: t("launcherpage.currentVersion_none") as string,
        color: "danger",
      });
      return;
    }
    if (!selectedPlayer) {
      addToast({
        title: t("contentpage.require_player_for_world_import") as string,
        color: "danger",
      });
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
            m.enableIsolation &&
            m.name !== sourceVersionName,
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

      await Promise.all(
        targets.map(async (target) => {
          try {
            const icon = await (GetVersionLogoDataUrl as any)?.(target.name);
            if (icon) target.icon = icon;
          } catch {}
        }),
      );

      setTransferTargets(targets);
      setSelectedTransferTargets(targets.length > 0 ? [targets[0].name] : []);
      transferTargetOnOpen();
    } catch (e) {
      addToast({
        title: "Error",
        description: String(e),
        color: "danger",
      });
    }
  }, [
    transferring,
    selection.selectedCount,
    currentVersionName,
    selectedPlayer,
    t,
    transferTargetOnOpen,
  ]);

  const transferSelectedWorldsToTargets = useCallback(async () => {
    if (transferring) return;

    const sourceVersionName = currentVersionName || readCurrentVersionName();
    if (!sourceVersionName || !selectedPlayer) {
      addToast({
        title: t("contentpage.require_player_for_world_import") as string,
        color: "danger",
      });
      return;
    }

    const selectedWorldPaths = selection.getSelectedKeys().filter(Boolean);
    const targetNames = selectedTransferTargets.filter(Boolean);
    if (selectedWorldPaths.length === 0 || targetNames.length === 0) return;

    transferTargetOnClose();

    const worldNameMap = new Map<string, string>(
      worlds.map((w) => [
        w.Path,
        w.FolderName || getPathBaseName(w.Path) || w.Path,
      ]),
    );

    const succFiles: string[] = [];
    const errPairs: Array<{ name: string; err: string }> = [];

    try {
      setTransferring(true);
      setCurrentTransferItem("");

      for (const targetName of targetNames) {
        for (const worldPath of selectedWorldPaths) {
          const worldName =
            worldNameMap.get(worldPath) ||
            getPathBaseName(worldPath) ||
            worldPath;
          const itemLabel = `${worldName} -> ${targetName}`;
          setCurrentTransferItem(itemLabel);

          const err = await TransferWorldToVersion(
            sourceVersionName,
            selectedPlayer,
            worldPath,
            targetName,
            selectedPlayer,
          );
          if (err) {
            errPairs.push({ name: itemLabel, err: String(err) });
            continue;
          }
          succFiles.push(itemLabel);
        }
      }

      if (succFiles.length > 0 || errPairs.length > 0) {
        setTransferResultSuccess(succFiles);
        setTransferResultFailed(errPairs);
        transferResultOnOpen();
      }
      if (succFiles.length > 0) {
        selection.clearSelection();
      }
    } catch (e) {
      addToast({
        title: "Error",
        description: String(e),
        color: "danger",
      });
    } finally {
      setTransferring(false);
      setCurrentTransferItem("");
    }
  }, [
    transferring,
    currentVersionName,
    selectedPlayer,
    t,
    selection,
    selectedTransferTargets,
    transferTargetOnClose,
    worlds,
    transferResultOnOpen,
  ]);

  return (
    <PageContainer ref={scrollRef}>
      <Card className={LAYOUT.GLASS_CARD.BASE}>
        <CardBody className="p-6 flex flex-col gap-6">
          <PageHeader
            title={t("contentpage.worlds_list")}
            endContent={
              <div className="flex items-center gap-2">
                <Dropdown classNames={COMPONENT_STYLES.dropdown}>
                  <DropdownTrigger>
                    <Button
                      radius="full"
                      variant="flat"
                      className="w-full sm:w-auto sm:min-w-[200px] bg-default-100 dark:bg-zinc-800 text-default-600 dark:text-zinc-200 font-medium"
                      isDisabled={!players.length}
                      startContent={<FaUser />}
                    >
                      {selectedPlayer
                        ? resolvePlayerDisplayName(
                            selectedPlayer,
                            playerGamertagMap,
                          )
                        : t("contentpage.select_player")}
                    </Button>
                  </DropdownTrigger>
                  <DropdownMenu
                    aria-label={
                      t("contentpage.players_aria") as unknown as string
                    }
                    selectionMode="single"
                    selectedKeys={
                      selectedPlayer ? new Set([selectedPlayer]) : new Set()
                    }
                    onSelectionChange={(keys) => {
                      const arr = Array.from(keys as unknown as Set<string>);
                      const next = arr[0] || "";
                      if (typeof next === "string" && next)
                        setSelectedPlayer(next);
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
                <Button
                  radius="full"
                  variant="flat"
                  startContent={<FaFolderOpen />}
                  onPress={() => {
                    if (currentWorldsPath) OpenPathDir(currentWorldsPath);
                  }}
                  isDisabled={!currentWorldsPath}
                  className="bg-default-100 dark:bg-zinc-800 text-default-600 dark:text-zinc-200 font-medium"
                >
                  {t("common.open")}
                </Button>
                <Tooltip content={t("common.select_mode")}>
                  <Button
                    isIconOnly
                    radius="full"
                    variant="flat"
                    className="bg-default-100 dark:bg-zinc-800 text-default-600 dark:text-zinc-200"
                    onPress={selection.toggleSelectMode}
                  >
                    <FaCheckSquare />
                  </Button>
                </Tooltip>
                <Tooltip content={t("common.refresh") as unknown as string}>
                  <Button
                    isIconOnly
                    radius="full"
                    variant="flat"
                    className="bg-default-100 dark:bg-zinc-800 text-default-600 dark:text-zinc-200"
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
            }
          />

          <div className="flex flex-col md:flex-row gap-4 items-end md:items-center justify-between">
            <Input
              placeholder={t("common.search_placeholder") as string}
              value={sort.query}
              onValueChange={sort.setQuery}
              startContent={<FaFilter className="text-default-400" />}
              endContent={
                sort.query && (
                  <button onClick={() => sort.setQuery("")}>
                    <FaTimes className="text-default-400 hover:text-default-600" />
                  </button>
                )
              }
              radius="full"
              variant="flat"
              className="w-full md:max-w-xs"
              classNames={COMPONENT_STYLES.input}
            />

            <div className="flex items-center gap-3">
              <Dropdown classNames={COMPONENT_STYLES.dropdown}>
                <DropdownTrigger>
                  <Button
                    variant="flat"
                    radius="full"
                    className="min-w-[120px] bg-default-100 dark:bg-zinc-800 text-default-600 dark:text-zinc-200 font-medium"
                    startContent={
                      sort.sortAsc ? <FaSortAmountDown /> : <FaSortAmountUp />
                    }
                  >
                    {sort.sortKey === "name"
                      ? (t("filemanager.sort.name") as string)
                      : (t("contentpage.sort_time") as string)}
                    {" / "}
                    {sort.sortAsc
                      ? t("contentpage.sort_asc")
                      : t("contentpage.sort_desc")}
                  </Button>
                </DropdownTrigger>
                <DropdownMenu
                  selectionMode="single"
                  selectedKeys={
                    new Set([
                      `${sort.sortKey}-${sort.sortAsc ? "asc" : "desc"}`,
                    ])
                  }
                  onSelectionChange={(keys) => {
                    const val = Array.from(keys)[0] as string;
                    const [k, order] = val.split("-");
                    const nextKey = (k as "name" | "time") || "name";
                    const nextAsc = order === "asc";
                    sort.setSort(nextKey, nextAsc);
                  }}
                >
                  <DropdownItem
                    key="name-asc"
                    startContent={<FaSortAmountDown />}
                  >
                    {t("filemanager.sort.name")} (A-Z)
                  </DropdownItem>
                  <DropdownItem
                    key="name-desc"
                    startContent={<FaSortAmountUp />}
                  >
                    {t("filemanager.sort.name")} (Z-A)
                  </DropdownItem>
                  <DropdownItem
                    key="time-asc"
                    startContent={<FaSortAmountDown />}
                  >
                    {t("contentpage.sort_time")} (Old-New)
                  </DropdownItem>
                  <DropdownItem
                    key="time-desc"
                    startContent={<FaSortAmountUp />}
                  >
                    {t("contentpage.sort_time")} (New-Old)
                  </DropdownItem>
                </DropdownMenu>
              </Dropdown>
            </div>
          </div>

          <div className="mt-2 text-default-500 dark:text-zinc-400 text-sm flex flex-wrap items-center gap-2">
            <span>{t("contentpage.current_version")}:</span>
            <span className="font-medium text-default-700 dark:text-zinc-200 bg-default-100 dark:bg-zinc-800 px-2 py-0.5 rounded-md">
              {currentVersionName || t("contentpage.none")}
            </span>
            <span className="text-default-300 dark:text-zinc-600">|</span>
            <span>{t("contentpage.isolation")}:</span>
            <span className="font-medium text-default-700 dark:text-zinc-200 bg-default-100 dark:bg-zinc-800 px-2 py-0.5 rounded-md">
              {roots.isIsolation ? t("common.yes") : t("common.no")}
            </span>
          </div>
        </CardBody>
      </Card>

      <SelectionBar
        selectedCount={selection.selectedCount}
        totalCount={sort.filtered.length}
        onSelectAll={selection.selectAll}
        onDelete={delManyCfmOnOpen}
        isSelectMode={selection.isSelectMode}
        onTransfer={openTransferTargetModal}
        isTransferDisabled={
          !selectedPlayer || selection.selectedCount === 0 || transferring
        }
      />

      {loading && worlds.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 gap-4">
          <Spinner size="lg" />
          <span className="text-default-500 dark:text-zinc-400">
            {t("common.loading")}
          </span>
        </div>
      ) : sort.filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-default-400">
          <FaBox className="text-6xl mb-4 opacity-20" />
          <p>
            {sort.query ? t("common.no_results") : t("contentpage.no_items")}
          </p>
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-3">
            {sort.paginatedItems.map((w, idx) => (
              <motion.div
                key={`${w.Path}-${idx}`}
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                transition={{ duration: 0.2 }}
              >
                <div
                  className={cn(
                    COMPONENT_STYLES.contentListItem,
                    "w-full p-5 flex gap-5 group cursor-pointer relative overflow-hidden",
                    selection.isSelectMode && selection.selected[w.Path]
                      ? "ring-2 ring-primary bg-primary/5"
                      : "",
                  )}
                  onClick={() => {
                    if (selection.isSelectMode) selection.toggleSelect(w.Path);
                  }}
                >
                  <div className="relative shrink-0">
                    <div className="h-24 sm:h-28 aspect-video rounded-2xl bg-default-100/50 flex items-center justify-center overflow-hidden shadow-sm group-hover:shadow-md transition-shadow">
                      <Image
                        src={w.IconBase64 || DefaultWorldPreview}
                        alt={w.FolderName}
                        classNames={{
                          wrapper: "w-full h-full",
                          img: "w-full h-full object-cover object-center",
                        }}
                        radius="none"
                        fallbackSrc={DefaultWorldPreview}
                      />
                    </div>
                    {selection.isSelectMode && (
                      <div className="absolute -top-2 -left-2 z-20">
                        <Checkbox
                          isSelected={!!selection.selected[w.Path]}
                          onValueChange={() => selection.toggleSelect(w.Path)}
                          classNames={{
                            wrapper:
                              "bg-white dark:bg-zinc-900 shadow-lg scale-110",
                          }}
                        />
                      </div>
                    )}
                  </div>

                  <div className="flex flex-col flex-1 min-w-0">
                    <div className="flex justify-between items-start gap-2 mb-1">
                      <h3
                        className="text-lg font-bold text-default-900 dark:text-white truncate"
                        title={w.FolderName}
                      >
                        {w.FolderName}
                      </h3>
                    </div>

                    <div className="flex items-end justify-between mt-auto">
                      <div className="flex flex-wrap items-center gap-4 text-xs text-default-400 dark:text-zinc-500">
                        <div className="flex items-center gap-1.5 bg-default-100/50 dark:bg-zinc-800/50 px-2 py-1 rounded-lg">
                          <FaHdd className="text-default-400" />
                          <span>{formatBytes(w.Size)}</span>
                        </div>
                        <div className="flex items-center gap-1.5 bg-default-100/50 dark:bg-zinc-800/50 px-2 py-1 rounded-lg">
                          <FaClock className="text-default-400" />
                          <span>
                            {new Date(w.LastModified * 1000).toLocaleString()}
                          </span>
                        </div>
                      </div>

                      <div className="flex gap-2 opacity-0 group-hover:opacity-100 transition-opacity ml-4">
                        <Tooltip content={t("common.open")}>
                          <Button
                            isIconOnly
                            size="sm"
                            variant="flat"
                            radius="lg"
                            onClick={(e) => {
                              e.stopPropagation();
                              OpenPathDir(w.Path);
                            }}
                            className="bg-default-100 hover:bg-default-200 text-default-600 dark:bg-zinc-800 dark:hover:bg-zinc-700 dark:text-zinc-200"
                          >
                            <FaFolderOpen size={14} />
                          </Button>
                        </Tooltip>
                        <Tooltip content={t("common.backup")}>
                          <Button
                            isIconOnly
                            size="sm"
                            variant="flat"
                            radius="lg"
                            onClick={(e) => {
                              e.stopPropagation();
                              handleBackup(w);
                            }}
                            isLoading={backingUp === w.Path}
                            className="bg-default-100 hover:bg-default-200 text-default-600 dark:bg-zinc-800 dark:hover:bg-zinc-700 dark:text-zinc-200"
                          >
                            <FaArchive size={14} />
                          </Button>
                        </Tooltip>
                        <Tooltip content={t("common.edit")}>
                          <Button
                            isIconOnly
                            size="sm"
                            variant="flat"
                            radius="lg"
                            onClick={(e) => {
                              e.stopPropagation();
                              navigate(
                                `/content/worlds/worldEdit?path=${encodeURIComponent(
                                  w.Path,
                                )}`,
                              );
                            }}
                            className="bg-default-100 hover:bg-default-200 text-default-600 dark:bg-zinc-800 dark:hover:bg-zinc-700 dark:text-zinc-200"
                          >
                            <FaEdit size={14} />
                          </Button>
                        </Tooltip>
                        <Tooltip content={t("common.delete")}>
                          <Button
                            isIconOnly
                            size="sm"
                            color="danger"
                            variant="flat"
                            radius="lg"
                            onClick={(e) => {
                              e.stopPropagation();
                              setActiveWorld(w);
                              delOnOpen();
                            }}
                            className="bg-danger-50 hover:bg-danger-100 text-danger-500 dark:bg-danger-900/20 dark:hover:bg-danger-900/30"
                          >
                            <FaTrash size={14} />
                          </Button>
                        </Tooltip>
                      </div>
                    </div>
                  </div>
                </div>
              </motion.div>
            ))}
          </div>

          {sort.totalPages > 1 && (
            <div className="flex justify-center pb-4">
              <Pagination
                total={sort.totalPages}
                page={sort.currentPage}
                onChange={sort.setCurrentPage}
                showControls
                size="sm"
              />
            </div>
          )}
        </div>
      )}

      <UnifiedModal
        isOpen={transferring}
        type="primary"
        title={t("contentpage.transfer_progress_title")}
        icon={<FaExchangeAlt className="w-6 h-6" />}
        hideCloseButton
        isDismissable={false}
        showConfirmButton={false}
        showCancelButton={false}
      >
        <div className="flex flex-col gap-4">
          <Progress
            isIndeterminate
            aria-label="transferring"
            className="w-full"
            size="sm"
            color="primary"
          />
          <div className="text-default-600 dark:text-zinc-300 text-sm">
            {t("contentpage.transfer_progress_body")}
          </div>
          {currentTransferItem ? (
            <div className="p-3 bg-default-100/50 dark:bg-zinc-800 rounded-xl border border-default-200/50 text-small font-mono text-default-800 dark:text-zinc-200 break-all">
              {currentTransferItem}
            </div>
          ) : null}
        </div>
      </UnifiedModal>

      <UnifiedModal
        isOpen={transferTargetOpen}
        onOpenChange={(open) => {
          if (!open) transferTargetOnClose();
        }}
        type="primary"
        title={t("contentpage.transfer_resources_title")}
        confirmText={t("common.confirm")}
        cancelText={t("common.cancel")}
        showCancelButton
        onConfirm={() => void transferSelectedWorldsToTargets()}
        onCancel={() => transferTargetOnClose()}
        confirmButtonProps={{
          isDisabled:
            selectedTransferTargets.length === 0 ||
            selection.selectedCount === 0 ||
            transferring,
        }}
      >
        <div className="flex flex-col gap-4">
          <div className="text-sm text-default-700 dark:text-zinc-300">
            {t("contentpage.transfer_resources_body_simple")}
          </div>

          {transferTargets.length > 0 ? (
            <Select
              items={transferTargets}
              label={t("mirror.target") || "Target Instance"}
              placeholder={t("contentpage.transfer_target_placeholder")}
              selectedKeys={new Set(selectedTransferTargets)}
              onSelectionChange={(keys) => {
                const selected = Array.from(keys).map(String);
                setSelectedTransferTargets(selected);
              }}
              classNames={COMPONENT_STYLES.select}
            >
              {(item) => (
                <SelectItem key={item.name} textValue={item.name}>
                  <div className="flex gap-2 items-center">
                    <div className="w-8 h-8 rounded bg-default-200 flex items-center justify-center overflow-hidden">
                      <img
                        src={
                          item.icon ||
                          "https://raw.githubusercontent.com/LiteLDev/LeviLauncher/main/build/appicon.png"
                        }
                        alt="icon"
                        className="w-full h-full object-cover"
                        onError={(e) =>
                          (e.currentTarget.style.display = "none")
                        }
                      />
                    </div>
                    <div className="flex flex-col">
                      <span className="text-small">{item.name}</span>
                      <span className="text-tiny text-default-400">
                        {item.gameVersion}
                      </span>
                    </div>
                  </div>
                </SelectItem>
              )}
            </Select>
          ) : (
            <div className="flex flex-col items-center justify-center py-8 text-default-400 dark:text-zinc-500">
              <FaExchangeAlt className="text-4xl mb-3 opacity-20" />
              <p className="text-sm">{t("contentpage.transfer_no_targets")}</p>
            </div>
          )}
        </div>
      </UnifiedModal>

      <ImportResultModal
        isOpen={transferResultOpen}
        onOpenChange={transferResultOnOpenChange}
        results={{
          success: transferResultSuccess,
          failed: transferResultFailed,
        }}
        onConfirm={() => {
          setTransferResultSuccess([]);
          setTransferResultFailed([]);
        }}
      />

      <DeleteConfirmModal
        isOpen={delOpen}
        onOpenChange={delOnOpenChange}
        title={t("common.confirm_delete")}
        description={t("contentpage.delete_world_confirm", {
          name: activeWorld?.FolderName || "",
        })}
        itemName={activeWorld?.FolderName}
        isPending={deletingOne}
        onConfirm={handleDelete}
      />

      <DeleteConfirmModal
        isOpen={delManyCfmOpen}
        onOpenChange={delManyCfmOnOpenChange}
        title={t("common.confirm_delete")}
        description={t("contentpage.delete_selected_confirm", {
          count: selection.selectedCount,
        })}
        isPending={deletingMany}
        onConfirm={handleBatchDelete}
      />
    </PageContainer>
  );
}
