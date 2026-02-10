import React from "react";
import { WindowControls } from "./WindowControls";
import { UserAvatar } from "@/components/UserAvatar";

import { Button, Tooltip } from "@heroui/react";
import { IoArrowBack, IoArrowForward, IoChevronForward } from "react-icons/io5";
import { useNavigate, useLocation, Link } from "react-router-dom";
import { LeviIcon } from "@/icons/LeviIcon";
import { motion, AnimatePresence } from "framer-motion";
import { useTranslation } from "react-i18next";
import { useNavigationHistory } from "@/utils/NavigationHistoryContext";

interface TopBarProps {
  navLocked: boolean;
  isOnboardingMode: boolean;
  revealStarted: boolean;
}

export const TopBar: React.FC<TopBarProps> = ({
  navLocked,
  isOnboardingMode,
  revealStarted,
}) => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const { canGoBack, canGoForward, getBackEntry, getForwardEntry } =
    useNavigationHistory();

  const pathnames = location.pathname.split("/").filter((x) => x);

  const getName = (value: string) => {
    try {
      const decoded = decodeURIComponent(value);
      return decoded.charAt(0).toUpperCase() + decoded.slice(1);
    } catch {
      return value.charAt(0).toUpperCase() + value.slice(1);
    }
  };

  const NON_CLICKABLE_SEGMENTS = ["mod", "package"];

  return (
    <AnimatePresence>
      <motion.div
        key="topbar"
        id="wails-draggable"
        className="fixed top-0 right-0 left-0 h-14 z-[60] flex items-center justify-between pr-4 bg-white/80 dark:bg-zinc-900/40 backdrop-blur-xl"
        initial={{ x: -80, opacity: 0 }}
        animate={{
          x: revealStarted ? 0 : -80,
          opacity: revealStarted ? 1 : 0,
        }}
        exit={{ x: -80, opacity: 0 }}
        transition={{ duration: 0.5, ease: "easeOut" }}
      >
        <div className="absolute bottom-0 right-0 left-[calc(3.5rem+20px)] h-px bg-default-200/50 dark:bg-zinc-800/50" />
        <div className="flex items-center gap-2 overflow-hidden">
          <div className="w-14 shrink-0 flex items-center justify-center">
            <LeviIcon width={32} height={32} />
          </div>

          <div className="flex items-center gap-1 shrink-0">
            <Tooltip
              content={getBackEntry()?.title || t("nav.back")}
              delay={500}
              closeDelay={0}
              isDisabled={!canGoBack || navLocked}
            >
              <Button
                isIconOnly
                variant="light"
                size="sm"
                radius="lg"
                onPress={() => navigate(-1)}
                isDisabled={navLocked || !canGoBack}
                className="wails-no-drag text-default-500"
                aria-label="Go back"
              >
                <IoArrowBack size={20} />
              </Button>
            </Tooltip>
            <Tooltip
              content={getForwardEntry()?.title || t("nav.forward")}
              delay={500}
              closeDelay={0}
              isDisabled={!canGoForward || navLocked}
            >
              <Button
                isIconOnly
                variant="light"
                size="sm"
                radius="lg"
                onPress={() => navigate(1)}
                isDisabled={navLocked || !canGoForward}
                className="wails-no-drag text-default-500"
                aria-label="Go forward"
              >
                <IoArrowForward size={20} />
              </Button>
            </Tooltip>
          </div>

          <div className="w-px h-5 bg-default-300/50 mx-1 shrink-0" />

          <nav className="flex items-center text-sm font-medium whitespace-nowrap overflow-hidden text-ellipsis min-w-0">
            {navLocked ? (
              <span
                className={`${
                  pathnames.length === 0
                    ? "font-bold text-lg bg-gradient-to-r from-emerald-600 to-teal-600 bg-clip-text text-transparent"
                    : "text-default-500"
                }`}
              >
                {pathnames.length === 0 ? "LeviLauncher" : "Home"}
              </span>
            ) : (
              <Link
                to="/"
                className={`transition-colors hover:opacity-80 ${
                  pathnames.length === 0
                    ? "font-bold text-lg bg-gradient-to-r from-emerald-600 to-teal-600 bg-clip-text text-transparent"
                    : "text-default-500 hover:text-default-900"
                }`}
              >
                {pathnames.length === 0 ? "LeviLauncher" : "Home"}
              </Link>
            )}
            {pathnames.map((value, index) => {
              const to = `/${pathnames.slice(0, index + 1).join("/")}`;
              const isLast = index === pathnames.length - 1;
              const name = getName(value);
              const isNonClickable = NON_CLICKABLE_SEGMENTS.includes(value);

              return (
                <React.Fragment key={to}>
                  <IoChevronForward className="mx-1 text-default-400 shrink-0" />
                  {isLast || navLocked || isNonClickable ? (
                    <span
                      className={`${
                        isLast
                          ? "font-bold text-default-900"
                          : "text-default-500"
                      } truncate`}
                    >
                      {name}
                    </span>
                  ) : (
                    <Link
                      to={to}
                      className="text-default-500 hover:text-default-900 transition-colors truncate"
                    >
                      {name}
                    </Link>
                  )}
                </React.Fragment>
              );
            })}
          </nav>
        </div>

        <div className="flex items-center gap-2 shrink-0">
          <div className="hidden sm:block">
            <UserAvatar />
          </div>

          <WindowControls
            navLocked={navLocked}
            isOnboardingMode={isOnboardingMode}
          />
        </div>
      </motion.div>
    </AnimatePresence>
  );
};
