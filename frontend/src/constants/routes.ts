export const ROUTES = {
  home: "/",
  download: "/download",
  downloadTasks: "/download/tasks",
  instances: "/instances",
  instanceSettings: "/instances/settings",
  settings: "/settings",
  about: "/about",
  updating: "/updating",
  onboarding: "/onboarding",
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
