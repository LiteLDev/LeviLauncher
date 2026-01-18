import React, { useEffect, useState, useMemo } from "react";
import {
  Button,
  Chip,
  Input,
  Select,
  SelectItem,
  Pagination,
  Skeleton,
  Card,
  CardBody,
} from "@heroui/react";
import { PageHeader } from "@/components/PageHeader";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  GetCurseForgeGameVersions,
  SearchCurseForgeMods,
  GetCurseForgeCategories,
} from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { useCurseForge } from "@/utils/CurseForgeContext";
import * as types from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import {
  LuSearch,
  LuDownload,
  LuEye,
  LuClock,
  LuCalendar,
  LuFileDigit,
  LuGamepad2,
} from "react-icons/lu";
import { motion } from "framer-motion";

const CURSEFORGE_GAME_ID = "78022";

const formatNumber = (num: number | undefined) => {
  if (num === undefined) return "0";
  if (num >= 1000000) {
    return (num / 1000000).toFixed(1) + "M";
  }
  if (num >= 1000) {
    return (num / 1000).toFixed(1) + "K";
  }
  return num.toString();
};

const formatDate = (dateStr: string | undefined) => {
  if (!dateStr) return "-";
  return new Date(dateStr).toLocaleDateString();
};

const formatSize = (bytes: number | undefined) => {
  if (bytes === undefined) return "-";
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
};

const getLatestSupportedVersion = (mod: types.CurseForgeMod) => {
  const versions = new Set<string>();

  mod.latestFilesIndexes?.forEach((idx) => {
    if (idx.gameVersion) versions.add(idx.gameVersion);
  });

  mod.latestFiles?.forEach((file) => {
    file.gameVersions?.forEach((v) => {
      if (v && /^\d/.test(v)) {
        versions.add(v);
      }
    });
  });

  if (versions.size === 0) return "-";

  const sorted = Array.from(versions).sort((a, b) => {
    const partsA = a.split(".").map((p) => parseInt(p) || 0);
    const partsB = b.split(".").map((p) => parseInt(p) || 0);

    for (let i = 0; i < Math.max(partsA.length, partsB.length); i++) {
      const valA = partsA[i] || 0;
      const valB = partsB[i] || 0;
      if (valA !== valB) return valB - valA;
    }
    return 0;
  });

  return sorted[0];
};

export const CurseForgePage: React.FC = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const {
    query,
    setQuery,
    mods,
    setMods,
    gameVersions,
    setGameVersions,
    selectedMinecraftVersion,
    setSelectedMinecraftVersion,
    allCategories,
    setAllCategories,
    selectedClass,
    setSelectedClass,
    selectedCategories,
    setSelectedCategories,
    currentPage,
    setCurrentPage,
    searchToken,
    setSearchToken,
    totalCount,
    setTotalCount,
    selectedSort,
    setSelectedSort,
    initialLoaded,
    setInitialLoaded,
    scrollPosition,
    setScrollPosition,
    hasSearched,
    setHasSearched,
  } = useCurseForge();

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const pageRootRef = React.useRef<HTMLDivElement>(null);
  const scrollContainerRef = React.useRef<HTMLDivElement>(null);
  const lastScrollTopRef = React.useRef(0);
  const searchSeqRef = React.useRef(0);

  const classes = useMemo(() => {
    return allCategories
      .filter((c) => c.isClass)
      .sort((a, b) => a.displayIndex - b.displayIndex);
  }, [allCategories]);

  const categories = useMemo(() => {
    if (!selectedClass) return [];
    return allCategories
      .filter((c) => c.classId === selectedClass && !c.isClass)
      .sort((a, b) => a.displayIndex - b.displayIndex);
  }, [allCategories, selectedClass]);
  const pageSize = 20;

  const sortOptions = [
    { value: 1, label: "Featured" },
    { value: 2, label: "Popularity" },
    { value: 3, label: "Last Updated" },
    { value: 10, label: "Creation Date" },
    { value: 6, label: "Total Downloads" },
  ];

  const renderSkeletons = () => {
    return Array(5)
      .fill(0)
      .map((_, index) => (
        <div key={index} className="w-full flex items-center gap-3 p-3">
          <div>
            <Skeleton className="flex rounded-lg w-20 h-20 sm:w-24 sm:h-24" />
          </div>
          <div className="w-full flex flex-col gap-2">
            <Skeleton className="h-3 w-3/5 rounded-lg" />
            <Skeleton className="h-3 w-4/5 rounded-lg" />
            <div className="flex gap-2 pt-2">
              <Skeleton className="h-2 w-1/4 rounded-lg" />
              <Skeleton className="h-2 w-1/4 rounded-lg" />
            </div>
          </div>
        </div>
      ));
  };

  useEffect(() => {
    if (!initialLoaded) {
      void loadInitialData();
    }
  }, []);

  useEffect(() => {
    return () => {
      setScrollPosition(getScrollTop());
    };
  }, []);

  const collectScrollTargets = () => {
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

    walk(scrollContainerRef.current);
    walk(pageRootRef.current);

    return targets;
  };

  const getScrollTop = () => {
    let best = 0;
    for (const target of collectScrollTargets()) {
      if (target === window) {
        best = Math.max(best, window.scrollY || 0);
        continue;
      }
      best = Math.max(best, target.scrollTop || 0);
    }
    return best;
  };

  const applyScrollTop = (y: number) => {
    for (const target of collectScrollTargets()) {
      if (target === window) {
        window.scrollTo({ top: y, left: 0, behavior: "auto" });
        continue;
      }
      target.scrollTop = y;
    }
  };

  useEffect(() => {
    const handler = () => {
      lastScrollTopRef.current = getScrollTop();
    };
    document.addEventListener("scroll", handler, true);
    return () => {
      document.removeEventListener("scroll", handler, true);
    };
  }, []);

  const prevDepsRef = React.useRef<{
    selectedMinecraftVersion: string;
    selectedClass: number;
    selectedCategories: number[];
    selectedSort: number;
    currentPage: number;
    searchToken: number;
  } | null>(null);
  const restoredRef = React.useRef(false);
  const saveScrollPosition = () => {
    const y = getScrollTop();
    lastScrollTopRef.current = y;
    setScrollPosition(y);
  };

  useEffect(() => {
    if (!initialLoaded) return;

    const currentDeps = {
      selectedMinecraftVersion,
      selectedClass,
      selectedCategories,
      selectedSort,
      currentPage,
      searchToken,
    };

    const prevDeps = prevDepsRef.current;
    const depsChanged =
      !prevDeps ||
      prevDeps.selectedMinecraftVersion !==
        currentDeps.selectedMinecraftVersion ||
      prevDeps.selectedClass !== currentDeps.selectedClass ||
      prevDeps.selectedSort !== currentDeps.selectedSort ||
      prevDeps.currentPage !== currentDeps.currentPage ||
      prevDeps.searchToken !== currentDeps.searchToken ||
      JSON.stringify(prevDeps.selectedCategories) !==
        JSON.stringify(currentDeps.selectedCategories);

    prevDepsRef.current = currentDeps;

    const resetScroll = () => {
      for (const target of collectScrollTargets()) {
        if (target === window) {
          window.scrollTo({ top: 0, left: 0, behavior: "auto" });
          continue;
        }
        target.scrollTop = 0;
        target.scrollLeft = 0;
      }
    };

    const scheduleScrollReset = () => {
      resetScroll();
      const raf = requestAnimationFrame(resetScroll);
      const t0 = window.setTimeout(resetScroll, 0);
      const t1 = window.setTimeout(resetScroll, 120);
      return () => {
        cancelAnimationFrame(raf);
        clearTimeout(t0);
        clearTimeout(t1);
      };
    };

    if (hasSearched && (!depsChanged || prevDeps === null)) {
      if (!restoredRef.current) {
        applyScrollTop(scrollPosition);
        const raf = requestAnimationFrame(() => applyScrollTop(scrollPosition));
        const t0 = window.setTimeout(() => applyScrollTop(scrollPosition), 0);
        const t1 = window.setTimeout(() => applyScrollTop(scrollPosition), 120);
        lastScrollTopRef.current = scrollPosition;
        restoredRef.current = true;
        return () => {
          cancelAnimationFrame(raf);
          clearTimeout(t0);
          clearTimeout(t1);
        };
      }
      return;
    }

    const cleanup = scheduleScrollReset();
    const timer = setTimeout(() => {
      void searchMods().finally(() => {
        scheduleScrollReset();
      });
    }, 300);
    return () => {
      clearTimeout(timer);
      cleanup();
    };
  }, [
    initialLoaded,
    selectedMinecraftVersion,
    selectedClass,
    selectedCategories,
    selectedSort,
    currentPage,
    searchToken,
  ]);

  const loadInitialData = async () => {
    try {
      setLoading(true);
      setError(null);
      const [versions, cats] = await Promise.all([
        GetCurseForgeGameVersions(CURSEFORGE_GAME_ID),
        GetCurseForgeCategories(CURSEFORGE_GAME_ID),
      ]);

      if (versions && versions.length > 0) {
        setGameVersions(versions);
      }

      if (cats && cats.length > 0) {
        setAllCategories(cats);
        setSelectedClass(0);
      }
    } catch (error) {
      console.error("Failed to load initial data:", error);
      setError(
        t("curseforge.load_error", {
          defaultValue: "Failed to load data, please retry",
        }),
      );
    } finally {
      setLoading(false);
      setInitialLoaded(true);
    }
  };

  const searchMods = async () => {
    const seq = ++searchSeqRef.current;
    try {
      setLoading(true);
      setError(null);
      const index = (currentPage - 1) * pageSize;
      const response = await SearchCurseForgeMods(
        CURSEFORGE_GAME_ID,
        selectedMinecraftVersion,
        selectedClass,
        selectedCategories,
        query,
        selectedSort,
        0,
        pageSize,
        index,
      );

      if (seq !== searchSeqRef.current) return;
      if (response?.data) {
        setMods(response.data);
        setTotalCount(response.pagination?.totalCount || 0);
      } else {
        setMods([]);
        setTotalCount(0);
      }
      setHasSearched(true);
    } catch (error) {
      if (seq !== searchSeqRef.current) return;
      console.error("Failed to search mods:", error);
      setMods([]);
      setTotalCount(0);
      setError(
        t("curseforge.search_error", {
          defaultValue: "Search failed, please retry",
        }),
      );
    } finally {
      if (seq !== searchSeqRef.current) return;
      setLoading(false);
    }
  };

  const handleSearch = () => {
    setCurrentPage(1);
    setSearchToken((v) => v + 1);
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      handleSearch();
    }
  };

  const totalPages = Math.ceil(totalCount / pageSize);

  return (
    <div
      ref={pageRootRef}
      className="w-full max-w-full mx-auto p-4 h-full min-h-0 flex flex-col gap-4"
    >
      <Card className="shrink-0 bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl shadow-md border-none">
        <CardBody className="p-6 flex flex-col gap-4">
          <PageHeader
            title={t("curseforge.title", { defaultValue: "CurseForge" })}
          />

          <div className="flex flex-col sm:flex-row gap-3">
            <Input
              placeholder={t("curseforge.search_placeholder", {
                defaultValue: "搜索模组...",
              })}
              value={query}
              onValueChange={setQuery}
              onKeyPress={handleKeyPress}
              startContent={<LuSearch />}
              className="flex-1"
              size="sm"
              classNames={{
                inputWrapper:
                  "bg-default-100/50 dark:bg-default-50/20 backdrop-blur-md",
              }}
            />
            <Button
              color="primary"
              onPress={handleSearch}
              startContent={<LuSearch />}
              size="sm"
              className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
            >
              {t("curseforge.search", { defaultValue: "搜索" })}
            </Button>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
            <Select
              label={t("curseforge.minecraft_version", {
                defaultValue: "Minecraft版本",
              })}
              placeholder={t("curseforge.select_version", {
                defaultValue: "选择版本",
              })}
              selectedKeys={[selectedMinecraftVersion]}
              onSelectionChange={(keys) => {
                const value = Array.from(keys)[0] as string;
                setSelectedMinecraftVersion(value || "");
                setCurrentPage(1);
              }}
              size="sm"
              classNames={{
                trigger:
                  "bg-default-100/50 dark:bg-default-50/20 backdrop-blur-md",
              }}
            >
              <SelectItem key="" value="">
                {t("curseforge.all_versions", { defaultValue: "全部版本" })}
              </SelectItem>
              {gameVersions.map((version) => (
                <SelectItem key={version.name} value={version.name}>
                  {version.name}
                </SelectItem>
              ))}
            </Select>

            <Select
              label={t("curseforge.class", { defaultValue: "类型" })}
              placeholder={t("curseforge.select_class", {
                defaultValue: "选择类型",
              })}
              selectedKeys={
                selectedClass !== undefined ? [String(selectedClass)] : []
              }
              onSelectionChange={(keys) => {
                const value = Array.from(keys)[0] as string;
                setSelectedClass(value ? parseInt(value) : 0);
                setSelectedCategories([]);
                setCurrentPage(1);
              }}
              size="sm"
              classNames={{
                trigger:
                  "bg-default-100/50 dark:bg-default-50/20 backdrop-blur-md",
              }}
            >
              <SelectItem key="0" value="0">
                {t("curseforge.all_classes", { defaultValue: "全部类型" })}
              </SelectItem>
              {classes.map((cls) => (
                <SelectItem key={String(cls.id)} value={String(cls.id)}>
                  {cls.name}
                </SelectItem>
              ))}
            </Select>

            <Select
              label={t("curseforge.category", { defaultValue: "分类" })}
              placeholder={t("curseforge.select_category", {
                defaultValue: "选择分类",
              })}
              isDisabled={!selectedClass}
              selectionMode="multiple"
              selectedKeys={selectedCategories.map(String)}
              onSelectionChange={(keys) => {
                const values = Array.from(keys)
                  .map((k) => parseInt(String(k)))
                  .filter((n) => !isNaN(n));
                setSelectedCategories(values);
                setCurrentPage(1);
              }}
              size="sm"
              classNames={{
                trigger:
                  "bg-default-100/50 dark:bg-default-50/20 backdrop-blur-md",
              }}
            >
              {categories.map((cat) => (
                <SelectItem key={String(cat.id)} value={String(cat.id)}>
                  {cat.name}
                </SelectItem>
              ))}
            </Select>

            <Select
              label={t("curseforge.sort_by", { defaultValue: "排序" })}
              placeholder={t("curseforge.select_sort", {
                defaultValue: "选择排序",
              })}
              selectedKeys={selectedSort ? [String(selectedSort)] : []}
              onSelectionChange={(keys) => {
                const value = Array.from(keys)[0] as string;
                setSelectedSort(parseInt(value));
                setCurrentPage(1);
              }}
              size="sm"
              classNames={{
                trigger:
                  "bg-default-100/50 dark:bg-default-50/20 backdrop-blur-md",
              }}
            >
              {sortOptions.map((opt) => (
                <SelectItem key={String(opt.value)} value={String(opt.value)}>
                  {opt.label}
                </SelectItem>
              ))}
            </Select>
          </div>
        </CardBody>
      </Card>

      <Card className="flex-1 min-h-0 bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl shadow-md border-none">
        <CardBody className="p-0 overflow-hidden flex flex-col">
          <div
            ref={scrollContainerRef}
            onScroll={(e) => {
              lastScrollTopRef.current = getScrollTop();
            }}
            className="flex-1 overflow-y-auto p-4 relative [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]"
          >
            {error ? (
              <div className="flex flex-col items-center justify-center h-full gap-4 text-danger">
                <p>{error}</p>
                <Button
                  color="danger"
                  variant="flat"
                  onPress={() => {
                    if (
                      gameVersions.length === 0 &&
                      allCategories.length === 0
                    ) {
                      void loadInitialData();
                    } else {
                      setSearchToken((v) => v + 1);
                    }
                  }}
                >
                  {t("retry", { defaultValue: "Retry" })}
                </Button>
              </div>
            ) : loading || !hasSearched ? (
              <div className="flex flex-col gap-3">{renderSkeletons()}</div>
            ) : mods.length === 0 ? (
              <div className="flex items-center justify-center h-full text-default-500">
                <p>
                  {t("curseforge.no_results", { defaultValue: "未找到结果" })}
                </p>
              </div>
            ) : (
              <div className="flex flex-col gap-3">
                {mods.map((mod, index) => (
                  <motion.div
                    key={mod.id}
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    transition={{ duration: 0.2 }}
                  >
                    <div
                      className="w-full p-4 bg-default-50/50 dark:bg-white/5 hover:bg-default-100/50 dark:hover:bg-white/10 transition-all cursor-pointer rounded-2xl flex gap-4 group shadow-sm hover:shadow-md border border-default-100 dark:border-white/5"
                      onClick={() => {
                        saveScrollPosition();
                        navigate(`/curseforge/mod/${mod.id}`);
                      }}
                    >
                      <div className="shrink-0">
                        <img
                          src={mod.logo?.thumbnailUrl || mod.logo?.url || ""}
                          alt={mod.name}
                          className="w-20 h-20 sm:w-24 sm:h-24 rounded-2xl object-cover bg-content3 shadow-sm"
                          loading="lazy"
                        />
                      </div>

                      <div className="flex flex-col flex-1 min-w-0 gap-1">
                        <div className="flex items-baseline gap-2 truncate">
                          <h3 className="text-base sm:text-lg font-bold text-foreground truncate">
                            {mod.name}
                          </h3>
                          <span className="text-xs sm:text-sm text-default-500 truncate">
                            |{" "}
                            {t("curseforge.by_author", {
                              author:
                                mod.authors?.[0]?.name ||
                                mod.author ||
                                "Unknown",
                              defaultValue: `By ${mod.authors?.[0]?.name || mod.author || "Unknown"}`,
                            })}
                          </span>
                        </div>

                        <p className="text-xs sm:text-sm text-default-500 line-clamp-2 w-full">
                          {mod.summary ||
                            t("curseforge.no_description", {
                              defaultValue: "No description available.",
                            })}
                        </p>

                        <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-default-400 mt-1">
                          <div
                            className="flex items-center gap-1"
                            title={t("curseforge.downloads", {
                              defaultValue: "Downloads",
                            })}
                          >
                            <LuDownload />
                            <span>{formatNumber(mod.downloadCount)}</span>
                          </div>
                          <div
                            className="flex items-center gap-1"
                            title={t("curseforge.updated", {
                              defaultValue: "Updated",
                            })}
                          >
                            <LuClock />
                            <span>{formatDate(mod.dateModified)}</span>
                          </div>
                          <div
                            className="flex items-center gap-1"
                            title={t("curseforge.created", {
                              defaultValue: "Created",
                            })}
                          >
                            <LuCalendar />
                            <span>{formatDate(mod.dateCreated)}</span>
                          </div>
                          <div
                            className="flex items-center gap-1"
                            title={t("curseforge.size", {
                              defaultValue: "Size",
                            })}
                          >
                            <LuFileDigit />
                            <span>
                              {formatSize(mod.latestFiles?.[0]?.fileLength)}
                            </span>
                          </div>
                          <div
                            className="flex items-center gap-1"
                            title={t("curseforge.game_version", {
                              defaultValue: "Game Version",
                            })}
                          >
                            <LuGamepad2 />
                            <span>
                              {selectedMinecraftVersion ||
                                getLatestSupportedVersion(mod)}
                            </span>
                          </div>
                        </div>

                        <div className="flex flex-wrap gap-1 mt-2">
                          {(() => {
                            const classCat = allCategories.find(
                              (c) => c.id === mod.classId,
                            );
                            if (classCat) {
                              return (
                                <Chip
                                  key={`class-${classCat.id}`}
                                  size="sm"
                                  variant="flat"
                                  radius="sm"
                                  className="h-5 text-[10px] bg-primary/10 text-primary font-medium"
                                >
                                  {classCat.name}
                                </Chip>
                              );
                            }
                            return null;
                          })()}

                          {mod.categories
                            ?.filter((cat) => cat.id !== mod.classId)
                            .map((cat) => (
                              <Chip
                                key={cat.id}
                                size="sm"
                                variant="flat"
                                radius="sm"
                                className="h-5 text-[10px] bg-default-100 text-default-500 group-hover:bg-default-200 transition-colors"
                              >
                                {cat.name}
                              </Chip>
                            ))}
                        </div>
                      </div>
                    </div>
                  </motion.div>
                ))}
              </div>
            )}
          </div>

          {totalPages > 1 && (
            <div className="flex justify-center p-4 border-t border-default-100 dark:border-white/5 bg-white/50 dark:bg-zinc-900/50 backdrop-blur-md shrink-0">
              <Pagination
                total={totalPages}
                page={currentPage}
                onChange={(page) => {
                  setCurrentPage(page);
                  setSearchToken((v) => v + 1);
                }}
                showControls
                color="primary"
                className="gap-2"
                radius="full"
                classNames={{
                  cursor:
                    "bg-emerald-600 hover:bg-emerald-500 shadow-lg shadow-emerald-900/20 font-bold",
                }}
              />
            </div>
          )}
        </CardBody>
      </Card>
    </div>
  );
};

export default CurseForgePage;
