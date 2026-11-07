import React from "react";
import { Chip, ProgressBar, Spinner } from "@heroui/react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import {
  FiAlertTriangle,
  FiCheck,
  FiClock,
  FiMinus,
  FiX,
} from "react-icons/fi";
import { UnifiedModal, type ModalType } from "@/components/UnifiedModal";
import {
  ModalAction,
  ModalDescription,
  ModalNotice,
} from "@/components/ModalPrimitives";
import {
  getCurrentLaunchStep,
  getLaunchFlowPhase,
  resolveLaunchErrorMessage,
  type DependencyStepId,
  type LaunchFlow,
  type LaunchFlowPhase,
  type LaunchStep,
  type LaunchStepId,
} from "@/hooks/useLauncher";
import { notify } from "@/utils/activity";
import { cn } from "@/utils/cn";

const RETRYABLE_STEPS = new Set<LaunchStepId>([
  "migration",
  "register",
  "unregister",
  "launch",
]);

/** Readable text for a failed step's error code. */
export const resolveLaunchStepError = (step: LaunchStep, t: TFunction) =>
  resolveLaunchErrorMessage(
    String(step.error || ""),
    t,
    step.id === "launch"
      ? "errors.ERR_LAUNCH_GAME"
      : step.id === "register" || step.id === "unregister"
        ? "launcherpage.register.failed.body"
        : undefined,
  );

const STEP_DESCRIPTIONS: Partial<Record<LaunchStepId, Partial<Record<LaunchStep["status"], string>>>> = {
  migration: { running: "launcherpage.loader_migration.body" },
  vcRuntime: {
    missing: "launcherpage.vcruntime.missing.body",
    running: "launcherpage.vcruntime.installing.body",
  },
  gameInput: {
    missing: "launcherpage.gameinput.missing.body",
    running: "launcherpage.gameinput.installing.body",
  },
  gamingServices: {
    missing: "launcherpage.gs.missing.body",
    waiting: "launcherpage.install_confirm.body",
  },
  register: { running: "launcherpage.register.installing.body" },
  unregister: { running: "versions.edit.unregister_progress.body" },
  launch: { running: "launcherpage.mclaunch.loading.body" },
};

const TITLES: Record<LaunchFlow["intent"], Record<LaunchFlowPhase, string>> = {
  check: {
    working: "home.steps.title.check",
    success: "home.steps.title.ready",
    incomplete: "home.steps.title.incomplete",
    failed: "home.steps.title.check_failed",
  },
  register: {
    working: "launcherpage.register.installing.title",
    success: "launcherpage.register.success.title",
    incomplete: "launcherpage.register.failed.title",
    failed: "launcherpage.register.failed.title",
  },
  unregister: {
    working: "versions.edit.unregister_progress.title",
    success: "home.steps.title.unregistered",
    incomplete: "home.steps.title.unregister_failed",
    failed: "home.steps.title.unregister_failed",
  },
  launch: {
    working: "launcherpage.mclaunch.loading.title",
    success: "home.steps.title.launched",
    incomplete: "launcherpage.launch.failed.title",
    failed: "launcherpage.launch.failed.title",
  },
};

const SUMMARIES: Record<LaunchFlow["intent"], string> = {
  check: "home.steps.summary.ready",
  register: "launcherpage.register.success.body",
  unregister: "home.steps.summary.unregistered",
  launch: "home.steps.summary.launched",
};

const MODAL_TYPES: Record<LaunchFlowPhase, ModalType> = {
  working: "primary",
  success: "success",
  incomplete: "warning",
  failed: "error",
};

const formatTime = (at: number) =>
  new Date(at).toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });

const StepIcon: React.FC<{ step: LaunchStep; isCurrent: boolean }> = ({
  step,
  isCurrent,
}) => {
  const base =
    "flex size-7 shrink-0 items-center justify-center rounded-full border";
  switch (step.status) {
    case "ready":
    case "done":
      return (
        <span className={cn(base, "border-brand-500/30 bg-brand-500/10 text-brand-600 dark:text-brand-400")}>
          <FiCheck aria-hidden="true" className="size-4" />
        </span>
      );
    case "failed":
      return (
        <span className={cn(base, "border-rose-500/30 bg-rose-500/10 text-rose-600 dark:text-rose-400")}>
          <FiX aria-hidden="true" className="size-4" />
        </span>
      );
    case "skipped":
      return (
        <span className={cn(base, "border-border bg-surface-secondary text-muted")}>
          <FiMinus aria-hidden="true" className="size-4" />
        </span>
      );
    case "missing":
    case "waiting":
      return (
        <span className={cn(base, "border-amber-500/30 bg-amber-500/10 text-amber-600 dark:text-amber-400")}>
          {step.status === "waiting" ? (
            <FiClock aria-hidden="true" className="size-4" />
          ) : (
            <FiAlertTriangle aria-hidden="true" className="size-4" />
          )}
        </span>
      );
    default:
      return (
        <span className={cn(base, "border-accent/30 bg-accent/10 text-accent")}>
          {isCurrent || step.status === "running" ? (
            <Spinner size="sm" color="current" />
          ) : (
            <FiClock aria-hidden="true" className="size-4" />
          )}
        </span>
      );
  }
};

export type LaunchStepsDialogProps = {
  flow: LaunchFlow | null;
  /** An instance is selected, so the flow can end by launching it. */
  canLaunch: boolean;
  /** Label of the primary action ("Launch", or "Register" for UWP). */
  launchLabel: string;
  isUWP: boolean;
  onClose: () => void;
  onLaunch: () => void;
  onForceLaunch: () => void;
  onRetry: (id: LaunchStepId) => void;
  onInstall: (id: "vcRuntime" | "gameInput") => void;
  onRecheck: (id: DependencyStepId) => void;
  onSkip: (id: DependencyStepId) => void;
  onIgnoreGamingServices: () => void;
  onOpenStore: () => void;
};

/**
 * One dialog for everything between pressing Launch and the game window:
 * prerequisites, registration and the launch. Finished steps stay listed with
 * their log, and the dialog only closes when the user closes it.
 */
export const LaunchStepsDialog: React.FC<LaunchStepsDialogProps> = ({
  flow,
  canLaunch,
  launchLabel,
  isUWP,
  onClose,
  onLaunch,
  onForceLaunch,
  onRetry,
  onInstall,
  onRecheck,
  onSkip,
  onIgnoreGamingServices,
  onOpenStore,
}) => {
  const { t } = useTranslation();
  const phase = getLaunchFlowPhase(flow) ?? "working";
  const current = getCurrentLaunchStep(flow);
  const steps = flow?.steps ?? [];
  const intent = flow?.intent ?? "check";
  const finished = phase !== "working";
  const failedSteps = steps.filter((step) => step.status === "failed");
  const listRef = React.useRef<HTMLOListElement | null>(null);

  const stepLabel = (id: LaunchStepId) => String(t(`home.steps.step.${id}`));
  const isDependencyStep =
    current?.id === "vcRuntime" ||
    current?.id === "gameInput" ||
    current?.id === "gamingServices" ||
    current?.id === "migration";
  const titleKey =
    !finished && isDependencyStep
      ? TITLES.check.working
      : TITLES[intent][phase];

  // Keep the newest log line in view as steps append to it.
  const lineCount = steps.reduce((sum, step) => sum + step.log.length, 0);
  React.useEffect(() => {
    const list = listRef.current;
    const scroller = list?.closest(".modal__body") ?? list?.parentElement;
    const activeStep = list?.querySelector('[data-current="true"]');
    if (activeStep && "scrollIntoView" in activeStep) {
      (activeStep as HTMLElement).scrollIntoView({ block: "nearest" });
    } else if (finished && scroller) {
      scroller.scrollTop = scroller.scrollHeight;
    }
  }, [lineCount, current?.id, finished]);

  const copyDetails = async () => {
    const text = [
      String(t(titleKey)),
      ...steps.flatMap((step) => [
        `${stepLabel(step.id)} — ${String(t(`home.steps.status.${step.status}`))}${step.error ? ` (${step.error})` : ""}`,
        ...step.log.map(
          (line) => `  ${formatTime(line.at)} [${line.level}] ${line.message}`,
        ),
      ]),
    ].join("\n");
    try {
      await navigator.clipboard.writeText(text);
      notify({
        variant: "success",
        source: "launcher",
        title: t("home.steps.copied"),
      });
    } catch {
      notify({
        variant: "danger",
        source: "launcher",
        title: t("home.steps.copy_failed"),
      });
    }
  };

  const actions: React.ReactNode[] = [];
  if (!finished && current) {
    if (current.status === "checking") {
      // A check that hangs must not trap the user in the dialog.
      actions.push(
        <ModalAction key="later" variant="secondary" onPress={() => onSkip(current.id as DependencyStepId)}>
          {t("home.steps.later")}
        </ModalAction>,
      );
    } else if (current.id === "vcRuntime" || current.id === "gameInput") {
      if (current.status === "missing") {
        actions.push(
          <ModalAction key="later" variant="secondary" onPress={() => onSkip(current.id as DependencyStepId)}>
            {t("home.steps.later")}
          </ModalAction>,
        );
        if (current.attempted) {
          actions.push(
            <ModalAction key="recheck" variant="secondary" onPress={() => onRecheck(current.id as DependencyStepId)}>
              {t("launcherpage.install_confirm.done_and_check")}
            </ModalAction>,
          );
        }
        actions.push(
          <ModalAction key="install" variant="primary" onPress={() => onInstall(current.id as "vcRuntime" | "gameInput")}>
            {current.attempted
              ? t("common.retry")
              : t(
                  current.id === "vcRuntime"
                    ? "launcherpage.vcruntime.missing.install_now"
                    : "launcherpage.gameinput.missing.install_now",
                )}
          </ModalAction>,
        );
      } else if (current.status === "running") {
        // The installer can finish without reporting back; let the user confirm.
        actions.push(
          <ModalAction key="recheck" variant="secondary" onPress={() => onRecheck(current.id as DependencyStepId)}>
            {t("launcherpage.install_confirm.done_and_check")}
          </ModalAction>,
        );
      }
    } else if (current.id === "gamingServices") {
      if (current.status === "missing") {
        actions.push(
          <ModalAction key="ignore" variant="secondary" onPress={onIgnoreGamingServices}>
            {t("launcherpage.gs.missing.ignore_forever")}
          </ModalAction>,
          <ModalAction key="later" variant="secondary" onPress={() => onSkip("gamingServices")}>
            {t("home.steps.later")}
          </ModalAction>,
          <ModalAction key="store" variant="primary" onPress={onOpenStore}>
            {t("launcherpage.gs.missing.open_store")}
          </ModalAction>,
        );
      } else if (current.status === "waiting") {
        actions.push(
          <ModalAction key="later" variant="secondary" onPress={() => onSkip("gamingServices")}>
            {t("home.steps.later")}
          </ModalAction>,
          <ModalAction key="store" variant="secondary" onPress={onOpenStore}>
            {t("home.steps.open_store_again")}
          </ModalAction>,
          <ModalAction key="recheck" variant="primary" onPress={() => onRecheck("gamingServices")}>
            {t("launcherpage.install_confirm.done_and_check")}
          </ModalAction>,
        );
      }
    } else if (current.id === "launch" && current.status === "running") {
      // Hiding the dialog lets the launch continue; failures reopen it.
      actions.push(
        <ModalAction key="close" variant="secondary" onPress={onClose}>
          {t("common.close")}
        </ModalAction>,
      );
    }
  }
  const proceedToLaunch =
    finished &&
    phase !== "failed" &&
    (intent === "check" || intent === "register") &&
    canLaunch;
  if (finished) {
    if (phase === "failed" || phase === "incomplete") {
      actions.push(
        <ModalAction key="copy" variant="secondary" onPress={() => void copyDetails()}>
          {t("home.steps.copy")}
        </ModalAction>,
      );
    }
    const launchFailure = failedSteps.find((step) => step.id === "launch");
    if (launchFailure?.error === "ERR_GAME_ALREADY_RUNNING") {
      actions.push(
        <ModalAction key="force" variant="secondary" isDisabled={isUWP} onPress={onForceLaunch}>
          {t("launcherpage.launch.force_run_button")}
        </ModalAction>,
      );
    }
    const retryable = failedSteps.find((step) => RETRYABLE_STEPS.has(step.id));
    actions.push(
      <ModalAction
        key="close"
        variant={proceedToLaunch || retryable ? "secondary" : "primary"}
        onPress={onClose}
      >
        {t("common.close")}
      </ModalAction>,
    );
    if (retryable) {
      actions.push(
        <ModalAction key="retry" variant="primary" onPress={() => onRetry(retryable.id)}>
          {retryable.id === "launch" ? launchLabel : t("common.retry")}
        </ModalAction>,
      );
    } else if (proceedToLaunch) {
      actions.push(
        <ModalAction key="launch" variant="primary" onPress={onLaunch}>
          {launchLabel}
        </ModalAction>,
      );
    }
  }

  const summaryTone =
    phase === "failed" ? "danger" : phase === "incomplete" ? "warning" : "info";
  const summaryText =
    phase === "success"
      ? t(SUMMARIES[intent])
      : phase === "incomplete"
        ? t("home.steps.summary.incomplete")
        : failedSteps.some((step) => step.id === "launch")
          ? t("launcherpage.launch.failed.content")
          : t("home.steps.summary.failed");

  return (
    <UnifiedModal
      isOpen={Boolean(flow?.open)}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      isDismissable={finished}
      isKeyboardDismissDisabled={!finished}
      type={MODAL_TYPES[phase]}
      title={t(titleKey)}
      contentKey="launch-steps"
      showConfirmButton={false}
      footer={actions.length > 0 ? <>{actions}</> : undefined}
    >
      <ol
        ref={listRef}
        aria-label={String(t("home.steps.list_label"))}
        className="flex min-w-0 flex-col gap-2"
      >
        {steps.map((step) => {
          const isCurrent = !finished && step.id === current?.id;
          const description = isCurrent
            ? STEP_DESCRIPTIONS[step.id]?.[step.status]
            : undefined;
          const errorText =
            step.status === "failed" ? resolveLaunchStepError(step, t) : "";
          // Finished steps keep their result line; the full log stays in
          // Activity and in the copied details.
          const visibleLog = isCurrent ? step.log : step.log.slice(-1);
          return (
            <li
              key={step.id}
              data-step={step.id}
              data-status={step.status}
              data-current={isCurrent ? "true" : undefined}
              aria-current={isCurrent ? "step" : undefined}
              className={cn(
                "flex min-w-0 items-start gap-3 rounded-xl border p-3 transition-colors",
                isCurrent
                  ? "border-accent/40 bg-accent/5"
                  : "border-border/60 bg-surface-secondary/30",
              )}
            >
              <StepIcon step={step} isCurrent={isCurrent} />
              <div className="flex min-w-0 flex-1 flex-col gap-1.5">
                <div className="flex min-w-0 flex-wrap items-baseline justify-between gap-x-3 gap-y-0.5">
                  <span className="min-w-0 font-semibold text-foreground [overflow-wrap:anywhere]">
                    {stepLabel(step.id)}
                  </span>
                  <span className="shrink-0 text-xs font-medium text-muted">
                    {t(`home.steps.status.${step.status}`)}
                  </span>
                </div>
                {description && <ModalDescription>{t(description)}</ModalDescription>}
                {isCurrent && step.status === "running" && (
                  <ProgressBar
                    aria-label={stepLabel(step.id)}
                    value={step.progress}
                    isIndeterminate={step.progress === undefined}
                    size="sm"
                    color="accent"
                    className="w-full"
                  >
                    <ProgressBar.Track>
                      <ProgressBar.Fill />
                    </ProgressBar.Track>
                  </ProgressBar>
                )}
                {isCurrent && step.detail && (
                  <span className="text-xs tabular-nums text-muted">
                    {step.detail}
                  </span>
                )}
                {errorText && (
                  <div className="flex min-w-0 flex-wrap items-center gap-2">
                    <span className="min-w-0 text-sm text-rose-700 dark:text-rose-300 [overflow-wrap:anywhere]">
                      {errorText}
                    </span>
                    {step.error && step.error !== errorText && (
                      <Chip size="sm" variant="soft" color="default" className="max-w-full font-mono">
                        <Chip.Label className="truncate">{step.error}</Chip.Label>
                      </Chip>
                    )}
                  </div>
                )}
                {visibleLog.length > 0 && (
                  <ul className="flex min-w-0 flex-col gap-0.5 text-xs leading-5 text-muted">
                    {visibleLog.map((line, index) => (
                      <li
                        key={`${line.at}-${index}`}
                        className={cn(
                          "flex min-w-0 gap-2",
                          line.level === "error" && "text-rose-700 dark:text-rose-300",
                          line.level === "warning" && "text-amber-700 dark:text-amber-300",
                        )}
                      >
                        <time className="shrink-0 tabular-nums opacity-70">
                          {formatTime(line.at)}
                        </time>
                        <span className="min-w-0 [overflow-wrap:anywhere]">
                          {line.message}
                        </span>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            </li>
          );
        })}
      </ol>
      {finished && (
        <ModalNotice tone={summaryTone} role="status">
          {summaryText}
        </ModalNotice>
      )}
    </UnifiedModal>
  );
};
