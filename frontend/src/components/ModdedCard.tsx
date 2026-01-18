import React, { useEffect } from "react";
import {
  Card,
  CardBody,
  Button,
  ScrollShadow,
  Chip,
  CardHeader,
} from "@heroui/react";
import {
  GetMods,
  IsModEnabled,
} from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import * as types from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import { FaPuzzlePiece, FaArrowRight } from "react-icons/fa";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { motion, AnimatePresence } from "framer-motion";

export const ModCard = (args: {
  localVersionMap: Map<string, any>;
  currentVersion: string;
}) => {
  const { t } = useTranslation();
  const [modsInfo, setModsInfo] = React.useState<Array<types.ModInfo>>([]);
  const navigate = useNavigate();

  useEffect(() => {
    const name = String(args.currentVersion || "");
    if (name) {
      (async () => {
        try {
          const data = (await GetMods(name)) || [];
          const statusList = await Promise.all(
            data.map(async (m) => {
              try {
                return await (IsModEnabled as any)?.(name, m.name);
              } catch {
                return false;
              }
            }),
          );
          const filtered = data.filter((_, i) => statusList[i]);
          setModsInfo(filtered);
        } catch {
          setModsInfo([]);
        }
      })();
    } else {
      setModsInfo([]);
    }
  }, [args.currentVersion, args.localVersionMap]);

  return (
    <Card className="h-full border-none shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl transition-all hover:bg-white/80 dark:hover:bg-zinc-900/80 group">
      <CardHeader className="px-5 py-3 border-b border-default-100 dark:border-white/5 flex justify-between items-center">
        <div className="flex items-center gap-2">
          <div className="p-1.5 rounded-lg bg-indigo-500/10 text-indigo-600 dark:text-indigo-400">
            <FaPuzzlePiece size={16} />
          </div>
          <h3 className="text-base font-bold text-default-800 dark:text-zinc-100">
            {t("moddedcard.title", { defaultValue: "Mods" })}
          </h3>
        </div>
        {modsInfo.length > 0 && (
          <Chip size="sm" variant="flat" color="primary" className="h-6">
            {modsInfo.length}
          </Chip>
        )}
      </CardHeader>

      <CardBody className="p-0 overflow-hidden relative">
        <ScrollShadow className="h-[140px] w-full p-4 overflow-y-auto [&::-webkit-scrollbar]:hidden [-ms-overflow-style:'none'] [scrollbar-width:'none']">
          {modsInfo.length > 0 ? (
            <div className="flex flex-col gap-2">
              <AnimatePresence>
                {modsInfo.map((mod, idx) => (
                  <motion.div
                    key={`${mod.name}-${idx}`}
                    initial={{ opacity: 0, x: -10 }}
                    animate={{ opacity: 1, x: 0 }}
                    transition={{ delay: idx * 0.05 }}
                    className="flex items-center justify-between p-2 rounded-xl hover:bg-default-200/50 dark:hover:bg-zinc-700/50 transition-colors"
                  >
                    <div className="flex flex-col min-w-0 w-full">
                      <span className="text-sm font-semibold truncate text-default-700 dark:text-zinc-200">
                        {mod.name}
                      </span>
                    </div>
                  </motion.div>
                ))}
              </AnimatePresence>
            </div>
          ) : (
            <div className="h-full flex flex-col items-center justify-center text-default-400 gap-2">
              <FaPuzzlePiece size={32} className="opacity-20" />
              <span className="text-sm">
                {t("moddedcard.content.none", {
                  defaultValue: "No mods found",
                })}
              </span>
            </div>
          )}
        </ScrollShadow>

        {/* Action Overlay / Footer */}
        <div className="absolute bottom-0 left-0 right-0 p-3 bg-linear-to-t from-white/90 via-white/50 to-transparent dark:from-zinc-900/90 dark:via-zinc-900/50 pt-6 flex justify-end opacity-0 group-hover:opacity-100 transition-opacity duration-200">
          <Button
            size="sm"
            color="primary"
            variant="flat"
            endContent={<FaArrowRight />}
            onPress={() => navigate("/mods")}
            isDisabled={!args.currentVersion}
            className="font-semibold shadow-sm"
          >
            {t("moddedcard.manage", { defaultValue: "Manage Mods" })}
          </Button>
        </div>
      </CardBody>
    </Card>
  );
};
