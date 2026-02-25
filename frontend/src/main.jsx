import "./style.css";
import i18n from "./i18n";
import { I18nextProvider } from "react-i18next";
import React from "react";
import { createRoot } from "react-dom/client";
import { ThemeProvider as NextThemesProvider } from "next-themes";
import { HeroUIProvider } from "@heroui/react";
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
