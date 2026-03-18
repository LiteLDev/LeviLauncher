import React, {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import {
  CURRENT_VERSION_CHANGED_EVENT,
  clearCurrentVersionName as clearCurrentVersionNameStorage,
  readCurrentVersionName,
  saveCurrentVersionName as saveCurrentVersionNameStorage,
  type CurrentVersionChangedDetail,
} from "@/utils/currentVersion";

type CurrentVersionContextValue = {
  currentVersionName: string;
  setCurrentVersionName: (name: string, source?: string) => void;
  clearCurrentVersionName: (source?: string) => void;
};

const CurrentVersionContext = createContext<
  CurrentVersionContextValue | undefined
>(undefined);

export const CurrentVersionProvider: React.FC<{
  children: React.ReactNode;
}> = ({ children }) => {
  const [currentVersionName, setCurrentVersionNameState] = useState<string>(
    () => readCurrentVersionName(),
  );

  useEffect(() => {
    const handler = (event: Event) => {
      const custom = event as CustomEvent<CurrentVersionChangedDetail>;
      const next = String(custom.detail?.next || "").trim();
      setCurrentVersionNameState(next);
    };
    window.addEventListener(
      CURRENT_VERSION_CHANGED_EVENT,
      handler as EventListener,
    );
    return () =>
      window.removeEventListener(
        CURRENT_VERSION_CHANGED_EVENT,
        handler as EventListener,
      );
  }, []);

  const value = useMemo<CurrentVersionContextValue>(
    () => ({
      currentVersionName,
      setCurrentVersionName: (name: string, source = "context") => {
        saveCurrentVersionNameStorage(name, source);
      },
      clearCurrentVersionName: (source = "context") => {
        clearCurrentVersionNameStorage(source);
      },
    }),
    [currentVersionName],
  );

  return (
    <CurrentVersionContext.Provider value={value}>
      {children}
    </CurrentVersionContext.Provider>
  );
};

export const useCurrentVersion = (): CurrentVersionContextValue => {
  const context = useContext(CurrentVersionContext);
  if (!context) {
    throw new Error(
      "useCurrentVersion must be used within CurrentVersionProvider",
    );
  }
  return context;
};
