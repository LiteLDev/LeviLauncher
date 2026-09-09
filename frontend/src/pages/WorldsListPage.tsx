import { ModalDescription, ModalProgress } from "@/components/ModalPrimitives";
import { PagePagination } from "@/components/PagePagination";
import {
  Button,
  Card,
  Checkbox,
  Dropdown,
  InputGroup,
  Label,
  ListBox, Select,
  Spinner,
  TextField,
  Tooltip,
  toast,
  useOverlayState
} from "@heroui/react";

import React, { useState, useEffect, useCallback } from "react";
import { PageHeader } from "@/components/PageHeader";
import { routeTo } from "@/constants/routes";

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
import { readCurrentVersionName } from "@/utils/currentVersion";
import { deleteContentItems } from "@/utils/contentDeletion";
import { DeleteConfirmModal } from "@/components/DeleteConfirmModal";
import { UnifiedModal } from "@/components/UnifiedModal";
import { ImportResultModal } from "@/components/ImportResultModal";
import { useTranslation } from "react-i18next";
import { motion } from "framer-motion";
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
  const selectedPlayerRef = React.useRef(selectedPlayer);
  selectedPlayerRef.current = selectedPlayer;
  const playerWasChosen = React.useRef(Boolean(location.state?.player));
  const [players, setPlayers] = useState<string[]>([]);
  const [playerGamertagMap, setPlayerGamertagMap] = useState<
    Record<string, string>
  >({});
  const [worlds, setWorlds] = useState<WorldInfo[]>([]);
  const worldsLoadGeneration = React.useRef(0);
  const [loading, setLoading] = useState(false);
  const [roots, setRoots] = useState<any>({});

  const [deletingOne, setDeletingOne] = useState<boolean>(false);
  const [deletingMany, setDeletingMany] = useState<boolean>(false);

  const [backingUp, setBackingUp] = useState("");
  const [activeWorld, setActiveWorld] = useState<WorldInfo | null>(null);

  const {
    isOpen: delOpen,
    open: delOnOpen,
    close: delOnClose,
    setOpen: delOnOpenChange,
  } = useOverlayState();

  const {
    isOpen: delManyCfmOpen,
    open: delManyCfmOnOpen,
    close: delManyCfmOnClose,
    setOpen: delManyCfmOnOpenChange,
  } = useOverlayState();
  const {
    isOpen: transferTargetOpen,
    open: transferTargetOnOpen,
    close: transferTargetOnClose,
    setOpen: transferTargetOnOpenChange,
  } = useOverlayState();
  const {
    isOpen: transferResultOpen,
    open: transferResultOnOpen,
    setOpen: transferResultOnOpenChange,
  } = useOverlayState();

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
  const contentScope = JSON.stringify([currentVersionName, selectedPlayer]);
  const contentScopeRef = React.useRef(contentScope);
  contentScopeRef.current = contentScope;
  const selection = useSelectionMode(sort.filtered, (w: WorldInfo) => w.Path, JSON.stringify([currentVersionName, selectedPlayer]), worlds);
  React.useEffect(() => {
    delOnOpenChange(false);
    delManyCfmOnOpenChange(false);
    setActiveWorld(null);
  }, [currentVersionName, selectedPlayer]);


  useEffect(() => {
    let cancelled = false;
    const fetchPlayers = async () => {
      try {
        const r = await GetContentRoots(currentVersionName || "");
        if (cancelled) return;
        setRoots(r);
        if (r.usersRoot) {
          const pList = await listPlayers(r.usersRoot);
          if (cancelled) return;
          setPlayers(pList);

          let defaultP = "";
          if (!pList.includes(selectedPlayerRef.current) && pList.length > 0) {
            defaultP = pList[0];
            playerWasChosen.current = false;
            setSelectedPlayer(defaultP);
          }

          (async () => {
            try {
              const map = await getPlayerGamertagMap(r.usersRoot);
              if (cancelled) return;
              setPlayerGamertagMap(map);

              const tag = await GetLocalUserGamertag();
              if (cancelled || playerWasChosen.current) return;
              if (tag) {
                for (const p of pList) {
                  if (map[p] === tag) {
                    if (
                      p !== defaultP &&
                      (!selectedPlayerRef.current || selectedPlayerRef.current === defaultP)
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
        if (cancelled) return;
        console.error("Failed to list players", e);
        setPlayers([]);
        setPlayerGamertagMap({});
      }
    };
    fetchPlayers();
    return () => { cancelled = true; };
  }, [currentVersionName]);

  const refreshAll = useCallback(() => {
    const generation = ++worldsLoadGeneration.current;
    setLoading(true);

    const fetchWorlds = async () => {
      try {
        const r = await GetContentRoots(currentVersionName || "");
        if (generation !== worldsLoadGeneration.current) return;
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
        if (generation !== worldsLoadGeneration.current) return;
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

        if (generation === worldsLoadGeneration.current) setWorlds(list);
      } catch (err: any) {
        if (generation !== worldsLoadGeneration.current) return;
        console.error(err);
        toast(undefined, {
          description: String(err),
          variant: "danger",
          timeout: 2000,
        });
      } finally {
        if (generation === worldsLoadGeneration.current) setLoading(false);
      }
    };

    return fetchWorlds();
  }, [selectedPlayer, currentVersionName]);

  useEffect(() => {
    setWorlds([]);
    refreshAll();
    if (selectedPlayer) {
      localStorage.setItem("content.selectedPlayer", selectedPlayer);
    }
    return () => { worldsLoadGeneration.current++; };
  }, [selectedPlayer, refreshAll]);

  const handleDelete = async () => {
    if (!activeWorld) return;
    setDeletingOne(true);
    try {
      const result = await DeleteWorld(currentVersionName || "", activeWorld.Path);
      if (result) throw new Error(result);
      toast(t("common.success"), { variant: "success", timeout: 2000 });
      if (contentScopeRef.current === contentScope) refreshAll();
      delOnClose();
    } catch (e) {
      toast(undefined, {
        description: String(e),
        variant: "danger",
        timeout: 2000,
      });
      throw e;
    } finally {
      setDeletingOne(false);
    }
  };

  const handleBatchDelete = async () => {
    const targets = selection.getSelectedKeys();
    if (!targets.length) return false;
    setDeletingMany(true);
    try {
      return await deleteContentItems(targets, (path) => DeleteWorld(currentVersionName || "", path), selection.retainSelection, () => refreshAll(), t, () => contentScopeRef.current === contentScope);
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
        toast(t("contentpage.backup_success"), {
          variant: "success",
          timeout: 2000,
        });
      } else {
        toast(undefined, {
          description: t("contentpage.backup_failed"),
          variant: "danger",
          timeout: 2000,
        });
      }
    } catch (e) {
      toast(undefined, {
        description: t("contentpage.backup_failed") + ": " + String(e),
        variant: "danger",
        timeout: 2000,
      });
    } finally {
      setBackingUp("");
    }
  };

  const openTransferTargetModal = useCallback(async () => {
    if (transferring || selection.selectedCount === 0) return;

    const sourceVersionName = currentVersionName || readCurrentVersionName();
    if (!sourceVersionName) {
      toast(t("launcherpage.currentVersion_none") as string, {
        variant: "danger",
        timeout: 2000,
      });
      return;
    }
    if (!selectedPlayer) {
      toast(t("contentpage.require_player_for_world_import") as string, {
        variant: "danger",
        timeout: 2000,
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
      toast("Error", {
        description: String(e),
        variant: "danger",
        timeout: 2000,
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
      toast(t("contentpage.require_player_for_world_import") as string, {
        variant: "danger",
        timeout: 2000,
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
      toast("Error", {
        description: String(e),
        variant: "danger",
        timeout: 2000,
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
        <Card.Content className="p-6 flex flex-col gap-6">
          <PageHeader
            title={t("contentpage.worlds_list")}
            endContent={
              <div className="flex items-center gap-2">
                <Dropdown>
                  <Button
                    isDisabled={!players.length}
                    variant={"secondary"}
                    className={cn(
                      "rounded-full",
                      "w-full sm:w-auto sm:min-w-[200px] bg-surface-secondary dark:bg-zinc-800 text-foreground dark:text-zinc-200 font-medium",
                    )}
                  >
                    {<FaUser />}
                    {selectedPlayer
                      ? resolvePlayerDisplayName(
                          selectedPlayer,
                          playerGamertagMap,
                        )
                      : t("contentpage.select_player")}
                  </Button>
                  <Dropdown.Popover
                    className={COMPONENT_STYLES.dropdown.content}
                  >
                    <Dropdown.Menu
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
                          playerWasChosen.current = true;
                          setSelectedPlayer(next);
                      }}
                    >
                      {players.length ? (
                        players.map((p) => (
                          <Dropdown.Item
                            key={p}
                            id={p}
                            textValue={resolvePlayerDisplayName(
                              p,
                              playerGamertagMap,
                            )}
                          >
                            <Label>
                              {resolvePlayerDisplayName(p, playerGamertagMap)}
                            </Label>
                            <Dropdown.ItemIndicator />
                          </Dropdown.Item>
                        ))
                      ) : (
                        <Dropdown.Item
                          key="none"
                          isDisabled
                          id={"none"}
                          textValue={t("contentpage.no_players")}
                        >
                          <Label>{t("contentpage.no_players")}</Label>
                          <Dropdown.ItemIndicator />
                        </Dropdown.Item>
                      )}
                    </Dropdown.Menu>
                  </Dropdown.Popover>
                </Dropdown>
                <Button
                  onPress={() => {
                    if (currentWorldsPath) OpenPathDir(currentWorldsPath);
                  }}
                  isDisabled={!currentWorldsPath}
                  variant={"secondary"}
                  className={cn(
                    "rounded-full",
                    "bg-surface-secondary dark:bg-zinc-800 text-foreground dark:text-zinc-200 font-medium",
                  )}
                >
                  {<FaFolderOpen />}
                  {t("common.open")}
                </Button>
                <Tooltip>
                  <Button
                    isIconOnly
                    aria-label={t("common.select_mode")}
                    onPress={selection.toggleSelectMode}
                    variant={"secondary"}
                    className={cn(
                      "rounded-full",
                      "bg-surface-secondary dark:bg-zinc-800 text-foreground dark:text-zinc-200",
                    )}
                  >
                    <FaCheckSquare />
                  </Button>
                  <Tooltip.Content>{t("common.select_mode")}</Tooltip.Content>
                </Tooltip>
                <Tooltip>
                  <Button
                    isIconOnly
                    aria-label={t("common.refresh")}
                    onPress={() => refreshAll()}
                    isDisabled={loading}
                    variant={"secondary"}
                    className={cn(
                      "rounded-full",
                      "bg-surface-secondary dark:bg-zinc-800 text-foreground dark:text-zinc-200",
                    )}
                  >
                    <FaSync
                      className={loading ? "animate-spin" : ""}
                      size={18}
                    />
                  </Button>
                  <Tooltip.Content>
                    {t("common.refresh") as unknown as string}
                  </Tooltip.Content>
                </Tooltip>
              </div>
            }
          />
          <div className="flex flex-col md:flex-row gap-4 items-end md:items-center justify-between">
            <TextField
              aria-label={t("common.search_placeholder") as string}
              className={cn(
                "group",
                COMPONENT_STYLES.input.mainWrapper,
                "w-full md:max-w-xs",
              )}
              value={sort.query}
              onChange={sort.setQuery}
            >
              <InputGroup
                className={cn(
                  COMPONENT_STYLES.input.inputWrapper,
                  COMPONENT_STYLES.input.innerWrapper,
                  "rounded-full",
                )}
              >
                <InputGroup.Prefix>
                  {<FaFilter className="text-muted" />}
                </InputGroup.Prefix>
                <InputGroup.Input
                  placeholder={t("common.search_placeholder") as string}
                  className={COMPONENT_STYLES.input.input}
                />
                <InputGroup.Suffix>
                  {sort.query && (
                    <button onClick={() => sort.setQuery("")}>
                      <FaTimes />
                    </button>
                  )}
                </InputGroup.Suffix>
              </InputGroup>
            </TextField>

            <div className="flex items-center gap-3">
              <Dropdown>
                <Button
                  variant={"secondary"}
                  className={cn(
                    "rounded-full",
                    "min-w-[120px] bg-surface-secondary dark:bg-zinc-800 text-foreground dark:text-zinc-200 font-medium",
                  )}
                >
                  {sort.sortAsc ? <FaSortAmountDown /> : <FaSortAmountUp />}
                  {sort.sortKey === "name"
                    ? (t("filemanager.sort.name") as string)
                    : (t("contentpage.sort_time") as string)}
                  {" / "}
                  {sort.sortAsc
                    ? t("contentpage.sort_asc")
                    : t("contentpage.sort_desc")}
                </Button>
                <Dropdown.Popover className={COMPONENT_STYLES.dropdown.content}>
                  <Dropdown.Menu
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
                    <Dropdown.Item
                      key="name-asc"
                      id={"name-asc"}
                      textValue={String("name-asc")}
                    >
                      {<FaSortAmountDown />}
                      <Label>{t("filemanager.sort.name")}(A-Z)</Label>
                      <Dropdown.ItemIndicator />
                    </Dropdown.Item>
                    <Dropdown.Item
                      key="name-desc"
                      id={"name-desc"}
                      textValue={String("name-desc")}
                    >
                      {<FaSortAmountUp />}
                      <Label>{t("filemanager.sort.name")}(Z-A)</Label>
                      <Dropdown.ItemIndicator />
                    </Dropdown.Item>
                    <Dropdown.Item
                      key="time-asc"
                      id={"time-asc"}
                      textValue={String("time-asc")}
                    >
                      {<FaSortAmountDown />}
                      <Label>{t("contentpage.sort_old_new")}</Label>
                      <Dropdown.ItemIndicator />
                    </Dropdown.Item>
                    <Dropdown.Item
                      key="time-desc"
                      id={"time-desc"}
                      textValue={String("time-desc")}
                    >
                      {<FaSortAmountUp />}
                      <Label>{t("contentpage.sort_new_old")}</Label>
                      <Dropdown.ItemIndicator />
                    </Dropdown.Item>
                  </Dropdown.Menu>
                </Dropdown.Popover>
              </Dropdown>
            </div>
          </div>
          <div className="mt-2 text-muted dark:text-zinc-400 text-sm flex flex-wrap items-center gap-2">
            <span>{t("contentpage.current_version")}:</span>
            <span className="font-medium text-foreground dark:text-zinc-200 bg-surface-secondary dark:bg-zinc-800 px-2 py-0.5 rounded-md">
              {currentVersionName || t("contentpage.none")}
            </span>
            <span className="text-muted dark:text-zinc-600">|</span>
            <span>{t("contentpage.isolation")}:</span>
            <span className="font-medium text-foreground dark:text-zinc-200 bg-surface-secondary dark:bg-zinc-800 px-2 py-0.5 rounded-md">
              {roots.isIsolation ? t("common.yes") : t("common.no")}
            </span>
          </div>
        </Card.Content>
      </Card>

      <SelectionBar
        hiddenSelectedCount={selection.hiddenSelectedCount}
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
          <span className="text-muted dark:text-zinc-400">
            {t("common.loading")}
          </span>
        </div>
      ) : sort.filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-muted">
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
                    "w-full p-5 flex gap-5 group relative overflow-hidden",
                    selection.isSelectMode ? "cursor-pointer" : "cursor-default",
                    selection.isSelectMode && selection.selected[w.Path]
                      ? "ring-2 ring-accent bg-accent/5"
                      : "",
                  )}
                  onClick={() => {
                    if (selection.isSelectMode) selection.toggleSelect(w.Path);
                  }}
                >
                  <div className="relative shrink-0">
                    <div className="h-24 sm:h-28 aspect-video rounded-2xl bg-surface-secondary/50 flex items-center justify-center overflow-hidden shadow-sm group-hover:shadow-md transition-shadow">
                      <img
                        src={w.IconBase64 || DefaultWorldPreview}
                        alt={w.FolderName}
                        className={cn(
                          "rounded-none",
                          "w-full h-full",
                          "w-full h-full object-cover object-center",
                        )}
                        onError={(event) => {
                          if (event.currentTarget.dataset.fallbackApplied)
                            return;
                          event.currentTarget.dataset.fallbackApplied = "true";
                          event.currentTarget.src = DefaultWorldPreview;
                        }}
                      />
                    </div>
                    {selection.isSelectMode && (
                      <div className="absolute -top-2 -left-2 z-20">
                        <Checkbox
                          aria-label={t("contentpage.select_item", { name: w.FolderName })}
                          onClick={(event) => event.stopPropagation()}
                          isSelected={!!selection.selected[w.Path]}
                          onChange={() => selection.toggleSelect(w.Path)}
                          className={"group"}
                        >
                          <Checkbox.Content>
                            <Checkbox.Control
                              className={
                                "bg-white dark:bg-zinc-900 shadow-lg scale-110"
                              }
                            >
                              <Checkbox.Indicator />
                            </Checkbox.Control>
                            <span></span>
                          </Checkbox.Content>
                        </Checkbox>
                      </div>
                    )}
                  </div>

                  <div className="flex flex-col flex-1 min-w-0">
                    <div className="flex justify-between items-start gap-2 mb-1">
                      <h3
                        className="text-lg font-bold text-foreground dark:text-white truncate"
                        title={w.FolderName}
                      >
                        {w.FolderName}
                      </h3>
                    </div>

                    <div className="flex items-end justify-between mt-auto">
                      <div className="flex flex-wrap items-center gap-4 text-xs text-muted dark:text-zinc-500">
                        <div className="flex items-center gap-1.5 bg-surface-secondary/50 dark:bg-zinc-800/50 px-2 py-1 rounded-lg">
                          <FaHdd className="text-muted" />
                          <span>{formatBytes(w.Size)}</span>
                        </div>
                        <div className="flex items-center gap-1.5 bg-surface-secondary/50 dark:bg-zinc-800/50 px-2 py-1 rounded-lg">
                          <FaClock className="text-muted" />
                          <span>
                            {new Date(w.LastModified * 1000).toLocaleString()}
                          </span>
                        </div>
                      </div>

                      <div className="flex gap-2 opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 transition-opacity ml-4">
                        <Tooltip>
                          <Button
                            isIconOnly
                    aria-label={t("common.open")}
                            size="sm"
                            variant={"secondary"}
                            onClick={(event) => event.stopPropagation()}
                            onPress={(e) => {
                              OpenPathDir(w.Path);
                            }}
                            className={cn(
                              "rounded-full",
                              "bg-surface-secondary hover:bg-surface-tertiary text-foreground dark:bg-zinc-800 dark:hover:bg-zinc-700 dark:text-zinc-200",
                            )}
                          >
                            <FaFolderOpen size={14} />
                          </Button>
                          <Tooltip.Content>{t("common.open")}</Tooltip.Content>
                        </Tooltip>
                        <Tooltip>
                          <Button
                            isIconOnly
                    aria-label={t("common.backup")}
                            size="sm"
                            variant={"secondary"}
                            isPending={backingUp === w.Path}
                            onClick={(event) => event.stopPropagation()}
                            onPress={(e) => {
                              handleBackup(w);
                            }}
                            className={cn(
                              "rounded-full",
                              "bg-surface-secondary hover:bg-surface-tertiary text-foreground dark:bg-zinc-800 dark:hover:bg-zinc-700 dark:text-zinc-200",
                            )}
                          >
                            {({ isPending }) => (
                              <>
                                <Spinner
                                  size="sm"
                                  color="current"
                                  className={isPending ? "" : "hidden"}
                                />
                                <FaArchive size={14} />
                              </>
                            )}
                          </Button>
                          <Tooltip.Content>
                            {t("common.backup")}
                          </Tooltip.Content>
                        </Tooltip>
                        <Tooltip>
                          <Button
                            isIconOnly
                    aria-label={t("common.edit")}
                            size="sm"
                            variant={"secondary"}
                            onClick={(event) => event.stopPropagation()}
                            onPress={(e) => {
                              navigate(routeTo.contentWorldEditor(w.Path));
                            }}
                            className={cn(
                              "rounded-full",
                              "bg-surface-secondary hover:bg-surface-tertiary text-foreground dark:bg-zinc-800 dark:hover:bg-zinc-700 dark:text-zinc-200",
                            )}
                          >
                            <FaEdit size={14} />
                          </Button>
                          <Tooltip.Content>{t("common.edit")}</Tooltip.Content>
                        </Tooltip>
                        <Tooltip>
                          <Button
                            isIconOnly
                    aria-label={t("common.delete")}
                            size="sm"
                            variant={"danger-soft"}
                            onClick={(event) => event.stopPropagation()}
                            onPress={(e) => {
                              setActiveWorld(w);
                              delOnOpen();
                            }}
                            className={cn(
                              "rounded-full",
                              "bg-rose-50 hover:bg-rose-100 text-rose-500 dark:bg-rose-900/20 dark:hover:bg-rose-900/30",
                            )}
                          >
                            <FaTrash size={14} />
                          </Button>
                          <Tooltip.Content>
                            {t("common.delete")}
                          </Tooltip.Content>
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
              <PagePagination
                size="sm"
                pageCount={sort.totalPages}
                currentPage={sort.currentPage}
                onPageChange={sort.setCurrentPage}
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
        isDismissable={false}
        showConfirmButton={false}
        showCancelButton={false}
      >
        <ModalProgress
          label={t("contentpage.transfer_progress_title")}
          description={<> {t("contentpage.transfer_progress_body")} </>}
          currentItem={currentTransferItem}
        />
      </UnifiedModal>

      <UnifiedModal
        size="wide"
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
          <ModalDescription>
            {t("contentpage.transfer_resources_body_simple")}
          </ModalDescription>

          {transferTargets.length > 0 ? (
            <Select
              placeholder={t("contentpage.transfer_target_placeholder")}
              value={Array.from(new Set(selectedTransferTargets))[0] ?? null}
              onChange={(keys) => {
                const selected = [keys].map(String);
                setSelectedTransferTargets(selected);
              }}
            >
              <Label>{t("mirror.target") || "Target Instance"}</Label>
              <Select.Trigger className={COMPONENT_STYLES.select.trigger}>
                <Select.Value />
                <Select.Indicator />
              </Select.Trigger>
              <Select.Popover
                className={COMPONENT_STYLES.select.popoverContent}
              >
                <ListBox
                  items={transferTargets}
                  className={COMPONENT_STYLES.select.listbox}
                >
                  {(item) => (
                    <ListBox.Item
                      key={item.name}
                      id={item.name}
                      textValue={item.name}
                    >
                      <Label>
                        <div className="flex gap-2 items-center">
                          <div className="w-8 h-8 rounded bg-surface-tertiary flex items-center justify-center overflow-hidden">
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
                            <span className="text-sm">{item.name}</span>
                            <span className="text-xs text-muted">
                              {item.gameVersion}
                            </span>
                          </div>
                        </div>
                      </Label>
                      <ListBox.ItemIndicator />
                    </ListBox.Item>
                  )}
                </ListBox>
              </Select.Popover>
            </Select>
          ) : (
            <div className="flex flex-col items-center justify-center py-8 text-muted dark:text-zinc-500">
              <FaExchangeAlt className="text-4xl mb-3 opacity-20" />
              <ModalDescription>{t("contentpage.transfer_no_targets")}</ModalDescription>
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
        scopeLabel={t("contentpage.delete_scope", { instance: currentVersionName, player: selectedPlayer || t("contentpage.select_player") })}
        itemNames={worlds.filter((item) => selection.selected[item.Path]).map((item) => item.FolderName)}
        confirmDisabled={selection.selectedCount === 0}
        description={t("contentpage.delete_selected_confirm", {
          count: selection.selectedCount,
        })}
        isPending={deletingMany}
        onConfirm={handleBatchDelete}
      />
    </PageContainer>
  );
}
