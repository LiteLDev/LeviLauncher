import React from "react";
import {
  Modal,
  type ModalBackdropProps,
  type ModalDialogProps,
} from "@heroui/react";
import { cn } from "@/utils/cn";

// Apply widths to Dialog, not the viewport-sized positioning Container.
// Widths are in rem so they grow with the fluid root font size on 4K/8K, and
// never exceed the window minus a 1rem margin on each side.
export const MODAL_WIDTHS = {
  compact: "max-w-[min(35rem,calc(100vw-2rem))]",
  standard: "max-w-[min(42.5rem,calc(100vw-2rem))]",
  wide: "max-w-[min(55rem,calc(100vw-2rem))]",
  detail: "max-w-[min(70rem,calc(100vw-2rem))]",
  full: "max-w-none",
} as const;
export type ModalSize = keyof typeof MODAL_WIDTHS;

// Below 640px wide or 520px tall a centered card would clip its content, so
// the dialog becomes a full-window sheet. Tailwind needs literal class names,
// hence the two spelled-out variants.
const SHEET_CONTAINER =
  "max-sm:p-0 [@media(max-height:519.98px)]:p-0";
const SHEET_DIALOG =
  "max-sm:h-full max-sm:max-h-full max-sm:max-w-none max-sm:rounded-none max-sm:border-0 max-sm:shadow-none " +
  "[@media(max-height:519.98px)]:h-full [@media(max-height:519.98px)]:max-h-full [@media(max-height:519.98px)]:max-w-none " +
  "[@media(max-height:519.98px)]:rounded-none [@media(max-height:519.98px)]:border-0 [@media(max-height:519.98px)]:shadow-none";

export interface BaseModalProps
  extends Pick<
    ModalBackdropProps,
    "isOpen" | "onOpenChange" | "isDismissable" | "isKeyboardDismissDisabled"
  > {
  children: ModalDialogProps["children"];
  size?: ModalSize;
  scrollBehavior?: "inside" | "outside";
  isPending?: boolean;
  className?: string;
  containerClassName?: string;
  backdropClassName?: string;
}
export const BaseModal = ({
  children,
  size = "standard",
  scrollBehavior = "inside",
  className,
  containerClassName,
  backdropClassName,
  isDismissable = false,
  isKeyboardDismissDisabled = false,
  isPending = false,
  ...backdropProps
}: BaseModalProps) => (
  <>
    {backdropProps.isOpen && (
      // Keeps the window draggable by its title bar while a dialog is open.
      // Sheets cover the whole window, so the strip would sit on their header.
      <div
        aria-hidden="true"
        className="wails-draggable fixed inset-x-0 top-0 h-14 z-[75] max-sm:hidden [@media(max-height:519.98px)]:hidden"
      />
    )}
    <Modal.Backdrop
      {...backdropProps}
      onOpenChange={(open) => {
        if (!isPending) backdropProps.onOpenChange?.(open);
      }}
      isDismissable={isDismissable && !isPending}
      isKeyboardDismissDisabled={isKeyboardDismissDisabled || isPending}
      variant="blur"
      className={cn("z-[70]", backdropClassName)}
    >
      <Modal.Container
        size={size === "full" ? "full" : "lg"}
        placement="center"
        scroll={scrollBehavior}
        className={cn(
          "z-[70] w-full sm:w-full min-w-0",
          // Top padding clears the draggable title-bar strip above.
          size !== "full" && "px-4 pb-4 pt-14 sm:px-4 sm:pb-4 sm:pt-14",
          size !== "full" && SHEET_CONTAINER,
          containerClassName,
        )}
      >
        {/* With scroll="inside" HeroUI caps the dialog at the container height,
            so header and footer stay pinned and only the body scrolls. */}
        <Modal.Dialog
          className={cn(
            "w-full min-w-0 gap-0 p-0 bg-overlay border border-white/40 dark:border-zinc-700/50 shadow-2xl rounded-3xl",
            MODAL_WIDTHS[size],
            size === "full" ? "rounded-none" : SHEET_DIALOG,
            className,
          )}
        >
          {children}
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  </>
);

// Paddings shrink on small windows and follow rem on large ones.
export const BaseModalHeader = ({
  className,
  ...props
}: React.ComponentProps<typeof Modal.Header>) => (
  <Modal.Header
    className={cn(
      "flex min-w-0 shrink-0 flex-col gap-1 px-[clamp(1rem,2.5vw,1.5rem)] py-[clamp(0.75rem,2.5vh,1.25rem)]",
      className,
    )}
    {...props}
  />
);
export const BaseModalBody = ({
  className,
  ...props
}: React.ComponentProps<typeof Modal.Body>) => (
  <Modal.Body
    className={cn(
      "m-0! min-w-0 px-[clamp(1rem,2.5vw,1.5rem)] pt-0 pb-[clamp(0.75rem,3vh,1.5rem)] text-sm font-normal leading-6 text-foreground/80 dark:text-zinc-300 [overflow-wrap:anywhere]",
      className,
    )}
    {...props}
  />
);
// Buttons wrap instead of overflowing; the primary action is rendered last so
// it stays at the right end (or the bottom when wrapped).
export const BaseModalFooter = ({
  className,
  ...props
}: React.ComponentProps<typeof Modal.Footer>) => (
  <Modal.Footer
    className={cn(
      "m-0! min-w-0 shrink-0 flex-wrap gap-2 px-[clamp(1rem,2.5vw,1.5rem)] py-[clamp(0.625rem,2vh,1rem)]",
      className,
    )}
    {...props}
  />
);
