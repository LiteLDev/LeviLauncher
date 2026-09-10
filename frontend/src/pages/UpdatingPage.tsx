import { ModalDescription } from "@/components/ModalPrimitives";
import { Card, ProgressBar } from "@heroui/react";

import { useEffect, useMemo, useRef, useState } from "react";

import { useTranslation } from "react-i18next";
import { FaRocket } from "react-icons/fa";
import { motion } from "framer-motion";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import { Events, Window } from "@wailsio/runtime";
import { UnifiedModal } from "@/components/UnifiedModal";
import { PageContainer } from "@/components/PageContainer";
import { PageHeader } from "@/components/PageHeader";
import { LAYOUT } from "@/constants/layout";

const UPDATE_ERROR_SEPARATOR = "::";

type ParsedUpdateError = {
  code: string;
  detail: string;
};

const parseUpdateError = (raw: unknown): ParsedUpdateError => {
  const value = String(raw || "").trim();
  if (!value) {
    return { code: "ERR_UPDATE_GENERIC", detail: "" };
  }
  const [code, ...detailParts] = value.split(UPDATE_ERROR_SEPARATOR);
  return {
    code: String(code || "ERR_UPDATE_GENERIC").trim() || "ERR_UPDATE_GENERIC",
    detail: detailParts.join(UPDATE_ERROR_SEPARATOR).trim(),
  };
};

const getUpdateErrorKey = (code: string) => {
  switch (code) {
    case "ERR_UPDATE_ADMIN_REQUIRED":
      return "updating.errors.update_admin_required";
    case "ERR_UPDATE_ASSET_MISSING":
      return "updating.errors.update_asset_missing";
    case "ERR_UPDATE_RELEASE_UNAVAILABLE":
      return "updating.errors.update_release_unavailable";
    case "ERR_UPDATE_VERIFY_FAILED":
      return "updating.errors.update_verify_failed";
    case "ERR_UPDATE_DOWNLOAD_FAILED":
      return "updating.errors.update_download_failed";
    case "ERR_UPDATE_INSTALL_FAILED":
      return "updating.errors.update_install_failed";
    case "ERR_UPDATE_RESTART_FAILED":
      return "updating.errors.update_restart_failed";
    default:
      return "updating.errors.generic";
  }
};

const getStatusText = (
  status: string,
  t: (key: string, options?: Record<string, unknown>) => string,
) => {
  switch (status) {
    case "checking":
      return t("updating.status.checking");
    case "verifying":
      return t("updating.status.verifying");
    case "elevating":
      return t("updating.status.elevating");
    case "restarting":
      return t("updating.status.restarting");
    case "installing":
      return t("common.processing");
    case "installed":
      return t("common.done");
    case "downloading":
      return t("common.downloading");
    case "downloaded":
      return t("common.completed");
    default:
      return t("common.wait");
  }
};

export default function UpdatingPage() {
  const { t } = useTranslation();
  const mountedRef = useRef(false);
  const [running, setRunning] = useState<boolean>(true);
  const [errorCode, setErrorCode] = useState<string>("");
  const [status, setStatus] = useState<string>("checking");
  const [downloaded, setDownloaded] = useState<number>(0);
  const [total, setTotal] = useState<number>(0);

  const currentStatus = status || (running ? "checking" : "");
  const statusText = useMemo(
    () => getStatusText(currentStatus, t),
    [currentStatus, t],
  );
  const errorText = useMemo(
    () => t(getUpdateErrorKey(errorCode)),
    [errorCode, t],
  );

  useEffect(() => {
    let cancelled = false;
    mountedRef.current = true;

    const safeSet = (updater: () => void) => {
      if (cancelled || !mountedRef.current) return;
      updater();
    };

    const offStatus = Events.On("app_update_status", (event) => {
      const nextStatus = String(event?.data || "").trim();
      if (!nextStatus) return;
      safeSet(() => {
        setStatus(nextStatus);
      });
    });

    const offProgress = Events.On("app_update_progress", (event) => {
      const data = event?.data;
      if (!data || typeof data !== "object") return;
      const payload = data as unknown as {
        phase?: unknown;
        downloaded?: unknown;
        total?: unknown;
      };
      const phase = String(payload.phase || "").trim();
      if (phase !== "download") return;
      const nextDownloaded = Number(payload.downloaded || 0);
      const nextTotal = Number(payload.total || 0);
      safeSet(() => {
        setDownloaded(
          Number.isFinite(nextDownloaded) ? Math.max(0, nextDownloaded) : 0,
        );
        setTotal(Number.isFinite(nextTotal) ? Math.max(0, nextTotal) : 0);
      });
    });

    const offError = Events.On("app_update_error", (event) => {
      const parsed = parseUpdateError(event?.data);
      if (parsed.detail) {
        console.error("Update failed detail:", parsed.detail);
      }
      safeSet(() => {
        setErrorCode(parsed.code);
      });
    });

    const runUpdate = async () => {
      try {
        safeSet(() => {
          setRunning(true);
          setErrorCode("");
          setStatus("checking");
        });
        const ok = await minecraft?.Update?.();
        if (!ok) {
          safeSet(() => {
            setErrorCode((prev) => prev || "ERR_UPDATE_GENERIC");
          });
        }
      } catch (error) {
        console.error("Update request failed:", error);
        safeSet(() => {
          setErrorCode("ERR_UPDATE_GENERIC");
        });
      } finally {
        safeSet(() => {
          setRunning(false);
        });
      }
    };

    void runUpdate();

    return () => {
      cancelled = true;
      mountedRef.current = false;
      offStatus();
      offProgress();
      offError();
    };
  }, []);

  const installBusy = [
    "installing",
    "verifying",
    "elevating",
    "restarting",
  ].includes(currentStatus);

  return (
    <PageContainer>
      <motion.div
        initial={{ opacity: 0, y: -20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4 }}
      >
        <Card className={LAYOUT.GLASS_CARD.BASE}>
          <Card.Content className="p-6">
            <PageHeader
              title={t("updating.title")}
              description={t("updating.body")}
              titleClassName="text-2xl"
              descriptionClassName="text-sm sm:text-base"
              startContent={
                <div className="p-2 rounded-xl bg-brand-500/10 text-brand-600 dark:text-brand-500">
                  <FaRocket className="w-5 h-5" />
                </div>
              }
            />
            <div className="mt-6 rounded-3xl border border-black/5 dark:border-white/10 bg-surface/50 launcher-material-blur px-4 py-3">
              <div className="text-xs font-semibold uppercase tracking-wide text-muted">
                {t("common.updating")}
              </div>
              <div className="mt-1 text-sm font-medium text-foreground dark:text-zinc-200">
                {statusText}
              </div>
            </div>
            <div className="mt-8 space-y-6 w-full pb-4">
              <div className="space-y-2">
                <div className="flex justify-between items-end gap-4">
                  <div className="text-sm font-medium text-foreground">
                    {t("updating.phase.download")}
                  </div>
                  <div className="text-xs text-muted font-mono text-right">
                    {total > 0
                      ? `${(downloaded / 1024 / 1024).toFixed(1)} / ${(
                          total /
                          1024 /
                          1024
                        ).toFixed(1)} MB`
                      : `${(downloaded / 1024 / 1024).toFixed(1)} MB`}
                  </div>
                </div>
                {total > 0 ? (
                  <ProgressBar
                    aria-label={t("updating.phase.download")}
                    size="md"
                    value={Math.max(
                      0,
                      Math.min(100, Math.round((downloaded / total) * 100)),
                    )}
                    color={"success"}
                    className={"w-full"}
                  >
                    <ProgressBar.Track className={"rounded-md"}>
                      <ProgressBar.Fill
                        className={
                          "bg-accent"
                        }
                      />
                    </ProgressBar.Track>
                  </ProgressBar>
                ) : (
                  <ProgressBar
                    isIndeterminate={currentStatus !== "downloaded"}
                    aria-label={t("updating.phase.download")}
                    size="md"
                    value={currentStatus === "downloaded" ? 100 : undefined}
                    color={"success"}
                    className={"w-full"}
                  >
                    <ProgressBar.Track className={"rounded-md"}>
                      <ProgressBar.Fill
                        className={
                          "bg-accent"
                        }
                      />
                    </ProgressBar.Track>
                  </ProgressBar>
                )}
              </div>

              <div className="space-y-2">
                <div className="flex justify-between items-end gap-4">
                  <div className="text-sm font-medium text-foreground">
                    {t("updating.phase.install")}
                  </div>
                  <div className="text-xs text-muted text-right">
                    {statusText}
                  </div>
                </div>
                {installBusy ? (
                  <ProgressBar
                    isIndeterminate
                    aria-label={t("updating.phase.install")}
                    size="md"
                    color={"success"}
                    className={"w-full"}
                  >
                    <ProgressBar.Track className={"rounded-md"}>
                      <ProgressBar.Fill
                        className={
                          "bg-accent"
                        }
                      />
                    </ProgressBar.Track>
                  </ProgressBar>
                ) : (
                  <ProgressBar
                    aria-label={t("updating.phase.install")}
                    size="md"
                    value={currentStatus === "installed" ? 100 : 0}
                    color={"success"}
                    className={"w-full"}
                  >
                    <ProgressBar.Track className={"rounded-md"}>
                      <ProgressBar.Fill
                        className={
                          "bg-accent"
                        }
                      />
                    </ProgressBar.Track>
                  </ProgressBar>
                )}
              </div>
            </div>
          </Card.Content>
        </Card>
      </motion.div>

      <UnifiedModal
        size="standard"
        isOpen={!!errorCode}
        type="error"
        title={t("updating.failed_title")}
        confirmText={t("audit.mods.close_update_window")}
        onConfirm={() => Window.Close()}
        showCancelButton={false}
        isDismissable={false}
      >
        <div role="alert" className="select-text text-foreground text-sm wrap-break-word whitespace-pre-wrap">
          {errorText}
        </div>
        <ModalDescription className="mt-3">
          {t("audit.mods.update_recovery")}
        </ModalDescription>
      </UnifiedModal>
    </PageContainer>
  );
}
