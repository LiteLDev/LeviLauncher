import {
  Button,
  Card,
  Chip,
  Description,
  Dropdown,
  InputGroup,
  Label,
  ProgressBar,
  Spinner,
  TextField,
} from "@heroui/react";

import { useEffect, useMemo } from "react";

import { useTranslation } from "react-i18next";

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
  FaDesktop,
  FaCube,
  FaArrowRight,
  FaDownload,
  FaExclamationTriangle,
} from "react-icons/fa";
import { ModCard } from "@/components/ModdedCard";
import { ContentDownloadCard } from "@/components/ContentDownloadCard";
import { Window, Browser } from "@wailsio/runtime";
import { motion, AnimatePresence } from "framer-motion";
import {
  UnifiedModal,
  getUnifiedModalConfirmButtonProps,
} from "@/components/UnifiedModal";
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
  "version_search",
  "manage_versions",
  "quick_actions_menu",
  "launch_dependencies",
  "mods_card",
  "content_counts_card",
  "incompatible_resource_packs",
  "content_download_sources",
  "settings_personalize",
  "settings_storage_path",
  "download_mirror",
  "open_source",
  "backup_worlds",
] as const;

export const LauncherPage = (args: any) => {
  const { t } = useTranslation();
  const warningConfirmButtonProps =
    getUnifiedModalConfirmButtonProps("warning");
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
    launchErrorCode,
    contentCounts,
    incompatibleShaderCount,
    giTotal,
    giDownloaded,
    vcTotal,
    vcDownloaded,
    logoDataUrl,
    versionQuery,
    setVersionQuery,
    logoByName,
    isLoadingVersions,
    registerAction,
    tipIndex,

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

    // Navigation
    navigate,

    // Computed
    buildVersionMenuItems,

    // Tip timer
    startTipTimer,
    stopTipTimer,

    // Handlers
    doLaunch,
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
    if (!currentVersionName || !currentVersionInfo?.isLeviLaminaInstalled) {
      return;
    }
    void ensureInstanceHydrated(currentVersionName, {
      background: true,
      reason: "launcher-hero-ll-chip",
    });
  }, [
    currentVersionInfo?.isLeviLaminaInstalled,
    currentVersionName,
    ensureInstanceHydrated,
  ]);

  const worldsLabel = t("content.count.worlds") as string;
  const resourceLabel = t("content.count.resource_packs") as string;
  const behaviorLabel = t("content.count.behavior_packs") as string;
  const launchErrorMessage = useMemo(() => {
    const key = `errors.${launchErrorCode}`;
    const translated = t(key) as unknown as string;
    if (launchErrorCode && translated && translated !== key) return translated;

    const fallback = t("errors.ERR_LAUNCH_GAME") as unknown as string;
    if (fallback && fallback !== "errors.ERR_LAUNCH_GAME") return fallback;

    return "Launch failed";
  }, [launchErrorCode, t]);

  const versionMenuItems = useMemo(
    () => buildVersionMenuItems(t("common.empty") as string),
    [buildVersionMenuItems, t],
  );
  const currentInstanceSnapshot = useMemo(
    () => (currentVersionName ? getInstanceSnapshot(currentVersionName) : null),
    [currentVersionName, getInstanceSnapshot, snapshotRevision],
  );
  const isCurrentVersionRegistered = Boolean(currentVersionInfo?.isRegistered);
  const currentVersionHasLeviLamina = Boolean(
    currentVersionInfo?.isLeviLaminaInstalled,
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
        className={cn("relative", isAnimating ? "overflow-hidden" : "")}
        animate={false}
      >
        {/* Hero Launch Card */}
        <motion.div
          initial={{ opacity: 0, y: -20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.4, ease: "easeOut" }}
        >
          <Card
            className={cn("relative overflow-hidden", LAYOUT.GLASS_CARD.BASE)}
          >
            <Card.Content className="p-6 relative flex flex-col gap-6">
              {/* Main Layout */}
              <div className="flex flex-col md:flex-row md:items-center justify-between gap-6">
                {/* Left: Title & Info */}
                <div className="flex flex-col gap-1 min-w-0">
                  <div className="flex items-center gap-3">
                    <motion.h1
                      className="text-4xl sm:text-5xl font-black tracking-tight text-transparent bg-clip-text bg-gradient-to-r from-brand-600 to-brand-400 dark:from-brand-400 dark:to-brand-600 truncate pb-2"
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
                        <Chip
                          variant="soft"
                          color={
                            hasLeviLaminaUpdateAvailable ? "warning" : "accent"
                          }
                          className={cn(
                            "hidden sm:flex border",
                            hasLeviLaminaUpdateAvailable
                              ? "bg-amber-500/10 border-amber-500/20"
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
                    <span className="text-lg sm:text-xl font-medium text-foreground dark:text-zinc-300">
                      {t("launcherpage.edition")}
                    </span>
                  </div>
                </div>

                {/* Right: Actions */}
                <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-3 shrink-0">
                  {/* Version Selector */}
                  <div className="flex items-center gap-3 p-1.5 rounded-2xl">
                    <Dropdown>
                      <Button
                        variant={"ghost"}
                        className={
                          "h-12 px-3 rounded-xl data-[hovered]:bg-surface-tertiary/50 dark:data-[hovered]:bg-white/5"
                        }
                      >
                        <div className="flex items-center gap-3 text-left">
                          <div className="w-8 h-8 rounded-lg bg-surface-tertiary/50 dark:bg-white/10 flex items-center justify-center overflow-hidden shadow-sm">
                            {logoDataUrl ? (
                              <img
                                src={logoDataUrl}
                                alt="logo"
                                className="w-full h-full object-cover"
                              />
                            ) : (
                              <span className="text-base font-bold text-foreground dark:text-zinc-300">
                                M
                              </span>
                            )}
                          </div>
                          <div className="flex flex-col hidden lg:flex">
                            <span className="text-xs text-foreground dark:text-zinc-300 font-medium">
                              {t("launcherpage.currentVersion")}
                            </span>
                            <span className="text-sm font-bold text-foreground dark:text-white leading-tight max-w-[120px] truncate">
                              {displayName ||
                                t("launcherpage.currentVersion_none")}
                            </span>
                          </div>
                          <span className="text-sm font-bold text-foreground dark:text-white leading-tight max-w-[120px] truncate lg:hidden">
                            {displayName ||
                              t("launcherpage.currentVersion_none")}
                          </span>
                          <FaChevronDown
                            className="text-muted dark:text-zinc-300 ml-1"
                            size={12}
                          />
                        </div>
                      </Button>
                      <Dropdown.Popover
                        placement="bottom end"
                        className={COMPONENT_STYLES.dropdown.content}
                      >
                        {
                          <div className="p-3 border-b border-border dark:border-border/10">
                            <TextField
                              aria-label={t("launcherpage.search_versions")}
                              className={cn(
                                "group",
                                COMPONENT_STYLES.input.mainWrapper,
                              )}
                              value={versionQuery}
                              onChange={setVersionQuery}
                            >
                              <InputGroup
                                className={cn(
                                  COMPONENT_STYLES.input.inputWrapper,
                                  COMPONENT_STYLES.input.innerWrapper,
                                  "min-h-8 text-sm",
                                )}
                              >
                                <InputGroup.Prefix>
                                  {<FaList className="text-muted" />}
                                </InputGroup.Prefix>
                                <InputGroup.Input
                                  placeholder={t(
                                    "launcherpage.search_versions",
                                  )}
                                  className={COMPONENT_STYLES.input.input}
                                />
                              </InputGroup>
                            </TextField>
                            <Button
                              fullWidth
                              size="sm"
                              onPress={() => navigate(ROUTES.instances)}
                              variant={"secondary"}
                              className={"mt-2"}
                            >
                              {t("launcherpage.manage_versions")}
                            </Button>
                          </div>
                        }
                        <Dropdown.Menu
                          aria-label="Version Selection"
                          selectionMode="single"
                          selectedKeys={
                            new Set(currentVersion ? [currentVersion] : [])
                          }
                          className="max-h-[400px] overflow-y-auto no-scrollbar min-w-[300px]"
                          items={versionMenuItems}
                          onSelectionChange={handleVersionSelect}
                        >
                          {(item: any) => (
                            <Dropdown.Item
                              key={item.key}
                              id={item.key}
                              textValue={item.name}
                            >
                              {
                                <div className="w-8 h-8 shrink-0 rounded-lg bg-surface-secondary dark:bg-white/10 flex items-center justify-center overflow-hidden">
                                  {(() => {
                                    const u =
                                      item.logo || logoByName.get(item.name);
                                    return u ? (
                                      <img
                                        src={u}
                                        alt=""
                                        aria-hidden="true"
                                        className="w-full h-full object-cover"
                                      />
                                    ) : (
                                      <span className="text-sm font-bold text-foreground dark:text-zinc-300">
                                        M
                                      </span>
                                    );
                                  })()}
                                </div>
                              }
                              <Label>
                                <div className="flex justify-between items-center gap-2">
                                  <span className="font-semibold">
                                    {item.name}
                                  </span>
                                  <div className="flex items-center gap-1">
                                    {item.isLeviLaminaInstalled && (
                                      <Chip
                                        size="sm"
                                        variant="soft"
                                        color={"accent"}
                                        className={
                                          "bg-brand-500/10 border border-brand-500/20 h-5 px-1"
                                        }
                                      >
                                        <Chip.Label
                                          className={
                                            "text-brand-600 dark:text-brand-500 font-bold text-[10px]"
                                          }
                                        >
                                          LeviLamina
                                        </Chip.Label>
                                      </Chip>
                                    )}
                                    {item.isRegistered && (
                                      <Chip
                                        size="sm"
                                        variant="soft"
                                        color={"accent"}
                                        className={
                                          "bg-brand-500/10 border border-brand-500/20 h-5 px-1"
                                        }
                                      >
                                        <Chip.Label
                                          className={
                                            "text-brand-600 dark:text-brand-500 font-bold text-[10px]"
                                          }
                                        >
                                          {t("launcherpage.registered_tip")}
                                        </Chip.Label>
                                      </Chip>
                                    )}
                                  </div>
                                </div>
                              </Label>
                              <Description>{item.version}</Description>
                              <Dropdown.ItemIndicator />
                            </Dropdown.Item>
                          )}
                        </Dropdown.Menu>
                        {isLoadingVersions ? (
                          <div className="p-2 flex justify-center items-center gap-2 text-muted text-xs border-t border-border dark:border-white/5">
                            <Spinner size="sm" color={"accent"} />
                            <span>{t("common.loading")}</span>
                          </div>
                        ) : null}
                      </Dropdown.Popover>
                    </Dropdown>

                    <Dropdown>
                      <Button
                        isIconOnly
                        size="sm"
                        aria-label={t("launcherpage.tip.quick_actions_menu")}
                        variant={"ghost"}
                        className={cn(
                          "rounded-full",
                          "data-[hovered]:bg-surface-tertiary/50 dark:data-[hovered]:bg-white/5",
                        )}
                      >
                        <FaCogs
                          size={18}
                          className="text-foreground dark:text-zinc-300"
                        />
                      </Button>
                      <Dropdown.Popover
                        className={COMPONENT_STYLES.dropdown.content}
                      >
                        <Dropdown.Menu
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
                            onAction={doRegister}
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
                  >
                    <Button
                      data-testid="primary-launch-button"
                      size="lg"
                      onPress={doLaunch}
                      variant={"secondary"}
                      isPending={mcLaunchLoadingDisclosure.isOpen}
                      className={
                        "h-14 px-8 text-lg font-bold brand-primary-foreground shadow-brand-900/20 shadow-lg bg-brand-500 hover:bg-brand-500 rounded-2xl w-full sm:w-auto"
                      }
                    >
                      {({ isPending }) => (
                        <>
                          <Spinner
                            size="sm"
                            color="current"
                            className={isPending ? "" : "hidden"}
                          />
                          {currentVersion ? (
                            <FaRocket className="mb-0.5" />
                          ) : (
                            <FaList className="mb-0.5" />
                          )}
                          {t(
                            currentVersion
                              ? "launcherpage.launch_button"
                              : "launcherpage.manage_versions",
                          )}
                        </>
                      )}
                    </Button>
                  </motion.div>
                </div>
              </div>
              {/* Tips (Bottom) */}
              <div className="w-full rounded-xl px-4 py-2 flex items-center gap-2">
                <span className="text-lg">💡</span>
                <div className="flex-1 overflow-hidden h-[20px] relative">
                  <AnimatePresence mode="wait">
                    <motion.div
                      key={tipIndex}
                      initial={{ opacity: 0, y: 10 }}
                      animate={{ opacity: 1, y: 0 }}
                      exit={{ opacity: 0, y: -10 }}
                      className="text-sm text-foreground dark:text-zinc-300 font-medium truncate absolute inset-0"
                    >
                      {currentLaunchTip}
                    </motion.div>
                  </AnimatePresence>
                </div>
              </div>
            </Card.Content>
          </Card>
        </motion.div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 lg:gap-4 items-stretch">
          {/* Mod Card */}
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.3 }}
            className="md:col-span-1"
          >
            <ModCard
              localVersionMap={localVersionMap}
              currentVersion={currentVersion}
            />
          </motion.div>

          {/* Content Management */}
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.4 }}
            className="md:col-span-1"
          >
            <Card
              className={cn(
                "h-full transition-all group",
                LAYOUT.GLASS_CARD.BASE,
                "rounded-3xl",
              )}
            >
              <Card.Header className="px-5 py-3 border-b border-border dark:border-white/5 flex justify-between items-center">
                <div className="flex items-center gap-2">
                  <div className="p-1.5 rounded-lg bg-pink-500/10 text-pink-600 dark:text-pink-400">
                    <FaCube size={16} />
                  </div>
                  <h3 className="text-base font-bold text-foreground dark:text-zinc-100">
                    {t("launcherpage.content_manage")}
                  </h3>
                </div>
                <Button
                  size="sm"
                  onPress={() => navigate(ROUTES.content)}
                  variant={"ghost"}
                  className={
                    "text-xs text-foreground dark:text-zinc-300 data-[hovered]:text-foreground dark:data-[hovered]:text-zinc-200"
                  }
                >
                  {t("common.view_all")}
                  {<FaArrowRight size={10} />}
                </Button>
              </Card.Header>
              <Card.Content className="p-3 gap-2 relative">
                {incompatibleShaderCount > 0 && (
                  <button
                    type="button"
                    className="group/hint flex w-full items-center justify-between p-2 text-left rounded-xl bg-amber-500/10 hover:bg-amber-500/20 text-amber-600 dark:text-amber-400 cursor-pointer transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-500"
                    onClick={() =>
                      navigate(ROUTES.contentResourcePacks, {
                        state: { showIncompatible: true },
                      })
                    }
                  >
                    <div className="flex items-center gap-3">
                      <FaExclamationTriangle size={16} />
                      <span className="font-medium text-sm">
                        {t("contentpage.only_show_updates")}
                      </span>
                    </div>
                    <span className="font-bold text-sm">
                      {incompatibleShaderCount}
                    </span>
                  </button>
                )}
                {[
                  {
                    label: worldsLabel,
                    count: contentCounts.worlds,
                    icon: FaGlobe,
                    path: ROUTES.contentWorlds,
                    color: "text-blue-500",
                  },
                  {
                    label: resourceLabel,
                    count: contentCounts.resourcePacks,
                    icon: FaImage,
                    path: ROUTES.contentResourcePacks,
                    color: "text-purple-500",
                  },
                  {
                    label: behaviorLabel,
                    count: contentCounts.behaviorPacks,
                    icon: FaCogs,
                    path: ROUTES.contentBehaviorPacks,
                    color: "text-orange-500",
                  },
                ].map((item) => (
                  <button
                    key={item.path}
                    type="button"
                    className="group/item flex w-full items-center justify-between p-2 text-left rounded-xl hover:bg-surface-tertiary/50 dark:hover:bg-zinc-700/50 cursor-pointer transition-all duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
                    onClick={() => navigate(item.path)}
                  >
                    <div className="flex items-center gap-3">
                      <div
                        className={`p-1.5 rounded-lg bg-surface-secondary dark:bg-surface/20 ${item.color} bg-opacity-20`}
                      >
                        <item.icon size={16} />
                      </div>
                      <span className="font-medium text-sm text-foreground dark:text-zinc-200 truncate max-w-[100px] lg:max-w-none">
                        {item.label}
                      </span>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="text-base font-bold text-foreground dark:text-zinc-200">
                        {item.count}
                      </span>
                      <FaChevronDown
                        className="text-muted dark:text-zinc-500 -rotate-90"
                        size={10}
                      />
                    </div>
                  </button>
                ))}
              </Card.Content>
            </Card>
          </motion.div>

          {/* Content Download */}
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.5 }}
            className="md:col-span-1"
            onAnimationComplete={() => setIsAnimating(false)}
          >
            <ContentDownloadCard />
          </motion.div>
        </div>

        {/* --- Modals --- */}

        {/* Launch Failed */}
        <UnifiedModal
          isOpen={launchFailedDisclosure.isOpen}
          onOpenChange={launchFailedDisclosure.setOpen}
          type="error"
          title={t("launcherpage.launch.failed.title")}
          footer={
            <>
              {launchErrorCode === "ERR_GAME_ALREADY_RUNNING" && (
                <Button
                  onPress={handleLaunchFailedForceRun}
                  variant={"primary"}
                  className={cn(
                    "rounded-full",
                    "bg-warning text-warning-foreground",
                    "text-warning-foreground font-bold",
                  )}
                >
                  {t("launcherpage.launch.force_run_button")}
                </Button>
              )}
              <Button
                onPress={launchFailedDisclosure.close}
                variant={"danger"}
                className={cn(
                  "rounded-full",
                  "font-bold shadow-lg shadow-rose-500/20",
                )}
              >
                {t("launcherpage.launch.failed.close_button")}
              </Button>
            </>
          }
        >
          <div className="flex flex-col items-center gap-3 text-center px-2">
            <p className="text-lg font-semibold text-foreground dark:text-zinc-100">
              {launchErrorMessage}
            </p>
            <p className="text-sm leading-6 text-foreground dark:text-zinc-300 max-w-[520px]">
              {t("launcherpage.launch.failed.content") as unknown as string}
            </p>
            {launchErrorCode && (
              <Chip
                size="sm"
                variant="soft"
                color={"default"}
                className={"font-mono tracking-wide"}
              >
                <Chip.Label>{launchErrorCode}</Chip.Label>
              </Chip>
            )}
          </div>
        </UnifiedModal>

        {/* GameInput Installing */}
        <UnifiedModal
          isOpen={gameInputInstallingDisclosure.isOpen}
          onOpenChange={gameInputInstallingDisclosure.setOpen}
          type="success"
          title={t("launcherpage.gameinput.installing.title")}
          hideCloseButton
          icon={<FaDownload className="w-6 h-6" />}
        >
          <>
            <p className="text-foreground dark:text-zinc-300 font-medium">
              {t("launcherpage.gameinput.installing.body")}
            </p>
            <div className="mt-4">
              {giTotal > 0 ? (
                <div className="flex flex-col gap-2">
                  <div className="flex justify-between text-sm font-bold text-foreground dark:text-zinc-300">
                    <span>
                      {Math.min(
                        100,
                        Math.floor((giDownloaded / giTotal) * 100),
                      )}
                      %
                    </span>
                    <span className="font-mono">
                      {(giDownloaded / 1024 / 1024).toFixed(1)} /{" "}
                      {(giTotal / 1024 / 1024).toFixed(1)} MB
                    </span>
                  </div>
                  <ProgressBar
                    aria-label="Downloading"
                    value={(giDownloaded / giTotal) * 100}
                    size="md"
                    color={"accent"}
                  >
                    <ProgressBar.Track>
                      <ProgressBar.Fill
                        className={"bg-brand-500 hover:bg-brand-500"}
                      />
                    </ProgressBar.Track>
                  </ProgressBar>
                </div>
              ) : (
                <div className="flex items-center gap-3 text-foreground dark:text-zinc-300">
                  <Spinner size="sm" color={"accent"} />
                  <span>
                    {t("launcherpage.gameinput.installing.preparing")}
                  </span>
                </div>
              )}
            </div>
          </>
        </UnifiedModal>

        {/* GameInput Missing */}
        <UnifiedModal
          isOpen={gameInputMissingDisclosure.isOpen}
          onOpenChange={gameInputMissingDisclosure.setOpen}
          type="warning"
          title={t("launcherpage.gameinput.missing.title")}
          footer={
            <>
              <Button
                onPress={() => Window.Close()}
                variant={"ghost"}
                className={cn(
                  "rounded-full",
                  "text-rose-700 dark:text-rose-300",
                )}
              >
                {t("common.quit_launcher")}
              </Button>
              <Button
                onPress={handleGameInputInstall}
                variant={"primary"}
                className={cn(
                  "rounded-full",
                  "bg-warning text-warning-foreground",
                  "text-warning-foreground font-bold",
                )}
              >
                {t("launcherpage.gameinput.missing.install_now")}
              </Button>
            </>
          }
        >
          <p className="text-foreground dark:text-zinc-300 font-medium">
            {t("launcherpage.gameinput.missing.body")}
          </p>
        </UnifiedModal>

        {/* VCRuntime Installing */}
        <UnifiedModal
          isOpen={vcRuntimeInstallingDisclosure.isOpen}
          onOpenChange={vcRuntimeInstallingDisclosure.setOpen}
          type="success"
          title={t("launcherpage.vcruntime.installing.title")}
          hideCloseButton
          icon={<FaDownload className="w-6 h-6" />}
        >
          <>
            <p className="text-foreground dark:text-zinc-300 font-medium">
              {t("launcherpage.vcruntime.installing.body")}
            </p>
            <div className="mt-4">
              {vcTotal > 0 ? (
                <div className="flex flex-col gap-2">
                  <div className="flex justify-between text-sm font-bold text-foreground dark:text-zinc-300">
                    <span>
                      {Math.min(
                        100,
                        Math.floor((vcDownloaded / vcTotal) * 100),
                      )}
                      %
                    </span>
                    <span className="font-mono">
                      {(vcDownloaded / 1024 / 1024).toFixed(1)} /{" "}
                      {(vcTotal / 1024 / 1024).toFixed(1)} MB
                    </span>
                  </div>
                  <ProgressBar
                    aria-label="Downloading"
                    value={(vcDownloaded / vcTotal) * 100}
                    size="md"
                    color={"accent"}
                  >
                    <ProgressBar.Track>
                      <ProgressBar.Fill
                        className={"bg-brand-500 hover:bg-brand-500"}
                      />
                    </ProgressBar.Track>
                  </ProgressBar>
                </div>
              ) : (
                <div className="flex items-center gap-3 text-foreground dark:text-zinc-300">
                  <Spinner size="sm" color={"accent"} />
                  <span>
                    {t("launcherpage.vcruntime.installing.preparing")}
                  </span>
                </div>
              )}
            </div>
          </>
        </UnifiedModal>

        {/* VCRuntime Missing */}
        <UnifiedModal
          isOpen={vcRuntimeMissingDisclosure.isOpen}
          onOpenChange={vcRuntimeMissingDisclosure.setOpen}
          type="warning"
          title={t("launcherpage.vcruntime.missing.title")}
          footer={
            <>
              <Button
                onPress={() => Window.Close()}
                variant={"ghost"}
                className={cn(
                  "rounded-full",
                  "text-rose-700 dark:text-rose-300",
                )}
              >
                {t("common.quit_launcher")}
              </Button>
              <Button
                onPress={handleVcRuntimeInstall}
                variant={"primary"}
                className={cn(
                  "rounded-full",
                  "bg-warning text-warning-foreground",
                  "text-warning-foreground font-bold",
                )}
              >
                {t("launcherpage.vcruntime.missing.install_now")}
              </Button>
            </>
          }
        >
          <p className="text-foreground dark:text-zinc-300 font-medium">
            {t("launcherpage.vcruntime.missing.body")}
          </p>
        </UnifiedModal>

        {/* Gaming Services Missing */}
        <UnifiedModal
          isOpen={gamingServicesMissingDisclosure.isOpen}
          onOpenChange={gamingServicesMissingDisclosure.setOpen}
          type="warning"
          title={t("launcherpage.gs.missing.title")}
          icon={<FaWindows className="w-6 h-6" />}
          footer={
            <>
              <Button
                onPress={() => Window.Close()}
                variant={"ghost"}
                className={cn(
                  "rounded-full",
                  "text-rose-700 dark:text-rose-300",
                )}
              >
                {t("common.quit_launcher")}
              </Button>
              <Button
                onPress={handleIgnoreGamingServices}
                variant={"secondary"}
                className={"rounded-full"}
              >
                {t("launcherpage.gs.missing.ignore_forever")}
              </Button>
              <Button
                {...warningConfirmButtonProps}
                onPress={() => handleGamingServicesInstall(Browser.OpenURL)}
                variant={"secondary"}
                className={"rounded-full"}
              >
                {t("launcherpage.gs.missing.open_store")}
              </Button>
            </>
          }
        >
          <p className="text-foreground dark:text-zinc-300 font-medium">
            {t("launcherpage.gs.missing.body")}
          </p>
        </UnifiedModal>

        {/* Install Confirm (GameInput / GamingServices) */}
        <UnifiedModal
          isOpen={installConfirmDisclosure.isOpen}
          onOpenChange={handleInstallConfirmOpenChange}
          type="success"
          title={t("launcherpage.install_confirm.title")}
          icon={<FaDownload className="w-6 h-6" />}
          footer={
            <>
              <Button
                onPress={handleInstallConfirmContinue}
                variant={"ghost"}
                className={"rounded-full"}
              >
                {t("launcherpage.install_confirm.continue")}
              </Button>
              <Button
                onPress={handleInstallConfirmCheck}
                variant={"primary"}
                className={cn(
                  "rounded-full",
                  "bg-brand-500 hover:bg-brand-500 brand-primary-foreground font-bold shadow-lg shadow-brand-900/20",
                )}
              >
                {t("launcherpage.install_confirm.done_and_check")}
              </Button>
            </>
          }
        >
          <p className="text-foreground dark:text-zinc-300 font-medium">
            {t("launcherpage.install_confirm.body")}
          </p>
        </UnifiedModal>

        {/* VCRuntime Completing */}
        <UnifiedModal
          isOpen={vcRuntimeCompletingDisclosure.isOpen}
          onOpenChange={vcRuntimeCompletingDisclosure.setOpen}
          type="success"
          title={t("launcherpage.vcruntime.completing.title")}
          icon={<FaCogs className="w-6 h-6" />}
        >
          <p className="text-foreground dark:text-zinc-300 font-medium">
            {t("launcherpage.vcruntime.completing.body")}
          </p>
        </UnifiedModal>

        {/* MC Launch Loading */}
        <UnifiedModal
          isOpen={mcLaunchLoadingDisclosure.isOpen}
          onOpenChange={mcLaunchLoadingDisclosure.setOpen}
          type="success"
          title={t("launcherpage.mclaunch.loading.title")}
          hideCloseButton
          footer={
            <Button
              onPress={mcLaunchLoadingDisclosure.close}
              variant={"primary"}
              className={cn(
                "rounded-full",
                "bg-brand-500 hover:bg-brand-500 brand-primary-foreground font-bold shadow-lg shadow-brand-900/20",
              )}
            >
              {t("common.close")}
            </Button>
          }
        >
          <div className="flex flex-col gap-6">
            <div className="flex items-center gap-4">
              <Spinner size="lg" color={"accent"} />
              <div className="flex flex-col gap-1">
                <motion.p
                  className="text-foreground dark:text-zinc-300 font-medium"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  transition={{ duration: 0.25, delay: 0.1 }}
                >
                  {t("launcherpage.mclaunch.loading.body")}
                </motion.p>
                <div className="min-h-[24px] text-sm text-muted">
                  <AnimatePresence mode="wait">
                    <motion.span
                      key={`tip-${tipIndex}`}
                      initial={{ opacity: 0, y: 4 }}
                      animate={{ opacity: 1, y: 0 }}
                      exit={{ opacity: 0, y: -4 }}
                      transition={{ duration: 0.2 }}
                    >
                      {currentLaunchTip}
                    </motion.span>
                  </AnimatePresence>
                </div>
              </div>
            </div>
            <ProgressBar size="sm" isIndeterminate aria-label="Loading">
              <ProgressBar.Track>
                <ProgressBar.Fill
                  className={"bg-brand-500 hover:bg-brand-500"}
                />
              </ProgressBar.Track>
            </ProgressBar>
          </div>
        </UnifiedModal>

        {/* Shortcut Success */}
        <UnifiedModal
          isOpen={shortcutSuccessDisclosure.isOpen}
          onOpenChange={shortcutSuccessDisclosure.setOpen}
          type="success"
          title={t("launcherpage.shortcut.success.title")}
          footer={
            <Button
              onPress={shortcutSuccessDisclosure.close}
              variant={"primary"}
              className={cn(
                "rounded-full",
                "bg-brand-500 hover:bg-brand-500 brand-primary-foreground font-bold shadow-lg shadow-brand-900/20",
              )}
            >
              {t("common.close")}
            </Button>
          }
        >
          <p className="text-foreground dark:text-zinc-300 font-medium">
            {t("launcherpage.shortcut.success.body")}
          </p>
        </UnifiedModal>

        {/* Register Installing */}
        <UnifiedModal
          isOpen={registerInstallingDisclosure.isOpen}
          onOpenChange={registerInstallingDisclosure.setOpen}
          type={registerAction === "unregister" ? "warning" : "success"}
          title={
            registerAction === "unregister"
              ? t("versions.edit.unregister_progress.title")
              : t("launcherpage.register.installing.title")
          }
          icon={
            registerAction === "unregister" ? (
              <FaExclamationTriangle className="w-6 h-6" />
            ) : (
              <FaDownload className="w-6 h-6" />
            )
          }
        >
          <>
            <p className="text-foreground dark:text-zinc-300 font-medium mb-4">
              {registerAction === "unregister"
                ? t("versions.edit.unregister_progress.body")
                : t("launcherpage.register.installing.body")}
            </p>
            <ProgressBar
              size="sm"
              isIndeterminate
              aria-label={
                registerAction === "unregister"
                  ? "Unregistering"
                  : "Registering"
              }
            >
              <ProgressBar.Track>
                <ProgressBar.Fill
                  className={
                    registerAction === "unregister"
                      ? "bg-amber-500 hover:bg-amber-500"
                      : "bg-brand-500 hover:bg-brand-500"
                  }
                />
              </ProgressBar.Track>
            </ProgressBar>
          </>
        </UnifiedModal>

        {/* Register Success */}
        <UnifiedModal
          isOpen={registerSuccessDisclosure.isOpen}
          onOpenChange={handleRegisterSuccessOpenChange}
          type="success"
          title={t("launcherpage.register.success.title")}
          footer={
            <Button
              onPress={registerSuccessDisclosure.close}
              variant={"primary"}
              className={cn(
                "rounded-full",
                "bg-brand-500 hover:bg-brand-500 brand-primary-foreground font-bold shadow-lg shadow-brand-900/20",
              )}
            >
              {t("common.close")}
            </Button>
          }
        >
          <p className="text-foreground dark:text-zinc-300 font-medium">
            {t("launcherpage.register.success.body")}
          </p>
        </UnifiedModal>

        {/* Register Failed */}
        <UnifiedModal
          isOpen={registerFailedDisclosure.isOpen}
          onOpenChange={registerFailedDisclosure.setOpen}
          type="error"
          title={t("launcherpage.register.failed.title")}
          confirmText={t("common.close")}
          onConfirm={registerFailedDisclosure.close}
        >
          <div className="p-4 rounded-2xl bg-rose-50 dark:bg-rose-500/10 border border-rose-100 dark:border-rose-500/20 text-rose-600 dark:text-rose-400">
            <p className="font-medium text-center">
              {(() => {
                const key = `errors.${launchErrorCode}`;
                const translated = t(key) as unknown as string;
                if (launchErrorCode && translated && translated !== key)
                  return translated;
                return t(
                  "launcherpage.register.failed.body",
                ) as unknown as string;
              })()}
            </p>
          </div>
        </UnifiedModal>
      </PageContainer>
    </>
  );
};
