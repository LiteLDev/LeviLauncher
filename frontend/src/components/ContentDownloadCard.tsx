import React from "react";
import { Card, CardBody } from "@heroui/react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { SiCurseforge } from "react-icons/si";

export const ContentDownloadCard = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();

  return (
    <Card className="rounded-2xl shadow-md h-full min-h-[160px] bg-white/70 dark:bg-black/30 backdrop-blur-md border border-white/30">
      <CardBody className="relative p-4 sm:p-5 flex flex-col gap-3 text-left">
        <div className="flex items-center gap-2">
          <span className="font-bold text-lg">{t("contentdownload.title")}</span>
        </div>

        <div className="grid grid-cols-1 gap-2 mt-auto">
          <div 
            className="flex items-center justify-between p-3 rounded-xl border border-white/30 bg-default-100/70 dark:bg-default-50/10 hover:bg-default-200 dark:hover:bg-default-50/20 cursor-pointer transition-all"
            onClick={() => navigate("/curseforge")}
          >
            <div className="flex items-center gap-3">
              <SiCurseforge className="text-[#f16436] text-xl" />
              <span className="font-medium">{t("curseforge.title")}</span>
            </div>
          </div>
        </div>
      </CardBody>
    </Card>
  );
};
