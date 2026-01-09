import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import LanguageDetector from "i18next-browser-languagedetector";
import en from "./assets/locales/en_US.json";
import zh_CN from "./assets/locales/zh_CN.json";
import ru_RU from "./assets/locales/ru_RU.json";
import zh_HK from "./assets/locales/zh_HK.json";
import ja_JP from "./assets/locales/ja_JP.json";

export const resources = {
  en_US: { translation: en },
  "en-US": { translation: en },
  en: { translation: en },
  zh_CN: { translation: zh_CN },
  "zh-CN": { translation: zh_CN },
  zh: { translation: zh_CN },
  zh_HK: { translation: zh_HK },
  "zh-HK": { translation: zh_HK },
  zhHK: { translation: zh_HK },
  ru_RU: { translation: ru_RU },
  "ru-RU": { translation: ru_RU },
  ru: { translation: ru_RU },
  ja_JP: { translation: ja_JP },
  "ja-JP": { translation: ja_JP },
  ja: { translation: ja_JP },
};

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    load: "currentOnly",
    fallbackLng: "en_US",
    supportedLngs: [
      "en_US",
      "en-US",
      "en",
      "zh_CN",
      "zh-CN",
      "zh",
      "ru_RU",
      "ru-RU",
      "ru",
      "zh_HK",
      "zh-HK",
      "zhhk",
      "ja_JP",
      "ja-JP",
      "ja",
    ],
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
