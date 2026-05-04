/**
 * Normalize language code to standard format used in the application.
 * e.g. "en-us" -> "en_US", "zh-cn" -> "zh_CN"
 */
export const normalizeLanguage = (lng: string): string => {
  if (!lng) return "en_US";
  const normalized = lng.trim().replace(/_/g, "-").toLowerCase();
  const aliases: Record<string, string> = {
    en: "en_US",
    "en-us": "en_US",
    zh: "zh_CN",
    "zh-cn": "zh_CN",
    "zh-hans": "zh_CN",
    "zh-hans-cn": "zh_CN",
    zhhk: "zh_HK",
    "zh-hk": "zh_HK",
    "zh-hant": "zh_HK",
    "zh-hant-hk": "zh_HK",
    "zh-tw": "zh_HK",
    ja: "ja_JP",
    "ja-jp": "ja_JP",
    ru: "ru_RU",
    "ru-ru": "ru_RU",
    ko: "ko_KR",
    "ko-kr": "ko_KR",
    fr: "fr_FR",
    "fr-fr": "fr_FR",
    de: "de_DE",
    "de-de": "de_DE",
    es: "es_ES",
    "es-es": "es_ES",
    pt: "pt_PT",
    "pt-pt": "pt_PT",
    it: "it_IT",
    "it-it": "it_IT",
  };

  return aliases[normalized] || aliases[normalized.split("-")[0]] || "en_US";
};
