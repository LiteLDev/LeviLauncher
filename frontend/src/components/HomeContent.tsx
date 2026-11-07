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
import { useNavigate } from "react-router-dom";

import { Dialogs } from "@wailsio/runtime";
import { UnifiedModal } from "@/components/UnifiedModal";
import { AnimatePresence, motion } from "framer-motion";
import {
  FaChevronRight,
  FaCloudDownloadAlt,
  FaCube,
  FaExchangeAlt,
  FaFolderOpen,
  FaSync,
} from "react-icons/fa";
import { FiUploadCloud } from "react-icons/fi";
import { resolvePlayerDisplayName } from "@/utils/content";
import { FileDropOverlay } from "@/components/FileDropOverlay";
import { ContentTaskDialog } from "@/components/ContentTaskDialog";
import {
  CATALOG_META,
  CONTENT_SECTION_META,
} from "@/components/ShellControls";
import { useFileDrag } from "@/hooks/useFileDrag";
import { useContentPage } from "@/hooks/useContentPage";
import { LAYOUT } from "@/constants/layout";
import {
  CONTENT_SECTIONS,
  DISCOVER_CATALOGS,
  type ContentSectionKey,
} from "@/constants/routes";
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

const tileClass =
  "group/tile flex w-full min-w-0 items-center justify-between gap-3 rounded-2xl border border-border/60 bg-surface-secondary/40 p-3 text-left transition-colors hover:bg-surface-secondary dark:border-white/5 cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 disabled:cursor-not-allowed disabled:opacity-60";

const SectionTitle: React.FC<{
  icon: React.ReactNode;
  tone: string;
  children: React.ReactNode;
}> = ({ icon, tone, children }) => (
  <div className="flex min-w-0 items-center gap-2">
    <div className={cn("shrink-0 p-1.5 rounded-lg", tone)}>{icon}</div>
    <h2 className="truncate text-lg font-bold text-foreground dark:text-zinc-100">
      {children}
    </h2>
  </div>
);

/**
 * The current instance's content, on Home: every kind of content is one
 * click away, with import, transfer and the player choice alongside. The
 * instance itself is chosen in the top bar.
 */
export const HomeContentSection: React.FC<{
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

      <Card className={LAYOUT.GLASS_CARD.BASE}>
        <Card.Content className="@container flex flex-col gap-4 p-4 sm:p-6">
          <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-3">
            <SectionTitle
              icon={<FaCube size={16} />}
              tone="bg-pink-500/10 text-pink-600 dark:text-pink-400"
            >
              {t("launcherpage.content_manage")}
            </SectionTitle>
            <div className="flex flex-wrap items-center gap-2">
              <Button
                size="sm"
                onPress={importContent}
                isDisabled={cp.importing || !hasInstance}
                variant={"secondary"}
                className="rounded-full bg-brand-500 brand-primary-foreground font-medium shadow-sm"
              >
                <FiUploadCloud />
                {t("contentpage.import_button")}
              </Button>
              <Button
                size="sm"
                onPress={() => cp.openResourceTransferModal()}
                isDisabled={cp.importing || !hasInstance}
                variant={"secondary"}
                className="rounded-full bg-surface-secondary text-foreground dark:text-zinc-200 font-medium"
              >
                <FaExchangeAlt />
                {t("contentpage.transfer_resources_button")}
              </Button>
              <Tooltip>
                <Button
                  size="sm"
                  onPress={() => {
                    const path = cp.roots.comMojangRoot || cp.roots.usersRoot;
                    if (path) openDirectory(path);
                  }}
                  isDisabled={!(cp.roots.comMojangRoot || cp.roots.usersRoot)}
                  variant={"secondary"}
                  className="rounded-full bg-surface-secondary text-foreground dark:text-zinc-200 font-medium"
                >
                  <FaFolderOpen />
                  {t("common.open")}
                </Button>
                <Tooltip.Content>
                  {t(cp.roots.packageType === "uwp" ? "uwp.open_content_dir" : "contentpage.open_users_dir") as unknown as string}
                </Tooltip.Content>
              </Tooltip>
              <Tooltip>
                <Button
                  isIconOnly
                  size="sm"
                  aria-label={t("common.refresh")}
                  onPress={() => cp.refreshAll()}
                  isDisabled={cp.loading || !hasInstance}
                  variant={"ghost"}
                  className={"rounded-full"}
                >
                  <FaSync className={cp.loading ? "animate-spin" : ""} size={16} />
                </Button>
                <Tooltip.Content>{t("common.refresh") as unknown as string}</Tooltip.Content>
              </Tooltip>
            </div>
          </div>

          {hasInstance ? (
            <div className="rounded-2xl border border-border bg-surface-secondary px-4 py-2.5 text-sm flex flex-wrap items-center gap-x-3 gap-y-2">
              <span>{t("contentpage.isolation")}:</span>
              <span className="font-medium text-foreground dark:text-zinc-200 bg-surface-secondary px-2 py-0.5 rounded-md">
                {cp.roots.isIsolation ? t("common.yes") : t("common.no")}
              </span>
              {cp.roots.packageType !== "uwp" && (
                <>
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
                    <span className="text-rose-500 text-xs">
                      ({t("contentpage.require_player_for_world_import")})
                    </span>
                  )}
                </>
              )}
              {cp.roots.packageType === "uwp" && !cp.roots.isIsolation && (
                <>
                  <span className="text-muted dark:text-zinc-600">|</span>
                  <span>{t("uwp.shared_content")}</span>
                </>
              )}
            </div>
          ) : (
            <div role="status" className="rounded-2xl border border-border bg-surface-secondary px-4 py-2.5 text-sm text-muted">
              {t("content_sub.no_instance_body")}
            </div>
          )}
          {!!cp.error && (
            <div role="alert" className="text-rose-500 text-sm">{cp.error}</div>
          )}

          {/* Tailwind source for LAYOUT.GRID.AUTO(14): grid-cols-[repeat(auto-fill,minmax(min(100%,14rem),1fr))] */}
          <ul aria-label={t("launcherpage.content_manage")} className={LAYOUT.GRID.AUTO(14)}>
            {CONTENT_SECTIONS.map((section) => {
              const meta = CONTENT_SECTION_META[section.key];
              return (
                <li key={section.key} className="min-w-0">
                  <button
                    type="button"
                    disabled={!hasInstance}
                    className={tileClass}
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
                      <div className={cn("p-2 rounded-xl shrink-0", meta.tone)}>
                        <meta.Icon className="w-5 h-5" />
                      </div>
                      <span className="truncate font-medium">{t(meta.labelKey)}</span>
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
                            className="text-xl font-bold tabular-nums"
                          >
                            {hasInstance ? counts[section.key] : "–"}
                          </motion.span>
                        )}
                      </AnimatePresence>
                      <FaChevronRight
                        aria-hidden="true"
                        size={10}
                        className="text-muted transition-transform group-hover/tile:translate-x-0.5"
                      />
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

/** Catalogs that install into the current instance. */
export const ContentMarketSection: React.FC = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();

  return (
    <Card className={LAYOUT.GLASS_CARD.BASE}>
      <Card.Content className="flex flex-col gap-4 p-4 sm:p-6">
        <SectionTitle
          icon={<FaCloudDownloadAlt size={16} />}
          tone="bg-orange-500/10 text-orange-600 dark:text-orange-400"
        >
          {t("shell.nav.market")}
        </SectionTitle>
        <ul aria-label={t("shell.nav.market")} className={LAYOUT.GRID.AUTO(14)}>
          {DISCOVER_CATALOGS.map((catalog) => {
            const meta = CATALOG_META[catalog.key];
            return (
              <li key={catalog.key} className="min-w-0">
                <button
                  type="button"
                  className={tileClass}
                  onClick={() => navigate(catalog.path)}
                >
                  <div className="flex min-w-0 items-center gap-3">
                    <div className="p-2 rounded-xl shrink-0 bg-surface-secondary">
                      <meta.Icon className={cn("w-5 h-5", meta.iconClassName)} />
                    </div>
                    <span className="truncate font-medium">{t(meta.labelKey)}</span>
                  </div>
                  <FaChevronRight
                    aria-hidden="true"
                    size={10}
                    className="shrink-0 text-muted transition-transform group-hover/tile:translate-x-0.5"
                  />
                </button>
              </li>
            );
          })}
        </ul>
      </Card.Content>
    </Card>
  );
};
