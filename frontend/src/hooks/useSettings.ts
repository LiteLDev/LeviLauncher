import { useOverlayState } from "@heroui/react";
import { useEffect, useRef, useState } from "react";
import { useNavigate, useLocation } from "react-router-dom";

import {
  CacheClean,
  GetAppVersion,
  CheckUpdate,
  GetLanguageNames,
  GetBaseRoot,
  GetDisableDiscordRPC,
  GetEnableBetaUpdates,
  GetGameLaunchBehavior,
  SetGameLaunchBehavior,
  GetGameExitBehavior,
  SetGameExitBehavior,
  GetMinimizeToTray,
  SetMinimizeToTray,
  ListMinecraftProcesses,
  KillProcess,
  KillAllMinecraftProcesses,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import {
  GetInstallerDir,
  GetVersionsDir,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/versionservice";
import * as types from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import {
  persistAnalyticsChoice,
  readAnalyticsEnabled,
} from "@/utils/analyticsConsent";
import {
  EXPERIMENTAL_FEATURES_EVENT_NAME,
  persistExperimentalInstanceBackupEnabled,
  readExperimentalInstanceBackupEnabled,
} from "@/utils/experimentalFeatures";
import { normalizeLanguage } from "@/utils/i18nUtils";
import { useThemeManager, ThemeMode } from "@/utils/useThemeManager";
import { ROUTES } from "@/constants/routes";
import { useLipTaskConsole } from "@/utils/LipTaskConsoleContext";
import { useCurrentBackground } from "@/utils/BackgroundContext";
import { clampNumber } from "@/utils/backgroundAppearance";

export type { ThemeMode };

export const useSettings = (i18n: { language: string }) => {
  const currentBackground = useCurrentBackground();
  const navigate = useNavigate();
  const { runWithLipTask } = useLipTaskConsole();
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
  const [enableBetaUpdates, setEnableBetaUpdatesState] =
    useState<boolean>(false);
  const [analyticsEnabled, setAnalyticsEnabledState] =
    useState<boolean>(readAnalyticsEnabled);
  const [
    experimentalInstanceBackupEnabled,
    setExperimentalInstanceBackupEnabledState,
  ] = useState<boolean>(() => readExperimentalInstanceBackupEnabled());

  // Game lifecycle behaviors
  const [gameLaunchBehavior, setGameLaunchBehaviorState] =
    useState<string>("minimize");
  const [gameExitBehavior, setGameExitBehaviorState] =
    useState<string>("reopen");
  const [minimizeToTray, setMinimizeToTrayState] =
    useState<boolean>(false);

  // Tabs
  const [selectedTab, setSelectedTab] = useState<string>(() => {
    try {
      const saved = localStorage.getItem("app.settingsTab") || "";
      if (["general", "personalization", "components", "others", "privacy", "updates", "about"].includes(saved)) return saved;
    } catch {}
    return "general";
  });
  useEffect(() => {
    try { localStorage.setItem("app.settingsTab", selectedTab); } catch {}
  }, [selectedTab]);

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
    clampNumber(localStorage.getItem("app.backgroundBlur"), 0, 0, 50),
  );

  const [backgroundBrightness, setBackgroundBrightness] = useState<number>(
    () => {
      const item = localStorage.getItem("app.backgroundBrightness");
      return clampNumber(item, 100, 0, 200);
    },
  );

  const [backgroundOpacity, setBackgroundOpacity] = useState<number>(() => {
    const item = localStorage.getItem("app.backgroundOpacity");
    return clampNumber(item, 100, 0, 100);
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
      return clampNumber(item, 50, 0, 100);
    });

  const [darkBackgroundBaseOpacity, setDarkBackgroundBaseOpacity] =
    useState<number>(() => {
      const item = localStorage.getItem("app.darkBackgroundBaseOpacity");
      return clampNumber(item, 50, 0, 100);
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
  const previewBgData = currentBackground?.bgData || "";

  // Lip
  const [cleaningLipCache, setCleaningLipCache] = useState<boolean>(false);

  // Process management
  const [processModalOpen, setProcessModalOpen] = useState(false);
  const [processes, setProcesses] = useState<types.ProcessInfo[]>([]);
  const [scanningProcesses, setScanningProcesses] = useState(false);
  const [processError, setProcessError] = useState("");
  const [terminatingProcess, setTerminatingProcess] = useState<number | "all" | null>(null);
  const terminatingRef = useRef(false);

  // Sun times loading
  const [loadingSunTimes, setLoadingSunTimes] = useState(false);

  // Unsaved changes / navigation
  const {
    isOpen: unsavedOpen,
    open: unsavedOnOpen,
    setOpen: unsavedOnOpenChange,
    close: unsavedOnClose,
  } = useOverlayState();
  const [pendingNavPath, setPendingNavPath] = useState<string | number | null>(
    null,
  );
  const {
    isOpen: resetOpen,
    open: resetOnOpen,
    setOpen: resetOnOpenChange,
    close: resetOnClose,
  } = useOverlayState();

  // --- Handlers ---
  const setAnalyticsEnabled = (enabled: boolean) => {
    setAnalyticsEnabledState(enabled);
    persistAnalyticsChoice(enabled);
  };

  const setExperimentalInstanceBackupEnabled = (enabled: boolean) => {
    setExperimentalInstanceBackupEnabledState(enabled);
    persistExperimentalInstanceBackupEnabled(enabled);
  };

  // The Go setters report failure as a non-empty error code rather than by
  // rejecting, so the optimistic state has to be rolled back explicitly.
  const setGameLaunchBehavior = async (behavior: string) => {
    const previous = gameLaunchBehavior;
    setGameLaunchBehaviorState(behavior);
    if (await SetGameLaunchBehavior(behavior)) {
      setGameLaunchBehaviorState(previous);
    }
  };

  const setGameExitBehavior = async (behavior: string) => {
    const previous = gameExitBehavior;
    setGameExitBehaviorState(behavior);
    if (await SetGameExitBehavior(behavior)) {
      setGameExitBehaviorState(previous);
    }
  };

  const setMinimizeToTray = async (enable: boolean) => {
    setMinimizeToTrayState(enable);
    if (await SetMinimizeToTray(enable)) {
      setMinimizeToTrayState(!enable);
    }
  };

  const refreshSunTimes = async () => {
    setLoadingSunTimes(true);
    await fetchSunTimes();
    setLoadingSunTimes(false);
  };

  const refreshProcesses = async () => {
    setScanningProcesses(true);
    setProcessError("");
    try {
      const list = await ListMinecraftProcesses();
      setProcesses(list || []);
      return true;
    } catch (error) {
      setProcessError(error instanceof Error ? error.message : String(error));
      return false;
    } finally {
      setScanningProcesses(false);
    }
  };

  const terminateProcesses = async (pid?: number) => {
    if (terminatingRef.current) return false;
    terminatingRef.current = true;
    setTerminatingProcess(pid ?? "all");
    setProcessError("");
    try {
      const error = pid === undefined
        ? await KillAllMinecraftProcesses()
        : await KillProcess(pid);
      // A bulk termination can partially succeed. Always refresh its survivors.
      const refreshed = await refreshProcesses();
      if (error) {
        setProcessError(error);
        return false;
      }
      return refreshed;
    } catch (error) {
      setProcessError(error instanceof Error ? error.message : String(error));
      return false;
    } finally {
      terminatingRef.current = false;
      setTerminatingProcess(null);
    }
  };

  const handleKillProcess = (pid: number) => terminateProcesses(pid);
  const handleKillAllProcesses = () => terminateProcesses();

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
      navigate(ROUTES.updating, { replace: true });
    } finally {
      setUpdating(false);
    }
  };

  const cleanLipCache = async (): Promise<string> => {
    setCleaningLipCache(true);
    try {
      // CacheClean is a lip task with an empty target; its cache_clean bar
      // counts the cache entries deleted.
      return await runWithLipTask(
        {
          action: "generic",
          target: "",
          methods: ["CacheClean"],
          feedbackMode: "inline",
          errorConsole: false,
        },
        async () => String((await CacheClean()) || ""),
      );
    } catch (e: any) {
      return String(e?.data || e?.message || e || "ERR_LIP_CACHE_CLEAN_FAILED");
    } finally {
      setCleaningLipCache(false);
    }
  };

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
        const targetPath: unknown = ev?.detail?.path;
        if (typeof targetPath !== "string" && typeof targetPath !== "number") {
          return;
        }
        if (
          typeof targetPath === "number" &&
          (!Number.isInteger(targetPath) || targetPath === 0)
        ) {
          return;
        }
        const hasUnsaved = newBaseRoot !== baseRoot;
        if (!targetPath || targetPath === location.pathname) return;
        if (hasUnsaved) {
          setPendingNavPath(targetPath);
          unsavedOnOpen();
          return;
        }
        if (typeof targetPath === "number") {
          navigate(targetPath);
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
          const [launchBehavior, exitBehavior, trayEnabled] =
            await Promise.all([
              GetGameLaunchBehavior(),
              GetGameExitBehavior(),
              GetMinimizeToTray(),
            ]);
          setGameLaunchBehaviorState(launchBehavior);
          setGameExitBehaviorState(exitBehavior);
          setMinimizeToTrayState(trayEnabled);
        } catch {}
      })
      .catch(() => {});
  }, []);

  useEffect(() => {
    setBaseRootWritable(true);
  }, [newBaseRoot]);

  useEffect(() => {
    setBackgroundImageError(false);
  }, [backgroundImage, previewBgData]);

  useEffect(() => {
    const handleExperimentalFeaturesChange = () => {
      setExperimentalInstanceBackupEnabledState(
        readExperimentalInstanceBackupEnabled(),
      );
    };
    window.addEventListener(
      EXPERIMENTAL_FEATURES_EVENT_NAME,
      handleExperimentalFeaturesChange,
    );
    return () =>
      window.removeEventListener(
        EXPERIMENTAL_FEATURES_EVENT_NAME,
        handleExperimentalFeaturesChange,
      );
  }, []);

  // The preview shares the decoded image with App; only enumerate folder size here.
  useEffect(() => {
    const folderPath = backgroundImage;
    let cancelled = false;
    if (!folderPath) {
      setBackgroundImageCount(0);
      return;
    }

    const loadPreview = async () => {
      try {
        const entries = await (minecraft as any).ListDir(folderPath);
        if (cancelled) return;
        if (!entries || entries.length === 0) {
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
      } catch {
        if (cancelled) return;
        setBackgroundImageCount(0);
        setBackgroundImageError(true);
      }
    };

    void loadPreview();
    return () => { cancelled = true; };
  }, [backgroundImage]);

  // Tab from location state
  useEffect(() => {
    if (location.state && (location.state as any).tab) {
      setSelectedTab((location.state as any).tab);
    }
  }, [location.state]);

  return {
    // Computed

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
    analyticsEnabled,
    setAnalyticsEnabled,
    experimentalInstanceBackupEnabled,
    setExperimentalInstanceBackupEnabled,

    // Game lifecycle behaviors
    gameLaunchBehavior,
    setGameLaunchBehavior,
    gameExitBehavior,
    setGameExitBehavior,
    minimizeToTray,
    setMinimizeToTray,

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
    cleaningLipCache,
    cleanLipCache,

    // Resource rules

    // Process management
    processModalOpen,
    setProcessModalOpen,
    processes,
    scanningProcesses,
    processError,
    terminatingProcess,
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
