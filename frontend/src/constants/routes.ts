export const ROUTES = {
  home: "/",
  download: "/download",
  downloadTasks: "/download/tasks",
  install: "/install",
  instances: "/instances",
  instanceSettings: "/instances/settings",
  mods: "/mods",
  curseForge: "/curseforge",
  curseForgeMod: "/curseforge/mod/:id",
  lip: "/lip",
  lipPackage: "/lip/package/:id",
  /** Opens the catalog the user browsed last (LIP or CurseForge). */
  discover: "/discover",
  content: "/content",
  contentWorlds: "/content/worlds",
  contentWorldEditor: "/content/worlds/worldEdit",
  contentResourcePacks: "/content/resourcePacks",
  contentBehaviorPacks: "/content/behaviorPacks",
  contentSkinPacks: "/content/skinPacks",
  contentScreenshots: "/content/screenshots",
  contentServers: "/content/servers",
  settings: "/settings",
  about: "/about",
  updating: "/updating",
  onboarding: "/onboarding",
} as const;

export const routeTo = {
  curseForgeMod: (id: string | number): string =>
    ROUTES.curseForgeMod.replace(":id", encodeURIComponent(String(id))),
  lipPackage: (id: string): string =>
    ROUTES.lipPackage.replace(":id", encodeURIComponent(String(id))),
  contentWorldEditor: (path: string): string =>
    `${ROUTES.contentWorldEditor}?path=${encodeURIComponent(path)}`,
} as const;

export const isRouteActive = (pathname: string, route: string): boolean => {
  const currentPath = String(pathname || "");
  const targetRoute = String(route || "");

  if (!targetRoute) return false;
  if (targetRoute === ROUTES.home) return currentPath === ROUTES.home;

  return (
    currentPath === targetRoute || currentPath.startsWith(`${targetRoute}/`)
  );
};

export type PrimaryNavKey =
  | "home"
  | "instances"
  | "mods"
  | "content"
  | "discover"
  | "download"
  | "settings";

/** Primary destinations shared by the sidebar and the navbar layout. Each
 * `match` prefix makes nested routes highlight their parent destination. */
export const PRIMARY_NAV: ReadonlyArray<{
  key: PrimaryNavKey;
  path: string;
  match: readonly string[];
}> = [
  { key: "home", path: ROUTES.home, match: [ROUTES.home] },
  { key: "instances", path: ROUTES.instances, match: [ROUTES.instances] },
  { key: "mods", path: ROUTES.mods, match: [ROUTES.mods] },
  { key: "content", path: ROUTES.content, match: [ROUTES.content] },
  {
    key: "discover",
    path: ROUTES.discover,
    match: [ROUTES.discover, ROUTES.lip, ROUTES.curseForge],
  },
  {
    key: "download",
    path: ROUTES.download,
    match: [ROUTES.download, ROUTES.install],
  },
  {
    key: "settings",
    path: ROUTES.settings,
    match: [ROUTES.settings, ROUTES.about],
  },
];

export const getActiveNavKey = (pathname: string): PrimaryNavKey | null =>
  PRIMARY_NAV.find((item) =>
    item.match.some((route) => isRouteActive(pathname, route)),
  )?.key ?? null;

export type DiscoverCatalog = "lip" | "curseforge";

export const DISCOVER_CATALOGS: ReadonlyArray<{
  key: DiscoverCatalog;
  path: string;
}> = [
  { key: "lip", path: ROUTES.lip },
  { key: "curseforge", path: ROUTES.curseForge },
];

export const getActiveCatalog = (pathname: string): DiscoverCatalog | null =>
  DISCOVER_CATALOGS.find((item) => isRouteActive(pathname, item.path))?.key ??
  null;
