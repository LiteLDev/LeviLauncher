import React, { useEffect, useState } from "react";
import { useNavigate, useLocation } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { motion, AnimatePresence } from "framer-motion";
import {
  Card,
  CardHeader,
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
  Modal,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  useDisclosure,
  Switch,
} from "@heroui/react";
import { IoSettingsOutline } from "react-icons/io5";
import { RxUpdate } from "react-icons/rx";
import { FaGithub, FaDiscord } from "react-icons/fa";
import { LuFolderOpen, LuHardDrive } from "react-icons/lu";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import {
  GetAppVersion,
  CheckUpdate,
  Update,
  GetLanguageNames,
  GetBaseRoot,
  SetBaseRoot,
  ResetBaseRoot,
  GetInstallerDir,
  GetVersionsDir,
  OpenPathDir,
  CanWriteToDir,
  IsGDKInstalled,
  StartGDKDownload,
  CancelGDKDownload,
  InstallGDKFromZip,
  GetDisableDiscordRPC,
  SetDisableDiscordRPC,
} from "../../bindings/github.com/liteldev/LeviLauncher/minecraft";
import { Browser, Events } from "@wailsio/runtime";
import * as types from "../../bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import * as minecraft from "../../bindings/github.com/liteldev/LeviLauncher/minecraft";

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
    if (hasBackend) {
      GetLanguageNames().then((res) => setLangNames(res));
    } else {
      setLangNames([
        { language: "English", code: "en_US" } as unknown as types.LanguageJson,
        {
          language: "简体中文",
          code: "zh_CN",
        } as unknown as types.LanguageJson,
        {
          language: "繁体中文",
          code: "zh_HK",
        } as unknown as types.LanguageJson,
        {
          language: "日本語",
          code: "ja_JP",
        } as unknown as types.LanguageJson,
      ]);
    }
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
                : String(br || "")
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
    const lastRef: { ts: number; bytes: number } | null = null as any;
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
        })
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
        })
      );
      offs.push(
        Events.On("gdk_download_error", (event) => {
          setGdkDlError(String(event?.data || ""));
        })
      );
      offs.push(
        Events.On("gdk_download_done", async (event) => {
          const dest = String(event?.data || gdkDlProgress?.dest || "");
          gdkProgressDisclosure.onClose();
          try {
            gdkInstallDisclosure.onOpen();
            await InstallGDKFromZip(dest);
          } catch {}
        })
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
        })
      );
      offs.push(
        Events.On("gdk_install_error", (event) => {
          gdkInstallDisclosure.onClose();
          setGdkDlError(String(event?.data || ""));
        })
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
    <div className="px-3 sm:px-5 lg:px-8 py-2 sm:py-4 lg:py-6 w-full max-w-none">
      <motion.div
        initial={{ opacity: 0, y: 8 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.25 }}
      >
        <Card className="rounded-3xl shadow-xl p-2 bg-white/60 dark:bg-black/30 backdrop-blur-md border border-white/30">
          <CardHeader className="flex flex-col items-start px-4 pb-0 pt-4">
            <div className="flex items-center">
              <IoSettingsOutline size={24} className="spin-animation" />
              <p className="text-large ml-2">
                {t("settingscard.header.title")}
              </p>
            </div>
            <p className="text-small text-default-500">
              {t("settingscard.header.content")}
            </p>
          </CardHeader>
          <Divider className="my-2 bg-default-200/60 h-px" />
          <CardBody className="space-y-4">
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 items-start">
              <motion.div
                initial={{ opacity: 0, y: 6 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.25, delay: 0.04 }}
                className="rounded-2xl p-4 bg-default-100/40 dark:bg-default-50/20 border border-default-200/50"
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <LuFolderOpen size={18} />
                    <p className="font-medium">
                      {t("settingscard.body.paths.title", {
                        defaultValue: "内容路径",
                      })}
                    </p>
                  </div>
                  <div className="flex items-center gap-1.5">
                    <Button
                      color="primary"
                      radius="full"
                      isDisabled={!newBaseRoot || !baseRootWritable}
                      isLoading={savingBaseRoot}
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
                    <Button
                      variant="light"
                      radius="full"
                      onPress={() => {
                        resetOnOpen();
                      }}
                    >
                      {t("settingscard.body.paths.reset", {
                        defaultValue: "恢复默认",
                      })}
                    </Button>
                  </div>
                </div>
                <Divider className="my-3 bg-default-200/60 h-px" />
                <div className="space-y-2">
                  <Input
                    label={
                      t("settingscard.body.paths.base_root", {
                        defaultValue: "根目录",
                      }) as string
                    }
                    value={newBaseRoot}
                    onValueChange={setNewBaseRoot}
                    endContent={
                      <Button
                        size="sm"
                        variant="flat"
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
                  {newBaseRoot &&
                  newBaseRoot !== baseRoot &&
                  baseRootWritable ? (
                    <div
                      className="text-tiny text-warning-500"
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
                    <div className="text-tiny text-danger-500">
                      {t("settingscard.body.paths.not_writable", {
                        defaultValue: "目录不可写入",
                      })}
                    </div>
                  ) : null}
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 pt-1">
                    <div
                      className="text-tiny text-default-500 flex items-center gap-1 truncate"
                      title={installerDir || "-"}
                    >
                      <LuHardDrive size={14} />
                      {t("settingscard.body.paths.installer", {
                        defaultValue: "安装器目录",
                      })}
                      : {installerDir || "-"}
                    </div>
                    <div
                      className="text-tiny text-default-500 flex items-center gap-1 truncate"
                      title={versionsDir || "-"}
                    >
                      <LuHardDrive size={14} />
                      {t("settingscard.body.paths.versions", {
                        defaultValue: "版本目录",
                      })}
                      : {versionsDir || "-"}
                    </div>
                  </div>
                </div>
              </motion.div>

              <motion.div
                initial={{ opacity: 0, y: 6 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.25, delay: 0.06 }}
                className="rounded-2xl p-4 bg-default-100/40 dark:bg-default-50/20 border border-default-200/50"
              >
                <div className="flex items-center justify-between">
                  <p className="font-medium">
                    {t("settingscard.body.language.name", {
                      defaultValue: t("app.lang"),
                    })}
                  </p>
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
                            }
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
                <Divider className="my-3 bg-default-200/60 h-px" />
                <div className="text-small text-default-500">
                  {langNames.find((l) => l.code === selectedLang)?.language ||
                    selectedLang}
                </div>
                {languageChanged ? (
                  <div className="text-tiny text-warning-500 mt-1">
                    {t("settings.lang.changed", { defaultValue: "语言已更改" })}
                  </div>
                ) : null}
              </motion.div>

              <motion.div
                initial={{ opacity: 0, y: 6 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.25, delay: 0.07 }}
                className="rounded-2xl p-4 bg-default-100/40 dark:bg-default-50/20 border border-default-200/50"
              >
                <div className="flex items-center justify-between">
                  <p className="font-medium">
                    {t("settings.discord_rpc.title", {
                      defaultValue: "Discord 游戏状态",
                    })}
                  </p>
                  <Switch
                    size="sm"
                    isSelected={discordRpcEnabled}
                    onValueChange={(isSelected) => {
                      setDiscordRpcEnabled(isSelected);
                      SetDisableDiscordRPC(!isSelected);
                    }}
                  />
                </div>
                <Divider className="my-3 bg-default-200/60 h-px" />
                <div className="text-small text-default-500">
                  {t("settings.discord_rpc.desc", {
                    defaultValue: "在 Discord 上显示您的游戏状态",
                  })}
                </div>
              </motion.div>

              <motion.div
                initial={{ opacity: 0, y: 6 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.25, delay: 0.07 }}
                className="rounded-2xl p-4 bg-default-100/40 dark:bg-default-50/20 border border-default-200/50"
              >
                <div className="flex items-center justify-between">
                  <p className="font-medium">{t("settings.gdk.title")}</p>
                  <div className="flex items-center gap-2">
                    {gdkInstalled ? (
                      <Chip color="success" variant="flat">
                        {t("settings.gdk.installed")}
                      </Chip>
                    ) : (
                      <Button
                        radius="full"
                        variant="bordered"
                        onPress={() => {
                          setGdkLicenseAccepted(false);
                          gdkLicenseDisclosure.onOpen();
                        }}
                      >
                        {t("settings.gdk.install_button")}
                      </Button>
                    )}
                  </div>
                </div>
                <Divider className="my-3 bg-default-200/60 h-px" />
                <div className="text-small text-default-500">
                  {t("settings.gdk.path_label", {
                    path: "C:\\Program Files (x86)\\Microsoft GDK",
                  })}
                </div>
              </motion.div>

              <motion.div
                initial={{ opacity: 0, y: 6 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.25, delay: 0.08 }}
                className="lg:col-span-2 rounded-2xl p-4 bg-default-100/40 dark:bg-default-50/20 border border-default-200/50"
              >
                <div className="flex items-center justify-between">
                  <p className="font-medium">
                    {t("settingscard.body.version.name")}
                  </p>
                  <div className="flex items-center gap-2">
                    {checkingUpdate ? (
                      <Spinner size="sm" />
                    ) : (
                      <motion.div
                        whileHover={{ scale: 1.03 }}
                        whileTap={{ scale: 0.97 }}
                      >
                        <Button
                          radius="full"
                          variant="bordered"
                          onPress={onCheckUpdate}
                        >
                          {t("settingscard.body.version.button")}
                        </Button>
                      </motion.div>
                    )}
                  </div>
                </div>
                <Divider className="my-3 bg-default-200/60 h-px" />
                <div className="space-y-2">
                  <div className="text-small text-default-500">
                    {appVersion}
                  </div>
                  <AnimatePresence>
                    {hasUpdate ? (
                      <motion.div
                        key="hasUpdate"
                        initial={{ opacity: 0, y: 6 }}
                        animate={{ opacity: 1, y: 0 }}
                        exit={{ opacity: 0, y: 6 }}
                        transition={{ duration: 0.2 }}
                        className="flex items-center justify-between"
                      >
                        <p className="text-small text-danger-500">
                          {t("settingscard.body.version.hasnew")} {newVersion}
                        </p>
                        <motion.div
                          whileHover={{ scale: 1.03 }}
                          whileTap={{ scale: 0.97 }}
                        >
                          <Button
                            color="primary"
                            radius="full"
                            onPress={onUpdate}
                            isDisabled={updating}
                            startContent={<RxUpdate />}
                          >
                            {updating
                              ? t("common.updating", { defaultValue: "更新中" })
                              : t(
                                  "settingscard.modal.2.footer.download_button"
                                )}
                          </Button>
                        </motion.div>
                      </motion.div>
                    ) : null}
                  </AnimatePresence>
                  {hasUpdate && changelog ? (
                    <div className="mt-1 rounded-md bg-default-100/60 border border-default-200 px-3 py-2">
                      <div className="text-small font-semibold mb-1">
                        {t("downloadpage.changelog.title", {
                          defaultValue: "最新更新日志",
                        })}
                      </div>
                      <div className="text-small break-words leading-6 max-h-[24vh] sm:max-h-[32vh] overflow-y-auto pr-1">
                        <ReactMarkdown
                          remarkPlugins={[remarkGfm]}
                          components={{
                            h1: ({ children }) => (
                              <h1 className="text-xl font-semibold mt-2 mb-2">
                                {children}
                              </h1>
                            ),
                            h2: ({ children }) => (
                              <h2 className="text-lg font-semibold mt-2 mb-2">
                                {children}
                              </h2>
                            ),
                            h3: ({ children }) => (
                              <h3 className="text-base font-semibold mt-2 mb-2">
                                {children}
                              </h3>
                            ),
                            p: ({ children }) => (
                              <p className="my-1">{children}</p>
                            ),
                            ul: ({ children }) => (
                              <ul className="list-disc pl-6 my-2">
                                {children}
                              </ul>
                            ),
                            ol: ({ children }) => (
                              <ol className="list-decimal pl-6 my-2">
                                {children}
                              </ol>
                            ),
                            li: ({ children }) => (
                              <li className="my-1">{children}</li>
                            ),
                            a: ({ href, children }) => (
                              <a
                                href={href}
                                target="_blank"
                                rel="noreferrer"
                                className="text-primary underline"
                              >
                                {children}
                              </a>
                            ),
                            hr: () => (
                              <hr className="my-3 border-default-200" />
                            ),
                          }}
                        >
                          {changelog}
                        </ReactMarkdown>
                      </div>
                    </div>
                  ) : null}
                  {updating ? (
                    <div className="py-2">
                      <Progress
                        size="md"
                        radius="sm"
                        aria-label="Installation Progress"
                        color="warning"
                        isIndeterminate={true}
                      />
                    </div>
                  ) : null}
                </div>
              </motion.div>

              <motion.div
                initial={{ opacity: 0, y: 6 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.25, delay: 0.1 }}
                className="lg:col-span-2 rounded-2xl p-4 bg-default-100/40 dark:bg-default-50/20 border border-default-200/50"
              >
                <div className="flex items-center justify-between">
                  <p className="font-medium">{t("aboutcard.title")}</p>
                  <div className="flex items-center gap-2">
                    <motion.div
                      whileHover={{ scale: 1.08 }}
                      whileTap={{ scale: 0.95 }}
                    >
                      <Button
                        isIconOnly
                        variant="light"
                        onPress={() => {
                          Browser.OpenURL("https://github.com/liteldev");
                        }}
                      >
                        <FaGithub size={24} />
                      </Button>
                    </motion.div>
                    <motion.div
                      whileHover={{ scale: 1.08 }}
                      whileTap={{ scale: 0.95 }}
                    >
                      <Button
                        isIconOnly
                        variant="light"
                        onPress={() => {
                          Browser.OpenURL("https://discord.gg/v5R5P4vRZk");
                        }}
                      >
                        <FaDiscord size={24} />
                      </Button>
                    </motion.div>
                  </div>
                </div>
                <Divider className="my-3 bg-default-200/60 h-px" />
                <p className="text-small text-default-500">
                  {t("aboutcard.description", { name: "LeviMC" })}
                </p>
                <p className="text-small text-default-500">
                  {t("aboutcard.font", { name: "MiSans" })}
                </p>
              </motion.div>
            </div>
          </CardBody>
        </Card>
      </motion.div>
      {/* GDK License */}
      <Modal
        size="md"
        isOpen={gdkLicenseDisclosure.isOpen}
        onOpenChange={gdkLicenseDisclosure.onOpenChange}
      >
        <ModalContent>
          {(onClose) => (
            <>
              <ModalHeader>{t("settings.gdk.license.title")}</ModalHeader>
              <ModalBody>
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
              </ModalBody>
              <ModalFooter>
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
                        "https://github.bibk.top/microsoft/GDK/releases/download/October-2025-v2510.0.6194/GDK_2510.0.6194.zip"
                      );
                    } catch {}
                  }}
                >
                  {t("downloadmodal.download_button")}
                </Button>
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>

      {/* GDK Download Progress */}
      <Modal
        size="md"
        isOpen={gdkProgressDisclosure.isOpen}
        onOpenChange={gdkProgressDisclosure.onOpenChange}
        hideCloseButton
        isDismissable={false}
      >
        <ModalContent>
          {(onClose) => (
            <>
              <ModalHeader className="text-medium">
                {t("settings.gdk.download.title")}
              </ModalHeader>
              <ModalBody>
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
                            Math.round((done / total) * 100)
                          );
                          return `${fmt(done)} / ${fmt(
                            total
                          )} (${pct}%) · ${fmtSpd(gdkDlSpeed || 0)}`;
                        }
                        return `${fmt(done)} · ${fmtSpd(gdkDlSpeed || 0)}`;
                      })()}
                    </div>
                  </div>
                )}
              </ModalBody>
              <ModalFooter>
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
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>

      {/* GDK Install */}
      <Modal
        size="md"
        isOpen={gdkInstallDisclosure.isOpen}
        onOpenChange={gdkInstallDisclosure.onOpenChange}
        hideCloseButton
        isDismissable={false}
      >
        <ModalContent>
          {() => (
            <>
              <ModalHeader>{t("settings.gdk.install.title")}</ModalHeader>
              <ModalBody>
                <div className="text-small text-default-500">
                  {t("settings.gdk.install.body")}
                </div>
              </ModalBody>
            </>
          )}
        </ModalContent>
      </Modal>
      <Modal
        size="md"
        isOpen={unsavedOpen}
        onOpenChange={unsavedOnOpenChange}
        hideCloseButton
      >
        <ModalContent>
          {(onClose) => (
            <>
              <ModalHeader className="text-warning-600">
                {t("settings.unsaved.title", { defaultValue: "未保存修改" })}
              </ModalHeader>
              <ModalBody>
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
              </ModalBody>
              <ModalFooter>
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
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>
      <Modal
        size="sm"
        isOpen={resetOpen}
        onOpenChange={resetOnOpenChange}
        hideCloseButton
      >
        <ModalContent>
          {(onClose) => (
            <>
              <ModalHeader className="text-danger-600">
                {t("settings.reset.confirm.title", {
                  defaultValue: "恢复默认路径？",
                })}
              </ModalHeader>
              <ModalBody>
                <div className="text-default-700 text-sm">
                  {t("settings.reset.confirm.body", {
                    defaultValue:
                      "这将把内容路径重置为默认位置。确定要继续吗？",
                  })}
                </div>
              </ModalBody>
              <ModalFooter>
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
                        setBaseRootWritable(true);
                      }
                    } catch {}
                    onClose();
                  }}
                >
                  {t("common.confirm", { defaultValue: "确定" })}
                </Button>
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>
    </div>
  );
};
