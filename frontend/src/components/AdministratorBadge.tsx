import { FaShieldAlt } from "react-icons/fa";
import { useTranslation } from "react-i18next";
import { useProcessElevation } from "@/hooks/useProcessElevation";

export function AdministratorBadge() {
  const { t } = useTranslation();
  const elevated = useProcessElevation();
  if (!elevated) return null;
  return (
    <span
      className="inline-flex shrink-0 items-center gap-1 whitespace-nowrap text-xs font-medium text-warning"
      title={t("app.administrator_description")}
    >
      <FaShieldAlt size={12} aria-hidden="true" />
      {t("app.administrator")}
    </span>
  );
}
