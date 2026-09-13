import { useCallback, useEffect, useState } from "react";
import {
  APPEARANCE_EVENT,
  APPEARANCE_KEY,
  readBackgroundAppearance,
  type AppearanceMode,
  type BackgroundAppearance,
} from "@/utils/backgroundAppearance";

export const useBackgroundAppearance = () => {
  const [profiles, setProfiles] = useState(readBackgroundAppearance);
  useEffect(() => {
    const refresh = () => setProfiles(readBackgroundAppearance());
    const onStorage = (event: StorageEvent) => {
      if (event.key === APPEARANCE_KEY || event.key === null) refresh();
    };
    window.addEventListener(APPEARANCE_EVENT, refresh);
    window.addEventListener("storage", onStorage);
    return () => {
      window.removeEventListener(APPEARANCE_EVENT, refresh);
      window.removeEventListener("storage", onStorage);
    };
  }, []);

  const updateProfile = useCallback(
    (mode: AppearanceMode, patch: Partial<BackgroundAppearance>) => {
      const current = readBackgroundAppearance();
      const next = { ...current, [mode]: { ...current[mode], ...patch } };
      localStorage.setItem(APPEARANCE_KEY, JSON.stringify(next));
      window.dispatchEvent(new Event(APPEARANCE_EVENT));
    },
    [],
  );

  return { profiles, updateProfile };
};
