import { Button, Tooltip } from "@heroui/react";

import React from "react";

import { AnimatePresence, motion } from "framer-motion";
import { useTranslation } from "react-i18next";
import { LuChevronDown, LuPanelLeftClose, LuPanelLeftOpen } from "react-icons/lu";
import { ThemeSwitcher } from "@/components/ThemeSwitcher";
import {
  useNavPress,
  usePrimaryNav,
  type ChildNavItem,
  type IconComponent,
} from "@/components/ShellControls";
import { cn } from "@/utils/cn";

interface SidebarProps {
  navLocked: boolean;
  themeMode: string;
  tryNavigate: (path: string | number) => void;
  /** Labeled rail instead of icons with tooltips. */
  expanded: boolean;
  onToggleExpanded: () => void;
}

const HOME_GROUP_OPEN_KEY = "ll.sidebar.homeOpen";

const readHomeGroupOpen = () => {
  try {
    return localStorage.getItem(HOME_GROUP_OPEN_KEY) !== "false";
  } catch {
    return true;
  }
};

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

/** Marks the page being shown; one pill slides between items. */
const ActivePill: React.FC<{ expanded: boolean }> = ({ expanded }) => (
  <motion.div
    layoutId="active-pill"
    className={cn(
      "absolute top-1/2 -translate-y-1/2 w-1 bg-brand-500 rounded-r-full",
      expanded ? "-left-3 h-5" : "-left-2 h-8",
    )}
    transition={{ type: "spring", stiffness: 300, damping: 30 }}
  />
);

const RailItem: React.FC<{
  label: string;
  Icon: IconComponent;
  iconClassName?: string;
  isActive: boolean;
  isCurrent: boolean;
  isChild?: boolean;
  expanded: boolean;
  isDisabled: boolean;
  onPress: (e: { pointerType: string; target: EventTarget }) => void;
}> = ({
  label,
  Icon,
  iconClassName,
  isActive,
  isCurrent,
  isChild,
  expanded,
  isDisabled,
  onPress,
}) => (
  <RailTooltip label={label} expanded={expanded}>
    <div className={cn("relative", expanded && "w-full min-w-0")}>
      {isCurrent && <ActivePill expanded={expanded} />}
      <Button
        size={isChild ? "sm" : "md"}
        isIconOnly={!expanded}
        aria-label={expanded ? undefined : label}
        aria-current={isCurrent ? "page" : undefined}
        isDisabled={isDisabled}
        onPress={onPress}
        variant={isActive && !isChild ? "secondary" : "ghost"}
        className={cn(
          "transition-colors duration-200",
          isChild
            ? expanded
              ? "w-full h-8 justify-start gap-2.5 rounded-lg pl-9 pr-3 text-sm"
              : "size-8 min-w-8 rounded-lg"
            : expanded
              ? "w-full h-10 justify-start gap-3 rounded-xl px-3 font-medium"
              : "size-12 min-w-12 rounded-xl",
          itemClass(isActive),
        )}
      >
        <Icon
          size={isChild ? (expanded ? 14 : 15) : expanded ? 18 : 20}
          className={cn("shrink-0", isChild && !isActive && iconClassName)}
        />
        {expanded && <span className="truncate">{label}</span>}
      </Button>
    </div>
  </RailTooltip>
);

export const Sidebar: React.FC<SidebarProps> = ({
  navLocked,
  themeMode,
  tryNavigate,
  expanded,
  onToggleExpanded,
}) => {
  const { t } = useTranslation();
  const { items, homeGroups } = usePrimaryNav();
  const [homeOpen, setHomeOpen] = React.useState(readHomeGroupOpen);
  const go = useNavPress(tryNavigate);

  const toggleHomeGroup = () => {
    setHomeOpen((open) => {
      try {
        localStorage.setItem(HOME_GROUP_OPEN_KEY, String(!open));
      } catch {}
      return !open;
    });
  };

  const activeChild = homeGroups
    .flatMap((group) => group.items)
    .find((item) => item.isActive);
  const showManualTheme =
    themeMode !== "auto" && themeMode !== "schedule" && themeMode !== "system";
  const toggleLabel = expanded
    ? t("shell.nav.collapse_sidebar")
    : t("shell.nav.expand_sidebar");

  const renderChild = (child: ChildNavItem) => (
    <li key={child.key} className={cn(expanded && "w-full")}>
      <RailItem
        label={child.label}
        Icon={child.Icon}
        iconClassName={child.iconClassName}
        isActive={child.isActive}
        isCurrent={child.isActive}
        isChild
        expanded={expanded}
        isDisabled={navLocked}
        onPress={go(child.path)}
      />
    </li>
  );

  return (
    <aside className="fixed left-0 top-[var(--topbar-h)] bottom-0 z-50 flex flex-col w-[var(--rail-w)]">
      <nav
        aria-label={t("audit.usability.main_nav")}
        className={cn(
          "flex-1 flex flex-col w-full overflow-y-auto overflow-x-hidden scrollbar-hide",
          // The icon rail keeps the spacing released versions had.
          // It tightens while Home's pages are listed so every destination
          // still fits.
          expanded
            ? "gap-1 px-3 py-3"
            : cn("items-center px-2", activeChild ? "gap-2 py-2" : "gap-4 py-4"),
        )}
      >
        {items.map((item) => {
          const isHome = item.key === "home";
          // The labeled rail lets the user fold Home's pages away. The icon
          // rail stays as released versions had it, and lists Home's pages
          // only while one of them is open, so the way back stays visible.
          const showChildren = isHome && (expanded ? homeOpen : Boolean(activeChild));
          const homeLabel = homeGroups.map((group) => group.label).join(", ");
          return (
            <React.Fragment key={item.key}>
              <div className={cn("flex items-center gap-0.5", expanded && "w-full")}>
                <div className={cn(expanded && "flex-1 min-w-0")}>
                  <RailItem
                    label={item.label}
                    Icon={item.Icon}
                    isActive={item.isActive}
                    isCurrent={item.isActive && !(isHome && activeChild)}
                    expanded={expanded}
                    isDisabled={navLocked}
                    onPress={go(item.path)}
                  />
                </div>
                {isHome && expanded && (
                  <Button
                    isIconOnly
                    size="sm"
                    variant="ghost"
                    aria-expanded={homeOpen}
                    aria-controls="sidebar-home-pages"
                    aria-label={t("shell.nav.toggle_group", { name: homeLabel })}
                    onPress={toggleHomeGroup}
                    className="size-8 min-w-8 shrink-0 rounded-lg text-zinc-500 hover:bg-surface-secondary dark:text-zinc-400"
                  >
                    <LuChevronDown
                      size={16}
                      className={cn(
                        "transition-transform duration-200",
                        !homeOpen && "-rotate-90",
                      )}
                    />
                  </Button>
                )}
              </div>
              <AnimatePresence initial={false}>
                {showChildren && (
                  <motion.div
                    id="sidebar-home-pages"
                    initial={{ height: 0, opacity: 0 }}
                    animate={{ height: "auto", opacity: 1 }}
                    exit={{ height: 0, opacity: 0 }}
                    transition={{ duration: 0.2 }}
                    className={cn(
                      "flex flex-col gap-1 overflow-hidden shrink-0",
                      expanded ? "w-full" : "items-center",
                    )}
                  >
                    {/* The icon rail lists only the open page's group. */}
                    {homeGroups
                      .filter(
                        (group) =>
                          expanded || group.items.some((child) => child.isActive),
                      )
                      .map((group) => (
                      <div
                        key={group.key}
                        className={cn(
                          "flex flex-col gap-0.5",
                          expanded ? "w-full" : "items-center",
                        )}
                      >
                        {expanded ? (
                          <span
                            aria-hidden="true"
                            className="pl-9 pr-3 pt-1.5 pb-0.5 text-[0.6875rem] font-semibold uppercase tracking-wide text-muted/80 truncate"
                          >
                            {group.label}
                          </span>
                        ) : (
                          <div
                            aria-hidden="true"
                            className="mb-1 h-px w-6 bg-border"
                          />
                        )}
                        <ul
                          aria-label={group.label}
                          className={cn(
                            "flex flex-col gap-0.5",
                            expanded ? "w-full" : "items-center",
                          )}
                        >
                          {group.items.map(renderChild)}
                        </ul>
                      </div>
                      ))}
                    {!expanded && (
                      <div
                        aria-hidden="true"
                        className="mt-1 h-px w-6 bg-border"
                      />
                    )}
                  </motion.div>
                )}
              </AnimatePresence>
            </React.Fragment>
          );
        })}
      </nav>

      <div
        className={cn(
          "flex w-full",
          expanded
            ? "gap-1 px-3 pb-3 pt-2 flex-row items-center"
            : "gap-4 flex-col items-center px-2 pb-6 pt-2",
        )}
      >
        <RailTooltip label={toggleLabel} expanded={false}>
          <Button
            isIconOnly={!expanded}
            aria-label={expanded ? undefined : toggleLabel}
            aria-expanded={expanded}
            onPress={onToggleExpanded}
            variant="ghost"
            className={cn(
              "transition-colors duration-200 text-zinc-500 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100 hover:bg-surface-secondary",
              expanded
                ? "flex-1 min-w-0 h-10 justify-start gap-3 rounded-xl px-3 text-sm font-normal"
                : "size-12 min-w-12 rounded-xl",
            )}
          >
            {expanded ? (
              <LuPanelLeftClose size={18} className="shrink-0" />
            ) : (
              <LuPanelLeftOpen size={20} />
            )}
            {expanded && <span className="truncate">{toggleLabel}</span>}
          </Button>
        </RailTooltip>
        {showManualTheme && (
          <RailTooltip label={t("theme.toggle")} expanded={false}>
            <div className="shrink-0">
              <ThemeSwitcher
                isDisabled={navLocked}
                className={cn(expanded ? "size-10 min-w-10" : "size-12 min-w-12", "rounded-xl text-zinc-500 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100 hover:bg-surface-secondary transition-colors duration-200")}
                iconSize={expanded ? 18 : 20}
              />
            </div>
          </RailTooltip>
        )}
      </div>
    </aside>
  );
};
