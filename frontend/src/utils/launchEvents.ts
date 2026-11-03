import type { Events } from "@wailsio/runtime";

type LaunchEventState = {
  launchRequestActiveRef: { current: boolean };
  loadingDisclosure: { open: () => void; close: () => void };
  failedDisclosure: { open: () => void };
  setLaunchErrorCode: (code: string) => void;
};

export const subscribeToLaunchEvents = (
  events: Pick<typeof Events, "On">,
  {
    launchRequestActiveRef,
    loadingDisclosure,
    failedDisclosure,
    setLaunchErrorCode,
  }: LaunchEventState,
) => {
  const unlisteners = [
    events.On("mc.launch.start", () => {
      if (launchRequestActiveRef.current) loadingDisclosure.open();
    }),
    events.On("mc.launch.window_detected", () => {
      // A window can appear before UWP activation returns. Dismiss the wait
      // dialog now, while leaving completion and failure tracking active.
      loadingDisclosure.close();
    }),
    events.On("mc.launch.done", () => {
      launchRequestActiveRef.current = false;
      loadingDisclosure.close();
    }),
    events.On("mc.launch.failed", (event) => {
      launchRequestActiveRef.current = false;
      loadingDisclosure.close();
      const payload: any = event?.data ?? event;
      const first = Array.isArray(payload) ? payload[0] : payload;
      setLaunchErrorCode(String(first || "") || "ERR_LAUNCH_GAME");
      failedDisclosure.open();
    }),
  ];

  return () => {
    for (const unlisten of unlisteners) {
      try {
        unlisten();
      } catch {}
    }
  };
};
