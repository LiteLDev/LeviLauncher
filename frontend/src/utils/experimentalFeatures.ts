export const EXPERIMENTAL_INSTANCE_BACKUP_KEY =
  "app.experimental.instanceBackup";
export const EXPERIMENTAL_FEATURES_EVENT_NAME =
  "app-experimental-features-changed";

export const readExperimentalInstanceBackupEnabled = (): boolean => {
  try {
    return localStorage.getItem(EXPERIMENTAL_INSTANCE_BACKUP_KEY) === "true";
  } catch {
    return false;
  }
};

export const persistExperimentalInstanceBackupEnabled = (enabled: boolean) => {
  try {
    localStorage.setItem(
      EXPERIMENTAL_INSTANCE_BACKUP_KEY,
      enabled ? "true" : "false",
    );
    window.dispatchEvent(
      new CustomEvent(EXPERIMENTAL_FEATURES_EVENT_NAME, {
        detail: { instanceBackup: enabled },
      }),
    );
  } catch {}
};
