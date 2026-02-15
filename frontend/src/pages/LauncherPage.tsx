import { useEffect, useMemo } from "react";
import {
  Button,
  Card,
  CardHeader,
  CardBody,
  Dropdown,
  DropdownTrigger,
  DropdownMenu,
  DropdownItem,
  Input,
  Chip,
  Progress,
  Spinner,
} from "@heroui/react";
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
} from "react-icons/fa";
import { ModCard } from "@/components/ModdedCard";
import { ContentDownloadCard } from "@/components/ContentDownloadCard";
import { Window, Browser } from "@wailsio/runtime";
import { motion, AnimatePresence } from "framer-motion";
import { UnifiedModal } from "@/components/UnifiedModal";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { cn } from "@/utils/cn";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { useLauncher } from "@/hooks/useLauncher";

export const LauncherPage = (args: any) => {
  const { t } = useTranslation();

  const {
    // State
    isAnimating,
    setIsAnimating,
    currentVersion,
    displayName,
    localVersionMap,
    launchErrorCode,
    contentCounts,
    giTotal,
    giDownloaded,
    vcTotal,
    vcDownloaded,
    logoDataUrl,
    versionQuery,
    setVersionQuery,
    logoByName,
    isLoadingVersions,
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
  } = useLauncher(args);

  const launchTips = useMemo(
    () => [
      t("launcherpage.tip.choose_version_dropdown") as unknown as string,
      t("launcherpage.tip.open_version_settings_gear") as unknown as string,
      t("launcherpage.tip.mods_import_button") as unknown as string,
      t("launcherpage.tip.file_manager_pick") as unknown as string,
      t("launcherpage.tip.download_versions") as unknown as string,
      t("launcherpage.tip.content_counts_card") as unknown as string,
      t("launcherpage.tip.settings_base_root") as unknown as string,
      t("launcherpage.tip.directory_write_check") as unknown as string,
      t("launcherpage.tip.general") as unknown as string,
    ],
    [t],
  );

  useEffect(() => {
    startTipTimer(launchTips.length);
    return () => {
      stopTipTimer();
    };
  }, [launchTips.length, startTipTimer, stopTipTimer]);

  const worldsLabel = t("content.count.worlds") as string;
  const resourceLabel = t("content.count.resource_packs") as string;
  const behaviorLabel = t("content.count.behavior_packs") as string;

  const versionMenuItems = useMemo(
    () => buildVersionMenuItems(t("common.empty") as string),
    [buildVersionMenuItems, t],
  );

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
            <CardBody className="p-6 relative flex flex-col gap-6">
              {/* Main Layout */}
              <div className="flex flex-col md:flex-row md:items-center justify-between gap-6">
                {/* Left: Title & Info */}
                <div className="flex flex-col gap-1 min-w-0">
                  <div className="flex items-center gap-3">
                    <motion.h1
                      className="text-4xl sm:text-5xl font-black tracking-tight text-transparent bg-clip-text bg-gradient-to-r from-primary-600 to-primary-400 dark:from-primary-400 dark:to-primary-600 truncate pb-2"
                      initial={{ opacity: 0, x: -20 }}
                      animate={{ opacity: 1, x: 0 }}
                      transition={{ delay: 0.2 }}
                    >
                      Minecraft
                    </motion.h1>
                    {localVersionMap.get(currentVersion)?.isRegistered && (
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
                          variant="flat"
                          color="success"
                          classNames={{
                            base: "bg-primary-500/10 border border-primary-500/20 hidden sm:flex",
                            content:
                              "font-semibold text-primary-600 dark:text-primary-500",
                          }}
                        >
                          {t("launcherpage.registered_tip")}
                        </Chip>
                      </motion.div>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="text-lg sm:text-xl font-medium text-default-500 dark:text-zinc-400">
                      {t("launcherpage.edition")}
                    </span>
                  </div>
                </div>

                {/* Right: Actions */}
                <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-3 shrink-0">
                  {/* Version Selector */}
                  <div className="flex items-center gap-3 p-1.5 rounded-2xl">
                    <Dropdown
                      placement="bottom-end"
                      classNames={COMPONENT_STYLES.dropdown}
                    >
                      <DropdownTrigger>
                        <Button
                          variant="light"
                          className="h-12 px-3 rounded-xl data-[hover=true]:bg-default-200/50 dark:data-[hover=true]:bg-white/5"
                        >
                          <div className="flex items-center gap-3 text-left">
                            <div className="w-8 h-8 rounded-lg bg-default-200/50 dark:bg-white/10 flex items-center justify-center overflow-hidden shadow-sm">
                              {logoDataUrl ? (
                                <img
                                  src={logoDataUrl}
                                  alt="logo"
                                  className="w-full h-full object-cover"
                                />
                              ) : (
                                <span className="text-base font-bold text-default-500 dark:text-zinc-400">
                                  M
                                </span>
                              )}
                            </div>
                            <div className="flex flex-col hidden lg:flex">
                              <span className="text-xs text-default-500 dark:text-zinc-400 font-medium">
                                {t("launcherpage.currentVersion")}
                              </span>
                              <span className="text-sm font-bold text-default-900 dark:text-white leading-tight max-w-[120px] truncate">
                                {displayName ||
                                  t("launcherpage.currentVersion_none")}
                              </span>
                            </div>
                            <span className="text-sm font-bold text-default-900 dark:text-white leading-tight max-w-[120px] truncate lg:hidden">
                              {displayName ||
                                t("launcherpage.currentVersion_none")}
                            </span>
                            <FaChevronDown
                              className="text-default-400 dark:text-zinc-300 ml-1"
                              size={12}
                            />
                          </div>
                        </Button>
                      </DropdownTrigger>
                      <DropdownMenu
                        aria-label="Version Selection"
                        selectionMode="single"
                        selectedKeys={
                          new Set(currentVersion ? [currentVersion] : [])
                        }
                        className="max-h-[400px] overflow-y-auto no-scrollbar min-w-[300px]"
                        bottomContent={
                          isLoadingVersions ? (
                            <div className="p-2 flex justify-center items-center gap-2 text-default-400 text-xs border-t border-default-100 dark:border-white/5">
                              <Spinner size="sm" color="primary" />
                              <span>{t("common.loading")}</span>
                            </div>
                          ) : null
                        }
                        topContent={
                          <div className="p-3 border-b border-default-100 dark:border-default-50/10">
                            <Input
                              size="sm"
                              placeholder={t("launcherpage.search_versions")}
                              value={versionQuery}
                              onValueChange={setVersionQuery}
                              startContent={
                                <FaList className="text-default-400" />
                              }
                              classNames={COMPONENT_STYLES.input}
                            />
                            <Button
                              fullWidth
                              size="sm"
                              variant="flat"
                              className="mt-2"
                              onPress={() => navigate("/versions")}
                            >
                              {t("launcherpage.manage_versions")}
                            </Button>
                          </div>
                        }
                        items={versionMenuItems}
                        onSelectionChange={handleVersionSelect}
                      >
                        {(item: any) => (
                          <DropdownItem
                            key={item.key}
                            textValue={item.name}
                            description={item.version}
                            startContent={
                              <div className="w-8 h-8 shrink-0 rounded-lg bg-default-100 dark:bg-white/10 flex items-center justify-center overflow-hidden">
                                {(() => {
                                  const u =
                                    item.logo || logoByName.get(item.name);
                                  if (!u) ensureLogo(item.name);
                                  return u ? (
                                    <img
                                      src={u}
                                      className="w-full h-full object-cover"
                                    />
                                  ) : (
                                    <span className="text-sm font-bold text-default-500 dark:text-zinc-400">
                                      M
                                    </span>
                                  );
                                })()}
                              </div>
                            }
                          >
                            <div className="flex justify-between items-center gap-2">
                              <span className="font-semibold">{item.name}</span>
                              {item.isRegistered && (
                                <Chip
                                  size="sm"
                                  variant="flat"
                                  color="success"
                                  classNames={{
                                    base: "bg-primary-500/10 border border-primary-500/20 h-5 px-1",
                                    content:
                                      "text-primary-600 dark:text-primary-500 font-bold text-[10px]",
                                  }}
                                >
                                  {t("launcherpage.registered_tip")}
                                </Chip>
                              )}
                            </div>
                          </DropdownItem>
                        )}
                      </DropdownMenu>
                    </Dropdown>

                    <Dropdown classNames={COMPONENT_STYLES.dropdown}>
                      <DropdownTrigger>
                        <Button
                          isIconOnly
                          variant="light"
                          radius="full"
                          size="sm"
                          className="data-[hover=true]:bg-default-200/50 dark:data-[hover=true]:bg-white/5"
                        >
                          <FaCogs
                            size={18}
                            className="text-default-500 dark:text-zinc-400"
                          />
                        </Button>
                      </DropdownTrigger>
                      <DropdownMenu aria-label="Version Actions">
                        <DropdownItem
                          key="settings"
                          startContent={<FaCog />}
                          onPress={() => {
                            if (currentVersion) {
                              navigate("/versionSettings", {
                                state: { name: currentVersion, returnTo: "/" },
                              });
                            } else {
                              navigate("/versions");
                            }
                          }}
                        >
                          {t("launcherpage.go_version_settings")}
                        </DropdownItem>
                        <DropdownItem
                          key="shortcut"
                          startContent={<FaDesktop />}
                          onPress={doCreateShortcut}
                        >
                          {t("launcherpage.shortcut.create_button")}
                        </DropdownItem>
                        <DropdownItem
                          key="folder"
                          startContent={<FaFolderOpen />}
                          onPress={doOpenFolder}
                        >
                          {t("launcherpage.open_exe_dir")}
                        </DropdownItem>
                        <DropdownItem
                          key="register"
                          startContent={<FaWindows />}
                          onPress={doRegister}
                        >
                          {t("launcherpage.register_system_button")}
                        </DropdownItem>
                      </DropdownMenu>
                    </Dropdown>
                  </div>
                  {/* Launch Button */}
                  <motion.div
                    whileHover={{ scale: 1.02 }}
                    whileTap={{ scale: 0.98 }}
                  >
                    <Button
                      size="lg"
                      className="h-14 px-8 text-lg font-bold text-white shadow-primary-900/20 shadow-lg bg-primary-600 hover:bg-primary-500 rounded-2xl w-full sm:w-auto"
                      startContent={<FaRocket className="mb-0.5" />}
                      onPress={doLaunch}
                      isLoading={mcLaunchLoadingDisclosure.isOpen}
                    >
                      {t("launcherpage.launch_button")}
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
                      className="text-sm text-default-500 dark:text-zinc-400 font-medium truncate absolute inset-0"
                    >
                      {launchTips[tipIndex]}
                    </motion.div>
                  </AnimatePresence>
                </div>
              </div>
            </CardBody>
          </Card>
        </motion.div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 lg:gap-6 items-stretch">
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
              )}
            >
              <CardHeader className="px-5 py-3 border-b border-default-100 dark:border-white/5 flex justify-between items-center">
                <div className="flex items-center gap-2">
                  <div className="p-1.5 rounded-lg bg-pink-500/10 text-pink-600 dark:text-pink-400">
                    <FaCube size={16} />
                  </div>
                  <h3 className="text-base font-bold text-default-800 dark:text-zinc-100">
                    {t("launcherpage.content_manage")}
                  </h3>
                </div>
                <Button
                  size="sm"
                  variant="light"
                  className="text-xs text-default-500 dark:text-zinc-400 data-[hover=true]:text-default-800 dark:data-[hover=true]:text-zinc-200"
                  endContent={<FaArrowRight size={10} />}
                  onPress={() => navigate("/content")}
                >
                  {t("common.view_all")}
                </Button>
              </CardHeader>
              <CardBody className="p-3 gap-2 relative">
                {[
                  {
                    label: worldsLabel,
                    count: contentCounts.worlds,
                    icon: FaGlobe,
                    path: "/content/worlds",
                    color: "text-blue-500",
                  },
                  {
                    label: resourceLabel,
                    count: contentCounts.resourcePacks,
                    icon: FaImage,
                    path: "/content/resourcePacks",
                    color: "text-purple-500",
                  },
                  {
                    label: behaviorLabel,
                    count: contentCounts.behaviorPacks,
                    icon: FaCogs,
                    path: "/content/behaviorPacks",
                    color: "text-orange-500",
                  },
                ].map((item, idx) => (
                  <div
                    key={idx}
                    className="group/item flex items-center justify-between p-2 rounded-xl hover:bg-default-200/50 dark:hover:bg-zinc-700/50 cursor-pointer transition-all duration-200"
                    onClick={() => navigate(item.path)}
                  >
                    <div className="flex items-center gap-3">
                      <div
                        className={`p-1.5 rounded-lg bg-default-100 dark:bg-default-50/20 ${item.color} bg-opacity-20`}
                      >
                        <item.icon size={16} />
                      </div>
                      <span className="font-medium text-sm text-default-600 dark:text-zinc-200 truncate max-w-[100px] lg:max-w-none">
                        {item.label}
                      </span>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="text-base font-bold text-default-800 dark:text-zinc-200">
                        {item.count}
                      </span>
                      <FaChevronDown
                        className="text-default-300 dark:text-zinc-500 -rotate-90"
                        size={10}
                      />
                    </div>
                  </div>
                ))}
              </CardBody>
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
          onOpenChange={launchFailedDisclosure.onOpenChange}
          type="error"
          title={t("launcherpage.launch.failed.title")}
          footer={
            <>
              {launchErrorCode === "ERR_GAME_ALREADY_RUNNING" && (
                <Button
                  color="warning"
                  radius="full"
                  className="text-white font-bold"
                  onPress={handleLaunchFailedForceRun}
                >
                  {t("launcherpage.launch.force_run_button")}
                </Button>
              )}
              <Button
                color="danger"
                variant="solid"
                radius="full"
                className="font-bold shadow-lg shadow-danger-500/20"
                onPress={launchFailedDisclosure.onClose}
              >
                {t("launcherpage.launch.failed.close_button")}
              </Button>
            </>
          }
        >
          <div className="p-4 rounded-2xl bg-danger-50 dark:bg-danger-500/10 border border-danger-100 dark:border-danger-500/20 text-danger-600 dark:text-danger-400">
            <p className="font-medium text-center">
              {(() => {
                const key = `errors.${launchErrorCode}`;
                const translated = t(key) as unknown as string;
                if (launchErrorCode && translated && translated !== key)
                  return translated;
                return t(
                  "launcherpage.launch.failed.content",
                ) as unknown as string;
              })()}
            </p>
          </div>
        </UnifiedModal>

        {/* GameInput Installing */}
        <UnifiedModal
          isOpen={gameInputInstallingDisclosure.isOpen}
          onOpenChange={gameInputInstallingDisclosure.onOpenChange}
          type="success"
          title={t("launcherpage.gameinput.installing.title")}
          hideCloseButton
          icon={<FaDownload className="w-6 h-6 text-primary-500" />}
        >
          <>
            <p className="text-default-600 dark:text-zinc-300 font-medium">
              {t("launcherpage.gameinput.installing.body")}
            </p>
            <div className="mt-4">
              {giTotal > 0 ? (
                <div className="flex flex-col gap-2">
                  <div className="flex justify-between text-small font-bold text-default-500 dark:text-zinc-400">
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
                  <Progress
                    aria-label="Downloading"
                    value={(giDownloaded / giTotal) * 100}
                    color="primary"
                    size="md"
                    classNames={{
                      indicator: "bg-primary-600 hover:bg-primary-500",
                    }}
                  />
                </div>
              ) : (
                <div className="flex items-center gap-3 text-default-500 dark:text-zinc-400">
                  <Spinner size="sm" color="primary" />
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
          onOpenChange={gameInputMissingDisclosure.onOpenChange}
          type="warning"
          title={t("launcherpage.gameinput.missing.title")}
          footer={
            <>
              <Button
                color="danger"
                variant="light"
                radius="full"
                onPress={() => Window.Close()}
              >
                {t("common.quit_launcher")}
              </Button>
              <Button
                color="warning"
                radius="full"
                className="text-white font-bold"
                onPress={handleGameInputInstall}
              >
                {t("launcherpage.gameinput.missing.install_now")}
              </Button>
            </>
          }
        >
          <p className="text-default-600 dark:text-zinc-300 font-medium">
            {t("launcherpage.gameinput.missing.body")}
          </p>
        </UnifiedModal>

        {/* VCRuntime Installing */}
        <UnifiedModal
          isOpen={vcRuntimeInstallingDisclosure.isOpen}
          onOpenChange={vcRuntimeInstallingDisclosure.onOpenChange}
          type="success"
          title={t("launcherpage.vcruntime.installing.title")}
          hideCloseButton
          icon={<FaDownload className="w-6 h-6 text-primary-500" />}
        >
          <>
            <p className="text-default-600 dark:text-zinc-300 font-medium">
              {t("launcherpage.vcruntime.installing.body")}
            </p>
            <div className="mt-4">
              {vcTotal > 0 ? (
                <div className="flex flex-col gap-2">
                  <div className="flex justify-between text-small font-bold text-default-500 dark:text-zinc-400">
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
                  <Progress
                    aria-label="Downloading"
                    value={(vcDownloaded / vcTotal) * 100}
                    color="primary"
                    size="md"
                    classNames={{
                      indicator: "bg-primary-600 hover:bg-primary-500",
                    }}
                  />
                </div>
              ) : (
                <div className="flex items-center gap-3 text-default-500 dark:text-zinc-400">
                  <Spinner size="sm" color="primary" />
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
          onOpenChange={vcRuntimeMissingDisclosure.onOpenChange}
          type="warning"
          title={t("launcherpage.vcruntime.missing.title")}
          footer={
            <>
              <Button
                color="danger"
                variant="light"
                radius="full"
                onPress={() => Window.Close()}
              >
                {t("common.quit_launcher")}
              </Button>
              <Button
                color="warning"
                radius="full"
                className="text-white font-bold"
                onPress={handleVcRuntimeInstall}
              >
                {t("launcherpage.vcruntime.missing.install_now")}
              </Button>
            </>
          }
        >
          <p className="text-default-600 dark:text-zinc-300 font-medium">
            {t("launcherpage.vcruntime.missing.body")}
          </p>
        </UnifiedModal>

        {/* Gaming Services Missing */}
        <UnifiedModal
          isOpen={gamingServicesMissingDisclosure.isOpen}
          onOpenChange={gamingServicesMissingDisclosure.onOpenChange}
          type="warning"
          title={t("launcherpage.gs.missing.title")}
          icon={<FaWindows className="w-6 h-6 text-warning-500" />}
          footer={
            <>
              <Button
                color="danger"
                variant="light"
                radius="full"
                onPress={() => Window.Close()}
              >
                {t("common.quit_launcher")}
              </Button>
              <Button
                color="default"
                variant="flat"
                radius="full"
                onPress={handleIgnoreGamingServices}
              >
                {t("launcherpage.gs.missing.ignore_forever")}
              </Button>
              <Button
                color="primary"
                radius="full"
                className="bg-primary-600 hover:bg-primary-500 text-white font-bold shadow-lg shadow-primary-900/20"
                onPress={() => handleGamingServicesInstall(Browser.OpenURL)}
              >
                {t("launcherpage.gs.missing.open_store")}
              </Button>
            </>
          }
        >
          <p className="text-default-600 dark:text-zinc-300 font-medium">
            {t("launcherpage.gs.missing.body")}
          </p>
        </UnifiedModal>

        {/* Install Confirm (GameInput / GamingServices) */}
        <UnifiedModal
          isOpen={installConfirmDisclosure.isOpen}
          onOpenChange={handleInstallConfirmOpenChange}
          type="success"
          title={t("launcherpage.install_confirm.title")}
          icon={<FaDownload className="w-6 h-6 text-primary-500" />}
          footer={
            <>
              <Button
                color="default"
                variant="light"
                radius="full"
                onPress={handleInstallConfirmContinue}
              >
                {t("launcherpage.install_confirm.continue")}
              </Button>
              <Button
                color="primary"
                radius="full"
                className="bg-primary-600 hover:bg-primary-500 text-white font-bold shadow-lg shadow-primary-900/20"
                onPress={handleInstallConfirmCheck}
              >
                {t("launcherpage.install_confirm.done_and_check")}
              </Button>
            </>
          }
        >
          <p className="text-default-600 dark:text-zinc-300 font-medium">
            {t("launcherpage.install_confirm.body")}
          </p>
        </UnifiedModal>

        {/* VCRuntime Completing */}
        <UnifiedModal
          isOpen={vcRuntimeCompletingDisclosure.isOpen}
          onOpenChange={vcRuntimeCompletingDisclosure.onOpenChange}
          type="success"
          title={t("launcherpage.vcruntime.completing.title")}
          icon={<FaCogs className="w-6 h-6 text-primary-500" />}
        >
          <p className="text-default-600 dark:text-zinc-300 font-medium">
            {t("launcherpage.vcruntime.completing.body")}
          </p>
        </UnifiedModal>

        {/* MC Launch Loading */}
        <UnifiedModal
          isOpen={mcLaunchLoadingDisclosure.isOpen}
          onOpenChange={mcLaunchLoadingDisclosure.onOpenChange}
          type="success"
          title={t("launcherpage.mclaunch.loading.title")}
          hideCloseButton
          footer={
            <Button
              color="primary"
              radius="full"
              className="bg-primary-600 hover:bg-primary-500 text-white font-bold shadow-lg shadow-primary-900/20"
              onPress={mcLaunchLoadingDisclosure.onClose}
            >
              {t("common.close")}
            </Button>
          }
        >
          <div className="flex flex-col gap-6">
            <div className="flex items-center gap-4">
              <Spinner
                size="lg"
                color="success"
                classNames={{
                  circle1: "border-b-primary-500",
                  circle2: "border-b-teal-500",
                }}
              />
              <div className="flex flex-col gap-1">
                <motion.p
                  className="text-default-600 dark:text-zinc-300 font-medium"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  transition={{ duration: 0.25, delay: 0.1 }}
                >
                  {t("launcherpage.mclaunch.loading.body")}
                </motion.p>
                <div className="min-h-[24px] text-sm text-default-400">
                  <AnimatePresence mode="wait">
                    <motion.span
                      key={`tip-${tipIndex}`}
                      initial={{ opacity: 0, y: 4 }}
                      animate={{ opacity: 1, y: 0 }}
                      exit={{ opacity: 0, y: -4 }}
                      transition={{ duration: 0.2 }}
                    >
                      {launchTips[tipIndex]}
                    </motion.span>
                  </AnimatePresence>
                </div>
              </div>
            </div>
            <Progress
              size="sm"
              isIndeterminate
              aria-label="Loading"
              classNames={{ indicator: "bg-primary-600 hover:bg-primary-500" }}
            />
          </div>
        </UnifiedModal>

        {/* Shortcut Success */}
        <UnifiedModal
          isOpen={shortcutSuccessDisclosure.isOpen}
          onOpenChange={shortcutSuccessDisclosure.onOpenChange}
          type="success"
          title={t("launcherpage.shortcut.success.title")}
          footer={
            <Button
              color="primary"
              radius="full"
              className="bg-primary-600 hover:bg-primary-500 text-white font-bold shadow-lg shadow-primary-900/20"
              onPress={shortcutSuccessDisclosure.onClose}
            >
              {t("common.close")}
            </Button>
          }
        >
          <p className="text-default-600 dark:text-zinc-300 font-medium">
            {t("launcherpage.shortcut.success.body")}
          </p>
        </UnifiedModal>

        {/* Register Installing */}
        <UnifiedModal
          isOpen={registerInstallingDisclosure.isOpen}
          onOpenChange={registerInstallingDisclosure.onOpenChange}
          type="success"
          title={t("launcherpage.register.installing.title")}
          icon={<FaDownload className="w-6 h-6 text-primary-500" />}
        >
          <>
            <p className="text-default-600 dark:text-zinc-300 font-medium mb-4">
              {t("launcherpage.register.installing.body")}
            </p>
            <Progress
              size="sm"
              isIndeterminate
              aria-label="Registering"
              classNames={{ indicator: "bg-primary-600 hover:bg-primary-500" }}
            />
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
              color="primary"
              radius="full"
              className="bg-primary-600 hover:bg-primary-500 text-white font-bold shadow-lg shadow-primary-900/20"
              onPress={registerSuccessDisclosure.onClose}
            >
              {t("common.close")}
            </Button>
          }
        >
          <p className="text-default-600 dark:text-zinc-300 font-medium">
            {t("launcherpage.register.success.body")}
          </p>
        </UnifiedModal>

        {/* Register Failed */}
        <UnifiedModal
          isOpen={registerFailedDisclosure.isOpen}
          onOpenChange={registerFailedDisclosure.onOpenChange}
          type="error"
          title={t("launcherpage.register.failed.title")}
          footer={
            <Button
              color="primary"
              radius="full"
              className="bg-primary-600 hover:bg-primary-500 text-white font-bold shadow-lg shadow-primary-900/20"
              onPress={registerFailedDisclosure.onClose}
            >
              {t("common.close")}
            </Button>
          }
        >
          <div className="p-4 rounded-2xl bg-danger-50 dark:bg-danger-500/10 border border-danger-100 dark:border-danger-500/20 text-danger-600 dark:text-danger-400">
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

        {/* GDK Missing */}
        <UnifiedModal
          isOpen={gdkMissingDisclosure.isOpen}
          onOpenChange={gdkMissingDisclosure.onOpenChange}
          type="warning"
          title={t("launcherpage.gdk_missing.title")}
          footer={
            <>
              <Button
                variant="light"
                radius="full"
                onPress={gdkMissingDisclosure.onClose}
              >
                {t("common.cancel")}
              </Button>
              <Button
                color="warning"
                radius="full"
                className="text-white"
                onPress={handleGdkMissingGoSettings}
              >
                {t("launcherpage.gdk_missing.go_settings")}
              </Button>
            </>
          }
        >
          <p className="text-default-600 dark:text-zinc-300 font-medium">
            {t("launcherpage.gdk_missing.body")}
          </p>
        </UnifiedModal>
      </PageContainer>
    </>
  );
};
