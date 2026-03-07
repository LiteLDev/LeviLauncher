export const CLARITY_ENABLED_KEY = "ll.clarity.enabled";
export const CLARITY_CHOICE_KEY = "ll.clarity.choiceMade";
export const LEGACY_CLARITY_CHOICE_KEY = "ll.clarity.choiceMade3";
export const CLARITY_EVENT_NAME = "ll-clarity-consent-changed";

export const hasClarityChoiceMade = (): boolean => {
  try {
    if (localStorage.getItem(CLARITY_CHOICE_KEY)) {
      return true;
    }
    const legacyValue = localStorage.getItem(LEGACY_CLARITY_CHOICE_KEY);
    if (!legacyValue) {
      return false;
    }
    localStorage.setItem(CLARITY_CHOICE_KEY, legacyValue);
    localStorage.removeItem(LEGACY_CLARITY_CHOICE_KEY);
    return true;
  } catch {
    return false;
  }
};

export const persistClarityChoice = (enabled: boolean) => {
  try {
    localStorage.setItem(CLARITY_ENABLED_KEY, enabled ? "true" : "false");
    localStorage.setItem(CLARITY_CHOICE_KEY, "1");
    localStorage.removeItem(LEGACY_CLARITY_CHOICE_KEY);
    window.dispatchEvent(
      new CustomEvent(CLARITY_EVENT_NAME, { detail: { enabled } }),
    );
  } catch {}
};
