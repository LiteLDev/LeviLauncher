import React from "react";
import { ProgressBar } from "@heroui/react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { cn } from "@/utils/cn";
import { formatBytes } from "@/utils/formatting";
import type {
  LipProgressBar,
  LipTaskProgressSnapshot,
} from "@/utils/lipTaskProgress";

type BarColor = "accent" | "success" | "danger";

const stageLabel = (bar: LipProgressBar, t: TFunction): string => {
  switch (bar.stage) {
    case "task":
      return t("lip.task_progress.stage_task");
    case "resolve":
      return bar.total > 0
        ? t("lip.task_progress.stage_resolve")
        : t("lip.task_progress.stage_solve");
    case "git_fetch":
      return t("lip.task_progress.stage_git_fetch");
    case "download":
      return t("lip.task_progress.stage_download");
    case "extract":
      return t("lip.task_progress.stage_extract");
    case "place":
      return t("lip.task_progress.stage_place");
    case "scripts":
      return t("lip.task_progress.stage_scripts");
    case "remove":
      return t("lip.task_progress.stage_remove");
    case "cache_clean":
      return t("lip.task_progress.stage_cache_clean");
    default:
      return bar.message || t("lip.task_progress.stage_generic");
  }
};

const unitLabel = (bar: LipProgressBar, t: TFunction): string => {
  // While solving, the resolve bar counts the manifests read so far.
  if (bar.stage === "resolve" && bar.total <= 0) {
    return t("lip.task_progress.unit_manifests");
  }
  switch (bar.unit) {
    case "files":
      return t("lip.task_progress.unit_files");
    case "objects":
      return t("lip.task_progress.unit_objects");
    case "blobs":
      return t("lip.task_progress.unit_blobs");
    case "packages":
      return t("lip.task_progress.unit_packages");
    case "commands":
      return t("lip.task_progress.unit_commands");
    default:
      return "";
  }
};

const amountText = (bar: LipProgressBar, t: TFunction): string => {
  if (bar.stage === "") return "";
  if (bar.unit === "bytes") {
    return bar.total > 0
      ? `${formatBytes(bar.current)} / ${formatBytes(bar.total)}`
      : formatBytes(bar.current);
  }
  const unit = unitLabel(bar, t);
  if (!unit) return "";
  return bar.total > 0
    ? t("lip.task_progress.amount", {
        unit,
        current: bar.current,
        total: bar.total,
      })
    : t("lip.task_progress.amount_unknown", { unit, current: bar.current });
};

const percentText = (value: number | undefined): string =>
  value === undefined ? "" : `${Math.floor(value)}%`;

function ProgressRow({
  label,
  detail,
  amount,
  value,
  item,
  color,
  emphasis = false,
}: {
  label: string;
  detail?: string;
  amount?: string;
  value: number | undefined;
  item?: string;
  color: BarColor;
  emphasis?: boolean;
}) {
  return (
    <div className="min-w-0 space-y-1.5">
      <div className="flex min-w-0 items-baseline justify-between gap-3 text-xs leading-5">
        <div className="flex min-w-0 items-baseline gap-1.5">
          <span
            className={cn(
              "shrink-0",
              emphasis
                ? "font-semibold text-foreground"
                : "font-medium text-foreground/80 dark:text-zinc-300",
            )}
          >
            {label}
          </span>
          {detail ? (
            <span className="min-w-0 truncate font-mono text-muted" title={detail}>
              {detail}
            </span>
          ) : null}
        </div>
        {value !== undefined ? (
          <span className="min-w-9 shrink-0 text-right font-medium tabular-nums text-foreground/80 dark:text-zinc-300">
            {percentText(value)}
          </span>
        ) : null}
      </div>
      <ProgressBar
        aria-label={detail ? `${label} ${detail}` : label}
        value={value}
        isIndeterminate={value === undefined}
        size="sm"
        color={color}
        className="w-full"
      >
        <ProgressBar.Track>
          <ProgressBar.Fill />
        </ProgressBar.Track>
      </ProgressBar>
      {item || amount ? (
        <div className="flex min-w-0 items-baseline justify-between gap-3 text-[11px] leading-4 text-muted">
          <span className="min-w-0 truncate font-mono" title={item || undefined}>
            {item}
          </span>
          {amount ? (
            <span className="shrink-0 tabular-nums">{amount}</span>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

export function LipTaskProgressView({
  snapshot,
  className,
}: {
  snapshot: LipTaskProgressSnapshot;
  className?: string;
}) {
  const { t } = useTranslation();
  const { status, primary, secondary, outer, legacy } = snapshot;
  const color: BarColor =
    status === "failed" ? "danger" : status === "success" ? "success" : "accent";

  let primaryRow: React.ReactNode;
  if (primary) {
    primaryRow = (
      <ProgressRow
        emphasis
        label={stageLabel(primary, t)}
        detail={primary.stage === "task" ? primary.item : ""}
        amount={amountText(primary, t)}
        value={primary.percentage}
        item={primary.stage === "resolve" ? primary.item : ""}
        color={color}
      />
    );
  } else if (legacy) {
    primaryRow = (
      <ProgressRow
        emphasis
        label={t("lip.task_console.progress_label")}
        detail={legacy.message}
        value={legacy.percentage}
        color={color}
      />
    );
  } else if (secondary.length > 0 && status === "running") {
    // A task without a plan (e.g. the cache clean) only has step bars.
    primaryRow = null;
  } else if (outer && outer.total > 0) {
    // A batch item may run no lip task at all (e.g. a plain folder delete):
    // the measured item counter is all there is to show.
    primaryRow = null;
  } else {
    // Nothing measured yet: the task has not reported a bar.
    primaryRow = (
      <ProgressRow
        emphasis
        label={t("lip.task_console.waiting_start")}
        value={status === "running" ? undefined : 0}
        color={color}
      />
    );
  }

  return (
    <div className={cn("flex min-w-0 flex-col gap-3", className)}>
      {outer && outer.total > 0 ? (
        <ProgressRow
          emphasis
          label={t("lip.task_progress.outer", {
            current: Math.min(outer.current, outer.total),
            total: outer.total,
          })}
          detail={outer.label}
          value={(Math.min(outer.completed, outer.total) / outer.total) * 100}
          color={color}
        />
      ) : null}
      {primaryRow}
      {status === "running" && secondary.length > 0 ? (
        // Step bars keep the full width so every track shares the same edges;
        // the non-emphasised label is what marks them as sub-steps.
        <div className="flex min-w-0 flex-col gap-3">
          {secondary.map((bar) => (
            <ProgressRow
              key={bar.key}
              label={stageLabel(bar, t)}
              detail={bar.packageName}
              amount={amountText(bar, t)}
              value={bar.percentage}
              item={bar.item}
              color={color}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}
