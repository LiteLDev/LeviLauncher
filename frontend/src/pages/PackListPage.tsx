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

import { deleteContentItems } from "@/utils/contentDeletion";
import { DeleteConfirmModal } from "@/components/DeleteConfirmModal";
import { motion } from "framer-motion";
import {
  FaSync,
  FaFolderOpen,
  FaTrash,
  FaBox,
  FaCheckSquare,
  FaClock,
  FaHdd,
  FaTag,
} from "react-icons/fa";
import { DeletePack } from "bindings/github.com/liteldev/LeviLauncher/internal/app/contentservice";
import { PackType } from "bindings/github.com/liteldev/LeviLauncher/internal/packages/models";
import { usePackList, type ListedPack } from "@/hooks/usePackList";
import { usePackTransfer } from "@/hooks/usePackTransfer";
import { renderMcText } from "@/utils/mcformat";
import { PageContainer } from "@/components/PageContainer";
import {
  ContentPrerequisite,
  ContentSearchField,
  ContentSortMenu,
  ContentSubpageHeader,
} from "@/components/ContentSubpage";
import { InstancePicker } from "@/components/InstancePicker";
import { LAYOUT } from "@/constants/layout";
import { cn } from "@/utils/cn";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { useScrollManager } from "@/hooks/useScrollManager";
import { useSelectionMode } from "@/hooks/useSelectionMode";
import { useContentSort } from "@/hooks/useContentSort";
import { formatBytes, formatDate } from "@/utils/formatting";
import { notify } from "@/utils/activity";
import { SelectionBar } from "@/components/SelectionBar";
import { getPathBaseName } from "@/utils/fs";

const getNameFn = (p: ListedPack) => p.name || getPathBaseName(p.path);
const getTimeFn = (p: ListedPack) => p.modTime;

export default function PackListPage({ kind }: { kind: "resource" | "behavior" }) {
  const { t } = useTranslation();
  const isResource = kind === "resource";
  const packType = isResource ? PackType.PackTypeResources : PackType.PackTypeBehavior;
  const { loading, currentVersionName, roots, packs, refreshAll } = usePackList(packType);
  const directory = isResource ? roots.resourcePacks : roots.behaviorPacks;
  const MissingIcon = isResource ? FaFolderOpen : FaBox;
  const [activePack, setActivePack] = React.useState<ListedPack | null>(null);
  const activePackName = activePack?.name || (isResource ? activePack?.path : undefined);
  const deleteWarning = isResource ? t("contentpage.delete_warning") : undefined;

  const {
    isOpen: delOpen,
    open: delOnOpen,
    setOpen: delOnOpenChange,
  } = useOverlayState();

  const {
    isOpen: delManyCfmOpen,
    open: delManyCfmOnOpen,
    setOpen: delManyCfmOnOpenChange,
  } = useOverlayState();

  const [deletingOne, setDeletingOne] = React.useState<boolean>(false);
  const [deletingMany, setDeletingMany] = React.useState<boolean>(false);

  const scrollRef = React.useRef<HTMLDivElement | null>(null);
  const sort = useContentSort(
    `content.${kind}.sort`,
    packs,
    getNameFn,
    getTimeFn,
  );
  useScrollManager(
    scrollRef,
    [packs],
    [sort.currentPage],
  );
  const contentScope = currentVersionName;
  const contentScopeRef = React.useRef(contentScope);
  contentScopeRef.current = contentScope;
  const selection = useSelectionMode(sort.filtered, (item) => item.path, currentVersionName, packs);

  const packNames = React.useMemo(
    () => new Map(packs.map((pack) => [pack.path, getNameFn(pack)])),
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

  const handleDelete = async () => {
    if (!activePack) return;
    setDeletingOne(true);
    try {
      const result = await DeletePack(currentVersionName, activePack.path);
      if (result) throw new Error(result);
      notify({
        title: isResource ? t("contentpage.deleted_name", { name: activePack.name }) : t("common.success"),
        variant: "success",
        source: "content",
      });
      setActivePack(null);
      if (contentScopeRef.current === contentScope) void refreshAll(!isResource);
    } catch (e) {
      notify({
        title: t("content_sub.delete_failed"),
        description: String(e),
        variant: "danger",
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
      return await deleteContentItems(
        targets,
        (path) => DeletePack(currentVersionName, path),
        selection.retainSelection,
        () => refreshAll(),
        t,
        () => contentScopeRef.current === contentScope,
      );
    } finally {
      setDeletingMany(false);
    }
  };

  return (
    <PageContainer ref={scrollRef}>
      <ContentSubpageHeader
        title={isResource ? t("contentpage.resource_packs") : t("contentpage.behavior_packs")}
        isIsolation={currentVersionName ? roots.isIsolation : undefined}
        actions={[
          {
            key: "open",
            label: t("common.open_folder"),
            icon: <FaFolderOpen />,
            onPress: () => openDirectory(directory),
            isDisabled: !directory,
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
            icon: <FaSync className={loading ? "animate-spin" : ""} />,
            onPress: () => void refreshAll(),
            isDisabled: loading || !currentVersionName,
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
        onTransfer={() => void transfer.open()}
        isTransferDisabled={
          selection.selectedCount === 0 || transfer.transferring
        }
      />

      {!currentVersionName ? (
        <ContentPrerequisite
          icon={<MissingIcon />}
          title={t("content_sub.no_instance_title")}
          body={t("content_sub.no_instance_body")}
        >
          <InstancePicker />
        </ContentPrerequisite>
      ) : loading ? (
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
              {sort.paginatedItems.map((p, idx) => (
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
                          <MissingIcon aria-hidden="true" className="text-4xl text-muted" />
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
          ) : (
            <div className="flex flex-col items-center justify-center py-20 text-muted dark:text-zinc-500">
              <FaBox className="text-6xl mb-4 opacity-20" />
              <p>
                {sort.query
                  ? t("common.no_results")
                  : isResource ? t("contentpage.no_resource_packs") : t("contentpage.no_behavior_packs")}
              </p>
            </div>
          )}

          {sort.totalPages > 1 && (
            <div className="flex justify-center">
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
      {/* Delete Confirmation Modal */}
      <DeleteConfirmModal
        isOpen={delOpen}
        onOpenChange={delOnOpenChange}
        title={t("common.confirm_delete")}
        description={t("contentpage.delete_pack_confirm", {
          name: activePackName || "",
        })}
        itemName={activePackName}
        warning={deleteWarning}
        isPending={deletingOne}
        onConfirm={handleDelete}
      />

      {/* Batch Delete Confirmation Modal */}
      <DeleteConfirmModal
        isOpen={delManyCfmOpen}
        onOpenChange={delManyCfmOnOpenChange}
        title={t("common.confirm_delete")}
        itemNames={packs.filter((item) => selection.selected[item.path]).map((item) => item.name || item.path)}
        confirmDisabled={selection.selectedCount === 0}
        description={t("contentpage.delete_selected_confirm", {
          count: selection.selectedCount,
        })}
        warning={deleteWarning}
        isPending={deletingMany}
        onConfirm={handleBatchDelete}
      />

      {transfer.modal}
    </PageContainer>
  );
}
