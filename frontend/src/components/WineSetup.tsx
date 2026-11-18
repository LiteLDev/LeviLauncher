import { useEffect, useState } from "react";
import { Button, Spinner } from "@heroui/react";
import { LuCircleCheck } from "react-icons/lu";
import { useTranslation } from "react-i18next";
import { CheckWineSettings, GetWineSettings } from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import type { WineSettings } from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import { WineSettingsPanel } from "@/components/WineSettingsPanel";

type ConfiguredWine = { settings: WineSettings; version: string };

export function WineSetup({ onReadyChange }: { onReadyChange: (ready: boolean) => void }) {
  const { t } = useTranslation();
  const [current, setCurrent] = useState<ConfiguredWine | null>(null);
  const [checking, setChecking] = useState(true);
  const [choosing, setChoosing] = useState(false);

  useEffect(() => {
    let active = true;
    onReadyChange(false);
    const checkCurrent = async () => {
      try {
        const settings = await GetWineSettings();
        if (!active || !settings.executable) return;
        const version = await CheckWineSettings(settings);
        if (active) {
          setCurrent({ settings, version });
          onReadyChange(true);
        }
      } catch {
        // A missing or unusable configuration opens the picker.
      } finally {
        if (active) setChecking(false);
      }
    };
    void checkCurrent();
    return () => { active = false; };
  }, [onReadyChange]);

  if (checking) {
    return (
      <p role="status" className="flex items-center gap-2 text-sm text-muted">
        <Spinner size="sm" />{t("settings.wine.working")}
      </p>
    );
  }

  if (current && !choosing) {
    const runtime = current.settings.app.split("/").pop()?.replace(/\.app$/i, "");
    return (
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="min-w-0">
          <p role="status" className="flex items-center gap-2 font-medium">
            <LuCircleCheck aria-hidden="true" className="shrink-0 text-success" />
            {t("settings.wine.setup.ready")}
          </p>
          <p className="mt-1 break-words text-sm text-muted">
            {[runtime, current.version].filter(Boolean).join(" · ")}
          </p>
        </div>
        <Button variant="outline" onPress={() => {
          onReadyChange(false);
          setChoosing(true);
        }}>{t("settings.wine.setup.change")}</Button>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <p className="text-sm text-muted">{t("settings.wine.setup.choose_or_install")}</p>
      <WineSettingsPanel
        onConfigured={(settings, version) => {
          setCurrent({ settings, version });
          setChoosing(false);
          onReadyChange(true);
        }}
        onKeepCurrent={current ? () => {
          setChoosing(false);
          onReadyChange(true);
        } : undefined}
      />
    </div>
  );
}
