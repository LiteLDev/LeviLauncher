import { useState, useEffect, useCallback } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";

export const useAppNavigation = () => {
  const hasBackend = minecraft !== undefined;
  const location = useLocation();
  const navigate = useNavigate();

  const [splashVisible, setSplashVisible] = useState(true);
  const [revealStarted, setRevealStarted] = useState(false);
  const [isFirstLoad, setIsFirstLoad] = useState(false);
  const [isBeta, setIsBeta] = useState(false);
  const [hasEnteredLauncher, setHasEnteredLauncher] = useState(false);
  const [navLocked, setNavLocked] = useState<boolean>(() => {
    try {
      const h =
        typeof window !== "undefined" ? String(window.location.hash || "") : "";
      const initLock =
        h.startsWith("#/updating") ||
        h.startsWith("#/onboarding") ||
        Boolean((window as any).llNavLock);
      return initLock;
    } catch {
      return false;
    }
  });

  const isUpdatingMode = (() => {
    const p = String(location?.pathname || "");
    if (p === "/updating") return true;
    const h =
      typeof window !== "undefined" ? String(window.location.hash || "") : "";
    return h.startsWith("#/updating");
  })();

  const isOnboardingMode = (() => {
    const p = String(location?.pathname || "");
    if (p === "/onboarding") return true;
    const h =
      typeof window !== "undefined" ? String(window.location.hash || "") : "";
    return h.startsWith("#/onboarding");
  })();

  const tryNavigate = useCallback(
    (path: string | number) => {
      if (navLocked || isUpdatingMode || isOnboardingMode) return;
      if (
        location.pathname === "/settings" ||
        location.pathname === "/versionSettings"
      ) {
        try {
          window.dispatchEvent(
            new CustomEvent("ll-try-nav", { detail: { path } }),
          );
          return;
        } catch {}
      }
      navigate(path as any);
    },
    [navLocked, isUpdatingMode, isOnboardingMode, location.pathname, navigate],
  );

  useEffect(() => {
    if (location.pathname === "/" && revealStarted) {
      try {
        const onboarded = localStorage.getItem("ll.onboarded");
        if (onboarded) {
          setHasEnteredLauncher(true);
        }
      } catch {}
    }
  }, [location.pathname, revealStarted]);

  useEffect(() => {
    if (isUpdatingMode || isOnboardingMode) setNavLocked(true);
    else setNavLocked(Boolean((window as any).llNavLock));
  }, [isUpdatingMode, isOnboardingMode]);

  useEffect(() => {
    if (isUpdatingMode) {
      setSplashVisible(false);
      setRevealStarted(true);
      return;
    }
    const splashDurationMs = 1400;

    setIsFirstLoad(false);

    const tHide = setTimeout(() => setSplashVisible(false), splashDurationMs);
    const tHeader = setTimeout(
      () => setRevealStarted(true),
      splashDurationMs - 200,
    );
    return () => {
      clearTimeout(tHide);
      clearTimeout(tHeader);
    };
  }, [isUpdatingMode]);

  useEffect(() => {
    if (!revealStarted) return;
    if (isUpdatingMode) return;
    try {
      const onboarded = localStorage.getItem("ll.onboarded");
      if (!onboarded && location.pathname !== "/onboarding") {
        setNavLocked(true);
        navigate("/onboarding", { replace: true });
      }
    } catch {}
  }, [revealStarted, isUpdatingMode, location?.pathname]);

  useEffect(() => {
    try {
      setNavLocked(Boolean((window as any).llNavLock));
    } catch {}
    const handler = (e: any) => {
      try {
        if (isUpdatingMode) return;
        setNavLocked(Boolean(e?.detail?.lock ?? (window as any).llNavLock));
      } catch {}
    };
    window.addEventListener("ll-nav-lock-changed", handler as any);
    return () =>
      window.removeEventListener("ll-nav-lock-changed", handler as any);
  }, [isUpdatingMode]);

  useEffect(() => {
    if (!hasBackend) return;
    try {
      minecraft
        ?.GetIsBeta?.()
        .then((v: boolean) => setIsBeta(!!v))
        .catch(() => {});
    } catch {}
  }, [hasBackend]);

  return {
    navLocked,
    setNavLocked,
    splashVisible,
    revealStarted,
    isFirstLoad,
    hasEnteredLauncher,
    isBeta,
    isUpdatingMode,
    isOnboardingMode,
    tryNavigate,
    navigate,
  };
};
