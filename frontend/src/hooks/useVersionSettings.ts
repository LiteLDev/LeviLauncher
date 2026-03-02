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
import { useTranslation } from "react-i18next";

export const useVersionSettings = () => {
  const { t } = useTranslation();
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
  const [enableCtrlRReloadResources, setEnableCtrlRReloadResources] =
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
  const [originalCtrlRReloadResources, setOriginalCtrlRReloadResources] =
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
  const {
    isLLSupported,
    getSupportedLLVersions,
    getLatestLLVersion,
    compareLLVersions,
  } = useLeviLamina();
  const [installingLL, setInstallingLL] = React.useState(false);
  const [uninstallingLL, setUninstallingLL] = React.useState(false);
  const {
    isOpen: llVersionSelectOpen,
    onOpen: llVersionSelectOnOpen,
    onOpenChange: llVersionSelectOnOpenChange,
    onClose: llVersionSelectOnClose,
  } = useDisclosure();
  const {
    isOpen: llUninstallConfirmOpen,
    onOpen: llUninstallConfirmOnOpen,
    onClose: llUninstallConfirmOnClose,
  } = useDisclosure();
  const {
    isOpen: rcOpen,
    onOpen: rcOnOpen,
    onOpenChange: rcOnOpenChange,
    onClose: rcOnClose,
  } = useDisclosure();
  const [rcVersion, setRcVersion] = React.useState("");
  const [isLLInstalled, setIsLLInstalled] = React.useState(false);
  const [currentLLVersion, setCurrentLLVersion] = React.useState("");
  const [selectedLLVersion, setSelectedLLVersion] = React.useState("");
  const [llSupportedVersions, setLLSupportedVersions] = React.useState<
    string[]
  >([]);

  const latestLLVersion = React.useMemo(
    () => llSupportedVersions[0] || "",
    [llSupportedVersions],
  );

  const isSemverLike = React.useCallback((version: string) => {
    const v = String(version || "").trim();
    return /^[vV]?\d+\.\d+\.\d+(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/.test(
      v,
    );
  }, []);

  const canInstallSelectedLLVersion = React.useMemo(() => {
    const selected = String(selectedLLVersion || "").trim();
    if (!selected) return false;
    if (!isLLInstalled) return true;

    const current = String(currentLLVersion || "").trim();
    if (!current) {
      return selected === latestLLVersion;
    }
    if (!isSemverLike(current)) {
      return selected === latestLLVersion;
    }
    const cmp = compareLLVersions(selected, current);
    if (Number.isNaN(cmp)) {
      return selected === latestLLVersion;
    }
    return cmp > 0;
  }, [
    selectedLLVersion,
    isLLInstalled,
    currentLLVersion,
    latestLLVersion,
    isSemverLike,
    compareLLVersions,
  ]);

  const llInstallBlockedReasonKey = React.useMemo(() => {
    if (!isLLInstalled) return "";
    if (canInstallSelectedLLVersion) return "";

    const selected = String(selectedLLVersion || "").trim();
    const current = String(currentLLVersion || "").trim();
    if (!selected) {
      return "versions.edit.loader.ll_upgrade_only_hint";
    }
    if (!current || !isSemverLike(current)) {
      return "versions.edit.loader.ll_only_latest_when_current_unknown";
    }
    const cmp = compareLLVersions(selected, current);
    if (Number.isNaN(cmp)) {
      return "versions.edit.loader.ll_only_latest_when_current_unknown";
    }
    return "versions.edit.loader.ll_upgrade_only_hint";
  }, [
    isLLInstalled,
    canInstallSelectedLLVersion,
    selectedLLVersion,
    currentLLVersion,
    isSemverLike,
    compareLLVersions,
  ]);

  const llOnlyLatestSelectable = React.useMemo(() => {
    if (!isLLInstalled) return false;
    const current = String(currentLLVersion || "").trim();
    if (!current) return true;
    if (!isSemverLike(current)) return true;
    const cmp = compareLLVersions(latestLLVersion, current);
    return Number.isNaN(cmp);
  }, [
    isLLInstalled,
    currentLLVersion,
    isSemverLike,
    latestLLVersion,
    compareLLVersions,
  ]);

  // Load LL install status
  React.useEffect(() => {
    if (selectedTab === "loader" && targetName) {
      GetMods(targetName)
        .then((mods: any[]) => {
          if (mods) {
            const llMod = mods.find((m: any) => m?.name === "LeviLamina");
            setIsLLInstalled(Boolean(llMod));
            setCurrentLLVersion(String(llMod?.version || "").trim());
            return;
          }
          setIsLLInstalled(false);
          setCurrentLLVersion("");
        })
        .catch(() => {
          setIsLLInstalled(false);
          setCurrentLLVersion("");
        });
    }
  }, [selectedTab, targetName, installingLL, uninstallingLL]);

  React.useEffect(() => {
    setLLSupportedVersions(getSupportedLLVersions(gameVersion));
  }, [gameVersion, getSupportedLLVersions]);

  React.useEffect(() => {
    setSelectedLLVersion((prev) => {
      if (llSupportedVersions.length === 0) {
        return "";
      }
      if (prev && llSupportedVersions.includes(prev)) {
        return prev;
      }
      if (
        isLLInstalled &&
        currentLLVersion &&
        isSemverLike(currentLLVersion) &&
        llSupportedVersions.includes(currentLLVersion)
      ) {
        return currentLLVersion;
      }
      return latestLLVersion;
    });
  }, [
    llSupportedVersions,
    isLLInstalled,
    currentLLVersion,
    latestLLVersion,
    isSemverLike,
  ]);

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
            setEnableCtrlRReloadResources(!!meta?.enableCtrlRReloadResources);
            setOriginalCtrlRReloadResources(
              !!meta?.enableCtrlRReloadResources,
            );
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
          enableCtrlRReloadResources !== originalCtrlRReloadResources ||
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
    enableCtrlRReloadResources,
    originalCtrlRReloadResources,
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

  const resolveToastText = React.useCallback(
    (value: string) => {
      const raw = String(value || "").trim();
      if (!raw) return "";

      const direct = t(raw);
      if (direct && direct !== raw) {
        return String(direct);
      }

      if (/^ERR_[A-Z0-9_]+$/.test(raw)) {
        const mapped = getErrorKey(raw);
        const translated = t(mapped);
        if (translated && translated !== mapped) {
          return String(translated);
        }
      }

      const errorsKey = `errors.${raw}`;
      const errorsTranslated = t(errorsKey);
      if (errorsTranslated && errorsTranslated !== errorsKey) {
        return String(errorsTranslated);
      }

      return raw;
    },
    [t],
  );

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
          !!enableCtrlRReloadResources,
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
      enableCtrlRReloadResources,
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
  const proceedInstallLeviLamina = React.useCallback(async (forcedVersion?: string) => {
    if (!hasBackend || !targetName || !gameVersion) return;
    const installVersion = String(
      forcedVersion || selectedLLVersion || latestLLVersion || "",
    ).trim();
    if (!installVersion) {
      addToast({
        description: resolveToastText("ERR_LL_VERSION_UNSUPPORTED"),
        color: "danger",
      });
      return;
    }
    setInstallingLL(true);
    try {
      const isLip = await (minecraft as any)?.IsLipInstalled();
      if (!isLip) {
        addToast({
          description: resolveToastText("mods.err_lip_not_installed"),
          color: "danger",
        });
        setInstallingLL(false);
        return;
      }
      const installLL = (minecraft as any)?.InstallLeviLamina;
      if (typeof installLL === "function") {
        const err: string = await installLL(
          gameVersion,
          targetName,
          installVersion,
        );
        if (err) {
          let msg = err;
          if (err.includes("ERR_LIP_INSTALL_FAILED")) {
            msg = "mods.err_lip_install_failed_suggestion";
          }
          addToast({ description: resolveToastText(msg), color: "danger" });
        } else {
          setIsLLInstalled(true);
          setCurrentLLVersion(installVersion);
          addToast({
            title: t("downloadpage.install.success") as string,
            color: "success",
          });
        }
      }
    } catch (e: any) {
      addToast({
        description: resolveToastText(String(e?.message || e)),
        color: "danger",
      });
    } finally {
      setInstallingLL(false);
    }
  }, [
    hasBackend,
    targetName,
    gameVersion,
    selectedLLVersion,
    latestLLVersion,
    resolveToastText,
    t,
  ]);

  const handleInstallLeviLamina = React.useCallback(async () => {
    if (!hasBackend || !targetName || !gameVersion) return;

    const targetLLVersion = String(
      selectedLLVersion || getLatestLLVersion(gameVersion),
    ).trim();
    if (!targetLLVersion) {
      addToast({
        description: resolveToastText("ERR_LL_VERSION_UNSUPPORTED"),
        color: "danger",
      });
      return;
    }
    if (isLLInstalled && !canInstallSelectedLLVersion) {
      return;
    }

    if (targetLLVersion && targetLLVersion.includes("rc")) {
      setRcVersion(targetLLVersion);
      rcOnOpen();
      return;
    }

    await proceedInstallLeviLamina(targetLLVersion);
  }, [
    hasBackend,
    targetName,
    gameVersion,
    selectedLLVersion,
    getLatestLLVersion,
    isLLInstalled,
    canInstallSelectedLLVersion,
    rcOnOpen,
    proceedInstallLeviLamina,
    resolveToastText,
  ]);

  const openLeviLaminaVersionSelect = React.useCallback(() => {
    llVersionSelectOnOpen();
  }, [llVersionSelectOnOpen]);

  const confirmLeviLaminaVersionSelect = React.useCallback(async () => {
    llVersionSelectOnClose();
    await handleInstallLeviLamina();
  }, [llVersionSelectOnClose, handleInstallLeviLamina]);

  const openLLUninstallConfirm = React.useCallback(() => {
    llUninstallConfirmOnOpen();
  }, [llUninstallConfirmOnOpen]);

  const closeLLUninstallConfirm = React.useCallback(() => {
    if (uninstallingLL) return;
    llUninstallConfirmOnClose();
  }, [llUninstallConfirmOnClose, uninstallingLL]);

  const confirmUninstallLL = React.useCallback(async (): Promise<boolean> => {
    if (!targetName) return false;
    setUninstallingLL(true);
    try {
      const err = await (minecraft as any)?.UninstallLeviLamina?.(targetName);
      if (err) {
        addToast({
          description: resolveToastText(String(err)),
          color: "danger",
        });
        return false;
      } else {
        setIsLLInstalled(false);
        setCurrentLLVersion("");
        addToast({ title: t("common.success") as string, color: "success" });
        return true;
      }
    } catch (e) {
      addToast({ description: resolveToastText(String(e)), color: "danger" });
      return false;
    } finally {
      setUninstallingLL(false);
    }
  }, [targetName, resolveToastText, t]);

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
    enableCtrlRReloadResources,
    setEnableCtrlRReloadResources,
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
    llSupportedVersions,
    latestLLVersion,
    selectedLLVersion,
    setSelectedLLVersion,
    currentLLVersion,
    canInstallSelectedLLVersion,
    llInstallBlockedReasonKey,
    llOnlyLatestSelectable,
    installingLL,
    uninstallingLL,
    llUninstallConfirmOpen,
    llVersionSelectOpen,
    llVersionSelectOnOpenChange,
    llVersionSelectOnClose,
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
    openLeviLaminaVersionSelect,
    confirmLeviLaminaVersionSelect,
    openLLUninstallConfirm,
    closeLLUninstallConfirm,
    confirmUninstallLL,
    handleInstallLeviLamina,
  };
};
