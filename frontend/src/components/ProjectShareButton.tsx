import { useState } from "react";
import { Button, Tooltip, toast } from "@heroui/react";
import { Clipboard } from "@wailsio/runtime";
import { useTranslation } from "react-i18next";
import { LuShare2 } from "react-icons/lu";

export function ProjectShareButton({ url }: { url?: string }) {
  const { t } = useTranslation();
  const [copying, setCopying] = useState(false);
  // The catalog's public project URL is shareable outside this local app.
  if (!url || !/^https?:\/\//i.test(url)) return null;

  const copyLink = async () => {
    setCopying(true);
    try {
      await Clipboard.SetText(url);
      toast.success(t("audit.mods.link_copied"));
    } catch {
      toast.danger(t("audit.mods.copy_failed"));
    } finally {
      setCopying(false);
    }
  };

  return (
    <Tooltip>
      <Button
        isIconOnly
        aria-label={t("audit.mods.copy_project_link")}
        variant="secondary"
        isPending={copying}
        onPress={() => void copyLink()}
      >
        <LuShare2 size={20} />
      </Button>
      <Tooltip.Content>{t("audit.mods.copy_project_link")}</Tooltip.Content>
    </Tooltip>
  );
}
