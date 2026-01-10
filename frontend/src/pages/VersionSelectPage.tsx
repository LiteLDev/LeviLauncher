import React from "react";
import { BaseModal, BaseModalHeader, BaseModalBody, BaseModalFooter } from "../components/BaseModal";
import {
  Button,
  Card,
  CardBody,
  CardHeader,
  Tabs,
  Tab,
  Input,
  Chip,
  Select,
  SelectItem,
  ModalContent,
  useDisclosure,
} from "@heroui/react";
import { motion } from "framer-motion";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { compareVersions } from "../utils/version";
import {
  readCurrentVersionName,
  saveCurrentVersionName,
} from "../utils/currentVersion";
import * as minecraft from "../../bindings/github.com/liteldev/LeviLauncher/minecraft";

export const VersionSelectPage: React.FC<{ refresh?: () => void }> = (
  props
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
  const navigate = useNavigate();
  const { t } = useTranslation();
  const hasBackend = minecraft !== undefined;
  const guardBypassRef = React.useRef<boolean>(false);
  const userChangedRef = React.useRef<boolean>(false);
  const unsavedDisclosure = useDisclosure();
  const navAttemptRef = React.useRef<{
    type: "push" | "replace" | "";
    args: any[];
    push?: typeof window.history.pushState;
    replace?: typeof window.history.replaceState;
  } | null>(null);

  const commitNavAttempt = React.useCallback(() => {
    const a = navAttemptRef.current;
    if (!a) return;
    try {
      const raw = String((a.args && a.args[2]) || "");
      let target = "";
      if (raw) {
        try {
          const u = new URL(raw, window.location.href);
          target = u.hash ? u.hash.slice(1) : u.pathname || raw;
        } catch {
          target = raw.startsWith("#") ? raw.slice(1) : raw;
        }
      }

      if (!target) target = "/";
      guardBypassRef.current = true;
      navigate(target, { replace: a.type === "replace" });
    } finally {
      navAttemptRef.current = null;
    }
  }, [navigate]);

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
                  getter(n).then((u: string) => [n, String(u || "")] as const)
                )
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
    []
  );

  const itemVariants = React.useMemo(
    () => ({
      hidden: { opacity: 0, y: 6 },
      show: { opacity: 1, y: 0 },
    }),
    []
  );

  const handleConfirm = () => {
    const name = selectedVersionName;
    if (name) {
      saveCurrentVersionName(name);
    }
    try {
      props.refresh && props.refresh();
    } catch {}
    setPersistedName(name || persistedName);
    guardBypassRef.current = true;
    navigate("/");
  };

  const openEditFor = React.useCallback(
    (name: string) => {
      navigate("/version-settings", { state: { name, returnTo: "/versions" } });
    },
    [navigate]
  );

  const selectVersionByUser = (name: string) => {
    userChangedRef.current = true;
    setSelectedVersionName(name);
  };

  const guardActive = React.useMemo(() => {
    if (guardBypassRef.current) return false;
    return (
      userChangedRef.current &&
      !!persistedName &&
      selectedVersionName !== persistedName
    );
  }, [persistedName, selectedVersionName]);

  React.useEffect(() => {
    const originalPush = window.history.pushState.bind(window.history);
    const originalReplace = window.history.replaceState.bind(window.history);
    if (!navAttemptRef.current)
      navAttemptRef.current = {
        type: "",
        args: [],
        push: originalPush,
        replace: originalReplace,
      };
    else {
      navAttemptRef.current.push = originalPush;
      navAttemptRef.current.replace = originalReplace;
    }

    const openModalWithAttempt = (type: "push" | "replace", args: any[]) => {
      if (!navAttemptRef.current)
        navAttemptRef.current = {
          type,
          args,
          push: originalPush,
          replace: originalReplace,
        };
      else {
        navAttemptRef.current.type = type;
        navAttemptRef.current.args = args;
      }
      try {
        unsavedDisclosure.onOpen();
      } catch {}
    };

    const interceptPush: typeof window.history.pushState = function (
      ...args: any[]
    ) {
      if (!guardActive || guardBypassRef.current)
        return originalPush.apply(
          window.history,
          args as [any, string, (string | URL | null)?]
        );
      openModalWithAttempt("push", args);
      return;
    } as any;

    const interceptReplace: typeof window.history.replaceState = function (
      ...args: any[]
    ) {
      if (!guardActive || guardBypassRef.current)
        return originalPush.apply(
          window.history,
          args as [any, string, (string | URL | null)?]
        );
      openModalWithAttempt("replace", args);
      return;
    } as any;

    if (guardActive) {
      (window.history as any).pushState = interceptPush as any;
      (window.history as any).replaceState = interceptReplace as any;
    }

    return () => {
      (window.history as any).pushState = originalPush as any;
      (window.history as any).replaceState = originalReplace as any;
    };
  }, [guardActive, unsavedDisclosure]);

  React.useEffect(() => {
    const handler = (e: BeforeUnloadEvent) => {
      if (
        userChangedRef.current &&
        !!persistedName &&
        selectedVersionName !== persistedName
      ) {
        e.preventDefault();
        e.returnValue = "";
      }
    };
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [persistedName, selectedVersionName]);

  return (
    <>
      <div className="w-full h-full max-w-[100vw] flex flex-col overflow-x-hidden gutter-stable overflow-auto no-scrollbar">
        <div className="px-3 sm:px-5 lg:px-8 py-3 sm:py-4 lg:py-6 w-full flex flex-col flex-1 min-h-0">
          <motion.div
            className="fixed inset-x-0 top-[84px] z-40"
            initial={{ y: -8, opacity: 0 }}
            animate={{ y: 0, opacity: 1 }}
            transition={{ duration: 0.18, ease: [0.16, 0.84, 0.44, 1] }}
          >
            <Card className="rounded-3xl shadow-xl mx-3 sm:mx-5 lg:mx-8 bg-white/60 dark:bg-black/30 backdrop-blur-md border border-white/30">
              <CardHeader className="flex flex-col gap-3 p-3 sm:p-4">
                <div className="flex w-full items-center justify-between gap-2">
                  <div className="text-2xl font-bold">
                    {t("launcherpage.version_select.title", {
                      defaultValue: "选择版本",
                    })}
                  </div>
                  <div className="flex items-center gap-2">
                    <Button
                      variant="light"
                      onPress={() => {
                        guardBypassRef.current = true;
                        navigate("/");
                      }}
                    >
                      {t("common.cancel", { defaultValue: "取消" })}
                    </Button>
                    <Button color="primary" onPress={handleConfirm}>
                      {t("common.ok", { defaultValue: "确定" })}
                    </Button>
                  </div>
                </div>
                <div className="flex w-full flex-wrap items-center gap-3">
                  <Tabs
                    aria-label="Filter versions"
                    selectedKey={activeTab}
                    onSelectionChange={(k) => setActiveTab(k as any)}
                    variant="solid"
                    color="primary"
                    classNames={{ tabList: "bg-default-100 rounded-xl px-1" }}
                  >
                    <Tab
                      key="all"
                      title={t("versions.tab.all", { defaultValue: "All" })}
                    />
                    <Tab
                      key="release"
                      title={t("versions.tab.release", {
                        defaultValue: "Release",
                      })}
                    />
                    <Tab
                      key="preview"
                      title={t("versions.tab.preview", {
                        defaultValue: "Preview",
                      })}
                    />
                  </Tabs>
                  <div className="flex-1 min-w-[200px]">
                    <Input
                      value={query}
                      onValueChange={setQuery}
                      placeholder={
                        t("common.search", {
                          defaultValue: "Search name or version",
                        }) as string
                      }
                      variant="bordered"
                      size="sm"
                    />
                  </div>
                  <div className="flex items-center gap-2">
                    <div className="min-w-[200px]">
                      <Select
                        size="sm"
                        label={
                          t("versions.sort.label", {
                            defaultValue: "Sort by",
                          }) as string
                        }
                        selectedKeys={new Set([sortBy])}
                        onSelectionChange={(keys) => {
                          const v = Array.from(keys as Set<string>)[0] as any;
                          if (v === "version" || v === "name") setSortBy(v);
                        }}
                      >
                        <SelectItem key="version">
                          {sortAsc
                            ? t("versions.sort.version_old_new", {
                                defaultValue: "Version (old → new)",
                              })
                            : t("versions.sort.version", {
                                defaultValue: "Version (new → old)",
                              })}
                        </SelectItem>
                        <SelectItem key="name">
                          {sortAsc
                            ? t("versions.sort.name_za", {
                                defaultValue: "Name (Z → A)",
                              })
                            : t("versions.sort.name", {
                                defaultValue: "Name (A → Z)",
                              })}
                        </SelectItem>
                      </Select>
                    </div>
                    <Button
                      size="sm"
                      variant="flat"
                      onPress={() => setSortAsc((v) => !v)}
                      className="min-w-0 px-2 sm:px-3"
                      aria-label={
                        (sortAsc
                          ? t("versions.sort.order_asc", {
                              defaultValue: "升序",
                            })
                          : t("versions.sort.order_desc", {
                              defaultValue: "降序",
                            })) as string
                      }
                    >
                      {sortAsc ? "↑" : "↓"}
                    </Button>
                  </div>
                  <Chip variant="flat" color="default">
                    {flatItems.length}
                  </Chip>
                </div>
              </CardHeader>
            </Card>
          </motion.div>

          <div className="h-[140px] sm:h-[156px] lg:h-[168px]" />
          <div className="flex-1 min-h-0 w-full box-border pr-3 md:pr-4">
            <motion.div
              className="grid gap-3 grid-cols-[repeat(auto-fill,minmax(240px,1fr))] p-1 pb-6"
              layout
              variants={listVariants}
              initial="hidden"
              animate="show"
              transition={{
                layout: { duration: 0.35, ease: [0.22, 0.61, 0.36, 1] },
              }}
            >
              {flatItems.map((it) => (
                <motion.div
                  key={it.name}
                  layout
                  variants={itemVariants}
                  transition={{
                    layout: { duration: 0.35, ease: [0.22, 0.61, 0.36, 1] },
                  }}
                  whileHover={{ y: -2 }}
                  whileTap={{ scale: 0.98 }}
                  className="w-full min-w-0"
                >
                  <Card
                    className={`w-full rounded-2xl shadow-md bg-white/70 dark:bg-black/30 backdrop-blur-md border border-white/30 transition ${
                      selectedVersionName === it.name
                        ? "ring-2 ring-primary-500"
                        : ""
                    }`}
                  >
                    <CardBody className="p-3 sm:p-4" onClick={() => selectVersionByUser(it.name)}>
                      <div className="flex items-center justify-between gap-2">
                        <div className="font-semibold text-base truncate">
                          {it.name}
                        </div>
                        <div className="flex items-center gap-2">
                          {it.isPreview ? (
                            <Chip size="sm" color="warning" variant="flat">
                              Preview
                            </Chip>
                          ) : (
                            <Chip size="sm" color="success" variant="flat">
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
                      <div className="text-xs text-default-500 mt-1 flex items-center gap-2">
                        {(() => {
                          const u = logoMap.get(it.name);
                          return u ? (
                            <img
                              src={u}
                              alt="logo"
                              className="h-4 w-4 rounded"
                            />
                          ) : (
                            <div className="h-4 w-4 rounded bg-default-200" />
                          );
                        })()}
                        <span>
                          Vanilla{" "}
                          {it.version ||
                            t("launcherpage.version_select.unknown", {
                              defaultValue: "Unknown",
                            })}
                        </span>
                      </div>
                    </CardBody>
                  </Card>
                </motion.div>
              ))}
            </motion.div>
          </div>
        </div>
      </div>

      <BaseModal
        isOpen={unsavedDisclosure.isOpen}
        onOpenChange={unsavedDisclosure.onOpenChange}
        size="md"
      >
        <ModalContent>
          {(onClose) => (
            <>
              <BaseModalHeader className="flex items-center gap-2">
                <h2 className="text-lg font-semibold">
                  {t("versionselect.unsaved.title", {
                    defaultValue: "未保存修改",
                  })}
                </h2>
              </BaseModalHeader>
              <BaseModalBody>
                <div className="text-small text-default-600">
                  {t("versionselect.unsaved.body", {
                    defaultValue:
                      "您更改了选择的版本，但尚未保存。是否保存后离开？",
                  })}
                </div>
              </BaseModalBody>
              <BaseModalFooter>
                <Button
                  variant="light"
                  onPress={() => {
                    onClose?.();
                    if (navAttemptRef.current) navAttemptRef.current = null;
                  }}
                >
                  {t("common.cancel", { defaultValue: "取消" })}
                </Button>
                <Button
                  color="danger"
                  variant="light"
                  onPress={() => {
                    onClose?.();
                    commitNavAttempt();
                  }}
                >
                  {t("versionselect.unsaved.discard", {
                    defaultValue: "不保存离开",
                  })}
                </Button>
                <Button
                  color="primary"
                  onPress={() => {
                    const name = selectedVersionName;
                    if (name) {
                      try {
                        localStorage.setItem("ll.currentVersionName", name);
                      } catch {}
                    }
                    try {
                      props.refresh && props.refresh();
                    } catch {}
                    setPersistedName(name || persistedName);
                    onClose?.();
                    commitNavAttempt();
                  }}
                >
                  {t("versionselect.unsaved.save_and_leave", {
                    defaultValue: "保存并离开",
                  })}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>
    </>
  );
};
