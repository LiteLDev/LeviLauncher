# UX guidelines

Rules every page and dialog follows. Shared building blocks are listed at the end.

## Reach

- Every feature is at most two clicks from anywhere: one primary-nav click (or the command palette) plus one in-page action.
- A task adds at most one confirmation step. Merge "pick" and "confirm" dialogs into one dialog.
- Never send the user to another page to satisfy a prerequisite. Resolve it in place (for example, install LeviLamina in the same install dialog) and then continue what the user started.

## Continuity

- Going back restores the previous selection, scroll position and tab. Respect `returnTo` in route state.
- Each view has one primary action. Ask for confirmation of a destructive action once.
- A dialog that holds content (progress, results, logs) never closes on its own; the user closes it.
- Pages that work on the current instance show `InstancePicker` instead of redirecting when no instance is selected.

## Responsive

- Support every window from 640×480 up to 8K fullscreen. Check at 640×480 and 3840×2160 at minimum.
- The root font size is 14px for small windows, 16px normally, and scales with width from 2560 CSS px. Size everything in `rem` so it scales with it.
- Use `LAYOUT.PAGE.CONTAINER` (capped at `--page-max-width`, fluid `--page-gutter`), `LAYOUT.PAGE.READABLE` for prose and forms, and `LAYOUT.GRID.AUTO(minRem)` for card grids.
- Don't use fixed `min-h-[…px]` above 360px, or fixed pixel widths above 320px without `max-w-full`. The page must never scroll horizontally.
- Lists and tables become cards or stacked rows below about 768px of content width.

## Messages stay readable (append, don't replace)

- Use `notify()` from `@/utils/activity` instead of `toast()`. It records the notice in the Activity center and keeps the toast up for at least 6 seconds; errors stay until dismissed.
- Long-running work appends to a log instead of overwriting one status line. Use `startActivityTask` / `appendTaskLog` / `setTaskProgress` / `finishActivityTask`, and show the growing step list in the dialog.
- Never clear finished output automatically.

## Strings

New strings go in `src/assets/locales/fragments/<feature>/<locale>.json` for all 12 locales. They are deep-merged over the base locale files at load time. `en_US` and `zh_CN` must be translated; the others may fall back to English.

## Building blocks

| Need | Use |
| --- | --- |
| Notice with history | `notify({ title, description, variant, source })` |
| Task log | `startActivityTask`, `appendTaskLog`, `setTaskProgress`, `finishActivityTask`, `useActivityTask(id)` |
| Open the Activity center | `openActivityCenter()`; unread badge: `useUnreadActivityCount()` |
| Switch the current instance in place | `<InstancePicker />`; list only: `useInstanceList()` |
| Current instance state | `useCurrentVersion()` |
| Page layout | `LAYOUT.PAGE.CONTAINER`, `LAYOUT.PAGE.READABLE`, `LAYOUT.GRID.AUTO(minRem)` |
