import React, {
  createContext,
  useContext,
  useEffect,
  useCallback,
  useMemo,
  useState,
} from "react";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { useStartupInteractive } from "@/utils/startupState";

type LeviLaminaDB = Record<string, string[]>;

type CtxValue = {
  llMap: Map<string, string[]>;
  gameToLLVersions: Record<string, string[]>;
  refreshLLDB: () => Promise<void>;
  isLLSupported: (version: string) => boolean;
  getSupportedLLVersions: (version: string) => string[];
  getLatestLLVersion: (version: string) => string;
  compareLLVersions: (a: string, b: string) => number;
  normalizeGameVersion: (version: string) => string;
  loading: boolean;
  mappingAvailable: boolean;
  mappingInitialized: boolean;
};

const normalizeLLVersion = (version: string): string =>
  String(version || "").trim();

const normalizeGameVersionValue = (version: string): string =>
  String(version || "")
    .trim()
    .replace(/^v/i, "");

const normalizeLLVersionList = (list: unknown): string[] => {
  if (!Array.isArray(list)) return [];
  const normalized = list
    .map((item) => normalizeLLVersion(String(item || "")))
    .filter(Boolean);
  const unique = Array.from(new Set(normalized));
  unique.sort((a, b) => compareLLVersionValue(b, a));
  return unique;
};

type ParsedSemver = {
  major: number;
  minor: number;
  patch: number;
  pre: string[];
};

const parseSemver = (input: string): ParsedSemver | null => {
  const v = normalizeLLVersion(input).replace(/^v/i, "");
  if (!v) return null;
  const match = v.match(
    /^(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$/,
  );
  if (!match) return null;
  return {
    major: Number(match[1]),
    minor: Number(match[2]),
    patch: Number(match[3]),
    pre: match[4] ? match[4].split(".") : [],
  };
};

const compareSemver = (a: ParsedSemver, b: ParsedSemver): number => {
  if (a.major !== b.major) return a.major > b.major ? 1 : -1;
  if (a.minor !== b.minor) return a.minor > b.minor ? 1 : -1;
  if (a.patch !== b.patch) return a.patch > b.patch ? 1 : -1;

  const aRelease = a.pre.length === 0;
  const bRelease = b.pre.length === 0;
  if (aRelease && bRelease) return 0;
  if (aRelease) return 1;
  if (bRelease) return -1;

  const len = Math.max(a.pre.length, b.pre.length);
  for (let i = 0; i < len; i += 1) {
    const ai = a.pre[i];
    const bi = b.pre[i];
    if (ai === undefined) return -1;
    if (bi === undefined) return 1;
    const aiNum = /^[0-9]+$/.test(ai);
    const biNum = /^[0-9]+$/.test(bi);
    if (aiNum && biNum) {
      const an = Number(ai);
      const bn = Number(bi);
      if (an !== bn) return an > bn ? 1 : -1;
      continue;
    }
    if (aiNum && !biNum) return -1;
    if (!aiNum && biNum) return 1;
    if (ai !== bi) return ai > bi ? 1 : -1;
  }
  return 0;
};

const compareLLVersionValue = (a: string, b: string): number => {
  const av = parseSemver(a);
  const bv = parseSemver(b);
  if (av && bv) return compareSemver(av, bv);
  if (av) return 1;
  if (bv) return -1;
  return normalizeLLVersion(a).localeCompare(normalizeLLVersion(b));
};

const LeviLaminaContext = createContext<CtxValue | undefined>(undefined);

export const LeviLaminaProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const hasBackend = minecraft !== undefined;
  const startupInteractive = useStartupInteractive();
  const [llMap, setLlMap] = useState<Map<string, string[]>>(new Map());
  const [loading, setLoading] = useState<boolean>(false);
  const [mappingAvailable, setMappingAvailable] = useState<boolean>(false);
  const [mappingInitialized, setMappingInitialized] = useState<boolean>(false);

  const fetchLLDB = useCallback(async () => {
    if (!hasBackend || !(minecraft as any).FetchLeviLaminaVersionDB) {
      setLlMap(new Map());
      setMappingAvailable(false);
      setMappingInitialized(true);
      return;
    }

    setLoading(true);
    try {
      const res = ((await (minecraft as any).FetchLeviLaminaVersionDB()) ||
        {}) as LeviLaminaDB;
      const nextMap = new Map<string, string[]>();

      for (const [gameVersion, llVersions] of Object.entries(res)) {
        const normalizedGameVersion = normalizeGameVersionValue(gameVersion);
        const normalizedLLVersions = normalizeLLVersionList(llVersions);
        if (!normalizedGameVersion || normalizedLLVersions.length === 0) {
          continue;
        }
        nextMap.set(normalizedGameVersion, normalizedLLVersions);
      }

      setLlMap(nextMap);
      setMappingAvailable(nextMap.size > 0);
    } catch (e) {
      console.error("FetchLeviLaminaVersionDB failed", e);
      setLlMap(new Map());
      setMappingAvailable(false);
    } finally {
      setLoading(false);
      setMappingInitialized(true);
    }
  }, [hasBackend]);

  useEffect(() => {
    if (!startupInteractive) return;
    fetchLLDB();
  }, [fetchLLDB, startupInteractive]);

  const compareLLVersions = useCallback((a: string, b: string): number => {
    const av = parseSemver(a);
    const bv = parseSemver(b);
    if (!av || !bv) return Number.NaN;
    return compareSemver(av, bv);
  }, []);

  const normalizeGameVersion = useCallback((version: string): string => {
    return normalizeGameVersionValue(version);
  }, []);

  const getSupportedLLVersions = useCallback(
    (version: string): string[] => {
      const v = normalizeGameVersionValue(version);
      if (!v || !llMap.size) return [];
      return llMap.get(v) || [];
    },
    [llMap],
  );

  const getLatestLLVersion = useCallback(
    (version: string): string => {
      const versions = getSupportedLLVersions(version);
      return versions.length > 0 ? versions[0] : "";
    },
    [getSupportedLLVersions],
  );

  const isLLSupported = useCallback(
    (version: string) => {
      return getSupportedLLVersions(version).length > 0;
    },
    [getSupportedLLVersions],
  );

  const gameToLLVersions = useMemo<Record<string, string[]>>(() => {
    return Object.fromEntries(
      Array.from(llMap.entries()).map(([gameVersion, llVersions]) => [
        gameVersion,
        [...llVersions],
      ]),
    );
  }, [llMap]);

  return (
    <LeviLaminaContext.Provider
      value={{
        llMap,
        gameToLLVersions,
        refreshLLDB: fetchLLDB,
        isLLSupported,
        getSupportedLLVersions,
        getLatestLLVersion,
        compareLLVersions,
        normalizeGameVersion,
        loading,
        mappingAvailable,
        mappingInitialized,
      }}
    >
      {children}
    </LeviLaminaContext.Provider>
  );
};

export const useLeviLamina = () => {
  const context = useContext(LeviLaminaContext);
  if (context === undefined) {
    throw new Error("useLeviLamina must be used within a LeviLaminaProvider");
  }
  return context;
};
