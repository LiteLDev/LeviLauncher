import { openDirectory } from "@/utils/explorer";
import { ModalDescription } from "@/components/ModalPrimitives";
import {
  Button,
  Card,
  Dropdown,
  Label,
  ListBox,
  Select,
  Spinner,
  Tooltip,
} from "@heroui/react";

import React from "react";
import { useTranslation } from "react-i18next";

import { Dialogs } from "@wailsio/runtime";
import { UnifiedModal } from "@/components/UnifiedModal";
import { AnimatePresence, motion } from "framer-motion";
import {
  FaChevronRight,
  FaCube,
  FaExchangeAlt,
  FaFolderOpen,
  FaSync,
} from "react-icons/fa";
import { FiUploadCloud } from "react-icons/fi";
import { resolvePlayerDisplayName } from "@/utils/content";
import { FileDropOverlay } from "@/components/FileDropOverlay";
import { ContentTaskDialog } from "@/components/ContentTaskDialog";
import { CONTENT_SECTION_META } from "@/components/ShellControls";
import { useFileDrag } from "@/hooks/useFileDrag";
import { useContentPage } from "@/hooks/useContentPage";
import { LAYOUT } from "@/constants/layout";
import { CONTENT_SECTIONS, type ContentSectionKey } from "@/constants/routes";
import { cn } from "@/utils/cn";
import { notify } from "@/utils/activity";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { useModIntelligence } from "@/utils/ModIntelligenceContext";

/** Pages that list one player's content get the player chosen here. */
const PLAYER_SCOPED: ReadonlySet<ContentSectionKey> = new Set([
  "worlds",
  "skinPacks",
  "servers",
  "screenshots",
]);

/** Icon button in the card header; the label doubles as its tooltip. */
const HeaderAction: React.FC<{
  label: string;
  tooltip?: string;
  onPress: () => void;
  isDisabled?: boolean;
  children: React.ReactNode;
}> = ({ label, tooltip, onPress, isDisabled, children }) => (
  <Tooltip delay={300} closeDelay={0}>
    <Button
      isIconOnly
      size="sm"
      variant="ghost"
      aria-label={label}
      onPress={onPress}
      isDisabled={isDisabled}
      className="rounded-full text-foreground dark:text-zinc-300"
    >
      {children}
    </Button>
    <Tooltip.Content>{tooltip || label}</Tooltip.Content>
  </Tooltip>
);

/**
 * The current instance's content on Home. It keeps the card released
 * versions showed under the launch card, now listing every kind of content
 * (mods second) with import, transfer and the player choice in place. The
 * instance itself is chosen in the top bar.
 */
export const HomeContentCard: React.FC<{
  dropZoneRef: React.RefObject<HTMLElement | null>;
}> = ({ dropZoneRef }) => {
  const { t } = useTranslation();
  const isDragActive = useFileDrag(dropZoneRef as React.RefObject<HTMLElement>);
  const cp = useContentPage(t as any);
  const hasInstance = !!cp.currentVersionName;
  const { ensureInstanceHydrated, getInstanceSnapshot, snapshotRevision } =
    useModIntelligence();

  React.useEffect(() => {
    if (!cp.currentVersionName) return;
    void ensureInstanceHydrated(cp.currentVersionName, {
      background: true,
      reason: "home-content-mods",
    });
  }, [cp.currentVersionName, ensureInstanceHydrated]);

  const modsCount = React.useMemo(() => {
    if (!cp.currentVersionName) return 0;
    return getInstanceSnapshot(cp.currentVersionName)?.modsInfo?.length ?? 0;
  }, [cp.currentVersionName, getInstanceSnapshot, snapshotRevision]);

  const counts: Record<ContentSectionKey, number> = {
    worlds: cp.worldsCount,
    mods: modsCount,
    resourcePacks: cp.resCount,
    behaviorPacks: cp.bpCount,
    skinPacks: cp.skinCount,
    servers: cp.serversCount,
    screenshots: cp.screenshotsCount,
  };

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

  return (
    <>
      <FileDropOverlay
        isDragActive={isDragActive && hasInstance}
        text={t("contentpage.drop_hint")}
      />

      <Card
        className={cn(
          "h-full transition-all group",
          LAYOUT.GLASS_CARD.BASE,
          "rounded-3xl",
        )}
      >
        <Card.Header className="min-h-14 px-5 py-2 border-b border-border dark:border-white/5 flex flex-wrap justify-between items-center gap-2">
          <div className="flex min-w-0 items-center gap-2">
            <div className="shrink-0 p-1.5 rounded-lg bg-pink-500/10 text-pink-600 dark:text-pink-400">
              <FaCube size={16} />
            </div>
            <h3 className="truncate text-base font-bold text-foreground dark:text-zinc-100">
              {t("launcherpage.content_manage")}
            </h3>
          </div>
          <div className="flex shrink-0 items-center gap-0.5">
            <HeaderAction
              label={t("contentpage.import_button")}
              onPress={importContent}
              isDisabled={cp.importing || !hasInstance}
            >
              <FiUploadCloud size={16} />
            </HeaderAction>
            <HeaderAction
              label={t("contentpage.transfer_resources_button")}
              onPress={() => cp.openResourceTransferModal()}
              isDisabled={cp.importing || !hasInstance}
            >
              <FaExchangeAlt size={14} />
            </HeaderAction>
            <HeaderAction
              label={t("common.open")}
              tooltip={t(
                cp.roots.packageType === "uwp"
                  ? "uwp.open_content_dir"
                  : "contentpage.open_users_dir",
              )}
              onPress={() => {
                const path = cp.roots.comMojangRoot || cp.roots.usersRoot;
                if (path) openDirectory(path);
              }}
              isDisabled={!(cp.roots.comMojangRoot || cp.roots.usersRoot)}
            >
              <FaFolderOpen size={15} />
            </HeaderAction>
            <HeaderAction
              label={t("common.refresh")}
              onPress={() => cp.refreshAll()}
              isDisabled={cp.loading || !hasInstance}
            >
              <FaSync className={cp.loading ? "animate-spin" : ""} size={13} />
            </HeaderAction>
          </div>
        </Card.Header>
        <Card.Content className="@container p-3 gap-2 relative">
          {hasInstance ? (
            <div className="flex flex-wrap items-center gap-x-2 gap-y-1 px-2 text-xs text-muted">
              <span>
                {t("contentpage.isolation")}:{" "}
                <span className="font-medium text-foreground dark:text-zinc-200">
                  {cp.roots.isIsolation ? t("common.yes") : t("common.no")}
                </span>
              </span>
              {cp.roots.packageType !== "uwp" && (
                <>
                  <span aria-hidden="true">|</span>
                  <span>{t("contentpage.select_player")}:</span>
                  <Dropdown>
                    <Button
                      size="sm"
                      variant={"secondary"}
                      className={cn(
                        COMPONENT_STYLES.dropdownTriggerButton,
                        "h-6 min-w-0 max-w-full px-2 text-xs font-medium text-foreground dark:text-zinc-200",
                      )}
                    >
                      <span className="truncate">
                        {cp.selectedPlayer
                          ? resolvePlayerDisplayName(cp.selectedPlayer, cp.playerGamertagMap)
                          : t("contentpage.no_players")}
                      </span>
                    </Button>
                    <Dropdown.Popover className={COMPONENT_STYLES.dropdown.content}>
                      <Dropdown.Menu
                        aria-label="Players"
                        selectionMode="single"
                        selectedKeys={new Set([cp.selectedPlayer])}
                        onSelectionChange={(keys) => {
                          const next = Array.from(keys as unknown as Set<string>)[0] || "";
                          if (typeof next === "string") cp.onChangePlayer(next);
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
                    <span className="text-rose-500">
                      ({t("contentpage.require_player_for_world_import")})
                    </span>
                  )}
                </>
              )}
              {cp.roots.packageType === "uwp" && !cp.roots.isIsolation && (
                <>
                  <span aria-hidden="true">|</span>
                  <span>{t("uwp.shared_content")}</span>
                </>
              )}
            </div>
          ) : (
            <p role="status" className="px-2 text-xs text-muted">
              {t("content_sub.no_instance_body")}
            </p>
          )}
          {!!cp.error && (
            <p role="alert" className="px-2 text-xs text-rose-500">
              {cp.error}
            </p>
          )}

          <ul
            aria-label={t("launcherpage.content_manage")}
            className="grid grid-cols-1 gap-x-2 gap-y-1 @lg:grid-cols-2"
          >
            {CONTENT_SECTIONS.map((section) => {
              const meta = CONTENT_SECTION_META[section.key];
              return (
                <li key={section.key} className="min-w-0">
                  <button
                    type="button"
                    disabled={!hasInstance}
                    className="group/item flex w-full min-w-0 items-center justify-between gap-2 p-2 text-left rounded-xl hover:bg-surface-tertiary/50 cursor-pointer transition-all duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 disabled:cursor-not-allowed disabled:opacity-60"
                    onClick={() =>
                      cp.navigate(
                        section.path,
                        PLAYER_SCOPED.has(section.key)
                          ? { state: { player: cp.selectedPlayer } }
                          : undefined,
                      )
                    }
                  >
                    <div className="flex min-w-0 items-center gap-3">
                      <div
                        className={cn(
                          "shrink-0 p-1.5 rounded-lg bg-surface-secondary dark:bg-surface/20",
                          meta.iconClassName,
                        )}
                      >
                        <meta.Icon size={16} />
                      </div>
                      <span className="min-w-0 truncate font-medium text-sm">
                        {t(meta.labelKey)}
                      </span>
                    </div>
                    <div className="flex shrink-0 items-center gap-2">
                      <AnimatePresence mode="wait" initial={false}>
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
                            className="text-base font-bold tabular-nums"
                          >
                            {hasInstance ? counts[section.key] : "–"}
                          </motion.span>
                        )}
                      </AnimatePresence>
                      <FaChevronRight aria-hidden="true" size={10} />
                    </div>
                  </button>
                </li>
              );
            })}
          </ul>
        </Card.Content>
      </Card>

      <ContentTaskDialog task={cp.importTask} players={playerOptions} />
      <ContentTaskDialog task={cp.transferTask} />
      <UnifiedModal
        size="wide"
        isOpen={cp.transferTargetOpen}
        onOpenChange={(open) => {
          if (!open) cp.transferTargetOnClose();
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
                cp.setSelectedTransferTargets([keys].map(String));
              }}
            >
              <Label>{t("mirror.target") || "Target Instance"}</Label>
              <Select.Trigger className={COMPONENT_STYLES.select.trigger}>
                <Select.Value />
                <Select.Indicator />
              </Select.Trigger>
              <Select.Popover className={COMPONENT_STYLES.select.popoverContent}>
                <ListBox items={cp.transferTargets} className={COMPONENT_STYLES.select.listbox}>
                  {(item) => (
                    <ListBox.Item key={item.name} id={item.name} textValue={item.name}>
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
                              onError={(e) => (e.currentTarget.style.display = "none")}
                            />
                          </div>
                          <div className="flex flex-col">
                            <span className="text-sm">{item.name}</span>
                            <span className="text-xs text-muted">{item.gameVersion}</span>
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
    </>
  );
};
