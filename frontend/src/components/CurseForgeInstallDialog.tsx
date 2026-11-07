import React from "react";
import { Label, ListBox, ProgressBar, Select, Spinner } from "@heroui/react";
import { useTranslation } from "react-i18next";
import { AnimatePresence, motion } from "framer-motion";
import { Events } from "@wailsio/runtime";
import {
  FiAlertTriangle,
  FiCheckCircle,
  FiMinusCircle,
  FiXCircle,
} from "react-icons/fi";
import { LuDownload } from "react-icons/lu";
import {
  GetCurseForgeModFiles,
  StartFileDownload,
  CancelFileDownload,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import {
  GetContentRoots,
  ImportMcpackPath,
  ImportMcpackPathWithPlayer,
  ImportMcaddonPath,
  ImportMcaddonPathWithPlayer,
  ImportMcworldPath,
  IsMcpackSkinPackPath,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/contentservice";
import { GetLocalUserGamertag } from "bindings/github.com/liteldev/LeviLauncher/internal/app/userservice";
import { File as ModFile } from "bindings/github.com/liteldev/LeviLauncher/internal/curseforge/client/types";
import { UnifiedModal } from "@/components/UnifiedModal";
import {
  ModalAction,
  ModalDescription,
  ModalNotice,
  MODAL_STYLES,
} from "@/components/ModalPrimitives";
import { useInstanceList } from "@/components/InstancePicker";
import { useCurrentVersion } from "@/utils/CurrentVersionContext";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import {
  getPlayerGamertagMap,
  listPlayers,
  resolvePlayerDisplayName,
} from "@/utils/content";
import { compareVersions } from "@/utils/version";
import {
  formatDateStr,
  formatFileSize,
  sortGameVersions,
} from "@/utils/formatting";
import {
  appendTaskLog,
  finishActivityTask,
  setTaskProgress,
  startActivityTask,
} from "@/utils/activity";
import { shouldDisableAnimations } from "@/hooks/useAnimations";
import { cn } from "@/utils/cn";

export type CurseForgeInstallTarget = {
  modId: number;
  modName: string;
  /** Preselected file; when omitted the best match for the instance is picked. */
  file?: ModFile | null;
};

type StepStatus = "active" | "done" | "error" | "skipped" | "waiting";
type Step = {
  id: string;
  label: string;
  status: StepStatus;
  detail?: string;
};
type Phase = "setup" | "running" | "done";
type PackageType = "mcworld" | "mcaddon" | "mcpack" | "skin_pack" | "unknown";

// Skin packs for newer games are imported without a player.
const SKIN_PACK_PLAYER_MAX_VERSION = "1.26.0";

const fileLabel = (file: ModFile) => file.displayName || file.fileName;

/** Newest release that lists the instance's game version, else the newest file. */
const pickDefaultFile = (files: ModFile[], gameVersion: string) => {
  const usable = files.filter((file) => file.downloadUrl);
  if (usable.length === 0) return undefined;
  const byDate = [...usable].sort(
    (a, b) =>
      new Date(b.fileDate || 0).getTime() - new Date(a.fileDate || 0).getTime(),
  );
  const minor = gameVersion.split(".").slice(0, 2).join(".");
  const matches = (file: ModFile) =>
    !!gameVersion &&
    (file.gameVersions || []).some(
      (v) => v === gameVersion || (!!minor && v.startsWith(minor)),
    );
  return (
    byDate.find((file) => matches(file) && file.releaseType === 1) ||
    byDate.find(matches) ||
    byDate.find((file) => file.releaseType === 1) ||
    byDate[0]
  );
};

const extensionType = (fileName: string): PackageType => {
  const lower = fileName.toLowerCase();
  if (lower.endsWith(".mcworld")) return "mcworld";
  if (lower.endsWith(".mcaddon")) return "mcaddon";
  if (lower.endsWith(".mcpack")) return "mcpack";
  return "unknown";
};

const isDuplicateError = (err: string) =>
  err === "ERR_DUPLICATE_FOLDER" || err === "ERR_DUPLICATE_UUID";

type Props = {
  target: CurseForgeInstallTarget | null;
  isOpen: boolean;
  onClose: () => void;
  /** Files the caller already loaded; skips fetching them again. */
  files?: ModFile[];
};

/**
 * One dialog for installing a CurseForge file: choose file, instance and
 * player up front, then download and import with every step kept on screen.
 */
export const CurseForgeInstallDialog: React.FC<Props> = ({
  target,
  isOpen,
  onClose,
  files: knownFiles,
}) => {
  const { t } = useTranslation();
  const { currentVersionName } = useCurrentVersion();
  const { instances, loading: instancesLoading } = useInstanceList();

  const [phase, setPhase] = React.useState<Phase>("setup");
  const [outcome, setOutcome] = React.useState<
    "success" | "kept" | "error" | null
  >(null);
  const [errorText, setErrorText] = React.useState("");
  const [steps, setSteps] = React.useState<Step[]>([]);
  const [downloadProgress, setDownloadProgress] = React.useState<{
    downloaded: number;
    total: number;
  } | null>(null);

  const [files, setFiles] = React.useState<ModFile[]>([]);
  const [filesState, setFilesState] = React.useState<
    "idle" | "loading" | "error"
  >("idle");
  const [fileId, setFileId] = React.useState<string>("");
  const [instanceName, setInstanceName] = React.useState("");
  const [isUwp, setIsUwp] = React.useState(false);
  const [players, setPlayers] = React.useState<string[]>([]);
  const [playersLoading, setPlayersLoading] = React.useState(false);
  const [gamertags, setGamertags] = React.useState<Record<string, string>>({});
  const [player, setPlayer] = React.useState("");
  const [duplicatePending, setDuplicatePending] = React.useState(false);

  const attemptRef = React.useRef(0);
  const unsubscribeRef = React.useRef<() => void>(() => {});
  const duplicateResolveRef = React.useRef<((overwrite: boolean) => void) | null>(null);
  const taskIdRef = React.useRef<string | null>(null);
  const downloadingRef = React.useRef(false);
  const lastLoggedPercentRef = React.useRef(-1);
  const cancelPendingRef = React.useRef<Promise<unknown>>(Promise.resolve());
  const userPickedFileRef = React.useRef(false);
  const userPickedPlayerRef = React.useRef(false);

  // Reset the form while rendering when a new target opens, so the previous
  // run's result never flashes under the new title.
  const [sessionTarget, setSessionTarget] =
    React.useState<CurseForgeInstallTarget | null>(null);
  if (isOpen && target && target !== sessionTarget) {
    setSessionTarget(target);
    setPhase("setup");
    setOutcome(null);
    setErrorText("");
    setSteps([]);
    setDownloadProgress(null);
    setDuplicatePending(false);
    const initial =
      knownFiles && knownFiles.length > 0
        ? knownFiles
        : target.file
          ? [target.file]
          : [];
    setFiles(initial);
    setFileId(target.file ? String(target.file.id) : "");
    setFilesState(knownFiles && knownFiles.length > 0 ? "idle" : "loading");
  }

  React.useEffect(() => {
    if (!isOpen || !target) return;
    userPickedFileRef.current = false;
    userPickedPlayerRef.current = false;
    if (knownFiles && knownFiles.length > 0) return;

    let cancelled = false;
    GetCurseForgeModFiles(target.modId)
      .then((result) => {
        if (cancelled) return;
        const list = (result?.data ?? []) as ModFile[];
        if (list.length > 0) setFiles(list);
        setFilesState("idle");
      })
      .catch(() => {
        if (cancelled) return;
        setFilesState(target.file ? "idle" : "error");
      });
    return () => {
      cancelled = true;
    };
    // knownFiles is read once per opened target.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen, target]);

  // Default the instance to the current one.
  React.useEffect(() => {
    if (!isOpen) return;
    setInstanceName((prev) => {
      if (prev && instances.some((item) => item.name === prev)) return prev;
      if (instances.some((item) => item.name === currentVersionName)) {
        return currentVersionName;
      }
      return instances[0]?.name ?? "";
    });
  }, [isOpen, instances, currentVersionName]);

  const instance = instances.find((item) => item.name === instanceName);

  // Pick the best file for the instance until the user picks one.
  React.useEffect(() => {
    if (!isOpen || userPickedFileRef.current || target?.file) return;
    const best = pickDefaultFile(files, instance?.gameVersion ?? "");
    setFileId(best ? String(best.id) : "");
  }, [isOpen, files, instance?.gameVersion, target?.file]);

  const file = files.find((item) => String(item.id) === fileId);
  const fileType = file ? extensionType(file.fileName) : "unknown";
  const skinPackNeedsPlayer =
    compareVersions(
      instance?.gameVersion || "0",
      SKIN_PACK_PLAYER_MAX_VERSION,
    ) <= 0;
  const asksPlayer =
    !isUwp &&
    (fileType === "mcworld" || (fileType === "mcpack" && skinPackNeedsPlayer));
  const playerRequired = asksPlayer && fileType === "mcworld";

  // Load players (and the package type) of the chosen instance.
  React.useEffect(() => {
    if (!isOpen || !instanceName) return;
    let cancelled = false;
    setPlayersLoading(true);
    (async () => {
      try {
        const roots = await GetContentRoots(instanceName);
        if (cancelled) return;
        const uwp = roots?.packageType === "uwp";
        setIsUwp(uwp);
        if (uwp || !roots?.usersRoot) {
          setPlayers([]);
          setGamertags({});
          setPlayer("");
          return;
        }
        const list = await listPlayers(roots.usersRoot);
        if (cancelled) return;
        setPlayers(list);
        setPlayer((prev) => (list.includes(prev) ? prev : list[0] ?? ""));
        try {
          const map = await getPlayerGamertagMap(roots.usersRoot);
          if (cancelled) return;
          setGamertags(map);
          const tag = await GetLocalUserGamertag();
          const own = tag ? list.find((p) => map[p] === tag) : undefined;
          if (!cancelled && own && !userPickedPlayerRef.current) setPlayer(own);
        } catch {}
      } catch {
        if (cancelled) return;
        setIsUwp(false);
        setPlayers([]);
      } finally {
        if (!cancelled) setPlayersLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [isOpen, instanceName]);

  const pushStep = (step: Step) => {
    setSteps((prev) => [...prev, step]);
    if (taskIdRef.current) appendTaskLog(taskIdRef.current, step.label);
  };

  const updateStep = (id: string, patch: Partial<Step>) => {
    setSteps((prev) =>
      prev.map((step) => (step.id === id ? { ...step, ...patch } : step)),
    );
    const taskId = taskIdRef.current;
    if (taskId && patch.detail) {
      appendTaskLog(
        taskId,
        patch.detail,
        patch.status === "error"
          ? "error"
          : patch.status === "done"
            ? "success"
            : "info",
      );
    }
  };

  const finish = (
    result: "success" | "kept" | "error" | "cancelled",
    summary: string,
  ) => {
    const taskId = taskIdRef.current;
    if (taskId) {
      finishActivityTask(
        taskId,
        result === "kept" ? "success" : result,
        summary,
      );
    }
    taskIdRef.current = null;
    if (result === "cancelled") return;
    setOutcome(result);
    if (result === "error") setErrorText(summary);
    setPhase("done");
  };

  /** Starts the download and resolves with the saved path. */
  const download = (selected: ModFile, attempt: number) =>
    new Promise<string>((resolve, reject) => {
      const isCurrent = () => attemptRef.current === attempt;
      const subscriptions: (() => void)[] = [];
      const off = () => subscriptions.splice(0).forEach((fn) => fn());
      unsubscribeRef.current = off;
      subscriptions.push(
        Events.On("file.download.progress", (event) => {
          if (!isCurrent()) return;
          const data = event.data || {};
          const progress = {
            downloaded: Number(data.Downloaded || 0),
            total: Number(data.Total || 0),
          };
          setDownloadProgress(progress);
          const taskId = taskIdRef.current;
          if (taskId && progress.total > 0) {
            const percent = Math.floor((progress.downloaded / progress.total) * 100);
            setTaskProgress(taskId, percent);
            // Log every 25% so the history shows movement without flooding.
            const bucket = Math.floor(percent / 25) * 25;
            if (bucket > lastLoggedPercentRef.current) {
              lastLoggedPercentRef.current = bucket;
              appendTaskLog(taskId, `${bucket}%`);
            }
          }
        }),
      );
      subscriptions.push(
        Events.On("file.download.done", (event) => {
          if (!isCurrent()) return;
          off();
          resolve(String(event.data || ""));
        }),
      );
      subscriptions.push(
        Events.On("file.download.error", (event) => {
          if (!isCurrent()) return;
          off();
          reject(new Error(String(event.data || "Download failed")));
        }),
      );
      // Subscribe first: small downloads may finish before the call returns.
      StartFileDownload(selected.downloadUrl, selected.fileName).catch(
        (e: unknown) => {
          if (!isCurrent()) return;
          off();
          reject(e instanceof Error ? e : new Error(String(e)));
        },
      );
    });

  const runImport = async (
    type: PackageType,
    path: string,
    playerName: string,
    overwrite: boolean,
  ): Promise<string> => {
    if (type === "mcworld") {
      return String(
        await ImportMcworldPath(instanceName, playerName, path, overwrite),
      );
    }
    if (type === "mcaddon") {
      return String(
        playerName
          ? await ImportMcaddonPathWithPlayer(instanceName, playerName, path, overwrite)
          : await ImportMcaddonPath(instanceName, path, overwrite),
      );
    }
    if (type === "skin_pack" && playerName) {
      return String(
        await ImportMcpackPathWithPlayer(instanceName, playerName, path, overwrite),
      );
    }
    return String(await ImportMcpackPath(instanceName, path, overwrite));
  };

  const typeLabel = (type: PackageType) =>
    t(`curseforge.install.type.${type}`);

  const startInstall = async () => {
    if (!file || !target || !instanceName) return;
    const selected = file;
    const attempt = ++attemptRef.current;
    const isCurrent = () => attemptRef.current === attempt;
    const name = fileLabel(selected);

    setPhase("running");
    setSteps([]);
    setOutcome(null);
    setErrorText("");
    setDownloadProgress(null);
    lastLoggedPercentRef.current = -1;
    // A just-cancelled download must stop before the same file starts again.
    await cancelPendingRef.current;
    if (!isCurrent()) return;
    taskIdRef.current = startActivityTask({
      title: t("curseforge.install.task_title", { name: target.modName }),
      description: t("curseforge.install.task_desc", {
        file: name,
        instance: instanceName,
      }),
      source: "curseforge",
    });

    let current = "download";
    try {
      pushStep({
        id: "download",
        label: t("curseforge.install.step_download", { name }),
        status: "active",
      });
      downloadingRef.current = true;
      const dest = await download(selected, attempt);
      downloadingRef.current = false;
      if (!isCurrent()) return;
      updateStep("download", {
        status: "done",
        detail: t("curseforge.install.step_download_done", {
          size: formatFileSize(selected.fileLength),
        }),
      });

      current = "detect";
      pushStep({
        id: "detect",
        label: t("curseforge.install.step_detect"),
        status: "active",
      });
      let type = extensionType(selected.fileName);
      if (type === "mcpack" && (await IsMcpackSkinPackPath(dest))) {
        type = "skin_pack";
      }
      if (!isCurrent()) return;
      updateStep("detect", { status: "done", detail: typeLabel(type) });

      // Only worlds and older-game skin packs go to a player; add-ons and
      // other packs never asked for one.
      const playerName =
        !isUwp &&
        (type === "mcworld" || (type === "skin_pack" && skinPackNeedsPlayer))
          ? player
          : "";
      if (type === "mcworld" && !isUwp && !playerName) {
        throw new Error(t("curseforge.install.player_required"));
      }

      current = "import";
      pushStep({
        id: "import",
        label: t("curseforge.install.step_import", { instance: instanceName }),
        status: "active",
      });
      let err = await runImport(type, dest, playerName, false);
      if (!isCurrent()) return;
      if (isDuplicateError(err)) {
        updateStep("import", {
          status: "waiting",
          detail: t("curseforge.install.duplicate_found"),
        });
        setDuplicatePending(true);
        const overwrite = await new Promise<boolean>((resolve) => {
          duplicateResolveRef.current = resolve;
        });
        duplicateResolveRef.current = null;
        setDuplicatePending(false);
        if (!isCurrent()) return;
        if (!overwrite) {
          updateStep("import", {
            status: "skipped",
            detail: t("curseforge.install.duplicate_kept"),
          });
          finish("kept", t("curseforge.install.duplicate_kept"));
          return;
        }
        updateStep("import", {
          status: "active",
          detail: t("curseforge.install.duplicate_overwriting"),
        });
        err = await runImport(type, dest, playerName, true);
        if (!isCurrent()) return;
      }
      if (err) throw new Error(err);
      updateStep("import", {
        status: "done",
        detail: t("curseforge.install.step_import_done"),
      });
      finish(
        "success",
        t("curseforge.install.done_success", {
          name,
          instance: instanceName,
        }),
      );
    } catch (e) {
      downloadingRef.current = false;
      if (!isCurrent()) return;
      const message = e instanceof Error ? e.message : String(e);
      setSteps((prev) =>
        prev.some((step) => step.id === current)
          ? prev.map((step) =>
              step.id === current
                ? { ...step, status: "error", detail: message }
                : step,
            )
          : prev,
      );
      const taskId = taskIdRef.current;
      if (taskId) appendTaskLog(taskId, message, "error");
      finish("error", message);
    }
  };

  const cancelDownload = () => {
    attemptRef.current++;
    unsubscribeRef.current();
    downloadingRef.current = false;
    cancelPendingRef.current = CancelFileDownload().catch((e) =>
      console.warn("Cancel failed", e),
    );
    const taskId = taskIdRef.current;
    if (taskId) appendTaskLog(taskId, t("curseforge.install.cancelled"), "warning");
    finish("cancelled", t("curseforge.install.cancelled"));
  };

  // Leaving the page while downloading cancels the download.
  React.useEffect(
    () => () => {
      if (downloadingRef.current) cancelDownload();
      duplicateResolveRef.current?.(false);
      unsubscribeRef.current();
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  );

  const resolveDuplicate = (overwrite: boolean) => {
    duplicateResolveRef.current?.(overwrite);
  };

  const requestClose = () => {
    if (phase === "running") {
      if (duplicatePending) {
        resolveDuplicate(false);
        return;
      }
      if (downloadingRef.current) {
        cancelDownload();
        onClose();
      }
      // Importing cannot be interrupted; the dialog stays until it finishes.
      return;
    }
    onClose();
  };

  const usableFiles = files.filter((item) => item.downloadUrl);
  const noDownloadUrl = !!file && !file.downloadUrl;
  const canInstall =
    phase === "setup" &&
    !!file &&
    !noDownloadUrl &&
    !!instanceName &&
    !playersLoading &&
    (!playerRequired || !!player);
  const downloading =
    phase === "running" &&
    steps.some((step) => step.id === "download" && step.status === "active");
  const importing = phase === "running" && !downloading && !duplicatePending;
  const noAnim = shouldDisableAnimations();

  const stepIcon = (status: StepStatus) => {
    switch (status) {
      case "active":
        return <Spinner size="sm" color="accent" />;
      case "done":
        return <FiCheckCircle className="size-5 text-success" aria-hidden />;
      case "error":
        return <FiXCircle className="size-5 text-danger" aria-hidden />;
      case "waiting":
        return <FiAlertTriangle className="size-5 text-warning" aria-hidden />;
      default:
        return <FiMinusCircle className="size-5 text-muted" aria-hidden />;
    }
  };

  const progressPercent =
    downloadProgress && downloadProgress.total > 0
      ? (downloadProgress.downloaded / downloadProgress.total) * 100
      : undefined;

  return (
    <UnifiedModal
      size="wide"
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) requestClose();
      }}
      isDismissable={false}
      type={
        outcome === "error" ? "error" : outcome === "success" ? "success" : "primary"
      }
      icon={
        outcome === "success" ? (
          <FiCheckCircle size={24} />
        ) : outcome === "error" ? undefined : (
          <LuDownload size={24} />
        )
      }
      title={
        outcome === "success"
          ? t("curseforge.install.success_title_named", { name: target?.modName })
          : outcome === "error"
            ? t("curseforge.install.error_title_named", { name: target?.modName })
            : t("curseforge.install.dialog_title", { name: target?.modName })
      }
      footer={
        phase === "setup" ? (
          <>
            <ModalAction variant="secondary" onPress={onClose}>
              {t("common.cancel")}
            </ModalAction>
            <ModalAction
              variant="primary"
              isDisabled={!canInstall}
              onPress={() => void startInstall()}
              data-testid="curseforge-install-confirm"
            >
              <LuDownload size={16} />
              {t("curseforge.install.install")}
            </ModalAction>
          </>
        ) : phase === "running" ? (
          duplicatePending ? (
            <>
              <ModalAction variant="secondary" onPress={() => resolveDuplicate(false)}>
                {t("curseforge.install.keep_existing")}
              </ModalAction>
              <ModalAction variant="primary" onPress={() => resolveDuplicate(true)}>
                {t("curseforge.install.overwrite")}
              </ModalAction>
            </>
          ) : (
            <ModalAction
              variant="secondary"
              isDisabled={importing}
              onPress={requestClose}
            >
              {t("common.cancel")}
            </ModalAction>
          )
        ) : (
          <ModalAction variant="primary" onPress={onClose}>
            {t("curseforge.install.close")}
          </ModalAction>
        )
      }
    >
      <div className={MODAL_STYLES.stack} data-testid="curseforge-install-dialog">
        {phase === "setup" ? (
          <>
            <ModalDescription>{t("curseforge.install.setup_body")}</ModalDescription>
            <div className={MODAL_STYLES.fieldGrid}>
              <Select
                className="min-w-0 sm:col-span-2"
                placeholder={
                  filesState === "loading" && files.length === 0
                    ? t("curseforge.install.files_loading")
                    : t("curseforge.install.file_placeholder")
                }
                isDisabled={usableFiles.length === 0}
                value={fileId || null}
                onChange={(key) => {
                  userPickedFileRef.current = true;
                  setFileId(String(key ?? ""));
                }}
              >
                <Label>{t("curseforge.install.file_label")}</Label>
                <Select.Trigger className={COMPONENT_STYLES.select.trigger}>
                  <Select.Value />
                  <Select.Indicator />
                </Select.Trigger>
                <Select.Popover className={COMPONENT_STYLES.select.popoverContent}>
                  <ListBox items={usableFiles} className={COMPONENT_STYLES.select.listbox}>
                    {(item) => (
                      <ListBox.Item
                        key={item.id}
                        id={String(item.id)}
                        textValue={fileLabel(item)}
                      >
                        <div className="flex min-w-0 flex-col">
                          <span className="truncate text-sm">{fileLabel(item)}</span>
                          <span className="truncate text-xs text-muted">
                            {[
                              sortGameVersions(item.gameVersions || [])[0],
                              formatDateStr(item.fileDate),
                              formatFileSize(item.fileLength),
                            ]
                              .filter(Boolean)
                              .join(" · ")}
                          </span>
                        </div>
                        <ListBox.ItemIndicator />
                      </ListBox.Item>
                    )}
                  </ListBox>
                </Select.Popover>
              </Select>

              <Select
                className="min-w-0"
                placeholder={
                  instancesLoading
                    ? t("common.loading")
                    : instances.length === 0
                      ? t("instance_picker.none_installed")
                      : t("curseforge.install.select_version_placeholder")
                }
                isDisabled={instances.length === 0}
                value={instanceName || null}
                onChange={(key) => setInstanceName(String(key ?? ""))}
              >
                <Label>{t("curseforge.install.local_installation")}</Label>
                <Select.Trigger className={COMPONENT_STYLES.select.trigger}>
                  <Select.Value />
                  <Select.Indicator />
                </Select.Trigger>
                <Select.Popover className={COMPONENT_STYLES.select.popoverContent}>
                  <ListBox items={instances} className={COMPONENT_STYLES.select.listbox}>
                    {(item) => (
                      <ListBox.Item key={item.name} id={item.name} textValue={item.name}>
                        <div className="flex min-w-0 flex-col">
                          <span className="truncate text-sm">{item.name}</span>
                          {item.gameVersion && (
                            <span className="truncate text-xs text-muted">
                              {item.gameVersion}
                            </span>
                          )}
                        </div>
                        <ListBox.ItemIndicator />
                      </ListBox.Item>
                    )}
                  </ListBox>
                </Select.Popover>
              </Select>

              {asksPlayer && (
                <Select
                  className="min-w-0"
                  placeholder={
                    playersLoading
                      ? t("common.loading")
                      : t("curseforge.install.select_player_placeholder")
                  }
                  isDisabled={players.length === 0}
                  value={player || null}
                  onChange={(key) => {
                    userPickedPlayerRef.current = true;
                    setPlayer(String(key ?? ""));
                  }}
                >
                  <Label>{t("curseforge.install.player_label")}</Label>
                  <Select.Trigger className={COMPONENT_STYLES.select.trigger}>
                    <Select.Value />
                    <Select.Indicator />
                  </Select.Trigger>
                  <Select.Popover className={COMPONENT_STYLES.select.popoverContent}>
                    <ListBox className={COMPONENT_STYLES.select.listbox}>
                      {players.map((p) => (
                        <ListBox.Item
                          key={p}
                          id={p}
                          textValue={resolvePlayerDisplayName(p, gamertags)}
                        >
                          <Label>{resolvePlayerDisplayName(p, gamertags)}</Label>
                          <ListBox.ItemIndicator />
                        </ListBox.Item>
                      ))}
                    </ListBox>
                  </Select.Popover>
                </Select>
              )}
            </div>
            {asksPlayer && fileType === "mcpack" && (
              <ModalDescription className="text-xs text-muted">
                {t("curseforge.install.player_hint_skin")}
              </ModalDescription>
            )}
            {filesState === "error" && (
              <ModalNotice tone="danger" role="alert">
                {t("audit.mods.files_load_failed")}
              </ModalNotice>
            )}
            {filesState === "idle" && usableFiles.length === 0 && (
              <ModalNotice tone="warning">
                {t("curseforge.install.no_files")}
              </ModalNotice>
            )}
            {noDownloadUrl && (
              <ModalNotice tone="warning">{t("curseforge.no_download_url")}</ModalNotice>
            )}
            {!instancesLoading && instances.length === 0 && (
              <ModalNotice tone="warning">{t("curseforge.install.no_instances")}</ModalNotice>
            )}
            {playerRequired && !playersLoading && players.length === 0 && (
              <ModalNotice tone="warning">{t("curseforge.install.no_players")}</ModalNotice>
            )}
          </>
        ) : (
          <>
            <ol
              className="flex min-w-0 flex-col gap-2"
              aria-live="polite"
              data-testid="curseforge-install-steps"
            >
              <AnimatePresence initial={false}>
                {steps.map((step) => (
                  <motion.li
                    key={step.id}
                    data-status={step.status}
                    initial={noAnim ? false : { opacity: 0, y: 6 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ duration: 0.2 }}
                    className={cn(
                      MODAL_STYLES.panel,
                      "flex min-w-0 items-start gap-3 py-3",
                      step.status === "error" &&
                        "border-rose-200/70 dark:border-rose-500/30",
                      step.status === "waiting" &&
                        "border-amber-200/70 dark:border-amber-500/30",
                    )}
                  >
                    <span className="mt-0.5 flex size-5 shrink-0 items-center justify-center">
                      {stepIcon(step.status)}
                    </span>
                    <div className="flex min-w-0 flex-1 flex-col gap-1">
                      <span className="font-medium [overflow-wrap:anywhere]">
                        {step.label}
                      </span>
                      {step.id === "download" && step.status === "active" && (
                        <div className="flex flex-col gap-1">
                          <ProgressBar
                            aria-label={step.label}
                            value={progressPercent}
                            isIndeterminate={progressPercent === undefined}
                            size="sm"
                            color="accent"
                            className="w-full"
                          >
                            <ProgressBar.Track>
                              <ProgressBar.Fill />
                            </ProgressBar.Track>
                          </ProgressBar>
                          {downloadProgress && (
                            <span className="text-xs text-muted tabular-nums">
                              {formatFileSize(downloadProgress.downloaded)}
                              {downloadProgress.total > 0 &&
                                ` / ${formatFileSize(downloadProgress.total)}`}
                              {progressPercent !== undefined &&
                                ` · ${Math.floor(progressPercent)}%`}
                            </span>
                          )}
                        </div>
                      )}
                      {step.detail && (
                        <span
                          className={cn(
                            "text-xs [overflow-wrap:anywhere]",
                            step.status === "error"
                              ? "font-mono text-danger"
                              : "text-muted",
                          )}
                        >
                          {step.detail}
                        </span>
                      )}
                      {step.status === "waiting" && duplicatePending && (
                        <span className="text-xs text-foreground/80">
                          {t("curseforge.install.duplicate_body")}
                        </span>
                      )}
                    </div>
                  </motion.li>
                ))}
              </AnimatePresence>
            </ol>
            {phase === "done" && outcome === "success" && (
              <ModalNotice tone="info" role="status">
                {t("curseforge.install.done_success", {
                  name: file ? fileLabel(file) : "",
                  instance: instanceName,
                })}
              </ModalNotice>
            )}
            {phase === "done" && outcome === "kept" && (
              <ModalNotice tone="info" role="status">
                {t("curseforge.install.done_kept")}
              </ModalNotice>
            )}
            {phase === "done" && outcome === "error" && (
              <ModalNotice tone="danger" role="alert">
                <div className="font-semibold">{t("curseforge.install.failed_msg")}</div>
                <div className="font-mono text-xs">{errorText}</div>
              </ModalNotice>
            )}
          </>
        )}
      </div>
    </UnifiedModal>
  );
};
