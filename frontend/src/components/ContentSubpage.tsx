import React from "react";
import {
  Button,
  Card,
  Dropdown,
  InputGroup,
  Label,
  ListBox,
  Select,
  TextField,
  Tooltip,
} from "@heroui/react";
import { useTranslation } from "react-i18next";
import { useLocation } from "react-router-dom";
import {
  FaFilter,
  FaSortAmountDown,
  FaSortAmountUp,
  FaTimes,
  FaUser,
} from "react-icons/fa";
import { GetLocalUserGamertag } from "bindings/github.com/liteldev/LeviLauncher/internal/app/userservice";
import { InstancePicker } from "@/components/InstancePicker";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { LAYOUT } from "@/constants/layout";
import { GetContentRoots } from "bindings/github.com/liteldev/LeviLauncher/internal/app/contentservice";
import { GetVersionMeta } from "bindings/github.com/liteldev/LeviLauncher/internal/app/versionservice";
import type { ContentRoots } from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import {
  EMPTY_CONTENT_ROOTS,
  getPlayerGamertagMap,
  listPlayers,
  resolvePlayerDisplayName,
} from "@/utils/content";
import { cn } from "@/utils/cn";

// Shared pieces for the Content sub-pages (packs, skins, screenshots, servers):
// roots and players that follow the current instance, the in-place instance
// and player pickers, and the search/sort row.

// Tailwind only generates class names it finds literally in source, and
// LAYOUT.GRID.AUTO builds its column class at runtime. The sizes these pages use:
// grid-cols-[repeat(auto-fill,minmax(min(100%,30rem),1fr))]
// grid-cols-[repeat(auto-fill,minmax(min(100%,15rem),1fr))]

const PLAYER_STORAGE_KEY = "content.selectedPlayer";

/**
 * Content roots of an instance (and optionally its game version). `ready` is
 * true once the roots in state belong to `name`; `retry` re-reads them.
 */
export function useContentRoots(name: string, withGameVersion = false) {
  const [roots, setRoots] = React.useState<ContentRoots>(EMPTY_CONTENT_ROOTS);
  const [gameVersion, setGameVersion] = React.useState("");
  const [loadedFor, setLoadedFor] = React.useState<string | null>(null);
  const [error, setError] = React.useState("");
  const [retryKey, setRetryKey] = React.useState(0);

  React.useEffect(() => {
    let cancelled = false;
    setLoadedFor(null);
    setError("");
    setRoots(EMPTY_CONTENT_ROOTS);
    setGameVersion("");
    if (!name) return;
    (async () => {
      try {
        const next = (await GetContentRoots(name)) || EMPTY_CONTENT_ROOTS;
        let version = "";
        if (withGameVersion) {
          try {
            version = String((await GetVersionMeta(name))?.gameVersion || "");
          } catch {}
        }
        if (cancelled) return;
        setRoots(next);
        setGameVersion(version);
        setLoadedFor(name);
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [name, withGameVersion, retryKey]);

  const retry = React.useCallback(() => setRetryKey((key) => key + 1), []);
  return { roots, gameVersion, ready: !!name && loadedFor === name, error, retry };
}

export type ContentAction = {
  key: string;
  label: string;
  icon: React.ReactNode;
  onPress: () => void;
  isDisabled?: boolean;
};

type PlayerSelectProps = {
  players: string[];
  value: string;
  gamertagMap: Record<string, string>;
  onChange: (player: string) => void;
  isDisabled?: boolean;
};

export const PlayerSelect: React.FC<PlayerSelectProps> = ({
  players,
  value,
  gamertagMap,
  onChange,
  isDisabled,
}) => {
  const { t } = useTranslation();
  const items = players.map((id) => ({
    id,
    label: resolvePlayerDisplayName(id, gamertagMap),
  }));
  return (
    <Select
      aria-label={t("content_sub.player")}
      className="w-48 min-w-0 max-w-full"
      placeholder={
        players.length ? t("contentpage.select_player") : t("contentpage.no_players")
      }
      isDisabled={isDisabled || players.length === 0}
      value={players.includes(value) ? value : null}
      onChange={(key) => {
        const next = String(key ?? "");
        if (next && next !== value) onChange(next);
      }}
    >
      <Select.Trigger
        className={cn(COMPONENT_STYLES.select.trigger, "min-h-8 h-8 gap-2 text-sm")}
      >
        <FaUser aria-hidden="true" className="shrink-0 text-muted" />
        <Select.Value />
        <Select.Indicator />
      </Select.Trigger>
      <Select.Popover className={COMPONENT_STYLES.select.popoverContent}>
        <ListBox items={items} className={COMPONENT_STYLES.select.listbox}>
          {(item) => (
            <ListBox.Item key={item.id} id={item.id} textValue={item.label}>
              <span className="truncate text-sm">{item.label}</span>
              <ListBox.ItemIndicator />
            </ListBox.Item>
          )}
        </ListBox>
      </Select.Popover>
    </Select>
  );
};

/**
 * Players of an instance. The initial choice comes from route state, then the
 * last choice, then the signed-in gamertag, then the first folder. A choice the
 * user made is remembered so the Content hub and its sub-pages agree.
 */
export function useContentPlayers(usersRoot: string, throwOnError = false) {
  const location = useLocation();
  const statePlayer = String((location.state as any)?.player || "");
  const [players, setPlayers] = React.useState<string[]>([]);
  const [gamertagMap, setGamertagMap] = React.useState<Record<string, string>>({});
  const [selectedPlayer, setSelected] = React.useState<string>(statePlayer);
  const [loading, setLoading] = React.useState(false);
  // Root whose player list is in state; until it matches, the list is stale.
  const [loadedRoot, setLoadedRoot] = React.useState("");
  const [error, setError] = React.useState("");
  const [reloadKey, setReloadKey] = React.useState(0);
  const chosen = React.useRef(Boolean(statePlayer));

  React.useEffect(() => {
    let cancelled = false;
    if (!usersRoot) {
      setPlayers([]);
      setGamertagMap({});
      setSelected("");
      setLoading(false);
      setError("");
      setLoadedRoot("");
      return;
    }
    setLoading(true);
    setError("");
    (async () => {
      try {
        const names = await listPlayers(usersRoot, throwOnError);
        if (cancelled) return;
        let remembered = "";
        try {
          remembered = localStorage.getItem(PLAYER_STORAGE_KEY) || "";
        } catch {}
        const pick = (current: string) =>
          names.includes(current)
            ? current
            : names.includes(statePlayer)
              ? statePlayer
              : names.includes(remembered)
                ? remembered
                : names[0] || "";
        setPlayers(names);
        setSelected(pick);
        setLoadedRoot(usersRoot);
        const map: Record<string, string> = await getPlayerGamertagMap(usersRoot).catch(() => ({}));
        if (cancelled) return;
        setGamertagMap(map);
        // Prefer the signed-in player unless a remembered choice applies here.
        if (!chosen.current && !names.includes(remembered)) {
          const tag = await GetLocalUserGamertag().catch(() => "");
          if (cancelled || chosen.current || !tag) return;
          const own = names.find((name) => map[name] === tag);
          if (own) setSelected(own);
        }
      } catch (e) {
        if (cancelled) return;
        setPlayers([]);
        setGamertagMap({});
        setSelected("");
        setError(String(e));
        setLoadedRoot(usersRoot);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
    // statePlayer is read once per root; later route state is ignored on purpose.
  }, [usersRoot, throwOnError, reloadKey]);

  const setSelectedPlayer = React.useCallback((player: string) => {
    chosen.current = true;
    setSelected(player);
    try {
      if (player) localStorage.setItem(PLAYER_STORAGE_KEY, player);
    } catch {}
  }, []);

  const reload = React.useCallback(() => setReloadKey((value) => value + 1), []);

  return {
    players,
    gamertagMap,
    selectedPlayer,
    setSelectedPlayer,
    loading: loading || loadedRoot !== usersRoot,
    error,
    reload,
  };
}

const ActionButton: React.FC<{ action: ContentAction }> = ({ action }) => (
  <Tooltip>
    <Button
      isIconOnly
      aria-label={action.label}
      onPress={action.onPress}
      isDisabled={action.isDisabled}
      variant="secondary"
      className="rounded-full bg-surface-secondary text-foreground dark:text-zinc-200"
    >
      {action.icon}
    </Button>
    <Tooltip.Content>{action.label}</Tooltip.Content>
  </Tooltip>
);

type ContentSubpageHeaderProps = {
  title: React.ReactNode;
  description?: React.ReactNode;
  actions: ContentAction[];
  /** Player picker; omit when the page does not depend on a player. */
  player?: PlayerSelectProps | null;
  isIsolation?: boolean;
  /** Search and sort controls. */
  children?: React.ReactNode;
};

export const ContentSubpageHeader: React.FC<ContentSubpageHeaderProps> = ({
  title,
  description,
  actions,
  player,
  isIsolation,
  children,
}) => {
  const { t } = useTranslation();

  // Going back to the Content hub uses the shell breadcrumb and back button,
  // the same as every other page; the hub restores the last chosen player.
  return (
    <Card className={LAYOUT.GLASS_CARD.BASE}>
      <Card.Content className="@container flex flex-col gap-4 p-4 @3xl:p-6">
        <div className="flex min-w-0 flex-wrap items-center justify-between gap-x-4 gap-y-2">
          <div className="min-w-0">
            <h1 className="text-2xl font-black tracking-tight text-brand-600 dark:text-brand-500 @3xl:text-3xl [overflow-wrap:anywhere]">
              {title}
            </h1>
            {description && (
              <p className="mt-1 text-sm text-muted dark:text-zinc-400">{description}</p>
            )}
          </div>
          <div className="flex shrink-0 items-center gap-2">
            {actions.map((action) => (
              <ActionButton key={action.key} action={action} />
            ))}
          </div>
        </div>
        <div className="flex min-w-0 flex-wrap items-center gap-2 text-sm text-muted dark:text-zinc-400">
          {/* The compact trigger only has room for the name, not the version line. */}
          <InstancePicker size="sm" className="[&_button_.text-xs]:hidden" />
          {player && <PlayerSelect {...player} />}
          {typeof isIsolation === "boolean" && (
            <span className="rounded-md bg-surface-secondary px-2 py-1 text-xs">
              {t("contentpage.isolation")}:{" "}
              <span className="font-medium text-foreground dark:text-zinc-200">
                {isIsolation ? t("common.yes") : t("common.no")}
              </span>
            </span>
          )}
        </div>
        {children && (
          <div className="flex min-w-0 flex-wrap items-center justify-between gap-3">
            {children}
          </div>
        )}
      </Card.Content>
    </Card>
  );
};

export const ContentSearchField: React.FC<{
  value: string;
  onChange: (value: string) => void;
}> = ({ value, onChange }) => {
  const { t } = useTranslation();
  return (
    <TextField
      aria-label={t("common.search_placeholder")}
      className={cn(
        "group min-w-0 flex-[1_1_14rem] max-w-md",
        COMPONENT_STYLES.input.mainWrapper,
      )}
      value={value}
      onChange={onChange}
    >
      <InputGroup
        className={cn(
          COMPONENT_STYLES.input.inputWrapper,
          COMPONENT_STYLES.input.innerWrapper,
          "rounded-full",
        )}
      >
        <InputGroup.Prefix>
          <FaFilter className="text-muted" />
        </InputGroup.Prefix>
        <InputGroup.Input
          placeholder={t("common.search_placeholder")}
          className={COMPONENT_STYLES.input.input}
        />
        <InputGroup.Suffix>
          {value && (
            <button
              type="button"
              aria-label={t("content_sub.clear_search")}
              onClick={() => onChange("")}
            >
              <FaTimes />
            </button>
          )}
        </InputGroup.Suffix>
      </InputGroup>
    </TextField>
  );
};

export const ContentSortMenu: React.FC<{
  sortKey: "name" | "time";
  sortAsc: boolean;
  onChange: (key: "name" | "time", asc: boolean) => void;
}> = ({ sortKey, sortAsc, onChange }) => {
  const { t } = useTranslation();
  const options = [
    { id: "name-asc", icon: <FaSortAmountDown />, label: `${t("filemanager.sort.name")}(A-Z)` },
    { id: "name-desc", icon: <FaSortAmountUp />, label: `${t("filemanager.sort.name")}(Z-A)` },
    { id: "time-asc", icon: <FaSortAmountDown />, label: t("contentpage.sort_old_new") },
    { id: "time-desc", icon: <FaSortAmountUp />, label: t("contentpage.sort_new_old") },
  ];
  return (
    <Dropdown>
      <Button
        variant="secondary"
        className="shrink-0 rounded-full bg-surface-secondary font-medium text-foreground dark:text-zinc-200"
      >
        {sortAsc ? <FaSortAmountDown /> : <FaSortAmountUp />}
        {sortKey === "name" ? t("filemanager.sort.name") : t("contentpage.sort_time")}
        {" / "}
        {sortAsc ? t("contentpage.sort_asc") : t("contentpage.sort_desc")}
      </Button>
      <Dropdown.Popover className={COMPONENT_STYLES.dropdown.content}>
        <Dropdown.Menu
          selectionMode="single"
          selectedKeys={new Set([`${sortKey}-${sortAsc ? "asc" : "desc"}`])}
          onSelectionChange={(keys) => {
            const value = String(Array.from(keys as Set<string>)[0] || "name-asc");
            const [key, order] = value.split("-");
            onChange(key as "name" | "time", order === "asc");
          }}
        >
          {options.map((option) => (
            <Dropdown.Item key={option.id} id={option.id} textValue={option.label}>
              {option.icon}
              <Label>{option.label}</Label>
              <Dropdown.ItemIndicator />
            </Dropdown.Item>
          ))}
        </Dropdown.Menu>
      </Dropdown.Popover>
    </Dropdown>
  );
};

/** Empty state for a missing prerequisite, resolved in place. */
export const ContentPrerequisite: React.FC<{
  icon: React.ReactNode;
  title: React.ReactNode;
  body?: React.ReactNode;
  children?: React.ReactNode;
}> = ({ icon, title, body, children }) => (
  <div
    role="status"
    className="flex flex-col items-center gap-3 px-4 py-12 text-center text-muted dark:text-zinc-400"
  >
    <div className="text-5xl opacity-30" aria-hidden="true">
      {icon}
    </div>
    <p className="font-medium text-foreground dark:text-zinc-200">{title}</p>
    {body && <p className="max-w-md text-sm">{body}</p>}
    {children && (
      <div className="flex flex-wrap items-center justify-center gap-2">{children}</div>
    )}
  </div>
);
