import { toast } from "@heroui/react";
import React, {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
} from "react";
import { Events } from "@wailsio/runtime";
import { useNavigate } from "react-router-dom";
import { ROUTES } from "@/constants/routes";
import { UnifiedModal } from "@/components/UnifiedModal";
import { ModalDescription } from "@/components/ModalPrimitives";
import { resolveInstallError } from "@/utils/installError";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";

import { useTranslation } from "react-i18next";

export interface DownloadItem {
  dest: string;
  progress: {
    downloaded: number;
    total: number;
    dest?: string;
  } | null;
  speed: number;
  status: string;
  error: string;
  fileName: string;
  url?: string;
  md5sum?: string;
  installer?: {
    version: string;
    type: string;
    packageType?: "gdk" | "uwp";
    uuid?: string;
    isLeviLaminaSupported: boolean;
    // Set when the download was started to upgrade this existing instance;
    // the finished download then offers the upgrade instead of a new install.
    upgradeTarget?: string;
  };
  // Set while the download waits for a free slot (status "queued").
  queuedAt?: number;
}

// At most this many downloads transfer at once; later ones wait in a queue
// and start by themselves as slots free up.
export const MAX_CONCURRENT_DOWNLOADS = 2;

export const isDownloadActive = (status: string): boolean =>
  ["starting", "started", "resumed", "verifying"].includes(status);

export const isDownloadTerminal = (status: string): boolean =>
  ["done", "cancelled", "error"].includes(status);

// Pending downloads have not finished yet: running, paused or queued.
export const isDownloadPending = (status: string): boolean =>
  isDownloadActive(status) || status === "paused" || status === "queued";

interface DownloadsContextType {
  downloads: DownloadItem[];
  startDownload: (
    url: string,
    filename?: string,
    md5sum?: string,
    installer?: DownloadItem["installer"],
  ) => Promise<boolean>;
  cancelDownload: (dest?: string) => void;
  pauseDownload: (dest: string) => void;
  resumeDownload: (dest: string) => void;
  // Suppresses the global "finished, install now?" prompt for a download
  // whose page already offers the next step.
  dismissPrompt: (dest: string) => void;
  // Names the instance whose upgrade dialog is open ("" when none); its
  // finished downloads are offered by that dialog, not the global prompt.
  setUpgradeDialogTarget: (name: string) => void;
  removeDownload: (dest: string) => void;
  clearError: (dest?: string) => void;
  isDownloading: boolean;
}

const DownloadsContext = createContext<DownloadsContextType | null>(null);

export const useDownloads = () => {
  const context = useContext(DownloadsContext);
  if (!context) {
    return {
      downloads: [],
      startDownload: () => Promise.resolve(false),
      cancelDownload: () => {},
      pauseDownload: () => {},
      resumeDownload: () => {},
      dismissPrompt: () => {},
      setUpgradeDialogTarget: () => {},
      removeDownload: () => {},
      clearError: () => {},
      isDownloading: false,
    };
  }
  return context;
};

export const DownloadsProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [dismissedInstallPrompts, setDismissedInstallPrompts] = useState<Set<string>>(
    () => new Set(),
  );
  const [upgradeDialogTarget, setUpgradeDialogTarget] = useState("");
  const [downloadsMap, setDownloadsMap] = useState<
    Record<string, DownloadItem>
  >({});
  const speedRef = useRef<Record<string, { ts: number; bytes: number }>>({});
  const downloadsRef = useRef<Record<string, DownloadItem>>({});
  const cancelledRef = useRef<Set<string>>(new Set());
  const startingRef = useRef<Set<string>>(new Set());

  const getFileNameFromDest = (dest: string) => {
    if (!dest || typeof dest !== "string") return "";
    return dest.split(/[\\/]/).pop() || "";
  };

  const updateDownload = (
    dest: string,
    updater: (current: DownloadItem) => DownloadItem,
  ) => {
    if (!dest) return;
    setDownloadsMap((prev) => {
      const existing = prev[dest];
      const base: DownloadItem = existing || {
        dest,
        progress: null,
        speed: 0,
        status: "",
        error: "",
        fileName: getFileNameFromDest(dest),
      };
      const next = updater(base);
      return { ...prev, [dest]: next };
    });
  };

  useEffect(() => {
    downloadsRef.current = downloadsMap;
  }, [downloadsMap]);

  useEffect(() => {
    const offs: (() => void)[] = [];

    offs.push(
      Events.On("msixvc_download_progress", (event) => {
        const downloaded = Number(event?.data?.Downloaded || 0);
        const total = Number(event?.data?.Total || 0);
        const dest = String(event?.data?.Dest || "");
        if (!dest || cancelledRef.current.has(dest)) return;

        updateDownload(dest, (prev) => {
          let speed = prev.speed;
          try {
            const now = Date.now();
            const prevRef = speedRef.current[dest];
            if (prevRef) {
              const dt = (now - prevRef.ts) / 1000;
              const db = downloaded - prevRef.bytes;
              if (dt > 0 && db >= 0) {
                const newSpeed = db / dt;
                if (newSpeed > 0 || (newSpeed === 0 && dt > 2)) {
                  speed = newSpeed;
                }
              }
            }
            if (!prevRef || now - prevRef.ts >= 1000) {
              speedRef.current[dest] = { ts: now, bytes: downloaded };
            }
          } catch {}

          return {
            ...prev,
            progress: { downloaded, total, dest },
            speed,
            fileName: prev.fileName || getFileNameFromDest(dest),
          };
        });
      }),
    );

    offs.push(
      Events.On("msixvc_download_status", (event) => {
        const raw = event?.data;
        const status =
          typeof raw === "string" ? String(raw) : String(raw?.Status || "");
        const dest = typeof raw === "string" ? "" : String(raw?.Dest || "");

        if (dest && cancelledRef.current.has(dest)) return;

        const applyStatus = (key: string) => {
          updateDownload(key, (prev) => {
            const next = { ...prev, status };
            if (
              status === "started" ||
              status === "resumed" ||
              status === "cancelled"
            ) {
              next.error = "";
              if (status !== "resumed") {
                speedRef.current[key] = { ts: 0, bytes: 0 };
                next.speed = 0;
              }
              if (status === "cancelled") {
                next.progress = null;
                next.speed = 0;
              }
            }
            return next;
          });
        };

        if (dest) {
          applyStatus(dest);
        } else {
          Object.keys(downloadsRef.current).forEach(applyStatus);
        }
      }),
    );

    offs.push(
      Events.On("msixvc_download_error", (event) => {
        const raw = event?.data;
        const msg =
          typeof raw === "string"
            ? String(raw)
            : String(raw?.Error || "Unknown Error");
        const dest = typeof raw === "string" ? "" : String(raw?.Dest || "");

        if (dest && cancelledRef.current.has(dest)) return;

        if (dest) {
          updateDownload(dest, (prev) => ({
            ...prev,
            error: msg,
            status: "error",
            speed: 0,
          }));
        } else {
          setDownloadsMap((prev) => {
            const next: Record<string, DownloadItem> = { ...prev };
            Object.keys(next).forEach((key) => {
              next[key] = {
                ...next[key],
                error: msg,
                status: "error",
                speed: 0,
              };
            });
            return next;
          });
        }
        toast(undefined, {
          description: msg.includes("ERR_UWP_") ? resolveInstallError(msg, t) : msg,
          variant: "danger",
          timeout: 2000,
        });
      }),
    );

    offs.push(
      Events.On("msixvc_download_done", (event) => {
        const raw = event?.data;
        const dest =
          typeof raw === "string" ? String(raw) : String(raw?.Dest || "");
        if (!dest || cancelledRef.current.has(dest)) return;
        const fname = getFileNameFromDest(dest);
        updateDownload(dest, (prev) => ({
          ...prev,
          status: "done",
          progress: {
            downloaded: prev.progress?.total || prev.progress?.downloaded || 0,
            total: prev.progress?.total || 0,
            dest,
          },
          speed: 0,
          fileName: fname || prev.fileName,
        }));
        speedRef.current[dest] = { ts: 0, bytes: 0 };
        toast(t("downloadpage.download.success_body") + " " + (fname || ""), {
          variant: "success",
          timeout: 2000,
        });
      }),
    );

    return () => {
      offs.forEach((off) => off());
    };
  }, [t]);

  const startDownload = async (
    url: string,
    filename?: string,
    md5sum?: string,
    installer?: DownloadItem["installer"],
  ): Promise<boolean> => {
    if (typeof minecraft === "undefined") return false;

    const displayName = filename
      ? filename
      : (() => {
          try {
            const u = new URL(url);
            const last = u.pathname.split("/").pop() || "";
            return last;
          } catch {
            return "";
          }
        })();

    const isAlreadyDownloading = startingRef.current.has(displayName || url) || Object.values(downloadsRef.current).some(
      (dl) =>
        (dl.fileName === displayName ||
          (dl.dest && getFileNameFromDest(dl.dest) === displayName)) &&
        isDownloadPending(dl.status),
    );

    if (isAlreadyDownloading) {
      toast(undefined, {
        description: t("downloadpage.error.already_downloading"),
        variant: "danger",
        timeout: 2000,
      });
      return false;
    }

    const activeCount =
      startingRef.current.size +
      Object.values(downloadsRef.current).filter((dl) => isDownloadActive(dl.status)).length;
    if (activeCount >= MAX_CONCURRENT_DOWNLOADS) {
      const key = `queued:${displayName || url}`;
      cancelledRef.current.delete(key);
      updateDownload(key, () => ({
        dest: key,
        progress: null,
        speed: 0,
        status: "queued",
        error: "",
        fileName: displayName,
        url,
        md5sum,
        installer,
        queuedAt: Date.now(),
      }));
      toast(t("download_manager.queue.added"), { variant: "success", timeout: 2000 });
      return true;
    }

    let urlWithFilename = url;
    if (filename) {
      try {
        const u = new URL(url);
        u.searchParams.set("filename", filename);
        urlWithFilename = u.toString();
      } catch {
        const sep = url.includes("?") ? "&" : "?";
        urlWithFilename = `${url}${sep}filename=${encodeURIComponent(filename)}`;
      }
    }
    startingRef.current.add(displayName || url);
    try {
      const downloadsBeforeStart = downloadsRef.current;
      const dest = installer?.packageType === "uwp"
        ? await minecraft.StartUWPDownload(installer.version, installer.uuid || "", installer.type.toLowerCase())
        : await minecraft.StartMsixvcDownload(urlWithFilename, md5sum || "");
      if (dest) cancelledRef.current.delete(dest);

      const key = dest || displayName || urlWithFilename;
      setDismissedInstallPrompts((prev) => {
        const next = new Set(prev);
        next.delete(key);
        return next;
      });
      updateDownload(key, (prev) => {
        // A cached/fast download can finish before the start call returns.
        const completedEarly =
          prev !== downloadsBeforeStart[key] && prev.status === "done";
        return {
          ...prev,
          dest: key,
          status: completedEarly ? "done" : "starting",
          error: "",
          progress: completedEarly ? prev.progress : null,
          speed: 0,
          fileName: displayName || prev.fileName || getFileNameFromDest(key),
          url: url,
          md5sum,
          installer,
        };
      });
      toast(t("downloadpage.mirror.download_started"), {
        variant: "success",
        timeout: 2000,
      });
      return true;
    } catch (e) {
      toast(undefined, {
        description: String(e).includes("ERR_UWP_") ? resolveInstallError(String(e), t) : String(e),
        variant: "danger",
        timeout: 2000,
      });
      return false;
    } finally {
      startingRef.current.delete(displayName || url);
    }
  };

  const cancelDownload = (dest?: string) => {
    if (typeof minecraft === "undefined") return;

    if (dest && downloadsRef.current[dest]?.status === "queued") {
      setDownloadsMap((prev) => {
        const next = { ...prev };
        delete next[dest];
        return next;
      });
      return;
    }

    if (dest) {
      if (typeof minecraft.CancelMsixvcDownloadTask === "function") {
        minecraft.CancelMsixvcDownloadTask(dest);
      }

      updateDownload(dest, (prev) => ({
        ...prev,
        status: "cancelled",
        error: "",
        speed: 0,
      }));
      return;
    }

    minecraft.CancelMsixvcDownload();
    setDownloadsMap((prev) => {
      const next = { ...prev };
      Object.keys(next).forEach((key) => {
        if (next[key].status === "queued") {
          delete next[key];
          return;
        }
        if (
          next[key].status === "started" ||
          next[key].status === "starting" ||
          next[key].status === "resumed" ||
          next[key].status === "paused"
        ) {
          next[key] = {
            ...next[key],
            status: "cancelled",
            error: "",
            speed: 0,
          };
        }
      });
      return next;
    });
  };

  const pauseDownload = (dest: string) => {
    const status = downloadsRef.current[dest]?.status || "";
    if (!["started", "resumed"].includes(status)) return;
    if (typeof minecraft?.PauseMsixvcDownloadTask === "function") {
      minecraft.PauseMsixvcDownloadTask(dest);
    }
  };

  const resumeDownload = (dest: string) => {
    if (downloadsRef.current[dest]?.status !== "paused") return;
    if (typeof minecraft?.ResumeMsixvcDownloadTask === "function") {
      minecraft.ResumeMsixvcDownloadTask(dest);
    }
  };

  // Start queued downloads, oldest first, whenever a slot is free.
  const pumpingRef = useRef(false);
  useEffect(() => {
    if (pumpingRef.current) return;
    const all = Object.values(downloadsMap);
    const running =
      startingRef.current.size + all.filter((dl) => isDownloadActive(dl.status)).length;
    if (running >= MAX_CONCURRENT_DOWNLOADS) return;
    const next = all
      .filter((dl) => dl.status === "queued")
      .sort((a, b) => (a.queuedAt || 0) - (b.queuedAt || 0))[0];
    if (!next || !next.url) return;
    pumpingRef.current = true;
    setDownloadsMap((prev) => {
      const copy = { ...prev };
      delete copy[next.dest];
      return copy;
    });
    downloadsRef.current = { ...downloadsRef.current };
    delete downloadsRef.current[next.dest];
    void startDownload(next.url, next.fileName || undefined, next.md5sum, next.installer)
      .finally(() => {
        pumpingRef.current = false;
      });
  }, [downloadsMap]);

  const removeDownload = (dest: string) => {
    if (!isDownloadTerminal(downloadsRef.current[dest]?.status || "")) return;
    cancelledRef.current.add(dest);
    setDownloadsMap((prev) => {
      const next = { ...prev };
      delete next[dest];
      return next;
    });
  };

  const clearError = (dest?: string) => {
    if (dest) {
      setDownloadsMap((prev) => {
        const next = { ...prev };
        if (
          next[dest]?.status === "done" ||
          next[dest]?.status === "cancelled" ||
          next[dest]?.status === "error"
        ) {
          delete next[dest];
        } else {
          next[dest] = { ...next[dest], error: "" };
        }
        return next;
      });
      return;
    }
    setDownloadsMap((prev) => {
      const next: Record<string, DownloadItem> = { ...prev };
      Object.keys(next).forEach((key) => {
        if (
          next[key]?.status === "done" ||
          next[key]?.status === "cancelled" ||
          next[key]?.status === "error"
        ) {
          delete next[key];
        } else {
          next[key] = { ...next[key], error: "" };
        }
      });
      return next;
    });
  };

  const downloads = Object.values(downloadsMap);
  const isDownloading = downloads.some((dl) => isDownloadActive(dl.status));
  const installPrompt = downloads.find((dl) =>
    dl.status === "done" && dl.installer && !dismissedInstallPrompts.has(dl.dest) &&
    !(dl.installer.upgradeTarget && dl.installer.upgradeTarget === upgradeDialogTarget),
  );
  const dismissInstallPrompt = () => {
    if (!installPrompt) return;
    setDismissedInstallPrompts((prev) => new Set(prev).add(installPrompt.dest));
  };

  return (
    <DownloadsContext.Provider
      value={{
        downloads,
        startDownload,
        cancelDownload,
        pauseDownload,
        resumeDownload,
        dismissPrompt: (dest: string) =>
          setDismissedInstallPrompts((prev) => (prev.has(dest) ? prev : new Set(prev).add(dest))),
        setUpgradeDialogTarget,
        removeDownload,
        clearError,
        isDownloading,
      }}
    >
      {children}
      <UnifiedModal
        isOpen={!!installPrompt}
        onOpenChange={(open) => { if (!open) dismissInstallPrompt(); }}
        type="success"
        title={t(installPrompt?.installer?.upgradeTarget
          ? "download_manager.upgrade_prompt.title"
          : "download_manager.install_prompt.title")}
        confirmText={t(installPrompt?.installer?.upgradeTarget
          ? "download_manager.upgrade_prompt.confirm"
          : "download_manager.install_prompt.confirm")}
        cancelText={t("download_manager.install_prompt.later")}
        showCancelButton
        onCancel={dismissInstallPrompt}
        onConfirm={() => {
          if (!installPrompt?.installer) return;
          dismissInstallPrompt();
          if (installPrompt.installer.upgradeTarget) {
            navigate(ROUTES.instanceSettings, { state: {
              name: installPrompt.installer.upgradeTarget,
              tab: "manage",
              upgradeInstaller: {
                version: installPrompt.installer.version,
                type: installPrompt.installer.type,
                packageType: installPrompt.installer.packageType,
                installerPath: installPrompt.dest,
              },
            } });
            return;
          }
          navigate(ROUTES.install, { state: {
            mirrorVersion: installPrompt.installer.version,
            mirrorType: installPrompt.installer.type,
            isLeviLaminaSupported: installPrompt.installer.isLeviLaminaSupported,
            packageType: installPrompt.installer.packageType,
            installerPath: installPrompt.dest,
            returnTo: ROUTES.downloadTasks,
          } });
        }}
      >
        <ModalDescription>
          {installPrompt?.installer?.upgradeTarget
            ? t("download_manager.upgrade_prompt.description", {
                name: installPrompt.installer.upgradeTarget,
                version: installPrompt.installer.version,
              })
            : t("download_manager.install_prompt.description", {
                name: installPrompt?.fileName || installPrompt?.installer?.version || "",
              })}
        </ModalDescription>
      </UnifiedModal>
    </DownloadsContext.Provider>
  );
};
