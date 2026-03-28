import DefaultTheme from "vitepress/theme";
import { inBrowser, type Theme } from "vitepress";
import "./custom.css";

// Key used for storing the user's locale preference in localStorage
const LOCALE_STORAGE_KEY = "levilauncher-docs-locale";

// Supported locale identifiers
const ENGLISH_LOCALE = "en-US";
const CHINESE_CN_LOCALE = "zh-CN";
const CHINESE_HK_LOCALE = "zh-HK"; // Traditional Chinese (Hong Kong)
const RUSSIAN_LOCALE = "ru-RU";    // Russian
const GERMAN_LOCALE = "de-DE";     // German
const JAPANESE_LOCALE = "ja-JP";   // Japanese —— ONLY ADDITION HERE

/**
 * Removes the base path from the given pathname.
 */
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

/**
 * Normalizes a path by removing query strings, hashes, 'index.html', and trailing slashes.
 */
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

/**
 * Determines the locale based on the current URL path.
 */
function getLocaleFromPath(pathname: string, base: string): string {
  const normalizedPath = normalizePath(stripBase(pathname, base));

  // Check for Japanese —— ONLY ADDITION HERE
  if (
    normalizedPath === `/${JAPANESE_LOCALE}` ||
    normalizedPath.startsWith(`/${JAPANESE_LOCALE}/`)
  ) {
    return JAPANESE_LOCALE;
  }

  // Check for German
  if (
    normalizedPath === `/${GERMAN_LOCALE}` ||
    normalizedPath.startsWith(`/${GERMAN_LOCALE}/`)
  ) {
    return GERMAN_LOCALE;
  }

  // Check for Russian
  if (
    normalizedPath === `/${RUSSIAN_LOCALE}` ||
    normalizedPath.startsWith(`/${RUSSIAN_LOCALE}/`)
  ) {
    return RUSSIAN_LOCALE;
  }

  // Check for Traditional Chinese (Hong Kong)
  if (
    normalizedPath === `/${CHINESE_HK_LOCALE}` ||
    normalizedPath.startsWith(`/${CHINESE_HK_LOCALE}/`)
  ) {
    return CHINESE_HK_LOCALE;
  }

  // Check for Simplified Chinese
  if (
    normalizedPath === `/${CHINESE_CN_LOCALE}` ||
    normalizedPath.startsWith(`/${CHINESE_CN_LOCALE}/`)
  ) {
    return CHINESE_CN_LOCALE;
  }

  // Default to English
  return ENGLISH_LOCALE;
}

/**
 * Reads the stored locale preference from localStorage.
 */
function readStoredLocale(): string | null {
  try {
    const locale = window.localStorage.getItem(LOCALE_STORAGE_KEY);

    // Validate that the stored value is a supported locale
    if (
      locale === ENGLISH_LOCALE ||
      locale === CHINESE_CN_LOCALE ||
      locale === CHINESE_HK_LOCALE ||
      locale === RUSSIAN_LOCALE ||
      locale === GERMAN_LOCALE ||
      locale === JAPANESE_LOCALE
    ) {
      return locale;
    }
  } catch {
    return null;
  }

  return null;
}

/**
 * Writes the locale preference to localStorage.
 */
function writeStoredLocale(locale: string): void {
  try {
    window.localStorage.setItem(LOCALE_STORAGE_KEY, locale);
  } catch {}
}

/**
 * Determines the user's preferred locale.
 * Priority: Stored Preference > Browser Language Settings > Default (English)
 */
function getPreferredLocale(): string {
  const storedLocale = readStoredLocale();

  if (storedLocale) {
    return storedLocale;
  }

  const browserLocales = [
    ...(navigator.languages ?? []),
    navigator.language,
  ].filter((locale): locale is string => Boolean(locale));

  // Check if the browser prefers Japanese —— ONLY ADDITION HERE
  const hasJapanese = browserLocales.some((locale) => {
    const lower = locale.toLowerCase();
    return lower.startsWith("ja");
  });

  // Check if the browser prefers German
  const hasGerman = browserLocales.some((locale) => {
    const lower = locale.toLowerCase();
    return lower.startsWith("de");
  });

  // Check if the browser prefers Russian
  const hasRussian = browserLocales.some((locale) => {
    const lower = locale.toLowerCase();
    return lower.startsWith("ru");
  });

  // Check if the browser prefers Traditional Chinese (zh-HK, zh-TW, etc.)
  const hasTraditionalChinese = browserLocales.some((locale) => {
    const lower = locale.toLowerCase();
    return lower.startsWith("zh-hk") || lower.startsWith("zh-tw") || lower === "zh-hant";
  });

  // Check if the browser prefers Simplified Chinese
  const hasSimplifiedChinese = browserLocales.some((locale) => {
    const lower = locale.toLowerCase();
    return lower.startsWith("zh-cn") || (lower.startsWith("zh") && !hasTraditionalChinese);
  });

  // Prioritize Japanese if detected —— ONLY ADDITION HERE
  if (hasJapanese) {
    return JAPANESE_LOCALE;
  }

  // Prioritize German if detected
  if (hasGerman) {
    return GERMAN_LOCALE;
  }

  // Prioritize Russian if detected
  if (hasRussian) {
    return RUSSIAN_LOCALE;
  }

  // Prioritize Traditional Chinese if detected
  if (hasTraditionalChinese) {
    return CHINESE_HK_LOCALE;
  }
  
  // Fallback to Simplified Chinese if detected
  if (hasSimplifiedChinese) {
    return CHINESE_CN_LOCALE;
  }

  // Default to English
  return ENGLISH_LOCALE;
}

/**
 * Constructs the root path for a specific locale.
 */
function buildLocaleRoot(base: string, locale: string): string {
  const normalizedBase = base.endsWith("/") ? base : `${base}/`;

  if (locale === CHINESE_CN_LOCALE) {
    return `${normalizedBase}${CHINESE_CN_LOCALE}/`;
  }
  
  if (locale === CHINESE_HK_LOCALE) {
    return `${normalizedBase}${CHINESE_HK_LOCALE}/`;
  }

  if (locale === RUSSIAN_LOCALE) {
    return `${normalizedBase}${RUSSIAN_LOCALE}/`;
  }

  if (locale === GERMAN_LOCALE) {
    return `${normalizedBase}${GERMAN_LOCALE}/`;
  }

  if (locale === JAPANESE_LOCALE) {
    return `${normalizedBase}${JAPANESE_LOCALE}/`; // —— ONLY ADDITION HERE
  }

  return normalizedBase;
}

/**
 * Syncs the current locale preference to localStorage based on the path.
 */
function syncLocalePreference(pathname: string, base: string): void {
  writeStoredLocale(getLocaleFromPath(pathname, base));
}

/**
 * Redirects the user from the root path to their preferred locale if necessary.
 * Returns true if a redirect was performed.
 */
function maybeRedirectRoot(base: string): boolean {
  const currentPath = normalizePath(stripBase(window.location.pathname, base));

  // Only handle redirects at the root path
  if (currentPath !== "/") {
    return false;
  }

  const preferredLocale = getPreferredLocale();

  // No redirect needed if the preferred locale is English (root)
  if (preferredLocale === ENGLISH_LOCALE) {
    writeStoredLocale(ENGLISH_LOCALE);
    return false;
  }

  const target = `${buildLocaleRoot(base, preferredLocale)}${window.location.search}${window.location.hash}`;
  const current = `${window.location.pathname}${window.location.search}${window.location.hash}`;

  if (target !== current) {
    window.location.replace(target);
  }

  return true;
}

const theme: Theme = {
  extends: DefaultTheme,
  enhanceApp({ router, siteData }) {
    // Only run in the browser environment
    if (!inBrowser) {
      return;
    }

    const base = siteData.value.base || "/";

    // Attempt to redirect from root if needed
    if (maybeRedirectRoot(base)) {
      return;
    }

    // Sync locale preference for the initial load
    syncLocalePreference(window.location.pathname, base);

    // Intercept route changes to keep locale preference in sync with the URL
    const previousAfterRouteChange = router.onAfterRouteChange;
    router.onAfterRouteChange = async (to) => {
      syncLocalePreference(to, base);
      await previousAfterRouteChange?.(to);
    };
  },
};

export default theme;