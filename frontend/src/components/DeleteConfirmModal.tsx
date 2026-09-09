import React from "react";
import { useTranslation } from "react-i18next";
import { FiAlertTriangle, FiTrash2 } from "react-icons/fi";
import { AnimatePresence, motion } from "framer-motion";
import { UnifiedModal } from "./UnifiedModal";

export interface DeleteConfirmModalProps {
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  onConfirm: () => boolean | void | Promise<boolean | void>;
  title: string;
  description?: React.ReactNode;
  itemName?: string;
  itemNames?: string[];
  scopeLabel?: string;
  isPending?: boolean;
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
      onOpenChange={onOpenChange}
      type="error"
      title={title}
      icon={<FiTrash2 className="w-6 h-6" />}
      isDismissable={!isPending}
      hideCloseButton={isPending}
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
      showCancelButton={!isPending}
      confirmButtonProps={{
        isPending: isPending,
        isDisabled: isPending || confirmDisabled,
        className: "font-bold shadow-lg shadow-rose-500/20",
      }}
      cancelButtonProps={{
        isDisabled: isPending,
      }}
    >
      <div className="flex flex-col gap-3">
        {scopeLabel && <p className="text-sm font-medium text-muted">{scopeLabel}</p>}
        {description && (
          <div className="text-base text-foreground dark:text-zinc-300 font-medium whitespace-pre-wrap">
            {description}
          </div>
        )}

        {itemName && (
          <div className="p-3 bg-surface-secondary/50 dark:bg-zinc-800 rounded-xl border border-border/50">
            <span className="font-mono text-foreground dark:text-zinc-200 font-bold break-all text-sm">
              {itemName}
            </span>
          </div>
        )}
        {!!itemNames?.length && (
          <ul className="max-h-40 overflow-y-auto rounded-xl bg-surface-secondary/50 p-3 text-sm break-all">
            {itemNames.map((name, index) => <li key={`${index}-${name}`}>{name}</li>)}
          </ul>
        )}

        {warning && (
          <div className="text-sm text-rose-700 dark:text-rose-300 font-bold flex items-center gap-2">
            <FiAlertTriangle className="w-4 h-4 shrink-0" />
            {warning}
          </div>
        )}

        <AnimatePresence>
          {(error || confirmError) && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: "auto" }}
              exit={{ opacity: 0, height: 0 }}
              role="alert"
              className="text-sm text-rose-800 dark:text-rose-200 bg-rose-100 dark:bg-rose-950/50 px-3 py-2 rounded-lg whitespace-pre-wrap max-h-48 overflow-y-auto"
            >
              {error || confirmError}
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </UnifiedModal>
  );
};
