import React from "react";
import { useLocation, useNavigate } from "react-router-dom";
import {
  Button,
  Card,
  CardBody,
  Input,
  Switch,
  Chip,
  Modal,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
} from "@heroui/react";
import { useTranslation } from "react-i18next";
import { motion } from "framer-motion";
import { Call as RuntimeCall } from "@wailsio/runtime";
import { FaWindows } from "react-icons/fa";
import * as minecraft from "../../bindings/github.com/liteldev/LeviLauncher/minecraft";

export default function VersionSettingsPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation() as any;
  const initialName: string = String(location?.state?.name || "");
  const returnToPath: string = String(location?.state?.returnTo || "/versions");

  const [targetName, setTargetName] = React.useState<string>(initialName);
  const [newName, setNewName] = React.useState<string>(initialName);
  const [gameVersion, setGameVersion] = React.useState<string>("");
  const [isPreview, setIsPreview] = React.useState<boolean>(false);
  const [enableIsolation, setEnableIsolation] = React.useState<boolean>(false);
  const [enableConsole, setEnableConsole] = React.useState<boolean>(false);
  const [enableEditorMode, setEnableEditorMode] =
    React.useState<boolean>(false);
  const [error, setError] = React.useState<string>("");
  const [logoDataUrl, setLogoDataUrl] = React.useState<string>("");
  const [errorOpen, setErrorOpen] = React.useState<boolean>(false);
  const [deleteOpen, setDeleteOpen] = React.useState<boolean>(false);
  const [deleteSuccessOpen, setDeleteSuccessOpen] =
    React.useState<boolean>(false);
  const [shortcutSuccessOpen, setShortcutSuccessOpen] = React.useState<boolean>(false);
  const [deleting, setDeleting] = React.useState<boolean>(false);
  const [deleteSuccessMsg, setDeleteSuccessMsg] = React.useState<string>("");

  const hasBackend = minecraft !== undefined;

  React.useEffect(() => {
    if (!hasBackend || !targetName) return;
    try {
      const list = minecraft?.ListVersionMetas;
      if (typeof list === "function") {
        list().then((metas: any[]) => {
          const meta = (metas || []).find(
            (m: any) => String(m?.name || "") === targetName
          );
          if (meta) {
            setGameVersion(String(meta?.gameVersion || ""));
            const type = String(meta?.type || "release").toLowerCase();
            setIsPreview(type === "preview");
            setEnableIsolation(!!meta?.enableIsolation);
            setEnableConsole(!!meta?.enableConsole);
            setEnableEditorMode(!!meta?.enableEditorMode);
          }
        });
      }
    } catch {}
    try {
      const getter = minecraft?.GetVersionLogoDataUrl;
      if (typeof getter === "function") {
        getter(targetName).then((u: string) => setLogoDataUrl(String(u || "")));
      }
    } catch {
      setLogoDataUrl("");
    }
  }, [hasBackend, targetName]);

  React.useEffect(() => {
    setErrorOpen(!!error);
  }, [error]);

  const errorDefaults: Record<string, string> = {
    ERR_INVALID_NAME: t("errors.ERR_INVALID_NAME", {
      defaultValue: "名称无效",
    }) as string,
    ERR_ICON_DECODE: t("errors.ERR_ICON_DECODE", {
      defaultValue: "图标解析失败",
    }) as string,
    ERR_ICON_NOT_SQUARE: t("errors.ERR_ICON_NOT_SQUARE", {
      defaultValue: "图标必须是正方形",
    }) as string,
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
                setLogoDataUrl(String(u || ""))
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

  const onSave = React.useCallback(async () => {
    if (!hasBackend || !targetName) {
      navigate(-1);
      return;
    }
    const validate = minecraft?.ValidateVersionFolderName;
    const rename = minecraft?.RenameVersionFolder;
    const save = minecraft?.SaveVersionMeta;
    const saver = minecraft?.SaveVersionLogoDataUrl;

    const nn = (newName || "").trim();
    if (!nn) {
      setError("ERR_INVALID_NAME");
      return;
    }
    const type = isPreview ? "preview" : "release";

    if (nn !== targetName) {
      if (typeof validate === "function") {
        const msg: string = await validate(nn);
        if (msg) {
          setError(msg);
          return;
        }
      }
      if (typeof rename === "function") {
        const err: string = await rename(targetName, nn);
        if (err) {
          setError(err);
          return;
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
        !!enableEditorMode
      );
      if (err2) {
        setError(err2);
        return;
      }
    }
    try {
      if (typeof saver === "function" && logoDataUrl) {
        const e = await saver(nn, logoDataUrl);
        if (e) {
          setError(e);
          return;
        }
      }
    } catch {}

    navigate(returnToPath);
  }, [
    hasBackend,
    targetName,
    newName,
    gameVersion,
    isPreview,
    enableIsolation,
    enableConsole,
    enableEditorMode,
    logoDataUrl,
    returnToPath,
    navigate,
  ]);

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
    <div className="w-full h-full flex flex-col overflow-auto">
      <div className="px-3 sm:px-5 lg:px-8 py-3 sm:py-4 lg:py-6">
        <div className="sticky top-2 z-10">
          <motion.div
            initial={{ y: -8, opacity: 0 }}
            animate={{ y: 0, opacity: 1 }}
            transition={{ duration: 0.18, ease: [0.16, 0.84, 0.44, 1] }}
          >
            <Card className="rounded-3xl shadow-xl mb-4 bg-white/60 dark:bg-black/30 backdrop-blur-md border border-white/30">
              <CardBody className="px-4 sm:px-5 py-3 w-full">
                <div className="w-full">
                  <div className="flex items-center justify-between gap-2 w-full">
                    <h1 className="text-2xl font-bold text-left !text-left">
                      {t("versions.edit.title", { defaultValue: "版本设置" })}
                    </h1>
                    <div className="hidden sm:flex items-center gap-2">
                      <Button
                        variant="light"
                        onPress={() => navigate(returnToPath)}
                      >
                        {t("common.cancel", { defaultValue: "取消" })}
                      </Button>
                      <Button color="primary" onPress={onSave}>
                        {t("common.ok", { defaultValue: "确定" })}
                      </Button>
                    </div>
                  </div>
                  <div className="mt-1 text-sm text-default-500 truncate text-left !text-left">
                    {t("versions.info.version", {
                      defaultValue: "Game Version",
                    })}
                    :{" "}
                    <span className="text-default-700 font-medium">
                      {gameVersion ||
                        t("launcherpage.version_select.unknown", {
                          defaultValue: "Unknown",
                        })}
                    </span>
                    <span className="mx-2 text-default-400">·</span>
                    {t("versions.info.name", { defaultValue: "名称" })}:{" "}
                    <span className="text-default-700 font-medium">
                      {targetName || "-"}
                    </span>
                    <span className="mx-2 text-default-400">·</span>
                    {isPreview ? (
                      <Chip size="sm" variant="flat" color="warning">
                        {t("common.preview", { defaultValue: "预览版" })}
                      </Chip>
                    ) : (
                      <span className="text-default-600">
                        {t("common.release", { defaultValue: "正式版" })}
                      </span>
                    )}
                  </div>
                </div>
              </CardBody>
            </Card>
          </motion.div>
        </div>
        <div className="mt-4 grid grid-cols-1 md:grid-cols-2 gap-4">
          <motion.div
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.25, delay: 0.1 }}
          >
            <Card className="rounded-2xl shadow-md h-full min-h-[160px] bg-white/70 dark:bg-black/30 backdrop-blur-md border border-white/30">
              <CardBody className="p-4 sm:p-6 flex flex-col gap-4">
                <div>
                  <label className="text-small text-default-600 mb-1 block">
                    {t("versions.edit.new_name", { defaultValue: "新名称" })}
                  </label>
                  <Input
                    value={newName}
                    onValueChange={(v) => {
                      setNewName(v);
                      if (error) setError("");
                    }}
                    size="sm"
                    variant="bordered"
                    placeholder={
                      t("versions.edit.placeholder", {
                        defaultValue: "输入新名称",
                      }) as unknown as string
                    }
                  />
                  <p className="text-tiny text-default-400 mt-1">
                    {t("versions.edit.hint", {
                      defaultValue: "仅修改文件夹名称，不影响游戏版本与类型。",
                    })}
                  </p>
                </div>
                <div className="flex flex-col gap-2">
                  <div className="text-small text-default-600">
                    {t("versions.logo.title", {
                      defaultValue: "自定义图标（要求正方形）",
                    })}
                  </div>
                  <div className="flex items-center gap-3">
                    <div
                      className="relative h-20 w-20 rounded-xl overflow-hidden bg-default-100 flex items-center justify-center border border-default-200 cursor-pointer group"
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
                            title: t("versions.logo.title", {
                              defaultValue: "自定义图标（要求正方形）",
                            }),
                            returnState: {
                              name: targetName,
                              returnTo: returnToPath,
                            },
                          },
                        });
                      }}
                      title={
                        t("versions.logo.change", {
                          defaultValue: "点击更换",
                        }) as string
                      }
                    >
                      {logoDataUrl ? (
                        <img
                          src={logoDataUrl}
                          alt="logo"
                          className="h-full w-full object-cover"
                        />
                      ) : null}
                      <div className="absolute inset-0 bg-black/35 opacity-0 group-hover:opacity-100 transition flex items-center justify-center text-white text-tiny">
                        {t("versions.logo.change", {
                          defaultValue: "点击更换",
                        })}
                      </div>
                    </div>
                    <Button
                      size="sm"
                      variant="flat"
                      color="danger"
                      className="rounded-full shadow-none"
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
                      {t("versions.logo.clear", { defaultValue: "清除" })}
                    </Button>
                    <Button
                      size="sm"
                      variant="flat"
                      className="rounded-full justify-start shadow-none"
                      onPress={async () => {
                        try {
                          const err: string = await RuntimeCall.ByName("main.Minecraft.CreateDesktopShortcut", targetName);
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
                      {t("launcherpage.shortcut.create_button", { defaultValue: "创建桌面快捷方式" }) as unknown as string}
                    </Button>
                  </div>
                  <p className="text-tiny text-default-400">
                    {t("versions.logo.hint", {
                      defaultValue:
                        "将保存到版本文件夹的 LeviLauncher/Logo.png。点击图标更换。",
                    })}
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
            <Card className="rounded-2xl shadow-md h-full min-h-[160px] bg-white/70 dark:bg-black/30 backdrop-blur-md border border-white/30">
              <CardBody className="p-4 sm:p-6 flex flex-col gap-4">
                <div className="flex items-center justify-between">
                  <div className="text-small">
                    {t("versions.edit.enable_isolation", {
                      defaultValue: "启用隔离环境",
                    })}
                  </div>
                  <Switch
                    size="sm"
                    isSelected={enableIsolation}
                    onValueChange={setEnableIsolation}
                  />
                </div>
                <div className="flex items-center justify-between">
                  <div className="text-small">
                    {t("versions.edit.enable_console", {
                      defaultValue: "启用控制台",
                    })}
                  </div>
                  <Switch
                    size="sm"
                    isSelected={enableConsole}
                    onValueChange={setEnableConsole}
                  />
                </div>
                <div className="flex items-center justify-between">
                  <div className="text-small">
                    {t("versions.edit.enable_editor_mode", {
                      defaultValue: "启用编辑器模式",
                    })}
                  </div>
                  <Switch
                    size="sm"
                    isSelected={enableEditorMode}
                    onValueChange={setEnableEditorMode}
                  />
                </div>
                <div className="mt-3 pt-3 border-t border-white/30">
                  <div className="text-tiny text-default-500 mb-1">
                    {t("versions.edit.danger_zone_title", {
                      defaultValue: "危险操作",
                    })}
                  </div>
                  <div className="flex items-center justify-between">
                    <div className="text-tiny text-default-400">
                      {t("versions.edit.delete_hint", {
                        defaultValue: "点击以打开删除确认，避免误触。",
                      })}
                    </div>
                    <Button
                      size="sm"
                      variant="light"
                      color="danger"
                      onPress={() => setDeleteOpen(true)}
                    >
                      {t("launcherpage.delete.confirm.delete_button", {
                        defaultValue: "删除",
                      })}
                    </Button>
                  </div>
                </div>
              </CardBody>
            </Card>
          </motion.div>
        </div>
      </div>

      <div className="sm:hidden sticky bottom-0 inset-x-0 z-40 bg-white/60 dark:bg-black/30 backdrop-blur-md border-t border-white/30 px-3 py-2 flex items-center justify-end gap-2">
        <Button
          variant="light"
          onPress={() => navigate(returnToPath)}
          className="min-w-0"
        >
          {t("common.cancel", { defaultValue: "取消" })}
        </Button>
        <Button color="primary" onPress={onSave} className="min-w-0">
          {t("common.ok", { defaultValue: "确定" })}
        </Button>
      </div>

      <Modal
        isOpen={errorOpen}
        onOpenChange={(open) => {
          if (!open) setErrorOpen(false);
        }}
        hideCloseButton
        closeButton
        aria-label="error-modal"
      >
        <ModalContent>
          {(onClose) => (
            <>
              <ModalHeader className="text-danger">
                {t("common.error", { defaultValue: "错误" })}
              </ModalHeader>
              <ModalBody>
                <div className="text-sm break-words">{getErrorText(error)}</div>
              </ModalBody>
              <ModalFooter>
                <Button
                  variant="light"
                  onPress={() => {
                    setError("");
                    onClose();
                  }}
                >
                  {t("common.ok", { defaultValue: "确定" })}
                </Button>
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>

      <Modal
        isOpen={shortcutSuccessOpen}
        onOpenChange={(open) => {
          if (!open) setShortcutSuccessOpen(false);
        }}
        size="md"
      >
        <ModalContent>
          {(onClose) => (
            <>
              <ModalHeader className="flex items-center gap-2 text-success-600">
                <h2 className="text-lg font-semibold">
                  {t("launcherpage.shortcut.success.title", { defaultValue: "快捷方式已创建" }) as unknown as string}
                </h2>
              </ModalHeader>
              <ModalBody>
                <div className="text-small text-default-600">
                  {t("launcherpage.shortcut.success.body", { defaultValue: "已在桌面创建该版本的快捷方式。" }) as unknown as string}
                </div>
              </ModalBody>
              <ModalFooter>
                <Button
                  color="primary"
                  onPress={() => {
                    onClose?.();
                    setShortcutSuccessOpen(false);
                  }}
                >
                  {t("launcherpage.delete.complete.close_button", { defaultValue: "关闭" }) as unknown as string}
                </Button>
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>

      <Modal
        isOpen={deleteOpen}
        onOpenChange={(open) => {
          if (!open) setDeleteOpen(false);
        }}
        hideCloseButton
        isDismissable={!deleting}
      >
        <ModalContent>
          {(onClose) => (
            <>
              <ModalHeader className="text-danger">
                {t("launcherpage.delete.confirm.title", {
                  defaultValue: "确认删除",
                })}
              </ModalHeader>
              <ModalBody>
                <div className="text-sm text-default-700 break-words whitespace-pre-wrap">
                  {t("launcherpage.delete.confirm.content", {
                    defaultValue: "您确定要删除此版本吗？此操作无法撤销。",
                  })}
                </div>
                <div className="mt-1 rounded-md bg-default-100/60 border border-default-200 px-3 py-2 text-default-800 text-sm break-words whitespace-pre-wrap">
                  {targetName}
                </div>
              </ModalBody>
              <ModalFooter>
                <Button
                  variant="light"
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
                  onPress={async () => {
                    await onDeleteConfirm();
                  }}
                  isLoading={deleting}
                >
                  {t("launcherpage.delete.confirm.delete_button", {
                    defaultValue: "删除",
                  })}
                </Button>
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>

      <Modal
        isOpen={deleteSuccessOpen}
        onOpenChange={(open) => {
          if (!open) setDeleteSuccessOpen(open);
        }}
        size="md"
      >
        <ModalContent>
          {(onClose) => (
            <>
              <ModalHeader className="flex items-center gap-2">
                <div className="w-6 h-6 rounded-full bg-green-500 flex items-center justify-center">
                  <svg
                    viewBox="0 0 24 24"
                    width="14"
                    height="14"
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
                <h2 className="text-lg font-semibold">
                  {t("launcherpage.delete.complete.title", {
                    defaultValue: "删除完成",
                  })}
                </h2>
              </ModalHeader>
              <ModalBody>
                <div className="text-small text-default-600">
                  {t("launcherpage.delete.complete.content", {
                    defaultValue: "所选版本已成功删除！",
                  })}
                  {deleteSuccessMsg ? (
                    <span className="font-mono text-default-700">
                      {" "}
                      {deleteSuccessMsg}
                    </span>
                  ) : null}
                </div>
              </ModalBody>
              <ModalFooter>
                <Button
                  color="primary"
                  onPress={() => {
                    onClose?.();
                    setDeleteSuccessOpen(false);
                    navigate(returnToPath);
                  }}
                >
                  {t("launcherpage.delete.complete.close_button", {
                    defaultValue: "关闭",
                  })}
                </Button>
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>
    </div>
  );
}
