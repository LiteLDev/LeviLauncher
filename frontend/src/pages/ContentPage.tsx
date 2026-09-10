import { openDirectory } from "@/utils/explorer";
import { ModalDescription, ModalPanel, ModalProgress } from "@/components/ModalPrimitives";
import {
  Button,
  Card,
  Dropdown,
  Label,
  ListBox, Select,
  Spinner,
  Tooltip,
  toast,
} from "@heroui/react";

import React from "react";
import { useTranslation } from "react-i18next";

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
  FaExchangeAlt,
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
import { ROUTES } from "@/constants/routes";
import { cn } from "@/utils/cn";
import { COMPONENT_STYLES } from "@/constants/componentStyles";

export default function ContentPage() {
  const { t } = useTranslation();
  const scrollRef = React.useRef<HTMLDivElement>(null);
  const isDragActive = useFileDrag(scrollRef as React.RefObject<HTMLElement>);
  const cp = useContentPage(t as any);

  const importContent = async () => {
    try {
      const paths = await Dialogs.OpenFile({
        Title: t("contentpage.import_button"),
        Filters: [{ DisplayName: "Content Files", Pattern: "*.mcworld;*.mcpack;*.mcaddon" }],
        AllowsMultipleSelection: true,
      });
      if (Array.isArray(paths) && paths.length > 0) {
        await cp.doImportFromPaths(paths);
      }
    } catch (error) {
      console.error(error);
      toast.danger(t("common.load_failed"));
    }
  };

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
          <Card.Content className="p-6">
            <div className="flex flex-col gap-6">
              <div className="flex flex-col xl:flex-row items-start xl:items-center justify-between gap-4">
                <div className="min-w-0 w-full xl:flex-1">
                  <div className="flex items-center gap-3">
                    <PageHeader
                      title={t("launcherpage.content_manage")}
                      titleClassName="pb-1"
                    />
                  </div>
                  <div className="mt-3 rounded-2xl border border-border bg-surface-secondary px-4 py-3 text-sm flex flex-wrap items-center gap-x-3 gap-y-2">
                    <span>{t("contentpage.current_version")}:</span>
                    <span className="font-medium text-foreground dark:text-zinc-200 bg-surface-secondary px-2 py-0.5 rounded-md">
                      {cp.currentVersionName || t("contentpage.none")}
                    </span>
                    <span className="text-muted dark:text-zinc-600">|</span>
                    <span>{t("contentpage.isolation")}:</span>
                    <span className="font-medium text-foreground dark:text-zinc-200 bg-surface-secondary px-2 py-0.5 rounded-md">
                      {cp.roots.isIsolation ? t("common.yes") : t("common.no")}
                    </span>
                    <span className="text-muted dark:text-zinc-600">|</span>
                    <span>{t("contentpage.select_player")}:</span>
                    <Dropdown>
                      <Button
                        size="sm"
                        variant={"secondary"}
                        className={cn(
                          COMPONENT_STYLES.dropdownTriggerButton,
                          "h-6 min-w-0 px-2 text-sm font-medium text-foreground dark:text-zinc-200",
                        )}
                      >
                        {cp.selectedPlayer
                          ? resolvePlayerDisplayName(
                              cp.selectedPlayer,
                              cp.playerGamertagMap,
                            )
                          : t("contentpage.no_players")}
                      </Button>
                      <Dropdown.Popover
                        className={COMPONENT_STYLES.dropdown.content}
                      >
                        <Dropdown.Menu
                          aria-label="Players"
                          selectionMode="single"
                          selectedKeys={new Set([cp.selectedPlayer])}
                          onSelectionChange={(keys) => {
                            const arr = Array.from(
                              keys as unknown as Set<string>,
                            );
                            const next = arr[0] || "";
                            if (typeof next === "string")
                              cp.onChangePlayer(next);
                          }}
                        >
                          {cp.players.length ? (
                            cp.players.map((p) => (
                              <Dropdown.Item
                                key={p}
                                id={p}
                                textValue={resolvePlayerDisplayName(
                                  p,
                                  cp.playerGamertagMap,
                                )}
                              >
                                <Label>
                                  {resolvePlayerDisplayName(
                                    p,
                                    cp.playerGamertagMap,
                                  )}
                                </Label>
                                <Dropdown.ItemIndicator />
                              </Dropdown.Item>
                            ))
                          ) : (
                            <Dropdown.Item
                              key="none"
                              isDisabled
                              id={"none"}
                              textValue={t("contentpage.no_players")}
                            >
                              <Label>{t("contentpage.no_players")}</Label>
                              <Dropdown.ItemIndicator />
                            </Dropdown.Item>
                          )}
                        </Dropdown.Menu>
                      </Dropdown.Popover>
                    </Dropdown>
                    {!cp.selectedPlayer && (
                      <span className="text-rose-500 text-xs">
                        ({t("contentpage.require_player_for_world_import")})
                      </span>
                    )}
                  </div>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <Button
                    onPress={importContent}
                    isDisabled={cp.importing}
                    variant={"secondary"}
                    className={cn(
                      "rounded-full",
                      "bg-brand-500 brand-primary-foreground font-medium shadow-sm",
                    )}
                  >
                    {<FiUploadCloud />}
                    {t("contentpage.import_button")}
                  </Button>
                  <Button
                    onPress={() => cp.openResourceTransferModal()}
                    isDisabled={!cp.hasBackend || cp.importing}
                    variant={"secondary"}
                    className={cn(
                      "rounded-full",
                      "bg-surface-secondary text-foreground dark:text-zinc-200 font-medium",
                    )}
                  >
                    {<FaExchangeAlt />}
                    {t("contentpage.transfer_resources_button")}
                  </Button>
                  <Tooltip>
                    <Button
                      onPress={() => {
                        if (cp.roots.usersRoot) {
                          openDirectory(cp.roots.usersRoot);
                        }
                      }}
                      isDisabled={!cp.hasBackend || !cp.roots.usersRoot}
                      variant={"secondary"}
                      className={cn(
                        "rounded-full",
                        "bg-surface-secondary text-foreground dark:text-zinc-200 font-medium",
                      )}
                    >
                      {<FaFolderOpen />}
                      {t("common.open")}
                    </Button>
                    <Tooltip.Content>
                      {t("contentpage.open_users_dir") as unknown as string}
                    </Tooltip.Content>
                  </Tooltip>
                  <Tooltip>
                    <Button
                      isIconOnly
                      aria-label={t("common.refresh")}
                      onPress={() => cp.refreshAll()}
                      isDisabled={cp.loading}
                      variant={"ghost"}
                      className={"rounded-full"}
                    >
                      <FaSync
                        className={cp.loading ? "animate-spin" : ""}
                        size={18}
                      />
                    </Button>
                    <Tooltip.Content>
                      {t("common.refresh") as unknown as string}
                    </Tooltip.Content>
                  </Tooltip>
                </div>
              </div>
              {!!cp.error && (
                <div className="text-rose-500 text-sm">{cp.error}</div>
              )}
            </div>
          </Card.Content>
        </Card>
      </motion.div>

      {/* Content Grid */}
      <motion.div
        className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6"
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4, delay: 0.1 }}
      >
        <Card className={cn("h-full", LAYOUT.GLASS_CARD.BASE)}>
          <button
            type="button"
            className="w-full cursor-pointer text-left rounded-[inherit] focus-visible:outline-2 focus-visible:outline-focus"
            onClick={() =>
              cp.navigate(ROUTES.contentWorlds, {
                state: { player: cp.selectedPlayer },
              })
            }
          >
            <Card.Content className="p-6">
              <div className="flex items-center justify-between w-full">
                <div className="flex items-center gap-3">
                  <div className="p-2 rounded-xl bg-blue-50 dark:bg-blue-900/20 text-blue-500">
                    <FaGlobe className="w-6 h-6" />
                  </div>
                  <span className="text-lg font-medium">
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
                      className="text-2xl font-bold"
                    >
                      {cp.worldsCount}
                    </motion.span>
                  )}
                </AnimatePresence>
              </div>
            </Card.Content>
          </button>
                  {!cp.loading && !cp.error && cp.worldsCount === 0 && <Card.Footer className="flex flex-wrap gap-2 px-6 pb-4 pt-0">
            <Button size="sm" variant="secondary" isDisabled={cp.importing || !cp.hasBackend} onPress={importContent}>{t("contentpage.import_button")}</Button>
            <Button size="sm" variant="ghost" onPress={() => cp.navigate(ROUTES.curseForge)}>{t("audit.usability.browse_content")}</Button>
          </Card.Footer>}
        </Card>

        <Card className={cn("h-full", LAYOUT.GLASS_CARD.BASE)}>
          <button
            type="button"
            className="w-full cursor-pointer text-left rounded-[inherit] focus-visible:outline-2 focus-visible:outline-focus"
            onClick={() => cp.navigate(ROUTES.contentResourcePacks)}
          >
            <Card.Content className="p-6">
              <div className="flex items-center justify-between w-full">
                <div className="flex items-center gap-3">
                  <div className="p-2 rounded-xl bg-purple-50 dark:bg-purple-900/20 text-purple-500">
                    <FaImage className="w-6 h-6" />
                  </div>
                  <span className="text-lg font-medium">
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
                      className="text-2xl font-bold"
                    >
                      {cp.resCount}
                    </motion.span>
                  )}
                </AnimatePresence>
              </div>
            </Card.Content>
          </button>
                  {!cp.loading && !cp.error && cp.resCount === 0 && <Card.Footer className="flex flex-wrap gap-2 px-6 pb-4 pt-0">
            <Button size="sm" variant="secondary" isDisabled={cp.importing || !cp.hasBackend} onPress={importContent}>{t("contentpage.import_button")}</Button>
            <Button size="sm" variant="ghost" onPress={() => cp.navigate(ROUTES.curseForge)}>{t("audit.usability.browse_content")}</Button>
          </Card.Footer>}
        </Card>

        <Card className={cn("h-full", LAYOUT.GLASS_CARD.BASE)}>
          <button
            type="button"
            className="w-full cursor-pointer text-left rounded-[inherit] focus-visible:outline-2 focus-visible:outline-focus"
            onClick={() => cp.navigate(ROUTES.contentBehaviorPacks)}
          >
            <Card.Content className="p-6">
              <div className="flex items-center justify-between w-full">
                <div className="flex items-center gap-3">
                  <div className="p-2 rounded-xl bg-orange-50 dark:bg-orange-900/20 text-orange-500">
                    <FaCogs className="w-6 h-6" />
                  </div>
                  <span className="text-lg font-medium">
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
                      className="text-2xl font-bold"
                    >
                      {cp.bpCount}
                    </motion.span>
                  )}
                </AnimatePresence>
              </div>
            </Card.Content>
          </button>
                  {!cp.loading && !cp.error && cp.bpCount === 0 && <Card.Footer className="flex flex-wrap gap-2 px-6 pb-4 pt-0">
            <Button size="sm" variant="secondary" isDisabled={cp.importing || !cp.hasBackend} onPress={importContent}>{t("contentpage.import_button")}</Button>
            <Button size="sm" variant="ghost" onPress={() => cp.navigate(ROUTES.curseForge)}>{t("audit.usability.browse_content")}</Button>
          </Card.Footer>}
        </Card>

        <Card className={cn("h-full", LAYOUT.GLASS_CARD.BASE)}>
          <button
            type="button"
            className="w-full cursor-pointer text-left rounded-[inherit] focus-visible:outline-2 focus-visible:outline-focus"
            onClick={() =>
              cp.navigate(ROUTES.contentSkinPacks, {
                state: { player: cp.selectedPlayer },
              })
            }
          >
            <Card.Content className="p-6">
              <div className="flex items-center justify-between w-full">
                <div className="flex items-center gap-3">
                  <div className="p-2 rounded-xl bg-pink-50 dark:bg-pink-900/20 text-pink-500">
                    <FaUserTag className="w-6 h-6" />
                  </div>
                  <span className="text-lg font-medium">
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
                      className="text-2xl font-bold"
                    >
                      {cp.skinCount}
                    </motion.span>
                  )}
                </AnimatePresence>
              </div>
            </Card.Content>
          </button>
                  {!cp.loading && !cp.error && cp.skinCount === 0 && <Card.Footer className="flex flex-wrap gap-2 px-6 pb-4 pt-0">
            <Button size="sm" variant="secondary" isDisabled={cp.importing || !cp.hasBackend} onPress={importContent}>{t("contentpage.import_button")}</Button>
            <Button size="sm" variant="ghost" onPress={() => cp.navigate(ROUTES.curseForge)}>{t("audit.usability.browse_content")}</Button>
          </Card.Footer>}
        </Card>

        <Card className={cn("h-full", LAYOUT.GLASS_CARD.BASE)}>
          <button
            type="button"
            className="w-full cursor-pointer text-left rounded-[inherit] focus-visible:outline-2 focus-visible:outline-focus"
            onClick={() =>
              cp.navigate(ROUTES.contentServers, {
                state: { player: cp.selectedPlayer },
              })
            }
          >
            <Card.Content className="p-6">
              <div className="flex items-center justify-between w-full">
                <div className="flex items-center gap-3">
                  <div className="p-2 rounded-xl bg-indigo-50 dark:bg-indigo-900/20 text-indigo-500">
                    <FaServer className="w-6 h-6" />
                  </div>
                  <span className="text-lg font-medium">
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
                      className="text-2xl font-bold"
                    >
                      {cp.serversCount}
                    </motion.span>
                  )}
                </AnimatePresence>
              </div>
            </Card.Content>
          </button>
        </Card>

        <Card className={cn("h-full", LAYOUT.GLASS_CARD.BASE)}>
          <button
            type="button"
            className="w-full cursor-pointer text-left rounded-[inherit] focus-visible:outline-2 focus-visible:outline-focus"
            onClick={() =>
              cp.navigate(ROUTES.contentScreenshots, {
                state: { player: cp.selectedPlayer },
              })
            }
          >
            <Card.Content className="p-6">
              <div className="flex items-center justify-between w-full">
                <div className="flex items-center gap-3">
                  <div className="p-2 rounded-xl bg-teal-50 dark:bg-teal-900/20 text-teal-500">
                    <FaCamera className="w-6 h-6" />
                  </div>
                  <span className="text-lg font-medium">
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
                      className="text-2xl font-bold"
                    >
                      {cp.screenshotsCount}
                    </motion.span>
                  )}
                </AnimatePresence>
              </div>
            </Card.Content>
          </button>
        </Card>
      </motion.div>

      <UnifiedModal
        isOpen={cp.importing}
        type="primary"
        title={
          cp.transferring
            ? t("contentpage.transfer_progress_title")
            : t("mods.importing_title")
        }
        icon={<FiUploadCloud className="w-6 h-6" />}
        isDismissable={false}
        showConfirmButton={false}
        showCancelButton={false}
      >
        <ModalProgress
          label={t("contentpage.transfer_progress_title")}
          description={<> {cp.transferring
              ? t("contentpage.transfer_progress_body")
              : t("mods.importing_body")} </>}
          currentItem={cp.currentFile}
        />
      </UnifiedModal>
      <UnifiedModal
        size="wide"
        isOpen={cp.transferTargetOpen}
        onOpenChange={(open) => {
          if (!open) {
            cp.transferTargetOnClose();
          }
        }}
        type="primary"
        title={t("contentpage.transfer_resources_title")}
        confirmText={t("contentpage.transfer_resources_button")}
        cancelText={t("common.cancel")}
        showCancelButton
        onConfirm={() => cp.transferResourcesToTargets()}
        onCancel={() => cp.transferTargetOnClose()}
        confirmButtonProps={{
          isDisabled: cp.selectedTransferTargets.length === 0 || cp.importing,
        }}
      >
        <div className="flex flex-col gap-4">
          <ModalDescription>
            {t("contentpage.transfer_resources_body_overview")}
          </ModalDescription>

          {cp.transferTargets.length > 0 ? (
            <Select
              placeholder={t("contentpage.transfer_target_placeholder")}
              value={Array.from(new Set(cp.selectedTransferTargets))[0] ?? null}
              onChange={(keys) => {
                const selected = [keys].map(String);
                cp.setSelectedTransferTargets(selected);
              }}
            >
              <Label>{t("mirror.target") || "Target Instance"}</Label>
              <Select.Trigger className={COMPONENT_STYLES.select.trigger}>
                <Select.Value />
                <Select.Indicator />
              </Select.Trigger>
              <Select.Popover
                className={COMPONENT_STYLES.select.popoverContent}
              >
                <ListBox
                  items={cp.transferTargets}
                  className={COMPONENT_STYLES.select.listbox}
                >
                  {(item) => (
                    <ListBox.Item
                      key={item.name}
                      id={item.name}
                      textValue={item.name}
                    >
                      <Label>
                        <div className="flex gap-2 items-center">
                          <div className="w-8 h-8 rounded bg-surface-tertiary flex items-center justify-center overflow-hidden">
                            <img
                              src={
                                item.icon ||
                                "https://raw.githubusercontent.com/LiteLDev/LeviLauncher/main/build/appicon.png"
                              }
                              alt="icon"
                              className="w-full h-full object-cover"
                              onError={(e) =>
                                (e.currentTarget.style.display = "none")
                              }
                            />
                          </div>
                          <div className="flex flex-col">
                            <span className="text-sm">{item.name}</span>
                            <span className="text-xs text-muted">
                              {item.gameVersion}
                            </span>
                          </div>
                        </div>
                      </Label>
                      <ListBox.ItemIndicator />
                    </ListBox.Item>
                  )}
                </ListBox>
              </Select.Popover>
            </Select>
          ) : (
            <div className="flex flex-col items-center justify-center py-8 text-muted dark:text-zinc-500">
              <FaExchangeAlt className="text-4xl mb-3 opacity-20" />
              <ModalDescription>{t("contentpage.transfer_no_targets")}</ModalDescription>
            </div>
          )}
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
        confirmText={t("mods.overwrite_and_import")}
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
        <ModalDescription>{t("mods.overwrite_modal_body")}</ModalDescription>
        {cp.dupNameRef.current ? <ModalPanel className="font-mono">{cp.dupNameRef.current}</ModalPanel> : null}
      </UnifiedModal>
      <UnifiedModal
        size="wide"
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
          <ModalDescription>
            {t("contentpage.select_player_for_import")}
          </ModalDescription>
          <div className="flex flex-col gap-2 max-h-[300px] overflow-y-auto custom-scrollbar p-1">
            {cp.players.length ? (
              cp.players.map((p) => (
                <Button
                  key={p}
                  onPress={() => {
                    cp.playerSelectResolveRef.current?.(p);
                    cp.playerSelectOnClose();
                  }}
                  variant={"secondary"}
                  className={
                    "w-full justify-start bg-surface-secondary text-foreground dark:text-zinc-200"
                  }
                >
                  {resolvePlayerDisplayName(p, cp.playerGamertagMap)}
                </Button>
              ))
            ) : (
              <div className="text-sm text-muted dark:text-zinc-400">
                {t("contentpage.no_players")}
              </div>
            )}
          </div>
        </div>
      </UnifiedModal>
    </PageContainer>
  );
}
