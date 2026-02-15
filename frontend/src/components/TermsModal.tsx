import React from "react";
import { Button } from "@heroui/react";
import { UnifiedModal } from "@/components/UnifiedModal";
import { useTranslation } from "react-i18next";
import { Window } from "@wailsio/runtime";

interface TermsModalProps {
  isOpen: boolean;
  countdown: number;
  onAccept: () => void;
}

export const TermsModal: React.FC<TermsModalProps> = ({
  isOpen,
  countdown,
  onAccept,
}) => {
  const { t } = useTranslation();

  return (
    <UnifiedModal
      size="lg"
      isOpen={isOpen}
      onOpenChange={() => {}}
      type="primary"
      title={t("terms.title")}
      hideCloseButton
      isDismissable={false}
      showConfirmButton={false}
      showCancelButton={false}
      footer={
        <div className="flex w-full justify-end gap-2">
          <Button
            variant="light"
            onPress={() => {
              Window.Close();
            }}
          >
            {t("terms.decline")}
          </Button>
          <Button
            color="primary"
            isDisabled={countdown > 0}
            onPress={onAccept}
          >
            {countdown > 0
              ? `${t("terms.agree")} (${countdown}s)`
              : t("terms.agree")}
          </Button>
        </div>
      }
    >
      <div className="text-[15px] sm:text-[16px] leading-7 text-default-900 dark:text-zinc-100 font-medium antialiased whitespace-pre-wrap wrap-break-word max-h-[56vh] overflow-y-auto pr-2 custom-scrollbar">
        {t("terms.body")}
      </div>
    </UnifiedModal>
  );
};
