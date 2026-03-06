import { TFunction } from "i18next";

export const resolveImportError = (err: string, t: TFunction): string => {
  const code = String(err || "").trim();
  switch (code) {
    case "ERR_NO_PLAYER":
      return t("contentpage.no_player_selected");
    case "ERR_INVALID_NAME":
      return t("mods.err_invalid_name");
    case "ERR_ACCESS_VERSIONS_DIR":
      return t("mods.err_access_versions_dir");
    case "ERR_CREATE_TARGET_DIR":
      return t("mods.err_create_target_dir");
    case "ERR_OPEN_ZIP":
      return t("mods.err_open_zip");
    case "ERR_MANIFEST_NOT_FOUND":
      return t("mods.err_manifest_not_found");
    case "ERR_INVALID_PACKAGE":
      return t("mods.err_invalid_package");
    case "ERR_DUPLICATE_FOLDER":
      return t("mods.err_duplicate_folder");
    case "ERR_READ_ZIP_ENTRY":
      return t("mods.err_read_zip_entry");
    case "ERR_WRITE_FILE":
      return t("mods.err_write_file");
    case "ERR_LL_MANAGED_IN_VERSION_SETTINGS":
      return t("mods.err_ll_managed_in_version_settings");
    case "ERR_NO_UPDATE_SOURCE":
      return t("mods.no_update_source");
    case "ERR_ALREADY_LATEST":
      return t("mods.update_latest");
    default:
      if (code) {
        const translated = t(`errors.${code}`);
        if (translated !== `errors.${code}`) {
          return translated;
        }
      }
      return code || t("mods.err_unknown");
  }
};
