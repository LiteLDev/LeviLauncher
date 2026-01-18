import React from "react";
import { useLocation, useNavigate } from "react-router-dom";
import {
  BaseModal,
  BaseModalHeader,
  BaseModalBody,
  BaseModalFooter,
} from "@/components/BaseModal";
import {
  Button,
  Card,
  CardBody,
  Input,
  Switch,
  Chip,
  ModalContent,
  Progress,
  Spinner,
  useDisclosure,
} from "@heroui/react";
import { useTranslation } from "react-i18next";
import { motion } from "framer-motion";
import { FaWindows } from "react-icons/fa";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { PageHeader } from "@/components/PageHeader";

export default function VersionSettingsPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation() as any;
  const initialName: string = String(location?.state?.name || "");
  const returnToPath: string = String(location?.state?.returnTo || "/versions");

  const [targetName, setTargetName] = React.useState<string>(initialName);
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
  const {
    isOpen: unsavedOpen,
    onOpen: unsavedOnOpen,
    onOpenChange: unsavedOnOpenChange,
  } = useDisclosure();
  const [pendingNavPath, setPendingNavPath] = React.useState<string>("");

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
        const targetPath = String(ev?.detail?.path || "");
        const hasUnsaved =
          (newName && newName !== targetName) ||
          enableIsolation !== originalIsolation ||
          enableConsole !== originalConsole ||
          enableEditorMode !== originalEditorMode ||
          enableRenderDragon !== originalRenderDragon;

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
    const def =
      errorDefaults[code] || t(`errors.${code}`, { defaultValue: code });
    return def as string;
  };

  React.useEffect(() => {
    const result: string[] | undefined = location?.state?.fileManagerResult;
    if (!hasBackend || !targetName) return;
    if (result && Array.isArray(result) && result.length === 1) {
      const nextState = {
        ...(location.state || {}),
        fileManagerResult: undefined,
      };
      navigate(location.pathname, { replace: true, state: nextState });
      try {
        const saver = minecraft?.SaveVersionLogoFromPath;
        const getter = minecraft?.GetVersionLogoDataUrl;
        if (typeof saver === "function") {
          saver(targetName, result[0]).then((err: string) => {
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
      } catch {}
    }
  }, [
    hasBackend,
    targetName,
    location?.state?.fileManagerResult,
    navigate,
    location?.pathname,
  ]);

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

      navigate(typeof destPath === "string" ? destPath : returnToPath);
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

  return (
    <div className="w-full h-full flex flex-col overflow-hidden relative">
      <div className="z-20 px-3 sm:px-5 lg:px-8 pt-3 sm:pt-4 lg:pt-6 shrink-0">
        <motion.div
          initial={{ y: -8, opacity: 0 }}
          animate={{ y: 0, opacity: 1 }}
          transition={{ duration: 0.18, ease: [0.16, 0.84, 0.44, 1] }}
        >
          <Card className="border-none shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl mb-6">
            <CardBody className="px-6 sm:px-8 py-5 w-full">
              <PageHeader
                title={t("versions.edit.title")}
                titleClassName="text-left pb-1"
                description={
                  <div className="mt-1 text-sm text-default-500 truncate text-left text-left!">
                    {t("versions.info.version")}:{" "}
                    <span className="text-default-700 font-medium">
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
                    <span className="text-default-700 font-medium">
                      {targetName || "-"}
                    </span>
                    <span className="mx-2 text-default-400">·</span>
                    {versionType === "preview" ? (
                      <Chip size="sm" variant="flat" color="warning">
                        {t("common.preview")}
                      </Chip>
                    ) : versionType === "release" ? (
                      <span className="text-default-700 dark:text-default-200">
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
                      className="font-medium text-default-600"
                    >
                      {t("common.cancel")}
                    </Button>
                    <Button
                      color="primary"
                      radius="full"
                      className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
                      onPress={onSave}
                    >
                      {t("common.ok")}
                    </Button>
                  </div>
                }
              />
            </CardBody>
          </Card>
        </motion.div>
      </div>
      <div className="flex-1 overflow-auto">
        <div className="px-3 sm:px-5 lg:px-8 pb-3 sm:pb-4 lg:pb-6">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <motion.div
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.25, delay: 0.1 }}
            >
              <Card className="h-full min-h-[160px] border-none shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl">
                <CardBody className="p-6 sm:p-8 flex flex-col gap-6">
                  <div>
                    <label className="text-small font-medium text-default-700 dark:text-default-200 mb-2 block">
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
                          "bg-default-100/50 dark:bg-default-100/20 border-default-200 dark:border-default-700 hover:border-emerald-500 focus-within:border-emerald-500!",
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
                    <div className="text-small font-medium text-default-700 dark:text-default-200">
                      {t("versions.logo.title")}
                    </div>
                    <div className="flex items-center gap-4">
                      <div
                        className="relative h-24 w-24 rounded-2xl overflow-hidden bg-default-100 flex items-center justify-center border border-default-200 cursor-pointer group transition-all hover:scale-105 hover:shadow-lg"
                        onClick={() => {
                          navigate("/filemanager", {
                            state: {
                              allowedExt: [
                                ".png",
                                ".jpg",
                                ".jpeg",
                                ".gif",
                                ".webp",
                              ],
                              multi: false,
                              returnTo: "/version-settings",
                              title: t("versions.logo.title"),
                              returnState: {
                                name: targetName,
                                returnTo: returnToPath,
                              },
                            },
                          });
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
                          className="justify-start px-4 font-medium"
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
                          className="justify-start px-4 font-medium bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
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
                </CardBody>
              </Card>
            </motion.div>

            <motion.div
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.25, delay: 0.15 }}
            >
              <Card className="h-full min-h-[160px] border-none shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl">
                <CardBody className="p-6 sm:p-8 flex flex-col gap-5">
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
                        wrapper: "group-data-[selected=true]:bg-emerald-500",
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
                        wrapper: "group-data-[selected=true]:bg-emerald-500",
                      }}
                    />
                  </div>
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
                        wrapper: "group-data-[selected=true]:bg-emerald-500",
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
                        wrapper: "group-data-[selected=true]:bg-emerald-500",
                      }}
                    />
                  </div>
                  <div className="mt-4 pt-4 border-t border-default-200/50">
                    <div className="text-xs font-bold text-danger-500 mb-2 uppercase tracking-wider">
                      {t("versions.edit.danger_zone_title")}
                    </div>
                    <div className="flex items-center justify-between bg-danger-50 dark:bg-danger-500/10 p-4 rounded-2xl border border-danger-100 dark:border-danger-500/20">
                      <div className="text-tiny text-default-500 max-w-[60%]">
                        {isRegistered
                          ? t("versions.edit.unregister_hint")
                          : t("versions.edit.delete_hint")}
                      </div>
                      <div className="flex items-center gap-2">
                        {isRegistered ? (
                          <Button
                            size="sm"
                            variant="flat"
                            color="warning"
                            radius="full"
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
                            size="sm"
                            color="danger"
                            variant="flat"
                            radius="full"
                            className="font-medium bg-white/80 dark:bg-zinc-800/80 shadow-sm"
                            onPress={() => setDeleteOpen(true)}
                          >
                            {t("common.delete")}
                          </Button>
                        )}
                      </div>
                    </div>
                  </div>
                </CardBody>
              </Card>
            </motion.div>
          </div>
        </div>

        <div className="sm:hidden sticky bottom-0 inset-x-0 z-40 bg-white/80 dark:bg-zinc-900/50 backdrop-blur-xl border-t border-default-200/50 px-4 py-3 flex items-center justify-end gap-3">
          <Button
            variant="light"
            radius="full"
            onPress={() => navigate(returnToPath)}
            className="min-w-0 font-medium text-default-600"
          >
            {t("common.cancel")}
          </Button>
          <Button
            color="primary"
            radius="full"
            className="min-w-0 bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
            onPress={onSave}
          >
            {t("common.ok")}
          </Button>
        </div>
      </div>

      <BaseModal
        isOpen={unregisterOpen}
        onOpenChange={(open) => {
          if (!open) setUnregisterOpen(false);
        }}
        hideCloseButton
        isDismissable={false}
      >
        <ModalContent className="shadow-none">
          {
            (/* onClose */) => (
              <>
                <BaseModalHeader>
                  <motion.h2
                    className="text-2xl font-black tracking-tight text-warning-500"
                    initial={{ opacity: 0, y: -10 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ duration: 0.25 }}
                  >
                    {
                      t(
                        "versions.edit.unregister_progress.title",
                      ) as unknown as string
                    }
                  </motion.h2>
                </BaseModalHeader>
                <BaseModalBody>
                  <motion.div
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    transition={{ duration: 0.25, delay: 0.1 }}
                  >
                    <div className="text-medium font-medium text-default-600 mb-4">
                      {
                        t(
                          "versions.edit.unregister_progress.body",
                        ) as unknown as string
                      }
                    </div>
                    <Progress
                      size="sm"
                      isIndeterminate
                      aria-label="Unregistering"
                      classNames={{ indicator: "bg-warning-500" }}
                    />
                  </motion.div>
                </BaseModalBody>
              </>
            )
          }
        </ModalContent>
      </BaseModal>

      <BaseModal
        isOpen={unregisterSuccessOpen}
        onOpenChange={(open) => {
          if (!open) setUnregisterSuccessOpen(false);
        }}
        size="md"
      >
        <ModalContent className="shadow-none">
          {(onClose) => (
            <>
              <BaseModalHeader>
                <motion.h2
                  className="text-2xl font-black tracking-tight text-success-500"
                  initial={{ opacity: 0, y: -10 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.25 }}
                >
                  {
                    t(
                      "versions.edit.unregister_success.title",
                    ) as unknown as string
                  }
                </motion.h2>
              </BaseModalHeader>
              <BaseModalBody>
                <motion.div
                  className="text-medium font-medium text-default-600"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  transition={{ duration: 0.25, delay: 0.1 }}
                >
                  {
                    t(
                      "versions.edit.unregister_success.body",
                    ) as unknown as string
                  }
                </motion.div>
              </BaseModalBody>
              <BaseModalFooter>
                <Button
                  color="primary"
                  radius="full"
                  className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
                  onPress={() => {
                    onClose?.();
                    setUnregisterSuccessOpen(false);
                  }}
                >
                  {
                    t(
                      "launcherpage.delete.complete.close_button",
                    ) as unknown as string
                  }
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>

      <BaseModal
        isOpen={gdkMissingOpen}
        onOpenChange={(open) => {
          if (!open) setGdkMissingOpen(false);
        }}
      >
        <ModalContent className="shadow-none">
          {(onClose) => (
            <>
              <BaseModalHeader>
                <motion.h2
                  className="text-2xl font-black tracking-tight text-warning-500"
                  initial={{ opacity: 0, y: -10 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.25 }}
                >
                  {t("launcherpage.gdk_missing.title") as unknown as string}
                </motion.h2>
              </BaseModalHeader>
              <BaseModalBody>
                <motion.div
                  className="text-medium font-medium text-default-600"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  transition={{ duration: 0.25, delay: 0.1 }}
                >
                  {t("launcherpage.gdk_missing.body") as unknown as string}
                </motion.div>
              </BaseModalBody>
              <BaseModalFooter>
                <Button
                  variant="light"
                  radius="full"
                  onPress={() => {
                    onClose?.();
                    setGdkMissingOpen(false);
                  }}
                >
                  {t("common.cancel") as unknown as string}
                </Button>
                <Button
                  color="primary"
                  radius="full"
                  className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
                  onPress={() => {
                    onClose?.();
                    setGdkMissingOpen(false);
                    navigate("/settings");
                  }}
                >
                  {
                    t(
                      "launcherpage.gdk_missing.go_settings",
                    ) as unknown as string
                  }
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>
      <BaseModal
        isOpen={errorOpen}
        onOpenChange={(open) => {
          if (!open) setErrorOpen(false);
        }}
        hideCloseButton
        closeButton
        aria-label="error-modal"
        classNames={{
          closeButton:
            "absolute right-5 top-5 z-50 hover:bg-black/5 dark:hover:bg-white/5 active:bg-black/10 dark:active:bg-white/10 text-default-500",
        }}
      >
        <ModalContent className="shadow-none">
          {(onClose) => (
            <>
              <BaseModalHeader>
                <motion.h2
                  className="text-2xl font-black tracking-tight text-danger-500"
                  initial={{ opacity: 0, y: -10 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.25 }}
                >
                  {t("common.error")}
                </motion.h2>
              </BaseModalHeader>
              <BaseModalBody>
                <motion.div
                  className="p-4 rounded-2xl bg-danger-50 dark:bg-danger-500/10 border border-danger-100 dark:border-danger-500/20 text-danger-600 dark:text-danger-400 font-medium"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  transition={{ duration: 0.25, delay: 0.1 }}
                >
                  <div className="text-medium wrap-break-word">
                    {getErrorText(error)}
                  </div>
                </motion.div>
              </BaseModalBody>
              <BaseModalFooter>
                <Button
                  variant="light"
                  radius="full"
                  onPress={() => {
                    setError("");
                    onClose();
                  }}
                >
                  {t("common.ok")}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>

      <BaseModal
        isOpen={shortcutSuccessOpen}
        onOpenChange={(open) => {
          if (!open) setShortcutSuccessOpen(false);
        }}
        size="md"
        classNames={{
          closeButton:
            "absolute right-5 top-5 z-50 hover:bg-black/5 dark:hover:bg-white/5 active:bg-black/10 dark:active:bg-white/10 text-default-500",
        }}
      >
        <ModalContent className="shadow-none">
          {(onClose) => (
            <>
              <BaseModalHeader>
                <motion.h2
                  className="text-2xl font-black tracking-tight text-success-500"
                  initial={{ opacity: 0, y: -10 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.25 }}
                >
                  {
                    t(
                      "launcherpage.shortcut.success.title",
                    ) as unknown as string
                  }
                </motion.h2>
              </BaseModalHeader>
              <BaseModalBody>
                <motion.div
                  className="text-medium font-medium text-default-600"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  transition={{ duration: 0.25, delay: 0.1 }}
                >
                  {t("launcherpage.shortcut.success.body") as unknown as string}
                </motion.div>
              </BaseModalBody>
              <BaseModalFooter>
                <Button
                  color="primary"
                  radius="full"
                  className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
                  onPress={() => {
                    onClose?.();
                    setShortcutSuccessOpen(false);
                  }}
                >
                  {t("common.close") as unknown as string}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>

      <BaseModal
        isOpen={deleteOpen}
        onOpenChange={(open) => {
          if (!open) setDeleteOpen(false);
        }}
        hideCloseButton
        isDismissable={!deleting}
        classNames={{}}
      >
        <ModalContent className="shadow-none">
          {(onClose) => (
            <>
              <BaseModalHeader>
                <motion.h2
                  className="text-2xl font-black tracking-tight text-danger-500"
                  initial={{ opacity: 0, y: -10 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.25 }}
                >
                  {t("launcherpage.delete.confirm.title")}
                </motion.h2>
              </BaseModalHeader>
              <BaseModalBody>
                <motion.div
                  className="flex flex-col gap-4"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  transition={{ duration: 0.25, delay: 0.1 }}
                >
                  <div className="text-medium font-medium text-default-600 wrap-break-word whitespace-pre-wrap">
                    {t("launcherpage.delete.confirm.content")}
                  </div>
                  <div className="p-4 rounded-2xl bg-danger-50 dark:bg-danger-500/10 border border-danger-100 dark:border-danger-500/20 text-danger-600 dark:text-danger-400 text-sm wrap-break-word whitespace-pre-wrap font-mono">
                    {targetName}
                  </div>
                </motion.div>
              </BaseModalBody>
              <BaseModalFooter>
                <Button
                  variant="light"
                  radius="full"
                  onPress={() => {
                    onClose?.();
                  }}
                  isDisabled={deleting}
                >
                  {t("launcherpage.delete.confirm.cancel_button", {
                    defaultValue: "取消",
                  })}
                </Button>
                <Button
                  color="danger"
                  radius="full"
                  className="bg-danger shadow-lg shadow-danger-500/20 font-bold"
                  onPress={async () => {
                    await onDeleteConfirm();
                  }}
                  isLoading={deleting}
                >
                  {t("launcherpage.delete.confirm.delete_button")}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>

      <BaseModal
        isOpen={deleteSuccessOpen}
        onOpenChange={(open) => {
          if (!open) setDeleteSuccessOpen(open);
        }}
        size="md"
      >
        <ModalContent className="shadow-none">
          {(onClose) => (
            <>
              <BaseModalHeader>
                <motion.div
                  className="flex items-center gap-3"
                  initial={{ opacity: 0, y: -10 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.25 }}
                >
                  <div className="w-8 h-8 rounded-full bg-emerald-500 flex items-center justify-center shadow-lg shadow-emerald-900/20">
                    <svg
                      viewBox="0 0 24 24"
                      width="16"
                      height="16"
                      className="text-white"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="3"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    >
                      <path d="M20 6L9 17l-5-5" />
                    </svg>
                  </div>
                  <h2 className="text-2xl font-black tracking-tight text-emerald-500">
                    {t("launcherpage.delete.complete.title")}
                  </h2>
                </motion.div>
              </BaseModalHeader>
              <BaseModalBody>
                <motion.div
                  className="text-medium font-medium text-default-600"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  transition={{ duration: 0.25, delay: 0.1 }}
                >
                  {t("launcherpage.delete.complete.content")}
                  {deleteSuccessMsg ? (
                    <span className="font-mono text-default-700 font-bold">
                      {" "}
                      {deleteSuccessMsg}
                    </span>
                  ) : null}
                </motion.div>
              </BaseModalBody>
              <BaseModalFooter>
                <Button
                  color="primary"
                  radius="full"
                  className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
                  onPress={() => {
                    onClose?.();
                    setDeleteSuccessOpen(false);
                    navigate(returnToPath);
                  }}
                >
                  {t("launcherpage.delete.complete.close_button")}
                </Button>
              </BaseModalFooter>
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
                  {t("versions.unsaved.body", {
                    defaultValue:
                      "您更改了版本设置但尚未保存。是否保存后离开？",
                  })}
                </div>
              </BaseModalBody>
              <BaseModalFooter>
                <Button variant="light" onPress={onClose}>
                  {t("settings.unsaved.cancel", { defaultValue: "取消" })}
                </Button>
                <Button
                  color="primary"
                  onPress={async () => {
                    const ok = await onSave(pendingNavPath);
                    if (ok) {
                      onClose();
                    }
                  }}
                >
                  {t("settings.unsaved.save", { defaultValue: "保存并离开" })}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>
    </div>
  );
}
