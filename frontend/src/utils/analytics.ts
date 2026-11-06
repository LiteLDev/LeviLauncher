import { matchPath } from "react-router-dom";
import { ROUTES } from "../constants/routes";
import { ANALYTICS_EVENT_NAME, readAnalyticsEnabled } from "./analyticsConsent";

export const GA_MEASUREMENT_ID = "G-ZHT4LTLTNK";

declare global {
  interface Window {
    dataLayer?: IArguments[];
    gtag?: (...args: unknown[]) => void;
    [key: `ga-disable-${string}`]: boolean | undefined;
  }
}

export const initializeAnalytics = (getPathname: () => string) => {
  const analyticsWindow = window;
  const disableKey = `ga-disable-${GA_MEASUREMENT_ID}` as const;
  let initialized = false;
  let enabled = false;
  let previousPath = "";
  let previousLocation = "";

  const getPage = (pathname: string) => {
    // Only known route templates are reported. Queries and dynamic IDs can
    // contain account details, instance names, search terms, or local paths.
    const path =
      Object.values(ROUTES).find((route) => matchPath(route, pathname)) || "/other";
    return { path, location: new URL(path, window.location.origin).href };
  };

  const trackPageView = (pathname: string) => {
    if (!enabled || !initialized) return;
    const page = getPage(pathname);
    if (pathname === previousPath) return;

    const parameters = {
      page_location: page.location,
      page_title: "LeviLauncher",
      page_referrer: previousLocation,
    };
    analyticsWindow.gtag!("set", parameters);
    analyticsWindow.gtag!("event", "page_view", {
      ...parameters,
      send_to: GA_MEASUREMENT_ID,
    });
    previousPath = pathname;
    previousLocation = page.location;
  };

  const applyConsent = (nextEnabled: boolean) => {
    enabled = nextEnabled;
    // This also blocks automatic events and a tag that finishes loading after
    // the user has withdrawn consent.
    analyticsWindow[disableKey] = !enabled;
    if (!enabled) {
      if (initialized) {
        analyticsWindow.gtag!("consent", "update", {
          analytics_storage: "denied",
        });
      }
      previousPath = "";
      previousLocation = "";
      return;
    }

    if (!initialized) {
      analyticsWindow.dataLayer = analyticsWindow.dataLayer || [];
      analyticsWindow.gtag = function (..._args: unknown[]) {
        analyticsWindow.dataLayer!.push(arguments);
      };
      analyticsWindow.gtag("consent", "default", {
        analytics_storage: "denied",
        ad_storage: "denied",
        ad_user_data: "denied",
        ad_personalization: "denied",
      });
      analyticsWindow.gtag("js", new Date());
      initialized = true;
    }

    analyticsWindow.gtag!("consent", "update", {
      analytics_storage: "granted",
    });
    if (!document.getElementById("google-analytics")) {
      analyticsWindow.gtag!("config", GA_MEASUREMENT_ID, {
        send_page_view: false,
        allow_google_signals: false,
        allow_ad_personalization_signals: false,
        page_location: getPage(getPathname()).location,
        page_title: "LeviLauncher",
        page_referrer: "",
      });
      const script = document.createElement("script");
      script.id = "google-analytics";
      script.async = true;
      script.src = `https://www.googletagmanager.com/gtag/js?id=${GA_MEASUREMENT_ID}`;
      document.head.appendChild(script);
    }
    trackPageView(getPathname());
  };

  window.addEventListener(ANALYTICS_EVENT_NAME, (event) => {
    applyConsent((event as CustomEvent<{ enabled: boolean }>).detail.enabled);
  });
  applyConsent(readAnalyticsEnabled());
  return trackPageView;
};
