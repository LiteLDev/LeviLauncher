import { toast } from "@heroui/react";
import type { TFunction } from "i18next";

/** Keep failed objects selected and leave their confirmation open for retry. */
export async function deleteContentItems(
  paths: string[],
  remove: (path: string) => Promise<unknown>,
  retainSelection: (paths: string[]) => void,
  refresh: () => unknown,
  t: TFunction,
  isCurrentScope: () => boolean = () => true,
): Promise<boolean> {
  if (!paths.length) return false;
  const failed: Array<{ path: string; reason: string }> = [];
  for (const path of paths) {
    try {
      const result = await remove(path);
      if (typeof result === "string" && result.trim()) throw new Error(result);
    } catch (error) {
      failed.push({ path, reason: error instanceof Error ? error.message : String(error) });
    }
  }
  if (isCurrentScope()) retainSelection(failed.map((item) => item.path));
  const summary = t("contentpage.delete_result", {
    success: paths.length - failed.length,
    failed: failed.length,
  });
  toast(summary, {
    variant: failed.length ? (failed.length === paths.length ? "danger" : "warning") : "success",
    timeout: failed.length ? 6000 : 2000,
  });
  if (isCurrentScope()) await refresh();
  if (failed.length) {
    throw new Error(`${summary}\n${t("contentpage.delete_retry_failed")}\n${failed.map(({ path, reason }) => `${path.split(/[\\/]/).pop()}: ${reason}`).join("\n")}`);
  }
  return true;
}
