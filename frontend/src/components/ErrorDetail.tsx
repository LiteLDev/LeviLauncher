import { Button, toast } from "@heroui/react";
import { Clipboard } from "@wailsio/runtime";
import { GetLogsDir } from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import React from "react";
import { useTranslation } from "react-i18next";
import { cn } from "@/utils/cn";
import { explainErrorDetail } from "@/utils/errorDetail";
import { openDirectory, showDirectoryOpenError } from "@/utils/explorer";

type ErrorDetailProps = {
  code?: string;
  // The raw detail the backend returned after the code (may span lines).
  detail: string;
  // Offer the logs folder even when there is no detail to show.
  alwaysShowLogs?: boolean;
  className?: string;
};

export const openLogsFolder = async (): Promise<void> => {
  try {
    const dir = String((await GetLogsDir()) || "").trim();
    if (dir) await openDirectory(dir);
  } catch (error) {
    showDirectoryOpenError(error);
  }
};

// ErrorDetail is the part of an error dialog below the one-line message: what
// the failure most likely means in plain words, the technical text exactly as
// the backend reported it (selectable, copyable) and the way to the full log.
export const ErrorDetail: React.FC<ErrorDetailProps> = ({
  code,
  detail,
  alwaysShowLogs,
  className,
}) => {
  const { t } = useTranslation();
  const text = detail.trim();
  const causes = React.useMemo(() => explainErrorDetail(text), [text]);
  if (!text && !alwaysShowLogs) return null;

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
    <div className={cn("flex min-w-0 flex-col gap-3 text-sm", className)}>
      {causes.length > 0 && (
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
        {text && (
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
