import { openDirectory } from "@/utils/explorer";
import { ModalAction, ModalPanel, ModalDescription, ModalNotice } from "@/components/ModalPrimitives";
import {
  Button,
  Card,
  Chip,
  Dropdown,
  Input,
  InputGroup,
  Label,
  ListBox,
  NumberField,
  ProgressBar,
  Select,
  Separator,
  Slider,
  Spinner,
  Switch,
  Tabs,
  TextField,
  Tooltip,
  toast,
} from "@heroui/react";

import { cn } from "@/utils/cn";

import React from "react";
import { useTranslation } from "react-i18next";
import { motion, AnimatePresence } from "framer-motion";

import { useTheme } from "next-themes";
import { RxUpdate, RxDesktop } from "react-icons/rx";
import {
  FaGithub,
  FaDiscord,
  FaDownload, FaList
} from "react-icons/fa";
import {
  LuHardDrive,
  LuPalette,
  LuSun,
  LuMoon,
  LuMonitor,
  LuImage, LuLayers,
  LuShield
} from "react-icons/lu";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import {
  GetBaseRoot,
  SetBaseRoot,
  CanWriteToDir,
  SetDisableDiscordRPC,
  SetEnableBetaUpdates,
  ResetBaseRoot,
  InstallLip,
} from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import {
  GetInstallerDir,
  GetVersionsDir,
} from "bindings/github.com/liteldev/LeviLauncher/versionservice";
import { Browser, Dialogs } from "@wailsio/runtime";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { UnifiedModal } from "@/components/UnifiedModal";
import { PageHeader } from "@/components/PageHeader";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { THEMES, THEME_GROUPS } from "@/constants/themes";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { CustomColorPicker } from "@/components/CustomColorPicker";
import { BackgroundAppearanceSettings } from "@/components/BackgroundAppearanceSettings";
import { useSettings, ThemeMode } from "@/hooks/useSettings";

const normalizeHexColor = (
  value: string | undefined,
  fallback: string = "#10b981",
) => {
  if (!value) return fallback;
  const trimmed = value.trim();
  if (/^#[0-9A-Fa-f]{6}$/.test(trimmed)) return trimmed;
  if (/^[0-9A-Fa-f]{6}$/.test(trimmed)) return `#${trimmed}`;
  return fallback;
};

const getColorLuminance = (hexColor: string) => {
  const color = normalizeHexColor(hexColor, "#000000");
  const r = parseInt(color.slice(1, 3), 16);
  const g = parseInt(color.slice(3, 5), 16);
  const b = parseInt(color.slice(5, 7), 16);
  return (0.299 * r + 0.587 * g + 0.114 * b) / 255;
};

const AppearanceNumberField = ({ label, value, min, max, onChange }: {
  label: string;
  value: number;
  min: number;
  max: number;
  onChange: (value: number) => void;
}) => (
  <NumberField
    aria-label={label}
    value={value}
    minValue={min}
    maxValue={max}
    step={1}
    onChange={(next) => { if (Number.isFinite(next)) onChange(next); }}
  >
    <NumberField.Group className="h-8 min-h-8 rounded-lg bg-field">
      <NumberField.Input className="w-16 rounded-lg px-2 text-right text-sm tabular-nums focus-visible:outline-2 focus-visible:outline-focus" />
    </NumberField.Group>
  </NumberField>
);

const APPEARANCE_SECTIONS = ["theme", "layout", "wallpaper", "material"] as const;
type AppearanceSection = typeof APPEARANCE_SECTIONS[number];

export const SettingsPage: React.FC = () => {
  const { t, i18n } = useTranslation();
  const { theme, setTheme } = useTheme();
  const settings = useSettings(i18n);
  const {
    hasBackend,
    navigate,
    location,
    appVersion,
    checkingUpdate,
    updating,
    newVersion,
    hasUpdate,
    changelog,
    onCheckUpdate,
    onUpdate,
    langNames,
    selectedLang,
    setSelectedLang,
    languageChanged,
    setLanguageChanged,
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
    discordRpcEnabled,
    setDiscordRpcEnabled,
    enableBetaUpdates,
    setEnableBetaUpdates,
    clarityEnabled,
    setClarityEnabled,
    experimentalInstanceBackupEnabled,
    setExperimentalInstanceBackupEnabled,
    selectedTab,
    setSelectedTab,
    layoutMode,
    setLayoutMode,
    disableAnimations,
    setDisableAnimations,
    lightThemeColor,
    setLightThemeColor,
    darkThemeColor,
    setDarkThemeColor,
    lightCustomThemeColor,
    setLightCustomThemeColor,
    darkCustomThemeColor,
    setDarkCustomThemeColor,
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
    lipInstalled,
    lipVersion,
    lipLatestVersion,
    lipPath,
    lipUpToDate,
    lipStatusError,
    installingLip,
    setInstallingLip,
    cleaningLipCache,
    lipStatus,
    lipProgress,
    lipError,
    setLipError,
    lipProgressDisclosure,
    refreshLipStatus,
    cleanLipCache,
    resourceRulesInstalled,
    resourceRulesUpToDate,
    resourceRulesLocalSha,
    resourceRulesRemoteSha,
    resourceRulesError,
    resourceRulesChecking,
    resourceRulesUpdating,
    refreshResourceRulesStatus,
    onUpdateResourceRules,
    processModalOpen,
    setProcessModalOpen,
    processes,
    scanningProcesses,
    processError,
    terminatingProcess,
    refreshProcesses,
    handleKillProcess,
    handleKillAllProcesses,
    unsavedOpen,
    unsavedOnClose,
    unsavedOnOpenChange,
    pendingNavPath,
    resetOpen,
    resetOnOpen,
    resetOnOpenChange,
    resetOnClose,
  } = settings;
  const [pathError, setPathError] = React.useState("");
  const pageRef = React.useRef<HTMLDivElement>(null);
  const [settingsQuery, setSettingsQuery] = React.useState("");
  const [appearanceSection, setAppearanceSection] = React.useState<AppearanceSection>(() => {
    try {
      const saved = localStorage.getItem("app.appearanceSection") as AppearanceSection;
      if (APPEARANCE_SECTIONS.includes(saved)) return saved;
    } catch {}
    return "theme";
  });
  React.useEffect(() => {
    try { localStorage.setItem("app.appearanceSection", appearanceSection); } catch {}
  }, [appearanceSection]);
  const searchEntries: { tab: string; section?: AppearanceSection; keys: string[] }[] = [
    { tab: "general", keys: ["settings.body.paths.title", "settings.body.paths.base_root", "settings.body.language.name", "settings.discord_rpc.title"] },
    { tab: "personalization", section: "theme", keys: ["audit.usability.appearance_theme", "settings.appearance.theme_mode", "settings.appearance.theme_light", "settings.appearance.theme_dark", "settings.appearance.mode_config"] },
    { tab: "personalization", section: "layout", keys: ["audit.usability.appearance_layout", "settings.layout.title_navbar", "settings.appearance.disable_animations"] },
    { tab: "personalization", section: "wallpaper", keys: ["audit.usability.appearance_wallpaper", "settings.appearance.background_image", "settings.appearance.background_blur", "settings.appearance.background_brightness", "settings.appearance.background_opacity"] },
    { tab: "personalization", section: "material", keys: ["audit.usability.appearance_material"] },
    { tab: "components", keys: ["settings.tabs.components", "settings.lip.title", "settings.resource_rules.title"] },
    { tab: "others", keys: ["settings.process.title", "settings.experimental.title", "settings.experimental.instance_backup.title"] },
    { tab: "privacy", keys: ["settings.tabs.privacy", "settings.privacy.analytics.title"] },
    { tab: "updates", keys: ["settings.tabs.updates", "settings.beta_updates.title"] },
    { tab: "about", keys: ["settings.tabs.about"] },
  ];
  const searchResults = searchEntries.filter((entry) =>
    [t(`settings.tabs.${entry.tab}`), ...entry.keys.map((key) => t(key))]
      .join(" ").toLocaleLowerCase().includes(settingsQuery.trim().toLocaleLowerCase()),
  );
  const [killTarget, setKillTarget] = React.useState<{ pid?: number; label: string } | null>(null);
  const continuePendingNavigation = () => {
    unsavedOnClose();
    if (typeof pendingNavPath === "number") navigate(pendingNavPath);
    else if (pendingNavPath) navigate(pendingNavPath);
  };

  React.useEffect(() => {
    setPathError("");
    setBaseRootWritable(true);
  }, [newBaseRoot, setBaseRootWritable]);

  const persistBasePath = async (reset = false): Promise<boolean> => {
    if (savingBaseRoot) return false;
    setSavingBaseRoot(true);
    setPathError("");
    try {
      if (!reset && !(await CanWriteToDir(newBaseRoot))) {
        setBaseRootWritable(false);
        return false;
      }
      const error = reset ? await ResetBaseRoot() : await SetBaseRoot(newBaseRoot);
      if (error) throw new Error(error);
      const [root, installers, versions] = await Promise.all([
        GetBaseRoot(),
        GetInstallerDir(),
        GetVersionsDir(),
      ]);
      setBaseRoot(String(root || ""));
      setNewBaseRoot(String(root || ""));
      setInstallerDir(String(installers || ""));
      setVersionsDir(String(versions || ""));
      setBaseRootWritable(true);
      toast.success(
        t(reset ? "audit.mods.path_reset_success" : "audit.mods.path_save_success"),
      );
      return true;
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      setPathError(
        t(`errors.${message}`, {
          defaultValue: t("audit.mods.path_save_failed", { message }),
        }),
      );
      return false;
    } finally {
      setSavingBaseRoot(false);
    }
  };

  const [instanceBackupWarningOpen, setInstanceBackupWarningOpen] =
    React.useState(false);
  const [instanceBackupWarningCountdown, setInstanceBackupWarningCountdown] =
    React.useState(0);

  React.useEffect(() => {
    if (!instanceBackupWarningOpen) {
      setInstanceBackupWarningCountdown(0);
      return;
    }
    setInstanceBackupWarningCountdown(10);
    const timer = window.setInterval(() => {
      setInstanceBackupWarningCountdown((value) => (value > 0 ? value - 1 : 0));
    }, 1000);
    return () => window.clearInterval(timer);
  }, [instanceBackupWarningOpen]);

  const closeInstanceBackupWarning = React.useCallback(() => {
    setInstanceBackupWarningOpen(false);
  }, []);

  const confirmInstanceBackupWarning = React.useCallback(() => {
    if (instanceBackupWarningCountdown > 0) return;
    setExperimentalInstanceBackupEnabled(true);
    setInstanceBackupWarningOpen(false);
  }, [instanceBackupWarningCountdown, setExperimentalInstanceBackupEnabled]);

  const handleInstanceBackupExperimentalToggle = React.useCallback(
    (isSelected: boolean) => {
      if (!isSelected) {
        setExperimentalInstanceBackupEnabled(false);
        setInstanceBackupWarningOpen(false);
        return;
      }
      if (experimentalInstanceBackupEnabled) {
        return;
      }
      setInstanceBackupWarningOpen(true);
    },
    [experimentalInstanceBackupEnabled, setExperimentalInstanceBackupEnabled],
  );

  const activeCustomThemeColor = normalizeHexColor(
    themeSettingMode === "light" ? lightCustomThemeColor : darkCustomThemeColor,
  );
  const lipSummaryText = React.useMemo(() => {
    if (!lipInstalled) {
      return t("settings.lip.status.missing");
    }
    return t("settings.lip.version_label", {
      currentVersion: lipVersion || t("settings.lip.unknown_version"),
      latestVersion: lipLatestVersion || t("settings.lip.unknown_version"),
    });
  }, [lipInstalled, lipLatestVersion, lipVersion, t]);

  const customThemeIconColor =
    getColorLuminance(activeCustomThemeColor) > 0.6 ? "#111827" : "#ffffff";

  return (
    <PageContainer ref={pageRef} className="relative [&_.tabs__tab]:min-w-max [&_.tabs__tab]:whitespace-nowrap" animate={false}>
      <div className="flex flex-col gap-4">
        {/* Header Card */}
        <motion.div
          initial={{ opacity: 0, y: -20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.4 }}
        >
          <Card className={LAYOUT.GLASS_CARD.BASE}>
            <Card.Content className="p-6">
              <PageHeader
                title={t("settings.header.title")}
                description={t("settings.header.content")}
                endContent={
                  <TextField aria-label={t("audit.usability.search_settings")} value={settingsQuery} onChange={setSettingsQuery} className="w-full sm:w-64">
                    <InputGroup><InputGroup.Input placeholder={t("audit.usability.search_settings")} /></InputGroup>
                  </TextField>
                }
              />
              {settingsQuery.trim() && (
                <div className="mt-4 space-y-2">
                  <p role="status" className="text-sm text-muted">{t("audit.usability.search_count", { count: searchResults.length })}</p>
                  <div className="flex flex-wrap gap-2">
                    {searchResults.map((entry) => <Button key={entry.section || entry.tab} size="sm" variant="secondary" onPress={() => {
                      setSelectedTab(entry.tab);
                      if (entry.section) setAppearanceSection(entry.section);
                      setSettingsQuery("");
                      pageRef.current?.scrollTo({ top: 0 });
                    }}>{entry.section ? t(`audit.usability.appearance_${entry.section}`) : t(`settings.tabs.${entry.tab}`)}</Button>)}
                  </div>
                </div>
              )}
              <Tabs
                selectedKey={selectedTab}
                onSelectionChange={(k) => setSelectedTab(k as string)}
                variant="primary"
                className={"mt-4"}
              >
                <Tabs.ListContainer>
                  <Tabs.List
                    aria-label={t("settings.header.title")}
                    className={COMPONENT_STYLES.tabs.tabList}
                  >
                    <Tabs.Tab key="general" id={"general"}>
                      {t("settings.tabs.general")}
                      <Tabs.Indicator />
                    </Tabs.Tab>
                    <Tabs.Tab key="personalization" id={"personalization"}>
                      {t("settings.tabs.personalization")}
                      <Tabs.Indicator />
                    </Tabs.Tab>
                    <Tabs.Tab key="components" id={"components"}>
                      {t("settings.tabs.components")}
                      <Tabs.Indicator />
                    </Tabs.Tab>
                    <Tabs.Tab key="others" id={"others"}>
                      {t("settings.tabs.others")}
                      <Tabs.Indicator />
                    </Tabs.Tab>
                    <Tabs.Tab key="privacy" id={"privacy"}>
                      {t("settings.tabs.privacy")}
                      <Tabs.Indicator />
                    </Tabs.Tab>
                    <Tabs.Tab key="updates" id={"updates"}>
                      {t("settings.tabs.updates")}
                      <Tabs.Indicator />
                    </Tabs.Tab>
                    <Tabs.Tab key="about" id={"about"}>
                      {t("settings.tabs.about")}
                      <Tabs.Indicator />
                    </Tabs.Tab>
                  </Tabs.List>
                </Tabs.ListContainer>
              </Tabs>
            </Card.Content>
          </Card>
        </motion.div>

        {/* Content Card */}
        <motion.div
          key={selectedTab}
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3 }}
        >
          <Card className={LAYOUT.GLASS_CARD.BASE}>
            <Card.Content className="p-6">
              {selectedTab === "general" && (
                <div className="flex flex-col gap-6">
                  {/* Paths */}
                  <div className="flex flex-col gap-6">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <p className="font-medium">
                          {t("settings.body.paths.title")}
                        </p>
                      </div>
                      <div className="flex items-center gap-2">
                        <Button
                          onPress={() => {
                            setPathError("");
                            resetOnOpen();
                          }}
                          isDisabled={savingBaseRoot}
                          variant={"ghost"}
                          className={"rounded-full"}
                        >
                          {t("settings.body.paths.reset")}
                        </Button>
                        <Button
                          isDisabled={!newBaseRoot || !baseRootWritable}
                          onPress={() => void persistBasePath()}
                          variant={"primary"}
                          isPending={savingBaseRoot}
                          className={cn(
                            "rounded-full",
                            "bg-brand-500 hover:bg-brand-500 brand-primary-foreground font-bold shadow-lg shadow-brand-900/20",
                          )}
                        >
                          {({ isPending }) => (
                            <>
                              <Spinner
                                size="sm"
                                color="current"
                                className={isPending ? "" : "hidden"}
                              />
                              {t("settings.body.paths.apply")}
                            </>
                          )}
                        </Button>
                      </div>
                    </div>

                    <div className="space-y-4">
                      <TextField
                        className={cn(
                          "group",
                          COMPONENT_STYLES.input.mainWrapper,
                        )}
                        value={newBaseRoot}
                        isInvalid={Boolean(pathError) || !baseRootWritable}
                        isDisabled={savingBaseRoot}
                        onChange={setNewBaseRoot}
                      >
                        <Label className={COMPONENT_STYLES.input.label}>
                          {t("settings.body.paths.base_root") as string}
                        </Label>
                        <InputGroup
                          className={cn(
                            COMPONENT_STYLES.input.inputWrapper,
                            COMPONENT_STYLES.input.innerWrapper,
                            "rounded-lg",
                          )}
                        >
                          <InputGroup.Input
                            className={COMPONENT_STYLES.input.input}
                          />
                          <InputGroup.Suffix>
                            {
                              <Button
                                size="sm"
                                onPress={async () => {
                                  try {
                                    const options: any = {
                                      Title: t("settings.body.paths.title"),
                                      CanChooseDirectories: true,
                                      CanChooseFiles: false,
                                      PromptForSingleSelection: true,
                                    };
                                    if (baseRoot) {
                                      options.Directory = baseRoot;
                                    }
                                    console.log(options);
                                    const result =
                                      await Dialogs.OpenFile(options);
                                    if (
                                      Array.isArray(result) &&
                                      result.length > 0
                                    ) {
                                      setNewBaseRoot(result[0]);
                                    } else if (
                                      typeof result === "string" &&
                                      result
                                    ) {
                                      setNewBaseRoot(result);
                                    }
                                  } catch (e) {
                                    console.error(e);
                                  }
                                }}
                                variant={"secondary"}
                                className={"rounded-full"}
                              >
                                {t("common.browse")}
                              </Button>
                            }
                          </InputGroup.Suffix>
                        </InputGroup>
                      </TextField>
                      {pathError && (
                        <p role="alert" className="text-sm text-danger px-1">
                          {pathError}
                        </p>
                      )}
                      {newBaseRoot &&
                      newBaseRoot !== baseRoot &&
                      baseRootWritable ? (
                        <div
                          className="select-text text-xs text-amber-500 px-1"
                          title={newBaseRoot}
                        >
                          {t("settings.body.paths.base_root") +
                            ": " +
                            newBaseRoot}
                        </div>
                      ) : null}
                      {!baseRootWritable ? (
                        <div className="text-xs text-rose-500 px-1">
                          {t("settings.body.paths.not_writable")}
                        </div>
                      ) : null}

                      <div className="grid grid-cols-1 gap-2 pt-2">
                        <div className="p-3 rounded-xl bg-surface-secondary/50 dark:bg-surface-secondary/30 border border-border/50 dark:border-white/5">
                          <div
                            className="text-xs text-muted dark:text-zinc-400 flex items-center gap-2 truncate"
                            title={installerDir || "-"}
                          >
                            <LuHardDrive size={14} />
                            <span className="font-medium">
                              {t("settings.body.paths.installer")}:
                            </span>
                            <span className="select-text opacity-70">
                              {installerDir || "-"}
                            </span>
                          </div>
                        </div>
                        <div className="p-3 rounded-xl bg-surface-secondary/50 dark:bg-surface-secondary/30 border border-border/50 dark:border-white/5">
                          <div
                            className="text-xs text-muted dark:text-zinc-400 flex items-center gap-2 truncate"
                            title={versionsDir || "-"}
                          >
                            <LuHardDrive size={14} />
                            <span className="font-medium">
                              {t("settings.body.paths.versions")}:
                            </span>
                            <span className="select-text opacity-70">
                              {versionsDir || "-"}
                            </span>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>

                  <Separator className="bg-surface-tertiary/50" />

                  {/* Language */}
                  <div className="flex items-center justify-between">
                    <div className="flex flex-col">
                      <p className="font-medium">
                        {t("settings.body.language.name")}
                      </p>
                      <p className="text-xs text-muted dark:text-zinc-400">
                        {langNames.find((l) => l.code === selectedLang)
                          ?.language || selectedLang}
                      </p>
                      {languageChanged && (
                        <div className="text-xs text-amber-500 mt-1">
                          {t("settings.lang.changed")}
                        </div>
                      )}
                    </div>
                    <Dropdown>
                      <Button variant={"outline"} className={"rounded-full"}>
                        {t("settings.body.language.button")}
                      </Button>
                      <Dropdown.Popover
                        className={COMPONENT_STYLES.dropdown.content}
                      >
                        <Dropdown.Menu
                          aria-label={t("settings.body.language.button")}
                          disallowEmptySelection
                          selectionMode="single"
                          className="max-h-60 overflow-y-auto"
                          selectedKeys={new Set([selectedLang])}
                          onSelectionChange={(keys) => {
                            const arr = Array.from(
                              keys as unknown as Set<string>,
                            );
                            const next = arr[0];
                            if (typeof next === "string" && next.length > 0) {
                              setSelectedLang(next);
                              Promise.resolve(i18n.changeLanguage(next)).then(
                                () => {
                                  try {
                                    localStorage.setItem("i18nextLng", next);
                                  } catch {}
                                  setLanguageChanged(true);
                                },
                              );
                            }
                          }}
                        >
                          {langNames.map((lang) => (
                            <Dropdown.Item
                              key={lang.code}
                              id={lang.code}
                              textValue={lang.language}
                            >
                              <Label>{lang.language}</Label>
                              <Dropdown.ItemIndicator />
                            </Dropdown.Item>
                          ))}
                        </Dropdown.Menu>
                      </Dropdown.Popover>
                    </Dropdown>
                  </div>

                  <Separator className="bg-surface-tertiary/50" />

                  {/* Discord RPC */}
                  <div className="flex items-center justify-between">
                    <div className="flex flex-col gap-1">
                      <p className="font-medium">
                        {t("settings.discord_rpc.title")}
                      </p>
                      <p className="text-xs text-muted dark:text-zinc-400">
                        {t("settings.discord_rpc.desc")}
                      </p>
                    </div>
                    <Switch
                      aria-label={t("settings.discord_rpc.title")}
                      size="sm"
                      isSelected={discordRpcEnabled}
                      onChange={(isSelected: boolean) => {
                        setDiscordRpcEnabled(isSelected);
                        SetDisableDiscordRPC(!isSelected);
                      }}
                      className={"group"}
                    >
                      <Switch.Content>
                        <Switch.Control
                          className={"group-data-[selected]:bg-brand-500"}
                        >
                          <Switch.Thumb></Switch.Thumb>
                        </Switch.Control>
                        <span></span>
                      </Switch.Content>
                    </Switch>
                  </div>
                </div>
              )}
              {selectedTab === "personalization" && (
                <div className="flex flex-col gap-6">
                  <nav aria-label={t("settings.tabs.personalization")} className="sticky top-14 z-20 flex flex-wrap gap-2 rounded-xl bg-surface p-2 launcher-material-panel">
                    {APPEARANCE_SECTIONS.map((section) => (
                      <Button key={section} size="sm" aria-pressed={appearanceSection === section} variant={appearanceSection === section ? "primary" : "secondary"} onPress={() => setAppearanceSection(section)}>{t(`audit.usability.appearance_${section}`)}</Button>
                    ))}
                  </nav>
                  <section aria-label={t(`audit.usability.appearance_${appearanceSection}`)} className="min-w-0 space-y-6">
                    {appearanceSection === "theme" && <><div className="py-4 flex flex-col gap-4 border-b border-border/50">
                        <div className="flex flex-col gap-4">
                          <div className="flex flex-col gap-1">
                            <p className="font-medium text-foreground dark:text-zinc-200">
                              {t("settings.appearance.theme_mode")}
                            </p>
                            <p className="text-xs text-muted dark:text-zinc-400">
                              {t("settings.appearance.theme_mode_desc")}
                            </p>
                          </div>
                          <div className="w-full overflow-x-auto scrollbar-hide">
                            <Tabs
                              selectedKey={themeMode}
                              onSelectionChange={(key) => {
                                const val = key as ThemeMode;
                                setThemeMode(val);
                              }}
                              variant="primary"
                            >
                              <Tabs.ListContainer>
                                <Tabs.List
                                  className={COMPONENT_STYLES.tabs.tabList}
                                >
                                  <Tabs.Tab key="light" id={"light"}>
                                    {
                                      <div className="flex items-center gap-2">
                                        <LuSun size={14} />
                                        <span>
                                          {t("settings.appearance.theme_light")}
                                        </span>
                                      </div>
                                    }
                                    <Tabs.Indicator />
                                  </Tabs.Tab>
                                  <Tabs.Tab key="dark" id={"dark"}>
                                    {
                                      <div className="flex items-center gap-2">
                                        <LuMoon size={14} />
                                        <span>
                                          {t("settings.appearance.theme_dark")}
                                        </span>
                                      </div>
                                    }
                                    <Tabs.Indicator />
                                  </Tabs.Tab>
                                  <Tabs.Tab key="schedule" id={"schedule"}>
                                    {
                                      <div className="flex items-center gap-2">
                                        <LuHardDrive size={14} />
                                        <span>
                                          {t(
                                            "settings.appearance.theme_schedule",
                                          )}
                                        </span>
                                      </div>
                                    }
                                    <Tabs.Indicator />
                                  </Tabs.Tab>
                                  <Tabs.Tab key="auto" id={"auto"}>
                                    {
                                      <div className="flex items-center gap-2">
                                        <RxDesktop size={14} />
                                        <span>
                                          {t("settings.appearance.theme_auto")}
                                        </span>
                                      </div>
                                    }
                                    <Tabs.Indicator />
                                  </Tabs.Tab>
                                  <Tabs.Tab key="system" id={"system"}>
                                    {
                                      <div className="flex items-center gap-2">
                                        <LuMonitor size={14} />
                                        <span>
                                          {t(
                                            "settings.appearance.theme_system",
                                          )}
                                        </span>
                                      </div>
                                    }
                                    <Tabs.Indicator />
                                  </Tabs.Tab>
                                </Tabs.List>
                              </Tabs.ListContainer>
                            </Tabs>
                          </div>
                        </div>

                        <AnimatePresence initial={false} mode="wait">
                          {themeMode === "system" && (
                            <motion.div
                              key="system-panel"
                              initial={{ opacity: 0, height: 0 }}
                              animate={{ opacity: 1, height: "auto" }}
                              exit={{ opacity: 0, height: 0 }}
                              transition={{ duration: 0.2, ease: "easeInOut" }}
                              className="overflow-hidden"
                            >
                              <div className="flex flex-col gap-4 p-4 mt-2 rounded-2xl bg-surface-secondary/50 border border-border/50">
                                <div className="flex flex-col gap-1">
                                  <p className="text-xs font-bold text-foreground uppercase tracking-wider">
                                    {t("settings.appearance.theme_system_desc")}
                                  </p>
                                </div>
                              </div>
                            </motion.div>
                          )}
                          {themeMode === "schedule" && (
                            <motion.div
                              key="schedule-panel"
                              initial={{ opacity: 0, height: 0 }}
                              animate={{ opacity: 1, height: "auto" }}
                              exit={{ opacity: 0, height: 0 }}
                              transition={{ duration: 0.2, ease: "easeInOut" }}
                              className="overflow-hidden"
                            >
                              <div className="flex flex-col gap-4 p-4 mt-2 rounded-2xl bg-surface-secondary/50 border border-border/50">
                                <div className="flex flex-col gap-1">
                                  <p className="text-xs font-bold text-foreground uppercase tracking-wider">
                                    {t(
                                      "settings.appearance.theme_schedule_desc",
                                    )}
                                  </p>
                                </div>
                                <div className="flex gap-4">
                                  <TextField
                                    className={cn(
                                      "group",
                                      COMPONENT_STYLES.input.mainWrapper,
                                    )}
                                  >
                                    <Label
                                      className={COMPONENT_STYLES.input.label}
                                    >
                                      {t(
                                        "settings.appearance.theme_start_time",
                                      )}
                                    </Label>
                                    <Input
                                      type="time"
                                      value={scheduleStart}
                                      onChange={(e) => {
                                        const val = e.target.value;
                                        setScheduleStart(val);
                                        localStorage.setItem(
                                          "app.scheduleStart",
                                          val,
                                        );
                                        window.dispatchEvent(
                                          new CustomEvent(
                                            "app-theme-mode-changed",
                                          ),
                                        );
                                      }}
                                      className={cn(
                                        COMPONENT_STYLES.input.inputWrapper,
                                        COMPONENT_STYLES.input.input,
                                        "min-h-8 text-sm",
                                      )}
                                    />
                                  </TextField>
                                  <TextField
                                    className={cn(
                                      "group",
                                      COMPONENT_STYLES.input.mainWrapper,
                                    )}
                                  >
                                    <Label
                                      className={COMPONENT_STYLES.input.label}
                                    >
                                      {t("settings.appearance.theme_end_time")}
                                    </Label>
                                    <Input
                                      type="time"
                                      value={scheduleEnd}
                                      onChange={(e) => {
                                        const val = e.target.value;
                                        setScheduleEnd(val);
                                        localStorage.setItem(
                                          "app.scheduleEnd",
                                          val,
                                        );
                                        window.dispatchEvent(
                                          new CustomEvent(
                                            "app-theme-mode-changed",
                                          ),
                                        );
                                      }}
                                      className={cn(
                                        COMPONENT_STYLES.input.inputWrapper,
                                        COMPONENT_STYLES.input.input,
                                        "min-h-8 text-sm",
                                      )}
                                    />
                                  </TextField>
                                </div>
                              </div>
                            </motion.div>
                          )}

                          {themeMode === "auto" && (
                            <motion.div
                              key="auto-panel"
                              initial={{ opacity: 0, height: 0 }}
                              animate={{ opacity: 1, height: "auto" }}
                              exit={{ opacity: 0, height: 0 }}
                              transition={{ duration: 0.2, ease: "easeInOut" }}
                              className="overflow-hidden"
                            >
                              <div className="flex flex-col gap-4 p-4 mt-2 rounded-2xl bg-surface-secondary/50 border border-border/50">
                                <div className="flex flex-col gap-1">
                                  <p className="text-xs font-bold text-foreground uppercase tracking-wider">
                                    {t("settings.appearance.theme_auto_desc")}
                                  </p>
                                </div>

                                {loadingSunTimes ? (
                                  <div className="flex items-center gap-2 py-2">
                                    <Spinner size="sm" color={"accent"} />
                                    <p className="text-xs text-muted">
                                      {t("settings.appearance.calculating")}
                                    </p>
                                  </div>
                                ) : sunTimes ? (
                                  <div className="flex flex-col gap-3">
                                    <div className="flex items-center gap-6">
                                      <div className="flex items-center gap-2">
                                        <div className="p-1.5 rounded-lg bg-amber-100/50 text-amber-600">
                                          <LuSun size={14} />
                                        </div>
                                        <div className="flex flex-col">
                                          <p className="text-[10px] text-muted uppercase font-bold">
                                            {t(
                                              "settings.appearance.sunrise_time",
                                            )}
                                          </p>
                                          <p className="text-sm font-mono font-bold text-foreground">
                                            {sunTimes.sunrise}
                                          </p>
                                        </div>
                                      </div>
                                      <div className="flex items-center gap-2">
                                        <div className="p-1.5 rounded-lg bg-brand-100/50 dark:bg-brand-500/10 text-amber-400">
                                          <LuMoon size={14} />
                                        </div>
                                        <div className="flex flex-col">
                                          <p className="text-[10px] text-muted uppercase font-bold">
                                            {t(
                                              "settings.appearance.sunset_time",
                                            ) || "日落"}
                                          </p>
                                          <p className="text-sm font-mono font-bold text-foreground">
                                            {sunTimes.sunset}
                                          </p>
                                        </div>
                                      </div>
                                    </div>

                                    <Button
                                      size="sm"
                                      variant={"secondary"}
                                      onPress={refreshSunTimes}
                                      className={cn(
                                        "rounded-full",
                                        "h-7 text-xs self-start bg-surface-tertiary/50 hover:bg-surface-quaternary/50",
                                      )}
                                    >
                                      {<RxUpdate size={12} />}
                                      {t("common.refresh")}
                                    </Button>
                                  </div>
                                ) : (
                                  <div className="flex flex-col gap-2 py-2">
                                    <p className="text-xs text-rose-500">
                                      {t(
                                        "settings.appearance.sun_fetch_failed",
                                      )}
                                    </p>
                                    <Button
                                      size="sm"
                                      variant={"secondary"}
                                      onPress={refreshSunTimes}
                                      className={cn(
                                        "rounded-full",
                                        "h-7 text-xs self-start bg-surface-tertiary/50",
                                      )}
                                    >
                                      {<RxUpdate size={12} />}
                                      {t("common.retry")}
                                    </Button>
                                  </div>
                                )}
                              </div>
                            </motion.div>
                          )}
                        </AnimatePresence>
                      </div><div className="flex flex-col gap-4">
                    <div className="flex items-center gap-2.5 mb-2">
                      <div className="w-1.5 h-5 bg-brand-500 rounded-full" />
                      <p className="text-base font-bold text-foreground uppercase tracking-wider">
                        {t("settings.appearance.mode_config")}
                      </p>
                    </div>

                    <Card className="border-none shadow-none bg-transparent overflow-visible">
                      <Card.Content className="p-0 flex flex-col">
                        {/* Mode Switcher Header */}
                        <div className="py-4 border-b border-border/50 flex items-center justify-between rounded-t-2xl">
                          <div className="flex flex-col gap-0.5">
                            <p className="font-bold text-foreground dark:text-zinc-100 flex items-center gap-2">
                              {themeSettingMode === "light" ? (
                                <LuSun className="text-amber-500" size={18} />
                              ) : (
                                <LuMoon className="text-amber-400" size={18} />
                              )}
                              {themeSettingMode === "light"
                                ? t("settings.appearance.theme_light")
                                : t("settings.appearance.theme_dark")}
                            </p>
                            <p className="text-xs text-muted">
                              {t("settings.appearance.edit_mode_desc")}
                            </p>
                          </div>
                          <Tabs
                            selectedKey={themeSettingMode}
                            onSelectionChange={(key) =>
                              setThemeSettingMode(key as "light" | "dark")
                            }
                            variant="primary"
                          >
                            <Tabs.ListContainer>
                              <Tabs.List
                                className={COMPONENT_STYLES.tabs.tabList}
                              >
                                <Tabs.Tab
                                  key="light"
                                  id={"light"}
                                  className={COMPONENT_STYLES.tabs.tabContent}
                                >
                                  {t("settings.appearance.theme_light")}
                                  <Tabs.Indicator
                                    className={COMPONENT_STYLES.tabs.cursor}
                                  />
                                </Tabs.Tab>
                                <Tabs.Tab
                                  key="dark"
                                  id={"dark"}
                                  className={COMPONENT_STYLES.tabs.tabContent}
                                >
                                  {t("settings.appearance.theme_dark")}
                                  <Tabs.Indicator
                                    className={COMPONENT_STYLES.tabs.cursor}
                                  />
                                </Tabs.Tab>
                              </Tabs.List>
                            </Tabs.ListContainer>
                          </Tabs>
                        </div>
                        {/* Content Area */}
                        <div className="py-6 flex flex-col gap-8">
                          {/* Theme Color Group */}
                          <div className="flex flex-col gap-4 p-5 rounded-3xl bg-surface-tertiary/10 border border-border/50">
                            {/* Theme Color Section */}
                            <div className="flex flex-col gap-4">
                              <div className="flex items-center gap-2">
                                <LuPalette
                                  className="text-brand-500"
                                  size={18}
                                />
                                <div className="flex flex-col gap-0.5">
                                  <p className="text-sm font-bold text-foreground">
                                    {t("settings.appearance.theme_color")}
                                  </p>
                                  <p className="text-xs text-muted">
                                    {t("settings.appearance.theme_color_desc")}
                                  </p>
                                </div>
                              </div>
                              <div className="flex flex-col gap-6">
                                {/* Preset Colors Group (Manual 50-950) */}
                                <div className="flex flex-col gap-3">
                                  <div className="flex items-center justify-between px-1">
                                    <span className="text-xs font-bold text-muted uppercase tracking-wider">
                                      {t("settings.appearance.theme_standard")}
                                    </span>
                                  </div>
                                  <div className="flex gap-3 flex-wrap p-4 rounded-2xl bg-surface-tertiary/20 border border-border/50">
                                    {THEME_GROUPS.preset.map((colorName) => {
                                      const isSelected =
                                        themeSettingMode === "light"
                                          ? lightThemeColor === colorName
                                          : darkThemeColor === colorName;
                                      return (
                                        <Tooltip key={colorName}>
                                          <Button
                                            isIconOnly
                                            variant="ghost"
                                            aria-pressed={isSelected}
                                            aria-label={t("audit.mods.theme_color", {
                                              name: t(`audit.mods.colors.${colorName}`),
                                              hex: THEMES[colorName][500],
                                            })}
                                            className={`w-8 h-8 min-w-0 p-0 rounded-full cursor-pointer flex items-center justify-center transition-all hover:scale-110 active:scale-95 focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-focus ${
                                              isSelected
                                                ? "ring-2 ring-offset-2 ring-brand-500 shadow-lg"
                                                : ""
                                            }`}
                                            style={{
                                              backgroundColor:
                                                THEMES[colorName][500],
                                            }}
                                            onPress={() => {
                                              if (themeSettingMode === "light") {
                                                setLightThemeColor(colorName);
                                                localStorage.setItem(
                                                  "app.lightThemeColor",
                                                  colorName,
                                                );
                                              } else {
                                                setDarkThemeColor(colorName);
                                                localStorage.setItem(
                                                  "app.darkThemeColor",
                                                  colorName,
                                                );
                                              }
                                              window.dispatchEvent(
                                                new CustomEvent(
                                                  "app-theme-changed",
                                                ),
                                              );
                                            }}
                                          >
                                            {isSelected && (
                                              <div className="w-2.5 h-2.5 bg-white rounded-full shadow-sm" />
                                            )}
                                          </Button>
                                          <Tooltip.Content>
                                            {t(`audit.mods.colors.${colorName}`)}
                                          </Tooltip.Content>
                                        </Tooltip>
                                      );
                                    })}
                                  </div>
                                </div>

                                {/* Generated Colors Group (Automatic) */}
                                <div className="flex flex-col gap-3">
                                  <div className="flex items-center justify-between px-1">
                                    <span className="text-xs font-bold text-muted uppercase tracking-wider">
                                      {t("settings.appearance.theme_generated")}
                                    </span>
                                  </div>
                                  <div className="flex gap-3 flex-wrap p-4 rounded-2xl bg-surface-tertiary/20 border border-border/50">
                                    {THEME_GROUPS.generated.map((colorName) => {
                                      const isSelected =
                                        themeSettingMode === "light"
                                          ? lightThemeColor === colorName
                                          : darkThemeColor === colorName;
                                      return (
                                        <Tooltip key={colorName}>
                                          <Button
                                            isIconOnly
                                            variant="ghost"
                                            aria-pressed={isSelected}
                                            aria-label={t("audit.mods.theme_color", {
                                              name: t(`audit.mods.colors.${colorName}`),
                                              hex: THEMES[colorName][500],
                                            })}
                                            className={`w-8 h-8 min-w-0 p-0 rounded-full cursor-pointer flex items-center justify-center transition-all hover:scale-110 active:scale-95 focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-focus ${
                                              isSelected
                                                ? "ring-2 ring-offset-2 ring-brand-500 shadow-lg"
                                                : ""
                                            }`}
                                            style={{
                                              backgroundColor:
                                                THEMES[colorName][500],
                                            }}
                                            onPress={() => {
                                              if (themeSettingMode === "light") {
                                                setLightThemeColor(colorName);
                                                localStorage.setItem(
                                                  "app.lightThemeColor",
                                                  colorName,
                                                );
                                              } else {
                                                setDarkThemeColor(colorName);
                                                localStorage.setItem(
                                                  "app.darkThemeColor",
                                                  colorName,
                                                );
                                              }
                                              window.dispatchEvent(
                                                new CustomEvent(
                                                  "app-theme-changed",
                                                ),
                                              );
                                            }}
                                          >
                                            {isSelected && (
                                              <div className="w-2.5 h-2.5 bg-white rounded-full shadow-sm" />
                                            )}
                                          </Button>
                                          <Tooltip.Content>
                                            {t(`audit.mods.colors.${colorName}`)}
                                          </Tooltip.Content>
                                        </Tooltip>
                                      );
                                    })}
                                    <Tooltip>
                                      <Button
                                        isIconOnly
                                        variant="ghost"
                                        aria-label={t("audit.mods.custom_color")}
                                        aria-pressed={
                                          themeSettingMode === "light"
                                            ? lightThemeColor === "custom"
                                            : darkThemeColor === "custom"
                                        }
                                        className={`w-8 h-8 min-w-0 p-0 rounded-full cursor-pointer flex items-center justify-center transition-all hover:scale-110 active:scale-95 relative overflow-hidden group focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-focus ${
                                          (
                                            themeSettingMode === "light"
                                              ? lightThemeColor === "custom"
                                              : darkThemeColor === "custom"
                                          )
                                            ? "ring-2 ring-offset-2 ring-brand-500 shadow-lg"
                                            : "hover:shadow-md"
                                        }`}
                                        onPress={() => {
                                          if (themeSettingMode === "light") {
                                            setLightThemeColor("custom");
                                            localStorage.setItem(
                                              "app.lightThemeColor",
                                              "custom",
                                            );
                                          } else {
                                            setDarkThemeColor("custom");
                                            localStorage.setItem(
                                              "app.darkThemeColor",
                                              "custom",
                                            );
                                          }
                                          window.dispatchEvent(
                                            new CustomEvent("app-theme-changed"),
                                          );
                                        }}
                                        style={{
                                          backgroundColor: activeCustomThemeColor,
                                        }}
                                      >
                                        <LuPalette
                                          className="relative z-10 w-4 h-4 drop-shadow-sm"
                                          style={{ color: customThemeIconColor }}
                                        />

                                        {(themeSettingMode === "light"
                                          ? lightThemeColor === "custom"
                                          : darkThemeColor === "custom") && (
                                          <div className="absolute inset-0 bg-black/10 z-0 flex items-center justify-center">
                                            <div className="w-2.5 h-2.5 bg-white rounded-full shadow-sm z-20" />
                                          </div>
                                        )}
                                      </Button>
                                      <Tooltip.Content>
                                        {t("audit.mods.custom_color")}
                                      </Tooltip.Content>
                                    </Tooltip>
                                  </div>

                                  <Separator className="bg-surface-tertiary/50" />

                                  {/* Base Mode Section */}
                                  <div className="flex flex-col gap-4">
                                    <div className="flex items-center gap-2">
                                      <LuLayers
                                        className="text-brand-500"
                                        size={18}
                                      />
                                      <div className="flex flex-col gap-0.5">
                                        <p className="text-sm font-bold text-foreground">
                                          {t(
                                            "settings.appearance.background_base_mode",
                                          )}
                                        </p>
                                        <p className="text-xs text-muted">
                                          {t(
                                            "settings.appearance.background_base_mode_desc",
                                          )}
                                        </p>
                                      </div>
                                    </div>

                                    <div className="flex flex-col gap-6 mt-2">
                                      <Tabs
                                        selectedKey={
                                          themeSettingMode === "light"
                                            ? lightBackgroundBaseMode
                                            : darkBackgroundBaseMode
                                        }
                                        onSelectionChange={(key) => {
                                          const val = key as string;
                                          if (themeSettingMode === "light") {
                                            setLightBackgroundBaseMode(val);
                                            localStorage.setItem(
                                              "app.lightBackgroundBaseMode",
                                              val,
                                            );
                                          } else {
                                            setDarkBackgroundBaseMode(val);
                                            localStorage.setItem(
                                              "app.darkBackgroundBaseMode",
                                              val,
                                            );
                                          }
                                          window.dispatchEvent(
                                            new CustomEvent(
                                              "app-background-settings-changed",
                                            ),
                                          );
                                        }}
                                        variant="primary"
                                      >
                                        <Tabs.ListContainer>
                                          <Tabs.List
                                            className={
                                              COMPONENT_STYLES.tabs.tabList
                                            }
                                          >
                                            <Tabs.Tab
                                              key="none"
                                              id={"none"}
                                              className={
                                                COMPONENT_STYLES.tabs.tabContent
                                              }
                                            >
                                              {t(
                                                "settings.appearance.background_base_none",
                                              )}
                                              <Tabs.Indicator
                                                className={
                                                  COMPONENT_STYLES.tabs.cursor
                                                }
                                              />
                                            </Tabs.Tab>
                                            <Tabs.Tab
                                              key="theme"
                                              id={"theme"}
                                              className={
                                                COMPONENT_STYLES.tabs.tabContent
                                              }
                                            >
                                              {t(
                                                "settings.appearance.background_base_theme",
                                              )}
                                              <Tabs.Indicator
                                                className={
                                                  COMPONENT_STYLES.tabs.cursor
                                                }
                                              />
                                            </Tabs.Tab>
                                            <Tabs.Tab
                                              key="color"
                                              id={"color"}
                                              className={
                                                COMPONENT_STYLES.tabs.tabContent
                                              }
                                            >
                                              {t(
                                                "settings.appearance.background_base_color",
                                              )}
                                              <Tabs.Indicator
                                                className={
                                                  COMPONENT_STYLES.tabs.cursor
                                                }
                                              />
                                            </Tabs.Tab>
                                          </Tabs.List>
                                        </Tabs.ListContainer>
                                      </Tabs>

                                      <AnimatePresence
                                        initial={false}
                                        mode="wait"
                                      >
                                        {(themeSettingMode === "light"
                                          ? lightBackgroundBaseMode
                                          : darkBackgroundBaseMode) !==
                                          "none" && (
                                          <motion.div
                                            key="background-base-settings"
                                            initial={{
                                              opacity: 0,
                                              height: 0,
                                            }}
                                            animate={{
                                              opacity: 1,
                                              height: "auto",
                                            }}
                                            exit={{
                                              opacity: 0,
                                              height: 0,
                                            }}
                                            className="flex flex-col gap-6 overflow-hidden"
                                          >
                                            {(themeSettingMode === "light"
                                              ? lightBackgroundBaseMode
                                              : darkBackgroundBaseMode) ===
                                              "color" && (
                                              <div className="flex flex-col gap-3">
                                                <div className="flex items-center justify-between">
                                                  <p className="text-xs font-medium text-foreground">
                                                    {t(
                                                      "settings.appearance.background_base_color_pick",
                                                    )}
                                                  </p>
                                                  <div className="px-2 py-0.5 bg-surface-tertiary/50 rounded font-mono text-[10px] text-brand-500">
                                                    {themeSettingMode ===
                                                    "light"
                                                      ? lightBackgroundBaseColor
                                                      : darkBackgroundBaseColor}
                                                  </div>
                                                </div>
                                                <CustomColorPicker
                                                  color={
                                                    themeSettingMode === "light"
                                                      ? lightBackgroundBaseColor
                                                      : darkBackgroundBaseColor
                                                  }
                                                  onChange={(hex) => {
                                                    if (
                                                      themeSettingMode ===
                                                      "light"
                                                    ) {
                                                      setLightBackgroundBaseColor(
                                                        hex,
                                                      );
                                                      localStorage.setItem(
                                                        "app.lightBackgroundBaseColor",
                                                        hex,
                                                      );
                                                    } else {
                                                      setDarkBackgroundBaseColor(
                                                        hex,
                                                      );
                                                      localStorage.setItem(
                                                        "app.darkBackgroundBaseColor",
                                                        hex,
                                                      );
                                                    }
                                                    window.dispatchEvent(
                                                      new CustomEvent(
                                                        "app-background-settings-changed",
                                                      ),
                                                    );
                                                  }}
                                                />
                                              </div>
                                            )}

                                            <div className="flex flex-col gap-2">
                                              <div className="flex items-center justify-between">
                                                <p className="text-xs font-medium text-foreground">
                                                  {t(
                                                    "settings.appearance.background_base_opacity",
                                                  )}
                                                </p>
                                                <div className="flex items-center">
                                                  <span className="text-xs font-mono text-brand-500">
                                                    {themeSettingMode ===
                                                    "light"
                                                      ? lightBackgroundBaseOpacity
                                                      : darkBackgroundBaseOpacity}
                                                    %
                                                  </span>
                                                </div>
                                              </div>
                                              <Slider
                                                step={1}
                                                maxValue={100}
                                                minValue={0}
                                                aria-label={t(
                                                  "settings.appearance.background_base_opacity",
                                                )}
                                                value={
                                                  themeSettingMode === "light"
                                                    ? lightBackgroundBaseOpacity
                                                    : darkBackgroundBaseOpacity
                                                }
                                                onChange={(v) => {
                                                  const val = Number(v);
                                                  if (
                                                    themeSettingMode === "light"
                                                  ) {
                                                    setLightBackgroundBaseOpacity(
                                                      val,
                                                    );
                                                    localStorage.setItem(
                                                      "app.lightBackgroundBaseOpacity",
                                                      String(val),
                                                    );
                                                  } else {
                                                    setDarkBackgroundBaseOpacity(
                                                      val,
                                                    );
                                                    localStorage.setItem(
                                                      "app.darkBackgroundBaseOpacity",
                                                      String(val),
                                                    );
                                                  }
                                                  window.dispatchEvent(
                                                    new CustomEvent(
                                                      "app-background-settings-changed",
                                                    ),
                                                  );
                                                }}
                                              >
                                                <Slider.Track>
                                                  <Slider.Fill
                                                    className={"bg-brand-500"}
                                                  />
                                                  <Slider.Thumb
                                                    className={"bg-brand-500"}
                                                  />
                                                </Slider.Track>
                                              </Slider>
                                            </div>
                                          </motion.div>
                                        )}
                                      </AnimatePresence>
                                    </div>
                                  </div>
                                </div>
                              </div>

                              {(themeSettingMode === "light"
                                ? lightThemeColor === "custom"
                                : darkThemeColor === "custom") && (
                                <div className="flex flex-col gap-4 bg-surface-tertiary/20 p-6 rounded-2xl border border-border/50">
                                  <div className="flex items-center justify-between">
                                    <p className="text-sm font-bold text-foreground">
                                      {t("settings.appearance.custom_color") ||
                                        "Custom Color"}
                                    </p>
                                    <div className="px-3 py-1 bg-surface-tertiary/50 rounded-lg">
                                      <span className="text-xs font-mono uppercase text-brand-500 font-bold">
                                        {activeCustomThemeColor}
                                      </span>
                                    </div>
                                  </div>

                                  <CustomColorPicker
                                    color={activeCustomThemeColor}
                                    onChange={(hex) => {
                                      if (themeSettingMode === "light") {
                                        setLightCustomThemeColor(hex);
                                        localStorage.setItem(
                                          "app.lightCustomThemeColor",
                                          hex,
                                        );
                                      } else {
                                        setDarkCustomThemeColor(hex);
                                        localStorage.setItem(
                                          "app.darkCustomThemeColor",
                                          hex,
                                        );
                                      }
                                      window.dispatchEvent(
                                        new CustomEvent("app-theme-changed"),
                                      );
                                    }}
                                  />
                                </div>
                              )}
                            </div>
                          </div>
                        </div>
                      </Card.Content>
                    </Card>
                  </div></>}
                    {appearanceSection === "layout" && <><div className="py-4 flex items-center justify-between border-b border-border/50">
                        <div className="flex flex-col gap-1">
                          <p className="font-medium text-foreground dark:text-zinc-200">
                            {t("settings.layout.title_navbar")}
                          </p>
                          <p className="text-xs text-muted dark:text-zinc-400">
                            {t("settings.layout.desc_navbar")}
                          </p>
                        </div>
                        <Switch
                          aria-label={t("settings.layout.title_navbar")}
                          size="sm"
                          isSelected={layoutMode === "navbar"}
                          onChange={(isSelected: boolean) => {
                            const mode = isSelected ? "navbar" : "sidebar";
                            setLayoutMode(mode);
                            localStorage.setItem("app.layoutMode", mode);
                            window.dispatchEvent(
                              new CustomEvent("app-layout-changed"),
                            );
                          }}
                          className={"group"}
                        >
                          <Switch.Content>
                            <Switch.Control
                              className={"group-data-[selected]:bg-brand-500"}
                            >
                              <Switch.Thumb></Switch.Thumb>
                            </Switch.Control>
                            <span></span>
                          </Switch.Content>
                        </Switch>
                      </div><div className="py-4 border-b border-border/50 flex items-center justify-between">
                        <div className="flex flex-col gap-1">
                          <p className="font-medium text-foreground dark:text-zinc-200">
                            {t("settings.appearance.disable_animations")}
                          </p>
                          <p className="text-xs text-muted dark:text-zinc-400">
                            {t("settings.appearance.disable_animations_desc")}
                          </p>
                        </div>
                        <Switch
                          aria-label={t(
                            "settings.appearance.disable_animations",
                          )}
                          size="sm"
                          isSelected={disableAnimations}
                          onChange={(isSelected: boolean) => {
                            setDisableAnimations(isSelected);
                            localStorage.setItem(
                              "app.disableAnimations",
                              String(isSelected),
                            );
                            window.dispatchEvent(
                              new CustomEvent("app-animations-changed"),
                            );
                          }}
                          className={"group"}
                        >
                          <Switch.Content>
                            <Switch.Control
                              className={"group-data-[selected]:bg-brand-500"}
                            >
                              <Switch.Thumb></Switch.Thumb>
                            </Switch.Control>
                            <span></span>
                          </Switch.Content>
                        </Switch>
                      </div></>}
                    {appearanceSection === "wallpaper" && <div className="flex flex-col gap-4 p-5 mt-6 rounded-3xl bg-surface-tertiary/10 border border-border/50">
                        <div className="flex items-center gap-2">
                          <LuImage className="text-brand-500" size={18} />
                          <div className="flex flex-col gap-0.5">
                            <div className="flex items-center gap-2">
                              <p className="text-sm font-bold text-foreground">
                                {t("settings.appearance.background_image")}
                              </p>
                              {backgroundImageCount > 0 && (
                                <Chip
                                  size="sm"
                                  variant="soft"
                                  color={"accent"}
                                  className={"h-5 px-1.5 text-[10px] min-w-0"}
                                >
                                  <Chip.Label>
                                    {backgroundImageCount}
                                  </Chip.Label>
                                </Chip>
                              )}
                            </div>
                            <p className="text-xs text-muted">
                              {t("settings.appearance.background_image_desc")}
                            </p>
                          </div>
                        </div>

                        <div className="flex flex-col gap-6">
                          {/* Image Picker Row */}
                          <div className="flex flex-wrap items-center justify-between gap-3">
                            <div className="flex items-center gap-3 overflow-hidden">
                              <div className="w-12 h-12 rounded-lg bg-surface-quaternary/30 flex items-center justify-center flex-shrink-0 border border-border/50 overflow-hidden">
                                {previewBgData && !backgroundImageError ? (
                                  <img
                                    src={previewBgData}
                                    alt=""
                                    aria-hidden="true"
                                    className="w-full h-full object-cover"
                                    onError={() =>
                                      setBackgroundImageError(true)
                                    }
                                  />
                                ) : (
                                  <LuImage className="text-muted" size={20} />
                                )}
                              </div>
                              <div className="flex flex-col min-w-0">
                                <p className="text-xs font-medium truncate text-foreground">
                                  {backgroundImage ||
                                    t("settings.appearance.no_image_selected")}
                                </p>
                              </div>
                            </div>
                            <div className="flex items-center gap-2">
                              {backgroundImage && (
                                <>
                                  <Button size="sm" variant="secondary" isDisabled={backgroundImageCount < 2}
                                    onPress={() => window.dispatchEvent(new Event("app-background-changed"))}>
                                    {t("settings.appearance.material.next_image")}
                                  </Button>
                                  <Button
                                    size="sm"
                                    onPress={async () => {
                                      const path = backgroundImage;
                                      if (path) {
                                        await openDirectory(
                                          path,
                                        );
                                      }
                                    }}
                                    variant={"secondary"}
                                    className={"h-8"}
                                  >
                                    {t("common.open_folder")}
                                  </Button>
                                  <Button
                                    size="sm"
                                    onPress={() => {
                                      setBackgroundImage("");
                                      localStorage.setItem(
                                        "app.backgroundImage",
                                        "",
                                      );
                                      window.dispatchEvent(
                                        new CustomEvent(
                                          "app-background-changed",
                                        ),
                                      );
                                    }}
                                    variant={"danger-soft"}
                                    className={"h-8"}
                                  >
                                    {t("settings.appearance.clear_image")}
                                  </Button>
                                </>
                              )}
                              <Button
                                size="sm"
                                onPress={async () => {
                                  try {
                                    const result = await Dialogs.OpenFile({
                                      Title: t(
                                        "settings.appearance.select_image",
                                      ),
                                      CanChooseDirectories: true,
                                      CanChooseFiles: false,
                                    });
                                    let path = "";
                                    if (
                                      Array.isArray(result) &&
                                      result.length > 0
                                    )
                                      path = result[0];
                                    else if (
                                      typeof result === "string" &&
                                      result
                                    )
                                      path = result;

                                    if (path) {
                                      setBackgroundImage(path);
                                      localStorage.setItem(
                                        "app.backgroundImage",
                                        path,
                                      );
                                      window.dispatchEvent(
                                        new CustomEvent(
                                          "app-background-changed",
                                        ),
                                      );
                                    }
                                  } catch {}
                                }}
                                variant={"primary"}
                                className={"h-8 shadow-sm"}
                              >
                                {t("settings.appearance.select_image")}
                              </Button>
                            </div>
                          </div>

                          {backgroundImage && (
                            <>
                              <Separator className="bg-surface-tertiary/50" />

                              {/* Fit Mode and Play Order */}
                              <div className="grid grid-cols-1 md:grid-cols-2 gap-x-8 gap-y-6">
                                <div className="flex flex-col gap-2">
                                  <p className="text-xs font-medium text-foreground">
                                    {t(
                                      "settings.appearance.background_fit_mode",
                                    )}
                                  </p>
                                  <Select
                                    aria-label={t(
                                      "settings.appearance.background_fit_mode",
                                    )}
                                    value={
                                      Array.from(
                                        new Set([backgroundFitMode]),
                                      )[0] ?? null
                                    }
                                    onChange={(keys) => {
                                      const val = keys as string;
                                      if (!val) return; // Prevent empty selection
                                      setBackgroundFitMode(val);
                                      localStorage.setItem(
                                        "app.backgroundFitMode",
                                        val,
                                      );
                                      window.dispatchEvent(
                                        new CustomEvent(
                                          "app-background-settings-changed",
                                        ),
                                      );
                                    }}
                                  >
                                    <Select.Trigger
                                      className={cn(
                                        COMPONENT_STYLES.select.trigger,
                                        "min-h-8 text-sm",
                                      )}
                                    >
                                      <Select.Value />
                                      <Select.Indicator />
                                    </Select.Trigger>
                                    <Select.Popover
                                      className={
                                        COMPONENT_STYLES.select.popoverContent
                                      }
                                    >
                                      <ListBox
                                        className={
                                          COMPONENT_STYLES.select.listbox
                                        }
                                      >
                                        <ListBox.Item
                                          key="smart"
                                          id={"smart"}
                                          textValue={t(
                                            "settings.appearance.background_fit_smart",
                                          )}
                                        >
                                          <Label>
                                            {t(
                                              "settings.appearance.background_fit_smart",
                                            )}
                                          </Label>
                                          <ListBox.ItemIndicator />
                                        </ListBox.Item>
                                        <ListBox.Item
                                          key="center"
                                          id={"center"}
                                          textValue={t(
                                            "settings.appearance.background_fit_center",
                                          )}
                                        >
                                          <Label>
                                            {t(
                                              "settings.appearance.background_fit_center",
                                            )}
                                          </Label>
                                          <ListBox.ItemIndicator />
                                        </ListBox.Item>
                                        <ListBox.Item
                                          key="fit"
                                          id={"fit"}
                                          textValue={t(
                                            "settings.appearance.background_fit_fit",
                                          )}
                                        >
                                          <Label>
                                            {t(
                                              "settings.appearance.background_fit_fit",
                                            )}
                                          </Label>
                                          <ListBox.ItemIndicator />
                                        </ListBox.Item>
                                        <ListBox.Item
                                          key="stretch"
                                          id={"stretch"}
                                          textValue={t(
                                            "settings.appearance.background_fit_stretch",
                                          )}
                                        >
                                          <Label>
                                            {t(
                                              "settings.appearance.background_fit_stretch",
                                            )}
                                          </Label>
                                          <ListBox.ItemIndicator />
                                        </ListBox.Item>
                                        <ListBox.Item
                                          key="tile"
                                          id={"tile"}
                                          textValue={t(
                                            "settings.appearance.background_fit_tile",
                                          )}
                                        >
                                          <Label>
                                            {t(
                                              "settings.appearance.background_fit_tile",
                                            )}
                                          </Label>
                                          <ListBox.ItemIndicator />
                                        </ListBox.Item>
                                        <ListBox.Item
                                          key="top_left"
                                          id={"top_left"}
                                          textValue={t(
                                            "settings.appearance.background_fit_top_left",
                                          )}
                                        >
                                          <Label>
                                            {t(
                                              "settings.appearance.background_fit_top_left",
                                            )}
                                          </Label>
                                          <ListBox.ItemIndicator />
                                        </ListBox.Item>
                                        <ListBox.Item
                                          key="top_right"
                                          id={"top_right"}
                                          textValue={t(
                                            "settings.appearance.background_fit_top_right",
                                          )}
                                        >
                                          <Label>
                                            {t(
                                              "settings.appearance.background_fit_top_right",
                                            )}
                                          </Label>
                                          <ListBox.ItemIndicator />
                                        </ListBox.Item>
                                      </ListBox>
                                    </Select.Popover>
                                  </Select>
                                </div>

                                <div className="flex flex-col gap-2">
                                  <p className="text-xs font-medium text-foreground">
                                    {t(
                                      "settings.appearance.background_play_order",
                                    )}
                                  </p>
                                  <Select
                                    aria-label={t(
                                      "settings.appearance.background_play_order",
                                    )}
                                    value={
                                      Array.from(
                                        new Set([backgroundPlayOrder]),
                                      )[0] ?? null
                                    }
                                    onChange={(keys) => {
                                      const val = keys as
                                        | "random"
                                        | "sequential";
                                      if (!val) return; // Prevent empty selection
                                      setBackgroundPlayOrder(val);
                                      localStorage.setItem(
                                        "app.backgroundPlayOrder",
                                        val,
                                      );
                                      window.dispatchEvent(
                                        new CustomEvent(
                                          "app-background-settings-changed",
                                        ),
                                      );
                                    }}
                                  >
                                    <Select.Trigger
                                      className={cn(
                                        COMPONENT_STYLES.select.trigger,
                                        "min-h-8 text-sm",
                                      )}
                                    >
                                      <Select.Value />
                                      <Select.Indicator />
                                    </Select.Trigger>
                                    <Select.Popover
                                      className={
                                        COMPONENT_STYLES.select.popoverContent
                                      }
                                    >
                                      <ListBox
                                        className={
                                          COMPONENT_STYLES.select.listbox
                                        }
                                      >
                                        <ListBox.Item
                                          key="random"
                                          id={"random"}
                                          textValue={t(
                                            "settings.appearance.background_play_random",
                                          )}
                                        >
                                          <Label>
                                            {t(
                                              "settings.appearance.background_play_random",
                                            )}
                                          </Label>
                                          <ListBox.ItemIndicator />
                                        </ListBox.Item>
                                        <ListBox.Item
                                          key="sequential"
                                          id={"sequential"}
                                          textValue={t(
                                            "settings.appearance.background_play_sequential",
                                          )}
                                        >
                                          <Label>
                                            {t(
                                              "settings.appearance.background_play_sequential",
                                            )}
                                          </Label>
                                          <ListBox.ItemIndicator />
                                        </ListBox.Item>
                                      </ListBox>
                                    </Select.Popover>
                                  </Select>
                                </div>
                              </div>

                              <div className="grid grid-cols-1 md:grid-cols-2 gap-x-8 gap-y-6">
                                {/* Blur */}
                                <div className="flex flex-col gap-2">
                                  <div className="flex items-center justify-between">
                                    <p className="text-xs font-medium text-foreground">
                                      {t("settings.appearance.background_blur")}
                                    </p>
                                    <div className="flex items-center">
                                      <AppearanceNumberField
                                        label={t("settings.appearance.background_blur")}
                                        value={backgroundBlur}
                                        min={0}
                                        max={50}
                                        onChange={(val) => {
                                          setBackgroundBlur(val);
                                          localStorage.setItem("app.backgroundBlur", String(val));
                                          window.dispatchEvent(new CustomEvent("app-blur-changed"));
                                        }}
                                      />
                                      <span className="text-xs font-mono text-brand-500 ml-0.5">
                                        px
                                      </span>
                                    </div>
                                  </div>
                                  <Slider
                                    step={1}
                                    maxValue={50}
                                    minValue={0}
                                    aria-label={t(
                                      "settings.appearance.background_blur",
                                    )}
                                    value={backgroundBlur}
                                    onChange={(v) => {
                                      const val = Number(v);
                                      setBackgroundBlur(val);
                                      localStorage.setItem(
                                        "app.backgroundBlur",
                                        String(val),
                                      );
                                      window.dispatchEvent(
                                        new CustomEvent("app-blur-changed"),
                                      );
                                    }}
                                  >
                                    <Slider.Track>
                                      <Slider.Fill className={"bg-brand-500"} />
                                      <Slider.Thumb
                                        className={"bg-brand-500"}
                                      />
                                    </Slider.Track>
                                  </Slider>
                                </div>

                                {/* Brightness */}
                                <div className="flex flex-col gap-2">
                                  <div className="flex items-center justify-between">
                                    <p className="text-xs font-medium text-foreground">
                                      {t(
                                        "settings.appearance.background_brightness",
                                      )}
                                    </p>
                                    <div className="flex items-center">
                                      <AppearanceNumberField
                                        label={t("settings.appearance.background_brightness")}
                                        value={backgroundBrightness}
                                        min={20}
                                        max={100}
                                        onChange={(val) => {
                                          setBackgroundBrightness(val);
                                          localStorage.setItem("app.backgroundBrightness", String(val));
                                          window.dispatchEvent(new CustomEvent("app-brightness-changed"));
                                        }}
                                      />
                                      <span className="text-xs font-mono text-brand-500 ml-0.5">
                                        %
                                      </span>
                                    </div>
                                  </div>
                                  <Slider
                                    step={1}
                                    maxValue={100}
                                    minValue={20}
                                    aria-label={t(
                                      "settings.appearance.background_brightness",
                                    )}
                                    value={backgroundBrightness}
                                    onChange={(v) => {
                                      const val = Number(v);
                                      setBackgroundBrightness(val);
                                      localStorage.setItem(
                                        "app.backgroundBrightness",
                                        String(val),
                                      );
                                      window.dispatchEvent(
                                        new CustomEvent(
                                          "app-brightness-changed",
                                        ),
                                      );
                                    }}
                                  >
                                    <Slider.Track>
                                      <Slider.Fill className={"bg-brand-500"} />
                                      <Slider.Thumb
                                        className={"bg-brand-500"}
                                      />
                                    </Slider.Track>
                                  </Slider>
                                </div>

                                {/* Opacity */}
                                <div className="flex flex-col gap-2">
                                  <div className="flex items-center justify-between">
                                    <p className="text-xs font-medium text-foreground">
                                      {t(
                                        "settings.appearance.background_opacity",
                                      )}
                                    </p>
                                    <div className="flex items-center">
                                      <AppearanceNumberField
                                        label={t("settings.appearance.background_opacity")}
                                        value={backgroundOpacity}
                                        min={0}
                                        max={100}
                                        onChange={(val) => {
                                          setBackgroundOpacity(val);
                                          localStorage.setItem("app.backgroundOpacity", String(val));
                                          window.dispatchEvent(new CustomEvent("app-opacity-changed"));
                                        }}
                                      />
                                      <span className="text-xs font-mono text-brand-500 ml-0.5">
                                        %
                                      </span>
                                    </div>
                                  </div>
                                  <Slider
                                    step={1}
                                    maxValue={100}
                                    minValue={0}
                                    aria-label={t(
                                      "settings.appearance.background_opacity",
                                    )}
                                    value={backgroundOpacity}
                                    onChange={(v) => {
                                      const val = Number(v);
                                      setBackgroundOpacity(val);
                                      localStorage.setItem(
                                        "app.backgroundOpacity",
                                        String(val),
                                      );
                                      window.dispatchEvent(
                                        new CustomEvent("app-opacity-changed"),
                                      );
                                    }}
                                  >
                                    <Slider.Track>
                                      <Slider.Fill className={"bg-brand-500"} />
                                      <Slider.Thumb
                                        className={"bg-brand-500"}
                                      />
                                    </Slider.Track>
                                  </Slider>
                                </div>
                              </div>
                            </>
                          )}
                        </div>
                      </div>}
                    {appearanceSection === "material" && <BackgroundAppearanceSettings mode={themeSettingMode} onModeChange={setThemeSettingMode} />}
                  </section>
                </div>
              )}
              {selectedTab === "general" && (
                <div className="flex flex-col gap-6"></div>
              )}
              {selectedTab === "components" && (
                <div className="flex flex-col gap-6">
                  {/* LIP */}
                  <div className="flex items-center justify-between gap-4">
                    <div className="flex flex-col gap-1 min-w-0">
                      <p className="font-medium">{t("settings.lip.title")}</p>
                      <p
                        className="text-xs text-muted dark:text-zinc-400 truncate"
                        title={lipSummaryText}
                      >
                        {lipSummaryText}
                      </p>
                    </div>
                    <div className="flex items-center gap-2 shrink-0">
                      <Chip
                        variant="soft"
                        color={
                          lipUpToDate
                            ? "success"
                            : lipInstalled
                              ? "warning"
                              : "default"
                        }
                      >
                        <Chip.Label>
                          {lipUpToDate
                            ? t("settings.lip.latest_label")
                            : lipInstalled
                              ? t("settings.lip.outdated_label")
                              : t("settings.lip.missing_label")}
                        </Chip.Label>
                      </Chip>
                      {lipInstalled && (
                        <Button
                          size="sm"
                          isDisabled={installingLip || cleaningLipCache}
                          onPress={() => {
                            void cleanLipCache().then((err) => {
                              if (err) {
                                toast(t("common.error"), {
                                  description: t(`errors.${err}`, {
                                    defaultValue: err,
                                  }),
                                  variant: "danger",
                                  timeout: 2000,
                                });
                                return;
                              }

                              toast(t("common.success"), {
                                description: t(
                                  "settings.lip.cache_clean_success",
                                ),
                                variant: "success",
                                timeout: 2000,
                              });
                            });
                          }}
                          variant={"outline"}
                          isPending={cleaningLipCache}
                          className={"rounded-full"}
                        >
                          {({ isPending }) => (
                            <>
                              <Spinner
                                size="sm"
                                color="current"
                                className={isPending ? "" : "hidden"}
                              />
                              {cleaningLipCache
                                ? t("settings.lip.cache_cleaning")
                                : t("settings.lip.cache_clean_button")}
                            </>
                          )}
                        </Button>
                      )}
                      <Button
                        size="sm"
                        isDisabled={installingLip || cleaningLipCache}
                        onPress={() => {
                          if (lipInstalled && lipUpToDate) {
                            setLipError("");
                            void refreshLipStatus();
                            return;
                          }
                          setInstallingLip(true);
                          setLipError("");
                          lipProgressDisclosure.open();
                          InstallLip().then((err) => {
                            if (err) {
                              setInstallingLip(false);
                              setLipError(err);
                            } else {
                              refreshLipStatus().finally(() => {
                                setInstallingLip(false);
                              });
                            }
                          });
                        }}
                        variant={"outline"}
                        isPending={installingLip}
                        className={"rounded-full"}
                      >
                        {({ isPending }) => (
                          <>
                            <Spinner
                              size="sm"
                              color="current"
                              className={isPending ? "" : "hidden"}
                            />
                            {installingLip
                              ? t("settings.lip.installing")
                              : lipInstalled
                                ? lipUpToDate
                                  ? t("settings.lip.check_button")
                                  : t("settings.lip.update_button")
                                : t("settings.lip.install_button")}
                          </>
                        )}
                      </Button>
                    </div>
                  </div>

                  <Separator className="bg-surface-tertiary/50" />

                  {/* resource_pack_rules.bin */}
                  <div className="flex items-center justify-between gap-4">
                    <div className="flex flex-col gap-1 min-w-0">
                      <p className="font-medium">
                        {t("settings.resource_rules.title")}
                      </p>
                      <p className="text-xs text-muted dark:text-zinc-400">
                        {resourceRulesChecking
                          ? t("settings.resource_rules.status.checking")
                          : resourceRulesError
                            ? t("settings.resource_rules.status.check_failed", {
                                error: resourceRulesError,
                              })
                            : resourceRulesInstalled
                              ? resourceRulesUpToDate
                                ? t("settings.resource_rules.status.up_to_date")
                                : t("settings.resource_rules.status.outdated")
                              : t("settings.resource_rules.status.missing")}
                      </p>
                      {resourceRulesLocalSha || resourceRulesRemoteSha ? (
                        <p
                          className="text-xs font-mono text-muted dark:text-zinc-500 truncate"
                          title={`local: ${resourceRulesLocalSha || "-"} | remote: ${resourceRulesRemoteSha || "-"}`}
                        >
                          {`local: ${resourceRulesLocalSha ? `${resourceRulesLocalSha.slice(0, 12)}...` : "-"} | remote: ${resourceRulesRemoteSha ? `${resourceRulesRemoteSha.slice(0, 12)}...` : "-"}`}
                        </p>
                      ) : null}
                    </div>
                    <div className="flex items-center gap-2 shrink-0">
                      <Chip
                        variant="soft"
                        color={
                          resourceRulesUpToDate
                            ? "success"
                            : resourceRulesInstalled
                              ? "warning"
                              : "default"
                        }
                      >
                        <Chip.Label>
                          {resourceRulesUpToDate
                            ? t("settings.resource_rules.latest_label")
                            : resourceRulesInstalled
                              ? t("settings.resource_rules.outdated_label")
                              : t("settings.resource_rules.missing_label")}
                        </Chip.Label>
                      </Chip>
                      <Button
                        size="sm"
                        isDisabled={resourceRulesUpdating}
                        onPress={refreshResourceRulesStatus}
                        variant={"outline"}
                        isPending={resourceRulesChecking}
                        className={"rounded-full"}
                      >
                        {({ isPending }) => (
                          <>
                            <Spinner
                              size="sm"
                              color="current"
                              className={isPending ? "" : "hidden"}
                            />
                            {t("settings.resource_rules.check_button")}
                          </>
                        )}
                      </Button>
                      {!resourceRulesUpToDate && (
                        <Button
                          size="sm"
                          isDisabled={resourceRulesChecking}
                          onPress={onUpdateResourceRules}
                          variant={"outline"}
                          isPending={resourceRulesUpdating}
                          className={"rounded-full"}
                        >
                          {({ isPending }) => (
                            <>
                              <Spinner
                                size="sm"
                                color="current"
                                className={isPending ? "" : "hidden"}
                              />
                              {t("settings.resource_rules.update_button")}
                            </>
                          )}
                        </Button>
                      )}
                    </div>
                  </div>
                </div>
              )}
              {selectedTab === "others" && (
                <div className="flex flex-col gap-6">
                  <div className="flex items-center justify-between gap-4">
                    <div className="flex flex-col gap-1">
                      <p className="font-medium">
                        {t("settings.process.title")}
                      </p>
                      <p className="text-xs text-muted dark:text-zinc-400">
                        {t("settings.process.desc")}
                      </p>
                    </div>
                    <Button
                      onPress={() => setProcessModalOpen(true)}
                      variant={"outline"}
                      className={"rounded-full"}
                    >
                      {t("settings.process.scan")}
                    </Button>
                  </div>

                  <Separator className="bg-surface-tertiary/50" />

                  <div className="flex flex-col gap-4">
                    <div className="flex flex-col gap-1">
                      <p className="font-medium">
                        {t("settings.experimental.title")}
                      </p>
                      <p className="text-xs text-muted dark:text-zinc-400 max-w-2xl">
                        {t("settings.experimental.desc")}
                      </p>
                    </div>
                    <div className="flex items-center justify-between gap-4">
                      <div className="flex flex-col gap-1">
                        <p className="font-medium">
                          {t("settings.experimental.instance_backup.title")}
                        </p>
                        <p className="text-xs text-muted dark:text-zinc-400 max-w-2xl">
                          {t("settings.experimental.instance_backup.desc")}
                        </p>
                      </div>
                      <Switch
                        aria-label={t(
                          "settings.experimental.instance_backup.title",
                        )}
                        size="sm"
                        isSelected={experimentalInstanceBackupEnabled}
                        onChange={handleInstanceBackupExperimentalToggle}
                        className={"group"}
                      >
                        <Switch.Content>
                          <Switch.Control
                            className={"group-data-[selected]:bg-brand-500"}
                          >
                            <Switch.Thumb></Switch.Thumb>
                          </Switch.Control>
                          <span></span>
                        </Switch.Content>
                      </Switch>
                    </div>
                  </div>
                </div>
              )}
              {selectedTab === "privacy" && (
                <div className="flex flex-col gap-6">
                  <div className="flex items-center justify-between gap-4">
                    <div className="flex items-start gap-3">
                      <div className="w-9 h-9 rounded-xl bg-brand-500/10 flex items-center justify-center text-brand-600 dark:text-brand-500 shrink-0">
                        <LuShield className="w-5 h-5" />
                      </div>
                      <div className="flex flex-col gap-1">
                        <p className="font-medium">
                          {t("settings.privacy.analytics.title")}
                        </p>
                        <p className="text-xs text-muted dark:text-zinc-400 max-w-2xl">
                          {t("settings.privacy.analytics.desc")}
                        </p>
                      </div>
                    </div>
                    <Switch
                      aria-label={t("settings.privacy.analytics.title")}
                      size="sm"
                      isSelected={clarityEnabled}
                      onChange={setClarityEnabled}
                      className={"group"}
                    >
                      <Switch.Content>
                        <Switch.Control
                          className={"group-data-[selected]:bg-brand-500"}
                        >
                          <Switch.Thumb></Switch.Thumb>
                        </Switch.Control>
                        <span></span>
                      </Switch.Content>
                    </Switch>
                  </div>

                  <Separator className="bg-surface-tertiary/50" />

                  <div className="flex flex-wrap gap-2">
                    <Button
                      onPress={() =>
                        Browser.OpenURL("https://clarity.microsoft.com/terms")
                      }
                      variant={"outline"}
                      className={"rounded-full"}
                    >
                      {t("settings.privacy.links.clarity_terms")}
                    </Button>
                    <Button
                      onPress={() =>
                        Browser.OpenURL(
                          "https://privacy.microsoft.com/privacystatement",
                        )
                      }
                      variant={"ghost"}
                      className={"rounded-full"}
                    >
                      {t("settings.privacy.links.microsoft_privacy")}
                    </Button>
                  </div>
                </div>
              )}
              {selectedTab === "updates" && (
                <div className="flex flex-col gap-6">
                  <div className="flex items-center justify-between">
                    <div className="flex flex-col gap-1">
                      <p className="font-medium">
                        {t("settings.beta_updates.title")}
                      </p>
                      <p className="text-xs text-muted dark:text-zinc-400">
                        {t("settings.beta_updates.desc")}
                      </p>
                    </div>
                    <Switch
                      aria-label={t("settings.beta_updates.title")}
                      size="sm"
                      isSelected={enableBetaUpdates}
                      onChange={(isSelected: boolean) => {
                        setEnableBetaUpdates(isSelected);
                        SetEnableBetaUpdates(isSelected);
                      }}
                      className={"group"}
                    >
                      <Switch.Content>
                        <Switch.Control
                          className={"group-data-[selected]:bg-brand-500"}
                        >
                          <Switch.Thumb></Switch.Thumb>
                        </Switch.Control>
                        <span></span>
                      </Switch.Content>
                    </Switch>
                  </div>

                  <Separator className="bg-surface-tertiary/50" />

                  <div className="flex items-center justify-between">
                    <div className="flex flex-col">
                      <p className="font-medium text-lg">
                        {t("settings.body.version.name")}
                      </p>
                      <p className="text-xs text-muted dark:text-zinc-400">
                        v{appVersion}
                      </p>
                    </div>
                    {checkingUpdate ? (
                      <Spinner size="sm" color={"accent"} />
                    ) : (
                      <Button
                        onPress={onCheckUpdate}
                        variant={"outline"}
                        className={"rounded-full"}
                      >
                        {t("settings.body.version.button")}
                      </Button>
                    )}
                  </div>

                  <AnimatePresence>
                    {hasUpdate && (
                      <motion.div
                        initial={{ opacity: 0, height: 0 }}
                        animate={{ opacity: 1, height: "auto" }}
                        exit={{ opacity: 0, height: 0 }}
                        className="overflow-hidden"
                      >
                        <div className="rounded-xl bg-surface-secondary/50 dark:bg-surface-secondary/30 p-4 border border-border/50 dark:border-white/5">
                          <div className="flex items-center justify-between mb-3">
                            <p className="text-sm font-bold text-brand-600 dark:text-brand-500">
                              {t("settings.body.version.hasnew")} {newVersion}
                            </p>
                            <Button
                              onPress={onUpdate}
                              isDisabled={updating}
                              variant={"primary"}
                              className={cn(
                                "rounded-full",
                                "bg-brand-500 hover:bg-brand-500 brand-primary-foreground font-bold shadow-lg shadow-brand-900/20",
                              )}
                            >
                              {<RxUpdate />}
                              {updating
                                ? t("common.updating")
                                : t("settings.modal.2.footer.download_button")}
                            </Button>
                          </div>

                          {changelog && (
                            <div className="text-sm wrap-break-word leading-6 max-h-[200px] overflow-y-auto pr-1 scrollbar-thin">
                              <ReactMarkdown
                                remarkPlugins={[remarkGfm]}
                                components={{
                                  h1: ({ children }) => (
                                    <h1 className="text-base font-bold my-1">
                                      {children}
                                    </h1>
                                  ),
                                  h2: ({ children }) => (
                                    <h2 className="text-sm font-bold my-1">
                                      {children}
                                    </h2>
                                  ),
                                  p: ({ children }) => (
                                    <p className="my-1 text-foreground">
                                      {children}
                                    </p>
                                  ),
                                  ul: ({ children }) => (
                                    <ul className="list-disc pl-5 my-1 text-foreground">
                                      {children}
                                    </ul>
                                  ),
                                  li: ({ children }) => (
                                    <li className="my-0.5">{children}</li>
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
                                        className="text-brand-500 underline"
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
                                }}
                              >
                                {changelog}
                              </ReactMarkdown>
                            </div>
                          )}

                          {updating && (
                            <div className="mt-3">
                              <ProgressBar
                                size="sm"
                                isIndeterminate={true}
                                color={"success"}
                              >
                                <ProgressBar.Track className={"rounded-sm"}>
                                  <ProgressBar.Fill
                                    className={
                                      "bg-brand-500 hover:bg-brand-500"
                                    }
                                  />
                                </ProgressBar.Track>
                              </ProgressBar>
                            </div>
                          )}
                        </div>
                      </motion.div>
                    )}
                  </AnimatePresence>
                </div>
              )}
              {selectedTab === "about" && (
                <div className="flex flex-col gap-6">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-4">
                      <div>
                        <p className="font-medium text-lg">
                          {t("aboutcard.title")}
                        </p>
                        <p className="text-xs text-muted dark:text-zinc-400">
                          {t("aboutcard.description", { name: "LeviMC" })} ·{" "}
                          {t("aboutcard.font", { name: "MiSans" })}
                        </p>
                      </div>
                    </div>
                    <div className="flex items-center gap-1">
                      <Tooltip>
                        <Button
                          isIconOnly
                          aria-label={t("audit.mods.github")}
                          onPress={() =>
                            Browser.OpenURL("https://github.com/liteldev")
                          }
                          variant={"ghost"}
                          className={"rounded-full"}
                        >
                          <FaGithub size={20} />
                        </Button>
                        <Tooltip.Content>
                          {t("audit.mods.github")}
                        </Tooltip.Content>
                      </Tooltip>
                      <Tooltip>
                        <Button
                          isIconOnly
                          aria-label={t("audit.mods.discord")}
                          onPress={() =>
                            Browser.OpenURL("https://discord.gg/v5R5P4vRZk")
                          }
                          variant={"ghost"}
                          className={"rounded-full"}
                        >
                          <FaDiscord size={20} />
                        </Button>
                        <Tooltip.Content>
                          {t("audit.mods.discord")}
                        </Tooltip.Content>
                      </Tooltip>
                    </div>
                  </div>
                </div>
              )}
            </Card.Content>
          </Card>
        </motion.div>
      </div>

      {/* Process Management Modal */}
      <UnifiedModal
        size="wide"
        isOpen={processModalOpen}
        onOpenChange={(open) => {
          if (terminatingProcess === null) setProcessModalOpen(open);
        }}
        isDismissable={terminatingProcess === null}
        scrollBehavior="inside"
        type="primary"
        title={
          <div className="flex flex-col gap-1">
            <span>{t("settings.process.title")}</span>
            <span className="text-sm font-normal text-muted dark:text-zinc-400">
              {t("settings.process.desc")}
            </span>
          </div>
        }
        icon={<FaList className="w-6 h-6" />}
        footer={
          <ModalAction isDisabled={terminatingProcess !== null} onPress={() => setProcessModalOpen(false)} variant="secondary">
            {t("common.close")}
          </ModalAction>
        }
      >
        <div className="flex items-center justify-end mb-4 gap-2">
          <Button
            size="sm"
            onPress={() => void refreshProcesses()}
            isDisabled={terminatingProcess !== null}
            variant={"secondary"}
            isPending={scanningProcesses}
          >
            {({ isPending }) => (
              <>
                <Spinner
                  size="sm"
                  color="current"
                  className={isPending ? "" : "hidden"}
                />
                {t("settings.process.scan")}
              </>
            )}
          </Button>
          {processes.length > 0 && (
            <Button
              size="sm"
              onPress={() => setKillTarget({ label: t("audit.usability.process_all", { count: processes.length }) })}
              isDisabled={scanningProcesses || terminatingProcess !== null}
              variant={"danger-soft"}
            >
              {t("settings.process.kill_all")}
            </Button>
          )}
        </div>
        {processError && <ModalNotice role="alert" tone="danger">{t("audit.usability.process_failed", { message: processError })}</ModalNotice>}
        <div className="flex flex-col gap-4" aria-busy={scanningProcesses}>
          {scanningProcesses && processes.length === 0 ? (
            <div role="status" className="flex justify-center gap-2 py-8"><Spinner size="sm" />{t("common.loading")}</div>
          ) : processes.length === 0 && !processError ? (
            <div className="text-center py-8 text-muted dark:text-zinc-400">
              {t("settings.process.no_process")}
            </div>
          ) : (
            <div className="flex flex-col gap-2">
              {processes.map((p) => (
                <ModalPanel
                  key={p.pid}
                  className="flex items-center justify-between"
                >
                  <div className="flex flex-col gap-1 overflow-hidden">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-sm bg-surface-tertiary/50 px-1.5 rounded text-foreground">
                        {p.pid}
                      </span>
                      {p.isLauncher && p.versionName ? (
                        <Chip
                          size="sm"
                          variant="soft"
                          color={"success"}
                          className={"h-5 text-[10px]"}
                        >
                          <Chip.Label>{p.versionName}</Chip.Label>
                        </Chip>
                      ) : (
                        <span className="text-sm font-medium">
                          Minecraft.Windows.exe
                        </span>
                      )}
                    </div>
                    <span
                      className="text-xs text-muted truncate max-w-[400px]"
                      title={p.exePath}
                    >
                      {p.exePath}
                    </span>
                  </div>
                  <Button
                    size="sm"
                    onPress={() => setKillTarget({ pid: p.pid, label: `${p.versionName || "Minecraft.Windows.exe"} (PID ${p.pid})` })}
                    isDisabled={scanningProcesses || terminatingProcess !== null}
                    variant={"ghost"}
                    className={"text-rose-700 dark:text-rose-300"}
                  >
                    {t("settings.process.kill")}
                  </Button>
                </ModalPanel>
              ))}
            </div>
          )}
        </div>
      </UnifiedModal>

      <UnifiedModal
        size="wide"
        isOpen={instanceBackupWarningOpen}
        onOpenChange={setInstanceBackupWarningOpen}
        type="warning"
        title={t("settings.experimental.instance_backup.warning.title")}
        isDismissable={false}
        showConfirmButton={false}
        showCancelButton={false}
        footer={
          <div className="flex w-full justify-end gap-2">
            <ModalAction onPress={closeInstanceBackupWarning} variant="secondary">
              {t("common.cancel")}
            </ModalAction>
            <ModalAction
              isDisabled={instanceBackupWarningCountdown > 0}
              onPress={confirmInstanceBackupWarning}
              variant={"primary"}
            >
              {instanceBackupWarningCountdown > 0
                ? `${t("settings.experimental.instance_backup.warning.confirm")} (${instanceBackupWarningCountdown}s)`
                : t("settings.experimental.instance_backup.warning.confirm")}
            </ModalAction>
          </div>
        }
      >
        <ModalDescription className="space-y-4">
          <p className="font-medium text-amber-700 dark:text-amber-400">
            {t("settings.experimental.instance_backup.warning.body_1")}
          </p>
          <p>{t("settings.experimental.instance_backup.warning.body_2")}</p>
          <p>{t("settings.experimental.instance_backup.warning.body_3")}</p>
        </ModalDescription>
      </UnifiedModal>

      {/* LIP Install Progress */}
      <UnifiedModal
        size="standard"
        isOpen={lipProgressDisclosure.isOpen}
        onOpenChange={lipProgressDisclosure.setOpen}
        isDismissable={false}
        type={lipError ? "error" : "info"}
        title={lipError ? t("common.error") : t("settings.lip.installing")}
        icon={lipError ? undefined : <FaDownload className="w-6 h-6" />}
        confirmText={lipError ? t("common.close") : undefined}
        onConfirm={lipError ? () => lipProgressDisclosure.close() : undefined}
        footer={
          lipError ? undefined : (
            <>
              <ModalAction
                onPress={lipProgressDisclosure.close}
                isDisabled={!installingLip}
                variant="secondary"
              >
                {t("common.hide")}
              </ModalAction>
              <ModalAction
                onPress={lipProgressDisclosure.close}
                isDisabled={installingLip && !lipError}
                variant={"primary"}
              >
                {t("common.ok")}
              </ModalAction>
            </>
          )
        }
      >
        {lipError ? (
          <div className="text-danger">
            {t(
              `settings.lip.error.${lipError
                .toLowerCase()
                .replace(/^err_/, "")}`,
            )}
          </div>
        ) : (
          <div className="flex flex-col gap-3">
            <div className="h-2 w-full rounded bg-surface-tertiary overflow-hidden">
              <div
                className="h-full bg-accent"
                style={{ width: `${lipProgress.percentage}%` }}
              />
            </div>
            <div className="text-sm text-muted dark:text-zinc-400">
              {t(`settings.lip.status.${lipStatus}`)}
              {lipProgress.total > 0 ? (
                <span className="ml-2">
                  {`${(lipProgress.current / (1024 * 1024)).toFixed(2)} MB / ${(lipProgress.total / (1024 * 1024)).toFixed(2)} MB`}
                </span>
              ) : (
                ` (${lipProgress.percentage.toFixed(0)}%)`
              )}
            </div>
          </div>
        )}
      </UnifiedModal>

      <UnifiedModal
        size="standard"
        isOpen={killTarget !== null}
        onOpenChange={(open) => { if (!open && terminatingProcess === null) setKillTarget(null); }}
        isDismissable={terminatingProcess === null}
        type="warning"
        title={t("audit.usability.process_confirm")}
        showCancelButton
        cancelText={t("common.cancel")}
        confirmText={t("settings.process.kill")}
        confirmButtonProps={{ variant: "danger", isPending: terminatingProcess !== null }}
        cancelButtonProps={{ isDisabled: terminatingProcess !== null }}
        onCancel={() => setKillTarget(null)}
        onConfirm={async () => {
          if (!killTarget) return;
          const ok = killTarget.pid === undefined
            ? await handleKillAllProcesses()
            : await handleKillProcess(killTarget.pid);
          setKillTarget(null);
          if (ok) toast.success(t("audit.usability.process_success"));
        }}
      >
        <ModalDescription>{t("audit.usability.process_warning", { target: killTarget?.label })}</ModalDescription>
      </UnifiedModal>

      <UnifiedModal
        size="standard"
        isOpen={unsavedOpen}
        onOpenChange={(open) => {
          if (!savingBaseRoot) unsavedOnOpenChange(open);
        }}
        type="warning"
        title={t("settings.unsaved.title")}
        isDismissable={!savingBaseRoot}
        footer={
          <div className="flex w-full flex-wrap justify-end gap-2">
            <ModalAction variant="secondary" isDisabled={savingBaseRoot} onPress={unsavedOnClose}>{t("audit.usability.continue_editing")}</ModalAction>
            <ModalAction variant="danger-soft" isDisabled={savingBaseRoot} onPress={() => {
              setNewBaseRoot(baseRoot);
              setPathError("");
              continuePendingNavigation();
            }}>{t("audit.usability.discard_leave")}</ModalAction>
            <ModalAction variant="primary" isPending={savingBaseRoot} isDisabled={!newBaseRoot.trim() || !baseRootWritable} onPress={async () => {
              if (await persistBasePath()) continuePendingNavigation();
            }}>{t("settings.unsaved.save")}</ModalAction>
          </div>
        }
      >
        <ModalDescription>
          {t("settings.unsaved.body")}
        </ModalDescription>
        {pathError && (
          <p role="alert" className="text-sm text-danger mt-2">{pathError}</p>
        )}
        {!baseRootWritable && (
          <div className="text-xs text-rose-500 mt-1">
            {t("settings.body.paths.not_writable")}
          </div>
        )}
      </UnifiedModal>

      <UnifiedModal
        size="standard"
        isOpen={resetOpen}
        onOpenChange={(open) => {
          if (!savingBaseRoot) resetOnOpenChange(open);
        }}
        type="error"
        title={t("settings.reset.confirm.title")}
        cancelText={t("common.cancel")}
        confirmText={t("common.confirm")}
        showCancelButton
        isDismissable={!savingBaseRoot}
        confirmButtonProps={{ isPending: savingBaseRoot, variant: "danger" }}
        cancelButtonProps={{ isDisabled: savingBaseRoot }}
        onCancel={() => resetOnClose()}
        onConfirm={async () => {
          if (await persistBasePath(true)) resetOnClose();
        }}
      >
        <ModalDescription>
          {t("settings.reset.confirm.body")}
        </ModalDescription>
        {pathError && (
          <p role="alert" className="text-sm text-danger mt-2">{pathError}</p>
        )}
      </UnifiedModal>
    </PageContainer>
  );
};

export default SettingsPage;
