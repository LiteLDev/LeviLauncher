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
} from "bindings/github.com/liteldev/LeviLauncher/contentservice";
import { GetLocalUserGamertag } from "bindings/github.com/liteldev/LeviLauncher/userservice";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { readCurrentVersionName } from "@/utils/currentVersion";
import { DeleteConfirmModal } from "@/components/DeleteConfirmModal";
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

interface WorldInfo {
  Path: string;
  FolderName: string;
  IconBase64: string;
  Size: number;
  LastModified: number;
}

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

  const [currentWorldsPath, setCurrentWorldsPath] = useState("");

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
                      className={cn(
                        COMPONENT_STYLES.dropdownTriggerButton,
                        "w-full sm:w-auto sm:min-w-[200px]",
                      )}
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
                    variant={selection.isSelectMode ? "solid" : "flat"}
                    color={selection.isSelectMode ? "primary" : "default"}
                    onPress={selection.toggleSelectMode}
                  >
                    <FaCheckSquare />
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
                    className={cn(
                      COMPONENT_STYLES.dropdownTriggerButton,
                      "min-w-[120px]",
                    )}
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
            <span
              className={`font-medium px-2 py-0.5 rounded-md ${
                roots.isIsolation
                  ? "bg-success-50 text-success-600 dark:bg-success-900/20 dark:text-success-400"
                  : "bg-default-100 dark:bg-zinc-800 text-default-700 dark:text-zinc-200"
              }`}
            >
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
                    COMPONENT_STYLES.listItem,
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
