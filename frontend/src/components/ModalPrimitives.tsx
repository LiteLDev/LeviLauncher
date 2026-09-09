import React from "react";
import { Button, ProgressBar, type ButtonProps } from "@heroui/react";
import { FiAlertTriangle, FiInfo } from "react-icons/fi";
import { cn } from "@/utils/cn";

export const MODAL_STYLES = {
  stack: "flex min-w-0 flex-col gap-4",
  description: "text-sm font-normal leading-6 text-foreground/80 dark:text-zinc-300 [overflow-wrap:anywhere]",
  panel: "min-w-0 rounded-xl border border-border/70 bg-surface-secondary/50 p-4 text-sm leading-6",
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
        "h-9 rounded-xl text-sm font-semibold shadow-none",
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
        tone === "info" && "border-border/70 bg-surface-secondary/50 text-foreground/80 dark:text-zinc-300",
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
