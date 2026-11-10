import i18next from "i18next";
import { toIntlLocale } from "@/utils/i18nUtils";

/** The current UI language as a BCP 47 tag (e.g. "zh-CN"), for Intl / toLocale* formatting. */
export const getIntlLocale = (): string =>
  toIntlLocale(i18next.resolvedLanguage || i18next.language || "");
