import React, {
  createContext,
  useContext,
  useEffect,
  useState,
  useCallback,
} from "react";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";

type LeviLaminaDB = Record<string, any>;

type CtxValue = {
  llMap: Map<string, any>;
  refreshLLDB: () => Promise<void>;
  isLLSupported: (version: string) => boolean;
  getSupportedLLVersions: (version: string) => string[];
  getLatestLLVersion: (version: string) => string;
  compareLLVersions: (a: string, b: string) => number;
  loading: boolean;
};

const normalizeLLVersion = (version: string): string =>
  String(version || "").trim();

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
  const [llMap, setLlMap] = useState<Map<string, any>>(new Map());
  const [loading, setLoading] = useState<boolean>(false);

  const fetchLLDB = useCallback(async () => {
    if (hasBackend && (minecraft as any).FetchLeviLaminaVersionDB) {
      setLoading(true);
      try {
        const res = await (minecraft as any).FetchLeviLaminaVersionDB();
        if (res) {
          setLlMap(new Map(Object.entries(res)));
        }
      } catch (e) {
        console.error("FetchLeviLaminaVersionDB failed", e);
      } finally {
        setLoading(false);
      }
    }
  }, [hasBackend]);

  useEffect(() => {
    fetchLLDB();
  }, [fetchLLDB]);

  const compareLLVersions = useCallback((a: string, b: string): number => {
    const av = parseSemver(a);
    const bv = parseSemver(b);
    if (!av || !bv) return Number.NaN;
    return compareSemver(av, bv);
  }, []);

  const getSupportedLLVersions = useCallback(
    (version: string): string[] => {
      const v = String(version || "").trim();
      if (!v || !llMap.size) return [];

      const normalize = (list: any): string[] => {
        if (!Array.isArray(list)) return [];
        const normalized = list
          .map((it) => String(it || "").trim())
          .filter((it) => it.length > 0);
        const unique = Array.from(new Set(normalized));
        unique.sort((a, b) => compareLLVersionValue(b, a));
        return unique;
      };

      const exact = normalize(llMap.get(v));
      if (exact.length > 0) return exact;

      const parts = v.split(".");
      if (parts.length >= 3) {
        const key = `${parts[0]}.${parts[1]}.${parts[2]}`;
        const fallback = normalize(llMap.get(key));
        if (fallback.length > 0) return fallback;
      }
      return [];
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

  return (
    <LeviLaminaContext.Provider
      value={{
        llMap,
        refreshLLDB: fetchLLDB,
        isLLSupported,
        getSupportedLLVersions,
        getLatestLLVersion,
        compareLLVersions,
        loading,
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
