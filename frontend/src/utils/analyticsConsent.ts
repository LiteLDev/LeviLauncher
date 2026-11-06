export const ANALYTICS_ENABLED_KEY = "ll.clarity.enabled";
export const ANALYTICS_CHOICE_KEY = "ll.clarity.choiceMade";
export const ANALYTICS_EVENT_NAME = "ll-analytics-consent-changed";

export const hasAnalyticsChoiceMade = (): boolean => {
  try {
    return Boolean(
      localStorage.getItem(ANALYTICS_CHOICE_KEY) ||
      localStorage.getItem("ll.clarity.choiceMade3"),
    );
  } catch {
    return false;
  }
};

export const readAnalyticsEnabled = (): boolean => {
  try {
    return (
      hasAnalyticsChoiceMade() &&
      localStorage.getItem(ANALYTICS_ENABLED_KEY) === "true"
    );
  } catch {
    return false;
  }
};

export const persistAnalyticsChoice = (enabled: boolean) => {
  try {
    localStorage.setItem(ANALYTICS_ENABLED_KEY, enabled ? "true" : "false");
    localStorage.setItem(ANALYTICS_CHOICE_KEY, "1");
  } catch {}
  window.dispatchEvent(
    new CustomEvent(ANALYTICS_EVENT_NAME, { detail: { enabled } }),
  );
};
