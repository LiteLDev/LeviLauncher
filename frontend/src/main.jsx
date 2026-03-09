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
import {
  CLARITY_ENABLED_KEY,
  CLARITY_EVENT_NAME,
} from "./utils/clarityConsent";

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

const EDITABLE_FOCUSABLE_SELECTOR = [
  'input:not([type="hidden"]):not([type="button"]):not([type="submit"]):not([type="reset"]):not([type="checkbox"]):not([type="radio"]):not([type="range"]):not([type="color"]):not([type="file"]):not([disabled])',
  "textarea:not([disabled])",
  "select:not([disabled])",
  '[contenteditable=""]',
  '[contenteditable="true"]',
].join(", ");

const isVisibleFocusableElement = (element) => {
  if (!(element instanceof HTMLElement)) {
    return false;
  }

  if (element.getAttribute("aria-hidden") === "true") {
    return false;
  }

  return element.getClientRects().length > 0;
};

const isEditableFocusableElement = (element) => {
  if (!(element instanceof HTMLElement)) {
    return false;
  }

  if (element.isContentEditable) {
    return isVisibleFocusableElement(element);
  }

  return (
    element.matches(EDITABLE_FOCUSABLE_SELECTOR) &&
    isVisibleFocusableElement(element)
  );
};

const getEditableFocusableElements = () =>
  Array.from(document.querySelectorAll(EDITABLE_FOCUSABLE_SELECTOR)).filter(
    isVisibleFocusableElement,
  );

const focusAdjacentEditableElement = (currentElement, direction) => {
  const editableElements = getEditableFocusableElements();
  const currentIndex = editableElements.findIndex(
    (element) => element === currentElement || element.contains(currentElement),
  );

  if (currentIndex === -1) {
    currentElement.blur();
    return;
  }

  const nextElement = editableElements[currentIndex + direction];

  if (!(nextElement instanceof HTMLElement)) {
    currentElement.blur();
    return;
  }

  nextElement.focus();
};

window.addEventListener(
  "keydown",
  (event) => {
    if (
      event.key !== "Tab" ||
      event.ctrlKey ||
      event.metaKey ||
      event.altKey
    ) {
      return;
    }

    const activeElement = document.activeElement;

    event.preventDefault();
    event.stopPropagation();
    event.stopImmediatePropagation();

    if (isEditableFocusableElement(activeElement)) {
      focusAdjacentEditableElement(activeElement, event.shiftKey ? -1 : 1);
      return;
    }

    if (activeElement instanceof HTMLElement && activeElement !== document.body) {
      activeElement.blur();
    }
  },
  true,
);

const router = createHashRouter(
  createRoutesFromElements(<Route path="/*" element={<App />} />),
);

const CLARITY_PROJECT_ID = "voq9l7h41c";
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

window.addEventListener(CLARITY_EVENT_NAME, (event) => {
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
