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
} from "@heroui/react";
import { RxUpdate } from "react-icons/rx";
import {
  FaGithub,
  FaDiscord,
  FaDownload,
  FaCogs,
  FaList,
} from "react-icons/fa";
import { FiAlertTriangle, FiCheckCircle, FiUploadCloud } from "react-icons/fi";
import { LuHardDrive } from "react-icons/lu";
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

export const SettingsPage: React.FC = () => {
  const { t, i18n } = useTranslation();
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
                  base: "mt-4",
                }}
              >
                <Tab key="general" title={t("settings.tabs.general")} />
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
                          className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
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
                        classNames={{
                          inputWrapper:
                            "bg-default-100/50 dark:bg-default-100/20 border-default-200 dark:border-default-700 hover:border-emerald-500 focus-within:border-emerald-500!",
                        }}
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
                    <Dropdown>
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

                  {/* Navigation Layout */}
                  <div className="flex flex-col gap-6">
                    <div className="flex items-center justify-between">
                      <div className="flex flex-col gap-1">
                        <p className="font-medium">
                          {t("settings.layout.title_navbar")}
                        </p>
                        <p className="text-tiny text-default-500 dark:text-zinc-400">
                          {t("settings.layout.desc_navbar")}
                        </p>
                      </div>
                      <Switch
                        size="sm"
                        isSelected={layoutMode === "navbar"}
                        onValueChange={(isSelected) => {
                          const mode = isSelected ? "navbar" : "sidebar";
                          setLayoutMode(mode);
                          localStorage.setItem("app.layoutMode", mode);
                          window.dispatchEvent(
                            new CustomEvent("app-layout-changed"),
                          );
                        }}
                        classNames={{
                          wrapper: "group-data-[selected=true]:bg-emerald-500",
                        }}
                      />
                    </div>

                    <Divider className="bg-default-200/50" />

                    {/* Animation */}
                    <div className="flex items-center justify-between">
                      <div className="flex flex-col">
                        <p className="font-medium">
                          {t("settings.appearance.disable_animations")}
                        </p>
                        <p className="text-tiny text-default-500 dark:text-zinc-400">
                          {t("settings.appearance.disable_animations_desc")}
                        </p>
                      </div>
                      <Switch
                        size="sm"
                        isSelected={disableAnimations}
                        onValueChange={(isSelected) => {
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
                          wrapper: "group-data-[selected=true]:bg-emerald-500",
                        }}
                      />
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
                        onValueChange={(isSelected) => {
                          setDiscordRpcEnabled(isSelected);
                          SetDisableDiscordRPC(!isSelected);
                        }}
                        classNames={{
                          wrapper: "group-data-[selected=true]:bg-emerald-500",
                        }}
                      />
                    </div>
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
                      onValueChange={(isSelected) => {
                        setEnableBetaUpdates(isSelected);
                        SetEnableBetaUpdates(isSelected);
                      }}
                      classNames={{
                        wrapper: "group-data-[selected=true]:bg-emerald-500",
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
                      <Spinner size="sm" color="success" />
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
                            <p className="text-small font-bold text-emerald-600 dark:text-emerald-500">
                              {t("settings.body.version.hasnew")} {newVersion}
                            </p>
                            <Button
                              color="primary"
                              radius="full"
                              onPress={onUpdate}
                              isDisabled={updating}
                              className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
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
                                        className="text-emerald-500 underline"
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
                                    "bg-emerald-600 hover:bg-emerald-500",
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
