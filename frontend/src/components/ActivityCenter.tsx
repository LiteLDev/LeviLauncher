import React, {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  Button,
  Drawer,
  ProgressBar,
} from "@heroui/react";
import { Clipboard } from "@wailsio/runtime";
import { useTranslation } from "react-i18next";
import {
  LuCheck,
  LuChevronDown,
  LuChevronRight,
  LuCircleCheck,
  LuCircleX,
  LuCopy,
  LuInbox,
  LuInfo,
  LuLoaderCircle,
  LuTrash2,
  LuTriangleAlert,
  LuX,
} from "react-icons/lu";
import { cn } from "@/utils/cn";
import {
  clearActivity,
  closeActivityCenter,
  removeActivity,
  useActivityCenterOpen,
  useActivityEntries,
  type ActivityEntry,
} from "@/utils/activity";

// The Activity center keeps every notice and task log. Running tasks are
// pinned on top with their live progress; the rest is grouped by day, newest
// first.

type Filter = "all" | "errors" | "tasks";

const DAY_MS = 24 * 60 * 60 * 1000;

const isRunning = (entry: ActivityEntry) =>
  entry.kind === "task" && entry.status === "running";

const isError = (entry: ActivityEntry) =>
  entry.variant === "danger" ||
  entry.variant === "warning" ||
  entry.status === "error";

const matchesFilter = (entry: ActivityEntry, filter: Filter) =>
  filter === "all"
    ? true
    : filter === "errors"
      ? isError(entry)
      : entry.kind === "task";

const startOfDay = (at: number) => {
  const date = new Date(at);
  date.setHours(0, 0, 0, 0);
  return date.getTime();
};

const formatTime = (at: number, locale?: string) =>
  new Date(at).toLocaleTimeString(locale, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });

const formatDateTime = (at: number) =>
  new Date(at).toLocaleString(undefined, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });

const entryText = (entry: ActivityEntry) =>
  [
    `[${formatDateTime(entry.at)}] ${entry.title}`.trim(),
    entry.description,
    ...(entry.lines ?? []).map(
      (line) => `  ${formatTime(line.at)} ${line.level}: ${line.message}`,
    ),
  ]
    .filter(Boolean)
    .join("\n");

async function copyText(text: string): Promise<boolean> {
  try {
    await Clipboard.SetText(text);
    return true;
  } catch {}
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    return false;
  }
}

// Copy button whose icon confirms the copy; the label says it for screen
// readers.
function CopyButton({
  getText,
  label,
  className,
  isDisabled,
}: {
  getText: () => string;
  label: string;
  className?: string;
  isDisabled?: boolean;
}) {
  const { t } = useTranslation();
  const [state, setState] = useState<"idle" | "copied" | "failed">("idle");
  useEffect(() => {
    if (state === "idle") return;
    const timer = window.setTimeout(() => setState("idle"), 2000);
    return () => window.clearTimeout(timer);
  }, [state]);
  return (
    <Button
      size="sm"
      variant="ghost"
      isIconOnly
      isDisabled={isDisabled}
      className={className}
      aria-label={
        state === "copied"
          ? t("activity.copied")
          : state === "failed"
            ? t("activity.copy_failed")
            : label
      }
      onPress={async () => setState((await copyText(getText())) ? "copied" : "failed")}
    >
      {state === "copied" ? (
        <LuCheck className="text-success" aria-hidden="true" />
      ) : state === "failed" ? (
        <LuX className="text-danger" aria-hidden="true" />
      ) : (
        <LuCopy aria-hidden="true" />
      )}
    </Button>
  );
}

const EntryIcon = ({ entry }: { entry: ActivityEntry }) => {
  const className = "size-[1.125rem]";
  if (isRunning(entry)) {
    return <LuLoaderCircle className={cn(className, "animate-spin text-accent")} />;
  }
  if (entry.status === "cancelled") {
    return <LuCircleX className={cn(className, "text-muted")} />;
  }
  switch (entry.variant) {
    case "success":
      return <LuCircleCheck className={cn(className, "text-success")} />;
    case "warning":
      return <LuTriangleAlert className={cn(className, "text-warning")} />;
    case "danger":
      return <LuCircleX className={cn(className, "text-danger")} />;
    default:
      return <LuInfo className={cn(className, "text-muted")} />;
  }
};

function LogLines({ entry }: { entry: ActivityEntry }) {
  const { i18n } = useTranslation();
  const locale = i18n.language?.replace("_", "-");
  const listRef = useRef<HTMLOListElement | null>(null);
  const followRef = useRef(true);
  const lines = entry.lines ?? [];

  // Follow new lines of a running task unless the user scrolled up to read.
  useEffect(() => {
    const list = listRef.current;
    if (list && followRef.current) list.scrollTop = list.scrollHeight;
    // The last line changes even once the log is capped at its length limit.
  }, [lines[lines.length - 1]]);

  return (
    <ol
      ref={listRef}
      tabIndex={0}
      onScroll={(event) => {
        const el = event.currentTarget;
        followRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 24;
      }}
      className="custom-scrollbar mt-1.5 max-h-[min(40vh,20rem)] overflow-y-auto rounded-lg bg-surface-tertiary/50 px-2.5 py-1.5 font-mono text-xs leading-5 outline-none focus-visible:ring-2 focus-visible:ring-focus"
    >
      {lines.map((line, index) => (
        <li
          key={index}
          className={cn(
            "flex min-w-0 gap-2",
            line.level === "error"
              ? "text-danger"
              : line.level === "warning"
                ? "text-warning"
                : line.level === "success"
                  ? "text-success"
                  : "text-foreground/85",
          )}
        >
          <time className="shrink-0 text-muted" dateTime={new Date(line.at).toISOString()}>
            {formatTime(line.at, locale)}
          </time>
          <span className="min-w-0 whitespace-pre-wrap [overflow-wrap:anywhere]">
            {line.message}
          </span>
        </li>
      ))}
    </ol>
  );
}

const ActivityItem = React.memo(function ActivityItem({
  entry,
  isExpanded,
  onToggle,
}: {
  entry: ActivityEntry;
  isExpanded: boolean;
  onToggle: (id: string, expanded: boolean) => void;
}) {
  const { t, i18n } = useTranslation();
  const locale = i18n.language?.replace("_", "-");
  const logId = useId();
  const lines = entry.lines ?? [];
  const running = isRunning(entry);

  return (
    <li
      className={cn(
        "flex min-w-0 gap-3 rounded-2xl p-3",
        running
          ? "bg-accent/8 ring-1 ring-accent/25"
          : entry.variant === "danger"
            ? "bg-danger/6"
            : "bg-surface-secondary/60",
      )}
    >
      <div className="shrink-0 pt-0.5" aria-hidden="true">
        <EntryIcon entry={entry} />
      </div>
      <div className="min-w-0 flex-1 select-text">
        <div className="flex min-w-0 items-start gap-2">
          <p className="min-w-0 flex-1 text-sm font-medium leading-5 [overflow-wrap:anywhere]">
            {entry.title || entry.description}
          </p>
          <time
            className="shrink-0 pt-px text-xs tabular-nums leading-5 text-muted"
            dateTime={new Date(entry.at).toISOString()}
            title={formatDateTime(entry.at)}
          >
            {formatTime(entry.at, locale)}
          </time>
          <div className="-my-1 -me-1.5 flex shrink-0 items-center">
            <CopyButton
              label={t("activity.copy_entry")}
              className="size-7 min-w-7 text-muted"
              getText={() => entryText(entry)}
            />
            <Button
              size="sm"
              variant="ghost"
              isIconOnly
              className="size-7 min-w-7 text-muted"
              aria-label={t("activity.remove_entry")}
              isDisabled={running}
              onPress={() => removeActivity(entry.id)}
            >
              <LuX aria-hidden="true" />
            </Button>
          </div>
        </div>
        {entry.title && entry.description ? (
          <p
            className={cn(
              "mt-0.5 text-sm leading-5 whitespace-pre-wrap [overflow-wrap:anywhere]",
              entry.status === "error" ? "text-danger" : "text-muted",
            )}
          >
            {entry.description}
          </p>
        ) : null}
        {running ? (
          <ProgressBar
            aria-label={entry.title}
            value={entry.progress}
            isIndeterminate={typeof entry.progress !== "number"}
            size="sm"
            color="accent"
            className="mt-2 w-full"
          >
            <ProgressBar.Track>
              <ProgressBar.Fill />
            </ProgressBar.Track>
          </ProgressBar>
        ) : null}
        {lines.length > 0 ? (
          <Button
            size="sm"
            variant="ghost"
            className="-ms-2 mt-1 h-7 min-h-7 gap-1 px-2 text-xs text-muted"
            aria-expanded={isExpanded}
            aria-controls={logId}
            onPress={() => onToggle(entry.id, !isExpanded)}
          >
            {isExpanded ? (
              <LuChevronDown aria-hidden="true" />
            ) : (
              <LuChevronRight aria-hidden="true" />
            )}
            {t("activity.log_lines", { count: lines.length })}
          </Button>
        ) : null}
        {lines.length > 0 && isExpanded ? (
          <div id={logId}>
            <LogLines entry={entry} />
          </div>
        ) : null}
      </div>
    </li>
  );
});

function Section({
  label,
  entries,
  expanded,
  onToggle,
}: {
  label: string;
  entries: ActivityEntry[];
  expanded: Record<string, boolean>;
  onToggle: (id: string, expanded: boolean) => void;
}) {
  const headingId = useId();
  return (
    <section aria-labelledby={headingId} className="flex min-w-0 flex-col gap-2">
      <h3
        id={headingId}
        className="sticky top-0 z-[1] -mx-1 bg-overlay/95 px-1 py-1 text-xs font-semibold uppercase tracking-wide text-muted backdrop-blur-sm"
      >
        {label}
      </h3>
      <ul className="flex min-w-0 flex-col gap-2">
        {entries.map((entry) => (
          <ActivityItem
            key={entry.id}
            entry={entry}
            isExpanded={expanded[entry.id] ?? isRunning(entry)}
            onToggle={onToggle}
          />
        ))}
      </ul>
    </section>
  );
}

export const ActivityCenter: React.FC = () => {
  const { t, i18n } = useTranslation();
  const isOpen = useActivityCenterOpen();
  const liveEntries = useActivityEntries();
  // While closed, keep the last list (it stays visible during the exit
  // animation) instead of regrouping on every log line of a running task.
  const shownRef = useRef(liveEntries);
  if (isOpen) shownRef.current = liveEntries;
  const entries = shownRef.current;
  const [filter, setFilter] = useState<Filter>("all");
  const [confirmClear, setConfirmClear] = useState(false);
  // Expanded logs by entry. A running task's log opens on its own and stays
  // open when the task finishes, until the user collapses it.
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const onToggle = useCallback(
    (id: string, value: boolean) =>
      setExpanded((prev) => ({ ...prev, [id]: value })),
    [],
  );

  useEffect(() => {
    const pending = entries.filter(
      (entry) => isRunning(entry) && expanded[entry.id] === undefined,
    );
    if (pending.length === 0) return;
    setExpanded((prev) => {
      const next = { ...prev };
      for (const entry of pending) next[entry.id] = true;
      return next;
    });
  }, [entries, expanded]);

  useEffect(() => {
    if (!isOpen) setConfirmClear(false);
  }, [isOpen]);

  const counts = useMemo(
    () => ({
      all: entries.length,
      errors: entries.filter(isError).length,
      tasks: entries.filter((entry) => entry.kind === "task").length,
    }),
    [entries],
  );

  const { running, groups } = useMemo(() => {
    const locale = i18n.language?.replace("_", "-");
    const today = startOfDay(Date.now());
    const visible = entries.filter((entry) => matchesFilter(entry, filter));
    const running = visible.filter(isRunning);
    const groups: Array<{ key: number; label: string; entries: ActivityEntry[] }> = [];
    for (const entry of visible) {
      if (isRunning(entry)) continue;
      const day = startOfDay(entry.at);
      let group = groups[groups.length - 1];
      if (!group || group.key !== day) {
        const label =
          day === today
            ? t("activity.today")
            : day === today - DAY_MS
              ? t("activity.yesterday")
              : new Date(day).toLocaleDateString(locale, {
                  year: day < today - 300 * DAY_MS ? "numeric" : undefined,
                  month: "long",
                  day: "numeric",
                  weekday: "short",
                });
        group = { key: day, label, entries: [] };
        groups.push(group);
      }
      group.entries.push(entry);
    }
    return { running, groups };
  }, [entries, filter, i18n.language, t]);

  const finishedCount = entries.length - entries.filter(isRunning).length;
  const visibleCount =
    running.length + groups.reduce((sum, group) => sum + group.entries.length, 0);

  const filterButton = (key: Filter, label: string) => {
    const selected = filter === key;
    return (
      <Button
        key={key}
        size="sm"
        variant="ghost"
        aria-pressed={selected}
        onPress={() => setFilter(key)}
        className={cn(
          "h-8 min-h-8 min-w-0 flex-1 gap-1.5 rounded-full px-2 text-sm",
          selected
            ? "bg-surface font-semibold text-foreground shadow-sm dark:bg-surface-tertiary"
            : "text-muted",
        )}
      >
        <span className="truncate">{label}</span>
        <span className="text-xs tabular-nums opacity-70">{counts[key]}</span>
      </Button>
    );
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
            className="flex h-full w-full max-w-full flex-col md:w-[min(34rem,100vw)]"
          >
            <Drawer.Header className="flex-col items-stretch gap-3 pb-2">
              <div className="flex min-w-0 items-center justify-between gap-2">
                <Drawer.Heading className="min-w-0 truncate">
                  {t("activity.title")}
                </Drawer.Heading>
                <div className="flex shrink-0 items-center gap-1">
                  <CopyButton
                    label={t("activity.copy_all")}
                    isDisabled={visibleCount === 0}
                    getText={() =>
                      [...running, ...groups.flatMap((group) => group.entries)]
                        .map(entryText)
                        .join("\n\n")
                    }
                  />
                  <Button
                    size="sm"
                    variant="ghost"
                    isIconOnly
                    aria-label={t("activity.clear")}
                    aria-expanded={confirmClear}
                    isDisabled={finishedCount === 0}
                    onPress={() => setConfirmClear((value) => !value)}
                  >
                    <LuTrash2 aria-hidden="true" />
                  </Button>
                  <Drawer.CloseTrigger className="static" />
                </div>
              </div>
              {confirmClear ? (
                <div
                  role="group"
                  aria-label={t("activity.clear")}
                  className="flex min-w-0 flex-wrap items-center gap-2 rounded-xl bg-danger/8 px-3 py-2 text-sm"
                >
                  <span className="min-w-0 flex-1">
                    {t("activity.clear_confirm", { count: finishedCount })}
                  </span>
                  <Button size="sm" variant="ghost" onPress={() => setConfirmClear(false)}>
                    {t("common.cancel")}
                  </Button>
                  <Button
                    size="sm"
                    variant="danger"
                    onPress={() => {
                      clearActivity();
                      setConfirmClear(false);
                    }}
                  >
                    {t("activity.clear")}
                  </Button>
                </div>
              ) : null}
              <div
                role="group"
                aria-label={t("activity.filter_label")}
                className="flex min-w-0 gap-1 rounded-full bg-surface-secondary p-1"
              >
                {filterButton("all", t("activity.filter_all"))}
                {filterButton("errors", t("activity.filter_errors"))}
                {filterButton("tasks", t("activity.filter_tasks"))}
              </div>
            </Drawer.Header>
            <Drawer.Body className="custom-scrollbar min-h-0 flex-1 overflow-y-auto overflow-x-hidden">
              {visibleCount === 0 ? (
                <div className="flex flex-col items-center gap-2 py-12 text-center text-sm text-muted">
                  <LuInbox className="size-8 opacity-60" aria-hidden="true" />
                  <p>
                    {entries.length === 0
                      ? t("activity.empty")
                      : t("activity.empty_filtered")}
                  </p>
                </div>
              ) : (
                <div className="flex min-w-0 flex-col gap-4 pb-2">
                  {running.length > 0 ? (
                    <Section
                      label={t("activity.running")}
                      entries={running}
                      expanded={expanded}
                      onToggle={onToggle}
                    />
                  ) : null}
                  {groups.map((group) => (
                    <Section
                      key={group.key}
                      label={group.label}
                      entries={group.entries}
                      expanded={expanded}
                      onToggle={onToggle}
                    />
                  ))}
                </div>
              )}
            </Drawer.Body>
          </Drawer.Dialog>
        </Drawer.Content>
      </Drawer.Backdrop>
    </Drawer>
  );
};
