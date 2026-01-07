import React, { useEffect, useRef, useState } from "react";
import {
  Modal,
  useDisclosure,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
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
} from "@heroui/react";
import { useTranslation } from "react-i18next";
import {
  EnsureGameInputInteractive,
  GetContentRoots,
  ListDir,
} from "../../bindings/github.com/liteldev/LeviLauncher/minecraft";

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
} from "react-icons/fa";
import { ModCard } from "../components/ModdedCard";
import { CurseForgeCard } from "../components/CurseForgeCard";
import {
  ModdedChip,
  ReleaseChip,
  PreviewChip,
} from "../components/LauncherChip";
import { Events, Window, Browser } from "@wailsio/runtime";
import { motion, AnimatePresence } from "framer-motion";
import { useNavigate } from "react-router-dom";
import { compareVersions } from "../utils/version";
import { saveCurrentVersionName } from "../utils/currentVersion";
import * as minecraft from "../../bindings/github.com/liteldev/LeviLauncher/minecraft";

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
    new Map()
  );
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
    [t]
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
                setLogoDataUrl(String(u || ""))
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
      })
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
      (filteredVersionNames.length === 0
        ? [
            {
              key: "__empty",
              name: (t("common.empty", { defaultValue: "暂无数据" }) as string),
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
          }))),
    [filteredVersionNames, localVersionMap]
  );

  const ensureLogo = React.useCallback(
    (name: string) => {
      if (!name || logoByName.has(name)) return;
      try {
        const getter = minecraft?.GetVersionLogoDataUrl;
        if (typeof getter === "function") {
          getter(name).then((u: string) => {
            setLogoByName((prev) => {
              const m = new Map(prev);
              m.set(name, String(u || ""));
              return m;
            });
          });
        }
      } catch {}
    },
    [logoByName]
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
      }
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
      (data) => {}
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
                setLogoDataUrl(String(u || ""))
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
        <ModalHeader className="flex flex-col gap-1 text-red-600">
          <h2 className="text-xl font-bold">
            {t("launcherpage.launch.failed.title")}
          </h2>
        </ModalHeader>
        <ModalBody className="text-center">
          <p className="text-foreground">
            {(() => {
              const key = `errors.${launchErrorCode}`;
              const translated = t(key) as unknown as string;
              if (launchErrorCode && translated && translated !== key)
                return translated;
              return t(
                "launcherpage.launch.failed.content"
              ) as unknown as string;
            })()}
          </p>
        </ModalBody>
        <ModalFooter>
          {launchErrorCode === "ERR_GAME_ALREADY_RUNNING" && (
            <Button
              color="warning"
              variant="light"
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
            onPress={(e) => {
              onClose?.(e);
              setOverlayActive(false);
              setModalState(0);
            }}
          >
            {t("launcherpage.launch.failed.close_button")}
          </Button>
        </ModalFooter>
      </>
    ),
    7: (onClose) => (
      <>
        <ModalHeader className="flex flex-col gap-1 text-primary-600">
          <h2 className="text-xl font-bold">
            {t("launcherpage.gameinput.installing.title", {
              defaultValue: "正在安装 GameInput",
            })}
          </h2>
        </ModalHeader>
        <ModalBody className="text-center">
          <p className="text-foreground">
            {t("launcherpage.gameinput.installing.body", {
              defaultValue: "正在下载并启动安装程序，请根据系统提示完成安装。",
            })}
          </p>
          <div className="mt-2 text-sm text-default-600">
            {giTotal > 0 ? (
              <>
                <div>
                  {t("launcherpage.gameinput.installing.progress_prefix", {
                    defaultValue: "进度：",
                  })}
                  {Math.min(100, Math.floor((giDownloaded / giTotal) * 100))}%
                </div>
                <div className="font-mono">
                  {(giDownloaded / 1024 / 1024).toFixed(1)} MB /{" "}
                  {(giTotal / 1024 / 1024).toFixed(1)} MB
                </div>
              </>
            ) : (
              <div>
                {t("launcherpage.gameinput.installing.preparing", {
                  defaultValue: "正在准备下载...",
                })}
              </div>
            )}
          </div>
        </ModalBody>
      </>
    ),
    8: (onClose) => (
      <>
        <ModalHeader className="flex flex-col gap-1 text-warning-600">
          <h2 className="text-xl font-bold">
            {t("launcherpage.gameinput.missing.title", {
              defaultValue: "缺少 GameInput 组件",
            })}
          </h2>
        </ModalHeader>
        <ModalBody className="text-center">
          <p className="text-foreground">
            {t("launcherpage.gameinput.missing.body", {
              defaultValue:
                "运行游戏需要 Microsoft GameInput 组件。是否现在下载安装？",
            })}
          </p>
        </ModalBody>
        <ModalFooter>
          <Button
            color="danger"
            variant="light"
            onPress={() => {
              Window.Close();
            }}
          >
            {t("common.quit_launcher", { defaultValue: "退出启动器" })}
          </Button>
          <Button
            color="primary"
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
        </ModalFooter>
      </>
    ),
    9: (onClose) => (
      <>
        <ModalHeader className="flex flex-col gap-1 text-warning-600">
          <h2 className="text-xl font-bold">
            {t("launcherpage.gs.missing.title", {
              defaultValue: "缺少 Microsoft Gaming Services",
            })}
          </h2>
        </ModalHeader>
        <ModalBody className="text-center">
          <p className="text-gray-700">
            {t("launcherpage.gs.missing.body", {
              defaultValue:
                "未检测到 Microsoft Gaming Services。该组件是 Minecraft 运行所必须的依赖项。",
            })}
          </p>
        </ModalBody>
        <ModalFooter>
          <Button
            color="danger"
            variant="light"
            onPress={() => {
              Window.Close();
            }}
          >
            {t("common.quit_launcher", { defaultValue: "退出启动器" })}
          </Button>
          <Button
            color="default"
            variant="flat"
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
        </ModalFooter>
      </>
    ),
    10: (onClose) => (
      <>
        <ModalHeader className="flex flex-col gap-1 text-primary-600">
          <h2 className="text-xl font-bold">
            {t("launcherpage.install_confirm.title", {
              defaultValue: "是否已完成安装？",
            })}
          </h2>
        </ModalHeader>
        <ModalBody className="text-center">
          <p className="text-foreground">
            {t("launcherpage.install_confirm.body", {
              defaultValue:
                "安装完成后请点击“已完成，重新检测”。如果尚未完成，请继续安装。",
            })}
          </p>
        </ModalBody>
        <ModalFooter>
          <Button
            color="default"
            variant="light"
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
        </ModalFooter>
      </>
    ),
    4: () => (
      <>
        <ModalHeader className="flex flex-col gap-1 text-primary-600">
          <motion.h2
            className="text-xl font-bold"
            initial={{ opacity: 0, y: -10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.25 }}
          >
            {t("launcherpage.vcruntime.completing.title")}
          </motion.h2>
        </ModalHeader>
        <ModalBody className="text-center">
          <motion.p
            className="text-foreground"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ duration: 0.25, delay: 0.1 }}
          >
            {t("launcherpage.vcruntime.completing.body")}
          </motion.p>
        </ModalBody>
      </>
    ),
    5: () => (
      <>
        <ModalHeader className="flex flex-col gap-1 text-primary-600">
          <motion.h2
            className="text-xl font-bold"
            initial={{ opacity: 0, y: -10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.25 }}
          >
            {t("launcherpage.mclaunch.loading.title")}
          </motion.h2>
        </ModalHeader>
        <ModalBody className="text-center">
          <div className="flex flex-col items-center gap-3">
            <motion.div
              className="h-10 w-10 rounded-full border-2 border-default-300 border-t-default-600"
              animate={{ rotate: 360 }}
              transition={{ duration: 1, ease: "linear", repeat: Infinity }}
            />
            <motion.p
              className="text-foreground"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ duration: 0.25, delay: 0.1 }}
            >
              {t("launcherpage.mclaunch.loading.body")}
            </motion.p>
            <div className="w-full max-w-md mt-1">
              <div className="relative h-2 rounded-full bg-default-100/70 dark:bg-default-50/10 overflow-hidden border border-white/30">
                <div className="absolute top-0 bottom-0 rounded-full bg-default-400/60 indeterminate-bar1" />
                <div className="absolute top-0 bottom-0 rounded-full bg-default-400/40 indeterminate-bar2" />
              </div>
            </div>
            <div className="min-h-[24px] text-sm text-default-500">
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
        </ModalBody>
        <ModalFooter>
          <Button
            color="primary"
            onPress={(e) => {
              onClose?.();
              setOverlayActive(false);
              setModalState(0);
            }}
          >
            {t("common.close", { defaultValue: "关闭" }) as unknown as string}
          </Button>
        </ModalFooter>
      </>
    ),
    2: (onClose) => (
      <>
        <ModalHeader className="flex flex-col gap-1 text-warning-600">
          <h2 className="text-xl font-bold">
            {t("launcherpage.adminconfirm.title")}
          </h2>
        </ModalHeader>
        <ModalBody className="text-center">
          <p className="text-foreground">
            {t("launcherpage.adminconfirm.content")}
          </p>
        </ModalBody>
        <ModalFooter>
          <Button color="default" variant="light" onPress={onClose}>
            {t("launcherpage.adminconfirm.cancel_button")}
          </Button>
          <Button
            color="primary"
            onPress={(e) => {
              onClose?.(e);
            }}
          >
            {t("launcherpage.adminconfirm.confirm_button")}
          </Button>
        </ModalFooter>
      </>
    ),
    3: (onClose) => (
      <>
        <ModalHeader className="flex flex-col gap-1 text-red-600">
          <h2 className="text-xl font-bold">
            {t("launcherpage.admindeny.title")}
          </h2>
        </ModalHeader>
        <ModalBody className="text-center">
          <p className="text-foreground">
            {t("launcherpage.admindeny.content")}
          </p>
        </ModalBody>
        <ModalFooter>
          <Button color="default" variant="light" onPress={onClose}>
            {t("launcherpage.admindeny.cancel_button")}
          </Button>
          <Button
            color="primary"
            onPress={(e) => {
              onClose?.(e);
            }}
          >
            {t("launcherpage.admindeny.confirm_button")}
          </Button>
        </ModalFooter>
      </>
    ),
    12: (onClose) => (
      <>
        <ModalHeader className="flex flex-col gap-1 text-success-600">
          <h2 className="text-xl font-bold">
            {
              t("launcherpage.shortcut.success.title", {
                defaultValue: "快捷方式已创建",
              }) as unknown as string
            }
          </h2>
        </ModalHeader>
        <ModalBody className="text-center">
          <p className="text-foreground">
            {
              t("launcherpage.shortcut.success.body", {
                defaultValue: "已在桌面创建该版本的快捷方式。",
              }) as unknown as string
            }
          </p>
        </ModalBody>
        <ModalFooter>
          <Button
            color="primary"
            onPress={(e) => {
              onClose?.(e);
              setOverlayActive(false);
              setModalState(0);
            }}
          >
            {t("common.close", { defaultValue: "关闭" }) as unknown as string}
          </Button>
        </ModalFooter>
      </>
    ),
    13: (onClose) => (
      <>
        <ModalHeader className="flex flex-col gap-1 text-primary-600">
          <h2 className="text-xl font-bold">
            {t("launcherpage.register.installing.title", {
              defaultValue: "正在注册到系统",
            })}
          </h2>
        </ModalHeader>
        <ModalBody className="text-center">
          <p className="text-foreground">
            {t("launcherpage.register.installing.body", {
              defaultValue: "正在调用 wdapp.exe 执行注册，请稍候…",
            })}
          </p>
          <div className="w-full max-w-md mt-2 mx-auto">
            <div className="relative h-2 rounded-full bg-default-100/70 dark:bg-default-50/10 overflow-hidden border border-white/30">
              <div className="absolute top-0 bottom-0 rounded-full bg-default-400/60 indeterminate-bar1" />
              <div className="absolute top-0 bottom-0 rounded-full bg-default-400/40 indeterminate-bar2" />
            </div>
          </div>
        </ModalBody>
      </>
    ),
    15: (onClose) => (
      <>
        <ModalHeader className="flex flex-col gap-1 text-success-600">
          <h2 className="text-xl font-bold">
            {t("launcherpage.register.success.title", {
              defaultValue: "注册完成",
            })}
          </h2>
        </ModalHeader>
        <ModalBody className="text-center">
          <p className="text-foreground">
            {t("launcherpage.register.success.body", {
              defaultValue: "已成功注册到系统，您可以通过系统应用列表启动。",
            })}
          </p>
        </ModalBody>
        <ModalFooter>
          <Button
            color="primary"
            onPress={(e) => {
              onClose?.(e);
              setOverlayActive(false);
              setModalState(0);
            }}
          >
            {t("common.close", { defaultValue: "关闭" }) as unknown as string}
          </Button>
        </ModalFooter>
      </>
    ),
    16: (onClose) => (
      <>
        <ModalHeader className="flex flex-col gap-1 text-danger-600">
          <h2 className="text-xl font-bold">
            {t("launcherpage.register.failed.title", {
              defaultValue: "注册失败",
            })}
          </h2>
        </ModalHeader>
        <ModalBody className="text-center">
          <p className="text-foreground">
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
        </ModalBody>
        <ModalFooter>
          <Button
            color="primary"
            onPress={(e) => {
              onClose?.(e);
              setOverlayActive(false);
              setModalState(0);
            }}
          >
            {t("common.close", { defaultValue: "关闭" }) as unknown as string}
          </Button>
        </ModalFooter>
      </>
    ),
    17: (onClose) => (
      <>
        <ModalHeader className="flex flex-col gap-1 text-warning-600">
          <h2 className="text-xl font-bold">
            {t("launcherpage.gdk_missing.title", {
              defaultValue: "缺少 Microsoft GDK",
            })}
          </h2>
        </ModalHeader>
        <ModalBody className="text-center">
          <p className="text-foreground">
            {t("launcherpage.gdk_missing.body", {
              defaultValue:
                "未检测到 GDK 工具包，注册功能需先安装。是否跳转到设置页进行安装？",
            })}
          </p>
        </ModalBody>
        <ModalFooter>
          <Button
            variant="light"
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
        </ModalFooter>
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
            className="fixed inset-0 z-50 bg-black/30 backdrop-blur-sm"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.2 }}
          />
        )}
      </AnimatePresence>
      <div className="relative w-full max-w-none px-3 sm:px-5 lg:px-8 py-3 sm:py-4 lg:py-6">
        <div className="fixed inset-0 -z-10 pointer-events-none">
          <div className="absolute -top-32 -left-32 h-[28rem] w-[28rem] rounded-full bg-gradient-to-tr from-emerald-500/20 to-lime-400/20 blur-3xl" />
          <div className="absolute -bottom-32 -right-32 h-[30rem] w-[30rem] rounded-full bg-gradient-to-tr from-cyan-500/20 to-indigo-400/20 blur-3xl" />
        </div>

        <motion.div
          initial={{ opacity: 0, y: -8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.25 }}
        >
          <Card className="rounded-3xl shadow-xl mb-4 bg-white/60 dark:bg-black/30 backdrop-blur-md border border-white/30">
            <CardHeader className="flex flex-col gap-4 p-4 sm:p-6 lg:p-7">
              <div className="flex w-full items-start justify-between">
                <div className="flex items-center gap-3">
                  <div className="text-3xl sm:text-4xl font-extrabold gradient-text">
                    Minecraft
                  </div>
                  {localVersionMap.get(currentVersion)?.isPreLoader.valueOf() ||
                  false ? (
                    <ModdedChip />
                  ) : currentVersion === "" ? (
                    <></>
                  ) : localVersionMap.get(currentVersion)?.isPreview ||
                    false ? (
                    <PreviewChip />
                  ) : (
                    <ReleaseChip />
                  )}
                  {Boolean(
                    localVersionMap.get(currentVersion)?.isRegistered
                  ) ? (
                    <Chip
                      color="success"
                      variant="flat"
                      size="sm"
                      startContent={<FaCheckCircle size={12} />}
                      className="bg-gradient-to-r from-emerald-500/20 to-green-500/20 text-emerald-700 border border-emerald-400/40 shadow-sm"
                    >
                      {
                        t("launcherpage.registered_tip", {
                          defaultValue: "已注册",
                        }) as unknown as string
                      }
                    </Chip>
                  ) : null}
                </div>
                <div className="flex items-center gap-2 text-xs sm:text-sm">
                  <span className="text-default-500 font-medium mt-[2px]">
                    {t("launcherpage.currentVersion")}
                  </span>
                  <Tooltip
                    content={
                      <div className="flex flex-col gap-2">
                        <Button
                          size="sm"
                          variant="flat"
                          className="rounded-full justify-start shadow-none"
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
                          aria-label={
                            t("launcherpage.go_version_settings", {
                              defaultValue: "版本设置",
                            }) as unknown as string
                          }
                        >
                          {
                            t("launcherpage.go_version_settings", {
                              defaultValue: "版本设置",
                            }) as unknown as string
                          }
                        </Button>
                        <Button
                          size="sm"
                          variant="flat"
                          className="rounded-full justify-start shadow-none"
                          startContent={<FaWindows />}
                          onPress={doCreateShortcut}
                        >
                          {
                            t("launcherpage.shortcut.create_button", {
                              defaultValue: "创建桌面快捷方式",
                            }) as unknown as string
                          }
                        </Button>
                        <Button
                          size="sm"
                          variant="flat"
                          className="rounded-full justify-start shadow-none"
                          startContent={<FaFolderOpen />}
                          onPress={async () => {
                            try {
                              if (!currentVersion) {
                                navigate("/versions");
                                return;
                              }
                              const vdir = await (
                                minecraft as any
                              )?.GetVersionsDir?.();
                              const base = String(vdir || "");
                              if (!base) return;
                              const dir = `${base}\\${currentVersion}`;
                              await (minecraft as any)?.OpenPathDir?.(dir);
                            } catch {}
                          }}
                          aria-label={
                            t("launcherpage.open_exe_dir", {
                              defaultValue: "打开游戏安装目录",
                            }) as unknown as string
                          }
                        >
                          {
                            t("launcherpage.open_exe_dir", {
                              defaultValue: "打开游戏安装目录",
                            }) as unknown as string
                          }
                        </Button>
                        <Button
                          size="sm"
                          variant="flat"
                          className="rounded-full justify-start shadow-none"
                          startContent={<FaWindows />}
                          onPress={async () => {
                            try {
                              if (!currentVersion) return;
                              const isPrev = Boolean(
                                localVersionMap.get(currentVersion)?.isPreview
                              );
                              try {
                                const ok = await (
                                  minecraft as any
                                )?.IsGDKInstalled?.();
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
                              const fn = (minecraft as any)
                                ?.RegisterVersionWithWdapp;
                              if (typeof fn === "function") {
                                const err = await fn(currentVersion, isPrev);
                                if (err) {
                                  setLaunchErrorCode(String(err));
                                  setModalState(16);
                                  setOverlayActive(true);
                                  onOpen();
                                } else {
                                  try {
                                    const listFn =
                                      (minecraft as any)
                                        ?.ListVersionMetasWithRegistered ??
                                      (minecraft as any)?.ListVersionMetas;
                                    if (typeof listFn === "function") {
                                      const metas = await listFn();
                                      setLocalVersionMap((prev) => {
                                        const map = new Map(prev);
                                        (metas || []).forEach((m: any) => {
                                          const name = String(m?.name || "");
                                          const cur = map.get(name) || {};
                                          map.set(name, {
                                            ...cur,
                                            isRegistered: Boolean(
                                              m?.registered
                                            ),
                                            isPreview:
                                              String(
                                                m?.type ||
                                                  (cur?.isPreview
                                                    ? "preview"
                                                    : "release")
                                              ).toLowerCase() === "preview",
                                            version: String(
                                              m?.gameVersion ||
                                                cur?.version ||
                                                ""
                                            ),
                                          });
                                        });
                                        return map;
                                      });
                                    }
                                  } catch {}
                                  setModalState(15);
                                  setOverlayActive(true);
                                  onOpen();
                                }
                              }
                            } catch {}
                          }}
                        >
                          {
                            t("launcherpage.register_system_button", {
                              defaultValue: "注册到系统",
                            }) as unknown as string
                          }
                        </Button>
                      </div>
                    }
                    placement="left"
                    offset={8}
                  >
                    <div className="inline-flex items-center px-3 py-1 rounded-full bg-default-100/70 dark:bg-default-50/10 border border-white/30 backdrop-blur-sm">
                      {logoDataUrl ? (
                        <img
                          src={logoDataUrl}
                          alt="logo"
                          className="mr-2 h-6 w-6 rounded"
                        />
                      ) : null}
                      <div className="flex flex-col leading-tight">
                        <span className="text-[0.95rem] sm:text-base font-semibold text-default-900 tracking-tight">
                          {displayName ||
                            currentVersion ||
                            (t("launcherpage.currentVersion_none", {
                              defaultValue: "None",
                            }) as unknown as string)}
                        </span>
                        <span className="text-[0.70rem] sm:text-xs text-default-500">
                          {displayVersion || ""}
                        </span>
                      </div>
                    </div>
                  </Tooltip>
                </div>
              </div>
              <div className="flex w-full flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
                <div className="text-default-600 text-sm sm:text-base min-h-[24px]">
                  <AnimatePresence mode="wait">
                    <motion.span
                      key={`use-tip-${tipIndex}`}
                      initial={{ opacity: 0, y: 4 }}
                      animate={{ opacity: 1, y: 0 }}
                      exit={{ opacity: 0, y: -4 }}
                      transition={{ duration: 0.2 }}
                    >
                      {launchTips[tipIndex]}
                    </motion.span>
                  </AnimatePresence>
                </div>
                <div className="flex flex-col items-end gap-2 w-full sm:w-auto">
                  <div className="flex items-center gap-2">
                    <motion.div
                      whileHover={{ scale: 1.02 }}
                      whileTap={{ scale: 0.98 }}
                    >
                      <Button
                        size="md"
                        color="success"
                        className="text-white rounded-full px-4 sm:px-6 shadow-md flex items-center"
                        onPress={doLaunch}
                      >
                        <FaRocket className="mr-2" />
                        {t("launcherpage.launch_button")}
                      </Button>
                    </motion.div>

                    <Dropdown>
                      <DropdownTrigger>
                        <Button
                          isIconOnly
                          size="md"
                          variant="light"
                          className="rounded-full"
                          aria-label={
                            t(
                              "launcherpage.currentVersion_choose_version_aria",
                              { defaultValue: "选择版本" }
                            ) as unknown as string
                          }
                        >
                          <FaChevronDown size={16} />
                        </Button>
                      </DropdownTrigger>
                      <DropdownMenu
                        aria-label="quick-version-select"
                        selectionMode="single"
                        disallowEmptySelection
                        selectedKeys={
                          new Set(currentVersion ? [currentVersion] : [])
                        }
                        className="max-h-72 overflow-y-auto min-w-[280px] no-scrollbar"
                        topContent={
                          <div className="p-2 flex items-center gap-2">
                            <Input
                              className="flex-1"
                              size="sm"
                              placeholder={
                                t("launcherpage.search_versions", {
                                  defaultValue: "搜索版本或名称",
                                }) as unknown as string
                              }
                              value={versionQuery}
                              onValueChange={setVersionQuery}
                              autoFocus
                            />
                            <Button
                              size="sm"
                              variant="light"
                              startContent={<FaList size={14} />}
                              onPress={() => navigate("/versions")}
                            >
                              {t("launcherpage.manage_versions", {
                                defaultValue: "版本列表",
                              })}
                            </Button>
                          </div>
                        }
                        items={versionMenuItems}
                        onSelectionChange={(keys) => {
                          const arr = Array.from(
                            keys as unknown as Set<string>
                          );
                          const id = String(arr[0] || "");
                          if (!id) return;
                          const name = id;
                          setCurrentVersion(name);
                          setDisplayName(name);
                          const ver = localVersionMap.get(name)?.version || "";
                          setDisplayVersion(
                            ver ||
                              (t("launcherpage.currentVersion_none", {
                                defaultValue: "None",
                              }) as unknown as string)
                          );
                          try {
                            localStorage.setItem("ll.currentVersionName", name);
                          } catch {}
                          try {
                            const getter = minecraft?.GetVersionLogoDataUrl;
                            if (typeof getter === "function") {
                              getter(name).then((u: string) =>
                                setLogoDataUrl(String(u || ""))
                              );
                            } else {
                              setLogoDataUrl("");
                            }
                          } catch {
                            setLogoDataUrl("");
                          }
                        }}
                      >
                        {(item: {
                          key: string;
                          name: string;
                          version: string;
                          isRegistered: boolean;
                          isDisabled?: boolean;
                        }) => (
                          <DropdownItem
                            key={item.key}
                            isDisabled={item.isDisabled}
                            textValue={item.name}
                            startContent={
                              item.isDisabled
                                ? null
                                : (() => {
                                    const u = logoByName.get(item.name);
                                    if (!u) ensureLogo(item.name);
                                    return u ? (
                                      <img
                                        src={u}
                                        alt="logo"
                                        className="h-5 w-5 rounded"
                                      />
                                    ) : (
                                      <div className="h-5 w-5 rounded bg-default-200" />
                                    );
                                  })()
                            }
                          >
                            <div className="flex items-center justify-between">
                              <span
                                className="font-medium truncate max-w-[160px]"
                                title={item.name}
                              >
                                {item.name}
                              </span>
                              <div className="flex items-center gap-2 ml-2">
                                {item.isRegistered ? (
                                  <Chip
                                    size="sm"
                                    color="success"
                                    variant="flat"
                                    startContent={<FaCheckCircle size={12} />}
                                    className="bg-gradient-to-r from-emerald-500/20 to-green-500/20 text-emerald-700 border border-emerald-400/40 shadow-sm transition-transform duration-200 hover:scale-[1.03]"
                                  >
                                    {
                                      t("launcherpage.registered_tip", {
                                        defaultValue: "已注册",
                                      }) as unknown as string
                                    }
                                  </Chip>
                                ) : null}
                                <span className="text-xs text-default-500">
                                  {item.version}
                                </span>
                              </div>
                            </div>
                          </DropdownItem>
                        )}
                      </DropdownMenu>
                    </Dropdown>
                  </div>
                </div>
              </div>
            </CardHeader>
          </Card>
        </motion.div>

        <div className="grid items-stretch gap-3 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3">
          <motion.div
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.25, delay: 0.1 }}
          >
            <ModCard
              localVersionMap={localVersionMap}
              currentVersion={currentVersion}
            />
          </motion.div>

          <motion.div
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.25, delay: 0.15 }}
          >
            <CurseForgeCard />
          </motion.div>

          <motion.div
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.25, delay: 0.2 }}
          >
            <Card className="rounded-2xl shadow-md h-full min-h-[160px] bg-white/70 dark:bg-black/30 backdrop-blur-md border border-white/30">
              <CardBody className="relative p-4 sm:p-5 flex flex-col gap-3 text-left">
                <div className="flex items-center gap-2">
                  <span className="font-bold text-lg">
                    {t("launcherpage.content_manage", {
                      defaultValue: "内容管理",
                    })}
                  </span>
                </div>
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 mt-2">
                  <div className="rounded-xl border border-white/30 bg-default-100/70 dark:bg-default-50/10 shadow-sm backdrop-blur-sm px-3 py-2 cursor-pointer hover:shadow-md transition-all duration-200 hover:bg-default-100 dark:hover:bg-default-50/15"
                    onClick={() => navigate("/content/worlds")}>
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2 min-w-0">
                        <FaGlobe className="text-default-500" />
                        <span
                          className={`text-default-600 ${labelSizeClass(
                            worldsLabel
                          )} truncate max-w-[140px] sm:max-w-[160px]`}
                        >
                          {worldsLabel}
                        </span>
                      </div>
                      <span className="text-base font-semibold text-default-800">
                        {contentCounts.worlds}
                      </span>
                    </div>
                  </div>
                  <div className="rounded-xl border border-white/30 bg-default-100/70 dark:bg-default-50/10 shadow-sm backdrop-blur-sm px-3 py-2 cursor-pointer hover:shadow-md transition-all duration-200 hover:bg-default-100 dark:hover:bg-default-50/15"
                    onClick={() => navigate("/content/resource-packs")}>
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2 min-w-0">
                        <FaImage className="text-default-500" />
                        <span
                          className={`text-default-600 ${labelSizeClass(
                            resourceLabel
                          )} truncate max-w-[140px] sm:max-w-[160px]`}
                        >
                          {resourceLabel}
                        </span>
                      </div>
                      <span className="text-base font-semibold text-default-800">
                        {contentCounts.resourcePacks}
                      </span>
                    </div>
                  </div>
                  <div className="rounded-xl border border-white/30 bg-default-100/70 dark:bg-default-50/10 shadow-sm backdrop-blur-sm px-3 py-2 cursor-pointer hover:shadow-md transition-all duration-200 hover:bg-default-100 dark:hover:bg-default-50/15"
                    onClick={() => navigate("/content/behavior-packs")}>
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2 min-w-0">
                        <FaCogs className="text-default-500" />
                        <span
                          className={`text-default-600 ${labelSizeClass(
                            behaviorLabel
                          )} truncate max-w-[140px] sm:max-w-[160px]`}
                        >
                          {behaviorLabel}
                        </span>
                      </div>
                      <span className="text-base font-semibold text-default-800">
                        {contentCounts.behaviorPacks}
                      </span>
                    </div>
                  </div>
                </div>
                <div className="absolute bottom-3 right-3">
                  <Tooltip
                    content={
                      t("launcherpage.content_manage", {
                        defaultValue: "内容管理",
                      }) as unknown as string
                    }
                    placement="left"
                  >
                    <Button
                      size="sm"
                      variant="light"
                      className="rounded-full px-4"
                      onPress={() => navigate("/content")}
                    >
                      {t("common.go", { defaultValue: "前往" })}
                    </Button>
                  </Tooltip>
                </div>
              </CardBody>
            </Card>
          </motion.div>
        </div>

        <Modal
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
          <ModalContent>
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
        </Modal>
      </div>
    </>
  );
};
