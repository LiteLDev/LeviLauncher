import "./style.css";
import React, { startTransition } from "react";
import { createRoot } from "react-dom/client";
import { ROUTES } from "./constants/routes";
import {
  markStartupInteractive,
  markStartupPhase,
  measureStartupPhase,
  useStartupVisualReady,
} from "./utils/startupState";

const container = document.getElementById("root");

const root = createRoot(container);
let startupLifecycleCommitted = false;

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

const getCurrentHashPath = () => {
  const hash = String(window.location.hash || "");
  const normalizedHash = hash.startsWith("#") ? hash.slice(1) : hash;
  const [pathname = ""] = normalizedHash.split("?");

  if (!pathname) {
    return ROUTES.home;
  }

  return pathname.startsWith("/") ? pathname : `/${pathname}`;
};

const shouldRedirectToOnboarding = (() => {
  const pathname = getCurrentHashPath();
  try {
    const onboarded = localStorage.getItem("ll.onboarded");
    return (
      !onboarded &&
      pathname !== ROUTES.updating &&
      pathname !== ROUTES.onboarding
    );
  } catch {
    return false;
  }
})();

if (shouldRedirectToOnboarding) {
  window.history.replaceState(null, "", `#${ROUTES.onboarding}`);
}

const StartupShell = ({ errorMessage = "", visible = true }) => (
  <div
    style={{
      position: "fixed",
      inset: 0,
      display: "flex",
      alignItems: "center",
      justifyContent: "center",
      background:
        "linear-gradient(180deg, rgba(248,250,252,1) 0%, rgba(241,245,249,1) 100%)",
      color: "#0f172a",
      fontFamily: 'var(--font-sans, sans-serif)',
      opacity: visible ? 1 : 0,
      visibility: visible ? "visible" : "hidden",
      transition: "opacity 220ms ease, visibility 220ms ease",
      pointerEvents: "none",
    }}
  >
    {errorMessage ? (
      <p
        style={{
          margin: 0,
          padding: "0 24px",
          fontSize: "14px",
          lineHeight: 1.6,
          color: "#475569",
          textAlign: "center",
        }}
      >
        {errorMessage}
      </p>
    ) : null}
  </div>
);

const StartupLifecycle = ({ children }) => {
  React.useEffect(() => {
    if (startupLifecycleCommitted) {
      return;
    }
    startupLifecycleCommitted = true;
    markStartupPhase("ll-startup-first-react-commit");
    measureStartupPhase(
      "ll-startup-render-after-bundle",
      "ll-startup-app-bundle-loaded",
      "ll-startup-first-react-commit",
    );

    const rafId = window.requestAnimationFrame(() => {
      markStartupInteractive();
    });

    return () => {
      window.cancelAnimationFrame(rafId);
    };
  }, []);

  return children;
};

const BootRoot = ({ router, RouterProviderComponent }) => {
  const visualReady = useStartupVisualReady();

  return (
    <>
      <RouterProviderComponent router={router} />
      <StartupShell visible={!visualReady} />
    </>
  );
};

const bootstrapApp = async () => {
  try {
    const [
      { default: App },
      { default: i18n, i18nReady },
      { I18nextProvider },
      { ThemeProvider: NextThemesProvider },
      { HeroUIProvider },
      { createHashRouter, RouterProvider, createRoutesFromElements, Route },
      { default: Clarity },
      { CLARITY_ENABLED_KEY, CLARITY_EVENT_NAME },
    ] = await Promise.all([
      import("./App"),
      import("./i18n"),
      import("react-i18next"),
      import("next-themes"),
      import("@heroui/react"),
      import("react-router-dom"),
      import("@microsoft/clarity"),
      import("./utils/clarityConsent"),
    ]);

    markStartupPhase("ll-startup-app-bundle-loaded");
    measureStartupPhase(
      "ll-startup-bundle-load",
      "ll-startup-bootloader-mounted",
      "ll-startup-app-bundle-loaded",
    );

    await i18nReady;

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

    startTransition(() => {
      root.render(
        <HeroUIProvider>
          <NextThemesProvider attribute="class" defaultTheme="light" enableSystem>
            <I18nextProvider i18n={i18n}>
              <React.StrictMode>
                <StartupLifecycle>
                  <BootRoot
                    router={router}
                    RouterProviderComponent={RouterProvider}
                  />
                </StartupLifecycle>
              </React.StrictMode>
            </I18nextProvider>
          </NextThemesProvider>
        </HeroUIProvider>,
      );
    });
  } catch (error) {
    console.error("[startup] Failed to bootstrap app", error);
    root.render(
      <StartupShell errorMessage="启动失败，请查看日志或稍后重试。" />,
    );
  }
};

root.render(<StartupShell />);
markStartupPhase("ll-startup-bootloader-mounted");

window.requestAnimationFrame(() => {
  void bootstrapApp();
});
