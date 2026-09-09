import { ModalAction, ModalDescription } from "@/components/ModalPrimitives";
import { Button } from "@heroui/react";
import React from "react";

import { useTranslation } from "react-i18next";
import { Browser } from "@wailsio/runtime";
import { UnifiedModal } from "@/components/UnifiedModal";

interface ClarityConsentModalProps {
  isOpen: boolean;
  onEnable: () => void;
  onKeepDisabled: () => void;
}

export const ClarityConsentModal: React.FC<ClarityConsentModalProps> = ({
  isOpen,
  onEnable,
  onKeepDisabled,
}) => {
  const { t } = useTranslation();

  return (
    <UnifiedModal
      size="wide"
      isOpen={isOpen}
      type="info"
      title={t("clarity.prompt.title")}
      isDismissable={false}
      showCancelButton={false}
      showConfirmButton={false}
      footer={
        <div className="flex w-full flex-wrap justify-end gap-2">
          <ModalAction onPress={onKeepDisabled} variant="secondary">
            {t("clarity.prompt.disable")}
          </ModalAction>
          <ModalAction onPress={onEnable} variant={"primary"}>
            {t("clarity.prompt.enable")}
          </ModalAction>
        </div>
      }
    >
      <div className="flex flex-col gap-4">
        <ModalDescription>
          {t("clarity.prompt.body")}
        </ModalDescription>
        <div className="flex flex-wrap gap-2">
          <Button
            size="sm"
            onPress={() =>
              Browser.OpenURL("https://clarity.microsoft.com/terms")
            }
            variant={"secondary"}
            className={"rounded-full"}
          >
            {t("clarity.prompt.clarity_terms")}
          </Button>
          <Button
            size="sm"
            onPress={() =>
              Browser.OpenURL("https://privacy.microsoft.com/privacystatement")
            }
            variant={"ghost"}
            className={"rounded-full"}
          >
            {t("clarity.prompt.microsoft_privacy")}
          </Button>
        </div>
      </div>
    </UnifiedModal>
  );
};
