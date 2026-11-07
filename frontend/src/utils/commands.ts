import { ROUTES } from "@/constants/routes";
export { getRouteLabelKey as routeLabelKey } from "@/constants/routeLabels";

/** Window event that opens the command palette from anywhere (e.g. a top-bar button). */
export const OPEN_COMMAND_PALETTE_EVENT = "ll:open-command-palette";
/** Window event that opens the keyboard shortcuts help dialog. */
export const OPEN_SHORTCUTS_HELP_EVENT = "ll:open-shortcuts-help";

export const openCommandPalette = () => {
  window.dispatchEvent(new CustomEvent(OPEN_COMMAND_PALETTE_EVENT));
};

export const openShortcutsHelp = () => {
  window.dispatchEvent(new CustomEvent(OPEN_SHORTCUTS_HELP_EVENT));
};

export type CommandGroup = "recent" | "navigate" | "settings" | "instances" | "actions";

export type PaletteCommand = {
  /** Stable id, persisted for the "recent" list. */
  id: string;
  group: Exclude<CommandGroup, "recent">;
  title: string;
  subtitle?: string;
  /** Extra search terms; English terms are always included so either language matches. */
  keywords?: string[];
  /** Shortcut shown next to the command, in KeybindingContext combo syntax. */
  shortcut?: string;
  /** Navigation commands are hidden while navigation is locked. */
  navigates?: boolean;
  /** Shown but not runnable, e.g. the instance that is already current. */
  disabled?: boolean;
  run: () => void;
};

type NavTarget = { id: string; path: string; keywords: string[] };

// Every user-facing route without params. Internal flows (onboarding,
// updating, world editor) and detail pages are left out on purpose.
export const NAV_TARGETS: NavTarget[] = [
  { id: "nav.home", path: ROUTES.home, keywords: ["home", "launch", "play", "start", "run"] },
  { id: "nav.download", path: ROUTES.download, keywords: ["download", "versions", "get", "minecraft"] },
  { id: "nav.download_tasks", path: ROUTES.downloadTasks, keywords: ["download", "tasks", "queue", "progress"] },
  { id: "nav.install", path: ROUTES.install, keywords: ["install", "local", "file", "appx", "msixvc", "import"] },
  { id: "nav.instances", path: ROUTES.instances, keywords: ["instances", "versions", "manage"] },
  { id: "nav.mods", path: ROUTES.mods, keywords: ["mods", "plugins", "addons"] },
  { id: "nav.curseforge", path: ROUTES.curseForge, keywords: ["curseforge", "browse", "mods", "search"] },
  { id: "nav.lip", path: ROUTES.lip, keywords: ["lip", "packages", "levilamina", "browse"] },
  { id: "nav.content", path: ROUTES.content, keywords: ["content", "manage", "files"] },
  { id: "nav.worlds", path: ROUTES.contentWorlds, keywords: ["worlds", "maps", "saves"] },
  { id: "nav.resource_packs", path: ROUTES.contentResourcePacks, keywords: ["resource", "packs", "textures"] },
  { id: "nav.behavior_packs", path: ROUTES.contentBehaviorPacks, keywords: ["behavior", "behaviour", "packs", "addons"] },
  { id: "nav.skin_packs", path: ROUTES.contentSkinPacks, keywords: ["skin", "packs", "skins"] },
  { id: "nav.screenshots", path: ROUTES.contentScreenshots, keywords: ["screenshots", "images", "pictures"] },
  { id: "nav.servers", path: ROUTES.contentServers, keywords: ["servers", "multiplayer"] },
  { id: "nav.settings", path: ROUTES.settings, keywords: ["settings", "preferences", "options", "config"] },
  { id: "nav.about", path: ROUTES.about, keywords: ["about", "version", "credits", "license"] },
];

export const SETTINGS_TABS: { tab: string; keywords: string[] }[] = [
  { tab: "general", keywords: ["general", "language", "directory", "folder"] },
  { tab: "personalization", keywords: ["personalization", "appearance", "theme", "background", "wallpaper", "layout", "color"] },
  { tab: "components", keywords: ["components", "mirror", "lip", "proxy", "cache"] },
  { tab: "others", keywords: ["others", "misc", "advanced"] },
  { tab: "privacy", keywords: ["privacy", "analytics", "telemetry"] },
  { tab: "updates", keywords: ["updates", "beta", "upgrade"] },
  { tab: "about", keywords: ["about", "version", "launcher"] },
];

export const settingsTabPath = (tab: string) =>
  `${ROUTES.settings}?tab=${encodeURIComponent(tab)}`;

// ---------------------------------------------------------------------------
// Matching

const normalize = (value: string) =>
  value
    .toLocaleLowerCase()
    .normalize("NFKD")
    .replace(/[̀-ͯ]/g, "");

/** In-order subsequence match, e.g. "rsp" matches "resource packs". */
const isSubsequence = (needle: string, haystack: string) => {
  let i = 0;
  for (const ch of haystack) {
    if (ch === needle[i]) i++;
    if (i === needle.length) return true;
  }
  return needle.length === 0;
};

const scoreToken = (token: string, title: string, rest: string): number => {
  if (title.startsWith(token)) return 100;
  if (title.split(/[\s·:/\-_]+/).some((word) => word.startsWith(token))) return 80;
  if (title.includes(token)) return 60;
  if (rest.split(/\s+/).some((word) => word.startsWith(token))) return 45;
  if (rest.includes(token)) return 30;
  if (token.length > 1 && isSubsequence(token, title)) return 15;
  return 0;
};

/**
 * Scores a command against the query; 0 means no match. Every whitespace
 * separated token has to match the title, subtitle or keywords.
 */
export const scoreCommand = (query: string, command: PaletteCommand): number => {
  const tokens = normalize(query).split(/\s+/).filter(Boolean);
  if (tokens.length === 0) return 1;
  const title = normalize(command.title);
  const rest = normalize(
    [command.subtitle ?? "", ...(command.keywords ?? [])].join(" "),
  );
  let total = 0;
  for (const token of tokens) {
    const score = scoreToken(token, title, rest);
    if (score === 0) return 0;
    total += score;
  }
  return total;
};

// ---------------------------------------------------------------------------
// Recent commands

const RECENT_KEY = "ll.commandPalette.recent";
const RECENT_LIMIT = 5;

export const readRecentCommandIds = (): string[] => {
  try {
    const parsed = JSON.parse(localStorage.getItem(RECENT_KEY) || "[]");
    return Array.isArray(parsed)
      ? parsed.filter((id): id is string => typeof id === "string").slice(0, RECENT_LIMIT)
      : [];
  } catch {
    return [];
  }
};

export const pushRecentCommandId = (id: string) => {
  try {
    const next = [id, ...readRecentCommandIds().filter((item) => item !== id)].slice(
      0,
      RECENT_LIMIT,
    );
    localStorage.setItem(RECENT_KEY, JSON.stringify(next));
  } catch {}
};
