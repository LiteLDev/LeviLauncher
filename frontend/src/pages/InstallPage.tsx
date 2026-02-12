import React, { useEffect, useMemo, useState } from "react";
import {
  Button,
  Input,
  Switch,
  Dropdown,
  DropdownTrigger,
  DropdownMenu,
  DropdownItem,
  Card,
  CardBody,
  CardHeader,
  Chip,
  Spinner,
  useDisclosure,
  addToast,
} from "@heroui/react";
import { useLocation, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { FaChevronDown } from "react-icons/fa";
import { useVersionStatus } from "@/utils/VersionStatusContext";
import { useLeviLamina } from "@/utils/LeviLaminaContext";
import { resolveInstallError } from "@/utils/installError";
import { motion, AnimatePresence } from "framer-motion";
import { Dialogs, Events } from "@wailsio/runtime";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { UnifiedModal } from "@/components/UnifiedModal";
import { PageHeader } from "@/components/PageHeader";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { cn } from "@/utils/cn";

type ItemType = "Preview" | "Release";

export default function InstallPage() {
  const { t } = useTranslation();
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
  const returnTo: string = String(location?.state?.returnTo || "/download");
  const isLeviLaminaSupported = Boolean(location?.state?.isLeviLaminaSupported);

  const [installName, setInstallName] = useState<string>(mirrorVersion || "");
  const [installIsolation, setInstallIsolation] = useState<boolean>(true);
  const [installLeviLamina, setInstallLeviLamina] = useState<boolean>(false);
  const [inheritSource, setInheritSource] = useState<string>("");
  const [inheritMetas, setInheritMetas] = useState<any[]>([]);
  const [inheritCandidates, setInheritCandidates] = useState<string[]>([]);
  const [installError, setInstallError] = useState<string>("");
  const [installing, setInstalling] = useState<boolean>(false);
  const [installingVersion, setInstallingVersion] = useState<string>("");
  const [installingTargetName, setInstallingTargetName] = useState<string>("");
  const [installedFolderName, setInstalledFolderName] = useState<string>("");
  const [resultMsg, setResultMsg] = useState<string>("");
  const [customInstallerPath, setCustomInstallerPath] = useState<string>("");
  const [installerDir, setInstallerDir] = useState<string>("");
  const [downloadResolved, setDownloadResolved] = useState<boolean>(false);
  const { llMap } = useLeviLamina();
  const {
    isOpen: rcOpen,
    onOpen: rcOnOpen,
    onOpenChange: rcOnOpenChange,
    onClose: rcOnClose,
  } = useDisclosure();
  const [rcVersion, setRcVersion] = useState("");
  const [extractInfo, setExtractInfo] = useState<{
    files: number;
    bytes: number;
    dir: string;
    totalBytes?: number;
    currentFile?: string;
  } | null>(null);

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
    try {
      (window as any).llNavLock = guardActive;
      window.dispatchEvent(
        new CustomEvent("ll-nav-lock-changed", {
          detail: { lock: guardActive },
        }),
      );
    } catch {}
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
      try {
        (window as any).llNavLock = false;
        window.dispatchEvent(
          new CustomEvent("ll-nav-lock-changed", { detail: { lock: false } }),
        );
      } catch {}
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
        const validate = minecraft?.ValidateVersionFolderName;
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
      const list = minecraft?.ListVersionMetas;
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
      const listInh = minecraft?.ListInheritableVersionNames;
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
      const getDir = minecraft?.GetInstallerDir;
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

  const headerTitle = useMemo(() => {
    if (installing)
      return t("downloadmodal.installing.title") as unknown as string;
    if (resultMsg)
      return t("downloadpage.install.success_title") as unknown as string;
    return t("downloadpage.install_folder.confirm_title") as unknown as string;
  }, [installing, resultMsg, t]);

  const proceedInstall = async () => {
    setInstallError("");
    setResultMsg("");

    if (installLeviLamina) {
      try {
        const lipInstalled = await minecraft.IsLipInstalled();
        if (!lipInstalled) {
          addToast({
            title: t("common.error"),
            description: resolveInstallError("ERR_LIP_NOT_INSTALLED", t),
            color: "danger",
          });
          return;
        }
      } catch {}
    }

    const name = (installName || "").trim();
    let installationCreated = false;
    const rollback = async () => {
      if (!installationCreated) return;
      try {
        const del = (minecraft as any)?.DeleteVersionFolder;
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
      const validate = minecraft?.ValidateVersionFolderName;
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
      const saveMeta = minecraft?.SaveVersionMeta;
      const copyFromGDK = minecraft?.CopyVersionDataFromGDK;
      const copyFromVersion = minecraft?.CopyVersionDataFromVersion;
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
        addToast({
          title: t("common.error"),
          description: resolveInstallError("ERR_MSIXVC_NOT_SPECIFIED", t),
          color: "danger",
        });
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
          addToast({
            title: t("common.error"),
            description: resolveInstallError(err, t),
            color: "danger",
          });
          setInstalling(false);
          return;
        }
        installationCreated = true;
      }

      if (typeof saveMeta === "function") {
        await saveMeta(
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
            addToast({
              title: t("common.error"),
              description: resolveInstallError(copyErr, t),
              color: "danger",
            });
            setInstalling(false);
            await rollback();
            return;
          }
        } catch (e: any) {
          addToast({
            title: t("common.error"),
            description: resolveInstallError(
              String(e?.message || e || ""),
              t,
              typeLabel,
            ),
            color: "danger",
          });
          setInstalling(false);
          await rollback();
          return;
        }
      }

      if (installLeviLamina) {
        try {
          const installLL = (minecraft as any)?.InstallLeviLamina;
          if (typeof installLL === "function") {
            const llErr: string = await installLL(
              mirrorVersion || installName || "",
              name,
            );
            if (llErr) {
              addToast({
                title: t("common.error"),
                description: resolveInstallError(llErr, t),
                color: "danger",
              });
              setInstalling(false);
              await rollback();
              return;
            }
          }
        } catch (e: any) {
          addToast({
            title: t("common.error"),
            description: resolveInstallError(
              String(e?.message || e || ""),
              t,
              typeLabel,
            ),
            color: "danger",
          });
          setInstalling(false);
          await rollback();
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
      addToast({
        title: t("common.error"),
        description: resolveInstallError(
          String(e?.message || e || ""),
          t,
          typeLabel,
        ),
        color: "danger",
      });
      setInstalling(false);
      await rollback();
    }
  };

  const handleInstall = async () => {
    if (installLeviLamina && mirrorVersion) {
      let targetLLVersion = "";
      if (llMap && llMap.size > 0) {
        let versions = llMap.get(mirrorVersion);
        if (!versions && mirrorVersion.split(".").length >= 3) {
          const parts = mirrorVersion.split(".");
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
    }
    await proceedInstall();
  };

  const handleOpenFolder = async () => {
    if (!installedFolderName) return;
    try {
      const vdir = await minecraft.GetVersionsDir();
      const sep = vdir.includes("\\") ? "\\" : "/";
      const path = `${vdir}${sep}${installedFolderName}`;
      await minecraft.OpenPathDir(path);
    } catch (e) {
      console.error(e);
    }
  };

  return (
    <PageContainer>
      <div className="flex flex-col gap-6 w-full">
        {/* Header Card */}
        <motion.div
          initial={{ opacity: 0, y: -20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.4 }}
        >
          <Card className={LAYOUT.GLASS_CARD.BASE}>
            <CardBody className="p-6 flex flex-row items-center justify-between gap-4">
              <div className="flex-1">
                <PageHeader
                  title={headerTitle}
                  description={
                    <div className="flex items-center gap-2">
                      <Chip
                        size="sm"
                        variant="flat"
                        color={mirrorType === "Preview" ? "warning" : "primary"}
                      >
                        {mirrorType === "Preview"
                          ? `${t("common.preview")} Minecraft`
                          : `${t("common.release")} Minecraft`}
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
                        variant="light"
                        onPress={() => navigate(returnTo)}
                      >
                        {t("common.cancel")}
                      </Button>
                      <Button
                        className="bg-primary-600 hover:bg-primary-500 text-white font-bold shadow-lg shadow-primary-900/20"
                        radius="full"
                        onPress={handleInstall}
                      >
                        {t(
                          "downloadpage.customappx.modal.1.footer.install_button",
                        )}
                      </Button>
                    </>
                  ) : (
                    <div className="flex gap-2">
                      <Button
                        variant="flat"
                        onPress={handleOpenFolder}
                        className="bg-default-100 dark:bg-zinc-800 text-default-700 dark:text-zinc-300 font-medium"
                        radius="full"
                      >
                        {t("common.open_folder")}
                      </Button>
                      <Button
                        className="bg-primary-600 hover:bg-primary-500 text-white font-bold shadow-lg shadow-primary-900/20"
                        radius="full"
                        onPress={() => navigate(returnTo)}
                      >
                        {t("common.back")}
                      </Button>
                    </div>
                  )}
                </div>
              )}
            </CardBody>
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
                <CardBody className="py-12">
                  <div className="flex flex-col items-center justify-center h-full gap-4">
                    <div className="relative flex items-center justify-center">
                      <div className="absolute inset-0 bg-primary-500/20 blur-xl rounded-full animate-pulse" />
                      <div className="w-16 h-16 rounded-full bg-default-50 dark:bg-zinc-800 border-4 border-default-100 dark:border-zinc-700 flex items-center justify-center relative z-10">
                        <Spinner
                          size="md"
                          color="success"
                          classNames={{ wrapper: "w-8 h-8" }}
                        />
                      </div>
                    </div>

                    <div className="flex flex-col items-center gap-1 text-center max-w-sm">
                      <h2 className="text-xl font-bold text-default-900 dark:text-white">
                        {t("downloadmodal.installing.title")}
                      </h2>
                      <p className="text-small text-default-500 dark:text-zinc-400">
                        {t("downloadpage.install.hint")}
                      </p>
                    </div>

                    <div className="w-full max-w-lg flex flex-col gap-2">
                      {installingVersion && (
                        <div className="flex items-center justify-between px-3 py-2 rounded-xl bg-default-100/50 dark:bg-zinc-800/50">
                          <span className="text-small font-medium text-default-500 dark:text-zinc-400">
                            {t("downloadpage.install.version_label")}
                          </span>
                          <span className="text-small font-bold text-default-700 dark:text-zinc-300">
                            {installingVersion}
                          </span>
                        </div>
                      )}

                      {installingTargetName && (
                        <div className="flex flex-col gap-1 px-3 py-2 rounded-xl bg-default-100/50 dark:bg-zinc-800/50">
                          <span className="text-[10px] font-bold uppercase tracking-wider text-default-400">
                            {t("downloadpage.install.target")}
                          </span>
                          <span className="font-mono text-xs text-default-600 dark:text-zinc-400 truncate">
                            {installingTargetName}
                          </span>
                        </div>
                      )}

                      <div className="mt-1 flex flex-col gap-2">
                        <div className="h-1.5 w-full rounded-full bg-default-200/50 dark:bg-zinc-700/50 overflow-hidden border border-default-100 dark:border-white/5 relative">
                          {extractInfo?.totalBytes ? (
                            <motion.div
                              className="h-full bg-gradient-to-r from-primary-500 to-teal-500 rounded-full"
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
                          <div className="flex justify-between text-tiny text-default-500 dark:text-zinc-400 font-medium">
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
                </CardBody>
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
                <CardBody className="py-12">
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
                        className="w-16 h-16 rounded-full bg-linear-to-br from-primary-400 to-teal-600 flex items-center justify-center shadow-lg shadow-primary-900/20"
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
                      <h2 className="text-2xl font-black bg-linear-to-br from-primary-600 to-teal-600 dark:from-primary-500 dark:to-teal-500 bg-clip-text text-transparent">
                        {t("downloadpage.install.success_title")}
                      </h2>
                      {installingVersion && (
                        <Chip
                          variant="flat"
                          color="success"
                          size="sm"
                          classNames={{ content: "font-bold" }}
                        >
                          {installingVersion}
                        </Chip>
                      )}
                      <p className="text-default-500 dark:text-zinc-400 text-sm mt-2 max-w-xs">
                        {t("downloadpage.install.success")}
                      </p>
                    </div>

                    <div className="w-full max-w-lg mt-1">
                      {installingTargetName && (
                        <div className="rounded-xl bg-default-100/50 dark:bg-zinc-800/50 border border-default-200/50 dark:border-white/5 p-3 flex flex-col gap-1 items-center">
                          <span className="text-[10px] uppercase tracking-wider text-default-400 font-bold">
                            {t("downloadpage.install.target")}
                          </span>
                          <span className="font-mono text-xs text-default-600 dark:text-zinc-400 truncate w-full text-center">
                            {installingTargetName}
                          </span>
                        </div>
                      )}
                    </div>
                  </div>
                </CardBody>
              </Card>
            </motion.div>
          ) : (
            <motion.div
              key="input"
              className="flex flex-col gap-6"
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -10 }}
            >
              {/* Removed global error card */}

              {/* Basic Configuration */}
              <Card className={LAYOUT.GLASS_CARD.BASE}>
                <CardHeader className={LAYOUT.GLASS_CARD.HEADER}>
                  <h3 className="text-large font-medium">
                    {t("settings.tabs.general")}
                  </h3>
                </CardHeader>
                <CardBody className="p-6">
                  <Input
                    label={
                      t(
                        "downloadpage.install_folder.name_label",
                      ) as unknown as string
                    }
                    placeholder={
                      t(
                        "downloadpage.install_folder.placeholder_name",
                      ) as unknown as string
                    }
                    value={installName}
                    onValueChange={setInstallName}
                    isInvalid={!!installError}
                    errorMessage={
                      installError
                        ? resolveInstallError(installError, t, typeLabel)
                        : undefined
                    }
                    variant="bordered"
                    size="sm"
                  />
                </CardBody>
              </Card>

              {/* Advanced Options */}
              <Card className={LAYOUT.GLASS_CARD.BASE}>
                <CardHeader className={LAYOUT.GLASS_CARD.HEADER}>
                  <h3 className="text-large font-medium">
                    {t("settings.tabs.others")}
                  </h3>
                </CardHeader>
                <CardBody className="p-6 flex flex-col gap-4">
                  {!downloadResolved && (
                    <div className="flex items-center justify-between p-3 rounded-2xl bg-default-50/50 dark:bg-default-100/10 border border-default-100 dark:border-white/5">
                      <div className="min-w-0">
                        <div className="text-small font-medium">
                          {t("downloadpage.install.custom_installer.label")}
                        </div>
                        <div className="text-tiny text-default-500 dark:text-zinc-400">
                          {customInstallerPath
                            ? customInstallerPath
                            : (t(
                                "downloadpage.install.custom_installer.hint",
                              ) as unknown as string)}
                        </div>
                      </div>
                      <Button
                        variant="flat"
                        size="sm"
                        className="bg-default-100 dark:bg-white/10"
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
                      >
                        {t("common.browse")}
                      </Button>
                    </div>
                  )}

                  {isLeviLaminaSupported && (
                    <div className="flex items-center justify-between p-3 rounded-2xl bg-default-50/50 dark:bg-default-100/10 border border-default-100 dark:border-white/5">
                      <div className="min-w-0">
                        <div className="text-small font-medium">
                          {t("downloadpage.install.levilamina_label")}
                        </div>
                        <div className="text-tiny text-default-500 dark:text-zinc-400">
                          {t("downloadpage.install.levilamina_desc")}
                        </div>
                      </div>
                      <Switch
                        isSelected={installLeviLamina}
                        onValueChange={setInstallLeviLamina}
                        color="success"
                      />
                    </div>
                  )}

                  <div className="flex items-center justify-between p-3 rounded-2xl bg-default-50/50 dark:bg-default-100/10 border border-default-100 dark:border-white/5">
                    <div className="min-w-0">
                      <div className="text-small font-medium">
                        {t("downloadpage.install_folder.enable_isolation")}
                      </div>
                      <div className="text-tiny text-default-500 dark:text-zinc-400">
                        {t("downloadpage.install_folder.enable_isolation_desc")}
                      </div>
                    </div>
                    <Switch
                      isSelected={installIsolation}
                      onValueChange={setInstallIsolation}
                      color="success"
                    />
                  </div>

                  {installIsolation && (
                    <div className="flex items-center justify-between p-3 rounded-2xl bg-default-50/50 dark:bg-default-100/10 border border-default-100 dark:border-white/5">
                      <div className="min-w-0">
                        <div className="text-small font-medium">
                          {t("downloadpage.install_folder.inherit_label")}
                        </div>
                        <div className="text-tiny text-default-500 dark:text-zinc-400">
                          {t("downloadpage.install_folder.inherit_hint")}
                        </div>
                      </div>
                      <div className="shrink-0 min-w-[240px]">
                        <Dropdown closeOnSelect>
                          <DropdownTrigger>
                            <Button
                              variant="flat"
                              size="sm"
                              className="bg-default-100 dark:bg-white/10 w-full justify-between"
                              endContent={<FaChevronDown size={12} />}
                            >
                              {inheritLabel}
                            </Button>
                          </DropdownTrigger>
                          <DropdownMenu
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
                              <DropdownItem key={item.key}>
                                {item.label}
                              </DropdownItem>
                            )}
                          </DropdownMenu>
                        </Dropdown>
                      </div>
                    </div>
                  )}
                </CardBody>
              </Card>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
      <UnifiedModal
        size="md"
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
