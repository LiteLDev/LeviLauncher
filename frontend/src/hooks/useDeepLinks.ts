import { useEffect, useRef } from "react";
import { Events } from "@wailsio/runtime";
import { TakePending } from "bindings/github.com/liteldev/LeviLauncher/internal/app/deeplinkservice";
import { routeTo } from "@/constants/routes";

/** Emitted by the backend when a levilauncher:// link is waiting (internal/deeplink). */
const DEEP_LINK_EVENT = "deeplink.open";

/**
 * Opens levilauncher:// links on the LeviMC Mods page of the mod they name.
 * The backend keeps the latest link until it is taken, so a link that
 * started the launcher, or came while navigation was locked (onboarding, an
 * update, a running task), is opened once navigation is possible. The link
 * is validated by the backend; the page only ever offers the install, which
 * the user confirms after picking the instance.
 */
export const useDeepLinks = (
  blocked: boolean,
  tryNavigate: (path: string | number) => void,
) => {
  const navigateRef = useRef(tryNavigate);
  navigateRef.current = tryNavigate;

  useEffect(() => {
    if (blocked) return;
    let active = true;
    const take = async () => {
      try {
        const link = await TakePending();
        if (!link || link.action !== "install" || !link.modulePath) return;
        navigateRef.current(routeTo.lipPackageFromLink(link));
      } catch (err) {
        console.warn("Failed to take the pending levilauncher:// link", err);
      }
    };
    void take();
    const off = Events.On(DEEP_LINK_EVENT, () => {
      if (active) void take();
    });
    return () => {
      active = false;
      off();
    };
  }, [blocked]);
};
