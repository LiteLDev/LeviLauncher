import { openDirectory } from "@/utils/explorer";
import { ModalDescription } from "@/components/ModalPrimitives";
import {
  Button,
  Card,
  Dropdown,
  Label,
  ListBox, Select,
  Spinner,
  Tooltip,
} from "@heroui/react";

import React from "react";
import { useTranslation } from "react-i18next";

import { Dialogs } from "@wailsio/runtime";
import { UnifiedModal } from "@/components/UnifiedModal";
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
import { FiUploadCloud } from "react-icons/fi";
import { PageHeader } from "@/components/PageHeader";
import { PageContainer } from "@/components/PageContainer";
import { FileDropOverlay } from "@/components/FileDropOverlay";
import { InstancePicker } from "@/components/InstancePicker";
import { ContentTaskDialog } from "@/components/ContentTaskDialog";
import { useFileDrag } from "@/hooks/useFileDrag";
import { useContentPage } from "@/hooks/useContentPage";
import { LAYOUT } from "@/constants/layout";
import { ROUTES } from "@/constants/routes";
import { cn } from "@/utils/cn";
import { notify } from "@/utils/activity";
import { COMPONENT_STYLES } from "@/constants/componentStyles";

export default function ContentPage() {
  const { t } = useTranslation();
  const scrollRef = React.useRef<HTMLDivElement>(null);
  const isDragActive = useFileDrag(scrollRef as React.RefObject<HTMLElement>);
  const cp = useContentPage(t as any);
  const hasInstance = !!cp.currentVersionName;

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
      notify({
        variant: "danger",
        title: t("common.load_failed"),
        description: String((error as any)?.message || error || ""),
        source: "content",
      });
    }
  };

  const playerOptions = cp.players.map((p) => ({
    id: p,
    label: resolvePlayerDisplayName(p, cp.playerGamertagMap),
  }));

  // Each category is one click away; player-scoped pages get the chosen player.
  const categories = [
    { key: "worlds", route: ROUTES.contentWorlds, label: t("contentpage.worlds"), count: cp.worldsCount, icon: FaGlobe, tone: "bg-blue-50 dark:bg-blue-900/20 text-blue-500", withPlayer: true },
    { key: "resource", route: ROUTES.contentResourcePacks, label: t("contentpage.resource_packs"), count: cp.resCount, icon: FaImage, tone: "bg-purple-50 dark:bg-purple-900/20 text-purple-500", withPlayer: false },
    { key: "behavior", route: ROUTES.contentBehaviorPacks, label: t("contentpage.behavior_packs"), count: cp.bpCount, icon: FaCogs, tone: "bg-orange-50 dark:bg-orange-900/20 text-orange-500", withPlayer: false },
    { key: "skins", route: ROUTES.contentSkinPacks, label: t("contentpage.skin_packs"), count: cp.skinCount, icon: FaUserTag, tone: "bg-pink-50 dark:bg-pink-900/20 text-pink-500", withPlayer: true },
    { key: "servers", route: ROUTES.contentServers, label: t("contentpage.servers"), count: cp.serversCount, icon: FaServer, tone: "bg-indigo-50 dark:bg-indigo-900/20 text-indigo-500", withPlayer: true },
    { key: "screenshots", route: ROUTES.contentScreenshots, label: t("contentpage.screenshots"), count: cp.screenshotsCount, icon: FaCamera, tone: "bg-teal-50 dark:bg-teal-900/20 text-teal-500", withPlayer: true },
  ];

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
        data-material-motion
        initial={{ opacity: 0, y: -20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4 }}
      >
        <Card className={LAYOUT.GLASS_CARD.BASE}>
          <Card.Content className="p-4 min-[1200px]:p-6">
            <div className="flex flex-col gap-4">
              <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-3">
                <div className="flex min-w-0 flex-wrap items-center gap-x-4 gap-y-2">
                  <PageHeader
                    title={t("launcherpage.content_manage")}
                    titleClassName="pb-1"
                  />
                  <InstancePicker size="sm" />
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <Button
                    onPress={importContent}
                    isDisabled={cp.importing || !hasInstance}
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
                    isDisabled={cp.importing || !hasInstance}
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
                        const path = cp.roots.comMojangRoot || cp.roots.usersRoot;
                        if (path) {
                          openDirectory(path);
                        }
                      }}
                      isDisabled={!(cp.roots.comMojangRoot || cp.roots.usersRoot)}
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
                      {t(cp.roots.packageType === "uwp" ? "uwp.open_content_dir" : "contentpage.open_users_dir") as unknown as string}
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
              {hasInstance ? (
                <div className="rounded-2xl border border-border bg-surface-secondary px-4 py-3 text-sm flex flex-wrap items-center gap-x-3 gap-y-2">
                  <span>{t("contentpage.isolation")}:</span>
                  <span className="font-medium text-foreground dark:text-zinc-200 bg-surface-secondary px-2 py-0.5 rounded-md">
                    {cp.roots.isIsolation ? t("common.yes") : t("common.no")}
                  </span>
                  {cp.roots.packageType !== "uwp" && <>
                  <span className="text-muted dark:text-zinc-600">|</span>
                  <span>{t("contentpage.select_player")}:</span>
                  <Dropdown>
                    <Button
                      size="sm"
                      variant={"secondary"}
                      className={cn(
                        COMPONENT_STYLES.dropdownTriggerButton,
                        "h-6 min-w-0 max-w-full px-2 text-sm font-medium text-foreground dark:text-zinc-200",
                      )}
                    >
                      <span className="truncate">
                        {cp.selectedPlayer
                          ? resolvePlayerDisplayName(
                              cp.selectedPlayer,
                              cp.playerGamertagMap,
                            )
                          : t("contentpage.no_players")}
                      </span>
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
                          playerOptions.map((p) => (
                            <Dropdown.Item key={p.id} id={p.id} textValue={p.label}>
                              <Label>{p.label}</Label>
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
                  </>}
                  {cp.roots.packageType === "uwp" && !cp.roots.isIsolation && <>
                    <span className="text-muted dark:text-zinc-600">|</span>
                    <span>{t("uwp.shared_content")}</span>
                  </>}
                </div>
              ) : (
                <div role="status" className="rounded-2xl border border-border bg-surface-secondary px-4 py-3 text-sm text-muted">
                  {t("content_task.choose_instance_hint")}
                </div>
              )}
              {!!cp.error && (
                <div role="alert" className="text-rose-500 text-sm">{cp.error}</div>
              )}
            </div>
          </Card.Content>
        </Card>
      </motion.div>

      {/* Content Grid */}
      {/* Tailwind source for LAYOUT.GRID.AUTO(16): grid-cols-[repeat(auto-fill,minmax(min(100%,16rem),1fr))] */}
      <motion.div
        data-material-motion
        className={LAYOUT.GRID.AUTO(16)}
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4, delay: 0.1 }}
      >
        {categories.map((c) => (
          <Card key={c.key} className={cn("h-full", LAYOUT.GLASS_CARD.BASE)}>
            <button
              type="button"
              className="w-full h-full cursor-pointer text-left rounded-[inherit] focus-visible:outline-2 focus-visible:outline-focus"
              onClick={() =>
                cp.navigate(
                  c.route,
                  c.withPlayer ? { state: { player: cp.selectedPlayer } } : undefined,
                )
              }
            >
              <Card.Content className="p-5">
                <div className="flex items-center justify-between gap-3 w-full">
                  <div className="flex min-w-0 items-center gap-3">
                    <div className={cn("p-2 rounded-xl shrink-0", c.tone)}>
                      <c.icon className="w-6 h-6" />
                    </div>
                    <span className="text-lg font-medium truncate">{c.label}</span>
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
                        className="text-2xl font-bold tabular-nums"
                      >
                        {c.count}
                      </motion.span>
                    )}
                  </AnimatePresence>
                </div>
              </Card.Content>
            </button>
          </Card>
        ))}
      </motion.div>

      <ContentTaskDialog task={cp.importTask} players={playerOptions} />
      <ContentTaskDialog task={cp.transferTask} />
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
    </PageContainer>
  );
}
