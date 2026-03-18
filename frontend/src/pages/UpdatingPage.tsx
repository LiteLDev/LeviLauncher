import React, { useEffect, useMemo, useRef, useState } from "react";
import { Card, CardBody, Progress } from "@heroui/react";
import { useTranslation } from "react-i18next";
import { FaRocket } from "react-icons/fa";
import { motion } from "framer-motion";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
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
          <CardBody className="p-6">
            <PageHeader
              title={t("updating.title")}
              description={t("updating.body")}
              titleClassName="text-2xl"
              descriptionClassName="text-sm sm:text-base"
              startContent={
                <div className="p-2 rounded-xl bg-primary-500/10 text-primary-600 dark:text-primary-500">
                  <FaRocket className="w-5 h-5" />
                </div>
              }
            />

            <div className="mt-6 rounded-3xl border border-black/5 dark:border-white/10 bg-white/50 dark:bg-zinc-950/50 backdrop-blur-sm px-4 py-3">
              <div className="text-xs font-semibold uppercase tracking-wide text-default-500">
                {t("common.updating")}
              </div>
              <div className="mt-1 text-sm font-medium text-default-700 dark:text-zinc-200">
                {statusText}
              </div>
            </div>

            <div className="mt-8 space-y-6 w-full pb-4">
              <div className="space-y-2">
                <div className="flex justify-between items-end gap-4">
                  <div className="text-small font-medium text-default-600">
                    {t("updating.phase.download")}
                  </div>
                  <div className="text-tiny text-default-500 font-mono text-right">
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
                  <Progress
                    aria-label={t("updating.phase.download")}
                    className="w-full"
                    color="success"
                    size="md"
                    radius="md"
                    value={Math.max(
                      0,
                      Math.min(100, Math.round((downloaded / total) * 100)),
                    )}
                    classNames={{
                      indicator:
                        "bg-gradient-to-r from-primary-500 to-primary-400",
                    }}
                  />
                ) : (
                  <Progress
                    isIndeterminate={currentStatus !== "downloaded"}
                    aria-label={t("updating.phase.download")}
                    className="w-full"
                    color="success"
                    size="md"
                    radius="md"
                    value={currentStatus === "downloaded" ? 100 : undefined}
                    classNames={{
                      indicator:
                        "bg-gradient-to-r from-primary-500 to-primary-400",
                    }}
                  />
                )}
              </div>

              <div className="space-y-2">
                <div className="flex justify-between items-end gap-4">
                  <div className="text-small font-medium text-default-600">
                    {t("updating.phase.install")}
                  </div>
                  <div className="text-tiny text-default-500 text-right">
                    {statusText}
                  </div>
                </div>
                {installBusy ? (
                  <Progress
                    isIndeterminate
                    aria-label={t("updating.phase.install")}
                    className="w-full"
                    color="success"
                    size="md"
                    radius="md"
                    classNames={{
                      indicator:
                        "bg-gradient-to-r from-primary-500 to-primary-400",
                    }}
                  />
                ) : (
                  <Progress
                    aria-label={t("updating.phase.install")}
                    className="w-full"
                    color="success"
                    size="md"
                    radius="md"
                    value={currentStatus === "installed" ? 100 : 0}
                    classNames={{
                      indicator:
                        "bg-gradient-to-r from-primary-500 to-primary-400",
                    }}
                  />
                )}
              </div>
            </div>
          </CardBody>
        </Card>
      </motion.div>

      <UnifiedModal
        size="md"
        isOpen={!!errorCode}
        type="error"
        title={t("updating.failed_title")}
        confirmText={t("common.confirm")}
        onConfirm={() => Window.Close()}
        showCancelButton={false}
        hideCloseButton
        isDismissable={false}
      >
        <div className="text-default-700 text-sm wrap-break-word whitespace-pre-wrap">
          {errorText}
        </div>
      </UnifiedModal>
    </PageContainer>
  );
}
