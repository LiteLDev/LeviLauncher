import {
  Button,
  Card,
  Chip,
  CloseButton,
  Dropdown,
  InputGroup,
  Label,
  Tabs,
  TextField,
  Spinner,
  Tooltip,
  toast,
} from "@heroui/react";

import React from "react";
import { Button as AriaButton } from "react-aria-components";
import { getPlayerGamertagMap } from "@/utils/content";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { cn } from "@/utils/cn";

import { motion } from "framer-motion";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { compareVersions } from "@/utils/version";
import { FaSearch, FaSortAmountDown, FaSortAmountUp } from "react-icons/fa";
import { ROUTES } from "@/constants/routes";
import {
  readCurrentVersionName,
  saveCurrentVersionName,
} from "@/utils/currentVersion";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import {
  GetVersionLogoDataUrl,
  ListVersionMetas,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/versionservice";

export const InstanceSelectPage: React.FC<{ refresh?: () => void }> = (
  props,
) => {
  const [localVersionMap, setLocalVersionMap] = React.useState<
    Map<string, any>
  >(new Map());
  const [localVersionsMap, setLocalVersionsMap] = React.useState<
    Map<string, string[]>
  >(new Map());
  const [selectedVersionName, setSelectedVersionName] =
    React.useState<string>("");
  const [persistedName, setPersistedName] = React.useState<string>("");
  const [activeTab, setActiveTab] = React.useState<
    "all" | "release" | "preview"
  >("all");
  const [query, setQuery] = React.useState("");
  const [loading, setLoading] = React.useState(true);
  const [loadError, setLoadError] = React.useState(false);
  const [loadAttempt, setLoadAttempt] = React.useState(0);
  const [sortBy, setSortBy] = React.useState<"version" | "name">("version");
  const [sortAsc, setSortAsc] = React.useState<boolean>(false);
  const [logoMap, setLogoMap] = React.useState<Map<string, string>>(new Map());
  const [isAnimating, setIsAnimating] = React.useState(true);
  const navigate = useNavigate();
  const { t } = useTranslation();
  const hasBackend = minecraft !== undefined;

  React.useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setLoadError(false);
    if (hasBackend) {
      const listFn = ListVersionMetas as any;
      if (typeof listFn === "function") {
        listFn().then((metas: any[]) => {
          if (cancelled) return;
          const newLocalVersionMap = new Map();
          const newLocalVersionsMap = new Map();
          metas?.forEach((m: any) => {
            const name = String(m?.name || "");
            const gameVersion = String(m?.gameVersion || "");
            const type = String(m?.type || "release");
            const isPreview = type.toLowerCase() === "preview";
            const enableIsolation = !!m?.enableIsolation;
            const enableConsole = !!m?.enableConsole;
            const enableEditorMode = !!m?.enableEditorMode;
            const lv: any = {
              name,
              version: gameVersion,
              isPreview,
              type,
              enableIsolation,
              enableConsole,
              enableEditorMode,
              isLaunched: false,
              isPreLoader: false,
            };
            if (name) newLocalVersionMap.set(name, lv);
            if (gameVersion) {
              if (!newLocalVersionsMap.has(gameVersion))
                newLocalVersionsMap.set(gameVersion, []);
              if (!newLocalVersionsMap.get(gameVersion)?.includes(name))
                newLocalVersionsMap.get(gameVersion)?.push(name);
            }
          });
          setLocalVersionMap(newLocalVersionMap);
          setLocalVersionsMap(newLocalVersionsMap);
          const saved = readCurrentVersionName();
          const useName =
            saved && newLocalVersionMap.has(saved)
              ? saved
              : Array.from(newLocalVersionMap.keys())[0] || "";
          setSelectedVersionName(useName);
          setPersistedName(saved || "");
          try {
            const getter = GetVersionLogoDataUrl as any;
            if (typeof getter === "function") {
              const names = Array.from(newLocalVersionMap.keys());
              Promise.all(
                names.map((n) =>
                  getter(n).then((u: string) => [n, String(u || "")] as const).catch(() => [n, ""] as const),
                ),
              ).then((entries) => {
                const m = new Map<string, string>();
                entries.forEach(([n, u]) => {
                  if (u) m.set(n, u);
                });
                if (!cancelled) setLogoMap(m);
              });
            } else {
              setLogoMap(new Map());
            }
          } catch {
            setLogoMap(new Map());
          }
        }).catch((error: unknown) => {
          console.error("Failed to load instances", error);
          if (!cancelled) setLoadError(true);
        }).finally(() => {
          if (!cancelled) setLoading(false);
        });
      } else {
        setLoading(false);
        setLoadError(true);
      }
    } else {
      setLoading(false);
    }
    return () => { cancelled = true; };
  }, [hasBackend, loadAttempt]);

  const flatItems = React.useMemo(() => {
    const list = (
      Array.from(localVersionMap.values()) as Array<{
        name: string;
        version: string;
        isPreview: boolean;
      }>
    )
      .filter((it) => {
        if (activeTab === "release") return !it.isPreview;
        if (activeTab === "preview") return it.isPreview;
        return true;
      })
      .filter((it) => {
        if (!query.trim()) return true;
        const q = query.trim().toLowerCase();
        return (
          it.name.toLowerCase().includes(q) ||
          String(it.version || "")
            .toLowerCase()
            .includes(q)
        );
      })
      .sort((a, b) => {
        if (sortBy === "name") {
          const cmp = String(a.name).localeCompare(String(b.name));
          return sortAsc ? cmp : -cmp;
        }
        const av = String(a.version || "0");
        const bv = String(b.version || "0");
        const cmp = compareVersions(av, bv);
        return sortAsc ? cmp : -cmp;
      });
    return list;
  }, [localVersionMap, activeTab, query, sortBy, sortAsc, compareVersions]);

  const listVariants = React.useMemo(
    () => ({
      hidden: {},
      show: {
        transition: { staggerChildren: 0.05, delayChildren: 0.05 },
      },
    }),
    [],
  );

  const itemVariants = React.useMemo(
    () => ({
      hidden: { opacity: 0, y: 6 },
      show: { opacity: 1, y: 0 },
    }),
    [],
  );

  const handleSelectVersion = (name: string) => {
    if (name) {
      saveCurrentVersionName(name);
      setSelectedVersionName(name);
      setPersistedName(name);
      toast(t("common.success"), {
        description: t("launcherpage.currentVersion") + ": " + name,
        variant: "accent",
        timeout: 2000,
      });
    }
    try {
      props.refresh && props.refresh();
    } catch {}
  };

  const openEditFor = React.useCallback(
    (name: string) => {
      navigate(ROUTES.instanceSettings, {
        state: { name, returnTo: ROUTES.instances },
      });
    },
    [navigate],
  );

  return (
    <>
      <PageContainer
        className={cn("relative", isAnimating && "overflow-hidden")}
        animate={false}
      >
        <motion.div
          className="shrink-0"
          initial={{ y: -20, opacity: 0 }}
          animate={{ y: 0, opacity: 1 }}
          transition={{ duration: 0.4, ease: "easeOut" }}
          onAnimationComplete={() => setIsAnimating(false)}
        >
          <Card className={cn("w-full", LAYOUT.GLASS_CARD.BASE)}>
            <Card.Header className="px-4 py-3">
              <h1 className="sr-only">{t("launcherpage.version_select.title")}</h1>
              <div className="flex w-full flex-wrap items-center gap-3">
                <Tabs
                  selectedKey={activeTab}
                  onSelectionChange={(k) => setActiveTab(k as any)}
                  variant="primary"
                  className="w-auto shrink-0"
                >
                    <Tabs.List
                      aria-label={"Filter versions"}
                      className={cn(COMPONENT_STYLES.tabs.tabList, "w-max flex-nowrap")}
                    >
                      <Tabs.Tab
                        key="all"
                        id={"all"}
                        className={cn(COMPONENT_STYLES.tabs.tabContent, "w-auto flex-none whitespace-nowrap px-3")}
                      >
                        {t("versions.tab.all")}
                        <Tabs.Indicator
                          className={COMPONENT_STYLES.tabs.cursor}
                        />
                      </Tabs.Tab>
                      <Tabs.Tab
                        key="release"
                        id={"release"}
                        className={cn(COMPONENT_STYLES.tabs.tabContent, "w-auto flex-none whitespace-nowrap px-3")}
                      >
                        {t("versions.tab.release")}
                        <Tabs.Indicator
                          className={COMPONENT_STYLES.tabs.cursor}
                        />
                      </Tabs.Tab>
                      <Tabs.Tab
                        key="preview"
                        id={"preview"}
                        className={cn(COMPONENT_STYLES.tabs.tabContent, "w-auto flex-none whitespace-nowrap px-3")}
                      >
                        {t("versions.tab.preview")}
                        <Tabs.Indicator
                          className={COMPONENT_STYLES.tabs.cursor}
                        />
                      </Tabs.Tab>
                    </Tabs.List>
                </Tabs>
                <div className="min-w-0 flex-[1_1_12rem]">
                  <TextField
                    aria-label={t("common.search_placeholder") as string}
                    className={cn("group w-full min-w-0", COMPONENT_STYLES.input.mainWrapper)}
                    value={query}
                    onChange={setQuery}
                  >
                    <InputGroup
                      className={cn(
                        COMPONENT_STYLES.input.inputWrapper,
                        COMPONENT_STYLES.input.innerWrapper,
                        "rounded-full",
                        "min-h-8 text-sm",
                      )}
                    >
                      <InputGroup.Prefix>
                        {<FaSearch className="text-muted" />}
                      </InputGroup.Prefix>
                      <InputGroup.Input
                        placeholder={t("common.search_placeholder") as string}
                        className={COMPONENT_STYLES.input.input}
                      />
                      <InputGroup.Suffix>
                        {query && (
                          <CloseButton
                            aria-label={t("audit.primary.clear_search")}
                            onPress={() => setQuery("")}
                            className={COMPONENT_STYLES.input.clearButton}
                          />
                        )}
                      </InputGroup.Suffix>
                    </InputGroup>
                  </TextField>
                </div>
                <div className="shrink-0">
                    <Dropdown>
                      <Button
                        size="sm"
                        variant={"secondary"}
                        className={cn(
                          COMPONENT_STYLES.dropdownTriggerButton,
                          "whitespace-nowrap rounded-full",
                        )}
                      >
                        {sortAsc ? <FaSortAmountDown /> : <FaSortAmountUp />}
                        {sortBy === "name"
                          ? sortAsc
                            ? t("versions.sort.name")
                            : t("versions.sort.name_za")
                          : sortAsc
                            ? t("versions.sort.version_old_new")
                            : t("versions.sort.version")}
                      </Button>
                      <Dropdown.Popover
                        className={COMPONENT_STYLES.dropdown.content}
                      >
                        <Dropdown.Menu
                          selectionMode="single"
                          selectedKeys={
                            new Set([`${sortBy}-${sortAsc ? "asc" : "desc"}`])
                          }
                          onSelectionChange={(keys) => {
                            const val = Array.from(keys)[0] as string;
                            const [k, order] = val.split("-");
                            setSortBy(k as "version" | "name");
                            setSortAsc(order === "asc");
                          }}
                        >
                          <Dropdown.Item
                            key="version-desc"
                            id={"version-desc"}
                            textValue={t("versions.sort.version")}
                          >
                            {<FaSortAmountUp />}
                            <Label>{t("versions.sort.version")}</Label>
                            <Dropdown.ItemIndicator />
                          </Dropdown.Item>
                          <Dropdown.Item
                            key="version-asc"
                            id={"version-asc"}
                            textValue={t("versions.sort.version_old_new")}
                          >
                            {<FaSortAmountDown />}
                            <Label>{t("versions.sort.version_old_new")}</Label>
                            <Dropdown.ItemIndicator />
                          </Dropdown.Item>
                          <Dropdown.Item
                            key="name-asc"
                            id={"name-asc"}
                            textValue={t("versions.sort.name")}
                          >
                            {<FaSortAmountDown />}
                            <Label>{t("versions.sort.name")}</Label>
                            <Dropdown.ItemIndicator />
                          </Dropdown.Item>
                          <Dropdown.Item
                            key="name-desc"
                            id={"name-desc"}
                            textValue={t("versions.sort.name_za")}
                          >
                            {<FaSortAmountUp />}
                            <Label>{t("versions.sort.name_za")}</Label>
                            <Dropdown.ItemIndicator />
                          </Dropdown.Item>
                        </Dropdown.Menu>
                      </Dropdown.Popover>
                    </Dropdown>
                </div>
              </div>
            </Card.Header>
          </Card>
        </motion.div>

        <motion.div
          className="grid gap-3 grid-cols-[repeat(auto-fill,minmax(min(100%,20rem),1fr))]"
          layout
          variants={listVariants}
          initial="hidden"
          animate="show"
          onLayoutAnimationStart={() => setIsAnimating(true)}
          onLayoutAnimationComplete={() => setIsAnimating(false)}
          transition={{
            layout: { duration: 0.35, ease: [0.22, 0.61, 0.36, 1] },
          }}
        >
          {(loading || loadError || flatItems.length === 0) && (
            <Card className={cn(LAYOUT.GLASS_CARD.BASE, "col-span-full")}>
              <Card.Content className="flex flex-col items-center gap-3 px-6 py-12 text-center">
                {loading && <Spinner />}
                <p role={loadError ? "alert" : "status"} className="font-semibold text-foreground">
                  {t(loading ? "common.loading" : loadError ? "audit.primary.instances.failed" : localVersionMap.size === 0 ? "audit.primary.instances.empty" : "audit.primary.instances.no_matches")}
                </p>
                {!loading && (loadError ? (
                  <Button variant="secondary" onPress={() => setLoadAttempt((attempt) => attempt + 1)}>{t("download_manager.actions.retry")}</Button>
                ) : localVersionMap.size === 0 ? (
                  <>
                    <p className="text-sm text-muted">{t("audit.primary.instances.empty_description")}</p>
                    <div className="flex flex-wrap justify-center gap-2">
                      <Button variant="primary" onPress={() => navigate(ROUTES.download)}>{t("audit.primary.download_minecraft")}</Button>
                      <Button variant="secondary" onPress={() => navigate(ROUTES.install, { state: { returnTo: ROUTES.instances } })}>{t("audit.primary.local_install")}</Button>
                    </div>
                  </>
                ) : (
                  <Button variant="secondary" onPress={() => { setQuery(""); setActiveTab("all"); }}>{t("audit.primary.clear_filters")}</Button>
                ))}
              </Card.Content>
            </Card>
          )}
          {!loading && !loadError && flatItems.map((it) => (
            <motion.div
              key={it.name}
              layout
              variants={itemVariants}
              initial="hidden"
              animate="show"
              transition={{
                layout: { duration: 0.35, ease: [0.22, 0.61, 0.36, 1] },
              }}
              whileHover={{ scale: 1.02 }}
              whileTap={{ scale: 0.98 }}
              className="w-full min-w-0"
            >
              <Card
                className={cn(
                  "relative w-full h-full transition-all",
                  LAYOUT.GLASS_CARD.BASE,
                  "border-2 border-solid",
                  selectedVersionName === it.name
                    ? "border-brand-600 dark:border-brand-500 bg-brand-500/5 dark:bg-brand-500/10 shadow-brand-500/20"
                    : "border-transparent hover:border-border dark:hover:border-zinc-700",
                )}
              >
                <Card.Content className="p-4 flex flex-col gap-1">
                  <Tooltip>
                  <AriaButton
                    type="button"
                    aria-label={it.name}
                    aria-pressed={selectedVersionName === it.name}
                    className="absolute inset-0 z-10 cursor-pointer rounded-[inherit] focus-visible:outline-2 focus-visible:outline-focus"
                    onPress={() => handleSelectVersion(it.name)}
                  />
                  <Tooltip.Content>{it.name}</Tooltip.Content>
                  </Tooltip>
                  <div className="flex items-center justify-between gap-2 w-full min-w-0">
                    <div className="font-bold text-lg truncate min-w-0 flex-1">{it.name}</div>
                    <div className="flex shrink-0 items-center gap-2">
                      {activeTab === "all" && (it.isPreview ? (
                        <Chip
                          size="sm"
                          variant="soft"
                          color={"warning"}
                          className={"shrink-0"}
                        >
                          <Chip.Label>{t("versions.tab.preview")}</Chip.Label>
                        </Chip>
                      ) : (
                        <Chip
                          size="sm"
                          variant="soft"
                          color={"success"}
                          className={"shrink-0"}
                        >
                          <Chip.Label>{t("versions.tab.release")}</Chip.Label>
                        </Chip>
                      ))}
                      <Button
                        isIconOnly
                        size="sm"
                        onPress={() => {
                          openEditFor(it.name);
                        }}
                        aria-label={t("audit.primary.instances.settings", { name: it.name })}
                        variant={"ghost"}
                        className={"relative z-20 shrink-0"}
                      >
                        <svg
                          width="18"
                          height="18"
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          strokeWidth="2"
                          strokeLinecap="round"
                          strokeLinejoin="round"
                        >
                          <path d="M12 15.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7z" />
                          <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33h0a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51h0a1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82v0a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
                        </svg>
                      </Button>
                    </div>
                  </div>
                  <div className="flex min-w-0 items-center gap-2 text-muted dark:text-zinc-400 text-sm">
                    {(() => {
                      const u = logoMap.get(it.name);
                      return u ? (
                        <img src={u} alt="" className="h-4 w-4 shrink-0 rounded" />
                      ) : (
                        <div className="h-4 w-4 shrink-0 rounded bg-surface-tertiary" />
                      );
                    })()}
                    <span className="truncate" title={it.version}>
                      Vanilla{" "}
                      {it.version || t("launcherpage.version_select.unknown")}
                    </span>
                  </div>
                </Card.Content>
              </Card>
            </motion.div>
          ))}
        </motion.div>
      </PageContainer>
    </>
  );
};
