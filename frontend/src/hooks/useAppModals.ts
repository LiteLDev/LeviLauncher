import { useState, useEffect, useCallback } from "react";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";

interface UseAppModalsOptions {
  hasBackend: boolean;
  revealStarted: boolean;
  isUpdatingMode: boolean;
  setNavLocked: (v: boolean) => void;
}

export const useAppModals = ({
  hasBackend,
  revealStarted,
  isUpdatingMode,
  setNavLocked,
}: UseAppModalsOptions) => {
  const [termsOpen, setTermsOpen] = useState<boolean>(false);
  const [termsCountdown, setTermsCountdown] = useState<number>(0);
  const [updateOpen, setUpdateOpen] = useState<boolean>(false);
  const [updateVersion, setUpdateVersion] = useState<string>("");
  const [updateBody, setUpdateBody] = useState<string>("");
  const [updateLoading, setUpdateLoading] = useState<boolean>(false);

  const acceptTerms = useCallback(() => {
    try {
      localStorage.setItem("ll.termsAccepted", "1");
    } catch {}
    setTermsOpen(false);
    setNavLocked(Boolean((window as any).llNavLock));
  }, [setNavLocked]);

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
          }
        })
        .catch(() => {});
    } catch {}
  }, [hasBackend, revealStarted, isUpdatingMode, setNavLocked]);

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
    updateOpen,
    updateVersion,
    updateBody,
    updateLoading,
    setUpdateOpen,
    setUpdateLoading,
  };
};
