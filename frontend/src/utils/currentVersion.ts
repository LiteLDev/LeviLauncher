const KEY = "ll.currentVersionName";
export const CURRENT_VERSION_CHANGED_EVENT = "ll-current-version-changed";

export type CurrentVersionChangedDetail = {
  prev: string;
  next: string;
  source: string;
  at: number;
};

const emitCurrentVersionChanged = (
  prev: string,
  next: string,
  source: string,
): void => {
  try {
    window.dispatchEvent(
      new CustomEvent<CurrentVersionChangedDetail>(CURRENT_VERSION_CHANGED_EVENT, {
        detail: {
          prev: String(prev || ""),
          next: String(next || ""),
          source: String(source || "unknown"),
          at: Date.now(),
        },
      }),
    );
  } catch {}
};

export function readCurrentVersionName(): string {
  try {
    return localStorage.getItem(KEY) || "";
  } catch {
    return "";
  }
}

export function saveCurrentVersionName(name: string, source = "unknown"): void {
  const next = String(name || "").trim();
  const prev = readCurrentVersionName();
  if (prev === next) return;

  try {
    if (next) localStorage.setItem(KEY, next);
    else localStorage.removeItem(KEY);
  } catch {}
  emitCurrentVersionChanged(prev, next, source);
}

export function clearCurrentVersionName(source = "unknown"): void {
  const prev = readCurrentVersionName();
  if (!prev) return;
  try {
    localStorage.removeItem(KEY);
  } catch {}
  emitCurrentVersionChanged(prev, "", source);
}
