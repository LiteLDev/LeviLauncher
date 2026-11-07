const AUTO_GRID_COLUMNS: Record<number, string> = {
  8: "grid-cols-[repeat(auto-fill,minmax(min(100%,8rem),1fr))]",
  10: "grid-cols-[repeat(auto-fill,minmax(min(100%,10rem),1fr))]",
  12: "grid-cols-[repeat(auto-fill,minmax(min(100%,12rem),1fr))]",
  14: "grid-cols-[repeat(auto-fill,minmax(min(100%,14rem),1fr))]",
  16: "grid-cols-[repeat(auto-fill,minmax(min(100%,16rem),1fr))]",
  18: "grid-cols-[repeat(auto-fill,minmax(min(100%,18rem),1fr))]",
  20: "grid-cols-[repeat(auto-fill,minmax(min(100%,20rem),1fr))]",
  22: "grid-cols-[repeat(auto-fill,minmax(min(100%,22rem),1fr))]",
  24: "grid-cols-[repeat(auto-fill,minmax(min(100%,24rem),1fr))]",
  28: "grid-cols-[repeat(auto-fill,minmax(min(100%,28rem),1fr))]",
  32: "grid-cols-[repeat(auto-fill,minmax(min(100%,32rem),1fr))]",
  40: "grid-cols-[repeat(auto-fill,minmax(min(100%,40rem),1fr))]",
};
const AUTO_GRID_SIZES = Object.keys(AUTO_GRID_COLUMNS).map(Number);

export const LAYOUT = {
  PAGE: {
    CONTAINER:
      "w-full max-w-[min(100%,var(--page-max-width))] mx-auto px-[var(--page-gutter)] pb-[var(--page-gutter)] pt-[var(--content-pt)] h-full flex flex-col gap-4 overflow-y-auto overflow-x-hidden",
    /** Cap for prose and forms so lines stay readable on very wide screens. */
    READABLE: "w-full max-w-[75rem] mx-auto",
    ANIMATION: {
      initial: { opacity: 0, y: 8 },
      animate: { opacity: 1, y: 0 },
      transition: { duration: 0.25 },
    },
  },
  GLASS_CARD: {
    BASE: "launcher-glass border border-black/5 dark:border-white/10 shadow-sm bg-surface/85 rounded-4xl",
    HEADER: "p-6 block border-b border-border dark:border-white/10",
    BODY: "flex flex-col gap-4 p-4",
  },
  CATALOG: {
    PAGE: "min-h-0 !overflow-hidden gap-3",
    HEADER_BODY: "p-4 min-[1200px]:p-6 flex flex-col gap-3",
    HEADER_ROW: "flex min-w-0 flex-wrap items-center gap-3",
    SEARCH_ROW: "flex min-w-0 flex-[1_1_30rem] flex-wrap items-center gap-2 [&>.button]:shrink-0",
    FILTERS: "grid grid-cols-2 min-[900px]:grid-cols-4 gap-3 [&>*]:min-w-0",
    RESULTS_BODY: "p-0 min-h-0 flex-1 overflow-hidden flex flex-col",
    RESULTS_SCROLL: "min-h-0 flex-1 overflow-y-auto overflow-x-hidden p-3 min-[1200px]:p-4 relative",
    FOOTER: "flex shrink-0 justify-center px-4 py-2 border-t border-border dark:border-white/5 bg-surface/50 launcher-material-panel",
  },
  GRID: {
    /**
     * Auto-fill grid: as many columns of at least `minRem` as fit. Tailwind
     * only generates classes it can see literally, so sizes come from this
     * list; other values round to the nearest listed size.
     */
    AUTO: (minRem: number) => {
      const size = AUTO_GRID_SIZES.reduce((best, candidate) =>
        Math.abs(candidate - minRem) < Math.abs(best - minRem) ? candidate : best,
      );
      return `grid gap-[var(--page-gap)] ${AUTO_GRID_COLUMNS[size]}`;
    },
  },
  NAVBAR_BG: "launcher-chrome bg-surface/85 backdrop-blur-sm",
};
