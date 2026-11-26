import React, { useCallback, useEffect, useMemo, useState } from "react";
import { Button, Chip, Spinner, toast } from "@heroui/react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { FaPuzzlePiece } from "react-icons/fa";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import { EnableMod } from "bindings/github.com/liteldev/LeviLauncher/internal/app/modsservice";
import { routeTo } from "@/constants/routes";
import {
  fetchLIPPackageDetailDirect,
  type LIPPackageDetail,
} from "@/utils/content";
import { useLipTaskConsole } from "@/utils/LipTaskConsoleContext";
import { useModIntelligence } from "@/utils/ModIntelligenceContext";
import {
  normalizeIdentifierLookupKey,
  type LIPPackageInstallState,
} from "@/utils/modIntelligenceResolver";
import {
  LEVI_WINE_BRIDGE_IDENTIFIER,
  LEVI_WINE_BRIDGE_NAME,
  detectWineBridgeInstanceState,
  hasWineBridgeUpdate,
  resolveWineBridgeOptionState,
  type WineBridgeCatalogStatus,
} from "@/utils/wineBridge";

type WineBridgeLoaderRowProps = {
  targetName: string;
  isLLInstalled: boolean;
  currentLLVersion: string;
  busy?: boolean;
};

export const WineBridgeLoaderRow: React.FC<WineBridgeLoaderRowProps> = ({
  targetName,
  isLLInstalled,
  currentLLVersion,
  busy = false,
}) => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { runWithLipTask } = useLipTaskConsole();
  const {
    ensureInstanceHydrated,
    getInstanceSnapshot,
    ensurePackageInstallState,
    refreshInstance,
    snapshotRevision,
  } = useModIntelligence();

  const [catalogStatus, setCatalogStatus] =
    useState<WineBridgeCatalogStatus>("idle");
  const [pkgDetail, setPkgDetail] = useState<LIPPackageDetail | null>(null);
  const [reloadToken, setReloadToken] = useState(0);
  const [lipState, setLipState] = useState<LIPPackageInstallState | null>(null);
  const [stateLoading, setStateLoading] = useState(false);
  const [installing, setInstalling] = useState(false);
  const [enabling, setEnabling] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    let active = true;
    setCatalogStatus("loading");
    fetchLIPPackageDetailDirect(LEVI_WINE_BRIDGE_IDENTIFIER, {
      forceRefresh: reloadToken > 0,
      signal: controller.signal,
    })
      .then((detail) => {
        if (!active || controller.signal.aborted) return;
        setPkgDetail(detail);
        setCatalogStatus(detail ? "ready" : "error");
      })
      .catch(() => {
        if (!active || controller.signal.aborted) return;
        setPkgDetail(null);
        setCatalogStatus("error");
      });

    return () => {
      active = false;
      controller.abort();
    };
  }, [reloadToken]);

  const syncInstanceState = useCallback(
    async (forceRefresh = false) => {
      const name = String(targetName || "").trim();
      if (!name) return;
      setStateLoading(true);
      try {
        if (forceRefresh) {
          await refreshInstance(name, "version-settings-winebridge");
        } else {
          await ensureInstanceHydrated(name, {
            background: true,
            reason: "version-settings-winebridge",
          });
        }
        const pkgState = await ensurePackageInstallState(
          name,
          LEVI_WINE_BRIDGE_IDENTIFIER,
        );
        setLipState(pkgState);
      } catch {
        // Keep best-effort snapshot state on failure.
      } finally {
        setStateLoading(false);
      }
    },
    [ensureInstanceHydrated, ensurePackageInstallState, refreshInstance, targetName],
  );

  useEffect(() => {
    let cancelled = false;
    const name = String(targetName || "").trim();
    if (!name) return;
    setStateLoading(true);
    Promise.all([
      ensureInstanceHydrated(name, {
        background: true,
        reason: "version-settings-winebridge-init",
      }),
      ensurePackageInstallState(name, LEVI_WINE_BRIDGE_IDENTIFIER),
    ])
      .then(([, pkgState]) => {
        if (!cancelled) {
          setLipState(pkgState);
        }
      })
      .catch(() => {})
      .finally(() => {
        if (!cancelled) {
          setStateLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [
    ensureInstanceHydrated,
    ensurePackageInstallState,
    isLLInstalled,
    currentLLVersion,
    targetName,
  ]);

  const snapshot = useMemo(
    () => (targetName ? getInstanceSnapshot(targetName) : null),
    [getInstanceSnapshot, snapshotRevision, targetName],
  );

  const effectiveLipState = useMemo(() => {
    const lookupKey = normalizeIdentifierLookupKey(LEVI_WINE_BRIDGE_IDENTIFIER);
    const fromSnapshot = snapshot?.lipInstallStateByIdentifier?.get(lookupKey);
    if (fromSnapshot?.installed) return fromSnapshot;
    return lipState ?? fromSnapshot ?? null;
  }, [lipState, snapshot]);

  const instanceState = useMemo(
    () =>
      detectWineBridgeInstanceState({
        modsInfo: snapshot?.modsInfo,
        enabledByFolder: snapshot?.enabledByFolder,
        lipState: effectiveLipState,
        modLIPStateByFolder: snapshot?.modLIPStateByFolder,
      }),
    [effectiveLipState, snapshot],
  );

  const optionState = useMemo(
    () =>
      resolveWineBridgeOptionState({
        catalogStatus,
        pkg: pkgDetail,
        llVersion: currentLLVersion,
        userEnabled: true,
      }),
    [catalogStatus, currentLLVersion, pkgDetail],
  );

  const updateAvailable = useMemo(
    () =>
      instanceState.status === "enabled" &&
      Boolean(optionState.candidate?.version) &&
      hasWineBridgeUpdate(
        instanceState.installedVersion,
        optionState.candidate?.version || "",
      ),
    [
      instanceState.installedVersion,
      instanceState.status,
      optionState.candidate?.version,
    ],
  );

  const handleInstallOrUpdate = useCallback(async () => {
    const name = String(targetName || "").trim();
    const candidate = optionState.candidate;
    if (!name || !candidate || installing || enabling || busy) return;

    const installPackage = (minecraft as any)?.InstallLIPPackage;
    if (typeof installPackage !== "function") return;

    const isUpdate = instanceState.status !== "not_installed";
    setInstalling(true);
    try {
      await runWithLipTask(
        {
          action: isUpdate ? "upgrade" : "install",
          target: name,
          methods: ["Install", "Update"],
          feedbackMode: "progress",
        },
        async ({ addLog }) => {
          addLog("info", `Install ${candidate.identifier}@${candidate.version}`);
          const err: string = await installPackage(
            name,
            candidate.identifier,
            candidate.version,
          );
          if (String(err || "").trim()) {
            throw new Error(String(err));
          }
        },
      );
      await syncInstanceState(true);
      toast(
        t(
          isUpdate
            ? "versions.edit.loader.wine_bridge_update_success"
            : "versions.edit.loader.wine_bridge_install_success",
        ) as string,
        {
          variant: "success",
          timeout: 2500,
        },
      );
    } catch {
      toast(t("common.error") as string, {
        description: t("versions.edit.loader.wine_bridge_install_failed") as string,
        variant: "danger",
        timeout: 3000,
      });
    } finally {
      setInstalling(false);
    }
  }, [
    busy,
    enabling,
    installing,
    instanceState.status,
    optionState.candidate,
    runWithLipTask,
    syncInstanceState,
    t,
    targetName,
  ]);

  const handleEnable = useCallback(async () => {
    const name = String(targetName || "").trim();
    if (
      !name ||
      instanceState.disabledFolders.length === 0 ||
      enabling ||
      installing ||
      busy
    ) {
      return;
    }

    setEnabling(true);
    try {
      for (const folder of instanceState.disabledFolders) {
        const err = await (EnableMod as any)?.(name, folder);
        if (String(err || "").trim()) {
          throw new Error(String(err));
        }
      }
      await syncInstanceState(true);
      toast(t("versions.edit.loader.wine_bridge_enable_success") as string, {
        variant: "success",
        timeout: 2500,
      });
    } catch {
      toast(t("common.error") as string, {
        description: t("versions.edit.loader.wine_bridge_enable_failed") as string,
        variant: "danger",
        timeout: 3000,
      });
    } finally {
      setEnabling(false);
    }
  }, [
    busy,
    enabling,
    installing,
    instanceState.disabledFolders,
    syncInstanceState,
    t,
    targetName,
  ]);

  const handleViewDetails = useCallback(() => {
    navigate(routeTo.lipPackage(LEVI_WINE_BRIDGE_IDENTIFIER));
  }, [navigate]);

  const actionsDisabled = busy || installing || enabling;
  const versionText = instanceState.installedVersion
    ? `v${instanceState.installedVersion}`
    : "";

  return (
    <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
      <div className="flex items-start gap-4 min-w-0">
        <div className="w-12 h-12 rounded-xl overflow-hidden shrink-0 bg-brand-500/10 text-brand-600 dark:text-brand-400 flex items-center justify-center">
          {pkgDetail?.avatarUrl ? (
            <img
              src={pkgDetail.avatarUrl}
              alt={LEVI_WINE_BRIDGE_NAME}
              className="w-full h-full object-cover"
            />
          ) : (
            <FaPuzzlePiece className="w-5 h-5" />
          )}
        </div>
        <div className="min-w-0 space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <div className="text-base font-bold text-foreground">
              {LEVI_WINE_BRIDGE_NAME}
            </div>
            {stateLoading && !snapshot ? (
              <Spinner size="sm" />
            ) : instanceState.status === "enabled" ? (
              <>
                <Chip size="sm" variant="soft" color="success" className="h-5 px-1.5">
                  <Chip.Label className="text-[11px] font-semibold">
                    {t("versions.edit.loader.wine_bridge_status_enabled")}
                  </Chip.Label>
                </Chip>
                {versionText && (
                  <span className="text-sm font-semibold text-muted dark:text-zinc-400 font-mono">
                    {versionText}
                  </span>
                )}
              </>
            ) : instanceState.status === "disabled" ? (
              <>
                <Chip size="sm" variant="soft" color="warning" className="h-5 px-1.5">
                  <Chip.Label className="text-[11px] font-semibold">
                    {t("versions.edit.loader.wine_bridge_status_disabled")}
                  </Chip.Label>
                </Chip>
                {versionText && (
                  <span className="text-sm font-semibold text-muted dark:text-zinc-400 font-mono">
                    {versionText}
                  </span>
                )}
              </>
            ) : (
              <>
                <Chip size="sm" variant="soft" color="accent" className="h-5 px-1.5">
                  <Chip.Label className="text-[11px] font-semibold">
                    {t("downloadpage.install.wine_bridge_recommended")}
                  </Chip.Label>
                </Chip>
                <span className="text-sm font-medium text-muted dark:text-zinc-500">
                  {t("versions.edit.loader.wine_bridge_status_not_installed")}
                </span>
              </>
            )}
          </div>
          <div className="text-sm text-muted dark:text-zinc-400">
            {t("downloadpage.install.wine_bridge_desc")}
          </div>
          {instanceState.status === "not_installed" && (
            <>
              {!isLLInstalled ? (
                <p className="text-xs text-muted dark:text-zinc-400">
                  {t("versions.edit.loader.wine_bridge_requires_ll")}
                </p>
              ) : optionState.statusKind === "loading" ? (
                <div className="flex items-center gap-1.5 text-xs text-muted">
                  <Spinner size="sm" />
                  <span>{t("downloadpage.install.wine_bridge_checking")}</span>
                </div>
              ) : optionState.statusKind === "incompatible" ? (
                <p className="text-xs text-amber-600 dark:text-amber-400">
                  {t("downloadpage.install.wine_bridge_incompatible")}
                </p>
              ) : optionState.statusKind === "unavailable" ? (
                <div className="flex flex-wrap items-center gap-2 text-xs text-amber-600 dark:text-amber-400">
                  <span>{t("downloadpage.install.wine_bridge_unavailable")}</span>
                  <Button
                    size="sm"
                    variant="ghost"
                    className="h-6 min-w-0 px-2 text-xs"
                    onPress={() => setReloadToken((n) => n + 1)}
                  >
                    {t("common.retry")}
                  </Button>
                </div>
              ) : null}
            </>
          )}
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-2 shrink-0">
        <Button
          variant="ghost"
          isDisabled={actionsDisabled}
          onPress={handleViewDetails}
        >
          {t("versions.edit.loader.wine_bridge_view_details")}
        </Button>

        {instanceState.status === "not_installed" && (
          <Button
            onPress={() => void handleInstallOrUpdate()}
            isPending={installing}
            isDisabled={
              actionsDisabled || !isLLInstalled || !optionState.candidate
            }
            variant="secondary"
            className="bg-brand-500/10 text-brand-600 dark:text-brand-400 font-bold"
          >
            {({ isPending }) => (
              <>
                <Spinner
                  size="sm"
                  color="current"
                  className={isPending ? "" : "hidden"}
                />
                {t("versions.edit.loader.wine_bridge_install")}
              </>
            )}
          </Button>
        )}

        {instanceState.status === "disabled" && (
          <Button
            onPress={() => void handleEnable()}
            isPending={enabling}
            isDisabled={
              actionsDisabled || instanceState.disabledFolders.length === 0
            }
            variant="secondary"
            className="bg-brand-500/10 text-brand-600 dark:text-brand-400 font-bold"
          >
            {({ isPending }) => (
              <>
                <Spinner
                  size="sm"
                  color="current"
                  className={isPending ? "" : "hidden"}
                />
                {t("common.enable")}
              </>
            )}
          </Button>
        )}

        {updateAvailable && optionState.candidate && (
          <Button
            onPress={() => void handleInstallOrUpdate()}
            isPending={installing}
            isDisabled={actionsDisabled || !isLLInstalled}
            variant="secondary"
            className="bg-brand-500/10 text-brand-600 dark:text-brand-400 font-bold"
          >
            {({ isPending }) => (
              <>
                <Spinner
                  size="sm"
                  color="current"
                  className={isPending ? "" : "hidden"}
                />
                {t("mods.action_update")}
              </>
            )}
          </Button>
        )}
      </div>
    </div>
  );
};
