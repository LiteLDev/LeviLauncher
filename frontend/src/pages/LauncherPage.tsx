import React, { useEffect, useRef, useState } from "react";
import {
  useDisclosure,
  ModalContent,
  Button,
  Card,
  CardHeader,
  CardBody,
  PressEvent,
  Dropdown,
  DropdownTrigger,
  DropdownMenu,
  DropdownItem,
  Input,
  Tooltip,
  Chip,
  Progress,
  Spinner,
} from "@heroui/react";
import { useTranslation } from "react-i18next";
import {
  EnsureGameInputInteractive,
  GetContentRoots,
  IsGDKInstalled,
  ListDir,
} from "bindings/github.com/liteldev/LeviLauncher/minecraft";

import {
  FaRocket,
  FaChevronDown,
  FaCog,
  FaGlobe,
  FaImage,
  FaCogs,
  FaList,
  FaWindows,
  FaFolderOpen,
  FaCheckCircle,
  FaDesktop,
  FaCube,
  FaArrowRight,
} from "react-icons/fa";
import { ModCard } from "@/components/ModdedCard";
import { ContentDownloadCard } from "@/components/ContentDownloadCard";
import { Events, Window, Browser } from "@wailsio/runtime";
import { motion, AnimatePresence } from "framer-motion";
import { useNavigate } from "react-router-dom";
import { compareVersions } from "@/utils/version";
import { saveCurrentVersionName } from "@/utils/currentVersion";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import {
  BaseModal,
  BaseModalHeader,
  BaseModalBody,
  BaseModalFooter,
} from "@/components/BaseModal";

let __didCheckGameInput = false;
let __didCheckGamingServices = false;
const IGNORE_GS_KEY = "ll.ignore.gs";

export const LauncherPage = (args: any) => {
  let [currentVersion, setCurrentVersion] = React.useState<string>("");
  const [displayVersion, setDisplayVersion] = React.useState<string>("");
  const [displayName, setDisplayName] = React.useState<string>("");
  const [localVersionMap, setLocalVersionMap] = React.useState<
    Map<string, any>
  >(new Map());
  const { isOpen, onOpen, onOpenChange, onClose } = useDisclosure();
  const [modalState, setModalState] = React.useState(0);
  const [overlayActive, setOverlayActive] = React.useState(false);
  const { t, i18n } = useTranslation();
  const hasBackend = minecraft !== undefined;
  const navigate = useNavigate();
  const [launchErrorCode, setLaunchErrorCode] = React.useState<string>("");
  const [contentCounts, setContentCounts] = React.useState<{
    worlds: number;
    resourcePacks: number;
    behaviorPacks: number;
  }>({ worlds: 0, resourcePacks: 0, behaviorPacks: 0 });
  const [giTotal, setGiTotal] = React.useState<number>(0);
  const [giDownloaded, setGiDownloaded] = React.useState<number>(0);
  const [pendingInstallCheck, setPendingInstallCheck] = React.useState<
    "gi" | "gs" | null
  >(null);
  const [logoDataUrl, setLogoDataUrl] = React.useState<string>("");
  const [versionQuery, setVersionQuery] = React.useState<string>("");
  const [logoByName, setLogoByName] = React.useState<Map<string, string>>(
    new Map(),
  );
  const fetchingLogos = React.useRef<Set<string>>(new Set());
  const ensureOpsRef = React.useRef<number>(0);
  const launchTips = React.useMemo(
    () => [
      t("launcherpage.tip.choose_version_dropdown", {
        defaultValue: "右侧下拉按钮可快速切换当前版本。",
      }) as unknown as string,
      t("launcherpage.tip.open_version_settings_gear", {
        defaultValue: "点击齿轮打开“版本设置”，可更换图标并启用版本隔离。",
      }) as unknown as string,
      t("launcherpage.tip.mods_import_button", {
        defaultValue: "点击 Mods 卡片的“导入 .zip/.dll”选择文件。",
      }) as unknown as string,
      t("launcherpage.tip.file_manager_pick", {
        defaultValue: "文件管理器支持多选并回传路径到 Mods 导入。",
      }) as unknown as string,
      t("launcherpage.tip.download_versions", {
        defaultValue: "在“下载”页安装 Release/Preview，安装后出现在下拉列表。",
      }) as unknown as string,
      t("launcherpage.tip.content_counts_card", {
        defaultValue: "主页“内容管理”显示世界、资源包、行为包数量。",
      }) as unknown as string,
      t("launcherpage.tip.settings_base_root", {
        defaultValue:
          "设置页可修改内容存储路径，默认使用 %APPDATA% 下以当前可执行文件名命名的文件夹。",
      }) as unknown as string,
      t("launcherpage.tip.directory_write_check", {
        defaultValue: "仅可保存到可写目录；不可写目录将被禁用。",
      }) as unknown as string,
      t("launcherpage.tip.general", {
        defaultValue: "首次启动可能较慢，请耐心等待。",
      }) as unknown as string,
    ],
    [t],
  );
  const [tipIndex, setTipIndex] = React.useState<number>(0);
  const tipTimerRef = React.useRef<number | null>(null);

  const worldsLabel = t("content.count.worlds", {
    defaultValue: "世界",
  }) as string;
  const resourceLabel = t("content.count.resource_packs", {
    defaultValue: "资源包",
  }) as string;
  const behaviorLabel = t("content.count.behavior_packs", {
    defaultValue: "行为包",
  }) as string;
  const labelSizeClass = (s: string) => {
    const len = s?.length || 0;
    if (len <= 4) return "text-base";
    if (len <= 10) return "text-small";
    return "text-xs";
  };

  useEffect(() => {
    try {
      const saved = localStorage.getItem("ll.currentVersionName") || "";
      if (!saved) return;
      setCurrentVersion(saved);
      setDisplayName(saved);
      const fn = (minecraft as any)?.GetVersionMeta;
      if (typeof fn === "function") {
        fn(saved)
          .then((m: any) => {
            const ver = String(m?.gameVersion || "");
            setDisplayVersion(ver || "");
            setLocalVersionMap((prev) => {
              const map = new Map(prev);
              map.set(saved, {
                name: saved,
                version: ver,
                isPreview: String(m?.type || "").toLowerCase() === "preview",
                isRegistered: Boolean(m?.registered),
                isLaunched: false,
                isPreLoader: false,
              });
              return map;
            });
            const getter = (minecraft as any)?.GetVersionLogoDataUrl;
            if (typeof getter === "function") {
              getter(saved).then((u: string) =>
                setLogoDataUrl(String(u || "")),
              );
            }
          })
          .catch(() => {});
      }
    } catch {}
  }, []);

  useEffect(() => {
    try {
      minecraft.ReconcileRegisteredFlags();
    } catch {}
  }, []);

  useEffect(() => {
    try {
      if (tipTimerRef.current) {
        clearInterval(tipTimerRef.current);
        tipTimerRef.current = null;
      }
      const getNext = (prev: number, len: number) => {
        const L = Math.max(1, len);
        if (L <= 1) return 0;
        let next = Math.floor(Math.random() * L);
        if (next === prev) next = (next + 1) % L;
        return next;
      };
      tipTimerRef.current = window.setInterval(() => {
        setTipIndex((prev) => getNext(prev, launchTips.length));
      }, 10000);
    } catch {}
    return () => {
      try {
        if (tipTimerRef.current) {
          clearInterval(tipTimerRef.current);
          tipTimerRef.current = null;
        }
      } catch {}
    };
  }, [launchTips.length]);

  const sortedVersionNames = React.useMemo(() => {
    const entries = Array.from(localVersionMap.entries()).map(
      ([name, info]) => ({
        name,
        version: String(info?.version || ""),
        isPreview: !!info?.isPreview,
      }),
    );
    entries.sort((a, b) => {
      const byVer = compareVersions(b.version, a.version);
      if (byVer !== 0) return byVer;
      const byPreview = b.isPreview === a.isPreview ? 0 : b.isPreview ? 1 : -1;
      if (byPreview !== 0) return byPreview;
      return a.name.localeCompare(b.name);
    });
    return entries.map((e) => e.name);
  }, [localVersionMap, compareVersions]);

  const filteredVersionNames = React.useMemo(() => {
    const q = versionQuery.trim().toLowerCase();
    if (!q) return sortedVersionNames;
    return sortedVersionNames.filter((name) => {
      const ver = String(localVersionMap.get(name)?.version || "");
      return name.toLowerCase().includes(q) || ver.toLowerCase().includes(q);
    });
  }, [versionQuery, sortedVersionNames, localVersionMap]);

  const versionMenuItems = React.useMemo(
    () =>
      filteredVersionNames.length === 0
        ? [
            {
              key: "__empty",
              name: t("common.empty", { defaultValue: "暂无数据" }) as string,
              version: "",
              isRegistered: false,
              isDisabled: true,
            },
          ]
        : filteredVersionNames.map((name) => ({
            key: name,
            name,
            version: String(localVersionMap.get(name)?.version || ""),
            isRegistered: Boolean(localVersionMap.get(name)?.isRegistered),
            isDisabled: false,
            logo: logoByName.get(name),
          })),
    [filteredVersionNames, localVersionMap, logoByName],
  );

  const ensureLogo = React.useCallback(
    (name: string) => {
      if (!name || logoByName.has(name) || fetchingLogos.current.has(name))
        return;
      fetchingLogos.current.add(name);
      try {
        const getter = minecraft?.GetVersionLogoDataUrl;
        if (typeof getter === "function") {
          getter(name)
            .then((u: string) => {
              fetchingLogos.current.delete(name);
              if (u) {
                setLogoByName((prev) => {
                  const m = new Map(prev);
                  m.set(name, String(u));
                  return m;
                });
              }
            })
            .catch(() => {
              fetchingLogos.current.delete(name);
            });
        } else {
          fetchingLogos.current.delete(name);
        }
      } catch {
        fetchingLogos.current.delete(name);
      }
    },
    [logoByName],
  );

  useEffect(() => {
    try {
      versionMenuItems.forEach((it: any) => {
        if (!it?.isDisabled) ensureLogo(it.name);
      });
    } catch {}
  }, [versionMenuItems, ensureLogo]);

  const doLaunch = React.useCallback(() => {
    const name = currentVersion;
    if (name) {
      saveCurrentVersionName(name);
      const launch = minecraft?.LaunchVersionByName;
      if (typeof launch === "function") {
        launch(name)
          .then((err: string) => {
            const s = String(err || "");
            if (s) {
              setLaunchErrorCode(s);
              setModalState(1);
              setOverlayActive(true);
              onOpen();
            }
          })
          .catch(() => {
            setLaunchErrorCode("ERR_LAUNCH_GAME");
            setModalState(1);
            setOverlayActive(true);
            onOpen();
          });
      }
    } else {
      navigate("/versions");
    }
  }, [currentVersion, navigate]);

  const doForceLaunch = React.useCallback(() => {
    const name = currentVersion;
    if (name) {
      saveCurrentVersionName(name);
      const launchForce = minecraft?.LaunchVersionByNameForce;
      if (typeof launchForce === "function") {
        launchForce(name)
          .then((err: string) => {
            const s = String(err || "");
            if (s) {
              setLaunchErrorCode(s);
              setModalState(1);
              setOverlayActive(true);
              onOpen();
            }
          })
          .catch(() => {
            setLaunchErrorCode("ERR_LAUNCH_GAME");
            setModalState(1);
            setOverlayActive(true);
            onOpen();
          });
      }
    }
  }, [currentVersion]);

  const doCreateShortcut = React.useCallback(() => {
    const name = currentVersion;
    if (name) {
      minecraft
        ?.CreateDesktopShortcut(name)
        .then((err: string) => {
          const s = String(err || "");
          if (s) {
            setLaunchErrorCode(s);
            setModalState(1);
            setOverlayActive(true);
            onOpen();
          } else {
            setModalState(12);
            setOverlayActive(true);
            onOpen();
          }
        })
        .catch(() => {
          setLaunchErrorCode("ERR_SHORTCUT_CREATE_FAILED");
          setModalState(1);
          setOverlayActive(true);
          onOpen();
        });
    }
  }, [currentVersion]);

  const doOpenFolder = React.useCallback(async () => {
    if (!currentVersion) return;
    try {
      const vdir = await minecraft.GetVersionsDir();
      if (!vdir) return;
      const path = vdir + "\\" + currentVersion;
      await minecraft.OpenPathDir(path);
    } catch (e) {
      console.error(e);
    }
  }, [currentVersion]);

  const doRegister = React.useCallback(async () => {
    if (!currentVersion) return;
    try {
      const ok = await IsGDKInstalled();
      if (!ok) {
        setModalState(17);
        setOverlayActive(true);
        onOpen();
        return;
      }
    } catch {}
    setModalState(13);
    setOverlayActive(true);
    onOpen();
    try {
      const isPreview = localVersionMap.get(currentVersion)?.isPreview || false;
      const result = await minecraft.RegisterVersionWithWdapp(
        currentVersion,
        isPreview,
      );
      if (result === "success" || result === "") {
        setModalState(15);
        const fn = (minecraft as any)?.GetVersionMeta;
        if (typeof fn === "function") {
          fn(currentVersion).then((m: any) => {
            setLocalVersionMap((prev) => {
              const map = new Map(prev);
              const existing = map.get(currentVersion);
              if (existing) {
                map.set(currentVersion, {
                  ...existing,
                  isRegistered: Boolean(m?.registered),
                });
              }
              return map;
            });
          });
        }
      } else if (result === "ERR_GDK_MISSING") {
        setModalState(17);
      } else {
        setLaunchErrorCode(result);
        setModalState(16);
      }
    } catch (e) {
      setLaunchErrorCode(String(e));
      setModalState(16);
    }
  }, [currentVersion, localVersionMap, navigate, onOpen]);

  useEffect(() => {
    if (!hasBackend) return;
    const timer = setTimeout(() => {
      if (!__didCheckGameInput) {
        __didCheckGameInput = true;
        try {
          minecraft?.IsGameInputInstalled?.().then((ok: boolean) => {
            if (!ok) {
              setModalState(8);
              setOverlayActive(true);
              onOpen();
            }
          });
        } catch {}
      }
      if (!__didCheckGamingServices) {
        __didCheckGamingServices = true;
        try {
          const ig = String(localStorage.getItem(IGNORE_GS_KEY) || "") === "1";
          if (!ig) {
            minecraft?.IsGamingServicesInstalled?.().then((ok: boolean) => {
              if (!ok) {
                setModalState(9);
                setOverlayActive(true);
                onOpen();
              }
            });
          }
        } catch {}
      }
    }, 0);
    return () => clearTimeout(timer);
  }, [hasBackend, onOpen]);

  useEffect(() => {
    const ensureStart = () => {
      ensureOpsRef.current = (ensureOpsRef.current || 0) + 1;
      setModalState(4);
      setOverlayActive(true);
      onOpen();
    };
    const ensureDone = () => {
      ensureOpsRef.current = Math.max((ensureOpsRef.current || 0) - 1, 0);
      if (ensureOpsRef.current === 0) {
        setModalState(0);
        onClose();
        setOverlayActive(false);
      }
    };

    const unlistenStart = () => {};
    const unlistenDone = () => {};
    const unlistenPreStart = () => {};
    const unlistenPreDone = () => {};
    const unlistenPeStart = () => {};
    const unlistenPeDone = () => {};

    const unlistenGiStart = Events.On("gameinput.ensure.start", () => {
      if (pendingInstallCheck === "gi") return;
      setModalState(7);
      setOverlayActive(true);
      onOpen();
    });
    const unlistenGiDlStart = Events.On("gameinput.download.start", (event) => {
      if (pendingInstallCheck === "gi") return;
      setGiTotal(Number(event?.data || 0));
      setGiDownloaded(0);
      setModalState(7);
      setOverlayActive(true);
      onOpen();
    });
    const unlistenGiDlProgress = Events.On(
      "gameinput.download.progress",
      (event) => {
        if (pendingInstallCheck === "gi") return;
        const d = event?.data || {};
        if (typeof d?.Total === "number") setGiTotal(d.Total);
        if (typeof d?.Downloaded === "number") setGiDownloaded(d.Downloaded);
      },
    );
    const unlistenGiDlDone = Events.On("gameinput.download.done", () => {
      setGiDownloaded(giTotal);
      setPendingInstallCheck((prev) => prev ?? "gi");
      setModalState(10);
      setOverlayActive(true);
      onOpen();
    });
    const unlistenGiDlError = Events.On(
      "gameinput.download.error",
      (data) => {},
    );
    const unlistenGiDone = Events.On("gameinput.ensure.done", () => {
      setPendingInstallCheck((prev) => prev ?? "gi");
      setModalState(10);
      setOverlayActive(true);
      onOpen();
    });

    const unlistenGsMissing = Events.On("gamingservices.missing", () => {
      const ig = String(localStorage.getItem(IGNORE_GS_KEY) || "") === "1";
      if (ig) return;
      setModalState(9);
      setOverlayActive(true);
      onOpen();
    });

    return () => {
      try {
        unlistenStart && (unlistenStart as any)();
      } catch {}
      try {
        unlistenDone && (unlistenDone as any)();
      } catch {}
      try {
        unlistenPreStart && (unlistenPreStart as any)();
      } catch {}
      try {
        unlistenPreDone && (unlistenPreDone as any)();
      } catch {}
      try {
        unlistenPeStart && (unlistenPeStart as any)();
      } catch {}
      try {
        unlistenPeDone && (unlistenPeDone as any)();
      } catch {}
      try {
        unlistenGiStart && (unlistenGiStart as any)();
      } catch {}
      try {
        unlistenGiDlStart && (unlistenGiDlStart as any)();
      } catch {}
      try {
        unlistenGiDlProgress && (unlistenGiDlProgress as any)();
      } catch {}
      try {
        unlistenGiDlDone && (unlistenGiDlDone as any)();
      } catch {}
      try {
        unlistenGiDlError && (unlistenGiDlError as any)();
      } catch {}
      try {
        unlistenGiDone && (unlistenGiDone as any)();
      } catch {}
      try {
        unlistenGsMissing && (unlistenGsMissing as any)();
      } catch {}
    };
  }, []);

  const refreshContentCounts = React.useCallback(async () => {
    if (!hasBackend) {
      setContentCounts({ worlds: 0, resourcePacks: 0, behaviorPacks: 0 });
      return;
    }
    const readCurrentVersionName = (): string => {
      try {
        return localStorage.getItem("ll.currentVersionName") || "";
      } catch {
        return "";
      }
    };
    const countDir = async (path: string): Promise<number> => {
      try {
        const entries = await ListDir(path);
        return (entries || []).filter((e: any) => e.isDir).length;
      } catch {
        return 0;
      }
    };
    try {
      const name = readCurrentVersionName();
      if (!name) {
        setContentCounts({ worlds: 0, resourcePacks: 0, behaviorPacks: 0 });
        return;
      }
      const roots = await GetContentRoots(name);
      const safe = roots || {
        base: "",
        usersRoot: "",
        resourcePacks: "",
        behaviorPacks: "",
        isIsolation: false,
        isPreview: false,
      };
      let worlds = 0;
      if (safe.usersRoot) {
        try {
          const entries = await ListDir(safe.usersRoot);
          const players = (entries || [])
            .filter((e: any) => e.isDir)
            .map((e: any) => e.name)
            .filter((n: string) => n && n.toLowerCase() !== "shared");
          const nextPlayer = players[0] || "";
          if (nextPlayer) {
            const wp = `${safe.usersRoot}\\${nextPlayer}\\games\\com.mojang\\minecraftWorlds`;
            worlds = await countDir(wp);
          }
        } catch {}
      }
      const res = await countDir(safe.resourcePacks);
      const bp = await countDir(safe.behaviorPacks);
      setContentCounts({ worlds, resourcePacks: res, behaviorPacks: bp });
    } catch {
      setContentCounts({ worlds: 0, resourcePacks: 0, behaviorPacks: 0 });
    }
  }, [hasBackend]);

  useEffect(() => {
    refreshContentCounts();
  }, [refreshContentCounts, hasBackend, currentVersion]);

  useEffect(() => {
    const unlistenMcStart = Events.On("mc.launch.start", () => {
      setModalState(5);
      setOverlayActive(true);
      onOpen();
    });

    const unlistenMcDone = Events.On("mc.launch.done", () => {
      setOverlayActive(false);
      setModalState((prev) => {
        if (prev === 1) {
          return prev;
        }
        try {
          onClose();
        } catch {}
        return 0;
      });
    });
    const unlistenMcFailed = Events.On("mc.launch.failed", (data) => {
      setOverlayActive(false);
      const payload: any = (data as any)?.data ?? data;
      const first = Array.isArray(payload) ? payload[0] : payload;
      const code = String(first || "");
      setLaunchErrorCode(code || "ERR_LAUNCH_GAME");
      setModalState(1);
      onOpen();
    });

    return () => {
      try {
        unlistenMcStart && (unlistenMcStart as any)();
      } catch {}
      try {
        unlistenMcDone && (unlistenMcDone as any)();
      } catch {}
      try {
        unlistenMcFailed && (unlistenMcFailed as any)();
      } catch {}
    };
  }, []);

  useEffect(() => {
    if (hasBackend) {
      const listFn =
        (minecraft as any)?.ListVersionMetasWithRegistered ??
        (minecraft as any)?.ListVersionMetas;
      if (typeof listFn === "function") {
        listFn().then((metas: any[]) => {
          const newLocalVersionMap = new Map();
          const newLocalVersionsMap = new Map();
          metas?.forEach((m: any) => {
            const name = String(m?.name || "");
            const gameVersion = String(m?.gameVersion || "");
            const type = String(m?.type || "release");
            const isPreview = type.toLowerCase() === "preview";
            const lv: any = {
              name,
              version: gameVersion,
              isPreview,
              isRegistered: Boolean(m?.registered),
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

          const saved = (() => {
            try {
              return localStorage.getItem("ll.currentVersionName") || "";
            } catch {
              return "";
            }
          })();
          const useName =
            saved && newLocalVersionMap.has(saved)
              ? saved
              : Array.from(newLocalVersionMap.keys())[0] || "";
          setCurrentVersion(useName);
          try {
            saveCurrentVersionName(useName);
          } catch {}
          const ver = useName
            ? newLocalVersionMap.get(useName)?.version || ""
            : "";
          setDisplayVersion(ver || "None");
          setDisplayName(useName || "");
          try {
            const getter = minecraft?.GetVersionLogoDataUrl;
            if (typeof getter === "function" && useName) {
              getter(useName).then((u: string) =>
                setLogoDataUrl(String(u || "")),
              );
            } else {
              setLogoDataUrl("");
            }
          } catch {
            setLogoDataUrl("");
          }
        });
      }
    } else {
      setCurrentVersion("");
      setLocalVersionMap(new Map());
    }
  }, [args.count]);

  const MODAL_VIEWS: Record<
    number,
    (onClose: ((e: PressEvent) => void) | undefined) => JSX.Element
  > = {
    1: (onClose) => (
      <>
        <BaseModalHeader>
          <h2 className="text-2xl font-black tracking-tight text-danger-500">
            {t("launcherpage.launch.failed.title")}
          </h2>
        </BaseModalHeader>
        <BaseModalBody>
          <div className="p-4 rounded-2xl bg-danger-50 dark:bg-danger-500/10 border border-danger-100 dark:border-danger-500/20 text-danger-600 dark:text-danger-400">
            <p className="font-medium text-center">
              {(() => {
                const key = `errors.${launchErrorCode}`;
                const translated = t(key) as unknown as string;
                if (launchErrorCode && translated && translated !== key)
                  return translated;
                return t(
                  "launcherpage.launch.failed.content",
                ) as unknown as string;
              })()}
            </p>
          </div>
        </BaseModalBody>
        <BaseModalFooter>
          {launchErrorCode === "ERR_GAME_ALREADY_RUNNING" && (
            <Button
              color="warning"
              variant="flat"
              radius="full"
              onPress={(e) => {
                onClose?.(e);
                setOverlayActive(false);
                setModalState(0);
                doForceLaunch();
              }}
            >
              {t("launcherpage.launch.force_run_button", {
                defaultValue: "强制启动",
              })}
            </Button>
          )}
          <Button
            color="danger"
            variant="solid"
            radius="full"
            className="font-bold shadow-lg shadow-danger-500/20"
            onPress={(e) => {
              onClose?.(e);
              setOverlayActive(false);
              setModalState(0);
            }}
          >
            {t("launcherpage.launch.failed.close_button")}
          </Button>
        </BaseModalFooter>
      </>
    ),
    7: (onClose) => (
      <>
        <BaseModalHeader>
          <h2 className="text-2xl font-black tracking-tight text-emerald-600">
            {t("launcherpage.gameinput.installing.title", {
              defaultValue: "正在安装 GameInput",
            })}
          </h2>
        </BaseModalHeader>
        <BaseModalBody>
          <p className="text-default-600 font-medium">
            {t("launcherpage.gameinput.installing.body", {
              defaultValue: "正在下载并启动安装程序，请根据系统提示完成安装。",
            })}
          </p>
          <div className="mt-4">
            {giTotal > 0 ? (
              <div className="flex flex-col gap-2">
                <div className="flex justify-between text-small font-bold text-default-500">
                  <span>
                    {Math.min(100, Math.floor((giDownloaded / giTotal) * 100))}%
                  </span>
                  <span className="font-mono">
                    {(giDownloaded / 1024 / 1024).toFixed(1)} /{" "}
                    {(giTotal / 1024 / 1024).toFixed(1)} MB
                  </span>
                </div>
                <Progress
                  aria-label="Downloading"
                  value={(giDownloaded / giTotal) * 100}
                  color="success"
                  size="md"
                  classNames={{
                    indicator: "bg-emerald-600 hover:bg-emerald-500",
                  }}
                />
              </div>
            ) : (
              <div className="flex items-center gap-3 text-default-500">
                <Spinner size="sm" color="success" />
                <span>
                  {t("launcherpage.gameinput.installing.preparing", {
                    defaultValue: "正在准备下载...",
                  })}
                </span>
              </div>
            )}
          </div>
        </BaseModalBody>
      </>
    ),
    8: (onClose) => (
      <>
        <BaseModalHeader>
          <h2 className="text-2xl font-black tracking-tight text-warning-500">
            {t("launcherpage.gameinput.missing.title", {
              defaultValue: "缺少 GameInput 组件",
            })}
          </h2>
        </BaseModalHeader>
        <BaseModalBody>
          <p className="text-default-600 font-medium">
            {t("launcherpage.gameinput.missing.body", {
              defaultValue:
                "运行游戏需要 Microsoft GameInput 组件。是否现在下载安装？",
            })}
          </p>
        </BaseModalBody>
        <BaseModalFooter>
          <Button
            color="danger"
            variant="light"
            radius="full"
            onPress={() => {
              Window.Close();
            }}
          >
            {t("common.quit_launcher", { defaultValue: "退出启动器" })}
          </Button>
          <Button
            color="primary"
            radius="full"
            className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
            onPress={() => {
              setPendingInstallCheck("gi");
              EnsureGameInputInteractive();
              setModalState(10);
              setOverlayActive(true);
            }}
          >
            {t("launcherpage.gameinput.missing.install_now", {
              defaultValue: "立即安装",
            })}
          </Button>
        </BaseModalFooter>
      </>
    ),
    9: (onClose) => (
      <>
        <BaseModalHeader>
          <h2 className="text-2xl font-black tracking-tight text-warning-500">
            {t("launcherpage.gs.missing.title", {
              defaultValue: "缺少 Microsoft Gaming Services",
            })}
          </h2>
        </BaseModalHeader>
        <BaseModalBody>
          <p className="text-default-600 font-medium">
            {t("launcherpage.gs.missing.body", {
              defaultValue:
                "未检测到 Microsoft Gaming Services。该组件是 Minecraft 运行所必须的依赖项。",
            })}
          </p>
        </BaseModalBody>
        <BaseModalFooter>
          <Button
            color="danger"
            variant="light"
            radius="full"
            onPress={() => {
              Window.Close();
            }}
          >
            {t("common.quit_launcher", { defaultValue: "退出启动器" })}
          </Button>
          <Button
            color="default"
            variant="flat"
            radius="full"
            onPress={() => {
              localStorage.setItem(IGNORE_GS_KEY, "1");
              setOverlayActive(false);
              setModalState(0);
              onClose && onClose({} as any);
            }}
          >
            {t("launcherpage.gs.missing.ignore_forever", {
              defaultValue: "忽略并不再提醒",
            })}
          </Button>
          <Button
            color="primary"
            radius="full"
            className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
            onPress={() => {
              setPendingInstallCheck("gs");
              Browser.OpenURL("ms-windows-store://pdp/?ProductId=9MWPM2CQNLHN");
              setModalState(10);
              setOverlayActive(true);
            }}
          >
            {t("launcherpage.gs.missing.open_store", {
              defaultValue: "打开商店进行安装",
            })}
          </Button>
        </BaseModalFooter>
      </>
    ),
    10: (onClose) => (
      <>
        <BaseModalHeader>
          <h2 className="text-2xl font-black tracking-tight text-emerald-600">
            {t("launcherpage.install_confirm.title", {
              defaultValue: "是否已完成安装？",
            })}
          </h2>
        </BaseModalHeader>
        <BaseModalBody>
          <p className="text-default-600 font-medium">
            {t("launcherpage.install_confirm.body", {
              defaultValue:
                "安装完成后请点击“已完成，重新检测”。如果尚未完成，请继续安装。",
            })}
          </p>
        </BaseModalBody>
        <BaseModalFooter>
          <Button
            color="default"
            variant="light"
            radius="full"
            onPress={() => {
              if (pendingInstallCheck === "gi") setModalState(8);
              else if (pendingInstallCheck === "gs") setModalState(9);
            }}
          >
            {t("launcherpage.install_confirm.continue", {
              defaultValue: "继续安装",
            })}
          </Button>
          <Button
            color="primary"
            radius="full"
            className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
            onPress={() => {
              try {
                if (pendingInstallCheck === "gi") {
                  minecraft?.IsGameInputInstalled?.().then((ok: boolean) => {
                    if (ok) {
                      setPendingInstallCheck(null);
                      setModalState(0);
                      onOpenChange();
                      setOverlayActive(false);
                    } else {
                      setModalState(8);
                    }
                  });
                } else if (pendingInstallCheck === "gs") {
                  minecraft
                    ?.IsGamingServicesInstalled?.()
                    .then((ok: boolean) => {
                      if (ok) {
                        setPendingInstallCheck(null);
                        setModalState(0);
                        onOpenChange();
                        setOverlayActive(false);
                      } else {
                        setModalState(9);
                      }
                    });
                }
              } catch {}
            }}
          >
            {t("launcherpage.install_confirm.done_and_check", {
              defaultValue: "已完成，重新检测",
            })}
          </Button>
        </BaseModalFooter>
      </>
    ),
    4: () => (
      <>
        <BaseModalHeader>
          <motion.h2
            className="text-2xl font-black tracking-tight text-emerald-600"
            initial={{ opacity: 0, y: -10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.25 }}
          >
            {t("launcherpage.vcruntime.completing.title")}
          </motion.h2>
        </BaseModalHeader>
        <BaseModalBody>
          <motion.p
            className="text-default-600 font-medium"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ duration: 0.25, delay: 0.1 }}
          >
            {t("launcherpage.vcruntime.completing.body")}
          </motion.p>
        </BaseModalBody>
      </>
    ),
    5: () => (
      <>
        <BaseModalHeader>
          <motion.h2
            className="text-2xl font-black tracking-tight text-emerald-600"
            initial={{ opacity: 0, y: -10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.25 }}
          >
            {t("launcherpage.mclaunch.loading.title")}
          </motion.h2>
        </BaseModalHeader>
        <BaseModalBody>
          <div className="flex flex-col gap-6">
            <div className="flex items-center gap-4">
              <Spinner
                size="lg"
                color="success"
                classNames={{
                  circle1: "border-b-emerald-500",
                  circle2: "border-b-teal-500",
                }}
              />
              <div className="flex flex-col gap-1">
                <motion.p
                  className="text-default-600 font-medium"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  transition={{ duration: 0.25, delay: 0.1 }}
                >
                  {t("launcherpage.mclaunch.loading.body")}
                </motion.p>
                <div className="min-h-[24px] text-sm text-default-400">
                  <AnimatePresence mode="wait">
                    <motion.span
                      key={`tip-${tipIndex}`}
                      initial={{ opacity: 0, y: 4 }}
                      animate={{ opacity: 1, y: 0 }}
                      exit={{ opacity: 0, y: -4 }}
                      transition={{ duration: 0.2 }}
                    >
                      {launchTips[tipIndex]}
                    </motion.span>
                  </AnimatePresence>
                </div>
              </div>
            </div>
            <Progress
              size="sm"
              isIndeterminate
              aria-label="Loading"
              classNames={{ indicator: "bg-emerald-600 hover:bg-emerald-500" }}
            />
          </div>
        </BaseModalBody>
        <BaseModalFooter>
          <Button
            color="primary"
            radius="full"
            className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
            onPress={(e) => {
              onClose?.();
              setOverlayActive(false);
              setModalState(0);
            }}
          >
            {t("common.close", { defaultValue: "关闭" }) as unknown as string}
          </Button>
        </BaseModalFooter>
      </>
    ),
    2: (onClose) => (
      <>
        <BaseModalHeader>
          <h2 className="text-2xl font-black tracking-tight text-warning-500">
            {t("launcherpage.adminconfirm.title")}
          </h2>
        </BaseModalHeader>
        <BaseModalBody>
          <p className="text-default-600 font-medium">
            {t("launcherpage.adminconfirm.content")}
          </p>
        </BaseModalBody>
        <BaseModalFooter>
          <Button
            color="default"
            variant="light"
            radius="full"
            onPress={onClose}
          >
            {t("launcherpage.adminconfirm.cancel_button")}
          </Button>
          <Button
            color="primary"
            radius="full"
            className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
            onPress={(e) => {
              onClose?.(e);
            }}
          >
            {t("launcherpage.adminconfirm.confirm_button")}
          </Button>
        </BaseModalFooter>
      </>
    ),
    3: (onClose) => (
      <>
        <BaseModalHeader className="flex flex-col gap-1 px-8 pt-6 pb-2">
          <h2 className="text-2xl font-black tracking-tight text-danger-500">
            {t("launcherpage.admindeny.title")}
          </h2>
        </BaseModalHeader>
        <BaseModalBody className="px-8 py-4">
          <p className="text-default-600 font-medium">
            {t("launcherpage.admindeny.content")}
          </p>
        </BaseModalBody>
        <BaseModalFooter className="px-8 pb-8 pt-4">
          <Button
            color="default"
            variant="light"
            radius="full"
            onPress={onClose}
          >
            {t("launcherpage.admindeny.cancel_button")}
          </Button>
          <Button
            color="primary"
            radius="full"
            className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
            onPress={(e) => {
              onClose?.(e);
            }}
          >
            {t("launcherpage.admindeny.confirm_button")}
          </Button>
        </BaseModalFooter>
      </>
    ),
    12: (onClose) => (
      <>
        <BaseModalHeader className="flex flex-col gap-1 px-8 pt-6 pb-2">
          <h2 className="text-2xl font-black tracking-tight text-success-500">
            {
              t("launcherpage.shortcut.success.title", {
                defaultValue: "快捷方式已创建",
              }) as unknown as string
            }
          </h2>
        </BaseModalHeader>
        <BaseModalBody className="px-8 py-4">
          <p className="text-default-600 font-medium">
            {
              t("launcherpage.shortcut.success.body", {
                defaultValue: "已在桌面创建该版本的快捷方式。",
              }) as unknown as string
            }
          </p>
        </BaseModalBody>
        <BaseModalFooter className="px-8 pb-8 pt-4">
          <Button
            color="primary"
            radius="full"
            className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
            onPress={(e) => {
              onClose?.(e);
              setOverlayActive(false);
              setModalState(0);
            }}
          >
            {t("common.close", { defaultValue: "关闭" }) as unknown as string}
          </Button>
        </BaseModalFooter>
      </>
    ),
    13: (onClose) => (
      <>
        <BaseModalHeader className="flex flex-col gap-1 px-8 pt-6 pb-2">
          <h2 className="text-2xl font-black tracking-tight text-emerald-600">
            {t("launcherpage.register.installing.title", {
              defaultValue: "正在注册到系统",
            })}
          </h2>
        </BaseModalHeader>
        <BaseModalBody className="px-8 py-4">
          <p className="text-default-600 font-medium mb-4">
            {t("launcherpage.register.installing.body", {
              defaultValue: "正在调用 wdapp.exe 执行注册，请稍候…",
            })}
          </p>
          <Progress
            size="sm"
            isIndeterminate
            aria-label="Registering"
            classNames={{ indicator: "bg-emerald-600 hover:bg-emerald-500" }}
          />
        </BaseModalBody>
      </>
    ),
    15: (onClose) => (
      <>
        <BaseModalHeader className="flex flex-col gap-1 px-8 pt-6 pb-2">
          <h2 className="text-2xl font-black tracking-tight text-success-500">
            {t("launcherpage.register.success.title", {
              defaultValue: "注册完成",
            })}
          </h2>
        </BaseModalHeader>
        <BaseModalBody className="px-8 py-4">
          <p className="text-default-600 font-medium">
            {t("launcherpage.register.success.body", {
              defaultValue: "已成功注册到系统，您可以通过系统应用列表启动。",
            })}
          </p>
        </BaseModalBody>
        <BaseModalFooter className="px-8 pb-8 pt-4">
          <Button
            color="primary"
            radius="full"
            className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
            onPress={(e) => {
              onClose?.(e);
              setOverlayActive(false);
              setModalState(0);
            }}
          >
            {t("common.close", { defaultValue: "关闭" }) as unknown as string}
          </Button>
        </BaseModalFooter>
      </>
    ),
    16: (onClose) => (
      <>
        <BaseModalHeader className="flex flex-col gap-1 px-8 pt-6 pb-2">
          <h2 className="text-2xl font-black tracking-tight text-danger-500">
            {t("launcherpage.register.failed.title", {
              defaultValue: "注册失败",
            })}
          </h2>
        </BaseModalHeader>
        <BaseModalBody className="px-8 py-4">
          <div className="p-4 rounded-2xl bg-danger-50 dark:bg-danger-500/10 border border-danger-100 dark:border-danger-500/20 text-danger-600 dark:text-danger-400">
            <p className="font-medium text-center">
              {(() => {
                const key = `errors.${launchErrorCode}`;
                const translated = t(key) as unknown as string;
                if (launchErrorCode && translated && translated !== key)
                  return translated;
                return t("launcherpage.register.failed.body", {
                  defaultValue: "注册过程中发生错误，请重试或检查环境。",
                }) as unknown as string;
              })()}
            </p>
          </div>
        </BaseModalBody>
        <BaseModalFooter className="px-8 pb-8 pt-4">
          <Button
            color="primary"
            radius="full"
            className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
            onPress={(e) => {
              onClose?.(e);
              setOverlayActive(false);
              setModalState(0);
            }}
          >
            {t("common.close", { defaultValue: "关闭" }) as unknown as string}
          </Button>
        </BaseModalFooter>
      </>
    ),
    17: (onClose) => (
      <>
        <BaseModalHeader>
          <h2 className="text-2xl font-black tracking-tight text-warning-500">
            {t("launcherpage.gdk_missing.title", {
              defaultValue: "缺少 Microsoft GDK",
            })}
          </h2>
        </BaseModalHeader>
        <BaseModalBody>
          <p className="text-default-600 font-medium">
            {t("launcherpage.gdk_missing.body", {
              defaultValue:
                "未检测到 GDK 工具包，注册功能需先安装。是否跳转到设置页进行安装？",
            })}
          </p>
        </BaseModalBody>
        <BaseModalFooter>
          <Button
            variant="light"
            radius="full"
            onPress={(e) => {
              onClose?.(e);
              setOverlayActive(false);
              setModalState(0);
            }}
          >
            {t("common.cancel", { defaultValue: "取消" }) as unknown as string}
          </Button>
          <Button
            color="primary"
            radius="full"
            className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
            onPress={(e) => {
              onClose?.(e);
              setOverlayActive(false);
              setModalState(0);
              navigate("/settings");
            }}
          >
            {
              t("launcherpage.gdk_missing.go_settings", {
                defaultValue: "前往设置",
              }) as unknown as string
            }
          </Button>
        </BaseModalFooter>
      </>
    ),
  };

  const ModalUi = (onClose: ((e: PressEvent) => void) | undefined) => {
    const render = MODAL_VIEWS[modalState];
    return render ? render(onClose) : <></>;
  };

  return (
    <>
      <AnimatePresence>
        {overlayActive && (
          <motion.div
            className="fixed inset-0 z-50 bg-black/40 backdrop-blur-sm"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.2 }}
          />
        )}
      </AnimatePresence>
      <div className="relative w-full max-w-full mx-auto px-4 py-4 h-full flex flex-col justify-center">
        {/* Hero Launch Card */}
        <motion.div
          initial={{ opacity: 0, y: -20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.4, ease: "easeOut" }}
          className="mb-4 sm:mb-6"
        >
          <Card className="relative overflow-hidden border-none shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl">
            <CardBody className="p-6 sm:p-8 relative flex flex-col gap-6">
              {/* Main Layout */}
              <div className="flex flex-col md:flex-row md:items-center justify-between gap-6">
                {/* Left: Title & Info */}
                <div className="flex flex-col gap-1 min-w-0">
                  <div className="flex items-center gap-3">
                    <motion.h1
                      className="text-4xl sm:text-5xl font-black tracking-tight text-transparent bg-clip-text bg-gradient-to-r from-emerald-600 to-teal-600 dark:bg-none dark:text-emerald-500 truncate pb-2"
                      initial={{ opacity: 0, x: -20 }}
                      animate={{ opacity: 1, x: 0 }}
                      transition={{ delay: 0.2 }}
                    >
                      Minecraft
                    </motion.h1>
                    {localVersionMap.get(currentVersion)?.isRegistered && (
                      <Chip
                        startContent={<FaCheckCircle size={14} />}
                        variant="flat"
                        color="success"
                        classNames={{
                          base: "bg-emerald-500/10 border border-emerald-500/20 hidden sm:flex",
                          content:
                            "font-semibold text-emerald-600 dark:text-emerald-500",
                        }}
                      >
                        {t("launcherpage.registered_tip", {
                          defaultValue: "已注册",
                        })}
                      </Chip>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="text-lg sm:text-xl font-medium text-default-500 dark:text-zinc-400">
                      {t("launcherpage.edition", {
                        defaultValue: "Bedrock Edition",
                      })}
                    </span>
                  </div>
                </div>

                {/* Right: Actions */}
                <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-3 shrink-0">
                  {/* Version Selector */}
                  <div className="flex items-center gap-3 p-1.5 rounded-2xl">
                    <Dropdown placement="bottom-end">
                      <DropdownTrigger>
                        <Button
                          variant="light"
                          className="h-12 px-3 bg-transparent data-[hover=true]:bg-default-200/50 dark:data-[hover=true]:bg-white/5"
                        >
                          <div className="flex items-center gap-3 text-left">
                            <div className="w-8 h-8 rounded-lg bg-default-200/50 dark:bg-white/10 flex items-center justify-center overflow-hidden shadow-sm">
                              {logoDataUrl ? (
                                <img
                                  src={logoDataUrl}
                                  alt="logo"
                                  className="w-full h-full object-cover"
                                />
                              ) : (
                                <span className="text-base font-bold text-default-500 dark:text-zinc-400">
                                  M
                                </span>
                              )}
                            </div>
                            <div className="flex flex-col hidden lg:flex">
                              <span className="text-xs text-default-500 dark:text-zinc-400 font-medium">
                                {t("launcherpage.currentVersion")}
                              </span>
                              <span className="text-sm font-bold text-default-900 dark:text-white leading-tight max-w-[120px] truncate">
                                {displayName ||
                                  t("launcherpage.currentVersion_none", {
                                    defaultValue: "Select Version",
                                  })}
                              </span>
                            </div>
                            <span className="text-sm font-bold text-default-900 dark:text-white leading-tight max-w-[120px] truncate lg:hidden">
                              {displayName ||
                                t("launcherpage.currentVersion_none", {
                                  defaultValue: "Select",
                                })}
                            </span>
                            <FaChevronDown
                              className="text-default-400 dark:text-zinc-300 ml-1"
                              size={12}
                            />
                          </div>
                        </Button>
                      </DropdownTrigger>
                      <DropdownMenu
                        aria-label="Version Selection"
                        selectionMode="single"
                        selectedKeys={
                          new Set(currentVersion ? [currentVersion] : [])
                        }
                        className="max-h-[400px] overflow-y-auto no-scrollbar min-w-[300px]"
                        topContent={
                          <div className="p-3 border-b border-default-100 dark:border-default-50/10">
                            <Input
                              size="sm"
                              placeholder={t("launcherpage.search_versions", {
                                defaultValue: "Search versions...",
                              })}
                              value={versionQuery}
                              onValueChange={setVersionQuery}
                              startContent={
                                <FaList className="text-default-400" />
                              }
                              classNames={{
                                inputWrapper:
                                  "bg-default-100 dark:bg-default-50/20",
                              }}
                            />
                            <Button
                              fullWidth
                              size="sm"
                              variant="flat"
                              className="mt-2"
                              onPress={() => navigate("/versions")}
                            >
                              {t("launcherpage.manage_versions", {
                                defaultValue: "Manage All Versions",
                              })}
                            </Button>
                          </div>
                        }
                        items={versionMenuItems}
                        onSelectionChange={(keys) => {
                          const selected = Array.from(keys)[0] as string;
                          if (selected) {
                            setCurrentVersion(selected);
                            setDisplayName(selected);
                            const ver =
                              localVersionMap.get(selected)?.version || "";
                            setDisplayVersion(ver || "None");
                            try {
                              localStorage.setItem(
                                "ll.currentVersionName",
                                selected,
                              );
                              const getter = minecraft?.GetVersionLogoDataUrl;
                              if (typeof getter === "function") {
                                getter(selected).then((u: string) =>
                                  setLogoDataUrl(String(u || "")),
                                );
                              }
                            } catch {}
                          }
                        }}
                      >
                        {(item: any) => (
                          <DropdownItem
                            key={item.key}
                            textValue={item.name}
                            description={item.version}
                            startContent={
                              <div className="w-8 h-8 shrink-0 rounded-lg bg-default-100 dark:bg-white/10 flex items-center justify-center overflow-hidden">
                                {(() => {
                                  const u =
                                    item.logo || logoByName.get(item.name);
                                  if (!u) ensureLogo(item.name);
                                  return u ? (
                                    <img
                                      src={u}
                                      className="w-full h-full object-cover"
                                    />
                                  ) : (
                                    <span className="text-sm font-bold text-default-500 dark:text-zinc-400">
                                      M
                                    </span>
                                  );
                                })()}
                              </div>
                            }
                          >
                            <div className="flex justify-between items-center gap-2">
                              <span className="font-semibold">{item.name}</span>
                              {item.isRegistered && (
                                <Chip
                                  size="sm"
                                  variant="flat"
                                  color="success"
                                  classNames={{
                                    base: "bg-emerald-500/10 border border-emerald-500/20 h-5 px-1",
                                    content:
                                      "text-emerald-600 dark:text-emerald-500 font-bold text-[10px]",
                                  }}
                                >
                                  {t("launcherpage.registered_tip", {
                                    defaultValue: "已注册",
                                  })}
                                </Chip>
                              )}
                            </div>
                          </DropdownItem>
                        )}
                      </DropdownMenu>
                    </Dropdown>

                    <Dropdown>
                      <DropdownTrigger>
                        <Button
                          isIconOnly
                          variant="light"
                          radius="full"
                          size="sm"
                          className="data-[hover=true]:bg-default-200/50 dark:data-[hover=true]:bg-white/5"
                        >
                          <FaCogs
                            size={18}
                            className="text-default-500 dark:text-zinc-400"
                          />
                        </Button>
                      </DropdownTrigger>
                      <DropdownMenu aria-label="Version Actions">
                        <DropdownItem
                          key="settings"
                          startContent={<FaCog />}
                          onPress={() => {
                            if (currentVersion) {
                              navigate("/version-settings", {
                                state: { name: currentVersion, returnTo: "/" },
                              });
                            } else {
                              navigate("/versions");
                            }
                          }}
                        >
                          {t("launcherpage.go_version_settings", {
                            defaultValue: "Version Settings",
                          })}
                        </DropdownItem>
                        <DropdownItem
                          key="shortcut"
                          startContent={<FaDesktop />}
                          onPress={doCreateShortcut}
                        >
                          {t("launcherpage.shortcut.create_button", {
                            defaultValue: "Create Desktop Shortcut",
                          })}
                        </DropdownItem>
                        <DropdownItem
                          key="folder"
                          startContent={<FaFolderOpen />}
                          onPress={doOpenFolder}
                        >
                          {t("launcherpage.open_exe_dir", {
                            defaultValue: "Open Installation Folder",
                          })}
                        </DropdownItem>
                        <DropdownItem
                          key="register"
                          startContent={<FaWindows />}
                          onPress={doRegister}
                        >
                          {t("launcherpage.register_system_button", {
                            defaultValue: "Register to System",
                          })}
                        </DropdownItem>
                      </DropdownMenu>
                    </Dropdown>
                  </div>

                  {/* Launch Button */}
                  <motion.div
                    whileHover={{ scale: 1.02 }}
                    whileTap={{ scale: 0.98 }}
                  >
                    <Button
                      size="lg"
                      className="h-14 px-8 text-lg font-bold text-white shadow-emerald-900/20 shadow-lg bg-emerald-600 hover:bg-emerald-500 rounded-2xl w-full sm:w-auto"
                      startContent={<FaRocket className="mb-0.5" />}
                      onPress={doLaunch}
                      isLoading={modalState === 5}
                    >
                      {t("launcherpage.launch_button", {
                        defaultValue: "LAUNCH",
                      })}
                    </Button>
                  </motion.div>
                </div>
              </div>

              {/* Tips (Bottom) */}
              <div className="w-full rounded-xl px-4 py-2 flex items-center gap-2">
                <span className="text-lg">💡</span>
                <div className="flex-1 overflow-hidden h-[20px] relative">
                  <AnimatePresence mode="wait">
                    <motion.div
                      key={tipIndex}
                      initial={{ opacity: 0, y: 10 }}
                      animate={{ opacity: 1, y: 0 }}
                      exit={{ opacity: 0, y: -10 }}
                      className="text-sm text-default-500 font-medium truncate absolute inset-0"
                    >
                      {launchTips[tipIndex]}
                    </motion.div>
                  </AnimatePresence>
                </div>
              </div>
            </CardBody>
          </Card>
        </motion.div>

        {/* Content Grid - Responsive (1 col on mobile, 3 cols on md+) */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 lg:gap-6 items-stretch">
          {/* Mod Card */}
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.3 }}
            className="md:col-span-1"
          >
            <ModCard
              localVersionMap={localVersionMap}
              currentVersion={currentVersion}
            />
          </motion.div>

          {/* Content Management */}
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.4 }}
            className="md:col-span-1"
          >
            <Card className="h-full border-none shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl transition-all hover:bg-white/80 dark:hover:bg-zinc-900/80 group">
              <CardHeader className="px-5 py-3 border-b border-default-100 dark:border-white/5 flex justify-between items-center">
                <div className="flex items-center gap-2">
                  <div className="p-1.5 rounded-lg bg-pink-500/10 text-pink-600 dark:text-pink-400">
                    <FaCube size={16} />
                  </div>
                  <h3 className="text-base font-bold text-default-800 dark:text-zinc-100">
                    {t("launcherpage.content_manage", {
                      defaultValue: "Content Management",
                    })}
                  </h3>
                </div>
                <Button
                  size="sm"
                  variant="light"
                  className="text-xs text-default-500 dark:text-zinc-400 data-[hover=true]:text-default-800 dark:data-[hover=true]:text-zinc-200"
                  endContent={<FaArrowRight size={10} />}
                  onPress={() => navigate("/content")}
                >
                  {t("common.view_all", { defaultValue: "View All" })}
                </Button>
              </CardHeader>
              <CardBody className="p-3 gap-2 relative">
                {[
                  {
                    label: worldsLabel,
                    count: contentCounts.worlds,
                    icon: FaGlobe,
                    path: "/content/worlds",
                    color: "text-blue-500",
                  },
                  {
                    label: resourceLabel,
                    count: contentCounts.resourcePacks,
                    icon: FaImage,
                    path: "/content/resource-packs",
                    color: "text-purple-500",
                  },
                  {
                    label: behaviorLabel,
                    count: contentCounts.behaviorPacks,
                    icon: FaCogs,
                    path: "/content/behavior-packs",
                    color: "text-orange-500",
                  },
                ].map((item, idx) => (
                  <div
                    key={idx}
                    className="group/item flex items-center justify-between p-2 rounded-xl hover:bg-default-200/50 dark:hover:bg-zinc-700/50 cursor-pointer transition-all duration-200"
                    onClick={() => navigate(item.path)}
                  >
                    <div className="flex items-center gap-3">
                      <div
                        className={`p-1.5 rounded-lg bg-default-100 dark:bg-default-50/20 ${item.color} bg-opacity-20`}
                      >
                        <item.icon size={16} />
                      </div>
                      <span className="font-medium text-sm text-default-600 dark:text-zinc-200 truncate max-w-[100px] lg:max-w-none">
                        {item.label}
                      </span>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="text-base font-bold text-default-800 dark:text-zinc-200">
                        {item.count}
                      </span>
                      <FaChevronDown
                        className="text-default-300 dark:text-zinc-500 -rotate-90"
                        size={10}
                      />
                    </div>
                  </div>
                ))}
              </CardBody>
            </Card>
          </motion.div>

          {/* Content Download */}
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.5 }}
            className="md:col-span-1"
          >
            <ContentDownloadCard />
          </motion.div>
        </div>

        {/* Modal Render */}
        <BaseModal
          size="xl"
          isOpen={isOpen}
          hideCloseButton={true}
          isDismissable={false}
          onOpenChange={() => {
            onOpenChange();
            if (!isOpen) {
              setModalState(0);
              args.refresh();
            }
          }}
        >
          <ModalContent className="shadow-none">
            {(onClose) => (
              <AnimatePresence mode="wait">
                <motion.div
                  key={`modal-${modalState}`}
                  initial={{ opacity: 0, y: 10, scale: 0.98 }}
                  animate={{ opacity: 1, y: 0, scale: 1 }}
                  exit={{ opacity: 0, y: 10, scale: 0.98 }}
                  transition={{ duration: 0.22 }}
                >
                  {ModalUi(onClose)}
                </motion.div>
              </AnimatePresence>
            )}
          </ModalContent>
        </BaseModal>
      </div>
    </>
  );
};
