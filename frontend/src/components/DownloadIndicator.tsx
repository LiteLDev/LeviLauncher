import { Button, Tooltip } from "@heroui/react";
import React from "react";
import { useTranslation } from "react-i18next";
import { FaDownload } from "react-icons/fa";
import { ROUTES } from "@/constants/routes";
import { cn } from "@/utils/cn";
import { isDownloadActive, useDownloads } from "@/utils/DownloadsContext";

interface DownloadIndicatorProps {
  navLocked: boolean;
  tryNavigate: (path: string | number) => void;
}

// Shows downloads running in the background from any page, with their
// combined progress, and opens the task center.
export const DownloadIndicator: React.FC<DownloadIndicatorProps> = ({
  navLocked,
  tryNavigate,
}) => {
  const { t } = useTranslation();
  const { downloads } = useDownloads();
  const active = downloads.filter((dl) => isDownloadActive(dl.status));
  const queued = downloads.filter((dl) => dl.status === "queued").length;
  const paused = downloads.filter((dl) => dl.status === "paused").length;
  if (active.length + queued + paused === 0) return null;

  let done = 0;
  let total = 0;
  for (const dl of active) {
    done += dl.progress?.downloaded || 0;
    total += dl.progress?.total || 0;
  }
  const percent = total > 0 ? Math.min(100, Math.round((done / total) * 100)) : 0;
  const radius = 15;
  const circumference = 2 * Math.PI * radius;
  const label = t("download_manager.indicator.tooltip", {
    active: active.length,
    queued,
    paused,
  });

  return (
    <Tooltip delay={300} closeDelay={0}>
      <Button
        isIconOnly
        size="sm"
        variant="ghost"
        aria-label={label}
        isDisabled={navLocked}
        onPress={() => tryNavigate(ROUTES.downloadTasks)}
        className={cn("rounded-full relative wails-no-drag", "text-brand-600 dark:text-brand-300")}
      >
        <svg className="absolute inset-0 m-auto -rotate-90" width="36" height="36" viewBox="0 0 36 36" aria-hidden="true">
          <circle cx="18" cy="18" r={radius} fill="none" strokeWidth="2.5" className="stroke-surface-tertiary" />
          <circle
            cx="18"
            cy="18"
            r={radius}
            fill="none"
            strokeWidth="2.5"
            strokeLinecap="round"
            className="stroke-brand-500 transition-[stroke-dashoffset] duration-300"
            strokeDasharray={circumference}
            strokeDashoffset={circumference * (1 - percent / 100)}
          />
        </svg>
        <FaDownload size={12} className={active.length > 0 ? "animate-pulse" : ""} />
      </Button>
      <Tooltip.Content>
        {label}
        {total > 0 ? ` · ${percent}%` : ""}
      </Tooltip.Content>
    </Tooltip>
  );
};
