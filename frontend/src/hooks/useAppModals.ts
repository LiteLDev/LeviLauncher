import { useState, useEffect, useCallback } from "react";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";

interface UseAppModalsOptions {
  hasBackend: boolean;
  revealStarted: boolean;
  isUpdatingMode: boolean;
  setNavLocked: (v: boolean) => void;
}

const CLARITY_ENABLED_KEY = "ll.clarity.enabled";
const CLARITY_CHOICE_KEY = "ll.clarity.choiceMade3";
const CLARITY_EVENT_NAME = "ll-clarity-consent-changed";
const LIP_IGNORE_VERSION_KEY = "ll.ignoreLipVersion";

const normalizeVersion = (value: unknown): string =>
  String(value || "")
    .trim()
    .replace(/^v/i, "");

export const useAppModals = ({
  hasBackend,
  revealStarted,
  isUpdatingMode,
  setNavLocked,
}: UseAppModalsOptions) => {
  const [termsOpen, setTermsOpen] = useState<boolean>(false);
  const [termsCountdown, setTermsCountdown] = useState<number>(0);
  const [clarityPromptOpen, setClarityPromptOpen] = useState<boolean>(false);
  const [updateOpen, setUpdateOpen] = useState<boolean>(false);
  const [updateVersion, setUpdateVersion] = useState<string>("");
  const [updateBody, setUpdateBody] = useState<string>("");
  const [updateLoading, setUpdateLoading] = useState<boolean>(false);
  const [lipUpdateOpen, setLipUpdateOpen] = useState<boolean>(false);
  const [lipCurrentVersion, setLipCurrentVersion] = useState<string>("");
  const [lipLatestVersion, setLipLatestVersion] = useState<string>("");

  const checkLipUpdate = useCallback(() => {
    try {
      const ignored = normalizeVersion(
        localStorage.getItem(LIP_IGNORE_VERSION_KEY) || "",
      );
      const getter = (minecraft as any)?.GetLipStatus;
      if (typeof getter !== "function") {
        setNavLocked(Boolean((window as any).llNavLock));
        return;
      }
      getter()
        .then((res: any) => {
          const installed = Boolean(res?.installed);
          const upToDate = Boolean(res?.upToDate);
          const currentVersion = normalizeVersion(res?.currentVersion);
          const latestVersion = normalizeVersion(res?.latestVersion);
          if (
            installed &&
            !upToDate &&
            currentVersion &&
            latestVersion &&
            latestVersion !== ignored
          ) {
            setLipCurrentVersion(currentVersion);
            setLipLatestVersion(latestVersion);
            setLipUpdateOpen(true);
            setNavLocked(true);
            return;
          }
          setNavLocked(Boolean((window as any).llNavLock));
        })
        .catch(() => {
          setNavLocked(Boolean((window as any).llNavLock));
        });
    } catch {
      setNavLocked(Boolean((window as any).llNavLock));
    }
  }, [setNavLocked]);

  const checkUpdate = useCallback(() => {
    try {
      const ignored = localStorage.getItem("ll.ignoreVersion") || "";
      minecraft
        ?.CheckUpdate?.()
        .then((res: any) => {
          const ver = String(res?.version || "");
          const body = String(res?.body || "");
          const is = Boolean(res?.isUpdate);
          if (is && ver && ver !== ignored) {
            setUpdateVersion(ver);
            setUpdateBody(body);
            setUpdateOpen(true);
            setNavLocked(true);
            return;
          }
          checkLipUpdate();
        })
        .catch(() => {
          checkLipUpdate();
        });
    } catch {}
  }, [checkLipUpdate, setNavLocked]);

  const runPostTermsFlow = useCallback(() => {
    try {
      const onboarded = localStorage.getItem("ll.onboarded");
      if (!onboarded) {
        setNavLocked(Boolean((window as any).llNavLock));
        return;
      }
      const clarityChoiceMade = localStorage.getItem(CLARITY_CHOICE_KEY);
      if (!clarityChoiceMade) {
        setClarityPromptOpen(true);
        setNavLocked(true);
        return;
      }
      checkUpdate();
    } catch {}
  }, [checkUpdate, setNavLocked]);

  const acceptTerms = useCallback(() => {
    try {
      localStorage.setItem("ll.termsAccepted", "1");
    } catch {}
    setTermsOpen(false);
    setNavLocked(Boolean((window as any).llNavLock));
    runPostTermsFlow();
  }, [runPostTermsFlow, setNavLocked]);

  const applyClarityChoice = useCallback(
    (enabled: boolean) => {
      try {
        localStorage.setItem(CLARITY_ENABLED_KEY, enabled ? "true" : "false");
        localStorage.setItem(CLARITY_CHOICE_KEY, "1");
        window.dispatchEvent(
          new CustomEvent(CLARITY_EVENT_NAME, { detail: { enabled } }),
        );
      } catch {}

      setClarityPromptOpen(false);
      setNavLocked(Boolean((window as any).llNavLock));
      checkUpdate();
    },
    [checkUpdate, setNavLocked],
  );

  const acceptClarity = useCallback(() => {
    applyClarityChoice(true);
  }, [applyClarityChoice]);

  const declineClarity = useCallback(() => {
    applyClarityChoice(false);
  }, [applyClarityChoice]);

  useEffect(() => {
    if (!hasBackend) return;
    if (!revealStarted) return;
    if (isUpdatingMode) return;
    try {
      const accepted = localStorage.getItem("ll.termsAccepted");
      if (!accepted) {
        setTermsOpen(true);
        setNavLocked(true);
        return;
      }
      runPostTermsFlow();
    } catch {}
  }, [
    hasBackend,
    revealStarted,
    isUpdatingMode,
    setNavLocked,
    runPostTermsFlow,
  ]);

  useEffect(() => {
    if (!termsOpen) return;
    setTermsCountdown(10);
    const iv = setInterval(() => {
      setTermsCountdown((v) => (v > 0 ? v - 1 : 0));
    }, 1000);
    return () => clearInterval(iv);
  }, [termsOpen]);

  useEffect(() => {
    try {
      if (updateOpen) {
        document.body.style.overflow = "hidden";
        document.documentElement.style.overflow = "hidden";
        const root = document.getElementById("root");
        if (root) (root as HTMLElement).style.overflow = "hidden";
      } else {
        document.body.style.overflow = "";
        document.documentElement.style.overflow = "";
        const root = document.getElementById("root");
        if (root) (root as HTMLElement).style.overflow = "";
      }
    } catch {}
    return () => {
      try {
        document.body.style.overflow = "";
        document.documentElement.style.overflow = "";
        const root = document.getElementById("root");
        if (root) (root as HTMLElement).style.overflow = "";
      } catch {}
    };
  }, [updateOpen]);

  return {
    termsOpen,
    termsCountdown,
    acceptTerms,
    clarityPromptOpen,
    acceptClarity,
    declineClarity,
    updateOpen,
    updateVersion,
    updateBody,
    updateLoading,
    lipUpdateOpen,
    lipCurrentVersion,
    lipLatestVersion,
    setUpdateOpen,
    setUpdateLoading,
    setLipUpdateOpen,
  };
};
