import React, {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
} from "react";
import { Events } from "@wailsio/runtime";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { addToast } from "@heroui/react";
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
}

interface DownloadsContextType {
  downloads: DownloadItem[];
  startDownload: (
    url: string,
    filename?: string,
    md5sum?: string,
  ) => Promise<boolean>;
  cancelDownload: (dest?: string) => void;
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
  const [downloadsMap, setDownloadsMap] = useState<
    Record<string, DownloadItem>
  >({});
  const speedRef = useRef<Record<string, { ts: number; bytes: number }>>({});
  const downloadsRef = useRef<Record<string, DownloadItem>>({});
  const cancelledRef = useRef<Set<string>>(new Set());

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
        addToast({
          description: msg,
          color: "danger",
        });
      }),
    );

    offs.push(
      Events.On("msixvc_download_done", (event) => {
        const raw = event?.data;
        const dest =
          typeof raw === "string" ? String(raw) : String(raw?.Dest || "");
        if (!dest) return;
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
        addToast({
          title: t("downloadpage.download.success_body") + " " + (fname || ""),
          color: "success",
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

    const isAlreadyDownloading = Object.values(downloadsRef.current).some(
      (dl) =>
        (dl.fileName === displayName ||
          (dl.dest && getFileNameFromDest(dl.dest) === displayName)) &&
        (dl.status === "started" ||
          dl.status === "resumed" ||
          dl.status === "starting" ||
          dl.status === "verifying"),
    );

    if (isAlreadyDownloading) {
      addToast({
        description: t("downloadpage.error.already_downloading"),
        color: "danger",
      });
      return false;
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
    try {
      const dest = await minecraft.StartMsixvcDownload(
        urlWithFilename,
        md5sum || "",
      );
      if (dest) cancelledRef.current.delete(dest);

      const key = dest || displayName || urlWithFilename;
      updateDownload(key, (prev) => ({
        ...prev,
        dest: key,
        status: "starting",
        error: "",
        progress: null,
        speed: 0,
        fileName: displayName || prev.fileName || getFileNameFromDest(key),
        url: url,
      }));
      addToast({
        title: t("downloadpage.mirror.download_started"),
        color: "success",
      });
      return true;
    } catch (e) {
      addToast({
        description: String(e),
        color: "danger",
      });
      return false;
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
      return;
    }

    minecraft.CancelMsixvcDownload();
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

  const removeDownload = (dest: string) => {
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
  const isDownloading = downloads.some(
    (dl) =>
      dl.status === "started" ||
      dl.status === "resumed" ||
      dl.status === "starting",
  );

  return (
    <DownloadsContext.Provider
      value={{
        downloads,
        startDownload,
        cancelDownload,
        removeDownload,
        clearError,
        isDownloading,
      }}
    >
      {children}
    </DownloadsContext.Provider>
  );
};
