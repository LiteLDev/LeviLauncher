import { Button, Tooltip } from "@heroui/react";
import { cn } from "@/utils/cn";

import React from "react";

import { ThemeSwitcher } from "@/components/ThemeSwitcher";
import { UserAvatar } from "@/components/UserAvatar";
import { AdministratorBadge } from "@/components/AdministratorBadge";
import { LeviIcon } from "@/icons/LeviIcon";
import { WindowControls } from "@/components/WindowControls";
import {
  ActivityButton,
  CommandPaletteButton,
  ShellInstancePicker,
  useNavPress,
  usePrimaryNav,
} from "@/components/ShellControls";
import { IoArrowBack } from "react-icons/io5";
import { useTranslation } from "react-i18next";
import { LAYOUT } from "@/constants/layout";

interface GlobalNavbarProps {
  isBeta: boolean;
  navLocked: boolean;
  themeMode: string;
  isOnboardingMode: boolean;
  tryNavigate: (path: string | number) => void;
}

const itemClass = (isActive: boolean) =>
  isActive
    ? "bg-surface-secondary text-zinc-900 dark:text-zinc-100"
    : "text-zinc-700 hover:text-black dark:text-zinc-300 dark:hover:text-white";

export const GlobalNavbar: React.FC<GlobalNavbarProps> = ({
  isBeta,
  navLocked,
  themeMode,
  isOnboardingMode,
  tryNavigate,
}) => {
  const { t } = useTranslation();
  const { items, catalogs } = usePrimaryNav();

  const go = useNavPress(tryNavigate);

  return (
    <div
      id="wails-draggable"
      className="fixed top-0 left-0 right-0 z-50 flex justify-center"
    >
      <div
        className={`pointer-events-auto w-full h-[var(--topbar-h)] ${LAYOUT.NAVBAR_BG} border-b border-border/50 dark:border-zinc-800/50 shadow-sm dark:shadow-zinc-950/20 px-2 sm:px-4 flex items-center gap-2 sm:gap-4`}
      >
        <div className="flex items-center gap-2 sm:gap-3 shrink-0">
          <Button
            isIconOnly
            size="sm"
            onPress={() => tryNavigate(-1)}
            isDisabled={navLocked}
            aria-label={t("nav.back")}
            variant={"ghost"}
            className={cn(
              "rounded-full",
              "wails-no-drag text-muted dark:text-zinc-400",
            )}
          >
            <IoArrowBack size={20} />
          </Button>
          <LeviIcon width={28} height={28} />
          {isBeta && (
            <span className="lg:hidden text-[0.625rem] font-bold text-amber-500 uppercase tracking-wider">
              Beta
            </span>
          )}
          <div className="hidden lg:flex flex-col leading-none gap-0.5">
            <p className="font-bold text-base tracking-tight text-brand-700 dark:text-brand-300">
              LeviLauncher
            </p>
            <div className="flex items-center gap-2">
              {isBeta && (
                <span className="text-[0.625rem] font-bold text-amber-500 uppercase tracking-wider">
                  Beta
                </span>
              )}
              <AdministratorBadge />
            </div>
          </div>
        </div>

        <nav
          aria-label={t("audit.usability.main_nav")}
          // Scrolls instead of overlapping the right-hand controls if a
          // locale or an extra badge leaves too little room.
          className="flex-1 min-w-0 flex items-center justify-[safe_center] gap-0.5 sm:gap-1 whitespace-nowrap overflow-x-auto scrollbar-hide py-1"
        >
          {items.map((item) => (
            <React.Fragment key={item.key}>
              <Tooltip delay={300} closeDelay={0}>
                <Button
                  aria-label={item.label}
                  aria-current={item.isActive ? "page" : undefined}
                  isDisabled={navLocked}
                  onPress={go(item.path)}
                  variant={item.isActive ? "secondary" : "ghost"}
                  className={cn(
                    "min-w-0 h-9 px-2.5 xl:px-3 rounded-xl font-medium transition-colors duration-200",
                    itemClass(item.isActive),
                  )}
                >
                  <item.Icon size={17} />
                  <span className="hidden xl:inline">{item.label}</span>
                </Button>
                <Tooltip.Content>{item.label}</Tooltip.Content>
              </Tooltip>
              {/* Both catalogs stay one click away while Discover is open. */}
              {item.key === "discover" && item.isActive && (
                <div
                  role="group"
                  aria-label={item.label}
                  className="flex items-center gap-0.5 rounded-xl bg-surface-secondary/60 p-0.5"
                >
                  {catalogs.map((catalog) => (
                    <Tooltip key={catalog.key} delay={300} closeDelay={0}>
                      <Button
                        size="sm"
                        isIconOnly
                        aria-label={catalog.label}
                        aria-current={catalog.isActive ? "page" : undefined}
                        isDisabled={navLocked}
                        onPress={go(catalog.path)}
                        variant="ghost"
                        className={cn(
                          "size-8 min-w-8 rounded-lg",
                          catalog.isActive && "bg-surface shadow-sm",
                        )}
                      >
                        <catalog.Icon size={15} className={catalog.iconClassName} />
                      </Button>
                      <Tooltip.Content>{catalog.label}</Tooltip.Content>
                    </Tooltip>
                  ))}
                </div>
              )}
            </React.Fragment>
          ))}
        </nav>

        <div className="flex items-center gap-1 sm:gap-2 shrink-0 justify-end">
          <CommandPaletteButton isDisabled={navLocked} density="navbar" />
          <ShellInstancePicker isDisabled={navLocked} density="navbar" />
          <ActivityButton />
          {themeMode !== "auto" &&
            themeMode !== "schedule" &&
            themeMode !== "system" && (
              <div className="hidden md:block">
                <ThemeSwitcher isDisabled={navLocked} />
              </div>
            )}
          <div
            className={`hidden md:block ${navLocked ? "pointer-events-none opacity-50" : ""}`}
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
