import { useCallback, useState, useEffect } from "react";

export const useLayoutMode = () => {
  const [layoutMode, setLayoutMode] = useState<"navbar" | "sidebar">(() => {
    try {
      return (
        (localStorage.getItem("app.layoutMode") as "navbar" | "sidebar") ||
        "sidebar"
      );
    } catch {
      return "sidebar";
    }
  });

  useEffect(() => {
    const handler = () => {
      try {
        const mode =
          (localStorage.getItem("app.layoutMode") as "navbar" | "sidebar") ||
          "sidebar";
        setLayoutMode(mode);
      } catch {}
    };
    window.addEventListener("app-layout-changed", handler);
    return () => window.removeEventListener("app-layout-changed", handler);
  }, []);

  return { layoutMode };
};

/** Shell chrome sizes in rem, so they follow the fluid root font size. */
export const SHELL_SIZE = {
  TOPBAR_REM: 3.5,
  RAIL_REM: 3.5,
  RAIL_EXPANDED_REM: 13,
} as const;

const RAIL_EXPANDED_KEY = "ll.sidebar.expanded";

const readRailPreference = (): boolean | null => {
  try {
    const value = localStorage.getItem(RAIL_EXPANDED_KEY);
    return value === "true" ? true : value === "false" ? false : null;
  } catch {
    return null;
  }
};

const matchesQuery = (query: string) => {
  try {
    return window.matchMedia(query).matches;
  } catch {
    return false;
  }
};

export const useMediaQuery = (query: string) => {
  const [value, setValue] = useState(() => matchesQuery(query));

  useEffect(() => {
    let media: MediaQueryList;
    try {
      media = window.matchMedia(query);
    } catch {
      return;
    }
    const update = () => setValue(media.matches);
    update();
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, [query]);

  return value;
};

/** Whether the sidebar shows labels. It starts as the icon rail released
 * versions had, and keeps the user's choice once they toggle it. */
export const useRailExpanded = () => {
  const [preference, setPreference] = useState(readRailPreference);
  const expanded = preference ?? false;

  const toggle = useCallback(() => {
    const next = !expanded;
    setPreference(next);
    try {
      localStorage.setItem(RAIL_EXPANDED_KEY, String(next));
    } catch {}
  }, [expanded]);

  return { expanded, toggle };
};
