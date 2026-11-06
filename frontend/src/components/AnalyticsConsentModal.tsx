import { ModalAction, ModalDescription } from "@/components/ModalPrimitives";
import { Button } from "@heroui/react";
import React from "react";

import { useTranslation } from "react-i18next";
import { Browser } from "@wailsio/runtime";
import { UnifiedModal } from "@/components/UnifiedModal";

interface AnalyticsConsentModalProps {
  isOpen: boolean;
  onEnable: () => void;
  onKeepDisabled: () => void;
}

export const AnalyticsConsentModal: React.FC<AnalyticsConsentModalProps> = ({
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
      title={t("analytics.prompt.title")}
      isDismissable={false}
      showCancelButton={false}
      showConfirmButton={false}
      footer={
        <div className="flex w-full flex-wrap justify-end gap-2">
          <ModalAction onPress={onKeepDisabled} variant="secondary">
            {t("analytics.prompt.disable")}
          </ModalAction>
          <ModalAction onPress={onEnable} variant={"primary"}>
            {t("analytics.prompt.enable")}
          </ModalAction>
        </div>
      }
    >
      <div className="flex flex-col gap-4">
        <ModalDescription>
          {t("analytics.prompt.body")}
        </ModalDescription>
        <div className="flex flex-wrap gap-2">
          <Button
            size="sm"
            onPress={() =>
              Browser.OpenURL(
                "https://marketingplatform.google.com/about/analytics/terms/us/",
              )
            }
            variant={"secondary"}
            className={"rounded-full"}
          >
            {t("analytics.prompt.google_analytics_terms")}
          </Button>
          <Button
            size="sm"
            onPress={() =>
              Browser.OpenURL("https://policies.google.com/privacy")
            }
            variant={"ghost"}
            className={"rounded-full"}
          >
            {t("analytics.prompt.google_privacy")}
          </Button>
        </div>
      </div>
    </UnifiedModal>
  );
};
