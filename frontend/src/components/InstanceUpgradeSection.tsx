import { Button, Checkbox, Chip, Input, Spinner, TextField } from "@heroui/react";
import { Events } from "@wailsio/runtime";
import React from "react";
import { useTranslation } from "react-i18next";
import { useLocation } from "react-router-dom";
import { FiArrowDown, FiArrowUp, FiCheckCircle, FiDownload } from "react-icons/fi";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import { UnifiedModal } from "@/components/UnifiedModal";
import {
  ModalAction,
  ModalDescription,
  ModalNotice,
  ModalPanel,
  ModalProgress,
} from "@/components/ModalPrimitives";
import { setNavLockReason } from "@/hooks/useAppNavigation";
import { cn } from "@/utils/cn";
import { isDownloadPending, useDownloads } from "@/utils/DownloadsContext";
import { formatBytes } from "@/utils/formatting";
import { resolveInstallError } from "@/utils/installError";
import { normalizeVersionChannel, type PackageType, type VersionChannel } from "@/utils/packageType";
import { compareVersions } from "@/utils/version";

type CatalogItem = {
  short: string;
  type: VersionChannel;
  packageType: PackageType;
  urls: string[];
  md5?: string;
  uuid?: string;
};

// Installer handed over by the download task center or the global prompt.
type InstallerHandoff = {
  version: string;
  type: string;
  packageType?: string;
  installerPath: string;
};

interface InstanceUpgradeSectionProps {
  targetName: string;
  packageType: PackageType;
  versionType: string;
  gameVersion: string;
  // Installed LeviLamina version, "" when none.
  leviLaminaVersion: string;
  isDisabled?: boolean;
  onUpgraded: (newVersion: string) => void;
}

const isPreviewChannel = (channel: string) =>
  String(channel || "").toLowerCase() === "preview";

const gdkDownloadName = (item: CatalogItem) => `${item.type} ${item.short}.msixvc`;
const uwpDownloadName = (item: CatalogItem) => `Minecraft-UWP-${item.type}-${item.short}.appx`;

const fileNameOf = (path: string) => path.split(/[\\/]/).pop() || "";

async function fetchCatalog(packageType: PackageType): Promise<CatalogItem[]> {
  if (packageType === "uwp") {
    const data = await minecraft.FetchUWPVersions();
    return (data || []).map((v: any) => ({
      short: String(v.version),
      type: normalizeVersionChannel(v.type),
      packageType: "uwp" as const,
      urls: [],
      uuid: v.uuid,
    }));
  }
  const preferCN = await minecraft.IsMirrorEnabled().catch(() => false);
  const data: any = await minecraft.FetchHistoricalVersions(preferCN);
  return (["Preview", "Release"] as const).flatMap((type) =>
    ((type === "Preview" ? data?.previewVersions : data?.releaseVersions) || []).map((v: any) => {
      const raw = v.urls ?? v.url;
      const urls = (Array.isArray(raw) ? raw : raw ? [raw] : [])
        .map((u: unknown) => String(u || "").trim())
        .filter(Boolean);
      return {
        short: String(v.version).replace(/^(?:Preview|Release)\s*/, ""),
        type,
        packageType: "gdk" as const,
        urls,
        md5: v.md5,
      };
    }),
  );
}

async function resolveDownloaded(item: CatalogItem): Promise<string> {
  try {
    if (item.packageType === "uwp") {
      return String((await minecraft.ResolveDownloadedUWP(item.short, item.type.toLowerCase())) || "");
    }
    return String(
      (await minecraft.ResolveDownloadedMsixvc(`${item.type} ${item.short}`, item.type.toLowerCase())) || "",
    );
  } catch {
    return "";
  }
}

export const InstanceUpgradeSection: React.FC<InstanceUpgradeSectionProps> = ({
  targetName,
  packageType,
  versionType,
  gameVersion,
  leviLaminaVersion,
  isDisabled,
  onUpgraded,
}) => {
  const { t } = useTranslation();
  const location = useLocation() as any;
  const { downloads, startDownload, dismissPrompt, setUpgradeDialogTarget } = useDownloads();

  const [open, setOpen] = React.useState(false);
  const [catalog, setCatalog] = React.useState<CatalogItem[]>([]);
  const [catalogLoading, setCatalogLoading] = React.useState(false);
  const [catalogError, setCatalogError] = React.useState("");
  const [query, setQuery] = React.useState("");
  const [selected, setSelected] = React.useState<CatalogItem | null>(null);
  const [downloaded, setDownloaded] = React.useState<Record<string, string>>({});
  const [handoff, setHandoff] = React.useState<InstallerHandoff | null>(null);
  const [backup, setBackup] = React.useState(true);
  const [upgrading, setUpgrading] = React.useState(false);
  const [progress, setProgress] = React.useState<{ current: number; total: number; file: string } | null>(null);
  // Set while a UWP delta upgrade runs (see uwp.DeltaStats).
  const [deltaStats, setDeltaStats] = React.useState<{
    phase: string;
    total: number;
    reused: number;
    toDownload: number;
    downloaded: number;
  } | null>(null);
  const [error, setError] = React.useState("");
  const [result, setResult] = React.useState<{ from: string; to: string; backupPath: string } | null>(null);

  const instancePreview = isPreviewChannel(versionType);
  const keyOf = (item: CatalogItem) => `${item.type}:${item.short}`;

  const candidates = React.useMemo(() => {
    const q = query.trim().toLowerCase();
    return catalog
      .filter((item) => item.packageType === packageType)
      .filter((item) => isPreviewChannel(item.type) === instancePreview)
      .filter((item) => item.short !== gameVersion)
      .filter((item) => !q || item.short.toLowerCase().includes(q))
      .sort((a, b) => compareVersions(b.short, a.short));
  }, [catalog, packageType, instancePreview, gameVersion, query]);

  const downloadFor = React.useCallback(
    (item: CatalogItem) => {
      const name = item.packageType === "uwp" ? uwpDownloadName(item) : gdkDownloadName(item);
      return downloads.find(
        (dl) =>
          (dl.fileName === name || fileNameOf(dl.dest) === name) &&
          (isDownloadPending(dl.status) || dl.status === "done" || dl.status === "error"),
      );
    },
    [downloads],
  );

  const refreshDownloaded = React.useCallback(async (items: CatalogItem[]) => {
    const entries = await Promise.all(
      items.map(async (item) => [keyOf(item), await resolveDownloaded(item)] as const),
    );
    setDownloaded(Object.fromEntries(entries.filter(([, name]) => name)));
  }, []);

  const loadCatalog = React.useCallback(async () => {
    setCatalogLoading(true);
    setCatalogError("");
    try {
      const items = await fetchCatalog(packageType);
      setCatalog(items);
      await refreshDownloaded(
        items.filter(
          (item) => item.packageType === packageType && isPreviewChannel(item.type) === instancePreview,
        ),
      );
    } catch (e: any) {
      setCatalogError(String(e?.message || e || "ERR_FETCH_VERSIONS"));
    } finally {
      setCatalogLoading(false);
    }
  }, [packageType, instancePreview, refreshDownloaded]);

  const openModal = React.useCallback(() => {
    setError("");
    setResult(null);
    setProgress(null);
    setOpen(true);
    void loadCatalog();
  }, [loadCatalog]);

  // Opened from a finished upgrade download (task center or global prompt).
  // Each hand-off opens the dialog once, even if the page re-renders with
  // the same navigation state.
  const consumedHandoffRef = React.useRef("");
  React.useEffect(() => {
    const incoming: InstallerHandoff | undefined = location?.state?.upgradeInstaller;
    if (!incoming || !targetName || location?.state?.name !== targetName) return;
    const id = `${location?.key || ""}:${incoming.installerPath}`;
    if (consumedHandoffRef.current === id) return;
    consumedHandoffRef.current = id;
    setHandoff(incoming);
    setQuery("");
    openModal();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [location?.state, location?.key, targetName]);

  React.useEffect(() => {
    if (!handoff || catalog.length === 0) return;
    const match = catalog.find(
      (item) =>
        item.packageType === packageType &&
        item.short === handoff.version &&
        item.type.toLowerCase() === String(handoff.type).toLowerCase(),
    );
    if (match) setSelected(match);
  }, [handoff, catalog, packageType]);

  // While the dialog is open it offers finished downloads itself, so the
  // global "download complete" prompt stays quiet for this instance.
  React.useEffect(() => {
    if (!open) return;
    setUpgradeDialogTarget(targetName);
    return () => setUpgradeDialogTarget("");
  }, [open, targetName, setUpgradeDialogTarget]);

  // A download finished while the dialog is open: it was handled here, so
  // the global prompt does not ask again after the dialog closes.
  React.useEffect(() => {
    if (!open) return;
    const finished = downloads.filter(
      (dl) => dl.status === "done" && dl.installer?.upgradeTarget === targetName,
    );
    if (finished.length === 0) return;
    finished.forEach((dl) => dismissPrompt(dl.dest));
    const relevant = catalog.filter(
      (item) => item.packageType === packageType && isPreviewChannel(item.type) === instancePreview,
    );
    void refreshDownloaded(relevant);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, downloads]);

  React.useEffect(() => {
    if (!upgrading) return;
    const off = Events.On("extract.progress", (event: any) => {
      const payload = event?.data || {};
      const total = Number(payload?.global_total || payload?.totalBytes || 0);
      const current = Number(payload?.global_current || payload?.bytes || 0);
      setProgress({ current, total, file: String(payload?.currentFile || "") });
    });
    const offDelta = Events.On("upgrade.delta", (event: any) => {
      const d = event?.data || {};
      setDeltaStats({
        phase: String(d?.phase || ""),
        total: Number(d?.total || 0),
        reused: Number(d?.reused || 0),
        toDownload: Number(d?.toDownload || 0),
        downloaded: Number(d?.downloaded || 0),
      });
    });
    setNavLockReason("instance-upgrade", true);
    return () => {
      off();
      offDelta();
      setNavLockReason("instance-upgrade", false);
    };
  }, [upgrading]);

  const selectedDownload = selected ? downloadFor(selected) : undefined;
  const selectedInstaller =
    selected &&
    (downloaded[keyOf(selected)] ||
      (selectedDownload?.status === "done" ? selectedDownload.dest : "") ||
      (handoff &&
      handoff.version === selected.short &&
      handoff.type.toLowerCase() === selected.type.toLowerCase()
        ? handoff.installerPath
        : ""));
  const isDowngrade = !!selected && compareVersions(selected.short, gameVersion) < 0;

  const startBackgroundDownload = async () => {
    if (!selected) return;
    setError("");
    const installer = {
      version: selected.short,
      type: selected.type,
      packageType: selected.packageType,
      uuid: selected.uuid,
      isLeviLaminaSupported: false,
      upgradeTarget: targetName,
    };
    if (selected.packageType === "uwp") {
      await startDownload(`uwp:${selected.uuid}`, uwpDownloadName(selected), "", installer);
      return;
    }
    const url = selected.urls[0];
    if (!url) {
      setError("ERR_NO_DOWNLOAD_URL");
      return;
    }
    await startDownload(url, gdkDownloadName(selected), selected.md5, installer);
  };

  // A UWP build that is not downloaded can be built from the instance's
  // current files plus the changed blocks fetched from Windows Update.
  const canDelta = !!selected && selected.packageType === "uwp" && !!selected.uuid && !selectedInstaller;

  const runUpgrade = async (mode: "installer" | "delta" = "installer") => {
    if (!selected || (mode === "installer" && !selectedInstaller)) return;
    setError("");
    setProgress(null);
    setDeltaStats(null);
    setUpgrading(true);
    try {
      const check = await minecraft.UpgradeTargetError(
        targetName,
        selected.short,
        selected.type,
        selected.packageType,
      );
      if (check) {
        setError(check);
        return;
      }
      const res: any =
        mode === "delta"
          ? await minecraft.UpgradeInstanceDelta(targetName, selected.uuid || "", selected.short, backup)
          : await minecraft.UpgradeInstance(targetName, selectedInstaller || "", selected.short, backup);
      if (res?.errorCode) {
        setError(String(res.errorCode));
        return;
      }
      const to = String(res?.toVersion || selected.short);
      setResult({ from: String(res?.fromVersion || gameVersion), to, backupPath: String(res?.backupPath || "") });
      setHandoff(null);
      onUpgraded(to);
    } catch (e: any) {
      setError(String(e?.message || e || "ERR_UPGRADE_FAILED"));
    } finally {
      setUpgrading(false);
    }
  };

  const closeModal = () => {
    if (upgrading) return;
    setOpen(false);
    setSelected(null);
    setQuery("");
  };

  const progressValue =
    progress && progress.total > 0 ? Math.min(100, (progress.current / progress.total) * 100) : undefined;

  const footer = result ? (
    <ModalAction variant="primary" onPress={closeModal}>
      {t("common.ok")}
    </ModalAction>
  ) : (
    <>
      <ModalAction variant="secondary" onPress={closeModal} isDisabled={upgrading}>
        {t("common.cancel")}
      </ModalAction>
      {canDelta && (
        <ModalAction variant="primary" isPending={upgrading} onPress={() => void runUpgrade("delta")}>
          {t("versions.edit.upgrade.delta_confirm")}
        </ModalAction>
      )}
      {selected && !selectedInstaller && (
        <ModalAction
          variant={canDelta ? "secondary" : "primary"}
          isDisabled={upgrading || (!!selectedDownload && isDownloadPending(selectedDownload.status))}
          onPress={() => void startBackgroundDownload()}
        >
          <FiDownload />
          {selectedDownload && isDownloadPending(selectedDownload.status)
            ? t("versions.edit.upgrade.downloading")
            : canDelta
              ? t("versions.edit.upgrade.download_full")
              : t("versions.edit.upgrade.download")}
        </ModalAction>
      )}
      {selected && selectedInstaller && (
        <ModalAction variant="primary" isPending={upgrading} onPress={() => void runUpgrade("installer")}>
          {isDowngrade ? t("versions.edit.upgrade.confirm_downgrade") : t("versions.edit.upgrade.confirm")}
        </ModalAction>
      )}
    </>
  );

  return (
    <>
      <section className="flex flex-col gap-3">
        <div className="flex flex-col gap-1">
          <p className="font-medium text-foreground dark:text-zinc-200">{t("versions.edit.upgrade.title")}</p>
          <p className="text-xs text-muted dark:text-zinc-400 max-w-2xl">{t("versions.edit.upgrade.hint")}</p>
        </div>
        <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
          <p className="text-sm text-foreground dark:text-zinc-300 leading-7 max-w-2xl">
            {t("versions.edit.upgrade.section_body", { version: gameVersion || "-" })}
          </p>
          <div className="shrink-0">
            <Button
              isDisabled={isDisabled || !targetName}
              onPress={openModal}
              variant="primary"
              className={cn("rounded-full", "font-medium")}
            >
              {t("versions.edit.upgrade.button")}
            </Button>
          </div>
        </div>
      </section>

      <UnifiedModal
        isOpen={open}
        onOpenChange={(next) => {
          if (!next) closeModal();
        }}
        size="wide"
        type={result ? "success" : isDowngrade ? "warning" : "primary"}
        title={result ? t("versions.edit.upgrade.success_title") : t("versions.edit.upgrade.modal_title", { name: targetName })}
        footer={footer}
        isKeyboardDismissDisabled={upgrading}
        contentKey={result ? "result" : upgrading ? "upgrading" : "pick"}
      >
        {result ? (
          <>
            <ModalDescription>
              {t("versions.edit.upgrade.success_body", { name: targetName, from: result.from, to: result.to })}
            </ModalDescription>
            {result.backupPath && (
              <ModalPanel className="font-mono [overflow-wrap:anywhere]">
                {t("versions.edit.upgrade.backup_saved", { path: result.backupPath })}
              </ModalPanel>
            )}
          </>
        ) : upgrading ? (
          <ModalProgress
            label={t("versions.edit.upgrade.progress_label")}
            description={
              deltaStats
                ? t(`versions.edit.upgrade.delta_phase_${deltaStats.phase || "verify"}`, { version: selected?.short || "" })
                : progress
                  ? t("versions.edit.upgrade.phase_extract", { version: selected?.short || "" })
                  : backup
                    ? t("versions.edit.upgrade.phase_backup")
                    : t("versions.edit.upgrade.phase_prepare")
            }
            value={
              deltaStats?.phase === "download" && deltaStats.toDownload > 0
                ? Math.min(100, (deltaStats.downloaded / deltaStats.toDownload) * 100)
                : deltaStats?.phase === "verify" || deltaStats?.phase === "finish"
                  ? undefined
                  : progressValue
            }
            detail={
              deltaStats && deltaStats.total > 0
                ? t("versions.edit.upgrade.delta_detail", {
                    reused: formatBytes(deltaStats.reused),
                    total: formatBytes(deltaStats.total),
                    downloaded: formatBytes(deltaStats.downloaded),
                    toDownload: deltaStats.phase === "compare" ? "…" : formatBytes(deltaStats.toDownload),
                  })
                : progress && progress.total > 0
                  ? `${formatBytes(progress.current)} / ${formatBytes(progress.total)}`
                  : undefined
            }
            currentItem={deltaStats ? undefined : progress?.file || undefined}
          />
        ) : (
          <>
            <ModalDescription>
              {t("versions.edit.upgrade.modal_body", { version: gameVersion || "-" })}
            </ModalDescription>
            <TextField aria-label={t("versions.edit.upgrade.search") as string} value={query} onChange={setQuery}>
              <Input placeholder={t("versions.edit.upgrade.search") as string} className="rounded-lg" />
            </TextField>
            <div
              role="listbox"
              aria-label={t("versions.edit.upgrade.list_label") as string}
              className="flex max-h-72 flex-col gap-1 overflow-y-auto rounded-xl border border-border/70 p-1"
            >
              {catalogLoading ? (
                <div className="flex items-center justify-center gap-2 py-8 text-sm text-muted">
                  <Spinner size="sm" /> {t("versions.edit.upgrade.loading")}
                </div>
              ) : catalogError ? (
                <div className="flex flex-col items-center gap-2 py-6 text-sm text-danger">
                  {catalogError}
                  <Button size="sm" variant="secondary" onPress={() => void loadCatalog()}>
                    {t("common.retry")}
                  </Button>
                </div>
              ) : candidates.length === 0 ? (
                <div className="py-8 text-center text-sm text-muted">{t("versions.edit.upgrade.empty")}</div>
              ) : (
                candidates.map((item) => {
                  const key = keyOf(item);
                  const isSelected = selected ? keyOf(selected) === key : false;
                  const newer = compareVersions(item.short, gameVersion) > 0;
                  const dl = downloadFor(item);
                  const pct =
                    dl?.progress && dl.progress.total > 0
                      ? Math.round((dl.progress.downloaded / dl.progress.total) * 100)
                      : null;
                  return (
                    <button
                      key={key}
                      type="button"
                      role="option"
                      aria-selected={isSelected}
                      onClick={() => setSelected(item)}
                      className={cn(
                        "flex w-full items-center justify-between gap-3 rounded-lg px-3 py-2 text-left text-sm transition-colors",
                        isSelected
                          ? "bg-brand-500/15 text-foreground ring-1 ring-brand-500/40"
                          : "hover:bg-surface-secondary text-foreground/90",
                      )}
                    >
                      <span className="flex min-w-0 items-center gap-2">
                        {newer ? (
                          <FiArrowUp className="shrink-0 text-brand-500" aria-hidden="true" />
                        ) : (
                          <FiArrowDown className="shrink-0 text-amber-500" aria-hidden="true" />
                        )}
                        <span className="font-mono tabular-nums">{item.short}</span>
                        {item.type !== "Release" && item.type !== "Preview" && (
                          <Chip size="sm" variant="soft" color="accent">
                            <Chip.Label>{item.type}</Chip.Label>
                          </Chip>
                        )}
                      </span>
                      <span className="shrink-0 text-xs text-muted">
                        {downloaded[key] || dl?.status === "done" ? (
                          <span className="inline-flex items-center gap-1 text-success">
                            <FiCheckCircle aria-hidden="true" />
                            {t("versions.edit.upgrade.downloaded")}
                          </span>
                        ) : dl && dl.status === "queued" ? (
                          t("download_manager.queue.queued")
                        ) : dl && dl.status === "paused" ? (
                          t("download_manager.queue.paused")
                        ) : dl && isDownloadPending(dl.status) ? (
                          `${t("versions.edit.upgrade.downloading")}${pct !== null ? ` ${pct}%` : ""}`
                        ) : newer ? (
                          t("versions.edit.upgrade.newer")
                        ) : (
                          t("versions.edit.upgrade.older")
                        )}
                      </span>
                    </button>
                  );
                })
              )}
            </div>

            {selected && !selectedInstaller && (
              <ModalNotice tone="info">
                {t(canDelta ? "versions.edit.upgrade.delta_hint" : "versions.edit.upgrade.download_hint")}
              </ModalNotice>
            )}
            {isDowngrade && <ModalNotice tone="warning">{t("versions.edit.upgrade.downgrade_warning")}</ModalNotice>}
            {leviLaminaVersion && (
              <ModalNotice tone="warning">
                {t("versions.edit.upgrade.leviLamina_warning", { version: leviLaminaVersion })}
              </ModalNotice>
            )}

            <Checkbox isSelected={backup} onChange={setBackup} className="group">
              <Checkbox.Content>
                <Checkbox.Control>
                  <Checkbox.Indicator />
                </Checkbox.Control>
                <span className="text-sm text-foreground dark:text-zinc-200">
                  {t("versions.edit.upgrade.backup_first")}
                </span>
              </Checkbox.Content>
            </Checkbox>
            <p className="-mt-2 text-xs text-muted">{t("versions.edit.upgrade.backup_hint")}</p>
          </>
        )}
        {error && !upgrading && <ModalNotice tone="danger">{resolveInstallError(error, t)}</ModalNotice>}
      </UnifiedModal>
    </>
  );
};
