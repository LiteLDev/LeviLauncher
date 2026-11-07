import { notify } from "@/utils/activity";
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
  const reasons = failed.map(({ path, reason }) => `${path.split(/[\\/]/).pop()}: ${reason}`).join("\n");
  // The notice keeps every failure reason readable in the Activity center.
  // Failures are recorded silently: the confirmation shows them from the
  // thrown error.
  notify(failed.length
    ? {
      variant: failed.length === paths.length ? "danger" : "warning",
      title: t("activity.delete_failed_title"),
      description: `${summary}\n${reasons}`,
      source: "content",
      silent: true,
    }
    : { variant: "success", title: summary, source: "content" });
  if (isCurrentScope()) await refresh();
  if (failed.length) {
    throw new Error(`${summary}\n${t("contentpage.delete_retry_failed")}\n${reasons}`);
  }
  return true;
}
