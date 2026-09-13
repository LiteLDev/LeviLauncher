import { useLayoutEffect } from "react";
import {
  getAppearanceStyle,
  type AppearanceMode,
  type BackgroundAppearance,
} from "@/utils/backgroundAppearance";

/** Publish the same material to the app and body portals before the next paint. */
export const useDocumentAppearance = (
  active: boolean,
  appearance: BackgroundAppearance,
  mode: AppearanceMode,
  brightness: number,
  blur: number,
  opacity: number,
) => {
  useLayoutEffect(() => {
    const root = document.documentElement;
    const attributes = {
      "data-material-scope": "",
      "data-wallpaper-active": String(active),
      "data-readability": String(appearance.readability),
    };
    const style = getAppearanceStyle(
      appearance,
      mode,
      brightness,
      blur,
      opacity,
    );
    const previousAttributes = Object.keys(attributes).map(
      (key) => [key, root.getAttribute(key)] as const,
    );
    const previousStyle = Object.keys(style).map(
      (key) => [key, root.style.getPropertyValue(key)] as const,
    );
    Object.entries(attributes).forEach(([key, value]) =>
      root.setAttribute(key, value),
    );
    Object.entries(style).forEach(([key, value]) =>
      root.style.setProperty(key, String(value)),
    );
    return () => {
      previousAttributes.forEach(([key, value]) =>
        value === null
          ? root.removeAttribute(key)
          : root.setAttribute(key, value),
      );
      previousStyle.forEach(([key, value]) =>
        value
          ? root.style.setProperty(key, value)
          : root.style.removeProperty(key),
      );
    };
  }, [active, appearance, mode, brightness, blur, opacity]);
};
