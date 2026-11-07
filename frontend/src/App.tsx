import { Spinner, ToastProvider } from "@heroui/react";

import "./polyfills/wails";
import { Navigate, Route, Routes } from "react-router-dom";
import React, { useEffect, useState, Suspense, lazy } from "react";

import { ActivityCenter } from "@/components/ActivityCenter";
import { CommandPalette } from "@/components/CommandPalette";
import { ShortcutsHelp } from "@/components/ShortcutsHelp";
import { GlobalNavbar } from "@/components/GlobalNavbar";
import { Sidebar } from "@/components/Sidebar";
import { TopBar } from "@/components/TopBar";
import { NavigationFrame } from "@/components/NavigationFrame";
import { TermsModal } from "@/components/TermsModal";
import { AnalyticsConsentModal } from "@/components/AnalyticsConsentModal";
import { useTranslation } from "react-i18next";
import { VersionStatusProvider } from "@/utils/VersionStatusContext";
import { CurseForgeProvider } from "@/utils/CurseForgeContext";
import { LeviLaminaProvider } from "@/utils/LeviLaminaContext";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import { useTheme } from "next-themes";
import {
  KeybindingProvider,
  isOverlayOpen,
  useKeybinding,
} from "@/utils/KeybindingContext";
import { NavigationHistoryProvider } from "@/utils/NavigationHistoryContext";
import { useThemeManager } from "@/utils/useThemeManager";
import { DownloadsProvider } from "@/utils/DownloadsContext";
import { LipTaskConsoleProvider } from "@/utils/LipTaskConsoleContext";
import { CurrentVersionProvider } from "@/utils/CurrentVersionContext";
import { ModIntelligenceProvider } from "@/utils/ModIntelligenceContext";
import {
  SHELL_SIZE,
  useLayoutMode,
  useRailExpanded,
} from "@/hooks/useLayoutMode";
import { DiscoverRedirect } from "@/components/ShellControls";
import { useAnimations } from "@/hooks/useAnimations";
import { useBackgroundImage } from "@/hooks/useBackgroundImage";
import { useBackgroundAppearance } from "@/hooks/useBackgroundAppearance";
import { BackgroundLayers } from "@/components/BackgroundLayers";
import { BackgroundContext } from "@/utils/BackgroundContext";
import { useDocumentAppearance } from "@/hooks/useDocumentAppearance";
import { useThemeColors } from "@/hooks/useThemeColors";
import { useAppNavigation } from "@/hooks/useAppNavigation";
import { useAppModals } from "@/hooks/useAppModals";
import { ROUTES } from "@/constants/routes";
import { markStartupVisualReady } from "@/utils/startupState";
import { openCommandPalette, openShortcutsHelp } from "@/utils/commands";

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
const InstanceSettingsPage = lazy(() => import("@/pages/InstanceSettingsPage"));
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
const OnboardingPage = lazy(() => import("@/pages/OnboardingPage"));
const CurseForgePage = lazy(() => import("@/pages/CurseForgePage"));
const CurseForgeModPage = lazy(() => import("@/pages/CurseForgeModPage"));
const LIPPage = lazy(() => import("@/pages/LIPPage"));
const LIPPackagePage = lazy(() => import("@/pages/LIPPackagePage"));
const ServersPage = lazy(() => import("@/pages/ServersPage"));
const UpdateModal = lazy(() =>
  import("@/components/UpdateModal").then((module) => ({
    default: module.UpdateModal,
  })),
);
const SponsorModal = lazy(() =>
  import("@/components/SponsorModal").then((module) => ({
    default: module.SponsorModal,
  })),
);

const ModalLoadingFallback = ({ label }: { label: string }) => (
  <div
    aria-atomic="true"
    aria-live="polite"
    className="fixed inset-0 z-[100] flex items-center justify-center bg-black/20 launcher-material-blur"
    role="status"
  >
    <div className="flex flex-col items-center gap-2">
      <Spinner />
      <span>{label}</span>
    </div>
  </div>
);

const StartupContentReady = ({ ready }: { ready: boolean }) => {
  useEffect(() => {
    if (!ready) {
      return;
    }

    let secondFrame = 0;
    const firstFrame = window.requestAnimationFrame(() => {
      secondFrame = window.requestAnimationFrame(() => {
        markStartupVisualReady();
      });
    });

    return () => {
      window.cancelAnimationFrame(firstFrame);
      if (secondFrame) {
        window.cancelAnimationFrame(secondFrame);
      }
    };
  }, [ready]);

  return null;
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

    const shortcuts: [string, string, () => void, string][] = [
      ["command-palette", "ctrl+k", openCommandPalette, t("palette.shortcut_open")],
      ["command-palette-alt", "ctrl+p", openCommandPalette, t("palette.shortcut_open")],
      ["shortcuts-help", "shift+?", openShortcutsHelp, t("palette.show_shortcuts")],
      [
        "go-back-alt",
        "alt+arrowleft",
        () => {
          // Never step out of the app's own history.
          if (window.history.state?.idx > 0) tryNavigate(-1);
        },
        t("nav.back"),
      ],
      ["go-forward", "alt+arrowright", () => tryNavigate(1), t("nav.forward")],
      [
        "open-settings",
        "ctrl+,",
        () => tryNavigate(ROUTES.settings),
        t("palette.shortcut_settings"),
      ],
    ];
    for (const [id, combo, run, description] of shortcuts) {
      register(
        id,
        combo,
        (e) => {
          // Ctrl+P would open the browser print dialog even when ignored.
          if (combo === "ctrl+p") e.preventDefault();
          // Page shortcuts stay quiet while an overlay owns the keyboard.
          if (isOverlayOpen()) return;
          e.preventDefault();
          run();
        },
        description,
      );
    }

    return () => {
      unregister("go-back");
      for (const [id] of shortcuts) unregister(id);
    };
  }, [register, unregister, tryNavigate, t]);

  return null;
};

function App() {
  const { t } = useTranslation();
  const { resolvedTheme } = useTheme();
  const { themeMode } = useThemeManager();

  // The tray menu is built in Go, which has no access to the resolved
  // language, so the rendered labels are pushed down on every change.
  useEffect(() => {
    minecraft.SetTrayLabels(t("common.show_launcher"), t("common.quit_launcher"));
  }, [t]);

  // Extracted hooks
  const { layoutMode } = useLayoutMode();
  const railExpanded = useRailExpanded();
  const railRem = railExpanded ? SHELL_SIZE.RAIL_EXPANDED_REM : SHELL_SIZE.RAIL_REM;
  useAnimations();
  const { themeColorsReady } = useThemeColors(resolvedTheme);
  const background = useBackgroundImage();
  const { backgroundReady } = background;
  const { profiles } = useBackgroundAppearance();
  const appearanceMode = resolvedTheme === "dark" ? "dark" : "light";
  const appearance = profiles[appearanceMode];
  const hasWallpaper = Boolean(background.bgData) && background.backgroundOpacity > 0;
  useDocumentAppearance(hasWallpaper, appearance, appearanceMode, background.backgroundBrightness, background.backgroundBlur, background.backgroundOpacity);

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
    analyticsPromptOpen,
    acceptAnalytics,
    declineAnalytics,
    updateOpen,
    updateVersion,
    updateBody,
    updateLoading,
    setUpdateOpen,
    setUpdateLoading,
    sponsorLaunchCount,
    dismissSponsor,
  } = useAppModals({
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

  return (
    <BackgroundContext.Provider value={background}>
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
                        placement="top"
                        className="wails-no-drag z-[120] top-20"
                      />
                      <ActivityCenter />
                      <CommandPalette
                        tryNavigate={tryNavigate}
                        navLocked={effectiveNavLocked}
                      />
                      <ShortcutsHelp />

                      <div
                        data-wallpaper-active={hasWallpaper ? "true" : undefined}
                        data-readability={appearance.readability ? "true" : undefined}
                        style={
                          {
                            "--topbar-h": `${SHELL_SIZE.TOPBAR_REM}rem`,
                            "--rail-w":
                              layoutMode === "sidebar" ? `${railRem}rem` : "0rem",
                            // Pages start one small gap below the top bar.
                            "--content-pt": "calc(var(--topbar-h) + min(1rem, var(--page-gutter)))",
                          } as React.CSSProperties
                        }
                        className={`app-backdrop relative isolate w-full h-dvh flex ${
                          layoutMode === "sidebar" ? "flex-row" : "flex-col"
                        } overflow-x-hidden bg-background text-foreground ${updateOpen ? "overflow-y-hidden" : ""}`}
                      >
                        <BackgroundLayers background={background} mode={appearanceMode} />
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
                            <NavigationFrame
                              railRem={railRem}
                              topRem={SHELL_SIZE.TOPBAR_REM}
                            />
                            <Sidebar
                              navLocked={effectiveNavLocked}
                              themeMode={themeMode}
                              tryNavigate={tryNavigate}
                              expanded={railExpanded}
                            />
                            <TopBar
                              navLocked={effectiveNavLocked}
                              isOnboardingMode={isOnboardingMode}
                              tryNavigate={tryNavigate}
                              railExpanded={railExpanded}
                            />
                          </>
                        )}

                        <main
                          id="main-content"
                          tabIndex={-1}
                          className={`w-full flex-1 min-h-0 overflow-hidden ${
                            layoutMode === "sidebar" ? "pl-[var(--rail-w)]" : ""
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
                                  <LauncherPage
                                    refresh={refresh}
                                    count={count}
                                  />
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
                              <Route
                                path={ROUTES.install}
                                element={<InstallPage />}
                              />
                              <Route
                                path={ROUTES.settings}
                                element={<SettingsPage />}
                              />
                              <Route
                                path={ROUTES.instances}
                                element={
                                  <InstanceSelectPage refresh={refresh} />
                                }
                              />
                              <Route
                                path={ROUTES.instanceSettings}
                                element={<InstanceSettingsPage />}
                              />
                              <Route
                                path={ROUTES.mods}
                                element={<ModsPage />}
                              />
                              <Route
                                path={ROUTES.curseForge}
                                element={<CurseForgePage />}
                              />
                              <Route
                                path={ROUTES.curseForgeMod}
                                element={<CurseForgeModPage />}
                              />
                              <Route path={ROUTES.lip} element={<LIPPage />} />
                              <Route
                                path={ROUTES.lipPackage}
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
                              <Route
                                path={ROUTES.content}
                                element={<ContentPage />}
                              />
                              <Route
                                path={ROUTES.contentWorlds}
                                element={<WorldsListPage />}
                              />
                              <Route
                                path={ROUTES.contentWorldEditor}
                                element={<WorldLevelDatEditorPage />}
                              />
                              <Route
                                path={ROUTES.contentResourcePacks}
                                element={<ResourcePacksPage />}
                              />
                              <Route
                                path={ROUTES.contentBehaviorPacks}
                                element={<BehaviorPacksPage />}
                              />
                              <Route
                                path={ROUTES.contentSkinPacks}
                                element={<SkinPacksPage />}
                              />
                              <Route
                                path={ROUTES.contentScreenshots}
                                element={<ScreenshotsPage />}
                              />
                              <Route
                                path={ROUTES.contentServers}
                                element={<ServersPage />}
                              />
                              <Route
                                path={ROUTES.about}
                                element={
                                  <Navigate
                                    to={`${ROUTES.settings}?tab=about`}
                                    replace
                                  />
                                }
                              />
                              <Route
                                path={ROUTES.discover}
                                element={<DiscoverRedirect />}
                              />
                              <Route
                                path="*"
                                element={<Navigate to={ROUTES.home} replace />}
                              />
                            </Routes>
                            <StartupContentReady
                              ready={themeColorsReady && backgroundReady}
                            />
                          </Suspense>
                        </main>

                        <TermsModal
                          isOpen={
                            termsOpen && !isOnboardingMode && !isUpdatingMode
                          }
                          countdown={termsCountdown}
                          onAccept={acceptTerms}
                        />

                        <AnalyticsConsentModal
                          isOpen={
                            analyticsPromptOpen &&
                            !isOnboardingMode &&
                            !isUpdatingMode
                          }
                          onEnable={acceptAnalytics}
                          onKeepDisabled={declineAnalytics}
                        />

                        {sponsorLaunchCount > 0 &&
                        !isOnboardingMode &&
                        !isUpdatingMode ? (
                          <Suspense
                            fallback={
                              <ModalLoadingFallback
                                label={t("common.loading")}
                              />
                            }
                          >
                            <SponsorModal
                              launchCount={sponsorLaunchCount}
                              onDismiss={dismissSponsor}
                            />
                          </Suspense>
                        ) : null}

                        {updateOpen && !isOnboardingMode && !isUpdatingMode ? (
                          <Suspense
                            fallback={
                              <ModalLoadingFallback
                                label={t("common.loading")}
                              />
                            }
                          >
                            <UpdateModal
                              isOpen
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
                          </Suspense>
                        ) : null}
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
    </BackgroundContext.Provider>
  );
}

export default App;
