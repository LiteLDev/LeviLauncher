import { useEffect, useState, useMemo } from "react";
import { useNavigate, useLocation } from "react-router-dom";
import { useDisclosure } from "@heroui/react";
import {
  GetAppVersion,
  CheckUpdate,
  GetLanguageNames,
  GetBaseRoot,
  IsGDKInstalled,
  InstallGDKFromZip,
  GetDisableDiscordRPC,
  GetEnableBetaUpdates,
  GetLipVersion,
  IsLipInstalled,
  GetLatestLipVersion,
  GetResourceRulesStatus,
  UpdateResourceRules,
  ListMinecraftProcesses,
  KillProcess,
  KillAllMinecraftProcesses,
} from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import {
  GetInstallerDir,
  GetVersionsDir,
} from "bindings/github.com/liteldev/LeviLauncher/versionservice";
import { Events } from "@wailsio/runtime";
import * as types from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { normalizeLanguage } from "@/utils/i18nUtils";
import { useThemeManager, ThemeMode } from "@/utils/useThemeManager";

export type { ThemeMode };

export const useSettings = (i18n: { language: string }) => {
  const hasBackend = minecraft !== undefined;
  const navigate = useNavigate();
  const location = useLocation();

  // App version / update
  const [appVersion, setAppVersion] = useState("");
  const [checkingUpdate, setCheckingUpdate] = useState(false);
  const [updating, setUpdating] = useState(false);
  const [newVersion, setNewVersion] = useState("");
  const [hasUpdate, setHasUpdate] = useState(false);
  const [changelog, setChangelog] = useState<string>("");

  // Language
  const [langNames, setLangNames] = useState<Array<types.LanguageJson>>([]);
  const [selectedLang, setSelectedLang] = useState<string>("");
  const [languageChanged, setLanguageChanged] = useState<boolean>(false);

  // Base root / paths
  const [baseRoot, setBaseRoot] = useState<string>("");
  const [installerDir, setInstallerDir] = useState<string>("");
  const [versionsDir, setVersionsDir] = useState<string>("");
  const [newBaseRoot, setNewBaseRoot] = useState<string>("");
  const [savingBaseRoot, setSavingBaseRoot] = useState<boolean>(false);
  const [baseRootWritable, setBaseRootWritable] = useState<boolean>(true);

  // Discord RPC / Beta updates
  const [discordRpcEnabled, setDiscordRpcEnabled] = useState<boolean>(true);
  const [enableBetaUpdates, setEnableBetaUpdatesState] = useState<boolean>(false);

  // GDK
  const [gdkInstalled, setGdkInstalled] = useState<boolean>(false);
  const [gdkDlProgress, setGdkDlProgress] = useState<{
    downloaded: number;
    total: number;
    dest?: string;
  } | null>(null);
  const [gdkDlSpeed, setGdkDlSpeed] = useState<number>(0);
  const [gdkDlStatus, setGdkDlStatus] = useState<string>("");
  const [gdkDlError, setGdkDlError] = useState<string>("");
  const gdkProgressDisclosure = useDisclosure();
  const gdkLicenseDisclosure = useDisclosure();
  const gdkInstallDisclosure = useDisclosure();
  const [gdkLicenseAccepted, setGdkLicenseAccepted] = useState<boolean>(false);

  // Tabs
  const [selectedTab, setSelectedTab] = useState<string>("general");

  // Layout mode
  const [layoutMode, setLayoutMode] = useState<"navbar" | "sidebar">(() => {
    try {
      return (
        (localStorage.getItem("app.layoutMode") as "navbar" | "sidebar") ||
        "sidebar"
      );
    } catch {
      return "sidebar";
    }
  });

  // Animations
  const [disableAnimations, setDisableAnimations] = useState<boolean>(() => {
    try {
      return localStorage.getItem("app.disableAnimations") === "true";
    } catch {
      return false;
    }
  });

  // Theme colors (light/dark)
  const [lightThemeColor, setLightThemeColor] = useState<string>(() => {
    try {
      return (
        localStorage.getItem("app.lightThemeColor") ||
        localStorage.getItem("app.themeColor") ||
        "emerald"
      );
    } catch {
      return "emerald";
    }
  });

  const [darkThemeColor, setDarkThemeColor] = useState<string>(() => {
    try {
      return (
        localStorage.getItem("app.darkThemeColor") ||
        localStorage.getItem("app.themeColor") ||
        "emerald"
      );
    } catch {
      return "emerald";
    }
  });

  const [lightCustomThemeColor, setLightCustomThemeColor] = useState<string>(
    () => {
      try {
        return (
          localStorage.getItem("app.lightCustomThemeColor") ||
          localStorage.getItem("app.customThemeColor") ||
          "#10b981"
        );
      } catch {
        return "#10b981";
      }
    },
  );

  const [darkCustomThemeColor, setDarkCustomThemeColor] = useState<string>(
    () => {
      try {
        return (
          localStorage.getItem("app.darkCustomThemeColor") ||
          localStorage.getItem("app.customThemeColor") ||
          "#10b981"
        );
      } catch {
        return "#10b981";
      }
    },
  );

  // Background image settings
  const [backgroundImage, setBackgroundImage] = useState<string>(
    () => localStorage.getItem("app.backgroundImage") || "",
  );

  const [backgroundBlur, setBackgroundBlur] = useState<number>(() =>
    Number(localStorage.getItem("app.backgroundBlur") || "0"),
  );

  const [backgroundBrightness, setBackgroundBrightness] = useState<number>(
    () => {
      const item = localStorage.getItem("app.backgroundBrightness");
      return item !== null ? Number(item) : 100;
    },
  );

  const [backgroundOpacity, setBackgroundOpacity] = useState<number>(() => {
    const item = localStorage.getItem("app.backgroundOpacity");
    return item !== null ? Number(item) : 100;
  });

  const [backgroundPlayOrder, setBackgroundPlayOrder] = useState<
    "random" | "sequential"
  >(
    () =>
      (localStorage.getItem("app.backgroundPlayOrder") as
        | "random"
        | "sequential") || "random",
  );

  const [backgroundFitMode, setBackgroundFitMode] = useState<string>(
    () => localStorage.getItem("app.backgroundFitMode") || "smart",
  );

  // Background base mode (light/dark)
  const [lightBackgroundBaseMode, setLightBackgroundBaseMode] =
    useState<string>(
      () => localStorage.getItem("app.lightBackgroundBaseMode") || "none",
    );

  const [darkBackgroundBaseMode, setDarkBackgroundBaseMode] = useState<string>(
    () => localStorage.getItem("app.darkBackgroundBaseMode") || "none",
  );

  const [lightBackgroundBaseColor, setLightBackgroundBaseColor] =
    useState<string>(
      () => localStorage.getItem("app.lightBackgroundBaseColor") || "#ffffff",
    );

  const [darkBackgroundBaseColor, setDarkBackgroundBaseColor] =
    useState<string>(
      () => localStorage.getItem("app.darkBackgroundBaseColor") || "#18181b",
    );

  const [lightBackgroundBaseOpacity, setLightBackgroundBaseOpacity] =
    useState<number>(() => {
      const item = localStorage.getItem("app.lightBackgroundBaseOpacity");
      return item !== null ? Number(item) : 50;
    });

  const [darkBackgroundBaseOpacity, setDarkBackgroundBaseOpacity] =
    useState<number>(() => {
      const item = localStorage.getItem("app.darkBackgroundBaseOpacity");
      return item !== null ? Number(item) : 50;
    });

  // Theme manager
  const {
    themeMode,
    setThemeMode,
    scheduleStart,
    setScheduleStart,
    scheduleEnd,
    setScheduleEnd,
    sunTimes,
    fetchSunTimes,
    resolvedTheme,
  } = useThemeManager();

  const [themeSettingMode, setThemeSettingMode] = useState<"light" | "dark">(
    () => {
      const currentTheme = resolvedTheme || "light";
      return currentTheme === "dark" ? "dark" : "light";
    },
  );

  useEffect(() => {
    if (resolvedTheme === "dark" || resolvedTheme === "light") {
      setThemeSettingMode(resolvedTheme);
    }
  }, [resolvedTheme]);

  // Background image preview
  const [backgroundImageError, setBackgroundImageError] = useState(false);
  const [backgroundImageCount, setBackgroundImageCount] = useState<number>(0);
  const [previewBgData, setPreviewBgData] = useState<string>("");

  // Lip
  const [lipInstalled, setLipInstalled] = useState<boolean>(false);
  const [lipVersion, setLipVersion] = useState<string>("");
  const [lipLatestVersion, setLipLatestVersion] = useState<string>("");
  const [installingLip, setInstallingLip] = useState<boolean>(false);
  const [lipStatus, setLipStatus] = useState<string>("");
  const [lipProgress, setLipProgress] = useState<{
    percentage: number;
    current: number;
    total: number;
  }>({ percentage: 0, current: 0, total: 0 });
  const [lipError, setLipError] = useState<string>("");
  const lipProgressDisclosure = useDisclosure();

  // resource_pack_rules.bin
  const [resourceRulesInstalled, setResourceRulesInstalled] =
    useState<boolean>(false);
  const [resourceRulesUpToDate, setResourceRulesUpToDate] =
    useState<boolean>(false);
  const [resourceRulesLocalSha, setResourceRulesLocalSha] =
    useState<string>("");
  const [resourceRulesRemoteSha, setResourceRulesRemoteSha] =
    useState<string>("");
  const [resourceRulesError, setResourceRulesError] = useState<string>("");
  const [resourceRulesChecking, setResourceRulesChecking] =
    useState<boolean>(false);
  const [resourceRulesUpdating, setResourceRulesUpdating] =
    useState<boolean>(false);

  // Process management
  const [processModalOpen, setProcessModalOpen] = useState(false);
  const [processes, setProcesses] = useState<types.ProcessInfo[]>([]);
  const [scanningProcesses, setScanningProcesses] = useState(false);

  // Sun times loading
  const [loadingSunTimes, setLoadingSunTimes] = useState(false);

  // Unsaved changes / navigation
  const {
    isOpen: unsavedOpen,
    onOpen: unsavedOnOpen,
    onOpenChange: unsavedOnOpenChange,
    onClose: unsavedOnClose,
  } = useDisclosure();
  const [pendingNavPath, setPendingNavPath] = useState<string>("");
  const {
    isOpen: resetOpen,
    onOpen: resetOnOpen,
    onOpenChange: resetOnOpenChange,
    onClose: resetOnClose,
  } = useDisclosure();

  // --- Handlers ---

  const refreshSunTimes = async () => {
    setLoadingSunTimes(true);
    await fetchSunTimes();
    setLoadingSunTimes(false);
  };

  const refreshProcesses = async () => {
    setScanningProcesses(true);
    try {
      const list = await ListMinecraftProcesses();
      setProcesses(list || []);
    } finally {
      setScanningProcesses(false);
    }
  };

  const handleKillProcess = async (pid: number) => {
    try {
      await KillProcess(pid);
      await refreshProcesses();
    } catch {}
  };

  const handleKillAllProcesses = async () => {
    try {
      await KillAllMinecraftProcesses();
      await refreshProcesses();
    } catch {}
  };

  const onCheckUpdate = async () => {
    setCheckingUpdate(true);
    try {
      const res = await CheckUpdate();
      setHasUpdate(res.isUpdate);
      setNewVersion(res.version || "");
      try {
        const body = (res as any)?.body || "";
        setChangelog(String(body || ""));
      } catch {}
    } finally {
      setCheckingUpdate(false);
    }
  };

  const onUpdate = async () => {
    setUpdating(true);
    try {
      navigate("/updating", { replace: true });
    } finally {
      setUpdating(false);
    }
  };

  const refreshResourceRulesStatus = async () => {
    setResourceRulesChecking(true);
    try {
      const s = (await GetResourceRulesStatus()) as any;
      setResourceRulesInstalled(Boolean(s?.installed));
      setResourceRulesUpToDate(Boolean(s?.upToDate));
      setResourceRulesLocalSha(String(s?.localSha || ""));
      setResourceRulesRemoteSha(String(s?.remoteSha || ""));
      setResourceRulesError(String(s?.error || ""));
    } catch (e: any) {
      setResourceRulesError(String(e?.message || e || "check failed"));
      setResourceRulesUpToDate(false);
    } finally {
      setResourceRulesChecking(false);
    }
  };

  const onUpdateResourceRules = async () => {
    setResourceRulesUpdating(true);
    try {
      const err = await UpdateResourceRules();
      if (err) {
        setResourceRulesError(String(err));
      }
      await refreshResourceRulesStatus();
    } catch (e: any) {
      setResourceRulesError(String(e?.message || e || "update failed"));
    } finally {
      setResourceRulesUpdating(false);
    }
  };

  // --- Computed ---

  const hasLipUpdate = useMemo(() => {
    if (!lipInstalled || !lipLatestVersion || !lipVersion) return false;
    return lipLatestVersion.replace(/^v/, "") !== lipVersion.replace(/^v/, "");
  }, [lipInstalled, lipLatestVersion, lipVersion]);

  // --- Effects ---

  useEffect(() => {
    if (processModalOpen) {
      refreshProcesses();
    }
  }, [processModalOpen]);

  // Unsaved changes navigation guard
  useEffect(() => {
    const handler = (ev: any) => {
      try {
        let targetPath = ev?.detail?.path;
        if (targetPath === -1) targetPath = "-1";
        targetPath = String(targetPath || "");
        const hasUnsaved = !!newBaseRoot && newBaseRoot !== baseRoot;
        if (!targetPath || targetPath === location.pathname) return;
        if (hasUnsaved) {
          setPendingNavPath(targetPath);
          unsavedOnOpen();
          return;
        }
        if (targetPath === "-1") {
          navigate(-1);
        } else {
          navigate(targetPath);
        }
      } catch {}
    };
    window.addEventListener("ll-try-nav", handler as any);
    return () => window.removeEventListener("ll-try-nav", handler as any);
  }, [
    newBaseRoot,
    baseRoot,
    baseRootWritable,
    navigate,
    location.pathname,
    unsavedOnOpen,
  ]);

  // Initial data loading
  useEffect(() => {
    GetAppVersion().then((version) => {
      setAppVersion(version);
    });

    GetLanguageNames().then((res) => setLangNames(res));

    setSelectedLang(normalizeLanguage(i18n.language));
    setLanguageChanged(false);
    Promise.resolve()
      .then(async () => {
        try {
          if (hasBackend) {
            const br = await GetBaseRoot();
            setBaseRoot(String(br || ""));
            setNewBaseRoot(String(br || ""));
            const id = await GetInstallerDir();
            setInstallerDir(String(id || ""));
            const vd = await GetVersionsDir();
            setVersionsDir(String(vd || ""));
            try {
              const disabled = await GetDisableDiscordRPC();
              setDiscordRpcEnabled(!disabled);
            } catch {}
            try {
              const enabled = await GetEnableBetaUpdates();
              setEnableBetaUpdatesState(enabled);
            } catch {}
            try {
              const ok = await IsGDKInstalled();
              setGdkInstalled(Boolean(ok));
            } catch {}
          }
        } catch {}
      })
      .catch(() => {});
  }, []);

  useEffect(() => {
    setBaseRootWritable(true);
  }, [newBaseRoot]);

  useEffect(() => {
    setBackgroundImageError(false);
  }, [backgroundImage]);

  // Background image preview loading
  useEffect(() => {
    const folderPath = backgroundImage;

    if (!folderPath) {
      setPreviewBgData("");
      setBackgroundImageCount(0);
      return;
    }

    const loadPreview = async () => {
      try {
        // Try to get current image from App.tsx first
        const currentPath = localStorage.getItem("app.currentBackgroundImage");

        let previewPath = "";
        if (currentPath && currentPath.startsWith(folderPath)) {
          previewPath = currentPath;
        }

        const entries = await (minecraft as any).ListDir(folderPath);
        if (!entries || entries.length === 0) {
          setPreviewBgData("");
          setBackgroundImageCount(0);
          return;
        }
        const images = entries.filter((e: any) => {
          const name = e.name.toLowerCase();
          return (
            !e.isDir &&
            (name.endsWith(".png") ||
              name.endsWith(".jpg") ||
              name.endsWith(".jpeg") ||
              name.endsWith(".webp") ||
              name.endsWith(".gif") ||
              name.endsWith(".bmp"))
          );
        });
        setBackgroundImageCount(images.length);

        if (images.length === 0) {
          setPreviewBgData("");
          return;
        }

        if (!previewPath) {
          // Use the first image for preview in settings if no current image
          previewPath = images[0].path;
        }

        const res = await (minecraft as any).GetImageBase64?.(previewPath);
        if (res) {
          setPreviewBgData(res);
          setBackgroundImageError(false);
        } else {
          // Fallback to first image if current fails
          const fallbackPath = images[0].path;
          const fallbackRes = await (minecraft as any).GetImageBase64?.(
            fallbackPath,
          );
          if (fallbackRes) {
            setPreviewBgData(fallbackRes);
            setBackgroundImageError(false);
          } else {
            setBackgroundImageError(true);
          }
        }
      } catch {
        setBackgroundImageError(true);
      }
    };

    loadPreview();
  }, [backgroundImage]);

  // Tab from location state
  useEffect(() => {
    if (location.state && (location.state as any).tab) {
      setSelectedTab((location.state as any).tab);
    }
  }, [location.state]);

  // GDK download/install events
  useEffect(() => {
    if (!hasBackend) return;
    const offs: (() => void)[] = [];
    try {
      const speedRef: { ts: number; bytes: number; ema: number } = {
        ts: 0,
        bytes: 0,
        ema: 0,
      } as any;
      offs.push(
        Events.On("gdk_download_progress", (event) => {
          const downloaded = Number(event?.data?.Downloaded || 0);
          const total = Number(event?.data?.Total || 0);
          const dest = String(event?.data?.Dest || "");
          setGdkDlProgress({ downloaded, total, dest });
          try {
            const now = Date.now();
            if (speedRef.ts > 0) {
              const dt = (now - speedRef.ts) / 1000;
              const db = downloaded - speedRef.bytes;
              const inst = dt > 0 && db >= 0 ? db / dt : 0;
              const alpha = 0.25;
              speedRef.ema =
                alpha * inst + (1 - alpha) * (speedRef.ema || inst);
              setGdkDlSpeed(speedRef.ema);
            }
            speedRef.ts = now;
            speedRef.bytes = downloaded;
          } catch {}
        }),
      );
      offs.push(
        Events.On("gdk_download_status", (event) => {
          const s = String(event?.data || "");
          setGdkDlStatus(s);
          if (s === "started" || s === "resumed" || s === "cancelled") {
            setGdkDlError("");
            try {
              (window as any).__gdkDlLast = null;
            } catch {}
            setGdkDlSpeed(0);
          }
        }),
      );
      offs.push(
        Events.On("gdk_download_error", (event) => {
          setGdkDlError(String(event?.data || ""));
        }),
      );
      offs.push(
        Events.On("gdk_download_done", async (event) => {
          const dest = String(event?.data || gdkDlProgress?.dest || "");
          gdkProgressDisclosure.onClose();
          try {
            gdkInstallDisclosure.onOpen();
            await InstallGDKFromZip(dest);
          } catch {}
        }),
      );
      offs.push(
        Events.On("gdk_install_done", (_event) => {
          gdkInstallDisclosure.onClose();
          setTimeout(async () => {
            try {
              const ok = await IsGDKInstalled();
              setGdkInstalled(Boolean(ok));
            } catch {}
          }, 500);
        }),
      );
      offs.push(
        Events.On("gdk_install_error", (event) => {
          gdkInstallDisclosure.onClose();
          setGdkDlError(String(event?.data || ""));
        }),
      );
    } catch {}
    return () => {
      for (const off of offs) {
        try {
          off();
        } catch {}
      }
    };
  }, [hasBackend]);

  // Lip install events
  useEffect(() => {
    if (!hasBackend) return;
    IsLipInstalled().then((ok) => {
      setLipInstalled(Boolean(ok));
      if (ok) {
        GetLipVersion().then((v) => setLipVersion(String(v || "")));
      }
    });
    GetLatestLipVersion()
      .then((v) => setLipLatestVersion(String(v || "")))
      .catch(() => {});

    const offs: (() => void)[] = [];
    offs.push(
      Events.On("lip_install_status", (e) =>
        setLipStatus(String(e?.data || "")),
      ),
    );
    offs.push(
      Events.On("lip_install_progress", (e) => {
        const data = e?.data as any;
        setLipProgress({
          percentage: Number(data?.percentage || 0),
          current: Number(data?.current || 0),
          total: Number(data?.total || 0),
        });
      }),
    );
    offs.push(
      Events.On("lip_install_done", (e) => {
        setInstallingLip(false);
        setLipStatus("done");
        setLipProgress({ percentage: 100, current: 0, total: 0 });
        setLipInstalled(true);
        setLipVersion(String(e?.data || ""));
        setLipError("");
      }),
    );
    offs.push(
      Events.On("lip_install_error", (e) => {
        setInstallingLip(false);
        setLipStatus("");
        setLipError(String(e?.data || ""));
      }),
    );
    return () => offs.forEach((off) => off());
  }, [hasBackend]);

  useEffect(() => {
    if (!hasBackend) return;
    refreshResourceRulesStatus();
  }, [hasBackend]);

  return {
    // Computed
    hasBackend,
    hasLipUpdate,

    // Navigation
    navigate,
    location,

    // App version / update
    appVersion,
    checkingUpdate,
    updating,
    newVersion,
    hasUpdate,
    changelog,
    onCheckUpdate,
    onUpdate,

    // Language
    langNames,
    selectedLang,
    setSelectedLang,
    languageChanged,
    setLanguageChanged,

    // Base root / paths
    baseRoot,
    setBaseRoot,
    installerDir,
    setInstallerDir,
    versionsDir,
    setVersionsDir,
    newBaseRoot,
    setNewBaseRoot,
    savingBaseRoot,
    setSavingBaseRoot,
    baseRootWritable,
    setBaseRootWritable,

    // Discord RPC / Beta updates
    discordRpcEnabled,
    setDiscordRpcEnabled,
    enableBetaUpdates,
    setEnableBetaUpdates: setEnableBetaUpdatesState,

    // GDK
    gdkInstalled,
    gdkDlProgress,
    gdkDlSpeed,
    gdkDlStatus,
    gdkDlError,
    setGdkDlError,
    setGdkDlProgress,
    gdkProgressDisclosure,
    gdkLicenseDisclosure,
    gdkInstallDisclosure,
    gdkLicenseAccepted,
    setGdkLicenseAccepted,

    // Tabs
    selectedTab,
    setSelectedTab,

    // Layout mode
    layoutMode,
    setLayoutMode,

    // Animations
    disableAnimations,
    setDisableAnimations,

    // Theme colors
    lightThemeColor,
    setLightThemeColor,
    darkThemeColor,
    setDarkThemeColor,
    lightCustomThemeColor,
    setLightCustomThemeColor,
    darkCustomThemeColor,
    setDarkCustomThemeColor,

    // Background image
    backgroundImage,
    setBackgroundImage,
    backgroundBlur,
    setBackgroundBlur,
    backgroundBrightness,
    setBackgroundBrightness,
    backgroundOpacity,
    setBackgroundOpacity,
    backgroundPlayOrder,
    setBackgroundPlayOrder,
    backgroundFitMode,
    setBackgroundFitMode,
    backgroundImageError,
    setBackgroundImageError,
    backgroundImageCount,
    previewBgData,

    // Background base mode
    lightBackgroundBaseMode,
    setLightBackgroundBaseMode,
    darkBackgroundBaseMode,
    setDarkBackgroundBaseMode,
    lightBackgroundBaseColor,
    setLightBackgroundBaseColor,
    darkBackgroundBaseColor,
    setDarkBackgroundBaseColor,
    lightBackgroundBaseOpacity,
    setLightBackgroundBaseOpacity,
    darkBackgroundBaseOpacity,
    setDarkBackgroundBaseOpacity,

    // Theme manager
    themeMode,
    setThemeMode,
    scheduleStart,
    setScheduleStart,
    scheduleEnd,
    setScheduleEnd,
    sunTimes,
    resolvedTheme,
    themeSettingMode,
    setThemeSettingMode,
    loadingSunTimes,
    refreshSunTimes,

    // Lip
    lipInstalled,
    lipVersion,
    lipLatestVersion,
    installingLip,
    setInstallingLip,
    lipStatus,
    lipProgress,
    lipError,
    setLipError,
    lipProgressDisclosure,

    // Resource rules
    resourceRulesInstalled,
    resourceRulesUpToDate,
    resourceRulesLocalSha,
    resourceRulesRemoteSha,
    resourceRulesError,
    resourceRulesChecking,
    resourceRulesUpdating,
    refreshResourceRulesStatus,
    onUpdateResourceRules,

    // Process management
    processModalOpen,
    setProcessModalOpen,
    processes,
    scanningProcesses,
    refreshProcesses,
    handleKillProcess,
    handleKillAllProcesses,

    // Unsaved changes / reset modals
    unsavedOpen,
    unsavedOnClose,
    unsavedOnOpenChange,
    pendingNavPath,
    resetOpen,
    resetOnOpen,
    resetOnOpenChange,
    resetOnClose,
  };
};
