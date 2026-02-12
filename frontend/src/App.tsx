import "./polyfills/wails";
import { Route, Routes, useLocation, useNavigate } from "react-router-dom";
import React, { useEffect, useState } from "react";
import { Button, ToastProvider } from "@heroui/react";
import { UnifiedModal } from "@/components/UnifiedModal";
import { GlobalNavbar } from "@/components/GlobalNavbar";
import { Sidebar } from "@/components/Sidebar";
import { TopBar } from "@/components/TopBar";
import { LauncherPage } from "@/pages/LauncherPage";
import { DownloadManagerPage } from "@/pages/DownloadManagerPage";
import { DownloadsProvider } from "@/utils/DownloadsContext";
import { DownloadPage } from "@/pages/DownloadPage";
import { SplashScreen } from "@/pages/SplashScreen";
import { motion, AnimatePresence, MotionGlobalConfig } from "framer-motion";
import { Events, Browser, Window } from "@wailsio/runtime";
import { SettingsPage } from "@/pages/SettingsPage";
import { VersionSelectPage } from "@/pages/VersionSelectPage";
import VersionSettingsPage from "@/pages/VersionSettingsPage";
import ModsPage from "@/pages/ModsPage";
import UpdatingPage from "@/pages/UpdatingPage";
import ContentPage from "@/pages/ContentPage";
import WorldsListPage from "@/pages/WorldsListPage";
import WorldLevelDatEditorPage from "@/pages/WorldLevelDatEditorPage";
import ResourcePacksPage from "@/pages/ResourcePacksPage";
import BehaviorPacksPage from "@/pages/BehaviorPacksPage";
import SkinPacksPage from "@/pages/SkinPacksPage";
import { useTranslation } from "react-i18next";
import { VersionStatusProvider } from "@/utils/VersionStatusContext";
import { CurseForgeProvider } from "@/utils/CurseForgeContext";
import { LeviLaminaProvider } from "@/utils/LeviLaminaContext";
import InstallPage from "@/pages/InstallPage";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import AboutPage from "@/pages/AboutPage";
import OnboardingPage from "@/pages/OnboardingPage";
import CurseForgePage from "@/pages/CurseForgePage";
import CurseForgeModPage from "@/pages/CurseForgeModPage";
import LIPPage from "@/pages/LIPPage";
import LIPPackagePage from "@/pages/LIPPackagePage";
import ServersPage from "@/pages/ServersPage";
import { useTheme } from "next-themes";
import { KeybindingProvider, useKeybinding } from "@/utils/KeybindingContext";
import { FaRocket } from "react-icons/fa";
import { NavigationHistoryProvider } from "@/utils/NavigationHistoryContext";
import { THEMES, hexToRgb } from "@/constants/themes";

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
  const { theme, resolvedTheme } = useTheme();
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

  const [backgroundImage, setBackgroundImage] = useState<string>(() => {
    try {
      return localStorage.getItem("app.backgroundImage") || "";
    } catch {
      return "";
    }
  });
  const [bgData, setBgData] = useState<string>("");
  const [backgroundBlur, setBackgroundBlur] = useState<number>(() => {
    try {
      return Number(localStorage.getItem("app.backgroundBlur") || "0");
    } catch {
      return 0;
    }
  });
  const [backgroundBrightness, setBackgroundBrightness] = useState<number>(
    () => {
      try {
        const item = localStorage.getItem("app.backgroundBrightness");
        return item !== null ? Number(item) : 100;
      } catch {
        return 100;
      }
    },
  );
  const [backgroundOpacity, setBackgroundOpacity] = useState<number>(() => {
    try {
      const item = localStorage.getItem("app.backgroundOpacity");
      return item !== null ? Number(item) : 100;
    } catch {
      return 100;
    }
  });

  const [themeColor, setThemeColor] = useState<string>(() => {
    try {
      return localStorage.getItem("app.themeColor") || "emerald";
    } catch {
      return "emerald";
    }
  });

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
    const handler = () => {
      try {
        const img = localStorage.getItem("app.backgroundImage") || "";
        setBackgroundImage(img);
      } catch {}
    };
    window.addEventListener("app-background-changed", handler);
    return () => window.removeEventListener("app-background-changed", handler);
  }, []);

  useEffect(() => {
    const handler = () => {
      try {
        const val = localStorage.getItem("app.themeColor") || "emerald";
        setThemeColor(val);
      } catch {}
    };
    window.addEventListener("app-theme-changed", handler);
    return () => window.removeEventListener("app-theme-changed", handler);
  }, []);

  useEffect(() => {
    const theme = THEMES[themeColor] || THEMES.emerald;
    const root = document.documentElement;

    Object.keys(theme).forEach((key) => {
      const k = Number(key);
      root.style.setProperty(`--theme-${k}`, hexToRgb(theme[k]));
    });
  }, [themeColor]);

  useEffect(() => {
    if (!backgroundImage) {
      setBgData("");
      return;
    }
    if (
      backgroundImage.startsWith("data:") ||
      backgroundImage.startsWith("http")
    ) {
      setBgData(backgroundImage);
      return;
    }
    // Fetch from backend
    (minecraft as any)
      .GetImageBase64?.(backgroundImage)
      .then((res: string) => {
        if (res) setBgData(res);
      })
      .catch(() => {});
  }, [backgroundImage]);

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
    if (navLocked) return;
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

                <div
                  style={
                    {
                      "--content-pt":
                        layoutMode === "sidebar" ? "4.5rem" : "5rem",
                      ...(bgData ? { backgroundColor: "transparent" } : {}),
                    } as React.CSSProperties
                  }
                  className={`w-full min-h-dvh flex ${
                    layoutMode === "sidebar" ? "flex-row" : "flex-col"
                  } overflow-x-hidden bg-background text-foreground ${
                    updateOpen ? "overflow-y-hidden" : ""
                  }`}
                >
                  {bgData && (
                    <div
                      className="fixed inset-0 z-0"
                      style={{
                        backgroundImage: `url("${bgData}")`,
                        backgroundSize: "cover",
                        backgroundPosition: "center",
                        backgroundRepeat: "no-repeat",
                        filter: `blur(${backgroundBlur}px) brightness(${backgroundBrightness}%)`,
                        opacity: backgroundOpacity / 100,
                        transform: backgroundBlur > 0 ? "scale(1.1)" : "none",
                      }}
                    />
                  )}
                  {layoutMode === "navbar" ? (
                    <>
                      <GlobalNavbar
                        isBeta={isBeta}
                        navLocked={navLocked}
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
                        navLocked={navLocked}
                        revealStarted={revealStarted}
                        isUpdatingMode={isUpdatingMode}
                        isOnboardingMode={isOnboardingMode}
                        hasEnteredLauncher={hasEnteredLauncher}
                        tryNavigate={tryNavigate}
                      />
                      <TopBar
                        navLocked={navLocked}
                        isOnboardingMode={isOnboardingMode}
                        revealStarted={revealStarted}
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
                        <Routes>
                          <Route
                            path="/"
                            element={
                              <LauncherPage refresh={refresh} count={count} />
                            }
                          />
                          <Route path="/download" element={<DownloadPage />} />
                          <Route
                            path="/tasks"
                            element={<DownloadManagerPage />}
                          />
                          <Route path="/install" element={<InstallPage />} />
                          <Route path="/settings" element={<SettingsPage />} />
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
                          <Route path="/updating" element={<UpdatingPage />} />
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
                    <div className="text-[15px] sm:text-[16px] leading-7 text-default-900 dark:text-zinc-100 font-medium antialiased whitespace-pre-wrap wrap-break-word max-h-[56vh] overflow-y-auto pr-1">
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
