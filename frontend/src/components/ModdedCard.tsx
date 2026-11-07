import { Button, Card, Chip, ScrollShadow } from "@heroui/react";
import React, { useEffect } from "react";

import * as types from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import { FaPuzzlePiece, FaArrowRight } from "react-icons/fa";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { motion, AnimatePresence } from "framer-motion";
import { LAYOUT } from "@/constants/layout";
import { ROUTES } from "@/constants/routes";
import { cn } from "@/utils/cn";
import { useModIntelligence } from "@/utils/ModIntelligenceContext";
import { resolveModFolder } from "@/utils/modIntelligenceResolver";

export const ModCard = (args: {
  localVersionMap: Map<string, any>;
  currentVersion: string;
}) => {
  const { t } = useTranslation();
  const { ensureInstanceHydrated, getInstanceSnapshot, snapshotRevision } =
    useModIntelligence();
  const currentVersionName = String(args.currentVersion || "").trim();
  const snapshot = React.useMemo(
    () => (currentVersionName ? getInstanceSnapshot(currentVersionName) : null),
    [currentVersionName, getInstanceSnapshot, snapshotRevision],
  );
  const modsInfo = React.useMemo<Array<types.ModInfo>>(() => {
    if (!snapshot) return [];
    const enabledMap =
      snapshot.enabledByFolder instanceof Map
        ? snapshot.enabledByFolder
        : new Map<string, boolean>();
    return (snapshot.modsInfo || []).filter((mod) =>
      Boolean(enabledMap.get(resolveModFolder(mod))),
    );
  }, [snapshot]);
  const navigate = useNavigate();

  useEffect(() => {
    if (!currentVersionName) return;
    void ensureInstanceHydrated(currentVersionName, {
      background: true,
      reason: "launcher-modded-card",
    });
  }, [args.localVersionMap, currentVersionName, ensureInstanceHydrated]);

  return (
    <Card
      className={cn(
        "h-full transition-all group",
        LAYOUT.GLASS_CARD.BASE,
        "rounded-3xl",
      )}
    >
      <Card.Header className="px-5 py-3 border-b border-border dark:border-white/5 flex justify-between items-center gap-2">
        <div className="flex min-w-0 items-center gap-2">
          <div className="shrink-0 p-1.5 rounded-lg bg-indigo-500/10 text-indigo-600 dark:text-indigo-400">
            <FaPuzzlePiece size={16} />
          </div>
          <h3 className="truncate text-base font-bold text-foreground dark:text-zinc-100">
            {t("moddedcard.title")}
          </h3>
        </div>
        <div className="flex shrink-0 items-center gap-1">
          {modsInfo.length > 0 && (
            <Chip size="sm" variant="soft" color={"accent"} className={"h-6"}>
              <Chip.Label>{modsInfo.length}</Chip.Label>
            </Chip>
          )}
          <Button
            size="sm"
            onPress={() => navigate(ROUTES.mods)}
            isDisabled={!args.currentVersion}
            variant={"ghost"}
            className={
              "text-xs text-foreground dark:text-zinc-300 data-[hovered]:text-foreground dark:data-[hovered]:text-zinc-200"
            }
          >
            {t("moddedcard.manage")}
            {<FaArrowRight size={10} />}
          </Button>
        </div>
      </Card.Header>
      <Card.Content className="p-0 overflow-hidden relative">
        {/* Height follows the window so short windows keep the launch area in view. */}
        <ScrollShadow className="h-[clamp(7rem,22vh,18rem)] w-full p-4 overflow-y-auto [&::-webkit-scrollbar]:hidden [-ms-overflow-style:'none'] [scrollbar-width:'none']">
          {modsInfo.length > 0 ? (
            <div className="flex flex-col gap-2">
              <AnimatePresence>
                {modsInfo.map((mod, idx) => (
                  <motion.div
                    key={`${mod.name}-${idx}`}
                    initial={{ opacity: 0, x: -10 }}
                    animate={{ opacity: 1, x: 0 }}
                    transition={{ delay: idx * 0.05 }}
                    className="flex items-center justify-between p-2 rounded-xl hover:bg-surface-tertiary/50 transition-colors"
                  >
                    <div className="flex flex-col min-w-0 w-full">
                      <span className="text-sm font-semibold truncate text-foreground dark:text-zinc-200">
                        {mod.name}
                      </span>
                    </div>
                  </motion.div>
                ))}
              </AnimatePresence>
            </div>
          ) : (
            <div className="h-full flex flex-col items-center justify-center text-foreground dark:text-zinc-300 gap-2">
              <FaPuzzlePiece size={32} className="opacity-20" />
              <span className="text-sm">{t("moddedcard.content.none")}</span>
            </div>
          )}
        </ScrollShadow>
      </Card.Content>
    </Card>
  );
};
