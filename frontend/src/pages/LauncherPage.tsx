import {
  Button,
  Card,
  Chip,
  Dropdown,
  Label,
  Spinner,
} from "@heroui/react";

import { useEffect, useMemo, useRef } from "react";

import { useTranslation } from "react-i18next";

import {
  FaRocket,
  FaCog,
  FaCogs,
  FaList,
  FaWindows,
  FaFolderOpen,
  FaDesktop,
  FaRegLightbulb,
} from "react-icons/fa";
import { HomeContentCard } from "@/components/HomeContent";
import { ContentDownloadCard } from "@/components/ContentDownloadCard";
import { LaunchStepsDialog } from "@/components/LaunchStepsDialog";
import { Browser } from "@wailsio/runtime";
import { motion, AnimatePresence, useReducedMotion } from "framer-motion";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { cn } from "@/utils/cn";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { ROUTES } from "@/constants/routes";
import { useLauncher } from "@/hooks/useLauncher";
import { useModIntelligence } from "@/utils/ModIntelligenceContext";
import { useLeviLamina } from "@/utils/LeviLaminaContext";

const LAUNCH_TIP_KEYS = [
  "version_selector",
  "sidebar_toggle",
  "manage_versions",
  "quick_actions_menu",
  "launch_dependencies",
  "content_section",
  "content_download_sources",
  "settings_personalize",
  "settings_storage_path",
  "download_mirror",
  "open_source",
  "backup_worlds",
] as const;

export const LauncherPage = (args: any) => {
  const { t } = useTranslation();
  const prefersReducedMotion = useReducedMotion();
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const { ensureInstanceHydrated, getInstanceSnapshot, snapshotRevision } =
    useModIntelligence();
  const { getLatestLLVersion, compareLLVersions } = useLeviLamina();

  const {
    // State
    isAnimating,
    setIsAnimating,
    currentVersion,
    displayName,
    localVersionMap,
    logoDataUrl,
    registrationPendingAction,
    launchPending,
    launchStepRunning,
    requiresUWPRegistration,
    tipIndex,
    launchFlow,

    // Navigation
    navigate,

    // Computed

    // Tip timer
    startTipTimer,
    stopTipTimer,

    // Handlers
    doPrimaryAction,
    doForceLaunch,
    doCreateShortcut,
    doOpenFolder,
    doRegister,
    installDependency,
    recheckDependency,
    skipDependency,
    ignoreGamingServices,
    openGamingServicesStore,
    retryLaunchStep,
    closeLaunchFlow,
  } = useLauncher(args);
  const currentVersionName = String(currentVersion || "").trim();
  const currentVersionInfo = currentVersionName
    ? localVersionMap.get(currentVersionName)
    : undefined;

  const launchTips = useMemo(
    () => LAUNCH_TIP_KEYS.map((key) => String(t(`launcherpage.tip.${key}`))),
    [t],
  );
  const currentLaunchTip = launchTips[tipIndex] ?? launchTips[0] ?? "";

  useEffect(() => {
    startTipTimer(launchTips.length);
    return () => {
      stopTipTimer();
    };
  }, [launchTips.length, startTipTimer, stopTipTimer]);

  useEffect(() => {
    if (!currentVersionName || currentVersionInfo?.packageType === "uwp" || !currentVersionInfo?.isLeviLaminaInstalled) {
      return;
    }
    void ensureInstanceHydrated(currentVersionName, {
      background: true,
      reason: "launcher-hero-ll-chip",
    });
  }, [
    currentVersionInfo?.isLeviLaminaInstalled,
    currentVersionInfo?.packageType,
    currentVersionName,
    ensureInstanceHydrated,
  ]);

  const currentInstanceSnapshot = useMemo(
    () => (currentVersionName ? getInstanceSnapshot(currentVersionName) : null),
    [currentVersionName, getInstanceSnapshot, snapshotRevision],
  );
  const isCurrentVersionRegistered = Boolean(currentVersionInfo?.isRegistered);
  const currentVersionHasLeviLamina = Boolean(
    currentVersionInfo?.packageType !== "uwp" && currentVersionInfo?.isLeviLaminaInstalled,
  );
  const currentGameVersion = String(currentVersionInfo?.version || "").trim();
  const currentLeviLaminaVersion = String(
    currentInstanceSnapshot?.llState?.installedVersion || "",
  ).trim();
  const latestLeviLaminaVersion = useMemo(
    () =>
      currentGameVersion
        ? String(getLatestLLVersion(currentGameVersion) || "").trim()
        : "",
    [currentGameVersion, getLatestLLVersion],
  );
  const hasLeviLaminaUpdateAvailable = useMemo(() => {
    if (
      !currentVersionHasLeviLamina ||
      !currentLeviLaminaVersion ||
      !latestLeviLaminaVersion
    ) {
      return false;
    }
    const compared = compareLLVersions(
      latestLeviLaminaVersion,
      currentLeviLaminaVersion,
    );
    return Number.isFinite(compared) && compared > 0;
  }, [
    compareLLVersions,
    currentLeviLaminaVersion,
    currentVersionHasLeviLamina,
    latestLeviLaminaVersion,
  ]);

  return (
    <>
      <PageContainer
        ref={scrollRef}
        id="content-drop-zone"
        {...({ "data-file-drop-target": true } as any)}
        className={cn("relative", isAnimating ? "overflow-hidden" : "")}
        animate={false}
      >
        {/* Cap the dashboard so cards never stretch into empty slabs on wide screens. */}
        <div className="mx-auto flex w-full max-w-[100rem] flex-col gap-[var(--page-gap)]">
        {/* Hero Launch Card */}
        <motion.div
          data-material-motion
          initial={{ opacity: 0, y: -20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.4, ease: "easeOut" }}
        >
          <Card
            className={cn("relative overflow-hidden", LAYOUT.GLASS_CARD.BASE)}
          >
            <Card.Content className="@container relative flex flex-col gap-3 p-4 sm:p-6 sm:pb-4 @7xl:gap-5">
              {/* Main Layout */}
              <div className="flex flex-col gap-3 @3xl:flex-row @3xl:items-center @3xl:justify-between @3xl:gap-6">
                {/* Left: Title & Info */}
                <div className="flex flex-col gap-1 min-w-0">
                  <div className="flex min-w-0 items-center gap-3">
                    <motion.h1
                      className="min-w-0 text-4xl @xl:text-5xl @7xl:text-6xl font-black tracking-tight text-brand-heading truncate pb-1 @xl:pb-2"
                      initial={{ opacity: 0, x: -20 }}
                      animate={{ opacity: 1, x: 0 }}
                      transition={{ delay: 0.2 }}
                    >
                      Minecraft
                    </motion.h1>
                    {isCurrentVersionRegistered && (
                      <motion.div
                        initial={{ opacity: 0, scale: 0.8, x: -10 }}
                        animate={{ opacity: 1, scale: 1, x: 0 }}
                        transition={{
                          delay: 0.7,
                          type: "spring",
                          stiffness: 500,
                          damping: 30,
                        }}
                      >
                        <Chip
                          variant="soft"
                          color={"accent"}
                          className={
                            "bg-brand-500/10 border border-brand-500/20 hidden sm:flex"
                          }
                        >
                          <Chip.Label
                            className={
                              "font-semibold text-brand-600 dark:text-brand-500"
                            }
                          >
                            {t("launcherpage.registered_tip")}
                          </Chip.Label>
                        </Chip>
                      </motion.div>
                    )}
                    {currentVersionHasLeviLamina && (
                      <motion.div
                        initial={{ opacity: 0, scale: 0.8, x: -10 }}
                        animate={{ opacity: 1, scale: 1, x: 0 }}
                        transition={{
                          delay: 0.75,
                          type: "spring",
                          stiffness: 500,
                          damping: 30,
                        }}
                      >
                        <Chip<"button">
                          render={
                            hasLeviLaminaUpdateAvailable
                              ? (props) => (
                                  <button
                                    {...props}
                                    type="button"
                                    onClick={() =>
                                      navigate(ROUTES.instanceSettings, {
                                        state: {
                                          name: currentVersionName,
                                          tab: "loader",
                                          returnTo: ROUTES.home,
                                        },
                                      })
                                    }
                                  />
                                )
                              : undefined
                          }
                          variant="soft"
                          color={
                            hasLeviLaminaUpdateAvailable ? "warning" : "accent"
                          }
                          className={cn(
                            "hidden sm:flex border",
                            hasLeviLaminaUpdateAvailable
                              ? "bg-amber-500/10 border-amber-500/20 cursor-pointer transition-colors hover:bg-amber-500/20 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-500"
                              : "bg-brand-500/10 border-brand-500/20",
                          )}
                        >
                          <Chip.Label
                            className={cn(
                              "font-semibold",
                              hasLeviLaminaUpdateAvailable
                                ? "text-amber-600 dark:text-amber-400"
                                : "text-brand-600 dark:text-brand-500",
                            )}
                          >
                            {hasLeviLaminaUpdateAvailable
                              ? `LeviLamina · ${t("launcherpage.levilamina_update_available")}`
                              : "LeviLamina"}
                          </Chip.Label>
                        </Chip>
                      </motion.div>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="text-base @xl:text-lg @7xl:text-xl font-medium text-foreground dark:text-zinc-300">
                      {t("launcherpage.edition")}
                    </span>
                  </div>
                </div>

                {/* Right: Actions */}
                <div className="flex min-w-0 flex-wrap items-center gap-2 @3xl:shrink-0 @3xl:flex-nowrap @3xl:gap-3">
                  {/* Version Selector */}
                  <div className="flex min-w-0 flex-1 items-center gap-1 @3xl:flex-none">
                    {/* The instance is picked in the top bar; Home shows which one launches. */}
                    <div
                      title={displayName || undefined}
                      className="flex h-12 min-w-0 max-w-full @3xl:max-w-[min(22rem,40cqi)] items-center gap-3 px-3"
                    >
                      <div className="w-8 h-8 shrink-0 rounded-lg bg-surface-tertiary/50 dark:bg-surface/10 flex items-center justify-center overflow-hidden shadow-sm">
                        {logoDataUrl ? (
                          <img
                            src={logoDataUrl}
                            alt=""
                            aria-hidden="true"
                            className="w-full h-full object-cover"
                          />
                        ) : (
                          <span aria-hidden="true" className="text-base font-bold">
                            M
                          </span>
                        )}
                      </div>
                      <div className="flex min-w-0 flex-1 flex-col">
                        <span className="text-xs font-medium text-muted">
                          {t("launcherpage.currentVersion")}
                        </span>
                        <span className="truncate text-sm font-bold leading-tight">
                          {displayName || t("launcherpage.currentVersion_none")}
                          {currentGameVersion && (
                            <span className="ml-2 font-normal text-muted tabular-nums">
                              {currentGameVersion}
                            </span>
                          )}
                        </span>
                      </div>
                    </div>

                    <Dropdown>
                      <Button
                        isIconOnly
                        size="sm"
                        aria-label={t("launcherpage.tip.quick_actions_menu")}
                        variant={"ghost"}
                        className={cn(
                          "rounded-full",
                          "data-[hovered]:bg-surface-tertiary/50 dark:data-[hovered]:bg-surface/5",
                        )}
                      >
                        <FaCogs size={18} />
                      </Button>
                      <Dropdown.Popover
                        className={COMPONENT_STYLES.dropdown.content}
                      >
                        {!currentVersion && <p className="px-3 py-2 text-xs text-muted">{t("audit.primary.select_instance_first")}</p>}
                        <Dropdown.Menu
                          disabledKeys={currentVersion ? [] : ["shortcut", "folder", "register"]}
                          aria-label={t("launcherpage.tip.quick_actions_menu")}
                        >
                          <Dropdown.Item
                            key="settings"
                            id={"settings"}
                            textValue={t("launcherpage.go_version_settings")}
                            onAction={() => {
                              if (currentVersion) {
                                navigate(ROUTES.instanceSettings, {
                                  state: {
                                    name: currentVersion,
                                    returnTo: ROUTES.home,
                                  },
                                });
                              } else {
                                navigate(ROUTES.instances);
                              }
                            }}
                          >
                            {<FaCog />}
                            <Label>
                              {t("launcherpage.go_version_settings")}
                            </Label>
                            <Dropdown.ItemIndicator />
                          </Dropdown.Item>
                          <Dropdown.Item
                            key="shortcut"
                            id={"shortcut"}
                            textValue={t("launcherpage.shortcut.create_button")}
                            onAction={doCreateShortcut}
                          >
                            {<FaDesktop />}
                            <Label>
                              {t("launcherpage.shortcut.create_button")}
                            </Label>
                            <Dropdown.ItemIndicator />
                          </Dropdown.Item>
                          <Dropdown.Item
                            key="folder"
                            id={"folder"}
                            textValue={t("launcherpage.open_exe_dir")}
                            onAction={doOpenFolder}
                          >
                            {<FaFolderOpen />}
                            <Label>{t("launcherpage.open_exe_dir")}</Label>
                            <Dropdown.ItemIndicator />
                          </Dropdown.Item>
                          <Dropdown.Item
                            key="register"
                            id={"register"}
                            textValue={
                              isCurrentVersionRegistered
                                ? t("versions.edit.unregister_button")
                                : t("launcherpage.register_system_button")
                            }
                            onAction={() => void doRegister()}
                            isDisabled={!currentVersion || registrationPendingAction !== null || launchPending}
                          >
                            {<FaWindows />}
                            <Label>
                              {isCurrentVersionRegistered
                                ? t("versions.edit.unregister_button")
                                : t("launcherpage.register_system_button")}
                            </Label>
                            <Dropdown.ItemIndicator />
                          </Dropdown.Item>
                        </Dropdown.Menu>
                      </Dropdown.Popover>
                    </Dropdown>
                  </div>
                  {/* Launch Button */}
                  <motion.div
                    whileHover={{ scale: 1.02 }}
                    whileTap={{ scale: 0.98 }}
                    className="w-full @md:w-auto"
                  >
                    <Button
                      data-testid="primary-launch-button"
                      size="lg"
                      onPress={doPrimaryAction}
                      variant={"secondary"}
                      isPending={registrationPendingAction !== null || launchPending || launchStepRunning}
                      isDisabled={registrationPendingAction !== null || launchPending || (Boolean(currentVersion) && !currentVersionInfo)}
                      className={
                        "h-12 @xl:h-14 @7xl:h-16 px-8 @7xl:px-10 text-lg @7xl:text-xl font-bold brand-primary-foreground shadow-brand-900/20 shadow-lg bg-brand-500 hover:bg-brand-500 rounded-2xl w-full @md:w-auto"
                      }
                    >
                      {({ isPending }) => (
                        <>
                          <Spinner
                            size="sm"
                            color="current"
                            className={isPending ? "" : "hidden"}
                          />
                          {requiresUWPRegistration ? (
                            <FaWindows className="mb-0.5" />
                          ) : currentVersion ? (
                            <FaRocket className="mb-0.5" />
                          ) : (
                            <FaList className="mb-0.5" />
                          )}
                          {t(
                            registrationPendingAction === "register"
                              ? "uwp.registering"
                              : registrationPendingAction === "unregister"
                                ? "uwp.unregistering"
                                : launchPending || launchStepRunning
                                  ? "uwp.launching"
                                    : requiresUWPRegistration
                                      ? "uwp.register_button"
                                      : currentVersion
                                        ? "launcherpage.launch_button"
                                        : "audit.primary.download_minecraft",
                          )}
                        </>
                      )}
                    </Button>
                  </motion.div>
                </div>
              </div>
              {/* Reserve the tallest translated tip so rotation never shifts the cards below. */}
              <div
                role="note"
                className="flex min-w-0 items-start gap-2 border-t border-border/60 pt-3 text-sm leading-5 text-muted"
              >
                <FaRegLightbulb
                  aria-hidden="true"
                  className="mt-0.5 size-4 shrink-0"
                />
                <div className="grid min-w-0 flex-1">
                  {launchTips.map((tip, index) => (
                    <p
                      key={LAUNCH_TIP_KEYS[index]}
                      aria-hidden="true"
                      className="invisible col-start-1 row-start-1 select-none wrap-anywhere"
                    >
                      {tip}
                    </p>
                  ))}
                  <AnimatePresence initial={false} mode="wait">
                    <motion.p
                      key={tipIndex}
                      initial={{ opacity: 0 }}
                      animate={{ opacity: 1 }}
                      exit={{ opacity: 0 }}
                      transition={{ duration: prefersReducedMotion ? 0 : 0.2 }}
                      className="col-start-1 row-start-1 wrap-anywhere"
                    >
                      {currentLaunchTip}
                    </motion.p>
                  </AnimatePresence>
                </div>
              </div>
            </Card.Content>
          </Card>
        </motion.div>

        {/* The row of cards released versions had under the launch card; the
            content card now also holds mods, so it takes two columns. */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-[var(--page-gap)] items-stretch">
          <motion.div
            data-material-motion
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.3 }}
            className="min-w-0 md:col-span-2"
          >
            <HomeContentCard dropZoneRef={scrollRef} />
          </motion.div>
          <motion.div
            data-material-motion
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.4 }}
            className="min-w-0"
            onAnimationComplete={() => setIsAnimating(false)}
          >
            <ContentDownloadCard />
          </motion.div>
        </div>
        </div>

        <LaunchStepsDialog
          flow={launchFlow}
          canLaunch={Boolean(currentVersion)}
          launchLabel={String(
            t(requiresUWPRegistration ? "uwp.register_button" : "launcherpage.launch_button"),
          )}
          isUWP={currentVersionInfo?.packageType === "uwp"}
          onClose={closeLaunchFlow}
          onLaunch={doPrimaryAction}
          onForceLaunch={doForceLaunch}
          onRetry={retryLaunchStep}
          onInstall={installDependency}
          onRecheck={recheckDependency}
          onSkip={skipDependency}
          onIgnoreGamingServices={ignoreGamingServices}
          onOpenStore={() => openGamingServicesStore(Browser.OpenURL)}
        />
      </PageContainer>
    </>
  );
};
