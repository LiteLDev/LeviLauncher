import React from "react";
import { Button, Input, Label, ListBox, Select, Spinner, Switch, TextField, toast } from "@heroui/react";
import { Browser } from "@wailsio/runtime";
import { LuChevronDown, LuCircle, LuCircleAlert, LuCircleCheck, LuRefreshCw, LuTrash2 } from "react-icons/lu";
import { useTranslation } from "react-i18next";
import {
  CheckWineSettings, GetLegacyWineInfo, GetWineDownloadSettings, ListInstalledWineVersions,
  ListWineRuntimeReleases, RemoveLegacyWineRuntime, RemoveWineVersion, SaveWineDownloadSettings,
  SaveWineSettings, ScanWineInstallations, SwitchWineVersion,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import type {
  LegacyWineInfo, ManagedWineVersion, WineDownloadSettings, WineInstallation,
  WineRuntimeRelease, WineSettings as Settings,
} from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { InstallProgressRow } from "@/components/InstallProgressRow";
import { formatBytes } from "@/utils/formatting";
import { UnifiedModal } from "@/components/UnifiedModal";
import { WineSettings } from "@/components/WineSettings";
import { GITHUB_PROXY_PRESETS, githubProxyLabel } from "@/constants/githubProxy";
import { installWine, useWineInstall, wineInstallStages } from "@/utils/wineInstallStore";

const appleRosettaURL = "https://support.apple.com/102527";

function normalizeVersion(raw?: string): string {
  return (raw || "").trim().replace(/^v/i, "");
}

function compareVersions(a: string, b: string): number {
  const pa = normalizeVersion(a).split(".").map((part) => Number.parseInt(part, 10) || 0);
  const pb = normalizeVersion(b).split(".").map((part) => Number.parseInt(part, 10) || 0);
  const len = Math.max(pa.length, pb.length);
  for (let i = 0; i < len; i++) {
    const diff = (pa[i] ?? 0) - (pb[i] ?? 0);
    if (diff !== 0) return diff;
  }
  return 0;
}

export function WineSettingsPanel() {
  const { t } = useTranslation();
  const [installations, setInstallations] = React.useState<WineInstallation[] | null>(null);
  const [selecting, setSelecting] = React.useState(false);
  const [active, setActive] = React.useState<Settings | null>(null);
  const [legacyInfo, setLegacyInfo] = React.useState<LegacyWineInfo | null>(null);
  const [cleaningLegacy, setCleaningLegacy] = React.useState(false);
  const [releases, setReleases] = React.useState<WineRuntimeRelease[]>([]);
  const [loadingReleases, setLoadingReleases] = React.useState(false);
  const [selectedReleaseTag, setSelectedReleaseTag] = React.useState("");
  const [installedVersions, setInstalledVersions] = React.useState<ManagedWineVersion[]>([]);
  const [switchingVersionPath, setSwitchingVersionPath] = React.useState("");
  const [removingVersionPath, setRemovingVersionPath] = React.useState("");
  const [confirmDeleteVersion, setConfirmDeleteVersion] = React.useState<ManagedWineVersion | null>(null);
  const metalSharp = active?.runtime === "metalsharp";
  const installedVersion = normalizeVersion(active?.runtimeVersion);
  const latestRelease = releases.find((item) => item.isLatest) ?? releases[0] ?? null;
  const updateAvailable = metalSharp && !!latestRelease && !!installedVersion &&
    compareVersions(latestRelease.version, installedVersion) > 0;
  const legacy = legacyInfo?.active ?? active?.app.endsWith("/Runtimes/Sikarugir/Levi Wine.app");
  const runtimeName = metalSharp
    ? (installedVersion ? `MetalSharp Wine ${installedVersion}` : "MetalSharp Wine")
    : (active?.app || active?.executable)?.split("/").pop()?.replace(/\.app$/i, "");
  const [settingsRevision, refreshSettings] = React.useReducer((revision) => revision + 1, 0);
  const [download, setDownload] = React.useState<WineDownloadSettings | null>(null);
  const [proxy, setProxy] = React.useState("");
  const [saving, setSaving] = React.useState(false);
  const [scanning, setScanning] = React.useState(false);
  const [installOpen, setInstallOpen] = React.useState(false);
  const [error, setError] = React.useState("");
  const [downloadError, setDownloadError] = React.useState("");
  const { status, progress, error: installFailure, completed, targetTag } = useWineInstall();
  const installPending = status === "running";
  const needsConsent = status === "consent";
  const finished = status === "done";
  const locked = installPending || scanning || selecting || cleaningLegacy || !!switchingVersionPath || !!removingVersionPath;

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

  const loadInstalledVersions = React.useCallback(async () => {
    try {
      const items = await ListInstalledWineVersions();
      setInstalledVersions(items || []);
    } catch {
      setInstalledVersions([]);
    }
  }, []);

  const loadReleases = React.useCallback(async () => {
    setLoadingReleases(true);
    try {
      const items = await ListWineRuntimeReleases();
      setReleases(items || []);
      setSelectedReleaseTag((prev) => {
        if (prev && items?.some((r) => r.tag === prev)) return prev;
        return items?.[0]?.tag || "";
      });
    } catch {
      setReleases([]);
    } finally {
      setLoadingReleases(false);
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
    void loadInstalledVersions();
    void loadReleases();
    return () => { mounted = false; };
  }, [loadInstalledVersions, loadLegacyInfo, loadReleases]);

  React.useEffect(() => {
    setInstallations(null);
    void loadInstalledVersions();
  }, [completed, loadInstalledVersions]);

  const handleLoaded = React.useCallback((settings: Settings) => {
    setActive(settings);
    void loadLegacyInfo();
    void loadInstalledVersions();
  }, [loadInstalledVersions, loadLegacyInfo]);

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

  const requestInstall = (tag = "") => {
    setInstallOpen(true);
    void installWine(false, tag);
  };

  const switchInstalledVersion = async (item: ManagedWineVersion) => {
    setSwitchingVersionPath(item.path);
    setError("");
    try {
      await SwitchWineVersion(item.path);
      refreshSettings();
      await loadInstalledVersions();
      toast.success(t("settings.wine.managed.version_switched", { version: item.tag || item.version }));
    } catch (failure) {
      setError(describeError(failure).text);
    } finally {
      setSwitchingVersionPath("");
    }
  };

  const removeInstalledVersion = async (item: ManagedWineVersion) => {
    setRemovingVersionPath(item.path);
    setConfirmDeleteVersion(null);
    setError("");
    try {
      await RemoveWineVersion(item.path);
      await loadInstalledVersions();
      setInstallations(null);
      toast.success(t("settings.wine.managed.version_removed", { version: item.tag || item.version }));
    } catch (failure) {
      setError(describeError(failure).text);
    } finally {
      setRemovingVersionPath("");
    }
  };

  const selectedRelease = releases.find((item) => item.tag === selectedReleaseTag) ?? latestRelease;
  const matchedInstalledForSelected = selectedRelease
    ? installedVersions.find((item) => normalizeVersion(item.version) === normalizeVersion(selectedRelease.version)) ?? null
    : null;
  const isSelectedVersionActive = !!matchedInstalledForSelected?.active ||
    (metalSharp && !!selectedRelease && !!installedVersion && normalizeVersion(selectedRelease.version) === installedVersion);
  const totalInstalledBytes = installedVersions.reduce((sum, item) => sum + (item.sizeBytes || 0), 0);
  const hasMultipleInstalled = installedVersions.length > 1;
  const versionManagerSummary = React.useMemo(() => {
    const parts: string[] = [];
    if (installedVersions.length > 0) {
      parts.push(t("settings.wine.managed.installed_total", {
        count: installedVersions.length,
        size: formatBytes(totalInstalledBytes),
      }));
    }
    if (updateAvailable && latestRelease) {
      parts.push(t("settings.wine.managed.summary_update", { latest: latestRelease.tag }));
    } else if (hasMultipleInstalled) {
      parts.push(t("settings.wine.managed.summary_cleanup"));
    } else if (metalSharp && installedVersion) {
      parts.push(t("settings.wine.managed.summary_up_to_date"));
    }
    return parts.join(" · ");
  }, [hasMultipleInstalled, installedVersion, installedVersions.length, latestRelease, metalSharp, t, totalInstalledBytes, updateAvailable]);

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
            {active && (
              <div className="mt-1 flex flex-wrap items-center gap-2">
                <p className="break-words text-sm text-muted">{runtimeName || t("settings.wine.managed.not_configured")}</p>
                {updateAvailable && latestRelease && (
                  <span className="rounded-md bg-accent/15 px-1.5 py-0.5 text-[11px] font-medium text-accent">
                    {t("settings.wine.managed.summary_update", { latest: latestRelease.tag })}
                  </span>
                )}
              </div>
            )}
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
              <Button size="sm" variant="primary" aria-haspopup="dialog" isDisabled={locked || saving || !download} onPress={() => requestInstall(selectedRelease?.tag || "")}>
                {t(legacy ? "settings.wine.managed.upgrade" : "settings.wine.managed.install")}
              </Button>
            )}
            {updateAvailable && latestRelease && (status === "idle" || finished) && (
              <Button size="sm" variant="primary" aria-haspopup="dialog" isDisabled={locked || saving || !download} onPress={() => requestInstall(latestRelease.tag)}>
                {t("settings.wine.managed.update_to", { version: latestRelease.tag })}
              </Button>
            )}
          </div>
        </div>
        {(releases.length > 0 || installedVersions.length > 0) && (
          <details className="group mt-4 min-w-0 rounded-xl border border-border/70">
            <summary className="flex cursor-pointer list-none items-center gap-3 rounded-xl px-4 py-3 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent [&::-webkit-details-marker]:hidden">
              <span className="min-w-0 flex-1">
                <span className="block text-sm font-medium">{t("settings.wine.managed.version_manager")}</span>
                {versionManagerSummary && (
                  <span className="mt-0.5 block text-xs leading-relaxed text-muted">{versionManagerSummary}</span>
                )}
              </span>
              <LuChevronDown aria-hidden="true" className="size-4 shrink-0 text-muted group-open:rotate-180" />
            </summary>
            <div className="grid gap-4 border-t border-border/70 px-4 py-3.5">
              {releases.length > 0 && (
                <div className="grid min-w-0 gap-2">
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-xs font-medium text-muted">{t("settings.wine.managed.version_label")}</span>
                    <Button
                      size="sm"
                      variant="ghost"
                      isDisabled={locked || saving || loadingReleases}
                      onPress={() => void loadReleases()}
                    >
                      {loadingReleases ? <Spinner size="sm" /> : <LuRefreshCw aria-hidden="true" className="size-3.5" />}
                      {t("settings.wine.managed.refresh_versions")}
                    </Button>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <Select
                      aria-label={t("settings.wine.managed.version_label")}
                      className="min-w-[14rem] flex-1 sm:max-w-xs"
                      value={selectedRelease?.tag || ""}
                      isDisabled={locked || saving || loadingReleases}
                      onChange={(value) => { if (value != null) setSelectedReleaseTag(String(value)); }}
                    >
                      <Select.Trigger className={COMPONENT_STYLES.select.trigger}>
                        <Select.Value /><Select.Indicator />
                      </Select.Trigger>
                      <Select.Popover className={COMPONENT_STYLES.select.popoverContent}>
                        <ListBox className={COMPONENT_STYLES.select.listbox}>
                          {releases.map((rel) => {
                            const badges: string[] = [];
                            const localMatch = installedVersions.find(
                              (item) => normalizeVersion(item.version) === normalizeVersion(rel.version),
                            );
                            if (rel.isLatest) badges.push(t("settings.wine.managed.version_latest"));
                            if (localMatch?.active || (metalSharp && installedVersion && normalizeVersion(rel.version) === installedVersion)) {
                              badges.push(t("settings.wine.in_use"));
                            } else if (localMatch) {
                              badges.push(t("settings.wine.managed.version_installed"));
                            }
                            const label = badges.length > 0 ? `${rel.tag} (${badges.join(" · ")})` : rel.tag;
                            return (
                              <ListBox.Item key={rel.tag} id={rel.tag} textValue={label}>
                                <Label>{label}</Label>
                                <ListBox.ItemIndicator />
                              </ListBox.Item>
                            );
                          })}
                        </ListBox>
                      </Select.Popover>
                    </Select>
                    {selectedRelease && !isSelectedVersionActive && (
                      <Button
                        size="sm"
                        variant={matchedInstalledForSelected ? "outline" : "primary"}
                        aria-haspopup={matchedInstalledForSelected ? undefined : "dialog"}
                        isDisabled={locked || saving || !download}
                        onPress={() => {
                          if (matchedInstalledForSelected) {
                            void switchInstalledVersion(matchedInstalledForSelected);
                          } else {
                            requestInstall(selectedRelease.tag);
                          }
                        }}
                      >
                        {matchedInstalledForSelected && switchingVersionPath === matchedInstalledForSelected.path && <Spinner size="sm" />}
                        {matchedInstalledForSelected
                          ? t("settings.wine.managed.switch_version")
                          : t("settings.wine.managed.install_version")}
                      </Button>
                    )}
                  </div>
                  <p className="text-xs leading-relaxed text-muted">
                    {t("settings.wine.managed.version_desc")}
                  </p>
                </div>
              )}

              {installedVersions.length > 0 && (
                <div className={releases.length > 0 ? "border-t border-border/60 pt-3" : undefined}>
                  <div className="flex items-center justify-between gap-2">
                    <h3 className="text-xs font-medium text-muted">
                      {t("settings.wine.managed.installed_versions")}
                    </h3>
                    <span className="text-xs text-muted">
                      {formatBytes(totalInstalledBytes)}
                    </span>
                  </div>
                  <ul className="mt-2 divide-y divide-border/60">
                    {installedVersions.map((item) => {
                      const isSwitching = switchingVersionPath === item.path;
                      const isRemoving = removingVersionPath === item.path;
                      const label = item.version ? `MetalSharp Wine ${item.version}` : "MetalSharp Wine";
                      return (
                        <li key={item.path} className="flex min-w-0 items-center justify-between gap-3 py-2 first:pt-0.5 last:pb-0">
                          <div className="flex min-w-0 flex-wrap items-center gap-x-2.5 gap-y-1">
                            <span className="text-sm font-medium">{label}</span>
                            <span className="text-xs text-muted">{formatBytes(item.sizeBytes)}</span>
                            {item.active && (
                              <span className="rounded-md bg-accent/15 px-1.5 py-0.5 text-[11px] font-medium text-accent">
                                {t("settings.wine.in_use")}
                              </span>
                            )}
                          </div>
                          {!item.active && (
                            <div className="flex shrink-0 items-center gap-2">
                              <Button
                                size="sm"
                                variant="outline"
                                isDisabled={locked || saving}
                                onPress={() => void switchInstalledVersion(item)}
                              >
                                {isSwitching && <Spinner size="sm" />}
                                {t("settings.wine.use")}
                              </Button>
                              <Button
                                size="sm"
                                variant="danger-soft"
                                isDisabled={locked || saving}
                                onPress={() => setConfirmDeleteVersion(item)}
                              >
                                {isRemoving ? <Spinner size="sm" /> : <LuTrash2 aria-hidden="true" className="size-3.5" />}
                                {t("settings.wine.managed.delete_version")}
                              </Button>
                            </div>
                          )}
                        </li>
                      );
                    })}
                  </ul>
                </div>
              )}
            </div>
          </details>
        )}
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
        {installations !== null && (
          <div className="mt-4 rounded-xl border border-border/70 px-4 py-3">
            <div className="flex items-center justify-between gap-2">
              <p role="status" className="text-xs font-medium text-muted">
                {installations.length === 0 ? t("settings.wine.empty") : t("settings.wine.found", { count: installations.length })}
              </p>
              <Button size="sm" variant="ghost" onPress={() => setInstallations(null)}>
                {t("common.close")}
              </Button>
            </div>
            {installations.length > 0 && (
              <ul className="mt-2 divide-y divide-border/60">
                {installations.map((item) => {
                  const current = active !== null && item.settings.app === active.app && item.settings.executable === active.executable && item.settings.prefix === active.prefix;
                  return (
                    <li key={JSON.stringify([item.settings.app, item.settings.executable, item.settings.prefix])} className="flex min-w-0 flex-wrap items-center justify-between gap-3 py-2.5 first:pt-1 last:pb-0">
                      <div className="min-w-0 flex-1">
                        <h3 className="break-words text-sm font-medium">{item.name}</h3>
                        <p className="mt-0.5 break-all text-xs text-muted">{item.settings.executable}</p>
                      </div>
                      {item.status !== "ready" || current ? (
                        <span className="shrink-0 text-xs font-medium text-muted">
                          {t(item.status === "needs_prefix" ? "settings.wine.setup.needs_prefix" :
                            item.status === "unsupported" ? "settings.wine.unsupported" : "settings.wine.in_use")}
                        </span>
                      ) : (
                        <Button size="sm" className="shrink-0" variant="outline" isDisabled={locked} onPress={() => void choose(item.settings)}>
                          {t("settings.wine.use")}
                        </Button>
                      )}
                    </li>
                  );
                })}
              </ul>
            )}
          </div>
        )}
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
          if (needsConsent) void installWine(true, targetTag);
          else if (installError) requestInstall(targetTag);
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
      <UnifiedModal
        isOpen={confirmDeleteVersion !== null}
        onOpenChange={(open) => { if (!open) setConfirmDeleteVersion(null); }}
        size="compact"
        type="warning"
        title={t("settings.wine.managed.delete_confirm_title")}
        showCancelButton
        cancelText={t("common.cancel")}
        confirmText={t("settings.wine.managed.delete_version")}
        onConfirm={() => { if (confirmDeleteVersion) void removeInstalledVersion(confirmDeleteVersion); }}
      >
        {confirmDeleteVersion && (
          <p className="text-sm leading-relaxed text-muted">
            {t("settings.wine.managed.delete_confirm_body", {
              version: confirmDeleteVersion.tag || confirmDeleteVersion.version,
              size: formatBytes(confirmDeleteVersion.sizeBytes),
            })}
          </p>
        )}
      </UnifiedModal>
    </div>
  );
}
