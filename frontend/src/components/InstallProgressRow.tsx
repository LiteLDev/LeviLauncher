import { ProgressBar } from "@heroui/react";
import { cn } from "@/utils/cn";

// Mirrors LipTaskProgressView's row (label + amount/percent above a small
// bar) so the extract bar and the LeviLamina step look the same in one slot.
export function InstallProgressRow({
  label,
  amount,
  value,
  className,
}: {
  label: string;
  amount?: string;
  value?: number;
  className?: string;
}) {
  return (
    <div className={cn("min-w-0 space-y-1.5", className)}>
      <div className="flex min-w-0 items-baseline justify-between gap-3 text-xs leading-5">
        <span className="min-w-0 truncate font-semibold text-foreground">
          {label}
        </span>
        <div className="flex shrink-0 items-baseline gap-2 tabular-nums text-muted">
          {amount ? <span>{amount}</span> : null}
          {value !== undefined ? (
            <span className="min-w-9 text-right font-medium text-foreground/80 dark:text-zinc-300">
              {`${Math.floor(value)}%`}
            </span>
          ) : null}
        </div>
      </div>
      <ProgressBar
        aria-label={label}
        value={value}
        isIndeterminate={value === undefined}
        size="sm"
        color="accent"
        className="w-full"
      >
        <ProgressBar.Track>
          <ProgressBar.Fill />
        </ProgressBar.Track>
      </ProgressBar>
    </div>
  );
}
