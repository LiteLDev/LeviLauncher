import { PagePagination } from "@/components/PagePagination";
import {
  Button,
  Card,
  Chip,
  InputGroup,
  Label,
  ListBox,
  Select,
  Skeleton,
  TextField,
} from "@heroui/react";

import React, { useEffect, useState, useMemo } from "react";

import { PageHeader } from "@/components/PageHeader";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { routeTo } from "@/constants/routes";
import { cn } from "@/utils/cn";
import {
  formatNumber,
  formatDateStr,
  formatFileSize,
  sortGameVersions,
} from "@/utils/formatting";
import {
  GetCurseForgeGameVersions,
  SearchCurseForgeMods,
  GetCurseForgeCategories,
} from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { ModData } from "bindings/github.com/liteldev/LeviLauncher/internal/curseforge/client/types";
import { useCurseForge } from "@/utils/CurseForgeContext";
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

const getLatestSupportedVersion = (mod: ModData) => {
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

  const sorted = sortGameVersions(Array.from(versions));
  return sorted[0];
};

export const CurseForgePage: React.FC = () => {
  const { t } = useTranslation();
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
      best = Math.max(best, (target as HTMLElement).scrollTop || 0);
    }
    return best;
  };

  const applyScrollTop = (y: number) => {
    for (const target of collectScrollTargets()) {
      if (target === window) {
        window.scrollTo({ top: y, left: 0, behavior: "auto" });
        continue;
      }
      (target as HTMLElement).scrollTop = y;
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
        if (target instanceof HTMLElement) {
          target.scrollTop = 0;
          target.scrollLeft = 0;
        }
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
      setError(t("curseforge.load_error"));
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
      setError(t("curseforge.search_error"));
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
    <PageContainer
      ref={pageRootRef}
      className={LAYOUT.CATALOG.PAGE}
      animate={false}
    >
      <motion.div
        className="shrink-0"
        data-testid="catalog-toolbar"
        initial={{ opacity: 0, y: -20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4 }}
      >
        <Card className={cn("shrink-0", LAYOUT.GLASS_CARD.BASE)}>
          <Card.Content className={LAYOUT.CATALOG.HEADER_BODY}>
            <div className={LAYOUT.CATALOG.HEADER_ROW}>
              <PageHeader className="shrink-0" title={t("curseforge.title")} />
              <div className={LAYOUT.CATALOG.SEARCH_ROW}>
                <TextField
                  aria-label={t("curseforge.search_placeholder")}
                  className={cn(
                    "group",
                    COMPONENT_STYLES.input.mainWrapper,
                    "min-w-0 flex-[1_1_11rem]",
                  )}
                  value={query}
                  onChange={setQuery}
                >
                  <InputGroup
                    className={cn(
                      COMPONENT_STYLES.input.inputWrapper,
                      COMPONENT_STYLES.input.innerWrapper,
                      "min-h-8 text-sm",
                    )}
                  >
                    <InputGroup.Prefix>{<LuSearch />}</InputGroup.Prefix>
                    <InputGroup.Input
                      placeholder={t("curseforge.search_placeholder")}
                      onKeyPress={handleKeyPress}
                      className={COMPONENT_STYLES.input.input}
                    />
                  </InputGroup>
                </TextField>
                <Button
                  onPress={handleSearch}
                  size="sm"
                  variant={"primary"}
                  className={
                    "bg-brand-500 hover:bg-brand-500 brand-primary-foreground font-bold shadow-lg shadow-brand-900/20"
                  }
                >
                  {<LuSearch />}
                  {t("curseforge.search")}
                </Button>
              </div>
            </div>
            <div data-testid="catalog-filters" className={LAYOUT.CATALOG.FILTERS}>
              <Select
                placeholder={t("curseforge.select_version")}
                value={Array.from([selectedMinecraftVersion])[0] ?? null}
                onChange={(keys) => {
                  const value = keys as string;
                  setSelectedMinecraftVersion(value || "");
                  setCurrentPage(1);
                }}
              >
                <Label>{t("curseforge.minecraft_version")}</Label>
                <Select.Trigger
                  className={cn(
                    COMPONENT_STYLES.select.trigger,
                    "min-h-8 text-sm",
                  )}
                >
                  <Select.Value />
                  <Select.Indicator />
                </Select.Trigger>
                <Select.Popover
                  className={COMPONENT_STYLES.select.popoverContent}
                >
                  <ListBox
                    items={[
                      { key: "", label: t("curseforge.all_versions") },
                      ...gameVersions.map((v) => ({
                        key: v.name,
                        label: v.name,
                      })),
                    ]}
                    className={COMPONENT_STYLES.select.listbox}
                  >
                    {(item) => (
                      <ListBox.Item
                        key={item.key}
                        id={item.key}
                        textValue={item.label}
                      >
                        <Label>{item.label}</Label>
                        <ListBox.ItemIndicator />
                      </ListBox.Item>
                    )}
                  </ListBox>
                </Select.Popover>
              </Select>

              <Select
                placeholder={t("curseforge.select_class")}
                value={
                  Array.from(
                    selectedClass !== undefined ? [String(selectedClass)] : [],
                  )[0] ?? null
                }
                onChange={(keys) => {
                  const value = keys as string;
                  setSelectedClass(value ? parseInt(value) : 0);
                  setSelectedCategories([]);
                  setCurrentPage(1);
                }}
              >
                <Label>{t("curseforge.class")}</Label>
                <Select.Trigger
                  className={cn(
                    COMPONENT_STYLES.select.trigger,
                    "min-h-8 text-sm",
                  )}
                >
                  <Select.Value />
                  <Select.Indicator />
                </Select.Trigger>
                <Select.Popover
                  className={COMPONENT_STYLES.select.popoverContent}
                >
                  <ListBox
                    items={[
                      { name: t("curseforge.all_classes"), id: 0 },
                      ...classes,
                    ]}
                    className={COMPONENT_STYLES.select.listbox}
                  >
                    {(item) => (
                      <ListBox.Item
                        key={String(item.id)}
                        id={String(item.id)}
                        textValue={item.name}
                      >
                        <Label>{item.name}</Label>
                        <ListBox.ItemIndicator />
                      </ListBox.Item>
                    )}
                  </ListBox>
                </Select.Popover>
              </Select>

              <Select
                placeholder={t("curseforge.select_category")}
                isDisabled={!selectedClass}
                selectionMode="multiple"
                value={Array.from(selectedCategories.map(String))}
                onChange={(keys) => {
                  const values = keys
                    .map((k) => parseInt(String(k)))
                    .filter((n) => !isNaN(n));
                  setSelectedCategories(values);
                  setCurrentPage(1);
                }}
              >
                <Label>{t("curseforge.category")}</Label>
                <Select.Trigger
                  className={cn(
                    COMPONENT_STYLES.select.trigger,
                    "min-h-8 text-sm",
                  )}
                >
                  <Select.Value />
                  <Select.Indicator />
                </Select.Trigger>
                <Select.Popover
                  className={COMPONENT_STYLES.select.popoverContent}
                >
                  <ListBox className={COMPONENT_STYLES.select.listbox}>
                    {categories.map((cat) => (
                      <ListBox.Item
                        key={String(cat.id)}
                        id={String(cat.id)}
                        textValue={cat.name}
                      >
                        <Label>{cat.name}</Label>
                        <ListBox.ItemIndicator />
                      </ListBox.Item>
                    ))}
                  </ListBox>
                </Select.Popover>
              </Select>

              <Select
                placeholder={t("curseforge.select_sort")}
                value={
                  Array.from(selectedSort ? [String(selectedSort)] : [])[0] ??
                  null
                }
                onChange={(keys) => {
                  const value = keys as string;
                  setSelectedSort(parseInt(value));
                  setCurrentPage(1);
                }}
              >
                <Label>{t("curseforge.sort_by")}</Label>
                <Select.Trigger
                  className={cn(
                    COMPONENT_STYLES.select.trigger,
                    "min-h-8 text-sm",
                  )}
                >
                  <Select.Value />
                  <Select.Indicator />
                </Select.Trigger>
                <Select.Popover
                  className={COMPONENT_STYLES.select.popoverContent}
                >
                  <ListBox
                    items={sortOptions}
                    className={COMPONENT_STYLES.select.listbox}
                  >
                    {(opt) => (
                      <ListBox.Item
                        key={String(opt.value)}
                        id={String(opt.value)}
                        textValue={opt.label}
                      >
                        <Label>{opt.label}</Label>
                        <ListBox.ItemIndicator />
                      </ListBox.Item>
                    )}
                  </ListBox>
                </Select.Popover>
              </Select>
            </div>
          </Card.Content>
        </Card>
      </motion.div>

      <motion.div
        className="flex-1 min-h-0 flex flex-col"
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4, delay: 0.1 }}
      >
        <Card className={cn("flex-1 min-h-0 overflow-hidden", LAYOUT.GLASS_CARD.BASE)}>
          <Card.Content className={LAYOUT.CATALOG.RESULTS_BODY}>
            <div
              ref={scrollContainerRef}
              onScroll={(e) => {
                lastScrollTopRef.current = getScrollTop();
              }}
              data-testid="catalog-results"
              className={LAYOUT.CATALOG.RESULTS_SCROLL}
            >
              {error ? (
                <div className="flex flex-col items-center justify-center h-full gap-4 text-danger">
                  <p>{error}</p>
                  <Button
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
                    variant={"danger-soft"}
                  >
                    {t("common.retry")}
                  </Button>
                </div>
              ) : loading || !hasSearched ? (
                <div className="flex flex-col gap-3">{renderSkeletons()}</div>
              ) : mods.length === 0 ? (
                <div className="flex items-center justify-center h-full text-muted dark:text-zinc-400">
                  <p>{t("curseforge.no_results")}</p>
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
                      <Link
                        className="w-full p-4 bg-surface/50 dark:bg-surface/5 hover:bg-surface-secondary/50 dark:hover:bg-surface/10 transition-all cursor-pointer rounded-2xl flex gap-4 group shadow-sm hover:shadow-md border border-border dark:border-white/5 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
                        to={routeTo.curseForgeMod(mod.id)}
                        aria-label={t("audit.mods.view_details", { name: mod.name })}
                        onClick={saveScrollPosition}
                      >
                        <div className="shrink-0">
                          <img
                            src={mod.logo?.thumbnailUrl || mod.logo?.url || ""}
                            alt={mod.name}
                            className="w-20 h-20 sm:w-24 sm:h-24 rounded-2xl object-cover bg-surface-tertiary shadow-sm"
                            loading="lazy"
                          />
                        </div>

                        <div className="flex flex-col flex-1 min-w-0 gap-1">
                          <div className="flex items-baseline gap-2 truncate">
                            <h3 className="text-base sm:text-lg font-bold text-foreground truncate">
                              {mod.name}
                            </h3>
                            <span className="text-xs sm:text-sm text-muted dark:text-zinc-400 truncate">
                              |{" "}
                              {t("curseforge.by_author", {
                                author: mod.authors?.[0]?.name || "Unknown",
                              })}
                            </span>
                          </div>

                          <p className="text-xs sm:text-sm text-muted dark:text-zinc-400 line-clamp-2 w-full">
                            {mod.summary || t("curseforge.no_description")}
                          </p>

                          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted mt-1">
                            <div
                              className="flex items-center gap-1"
                              title={t("curseforge.downloads")}
                            >
                              <LuDownload />
                              <span>{formatNumber(mod.downloadCount)}</span>
                            </div>
                            <div
                              className="flex items-center gap-1"
                              title={t("curseforge.updated")}
                            >
                              <LuClock />
                              <span>{formatDateStr(mod.dateModified)}</span>
                            </div>
                            <div
                              className="flex items-center gap-1"
                              title={t("curseforge.created")}
                            >
                              <LuCalendar />
                              <span>{formatDateStr(mod.dateCreated)}</span>
                            </div>
                            <div
                              className="flex items-center gap-1"
                              title={t("curseforge.size")}
                            >
                              <LuFileDigit />
                              <span>
                                {formatFileSize(
                                  mod.latestFiles?.[0]?.fileLength,
                                )}
                              </span>
                            </div>
                            <div
                              className="flex items-center gap-1"
                              title={t("curseforge.game_version")}
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
                                    variant="soft"
                                    className={cn(
                                      "rounded-sm",
                                      "h-5 text-[10px] bg-accent/10 text-accent font-medium",
                                    )}
                                  >
                                    <Chip.Label>{classCat.name}</Chip.Label>
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
                                  variant="soft"
                                  className={cn(
                                    "rounded-sm",
                                    "h-5 text-[10px] bg-surface-secondary text-muted dark:text-zinc-400 group-hover:bg-surface-tertiary dark:group-hover:bg-surface-tertiary transition-colors",
                                  )}
                                >
                                  <Chip.Label>{cat.name}</Chip.Label>
                                </Chip>
                              ))}
                          </div>
                        </div>
                      </Link>
                    </motion.div>
                  ))}
                </div>
              )}
            </div>
            {totalPages > 1 && (
              <div data-testid="catalog-pagination" className={LAYOUT.CATALOG.FOOTER}>
                <PagePagination
                  className="gap-2"
                  pageCount={totalPages}
                  currentPage={currentPage}
                  onPageChange={(page) => {
                    setCurrentPage(page);
                    setSearchToken((v) => v + 1);
                  }}
                  pageClassName="rounded-full"
                />
              </div>
            )}
          </Card.Content>
        </Card>
      </motion.div>
    </PageContainer>
  );
};

export default CurseForgePage;
