/**
 * Normalize language code to standard format used in the application.
 * e.g. "en-us" -> "en_US", "zh-cn" -> "zh_CN"
 */
export const normalizeLanguage = (lng: string): string => {
  if (!lng) return "en_US";
  const lower = lng.toLowerCase();
  if (lower === "en-us" || lower === "en") return "en_US";
  if (lower === "zh-cn" || lower === "zh") return "zh_CN";
  if (lower === "ja-jp" || lower === "ja") return "ja_JP";
  if (lower === "ru-ru" || lower === "ru") return "ru_RU";
  if (lower === "zh-hk" || lower === "zhhk") return "zh_HK";
  return lng;
};
