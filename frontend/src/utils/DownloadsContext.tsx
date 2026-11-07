import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from "react";
import { Events } from "@wailsio/runtime";
import { useLocation, useNavigate } from "react-router-dom";
import { ROUTES } from "@/constants/routes";
import { UnifiedModal } from "@/components/UnifiedModal";
import { ModalDescription } from "@/components/ModalPrimitives";
import { resolveInstallError } from "@/utils/installError";
import {
  appendTaskLog,
  finishActivityTask,
  notify,
  setTaskProgress,
  startActivityTask,
} from "@/utils/activity";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";

import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";

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
  installer?: { version: string; type: string; packageType?: "gdk" | "uwp"; uuid?: string; isLeviLaminaSupported: boolean };
}

export const isDownloadActive = (status: string): boolean =>
  ["starting", "started", "resumed", "verifying"].includes(status);

export const isDownloadTerminal = (status: string): boolean =>
  ["done", "cancelled", "error"].includes(status);

/** Route state that opens the install page for a finished or running download. */
export const installStateForDownload = (item: DownloadItem, returnTo: string) => ({
  mirrorVersion: item.installer?.version || "",
  mirrorType: item.installer?.type || "Release",
  packageType: item.installer?.packageType,
  isLeviLaminaSupported: Boolean(item.installer?.isLeviLaminaSupported),
  // A running download is followed on the install page and installed when done.
  ...(item.status === "done"
    ? { installerPath: item.dest }
    : { pendingDownload: item.dest }),
  returnTo,
});

export const downloadErrorText = (
  error: string,
  t: TFunction,
): string => (error.includes("ERR_UWP_") ? resolveInstallError(error, t) : error);

// Progress is mirrored into the activity task at most this often.
const ACTIVITY_PROGRESS_INTERVAL_MS = 1000;

type DownloadActivity = {
  id: string;
  startedAt: number;
  finished: boolean;
  lastProgressAt: number;
  lastPercent: number;
};

interface DownloadsContextType {
  downloads: DownloadItem[];
  /** Resolves to the task key (the destination path) or null when it did not start. */
  startDownload: (
    url: string,
    filename?: string,
    md5sum?: string,
    installer?: DownloadItem["installer"],
  ) => Promise<string | null>;
  cancelDownload: (dest?: string) => void;
  removeDownload: (dest: string) => void;
  clearError: (dest?: string) => void;
  /** Suppress the global "install now?" prompt while a page follows this download. */
  watchInstall: (dest: string) => () => void;
  /** The download was handed to the install page; never prompt for it again. */
  dismissInstallPrompt: (dest: string) => void;
  isDownloading: boolean;
}

const DownloadsContext = createContext<DownloadsContextType | null>(null);

export const useDownloads = () => {
  const context = useContext(DownloadsContext);
  if (!context) {
    return {
      downloads: [],
      startDownload: () => Promise.resolve(null),
      cancelDownload: () => {},
      removeDownload: () => {},
      clearError: () => {},
      watchInstall: () => () => {},
      dismissInstallPrompt: () => {},
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
  const location = useLocation();
  const [dismissedInstallPrompts, setDismissedInstallPrompts] = useState<Set<string>>(
    () => new Set(),
  );
  const [watchedInstalls, setWatchedInstalls] = useState<Record<string, number>>({});
  const [downloadsMap, setDownloadsMap] = useState<Record<string, DownloadItem>>({});
  const speedRef = useRef<Record<string, { ts: number; bytes: number }>>({});
  const downloadsRef = useRef<Record<string, DownloadItem>>({});
  const cancelledRef = useRef<Set<string>>(new Set());
  const startingRef = useRef<Set<string>>(new Set());
  const activityRef = useRef<Record<string, DownloadActivity>>({});
  const outcomeRef = useRef<
    Record<string, { at: number; status: "success" | "error" | "cancelled"; summary: string }>
  >({});
  const tRef = useRef(t);
  tRef.current = t;

  const getFileNameFromDest = (dest: string) => {
    if (!dest || typeof dest !== "string") return "";
    return dest.split(/[\\/]/).pop() || "";
  };

  const displayNameOf = (dest: string) =>
    downloadsRef.current[dest]?.fileName || getFileNameFromDest(dest) || dest;

  // Every download is mirrored as an activity task so its history survives
  // after the row or toast is gone.
  const activityFor = (dest: string, fresh = false): DownloadActivity => {
    const existing = activityRef.current[dest];
    if (existing && !fresh) return existing;
    const entry: DownloadActivity = {
      id: startActivityTask({
        title: tRef.current("download_flow.activity.title", { name: displayNameOf(dest) }),
        source: "downloads",
      }),
      startedAt: Date.now(),
      finished: false,
      lastProgressAt: 0,
      lastPercent: -1,
    };
    activityRef.current[dest] = entry;
    return entry;
  };

  const finishActivity = (
    dest: string,
    status: "success" | "error" | "cancelled",
    summary: string,
  ) => {
    outcomeRef.current[dest] = { at: Date.now(), status, summary };
    const activity = activityFor(dest);
    if (activity.finished) return;
    activity.finished = true;
    appendTaskLog(
      activity.id,
      summary,
      status === "success" ? "success" : status === "error" ? "error" : "warning",
    );
    finishActivityTask(activity.id, status, summary);
  };

  const logStatus = (dest: string, status: string) => {
    const activity = activityFor(dest);
    if (activity.finished) return;
    if (status === "cancelled") {
      finishActivity(dest, "cancelled", tRef.current("download_flow.status.cancelled"));
      return;
    }
    if (["starting", "started", "resumed", "verifying"].includes(status)) {
      appendTaskLog(activity.id, tRef.current(`download_flow.status.${status}`));
    }
  };

  const updateDownload = (
    dest: string,
    updater: (current: DownloadItem) => DownloadItem,
  ) => {
    if (!dest) return;
    const apply = (prev: Record<string, DownloadItem>) => {
      const existing = prev[dest];
      const base: DownloadItem = existing || {
        dest,
        progress: null,
        speed: 0,
        status: "",
        error: "",
        fileName: getFileNameFromDest(dest),
      };
      return { ...prev, [dest]: updater(base) };
    };
    // Keep the ref current before React commits so events that arrive in
    // between (e.g. a dest-less error right after start) see this row.
    downloadsRef.current = apply(downloadsRef.current);
    setDownloadsMap(apply);
  };

  useEffect(() => {
    downloadsRef.current = downloadsMap;
  }, [downloadsMap]);

  const failDownload = (dest: string, msg: string) => {
    const text = downloadErrorText(msg, tRef.current);
    finishActivity(dest, "error", text);
    notify({
      variant: "danger",
      source: "downloads",
      title: tRef.current("download_flow.failed_title", { name: displayNameOf(dest) }),
      description: text,
      actionProps: {
        children: tRef.current("download_flow.view_tasks"),
        onPress: () => navigate(ROUTES.downloadTasks),
      },
    });
  };

  useEffect(() => {
    const offs: (() => void)[] = [];

    offs.push(
      Events.On("msixvc_download_progress", (event) => {
        const downloaded = Number(event?.data?.Downloaded || 0);
        const total = Number(event?.data?.Total || 0);
        const dest = String(event?.data?.Dest || "");
        if (!dest || cancelledRef.current.has(dest)) return;

        const activity = activityFor(dest);
        const now = Date.now();
        if (!activity.finished && total > 0) {
          const percent = Math.floor((downloaded / total) * 100);
          if (
            percent !== activity.lastPercent &&
            (percent >= 100 || now - activity.lastProgressAt >= ACTIVITY_PROGRESS_INTERVAL_MS)
          ) {
            activity.lastPercent = percent;
            activity.lastProgressAt = now;
            setTaskProgress(activity.id, percent);
          }
        }

        updateDownload(dest, (prev) => {
          let speed = prev.speed;
          try {
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
          logStatus(key, status);
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
          // A status without a destination applies to running tasks only;
          // finished rows keep their own outcome.
          Object.keys(downloadsRef.current)
            .filter((key) => isDownloadActive(downloadsRef.current[key]?.status || ""))
            .forEach(applyStatus);
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

        const targets = dest
          ? [dest]
          : Object.keys(downloadsRef.current).filter((key) =>
              isDownloadActive(downloadsRef.current[key]?.status || ""),
            );
        targets.forEach((key) => {
          updateDownload(key, (prev) => ({
            ...prev,
            error: msg,
            status: "error",
            speed: 0,
          }));
          failDownload(key, msg);
        });
        if (targets.length === 0) {
          notify({
            variant: "danger",
            source: "downloads",
            title: tRef.current("download_flow.failed"),
            description: downloadErrorText(msg, tRef.current),
          });
        }
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
          error: "",
          progress: {
            downloaded: prev.progress?.total || prev.progress?.downloaded || 0,
            total: prev.progress?.total || 0,
            dest,
          },
          speed: 0,
          fileName: fname || prev.fileName,
        }));
        speedRef.current[dest] = { ts: 0, bytes: 0 };
        finishActivity(dest, "success", tRef.current("download_flow.status.done"));
        notify({
          variant: "success",
          source: "downloads",
          title: tRef.current("download_flow.done_title"),
          description: fname || dest,
        });
      }),
    );

    return () => {
      offs.forEach((off) => off());
    };
  }, []);

  const startDownload = async (
    url: string,
    filename?: string,
    md5sum?: string,
    installer?: DownloadItem["installer"],
  ): Promise<string | null> => {
    if (typeof minecraft === "undefined") return null;

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

    const running = Object.values(downloadsRef.current).find(
      (dl) =>
        (dl.fileName === displayName ||
          (dl.dest && getFileNameFromDest(dl.dest) === displayName)) &&
        isDownloadActive(dl.status),
    );
    if (running) {
      // The same file is already on its way; hand back that task so callers
      // can follow it instead of failing.
      notify({
        variant: "warning",
        source: "downloads",
        title: t("download_flow.already_title"),
        description: t("downloadpage.error.already_downloading"),
      });
      return running.dest;
    }
    if (startingRef.current.has(displayName || url)) {
      notify({
        variant: "warning",
        source: "downloads",
        title: t("download_flow.already_title"),
        description: t("downloadpage.error.already_downloading"),
      });
      return null;
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
    const calledAt = Date.now();
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
      // A retry gets its own activity entry; events that arrived while the
      // start call was pending already created one for this attempt.
      const previous = activityRef.current[key];
      const activity =
        previous && previous.startedAt >= calledAt
          ? previous
          : activityFor(key, true);
      if (!activity.finished) {
        appendTaskLog(activity.id, t("download_flow.status.starting"));
        // It may already have ended while the start call was pending.
        const outcome = outcomeRef.current[key];
        if (outcome && outcome.at >= calledAt) {
          finishActivity(key, outcome.status, outcome.summary);
        }
      }
      return key;
    } catch (e) {
      notify({
        variant: "danger",
        source: "downloads",
        title: t("download_flow.start_failed_title", { name: displayName }),
        description: downloadErrorText(String(e), t),
      });
      return null;
    } finally {
      startingRef.current.delete(displayName || url);
    }
  };

  const cancelDownload = (dest?: string) => {
    if (typeof minecraft === "undefined") return;

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
      logStatus(dest, "cancelled");
      return;
    }

    minecraft.CancelMsixvcDownload();
    // Verifying tasks are not cancellable and keep their own outcome.
    Object.keys(downloadsRef.current)
      .filter((key) => ["started", "starting", "resumed"].includes(downloadsRef.current[key]?.status || ""))
      .forEach((key) => logStatus(key, "cancelled"));
    setDownloadsMap((prev) => {
      const next = { ...prev };
      Object.keys(next).forEach((key) => {
        if (
          next[key].status === "started" ||
          next[key].status === "starting" ||
          next[key].status === "resumed"
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

  // Rows are only removed when the user asks; finished and failed tasks stay.
  const removeDownload = (dest: string) => {
    if (!isDownloadTerminal(downloadsRef.current[dest]?.status || "")) return;
    cancelledRef.current.add(dest);
    delete activityRef.current[dest];
    setDownloadsMap((prev) => {
      const next = { ...prev };
      delete next[dest];
      return next;
    });
  };

  const clearError = (dest?: string) => {
    setDownloadsMap((prev) => {
      const next: Record<string, DownloadItem> = { ...prev };
      (dest ? [dest] : Object.keys(next)).forEach((key) => {
        if (next[key]) next[key] = { ...next[key], error: "" };
      });
      return next;
    });
  };

  const watchInstall = useCallback((dest: string) => {
    setWatchedInstalls((prev) => ({ ...prev, [dest]: (prev[dest] || 0) + 1 }));
    return () =>
      setWatchedInstalls((prev) => {
        const next = { ...prev };
        if ((next[dest] || 0) <= 1) delete next[dest];
        else next[dest] -= 1;
        return next;
      });
  }, []);

  const dismissInstallPrompt = useCallback((dest: string) => {
    setDismissedInstallPrompts((prev) => new Set(prev).add(dest));
  }, []);

  const downloads = Object.values(downloadsMap);
  const isDownloading = downloads.some((dl) => isDownloadActive(dl.status));
  // The install page continues its own download; the prompt only picks up
  // downloads nobody is following so the original intent still completes.
  const onInstallPage = location.pathname === ROUTES.install;
  const installPrompt = onInstallPage
    ? undefined
    : downloads.find((dl) =>
        dl.status === "done" &&
        dl.installer &&
        !dismissedInstallPrompts.has(dl.dest) &&
        !watchedInstalls[dl.dest],
      );
  const closeInstallPrompt = () => {
    if (installPrompt) dismissInstallPrompt(installPrompt.dest);
  };

  return (
    <DownloadsContext.Provider
      value={{
        downloads,
        startDownload,
        cancelDownload,
        removeDownload,
        clearError,
        watchInstall,
        dismissInstallPrompt,
        isDownloading,
      }}
    >
      {children}
      <UnifiedModal
        isOpen={!!installPrompt}
        onOpenChange={(open) => { if (!open) closeInstallPrompt(); }}
        type="success"
        title={t("download_manager.install_prompt.title")}
        confirmText={t("download_manager.install_prompt.confirm")}
        cancelText={t("download_manager.install_prompt.later")}
        showCancelButton
        onCancel={closeInstallPrompt}
        onConfirm={() => {
          if (!installPrompt?.installer) return;
          closeInstallPrompt();
          navigate(ROUTES.install, {
            state: installStateForDownload(installPrompt, location.pathname),
          });
        }}
      >
        <ModalDescription>
          {t("download_manager.install_prompt.description", {
            name: installPrompt?.fileName || installPrompt?.installer?.version || "",
          })}
        </ModalDescription>
      </UnifiedModal>
    </DownloadsContext.Provider>
  );
};
