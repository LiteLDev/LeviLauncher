import { ModalDescription, ModalPanel, ModalNotice } from "@/components/ModalPrimitives";
import React from "react";
import { useTranslation } from "react-i18next";
import { FiTrash2 } from "react-icons/fi";
import { AnimatePresence, motion } from "framer-motion";
import { UnifiedModal, type UnifiedModalProps } from "./UnifiedModal";

export interface DeleteConfirmModalProps
  extends Pick<UnifiedModalProps, "isOpen" | "size" | "isPending"> {
  onOpenChange: NonNullable<UnifiedModalProps["onOpenChange"]>;
  onConfirm: () => boolean | void | Promise<boolean | void>;
  title: string;
  description?: React.ReactNode;
  itemName?: string;
  itemNames?: string[];
  scopeLabel?: string;
  confirmDisabled?: boolean;
  error?: string | null;
  warning?: string;
  confirmText?: string;
}

export const DeleteConfirmModal: React.FC<DeleteConfirmModalProps> = ({
  isOpen,
  onOpenChange,
  onConfirm,
  title,
  description,
  itemName,
  itemNames,
  scopeLabel,
  isPending = false,
  confirmDisabled = false,
  error,
  warning,
  confirmText,
  size,
}) => {
  const { t } = useTranslation();
  const [confirmError, setConfirmError] = React.useState("");
  const confirmationGeneration = React.useRef(0);
  React.useEffect(() => {
    confirmationGeneration.current++;
    if (isOpen) setConfirmError("");
  }, [isOpen, scopeLabel]);

  return (
    <UnifiedModal
      isOpen={isOpen}
      size={size}
      isPending={isPending}
      onOpenChange={onOpenChange}
      type="error"
      title={title}
      icon={<FiTrash2 className="w-6 h-6" />}
      isDismissable={!isPending}
      confirmText={confirmText || t("common.delete")}
      cancelText={t("common.cancel")}
      onConfirm={async () => {
        const generation = ++confirmationGeneration.current;
        setConfirmError("");
        try {
          const shouldClose = await onConfirm();
          if (generation !== confirmationGeneration.current) return;
          if (shouldClose === false) return;
          onOpenChange(false);
        } catch (cause) {
          if (generation !== confirmationGeneration.current) return;
          setConfirmError(cause instanceof Error ? cause.message : String(cause));
        }
      }}
      onCancel={() => onOpenChange(false)}
      showCancelButton
      confirmButtonProps={{
        isPending: isPending,
        isDisabled: isPending || confirmDisabled,
        variant: "danger",
      }}
      cancelButtonProps={{
        isDisabled: isPending,
      }}
    >
      <div className="flex flex-col gap-4">
        {scopeLabel && <ModalDescription>{scopeLabel}</ModalDescription>}
        {description && (
          <ModalDescription className="whitespace-pre-wrap">
            {description}
          </ModalDescription>
        )}

        {itemName && (
          <ModalPanel>
            <span className="font-mono text-foreground dark:text-zinc-200 font-medium [overflow-wrap:anywhere] text-sm">
              {itemName}
            </span>
          </ModalPanel>
        )}
        {!!itemNames?.length && (
          <ul className="max-h-40 overflow-y-auto rounded-xl border border-border/70 bg-surface-secondary/50 p-4 text-sm font-mono leading-6 [overflow-wrap:anywhere]">
            {itemNames.map((name, index) => <li key={`${index}-${name}`}>{name}</li>)}
          </ul>
        )}

        {warning && (
          <ModalNotice tone="danger">
            {warning}
          </ModalNotice>
        )}

        <AnimatePresence>
          {(error || confirmError) && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: "auto" }}
              exit={{ opacity: 0, height: 0 }}
              className="max-h-48 overflow-y-auto"
            >
              <ModalNotice role="alert" tone="danger">{error || confirmError}</ModalNotice>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </UnifiedModal>
  );
};
