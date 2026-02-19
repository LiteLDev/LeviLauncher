import React from "react";
import { useTranslation } from "react-i18next";
import {
  Button,
  Spinner,
  Tooltip,
  useDisclosure,
  Card,
  CardBody,
  addToast,
  Checkbox,
  Image,
} from "@heroui/react";
import { DeleteConfirmModal } from "@/components/DeleteConfirmModal";
import { motion, AnimatePresence } from "framer-motion";
import { useLocation } from "react-router-dom";
import {
  FaSync,
  FaFolderOpen,
  FaTrash,
  FaCheckSquare,
  FaCamera,
  FaClock,
} from "react-icons/fa";
import {
  OpenPathDir,
  GetImageBase64,
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

interface ScreenshotItem {
  name: string;
  path: string;
  dir: string;
  captureTime: number;
  dataUrl?: string;
}

import { SelectionBar } from "@/components/SelectionBar";

export default function ScreenshotsPage() {
  const { t } = useTranslation();
  const location = useLocation();
  const hasBackend = minecraft !== undefined;
  const [loading, setLoading] = React.useState<boolean>(true);
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
    onOpen: delCfmOnOpen,
    onOpenChange: delCfmOnOpenChange,
  } = useDisclosure();
  const {
    isOpen: delManyCfmOpen,
    onOpen: delManyCfmOnOpen,
    onOpenChange: delManyCfmOnOpenChange,
  } = useDisclosure();
  const [deletingOne, setDeletingOne] = React.useState(false);
  const [deletingMany, setDeletingMany] = React.useState(false);
  const scrollRef = React.useRef<HTMLDivElement | null>(null);

  const player = (location.state as any)?.player || "";

  const selection = useSelectionMode(screenshots);

  const screenshotsRoot = React.useMemo(() => {
    if (!roots.usersRoot || !player) return "";
    return `${roots.usersRoot}\\${player}\\games\\com.mojang\\Screenshots`;
  }, [roots.usersRoot, player]);

  const refreshAll = React.useCallback(async () => {
    setLoading(true);
    const name = readCurrentVersionName();
    setCurrentVersionName(name);
    try {
      if (!hasBackend || !name || !player) {
        setScreenshots([]);
        return;
      }
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

      const list: any[] = await (contentService as any)?.ListScreenshots?.(
        name,
        player,
      );
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

      const limit = 4;
      for (let i = 0; i < items.length; i += limit) {
        const chunk = items.slice(i, i + limit);
        const urls = await Promise.all(
          chunk.map(async (item) => {
            try {
              return await GetImageBase64(item.path);
            } catch {
              return "";
            }
          }),
        );
        setScreenshots((prev) =>
          prev.map((s) => {
            const idx = chunk.findIndex((c) => c.path === s.path);
            if (idx >= 0 && urls[idx]) {
              return { ...s, dataUrl: urls[idx] };
            }
            return s;
          }),
        );
      }
    } catch {
      setScreenshots([]);
    } finally {
      setLoading(false);
    }
  }, [hasBackend, player]);

  React.useEffect(() => {
    refreshAll();
  }, []);

  return (
    <PageContainer ref={scrollRef}>
      <Card className={LAYOUT.GLASS_CARD.BASE}>
        <CardBody className="p-6 flex flex-col gap-6">
          <PageHeader
            title={t("contentpage.screenshots")}
            endContent={
              <div className="flex items-center gap-2">
                <Button
                  radius="full"
                  variant="flat"
                  startContent={<FaFolderOpen />}
                  onPress={() => {
                    if (screenshotsRoot) OpenPathDir(screenshotsRoot);
                  }}
                  isDisabled={!screenshotsRoot}
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

          <div className="mt-2 text-default-500 dark:text-zinc-400 text-sm flex flex-wrap items-center gap-2">
            <span>{t("contentpage.current_version")}:</span>
            <span className="font-medium text-default-700 dark:text-zinc-200 bg-default-100 dark:bg-zinc-800 px-2 py-0.5 rounded-md">
              {currentVersionName || t("contentpage.none")}
            </span>
            <span className="text-default-300 dark:text-zinc-700">|</span>
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
        totalCount={screenshots.length}
        onSelectAll={selection.selectAll}
        onDelete={delManyCfmOnOpen}
        isSelectMode={selection.isSelectMode}
      />

      {loading ? (
        <div className="flex flex-col items-center justify-center py-20 gap-4">
          <Spinner size="lg" />
          <span className="text-default-500 dark:text-zinc-400">
            {t("common.loading")}
          </span>
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
                    COMPONENT_STYLES.listItem,
                    "relative group cursor-pointer overflow-hidden rounded-2xl",
                    selection.isSelectMode && selection.selected[s.path]
                      ? "ring-2 ring-primary"
                      : "",
                  )}
                  onClick={() => {
                    if (selection.isSelectMode) selection.toggleSelect(s.path);
                  }}
                >
                  <div className="aspect-video bg-default-100/50 dark:bg-zinc-800/50 flex items-center justify-center overflow-hidden">
                    {s.dataUrl ? (
                      <Image
                        src={s.dataUrl}
                        alt={s.name}
                        className="w-full h-full object-cover"
                        radius="none"
                      />
                    ) : (
                      <FaCamera className="text-3xl text-default-300" />
                    )}
                  </div>

                  {selection.isSelectMode && (
                    <div className="absolute top-2 left-2 z-20">
                      <Checkbox
                        isSelected={!!selection.selected[s.path]}
                        onValueChange={() => selection.toggleSelect(s.path)}
                        classNames={{
                          wrapper:
                            "bg-white dark:bg-zinc-900 shadow-lg scale-110",
                        }}
                      />
                    </div>
                  )}

                  <div className="p-3 flex items-center justify-between">
                    <div className="flex items-center gap-1.5 text-xs text-default-400 dark:text-zinc-500 truncate">
                      <FaClock className="shrink-0" />
                      <span className="truncate">
                        {s.captureTime ? formatDate(s.captureTime) : s.name}
                      </span>
                    </div>
                    <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                      <Tooltip content={t("common.delete")}>
                        <Button
                          isIconOnly
                          size="sm"
                          color="danger"
                          variant="flat"
                          radius="lg"
                          onClick={(e) => {
                            e.stopPropagation();
                            setActiveShot(s);
                            delCfmOnOpen();
                          }}
                          className="bg-danger-50 hover:bg-danger-100 text-danger-500 dark:bg-danger-900/20 dark:hover:bg-danger-900/30"
                        >
                          <FaTrash size={12} />
                        </Button>
                      </Tooltip>
                    </div>
                  </div>
                </motion.div>
              ))}
            </div>
          ) : (
            <div className="flex flex-col items-center justify-center py-20 text-default-400 dark:text-zinc-500">
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
              await (contentService as any)?.DeleteScreenshot?.(
                currentVersionName,
                player,
                activeShot.path,
              );
              addToast({
                title: t("contentpage.deleted_name", {
                  name: activeShot.name,
                }),
                color: "success",
              });
              setActiveShot(null);
              refreshAll();
            } catch (e) {
              addToast({
                title: "Error",
                description: String(e),
                color: "danger",
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
        description={t("contentpage.delete_selected_confirm", {
          count: Object.values(selection.selected).filter(Boolean).length,
        })}
        warning={t("contentpage.delete_warning")}
        isPending={deletingMany}
        onConfirm={async () => {
          const targets = selection.getSelectedKeys();
          if (targets.length === 0) return;
          setDeletingMany(true);
          try {
            let success = 0;
            for (const p of targets) {
              try {
                await (contentService as any)?.DeleteScreenshot?.(
                  currentVersionName,
                  player,
                  p,
                );
                success++;
              } catch (e) {
                console.error(e);
              }
            }
            addToast({
              title: t("contentpage.deleted_count", { count: success }),
              color: "success",
            });
            selection.clearSelection();
            refreshAll();
          } finally {
            setDeletingMany(false);
          }
        }}
      />
    </PageContainer>
  );
}
