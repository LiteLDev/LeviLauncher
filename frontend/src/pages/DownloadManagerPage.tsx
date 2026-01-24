import React from "react";
import { PageHeader } from "@/components/PageHeader";
import { useDownloads } from "@/utils/DownloadsContext";
import { useTranslation } from "react-i18next";
import { Card, CardBody, Progress, Button, Chip, Tooltip } from "@heroui/react";
import { motion } from "framer-motion";
import { FaDownload, FaTimes, FaCheckCircle, FaExclamationCircle, FaBoxOpen, FaRedo, FaTrash } from "react-icons/fa";

export const DownloadManagerPage: React.FC = () => {
  const { t } = useTranslation();
  const { downloads, cancelDownload, removeDownload, startDownload } = useDownloads();

  // Formatting helpers
  const formatBytes = (bytes: number) => {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
  };

  const formatSpeed = (bytesPerSec: number) => {
    return formatBytes(bytesPerSec) + "/s";
  };

  return (
    <motion.div
      className="w-full max-w-full mx-auto p-4 h-full flex flex-col"
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.25 }}
    >
      <div className="flex flex-col h-full">
        <Card className="flex-1 min-h-0 border-none shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl">
          <CardBody className="p-0 flex flex-col h-full overflow-hidden">
            <div className="shrink-0 p-4 sm:p-6 pb-2 flex flex-col gap-4 border-b border-default-200 dark:border-white/10">
              <PageHeader
                title={t("download_manager.title", { defaultValue: "下载管理" })}
                description={t("download_manager.description", {
                  defaultValue: "查看和管理当前的下载任务",
                })}
              />
            </div>

            <div className="flex-1 overflow-y-auto p-4 sm:p-8">
              <div className="max-w-4xl mx-auto flex flex-col gap-6">
                {downloads.length === 0 && (
                  <div className="flex flex-col items-center justify-center py-20 text-default-400 opacity-60">
                    <FaDownload size={48} className="mb-4" />
                    <p>
                      {t("download_manager.no_downloads", {
                        defaultValue: "当前没有活动的下载任务",
                      })}
                    </p>
                  </div>
                )}

                {downloads.map((task) => {
                  const active =
                    task.status === "started" ||
                    task.status === "resumed" ||
                    task.status === "starting";
                  return (
                    <motion.div
                      key={task.dest}
                      initial={{ opacity: 0, y: 10 }}
                      animate={{ opacity: 1, y: 0 }}
                    >
                      <Card className="bg-default-50 dark:bg-zinc-900/50 border border-default-200 dark:border-white/5 shadow-sm">
                        <CardBody className="p-6">
                          <div className="flex flex-col gap-4">
                            <div className="flex items-start justify-between">
                              <div className="flex items-center gap-4">
                                <div className="p-3 rounded-2xl bg-gradient-to-br from-emerald-500 to-teal-600 text-white shadow-lg shadow-emerald-500/20">
                                  <FaBoxOpen size={24} />
                                </div>
                                <div>
                                  <h3 className="text-lg font-bold text-default-900 dark:text-zinc-100 line-clamp-1">
                                    {task.fileName ||
                                      t("download_manager.unknown_file", {
                                        defaultValue: "未知文件",
                                      })}
                                  </h3>
                                  <div className="flex items-center gap-2 text-small text-default-500">
                                    {task.status === "done" ? (
                                      <Chip
                                        color="success"
                                        size="sm"
                                        variant="flat"
                                        startContent={<FaCheckCircle />}
                                      >
                                        {t("common.completed", {
                                          defaultValue: "已完成",
                                        })}
                                      </Chip>
                                    ) : task.status === "cancelled" ? (
                                      <Chip
                                        color="warning"
                                        size="sm"
                                        variant="flat"
                                      >
                                        {t("common.cancelled")}
                                      </Chip>
                                    ) : task.error ? (
                                      <Chip
                                        color="danger"
                                        size="sm"
                                        variant="flat"
                                        startContent={<FaExclamationCircle />}
                                      >
                                        {t("common.error", {
                                          defaultValue: "错误",
                                        })}
                                      </Chip>
                                    ) : (
                                      <Chip
                                        color="primary"
                                        size="sm"
                                        variant="flat"
                                        className="animate-pulse"
                                      >
                                        {t("common.downloading", {
                                          defaultValue: "下载中",
                                        })}
                                      </Chip>
                                    )}
                                    {active && (
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
                                <Button
                                  isIconOnly
                                  color="danger"
                                  variant="flat"
                                  radius="full"
                                  onPress={() => cancelDownload(task.dest)}
                                >
                                  <FaTimes />
                                </Button>
                              )}
                              {!active && (
                                <div className="flex gap-2">
                                  {(task.status === "cancelled" || task.status === "error") && task.url && (
                                    <Tooltip content={t("download_manager.actions.retry")}>
                                      <Button
                                        isIconOnly
                                        color="primary"
                                        variant="flat"
                                        radius="full"
                                        onPress={() => startDownload(task.url!, task.fileName)}
                                      >
                                        <FaRedo />
                                      </Button>
                                    </Tooltip>
                                  )}
                                  <Tooltip content={t("download_manager.actions.delete")}>
                                    <Button
                                      isIconOnly
                                      color="default"
                                      variant="light"
                                      radius="full"
                                      className="text-default-400 hover:text-danger hover:bg-danger/10"
                                      onPress={() => removeDownload(task.dest)}
                                    >
                                      <FaTimes />
                                    </Button>
                                  </Tooltip>
                                </div>
                              )}
                            </div>

                            <div className="flex flex-col gap-2">
                              <div className="flex justify-between text-tiny text-default-500 font-medium">
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
                              <Progress
                                aria-label="Download progress"
                                value={
                                  task.progress && task.progress.total > 0
                                    ? (task.progress.downloaded / task.progress.total) * 100
                                    : 0
                                }
                                classNames={{
                                  indicator:
                                    "bg-gradient-to-r from-emerald-500 to-teal-500",
                                }}
                                size="md"
                                isIndeterminate={
                                  active && (!task.progress || task.progress.total === 0)
                                }
                              />
                            </div>

                            {task.error && (
                              <div className="text-small text-danger-500 bg-danger-50 dark:bg-danger-900/20 p-3 rounded-lg">
                                {task.error}
                              </div>
                            )}
                          </div>
                        </CardBody>
                      </Card>
                    </motion.div>
                  );
                })}
              </div>
            </div>
          </CardBody>
        </Card>
      </div>
    </motion.div>
  );
};
