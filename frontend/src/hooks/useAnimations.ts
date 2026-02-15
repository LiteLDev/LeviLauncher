import { useState, useEffect } from "react";
import { MotionGlobalConfig } from "framer-motion";

export const useAnimations = () => {
  const [disableAnimations, setDisableAnimations] = useState<boolean>(() => {
    try {
      return localStorage.getItem("app.disableAnimations") === "true";
    } catch {
      return false;
    }
  });

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
      const style = document.getElementById("disable-animations-style");
      if (style) {
        style.remove();
      }
    };
  }, [disableAnimations]);

  useEffect(() => {
    const handleAnimationsChange = () => {
      try {
        const val = localStorage.getItem("app.disableAnimations") === "true";
        setDisableAnimations(val);
      } catch {}
    };
    window.addEventListener("app-animations-changed", handleAnimationsChange);
    return () => {
      window.removeEventListener(
        "app-animations-changed",
        handleAnimationsChange,
      );
    };
  }, []);

  return { disableAnimations };
};
