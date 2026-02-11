import React from "react";
import { useTranslation } from "react-i18next";
import { FiAlertTriangle } from "react-icons/fi";
import { resolveImportError } from "@/utils/importError";
import { UnifiedModal, ModalType } from "./UnifiedModal";

export interface ImportResultModalProps {
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  results: {
    success: string[];
    failed: Array<{ name: string; err: string }>;
  };
  onConfirm?: () => void;
}

export const ImportResultModal: React.FC<ImportResultModalProps> = ({
  isOpen,
  onOpenChange,
  results,
  onConfirm,
}) => {
  const { t } = useTranslation();
  const { success, failed } = results;
  const hasFailed = failed.length > 0;
  const hasSuccess = success.length > 0;
  const isPartial = hasSuccess && hasFailed;

  // Determine modal type and title
  let type: ModalType = "success";
  let title = t("mods.summary_title_done");

  if (isPartial) {
    type = "warning";
    title = t("mods.summary_title_partial");
  } else if (hasFailed) {
    type = "error"; // Changed from warning to error as requested
    title = t("mods.summary_failed");
  }

  return (
    <UnifiedModal
      isOpen={isOpen}
      onOpenChange={onOpenChange}
      type={type}
      title={title}
      hideCloseButton
      showCancelButton={false}
      confirmText={t("common.confirm")}
      onConfirm={() => {
        onConfirm?.();
        onOpenChange(false);
      }}
    >
      <div className="flex flex-col gap-4">
        {success.length > 0 && (
          <div className="flex flex-col gap-2">
            <div className="text-medium text-default-700 dark:text-zinc-300 font-bold flex items-center gap-2">
              {t("mods.summary_success")} ({success.length})
            </div>
            <div className="p-3 bg-default-100/50 dark:bg-zinc-800 rounded-xl border border-default-200/50 max-h-[150px] overflow-y-auto custom-scrollbar">
              <div className="text-small font-bold font-mono text-default-800 dark:text-zinc-200 whitespace-pre-wrap break-all">
                {success.join("\n")}
              </div>
            </div>
          </div>
        )}

        {failed.length > 0 && (
          <div className="flex flex-col gap-2">
            <div className="text-small font-bold text-danger-600 dark:text-danger-500 flex items-center gap-2">
              <FiAlertTriangle className="w-4 h-4" />
              {t("mods.summary_failed")} ({failed.length})
            </div>
            <div className="p-3 bg-danger-50/50 dark:bg-danger-500/10 rounded-xl border border-danger-100 dark:border-danger-500/20 max-h-[150px] overflow-y-auto custom-scrollbar">
              <div className="text-xs font-mono text-danger-700 dark:text-danger-400 whitespace-pre-wrap break-all flex flex-col gap-1">
                {failed.map((it, idx) => (
                  <div key={idx} className="flex gap-2">
                    <span className="font-bold shrink-0">{it.name}:</span>
                    <span>{resolveImportError(it.err, t)}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}
      </div>
    </UnifiedModal>
  );
};
