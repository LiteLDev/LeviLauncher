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
