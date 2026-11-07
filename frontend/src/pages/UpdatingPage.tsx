import { ModalNotice } from "@/components/ModalPrimitives";
import { Button, Card, ProgressBar, Spinner } from "@heroui/react";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { useTranslation } from "react-i18next";
import { FaRocket } from "react-icons/fa";
import { FiCopy, FiRefreshCw } from "react-icons/fi";
import { motion } from "framer-motion";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import { Events } from "@wailsio/runtime";
import { PageContainer } from "@/components/PageContainer";
import { PageHeader } from "@/components/PageHeader";
import { LAYOUT } from "@/constants/layout";
import { cn } from "@/utils/cn";
import {
  appendTaskLog,
  finishActivityTask,
  notify,
  setTaskProgress,
  startActivityTask,
} from "@/utils/activity";

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

type UpdateLogLine = {
  at: number;
  level: "info" | "success" | "error";
  text: string;
};

type UpdateFailure = ParsedUpdateError & { at: number };

const DOWNLOAD_LOG_STEPS = [25, 50, 75, 100];

const formatMb = (bytes: number) => (bytes / 1024 / 1024).toFixed(1);

const formatTime = (at: number) =>
  new Date(at).toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });

export default function UpdatingPage() {
  const { t } = useTranslation();
  // Event handlers outlive renders; read the latest `t` through a ref so a
  // language change does not resubscribe (and cancel) the running task.
  const tRef = useRef(t);
  tRef.current = t;
  const mountedRef = useRef(false);
  const taskIdRef = useRef<string | null>(null);
  const loggedDownloadStepRef = useRef(0);
  const lastStatusRef = useRef("");
  const startedAttemptRef = useRef(-1);
  const [running, setRunning] = useState<boolean>(true);
  const [failure, setFailure] = useState<UpdateFailure | null>(null);
  const [status, setStatus] = useState<string>("checking");
  const [downloaded, setDownloaded] = useState<number>(0);
  const [total, setTotal] = useState<number>(0);
  // Every step stays listed, so a status that flashed by can still be read.
  const [log, setLog] = useState<UpdateLogLine[]>([]);
  const [attempt, setAttempt] = useState(0);
  const [copyState, setCopyState] = useState<"idle" | "copied" | "failed">("idle");

  const currentStatus = status || (running ? "checking" : "");
  const statusText = useMemo(
    () => getStatusText(currentStatus, t),
    [currentStatus, t],
  );

  const appendLog = useCallback(
    (text: string, level: UpdateLogLine["level"] = "info") => {
      if (!mountedRef.current) return;
      setLog((prev) => [...prev, { at: Date.now(), level, text }]);
      if (taskIdRef.current) {
        appendTaskLog(
          taskIdRef.current,
          text,
          level === "error" ? "error" : level,
        );
      }
    },
    [],
  );

  const fail = useCallback(
    (parsed: ParsedUpdateError) => {
      if (!mountedRef.current) return;
      const t = tRef.current;
      const errorText = t(getUpdateErrorKey(parsed.code));
      // The first error event carries the detail; a later generic failure from
      // the same attempt must not overwrite it.
      setFailure((prev) => prev ?? { ...parsed, at: Date.now() });
      setCopyState("idle");
      if (taskIdRef.current) {
        appendLog(
          parsed.detail ? `${errorText} (${parsed.detail})` : errorText,
          "error",
        );
        finishActivityTask(taskIdRef.current, "error", errorText);
        notify({
          title: t("updating.failed_title"),
          description: errorText,
          variant: "danger",
          source: "updates",
          silent: true,
        });
        taskIdRef.current = null;
      }
    },
    [appendLog],
  );

  useEffect(() => {
    mountedRef.current = true;
    const t = (key: string, options?: Record<string, unknown>) =>
      tRef.current(key, options);

    const offStatus = Events.On("app_update_status", (event) => {
      const nextStatus = String(event?.data || "").trim();
      if (!nextStatus || !mountedRef.current) return;
      setStatus(nextStatus);
      if (nextStatus === lastStatusRef.current) return;
      lastStatusRef.current = nextStatus;
      const text = getStatusText(nextStatus, t);
      appendLog(text, nextStatus === "installed" ? "success" : "info");
      if (nextStatus === "installed" && taskIdRef.current) {
        setTaskProgress(taskIdRef.current, 100);
      }
    });

    const offProgress = Events.On("app_update_progress", (event) => {
      const data = event?.data;
      if (!data || typeof data !== "object" || !mountedRef.current) return;
      const payload = data as unknown as {
        phase?: unknown;
        downloaded?: unknown;
        total?: unknown;
      };
      const phase = String(payload.phase || "").trim();
      if (phase !== "download") return;
      const nextDownloaded = Math.max(0, Number(payload.downloaded || 0) || 0);
      const nextTotal = Math.max(0, Number(payload.total || 0) || 0);
      setDownloaded(nextDownloaded);
      setTotal(nextTotal);
      if (nextTotal <= 0) return;
      const percent = Math.min(100, Math.floor((nextDownloaded / nextTotal) * 100));
      if (taskIdRef.current) setTaskProgress(taskIdRef.current, percent);
      // Log quarter milestones instead of every byte so the list stays short.
      const step = DOWNLOAD_LOG_STEPS.filter((value) => percent >= value).pop() ?? 0;
      if (step > loggedDownloadStepRef.current) {
        loggedDownloadStepRef.current = step;
        appendLog(
          t("settings_ux.updating.download_line", {
            percent: step,
            downloaded: formatMb(nextDownloaded),
            total: formatMb(nextTotal),
          }),
        );
      }
    });

    const offError = Events.On("app_update_error", (event) => {
      const parsed = parseUpdateError(event?.data);
      if (parsed.detail) {
        console.error("Update failed detail:", parsed.detail);
      }
      fail(parsed);
    });

    return () => {
      mountedRef.current = false;
      offStatus();
      offProgress();
      offError();
      // The update keeps running in the backend after an unmount, so its
      // activity task is left open rather than marked cancelled.
    };
  }, [appendLog, fail]);

  useEffect(() => {
    // Start each attempt once, even if the effect is replayed (StrictMode).
    if (!mountedRef.current || startedAttemptRef.current === attempt) return;
    startedAttemptRef.current = attempt;
    const runUpdate = async () => {
      setRunning(true);
      setFailure(null);
      setStatus("checking");
      setDownloaded(0);
      setTotal(0);
      loggedDownloadStepRef.current = 0;
      lastStatusRef.current = "checking";
      taskIdRef.current = startActivityTask({
        title: t("updating.title"),
        source: "updates",
      });
      appendLog(
        attempt > 0
          ? t("settings_ux.updating.retry_line", { count: attempt + 1 })
          : t("updating.status.checking"),
      );
      try {
        const ok = await minecraft?.Update?.();
        if (!ok) {
          fail({ code: "ERR_UPDATE_GENERIC", detail: "" });
        } else if (taskIdRef.current) {
          appendLog(t("common.done"), "success");
          finishActivityTask(taskIdRef.current, "success");
          taskIdRef.current = null;
        }
      } catch (error) {
        console.error("Update request failed:", error);
        fail({
          code: "ERR_UPDATE_GENERIC",
          detail: error instanceof Error ? error.message : String(error ?? ""),
        });
      } finally {
        if (mountedRef.current) setRunning(false);
      }
    };
    void runUpdate();
    // `attempt` is the only trigger: each retry starts a new run.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [attempt]);

  const failureText = failure ? t(getUpdateErrorKey(failure.code)) : "";
  const failureReport = failure
    ? [
        `${t("updating.failed_title")}: ${failureText}`,
        `${t("settings_ux.updating.error_code")}: ${failure.code}`,
        ...(failure.detail
          ? [`${t("settings_ux.updating.details")}: ${failure.detail}`]
          : []),
        "",
        ...log.map((line) => `[${formatTime(line.at)}] ${line.text}`),
      ].join("\n")
    : "";

  const copyFailure = async () => {
    try {
      await navigator.clipboard.writeText(failureReport);
      setCopyState("copied");
    } catch {
      setCopyState("failed");
    }
  };

  const installBusy = [
    "installing",
    "verifying",
    "elevating",
    "restarting",
  ].includes(currentStatus);

  return (
    <PageContainer>
      <motion.div
        data-material-motion
        initial={{ opacity: 0, y: -20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4 }}
        className={LAYOUT.PAGE.READABLE}
      >
        <Card className={LAYOUT.GLASS_CARD.BASE}>
          <Card.Content className="p-4 min-[1200px]:p-6">
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

            {failure && (
              <ModalNotice role="alert" tone="danger" className="mt-6">
                <p className="font-semibold">{t("updating.failed_title")}</p>
                <p className="select-text">{failureText}</p>
                <dl className="mt-2 grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1 text-xs">
                  <dt className="opacity-80">{t("settings_ux.updating.error_code")}</dt>
                  <dd className="select-text font-mono">{failure.code}</dd>
                  {failure.detail && (
                    <>
                      <dt className="opacity-80">{t("settings_ux.updating.details")}</dt>
                      <dd className="select-text font-mono [overflow-wrap:anywhere]">{failure.detail}</dd>
                    </>
                  )}
                </dl>
                <p className="mt-2 text-xs opacity-90">{t("audit.mods.update_recovery")}</p>
                <div className="mt-3 flex flex-wrap gap-2">
                  {/* After a failed restart the new binary is already installed,
                      so running the update again would only reinstall it. */}
                  {failure.code !== "ERR_UPDATE_RESTART_FAILED" && (
                    <Button
                      size="sm"
                      variant="primary"
                      isDisabled={running}
                      onPress={() => setAttempt((value) => value + 1)}
                    >
                      <FiRefreshCw aria-hidden />
                      {t("common.retry")}
                    </Button>
                  )}
                  <Button size="sm" variant="secondary" onPress={() => void copyFailure()}>
                    <FiCopy aria-hidden />
                    {t("audit.primary.install.copy_details")}
                  </Button>
                  <Button size="sm" variant="ghost" onPress={() => minecraft.QuitLauncher()}>
                    {t("audit.mods.close_update_window")}
                  </Button>
                </div>
                {copyState !== "idle" && (
                  <p role="status" className="mt-2 text-xs">
                    {copyState === "copied"
                      ? t("audit.primary.install.copied")
                      : t("audit.primary.install.copy_failed")}
                  </p>
                )}
              </ModalNotice>
            )}

            <div className="mt-6 space-y-6 w-full">
              <div className="space-y-2">
                <div className="flex flex-wrap justify-between items-end gap-x-4 gap-y-1">
                  <div className="text-sm font-medium text-foreground">
                    {t("updating.phase.download")}
                  </div>
                  <div className="text-xs text-muted font-mono text-right tabular-nums">
                    {total > 0
                      ? `${formatMb(downloaded)} / ${formatMb(total)} MB`
                      : `${formatMb(downloaded)} MB`}
                  </div>
                </div>
                <ProgressBar
                  aria-label={t("updating.phase.download")}
                  size="md"
                  isIndeterminate={total <= 0 && running && !failure && currentStatus !== "downloaded"}
                  value={
                    total > 0
                      ? Math.max(0, Math.min(100, Math.round((downloaded / total) * 100)))
                      : currentStatus === "downloaded" || currentStatus === "installed" || installBusy
                        ? 100
                        : 0
                  }
                  color={"success"}
                  className={"w-full"}
                >
                  <ProgressBar.Track className={"rounded-md"}>
                    <ProgressBar.Fill className={"bg-accent"} />
                  </ProgressBar.Track>
                </ProgressBar>
              </div>

              <div className="space-y-2">
                <div className="flex flex-wrap justify-between items-end gap-x-4 gap-y-1">
                  <div className="text-sm font-medium text-foreground">
                    {t("updating.phase.install")}
                  </div>
                  <div className="text-xs text-muted text-right">
                    {statusText}
                  </div>
                </div>
                <ProgressBar
                  isIndeterminate={installBusy && !failure}
                  aria-label={t("updating.phase.install")}
                  size="md"
                  value={currentStatus === "installed" ? 100 : 0}
                  color={"success"}
                  className={"w-full"}
                >
                  <ProgressBar.Track className={"rounded-md"}>
                    <ProgressBar.Fill className={"bg-accent"} />
                  </ProgressBar.Track>
                </ProgressBar>
              </div>

              <section aria-labelledby="update-log-title" className="space-y-2">
                <h2 id="update-log-title" className="text-xs font-semibold uppercase tracking-wide text-muted">
                  {t("settings_ux.updating.log_title")}
                </h2>
                <ol
                  aria-live="polite"
                  className="max-h-[min(18rem,40vh)] overflow-y-auto rounded-2xl border border-black/5 dark:border-white/10 bg-surface/50 launcher-material-blur px-4 py-3 space-y-1 text-sm"
                >
                  {log.map((line, index) => (
                    <li key={index} className="flex min-w-0 gap-3">
                      <time className="shrink-0 font-mono text-xs leading-5 text-muted tabular-nums">
                        {formatTime(line.at)}
                      </time>
                      <span
                        className={cn(
                          "min-w-0 select-text [overflow-wrap:anywhere]",
                          line.level === "error" && "text-danger",
                          line.level === "success" && "text-success",
                        )}
                      >
                        {line.text}
                      </span>
                    </li>
                  ))}
                  {running && !failure && (
                    <li className="flex items-center gap-2 text-muted">
                      <Spinner size="sm" />
                      {statusText}
                    </li>
                  )}
                </ol>
              </section>
            </div>
          </Card.Content>
        </Card>
      </motion.div>
    </PageContainer>
  );
}
