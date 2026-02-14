import "./polyfills/wails";
import { Route, Routes, useLocation, useNavigate } from "react-router-dom";
import React, { useEffect, useState, Suspense, lazy } from "react";
import { Button, ToastProvider, Spinner } from "@heroui/react";
import { UnifiedModal } from "@/components/UnifiedModal";
import { GlobalNavbar } from "@/components/GlobalNavbar";
import { Sidebar } from "@/components/Sidebar";
import { TopBar } from "@/components/TopBar";
import { motion, AnimatePresence, MotionGlobalConfig } from "framer-motion";
import { Events, Browser, Window } from "@wailsio/runtime";
import { useTranslation } from "react-i18next";
import { VersionStatusProvider } from "@/utils/VersionStatusContext";
import { CurseForgeProvider } from "@/utils/CurseForgeContext";
import { LeviLaminaProvider } from "@/utils/LeviLaminaContext";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { useTheme } from "next-themes";
import { KeybindingProvider, useKeybinding } from "@/utils/KeybindingContext";
import { FaRocket } from "react-icons/fa";
import { NavigationHistoryProvider } from "@/utils/NavigationHistoryContext";
import { THEMES, hexToRgb, generateTheme } from "@/constants/themes";
import { useThemeManager } from "@/utils/useThemeManager";
import { SplashScreen } from "@/pages/SplashScreen";
import { DownloadsProvider } from "@/utils/DownloadsContext";

// Lazy load pages
const LauncherPage = lazy(() =>
  import("@/pages/LauncherPage").then((m) => ({ default: m.LauncherPage })),
);
const DownloadManagerPage = lazy(() =>
  import("@/pages/DownloadManagerPage").then((m) => ({
    default: m.DownloadManagerPage,
  })),
);
const DownloadPage = lazy(() =>
  import("@/pages/DownloadPage").then((m) => ({ default: m.DownloadPage })),
);
const SettingsPage = lazy(() =>
  import("@/pages/SettingsPage").then((m) => ({ default: m.SettingsPage })),
);
const VersionSelectPage = lazy(() =>
  import("@/pages/VersionSelectPage").then((m) => ({
    default: m.VersionSelectPage,
  })),
);
const VersionSettingsPage = lazy(() => import("@/pages/VersionSettingsPage"));
const ModsPage = lazy(() => import("@/pages/ModsPage"));
const UpdatingPage = lazy(() => import("@/pages/UpdatingPage"));
const ContentPage = lazy(() => import("@/pages/ContentPage"));
const WorldsListPage = lazy(() => import("@/pages/WorldsListPage"));
const WorldLevelDatEditorPage = lazy(
  () => import("@/pages/WorldLevelDatEditorPage"),
);
const ResourcePacksPage = lazy(() => import("@/pages/ResourcePacksPage"));
const BehaviorPacksPage = lazy(() => import("@/pages/BehaviorPacksPage"));
const SkinPacksPage = lazy(() => import("@/pages/SkinPacksPage"));
const InstallPage = lazy(() => import("@/pages/InstallPage"));
const AboutPage = lazy(() => import("@/pages/AboutPage"));
const OnboardingPage = lazy(() => import("@/pages/OnboardingPage"));
const CurseForgePage = lazy(() => import("@/pages/CurseForgePage"));
const CurseForgeModPage = lazy(() => import("@/pages/CurseForgeModPage"));
const LIPPage = lazy(() => import("@/pages/LIPPage"));
const LIPPackagePage = lazy(() => import("@/pages/LIPPackagePage"));
const ServersPage = lazy(() => import("@/pages/ServersPage"));

const getFitStyles = (mode: string) => {
  switch (mode) {
    case "center":
      return {
        backgroundSize: "auto",
        backgroundPosition: "center",
        backgroundRepeat: "no-repeat",
      };
    case "fit":
      return {
        backgroundSize: "contain",
        backgroundPosition: "center",
        backgroundRepeat: "no-repeat",
      };
    case "stretch":
      return {
        backgroundSize: "100% 100%",
        backgroundPosition: "center",
        backgroundRepeat: "no-repeat",
      };
    case "tile":
      return {
        backgroundSize: "auto",
        backgroundPosition: "top left",
        backgroundRepeat: "repeat",
      };
    case "top_left":
      return {
        backgroundSize: "auto",
        backgroundPosition: "top left",
        backgroundRepeat: "no-repeat",
      };
    case "top_right":
      return {
        backgroundSize: "auto",
        backgroundPosition: "top right",
        backgroundRepeat: "no-repeat",
      };
    case "smart":
    default:
      return {
        backgroundSize: "cover",
        backgroundPosition: "center",
        backgroundRepeat: "no-repeat",
      };
  }
};

const GlobalShortcuts = ({
  tryNavigate,
}: {
  tryNavigate: (path: string | number) => void;
}) => {
  const { register, unregister } = useKeybinding();
  const { t } = useTranslation();

  useEffect(() => {
    register(
      "go-back",
      "Escape",
      (e) => {
        e.preventDefault();
        tryNavigate(-1);
      },
      t("nav.back"),
    );

    return () => {
      unregister("go-back");
    };
  }, [register, unregister, tryNavigate, t]);

  return null;
};

function App() {
  const { theme, resolvedTheme, setTheme } = useTheme();
  const [splashVisible, setSplashVisible] = useState(true);
  const [revealStarted, setRevealStarted] = useState(false);
  const [isFirstLoad, setIsFirstLoad] = useState(false);
  const [count, setCount] = useState(0);
  const { t, i18n } = useTranslation();
  const hasBackend = minecraft !== undefined;
  const [isBeta, setIsBeta] = useState(false);
  const [navLocked, setNavLocked] = useState<boolean>(() => {
    try {
      const h =
        typeof window !== "undefined" ? String(window.location.hash || "") : "";
      const initLock =
        h.startsWith("#/updating") ||
        h.startsWith("#/onboarding") ||
        Boolean((window as any).llNavLock);
      return initLock;
    } catch {
      return false;
    }
  });
  const [termsOpen, setTermsOpen] = useState<boolean>(false);
  const [termsCountdown, setTermsCountdown] = useState<number>(0);
  const [updateOpen, setUpdateOpen] = useState<boolean>(false);
  const [updateVersion, setUpdateVersion] = useState<string>("");
  const [updateBody, setUpdateBody] = useState<string>("");
  const [updateLoading, setUpdateLoading] = useState<boolean>(false);
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
  const [disableAnimations, setDisableAnimations] = useState<boolean>(() => {
    try {
      return localStorage.getItem("app.disableAnimations") === "true";
    } catch {
      return false;
    }
  });

  const { themeMode } = useThemeManager();

  const [backgroundImagePath, setBackgroundImagePath] = useState<string>(
    () => localStorage.getItem("app.backgroundImage") || "",
  );
  const [backgroundFitMode, setBackgroundFitMode] = useState<string>(
    () => localStorage.getItem("app.backgroundFitMode") || "smart",
  );
  const [bgData, setBgData] = useState<string>("");

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

  const pickNextImage = async (folderPath: string) => {
    if (!folderPath) return "";
    try {
      const entries = await (minecraft as any).ListDir(folderPath);
      if (!entries || entries.length === 0) return "";
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
      if (images.length === 0) return "";

      const playOrder =
        localStorage.getItem("app.backgroundPlayOrder") || "random";

      if (playOrder === "sequential") {
        const lastIdxKey = "app.backgroundLastIndex";
        let lastIdx = parseInt(localStorage.getItem(lastIdxKey) || "-1");
        let nextIdx = lastIdx + 1;
        if (nextIdx >= images.length) {
          nextIdx = 0;
        }
        localStorage.setItem(lastIdxKey, String(nextIdx));
        return images[nextIdx].path;
      } else {
        const randomIdx = Math.floor(Math.random() * images.length);
        return images[randomIdx].path;
      }
    } catch (err) {
      console.error("Failed to pick next image:", err);
      return "";
    }
  };

  useEffect(() => {
    const initBackgrounds = async () => {
      try {
        const folder = localStorage.getItem("app.backgroundImage") || "";
        const img = await pickNextImage(folder);

        setBackgroundImagePath(img);
        localStorage.setItem("app.currentBackgroundImage", img);
      } catch {}
    };
    initBackgrounds();
  }, []);

  const [lightThemeColor, setLightThemeColor] = useState<string>(() => {
    try {
      const saved = localStorage.getItem("app.lightThemeColor");
      if (saved) return saved;
      const old = localStorage.getItem("app.themeColor");
      if (old === "rose") return "pink";
      return old || "emerald";
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

  const [darkThemeColor, setDarkThemeColor] = useState<string>(() => {
    try {
      const saved = localStorage.getItem("app.darkThemeColor");
      if (saved) return saved;
      const old = localStorage.getItem("app.themeColor");
      if (old === "rose") return "pink";
      return old || "emerald";
    } catch {
      return "emerald";
    }
  });

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

  useEffect(() => {
    const handler = () => {
      try {
        const val = Number(localStorage.getItem("app.backgroundBlur") || "0");
        setBackgroundBlur(val);
      } catch {}
    };
    window.addEventListener("app-blur-changed", handler);
    return () => window.removeEventListener("app-blur-changed", handler);
  }, []);

  useEffect(() => {
    const handler = () => {
      try {
        const item = localStorage.getItem("app.backgroundOpacity");
        setBackgroundOpacity(item !== null ? Number(item) : 100);
      } catch {}
    };
    window.addEventListener("app-opacity-changed", handler);
    return () => window.removeEventListener("app-opacity-changed", handler);
  }, []);

  useEffect(() => {
    const handler = () => {
      try {
        const item = localStorage.getItem("app.backgroundBrightness");
        setBackgroundBrightness(item !== null ? Number(item) : 100);
      } catch {}
    };
    window.addEventListener("app-brightness-changed", handler);
    return () => window.removeEventListener("app-brightness-changed", handler);
  }, []);

  useEffect(() => {
    const handler = async () => {
      try {
        const folder = localStorage.getItem("app.backgroundImage") || "";
        const img = await pickNextImage(folder);

        setBackgroundImagePath(img);
        localStorage.setItem("app.currentBackgroundImage", img);
      } catch {}
    };
    window.addEventListener("app-background-changed", handler);
    return () => window.removeEventListener("app-background-changed", handler);
  }, []);

  useEffect(() => {
    const handler = () => {
      try {
        setBackgroundFitMode(
          localStorage.getItem("app.backgroundFitMode") || "smart",
        );

        setLightBackgroundBaseMode(
          localStorage.getItem("app.lightBackgroundBaseMode") || "none",
        );
        setDarkBackgroundBaseMode(
          localStorage.getItem("app.darkBackgroundBaseMode") || "none",
        );

        setLightBackgroundBaseColor(
          localStorage.getItem("app.lightBackgroundBaseColor") || "#ffffff",
        );
        setDarkBackgroundBaseColor(
          localStorage.getItem("app.darkBackgroundBaseColor") || "#18181b",
        );

        const lightOpacity = localStorage.getItem(
          "app.lightBackgroundBaseOpacity",
        );
        setLightBackgroundBaseOpacity(
          lightOpacity !== null ? Number(lightOpacity) : 50,
        );

        const darkOpacity = localStorage.getItem(
          "app.darkBackgroundBaseOpacity",
        );
        setDarkBackgroundBaseOpacity(
          darkOpacity !== null ? Number(darkOpacity) : 50,
        );
      } catch {}
    };
    window.addEventListener("app-background-settings-changed", handler);
    return () =>
      window.removeEventListener("app-background-settings-changed", handler);
  }, []);

  useEffect(() => {
    const handler = (e: any) => {
      try {
        const lightColor =
          localStorage.getItem("app.lightThemeColor") ||
          localStorage.getItem("app.themeColor") ||
          "emerald";
        setLightThemeColor(lightColor);
        const lightCustom =
          localStorage.getItem("app.lightCustomThemeColor") ||
          localStorage.getItem("app.customThemeColor") ||
          "#10b981";
        setLightCustomThemeColor(lightCustom);

        const darkColor =
          localStorage.getItem("app.darkThemeColor") ||
          localStorage.getItem("app.themeColor") ||
          "emerald";
        setDarkThemeColor(darkColor);
        const darkCustom =
          localStorage.getItem("app.darkCustomThemeColor") ||
          localStorage.getItem("app.customThemeColor") ||
          "#10b981";
        setDarkCustomThemeColor(darkCustom);
      } catch {}
    };
    window.addEventListener("app-theme-changed", handler);
    return () => window.removeEventListener("app-theme-changed", handler);
  }, []);

  useEffect(() => {
    const isDark = resolvedTheme === "dark";
    const currentColor = isDark ? darkThemeColor : lightThemeColor;
    const currentCustomColor = isDark
      ? darkCustomThemeColor
      : lightCustomThemeColor;

    let theme = THEMES[currentColor];
    if (currentColor === "custom") {
      theme = generateTheme(currentCustomColor);
    }
    if (!theme) theme = THEMES.emerald;

    const root = document.documentElement;

    Object.keys(theme).forEach((key) => {
      const k = Number(key);
      root.style.setProperty(`--theme-${k}`, hexToRgb(theme[k]));
    });
  }, [
    resolvedTheme,
    lightThemeColor,
    lightCustomThemeColor,
    darkThemeColor,
    darkCustomThemeColor,
  ]);

  useEffect(() => {
    const currentImg = backgroundImagePath;

    if (!currentImg) {
      setBgData("");
      return;
    }
    if (currentImg.startsWith("data:") || currentImg.startsWith("http")) {
      setBgData(currentImg);
      return;
    }

    (minecraft as any)
      .GetImageBase64?.(currentImg)
      .then((res: string) => {
        if (res) setBgData(res);
      })
      .catch(() => {});
  }, [backgroundImagePath]);

  useEffect(() => {
    MotionGlobalConfig.skipAnimations = disableAnimations;

    if (disableAnimations) {
      const style = document.createElement("style");
      style.id = "disable-animations-style";
      style.innerHTML = `
        *, *::before, *::after {
          animation-duration: 0s !important;
          animation-delay: 0s !important;
          transition-duration: 0s !important;
          transition-delay: 0s !important;
        }
      `;
      document.head.appendChild(style);
    } else {
      const style = document.getElementById("disable-animations-style");
      if (style) {
        style.remove();
      }
    }

    return () => {
      const style = document.getElementById("disable-animations-style");
      if (style) {
        style.remove();
      }
    };
  }, [disableAnimations]);

  useEffect(() => {
    const handleAnimationsChange = () => {
      try {
        const val = localStorage.getItem("app.disableAnimations") === "true";
        setDisableAnimations(val);
      } catch {}
    };
    window.addEventListener("app-animations-changed", handleAnimationsChange);
    return () => {
      window.removeEventListener(
        "app-animations-changed",
        handleAnimationsChange,
      );
    };
  }, []);

  useEffect(() => {
    const handler = () => {
      try {
        const mode =
          (localStorage.getItem("app.layoutMode") as "navbar" | "sidebar") ||
          "sidebar";
        setLayoutMode(mode);
      } catch {}
    };
    window.addEventListener("app-layout-changed", handler);
    return () => window.removeEventListener("app-layout-changed", handler);
  }, []);

  const refresh = () => {
    setCount((prevCount) => {
      return prevCount + 1;
    });
  };

  const location = useLocation();
  const navigate = useNavigate();
  const isUpdatingMode = (() => {
    const p = String(location?.pathname || "");
    if (p === "/updating") return true;
    const h =
      typeof window !== "undefined" ? String(window.location.hash || "") : "";
    return h.startsWith("#/updating");
  })();
  const isOnboardingMode = (() => {
    const p = String(location?.pathname || "");
    if (p === "/onboarding") return true;
    const h =
      typeof window !== "undefined" ? String(window.location.hash || "") : "";
    return h.startsWith("#/onboarding");
  })();

  const [hasEnteredLauncher, setHasEnteredLauncher] = useState(false);

  useEffect(() => {
    if (location.pathname === "/" && revealStarted) {
      try {
        const onboarded = localStorage.getItem("ll.onboarded");
        if (onboarded) {
          setHasEnteredLauncher(true);
        }
      } catch {}
    }
  }, [location.pathname, revealStarted]);

  useEffect(() => {
    if (isUpdatingMode || isOnboardingMode) setNavLocked(true);
    else setNavLocked(Boolean((window as any).llNavLock));
  }, [isUpdatingMode, isOnboardingMode]);

  useEffect(() => {
    if (isUpdatingMode) {
      setSplashVisible(false);
      setRevealStarted(true);
      return;
    }
    const splashDurationMs = 1400;
    const overlayFadeMs = 600;

    setIsFirstLoad(false);

    const tHide = setTimeout(() => setSplashVisible(false), splashDurationMs);
    const tHeader = setTimeout(
      () => setRevealStarted(true),
      splashDurationMs - 200,
    );
    return () => {
      clearTimeout(tHide);
      clearTimeout(tHeader);
    };
  }, [isUpdatingMode]);

  useEffect(() => {
    if (!revealStarted) return;
    if (isUpdatingMode) return;
    try {
      const onboarded = localStorage.getItem("ll.onboarded");
      if (!onboarded && location.pathname !== "/onboarding") {
        setNavLocked(true);
        navigate("/onboarding", { replace: true });
      }
    } catch {}
  }, [revealStarted, isUpdatingMode, location?.pathname]);

  useEffect(() => {
    try {
      setNavLocked(Boolean((window as any).llNavLock));
    } catch {}
    const handler = (e: any) => {
      try {
        if (isUpdatingMode) return;
        setNavLocked(Boolean(e?.detail?.lock ?? (window as any).llNavLock));
      } catch {}
    };
    window.addEventListener("ll-nav-lock-changed", handler as any);
    return () =>
      window.removeEventListener("ll-nav-lock-changed", handler as any);
  }, [isUpdatingMode]);

  useEffect(() => {
    if (!hasBackend) return;
    if (!revealStarted) return;
    if (isUpdatingMode) return;
    try {
      const accepted = localStorage.getItem("ll.termsAccepted");
      if (!accepted) {
        setTermsOpen(true);
        setNavLocked(true);
        return;
      }
      const ignored = localStorage.getItem("ll.ignoreVersion") || "";
      minecraft
        ?.CheckUpdate?.()
        .then((res: any) => {
          const ver = String(res?.version || "");
          const body = String(res?.body || "");
          const is = Boolean(res?.isUpdate);
          if (is && ver && ver !== ignored) {
            setUpdateVersion(ver);
            setUpdateBody(body);
            setUpdateOpen(true);
            setNavLocked(true);
          }
        })
        .catch(() => {});
    } catch {}
  }, [hasBackend, revealStarted, isUpdatingMode]);

  useEffect(() => {
    if (!termsOpen) return;
    setTermsCountdown(10);
    const iv = setInterval(() => {
      setTermsCountdown((v) => (v > 0 ? v - 1 : 0));
    }, 1000);
    return () => clearInterval(iv);
  }, [termsOpen]);

  useEffect(() => {
    try {
      if (updateOpen) {
        document.body.style.overflow = "hidden";
        document.documentElement.style.overflow = "hidden";
        const root = document.getElementById("root");
        if (root) (root as HTMLElement).style.overflow = "hidden";
      } else {
        document.body.style.overflow = "";
        document.documentElement.style.overflow = "";
        const root = document.getElementById("root");
        if (root) (root as HTMLElement).style.overflow = "";
      }
    } catch {}
    return () => {
      try {
        document.body.style.overflow = "";
        document.documentElement.style.overflow = "";
        const root = document.getElementById("root");
        if (root) (root as HTMLElement).style.overflow = "";
      } catch {}
    };
  }, [updateOpen]);

  const acceptTerms = () => {
    try {
      localStorage.setItem("ll.termsAccepted", "1");
    } catch {}
    setTermsOpen(false);
    setNavLocked(Boolean((window as any).llNavLock));
  };

  useEffect(() => {
    if (!hasBackend) return;
    try {
      minecraft
        ?.GetIsBeta?.()
        .then((v: boolean) => setIsBeta(!!v))
        .catch(() => {});
    } catch {}
  }, [hasBackend]);

  const tryNavigate = (path: string | number) => {
    if (navLocked || isUpdatingMode || isOnboardingMode) return;
    if (
      location.pathname === "/settings" ||
      location.pathname === "/versionSettings"
    ) {
      try {
        window.dispatchEvent(
          new CustomEvent("ll-try-nav", { detail: { path } }),
        );
        return;
      } catch {}
    }
    navigate(path as any);
  };

  useEffect(() => {
    if (!hasBackend) return;

    const off1 = Events.On("msixvc_download_progress", () => {});
    const off2 = Events.On("msixvc_download_status", () => {});
    const off3 = Events.On("msixvc_download_error", () => {});
    const off4 = Events.On("msixvc_download_done", () => {});
    return () => {
      try {
        off1 && off1();
      } catch {}
      try {
        off2 && off2();
      } catch {}
      try {
        off3 && off3();
      } catch {}
      try {
        off4 && off4();
      } catch {}
    };
  }, [hasBackend]);

  useEffect(() => {
    const isFileDrag = (e: DragEvent) => {
      try {
        const types = e?.dataTransfer?.types;
        if (!types) return false;
        return Array.from(types).includes("Files");
      } catch {
        return false;
      }
    };
    const onDocDragOverCapture = (e: DragEvent) => {
      if (isFileDrag(e)) {
        e.preventDefault();
        try {
          (e.dataTransfer as any).dropEffect = "copy";
        } catch {}
      }
    };
    const onDocDropCapture = (e: DragEvent) => {
      if (isFileDrag(e)) {
        e.preventDefault();
      }
    };
    document.addEventListener("dragover", onDocDragOverCapture, true);
    document.addEventListener("drop", onDocDropCapture, true);
    return () => {
      document.removeEventListener("dragover", onDocDragOverCapture, true);
      document.removeEventListener("drop", onDocDropCapture, true);
    };
  }, []);

  useEffect(() => {
    const onDragStartCapture = (e: DragEvent) => {
      try {
        const target = e.target;
        if (target instanceof HTMLImageElement) {
          e.preventDefault();
        }
      } catch {}
    };
    document.addEventListener("dragstart", onDragStartCapture, true);
    return () => {
      document.removeEventListener("dragstart", onDragStartCapture, true);
    };
  }, []);

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Tab") {
        e.preventDefault();
      }
    };
    document.addEventListener("keydown", onKeyDown, true);
    return () => {
      document.removeEventListener("keydown", onKeyDown, true);
    };
  }, []);

  return (
    <KeybindingProvider>
      <GlobalShortcuts tryNavigate={tryNavigate} />
      <VersionStatusProvider>
        <DownloadsProvider>
          <LeviLaminaProvider>
            <CurseForgeProvider>
              <NavigationHistoryProvider>
                <ToastProvider
                  placement="top-center"
                  toastOffset={80}
                  toastProps={{ timeout: 2000 }}
                />
                <AnimatePresence>
                  {splashVisible && (
                    <motion.div
                      key="splash-overlay"
                      className="fixed inset-0 z-[9999]"
                      initial={{ opacity: 1 }}
                      animate={{ opacity: 1 }}
                      exit={{ opacity: 0 }}
                      transition={{ duration: 0.6, ease: "easeOut" }}
                    >
                      <SplashScreen />
                    </motion.div>
                  )}
                </AnimatePresence>

                {(resolvedTheme === "light"
                  ? lightBackgroundBaseMode
                  : darkBackgroundBaseMode) !== "none" && (
                  <div
                    className="fixed inset-0 z-[-2] pointer-events-none"
                    style={{
                      backgroundColor:
                        (resolvedTheme === "light"
                          ? lightBackgroundBaseMode
                          : darkBackgroundBaseMode) === "theme"
                          ? resolvedTheme === "light"
                            ? "rgb(var(--theme-50))"
                            : "rgb(var(--theme-900))"
                          : resolvedTheme === "light"
                            ? lightBackgroundBaseColor
                            : darkBackgroundBaseColor,
                      opacity:
                        (resolvedTheme === "light"
                          ? lightBackgroundBaseOpacity
                          : darkBackgroundBaseOpacity) / 100,
                    }}
                  />
                )}
                {bgData && (
                  <div
                    className="fixed inset-0 z-[-1]"
                    style={{
                      backgroundImage: `url("${bgData}")`,
                      ...getFitStyles(backgroundFitMode),
                      filter: `blur(${backgroundBlur}px) brightness(${backgroundBrightness}%)`,
                      opacity: backgroundOpacity / 100,
                      transform: backgroundBlur > 0 ? "scale(1.1)" : "none",
                    }}
                  />
                )}

                <div
                  style={
                    {
                      "--content-pt":
                        layoutMode === "sidebar" ? "4.5rem" : "5rem",
                    } as React.CSSProperties
                  }
                  className={`w-full min-h-dvh flex ${
                    layoutMode === "sidebar" ? "flex-row" : "flex-col"
                  } overflow-x-hidden ${
                    bgData ||
                    (resolvedTheme === "light"
                      ? lightBackgroundBaseMode
                      : darkBackgroundBaseMode) !== "none"
                      ? "bg-transparent"
                      : "bg-background"
                  } text-foreground ${updateOpen ? "overflow-y-hidden" : ""}`}
                >
                  {layoutMode === "navbar" ? (
                    <>
                      <GlobalNavbar
                        isBeta={isBeta}
                        navLocked={
                          navLocked || isUpdatingMode || isOnboardingMode
                        }
                        themeMode={themeMode}
                        revealStarted={revealStarted}
                        isUpdatingMode={isUpdatingMode}
                        isOnboardingMode={isOnboardingMode}
                        hasEnteredLauncher={hasEnteredLauncher}
                        tryNavigate={tryNavigate}
                      />
                    </>
                  ) : (
                    <>
                      <Sidebar
                        isBeta={isBeta}
                        navLocked={
                          navLocked || isUpdatingMode || isOnboardingMode
                        }
                        themeMode={themeMode}
                        revealStarted={revealStarted}
                        isUpdatingMode={isUpdatingMode}
                        isOnboardingMode={isOnboardingMode}
                        hasEnteredLauncher={hasEnteredLauncher}
                        tryNavigate={tryNavigate}
                      />
                      <TopBar
                        navLocked={
                          navLocked || isUpdatingMode || isOnboardingMode
                        }
                        revealStarted={revealStarted}
                        isOnboardingMode={isOnboardingMode}
                        tryNavigate={tryNavigate}
                      />
                    </>
                  )}

                  <motion.div
                    className={`w-full flex-1 min-h-0 overflow-hidden ${
                      layoutMode === "sidebar" ? "pl-14" : ""
                    }`}
                    initial={{ opacity: 0 }}
                    animate={{ opacity: revealStarted ? 1 : 0 }}
                    transition={{ duration: 0.6, ease: "easeOut" }}
                    style={{ pointerEvents: revealStarted ? "auto" : "none" }}
                  >
                    {revealStarted &&
                      (isFirstLoad ? (
                        <></>
                      ) : (
                        <Suspense
                          fallback={
                            <div className="w-full h-full flex items-center justify-center">
                              <Spinner size="lg" />
                            </div>
                          }
                        >
                          <Routes>
                            <Route
                              path="/"
                              element={
                                <LauncherPage refresh={refresh} count={count} />
                              }
                            />
                            <Route
                              path="/download"
                              element={<DownloadPage />}
                            />
                            <Route
                              path="/tasks"
                              element={<DownloadManagerPage />}
                            />
                            <Route path="/install" element={<InstallPage />} />
                            <Route
                              path="/settings"
                              element={<SettingsPage />}
                            />
                            <Route
                              path="/versions"
                              element={<VersionSelectPage refresh={refresh} />}
                            />
                            <Route
                              path="/versionSettings"
                              element={<VersionSettingsPage />}
                            />
                            <Route path="/mods" element={<ModsPage />} />
                            <Route
                              path="/curseforge"
                              element={<CurseForgePage />}
                            />
                            <Route
                              path="/curseforge/mod/:id"
                              element={<CurseForgeModPage />}
                            />
                            <Route path="/lip" element={<LIPPage />} />
                            <Route
                              path="/lip/package/:id"
                              element={<LIPPackagePage />}
                            />
                            <Route
                              path="/updating"
                              element={<UpdatingPage />}
                            />
                            <Route
                              path="/onboarding"
                              element={<OnboardingPage />}
                            />
                            <Route path="/content" element={<ContentPage />} />
                            <Route
                              path="/content/worlds"
                              element={<WorldsListPage />}
                            />
                            <Route
                              path="/content/worlds/worldEdit"
                              element={<WorldLevelDatEditorPage />}
                            />
                            <Route
                              path="/content/resourcePacks"
                              element={<ResourcePacksPage />}
                            />
                            <Route
                              path="/content/behaviorPacks"
                              element={<BehaviorPacksPage />}
                            />
                            <Route
                              path="/content/skinPacks"
                              element={<SkinPacksPage />}
                            />
                            <Route
                              path="/content/servers"
                              element={<ServersPage />}
                            />
                            <Route path="/about" element={<AboutPage />} />
                          </Routes>
                        </Suspense>
                      ))}
                  </motion.div>

                  <UnifiedModal
                    size="lg"
                    isOpen={termsOpen}
                    onOpenChange={() => {}}
                    type="primary"
                    title={t("terms.title")}
                    hideCloseButton
                    isDismissable={false}
                    showConfirmButton={false}
                    showCancelButton={false}
                    footer={
                      <div className="flex w-full justify-end gap-2">
                        <Button
                          variant="light"
                          onPress={() => {
                            Window.Close();
                          }}
                        >
                          {t("terms.decline")}
                        </Button>
                        <Button
                          color="primary"
                          isDisabled={termsCountdown > 0}
                          onPress={acceptTerms}
                        >
                          {termsCountdown > 0
                            ? `${t("terms.agree")} (${termsCountdown}s)`
                            : t("terms.agree")}
                        </Button>
                      </div>
                    }
                  >
                    <div className="text-[15px] sm:text-[16px] leading-7 text-default-900 dark:text-zinc-100 font-medium antialiased whitespace-pre-wrap wrap-break-word max-h-[56vh] overflow-y-auto pr-2 custom-scrollbar">
                      {t("terms.body")}
                    </div>
                  </UnifiedModal>

                  <UnifiedModal
                    size="md"
                    isOpen={updateOpen}
                    onOpenChange={(open) => {
                      if (!open) {
                        setUpdateOpen(false);
                        setNavLocked(Boolean((window as any).llNavLock));
                      }
                    }}
                    type="primary"
                    title={
                      <span className="truncate">
                        {t("settings.body.version.hasnew")}
                        {updateVersion}
                      </span>
                    }
                    icon={<FaRocket className="w-5 h-5 text-primary-500" />}
                    hideCloseButton
                    showConfirmButton={false}
                    showCancelButton={false}
                    footer={
                      <div className="flex w-full justify-end gap-2">
                        <Button
                          variant="light"
                          onPress={() => {
                            setUpdateOpen(false);
                            setNavLocked(Boolean((window as any).llNavLock));
                          }}
                        >
                          {t("common.cancel")}
                        </Button>
                        <Button
                          variant="flat"
                          onPress={() => {
                            try {
                              localStorage.setItem(
                                "ll.ignoreVersion",
                                updateVersion || "",
                              );
                            } catch {}
                            setUpdateOpen(false);
                            setNavLocked(Boolean((window as any).llNavLock));
                          }}
                        >
                          {t("settings.body.version.ignore")}
                        </Button>
                        <Button
                          color="primary"
                          isLoading={updateLoading}
                          onPress={async () => {
                            setUpdateLoading(true);
                            try {
                              setUpdateOpen(false);
                              setNavLocked(true);
                              navigate("/updating", { replace: true });
                            } finally {
                              setUpdateLoading(false);
                            }
                          }}
                        >
                          {t("settings.modal.2.footer.download_button")}
                        </Button>
                      </div>
                    }
                  >
                    {updateBody ? (
                      <div className="rounded-xl bg-default-100/60 dark:bg-zinc-800/60 border border-default-200 dark:border-zinc-700 px-3 py-2">
                        <div className="text-small font-semibold mb-1 text-default-700 dark:text-zinc-200">
                          {t("downloadpage.changelog.title")}
                        </div>
                        <div className="text-small text-default-600 dark:text-zinc-300 wrap-break-word leading-6 max-h-[32vh] sm:max-h-[40vh] lg:max-h-[44vh] overflow-y-auto pr-1">
                          <ReactMarkdown
                            remarkPlugins={[remarkGfm]}
                            components={{
                              h1: ({ children }) => (
                                <h1 className="text-xl font-semibold mt-2 mb-2">
                                  {children}
                                </h1>
                              ),
                              h2: ({ children }) => (
                                <h2 className="text-lg font-semibold mt-2 mb-2">
                                  {children}
                                </h2>
                              ),
                              h3: ({ children }) => (
                                <h3 className="text-base font-semibold mt-2 mb-2">
                                  {children}
                                </h3>
                              ),
                              p: ({ children }) => (
                                <p className="my-1">{children}</p>
                              ),
                              ul: ({ children }) => (
                                <ul className="list-disc pl-6 my-2">
                                  {children}
                                </ul>
                              ),
                              ol: ({ children }) => (
                                <ol className="list-decimal pl-6 my-2">
                                  {children}
                                </ol>
                              ),
                              li: ({ children }) => (
                                <li className="my-1">{children}</li>
                              ),
                              a: ({ href, children }) => {
                                const cleanUrl = (url: string) => {
                                  const target = "https://github.com";
                                  const idx = url.lastIndexOf(target);
                                  return idx > 0 ? url.substring(idx) : url;
                                };

                                return (
                                  <a
                                    href={href}
                                    target="_blank"
                                    rel="noreferrer"
                                    className="text-primary underline cursor-pointer"
                                    onClick={(e) => {
                                      e.preventDefault();
                                      if (href) {
                                        Browser.OpenURL(cleanUrl(href));
                                      }
                                    }}
                                  >
                                    {Array.isArray(children)
                                      ? children.map((child) =>
                                          typeof child === "string"
                                            ? cleanUrl(child)
                                            : child,
                                        )
                                      : typeof children === "string"
                                        ? cleanUrl(children)
                                        : children}
                                  </a>
                                );
                              },
                              hr: () => (
                                <hr className="my-3 border-default-200" />
                              ),
                            }}
                          >
                            {updateBody}
                          </ReactMarkdown>
                        </div>
                      </div>
                    ) : null}
                  </UnifiedModal>
                </div>
              </NavigationHistoryProvider>
            </CurseForgeProvider>
          </LeviLaminaProvider>
        </DownloadsProvider>
      </VersionStatusProvider>
    </KeybindingProvider>
  );
}

export default App;
