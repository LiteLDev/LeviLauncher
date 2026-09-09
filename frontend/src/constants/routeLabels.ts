import { ROUTES } from "./routes";

export const ROUTE_LABEL_KEYS: Record<string, string> = {
  [ROUTES.home]: "nav.home",
  [ROUTES.download]: "downloadmodal.download_button",
  [ROUTES.downloadTasks]: "download_manager.title",
  [ROUTES.install]: "nav.local_install",
  [ROUTES.instances]: "nav.versions",
  [ROUTES.instanceSettings]: "versions.edit.title",
  [ROUTES.mods]: "nav.mods",
  [ROUTES.curseForge]: "CurseForge",
  [ROUTES.curseForgeMod]: "nav.mod_details",
  [ROUTES.lip]: "lip.title",
  [ROUTES.lipPackage]: "nav.package_details",
  [ROUTES.content]: "launcherpage.content_manage",
  [ROUTES.contentWorlds]: "contentpage.worlds",
  [ROUTES.contentWorldEditor]: "contentpage.world_leveldat_editor",
  [ROUTES.contentResourcePacks]: "contentpage.resource_packs",
  [ROUTES.contentBehaviorPacks]: "contentpage.behavior_packs",
  [ROUTES.contentSkinPacks]: "contentpage.skin_packs",
  [ROUTES.contentScreenshots]: "contentpage.screenshots",
  [ROUTES.contentServers]: "contentpage.servers",
  [ROUTES.settings]: "app.settings",
  [ROUTES.about]: "nav.about",
  [ROUTES.updating]: "nav.updating",
  [ROUTES.onboarding]: "onboarding.title",
};

export const getRouteLabelKey = (pathname: string): string => {
  if (ROUTE_LABEL_KEYS[pathname]) return ROUTE_LABEL_KEYS[pathname];
  for (const route of [ROUTES.curseForgeMod, ROUTES.lipPackage]) {
    const parent = route.slice(0, route.indexOf("/:id"));
    if (pathname === parent || pathname.startsWith(`${parent}/`)) {
      return ROUTE_LABEL_KEYS[route];
    }
  }
  return "nav.breadcrumb";
};
