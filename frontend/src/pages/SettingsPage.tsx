import React, { useEffect, useState } from "react";
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
} from "@heroui/react";
import { RxUpdate } from "react-icons/rx";
import { FaGithub, FaDiscord } from "react-icons/fa";
import { LuFolderOpen, LuHardDrive, LuMonitor } from "react-icons/lu";
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
  ResetBaseRoot,
} from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { Browser, Events } from "@wailsio/runtime";
import * as types from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import {
  BaseModal,
  BaseModalHeader,
  BaseModalBody,
  BaseModalFooter,
} from "@/components/BaseModal";
import { PageHeader } from "@/components/PageHeader";
import Logo from "@/assets/images/ic_leaf_logo.png";

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
  const navigate = useNavigate();
  const location = useLocation();
  const {
    isOpen: unsavedOpen,
    onOpen: unsavedOnOpen,
    onOpenChange: unsavedOnOpenChange,
  } = useDisclosure();
  const [pendingNavPath, setPendingNavPath] = useState<string>("");
  const {
    isOpen: resetOpen,
    onOpen: resetOnOpen,
    onOpenChange: resetOnOpenChange,
  } = useDisclosure();

  useEffect(() => {
    const handler = (ev: any) => {
      try {
        const targetPath = String(ev?.detail?.path || "");
        const hasUnsaved = !!newBaseRoot && newBaseRoot !== baseRoot;
        if (!targetPath || targetPath === location.pathname) return;
        if (hasUnsaved) {
          setPendingNavPath(targetPath);
          unsavedOnOpen();
          return;
        }
        navigate(targetPath);
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

    const normalize = (lng: string) => {
      if (!lng) return "en_US";
      const lower = lng.toLowerCase();
      if (lower === "en-us" || lower === "en") return "en_US";
      if (lower === "zh-cn" || lower === "zh") return "zh_CN";
      return lng;
    };
    setSelectedLang(normalize(i18n.language));
    setLanguageChanged(false);
    Promise.resolve()
      .then(async () => {
        try {
          if (hasBackend) {
            const br = await GetBaseRoot();
            const pick = (location?.state as any)?.baseRootPickResult;
            setBaseRoot(String(br || ""));
            setNewBaseRoot(
              typeof pick === "string" && pick.length > 0
                ? String(pick)
                : String(br || ""),
            );
            const id = await GetInstallerDir();
            setInstallerDir(String(id || ""));
            const vd = await GetVersionsDir();
            setVersionsDir(String(vd || ""));
            try {
              const disabled = await GetDisableDiscordRPC();
              setDiscordRpcEnabled(!disabled);
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
    const v: string | undefined = (location?.state as any)?.baseRootPickResult;
    if (v && typeof v === "string" && v.length > 0) {
      setNewBaseRoot(v);
      setTimeout(() => {
        navigate(location.pathname, {
          replace: true,
          state: { ...(location.state as any), baseRootPickResult: undefined },
        });
      }, 0);
    }
  }, [location?.state]);

  useEffect(() => {
    const checkWritable = async () => {
      try {
        if (!newBaseRoot) {
          setBaseRootWritable(false);
          return;
        }
        const ok = await CanWriteToDir(newBaseRoot);
        setBaseRootWritable(Boolean(ok));
      } catch {
        setBaseRootWritable(false);
      }
    };
    checkWritable();
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

  return (
    <div className="relative w-full p-4 flex flex-col">
      {/* Background Gradients */}

      <motion.div
        initial={{ y: -8, opacity: 0 }}
        animate={{ y: 0, opacity: 1 }}
        transition={{ duration: 0.18, ease: [0.16, 0.84, 0.44, 1] }}
        className="mb-8"
      >
        <Card className="border-none shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl">
          <CardBody className="p-8">
            <PageHeader
              title={t("settingscard.header.title")}
              description={t("settingscard.header.content")}
            />
          </CardBody>
        </Card>
      </motion.div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Left Column: Paths */}
        <motion.div
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.25, delay: 0.1 }}
        >
          <Card className="h-full border-none shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl">
            <CardBody className="p-6 sm:p-8 flex flex-col gap-6">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <p className="text-large font-bold">
                    {t("settingscard.body.paths.title", {
                      defaultValue: "内容路径",
                    })}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  <Button
                    variant="light"
                    radius="full"
                    size="sm"
                    onPress={() => resetOnOpen()}
                  >
                    {t("settingscard.body.paths.reset", {
                      defaultValue: "恢复默认",
                    })}
                  </Button>
                  <Button
                    color="primary"
                    radius="full"
                    size="sm"
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
                    {t("settingscard.body.paths.apply", {
                      defaultValue: "应用",
                    })}
                  </Button>
                </div>
              </div>

              <div className="space-y-4">
                <Input
                  label={
                    t("settingscard.body.paths.base_root", {
                      defaultValue: "根目录",
                    }) as string
                  }
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
                      onPress={() => {
                        navigate("/filemanager", {
                          state: {
                            directoryPickMode: true,
                            returnTo: "/settings",
                            returnState: {},
                            title: t("settingscard.body.paths.title", {
                              defaultValue: "内容路径",
                            }),
                            initialPath: newBaseRoot || baseRoot || "",
                          },
                        });
                      }}
                    >
                      {t("common.browse", { defaultValue: "选择..." })}
                    </Button>
                  }
                />
                {newBaseRoot && newBaseRoot !== baseRoot && baseRootWritable ? (
                  <div
                    className="text-tiny text-warning-500 px-1"
                    title={newBaseRoot}
                  >
                    {t("settingscard.body.paths.base_root", {
                      defaultValue: "根目录",
                    }) +
                      ": " +
                      newBaseRoot}
                  </div>
                ) : null}
                {!baseRootWritable ? (
                  <div className="text-tiny text-danger-500 px-1">
                    {t("settingscard.body.paths.not_writable", {
                      defaultValue: "目录不可写入",
                    })}
                  </div>
                ) : null}

                <div className="grid grid-cols-1 gap-2 pt-2">
                  <div className="p-3 rounded-xl bg-default-100/50 dark:bg-zinc-800/30 border border-default-200/50 dark:border-white/5">
                    <div
                      className="text-tiny text-default-500 flex items-center gap-2 truncate"
                      title={installerDir || "-"}
                    >
                      <LuHardDrive size={14} />
                      <span className="font-medium">
                        {t("settingscard.body.paths.installer", {
                          defaultValue: "安装器目录",
                        })}
                        :
                      </span>
                      <span className="opacity-70">{installerDir || "-"}</span>
                    </div>
                  </div>
                  <div className="p-3 rounded-xl bg-default-100/50 dark:bg-zinc-800/30 border border-default-200/50 dark:border-white/5">
                    <div
                      className="text-tiny text-default-500 flex items-center gap-2 truncate"
                      title={versionsDir || "-"}
                    >
                      <LuHardDrive size={14} />
                      <span className="font-medium">
                        {t("settingscard.body.paths.versions", {
                          defaultValue: "版本目录",
                        })}
                        :
                      </span>
                      <span className="opacity-70">{versionsDir || "-"}</span>
                    </div>
                  </div>
                </div>
              </div>
            </CardBody>
          </Card>
        </motion.div>

        {/* Right Column: Preferences, GDK, Update, About */}
        <div className="flex flex-col gap-6">
          <motion.div
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.25, delay: 0.15 }}
          >
            <Card className="border-none shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl">
              <CardBody className="p-6 sm:p-8 flex flex-col gap-6">
                {/* Language */}
                <div className="flex items-center justify-between">
                  <div className="flex flex-col">
                    <p className="font-medium text-large">
                      {t("settingscard.body.language.name", {
                        defaultValue: t("app.lang"),
                      })}
                    </p>
                    <p className="text-small text-default-500">
                      {langNames.find((l) => l.code === selectedLang)
                        ?.language || selectedLang}
                    </p>
                    {languageChanged && (
                      <div className="text-tiny text-warning-500 mt-1">
                        {t("settings.lang.changed", {
                          defaultValue: "语言已更改",
                        })}
                      </div>
                    )}
                  </div>
                  <Dropdown>
                    <DropdownTrigger>
                      <Button radius="full" variant="bordered">
                        {t("settingscard.body.language.button", {
                          defaultValue: "更改",
                        })}
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
                        const arr = Array.from(keys as unknown as Set<string>);
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
                        <DropdownItem key={lang.code} textValue={lang.language}>
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
                      {t("settings.discord_rpc.title", {
                        defaultValue: "Discord 游戏状态",
                      })}
                    </p>
                    <p className="text-tiny text-default-500">
                      {t("settings.discord_rpc.desc", {
                        defaultValue: "在 Discord 上显示您的游戏状态",
                      })}
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

                <Divider className="bg-default-200/50" />

                {/* GDK */}
                <div className="flex items-center justify-between">
                  <div className="flex flex-col gap-1">
                    <p className="font-medium">{t("settings.gdk.title")}</p>
                    <p className="text-tiny text-default-500">
                      {t("settings.gdk.path_label", {
                        path: "C:\\Program Files (x86)\\Microsoft GDK",
                      })}
                    </p>
                  </div>
                  {gdkInstalled ? (
                    <Chip color="success" variant="flat" size="sm">
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
              </CardBody>
            </Card>
          </motion.div>

          <motion.div
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.25, delay: 0.2 }}
          >
            <Card className="border-none shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl">
              <CardBody className="p-6 sm:p-8 flex flex-col gap-6">
                <div className="flex items-center justify-between">
                  <div className="flex flex-col">
                    <p className="font-medium text-large">
                      {t("settingscard.body.version.name")}
                    </p>
                    <p className="text-small text-default-500">v{appVersion}</p>
                  </div>
                  {checkingUpdate ? (
                    <Spinner size="sm" color="success" />
                  ) : (
                    <Button
                      radius="full"
                      variant="bordered"
                      onPress={onCheckUpdate}
                    >
                      {t("settingscard.body.version.button")}
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
                            {t("settingscard.body.version.hasnew")} {newVersion}
                          </p>
                          <Button
                            color="primary"
                            radius="full"
                            size="sm"
                            onPress={onUpdate}
                            isDisabled={updating}
                            className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
                            startContent={<RxUpdate />}
                          >
                            {updating
                              ? t("common.updating", { defaultValue: "更新中" })
                              : t(
                                  "settingscard.modal.2.footer.download_button",
                                )}
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
                                a: ({ href, children }) => (
                                  <a
                                    href={href}
                                    target="_blank"
                                    rel="noreferrer"
                                    className="text-emerald-500 underline"
                                  >
                                    {children}
                                  </a>
                                ),
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

                <Divider className="bg-default-200/50" />

                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-4">
                    <div className="w-12 h-12 rounded-2xl bg-linear-to-br from-emerald-500/20 to-teal-500/20 p-2 border border-emerald-500/20 flex items-center justify-center">
                      <img
                        src={Logo}
                        alt="Logo"
                        className="w-full h-full object-contain"
                      />
                    </div>
                    <div>
                      <p className="font-medium text-large">
                        {t("aboutcard.title")}
                      </p>
                      <p className="text-tiny text-default-500">
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
                      <FaGithub size={20} className="text-default-500" />
                    </Button>
                    <Button
                      isIconOnly
                      variant="light"
                      radius="full"
                      onPress={() =>
                        Browser.OpenURL("https://discord.gg/v5R5P4vRZk")
                      }
                    >
                      <FaDiscord size={20} className="text-default-500" />
                    </Button>
                  </div>
                </div>
              </CardBody>
            </Card>
          </motion.div>
        </div>
      </div>

      {/* GDK License */}
      <BaseModal
        size="md"
        isOpen={gdkLicenseDisclosure.isOpen}
        onOpenChange={gdkLicenseDisclosure.onOpenChange}
      >
        <ModalContent className="shadow-none">
          {(onClose) => (
            <>
              <BaseModalHeader>
                {t("settings.gdk.license.title")}
              </BaseModalHeader>
              <BaseModalBody>
                <div className="text-default-700 text-sm">
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
                    onChange={(e) =>
                      setGdkLicenseAccepted(Boolean(e.target.checked))
                    }
                  />
                  <label htmlFor="gdk-license" className="text-small">
                    {t("settings.gdk.license.accept")}
                  </label>
                </div>
              </BaseModalBody>
              <BaseModalFooter>
                <Button variant="light" onPress={onClose}>
                  {t("common.cancel")}
                </Button>
                <Button
                  color="primary"
                  isDisabled={!gdkLicenseAccepted}
                  onPress={() => {
                    onClose?.();
                    try {
                      setGdkDlError("");
                      setGdkDlProgress(null);
                      gdkProgressDisclosure.onOpen();
                      StartGDKDownload(
                        "https://github.bibk.top/microsoft/GDK/releases/download/October-2025-v2510.0.6194/GDK_2510.0.6194.zip",
                      );
                    } catch {}
                  }}
                >
                  {t("downloadmodal.download_button")}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>

      {/* GDK Download Progress */}
      <BaseModal
        size="md"
        isOpen={gdkProgressDisclosure.isOpen}
        onOpenChange={gdkProgressDisclosure.onOpenChange}
        hideCloseButton
        isDismissable={false}
      >
        <ModalContent className="shadow-none">
          {(onClose) => (
            <>
              <BaseModalHeader className="text-medium">
                {t("settings.gdk.download.title")}
              </BaseModalHeader>
              <BaseModalBody>
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
                    <div className="text-small text-default-500">
                      {(() => {
                        const total = gdkDlProgress?.total || 0;
                        const done = gdkDlProgress?.downloaded || 0;
                        const fmt = (n: number) =>
                          `${(n / (1024 * 1024)).toFixed(2)} MB`;
                        const fmtSpd = (bps: number) =>
                          `${(bps / (1024 * 1024)).toFixed(2)} MB/s`;
                        if (total > 0) {
                          const pct = Math.min(
                            100,
                            Math.round((done / total) * 100),
                          );
                          return `${fmt(done)} / ${fmt(
                            total,
                          )} (${pct}%) · ${fmtSpd(gdkDlSpeed || 0)}`;
                        }
                        return `${fmt(done)} · ${fmtSpd(gdkDlSpeed || 0)}`;
                      })()}
                    </div>
                  </div>
                )}
              </BaseModalBody>
              <BaseModalFooter>
                <Button
                  color="danger"
                  variant="light"
                  isDisabled={gdkDlStatus === "done"}
                  onPress={() => {
                    try {
                      CancelGDKDownload();
                    } catch {}
                    onClose?.();
                  }}
                >
                  {t("common.cancel")}
                </Button>
                <Button
                  color="primary"
                  isDisabled={gdkDlStatus !== "done"}
                  onPress={() => onClose?.()}
                >
                  {t("common.ok")}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>

      {/* GDK Install */}
      <BaseModal
        size="md"
        isOpen={gdkInstallDisclosure.isOpen}
        onOpenChange={gdkInstallDisclosure.onOpenChange}
        hideCloseButton
        isDismissable={false}
      >
        <ModalContent className="shadow-none">
          {() => (
            <>
              <BaseModalHeader>
                {t("settings.gdk.install.title")}
              </BaseModalHeader>
              <BaseModalBody>
                <div className="text-small text-default-500">
                  {t("settings.gdk.install.body")}
                </div>
              </BaseModalBody>
            </>
          )}
        </ModalContent>
      </BaseModal>

      <BaseModal
        size="md"
        isOpen={unsavedOpen}
        onOpenChange={unsavedOnOpenChange}
        hideCloseButton
      >
        <ModalContent className="shadow-none">
          {(onClose) => (
            <>
              <BaseModalHeader className="text-warning-600">
                {t("settings.unsaved.title", { defaultValue: "未保存修改" })}
              </BaseModalHeader>
              <BaseModalBody>
                <div className="text-default-700 text-sm">
                  {t("settings.unsaved.body", {
                    defaultValue:
                      "您更改了内容路径但尚未保存。是否保存后离开？",
                  })}
                </div>
                {!baseRootWritable && (
                  <div className="text-tiny text-danger-500 mt-1">
                    {t("settingscard.body.paths.not_writable", {
                      defaultValue: "目录不可写入",
                    })}
                  </div>
                )}
              </BaseModalBody>
              <BaseModalFooter>
                <Button variant="light" onPress={onClose}>
                  {t("settings.unsaved.cancel", { defaultValue: "取消" })}
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
                          onClose();
                          if (pendingNavPath) navigate(pendingNavPath);
                        }
                      }
                    } catch {}
                    setSavingBaseRoot(false);
                  }}
                >
                  {t("settings.unsaved.save", { defaultValue: "保存并离开" })}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>

      <BaseModal
        size="sm"
        isOpen={resetOpen}
        onOpenChange={resetOnOpenChange}
        hideCloseButton
      >
        <ModalContent className="shadow-none">
          {(onClose) => (
            <>
              <BaseModalHeader className="text-danger-600">
                {t("settings.reset.confirm.title", {
                  defaultValue: "恢复默认路径？",
                })}
              </BaseModalHeader>
              <BaseModalBody>
                <div className="text-default-700 text-sm">
                  {t("settings.reset.confirm.body", {
                    defaultValue:
                      "这将把内容路径重置为默认位置。确定要继续吗？",
                  })}
                </div>
              </BaseModalBody>
              <BaseModalFooter>
                <Button variant="light" onPress={onClose}>
                  {t("common.cancel", { defaultValue: "取消" })}
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
                    onClose();
                  }}
                >
                  {t("common.confirm", { defaultValue: "确定" })}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>
    </div>
  );
};

export default SettingsPage;
