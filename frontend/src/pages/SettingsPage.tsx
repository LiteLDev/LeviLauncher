import React, { useEffect, useState, useMemo } from "react";
import { useNavigate, useLocation } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { motion, AnimatePresence } from "framer-motion";
import {
  Card,
  CardBody,
  Button,
  Chip,
  Input,
  Divider,
  Dropdown,
  DropdownTrigger,
  DropdownMenu,
  DropdownItem,
  Spinner,
  Progress,
  ModalContent,
  useDisclosure,
  Switch,
  Tabs,
  Tab,
  Slider,
} from "@heroui/react";

import { useTheme } from "next-themes";
import { RxUpdate, RxDesktop } from "react-icons/rx";
import {
  FaGithub,
  FaDiscord,
  FaDownload,
  FaCogs,
  FaList,
} from "react-icons/fa";
import {
  LuHardDrive,
  LuPalette,
  LuSun,
  LuMoon,
  LuMonitor,
} from "react-icons/lu";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import {
  GetAppVersion,
  CheckUpdate,
  GetLanguageNames,
  GetBaseRoot,
  SetBaseRoot,
  GetInstallerDir,
  GetVersionsDir,
  CanWriteToDir,
  IsGDKInstalled,
  StartGDKDownload,
  CancelGDKDownload,
  InstallGDKFromZip,
  GetDisableDiscordRPC,
  SetDisableDiscordRPC,
  GetEnableBetaUpdates,
  SetEnableBetaUpdates,
  ResetBaseRoot,
  InstallLip,
  GetLipVersion,
  IsLipInstalled,
  GetLatestLipVersion,
  ListMinecraftProcesses,
  KillProcess,
  KillAllMinecraftProcesses,
} from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { Browser, Events, Dialogs } from "@wailsio/runtime";
import * as types from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { UnifiedModal } from "@/components/UnifiedModal";
import { PageHeader } from "@/components/PageHeader";
import { normalizeLanguage } from "@/utils/i18nUtils";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { THEMES, THEME_GROUPS } from "@/constants/themes";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { CustomColorPicker } from "@/components/CustomColorPicker";
import { useThemeManager, ThemeMode } from "@/utils/useThemeManager";

export const SettingsPage: React.FC = () => {
  const { t, i18n } = useTranslation();
  const { theme, setTheme } = useTheme();
  const hasBackend = minecraft !== undefined;
  const [appVersion, setAppVersion] = useState("");
  const [checkingUpdate, setCheckingUpdate] = useState(false);
  const [updating, setUpdating] = useState(false);
  const [newVersion, setNewVersion] = useState("");
  const [hasUpdate, setHasUpdate] = useState(false);
  const [changelog, setChangelog] = useState<string>("");
  const [langNames, setLangNames] = useState<Array<types.LanguageJson>>([]);
  const [selectedLang, setSelectedLang] = useState<string>("");
  const [languageChanged, setLanguageChanged] = useState<boolean>(false);
  const [baseRoot, setBaseRoot] = useState<string>("");
  const [installerDir, setInstallerDir] = useState<string>("");
  const [versionsDir, setVersionsDir] = useState<string>("");
  const [newBaseRoot, setNewBaseRoot] = useState<string>("");
  const [savingBaseRoot, setSavingBaseRoot] = useState<boolean>(false);
  const [baseRootWritable, setBaseRootWritable] = useState<boolean>(true);
  const [discordRpcEnabled, setDiscordRpcEnabled] = useState<boolean>(true);
  const [enableBetaUpdates, setEnableBetaUpdates] = useState<boolean>(false);
  const [gdkInstalled, setGdkInstalled] = useState<boolean>(false);
  const [gdkDlProgress, setGdkDlProgress] = useState<{
    downloaded: number;
    total: number;
    dest?: string;
  } | null>(null);
  const [gdkDlSpeed, setGdkDlSpeed] = useState<number>(0);
  const [gdkDlStatus, setGdkDlStatus] = useState<string>("");
  const [gdkDlError, setGdkDlError] = useState<string>("");
  const gdkProgressDisclosure = useDisclosure();
  const gdkLicenseDisclosure = useDisclosure();
  const gdkInstallDisclosure = useDisclosure();
  const [gdkLicenseAccepted, setGdkLicenseAccepted] = useState<boolean>(false);

  const [selectedTab, setSelectedTab] = useState<string>("general");

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

  const [lightBackgroundImage, setLightBackgroundImage] = useState<string>(
    () => {
      try {
        const saved = localStorage.getItem("app.lightBackgroundImage");
        if (saved !== null) return saved;
        return localStorage.getItem("app.backgroundImage") || "";
      } catch {
        return "";
      }
    },
  );

  const [darkBackgroundImage, setDarkBackgroundImage] = useState<string>(() => {
    try {
      const saved = localStorage.getItem("app.darkBackgroundImage");
      if (saved !== null) return saved;
      return localStorage.getItem("app.backgroundImage") || "";
    } catch {
      return "";
    }
  });

  const [lightBackgroundBlur, setLightBackgroundBlur] = useState<number>(() => {
    try {
      const saved = localStorage.getItem("app.lightBackgroundBlur");
      if (saved !== null) return Number(saved);
      return Number(localStorage.getItem("app.backgroundBlur") || "0");
    } catch {
      return 0;
    }
  });
  const [darkBackgroundBlur, setDarkBackgroundBlur] = useState<number>(() => {
    try {
      const saved = localStorage.getItem("app.darkBackgroundBlur");
      if (saved !== null) return Number(saved);
      return Number(localStorage.getItem("app.backgroundBlur") || "0");
    } catch {
      return 0;
    }
  });

  const [lightBackgroundBrightness, setLightBackgroundBrightness] =
    useState<number>(() => {
      try {
        const item =
          localStorage.getItem("app.lightBackgroundBrightness") ||
          localStorage.getItem("app.backgroundBrightness");
        return item !== null ? Number(item) : 100;
      } catch {
        return 100;
      }
    });
  const [darkBackgroundBrightness, setDarkBackgroundBrightness] =
    useState<number>(() => {
      try {
        const item =
          localStorage.getItem("app.darkBackgroundBrightness") ||
          localStorage.getItem("app.backgroundBrightness");
        return item !== null ? Number(item) : 100;
      } catch {
        return 100;
      }
    });

  const [lightBackgroundOpacity, setLightBackgroundOpacity] = useState<number>(
    () => {
      try {
        const item =
          localStorage.getItem("app.lightBackgroundOpacity") ||
          localStorage.getItem("app.backgroundOpacity");
        return item !== null ? Number(item) : 100;
      } catch {
        return 100;
      }
    },
  );
  const [darkBackgroundOpacity, setDarkBackgroundOpacity] = useState<number>(
    () => {
      try {
        const item =
          localStorage.getItem("app.darkBackgroundOpacity") ||
          localStorage.getItem("app.backgroundOpacity");
        return item !== null ? Number(item) : 100;
      } catch {
        return 100;
      }
    },
  );

  const [lightBackgroundBaseOpacity, setLightBackgroundBaseOpacity] =
    useState<number>(() => {
      try {
        const item =
          localStorage.getItem("app.lightBackgroundBaseOpacity") ||
          localStorage.getItem("app.backgroundBaseOpacity");
        return item !== null ? Number(item) : 100;
      } catch {
        return 100;
      }
    });
  const [darkBackgroundBaseOpacity, setDarkBackgroundBaseOpacity] =
    useState<number>(() => {
      try {
        const item =
          localStorage.getItem("app.darkBackgroundBaseOpacity") ||
          localStorage.getItem("app.backgroundBaseOpacity");
        return item !== null ? Number(item) : 100;
      } catch {
        return 100;
      }
    });

  const [lightBackgroundUseTheme, setLightBackgroundUseTheme] =
    useState<boolean>(() => {
      try {
        const saved = localStorage.getItem("app.lightBackgroundUseTheme");
        if (saved !== null) return saved === "true";
        return localStorage.getItem("app.backgroundUseTheme") === "true";
      } catch {
        return false;
      }
    });
  const [darkBackgroundUseTheme, setDarkBackgroundUseTheme] = useState<boolean>(
    () => {
      try {
        const saved = localStorage.getItem("app.darkBackgroundUseTheme");
        if (saved !== null) return saved === "true";
        return localStorage.getItem("app.backgroundUseTheme") === "true";
      } catch {
        return false;
      }
    },
  );

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
      return (
        localStorage.getItem("app.darkThemeColor") ||
        localStorage.getItem("app.themeColor") ||
        "emerald"
      );
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
      const currentTheme = resolvedTheme || theme || "light";
      return currentTheme === "dark" ? "dark" : "light";
    },
  );

  useEffect(() => {
    if (resolvedTheme === "dark" || resolvedTheme === "light") {
      setThemeSettingMode(resolvedTheme);
    }
  }, [resolvedTheme]);

  const [backgroundImageError, setBackgroundImageError] = useState(false);
  const [previewBgData, setPreviewBgData] = useState<string>("");

  const [lipInstalled, setLipInstalled] = useState<boolean>(false);
  const [lipVersion, setLipVersion] = useState<string>("");
  const [lipLatestVersion, setLipLatestVersion] = useState<string>("");
  const [installingLip, setInstallingLip] = useState<boolean>(false);
  const [lipStatus, setLipStatus] = useState<string>("");
  const [lipProgress, setLipProgress] = useState<{
    percentage: number;
    current: number;
    total: number;
  }>({ percentage: 0, current: 0, total: 0 });
  const [lipError, setLipError] = useState<string>("");
  const lipProgressDisclosure = useDisclosure();

  const [processModalOpen, setProcessModalOpen] = useState(false);
  const [processes, setProcesses] = useState<types.ProcessInfo[]>([]);
  const [scanningProcesses, setScanningProcesses] = useState(false);

  const [loadingSunTimes, setLoadingSunTimes] = useState(false);

  const refreshSunTimes = async () => {
    setLoadingSunTimes(true);
    await fetchSunTimes();
    setLoadingSunTimes(false);
  };

  const refreshProcesses = async () => {
    setScanningProcesses(true);
    try {
      const list = await ListMinecraftProcesses();
      setProcesses(list || []);
    } finally {
      setScanningProcesses(false);
    }
  };

  const handleKillProcess = async (pid: number) => {
    try {
      await KillProcess(pid);
      await refreshProcesses();
    } catch {}
  };

  const handleKillAllProcesses = async () => {
    try {
      await KillAllMinecraftProcesses();
      await refreshProcesses();
    } catch {}
  };

  useEffect(() => {
    if (processModalOpen) {
      refreshProcesses();
    }
  }, [processModalOpen]);

  const hasLipUpdate = useMemo(() => {
    if (!lipInstalled || !lipLatestVersion || !lipVersion) return false;
    return lipLatestVersion.replace(/^v/, "") !== lipVersion.replace(/^v/, "");
  }, [lipInstalled, lipLatestVersion, lipVersion]);

  const navigate = useNavigate();
  const location = useLocation();
  const {
    isOpen: unsavedOpen,
    onOpen: unsavedOnOpen,
    onOpenChange: unsavedOnOpenChange,
    onClose: unsavedOnClose,
  } = useDisclosure();
  const [pendingNavPath, setPendingNavPath] = useState<string>("");
  const {
    isOpen: resetOpen,
    onOpen: resetOnOpen,
    onOpenChange: resetOnOpenChange,
    onClose: resetOnClose,
  } = useDisclosure();

  useEffect(() => {
    const handler = (ev: any) => {
      try {
        let targetPath = ev?.detail?.path;
        if (targetPath === -1) targetPath = "-1";
        targetPath = String(targetPath || "");
        const hasUnsaved = !!newBaseRoot && newBaseRoot !== baseRoot;
        if (!targetPath || targetPath === location.pathname) return;
        if (hasUnsaved) {
          setPendingNavPath(targetPath);
          unsavedOnOpen();
          return;
        }
        if (targetPath === "-1") {
          navigate(-1);
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
          if (hasBackend) {
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
              setEnableBetaUpdates(enabled);
            } catch {}
            try {
              const ok = await IsGDKInstalled();
              setGdkInstalled(Boolean(ok));
            } catch {}
          }
        } catch {}
      })
      .catch(() => {});
  }, []);

  useEffect(() => {
    setBaseRootWritable(true);
  }, [newBaseRoot]);

  useEffect(() => {
    setBackgroundImageError(false);
  }, [lightBackgroundImage, darkBackgroundImage, themeSettingMode]);

  useEffect(() => {
    const currentImg =
      themeSettingMode === "light" ? lightBackgroundImage : darkBackgroundImage;

    if (!currentImg) {
      setPreviewBgData("");
      return;
    }

    if (currentImg.startsWith("data:") || currentImg.startsWith("http")) {
      setPreviewBgData(currentImg);
      return;
    }

    (minecraft as any)
      .GetImageBase64?.(currentImg)
      .then((res: string) => {
        if (res) {
          setPreviewBgData(res);
          setBackgroundImageError(false);
        } else {
          setBackgroundImageError(true);
        }
      })
      .catch(() => {
        setBackgroundImageError(true);
      });
  }, [lightBackgroundImage, darkBackgroundImage, themeSettingMode]);

  useEffect(() => {
    if (location.state && (location.state as any).tab) {
      setSelectedTab((location.state as any).tab);
    }
  }, [location.state]);

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
      navigate("/updating", { replace: true });
    } finally {
      setUpdating(false);
    }
  };

  useEffect(() => {
    if (!hasBackend) return;
    const offs: (() => void)[] = [];
    try {
      const speedRef: { ts: number; bytes: number; ema: number } = {
        ts: 0,
        bytes: 0,
        ema: 0,
      } as any;
      offs.push(
        Events.On("gdk_download_progress", (event) => {
          const downloaded = Number(event?.data?.Downloaded || 0);
          const total = Number(event?.data?.Total || 0);
          const dest = String(event?.data?.Dest || "");
          setGdkDlProgress({ downloaded, total, dest });
          try {
            const now = Date.now();
            if (speedRef.ts > 0) {
              const dt = (now - speedRef.ts) / 1000;
              const db = downloaded - speedRef.bytes;
              const inst = dt > 0 && db >= 0 ? db / dt : 0;
              const alpha = 0.25;
              speedRef.ema =
                alpha * inst + (1 - alpha) * (speedRef.ema || inst);
              setGdkDlSpeed(speedRef.ema);
            }
            speedRef.ts = now;
            speedRef.bytes = downloaded;
          } catch {}
        }),
      );
      offs.push(
        Events.On("gdk_download_status", (event) => {
          const s = String(event?.data || "");
          setGdkDlStatus(s);
          if (s === "started" || s === "resumed" || s === "cancelled") {
            setGdkDlError("");
            try {
              (window as any).__gdkDlLast = null;
            } catch {}
            setGdkDlSpeed(0);
          }
        }),
      );
      offs.push(
        Events.On("gdk_download_error", (event) => {
          setGdkDlError(String(event?.data || ""));
        }),
      );
      offs.push(
        Events.On("gdk_download_done", async (event) => {
          const dest = String(event?.data || gdkDlProgress?.dest || "");
          gdkProgressDisclosure.onClose();
          try {
            gdkInstallDisclosure.onOpen();
            await InstallGDKFromZip(dest);
          } catch {}
        }),
      );
      offs.push(
        Events.On("gdk_install_done", (_event) => {
          gdkInstallDisclosure.onClose();
          setTimeout(async () => {
            try {
              const ok = await IsGDKInstalled();
              setGdkInstalled(Boolean(ok));
            } catch {}
          }, 500);
        }),
      );
      offs.push(
        Events.On("gdk_install_error", (event) => {
          gdkInstallDisclosure.onClose();
          setGdkDlError(String(event?.data || ""));
        }),
      );
    } catch {}
    return () => {
      for (const off of offs) {
        try {
          off();
        } catch {}
      }
    };
  }, [hasBackend]);

  useEffect(() => {
    if (!hasBackend) return;
    IsLipInstalled().then((ok) => {
      setLipInstalled(Boolean(ok));
      if (ok) {
        GetLipVersion().then((v) => setLipVersion(String(v || "")));
      }
    });
    GetLatestLipVersion()
      .then((v) => setLipLatestVersion(String(v || "")))
      .catch(() => {});

    const offs: (() => void)[] = [];
    offs.push(
      Events.On("lip_install_status", (e) =>
        setLipStatus(String(e?.data || "")),
      ),
    );
    offs.push(
      Events.On("lip_install_progress", (e) => {
        const data = e?.data as any;
        setLipProgress({
          percentage: Number(data?.percentage || 0),
          current: Number(data?.current || 0),
          total: Number(data?.total || 0),
        });
      }),
    );
    offs.push(
      Events.On("lip_install_done", (e) => {
        setInstallingLip(false);
        setLipStatus("done");
        setLipProgress({ percentage: 100, current: 0, total: 0 });
        setLipInstalled(true);
        setLipVersion(String(e?.data || ""));
        setLipError("");
      }),
    );
    offs.push(
      Events.On("lip_install_error", (e) => {
        setInstallingLip(false);
        setLipStatus("");
        setLipError(String(e?.data || ""));
      }),
    );
    return () => offs.forEach((off) => off());
  }, [hasBackend]);

  return (
    <PageContainer className="relative" animate={false}>
      <div className="flex flex-col gap-6">
        {/* Header Card */}
        <motion.div
          initial={{ opacity: 0, y: -20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.4 }}
        >
          <Card className={LAYOUT.GLASS_CARD.BASE}>
            <CardBody className="p-6">
              <PageHeader
                title={t("settings.header.title")}
                description={t("settings.header.content")}
              />
              <Tabs
                aria-label="Settings Tabs"
                selectedKey={selectedTab}
                onSelectionChange={(k) => setSelectedTab(k as string)}
                classNames={{
                  ...COMPONENT_STYLES.tabs,
                  base: "mt-4",
                }}
              >
                <Tab key="general" title={t("settings.tabs.general")} />
                <Tab
                  key="personalization"
                  title={t("settings.tabs.personalization")}
                />
                <Tab key="components" title={t("settings.tabs.components")} />
                <Tab key="others" title={t("settings.tabs.others")} />
                <Tab key="updates" title={t("settings.tabs.updates")} />
                <Tab key="about" title={t("settings.tabs.about")} />
              </Tabs>
            </CardBody>
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
            <CardBody className="p-6">
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
                          variant="light"
                          radius="full"
                          onPress={() => resetOnOpen()}
                        >
                          {t("settings.body.paths.reset")}
                        </Button>
                        <Button
                          color="primary"
                          radius="full"
                          isDisabled={!newBaseRoot || !baseRootWritable}
                          isLoading={savingBaseRoot}
                          className="bg-primary-600 hover:bg-primary-500 text-white font-bold shadow-lg shadow-primary-900/20"
                          onPress={async () => {
                            setSavingBaseRoot(true);
                            try {
                              const ok = await CanWriteToDir(newBaseRoot);
                              if (!ok) {
                                setBaseRootWritable(false);
                              } else {
                                const err = await SetBaseRoot(newBaseRoot);
                                if (!err) {
                                  const br = await GetBaseRoot();
                                  setBaseRoot(String(br || ""));
                                  const id = await GetInstallerDir();
                                  setInstallerDir(String(id || ""));
                                  const vd = await GetVersionsDir();
                                  setVersionsDir(String(vd || ""));
                                }
                              }
                            } catch {}
                            setSavingBaseRoot(false);
                          }}
                        >
                          {t("settings.body.paths.apply")}
                        </Button>
                      </div>
                    </div>

                    <div className="space-y-4">
                      <Input
                        label={t("settings.body.paths.base_root") as string}
                        value={newBaseRoot}
                        onValueChange={setNewBaseRoot}
                        radius="lg"
                        variant="bordered"
                        classNames={COMPONENT_STYLES.input}
                        endContent={
                          <Button
                            size="sm"
                            variant="flat"
                            radius="full"
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
                                const result = await Dialogs.OpenFile(options);
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
                          >
                            {t("common.browse")}
                          </Button>
                        }
                      />
                      {newBaseRoot &&
                      newBaseRoot !== baseRoot &&
                      baseRootWritable ? (
                        <div
                          className="text-tiny text-warning-500 px-1"
                          title={newBaseRoot}
                        >
                          {t("settings.body.paths.base_root") +
                            ": " +
                            newBaseRoot}
                        </div>
                      ) : null}
                      {!baseRootWritable ? (
                        <div className="text-tiny text-danger-500 px-1">
                          {t("settings.body.paths.not_writable")}
                        </div>
                      ) : null}

                      <div className="grid grid-cols-1 gap-2 pt-2">
                        <div className="p-3 rounded-xl bg-default-100/50 dark:bg-zinc-800/30 border border-default-200/50 dark:border-white/5">
                          <div
                            className="text-tiny text-default-500 dark:text-zinc-400 flex items-center gap-2 truncate"
                            title={installerDir || "-"}
                          >
                            <LuHardDrive size={14} />
                            <span className="font-medium">
                              {t("settings.body.paths.installer")}:
                            </span>
                            <span className="opacity-70">
                              {installerDir || "-"}
                            </span>
                          </div>
                        </div>
                        <div className="p-3 rounded-xl bg-default-100/50 dark:bg-zinc-800/30 border border-default-200/50 dark:border-white/5">
                          <div
                            className="text-tiny text-default-500 dark:text-zinc-400 flex items-center gap-2 truncate"
                            title={versionsDir || "-"}
                          >
                            <LuHardDrive size={14} />
                            <span className="font-medium">
                              {t("settings.body.paths.versions")}:
                            </span>
                            <span className="opacity-70">
                              {versionsDir || "-"}
                            </span>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>

                  <Divider className="bg-default-200/50" />

                  {/* Language */}
                  <div className="flex items-center justify-between">
                    <div className="flex flex-col">
                      <p className="font-medium">
                        {t("settings.body.language.name")}
                      </p>
                      <p className="text-tiny text-default-500 dark:text-zinc-400">
                        {langNames.find((l) => l.code === selectedLang)
                          ?.language || selectedLang}
                      </p>
                      {languageChanged && (
                        <div className="text-tiny text-warning-500 mt-1">
                          {t("settings.lang.changed")}
                        </div>
                      )}
                    </div>
                    <Dropdown classNames={COMPONENT_STYLES.dropdown}>
                      <DropdownTrigger>
                        <Button radius="full" variant="bordered">
                          {t("settings.body.language.button")}
                        </Button>
                      </DropdownTrigger>
                      <DropdownMenu
                        aria-label="Language selection"
                        variant="flat"
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
                          <DropdownItem
                            key={lang.code}
                            textValue={lang.language}
                          >
                            {lang.language}
                          </DropdownItem>
                        ))}
                      </DropdownMenu>
                    </Dropdown>
                  </div>

                  <Divider className="bg-default-200/50" />

                  {/* Discord RPC */}
                  <div className="flex items-center justify-between">
                    <div className="flex flex-col gap-1">
                      <p className="font-medium">
                        {t("settings.discord_rpc.title")}
                      </p>
                      <p className="text-tiny text-default-500 dark:text-zinc-400">
                        {t("settings.discord_rpc.desc")}
                      </p>
                    </div>
                    <Switch
                      size="sm"
                      isSelected={discordRpcEnabled}
                      onValueChange={(isSelected: boolean) => {
                        setDiscordRpcEnabled(isSelected);
                        SetDisableDiscordRPC(!isSelected);
                      }}
                      classNames={{
                        wrapper: "group-data-[selected=true]:bg-primary-500",
                      }}
                    />
                  </div>
                </div>
              )}

              {selectedTab === "personalization" && (
                <div className="flex flex-col gap-8">
                  {/* Global Settings Group */}
                  <div className="flex flex-col gap-4">
                    <div className="flex items-center gap-2 mb-1">
                      <div className="w-1 h-4 bg-primary-500 rounded-full" />
                      <p className="text-sm font-bold text-default-600 uppercase tracking-wider">
                        {t("settings.appearance.global_config")}
                      </p>
                    </div>
                    <div className="flex flex-col gap-0">
                      {/* Theme Mode Switcher */}
                      <div className="py-4 flex flex-col gap-4 border-b border-default-200/50">
                        <div className="flex flex-col gap-4">
                          <div className="flex flex-col gap-1">
                            <p className="font-medium text-default-700 dark:text-zinc-200">
                              {t("settings.appearance.theme_mode")}
                            </p>
                            <p className="text-tiny text-default-500 dark:text-zinc-400">
                              {t("settings.appearance.theme_mode_desc")}
                            </p>
                          </div>
                          <div className="w-full overflow-x-auto scrollbar-hide">
                            <Tabs
                              size="sm"
                              selectedKey={themeMode}
                              onSelectionChange={(key) => {
                                const val = key as ThemeMode;
                                setThemeMode(val);
                              }}
                              classNames={{
                                ...COMPONENT_STYLES.tabs,
                                base: "w-full",
                                tabList: "w-full flex-wrap sm:flex-nowrap",
                              }}
                            >
                              <Tab
                                key="light"
                                title={
                                  <div className="flex items-center gap-2">
                                    <LuSun size={14} />
                                    <span>
                                      {t("settings.appearance.theme_light")}
                                    </span>
                                  </div>
                                }
                              />
                              <Tab
                                key="dark"
                                title={
                                  <div className="flex items-center gap-2">
                                    <LuMoon size={14} />
                                    <span>
                                      {t("settings.appearance.theme_dark")}
                                    </span>
                                  </div>
                                }
                              />
                              <Tab
                                key="schedule"
                                title={
                                  <div className="flex items-center gap-2">
                                    <LuHardDrive size={14} />
                                    <span>
                                      {t("settings.appearance.theme_schedule")}
                                    </span>
                                  </div>
                                }
                              />
                              <Tab
                                key="auto"
                                title={
                                  <div className="flex items-center gap-2">
                                    <RxDesktop size={14} />
                                    <span>
                                      {t("settings.appearance.theme_auto")}
                                    </span>
                                  </div>
                                }
                              />
                              <Tab
                                key="system"
                                title={
                                  <div className="flex items-center gap-2">
                                    <LuMonitor size={14} />
                                    <span>
                                      {t("settings.appearance.theme_system")}
                                    </span>
                                  </div>
                                }
                              />
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
                              <div className="flex flex-col gap-4 p-4 mt-2 rounded-2xl bg-default-100/50 border border-default-200/50">
                                <div className="flex flex-col gap-1">
                                  <p className="text-tiny font-bold text-default-600 uppercase tracking-wider">
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
                              <div className="flex flex-col gap-4 p-4 mt-2 rounded-2xl bg-default-100/50 border border-default-200/50">
                                <div className="flex flex-col gap-1">
                                  <p className="text-tiny font-bold text-default-600 uppercase tracking-wider">
                                    {t(
                                      "settings.appearance.theme_schedule_desc",
                                    )}
                                  </p>
                                </div>
                                <div className="flex gap-4">
                                  <Input
                                    type="time"
                                    label={t(
                                      "settings.appearance.theme_start_time",
                                    )}
                                    size="sm"
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
                                    classNames={COMPONENT_STYLES.input}
                                  />
                                  <Input
                                    type="time"
                                    label={t(
                                      "settings.appearance.theme_end_time",
                                    )}
                                    size="sm"
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
                                    classNames={COMPONENT_STYLES.input}
                                  />
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
                              <div className="flex flex-col gap-4 p-4 mt-2 rounded-2xl bg-default-100/50 border border-default-200/50">
                                <div className="flex flex-col gap-1">
                                  <p className="text-tiny font-bold text-default-600 uppercase tracking-wider">
                                    {t("settings.appearance.theme_auto_desc")}
                                  </p>
                                </div>

                                {loadingSunTimes ? (
                                  <div className="flex items-center gap-2 py-2">
                                    <Spinner size="sm" color="primary" />
                                    <p className="text-tiny text-default-400">
                                      {t("settings.appearance.calculating")}
                                    </p>
                                  </div>
                                ) : sunTimes ? (
                                  <div className="flex flex-col gap-3">
                                    <div className="flex items-center gap-6">
                                      <div className="flex items-center gap-2">
                                        <div className="p-1.5 rounded-lg bg-warning-100/50 text-warning-600">
                                          <LuSun size={14} />
                                        </div>
                                        <div className="flex flex-col">
                                          <p className="text-[10px] text-default-400 uppercase font-bold">
                                            {t(
                                              "settings.appearance.sunrise_time",
                                            )}
                                          </p>
                                          <p className="text-sm font-mono font-bold text-default-700">
                                            {sunTimes.sunrise}
                                          </p>
                                        </div>
                                      </div>
                                      <div className="flex items-center gap-2">
                                        <div className="p-1.5 rounded-lg bg-primary-100/50 dark:bg-primary-500/10 text-amber-400">
                                          <LuMoon size={14} />
                                        </div>
                                        <div className="flex flex-col">
                                          <p className="text-[10px] text-default-400 uppercase font-bold">
                                            {t(
                                              "settings.appearance.sunset_time",
                                            ) || "日落"}
                                          </p>
                                          <p className="text-sm font-mono font-bold text-default-700">
                                            {sunTimes.sunset}
                                          </p>
                                        </div>
                                      </div>
                                    </div>

                                    <Button
                                      size="sm"
                                      variant="flat"
                                      radius="full"
                                      startContent={<RxUpdate size={12} />}
                                      className="h-7 text-tiny self-start bg-default-200/50 hover:bg-default-300/50"
                                      onClick={refreshSunTimes}
                                    >
                                      {t("common.refresh")}
                                    </Button>
                                  </div>
                                ) : (
                                  <div className="flex flex-col gap-2 py-2">
                                    <p className="text-tiny text-danger-500">
                                      {t(
                                        "settings.appearance.sun_fetch_failed",
                                      )}
                                    </p>
                                    <Button
                                      size="sm"
                                      variant="flat"
                                      radius="full"
                                      startContent={<RxUpdate size={12} />}
                                      className="h-7 text-tiny self-start bg-default-200/50"
                                      onClick={refreshSunTimes}
                                    >
                                      {t("common.retry")}
                                    </Button>
                                  </div>
                                )}
                              </div>
                            </motion.div>
                          )}
                        </AnimatePresence>
                      </div>

                      {/* Navigation Layout */}
                      <div className="py-4 flex items-center justify-between border-b border-default-200/50">
                        <div className="flex flex-col gap-1">
                          <p className="font-medium text-default-700 dark:text-zinc-200">
                            {t("settings.layout.title_navbar")}
                          </p>
                          <p className="text-tiny text-default-500 dark:text-zinc-400">
                            {t("settings.layout.desc_navbar")}
                          </p>
                        </div>
                        <Switch
                          size="sm"
                          isSelected={layoutMode === "navbar"}
                          onValueChange={(isSelected: boolean) => {
                            const mode = isSelected ? "navbar" : "sidebar";
                            setLayoutMode(mode);
                            localStorage.setItem("app.layoutMode", mode);
                            window.dispatchEvent(
                              new CustomEvent("app-layout-changed"),
                            );
                          }}
                          classNames={{
                            wrapper:
                              "group-data-[selected=true]:bg-primary-500",
                          }}
                        />
                      </div>

                      {/* Animation */}
                      <div className="py-4 flex items-center justify-between">
                        <div className="flex flex-col gap-1">
                          <p className="font-medium text-default-700 dark:text-zinc-200">
                            {t("settings.appearance.disable_animations")}
                          </p>
                          <p className="text-tiny text-default-500 dark:text-zinc-400">
                            {t("settings.appearance.disable_animations_desc")}
                          </p>
                        </div>
                        <Switch
                          size="sm"
                          isSelected={disableAnimations}
                          onValueChange={(isSelected: boolean) => {
                            setDisableAnimations(isSelected);
                            localStorage.setItem(
                              "app.disableAnimations",
                              String(isSelected),
                            );
                            window.dispatchEvent(
                              new CustomEvent("app-animations-changed"),
                            );
                          }}
                          classNames={{
                            wrapper:
                              "group-data-[selected=true]:bg-primary-500",
                          }}
                        />
                      </div>
                    </div>
                  </div>

                  {/* Mode-Specific Settings Card */}
                  <div className="flex flex-col gap-4">
                    <div className="flex items-center gap-2 mb-1">
                      <div className="w-1 h-4 bg-primary-500 rounded-full" />
                      <p className="text-sm font-bold text-default-600 uppercase tracking-wider">
                        {t("settings.appearance.mode_config")}
                      </p>
                    </div>

                    <Card className="border-none shadow-none bg-transparent overflow-visible">
                      <CardBody className="p-0 flex flex-col">
                        {/* Mode Switcher Header */}
                        <div className="py-4 border-b border-default-200/50 flex items-center justify-between rounded-t-2xl">
                          <div className="flex flex-col gap-0.5">
                            <p className="font-bold text-default-700 dark:text-zinc-100 flex items-center gap-2">
                              {themeSettingMode === "light" ? (
                                <LuSun className="text-warning-500" size={18} />
                              ) : (
                                <LuMoon className="text-amber-400" size={18} />
                              )}
                              {themeSettingMode === "light"
                                ? t("settings.appearance.theme_light")
                                : t("settings.appearance.theme_dark")}
                            </p>
                            <p className="text-tiny text-default-500">
                              {t("settings.appearance.edit_mode_desc")}
                            </p>
                          </div>
                          <Tabs
                            size="sm"
                            selectedKey={themeSettingMode}
                            onSelectionChange={(key) =>
                              setThemeSettingMode(key as "light" | "dark")
                            }
                            classNames={COMPONENT_STYLES.tabs}
                          >
                            <Tab
                              key="light"
                              title={t("settings.appearance.theme_light")}
                            />
                            <Tab
                              key="dark"
                              title={t("settings.appearance.theme_dark")}
                            />
                          </Tabs>
                        </div>

                        {/* Content Area */}
                        <div className="py-6 flex flex-col gap-8">
                          {/* Theme Color Group */}
                          <div className="flex flex-col gap-4 p-5 rounded-3xl bg-default-200/10 border border-default-200/50">
                            {/* Theme Color Section */}
                            <div className="flex flex-col gap-4">
                              <div className="flex items-center gap-2">
                                <LuPalette
                                  className="text-primary-500"
                                  size={18}
                                />
                                <div className="flex flex-col gap-0.5">
                                  <p className="text-small font-bold text-default-700">
                                    {t("settings.appearance.theme_color")}
                                  </p>
                                  <p className="text-tiny text-default-400">
                                    {t("settings.appearance.theme_color_desc")}
                                  </p>
                                </div>
                              </div>
                              <div className="flex flex-col gap-6">
                                {/* Preset Colors Group (Manual 50-950) */}
                                <div className="flex flex-col gap-3">
                                  <div className="flex items-center justify-between px-1">
                                    <span className="text-tiny font-bold text-default-400 uppercase tracking-wider">
                                      {t("settings.appearance.theme_standard")}
                                    </span>
                                  </div>
                                  <div className="flex gap-3 flex-wrap p-4 rounded-2xl bg-default-200/20 border border-default-200/50">
                                    {THEME_GROUPS.preset.map((colorName) => {
                                      const isSelected =
                                        themeSettingMode === "light"
                                          ? lightThemeColor === colorName
                                          : darkThemeColor === colorName;
                                      return (
                                        <div
                                          key={colorName}
                                          className={`w-8 h-8 rounded-full cursor-pointer flex items-center justify-center transition-all hover:scale-110 active:scale-95 ${
                                            isSelected
                                              ? "ring-2 ring-offset-2 ring-primary-500 shadow-lg"
                                              : ""
                                          }`}
                                          style={{
                                            backgroundColor:
                                              THEMES[colorName][500],
                                          }}
                                          onClick={() => {
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
                                        </div>
                                      );
                                    })}
                                  </div>
                                </div>

                                {/* Generated Colors Group (Automatic) */}
                                <div className="flex flex-col gap-3">
                                  <div className="flex items-center justify-between px-1">
                                    <span className="text-tiny font-bold text-default-400 uppercase tracking-wider">
                                      {t("settings.appearance.theme_generated")}
                                    </span>
                                  </div>
                                  <div className="flex gap-3 flex-wrap p-4 rounded-2xl bg-default-200/20 border border-default-200/50">
                                    {THEME_GROUPS.generated.map((colorName) => {
                                      const isSelected =
                                        themeSettingMode === "light"
                                          ? lightThemeColor === colorName
                                          : darkThemeColor === colorName;
                                      return (
                                        <div
                                          key={colorName}
                                          className={`w-8 h-8 rounded-full cursor-pointer flex items-center justify-center transition-all hover:scale-110 active:scale-95 ${
                                            isSelected
                                              ? "ring-2 ring-offset-2 ring-primary-500 shadow-lg"
                                              : ""
                                          }`}
                                          style={{
                                            backgroundColor:
                                              THEMES[colorName][500],
                                          }}
                                          onClick={() => {
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
                                        </div>
                                      );
                                    })}
                                    <div
                                      className={`w-8 h-8 rounded-full cursor-pointer flex items-center justify-center transition-all hover:scale-110 active:scale-95 relative overflow-hidden group ${
                                        (
                                          themeSettingMode === "light"
                                            ? lightThemeColor === "custom"
                                            : darkThemeColor === "custom"
                                        )
                                          ? "ring-2 ring-offset-2 ring-primary-500 shadow-lg"
                                          : "hover:shadow-md"
                                      }`}
                                      onClick={() => {
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
                                    >
                                      {/* Modern Mesh Gradient Background */}
                                      <div className="absolute inset-0 bg-gradient-to-tr from-blue-500 via-purple-500 to-pink-500 group-hover:scale-125 transition-transform duration-500" />
                                      <div className="absolute inset-0 bg-[radial-gradient(circle_at_bottom_left,_var(--tw-gradient-from)_0%,_transparent_50%)] from-yellow-400/40" />

                                      <LuPalette className="text-white relative z-10 w-4 h-4 drop-shadow-sm" />

                                      {(themeSettingMode === "light"
                                        ? lightThemeColor === "custom"
                                        : darkThemeColor === "custom") && (
                                        <div className="absolute inset-0 bg-black/10 z-0 flex items-center justify-center">
                                          <div className="w-2.5 h-2.5 bg-white rounded-full shadow-sm z-20" />
                                        </div>
                                      )}
                                    </div>
                                  </div>
                                </div>
                              </div>

                              {(themeSettingMode === "light"
                                ? lightThemeColor === "custom"
                                : darkThemeColor === "custom") && (
                                <div className="flex flex-col gap-4 bg-default-200/20 p-6 rounded-2xl border border-default-200/50">
                                  <div className="flex items-center justify-between">
                                    <p className="text-small font-bold text-default-700">
                                      {t("settings.appearance.custom_color") ||
                                        "Custom Color"}
                                    </p>
                                    <div className="px-3 py-1 bg-default-200/50 rounded-lg">
                                      <span className="text-tiny font-mono uppercase text-primary-500 font-bold">
                                        {themeSettingMode === "light"
                                          ? lightCustomThemeColor
                                          : darkCustomThemeColor}
                                      </span>
                                    </div>
                                  </div>

                                  <CustomColorPicker
                                    color={
                                      themeSettingMode === "light"
                                        ? lightCustomThemeColor
                                        : darkCustomThemeColor
                                    }
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

                            <Divider className="bg-default-200/50" />

                            {/* Background Use Theme */}
                            <div className="flex items-center justify-between">
                              <div className="flex flex-col gap-0.5">
                                <p className="text-small font-bold text-default-700">
                                  {t(
                                    "settings.appearance.background_use_theme",
                                  )}
                                </p>
                                <p className="text-tiny text-default-400">
                                  {t(
                                    "settings.appearance.background_use_theme_desc",
                                  )}
                                </p>
                              </div>
                              <Switch
                                size="sm"
                                isSelected={
                                  themeSettingMode === "light"
                                    ? lightBackgroundUseTheme
                                    : darkBackgroundUseTheme
                                }
                                onValueChange={(isSelected: boolean) => {
                                  if (themeSettingMode === "light") {
                                    setLightBackgroundUseTheme(isSelected);
                                    localStorage.setItem(
                                      "app.lightBackgroundUseTheme",
                                      String(isSelected),
                                    );
                                  } else {
                                    setDarkBackgroundUseTheme(isSelected);
                                    localStorage.setItem(
                                      "app.darkBackgroundUseTheme",
                                      String(isSelected),
                                    );
                                  }
                                  window.dispatchEvent(
                                    new CustomEvent("app-bg-theme-changed"),
                                  );
                                }}
                                classNames={{
                                  wrapper:
                                    "group-data-[selected=true]:bg-primary-500",
                                }}
                              />
                            </div>

                            <Divider className="bg-default-200/50" />

                            {/* Base Opacity */}
                            <div className="flex flex-col gap-3">
                              <div className="flex items-center justify-between">
                                <p className="text-tiny font-medium text-default-600">
                                  {t(
                                    "settings.appearance.background_base_opacity",
                                  ) || "Base Opacity"}
                                </p>
                                <div className="flex items-center">
                                  <input
                                    type="number"
                                    className="w-10 bg-transparent text-tiny font-mono text-primary-500 text-right outline-none border-none p-0 [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
                                    onFocus={(e) => e.target.select()}
                                    value={
                                      themeSettingMode === "light"
                                        ? lightBackgroundBaseOpacity
                                        : darkBackgroundBaseOpacity
                                    }
                                    min={0}
                                    max={100}
                                    onChange={(e) => {
                                      const val = parseInt(e.target.value) || 0;
                                      const clampedVal = Math.min(
                                        100,
                                        Math.max(0, val),
                                      );
                                      if (themeSettingMode === "light") {
                                        setLightBackgroundBaseOpacity(
                                          clampedVal,
                                        );
                                        localStorage.setItem(
                                          "app.lightBackgroundBaseOpacity",
                                          String(clampedVal),
                                        );
                                      } else {
                                        setDarkBackgroundBaseOpacity(
                                          clampedVal,
                                        );
                                        localStorage.setItem(
                                          "app.darkBackgroundBaseOpacity",
                                          String(clampedVal),
                                        );
                                      }
                                      window.dispatchEvent(
                                        new CustomEvent(
                                          "app-base-opacity-changed",
                                        ),
                                      );
                                    }}
                                  />
                                  <span className="text-tiny font-mono text-primary-500 ml-0.5">
                                    %
                                  </span>
                                </div>
                              </div>
                              <Slider
                                size="sm"
                                step={1}
                                maxValue={100}
                                minValue={0}
                                aria-label="Base Opacity"
                                value={
                                  themeSettingMode === "light"
                                    ? lightBackgroundBaseOpacity
                                    : darkBackgroundBaseOpacity
                                }
                                classNames={{
                                  filler: "bg-primary-500",
                                  thumb: "bg-primary-500",
                                }}
                                onChange={(v) => {
                                  const val = Number(v);
                                  if (themeSettingMode === "light") {
                                    setLightBackgroundBaseOpacity(val);
                                    localStorage.setItem(
                                      "app.lightBackgroundBaseOpacity",
                                      String(val),
                                    );
                                  } else {
                                    setDarkBackgroundBaseOpacity(val);
                                    localStorage.setItem(
                                      "app.darkBackgroundBaseOpacity",
                                      String(val),
                                    );
                                  }
                                  window.dispatchEvent(
                                    new CustomEvent("app-base-opacity-changed"),
                                  );
                                }}
                              />
                            </div>
                          </div>

                          {/* Background Section */}
                          <div className="flex flex-col gap-6">
                            <div className="flex items-center gap-2">
                              <LuSun className="text-primary-500" size={18} />
                              <div className="flex flex-col gap-0.5">
                                <p className="text-small font-bold text-default-700">
                                  {t("settings.appearance.background_image")}
                                </p>
                                <p className="text-tiny text-default-400">
                                  {t(
                                    "settings.appearance.background_image_desc",
                                  )}
                                </p>
                              </div>
                            </div>

                            <div className="flex flex-col gap-6 p-5 rounded-2xl bg-default-200/20 border border-default-200/50">
                              {/* Image Picker Row */}
                              <div className="flex items-center justify-between">
                                <div className="flex items-center gap-3 overflow-hidden">
                                  <div className="w-12 h-12 rounded-lg bg-default-300/30 flex items-center justify-center flex-shrink-0 border border-default-300/50 overflow-hidden">
                                    {previewBgData && !backgroundImageError ? (
                                      <img
                                        src={previewBgData}
                                        className="w-full h-full object-cover"
                                        onError={() =>
                                          setBackgroundImageError(true)
                                        }
                                      />
                                    ) : (
                                      <LuPalette
                                        className="text-default-400"
                                        size={20}
                                      />
                                    )}
                                  </div>
                                  <div className="flex flex-col min-w-0">
                                    <p className="text-tiny font-medium truncate text-default-600">
                                      {(themeSettingMode === "light"
                                        ? lightBackgroundImage
                                        : darkBackgroundImage) ||
                                        t(
                                          "settings.appearance.no_image_selected",
                                        )}
                                    </p>
                                  </div>
                                </div>
                                <div className="flex items-center gap-2">
                                  {(themeSettingMode === "light"
                                    ? lightBackgroundImage
                                    : darkBackgroundImage) && (
                                    <Button
                                      size="sm"
                                      color="danger"
                                      variant="flat"
                                      className="h-8"
                                      onPress={() => {
                                        if (themeSettingMode === "light") {
                                          setLightBackgroundImage("");
                                          localStorage.setItem(
                                            "app.lightBackgroundImage",
                                            "",
                                          );
                                        } else {
                                          setDarkBackgroundImage("");
                                          localStorage.setItem(
                                            "app.darkBackgroundImage",
                                            "",
                                          );
                                        }
                                        localStorage.removeItem(
                                          "app.backgroundImage",
                                        );
                                        window.dispatchEvent(
                                          new CustomEvent(
                                            "app-background-changed",
                                          ),
                                        );
                                      }}
                                    >
                                      {t("settings.appearance.clear_image")}
                                    </Button>
                                  )}
                                  <Button
                                    size="sm"
                                    variant="solid"
                                    color="primary"
                                    className="h-8 shadow-sm"
                                    onPress={async () => {
                                      try {
                                        const result = await Dialogs.OpenFile({
                                          Title: t(
                                            "settings.appearance.select_image",
                                          ),
                                          Filters: [
                                            {
                                              DisplayName: "Images",
                                              Pattern:
                                                "*.png;*.jpg;*.jpeg;*.webp;*.gif;*.bmp",
                                            },
                                          ],
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
                                          if (themeSettingMode === "light") {
                                            setLightBackgroundImage(path);
                                            localStorage.setItem(
                                              "app.lightBackgroundImage",
                                              path,
                                            );
                                          } else {
                                            setDarkBackgroundImage(path);
                                            localStorage.setItem(
                                              "app.darkBackgroundImage",
                                              path,
                                            );
                                          }
                                          window.dispatchEvent(
                                            new CustomEvent(
                                              "app-background-changed",
                                            ),
                                          );
                                        }
                                      } catch {}
                                    }}
                                  >
                                    {t("settings.appearance.select_image")}
                                  </Button>
                                </div>
                              </div>

                              <div className="grid grid-cols-1 md:grid-cols-2 gap-x-8 gap-y-6">
                                {/* Background Blur */}
                                <div className="flex flex-col gap-2">
                                  <div className="flex items-center justify-between">
                                    <p className="text-tiny font-medium text-default-600">
                                      {t("settings.appearance.background_blur")}
                                    </p>
                                    <div className="flex items-center">
                                      <input
                                        type="number"
                                        className="w-10 bg-transparent text-tiny font-mono text-primary-500 text-right outline-none border-none p-0 [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
                                        onFocus={(e) => e.target.select()}
                                        value={
                                          themeSettingMode === "light"
                                            ? lightBackgroundBlur
                                            : darkBackgroundBlur
                                        }
                                        min={0}
                                        max={50}
                                        onChange={(e) => {
                                          const val =
                                            parseInt(e.target.value) || 0;
                                          const clampedVal = Math.min(
                                            50,
                                            Math.max(0, val),
                                          );
                                          if (themeSettingMode === "light") {
                                            setLightBackgroundBlur(clampedVal);
                                            localStorage.setItem(
                                              "app.lightBackgroundBlur",
                                              String(clampedVal),
                                            );
                                          } else {
                                            setDarkBackgroundBlur(clampedVal);
                                            localStorage.setItem(
                                              "app.darkBackgroundBlur",
                                              String(clampedVal),
                                            );
                                          }
                                          window.dispatchEvent(
                                            new CustomEvent("app-blur-changed"),
                                          );
                                        }}
                                      />
                                      <span className="text-tiny font-mono text-primary-500 ml-0.5">
                                        px
                                      </span>
                                    </div>
                                  </div>
                                  <Slider
                                    size="sm"
                                    step={1}
                                    maxValue={50}
                                    minValue={0}
                                    aria-label="Blur"
                                    value={
                                      themeSettingMode === "light"
                                        ? lightBackgroundBlur
                                        : darkBackgroundBlur
                                    }
                                    classNames={{
                                      filler: "bg-primary-500",
                                      thumb: "bg-primary-500",
                                    }}
                                    onChange={(v) => {
                                      const val = Number(v);
                                      if (themeSettingMode === "light") {
                                        setLightBackgroundBlur(val);
                                        localStorage.setItem(
                                          "app.lightBackgroundBlur",
                                          String(val),
                                        );
                                      } else {
                                        setDarkBackgroundBlur(val);
                                        localStorage.setItem(
                                          "app.darkBackgroundBlur",
                                          String(val),
                                        );
                                      }
                                      window.dispatchEvent(
                                        new CustomEvent("app-blur-changed"),
                                      );
                                    }}
                                  />
                                </div>

                                {/* Background Brightness */}
                                <div className="flex flex-col gap-2">
                                  <div className="flex items-center justify-between">
                                    <p className="text-tiny font-medium text-default-600">
                                      {t(
                                        "settings.appearance.background_brightness",
                                      )}
                                    </p>
                                    <div className="flex items-center">
                                      <input
                                        type="number"
                                        className="w-10 bg-transparent text-tiny font-mono text-primary-500 text-right outline-none border-none p-0 [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
                                        onFocus={(e) => e.target.select()}
                                        value={
                                          themeSettingMode === "light"
                                            ? lightBackgroundBrightness
                                            : darkBackgroundBrightness
                                        }
                                        min={20}
                                        max={100}
                                        onChange={(e) => {
                                          const val =
                                            parseInt(e.target.value) || 0;
                                          const clampedVal = Math.min(
                                            100,
                                            Math.max(20, val),
                                          );
                                          if (themeSettingMode === "light") {
                                            setLightBackgroundBrightness(
                                              clampedVal,
                                            );
                                            localStorage.setItem(
                                              "app.lightBackgroundBrightness",
                                              String(clampedVal),
                                            );
                                          } else {
                                            setDarkBackgroundBrightness(
                                              clampedVal,
                                            );
                                            localStorage.setItem(
                                              "app.darkBackgroundBrightness",
                                              String(clampedVal),
                                            );
                                          }
                                          window.dispatchEvent(
                                            new CustomEvent(
                                              "app-brightness-changed",
                                            ),
                                          );
                                        }}
                                      />
                                      <span className="text-tiny font-mono text-primary-500 ml-0.5">
                                        %
                                      </span>
                                    </div>
                                  </div>
                                  <Slider
                                    size="sm"
                                    step={1}
                                    maxValue={100}
                                    minValue={20}
                                    aria-label="Brightness"
                                    value={
                                      themeSettingMode === "light"
                                        ? lightBackgroundBrightness
                                        : darkBackgroundBrightness
                                    }
                                    classNames={{
                                      filler: "bg-primary-500",
                                      thumb: "bg-primary-500",
                                    }}
                                    onChange={(v) => {
                                      const val = Number(v);
                                      if (themeSettingMode === "light") {
                                        setLightBackgroundBrightness(val);
                                        localStorage.setItem(
                                          "app.lightBackgroundBrightness",
                                          String(val),
                                        );
                                      } else {
                                        setDarkBackgroundBrightness(val);
                                        localStorage.setItem(
                                          "app.darkBackgroundBrightness",
                                          String(val),
                                        );
                                      }
                                      window.dispatchEvent(
                                        new CustomEvent(
                                          "app-brightness-changed",
                                        ),
                                      );
                                    }}
                                  />
                                </div>

                                {/* Background Opacity */}
                                <div className="flex flex-col gap-2">
                                  <div className="flex items-center justify-between">
                                    <p className="text-tiny font-medium text-default-600">
                                      {t(
                                        "settings.appearance.background_opacity",
                                      )}
                                    </p>
                                    <div className="flex items-center">
                                      <input
                                        type="number"
                                        className="w-10 bg-transparent text-tiny font-mono text-primary-500 text-right outline-none border-none p-0 [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
                                        onFocus={(e) => e.target.select()}
                                        value={
                                          themeSettingMode === "light"
                                            ? lightBackgroundOpacity
                                            : darkBackgroundOpacity
                                        }
                                        min={0}
                                        max={100}
                                        onChange={(e) => {
                                          const val =
                                            parseInt(e.target.value) || 0;
                                          const clampedVal = Math.min(
                                            100,
                                            Math.max(0, val),
                                          );
                                          if (themeSettingMode === "light") {
                                            setLightBackgroundOpacity(
                                              clampedVal,
                                            );
                                            localStorage.setItem(
                                              "app.lightBackgroundOpacity",
                                              String(clampedVal),
                                            );
                                          } else {
                                            setDarkBackgroundOpacity(
                                              clampedVal,
                                            );
                                            localStorage.setItem(
                                              "app.darkBackgroundOpacity",
                                              String(clampedVal),
                                            );
                                          }
                                          window.dispatchEvent(
                                            new CustomEvent(
                                              "app-opacity-changed",
                                            ),
                                          );
                                        }}
                                      />
                                      <span className="text-tiny font-mono text-primary-500 ml-0.5">
                                        %
                                      </span>
                                    </div>
                                  </div>
                                  <Slider
                                    size="sm"
                                    step={1}
                                    maxValue={100}
                                    minValue={0}
                                    aria-label="Opacity"
                                    value={
                                      themeSettingMode === "light"
                                        ? lightBackgroundOpacity
                                        : darkBackgroundOpacity
                                    }
                                    classNames={{
                                      filler: "bg-primary-500",
                                      thumb: "bg-primary-500",
                                    }}
                                    onChange={(v) => {
                                      const val = Number(v);
                                      if (themeSettingMode === "light") {
                                        setLightBackgroundOpacity(val);
                                        localStorage.setItem(
                                          "app.lightBackgroundOpacity",
                                          String(val),
                                        );
                                      } else {
                                        setDarkBackgroundOpacity(val);
                                        localStorage.setItem(
                                          "app.darkBackgroundOpacity",
                                          String(val),
                                        );
                                      }
                                      window.dispatchEvent(
                                        new CustomEvent("app-opacity-changed"),
                                      );
                                    }}
                                  />
                                </div>
                              </div>
                            </div>
                          </div>
                        </div>
                      </CardBody>
                    </Card>
                  </div>
                </div>
              )}

              {selectedTab === "components" && (
                <div className="flex flex-col gap-6">
                  {/* GDK */}
                  <div className="flex items-center justify-between">
                    <div className="flex flex-col gap-1">
                      <p className="font-medium">{t("settings.gdk.title")}</p>
                      <p className="text-tiny text-default-500 dark:text-zinc-400">
                        {t("settings.gdk.path_label", {
                          path: "C:\\Program Files (x86)\\Microsoft GDK",
                        })}
                      </p>
                    </div>
                    {gdkInstalled ? (
                      <Chip color="success" variant="flat">
                        {t("settings.gdk.installed")}
                      </Chip>
                    ) : (
                      <Button
                        radius="full"
                        variant="bordered"
                        size="sm"
                        onPress={() => {
                          setGdkLicenseAccepted(false);
                          gdkLicenseDisclosure.onOpen();
                        }}
                      >
                        {t("settings.gdk.install_button")}
                      </Button>
                    )}
                  </div>

                  <Divider className="bg-default-200/50" />

                  {/* LIP */}
                  <div className="flex items-center justify-between">
                    <div className="flex flex-col gap-1">
                      <p className="font-medium">{t("settings.lip.title")}</p>
                      <p className="text-tiny text-default-500 dark:text-zinc-400">
                        {lipInstalled
                          ? t("settings.lip.installed", {
                              version: lipVersion,
                            })
                          : t("settings.lip.description")}
                      </p>
                    </div>
                    {lipInstalled ? (
                      <div className="flex items-center gap-2">
                        <Chip color="success" variant="flat">
                          {t("settings.lip.installed_label")}
                        </Chip>
                        {hasLipUpdate && (
                          <Button
                            variant="bordered"
                            radius="full"
                            isLoading={installingLip}
                            isDisabled={installingLip}
                            onPress={() => {
                              setInstallingLip(true);
                              setLipError("");
                              lipProgressDisclosure.onOpen();
                              InstallLip().then((err) => {
                                if (err) {
                                  setInstallingLip(false);
                                  setLipError(err);
                                }
                              });
                            }}
                          >
                            {t("settings.lip.update_button", {
                              version: lipLatestVersion,
                            })}
                          </Button>
                        )}
                      </div>
                    ) : (
                      <Button
                        radius="full"
                        variant="bordered"
                        size="sm"
                        isLoading={installingLip}
                        isDisabled={installingLip}
                        onPress={() => {
                          setInstallingLip(true);
                          setLipError("");
                          lipProgressDisclosure.onOpen();
                          InstallLip().then((err) => {
                            if (err) {
                              setInstallingLip(false);
                              setLipError(err);
                            }
                          });
                        }}
                      >
                        {installingLip
                          ? t("settings.lip.installing")
                          : t("settings.lip.install_button")}
                      </Button>
                    )}
                  </div>
                </div>
              )}

              {selectedTab === "others" && (
                <div className="flex items-center justify-between">
                  <div className="flex flex-col gap-1">
                    <p className="font-medium">{t("settings.process.title")}</p>
                    <p className="text-tiny text-default-500 dark:text-zinc-400">
                      {t("settings.process.desc")}
                    </p>
                  </div>
                  <Button
                    radius="full"
                    variant="bordered"
                    onPress={() => setProcessModalOpen(true)}
                  >
                    {t("settings.process.scan")}
                  </Button>
                </div>
              )}

              {selectedTab === "updates" && (
                <div className="flex flex-col gap-6">
                  <div className="flex items-center justify-between">
                    <div className="flex flex-col gap-1">
                      <p className="font-medium">
                        {t("settings.beta_updates.title")}
                      </p>
                      <p className="text-tiny text-default-500 dark:text-zinc-400">
                        {t("settings.beta_updates.desc")}
                      </p>
                    </div>
                    <Switch
                      size="sm"
                      isSelected={enableBetaUpdates}
                      onValueChange={(isSelected: boolean) => {
                        setEnableBetaUpdates(isSelected);
                        SetEnableBetaUpdates(isSelected);
                      }}
                      classNames={{
                        wrapper: "group-data-[selected=true]:bg-primary-500",
                      }}
                    />
                  </div>

                  <Divider className="bg-default-200/50" />

                  <div className="flex items-center justify-between">
                    <div className="flex flex-col">
                      <p className="font-medium text-large">
                        {t("settings.body.version.name")}
                      </p>
                      <p className="text-tiny text-default-500 dark:text-zinc-400">
                        v{appVersion}
                      </p>
                    </div>
                    {checkingUpdate ? (
                      <Spinner size="sm" color="primary" />
                    ) : (
                      <Button
                        radius="full"
                        variant="bordered"
                        onPress={onCheckUpdate}
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
                        <div className="rounded-xl bg-default-100/50 dark:bg-zinc-800/30 p-4 border border-default-200/50 dark:border-white/5">
                          <div className="flex items-center justify-between mb-3">
                            <p className="text-small font-bold text-primary-600 dark:text-primary-500">
                              {t("settings.body.version.hasnew")} {newVersion}
                            </p>
                            <Button
                              color="primary"
                              radius="full"
                              onPress={onUpdate}
                              isDisabled={updating}
                              className="bg-primary-600 hover:bg-primary-500 text-white font-bold shadow-lg shadow-primary-900/20"
                              startContent={<RxUpdate />}
                            >
                              {updating
                                ? t("common.updating")
                                : t("settings.modal.2.footer.download_button")}
                            </Button>
                          </div>

                          {changelog && (
                            <div className="text-small wrap-break-word leading-6 max-h-[200px] overflow-y-auto pr-1 scrollbar-thin scrollbar-thumb-default-300">
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
                                    <p className="my-1 text-default-600">
                                      {children}
                                    </p>
                                  ),
                                  ul: ({ children }) => (
                                    <ul className="list-disc pl-5 my-1 text-default-600">
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
                                        className="text-primary-500 underline"
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
                              <Progress
                                size="sm"
                                radius="sm"
                                color="success"
                                isIndeterminate={true}
                                classNames={{
                                  indicator:
                                    "bg-primary-600 hover:bg-primary-500",
                                }}
                              />
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
                        <p className="font-medium text-large">
                          {t("aboutcard.title")}
                        </p>
                        <p className="text-tiny text-default-500 dark:text-zinc-400">
                          {t("aboutcard.description", { name: "LeviMC" })} ·{" "}
                          {t("aboutcard.font", { name: "MiSans" })}
                        </p>
                      </div>
                    </div>
                    <div className="flex items-center gap-1">
                      <Button
                        isIconOnly
                        variant="light"
                        radius="full"
                        onPress={() =>
                          Browser.OpenURL("https://github.com/liteldev")
                        }
                      >
                        <FaGithub
                          size={20}
                          className="text-default-500 dark:text-zinc-400"
                        />
                      </Button>
                      <Button
                        isIconOnly
                        variant="light"
                        radius="full"
                        onPress={() =>
                          Browser.OpenURL("https://discord.gg/v5R5P4vRZk")
                        }
                      >
                        <FaDiscord
                          size={20}
                          className="text-default-500 dark:text-zinc-400"
                        />
                      </Button>
                    </div>
                  </div>
                </div>
              )}
            </CardBody>
          </Card>
        </motion.div>
      </div>

      {/* GDK License */}
      <UnifiedModal
        size="md"
        isOpen={gdkLicenseDisclosure.isOpen}
        onOpenChange={gdkLicenseDisclosure.onOpenChange}
        type="info"
        title={t("settings.gdk.license.title")}
        icon={<FaDownload className="w-6 h-6 text-primary-500" />}
        footer={
          <>
            <Button variant="light" onPress={gdkLicenseDisclosure.onClose}>
              {t("common.cancel")}
            </Button>
            <Button
              color="primary"
              isDisabled={!gdkLicenseAccepted}
              onPress={() => {
                gdkLicenseDisclosure.onClose();
                try {
                  setGdkDlError("");
                  setGdkDlProgress(null);
                  gdkProgressDisclosure.onOpen();
                  StartGDKDownload(
                    "https://github.bibk.top/microsoft/GDK/releases/download/October-2025-Update-1-v2510.1.6224/GDK_2510.1.6224.zip",
                  );
                } catch {}
              }}
            >
              {t("downloadmodal.download_button")}
            </Button>
          </>
        }
      >
        <div className="text-default-700 dark:text-zinc-300 text-sm">
          {t("settings.gdk.license.body")}{" "}
          <a
            className="text-primary underline"
            href="https://aka.ms/GDK_EULA"
            target="_blank"
            rel="noreferrer"
          >
            Microsoft Public Game Development Kit License Agreement
          </a>
        </div>
        <div className="flex items-center gap-2 mt-3">
          <input
            type="checkbox"
            id="gdk-license"
            checked={gdkLicenseAccepted}
            onChange={(e) => setGdkLicenseAccepted(Boolean(e.target.checked))}
          />
          <label htmlFor="gdk-license" className="text-small">
            {t("settings.gdk.license.accept")}
          </label>
        </div>
      </UnifiedModal>

      {/* Process Management Modal */}
      <UnifiedModal
        size="2xl"
        isOpen={processModalOpen}
        onOpenChange={setProcessModalOpen}
        scrollBehavior="inside"
        type="primary"
        title={
          <div className="flex flex-col gap-1">
            <span>{t("settings.process.title")}</span>
            <span className="text-small font-normal text-default-500 dark:text-zinc-400">
              {t("settings.process.desc")}
            </span>
          </div>
        }
        icon={<FaList className="w-6 h-6 text-primary-500" />}
        footer={
          <Button variant="light" onPress={() => setProcessModalOpen(false)}>
            {t("common.close")}
          </Button>
        }
      >
        <div className="flex items-center justify-end mb-4 gap-2">
          <Button
            size="sm"
            variant="flat"
            color="primary"
            isLoading={scanningProcesses}
            onPress={refreshProcesses}
          >
            {t("settings.process.scan")}
          </Button>
          {processes.length > 0 && (
            <Button
              size="sm"
              color="danger"
              variant="flat"
              onPress={handleKillAllProcesses}
            >
              {t("settings.process.kill_all")}
            </Button>
          )}
        </div>
        <div className="flex flex-col gap-4">
          {processes.length === 0 ? (
            <div className="text-center py-8 text-default-500 dark:text-zinc-400">
              {t("settings.process.no_process")}
            </div>
          ) : (
            <div className="flex flex-col gap-2">
              {processes.map((p) => (
                <div
                  key={p.pid}
                  className="flex items-center justify-between p-3 rounded-xl bg-default-100/50 dark:bg-default-100/10 border border-default-200/50"
                >
                  <div className="flex flex-col gap-1 overflow-hidden">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-small bg-default-200/50 px-1.5 rounded text-default-600">
                        {p.pid}
                      </span>
                      {p.isLauncher && p.versionName ? (
                        <Chip
                          size="sm"
                          color="success"
                          variant="flat"
                          className="h-5 text-[10px]"
                        >
                          {p.versionName}
                        </Chip>
                      ) : (
                        <span className="text-small font-medium">
                          Minecraft.Windows.exe
                        </span>
                      )}
                    </div>
                    <span
                      className="text-tiny text-default-400 truncate max-w-[400px]"
                      title={p.exePath}
                    >
                      {p.exePath}
                    </span>
                  </div>
                  <Button
                    size="sm"
                    color="danger"
                    variant="light"
                    onPress={() => handleKillProcess(p.pid)}
                  >
                    {t("settings.process.kill")}
                  </Button>
                </div>
              ))}
            </div>
          )}
        </div>
      </UnifiedModal>

      {/* GDK Download Progress */}
      <UnifiedModal
        size="md"
        isOpen={gdkProgressDisclosure.isOpen}
        onOpenChange={gdkProgressDisclosure.onOpenChange}
        hideCloseButton
        isDismissable={false}
        type="info"
        title={t("settings.gdk.download.title")}
        icon={<FaDownload className="w-6 h-6 text-primary-500" />}
        footer={
          <>
            <Button
              color="danger"
              variant="light"
              isDisabled={gdkDlStatus === "done"}
              onPress={() => {
                try {
                  CancelGDKDownload();
                } catch {}
                gdkProgressDisclosure.onClose();
              }}
            >
              {t("common.cancel")}
            </Button>
            <Button
              color="primary"
              isDisabled={gdkDlStatus !== "done"}
              onPress={() => gdkProgressDisclosure.onClose()}
            >
              {t("common.ok")}
            </Button>
          </>
        }
      >
        {gdkDlError ? (
          <div className="text-danger">{gdkDlError}</div>
        ) : (
          <div className="flex flex-col gap-3">
            <div className="h-2 w-full rounded bg-default-200 overflow-hidden">
              {(() => {
                const total = gdkDlProgress?.total || 0;
                const done = gdkDlProgress?.downloaded || 0;
                const pct =
                  total > 0
                    ? Math.min(100, Math.round((done / total) * 100))
                    : 0;
                return (
                  <div
                    className="h-full bg-primary"
                    style={{ width: `${pct}%` }}
                  />
                );
              })()}
            </div>
            <div className="text-small text-default-500 dark:text-zinc-400">
              {(() => {
                const total = gdkDlProgress?.total || 0;
                const done = gdkDlProgress?.downloaded || 0;
                const fmt = (n: number) =>
                  `${(n / (1024 * 1024)).toFixed(2)} MB`;
                const fmtSpd = (bps: number) =>
                  `${(bps / (1024 * 1024)).toFixed(2)} MB/s`;
                if (total > 0) {
                  const pct = Math.min(100, Math.round((done / total) * 100));
                  return `${fmt(done)} / ${fmt(total)} (${pct}%) · ${fmtSpd(
                    gdkDlSpeed || 0,
                  )}`;
                }
                return `${fmt(done)} · ${fmtSpd(gdkDlSpeed || 0)}`;
              })()}
            </div>
          </div>
        )}
      </UnifiedModal>

      {/* GDK Install */}
      <UnifiedModal
        size="md"
        isOpen={gdkInstallDisclosure.isOpen}
        onOpenChange={gdkInstallDisclosure.onOpenChange}
        hideCloseButton
        isDismissable={false}
        type="info"
        title={t("settings.gdk.install.title")}
        icon={<FaCogs className="w-6 h-6 text-primary-500" />}
      >
        <div className="text-small text-default-500 dark:text-zinc-400">
          {t("settings.gdk.install.body")}
        </div>
      </UnifiedModal>

      {/* LIP Install Progress */}
      <UnifiedModal
        size="md"
        isOpen={lipProgressDisclosure.isOpen}
        onOpenChange={lipProgressDisclosure.onOpenChange}
        hideCloseButton
        isDismissable={false}
        type="info"
        title={t("settings.lip.installing")}
        icon={<FaDownload className="w-6 h-6 text-primary-500" />}
        footer={
          <>
            <Button
              color="danger"
              variant="light"
              onPress={lipProgressDisclosure.onClose}
              isDisabled={!installingLip}
            >
              {t("common.hide")}
            </Button>
            <Button
              color="primary"
              onPress={lipProgressDisclosure.onClose}
              isDisabled={installingLip && !lipError}
            >
              {lipError ? t("common.close") : t("common.ok")}
            </Button>
          </>
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
            <div className="h-2 w-full rounded bg-default-200 overflow-hidden">
              <div
                className="h-full bg-primary"
                style={{ width: `${lipProgress.percentage}%` }}
              />
            </div>
            <div className="text-small text-default-500 dark:text-zinc-400">
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
        size="md"
        isOpen={unsavedOpen}
        onOpenChange={unsavedOnOpenChange}
        hideCloseButton
        type="warning"
        title={t("settings.unsaved.title")}
        footer={
          <>
            <Button variant="light" onPress={unsavedOnClose}>
              {t("settings.unsaved.cancel")}
            </Button>
            <Button
              color="primary"
              isLoading={savingBaseRoot}
              isDisabled={!newBaseRoot || !baseRootWritable}
              onPress={async () => {
                setSavingBaseRoot(true);
                try {
                  const ok = await CanWriteToDir(newBaseRoot);
                  if (!ok) {
                    setBaseRootWritable(false);
                  } else {
                    const err = await SetBaseRoot(newBaseRoot);
                    if (!err) {
                      const br = await GetBaseRoot();
                      setBaseRoot(String(br || ""));
                      const id = await GetInstallerDir();
                      setInstallerDir(String(id || ""));
                      const vd = await GetVersionsDir();
                      setVersionsDir(String(vd || ""));
                      unsavedOnClose();
                      if (pendingNavPath === "-1") {
                        navigate(-1);
                      } else if (pendingNavPath) {
                        navigate(pendingNavPath);
                      }
                    }
                  }
                } catch {}
                setSavingBaseRoot(false);
              }}
            >
              {t("settings.unsaved.save")}
            </Button>
          </>
        }
      >
        <div className="text-default-700 dark:text-zinc-300 text-sm">
          {t("settings.unsaved.body")}
        </div>
        {!baseRootWritable && (
          <div className="text-tiny text-danger-500 mt-1">
            {t("settings.body.paths.not_writable")}
          </div>
        )}
      </UnifiedModal>

      <UnifiedModal
        size="sm"
        isOpen={resetOpen}
        onOpenChange={resetOnOpenChange}
        hideCloseButton
        type="error"
        title={t("settings.reset.confirm.title")}
        footer={
          <>
            <Button variant="light" onPress={resetOnClose}>
              {t("common.cancel")}
            </Button>
            <Button
              color="danger"
              onPress={async () => {
                try {
                  const err = await ResetBaseRoot();
                  if (!err) {
                    const br = await GetBaseRoot();
                    setBaseRoot(String(br || ""));
                    setNewBaseRoot(String(br || ""));
                    const id = await GetInstallerDir();
                    setInstallerDir(String(id || ""));
                    const vd = await GetVersionsDir();
                    setVersionsDir(String(vd || ""));
                  }
                } catch {}
                resetOnClose();
              }}
            >
              {t("common.confirm")}
            </Button>
          </>
        }
      >
        <div className="text-default-700 dark:text-zinc-300 text-sm">
          {t("settings.reset.confirm.body")}
        </div>
      </UnifiedModal>
    </PageContainer>
  );
};

export default SettingsPage;
