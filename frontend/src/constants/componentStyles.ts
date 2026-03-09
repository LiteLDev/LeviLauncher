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
    mainWrapper: "gap-1",
    inputWrapper:
      "bg-default-100/70 dark:bg-zinc-800/60 border-[1.5px] border-default-200/70 dark:border-white/10 hover:bg-default-100/85 dark:hover:bg-zinc-800/72 group-data-[hover=true]:bg-default-100/85 dark:group-data-[hover=true]:bg-zinc-800/72 hover:border-default-300/80 dark:hover:border-white/15 group-data-[hover=true]:border-default-300/80 dark:group-data-[hover=true]:border-white/15 focus-within:bg-default-50 dark:focus-within:bg-zinc-800 focus-within:!border-primary-500 group-data-[focus=true]:bg-default-50 dark:group-data-[focus=true]:bg-zinc-800 group-data-[focus=true]:!border-primary-500 shadow-sm rounded-xl transition-all duration-200 group-data-[invalid=true]:!border-danger-400 dark:group-data-[invalid=true]:!border-danger-500 group-data-[invalid=true]:bg-danger-50/35 dark:group-data-[invalid=true]:bg-danger-950/20 group-data-[disabled=true]:bg-default-100/55 dark:group-data-[disabled=true]:bg-zinc-900/45 group-data-[disabled=true]:shadow-none",
    innerWrapper: "gap-2",
    input:
      "text-default-800 dark:text-zinc-100 placeholder:text-default-500 dark:placeholder:text-zinc-400 [&::-webkit-calendar-picker-indicator]:cursor-pointer [&::-webkit-calendar-picker-indicator]:opacity-60 dark:[&::-webkit-calendar-picker-indicator]:invert transition-colors",
    label:
      "text-default-600 dark:text-zinc-300 font-medium transition-colors group-data-[invalid=true]:!text-danger-500 dark:group-data-[invalid=true]:!text-danger-400",
    clearButton:
      "text-default-400 hover:text-default-600 dark:text-zinc-500 dark:hover:text-zinc-300 transition-colors",
    helperWrapper: "px-1 pt-1",
    description: "text-tiny text-default-400 dark:text-zinc-500",
    errorMessage:
      "text-tiny font-medium text-danger-500 dark:text-danger-400",
  },
  tabs: {
    tabList: "bg-default-100/50 dark:bg-zinc-800/50 rounded-xl px-1",
    cursor: "bg-primary-500 dark:bg-primary-900 hover:bg-primary-500 shadow-md",
    tabContent: "group-data-[selected=true]:text-white font-medium",
  },
  table: {
    thead: "rounded-none after:hidden",
    th: "bg-default-100/55 dark:bg-zinc-900/55 text-default-500 dark:text-zinc-400 font-semibold border-b border-default-200/80 dark:border-white/10 h-12 first:rounded-s-none last:rounded-e-none",
  },
  tableSticky: {
    thead: "rounded-none after:hidden [&>tr]:first:!shadow-none",
    th: "bg-default-100/60 dark:bg-zinc-900/60 text-default-500 dark:text-zinc-400 font-semibold border-b border-default-200/80 dark:border-white/10 h-12 first:rounded-s-none last:rounded-e-none backdrop-blur-none",
  },
};
