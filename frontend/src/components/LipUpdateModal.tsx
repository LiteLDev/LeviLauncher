import React from "react";
import { Button } from "@heroui/react";
import { UnifiedModal } from "@/components/UnifiedModal";
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

  return (
    <UnifiedModal
      size="md"
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) {
          onDismiss();
        }
      }}
      type="warning"
      title={t("settings.lip.startup_prompt.title")}
      icon={<FaDownload className="w-5 h-5 text-warning-500" />}
      hideCloseButton
      showConfirmButton={false}
      showCancelButton={false}
      footer={
        <div className="flex w-full flex-wrap justify-end gap-2">
          <Button variant="light" onPress={onDismiss}>
            {t("settings.lip.startup_prompt.later_button")}
          </Button>
          <Button variant="flat" onPress={onIgnore}>
            {t("settings.lip.startup_prompt.ignore_button")}
          </Button>
          <Button color="primary" onPress={onOpenSettings}>
            {t("settings.lip.startup_prompt.open_settings_button")}
          </Button>
        </div>
      }
    >
      <div className="flex flex-col gap-3">
        <p className="text-small text-default-600 dark:text-zinc-300 leading-6">
          {t("settings.lip.startup_prompt.description", {
            currentVersion,
            latestVersion,
          })}
        </p>
        <div className="rounded-2xl border border-default-200 dark:border-zinc-700 bg-default-100/60 dark:bg-zinc-800/60 px-4 py-3 text-small text-default-600 dark:text-zinc-300">
          <div>{t("settings.lip.startup_prompt.current_version", { currentVersion })}</div>
          <div>{t("settings.lip.startup_prompt.latest_version", { latestVersion })}</div>
        </div>
      </div>
    </UnifiedModal>
  );
};
