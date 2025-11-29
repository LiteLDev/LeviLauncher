import React, { useEffect } from "react";
import { Tooltip, Card, CardBody, Button } from "@heroui/react";
import { GetMods } from "../../bindings/github.com/liteldev/LeviLauncher/minecraft";
import * as types from "../../bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import { FaCheckCircle } from "react-icons/fa";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { LuFolderPlus } from "react-icons/lu";

export const ModCard = (args: {
  localVersionMap: Map<string, any>;
  currentVersion: string;
}) => {
  const { t, i18n } = useTranslation();
  const [modsInfo, setModsInfo] = React.useState<Array<types.ModInfo>>([]);
  const [isModded, setIsModded] = React.useState<boolean>(false);
  useEffect(() => {
    const name = String(args.currentVersion || "");
    if (name) {
      GetMods(name)
        .then((data) => setModsInfo(data || []))
        .catch(() => setModsInfo([]));
    } else {
      setModsInfo([]);
    }

    setIsModded(
      Boolean(
        args.localVersionMap.get(args.currentVersion)?.isPreLoader?.valueOf()
      ) || false
    );
  }, [args.currentVersion, args.localVersionMap]);

  const getNameContent = (version: string) => {
    if (!modsInfo) return "no found";
    return modsInfo.map((mod, index) => (
      <>
        {mod.name} {mod.version}
        {index < modsInfo.length - 1 && <br key={index} />}
      </>
    ));
  };

  const navigate = useNavigate();

  return (
    <Card className="rounded-2xl shadow-md h-full min-h-[160px] bg-white/70 dark:bg-black/30 backdrop-blur-md border border-white/30">
      <CardBody className="relative p-4 sm:p-5 flex flex-col gap-3 text-left">
        <div className="flex items-center gap-2">
          <span className="font-bold text-lg">{t("moddedcard.title")}</span>
        </div>
        <div className="flex items-center text-base font-semibold">
          <FaCheckCircle className="text-green-500 mr-2" />
          {modsInfo && modsInfo.length > 0 ? (
            <Tooltip
              showArrow
              content={getNameContent(args.currentVersion)}
              placement="bottom"
              color="primary"
            >
              {t("moddedcard.content.found", { count: modsInfo.length })}
            </Tooltip>
          ) : (
            <span>{t("moddedcard.content.none")}</span>
          )}
        </div>
        <div className="absolute bottom-3 right-3">
          <Tooltip
            content={
              t("moddedcard.manage", {
                defaultValue: "管理模组",
              }) as unknown as string
            }
            placement="left"
          >
            <Button
              isIconOnly
              size="sm"
              variant="light"
              radius="full"
              onPress={() => navigate("/mods")}
              isDisabled={!args.currentVersion}
              aria-label={
                (args.currentVersion
                  ? t("moddedcard.manage", { defaultValue: "管理模组" })
                  : t("launcherpage.currentVersion_choose_version_aria", {
                      defaultValue: "选择版本",
                    })) as unknown as string
              }
            >
              <LuFolderPlus size={20} />
            </Button>
          </Tooltip>
        </div>
      </CardBody>
    </Card>
  );
};
