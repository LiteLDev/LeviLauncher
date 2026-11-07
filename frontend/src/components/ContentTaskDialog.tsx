import React from "react";
import { Button, Checkbox, ProgressBar, Spinner } from "@heroui/react";
import { useTranslation } from "react-i18next";
import {
  FiAlertTriangle,
  FiCheckCircle,
  FiMinusCircle,
  FiUploadCloud,
  FiXCircle,
} from "react-icons/fi";
import { FaExchangeAlt } from "react-icons/fa";
import { ModalAction, ModalDescription, ModalNotice } from "@/components/ModalPrimitives";
import { UnifiedModal, type ModalType } from "@/components/UnifiedModal";
import {
  appendTaskLog,
  finishActivityTask,
  setTaskProgress,
  startActivityTask,
} from "@/utils/activity";
import { resolveImportError } from "@/utils/importError";
import { cn } from "@/utils/cn";

export type ContentTaskKind = "import" | "transfer";
export type ContentTaskItemStatus = "running" | "success" | "failed" | "skipped";
export type ContentTaskItem = {
  id: number;
  name: string;
  status: ContentTaskItemStatus;
  err?: string;
  overwritten?: boolean;
};
type Phase = "idle" | "player" | "running" | "duplicate" | "done";
export type ContentTaskPlayerOption = { id: string; label: string };

/**
 * One dialog for a whole content import or transfer: optional player choice up
 * front, a per-item log that only grows, inline duplicate decisions and the
 * final result. The log is mirrored to the Activity center.
 */
export function useContentTask(kind: ContentTaskKind) {
  const { t } = useTranslation();
  const [phase, setPhase] = React.useState<Phase>("idle");
  const [items, setItems] = React.useState<ContentTaskItem[]>([]);
  const [total, setTotalState] = React.useState(0);
  const [duplicateName, setDuplicateName] = React.useState("");
  const [applyAll, setApplyAll] = React.useState(false);
  const itemsRef = React.useRef<ContentTaskItem[]>([]);
  const totalRef = React.useRef(0);
  const nextIdRef = React.useRef(0);
  const activityRef = React.useRef<string | null>(null);
  const rememberedRef = React.useRef<boolean | null>(null);
  const applyAllRef = React.useRef(false);
  const playerResolveRef = React.useRef<((player: string) => void) | null>(null);
  const dupResolveRef = React.useRef<((overwrite: boolean) => void) | null>(null);

  const taskTitle = t(kind === "import" ? "content_task.import_title" : "content_task.transfer_title");

  const commit = (next: ContentTaskItem[]) => {
    itemsRef.current = next;
    setItems(next);
  };

  const ensureActivity = () => {
    if (!activityRef.current) {
      activityRef.current = startActivityTask({ title: taskTitle, source: "content" });
    }
    return activityRef.current;
  };

  const open = React.useCallback((count = 0) => {
    commit([]);
    totalRef.current = count;
    setTotalState(count);
    activityRef.current = null;
    rememberedRef.current = null;
    setDuplicateName("");
    setPhase("running");
  }, []);

  const setTotal = (count: number) => {
    totalRef.current = count;
    setTotalState(count);
  };

  /** Ask for a player inside the dialog; resolves "" when the user cancels. */
  const askPlayer = () =>
    new Promise<string>((resolve) => {
      playerResolveRef.current = resolve;
      setPhase("player");
    });

  const resolvePlayer = (player: string) => {
    const resolve = playerResolveRef.current;
    playerResolveRef.current = null;
    if (player) setPhase("running");
    else setPhase("idle");
    resolve?.(player);
  };

  const begin = (name: string) => {
    ensureActivity();
    const id = ++nextIdRef.current;
    commit([...itemsRef.current, { id, name, status: "running" }]);
    return id;
  };

  const end = (
    id: number,
    status: Exclude<ContentTaskItemStatus, "running">,
    err?: string,
    overwritten?: boolean,
  ) => {
    const activity = ensureActivity();
    const item = itemsRef.current.find((it) => it.id === id);
    const next = itemsRef.current.map((it) =>
      it.id === id ? { ...it, status, err, overwritten } : it,
    );
    commit(next);
    const name = item?.name || "";
    if (status === "success") {
      appendTaskLog(activity, `${name}: ${t(overwritten ? "content_task.status_overwritten" : "content_task.status_success")}`, "success");
    } else if (status === "skipped") {
      appendTaskLog(activity, `${name}: ${t("content_task.status_skipped")}`, "warning");
    } else {
      appendTaskLog(activity, `${name}: ${resolveImportError(String(err || ""), t as any)}`, "error");
    }
    const finished = next.filter((it) => it.status !== "running").length;
    const count = Math.max(totalRef.current, next.length);
    if (count > 0) setTaskProgress(activity, (finished / count) * 100);
  };

  /** Ask whether to overwrite a duplicate, unless "apply to all" was chosen. */
  const askOverwrite = (name: string) => {
    if (rememberedRef.current !== null) return Promise.resolve(rememberedRef.current);
    return new Promise<boolean>((resolve) => {
      dupResolveRef.current = resolve;
      applyAllRef.current = false;
      setApplyAll(false);
      setDuplicateName(name);
      setPhase("duplicate");
    });
  };

  const resolveDuplicate = (overwrite: boolean, dismissed = false) => {
    const resolve = dupResolveRef.current;
    if (!resolve) return;
    dupResolveRef.current = null;
    // Escape skips this item only; "apply to all" needs an explicit button.
    if (applyAllRef.current && !dismissed) rememberedRef.current = overwrite;
    setPhase("running");
    resolve(overwrite);
  };

  const finish = () => {
    const all = itemsRef.current;
    const ok = all.filter((it) => it.status === "success").length;
    const failed = all.filter((it) => it.status === "failed").length;
    const skipped = all.filter((it) => it.status === "skipped").length;
    if (activityRef.current) {
      finishActivityTask(
        activityRef.current,
        failed > 0 && ok === 0 ? "error" : "success",
        t("content_task.summary", { ok, failed, skipped }),
      );
    }
    setPhase("done");
  };

  /** Record an unexpected error as a failed log line and show the result. */
  const fail = (err: unknown) => {
    const id = begin(t("common.error"));
    end(id, "failed", String((err as any)?.message || err || "ERROR"));
    finish();
  };

  const close = () => {
    playerResolveRef.current?.("");
    playerResolveRef.current = null;
    dupResolveRef.current?.(false);
    dupResolveRef.current = null;
    setPhase("idle");
  };

  return {
    kind,
    phase,
    items,
    total,
    duplicateName,
    applyAll,
    setApplyAll: (value: boolean) => {
      applyAllRef.current = value;
      setApplyAll(value);
    },
    isBusy: phase !== "idle" && phase !== "done",
    isOpen: phase !== "idle",
    open,
    setTotal,
    askPlayer,
    resolvePlayer,
    begin,
    end,
    askOverwrite,
    resolveDuplicate,
    finish,
    fail,
    close,
  };
}

export type ContentTask = ReturnType<typeof useContentTask>;

const STATUS_ICON: Record<ContentTaskItemStatus, React.ReactNode> = {
  running: <Spinner size="sm" color="current" className="size-4" />,
  success: <FiCheckCircle aria-hidden="true" className="size-4 text-brand-600 dark:text-brand-400" />,
  failed: <FiXCircle aria-hidden="true" className="size-4 text-rose-600 dark:text-rose-300" />,
  skipped: <FiMinusCircle aria-hidden="true" className="size-4 text-muted" />,
};

export const ContentTaskDialog: React.FC<{
  task: ContentTask;
  players?: ContentTaskPlayerOption[];
}> = ({ task, players = [] }) => {
  const { t } = useTranslation();
  const logRef = React.useRef<HTMLOListElement>(null);
  const { phase, items } = task;
  const ok = items.filter((it) => it.status === "success").length;
  const failed = items.filter((it) => it.status === "failed").length;
  const skipped = items.filter((it) => it.status === "skipped").length;
  const finished = items.filter((it) => it.status !== "running").length;
  const count = Math.max(task.total, items.length);

  React.useEffect(() => {
    const el = logRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [items.length, phase]);

  let type: ModalType = "primary";
  let title = t(task.kind === "import" ? "mods.importing_title" : "contentpage.transfer_progress_title");
  if (phase === "player") title = t("contentpage.select_player_title");
  if (phase === "duplicate") {
    type = "warning";
    title = t("mods.overwrite_modal_title");
  }
  if (phase === "done") {
    const isImport = task.kind === "import";
    if (failed > 0 && ok > 0) {
      type = "warning";
      title = t(isImport ? "mods.summary_title_partial" : "content_task.transfer_partial");
    } else if (failed > 0) {
      type = "error";
      title = t(isImport ? "mods.summary_failed" : "content_task.transfer_failed");
    } else {
      type = "success";
      title = t(isImport ? "mods.summary_title_done" : "content_task.transfer_done");
    }
  }

  const footer =
    phase === "player" ? (
      <ModalAction variant="secondary" onPress={() => task.resolvePlayer("")}>
        {t("common.cancel")}
      </ModalAction>
    ) : phase === "duplicate" ? (
      <>
        <ModalAction variant="secondary" onPress={() => task.resolveDuplicate(false)}>
          {t("content_task.skip")}
        </ModalAction>
        <ModalAction variant="primary" className="font-semibold" onPress={() => task.resolveDuplicate(true)}>
          {t("mods.overwrite_and_import")}
        </ModalAction>
      </>
    ) : phase === "done" ? (
      <ModalAction variant="primary" className="font-semibold" onPress={task.close}>
        {t("common.confirm")}
      </ModalAction>
    ) : null;

  return (
    <UnifiedModal
      isOpen={task.isOpen}
      size="wide"
      type={type}
      title={title}
      contentKey="content-task"
      icon={
        phase === "running"
          ? task.kind === "import"
            ? <FiUploadCloud />
            : <FaExchangeAlt />
          : undefined
      }
      isKeyboardDismissDisabled={phase === "running"}
      onOpenChange={(isOpen) => {
        if (isOpen) return;
        // Escape answers the current question; a running task can't be dismissed.
        if (phase === "player") task.resolvePlayer("");
        else if (phase === "duplicate") task.resolveDuplicate(false, true);
        else if (phase === "done") task.close();
      }}
      showConfirmButton={false}
      showCancelButton={false}
      footer={footer}
    >
      {phase === "player" ? (
        <div className="flex flex-col gap-4">
          <ModalDescription>{t("contentpage.select_player_for_import")}</ModalDescription>
          <div className="flex flex-col gap-2 max-h-[min(18rem,40vh)] overflow-y-auto custom-scrollbar p-1">
            {players.length ? (
              players.map((p) => (
                <Button
                  key={p.id}
                  onPress={() => task.resolvePlayer(p.id)}
                  variant="secondary"
                  className="w-full justify-start bg-surface-secondary text-foreground dark:text-zinc-200"
                >
                  {p.label}
                </Button>
              ))
            ) : (
              <div className="text-sm text-muted dark:text-zinc-400">{t("contentpage.no_players")}</div>
            )}
          </div>
        </div>
      ) : (
        <div className="flex min-w-0 flex-col gap-4">
          {phase === "running" && (
            <ModalDescription>
              {t(task.kind === "import" ? "mods.importing_body" : "contentpage.transfer_progress_body")}
            </ModalDescription>
          )}
          <div className="space-y-2">
            <ProgressBar
              aria-label={t("content_task.progress_label")}
              value={count > 0 ? (finished / count) * 100 : undefined}
              isIndeterminate={count === 0 && phase !== "done"}
              size="sm"
              color={phase === "done" && failed > 0 ? "danger" : "accent"}
              className="w-full"
            >
              <ProgressBar.Track><ProgressBar.Fill /></ProgressBar.Track>
            </ProgressBar>
            <div className="text-xs leading-5 text-muted tabular-nums" aria-live="polite">
              {t("content_task.progress", { done: finished, total: count })}
            </div>
          </div>
          {phase === "duplicate" && (
            <ModalNotice tone="warning">
              <div className="flex min-w-0 flex-col gap-2">
                <span>{t("content_task.overwrite_body")}</span>
                <span className="font-mono font-medium [overflow-wrap:anywhere]">{task.duplicateName}</span>
                <Checkbox isSelected={task.applyAll} onChange={task.setApplyAll} className="group">
                  <Checkbox.Content>
                    <Checkbox.Control>
                      <Checkbox.Indicator />
                    </Checkbox.Control>
                    <span className="text-sm">{t("content_task.apply_to_all")}</span>
                  </Checkbox.Content>
                </Checkbox>
              </div>
            </ModalNotice>
          )}
          {items.length > 0 ? (
            <ol
              ref={logRef}
              aria-label={t("content_task.log_label")}
              className="launcher-modal-panel flex max-h-[min(18rem,40vh)] min-w-0 flex-col gap-1 overflow-y-auto custom-scrollbar rounded-xl border border-border/70 bg-surface-secondary/50 p-2 text-sm"
            >
              {items.map((it) => (
                <li key={it.id} className="flex min-w-0 items-start gap-2 rounded-lg px-2 py-1.5">
                  <span className="mt-0.5 flex shrink-0 items-center">{STATUS_ICON[it.status]}</span>
                  <span className="flex min-w-0 flex-1 flex-col">
                    <span className="font-mono font-medium [overflow-wrap:anywhere]">{it.name}</span>
                    <span
                      className={cn(
                        "text-xs leading-5",
                        it.status === "failed" ? "text-rose-700 dark:text-rose-300" : "text-muted",
                      )}
                    >
                      {it.status === "running"
                        ? t("content_task.status_running")
                        : it.status === "failed"
                          ? resolveImportError(String(it.err || ""), t as any)
                          : it.status === "skipped"
                            ? t("content_task.status_skipped")
                            : t(it.overwritten ? "content_task.status_overwritten" : "content_task.status_success")}
                    </span>
                  </span>
                </li>
              ))}
            </ol>
          ) : phase === "done" ? (
            <ModalNotice tone="info">{t("content_task.nothing_done")}</ModalNotice>
          ) : null}
          {phase === "done" && failed > 0 && (
            <div className="flex items-center gap-2 text-sm text-rose-700 dark:text-rose-300">
              <FiAlertTriangle aria-hidden="true" className="size-4 shrink-0" />
              {t("content_task.summary", { ok, failed, skipped })}
            </div>
          )}
        </div>
      )}
    </UnifiedModal>
  );
};
