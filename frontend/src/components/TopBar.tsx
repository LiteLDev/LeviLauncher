import { Button, Tooltip } from "@heroui/react";
import { cn } from "@/utils/cn";

import React from "react";
import { WindowControls } from "./WindowControls";
import { UserAvatar } from "@/components/UserAvatar";

import { IoArrowBack, IoArrowForward, IoChevronForward } from "react-icons/io5";
import { useLocation, Link } from "react-router-dom";
import { LeviIcon } from "@/icons/LeviIcon";
import { useTranslation } from "react-i18next";
import { useNavigationHistory } from "@/utils/NavigationHistoryContext";
import { LAYOUT } from "@/constants/layout";
import { ROUTES } from "@/constants/routes";
import { getRouteLabelKey, ROUTE_LABEL_KEYS } from "@/constants/routeLabels";

interface TopBarProps {
  navLocked: boolean;
  isOnboardingMode: boolean;
  tryNavigate: (path: string | number) => void;
}

export const TopBar: React.FC<TopBarProps> = ({
  navLocked,
  isOnboardingMode,
  tryNavigate,
}) => {
  const { t } = useTranslation();
  const location = useLocation();
  const { canGoBack, canGoForward, getBackEntry, getForwardEntry, currentTitle } =
    useNavigationHistory();

  const pathnames = location.pathname.split("/").filter((x) => x);

  const getHistoryTitle = (entry: ReturnType<typeof getBackEntry>, fallback: string) =>
    entry ? entry.title || t(getRouteLabelKey(entry.pathname)) : t(fallback);

  return (
    <>
      <div
        id="wails-draggable"
        className={`fixed top-0 right-0 left-0 h-14 z-[60] flex items-center justify-between pr-4 ${LAYOUT.NAVBAR_BG}`}
      >
        <div className="absolute bottom-0 right-0 left-[calc(3.5rem+20px)] h-px bg-surface-tertiary/50 dark:bg-zinc-800/50" />
        <div className="flex items-center gap-2 overflow-hidden">
          <div className="w-14 shrink-0 flex items-center justify-center">
            <LeviIcon width={32} height={32} />
          </div>

          <div className="flex items-center gap-1 shrink-0">
            <Tooltip
              delay={500}
              closeDelay={0}
              isDisabled={!canGoBack || navLocked}
            >
              <Button
                isIconOnly
                size="sm"
                onPress={() => tryNavigate(-1)}
                isDisabled={navLocked || !canGoBack}
                aria-label={t("nav.back")}
                variant={"ghost"}
                className={cn(
                  "rounded-lg",
                  "wails-no-drag text-muted dark:text-zinc-400",
                )}
              >
                <IoArrowBack size={20} />
              </Button>
              <Tooltip.Content>
                {getHistoryTitle(getBackEntry(), "nav.back")}
              </Tooltip.Content>
            </Tooltip>
            <Tooltip
              delay={500}
              closeDelay={0}
              isDisabled={!canGoForward || navLocked}
            >
              <Button
                isIconOnly
                size="sm"
                onPress={() => tryNavigate(1)}
                isDisabled={navLocked || !canGoForward}
                aria-label={t("nav.forward")}
                variant={"ghost"}
                className={cn(
                  "rounded-lg",
                  "wails-no-drag text-muted dark:text-zinc-400",
                )}
              >
                <IoArrowForward size={20} />
              </Button>
              <Tooltip.Content>
                {getHistoryTitle(getForwardEntry(), "nav.forward")}
              </Tooltip.Content>
            </Tooltip>
          </div>

          <div className="w-px h-5 bg-surface-quaternary/50 mx-1 shrink-0" />

          <nav
            aria-label={t("nav.breadcrumb")}
            className="flex items-center text-sm font-medium whitespace-nowrap overflow-hidden text-ellipsis min-w-0"
          >
            {navLocked ? (
              <span
                className={`${
                  pathnames.length === 0
                    ? "font-bold text-lg text-brand-700 dark:text-brand-300"
                    : "text-muted dark:text-zinc-400"
                }`}
              >
                {pathnames.length === 0 ? "LeviLauncher" : t("nav.home")}
              </span>
            ) : (
              <Link
                to={ROUTES.home}
                onClick={(e) => {
                  e.preventDefault();
                  tryNavigate(ROUTES.home);
                }}
                className={`transition-colors hover:opacity-80 ${
                  pathnames.length === 0
                    ? "font-bold text-lg text-brand-700 dark:text-brand-300"
                    : "text-muted dark:text-zinc-400 hover:text-foreground dark:hover:text-zinc-200"
                }`}
              >
                {pathnames.length === 0 ? "LeviLauncher" : t("nav.home")}
              </Link>
            )}
            {pathnames.map((_, index) => {
              const to = `/${pathnames.slice(0, index + 1).join("/")}`;
              const isLast = index === pathnames.length - 1;
              const name = isLast && currentTitle ? currentTitle : t(getRouteLabelKey(to));
              const isNonClickable = !ROUTE_LABEL_KEYS[to];

              return (
                <React.Fragment key={to}>
                  <IoChevronForward className="mx-1 text-muted shrink-0" />
                  {isLast || navLocked || isNonClickable ? (
                    <span
                      title={name}
                      aria-current={isLast ? "page" : undefined}
                      className={`${
                        isLast
                          ? "font-bold text-foreground dark:text-zinc-100"
                          : "text-muted dark:text-zinc-400"
                      } truncate`}
                    >
                      {name}
                    </span>
                  ) : (
                    <Link
                      to={to}
                      title={name}
                      onClick={(e) => {
                        e.preventDefault();
                        tryNavigate(to);
                      }}
                      className="text-muted dark:text-zinc-400 hover:text-foreground dark:hover:text-zinc-200 transition-colors truncate"
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
          <div
            className={`hidden sm:block ${navLocked ? "pointer-events-none opacity-50" : ""}`}
          >
            <UserAvatar />
          </div>

          <div
            inert
            className="opacity-0 pointer-events-none"
            aria-hidden="true"
          >
            <WindowControls
              navLocked={navLocked}
              isOnboardingMode={isOnboardingMode}
            />
          </div>
        </div>
      </div>

      <div className="fixed top-0 right-0 h-14 z-[80] flex items-center justify-end pr-4 pointer-events-none">
        <div className="pointer-events-auto">
          <WindowControls
            navLocked={navLocked}
            isOnboardingMode={isOnboardingMode}
            hideSeparator
            className="text-foreground dark:text-zinc-300 [&_button]:text-foreground dark:[&_button]:text-zinc-300 [&_button:hover]:text-foreground dark:[&_button:hover]:text-zinc-100"
          />
        </div>
      </div>
    </>
  );
};
