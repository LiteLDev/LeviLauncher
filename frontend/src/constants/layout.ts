export const LAYOUT = {
  PAGE: {
    CONTAINER:
      "w-full max-w-full mx-auto px-4 pb-4 pt-[var(--content-pt)] h-full flex flex-col gap-4 overflow-y-auto overflow-x-hidden",
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
    FOOTER: "flex shrink-0 justify-center px-4 py-2 border-t border-border dark:border-white/5 bg-surface/50 launcher-material-blur",
  },
  NAVBAR_BG: "launcher-chrome bg-surface/85 backdrop-blur-sm",
};
