import { useState, useEffect, useCallback, useRef } from "react";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import { setNavLockReason } from "@/hooks/useAppNavigation";
import {
  hasAnalyticsChoiceMade,
  persistAnalyticsChoice,
} from "@/utils/analyticsConsent";
import { useStartupInteractive } from "@/utils/startupState";

interface UseAppModalsOptions {
  isUpdatingMode: boolean;
  isOnboardingMode: boolean;
}

export const useAppModals = ({
  isUpdatingMode,
  isOnboardingMode,
}: UseAppModalsOptions) => {
  const startupInteractive = useStartupInteractive();
  const previousOnboardingModeRef = useRef<boolean>(isOnboardingMode);
  const startupFlowArmedRef = useRef<boolean>(true);
  const [termsOpen, setTermsOpen] = useState<boolean>(false);
  const [termsCountdown, setTermsCountdown] = useState<number>(0);
  const [analyticsPromptOpen, setAnalyticsPromptOpen] = useState<boolean>(false);
  const [updateOpen, setUpdateOpen] = useState<boolean>(false);
  const [updateVersion, setUpdateVersion] = useState<string>("");
  const [updateBody, setUpdateBody] = useState<string>("");
  const [updateLoading, setUpdateLoading] = useState<boolean>(false);
  const [startupChecksComplete, setStartupChecksComplete] = useState(false);
  const [sponsorLaunchCount, setSponsorLaunchCount] = useState(0);
  const sponsorCheckedRef = useRef(false);

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
          }
        })
        .catch(() => {})
        .finally(() => setStartupChecksComplete(true));
    } catch {
      setStartupChecksComplete(true);
    }
  }, []);

  const runPostTermsFlow = useCallback(() => {
    try {
      const onboarded = localStorage.getItem("ll.onboarded");
      if (!onboarded) {
        return;
      }
      if (!hasAnalyticsChoiceMade()) {
        setAnalyticsPromptOpen(true);
        return;
      }
      checkUpdate();
    } catch {}
  }, [checkUpdate]);

  const acceptTerms = useCallback(() => {
    try {
      localStorage.setItem("ll.termsAccepted", "1");
    } catch {}
    setTermsOpen(false);
    runPostTermsFlow();
  }, [runPostTermsFlow]);

  const applyAnalyticsChoice = useCallback(
    (enabled: boolean) => {
      persistAnalyticsChoice(enabled);
      setAnalyticsPromptOpen(false);
      checkUpdate();
    },
    [checkUpdate],
  );

  const acceptAnalytics = useCallback(() => {
    applyAnalyticsChoice(true);
  }, [applyAnalyticsChoice]);

  const declineAnalytics = useCallback(() => {
    applyAnalyticsChoice(false);
  }, [applyAnalyticsChoice]);

  const runStartupModalFlow = useCallback(() => {
    if (!startupFlowArmedRef.current) {
      return;
    }

    startupFlowArmedRef.current = false;

    try {
      const accepted = localStorage.getItem("ll.termsAccepted");
      if (!accepted) {
        setTermsOpen(true);
        return;
      }
      runPostTermsFlow();
    } catch {}
  }, [runPostTermsFlow]);

  useEffect(() => {
    if (!startupInteractive) return;
    if (isUpdatingMode) return;
    if (isOnboardingMode) return;
    runStartupModalFlow();
  }, [
    startupInteractive,
    isUpdatingMode,
    isOnboardingMode,
    runStartupModalFlow,
  ]);

  useEffect(() => {
    const wasOnboardingMode = previousOnboardingModeRef.current;
    previousOnboardingModeRef.current = isOnboardingMode;

    if (!wasOnboardingMode || isOnboardingMode) return;
    if (!startupInteractive) return;
    if (isUpdatingMode) return;
    runStartupModalFlow();
  }, [
    isOnboardingMode,
    isUpdatingMode,
    runStartupModalFlow,
    startupInteractive,
  ]);

  useEffect(() => {
    if (
      !startupInteractive ||
      !startupChecksComplete ||
      isUpdatingMode ||
      isOnboardingMode ||
      termsOpen ||
      analyticsPromptOpen ||
      updateOpen ||
      sponsorCheckedRef.current
    ) {
      return;
    }

    sponsorCheckedRef.current = true;
    minecraft.TakeSponsorPrompt()
      .then((count) => {
        if (Number.isSafeInteger(count) && count > 0) {
          setSponsorLaunchCount(count);
        }
      })
      .catch(() => {});
  }, [
    startupInteractive,
    startupChecksComplete,
    isUpdatingMode,
    isOnboardingMode,
    termsOpen,
    analyticsPromptOpen,
    updateOpen,
  ]);

  const dismissSponsor = useCallback(() => setSponsorLaunchCount(0), []);

  useEffect(() => {
    const modalLocked =
      termsOpen ||
      analyticsPromptOpen ||
      updateOpen ||
      sponsorLaunchCount > 0;
    setNavLockReason("app-modal", modalLocked);
    return () => {
      if (modalLocked) {
        setNavLockReason("app-modal", false);
      }
    };
  }, [termsOpen, analyticsPromptOpen, updateOpen, sponsorLaunchCount]);

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
    analyticsPromptOpen,
    acceptAnalytics,
    declineAnalytics,
    updateOpen,
    updateVersion,
    updateBody,
    updateLoading,
    setUpdateOpen,
    setUpdateLoading,
    sponsorLaunchCount,
    dismissSponsor,
  };
};
