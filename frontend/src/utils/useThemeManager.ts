import { useState, useEffect, useCallback } from "react";
import { useTheme } from "next-themes";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";

export type ThemeMode = "light" | "dark" | "schedule" | "auto" | "system";

export interface SunTimes {
  sunrise: string;
  sunset: string;
  ip?: string;
}

export const useThemeManager = () => {
  const { theme, setTheme, resolvedTheme } = useTheme();

  const [themeMode, setThemeMode] = useState<ThemeMode>(() => {
    try {
      return (localStorage.getItem("app.themeMode") as ThemeMode) || "light";
    } catch {
      return "light";
    }
  });

  const [scheduleStart, setScheduleStart] = useState<string>(() => {
    try {
      return localStorage.getItem("app.scheduleStart") || "20:00";
    } catch {
      return "20:00";
    }
  });

  const [scheduleEnd, setScheduleEnd] = useState<string>(() => {
    try {
      return localStorage.getItem("app.scheduleEnd") || "07:00";
    } catch {
      return "07:00";
    }
  });

  const [sunTimes, setSunTimes] = useState<SunTimes | null>(() => {
    try {
      const sunrise = localStorage.getItem("app.sunrise");
      const sunset = localStorage.getItem("app.sunset");
      const ip = localStorage.getItem("app.sunIP") || undefined;
      if (sunrise && sunset) {
        return { sunrise, sunset, ip };
      }
    } catch {}
    return null;
  });

  const [isThemeOverridden, setIsThemeOverridden] = useState(false);

  const fetchSunTimes = useCallback(
    async (isRetry = false) => {
      if (themeMode !== "auto") return;
      try {
        const res = await (minecraft as any).GetSunTimes();
        if (res) {
          if (res.sunrise === "06:00" && res.sunset === "18:00" && !isRetry) {
            setTimeout(() => fetchSunTimes(true), 5000);
          }
          setSunTimes(res);
          localStorage.setItem("app.sunrise", res.sunrise);
          localStorage.setItem("app.sunset", res.sunset);
          if (res.ip) localStorage.setItem("app.sunIP", res.ip);
        }
      } catch (err) {
        console.error("Failed to fetch sun times:", err);
        if (!isRetry) setTimeout(() => fetchSunTimes(true), 5000);
      }
    },
    [themeMode],
  );

  const updateThemeMode = useCallback((mode: ThemeMode) => {
    setThemeMode(mode);
    localStorage.setItem("app.themeMode", mode);
    setIsThemeOverridden(false);
    window.dispatchEvent(new CustomEvent("app-theme-mode-changed"));
  }, []);

  const toggleTheme = useCallback(() => {
    const nextTheme = resolvedTheme === "dark" ? "light" : "dark";
    setTheme(nextTheme);
    localStorage.setItem("app.themeMode", nextTheme);
    setThemeMode(nextTheme as ThemeMode);
    setIsThemeOverridden(true);
    window.dispatchEvent(new CustomEvent("app-theme-mode-changed"));
    window.dispatchEvent(new CustomEvent("app-theme-overridden"));
  }, [resolvedTheme, setTheme]);

  const checkTheme = useCallback(() => {
    if (isThemeOverridden) return;

    let targetTheme = "light";

    if (themeMode === "schedule") {
      const now = new Date();
      const currentTime = now.getHours() * 60 + now.getMinutes();
      const [startH, startM] = scheduleStart.split(":").map(Number);
      const [endH, endM] = scheduleEnd.split(":").map(Number);
      const startTime = startH * 60 + startM;
      const endTime = endH * 60 + endM;

      if (startTime < endTime) {
        if (currentTime >= startTime && currentTime < endTime)
          targetTheme = "dark";
      } else {
        if (currentTime >= startTime || currentTime < endTime)
          targetTheme = "dark";
      }
    } else if (themeMode === "auto") {
      const now = new Date();
      const currentTime = now.getHours() * 60 + now.getMinutes();
      let sunrise = 6 * 60;
      let sunset = 18 * 60;

      if (sunTimes) {
        const [srH, srM] = sunTimes.sunrise.split(":").map(Number);
        const [ssH, ssM] = sunTimes.sunset.split(":").map(Number);
        sunrise = srH * 60 + srM;
        sunset = ssH * 60 + ssM;
      }

      if (sunrise < sunset) {
        if (currentTime < sunrise || currentTime >= sunset)
          targetTheme = "dark";
      } else if (sunrise > sunset) {
        if (currentTime >= sunset && currentTime < sunrise)
          targetTheme = "dark";
      } else {
        targetTheme = "dark";
      }
    } else if (themeMode === "system") {
      targetTheme = window.matchMedia("(prefers-color-scheme: dark)").matches
        ? "dark"
        : "light";
    } else if (themeMode === "light" || themeMode === "dark") {
      targetTheme = themeMode;
    }

    if (theme !== targetTheme) {
      setTheme(targetTheme);
    }
  }, [
    themeMode,
    scheduleStart,
    scheduleEnd,
    sunTimes,
    isThemeOverridden,
    theme,
    setTheme,
  ]);

  useEffect(() => {
    const handler = () => {
      try {
        const mode =
          (localStorage.getItem("app.themeMode") as ThemeMode) || "light";
        setThemeMode(mode);
        setScheduleStart(localStorage.getItem("app.scheduleStart") || "20:00");
        setScheduleEnd(localStorage.getItem("app.scheduleEnd") || "07:00");
        setIsThemeOverridden(false);

        const sr = localStorage.getItem("app.sunrise");
        const ss = localStorage.getItem("app.sunset");
        if (sr && ss)
          setSunTimes({
            sunrise: sr,
            sunset: ss,
            ip: localStorage.getItem("app.sunIP") || undefined,
          });
      } catch {}
    };
    window.addEventListener("app-theme-mode-changed", handler);
    return () => window.removeEventListener("app-theme-mode-changed", handler);
  }, []);

  useEffect(() => {
    const handler = () => setIsThemeOverridden(true);
    window.addEventListener("app-theme-overridden", handler);
    return () => window.removeEventListener("app-theme-overridden", handler);
  }, []);

  useEffect(() => {
    fetchSunTimes();
    if (themeMode === "auto") {
      const timer = setInterval(() => fetchSunTimes(), 3600 * 1000);
      return () => clearInterval(timer);
    }
  }, [themeMode, fetchSunTimes]);

  useEffect(() => {
    checkTheme();
    const interval = setInterval(checkTheme, 5000);

    const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
    const handler = () => {
      if (themeMode === "system") {
        checkTheme();
      }
    };
    mediaQuery.addEventListener("change", handler);

    return () => {
      clearInterval(interval);
      mediaQuery.removeEventListener("change", handler);
    };
  }, [checkTheme, themeMode]);

  return {
    themeMode,
    setThemeMode: updateThemeMode,
    scheduleStart,
    setScheduleStart,
    scheduleEnd,
    setScheduleEnd,
    sunTimes,
    fetchSunTimes,
    toggleTheme,
    isThemeOverridden,
    resolvedTheme,
  };
};
