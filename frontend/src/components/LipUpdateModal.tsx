import { ModalAction, ModalDescription, ModalDetails } from "@/components/ModalPrimitives";
import React from "react";

import {
  UnifiedModal,
  getUnifiedModalConfirmButtonProps,
} from "@/components/UnifiedModal";
import { useTranslation } from "react-i18next";
import { FaDownload } from "react-icons/fa";

interface LipUpdateModalProps {
  isOpen: boolean;
  currentVersion: string;
  latestVersion: string;
  onDismiss: () => void;
  onIgnore: () => void;
  onOpenSettings: () => void;
}

export const LipUpdateModal: React.FC<LipUpdateModalProps> = ({
  isOpen,
  currentVersion,
  latestVersion,
  onDismiss,
  onIgnore,
  onOpenSettings,
}) => {
  const { t } = useTranslation();
  const warningConfirmButtonProps =
    getUnifiedModalConfirmButtonProps("warning");

  return (
    <UnifiedModal
      size="standard"
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) {
          onDismiss();
        }
      }}
      type="warning"
      title={t("settings.lip.startup_prompt.title")}
      icon={<FaDownload className="w-5 h-5" />}
      showConfirmButton={false}
      showCancelButton={false}
      footer={
        <div className="flex w-full flex-wrap justify-end gap-2">
          <ModalAction onPress={onDismiss} variant="secondary">
            {t("settings.lip.startup_prompt.later_button")}
          </ModalAction>
          <ModalAction onPress={onIgnore} variant={"secondary"}>
            {t("settings.lip.startup_prompt.ignore_button")}
          </ModalAction>
          <ModalAction
            {...warningConfirmButtonProps}
            onPress={onOpenSettings}
            variant="primary"
          >
            {t("settings.lip.startup_prompt.open_settings_button")}
          </ModalAction>
        </div>
      }
    >
      <div className="flex flex-col gap-3">
        <ModalDescription>
          {t("settings.lip.startup_prompt.description", {
            currentVersion,
            latestVersion,
          })}
        </ModalDescription>
        <ModalDetails items={[
          { label: t("settings.lip.startup_prompt.current_version", { currentVersion: "" }).replace(/[:：]\s*$/, ""), value: currentVersion, mono: true },
          { label: t("settings.lip.startup_prompt.latest_version", { latestVersion: "" }).replace(/[:：]\s*$/, ""), value: latestVersion, mono: true },
        ]} />
      </div>
    </UnifiedModal>
  );
};
