import { ButtonProps, Spinner, Modal } from "@heroui/react";
import { ModalAction, MODAL_STYLES } from "./ModalPrimitives";
import React from "react";

import { motion } from "framer-motion";
import { useTranslation } from "react-i18next";
import { cn } from "@/utils/cn";
import {
  FiCheckCircle,
  FiAlertTriangle,
  FiInfo,
  FiXCircle,
} from "react-icons/fi";
import {
  BaseModal,
  BaseModalHeader,
  BaseModalBody,
  BaseModalFooter,
  type BaseModalProps,
} from "./BaseModal";

export type ModalActionProps = Omit<ButtonProps, "className"> & {
  className?: string;
};

export type ModalType = "success" | "warning" | "error" | "info" | "primary";

export interface UnifiedModalProps
  extends Omit<BaseModalProps, "children" | "isOpen"> {
  isOpen: boolean;
  onOpenChange?: (open: boolean) => void;
  type?: ModalType;
  title: React.ReactNode;
  children?: React.ReactNode;
  footer?: React.ReactNode;
  icon?: React.ReactNode;
  onConfirm?: () => void;
  confirmText?: string;
  onCancel?: () => void;
  cancelText?: string;
  confirmButtonProps?: ModalActionProps;
  cancelButtonProps?: ModalActionProps;
  showCancelButton?: boolean;
  showConfirmButton?: boolean;
  hideScrollbar?: boolean;
  titleClass?: string;
  iconBgClass?: string;

  contentKey?: string | number;
}

const TYPE_CONFIG: Record<
  ModalType,
  {
    icon: React.ElementType;
    colorClass: string;
    bgClass: string;
    borderClass: string;
  }
> = {
  success: {
    icon: FiCheckCircle,
    colorClass: "text-brand-500",
    bgClass: "bg-brand-50 dark:bg-brand-500/10",
    borderClass: "border-brand-100 dark:border-brand-500/20",
  },
  warning: {
    icon: FiAlertTriangle,
    colorClass: "text-amber-500",
    bgClass: "bg-amber-50 dark:bg-amber-500/10",
    borderClass: "border-amber-100 dark:border-amber-500/20",
  },
  error: {
    icon: FiXCircle,
    colorClass: "text-rose-500",
    bgClass: "bg-rose-50 dark:bg-rose-500/10",
    borderClass: "border-rose-100 dark:border-rose-500/20",
  },
  info: {
    icon: FiInfo,
    colorClass: "text-brand-500",
    bgClass: "bg-brand-50 dark:bg-brand-500/10",
    borderClass: "border-brand-100 dark:border-brand-500/20",
  },
  primary: {
    icon: FiInfo,
    colorClass: "text-brand-500",
    bgClass: "bg-brand-50 dark:bg-brand-500/10",
    borderClass: "border-brand-100 dark:border-brand-500/20",
  },
};

const CONFIRM_BUTTON_CONFIG: Record<
  ModalType,
  Pick<ModalActionProps, "variant" | "className">
> = {
  success: {
    variant: "primary",
    className: "font-semibold",
  },
  warning: {
    variant: "primary",
    className: "font-semibold",
  },
  error: {
    variant: "primary",
    className: "font-semibold",
  },
  info: {
    variant: "primary",
    className: "font-semibold",
  },
  primary: {
    variant: "primary",
    className: "font-semibold",
  },
};

export const getUnifiedModalConfirmButtonProps = (
  type: ModalType,
  overrides?: Pick<ModalActionProps, "variant" | "className">,
): Pick<ModalActionProps, "variant" | "className"> => {
  const defaults = CONFIRM_BUTTON_CONFIG[type];

  return {
    variant: overrides?.variant ?? defaults.variant,
    className: overrides?.className
      ? cn(defaults.className, overrides.className)
      : defaults.className,
  };
};

export const UnifiedModal: React.FC<UnifiedModalProps> = ({
  isOpen,
  onOpenChange,
  type = "primary",
  title,
  children,
  footer,
  isDismissable = false,
  size,
  scrollBehavior = "inside",
  icon,
  onConfirm,
  confirmText,
  onCancel,
  cancelText,
  confirmButtonProps,
  cancelButtonProps,
  showCancelButton = false,
  showConfirmButton = true,
  hideScrollbar,
  titleClass,
  iconBgClass,
  className,
  containerClassName,
  backdropClassName,
  isKeyboardDismissDisabled,
  isPending = confirmButtonProps?.isPending ?? false,

  contentKey,
}) => {
  const { t } = useTranslation();
  const config = TYPE_CONFIG[type];
  const Icon = config.icon;
  const handleOpenChange = onOpenChange ?? (() => {});
  const resolvedConfirmText = confirmText ?? t("common.confirm");
  const resolvedCancelText = cancelText ?? t("common.cancel");
  const resolvedConfirmButtonProps = getUnifiedModalConfirmButtonProps(type, {
    variant: confirmButtonProps?.variant,
    className: confirmButtonProps?.className,
  });

  const resolvedIcon = React.useMemo(() => {
    if (!icon) {
      return <Icon className={cn("size-5", config.colorClass)} />;
    }

    if (!React.isValidElement<{ className?: string }>(icon)) {
      return icon;
    }

    return React.cloneElement(icon, {
      className: cn(icon.props.className, "size-5", config.colorClass),
    });
  }, [Icon, config.colorClass, icon]);

  return (
    <BaseModal
      isOpen={isOpen}
      onOpenChange={handleOpenChange}
      size={size}
      isDismissable={isDismissable}
      scrollBehavior={scrollBehavior}
      isPending={isPending}
      isKeyboardDismissDisabled={isKeyboardDismissDisabled}
      className={className}
      containerClassName={containerClassName}
      backdropClassName={backdropClassName}
    >
      {({ close: onClose }) => (
        <>
          <BaseModalHeader className="flex flex-row items-center gap-3">
            <motion.div
              key={type}
              initial={{ scale: 0 }}
              animate={{ scale: 1 }}
              transition={{
                type: "spring",
                stiffness: 260,
                damping: 20,
              }}
              className={`size-10 rounded-xl flex items-center justify-center shrink-0 border ${iconBgClass || `${config.bgClass} ${config.borderClass}`}`}
            >
              {resolvedIcon}
            </motion.div>
            <motion.div
              initial={{ opacity: 0, x: -10 }}
              animate={{ opacity: 1, x: 0 }}
              transition={{ duration: 0.25 }}
              className="flex min-w-0 flex-col [overflow-wrap:anywhere]"
            >
              <Modal.Heading
                className={`text-xl font-semibold leading-7 ${
                  titleClass || "text-foreground dark:text-zinc-100"
                }`}
              >
                {title}
              </Modal.Heading>
            </motion.div>
          </BaseModalHeader>
          <BaseModalBody className={hideScrollbar ? "no-scrollbar" : ""}>
            <motion.div
              key={contentKey ?? type}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.25, delay: 0.1 }}
              className={cn("w-full", MODAL_STYLES.stack)}
            >
              {children}
            </motion.div>
          </BaseModalBody>
          {(footer != null ||
            (showConfirmButton && onConfirm) ||
            showCancelButton) && (
            <BaseModalFooter>
              {footer != null ? (
                footer
              ) : (
                <>
                  {showCancelButton && (
                    <ModalAction
                      {...cancelButtonProps}
                      onPress={() => {
                        onCancel?.();
                        if (!onCancel) onClose();
                      }}
                      isDisabled={isPending || cancelButtonProps?.isDisabled}
                      variant={cancelButtonProps?.variant ?? "secondary"}
                    >
                      {resolvedCancelText}
                    </ModalAction>
                  )}
                  {showConfirmButton && onConfirm && (
                    <ModalAction
                      {...confirmButtonProps}
                      onPress={onConfirm}
                      isPending={isPending}
                      isDisabled={isPending || confirmButtonProps?.isDisabled}
                      variant={resolvedConfirmButtonProps.variant}
                      className={resolvedConfirmButtonProps.className}
                    >
                      {({ isPending }) => (
                        <>
                          {isPending && <Spinner size="sm" color="current" />}
                          {resolvedConfirmText}
                        </>
                      )}
                    </ModalAction>
                  )}
                </>
              )}
            </BaseModalFooter>
          )}
        </>
      )}
    </BaseModal>
  );
};
