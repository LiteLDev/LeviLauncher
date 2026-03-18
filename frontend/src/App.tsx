import "./polyfills/wails";
import { Route, Routes } from "react-router-dom";
import React, { useEffect, useState, Suspense, lazy } from "react";
import { ToastProvider, Spinner } from "@heroui/react";
import { GlobalNavbar } from "@/components/GlobalNavbar";
import { Sidebar } from "@/components/Sidebar";
import { TopBar } from "@/components/TopBar";
import { TermsModal } from "@/components/TermsModal";
import { UpdateModal } from "@/components/UpdateModal";
import { LipUpdateModal } from "@/components/LipUpdateModal";
import { ClarityConsentModal } from "@/components/ClarityConsentModal";
import { useTranslation } from "react-i18next";
import { VersionStatusProvider } from "@/utils/VersionStatusContext";
import { CurseForgeProvider } from "@/utils/CurseForgeContext";
import { LeviLaminaProvider } from "@/utils/LeviLaminaContext";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { useTheme } from "next-themes";
import { KeybindingProvider, useKeybinding } from "@/utils/KeybindingContext";
import { NavigationHistoryProvider } from "@/utils/NavigationHistoryContext";
import { useThemeManager } from "@/utils/useThemeManager";
import { DownloadsProvider } from "@/utils/DownloadsContext";
import { LipTaskConsoleProvider } from "@/utils/LipTaskConsoleContext";
import { CurrentVersionProvider } from "@/utils/CurrentVersionContext";
import { ModIntelligenceProvider } from "@/utils/ModIntelligenceContext";
import { useLayoutMode } from "@/hooks/useLayoutMode";
import { useAnimations } from "@/hooks/useAnimations";
import { useBackgroundImage, getFitStyles } from "@/hooks/useBackgroundImage";
import { useThemeColors } from "@/hooks/useThemeColors";
import { useAppNavigation } from "@/hooks/useAppNavigation";
import { useAppModals } from "@/hooks/useAppModals";
import { ROUTES } from "@/constants/routes";
import { markStartupVisualReady } from "@/utils/startupState";

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
const InstanceSelectPage = lazy(() =>
  import("@/pages/InstanceSelectPage").then((m) => ({
    default: m.InstanceSelectPage,
  })),
);
const InstanceSettingsPage = lazy(() =>
  import("@/pages/InstanceSettingsPage"),
);
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
const ScreenshotsPage = lazy(() => import("@/pages/ScreenshotsPage"));
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
  const { themeColorsReady } = useThemeColors(resolvedTheme);
  const {
    bgData,
    backgroundReady,
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
    clarityPromptOpen,
    acceptClarity,
    declineClarity,
    updateOpen,
    updateVersion,
    updateBody,
    updateLoading,
    lipUpdateOpen,
    lipCurrentVersion,
    lipLatestVersion,
    setUpdateOpen,
    setUpdateLoading,
    setLipUpdateOpen,
  } = useAppModals({
    hasBackend,
    isUpdatingMode,
    isOnboardingMode,
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

  const effectiveNavLocked = navLocked || isUpdatingMode || isOnboardingMode;

  useEffect(() => {
    if (!themeColorsReady || !backgroundReady) return;
    markStartupVisualReady();
  }, [backgroundReady, themeColorsReady]);

  return (
    <KeybindingProvider>
      <CurrentVersionProvider>
        <ModIntelligenceProvider>
          <GlobalShortcuts tryNavigate={tryNavigate} />
          <VersionStatusProvider>
            <DownloadsProvider>
              <LeviLaminaProvider>
                <CurseForgeProvider>
                  <NavigationHistoryProvider>
                    <LipTaskConsoleProvider>
                      <ToastProvider
                        placement="top-center"
                        toastOffset={80}
                        toastProps={{ timeout: 2000 }}
                      />

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
                          }}
                        />
                      )}

                      <div
                        style={
                          {
                            "--content-pt": "4.5rem",
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
                          <GlobalNavbar
                            isBeta={isBeta}
                            navLocked={effectiveNavLocked}
                            themeMode={themeMode}
                            isOnboardingMode={isOnboardingMode}
                            tryNavigate={tryNavigate}
                          />
                        ) : (
                          <>
                            <Sidebar
                              navLocked={effectiveNavLocked}
                              themeMode={themeMode}
                              tryNavigate={tryNavigate}
                            />
                            <TopBar
                              navLocked={effectiveNavLocked}
                              isOnboardingMode={isOnboardingMode}
                              tryNavigate={tryNavigate}
                            />
                          </>
                        )}

                        <div
                          className={`w-full flex-1 min-h-0 overflow-hidden ${
                            layoutMode === "sidebar" ? "pl-14" : ""
                          }`}
                        >
                          <Suspense
                            fallback={
                              <div className="w-full h-full flex items-center justify-center">
                                <Spinner size="lg" />
                              </div>
                            }
                          >
                            <Routes>
                              <Route
                                path={ROUTES.home}
                                element={
                                  <LauncherPage refresh={refresh} count={count} />
                                }
                              />
                              <Route
                                path={ROUTES.download}
                                element={<DownloadPage />}
                              />
                              <Route
                                path={ROUTES.downloadTasks}
                                element={<DownloadManagerPage />}
                              />
                              <Route path="/install" element={<InstallPage />} />
                              <Route
                                path={ROUTES.settings}
                                element={<SettingsPage />}
                              />
                              <Route
                                path={ROUTES.instances}
                                element={<InstanceSelectPage refresh={refresh} />}
                              />
                              <Route
                                path={ROUTES.instanceSettings}
                                element={<InstanceSettingsPage />}
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
                                path={ROUTES.updating}
                                element={<UpdatingPage />}
                              />
                              <Route
                                path={ROUTES.onboarding}
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
                                path="/content/screenshots"
                                element={<ScreenshotsPage />}
                              />
                              <Route
                                path="/content/servers"
                                element={<ServersPage />}
                              />
                              <Route path={ROUTES.about} element={<AboutPage />} />
                            </Routes>
                          </Suspense>
                        </div>

                        <TermsModal
                          isOpen={termsOpen && !isOnboardingMode}
                          countdown={termsCountdown}
                          onAccept={acceptTerms}
                        />

                        <ClarityConsentModal
                          isOpen={clarityPromptOpen && !isOnboardingMode}
                          onEnable={acceptClarity}
                          onKeepDisabled={declineClarity}
                        />

                        <UpdateModal
                          isOpen={updateOpen && !isOnboardingMode}
                          version={updateVersion}
                          body={updateBody}
                          loading={updateLoading}
                          onDismiss={() => {
                            setUpdateOpen(false);
                          }}
                          onIgnore={() => {
                            try {
                              localStorage.setItem(
                                "ll.ignoreVersion",
                                updateVersion || "",
                              );
                            } catch {}
                            setUpdateOpen(false);
                          }}
                          onUpdate={async () => {
                            setUpdateLoading(true);
                            try {
                              setUpdateOpen(false);
                              navigate(ROUTES.updating, { replace: true });
                            } finally {
                              setUpdateLoading(false);
                            }
                          }}
                        />

                        <LipUpdateModal
                          isOpen={lipUpdateOpen && !isOnboardingMode}
                          currentVersion={lipCurrentVersion}
                          latestVersion={lipLatestVersion}
                          onDismiss={() => {
                            setLipUpdateOpen(false);
                          }}
                          onIgnore={() => {
                            try {
                              localStorage.setItem(
                                "ll.ignoreLipVersion",
                                lipLatestVersion || "",
                              );
                            } catch {}
                            setLipUpdateOpen(false);
                          }}
                          onOpenSettings={() => {
                            setLipUpdateOpen(false);
                            navigate(ROUTES.settings, {
                              state: { tab: "components" },
                            });
                          }}
                        />
                      </div>
                    </LipTaskConsoleProvider>
                  </NavigationHistoryProvider>
                </CurseForgeProvider>
              </LeviLaminaProvider>
            </DownloadsProvider>
          </VersionStatusProvider>
        </ModIntelligenceProvider>
      </CurrentVersionProvider>
    </KeybindingProvider>
  );
}

export default App;
