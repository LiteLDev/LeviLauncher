import { Button, Card, Chip } from "@heroui/react";
import React, { useState } from "react";
import { useTranslation } from "react-i18next";

import { PageHeader, SectionHeader } from "@/components/PageHeader";
import {
  FaGithub,
  FaUsers,
  FaHeart,
  FaCode,
  FaPatreon,
  FaStar,
} from "react-icons/fa";
import { Browser } from "@wailsio/runtime";
import { motion, Variants } from "framer-motion";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { cn } from "@/utils/cn";

export default function AboutPage() {
  const { t } = useTranslation();
  const [isAnimating, setIsAnimating] = useState(true);

  const repoUrl = "https://github.com/LiteLDev/LeviLauncher";
  const orgUrl = "https://github.com/LiteLDev";

  const cardVariants: Variants = {
    hidden: { opacity: 0, y: 20 },
    visible: (i: number) => ({
      opacity: 1,
      y: 0,
      transition: {
        delay: i * 0.1,
        duration: 0.4,
        ease: "easeOut",
      },
    }),
  };

  return (
    <PageContainer
      className={cn("relative", isAnimating && "overflow-hidden")}
      animate={false}
    >
      <motion.div
        initial={{ opacity: 0, y: -20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4 }}
      >
        <Card className={LAYOUT.GLASS_CARD.BASE}>
          <Card.Content className="p-6">
            <PageHeader
              title={t("nav.about")}
              description={t("about.description")}
            />
          </Card.Content>
        </Card>
      </motion.div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Authors Section */}
        <motion.div
          custom={0}
          initial="hidden"
          animate="visible"
          variants={cardVariants}
        >
          <Card className={cn("h-full", LAYOUT.GLASS_CARD.BASE)}>
            <Card.Content className="p-6">
              <SectionHeader
                className="mb-4"
                icon={<FaUsers size={20} />}
                title={t("about.authors")}
              />
              <div className="flex items-center justify-between gap-3 p-4 rounded-2xl bg-surface/50 dark:bg-white/5 border border-border dark:border-white/5">
                <div className="flex items-center gap-3">
                  <img
                    src="https://avatars.githubusercontent.com/u/62042544?v=4"
                    alt="DreamGuXiang Avatar"
                    className="w-14 h-14 rounded-full border-2 border-white dark:border-zinc-700 shadow-sm"
                  />
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="text-foreground dark:text-zinc-100 font-bold text-lg">
                        DreamGuXiang
                      </span>
                      <Chip
                        size="sm"
                        variant="soft"
                        color={"accent"}
                        className={"h-5"}
                      >
                        <Chip.Label>{t("about.author")}</Chip.Label>
                      </Chip>
                    </div>
                    <div className="text-sm text-muted dark:text-zinc-400">
                      {t("about.ll_authors")}
                    </div>
                  </div>
                </div>
              </div>
              <div className="flex gap-2 mt-4">
                <Button
                  size="sm"
                  onPress={() =>
                    Browser.OpenURL("https://afdian.com/a/DreamGuXiang")
                  }
                  variant={"secondary"}
                  className={
                    "bg-surface-secondary dark:bg-white/10 text-foreground dark:text-zinc-300"
                  }
                >
                  {<FaHeart className="text-pink-500" />}
                  {t("about.afdian")}
                </Button>
                <Button
                  size="sm"
                  onPress={() =>
                    Browser.OpenURL("https://www.patreon.com/c/DreamGuXiang")
                  }
                  variant={"secondary"}
                  className={
                    "bg-surface-secondary dark:bg-white/10 text-foreground dark:text-zinc-300"
                  }
                >
                  {<FaPatreon className="text-orange-500" />}
                  {t("about.patreon")}
                </Button>
              </div>
            </Card.Content>
          </Card>
        </motion.div>

        {/* Special Thanks Section */}
        <motion.div
          custom={1}
          initial="hidden"
          animate="visible"
          variants={cardVariants}
        >
          <Card className={cn("h-full", LAYOUT.GLASS_CARD.BASE)}>
            <Card.Content className="p-6">
              <SectionHeader
                className="mb-4"
                icon={<FaStar size={20} />}
                iconWrapperClassName="bg-amber-500/10 text-amber-600 dark:text-amber-400"
                title={t("about.thanks")}
              />
              <p className="text-foreground dark:text-zinc-400 leading-relaxed mb-4">
                {t("about.thanks.desc")}
              </p>
              <div className="flex items-center justify-between gap-3 p-4 rounded-2xl bg-surface/50 dark:bg-white/5 border border-border dark:border-white/5">
                <div className="flex items-center gap-3">
                  <img
                    src="https://www.rhymc.com/assets/img/logo.png"
                    alt={t("about.rhymc_name")}
                    className="h-8 object-contain"
                  />
                  <span className="font-semibold text-foreground dark:text-zinc-200">
                    {t("about.rhymc_name")}
                  </span>
                </div>
                <Button
                  size="sm"
                  onPress={() => Browser.OpenURL("https://www.rhymc.com/")}
                  variant={"ghost"}
                >
                  {t("about.website")}
                </Button>
              </div>
            </Card.Content>
          </Card>
        </motion.div>

        {/* Sponsors Section */}
        <motion.div
          custom={2}
          initial="hidden"
          animate="visible"
          variants={cardVariants}
          className="lg:col-span-2"
        >
          <Card className={cn("h-full", LAYOUT.GLASS_CARD.BASE)}>
            <Card.Content className="p-6">
              <SectionHeader
                className="mb-4"
                icon={<FaHeart size={20} />}
                iconWrapperClassName="bg-rose-500/10 text-rose-600 dark:text-rose-400"
                title={t("about.sponsors")}
              />
              <p className="text-foreground dark:text-zinc-400 leading-relaxed">
                {t("about.sponsors.desc")}
              </p>
            </Card.Content>
          </Card>
        </motion.div>

        {/* Source Code Section */}
        <motion.div
          custom={3}
          initial="hidden"
          animate="visible"
          variants={cardVariants}
          className="lg:col-span-2"
          onAnimationComplete={() => setIsAnimating(false)}
        >
          <Card className={cn("h-full", LAYOUT.GLASS_CARD.BASE)}>
            <Card.Content className="p-6">
              <div className="flex flex-col md:flex-row md:items-start justify-between gap-4">
                <div className="flex-1">
                  <SectionHeader
                    className="mb-4"
                    icon={<FaCode size={20} />}
                    iconWrapperClassName="bg-blue-500/10 text-blue-600 dark:text-blue-400"
                    title={t("about.source")}
                  />
                  <div className="flex flex-wrap items-center gap-3 mb-4">
                    <Button
                      onPress={() => Browser.OpenURL(repoUrl)}
                      variant={"secondary"}
                      className={"bg-surface-secondary dark:bg-white/10"}
                    >
                      {<FaGithub className="text-lg" />}
                      {t("about.github_repo")}
                    </Button>
                    <Button
                      onPress={() => Browser.OpenURL(orgUrl)}
                      variant={"secondary"}
                      className={"bg-surface-secondary dark:bg-white/10"}
                    >
                      {<FaGithub className="text-lg" />}
                      {t("about.github_org")}
                    </Button>
                  </div>
                  <p className="text-sm text-muted dark:text-zinc-400">
                    {t("about.license.tip")}
                  </p>
                </div>

                <div className="flex-1 border-t md:border-t-0 md:border-l border-border dark:border-white/5 pt-6 md:pt-0 md:pl-6">
                  <SectionHeader
                    className="mb-4"
                    icon={<FaGithub size={20} />}
                    iconWrapperClassName="bg-muted/10 text-foreground dark:text-zinc-400"
                    title={t("about.contribute")}
                  />
                  <p className="text-foreground dark:text-zinc-400 leading-relaxed mb-4">
                    {t("about.contribute.desc")}
                  </p>
                  <div className="flex gap-2">
                    <Button
                      size="sm"
                      onPress={() => Browser.OpenURL(`${repoUrl}/issues`)}
                      variant={"secondary"}
                      className={"bg-surface-secondary dark:bg-white/10"}
                    >
                      {<FaGithub />}
                      {t("about.issue")}
                    </Button>
                    <Button
                      size="sm"
                      onPress={() => Browser.OpenURL(`${repoUrl}`)}
                      variant={"primary"}
                      className={
                        "bg-brand-500 brand-primary-foreground shadow-lg shadow-brand-900/20"
                      }
                    >
                      {<FaStar />}
                      {t("about.star_fork")}
                    </Button>
                  </div>
                </div>
              </div>
            </Card.Content>
          </Card>
        </motion.div>
      </div>
    </PageContainer>
  );
}
