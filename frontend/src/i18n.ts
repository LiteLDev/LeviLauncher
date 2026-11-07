/// <reference types="vite/client" />
import i18n from "i18next";
import type { BackendModule } from "i18next";
import { initReactI18next } from "react-i18next";
import LanguageDetector from "i18next-browser-languagedetector";
import { normalizeLanguage } from "@/utils/i18nUtils";

const localeLoaders = {
  en_US: () => import("@/assets/locales/en_US.json"),
  zh_CN: () => import("@/assets/locales/zh_CN.json"),
  ru_RU: () => import("@/assets/locales/ru_RU.json"),
  ja_JP: () => import("@/assets/locales/ja_JP.json"),
  zh_HK: () => import("@/assets/locales/zh_HK.json"),
  de_DE: () => import("@/assets/locales/de_DE.json"),
  es_ES: () => import("@/assets/locales/es_ES.json"),
  ko_KR: () => import("@/assets/locales/ko_KR.json"),
  fr_FR: () => import("@/assets/locales/fr_FR.json"),
  it_IT: () => import("@/assets/locales/it_IT.json"),
  pt_PT: () => import("@/assets/locales/pt_PT.json"),
  pl_PL: () => import("@/assets/locales/pl_PL.json"),
} as const;

type SupportedLocale = keyof typeof localeLoaders;

// Feature fragments live in assets/locales/fragments/<feature>/<locale>.json and
// are deep-merged over the base locale, so features can add strings without
// editing the shared locale files.
const fragmentLoaders = import.meta.glob<{ default: Record<string, unknown> }>(
  "./assets/locales/fragments/*/*.json",
);

const isPlainObject = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);

const deepMerge = (
  target: Record<string, unknown>,
  source: Record<string, unknown>,
): Record<string, unknown> => {
  for (const [key, value] of Object.entries(source)) {
    const current = target[key];
    target[key] =
      isPlainObject(current) && isPlainObject(value)
        ? deepMerge({ ...current }, value)
        : value;
  }
  return target;
};

const loadLocaleResources = async (
  locale: SupportedLocale,
): Promise<Record<string, unknown>> => {
  const base = (await localeLoaders[locale]()).default as Record<
    string,
    unknown
  >;
  const fragments = await Promise.all(
    Object.entries(fragmentLoaders)
      .filter(([path]) => path.endsWith(`/${locale}.json`))
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([, load]) => load().then((module) => module.default)),
  );
  return fragments.reduce(
    (merged, fragment) => deepMerge(merged, fragment),
    deepMerge({}, base),
  );
};

const supportedLngs = [
  "en_US",
  "en-US",
  "en",
  "zh_CN",
  "zh-CN",
  "zh",
  "ru_RU",
  "ru-RU",
  "ru",
  "ja_JP",
  "ja-JP",
  "ja",
  "zh_HK",
  "zh-HK",
  "zhhk",
  "ko_KR",
  "ko-KR",
  "ko",
  "fr_FR",
  "fr-FR",
  "fr",
  "de_DE",
  "de-DE",
  "de",
  "es_ES",
  "es-ES",
  "es",
  "pt_PT",
  "pt-PT",
  "pt",
  "it_IT",
  "it-IT",
  "it",
  "pl_PL",
  "pl-PL",
  "pl",

] as const;

const resolveLocale = (language: string): SupportedLocale => {
  const normalized = normalizeLanguage(language);

  if (normalized in localeLoaders) {
    return normalized as SupportedLocale;
  }

  return "en_US";
};

const syncDocumentLanguage = (language: string) => {
  if (typeof document === "undefined") {
    return;
  }

  document.documentElement.lang = resolveLocale(language).replace("_", "-");
};

const localeBackend: BackendModule = {
  type: "backend",
  init: () => {},
  read(language, _namespace, callback) {
    const locale = resolveLocale(language);

    loadLocaleResources(locale)
      .then((resources) => {
        callback(null, resources);
      })
      .catch((error) => {
        callback(error, null);
      });
  },
};

i18n.on("languageChanged", syncDocumentLanguage);

export const i18nReady = i18n
  .use(LanguageDetector)
  .use(localeBackend)
  .use(initReactI18next)
  .init({
    load: "currentOnly",
    partialBundledLanguages: true,
    fallbackLng: "en_US",
    supportedLngs,
    lowerCaseLng: false,
    nonExplicitSupportedLngs: true,
    detection: {
      order: ["localStorage", "navigator"],
      caches: ["localStorage"],
    },
    interpolation: {
      escapeValue: false,
    },
  })
  .then((result) => {
    syncDocumentLanguage(i18n.resolvedLanguage || i18n.language);
    return result;
  });

export default i18n;
