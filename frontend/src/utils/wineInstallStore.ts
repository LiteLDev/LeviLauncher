import { useSyncExternalStore } from "react";
import { Events } from "@wailsio/runtime";
import { StartWineInstaller } from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import type { WineInstallProgress } from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";

export const wineInstallStages = ["rosetta", "runtime", "extract", "initialize", "select"] as const;

type InstallState = {
  status: "idle" | "running" | "consent" | "done" | "failed";
  progress: WineInstallProgress;
  error: string;
  completed: number;
};

const initialProgress = { stage: "rosetta", status: "working", downloaded: 0, total: 0 };
const listeners = new Set<() => void>();
let state: InstallState = { status: "idle", progress: initialProgress, error: "", completed: 0 };

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => { listeners.delete(listener); };
}

function update(patch: Partial<InstallState>) {
  state = { ...state, ...patch };
  for (const listener of listeners) listener();
}

export function useWineInstall() {
  return useSyncExternalStore(subscribe, () => state);
}

// The installation and its progress subscription outlive the Wine tab.
export async function installWine(consent: boolean) {
  if (state.status === "running") return;
  update({ status: "running", progress: initialProgress, error: "" });
  const unsubscribe = Events.On("wine.install.progress", ({ data }) => {
    const progress = data as WineInstallProgress | null;
    if (progress && wineInstallStages.some((stage) => stage === progress.stage)) {
      update({ progress });
    }
  });
  try {
    await StartWineInstaller(consent);
    update({ status: "done", completed: state.completed + 1 });
  } catch (failure) {
    const error = failure instanceof Error ? failure.message : String(failure);
    if (/\bERR_WINE_ROSETTA_CONSENT\b/.test(error)) update({ status: "consent" });
    else update({ status: "failed", error });
  } finally {
    unsubscribe();
  }
}
