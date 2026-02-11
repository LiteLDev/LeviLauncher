import React, { useState, useEffect, useCallback, useMemo } from "react";
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
  FaSearch,
  FaSortAmountDown,
  FaSortAmountUp,
  FaTrash,
  FaFolderOpen,
  FaSync,
  FaArrowLeft,
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
  GetContentRoots,
  ListDir,
  GetWorldLevelName,
  GetWorldIconDataUrl,
  GetPathSize,
  GetPathModTime,
  DeleteWorld,
  BackupWorld,
  BackupWorldWithVersion,
  OpenPathDir,
} from "bindings/github.com/liteldev/LeviLauncher/minecraft";
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

interface WorldInfo {
  Path: string;
  FolderName: string;
  IconBase64: string;
  Size: number;
  LastModified: number;
}

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
  const [search, setSearch] = useState("");
  const [sortKey, setSortKey] = useState<"name" | "time">(() => {
    try {
      const saved = JSON.parse(
        localStorage.getItem("content.worlds.sort") || "{}",
      );
      const k = saved?.sortKey;
      if (k === "name" || k === "time") return k;
    } catch {}
    return "name";
  });
  const [sortAsc, setSortAsc] = useState<boolean>(() => {
    try {
      const saved = JSON.parse(
        localStorage.getItem("content.worlds.sort") || "{}",
      );
      const a = saved?.sortAsc;
      if (typeof a === "boolean") return a;
    } catch {}
    const legacy = localStorage.getItem("worlds_sort") as "asc" | "desc" | null;
    if (legacy === "desc") return false;
    return true;
  });
  const [selected, setSelected] = useState<Record<string, boolean>>({});
  const [isSelectMode, setIsSelectMode] = useState<boolean>(false);
  const [deletingOne, setDeletingOne] = useState<boolean>(false);
  const [deletingMany, setDeletingMany] = useState<boolean>(false);
  const [currentPage, setCurrentPage] = React.useState(1);
  const pageSize = 20;

  const [backingUp, setBackingUp] = useState("");
  const [activeWorld, setActiveWorld] = useState<WorldInfo | null>(null);

  const {
    isOpen: delOpen,
    onOpen: delOnOpen,
    onOpenChange: delOnOpenChange,
  } = useDisclosure();

  const {
    isOpen: delManyCfmOpen,
    onOpen: delManyCfmOnOpen,
    onOpenChange: delManyCfmOnOpenChange,
  } = useDisclosure();

  const [currentWorldsPath, setCurrentWorldsPath] = useState("");

  const scrollRef = React.useRef<HTMLDivElement | null>(null);
  const lastScrollTopRef = React.useRef<number>(0);
  const restorePendingRef = React.useRef<boolean>(false);
  const collectScrollTargets = React.useCallback(() => {
    const seen = new Set<unknown>();
    const targets: Array<Window | HTMLElement> = [];

    const add = (target: Window | HTMLElement | null | undefined) => {
      if (!target) return;
      if (seen.has(target)) return;
      seen.add(target);
      targets.push(target);
    };

    add(window);
    add((document.scrollingElement as HTMLElement) || document.documentElement);
    add(document.body);

    const walk = (seed: HTMLElement | null) => {
      let el: HTMLElement | null = seed;
      while (el) {
        add(el);
        el = el.parentElement;
      }
    };

    walk(scrollRef.current);

    return targets;
  }, []);

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

              const tag = await (minecraft as any)?.GetLocalUserGamertag?.();
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

  const persistSort = useCallback((key: "name" | "time", asc: boolean) => {
    localStorage.setItem(
      "content.worlds.sort",
      JSON.stringify({ sortKey: key, sortAsc: asc }),
    );
  }, []);

  const filtered = useMemo(() => {
    let list = [...worlds];
    if (search.trim()) {
      const q = search.toLowerCase();
      list = list.filter((w) => w.FolderName.toLowerCase().includes(q));
    }
    list.sort((a, b) => {
      if (sortKey === "name") {
        const nameA = a.FolderName.toLowerCase();
        const nameB = b.FolderName.toLowerCase();
        const res = nameA.localeCompare(nameB);
        return sortAsc ? res : -res;
      }
      const ta = Number(a.LastModified || 0);
      const tb = Number(b.LastModified || 0);
      const res = ta - tb;
      return sortAsc ? res : -res;
    });
    return list;
  }, [worlds, search, sortKey, sortAsc]);

  useEffect(() => {
    setCurrentPage(1);
  }, [search, sortKey, sortAsc]);

  const paginatedItems = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return filtered.slice(start, start + pageSize);
  }, [filtered, currentPage]);

  const totalPages = Math.ceil(filtered.length / pageSize);

  const toggleSelect = (path: string) => {
    setSelected((prev) => ({
      ...prev,
      [path]: !prev[path],
    }));
  };

  const selectAll = (val: boolean) => {
    if (val) {
      const next: Record<string, boolean> = {};
      filtered.forEach((w) => (next[w.Path] = true));
      setSelected(next);
    } else {
      setSelected({});
    }
  };

  const selectedCount = Object.keys(selected).filter((k) => selected[k]).length;

  const handleDelete = async () => {
    if (!activeWorld) return;
    setDeletingOne(true);
    try {
      await DeleteWorld(currentVersionName || "", activeWorld.Path);
      addToast({ title: t("common.success"), color: "success" });
      refreshAll();
      delOnOpenChange(false);
    } catch (e) {
      addToast({ description: String(e), color: "danger" });
    } finally {
      setDeletingOne(false);
    }
  };

  const handleBatchDelete = async () => {
    const paths = Object.keys(selected).filter((k) => selected[k]);
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
      setSelected({});
      refreshAll();
      delManyCfmOnOpenChange(false);
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

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
  };

  React.useEffect(() => {
    const resetScroll = () => {
      try {
        const active = document.activeElement as HTMLElement | null;
        if (active && scrollRef.current && scrollRef.current.contains(active)) {
          active.blur();
        }
      } catch {}

      for (const target of collectScrollTargets()) {
        if (target === window) {
          window.scrollTo({ top: 0, left: 0, behavior: "auto" });
          continue;
        }
        target.scrollTop = 0;
        target.scrollLeft = 0;
      }
    };

    resetScroll();
    const raf = requestAnimationFrame(resetScroll);
    const t0 = window.setTimeout(resetScroll, 0);
    const t1 = window.setTimeout(resetScroll, 120);
    return () => {
      cancelAnimationFrame(raf);
      clearTimeout(t0);
      clearTimeout(t1);
    };
  }, [currentPage, collectScrollTargets]);

  React.useLayoutEffect(() => {
    if (!restorePendingRef.current) return;
    requestAnimationFrame(() => {
      try {
        if (scrollRef.current)
          scrollRef.current.scrollTop = lastScrollTopRef.current;
        else window.scrollTo({ top: lastScrollTopRef.current });
      } catch {}
    });
    restorePendingRef.current = false;
  }, [worlds]);

  return (
    <PageContainer>
      <Card className={LAYOUT.GLASS_CARD.BASE}>
        <CardBody className="p-6 flex flex-col gap-6">
          <PageHeader
            title={t("contentpage.worlds_list")}
            endContent={
              <div className="flex items-center gap-2">
                <Dropdown>
                  <DropdownTrigger>
                    <Button
                      radius="full"
                      variant="flat"
                      className="bg-default-100 dark:bg-zinc-800 text-default-600 dark:text-zinc-200 font-medium w-full sm:w-auto sm:min-w-[200px]"
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
              value={search}
              onValueChange={setSearch}
              startContent={<FaFilter className="text-default-400" />}
              endContent={
                search && (
                  <button onClick={() => setSearch("")}>
                    <FaTimes className="text-default-400 hover:text-default-600" />
                  </button>
                )
              }
              radius="full"
              variant="flat"
              className="w-full md:max-w-xs"
              classNames={{
                inputWrapper:
                  "bg-default-100 dark:bg-default-50/50 hover:bg-default-200/70 transition-colors group-data-[focus=true]:bg-white dark:group-data-[focus=true]:bg-zinc-900 shadow-sm",
              }}
            />

            <div className="flex items-center gap-3">
              <Tooltip content={t("common.select_mode")}>
                <Button
                  isIconOnly
                  radius="full"
                  variant={isSelectMode ? "solid" : "flat"}
                  color={isSelectMode ? "primary" : "default"}
                  onPress={() => {
                    setIsSelectMode(!isSelectMode);
                    if (isSelectMode) setSelected({});
                  }}
                >
                  <FaCheckSquare />
                </Button>
              </Tooltip>

              {isSelectMode && (
                <Checkbox
                  isSelected={
                    filtered.length > 0 && selectedCount === filtered.length
                  }
                  onValueChange={selectAll}
                  radius="full"
                  size="lg"
                  classNames={{ wrapper: "after:bg-primary" }}
                >
                  <span className="text-sm text-default-600">
                    {t("common.select_all")}
                  </span>
                </Checkbox>
              )}

              <Dropdown>
                <DropdownTrigger>
                  <Button
                    variant="flat"
                    radius="full"
                    startContent={
                      sortAsc ? <FaSortAmountDown /> : <FaSortAmountUp />
                    }
                    className="min-w-[120px]"
                  >
                    {sortKey === "name"
                      ? (t("filemanager.sort.name") as string)
                      : (t("contentpage.sort_time") as string)}
                    {" / "}
                    {sortAsc
                      ? t("contentpage.sort_asc")
                      : t("contentpage.sort_desc")}
                  </Button>
                </DropdownTrigger>
                <DropdownMenu
                  selectionMode="single"
                  selectedKeys={
                    new Set([`${sortKey}-${sortAsc ? "asc" : "desc"}`])
                  }
                  onSelectionChange={(keys) => {
                    const val = Array.from(keys)[0] as string;
                    const [k, order] = val.split("-");
                    const nextKey = (k as "name" | "time") || "name";
                    const nextAsc = order === "asc";
                    setSortKey(nextKey);
                    setSortAsc(nextAsc);
                    persistSort(nextKey, nextAsc);
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

              <AnimatePresence>
                {selectedCount > 0 && (
                  <motion.div
                    initial={{ opacity: 0, scale: 0.9 }}
                    animate={{ opacity: 1, scale: 1 }}
                    exit={{ opacity: 0, scale: 0.9 }}
                  >
                    <Button
                      color="danger"
                      variant="flat"
                      radius="full"
                      startContent={<FaTrash />}
                      onPress={delManyCfmOnOpen}
                    >
                      {t("common.delete_selected", {
                        count: selectedCount,
                      })}
                    </Button>
                  </motion.div>
                )}
              </AnimatePresence>
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

      {loading && worlds.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 gap-4">
          <Spinner size="lg" />
          <span className="text-default-500 dark:text-zinc-400">
            {t("common.loading")}
          </span>
        </div>
      ) : filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-default-400">
          <FaBox className="text-6xl mb-4 opacity-20" />
          <p>{search ? t("common.no_results") : t("contentpage.no_items")}</p>
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-3">
            {paginatedItems.map((w, idx) => (
              <motion.div
                key={`${w.Path}-${idx}`}
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                transition={{ duration: 0.2 }}
              >
                <div
                  className={`w-full p-4 bg-white dark:bg-zinc-900/50 hover:bg-default-50 dark:hover:bg-zinc-800 transition-all rounded-2xl flex gap-4 group shadow-sm hover:shadow-md border ${
                    isSelectMode && selected[w.Path]
                      ? "border-primary bg-primary/10"
                      : "border-default-200 dark:border-zinc-700/50 hover:border-default-400 dark:hover:border-zinc-600"
                  }`}
                  onClick={() => {
                    if (isSelectMode) toggleSelect(w.Path);
                  }}
                >
                  <div className="relative shrink-0">
                    <div className="h-20 sm:h-24 aspect-video rounded-lg bg-default-100 flex items-center justify-center overflow-hidden shadow-inner">
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
                    {isSelectMode && (
                      <Checkbox
                        isSelected={!!selected[w.Path]}
                        onValueChange={() => toggleSelect(w.Path)}
                        className="absolute -top-2 -left-2 z-20"
                        classNames={{
                          wrapper: "bg-white dark:bg-zinc-900 shadow-md",
                        }}
                      />
                    )}
                  </div>

                  <div className="flex flex-col flex-1 min-w-0 gap-1">
                    <div className="flex items-baseline gap-2 truncate">
                      <h3
                        className="text-base sm:text-lg font-bold text-default-900 dark:text-white truncate"
                        title={w.FolderName}
                      >
                        {w.FolderName}
                      </h3>
                    </div>

                    <p className="text-xs sm:text-sm text-zinc-600 dark:text-zinc-300 line-clamp-2 w-full"></p>

                    <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-zinc-500 dark:text-zinc-400 mt-1">
                      <div
                        className="flex items-center gap-1"
                        title={t("common.size")}
                      >
                        <FaHdd />
                        <span>{formatBytes(w.Size)}</span>
                      </div>
                      <div
                        className="flex items-center gap-1"
                        title={t("common.date")}
                      >
                        <FaClock />
                        <span>
                          {new Date(w.LastModified * 1000).toLocaleString()}
                        </span>
                      </div>
                    </div>

                    <div className="flex flex-1 items-end justify-between mt-2">
                      <div className="flex gap-1"></div>
                      <div className="flex gap-2">
                        <Button
                          size="sm"
                          variant="flat"
                          radius="full"
                          isIconOnly
                          onPress={() => OpenPathDir(w.Path)}
                          className="h-8 w-8 min-w-0"
                          title={t("common.open")}
                        >
                          <FaFolderOpen className="text-xs" />
                        </Button>
                        <Button
                          size="sm"
                          variant="flat"
                          radius="full"
                          isIconOnly
                          onPress={() => handleBackup(w)}
                          isLoading={backingUp === w.Path}
                          className="h-8 w-8 min-w-0"
                          title={t("common.backup")}
                        >
                          <FaArchive className="text-xs" />
                        </Button>
                        <Button
                          size="sm"
                          variant="flat"
                          radius="full"
                          isIconOnly
                          onPress={() =>
                            navigate(
                              `/content/worlds/worldEdit?path=${encodeURIComponent(
                                w.Path,
                              )}`,
                            )
                          }
                          className="h-8 w-8 min-w-0"
                          title={t("common.edit")}
                        >
                          <FaEdit className="text-xs" />
                        </Button>
                        <Button
                          size="sm"
                          color="danger"
                          variant="flat"
                          radius="full"
                          isIconOnly
                          onPress={() => {
                            setActiveWorld(w);
                            delOnOpen();
                          }}
                          className="h-8 w-8 min-w-0"
                          title={t("common.delete")}
                        >
                          <FaTrash className="text-xs" />
                        </Button>
                      </div>
                    </div>
                  </div>
                </div>
              </motion.div>
            ))}
          </div>

          {totalPages > 1 && (
            <div className="flex justify-center pb-4">
              <Pagination
                total={totalPages}
                page={currentPage}
                onChange={setCurrentPage}
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
          count: selectedCount,
        })}
        isPending={deletingMany}
        onConfirm={handleBatchDelete}
      />
    </PageContainer>
  );
}
