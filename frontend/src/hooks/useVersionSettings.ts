import React from "react";
import { useNavigate, useLocation } from "react-router-dom";
import { useDisclosure, addToast } from "@heroui/react";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { GetMods } from "bindings/github.com/liteldev/LeviLauncher/modsservice";
import {
  DeleteVersionFolder,
  GetVersionLogoDataUrl,
  GetVersionMeta,
  RenameVersionFolder,
  SaveVersionLogoDataUrl,
  SaveVersionMeta,
  ValidateVersionFolderName,
} from "bindings/github.com/liteldev/LeviLauncher/versionservice";
import { useLeviLamina } from "@/utils/LeviLaminaContext";

export const useVersionSettings = () => {
  const navigate = useNavigate();
  const location = useLocation() as any;
  const initialName: string = String(location?.state?.name || "");
  const returnToPath: string = String(
    location?.state?.returnTo || "/versions",
  );
  const hasBackend = minecraft !== undefined;

  // Form state
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
  const [error, setError] = React.useState<string>("");
  const [logoDataUrl, setLogoDataUrl] = React.useState<string>("");

  // Original values for unsaved changes detection
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

  // Modal states
  const [unregisterOpen, setUnregisterOpen] = React.useState<boolean>(false);
  const [unregisterSuccessOpen, setUnregisterSuccessOpen] =
    React.useState<boolean>(false);
  const [gdkMissingOpen, setGdkMissingOpen] = React.useState<boolean>(false);
  const [errorOpen, setErrorOpen] = React.useState<boolean>(false);
  const [deleteOpen, setDeleteOpen] = React.useState<boolean>(false);
  const [deleteSuccessOpen, setDeleteSuccessOpen] =
    React.useState<boolean>(false);
  const [shortcutSuccessOpen, setShortcutSuccessOpen] =
    React.useState<boolean>(false);
  const [deleting, setDeleting] = React.useState<boolean>(false);
  const [deleteSuccessMsg, setDeleteSuccessMsg] = React.useState<string>("");

  // Unsaved changes modal
  const {
    isOpen: unsavedOpen,
    onOpen: unsavedOnOpen,
    onClose: unsavedOnClose,
    onOpenChange: unsavedOnOpenChange,
  } = useDisclosure();
  const [pendingNavPath, setPendingNavPath] = React.useState<string>("");

  // LeviLamina
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

  // Load LL install status
  React.useEffect(() => {
    if (selectedTab === "loader" && targetName) {
      GetMods(targetName)
        .then((mods: any[]) => {
          if (mods) {
            const installed = mods.some((m: any) => m.name === "LeviLamina");
            setIsLLInstalled(installed);
          }
        })
        .catch(() => {});
    }
  }, [selectedTab, targetName, installingLL]);

  // Load version metadata
  React.useEffect(() => {
    if (!hasBackend || !targetName) return;
    (async () => {
      try {
        const getMeta = GetVersionMeta as any;
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
        const getter = GetVersionLogoDataUrl as any;
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

  // Unsaved changes navigation guard
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

  // Error text helper
  const errorDefaults: Record<string, string> = {
    ERR_INVALID_NAME: "errors.ERR_INVALID_NAME",
    ERR_ICON_DECODE: "errors.ERR_ICON_DECODE",
    ERR_ICON_NOT_SQUARE: "errors.ERR_ICON_NOT_SQUARE",
  };
  const getErrorKey = (code: string) => {
    if (!code) return "";
    return errorDefaults[code] || `errors.${code}`;
  };

  // Save handler
  const onSave = React.useCallback(
    async (destPath?: string) => {
      if (!hasBackend || !targetName) {
        navigate(-1);
        return false;
      }
      const validate = ValidateVersionFolderName as any;
      const rename = RenameVersionFolder as any;
      const save = SaveVersionMeta as any;
      const saver = SaveVersionLogoDataUrl as any;

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

  // Delete handler
  const onDeleteConfirm = React.useCallback(async () => {
    if (!hasBackend || !targetName) {
      setDeleteOpen(false);
      return;
    }
    setDeleting(true);
    try {
      const del = DeleteVersionFolder as any;
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

  // LeviLamina install handlers
  const proceedInstallLeviLamina = React.useCallback(async () => {
    if (!hasBackend || !targetName || !gameVersion) return;
    setInstallingLL(true);
    try {
      const isLip = await (minecraft as any)?.IsLipInstalled();
      if (!isLip) {
        addToast({
          description: "mods.err_lip_not_installed",
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
            msg = "mods.err_lip_install_failed_suggestion";
          }
          addToast({ description: msg, color: "danger" });
        } else {
          addToast({
            title: "downloadpage.install.success",
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
  }, [hasBackend, targetName, gameVersion]);

  const handleInstallLeviLamina = React.useCallback(async () => {
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
  }, [hasBackend, targetName, gameVersion, llMap, rcOnOpen, proceedInstallLeviLamina]);

  const handleUninstallLL = React.useCallback(async () => {
    if (!targetName) return;
    setInstallingLL(true);
    try {
      const err = await (minecraft as any)?.UninstallLeviLamina?.(targetName);
      if (err) {
        addToast({ description: String(err), color: "danger" });
      } else {
        setIsLLInstalled(false);
        addToast({ title: "common.success", color: "success" });
      }
    } catch (e) {
      addToast({ description: String(e), color: "danger" });
    } finally {
      setInstallingLL(false);
    }
  }, [targetName]);

  return {
    // Navigation
    navigate,
    returnToPath,

    // Form state
    targetName,
    selectedTab,
    setSelectedTab,
    newName,
    setNewName,
    gameVersion,
    versionType,
    isPreview,
    enableIsolation,
    setEnableIsolation,
    enableConsole,
    setEnableConsole,
    enableEditorMode,
    setEnableEditorMode,
    enableRenderDragon,
    setEnableRenderDragon,
    envVars,
    setEnvVars,
    launchArgs,
    setLaunchArgs,
    isRegistered,
    setIsRegistered,
    loading,
    error,
    setError,
    logoDataUrl,
    setLogoDataUrl,
    hasBackend,

    // Modal states
    unregisterOpen,
    setUnregisterOpen,
    unregisterSuccessOpen,
    setUnregisterSuccessOpen,
    gdkMissingOpen,
    setGdkMissingOpen,
    errorOpen,
    setErrorOpen,
    deleteOpen,
    setDeleteOpen,
    deleteSuccessOpen,
    setDeleteSuccessOpen,
    shortcutSuccessOpen,
    setShortcutSuccessOpen,
    deleting,
    deleteSuccessMsg,

    // Unsaved changes
    unsavedOpen,
    unsavedOnClose,
    unsavedOnOpenChange,
    pendingNavPath,

    // LeviLamina
    isLLSupported,
    installingLL,
    rcOpen,
    rcOnOpenChange,
    rcOnClose,
    rcVersion,
    isLLInstalled,

    // Error helper
    getErrorKey,

    // Actions
    onSave,
    onDeleteConfirm,
    proceedInstallLeviLamina,
    handleInstallLeviLamina,
    handleUninstallLL,
  };
};
