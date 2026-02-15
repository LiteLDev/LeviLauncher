import { useState, useEffect } from "react";
import { THEMES, hexToRgb, generateTheme } from "@/constants/themes";

export const useThemeColors = (resolvedTheme: string | undefined) => {
  const [lightThemeColor, setLightThemeColor] = useState<string>(() => {
    try {
      const saved = localStorage.getItem("app.lightThemeColor");
      if (saved) return saved;
      const old = localStorage.getItem("app.themeColor");
      if (old === "rose") return "pink";
      return old || "emerald";
    } catch {
      return "emerald";
    }
  });

  const [lightCustomThemeColor, setLightCustomThemeColor] = useState<string>(
    () => {
      try {
        return (
          localStorage.getItem("app.lightCustomThemeColor") ||
          localStorage.getItem("app.customThemeColor") ||
          "#10b981"
        );
      } catch {
        return "#10b981";
      }
    },
  );

  const [darkThemeColor, setDarkThemeColor] = useState<string>(() => {
    try {
      const saved = localStorage.getItem("app.darkThemeColor");
      if (saved) return saved;
      const old = localStorage.getItem("app.themeColor");
      if (old === "rose") return "pink";
      return old || "emerald";
    } catch {
      return "emerald";
    }
  });

  const [darkCustomThemeColor, setDarkCustomThemeColor] = useState<string>(
    () => {
      try {
        return (
          localStorage.getItem("app.darkCustomThemeColor") ||
          localStorage.getItem("app.customThemeColor") ||
          "#10b981"
        );
      } catch {
        return "#10b981";
      }
    },
  );

  useEffect(() => {
    const handler = () => {
      try {
        const lightColor =
          localStorage.getItem("app.lightThemeColor") ||
          localStorage.getItem("app.themeColor") ||
          "emerald";
        setLightThemeColor(lightColor);
        const lightCustom =
          localStorage.getItem("app.lightCustomThemeColor") ||
          localStorage.getItem("app.customThemeColor") ||
          "#10b981";
        setLightCustomThemeColor(lightCustom);

        const darkColor =
          localStorage.getItem("app.darkThemeColor") ||
          localStorage.getItem("app.themeColor") ||
          "emerald";
        setDarkThemeColor(darkColor);
        const darkCustom =
          localStorage.getItem("app.darkCustomThemeColor") ||
          localStorage.getItem("app.customThemeColor") ||
          "#10b981";
        setDarkCustomThemeColor(darkCustom);
      } catch {}
    };
    window.addEventListener("app-theme-changed", handler);
    return () => window.removeEventListener("app-theme-changed", handler);
  }, []);

  useEffect(() => {
    const isDark = resolvedTheme === "dark";
    const currentColor = isDark ? darkThemeColor : lightThemeColor;
    const currentCustomColor = isDark
      ? darkCustomThemeColor
      : lightCustomThemeColor;

    let theme = THEMES[currentColor];
    if (currentColor === "custom") {
      theme = generateTheme(currentCustomColor);
    }
    if (!theme) theme = THEMES.emerald;

    const root = document.documentElement;

    Object.keys(theme).forEach((key) => {
      const k = Number(key);
      root.style.setProperty(`--theme-${k}`, hexToRgb(theme[k]));
    });
  }, [
    resolvedTheme,
    lightThemeColor,
    lightCustomThemeColor,
    darkThemeColor,
    darkCustomThemeColor,
  ]);
};
