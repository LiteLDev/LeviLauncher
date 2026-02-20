import React, { useEffect, useMemo, useRef, useState } from "react";
import {
  Button,
  Input,
  Pagination,
  Skeleton,
  Card,
  CardBody,
  Chip,
  Select,
  SelectItem,
} from "@heroui/react";
import { PageHeader } from "@/components/PageHeader";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { cn } from "@/utils/cn";
import { fetchLIPPackagesIndex, type LIPPackageBasicInfo } from "@/utils/content";
import { LuSearch, LuDownload, LuClock, LuFlame } from "react-icons/lu";
import { motion } from "framer-motion";

const PAGE_SIZE = 20;
const ALL_TAG = "__all__";

type LIPSortKey = "hotness" | "updated" | "name";
type LIPOrderKey = "asc" | "desc";

const parseUpdatedTime = (value: string): number => {
  const timestamp = Date.parse(value);
  return Number.isFinite(timestamp) ? timestamp : 0;
};

const sortPackages = (
  list: LIPPackageBasicInfo[],
  sort: LIPSortKey,
  order: LIPOrderKey,
) => {
  const sorted = [...list].sort((a, b) => {
    if (sort === "hotness") return a.hotness - b.hotness;
    if (sort === "updated") return parseUpdatedTime(a.updated) - parseUpdatedTime(b.updated);
    return a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
  });

  if (order === "desc") {
    sorted.reverse();
  }

  return sorted;
};

const LIPPage: React.FC = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [queryInput, setQueryInput] = useState("");
  const [query, setQuery] = useState("");
  const [page, setPage] = useState(1);
  const [allPackages, setAllPackages] = useState<LIPPackageBasicInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [sort, setSort] = useState<LIPSortKey>("hotness");
  const [order, setOrder] = useState<LIPOrderKey>("desc");
  const [selectedTag, setSelectedTag] = useState<string>(ALL_TAG);
  const [error, setError] = useState("");

  const pageRootRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);

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

  const resetScrollTop = () => {
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
    resetScrollTop();
    const raf = requestAnimationFrame(resetScrollTop);
    const t0 = window.setTimeout(resetScrollTop, 0);
    const t1 = window.setTimeout(resetScrollTop, 120);
    return () => {
      cancelAnimationFrame(raf);
      clearTimeout(t0);
      clearTimeout(t1);
    };
  };

  const loadPackages = async (forceRefresh = false) => {
    setLoading(true);
    setError("");
    scheduleScrollReset();
    try {
      const list = await fetchLIPPackagesIndex({ forceRefresh });
      setAllPackages(list);
    } catch (err) {
      console.error(err);
      setError(t("common.load_failed"));
    } finally {
      setLoading(false);
      scheduleScrollReset();
    }
  };

  useEffect(() => {
    void loadPackages();
  }, []);

  const availableTags = useMemo(() => {
    const unique = new Set<string>();
    for (const pkg of allPackages) {
      for (const tag of pkg.tags) {
        const normalized = String(tag || "").trim();
        if (normalized) unique.add(normalized);
      }
    }
    return Array.from(unique).sort((a, b) =>
      a.localeCompare(b, undefined, { sensitivity: "base" }),
    );
  }, [allPackages]);

  useEffect(() => {
    if (selectedTag !== ALL_TAG && !availableTags.includes(selectedTag)) {
      setSelectedTag(ALL_TAG);
    }
  }, [availableTags, selectedTag]);

  const filteredPackages = useMemo(() => {
    const keyword = query.trim().toLowerCase();

    return allPackages.filter((pkg) => {
      if (selectedTag !== ALL_TAG && !pkg.tags.includes(selectedTag)) {
        return false;
      }

      if (!keyword) {
        return true;
      }

      const haystacks = [pkg.identifier, pkg.name, pkg.description, pkg.author, ...pkg.tags].map(
        (part) => String(part || "").toLowerCase(),
      );
      return haystacks.some((part) => part.includes(keyword));
    });
  }, [allPackages, query, selectedTag]);

  const sortedPackages = useMemo(
    () => sortPackages(filteredPackages, sort, order),
    [filteredPackages, sort, order],
  );

  const totalPages = Math.max(1, Math.ceil(sortedPackages.length / PAGE_SIZE));

  useEffect(() => {
    if (page > totalPages) {
      setPage(totalPages);
    }
  }, [page, totalPages]);

  const currentPageItems = useMemo(() => {
    const start = (page - 1) * PAGE_SIZE;
    return sortedPackages.slice(start, start + PAGE_SIZE);
  }, [sortedPackages, page]);

  const handleSearch = () => {
    setQuery(queryInput.trim());
    setPage(1);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      handleSearch();
    }
  };

  const handleSortChange = (value: LIPSortKey) => {
    setSort(value);
    setPage(1);
    if (value === "name") {
      setOrder("asc");
    } else {
      setOrder("desc");
    }
  };

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

  const formatDate = (dateStr: string) => {
    if (!dateStr) return "-";
    return new Date(dateStr).toLocaleDateString();
  };

  return (
    <PageContainer ref={pageRootRef} className="min-h-0" animate={false}>
      <Card className={cn("shrink-0", LAYOUT.GLASS_CARD.BASE)}>
        <CardBody className="p-6 flex flex-col gap-6">
          <PageHeader title="LIP Content" />

          <div className="flex flex-col sm:flex-row gap-3">
            <Input
              placeholder={t("lip.search_placeholder")}
              value={queryInput}
              onValueChange={setQueryInput}
              onKeyDown={handleKeyDown}
              startContent={<LuSearch />}
              className="flex-1"
              size="sm"
              classNames={COMPONENT_STYLES.input}
            />
            <Button
              color="primary"
              onPress={handleSearch}
              startContent={<LuSearch />}
              size="sm"
              className="bg-primary-600 hover:bg-primary-500 text-white font-bold shadow-lg shadow-primary-900/20"
            >
              {t("common.search")}
            </Button>
            <Button
              variant="flat"
              size="sm"
              onPress={() => {
                setPage(1);
                void loadPackages(true);
              }}
              isDisabled={loading}
            >
              {t("common.refresh")}
            </Button>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
            <Select
              label={t("common.all")}
              placeholder={t("common.all")}
              className="max-w-xs"
              selectedKeys={[selectedTag]}
              onSelectionChange={(keys) => {
                const value = Array.from(keys)[0] as string;
                setSelectedTag(value || ALL_TAG);
                setPage(1);
              }}
              size="sm"
              classNames={COMPONENT_STYLES.select}
              items={[
                { key: ALL_TAG, label: t("common.all") },
                ...availableTags.map((tag) => ({ key: tag, label: tag })),
              ]}
            >
              {(item) => <SelectItem key={item.key}>{item.label}</SelectItem>}
            </Select>

            <Select
              label={t("lip.sort_by")}
              placeholder={t("lip.select_sort")}
              className="max-w-xs"
              selectedKeys={[sort]}
              onChange={(e) => handleSortChange(e.target.value as LIPSortKey)}
              size="sm"
              classNames={COMPONENT_STYLES.select}
            >
              <SelectItem key="hotness">Hotness</SelectItem>
              <SelectItem key="updated">Updated</SelectItem>
              <SelectItem key="name">Name</SelectItem>
            </Select>

            <Select
              label={t("lip.order_by")}
              placeholder={t("lip.select_order")}
              className="max-w-xs"
              selectedKeys={[order]}
              onChange={(e) => {
                setOrder(e.target.value as LIPOrderKey);
                setPage(1);
              }}
              size="sm"
              classNames={COMPONENT_STYLES.select}
            >
              <SelectItem key="desc">Desc</SelectItem>
              <SelectItem key="asc">Asc</SelectItem>
            </Select>
          </div>

          <div className="text-sm text-default-500 dark:text-zinc-400">
            {t("common.all")}: {sortedPackages.length}
          </div>

          {error ? (
            <div className="text-sm text-danger-500" role="alert">
              {error}
            </div>
          ) : null}
        </CardBody>
      </Card>

      <Card className={cn("flex-1 min-h-0", LAYOUT.GLASS_CARD.BASE)}>
        <CardBody className="p-0 overflow-hidden flex flex-col">
          <div
            ref={scrollContainerRef}
            className="flex-1 overflow-y-auto p-4 relative [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]"
          >
            {loading ? (
              <div className="flex flex-col gap-3">{renderSkeletons()}</div>
            ) : currentPageItems.length === 0 ? (
              <div className="flex items-center justify-center h-full text-default-500 dark:text-zinc-400">
                <p>{t("common.no_results")}</p>
              </div>
            ) : (
              <div className="flex flex-col gap-3">
                {currentPageItems.map((pkg) => (
                  <motion.div
                    key={pkg.identifier}
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    transition={{ duration: 0.2 }}
                  >
                    <div
                      className="w-full p-4 bg-default-50/50 dark:bg-white/5 hover:bg-default-100/50 dark:hover:bg-white/10 transition-all cursor-pointer rounded-2xl flex gap-4 group shadow-sm hover:shadow-md border border-default-100 dark:border-white/5"
                      onClick={() =>
                        navigate(`/lip/package/${encodeURIComponent(pkg.identifier)}`)
                      }
                    >
                      <div className="shrink-0">
                        {pkg.avatarUrl ? (
                          <img
                            src={pkg.avatarUrl}
                            alt={pkg.name}
                            className="w-20 h-20 sm:w-24 sm:h-24 rounded-2xl object-cover bg-content3 shadow-sm"
                            loading="lazy"
                          />
                        ) : (
                          <div className="w-20 h-20 sm:w-24 sm:h-24 rounded-2xl bg-content3 shadow-sm flex items-center justify-center text-default-300">
                            <LuDownload size={24} />
                          </div>
                        )}
                      </div>

                      <div className="flex flex-col flex-1 min-w-0 gap-1">
                        <div className="flex items-baseline gap-2 truncate">
                          <h3 className="text-base sm:text-lg font-bold text-foreground truncate">
                            {pkg.name}
                          </h3>
                          <span className="text-xs sm:text-sm text-default-500 dark:text-zinc-400 truncate">
                            | By {pkg.author || t("common.unknown")}
                          </span>
                        </div>

                        <p className="text-xs sm:text-sm text-default-500 dark:text-zinc-400 line-clamp-2 w-full">
                          {pkg.description || t("lip.no_description")}
                        </p>

                        <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-default-400 mt-1">
                          <div className="flex items-center gap-1" title="Hotness">
                            <LuFlame className="text-orange-500" />
                            <span>{pkg.hotness}</span>
                          </div>
                          <div className="flex items-center gap-1" title="Updated">
                            <LuClock />
                            <span>{formatDate(pkg.updated)}</span>
                          </div>
                        </div>

                        <div className="flex flex-wrap gap-1 mt-2">
                          {pkg.tags.slice(0, 5).map((tag) => (
                            <Chip
                              key={tag}
                              size="sm"
                              variant="flat"
                              radius="sm"
                              className="h-5 text-[10px] bg-default-100 dark:bg-zinc-800 text-default-500 dark:text-zinc-400 group-hover:bg-default-200 dark:group-hover:bg-zinc-700 transition-colors"
                            >
                              {tag}
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
                page={page}
                onChange={(p) => {
                  scheduleScrollReset();
                  setPage(p);
                }}
                showControls
                color="primary"
                className="gap-2"
                radius="full"
                classNames={{
                  cursor:
                    "bg-primary-600 hover:bg-primary-500 shadow-lg shadow-primary-900/20 font-bold",
                }}
              />
            </div>
          )}
        </CardBody>
      </Card>
    </PageContainer>
  );
};

export default LIPPage;
