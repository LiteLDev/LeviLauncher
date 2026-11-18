import React from "react";
import { Button, Spinner, Separator } from "@heroui/react";
import { Browser, System } from "@wailsio/runtime";
import { useTranslation } from "react-i18next";
import { CheckWineSettings, GetWineInstallPlan, SaveWineSettings, ScanWineInstallations, StartWineInstaller } from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import type { WineInstallation, WineInstallPlan, WineSettings as Settings } from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import { UnifiedModal } from "@/components/UnifiedModal";
import { WineSettings } from "@/components/WineSettings";

type OnConfigured = (settings: Settings, version: string) => void;

export function WineSettingsPanel({ onConfigured, onKeepCurrent }: { onConfigured?: OnConfigured; onKeepCurrent?: () => void }) {
  const { t } = useTranslation();
  const [installations, setInstallations] = React.useState<WineInstallation[] | null>(null);
  const [selection, setSelection] = React.useState<Settings | null>(null);
  const [scanning, setScanning] = React.useState(false);
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState("");
  const [status, setStatus] = React.useState("");
  const [plan, setPlan] = React.useState<WineInstallPlan | null>(null);
  const [installOpen, setInstallOpen] = React.useState(false);
  const [manual, setManual] = React.useState(false);
  const reportError = React.useCallback((error: unknown) => setError(error instanceof Error ? error.message : String(error)), []);

  const scan = React.useCallback(async () => {
    setScanning(true);
    setError("");
    setStatus("");
    try { setInstallations(await ScanWineInstallations()); }
    catch (error) { reportError(error); }
    finally { setScanning(false); }
  }, [reportError]);
  const inSetup = Boolean(onConfigured);
  React.useEffect(() => {
    if (inSetup) void scan();
  }, [inSetup, scan]);

  const choose = async (settings: Settings) => {
    setStatus("");
    if (!onConfigured) {
      setSelection(settings);
      return;
    }
    setBusy(true);
    setError("");
    try {
      const version = await CheckWineSettings(settings);
      await SaveWineSettings(settings);
      onConfigured(settings, version);
    } catch (error) { reportError(error); }
    finally { setBusy(false); }
  };
  const showInstaller = async () => {
    setBusy(true);
    setError("");
    try { setPlan(await GetWineInstallPlan()); setInstallOpen(true); }
    catch (error) { reportError(error); }
    finally { setBusy(false); }
  };
  const install = async () => {
    if (!plan) return;
    setBusy(true);
    setError("");
    try {
      if (plan.canInstall) {
        await StartWineInstaller();
        setStatus(t(inSetup ? "settings.wine.setup.terminal_started" : "settings.wine.installer_started"));
      } else { await Browser.OpenURL(plan.url); }
      setInstallOpen(false);
    } catch (error) { reportError(error); }
    finally { setBusy(false); }
  };

  if (inSetup && manual) {
    return <WineSettings onConfigured={onConfigured} onCancel={() => setManual(false)} />;
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        {!inSetup && <h2 className="font-medium">Wine</h2>}
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" isDisabled={scanning || busy} onPress={() => void scan()}>
            {scanning && <Spinner size="sm" />}
            {t(inSetup && installations !== null ? "settings.wine.setup.rescan" : "settings.wine.scan")}
          </Button>
          <Button variant={inSetup && !installations?.some((item) => item.status === "ready") ? "primary" : "outline"} isDisabled={busy || scanning} onPress={() => void showInstaller()}>{t("settings.wine.install")}</Button>
          {inSetup && <Button variant="ghost" isDisabled={busy || scanning} onPress={() => setManual(true)}>{t("settings.wine.setup.manual")}</Button>}
        </div>
      </div>
      {error && <p role="alert" className="break-words text-sm text-danger">{error}</p>}
      {status && <p role="status" className="text-sm text-success">{status}</p>}
      {installations !== null && <section aria-label={t("settings.wine.found", { count: installations.length })}>
        <p role="status" className="mb-2 text-sm text-muted">{t("settings.wine.found", { count: installations.length })}</p>
        {installations.length === 0 && <p className="text-sm">{t("settings.wine.empty")}</p>}
        <ul className="divide-y divide-border">
          {installations.map((item) => <li key={`${item.settings.app}:${item.settings.executable}:${item.settings.prefix}`} className="flex flex-col justify-between gap-3 py-4 sm:flex-row sm:items-center">
            <div className="min-w-0">
              <h3 className="font-medium">{item.name}</h3>
              <p className="mt-1 break-all text-sm text-muted">{item.settings.executable}</p>
            </div>
            <Button className="shrink-0 self-start sm:self-auto" variant="outline" isDisabled={item.status === "unsupported" || (inSetup && (busy || scanning || item.status !== "ready"))} onPress={() => void choose(item.settings)}>
              {t(inSetup && item.status === "needs_prefix" ? "settings.wine.setup.needs_prefix" : "settings.wine.use")}
            </Button>
          </li>)}
        </ul>
      </section>}
      {onKeepCurrent && <Button variant="ghost" className="self-start" isDisabled={scanning || busy} onPress={onKeepCurrent}>{t("settings.wine.setup.keep_current")}</Button>}
      {!inSetup && <>
        <Separator className="bg-surface-tertiary/50" />
        <WineSettings selection={selection} />
      </>}
      <UnifiedModal
        isOpen={installOpen} onOpenChange={setInstallOpen} title={t("settings.wine.install")}
        showCancelButton cancelText={t("common.cancel")}
        confirmText={t(plan?.canInstall ? "settings.wine.install_terminal" : "settings.wine.install_instructions")}
        confirmButtonProps={{ isDisabled: busy }} onConfirm={() => void install()} onCancel={() => setInstallOpen(false)}
      >
        <p>{t(inSetup && System.IsMac() ? "settings.wine.setup.install_help" : plan?.canInstall ? "settings.wine.install_help" : "settings.wine.install_manual")}</p>
        {inSetup && System.IsMac() && <p className="mt-2 text-sm text-muted">{t("settings.wine.setup.install_next")}</p>}
        {plan?.canInstall && <pre className="mt-4 whitespace-pre-wrap break-words rounded-xl bg-surface-secondary p-4 text-sm"><code>{plan.command}</code></pre>}
        {plan?.canInstall && <Button className="mt-4" variant="ghost" isDisabled={busy} onPress={() => {
          if (plan) void Browser.OpenURL(plan.url).catch(reportError);
        }}>{t("settings.wine.install_instructions")}</Button>}
        {error && <p role="alert" className="mt-3 break-words text-sm text-danger">{error}</p>}
      </UnifiedModal>
    </div>
  );
}
