import React from "react";
import { Button } from "@heroui/react";
import { useTranslation } from "react-i18next";
import { FiActivity } from "react-icons/fi";
import { TroubleshootGuide } from "@/components/ErrorDetail";
import { explainErrorDetail, splitErrorResult } from "@/utils/errorDetail";
import {
  GUIDES,
  guideFor,
  type DiagnosticTopic,
  type GuideId,
} from "@/utils/troubleshoot";
import { useFixAction, useHasDiagnostics } from "@/utils/DiagnosticsContext";

// FailureHelp is the compact troubleshooting block for places with little
// room (a task console, a list row): a "Run diagnostics" button and the
// guide's causes and fixes behind a disclosure.
export const FailureHelp: React.FC<{
  error: string;
  topic: DiagnosticTopic;
  instance?: string;
  channel?: string;
  // Guide to use when the error text matches none.
  fallback?: GuideId;
  onLeave?: () => void;
  className?: string;
}> = ({ error, topic, instance, channel, fallback, onLeave, className }) => {
  const { t } = useTranslation();
  const hasDiagnostics = useHasDiagnostics();
  const { code, detail } = splitErrorResult(error);
  const causes = React.useMemo(() => explainErrorDetail(detail || error), [detail, error]);
  const found = guideFor(code, causes, topic) ?? (fallback ? GUIDES[fallback] : undefined);
  const guide = found && !found.topic ? { ...found, topic } : found;
  const runFix = useFixAction({ topic: guide?.topic ?? topic, instance, channel });
  if (!guide && !hasDiagnostics) return null;
  return (
    <div className={className}>
      {hasDiagnostics && (
        <Button
          size="sm"
          variant="primary"
          onPress={() => void runFix("diagnose")}
        >
          <FiActivity aria-hidden="true" />
          {t("troubleshoot.action.diagnose")}
        </Button>
      )}
      {guide && (
        <details className="mt-2 rounded-xl border border-border/70 text-sm">
          <summary className="cursor-pointer select-none px-3 py-2 font-medium text-foreground/80">
            {t("troubleshoot.how_to_fix")}
          </summary>
          <div className="border-t border-border/70 px-3 py-3">
            <TroubleshootGuide
              guide={guide}
              matchedCauses={causes}
              instance={instance}
              channel={channel}
              onLeave={onLeave}
              headingLevel="h4"
              hideDiagnose={hasDiagnostics}
            />
          </div>
        </details>
      )}
    </div>
  );
};
