import React, { useEffect, useState, useRef } from "react";
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
import { cn } from "@/utils/cn";
import { SearchLIPPackages } from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import * as liptypes from "bindings/github.com/liteldev/LeviLauncher/internal/lip/client/types";
import { LuSearch, LuDownload, LuClock, LuFlame } from "react-icons/lu";
import { motion } from "framer-motion";

const LIPPage: React.FC = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  const [page, setPage] = useState(1);
  const [packages, setPackages] = useState<liptypes.PackageItem[]>([]);
  const [totalPages, setTotalPages] = useState(1);
  const [loading, setLoading] = useState(false);
  const [sort, setSort] = useState("hotness");
  const [order, setOrder] = useState("desc");
  const [hasSearched, setHasSearched] = useState(false);
  const [totalCount, setTotalCount] = useState(0);
  const [searchToken, setSearchToken] = useState(0);

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

  const loadPackages = async () => {
    setLoading(true);
    scheduleScrollReset();
    try {
      const res = await SearchLIPPackages(query, 20, page, sort, order);
      if (res) {
        setPackages(res.items || []);
        setTotalPages(res.totalPages || 1);
        setTotalCount(res.totalPages * 20);
      }
      setHasSearched(true);
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
      scheduleScrollReset();
    }
  };

  useEffect(() => {
    loadPackages();
  }, [page, sort, order, searchToken]);

  const handleSearch = () => {
    setPage(1);
    setSearchToken((v) => v + 1);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      handleSearch();
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
              value={query}
              onValueChange={setQuery}
              onKeyDown={handleKeyDown}
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
              className="bg-primary-600 hover:bg-primary-500 text-white font-bold shadow-lg shadow-primary-900/20"
            >
              {t("common.search")}
            </Button>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
            <Select
              label={t("lip.sort_by")}
              placeholder={t("lip.select_sort")}
              selectedKeys={[sort]}
              onChange={(e) => {
                setSort(e.target.value);
                setPage(1);
              }}
              size="sm"
              classNames={{
                trigger:
                  "bg-default-100/50 dark:bg-default-50/20 backdrop-blur-md",
              }}
            >
              <SelectItem key="hotness">Hotness</SelectItem>
              <SelectItem key="updated">Updated</SelectItem>
            </Select>
          </div>
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
            ) : packages.length === 0 ? (
              <div className="flex items-center justify-center h-full text-default-500 dark:text-zinc-400">
                <p>{t("common.no_results")}</p>
              </div>
            ) : (
              <div className="flex flex-col gap-3">
                {packages.map((pkg) => (
                  <motion.div
                    key={pkg.identifier}
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    transition={{ duration: 0.2 }}
                  >
                    <div
                      className="w-full p-4 bg-default-50/50 dark:bg-white/5 hover:bg-default-100/50 dark:hover:bg-white/10 transition-all cursor-pointer rounded-2xl flex gap-4 group shadow-sm hover:shadow-md border border-default-100 dark:border-white/5"
                      onClick={() => navigate(`/lip/package/${pkg.identifier}`)}
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
                            | By {pkg.author}
                          </span>
                        </div>

                        <p className="text-xs sm:text-sm text-default-500 dark:text-zinc-400 line-clamp-2 w-full">
                          {pkg.description || t("lip.no_description")}
                        </p>

                        <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-default-400 mt-1">
                          <div
                            className="flex items-center gap-1"
                            title="Hotness"
                          >
                            <LuFlame className="text-orange-500" />
                            <span>{pkg.hotness}</span>
                          </div>
                          <div
                            className="flex items-center gap-1"
                            title="Updated"
                          >
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
