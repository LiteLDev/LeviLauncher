import React from "react";
import {
  Modal,
  type ModalBackdropProps,
  type ModalDialogProps,
} from "@heroui/react";
import { cn } from "@/utils/cn";

// Apply widths to Dialog, not the viewport-sized positioning Container.
export const MODAL_WIDTHS = {
  compact: "max-w-[560px]",
  standard: "max-w-[680px]",
  wide: "max-w-[880px]",
  detail: "max-w-[1120px]",
  full: "max-w-none",
} as const;
export type ModalSize = keyof typeof MODAL_WIDTHS;
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
      <div
        aria-hidden="true"
        className="wails-draggable fixed inset-x-0 top-0 h-14 z-[75]"
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
        className={cn("z-[70] w-full sm:w-full min-w-0", containerClassName)}
      >
        <Modal.Dialog
          className={cn(
            "w-full min-w-0 gap-0 p-0 bg-overlay border border-white/40 dark:border-zinc-700/50 shadow-2xl rounded-3xl",
            MODAL_WIDTHS[size],
            size === "full" && "rounded-none",
            className,
          )}
        >
          {children}
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  </>
);
export const BaseModalHeader = ({
  className,
  ...props
}: React.ComponentProps<typeof Modal.Header>) => (
  <Modal.Header
    className={cn(
      "flex min-w-0 shrink-0 flex-col gap-1 px-6 py-5",
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
      "m-0! min-w-0 px-6 pt-0 pb-6 text-sm font-normal leading-6 text-foreground/80 dark:text-zinc-300 [overflow-wrap:anywhere]",
      className,
    )}
    {...props}
  />
);
export const BaseModalFooter = ({
  className,
  ...props
}: React.ComponentProps<typeof Modal.Footer>) => (
  <Modal.Footer
    className={cn(
      "m-0! min-w-0 shrink-0 flex-wrap gap-2 px-6 py-4",
      className,
    )}
    {...props}
  />
);
