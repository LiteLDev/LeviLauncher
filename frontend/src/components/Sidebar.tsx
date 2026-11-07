import { Button, Tooltip } from "@heroui/react";

import React from "react";

import { AnimatePresence, motion } from "framer-motion";
import { useTranslation } from "react-i18next";
import { ThemeSwitcher } from "@/components/ThemeSwitcher";
import { useNavPress, usePrimaryNav } from "@/components/ShellControls";
import { cn } from "@/utils/cn";

interface SidebarProps {
  navLocked: boolean;
  themeMode: string;
  tryNavigate: (path: string | number) => void;
  /** Labeled rail (wide windows) instead of icons with tooltips. */
  expanded: boolean;
}

const itemClass = (isActive: boolean) =>
  isActive
    ? "bg-brand-500/10 text-brand-600 dark:text-brand-400"
    : "text-zinc-500 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100 hover:bg-surface-secondary";

/** Wraps a rail control in a tooltip only while labels are hidden. */
const RailTooltip: React.FC<{
  label: string;
  expanded: boolean;
  children: React.ReactNode;
}> = ({ label, expanded, children }) => (
  <Tooltip delay={0} closeDelay={0} isDisabled={expanded}>
    {children}
    <Tooltip.Content placement="right">{label}</Tooltip.Content>
  </Tooltip>
);

export const Sidebar: React.FC<SidebarProps> = ({
  navLocked,
  themeMode,
  tryNavigate,
  expanded,
}) => {
  const { t } = useTranslation();
  const { items, catalogs } = usePrimaryNav();

  const go = useNavPress(tryNavigate);

  return (
    <aside
      className="fixed left-0 top-[var(--topbar-h)] bottom-0 z-50 flex flex-col w-[var(--rail-w)]"
    >
      <nav
        aria-label={t("audit.usability.main_nav")}
        className={cn(
          "flex-1 flex flex-col gap-1 w-full overflow-y-auto overflow-x-hidden scrollbar-hide py-3",
          expanded ? "px-3" : "items-center px-1.5",
        )}
      >
        {items.map((item) => {
          const isDiscover = item.key === "discover";
          // The labeled rail always lists both catalogs; the icon rail shows
          // them once Discover is open, so either is one click away there.
          const showCatalogs = isDiscover && (expanded || item.isActive);
          return (
            <React.Fragment key={item.key}>
              <RailTooltip label={item.label} expanded={expanded}>
                <div className={cn("relative group", expanded && "w-full")}>
                  {item.isActive && (
                    <motion.div
                      layoutId="active-pill"
                      className={cn(
                        "absolute top-1/2 -translate-y-1/2 w-1 h-6 bg-brand-500 rounded-r-full",
                        expanded ? "-left-3" : "-left-1.5",
                      )}
                      transition={{
                        type: "spring",
                        stiffness: 300,
                        damping: 30,
                      }}
                    />
                  )}
                  <Button
                    isIconOnly={!expanded}
                    aria-label={expanded ? undefined : item.label}
                    aria-current={item.isActive ? "page" : undefined}
                    isDisabled={navLocked}
                    onPress={go(item.path)}
                    variant={item.isActive ? "secondary" : "ghost"}
                    className={cn(
                      "rounded-xl transition-colors duration-200",
                      expanded
                        ? "w-full h-10 justify-start gap-3 px-3 font-medium"
                        : "size-11 min-w-11",
                      itemClass(item.isActive),
                    )}
                  >
                    <item.Icon size={18} />
                    {expanded && <span className="truncate">{item.label}</span>}
                  </Button>
                </div>
              </RailTooltip>
              <AnimatePresence initial={false}>
                {showCatalogs && (
                  <motion.ul
                    aria-label={item.label}
                    initial={{ height: 0, opacity: 0 }}
                    animate={{ height: "auto", opacity: 1 }}
                    exit={{ height: 0, opacity: 0 }}
                    transition={{ duration: 0.2 }}
                    className={cn(
                      "flex flex-col gap-1 overflow-hidden",
                      expanded ? "w-full pl-6" : "items-center",
                    )}
                  >
                    {catalogs.map((catalog) => (
                      <li key={catalog.key} className={cn(expanded && "w-full")}>
                        <RailTooltip label={catalog.label} expanded={expanded}>
                          <Button
                            size="sm"
                            isIconOnly={!expanded}
                            aria-label={expanded ? undefined : catalog.label}
                            aria-current={catalog.isActive ? "page" : undefined}
                            isDisabled={navLocked}
                            onPress={go(catalog.path)}
                            variant="ghost"
                            className={cn(
                              "rounded-lg transition-colors duration-200",
                              expanded
                                ? "w-full h-8 justify-start gap-2.5 px-3 text-sm"
                                : "size-9 min-w-9",
                              itemClass(catalog.isActive),
                            )}
                          >
                            <catalog.Icon
                              size={expanded ? 14 : 16}
                              className={catalog.iconClassName}
                            />
                            {expanded && (
                              <span className="truncate">{catalog.label}</span>
                            )}
                          </Button>
                        </RailTooltip>
                      </li>
                    ))}
                  </motion.ul>
                )}
              </AnimatePresence>
            </React.Fragment>
          );
        })}
      </nav>

      {themeMode !== "auto" &&
        themeMode !== "schedule" &&
        themeMode !== "system" && (
          <div
            className={cn(
              "flex flex-col gap-2 pb-3 w-full",
              expanded ? "px-3" : "items-center px-1.5",
            )}
          >
            <RailTooltip label={t("theme.toggle")} expanded={false}>
              <div>
                <ThemeSwitcher
                  isDisabled={navLocked}
                  className="size-11 min-w-11 rounded-xl text-zinc-500 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100 hover:bg-surface-secondary transition-colors duration-200"
                  iconSize={18}
                />
              </div>
            </RailTooltip>
          </div>
        )}
    </aside>
  );
};
