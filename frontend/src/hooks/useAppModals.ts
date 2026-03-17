import { useState, useEffect, useCallback } from "react";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { setNavLockReason } from "@/hooks/useAppNavigation";
import {
  hasClarityChoiceMade,
  persistClarityChoice,
} from "@/utils/clarityConsent";
import { useStartupInteractive } from "@/utils/startupState";

interface UseAppModalsOptions {
  hasBackend: boolean;
  isUpdatingMode: boolean;
}

const LIP_IGNORE_VERSION_KEY = "ll.ignoreLipVersion";

const normalizeVersion = (value: unknown): string =>
  String(value || "")
    .trim()
    .replace(/^v/i, "");

export const useAppModals = ({
  hasBackend,
  isUpdatingMode,
}: UseAppModalsOptions) => {
  const startupInteractive = useStartupInteractive();
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
            return;
          }
        })
        .catch(() => {});
    } catch {
      return;
    }
  }, []);

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
            return;
          }
          checkLipUpdate();
        })
        .catch(() => {
          checkLipUpdate();
        });
    } catch {}
  }, [checkLipUpdate]);

  const runPostTermsFlow = useCallback(() => {
    try {
      const onboarded = localStorage.getItem("ll.onboarded");
      if (!onboarded) {
        return;
      }
      if (!hasClarityChoiceMade()) {
        setClarityPromptOpen(true);
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

  const applyClarityChoice = useCallback(
    (enabled: boolean) => {
      persistClarityChoice(enabled);
      setClarityPromptOpen(false);
      checkUpdate();
    },
    [checkUpdate],
  );

  const acceptClarity = useCallback(() => {
    applyClarityChoice(true);
  }, [applyClarityChoice]);

  const declineClarity = useCallback(() => {
    applyClarityChoice(false);
  }, [applyClarityChoice]);

  useEffect(() => {
    if (!hasBackend) return;
    if (!startupInteractive) return;
    if (isUpdatingMode) return;
    try {
      const accepted = localStorage.getItem("ll.termsAccepted");
      if (!accepted) {
        setTermsOpen(true);
        return;
      }
      runPostTermsFlow();
    } catch {}
  }, [
    hasBackend,
    startupInteractive,
    isUpdatingMode,
    runPostTermsFlow,
  ]);

  useEffect(() => {
    const modalLocked =
      termsOpen || clarityPromptOpen || updateOpen || lipUpdateOpen;
    setNavLockReason("app-modal", modalLocked);
    return () => {
      if (modalLocked) {
        setNavLockReason("app-modal", false);
      }
    };
  }, [termsOpen, clarityPromptOpen, updateOpen, lipUpdateOpen]);

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
