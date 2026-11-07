import { openDirectory, showDirectoryOpenError } from "@/utils/explorer";
import React, { useEffect } from "react";

import { Events } from "@wailsio/runtime";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  appendTaskLog,
  finishActivityTask,
  notify,
  setTaskProgress,
  startActivityTask,
  type ActivityLogLevel,
} from "@/utils/activity";
import { compareVersions } from "@/utils/version";
import {
  readCurrentVersionName,
  saveCurrentVersionName,
} from "@/utils/currentVersion";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import {
  EnsureGameInputInteractive,
  EnsureVcRuntimeInteractive,
  ListDir,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import * as contentService from "bindings/github.com/liteldev/LeviLauncher/internal/app/contentservice";
import * as modsService from "bindings/github.com/liteldev/LeviLauncher/internal/app/modsservice";
import * as versionService from "bindings/github.com/liteldev/LeviLauncher/internal/app/versionservice";
import * as userService from "bindings/github.com/liteldev/LeviLauncher/internal/app/userservice";
import { getPlayerGamertagMap, listPlayers } from "@/utils/content";
import { ROUTES } from "@/constants/routes";
import { normalizePackageType } from "@/utils/packageType";
import { resolveInstallError } from "@/utils/installError";

const IGNORE_GS_KEY = "ll.ignore.gs";
const DEPENDENCY_CHECK_SESSION_KEYS = {
  gameInput: "ll.session.dependency-check.gameinput",
  gamingServices: "ll.session.dependency-check.gamingservices",
  vcRuntime: "ll.session.dependency-check.vcruntime",
} as const;
const VC_RUNTIME_CHECK_RETRY_DELAY_MS = 800;

const hasSessionDependencyCheckRun = (
  key: (typeof DEPENDENCY_CHECK_SESSION_KEYS)[keyof typeof DEPENDENCY_CHECK_SESSION_KEYS],
): boolean => {
  try {
    return window.sessionStorage.getItem(key) === "1";
  } catch {
    return false;
  }
};

const markSessionDependencyCheckRun = (
  key: (typeof DEPENDENCY_CHECK_SESSION_KEYS)[keyof typeof DEPENDENCY_CHECK_SESSION_KEYS],
) => {
  try {
    window.sessionStorage.setItem(key, "1");
  } catch {}
};

const getNextRandomTipIndex = (currentIndex: number, totalTips: number) => {
  const normalizedTotal = Math.max(1, totalTips);

  if (normalizedTotal <= 1) {
    return 0;
  }

  let nextIndex = Math.floor(Math.random() * normalizedTotal);

  if (nextIndex === currentIndex) {
    nextIndex = (nextIndex + 1) % normalizedTotal;
  }

  return nextIndex;
};

// Launch steps: every prerequisite (loader migration, runtimes, Gaming
// Services), registration and the launch itself is a step in one dialog. Steps
// keep their log once finished so the dialog reads as a history, and the same
// lines are mirrored into the activity store.
export type LaunchStepId =
  | "migration"
  | "vcRuntime"
  | "gameInput"
  | "gamingServices"
  | "register"
  | "unregister"
  | "launch";

export type LaunchStepStatus =
  | "checking"
  | "ready"
  | "missing"
  | "waiting"
  | "running"
  | "done"
  | "skipped"
  | "failed";

export type LaunchStepLogLine = {
  at: number;
  level: ActivityLogLevel;
  message: string;
};

export type LaunchStep = {
  id: LaunchStepId;
  status: LaunchStepStatus;
  progress?: number;
  detail?: string;
  /** Error code or message of the last failure. */
  error?: string;
  /** The user already tried to install this dependency once. */
  attempted?: boolean;
  log: LaunchStepLogLine[];
};

export type LaunchIntent = "check" | "register" | "unregister" | "launch";

export type LaunchFlow = {
  intent: LaunchIntent;
  target: string;
  open: boolean;
  taskId: string | null;
  taskDone: boolean;
  /** Log lines per step already copied into the activity task. */
  logged: Partial<Record<LaunchStepId, number>>;
  steps: LaunchStep[];
};

export type LaunchFlowPhase = "working" | "success" | "incomplete" | "failed";

export type DependencyStepId = "vcRuntime" | "gameInput" | "gamingServices";

const STEP_ORDER: LaunchStepId[] = [
  "migration",
  "vcRuntime",
  "gameInput",
  "gamingServices",
  "register",
  "unregister",
  "launch",
];

const TERMINAL_STEP_STATUSES = new Set<LaunchStepStatus>([
  "ready",
  "done",
  "skipped",
  "failed",
]);

export const isTerminalLaunchStep = (step: LaunchStep) =>
  TERMINAL_STEP_STATUSES.has(step.status);

export const getLaunchFlowPhase = (
  flow: LaunchFlow | null,
): LaunchFlowPhase | null => {
  if (!flow) return null;
  if (!flow.steps.every(isTerminalLaunchStep)) return "working";
  // The step the user asked for decides the outcome; earlier skipped or
  // failed prerequisites stay listed but don't overrule it.
  const own =
    flow.intent === "check"
      ? undefined
      : flow.steps.find((step) => step.id === flow.intent);
  if (own?.status === "done") return "success";
  if (own?.status === "failed") return "failed";
  if (flow.steps.some((step) => step.status === "failed")) return "failed";
  if (flow.steps.some((step) => step.status === "skipped")) return "incomplete";
  return "success";
};

export const getCurrentLaunchStep = (flow: LaunchFlow | null) =>
  flow?.steps.find((step) => !isTerminalLaunchStep(step));

/** Readable text for an error code from launch, registration or shortcuts. */
export const resolveLaunchErrorMessage = (
  code: string,
  t: (key: string) => unknown,
  fallbackKey?: string,
): string => {
  if (!code) return "";
  if (code.includes("ERR_UWP_")) return resolveInstallError(code, t as any);
  const key = `errors.${code}`;
  const translated = String(t(key));
  if (translated && translated !== key) return translated;
  return fallbackKey ? String(t(fallbackKey)) : code;
};

const createLaunchFlow = (intent: LaunchIntent, target: string): LaunchFlow => ({
  intent,
  target,
  open: false,
  taskId: null,
  taskDone: false,
  logged: {},
  steps: [],
});

const formatMegabytes = (done: number, total: number) =>
  `${(done / 1024 / 1024).toFixed(1)} / ${(total / 1024 / 1024).toFixed(1)} MB`;

// A missing binding means the check can't run, so nothing is reported missing.
const runCheck = (check: unknown): Promise<boolean> =>
  typeof check === "function"
    ? Promise.resolve(check()).then(Boolean)
    : Promise.resolve(true);

const DEPENDENCY_CHECKS: Record<DependencyStepId, () => Promise<boolean>> = {
  vcRuntime: () => runCheck((minecraft as any)?.IsVcRuntimeInstalled),
  gameInput: () => runCheck(minecraft?.IsGameInputInstalled),
  gamingServices: () => runCheck(minecraft?.IsGamingServicesInstalled),
};

export const useLauncher = (args: any) => {
  const { t } = useTranslation();
  const tRef = React.useRef(t);
  tRef.current = t;
  const [isAnimating, setIsAnimating] = React.useState(true);
  let [currentVersion, setCurrentVersion] = React.useState<string>("");
  const [displayVersion, setDisplayVersion] = React.useState<string>("");
  const [displayName, setDisplayName] = React.useState<string>("");
  const [localVersionMap, setLocalVersionMap] = React.useState<
    Map<string, any>
  >(new Map());

  const currentPackageType = localVersionMap.has(currentVersion)
    ? normalizePackageType(localVersionMap.get(currentVersion)?.packageType)
    : undefined;

  const navigate = useNavigate();
  const [contentCounts, setContentCounts] = React.useState<{
    worlds: number;
    resourcePacks: number;
    behaviorPacks: number;
  }>({ worlds: 0, resourcePacks: 0, behaviorPacks: 0 });
  const [logoDataUrl, setLogoDataUrl] = React.useState<string>("");
  const [versionQuery, setVersionQuery] = React.useState<string>("");
  const [logoByName, setLogoByName] = React.useState<Map<string, string>>(
    new Map(),
  );
  const [isLoadingVersions, setIsLoadingVersions] =
    React.useState<boolean>(false);
  const [tipIndex, setTipIndex] = React.useState<number>(0);
  const tipTimerRef = React.useRef<number | null>(null);
  const launchRequestActiveRef = React.useRef<boolean>(false);
  const [launchPending, setLaunchPending] = React.useState(false);
  const [registrationPendingAction, setRegistrationPendingAction] = React.useState<"register" | "unregister" | null>(null);
  const registrationRequestActiveRef = React.useRef(false);
  const registrationRevisionRef = React.useRef(0);

  const [launchFlow, setLaunchFlow] = React.useState<LaunchFlow | null>(null);
  const launchFlowRef = React.useRef<LaunchFlow | null>(null);
  const downloadTotalsRef = React.useRef<Partial<Record<LaunchStepId, number>>>({});
  const taskProgressRef = React.useRef<number | undefined>(undefined);

  const stepLabel = React.useCallback(
    (id: LaunchStepId) => String(tRef.current(`home.steps.step.${id}`)),
    [],
  );

  // Stores the next flow and mirrors new log lines, progress and the outcome
  // into the activity store. The task starts when the dialog first opens, so
  // silent checks that find nothing missing leave no history behind.
  const commitLaunchFlow = React.useCallback(
    (next: LaunchFlow | null) => {
      let flow = next;
      const phase = getLaunchFlowPhase(flow);
      // Once a task exists it keeps mirroring even while the dialog is hidden
      // (a launch that finishes in the background still records its outcome).
      if (flow?.open && (!flow.taskId || (flow.taskDone && phase === "working"))) {
        const title = String(
          tRef.current(`home.steps.task.${flow.intent}`, {
            name: flow.target,
          }),
        );
        flow = {
          ...flow,
          taskId: startActivityTask({ title, source: "launcher" }),
          taskDone: false,
          logged: flow.taskId ? flow.logged : {},
        };
        taskProgressRef.current = undefined;
      }
      if (flow?.taskId) {
        const taskId = flow.taskId;
        const logged = { ...flow.logged };
        flow.steps.forEach((step) => {
          const from = logged[step.id] ?? 0;
          step.log.slice(from).forEach((line) => {
            appendTaskLog(taskId, `${stepLabel(step.id)}: ${line.message}`, line.level);
          });
          logged[step.id] = step.log.length;
        });
        flow = { ...flow, logged };
        const progress = getCurrentLaunchStep(flow)?.progress;
        if (typeof progress === "number") {
          const rounded = Math.floor(progress);
          if (rounded !== taskProgressRef.current) {
            taskProgressRef.current = rounded;
            setTaskProgress(taskId, rounded);
          }
        }
        if (phase !== "working" && !flow.taskDone) {
          finishActivityTask(
            taskId,
            phase === "failed" ? "error" : "success",
            String(tRef.current(`home.steps.phase.${phase}`)),
          );
          flow = { ...flow, taskDone: true };
        }
      }
      launchFlowRef.current = flow;
      setLaunchFlow(flow);
    },
    [stepLabel],
  );

  /** Reuse the current flow while it is visible or still working. */
  const activeLaunchFlow = React.useCallback(() => {
    const flow = launchFlowRef.current;
    if (!flow) return null;
    return flow.open || getLaunchFlowPhase(flow) === "working" ? flow : null;
  }, []);

  const updateLaunchStep = React.useCallback(
    (
      id: LaunchStepId,
      patch: Partial<Omit<LaunchStep, "id" | "log">>,
      message?: string,
      level: ActivityLogLevel = "info",
      options: { insert?: boolean; open?: boolean; intent?: LaunchIntent; target?: string } = {},
    ) => {
      let flow = activeLaunchFlow();
      if (!flow) {
        if (!options.insert) return;
        flow = createLaunchFlow(
          options.intent ?? "check",
          options.target ?? readCurrentVersionName(),
        );
      } else if (options.intent) {
        flow = {
          ...flow,
          intent: options.intent,
          target: options.target ?? flow.target,
        };
      }
      const index = flow.steps.findIndex((step) => step.id === id);
      if (index < 0 && !options.insert) return;
      const base: LaunchStep =
        index >= 0 ? flow.steps[index] : { id, status: "checking", log: [] };
      const step: LaunchStep = {
        ...base,
        ...patch,
        id,
        log: message
          ? [...base.log, { at: Date.now(), level, message }]
          : base.log,
      };
      const steps = [...flow.steps];
      if (index >= 0) steps[index] = step;
      else {
        steps.push(step);
        steps.sort((a, b) => STEP_ORDER.indexOf(a.id) - STEP_ORDER.indexOf(b.id));
      }
      commitLaunchFlow({
        ...flow,
        steps,
        open: flow.open || Boolean(options.open),
      });
    },
    [activeLaunchFlow, commitLaunchFlow],
  );

  useEffect(() => {
    const registrationRevision = registrationRevisionRef.current;
    try {
      const saved = readCurrentVersionName();
      if (!saved) return;
      setCurrentVersion(saved);
      setDisplayName(saved);
      const fn = (versionService as any)?.GetVersionMeta;
      if (typeof fn === "function") {
        fn(saved)
          .then((m: any) => {
            const ver = String(m?.gameVersion || "");
            setDisplayVersion(ver || "");
            setLocalVersionMap((prev) => {
              const map = new Map(prev);
              const prevInfo = map.get(saved);
              const knownLeviLaminaStatus =
                typeof prevInfo?.isLeviLaminaInstalled === "boolean"
                  ? prevInfo.isLeviLaminaInstalled
                  : undefined;
              map.set(saved, {
                name: saved,
                version: ver,
                packageType: normalizePackageType(m?.packageType),
                type: String(m?.type || "release").toLowerCase(),
                isPreview: String(m?.type || "").toLowerCase() === "preview",
                isRegistered: registrationRevision === registrationRevisionRef.current
                  ? Boolean(m?.registered)
                  : Boolean(prevInfo?.isRegistered),
                isLaunched: false,
                isPreLoader: false,
                isLeviLaminaInstalled: knownLeviLaminaStatus,
              });
              return map;
            });
            const getter = (versionService as any)?.GetVersionLogoDataUrl;
            if (typeof getter === "function") {
              getter(saved).then((u: string) => {
                if (readCurrentVersionName() === saved) {
                  setLogoDataUrl(String(u || ""));
                }
              });
            }
          })
          .catch(() => {});
      }
    } catch {}
  }, []);

  useEffect(() => {
    try {
      versionService.ReconcileRegisteredFlags();
    } catch {}
  }, []);

  const startTipTimer = React.useCallback((tipsLength: number) => {
    try {
      if (tipTimerRef.current !== null) {
        clearInterval(tipTimerRef.current);
        tipTimerRef.current = null;
      }

      const normalizedTipsLength = Math.max(0, tipsLength);

      setTipIndex((prev) =>
        normalizedTipsLength <= 0 ? 0 : prev % normalizedTipsLength,
      );

      if (normalizedTipsLength <= 1) {
        return;
      }

      tipTimerRef.current = window.setInterval(() => {
        setTipIndex((prev) =>
          getNextRandomTipIndex(prev, normalizedTipsLength),
        );
      }, 10000);
    } catch {}
  }, []);

  const stopTipTimer = React.useCallback(() => {
    try {
      if (tipTimerRef.current !== null) {
        clearInterval(tipTimerRef.current);
        tipTimerRef.current = null;
      }
    } catch {}
  }, []);

  const sortedVersionNames = React.useMemo(() => {
    const entries = Array.from(localVersionMap.entries()).map(
      ([name, info]) => ({
        name,
        version: String(info?.version || ""),
        isPreview: !!info?.isPreview,
      }),
    );
    entries.sort((a, b) => {
      const byVer = compareVersions(b.version, a.version);
      if (byVer !== 0) return byVer;
      const byPreview = b.isPreview === a.isPreview ? 0 : b.isPreview ? 1 : -1;
      if (byPreview !== 0) return byPreview;
      return a.name.localeCompare(b.name);
    });
    return entries.map((e) => e.name);
  }, [localVersionMap, compareVersions]);

  const filteredVersionNames = React.useMemo(() => {
    const q = versionQuery.trim().toLowerCase();
    if (!q) return sortedVersionNames;
    return sortedVersionNames.filter((name) => {
      const ver = String(localVersionMap.get(name)?.version || "");
      return name.toLowerCase().includes(q) || ver.toLowerCase().includes(q);
    });
  }, [versionQuery, sortedVersionNames, localVersionMap]);

  const buildVersionMenuItems = React.useCallback(
    (emptyLabel: string) => {
      if (filteredVersionNames.length === 0) {
        return [
          {
            key: "__empty",
            name: emptyLabel,
            version: "",
            isRegistered: false,
            isDisabled: true,
          },
        ];
      }
      return filteredVersionNames.map((name) => ({
        key: name,
        name,
        version: String(localVersionMap.get(name)?.version || ""),
        packageType: normalizePackageType(localVersionMap.get(name)?.packageType),
        type: String(localVersionMap.get(name)?.type || "release"),
        isRegistered: Boolean(localVersionMap.get(name)?.isRegistered),
        isLeviLaminaInstalled: Boolean(
          localVersionMap.get(name)?.isLeviLaminaInstalled,
        ),
        isDisabled: false,
        logo: logoByName.get(name),
      }));
    },
    [filteredVersionNames, localVersionMap, logoByName],
  );

  const failLaunch = React.useCallback(
    (code: string) => {
      launchRequestActiveRef.current = false;
      const existing = launchFlowRef.current?.steps.find(
        (step) => step.id === "launch",
      );
      // The launch call and the process monitor can both report one failure.
      if (existing?.status === "failed" && existing.error === code) return;
      updateLaunchStep(
        "launch",
        { status: "failed", error: code, progress: undefined },
        String(tRef.current("home.steps.log.failed", { error: code })),
        "error",
        { insert: true, open: true },
      );
    },
    [updateLaunchStep],
  );

  const runLaunch = React.useCallback(
    (name: string, force: boolean) => {
      saveCurrentVersionName(name);
      launchRequestActiveRef.current = true;
      setLaunchPending(true);
      updateLaunchStep(
        "launch",
        { status: "running", error: undefined, progress: undefined },
        String(
          tRef.current(force ? "home.steps.log.force_launch" : "home.steps.log.launch_started", {
            name,
          }),
        ),
        "info",
        { insert: true, open: true, intent: "launch", target: name },
      );
      const launch = force
        ? versionService?.LaunchVersionByNameForce
        : versionService?.LaunchVersionByName;
      if (typeof launch !== "function") {
        launchRequestActiveRef.current = false;
        setLaunchPending(false);
        failLaunch("ERR_LAUNCH_GAME");
        return;
      }
      launch(name)
        .then((err: string) => {
          const s = String(err || "");
          if (!s) return;
          if (s.startsWith("ERR_UWP_NOT_REGISTERED")) {
            registrationRevisionRef.current += 1;
            setLocalVersionMap((previous) => {
              const next = new Map(previous);
              const current = next.get(name);
              if (current) next.set(name, { ...current, isRegistered: false });
              return next;
            });
          }
          failLaunch(s);
        })
        .catch(() => failLaunch("ERR_LAUNCH_GAME"))
        .finally(() => {
          setLaunchPending(false);
        });
    },
    [failLaunch, updateLaunchStep],
  );

  const doLaunch = React.useCallback(() => {
    if (launchRequestActiveRef.current || registrationRequestActiveRef.current) return;
    const name = currentVersion;
    if (name) {
      runLaunch(name, false);
    } else {
      launchRequestActiveRef.current = false;
      navigate(ROUTES.download);
    }
  }, [currentVersion, navigate, runLaunch]);

  const doForceLaunch = React.useCallback(() => {
    if (launchRequestActiveRef.current || registrationRequestActiveRef.current) return;
    if (currentVersion) runLaunch(currentVersion, true);
  }, [currentVersion, runLaunch]);

  const doCreateShortcut = React.useCallback(() => {
    const name = currentVersion;
    if (!name) return;
    const fail = (code: string) => {
      const message = resolveLaunchErrorMessage(code, tRef.current);
      notify({
        variant: "danger",
        source: "launcher",
        title: tRef.current("home.shortcut_failed"),
        description: message === code ? code : `${message} (${code})`,
      });
    };
    versionService
      ?.CreateDesktopShortcut(name)
      .then((err: string) => {
        const s = String(err || "");
        if (s) fail(s);
        else
          notify({
            variant: "success",
            source: "launcher",
            title: tRef.current("launcherpage.shortcut.success.title"),
            description: tRef.current("launcherpage.shortcut.success.body"),
          });
      })
      .catch(() => fail("ERR_SHORTCUT_CREATE_FAILED"));
  }, [currentVersion]);

  const doOpenFolder = React.useCallback(async () => {
    if (!currentVersion) return;
    try {
      const vdir = await versionService.GetVersionsDir();
      if (!vdir) return;
      const path = vdir + "\\" + currentVersion;
      await openDirectory(path);
    } catch (e) {
      showDirectoryOpenError(e);
    }
  }, [currentVersion]);

  const syncRegisteredFlags = React.useCallback(async () => {
    const registrationRevision = registrationRevisionRef.current;
    const listFn = versionService.ListVersionMetasWithRegistered;
    if (typeof listFn !== "function") return null;
    try {
      const metas = (await listFn()) || [];
      if (!Array.isArray(metas) || registrationRevision !== registrationRevisionRef.current) return null;
      setLocalVersionMap((prev) => {
        const next = new Map(prev);
        let changed = false;
        metas.forEach((m: any) => {
          const name = String(m?.name || "");
          if (!name) return;
          const version = String(m?.gameVersion || "");
          const isPreview =
            String(m?.type || "release").toLowerCase() === "preview";
          const isRegistered = Boolean(m?.registered);
          const existing = next.get(name);
          if (!existing) {
            next.set(name, {
              name,
              version,
              packageType: normalizePackageType(m?.packageType),
              type: String(m?.type || "release").toLowerCase(),
              isPreview,
              isRegistered,
              isLaunched: false,
              isPreLoader: false,
              isLeviLaminaInstalled: undefined,
            });
            changed = true;
            return;
          }
          if (
            String(existing?.version || "") !== version ||
            Boolean(existing?.isPreview) !== isPreview ||
            Boolean(existing?.isRegistered) !== isRegistered
          ) {
            next.set(name, {
              ...existing,
              version,
              packageType: normalizePackageType(m?.packageType),
              type: String(m?.type || "release").toLowerCase(),
              isPreview,
              isRegistered,
            });
            changed = true;
          }
        });
        return changed ? next : prev;
      });
      return metas;
    } catch {
      return null;
    }
  }, []);

  const applyOptimisticRegisterState = React.useCallback((name: string) => {
    if (!name) return;
    setLocalVersionMap((prev) => {
      const target = prev.get(name);
      if (!target) return prev;
      const targetIsPreview = Boolean(target?.isPreview);
      const next = new Map(prev);
      let changed = false;
      next.forEach((info, key) => {
        // GDK and UWP share the Windows package identity; Beta uses Release.
        if (Boolean(info?.isPreview) !== targetIsPreview) return;
        const shouldRegistered = key === name;
        if (Boolean(info?.isRegistered) !== shouldRegistered) {
          next.set(key, { ...info, isRegistered: shouldRegistered });
          changed = true;
        }
      });
      return changed ? next : prev;
    });
  }, []);

  const applyOptimisticUnregisterState = React.useCallback((name: string) => {
    if (!name) return;
    setLocalVersionMap((prev) => {
      const target = prev.get(name);
      if (!target || !target.isRegistered) return prev;
      const next = new Map(prev);
      next.set(name, { ...target, isRegistered: false });
      return next;
    });
  }, []);

  /** Resolves true once the instance is confirmed registered (UWP). */
  const doRegister = React.useCallback(async (registerOnly = false): Promise<boolean> => {
    if (!currentVersion || registrationRequestActiveRef.current || launchRequestActiveRef.current) return false;
    const isCurrentlyRegistered = Boolean(
      localVersionMap.get(currentVersion)?.isRegistered,
    );
    const startStep = (action: "register" | "unregister") =>
      updateLaunchStep(
        action,
        { status: "running", error: undefined },
        String(tRef.current(`home.steps.log.${action}_started`, { name: currentVersion })),
        "info",
        { insert: true, open: true, intent: action, target: currentVersion },
      );
    const finishStep = (action: "register" | "unregister") =>
      updateLaunchStep(
        action,
        { status: "done" },
        String(tRef.current(`home.steps.log.${action}_done`)),
        "success",
      );
    const failStep = (action: "register" | "unregister", code: string) =>
      updateLaunchStep(
        action,
        { status: "failed", error: code },
        String(tRef.current("home.steps.log.failed", { error: code })),
        "error",
        { insert: true, open: true },
      );
    if (currentPackageType === "uwp") {
      const action = isCurrentlyRegistered && !registerOnly ? "unregister" : "register";
      registrationRequestActiveRef.current = true;
      registrationRevisionRef.current += 1;
      setRegistrationPendingAction(action);
      startStep(action);
      let succeeded = false;
      try {
        const result = action === "register"
          ? await versionService.RegisterVersionWithWdapp(currentVersion, Boolean(localVersionMap.get(currentVersion)?.isPreview))
          : await versionService.UnregisterVersionByName(currentVersion);
        if (result !== "success" && result !== "") throw new Error(result);
        const metas = await syncRegisteredFlags();
        if (!metas) throw new Error("ERR_UWP_QUERY");
        const registered = Boolean(metas.find((meta) => meta.name === currentVersion)?.registered);
        if (action === "register" && !registered) throw new Error("ERR_UWP_NOT_REGISTERED");
        if (action === "unregister" && registered) throw new Error("ERR_UWP_UNREGISTER");
        finishStep(action);
        succeeded = action === "register";
      } catch (error) {
        // A failed registration must never enable the launch action optimistically.
        if (action === "register") applyOptimisticUnregisterState(currentVersion);
        failStep(action, String(error instanceof Error ? error.message : error));
      } finally {
        registrationRequestActiveRef.current = false;
        setRegistrationPendingAction(null);
      }
      return succeeded;
    }
    if (isCurrentlyRegistered && !registerOnly) {
      startStep("unregister");
      try {
        const fn = (versionService as any)?.UnregisterVersionByName;
        if (typeof fn !== "function") {
          failStep("unregister", "ERR_UNREGISTER_FAILED");
          return false;
        }
        const result = await fn(currentVersion);
        if (result === "") {
          applyOptimisticUnregisterState(currentVersion);
          const synced = await syncRegisteredFlags();
          if (!synced) {
            applyOptimisticUnregisterState(currentVersion);
          }
          window.setTimeout(() => {
            void syncRegisteredFlags();
          }, 1200);
          finishStep("unregister");
          return false;
        }
        failStep("unregister", result);
      } catch {
        failStep("unregister", "ERR_UNREGISTER_FAILED");
      }
      return false;
    }
    startStep("register");
    try {
      const isPreview = localVersionMap.get(currentVersion)?.isPreview || false;
      const result = await versionService.RegisterVersionWithWdapp(
        currentVersion,
        isPreview,
      );
      if (result === "success" || result === "") {
        finishStep("register");
        applyOptimisticRegisterState(currentVersion);
        const synced = await syncRegisteredFlags();
        if (!synced) {
          const fn = (versionService as any)?.GetVersionMeta;
          if (typeof fn === "function") {
            fn(currentVersion).then((m: any) => {
              setLocalVersionMap((prev) => {
                const map = new Map(prev);
                const existing = map.get(currentVersion);
                if (existing) {
                  map.set(currentVersion, {
                    ...existing,
                    isRegistered: Boolean(m?.registered),
                  });
                }
                return map;
              });
            });
          }
        }
        window.setTimeout(() => {
          void syncRegisteredFlags();
        }, 1200);
      } else {
        failStep("register", result);
      }
    } catch (e) {
      failStep("register", String(e));
    }
    return false;
  }, [
    currentVersion,
    currentPackageType,
    localVersionMap,
    applyOptimisticRegisterState,
    applyOptimisticUnregisterState,
    syncRegisteredFlags,
    updateLaunchStep,
  ]);

  const requiresUWPRegistration = currentPackageType === "uwp" && !Boolean(localVersionMap.get(currentVersion)?.isRegistered);
  const doPrimaryAction = React.useCallback(() => {
    if (requiresUWPRegistration) void doRegister(true);
    else doLaunch();
  }, [requiresUWPRegistration, doRegister, doLaunch]);

  const markDependency = React.useCallback(
    (id: DependencyStepId, installed: boolean) => {
      if (installed) {
        updateLaunchStep(
          id,
          { status: "ready" },
          String(tRef.current("home.steps.log.ready")),
          "success",
        );
      } else {
        updateLaunchStep(
          id,
          { status: "missing" },
          String(tRef.current("home.steps.log.missing")),
          "warning",
          { insert: true, open: true },
        );
      }
    },
    [updateLaunchStep],
  );

  useEffect(() => {
    let disposed = false;
    let vcRuntimeRetryTimer: number | null = null;

    const beginCheck = (id: DependencyStepId) =>
      updateLaunchStep(id, { status: "checking" }, undefined, "info", {
        insert: true,
      });

    const giveUpCheck = (id: DependencyStepId) =>
      updateLaunchStep(
        id,
        { status: "skipped" },
        String(tRef.current("home.steps.log.check_failed")),
        "warning",
      );

    const runVcRuntimeDependencyCheck = (attempt: number) => {
      const checkVcRuntime = (minecraft as any)?.IsVcRuntimeInstalled;
      if (typeof checkVcRuntime !== "function") {
        return;
      }
      if (attempt === 0) beginCheck("vcRuntime");

      Promise.resolve(checkVcRuntime())
        // Results still apply after a re-run: they only update the step.
        .then((ok: boolean) => {
          markSessionDependencyCheckRun(
            DEPENDENCY_CHECK_SESSION_KEYS.vcRuntime,
          );
          markDependency("vcRuntime", Boolean(ok));
        })
        .catch((error: unknown) => {
          // A newer effect run owns the check now.
          if (disposed) return;
          console.warn("Failed to check VC runtime status", error);
          if (attempt > 0 || vcRuntimeRetryTimer !== null) {
            giveUpCheck("vcRuntime");
            return;
          }
          vcRuntimeRetryTimer = window.setTimeout(() => {
            vcRuntimeRetryTimer = null;
            if (disposed) return;
            runVcRuntimeDependencyCheck(attempt + 1);
          }, VC_RUNTIME_CHECK_RETRY_DELAY_MS);
        });
    };

    const runDependencyCheck = (id: "gameInput" | "gamingServices") => {
      beginCheck(id);
      DEPENDENCY_CHECKS[id]()
        .then((ok) => markDependency(id, ok))
        .catch(() => giveUpCheck(id));
    };

    const timer = setTimeout(() => {
      if (
        !hasSessionDependencyCheckRun(DEPENDENCY_CHECK_SESSION_KEYS.vcRuntime)
      ) {
        try {
          runVcRuntimeDependencyCheck(0);
        } catch {}
      }
      // The native DLL loader needs the desktop VC runtime on UWP too.
      // GameInput and Gaming Services remain GDK-specific dependencies.
      if (currentPackageType !== "gdk") return;
      if (
        !hasSessionDependencyCheckRun(DEPENDENCY_CHECK_SESSION_KEYS.gameInput)
      ) {
        markSessionDependencyCheckRun(DEPENDENCY_CHECK_SESSION_KEYS.gameInput);
        runDependencyCheck("gameInput");
      }
      if (
        !hasSessionDependencyCheckRun(
          DEPENDENCY_CHECK_SESSION_KEYS.gamingServices,
        )
      ) {
        markSessionDependencyCheckRun(
          DEPENDENCY_CHECK_SESSION_KEYS.gamingServices,
        );
        let ignored = false;
        try {
          ignored = String(localStorage.getItem(IGNORE_GS_KEY) || "") === "1";
        } catch {}
        if (!ignored) runDependencyCheck("gamingServices");
      }
    }, 0);

    return () => {
      disposed = true;
      clearTimeout(timer);
      if (vcRuntimeRetryTimer !== null) {
        clearTimeout(vcRuntimeRetryTimer);
      }
    };
  }, [currentPackageType, markDependency, updateLaunchStep]);

  const runLoaderMigration = React.useCallback(async () => {
    updateLaunchStep(
      "migration",
      { status: "running", error: undefined, progress: undefined, detail: undefined },
      String(tRef.current("home.steps.log.migration_started")),
      "info",
      { insert: true, open: true },
    );
    try {
      const error = await minecraft.RunLoaderMigration();
      // The done/error events may already have settled the step.
      if (
        !launchFlowRef.current?.steps.some(
          (step) => step.id === "migration" && step.status === "running",
        )
      ) {
        return;
      }
      if (error) {
        updateLaunchStep(
          "migration",
          { status: "failed", error },
          String(tRef.current("home.steps.log.failed", { error })),
          "error",
        );
      } else {
        updateLaunchStep(
          "migration",
          { status: "done", progress: 100 },
          String(tRef.current("home.steps.log.migration_done")),
          "success",
        );
      }
    } catch (error) {
      const message = String(error);
      updateLaunchStep(
        "migration",
        { status: "failed", error: message },
        String(tRef.current("home.steps.log.failed", { error: message })),
        "error",
        { insert: true, open: true },
      );
    }
  }, [updateLaunchStep]);

  useEffect(() => {
    let cancelled = false;

    const unlistenMigrateStart = Events.On("leviloader.migrate.start", () => {
      updateLaunchStep(
        "migration",
        { status: "running", error: undefined },
        undefined,
        "info",
        { insert: true, open: true },
      );
    });
    const unlistenMigrateProgress = Events.On(
      "leviloader.migrate.progress",
      (event) => {
        const progress = event.data as { done: number; total: number };
        if (!progress || !(progress.total > 0)) return;
        updateLaunchStep("migration", {
          progress: Math.min(100, (progress.done / progress.total) * 100),
          detail: `${progress.done} / ${progress.total}`,
        });
      },
    );
    // The backend reports the outcome too; only a still-running step moves.
    const migrationRunning = () =>
      launchFlowRef.current?.steps.some(
        (step) => step.id === "migration" && step.status === "running",
      );
    const unlistenMigrateDone = Events.On("leviloader.migrate.done", () => {
      if (!migrationRunning()) return;
      updateLaunchStep(
        "migration",
        { status: "done", progress: 100 },
        String(tRef.current("home.steps.log.migration_done")),
        "success",
      );
    });
    const unlistenMigrateError = Events.On("leviloader.migrate.error", (event) => {
      if (!migrationRunning()) return;
      const error = String(event?.data || "") || "ERR_LOADER_MIGRATION";
      updateLaunchStep(
        "migration",
        { status: "failed", error },
        String(tRef.current("home.steps.log.failed", { error })),
        "error",
      );
    });
    (async () => {
      try {
        const need = await minecraft.NeedsLoaderMigration();
        if (cancelled || !need) return;
        await runLoaderMigration();
      } catch {}
    })();

    return () => {
      cancelled = true;
      unlistenMigrateStart();
      unlistenMigrateProgress();
      unlistenMigrateDone();
      unlistenMigrateError();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Installer events arrive both for installs started here and for ones the
  // backend starts on its own; either way they land in the same step.
  useEffect(() => {
    const unlisteners: Array<() => void> = [];
    const listen = (name: string, handler: (event: any) => void) => {
      const off = Events.On(name, handler);
      if (typeof off === "function") unlisteners.push(off as () => void);
    };
    const tr = (key: string, options?: Record<string, unknown>) =>
      String(tRef.current(key, options));

    (
      [
        ["gameinput", "gameInput"],
        ["vcruntime", "vcRuntime"],
      ] as const
    ).forEach(([prefix, id]) => {
      listen(`${prefix}.ensure.start`, () => {
        updateLaunchStep(
          id,
          { status: "running", attempted: true, error: undefined },
          tr("home.steps.log.install_started"),
          "info",
          { insert: true, open: true },
        );
      });
      listen(`${prefix}.download.start`, (event) => {
        const total = Number(event?.data || 0);
        downloadTotalsRef.current[id] = total;
        updateLaunchStep(
          id,
          {
            status: "running",
            progress: total > 0 ? 0 : undefined,
            detail: total > 0 ? formatMegabytes(0, total) : undefined,
          },
          total > 0
            ? tr("home.steps.log.downloading", {
                size: `${(total / 1024 / 1024).toFixed(1)} MB`,
              })
            : tr("home.steps.log.preparing"),
          "info",
          { insert: true, open: true },
        );
      });
      listen(`${prefix}.download.progress`, (event) => {
        const d = event?.data || {};
        const total =
          typeof d?.Total === "number" && d.Total > 0
            ? d.Total
            : downloadTotalsRef.current[id] || 0;
        const downloaded = typeof d?.Downloaded === "number" ? d.Downloaded : 0;
        if (!(total > 0)) return;
        downloadTotalsRef.current[id] = total;
        updateLaunchStep(id, {
          progress: Math.min(100, (downloaded / total) * 100),
          detail: formatMegabytes(downloaded, total),
        });
      });
      listen(`${prefix}.download.done`, () => {
        updateLaunchStep(
          id,
          { progress: undefined, detail: undefined },
          tr("home.steps.log.download_done"),
        );
      });
      listen(`${prefix}.download.error`, (event) => {
        const error = String(event?.data || "");
        updateLaunchStep(
          id,
          { error },
          tr("home.steps.log.download_error", { error }),
          "error",
        );
      });
      listen(`${prefix}.ensure.done`, (event) => {
        if (event?.data) {
          updateLaunchStep(
            id,
            { status: "done", progress: 100, detail: undefined, error: undefined },
            tr("home.steps.log.installed"),
            "success",
            { insert: true },
          );
        } else {
          updateLaunchStep(
            id,
            { status: "missing", progress: undefined, detail: undefined },
            tr("home.steps.log.install_incomplete"),
            "warning",
            { insert: true, open: true },
          );
        }
      });
    });

    listen("gamingservices.missing", () => {
      try {
        if (String(localStorage.getItem(IGNORE_GS_KEY) || "") === "1") return;
      } catch {}
      updateLaunchStep(
        "gamingServices",
        { status: "missing" },
        tr("home.steps.log.missing"),
        "warning",
        { insert: true, open: true },
      );
    });

    return () => {
      unlisteners.forEach((off) => {
        try {
          off();
        } catch {}
      });
    };
  }, [updateLaunchStep]);

  const refreshContentCounts = React.useCallback(async () => {
    const countDir = async (path: string): Promise<number> => {
      try {
        const entries = await ListDir(path);
        return (entries || []).filter((e: any) => e.isDir).length;
      } catch {
        return 0;
      }
    };
    try {
      const name = readCurrentVersionName();
      if (!name) {
        setContentCounts({ worlds: 0, resourcePacks: 0, behaviorPacks: 0 });
        return;
      }
      const roots = await contentService.GetContentRoots(name);
      const safe = roots || {
        base: "",
        usersRoot: "",
        resourcePacks: "",
        behaviorPacks: "",
        isIsolation: false,
        isPreview: false,
      };
      let worlds = 0;

      let res = 0;
      let bp = 0;
      try {
        const resEntries = await ListDir(safe.resourcePacks);
        const resDirs = (resEntries || []).filter((e: any) => e.isDir);
        res = resDirs.length;

        bp = await countDir(safe.behaviorPacks);
      } catch {}

      if (roots?.packageType === "uwp") {
        worlds = roots.worlds ? await countDir(roots.worlds) : 0;
        setContentCounts({ worlds, resourcePacks: res, behaviorPacks: bp });
        return;
      }
      if (safe.usersRoot) {
        try {
          const players = await listPlayers(safe.usersRoot);
          let nextPlayer = players[0] || "";

          if (nextPlayer) {
            const wp = `${safe.usersRoot}\\${nextPlayer}\\games\\com.mojang\\minecraftWorlds`;
            const defaultWorlds = await countDir(wp);
            setContentCounts({
              worlds: defaultWorlds,
              resourcePacks: res,
              behaviorPacks: bp,
            });
          } else {
            setContentCounts({
              worlds: 0,
              resourcePacks: res,
              behaviorPacks: bp,
            });
          }

          (async () => {
            try {
              const tag = await (userService as any)?.GetLocalUserGamertag?.();
              if (tag) {
                const map = await getPlayerGamertagMap(safe.usersRoot);

                let matchedPlayer = "";
                for (const p of players) {
                  if (map[p] === tag) {
                    matchedPlayer = p;
                    break;
                  }
                }

                if (matchedPlayer && matchedPlayer !== nextPlayer) {
                  const wp = `${safe.usersRoot}\\${matchedPlayer}\\games\\com.mojang\\minecraftWorlds`;
                  const newWorlds = await countDir(wp);
                  setContentCounts((prev) => ({ ...prev, worlds: newWorlds }));
                }
              }
            } catch {}
          })();
          return;
        } catch {}
      }
      setContentCounts({ worlds: 0, resourcePacks: res, behaviorPacks: bp });
    } catch {
      setContentCounts({ worlds: 0, resourcePacks: 0, behaviorPacks: 0 });
    }
  }, []);

  useEffect(() => {
    refreshContentCounts();
  }, [refreshContentCounts, currentVersion]);

  useEffect(() => {
    const unlistenMcStart = Events.On("mc.launch.start", () => {
      if (!launchRequestActiveRef.current) return;
      updateLaunchStep(
        "launch",
        {},
        String(tRef.current("home.steps.log.launch_waiting")),
      );
    });

    // A finished launch keeps its step (and the dialog, if visible) so the
    // result never flashes away; a hidden dialog only reopens on failure.
    const unlistenMcDone = Events.On("mc.launch.done", () => {
      launchRequestActiveRef.current = false;
      updateLaunchStep(
        "launch",
        { status: "done", error: undefined },
        String(tRef.current("home.steps.log.launched")),
        "success",
      );
    });
    const unlistenMcFailed = Events.On("mc.launch.failed", (data) => {
      const payload: any = (data as any)?.data ?? data;
      const first = Array.isArray(payload) ? payload[0] : payload;
      failLaunch(String(first || "") || "ERR_LAUNCH_GAME");
    });

    return () => {
      try {
        unlistenMcStart && (unlistenMcStart as any)();
      } catch {}
      try {
        unlistenMcDone && (unlistenMcDone as any)();
      } catch {}
      try {
        unlistenMcFailed && (unlistenMcFailed as any)();
      } catch {}
    };
  }, [failLaunch, updateLaunchStep]);

  useEffect(() => {
    const registrationRevision = registrationRevisionRef.current;
    setIsLoadingVersions(true);

    const processMetas = (metas: any[]) => {
      const newLocalVersionMap = new Map();
      const newLocalVersionsMap = new Map();
      metas?.forEach((m: any) => {
        const name = String(m?.name || "");
        const gameVersion = String(m?.gameVersion || "");
        const type = String(m?.type || "release");
        const isPreview = type.toLowerCase() === "preview";
        const lv: any = {
          name,
          version: gameVersion,
          packageType: normalizePackageType(m?.packageType),
          type: type.toLowerCase(),
          isPreview,
          isRegistered: Boolean(m?.registered),
          isLaunched: false,
          isPreLoader: false,
          isLeviLaminaInstalled: undefined,
        };
        if (name) newLocalVersionMap.set(name, lv);
        if (gameVersion) {
          if (!newLocalVersionsMap.has(gameVersion))
            newLocalVersionsMap.set(gameVersion, []);
          if (!newLocalVersionsMap.get(gameVersion)?.includes(name))
            newLocalVersionsMap.get(gameVersion)?.push(name);
        }
      });
      setLocalVersionMap((prev) => {
        const merged = new Map();
        newLocalVersionMap.forEach((info: any, name: string) => {
          const known = prev.get(name)?.isLeviLaminaInstalled;
          merged.set(name, {
            ...info,
            isRegistered: registrationRevision === registrationRevisionRef.current
              ? info.isRegistered
              : Boolean(prev.get(name)?.isRegistered),
            isLeviLaminaInstalled:
              typeof known === "boolean" ? known : undefined,
          });
        });
        return merged;
      });

      const saved = (() => {
        return readCurrentVersionName();
      })();
      const useName =
        saved && newLocalVersionMap.has(saved)
          ? saved
          : Array.from(newLocalVersionMap.keys())[0] || "";
      setCurrentVersion(useName);
      try {
        saveCurrentVersionName(useName, "launcher.refresh-versions");
      } catch {}
      const ver = useName
        ? newLocalVersionMap.get(useName)?.version || ""
        : "";
      setDisplayVersion(ver || "None");
      setDisplayName(useName || "");
      setLogoDataUrl("");
    };

    const fastFn = (versionService as any)?.ListVersionMetas;
    const slowFn = (versionService as any)?.ListVersionMetasWithRegistered;
    const detailsFn = (versionService as any)?.GetVersionMenuDetails;

    const applyDetails = (details: any[]) => {
      const detailList = Array.isArray(details) ? details : [];
      const nextLogos = new Map<string, string>();
      const detailByName = new Map<string, any>();

      detailList.forEach((detail: any) => {
        const name = String(detail?.name || "");
        if (!name) return;
        detailByName.set(name, detail);
        const logo = String(detail?.logoDataUrl || "");
        if (logo) {
          nextLogos.set(name, logo);
        }
      });

      setLogoByName(nextLogos);
      setLocalVersionMap((prev) => {
        const next = new Map(prev);
        detailByName.forEach((detail, name) => {
          const current = next.get(name);
          if (!current) return;
          next.set(name, {
            ...current,
            isRegistered: registrationRevision === registrationRevisionRef.current
              ? Boolean(detail?.registered)
              : Boolean(current?.isRegistered),
            isLeviLaminaInstalled: Boolean(detail?.leviLaminaInstalled),
          });
        });
        return next;
      });

      const selectedName = readCurrentVersionName();
      setLogoDataUrl(nextLogos.get(selectedName) || "");
    };

    const loadLegacyDetails = async () => {
      if (typeof slowFn !== "function") {
        throw new Error("Version detail fallback is unavailable");
      }

      const fullMetas = await slowFn();
      processMetas(fullMetas);
      setIsLoadingVersions(false);
      const details = await Promise.all(
        (Array.isArray(fullMetas) ? fullMetas : []).map(async (meta: any) => {
          const name = String(meta?.name || "");
          const [logoDataUrl, installedMods] = await Promise.all([
            Promise.resolve(
              (versionService as any)?.GetVersionLogoDataUrl?.(name),
            ).catch(() => ""),
            Promise.resolve((modsService as any)?.GetMods?.(name)).catch(
              () => [],
            ),
          ]);
          return {
            name,
            registered: Boolean(meta?.registered),
            logoDataUrl: String(logoDataUrl || ""),
            leviLaminaInstalled: (Array.isArray(installedMods)
              ? installedMods
              : []
            ).some(
              (mod: any) =>
                String(mod?.name || "").toLowerCase() === "levilamina",
            ),
          };
        }),
      );
      applyDetails(details);
    };

    if (typeof fastFn === "function") {
      fastFn()
        .then((metas: any[]) => {
          processMetas(metas);
          setIsLoadingVersions(false);

          if (typeof detailsFn === "function") {
            detailsFn()
              .then(applyDetails)
              .catch(() => void loadLegacyDetails().catch(() => {}));
          } else if (typeof slowFn === "function") {
            void loadLegacyDetails().catch(() => {});
          }
        })
        .catch(() => {
          if (typeof slowFn === "function") {
            void loadLegacyDetails().catch(() => setIsLoadingVersions(false));
          } else {
            setIsLoadingVersions(false);
          }
        });
    } else if (typeof slowFn === "function") {
      void loadLegacyDetails().catch(() => setIsLoadingVersions(false));
    } else {
      setIsLoadingVersions(false);
    }
  }, [args.count]);

  const handleVersionSelect = React.useCallback(
    (keys: any) => {
      const selected = Array.from(keys)[0] as string;
      if (selected) {
        setCurrentVersion(selected);
        setDisplayName(selected);
        const ver = localVersionMap.get(selected)?.version || "";
        setDisplayVersion(ver || "None");
        try {
          saveCurrentVersionName(selected, "launcher.select");
          setLogoDataUrl(logoByName.get(selected) || "");
        } catch {}
      }
    },
    [localVersionMap, logoByName],
  );

  const installDependency = React.useCallback(
    (id: "vcRuntime" | "gameInput") => {
      updateLaunchStep(
        id,
        { status: "running", attempted: true, error: undefined },
        String(tRef.current("home.steps.log.install_requested")),
      );
      try {
        if (id === "vcRuntime") EnsureVcRuntimeInteractive();
        else EnsureGameInputInteractive();
      } catch {}
    },
    [updateLaunchStep],
  );

  const recheckDependency = React.useCallback(
    (id: DependencyStepId) => {
      DEPENDENCY_CHECKS[id]()
        .catch(() => false)
        .then((ok) => {
          if (ok) {
            updateLaunchStep(
              id,
              { status: "done", progress: 100, detail: undefined, error: undefined },
              String(tRef.current("home.steps.log.recheck_ok")),
              "success",
            );
          } else {
            updateLaunchStep(
              id,
              {
                status: id === "gamingServices" ? "waiting" : "missing",
                progress: undefined,
                detail: undefined,
              },
              String(tRef.current("home.steps.log.recheck_missing")),
              "warning",
            );
          }
        });
    },
    [updateLaunchStep],
  );

  const skipDependency = React.useCallback(
    (id: DependencyStepId) => {
      updateLaunchStep(
        id,
        { status: "skipped" },
        String(tRef.current("home.steps.log.skipped")),
        "warning",
      );
    },
    [updateLaunchStep],
  );

  const ignoreGamingServices = React.useCallback(() => {
    try {
      localStorage.setItem(IGNORE_GS_KEY, "1");
    } catch {}
    updateLaunchStep(
      "gamingServices",
      { status: "skipped" },
      String(tRef.current("home.steps.log.ignored")),
      "warning",
    );
  }, [updateLaunchStep]);

  const openGamingServicesStore = React.useCallback(
    (openUrl: (url: string) => void) => {
      try {
        openUrl("ms-windows-store://pdp/?ProductId=9MWPM2CQNLHN");
      } catch {}
      updateLaunchStep(
        "gamingServices",
        { status: "waiting", attempted: true },
        String(tRef.current("home.steps.log.store_opened")),
      );
    },
    [updateLaunchStep],
  );

  /** Retries the failed step in place; a lost UWP registration registers first. */
  const retryLaunchStep = React.useCallback(
    (id: LaunchStepId) => {
      if (id === "migration") void runLoaderMigration();
      else if (id === "register") void doRegister(true);
      else if (id === "unregister") void doRegister();
      else if (id === "launch") {
        // A lost UWP registration is fixed in place, then the launch continues.
        if (requiresUWPRegistration) {
          void doRegister(true).then((registered) => {
            if (registered) doLaunch();
          });
        } else doLaunch();
      }
    },
    [doLaunch, doRegister, requiresUWPRegistration, runLoaderMigration],
  );

  const closeLaunchFlow = React.useCallback(() => {
    const flow = launchFlowRef.current;
    if (!flow?.open) return;
    const finished = getLaunchFlowPhase(flow) !== "working";
    commitLaunchFlow({ ...flow, open: false });
    if (!finished) return;
    const changedRegistration = flow.steps.some(
      (step) => step.id === "register" || step.id === "unregister",
    );
    if (changedRegistration) void syncRegisteredFlags();
    if (changedRegistration || flow.intent === "check") args.refresh();
  }, [args, commitLaunchFlow, syncRegisteredFlags]);

  const launchStepRunning =
    launchFlow?.open === true &&
    launchFlow.steps.some(
      (step) => step.id === "launch" && step.status === "running",
    );

  return {
    // State
    isAnimating,
    setIsAnimating,
    currentVersion,
    displayVersion,
    displayName,
    localVersionMap,
    contentCounts,
    logoDataUrl,
    versionQuery,
    setVersionQuery,
    logoByName,
    isLoadingVersions,
    registrationPendingAction,
    launchPending,
    launchStepRunning,
    requiresUWPRegistration,
    tipIndex,
    launchFlow,

    // Navigation
    navigate,

    // Computed
    buildVersionMenuItems,

    // Tip timer
    startTipTimer,
    stopTipTimer,

    // Handlers
    doLaunch,
    doPrimaryAction,
    doForceLaunch,
    doCreateShortcut,
    doOpenFolder,
    doRegister,
    handleVersionSelect,
    installDependency,
    recheckDependency,
    skipDependency,
    ignoreGamingServices,
    openGamingServicesStore,
    retryLaunchStep,
    closeLaunchFlow,
  };
};
