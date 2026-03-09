import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Button, Chip, Progress, addToast } from "@heroui/react";
import { Events } from "@wailsio/runtime";
import { useTranslation } from "react-i18next";
import { LuTerminal } from "react-icons/lu";
import { UnifiedModal } from "@/components/UnifiedModal";

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

type RunWithLipTaskOptions = {
  action?: LipTaskAction;
  target: string;
  methods?: string[];
  feedbackMode?: "always" | "on_error" | "never";
};

type RunWithLipTaskTools = {
  addLog: (level: LipTaskLogLevel, message: string) => void;
  setProgress: (percentage: number, message?: string) => void;
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

type LipTaskProgressEvent = {
  taskId?: string;
  method?: string;
  target?: string;
  packages?: string[];
  timestamp?: number;
  progressId?: string;
  message?: string;
  percentage?: number;
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
  feedbackMode: "always" | "on_error" | "never";
};

type PendingFailure = {
  message: string;
  stderr: string;
};

const MAX_LOG_COUNT = 800;
const SUCCESS_AUTO_CLOSE_MS = 700;
const DEFAULT_FEEDBACK_MODE: "always" | "on_error" | "never" = "on_error";

const LipTaskConsoleContext = createContext<LipTaskConsoleContextValue>({
  runWithLipTask: async <T,>(
    _options: RunWithLipTaskOptions,
    runner: (tools: RunWithLipTaskTools) => Promise<T>,
  ) =>
    runner({
      addLog: () => {},
      setProgress: () => {},
    }),
});

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
  const [stderrText, setStderrText] = useState<string>("");
  const [progress, setProgress] = useState<{
    percentage: number;
    message: string;
  } | null>(null);

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
      const safePercentage = Number.isFinite(percentage)
        ? Math.max(0, Math.min(100, percentage))
        : 0;
      setProgress({
        percentage: safePercentage,
        message: normalizeString(message),
      });
    },
    [],
  );

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
    setStderrText("");
    setProgress(null);
  }, []);

  const closeConsole = useCallback(() => {
    if (running) return;
    clearCloseTimer();
    setOpen(false);
    resetSessionState();
  }, [clearCloseTimer, resetSessionState, running]);

  const setFailedState = useCallback(
    (message: string, stderr: string) => {
      clearCloseTimer();
      runningRef.current = false;
      setRunning(false);
      setStatus("failed");
      if (normalizeString(message)) {
        setErrorText(normalizeString(message));
      }
      if (normalizeString(stderr)) {
        setStderrText(normalizeString(stderr));
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

      sessionRef.current = {
        id: ++sessionSeqRef.current,
        startedAt,
        targetNorm: normalizeTarget(targetName),
        methodNormSet,
        trackedTaskIDs: new Set(),
        feedbackMode: options.feedbackMode || DEFAULT_FEEDBACK_MODE,
      };

      pendingFailureRef.current = null;
      const feedbackMode = options.feedbackMode || DEFAULT_FEEDBACK_MODE;
      setOpen(feedbackMode === "always");
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
      setStderrText("");
      setProgress(null);

      const tools: RunWithLipTaskTools = {
        addLog: (level, message) => appendLog(level, message),
        setProgress: (percentage, message) =>
          setProgressValue(Number(percentage || 0), message),
      };

      try {
        const result = await runner(tools);
        if (feedbackMode === "always") {
          completeSuccess();
        } else {
          setOpen(false);
          resetSessionState();
        }
        return result;
      } catch (error) {
        const pendingFailure =
          pendingFailureRef.current as PendingFailure | null;
        const pendingMessage = pendingFailure ? pendingFailure.message : "";
        const pendingStderr = pendingFailure ? pendingFailure.stderr : "";
        const message =
          parseErrorText(error) ||
          normalizeString(pendingMessage) ||
          t("common.error");
        const stderr = normalizeString(pendingStderr);
        appendLog("error", message);
        if (feedbackMode === "on_error" || feedbackMode === "always") {
          setFailedState(message, stderr);
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
      resetSessionState,
      setFailedState,
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
        if (!session) return;
        session.trackedTaskIDs.add(taskID);

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

        setProgressValue(
          Number(data.percentage || 0),
          normalizeString(data.message),
        );
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
        const eventStderr = normalizeString(data.stderr);
        const failureMessage = eventError || t("common.error");

        if (eventError) {
          appendLog("error", eventError);
        }
        if (eventStderr) {
          setStderrText(eventStderr);
        }

        pendingFailureRef.current = {
          message: failureMessage,
          stderr: eventStderr,
        };

        if (!runningRef.current) {
          const feedbackMode =
            sessionRef.current?.feedbackMode || DEFAULT_FEEDBACK_MODE;
          if (feedbackMode === "on_error" || feedbackMode === "always") {
            setFailedState(failureMessage, eventStderr);
          }
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
  }, [logs]);

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

  const closeButtonColor = useMemo(() => {
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

  const showStderr = useMemo(
    () => Boolean(stderrText) && !(status === "failed" && action === "install"),
    [action, status, stderrText],
  );

  const handleCopyLogs = useCallback(async () => {
    const lines: string[] = [];
    lines.push(`${t("lip.task_console.target_label")}: ${target || "-"}`);
    lines.push(`${t("lip.task_console.status_running")}: ${statusText}`);
    if (errorText) {
      lines.push(`${t("lip.task_console.status_failed")}: ${errorText}`);
    }
    if (showStderr) {
      lines.push(`${t("lip.task_console.stderr_title")}:`);
      lines.push(stderrText);
    }
    lines.push("");
    for (const item of logs) {
      const time = new Date(item.timestamp).toLocaleTimeString();
      lines.push(`[${time}] [${item.level}] ${item.message}`);
    }

    try {
      await navigator.clipboard.writeText(lines.join("\n"));
      addToast({
        color: "success",
        title: t("common.success"),
        description: t("lip.task_console.copy_success"),
      });
    } catch {
      addToast({
        color: "danger",
        title: t("common.error"),
        description: t("common.error"),
      });
    }
  }, [errorText, logs, showStderr, statusText, stderrText, t, target]);

  const contextValue = useMemo<LipTaskConsoleContextValue>(
    () => ({
      runWithLipTask,
    }),
    [runWithLipTask],
  );

  return (
    <LipTaskConsoleContext.Provider value={contextValue}>
      {children}
      <UnifiedModal
        size="xl"
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
          <>
            <Button variant="flat" onPress={() => void handleCopyLogs()}>
              {t("lip.task_console.copy_logs")}
            </Button>
            <Button
              color={closeButtonColor}
              onPress={() => closeConsole()}
              isDisabled={running}
            >
              {t("lip.task_console.close")}
            </Button>
          </>
        }
      >
        <div className="flex h-[min(44vh,26rem)] min-h-[15rem] flex-col gap-3">
          <div className="shrink-0 space-y-3">
            <div className="flex items-center gap-2 text-small">
              <span className="text-default-500 dark:text-zinc-400">
                {t("lip.task_console.target_label")}:
              </span>
              <span className="font-mono">{target || "-"}</span>
              <Chip size="sm" variant="flat" color={statusChipColor}>
                {statusText}
              </Chip>
            </div>

            <div className="space-y-1">
              <div className="text-xs text-default-500 dark:text-zinc-400">
                {t("lip.task_console.progress_label")}
                {progress?.message ? ` - ${progress.message}` : ""}
              </div>
              {running ? (
                <Progress
                  aria-label={t("lip.task_console.progress_label") as string}
                  value={progress ? progress.percentage : undefined}
                  isIndeterminate={!progress}
                  color={status === "failed" ? "danger" : "primary"}
                />
              ) : progress ? (
                <Progress
                  aria-label={t("lip.task_console.progress_label") as string}
                  value={progress.percentage}
                  color={status === "failed" ? "danger" : "success"}
                />
              ) : null}
            </div>

            {errorText ? (
              <div className="rounded-xl border border-danger-300/60 bg-danger-50/60 dark:border-danger-500/30 dark:bg-danger-500/10 px-3 py-2 text-xs text-danger-700 dark:text-danger-300 whitespace-pre-wrap break-all">
                {errorText}
              </div>
            ) : null}

            {showStderr ? (
              <div className="space-y-1">
                <div className="text-xs font-semibold text-danger-600 dark:text-danger-400">
                  {t("lip.task_console.stderr_title")}
                </div>
                <div className="max-h-[120px] overflow-y-auto custom-scrollbar rounded-xl border border-danger-200/70 bg-danger-50/60 px-3 py-2 text-xs font-mono text-danger-700 dark:border-danger-500/30 dark:bg-danger-500/10 dark:text-danger-300 whitespace-pre-wrap break-all">
                  {stderrText}
                </div>
              </div>
            ) : null}
          </div>

          <div className="min-h-0 flex flex-1 flex-col space-y-1">
            <div className="text-xs font-semibold text-default-600 dark:text-zinc-300">
              {t("lip.task_console.log_title")}
            </div>
            <div
              ref={logContainerRef}
              className="min-h-0 flex-1 overflow-y-auto custom-scrollbar rounded-xl border border-default-200 dark:border-zinc-700 bg-default-100/50 dark:bg-zinc-800/60 px-3 py-2"
            >
              {logs.length === 0 ? (
                <div className="text-xs text-default-400 dark:text-zinc-500">
                  {t("lip.task_console.no_output")}
                </div>
              ) : (
                <div className="flex flex-col gap-1">
                  {logs.map((item) => (
                    <div
                      key={item.id}
                      className={`text-xs font-mono whitespace-pre-wrap break-all ${
                        item.level === "error"
                          ? "text-danger-600 dark:text-danger-400"
                          : item.level === "warning"
                            ? "text-warning-600 dark:text-warning-400"
                            : item.level === "success"
                              ? "text-success-600 dark:text-success-400"
                              : "text-default-700 dark:text-zinc-200"
                      }`}
                    >
                      [{new Date(item.timestamp).toLocaleTimeString()}]{" "}
                      {item.message}
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      </UnifiedModal>
    </LipTaskConsoleContext.Provider>
  );
};

export const useLipTaskConsole = (): LipTaskConsoleContextValue =>
  useContext(LipTaskConsoleContext);
