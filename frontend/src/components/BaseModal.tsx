import React from "react";
import {
  Modal,
  type ModalBackdropProps,
  type ModalDialogProps,
} from "@heroui/react";
import { cn } from "@/utils/cn";

const widths = {
  xs: "sm:max-w-xs",
  sm: "sm:max-w-sm",
  md: "sm:max-w-md",
  lg: "sm:max-w-lg",
  xl: "sm:max-w-xl",
  "2xl": "sm:max-w-2xl",
  "3xl": "sm:max-w-3xl",
  "4xl": "sm:max-w-4xl",
  "5xl": "sm:max-w-5xl",
  full: "max-w-none",
};
export interface BaseModalProps
  extends Pick<
    ModalBackdropProps,
    "isOpen" | "onOpenChange" | "isDismissable" | "isKeyboardDismissDisabled"
  > {
  children: ModalDialogProps["children"];
  size?: keyof typeof widths;
  scrollBehavior?: "inside" | "outside" | "normal";
  hideCloseButton?: boolean;
  className?: string;
  containerClassName?: string;
  backdropClassName?: string;
}
export const BaseModal = ({
  children,
  size = "md",
  scrollBehavior = "inside",
  hideCloseButton = true,
  className,
  containerClassName,
  backdropClassName,
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
      variant="blur"
      className={cn("z-[70]", backdropClassName)}
    >
      <Modal.Container
        size={size === "full" ? "full" : "lg"}
        scroll={scrollBehavior === "normal" ? "inside" : scrollBehavior}
        className={cn("z-[70]", widths[size], containerClassName)}
      >
        <Modal.Dialog
          className={cn(
            "bg-white dark:bg-zinc-950 border border-border shadow-2xl rounded-[2.5rem]",
            className,
          )}
        >
          {(state) => (
            <>
              {!hideCloseButton && (
                <Modal.CloseTrigger className="absolute right-5 top-5 z-50" />
              )}
              {typeof children === "function" ? children(state) : children}
            </>
          )}
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
    className={cn("flex flex-col gap-1 px-8 pt-6 pb-2", className)}
    {...props}
  />
);
export const BaseModalBody = ({
  className,
  ...props
}: React.ComponentProps<typeof Modal.Body>) => (
  <Modal.Body className={cn("px-8 py-4", className)} {...props} />
);
export const BaseModalFooter = ({
  className,
  ...props
}: React.ComponentProps<typeof Modal.Footer>) => (
  <Modal.Footer className={cn("px-8 pb-8 pt-4", className)} {...props} />
);
