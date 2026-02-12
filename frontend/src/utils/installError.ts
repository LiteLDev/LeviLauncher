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
