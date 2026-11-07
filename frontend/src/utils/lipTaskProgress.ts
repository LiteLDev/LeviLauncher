// Progress bars of lip tasks, built from lip_task_progress events. Every value
// shown comes from a measured event (bytes, objects, files, packages); nothing
// here is derived from elapsed time.

export type LipProgressStage =
  | "task"
  | "resolve"
  | "git_fetch"
  | "download"
  | "extract"
  | "place"
  | "scripts"
  | "remove"
  | "cache_clean"
  | "";

export type LipProgressUnit =
  | "bytes"
  | "files"
  | "objects"
  | "blobs"
  | "packages"
  | "commands"
  | "";

export type LipProgressEventData = {
  taskId?: string;
  progressId?: string;
  message?: string;
  percentage?: number;
  stage?: string;
  package?: string;
  current?: number;
  total?: number;
  unit?: string;
  item?: string;
  done?: boolean;
  indeterminate?: boolean;
};

export type LipProgressBar = {
  key: string;
  taskId: string;
  progressId: string;
  stage: LipProgressStage;
  packageName: string;
  message: string;
  current: number;
  total: number;
  unit: LipProgressUnit;
  item: string;
  // In [0, 100]; undefined while the total is unknown (indeterminate).
  percentage: number | undefined;
  done: boolean;
  order: number;
};

export type LipOuterProgress = {
  // Items finished so far, the item being worked on (1-based) and the count.
  completed: number;
  current: number;
  total: number;
  label: string;
};

export type LipLegacyProgress = {
  percentage: number;
  message: string;
};

// One line of the step history of a session: every distinct stage, batch item
// and progress message is appended once and stays listed after it ends.
export type LipProgressStep = {
  key: string;
  at: number;
  label: string;
  detail: string;
};

export type LipTaskProgressSnapshot = {
  status: "idle" | "running" | "success" | "failed";
  target: string;
  primary: LipProgressBar | null;
  secondary: LipProgressBar[];
  outer: LipOuterProgress | null;
  legacy: LipLegacyProgress | null;
  // Step history, oldest first, and the keys of the steps still in progress.
  steps?: LipProgressStep[];
  activeStepKeys?: string[];
};

export type LipProgressBarsState = {
  // The task whose bars are shown; a composite call runs several tasks in
  // sequence and the latest one replaces the bars of the previous one.
  taskId: string;
  bars: Record<string, LipProgressBar>;
  order: number;
};

export const EMPTY_LIP_PROGRESS_BARS: LipProgressBarsState = {
  taskId: "",
  bars: {},
  order: 0,
};

const STAGES: ReadonlySet<string> = new Set([
  "task",
  "resolve",
  "git_fetch",
  "download",
  "extract",
  "place",
  "scripts",
  "remove",
  "cache_clean",
]);

const UNITS: ReadonlySet<string> = new Set([
  "bytes",
  "files",
  "objects",
  "blobs",
  "packages",
  "commands",
]);

const toCount = (value: unknown): number => {
  const n = Number(value);
  return Number.isFinite(n) && n > 0 ? Math.floor(n) : 0;
};

export const normalizeLipStage = (value: unknown): LipProgressStage => {
  const s = String(value || "").trim();
  return STAGES.has(s) ? (s as LipProgressStage) : "";
};

export const normalizeLipUnit = (value: unknown): LipProgressUnit => {
  const s = String(value || "").trim();
  return UNITS.has(s) ? (s as LipProgressUnit) : "";
};

export const isLegacyLipProgress = (data: LipProgressEventData): boolean =>
  normalizeLipStage(data.stage) === "";

// Starts showing the bars of taskId, dropping those of an earlier task.
export const beginLipTaskBars = (
  state: LipProgressBarsState,
  taskId: string,
): LipProgressBarsState => {
  if (!taskId || state.taskId === taskId) return state;
  return { taskId, bars: {}, order: state.order };
};

// Applies one structured lip_task_progress update. Updates of a task other
// than the one shown are ignored once a task is shown; the first update seen
// selects its task when no lip_task_started was seen for it.
export const applyLipProgressEvent = (
  state: LipProgressBarsState,
  data: LipProgressEventData,
): LipProgressBarsState => {
  const taskId = String(data.taskId || "").trim();
  const progressId = String(data.progressId || "").trim();
  if (!progressId) return state;
  let next = state;
  if (!next.taskId) next = beginLipTaskBars(next, taskId);
  if (taskId && next.taskId && taskId !== next.taskId) return next;

  const key = `${taskId}::${progressId}`;
  const prev = next.bars[key];
  const total = toCount(data.total);
  const current = Math.min(toCount(data.current), total > 0 ? total : Infinity);
  const done = Boolean(data.done);
  let percentage: number | undefined;
  if (total > 0) {
    percentage = (current / total) * 100;
  } else if (done) {
    percentage = 100;
  } else {
    percentage = undefined;
  }
  const order = prev ? prev.order : next.order + 1;
  const bar: LipProgressBar = {
    key,
    taskId,
    progressId,
    stage: normalizeLipStage(data.stage),
    packageName: String(data.package || "").trim(),
    message: String(data.message || "").trim(),
    current,
    total,
    unit: normalizeLipUnit(data.unit),
    item: String(data.item || "").trim(),
    percentage,
    done,
    order,
  };
  return {
    taskId: next.taskId,
    bars: { ...next.bars, [key]: bar },
    order: Math.max(next.order, order),
  };
};

const latest = (bars: LipProgressBar[]): LipProgressBar | null =>
  bars.reduce<LipProgressBar | null>(
    (acc, bar) => (!acc || bar.order > acc.order ? bar : acc),
    null,
  );

// The primary bar is the task's packages-applied bar; before the plan is
// known it is the resolve bar.
export const selectLipPrimaryBar = (
  state: LipProgressBarsState,
): LipProgressBar | null => {
  const bars = Object.values(state.bars);
  return (
    latest(bars.filter((bar) => bar.stage === "task")) ||
    latest(bars.filter((bar) => bar.stage === "resolve"))
  );
};

// Secondary bars are the open bars of the steps of the task (downloads,
// extraction, copying, scripts, removal, cache cleaning), oldest first.
export const selectLipSecondaryBars = (
  state: LipProgressBarsState,
): LipProgressBar[] =>
  Object.values(state.bars)
    .filter(
      (bar) =>
        !bar.done &&
        bar.stage !== "task" &&
        bar.stage !== "resolve" &&
        bar.stage !== "",
    )
    .sort((a, b) => a.order - b.order);

// Key of the step a bar belongs to. The task bar names the package being
// applied, so each package is its own step; other bars are one step each.
export const lipStepKey = (bar: LipProgressBar): string =>
  bar.stage === "task" && bar.item
    ? `bar:${bar.key}:${bar.item}`
    : `bar:${bar.key}`;

// Keys of the steps that are still in progress in the given state.
export const activeLipStepKeys = (
  state: LipProgressBarsState,
  legacy: LipLegacyProgress | null,
  outer: LipOuterProgress | null,
): string[] => {
  const keys = Object.values(state.bars)
    .filter((bar) => bar.stage !== "" && !bar.done)
    .map(lipStepKey);
  if (legacy?.message) keys.push(`legacy:${legacy.message}`);
  if (outer && outer.completed < outer.total) {
    keys.push(`outer:${outer.current}:${outer.label}`);
  }
  return keys;
};

// Overall percentage of a session, or undefined while it is unknown. A batch
// counts its finished items plus the share of the running one.
export const overallLipPercentage = (
  primary: LipProgressBar | null,
  legacy: LipLegacyProgress | null,
  outer: LipOuterProgress | null,
): number | undefined => {
  const inner = primary ? primary.percentage : legacy?.percentage;
  if (outer && outer.total > 0) {
    const share = outer.completed < outer.total ? (inner ?? 0) / 100 : 0;
    return clampPercentage(((outer.completed + share) / outer.total) * 100);
  }
  return inner === undefined ? undefined : clampPercentage(inner);
};

export const clampPercentage = (value: unknown): number => {
  const n = Number(value);
  if (!Number.isFinite(n)) return 0;
  return Math.max(0, Math.min(100, n));
};
