import React from "react";
import { Button } from "@heroui/react";
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
      size="lg"
      isOpen={isOpen}
      onOpenChange={() => {}}
      type="info"
      title={t("clarity.prompt.title")}
      hideCloseButton
      isDismissable={false}
      showCancelButton={false}
      showConfirmButton={false}
      footer={
        <div className="flex w-full flex-wrap justify-end gap-2">
          <Button variant="light" onPress={onKeepDisabled}>
            {t("clarity.prompt.disable")}
          </Button>
          <Button color="primary" onPress={onEnable}>
            {t("clarity.prompt.enable")}
          </Button>
        </div>
      }
    >
      <div className="flex flex-col gap-4">
        <p className="text-default-700 dark:text-zinc-300 text-sm leading-6">
          {t("clarity.prompt.body")}
        </p>
        <div className="flex flex-wrap gap-2">
          <Button
            size="sm"
            radius="full"
            variant="flat"
            onPress={() => Browser.OpenURL("https://clarity.microsoft.com/terms")}
          >
            {t("clarity.prompt.clarity_terms")}
          </Button>
          <Button
            size="sm"
            radius="full"
            variant="light"
            onPress={() =>
              Browser.OpenURL("https://privacy.microsoft.com/privacystatement")
            }
          >
            {t("clarity.prompt.microsoft_privacy")}
          </Button>
        </div>
      </div>
    </UnifiedModal>
  );
};
