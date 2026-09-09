import { ModalNotice, ModalPanel } from "@/components/ModalPrimitives";
import React from "react";
import { useTranslation } from "react-i18next";
import { FiAlertTriangle, FiCheckCircle } from "react-icons/fi";
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
  titleDone?: string;
  titlePartial?: string;
  titleFailed?: string;
  successLabel?: string;
  failedLabel?: string;
}

export const ImportResultModal: React.FC<ImportResultModalProps> = ({
  isOpen,
  onOpenChange,
  results,
  onConfirm,
  titleDone,
  titlePartial,
  titleFailed,
  successLabel,
  failedLabel,
}) => {
  const { t } = useTranslation();
  const { success, failed } = results;
  const hasFailed = failed.length > 0;
  const hasSuccess = success.length > 0;
  const isPartial = hasSuccess && hasFailed;

  const resolvedDoneTitle = titleDone || t("mods.summary_title_done");
  const resolvedPartialTitle = titlePartial || t("mods.summary_title_partial");
  const resolvedFailedTitle = titleFailed || t("mods.summary_failed");
  const resolvedSuccessLabel = successLabel || t("mods.summary_success");
  const resolvedFailedLabel = failedLabel || t("mods.summary_failed");

  let type: ModalType = "success";
  let title = resolvedDoneTitle;

  if (isPartial) {
    type = "warning";
    title = resolvedPartialTitle;
  } else if (hasFailed) {
    type = "error";
    title = resolvedFailedTitle;
  }

  return (
    <UnifiedModal
      isOpen={isOpen}
      size={isPartial ? "wide" : "standard"}
      onOpenChange={onOpenChange}
      type={type}
      title={title}
      showCancelButton={false}
      confirmText={t("common.confirm")}
      onConfirm={() => {
        onConfirm?.();
        onOpenChange(false);
      }}
    >
      <div className={isPartial ? "grid grid-cols-1 gap-4 sm:grid-cols-2" : "flex flex-col gap-4"}>
        {success.length > 0 && (
          <div className="flex flex-col gap-2">
            <div className="text-sm text-foreground font-semibold flex items-center gap-2">
              <FiCheckCircle aria-hidden="true" className="size-4 text-brand-600 dark:text-brand-400" />
              {resolvedSuccessLabel} ({success.length})
            </div>
            <ModalPanel className="max-h-60 overflow-y-auto custom-scrollbar">
              <div className="text-sm font-mono text-foreground whitespace-pre-wrap [overflow-wrap:anywhere]">
                {success.join("\n")}
              </div>
            </ModalPanel>
          </div>
        )}

        {failed.length > 0 && (
          <div className="flex flex-col gap-2">
            <div className="text-sm font-semibold text-foreground flex items-center gap-2">
              <FiAlertTriangle aria-hidden="true" className="size-4 text-rose-600 dark:text-rose-300" />
              {resolvedFailedLabel} ({failed.length})
            </div>
            <ModalNotice tone="danger" className="max-h-60 overflow-y-auto">
              <div className="whitespace-pre-wrap [overflow-wrap:anywhere] flex flex-col gap-3">
                {failed.map((it, idx) => (
                  <div key={idx} className="flex min-w-0 flex-col gap-1">
                    <span className="font-mono font-medium">{it.name}</span>
                    <span>{resolveImportError(it.err, t)}</span>
                  </div>
                ))}
              </div>
            </ModalNotice>
          </div>
        )}
      </div>
    </UnifiedModal>
  );
};
