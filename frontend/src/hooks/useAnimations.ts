import { useState, useEffect } from "react";
import { MotionGlobalConfig } from "framer-motion";

const prefersReducedMotion = () =>
  typeof window !== "undefined" &&
  (window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ?? false);

export const shouldDisableAnimations = () => {
  if (typeof window === "undefined") {
    return false;
  }

  try {
    return (
      window.localStorage.getItem("app.disableAnimations") === "true" ||
      prefersReducedMotion()
    );
  } catch {
    return prefersReducedMotion();
  }
};

export const useAnimations = () => {
  const [disableAnimations, setDisableAnimations] = useState<boolean>(
    shouldDisableAnimations,
  );

  useEffect(() => {
    MotionGlobalConfig.skipAnimations = disableAnimations;

    if (disableAnimations) {
      const style = document.createElement("style");
      style.id = "disable-animations-style";
      style.innerHTML = `
        *, *::before, *::after {
          animation-duration: 0s !important;
          animation-delay: 0s !important;
          transition-duration: 0s !important;
          transition-delay: 0s !important;
        }
      `;
      document.head.appendChild(style);
    } else {
      const style = document.getElementById("disable-animations-style");
      if (style) {
        style.remove();
      }
    }

    return () => {
      MotionGlobalConfig.skipAnimations = false;
      const style = document.getElementById("disable-animations-style");
      if (style) {
        style.remove();
      }
    };
  }, [disableAnimations]);

  useEffect(() => {
    const reducedMotionMedia = window.matchMedia?.(
      "(prefers-reduced-motion: reduce)",
    );
    const handleAnimationsChange = () =>
      setDisableAnimations(shouldDisableAnimations());

    window.addEventListener("app-animations-changed", handleAnimationsChange);
    reducedMotionMedia?.addEventListener("change", handleAnimationsChange);

    return () => {
      window.removeEventListener(
        "app-animations-changed",
        handleAnimationsChange,
      );
      reducedMotionMedia?.removeEventListener("change", handleAnimationsChange);
    };
  }, []);

  return { disableAnimations };
};
