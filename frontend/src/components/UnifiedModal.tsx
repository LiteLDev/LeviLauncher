import React from "react";
import { ModalContent, Button, ButtonProps } from "@heroui/react";
import { motion } from "framer-motion";
import {
  FiCheckCircle,
  FiAlertTriangle,
  FiInfo,
  FiXCircle,
  FiHelpCircle,
} from "react-icons/fi";
import {
  BaseModal,
  BaseModalHeader,
  BaseModalBody,
  BaseModalFooter,
} from "./BaseModal";

export type ModalType = "success" | "warning" | "error" | "info" | "primary";

export interface UnifiedModalProps {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  type?: ModalType;
  title: React.ReactNode;
  children?: React.ReactNode;
  footer?: React.ReactNode;
  hideCloseButton?: boolean;
  isDismissable?: boolean;
  size?:
    | "xs"
    | "sm"
    | "md"
    | "lg"
    | "xl"
    | "2xl"
    | "3xl"
    | "4xl"
    | "5xl"
    | "full";
  scrollBehavior?: "inside" | "outside" | "normal";
  icon?: React.ReactNode;
  onConfirm?: () => void;
  confirmText?: string;
  onCancel?: () => void;
  cancelText?: string;
  confirmButtonProps?: ButtonProps;
  cancelButtonProps?: ButtonProps;
  showCancelButton?: boolean;
  showConfirmButton?: boolean;
  hideScrollbar?: boolean;
  titleClass?: string;
  iconBgClass?: string;
  classNames?: Record<string, string>;
  motionProps?: any;
  contentKey?: string | number;
}

const TYPE_CONFIG: Record<
  ModalType,
  {
    icon: React.ElementType;
    colorClass: string;
    bgClass: string;
    borderClass: string;
    buttonColor: ButtonProps["color"];
  }
> = {
  success: {
    icon: FiCheckCircle,
    colorClass: "text-success-500",
    bgClass: "bg-success-50 dark:bg-success-500/10",
    borderClass: "border-success-100 dark:border-success-500/20",
    buttonColor: "success",
  },
  warning: {
    icon: FiAlertTriangle,
    colorClass: "text-warning-500",
    bgClass: "bg-warning-50 dark:bg-warning-500/10",
    borderClass: "border-warning-100 dark:border-warning-500/20",
    buttonColor: "warning",
  },
  error: {
    icon: FiXCircle,
    colorClass: "text-danger-500",
    bgClass: "bg-danger-50 dark:bg-danger-500/10",
    borderClass: "border-danger-100 dark:border-danger-500/20",
    buttonColor: "danger",
  },
  info: {
    icon: FiInfo,
    colorClass: "text-primary-500",
    bgClass: "bg-primary-50 dark:bg-primary-500/10",
    borderClass: "border-primary-100 dark:border-primary-500/20",
    buttonColor: "primary",
  },
  primary: {
    icon: FiHelpCircle,
    colorClass: "text-primary-500",
    bgClass: "bg-primary-50 dark:bg-primary-500/10",
    borderClass: "border-primary-100 dark:border-primary-500/20",
    buttonColor: "primary",
  },
};

export const UnifiedModal: React.FC<UnifiedModalProps> = ({
  isOpen,
  onOpenChange,
  type = "primary",
  title,
  children,
  footer,
  isDismissable = false,
  size = "md",
  scrollBehavior = "inside",
  icon,
  onConfirm,
  confirmText = "Confirm",
  onCancel,
  cancelText = "Cancel",
  confirmButtonProps,
  cancelButtonProps,
  showCancelButton = false,
  showConfirmButton = true,
  hideScrollbar,
  titleClass,
  iconBgClass,
  classNames,
  motionProps,
  contentKey,
}) => {
  const config = TYPE_CONFIG[type];
  const Icon = config.icon;

  const confirmBtnColor =
    confirmButtonProps?.color || config.buttonColor;
  const confirmBtnClass =
    type === "success" && !confirmButtonProps?.className
      ? "bg-success-600 hover:bg-success-500 text-white font-bold shadow-lg shadow-success-900/20"
      : type === "warning" && !confirmButtonProps?.className
        ? "text-white! font-bold shadow-lg shadow-warning-500/20"
        : confirmButtonProps?.className || "font-bold shadow-lg";

  return (
    <BaseModal
      isOpen={isOpen}
      onOpenChange={onOpenChange}
      size={size}
      hideCloseButton={true}
      isDismissable={isDismissable}
      scrollBehavior={scrollBehavior}
      classNames={classNames}
      motionProps={motionProps}
    >
      <ModalContent className="shadow-none">
        {(onClose) => (
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
                className={`w-12 h-12 rounded-full flex items-center justify-center shrink-0 border ${iconBgClass || `${config.bgClass} ${config.borderClass}`}`}
              >
                {icon ? (
                  icon
                ) : (
                  <Icon className={`w-6 h-6 ${config.colorClass}`} />
                )}
              </motion.div>
              <motion.div
                initial={{ opacity: 0, x: -10 }}
                animate={{ opacity: 1, x: 0 }}
                transition={{ duration: 0.25 }}
                className="flex flex-col"
              >
                <h2
                  className={`text-xl font-bold ${titleClass || config.colorClass}`}
                >
                  {title}
                </h2>
              </motion.div>
            </BaseModalHeader>
            <BaseModalBody className={hideScrollbar ? "no-scrollbar" : ""}>
              <motion.div
                key={contentKey || type}
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.25, delay: 0.1 }}
                className="w-full"
              >
                {children}
              </motion.div>
            </BaseModalBody>
            {(footer || onConfirm || (showCancelButton && onCancel)) && (
              <BaseModalFooter>
                {footer ? (
                  footer
                ) : (
                  <>
                    {showCancelButton && (
                      <Button
                        variant="light"
                        radius="full"
                        onPress={() => {
                          onCancel?.();
                          if (!onCancel) onClose();
                        }}
                        {...cancelButtonProps}
                      >
                        {cancelText}
                      </Button>
                    )}
                    {showConfirmButton && (
                      <Button
                        color={confirmBtnColor}
                        radius="full"
                        className={confirmBtnClass}
                        onPress={onConfirm}
                        {...confirmButtonProps}
                      >
                        {confirmText}
                      </Button>
                    )}
                  </>
                )}
              </BaseModalFooter>
            )}
          </>
        )}
      </ModalContent>
    </BaseModal>
  );
};
