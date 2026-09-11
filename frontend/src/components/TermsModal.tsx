import { ModalAction, ModalDescription } from "@/components/ModalPrimitives";
import React from "react";

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
      size="wide"
      isOpen={isOpen}
      type="primary"
      title={t("terms.title")}
      isDismissable={false}
      showConfirmButton={false}
      showCancelButton={false}
      footer={
        <div className="flex w-full justify-end gap-2">
          <ModalAction
            onPress={() => {
              Window.Close();
            }}
            variant="secondary"
          >
            {t("terms.decline")}
          </ModalAction>
          <ModalAction
            isDisabled={countdown > 0}
            onPress={onAccept}
            variant={"primary"}
          >
            {countdown > 0
              ? `${t("terms.agree")} (${countdown}s)`
              : t("terms.agree")}
          </ModalAction>
        </div>
      }
    >
      <ModalDescription className="whitespace-pre-wrap">
        {t("terms.body")}
      </ModalDescription>
    </UnifiedModal>
  );
};
