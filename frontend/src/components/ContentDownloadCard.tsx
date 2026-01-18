import React from "react";
import { Card, CardBody, CardHeader } from "@heroui/react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { SiCurseforge } from "react-icons/si";
import { FaCloudDownloadAlt, FaCube } from "react-icons/fa";
import { toast } from "react-hot-toast";

export const ContentDownloadCard = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();

  return (
    <Card className="h-full border-none shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl transition-all hover:bg-white/80 dark:hover:bg-zinc-900/80 group">
      <CardHeader className="px-5 py-3 border-b border-default-100 dark:border-white/5 flex justify-between items-center">
        <div className="flex items-center gap-2">
          <div className="p-1.5 rounded-lg bg-orange-500/10 text-orange-600 dark:text-orange-400">
            <FaCloudDownloadAlt size={16} />
          </div>
          <h3 className="text-base font-bold text-default-800 dark:text-zinc-100">
            {t("contentdownload.title", { defaultValue: "Content Download" })}
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
              {t("curseforge.title", { defaultValue: "CurseForge" })}
            </span>
          </div>
        </div>

        <div
          className="flex items-center justify-between p-3 rounded-xl hover:bg-default-200/50 dark:hover:bg-zinc-700/50 cursor-pointer transition-all border border-transparent hover:border-default-200/50 dark:hover:border-white/10"
          onClick={() => {
            toast(t("lip.maintenance"), {
              icon: "🚧",
            });
          }}
        >
          <div className="flex items-center gap-3">
            <FaCube className="text-emerald-500 text-xl" />
            <span className="font-medium text-default-700 dark:text-zinc-200">
              lip
            </span>
          </div>
        </div>
      </CardBody>
    </Card>
  );
};
