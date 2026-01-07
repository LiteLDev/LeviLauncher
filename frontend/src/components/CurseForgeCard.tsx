import React from "react";
import { Tooltip, Card, CardBody, Button } from "@heroui/react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { LuDownload } from "react-icons/lu";

export const CurseForgeCard = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();

  return (
    <Card className="rounded-2xl shadow-md h-full min-h-[160px] bg-white/70 dark:bg-black/30 backdrop-blur-md border border-white/30">
      <CardBody className="relative p-4 sm:p-5 flex flex-col gap-3 text-left">
        <div className="flex items-center gap-2">
          <span className="font-bold text-lg">{t("curseforgecard.title")}</span>
        </div>
        <div className="flex items-center text-base font-semibold">
          <LuDownload className="text-blue-500 mr-2" />
          <span>{t("curseforgecard.content")}</span>
        </div>
        <div className="absolute bottom-3 right-3">
          <Tooltip
            content={
              t("curseforgecard.browse", {
                defaultValue: "浏览CurseForge",
              }) as unknown as string
            }
            placement="left"
          >
            <Button
              isIconOnly
              size="sm"
              variant="light"
              radius="full"
              onPress={() => navigate("/curseforge")}
              aria-label={
                t("curseforgecard.browse", {
                  defaultValue: "浏览CurseForge",
                }) as unknown as string
              }
            >
              <LuDownload size={20} />
            </Button>
          </Tooltip>
        </div>
      </CardBody>
    </Card>
  );
};
