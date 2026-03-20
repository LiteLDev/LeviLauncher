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
} as const;

type SupportedLocale = keyof typeof localeLoaders;

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

] as const;

const resolveLocale = (language: string): SupportedLocale => {
  const normalized = normalizeLanguage(language);

  if (normalized in localeLoaders) {
    return normalized as SupportedLocale;
  }

  return "en_US";
};

const localeBackend: BackendModule = {
  type: "backend",
  init: () => {},
  read(language, _namespace, callback) {
    const locale = resolveLocale(language);

    localeLoaders[locale]()
      .then((module) => {
        callback(null, module.default);
      })
      .catch((error) => {
        callback(error, null);
      });
  },
};

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
  });

export default i18n;
