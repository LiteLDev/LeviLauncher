import React from "react";
import { useTranslation } from "react-i18next";
import { FiAlertTriangle, FiTrash2 } from "react-icons/fi";
import { AnimatePresence, motion } from "framer-motion";
import { UnifiedModal } from "./UnifiedModal";

export interface DeleteConfirmModalProps {
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  onConfirm: () => void | Promise<void>;
  title: string;
  description?: React.ReactNode;
  itemName?: string;
  isPending?: boolean;
  error?: string | null;
  warning?: string;
}

export const DeleteConfirmModal: React.FC<DeleteConfirmModalProps> = ({
  isOpen,
  onOpenChange,
  onConfirm,
  title,
  description,
  itemName,
  isPending = false,
  error,
  warning,
}) => {
  const { t } = useTranslation();

  return (
    <UnifiedModal
      isOpen={isOpen}
      onOpenChange={onOpenChange}
      type="error"
      title={title}
      icon={<FiTrash2 className="w-6 h-6 text-danger-500" />}
      isDismissable={!isPending}
      hideCloseButton={isPending}
      confirmText={t("common.delete")}
      cancelText={t("common.cancel")}
      onConfirm={async () => {
        await onConfirm();
        onOpenChange(false);
      }}
      onCancel={() => onOpenChange(false)}
      showCancelButton={!isPending}
      confirmButtonProps={{
        isLoading: isPending,
        className: "font-bold shadow-lg shadow-danger-500/20",
      }}
      cancelButtonProps={{
        isDisabled: isPending,
      }}
    >
      <div className="flex flex-col gap-3">
        {description && (
          <div className="text-medium text-default-700 dark:text-zinc-300 font-medium whitespace-pre-wrap">
            {description}
          </div>
        )}

        {itemName && (
          <div className="p-3 bg-default-100/50 dark:bg-zinc-800 rounded-xl border border-default-200/50">
            <span className="font-mono text-default-800 dark:text-zinc-200 font-bold break-all text-small">
              {itemName}
            </span>
          </div>
        )}

        {warning && (
          <div className="text-small text-danger-500 font-bold flex items-center gap-2">
            <FiAlertTriangle className="w-4 h-4 shrink-0" />
            {warning}
          </div>
        )}

        <AnimatePresence>
          {error && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: "auto" }}
              exit={{ opacity: 0, height: 0 }}
              className="text-small text-white bg-danger-500/90 px-3 py-2 rounded-lg"
            >
              {error}
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </UnifiedModal>
  );
};
