import { Button, Popover, Tooltip } from "@heroui/react";

import React, { useEffect } from "react";
import { Navigate, useLocation } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  FaCamera,
  FaCog,
  FaCogs,
  FaCube,
  FaDownload,
  FaGlobe,
  FaImage,
  FaInfoCircle,
  FaLayerGroup,
  FaList,
  FaPuzzlePiece,
  FaRocket,
  FaServer,
  FaUserTag,
} from "react-icons/fa";
import { SiCurseforge } from "react-icons/si";
import { IoNotificationsOutline, IoSearch } from "react-icons/io5";
import { InstancePicker } from "@/components/InstancePicker";
import {
  CONTENT_SECTIONS,
  DISCOVER_CATALOGS,
  PRIMARY_NAV,
  ROUTES,
  getActiveCatalog,
  getActiveContentSection,
  getActiveNavKey,
  isInstanceScopedRoute,
  type ContentSectionKey,
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

export type IconComponent = React.ComponentType<{
  size?: number;
  className?: string;
}>;

// Icons match released versions so existing users find their way.
const NAV_ICONS: Record<PrimaryNavKey, IconComponent> = {
  home: FaRocket,
  download: FaDownload,
  instances: FaList,
  about: FaInfoCircle,
  settings: FaCog,
};

const NAV_LABEL_KEYS: Record<PrimaryNavKey, string> = {
  home: "launcherpage.launch_button",
  download: "downloadmodal.download_button",
  instances: "nav.versions",
  about: "nav.about",
  settings: "app.settings",
};

/** Icon, label and accent of each content kind, shared by the sidebar and
 * Home's content tiles so both read the same. */
export const CONTENT_SECTION_META: Record<
  ContentSectionKey,
  { labelKey: string; Icon: IconComponent; tone: string; iconClassName: string }
> = {
  worlds: {
    labelKey: "contentpage.worlds",
    Icon: FaGlobe,
    tone: "bg-blue-50 dark:bg-blue-900/20 text-blue-500",
    iconClassName: "text-blue-500",
  },
  mods: {
    labelKey: "nav.mods",
    Icon: FaPuzzlePiece,
    tone: "bg-indigo-50 dark:bg-indigo-900/20 text-indigo-500",
    iconClassName: "text-indigo-500",
  },
  resourcePacks: {
    labelKey: "contentpage.resource_packs",
    Icon: FaImage,
    tone: "bg-purple-50 dark:bg-purple-900/20 text-purple-500",
    iconClassName: "text-purple-500",
  },
  behaviorPacks: {
    labelKey: "contentpage.behavior_packs",
    Icon: FaCogs,
    tone: "bg-orange-50 dark:bg-orange-900/20 text-orange-500",
    iconClassName: "text-orange-500",
  },
  skinPacks: {
    labelKey: "contentpage.skin_packs",
    Icon: FaUserTag,
    tone: "bg-pink-50 dark:bg-pink-900/20 text-pink-500",
    iconClassName: "text-pink-500",
  },
  servers: {
    labelKey: "contentpage.servers",
    Icon: FaServer,
    tone: "bg-sky-50 dark:bg-sky-900/20 text-sky-500",
    iconClassName: "text-sky-500",
  },
  screenshots: {
    labelKey: "contentpage.screenshots",
    Icon: FaCamera,
    tone: "bg-teal-50 dark:bg-teal-900/20 text-teal-500",
    iconClassName: "text-teal-500",
  },
};

export const CATALOG_META: Record<
  DiscoverCatalog,
  { labelKey: string; Icon: IconComponent; iconClassName: string }
> = {
  lip: { labelKey: "lip.title", Icon: FaCube, iconClassName: "text-green-500" },
  curseforge: {
    labelKey: "curseforge.title",
    Icon: SiCurseforge,
    iconClassName: "text-[#f16436]",
  },
};

export type PrimaryNavItem = {
  key: PrimaryNavKey;
  label: string;
  path: string;
  Icon: IconComponent;
  isActive: boolean;
};

export type ChildNavItem = {
  key: string;
  label: string;
  path: string;
  Icon: IconComponent;
  iconClassName: string;
  isActive: boolean;
};

/** A labeled group of Home's children: the instance's content, and the
 * catalogs that install into it. */
export type ChildNavGroup = {
  key: "content" | "download";
  label: string;
  items: ChildNavItem[];
};

/** Primary destinations with labels, icons and the active state for the
 * current route, plus Home's child pages, shared by the sidebar and the
 * navbar layout. */
export const usePrimaryNav = () => {
  const { t } = useTranslation();
  const { pathname, search } = useLocation();
  const activeKey = getActiveNavKey(pathname, search);
  const activeCatalog = getActiveCatalog(pathname);
  const activeSection = getActiveContentSection(pathname);

  useEffect(() => {
    if (!activeCatalog) return;
    try {
      localStorage.setItem(LAST_CATALOG_KEY, activeCatalog);
    } catch {}
  }, [activeCatalog]);

  const items: PrimaryNavItem[] = PRIMARY_NAV.map((item) => ({
    key: item.key,
    label: t(NAV_LABEL_KEYS[item.key]),
    path: item.path,
    Icon: NAV_ICONS[item.key],
    isActive: item.key === activeKey,
  }));

  const homeGroups: ChildNavGroup[] = [
    {
      key: "content",
      label: t("launcherpage.content_manage"),
      items: CONTENT_SECTIONS.map((section) => {
        const meta = CONTENT_SECTION_META[section.key];
        return {
          key: section.key,
          label: t(meta.labelKey),
          path: section.path,
          Icon: meta.Icon,
          iconClassName: meta.iconClassName,
          isActive: section.key === activeSection,
        };
      }),
    },
    {
      key: "download",
      label: t("contentdownload.title"),
      items: DISCOVER_CATALOGS.map((catalog) => {
        const meta = CATALOG_META[catalog.key];
        return {
          key: catalog.key,
          label: t(meta.labelKey),
          path: catalog.path,
          Icon: meta.Icon,
          iconClassName: meta.iconClassName,
          isActive: catalog.key === activeCatalog,
        };
      }),
    },
  ];

  return { items, homeGroups, activeKey };
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

/** The app-wide current-instance switcher, the only one in the app: inline
 * on wide windows, behind a compact button on narrow ones so the bar never
 * clips, and hidden on pages that have no current instance. */
export const ShellInstancePicker: React.FC<{
  isDisabled?: boolean;
  density?: ShellDensity;
}> = ({ isDisabled, density = "topbar" }) => {
  const { t } = useTranslation();
  const { currentVersionName } = useCurrentVersion();
  const { pathname } = useLocation();
  const [open, setOpen] = React.useState(false);
  const inline = useMediaQuery(PICKER_INLINE_QUERY[density]);
  const label = currentVersionName
    ? `${t("instance_picker.label")}: ${currentVersionName}`
    : t("instance_picker.label");

  // The instance list, downloads and settings have no current instance.
  if (!isInstanceScopedRoute(pathname)) return null;

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
