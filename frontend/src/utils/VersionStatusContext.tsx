import React, {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import { Events } from "@wailsio/runtime";
import {
  GetAllVersionsStatus,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/versionservice";
import * as main from "bindings/github.com/liteldev/LeviLauncher/internal/app/models";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import * as versionService from "bindings/github.com/liteldev/LeviLauncher/internal/app/versionservice";
import { installerIdentityFromPath, normalizePackageType, versionStatusKey, type PackageType, type VersionChannel } from "@/utils/packageType";
type ItemType = VersionChannel;

type VersionItemLite = {
  version: string;
  short: string;
  type: ItemType;
  packageType?: PackageType;
};

type CtxValue = {
  map: Map<string, main.VersionStatus>;
  refreshAll: (items: VersionItemLite[]) => Promise<void>;
  refreshOne: (short: string, type: string, packageType?: PackageType) => Promise<void>;
  refreshing: boolean;
  markDownloaded: (short: string, type: string, packageType?: PackageType) => void;
};

const VersionStatusContext = createContext<CtxValue | undefined>(undefined);

export const VersionStatusProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const hasBackend = minecraft !== undefined;
  const [map, setMap] = useState<Map<string, main.VersionStatus>>(() => {
    try {
      const raw = localStorage.getItem("levi_version_status_map.v2");
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
      localStorage.setItem("levi_version_status_map.v2", JSON.stringify(obj));
    } catch {}
  }, [map]);

  const refreshAll = async (items: VersionItemLite[]) => {
    if (!hasBackend) return;
    if (!items || items.length === 0) return;
    try {
      setRefreshing(true);
      const data = items.map((it) => ({
        version: it.version || it.short,
        short: it.short,
        type: it.type.toLowerCase() as any,
        packageType: normalizePackageType(it.packageType),
      }));
      const statusList = await GetAllVersionsStatus(data as any);
      const newMap = new Map<string, main.VersionStatus>();
      (statusList || []).forEach(
        (status: main.VersionStatus, index: number) => {
          const originalItem = data[index];
          if (originalItem) {
            newMap.set(versionStatusKey(originalItem.short, originalItem.type, originalItem.packageType), status);
          }
        },
      );
      setMap((previous) => new Map([...previous, ...newMap]));
    } catch (err) {
      console.error("refreshAll failed:", err);
    } finally {
      setRefreshing(false);
    }
  };

  const refreshOne = async (short: string, type: string, packageType: PackageType = "gdk") => {
    if (!hasBackend) return;
    try {
      const s = await versionService.GetVersionStatusForPackage(short, type.toLowerCase(), packageType);
      setMap((prev) => {
        const m = new Map(prev);
        m.set(versionStatusKey(short, type, packageType), s);
        return m;
      });
    } catch (err) {
      console.error("refreshOne failed:", err);
    }
  };

  const markDownloaded = (short: string, type: string, packageType: PackageType = "gdk") => {
    setMap((prev) => {
      const m = new Map(prev);
      const key = versionStatusKey(short, type, packageType);
      const existing = m.get(key);
      m.set(key, {
        version: short,
        type: (existing?.type || String(type).toLowerCase()) as any,
        packageType,
        isInstalled: existing?.isInstalled || false,
        isDownloaded: true,
      } as any);
      return m;
    });
  };

  useEffect(() => {
    if (!hasBackend) return;
    const off = Events.On("msixvc_download_done", (event) => {
      const raw = event?.data;
      const d =
        typeof raw === "string"
          ? String(raw)
          : String(raw?.Dest || "");

      // Derive identity from the completed task, never from the last clicked row.
      const identity = installerIdentityFromPath(d);
      if (!identity) return;
      markDownloaded(identity.version, identity.type, identity.packageType);
      void refreshOne(identity.version, identity.type, identity.packageType);

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
