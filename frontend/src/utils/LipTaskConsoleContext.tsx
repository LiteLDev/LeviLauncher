import { ModalAction, ModalDescription, ModalNotice } from "@/components/ModalPrimitives";
import { LipTaskProgressView } from "@/components/LipTaskProgressView";
import { cn } from "@/utils/cn";
import { Button, Chip, toast } from "@heroui/react";

import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import { Clipboard, Events } from "@wailsio/runtime";
import { useTranslation } from "react-i18next";
import { getIntlLocale } from "@/utils/intlLocale";
import { LuChevronDown, LuChevronRight, LuTerminal } from "react-icons/lu";
import { UnifiedModal } from "@/components/UnifiedModal";
import {
  EMPTY_LIP_PROGRESS_BARS,
  applyLipProgressEvent,
  beginLipTaskBars,
  clampPercentage,
  isLegacyLipProgress,
  selectLipPrimaryBar,
  selectLipSecondaryBars,
  type LipLegacyProgress,
  type LipOuterProgress,
  type LipProgressBarsState,
  type LipProgressEventData,
  type LipTaskProgressSnapshot,
} from "@/utils/lipTaskProgress";

export type LipTaskAction =
  | "install"
  | "upgrade"
  | "downgrade"
  | "reinstall"
  | "update"
  | "uninstall"
  | "generic";
type LipTaskStatus = "idle" | "running" | "success" | "failed";
type LipTaskLogLevel = "info" | "success" | "warning" | "error";

type LipTaskLogItem = {
  id: string;
  timestamp: number;
  level: LipTaskLogLevel;
  message: string;
};

// How a session is shown while it runs:
// - "progress" (default): a compact progress dialog, closed on success;
// - "inline": nothing while running (the caller renders <LipTaskProgress/>);
// - "always": the full console, closed shortly after success;
// - "on_error": nothing while running;
// - "never": nothing, not even on failure.
// Except for "never", a failure opens the console unless errorConsole is false.
export type LipTaskFeedbackMode =
  | "progress"
  | "inline"
  | "always"
  | "on_error"
  | "never";

type RunWithLipTaskOptions = {
  action?: LipTaskAction;
  // Instance name the tasks run in; "" matches tasks of any target (e.g. the
  // cache clean, whose target is empty).
  target: string;
  methods?: string[];
  feedbackMode?: LipTaskFeedbackMode;
  errorConsole?: boolean;
};

type RunWithLipTaskTools = {
  addLog: (level: LipTaskLogLevel, message: string) => void;
  setProgress: (percentage: number, message?: string) => void;
  // Outer count of a caller running one task per item: completed items
  // finished out of total; label names the item being worked on.
  setOuter: (completed: number, total: number, label?: string) => void;
};

type LipTaskConsoleContextValue = {
  runWithLipTask: <T>(
    options: RunWithLipTaskOptions,
    runner: (tools: RunWithLipTaskTools) => Promise<T>,
  ) => Promise<T>;
};

type RuntimeEventEnvelope<T> = {
  data?: T;
};

type LipTaskStartedEvent = {
  taskId?: string;
  method?: string;
  target?: string;
  packages?: string[];
  timestamp?: number;
};

type LipTaskLogEvent = {
  taskId?: string;
  method?: string;
  target?: string;
  packages?: string[];
  timestamp?: number;
  level?: string;
  message?: string;
  raw?: string;
};

type LipTaskProgressEvent = LipProgressEventData & {
  method?: string;
  target?: string;
  packages?: string[];
  timestamp?: number;
};

type LipTaskFinishedEvent = {
  taskId?: string;
  method?: string;
  target?: string;
  packages?: string[];
  timestamp?: number;
  success?: boolean;
  error?: string;
  stderr?: string;
  durationMs?: number;
};

type SessionMeta = {
  id: number;
  startedAt: number;
  targetNorm: string;
  methodNormSet: Set<string>;
  trackedTaskIDs: Set<string>;
  // Tasks of a batch item that has ended (setOuter moved on): their late
  // progress updates must not be shown under the next item.
  retiredTaskIDs: Set<string>;
  feedbackMode: LipTaskFeedbackMode;
  errorConsole: boolean;
};

type PendingFailure = {
  message: string;
};

const MAX_LOG_COUNT = 800;
const SUCCESS_AUTO_CLOSE_MS = 700;
const DEFAULT_FEEDBACK_MODE: LipTaskFeedbackMode = "progress";

const LipTaskConsoleContext = createContext<LipTaskConsoleContextValue>({
  runWithLipTask: async <T,>(
    _options: RunWithLipTaskOptions,
    runner: (tools: RunWithLipTaskTools) => Promise<T>,
  ) =>
    runner({
      addLog: () => {},
      setProgress: () => {},
      setOuter: () => {},
    }),
});

const IDLE_PROGRESS_SNAPSHOT: LipTaskProgressSnapshot = {
  status: "idle",
  target: "",
  primary: null,
  secondary: [],
  outer: null,
  legacy: null,
};

const LipTaskProgressContext = createContext<LipTaskProgressSnapshot>(
  IDLE_PROGRESS_SNAPSHOT,
);

const normalizeString = (value: unknown): string => String(value || "").trim();

const normalizeTarget = (value: unknown): string =>
  normalizeString(value).toLowerCase();

const normalizeMethod = (value: unknown): string =>
  normalizeString(value).toLowerCase();

const normalizeLogLevel = (value: unknown): LipTaskLogLevel => {
  const normalized = normalizeString(value).toLowerCase();
  if (normalized === "success") return "success";
  if (normalized === "warning") return "warning";
  if (normalized === "error") return "error";
  return "info";
};

const getEventData = <T,>(event: unknown): T | null => {
  const envelope = event as RuntimeEventEnvelope<T>;
  if (!envelope || typeof envelope !== "object") return null;
  return (envelope.data || null) as T | null;
};

const parseErrorText = (error: unknown): string => {
  if (typeof error === "string") return normalizeString(error);
  if (error instanceof Error) return normalizeString(error.message);
  return normalizeString(error);
};

export const LipTaskConsoleProvider: React.FC<{
  children: React.ReactNode;
}> = ({ children }) => {
  const { t } = useTranslation();

  const [open, setOpen] = useState<boolean>(false);
  const [running, setRunning] = useState<boolean>(false);
  const [status, setStatus] = useState<LipTaskStatus>("idle");
  const [action, setAction] = useState<LipTaskAction>("generic");
  const [target, setTarget] = useState<string>("");
  const [logs, setLogs] = useState<LipTaskLogItem[]>([]);
  const [errorText, setErrorText] = useState<string>("");
  const [feedbackMode, setFeedbackMode] =
    useState<LipTaskFeedbackMode>(DEFAULT_FEEDBACK_MODE);
  const [logExpanded, setLogExpanded] = useState<boolean>(false);
  const [legacyProgress, setLegacyProgress] =
    useState<LipLegacyProgress | null>(null);
  const [bars, setBars] = useState<LipProgressBarsState>(
    EMPTY_LIP_PROGRESS_BARS,
  );
  const [outer, setOuter] = useState<LipOuterProgress | null>(null);

  const sessionRef = useRef<SessionMeta | null>(null);
  const sessionSeqRef = useRef<number>(0);
  const closeTimerRef = useRef<number | null>(null);
  const logSeqRef = useRef<number>(0);
  const logContainerRef = useRef<HTMLDivElement | null>(null);
  const runningRef = useRef<boolean>(false);
  const pendingFailureRef = useRef<PendingFailure | null>(null);

  const clearCloseTimer = useCallback(() => {
    if (closeTimerRef.current !== null) {
      window.clearTimeout(closeTimerRef.current);
      closeTimerRef.current = null;
    }
  }, []);

  const appendLog = useCallback((level: LipTaskLogLevel, message: string) => {
    const normalizedMessage = normalizeString(message);
    if (!normalizedMessage) return;
    const logItem: LipTaskLogItem = {
      id: `${Date.now()}-${++logSeqRef.current}`,
      timestamp: Date.now(),
      level,
      message: normalizedMessage,
    };
    setLogs((prev) => {
      const next = [...prev, logItem];
      if (next.length <= MAX_LOG_COUNT) return next;
      return next.slice(next.length - MAX_LOG_COUNT);
    });
  }, []);

  const setProgressValue = useCallback(
    (percentage: number, message?: string) => {
      setLegacyProgress({
        percentage: clampPercentage(percentage),
        message: normalizeString(message),
      });
    },
    [],
  );

  const setOuterValue = useCallback(
    (completed: number, total: number, label?: string) => {
      const safeTotal = Math.max(0, Math.floor(Number(total) || 0));
      if (safeTotal <= 0) {
        setOuter(null);
        return;
      }
      const safeCompleted = Math.max(
        0,
        Math.min(safeTotal, Math.floor(Number(completed) || 0)),
      );
      const session = sessionRef.current;
      if (session) {
        // The previous item's tasks are over: ignore their late updates
        // (Wails events can arrive after the call that ran them returned).
        for (const id of session.trackedTaskIDs) session.retiredTaskIDs.add(id);
      }
      if (safeCompleted < safeTotal) {
        // A new item starts: the bars of the previous item's tasks no longer
        // describe what is running.
        setBars(EMPTY_LIP_PROGRESS_BARS);
        setLegacyProgress(null);
      }
      setOuter({
        completed: safeCompleted,
        current: Math.min(safeTotal, safeCompleted + 1),
        total: safeTotal,
        label: normalizeString(label),
      });
    },
    [],
  );

  const resetProgressState = useCallback(() => {
    setLegacyProgress(null);
    setBars(EMPTY_LIP_PROGRESS_BARS);
    setOuter(null);
  }, []);

  const resetSessionState = useCallback(() => {
    sessionRef.current = null;
    runningRef.current = false;
    pendingFailureRef.current = null;
    setRunning(false);
    setStatus("idle");
    setAction("generic");
    setTarget("");
    setLogs([]);
    setErrorText("");
    setLogExpanded(false);
    resetProgressState();
  }, [resetProgressState]);

  const closeConsole = useCallback(() => {
    if (running) return;
    clearCloseTimer();
    setOpen(false);
    resetSessionState();
  }, [clearCloseTimer, resetSessionState, running]);

  const setFailedState = useCallback(
    (message: string) => {
      clearCloseTimer();
      runningRef.current = false;
      setRunning(false);
      setStatus("failed");
      if (normalizeString(message)) {
        setErrorText(normalizeString(message));
      }
      setOpen(true);
    },
    [clearCloseTimer],
  );

  const completeSuccess = useCallback(() => {
    clearCloseTimer();
    runningRef.current = false;
    setRunning(false);
    setStatus("success");
    closeTimerRef.current = window.setTimeout(() => {
      setOpen(false);
      resetSessionState();
    }, SUCCESS_AUTO_CLOSE_MS);
  }, [clearCloseTimer, resetSessionState]);

  const matchSessionBase = useCallback(
    (
      targetValue: string,
      methodValue: string,
      timestampValue: number,
    ): boolean => {
      const session = sessionRef.current;
      if (!session) return false;

      if (timestampValue > 0 && timestampValue + 50 < session.startedAt) {
        return false;
      }

      const targetNorm = normalizeTarget(targetValue);
      if (session.targetNorm && targetNorm !== session.targetNorm) {
        return false;
      }

      const methodNorm = normalizeMethod(methodValue);
      if (
        session.methodNormSet.size > 0 &&
        !session.methodNormSet.has(methodNorm)
      ) {
        return false;
      }
      return true;
    },
    [],
  );

  const isTrackedEvent = useCallback(
    (
      taskID: string,
      targetValue: string,
      methodValue: string,
      timestampValue: number,
    ): boolean => {
      const session = sessionRef.current;
      if (!session) return false;

      if (taskID && session.trackedTaskIDs.has(taskID)) {
        return true;
      }

      if (!matchSessionBase(targetValue, methodValue, timestampValue)) {
        return false;
      }

      if (taskID) {
        session.trackedTaskIDs.add(taskID);
      }
      return true;
    },
    [matchSessionBase],
  );

  const runWithLipTask = useCallback(
    async <T,>(
      options: RunWithLipTaskOptions,
      runner: (tools: RunWithLipTaskTools) => Promise<T>,
    ): Promise<T> => {
      clearCloseTimer();
      const startedAt = Date.now();
      const targetName = normalizeString(options.target);
      const methodNormSet = new Set(
        (options.methods || [])
          .map((method) => normalizeMethod(method))
          .filter(Boolean),
      );

      const feedbackMode = options.feedbackMode || DEFAULT_FEEDBACK_MODE;
      const errorConsole =
        feedbackMode !== "never" && options.errorConsole !== false;
      const sessionId = ++sessionSeqRef.current;
      // Another runWithLipTask call can start while this one runs (e.g. the
      // non-blocking cache clean); only the latest session owns the state.
      const isCurrentSession = () => sessionRef.current?.id === sessionId;
      sessionRef.current = {
        id: sessionId,
        startedAt,
        targetNorm: normalizeTarget(targetName),
        methodNormSet,
        trackedTaskIDs: new Set(),
        retiredTaskIDs: new Set(),
        feedbackMode,
        errorConsole,
      };

      pendingFailureRef.current = null;
      setFeedbackMode(feedbackMode);
      setLogExpanded(false);
      setOpen(feedbackMode === "always" || feedbackMode === "progress");
      runningRef.current = true;
      setRunning(true);
      setStatus("running");
      setAction(options.action || "generic");
      setTarget(targetName);
      setLogs([
        {
          id: `${Date.now()}-${++logSeqRef.current}`,
          timestamp: Date.now(),
          level: "info",
          message: t("lip.task_console.waiting_start"),
        },
      ]);
      setErrorText("");
      resetProgressState();

      const tools: RunWithLipTaskTools = {
        addLog: (level, message) => {
          if (isCurrentSession()) appendLog(level, message);
        },
        setProgress: (percentage, message) => {
          if (isCurrentSession()) {
            setProgressValue(Number(percentage || 0), message);
          }
        },
        setOuter: (completed, total, label) => {
          if (isCurrentSession()) setOuterValue(completed, total, label);
        },
      };

      try {
        const result = await runner(tools);
        if (!isCurrentSession()) return result;
        if (feedbackMode === "always") {
          completeSuccess();
        } else {
          setOpen(false);
          resetSessionState();
        }
        return result;
      } catch (error) {
        if (!isCurrentSession()) throw error;
        const pendingFailure =
          pendingFailureRef.current as PendingFailure | null;
        const pendingMessage = pendingFailure ? pendingFailure.message : "";
        const message =
          parseErrorText(error) ||
          normalizeString(pendingMessage) ||
          t("common.error");
        appendLog("error", message);
        if (errorConsole) {
          setFailedState(message);
        } else {
          setOpen(false);
          resetSessionState();
        }
        throw error;
      }
    },
    [
      appendLog,
      clearCloseTimer,
      completeSuccess,
      resetProgressState,
      resetSessionState,
      setFailedState,
      setOuterValue,
      setProgressValue,
      t,
    ],
  );

  useEffect(() => {
    const offs: Array<() => void> = [];

    offs.push(
      Events.On("lip_task_started", (event: unknown) => {
        const data = getEventData<LipTaskStartedEvent>(event);
        if (!data) return;

        const taskID = normalizeString(data.taskId);
        const method = normalizeString(data.method);
        const targetValue = normalizeString(data.target);
        const timestampValue = Number(data.timestamp || 0);

        if (!taskID || !matchSessionBase(targetValue, method, timestampValue)) {
          return;
        }

        const session = sessionRef.current;
        if (!session || session.retiredTaskIDs.has(taskID)) return;
        session.trackedTaskIDs.add(taskID);
        // A composite call runs its tasks one after another: show the bars
        // of the latest one.
        setBars((prev) => beginLipTaskBars(prev, taskID));

        const packages = Array.isArray(data.packages)
          ? data.packages.map((item) => normalizeString(item)).filter(Boolean)
          : [];
        const packageText = packages.length > 0 ? packages.join(", ") : method;
        appendLog("info", packageText);
      }),
    );

    offs.push(
      Events.On("lip_task_log", (event: unknown) => {
        const data = getEventData<LipTaskLogEvent>(event);
        if (!data) return;

        const taskID = normalizeString(data.taskId);
        const method = normalizeString(data.method);
        const targetValue = normalizeString(data.target);
        const timestampValue = Number(data.timestamp || 0);

        if (!isTrackedEvent(taskID, targetValue, method, timestampValue)) {
          return;
        }

        const message =
          normalizeString(data.message) || normalizeString(data.raw);
        if (!message) return;
        appendLog(normalizeLogLevel(data.level), message);
      }),
    );

    offs.push(
      Events.On("lip_task_progress", (event: unknown) => {
        const data = getEventData<LipTaskProgressEvent>(event);
        if (!data) return;

        const taskID = normalizeString(data.taskId);
        const method = normalizeString(data.method);
        const targetValue = normalizeString(data.target);
        const timestampValue = Number(data.timestamp || 0);

        if (!isTrackedEvent(taskID, targetValue, method, timestampValue)) {
          return;
        }
        if (taskID && sessionRef.current?.retiredTaskIDs.has(taskID)) return;

        if (isLegacyLipProgress(data)) {
          setProgressValue(
            Number(data.percentage || 0),
            normalizeString(data.message),
          );
          return;
        }
        setBars((prev) => applyLipProgressEvent(prev, data));
      }),
    );

    offs.push(
      Events.On("lip_task_finished", (event: unknown) => {
        const data = getEventData<LipTaskFinishedEvent>(event);
        if (!data) return;

        const taskID = normalizeString(data.taskId);
        const method = normalizeString(data.method);
        const targetValue = normalizeString(data.target);
        const timestampValue = Number(data.timestamp || 0);

        if (!isTrackedEvent(taskID, targetValue, method, timestampValue)) {
          return;
        }

        if (data.success) {
          pendingFailureRef.current = null;
          appendLog("info", `${method || "task"} finished`);
          return;
        }

        const eventError = normalizeString(data.error);
        const failureMessage = eventError || t("common.error");

        if (eventError) {
          appendLog("error", eventError);
        }

        pendingFailureRef.current = {
          message: failureMessage,
        };

        if (!runningRef.current && sessionRef.current?.errorConsole) {
          setFailedState(failureMessage);
        }
      }),
    );

    return () => {
      for (const off of offs) {
        try {
          off();
        } catch {}
      }
    };
  }, [
    appendLog,
    isTrackedEvent,
    matchSessionBase,
    setFailedState,
    setProgressValue,
    t,
  ]);

  useEffect(() => {
    if (!logContainerRef.current) return;
    logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight;
  }, [logs, logExpanded]);

  useEffect(
    () => () => {
      clearCloseTimer();
    },
    [clearCloseTimer],
  );

  const modalType = useMemo(() => {
    if (status === "failed") return "error" as const;
    if (status === "success") return "success" as const;
    return "primary" as const;
  }, [status]);

  const statusText = useMemo(() => {
    if (status === "failed") return t("lip.task_console.status_failed");
    if (status === "success") return t("lip.task_console.status_success");
    return t("lip.task_console.status_running");
  }, [status, t]);

  const statusChipColor = useMemo(() => {
    if (status === "failed") return "danger" as const;
    if (status === "success") return "success" as const;
    return "primary" as const;
  }, [status]);

  const modalTitle = useMemo(() => {
    const base =
      action === "install"
        ? t("lip.task_console.title_install")
        : action === "upgrade"
          ? t("lip.task_console.title_upgrade")
          : action === "downgrade"
            ? t("lip.task_console.title_downgrade")
            : action === "reinstall"
              ? t("lip.task_console.title_reinstall")
              : action === "update"
                ? t("lip.task_console.title_update")
                : action === "uninstall"
                  ? t("lip.task_console.title_uninstall")
                  : t("lip.task_console.title_generic");
    if (!target) return base;
    return `${base} - ${target}`;
  }, [action, target, t]);

  const handleCopyLogs = useCallback(async () => {
    const lines: string[] = [];
    lines.push(`${t("lip.task_console.target_label")}: ${target || "-"}`);
    lines.push(`${t("lip.task_console.status_running")}: ${statusText}`);
    if (errorText) {
      lines.push(`${t("lip.task_console.status_failed")}: ${errorText}`);
    }
    lines.push("");
    for (const item of logs) {
      const time = new Date(item.timestamp).toLocaleTimeString(getIntlLocale());
      lines.push(`[${time}] [${item.level}] ${item.message}`);
    }

    try {
      await Clipboard.SetText(lines.join("\n"));
      toast(t("common.success"), {
        variant: "success",
        description: t("lip.task_console.copy_success"),
        timeout: 2000,
      });
    } catch {
      toast(t("common.error"), {
        variant: "danger",
        description: t("common.error"),
        timeout: 2000,
      });
    }
  }, [errorText, logs, statusText, t, target]);

  const contextValue = useMemo<LipTaskConsoleContextValue>(
    () => ({
      runWithLipTask,
    }),
    [runWithLipTask],
  );

  const progressSnapshot = useMemo<LipTaskProgressSnapshot>(
    () => ({
      status,
      target,
      primary: selectLipPrimaryBar(bars),
      secondary: selectLipSecondaryBars(bars),
      outer,
      legacy: legacyProgress,
    }),
    [bars, legacyProgress, outer, status, target],
  );

  // The compact view shows a running "progress" session; a failure, or a
  // session in "always" mode, shows the full console.
  const compact = feedbackMode === "progress" && status !== "failed";

  const logPanel = (
    <div
      ref={logContainerRef}
      className="min-h-0 flex-1 overflow-y-auto custom-scrollbar rounded-xl border border-border dark:border-zinc-700 bg-surface-secondary/50 dark:bg-surface-secondary/60 px-3 py-2"
    >
      {logs.length === 0 ? (
        <div className="text-xs text-muted dark:text-zinc-500">
          {t("lip.task_console.no_output")}
        </div>
      ) : (
        <div className="flex flex-col gap-1">
          {logs.map((item) => (
            <div
              key={item.id}
              className={`text-xs font-mono whitespace-pre-wrap break-all ${
                item.level === "error"
                  ? "text-rose-600 dark:text-rose-400"
                  : item.level === "warning"
                    ? "text-amber-600 dark:text-amber-400"
                    : item.level === "success"
                      ? "text-green-600 dark:text-green-400"
                      : "text-foreground dark:text-zinc-200"
              }`}
            >
              [{new Date(item.timestamp).toLocaleTimeString(getIntlLocale())}]{" "}
              {item.message}
            </div>
          ))}
        </div>
      )}
    </div>
  );

  const statusHeader = (
    <div className="flex min-w-0 items-center gap-2 text-sm">
      <span className="shrink-0 text-muted dark:text-zinc-400">
        {t("lip.task_console.target_label")}:
      </span>
      <span className="min-w-0 truncate font-mono">{target || "-"}</span>
      <Chip
        size="sm"
        variant="soft"
        color={statusChipColor === "primary" ? "accent" : statusChipColor}
      >
        <Chip.Label>{statusText}</Chip.Label>
      </Chip>
    </div>
  );

  return (
    <LipTaskConsoleContext.Provider value={contextValue}>
      <LipTaskProgressContext.Provider value={progressSnapshot}>
        {children}
        <UnifiedModal
          size={compact ? "standard" : "wide"}
          isOpen={open}
          onOpenChange={(nextOpen) => {
            if (nextOpen) {
              setOpen(true);
              return;
            }
            closeConsole();
          }}
          type={modalType}
          scrollBehavior="inside"
          icon={<LuTerminal size={24} />}
          title={modalTitle}
          isDismissable={!running}
          footer={
            compact ? null : (
              <>
                <ModalAction
                  onPress={() => void handleCopyLogs()}
                  variant={"secondary"}
                >
                  {t("lip.task_console.copy_logs")}
                </ModalAction>
                <ModalAction
                  onPress={() => closeConsole()}
                  isDisabled={running}
                  variant="primary"
                >
                  {t("lip.task_console.close")}
                </ModalAction>
              </>
            )
          }
        >
          {compact ? (
            <div className="flex min-w-0 flex-col gap-4">
              {statusHeader}
              {/* The bars get their own bounded scroll region (tighter while
                  the log is open) so a long list of step bars never pushes
                  the modal body into a second scrollbar at 960x600. */}
              <div
                className={cn(
                  "min-w-0 overflow-y-auto custom-scrollbar",
                  logExpanded
                    ? "max-h-[clamp(4.5rem,calc(100vh-30rem),16rem)]"
                    : "max-h-[clamp(6rem,calc(100vh-18rem),28rem)]",
                )}
              >
                <LipTaskProgressView snapshot={progressSnapshot} />
              </div>
              <div className="flex min-w-0 flex-col gap-2">
                <Button
                  size="sm"
                  variant="ghost"
                  className="self-start"
                  aria-expanded={logExpanded}
                  onPress={() => setLogExpanded((prev) => !prev)}
                >
                  {logExpanded ? (
                    <LuChevronDown size={14} />
                  ) : (
                    <LuChevronRight size={14} />
                  )}
                  {logExpanded
                    ? t("lip.task_progress.hide_log")
                    : t("lip.task_progress.show_log")}
                </Button>
                {logExpanded ? (
                  <div className="flex h-[min(30vh,16rem)] min-h-[8rem] flex-col">
                    {logPanel}
                  </div>
                ) : null}
              </div>
            </div>
          ) : (
            <div className="flex h-[min(52vh,32rem)] min-h-[15rem] flex-col gap-3">
              <div className="shrink-0">{statusHeader}</div>
              {/* Bars shrink and scroll on their own before the log loses its
                  guaranteed minimum height. */}
              <div className="flex min-h-0 shrink flex-col gap-2">
                <div className="shrink-0 text-xs text-muted dark:text-zinc-400">
                  {t("lip.task_console.progress_label")}
                </div>
                <div className="min-h-0 overflow-y-auto custom-scrollbar">
                  <LipTaskProgressView snapshot={progressSnapshot} />
                </div>
              </div>

              {errorText ? (
                <ModalNotice
                  tone="danger"
                  className="max-h-24 shrink-0 overflow-y-auto whitespace-pre-wrap break-all custom-scrollbar"
                >
                  {errorText}
                </ModalNotice>
              ) : null}

              <div className="flex min-h-[7rem] flex-1 flex-col space-y-1">
                <ModalDescription>
                  {t("lip.task_console.log_title")}
                </ModalDescription>
                {logPanel}
              </div>
            </div>
          )}
        </UnifiedModal>
      </LipTaskProgressContext.Provider>
    </LipTaskConsoleContext.Provider>
  );
};

export const useLipTaskConsole = (): LipTaskConsoleContextValue =>
  useContext(LipTaskConsoleContext);

// Live progress of the running lip task session, for pages that show it
// inline (feedbackMode "inline").
export const useLipTaskProgress = (): LipTaskProgressSnapshot =>
  useContext(LipTaskProgressContext);

// Inline progress of the running lip task session; renders nothing unless a
// session is running.
export const LipTaskProgress: React.FC<{ className?: string }> = ({
  className,
}) => {
  const snapshot = useLipTaskProgress();
  if (snapshot.status !== "running") return null;
  return <LipTaskProgressView snapshot={snapshot} className={className} />;
};
