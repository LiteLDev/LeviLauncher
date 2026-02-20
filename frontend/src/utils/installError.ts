import { TFunction } from "i18next";

const trimString = (value: unknown): string => String(value ?? "").trim();

const pickMessageLikeField = (value: unknown): string => {
  if (!value || typeof value !== "object") return "";
  const obj = value as Record<string, unknown>;
  const keys = ["message", "error", "err", "code", "reason", "detail"];
  for (const key of keys) {
    const v = obj[key];
    if (typeof v === "string" && v.trim()) return v.trim();
  }
  return "";
};

const normalizeErrorText = (raw: string): string => {
  let text = trimString(raw);
  if (!text) return "";

  for (let i = 0; i < 3; i += 1) {
    const noPrefix = text.replace(/^error\s*:\s*/i, "").trim();
    if (noPrefix) text = noPrefix;

    if (!text.startsWith("{") && !text.startsWith("\"{")) break;

    try {
      const parsed = JSON.parse(text);
      if (typeof parsed === "string") {
        const next = trimString(parsed);
        if (!next || next === text) break;
        text = next;
        continue;
      }
      const fromObject = pickMessageLikeField(parsed);
      if (!fromObject || fromObject === text) break;
      text = fromObject;
    } catch {
      break;
    }
  }

  return text;
};

export const resolveInstallError = (
  err: string,
  t: TFunction,
  typeLabel?: string,
): string => {
  const normalized = normalizeErrorText(err);
  const exactCodeMatch = normalized.match(
    /^((?:ERR|NH_ERR)_[A-Z0-9_]+)(?::\s*(.+))?$/i,
  );
  const embeddedCodeMatch = normalized.match(
    /((?:ERR|NH_ERR)_[A-Z0-9_]+)(?::\s*(.+))?/i,
  );

  const code = trimString(
    exactCodeMatch?.[1] || embeddedCodeMatch?.[1] || normalized,
  ).toUpperCase();
  const rest = trimString(exactCodeMatch?.[2] || embeddedCodeMatch?.[2] || "");

  if (!code) return t("installpage.error.unknown");

  let resolved = "";
  switch (code) {
    case "ERR_DLL_DEPENDENCY_MISSING":
      resolved = t("installpage.error.dll_dependency_missing");
      break;
    case "ERR_NAME_REQUIRED":
      resolved = t("installpage.error.name_required");
      break;
    case "ERR_MSIXVC_NOT_SPECIFIED":
      resolved = t("installpage.error.msixvc_not_specified");
      break;
    case "ERR_LIP_NOT_INSTALLED":
      resolved = t("installpage.error.lip_not_installed");
      break;
    case "ERR_UNKNOWN":
      resolved = t("installpage.error.unknown");
      break;
    case "NH_ERR_UNKNOWN":
      resolved = t("installpage.error.nh_err_unknown");
      break;
    case "NH_ERR_EXCEPTION":
      resolved = t("installpage.error.nh_err_exception");
      break;
    case "NH_ERR_INVALID_PARAMS":
      resolved = t("installpage.error.nh_err_invalid_params");
      break;
    case "NH_ERR_KEY_NOT_FOUND":
      resolved = t("installpage.error.nh_err_key_not_found");
      break;
    case "NH_ERR_UNAUTHORIZED_CALLER":
      resolved = t("installpage.error.nh_err_unauthorized_caller");
      break;
    case "NH_ERR_PIPE_OPEN_FAILED":
      resolved = t("installpage.error.nh_err_pipe_open_failed");
      break;
    case "NH_ERR_INPUT_NOT_FOUND":
      resolved = t("installpage.error.nh_err_input_not_found");
      break;
    case "NH_ERR_OUTPUT_DIR_INVALID":
      resolved = t("installpage.error.nh_err_output_dir_invalid");
      break;
    case "NH_ERR_PARSE_FAILED":
      resolved = t("installpage.error.nh_err_parse_failed");
      break;
    case "NH_ERR_EXTRACT_FAILED":
      resolved = t("installpage.error.nh_err_extract_failed");
      break;
    default:
      const key = `errors.${code}`;
      const translated = t(key, { typeLabel: typeLabel || "" });
      if (translated && translated !== key) {
        resolved = translated;
      } else {
        if (code.startsWith("ERR_") || code.startsWith("NH_ERR_")) {
          resolved = t("installpage.error.unknown");
        } else {
          resolved = normalizeErrorText(err) || code;
        }
      }
  }

  if (rest && resolved && resolved !== code) {
    return `${resolved} (${rest})`;
  }
  return resolved || code;
};
