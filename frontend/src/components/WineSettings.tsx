import React from "react";
import { Button, Input, Label, ListBox, Select, Spinner, TextField } from "@heroui/react";
import { Dialogs, System } from "@wailsio/runtime";
import { useTranslation } from "react-i18next";
import { CheckWineSettings, GetWineSettings, SaveWineSettings } from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import type { WineSettings as Settings } from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import { COMPONENT_STYLES } from "@/constants/componentStyles";

export const WineSettings: React.FC = () => {
  const { t } = useTranslation();
  const [settings, setSettings] = React.useState<Settings | null>(null);
  const [saved, setSaved] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState("");
  const [result, setResult] = React.useState("");

  const reportError = (error: unknown) => setError(error instanceof Error ? error.message : String(error));
  const load = async () => {
    setBusy(true);
    setError("");
    try {
      const current = await GetWineSettings();
      setSettings(current);
      setSaved(JSON.stringify(current));
    } catch (error) { reportError(error); }
    finally { setBusy(false); }
  };
  React.useEffect(() => { void load(); }, []);

  const change = (patch: Partial<Settings>) => {
    setSettings((previous) => previous ? { ...previous, ...patch } : previous);
    setResult("");
    setError("");
  };
  const changeApp = (app: string) => change({
    app,
    ...(app.trim() ? {
      executable: `${app.replace(/\/+$/, "")}/Contents/SharedSupport/wine/bin/wine`,
      prefix: `${app.replace(/\/+$/, "")}/Contents/SharedSupport/prefix`,
    } : {}),
  });
  const choose = async (field: "app" | "executable" | "prefix") => {
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
      if (path) field === "app" ? changeApp(path) : change({ [field]: path });
    } catch (error) { reportError(error); }
  };

  const run = async (action: "save" | "check") => {
    if (!settings) return;
    setBusy(true);
    setError("");
    setResult("");
    try {
      if (action === "save") {
        await SaveWineSettings(settings);
        const current = await GetWineSettings();
        setSettings(current);
        setSaved(JSON.stringify(current));
        setResult(t("settings.wine.saved"));
      } else {
        const version = await CheckWineSettings(settings);
        setResult(t("settings.wine.checked", { version }));
      }
    } catch (error) { reportError(error); }
    finally { setBusy(false); }
  };

  const pathField = (field: "app" | "executable" | "prefix") => (
    <div className="flex items-end gap-2" key={field}>
      <TextField
        className="min-w-0 flex-1"
        value={settings?.[field] || ""}
        onChange={(value) => field === "app" ? changeApp(value) : change({ [field]: value })}
        isDisabled={busy}
      >
        <Label>{t(`settings.wine.${field}`)}</Label>
        <Input spellCheck={false} />
      </TextField>
      <Button variant="outline" isDisabled={busy} onPress={() => void choose(field)} aria-label={t("settings.wine.choose_path", { field: t(`settings.wine.${field}`) })}>
        {t("settings.wine.choose")}
      </Button>
    </div>
  );

  return (
    <section aria-labelledby="wine-settings-title" className="flex flex-col gap-4">
      <div>
        <h3 id="wine-settings-title" className="font-medium">{t("settings.wine.title")}</h3>
        <p className="mt-1 text-sm text-muted">{t("settings.wine.description")}</p>
      </div>
      {settings && <>
        {System.IsMac() && pathField("app")}
        {pathField("executable")}
        {pathField("prefix")}
        <p className="text-sm text-muted">{t("settings.wine.paths_help")}</p>
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          {(["graphics", "sync"] as const).map((field) => (
            <Select key={field} value={settings[field]} onChange={(value) => { if (value) change({ [field]: String(value) }); }} isDisabled={busy}>
              <Label>{t(`settings.wine.${field}`)}</Label>
              <Select.Trigger className={COMPONENT_STYLES.select.trigger}>
                <Select.Value /><Select.Indicator />
              </Select.Trigger>
              <Select.Popover className={COMPONENT_STYLES.select.popoverContent}>
                <ListBox className={COMPONENT_STYLES.select.listbox}>
                  {(field === "graphics" ? ["auto", ...(System.IsMac() ? ["d3dmetal"] : []), "wined3d"] : ["auto", ...(System.IsMac() ? ["msync"] : []), "esync", "none"]).map((value) => (
                    <ListBox.Item key={value} id={value} textValue={t(`settings.wine.${field}_${value}`)}>
                      <Label>{t(`settings.wine.${field}_${value}`)}</Label><ListBox.ItemIndicator />
                    </ListBox.Item>
                  ))}
                </ListBox>
              </Select.Popover>
            </Select>
          ))}
        </div>
        <p className="text-sm text-muted">{t("settings.wine.graphics_help")}</p>
      </>}
      {error && <p role="alert" className="break-words text-sm text-danger">{error}</p>}
      {result && <p role="status" className="break-words text-sm text-success">{result}</p>}
      <div className="flex flex-wrap items-center gap-2">
        <Button variant="primary" isDisabled={busy || !settings || JSON.stringify(settings) === saved} onPress={() => void run("save")}>{t("settings.wine.save")}</Button>
        <Button variant="outline" isDisabled={busy || !settings} onPress={() => void run("check")}>{t("settings.wine.check")}</Button>
        <Button variant="ghost" isDisabled={busy} onPress={() => void load()}>{t("settings.wine.reload")}</Button>
        {busy && <Spinner size="sm" aria-label={t("settings.wine.working")} />}
      </div>
    </section>
  );
};
