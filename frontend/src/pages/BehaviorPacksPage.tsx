import { EMPTY_CONTENT_ROOTS } from "@/utils/content";
import { openDirectory } from "@/utils/explorer";
import { ModalDescription, ModalPanel, ModalProgress } from "@/components/ModalPrimitives";
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

import React from "react";
import { useTranslation } from "react-i18next";

import { deleteContentItems } from "@/utils/contentDeletion";
import { DeleteConfirmModal } from "@/components/DeleteConfirmModal";
import { UnifiedModal } from "@/components/UnifiedModal";
import { motion } from "framer-motion";
import { useNavigate } from "react-router-dom";
import {
  FaSync,
  FaFolderOpen,
  FaSortAmountDown,
  FaSortAmountUp,
  FaFilter,
  FaTrash,
  FaBox,
  FaCheckSquare,
  FaTimes,
  FaClock,
  FaHdd,
  FaTag,
  FaExchangeAlt
} from "react-icons/fa";
import {
  GetContentRoots,
  ListPacksForVersion,
  DeletePack,
  GetPackInfo,
  TransferPackToVersion,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/contentservice";
import * as types from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import {
  GetVersionLogoDataUrl,
  ListVersionMetas,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/versionservice";
import { readCurrentVersionName } from "@/utils/currentVersion";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import { renderMcText } from "@/utils/mcformat";
import { PageHeader } from "@/components/PageHeader";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { cn } from "@/utils/cn";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { useScrollManager } from "@/hooks/useScrollManager";
import { useSelectionMode } from "@/hooks/useSelectionMode";
import { useContentSort } from "@/hooks/useContentSort";
import { formatBytes } from "@/utils/formatting";
import { compareVersions } from "@/utils/version";
import { ImportResultModal } from "@/components/ImportResultModal";
import { getPathBaseName } from "@/utils/fs";

const getNameFn = (p: any) => String(p.name || getPathBaseName(p.path) || "");
const getTimeFn = (p: any) => Number(p.modTime || 0);
type TransferTargetVersion = {
  name: string;
  gameVersion: string;
  type: string;
  icon?: string;
};

import { SelectionBar } from "@/components/SelectionBar";

export default function BehaviorPacksPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const hasBackend = minecraft !== undefined;
  const [loading, setLoading] = React.useState<boolean>(true);
  const [error, setError] = React.useState<string>("");
  const [currentVersionName, setCurrentVersionName] =
    React.useState<string>("");
  const [roots, setRoots] = React.useState<types.ContentRoots>(EMPTY_CONTENT_ROOTS);
  const [packs, setPacks] = React.useState<any[]>([]);
  const [activePack, setActivePack] = React.useState<any | null>(null);

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
  const {
    isOpen: dupOpen,
    open: dupOnOpen,
    close: dupOnClose,
    setOpen: dupOnOpenChange,
  } = useOverlayState();

  const [deletingOne, setDeletingOne] = React.useState<boolean>(false);
  const [deletingMany, setDeletingMany] = React.useState<boolean>(false);
  const [transferring, setTransferring] = React.useState<boolean>(false);
  const [currentTransferItem, setCurrentTransferItem] =
    React.useState<string>("");
  const [transferTargets, setTransferTargets] = React.useState<
    TransferTargetVersion[]
  >([]);
  const [selectedTransferTargets, setSelectedTransferTargets] = React.useState<
    string[]
  >([]);
  const [transferResultSuccess, setTransferResultSuccess] = React.useState<
    string[]
  >([]);
  const [transferResultFailed, setTransferResultFailed] = React.useState<
    Array<{ name: string; err: string }>
  >([]);
  const dupResolveRef = React.useRef<((overwrite: boolean) => void) | null>(
    null,
  );
  const dupNameRef = React.useRef<string>("");

  const scrollRef = React.useRef<HTMLDivElement | null>(null);
  const sort = useContentSort(
    "content.behavior.sort",
    packs,
    getNameFn,
    getTimeFn,
  );
  const { lastScrollTopRef, restorePendingRef } = useScrollManager(
    scrollRef,
    [packs],
    [sort.currentPage],
  );
  const contentScope = currentVersionName;
  const contentScopeRef = React.useRef(contentScope);
  contentScopeRef.current = contentScope;
  const selection = useSelectionMode(sort.filtered, (item) => item.path, currentVersionName, packs);

  const refreshAll = React.useCallback(
    async (silent?: boolean) => {
      if (!silent) setLoading(true);
      setError("");
      const name = readCurrentVersionName();
      setCurrentVersionName(name);
      try {
        if (!hasBackend || !name) {
          setRoots(EMPTY_CONTENT_ROOTS);
          setPacks([]);
        } else {
          const [r, allPacks] = await Promise.all([
            GetContentRoots(name),
            ListPacksForVersion(name, ""),
          ]);
          const safe = r || EMPTY_CONTENT_ROOTS;
          setRoots(safe);

          const filtered = (allPacks || []).filter(
            (p) => p.manifest.pack_type === 4,
          );

          const basic = await Promise.all(
            filtered.map(async (p) => {
              try {
                const info = await GetPackInfo(p.path);
                return { ...info, path: p.path };
              } catch {
                return {
                  name: p.manifest.name,
                  description: p.manifest.description,
                  version: p.manifest.identity.version
                    ? `${p.manifest.identity.version.major}.${p.manifest.identity.version.minor}.${p.manifest.identity.version.patch}`
                    : "",
                  minEngineVersion: "",
                  iconDataUrl: "",
                  path: p.path,
                };
              }
            }),
          );

          const withTime = await Promise.all(
            basic.map(async (p: any) => {
              let modTime = 0;
              try {
                if (typeof (minecraft as any).GetPathModTime === "function") {
                  modTime = await (minecraft as any).GetPathModTime(p.path);
                }
              } catch {}
              return { ...p, modTime };
            }),
          );
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
        }
      } catch (e) {
        setError(t("contentpage.error_resolve_paths") as string);
      } finally {
        if (!silent) setLoading(false);
      }
    },
    [hasBackend, t],
  );

  React.useEffect(() => {
    refreshAll();
  }, []);

  const handleDelete = async () => {
    if (!activePack) return;
    setDeletingOne(true);
    try {
      const result = await DeletePack(currentVersionName, activePack.path);
      if (result) throw new Error(result);
      toast(t("common.success"), { variant: "success", timeout: 2000 });
      if (contentScopeRef.current === contentScope) refreshAll(true);
      delOnClose();
    } catch (e) {
      toast("Error", {
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
      return await deleteContentItems(targets, (path) => DeletePack(currentVersionName, path), selection.retainSelection, () => refreshAll(), t, () => contentScopeRef.current === contentScope);
    } finally {
      setDeletingMany(false);
    }
  };

  const openTransferTargetModal = React.useCallback(async () => {
    if (transferring || selection.selectedCount === 0) return;

    const sourceVersionName = currentVersionName || readCurrentVersionName();
    if (!sourceVersionName) {
      toast(t("launcherpage.currentVersion_none") as string, {
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
            (m.packageType === "uwp" ? roots.packageType !== "uwp" || (m.type === "preview") !== roots.isPreview : m.enableIsolation) &&
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
    currentVersionName,
    selection.selectedCount,
    t,
    transferring,
    transferTargetOnOpen,
    roots.packageType,
    roots.isPreview,
  ]);

  const transferSelectedPacksToTargets = React.useCallback(async () => {
    if (transferring) return;

    const sourceVersionName = currentVersionName || readCurrentVersionName();
    if (!sourceVersionName) {
      toast(t("launcherpage.currentVersion_none") as string, {
        variant: "danger",
        timeout: 2000,
      });
      return;
    }

    const selectedPaths = selection.getSelectedKeys().filter(Boolean);
    const targetNames = selectedTransferTargets.filter(Boolean);
    if (selectedPaths.length === 0 || targetNames.length === 0) return;

    transferTargetOnClose();

    const packNameMap = new Map<string, string>(
      packs.map((pack: any) => {
        const path = String(pack?.path || "");
        const fallbackName = getPathBaseName(path);
        const displayName = String(pack?.name || fallbackName);
        return [path, displayName];
      }),
    );

    const succFiles: string[] = [];
    const errPairs: Array<{ name: string; err: string }> = [];

    try {
      setTransferring(true);
      setCurrentTransferItem("");

      for (const targetName of targetNames) {
        for (const packPath of selectedPaths) {
          const fallbackName = getPathBaseName(packPath);
          const packName = packNameMap.get(packPath) || fallbackName;
          const itemLabel = `${packName} -> ${targetName}`;
          setCurrentTransferItem(itemLabel);

          let err = await TransferPackToVersion(
            sourceVersionName,
            packPath,
            targetName,
            false,
          );
          if (err) {
            if (
              String(err) === "ERR_DUPLICATE_FOLDER" ||
              String(err) === "ERR_DUPLICATE_UUID"
            ) {
              dupNameRef.current = itemLabel;
              await new Promise<void>((resolve) => setTimeout(resolve, 0));
              dupOnOpen();
              const ok = await new Promise<boolean>((resolve) => {
                dupResolveRef.current = resolve;
              });
              if (ok) {
                err = await TransferPackToVersion(
                  sourceVersionName,
                  packPath,
                  targetName,
                  true,
                );
                if (!err) {
                  succFiles.push(itemLabel);
                  continue;
                }
              } else {
                continue;
              }
            }
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
        await refreshAll(true);
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
    currentVersionName,
    selectedTransferTargets,
    selection,
    packs,
    t,
    transferring,
    refreshAll,
    transferTargetOnClose,
    transferResultOnOpen,
    dupOnOpen,
  ]);

  return (
    <PageContainer ref={scrollRef}>
      <Card className={LAYOUT.GLASS_CARD.BASE}>
        <Card.Content className="p-6 flex flex-col gap-6">
          <PageHeader
            title={t("contentpage.behavior_packs")}
            endContent={
              <div className="flex items-center gap-2">
                <Button
                  onPress={() => openDirectory(roots.behaviorPacks)}
                  isDisabled={!roots.behaviorPacks}
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
                    onPress={selection.toggleSelectMode}
                    variant={"secondary"}
                    className={cn(
                      "rounded-full",
                      "bg-surface-secondary text-foreground dark:text-zinc-200",
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
            }
          />
          <div className="flex flex-col md:flex-row gap-4 items-end md:items-center justify-between">
            <TextField
              aria-label={t("common.search_placeholder")}
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
                  placeholder={t("common.search_placeholder")}
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
                    "min-w-[120px] bg-surface-secondary text-foreground dark:text-zinc-200 font-medium",
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
                      sort.setSortKey(k as "name" | "time");
                      sort.setSortAsc(order === "asc");
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
            <span className="font-medium text-foreground dark:text-zinc-200 bg-surface-secondary px-2 py-0.5 rounded-md">
              {currentVersionName || t("contentpage.none")}
            </span>
            <span className="text-muted dark:text-zinc-700">|</span>
            <span>{t("contentpage.isolation")}:</span>
            <span className="font-medium text-foreground dark:text-zinc-200 bg-surface-secondary px-2 py-0.5 rounded-md">
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
          !hasBackend || selection.selectedCount === 0 || transferring
        }
      />

      {loading ? (
        <div className="flex flex-col items-center justify-center py-20 gap-4">
          <Spinner size="lg" />
          <span className="text-muted dark:text-zinc-400">
            {t("common.loading")}
          </span>
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          {sort.filtered.length ? (
            <div className="flex flex-col gap-3">
              {sort.paginatedItems.map((p: any, idx: number) => (
                <motion.div
                  key={`${p.path}-${idx}`}
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  transition={{ duration: 0.2 }}
                >
                  <div
                    className={cn(
                      COMPONENT_STYLES.contentListItem,
                      "w-full p-5 flex gap-5 group relative overflow-hidden",
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
                      <div className="w-24 h-24 sm:w-28 sm:h-28 rounded-2xl bg-surface-secondary/50 flex items-center justify-center overflow-hidden shadow-sm group-hover:shadow-md transition-shadow">
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
                          <div className="flex flex-col items-center gap-2">
                            <FaBox className="text-4xl text-muted" />
                            <span className="text-[10px] text-muted font-medium uppercase tracking-wider">
                              No Icon
                            </span>
                          </div>
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
                      <div className="flex justify-between items-start gap-2 mb-1">
                        <h3
                          className="text-lg font-bold text-foreground dark:text-white truncate"
                          title={p.name}
                        >
                          {renderMcText(p.name || getPathBaseName(p.path))}
                        </h3>
                      </div>

                      <p
                        className="text-sm text-muted dark:text-zinc-400 line-clamp-2 w-full mb-3"
                        title={p.description}
                      >
                        {renderMcText(p.description || "")}
                      </p>

                      <div className="flex items-end justify-between mt-auto">
                        <div className="flex flex-wrap items-center gap-4 text-xs text-muted dark:text-zinc-500">
                          <div className="flex items-center gap-1.5 bg-surface-secondary/50 px-2 py-1 rounded-lg">
                            <FaHdd className="text-muted" />
                            <span>{formatBytes(p.size)}</span>
                          </div>
                          <div className="flex items-center gap-1.5 bg-surface-secondary/50 px-2 py-1 rounded-lg">
                            <FaClock className="text-muted" />
                            <span>{new Date(p.modTime).toLocaleString()}</span>
                          </div>
                          {p.version && (
                            <div className="flex items-center gap-1.5 bg-surface-secondary/50 px-2 py-1 rounded-lg">
                              <FaTag className="text-muted" />
                              <span>v{p.version}</span>
                            </div>
                          )}
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
                              onPress={(e) => {
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
                  : t("contentpage.no_behavior_packs")}
              </p>
            </div>
          )}

          {sort.totalPages > 1 && (
            <div className="relative h-12">
              <div className="absolute left-1/2 -translate-x-1/2 bottom-0 w-full flex justify-center">
                <PagePagination
                  size="sm"
                  pageCount={sort.totalPages}
                  currentPage={sort.currentPage}
                  onPageChange={sort.setCurrentPage}
                />
              </div>
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
          name: activePack?.name || "",
        })}
        itemName={activePack?.name}
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
        isPending={deletingMany}
        onConfirm={handleBatchDelete}
      />

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
        onConfirm={() => void transferSelectedPacksToTargets()}
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
        <ModalDescription>{t("mods.overwrite_modal_body")}</ModalDescription>
        {dupNameRef.current ? <ModalPanel className="font-mono">{dupNameRef.current}</ModalPanel> : null}
      </UnifiedModal>
    </PageContainer>
  );
}
