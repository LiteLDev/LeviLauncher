import React from "react";
import { Button, Input, Label, ListBox, Select, Spinner, TextField, toast } from "@heroui/react";
import { Dialogs, System } from "@wailsio/runtime";
import { useTranslation } from "react-i18next";
import { CheckWineSettings, GetWineSettings, SaveWineSettings } from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import type { WineSettings as Settings } from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import { COMPONENT_STYLES } from "@/constants/componentStyles";

export const WineSettings: React.FC<{
  selection?: Settings | null;
  onConfigured?: (settings: Settings, version: string) => void;
  onCancel?: () => void;
}> = ({ selection, onConfigured, onCancel }) => {
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
  React.useEffect(() => {
    if (!selection) return;
    setSettings({ ...selection });
    setResult("");
    setError("");
  }, [selection]);

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
  const changePath = (field: "app" | "executable" | "prefix", path: string) => {
    if (field === "app") {
      changeApp(path);
    } else if (onConfigured && field === "executable") {
      change({ app: "", executable: path });
    } else {
      change({ [field]: path });
    }
  };
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
      if (path) changePath(field, path);
    } catch (error) { reportError(error); }
  };

  const run = async (action: "save" | "check") => {
    if (!settings) return;
    setBusy(true);
    setError("");
    setResult("");
    try {
      if (action === "save") {
        if (onConfigured) {
          const version = await CheckWineSettings(settings);
          await SaveWineSettings(settings);
          onConfigured(settings, version);
          return;
        }
        await SaveWineSettings(settings);
        const current = await GetWineSettings();
        setSettings(current);
        setSaved(JSON.stringify(current));
        toast.success(t("settings.wine.saved"));
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
        onChange={(value) => changePath(field, value)}
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
    <section aria-label={t("settings.wine.title")} className="flex flex-col gap-4">
      {settings && <>
        {System.IsMac() && pathField("app")}
        {pathField("executable")}
        {pathField("prefix")}
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
      </>}
      {error && <p role="alert" className="break-words text-sm text-danger">{error}</p>}
      {result && <p role="status" className="break-words text-sm text-success">{result}</p>}
      <div className="flex flex-wrap items-center gap-2">
        {onCancel && <Button variant="ghost" isDisabled={busy} onPress={onCancel}>{t("settings.wine.setup.back_to_list")}</Button>}
        <Button variant="primary" isDisabled={busy || !settings || (!onConfigured && JSON.stringify(settings) === saved)} onPress={() => void run("save")}>
          {t(onConfigured ? "settings.wine.setup.use" : "settings.wine.save")}
        </Button>
        <Button variant="outline" isDisabled={busy || !settings} onPress={() => void run("check")}>{t("settings.wine.check")}</Button>
        <Button variant="ghost" isDisabled={busy} onPress={() => void load()}>{t("settings.wine.reload")}</Button>
        {busy && <Spinner size="sm" aria-label={t("settings.wine.working")} />}
      </div>
    </section>
  );
};
