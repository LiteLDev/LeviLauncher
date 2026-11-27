import React from "react";
import { Button, Input, Label, ListBox, Select, Spinner, Switch, TextField, toast } from "@heroui/react";
import { Browser } from "@wailsio/runtime";
import { LuCircle, LuCircleAlert, LuCircleCheck } from "react-icons/lu";
import { useTranslation } from "react-i18next";
import {
  CheckWineSettings, GetLegacyWineInfo, GetWineDownloadSettings, RemoveLegacyWineRuntime,
  SaveWineDownloadSettings, SaveWineSettings, ScanWineInstallations,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import type {
  LegacyWineInfo, WineDownloadSettings, WineInstallation, WineSettings as Settings,
} from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import { InstallProgressRow } from "@/components/InstallProgressRow";
import { formatBytes } from "@/utils/formatting";
import { UnifiedModal } from "@/components/UnifiedModal";
import { WineSettings } from "@/components/WineSettings";
import { GITHUB_PROXY_PRESETS, githubProxyLabel } from "@/constants/githubProxy";
import { installWine, useWineInstall, wineInstallStages } from "@/utils/wineInstallStore";

const appleRosettaURL = "https://support.apple.com/102527";

export function WineSettingsPanel() {
  const { t } = useTranslation();
  const [installations, setInstallations] = React.useState<WineInstallation[] | null>(null);
  const [selecting, setSelecting] = React.useState(false);
  const [active, setActive] = React.useState<Settings | null>(null);
  const [legacyInfo, setLegacyInfo] = React.useState<LegacyWineInfo | null>(null);
  const [cleaningLegacy, setCleaningLegacy] = React.useState(false);
  const metalSharp = active?.runtime === "metalsharp";
  const legacy = legacyInfo?.active ?? active?.app.endsWith("/Runtimes/Sikarugir/Levi Wine.app");
  const runtimeName = metalSharp ? "MetalSharp Wine" : (active?.app || active?.executable)?.split("/").pop()?.replace(/\.app$/i, "");
  const [settingsRevision, refreshSettings] = React.useReducer((revision) => revision + 1, 0);
  const [download, setDownload] = React.useState<WineDownloadSettings | null>(null);
  const [proxy, setProxy] = React.useState("");
  const [saving, setSaving] = React.useState(false);
  const [scanning, setScanning] = React.useState(false);
  const [installOpen, setInstallOpen] = React.useState(false);
  const [error, setError] = React.useState("");
  const [downloadError, setDownloadError] = React.useState("");
  const { status, progress, error: installFailure, completed } = useWineInstall();
  const installPending = status === "running";
  const needsConsent = status === "consent";
  const finished = status === "done";
  const locked = installPending || scanning || selecting || cleaningLegacy;

  const describeError = (failure: unknown) => {
    const message = failure instanceof Error ? failure.message : String(failure);
    const code = message.match(/ERR_WINE_[A-Z_]+/)?.[0];
    const detail = code && message.includes(":") ? message.slice(message.indexOf(":") + 1).trim() : "";
    const text = code ? [t("settings.wine.errors." + code, { defaultValue: code }), detail].filter(Boolean).join(" · ") : message;
    return { code, text };
  };
  const installError = installFailure && describeError(installFailure).text;

  const loadLegacyInfo = React.useCallback(async () => {
    try {
      setLegacyInfo(await GetLegacyWineInfo());
    } catch {
      setLegacyInfo(null);
    }
  }, []);

  React.useEffect(() => {
    let mounted = true;
    GetWineDownloadSettings().then((current) => {
      if (mounted) {
        setDownload(current);
        setProxy(current.githubProxy);
      }
    }).catch((failure: unknown) => {
      if (mounted) setDownloadError(failure instanceof Error ? failure.message : String(failure));
    });
    void loadLegacyInfo();
    return () => { mounted = false; };
  }, [loadLegacyInfo]);

  React.useEffect(() => { setInstallations(null); }, [completed]);

  const handleLoaded = React.useCallback((settings: Settings) => {
    setActive(settings);
    void loadLegacyInfo();
  }, [loadLegacyInfo]);

  const scan = async () => {
    setScanning(true);
    setError("");
    try { setInstallations(await ScanWineInstallations()); }
    catch (failure) { setError(describeError(failure).text); }
    finally { setScanning(false); }
  };

  const saveDownload = async (settings: WineDownloadSettings) => {
    setSaving(true);
    setDownloadError("");
    try {
      await SaveWineDownloadSettings(settings);
      const current = await GetWineDownloadSettings();
      setDownload(current);
      setProxy(current.githubProxy);
      toast.success(t("settings.wine.saved"));
    } catch (failure) { setDownloadError(describeError(failure).text); }
    finally { setSaving(false); }
  };

  const choose = async (settings: Settings) => {
    setSelecting(true);
    setError("");
    try {
      await CheckWineSettings(settings);
      await SaveWineSettings(settings);
      refreshSettings();
      toast.success(t("settings.wine.saved"));
    } catch (failure) { setError(describeError(failure).text); }
    finally { setSelecting(false); }
  };

  const requestInstall = () => {
    setInstallOpen(true);
    void installWine(false);
  };

  const removeLegacy = async () => {
    setCleaningLegacy(true);
    setError("");
    try {
      await RemoveLegacyWineRuntime();
      await loadLegacyInfo();
      setInstallations(null);
      toast.success(t("settings.wine.managed.cleanup_legacy_done"));
    } catch (failure) {
      setError(describeError(failure).text);
    } finally {
      setCleaningLegacy(false);
    }
  };

  const stageIndex = wineInstallStages.findIndex((stage) => stage === progress.stage);
  const downloadPercent = progress.total > 0
    ? Math.min(100, Math.max(0, (progress.downloaded / progress.total) * 100))
    : undefined;
  const statusLabel = t(`settings.wine.managed.status.${progress.status}`);
  const downloading = progress.status === "downloading";
  const downloadAmount = progress.total > 0
    ? `${formatBytes(progress.downloaded)} / ${formatBytes(progress.total)}`
    : progress.downloaded > 0 ? formatBytes(progress.downloaded) : undefined;

  return (
    <div className="flex min-w-0 flex-col gap-6">
      <section aria-labelledby="wine-runtime-heading" className="min-w-0">
        <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-3">
          <div className="min-w-0">
            <h2 id="wine-runtime-heading" className="text-sm font-semibold">{t("settings.wine.runtime")}</h2>
            {active && <p className="mt-1 break-words text-sm text-muted">{runtimeName || t("settings.wine.managed.not_configured")}</p>}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button size="sm" variant="outline" isDisabled={locked || saving} onPress={() => void scan()}>
              {scanning && <Spinner size="sm" />}
              {t("settings.wine.scan")}
            </Button>
            {status !== "idle" && (
              <Button size="sm" variant={needsConsent ? "primary" : "outline"} aria-haspopup="dialog" onPress={() => setInstallOpen(true)}>
                {t(installPending ? "settings.wine.managed.show_progress" : needsConsent ? "settings.wine.managed.continue_install" : "settings.wine.managed.show_result")}
              </Button>
            )}
            {!metalSharp && (status === "idle" || finished) && (
              <Button size="sm" variant="primary" aria-haspopup="dialog" isDisabled={locked || saving || !download} onPress={requestInstall}>
                {t(legacy ? "settings.wine.managed.upgrade" : "settings.wine.managed.install")}
              </Button>
            )}
          </div>
        </div>
        {legacy && <p className="mt-3 text-sm leading-relaxed text-muted">{t("settings.wine.managed.legacy")}</p>}
        {!legacy && legacyInfo?.present && (
          <div className="mt-4 flex flex-wrap items-center justify-between gap-x-6 gap-y-3 rounded-xl border border-border/70 px-4 py-3">
            <p className="min-w-0 flex-1 text-xs leading-relaxed text-muted">
              {t("settings.wine.managed.cleanup_legacy_desc", { size: formatBytes(legacyInfo.sizeBytes) })}
            </p>
            <Button size="sm" variant="outline" isDisabled={locked || saving} onPress={() => void removeLegacy()}>
              {cleaningLegacy && <Spinner size="sm" />}
              {t("settings.wine.managed.cleanup_legacy")}
            </Button>
          </div>
        )}
        {installations !== null && <div className="mt-4 border-t border-border/70 pt-3">
          <p role="status" className="text-sm text-muted">
            {installations.length === 0 ? t("settings.wine.empty") : t("settings.wine.found", { count: installations.length })}
          </p>
          <ul className="divide-y divide-border/70">
            {installations.map((item) => {
              const current = active !== null && item.settings.app === active.app && item.settings.executable === active.executable && item.settings.prefix === active.prefix;
              return (
                <li key={JSON.stringify([item.settings.app, item.settings.executable, item.settings.prefix])} className="flex min-w-0 flex-wrap items-center justify-between gap-3 py-3">
                  <div className="min-w-0 flex-1">
                    <h3 className="break-words text-sm font-medium">{item.name}</h3>
                    <p className="mt-1 break-all text-xs text-muted">{item.settings.executable}</p>
                  </div>
                  {item.status !== "ready" || current ? (
                    <span className="shrink-0 text-xs font-medium text-muted">
                      {t(item.status === "needs_prefix" ? "settings.wine.setup.needs_prefix" :
                        item.status === "unsupported" ? "settings.wine.unsupported" : "settings.wine.in_use")}
                    </span>
                  ) : (
                    <Button className="shrink-0" variant="outline" isDisabled={locked} onPress={() => void choose(item.settings)}>
                      {t("settings.wine.use")}
                    </Button>
                  )}
                </li>
              );
            })}
          </ul>
        </div>}
        {error && <p role="alert" className="mt-3 break-words text-sm text-danger">{error}</p>}
      </section>

      <WineSettings key={`${settingsRevision}:${completed}`} locked={selecting || installPending} onLoaded={handleLoaded} />

      <section className="min-w-0 border-t border-border/70 pt-5" aria-labelledby="wine-download-heading">
        <div className="flex items-center justify-between gap-4">
          <div className="min-w-0">
            <h2 id="wine-download-heading" className="text-sm font-semibold">{t("settings.wine.download_source")}</h2>
            <p id="wine-download-description" className="mt-1 text-xs leading-relaxed text-muted">
              {t("settings.wine.managed.mirror_desc")}
            </p>
          </div>
          <Switch
            size="sm"
            className="shrink-0"
            aria-label={t("settings.mirror_source.title")}
            aria-describedby="wine-download-description"
            isSelected={download?.mirror === true}
            isDisabled={!download || saving || locked}
            onChange={(mirror) => { if (download) void saveDownload({ ...download, mirror }); }}
          >
            <Switch.Content><Switch.Control><Switch.Thumb /></Switch.Control></Switch.Content>
          </Switch>
        </div>
        {download?.mirror && <div className="mt-4 grid min-w-0 items-end gap-3 md:grid-cols-[minmax(9rem,1fr)_minmax(0,2fr)_auto]">
          <Select className="min-w-0"
            isDisabled={saving || locked}
            value={GITHUB_PROXY_PRESETS.includes(proxy) ? proxy : "custom"}
            onChange={(value) => {
              if (value === "custom") setProxy("");
              else if (value) void saveDownload({ ...download, githubProxy: String(value) });
            }}>
            <Label>{t("settings.wine.proxy_preset")}</Label>
            <Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger>
            <Select.Popover>
              <ListBox>
                {GITHUB_PROXY_PRESETS.map((url) => (
                  <ListBox.Item key={url} id={url} textValue={githubProxyLabel(url)}>
                    <Label>{githubProxyLabel(url)}</Label>
                    <ListBox.ItemIndicator />
                  </ListBox.Item>
                ))}
                <ListBox.Item id="custom" textValue={t("settings.lip.github_proxy.preset_custom")}>
                  <Label>{t("settings.lip.github_proxy.preset_custom")}</Label>
                  <ListBox.ItemIndicator />
                </ListBox.Item>
              </ListBox>
            </Select.Popover>
          </Select>
          <TextField className="min-w-0" value={proxy} onChange={setProxy} isDisabled={saving || locked}>
            <Label>{t("settings.lip.github_proxy.title")}</Label>
            <Input type="url" spellCheck={false} aria-describedby={downloadError ? "wine-download-error" : undefined} placeholder="https://gh-proxy.org/https://github.com" />
          </TextField>
          <Button className="justify-self-start" variant="outline" isDisabled={saving || locked || proxy === download.githubProxy}
            onPress={() => void saveDownload({ ...download, githubProxy: proxy })}>{t("settings.wine.save")}</Button>
        </div>}
        {downloadError && <p id="wine-download-error" role="alert" className="mt-3 break-words text-sm text-danger">{downloadError}</p>}
      </section>
      <UnifiedModal
        isOpen={installOpen}
        onOpenChange={setInstallOpen}
        size="compact"
        type={installError ? "error" : finished ? "success" : needsConsent ? "warning" : "info"}
        title={t(needsConsent ? "settings.wine.managed.rosetta_title" :
          finished ? "settings.wine.managed.ready" :
          installError ? "settings.wine.managed.failed" : "settings.wine.managed.progress")}
        showCancelButton={needsConsent || !!installError}
        cancelText={t(installError ? "common.close" : "common.cancel")}
        confirmText={t(installPending ? "settings.wine.managed.run_in_background" : needsConsent ? "settings.wine.managed.rosetta_confirm" :
          installError ? "settings.wine.managed.retry" : "common.close")}
        onConfirm={() => {
          if (needsConsent) void installWine(true);
          else if (installError) requestInstall();
          else setInstallOpen(false);
        }}
      >
        {needsConsent && (
          <div>
            <p>{t("settings.wine.managed.rosetta_body")}</p>
            <Button variant="ghost" className="mt-2" onPress={() => void Browser.OpenURL(appleRosettaURL)}>
              {t("settings.wine.managed.rosetta_info")}
            </Button>
          </div>
        )}
        <ol className="flex flex-col gap-2">
          {wineInstallStages.map((stage, index) => {
            const current = index === stageIndex && !finished;
            const done = index < stageIndex || finished ||
              (current && (progress.status === "skipped" || progress.status === "done"));
            return (
              <li key={stage} aria-current={current ? "step" : undefined} className="flex items-start gap-2 text-sm">
                <span aria-hidden="true" className="mt-0.5 shrink-0">
                  {done ? <LuCircleCheck className="text-success" /> : current && installPending ? <Spinner size="sm" /> : current && installError ? <LuCircleAlert className="text-danger" /> : <LuCircle className="text-muted" />}
                </span>
                <div className="min-w-0 flex-1">
                  <span className={current ? "font-medium" : undefined}>{t(`settings.wine.managed.stage.${stage}`)}</span>
                  {current && !done && installPending && (
                    <div className="mt-1">
                      {downloading ? (
                        <InstallProgressRow
                          label={statusLabel}
                          amount={downloadAmount}
                          value={downloadPercent}
                        />
                      ) : (
                        <span role="status" className="block text-xs text-muted">{statusLabel}</span>
                      )}
                    </div>
                  )}
                </div>
              </li>
            );
          })}
        </ol>
        {installError && <p role="alert" className="text-sm text-danger">{installError}</p>}
      </UnifiedModal>
    </div>
  );
}
