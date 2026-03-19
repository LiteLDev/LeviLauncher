import DefaultTheme from "vitepress/theme";
import { inBrowser, type Theme } from "vitepress";
import "./custom.css";

const LOCALE_STORAGE_KEY = "levilauncher-docs-locale";
const ENGLISH_LOCALE = "en-US";
const CHINESE_LOCALE = "zh-CN";

function stripBase(pathname: string, base: string): string {
  if (base === "/") {
    return pathname || "/";
  }

  const normalizedBase = base.replace(/\/+$/, "");

  if (pathname === normalizedBase) {
    return "/";
  }

  if (pathname.startsWith(`${normalizedBase}/`)) {
    return pathname.slice(normalizedBase.length) || "/";
  }

  return pathname || "/";
}

function normalizePath(pathname: string): string {
  const cleanPath = pathname.split(/[?#]/u, 1)[0] ?? "/";
  const withLeadingSlash = cleanPath.startsWith("/")
    ? cleanPath
    : `/${cleanPath}`;
  const withoutIndex = withLeadingSlash.replace(/\/index\.html$/u, "/");
  const withoutTrailingSlash =
    withoutIndex.length > 1 ? withoutIndex.replace(/\/+$/u, "") : withoutIndex;

  return withoutTrailingSlash || "/";
}

function getLocaleFromPath(pathname: string, base: string): string {
  const normalizedPath = normalizePath(stripBase(pathname, base));

  if (
    normalizedPath === `/${CHINESE_LOCALE}` ||
    normalizedPath.startsWith(`/${CHINESE_LOCALE}/`)
  ) {
    return CHINESE_LOCALE;
  }

  return ENGLISH_LOCALE;
}

function readStoredLocale(): string | null {
  try {
    const locale = window.localStorage.getItem(LOCALE_STORAGE_KEY);

    if (locale === ENGLISH_LOCALE || locale === CHINESE_LOCALE) {
      return locale;
    }
  } catch {
    return null;
  }

  return null;
}

function writeStoredLocale(locale: string): void {
  try {
    window.localStorage.setItem(LOCALE_STORAGE_KEY, locale);
  } catch {}
}

function getPreferredLocale(): string {
  const storedLocale = readStoredLocale();

  if (storedLocale) {
    return storedLocale;
  }

  const browserLocales = [...(navigator.languages ?? []), navigator.language].filter(
    (locale): locale is string => Boolean(locale),
  );

  return browserLocales.some((locale) => locale.toLowerCase().startsWith("zh"))
    ? CHINESE_LOCALE
    : ENGLISH_LOCALE;
}

function buildLocaleRoot(base: string, locale: string): string {
  const normalizedBase = base.endsWith("/") ? base : `${base}/`;

  return locale === CHINESE_LOCALE
    ? `${normalizedBase}${CHINESE_LOCALE}/`
    : normalizedBase;
}

function syncLocalePreference(pathname: string, base: string): void {
  writeStoredLocale(getLocaleFromPath(pathname, base));
}

function maybeRedirectRoot(base: string): boolean {
  const currentPath = normalizePath(stripBase(window.location.pathname, base));

  if (currentPath !== "/") {
    return false;
  }

  const preferredLocale = getPreferredLocale();

  if (preferredLocale !== CHINESE_LOCALE) {
    writeStoredLocale(ENGLISH_LOCALE);
    return false;
  }

  const target = `${buildLocaleRoot(base, CHINESE_LOCALE)}${window.location.search}${window.location.hash}`;
  const current = `${window.location.pathname}${window.location.search}${window.location.hash}`;

  if (target !== current) {
    window.location.replace(target);
  }

  return true;
}

const theme: Theme = {
  extends: DefaultTheme,
  enhanceApp({ router, siteData }) {
    if (!inBrowser) {
      return;
    }

    const base = siteData.value.base || "/";

    if (maybeRedirectRoot(base)) {
      return;
    }

    syncLocalePreference(window.location.pathname, base);

    const previousAfterRouteChange = router.onAfterRouteChange;
    router.onAfterRouteChange = async (to) => {
      syncLocalePreference(to, base);
      await previousAfterRouteChange?.(to);
    };
  },
};

export default theme;
