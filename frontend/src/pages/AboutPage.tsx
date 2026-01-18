import React from "react";
import { useTranslation } from "react-i18next";
import { Button, Card, CardBody, Chip } from "@heroui/react";
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
import { motion } from "framer-motion";

export default function AboutPage() {
  const { t } = useTranslation();

  const repoUrl = "https://github.com/LiteLDev/LeviLauncher";
  const orgUrl = "https://github.com/LiteLDev";

  const cardVariants = {
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
    <div className="relative w-full p-4 flex flex-col">
      <motion.div
        initial={{ opacity: 0, y: -20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4 }}
        className="mb-8"
      >
        <Card className="border-none shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl">
          <CardBody className="p-8">
            <PageHeader
              title={t("nav.about", { defaultValue: "About" })}
              description={t("about.description", {
                defaultValue:
                  "LeviLauncher - A Modern Minecraft Bedrock Launcher",
              })}
            />
          </CardBody>
        </Card>
      </motion.div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Authors Section */}
        <motion.div
          custom={0}
          initial="hidden"
          animate="visible"
          variants={cardVariants}
        >
          <Card className="h-full border-none shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl">
            <CardBody className="p-6">
              <SectionHeader
                className="mb-4"
                icon={<FaUsers size={20} />}
                title={t("about.authors", {
                  defaultValue: "Authors & Maintainers",
                })}
              />

              <div className="flex items-center justify-between gap-3 p-4 rounded-2xl bg-default-50/50 dark:bg-white/5 border border-default-100 dark:border-white/5">
                <div className="flex items-center gap-3">
                  <img
                    src="https://avatars.githubusercontent.com/u/62042544?v=4"
                    alt="DreamGuXiang Avatar"
                    className="w-14 h-14 rounded-full border-2 border-white dark:border-zinc-700 shadow-sm"
                  />
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="text-default-900 dark:text-zinc-100 font-bold text-lg">
                        DreamGuXiang
                      </span>
                      <Chip
                        size="sm"
                        variant="flat"
                        color="primary"
                        classNames={{ base: "h-5" }}
                      >
                        {t("about.author", { defaultValue: "Author" })}
                      </Chip>
                    </div>
                    <div className="text-small text-default-500">
                      {t("about.ll_authors", {
                        defaultValue: "LeviLauncher Author",
                      })}
                    </div>
                  </div>
                </div>
              </div>

              <div className="flex gap-2 mt-4">
                <Button
                  size="sm"
                  variant="flat"
                  className="bg-default-100 dark:bg-white/10 text-default-600 dark:text-zinc-300"
                  startContent={<FaHeart className="text-pink-500" />}
                  onPress={() =>
                    Browser.OpenURL("https://afdian.com/a/DreamGuXiang")
                  }
                >
                  {t("about.afdian", { defaultValue: "Afdian" })}
                </Button>
                <Button
                  size="sm"
                  variant="flat"
                  className="bg-default-100 dark:bg-white/10 text-default-600 dark:text-zinc-300"
                  startContent={<FaPatreon className="text-orange-500" />}
                  onPress={() =>
                    Browser.OpenURL("https://www.patreon.com/c/DreamGuXiang")
                  }
                >
                  {t("about.patreon", { defaultValue: "Patreon" })}
                </Button>
              </div>
            </CardBody>
          </Card>
        </motion.div>

        {/* Special Thanks Section */}
        <motion.div
          custom={1}
          initial="hidden"
          animate="visible"
          variants={cardVariants}
        >
          <Card className="h-full border-none shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl">
            <CardBody className="p-6">
              <SectionHeader
                className="mb-4"
                icon={<FaStar size={20} />}
                iconWrapperClassName="bg-warning-500/10 text-warning-600 dark:text-warning-400"
                title={t("about.thanks", { defaultValue: "Special Thanks" })}
              />
              <p className="text-default-600 dark:text-zinc-400 leading-relaxed mb-4">
                {t("about.thanks.desc", {
                  defaultValue:
                    "Special thanks to the individuals and projects that provided help and support for LeviLauncher.",
                })}
              </p>
              <div className="flex items-center justify-between gap-3 p-4 rounded-2xl bg-default-50/50 dark:bg-white/5 border border-default-100 dark:border-white/5">
                <div className="flex items-center gap-3">
                  <img
                    src="https://www.rhymc.com/assets/img/logo.png"
                    alt={t("about.rhymc_name", { defaultValue: "Rhymc" })}
                    className="h-8 object-contain"
                  />
                  <span className="font-semibold text-default-800 dark:text-zinc-200">
                    {t("about.rhymc_name", { defaultValue: "Rhymc" })}
                  </span>
                </div>
                <Button
                  size="sm"
                  variant="light"
                  onPress={() => Browser.OpenURL("https://www.rhymc.com/")}
                >
                  {t("about.website", { defaultValue: "Website" })}
                </Button>
              </div>
            </CardBody>
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
          <Card className="h-full border-none shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl">
            <CardBody className="p-6">
              <SectionHeader
                className="mb-4"
                icon={<FaHeart size={20} />}
                iconWrapperClassName="bg-danger-500/10 text-danger-600 dark:text-danger-400"
                title={t("about.sponsors", {
                  defaultValue: "Sponsors & Supporters",
                })}
              />
              <p className="text-default-600 dark:text-zinc-400 leading-relaxed">
                {t("about.sponsors.desc", {
                  defaultValue:
                    "Thanks to all sponsors and supporters who helped the project develop!",
                })}
              </p>
            </CardBody>
          </Card>
        </motion.div>

        {/* Source Code Section */}
        <motion.div
          custom={3}
          initial="hidden"
          animate="visible"
          variants={cardVariants}
          className="lg:col-span-2"
        >
          <Card className="h-full border-none shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl">
            <CardBody className="p-6">
              <div className="flex flex-col md:flex-row md:items-start justify-between gap-6">
                <div className="flex-1">
                  <SectionHeader
                    className="mb-4"
                    icon={<FaCode size={20} />}
                    iconWrapperClassName="bg-blue-500/10 text-blue-600 dark:text-blue-400"
                    title={t("about.source", {
                      defaultValue: "Source Code & Open Source",
                    })}
                  />
                  <div className="flex flex-wrap items-center gap-3 mb-4">
                    <Button
                      variant="flat"
                      className="bg-default-100 dark:bg-white/10"
                      startContent={<FaGithub className="text-lg" />}
                      onPress={() => Browser.OpenURL(repoUrl)}
                    >
                      {t("about.github_repo", {
                        defaultValue: "GitHub · LeviLauncher",
                      })}
                    </Button>
                    <Button
                      variant="flat"
                      className="bg-default-100 dark:bg-white/10"
                      startContent={<FaGithub className="text-lg" />}
                      onPress={() => Browser.OpenURL(orgUrl)}
                    >
                      {t("about.github_org", {
                        defaultValue: "LiteLDev Organization",
                      })}
                    </Button>
                  </div>
                  <p className="text-small text-default-500">
                    {t("about.license.tip", {
                      defaultValue:
                        "For license and details, please refer to the LICENSE file in the repository.",
                    })}
                  </p>
                </div>

                <div className="flex-1 border-t md:border-t-0 md:border-l border-default-100 dark:border-white/5 pt-6 md:pt-0 md:pl-6">
                  <SectionHeader
                    className="mb-4"
                    icon={<FaGithub size={20} />}
                    iconWrapperClassName="bg-default-500/10 text-default-600 dark:text-default-400"
                    title={t("about.contribute", {
                      defaultValue: "Contribute",
                    })}
                  />
                  <p className="text-default-600 dark:text-zinc-400 leading-relaxed mb-4">
                    {t("about.contribute.desc", {
                      defaultValue:
                        "Welcome to contribute to LeviLauncher by submitting Issues or Pull Requests.",
                    })}
                  </p>
                  <div className="flex gap-2">
                    <Button
                      size="sm"
                      variant="flat"
                      className="bg-default-100 dark:bg-white/10"
                      startContent={<FaGithub />}
                      onPress={() => Browser.OpenURL(`${repoUrl}/issues`)}
                    >
                      {t("about.issue", { defaultValue: "Issue" })}
                    </Button>
                    <Button
                      size="sm"
                      color="primary"
                      className="bg-linear-to-r from-emerald-500 to-teal-500 text-white shadow-lg shadow-emerald-900/20"
                      startContent={<FaStar />}
                      onPress={() => Browser.OpenURL(`${repoUrl}`)}
                    >
                      {t("about.star_fork", { defaultValue: "Star / Fork" })}
                    </Button>
                  </div>
                </div>
              </div>
            </CardBody>
          </Card>
        </motion.div>
      </div>
    </div>
  );
}
