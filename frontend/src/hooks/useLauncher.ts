import React, { useEffect, useRef } from "react";
import { useDisclosure } from "@heroui/react";
import { Events } from "@wailsio/runtime";
import { useNavigate } from "react-router-dom";
import { compareVersions } from "@/utils/version";
import { saveCurrentVersionName } from "@/utils/currentVersion";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import {
  EnsureGameInputInteractive,
  EnsureVcRuntimeInteractive,
  IsGDKInstalled,
  ListDir,
} from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import {
  GetContentRoots,
  CheckResourcePackMaterialCompatibility,
} from "bindings/github.com/liteldev/LeviLauncher/contentservice";
import * as versionService from "bindings/github.com/liteldev/LeviLauncher/versionservice";
import * as userService from "bindings/github.com/liteldev/LeviLauncher/userservice";
import { getPlayerGamertagMap, listPlayers } from "@/utils/content";

let __didCheckGameInput = false;
let __didCheckGamingServices = false;
let __didCheckVcRuntime = false;
const IGNORE_GS_KEY = "ll.ignore.gs";

export const useLauncher = (args: any) => {
  const [isAnimating, setIsAnimating] = React.useState(true);
  let [currentVersion, setCurrentVersion] = React.useState<string>("");
  const [displayVersion, setDisplayVersion] = React.useState<string>("");
  const [displayName, setDisplayName] = React.useState<string>("");
  const [localVersionMap, setLocalVersionMap] = React.useState<
    Map<string, any>
  >(new Map());

  const launchFailedDisclosure = useDisclosure();
  const gameInputInstallingDisclosure = useDisclosure();
  const gameInputMissingDisclosure = useDisclosure();
  const vcRuntimeInstallingDisclosure = useDisclosure();
  const vcRuntimeMissingDisclosure = useDisclosure();
  const gamingServicesMissingDisclosure = useDisclosure();
  const installConfirmDisclosure = useDisclosure();
  const vcRuntimeCompletingDisclosure = useDisclosure();
  const mcLaunchLoadingDisclosure = useDisclosure();
  const shortcutSuccessDisclosure = useDisclosure();
  const registerInstallingDisclosure = useDisclosure();
  const registerSuccessDisclosure = useDisclosure();
  const registerFailedDisclosure = useDisclosure();
  const gdkMissingDisclosure = useDisclosure();

  const hasBackend = minecraft !== undefined;
  const navigate = useNavigate();
  const [launchErrorCode, setLaunchErrorCode] = React.useState<string>("");
  const [contentCounts, setContentCounts] = React.useState<{
    worlds: number;
    resourcePacks: number;
    behaviorPacks: number;
  }>({ worlds: 0, resourcePacks: 0, behaviorPacks: 0 });
  const [incompatibleShaderCount, setIncompatibleShaderCount] =
    React.useState<number>(0);
  const [giTotal, setGiTotal] = React.useState<number>(0);
  const [giDownloaded, setGiDownloaded] = React.useState<number>(0);
  const [vcTotal, setVcTotal] = React.useState<number>(0);
  const [vcDownloaded, setVcDownloaded] = React.useState<number>(0);
  const [pendingInstallCheck, setPendingInstallCheck] = React.useState<
    "gi" | "gs" | "vc" | null
  >(null);
  const [logoDataUrl, setLogoDataUrl] = React.useState<string>("");
  const [versionQuery, setVersionQuery] = React.useState<string>("");
  const [logoByName, setLogoByName] = React.useState<Map<string, string>>(
    new Map(),
  );
  const [isLoadingVersions, setIsLoadingVersions] =
    React.useState<boolean>(false);
  const fetchingLogos = React.useRef<Set<string>>(new Set());
  const [tipIndex, setTipIndex] = React.useState<number>(0);
  const tipTimerRef = React.useRef<number | null>(null);

  useEffect(() => {
    try {
      const saved = localStorage.getItem("ll.currentVersionName") || "";
      if (!saved) return;
      setCurrentVersion(saved);
      setDisplayName(saved);
      const fn = (versionService as any)?.GetVersionMeta;
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
            const getter = (versionService as any)?.GetVersionLogoDataUrl;
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
      versionService.ReconcileRegisteredFlags();
    } catch {}
  }, []);

  const startTipTimer = React.useCallback((tipsLength: number) => {
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
        setTipIndex((prev) => getNext(prev, tipsLength));
      }, 10000);
    } catch {}
  }, []);

  const stopTipTimer = React.useCallback(() => {
    try {
      if (tipTimerRef.current) {
        clearInterval(tipTimerRef.current);
        tipTimerRef.current = null;
      }
    } catch {}
  }, []);

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

  const buildVersionMenuItems = React.useCallback(
    (emptyLabel: string) => {
      if (filteredVersionNames.length === 0) {
        return [
          {
            key: "__empty",
            name: emptyLabel,
            version: "",
            isRegistered: false,
            isDisabled: true,
          },
        ];
      }
      return filteredVersionNames.map((name) => ({
        key: name,
        name,
        version: String(localVersionMap.get(name)?.version || ""),
        isRegistered: Boolean(localVersionMap.get(name)?.isRegistered),
        isDisabled: false,
        logo: logoByName.get(name),
      }));
    },
    [filteredVersionNames, localVersionMap, logoByName],
  );

  const ensureLogo = React.useCallback(
    (name: string) => {
      if (!name || logoByName.has(name) || fetchingLogos.current.has(name))
        return;
      fetchingLogos.current.add(name);
      try {
        const getter = versionService?.GetVersionLogoDataUrl;
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
      const items = buildVersionMenuItems("");
      items.forEach((it: any) => {
        if (!it?.isDisabled) ensureLogo(it.name);
      });
    } catch {}
  }, [buildVersionMenuItems, ensureLogo]);

  const doLaunch = React.useCallback(() => {
    const name = currentVersion;
    if (name) {
      saveCurrentVersionName(name);
      const launch = versionService?.LaunchVersionByName;
      if (typeof launch === "function") {
        launch(name)
          .then((err: string) => {
            const s = String(err || "");
            if (s) {
              setLaunchErrorCode(s);
              launchFailedDisclosure.onOpen();
            }
          })
          .catch(() => {
            setLaunchErrorCode("ERR_LAUNCH_GAME");
            launchFailedDisclosure.onOpen();
          });
      }
    } else {
      navigate("/versions");
    }
  }, [currentVersion, navigate, launchFailedDisclosure]);

  const doForceLaunch = React.useCallback(() => {
    const name = currentVersion;
    if (name) {
      saveCurrentVersionName(name);
      const launchForce = versionService?.LaunchVersionByNameForce;
      if (typeof launchForce === "function") {
        launchForce(name)
          .then((err: string) => {
            const s = String(err || "");
            if (s) {
              setLaunchErrorCode(s);
              launchFailedDisclosure.onOpen();
            }
          })
          .catch(() => {
            setLaunchErrorCode("ERR_LAUNCH_GAME");
            launchFailedDisclosure.onOpen();
          });
      }
    }
  }, [currentVersion, launchFailedDisclosure]);

  const doCreateShortcut = React.useCallback(() => {
    const name = currentVersion;
    if (name) {
      versionService
        ?.CreateDesktopShortcut(name)
        .then((err: string) => {
          const s = String(err || "");
          if (s) {
            setLaunchErrorCode(s);
            launchFailedDisclosure.onOpen();
          } else {
            shortcutSuccessDisclosure.onOpen();
          }
        })
        .catch(() => {
          setLaunchErrorCode("ERR_SHORTCUT_CREATE_FAILED");
          launchFailedDisclosure.onOpen();
        });
    }
  }, [currentVersion, launchFailedDisclosure, shortcutSuccessDisclosure]);

  const doOpenFolder = React.useCallback(async () => {
    if (!currentVersion) return;
    try {
      const vdir = await versionService.GetVersionsDir();
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
        gdkMissingDisclosure.onOpen();
        return;
      }
    } catch {}
    registerInstallingDisclosure.onOpen();
    try {
      const isPreview = localVersionMap.get(currentVersion)?.isPreview || false;
      const result = await versionService.RegisterVersionWithWdapp(
        currentVersion,
        isPreview,
      );
      if (result === "success" || result === "") {
        registerInstallingDisclosure.onClose();
        registerSuccessDisclosure.onOpen();
        const fn = (versionService as any)?.GetVersionMeta;
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
        registerInstallingDisclosure.onClose();
        gdkMissingDisclosure.onOpen();
      } else {
        registerInstallingDisclosure.onClose();
        setLaunchErrorCode(result);
        registerFailedDisclosure.onOpen();
      }
    } catch (e) {
      registerInstallingDisclosure.onClose();
      setLaunchErrorCode(String(e));
      registerFailedDisclosure.onOpen();
    }
  }, [
    currentVersion,
    localVersionMap,
    navigate,
    gdkMissingDisclosure,
    registerInstallingDisclosure,
    registerSuccessDisclosure,
    registerFailedDisclosure,
  ]);

  useEffect(() => {
    if (!hasBackend) return;
    const timer = setTimeout(() => {
      if (!__didCheckVcRuntime) {
        __didCheckVcRuntime = true;
        try {
          (minecraft as any)?.IsVcRuntimeInstalled?.().then((ok: boolean) => {
            if (!ok) {
              vcRuntimeMissingDisclosure.onOpen();
            }
          });
        } catch {}
      }
      if (!__didCheckGameInput) {
        __didCheckGameInput = true;
        try {
          minecraft?.IsGameInputInstalled?.().then((ok: boolean) => {
            if (!ok) {
              gameInputMissingDisclosure.onOpen();
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
                gamingServicesMissingDisclosure.onOpen();
              }
            });
          }
        } catch {}
      }
    }, 0);
    return () => clearTimeout(timer);
  }, [hasBackend, gameInputMissingDisclosure, gamingServicesMissingDisclosure]);

  useEffect(() => {
    const unlistenGiStart = Events.On("gameinput.ensure.start", () => {
      if (pendingInstallCheck === "gi") return;
      gameInputInstallingDisclosure.onOpen();
    });
    const unlistenGiDlStart = Events.On("gameinput.download.start", (event) => {
      if (pendingInstallCheck === "gi") return;
      setGiTotal(Number(event?.data || 0));
      setGiDownloaded(0);
      gameInputInstallingDisclosure.onOpen();
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
      gameInputInstallingDisclosure.onClose();
      installConfirmDisclosure.onOpen();
    });
    const unlistenGiDlError = Events.On(
      "gameinput.download.error",
      (data) => {},
    );
    const unlistenGiDone = Events.On("gameinput.ensure.done", (event) => {
      const success = Boolean(event?.data);
      gameInputInstallingDisclosure.onClose();
      if (success) {
        setPendingInstallCheck(null);
        installConfirmDisclosure.onClose();
      } else {
        setPendingInstallCheck((prev) => prev ?? "gi");
        installConfirmDisclosure.onOpen();
      }
    });

    const unlistenVcStart = Events.On("vcruntime.ensure.start", () => {
      if (pendingInstallCheck === "vc") return;
      vcRuntimeInstallingDisclosure.onOpen();
    });
    const unlistenVcDlStart = Events.On("vcruntime.download.start", (event) => {
      if (pendingInstallCheck === "vc") return;
      setVcTotal(Number(event?.data || 0));
      setVcDownloaded(0);
      vcRuntimeInstallingDisclosure.onOpen();
    });
    const unlistenVcDlProgress = Events.On(
      "vcruntime.download.progress",
      (event) => {
        if (pendingInstallCheck === "vc") return;
        const d = event?.data || {};
        if (typeof d?.Total === "number") setVcTotal(d.Total);
        if (typeof d?.Downloaded === "number") setVcDownloaded(d.Downloaded);
      },
    );
    const unlistenVcDlDone = Events.On("vcruntime.download.done", () => {
      setVcDownloaded(vcTotal);
      setPendingInstallCheck((prev) => prev ?? "vc");
      vcRuntimeInstallingDisclosure.onClose();
      installConfirmDisclosure.onOpen();
    });
    const unlistenVcDlError = Events.On(
      "vcruntime.download.error",
      (data) => {},
    );
    const unlistenVcDone = Events.On("vcruntime.ensure.done", (event) => {
      const success = Boolean(event?.data);
      vcRuntimeInstallingDisclosure.onClose();
      if (success) {
        setPendingInstallCheck(null);
        installConfirmDisclosure.onClose();
      } else {
        setPendingInstallCheck((prev) => prev ?? "vc");
        installConfirmDisclosure.onOpen();
      }
    });

    const unlistenGsMissing = Events.On("gamingservices.missing", () => {
      const ig = String(localStorage.getItem(IGNORE_GS_KEY) || "") === "1";
      if (ig) return;
      gamingServicesMissingDisclosure.onOpen();
    });

    return () => {
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
        unlistenVcStart && (unlistenVcStart as any)();
      } catch {}
      try {
        unlistenVcDlStart && (unlistenVcDlStart as any)();
      } catch {}
      try {
        unlistenVcDlProgress && (unlistenVcDlProgress as any)();
      } catch {}
      try {
        unlistenVcDlDone && (unlistenVcDlDone as any)();
      } catch {}
      try {
        unlistenVcDlError && (unlistenVcDlError as any)();
      } catch {}
      try {
        unlistenVcDone && (unlistenVcDone as any)();
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

      let res = 0;
      let bp = 0;
      let incompatibleCount = 0;
      try {
        const resEntries = await ListDir(safe.resourcePacks);
        const resDirs = (resEntries || []).filter((e: any) => e.isDir);
        res = resDirs.length;

        const promises = resDirs.map(async (dir: any) => {
          try {
            const compat = await CheckResourcePackMaterialCompatibility(
              name,
              dir.path,
            );
            if (compat.hasMaterialBin && !compat.compatible) {
              return 1;
            }
          } catch {}
          return 0;
        });
        const results = await Promise.all(promises);
        incompatibleCount = results.reduce((a: number, b) => a + b, 0);

        bp = await countDir(safe.behaviorPacks);
      } catch {}
      setIncompatibleShaderCount(incompatibleCount);

      if (safe.usersRoot) {
        try {
          const players = await listPlayers(safe.usersRoot);
          let nextPlayer = players[0] || "";

          if (nextPlayer) {
            const wp = `${safe.usersRoot}\\${nextPlayer}\\games\\com.mojang\\minecraftWorlds`;
            const defaultWorlds = await countDir(wp);
            setContentCounts({
              worlds: defaultWorlds,
              resourcePacks: res,
              behaviorPacks: bp,
            });
          } else {
            setContentCounts({
              worlds: 0,
              resourcePacks: res,
              behaviorPacks: bp,
            });
          }

          (async () => {
            try {
              const tag = await (userService as any)?.GetLocalUserGamertag?.();
              if (tag) {
                const map = await getPlayerGamertagMap(safe.usersRoot);

                let matchedPlayer = "";
                for (const p of players) {
                  if (map[p] === tag) {
                    matchedPlayer = p;
                    break;
                  }
                }

                if (matchedPlayer && matchedPlayer !== nextPlayer) {
                  const wp = `${safe.usersRoot}\\${matchedPlayer}\\games\\com.mojang\\minecraftWorlds`;
                  const newWorlds = await countDir(wp);
                  setContentCounts((prev) => ({ ...prev, worlds: newWorlds }));
                }
              }
            } catch {}
          })();
          return;
        } catch {}
      }
      setContentCounts({ worlds: 0, resourcePacks: res, behaviorPacks: bp });
    } catch {
      setContentCounts({ worlds: 0, resourcePacks: 0, behaviorPacks: 0 });
    }
  }, [hasBackend]);

  useEffect(() => {
    refreshContentCounts();
  }, [refreshContentCounts, hasBackend, currentVersion]);

  useEffect(() => {
    const unlistenMcStart = Events.On("mc.launch.start", () => {
      mcLaunchLoadingDisclosure.onOpen();
    });

    const unlistenMcDone = Events.On("mc.launch.done", () => {
      mcLaunchLoadingDisclosure.onClose();
    });
    const unlistenMcFailed = Events.On("mc.launch.failed", (data) => {
      mcLaunchLoadingDisclosure.onClose();
      const payload: any = (data as any)?.data ?? data;
      const first = Array.isArray(payload) ? payload[0] : payload;
      const code = String(first || "");
      setLaunchErrorCode(code || "ERR_LAUNCH_GAME");
      launchFailedDisclosure.onOpen();
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
      setIsLoadingVersions(true);

      const processMetas = (metas: any[]) => {
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
          const getter = versionService?.GetVersionLogoDataUrl;
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
      };

      const fastFn = (versionService as any)?.ListVersionMetas;
      const slowFn = (versionService as any)?.ListVersionMetasWithRegistered;

      if (typeof fastFn === "function") {
        fastFn()
          .then((metas: any[]) => {
            processMetas(metas);
            setIsLoadingVersions(false);

            if (typeof slowFn === "function") {
              slowFn()
                .then((fullMetas: any[]) => {
                  processMetas(fullMetas);
                })
                .catch(() => {});
            }
          })
          .catch(() => {
            if (typeof slowFn === "function") {
              slowFn()
                .then((fullMetas: any[]) => {
                  processMetas(fullMetas);
                  setIsLoadingVersions(false);
                })
                .catch(() => setIsLoadingVersions(false));
            } else {
              setIsLoadingVersions(false);
            }
          });
      } else if (typeof slowFn === "function") {
        slowFn()
          .then((metas: any[]) => {
            processMetas(metas);
            setIsLoadingVersions(false);
          })
          .catch(() => setIsLoadingVersions(false));
      } else {
        setIsLoadingVersions(false);
      }
    } else {
      setCurrentVersion("");
      setLocalVersionMap(new Map());
    }
  }, [args.count]);

  const handleVersionSelect = React.useCallback(
    (keys: any) => {
      const selected = Array.from(keys)[0] as string;
      if (selected) {
        setCurrentVersion(selected);
        setDisplayName(selected);
        const ver = localVersionMap.get(selected)?.version || "";
        setDisplayVersion(ver || "None");
        try {
          localStorage.setItem("ll.currentVersionName", selected);
          const getter = versionService?.GetVersionLogoDataUrl;
          if (typeof getter === "function") {
            getter(selected).then((u: string) =>
              setLogoDataUrl(String(u || "")),
            );
          }
        } catch {}
      }
    },
    [localVersionMap],
  );

  const handleGameInputInstall = React.useCallback(() => {
    setPendingInstallCheck("gi");
    EnsureGameInputInteractive();
    gameInputMissingDisclosure.onClose();
    installConfirmDisclosure.onOpen();
  }, [gameInputMissingDisclosure, installConfirmDisclosure]);

  const handleVcRuntimeInstall = React.useCallback(() => {
    setPendingInstallCheck("vc");
    EnsureVcRuntimeInteractive();
    vcRuntimeMissingDisclosure.onClose();
    installConfirmDisclosure.onOpen();
  }, [vcRuntimeMissingDisclosure, installConfirmDisclosure]);

  const handleGamingServicesInstall = React.useCallback(
    (openUrl: (url: string) => void) => {
      setPendingInstallCheck("gs");
      openUrl("ms-windows-store://pdp/?ProductId=9MWPM2CQNLHN");
      gamingServicesMissingDisclosure.onClose();
      installConfirmDisclosure.onOpen();
    },
    [gamingServicesMissingDisclosure, installConfirmDisclosure],
  );

  const handleIgnoreGamingServices = React.useCallback(() => {
    localStorage.setItem(IGNORE_GS_KEY, "1");
    gamingServicesMissingDisclosure.onClose();
  }, [gamingServicesMissingDisclosure]);

  const handleInstallConfirmContinue = React.useCallback(() => {
    if (pendingInstallCheck === "gi") gameInputMissingDisclosure.onOpen();
    else if (pendingInstallCheck === "gs")
      gamingServicesMissingDisclosure.onOpen();
    else if (pendingInstallCheck === "vc") vcRuntimeMissingDisclosure.onOpen();
    installConfirmDisclosure.onClose();
  }, [
    pendingInstallCheck,
    gameInputMissingDisclosure,
    gamingServicesMissingDisclosure,
    vcRuntimeMissingDisclosure,
    installConfirmDisclosure,
  ]);

  const handleInstallConfirmCheck = React.useCallback(() => {
    try {
      if (pendingInstallCheck === "gi") {
        minecraft?.IsGameInputInstalled?.().then((ok: boolean) => {
          if (ok) {
            setPendingInstallCheck(null);
            installConfirmDisclosure.onClose();
          } else {
            installConfirmDisclosure.onClose();
            gameInputMissingDisclosure.onOpen();
          }
        });
      } else if (pendingInstallCheck === "gs") {
        minecraft?.IsGamingServicesInstalled?.().then((ok: boolean) => {
          if (ok) {
            setPendingInstallCheck(null);
            installConfirmDisclosure.onClose();
          } else {
            installConfirmDisclosure.onClose();
            gamingServicesMissingDisclosure.onOpen();
          }
        });
      } else if (pendingInstallCheck === "vc") {
        (minecraft as any)?.IsVcRuntimeInstalled?.().then((ok: boolean) => {
          if (ok) {
            setPendingInstallCheck(null);
            installConfirmDisclosure.onClose();
          } else {
            installConfirmDisclosure.onClose();
            vcRuntimeMissingDisclosure.onOpen();
          }
        });
      }
    } catch {}
  }, [
    pendingInstallCheck,
    installConfirmDisclosure,
    gameInputMissingDisclosure,
    gamingServicesMissingDisclosure,
    vcRuntimeMissingDisclosure,
  ]);

  const handleInstallConfirmOpenChange = React.useCallback(
    (open: boolean) => {
      if (open) {
        installConfirmDisclosure.onOpen();
      } else {
        installConfirmDisclosure.onClose();
        args.refresh();
      }
    },
    [installConfirmDisclosure, args],
  );

  const handleRegisterSuccessOpenChange = React.useCallback(
    (open: boolean) => {
      if (open) registerSuccessDisclosure.onOpen();
      else registerSuccessDisclosure.onClose();
      if (!open) args.refresh();
    },
    [registerSuccessDisclosure, args],
  );

  const handleLaunchFailedForceRun = React.useCallback(() => {
    launchFailedDisclosure.onClose();
    doForceLaunch();
  }, [launchFailedDisclosure, doForceLaunch]);

  const handleGdkMissingGoSettings = React.useCallback(() => {
    gdkMissingDisclosure.onClose();
    navigate("/settings", { state: { tab: "components" } });
  }, [gdkMissingDisclosure, navigate]);

  return {
    // State
    isAnimating,
    setIsAnimating,
    currentVersion,
    displayVersion,
    displayName,
    localVersionMap,
    launchErrorCode,
    contentCounts,
    incompatibleShaderCount,
    giTotal,
    giDownloaded,
    vcTotal,
    vcDownloaded,
    pendingInstallCheck,
    logoDataUrl,
    versionQuery,
    setVersionQuery,
    logoByName,
    isLoadingVersions,
    tipIndex,
    hasBackend,

    // Disclosures
    launchFailedDisclosure,
    gameInputInstallingDisclosure,
    gameInputMissingDisclosure,
    vcRuntimeInstallingDisclosure,
    vcRuntimeMissingDisclosure,
    gamingServicesMissingDisclosure,
    installConfirmDisclosure,
    vcRuntimeCompletingDisclosure,
    mcLaunchLoadingDisclosure,
    shortcutSuccessDisclosure,
    registerInstallingDisclosure,
    registerSuccessDisclosure,
    registerFailedDisclosure,
    gdkMissingDisclosure,

    // Navigation
    navigate,

    // Computed
    buildVersionMenuItems,
    ensureLogo,

    // Tip timer
    startTipTimer,
    stopTipTimer,

    // Handlers
    doLaunch,
    doForceLaunch,
    doCreateShortcut,
    doOpenFolder,
    doRegister,
    handleVersionSelect,
    handleGameInputInstall,
    handleVcRuntimeInstall,
    handleGamingServicesInstall,
    handleIgnoreGamingServices,
    handleInstallConfirmContinue,
    handleInstallConfirmCheck,
    handleInstallConfirmOpenChange,
    handleRegisterSuccessOpenChange,
    handleLaunchFailedForceRun,
    handleGdkMissingGoSettings,
  };
};


