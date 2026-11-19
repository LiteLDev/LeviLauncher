import React from "react";
import { Button, toast } from "@heroui/react";
import { Browser, Clipboard, System } from "@wailsio/runtime";
import { useTranslation } from "react-i18next";
import {
  FiAlertTriangle,
  FiCheckCircle,
  FiInfo,
  FiMinusCircle,
  FiXCircle,
} from "react-icons/fi";
import * as diagnosticsService from "bindings/github.com/liteldev/LeviLauncher/internal/app/diagnosticsservice";
import { UnifiedModal } from "@/components/UnifiedModal";
import {
  ModalAction,
  ModalNotice,
  ModalProgress,
} from "@/components/ModalPrimitives";
import { cn } from "@/utils/cn";
import {
  LEAVING_ACTIONS,
  useFixAction,
  type DiagnosticsRequest,
  isFixActionAvailable,
} from "@/utils/DiagnosticsContext";
import {
  checkActions,
  checkMessage,
  checkParams,
  foldConsequences,
  reportText,
  sortChecks,
  type DiagnosticCheck,
  type DiagnosticReport,
} from "@/utils/diagnostics";
import { faqIndexUrl, type FixActionId } from "@/utils/troubleshoot";

type Props = {
  request: DiagnosticsRequest;
  onClose: () => void;
};

const STATUS_ICON = {
  fail: { Icon: FiXCircle, className: "text-danger" },
  warn: { Icon: FiAlertTriangle, className: "text-warning" },
  info: { Icon: FiInfo, className: "text-accent" },
  ok: { Icon: FiCheckCircle, className: "text-success" },
  skip: { Icon: FiMinusCircle, className: "text-muted" },
} as const;

const StatusIcon: React.FC<{ status: string }> = ({ status }) => {
  const { Icon, className } =
    STATUS_ICON[status as keyof typeof STATUS_ICON] ?? STATUS_ICON.info;
  return <Icon aria-hidden="true" className={cn("mt-0.5 size-4 shrink-0", className)} />;
};

// DiagnosticsModal runs the backend checks for a failure and lists what is
// wrong (with a way to fix each problem), what is worth knowing, and what
// was checked and found fine.
export const DiagnosticsModal: React.FC<Props> = ({ request, onClose }) => {
  const { t, i18n } = useTranslation();
  const isMac = System.IsMac();
  const [open, setOpen] = React.useState(true);
  const [report, setReport] = React.useState<DiagnosticReport | null>(null);
  const [failed, setFailed] = React.useState(false);
  const [elapsed, setElapsed] = React.useState(0);
  const [runId, setRunId] = React.useState(0);
  const runFix = useFixAction(request);

  React.useEffect(() => {
    setReport(null);
    setFailed(false);
    setElapsed(0);
    const started = Date.now();
    const timer = window.setInterval(
      () => setElapsed(Math.floor((Date.now() - started) / 1000)),
      1000,
    );
    const call = diagnosticsService.Run(
      request.topic,
      request.instance || "",
      request.channel || "",
    );
    let cancelled = false;
    call
      .then((result) => {
        if (!cancelled) setReport(result as unknown as DiagnosticReport);
      })
      .catch(() => {
        if (!cancelled) setFailed(true);
      })
      .finally(() => window.clearInterval(timer));
    return () => {
      cancelled = true;
      window.clearInterval(timer);
      call.cancel?.();
    };
  }, [request, runId]);

  const close = () => {
    setOpen(false);
    onClose();
  };

  const running = !report && !failed;
  const { checks, causedBy } = React.useMemo(
    () => foldConsequences(sortChecks(report?.checks ?? [])),
    [report],
  );
  const problems = checks.filter((c) => c.status === "fail" || c.status === "warn");
  const notes = checks.filter((c) => c.status === "info");
  const passed = checks.filter((c) => c.status === "ok" || c.status === "skip");
  const failures = problems.filter((c) => c.status === "fail").length;

  const copyReport = async () => {
    if (!report) return;
    try {
      await Clipboard.SetText(reportText(report, t));
      toast(t("diagnostics.copied"), { variant: "success", timeout: 2000 });
    } catch {
      toast(t("errorDetail.copy_failed"), { variant: "danger" });
    }
  };

  const onAction = (action: FixActionId) => {
    if (LEAVING_ACTIONS.has(action)) close();
    void runFix(action);
  };

  const scope = request.instance
    ? t("diagnostics.scope_instance", {
        topic: t(`diagnostics.topic.${request.topic}`),
        instance: request.instance,
      })
    : t(`diagnostics.topic.${request.topic}`);

  const title = running
    ? t("diagnostics.title_running")
    : failed
      ? t("diagnostics.title_failed")
      : problems.length > 0
        ? t("diagnostics.title_problems", { count: problems.length })
        : t("diagnostics.title_ok");
  const type = running
    ? "primary"
    : failed || failures > 0
      ? "error"
      : problems.length > 0
        ? "warning"
        : "success";

  const renderCheck = (check: DiagnosticCheck, withActions: boolean) => {
    const actions = withActions ? checkActions(check).filter(isFixActionAvailable) : [];
    const params = checkParams(check, t, i18n.language);
    return (
      <li key={check.id} className="flex min-w-0 items-start gap-3 py-3 first:pt-0 last:pb-0">
        <StatusIcon status={check.status} />
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <div className="text-sm font-semibold text-foreground">
            {t(`diagnostics.check.${check.id}.title`, params)}
            <span className="sr-only">
              {" "}
              ({t(`diagnostics.status.${check.status}`)})
            </span>
          </div>
          <p className="text-sm leading-6 text-foreground/80 [overflow-wrap:anywhere]">
            {checkMessage(check, t, params)}
          </p>
          {check.detail && withActions && (
            <p className="select-text font-mono text-xs leading-5 text-muted [overflow-wrap:anywhere]">
              {check.detail}
            </p>
          )}
          {withActions && causedBy[check.id] && (
            <div className="mt-1 rounded-lg bg-surface/70 px-3 py-2">
              <p className="text-xs font-medium text-foreground/80">
                {t("diagnostics.caused_by", { count: causedBy[check.id].length })}
              </p>
              <ul className="mt-1 flex flex-col gap-1">
                {causedBy[check.id].map((affected) => (
                  <li key={affected.id} className="flex min-w-0 items-start gap-2 text-xs text-foreground/75">
                    <StatusIcon status={affected.status} />
                    <span className="min-w-0 [overflow-wrap:anywhere]">
                      {t(`diagnostics.check.${affected.id}.title`, checkParams(affected, t, i18n.language))}
                    </span>
                  </li>
                ))}
              </ul>
            </div>
          )}
          {actions.length > 0 && (
            <div className="mt-1 flex flex-wrap gap-2">
              {actions.map((action) => (
                <Button
                  key={action}
                  size="sm"
                  variant="secondary"
                  onPress={() => onAction(action)}
                >
                  {t(`troubleshoot.action.${action}`)}
                </Button>
              ))}
            </div>
          )}
        </div>
      </li>
    );
  };

  return (
    <UnifiedModal
      isOpen={open}
      onOpenChange={(next) => {
        if (!next) close();
      }}
      size="wide"
      type={type}
      title={title}
      isDismissable={!running}
      contentKey={running ? "running" : failed ? "failed" : "done"}
      footer={
        <>
          {running ? (
            <ModalAction variant="secondary" onPress={close}>
              {t("common.cancel")}
            </ModalAction>
          ) : (
            <>
              {report && (
                <ModalAction variant="secondary" onPress={() => void copyReport()}>
                  {t("diagnostics.copy_report")}
                </ModalAction>
              )}
              <ModalAction variant="secondary" onPress={() => setRunId((n) => n + 1)}>
                {t("diagnostics.run_again")}
              </ModalAction>
              <ModalAction variant="primary" onPress={close}>
                {t("common.close")}
              </ModalAction>
            </>
          )}
        </>
      }
    >
      <p className="text-sm text-muted">{scope}</p>
      {running && (
        <ModalProgress
          label={t("diagnostics.running_label")}
          description={t(isMac ? "diagnostics.running_body_mac" : "diagnostics.running_body")}
          detail={t("diagnostics.elapsed", { seconds: elapsed })}
        />
      )}
      {failed && (
        <ModalNotice tone="danger" role="alert">
          {t("diagnostics.failed_body")}
        </ModalNotice>
      )}
      {report && (
        <>
          <div aria-live="polite" className="sr-only">
            {title}
          </div>
          {problems.length === 0 ? (
            <ModalNotice tone="info">
              <span className="block">
                {t(isMac ? "diagnostics.no_problems_body_mac" : "diagnostics.no_problems_body")}
              </span>
              <Button
                size="sm"
                variant="tertiary"
                className="mt-2"
                onPress={() => {
                  if (isMac) void runFix("open_logs");
                  else void Browser.OpenURL(faqIndexUrl(i18n.language));
                }}
              >
                {t(isMac ? "troubleshoot.action.open_logs" : "troubleshoot.open_faq_index")}
              </Button>
            </ModalNotice>
          ) : (
            <section aria-labelledby="diagnostics-problems">
              <h3 id="diagnostics-problems" className="mb-3 text-sm font-semibold text-foreground">
                {t("diagnostics.problems_heading")}
              </h3>
              <ul className="divide-y divide-border/70 rounded-xl border border-border/70 bg-surface-secondary/40 p-4">
                {problems.map((check) => renderCheck(check, true))}
              </ul>
            </section>
          )}
          {notes.length > 0 && (
            <section aria-labelledby="diagnostics-notes">
              <h3 id="diagnostics-notes" className="mb-3 text-sm font-semibold text-foreground">
                {t("diagnostics.notes_heading")}
              </h3>
              <ul className="divide-y divide-border/70 rounded-xl border border-border/70 p-4">
                {notes.map((check) => renderCheck(check, true))}
              </ul>
            </section>
          )}
          {passed.length > 0 && (
            <details className="rounded-xl border border-border/70">
              <summary className="cursor-pointer select-none px-4 py-3 text-sm font-medium text-foreground/80">
                {t(isMac ? "diagnostics.other_checks" : "diagnostics.passed_heading", { count: passed.length })}
              </summary>
              <ul className="divide-y divide-border/70 border-t border-border/70 p-4">
                {passed.map((check) => renderCheck(check, false))}
              </ul>
            </details>
          )}
          <p className="text-xs text-muted">
            {t("diagnostics.duration", {
              seconds: Math.max(1, Math.round((report.durationMs || 0) / 1000)),
            })}
          </p>
        </>
      )}
    </UnifiedModal>
  );
};
