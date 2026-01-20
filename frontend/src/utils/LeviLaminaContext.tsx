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
  loading: boolean;
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

  const isLLSupported = useCallback(
    (version: string) => {
      if (!version || !llMap.size) return false;
      if (llMap.has(version)) return true;
      const parts = version.split(".");
      if (parts.length >= 3) {
        const key = `${parts[0]}.${parts[1]}.${parts[2]}`;
        return llMap.has(key);
      }
      return false;
    },
    [llMap],
  );

  return (
    <LeviLaminaContext.Provider
      value={{
        llMap,
        refreshLLDB: fetchLLDB,
        isLLSupported,
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
