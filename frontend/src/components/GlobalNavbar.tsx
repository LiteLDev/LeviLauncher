import React from "react";
import { useLocation } from "react-router-dom";
import {
  Button,
  Tooltip,
  Chip,
  Dropdown,
  DropdownTrigger,
  DropdownMenu,
  DropdownItem,
} from "@heroui/react";
import { ThemeSwitcher } from "@/components/ThemeSwitcher";
import { UserAvatar } from "@/components/UserAvatar";
import { LeviIcon } from "@/icons/LeviIcon";
import { WindowControls } from "@/components/WindowControls";
import {
  FaRocket,
  FaDownload,
  FaCog,
  FaEllipsisH,
  FaList,
  FaInfoCircle,
} from "react-icons/fa";
import { IoArrowBack } from "react-icons/io5";
import { motion, AnimatePresence } from "framer-motion";
import { useTranslation } from "react-i18next";

interface GlobalNavbarProps {
  isBeta: boolean;
  navLocked: boolean;
  revealStarted: boolean;
  isUpdatingMode: boolean;
  isOnboardingMode: boolean;
  hasEnteredLauncher: boolean;
  tryNavigate: (path: string | number) => void;
}

export const GlobalNavbar: React.FC<GlobalNavbarProps> = ({
  isBeta,
  navLocked,
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
      icon: <FaRocket size={18} />,
      navbarClass: "flex",
    },
    {
      key: "download",
      label: t("downloadmodal.download_button"),
      path: "/download",
      icon: <FaDownload size={18} />,
      navbarClass: "flex",
    },
    {
      key: "versions",
      label: t("nav.versions"),
      path: "/versions",
      icon: <FaList size={18} />,
      navbarClass: "hidden lg:flex",
      menuClass: "flex lg:hidden",
      breakpoint: "lg",
    },
    {
      key: "about",
      label: t("nav.about"),
      path: "/about",
      icon: <FaInfoCircle size={18} />,
      navbarClass: "hidden lg:flex",
      menuClass: "flex lg:hidden",
      breakpoint: "lg",
    },
    {
      key: "settings",
      label: t("app.settings"),
      path: "/settings",
      icon: <FaCog size={18} />,
      navbarClass: "flex",
    },
  ];

  const extraItems = navItems.filter((item) => item.menuClass);

  const activeExtraItem = extraItems.find(
    (item) => location.pathname === item.path,
  );

  const moreButtonClass = React.useMemo(() => {
    const base =
      "min-w-0 px-3 h-10 rounded-xl transition-all duration-200 outline-none data-[focus-visible=true]:outline-none";
    const inactive =
      "text-zinc-700 hover:text-black dark:text-zinc-300 dark:hover:text-white font-medium";
    const active =
      "bg-zinc-100 dark:bg-zinc-800 text-zinc-900 dark:text-zinc-100 font-medium";

    if (!activeExtraItem) return `${base} ${inactive}`;

    if (activeExtraItem.breakpoint === "lg") {
      return `${base} ${active} lg:bg-transparent lg:dark:bg-transparent lg:text-zinc-700 lg:dark:text-zinc-300 lg:hover:text-black lg:dark:hover:text-white`;
    }

    return `${base} ${active}`;
  }, [activeExtraItem]);

  return (
    <AnimatePresence>
      <motion.div
        key="navbar"
        id="wails-draggable"
        className="fixed top-0 left-0 right-0 z-50 px-4 pt-2 flex justify-center pointer-events-none"
        initial={{ opacity: 0, y: -20 }}
        animate={{
          opacity: revealStarted ? 1 : 0,
          y: revealStarted ? 0 : -20,
        }}
        exit={{ opacity: 0, y: -20 }}
        transition={{ duration: 0.6, ease: [0.22, 1, 0.36, 1] }}
      >
        <div className="pointer-events-auto w-full bg-white/80 dark:bg-zinc-900/40 backdrop-blur-2xl border border-white/40 dark:border-zinc-800/50 shadow-sm dark:shadow-zinc-950/20 rounded-2xl px-2 py-2 sm:px-4 flex items-center gap-4">
          <div className="flex items-center gap-3 shrink-0">
            <Button
              isIconOnly
              variant="light"
              size="sm"
              radius="lg"
              onPress={() => tryNavigate(-1)}
              isDisabled={navLocked}
              className="wails-no-drag text-default-500"
            >
              <IoArrowBack size={20} />
            </Button>
            <div className="p-1">
              <LeviIcon width={28} height={28} />
            </div>
            <div className="hidden sm:flex flex-col leading-none gap-0.5">
              <p className="font-bold text-[16px] tracking-tight bg-gradient-to-r from-emerald-600 to-teal-600 bg-clip-text text-transparent">
                LeviLauncher
              </p>
              {isBeta && (
                <span className="text-[10px] font-bold text-amber-500 uppercase tracking-wider">
                  Beta
                </span>
              )}
            </div>
          </div>

          <div className="flex-1 flex items-center justify-center gap-1 sm:gap-2 whitespace-nowrap overflow-x-auto px-2 scrollbar-hide">
            {navItems.map((item) => {
              const isActive = location.pathname === item.path;
              return (
                <Tooltip
                  key={item.key}
                  content={item.label}
                  delay={500}
                  closeDelay={0}
                >
                  <Button
                    variant={isActive ? "flat" : "light"}
                    color="default"
                    aria-label={item.label}
                    isDisabled={navLocked}
                    onPress={(e) => {
                      tryNavigate(item.path);
                      (e.target as HTMLElement).blur();
                    }}
                    className={`min-w-0 px-3 h-10 rounded-xl transition-all duration-200 outline-none data-[focus-visible=true]:outline-none ${item.navbarClass} ${
                      isActive
                        ? "bg-zinc-100 dark:bg-zinc-800 text-zinc-900 dark:text-zinc-100 font-medium"
                        : "text-zinc-700 hover:text-black dark:text-zinc-300 dark:hover:text-white font-medium"
                    }`}
                    startContent={
                      <span
                        className={
                          isActive ? "text-zinc-900 dark:text-zinc-100" : ""
                        }
                      >
                        {item.icon}
                      </span>
                    }
                  >
                    <span className="hidden md:inline">{item.label}</span>
                  </Button>
                </Tooltip>
              );
            })}

            <div className="lg:hidden">
              <Dropdown>
                <DropdownTrigger>
                  <Button
                    variant="light"
                    aria-label="More Menu"
                    isDisabled={navLocked}
                    className={moreButtonClass}
                    startContent={<FaEllipsisH size={18} />}
                  >
                    <span className="hidden md:inline">{t("nav.more")}</span>
                  </Button>
                </DropdownTrigger>
                <DropdownMenu
                  aria-label="more-menu"
                  onAction={(key) => {
                    const item = extraItems.find((i) => i.key === key);
                    if (item) tryNavigate(item.path);
                  }}
                >
                  {extraItems.map((item) => (
                    <DropdownItem
                      key={item.key}
                      startContent={React.cloneElement(item.icon as any, {
                        size: 14,
                      })}
                      className={item.menuClass}
                    >
                      {item.label}
                    </DropdownItem>
                  ))}
                </DropdownMenu>
              </Dropdown>
            </div>
          </div>

          <div className="flex items-center gap-3 shrink-0 justify-end">
            <div className="h-8 w-px bg-default-200 dark:bg-zinc-700 mx-1 hidden sm:block" />

            <ThemeSwitcher />
            <div className="hidden sm:block">
              <UserAvatar />
            </div>

            <WindowControls
              navLocked={navLocked}
              isOnboardingMode={isOnboardingMode}
            />
          </div>
        </div>
      </motion.div>
    </AnimatePresence>
  );
};
