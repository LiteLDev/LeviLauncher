import React from "react";
import { Button, Drawer } from "@heroui/react";
import { useTranslation } from "react-i18next";
import {
  FaCheckCircle,
  FaExclamationTriangle,
  FaInfoCircle,
  FaSpinner,
  FaTimesCircle,
  FaRegCopy,
  FaTrash,
} from "react-icons/fa";
import {
  clearActivity,
  closeActivityCenter,
  useActivityCenterOpen,
  useActivityEntries,
  type ActivityEntry,
} from "@/utils/activity";

const formatTime = (at: number) =>
  new Date(at).toLocaleString(undefined, {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });

const EntryIcon = ({ entry }: { entry: ActivityEntry }) => {
  if (entry.kind === "task" && entry.status === "running") {
    return <FaSpinner className="animate-spin text-brand-500" />;
  }
  switch (entry.variant) {
    case "success":
      return <FaCheckCircle className="text-success" />;
    case "warning":
      return <FaExclamationTriangle className="text-warning" />;
    case "danger":
      return <FaTimesCircle className="text-danger" />;
    default:
      return <FaInfoCircle className="text-muted" />;
  }
};

const entryText = (entry: ActivityEntry) =>
  [
    `[${formatTime(entry.at)}] ${entry.title}`.trim(),
    entry.description,
    ...(entry.lines ?? []).map(
      (line) => `  ${formatTime(line.at)} ${line.level}: ${line.message}`,
    ),
  ]
    .filter(Boolean)
    .join("\n");

const ActivityItem = ({ entry }: { entry: ActivityEntry }) => {
  const { t } = useTranslation();
  const lines = entry.lines ?? [];
  return (
    <li className="flex gap-3 rounded-2xl bg-surface-secondary/60 p-3">
      <div className="pt-0.5 shrink-0">
        <EntryIcon entry={entry} />
      </div>
      <div className="min-w-0 flex-1 select-text">
        <div className="flex items-baseline justify-between gap-2">
          <p className="font-medium text-sm truncate">
            {entry.title || entry.description}
          </p>
          <time className="shrink-0 text-xs text-muted">
            {formatTime(entry.at)}
          </time>
        </div>
        {entry.title && entry.description && (
          <p className="text-sm text-muted [overflow-wrap:anywhere]">
            {entry.description}
          </p>
        )}
        {entry.kind === "task" && typeof entry.progress === "number" &&
          entry.status === "running" && (
            <div className="mt-2 h-1.5 rounded-full bg-surface-tertiary overflow-hidden">
              <div
                className="h-full bg-brand-500 transition-[width]"
                style={{ width: `${entry.progress}%` }}
              />
            </div>
          )}
        {lines.length > 0 && (
          <details className="mt-2" open={entry.status === "running"}>
            <summary className="cursor-pointer text-xs text-muted">
              {t("activity.log_lines", { count: lines.length })}
            </summary>
            <ol className="mt-1 max-h-60 overflow-y-auto font-mono text-xs leading-5">
              {lines.map((line, index) => (
                <li
                  key={index}
                  className={
                    line.level === "error"
                      ? "text-danger"
                      : line.level === "warning"
                        ? "text-warning"
                        : line.level === "success"
                          ? "text-success"
                          : ""
                  }
                >
                  {line.message}
                </li>
              ))}
            </ol>
          </details>
        )}
      </div>
    </li>
  );
};

export const ActivityCenter: React.FC = () => {
  const { t } = useTranslation();
  const isOpen = useActivityCenterOpen();
  const entries = useActivityEntries();

  const copyAll = () => {
    try {
      void navigator.clipboard.writeText(entries.map(entryText).join("\n\n"));
    } catch {}
  };

  return (
    <Drawer>
      <Drawer.Backdrop
        isOpen={isOpen}
        onOpenChange={(open) => {
          if (!open) closeActivityCenter();
        }}
        className="z-[110]"
      >
        <Drawer.Content placement="right">
          <Drawer.Dialog
            aria-label={t("activity.title")}
            className="h-full flex flex-col w-[min(28rem,100vw)] max-w-full"
          >
            <Drawer.Header className="flex-row items-center justify-between gap-2 pr-10">
              <Drawer.Heading>{t("activity.title")}</Drawer.Heading>
              <div className="flex gap-1">
                <Button
                  size="sm"
                  variant="ghost"
                  isIconOnly
                  aria-label={t("activity.copy_all")}
                  isDisabled={entries.length === 0}
                  onPress={copyAll}
                >
                  <FaRegCopy />
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  isIconOnly
                  aria-label={t("activity.clear")}
                  isDisabled={entries.length === 0}
                  onPress={clearActivity}
                >
                  <FaTrash />
                </Button>
                <Drawer.CloseTrigger />
              </div>
            </Drawer.Header>
            <Drawer.Body className="min-h-0 flex-1 overflow-y-auto">
              {entries.length === 0 ? (
                <p className="py-10 text-center text-sm text-muted">
                  {t("activity.empty")}
                </p>
              ) : (
                <ul className="flex flex-col gap-2" aria-live="polite">
                  {entries.map((entry) => (
                    <ActivityItem key={entry.id} entry={entry} />
                  ))}
                </ul>
              )}
            </Drawer.Body>
          </Drawer.Dialog>
        </Drawer.Content>
      </Drawer.Backdrop>
    </Drawer>
  );
};
