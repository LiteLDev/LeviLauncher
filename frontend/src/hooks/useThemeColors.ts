import { useState, useEffect } from "react";
import { THEMES, hexToRgb, generateTheme } from "@/constants/themes";

const getContrastingForeground = (hex: string): string => {
  if (!/^#[0-9a-f]{6}$/i.test(hex)) {
    return "0 0% 0%";
  }

  const channels = [1, 3, 5].map((offset) => {
    const value = Number.parseInt(hex.slice(offset, offset + 2), 16) / 255;
    return value <= 0.03928
      ? value / 12.92
      : Math.pow((value + 0.055) / 1.055, 2.4);
  });
  const luminance =
    channels[0] * 0.2126 + channels[1] * 0.7152 + channels[2] * 0.0722;
  const blackContrast = (luminance + 0.05) / 0.05;
  const whiteContrast = 1.05 / (luminance + 0.05);

  return blackContrast >= whiteContrast ? "0 0% 0%" : "0 0% 100%";
};

export const useThemeColors = (resolvedTheme: string | undefined) => {
  const [themeColorsReady, setThemeColorsReady] = useState<boolean>(false);
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
    root.style.setProperty(
      "--heroui-primary-foreground",
      getContrastingForeground(theme[500]),
    );
    setThemeColorsReady(true);
  }, [
    resolvedTheme,
    lightThemeColor,
    lightCustomThemeColor,
    darkThemeColor,
    darkCustomThemeColor,
  ]);

  return { themeColorsReady };
};
