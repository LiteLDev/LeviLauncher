import React from "react";
import { getPlayerGamertagMap } from "@/utils/content";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { cn } from "@/utils/cn";
import { PageHeader } from "@/components/PageHeader";
import {
  Button,
  Card,
  CardBody,
  CardHeader,
  Tabs,
  Tab,
  Input,
  Chip,
  Dropdown,
  DropdownTrigger,
  DropdownMenu,
  DropdownItem,
  addToast,
} from "@heroui/react";
import { motion } from "framer-motion";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { compareVersions } from "@/utils/version";
import { FaSortAmountDown, FaSortAmountUp } from "react-icons/fa";
import {
  readCurrentVersionName,
  saveCurrentVersionName,
} from "@/utils/currentVersion";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";

export const VersionSelectPage: React.FC<{ refresh?: () => void }> = (
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
  const [sortBy, setSortBy] = React.useState<"version" | "name">("version");
  const [sortAsc, setSortAsc] = React.useState<boolean>(false);
  const [logoMap, setLogoMap] = React.useState<Map<string, string>>(new Map());
  const [isAnimating, setIsAnimating] = React.useState(true);
  const navigate = useNavigate();
  const { t } = useTranslation();
  const hasBackend = minecraft !== undefined;

  React.useEffect(() => {
    if (hasBackend) {
      const listFn = minecraft?.ListVersionMetas;
      if (typeof listFn === "function") {
        listFn().then((metas: any[]) => {
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
            const getter = minecraft?.GetVersionLogoDataUrl;
            if (typeof getter === "function") {
              const names = Array.from(newLocalVersionMap.keys());
              Promise.all(
                names.map((n) =>
                  getter(n).then((u: string) => [n, String(u || "")] as const),
                ),
              ).then((entries) => {
                const m = new Map<string, string>();
                entries.forEach(([n, u]) => {
                  if (u) m.set(n, u);
                });
                setLogoMap(m);
              });
            } else {
              setLogoMap(new Map());
            }
          } catch {
            setLogoMap(new Map());
          }
        });
      }
    }
  }, [hasBackend]);

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
      addToast({
        title: t("common.success"),
        description: t("launcherpage.currentVersion") + ": " + name,
        color: "primary",
      });
    }
    try {
      props.refresh && props.refresh();
    } catch {}
  };

  const openEditFor = React.useCallback(
    (name: string) => {
      navigate("/versionSettings", { state: { name, returnTo: "/versions" } });
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
          initial={{ y: -20, opacity: 0 }}
          animate={{ y: 0, opacity: 1 }}
          transition={{ duration: 0.4, ease: "easeOut" }}
          onAnimationComplete={() => setIsAnimating(false)}
        >
          <Card className={cn("w-full", LAYOUT.GLASS_CARD.BASE)}>
            <CardHeader className="flex flex-col gap-6 p-6">
              <PageHeader
                className="w-full"
                title={t("launcherpage.version_select.title")}
                titleClassName="text-left pb-1"
              />
              <div className="flex w-full flex-wrap items-center gap-3">
                <Tabs
                  aria-label="Filter versions"
                  selectedKey={activeTab}
                  onSelectionChange={(k) => setActiveTab(k as any)}
                  variant="solid"
                  classNames={COMPONENT_STYLES.tabs}
                >
                  <Tab key="all" title={t("versions.tab.all")} />
                  <Tab key="release" title={t("versions.tab.release")} />
                  <Tab key="preview" title={t("versions.tab.preview")} />
                </Tabs>
                <div className="flex-1 min-w-[200px]">
                  <Input
                    value={query}
                    onValueChange={setQuery}
                    placeholder={t("common.search") as string}
                    variant="bordered"
                    size="sm"
                    classNames={COMPONENT_STYLES.input}
                  />
                </div>
                <div className="flex items-center gap-2">
                  <div className="min-w-[140px]">
                    <Dropdown classNames={COMPONENT_STYLES.dropdown}>
                      <DropdownTrigger>
                        <Button
                          variant="flat"
                          size="sm"
                          startContent={
                            sortAsc ? <FaSortAmountDown /> : <FaSortAmountUp />
                          }
                          className={cn(
                            COMPONENT_STYLES.dropdownTriggerButton,
                            "w-full justify-between",
                          )}
                        >
                          {sortBy === "name"
                            ? sortAsc
                              ? t("versions.sort.name")
                              : t("versions.sort.name_za")
                            : sortAsc
                              ? t("versions.sort.version_old_new")
                              : t("versions.sort.version")}
                        </Button>
                      </DropdownTrigger>
                      <DropdownMenu
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
                        <DropdownItem
                          key="version-desc"
                          startContent={<FaSortAmountUp />}
                        >
                          {t("versions.sort.version")}
                        </DropdownItem>
                        <DropdownItem
                          key="version-asc"
                          startContent={<FaSortAmountDown />}
                        >
                          {t("versions.sort.version_old_new")}
                        </DropdownItem>
                        <DropdownItem
                          key="name-asc"
                          startContent={<FaSortAmountDown />}
                        >
                          {t("versions.sort.name")}
                        </DropdownItem>
                        <DropdownItem
                          key="name-desc"
                          startContent={<FaSortAmountUp />}
                        >
                          {t("versions.sort.name_za")}
                        </DropdownItem>
                      </DropdownMenu>
                    </Dropdown>
                  </div>
                </div>
                <Chip variant="flat" color="default">
                  {flatItems.length}
                </Chip>
              </div>
            </CardHeader>
          </Card>
        </motion.div>

        <motion.div
          className="grid gap-4 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4"
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
          {flatItems.map((it) => (
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
                isPressable
                onPress={() => handleSelectVersion(it.name)}
                className={cn(
                  "w-full h-full transition-all",
                  LAYOUT.GLASS_CARD.BASE,
                  "border-2 border-solid",
                  selectedVersionName === it.name
                    ? "border-primary-600 dark:border-primary-500 bg-primary-500/5 dark:bg-primary-500/10 shadow-primary-500/20"
                    : "border-transparent hover:border-default-200 dark:hover:border-zinc-700",
                )}
              >
                <CardBody className="p-4 flex flex-col gap-1">
                  <div className="flex items-center justify-between w-full">
                    <div className="font-bold text-lg truncate">{it.name}</div>
                    <div className="flex items-center gap-2">
                      {it.isPreview ? (
                        <Chip
                          size="sm"
                          color="warning"
                          variant="flat"
                          className="shrink-0"
                        >
                          Preview
                        </Chip>
                      ) : (
                        <Chip
                          size="sm"
                          color="success"
                          variant="flat"
                          className="shrink-0"
                        >
                          Release
                        </Chip>
                      )}
                      <Button
                        isIconOnly
                        size="sm"
                        variant="light"
                        onPress={() => {
                          openEditFor(it.name);
                        }}
                        aria-label="settings"
                        className="shrink-0"
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
                  <div className="flex items-center gap-2 text-default-500 dark:text-zinc-400 shrink-0 text-sm">
                    {(() => {
                      const u = logoMap.get(it.name);
                      return u ? (
                        <img src={u} alt="logo" className="h-4 w-4 rounded" />
                      ) : (
                        <div className="h-4 w-4 rounded bg-default-200" />
                      );
                    })()}
                    <span>
                      Vanilla{" "}
                      {it.version || t("launcherpage.version_select.unknown")}
                    </span>
                  </div>
                </CardBody>
              </Card>
            </motion.div>
          ))}
        </motion.div>
      </PageContainer>
    </>
  );
};
