import React, {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Events } from "@wailsio/runtime";
import {
  GetAllVersionsStatus,
  GetVersionStatus,
} from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import * as main from "bindings/github.com/liteldev/LeviLauncher/models";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
type ItemType = "Preview" | "Release";

type VersionItemLite = {
  version: string;
  short: string;
  type: ItemType;
};

type CtxValue = {
  map: Map<string, main.VersionStatus>;
  setCurrentDownloadingInfo: (
    short: string | null,
    type: string | null,
  ) => void;
  refreshAll: (items: VersionItemLite[]) => Promise<void>;
  refreshOne: (short: string, type: string) => Promise<void>;
  refreshing: boolean;
  markDownloaded: (short: string, type: string) => void;
};

const VersionStatusContext = createContext<CtxValue | undefined>(undefined);

export const VersionStatusProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const hasBackend = minecraft !== undefined;
  const [map, setMap] = useState<Map<string, main.VersionStatus>>(() => {
    try {
      const raw = localStorage.getItem("levi_version_status_map");
      if (!raw) return new Map();
      const obj = JSON.parse(raw);
      const m = new Map<string, main.VersionStatus>();
      if (obj && typeof obj === "object") {
        Object.entries(obj).forEach(([k, v]) => {
          if (v && typeof v === "object") {
            m.set(String(k), v as main.VersionStatus);
          }
        });
      }
      return m;
    } catch {
      return new Map();
    }
  });
  const [refreshing, setRefreshing] = useState<boolean>(false);

  useEffect(() => {
    try {
      const obj: Record<string, any> = {};
      map.forEach((val, key) => {
        obj[key] = val;
      });
      localStorage.setItem("levi_version_status_map", JSON.stringify(obj));
    } catch {}
  }, [map]);

  const downloadingShortRef = useRef<string | null>(null);
  const downloadingTypeRef = useRef<string | null>(null);
  const setCurrentDownloadingInfo = (
    short: string | null,
    type: string | null,
  ) => {
    downloadingShortRef.current = short;
    downloadingTypeRef.current = type ? type.toLowerCase() : null;
  };

  const refreshAll = async (items: VersionItemLite[]) => {
    if (!hasBackend) return;
    if (!items || items.length === 0) return;
    try {
      setRefreshing(true);
      const data = items.map((it) => ({
        version: it.version || it.short,
        short: it.short,
        type: it.type.toLowerCase() as any,
      }));
      const statusList = await GetAllVersionsStatus(data as any);
      const newMap = new Map<string, main.VersionStatus>();
      (statusList || []).forEach(
        (status: main.VersionStatus, index: number) => {
          const originalItem = data[index];
          if (originalItem) {
            newMap.set(originalItem.short, status);
          }
        },
      );
      setMap(newMap);
    } catch (err) {
      console.error("refreshAll failed:", err);
    } finally {
      setRefreshing(false);
    }
  };

  const refreshOne = async (short: string, type: string) => {
    if (!hasBackend) return;
    try {
      const s = await GetVersionStatus(short, type.toLowerCase());
      setMap((prev) => {
        const m = new Map(prev);
        m.set(short, s);
        return m;
      });
    } catch (err) {
      console.error("refreshOne failed:", err);
    }
  };

  const markDownloaded = (short: string, type: string) => {
    setMap((prev) => {
      const m = new Map(prev);
      const existing = m.get(short);
      m.set(short, {
        version: short,
        type: (existing?.type || String(type).toLowerCase()) as any,
        isInstalled: existing?.isInstalled || false,
        isDownloaded: true,
      } as any);
      return m;
    });
  };

  useEffect(() => {
    if (!hasBackend) return;
    const off = Events.On("msixvc_download_done", (event) => {
      const d = String(event?.data || "");
      const inferShort = () => {
        if (downloadingShortRef.current)
          return String(downloadingShortRef.current);
        const base = d ? d.split(/[/\\]/).pop() || "" : "";
        return String(base)
          .replace(/^\s*(preview|release)\s+/i, "")
          .replace(/\.msixvc$/i, "")
          .trim();
      };
      const short = inferShort();
      const type = (downloadingTypeRef.current || "release").toLowerCase();
      if (!short) return;
      setMap((prev) => {
        const m = new Map(prev);
        const existing = m.get(short);
        m.set(short, {
          version: short,
          type: (existing?.type || type) as any,
          isInstalled: existing?.isInstalled || false,
          isDownloaded: true,
        } as any);
        return m;
      });
      refreshOne(short, type);
      downloadingShortRef.current = null;
      downloadingTypeRef.current = null;
    });
    return () => {
      try {
        off && off();
      } catch {}
    };
  }, [hasBackend]);

  const value = useMemo<CtxValue>(
    () => ({
      map,
      setCurrentDownloadingInfo,
      refreshAll,
      refreshOne,
      refreshing,
      markDownloaded,
    }),
    [map, refreshing],
  );

  return (
    <VersionStatusContext.Provider value={value}>
      {children}
    </VersionStatusContext.Provider>
  );
};

export const useVersionStatus = () => {
  const ctx = useContext(VersionStatusContext);
  if (!ctx)
    throw new Error(
      "useVersionStatus must be used within VersionStatusProvider",
    );
  return ctx;
};
