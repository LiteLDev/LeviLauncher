import { getFitStyles } from "@/hooks/useBackgroundImage";
import type { BackgroundState } from "@/utils/BackgroundContext";
import type { AppearanceMode } from "@/utils/backgroundAppearance";

export const BackgroundLayers = ({
  background,
  mode,
}: {
  background: BackgroundState;
  mode: AppearanceMode;
}) => {
  const {
    bgData,
    backgroundFitMode,
    backgroundBlur,
    backgroundBrightness,
    backgroundOpacity,
  } = background;
  const baseMode =
    mode === "light"
      ? background.lightBackgroundBaseMode
      : background.darkBackgroundBaseMode;
  const baseColor =
    mode === "light"
      ? background.lightBackgroundBaseColor
      : background.darkBackgroundBaseColor;
  const baseOpacity =
    mode === "light"
      ? background.lightBackgroundBaseOpacity
      : background.darkBackgroundBaseOpacity;
  return (
    <div className="wallpaper-layers" aria-hidden="true">
      {baseMode !== "none" && (
        <div
          className="absolute inset-0"
          style={{
            backgroundColor:
              baseMode === "theme"
                ? `rgb(var(--theme-${mode === "light" ? "50" : "900"}))`
                : baseColor,
            opacity: baseOpacity / 100,
          }}
        />
      )}
      {bgData && backgroundOpacity > 0 && (
        <>
          <div
            className="wallpaper-image absolute"
            style={{
              // Extend the filtered layer so blur does not expose a pale frame.
              inset: -backgroundBlur * 2,
              backgroundImage: `url(${JSON.stringify(bgData)})`,
              ...getFitStyles(backgroundFitMode),
              filter: `blur(${backgroundBlur}px) brightness(${backgroundBrightness}%)`,
              opacity: backgroundOpacity / 100,
            }}
          />
          <div className="wallpaper-scrim absolute inset-0" />
        </>
      )}
    </div>
  );
};
