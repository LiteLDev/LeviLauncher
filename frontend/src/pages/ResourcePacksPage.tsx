import React from "react";
import { useTranslation } from "react-i18next";
import {
  Button,
  Chip,
  Image,
  Spinner,
  Tooltip,
  useDisclosure,
  Input,
  Dropdown,
  DropdownTrigger,
  DropdownMenu,
  DropdownItem,
  Checkbox,
  Pagination,
  Card,
  CardBody,
  addToast,
} from "@heroui/react";
import { DeleteConfirmModal } from "@/components/DeleteConfirmModal";
import { motion, AnimatePresence } from "framer-motion";
import { useNavigate } from "react-router-dom";
import {
  FaArrowLeft,
  FaSync,
  FaFolderOpen,
  FaSortAmountDown,
  FaSortAmountUp,
  FaFilter,
  FaTrash,
  FaCheckSquare,
  FaBox,
  FaTimes,
  FaClock,
  FaHdd,
  FaTag,
} from "react-icons/fa";
import {
  GetContentRoots,
  OpenPathDir,
  ListPacksForVersion,
  DeletePack,
} from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import * as types from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import * as packages from "bindings/github.com/liteldev/LeviLauncher/internal/packages/models";
import { readCurrentVersionName } from "@/utils/currentVersion";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { renderMcText } from "@/utils/mcformat";
import { PageHeader } from "@/components/PageHeader";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { cn } from "@/utils/cn";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { useScrollManager } from "@/hooks/useScrollManager";
import { useSelectionMode } from "@/hooks/useSelectionMode";
import { useContentSort } from "@/hooks/useContentSort";
import { formatBytes, formatDate } from "@/utils/formatting";

const getNameFn = (p: any) =>
  String(p.name || p.path?.split("\\").pop() || "");
const getTimeFn = (p: any) => Number(p.modTime || 0);

export default function ResourcePacksPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const hasBackend = minecraft !== undefined;
  const [loading, setLoading] = React.useState<boolean>(true);
  const [error, setError] = React.useState<string>("");
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
  const [entries, setEntries] = React.useState<
    { name: string; path: string }[]
  >([]);
  const [packs, setPacks] = React.useState<any[]>([]);
  const [resultSuccess, setResultSuccess] = React.useState<string[]>([]);
  const [resultFailed, setResultFailed] = React.useState<
    Array<{ name: string; err: string }>
  >([]);
  const [activePack, setActivePack] = React.useState<any | null>(null);
  const {
    isOpen: delOpen,
    onOpen: delOnOpen,
    onOpenChange: delOnOpenChange,
  } = useDisclosure();
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
  const [deletingOne, setDeletingOne] = React.useState<boolean>(false);
  const [deletingMany, setDeletingMany] = React.useState<boolean>(false);

  const scrollRef = React.useRef<HTMLDivElement | null>(null);
  const sort = useContentSort("content.resource.sort", packs, getNameFn, getTimeFn);
  const { lastScrollTopRef, restorePendingRef } = useScrollManager(scrollRef, [packs], [sort.currentPage]);
  const selection = useSelectionMode(sort.filtered);

  const refreshAll = React.useCallback(
    async (silent?: boolean) => {
      if (!silent) setLoading(true);
      setError("");
      const name = readCurrentVersionName();
      setCurrentVersionName(name);
      try {
        if (!hasBackend || !name) {
          setRoots({
            base: "",
            usersRoot: "",
            resourcePacks: "",
            behaviorPacks: "",
            isIsolation: false,
            isPreview: false,
          });
          setEntries([]);
          setPacks([]);
        } else {
          const [r, allPacks] = await Promise.all([
            GetContentRoots(name),
            ListPacksForVersion(name, ""),
          ]);
          const safe = r || {
            base: "",
            usersRoot: "",
            resourcePacks: "",
            behaviorPacks: "",
            isIsolation: false,
            isPreview: false,
          };
          setRoots(safe);
          setEntries([]);

          const filtered = (allPacks || []).filter(
            (p) => p.manifest.pack_type === 6,
          );

          const basic = await Promise.all(
            filtered.map(async (p) => {
              try {
                const info = await (minecraft as any)?.GetPackInfo?.(p.path);
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

  return (
    <PageContainer ref={scrollRef}>
      <Card className={LAYOUT.GLASS_CARD.BASE}>
        <CardBody className="p-6 flex flex-col gap-6">
          <PageHeader
            title={t("contentpage.resource_packs")}
            endContent={
              <div className="flex items-center gap-2">
                <Button
                  radius="full"
                  variant="flat"
                  startContent={<FaFolderOpen />}
                  onPress={() => OpenPathDir(roots.resourcePacks)}
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
              placeholder={t("common.search_placeholder")}
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

              {selection.isSelectMode && (
                <Checkbox
                  isSelected={
                    sort.filtered.length > 0 && selection.selectedCount === sort.filtered.length
                  }
                  onValueChange={selection.selectAll}
                  radius="full"
                  size="lg"
                  classNames={{ wrapper: "after:bg-primary" }}
                >
                  <span className="text-sm text-default-600">
                    {t("common.select_all")}
                  </span>
                </Checkbox>
              )}

              <Dropdown classNames={COMPONENT_STYLES.dropdown}>
                <DropdownTrigger>
                  <Button
                    variant="flat"
                    radius="full"
                    startContent={
                      sort.sortAsc ? <FaSortAmountDown /> : <FaSortAmountUp />
                    }
                    className={cn(
                      COMPONENT_STYLES.dropdownTriggerButton,
                      "min-w-[120px]",
                    )}
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
                    new Set([`${sort.sortKey}-${sort.sortAsc ? "asc" : "desc"}`])
                  }
                  onSelectionChange={(keys) => {
                    const val = Array.from(keys)[0] as string;
                    const [k, order] = val.split("-");
                    sort.setSortKey(k as "name" | "time");
                    sort.setSortAsc(order === "asc");
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
                {selection.selectedCount > 0 && (
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
                        count: selection.selectedCount,
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

      {loading ? (
        <div className="flex flex-col items-center justify-center py-20 gap-4">
          <Spinner size="lg" />
          <span className="text-default-500 dark:text-zinc-400">
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
                      COMPONENT_STYLES.listItem,
                      "w-full p-5 flex gap-5 group cursor-pointer relative overflow-hidden",
                      selection.isSelectMode && selection.selected[p.path]
                        ? "ring-2 ring-primary bg-primary/5"
                        : "",
                    )}
                    onClick={() => {
                      if (selection.isSelectMode) selection.toggleSelect(p.path);
                    }}
                  >
                    <div className="relative shrink-0">
                      <div className="w-24 h-24 sm:w-28 sm:h-28 rounded-2xl bg-default-100/50 flex items-center justify-center overflow-hidden shadow-sm group-hover:shadow-md transition-shadow">
                        {p.iconDataUrl ? (
                          <Image
                            src={p.iconDataUrl}
                            alt={p.name || p.path}
                            className="w-full h-full object-cover"
                            radius="none"
                          />
                        ) : (
                          <div className="flex flex-col items-center gap-2">
                            <FaFolderOpen className="text-4xl text-default-300" />
                            <span className="text-[10px] text-default-400 font-medium uppercase tracking-wider">
                              No Icon
                            </span>
                          </div>
                        )}
                      </div>
                      {selection.isSelectMode && (
                        <div className="absolute -top-2 -left-2 z-20">
                          <Checkbox
                            isSelected={!!selection.selected[p.path]}
                            onValueChange={() => selection.toggleSelect(p.path)}
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
                          title={p.name}
                        >
                          {renderMcText(p.name || p.path.split("\\").pop())}
                        </h3>
                      </div>

                      <p
                        className="text-sm text-default-500 dark:text-zinc-400 line-clamp-2 w-full mb-3"
                        title={p.description}
                      >
                        {renderMcText(p.description || "")}
                      </p>

                      <div className="flex items-end justify-between mt-auto">
                        <div className="flex flex-wrap items-center gap-4 text-xs text-default-400 dark:text-zinc-500">
                          <div className="flex items-center gap-1.5 bg-default-100/50 dark:bg-zinc-800/50 px-2 py-1 rounded-lg">
                            <FaHdd className="text-default-400" />
                            <span>{formatBytes(p.size)}</span>
                          </div>
                          <div className="flex items-center gap-1.5 bg-default-100/50 dark:bg-zinc-800/50 px-2 py-1 rounded-lg">
                            <FaClock className="text-default-400" />
                            <span>{formatDate(p.modTime)}</span>
                          </div>
                          {p.version && (
                            <div className="flex items-center gap-1.5 bg-default-100/50 dark:bg-zinc-800/50 px-2 py-1 rounded-lg">
                              <FaTag className="text-default-400" />
                              <span>v{p.version}</span>
                            </div>
                          )}
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
                                OpenPathDir(p.path);
                              }}
                              className="bg-default-100 hover:bg-default-200 text-default-600 dark:bg-zinc-800 dark:hover:bg-zinc-700 dark:text-zinc-200"
                            >
                              <FaFolderOpen size={14} />
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
                                setActivePack(p);
                                delCfmOnOpen();
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
          ) : (
            <div className="flex flex-col items-center justify-center py-20 text-default-400 dark:text-zinc-500">
              <FaBox className="text-6xl mb-4 opacity-20" />
              <p>
                {sort.query
                  ? t("common.no_results")
                  : t("contentpage.no_resource_packs")}
              </p>
            </div>
          )}

          {sort.totalPages > 1 && (
            <div className="relative h-12">
              <div className="absolute left-1/2 -translate-x-1/2 bottom-0 w-full flex justify-center">
                <Pagination
                  total={sort.totalPages}
                  page={sort.currentPage}
                  onChange={sort.setCurrentPage}
                  showControls
                  size="sm"
                />
              </div>
            </div>
          )}
        </div>
      )}
      {/* Single Delete Modal */}
      <DeleteConfirmModal
        isOpen={delCfmOpen}
        onOpenChange={delCfmOnOpenChange}
        title={t("common.confirm_delete")}
        description={t("contentpage.delete_pack_confirm", {
          name: activePack?.name || activePack?.path,
        })}
        itemName={activePack?.name || activePack?.path}
        warning={t("contentpage.delete_warning")}
        isPending={deletingOne}
        onConfirm={async () => {
          if (activePack) {
            setDeletingOne(true);
            try {
              await DeletePack(currentVersionName, activePack.path);
              addToast({
                title: t("contentpage.deleted_name", {
                  name: activePack.name,
                }),
                color: "success",
              });
              setActivePack(null);
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
                await DeletePack(currentVersionName, p);
                success++;
              } catch (e) {
                console.error(e);
              }
            }
            addToast({
              title: t("contentpage.deleted_count", {
                count: success,
              }),
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
