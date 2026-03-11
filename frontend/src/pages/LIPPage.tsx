import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
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
import { UnifiedModal } from "@/components/UnifiedModal";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { cn } from "@/utils/cn";
import { Browser } from "@wailsio/runtime";
import {
  fetchLIPPackagesIndex,
  fetchLIPLeviLaminaClientMapping,
  isLeviLaminaRangesCompatibleWithAnyVersion,
  isLeviLaminaVersionCompatible,
  type LIPPackageBasicInfo,
} from "@/utils/content";
import {
  LuSearch,
  LuDownload,
  LuClock,
  LuFlame,
  LuBookOpen,
  LuExternalLink,
} from "react-icons/lu";
import { motion } from "framer-motion";
import { readCurrentVersionName } from "@/utils/currentVersion";
import { useCurrentVersion } from "@/utils/CurrentVersionContext";
import { useModIntelligence } from "@/utils/ModIntelligenceContext";
import {
  GetVersionMeta,
  ListVersionMetas,
} from "bindings/github.com/liteldev/LeviLauncher/versionservice";

const PAGE_SIZE = 20;
const ALL_GAME_VERSION = "__all_game__";
const ALL_LL_VERSION = "__all_ll__";
const LIP_DEVELOPER_GUIDE_URL =
  "https://github.com/LiteLDev/levilamina-mod-template/blob/d0a888ec116c9a00fe4dc8f28ccead4dfc6b6760/tooth.json#L51";
const HIDDEN_LIP_PACKAGES = new Set([
  "levilamina",
  "levilamina-loc",
  "crashlogger",
]);

type LIPSortKey = "hotness" | "updated" | "name";
type LIPOrderKey = "asc" | "desc";

type ParsedSemver = {
  major: number;
  minor: number;
  patch: number;
  pre: string[];
};

const normalizeSemverInput = (value: string): string =>
  String(value || "")
    .trim()
    .replace(/^v/i, "");

const parseSemver = (input: string): ParsedSemver | null => {
  const normalized = normalizeSemverInput(input);
  if (!normalized) return null;
  const match = normalized.match(
    /^(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$/,
  );
  if (!match) return null;
  return {
    major: Number(match[1]),
    minor: Number(match[2]),
    patch: Number(match[3]),
    pre: match[4] ? match[4].split(".") : [],
  };
};

const compareSemver = (a: ParsedSemver, b: ParsedSemver): number => {
  if (a.major !== b.major) return a.major > b.major ? 1 : -1;
  if (a.minor !== b.minor) return a.minor > b.minor ? 1 : -1;
  if (a.patch !== b.patch) return a.patch > b.patch ? 1 : -1;

  const aRelease = a.pre.length === 0;
  const bRelease = b.pre.length === 0;
  if (aRelease && bRelease) return 0;
  if (aRelease) return 1;
  if (bRelease) return -1;

  const length = Math.max(a.pre.length, b.pre.length);
  for (let i = 0; i < length; i += 1) {
    const aPart = a.pre[i];
    const bPart = b.pre[i];
    if (aPart === undefined) return -1;
    if (bPart === undefined) return 1;

    const aNumeric = /^[0-9]+$/.test(aPart);
    const bNumeric = /^[0-9]+$/.test(bPart);
    if (aNumeric && bNumeric) {
      const aNum = Number(aPart);
      const bNum = Number(bPart);
      if (aNum !== bNum) return aNum > bNum ? 1 : -1;
      continue;
    }
    if (aNumeric && !bNumeric) return -1;
    if (!aNumeric && bNumeric) return 1;
    if (aPart !== bPart) return aPart > bPart ? 1 : -1;
  }
  return 0;
};

const compareSemverLike = (a: string, b: string): number => {
  const parsedA = parseSemver(a);
  const parsedB = parseSemver(b);
  if (parsedA && parsedB) return compareSemver(parsedA, parsedB);
  if (parsedA) return 1;
  if (parsedB) return -1;
  return normalizeSemverInput(a).localeCompare(normalizeSemverInput(b));
};

const sortVersionValuesDesc = (values: string[]): string[] => {
  const unique = Array.from(
    new Set(values.map((value) => String(value || "").trim()).filter(Boolean)),
  );
  unique.sort((a, b) => compareSemverLike(b, a));
  return unique;
};

const normalizeGameVersionForFilter = (value: string): string => {
  const raw = String(value || "").trim();
  if (!raw) return "";
  const match = raw.match(/(\d+\.\d+\.\d+)/);
  return match ? match[1] : raw;
};

const normalizeLLVersionForFilter = (value: string): string =>
  (() => {
    const normalized = normalizeSemverInput(String(value || "").trim());
    if (!normalized) return "";
    const match = normalized.match(
      /(\d+\.\d+\.\d+(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?)/,
    );
    return match ? match[1] : normalized;
  })();

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
    if (sort === "updated")
      return parseUpdatedTime(a.updated) - parseUpdatedTime(b.updated);
    return a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
  });

  if (order === "desc") {
    sorted.reverse();
  }

  return sorted;
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
  const navigate = useNavigate();
  const { currentVersionName } = useCurrentVersion();
  const { ensureInstanceHydrated, getInstanceSnapshot, snapshotRevision } =
    useModIntelligence();
  const [queryInput, setQueryInput] = useState("");
  const [query, setQuery] = useState("");
  const [page, setPage] = useState(1);
  const [allPackages, setAllPackages] = useState<LIPPackageBasicInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [sort, setSort] = useState<LIPSortKey>("hotness");
  const [order, setOrder] = useState<LIPOrderKey>("desc");
  const [selectedGameVersion, setSelectedGameVersion] =
    useState<string>(ALL_GAME_VERSION);
  const [selectedLLVersion, setSelectedLLVersion] =
    useState<string>(ALL_LL_VERSION);
  const [gameVersionOptions, setGameVersionOptions] = useState<string[]>([]);
  const [llVersionOptions, setLLVersionOptions] = useState<string[]>([]);
  const [gameToLLVersions, setGameToLLVersions] = useState<
    Record<string, string[]>
  >({});
  const [currentInstalledLLVersion, setCurrentInstalledLLVersion] =
    useState("");
  const [error, setError] = useState("");
  const [filterContextReady, setFilterContextReady] = useState(false);
  const [developerGuideOpen, setDeveloperGuideOpen] = useState(false);

  const pageRootRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const gameFilterInitializedRef = useRef(false);
  const llFilterInitializedRef = useRef(false);

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

  const loadFilterContext = useCallback(async () => {
    const instanceName = await resolveCurrentInstanceName(currentVersionName);

    let currentGameVersion = "";
    let installedLLVersion = "";
    let mappingGameToLLVersions: Record<string, string[]> = {};

    try {
      const mapping = await fetchLIPLeviLaminaClientMapping();
      mappingGameToLLVersions = mapping.gameToLLVersions || {};
    } catch (err) {
      console.warn("Failed to load LeviLamina game mapping", err);
    }

    if (instanceName) {
      try {
        const meta: any = await GetVersionMeta(instanceName);
        currentGameVersion = normalizeGameVersionForFilter(
          String(meta?.gameVersion || "").trim(),
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

    const normalizedGameToLL = Object.fromEntries(
      Object.entries(mappingGameToLLVersions).map(([gameVersion, llVersions]) => [
        normalizeGameVersionForFilter(String(gameVersion || "").trim()),
        sortVersionValuesDesc(
          Array.isArray(llVersions)
            ? llVersions.map((version) =>
                normalizeLLVersionForFilter(String(version || "").trim()),
              )
            : [],
        ),
      ]),
    ) as Record<string, string[]>;

    const gameOptions = sortVersionValuesDesc(
      Object.keys(normalizedGameToLL).filter(Boolean),
    );
    if (currentGameVersion && !gameOptions.includes(currentGameVersion)) {
      gameOptions.unshift(currentGameVersion);
      if (!Array.isArray(normalizedGameToLL[currentGameVersion])) {
        normalizedGameToLL[currentGameVersion] = [];
      }
    }
    const currentGameHasMappedLLVersions =
      Boolean(currentGameVersion) &&
      Array.isArray(normalizedGameToLL[currentGameVersion]) &&
      normalizedGameToLL[currentGameVersion].length > 0;

    setGameToLLVersions(normalizedGameToLL);
    setGameVersionOptions(gameOptions);
    setCurrentInstalledLLVersion(installedLLVersion);
    setSelectedGameVersion((prev) => {
      if (!gameFilterInitializedRef.current) {
        gameFilterInitializedRef.current = true;
        if (
          currentGameVersion &&
          gameOptions.includes(currentGameVersion) &&
          currentGameHasMappedLLVersions
        ) {
          return currentGameVersion;
        }
        return ALL_GAME_VERSION;
      }
      if (
        prev === ALL_GAME_VERSION &&
        currentGameVersion &&
        gameOptions.includes(currentGameVersion) &&
        currentGameHasMappedLLVersions
      ) {
        return currentGameVersion;
      }
      if (prev === currentGameVersion && !currentGameHasMappedLLVersions) {
        return ALL_GAME_VERSION;
      }
      if (gameOptions.includes(prev)) {
        return prev;
      }
      return ALL_GAME_VERSION;
    });
    setFilterContextReady(true);
  }, [
    currentVersionName,
    ensureInstanceHydrated,
    getInstanceSnapshot,
    snapshotRevision,
  ]);

  useEffect(() => {
    void loadFilterContext();
  }, [loadFilterContext]);

  useEffect(() => {
    if (!filterContextReady) return;

    const allMappedLLVersions = sortVersionValuesDesc(
      Object.values(gameToLLVersions).flatMap((versions) => versions),
    );
    let nextOptions =
      selectedGameVersion === ALL_GAME_VERSION
        ? allMappedLLVersions
        : sortVersionValuesDesc(gameToLLVersions[selectedGameVersion] || []);

    if (
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
    gameToLLVersions,
    currentInstalledLLVersion,
  ]);

  const llCandidatesByGame = useMemo(() => {
    if (selectedGameVersion === ALL_GAME_VERSION) return [];
    return gameToLLVersions[selectedGameVersion] || [];
  }, [selectedGameVersion, gameToLLVersions]);

  const filteredPackages = useMemo(() => {
    const keyword = query.trim().toLowerCase();

    return allPackages.filter((pkg) => {
      const gameFilterActive = selectedGameVersion !== ALL_GAME_VERSION;
      const llFilterActive = selectedLLVersion !== ALL_LL_VERSION;

      if (gameFilterActive && !pkg.llDependencyRanges?.length) {
        return false;
      }

      if (gameFilterActive && !llFilterActive) {
        if (
          !isLeviLaminaRangesCompatibleWithAnyVersion(
            pkg.llDependencyRanges,
            llCandidatesByGame,
          )
        ) {
          return false;
        }
      } else if (gameFilterActive && llFilterActive) {
        if (
          llCandidatesByGame.length > 0 &&
          !llCandidatesByGame.includes(selectedLLVersion)
        ) {
          return false;
        }
        if (
          !isLeviLaminaVersionCompatible(
            selectedLLVersion,
            pkg.llDependencyRanges,
          )
        ) {
          return false;
        }
      } else if (llFilterActive) {
        if (
          !pkg.llDependencyRanges?.length ||
          !isLeviLaminaVersionCompatible(
            selectedLLVersion,
            pkg.llDependencyRanges,
          )
        ) {
          return false;
        }
      }

      if (!keyword) {
        return true;
      }

      const haystacks = [
        pkg.identifier,
        pkg.name,
        pkg.description,
        pkg.author,
        ...pkg.tags,
      ].map((part) => String(part || "").toLowerCase());
      return haystacks.some((part) => part.includes(keyword));
    });
  }, [
    allPackages,
    query,
    selectedGameVersion,
    selectedLLVersion,
    llCandidatesByGame,
  ]);

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
    <PageContainer
      ref={pageRootRef}
      className="min-h-0 !overflow-hidden"
      animate={false}
    >
      <motion.div
        initial={{ opacity: 0, y: -20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4 }}
      >
        <Card className={cn("shrink-0", LAYOUT.GLASS_CARD.BASE)}>
          <CardBody className="p-6 flex flex-col gap-6">
            <PageHeader title={t("lip.title")} />

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
                className="bg-primary-500 hover:bg-primary-500 text-white font-bold shadow-lg shadow-primary-900/20"
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
              <Button
                variant="flat"
                size="sm"
                startContent={<LuBookOpen />}
                onPress={() => setDeveloperGuideOpen(true)}
              >
                {t("lip.guide.open_button")}
              </Button>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <Select
                label={t("lip.game_version_label")}
                placeholder={t("lip.game_version_placeholder")}
                selectedKeys={[selectedGameVersion]}
                onSelectionChange={(keys) => {
                  const value = Array.from(keys)[0] as string;
                  setSelectedGameVersion(value || ALL_GAME_VERSION);
                  setPage(1);
                }}
                size="sm"
                classNames={COMPONENT_STYLES.select}
                items={[
                  { key: ALL_GAME_VERSION, label: t("lip.game_all_versions") },
                  ...gameVersionOptions.map((version) => ({
                    key: version,
                    label: version,
                  })),
                ]}
              >
                {(item) => <SelectItem key={item.key}>{item.label}</SelectItem>}
              </Select>

              <Select
                label={t("lip.ll_version_label")}
                placeholder={t("lip.ll_version_placeholder")}
                selectedKeys={[selectedLLVersion]}
                onSelectionChange={(keys) => {
                  const value = Array.from(keys)[0] as string;
                  setSelectedLLVersion(value || ALL_LL_VERSION);
                  setPage(1);
                }}
                size="sm"
                classNames={COMPONENT_STYLES.select}
                items={[
                  { key: ALL_LL_VERSION, label: t("lip.ll_all_versions") },
                  ...llVersionOptions.map((version) => ({
                    key: version,
                    label: version,
                  })),
                ]}
              >
                {(item) => <SelectItem key={item.key}>{item.label}</SelectItem>}
              </Select>

              <Select
                label={t("lip.sort_by")}
                placeholder={t("lip.select_sort")}
                selectedKeys={[sort]}
                onChange={(e) => handleSortChange(e.target.value as LIPSortKey)}
                size="sm"
                classNames={COMPONENT_STYLES.select}
              >
                <SelectItem key="hotness">
                  {t("lip.sort_options.hotness")}
                </SelectItem>
                <SelectItem key="updated">
                  {t("lip.sort_options.updated")}
                </SelectItem>
                <SelectItem key="name">{t("lip.sort_options.name")}</SelectItem>
              </Select>

              <Select
                label={t("lip.order_by")}
                placeholder={t("lip.select_order")}
                selectedKeys={[order]}
                onChange={(e) => {
                  setOrder(e.target.value as LIPOrderKey);
                  setPage(1);
                }}
                size="sm"
                classNames={COMPONENT_STYLES.select}
              >
                <SelectItem key="desc">{t("lip.order_options.desc")}</SelectItem>
                <SelectItem key="asc">{t("lip.order_options.asc")}</SelectItem>
              </Select>
            </div>

            {error ? (
              <div className="text-sm text-danger-500" role="alert">
                {error}
              </div>
            ) : null}
          </CardBody>
        </Card>
      </motion.div>

      <motion.div
        className="flex-1 min-h-0 flex flex-col"
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4, delay: 0.1 }}
      >
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
                          navigate(
                            `/lip/package/${encodeURIComponent(pkg.identifier)}`,
                          )
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
                              {t("lip.by_author_inline", {
                                author: pkg.author || t("common.unknown"),
                              })}
                            </span>
                          </div>

                          <p className="text-xs sm:text-sm text-default-500 dark:text-zinc-400 line-clamp-2 w-full">
                            {pkg.description || t("lip.no_description")}
                          </p>

                          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-default-400 mt-1">
                            <div
                              className="flex items-center gap-1"
                              title={t("lip.sort_options.hotness")}
                            >
                              <LuFlame className="text-orange-500" />
                              <span>{pkg.hotness}</span>
                            </div>
                            <div
                              className="flex items-center gap-1"
                              title={t("lip.sort_options.updated")}
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
                      "bg-primary-500 hover:bg-primary-500 shadow-lg shadow-primary-900/20 font-bold",
                  }}
                />
              </div>
            )}
          </CardBody>
        </Card>
      </motion.div>

      <UnifiedModal
        size="lg"
        isOpen={developerGuideOpen}
        onOpenChange={setDeveloperGuideOpen}
        type="primary"
        icon={<LuBookOpen size={24} />}
        title={t("lip.guide.title")}
        isDismissable
        footer={
          <>
            <Button
              variant="light"
              radius="full"
              onPress={() => setDeveloperGuideOpen(false)}
            >
              {t("lip.guide.close_button")}
            </Button>
            <Button
              color="primary"
              radius="full"
              startContent={<LuExternalLink />}
              onPress={() => void Browser.OpenURL(LIP_DEVELOPER_GUIDE_URL)}
            >
              {t("lip.guide.docs_button")}
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-4 text-sm leading-6 text-default-600 dark:text-zinc-300">
          <p>{t("lip.guide.description")}</p>
          <p>{t("lip.guide.manifest_hint")}</p>
          <p className="font-medium text-default-800 dark:text-zinc-100">
            {t("lip.guide.variant_hint")}
          </p>
        </div>
      </UnifiedModal>
    </PageContainer>
  );
};

export default LIPPage;
