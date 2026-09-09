import { Button, Label, Slider, Switch } from "@heroui/react";
import { useTranslation } from "react-i18next";
import { LuCheck, LuLayers, LuMoon, LuRotateCcw, LuSun } from "react-icons/lu";
import { useBackgroundAppearance } from "@/hooks/useBackgroundAppearance";
import { useCurrentBackground } from "@/utils/BackgroundContext";
import {
  getAppearanceStyle,
  getEffectiveSurfaceOpacity,
  getMaterialPreset,
  type AppearanceMode,
  type MaterialPreset,
} from "@/utils/backgroundAppearance";
import { BackgroundLayers } from "./BackgroundLayers";

const PRESETS: MaterialPreset[] = ["balanced", "clear", "solid"];

export const BackgroundAppearanceSettings = ({
  mode,
  onModeChange,
}: {
  mode: AppearanceMode;
  onModeChange: (mode: AppearanceMode) => void;
}) => {
  const { t } = useTranslation();
  const { profiles, updateProfile } = useBackgroundAppearance();
  const background = useCurrentBackground();
  const profile = profiles[mode];
  const hasImage =
    Boolean(background?.bgData) && (background?.backgroundOpacity ?? 0) > 0;
  const effectiveOpacity = getEffectiveSurfaceOpacity(
    profile,
    mode,
    background?.backgroundBrightness ?? 100,
    background?.backgroundBlur ?? 0,
    background?.backgroundOpacity ?? 100,
  );
  const preset = PRESETS.find((name) => {
    const values = getMaterialPreset(mode, name);
    return Object.keys(values).every(
      (key) =>
        values[key as keyof typeof values] ===
        profile[key as keyof typeof profile],
    );
  });
  const copy = (key: string) => t(`settings.appearance.material.${key}`);

  return (
    <section
      className="flex flex-col gap-5 rounded-2xl border border-border/60 p-4 sm:p-5"
      aria-label={copy("title")}
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <LuLayers size={18} className="text-brand-600 dark:text-brand-300" />
          <h3 className="text-sm font-bold">{copy("title")}</h3>
        </div>
        <div className="flex gap-1" role="group" aria-label={copy("edit_mode")}>
          {(["light", "dark"] as const).map((value) => (
            <Button
              key={value}
              size="sm"
              variant={mode === value ? "primary" : "secondary"}
              aria-pressed={mode === value}
              onPress={() => onModeChange(value)}
            >
              {value === "light" ? <LuSun /> : <LuMoon />}
              {t(`settings.appearance.theme_${value}`)}
            </Button>
          ))}
        </div>
      </div>
      <p className="text-xs text-muted">{copy("description")}</p>

      {background && (
        <div
          className={`appearance-preview ${mode}`}
          data-testid="appearance-preview"
          data-material-scope=""
          data-wallpaper-active={hasImage ? "true" : undefined}
          data-readability={profile.readability ? "true" : undefined}
          style={{
            ...getAppearanceStyle(
              profile,
              mode,
              background.backgroundBrightness,
              background.backgroundBlur,
              background.backgroundOpacity,
            ),
            ...(!hasImage ? { "--wallpaper-surface-opacity": 1 } : {}),
          }}
          role="img"
          aria-label={copy("preview_label")}
        >
          <BackgroundLayers background={background} mode={mode} />
          <div
            className="relative flex flex-col gap-3 p-4 sm:p-5"
            aria-hidden="true"
          >
            <div className="flex items-center justify-between gap-2 text-xs font-semibold">
              <span className="launcher-chrome rounded-full px-3 py-1.5">
                {copy("preview")}
              </span>
              <span className="launcher-chrome rounded-full px-3 py-1.5">
                {t(`settings.appearance.theme_${mode}`)}
              </span>
            </div>
            <div className="launcher-glass rounded-2xl border border-border/50 p-4">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <p className="text-xl font-black">Minecraft</p>
                  <p className="text-xs text-muted">Bedrock Edition</p>
                </div>
                <span className="rounded-full bg-foreground px-4 py-2 text-xs font-bold text-background">
                  {copy("preview_action")}
                </span>
              </div>
              <p className="mt-4 text-xs text-muted">{copy("preview_text")}</p>
              <div className="mt-3 flex gap-1 rounded-xl bg-default p-1 text-xs">
                <span className="rounded-lg bg-segment px-3 py-1.5">
                  {t("settings.tabs.personalization")}
                </span>
                <span className="px-3 py-1.5 text-muted">
                  {t("settings.tabs.general")}
                </span>
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              {["moddedcard.title", "contentdownload.title"].map((key) => (
                <div
                  key={key}
                  className="launcher-glass rounded-xl border border-border/50 p-3 text-xs font-semibold"
                >
                  {t(key)}
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {!hasImage && (
        <p className="text-xs text-muted" role="status">
          {copy("no_image")}
        </p>
      )}
      <div
        className="grid grid-cols-1 gap-2 sm:grid-cols-3"
        role="group"
        aria-label={copy("presets")}
      >
        {PRESETS.map((name) => (
          <Button
            key={name}
            variant="secondary"
            aria-pressed={preset === name}
            onPress={() => updateProfile(mode, getMaterialPreset(mode, name))}
            className={`h-auto min-h-16 w-full justify-start whitespace-normal rounded-xl border p-3 text-left ${preset === name ? "border-brand-500 bg-accent-soft" : "border-border/60"}`}
          >
            <span className="flex-1">
              <span className="block text-sm font-semibold">{copy(name)}</span>
              <span className="mt-1 block text-xs font-normal text-muted">
                {copy(`${name}_desc`)}
              </span>
            </span>
            {preset === name && (
              <LuCheck className="shrink-0" aria-hidden="true" />
            )}
          </Button>
        ))}
      </div>
      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2">
        {(
          [
            ["surfaceOpacity", "opacity", 100, "%"],
            ["surfaceBlur", "blur", 32, "px"],
            ["overlayOpacity", "overlay", 80, "%"],
          ] as const
        ).map(([key, label, max, unit]) => (
          <Slider
            key={key}
            minValue={0}
            maxValue={max}
            step={1}
            value={profile[key]}
            onChange={(value) => updateProfile(mode, { [key]: Number(value) })}
            aria-label={copy(label)}
          >
            <div className="mb-2 flex items-center justify-between gap-3 text-xs">
              <Label>{copy(label)}</Label>
              <Slider.Output className="tabular-nums">
                {profile[key]}
                {unit}
              </Slider.Output>
            </div>
            <Slider.Track>
              <Slider.Fill className="bg-brand-500" />
              <Slider.Thumb className="bg-brand-500" />
            </Slider.Track>
          </Slider>
        ))}
      </div>
      <div className="flex items-start justify-between gap-4 border-t border-border/50 pt-4">
        <div className="min-w-0">
          <p className="text-sm font-medium">{copy("readability")}</p>
          <p className="mt-1 text-xs text-muted">{copy("readability_desc")}</p>
        </div>
        <Switch
          size="sm"
          aria-label={copy("readability")}
          isSelected={profile.readability}
          onChange={(readability) => updateProfile(mode, { readability })}
        >
          <Switch.Content>
            <Switch.Control>
              <Switch.Thumb />
            </Switch.Control>
          </Switch.Content>
        </Switch>
      </div>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-xs text-muted" role="status">
          {hasImage && effectiveOpacity > profile.surfaceOpacity / 100 + 0.005
            ? t("settings.appearance.material.protected_opacity", {
                value: Math.round(effectiveOpacity * 100),
              })
            : copy(profile.readability ? "protected" : "unprotected")}
        </p>
        <Button
          size="sm"
          variant="ghost"
          onPress={() => updateProfile(mode, getMaterialPreset(mode))}
        >
          <LuRotateCcw />
          {copy("reset")}
        </Button>
      </div>
    </section>
  );
};
