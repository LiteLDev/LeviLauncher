export const LAYOUT = {
  PAGE: {
    CONTAINER:
      "w-full max-w-full mx-auto px-4 pb-4 pt-[var(--content-pt)] h-full flex flex-col gap-6 overflow-y-auto overflow-x-hidden",
    ANIMATION: {
      initial: { opacity: 0, y: 8 },
      animate: { opacity: 1, y: 0 },
      transition: { duration: 0.25 },
    },
  },
  GLASS_CARD: {
    BASE: "border-none shadow-sm bg-white/50 dark:bg-zinc-950/40 rounded-4xl",
    HEADER: "p-6 block border-b border-default-200 dark:border-white/10",
    BODY: "flex flex-col gap-4 p-4",
  },
  NAVBAR_BG: "bg-white/50 dark:bg-zinc-950/40 backdrop-blur-md",
};
