import React from "react";
import { useLocation } from "react-router-dom";
import { Button, Tooltip } from "@heroui/react";
import {
  FaRocket,
  FaDownload,
  FaCog,
  FaList,
  FaInfoCircle,
} from "react-icons/fa";
import { motion, AnimatePresence } from "framer-motion";
import { useTranslation } from "react-i18next";
import { ThemeSwitcher } from "@/components/ThemeSwitcher";
import { LAYOUT } from "@/constants/layout";

interface SidebarProps {
  isBeta: boolean;
  navLocked: boolean;
  themeMode: string;
  revealStarted: boolean;
  isUpdatingMode: boolean;
  isOnboardingMode: boolean;
  hasEnteredLauncher: boolean;
  tryNavigate: (path: string | number) => void;
}

export const Sidebar: React.FC<SidebarProps> = ({
  isBeta,
  navLocked,
  themeMode,
  revealStarted,
  isUpdatingMode,
  isOnboardingMode,
  hasEnteredLauncher,
  tryNavigate,
}) => {
  const { t } = useTranslation();
  const location = useLocation();

  const navItems = [
    {
      key: "home",
      label: t("launcherpage.launch_button"),
      path: "/",
      icon: <FaRocket size={20} />,
    },
    {
      key: "download",
      label: t("downloadmodal.download_button"),
      path: "/download",
      icon: <FaDownload size={20} />,
    },
    {
      key: "versions",
      label: t("nav.versions"),
      path: "/versions",
      icon: <FaList size={20} />,
    },
    {
      key: "about",
      label: t("nav.about"),
      path: "/about",
      icon: <FaInfoCircle size={20} />,
    },
    {
      key: "settings",
      label: t("app.settings"),
      path: "/settings",
      icon: <FaCog size={20} />,
    },
  ];

  return (
    <AnimatePresence>
      <motion.div
        key="sidebar"
        className={`fixed left-0 top-14 bottom-0 z-50 flex flex-col w-14 ${LAYOUT.NAVBAR_BG}`}
        initial={{ x: -80, opacity: 0 }}
        animate={{
          x: revealStarted ? 0 : -80,
          opacity: revealStarted ? 1 : 0,
        }}
        exit={{ x: -80, opacity: 0 }}
        transition={{ duration: 0.5, ease: "easeOut" }}
      >
        <div className="absolute right-0 top-[20px] bottom-0 w-px bg-default-200/50 dark:bg-zinc-800/50" />
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
              className="text-default-200/50 dark:text-zinc-800/50"
            />
          </svg>
        </div>

        <div className="flex-1 flex flex-col items-center gap-4 w-full px-2 overflow-y-auto scrollbar-hide py-4">
          {navItems.map((item) => {
            const isActive = location.pathname === item.path;
            return (
              <Tooltip
                key={item.key}
                content={item.label}
                placement="right"
                delay={0}
                closeDelay={0}
              >
                <div className="relative group">
                  {isActive && (
                    <motion.div
                      layoutId="active-pill"
                      className="absolute -left-2 top-1/2 -translate-y-1/2 w-1 h-8 bg-primary-500 rounded-r-full"
                      transition={{
                        type: "spring",
                        stiffness: 300,
                        damping: 30,
                      }}
                    />
                  )}
                  <Button
                    isIconOnly
                    variant={isActive ? "flat" : "light"}
                    color={isActive ? "success" : "default"}
                    aria-label={item.label}
                    isDisabled={navLocked}
                    onPress={(e) => {
                      tryNavigate(item.path);
                      (e.target as HTMLElement).blur();
                    }}
                    className={`w-12 h-12 rounded-xl transition-all duration-200 ${
                      isActive
                        ? "bg-primary-500/10 text-primary-600 dark:text-primary-400"
                        : "text-zinc-500 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100 hover:bg-default-100 dark:hover:bg-zinc-800"
                    }`}
                  >
                    {item.icon}
                  </Button>
                </div>
              </Tooltip>
            );
          })}
        </div>

        <div className="flex flex-col items-center gap-4 pb-6 w-full px-2">
          {themeMode !== "auto" &&
            themeMode !== "schedule" &&
            themeMode !== "system" && (
              <Tooltip
                content={t("theme.toggle")}
                placement="right"
                delay={0}
                closeDelay={0}
              >
                <div>
                  <ThemeSwitcher
                    className="w-12 h-12 rounded-xl text-zinc-500 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100 hover:bg-default-100 dark:hover:bg-zinc-800 transition-all duration-200"
                    iconSize={20}
                  />
                </div>
              </Tooltip>
            )}
        </div>
      </motion.div>
    </AnimatePresence>
  );
};
