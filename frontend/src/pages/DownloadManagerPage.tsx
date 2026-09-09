import { Button, Card, Chip, ProgressBar, Tooltip } from "@heroui/react";

import React from "react";
import { PageHeader } from "@/components/PageHeader";
import { isDownloadActive, isDownloadTerminal, useDownloads } from "@/utils/DownloadsContext";
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
  const { downloads, cancelDownload, removeDownload, startDownload } =
    useDownloads();

  return (
    <PageContainer>
      <Card className={cn("flex-none", LAYOUT.GLASS_CARD.BASE)}>
        <Card.Content className="p-6">
          <PageHeader
            title={t("download_manager.title")}
            description={t("download_manager.description")}
          />
        </Card.Content>
      </Card>

      <div className="flex flex-col gap-4">
        {downloads.length === 0 && (
          <div className="flex flex-col items-center justify-center py-20 text-muted opacity-60">
            <FaDownload size={48} className="mb-4" />
            <p>{t("download_manager.no_downloads")}</p>
          </div>
        )}

        {downloads.map((task) => {
          const active = isDownloadActive(task.status);
          const verifying = task.status === "verifying";
          const terminal = isDownloadTerminal(task.status);
          return (
            <motion.div
              key={task.dest}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
            >
              <Card className={cn(LAYOUT.GLASS_CARD.BASE, "overflow-hidden")}>
                <Card.Content className="p-4 sm:p-6">
                  <div className="flex flex-col gap-4">
                    <div className="flex items-start justify-between">
                      <div className="flex items-center gap-4">
                        <div className="p-3 rounded-2xl bg-brand-500 brand-primary-foreground shadow-lg shadow-brand-500/20">
                          <FaBoxOpen size={24} />
                        </div>
                        <div>
                          <h3 className="text-lg font-bold text-foreground dark:text-zinc-100 line-clamp-1">
                            {task.fileName ||
                              t("download_manager.unknown_file")}
                          </h3>
                          <div className="flex items-center gap-2 text-sm text-muted dark:text-zinc-400">
                            {task.status === "done" ? (
                              <Chip size="sm" variant="soft" color={"success"}>
                                {<FaCheckCircle />}
                                <Chip.Label>{t("common.completed")}</Chip.Label>
                              </Chip>
                            ) : task.status === "cancelled" ? (
                              <Chip size="sm" variant="soft" color={"warning"}>
                                <Chip.Label>{t("common.cancelled")}</Chip.Label>
                              </Chip>
                            ) : task.error ? (
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

                      {active && !verifying && (
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
                      {terminal && (
                        <div className="flex gap-2">
                          {(task.status === "cancelled" ||
                            task.status === "error") &&
                            task.url && (
                              <Tooltip>
                                <Button
                                  isIconOnly
                                  aria-label={t("download_manager.actions.retry")}
                                  onPress={() =>
                                    startDownload(task.url!, task.fileName)
                                  }
                                  variant={"secondary"}
                                  className={"rounded-full"}
                                >
                                  <FaRedo />
                                </Button>
                                <Tooltip.Content>
                                  {t("download_manager.actions.retry")}
                                </Tooltip.Content>
                              </Tooltip>
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
                            className={
                              "bg-brand-500"
                            }
                          />
                        </ProgressBar.Track>
                      </ProgressBar>
                    </div>

                    {task.error && (
                      <div className="text-sm text-rose-500 bg-rose-50 dark:bg-rose-900/20 p-3 rounded-2xl">
                        {task.error}
                      </div>
                    )}
                  </div>
                </Card.Content>
              </Card>
            </motion.div>
          );
        })}
      </div>
    </PageContainer>
  );
};
