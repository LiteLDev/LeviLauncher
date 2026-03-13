import React from "react";
import { ModalContent, Button, ButtonProps } from "@heroui/react";
import { motion } from "framer-motion";
import { useTranslation } from "react-i18next";
import { cn } from "@/utils/cn";
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
  }
> = {
  success: {
    icon: FiCheckCircle,
    colorClass: "text-primary-500",
    bgClass: "bg-primary-50 dark:bg-primary-500/10",
    borderClass: "border-primary-100 dark:border-primary-500/20",
  },
  warning: {
    icon: FiAlertTriangle,
    colorClass: "text-warning-500",
    bgClass: "bg-warning-50 dark:bg-warning-500/10",
    borderClass: "border-warning-100 dark:border-warning-500/20",
  },
  error: {
    icon: FiXCircle,
    colorClass: "text-danger-500",
    bgClass: "bg-danger-50 dark:bg-danger-500/10",
    borderClass: "border-danger-100 dark:border-danger-500/20",
  },
  info: {
    icon: FiInfo,
    colorClass: "text-primary-500",
    bgClass: "bg-primary-50 dark:bg-primary-500/10",
    borderClass: "border-primary-100 dark:border-primary-500/20",
  },
  primary: {
    icon: FiHelpCircle,
    colorClass: "text-primary-500",
    bgClass: "bg-primary-50 dark:bg-primary-500/10",
    borderClass: "border-primary-100 dark:border-primary-500/20",
  },
};

const CONFIRM_BUTTON_CONFIG: Record<
  ModalType,
  Pick<ButtonProps, "color" | "className">
> = {
  success: {
    color: "primary",
    className: "font-bold shadow-lg",
  },
  warning: {
    color: "warning",
    className: "text-white! font-bold shadow-lg shadow-warning-500/20",
  },
  error: {
    color: "danger",
    className: "font-bold shadow-lg shadow-danger-500/20",
  },
  info: {
    color: "primary",
    className: "font-bold shadow-lg",
  },
  primary: {
    color: "primary",
    className: "font-bold shadow-lg",
  },
};

export const getUnifiedModalConfirmButtonProps = (
  type: ModalType,
  overrides?: Pick<ButtonProps, "color" | "className">,
): Pick<ButtonProps, "color" | "className"> => {
  const defaults = CONFIRM_BUTTON_CONFIG[type];

  return {
    color: overrides?.color ?? defaults.color,
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
  size = "md",
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
  classNames,
  motionProps,
  contentKey,
}) => {
  const { t } = useTranslation();
  const config = TYPE_CONFIG[type];
  const Icon = config.icon;
  const resolvedConfirmText = confirmText ?? t("common.confirm");
  const resolvedCancelText = cancelText ?? t("common.cancel");
  const resolvedConfirmButtonProps = getUnifiedModalConfirmButtonProps(type, {
    color: confirmButtonProps?.color,
    className: confirmButtonProps?.className,
  });

  const resolvedIcon = React.useMemo(() => {
    if (!icon) {
      return <Icon className={cn("w-6 h-6", config.colorClass)} />;
    }

    if (!React.isValidElement<{ className?: string }>(icon)) {
      return icon;
    }

    return React.cloneElement(icon, {
      className: cn(icon.props.className, config.colorClass),
    });
  }, [Icon, config.colorClass, icon]);

  return (
    <BaseModal
      isOpen={isOpen}
      onOpenChange={onOpenChange}
      size={size}
      hideCloseButton={true}
      isDismissable={isDismissable}
      scrollBehavior={scrollBehavior}
      classNames={{
        base: "bg-white/80! dark:bg-zinc-900/80! backdrop-blur-2xl border-white/40! dark:border-zinc-700/50! shadow-2xl rounded-4xl",
        ...classNames,
      }}
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
                {resolvedIcon}
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
                        {resolvedCancelText}
                      </Button>
                    )}
                    {showConfirmButton && (
                      <Button
                        {...confirmButtonProps}
                        color={resolvedConfirmButtonProps.color}
                        radius="full"
                        className={resolvedConfirmButtonProps.className}
                        onPress={onConfirm}
                      >
                        {resolvedConfirmText}
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
