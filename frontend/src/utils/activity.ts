import { toast } from "@heroui/react";
import type { ButtonProps } from "@heroui/react";
import type { ReactNode } from "react";
import { useSyncExternalStore } from "react";

// Activity history: every notice and long-running task is appended here so
// nothing the user is shown disappears for good when a toast times out.

export type ActivityVariant =
  | "default"
  | "accent"
  | "info"
  | "success"
  | "warning"
  | "danger";

export type ActivityLogLevel = "info" | "success" | "warning" | "error";

export type ActivityLogLine = {
  at: number;
  level: ActivityLogLevel;
  message: string;
};

export type ActivityTaskStatus = "running" | "success" | "error" | "cancelled";

export type ActivityEntry = {
  id: string;
  at: number;
  updatedAt: number;
  kind: "notice" | "task";
  variant: ActivityVariant;
  title: string;
  description: string;
  source: string;
  read: boolean;
  status?: ActivityTaskStatus;
  progress?: number;
  lines?: ActivityLogLine[];
};

export type NotifyOptions = {
  title?: ReactNode;
  description?: ReactNode;
  variant?: ActivityVariant;
  /** Feature that raised the notice, e.g. "mods" or "downloads". */
  source?: string;
  /** Toast display time; clamped to at least MIN_TOAST_MS. Danger stays until closed. */
  timeout?: number;
  actionProps?: ButtonProps;
  /** Record in history without showing a toast. */
  silent?: boolean;
};

export type StartTaskOptions = {
  title: string;
  description?: string;
  source?: string;
};

const STORAGE_KEY = "ll.activity.v1";
const MAX_ENTRIES = 300;
const MAX_TASK_LINES = 500;
export const MIN_TOAST_MS = 6000;

let entries: ActivityEntry[] = load();
let centerOpen = false;
const listeners = new Set<() => void>();
let persistTimer: ReturnType<typeof setTimeout> | undefined;
let seq = 0;

function load(): ActivityEntry[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    const parsed = raw ? JSON.parse(raw) : [];
    if (!Array.isArray(parsed)) return [];
    // A task cannot still be running after a restart.
    return parsed.slice(0, MAX_ENTRIES).map((entry: ActivityEntry) =>
      entry.kind === "task" && entry.status === "running"
        ? { ...entry, status: "cancelled" as const }
        : entry,
    );
  } catch {
    return [];
  }
}

function persist() {
  if (persistTimer) clearTimeout(persistTimer);
  persistTimer = setTimeout(() => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(entries));
    } catch {}
  }, 300);
}

function emit() {
  persist();
  listeners.forEach((listener) => listener());
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function nextId() {
  seq += 1;
  return `${Date.now().toString(36)}-${seq.toString(36)}`;
}

const textOf = (node: ReactNode): string => {
  if (node === null || node === undefined || typeof node === "boolean") {
    return "";
  }
  if (typeof node === "string" || typeof node === "number") {
    return String(node);
  }
  if (Array.isArray(node)) return node.map(textOf).join("");
  return "";
};

const toastVariant = (variant: ActivityVariant) =>
  variant === "info" ? "accent" : variant;

function prepend(entry: ActivityEntry) {
  entries = [entry, ...entries].slice(0, MAX_ENTRIES);
  emit();
}

function update(id: string, patch: (entry: ActivityEntry) => ActivityEntry) {
  let changed = false;
  entries = entries.map((entry) => {
    if (entry.id !== id) return entry;
    changed = true;
    return patch(entry);
  });
  if (changed) emit();
}

/**
 * Show a notice and keep it in the activity history. Use this instead of a
 * bare `toast()` so short-lived messages can be read again later.
 */
export function notify(options: NotifyOptions): string {
  const variant = options.variant ?? "default";
  const title = textOf(options.title);
  const description = textOf(options.description);
  const id = nextId();
  const now = Date.now();
  prepend({
    id,
    at: now,
    updatedAt: now,
    kind: "notice",
    variant,
    title,
    description,
    source: options.source ?? "",
    read: centerOpen,
  });
  if (!options.silent) {
    toast(options.title ?? options.description ?? "", {
      description: options.title ? options.description : undefined,
      variant: toastVariant(variant),
      actionProps: options.actionProps,
      timeout:
        variant === "danger"
          ? 0
          : Math.max(options.timeout ?? MIN_TOAST_MS, MIN_TOAST_MS),
    });
  }
  return id;
}

/** Start a long-running task whose log lines are appended, never replaced. */
export function startActivityTask(options: StartTaskOptions): string {
  const id = nextId();
  const now = Date.now();
  prepend({
    id,
    at: now,
    updatedAt: now,
    kind: "task",
    variant: "accent",
    title: options.title,
    description: options.description ?? "",
    source: options.source ?? "",
    read: true,
    status: "running",
    lines: [],
  });
  return id;
}

export function appendTaskLog(
  id: string,
  message: string,
  level: ActivityLogLevel = "info",
): void {
  const now = Date.now();
  update(id, (entry) => {
    const lines = entry.lines ?? [];
    const last = lines[lines.length - 1];
    // Skip exact repeats so frequent progress events don't flood the log.
    if (last && last.message === message && last.level === level) {
      return entry;
    }
    return {
      ...entry,
      updatedAt: now,
      lines: [...lines, { at: now, level, message }].slice(-MAX_TASK_LINES),
    };
  });
}

export function setTaskProgress(id: string, progress: number): void {
  update(id, (entry) => ({
    ...entry,
    progress: Math.max(0, Math.min(100, progress)),
  }));
}

export function finishActivityTask(
  id: string,
  status: Exclude<ActivityTaskStatus, "running">,
  summary?: string,
  /** Keep the result in history without counting it as unread. */
  options?: { quiet?: boolean },
): void {
  const now = Date.now();
  update(id, (entry) => ({
    ...entry,
    status,
    updatedAt: now,
    variant:
      status === "success"
        ? "success"
        : status === "error"
          ? "danger"
          : "default",
    description: summary ?? entry.description,
    progress: status === "success" ? 100 : entry.progress,
    read: centerOpen || status === "success" || options?.quiet === true,
  }));
}

export function clearActivity(): void {
  entries = entries.filter(
    (entry) => entry.kind === "task" && entry.status === "running",
  );
  emit();
}

export function removeActivity(id: string): void {
  entries = entries.filter((entry) => entry.id !== id);
  emit();
}

export function markAllActivityRead(): void {
  if (!entries.some((entry) => !entry.read)) return;
  entries = entries.map((entry) => (entry.read ? entry : { ...entry, read: true }));
  emit();
}

export function getActivityEntries(): ActivityEntry[] {
  return entries;
}

export function openActivityCenter(): void {
  centerOpen = true;
  markAllActivityRead();
  listeners.forEach((listener) => listener());
}

export function closeActivityCenter(): void {
  centerOpen = false;
  listeners.forEach((listener) => listener());
}

export function useActivityEntries(): ActivityEntry[] {
  return useSyncExternalStore(subscribe, () => entries);
}

export function useActivityTask(id: string | null | undefined) {
  return useSyncExternalStore(subscribe, () =>
    id ? entries.find((entry) => entry.id === id) : undefined,
  );
}

export function useUnreadActivityCount(): number {
  return useSyncExternalStore(
    subscribe,
    () => entries.filter((entry) => !entry.read).length,
  );
}

export function useActivityCenterOpen(): boolean {
  return useSyncExternalStore(subscribe, () => centerOpen);
}
