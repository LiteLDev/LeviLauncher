import { Button, Card, Chip, ProgressBar, Tooltip } from "@heroui/react";

import React from "react";
import { useNavigate } from "react-router-dom";
import { ROUTES } from "@/constants/routes";
import { openDirectory } from "@/utils/explorer";
import { PageHeader } from "@/components/PageHeader";
import {
  downloadErrorText,
  installStateForDownload,
  isDownloadActive,
  isDownloadTerminal,
  useDownloads,
} from "@/utils/DownloadsContext";
import { openActivityCenter } from "@/utils/activity";
import { useTranslation } from "react-i18next";

import { motion } from "framer-motion";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { cn } from "@/utils/cn";
import { formatBytes, formatSpeed } from "@/utils/formatting";
import {
  FaDownload,
  FaTimes,
  FaCheckCircle,
  FaExclamationCircle,
  FaBoxOpen,
  FaRedo,
} from "react-icons/fa";

export const DownloadManagerPage: React.FC = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [filter, setFilter] = React.useState("all");
  const { downloads, cancelDownload, removeDownload, startDownload } =
    useDownloads();
  const matchesFilter = (status: string, value: string) => value === "all" ||
    (value === "active" ? isDownloadActive(status) : value === "done" ? status === "done" : ["error", "cancelled"].includes(status));
  const visibleDownloads = downloads.filter((task) => matchesFilter(task.status, filter));

  return (
    <PageContainer>
      <div className={cn(LAYOUT.PAGE.READABLE, "flex flex-col gap-4")}>
      <Card className={cn("flex-none", LAYOUT.GLASS_CARD.BASE)}>
        <Card.Content className="p-4 sm:p-6">
          <PageHeader
            title={t("download_manager.title")}
            description={t("download_manager.description")}
            endContent={<Button variant="primary" onPress={() => navigate(ROUTES.download)}>{t("audit.usability.browse_downloads")}</Button>}
          />
          <nav aria-label={t("audit.usability.download_filters")} className="mt-4 flex flex-wrap gap-2">
            {["all", "active", "done", "failed"].map((value) => (
              <Button key={value} size="sm" variant={filter === value ? "primary" : "secondary"} aria-pressed={filter === value} onPress={() => setFilter(value)}>
                {t(`audit.usability.download_${value}`)} ({downloads.filter((task) => matchesFilter(task.status, value)).length})
              </Button>
            ))}
          </nav>
        </Card.Content>
      </Card>

      <div className="flex flex-col gap-4">
        {visibleDownloads.length === 0 && (
          <div role="status" className="flex flex-col items-center justify-center gap-4 py-[clamp(2rem,8vh,5rem)] text-muted">
            <FaDownload size={48} className="mb-4" />
            <p>{t(downloads.length === 0 ? "download_manager.no_downloads" : "common.no_results")}</p>
            {downloads.length === 0 ? <Button variant="primary" onPress={() => navigate(ROUTES.download)}>{t("audit.usability.browse_downloads")}</Button> : <Button variant="secondary" onPress={() => setFilter("all")}>{t("audit.usability.download_all")}</Button>}
          </div>
        )}

        {visibleDownloads.map((task) => {
          const active = isDownloadActive(task.status);
          const verifying = task.status === "verifying";
          const terminal = isDownloadTerminal(task.status);
          const failed = task.status === "error" || (!!task.error && !terminal);
          return (
            <motion.div
              data-material-motion
              key={task.dest}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
            >
              <Card className={cn(LAYOUT.GLASS_CARD.BASE, "overflow-hidden", failed && "border-danger/30")}>
                <Card.Content className="p-4 sm:p-6">
                  <div className="flex flex-col gap-4">
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div className="flex min-w-0 flex-[1_1_14rem] items-center gap-4">
                        <div className="shrink-0 p-3 rounded-2xl bg-brand-500 brand-primary-foreground shadow-lg shadow-brand-500/20">
                          <FaBoxOpen size={24} />
                        </div>
                        <div className="min-w-0">
                          <h3 title={task.fileName} className="text-lg font-bold text-foreground dark:text-zinc-100 truncate">
                            {task.fileName ||
                              t("download_manager.unknown_file")}
                          </h3>
                          <div className="flex flex-wrap items-center gap-2 text-sm text-muted dark:text-zinc-400">
                            {task.status === "done" ? (
                              <Chip size="sm" variant="soft" color={"success"}>
                                {<FaCheckCircle />}
                                <Chip.Label>{t("common.completed")}</Chip.Label>
                              </Chip>
                            ) : task.status === "cancelled" ? (
                              <Chip size="sm" variant="soft" color={"warning"}>
                                <Chip.Label>{t("common.cancelled")}</Chip.Label>
                              </Chip>
                            ) : failed ? (
                              <Chip size="sm" variant="soft" color={"danger"}>
                                {<FaExclamationCircle />}
                                <Chip.Label>{t("common.error")}</Chip.Label>
                              </Chip>
                            ) : (
                              <Chip size="sm" variant="soft" color={"accent"}>
                                <Chip.Label>
                                  {t(verifying ? "audit.primary.download.verifying" : "common.downloading")}
                                </Chip.Label>
                              </Chip>
                            )}
                            {active && !verifying && (
                              <>
                                <span>•</span>
                                <span className="font-mono">
                                  {formatSpeed(task.speed)}
                                </span>
                              </>
                            )}
                          </div>
                        </div>
                      </div>

                      {active && (
                        <div className="flex shrink-0 flex-wrap gap-2">
                          {task.installer && (
                            <Button
                              variant="secondary"
                              onPress={() => navigate(ROUTES.install, { state: installStateForDownload(task, ROUTES.downloadTasks) })}
                            >
                              {t("download_flow.install_when_done")}
                            </Button>
                          )}
                          {!verifying && (
                            <Tooltip>
                              <Button
                                aria-label={t("audit.primary.download.cancel")}
                                isIconOnly
                                onPress={() => cancelDownload(task.dest)}
                                variant={"danger-soft"}
                                className={"rounded-full"}
                              >
                                <FaTimes />
                              </Button>
                              <Tooltip.Content>{t("audit.primary.download.cancel")}</Tooltip.Content>
                            </Tooltip>
                          )}
                        </div>
                      )}
                      {terminal && (
                        <div className="flex shrink-0 flex-wrap gap-2">
                          {task.status === "done" && (
                            <>
                              {task.installer && <Button variant="primary" onPress={() => navigate(ROUTES.install, { state: installStateForDownload(task, ROUTES.downloadTasks) })}>{t("audit.usability.install_download")}</Button>}
                              <Button variant="secondary" isDisabled={!/[\\/]/.test(task.dest)} onPress={() => {
                                const directory = task.dest.slice(0, Math.max(task.dest.lastIndexOf("/"), task.dest.lastIndexOf("\\")) + 1);
                                if (directory) void openDirectory(directory);
                              }}>{t("common.open_folder")}</Button>
                            </>
                          )}
                          {(task.status === "cancelled" ||
                            task.status === "error") &&
                            task.url && (
                              <Button
                                aria-label={t("download_manager.actions.retry")}
                                onPress={() =>
                                  startDownload(task.url!, task.fileName, task.md5sum, task.installer)
                                }
                                variant={task.status === "error" ? "primary" : "secondary"}
                              >
                                <FaRedo />
                                {t("download_manager.actions.retry")}
                              </Button>
                            )}
                          <Tooltip>
                            <Button
                              isIconOnly
                              aria-label={t("audit.primary.download.remove")}
                              onPress={() => removeDownload(task.dest)}
                              variant={"ghost"}
                              className={cn(
                                "rounded-full",
                                "text-muted hover:text-danger hover:bg-danger/10",
                              )}
                            >
                              <FaTimes />
                            </Button>
                            <Tooltip.Content>
                              {t("audit.primary.download.remove_hint")}
                            </Tooltip.Content>
                          </Tooltip>
                        </div>
                      )}
                    </div>

                    <div className="flex flex-col gap-2">
                      <div className="flex justify-between text-xs text-muted dark:text-zinc-400 font-medium">
                        <span>
                          {task.progress
                            ? formatBytes(task.progress.downloaded)
                            : "0 B"}
                        </span>
                        <span>
                          {task.progress
                            ? formatBytes(task.progress.total)
                            : "0 B"}
                        </span>
                      </div>
                      <ProgressBar
                        aria-label={t("audit.primary.download.progress", { name: task.fileName })}
                        value={
                          task.progress && task.progress.total > 0
                            ? (task.progress.downloaded / task.progress.total) *
                              100
                            : 0
                        }
                        size="md"
                        isIndeterminate={
                          verifying || (active &&
                          (!task.progress || task.progress.total === 0))
                        }
                      >
                        <ProgressBar.Track>
                          <ProgressBar.Fill
                            className={failed ? "bg-danger" : "bg-brand-500"}
                          />
                        </ProgressBar.Track>
                      </ProgressBar>
                    </div>

                    {task.error && (
                      <div role="alert" className="flex flex-wrap items-start justify-between gap-2 text-sm text-rose-600 dark:text-rose-400 bg-rose-50 dark:bg-rose-900/20 p-3 rounded-2xl">
                        <div className="min-w-0 flex-[1_1_16rem]">
                          <div className="font-semibold">{t("download_flow.failed")}</div>
                          <div className="break-words select-text">{downloadErrorText(task.error, t)}</div>
                        </div>
                        <Button size="sm" variant="ghost" onPress={openActivityCenter}>
                          {t("download_flow.show_log")}
                        </Button>
                      </div>
                    )}
                  </div>
                </Card.Content>
              </Card>
            </motion.div>
          );
        })}
      </div>
      </div>
    </PageContainer>
  );
};
