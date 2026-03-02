import "./style.css";
import i18n from "./i18n";
import { I18nextProvider } from "react-i18next";
import React from "react";
import { createRoot } from "react-dom/client";
import { ThemeProvider as NextThemesProvider } from "next-themes";
import { HeroUIProvider } from "@heroui/react";
import Clarity from "@microsoft/clarity";
import App from "./App";
import {
  createHashRouter,
  RouterProvider,
  createRoutesFromElements,
  Route,
} from "react-router-dom";

const container = document.getElementById("root");

const root = createRoot(container);

const coarsePointerMedia = window.matchMedia?.("(pointer: coarse)");
const isTouchDevice =
  navigator.maxTouchPoints > 0 || Boolean(coarsePointerMedia?.matches);

if (isTouchDevice) {
  window.addEventListener(
    "wheel",
    (e) => {
      if (e.ctrlKey) {
        e.preventDefault();
      }
    },
    { passive: false },
  );

  const preventGestureZoom = (e) => {
    e.preventDefault();
  };

  window.addEventListener("gesturestart", preventGestureZoom);
  window.addEventListener("gesturechange", preventGestureZoom);
  window.addEventListener("gestureend", preventGestureZoom);
}

window.addEventListener("contextmenu", (e) => {
  if (import.meta.env.DEV) return;
  const target = e.target;
  const tagName = target.tagName;
  const isInput =
    tagName === "INPUT" || tagName === "TEXTAREA" || target.isContentEditable;

  if (isInput) {
    const type = target.getAttribute("type")?.toLowerCase();
    if (
      !type ||
      ["text", "password", "email", "number", "search", "url", "tel"].includes(
        type,
      ) ||
      tagName === "TEXTAREA"
    ) {
      return;
    }
  }

  e.preventDefault();
});

const router = createHashRouter(
  createRoutesFromElements(<Route path="/*" element={<App />} />),
);

const CLARITY_PROJECT_ID = "voq9l7h41c";
const CLARITY_ENABLED_KEY = "ll.clarity.enabled";
let clarityInitialized = false;

const applyClarityConsent = (enabled) => {
  try {
    if (enabled) {
      if (!clarityInitialized) {
        Clarity.init(CLARITY_PROJECT_ID);
        clarityInitialized = true;
      }
      Clarity.consent(true);
      return;
    }

    if (clarityInitialized) {
      Clarity.consent(false);
    }
  } catch (error) {
    console.error("Failed to apply Clarity consent", error);
  }
};

const clarityEnabledOnStart = (() => {
  try {
    return localStorage.getItem(CLARITY_ENABLED_KEY) === "true";
  } catch {
    return false;
  }
})();

applyClarityConsent(clarityEnabledOnStart);

window.addEventListener("ll-clarity-consent-changed", (event) => {
  const enabled = Boolean(event?.detail?.enabled);
  applyClarityConsent(enabled);
});

root.render(
  <HeroUIProvider>
    <NextThemesProvider attribute="class" defaultTheme="light" enableSystem>
      <I18nextProvider i18n={i18n}>
        <React.StrictMode>
          <RouterProvider router={router} />
        </React.StrictMode>
      </I18nextProvider>
    </NextThemesProvider>
  </HeroUIProvider>,
);
