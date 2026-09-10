import { toast } from "@heroui/react";
import i18n from "@/i18n";
import { OpenPathDir } from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { OpenModsExplorer } from "bindings/github.com/liteldev/LeviLauncher/modsservice";

export function showDirectoryOpenError(error: unknown): void {
  const message = error instanceof Error ? error.message : String(error ?? "");
  toast.danger(i18n.t("common.open_folder_failed"), {
    description: message || undefined,
    timeout: 6000,
  });
}

async function openWithFeedback(open: () => Promise<void>): Promise<boolean> {
  try {
    await open();
    return true;
  } catch (error) {
    showDirectoryOpenError(error);
    return false;
  }
}

/** Resolves to false after reporting a failure, including for fire-and-forget buttons. */
export function openDirectory(path: string): Promise<boolean> {
  return openWithFeedback(() => OpenPathDir(path));
}

export function openModsDirectory(versionName: string): Promise<boolean> {
  return openWithFeedback(() => OpenModsExplorer(versionName));
}
