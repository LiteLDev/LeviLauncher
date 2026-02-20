import React from "react";
import { Card, CardBody, CardHeader, addToast } from "@heroui/react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { SiCurseforge } from "react-icons/si";
import { FaCloudDownloadAlt, FaCube } from "react-icons/fa";
import { LAYOUT } from "@/constants/layout";
import { cn } from "@/utils/cn";

export const ContentDownloadCard = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();

  return (
    <Card
      className={cn(
        "h-full transition-all group",
        LAYOUT.GLASS_CARD.BASE,
        "rounded-3xl",
      )}
    >
      <CardHeader className="px-5 py-3 border-b border-default-100 dark:border-white/5 flex justify-between items-center">
        <div className="flex items-center gap-2">
          <div className="p-1.5 rounded-lg bg-orange-500/10 text-orange-600 dark:text-orange-400">
            <FaCloudDownloadAlt size={16} />
          </div>
          <h3 className="text-base font-bold text-default-800 dark:text-zinc-100">
            {t("contentdownload.title")}
          </h3>
        </div>
      </CardHeader>

      <CardBody className="p-4 flex flex-col gap-3">
        <div
          className="flex items-center justify-between p-3 rounded-xl hover:bg-default-200/50 dark:hover:bg-zinc-700/50 cursor-pointer transition-all border border-transparent hover:border-default-200/50 dark:hover:border-white/10"
          onClick={() => navigate("/curseforge")}
        >
          <div className="flex items-center gap-3">
            <SiCurseforge className="text-[#f16436] text-xl" />
            <span className="font-medium text-default-700 dark:text-zinc-200">
              {t("curseforge.title")}
            </span>
          </div>
        </div>

        <div
          className="flex items-center justify-between p-3 rounded-xl hover:bg-default-200/50 dark:hover:bg-zinc-700/50 cursor-pointer transition-all border border-transparent hover:border-default-200/50 dark:hover:border-white/10"
          onClick={() => {
            // navigate("/lip");
            addToast({
              description: t("lip.maintenance"),
              icon: "🚧",
            });
          }}
        >
          <div className="flex items-center gap-3">
            <FaCube className="text-green-500 text-xl" />
            <span className="font-medium text-default-700 dark:text-zinc-200">
              lip
            </span>
          </div>
        </div>
      </CardBody>
    </Card>
  );
};
