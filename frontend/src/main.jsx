import "./style.css";
import React, { startTransition } from "react";
import { createRoot } from "react-dom/client";
import { ROUTES } from "./constants/routes";
import {
  markStartupDeferredWorkReady,
  markStartupPhase,
  measureStartupPhase,
  useStartupVisualReady,
} from "./utils/startupState";

const container = document.getElementById("root");

const root = createRoot(container);
let startupLifecycleCommitted = false;

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

const startupLanguage = (() => {
  try {
    return (
      localStorage.getItem("i18nextLng") ||
      navigator.language ||
      "en-US"
    );
  } catch {
    return navigator.language || "en-US";
  }
})();

document.documentElement.lang = String(startupLanguage).replace("_", "-");

const startupCopy = (() => {
  const language = String(startupLanguage).toLowerCase();
  if (language.startsWith("zh")) {
    return {
      loading: "正在启动…",
      error: "启动失败，请查看日志或稍后重试。",
      retry: "重新加载",
    };
  }

  return {
    loading: "Starting…",
    error: "Startup failed. Check the logs or try again.",
    retry: "Reload",
  };
})();

const StartupShell = ({
  errorMessage = "",
  visible = true,
  onRetry = null,
}) => {
  const hasError = Boolean(errorMessage);
  const retryButtonRef = React.useRef(null);

  React.useEffect(() => {
    if (hasError && visible) {
      retryButtonRef.current?.focus();
    }
  }, [hasError, visible]);

  return (
    <div
      role={hasError ? "alert" : "status"}
      aria-live={hasError ? "assertive" : "polite"}
      aria-atomic="true"
      aria-busy={!hasError && visible}
      style={{
        position: "fixed",
        inset: 0,
        zIndex: 9999,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        background:
          "linear-gradient(180deg, rgba(248,250,252,1) 0%, rgba(241,245,249,1) 100%)",
        color: "#0f172a",
        fontFamily:
          '"MiSans", "Segoe UI Variable", "Segoe UI", "Microsoft YaHei UI", sans-serif',
        opacity: visible ? 1 : 0,
        visibility: visible ? "visible" : "hidden",
        transition: "opacity 220ms ease, visibility 220ms ease",
        pointerEvents: visible ? "auto" : "none",
      }}
    >
      <div
        style={{
          display: "flex",
          maxWidth: "420px",
          flexDirection: "column",
          alignItems: "center",
          gap: "14px",
          padding: "24px",
          textAlign: "center",
        }}
      >
        <div
          aria-hidden="true"
          style={{
            display: "grid",
            width: "48px",
            height: "48px",
            placeItems: "center",
            borderRadius: "16px",
            background: "rgba(59, 130, 246, 0.12)",
            color: "#2563eb",
            fontSize: "24px",
            fontWeight: 800,
          }}
        >
          L
        </div>
        <strong style={{ fontSize: "18px", letterSpacing: "0.01em" }}>
          LeviLauncher
        </strong>
        <p
          style={{
            margin: 0,
            fontSize: "14px",
            lineHeight: 1.6,
            color: "#475569",
          }}
        >
          {errorMessage || startupCopy.loading}
        </p>
        {hasError && typeof onRetry === "function" ? (
          <button
            ref={retryButtonRef}
            type="button"
            onClick={onRetry}
            style={{
              minHeight: "38px",
              padding: "0 18px",
              border: 0,
              borderRadius: "999px",
              background: "#2563eb",
              color: "#ffffff",
              cursor: "pointer",
              font: "inherit",
              fontWeight: 700,
            }}
          >
            {startupCopy.retry}
          </button>
        ) : null}
      </div>
    </div>
  );
};

class AppErrorBoundary extends React.Component {
  constructor(props) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError() {
    return { hasError: true };
  }

  componentDidCatch(error, errorInfo) {
    console.error("[runtime] Unhandled render error", error, errorInfo);
  }

  render() {
    if (this.state.hasError) {
      return (
        <StartupShell
          errorMessage={startupCopy.error}
          onRetry={() => window.location.reload()}
        />
      );
    }

    return this.props.children;
  }
}

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
      markStartupDeferredWorkReady();
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
      <div
        data-startup-content
        inert={!visualReady}
        aria-hidden={!visualReady}
        style={{ display: "contents" }}
      >
        <RouterProviderComponent router={router} />
      </div>
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
      { CLARITY_ENABLED_KEY, CLARITY_EVENT_NAME },
    ] = await Promise.all([
      import("./App"),
      import("./i18n"),
      import("react-i18next"),
      import("next-themes"),
      import("./providers/HeroUIProvider"),
      import("react-router-dom"),
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
    let clarityConsentEnabled = false;
    let clarityClient = null;
    let clarityLoadPromise = null;

    const getClarityClient = () => {
      if (!clarityLoadPromise) {
        clarityLoadPromise = import("@microsoft/clarity").then(
          ({ default: Clarity }) => {
            clarityClient = Clarity;
            return Clarity;
          },
        );
      }
      return clarityLoadPromise;
    };

    const applyClarityConsent = async (enabled) => {
      clarityConsentEnabled = enabled;
      try {
        if (enabled) {
          const Clarity = await getClarityClient();
          if (!clarityConsentEnabled) {
            return;
          }
          if (!clarityInitialized) {
            Clarity.init(CLARITY_PROJECT_ID);
            clarityInitialized = true;
          }
          Clarity.consent(true);
          return;
        }

        if (clarityInitialized && clarityClient) {
          clarityClient.consent(false);
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

    void applyClarityConsent(clarityEnabledOnStart);

    window.addEventListener(CLARITY_EVENT_NAME, (event) => {
      const enabled = Boolean(event?.detail?.enabled);
      void applyClarityConsent(enabled);
    });

    startTransition(() => {
      root.render(
        <AppErrorBoundary>
          <HeroUIProvider>
            <NextThemesProvider
              attribute="class"
              defaultTheme="light"
              enableSystem
            >
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
          </HeroUIProvider>
        </AppErrorBoundary>,
      );
    });
  } catch (error) {
    console.error("[startup] Failed to bootstrap app", error);
    root.render(
      <StartupShell
        errorMessage={startupCopy.error}
        onRetry={() => window.location.reload()}
      />,
    );
  }
};

root.render(<StartupShell />);
markStartupPhase("ll-startup-bootloader-mounted");

window.requestAnimationFrame(() => {
  void bootstrapApp();
});
