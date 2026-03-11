import { useState, useEffect, useCallback } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { ROUTES } from "@/constants/routes";

type NavLockListener = (locked: boolean) => void;

const navLockReasons = new Set<string>();
const navLockListeners = new Set<NavLockListener>();

const emitNavLockChange = () => {
  const locked = navLockReasons.size > 0;
  navLockListeners.forEach((listener) => {
    try {
      listener(locked);
    } catch {}
  });
};

export const getNavLockState = (): boolean => navLockReasons.size > 0;

export const subscribeNavLock = (listener: NavLockListener) => {
  navLockListeners.add(listener);
  return () => {
    navLockListeners.delete(listener);
  };
};

export const setNavLockReason = (reason: string, locked: boolean) => {
  const key = String(reason || "app").trim() || "app";
  const hadKey = navLockReasons.has(key);
  if (locked) {
    if (hadKey) return;
    navLockReasons.add(key);
    emitNavLockChange();
    return;
  }
  if (!hadKey) return;
  navLockReasons.delete(key);
  emitNavLockChange();
};

export const setAppNavLock = (locked: boolean) => {
  setNavLockReason("app", locked);
};

export const useAppNavigation = () => {
  const hasBackend = minecraft !== undefined;
  const location = useLocation();
  const navigate = useNavigate();

  const [splashVisible, setSplashVisible] = useState(true);
  const [revealStarted, setRevealStarted] = useState(false);
  const [isFirstLoad, setIsFirstLoad] = useState(false);
  const [isBeta, setIsBeta] = useState(false);
  const [hasEnteredLauncher, setHasEnteredLauncher] = useState(false);
  const [navLocked, setNavLockedState] = useState<boolean>(() => getNavLockState());

  const isUpdatingMode = (() => {
    const p = String(location?.pathname || "");
    if (p === ROUTES.updating) return true;
    const h =
      typeof window !== "undefined" ? String(window.location.hash || "") : "";
    return h.startsWith(`#${ROUTES.updating}`);
  })();

  const isOnboardingMode = (() => {
    const p = String(location?.pathname || "");
    if (p === ROUTES.onboarding) return true;
    const h =
      typeof window !== "undefined" ? String(window.location.hash || "") : "";
    return h.startsWith(`#${ROUTES.onboarding}`);
  })();

  const tryNavigate = useCallback(
    (path: string | number) => {
      if (navLocked || isUpdatingMode || isOnboardingMode) return;
      if (
        location.pathname === ROUTES.settings ||
        location.pathname === ROUTES.instanceSettings
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
    if (location.pathname === ROUTES.home && revealStarted) {
      try {
        const onboarded = localStorage.getItem("ll.onboarded");
        if (onboarded) {
          setHasEnteredLauncher(true);
        }
      } catch {}
    }
  }, [location.pathname, revealStarted]);

  useEffect(() => {
    return subscribeNavLock(setNavLockedState);
  }, []);

  useEffect(() => {
    setNavLockReason("updating-route", isUpdatingMode);
    return () => {
      if (isUpdatingMode) {
        setNavLockReason("updating-route", false);
      }
    };
  }, [isUpdatingMode]);

  useEffect(() => {
    setNavLockReason("onboarding-route", isOnboardingMode);
    return () => {
      if (isOnboardingMode) {
        setNavLockReason("onboarding-route", false);
      }
    };
  }, [isOnboardingMode]);

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
      if (!onboarded && location.pathname !== ROUTES.onboarding) {
        setNavLockReason("onboarding-route", true);
        navigate(ROUTES.onboarding, { replace: true });
      }
    } catch {}
  }, [revealStarted, isUpdatingMode, location?.pathname, navigate]);

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
    setNavLocked: setAppNavLock,
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
