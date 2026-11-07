import { resolveContentPath, isContentTransferTarget } from "@/utils/content";
import { openDirectory } from "@/utils/explorer";
import { ModalDescription, ModalNotice } from "@/components/ModalPrimitives";
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
  useOverlayState
} from "@heroui/react";

import React, { useState, useEffect, useCallback } from "react";
import { PageHeader } from "@/components/PageHeader";
import { ROUTES, routeTo } from "@/constants/routes";

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
  FaArrowLeft,
} from "react-icons/fa";
import { useLocation, useNavigate, useNavigationType } from "react-router-dom";
import {
  ListDir,
  GetWorldLevelName,
  GetWorldIconDataUrl,
  GetPathSize,
  GetPathModTime,
  BackupWorld,
  BackupWorldWithVersion,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import {
  GetContentRoots,
  DeleteWorld,
  TransferWorldToVersion,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/contentservice";
import {
  GetVersionLogoDataUrl,
  ListVersionMetas,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/versionservice";
import { GetLocalUserGamertag } from "bindings/github.com/liteldev/LeviLauncher/internal/app/userservice";
import { useCurrentVersion } from "@/utils/CurrentVersionContext";
import { notify } from "@/utils/activity";
import { InstancePicker } from "@/components/InstancePicker";
import { ContentTaskDialog, useContentTask } from "@/components/ContentTaskDialog";
import { deleteContentItems } from "@/utils/contentDeletion";
import { DeleteConfirmModal } from "@/components/DeleteConfirmModal";
import { UnifiedModal } from "@/components/UnifiedModal";
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

// Page and scroll position kept while the user visits a world's editor.
const VIEW_KEY = "content.worlds.view";

export default function WorldsListPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const navigationType = useNavigationType();
  const { currentVersionName } = useCurrentVersion();

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
  const [loadError, setLoadError] = useState("");
  const [failedWorlds, setFailedWorlds] = useState<string[]>([]);
  const [playerLoadError, setPlayerLoadError] = useState("");
  const [playersLoading, setPlayersLoading] = useState(true);
  const [playerRefresh, setPlayerRefresh] = useState(0);
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
    setOpen: delManyCfmOnOpenChange,
  } = useOverlayState();
  const {
    isOpen: transferTargetOpen,
    open: transferTargetOnOpen,
    close: transferTargetOnClose,
  } = useOverlayState();
  const transferTask = useContentTask("transfer");
  const transferring = transferTask.isBusy;

  const [currentWorldsPath, setCurrentWorldsPath] = useState("");
  const [transferTargets, setTransferTargets] = useState<
    TransferTargetVersion[]
  >([]);
  const [selectedTransferTargets, setSelectedTransferTargets] = useState<
    string[]
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

  // Switching the instance in place starts over with that instance's players.
  const shownVersion = React.useRef(currentVersionName);
  useEffect(() => {
    if (shownVersion.current === currentVersionName) return;
    shownVersion.current = currentVersionName;
    playerWasChosen.current = false;
    selectedPlayerRef.current = "";
    setSelectedPlayer("");
  }, [currentVersionName]);


  useEffect(() => {
    let cancelled = false;
    const fetchPlayers = async () => {
      setPlayersLoading(true);
      setPlayerLoadError("");
      if (!currentVersionName) {
        // The page offers the instance picker inline instead.
        setRoots({});
        setPlayers([]);
        setPlayerGamertagMap({});
        setPlayersLoading(false);
        return;
      }
      try {
        const r = await GetContentRoots(currentVersionName || "");
        if (cancelled) return;
        setRoots(r);
        if (r.usersRoot) {
          const pList = await listPlayers(r.usersRoot, true);
          if (cancelled) return;
          setPlayers(pList);
          if (pList.length === 0) setSelectedPlayer("");

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
          setSelectedPlayer("");
          setPlayerGamertagMap({});
        }
      } catch (e) {
        if (cancelled) return;
        console.error("Failed to list players", e);
        setPlayerLoadError(String(e));
        setPlayers([]);
        setPlayerGamertagMap({});
      } finally {
        if (!cancelled) setPlayersLoading(false);
      }
    };
    fetchPlayers();
    return () => { cancelled = true; };
  }, [currentVersionName, playerRefresh]);

  const refreshAll = useCallback(() => {
    const generation = ++worldsLoadGeneration.current;
    setLoading(true);
    setLoadError("");
    setFailedWorlds([]);

    const fetchWorlds = async () => {
      if (!currentVersionName) {
        setCurrentWorldsPath("");
        setWorlds([]);
        setLoading(false);
        return;
      }
      try {
        const r = await GetContentRoots(currentVersionName || "");
        if (generation !== worldsLoadGeneration.current) return;
        setRoots(r);
        const worldsPath = resolveContentPath(r, "minecraftWorlds", selectedPlayer);
        if (!worldsPath) {
          setCurrentWorldsPath("");
          setWorlds([]);
          return;
        }

        setCurrentWorldsPath(worldsPath);

        const entries = await ListDir(worldsPath);
        if (generation !== worldsLoadGeneration.current) return;
        if (!entries) {
          setWorlds([]);
          return;
        }

        const list: WorldInfo[] = [];
        const failed: string[] = [];
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
              failed.push(e.name);
            }
          }),
        );

        if (generation === worldsLoadGeneration.current) {
          setWorlds(list);
          setFailedWorlds(failed);
        }
      } catch (err: any) {
        if (generation !== worldsLoadGeneration.current) return;
        console.error(err);
        setLoadError(String(err));
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
      notify({
        variant: "success",
        title: t("content_task.world_deleted"),
        description: activeWorld.FolderName,
        source: "content",
      });
      if (contentScopeRef.current === contentScope) refreshAll();
      delOnClose();
    } catch (e) {
      notify({
        variant: "danger",
        title: t("content_task.delete_failed"),
        description: `${activeWorld.FolderName}: ${String(e)}`,
        source: "content",
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
        notify({
          variant: "success",
          title: t("contentpage.backup_success"),
          description: dest,
          source: "content",
        });
      } else {
        notify({
          variant: "danger",
          title: t("contentpage.backup_failed"),
          description: w.FolderName,
          source: "content",
        });
      }
    } catch (e) {
      notify({
        variant: "danger",
        title: t("contentpage.backup_failed"),
        description: `${w.FolderName}: ${String(e)}`,
        source: "content",
      });
    } finally {
      setBackingUp("");
    }
  };

  const openWorldEditor = (w: WorldInfo) => {
    try {
      sessionStorage.setItem(
        VIEW_KEY,
        JSON.stringify({
          scope: contentScope,
          query: sort.query,
          page: sort.currentPage,
          scrollTop: scrollRef.current?.scrollTop || 0,
        }),
      );
    } catch {}
    navigate(routeTo.contentWorldEditor(w.Path));
  };

  // Coming back from the editor restores the page and scroll position once.
  const viewRestored = React.useRef(false);
  useEffect(() => {
    if (viewRestored.current || loading || playersLoading || worlds.length === 0) return;
    viewRestored.current = true;
    let saved: { scope?: string; query?: string; page?: number; scrollTop?: number } | null = null;
    try {
      saved = JSON.parse(sessionStorage.getItem(VIEW_KEY) || "null");
      sessionStorage.removeItem(VIEW_KEY);
    } catch {}
    // Only a back/forward step returns to where the user was.
    if (!saved || navigationType !== "POP" || saved.scope !== contentScope) return;
    if (saved.query) sort.setQuery(saved.query);
    const page = Number(saved.page || 1);
    const top = Number(saved.scrollTop || 0);
    // Apply the page after the query's page reset, then the scroll after
    // useScrollManager's page-change reset.
    window.setTimeout(() => {
      if (page > 1) sort.setCurrentPage(page);
      window.setTimeout(() => {
        if (scrollRef.current) scrollRef.current.scrollTop = top;
      }, 250);
    }, 0);
  }, [loading, playersLoading, worlds.length, contentScope]);

  const openTransferTargetModal = useCallback(async () => {
    if (transferring || selection.selectedCount === 0) return;

    const sourceVersionName = currentVersionName;
    // The page shows the instance and player pickers inline, so these only
    // guard against a stale button press.
    if (!sourceVersionName || (!selectedPlayer && roots.packageType !== "uwp")) {
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
      notify({
        variant: "danger",
        title: t("content_task.load_targets_failed"),
        description: String(e),
        source: "content",
      });
    }
  }, [
    transferring,
    selection.selectedCount,
    currentVersionName,
    selectedPlayer,
    roots.packageType,
    roots.isPreview,
    roots.isIsolation,
    t,
    transferTargetOnOpen,
  ]);

  const transferSelectedWorldsToTargets = useCallback(async () => {
    if (transferring) return;

    const sourceVersionName = currentVersionName;
    if (!sourceVersionName || (!selectedPlayer && roots.packageType !== "uwp")) {
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

    transferTask.open(selectedWorldPaths.length * targetNames.length);
    let anySuccess = false;
    try {
      for (const targetName of targetNames) {
        const targetRoots = await GetContentRoots(targetName);
        let targetPlayer = "";
        if (targetRoots.packageType !== "uwp") {
          const targetPlayers = (await listPlayers(targetRoots.usersRoot, true))
            .filter((player) => player.toLowerCase() !== "shared");
          targetPlayer = targetPlayers.includes(selectedPlayer) ? selectedPlayer : targetPlayers[0] || "";
        }
        for (const worldPath of selectedWorldPaths) {
          const worldName =
            worldNameMap.get(worldPath) ||
            getPathBaseName(worldPath) ||
            worldPath;
          const id = transferTask.begin(`${worldName} -> ${targetName}`);
          try {
            const err = await TransferWorldToVersion(
              sourceVersionName,
              selectedPlayer,
              worldPath,
              targetName,
              targetPlayer,
            );
            if (err) {
              transferTask.end(id, "failed", String(err));
              continue;
            }
          } catch (e) {
            transferTask.end(id, "failed", String(e));
            continue;
          }
          anySuccess = true;
          transferTask.end(id, "success");
        }
      }
      transferTask.finish();
    } catch (e) {
      transferTask.fail(e);
    } finally {
      if (anySuccess) selection.clearSelection();
    }
  }, [
    transferring,
    currentVersionName,
    selectedPlayer,
    roots.packageType,
    selection,
    selectedTransferTargets,
    transferTargetOnClose,
    worlds,
    transferTask,
  ]);

  const playerLabel = (p: string) => resolvePlayerDisplayName(p, playerGamertagMap);
  const needsPlayer = !selectedPlayer && roots.packageType !== "uwp";
  const playerPicker = (className?: string) => (
    <Select
      aria-label={t("contentpage.select_player")}
      className={cn("min-w-0 w-48 max-w-full", className)}
      placeholder={players.length ? t("contentpage.select_player") : t("contentpage.no_players")}
      isDisabled={!players.length}
      value={selectedPlayer || null}
      onChange={(key) => {
        const next = String(key ?? "");
        if (!next) return;
        playerWasChosen.current = true;
        setSelectedPlayer(next);
      }}
    >
      <Select.Trigger className={cn(COMPONENT_STYLES.select.trigger, "min-h-8 h-8 gap-2 text-sm")}>
        <FaUser className="shrink-0 text-muted" aria-hidden="true" />
        <Select.Value />
        <Select.Indicator />
      </Select.Trigger>
      <Select.Popover className={COMPONENT_STYLES.select.popoverContent}>
        <ListBox items={players.map((p) => ({ id: p }))} className={COMPONENT_STYLES.select.listbox}>
          {(item) => (
            <ListBox.Item key={item.id} id={item.id} textValue={playerLabel(item.id)}>
              <span className="truncate text-sm">{playerLabel(item.id)}</span>
              <ListBox.ItemIndicator />
            </ListBox.Item>
          )}
        </ListBox>
      </Select.Popover>
    </Select>
  );

  return (
    <PageContainer ref={scrollRef}>
      <Card className={LAYOUT.GLASS_CARD.BASE}>
        <Card.Content className="p-4 min-[1200px]:p-6 flex flex-col gap-4">
          <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-3">
            <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-2">
              <Tooltip>
                <Button
                  isIconOnly
                  aria-label={t("content_task.back_to_hub")}
                  onPress={() => navigate(ROUTES.content)}
                  variant={"ghost"}
                  className={"rounded-full"}
                >
                  <FaArrowLeft size={18} />
                </Button>
                <Tooltip.Content>{t("content_task.back_to_hub")}</Tooltip.Content>
              </Tooltip>
              <PageHeader title={t("contentpage.worlds_list")} />
              <InstancePicker size="sm" />
              {roots.packageType !== "uwp" && !!currentVersionName && playerPicker()}
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Button
                onPress={() => {
                  if (currentWorldsPath) openDirectory(currentWorldsPath);
                }}
                isDisabled={!currentWorldsPath}
                variant={"secondary"}
                className={cn(
                  "rounded-full",
                  "bg-surface-secondary text-foreground dark:text-zinc-200 font-medium",
                )}
              >
                {<FaFolderOpen />}
                {t("common.open")}
              </Button>
              <Tooltip>
                <Button
                  isIconOnly
                  aria-label={t("common.select_mode")}
                  aria-pressed={selection.isSelectMode}
                  onPress={selection.toggleSelectMode}
                  variant={"secondary"}
                  className={cn(
                    "rounded-full",
                    selection.isSelectMode
                      ? "bg-accent/15 text-accent"
                      : "bg-surface-secondary text-foreground dark:text-zinc-200",
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
                    "bg-surface-secondary text-foreground dark:text-zinc-200",
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
          </div>
          <div className="flex flex-wrap gap-3 items-center justify-between">
            <TextField
              aria-label={t("common.search_placeholder") as string}
              className={cn(
                "group",
                COMPONENT_STYLES.input.mainWrapper,
                "min-w-0 flex-[1_1_16rem] max-w-md",
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
                    <button
                      type="button"
                      aria-label={t("content_task.clear_search")}
                      onClick={() => sort.setQuery("")}
                    >
                      <FaTimes />
                    </button>
                  )}
                </InputGroup.Suffix>
              </InputGroup>
            </TextField>

            <div className="flex flex-wrap items-center gap-3">
              <span className="text-sm text-muted dark:text-zinc-400">
                {t("contentpage.isolation")}:{" "}
                <span className="font-medium text-foreground dark:text-zinc-200 bg-surface-secondary px-2 py-0.5 rounded-md">
                  {roots.isIsolation ? t("common.yes") : t("common.no")}
                </span>
              </span>
              <Dropdown>
                <Button
                  variant={"secondary"}
                  className={cn(
                    "rounded-full",
                    "bg-surface-secondary text-foreground dark:text-zinc-200 font-medium",
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
        isTransferDisabled={needsPlayer || selection.selectedCount === 0 || transferring}
      />

      {(loadError || playerLoadError) && (
        <ModalNotice role="alert" tone="danger">
          <p>{t("audit.usability.content_load_failed")}</p>
          <p className="mt-1 text-xs select-text">{playerLoadError || loadError}</p>
          <Button className="mt-3" variant="secondary" size="sm" isDisabled={loading || playersLoading} onPress={() => {
            setPlayerRefresh((value) => value + 1);
            void refreshAll();
          }}>{t("common.retry")}</Button>
        </ModalNotice>
      )}
      {failedWorlds.length > 0 && (
        <ModalNotice role="alert" tone="warning">
          <p>{t("audit.usability.worlds_partial", { count: failedWorlds.length })}</p>
          <p className="mt-1 text-xs select-text">{failedWorlds.join("、")}</p>
          <Button className="mt-3" variant="secondary" size="sm" isDisabled={loading} onPress={() => void refreshAll()}>{t("common.retry")}</Button>
        </ModalNotice>
      )}
      {!currentVersionName ? (
        // Resolve the missing instance in place instead of redirecting.
        <div role="status" className="flex flex-col items-center gap-3 py-12 text-center text-muted">
          <p>{t("content_task.choose_instance_hint")}</p>
          <InstancePicker />
        </div>
      ) : loading || playersLoading ? (
        <div className="flex flex-col items-center justify-center py-16 gap-4">
          <Spinner size="lg" />
          <span className="text-muted dark:text-zinc-400">
            {t("common.loading")}
          </span>
        </div>
      ) : loadError || playerLoadError ? null : needsPlayer ? (
        <div role="status" className="flex flex-col items-center gap-3 py-12 text-center text-muted">
          <p className="max-w-prose">{t("audit.usability.choose_player")}</p>
          {players.length > 0 && playerPicker("w-64")}
        </div>
      ) : sort.filtered.length === 0 && failedWorlds.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-muted">
          <FaBox className="text-6xl mb-4 opacity-20" />
          <p>
            {sort.query ? t("common.no_results") : t("contentpage.no_items")}
          </p>
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          {/* Auto-fill columns keep rows readable on wide screens. */}
          {/* Tailwind source for LAYOUT.GRID.AUTO(30): grid-cols-[repeat(auto-fill,minmax(min(100%,30rem),1fr))] */}
          <div className={LAYOUT.GRID.AUTO(30)}>
            {sort.paginatedItems.map((w, idx) => (
              <motion.div
                key={`${w.Path}-${idx}`}
                className="@container min-w-0"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                transition={{ duration: 0.2 }}
              >
                <div
                  className={cn(
                    COMPONENT_STYLES.contentListItem,
                    "h-full w-full p-3 @md:p-4 flex flex-col @xs:flex-row gap-3 @md:gap-4 group relative overflow-hidden cursor-pointer",
                    selection.isSelectMode && selection.selected[w.Path]
                      ? "ring-2 ring-accent bg-accent/5"
                      : "",
                  )}
                  onClick={() => {
                    if (selection.isSelectMode) selection.toggleSelect(w.Path);
                    else openWorldEditor(w);
                  }}
                >
                  <div className="relative shrink-0">
                    <div className="w-full @xs:w-auto @xs:h-20 @md:h-24 aspect-video rounded-2xl bg-surface-secondary/50 flex items-center justify-center overflow-hidden shadow-sm group-hover:shadow-md transition-shadow">
                      <img
                        src={w.IconBase64 || DefaultWorldPreview}
                        alt=""
                        className="w-full h-full object-cover object-center"
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
                                "bg-surface shadow-lg scale-110"
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

                  <div className="flex flex-col flex-1 min-w-0 gap-2">
                    <h3
                      className="text-base @md:text-lg font-bold text-foreground dark:text-white truncate"
                      title={w.FolderName}
                    >
                      {w.FolderName}
                    </h3>

                    <div className="flex flex-wrap items-end justify-between gap-2 mt-auto">
                      <div className="flex min-w-0 flex-wrap items-center gap-2 text-xs text-muted dark:text-zinc-500">
                        <div className="flex items-center gap-1.5 bg-surface-secondary/50 px-2 py-1 rounded-lg">
                          <FaHdd className="text-muted" />
                          <span>{formatBytes(w.Size)}</span>
                        </div>
                        <div className="flex items-center gap-1.5 bg-surface-secondary/50 px-2 py-1 rounded-lg">
                          <FaClock className="text-muted" />
                          <span>
                            {new Date(w.LastModified * 1000).toLocaleString()}
                          </span>
                        </div>
                      </div>

                      <div className="flex shrink-0 gap-1.5 ml-auto">
                        <Tooltip>
                          <Button
                            isIconOnly
                            aria-label={t("common.edit")}
                            size="sm"
                            variant={"secondary"}
                            onClick={(event) => event.stopPropagation()}
                            onPress={() => openWorldEditor(w)}
                            className={cn(
                              "rounded-full",
                              "bg-surface-secondary hover:bg-surface-tertiary text-foreground dark:text-zinc-200",
                            )}
                          >
                            <FaEdit size={14} />
                          </Button>
                          <Tooltip.Content>{t("common.edit")}</Tooltip.Content>
                        </Tooltip>
                        <Tooltip>
                          <Button
                            isIconOnly
                            aria-label={t("common.open")}
                            size="sm"
                            variant={"secondary"}
                            onClick={(event) => event.stopPropagation()}
                            onPress={() => {
                              openDirectory(w.Path);
                            }}
                            className={cn(
                              "rounded-full",
                              "bg-surface-secondary hover:bg-surface-tertiary text-foreground dark:text-zinc-200",
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
                            onPress={() => {
                              handleBackup(w);
                            }}
                            className={cn(
                              "rounded-full",
                              "bg-surface-secondary hover:bg-surface-tertiary text-foreground dark:text-zinc-200",
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
                            aria-label={t("common.delete")}
                            size="sm"
                            variant={"danger-soft"}
                            onClick={(event) => event.stopPropagation()}
                            onPress={() => {
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

      <ContentTaskDialog task={transferTask} />

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
