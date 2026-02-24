export const COMPONENT_STYLES = {
  select: {
    trigger:
      "bg-default-100/50 dark:bg-zinc-800/50 data-[hover=true]:bg-default-200/50 dark:data-[hover=true]:bg-zinc-700/50 data-[focus=true]:border-primary-600 rounded-xl",
    popoverContent:
      "bg-white dark:bg-zinc-900 backdrop-blur-sm backdrop-saturate-150 border border-black/5 dark:border-white/10 shadow-lg rounded-xl transform-gpu",
    listbox:
      "[&_li[data-focus=true]]:!ring-0 [&_li[data-focus=true]]:!outline-none [&_li[data-focus=true]]:!border-transparent",
  },
  dropdown: {
    content:
      "bg-white dark:bg-zinc-900 backdrop-blur-sm backdrop-saturate-150 border border-black/5 dark:border-white/10 shadow-lg rounded-xl transform-gpu [&_li[data-focus=true]]:!ring-0 [&_li[data-focus=true]]:!outline-none [&_li[data-focus=true]]:!border-transparent",
  },
  dropdownTriggerButton:
    "bg-default-100/50 dark:bg-zinc-800/50 data-[hover=true]:bg-default-200/50 dark:data-[hover=true]:bg-zinc-700/50 data-[focus=true]:border-primary-600 rounded-xl",
  listItem:
    "bg-white/60 dark:bg-zinc-950/65 border border-black/5 dark:border-white/10 shadow-sm hover:shadow-lg hover:bg-white/70 dark:hover:bg-zinc-900/70 rounded-3xl transition-all duration-300 ease-in-out transform hover:-translate-y-0.5",
  contentListItem:
    "bg-gradient-to-br from-white/52 to-white/34 dark:from-zinc-900/50 dark:to-zinc-900/32  border border-white/45 dark:border-white/12 shadow-[0_10px_30px_rgba(0,0,0,0.08)] hover:from-white/62 hover:to-white/42 dark:hover:from-zinc-900/60 dark:hover:to-zinc-900/42 hover:border-white/60 dark:hover:border-white/20 hover:shadow-[0_14px_36px_rgba(0,0,0,0.12)] rounded-3xl transition-all duration-300 ease-out transform hover:-translate-y-0.5",
  input: {
    inputWrapper:
      "bg-default-100/50 dark:bg-zinc-800/50 hover:bg-default-200/50 dark:hover:bg-zinc-700/50 group-data-[hover=true]:!border-primary-500 hover:!border-primary-500 focus-within:bg-default-100 dark:focus-within:bg-zinc-800 border-default-200/50 dark:border-white/10 focus-within:!border-primary-500 shadow-sm rounded-xl transition-all",
  },
  tabs: {
    tabList: "bg-default-100/50 dark:bg-zinc-800/50 rounded-xl px-1",
    cursor: "bg-primary-500 dark:bg-primary-900 hover:bg-primary-500 shadow-md",
    tabContent: "group-data-[selected=true]:text-white font-medium",
  },
};
