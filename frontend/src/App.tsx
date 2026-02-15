import "./polyfills/wails";
import { Route, Routes } from "react-router-dom";
import React, { useEffect, useState, Suspense, lazy } from "react";
import { ToastProvider, Spinner } from "@heroui/react";
import { GlobalNavbar } from "@/components/GlobalNavbar";
import { Sidebar } from "@/components/Sidebar";
import { TopBar } from "@/components/TopBar";
import { TermsModal } from "@/components/TermsModal";
import { UpdateModal } from "@/components/UpdateModal";
import { motion, AnimatePresence } from "framer-motion";
import { useTranslation } from "react-i18next";
import { VersionStatusProvider } from "@/utils/VersionStatusContext";
import { CurseForgeProvider } from "@/utils/CurseForgeContext";
import { LeviLaminaProvider } from "@/utils/LeviLaminaContext";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { useTheme } from "next-themes";
import { KeybindingProvider, useKeybinding } from "@/utils/KeybindingContext";
import { NavigationHistoryProvider } from "@/utils/NavigationHistoryContext";
import { useThemeManager } from "@/utils/useThemeManager";
import { SplashScreen } from "@/pages/SplashScreen";
import { DownloadsProvider } from "@/utils/DownloadsContext";
import { useLayoutMode } from "@/hooks/useLayoutMode";
import { useAnimations } from "@/hooks/useAnimations";
import { useBackgroundImage, getFitStyles } from "@/hooks/useBackgroundImage";
import { useThemeColors } from "@/hooks/useThemeColors";
import { useAppNavigation } from "@/hooks/useAppNavigation";
import { useAppModals } from "@/hooks/useAppModals";

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
  const { resolvedTheme } = useTheme();
  const hasBackend = minecraft !== undefined;
  const { themeMode } = useThemeManager();

  // Extracted hooks
  const { layoutMode } = useLayoutMode();
  useAnimations();
  useThemeColors(resolvedTheme);
  const {
    bgData,
    backgroundFitMode,
    backgroundBlur,
    backgroundBrightness,
    backgroundOpacity,
    lightBackgroundBaseMode,
    darkBackgroundBaseMode,
    lightBackgroundBaseColor,
    darkBackgroundBaseColor,
    lightBackgroundBaseOpacity,
    darkBackgroundBaseOpacity,
  } = useBackgroundImage();

  const {
    navLocked,
    setNavLocked,
    splashVisible,
    revealStarted,
    isFirstLoad,
    hasEnteredLauncher,
    isBeta,
    isUpdatingMode,
    isOnboardingMode,
    tryNavigate,
    navigate,
  } = useAppNavigation();

  const {
    termsOpen,
    termsCountdown,
    acceptTerms,
    updateOpen,
    updateVersion,
    updateBody,
    updateLoading,
    setUpdateOpen,
    setUpdateLoading,
  } = useAppModals({
    hasBackend,
    revealStarted,
    isUpdatingMode,
    setNavLocked,
  });

  // Counter/refresh for LauncherPage
  const [count, setCount] = useState(0);
  const refresh = () => {
    setCount((prevCount) => prevCount + 1);
  };

  // Prevent file drag/drop on document
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

  // Prevent image drag
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

  // Prevent Tab key
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

  const effectiveNavLocked = navLocked || isUpdatingMode || isOnboardingMode;

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
                        navLocked={effectiveNavLocked}
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
                        navLocked={effectiveNavLocked}
                        themeMode={themeMode}
                        revealStarted={revealStarted}
                        isUpdatingMode={isUpdatingMode}
                        isOnboardingMode={isOnboardingMode}
                        hasEnteredLauncher={hasEnteredLauncher}
                        tryNavigate={tryNavigate}
                      />
                      <TopBar
                        navLocked={effectiveNavLocked}
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

                  <TermsModal
                    isOpen={termsOpen}
                    countdown={termsCountdown}
                    onAccept={acceptTerms}
                  />

                  <UpdateModal
                    isOpen={updateOpen}
                    version={updateVersion}
                    body={updateBody}
                    loading={updateLoading}
                    onDismiss={() => {
                      setUpdateOpen(false);
                      setNavLocked(Boolean((window as any).llNavLock));
                    }}
                    onIgnore={() => {
                      try {
                        localStorage.setItem(
                          "ll.ignoreVersion",
                          updateVersion || "",
                        );
                      } catch {}
                      setUpdateOpen(false);
                      setNavLocked(Boolean((window as any).llNavLock));
                    }}
                    onUpdate={async () => {
                      setUpdateLoading(true);
                      try {
                        setUpdateOpen(false);
                        setNavLocked(true);
                        navigate("/updating", { replace: true });
                      } finally {
                        setUpdateLoading(false);
                      }
                    }}
                  />
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
