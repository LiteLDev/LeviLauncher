import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import LanguageDetector from "i18next-browser-languagedetector";
import en from "@/assets/locales/en_US.json";
import zh from "@/assets/locales/zh_CN.json";
import ru from "@/assets/locales/ru_RU.json";
import ja from "@/assets/locales/ja_JP.json";

export const resources = {
  en_US: { translation: en },
  "en-US": { translation: en },
  en: { translation: en },
  zh_CN: { translation: zh },
  "zh-CN": { translation: zh },
  zh: { translation: zh },
  ru_RU: { translation: ru },
  "ru-RU": { translation: ru },
  ru: { translation: ru },
  ja_JP: { translation: ja },
  "ja_JP": { translation: ja },
  ja: { translation: ja },
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
