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

export type PrimaryNavKey = "home" | "instances" | "download" | "settings";

/** Home owns the current instance: its content and the catalogs that install
 * into it are Home's children, so they highlight Home in the navigation. */
const HOME_CHILD_ROUTES = [
  ROUTES.home,
  ROUTES.content,
  ROUTES.mods,
  ROUTES.discover,
  ROUTES.lip,
  ROUTES.curseForge,
] as const;

/** Primary destinations shared by the sidebar and the navbar layout. Each
 * `match` prefix makes nested routes highlight their parent destination. */
export const PRIMARY_NAV: ReadonlyArray<{
  key: PrimaryNavKey;
  path: string;
  match: readonly string[];
}> = [
  { key: "home", path: ROUTES.home, match: HOME_CHILD_ROUTES },
  { key: "instances", path: ROUTES.instances, match: [ROUTES.instances] },
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

/** Pages that act on the current instance. Only these show the top bar's
 * instance picker; instance list, downloads and settings have no current
 * instance. */
export const isInstanceScopedRoute = (pathname: string): boolean =>
  HOME_CHILD_ROUTES.some((route) => isRouteActive(pathname, route));

export type ContentSectionKey =
  | "worlds"
  | "mods"
  | "resourcePacks"
  | "behaviorPacks"
  | "skinPacks"
  | "servers"
  | "screenshots";

/** The current instance's content, in the order Home and the sidebar list it. */
export const CONTENT_SECTIONS: ReadonlyArray<{
  key: ContentSectionKey;
  path: string;
}> = [
  { key: "worlds", path: ROUTES.contentWorlds },
  { key: "mods", path: ROUTES.mods },
  { key: "resourcePacks", path: ROUTES.contentResourcePacks },
  { key: "behaviorPacks", path: ROUTES.contentBehaviorPacks },
  { key: "skinPacks", path: ROUTES.contentSkinPacks },
  { key: "servers", path: ROUTES.contentServers },
  { key: "screenshots", path: ROUTES.contentScreenshots },
];

export const getActiveContentSection = (
  pathname: string,
): ContentSectionKey | null =>
  CONTENT_SECTIONS.find((item) => isRouteActive(pathname, item.path))?.key ??
  null;

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
