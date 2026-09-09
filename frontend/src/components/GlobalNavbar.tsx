import { Button, Dropdown, Label, Tooltip } from "@heroui/react";
import { cn } from "@/utils/cn";

import React from "react";
import { useLocation } from "react-router-dom";

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
import { useTranslation } from "react-i18next";
import { LAYOUT } from "@/constants/layout";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { ROUTES, isRouteActive } from "@/constants/routes";

interface GlobalNavbarProps {
  isBeta: boolean;
  navLocked: boolean;
  themeMode: string;
  isOnboardingMode: boolean;
  tryNavigate: (path: string | number) => void;
}

export const GlobalNavbar: React.FC<GlobalNavbarProps> = ({
  isBeta,
  navLocked,
  themeMode,
  isOnboardingMode,
  tryNavigate,
}) => {
  const { t } = useTranslation();
  const location = useLocation();

  const navItems = [
    {
      key: "home",
      label: t("launcherpage.launch_button"),
      path: ROUTES.home,
      icon: <FaRocket size={18} />,
      navbarClass: "flex",
    },
    {
      key: "download",
      label: t("downloadmodal.download_button"),
      path: ROUTES.download,
      icon: <FaDownload size={18} />,
      navbarClass: "flex",
    },
    {
      key: "versions",
      label: t("nav.versions"),
      path: ROUTES.instances,
      icon: <FaList size={18} />,
      navbarClass: "hidden lg:flex",
      menuClass: "flex lg:hidden",
      breakpoint: "lg",
    },
    {
      key: "about",
      label: t("nav.about"),
      path: ROUTES.about,
      icon: <FaInfoCircle size={18} />,
      navbarClass: "hidden lg:flex",
      menuClass: "flex lg:hidden",
      breakpoint: "lg",
    },
    {
      key: "settings",
      label: t("app.settings"),
      path: ROUTES.settings,
      icon: <FaCog size={18} />,
      navbarClass: "flex",
    },
  ];

  const extraItems = navItems.filter((item) => item.menuClass);

  const activeExtraItem = extraItems.find((item) =>
    isRouteActive(location.pathname, item.path),
  );

  const moreButtonClass = React.useMemo(() => {
    const base = "min-w-0 px-3 h-10 rounded-xl transition-all duration-200";
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
    <div
      id="wails-draggable"
      className="fixed top-0 left-0 right-0 z-50 flex justify-center"
    >
      <div
        className={`pointer-events-auto w-full ${LAYOUT.NAVBAR_BG} border-b border-border/50 dark:border-zinc-800/50 shadow-sm dark:shadow-zinc-950/20 px-4 py-2 flex items-center gap-4`}
      >
        <div className="flex items-center gap-3 shrink-0">
          <Button
            isIconOnly
            size="sm"
            onPress={() => tryNavigate(-1)}
            isDisabled={navLocked}
            aria-label={t("nav.back")}
            variant={"ghost"}
            className={cn(
              "rounded-lg",
              "wails-no-drag text-muted dark:text-zinc-400",
            )}
          >
            <IoArrowBack size={20} />
          </Button>
          <div className="p-1">
            <LeviIcon width={28} height={28} />
          </div>
          <div className="hidden sm:flex flex-col leading-none gap-0.5">
            <p className="font-bold text-[16px] tracking-tight text-brand-700 dark:text-brand-300">
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
            const isActive = isRouteActive(location.pathname, item.path);
            return (
              <Tooltip key={item.key} delay={500} closeDelay={0}>
                <Button
                  aria-label={item.label}
                  isDisabled={navLocked}
                  onPress={(e) => {
                    tryNavigate(item.path);
                    if (e.pointerType !== "keyboard") {
                      (e.target as HTMLElement).blur();
                    }
                  }}
                  variant={isActive ? "secondary" : "ghost"}
                  className={`min-w-0 px-3 h-10 rounded-xl transition-all duration-200 ${item.navbarClass} ${
                    isActive
                      ? "bg-zinc-100 dark:bg-zinc-800 text-zinc-900 dark:text-zinc-100 font-medium"
                      : "text-zinc-700 hover:text-black dark:text-zinc-300 dark:hover:text-white font-medium"
                  }`}
                >
                  {
                    <span>{item.icon}</span>
                  }
                  <span className="hidden md:inline">{item.label}</span>
                </Button>
                <Tooltip.Content>{item.label}</Tooltip.Content>
              </Tooltip>
            );
          })}

          <div className="lg:hidden">
            <Dropdown>
              <Button
                aria-label={t("nav.more")}
                isDisabled={navLocked}
                variant={"ghost"}
                className={moreButtonClass}
              >
                {<FaEllipsisH size={18} />}
                <span className="hidden md:inline">{t("nav.more")}</span>
              </Button>
              <Dropdown.Popover className={COMPONENT_STYLES.dropdown.content}>
                <Dropdown.Menu
                  aria-label={t("nav.more")}
                  onAction={(key) => {
                    const item = extraItems.find((i) => i.key === key);
                    if (item) tryNavigate(item.path);
                  }}
                >
                  {extraItems.map((item) => (
                    <Dropdown.Item
                      key={item.key}
                      className={item.menuClass}
                      id={item.key}
                      textValue={item.label}
                    >
                      {React.cloneElement(item.icon as any, {
                        size: 14,
                      })}
                      <Label>{item.label}</Label>
                      <Dropdown.ItemIndicator />
                    </Dropdown.Item>
                  ))}
                </Dropdown.Menu>
              </Dropdown.Popover>
            </Dropdown>
          </div>
        </div>

        <div className="flex items-center gap-3 shrink-0 justify-end">
          <div className="h-8 w-px bg-surface-tertiary dark:bg-zinc-700 mx-1 hidden sm:block" />

          {themeMode !== "auto" &&
            themeMode !== "schedule" &&
            themeMode !== "system" && <ThemeSwitcher isDisabled={navLocked} />}
          <div
            className={`hidden sm:block ${navLocked ? "pointer-events-none opacity-50" : ""}`}
          >
            <UserAvatar />
          </div>

          <WindowControls
            navLocked={navLocked}
            isOnboardingMode={isOnboardingMode}
          />
        </div>
      </div>
    </div>
  );
};
