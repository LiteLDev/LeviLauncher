import { Button, Popover, Tooltip } from "@heroui/react";

import React, { useEffect } from "react";
import { Navigate, useLocation } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  FaCog,
  FaCompass,
  FaCube,
  FaDownload,
  FaFolderOpen,
  FaHome,
  FaLayerGroup,
  FaPuzzlePiece,
} from "react-icons/fa";
import { SiCurseforge } from "react-icons/si";
import { IoNotificationsOutline, IoSearch } from "react-icons/io5";
import { InstancePicker } from "@/components/InstancePicker";
import {
  DISCOVER_CATALOGS,
  PRIMARY_NAV,
  ROUTES,
  getActiveCatalog,
  getActiveNavKey,
  type DiscoverCatalog,
  type PrimaryNavKey,
} from "@/constants/routes";
import { openActivityCenter, useUnreadActivityCount } from "@/utils/activity";
import { useCurrentVersion } from "@/utils/CurrentVersionContext";
import { useMediaQuery } from "@/hooks/useLayoutMode";
import { cn } from "@/utils/cn";

const LAST_CATALOG_KEY = "ll.discover.lastCatalog";

const readLastCatalog = (): DiscoverCatalog => {
  try {
    return localStorage.getItem(LAST_CATALOG_KEY) === "curseforge"
      ? "curseforge"
      : "lip";
  } catch {
    return "lip";
  }
};

const catalogPath = (catalog: DiscoverCatalog) =>
  DISCOVER_CATALOGS.find((item) => item.key === catalog)?.path ?? ROUTES.lip;

/** `/discover` resumes the catalog the user browsed last. */
export const DiscoverRedirect = () => (
  <Navigate to={catalogPath(readLastCatalog())} replace />
);

type IconComponent = React.ComponentType<{ size?: number; className?: string }>;

const NAV_ICONS: Record<PrimaryNavKey, IconComponent> = {
  home: FaHome,
  instances: FaLayerGroup,
  mods: FaPuzzlePiece,
  content: FaFolderOpen,
  discover: FaCompass,
  download: FaDownload,
  settings: FaCog,
};

const NAV_LABEL_KEYS: Record<PrimaryNavKey, string> = {
  home: "nav.home",
  instances: "nav.versions",
  mods: "nav.mods",
  content: "shell.nav.content",
  discover: "shell.nav.discover",
  download: "downloadmodal.download_button",
  settings: "app.settings",
};

export type PrimaryNavItem = {
  key: PrimaryNavKey;
  label: string;
  path: string;
  Icon: IconComponent;
  isActive: boolean;
};

export type CatalogNavItem = {
  key: DiscoverCatalog;
  label: string;
  path: string;
  Icon: IconComponent;
  iconClassName: string;
  isActive: boolean;
};

/** Primary destinations with labels, icons and the active state for the
 * current route, shared by the sidebar and the navbar layout. */
export const usePrimaryNav = () => {
  const { t } = useTranslation();
  const { pathname } = useLocation();
  const activeKey = getActiveNavKey(pathname);
  const activeCatalog = getActiveCatalog(pathname);

  useEffect(() => {
    if (!activeCatalog) return;
    try {
      localStorage.setItem(LAST_CATALOG_KEY, activeCatalog);
    } catch {}
  }, [activeCatalog]);

  const items: PrimaryNavItem[] = PRIMARY_NAV.map((item) => ({
    key: item.key,
    label: t(NAV_LABEL_KEYS[item.key]),
    // Resume the catalog browsed last instead of a chooser page.
    path:
      item.key === "discover"
        ? catalogPath(activeCatalog ?? readLastCatalog())
        : item.path,
    Icon: NAV_ICONS[item.key],
    isActive: item.key === activeKey,
  }));

  const catalogs: CatalogNavItem[] = DISCOVER_CATALOGS.map((item) => ({
    key: item.key,
    label: item.key === "lip" ? t("lip.title") : t("curseforge.title"),
    path: item.path,
    Icon: item.key === "lip" ? FaCube : SiCurseforge,
    iconClassName: item.key === "lip" ? "text-green-500" : "text-[#f16436]",
    isActive: item.key === activeCatalog,
  }));

  return { items, catalogs, activeKey };
};

/** Where a control switches from its compact to its full form. The navbar
 * layout spends its middle on navigation, so it expands later. Class names
 * are spelled out in full so Tailwind can see them. */
export type ShellDensity = "topbar" | "navbar";

const SEARCH_BREAKPOINTS: Record<ShellDensity, { compact: string; full: string }> = {
  topbar: { compact: "lg:hidden", full: "hidden lg:flex" },
  navbar: { compact: "2xl:hidden", full: "hidden 2xl:flex" },
};

const PICKER_INLINE_QUERY: Record<ShellDensity, string> = {
  topbar: "(min-width: 768px)",
  navbar: "(min-width: 1280px)",
};

/** Press handler for nav buttons: navigate, and drop the focus ring a mouse
 * press leaves behind while keeping it for keyboard users. */
export const useNavPress =
  (tryNavigate: (path: string | number) => void) =>
  (path: string) =>
  (e: { pointerType: string; target: EventTarget }) => {
    tryNavigate(path);
    if (e.pointerType !== "keyboard") {
      (e.target as HTMLElement).blur();
    }
  };

/** The picker's value repeats the list row (name, game version, checkmark);
 * the bar has room for the name only. */
const pickerValueClass =
  "[&_[data-slot=select-value]_.text-xs]:hidden [&_[data-slot=select-value]_[data-slot=list-box-item-indicator]]:hidden";

const iconButtonClass =
  "wails-no-drag relative size-8 min-w-8 shrink-0 rounded-full text-muted dark:text-zinc-400";

const openCommandPalette = () =>
  window.dispatchEvent(new Event("ll:open-command-palette"));

/** Opens the command palette, which listens for `ll:open-command-palette`. */
export const CommandPaletteButton: React.FC<{
  isDisabled?: boolean;
  density?: ShellDensity;
}> = ({ isDisabled, density = "topbar" }) => {
  const { t } = useTranslation();
  const breakpoint = SEARCH_BREAKPOINTS[density];

  return (
    <>
      <Tooltip delay={300} closeDelay={0}>
        <Button
          isIconOnly
          size="sm"
          variant="ghost"
          isDisabled={isDisabled}
          aria-label={t("shell.search")}
          aria-keyshortcuts="Control+K"
          onPress={openCommandPalette}
          className={cn(iconButtonClass, breakpoint.compact)}
        >
          <IoSearch size={18} />
        </Button>
        <Tooltip.Content>{t("shell.search")} (Ctrl+K)</Tooltip.Content>
      </Tooltip>
      <Button
        size="sm"
        variant="ghost"
        isDisabled={isDisabled}
        aria-label={t("shell.search")}
        aria-keyshortcuts="Control+K"
        onPress={openCommandPalette}
        className={cn(
          "wails-no-drag h-8 min-w-0 shrink-0 gap-2 rounded-full border border-border/70 bg-surface/60 px-3 text-sm font-normal text-muted dark:border-white/10 dark:text-zinc-400",
          breakpoint.full,
        )}
      >
        <IoSearch size={16} />
        <span className="truncate">{t("shell.search_placeholder")}</span>
        <kbd className="ml-2 rounded-md border border-border/70 px-1.5 py-0.5 font-sans text-xs leading-none dark:border-white/10">
          Ctrl K
        </kbd>
      </Button>
    </>
  );
};

/** Bell that opens the Activity center, with the unread count as a badge. */
export const ActivityButton: React.FC = () => {
  const { t } = useTranslation();
  const unread = useUnreadActivityCount();
  const label =
    unread > 0
      ? t("shell.activity_unread", { count: unread })
      : t("activity.open");

  return (
    <Tooltip delay={300} closeDelay={0}>
      <Button
        isIconOnly
        size="sm"
        variant="ghost"
        aria-label={label}
        onPress={() => openActivityCenter()}
        className={iconButtonClass}
      >
        <IoNotificationsOutline size={19} />
        {unread > 0 && (
          <span
            aria-hidden="true"
            className="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-danger px-1 text-[0.625rem] font-semibold leading-none text-danger-foreground"
          >
            {unread > 99 ? "99+" : unread}
          </span>
        )}
      </Button>
      <Tooltip.Content>{label}</Tooltip.Content>
    </Tooltip>
  );
};

/** Current-instance switcher: inline on wide windows, and behind a compact
 * button on narrow ones so the bar never clips. */
export const ShellInstancePicker: React.FC<{
  isDisabled?: boolean;
  density?: ShellDensity;
}> = ({ isDisabled, density = "topbar" }) => {
  const { t } = useTranslation();
  const { currentVersionName } = useCurrentVersion();
  const [open, setOpen] = React.useState(false);
  const inline = useMediaQuery(PICKER_INLINE_QUERY[density]);
  const label = currentVersionName
    ? `${t("instance_picker.label")}: ${currentVersionName}`
    : t("instance_picker.label");

  if (inline) {
    return (
      <div className="wails-no-drag min-w-0 shrink">
        <InstancePicker
          size="sm"
          isDisabled={isDisabled}
          className={cn("w-40 xl:w-48", pickerValueClass)}
        />
      </div>
    );
  }

  return (
    <>
      <div className="wails-no-drag shrink-0">
        <Popover isOpen={open} onOpenChange={setOpen}>
          <Button
            isIconOnly
            size="sm"
            variant="ghost"
            isDisabled={isDisabled}
            aria-label={label}
            className={iconButtonClass}
          >
            <FaLayerGroup size={16} />
          </Button>
          <Popover.Content
            placement="bottom end"
            className="rounded-2xl border border-border/70 bg-overlay p-0 shadow-2xl dark:border-zinc-700/60"
          >
            <Popover.Dialog aria-label={t("instance_picker.label")} className="p-3">
              <InstancePicker
                label={t("instance_picker.label")}
                className={cn("w-64", pickerValueClass)}
                onChange={() => setOpen(false)}
              />
            </Popover.Dialog>
          </Popover.Content>
        </Popover>
      </div>
    </>
  );
};
