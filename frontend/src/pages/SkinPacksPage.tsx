import { openDirectory } from "@/utils/explorer";
import { PagePagination } from "@/components/PagePagination";
import {
  Button,
  Checkbox,
  Spinner,
  Tooltip,
  useOverlayState
} from "@heroui/react";

import React from "react";
import { useTranslation } from "react-i18next";

import {
  FaSync,
  FaFolderOpen,
  FaCheckSquare,
  FaTrash, FaClock,
  FaBox,
  FaHdd,
  FaTag,
  FaUser,
  FaTshirt,
} from "react-icons/fa";
import { deleteContentItems } from "@/utils/contentDeletion";
import { DeleteConfirmModal } from "@/components/DeleteConfirmModal";
import { motion } from "framer-motion";
import {
  ListPackDetailsForVersion,
  DeletePack,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/contentservice";
import { PackType } from "bindings/github.com/liteldev/LeviLauncher/internal/packages/models";
import { compareVersions } from "@/utils/version";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import { renderMcText } from "@/utils/mcformat";
import { PageContainer } from "@/components/PageContainer";
import {
  ContentPrerequisite,
  ContentSearchField,
  ContentSortMenu,
  ContentSubpageHeader,
  useContentPlayers,
  useContentRoots,
} from "@/components/ContentSubpage";
import { InstancePicker } from "@/components/InstancePicker";
import { LAYOUT } from "@/constants/layout";
import { cn } from "@/utils/cn";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { useScrollManager } from "@/hooks/useScrollManager";
import { useSelectionMode } from "@/hooks/useSelectionMode";
import { useContentSort } from "@/hooks/useContentSort";
import { usePackTransfer } from "@/hooks/usePackTransfer";
import { formatBytes, formatDate } from "@/utils/formatting";
import { getPathBaseName } from "@/utils/fs";
import { notify } from "@/utils/activity";
import { useCurrentVersion } from "@/utils/CurrentVersionContext";
import { SelectionBar } from "@/components/SelectionBar";

const getNameFn = (p: any) => String(p.name || getPathBaseName(p.path) || "");
const getTimeFn = (p: any) => Number(p.modTime || 0);

export default function SkinPacksPage() {
  const { t } = useTranslation();
  const { currentVersionName } = useCurrentVersion();
  const [loading, setLoading] = React.useState<boolean>(true);
  const rootsState = useContentRoots(currentVersionName, true);
  const { roots } = rootsState;
  // Newer and UWP instances keep skin packs shared instead of per player.
  const isSharedMode =
    roots.packageType === "uwp" ||
    Boolean(rootsState.gameVersion && compareVersions(rootsState.gameVersion, "1.26.0") > 0);
  const playerState = useContentPlayers(
    rootsState.ready && !isSharedMode ? roots.usersRoot || "" : "",
  );
  const selectedPlayer = isSharedMode ? "" : playerState.selectedPlayer;
  const [packs, setPacks] = React.useState<any[]>([]);
  const packsLoadGeneration = React.useRef(0);
  const [activePack, setActivePack] = React.useState<any | null>(null);
  const {
    isOpen: delCfmOpen,
    open: delCfmOnOpen,
    setOpen: delCfmOnOpenChange,
  } = useOverlayState();
  const {
    isOpen: delManyCfmOpen,
    open: delManyCfmOnOpen,
    setOpen: delManyCfmOnOpenChange,
  } = useOverlayState();
  const [deletingOne, setDeletingOne] = React.useState<boolean>(false);
  const [deletingMany, setDeletingMany] = React.useState<boolean>(false);

  const scrollRef = React.useRef<HTMLDivElement | null>(null);
  const sort = useContentSort("content.skin.sort", packs, getNameFn, getTimeFn);
  const { lastScrollTopRef, restorePendingRef } = useScrollManager(
    scrollRef,
    [packs],
    [sort.currentPage],
  );
  const contentScope = JSON.stringify([currentVersionName, selectedPlayer]);
  const contentScopeRef = React.useRef(contentScope);
  contentScopeRef.current = contentScope;
  const selection = useSelectionMode(sort.filtered, (item) => item.path, contentScope, packs);
  React.useEffect(() => {
    delCfmOnOpenChange(false);
    delManyCfmOnOpenChange(false);
    setActivePack(null);
  }, [currentVersionName, selectedPlayer]);

  React.useEffect(() => {
    setPacks([]);
    setLoading(!!currentVersionName);
  }, [currentVersionName]);

  const ready = rootsState.ready && !playerState.loading;

  const refreshAll = React.useCallback(
    async (silent?: boolean) => {
      if (!ready) return;
      const generation = ++packsLoadGeneration.current;
      if (!silent) setLoading(true);
      const name = currentVersionName;
      try {
        if (!isSharedMode && !selectedPlayer) {
          setPacks([]);
          return;
        }
        const withTime = await ListPackDetailsForVersion(name, selectedPlayer || "", PackType.PackTypeSkins);
        if (generation !== packsLoadGeneration.current) return;
        setPacks(withTime);
        Promise.resolve()
          .then(async () => {
            const readCache = () => {
              try {
                return JSON.parse(
                  localStorage.getItem("content.size.cache") || "{}",
                );
              } catch {
                return {};
              }
            };
            const writeCache = (c: any) => {
              try {
                localStorage.setItem("content.size.cache", JSON.stringify(c));
              } catch {}
            };
            const cache = readCache();
            const limit = 4;
            const items = withTime.slice();
            for (let i = 0; i < items.length; i += limit) {
              if (generation !== packsLoadGeneration.current) return;
              const chunk = items.slice(i, i + limit);
              await Promise.all(
                chunk.map(async (p: any) => {
                  const key = p.path;
                  const c = cache[key];
                  if (
                    c &&
                    typeof c.size === "number" &&
                    Number(c.modTime || 0) === Number(p.modTime || 0)
                  ) {
                    setPacks((prev) =>
                      prev.map((it: any) =>
                        it.path === key ? { ...it, size: c.size } : it,
                      ),
                    );
                  } else {
                    let size = 0;
                    try {
                      if (
                        typeof (minecraft as any).GetPathSize === "function"
                      ) {
                        size = await (minecraft as any).GetPathSize(key);
                      }
                    } catch {}
                    if (generation !== packsLoadGeneration.current) return;
                    cache[key] = { modTime: p.modTime || 0, size };
                    setPacks((prev) =>
                      prev.map((it: any) =>
                        it.path === key ? { ...it, size } : it,
                      ),
                    );
                  }
                }),
              );
              writeCache(cache);
            }
          })
          .catch(() => {});
      } catch (e: any) {
        if (generation !== packsLoadGeneration.current) return;
        notify({
          title: t("contentpage.error_resolve_paths"),
          description: String(e),
          variant: "danger",
          source: "content",
        });
      } finally {
        if (generation === packsLoadGeneration.current) setLoading(false);
      }
    },
    [currentVersionName, isSharedMode, ready, selectedPlayer, t],
  );

  React.useEffect(() => {
    if (!ready) return;
    setPacks([]);
    void refreshAll();
    return () => { packsLoadGeneration.current++; };
  }, [ready, refreshAll]);

  const packNames = React.useMemo(
    () => new Map(packs.map((pack: any) => [String(pack?.path || ""), getNameFn(pack)])),
    [packs],
  );
  const transfer = usePackTransfer({
    sourceName: currentVersionName,
    roots,
    getSelectedPaths: selection.getSelectedKeys,
    nameOf: (path) => packNames.get(path) || getPathBaseName(path),
    onTransferred: () => {
      selection.clearSelection();
      return refreshAll(true);
    },
  });

  const openSkinFolder = async () => {
    if (!roots.resourcePacks) return;
    const dir = roots.resourcePacks.replace(/[\\/]resource_packs[\\/]?$/, "");
    const sep = roots.resourcePacks.includes("/") ? "/" : "\\";
    let sp = `${dir}${sep}skin_packs`;
    if (selectedPlayer && roots.usersRoot && !isSharedMode) {
      sp = `${roots.usersRoot}\\${selectedPlayer}\\games\\com.mojang\\skin_packs`;
    }
    await openDirectory(sp);
  };

  const needsPlayer =
    ready && !isSharedMode && !selectedPlayer;

  return (
    <PageContainer ref={scrollRef}>
      <ContentSubpageHeader
        title={t("contentpage.skin_packs")}
        isIsolation={currentVersionName ? roots.isIsolation : undefined}
        player={
          currentVersionName && !isSharedMode
            ? {
                players: playerState.players,
                value: selectedPlayer,
                gamertagMap: playerState.gamertagMap,
                onChange: playerState.setSelectedPlayer,
              }
            : null
        }
        actions={[
          {
            key: "open",
            label: t("common.open_folder"),
            icon: <FaFolderOpen />,
            onPress: () => void openSkinFolder(),
            isDisabled: !roots.resourcePacks,
          },
          {
            key: "select",
            label: t("common.select_mode"),
            icon: <FaCheckSquare />,
            onPress: selection.toggleSelectMode,
            isDisabled: !packs.length,
          },
          {
            key: "refresh",
            label: t("common.refresh"),
            icon: <FaSync className={loading && !rootsState.error ? "animate-spin" : ""} />,
            onPress: () => void refreshAll(),
            isDisabled: loading || !ready,
          },
        ]}
      >
        <ContentSearchField value={sort.query} onChange={sort.setQuery} />
        <ContentSortMenu
          sortKey={sort.sortKey}
          sortAsc={sort.sortAsc}
          onChange={(key, asc) => {
            sort.setSortKey(key);
            sort.setSortAsc(asc);
          }}
        />
      </ContentSubpageHeader>

      <SelectionBar
        hiddenSelectedCount={selection.hiddenSelectedCount}
        selectedCount={selection.selectedCount}
        totalCount={sort.filtered.length}
        onSelectAll={selection.selectAll}
        onDelete={delManyCfmOnOpen}
        isSelectMode={selection.isSelectMode}
        onTransfer={isSharedMode ? () => void transfer.open() : undefined}
        isTransferDisabled={
          !isSharedMode ||
          selection.selectedCount === 0 ||
          transfer.transferring
        }
      />

      {!currentVersionName ? (
        <ContentPrerequisite
          icon={<FaTshirt />}
          title={t("content_sub.no_instance_title")}
          body={t("content_sub.no_instance_body")}
        >
          <InstancePicker />
        </ContentPrerequisite>
      ) : rootsState.error ? (
        <ContentPrerequisite
          icon={<FaTshirt />}
          title={t("contentpage.error_resolve_paths")}
          body={<span className="select-text break-all">{rootsState.error}</span>}
        >
          <Button variant="primary" onPress={rootsState.retry}>{t("common.retry")}</Button>
        </ContentPrerequisite>
      ) : needsPlayer ? (
        <ContentPrerequisite
          icon={<FaUser />}
          title={t("contentpage.no_players")}
          body={t("audit.usability.choose_player")}
        />
      ) : loading || !ready ? (
        <div className="flex flex-col items-center justify-center py-20 gap-4">
          <Spinner size="lg" />
          <span className="text-muted dark:text-zinc-400">
            {t("common.loading")}
          </span>
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          {sort.filtered.length ? (
            <div className={LAYOUT.GRID.AUTO(30)}>
              {sort.paginatedItems.map((p: any, idx: number) => (
                <motion.div
                  key={`${p.path}-${idx}`}
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  transition={{ duration: 0.2 }}
                  className="min-w-0"
                >
                  <div
                    className={cn(
                      COMPONENT_STYLES.contentListItem,
                      "w-full h-full p-4 flex gap-4 group relative overflow-hidden",
                      selection.isSelectMode ? "cursor-pointer" : "cursor-default",
                      selection.isSelectMode && selection.selected[p.path]
                        ? "ring-2 ring-accent bg-accent/5"
                        : "",
                    )}
                    onClick={() => {
                      if (selection.isSelectMode)
                        selection.toggleSelect(p.path);
                    }}
                  >
                    <div className="relative shrink-0">
                      <div className="size-20 sm:size-24 rounded-2xl bg-surface-secondary/50 flex items-center justify-center overflow-hidden shadow-sm group-hover:shadow-md transition-shadow">
                        {p.iconDataUrl ? (
                          <img
                            src={p.iconDataUrl}
                            alt={p.name || p.path}
                            className={cn(
                              "rounded-none",
                              "w-full h-full object-cover",
                            )}
                          />
                        ) : (
                          <FaTshirt aria-hidden="true" className="text-4xl text-muted" />
                        )}
                      </div>
                      {selection.isSelectMode && (
                        <div className="absolute -top-2 -left-2 z-20">
                          <Checkbox
                            aria-label={t("contentpage.select_item", { name: p.name })}
                            onClick={(event) => event.stopPropagation()}
                            isSelected={!!selection.selected[p.path]}
                            onChange={() => selection.toggleSelect(p.path)}
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

                    <div className="flex flex-col flex-1 min-w-0">
                      <h3
                        className="text-lg font-bold text-foreground dark:text-white truncate mb-1"
                        title={p.name}
                      >
                        {renderMcText(p.name || getPathBaseName(p.path))}
                      </h3>

                      <p
                        className="text-sm text-muted dark:text-zinc-400 line-clamp-2 w-full mb-3"
                        title={p.description}
                      >
                        {renderMcText(p.description || "")}
                      </p>

                      <div className="flex flex-wrap items-end justify-between gap-2 mt-auto">
                        <div className="flex min-w-0 flex-wrap items-center gap-2 text-xs text-muted dark:text-zinc-500">
                          <div className="flex items-center gap-1.5 bg-surface-secondary/50 px-2 py-1 rounded-lg">
                            <FaHdd className="text-muted" />
                            <span>{formatBytes(p.size)}</span>
                          </div>
                          <div className="flex items-center gap-1.5 bg-surface-secondary/50 px-2 py-1 rounded-lg">
                            <FaClock className="text-muted" />
                            <span>{formatDate(p.modTime)}</span>
                          </div>
                          {p.version && (
                            <div className="flex items-center gap-1.5 bg-surface-secondary/50 px-2 py-1 rounded-lg">
                              <FaTag className="text-muted" />
                              <span>v{p.version}</span>
                            </div>
                          )}
                        </div>

                        <div className="flex gap-2 ml-auto opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 transition-opacity">
                          <Tooltip>
                            <Button
                              isIconOnly
                              aria-label={t("common.open")}
                              size="sm"
                              variant={"secondary"}
                              onClick={(event) => event.stopPropagation()}
                              onPress={() => {
                                openDirectory(p.path);
                              }}
                              className={cn(
                                "rounded-full",
                                "bg-surface-secondary hover:bg-surface-tertiary text-foreground dark:text-zinc-200",
                              )}
                            >
                              <FaFolderOpen size={14} />
                            </Button>
                            <Tooltip.Content>
                              {t("common.open")}
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
                                setActivePack(p);
                                delCfmOnOpen();
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
          ) : (
            <div className="flex flex-col items-center justify-center py-20 text-muted dark:text-zinc-500">
              <FaBox className="text-6xl mb-4 opacity-20" />
              <p>
                {sort.query
                  ? t("common.no_results")
                  : t("contentpage.no_skin_packs")}
              </p>
            </div>
          )}

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

      <DeleteConfirmModal
        isOpen={delCfmOpen}
        onOpenChange={delCfmOnOpenChange}
        title={t("common.confirm_delete")}
        description={t("contentpage.delete_pack_confirm", {
          name: activePack?.name || activePack?.path,
        })}
        itemName={activePack?.name || activePack?.path}
        isPending={deletingOne}
        onConfirm={async () => {
          if (!activePack) return;
          const pos =
            scrollRef.current?.scrollTop ??
            (document.scrollingElement as any)?.scrollTop ??
            0;
          setDeletingOne(true);
          lastScrollTopRef.current = pos;
          restorePendingRef.current = true;
          try {
            const result = await DeletePack(currentVersionName, activePack.path);
            if (result) throw new Error(result);
            if (contentScopeRef.current === contentScope) refreshAll();
            notify({
              title: t("contentpage.deleted_name", { name: activePack.name }),
              variant: "success",
              source: "content",
            });
          } catch (err) {
            notify({
              title: t("content_sub.delete_failed"),
              description: String(err),
              variant: "danger",
              source: "content",
            });
            throw err;
          } finally {
            setDeletingOne(false);
          }
        }}
      />

      <DeleteConfirmModal
        isOpen={delManyCfmOpen}
        onOpenChange={delManyCfmOnOpenChange}
        title={t("common.confirm_delete")}
        scopeLabel={t("contentpage.delete_scope", { instance: currentVersionName, player: selectedPlayer || t("contentpage.select_player") })}
        itemNames={packs.filter((item) => selection.selected[item.path]).map((item) => item.name || item.path)}
        confirmDisabled={selection.selectedCount === 0}
        description={t("contentpage.delete_selected_confirm", {
          count: selection.selectedCount,
        })}
        isPending={deletingMany}
        onConfirm={async () => {
          const targets = selection.getSelectedKeys();
          if (!targets.length) return false;
          setDeletingMany(true);
          try {
            return await deleteContentItems(targets, (path) => DeletePack(currentVersionName, path), selection.retainSelection, () => refreshAll(), t, () => contentScopeRef.current === contentScope);
          } finally {
            setDeletingMany(false);
          }
        }}
      />

      {transfer.modal}
    </PageContainer>
  );
}
