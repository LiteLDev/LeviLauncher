import React from "react";

/**
 * Manages scroll position restoration and page-change scroll reset.
 * Shared by content list pages (WorldsList, SkinPacks, ResourcePacks, BehaviorPacks, etc.)
 */
export const useScrollManager = (
  scrollRef: React.RefObject<HTMLDivElement | null>,
  restoreDeps: React.DependencyList,
  resetDeps: React.DependencyList,
) => {
  const lastScrollTopRef = React.useRef<number>(0);
  const restorePendingRef = React.useRef<boolean>(false);

  const collectScrollTargets = React.useCallback(() => {
    const seen = new Set<unknown>();
    const targets: Array<Window | HTMLElement> = [];

    const add = (target: Window | HTMLElement | null | undefined) => {
      if (!target) return;
      if (seen.has(target)) return;
      seen.add(target);
      targets.push(target);
    };

    add(window);
    add((document.scrollingElement as HTMLElement) || document.documentElement);
    add(document.body);

    const walk = (seed: HTMLElement | null) => {
      let el: HTMLElement | null = seed;
      while (el) {
        add(el);
        el = el.parentElement;
      }
    };

    walk(scrollRef.current);

    return targets;
  }, []);

  /** Call before any state update that changes the list to save current scroll position */
  const saveScrollPosition = React.useCallback(() => {
    try {
      lastScrollTopRef.current = scrollRef.current
        ? scrollRef.current.scrollTop
        : window.scrollY || 0;
      restorePendingRef.current = true;
    } catch {}
  }, []);

  /** Get current scroll position (for manual save before async operations) */
  const getCurrentScrollTop = React.useCallback(() => {
    return (
      scrollRef.current?.scrollTop ??
      (document.scrollingElement as any)?.scrollTop ??
      0
    );
  }, []);

  // Restore scroll position after data changes
  React.useLayoutEffect(() => {
    if (!restorePendingRef.current) return;
    requestAnimationFrame(() => {
      try {
        if (scrollRef.current)
          scrollRef.current.scrollTop = lastScrollTopRef.current;
        else window.scrollTo({ top: lastScrollTopRef.current });
      } catch {}
    });
    restorePendingRef.current = false;
  }, restoreDeps);

  // Reset scroll to top on page change
  React.useEffect(() => {
    const resetScroll = () => {
      try {
        const active = document.activeElement as HTMLElement | null;
        if (active && scrollRef.current && scrollRef.current.contains(active)) {
          active.blur();
        }
      } catch {}

      for (const target of collectScrollTargets()) {
        if (target === window) {
          window.scrollTo({ top: 0, left: 0, behavior: "auto" });
          continue;
        }
        if (target instanceof HTMLElement) {
          target.scrollTop = 0;
          target.scrollLeft = 0;
        }
      }
    };

    resetScroll();
    const raf = requestAnimationFrame(resetScroll);
    const t0 = window.setTimeout(resetScroll, 0);
    const t1 = window.setTimeout(resetScroll, 120);
    return () => {
      cancelAnimationFrame(raf);
      clearTimeout(t0);
      clearTimeout(t1);
    };
  }, resetDeps);

  return {
    lastScrollTopRef,
    restorePendingRef,
    saveScrollPosition,
    getCurrentScrollTop,
  };
};
