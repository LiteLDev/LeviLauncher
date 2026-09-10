import { ModalDescription } from "@/components/ModalPrimitives";
import {
  Button,
  Card,
  Chip,
  Dropdown,
  FieldError,
  Input,
  Label,
  ListBox,
  Select,
  Spinner,
  Switch,
  TextField,
  toast,
  useOverlayState,
} from "@heroui/react";

import { useEffect, useMemo, useState } from "react";

import { useLocation, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { FaChevronDown } from "react-icons/fa";
import { ROUTES } from "@/constants/routes";
import { useVersionStatus } from "@/utils/VersionStatusContext";
import { useLeviLamina } from "@/utils/LeviLaminaContext";
import { useLipTaskConsole } from "@/utils/LipTaskConsoleContext";
import { resolveInstallError } from "@/utils/installError";
import { saveCurrentVersionName } from "@/utils/currentVersion";
import { setNavLockReason } from "@/hooks/useAppNavigation";
import { motion, AnimatePresence } from "framer-motion";
import { Dialogs, Events } from "@wailsio/runtime";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import {
  CopyVersionDataFromGDK,
  CopyVersionDataFromVersion,
  DeleteVersionFolder,
  GetInstallerDir,
  GetVersionsDir,
  ListInheritableVersionNames,
  ListVersionMetas,
  SaveVersionMeta,
  ValidateVersionFolderName,
} from "bindings/github.com/liteldev/LeviLauncher/versionservice";
import { UnifiedModal } from "@/components/UnifiedModal";
import { PageHeader } from "@/components/PageHeader";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { cn } from "@/utils/cn";

type ItemType = "Preview" | "Release";

const INSTALL_ISOLATION_PREFERENCE_KEY = "ll.install.enableIsolation";

function readInstallIsolationPreference(): boolean {
  try {
    const value = localStorage.getItem(INSTALL_ISOLATION_PREFERENCE_KEY);
    if (value === "true") return true;
    if (value === "false") return false;
  } catch {}
  return true;
}

export default function InstallPage() {
  const { t } = useTranslation();
  const { runWithLipTask } = useLipTaskConsole();
  const navigate = useNavigate();
  const location = useLocation() as any;
  const { refreshAll } = useVersionStatus();

  const mirrorVersion: string = String(location?.state?.mirrorVersion || "");
  const mirrorType: ItemType = String(
    location?.state?.mirrorType || "Release",
  ) as ItemType;
  const typeLabel: string = (mirrorType === "Preview"
    ? (t("common.preview") as unknown as string)
    : (t("common.release") as unknown as string)) as unknown as string;
  const returnTo: string = String(location?.state?.returnTo || ROUTES.download);
  const isLeviLaminaSupported = Boolean(location?.state?.isLeviLaminaSupported);

  const [installName, setInstallName] = useState<string>(mirrorVersion || "");
  const [installIsolation, setInstallIsolation] = useState<boolean>(() =>
    readInstallIsolationPreference(),
  );
  const [installLeviLamina, setInstallLeviLamina] = useState<boolean>(false);
  const [inheritSource, setInheritSource] = useState<string>("");
  const [inheritMetas, setInheritMetas] = useState<any[]>([]);
  const [inheritCandidates, setInheritCandidates] = useState<string[]>([]);
  const [installError, setInstallError] = useState<string>("");
  const [failureDetails, setFailureDetails] = useState("");
  const [installing, setInstalling] = useState<boolean>(false);
  const [installingVersion, setInstallingVersion] = useState<string>("");
  const [installingTargetName, setInstallingTargetName] = useState<string>("");
  const [installedFolderName, setInstalledFolderName] = useState<string>("");
  const [resultMsg, setResultMsg] = useState<string>("");
  const [customInstallerPath, setCustomInstallerPath] = useState<string>("");
  const [installerDir, setInstallerDir] = useState<string>("");
  const [downloadResolved, setDownloadResolved] = useState<boolean>(false);
  const { getSupportedLLVersions, getLatestLLVersion } = useLeviLamina();
  const {
    isOpen: rcOpen,
    open: rcOnOpen,
    setOpen: rcOnOpenChange,
    close: rcOnClose,
  } = useOverlayState();
  const [rcVersion, setRcVersion] = useState("");
  const [llSupportedVersions, setLLSupportedVersions] = useState<string[]>([]);
  const [selectedLLVersion, setSelectedLLVersion] = useState<string>("");
  const [extractInfo, setExtractInfo] = useState<{
    files: number;
    bytes: number;
    dir: string;
    totalBytes?: number;
    currentFile?: string;
  } | null>(null);

  useEffect(() => {
    try {
      localStorage.setItem(
        INSTALL_ISOLATION_PREFERENCE_KEY,
        installIsolation ? "true" : "false",
      );
    } catch {}
  }, [installIsolation]);

  useEffect(() => {
    if (!installing) {
      setExtractInfo(null);
      return;
    }

    const off = Events.On("extract.progress", (event) => {
      const payload = event?.data || {};
      const files = Number(payload?.files || 0);

      const globalTotal =
        payload?.global_total !== undefined ? Number(payload.global_total) : 0;
      const globalCurrent =
        payload?.global_current !== undefined
          ? Number(payload.global_current)
          : 0;

      const hasGlobal = globalTotal > 0;

      const totalBytes = hasGlobal
        ? globalTotal
        : Number(payload?.totalBytes || 0);

      const bytes = hasGlobal ? globalCurrent : Number(payload?.bytes || 0);

      const dir = String(payload?.dir || "");
      const currentFile = String(
        (payload as any)?.file || (payload as any)?.currentFile || "",
      );
      setExtractInfo({ files, bytes, dir, totalBytes, currentFile });
    });
    return () => off();
  }, [installing]);

  useEffect(() => {
    const guardActive = installing && !resultMsg;
    setNavLockReason("install-flow", guardActive);
    const originalPush = window.history.pushState.bind(window.history);
    const originalReplace = window.history.replaceState.bind(window.history);

    if (guardActive) {
      (window.history as any).pushState = function (..._args: any[]) {
        return;
      } as any;
      (window.history as any).replaceState = function (..._args: any[]) {
        return;
      } as any;
    }

    return () => {
      (window.history as any).pushState = originalPush as any;
      (window.history as any).replaceState = originalReplace as any;
      if (guardActive) {
        setNavLockReason("install-flow", false);
      }
    };
  }, [installing, resultMsg]);

  useEffect(() => {
    const name = (installName || "").trim();
    if (!name) {
      setInstallError("ERR_NAME_REQUIRED");
      return;
    }
    let cancelled = false;
    const timer = setTimeout(async () => {
      try {
        const validate = ValidateVersionFolderName as any;
        if (typeof validate === "function") {
          const msg: string = await validate(name);
          if (!cancelled) setInstallError(msg || "");
        } else {
          if (!cancelled) setInstallError("");
        }
      } catch {
        if (!cancelled) setInstallError("");
      }
    }, 250);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [installName]);

  useEffect(() => {
    if (!installIsolation) {
      setInheritMetas([]);
      setInheritSource("");
      return;
    }
    try {
      const list = ListVersionMetas as any;
      if (typeof list === "function") {
        (async () => {
          try {
            const metas = await list();
            setInheritMetas(Array.isArray(metas) ? metas : []);
          } catch {
            setInheritMetas([]);
          }
        })();
      }
    } catch {
      setInheritMetas([]);
    }
  }, [installIsolation]);

  useEffect(() => {
    if (!installIsolation) {
      setInheritCandidates([]);
      return;
    }
    try {
      const listInh = ListInheritableVersionNames as any;
      if (typeof listInh === "function") {
        (async () => {
          try {
            const type = String(mirrorType || "Release").toLowerCase();
            const names: string[] = await listInh(type);
            setInheritCandidates(Array.isArray(names) ? names : []);
          } catch {
            setInheritCandidates([]);
          }
        })();
      }
    } catch {
      setInheritCandidates([]);
    }
  }, [installIsolation, mirrorType]);

  const inheritOptions = useMemo(() => {
    const type = String(mirrorType || "Release").toLowerCase();
    const allowed = new Set(
      (inheritCandidates || []).map((n) => String(n || "")),
    );
    return (inheritMetas || [])
      .filter(
        (m: any) =>
          Boolean(m?.enableIsolation) &&
          String(m?.type || "").toLowerCase() === type &&
          allowed.has(String(m?.name || "")),
      )
      .map((m: any) => ({
        key: String(m?.name || ""),
        label: `${m?.name || ""}${m?.gameVersion ? ` (${m.gameVersion})` : ""}`,
      }))
      .filter((x: any) => x.key);
  }, [inheritMetas, inheritCandidates, mirrorType]);

  const inheritLabel = useMemo(() => {
    const src = inheritSource || "none";
    if (src === "none")
      return t("downloadpage.install_folder.inherit_none") as unknown as string;
    if (src === "gdk")
      return t("downloadpage.install_folder.inherit_gdk") as unknown as string;
    return inheritOptions.find((o) => o.key === src)?.label || src;
  }, [inheritSource, inheritOptions]);

  const inheritMenuItems = useMemo(
    () => [
      {
        key: "none",
        label: t(
          "downloadpage.install_folder.inherit_none",
        ) as unknown as string,
      },
      {
        key: "gdk",
        label: t(
          "downloadpage.install_folder.inherit_gdk",
        ) as unknown as string,
      },
      ...inheritOptions,
    ],
    [inheritOptions, t],
  );

  useEffect(() => {
    try {
      const getDir = GetInstallerDir as any;
      if (typeof getDir === "function") {
        (async () => {
          try {
            const d = await getDir();
            setInstallerDir(String(d || ""));
          } catch {}
        })();
      }
    } catch {}
  }, []);

  useEffect(() => {
    const checkResolved = async () => {
      if (!mirrorVersion) {
        setDownloadResolved(false);
        return;
      }
      try {
        const resolver = minecraft?.ResolveDownloadedMsixvc;
        if (typeof resolver === "function") {
          const name = await resolver(
            `${mirrorType || "Release"} ${mirrorVersion}`,
            String(mirrorType || "Release").toLowerCase(),
          );
          setDownloadResolved(Boolean(name));
        } else {
          setDownloadResolved(false);
        }
      } catch {
        setDownloadResolved(false);
      }
    };
    checkResolved();
  }, [mirrorVersion, mirrorType]);

  useEffect(() => {
    if (!installLeviLamina) {
      setLLSupportedVersions([]);
      setSelectedLLVersion("");
      return;
    }
    const versions = getSupportedLLVersions(mirrorVersion);
    setLLSupportedVersions(versions);
    setSelectedLLVersion((prev) => {
      if (prev && versions.includes(prev)) return prev;
      return versions[0] || "";
    });
  }, [installLeviLamina, mirrorVersion, getSupportedLLVersions]);

  const headerTitle = useMemo(() => {
    if (installing)
      return t("downloadmodal.installing.title") as unknown as string;
    if (resultMsg)
      return t("downloadpage.install.success_title") as unknown as string;
    return t("downloadpage.install_folder.confirm_title") as unknown as string;
  }, [installing, resultMsg, t]);

  const reportInstallFailure = (details: string) => {
    const raw = String(details || "ERR_INSTALL_FAILED");
    setFailureDetails(raw);
    toast(t("common.error"), {
      description: resolveInstallError(raw, t, typeLabel),
      variant: "danger",
      timeout: 5000,
    });
  };

  const proceedInstall = async () => {
    setFailureDetails("");
    setInstallError("");
    setResultMsg("");

    if (installLeviLamina) {
      try {
        const lipInstalled = await minecraft.IsLipInstalled();
        if (!lipInstalled) {
          reportInstallFailure("ERR_LIP_NOT_INSTALLED");
          return;
        }
      } catch {}
    }

    const name = (installName || "").trim();
    let installationCreated = false;
    const rollback = async () => {
      if (!installationCreated) return;
      try {
        const del = DeleteVersionFolder as any;
        if (typeof del === "function") {
          await del(name);
        }
      } catch {}
    };

    if (!name) {
      setInstallError("ERR_NAME_REQUIRED");
      return;
    }
    try {
      const validate = ValidateVersionFolderName as any;
      if (typeof validate === "function") {
        const msg: string = await validate(name);
        if (msg) {
          setInstallError(msg);
          return;
        }
      }
    } catch {}

    try {
      const install = minecraft?.InstallExtractMsixvc;
      const saveMeta = SaveVersionMeta as any;
      const copyFromGDK = CopyVersionDataFromGDK as any;
      const copyFromVersion = CopyVersionDataFromVersion as any;
      const resolver = minecraft?.ResolveDownloadedMsixvc;
      const isPrev = (mirrorType || "Release") === "Preview";
      let fname = "";
      if (customInstallerPath && customInstallerPath.trim().length > 0) {
        fname = customInstallerPath.trim();
      } else if (typeof resolver === "function") {
        try {
          fname = await resolver(
            (mirrorType || "Release") +
              " " +
              (mirrorVersion || installName || ""),
            String(mirrorType || "Release").toLowerCase(),
          );
        } catch {}
      }
      if (!fname) {
        reportInstallFailure("ERR_MSIXVC_NOT_SPECIFIED");
        return;
      }

      setInstalling(true);
      setInstallingVersion(mirrorVersion || installName || "");
      try {
        const disp = fname?.toLowerCase().endsWith(".msixvc")
          ? fname
          : `${fname}.msixvc`;
        setInstallingTargetName(disp);
      } catch {
        setInstallingTargetName(fname);
      }

      if (typeof install === "function") {
        const err: string = await install(fname, name, isPrev);
        if (err) {
          reportInstallFailure(err);
          setInstalling(false);
          return;
        }
        installationCreated = true;
      }

      if (typeof saveMeta === "function") {
        const metaError: string = await saveMeta(
          name,
          mirrorVersion || name,
          String(mirrorType || "Release").toLowerCase(),
          installIsolation,
          false,
          false,
          false,
          "",
          "",
        );
        if (metaError) throw new Error(metaError);
      }

      if (installIsolation && inheritSource) {
        try {
          let copyErr: string = "";
          if (inheritSource === "gdk") {
            if (typeof copyFromGDK === "function")
              copyErr = await copyFromGDK(isPrev, name);
          } else {
            if (typeof copyFromVersion === "function")
              copyErr = await copyFromVersion(inheritSource, name);
          }
          if (copyErr) {
            reportInstallFailure(copyErr);
            await rollback();
            setInstalling(false);
            return;
          }
        } catch (e: any) {
          reportInstallFailure(String(e?.message || e || ""));
          await rollback();
          setInstalling(false);
          return;
        }
      }

      if (installLeviLamina) {
        try {
          const installVersion = String(
            selectedLLVersion || getLatestLLVersion(mirrorVersion),
          ).trim();
          if (!installVersion) {
            reportInstallFailure("ERR_LL_VERSION_UNSUPPORTED");
            await rollback();
            setInstalling(false);
            return;
          }
          const installLL = (minecraft as any)?.InstallLeviLamina;
          if (typeof installLL === "function") {
            await runWithLipTask(
              {
                action: "install",
                target: name,
                methods: ["Install", "Update"],
                feedbackMode: "on_error",
              },
              async ({ addLog }) => {
                const llErr: string = await installLL(
                  mirrorVersion || installName || "",
                  name,
                  installVersion,
                );
                if (llErr) {
                  throw new Error(String(llErr));
                }
              },
            );
          }
        } catch (e: any) {
          reportInstallFailure(String(e?.message || e || ""));
          await rollback();
          setInstalling(false);
          return;
        }
      }

      try {
        let cachedItems: { version: string; short: string; type: ItemType }[] =
          [];
        try {
          const raw = localStorage.getItem("ll.version_items");
          const parsed = raw ? JSON.parse(raw) : [];
          if (Array.isArray(parsed)) {
            cachedItems = parsed.map((it: any) => ({
              version: String(it?.version || it?.short || ""),
              short: String(it?.short || it?.version || ""),
              type: String(it?.type || "Release") as ItemType,
            }));
          }
        } catch {}
        const itemsToRefresh =
          cachedItems && cachedItems.length > 0
            ? cachedItems
            : [
                {
                  version: String(mirrorVersion || installName || ""),
                  short: String(mirrorVersion || installName || ""),
                  type: (mirrorType || "Release") as ItemType,
                },
              ];
        await refreshAll(itemsToRefresh as any);
      } catch {}
      setResultMsg(t("downloadpage.install.success") as unknown as string);
      setInstalledFolderName(name);
      setInstalling(false);
    } catch (e: any) {
      reportInstallFailure(String(e?.message || e || ""));
      await rollback();
      setInstalling(false);
    }
  };

  const handleInstall = async () => {
    if (installLeviLamina && mirrorVersion) {
      const targetLLVersion = String(
        selectedLLVersion || getLatestLLVersion(mirrorVersion),
      ).trim();
      if (!targetLLVersion) {
        reportInstallFailure("ERR_LL_VERSION_UNSUPPORTED");
        return;
      }

      if (targetLLVersion && targetLLVersion.includes("rc")) {
        setRcVersion(targetLLVersion);
        rcOnOpen();
        return;
      }
    }
    await proceedInstall();
  };

  const handleOpenFolder = async () => {
    if (!installedFolderName) return;
    try {
      const vdir = await GetVersionsDir();
      const sep = vdir.includes("\\") ? "\\" : "/";
      const path = `${vdir}${sep}${installedFolderName}`;
      await minecraft.OpenPathDir(path);
    } catch (e) {
      console.error(e);
    }
  };

  return (
    <PageContainer>
      <div className="flex flex-col gap-4 w-full">
        {/* Header Card */}
        <motion.div
          initial={{ opacity: 0, y: -20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.4 }}
        >
          <Card className={LAYOUT.GLASS_CARD.BASE}>
            <Card.Content className="p-6 flex flex-row items-center justify-between gap-4">
              <div className="flex-1">
                <PageHeader
                  title={headerTitle}
                  description={
                    <div className="flex items-center gap-2">
                      <Chip
                        size="sm"
                        variant="soft"
                        color={mirrorType === "Preview" ? "warning" : "accent"}
                      >
                        <Chip.Label>
                          {mirrorType === "Preview"
                            ? `${t("common.preview")} Minecraft`
                            : `${t("common.release")} Minecraft`}
                        </Chip.Label>
                      </Chip>
                      <span className="font-mono">{mirrorVersion}</span>
                    </div>
                  }
                />
              </div>
              {!installing && (
                <div className="flex gap-2">
                  {!resultMsg ? (
                    <>
                      <Button
                        onPress={() => navigate(returnTo)}
                        variant={"ghost"}
                      >
                        {t("common.cancel")}
                      </Button>
                      <Button
                        onPress={handleInstall}
                        variant={"secondary"}
                        className={cn(
                          "rounded-full",
                          "bg-brand-500 hover:bg-brand-500 brand-primary-foreground font-bold shadow-lg shadow-brand-900/20",
                        )}
                      >
                        {t(
                          "downloadpage.customappx.modal.1.footer.install_button",
                        )}
                      </Button>
                    </>
                  ) : (
                    <div className="flex gap-2">
                      <Button
                        onPress={handleOpenFolder}
                        variant={"secondary"}
                        className={cn(
                          "rounded-full",
                          "bg-surface-secondary text-foreground dark:text-zinc-300 font-medium",
                        )}
                      >
                        {t("common.open_folder")}
                      </Button>
                      <Button onPress={() => navigate(returnTo)} variant="ghost">{t("common.back")}</Button>
                      <Button
                        onPress={() => {
                          saveCurrentVersionName(installedFolderName, "install-complete");
                          navigate(ROUTES.home);
                        }}
                        variant={"secondary"}
                        className={cn(
                          "rounded-full",
                          "bg-brand-500 hover:bg-brand-500 brand-primary-foreground font-bold shadow-lg shadow-brand-900/20",
                        )}
                      >
                        {t("audit.primary.install.go_launch")}
                      </Button>
                    </div>
                  )}
                </div>
              )}
            </Card.Content>
          </Card>
        </motion.div>

        {/* Content Area */}
        <AnimatePresence mode="wait">
          {installing ? (
            <motion.div
              key="installing"
              initial={{ opacity: 0, scale: 0.95 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.95 }}
            >
              <Card className={LAYOUT.GLASS_CARD.BASE}>
                <Card.Content className="py-12">
                  <div className="flex flex-col items-center justify-center h-full gap-4">
                    <div className="relative flex items-center justify-center">
                      <div className="absolute inset-0 bg-brand-500/20 blur-xl rounded-full animate-pulse" />
                      <div className="w-16 h-16 rounded-full bg-surface dark:bg-surface-secondary border-4 border-border dark:border-zinc-700 flex items-center justify-center relative z-10">
                        <Spinner size="md" color={"accent"} />
                      </div>
                    </div>

                    <div className="flex flex-col items-center gap-1 text-center max-w-sm">
                      <h2 className="text-xl font-bold text-foreground dark:text-white">
                        {t("downloadmodal.installing.title")}
                      </h2>
                      <p className="text-sm text-muted dark:text-zinc-400">
                        {t("downloadpage.install.hint")}
                      </p>
                    </div>

                    <div className="w-full max-w-lg flex flex-col gap-2">
                      {installingVersion && (
                        <div className="flex items-center justify-between px-3 py-2 rounded-xl bg-surface-secondary/50 ">
                          <span className="text-sm font-medium text-muted dark:text-zinc-400">
                            {t("downloadpage.install.version_label")}
                          </span>
                          <span className="text-sm font-bold text-foreground dark:text-zinc-300">
                            {installingVersion}
                          </span>
                        </div>
                      )}

                      {installingTargetName && (
                        <div className="flex flex-col gap-1 px-3 py-2 rounded-xl bg-surface-secondary/50 ">
                          <span className="text-[10px] font-bold uppercase tracking-wider text-muted">
                            {t("downloadpage.install.target")}
                          </span>
                          <span className="font-mono text-xs text-foreground dark:text-zinc-400 truncate">
                            {installingTargetName}
                          </span>
                        </div>
                      )}

                      <div className="mt-1 flex flex-col gap-2">
                        <div className="h-1.5 w-full rounded-full bg-surface-tertiary/50 overflow-hidden border border-border dark:border-white/5 relative">
                          {extractInfo?.totalBytes ? (
                            <motion.div
                              className="h-full bg-brand-500 rounded-full"
                              initial={{ width: 0 }}
                              animate={{
                                width: `${Math.min(
                                  100,
                                  Math.max(
                                    0,
                                    (extractInfo.bytes /
                                      extractInfo.totalBytes) *
                                      100,
                                  ),
                                )}%`,
                              }}
                              transition={{ duration: 0.3, ease: "easeOut" }}
                            />
                          ) : (
                            <></>
                          )}
                        </div>

                        {typeof extractInfo?.bytes === "number" &&
                        extractInfo.bytes > 0 ? (
                          <div className="flex justify-between text-xs text-muted dark:text-zinc-400 font-medium">
                            <span>
                              {extractInfo.totalBytes
                                ? (() => {
                                    const formatSize = (n: number) => {
                                      const kb = 1024;
                                      const mb = kb * 1024;
                                      const gb = mb * 1024;
                                      if (n >= gb)
                                        return (n / gb).toFixed(2) + " GB";
                                      if (n >= mb)
                                        return (n / mb).toFixed(2) + " MB";
                                      if (n >= kb)
                                        return (n / kb).toFixed(2) + " KB";
                                      return n + " B";
                                    };
                                    return `${formatSize(extractInfo.bytes)} / ${formatSize(extractInfo.totalBytes)}`;
                                  })()
                                : t("downloadpage.install.estimated_size")}
                            </span>
                            <span className="font-mono">
                              {(() => {
                                const formatSize = (n: number) => {
                                  const kb = 1024;
                                  const mb = kb * 1024;
                                  const gb = mb * 1024;
                                  if (n >= gb)
                                    return (n / gb).toFixed(2) + " GB";
                                  if (n >= mb)
                                    return (n / mb).toFixed(2) + " MB";
                                  if (n >= kb)
                                    return (n / kb).toFixed(2) + " KB";
                                  return n + " B";
                                };
                                const current = formatSize(
                                  extractInfo?.bytes ?? 0,
                                );
                                if (extractInfo?.totalBytes) {
                                  const percent = (
                                    (extractInfo.bytes /
                                      extractInfo.totalBytes) *
                                    100
                                  ).toFixed(1);
                                  return `${percent}%`;
                                }
                                return current;
                              })()}
                            </span>
                          </div>
                        ) : null}
                      </div>
                    </div>
                  </div>
                </Card.Content>
              </Card>
            </motion.div>
          ) : resultMsg ? (
            <motion.div
              key="success"
              initial={{ opacity: 0, scale: 0.95 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.95 }}
            >
              <Card className={LAYOUT.GLASS_CARD.BASE}>
                <Card.Content className="py-12">
                  <div className="flex flex-col items-center justify-center h-full gap-4">
                    <div className="relative">
                      <motion.div
                        initial={{ scale: 0 }}
                        animate={{ scale: 1 }}
                        transition={{
                          type: "spring",
                          stiffness: 260,
                          damping: 20,
                          delay: 0.1,
                        }}
                        className="w-16 h-16 rounded-full bg-accent flex items-center justify-center shadow-lg shadow-brand-900/20"
                      >
                        <svg
                          viewBox="0 0 24 24"
                          width="32"
                          height="32"
                          className="text-white drop-shadow-md"
                          fill="none"
                          stroke="currentColor"
                          strokeWidth="3"
                          strokeLinecap="round"
                          strokeLinejoin="round"
                        >
                          <motion.path
                            initial={{ pathLength: 0 }}
                            animate={{ pathLength: 1 }}
                            transition={{ duration: 0.5, delay: 0.3 }}
                            d="M20 6L9 17l-5-5"
                          />
                        </svg>
                      </motion.div>
                    </div>

                    <div className="flex flex-col items-center gap-1 text-center">
                      <h2 className="text-2xl font-black text-brand-700 dark:text-brand-300">
                        {t("downloadpage.install.success_title")}
                      </h2>
                      {installingVersion && (
                        <Chip size="sm" variant="soft" color={"accent"}>
                          <Chip.Label className={"font-bold"}>
                            {installingVersion}
                          </Chip.Label>
                        </Chip>
                      )}
                      <p className="text-muted dark:text-zinc-400 text-sm mt-2 max-w-xs">
                        {t("downloadpage.install.success")}
                      </p>
                    </div>

                    <div className="w-full max-w-lg mt-1">
                      {installingTargetName && (
                        <div className="rounded-xl bg-surface-secondary/50 border border-border/50 dark:border-white/5 p-3 flex flex-col gap-1 items-center">
                          <span className="text-[10px] uppercase tracking-wider text-muted font-bold">
                            {t("downloadpage.install.target")}
                          </span>
                          <span className="font-mono text-xs text-foreground dark:text-zinc-400 truncate w-full text-center">
                            {installingTargetName}
                          </span>
                        </div>
                      )}
                    </div>
                  </div>
                </Card.Content>
              </Card>
            </motion.div>
          ) : (
            <motion.div
              key="input"
              className="flex flex-col gap-4"
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -10 }}
            >
              {failureDetails && (
                <Card className={cn(LAYOUT.GLASS_CARD.BASE, "border-danger/30")}>
                  <Card.Content className="space-y-3 p-5">
                    <div role="alert" className="space-y-2">
                      <h3 className="font-semibold text-danger">{t("audit.primary.install.failed")}</h3>
                      <p className="text-sm text-foreground">{resolveInstallError(failureDetails, t, typeLabel)}</p>
                      <pre className="whitespace-pre-wrap break-all text-xs text-muted select-text">{failureDetails}</pre>
                    </div>
                    <div className="flex flex-wrap gap-2">
                      <Button variant="secondary" size="sm" onPress={async () => {
                        try {
                          await navigator.clipboard.writeText(failureDetails);
                          toast(t("audit.primary.install.copied"), { variant: "success" });
                        } catch {
                          toast(t("audit.primary.install.copy_failed"), { variant: "danger" });
                        }
                      }}>{t("audit.primary.install.copy_details")}</Button>
                      <Button variant="primary" size="sm" onPress={handleInstall}>{t("download_manager.actions.retry")}</Button>
                    </div>
                  </Card.Content>
                </Card>
              )}

              {/* Basic Configuration */}
              <Card className={LAYOUT.GLASS_CARD.BASE}>
                <Card.Header className={LAYOUT.GLASS_CARD.HEADER}>
                  <h3 className="text-lg font-medium">
                    {t("settings.tabs.general")}
                  </h3>
                </Card.Header>
                <Card.Content className="p-6">
                  <TextField
                    isInvalid={!!installError}
                    className={cn("group", COMPONENT_STYLES.input.mainWrapper)}
                    value={installName}
                    onChange={setInstallName}
                  >
                    <Label className={COMPONENT_STYLES.input.label}>
                      {
                        t(
                          "downloadpage.install_folder.name_label",
                        ) as unknown as string
                      }
                    </Label>
                    <Input
                      placeholder={
                        t(
                          "downloadpage.install_folder.placeholder_name",
                        ) as unknown as string
                      }
                      className={cn(
                        COMPONENT_STYLES.input.inputWrapper,
                        COMPONENT_STYLES.input.input,
                        "min-h-8 text-sm",
                      )}
                    />
                    <FieldError className={COMPONENT_STYLES.input.errorMessage}>
                      {installError
                        ? resolveInstallError(installError, t, typeLabel)
                        : undefined}
                    </FieldError>
                  </TextField>
                </Card.Content>
              </Card>

              {/* Advanced Options */}
              <Card className={LAYOUT.GLASS_CARD.BASE}>
                <Card.Header className={LAYOUT.GLASS_CARD.HEADER}>
                  <h3 className="text-lg font-medium">
                    {t("settings.tabs.others")}
                  </h3>
                </Card.Header>
                <Card.Content className="p-6 flex flex-col gap-4">
                  {!downloadResolved && (
                    <div className="flex items-center justify-between p-3 rounded-2xl bg-surface/50 dark:bg-surface-secondary/10 border border-border dark:border-white/5">
                      <div className="min-w-0">
                        <div className="text-sm font-medium">
                          {t("downloadpage.install.custom_installer.label")}
                        </div>
                        <div className="text-xs text-muted dark:text-zinc-400">
                          {customInstallerPath
                            ? customInstallerPath
                            : (t(
                                "downloadpage.install.custom_installer.hint",
                              ) as unknown as string)}
                        </div>
                      </div>
                      <Button
                        size="sm"
                        onPress={async () => {
                          try {
                            const paths = await Dialogs.OpenFile({
                              Title: t(
                                "downloadpage.customappx.modal.1.header",
                              ),
                              Filters: [
                                {
                                  DisplayName: "Installer Files",
                                  Pattern: "*.msixvc",
                                },
                              ],
                              AllowsMultipleSelection: false,
                              Directory: installerDir || "",
                            });
                            if (Array.isArray(paths) && paths.length > 0) {
                              setCustomInstallerPath(paths[0]);
                            } else if (typeof paths === "string" && paths) {
                              setCustomInstallerPath(paths);
                            }
                          } catch (e) {
                            console.error(e);
                          }
                        }}
                        variant={"secondary"}
                        className={"bg-surface-secondary dark:bg-surface/10"}
                      >
                        {t("common.browse")}
                      </Button>
                    </div>
                  )}
                  {isLeviLaminaSupported && (
                    <div className="flex items-center justify-between p-3 rounded-2xl bg-surface/50 dark:bg-surface-secondary/10 border border-border dark:border-white/5">
                      <div className="min-w-0">
                        <div className="text-sm font-medium">
                          {t("downloadpage.install.levilamina_label")}
                        </div>
                        <div className="text-xs text-muted dark:text-zinc-400">
                          {t("downloadpage.install.levilamina_desc")}
                        </div>
                      </div>
                      <Switch
                        aria-label={t("downloadpage.install.levilamina_label")}
                        isSelected={installLeviLamina}
                        onChange={setInstallLeviLamina}
                        className={"group"}
                      >
                        <Switch.Content>
                          <Switch.Control>
                            <Switch.Thumb></Switch.Thumb>
                          </Switch.Control>
                          <span></span>
                        </Switch.Content>
                      </Switch>
                    </div>
                  )}
                  {isLeviLaminaSupported && installLeviLamina && (
                    <div className="p-3 rounded-2xl bg-surface/50 dark:bg-surface-secondary/10 border border-border dark:border-white/5">
                      <Select
                        placeholder={
                          t(
                            "downloadpage.install.ll_version_placeholder",
                          ) as unknown as string
                        }
                        value={
                          Array.from(
                            selectedLLVersion
                              ? new Set([selectedLLVersion])
                              : new Set([]),
                          )[0] ?? null
                        }
                        onChange={(keys) => {
                          const selected = keys;
                          setSelectedLLVersion(String(selected || ""));
                        }}
                      >
                        <Label>
                          {
                            t(
                              "downloadpage.install.ll_version_label",
                            ) as unknown as string
                          }
                        </Label>
                        <Select.Trigger
                          className={COMPONENT_STYLES.select.trigger}
                        >
                          <Select.Value />
                          <Select.Indicator />
                        </Select.Trigger>
                        <Select.Popover
                          className={COMPONENT_STYLES.select.popoverContent}
                        >
                          <ListBox className={COMPONENT_STYLES.select.listbox}>
                            {llSupportedVersions.map((version) => (
                              <ListBox.Item
                                key={version}
                                id={version}
                                textValue={version}
                              >
                                <Label>{version}</Label>
                                <ListBox.ItemIndicator />
                              </ListBox.Item>
                            ))}
                          </ListBox>
                        </Select.Popover>
                      </Select>
                    </div>
                  )}
                  <div className="flex items-center justify-between p-3 rounded-2xl bg-surface/50 dark:bg-surface-secondary/10 border border-border dark:border-white/5">
                    <div className="min-w-0">
                      <div className="text-sm font-medium">
                        {t("downloadpage.install_folder.enable_isolation")}
                      </div>
                      <div className="text-xs text-muted dark:text-zinc-400">
                        {t("downloadpage.install_folder.enable_isolation_desc")}
                      </div>
                    </div>
                    <Switch
                      aria-label={t("downloadpage.install_folder.enable_isolation")}
                      isSelected={installIsolation}
                      onChange={setInstallIsolation}
                      className={"group"}
                    >
                      <Switch.Content>
                        <Switch.Control>
                          <Switch.Thumb></Switch.Thumb>
                        </Switch.Control>
                        <span></span>
                      </Switch.Content>
                    </Switch>
                  </div>
                  {installIsolation && (
                    <div className="flex items-center justify-between p-3 rounded-2xl bg-surface/50 dark:bg-surface-secondary/10 border border-border dark:border-white/5">
                      <div className="min-w-0">
                        <div className="text-sm font-medium">
                          {t("downloadpage.install_folder.inherit_label")}
                        </div>
                        <div className="text-xs text-muted dark:text-zinc-400">
                          {t("downloadpage.install_folder.inherit_hint")}
                        </div>
                      </div>
                      <div className="shrink-0 min-w-[240px]">
                        <Dropdown>
                          <Button
                            size="sm"
                            variant={"secondary"}
                            className={
                              "bg-surface-secondary dark:bg-surface/10 w-full justify-between"
                            }
                          >
                            {inheritLabel}
                            {<FaChevronDown size={12} />}
                          </Button>
                          <Dropdown.Popover
                            className={COMPONENT_STYLES.dropdown.content}
                          >
                            <Dropdown.Menu
                              aria-label="inherit-source-select"
                              selectionMode="single"
                              disallowEmptySelection
                              selectedKeys={new Set([inheritSource || "none"])}
                              className="max-h-64 overflow-y-auto min-w-[240px] no-scrollbar"
                              items={inheritMenuItems}
                              onSelectionChange={(keys) => {
                                const arr = Array.from(
                                  keys as unknown as Set<string>,
                                );
                                const k = String(arr[0] || "");
                                if (!k) return;
                                setInheritSource(k === "none" ? "" : k);
                              }}
                            >
                              {(item: { key: string; label: string }) => (
                                <Dropdown.Item
                                  key={item.key}
                                  id={item.key}
                                  textValue={item.label}
                                >
                                  <Label>{item.label}</Label>
                                  <Dropdown.ItemIndicator />
                                </Dropdown.Item>
                              )}
                            </Dropdown.Menu>
                          </Dropdown.Popover>
                        </Dropdown>
                      </div>
                    </div>
                  )}
                </Card.Content>
              </Card>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
      <UnifiedModal
        size="standard"
        isOpen={rcOpen}
        onOpenChange={rcOnOpenChange}
        type="warning"
        title={t("mods.rc_warning.title")}
        cancelText={t("common.cancel")}
        confirmText={t("common.continue")}
        showCancelButton
        onCancel={rcOnClose}
        onConfirm={() => {
          rcOnClose();
          proceedInstall();
        }}
      >
        <ModalDescription className="space-y-2">
          <p>
            {t("mods.rc_warning.body_1", {
              version: rcVersion,
            })}
          </p>
          <p className="font-semibold text-amber-700 dark:text-amber-300">
            {t("mods.rc_warning.body_2")}
          </p>
          <p>{t("mods.rc_warning.body_3")}</p>
        </ModalDescription>
      </UnifiedModal>
    </PageContainer>
  );
}
