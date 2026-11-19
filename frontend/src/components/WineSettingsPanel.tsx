import React from "react";
import { Button, Input, Label, ListBox, Select, Separator, Spinner, Switch, TextField, toast } from "@heroui/react";
import { Browser, Events } from "@wailsio/runtime";
import { LuCircle, LuCircleAlert, LuCircleCheck } from "react-icons/lu";
import { useTranslation } from "react-i18next";
import {
  CheckWineSettings, GetWineDownloadSettings, SaveWineDownloadSettings, SaveWineSettings,
  ScanWineInstallations, StartWineInstaller, WineNeedsRosetta,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import type {
  WineDownloadSettings, WineInstallation, WineSettings as Settings,
} from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import { formatBytes } from "@/utils/formatting";
import { UnifiedModal } from "@/components/UnifiedModal";
import { WineSettings } from "@/components/WineSettings";
import { GITHUB_PROXY_PRESETS, githubProxyLabel } from "@/constants/githubProxy";

type WineInstallProgress = { stage: string; status: string; downloaded: number; total: number };

const stages = ["rosetta", "template", "engine", "container", "initialize", "select"] as const;
const appleRosettaURL = "https://support.apple.com/102527";
const checkingRosetta: WineInstallProgress = { stage: "rosetta", status: "working", downloaded: 0, total: 0 };

export function WineSettingsPanel() {
  const { t } = useTranslation();
  const [installations, setInstallations] = React.useState<WineInstallation[] | null>(null);
  const [selecting, setSelecting] = React.useState(false);
  const [settingsRevision, refreshSettings] = React.useReducer((revision) => revision + 1, 0);
  const [download, setDownload] = React.useState<WineDownloadSettings | null>(null);
  const [proxy, setProxy] = React.useState("");
  const [saving, setSaving] = React.useState(false);
  const [scanning, setScanning] = React.useState(false);
  const [checking, setChecking] = React.useState(false);
  const [installing, setInstalling] = React.useState(false);
  const [installOpen, setInstallOpen] = React.useState(false);
  const [needsConsent, setNeedsConsent] = React.useState(false);
  const [progress, setProgress] = React.useState(checkingRosetta);
  const [finished, setFinished] = React.useState(false);
  const [installError, setInstallError] = React.useState("");
  const [error, setError] = React.useState("");
  const installPending = checking || installing;
  const locked = installPending || scanning || selecting;

  const describeError = (failure: unknown) => {
    const message = failure instanceof Error ? failure.message : String(failure);
    const code = message.match(/ERR_WINE_[A-Z_]+/)?.[0];
    const detail = code && message.includes(":") ? message.slice(message.indexOf(":") + 1).trim() : "";
    const text = code ? [t("settings.wine.errors." + code, { defaultValue: code }), detail].filter(Boolean).join(" · ") : message;
    return { code, text };
  };

  React.useEffect(() => {
    let mounted = true;
    GetWineDownloadSettings().then((current) => {
      if (mounted) {
        setDownload(current);
        setProxy(current.githubProxy);
      }
    }).catch((failure: unknown) => {
      if (mounted) setError(failure instanceof Error ? failure.message : String(failure));
    });
    const stop = Events.On("wine.install.progress", (event) => {
      const next = event.data as WineInstallProgress;
      if (!next || !stages.some((stage) => stage === next.stage)) return;
      setProgress(next);
    });
    return () => { mounted = false; stop(); };
  }, []);

  const scan = async () => {
    setScanning(true);
    setError("");
    try { setInstallations(await ScanWineInstallations()); }
    catch (failure) { setError(describeError(failure).text); }
    finally { setScanning(false); }
  };

  const saveDownload = async (settings: WineDownloadSettings) => {
    setSaving(true);
    setError("");
    try {
      await SaveWineDownloadSettings(settings);
      const current = await GetWineDownloadSettings();
      setDownload(current);
      setProxy(current.githubProxy);
      toast.success(t("settings.wine.saved"));
    } catch (failure) { setError(describeError(failure).text); }
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

  const install = async (consent: boolean) => {
    setNeedsConsent(false);
    setInstalling(true);
    setProgress(checkingRosetta);
    try {
      await StartWineInstaller(consent);
      refreshSettings();
      setInstallations(null);
      setFinished(true);
    } catch (failure) {
      const { code, text } = describeError(failure);
      if (code === "ERR_WINE_ROSETTA_CONSENT") setNeedsConsent(true);
      else setInstallError(text);
    } finally { setInstalling(false); }
  };

  const requestInstall = async () => {
    setInstallOpen(true);
    setNeedsConsent(false);
    setFinished(false);
    setInstallError("");
    setProgress(checkingRosetta);
    setChecking(true);
    try {
      if (await WineNeedsRosetta()) setNeedsConsent(true);
      else void install(false);
    } catch (failure) { setInstallError(describeError(failure).text); }
    finally { setChecking(false); }
  };

  const stageIndex = stages.findIndex((stage) => stage === progress.stage);
  const downloading = progress.status === "downloading" && progress.total > 0;

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="font-medium">Wine</h2>
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" isDisabled={locked || saving} onPress={() => void scan()}>
            {scanning && <Spinner size="sm" />}
            {t("settings.wine.scan")}
          </Button>
          <Button variant="primary" isDisabled={locked || saving || !download} onPress={() => void requestInstall()}>
            {t("settings.wine.managed.install")}
          </Button>
        </div>
      </div>

      {error && <p role="alert" className="break-words text-sm text-danger">{error}</p>}

      {installations !== null && <section aria-label={t("settings.wine.found", { count: installations.length })}>
        <p role="status" className="mb-2 text-sm text-muted">{t("settings.wine.found", { count: installations.length })}</p>
        {installations.length === 0 && <p className="text-sm">{t("settings.wine.empty")}</p>}
        <ul className="divide-y divide-border">
          {installations.map((item) => <li key={`${item.settings.app}:${item.settings.executable}:${item.settings.prefix}`} className="flex flex-col justify-between gap-3 py-4 sm:flex-row sm:items-center">
            <div className="min-w-0">
              <h3 className="font-medium">{item.name}</h3>
              <p className="mt-1 break-all text-sm text-muted">{item.settings.executable}</p>
            </div>
            <Button className="shrink-0 self-start sm:self-auto" variant="outline" isDisabled={locked || item.status !== "ready"} onPress={() => void choose(item.settings)}>
              {t(item.status === "needs_prefix" ? "settings.wine.setup.needs_prefix" : "settings.wine.use")}
            </Button>
          </li>)}
        </ul>
      </section>}

      <Separator className="bg-surface-tertiary/50" />
      <div className="flex items-center justify-between gap-4">
        <div className="min-w-0">
          <p className="font-medium">{t("settings.mirror_source.title")}</p>
          <p className="text-xs text-muted">{t("settings.wine.managed.mirror_desc")}</p>
        </div>
        <Switch size="sm" aria-label={t("settings.mirror_source.title")} isSelected={download?.mirror === true}
          isDisabled={!download || saving || locked} onChange={(mirror) => { if (download) void saveDownload({ ...download, mirror }); }}>
          <Switch.Content><Switch.Control><Switch.Thumb /></Switch.Control></Switch.Content>
        </Switch>
      </div>
      {download?.mirror && <div className="flex flex-wrap items-end gap-2">
        <Select className="w-full sm:w-52" aria-label={t("settings.lip.github_proxy.title")}
          isDisabled={saving || locked} value={GITHUB_PROXY_PRESETS.includes(proxy) ? proxy : "custom"}
          onChange={(value) => {
            if (value === "custom") setProxy("");
            else if (value) void saveDownload({ ...download, githubProxy: String(value) });
          }}>
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
        <TextField className="min-w-52 flex-1" value={proxy} onChange={setProxy} isDisabled={saving || locked}>
          <Label>{t("settings.lip.github_proxy.title")}</Label>
          <Input type="url" spellCheck={false} placeholder="https://gh-proxy.org/https://github.com" />
        </TextField>
        <Button variant="outline" isDisabled={saving || locked || proxy === download.githubProxy}
          onPress={() => void saveDownload({ ...download, githubProxy: proxy })}>{t("settings.wine.save")}</Button>
      </div>}
      <Separator className="bg-surface-tertiary/50" />
      <fieldset disabled={selecting} className={selecting ? "pointer-events-none opacity-60" : undefined}>
        <WineSettings key={settingsRevision} />
      </fieldset>
      <UnifiedModal
        isOpen={installOpen}
        onOpenChange={setInstallOpen}
        isPending={installPending}
        size="compact"
        type={installError ? "error" : finished ? "success" : needsConsent ? "warning" : "info"}
        title={t(needsConsent ? "settings.wine.managed.rosetta_title" :
          finished ? "settings.wine.managed.ready" :
          installError ? "settings.wine.managed.failed" : "settings.wine.managed.progress")}
        showCancelButton={needsConsent || !!installError}
        cancelText={t(installError ? "common.close" : "common.cancel")}
        showConfirmButton={!installPending}
        confirmText={t(needsConsent ? "settings.wine.managed.rosetta_confirm" :
          installError ? "settings.wine.managed.retry" : "common.close")}
        onConfirm={() => {
          if (needsConsent) void install(true);
          else if (installError) void requestInstall();
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
          {stages.map((stage, index) => {
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
                    <div className="mt-1 text-xs text-muted">
                      <span role="status">
                        {t(`settings.wine.managed.status.${progress.status}`)}
                        {downloading ? ` · ${formatBytes(progress.downloaded)} / ${formatBytes(progress.total)}` : ""}
                      </span>
                      {downloading && <progress className="mt-1 block h-1.5 w-full max-w-xs accent-current" aria-label={t(`settings.wine.managed.stage.${stage}`)} max={progress.total} value={progress.downloaded} />}
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
