// Troubleshooting guides: for a failure (an ERR_* code, or the kind of
// problem the technical detail points at) the likely causes, the steps that
// fix it, where the LeviMC FAQ covers it and which diagnostics apply.
//
// Texts live in the locale files under troubleshoot.guide.<id>.cause.<key>
// and troubleshoot.guide.<id>.step.<key>; the lists here fix which keys a
// guide has and in which order they are shown (most common cause first).

import { FAQ_ENTRIES, type FaqId, type FaqLanguage } from "@/utils/faqEntries";

// What the diagnostics look at; matches the backend's diagnose topics.
export type DiagnosticTopic =
  | "launch"
  | "install"
  | "download"
  | "lip"
  | "license"
  | "uwp"
  | "full";

// Things a step's button does; see useFixAction.
export type FixActionId =
  | "diagnose"
  | "install_vcruntime"
  | "install_gameinput"
  | "open_gaming_services"
  | "open_minecraft_store"
  | "open_preview_store"
  | "open_buy"
  | "open_devices"
  | "open_orders"
  | "enable_developer_mode"
  | "open_developer_settings"
  | "open_proxy_settings"
  | "open_date_settings"
  | "open_region_settings"
  | "open_apps_settings"
  | "open_storage_settings"
  | "open_mirror_settings"
  | "open_data_settings"
  | "open_process_settings"
  | "open_update_settings"
  | "open_download_page"
  | "open_account"
  | "open_logs"
  | "open_releases";

export type GuideStep = { key: string; action?: FixActionId };

export type Guide = {
  id: string;
  causes: string[];
  steps: GuideStep[];
  faq?: FaqId;
  topic?: DiagnosticTopic;
};

const g = <T extends Record<string, Omit<Guide, "id">>>(guides: T) =>
  Object.fromEntries(
    Object.entries(guides).map(([id, guide]) => [id, { id, ...guide }]),
  ) as { [K in keyof T]: Guide };

export const GUIDES = g({
  launch_game: {
    causes: ["components", "first_run", "stale_login", "mods", "graphics", "outdated"],
    steps: [
      { key: "diagnose", action: "diagnose" },
      { key: "components", action: "open_gaming_services" },
      { key: "official_once", action: "open_minecraft_store" },
      { key: "proxy" },
      { key: "clean_instance" },
      { key: "graphics" },
      { key: "report", action: "open_logs" },
    ],
    faq: "launch_game",
    topic: "launch",
  },
  game_running: {
    causes: ["still_running", "stuck"],
    steps: [
      { key: "close" },
      { key: "end_process", action: "open_process_settings" },
    ],
    topic: "launch",
  },
  game_files: {
    causes: ["moved", "antivirus", "incomplete"],
    steps: [
      { key: "diagnose", action: "diagnose" },
      { key: "antivirus" },
      { key: "reinstall" },
    ],
    faq: "missing_components",
    topic: "launch",
  },
  launch_window: {
    causes: ["loading", "mods", "graphics", "components"],
    steps: [
      { key: "wait" },
      { key: "diagnose", action: "diagnose" },
      { key: "clean_instance" },
      { key: "graphics" },
      { key: "logs", action: "open_logs" },
    ],
    faq: "crash_logs",
    topic: "launch",
  },
  write_access: {
    causes: ["in_use", "protected", "antivirus", "disk", "path"],
    steps: [
      { key: "close_game", action: "open_process_settings" },
      { key: "data_folder", action: "open_data_settings" },
      { key: "antivirus" },
      { key: "disk", action: "open_storage_settings" },
      { key: "diagnose", action: "diagnose" },
    ],
    faq: "non_ascii_path",
    topic: "launch",
  },
  license_not_entitled: {
    causes: ["not_owned", "other_account", "xbox_only", "preview_separate", "region", "device_limit", "refund"],
    steps: [
      { key: "check_account", action: "open_account" },
      { key: "diagnose", action: "diagnose" },
      { key: "buy", action: "open_buy" },
      { key: "preview", action: "open_preview_store" },
      { key: "region", action: "open_region_settings" },
      { key: "devices", action: "open_devices" },
    ],
    faq: "license_not_entitled",
    topic: "license",
  },
  license_trial: {
    causes: ["not_owned", "other_account", "device_limit", "game_pass"],
    steps: [
      { key: "check_account", action: "open_account" },
      { key: "devices", action: "open_devices" },
      { key: "official_once", action: "open_minecraft_store" },
      { key: "diagnose", action: "diagnose" },
    ],
    faq: "preview_license",
    topic: "license",
  },
  license_network: {
    causes: ["network", "proxy", "clock", "service"],
    steps: [
      { key: "diagnose", action: "diagnose" },
      { key: "proxy", action: "open_proxy_settings" },
      { key: "clock", action: "open_date_settings" },
      { key: "retry" },
    ],
    faq: "license_not_entitled",
    topic: "license",
  },
  license_package: {
    causes: ["mismatch", "catalog"],
    steps: [
      { key: "refresh", action: "open_download_page" },
      { key: "check_account", action: "open_account" },
    ],
    topic: "license",
  },
  account: {
    causes: ["changed", "elevated", "cache"],
    steps: [
      { key: "sign_in", action: "open_account" },
      { key: "not_admin" },
      { key: "retry" },
    ],
    faq: "switch_account",
    topic: "license",
  },
  download_network: {
    causes: ["blocked", "proxy", "stale_proxy", "mirror", "disk"],
    steps: [
      { key: "diagnose", action: "diagnose" },
      { key: "mirror", action: "open_mirror_settings" },
      { key: "source", action: "open_download_page" },
      { key: "proxy", action: "open_proxy_settings" },
      { key: "local" },
    ],
    faq: "download_failing",
    topic: "download",
  },
  version_list: {
    causes: ["github", "mirror", "proxy"],
    steps: [
      { key: "diagnose", action: "diagnose" },
      { key: "mirror", action: "open_mirror_settings" },
      { key: "retry" },
    ],
    faq: "download_failing",
    topic: "download",
  },
  lip_install: {
    causes: ["github", "proxy_down", "stale_proxy", "cache", "timeout"],
    steps: [
      { key: "diagnose", action: "diagnose" },
      { key: "mirror", action: "open_mirror_settings" },
      { key: "clear_cache", action: "open_mirror_settings" },
      { key: "system_proxy", action: "open_proxy_settings" },
      { key: "dns" },
      { key: "later" },
    ],
    faq: "lip_install_failed",
    topic: "lip",
  },
  lip_package: {
    causes: ["unsupported", "conflict", "network"],
    steps: [
      { key: "version" },
      { key: "conflict" },
      { key: "diagnose", action: "diagnose" },
    ],
    faq: "greyed_out",
    topic: "lip",
  },
  ll_unsupported: {
    causes: ["new_version", "preview", "uwp"],
    steps: [
      { key: "wait" },
      { key: "supported_version", action: "open_download_page" },
      { key: "manual" },
    ],
    faq: "ll_unsupported",
    topic: "lip",
  },
  vcruntime: {
    causes: ["missing", "damaged", "system"],
    steps: [
      { key: "install", action: "install_vcruntime" },
      { key: "official_once", action: "open_minecraft_store" },
      { key: "repair" },
    ],
    faq: "missing_dll",
    topic: "launch",
  },
  developer_mode: {
    causes: ["off", "policy"],
    steps: [
      { key: "enable", action: "enable_developer_mode" },
      { key: "settings", action: "open_developer_settings" },
      { key: "policy" },
    ],
    faq: "register",
    topic: "uwp",
  },
  uwp_conflict: {
    causes: ["store", "other_launcher"],
    steps: [
      { key: "backup" },
      { key: "uninstall", action: "open_apps_settings" },
      { key: "retry" },
    ],
    faq: "register",
    topic: "uwp",
  },
  uwp_register: {
    causes: ["developer_mode", "conflict", "dependency", "folder", "deployment"],
    steps: [
      { key: "diagnose", action: "diagnose" },
      { key: "developer_mode", action: "open_developer_settings" },
      { key: "conflict", action: "open_apps_settings" },
      { key: "restart" },
      { key: "logs", action: "open_logs" },
    ],
    faq: "register",
    topic: "uwp",
  },
  uwp_dependency: {
    causes: ["framework", "old_windows"],
    steps: [
      { key: "store", action: "open_minecraft_store" },
      { key: "windows_update" },
      { key: "other_version", action: "open_download_page" },
    ],
    faq: "uwp_support",
    topic: "uwp",
  },
  uwp_system_version: {
    causes: ["old_windows", "locked"],
    steps: [
      { key: "other_version", action: "open_download_page" },
      { key: "windows_update" },
    ],
    faq: "windows_update",
  },
  package_corrupt: {
    causes: ["incomplete", "disk", "antivirus", "format"],
    steps: [
      { key: "redownload", action: "open_download_page" },
      { key: "disk", action: "open_storage_settings" },
      { key: "antivirus" },
    ],
    faq: "download_failing",
    topic: "download",
  },
  install_failed: {
    causes: ["vcruntime", "disk", "path", "antivirus", "license"],
    steps: [
      { key: "diagnose", action: "diagnose" },
      { key: "vcruntime", action: "install_vcruntime" },
      { key: "path", action: "open_data_settings" },
      { key: "antivirus" },
      { key: "report", action: "open_logs" },
    ],
    faq: "install_crash",
    topic: "install",
  },
  gaming_services: {
    causes: ["missing", "ltsc", "broken", "proxy"],
    steps: [
      { key: "store" },
      { key: "official_once", action: "open_minecraft_store" },
      { key: "ltsc" },
      { key: "diagnose", action: "diagnose" },
      { key: "repair" },
      { key: "ignore" },
    ],
    faq: "gaming_services",
    topic: "launch",
  },
  update_failed: {
    causes: ["network", "permission", "asset", "antivirus"],
    steps: [
      { key: "retry" },
      { key: "mirror", action: "open_mirror_settings" },
      { key: "manual", action: "open_releases" },
      { key: "admin" },
    ],
    topic: "download",
  },
  migration: {
    causes: ["running", "locked", "permission"],
    steps: [
      { key: "close", action: "open_process_settings" },
      { key: "retry" },
      { key: "logs", action: "open_logs" },
    ],
    topic: "launch",
  },
});

export type GuideId = keyof typeof GUIDES;

const CODE_GUIDES: Record<string, GuideId> = {
  ERR_LAUNCH_GAME: "launch_game",
  ERR_GAME_CRASH_DURING_LAUNCH: "launch_game",
  ERR_WAIT_KERNEL_TIMEOUT: "launch_window",
  ERR_GAME_ALREADY_RUNNING: "game_running",
  ERR_GAME_RUNNING: "game_running",
  ERR_NOT_FOUND_GAME_DIR: "game_files",
  ERR_NOT_FOUND_EXE: "game_files",
  ERR_UWP_MANIFEST: "game_files",
  ERR_TARGET_NOT_FOUND: "game_files",
  ERR_WRITE_TARGET: "write_access",
  ERR_CREATE_TARGET_DIR: "write_access",
  ERR_ACCESS_VERSIONS_DIR: "write_access",
  ERR_ACCESS_INSTALLERS_DIR: "write_access",
  ERR_READ_VERSIONS_DIR: "write_access",
  ERR_RENAME_FAILED: "write_access",
  ERR_DELETE_FAILED: "write_access",
  ERR_COPY_FAILED: "write_access",
  ERR_COPY_WALK: "write_access",
  ERR_INHERIT_COPY_FAILED: "write_access",
  ERR_MC_NOT_AUTHORIZED: "license_not_entitled",
  ERR_LICENSE_NOT_ENTITLED: "license_not_entitled",
  ERR_LICENSE_FULL_REQUIRED: "license_trial",
  ERR_LICENSE_HTTP: "license_network",
  ERR_LICENSE_CATALOG: "license_network",
  ERR_AUTH_FAILED: "license_network",
  ERR_AUTH_TIMEOUT: "license_network",
  ERR_AUTH_DEVICE_TOKEN: "license_network",
  ERR_AUTH_DEVICE_PROVISION: "license_network",
  ERR_LICENSE_MISSING_KEY: "license_package",
  ERR_LICENSE_UNSUPPORTED: "license_package",
  ERR_AUTH_ACCOUNT_CHANGED: "account",
  ERR_AUTH_INTERACTION_REQUIRED: "account",
  ERR_AUTH_ACCOUNT_SELECTION: "account",
  ERR_AUTH_DEVICE_CACHE: "account",
  ERR_AUTH_DEVICE_BUSY: "account",
  ERR_UWP_DOWNLOAD_URL: "download_network",
  ERR_DOWNLOAD_FAILED: "download_network",
  ERR_LIP_INSTALL_FAILED: "lip_install",
  ERR_FETCH_LL_DB: "lip_install",
  ERR_LIP_PACKAGE_INSTALL_FAILED: "lip_install",
  ERR_LIP_PACKAGE_QUERY_FAILED: "lip_install",
  ERR_LIP_CACHE_CLEAN_FAILED: "lip_install",
  ERR_LL_NOT_SUPPORTED: "ll_unsupported",
  ERR_LL_VERSION_UNSUPPORTED: "ll_unsupported",
  ERR_VCRUNTIME_MISSING: "vcruntime",
  ERR_DLL_DEPENDENCY_MISSING: "vcruntime",
  ERR_DEV_MODE_REQUIRED: "developer_mode",
  ERR_UWP_DEVELOPER_MODE: "developer_mode",
  ERR_UWP_PACKAGE_CONFLICT: "uwp_conflict",
  ERR_UWP_REGISTER: "uwp_register",
  ERR_UWP_REGISTER_ROLLBACK: "uwp_register",
  ERR_UWP_UNREGISTER: "uwp_register",
  ERR_UWP_PREPARE: "uwp_register",
  ERR_UWP_NOT_REGISTERED: "uwp_register",
  ERR_UWP_LAUNCH: "uwp_register",
  ERR_UWP_QUERY: "uwp_register",
  ERR_REGISTER_FAILED: "uwp_register",
  ERR_UNREGISTER_FAILED: "uwp_register",
  ERR_UWP_DEPENDENCY: "uwp_dependency",
  ERR_UWP_SYSTEM_VERSION: "uwp_system_version",
  ERR_UWP_ARCHIVE: "package_corrupt",
  ERR_UWP_INTEGRITY: "package_corrupt",
  ERR_UWP_UNSAFE_ARCHIVE: "package_corrupt",
  ERR_UWP_RESOURCE_BUNDLE: "package_corrupt",
  ERR_NATIVE_MSIXVC: "package_corrupt",
  ERR_MSIXVC_UNSUPPORTED_FORMAT: "package_corrupt",
  NH_ERR_EXTRACT_FAILED: "package_corrupt",
  NH_ERR_PARSE_FAILED: "package_corrupt",
  ERR_APPX_INSTALL_FAILED: "install_failed",
  ERR_UWP_INSTALL: "install_failed",
  NH_ERR_UNKNOWN: "install_failed",
  NH_ERR_EXCEPTION: "install_failed",
  NH_ERR_PIPE_OPEN_FAILED: "install_failed",
  NH_ERR_OUTPUT_DIR_INVALID: "write_access",
  ERR_UPDATE_GENERIC: "update_failed",
  ERR_UPDATE_DOWNLOAD_FAILED: "update_failed",
  ERR_UPDATE_INSTALL_FAILED: "update_failed",
  ERR_UPDATE_VERIFY_FAILED: "update_failed",
  ERR_UPDATE_ADMIN_REQUIRED: "update_failed",
  ERR_UPDATE_ASSET_MISSING: "update_failed",
  ERR_UPDATE_RELEASE_UNAVAILABLE: "update_failed",
  ERR_UPDATE_RESTART_FAILED: "update_failed",
};

// Guides for failures that carry no known code, picked from the
// plain-language causes explainErrorDetail found in the detail text.
const CAUSE_GUIDES: Record<string, GuideId> = {
  developer_mode: "developer_mode",
  access_denied: "write_access",
  file_in_use: "write_access",
  disk_full: "write_access",
  path_too_long: "write_access",
  game_exited_early: "launch_game",
  game_no_window: "launch_window",
  corrupt_download: "package_corrupt",
};

const NETWORK_CAUSES = new Set([
  "dns",
  "tls",
  "timeout",
  "connection",
  "http_403",
  "http_404",
  "http_429",
  "http_5xx",
]);

// guideFor picks the guide for a failure: by its code, otherwise by what the
// detail text suggests (a network failure gets the network guide of the
// failing area, given by fallbackTopic).
export const guideFor = (
  code: string | undefined,
  causes: string[] = [],
  fallbackTopic?: DiagnosticTopic,
): Guide | undefined => {
  const c = String(code || "").trim().toUpperCase();
  if (c && CODE_GUIDES[c]) return GUIDES[CODE_GUIDES[c]];
  if (c.startsWith("ERR_INSTANCE_BACKUP_") && causes.length === 0) return undefined;
  for (const cause of causes) {
    if (CAUSE_GUIDES[cause]) return GUIDES[CAUSE_GUIDES[cause]];
  }
  if (causes.some((cause) => NETWORK_CAUSES.has(cause))) {
    if (fallbackTopic === "lip") return GUIDES.lip_install;
    if (fallbackTopic === "license") return GUIDES.license_network;
    return GUIDES.download_network;
  }
  return undefined;
};

export const guideIdForCode = (code: string): GuideId | undefined =>
  CODE_GUIDES[String(code || "").trim().toUpperCase()];

const FAQ_BASE = "https://levimc.org/";

export const faqLanguage = (lng: string | undefined): FaqLanguage => {
  const l = String(lng || "").toLowerCase().replace("-", "_");
  if (l === "zh_hk" || l === "zh_tw" || l === "zh_hant") return "zh-Hant";
  if (l.startsWith("zh")) return "zh-Hans";
  return "en";
};

// faqUrl links to an FAQ question in the reader's language (English for
// languages the FAQ is not translated into).
export const faqUrl = (id: FaqId, lng?: string): string => {
  const entry = FAQ_ENTRIES[id];
  const lang = faqLanguage(lng);
  const prefix = lang === "en" ? "" : `${lang}/`;
  return `${FAQ_BASE}${prefix}faq/play/windows/${entry.section}/#${encodeURIComponent(entry[lang])}`;
};

export const faqIndexUrl = (lng?: string): string => {
  const lang = faqLanguage(lng);
  return `${FAQ_BASE}${lang === "en" ? "" : `${lang}/`}faq/`;
};

// --- Account licenses -----------------------------------------------------

export type LicenseState =
  | "checking"
  | "authorized"
  | "trial"
  | "not_entitled"
  | "error";

// licenseInsight explains a combination of Release and Preview license
// states: the key of troubleshoot.license.<key>, or "" when both are fine
// or still being checked.
export const licenseInsight = (
  release: LicenseState,
  preview: LicenseState,
): string => {
  if (release === "checking" || preview === "checking") return "";
  const owned = (s: LicenseState) => s === "authorized";
  const denied = (s: LicenseState) => s === "not_entitled" || s === "trial";
  if (owned(release) && owned(preview)) return "";
  if (release === "error" || preview === "error") {
    if (owned(release) || owned(preview)) return "partial_error";
    return "error";
  }
  if (owned(preview) && denied(release)) return "preview_only";
  if (owned(release) && denied(preview)) return "release_only";
  if (release === "trial" || preview === "trial") return "trial";
  return "none";
};
