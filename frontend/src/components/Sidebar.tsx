import { Button, Tooltip } from "@heroui/react";

import React from "react";
import { useLocation } from "react-router-dom";

import {
  FaRocket,
  FaDownload,
  FaCog,
  FaList,
  FaInfoCircle,
} from "react-icons/fa";
import { motion } from "framer-motion";
import { useTranslation } from "react-i18next";
import { ThemeSwitcher } from "@/components/ThemeSwitcher";
import { ROUTES, isRouteActive } from "@/constants/routes";

interface SidebarProps {
  navLocked: boolean;
  themeMode: string;
  tryNavigate: (path: string | number) => void;
}

export const Sidebar: React.FC<SidebarProps> = ({
  navLocked,
  themeMode,
  tryNavigate,
}) => {
  const { t } = useTranslation();
  const location = useLocation();

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
      className="fixed left-0 top-14 bottom-0 z-50 flex flex-col w-14"
    >
      <nav aria-label={t("audit.usability.main_nav")} className="flex-1 flex flex-col items-center gap-4 w-full px-2 overflow-y-auto overflow-x-hidden scrollbar-hide py-4">
        {navItems.map((item) => {
          const isActive = isRouteActive(location.pathname, item.path);
          return (
            <Tooltip key={item.key} delay={0} closeDelay={0}>
              <div className="relative group">
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
                  isIconOnly
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
                  className={`w-12 h-12 rounded-xl transition-all duration-200 ${
                    isActive
                      ? "bg-brand-500/10 text-brand-600 dark:text-brand-400"
                      : "text-zinc-500 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100 hover:bg-surface-secondary "
                  }`}
                >
                  {item.icon}
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
