import React from "react";
import { useTranslation } from "react-i18next";
import {
  Button,
  Chip,
  Image,
  Spinner,
  Tooltip,
  ModalContent,
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
} from "@heroui/react";
import {
  BaseModal,
  BaseModalHeader,
  BaseModalBody,
  BaseModalFooter,
} from "@/components/BaseModal";
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
import { toast } from "react-hot-toast";
import { PageHeader } from "@/components/PageHeader";

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
  const [query, setQuery] = React.useState<string>("");
  const [sortKey, setSortKey] = React.useState<"name" | "time">(() => {
    try {
      const saved = JSON.parse(
        localStorage.getItem("content.resource.sort") || "{}",
      );
      const k = saved?.sortKey;
      if (k === "name" || k === "time") return k;
    } catch {}
    return "name";
  });
  const [sortAsc, setSortAsc] = React.useState<boolean>(() => {
    try {
      const saved = JSON.parse(
        localStorage.getItem("content.resource.sort") || "{}",
      );
      const a = saved?.sortAsc;
      if (typeof a === "boolean") return a;
    } catch {}
    return true;
  });
  const [selected, setSelected] = React.useState<Record<string, boolean>>({});
  const [isSelectMode, setIsSelectMode] = React.useState<boolean>(false);
  const {
    isOpen: delManyCfmOpen,
    onOpen: delManyCfmOnOpen,
    onOpenChange: delManyCfmOnOpenChange,
  } = useDisclosure();
  // const [selectMode, setSelectMode] = React.useState<boolean>(false); // Removed selectMode
  const [deletingOne, setDeletingOne] = React.useState<boolean>(false);
  const [deletingMany, setDeletingMany] = React.useState<boolean>(false);
  const [currentPage, setCurrentPage] = React.useState(1);
  const pageSize = 20;

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

  // Memoized filtered and sorted packs
  const filtered = React.useMemo(() => {
    const q = query.trim().toLowerCase();
    const f = packs.filter((p) => {
      const nm = String(
        p.name || p.path?.split("\\").pop() || "",
      ).toLowerCase();
      return q ? nm.includes(q) : true;
    });
    return f.sort((a: any, b: any) => {
      if (sortKey === "name") {
        const an = String(
          a.name || a.path?.split("\\").pop() || "",
        ).toLowerCase();
        const bn = String(
          b.name || b.path?.split("\\").pop() || "",
        ).toLowerCase();
        const res = an.localeCompare(bn);
        return sortAsc ? res : -res;
      } else {
        const at = Number(a.modTime || 0);
        const bt = Number(b.modTime || 0);
        const res = at - bt;
        return sortAsc ? res : -res;
      }
    });
  }, [packs, query, sortKey, sortAsc]);

  // Reset page when filter/sort changes
  React.useEffect(() => {
    setCurrentPage(1);
  }, [query, sortKey, sortAsc]);

  const paginatedItems = React.useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return filtered.slice(start, start + pageSize);
  }, [filtered, currentPage]);

  const totalPages = Math.ceil(filtered.length / pageSize);

  const selectedCount = Object.keys(selected).filter((k) => selected[k]).length;

  const toggleSelect = (path: string) => {
    setSelected((prev) => ({
      ...prev,
      [path]: !prev[path],
    }));
  };

  const selectAll = (val: boolean) => {
    if (val) {
      const newSel: Record<string, boolean> = {};
      filtered.forEach((p) => {
        newSel[p.path] = true;
      });
      setSelected(newSel);
    } else {
      setSelected({});
    }
  };

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
        setError(
          t("contentpage.error_resolve_paths", {
            defaultValue: "无法解析内容路径。",
          }) as string,
        );
      } finally {
        if (!silent) setLoading(false);
      }
    },
    [hasBackend, t],
  );

  React.useEffect(() => {
    refreshAll();
  }, []);
  React.useEffect(() => {
    try {
      localStorage.setItem(
        "content.resource.sort",
        JSON.stringify({ sortKey, sortAsc }),
      );
    } catch {}
  }, [sortKey, sortAsc]);
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
  }, [packs]);

  const formatBytes = (n?: number) => {
    const v = typeof n === "number" ? n : 0;
    if (v < 1024) return `${v} B`;
    const k = 1024;
    const sizes = ["KB", "MB", "GB", "TB"];
    let i = -1;
    let val = v;
    do {
      val /= k;
      i++;
    } while (val >= k && i < sizes.length - 1);
    return `${val.toFixed(val >= 100 ? 0 : val >= 10 ? 1 : 2)} ${sizes[i]}`;
  };

  const formatDate = (ts?: number) => {
    const v = typeof ts === "number" ? ts : 0;
    if (!v) return "";
    const d = new Date(v * 1000);
    return d.toLocaleString();
  };

  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.25 }}
      className="w-full max-w-full mx-auto p-4 h-full flex flex-col"
    >
      <Card className="flex-1 min-h-0 border-none shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl">
        <CardBody className="p-0 flex flex-col h-full overflow-hidden">
          <div className="shrink-0 p-4 sm:p-6 pb-2 flex flex-col gap-4 border-b border-default-200 dark:border-white/10">
            <PageHeader
              title={t("contentpage.resource_packs", {
                defaultValue: "资源包",
              })}
              startContent={
                <Button
                  isIconOnly
                  radius="full"
                  variant="light"
                  onPress={() => navigate("/content")}
                >
                  <FaArrowLeft size={20} />
                </Button>
              }
              endContent={
                <div className="flex items-center gap-2">
                  <Button
                    radius="full"
                    variant="flat"
                    startContent={<FaFolderOpen />}
                    onPress={() => OpenPathDir(roots.resourcePacks)}
                    className="bg-default-100 dark:bg-zinc-800 text-default-600 dark:text-zinc-200 font-medium"
                  >
                    {t("common.open", { defaultValue: "打开" })}
                  </Button>
                  <Tooltip
                    content={
                      t("common.refresh", {
                        defaultValue: "刷新",
                      }) as unknown as string
                    }
                  >
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

            {/* Toolbar */}
            <div className="flex flex-col md:flex-row gap-4 items-end md:items-center justify-between">
              <Input
                placeholder={t("common.search_placeholder", {
                  defaultValue: "搜索...",
                })}
                value={query}
                onValueChange={setQuery}
                startContent={<FaFilter className="text-default-400" />}
                endContent={
                  query && (
                    <button onClick={() => setQuery("")}>
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
                <Tooltip
                  content={t("common.select_mode", {
                    defaultValue: "选择模式",
                  })}
                >
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
                      {t("common.select_all", { defaultValue: "全选" })}
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
                        ? (t("filemanager.sort.name", {
                            defaultValue: "名称",
                          }) as string)
                        : (t("contentpage.sort_time", {
                            defaultValue: "时间",
                          }) as string)}
                      {" / "}
                      {sortAsc
                        ? t("contentpage.sort_asc", {
                            defaultValue: "从上到下",
                          })
                        : t("contentpage.sort_desc", {
                            defaultValue: "从下到上",
                          })}
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
                      setSortKey(k as "name" | "time");
                      setSortAsc(order === "asc");
                    }}
                  >
                    <DropdownItem
                      key="name-asc"
                      startContent={<FaSortAmountDown />}
                    >
                      {t("filemanager.sort.name", { defaultValue: "名称" })}{" "}
                      (A-Z)
                    </DropdownItem>
                    <DropdownItem
                      key="name-desc"
                      startContent={<FaSortAmountUp />}
                    >
                      {t("filemanager.sort.name", { defaultValue: "名称" })}{" "}
                      (Z-A)
                    </DropdownItem>
                    <DropdownItem
                      key="time-asc"
                      startContent={<FaSortAmountDown />}
                    >
                      {t("contentpage.sort_time", { defaultValue: "时间" })}{" "}
                      (Old-New)
                    </DropdownItem>
                    <DropdownItem
                      key="time-desc"
                      startContent={<FaSortAmountUp />}
                    >
                      {t("contentpage.sort_time", { defaultValue: "时间" })}{" "}
                      (New-Old)
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
                          defaultValue: "删除选中",
                        })}
                      </Button>
                    </motion.div>
                  )}
                </AnimatePresence>
              </div>
            </div>
            <div className="mt-2 text-default-500 text-sm flex flex-wrap items-center gap-2">
              <span>
                {t("contentpage.current_version", { defaultValue: "当前版本" })}
                :
              </span>
              <span className="font-medium text-default-700 bg-default-100 px-2 py-0.5 rounded-md">
                {currentVersionName ||
                  t("contentpage.none", { defaultValue: "无" })}
              </span>
              <span className="text-default-300">|</span>
              <span>
                {t("contentpage.isolation", { defaultValue: "版本隔离" })}:
              </span>
              <span
                className={`font-medium px-2 py-0.5 rounded-md ${
                  roots.isIsolation
                    ? "bg-success-50 text-success-600 dark:bg-success-900/20 dark:text-success-400"
                    : "bg-default-100 text-default-700"
                }`}
              >
                {roots.isIsolation
                  ? t("common.yes", { defaultValue: "是" })
                  : t("common.no", { defaultValue: "否" })}
              </span>
            </div>
          </div>

          <div ref={scrollRef} className="flex-1 overflow-y-auto p-4 sm:p-6">
            {loading ? (
              <div className="flex flex-col items-center justify-center py-20 gap-4">
                <Spinner size="lg" />
                <span className="text-default-500">
                  {t("common.loading", { defaultValue: "加载中" })}
                </span>
              </div>
            ) : (
              <div className="flex flex-col gap-4">
                {filtered.length ? (
                  <div className="flex flex-col gap-3 pb-4">
                    {paginatedItems.map((p, idx) => (
                      <motion.div
                        key={`${p.path}-${idx}`}
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        transition={{ duration: 0.2 }}
                      >
                        <div
                          className={`w-full p-4 bg-white dark:bg-zinc-900/50 hover:bg-default-50 dark:hover:bg-zinc-800 transition-all rounded-2xl flex gap-4 group shadow-sm hover:shadow-md border ${
                            isSelectMode && selected[p.path]
                              ? "border-primary bg-primary/10"
                              : "border-default-200 dark:border-zinc-700/50 hover:border-default-400 dark:hover:border-zinc-600"
                          }`}
                          onClick={() => {
                            if (isSelectMode) toggleSelect(p.path);
                          }}
                        >
                          <div className="relative shrink-0">
                            <div className="w-20 h-20 sm:w-24 sm:h-24 rounded-lg bg-default-100 flex items-center justify-center overflow-hidden shadow-inner">
                              {p.iconDataUrl ? (
                                <Image
                                  src={p.iconDataUrl}
                                  alt={p.name || p.path}
                                  className="w-full h-full object-cover"
                                  radius="none"
                                />
                              ) : (
                                <FaFolderOpen className="text-3xl text-default-300" />
                              )}
                            </div>
                            {isSelectMode && (
                              <Checkbox
                                isSelected={!!selected[p.path]}
                                onValueChange={() => toggleSelect(p.path)}
                                className="absolute -top-2 -left-2 z-20"
                                classNames={{
                                  wrapper:
                                    "bg-white dark:bg-zinc-900 shadow-md",
                                }}
                              />
                            )}
                          </div>

                          <div className="flex flex-col flex-1 min-w-0 gap-1">
                            <div className="flex items-baseline gap-2 truncate">
                              <h3
                                className="text-base sm:text-lg font-bold text-default-900 dark:text-white truncate"
                                title={p.name}
                              >
                                {renderMcText(
                                  p.name || p.path.split("\\").pop(),
                                )}
                              </h3>
                            </div>

                            <p
                              className="text-xs sm:text-sm text-zinc-600 dark:text-zinc-300 line-clamp-2 w-full"
                              title={p.description}
                            >
                              {renderMcText(p.description || "")}
                            </p>

                            <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-zinc-500 dark:text-zinc-400 mt-1">
                              <div
                                className="flex items-center gap-1"
                                title={t("common.size", {
                                  defaultValue: "大小",
                                })}
                              >
                                <FaHdd />
                                <span>{formatBytes(p.size)}</span>
                              </div>
                              <div
                                className="flex items-center gap-1"
                                title={t("common.date", {
                                  defaultValue: "日期",
                                })}
                              >
                                <FaClock />
                                <span>{formatDate(p.modTime)}</span>
                              </div>
                              {p.version && (
                                <div
                                  className="flex items-center gap-1"
                                  title={t("common.version", {
                                    defaultValue: "版本",
                                  })}
                                >
                                  <FaTag />
                                  <span>v{p.version}</span>
                                </div>
                              )}
                            </div>

                            <div className="flex flex-1 items-end justify-between mt-2">
                              <div className="flex gap-1">
                                {/* Placeholder for future tags */}
                              </div>
                              <Button
                                size="sm"
                                color="danger"
                                variant="flat"
                                radius="full"
                                onPress={() => {
                                  setActivePack(p);
                                  delCfmOnOpen();
                                }}
                                className="h-8 min-w-0 px-3"
                              >
                                {t("common.delete", { defaultValue: "删除" })}
                              </Button>
                            </div>
                          </div>
                        </div>
                      </motion.div>
                    ))}
                  </div>
                ) : (
                  <div className="flex flex-col items-center justify-center py-20 text-default-400">
                    <FaBox className="text-6xl mb-4 opacity-20" />
                    <p>
                      {query
                        ? t("common.no_results", { defaultValue: "无搜索结果" })
                        : t("contentpage.no_resource_packs", {
                            defaultValue: "暂无资源包",
                          })}
                    </p>
                  </div>
                )}

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
          </div>
        </CardBody>
      </Card>

      {/* Single Delete Modal */}
      <BaseModal
        isOpen={delCfmOpen}
        onClose={delCfmOnOpenChange}
        isDismissable={!deletingOne}
        hideCloseButton={deletingOne}
        title={t("common.confirm_delete", { defaultValue: "确认删除" })}
      >
        <ModalContent>
          {(onClose) => (
            <>
              <BaseModalHeader className="text-danger">
                {t("common.confirm_delete", { defaultValue: "确认删除" })}
              </BaseModalHeader>
              <BaseModalBody>
                {deletingOne ? (
                  <div className="flex flex-col items-center justify-center py-6 gap-3">
                    <Spinner size="lg" color="danger" />
                    <p className="text-default-500 font-medium">
                      {t("common.deleting", { defaultValue: "正在删除..." })}
                    </p>
                  </div>
                ) : (
                  <div className="flex flex-col gap-2">
                    <p>
                      {t("contentpage.delete_pack_confirm", {
                        name: activePack?.name || activePack?.path,
                        defaultValue: `确定要删除 ${
                          activePack?.name || activePack?.path
                        } 吗？`,
                      })}
                    </p>
                    <p className="text-xs text-danger">
                      {t("contentpage.delete_warning", {
                        defaultValue: "此操作无法撤销！",
                      })}
                    </p>
                  </div>
                )}
              </BaseModalBody>
              <BaseModalFooter>
                <Button
                  variant="flat"
                  onPress={onClose}
                  isDisabled={deletingOne}
                >
                  {t("common.cancel", { defaultValue: "取消" })}
                </Button>
                <Button
                  color="danger"
                  isDisabled={deletingOne}
                  onPress={async () => {
                    if (activePack) {
                      setDeletingOne(true); // Ensure state is set if not already handled by wrapper
                      try {
                        await DeletePack(currentVersionName, activePack.path);
                        toast.success(
                          t("contentpage.deleted_name", {
                            name: activePack.name,
                            defaultValue: `已删除 ${activePack.name}`,
                          }),
                        );
                        setActivePack(null);
                        refreshAll();
                        delCfmOnOpenChange();
                      } finally {
                        setDeletingOne(false);
                      }
                    }
                  }}
                >
                  {t("common.delete", { defaultValue: "删除" })}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>

      {/* Batch Delete Modal */}
      <BaseModal
        isOpen={delManyCfmOpen}
        onClose={delManyCfmOnOpenChange}
        isDismissable={!deletingMany}
        hideCloseButton={deletingMany}
        title={t("common.confirm_delete", { defaultValue: "确认删除" })}
      >
        <ModalContent>
          {(onClose) => (
            <>
              <BaseModalHeader className="text-danger">
                {t("common.confirm_delete", { defaultValue: "确认删除" })}
              </BaseModalHeader>
              <BaseModalBody>
                {deletingMany ? (
                  <div className="flex flex-col items-center justify-center py-6 gap-3">
                    <Spinner size="lg" color="danger" />
                    <p className="text-default-500 font-medium">
                      {t("common.deleting", { defaultValue: "正在删除..." })}
                    </p>
                  </div>
                ) : (
                  <div className="flex flex-col gap-2">
                    <p>
                      {t("contentpage.delete_selected_confirm", {
                        count: Object.values(selected).filter(Boolean).length,
                        defaultValue: `确定要删除选中的 ${
                          Object.values(selected).filter(Boolean).length
                        } 个项目吗？`,
                      })}
                    </p>
                    <p className="text-xs text-danger">
                      {t("contentpage.delete_warning", {
                        defaultValue: "此操作无法撤销！",
                      })}
                    </p>
                  </div>
                )}
              </BaseModalBody>
              <BaseModalFooter>
                <Button
                  variant="flat"
                  onPress={onClose}
                  isDisabled={deletingMany}
                >
                  {t("common.cancel", { defaultValue: "取消" })}
                </Button>
                <Button
                  color="danger"
                  isDisabled={deletingMany}
                  onPress={async () => {
                    const targets = Object.keys(selected).filter(
                      (k) => selected[k],
                    );
                    if (targets.length === 0) return;

                    setDeletingMany(true);
                    let success = 0;
                    for (const p of targets) {
                      try {
                        await DeletePack(currentVersionName, p);
                        success++;
                      } catch (e) {
                        console.error(e);
                      }
                    }
                    toast.success(
                      t("contentpage.deleted_count", {
                        count: success,
                        defaultValue: `已删除 ${success} 个项目`,
                      }),
                    );
                    setSelected({});
                    // setSelectMode(false);
                    refreshAll();
                    setDeletingMany(false);
                    onClose();
                  }}
                >
                  {t("common.delete", { defaultValue: "删除" })}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>
    </motion.div>
  );
}
