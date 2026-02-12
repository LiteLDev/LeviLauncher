import React from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { Dialogs } from "@wailsio/runtime";
import { UnifiedModal } from "@/components/UnifiedModal";
import { DeleteConfirmModal } from "@/components/DeleteConfirmModal";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { cn } from "@/utils/cn";
import {
  Button,
  Card,
  CardBody,
  Input,
  Switch,
  Chip,
  ModalContent,
  Progress,
  useDisclosure,
  Textarea,
  Tabs,
  Tab,
  addToast,
} from "@heroui/react";
import { useTranslation } from "react-i18next";
import { motion } from "framer-motion";
import { FaWindows, FaDownload, FaCogs, FaList } from "react-icons/fa";
import {
  FiAlertTriangle,
  FiCheckCircle,
  FiUploadCloud,
  FiTrash2,
} from "react-icons/fi";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { PageHeader } from "@/components/PageHeader";
import { useLeviLamina } from "@/utils/LeviLaminaContext";
import LeviLaminaIcon from "@/assets/images/LeviLamina.png";

export default function VersionSettingsPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation() as any;
  const initialName: string = String(location?.state?.name || "");
  const returnToPath: string = String(location?.state?.returnTo || "/versions");

  const [targetName, setTargetName] = React.useState<string>(initialName);
  const [selectedTab, setSelectedTab] = React.useState<string>("general");
  const [newName, setNewName] = React.useState<string>(initialName);
  const [gameVersion, setGameVersion] = React.useState<string>("");
  const [versionType, setVersionType] = React.useState<string>("");
  const [isPreview, setIsPreview] = React.useState<boolean>(false);
  const [enableIsolation, setEnableIsolation] = React.useState<boolean>(false);
  const [enableConsole, setEnableConsole] = React.useState<boolean>(false);
  const [enableEditorMode, setEnableEditorMode] =
    React.useState<boolean>(false);
  const [enableRenderDragon, setEnableRenderDragon] =
    React.useState<boolean>(false);
  const [envVars, setEnvVars] = React.useState<string>("");
  const [launchArgs, setLaunchArgs] = React.useState<string>("");
  const [isRegistered, setIsRegistered] = React.useState<boolean>(false);
  const [loading, setLoading] = React.useState<boolean>(true);
  const [unregisterOpen, setUnregisterOpen] = React.useState<boolean>(false);
  const [unregisterSuccessOpen, setUnregisterSuccessOpen] =
    React.useState<boolean>(false);

  const [gdkMissingOpen, setGdkMissingOpen] = React.useState<boolean>(false);
  const [error, setError] = React.useState<string>("");
  const [logoDataUrl, setLogoDataUrl] = React.useState<string>("");
  const [errorOpen, setErrorOpen] = React.useState<boolean>(false);
  const [deleteOpen, setDeleteOpen] = React.useState<boolean>(false);
  const [deleteSuccessOpen, setDeleteSuccessOpen] =
    React.useState<boolean>(false);
  const [shortcutSuccessOpen, setShortcutSuccessOpen] =
    React.useState<boolean>(false);
  const [deleting, setDeleting] = React.useState<boolean>(false);
  const [deleteSuccessMsg, setDeleteSuccessMsg] = React.useState<string>("");

  const [originalIsolation, setOriginalIsolation] =
    React.useState<boolean>(false);
  const [originalConsole, setOriginalConsole] = React.useState<boolean>(false);
  const [originalEditorMode, setOriginalEditorMode] =
    React.useState<boolean>(false);
  const [originalRenderDragon, setOriginalRenderDragon] =
    React.useState<boolean>(false);
  const [originalEnvVars, setOriginalEnvVars] = React.useState<string>("");
  const [originalLaunchArgs, setOriginalLaunchArgs] =
    React.useState<string>("");
  const {
    isOpen: unsavedOpen,
    onOpen: unsavedOnOpen,
    onClose: unsavedOnClose,
    onOpenChange: unsavedOnOpenChange,
  } = useDisclosure();
  const [pendingNavPath, setPendingNavPath] = React.useState<string>("");
  const { isLLSupported, refreshLLDB, llMap } = useLeviLamina();
  const [installingLL, setInstallingLL] = React.useState(false);
  const {
    isOpen: rcOpen,
    onOpen: rcOnOpen,
    onOpenChange: rcOnOpenChange,
    onClose: rcOnClose,
  } = useDisclosure();
  const [rcVersion, setRcVersion] = React.useState("");
  const [isLLInstalled, setIsLLInstalled] = React.useState(false);

  React.useEffect(() => {
    if (selectedTab === "loader" && targetName) {
      (minecraft as any)
        ?.GetMods?.(targetName)
        .then((mods: any[]) => {
          if (mods) {
            const installed = mods.some((m: any) => m.name === "LeviLamina");
            setIsLLInstalled(installed);
          }
        })
        .catch(() => {});
    }
  }, [selectedTab, targetName, installingLL]);

  const hasBackend = minecraft !== undefined;

  React.useEffect(() => {
    if (!hasBackend || !targetName) return;
    (async () => {
      try {
        const getMeta = (minecraft as any)?.GetVersionMeta;
        if (typeof getMeta === "function") {
          const meta: any = await getMeta(targetName);
          if (meta) {
            setGameVersion(String(meta?.gameVersion || ""));
            const type = String(meta?.type || "release").toLowerCase();
            setVersionType(type);
            setIsPreview(type === "preview");
            setEnableIsolation(!!meta?.enableIsolation);
            setOriginalIsolation(!!meta?.enableIsolation);
            setEnableConsole(!!meta?.enableConsole);
            setOriginalConsole(!!meta?.enableConsole);
            setEnableEditorMode(!!meta?.enableEditorMode);
            setOriginalEditorMode(!!meta?.enableEditorMode);
            setEnableRenderDragon(!!meta?.enableRenderDragon);
            setOriginalRenderDragon(!!meta?.enableRenderDragon);
            setEnvVars(String(meta?.envVars || ""));
            setOriginalEnvVars(String(meta?.envVars || ""));
            setLaunchArgs(String(meta?.launchArgs || ""));
            setOriginalLaunchArgs(String(meta?.launchArgs || ""));
            setIsRegistered(Boolean(meta?.registered));
          }
        }
      } catch {}
      try {
        const getter = minecraft?.GetVersionLogoDataUrl;
        if (typeof getter === "function") {
          const u = await getter(targetName);
          setLogoDataUrl(String(u || ""));
        }
      } catch {
        setLogoDataUrl("");
      }

      setLoading(false);
    })();
  }, [hasBackend, targetName]);

  React.useEffect(() => {
    setErrorOpen(!!error);
  }, [error]);

  React.useEffect(() => {
    const handler = (ev: any) => {
      try {
        let targetPath = ev?.detail?.path;
        if (targetPath === -1) targetPath = "-1";
        targetPath = String(targetPath || "");
        const hasUnsaved =
          (newName && newName !== targetName) ||
          enableIsolation !== originalIsolation ||
          enableConsole !== originalConsole ||
          enableEditorMode !== originalEditorMode ||
          enableRenderDragon !== originalRenderDragon ||
          envVars !== originalEnvVars ||
          launchArgs !== originalLaunchArgs;

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
    newName,
    targetName,
    enableIsolation,
    originalIsolation,
    enableConsole,
    originalConsole,
    enableEditorMode,
    originalEditorMode,
    enableRenderDragon,
    originalRenderDragon,
    navigate,
    location.pathname,
    unsavedOnOpen,
  ]);

  const errorDefaults: Record<string, string> = {
    ERR_INVALID_NAME: t("errors.ERR_INVALID_NAME") as string,
    ERR_ICON_DECODE: t("errors.ERR_ICON_DECODE") as string,
    ERR_ICON_NOT_SQUARE: t("errors.ERR_ICON_NOT_SQUARE") as string,
  };
  const getErrorText = (code: string) => {
    if (!code) return "";
    const def = errorDefaults[code] || t(`errors.${code}`);
    return def as string;
  };

  const onSave = React.useCallback(
    async (destPath?: string) => {
      if (!hasBackend || !targetName) {
        navigate(-1);
        return false;
      }
      const validate = minecraft?.ValidateVersionFolderName;
      const rename = minecraft?.RenameVersionFolder;
      const save = minecraft?.SaveVersionMeta;
      const saver = minecraft?.SaveVersionLogoDataUrl;

      const nn = (newName || "").trim();
      if (!nn) {
        setError("ERR_INVALID_NAME");
        return false;
      }
      const type = versionType || (isPreview ? "preview" : "release");

      if (nn !== targetName) {
        if (typeof validate === "function") {
          const msg: string = await validate(nn);
          if (msg) {
            setError(msg);
            return false;
          }
        }
        if (typeof rename === "function") {
          const err: string = await rename(targetName, nn);
          if (err) {
            setError(err);
            return false;
          }
        }
        setTargetName(nn);
      }

      if (typeof save === "function") {
        const err2: string = await save(
          nn,
          gameVersion,
          type,
          !!enableIsolation,
          !!enableConsole,
          !!enableEditorMode,
          !!enableRenderDragon,
          launchArgs,
          envVars,
        );
        if (err2) {
          setError(err2);
          return false;
        }
      }
      try {
        if (typeof saver === "function" && logoDataUrl) {
          const e = await saver(nn, logoDataUrl);
          if (e) {
            setError(e);
            return false;
          }
        }
      } catch {}

      if (destPath === "-1") {
        navigate(-1);
      } else {
        navigate(typeof destPath === "string" ? destPath : returnToPath);
      }
      return true;
    },
    [
      hasBackend,
      targetName,
      newName,
      gameVersion,
      isPreview,
      enableIsolation,
      enableConsole,
      enableEditorMode,
      enableRenderDragon,
      logoDataUrl,
      returnToPath,
      navigate,
      versionType,
      envVars,
      launchArgs,
    ],
  );

  const onDeleteConfirm = React.useCallback(async () => {
    if (!hasBackend || !targetName) {
      setDeleteOpen(false);
      return;
    }
    setDeleting(true);
    try {
      const del = minecraft?.DeleteVersionFolder;
      if (typeof del === "function") {
        const err: string = await del(targetName);
        if (err) {
          setError(String(err));
          setDeleting(false);
          setDeleteOpen(false);
          return;
        }
        setDeleteOpen(false);
        setDeleteSuccessMsg(targetName);
        try {
          const cur = localStorage.getItem("ll.currentVersionName") || "";
          if (cur === targetName)
            localStorage.removeItem("ll.currentVersionName");
        } catch {}
        setDeleteSuccessOpen(true);
      }
    } catch {
      setError("ERR_DELETE_FAILED");
    } finally {
      setDeleting(false);
    }
  }, [hasBackend, targetName]);

  const proceedInstallLeviLamina = async () => {
    if (!hasBackend || !targetName || !gameVersion) return;
    setInstallingLL(true);
    try {
      const isLip = await (minecraft as any)?.IsLipInstalled();
      if (!isLip) {
        addToast({
          description: t("mods.err_lip_not_installed"),
          color: "danger",
        });
        setInstallingLL(false);
        return;
      }

      const installLL = (minecraft as any)?.InstallLeviLamina;
      if (typeof installLL === "function") {
        const err: string = await installLL(gameVersion, targetName);
        if (err) {
          let msg = err;
          if (err.includes("ERR_LIP_INSTALL_FAILED")) {
            msg = t("mods.err_lip_install_failed_suggestion");
          }
          addToast({ description: msg, color: "danger" });
        } else {
          addToast({
            title: t("downloadpage.install.success"),
            color: "success",
          });
        }
      }
    } catch (e: any) {
      addToast({
        description: String(e?.message || e),
        color: "danger",
      });
    } finally {
      setInstallingLL(false);
    }
  };

  const handleInstallLeviLamina = async () => {
    if (!hasBackend || !targetName || !gameVersion) return;

    let targetLLVersion = "";
    if (llMap && llMap.size > 0) {
      let versions = llMap.get(gameVersion);
      if (!versions && gameVersion.split(".").length >= 3) {
        const parts = gameVersion.split(".");
        const key = `${parts[0]}.${parts[1]}.${parts[2]}`;
        versions = llMap.get(key);
      }

      if (versions && Array.isArray(versions) && versions.length > 0) {
        targetLLVersion = versions[versions.length - 1];
      }
    }

    if (targetLLVersion && targetLLVersion.includes("rc")) {
      setRcVersion(targetLLVersion);
      rcOnOpen();
      return;
    }

    await proceedInstallLeviLamina();
  };

  const handleUninstallLL = async () => {
    if (!targetName) return;
    setInstallingLL(true);
    try {
      const err = await (minecraft as any)?.UninstallLeviLamina?.(targetName);
      if (err) {
        addToast({ description: String(err), color: "danger" });
      } else {
        setIsLLInstalled(false);
        addToast({ title: t("common.success"), color: "success" });
      }
    } catch (e) {
      addToast({ description: String(e), color: "danger" });
    } finally {
      setInstallingLL(false);
    }
  };

  return (
    <PageContainer className="relative" animate={false}>
      <div className="flex flex-col gap-6">
        <motion.div
          initial={{ y: -20, opacity: 0 }}
          animate={{ y: 0, opacity: 1 }}
          transition={{ duration: 0.4 }}
        >
          <Card className={LAYOUT.GLASS_CARD.BASE}>
            <CardBody className="p-6 w-full">
              <PageHeader
                title={t("versions.edit.title")}
                titleClassName="text-left pb-1"
                description={
                  <div className="mt-1 text-xs text-default-500 dark:text-zinc-400 truncate text-left">
                    {t("versions.edit.mc_version")}:{" "}
                    <span className="text-default-700 dark:text-zinc-200 font-medium">
                      {loading ? (
                        <span className="inline-block h-4 w-24 rounded bg-default-200 animate-pulse" />
                      ) : (
                        gameVersion ||
                        (t(
                          "launcherpage.version_select.unknown",
                        ) as unknown as string)
                      )}
                    </span>
                    <span className="mx-2 text-default-400">·</span>
                    {t("versions.info.name")}:{" "}
                    <span className="text-default-700 dark:text-zinc-200 font-medium">
                      {targetName || "-"}
                    </span>
                    <span className="mx-2 text-default-400">·</span>
                    {versionType === "preview" ? (
                      <Chip size="sm" variant="flat" color="warning">
                        {t("common.preview")}
                      </Chip>
                    ) : versionType === "release" ? (
                      <span className="text-default-700 dark:text-zinc-300">
                        {t("common.release")}
                      </span>
                    ) : (
                      <Chip size="sm" variant="flat" color="secondary">
                        {versionType || "-"}
                      </Chip>
                    )}
                  </div>
                }
                endContent={
                  <div className="hidden sm:flex items-center gap-3">
                    <Button
                      variant="light"
                      radius="full"
                      onPress={() => navigate(returnToPath)}
                      className="font-medium text-default-600 dark:text-zinc-300"
                    >
                      {t("common.cancel")}
                    </Button>
                    <Button
                      color="primary"
                      radius="full"
                      className="bg-primary-600 hover:bg-primary-500 text-white font-bold shadow-lg shadow-primary-900/20"
                      onPress={() => onSave()}
                    >
                      {t("common.ok")}
                    </Button>
                  </div>
                }
              />
              <Tabs
                aria-label="Version Settings Tabs"
                selectedKey={selectedTab}
                onSelectionChange={(k) => setSelectedTab(k as string)}
                classNames={{ base: "mt-4" }}
              >
                <Tab key="general" title={t("versions.edit.tabs.general")} />
                <Tab key="launch" title={t("versions.edit.tabs.launch")} />
                <Tab key="loader" title={t("versions.edit.tabs.loader")} />
                <Tab key="features" title={t("versions.edit.tabs.features")} />
                <Tab key="manage" title={t("versions.edit.tabs.manage")} />
              </Tabs>
            </CardBody>
          </Card>
        </motion.div>

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
                  <div>
                    <label className="text-small font-medium text-default-700 dark:text-zinc-200 mb-2 block">
                      {t("versions.edit.new_name")}
                    </label>
                    <Input
                      value={newName}
                      onValueChange={(v) => {
                        setNewName(v);
                        if (error) setError("");
                      }}
                      size="md"
                      variant="bordered"
                      radius="lg"
                      classNames={{
                        inputWrapper:
                          "bg-default-100/50 dark:bg-default-100/20 border-default-200 dark:border-default-700 hover:border-primary-500 focus-within:border-primary-500!",
                      }}
                      isDisabled={isRegistered || loading}
                      placeholder={
                        t("versions.edit.placeholder") as unknown as string
                      }
                    />
                    <p className="text-tiny text-default-400 mt-2">
                      {t("versions.edit.hint")}
                    </p>
                  </div>
                  <div className="flex flex-col gap-3">
                    <div className="text-small font-medium text-default-700 dark:text-zinc-200">
                      {t("versions.logo.title")}
                    </div>
                    <div className="flex items-center gap-4">
                      <div
                        className="relative h-24 w-24 rounded-2xl overflow-hidden bg-default-100 flex items-center justify-center border border-default-200 cursor-pointer group transition-all hover:scale-105 hover:shadow-lg"
                        onClick={async () => {
                          try {
                            const paths = await Dialogs.OpenFile({
                              Title: t("versions.logo.title"),
                              Filters: [
                                {
                                  DisplayName: "Image Files",
                                  Pattern: "*.png;*.jpg;*.jpeg;*.gif;*.webp",
                                },
                              ],
                              AllowsMultipleSelection: false,
                            });
                            let path = "";
                            if (Array.isArray(paths) && paths.length > 0) {
                              path = paths[0];
                            } else if (typeof paths === "string" && paths) {
                              path = paths;
                            }

                            if (path) {
                              const saver = minecraft?.SaveVersionLogoFromPath;
                              const getter = minecraft?.GetVersionLogoDataUrl;
                              if (typeof saver === "function") {
                                saver(targetName, path).then((err: string) => {
                                  if (err) {
                                    setError(String(err || "ERR_ICON_DECODE"));
                                    return;
                                  }
                                  if (typeof getter === "function") {
                                    getter(targetName).then((u: string) =>
                                      setLogoDataUrl(String(u || "")),
                                    );
                                  }
                                });
                              }
                            }
                          } catch (e) {
                            console.error(e);
                          }
                        }}
                        title={t("versions.logo.change") as string}
                      >
                        {logoDataUrl ? (
                          <img
                            src={logoDataUrl}
                            alt="logo"
                            className="h-full w-full object-cover"
                          />
                        ) : null}
                        <div className="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 transition flex items-center justify-center text-white text-tiny font-medium backdrop-blur-[2px]">
                          {t("versions.logo.change")}
                        </div>
                      </div>
                      <div className="flex flex-col gap-2">
                        <Button
                          size="sm"
                          variant="flat"
                          color="danger"
                          radius="full"
                          className="px-4 font-medium"
                          onPress={async () => {
                            try {
                              const rm = minecraft?.RemoveVersionLogo;
                              if (typeof rm === "function") {
                                await rm(targetName);
                              }
                            } catch {}
                            setLogoDataUrl("");
                          }}
                        >
                          {t("versions.logo.clear")}
                        </Button>
                        <Button
                          size="sm"
                          variant="flat"
                          color="primary"
                          radius="full"
                          className="justify-start px-4 font-medium bg-primary-500/10 text-primary-600 dark:text-primary-400"
                          onPress={async () => {
                            try {
                              const err: string =
                                await minecraft?.CreateDesktopShortcut(
                                  targetName,
                                );
                              if (err) {
                                setError(String(err));
                              } else {
                                setShortcutSuccessOpen(true);
                              }
                            } catch {
                              setError("ERR_SHORTCUT_CREATE_FAILED");
                            }
                          }}
                          startContent={<FaWindows />}
                        >
                          {
                            t(
                              "launcherpage.shortcut.create_button",
                            ) as unknown as string
                          }
                        </Button>
                      </div>
                    </div>
                    <p className="text-tiny text-default-400">
                      {t("versions.logo.hint")}
                    </p>
                  </div>
                </div>
              )}
              {selectedTab === "launch" && (
                <div className="flex flex-col gap-6">
                  <div className="flex items-center justify-between p-2 rounded-xl hover:bg-default-100/50 transition-colors">
                    <div className="text-medium font-medium">
                      {t("versions.edit.enable_isolation")}
                    </div>
                    <Switch
                      size="md"
                      color="success"
                      isSelected={enableIsolation}
                      onValueChange={setEnableIsolation}
                      classNames={{
                        wrapper: "group-data-[selected=true]:bg-primary-500",
                      }}
                    />
                  </div>
                  <div className="flex items-center justify-between p-2 rounded-xl hover:bg-default-100/50 transition-colors">
                    <div className="text-medium font-medium">
                      {t("versions.edit.enable_console")}
                    </div>
                    <Switch
                      size="md"
                      color="success"
                      isSelected={enableConsole}
                      onValueChange={setEnableConsole}
                      classNames={{
                        wrapper: "group-data-[selected=true]:bg-primary-500",
                      }}
                    />
                  </div>
                  <div className="flex flex-col gap-3">
                    <label className="text-small font-medium text-default-700 dark:text-zinc-200 block">
                      {t("versions.edit.launch_args")}
                    </label>
                    <Input
                      value={launchArgs}
                      onValueChange={(v) => {
                        setLaunchArgs(v);
                        if (error) setError("");
                      }}
                      size="md"
                      variant="bordered"
                      radius="lg"
                      placeholder={
                        t(
                          "versions.edit.launch_args_placeholder",
                        ) as unknown as string
                      }
                      classNames={{
                        inputWrapper:
                          "bg-default-100/50 dark:bg-default-100/20 border-default-200 dark:border-default-700 hover:border-primary-500 focus-within:border-primary-500!",
                      }}
                    />
                    <p className="text-tiny text-default-400">
                      {t("versions.edit.launch_args_hint")}
                    </p>
                  </div>
                  <div className="flex flex-col gap-3">
                    <label className="text-small font-medium text-default-700 dark:text-zinc-200 block">
                      {t("versions.edit.env_vars")}
                    </label>
                    <Textarea
                      value={envVars}
                      onValueChange={(v) => {
                        setEnvVars(v);
                        if (error) setError("");
                      }}
                      minRows={3}
                      variant="bordered"
                      radius="lg"
                      placeholder={
                        t(
                          "versions.edit.env_vars_placeholder",
                        ) as unknown as string
                      }
                      classNames={{
                        inputWrapper:
                          "bg-default-100/50 dark:bg-default-100/20 border-default-200 dark:border-default-700 hover:border-primary-500 focus-within:border-primary-500!",
                      }}
                    />
                    <p className="text-tiny text-default-400">
                      {t("versions.edit.env_vars_hint")}
                    </p>
                  </div>
                </div>
              )}
              {selectedTab === "loader" && (
                <div className="flex flex-col gap-6">
                  <div className="flex items-center justify-between p-4 rounded-xl border border-default-200 dark:border-default-100/10 bg-default-50 dark:bg-default-100/10">
                    <div className="flex items-center gap-4">
                      <div className="w-12 h-12 rounded-xl overflow-hidden">
                        <img
                          src={LeviLaminaIcon}
                          alt="LeviLamina"
                          className="w-full h-full object-cover"
                        />
                      </div>
                      <div>
                        <div className="text-medium font-bold text-foreground">
                          LeviLamina
                        </div>
                        <div className="text-small text-default-500 dark:text-zinc-400">
                          {t("downloadpage.install.levilamina_desc")}
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-3">
                      {isLLSupported(gameVersion) ? (
                        <>
                          {isLLInstalled && (
                            <Button
                              color="danger"
                              variant="flat"
                              onPress={handleUninstallLL}
                              isDisabled={installingLL}
                            >
                              {t("common.remove")}
                            </Button>
                          )}
                          <Button
                            color="success"
                            variant="flat"
                            className="bg-primary-500/10 text-primary-600 dark:text-primary-400 font-bold"
                            onPress={() => {
                              if (isLLInstalled) {
                                addToast({
                                  title: "提示",
                                  description: "该功能暂时不可用",
                                  color: "warning",
                                });
                                return;
                              }
                              handleInstallLeviLamina();
                            }}
                            isLoading={installingLL}
                          >
                            {isLLInstalled
                              ? t("common.update")
                              : t("downloadpage.install.levilamina_label")}
                          </Button>
                        </>
                      ) : (
                        <div className="text-small text-default-400 italic">
                          {t("downloadpage.install.levilamina_unsupported") ||
                            "Not Supported"}
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              )}
              {selectedTab === "features" && (
                <div className="flex flex-col gap-5">
                  <div className="flex items-center justify-between p-2 rounded-xl hover:bg-default-100/50 transition-colors">
                    <div className="text-medium font-medium">
                      {t("versions.edit.enable_render_dragon")}
                    </div>
                    <Switch
                      size="md"
                      color="success"
                      isSelected={enableRenderDragon}
                      onValueChange={setEnableRenderDragon}
                      classNames={{
                        wrapper: "group-data-[selected=true]:bg-primary-500",
                      }}
                    />
                  </div>
                  <div className="flex items-center justify-between p-2 rounded-xl hover:bg-default-100/50 transition-colors">
                    <div className="text-medium font-medium">
                      {t("versions.edit.enable_editor_mode")}
                    </div>
                    <Switch
                      size="md"
                      color="success"
                      isSelected={enableEditorMode}
                      onValueChange={setEnableEditorMode}
                      classNames={{
                        wrapper: "group-data-[selected=true]:bg-primary-500",
                      }}
                    />
                  </div>
                </div>
              )}
              {selectedTab === "manage" && (
                <div className="flex flex-col gap-1">
                  <div className="text-medium font-bold text-default-900 dark:text-zinc-100">
                    {isRegistered
                      ? t("versions.edit.unregister_button")
                      : t("common.delete")}
                  </div>
                  <div className="text-small text-default-500 dark:text-zinc-400 mb-4 max-w-lg">
                    {isRegistered
                      ? t("versions.edit.unregister_hint")
                      : t("versions.edit.delete_hint")}
                  </div>
                  <div>
                    {isRegistered ? (
                      <Button
                        color="warning"
                        variant="flat"
                        radius="lg"
                        isDisabled={loading}
                        className="font-medium"
                        onPress={async () => {
                          try {
                            const has = await (
                              minecraft as any
                            )?.IsGDKInstalled?.();
                            if (!has) {
                              setUnregisterOpen(false);
                              setGdkMissingOpen(true);
                              return;
                            }
                            const fn = (minecraft as any)
                              ?.UnregisterVersionByName;
                            if (typeof fn === "function") {
                              setUnregisterOpen(true);
                              const err: string = await fn(targetName);
                              setUnregisterOpen(false);
                              if (err) {
                                setError(String(err));
                              } else {
                                setIsRegistered(false);
                                setUnregisterSuccessOpen(true);
                              }
                            }
                          } catch {
                            setUnregisterOpen(false);
                            setError("ERR_UNREGISTER_FAILED");
                          }
                        }}
                      >
                        {t("versions.edit.unregister_button")}
                      </Button>
                    ) : (
                      <Button
                        color="danger"
                        variant="flat"
                        radius="lg"
                        className="font-medium"
                        onPress={() => setDeleteOpen(true)}
                      >
                        {t("common.delete")}
                      </Button>
                    )}
                  </div>
                </div>
              )}
            </CardBody>
          </Card>
        </motion.div>
      </div>

      <UnifiedModal
        isOpen={unregisterOpen}
        onOpenChange={(open) => {
          if (!open) setUnregisterOpen(false);
        }}
        hideCloseButton
        isDismissable={false}
        type="warning"
        title={t("versions.edit.unregister_progress.title")}
        icon={<FiAlertTriangle className="w-6 h-6 text-warning-500" />}
      >
        <div className="text-medium font-medium text-default-600 dark:text-zinc-300 mb-4">
          {t("versions.edit.unregister_progress.body")}
        </div>
        <Progress
          size="sm"
          isIndeterminate
          aria-label="Unregistering"
          classNames={{ indicator: "bg-warning-500" }}
        />
      </UnifiedModal>

      <UnifiedModal
        isOpen={unregisterSuccessOpen}
        onOpenChange={(open) => {
          if (!open) setUnregisterSuccessOpen(false);
        }}
        size="md"
        type="success"
        title={t("versions.edit.unregister_success.title")}
        icon={<FiCheckCircle className="w-6 h-6 text-success-500" />}
        onConfirm={() => {
          setUnregisterSuccessOpen(false);
        }}
        confirmText={t("launcherpage.delete.complete.close_button")}
        showCancelButton={false}
      >
        <div className="text-medium font-medium text-default-600 dark:text-zinc-300">
          {t("versions.edit.unregister_success.body")}
        </div>
      </UnifiedModal>

      <UnifiedModal
        isOpen={gdkMissingOpen}
        onOpenChange={(open) => {
          if (!open) setGdkMissingOpen(false);
        }}
        type="warning"
        title={t("launcherpage.gdk_missing.title")}
        icon={<FiAlertTriangle className="w-6 h-6 text-warning-500" />}
        footer={
          <>
            <Button
              variant="light"
              radius="full"
              onPress={() => {
                setGdkMissingOpen(false);
              }}
            >
              {t("common.cancel")}
            </Button>
            <Button
              color="primary"
              radius="full"
              className="bg-primary-600 hover:bg-primary-500 text-white font-bold shadow-lg shadow-primary-900/20"
              onPress={() => {
                setGdkMissingOpen(false);
                navigate("/settings");
              }}
            >
              {t("launcherpage.gdk_missing.go_settings")}
            </Button>
          </>
        }
      >
        <div className="text-medium font-medium text-default-600 dark:text-zinc-300">
          {t("launcherpage.gdk_missing.body")}
        </div>
      </UnifiedModal>

      <UnifiedModal
        isOpen={errorOpen}
        onOpenChange={(open) => {
          if (!open) setErrorOpen(false);
        }}
        hideCloseButton
        type="error"
        title={t("common.error")}
        icon={<FiAlertTriangle className="w-6 h-6 text-danger-500" />}
        onConfirm={() => {
          setError("");
          setErrorOpen(false);
        }}
        confirmText={t("common.ok")}
        showCancelButton={false}
      >
        <div className="p-4 rounded-2xl bg-danger-50 dark:bg-danger-500/10 border border-danger-100 dark:border-danger-500/20 text-danger-600 dark:text-danger-400 font-medium">
          <div className="text-medium wrap-break-word">
            {getErrorText(error)}
          </div>
        </div>
      </UnifiedModal>

      <UnifiedModal
        isOpen={shortcutSuccessOpen}
        onOpenChange={(open) => {
          if (!open) setShortcutSuccessOpen(false);
        }}
        size="md"
        type="success"
        title={t("launcherpage.shortcut.success.title")}
        icon={<FiCheckCircle className="w-6 h-6 text-success-500" />}
        onConfirm={() => {
          setShortcutSuccessOpen(false);
        }}
        confirmText={t("common.close")}
        showCancelButton={false}
      >
        <div className="text-medium font-medium text-default-600 dark:text-zinc-300">
          {t("launcherpage.shortcut.success.body")}
        </div>
      </UnifiedModal>

      <DeleteConfirmModal
        isOpen={deleteOpen}
        onOpenChange={setDeleteOpen}
        onConfirm={onDeleteConfirm}
        title={t("launcherpage.delete.confirm.title")}
        description={t("launcherpage.delete.confirm.content")}
        itemName={targetName}
        isPending={deleting}
      />

      <UnifiedModal
        isOpen={deleteSuccessOpen}
        onOpenChange={(open) => {
          if (!open) setDeleteSuccessOpen(open);
        }}
        size="md"
        type="success"
        title={t("launcherpage.delete.complete.title")}
        icon={<FiCheckCircle className="w-6 h-6 text-success-500" />}
        onConfirm={() => {
          setDeleteSuccessOpen(false);
          navigate(returnToPath);
        }}
        confirmText={t("launcherpage.delete.complete.close_button")}
        showCancelButton={false}
      >
        <div className="text-medium font-medium text-default-600 dark:text-zinc-300">
          {t("launcherpage.delete.complete.content")}
          {deleteSuccessMsg ? (
            <span className="font-mono text-default-700 dark:text-zinc-200 font-bold">
              {" "}
              {deleteSuccessMsg}
            </span>
          ) : null}
        </div>
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
              onPress={async () => {
                const ok = await onSave(pendingNavPath);
                if (ok) {
                  unsavedOnClose();
                }
              }}
            >
              {t("settings.unsaved.save")}
            </Button>
          </>
        }
      >
        <div className="text-default-700 dark:text-zinc-300 text-sm">
          {t("versions.unsaved.body")}
        </div>
      </UnifiedModal>

      <UnifiedModal
        size="md"
        isOpen={rcOpen}
        onOpenChange={rcOnOpenChange}
        hideCloseButton
        type="warning"
        title={t("mods.rc_warning.title")}
        icon={<FiAlertTriangle className="w-6 h-6 text-warning-500" />}
        footer={
          <>
            <Button variant="light" onPress={rcOnClose}>
              {t("common.cancel")}
            </Button>
            <Button
              color="warning"
              className="text-white font-bold shadow-lg shadow-warning-500/20"
              onPress={() => {
                rcOnClose();
                proceedInstallLeviLamina();
              }}
            >
              {t("common.continue")}
            </Button>
          </>
        }
      >
        <div className="text-sm text-default-700 dark:text-zinc-300 space-y-2">
          <p>
            {t("mods.rc_warning.body_1", {
              version: rcVersion,
            })}
          </p>
          <p className="font-semibold text-warning-700">
            {t("mods.rc_warning.body_2")}
          </p>
          <p>{t("mods.rc_warning.body_3")}</p>
        </div>
      </UnifiedModal>
    </PageContainer>
  );
}
