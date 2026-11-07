import React from "react";
import { Button, Checkbox, Label, ListBox, Select } from "@heroui/react";
import { useTranslation } from "react-i18next";
import {
  FaCheck,
  FaExchangeAlt,
  FaForward,
  FaSpinner,
  FaTimes,
} from "react-icons/fa";
import { TransferPackToVersion } from "bindings/github.com/liteldev/LeviLauncher/internal/app/contentservice";
import {
  GetVersionLogoDataUrl,
  ListVersionMetas,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/versionservice";
import type { ContentRoots } from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import {
  ModalDescription,
  ModalNotice,
  ModalProgress,
} from "@/components/ModalPrimitives";
import { UnifiedModal, type ModalType } from "@/components/UnifiedModal";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import {
  appendTaskLog,
  finishActivityTask,
  notify,
  setTaskProgress,
  startActivityTask,
} from "@/utils/activity";
import { cn } from "@/utils/cn";
import { isContentTransferTarget } from "@/utils/content";
import { resolveImportError } from "@/utils/importError";
import { compareVersions } from "@/utils/version";

type TransferTarget = {
  name: string;
  gameVersion: string;
  icon?: string;
};

type TransferEntry = {
  label: string;
  status: "running" | "ok" | "overwritten" | "skipped" | "failed";
  err?: string;
};

type Phase = "closed" | "pick" | "running" | "done";

const isDuplicate = (err: string) =>
  err === "ERR_DUPLICATE_FOLDER" || err === "ERR_DUPLICATE_UUID";

/**
 * Transfer selected packs to another instance in one dialog: pick the target,
 * watch a per-item log grow (duplicates are resolved inline), then read the
 * results until the dialog is closed. The run is mirrored to the activity log.
 */
export function usePackTransfer({
  sourceName,
  roots,
  getSelectedPaths,
  nameOf,
  onTransferred,
}: {
  sourceName: string;
  roots: ContentRoots;
  getSelectedPaths: () => string[];
  nameOf: (path: string) => string;
  onTransferred: () => void | Promise<unknown>;
}) {
  const { t } = useTranslation();
  const [phase, setPhase] = React.useState<Phase>("closed");
  const [targets, setTargets] = React.useState<TransferTarget[]>([]);
  const [target, setTarget] = React.useState("");
  const [entries, setEntries] = React.useState<TransferEntry[]>([]);
  const [total, setTotal] = React.useState(0);
  const [duplicate, setDuplicate] = React.useState("");
  const [applyToRest, setApplyToRestState] = React.useState(false);
  const applyToRestRef = React.useRef(false);
  const setApplyToRest = (value: boolean) => {
    applyToRestRef.current = value;
    setApplyToRestState(value);
  };
  const duplicateResolve = React.useRef<((overwrite: boolean) => void) | null>(null);
  const loadingTargets = React.useRef(false);
  const running = React.useRef(false);
  const unmounted = React.useRef(false);
  // Callbacks change every render; read the latest ones without re-creating run.
  const latest = React.useRef({ getSelectedPaths, nameOf, onTransferred });
  latest.current = { getSelectedPaths, nameOf, onTransferred };

  // Leaving the page mid-run must not leave the loop waiting on a prompt:
  // the open prompt and any later duplicate are skipped.
  React.useEffect(() => {
    unmounted.current = false;
    return () => {
      unmounted.current = true;
      duplicateResolve.current?.(false);
    };
  }, []);

  const open = React.useCallback(async () => {
    if (!sourceName || phase === "running" || loadingTargets.current) return;
    loadingTargets.current = true;
    try {
      const metas = await ListVersionMetas();
      const list: TransferTarget[] = (Array.isArray(metas) ? metas : [])
        .filter(
          (m: any) =>
            m?.name &&
            m.name !== sourceName &&
            isContentTransferTarget(roots, m),
        )
        .sort((a: any, b: any) => {
          const byVersion = compareVersions(
            String(b.gameVersion || "0"),
            String(a.gameVersion || "0"),
          );
          return byVersion || String(a.name).localeCompare(String(b.name));
        })
        .map((m: any) => ({
          name: String(m.name),
          gameVersion: String(m.gameVersion || ""),
        }));
      await Promise.all(
        list.map(async (item) => {
          try {
            const icon = await GetVersionLogoDataUrl(item.name);
            if (icon) item.icon = icon;
          } catch {}
        }),
      );
      setTargets(list);
      setTarget((current) =>
        list.some((item) => item.name === current) ? current : list[0]?.name || "",
      );
      setEntries([]);
      setPhase("pick");
    } catch (e) {
      notify({
        title: t("content_sub.targets_load_failed"),
        description: String(e),
        variant: "danger",
        source: "content",
      });
    } finally {
      loadingTargets.current = false;
    }
  }, [phase, roots, sourceName, t]);

  const run = React.useCallback(async () => {
    if (running.current) return;
    const { getSelectedPaths, nameOf, onTransferred } = latest.current;
    const paths = getSelectedPaths().filter(Boolean);
    const targetName = target;
    if (!sourceName || !targetName || paths.length === 0) return;

    running.current = true;
    setPhase("running");
    setEntries([]);
    setTotal(paths.length);
    setApplyToRest(false);
    let rememberedChoice: boolean | null = null;
    const taskId = startActivityTask({
      title: t("contentpage.transfer_resources_title"),
      description: `${sourceName} → ${targetName}`,
      source: "content",
    });
    const counts = { ok: 0, failed: 0, skipped: 0 };
    const update = (index: number, patch: Partial<TransferEntry>) =>
      setEntries((list) =>
        list.map((entry, i) => (i === index ? { ...entry, ...patch } : entry)),
      );

    for (let index = 0; index < paths.length; index++) {
      const path = paths[index];
      const label = nameOf(path);
      setEntries((list) => [...list, { label, status: "running" }]);
      let status: TransferEntry["status"] = "ok";
      let err = "";
      try {
        err = String((await TransferPackToVersion(sourceName, path, targetName, false)) || "");
        if (err && isDuplicate(err)) {
          let overwrite: boolean | null = unmounted.current ? false : rememberedChoice;
          if (overwrite === null) {
            setDuplicate(label);
            overwrite = await new Promise<boolean>((resolve) => {
              duplicateResolve.current = resolve;
            });
            duplicateResolve.current = null;
            setDuplicate("");
            if (applyToRestRef.current) rememberedChoice = overwrite;
          }
          if (overwrite) {
            err = String((await TransferPackToVersion(sourceName, path, targetName, true)) || "");
            status = err ? "failed" : "overwritten";
          } else {
            err = "";
            status = "skipped";
          }
        } else if (err) {
          status = "failed";
        }
      } catch (e) {
        err = String(e);
        status = "failed";
      }

      const message = err ? resolveImportError(err, t) : "";
      update(index, { status, err: message });
      if (status === "failed") counts.failed++;
      else if (status === "skipped") counts.skipped++;
      else counts.ok++;
      appendTaskLog(
        taskId,
        `${label}: ${t(`content_sub.item_${status}`)}${message ? ` — ${message}` : ""}`,
        status === "failed" ? "error" : status === "skipped" ? "warning" : "success",
      );
      setTaskProgress(taskId, ((index + 1) / paths.length) * 100);
    }

    const summary = t("content_sub.transfer_summary", {
      success: counts.ok,
      failed: counts.failed,
      skipped: counts.skipped,
    });
    finishActivityTask(
      taskId,
      counts.failed > 0 && counts.ok === 0 ? "error" : "success",
      summary,
    );
    running.current = false;
    setPhase("done");
    if (counts.ok > 0) await onTransferred();
  }, [sourceName, t, target]);

  const close = React.useCallback(() => {
    if (phase === "running") return;
    setPhase("closed");
  }, [phase]);

  const resolveDuplicate = (overwrite: boolean) => duplicateResolve.current?.(overwrite);

  const done = entries.filter((entry) => entry.status !== "running").length;
  const failed = entries.filter((entry) => entry.status === "failed").length;
  const succeeded = entries.filter(
    (entry) => entry.status === "ok" || entry.status === "overwritten",
  ).length;
  const skipped = entries.filter((entry) => entry.status === "skipped").length;

  let type: ModalType = "primary";
  let title = t("contentpage.transfer_resources_title");
  if (phase === "running") title = t("contentpage.transfer_progress_title");
  if (phase === "done") {
    if (failed && !succeeded) {
      type = "error";
      title = t("content_sub.transfer_failed_title");
    } else if (failed) {
      type = "warning";
      title = t("content_sub.transfer_partial_title");
    } else {
      type = "success";
      title = t("content_sub.transfer_done_title");
    }
  }

  const statusIcon = (status: TransferEntry["status"]) => {
    switch (status) {
      case "running":
        return <FaSpinner className="animate-spin text-muted" />;
      case "failed":
        return <FaTimes className="text-rose-500" />;
      case "skipped":
        return <FaForward className="text-amber-500" />;
      default:
        return <FaCheck className="text-brand-500" />;
    }
  };

  const modal = (
    <UnifiedModal
      size="wide"
      isOpen={phase !== "closed"}
      onOpenChange={(isOpen) => {
        if (!isOpen) close();
      }}
      isDismissable={phase !== "running"}
      isKeyboardDismissDisabled={phase === "running"}
      type={type}
      icon={phase === "done" ? undefined : <FaExchangeAlt />}
      title={title}
      contentKey={phase === "pick" ? "pick" : "log"}
      footer={
        phase === "pick" ? undefined : phase === "running" ? null : (
          <Button variant="primary" className="rounded-full" onPress={close}>
            {t("common.close")}
          </Button>
        )
      }
      showConfirmButton={phase === "pick"}
      showCancelButton={phase === "pick"}
      confirmText={t("content_sub.transfer_start")}
      cancelText={t("common.cancel")}
      onCancel={close}
      onConfirm={() => void run()}
      confirmButtonProps={{ isDisabled: !target }}
    >
      {phase === "pick" ? (
        targets.length > 0 ? (
          <>
            <ModalDescription>
              {t("contentpage.transfer_resources_body_simple")}
            </ModalDescription>
            <Select
              placeholder={t("contentpage.transfer_target_placeholder")}
              value={target || null}
              onChange={(key) => setTarget(String(key ?? ""))}
            >
              <Label>{t("mirror.target")}</Label>
              <Select.Trigger className={COMPONENT_STYLES.select.trigger}>
                <Select.Value />
                <Select.Indicator />
              </Select.Trigger>
              <Select.Popover className={COMPONENT_STYLES.select.popoverContent}>
                <ListBox items={targets} className={COMPONENT_STYLES.select.listbox}>
                  {(item) => (
                    <ListBox.Item key={item.name} id={item.name} textValue={item.name}>
                      <div className="flex min-w-0 items-center gap-2">
                        <div className="size-8 shrink-0 overflow-hidden rounded bg-surface-tertiary">
                          {item.icon && (
                            <img src={item.icon} alt="" className="size-full object-cover" />
                          )}
                        </div>
                        <div className="flex min-w-0 flex-col">
                          <span className="truncate text-sm">{item.name}</span>
                          <span className="truncate text-xs text-muted">{item.gameVersion}</span>
                        </div>
                      </div>
                      <ListBox.ItemIndicator />
                    </ListBox.Item>
                  )}
                </ListBox>
              </Select.Popover>
            </Select>
          </>
        ) : (
          <div className="flex flex-col items-center justify-center py-6 text-muted dark:text-zinc-500">
            <FaExchangeAlt className="mb-3 text-4xl opacity-20" />
            <ModalDescription>{t("contentpage.transfer_no_targets")}</ModalDescription>
          </div>
        )
      ) : (
        <>
          <ModalProgress
            label={t("contentpage.transfer_progress_title")}
            value={total ? Math.round((done / total) * 100) : 0}
            detail={
              phase === "done"
                ? t("content_sub.transfer_summary", {
                    success: succeeded,
                    failed,
                    skipped,
                  })
                : t("content_sub.transfer_running", { done, total, target })
            }
          />
          {duplicate && (
            <ModalNotice tone="warning" role="alertdialog">
              <p>{t("content_sub.duplicate_prompt", { name: duplicate })}</p>
              <div className="mt-3 flex flex-wrap items-center gap-2">
                <Button size="sm" variant="primary" onPress={() => resolveDuplicate(true)}>
                  {t("content_sub.overwrite")}
                </Button>
                <Button size="sm" variant="secondary" onPress={() => resolveDuplicate(false)}>
                  {t("content_sub.skip")}
                </Button>
                {total > 1 && (
                  <Checkbox isSelected={applyToRest} onChange={setApplyToRest}>
                    <Checkbox.Content>
                      <Checkbox.Control>
                        <Checkbox.Indicator />
                      </Checkbox.Control>
                      <Label className="text-xs">{t("content_sub.apply_to_rest")}</Label>
                    </Checkbox.Content>
                  </Checkbox>
                )}
              </div>
            </ModalNotice>
          )}
          <ol
            aria-label={t("content_sub.transfer_log")}
            aria-live="polite"
            className="launcher-modal-panel max-h-[min(40vh,20rem)] min-w-0 overflow-y-auto rounded-xl border border-border/70 bg-surface-secondary/50 p-2 text-sm custom-scrollbar"
          >
            {entries.map((entry, index) => (
              <li
                key={index}
                className={cn(
                  "flex min-w-0 items-start gap-2 rounded-lg px-2 py-1.5",
                  entry.status === "failed" && "bg-rose-500/5",
                )}
              >
                <span className="mt-1 shrink-0" aria-hidden="true">
                  {statusIcon(entry.status)}
                </span>
                <div className="min-w-0 flex-1">
                  <div className="flex min-w-0 flex-wrap items-baseline justify-between gap-x-2">
                    <span className="font-mono [overflow-wrap:anywhere]">{entry.label}</span>
                    <span className="shrink-0 text-xs text-muted">
                      {t(`content_sub.item_${entry.status}`)}
                    </span>
                  </div>
                  {entry.err && (
                    <p className="text-xs text-rose-600 select-text [overflow-wrap:anywhere] dark:text-rose-300">
                      {entry.err}
                    </p>
                  )}
                </div>
              </li>
            ))}
          </ol>
        </>
      )}
    </UnifiedModal>
  );

  return { open, transferring: phase === "running", modal };
}
