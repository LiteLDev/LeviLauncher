import { useEffect, useState } from "react";

const STARTUP_INTERACTIVE_EVENT = "ll-startup-interactive";
const STARTUP_VISUAL_READY_EVENT = "ll-startup-visual-ready";

let startupInteractive = false;
let startupVisualReady = false;

const getPerformance = (): Performance | null => {
  if (typeof window === "undefined") {
    return null;
  }
  return window.performance || null;
};

export const markStartupPhase = (name: string) => {
  const perf = getPerformance();
  if (!perf) return;
  try {
    perf.mark(name);
    console.info(`[startup] ${name}`);
  } catch {}
};

export const measureStartupPhase = (
  name: string,
  startMark: string,
  endMark: string,
) => {
  const perf = getPerformance();
  if (!perf) return;
  try {
    perf.measure(name, startMark, endMark);
    const entries = perf.getEntriesByName(name, "measure");
    const lastEntry = entries[entries.length - 1];
    if (lastEntry) {
      console.info(`[startup] ${name}: ${lastEntry.duration.toFixed(1)}ms`);
    }
  } catch {}
};

export const isStartupInteractive = (): boolean => startupInteractive;
export const isStartupVisualReady = (): boolean => startupVisualReady;

export const markStartupInteractive = () => {
  if (startupInteractive) return;
  startupInteractive = true;
  markStartupPhase("ll-startup-interactive-ready");
  measureStartupPhase(
    "ll-startup-time-to-interactive",
    "ll-startup-bootloader-mounted",
    "ll-startup-interactive-ready",
  );
  window.dispatchEvent(new CustomEvent(STARTUP_INTERACTIVE_EVENT));
};

export const markStartupVisualReady = () => {
  if (startupVisualReady) return;
  startupVisualReady = true;
  markStartupPhase("ll-startup-visual-ready");
  measureStartupPhase(
    "ll-startup-time-to-visual-ready",
    "ll-startup-bootloader-mounted",
    "ll-startup-visual-ready",
  );
  window.dispatchEvent(new CustomEvent(STARTUP_VISUAL_READY_EVENT));
};

export const subscribeStartupInteractive = (
  listener: (ready: boolean) => void,
) => {
  const onInteractive = () => {
    listener(true);
  };

  if (startupInteractive) {
    listener(true);
    return () => {};
  }

  window.addEventListener(STARTUP_INTERACTIVE_EVENT, onInteractive);
  return () => {
    window.removeEventListener(STARTUP_INTERACTIVE_EVENT, onInteractive);
  };
};

export const subscribeStartupVisualReady = (
  listener: (ready: boolean) => void,
) => {
  const onVisualReady = () => {
    listener(true);
  };

  if (startupVisualReady) {
    listener(true);
    return () => {};
  }

  window.addEventListener(STARTUP_VISUAL_READY_EVENT, onVisualReady);
  return () => {
    window.removeEventListener(STARTUP_VISUAL_READY_EVENT, onVisualReady);
  };
};

export const useStartupInteractive = (): boolean => {
  const [ready, setReady] = useState<boolean>(() => isStartupInteractive());

  useEffect(() => {
    return subscribeStartupInteractive(setReady);
  }, []);

  return ready;
};

export const useStartupVisualReady = (): boolean => {
  const [ready, setReady] = useState<boolean>(() => isStartupVisualReady());

  useEffect(() => {
    return subscribeStartupVisualReady(setReady);
  }, []);

  return ready;
};
