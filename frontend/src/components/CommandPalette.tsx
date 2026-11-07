import React from "react";
import { Kbd, Modal } from "@heroui/react";
import { useTranslation } from "react-i18next";
import { useLocation, useNavigate } from "react-router-dom";
import {
  FaBolt,
  FaCheck,
  FaCog,
  FaCompass,
  FaCube,
  FaHistory,
  FaSearch,
} from "react-icons/fa";
import { ROUTES } from "@/constants/routes";
import { useInstanceList } from "@/components/InstancePicker";
import { useCurrentVersion } from "@/utils/CurrentVersionContext";
import { formatCombo } from "@/utils/KeybindingContext";
import { useTheme } from "next-themes";
import { openActivityCenter } from "@/utils/activity";
import { cn } from "@/utils/cn";
import {
  NAV_TARGETS,
  OPEN_COMMAND_PALETTE_EVENT,
  SETTINGS_TABS,
  openShortcutsHelp,
  pushRecentCommandId,
  readRecentCommandIds,
  routeLabelKey,
  scoreCommand,
  settingsTabPath,
  type CommandGroup,
  type PaletteCommand,
} from "@/utils/commands";

type CommandPaletteProps = {
  /** App-level navigation that honors nav locks and unsaved-change prompts. */
  tryNavigate: (path: string | number) => void;
  navLocked: boolean;
};

const GROUP_ORDER: CommandGroup[] = [
  "recent",
  "actions",
  "navigate",
  "settings",
  "instances",
];

const GROUP_ICONS: Record<CommandGroup, React.ReactNode> = {
  recent: <FaHistory />,
  actions: <FaBolt />,
  navigate: <FaCompass />,
  settings: <FaCog />,
  instances: <FaCube />,
};

type Row = { key: string; group: CommandGroup; command: PaletteCommand };

const KeyHint = ({ combo }: { combo: string }) => (
  <span className="inline-flex items-center gap-1">
    {formatCombo(combo).map((label) => (
      <Kbd key={label} className="text-xs">
        <Kbd.Content>{label}</Kbd.Content>
      </Kbd>
    ))}
  </span>
);

const PaletteBody = ({
  tryNavigate,
  navLocked,
  onClose,
}: CommandPaletteProps & { onClose: () => void }) => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const { currentVersionName, setCurrentVersionName } = useCurrentVersion();
  const { instances } = useInstanceList();
  const { resolvedTheme, setTheme } = useTheme();
  const [query, setQuery] = React.useState("");
  const [activeIndex, setActiveIndex] = React.useState(0);
  const [recentIds] = React.useState(readRecentCommandIds);
  const listId = React.useId();

  // Settings pages intercept tryNavigate to ask about unsaved changes, and
  // that path-only hand-off cannot carry route state.
  const onGuardedPage =
    location.pathname === ROUTES.settings ||
    location.pathname === ROUTES.instanceSettings;

  const go = React.useCallback(
    (path: string, state?: unknown) => {
      if (navLocked) return;
      if (state) navigate(path, { state });
      else tryNavigate(path);
    },
    [navLocked, navigate, tryNavigate],
  );

  // Same effect as useThemeManager().toggleTheme; the app-level theme
  // managers pick the change up through these events.
  const toggleTheme = React.useCallback(() => {
    const next = resolvedTheme === "dark" ? "light" : "dark";
    setTheme(next);
    try {
      localStorage.setItem("app.themeMode", next);
    } catch {}
    window.dispatchEvent(new CustomEvent("app-theme-mode-changed"));
    window.dispatchEvent(new CustomEvent("app-theme-overridden"));
  }, [resolvedTheme, setTheme]);

  const commands = React.useMemo<PaletteCommand[]>(() => {
    const list: PaletteCommand[] = [];
    const launchLabel = String(t("launcherpage.launch_button"));

    list.push({
      id: "action.activity",
      group: "actions",
      title: t("palette.open_activity"),
      keywords: ["activity", "notifications", "history", "log", "tasks"],
      run: openActivityCenter,
    });
    list.push({
      id: "action.theme",
      group: "actions",
      title:
        resolvedTheme === "dark"
          ? t("palette.theme_light")
          : t("palette.theme_dark"),
      keywords: ["theme", "dark", "light", "mode", "toggle", t("theme.toggle")],
      run: toggleTheme,
    });
    list.push({
      id: "action.shortcuts",
      group: "actions",
      title: t("palette.show_shortcuts"),
      keywords: ["keyboard", "shortcuts", "hotkeys", "keys", "help"],
      shortcut: "shift+?",
      run: openShortcutsHelp,
    });
    if (currentVersionName && !onGuardedPage) {
      list.push({
        id: "action.instance_settings",
        group: "actions",
        title: t("palette.instance_settings", { name: currentVersionName }),
        keywords: ["instance", "settings", "edit", "rename", "loader"],
        navigates: true,
        run: () =>
          go(ROUTES.instanceSettings, {
            name: currentVersionName,
            returnTo: location.pathname,
          }),
      });
    }

    for (const target of NAV_TARGETS) {
      const isHome = target.path === ROUTES.home;
      list.push({
        id: target.id,
        group: "navigate",
        title: t(routeLabelKey(target.path)),
        // Launching lives on Home; the subtitle says which instance runs.
        subtitle:
          isHome && currentVersionName
            ? t("palette.launch_subtitle", {
                label: launchLabel,
                name: currentVersionName,
              })
            : undefined,
        keywords: isHome ? [...target.keywords, launchLabel] : target.keywords,
        shortcut: target.path === ROUTES.settings ? "ctrl+," : undefined,
        navigates: true,
        run: () => go(target.path),
      });
    }

    for (const entry of SETTINGS_TABS) {
      list.push({
        id: `settings.${entry.tab}`,
        group: "settings",
        title: `${t("app.settings")} › ${t(`settings.tabs.${entry.tab}`)}`,
        keywords: entry.keywords,
        navigates: true,
        run: () => go(settingsTabPath(entry.tab)),
      });
    }

    for (const instance of instances) {
      const isCurrent = instance.name === currentVersionName;
      list.push({
        id: `instance.${instance.name}`,
        group: "instances",
        title: t("palette.switch_instance", { name: instance.name }),
        subtitle: [
          instance.gameVersion,
          isCurrent ? t("palette.current") : "",
        ]
          .filter(Boolean)
          .join(" · "),
        keywords: ["instance", "switch", "select", "use", instance.name],
        disabled: isCurrent,
        run: () => setCurrentVersionName(instance.name, "command-palette"),
      });
    }

    return navLocked ? list.filter((command) => !command.navigates) : list;
  }, [
    t,
    resolvedTheme,
    toggleTheme,
    currentVersionName,
    setCurrentVersionName,
    instances,
    navLocked,
    go,
    onGuardedPage,
    location.pathname,
  ]);

  const rows = React.useMemo<Row[]>(() => {
    const trimmed = query.trim();
    if (trimmed) {
      // Ranked flat list; recently used commands win ties.
      return commands
        .map((command, order) => ({
          command,
          order,
          score: scoreCommand(trimmed, command),
          recent: recentIds.indexOf(command.id),
        }))
        .filter((entry) => entry.score > 0)
        .sort(
          (a, b) =>
            b.score - a.score ||
            (a.recent === -1 ? 99 : a.recent) - (b.recent === -1 ? 99 : b.recent) ||
            a.order - b.order,
        )
        .map(({ command }) => ({ key: command.id, group: command.group, command }));
    }
    const recent = recentIds
      .map((id) => commands.find((command) => command.id === id))
      .filter((command): command is PaletteCommand => Boolean(command));
    const recentSet = new Set(recent.map((command) => command.id));
    return [
      ...recent.map((command) => ({ key: command.id, group: "recent" as const, command })),
      ...GROUP_ORDER.slice(1).flatMap((group) =>
        commands
          .filter((command) => command.group === group && !recentSet.has(command.id))
          .map((command) => ({ key: command.id, group, command })),
      ),
    ];
  }, [commands, query, recentIds]);

  // Keep the highlight on a real row as the list changes.
  React.useEffect(() => {
    setActiveIndex(0);
  }, [query]);
  const safeIndex = rows.length ? Math.min(activeIndex, rows.length - 1) : -1;
  const activeRow = safeIndex >= 0 ? rows[safeIndex] : undefined;
  const optionId = (index: number) => `${listId}-option-${index}`;

  React.useEffect(() => {
    if (safeIndex < 0) return;
    document
      .getElementById(optionId(safeIndex))
      ?.scrollIntoView({ block: "nearest" });
    // optionId only depends on listId, which is stable.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [safeIndex]);

  const runRow = (row: Row | undefined) => {
    if (!row || row.command.disabled) return;
    pushRecentCommandId(row.command.id);
    onClose();
    // Let the palette release focus before the command opens other overlays.
    window.setTimeout(() => row.command.run(), 0);
  };

  const onKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    const count = rows.length;
    const page = 8;
    switch (event.key) {
      case "ArrowDown":
        if (!count) return;
        event.preventDefault();
        setActiveIndex((safeIndex + 1) % count);
        return;
      case "ArrowUp":
        if (!count) return;
        event.preventDefault();
        setActiveIndex((safeIndex - 1 + count) % count);
        return;
      case "PageDown":
        if (!count) return;
        event.preventDefault();
        setActiveIndex(Math.min(count - 1, safeIndex + page));
        return;
      case "PageUp":
        if (!count) return;
        event.preventDefault();
        setActiveIndex(Math.max(0, safeIndex - page));
        return;
      case "Enter":
        event.preventDefault();
        runRow(activeRow);
        return;
    }
    // The open shortcut toggles the palette closed again.
    if (
      (event.ctrlKey || event.metaKey) &&
      !event.altKey &&
      ["k", "p"].includes(event.key.toLowerCase())
    ) {
      event.preventDefault();
      onClose();
    }
  };

  const groupLabel = (group: CommandGroup) => t(`palette.group.${group}`);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex shrink-0 items-center gap-3 border-b border-border/60 px-4 py-3">
        <FaSearch aria-hidden className="shrink-0 text-muted" />
        <input
          autoFocus
          type="text"
          role="combobox"
          aria-expanded="true"
          aria-controls={listId}
          aria-autocomplete="list"
          aria-activedescendant={safeIndex >= 0 ? optionId(safeIndex) : undefined}
          aria-label={t("palette.title")}
          placeholder={t("palette.placeholder")}
          spellCheck={false}
          autoComplete="off"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          onKeyDown={onKeyDown}
          className="min-w-0 flex-1 bg-transparent text-base text-foreground outline-none placeholder:text-muted"
        />
        <span className="hidden shrink-0 sm:inline-flex">
          <KeyHint combo="Escape" />
        </span>
      </div>

      {navLocked && (
        <p className="shrink-0 px-4 pt-3 text-xs text-warning">
          {t("palette.nav_locked")}
        </p>
      )}

      <ul
        id={listId}
        role="listbox"
        aria-label={t("palette.results")}
        className="min-h-0 flex-1 overflow-y-auto overscroll-contain p-2"
      >
        {rows.length === 0 && (
          <li role="presentation" className="px-3 py-8 text-center text-sm text-muted">
            {t("palette.empty", { query: query.trim() })}
          </li>
        )}
        {rows.map((row, index) => {
          const showHeading =
            !query.trim() && (index === 0 || rows[index - 1].group !== row.group);
          const active = index === safeIndex;
          const disabled = Boolean(row.command.disabled);
          return (
            <React.Fragment key={`${row.group}:${row.key}`}>
              {showHeading && (
                <li
                  role="presentation"
                  className="px-3 pb-1 pt-3 text-xs font-semibold uppercase tracking-wide text-muted first:pt-1"
                >
                  {groupLabel(row.group)}
                </li>
              )}
              <li
                id={optionId(index)}
                role="option"
                aria-selected={active}
                aria-disabled={disabled || undefined}
                onMouseMove={() => {
                  if (!active) setActiveIndex(index);
                }}
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => runRow(row)}
                className={cn(
                  "flex items-center gap-3 rounded-xl px-3 py-2 text-sm",
                  disabled ? "cursor-default" : "cursor-pointer",
                  active
                    ? "bg-brand-500/15 text-foreground"
                    : "text-foreground/90",
                )}
              >
                <span
                  aria-hidden
                  className={cn(
                    "shrink-0 text-muted",
                    active && "text-brand-600 dark:text-brand-400",
                  )}
                >
                  {GROUP_ICONS[row.command.group]}
                </span>
                <span className="flex min-w-0 flex-1 flex-col">
                  <span className="truncate">{row.command.title}</span>
                  {row.command.subtitle && (
                    <span className="truncate text-xs text-muted">
                      {row.command.subtitle}
                    </span>
                  )}
                </span>
                {disabled && (
                  <FaCheck aria-hidden className="shrink-0 text-success" />
                )}
                {query.trim() && (
                  <span className="hidden shrink-0 text-xs text-muted sm:inline">
                    {groupLabel(row.command.group)}
                  </span>
                )}
                {row.command.shortcut && (
                  <span className="hidden shrink-0 sm:inline-flex">
                    <KeyHint combo={row.command.shortcut} />
                  </span>
                )}
              </li>
            </React.Fragment>
          );
        })}
      </ul>

      <div className="flex shrink-0 flex-wrap items-center gap-x-4 gap-y-1 border-t border-border/60 px-4 py-2 text-xs text-muted">
        <span className="inline-flex items-center gap-1.5">
          <KeyHint combo="arrowup" />
          <KeyHint combo="arrowdown" />
          {t("palette.hint_move")}
        </span>
        <span className="inline-flex items-center gap-1.5">
          <KeyHint combo="enter" />
          {t("palette.hint_run")}
        </span>
        <span className="inline-flex items-center gap-1.5">
          <KeyHint combo="Escape" />
          {t("palette.hint_close")}
        </span>
      </div>
    </div>
  );
};

/**
 * Keyboard-first launcher for every page, settings tab and common action.
 * Opens on Ctrl+K / Ctrl+P (registered in App) or the
 * `ll:open-command-palette` window event.
 */
export const CommandPalette: React.FC<CommandPaletteProps> = ({
  tryNavigate,
  navLocked,
}) => {
  const { t } = useTranslation();
  const [isOpen, setIsOpen] = React.useState(false);

  React.useEffect(() => {
    const open = () => setIsOpen(true);
    window.addEventListener(OPEN_COMMAND_PALETTE_EVENT, open);
    return () => window.removeEventListener(OPEN_COMMAND_PALETTE_EVENT, open);
  }, []);

  const close = React.useCallback(() => setIsOpen(false), []);

  return (
    <Modal>
      <Modal.Backdrop
        isOpen={isOpen}
        onOpenChange={setIsOpen}
        isDismissable
        variant="blur"
        className="z-[100]"
      >
        <Modal.Container
          placement="top"
          scroll="inside"
          className="z-[100] w-full min-w-0 px-2 pt-[min(12vh,6rem)] sm:px-4"
        >
          <Modal.Dialog
            aria-label={t("palette.title")}
            data-command-palette
            className="flex max-h-[min(36rem,calc(100dvh-2rem))] w-full min-w-0 max-w-[40rem] flex-col gap-0 overflow-hidden rounded-2xl border border-white/40 bg-overlay p-0 shadow-2xl dark:border-zinc-700/50"
          >
            {isOpen && (
              <PaletteBody
                tryNavigate={tryNavigate}
                navLocked={navLocked}
                onClose={close}
              />
            )}
          </Modal.Dialog>
        </Modal.Container>
      </Modal.Backdrop>
    </Modal>
  );
};
