import { Button, Tooltip } from "@heroui/react";

import React from "react";
import { useLocation } from "react-router-dom";

import {
  FaRocket,
  FaDownload,
  FaCog,
  FaList,
  FaInfoCircle,
  FaCube, FaPuzzlePiece, FaTasks, FaBars,
} from "react-icons/fa";
import { motion } from "framer-motion";
import { useTranslation } from "react-i18next";
import { ThemeSwitcher } from "@/components/ThemeSwitcher";
import { LAYOUT } from "@/constants/layout";
import { ROUTES, isRouteActive } from "@/constants/routes";
import { isDownloadActive, useDownloads } from "@/utils/DownloadsContext";

interface SidebarProps {
  expanded: boolean;
  onExpandedChange: (expanded: boolean) => void;
  navLocked: boolean;
  themeMode: string;
  tryNavigate: (path: string | number) => void;
}

export const Sidebar: React.FC<SidebarProps> = ({
  expanded,
  onExpandedChange,
  navLocked,
  themeMode,
  tryNavigate,
}) => {
  const { t } = useTranslation();
  const location = useLocation();
  const { downloads } = useDownloads();
  const activeDownloads = downloads.filter((task) => isDownloadActive(task.status)).length;

  const navItems = [
    {
      key: "home",
      label: t("launcherpage.launch_button"),
      path: ROUTES.home,
      icon: <FaRocket size={20} />,
    },
    {
      key: "download",
      label: t("downloadmodal.download_button"),
      path: ROUTES.download,
      icon: <FaDownload size={20} />,
    },
    {
      key: "versions",
      label: t("nav.versions"),
      path: ROUTES.instances,
      icon: <FaList size={20} />,
    },
    {
      key: "content", label: t("launcherpage.content_manage"), path: ROUTES.content, icon: <FaCube size={20} />,
    },
    {
      key: "mods", label: t("moddedcard.title"), path: ROUTES.mods, icon: <FaPuzzlePiece size={20} />,
    },
    {
      key: "tasks", label: t("download_manager.title"), path: ROUTES.downloadTasks, icon: <FaTasks size={20} />,
    },
    {
      key: "about",
      label: t("nav.about"),
      path: ROUTES.about,
      icon: <FaInfoCircle size={20} />,
    },
    {
      key: "settings",
      label: t("app.settings"),
      path: ROUTES.settings,
      icon: <FaCog size={20} />,
    },
  ];

  return (
    <div
      className={`fixed left-0 top-14 bottom-0 z-50 flex flex-col w-[var(--sidebar-width)] ${LAYOUT.NAVBAR_BG}`}
    >
      <div className="absolute right-0 top-[20px] bottom-0 w-px bg-surface-tertiary/50 dark:bg-surface-secondary/50" />
      {/* Corner Connector */}
      <div className="absolute top-0 -right-[20px] w-[20px] h-[20px] overflow-hidden pointer-events-none">
        {/* Fill the corner gap */}
        <div
          className={`absolute top-0 left-0 w-full h-full ${LAYOUT.NAVBAR_BG}`}
          style={{ clipPath: "path('M 0 20 Q 0 0 20 0 L 0 0 Z')" }}
        />
        {/* Border Stroke */}
        <svg
          width="20"
          height="20"
          viewBox="0 0 20 20"
          className="absolute top-0 left-0 w-full h-full"
        >
          <path
            d="M 0 20 Q 0 0 20 0"
            fill="none"
            stroke="currentColor"
            strokeWidth="1"
            className="text-surface-tertiary/50 dark:text-zinc-800/50"
          />
        </svg>
      </div>

      <Button isIconOnly={!expanded} variant="ghost" aria-label={t(expanded ? "audit.usability.collapse_nav" : "audit.usability.expand_nav")} aria-expanded={expanded} onPress={() => onExpandedChange(!expanded)} className="m-2 shrink-0 rounded-xl">
        <FaBars aria-hidden="true" />{expanded && <span>{t("audit.usability.collapse_nav")}</span>}
      </Button>
      <nav aria-label={t("audit.usability.main_nav")} className="flex-1 min-h-0 flex flex-col items-center gap-2 w-full px-2 overflow-y-auto overflow-x-hidden py-2">
        {navItems.map((item) => {
          const isActive = item.path === ROUTES.download ? location.pathname === ROUTES.download : isRouteActive(location.pathname, item.path);
          return (
            <Tooltip key={item.key} isDisabled={expanded} delay={0} closeDelay={0}>
              <div className={`relative group ${expanded ? "w-full" : ""}`}>
                {isActive && (
                  <motion.div
                    layoutId="active-pill"
                    className="absolute -left-2 top-1/2 -translate-y-1/2 w-1 h-8 bg-brand-500 rounded-r-full"
                    transition={{
                      type: "spring",
                      stiffness: 300,
                      damping: 30,
                    }}
                  />
                )}
                <Button
                  isIconOnly={!expanded}
                  aria-label={item.label}
                  aria-current={isActive ? "page" : undefined}
                  isDisabled={navLocked}
                  onPress={(e) => {
                    tryNavigate(item.path);
                    if (e.pointerType !== "keyboard") {
                      (e.target as HTMLElement).blur();
                    }
                  }}
                  variant={isActive ? "secondary" : "ghost"}
                  className={`${expanded ? "w-full justify-start px-3" : "w-12"} h-11 rounded-xl transition-colors duration-200 ${
                    isActive
                      ? "bg-brand-500/10 text-brand-600 dark:text-brand-400"
                      : "text-zinc-500 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100 hover:bg-surface-secondary "
                  }`}
                >
                  {item.icon}
                  {expanded && <span className="min-w-0 flex-1 truncate text-left text-sm">{item.label}</span>}
                  {item.key === "tasks" && activeDownloads > 0 && <span aria-label={t("audit.usability.active_downloads", { count: activeDownloads })} className={`${expanded ? "" : "absolute right-0 top-0"} rounded-full bg-accent px-1.5 text-xs text-accent-foreground tabular-nums`}>{activeDownloads}</span>}
                </Button>
              </div>
              <Tooltip.Content placement={"right"}>
                {item.label}
              </Tooltip.Content>
            </Tooltip>
          );
        })}
      </nav>

      <div className="flex flex-col items-center gap-4 pb-6 w-full px-2">
        {themeMode !== "auto" &&
          themeMode !== "schedule" &&
          themeMode !== "system" && (
            <Tooltip delay={0} closeDelay={0}>
              <div>
                <ThemeSwitcher
                  isDisabled={navLocked}
                  className="w-12 h-12 rounded-xl text-zinc-500 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100 hover:bg-surface-secondary transition-all duration-200"
                  iconSize={20}
                />
              </div>
              <Tooltip.Content placement={"right"}>
                {t("theme.toggle")}
              </Tooltip.Content>
            </Tooltip>
          )}
      </div>
    </div>
  );
};
