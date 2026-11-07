import { useState, useEffect } from "react";

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

/** Viewport width from which the sidebar shows labels instead of icons only.
 * Media queries ignore the root font size, so this stays at 1280 CSS px. */
export const RAIL_EXPANDED_QUERY = "(min-width: 1280px)";

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

export const useRailExpanded = () => useMediaQuery(RAIL_EXPANDED_QUERY);
