import { openDirectory, showDirectoryOpenError } from "@/utils/explorer";
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
  ProgressBar,
  useOverlayState,
} from "@heroui/react";

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import { useLocation, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  FaCheckCircle,
  FaChevronDown,
  FaCircle,
  FaExclamationCircle,
  FaExclamationTriangle,
  FaRedo,
} from "react-icons/fa";
import {
  appendTaskLog,
  finishActivityTask,
  notify,
  openActivityCenter,
  setTaskProgress,
  startActivityTask,
  type ActivityLogLevel,
  type ActivityLogLine,
} from "@/utils/activity";
import {
  downloadErrorText,
  isDownloadActive,
  useDownloads,
} from "@/utils/DownloadsContext";
import { formatBytes, formatSpeed } from "@/utils/formatting";
import { ROUTES } from "@/constants/routes";
import { useVersionStatus } from "@/utils/VersionStatusContext";
import { useLeviLamina } from "@/utils/LeviLaminaContext";
import {
  LipTaskProgress,
  useLipTaskConsole,
} from "@/utils/LipTaskConsoleContext";
import { resolveInstallError } from "@/utils/installError";
import { saveCurrentVersionName } from "@/utils/currentVersion";
import { setNavLockReason } from "@/hooks/useAppNavigation";
import { motion, AnimatePresence } from "framer-motion";
import { Clipboard, Dialogs, Events } from "@wailsio/runtime";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import {
  CopyVersionDataFromGDK,
  CopyVersionDataFromVersion,
  DeleteVersionFolder,
  GetInstallerDir,
  GetVersionMeta,
  GetVersionsDir,
  LaunchVersionByName,
  ListInheritableVersionNames,
  ListVersionMetas,
  SaveVersionMeta,
  ValidateVersionFolderName,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/versionservice";
import { UnifiedModal } from "@/components/UnifiedModal";
import { PageHeader } from "@/components/PageHeader";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { cn } from "@/utils/cn";
import { isAppxInstaller, normalizePackageType, normalizeVersionChannel, supportsVersionIsolation, UWP_ISOLATION_MIN_VERSION, type VersionChannel } from "@/utils/packageType";

type ItemType = VersionChannel;

const INSTALL_ISOLATION_PREFERENCE_KEY = "ll.install.enableIsolation";

function readInstallIsolationPreference(): boolean {
  try {
    const value = localStorage.getItem(INSTALL_ISOLATION_PREFERENCE_KEY);
    if (value === "true") return true;
    if (value === "false") return false;
  } catch {}
  return true;
}

const INSTALL_SET_CURRENT_PREFERENCE_KEY = "ll.install.setCurrent";

function readSetCurrentPreference(): boolean {
  try {
    return localStorage.getItem(INSTALL_SET_CURRENT_PREFERENCE_KEY) !== "false";
  } catch {}
  return true;
}

// Label for the "Back to …" exit; unknown origins fall back to a plain "Back".
const ORIGIN_LABEL_KEYS: Record<string, string> = {
  [ROUTES.download]: "download_flow.origin.download",
  [ROUTES.downloadTasks]: "download_flow.origin.tasks",
  [ROUTES.home]: "download_flow.origin.home",
  [ROUTES.instances]: "download_flow.origin.instances",
};


const STEP_ICONS: Record<ActivityLogLevel, ReactNode> = {
  info: <FaCircle size={6} className="text-muted" />,
  success: <FaCheckCircle size={12} className="text-success" />,
  warning: <FaExclamationTriangle size={12} className="text-warning" />,
  error: <FaExclamationCircle size={12} className="text-danger" />,
};

/** Appended install steps; earlier lines are never replaced or cleared. */
function InstallStepLog({ steps, className }: { steps: ActivityLogLine[]; className?: string }) {
  const { t } = useTranslation();
  const listRef = useRef<HTMLOListElement>(null);
  useEffect(() => {
    const list = listRef.current;
    if (list) list.scrollTop = list.scrollHeight;
  }, [steps.length]);
  if (steps.length === 0) return null;
  return (
    <section aria-label={t("download_flow.install.steps")} className={cn("w-full flex flex-col gap-1", className)}>
      <div className="flex items-center justify-between gap-2 px-1">
        <span className="text-xs font-bold uppercase tracking-wider text-muted">
          {t("download_flow.install.steps")}
        </span>
        <Button size="sm" variant="ghost" onPress={openActivityCenter}>
          {t("download_flow.show_log")}
        </Button>
      </div>
      <ol
        ref={listRef}
        aria-live="polite"
        className="max-h-[min(14rem,35vh)] overflow-y-auto rounded-xl bg-surface-secondary/50 border border-border/50 dark:border-white/5 p-2 flex flex-col gap-1 select-text"
      >
        {steps.map((step, index) => (
          <li key={`${step.at}-${index}`} className="flex items-start gap-2 text-xs">
            <span className="mt-1 flex w-3 shrink-0 justify-center">{STEP_ICONS[step.level]}</span>
            <span className="font-mono text-muted shrink-0">
              {new Date(step.at).toLocaleTimeString()}
            </span>
            <span
              className={cn(
                "min-w-0 break-words",
                step.level === "error" ? "text-danger" : "text-foreground dark:text-zinc-300",
              )}
            >
              {step.message}
            </span>
          </li>
        ))}
      </ol>
    </section>
  );
}

export default function InstallPage() {
  const { t } = useTranslation();
  const { runWithLipTask } = useLipTaskConsole();
  const navigate = useNavigate();
  const location = useLocation() as any;
  const { refreshAll } = useVersionStatus();

  const mirrorVersion: string = String(location?.state?.mirrorVersion || "");
  const mirrorType = normalizeVersionChannel(location?.state?.mirrorType);
  const typeLabel: string = (mirrorType === "Beta" ? t("uwp.beta") : mirrorType === "Preview"
    ? (t("common.preview") as unknown as string)
    : (t("common.release") as unknown as string)) as unknown as string;
  const returnTo: string = String(location?.state?.returnTo || ROUTES.download);

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
  // The LeviLamina step runs lip tasks; their progress replaces the
  // extraction bar while it runs.
  const [installingLL, setInstallingLL] = useState<boolean>(false);
  const [installingVersion, setInstallingVersion] = useState<string>("");
  const [installingTargetName, setInstallingTargetName] = useState<string>("");
  const [installedFolderName, setInstalledFolderName] = useState<string>("");
  const [resultMsg, setResultMsg] = useState<string>("");
  const [customInstallerPath, setCustomInstallerPath] = useState<string>(() => String(location?.state?.installerPath || ""));
  const packageType = customInstallerPath
    ? isAppxInstaller(customInstallerPath) ? "uwp" : "gdk"
    : normalizePackageType(location?.state?.packageType);
  const isUWP = packageType === "uwp";
  // Unknown local packages can request isolation; their manifest is checked after extraction.
  const isolationSupported = (isUWP && !!customInstallerPath && !mirrorVersion.trim()) ||
    supportsVersionIsolation(packageType, mirrorVersion);
  const isLeviLaminaSupported = !isUWP && Boolean(location?.state?.isLeviLaminaSupported);
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
  const [setAsCurrent, setSetAsCurrent] = useState<boolean>(() => readSetCurrentPreference());
  const [madeCurrent, setMadeCurrent] = useState(false);
  const [launching, setLaunching] = useState(false);

  // Install steps are appended here and mirrored to the activity center.
  const [steps, setSteps] = useState<ActivityLogLine[]>([]);
  const activityIdRef = useRef<string | null>(null);
  const extractMilestoneRef = useRef(-1);
  const logStep = useCallback((message: string, level: ActivityLogLevel = "info") => {
    setSteps((prev) => [...prev, { at: Date.now(), level, message }]);
    if (activityIdRef.current) appendTaskLog(activityIdRef.current, message, level);
  }, []);

  // "Download & install": the download keeps running in the background while
  // the user fills in the options here, and installation continues when it ends.
  const { downloads, startDownload, watchInstall, dismissInstallPrompt } = useDownloads();
  const [pendingDownload, setPendingDownload] = useState<string>(() =>
    String(location?.state?.pendingDownload || ""),
  );
  const [installQueued, setInstallQueued] = useState(false);
  const pendingTask = pendingDownload
    ? downloads.find((item) => item.dest === pendingDownload)
    : undefined;
  const pendingStatus = pendingTask?.status || "";
  // A download that is no longer tracked (e.g. after a reload) does not block installing.
  const awaitingDownload = !!pendingTask && pendingStatus !== "done";

  useEffect(() => {
    try {
      localStorage.setItem(
        INSTALL_ISOLATION_PREFERENCE_KEY,
        installIsolation ? "true" : "false",
      );
    } catch {}
  }, [installIsolation]);

  useEffect(() => {
    try {
      localStorage.setItem(
        INSTALL_SET_CURRENT_PREFERENCE_KEY,
        setAsCurrent ? "true" : "false",
      );
    } catch {}
  }, [setAsCurrent]);

  useEffect(() => {
    if (!pendingDownload) return;
    return watchInstall(pendingDownload);
  }, [pendingDownload, watchInstall]);

  useEffect(() => {
    if (pendingStatus !== "done" || !pendingDownload) return;
    // The finished file is the installer; a bare key falls back to the resolver.
    if (/[\\/]/.test(pendingDownload)) setCustomInstallerPath(pendingDownload);
    setDownloadResolved(true);
  }, [pendingStatus, pendingDownload]);

  useEffect(() => {
    if (!installing) return;
    // Extraction progress is kept after the install ends; milestones are
    // appended to the step log instead of overwriting one status line.
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
      if (totalBytes > 0) {
        const percent = Math.min(100, Math.floor((bytes / totalBytes) * 100));
        const milestone = Math.floor(percent / 25);
        if (activityIdRef.current) setTaskProgress(activityIdRef.current, percent);
        if (milestone > extractMilestoneRef.current) {
          extractMilestoneRef.current = milestone;
          if (milestone > 0) {
            logStep(
              t("download_flow.install.extracted", {
                percent: milestone * 25,
                done: formatBytes(bytes),
                total: formatBytes(totalBytes),
              }),
            );
          }
        }
      } else if (files > 0) {
        const milestone = Math.floor(files / 1000);
        if (milestone > extractMilestoneRef.current) {
          extractMilestoneRef.current = milestone;
          if (milestone > 0) {
            logStep(t("download_flow.install.extracted_files", { count: files }));
          }
        }
      }
    });
    return () => off();
  }, [installing, logStep, t]);

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
    if (isUWP || !installIsolation) {
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
  }, [installIsolation, isUWP]);

  useEffect(() => {
    if (isUWP || !installIsolation) {
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
  }, [installIsolation, mirrorType, isUWP]);

  const inheritOptions = useMemo(() => {
    const type = String(mirrorType || "Release").toLowerCase();
    const allowed = new Set(
      (inheritCandidates || []).map((n) => String(n || "")),
    );
    return (inheritMetas || [])
      .filter(
        (m: any) =>
          normalizePackageType(m?.packageType) === "gdk" &&
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
        const resolver = isUWP ? minecraft.ResolveDownloadedUWP : minecraft.ResolveDownloadedMsixvc;
        if (typeof resolver === "function") {
          const name = await resolver(
            isUWP ? mirrorVersion : `${mirrorType || "Release"} ${mirrorVersion}`,
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
  }, [mirrorVersion, mirrorType, isUWP]);

  useEffect(() => {
    if (isUWP || !installLeviLamina) {
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
  }, [installLeviLamina, mirrorVersion, getSupportedLLVersions, isUWP]);

  const headerTitle = useMemo(() => {
    if (installing)
      return t("downloadmodal.installing.title") as unknown as string;
    if (resultMsg)
      return t("downloadpage.install.success_title") as unknown as string;
    return t("downloadpage.install_folder.confirm_title") as unknown as string;
  }, [installing, resultMsg, t]);

  const reportInstallFailure = (details: string) => {
    const raw = String(details || "ERR_INSTALL_FAILED");
    const message = resolveInstallError(raw, t, typeLabel);
    setFailureDetails(raw);
    logStep(message, "error");
    if (activityIdRef.current) {
      finishActivityTask(activityIdRef.current, "error", message);
      activityIdRef.current = null;
    }
    notify({
      variant: "danger",
      source: "install",
      title: t("audit.primary.install.failed"),
      description: message,
    });
  };

  const proceedInstall = async () => {
    setFailureDetails("");
    setInstallError("");
    setResultMsg("");
    setMadeCurrent(false);

    const name = (installName || "").trim();
    let installationCreated = false;
    const rollback = async () => {
      if (!installationCreated) return;
      try {
        const del = DeleteVersionFolder as any;
        if (typeof del === "function") {
          await del(name);
          logStep(t("download_flow.install.rolled_back", { name }), "warning");
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

    if (activityIdRef.current) finishActivityTask(activityIdRef.current, "cancelled");
    activityIdRef.current = startActivityTask({
      title: t("download_flow.install.activity_title", { name }),
      source: "install",
    });
    // Each attempt gets a fresh step list; earlier attempts stay in the activity center.
    setSteps([]);
    logStep(t("download_flow.install.started", { name, version: mirrorVersion || name }));
    // This download is being installed now; don't offer it again elsewhere.
    if (pendingDownload) dismissInstallPrompt(pendingDownload);
    if (customInstallerPath) dismissInstallPrompt(customInstallerPath);

    try {
      const saveMeta = SaveVersionMeta as any;
      const copyFromGDK = CopyVersionDataFromGDK as any;
      const copyFromVersion = CopyVersionDataFromVersion as any;
      const resolver = isUWP ? minecraft.ResolveDownloadedUWP : minecraft.ResolveDownloadedMsixvc;
      const isPrev = (mirrorType || "Release") === "Preview";
      let fname = "";
      if (customInstallerPath && customInstallerPath.trim().length > 0) {
        fname = customInstallerPath.trim();
      } else if (typeof resolver === "function") {
        try {
          fname = await resolver(
            isUWP ? mirrorVersion : (mirrorType || "Release") + " " + (mirrorVersion || installName || ""),
            String(mirrorType || "Release").toLowerCase(),
          );
        } catch {}
      }
      if (!fname) {
        reportInstallFailure(isUWP ? "ERR_UWP_NOT_SPECIFIED" : "ERR_MSIXVC_NOT_SPECIFIED");
        return;
      }

      setInstalling(true);
      setInstallingVersion(mirrorVersion || installName || "");
      try {
        const disp = /\.(msixvc|appx|msix|appxbundle|msixbundle)$/i.test(fname)
          ? fname
          : `${fname}.msixvc`;
        setInstallingTargetName(disp);
      } catch {
        setInstallingTargetName(fname);
      }
      setExtractInfo(null);
      extractMilestoneRef.current = -1;
      logStep(t("download_flow.install.extracting", { file: fname.split(/[\\/]/).pop() || fname }));

      {
        const err = isUWP
          ? await minecraft.InstallExtractAppx(fname, name, mirrorType.toLowerCase())
          : await minecraft.InstallExtractMsixvc(fname, name, isPrev);
        if (err) {
          reportInstallFailure(err);
          setInstalling(false);
          return;
        }
        installationCreated = true;
        logStep(t("download_flow.install.extract_done"), "success");
      }

      if (typeof saveMeta === "function") {
        logStep(t("download_flow.install.saving_settings"));
        // Appx manifests determine the installed version and channel, including
        // local packages whose identity differs from the install page defaults.
        const installedMeta = isUWP ? await GetVersionMeta(name) : null;
        const metaError: string = await saveMeta(
          name,
          installedMeta ? installedMeta.gameVersion : mirrorVersion || name,
          installedMeta ? installedMeta.type : String(mirrorType || "Release").toLowerCase(),
          installIsolation && isolationSupported && supportsVersionIsolation(packageType, installedMeta ? installedMeta.gameVersion : mirrorVersion),
          false,
          false,
          "",
          "",
        );
        if (metaError) throw new Error(metaError);
      }

      if (!isUWP && installIsolation && inheritSource) {
        logStep(t("download_flow.install.copying_data", { source: inheritLabel }));
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

      if (!isUWP && installLeviLamina) {
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
            setInstallingLL(true);
            logStep(t("download_flow.install.installing_ll", { version: installVersion }));
            await runWithLipTask(
              {
                action: "install",
                target: name,
                methods: ["Install", "Update"],
                feedbackMode: "inline",
              },
              async () => {
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
            logStep(t("download_flow.install.ll_done", { version: installVersion }), "success");
          }
        } catch (e: any) {
          reportInstallFailure(String(e?.message || e || ""));
          await rollback();
          setInstalling(false);
          return;
        } finally {
          setInstallingLL(false);
        }
      }

      try {
        let cachedItems: { version: string; short: string; type: ItemType; packageType: "gdk" | "uwp" }[] =
          [];
        try {
          const raw = localStorage.getItem("ll.version_items");
          const parsed = raw ? JSON.parse(raw) : [];
          if (Array.isArray(parsed)) {
            cachedItems = parsed.map((it: any) => ({
              version: String(it?.version || it?.short || ""),
              short: String(it?.short || it?.version || ""),
              type: normalizeVersionChannel(it?.type),
              packageType: normalizePackageType(it?.packageType),
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
                  type: mirrorType,
                  packageType,
                },
              ];
        await refreshAll(itemsToRefresh as any);
      } catch {}
      if (setAsCurrent) {
        saveCurrentVersionName(name, "install-complete");
        setMadeCurrent(true);
        logStep(t("download_flow.install.set_current", { name }));
      }
      const doneMessage = t("download_flow.install.done", { name });
      logStep(doneMessage, "success");
      if (activityIdRef.current) {
        finishActivityTask(activityIdRef.current, "success", doneMessage);
        activityIdRef.current = null;
      }
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
    if (awaitingDownload) {
      // Remember the intent; installation starts as soon as the file is ready.
      if (!installError) setInstallQueued((queued) => !queued);
      return;
    }
    if (!isUWP && installLeviLamina && mirrorVersion) {
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

  const installReady =
    !awaitingDownload &&
    (!pendingTask || !/[\\/]/.test(pendingDownload) || customInstallerPath === pendingDownload);
  useEffect(() => {
    if (!installQueued || !installReady || installing) return;
    setInstallQueued(false);
    void handleInstall();
  }, [installQueued, installReady, installing]);

  useEffect(() => {
    // A failed or cancelled download drops the queued install; the user retries explicitly.
    if (pendingStatus === "error" || pendingStatus === "cancelled") setInstallQueued(false);
  }, [pendingStatus]);

  const retryPendingDownload = async () => {
    if (!pendingTask?.url) return;
    const key = await startDownload(
      pendingTask.url,
      pendingTask.fileName,
      pendingTask.md5sum,
      pendingTask.installer,
    );
    if (key) setPendingDownload(key);
  };

  const originLabelKey = ORIGIN_LABEL_KEYS[returnTo];
  const backLabel = originLabelKey
    ? t("download_flow.back_to", { place: t(originLabelKey) })
    : t("common.back");

  const goToInstance = () => {
    navigate(ROUTES.instanceSettings, {
      state: { name: installedFolderName, returnTo: ROUTES.instances },
    });
  };

  const makeCurrent = () => {
    saveCurrentVersionName(installedFolderName, "install-complete");
    setMadeCurrent(true);
  };

  const launchNow = async () => {
    if (!installedFolderName || launching) return;
    // Launching always targets the new instance, so it becomes the current one.
    makeCurrent();
    setLaunching(true);
    try {
      const err = String((await LaunchVersionByName(installedFolderName)) || "");
      if (err) throw new Error(err);
      notify({
        variant: "success",
        source: "install",
        title: t("download_flow.launch.started", { name: installedFolderName }),
      });
      navigate(ROUTES.home);
    } catch (e: any) {
      const code = String(e?.message || e || "ERR_LAUNCH_GAME");
      const key = `errors.${code.split(":")[0].trim()}`;
      const translated = t(key);
      notify({
        variant: "danger",
        source: "install",
        title: t("download_flow.launch.failed"),
        // The home page has the full launch flow (force launch, UWP registration).
        actionProps: {
          children: t("download_flow.launch.open_home"),
          onPress: () => navigate(ROUTES.home),
        },
        description: code.includes("ERR_UWP_")
          ? resolveInstallError(code, t)
          : translated && translated !== key
            ? translated
            : code,
      });
    } finally {
      setLaunching(false);
    }
  };

  const handleOpenFolder = async () => {
    if (!installedFolderName) return;
    try {
      const vdir = await GetVersionsDir();
      const sep = vdir.includes("\\") ? "\\" : "/";
      const path = `${vdir}${sep}${installedFolderName}`;
      await openDirectory(path);
    } catch (e) {
      showDirectoryOpenError(e);
    }
  };

  return (
    <PageContainer>
      <div className={cn(LAYOUT.PAGE.READABLE, "flex flex-col gap-4")}>
        {/* Header Card */}
        <motion.div
          data-material-motion
          initial={{ opacity: 0, y: -20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.4 }}
        >
          <Card className={LAYOUT.GLASS_CARD.BASE}>
            <Card.Content className="p-4 sm:p-6 flex flex-row flex-wrap items-center justify-between gap-4">
              <div className="min-w-0 flex-[1_1_16rem]">
                <PageHeader
                  title={headerTitle}
                  description={
                    <div className="flex flex-wrap items-center gap-2">
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
              {!installing && !resultMsg && (
                <div className="flex flex-wrap gap-2 ml-auto">
                  <Button
                    onPress={() => navigate(returnTo)}
                    variant={"ghost"}
                  >
                    {t("common.cancel")}
                  </Button>
                  <Button
                    onPress={handleInstall}
                    variant={installQueued ? "secondary" : "primary"}
                    aria-pressed={awaitingDownload ? installQueued : undefined}
                    isDisabled={awaitingDownload && (pendingStatus === "error" || pendingStatus === "cancelled")}
                    className={cn(
                      "rounded-full font-bold",
                      !installQueued && "bg-brand-500 hover:bg-brand-500 brand-primary-foreground shadow-lg shadow-brand-900/20",
                    )}
                  >
                    {awaitingDownload
                      ? t(installQueued ? "download_flow.install.queued" : "download_flow.install.when_downloaded")
                      : t("downloadpage.customappx.modal.1.footer.install_button")}
                  </Button>
                </div>
              )}
            </Card.Content>
          </Card>
        </motion.div>

        {/* Download that feeds this install ("Download & install") */}
        {pendingTask && !installing && !resultMsg && (
          <Card className={cn(LAYOUT.GLASS_CARD.BASE, pendingStatus === "error" && "border-danger/30")}>
            <Card.Content className="p-4 sm:p-5 flex flex-col gap-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="min-w-0 flex-[1_1_14rem]">
                  <div className="text-sm font-semibold text-foreground">
                    {pendingStatus === "done"
                      ? t("download_flow.install.download_ready")
                      : pendingStatus === "error"
                        ? t("download_flow.failed")
                        : pendingStatus === "cancelled"
                          ? t("download_flow.status.cancelled")
                          : t("download_flow.install.downloading")}
                  </div>
                  <div className="text-xs text-muted truncate" title={pendingTask?.fileName || pendingDownload}>
                    {pendingTask?.fileName || pendingDownload}
                  </div>
                </div>
                {(pendingStatus === "error" || pendingStatus === "cancelled") && pendingTask?.url && (
                  <Button size="sm" variant="primary" onPress={retryPendingDownload}>
                    <FaRedo size={12} />
                    {t("download_manager.actions.retry")}
                  </Button>
                )}
                {isDownloadActive(pendingStatus) && pendingTask?.progress && pendingTask.progress.total > 0 && (
                  <span className="text-xs font-mono text-muted">
                    {formatBytes(pendingTask.progress.downloaded)} / {formatBytes(pendingTask.progress.total)}
                    {pendingStatus !== "verifying" && ` · ${formatSpeed(pendingTask.speed)}`}
                  </span>
                )}
              </div>
              <ProgressBar
                aria-label={t("audit.primary.download.progress", { name: pendingTask?.fileName || pendingDownload })}
                value={
                  pendingStatus === "done"
                    ? 100
                    : pendingTask?.progress && pendingTask.progress.total > 0
                      ? (pendingTask.progress.downloaded / pendingTask.progress.total) * 100
                      : 0
                }
                size="sm"
                isIndeterminate={
                  pendingStatus === "verifying" ||
                  (isDownloadActive(pendingStatus) && !(pendingTask?.progress && pendingTask.progress.total > 0))
                }
              >
                <ProgressBar.Track>
                  <ProgressBar.Fill className={pendingStatus === "error" ? "bg-danger" : "bg-brand-500"} />
                </ProgressBar.Track>
              </ProgressBar>
              {pendingTask?.error ? (
                <p role="alert" className="text-sm text-danger break-words select-text">
                  {downloadErrorText(pendingTask.error, t)}
                </p>
              ) : awaitingDownload ? (
                <p className="text-xs text-muted">
                  {t(installQueued ? "download_flow.install.queued_hint" : "download_flow.install.background_hint")}
                </p>
              ) : null}
            </Card.Content>
          </Card>
        )}

        {/* Content Area */}
        <AnimatePresence mode="wait">
          {installing ? (
            <motion.div
              data-material-motion
              key="installing"
              initial={{ opacity: 0, scale: 0.95 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.95 }}
            >
              <Card className={LAYOUT.GLASS_CARD.BASE}>
                <Card.Content className="py-[clamp(1.5rem,5vh,3rem)] px-4 sm:px-6">
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

                    <div className="w-full max-w-2xl flex flex-col gap-2">
                      {installingVersion && (
                        <div className="flex items-center justify-between gap-2 px-3 py-2 rounded-xl bg-surface-secondary/50 ">
                          <span className="text-sm font-medium text-muted dark:text-zinc-400">
                            {t("downloadpage.install.version_label")}
                          </span>
                          <span className="text-sm font-bold text-foreground dark:text-zinc-300 truncate">
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

                      {installingLL ? (
                        <LipTaskProgress className="mt-1 px-1" />
                      ) : (
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
                            <div className="flex justify-between gap-2 text-xs text-muted dark:text-zinc-400 font-medium">
                              <span>
                                {extractInfo.totalBytes
                                  ? `${formatBytes(extractInfo.bytes)} / ${formatBytes(extractInfo.totalBytes)}`
                                  : t("downloadpage.install.estimated_size")}
                              </span>
                              <span className="font-mono">
                                {extractInfo.totalBytes
                                  ? `${((extractInfo.bytes / extractInfo.totalBytes) * 100).toFixed(1)}%`
                                  : formatBytes(extractInfo.bytes)}
                              </span>
                            </div>
                          ) : null}
                        </div>
                      )}

                      <InstallStepLog steps={steps} className="mt-2" />
                    </div>
                  </div>
                </Card.Content>
              </Card>
            </motion.div>
          ) : resultMsg ? (
            <motion.div
              data-material-motion
              key="success"
              initial={{ opacity: 0, scale: 0.95 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.95 }}
            >
              <Card className={LAYOUT.GLASS_CARD.BASE}>
                <Card.Content className="py-[clamp(1.5rem,5vh,3rem)] px-4 sm:px-6">
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
                      <p className="text-muted dark:text-zinc-400 text-sm mt-2 max-w-md">
                        {resultMsg}
                      </p>
                    </div>

                    {/* The user picks where to go next; nothing navigates on its own. */}
                    <div className="flex flex-wrap justify-center gap-2">
                      <Button
                        onPress={launchNow}
                        isPending={launching}
                        variant={"primary"}
                        className={cn(
                          "rounded-full",
                          "bg-brand-500 hover:bg-brand-500 brand-primary-foreground font-bold shadow-lg shadow-brand-900/20",
                        )}
                      >
                        {({ isPending }) => (
                          <>
                            {isPending && <Spinner size="sm" color="current" />}
                            {t("download_flow.launch.now")}
                          </>
                        )}
                      </Button>
                      <Button variant="secondary" className="rounded-full" onPress={goToInstance}>
                        {t("download_flow.go_to_instance")}
                      </Button>
                      <Button variant="ghost" className="rounded-full" onPress={() => navigate(returnTo)}>
                        {backLabel}
                      </Button>
                    </div>

                    <div className="w-full max-w-2xl flex flex-col gap-2">
                      <div className="flex flex-wrap items-center justify-between gap-2 rounded-xl bg-surface-secondary/50 border border-border/50 dark:border-white/5 px-3 py-2">
                        <div className="min-w-0 text-sm">
                          <span className="text-muted">{t("download_flow.install.instance")}</span>{" "}
                          <span className="font-semibold text-foreground break-all">{installedFolderName}</span>
                        </div>
                        <div className="flex flex-wrap items-center gap-2">
                          {madeCurrent ? (
                            <Chip size="sm" variant="soft" color="success">
                              <FaCheckCircle />
                              <Chip.Label>{t("download_flow.install.is_current")}</Chip.Label>
                            </Chip>
                          ) : (
                            <Button size="sm" variant="secondary" onPress={makeCurrent}>
                              {t("download_flow.install.set_current_action")}
                            </Button>
                          )}
                          <Button size="sm" variant="ghost" onPress={handleOpenFolder}>
                            {t("common.open_folder")}
                          </Button>
                        </div>
                      </div>
                      {installingTargetName && (
                        <div className="rounded-xl bg-surface-secondary/50 border border-border/50 dark:border-white/5 p-3 flex flex-col gap-1">
                          <span className="text-[10px] uppercase tracking-wider text-muted font-bold">
                            {t("downloadpage.install.target")}
                          </span>
                          <span className="font-mono text-xs text-foreground dark:text-zinc-400 truncate w-full">
                            {installingTargetName}
                          </span>
                        </div>
                      )}
                      <InstallStepLog steps={steps} />
                    </div>
                  </div>
                </Card.Content>
              </Card>
            </motion.div>
          ) : (
            <motion.div
              data-material-motion
              key="input"
              className="flex flex-col gap-4"
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -10 }}
            >
              {failureDetails && (
                <Card className={cn(LAYOUT.GLASS_CARD.BASE, "border-danger/30")}>
                  <Card.Content className="space-y-3 p-4 sm:p-5">
                    <div role="alert" className="space-y-2">
                      <h3 className="font-semibold text-danger">{t("audit.primary.install.failed")}</h3>
                      <p className="text-sm text-foreground">{resolveInstallError(failureDetails, t, typeLabel)}</p>
                      <pre className="whitespace-pre-wrap break-all text-xs text-muted select-text">{failureDetails}</pre>
                    </div>
                    <div className="flex flex-wrap gap-2">
                      <Button variant="secondary" size="sm" onPress={async () => {
                        try {
                          await Clipboard.SetText(failureDetails);
                          notify({ variant: "success", source: "install", title: t("audit.primary.install.copied") });
                        } catch {
                          notify({ variant: "danger", source: "install", title: t("audit.primary.install.copy_failed") });
                        }
                      }}>{t("audit.primary.install.copy_details")}</Button>
                      <Button variant="primary" size="sm" onPress={handleInstall}>{t("download_manager.actions.retry")}</Button>
                    </div>
                    <InstallStepLog steps={steps} />
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
                  <div className="mt-4 flex flex-wrap items-center justify-between gap-3 p-3 rounded-2xl bg-surface/50 dark:bg-surface-secondary/10 border border-border dark:border-white/5">
                    <div className="min-w-0 flex-[1_1_14rem]">
                      <div className="text-sm font-medium">
                        {t("download_flow.install.set_current_label")}
                      </div>
                      <div className="text-xs text-muted dark:text-zinc-400">
                        {t("download_flow.install.set_current_desc")}
                      </div>
                    </div>
                    <Switch
                      aria-label={t("download_flow.install.set_current_label")}
                      isSelected={setAsCurrent}
                      onChange={setSetAsCurrent}
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
                  {isUWP && <p className="text-sm text-muted">{t("uwp.install_description")}</p>}
                  {!downloadResolved && !pendingTask && (
                    <div className="flex flex-wrap items-center justify-between gap-3 p-3 rounded-2xl bg-surface/50 dark:bg-surface-secondary/10 border border-border dark:border-white/5">
                      <div className="min-w-0 flex-[1_1_14rem] break-all">
                        <div className="text-sm font-medium">
                          {t("uwp.local_installer")}
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
                                  Pattern: "*.msixvc;*.appx;*.msix;*.appxbundle;*.msixbundle",
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
                    <div className="flex items-center justify-between gap-3 p-3 rounded-2xl bg-surface/50 dark:bg-surface-secondary/10 border border-border dark:border-white/5">
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
                  <div className="flex items-center justify-between gap-3 p-3 rounded-2xl bg-surface/50 dark:bg-surface-secondary/10 border border-border dark:border-white/5">
                    <div className="min-w-0">
                      <div className="text-sm font-medium">
                        {t("downloadpage.install_folder.enable_isolation")}
                      </div>
                      <div className="text-xs text-muted dark:text-zinc-400">
                        {t("downloadpage.install_folder.enable_isolation_desc")}
                      </div>
                      {isUWP && <p className="text-xs text-muted">
                        {t("uwp.isolation_min_version", { version: UWP_ISOLATION_MIN_VERSION })}
                      </p>}
                    </div>
                    <Switch
                      aria-label={t("downloadpage.install_folder.enable_isolation")}
                      isSelected={isolationSupported && installIsolation}
                      isDisabled={!isolationSupported}
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
                  {!isUWP && installIsolation && (
                    <div className="flex flex-wrap items-center justify-between gap-3 p-3 rounded-2xl bg-surface/50 dark:bg-surface-secondary/10 border border-border dark:border-white/5">
                      <div className="min-w-0 flex-[1_1_14rem]">
                        <div className="text-sm font-medium">
                          {t("downloadpage.install_folder.inherit_label")}
                        </div>
                        <div className="text-xs text-muted dark:text-zinc-400">
                          {t("downloadpage.install_folder.inherit_hint")}
                        </div>
                      </div>
                      <div className="w-full max-w-full min-[480px]:w-60">

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
                              className="max-h-64 overflow-y-auto min-w-[min(15rem,90vw)] no-scrollbar"
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
