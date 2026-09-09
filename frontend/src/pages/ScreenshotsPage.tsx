import {
  Button,
  Card,
  Checkbox,
  Modal,
  Spinner,
  Tooltip,
  toast,
  useOverlayState,
} from "@heroui/react";

import React from "react";
import { useTranslation } from "react-i18next";

import { deleteContentItems } from "@/utils/contentDeletion";
import { DeleteConfirmModal } from "@/components/DeleteConfirmModal";
import { motion, AnimatePresence } from "framer-motion";
import { useLocation, useNavigate } from "react-router-dom";
import { ROUTES } from "@/constants/routes";
import {
  FaSync,
  FaFolderOpen,
  FaTrash,
  FaCheckSquare,
  FaCamera,
  FaClock,
  FaExpand,
  FaChevronLeft,
  FaChevronRight,
} from "react-icons/fa";
import {
  OpenPathDir,
  GetImageURL,
} from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { GetContentRoots } from "bindings/github.com/liteldev/LeviLauncher/contentservice";
import * as contentService from "bindings/github.com/liteldev/LeviLauncher/contentservice";
import * as types from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import { readCurrentVersionName } from "@/utils/currentVersion";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { PageHeader } from "@/components/PageHeader";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { cn } from "@/utils/cn";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { formatDate } from "@/utils/formatting";
import { useSelectionMode } from "@/hooks/useSelectionMode";
import {
  BaseModal,
  BaseModalBody,
  BaseModalFooter,
  BaseModalHeader,
} from "@/components/BaseModal";

interface ScreenshotItem {
  name: string;
  path: string;
  dir: string;
  captureTime: number;
  dataUrl?: string;
  imageError?: string;
}

import { SelectionBar } from "@/components/SelectionBar";

export default function ScreenshotsPage() {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const hasBackend = minecraft !== undefined;
  const [loading, setLoading] = React.useState<boolean>(true);
  const [loadError, setLoadError] = React.useState("");
  const loadGeneration = React.useRef(0);
  const [screenshots, setScreenshots] = React.useState<ScreenshotItem[]>([]);
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
  const [activeShot, setActiveShot] = React.useState<ScreenshotItem | null>(
    null,
  );
  const {
    isOpen: delCfmOpen,
    open: delCfmOnOpen,
    setOpen: delCfmOnOpenChange,
  } = useOverlayState();
  const {
    isOpen: previewOpen,
    open: previewOnOpen,
    setOpen: previewOnOpenChange,
    close: previewOnClose,
  } = useOverlayState();
  const {
    isOpen: delManyCfmOpen,
    open: delManyCfmOnOpen,
    setOpen: delManyCfmOnOpenChange,
  } = useOverlayState();
  const [deletingOne, setDeletingOne] = React.useState(false);
  const [deletingMany, setDeletingMany] = React.useState(false);
  const scrollRef = React.useRef<HTMLDivElement | null>(null);

  const player = (location.state as any)?.player || "";

  const contentScope = JSON.stringify([currentVersionName, player]);
  const contentScopeRef = React.useRef(contentScope);
  contentScopeRef.current = contentScope;
  const selection = useSelectionMode(screenshots, (item) => item.path, JSON.stringify([currentVersionName, player]), screenshots);
  const activeShotIndex = React.useMemo(
    () => screenshots.findIndex((shot) => shot.path === activeShot?.path),
    [activeShot, screenshots],
  );
  const previewShot = screenshots[activeShotIndex] || activeShot;
  const hasPrevShot = activeShotIndex > 0;
  const hasNextShot =
    activeShotIndex >= 0 && activeShotIndex < screenshots.length - 1;

  const screenshotsRoot = React.useMemo(() => {
    if (!roots.usersRoot || !player) return "";
    return `${roots.usersRoot}\\${player}\\games\\com.mojang\\Screenshots`;
  }, [roots.usersRoot, player]);

  const refreshAll = React.useCallback(async () => {
    const generation = ++loadGeneration.current;
    setLoading(true);
    setLoadError("");
    const name = readCurrentVersionName();
    setCurrentVersionName(name);
    try {
      if (!hasBackend || !name || !player) {
        setScreenshots([]);
        return;
      }
      const r = await GetContentRoots(name);
      if (generation !== loadGeneration.current) return;
      const safe = r || {
        base: "",
        usersRoot: "",
        resourcePacks: "",
        behaviorPacks: "",
        isIsolation: false,
        isPreview: false,
      };
      setRoots(safe);

      const list: any[] = await (contentService as any)?.ListScreenshots?.(
        name,
        player,
      );
      if (generation !== loadGeneration.current) return;
      if (!list || !Array.isArray(list)) {
        setScreenshots([]);
        return;
      }
      const items: ScreenshotItem[] = list.map((s: any) => ({
        name: s.name || "",
        path: s.path || "",
        dir: s.dir || "",
        captureTime: s.captureTime || 0,
      }));
      items.sort((a, b) => b.captureTime - a.captureTime);
      setScreenshots(items);
      // Metadata is enough to render the grid. Each image fills in independently.
      setLoading(false);

      const limit = 4;
      for (let i = 0; i < items.length; i += limit) {
        if (generation !== loadGeneration.current) return;
        const chunk = items.slice(i, i + limit);
        const urls = await Promise.all(
          chunk.map(async (item) => {
            try {
              const dataUrl = await GetImageURL(item.path);
              if (!dataUrl) throw new Error(t("common.load_failed"));
              return { dataUrl, imageError: "" };
            } catch (error) {
              return { dataUrl: "", imageError: error instanceof Error ? error.message : String(error) };
            }
          }),
        );
        if (generation !== loadGeneration.current) return;
        setScreenshots((prev) =>
          prev.map((s) => {
            const idx = chunk.findIndex((c) => c.path === s.path);
            if (idx >= 0 && urls[idx]) {
              return { ...s, ...urls[idx] };
            }
            return s;
          }),
        );
      }
    } catch (error) {
      if (generation !== loadGeneration.current) return;
      setScreenshots([]);
      setLoadError(error instanceof Error ? error.message : String(error));
    } finally {
      if (generation === loadGeneration.current) setLoading(false);
    }
  }, [hasBackend, player, t]);

  React.useEffect(() => {
    refreshAll();
    return () => { loadGeneration.current++; };
  }, [refreshAll]);

  const retryImage = React.useCallback(async (shot: ScreenshotItem) => {
    const generation = loadGeneration.current;
    setScreenshots((items) => items.map((item) => item.path === shot.path ? { ...item, imageError: "" } : item));
    try {
      const dataUrl = await GetImageURL(shot.path);
      if (!dataUrl) throw new Error(t("common.load_failed"));
      if (generation !== loadGeneration.current) return;
      setScreenshots((items) => items.map((item) => item.path === shot.path ? { ...item, dataUrl, imageError: "" } : item));
    } catch (error) {
      if (generation !== loadGeneration.current) return;
      setScreenshots((items) => items.map((item) => item.path === shot.path ? { ...item, imageError: error instanceof Error ? error.message : String(error) } : item));
    }
  }, [t]);

  React.useEffect(() => {
    if (!previewOpen) return;

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "ArrowLeft" && hasPrevShot) {
        setActiveShot(screenshots[activeShotIndex - 1]);
      }
      if (event.key === "ArrowRight" && hasNextShot) {
        setActiveShot(screenshots[activeShotIndex + 1]);
      }
    };

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [activeShotIndex, hasNextShot, hasPrevShot, previewOpen, screenshots]);

  const openPreview = React.useCallback(
    (shot: ScreenshotItem) => {
      setActiveShot(shot);
      previewOnOpen();
    },
    [previewOnOpen],
  );

  const movePreview = React.useCallback(
    (direction: "prev" | "next") => {
      if (activeShotIndex < 0) return;

      const targetIndex =
        direction === "prev" ? activeShotIndex - 1 : activeShotIndex + 1;

      if (targetIndex < 0 || targetIndex >= screenshots.length) return;
      setActiveShot(screenshots[targetIndex]);
    },
    [activeShotIndex, screenshots],
  );

  return (
    <PageContainer ref={scrollRef}>
      <Card className={LAYOUT.GLASS_CARD.BASE}>
        <Card.Content className="p-6 flex flex-col gap-6">
          <PageHeader
            title={t("contentpage.screenshots")}
            endContent={
              <div className="flex items-center gap-2">
                <Button
                  onPress={() => {
                    if (screenshotsRoot) OpenPathDir(screenshotsRoot);
                  }}
                  isDisabled={!screenshotsRoot}
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
          <div className="mt-2 text-muted dark:text-zinc-400 text-sm flex flex-wrap items-center gap-2">
            <span>{t("contentpage.current_version")}:</span>
            <span className="font-medium text-foreground dark:text-zinc-200 bg-surface-secondary dark:bg-zinc-800 px-2 py-0.5 rounded-md">
              {currentVersionName || t("contentpage.none")}
            </span>
            <span className="text-muted dark:text-zinc-700">|</span>
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
        totalCount={screenshots.length}
        onSelectAll={selection.selectAll}
        onDelete={delManyCfmOnOpen}
        isSelectMode={selection.isSelectMode}
      />

      {!player || (!loading && !currentVersionName) ? (
        <div role="status" className="flex flex-col items-center gap-4 py-16 text-muted">
          <p>{t(!player ? "contentpage.screenshot_select_player" : "contentpage.screenshot_no_instance")}</p>
          <Button variant="secondary" onPress={() => navigate(ROUTES.content)}>{t("common.back")}</Button>
        </div>
      ) : loading ? (
        <div className="flex flex-col items-center justify-center py-20 gap-4">
          <Spinner size="lg" />
          <span className="text-muted dark:text-zinc-400">
            {t("common.loading")}
          </span>
        </div>
      ) : loadError ? (
        <div role="alert" className="flex flex-col items-center gap-4 py-16">
          <p className="font-medium">{t("contentpage.screenshot_load_failed")}</p>
          <p className="text-sm text-muted select-text break-all">{loadError}</p>
          <div className="flex gap-2">
            <Button variant="primary" onPress={() => void refreshAll()}>{t("common.retry")}</Button>
            <Button variant="secondary" isDisabled={!screenshotsRoot} onPress={() => { if (screenshotsRoot) void OpenPathDir(screenshotsRoot); }}>{t("common.open")}</Button>
          </div>
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          {screenshots.length ? (
            <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4">
              {screenshots.map((s, idx) => (
                <motion.div
                  key={s.path}
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  transition={{ duration: 0.2 }}
                  className={cn(
                    COMPONENT_STYLES.contentListItem,
                    "relative group overflow-hidden rounded-2xl",
                    selection.isSelectMode
                      ? "cursor-pointer"
                      : "cursor-default",
                    selection.isSelectMode && selection.selected[s.path]
                      ? "ring-2 ring-accent"
                      : "",
                  )}
                  onClick={() => {
                    if (selection.isSelectMode) {
                      selection.toggleSelect(s.path);
                    }
                  }}
                >
                  <div className="relative aspect-video bg-surface-secondary/50 dark:bg-zinc-800/50 flex items-center justify-center overflow-hidden">
                    {s.dataUrl ? (
                      <>
                        <img
                          src={s.dataUrl}
                          alt={s.name}
                          loading="lazy"
                          decoding="async"
                          onError={() => setScreenshots((items) => items.map((item) => item.path === s.path ? { ...item, dataUrl: "", imageError: t("common.load_failed") } : item))}
                          className={cn(
                            "rounded-none",
                            "w-full h-full object-cover transition-transform duration-500 group-hover:scale-[1.03]",
                          )}
                        />
                        {!selection.isSelectMode && (
                          <div className="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-black/0 transition-all duration-300 group-hover:bg-black/25">
                            <Button
                              size="sm"
                              onPress={() => openPreview(s)}
                              variant={"secondary"}
                              className={cn(
                                "rounded-full",
                                "pointer-events-auto flex items-center gap-2 border border-white/20 bg-black/45 px-3 py-1.5 text-xs font-medium text-white opacity-0 shadow-lg backdrop-blur-md transition-all duration-300 group-hover:opacity-100 group-focus-within:opacity-100 hover:bg-black/55",
                              )}
                            >
                              <FaExpand size={12} />
                              <span>{t("contentpage.preview_screenshot")}</span>
                            </Button>
                          </div>
                        )}
                      </>
                    ) : s.imageError ? (
                      <div className="flex flex-col items-center gap-2 p-3 text-center">
                        <p className="text-xs text-danger" role="status">{t("contentpage.screenshot_image_failed", { name: s.name })}</p>
                        <p className="text-xs text-muted line-clamp-2 select-text">{s.imageError}</p>
                        <Button size="sm" variant="secondary" onClick={(event) => event.stopPropagation()} onPress={() => void retryImage(s)}>{t("common.retry")}</Button>
                      </div>
                    ) : (
                      <div role="status" aria-label={t("common.loading")} className="w-full h-full animate-pulse bg-surface-tertiary flex items-center justify-center"><FaCamera className="text-3xl text-muted" /></div>
                    )}
                  </div>

                  {selection.isSelectMode && (
                    <div className="absolute top-2 left-2 z-20">
                      <Checkbox
                        aria-label={t("contentpage.select_item", { name: s.name })}
                        onClick={(event) => event.stopPropagation()}
                        isSelected={!!selection.selected[s.path]}
                        onChange={() => selection.toggleSelect(s.path)}
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

                  <div className="p-3 flex items-center justify-between">
                    <div className="flex items-center gap-1.5 text-xs text-muted dark:text-zinc-500 truncate">
                      <FaClock className="shrink-0" />
                      <span className="truncate">
                        {s.captureTime ? formatDate(s.captureTime) : s.name}
                      </span>
                    </div>
                    <div className="flex gap-1 opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 transition-opacity">
                      <Tooltip>
                        <Button
                          isIconOnly
                    aria-label={t("common.delete")}
                          size="sm"
                          variant={"danger-soft"}
                          onClick={(event) => event.stopPropagation()}
                          onPress={(e) => {
                            setActiveShot(s);
                            delCfmOnOpen();
                          }}
                          className={cn(
                            "rounded-lg",
                            "bg-rose-50 hover:bg-rose-100 text-rose-500 dark:bg-rose-900/20 dark:hover:bg-rose-900/30",
                          )}
                        >
                          <FaTrash size={12} />
                        </Button>
                        <Tooltip.Content>{t("common.delete")}</Tooltip.Content>
                      </Tooltip>
                    </div>
                  </div>
                </motion.div>
              ))}
            </div>
          ) : (
            <div className="flex flex-col items-center justify-center py-20 text-muted dark:text-zinc-500">
              <FaCamera className="text-6xl mb-4 opacity-20" />
              <p>{t("contentpage.no_screenshots")}</p>
            </div>
          )}
        </div>
      )}

      {/* Single Delete Modal */}
      <DeleteConfirmModal
        isOpen={delCfmOpen}
        onOpenChange={delCfmOnOpenChange}
        title={t("common.confirm_delete")}
        description={t("contentpage.delete_screenshot_confirm", {
          name: activeShot?.name,
        })}
        itemName={activeShot?.name}
        warning={t("contentpage.delete_warning")}
        isPending={deletingOne}
        onConfirm={async () => {
          if (activeShot) {
            setDeletingOne(true);
            try {
              const result = await (contentService as any)?.DeleteScreenshot?.(
                currentVersionName,
                player,
                activeShot.path,
              );
              if (result) throw new Error(String(result));
              toast(
                t("contentpage.deleted_name", {
                  name: activeShot.name,
                }),
                { variant: "success", timeout: 2000 },
              );
              setActiveShot(null);
              if (contentScopeRef.current === contentScope) refreshAll();
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
          }
        }}
      />

      {/* Batch Delete Modal */}
      <DeleteConfirmModal
        isOpen={delManyCfmOpen}
        onOpenChange={delManyCfmOnOpenChange}
        title={t("common.confirm_delete")}
        scopeLabel={t("contentpage.delete_scope", { instance: currentVersionName, player: player || t("contentpage.select_player") })}
        itemNames={screenshots.filter((item) => selection.selected[item.path]).map((item) => item.name || item.path)}
        confirmDisabled={selection.selectedCount === 0}
        description={t("contentpage.delete_selected_confirm", {
          count: Object.values(selection.selected).filter(Boolean).length,
        })}
        warning={t("contentpage.delete_warning")}
        isPending={deletingMany}
        onConfirm={async () => {
          const targets = selection.getSelectedKeys();
    if (!targets.length) return false;
    setDeletingMany(true);
    try {
      return await deleteContentItems(targets, (path) => contentService.DeleteScreenshot(currentVersionName, player, path), selection.retainSelection, () => refreshAll(), t, () => contentScopeRef.current === contentScope);
    } finally {
      setDeletingMany(false);
    }
        }}
      />

      <BaseModal
        isOpen={previewOpen}
        onOpenChange={previewOnOpenChange}
        size="5xl"
        hideCloseButton={true}
        isDismissable={true}
        scrollBehavior="normal"
        className={
          "w-[min(94vw,1040px)] max-w-[1040px] max-h-[calc(100vh-2.5rem)] overflow-hidden bg-white/80! dark:bg-zinc-900/80! backdrop-blur-2xl border-white/40! dark:border-zinc-700/50! shadow-2xl rounded-4xl"
        }
        containerClassName={"overflow-hidden"}
      >
        {({ close: onClose }) => (
          <>
            <BaseModalHeader className="gap-3 pb-2 pt-7 sm:pt-6">
              <div className="flex items-start justify-between gap-4 pr-2 sm:pr-0">
                <div className="min-w-0">
                  <Modal.Heading className="text-xl font-bold text-foreground dark:text-zinc-100 truncate">
                    {activeShot?.name || t("contentpage.screenshots")}
                  </Modal.Heading>
                  <div className="mt-1 flex flex-wrap items-center gap-2 text-sm text-muted dark:text-zinc-400">
                    <span>
                      {activeShot?.captureTime
                        ? formatDate(activeShot.captureTime)
                        : t("contentpage.screenshots")}
                    </span>
                    {activeShotIndex >= 0 && (
                      <>
                        <span className="text-muted dark:text-zinc-700">•</span>
                        <span>
                          {activeShotIndex + 1} / {screenshots.length}
                        </span>
                      </>
                    )}
                  </div>
                </div>
                <div className="shrink-0 rounded-full bg-brand-50/80 px-3 py-1 text-xs font-medium text-brand-600 dark:bg-brand-500/10 dark:text-brand-400">
                  {t("contentpage.screenshot_viewer_hint")}
                </div>
              </div>
            </BaseModalHeader>

            <BaseModalBody className="overflow-hidden px-4 py-3 sm:px-5">
              <div className="relative overflow-hidden rounded-[2rem] border border-black/5 bg-surface-secondary/40 dark:border-white/10 dark:bg-zinc-900/40">
                <div className="absolute inset-y-0 left-0 z-20 hidden items-center pl-3 sm:flex sm:pl-4">
                  <Button
                    isIconOnly
                    aria-label={t("contentpage.previous_screenshot")}
                    onPress={() => movePreview("prev")}
                    isDisabled={!hasPrevShot}
                    variant={"secondary"}
                    className={cn(
                      "rounded-full",
                      "bg-white/75 text-foreground shadow-lg backdrop-blur-md disabled:opacity-30 dark:bg-zinc-900/75 dark:text-zinc-100",
                    )}
                  >
                    <FaChevronLeft />
                  </Button>
                </div>

                <div className="absolute inset-y-0 right-0 z-20 hidden items-center pr-3 sm:flex sm:pr-4">
                  <Button
                    isIconOnly
                    aria-label={t("contentpage.next_screenshot")}
                    onPress={() => movePreview("next")}
                    isDisabled={!hasNextShot}
                    variant={"secondary"}
                    className={cn(
                      "rounded-full",
                      "bg-white/75 text-foreground shadow-lg backdrop-blur-md disabled:opacity-30 dark:bg-zinc-900/75 dark:text-zinc-100",
                    )}
                  >
                    <FaChevronRight />
                  </Button>
                </div>

                <AnimatePresence mode="wait">
                  <motion.div
                    key={activeShot?.path || "empty-preview"}
                    initial={{ opacity: 0, scale: 0.985 }}
                    animate={{ opacity: 1, scale: 1 }}
                    exit={{ opacity: 0, scale: 0.985 }}
                    transition={{ duration: 0.2, ease: "easeOut" }}
                    className="flex h-[clamp(260px,52vh,560px)] items-center justify-center p-3 sm:p-4 md:p-5"
                  >
                    {previewShot?.dataUrl ? (
                      <img
                        src={previewShot.dataUrl}
                        alt={previewShot.name}
                        className="max-h-full w-auto max-w-full rounded-[1.5rem] object-contain shadow-[0_18px_50px_rgba(0,0,0,0.18)]"
                      />
                    ) : previewShot?.imageError ? (
                      <div role="alert" className="flex flex-col items-center gap-3 p-4 text-center">
                        <p>{t("contentpage.screenshot_image_failed", { name: previewShot.name })}</p>
                        <p className="text-sm text-muted select-text">{previewShot.imageError}</p>
                        <Button variant="secondary" onPress={() => void retryImage(previewShot)}>{t("common.retry")}</Button>
                      </div>
                    ) : (
                      <div className="flex h-full w-full flex-col items-center justify-center gap-3 text-muted dark:text-zinc-500">
                        <Spinner aria-label={t("common.loading")} />
                        <span role="status">{t("common.loading")}</span>
                      </div>
                    )}
                  </motion.div>
                </AnimatePresence>
              </div>

              <div className="mt-3 flex items-center justify-center gap-2 sm:hidden">
                <Button
                  isIconOnly
                  aria-label={t("contentpage.previous_screenshot")}
                  onPress={() => movePreview("prev")}
                  isDisabled={!hasPrevShot}
                  variant={"secondary"}
                  className={cn(
                    "rounded-full",
                    "bg-surface-secondary/80 dark:bg-zinc-800/80 text-foreground dark:text-zinc-100",
                  )}
                >
                  <FaChevronLeft />
                </Button>
                <Button
                  isIconOnly
                  aria-label={t("contentpage.next_screenshot")}
                  onPress={() => movePreview("next")}
                  isDisabled={!hasNextShot}
                  variant={"secondary"}
                  className={cn(
                    "rounded-full",
                    "bg-surface-secondary/80 dark:bg-zinc-800/80 text-foreground dark:text-zinc-100",
                  )}
                >
                  <FaChevronRight />
                </Button>
              </div>
            </BaseModalBody>

            <BaseModalFooter className="flex flex-col items-stretch justify-between gap-3 overflow-hidden pt-2 sm:flex-row sm:items-center">
              <div className="text-sm text-muted dark:text-zinc-400">
                {t("contentpage.screenshot_viewer_nav_hint")}
              </div>
              <div className="flex items-center justify-end gap-2">
                <Button
                  onPress={() => {
                    if (activeShot?.dir) {
                      OpenPathDir(activeShot.dir);
                    } else if (screenshotsRoot) {
                      OpenPathDir(screenshotsRoot);
                    }
                  }}
                  variant={"secondary"}
                  className={cn(
                    "rounded-full",
                    "bg-surface-secondary/80 dark:bg-zinc-800/80 text-foreground dark:text-zinc-100",
                  )}
                >
                  {<FaFolderOpen />}
                  {t("common.open")}
                </Button>
                <Button
                  onPress={onClose}
                  variant={"primary"}
                  className={cn("rounded-full", "font-semibold shadow-lg")}
                >
                  {t("common.close")}
                </Button>
              </div>
            </BaseModalFooter>
          </>
        )}
      </BaseModal>
    </PageContainer>
  );
}
