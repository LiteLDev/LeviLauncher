import { TFunction } from "i18next";

export const resolveInstallError = (
  err: string,
  t: TFunction,
  typeLabel?: string,
): string => {
  let code = String(err || "").trim();
  let rest = "";

  if (code.includes(":")) {
    const parts = code.split(":");
    code = parts[0].trim();
    rest = parts.slice(1).join(":").trim();
  }

  let resolved = "";
  switch (code) {
    case "ERR_DECOMPRESS_FAILED":
      resolved = t("installpage.error.decompress_failed");
      break;
    case "ERR_INVALID_ARGS":
      resolved = t("installpage.error.invalid_args");
      break;
    case "ERR_FILE_INVALID":
      resolved = t("installpage.error.file_invalid");
      break;
    case "ERR_LAUNCHER_NAME_MODIFIED":
      resolved = t("installpage.error.launcher_name_modified");
      break;
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
    default:
      const key = `errors.${code}`;
      const translated = t(key, { typeLabel: typeLabel || "" });
      if (translated && translated !== key) {
        resolved = translated;
      } else {
        if (code.startsWith("ERR_")) {
          resolved = t("installpage.error.unknown");
        } else {
          resolved = code;
        }
      }
  }

  if (rest && resolved && resolved !== code) {
    return `${resolved} (${rest})`;
  }
  return resolved || code;
};
