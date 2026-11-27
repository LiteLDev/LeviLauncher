import React from "react";
import { Button, Input, Label, ListBox, Select, Spinner, Switch, TextField, toast } from "@heroui/react";
import { Dialogs, System } from "@wailsio/runtime";
import { useTranslation } from "react-i18next";
import { LuChevronDown, LuCircleAlert } from "react-icons/lu";
import {
  CheckWineSettings, ClearWineShaderCache, GetWineSettings, GetWineShaderCacheInfo,
  KillWineProcesses, SaveWineSettings,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import type {
  WineSettings as Settings, WineShaderCacheInfo,
} from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { formatBytes } from "@/utils/formatting";

export function WineSettings({ locked, onLoaded }: { locked: boolean; onLoaded: (settings: Settings) => void }) {
  const { t } = useTranslation();
  const [settings, setSettings] = React.useState<Settings | null>(null);
  const [loading, setLoading] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const [checking, setChecking] = React.useState(false);
  const [clearingCache, setClearingCache] = React.useState(false);
  const [killingWine, setKillingWine] = React.useState(false);
  const [shaderCache, setShaderCache] = React.useState<WineShaderCacheInfo | null>(null);
  const [error, setError] = React.useState("");
  const [result, setResult] = React.useState("");
  const lastSavedRef = React.useRef<Settings | null>(null);
  const saveSeqRef = React.useRef(0);
  const disabled = loading || locked;

  const reportError = (failure: unknown) => setError(failure instanceof Error ? failure.message : String(failure));

  const loadShaderCache = React.useCallback(async () => {
    try {
      setShaderCache(await GetWineShaderCacheInfo());
    } catch {
      setShaderCache(null);
    }
  }, []);

  React.useEffect(() => {
    let active = true;
    setLoading(true);
    setError("");
    setResult("");
    GetWineSettings()
      .then((current) => {
        if (!active) return;
        lastSavedRef.current = current;
        setSettings(current);
        onLoaded(current);
      })
      .catch((failure: unknown) => {
        if (active) reportError(failure);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    void loadShaderCache();
    return () => { active = false; };
  }, [loadShaderCache, onLoaded]);

  const persist = React.useCallback(async (next: Settings, keepDraftOnError = false) => {
    const seq = ++saveSeqRef.current;
    setSettings(next);
    setSaving(true);
    setResult("");
    setError("");
    try {
      await SaveWineSettings(next);
      const current = await GetWineSettings();
      if (seq !== saveSeqRef.current) return;
      lastSavedRef.current = current;
      setSettings(current);
      onLoaded(current);
    } catch (failure) {
      if (seq !== saveSeqRef.current) return;
      reportError(failure);
      if (!keepDraftOnError && lastSavedRef.current) {
        setSettings(lastSavedRef.current);
      }
    } finally {
      if (seq === saveSeqRef.current) setSaving(false);
    }
  }, [onLoaded]);

  const applyImmediate = (patch: Partial<Settings>) => {
    if (!settings) return;
    void persist({ ...settings, ...patch }, false);
  };

  const buildPathPatch = (field: "app" | "executable" | "prefix", path: string): Partial<Settings> => {
    if (field === "app") {
      return {
        app: path,
        runtime: undefined,
        d3dMetalVersions: undefined,
        ...(path.trim() ? { executable: "", prefix: "", graphics: "auto", d3dMetalVersion: "auto" } : {}),
      };
    }
    return { [field]: path };
  };

  const changePathDraft = (field: "app" | "executable" | "prefix", path: string) => {
    setSettings((previous) => previous ? { ...previous, ...buildPathPatch(field, path) } : previous);
    setResult("");
    setError("");
  };

  const commitPathDraft = () => {
    if (!settings) return;
    if (lastSavedRef.current &&
      settings.app === lastSavedRef.current.app &&
      settings.executable === lastSavedRef.current.executable &&
      settings.prefix === lastSavedRef.current.prefix) {
      return;
    }
    void persist(settings, true);
  };

  const choose = async (field: "app" | "executable" | "prefix") => {
    if (!settings) return;
    setError("");
    try {
      const selected = await Dialogs.OpenFile({
        Title: t(`settings.wine.${field}`),
        CanChooseFiles: field !== "prefix",
        CanChooseDirectories: field !== "executable",
        TreatsFilePackagesAsDirectories: false,
        ShowHiddenFiles: true,
        AllowsMultipleSelection: false,
      });
      const path = Array.isArray(selected) ? selected[0] : selected;
      if (path) {
        void persist({ ...settings, ...buildPathPatch(field, path) }, true);
      }
    } catch (failure) { reportError(failure); }
  };

  const checkVersion = async () => {
    if (!settings) return;
    setChecking(true);
    setError("");
    setResult("");
    try {
      const version = await CheckWineSettings(settings);
      setResult(t("settings.wine.checked", { version }));
    } catch (failure) { reportError(failure); }
    finally { setChecking(false); }
  };

  const clearShaderCache = async () => {
    setClearingCache(true);
    setError("");
    try {
      await ClearWineShaderCache();
      await loadShaderCache();
      toast.success(t("settings.wine.shader_cache_cleared"));
    } catch (failure) { reportError(failure); }
    finally { setClearingCache(false); }
  };

  const killWine = async () => {
    setKillingWine(true);
    setError("");
    try {
      await KillWineProcesses();
      toast.success(t("settings.wine.kill_wine_done"));
    } catch (failure) { reportError(failure); }
    finally { setKillingWine(false); }
  };

  const pathField = (field: "app" | "executable" | "prefix") => (
    <div className="flex min-w-0 items-end gap-2" key={field}>
      <TextField
        className="min-w-0 flex-1"
        value={settings?.[field] || ""}
        onChange={(value) => changePathDraft(field, value)}
        isDisabled={disabled}
      >
        <Label>{t(`settings.wine.${field}`)}</Label>
        <Input
          spellCheck={false}
          onBlur={commitPathDraft}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              event.preventDefault();
              commitPathDraft();
            }
          }}
        />
      </TextField>
      <Button variant="outline" className="shrink-0" isDisabled={disabled} onPress={() => void choose(field)} aria-label={t("settings.wine.choose_path", { field: t(`settings.wine.${field}`) })}>
        {t("settings.wine.choose")}
      </Button>
    </div>
  );

  const mac = System.IsMac();
  const metalSharp = settings?.runtime === "metalsharp";
  const d3dMetalVersions = settings?.d3dMetalVersions?.length ? settings.d3dMetalVersions : ["auto"];
  const hasBundledD3DMetal = metalSharp && d3dMetalVersions.length > 1;
  const effectiveGraphics = metalSharp && settings?.graphics === "auto"
    ? (hasBundledD3DMetal ? "d3dmetal" : "mullion")
    : settings?.graphics;
  const isD3DMetal = metalSharp && effectiveGraphics === "d3dmetal";
  const supportsGraphicsTuning = metalSharp && ["mullion", "d3dmetal", "vkd3d"].includes(effectiveGraphics || "");
  const selectedD3DMetalVersion = settings?.d3dMetalVersion && settings.d3dMetalVersion !== "auto"
    ? settings.d3dMetalVersion : d3dMetalVersions[1];
  const d3dMetal4 = isD3DMetal && !!settings?.enableMetal4 &&
    selectedD3DMetalVersion === "gptk4" && d3dMetalVersions.includes("gptk4");
  const supportsSpatialScale = supportsGraphicsTuning && !d3dMetal4;
  const graphics = metalSharp ? ["auto", "d3dmetal", "mullion", "vkd3d", "wined3d"] : ["auto", "d3dmetal", "wined3d"];
  const showD3DMetalVersion = isD3DMetal && d3dMetalVersions.length > 1;
  const maxFps = settings?.maxFps ?? 0;
  const frameRates = [...new Set([0, 30, 60, 90, 120, 144, 165, 240, maxFps])].sort((a, b) => a - b);
  const vsync = settings?.vsync || "auto";
  const metalFxScale = settings?.metalFxScale || (settings?.enableMetalFx ? (settings?.retinaMode ? "performance" : "balanced") : "off");
  const threadQos = settings?.threadQos || "balanced";

  const handleRetinaChange = (retinaMode: boolean) => {
    if (!settings) return;
    if (retinaMode) {
      if (supportsSpatialScale && metalFxScale === "off") {
        applyImmediate({ retinaMode: true, metalFxScale: "performance", enableMetalFx: true });
        return;
      }
      applyImmediate({ retinaMode: true });
      return;
    }
    if (supportsSpatialScale && metalFxScale === "performance") {
      applyImmediate({ retinaMode: false, metalFxScale: "off", enableMetalFx: false });
      return;
    }
    applyImmediate({ retinaMode: false });
  };

  type DisplayPairingAdvice = {
    severity: "warning" | "info";
    messageKey: string;
    canUsePerformance?: boolean;
    canDisableRetina?: boolean;
  };

  const displayPairingAdvice = React.useMemo<DisplayPairingAdvice | null>(() => {
    if (!mac || !settings) return null;
    if (d3dMetal4) {
      return { severity: "info", messageKey: "settings.wine.d3dmetal4_metalfx_unavailable" };
    }
    if (settings.retinaMode) {
      if (supportsSpatialScale) {
        if (metalFxScale === "off") {
          return { severity: "warning", messageKey: "settings.wine.retina_off_warn", canUsePerformance: true, canDisableRetina: true };
        }
        if (metalFxScale === "quality") {
          return { severity: "warning", messageKey: "settings.wine.retina_quality_warn", canUsePerformance: true, canDisableRetina: true };
        }
        if (metalFxScale === "balanced") {
          return { severity: "info", messageKey: "settings.wine.retina_balanced_tip" };
        }
        if (metalFxScale === "performance" || metalFxScale === "ultra_performance" || metalFxScale === "extreme" || metalFxScale === "max") {
          return {
            severity: "info",
            messageKey: isD3DMetal ? "settings.wine.d3dmetal_metalfx_tip" : "settings.wine.retina_performance_tip",
          };
        }
      }
      return { severity: "warning", messageKey: "settings.wine.retina_no_upscale_warn", canDisableRetina: true };
    }
    if (isD3DMetal && metalFxScale !== "off") {
      return { severity: "info", messageKey: "settings.wine.d3dmetal_metalfx_tip" };
    }
    if (supportsSpatialScale && metalFxScale === "performance") {
      return { severity: "info", messageKey: "settings.wine.standard_performance_tip" };
    }
    return null;
  }, [d3dMetal4, isD3DMetal, mac, metalFxScale, settings, supportsSpatialScale]);

  const selectField = (
    field: "graphics" | "d3dmetal_version" | "sync" | "vsync" | "max_fps" | "metal_fx" | "thread_qos",
    value: string,
    choices: { id: string; label: string }[],
    onSelect: (value: string) => void,
    unavailable = false,
  ) => (
    <Select className="min-w-0" value={value} onChange={(value) => { if (value != null) onSelect(String(value)); }} isDisabled={disabled || unavailable}>
      <Label>{t(`settings.wine.${field}`)}</Label>
      <Select.Trigger className={COMPONENT_STYLES.select.trigger}>
        <Select.Value /><Select.Indicator />
      </Select.Trigger>
      <Select.Popover className={COMPONENT_STYLES.select.popoverContent}>
        <ListBox className={COMPONENT_STYLES.select.listbox}>
          {choices.map(({ id, label }) => (
            <ListBox.Item key={id} id={id} textValue={label}>
              <Label>{label}</Label><ListBox.ItemIndicator />
            </ListBox.Item>
          ))}
        </ListBox>
      </Select.Popover>
    </Select>
  );

  const switchRow = (
    id: string,
    labelKey: string,
    descKey: string,
    selected: boolean,
    unavailable: boolean,
    onChange: (selected: boolean) => void,
  ) => (
    <li key={id} className="flex min-w-0 items-center justify-between gap-4 py-3">
      <div className="min-w-0">
        <p className="text-sm font-medium">{t(labelKey)}</p>
        <p id={`wine-${id}-description`} className="mt-0.5 text-xs leading-relaxed text-muted">{t(descKey)}</p>
      </div>
      <Switch
        size="sm"
        className="shrink-0"
        aria-label={t(labelKey)}
        aria-describedby={`wine-${id}-description`}
        isSelected={selected}
        isDisabled={disabled || unavailable}
        onChange={onChange}
      >
        <Switch.Content>
          <Switch.Control><Switch.Thumb /></Switch.Control>
        </Switch.Content>
      </Switch>
    </li>
  );

  return (
    <section className="flex min-w-0 flex-col gap-4 border-t border-border/70 pt-5" aria-labelledby="wine-config-heading">
      <div className="flex items-center justify-between gap-3">
        <h2 id="wine-config-heading" className="text-sm font-semibold">{t("settings.wine.configuration")}</h2>
        {(loading || saving) && <Spinner size="sm" aria-label={t("settings.wine.working")} />}
      </div>
      {settings && <>
        <div className={showD3DMetalVersion ? "grid min-w-0 gap-3 sm:grid-cols-2" : "max-w-sm"}>
          {selectField("graphics", settings.graphics, graphics.map((id) => ({
            id,
            label: t(metalSharp && id === "auto"
              ? (hasBundledD3DMetal ? "settings.wine.graphics_auto_metalsharp" : "settings.wine.graphics_auto_mullion")
              : `settings.wine.graphics_${id}`),
          })), (nextGraphics) => applyImmediate({ graphics: nextGraphics }))}
          {showD3DMetalVersion && selectField("d3dmetal_version", settings.d3dMetalVersion || "auto", d3dMetalVersions.map((id) => ({
            id,
            label: t(`settings.wine.d3dmetal_version_${id}`),
          })), (d3dMetalVersion) => applyImmediate({ d3dMetalVersion }))}
        </div>

        <details className="group min-w-0 rounded-xl border border-border/70">
          <summary className="flex cursor-pointer list-none items-center gap-3 rounded-xl px-4 py-3 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent [&::-webkit-details-marker]:hidden">
            <span className="min-w-0 flex-1">
              <span className="block text-sm font-semibold">{t("settings.wine.advanced")}</span>
              <span className="mt-0.5 block text-xs leading-relaxed text-muted">{t("settings.wine.advanced_desc")}</span>
            </span>
            <LuChevronDown aria-hidden="true" className="size-4 shrink-0 text-muted group-open:rotate-180" />
          </summary>
          <div className="grid gap-6 border-t border-border/70 px-4 py-4">
            {/* Layer 1: Rendering & Frame Rate */}
            <div className="grid min-w-0 gap-4">
              <div>
                <h3 className="text-xs font-semibold uppercase tracking-wide text-muted">{t("settings.wine.group_rendering")}</h3>
              </div>
              <div className="grid min-w-0 gap-3 sm:grid-cols-2">
                {selectField("sync", settings.sync, ["auto", "msync", "esync", "none"].map((id) => ({
                  id,
                  label: t(`settings.wine.sync_${id}`),
                })), (sync) => applyImmediate({ sync }))}
                {mac && selectField("vsync", vsync, ["auto", "on", "off"].map((id) => ({
                  id,
                  label: t(`settings.wine.vsync_${id}`),
                })), (nextVsync) => applyImmediate({ vsync: nextVsync }), !metalSharp)}
                {selectField("max_fps", String(maxFps), frameRates.map((fps) => ({
                  id: String(fps),
                  label: fps === 0 ? t("settings.wine.default_fps") : `${fps} FPS`,
                })), (fps) => applyImmediate({ maxFps: Number(fps) }), !supportsGraphicsTuning)}
                {selectField("metal_fx", metalFxScale, ["off", "quality", "balanced", "performance", "ultra_performance", "extreme", "max"].map((id) => ({
                  id,
                  label: t(`settings.wine.metal_fx_${id}`),
                })), (scale) => applyImmediate({ metalFxScale: scale, enableMetalFx: scale !== "off" }), !supportsSpatialScale)}
              </div>
              {mac && metalSharp && (vsync !== "auto" || maxFps > 0) && (
                <p className="-mt-1 text-xs leading-relaxed text-muted">
                  {t(vsync === "off" && maxFps === 0 ? "settings.wine.vsync_off_tip" : "settings.wine.frame_pacing_tip")}
                </p>
              )}
              {!supportsGraphicsTuning && (
                <p className="-mt-1 text-xs leading-relaxed text-muted">
                  {t(metalSharp ? "settings.wine.backend_options" : "settings.wine.metalsharp_options")}
                </p>
              )}
              {mac && (
                <>
                  <ul className="divide-y divide-border/60 border-y border-border/60">
                    {switchRow(
                      "metal3_mode",
                      "settings.wine.metal3_mode",
                      "settings.wine.metal3_mode_desc",
                      !settings.enableMetal4,
                      !supportsGraphicsTuning,
                      (metal3Enabled) => applyImmediate({ enableMetal4: !metal3Enabled }),
                    )}
                    {switchRow(
                      "retina_mode",
                      "settings.wine.retina_mode",
                      "settings.wine.retina_mode_desc",
                      !!settings.retinaMode,
                      false,
                      handleRetinaChange,
                    )}
                    {switchRow(
                      "metal_hud",
                      "settings.wine.metal_hud",
                      "settings.wine.metal_hud_desc",
                      !!settings.metalHud,
                      !metalSharp,
                      (metalHud) => applyImmediate({ metalHud }),
                    )}
                  </ul>
                  {displayPairingAdvice && (
                    displayPairingAdvice.severity === "warning" ? (
                      <div role="status" className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-warning/30 bg-warning/10 px-3 py-2.5">
                        <div className="flex min-w-0 flex-1 items-start gap-2">
                          <LuCircleAlert aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-warning" />
                          <p className="text-xs leading-relaxed text-foreground">{t(displayPairingAdvice.messageKey)}</p>
                        </div>
                        {(displayPairingAdvice.canUsePerformance || displayPairingAdvice.canDisableRetina) && (
                          <div className="flex shrink-0 flex-wrap items-center gap-2">
                            {displayPairingAdvice.canUsePerformance && (
                              <Button
                                size="sm"
                                variant="outline"
                                isDisabled={disabled}
                                onPress={() => applyImmediate({ metalFxScale: "performance", enableMetalFx: true })}
                              >
                                {t("settings.wine.action_use_performance")}
                              </Button>
                            )}
                            {displayPairingAdvice.canDisableRetina && (
                              <Button
                                size="sm"
                                variant="outline"
                                isDisabled={disabled}
                                onPress={() => applyImmediate({ retinaMode: false })}
                              >
                                {t("settings.wine.action_disable_retina")}
                              </Button>
                            )}
                          </div>
                        )}
                      </div>
                    ) : (
                      <p className="-mt-1 text-xs leading-relaxed text-muted">{t(displayPairingAdvice.messageKey)}</p>
                    )
                  )}
                </>
              )}
            </div>

            {/* Layer 2: CPU Scheduling & Compatibility */}
            {mac && (
              <div className="grid min-w-0 gap-4">
                <div>
                  <h3 className="text-xs font-semibold uppercase tracking-wide text-muted">{t("settings.wine.group_scheduling")}</h3>
                </div>
                <div className="max-w-sm">
                  {selectField("thread_qos", threadQos, ["balanced", "aggressive", "default"].map((id) => ({
                    id,
                    label: t(`settings.wine.thread_qos_${id}`),
                  })), (nextQos) => applyImmediate({ threadQos: nextQos }), !metalSharp)}
                  <p className="mt-1 text-xs leading-relaxed text-muted">{t("settings.wine.thread_qos_desc")}</p>
                </div>
                <ul className="divide-y divide-border/60 border-y border-border/60">
                  {switchRow(
                    "rosetta_avx",
                    "settings.wine.rosetta_avx",
                    "settings.wine.rosetta_avx_desc",
                    !settings.disableRosettaAvx,
                    !metalSharp,
                    (enabled) => applyImmediate({ disableRosettaAvx: !enabled }),
                  )}
                  {switchRow(
                    "left_command_is_ctrl",
                    "settings.wine.left_command_is_ctrl",
                    "settings.wine.left_command_is_ctrl_desc",
                    !!settings.leftCommandIsCtrl,
                    false,
                    (leftCommandIsCtrl) => applyImmediate({ leftCommandIsCtrl }),
                  )}
                </ul>
              </div>
            )}

            {/* Layer 3: Diagnostics & Maintenance */}
            <div className="grid min-w-0 gap-4">
              <div>
                <h3 className="text-xs font-semibold uppercase tracking-wide text-muted">{t("settings.wine.group_maintenance")}</h3>
              </div>
              <ul className="divide-y divide-border/60 border-y border-border/60">
                {switchRow(
                  "compat_debug_log",
                  "settings.wine.compat_debug_log",
                  "settings.wine.compat_debug_log_desc",
                  !!settings.compatDebugLog,
                  false,
                  (compatDebugLog) => applyImmediate({ compatDebugLog }),
                )}
                <li className="flex min-w-0 flex-wrap items-center justify-between gap-4 py-3">
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium">{t("settings.wine.shader_cache")}</p>
                    <p className="mt-0.5 text-xs leading-relaxed text-muted">
                      {shaderCache && shaderCache.sizeBytes > 0
                        ? t("settings.wine.shader_cache_size", { size: formatBytes(shaderCache.sizeBytes) })
                        : t("settings.wine.shader_cache_empty")}
                    </p>
                  </div>
                  <Button
                    size="sm"
                    variant="outline"
                    className="shrink-0"
                    isDisabled={disabled || clearingCache || !shaderCache || shaderCache.sizeBytes === 0}
                    onPress={() => void clearShaderCache()}
                  >
                    {clearingCache && <Spinner size="sm" />}
                    {t("settings.wine.shader_cache_clear")}
                  </Button>
                </li>
                <li className="flex min-w-0 flex-wrap items-center justify-between gap-4 py-3">
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium">{t("settings.wine.kill_wine")}</p>
                    <p className="mt-0.5 text-xs leading-relaxed text-muted">{t("settings.wine.kill_wine_desc")}</p>
                  </div>
                  <Button
                    size="sm"
                    variant="outline"
                    className="shrink-0"
                    isDisabled={disabled || killingWine}
                    onPress={() => void killWine()}
                  >
                    {killingWine && <Spinner size="sm" />}
                    {t("settings.wine.kill_wine_action")}
                  </Button>
                </li>
                <li className="flex min-w-0 flex-wrap items-center justify-between gap-4 py-3">
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium">{t("settings.wine.check")}</p>
                    {result && <p role="status" className="mt-0.5 break-words text-xs text-success">{result}</p>}
                  </div>
                  <Button
                    size="sm"
                    variant="outline"
                    className="shrink-0"
                    isDisabled={disabled || checking}
                    onPress={() => void checkVersion()}
                  >
                    {checking && <Spinner size="sm" />}
                    {t("settings.wine.check")}
                  </Button>
                </li>
              </ul>
            </div>

            {/* Layer 4: Manual Paths (collapsed by default) */}
            <details className="group/paths min-w-0 rounded-lg border border-border/60">
              <summary className="flex cursor-pointer list-none items-center justify-between gap-3 rounded-lg px-3 py-2.5 text-sm font-medium focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent [&::-webkit-details-marker]:hidden">
                <span>{t("settings.wine.manual_paths")}</span>
                <LuChevronDown aria-hidden="true" className="size-4 shrink-0 text-muted group-open/paths:rotate-180" />
              </summary>
              <div className="grid min-w-0 gap-3 border-t border-border/60 px-3 py-3">
                {mac && pathField("app")}
                {pathField("executable")}
                {pathField("prefix")}
              </div>
            </details>
          </div>
        </details>
      </>}
      {error && <p role="alert" className="break-words text-sm text-danger">{error}</p>}
    </section>
  );
}
