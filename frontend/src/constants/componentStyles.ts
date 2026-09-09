export const COMPONENT_STYLES = {
  select: {
    trigger:
      "bg-surface-secondary/50 data-[hovered]:bg-surface-tertiary/50 data-[focused]:border-brand-600 rounded-xl",
    popoverContent:
      "bg-overlay border border-black/5 dark:border-white/10 shadow-lg rounded-xl transform-gpu",
    listbox:
      "[&_li[data-focused]]:!ring-0 [&_li[data-focused]]:!outline-none [&_li[data-focused]]:!border-transparent",
  },
  dropdown: {
    content:
      "bg-overlay border border-black/5 dark:border-white/10 shadow-lg rounded-xl transform-gpu [&_li[data-focused]]:!ring-0 [&_li[data-focused]]:!outline-none [&_li[data-focused]]:!border-transparent",
  },
  dropdownTriggerButton:
    "bg-surface-secondary/50 data-[hovered]:bg-surface-tertiary/50 data-[focused]:border-brand-600 rounded-xl",
  listItem:
    "launcher-glass-list bg-surface/60 dark:bg-surface/65 border border-black/5 dark:border-white/10 shadow-sm hover:shadow-lg hover:bg-surface/70 rounded-3xl transition-all duration-300 ease-in-out transform hover:-translate-y-0.5",
  contentListItem:
    "launcher-glass-list bg-surface/85 border border-white/45 dark:border-white/12 shadow-[0_10px_30px_rgba(0,0,0,0.08)] hover:bg-surface/95 hover:border-white/60 dark:hover:border-white/20 hover:shadow-[0_14px_36px_rgba(0,0,0,0.12)] rounded-3xl transition-all duration-300 ease-out transform hover:-translate-y-0.5",
  input: {
    mainWrapper: "gap-1",
    inputWrapper:
      "bg-surface-secondary/70 dark:bg-surface/60 border-[1.5px] border-border/70 dark:border-white/10 hover:bg-surface-secondary/85 dark:hover:bg-surface-secondary/72 group-data-[hovered]:bg-surface-secondary/85 dark:group-data-[hovered]:bg-surface-secondary/72 hover:border-border/80 dark:hover:border-white/15 group-data-[hovered]:border-border/80 dark:group-data-[hovered]:border-white/15 focus-within:bg-surface dark:focus-within:bg-surface-secondary focus-within:!border-brand-500 group-data-[focused]:bg-surface dark:group-data-[focused]:bg-surface-secondary group-data-[focused]:!border-brand-500 shadow-sm rounded-xl transition-all duration-200 group-data-[invalid]:!border-rose-400 dark:group-data-[invalid]:!border-rose-500 group-data-[invalid]:bg-rose-50/35 dark:group-data-[invalid]:bg-rose-950/20 group-data-[disabled]:bg-surface-secondary/55 dark:group-data-[disabled]:bg-surface/45 group-data-[disabled]:shadow-none",
    innerWrapper: "gap-2",
    input:
      "text-foreground dark:text-zinc-100 placeholder:text-muted dark:placeholder:text-zinc-400 [&::-webkit-calendar-picker-indicator]:cursor-pointer [&::-webkit-calendar-picker-indicator]:opacity-60 dark:[&::-webkit-calendar-picker-indicator]:invert transition-colors",
    label:
      "text-foreground dark:text-zinc-300 font-medium transition-colors group-data-[invalid]:!text-rose-500 dark:group-data-[invalid]:!text-rose-400",
    clearButton:
      "text-muted hover:text-foreground dark:text-zinc-500 dark:hover:text-zinc-300 transition-colors",
    helperWrapper: "px-1 pt-1",
    description: "text-xs text-muted dark:text-zinc-500",
    errorMessage: "text-xs font-medium text-rose-500 dark:text-rose-400",
  },
  tabs: {
    tabList: "bg-surface-secondary/50 rounded-xl px-1",
    cursor: "bg-accent hover:bg-accent shadow-md",
    tabContent: "data-[selected]:text-white font-medium",
  },
  table: {
    thead: "rounded-none after:hidden",
    th: "bg-surface-secondary/55 dark:bg-surface/55 text-muted dark:text-zinc-400 font-semibold border-b border-border/80 dark:border-white/10 h-12 first:rounded-s-none last:rounded-e-none",
  },
  tableSticky: {
    thead: "rounded-none after:hidden [&>tr]:first:!shadow-none",
    th: "bg-surface-secondary/60 dark:bg-surface/60 text-muted dark:text-zinc-400 font-semibold border-b border-border/80 dark:border-white/10 h-12 first:rounded-s-none last:rounded-e-none backdrop-blur-none",
  },
};
