import type { CSSProperties } from "react";

export type AppearanceMode = "light" | "dark";
export type MaterialPreset = "balanced" | "clear" | "solid";
export interface BackgroundAppearance {
  surfaceOpacity: number;
  surfaceBlur: number;
  overlayOpacity: number;
  readability: boolean;
}

export const APPEARANCE_KEY = "app.backgroundAppearance.v1";
export const APPEARANCE_EVENT = "app-background-appearance-changed";

export const clampNumber = (
  value: unknown,
  fallback: number,
  min: number,
  max: number,
) => {
  if (value === null || value === undefined || value === "") return fallback;
  const number = Number(value);
  return Number.isFinite(number)
    ? Math.min(max, Math.max(min, number))
    : fallback;
};

export const getMaterialPreset = (
  mode: AppearanceMode,
  preset: MaterialPreset = "balanced",
): BackgroundAppearance => ({
  surfaceOpacity: preset === "solid" ? 100 : preset === "clear" ? 35 : 60,
  surfaceBlur: preset === "solid" ? 0 : preset === "clear" ? 4 : 12,
  overlayOpacity: mode === "dark" ? 18 : 8,
  readability: preset !== "clear",
});

const normalizeAppearance = (
  value: Partial<BackgroundAppearance> | null,
  mode: AppearanceMode,
): BackgroundAppearance => {
  const defaults = getMaterialPreset(mode);
  return {
    surfaceOpacity: clampNumber(
      value?.surfaceOpacity,
      defaults.surfaceOpacity,
      0,
      100,
    ),
    surfaceBlur: clampNumber(value?.surfaceBlur, defaults.surfaceBlur, 0, 32),
    overlayOpacity: clampNumber(
      value?.overlayOpacity,
      defaults.overlayOpacity,
      0,
      80,
    ),
    readability:
      typeof value?.readability === "boolean"
        ? value.readability
        : defaults.readability,
  };
};

export const readBackgroundAppearance = (): Record<
  AppearanceMode,
  BackgroundAppearance
> => {
  let stored;
  try {
    stored = JSON.parse(localStorage.getItem(APPEARANCE_KEY) || "null");
  } catch {
    /* Invalid or unavailable storage uses safe defaults. */
  }
  return {
    light: normalizeAppearance(stored?.light, "light"),
    dark: normalizeAppearance(stored?.dark, "dark"),
  };
};

// Protect against the darkest/lightest possible pixel, including high-contrast
// images. No image upload, canvas sampling or per-frame analysis is needed.
export const getEffectiveSurfaceOpacity = (
  appearance: BackgroundAppearance,
  mode: AppearanceMode,
  brightness: number,
  imageBlur: number,
  imageOpacity = 100,
) => {
  if (!appearance.readability) return appearance.surfaceOpacity / 100;
  const overlay = appearance.overlayOpacity / 100;
  const detailGuard = imageBlur + appearance.surfaceBlur < 8 ? 0.06 : 0;
  // A translucent image can expose a custom bright base even at 0% brightness.
  const brightestBackdrop =
    ((Math.min(1, brightness / 100) * imageOpacity) / 100 +
      1 -
      imageOpacity / 100) *
    (1 - overlay);
  const floor =
    mode === "light"
      ? (0.68 + detailGuard - overlay) / (1 - overlay)
      : (brightestBackdrop - (0.36 - detailGuard)) /
        Math.max(0.01, brightestBackdrop - 27 / 255);
  return Math.max(
    appearance.surfaceOpacity / 100,
    Math.min(1, Math.max(0, floor)),
  );
};

export const getAppearanceStyle = (
  appearance: BackgroundAppearance,
  mode: AppearanceMode,
  brightness: number,
  imageBlur: number,
  imageOpacity = 100,
): CSSProperties =>
  ({
    "--wallpaper-surface-opacity": getEffectiveSurfaceOpacity(
      appearance,
      mode,
      brightness,
      imageBlur,
      imageOpacity,
    ),
    "--wallpaper-surface-blur": `${appearance.surfaceBlur}px`,
    "--wallpaper-overlay-opacity": appearance.overlayOpacity / 100,
    "--wallpaper-surface-rgb": mode === "dark" ? "24 24 27" : "255 255 255",
    "--wallpaper-control-rgb": mode === "dark" ? "63 63 70" : "228 228 231",
    "--wallpaper-ink": mode === "dark" ? "#fafafa" : "#27272a",
    "--wallpaper-muted": mode === "dark" ? "#e4e4e7" : "#3f3f46",
  }) as CSSProperties;
