import { Button, toast } from "@heroui/react";
import { Browser, Clipboard } from "@wailsio/runtime";
import { GetLogsDir } from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import React from "react";
import { useTranslation } from "react-i18next";
import { FiActivity, FiExternalLink } from "react-icons/fi";
import { cn } from "@/utils/cn";
import { explainErrorDetail } from "@/utils/errorDetail";
import { openDirectory, showDirectoryOpenError } from "@/utils/explorer";
import {
  faqUrl,
  guideFor,
  type DiagnosticTopic,
  type FixActionId,
  type Guide,
  type GuideId,
  GUIDES,
} from "@/utils/troubleshoot";
import {
  LEAVING_ACTIONS,
  useFixAction,
  useHasDiagnostics,
  isFixActionAvailable,
} from "@/utils/DiagnosticsContext";

type ErrorDetailProps = {
  code?: string;
  // The raw detail the backend returned after the code (may span lines).
  detail: string;
  // Offer the logs folder even when there is no detail to show.
  alwaysShowLogs?: boolean;
  className?: string;
  // What the user was doing; picks the diagnostics and, for failures with
  // no known code, the network guide of that area.
  topic?: DiagnosticTopic;
  // The instance the failure was about, for instance-specific diagnostics.
  instance?: string;
  // "release" or "preview", for license and install failures.
  channel?: string;
  // Use this guide instead of the one the code maps to.
  guide?: GuideId;
  // Called before a step's button leaves the current screen (opens another
  // page or the account menu), so the dialog showing this can close.
  onLeave?: () => void;
  // false shows only the causes found in the detail text (for lists of
  // many failures, where a full guide per item would drown the list).
  showGuide?: boolean;
};

export const openLogsFolder = async (): Promise<void> => {
  try {
    const dir = String((await GetLogsDir()) || "").trim();
    if (dir) await openDirectory(dir);
  } catch (error) {
    showDirectoryOpenError(error);
  }
};

// TroubleshootGuide lists the likely causes of a failure and the numbered
// steps that fix it, each step with a button when the launcher can do or
// open the thing it asks for.
export const TroubleshootGuide: React.FC<{
  guide: Guide;
  // Causes found in the failure's own text; shown first, as most likely.
  matchedCauses?: string[];
  channel?: string;
  instance?: string;
  onLeave?: () => void;
  headingLevel?: "h3" | "h4";
  // The caller already offers "Run diagnostics" next to this guide.
  hideDiagnose?: boolean;
  // The dialog has its own primary action; offer diagnostics less loudly.
  quietDiagnose?: boolean;
}> = ({ guide, matchedCauses = [], channel, instance, onLeave, headingLevel = "h3", hideDiagnose, quietDiagnose }) => {
  const { t, i18n } = useTranslation();
  const hasDiagnostics = useHasDiagnostics();
  const runFix = useFixAction(
    guide.topic ? { topic: guide.topic, instance, channel } : undefined,
  );
  const Heading = headingLevel;
  const channelName = channel ? t(`diagnostics.channel.${channel}`, { defaultValue: channel }) : "";
  const vars = { channel: channelName || t("diagnostics.channel.this") };
  const onAction = (action: FixActionId) => {
    if (LEAVING_ACTIONS.has(action)) onLeave?.();
    void runFix(action);
  };
  const canDiagnose = hasDiagnostics && Boolean(guide.topic);
  // "Run diagnostics" is offered once, above the lists; its step stays in
  // the numbered order as text.
  const steps = guide.steps.filter(({ action }) =>
    action === "diagnose" ? canDiagnose : !action || isFixActionAvailable(action),
  );
  const showBar = (canDiagnose && !hideDiagnose) || Boolean(guide.faq);

  return (
    <div className="flex min-w-0 flex-col gap-3">
      {showBar && (
        <div className="flex flex-wrap items-center gap-2">
          {canDiagnose && !hideDiagnose && (
            <Button size="sm" variant={quietDiagnose ? "secondary" : "primary"} onPress={() => onAction("diagnose")}>
              <FiActivity aria-hidden="true" />
              {t("troubleshoot.action.diagnose")}
            </Button>
          )}
          {guide.faq && (
            <Button
              size="sm"
              variant="ghost"
              className="text-accent"
              onPress={() => void Browser.OpenURL(faqUrl(guide.faq!, i18n.language))}
            >
              {t("troubleshoot.open_faq")}
              <FiExternalLink aria-hidden="true" />
            </Button>
          )}
        </div>
      )}
      <div className="flex flex-col gap-1">
        <Heading className="font-semibold text-foreground">{t("troubleshoot.causes_title")}</Heading>
        <ul className="list-disc space-y-1 pl-5 text-foreground/80">
          {matchedCauses.map((key) => (
            <li key={`m-${key}`}>{t(`errors.explain.${key}`)}</li>
          ))}
          {guide.causes.map((key) => (
            <li key={key}>{t(`troubleshoot.guide.${guide.id}.cause.${key}`, vars)}</li>
          ))}
        </ul>
      </div>
      <div className="flex flex-col gap-1">
        <Heading className="font-semibold text-foreground">{t("troubleshoot.steps_title")}</Heading>
        <ol className="list-decimal space-y-2 pl-5 text-foreground/80 marker:text-muted">
          {steps.map((step) => (
            <li key={step.key} className="pl-1">
              <span>{t(`troubleshoot.guide.${guide.id}.step.${step.key}`, vars)}</span>
              {step.action && step.action !== "diagnose" && (
                <Button
                  size="sm"
                  variant="secondary"
                  className="ml-2 mt-1 inline-flex align-middle"
                  onPress={() => onAction(step.action as FixActionId)}
                >
                  {t(`troubleshoot.action.${step.action}`)}
                </Button>
              )}
            </li>
          ))}
        </ol>
      </div>
    </div>
  );
};

// ErrorDetail is the part of an error dialog below the one-line message: why
// the failure most likely happened and how to fix it (with buttons for the
// fixes and for diagnostics), the technical text exactly as the backend
// reported it (selectable, copyable) and the way to the full log.
export const ErrorDetail: React.FC<ErrorDetailProps> = ({
  code,
  detail,
  alwaysShowLogs,
  className,
  topic,
  instance,
  channel,
  guide: guideId,
  onLeave,
  showGuide = true,
}) => {
  const { t } = useTranslation();
  const text = detail.trim();
  const causes = React.useMemo(() => explainErrorDetail(text), [text]);
  const guide = !showGuide
    ? undefined
    : guideId
      ? GUIDES[guideId]
      : guideFor(code, causes, topic);
  const resolvedGuide: Guide | undefined =
    guide && topic && !guide.topic ? { ...guide, topic } : guide;
  if (!text && !alwaysShowLogs && !resolvedGuide) return null;

  const report = [code, text].filter(Boolean).join("\n");
  const copy = async () => {
    try {
      await Clipboard.SetText(report);
      toast(t("errorDetail.copied"), { variant: "success", timeout: 2000 });
    } catch {
      toast(t("errorDetail.copy_failed"), { variant: "danger" });
    }
  };

  return (
    <div className={cn("flex min-w-0 flex-col gap-4 text-sm", className)}>
      {resolvedGuide ? (
        <TroubleshootGuide
          guide={resolvedGuide}
          matchedCauses={causes}
          channel={channel}
          instance={instance}
          onLeave={onLeave}
        />
      ) : (
        causes.length > 0 && (
          <div className="flex flex-col gap-1">
            <div className="font-semibold text-foreground">
              {t("errorDetail.causes_title")}
            </div>
            <ul className="list-disc space-y-1 pl-5 text-foreground/80">
              {causes.map((key) => (
                <li key={key}>{t(`errors.explain.${key}`)}</li>
              ))}
            </ul>
          </div>
        )
      )}
      {text && (
        <details className="rounded-xl border border-border/70 bg-surface-secondary/50">
          <summary className="cursor-pointer select-none px-3 py-2 font-medium text-foreground/80">
            {t("errorDetail.technical_title")}
          </summary>
          <pre className="custom-scrollbar max-h-56 select-text overflow-auto whitespace-pre-wrap break-words border-t border-border/70 px-3 py-2 font-mono text-xs text-foreground/80">
            {text}
          </pre>
        </details>
      )}
      <div className="flex flex-wrap items-center gap-2">
        {report && (
          <Button size="sm" variant="secondary" onPress={copy}>
            {t("errorDetail.copy")}
          </Button>
        )}
        <Button size="sm" variant="secondary" onPress={openLogsFolder}>
          {t("errorDetail.open_logs")}
        </Button>
        <span className="text-xs text-muted">{t("errorDetail.log_hint")}</span>
      </div>
    </div>
  );
};
