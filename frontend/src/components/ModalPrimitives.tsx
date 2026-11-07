import React from "react";
import { Button, ProgressBar, type ButtonProps } from "@heroui/react";
import { FiAlertTriangle, FiCheck, FiCopy, FiInfo } from "react-icons/fi";
import { useTranslation } from "react-i18next";
import { Clipboard } from "@wailsio/runtime";
import { cn } from "@/utils/cn";

export const MODAL_STYLES = {
  stack: "flex min-w-0 flex-col gap-4",
  description: "text-sm font-normal leading-6 text-foreground/80 dark:text-zinc-300 [overflow-wrap:anywhere]",
  panel: "launcher-modal-panel min-w-0 rounded-xl border border-border/70 bg-surface-secondary/50 p-4 text-sm leading-6",
  fieldGrid: "grid min-w-0 grid-cols-1 gap-4 sm:grid-cols-2",
  label: "text-xs font-medium leading-5 text-muted",
  value: "text-sm font-medium leading-6 text-foreground [overflow-wrap:anywhere]",
} as const;

export function ModalAction({ className, isIconOnly, ...props }: Omit<ButtonProps, "className"> & { className?: string }) {
  return (
    <Button
      {...props}
      isIconOnly={isIconOnly}
      className={cn(
        className,
        "h-9 rounded-full text-sm font-semibold shadow-none",
        !isIconOnly && "min-w-22 px-4 whitespace-normal h-auto min-h-9 py-2",
      )}
    />
  );
}

export function ModalDescription({ className, ...props }: React.ComponentProps<"div">) {
  return <div className={cn(MODAL_STYLES.description, className)} {...props} />;
}

export function ModalPanel({ className, ...props }: React.ComponentProps<"div">) {
  return <div className={cn(MODAL_STYLES.panel, className)} {...props} />;
}

export function ModalNotice({ tone = "warning", children, className, ...props }: React.ComponentProps<"div"> & {
  tone?: "info" | "warning" | "danger";
}) {
  const Icon = tone === "info" ? FiInfo : FiAlertTriangle;
  return (
    <div
      {...props}
      className={cn(
        "flex min-w-0 items-start gap-3 rounded-xl border p-4 text-sm font-normal leading-6 [overflow-wrap:anywhere]",
        tone === "info" && "launcher-modal-panel border-border/70 bg-surface-secondary/50 text-foreground/80 dark:text-zinc-300",
        tone === "warning" && "border-amber-200/70 bg-amber-50/70 text-amber-800 dark:border-amber-500/20 dark:bg-amber-500/10 dark:text-amber-200",
        tone === "danger" && "border-rose-200/70 bg-rose-50/70 text-rose-800 dark:border-rose-500/20 dark:bg-rose-500/10 dark:text-rose-200",
        className,
      )}
    >
      <Icon aria-hidden="true" className="mt-1 size-4 shrink-0" />
      <div className="min-w-0 flex-1 whitespace-pre-wrap">{children}</div>
    </div>
  );
}

export function ModalDetails({ items, className }: {
  items: { label: React.ReactNode; value: React.ReactNode; mono?: boolean; fullWidth?: boolean }[];
  className?: string;
}) {
  return (
    <dl className={cn(MODAL_STYLES.panel, MODAL_STYLES.fieldGrid, className)}>
      {items.map((item, index) => (
        <div key={index} className={cn("min-w-0 space-y-1", item.fullWidth && "sm:col-span-2")}>
          <dt className={MODAL_STYLES.label}>{item.label}</dt>
          <dd className={cn(MODAL_STYLES.value, item.mono && "font-mono")}>{item.value}</dd>
        </div>
      ))}
    </dl>
  );
}

export function ModalProgress({ label, description, value, detail, currentItem }: {
  label: string;
  description?: React.ReactNode;
  value?: number;
  detail?: React.ReactNode;
  currentItem?: React.ReactNode;
}) {
  return (
    <div className={MODAL_STYLES.stack}>
      {description && <ModalDescription>{description}</ModalDescription>}
      <div className="space-y-2">
        <ProgressBar
          aria-label={label}
          value={value}
          isIndeterminate={value === undefined}
          size="sm"
          color="accent"
          className="w-full"
        >
          {value !== undefined && <ProgressBar.Output />}
          <ProgressBar.Track><ProgressBar.Fill /></ProgressBar.Track>
        </ProgressBar>
        {detail && <div className="text-xs leading-5 text-muted tabular-nums">{detail}</div>}
      </div>
      {currentItem && <ModalPanel className="font-mono [overflow-wrap:anywhere]">{currentItem}</ModalPanel>}
    </div>
  );
}

/** Copies text through the native clipboard and keeps a "copied" state until `resetKey` changes. */
export function useCopyText(resetKey?: unknown) {
  const [copied, setCopied] = React.useState(false);
  React.useEffect(() => setCopied(false), [resetKey]);
  const copy = React.useCallback(async (text: string) => {
    setCopied(false);
    try {
      await Clipboard.SetText(text);
    } catch {
      try {
        await navigator.clipboard.writeText(text);
      } catch {
        return;
      }
    }
    setCopied(true);
  }, []);
  return { copied, copy };
}

/**
 * Small "Copy" button for copyable lists inside dialogs. `text` may be a
 * getter so large logs are only formatted when the user presses it.
 */
export function ModalCopyButton({ text, resetKey, isDisabled, className }: {
  text: string | (() => string);
  resetKey?: unknown;
  isDisabled?: boolean;
  className?: string;
}) {
  const { t } = useTranslation();
  const { copied, copy } = useCopyText(resetKey ?? text);
  return (
    <Button
      size="sm"
      variant="ghost"
      className={cn("h-7 min-h-7 shrink-0 gap-1.5 px-2 text-xs", className)}
      isDisabled={isDisabled ?? !text}
      onPress={() => void copy(typeof text === "function" ? text() : text)}
    >
      {copied ? <FiCheck aria-hidden="true" /> : <FiCopy aria-hidden="true" />}
      {copied ? t("primitives.copied") : t("primitives.copy")}
    </Button>
  );
}

export type ModalLogLevel = "info" | "success" | "warning" | "error";
export type ModalLogLine = { level?: ModalLogLevel; message: string; at?: number };

const LOG_LEVEL_CLASS: Record<ModalLogLevel, string> = {
  info: "text-foreground/80 dark:text-zinc-300",
  success: "text-brand-700 dark:text-brand-300",
  warning: "text-amber-700 dark:text-amber-300",
  error: "text-rose-700 dark:text-rose-300",
};

export const formatModalLog = (lines: readonly ModalLogLine[]) =>
  lines
    .map((line) => {
      const time = line.at ? `[${new Date(line.at).toLocaleTimeString()}] ` : "";
      const level = line.level && line.level !== "info" ? `${line.level.toUpperCase()}: ` : "";
      return `${time}${level}${line.message}`;
    })
    .join("\n");

/**
 * Append-only step log for long tasks. Lines are only ever appended, never
 * replaced or cleared, and the list follows the newest line unless the user
 * has scrolled up to read earlier ones. `lines` accepts `ActivityLogLine[]`.
 */
export function ModalLog({
  lines,
  title,
  emptyText,
  className,
  maxHeightClassName = "max-h-[clamp(8rem,32vh,22rem)]",
}: {
  lines: readonly ModalLogLine[];
  title?: React.ReactNode;
  emptyText?: React.ReactNode;
  className?: string;
  maxHeightClassName?: string;
}) {
  const { t } = useTranslation();
  const listRef = React.useRef<HTMLOListElement>(null);
  const followRef = React.useRef(true);

  React.useLayoutEffect(() => {
    const list = listRef.current;
    if (list && followRef.current) list.scrollTop = list.scrollHeight;
  }, [lines]);

  return (
    <section className={cn("launcher-modal-panel flex min-w-0 flex-col rounded-xl border border-border/70 bg-surface-secondary/50", className)}>
      <div className="flex min-w-0 items-center justify-between gap-2 border-b border-border/60 py-1 pl-4 pr-1.5">
        <span className={MODAL_STYLES.label}>
          {title ?? t("primitives.log_title")} ({lines.length})
        </span>
        <ModalCopyButton
          text={() => formatModalLog(lines)}
          resetKey={lines}
          isDisabled={lines.length === 0}
        />
      </div>
      <ol
        ref={listRef}
        role="log"
        aria-live="polite"
        tabIndex={0}
        onScroll={(event) => {
          const el = event.currentTarget;
          followRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 24;
        }}
        className={cn(
          "select-text min-h-0 overflow-y-auto overscroll-contain px-4 py-2 font-mono text-xs leading-5 custom-scrollbar",
          maxHeightClassName,
        )}
      >
        {lines.length === 0 ? (
          <li className="text-muted">{emptyText ?? t("primitives.log_empty")}</li>
        ) : (
          lines.map((line, index) => (
            <li key={index} className={cn("whitespace-pre-wrap [overflow-wrap:anywhere]", LOG_LEVEL_CLASS[line.level ?? "info"])}>
              {line.message}
            </li>
          ))
        )}
      </ol>
    </section>
  );
}
