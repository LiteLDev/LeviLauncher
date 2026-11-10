import { ModalAction, ModalDescription } from "@/components/ModalPrimitives";
import { PagePagination } from "@/components/PagePagination";
import {
  Button,
  Card,
  InputGroup,
  Label,
  ListBox,
  Select,
  Skeleton,
  TextField,
} from "@heroui/react";

import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import { PageHeader } from "@/components/PageHeader";
import { UnifiedModal } from "@/components/UnifiedModal";
import {
  LIPActiveFilters,
  LIPCatalogFilters,
  type LipCatalogFilterState,
} from "@/components/LIPCatalogFilters";
import { LipTagChips } from "@/components/LipTagChips";
import { Link, useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { getIntlLocale } from "@/utils/intlLocale";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { cn } from "@/utils/cn";
import { Browser } from "@wailsio/runtime";
import {
  compareSemverLike,
  fetchLIPPackagesIndex,
  normalizeSemverInput,
  isLeviLaminaVersionCompatible,
  type LIPPackageBasicInfo,
} from "@/utils/content";
import {
  catalogGameVersion,
  DEFAULT_QUERY_STATE,
  EMPTY_FACET,
  facetOptions,
  filterCount,
  isLipSortKey,
  licenseFilterValues,
  LIP_SORTS,
  ownerUrl,
  runQuery,
  searchCatalog,
  tagFacet,
  type LipSortKey,
} from "@/utils/lipCatalog";
import {
  LuSearch,
  LuDownload,
  LuClock,
  LuStar,
  LuBookOpen,
  LuExternalLink,
  LuFilter,
} from "react-icons/lu";
import { motion } from "framer-motion";
import { readCurrentVersionName } from "@/utils/currentVersion";
import { useCurrentVersion } from "@/utils/CurrentVersionContext";
import { useLeviLamina } from "@/utils/LeviLaminaContext";
import { useModIntelligence } from "@/utils/ModIntelligenceContext";
import {
  GetVersionMeta,
  ListVersionMetas,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/versionservice";
import { routeTo } from "@/constants/routes";

const PAGE_SIZE = 20;
const ALL_GAME_VERSION = "__all_game__";
const ALL_LL_VERSION = "__all_ll__";
/** lip's guide to writing lip.toml (docs/guides/creating_packages.md of LiteLDev/lip). */
const LIP_DEVELOPER_GUIDE_URL =
  "https://docs.levimc.org/lip/guides/creating_packages.html";
const HIDDEN_LIP_PACKAGES = new Set([
  "levilamina",
  "levilamina-loc",
  "crashlogger",
]);

const EMPTY_FILTERS: LipCatalogFilterState = {
  tags: EMPTY_FACET,
  license: [],
  dep: [],
  author: [],
};

/** Comma lists and repeated keys alike (`tags=a,b` = `tags=a&tags=b`), as the site reads them. */
const listParam = (params: URLSearchParams, key: string): string[] =>
  params
    .getAll(key)
    .flatMap((value) => value.split(","))
    .map((value) => value.trim())
    .filter(Boolean);

const uniqueAnyCase = (values: string[]) =>
  values.filter(
    (v, i, all) => all.findIndex((x) => x.toLowerCase() === v.toLowerCase()) === i,
  );

/** The filters a link to the page sets (`/lip?author=LiteLDev`, `?tags=economy,-tp`, `?license=permissive`, `?dep=...`). */
const filtersFromParams = (params: URLSearchParams): LipCatalogFilterState => ({
  tags: tagFacet(listParam(params, "tags")),
  license: licenseFilterValues(listParam(params, "license")),
  dep: uniqueAnyCase(listParam(params, "dep")),
  author: uniqueAnyCase(listParam(params, "author")),
});

const sortLLVersionValuesDesc = (values: string[]): string[] => {
  const unique = Array.from(
    new Set(values.map((value) => String(value || "").trim()).filter(Boolean)),
  );
  unique.sort((a, b) => compareSemverLike(b, a));
  return unique;
};

const normalizeLLVersionForFilter = (value: string): string => {
  const normalized = normalizeSemverInput(value);
  const match = normalized.match(
    /(\d+\.\d+\.\d+(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?)/,
  );
  return match?.[1] ?? normalized;
};

const resolveCurrentInstanceName = async (
  preferredName?: string,
): Promise<string> => {
  const preferred = String(preferredName || "").trim();
  const saved = preferred || readCurrentVersionName().trim();
  try {
    const metas = await ListVersionMetas();
    const names = Array.isArray(metas)
      ? metas
          .map((meta: any) => String(meta?.name || "").trim())
          .filter(Boolean)
      : [];
    if (saved && names.includes(saved)) {
      return saved;
    }
    return names[0] || saved || preferred;
  } catch {
    return saved || preferred;
  }
};

const shouldHidePackage = (pkg: LIPPackageBasicInfo): boolean => {
  const name = String(pkg.name || "")
    .trim()
    .toLowerCase();
  const identifier = String(pkg.identifier || "")
    .trim()
    .toLowerCase();
  const identifierLastSegment =
    identifier.split("/").filter(Boolean).pop() || "";
  return (
    HIDDEN_LIP_PACKAGES.has(name) ||
    HIDDEN_LIP_PACKAGES.has(identifier) ||
    HIDDEN_LIP_PACKAGES.has(identifierLastSegment)
  );
};

const LIPPage: React.FC = () => {
  const { t } = useTranslation();
  const { currentVersionName } = useCurrentVersion();
  const {
    gameToLLVersions,
    mappingAvailable,
    mappingInitialized,
    normalizeGameVersion,
  } = useLeviLamina();
  const { ensureInstanceHydrated, getInstanceSnapshot, snapshotRevision } =
    useModIntelligence();
  const [searchParams] = useSearchParams();
  const [queryInput, setQueryInput] = useState("");
  const [query, setQuery] = useState("");
  const [page, setPage] = useState(1);
  const [allPackages, setAllPackages] = useState<LIPPackageBasicInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [sort, setSort] = useState<LipSortKey>("relevance");
  const [filters, setFilters] = useState<LipCatalogFilterState>(() =>
    filtersFromParams(searchParams),
  );
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [selectedGameVersion, setSelectedGameVersion] =
    useState<string>(ALL_GAME_VERSION);
  const [selectedLLVersion, setSelectedLLVersion] =
    useState<string>(ALL_LL_VERSION);
  const [llVersionOptions, setLLVersionOptions] = useState<string[]>([]);
  const [currentGameVersion, setCurrentGameVersion] = useState("");
  const [currentInstalledLLVersion, setCurrentInstalledLLVersion] =
    useState("");
  const [error, setError] = useState("");
  const [filterContextReady, setFilterContextReady] = useState(false);
  const [developerGuideOpen, setDeveloperGuideOpen] = useState(false);

  const pageRootRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const gameFilterInitializedRef = useRef(false);
  const llFilterInitializedRef = useRef(false);
  // The LeviLamina filter needs the game-to-LeviLamina mapping; the game
  // version filter reads the catalogue's own game versions.
  const llFilterEnabled = mappingAvailable;
  const showMappingUnavailableHint = mappingInitialized && !mappingAvailable;

  // A link to the page ("More by <owner>" on a package page) sets filters.
  const searchKey = searchParams.toString();
  useEffect(() => {
    const next = filtersFromParams(new URLSearchParams(searchKey));
    if (filterCount({ ...DEFAULT_QUERY_STATE, ...next }) === 0) return;
    setFilters(next);
    setPage(1);
  }, [searchKey]);

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
      setAllPackages(list.filter((pkg) => !shouldHidePackage(pkg)));
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

  const options = useMemo(() => facetOptions(allPackages), [allPackages]);
  // Once the catalogue is known, SPDX ids from a link take its spelling and
  // ids no mod has go (the site's license=).
  useEffect(() => {
    if (allPackages.length === 0) return;
    const ids = options.license.flatMap((o) => o.ids);
    setFilters((current) => {
      const license = licenseFilterValues(current.license, ids);
      return license.length === current.license.length &&
        license.every((v, i) => v === current.license[i])
        ? current
        : { ...current, license };
    });
  }, [options, allPackages.length]);
  const gameVersionOptions = options.gv;

  /** Bare id (lower case) → name, for the Dependencies group. */
  const packageNames = useMemo(
    () =>
      new Map(allPackages.map((pkg) => [pkg.id.toLowerCase(), pkg.name] as const)),
    [allPackages],
  );

  const loadFilterContext = useCallback(async () => {
    const instanceName = await resolveCurrentInstanceName(currentVersionName);

    let gameVersion = "";
    let installedLLVersion = "";

    if (instanceName) {
      try {
        const meta: any = await GetVersionMeta(instanceName);
        gameVersion = catalogGameVersion(
          normalizeGameVersion(String(meta?.gameVersion || "").trim()),
        );
      } catch {}

      try {
        await ensureInstanceHydrated(instanceName, {
          background: true,
          reason: "lip-page-filter-context",
        });
        const snapshot = getInstanceSnapshot(instanceName);
        installedLLVersion = normalizeLLVersionForFilter(
          String(snapshot?.llState?.installedVersion || "").trim(),
        );
      } catch {}
    }

    setCurrentGameVersion(gameVersion);
    setCurrentInstalledLLVersion(installedLLVersion);
    setFilterContextReady(true);
  }, [
    currentVersionName,
    ensureInstanceHydrated,
    getInstanceSnapshot,
    normalizeGameVersion,
    snapshotRevision,
  ]);

  useEffect(() => {
    void loadFilterContext();
  }, [loadFilterContext]);

  // The game version starts at the current instance's, when some mod runs on it.
  useEffect(() => {
    if (!filterContextReady || gameVersionOptions.length === 0) return;
    setSelectedGameVersion((prev) => {
      if (!gameFilterInitializedRef.current) {
        gameFilterInitializedRef.current = true;
        return currentGameVersion && gameVersionOptions.includes(currentGameVersion)
          ? currentGameVersion
          : ALL_GAME_VERSION;
      }
      return prev === ALL_GAME_VERSION || gameVersionOptions.includes(prev)
        ? prev
        : ALL_GAME_VERSION;
    });
  }, [filterContextReady, gameVersionOptions, currentGameVersion]);

  /** The LeviLamina versions of the mapping, per catalogue game version (`1.21.50.07` → `1.21.50`). */
  const llVersionsByGame = useMemo(() => {
    const out: Record<string, string[]> = {};
    for (const [game, versions] of Object.entries(gameToLLVersions)) {
      const key = catalogGameVersion(normalizeGameVersion(game));
      out[key] = [
        ...(out[key] || []),
        ...(Array.isArray(versions) ? versions : []).map((version) =>
          normalizeLLVersionForFilter(String(version || "").trim()),
        ),
      ];
    }
    return out;
  }, [gameToLLVersions, normalizeGameVersion]);

  useEffect(() => {
    if (!filterContextReady) return;

    let nextOptions = !mappingAvailable
      ? []
      : selectedGameVersion === ALL_GAME_VERSION
        ? sortLLVersionValuesDesc(Object.values(llVersionsByGame).flat())
        : sortLLVersionValuesDesc(llVersionsByGame[selectedGameVersion] || []);

    if (
      mappingAvailable &&
      currentInstalledLLVersion &&
      !nextOptions.includes(currentInstalledLLVersion) &&
      selectedGameVersion === ALL_GAME_VERSION
    ) {
      nextOptions = [currentInstalledLLVersion, ...nextOptions];
    }

    setLLVersionOptions(nextOptions);
    setSelectedLLVersion((prev) => {
      if (!llFilterInitializedRef.current) {
        llFilterInitializedRef.current = true;
        if (
          currentInstalledLLVersion &&
          nextOptions.includes(currentInstalledLLVersion)
        ) {
          return currentInstalledLLVersion;
        }
        return ALL_LL_VERSION;
      }
      if (
        prev === ALL_LL_VERSION &&
        currentInstalledLLVersion &&
        nextOptions.includes(currentInstalledLLVersion)
      ) {
        return currentInstalledLLVersion;
      }
      if (nextOptions.includes(prev)) {
        return prev;
      }
      return ALL_LL_VERSION;
    });
  }, [
    filterContextReady,
    selectedGameVersion,
    llVersionsByGame,
    currentInstalledLLVersion,
    mappingAvailable,
  ]);

  const scores = useMemo(
    () => searchCatalog(allPackages, query),
    [allPackages, query],
  );

  // The launcher's one filter beyond the site's: a LeviLamina version the
  // mod's preferred variant accepts.
  const llFilter = useMemo(() => {
    if (!llFilterEnabled || selectedLLVersion === ALL_LL_VERSION) {
      return undefined;
    }
    return (pkg: LIPPackageBasicInfo) =>
      pkg.llDependencyRanges.length > 0 &&
      isLeviLaminaVersionCompatible(selectedLLVersion, pkg.llDependencyRanges);
  }, [llFilterEnabled, selectedLLVersion]);

  const result = useMemo(
    () =>
      runQuery(
        allPackages,
        {
          ...DEFAULT_QUERY_STATE,
          q: query,
          gv:
            selectedGameVersion === ALL_GAME_VERSION ? [] : [selectedGameVersion],
          tags: filters.tags,
          license: filters.license,
          dep: filters.dep,
          author: filters.author,
          sort,
        },
        scores,
        llFilter,
      ),
    [allPackages, query, selectedGameVersion, filters, sort, scores, llFilter],
  );
  const sortedPackages = result.matched;
  const activeFilterCount = filterCount({ ...DEFAULT_QUERY_STATE, ...filters });

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

  const handleFiltersChange = (next: LipCatalogFilterState) => {
    setFilters(next);
    setPage(1);
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
    return new Date(dateStr).toLocaleDateString(getIntlLocale());
  };

  return (
    <PageContainer
      ref={pageRootRef}
      className={LAYOUT.CATALOG.PAGE}
      animate={false}
    >
      <motion.div
        data-material-motion
        className="shrink-0"
        data-testid="catalog-toolbar"
        initial={{ opacity: 0, y: -20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4 }}
      >
        <Card className={cn("shrink-0", LAYOUT.GLASS_CARD.BASE)}>
          <Card.Content className={LAYOUT.CATALOG.HEADER_BODY}>
            <div className={LAYOUT.CATALOG.HEADER_ROW}>
              <PageHeader className="shrink-0" title={t("lip.title")} />
              <div className={LAYOUT.CATALOG.SEARCH_ROW}>
                <TextField
                  aria-label={t("lip.search_placeholder")}
                  className={cn(
                    "group",
                    COMPONENT_STYLES.input.mainWrapper,
                    "min-w-0 flex-[1_1_11rem]",
                  )}
                  value={queryInput}
                  onChange={setQueryInput}
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
                      placeholder={t("lip.search_placeholder")}
                      onKeyDown={handleKeyDown}
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
                  {t("common.search")}
                </Button>
                <Button
                  size="sm"
                  onPress={() => {
                    setPage(1);
                    void loadPackages(true);
                  }}
                  isDisabled={loading}
                  variant={"secondary"}
                >
                  {t("common.refresh")}
                </Button>
                <Button
                  size="sm"
                  onPress={() => setDeveloperGuideOpen(true)}
                  variant={"secondary"}
                >
                  {<LuBookOpen />}
                  {t("lip.guide.open_button")}
                </Button>
              </div>
            </div>
            <div data-testid="catalog-filters" className={LAYOUT.CATALOG.FILTERS}>
              <Select
                placeholder={t("lip.game_version_placeholder")}
                isDisabled={gameVersionOptions.length === 0}
                value={Array.from([selectedGameVersion])[0] ?? null}
                onChange={(keys) => {
                  const value = keys as string;
                  setSelectedGameVersion(value || ALL_GAME_VERSION);
                  setPage(1);
                }}
              >
                <Label>{t("lip.game_version_label")}</Label>
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
                      {
                        key: ALL_GAME_VERSION,
                        label: t("lip.game_all_versions"),
                      },
                      ...gameVersionOptions.map((version) => ({
                        key: version,
                        label: `${version} (${result.counts.gv[version] ?? 0})`,
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
                placeholder={t("lip.ll_version_placeholder")}
                isDisabled={!llFilterEnabled}
                value={Array.from([selectedLLVersion])[0] ?? null}
                onChange={(keys) => {
                  const value = keys as string;
                  setSelectedLLVersion(value || ALL_LL_VERSION);
                  setPage(1);
                }}
              >
                <Label>{t("lip.ll_version_label")}</Label>
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
                      { key: ALL_LL_VERSION, label: t("lip.ll_all_versions") },
                      ...llVersionOptions.map((version) => ({
                        key: version,
                        label: version,
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
                placeholder={t("lip.select_sort")}
                value={Array.from([sort])[0] ?? null}
                onChange={(value) => {
                  const key = String(value || "");
                  setSort(isLipSortKey(key) ? key : "relevance");
                  setPage(1);
                }}
              >
                <Label>{t("lip.sort_by")}</Label>
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
                    {LIP_SORTS.map((key) => (
                      <ListBox.Item
                        key={key}
                        id={key}
                        textValue={t(`lip.sort_options.${key}`)}
                      >
                        <Label>{t(`lip.sort_options.${key}`)}</Label>
                        <ListBox.ItemIndicator />
                      </ListBox.Item>
                    ))}
                  </ListBox>
                </Select.Popover>
              </Select>

              <div className="flex min-w-0 flex-col justify-end gap-1">
                <Button
                  size="sm"
                  variant={filtersOpen || activeFilterCount > 0 ? "primary" : "secondary"}
                  aria-expanded={filtersOpen}
                  aria-controls="lip-catalog-filters"
                  onPress={() => setFiltersOpen((open) => !open)}
                  className={cn(
                    "min-h-8 w-full",
                    (filtersOpen || activeFilterCount > 0) &&
                      "bg-brand-500 hover:bg-brand-500 brand-primary-foreground",
                  )}
                >
                  <LuFilter aria-hidden="true" />
                  {activeFilterCount > 0
                    ? t("lip.filters.title_count", { count: activeFilterCount })
                    : t("lip.filters.title")}
                </Button>
              </div>
            </div>
            {activeFilterCount > 0 ? (
              <div className="flex flex-wrap items-center gap-2">
                <LIPActiveFilters
                  state={filters}
                  names={packageNames}
                  onChange={handleFiltersChange}
                />
                <Button
                  size="sm"
                  variant="ghost"
                  onPress={() => handleFiltersChange(EMPTY_FILTERS)}
                >
                  {t("lip.filters.clear")}
                </Button>
              </div>
            ) : null}
            {filtersOpen ? (
              <div
                id="lip-catalog-filters"
                className="max-h-[40vh] overflow-y-auto rounded-xl border border-border bg-surface/40 p-3 dark:border-white/5"
              >
                <LIPCatalogFilters
                  state={filters}
                  options={options}
                  counts={result.counts}
                  names={packageNames}
                  onChange={handleFiltersChange}
                />
              </div>
            ) : null}
            {showMappingUnavailableHint ? (
              <div className="text-sm text-amber-600" role="status">
                {t("lip.mapping_unavailable_hint")}
              </div>
            ) : null}
            {error ? (
              <div className="text-sm text-rose-500" role="alert">
                {error}
              </div>
            ) : null}
          </Card.Content>
        </Card>
      </motion.div>

      <motion.div
        data-material-motion
        className="flex-1 min-h-0 flex flex-col"
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4, delay: 0.1 }}
      >
        <Card className={cn("flex-1 min-h-0 overflow-hidden", LAYOUT.GLASS_CARD.BASE)}>
          <Card.Content className={LAYOUT.CATALOG.RESULTS_BODY}>
            <div
              ref={scrollContainerRef}
              data-testid="catalog-results"
              className={LAYOUT.CATALOG.RESULTS_SCROLL}
            >
              {loading ? (
                <div className="flex flex-col gap-3">{renderSkeletons()}</div>
              ) : error && currentPageItems.length === 0 ? (
                <div className="flex flex-col gap-3 items-center justify-center h-full text-muted">
                  <p>{t("audit.mods.catalog_load_failed")}</p>
                  <Button
                    variant="secondary"
                    onPress={() => void loadPackages(true)}
                  >
                    {t("common.retry")}
                  </Button>
                </div>
              ) : currentPageItems.length === 0 ? (
                <div className="flex items-center justify-center h-full text-muted dark:text-zinc-400">
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
                      <Link
                        className="w-full p-4 bg-surface/50 dark:bg-surface/5 hover:bg-surface-secondary/50 dark:hover:bg-surface/10 transition-all cursor-pointer rounded-2xl flex gap-4 group shadow-sm hover:shadow-md border border-border dark:border-white/5 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
                        to={routeTo.lipPackage(pkg.identifier)}
                        aria-label={t("audit.mods.view_details", { name: pkg.name })}
                      >
                        <div className="shrink-0">
                          {pkg.avatarUrl ? (
                            <img
                              src={pkg.avatarUrl}
                              alt={pkg.name}
                              className="w-20 h-20 sm:w-24 sm:h-24 rounded-2xl object-cover bg-surface-tertiary shadow-sm"
                              loading="lazy"
                            />
                          ) : (
                            <div className="w-20 h-20 sm:w-24 sm:h-24 rounded-2xl bg-surface-tertiary shadow-sm flex items-center justify-center text-muted">
                              <LuDownload size={24} />
                            </div>
                          )}
                        </div>

                        <div className="flex flex-col flex-1 min-w-0 gap-1">
                          <div className="flex items-baseline gap-2 truncate">
                            <h3 className="text-base sm:text-lg font-bold text-foreground truncate">
                              {pkg.name}
                            </h3>
                            <span className="text-xs sm:text-sm text-muted dark:text-zinc-400 truncate">
                              <span aria-hidden="true">|</span>{" "}
                              {pkg.owner ? (
                                // The author links to its GitHub profile (A1);
                                // a span, as the row is a link already.
                                <span
                                  role="link"
                                  className="hover:text-brand-600 dark:hover:text-brand-500 hover:underline"
                                  title={ownerUrl(pkg.owner)}
                                  onClick={(e) => {
                                    e.preventDefault();
                                    e.stopPropagation();
                                    void Browser.OpenURL(ownerUrl(pkg.owner));
                                  }}
                                >
                                  {t("lip.by_author_inline", {
                                    author: pkg.owner,
                                  })}
                                </span>
                              ) : (
                                t("lip.by_author_inline", {
                                  author: t("common.unknown"),
                                })
                              )}
                            </span>
                          </div>

                          <p className="text-xs sm:text-sm text-muted dark:text-zinc-400 line-clamp-2 w-full">
                            {pkg.description || t("lip.no_description")}
                          </p>

                          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted mt-1">
                            <div
                              className="flex items-center gap-1"
                              title={t("lip.sort_options.stars")}
                            >
                              <LuStar className="text-amber-500" aria-hidden="true" />
                              <span>{pkg.stars}</span>
                            </div>
                            <div
                              className="flex items-center gap-1"
                              title={t("lip.sort_options.updated")}
                            >
                              <LuClock aria-hidden="true" />
                              <span>{formatDate(pkg.updated)}</span>
                            </div>
                          </div>

                          <LipTagChips
                            tags={pkg.tags}
                            max={5}
                            className="mt-2"
                            chipClassName="h-5 text-[10px] bg-surface-secondary text-muted dark:text-zinc-400 group-hover:bg-surface-tertiary dark:group-hover:bg-surface-tertiary transition-colors"
                          />
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
                  currentPage={page}
                  onPageChange={(p) => {
                    scheduleScrollReset();
                    setPage(p);
                  }}
                  pageClassName="rounded-full"
                />
              </div>
            )}
          </Card.Content>
        </Card>
      </motion.div>

      <UnifiedModal
        size="wide"
        isOpen={developerGuideOpen}
        onOpenChange={setDeveloperGuideOpen}
        type="primary"
        icon={<LuBookOpen size={24} />}
        title={t("lip.guide.title")}
        isDismissable
        footer={
          <>
            <ModalAction
              onPress={() => setDeveloperGuideOpen(false)}
              variant="secondary"
            >
              {t("lip.guide.close_button")}
            </ModalAction>
            <ModalAction
              onPress={() => void Browser.OpenURL(LIP_DEVELOPER_GUIDE_URL)}
              variant={"primary"}
            >
              {<LuExternalLink />}
              {t("lip.guide.docs_button")}
            </ModalAction>
          </>
        }
      >
        <div className="flex flex-col gap-4 text-sm leading-6 text-foreground dark:text-zinc-300">
          <p>{t("lip.guide.description")}</p>
          <p>{t("lip.guide.manifest_hint")}</p>
          <ModalDescription>
            {t("lip.guide.variant_hint")}
          </ModalDescription>
        </div>
      </UnifiedModal>

    </PageContainer>
  );
};

export default LIPPage;
