export const COMPONENT_STYLES = {
  select: {
    trigger:
      "bg-default-100/50 dark:bg-zinc-800/50 data-[hover=true]:bg-default-200/50 dark:data-[hover=true]:bg-zinc-700/50 data-[focus=true]:border-primary-600 rounded-xl",
    popoverContent:
      "bg-white dark:bg-zinc-900 backdrop-blur-sm backdrop-saturate-150 border border-black/5 dark:border-white/10 shadow-lg rounded-xl transform-gpu",
  },
  dropdown: {
    content:
      "bg-white dark:bg-zinc-900 backdrop-blur-sm backdrop-saturate-150 border border-black/5 dark:border-white/10 shadow-lg rounded-xl transform-gpu",
  },
  dropdownTriggerButton:
    "bg-default-100/50 dark:bg-zinc-800/50 data-[hover=true]:bg-default-200/50 dark:data-[hover=true]:bg-zinc-700/50 data-[focus=true]:border-primary-600 rounded-xl",
  listItem:
    "bg-white/60 dark:bg-zinc-950/65 backdrop-blur-md border border-black/5 dark:border-white/10 shadow-sm hover:shadow-lg hover:bg-white/70 dark:hover:bg-zinc-900/70 rounded-3xl transition-all duration-300 ease-in-out transform hover:-translate-y-0.5",
  input: {
    inputWrapper:
      "bg-default-100/50 dark:bg-zinc-800/50 hover:bg-default-200/50 dark:hover:bg-zinc-700/50 group-data-[hover=true]:!border-primary-500 hover:!border-primary-500 focus-within:bg-default-100 dark:focus-within:bg-zinc-800 border-default-200/50 dark:border-white/10 focus-within:!border-primary-500 shadow-sm rounded-xl transition-all",
  },
  tabs: {
    tabList: "bg-default-100/50 dark:bg-zinc-800/50 rounded-xl px-1",
    cursor: "bg-primary-600 dark:bg-primary-900 hover:bg-primary-500 shadow-md",
    tabContent: "group-data-[selected=true]:text-white font-medium",
  },
};
