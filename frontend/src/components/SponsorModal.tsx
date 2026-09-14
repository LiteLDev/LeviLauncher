import { useState } from "react";
import { toast } from "@heroui/react";
import { Browser } from "@wailsio/runtime";
import { useTranslation } from "react-i18next";
import { FaHeart, FaPatreon } from "react-icons/fa";
import { UnifiedModal } from "@/components/UnifiedModal";
import { ModalAction, ModalDescription } from "@/components/ModalPrimitives";
import { SPONSORSHIP_URLS } from "@/constants/sponsorship";

export function SponsorModal({
  launchCount,
  onDismiss,
}: {
  launchCount: number;
  onDismiss: () => void;
}) {
  const { t } = useTranslation();
  const [opening, setOpening] = useState(false);

  const openSponsor = async (url: string) => {
    setOpening(true);
    try {
      await Browser.OpenURL(url);
      onDismiss();
    } catch {
      toast.danger(t("sponsor_prompt.open_error"));
    } finally {
      setOpening(false);
    }
  };

  return (
    <UnifiedModal
      isOpen
      size="compact"
      title={t("sponsor_prompt.title")}
      icon={<FaHeart aria-hidden="true" />}
      isDismissable
      isPending={opening}
      onOpenChange={(open) => {
        if (!open) onDismiss();
      }}
      showConfirmButton={false}
      footer={
        <div className="flex w-full flex-wrap justify-end gap-2">
          <ModalAction
            variant="secondary"
            onPress={onDismiss}
            isDisabled={opening}
            autoFocus
          >
            {t("sponsor_prompt.dismiss")}
          </ModalAction>
          <ModalAction
            variant="secondary"
            onPress={() => void openSponsor(SPONSORSHIP_URLS.patreon)}
            isDisabled={opening}
          >
            <FaPatreon aria-hidden="true" />
            {t("about.patreon")}
          </ModalAction>
          <ModalAction
            variant="primary"
            onPress={() => void openSponsor(SPONSORSHIP_URLS.afdian)}
            isDisabled={opening}
          >
            <FaHeart aria-hidden="true" />
            {t("about.afdian")}
          </ModalAction>
        </div>
      }
    >
      <ModalDescription>
        {t("sponsor_prompt.body", { count: launchCount })}
      </ModalDescription>
    </UnifiedModal>
  );
}
