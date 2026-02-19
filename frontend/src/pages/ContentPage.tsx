import React from "react";
import { useTranslation } from "react-i18next";
import {
  Button,
  Card,
  CardBody,
  Dropdown,
  DropdownItem,
  DropdownMenu,
  DropdownTrigger,
  Spinner,
  Tooltip,
  Progress,
} from "@heroui/react";
import { Dialogs } from "@wailsio/runtime";
import { UnifiedModal } from "@/components/UnifiedModal";
import { ImportResultModal } from "@/components/ImportResultModal";
import { AnimatePresence, motion } from "framer-motion";
import {
  FaCogs,
  FaFolderOpen,
  FaGlobe,
  FaImage,
  FaSync,
  FaUserTag,
  FaServer,
  FaCamera,
} from "react-icons/fa";
import { resolvePlayerDisplayName } from "@/utils/content";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { FiUploadCloud } from "react-icons/fi";
import { PageHeader } from "@/components/PageHeader";
import { PageContainer } from "@/components/PageContainer";
import { FileDropOverlay } from "@/components/FileDropOverlay";
import { useFileDrag } from "@/hooks/useFileDrag";
import { useContentPage } from "@/hooks/useContentPage";
import { LAYOUT } from "@/constants/layout";
import { cn } from "@/utils/cn";
import { COMPONENT_STYLES } from "@/constants/componentStyles";

export default function ContentPage() {
  const { t } = useTranslation();
  const scrollRef = React.useRef<HTMLDivElement>(null);
  const isDragActive = useFileDrag(scrollRef as React.RefObject<HTMLElement>);
  const cp = useContentPage(t as any);

  return (
    <PageContainer
      ref={scrollRef}
      id="content-drop-zone"
      {...({ "data-file-drop-target": true } as any)}
      className="relative"
      animate={false}
    >
      {/* Drag Overlay */}
      <FileDropOverlay
        isDragActive={isDragActive}
        text={t("contentpage.drop_hint")}
      />

      {/* Header Card */}
      <motion.div
        initial={{ opacity: 0, y: -20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4 }}
      >
        <Card className={LAYOUT.GLASS_CARD.BASE}>
          <CardBody className="p-6">
            <div className="flex flex-col gap-6">
              <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
                <div>
                  <div className="flex items-center gap-3">
                    <PageHeader
                      title={t("launcherpage.content_manage")}
                      titleClassName="pb-1"
                    />
                  </div>
                  <div className="mt-2 text-default-500 dark:text-zinc-400 text-sm flex flex-wrap items-center gap-2">
                    <span>{t("contentpage.current_version")}:</span>
                    <span className="font-medium text-default-700 dark:text-zinc-200 bg-default-100 dark:bg-zinc-800 px-2 py-0.5 rounded-md">
                      {cp.currentVersionName || t("contentpage.none")}
                    </span>
                    <span className="text-default-300 dark:text-zinc-600">
                      |
                    </span>
                    <span>{t("contentpage.isolation")}:</span>
                    <span
                      className={`font-medium px-2 py-0.5 rounded-md ${
                        cp.roots.isIsolation
                          ? "bg-success-50 text-success-600 dark:bg-success-900/20 dark:text-success-400"
                          : "bg-default-100 dark:bg-zinc-800 text-default-700 dark:text-zinc-200"
                      }`}
                    >
                      {cp.roots.isIsolation ? t("common.yes") : t("common.no")}
                    </span>
                    <span className="text-default-300 dark:text-zinc-600">
                      |
                    </span>
                    <span>{t("contentpage.select_player")}:</span>
                    <Dropdown classNames={COMPONENT_STYLES.dropdown}>
                      <DropdownTrigger>
                        <Button
                          size="sm"
                          className={cn(
                            COMPONENT_STYLES.dropdownTriggerButton,
                            "h-6 min-w-0 px-2 text-small font-medium text-default-700 dark:text-zinc-200",
                          )}
                        >
                          {cp.selectedPlayer
                            ? resolvePlayerDisplayName(
                                cp.selectedPlayer,
                                cp.playerGamertagMap,
                              )
                            : t("contentpage.no_players")}
                        </Button>
                      </DropdownTrigger>
                      <DropdownMenu
                        aria-label="Players"
                        selectionMode="single"
                        selectedKeys={new Set([cp.selectedPlayer])}
                        onSelectionChange={(keys) => {
                          const arr = Array.from(
                            keys as unknown as Set<string>,
                          );
                          const next = arr[0] || "";
                          if (typeof next === "string") cp.onChangePlayer(next);
                        }}
                      >
                        {cp.players.length ? (
                          cp.players.map((p) => (
                            <DropdownItem
                              key={p}
                              textValue={resolvePlayerDisplayName(
                                p,
                                cp.playerGamertagMap,
                              )}
                            >
                              {resolvePlayerDisplayName(p, cp.playerGamertagMap)}
                            </DropdownItem>
                          ))
                        ) : (
                          <DropdownItem key="none" isDisabled>
                            {t("contentpage.no_players")}
                          </DropdownItem>
                        )}
                      </DropdownMenu>
                    </Dropdown>
                    {!cp.selectedPlayer && (
                      <span className="text-danger-500 text-xs">
                        ({t("contentpage.require_player_for_world_import")})
                      </span>
                    )}
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <Button
                    radius="full"
                    className="bg-primary-600 text-white font-medium shadow-sm"
                    startContent={<FiUploadCloud />}
                    onPress={async () => {
                      try {
                        const paths = await Dialogs.OpenFile({
                          Title: t("contentpage.import_button"),
                          Filters: [
                            {
                              DisplayName: "Content Files",
                              Pattern: "*.mcworld;*.mcpack;*.mcaddon",
                            },
                          ],
                          AllowsMultipleSelection: true,
                        });
                        if (paths && Array.isArray(paths) && paths.length > 0) {
                          cp.doImportFromPaths(paths);
                        }
                      } catch (e) {
                        console.error(e);
                      }
                    }}
                    isDisabled={cp.importing}
                  >
                    {t("contentpage.import_button")}
                  </Button>
                  <Tooltip
                    content={
                      t("contentpage.open_users_dir") as unknown as string
                    }
                  >
                    <Button
                      radius="full"
                      variant="flat"
                      startContent={<FaFolderOpen />}
                      onPress={() => {
                        if (cp.roots.usersRoot) {
                          (minecraft as any)?.OpenPathDir(cp.roots.usersRoot);
                        }
                      }}
                      isDisabled={!cp.hasBackend || !cp.roots.usersRoot}
                      className="bg-default-100 dark:bg-zinc-800 text-default-600 dark:text-zinc-200 font-medium"
                    >
                      {t("common.open")}
                    </Button>
                  </Tooltip>
                  <Tooltip content={t("common.refresh") as unknown as string}>
                    <Button
                      isIconOnly
                      radius="full"
                      variant="light"
                      onPress={() => cp.refreshAll()}
                      isDisabled={cp.loading}
                    >
                      <FaSync
                        className={cp.loading ? "animate-spin" : ""}
                        size={18}
                      />
                    </Button>
                  </Tooltip>
                </div>
              </div>
              {!!cp.error && (
                <div className="text-danger-500 text-sm">{cp.error}</div>
              )}
            </div>
          </CardBody>
        </Card>
      </motion.div>

      {/* Content Grid */}
      <motion.div
        className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6"
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4, delay: 0.1 }}
      >
        <Card
          isPressable
          onPress={() =>
            cp.navigate("/content/worlds", {
              state: { player: cp.selectedPlayer },
            })
          }
          className={cn("h-full", LAYOUT.GLASS_CARD.BASE)}
        >
          <CardBody className="p-6">
            <div className="flex items-center justify-between w-full">
              <div className="flex items-center gap-3">
                <div className="p-2 rounded-xl bg-blue-50 dark:bg-blue-900/20 text-blue-500">
                  <FaGlobe className="w-6 h-6" />
                </div>
                <span className="text-lg font-medium text-default-700 dark:text-zinc-200">
                  {t("contentpage.worlds")}
                </span>
              </div>
              <AnimatePresence mode="wait">
                {cp.loading ? (
                  <motion.div
                    key="spinner"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                  >
                    <Spinner size="sm" />
                  </motion.div>
                ) : (
                  <motion.span
                    key="count"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                    className="text-2xl font-bold text-default-900 dark:text-zinc-100"
                  >
                    {cp.worldsCount}
                  </motion.span>
                )}
              </AnimatePresence>
            </div>
          </CardBody>
        </Card>

        <Card
          isPressable
          onPress={() => cp.navigate("/content/resourcePacks")}
          className={cn("h-full", LAYOUT.GLASS_CARD.BASE)}
        >
          <CardBody className="p-6">
            <div className="flex items-center justify-between w-full">
              <div className="flex items-center gap-3">
                <div className="p-2 rounded-xl bg-purple-50 dark:bg-purple-900/20 text-purple-500">
                  <FaImage className="w-6 h-6" />
                </div>
                <span className="text-lg font-medium text-default-700 dark:text-zinc-200">
                  {t("contentpage.resource_packs")}
                </span>
              </div>
              <AnimatePresence mode="wait">
                {cp.loading ? (
                  <motion.div
                    key="spinner"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                  >
                    <Spinner size="sm" />
                  </motion.div>
                ) : (
                  <motion.span
                    key="count"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                    className="text-2xl font-bold text-default-900 dark:text-zinc-100"
                  >
                    {cp.resCount}
                  </motion.span>
                )}
              </AnimatePresence>
            </div>
          </CardBody>
        </Card>

        <Card
          isPressable
          onPress={() => cp.navigate("/content/behaviorPacks")}
          className={cn("h-full", LAYOUT.GLASS_CARD.BASE)}
        >
          <CardBody className="p-6">
            <div className="flex items-center justify-between w-full">
              <div className="flex items-center gap-3">
                <div className="p-2 rounded-xl bg-orange-50 dark:bg-orange-900/20 text-orange-500">
                  <FaCogs className="w-6 h-6" />
                </div>
                <span className="text-lg font-medium text-default-700 dark:text-zinc-200">
                  {t("contentpage.behavior_packs")}
                </span>
              </div>
              <AnimatePresence mode="wait">
                {cp.loading ? (
                  <motion.div
                    key="spinner"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                  >
                    <Spinner size="sm" />
                  </motion.div>
                ) : (
                  <motion.span
                    key="count"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                    className="text-2xl font-bold text-default-900 dark:text-zinc-100"
                  >
                    {cp.bpCount}
                  </motion.span>
                )}
              </AnimatePresence>
            </div>
          </CardBody>
        </Card>

        <Card
          isPressable
          onPress={() =>
            cp.navigate("/content/skinPacks", {
              state: { player: cp.selectedPlayer },
            })
          }
          className={cn("h-full", LAYOUT.GLASS_CARD.BASE)}
        >
          <CardBody className="p-6">
            <div className="flex items-center justify-between w-full">
              <div className="flex items-center gap-3">
                <div className="p-2 rounded-xl bg-pink-50 dark:bg-pink-900/20 text-pink-500">
                  <FaUserTag className="w-6 h-6" />
                </div>
                <span className="text-lg font-medium text-default-700 dark:text-zinc-200">
                  {t("contentpage.skin_packs")}
                </span>
              </div>
              <AnimatePresence mode="wait">
                {cp.loading ? (
                  <motion.div
                    key="spinner"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                  >
                    <Spinner size="sm" />
                  </motion.div>
                ) : (
                  <motion.span
                    key="count"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                    className="text-2xl font-bold text-default-900 dark:text-zinc-100"
                  >
                    {cp.skinCount}
                  </motion.span>
                )}
              </AnimatePresence>
            </div>
          </CardBody>
        </Card>

        <Card
          isPressable
          onPress={() =>
            cp.navigate("/content/servers", {
              state: { player: cp.selectedPlayer },
            })
          }
          className={cn("h-full", LAYOUT.GLASS_CARD.BASE)}
        >
          <CardBody className="p-6">
            <div className="flex items-center justify-between w-full">
              <div className="flex items-center gap-3">
                <div className="p-2 rounded-xl bg-indigo-50 dark:bg-indigo-900/20 text-indigo-500">
                  <FaServer className="w-6 h-6" />
                </div>
                <span className="text-lg font-medium text-default-700 dark:text-zinc-200">
                  {t("contentpage.servers")}
                </span>
              </div>
              <AnimatePresence mode="wait">
                {cp.loading ? (
                  <motion.div
                    key="spinner"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                  >
                    <Spinner size="sm" />
                  </motion.div>
                ) : (
                  <motion.span
                    key="count"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                    className="text-2xl font-bold text-default-900 dark:text-zinc-100"
                  >
                    {cp.serversCount}
                  </motion.span>
                )}
              </AnimatePresence>
            </div>
          </CardBody>
        </Card>

        <Card
          isPressable
          onPress={() =>
            cp.navigate("/content/screenshots", {
              state: { player: cp.selectedPlayer },
            })
          }
          className={cn("h-full", LAYOUT.GLASS_CARD.BASE)}
        >
          <CardBody className="p-6">
            <div className="flex items-center justify-between w-full">
              <div className="flex items-center gap-3">
                <div className="p-2 rounded-xl bg-teal-50 dark:bg-teal-900/20 text-teal-500">
                  <FaCamera className="w-6 h-6" />
                </div>
                <span className="text-lg font-medium text-default-700 dark:text-zinc-200">
                  {t("contentpage.screenshots")}
                </span>
              </div>
              <AnimatePresence mode="wait">
                {cp.loading ? (
                  <motion.div
                    key="spinner"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                  >
                    <Spinner size="sm" />
                  </motion.div>
                ) : (
                  <motion.span
                    key="count"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                    className="text-2xl font-bold text-default-900 dark:text-zinc-100"
                  >
                    {cp.screenshotsCount}
                  </motion.span>
                )}
              </AnimatePresence>
            </div>
          </CardBody>
        </Card>
      </motion.div>

      <UnifiedModal
        isOpen={cp.importing}
        onOpenChange={() => {}}
        type="primary"
        title={t("mods.importing_title")}
        icon={<FiUploadCloud className="w-6 h-6 text-primary-500" />}
        hideCloseButton
        isDismissable={false}
        showConfirmButton={false}
        showCancelButton={false}
      >
        <div className="flex flex-col gap-4">
          <Progress
            isIndeterminate
            aria-label="importing"
            className="w-full"
            size="sm"
            color="primary"
          />
          <div className="text-default-600 dark:text-zinc-300 text-sm">
            {t("mods.importing_body")}
          </div>
          {cp.currentFile ? (
            <div className="p-3 bg-default-100/50 dark:bg-zinc-800 rounded-xl border border-default-200/50 text-small font-mono text-default-800 dark:text-zinc-200 break-all">
              {cp.currentFile}
            </div>
          ) : null}
        </div>
      </UnifiedModal>
      <ImportResultModal
        isOpen={cp.errOpen}
        onOpenChange={cp.errOnOpenChange}
        results={{ success: cp.resultSuccess, failed: cp.resultFailed }}
        onConfirm={() => {
          cp.setErrorMsg("");
          cp.setResultSuccess([]);
          cp.setResultFailed([]);
        }}
      />
      <UnifiedModal
        isOpen={cp.dupOpen}
        onOpenChange={(open) => {
          if (!open) {
            cp.dupOnClose();
            cp.dupResolveRef.current?.(false);
          }
        }}
        type="warning"
        title={t("mods.overwrite_modal_title")}
        confirmText={t("common.confirm")}
        cancelText={t("common.cancel")}
        showCancelButton
        onConfirm={() => {
          cp.dupResolveRef.current?.(true);
          cp.dupOnClose();
        }}
        onCancel={() => {
          cp.dupResolveRef.current?.(false);
          cp.dupOnClose();
        }}
      >
        <div className="flex flex-col gap-4">
          <div className="text-sm text-default-700 dark:text-zinc-300">
            {t("mods.overwrite_modal_body")}
          </div>
          {cp.dupNameRef.current ? (
            <div className="p-3 bg-default-100/50 dark:bg-zinc-800 rounded-xl border border-default-200/50 text-small font-mono text-default-800 dark:text-zinc-200 break-all">
              {cp.dupNameRef.current}
            </div>
          ) : null}
        </div>
      </UnifiedModal>
      <UnifiedModal
        isOpen={cp.playerSelectOpen}
        onOpenChange={(open) => {
          if (!open) {
            cp.playerSelectOnClose();
            cp.playerSelectResolveRef.current?.("");
          }
        }}
        type="primary"
        title={t("contentpage.select_player_title")}
        cancelText={t("common.cancel")}
        showCancelButton
        showConfirmButton={false}
        onCancel={() => {
          cp.playerSelectResolveRef.current?.("");
          cp.playerSelectOnClose();
        }}
      >
        <div className="flex flex-col gap-4">
          <div className="text-sm text-default-700 dark:text-zinc-300">
            {t("contentpage.select_player_for_import")}
          </div>
          <div className="flex flex-col gap-2 max-h-[300px] overflow-y-auto custom-scrollbar p-1">
            {cp.players.length ? (
              cp.players.map((p) => (
                <Button
                  key={p}
                  variant="flat"
                  className="w-full justify-start bg-default-100 dark:bg-zinc-800 text-default-700 dark:text-zinc-200"
                  onPress={() => {
                    cp.playerSelectResolveRef.current?.(p);
                    cp.playerSelectOnClose();
                  }}
                >
                  {resolvePlayerDisplayName(p, cp.playerGamertagMap)}
                </Button>
              ))
            ) : (
              <div className="text-sm text-default-500 dark:text-zinc-400">
                {t("contentpage.no_players")}
              </div>
            )}
          </div>
        </div>
      </UnifiedModal>
    </PageContainer>
  );
}
