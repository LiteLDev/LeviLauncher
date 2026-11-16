import React from "react";
import { Button, Spinner, Separator } from "@heroui/react";
import { Browser } from "@wailsio/runtime";
import { useTranslation } from "react-i18next";
import { GetWineInstallPlan, ScanWineInstallations, StartWineInstaller } from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import type { WineInstallation, WineInstallPlan, WineSettings as Settings } from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import { UnifiedModal } from "@/components/UnifiedModal";
import { WineSettings } from "@/components/WineSettings";

export function WineSettingsPanel() {
  const { t } = useTranslation();
  const [installations, setInstallations] = React.useState<WineInstallation[] | null>(null);
  const [selection, setSelection] = React.useState<Settings | null>(null);
  const [scanning, setScanning] = React.useState(false);
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState("");
  const [status, setStatus] = React.useState("");
  const [plan, setPlan] = React.useState<WineInstallPlan | null>(null);
  const [installOpen, setInstallOpen] = React.useState(false);
  const reportError = (error: unknown) => setError(error instanceof Error ? error.message : String(error));

  const scan = async () => {
    setScanning(true);
    setError("");
    setStatus("");
    try { setInstallations(await ScanWineInstallations()); }
    catch (error) { reportError(error); }
    finally { setScanning(false); }
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
        setStatus(t("settings.wine.installer_started"));
      } else { await Browser.OpenURL(plan.url); }
      setInstallOpen(false);
    } catch (error) { reportError(error); }
    finally { setBusy(false); }
  };

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="font-medium">Wine</h2>
        <div className="flex items-center gap-2">
          <Button variant="outline" isDisabled={scanning || busy} onPress={() => void scan()}>
            {scanning && <Spinner size="sm" />}{t("settings.wine.scan")}
          </Button>
          <Button variant="outline" isDisabled={busy || scanning} onPress={() => void showInstaller()}>{t("settings.wine.install")}</Button>
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
            <Button className="shrink-0 self-start sm:self-auto" variant="outline" isDisabled={item.status === "unsupported"} onPress={() => {
              setSelection(item.settings);
              setStatus("");
            }}>{t("settings.wine.use")}</Button>
          </li>)}
        </ul>
      </section>}
      <Separator className="bg-surface-tertiary/50" />
      <WineSettings selection={selection} />
      <UnifiedModal
        isOpen={installOpen} onOpenChange={setInstallOpen} title={t("settings.wine.install")}
        showCancelButton cancelText={t("common.cancel")}
        confirmText={t(plan?.canInstall ? "settings.wine.install_terminal" : "settings.wine.install_instructions")}
        confirmButtonProps={{ isDisabled: busy }} onConfirm={() => void install()} onCancel={() => setInstallOpen(false)}
      >
        <p>{t(plan?.canInstall ? "settings.wine.install_help" : "settings.wine.install_manual")}</p>
        {plan?.canInstall && <pre className="mt-4 whitespace-pre-wrap break-words rounded-xl bg-surface-secondary p-4 text-sm"><code>{plan.command}</code></pre>}
        <Button className="mt-4" variant="ghost" isDisabled={busy} onPress={() => {
          if (plan) void Browser.OpenURL(plan.url).catch(reportError);
        }}>{t("settings.wine.install_instructions")}</Button>
      </UnifiedModal>
    </div>
  );
}
