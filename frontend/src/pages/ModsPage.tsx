import { ModalDescription, ModalPanel, ModalAction, ModalProgress, ModalNotice, ModalDetails } from "@/components/ModalPrimitives";
import {
  Button,
  Card,
  Checkbox,
  Chip,
  Dropdown,
  FieldError,
  Input,
  InputGroup,
  Label, Spinner,
  Switch,
  Tabs,
  TextField,
  Tooltip
} from "@heroui/react";

import React, { useMemo, useRef } from "react";

import { DeleteConfirmModal } from "@/components/DeleteConfirmModal";
import { ImportResultModal } from "@/components/ImportResultModal";
import { UnifiedModal } from "@/components/UnifiedModal";
import { useTranslation } from "react-i18next";
import { Dialogs } from "@wailsio/runtime";
import {
  FaPuzzlePiece,
  FaTrash,
  FaEllipsisVertical,
  FaChevronUp,
  FaChevronDown,
  FaBan,
  FaCheck,
  FaPen,
} from "react-icons/fa6";
import {
  FaSync,
  FaFilter,
  FaTimes,
  FaFolderOpen,
  FaInfoCircle,
  FaBoxOpen,
} from "react-icons/fa";
import { LuDownload } from "react-icons/lu";
import { FiUploadCloud, FiAlertTriangle } from "react-icons/fi";
import { FileDropOverlay } from "@/components/FileDropOverlay";
import { useFileDrag } from "@/hooks/useFileDrag";
import {
  useModsPage,
  type LipGroupItem,
  type ModListItem,
} from "@/hooks/useModsPage";
import { PageHeader } from "@/components/PageHeader";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { cn } from "@/utils/cn";
import { COMPONENT_STYLES } from "@/constants/componentStyles";

const resolveLipChildrenSummary = (
  t: (key: string, opts?: Record<string, unknown>) => string,
  item: LipGroupItem,
): string => {
  if (!item.childPreview) {
    return item.childLabels.join(", ");
  }
  if (item.extraChildrenCount <= 0) {
    return item.childPreview;
  }
  return t("mods.lip_children_summary", {
    children: item.childPreview,
    count: item.extraChildrenCount,
  });
};

export const ModsPage: React.FC = () => {
  const { t } = useTranslation();
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const isDragActive = useFileDrag(scrollRef as React.RefObject<HTMLElement>);
  const mp = useModsPage(t as any, scrollRef);

  const visibleCount = mp.visibleItems.length;
  const allSelected = visibleCount > 0 && mp.selectedKeys.size === visibleCount;
  const indeterminate =
    mp.selectedKeys.size > 0 && mp.selectedKeys.size < visibleCount;

  const selectedCount = mp.selectedItems.length;

  const selectedCountLabel = useMemo(() => {
    return String(selectedCount);
  }, [selectedCount]);
  const listGridColumns =
    "grid-cols-[2rem_minmax(0,1fr)_auto] md:grid-cols-[2rem_minmax(0,1fr)_minmax(10rem,12rem)_minmax(7rem,auto)]";

  return (
    <PageContainer
      ref={scrollRef}
      id="mods-drop-zone"
      {...({ "data-file-drop-target": true } as any)}
      className="relative"
    >
      <FileDropOverlay isDragActive={isDragActive} text={t("mods.drop_hint")} />

      <UnifiedModal
        size="wide"
        isOpen={mp.importing && !mp.dllOpen}
        title={t("mods.importing_title")}
        type="primary"
        icon={<FiUploadCloud className="w-6 h-6" />}
        isDismissable={false}
        showConfirmButton={false}
      >
        <ModalProgress
          label={t("mods.importing_title")}
          description={<> {t("mods.importing_body")} </>}
          currentItem={mp.currentFile}
        />
      </UnifiedModal>

      <ImportResultModal
        isOpen={mp.errOpen}
        onOpenChange={mp.errOnOpenChange}
        results={{ success: mp.resultSuccess, failed: mp.resultFailed }}
        onConfirm={() => {
          mp.setErrorMsg("");
          mp.setErrorFile("");
          mp.setResultSuccess([]);
          mp.setResultFailed([]);
          mp.errOnClose();
        }}
      />

      <ImportResultModal
        isOpen={mp.delOpen}
        onOpenChange={mp.delOnOpenChange}
        results={{ success: mp.resultSuccess, failed: mp.resultFailed }}
        titleDone={t("mods.action_result_title_done")}
        titlePartial={t("mods.action_result_title_partial")}
        titleFailed={t("mods.action_result_title_failed")}
        onConfirm={() => {
          mp.setErrorMsg("");
          mp.setErrorFile("");
          mp.setResultSuccess([]);
          mp.setResultFailed([]);
          mp.delOnClose();
        }}
      />

      <UnifiedModal
        size="wide"
        isOpen={mp.dllOpen}
        onOpenChange={mp.dllOnOpenChange}
        title={t("mods.dll_modal_title")}
        type="primary"
        icon={<FaPuzzlePiece className="w-6 h-6" />}
        onConfirm={() => {
          const nm = mp.dllName.trim();
          if (!nm) return;
          const tp = (mp.dllType || "").trim() || "preload-native";
          const ver = (mp.dllVersion || "").trim() || "0.0.0";
          mp.dllConfirmRef.current = {
            name: nm,
            type: tp,
            version: ver,
          };
          try {
            mp.dllResolveRef.current && mp.dllResolveRef.current(true);
          } finally {
            mp.dllOnClose();
          }
        }}
        onCancel={() => {
          try {
            mp.dllConfirmRef.current = null;
            mp.dllResolveRef.current && mp.dllResolveRef.current(false);
          } finally {
            mp.dllOnClose();
          }
        }}
        showCancelButton
        confirmText={t("common.confirm")}
        cancelText={t("common.cancel")}
        confirmButtonProps={{ isDisabled: !mp.dllName.trim() }}
      >
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <TextField
            isRequired
            isInvalid={!mp.dllName.trim()}
            className={cn("group sm:col-span-2", COMPONENT_STYLES.input.mainWrapper)}
            value={mp.dllName}
            onChange={mp.setDllName}
          >
            <Label className={COMPONENT_STYLES.input.label}>
              {t("mods.dll_name") as string}
            </Label>
            <Input
              autoFocus
              className={cn(
                COMPONENT_STYLES.input.inputWrapper,
                COMPONENT_STYLES.input.input,
                "min-h-8 text-sm",
              )}
            />
            <FieldError>{t("audit.mods.name_required")}</FieldError>
          </TextField>
          <TextField
            className={cn("group", COMPONENT_STYLES.input.mainWrapper)}
            value={mp.dllType}
            onChange={mp.setDllType}
          >
            <Label className={COMPONENT_STYLES.input.label}>
              {t("mods.dll_type") as string}
            </Label>
            <Input
              className={cn(
                COMPONENT_STYLES.input.inputWrapper,
                COMPONENT_STYLES.input.input,
                "min-h-8 text-sm",
              )}
            />
          </TextField>
          <TextField
            className={cn("group", COMPONENT_STYLES.input.mainWrapper)}
            value={mp.dllVersion}
            onChange={mp.setDllVersion}
          >
            <Label className={COMPONENT_STYLES.input.label}>
              {t("mods.dll_version") as string}
            </Label>
            <Input
              className={cn(
                COMPONENT_STYLES.input.inputWrapper,
                COMPONENT_STYLES.input.input,
                "min-h-8 text-sm",
              )}
            />
          </TextField>
        </div>
      </UnifiedModal>

      <UnifiedModal
        isOpen={mp.dupOpen}
        onOpenChange={mp.dupOnOpenChange}
        title={t("mods.overwrite_modal_title")}
        type="warning"
        onConfirm={() => {
          try {
            mp.dupResolveRef.current && mp.dupResolveRef.current(true);
          } finally {
            mp.dupOnClose();
          }
        }}
        onCancel={() => {
          try {
            mp.dupResolveRef.current && mp.dupResolveRef.current(false);
          } finally {
            mp.dupOnClose();
          }
        }}
        showCancelButton
        confirmText={t("common.confirm")}
        cancelText={t("common.cancel")}
      >
        <ModalDescription>{t("mods.overwrite_modal_body")}</ModalDescription>
        {mp.dupNameRef.current ? <ModalPanel className="font-mono">{mp.dupNameRef.current}</ModalPanel> : null}
      </UnifiedModal>

      <Card className={cn("shrink-0", LAYOUT.GLASS_CARD.BASE)}>
        <Card.Content className="p-6 flex flex-col gap-6">
          <PageHeader
            title={t("moddedcard.title")}
            description={
              <div className="flex items-center gap-2">
                <span>{mp.currentVersionName || "No Version Selected"}</span>
                {mp.modsInfo.length > 0 && (
                  <Chip
                    size="sm"
                    variant="soft"
                    className={
                      "h-5 text-xs bg-surface-secondary "
                    }
                  >
                    <Chip.Label>{mp.modsInfo.length}</Chip.Label>
                  </Chip>
                )}
              </div>
            }
            endContent={
              <>
                <Button
                  onPress={async () => {
                    try {
                      const result = await Dialogs.OpenFile({
                        Filters: [
                          { DisplayName: "Mod Files", Pattern: "*.zip;*.dll" },
                        ],
                        AllowsMultipleSelection: true,
                        Title: t("mods.import_button"),
                      });
                      if (result && result.length > 0) {
                        void mp.doImportFromPaths(result);
                      }
                    } catch (e) {
                      console.error(e);
                    }
                  }}
                  isDisabled={mp.importing}
                  variant={"primary"}
                  className={cn(
                    "shadow-md",
                    "bg-brand-500 hover:bg-brand-500 brand-primary-foreground shadow-lg shadow-brand-900/20",
                  )}
                >
                  {<FiUploadCloud />}
                  {t("mods.import_button")}
                </Button>
                <Button
                  onPress={mp.openFolder}
                  variant={"secondary"}
                  className={"bg-surface-secondary "}
                >
                  {t("downloadmodal.open_folder")}
                </Button>
              </>
            }
          />
        </Card.Content>
      </Card>

      <div className="flex flex-col gap-3 px-2">
        <div className="flex flex-col sm:flex-row items-center gap-3">
          <TextField
            aria-label={t("common.search_placeholder")}
            className={cn(
              "group",
              COMPONENT_STYLES.input.mainWrapper,
              "w-full sm:max-w-xs",
            )}
            value={mp.query}
            onChange={mp.setQuery}
          >
            <InputGroup
              className={cn(
                COMPONENT_STYLES.input.inputWrapper,
                COMPONENT_STYLES.input.innerWrapper,
                "rounded-full",
              )}
            >
              <InputGroup.Prefix>
                {<FaFilter className="text-muted" />}
              </InputGroup.Prefix>
              <InputGroup.Input
                placeholder={t("common.search_placeholder")}
                className={COMPONENT_STYLES.input.input}
              />
              <InputGroup.Suffix>
                {mp.query && (
                  <button
                    type="button"
                    aria-label={t("audit.mods.clear_search")}
                    title={t("audit.mods.clear_search")}
                    onClick={() => mp.setQuery("")}
                  >
                      <FaTimes />
                  </button>
                )}
              </InputGroup.Suffix>
            </InputGroup>
          </TextField>
          <div className="w-px h-6 bg-surface-tertiary dark:bg-surface/10 hidden sm:block" />
          <Checkbox
            isSelected={mp.onlyEnabled}
            onChange={mp.setOnlyEnabled}
            className={cn("group", "m-0")}
          >
            <Checkbox.Content>
              <Checkbox.Control>
                <Checkbox.Indicator />
              </Checkbox.Control>
              <span className={"text-muted dark:text-zinc-400"}>
                {t("mods.only_enabled") as string}
              </span>
            </Checkbox.Content>
          </Checkbox>
        </div>

        <Tabs
          selectedKey={mp.tabKey}
          onSelectionChange={(key) => mp.setTabKey(key as any)}
          variant="primary"
        >
          <Tabs.ListContainer>
            <Tabs.List
              aria-label={"Mods Tabs"}
              className={COMPONENT_STYLES.tabs.tabList}
            >
              <Tabs.Tab
                key="all"
                id={"all"}
                className={COMPONENT_STYLES.tabs.tabContent}
              >
                {t("mods.tab_all") as string}
                <Tabs.Indicator className={COMPONENT_STYLES.tabs.cursor} />
              </Tabs.Tab>
              <Tabs.Tab
                key="normal"
                id={"normal"}
                className={COMPONENT_STYLES.tabs.tabContent}
              >
                {t("mods.tab_normal") as string}
                <Tabs.Indicator className={COMPONENT_STYLES.tabs.cursor} />
              </Tabs.Tab>
              <Tabs.Tab
                key="lip"
                id={"lip"}
                className={COMPONENT_STYLES.tabs.tabContent}
              >
                {t("mods.tab_lip") as string}
                <Tabs.Indicator className={COMPONENT_STYLES.tabs.cursor} />
              </Tabs.Tab>
            </Tabs.List>
          </Tabs.ListContainer>
        </Tabs>

        {(mp.lipInfoPending || mp.lipInfoWarning) && (
          <div
            className={cn(
              "rounded-2xl border px-4 py-3 flex items-start gap-3 launcher-material-blur",
              mp.lipInfoWarning
                ? "border-amber-200/60 bg-amber-50/70 dark:border-amber-900/30 dark:bg-amber-900/10"
                : "border-brand-200/60 bg-brand-50/70 dark:border-brand-900/30 dark:bg-brand-900/10",
            )}
          >
            <div className="pt-0.5 shrink-0">
              {mp.lipInfoWarning ? (
                <FiAlertTriangle className="text-amber-600 dark:text-amber-400" />
              ) : (
                <Spinner size="sm" color={"accent"} />
              )}
            </div>
            <div className="min-w-0">
              <p
                className={cn(
                  "text-sm font-medium",
                  mp.lipInfoWarning
                    ? "text-amber-700 dark:text-amber-300"
                    : "text-brand-700 dark:text-brand-300",
                )}
              >
                {mp.lipInfoWarning
                  ? t("mods.lip_sync_error")
                  : t("mods.lip_sync_loading")}
              </p>
              {mp.lipInfoWarning && (
                <p className="mt-1 text-xs text-amber-700/80 dark:text-amber-300/80 font-mono break-all">
                  {mp.lipInfoWarning}
                </p>
              )}
            </div>
          </div>
        )}
      </div>

      <div className="flex-1 overflow-hidden flex flex-col">
        <div className="px-4 py-2 text-sm text-muted dark:text-zinc-400 font-semibold">
          {selectedCount > 0 ? (
            <div className="flex items-center gap-3 animate-in fade-in slide-in-from-left-2 duration-200">
              <div className="w-8 flex justify-center">
                <Checkbox
                  aria-label={t("common.select_all")}
                  isSelected={allSelected}
                  isIndeterminate={indeterminate}
                  onChange={mp.onSelectAll}
                  className={"group"}
                >
                  <Checkbox.Content>
                    <Checkbox.Control className={"after:bg-brand-500"}>
                      <Checkbox.Indicator />
                    </Checkbox.Control>
                    <span></span>
                  </Checkbox.Content>
                </Checkbox>
              </div>
              <Chip size="sm" variant="soft" className={"h-8 px-3"}>
                <Chip.Label>{selectedCountLabel}</Chip.Label>
              </Chip>
              <Button
                size="sm"
                onPress={mp.openBatchUpdateConfirm}
                variant={"secondary"}
                className={
                  "bg-brand-500/10 text-brand-600 dark:text-brand-400 h-8 min-w-0 px-3"
                }
              >
                {<FaSync />}
                {t("mods.action_update")}
              </Button>
              <Button
                size="sm"
                onPress={mp.handleBatchEnable}
                variant={"secondary"}
                className={
                  "bg-surface-secondary h-8 min-w-0 px-3"
                }
              >
                {<FaCheck />}
                {t("common.enable")}
              </Button>
              <Button
                size="sm"
                onPress={mp.handleBatchDisable}
                variant={"secondary"}
                className={
                  "bg-surface-secondary h-8 min-w-0 px-3"
                }
              >
                {<FaBan />}
                {t("common.disable")}
              </Button>
              <Button
                size="sm"
                onPress={mp.openBatchUninstallConfirm}
                variant={"danger-soft"}
                className={"bg-rose-500/10 text-danger h-8 min-w-0 px-3"}
              >
                {<FaTrash />}
                {t("mods.action_uninstall")}
              </Button>
            </div>
          ) : (
            <div className={cn("grid items-center gap-x-3", listGridColumns)}>
              <div className="flex justify-center">
                <Checkbox
                  aria-label={t("common.select_all")}
                  isSelected={allSelected}
                  isIndeterminate={indeterminate}
                  onChange={mp.onSelectAll}
                  className={"group"}
                >
                  <Checkbox.Content>
                    <Checkbox.Control className={"after:bg-brand-500"}>
                      <Checkbox.Indicator />
                    </Checkbox.Control>
                    <span></span>
                  </Checkbox.Content>
                </Checkbox>
              </div>
              <button
                type="button"
                className="min-w-0 cursor-pointer flex items-center gap-1 hover:text-foreground dark:hover:text-zinc-300 transition-colors select-none text-left"
                onClick={() => mp.handleSort("name")}
              >
                {t("mods.field_name")}{" "}
                {mp.sortConfig.key === "name" &&
                  (mp.sortConfig.direction === "asc" ? (
                    <FaChevronUp className="text-xs" />
                  ) : (
                    <FaChevronDown className="text-xs" />
                  ))}
              </button>
              <div className="hidden md:block">{t("common.version")}</div>
              <div className="col-start-3 md:col-start-4 justify-self-end">
                <button
                  className="flex items-center gap-1.5 text-xs hover:text-foreground dark:hover:text-zinc-300 transition-colors"
                  onClick={() => mp.refreshAll()}
                  disabled={mp.loading}
                >
                  <FaSync className={mp.loading ? "animate-spin" : ""} />
                  {t("common.refresh")}
                </button>
              </div>
            </div>
          )}
        </div>

        <div className="flex-1 overflow-y-auto p-2 custom-scrollbar flex flex-col gap-2">
          {!mp.currentVersionName ? (
            <div className="flex flex-col items-center justify-center h-full text-muted gap-2">
              <FiAlertTriangle className="w-8 h-8 opacity-50" />
              <p>{t("launcherpage.currentVersion_none")}</p>
            </div>
          ) : mp.listHydrating ? (
            <div className="flex flex-col items-center justify-center h-full text-muted gap-3">
              <Spinner size="lg" color={"accent"} />
              <p>{t("common.loading")}</p>
            </div>
          ) : mp.visibleItems.length === 0 ? (
            <div role="status" className="flex flex-col items-center justify-center h-full text-muted gap-2">
              <FaPuzzlePiece className="w-8 h-8 opacity-50" />
              <p>
                {t(
                  mp.hasInstalledItems
                    ? "audit.mods.no_filtered_results"
                    : "moddedcard.content.none",
                )}
              </p>
              {mp.hasInstalledItems && (
                <Button
                  variant="secondary"
                  onPress={() => {
                    mp.setQuery("");
                    mp.setOnlyEnabled(false);
                    mp.setTabKey("all");
                  }}
                >
                  {t("audit.mods.clear_filters")}
                </Button>
              )}
            </div>
          ) : (
            mp.visibleItems.map((item, idx) => {
              if (item.kind === "mod") {
                const modItem = item as ModListItem;
                const mod = modItem.mod;
                const folder = modItem.folder;
                const lipState = modItem.lipState;

                return (
                  <div
                    key={item.key || `${mod.name}-${mod.version}-${idx}`}
                    className={cn(
                      "grid rounded-2xl border transition-all p-3 bg-surface/60 dark:bg-surface-secondary/40 hover:bg-surface/80 dark:hover:bg-surface-secondary/80",
                      listGridColumns,
                      "grid-rows-[auto_auto] md:grid-rows-1 gap-x-3 gap-y-2",
                      mp.selectedKeys.has(item.key)
                        ? "border-brand-500/50 dark:border-brand-500/30 bg-brand-50/50 dark:bg-brand-900/10"
                        : "border-white/40 dark:border-white/5",
                    )}
                  >
                    <div
                      className="row-span-2 md:row-span-1 flex justify-center pt-1 md:pt-0"
                      onClick={(e) => e.stopPropagation()}
                    >
                      <Checkbox
                        aria-label={`${t("common.select_mode")}: ${item.key}`}
                        isSelected={mp.selectedKeys.has(item.key)}
                        onChange={() => mp.onSelectionChange(item.key)}
                        className={"group"}
                      >
                        <Checkbox.Content>
                          <Checkbox.Control>
                            <Checkbox.Indicator />
                          </Checkbox.Control>
                          <span></span>
                        </Checkbox.Content>
                      </Checkbox>
                    </div>

                    <div className="col-start-2 row-span-2 md:row-span-1 min-w-0 flex items-start gap-3">
                      <div className="w-11 h-11 rounded-xl bg-surface-secondary dark:bg-surface flex items-center justify-center text-muted dark:text-zinc-400 shrink-0">
                        <FaPuzzlePiece className="w-5 h-5" />
                      </div>
                      <div className="min-w-0 flex flex-col justify-center">
                        <div className="font-bold text-foreground dark:text-zinc-100 truncate text-base">
                          {mod.name}
                        </div>
                        <div className="text-xs text-muted dark:text-zinc-400 truncate">
                          by {mod.author || "Unknown"}
                        </div>
                        <div className="text-xs text-muted dark:text-zinc-400 truncate font-mono opacity-80 mt-0.5">
                          {mod.entry || mod.type}
                        </div>
                      </div>
                    </div>

                    <div className="col-start-3 row-start-2 md:col-start-3 md:row-start-1 min-w-0 flex flex-col justify-center items-start">
                      <div className="text-foreground dark:text-zinc-300 truncate text-sm">
                        {mod.version || "-"}
                      </div>
                      {lipState.sourceType === "unique" ? (
                        <div className="mt-1">
                          {lipState.canUpdate ? (
                            <Chip size="sm" variant="soft" color={"success"}>
                              <Chip.Label>
                                {t("mods.update_available", {
                                  version: lipState.targetVersion,
                                })}
                              </Chip.Label>
                            </Chip>
                          ) : (
                            <Chip size="sm" variant="soft">
                              <Chip.Label>{t("mods.update_latest")}</Chip.Label>
                            </Chip>
                          )}
                        </div>
                      ) : null}
                    </div>

                    <div
                      className="col-start-3 row-start-1 md:col-start-4 md:row-start-1 flex items-center justify-end gap-2 pr-1"
                      onClick={(e) => e.stopPropagation()}
                    >
                      {lipState.sourceType === "unique" &&
                      lipState.canUpdate ? (
                        <Button
                          isIconOnly
                          size="sm"
                          onPress={() => void mp.handleUpdateMod(mod)}
                          aria-label={t("mods.action_update") as string}
                          variant={"ghost"}
                          className={"text-brand-600 dark:text-brand-400"}
                        >
                          <LuDownload />
                        </Button>
                      ) : null}
                      <Switch
                        size="sm"
                        isSelected={!!mp.enabledByFolder.get(folder)}
                        aria-label={t("mods.toggle_label") as string}
                        onChange={(value: boolean) => {
                          mp.toggleModEnabled(folder, value);
                        }}
                        className={"group"}
                      >
                        <Switch.Content>
                          <Switch.Control
                            className={"group-data-[selected]:bg-brand-500"}
                          >
                            <Switch.Thumb></Switch.Thumb>
                          </Switch.Control>
                          <span></span>
                        </Switch.Content>
                      </Switch>

                      <Dropdown>
                        <Tooltip>
                          <Button
                            isIconOnly
                            size="sm"
                            variant={"ghost"}
                            className={"text-muted"}
                            aria-label={t("audit.mods.more_actions", {
                              name: mod.name,
                            })}
                          >
                            <FaEllipsisVertical />
                          </Button>
                          <Tooltip.Content>
                            {t("audit.mods.more_actions", { name: mod.name })}
                          </Tooltip.Content>
                        </Tooltip>
                        <Dropdown.Popover
                          className={COMPONENT_STYLES.dropdown.content}
                        >
                          <Dropdown.Menu aria-label="mod actions">
                            <Dropdown.Item
                              key="uninstall"
                              id={"uninstall"}
                              textValue={t("mods.action_uninstall")}
                              onAction={() => mp.openDeleteForMod(mod)}
                              variant="danger"
                            >
                              {<FaTrash />}
                              <Label>{t("mods.action_uninstall")}</Label>
                              <Dropdown.ItemIndicator />
                            </Dropdown.Item>
                            <Dropdown.Item
                              key="update"
                              isDisabled={
                                lipState.sourceType !== "unique" ||
                                !lipState.canUpdate
                              }
                              id={"update"}
                              textValue={t("mods.action_update")}
                              onAction={() => void mp.handleUpdateMod(mod)}
                            >
                              {<FaSync />}
                              <Label>{t("mods.action_update")}</Label>
                              <Dropdown.ItemIndicator />
                            </Dropdown.Item>
                            <Dropdown.Item
                              key="edit"
                              id={"edit"}
                              textValue={t("common.edit")}
                              onAction={() => mp.openEditForMod(mod)}
                            >
                              {<FaPen />}
                              <Label>{t("common.edit")}</Label>
                              <Dropdown.ItemIndicator />
                            </Dropdown.Item>
                            <Dropdown.Item
                              key="folder"
                              id={"folder"}
                              textValue={t("common.open_folder")}
                              onAction={() => mp.openModFolder(mod)}
                            >
                              {<FaFolderOpen />}
                              <Label>{t("common.open_folder")}</Label>
                              <Dropdown.ItemIndicator />
                            </Dropdown.Item>
                            <Dropdown.Item
                              key="details"
                              id={"details"}
                              textValue={t("common.details")}
                              onAction={() => mp.openDetails(mod)}
                            >
                              {<FaInfoCircle />}
                              <Label>{t("common.details")}</Label>
                              <Dropdown.ItemIndicator />
                            </Dropdown.Item>
                          </Dropdown.Menu>
                        </Dropdown.Popover>
                      </Dropdown>
                    </div>
                  </div>
                );
              }

              const lipItem = item as LipGroupItem;
              const lipState = lipItem.lipState;
              const childrenSummary = resolveLipChildrenSummary(
                t as any,
                lipItem,
              );

              return (
                <div
                  key={item.key || `${lipItem.identifier}-${idx}`}
                  className={cn(
                    "grid rounded-2xl border transition-all p-3 bg-surface/60 dark:bg-surface-secondary/40 hover:bg-surface/80 dark:hover:bg-surface-secondary/80",
                    listGridColumns,
                    "grid-rows-[auto_auto] md:grid-rows-1 gap-x-3 gap-y-2",
                    mp.selectedKeys.has(item.key)
                      ? "border-brand-500/50 dark:border-brand-500/30 bg-brand-50/50 dark:bg-brand-900/10"
                      : "border-white/40 dark:border-white/5",
                  )}
                >
                  <div
                    className="row-span-2 md:row-span-1 flex justify-center pt-1 md:pt-0"
                    onClick={(e) => e.stopPropagation()}
                  >
                    <Checkbox
                      aria-label={`${t("common.select_mode")}: ${item.key}`}
                      isSelected={mp.selectedKeys.has(item.key)}
                      onChange={() => mp.onSelectionChange(item.key)}
                      className={"group"}
                    >
                      <Checkbox.Content>
                        <Checkbox.Control>
                          <Checkbox.Indicator />
                        </Checkbox.Control>
                        <span></span>
                      </Checkbox.Content>
                    </Checkbox>
                  </div>

                  <div className="col-start-2 row-span-2 md:row-span-1 min-w-0 flex items-start gap-3">
                    <div className="w-11 h-11 rounded-xl bg-surface-secondary dark:bg-surface flex items-center justify-center text-muted dark:text-zinc-400 shrink-0">
                      <FaBoxOpen className="w-5 h-5" />
                    </div>
                    <div className="min-w-0 flex flex-col justify-center">
                      <div className="font-bold text-foreground dark:text-zinc-100 truncate text-base">
                        {lipItem.packageName}
                      </div>
                      <div className="text-xs text-muted dark:text-zinc-400 truncate font-mono">
                        {lipItem.displayIdentifier}
                      </div>
                      <div className="text-xs text-muted dark:text-zinc-400 truncate mt-0.5">
                        {childrenSummary}
                      </div>
                    </div>
                  </div>

                  <div className="col-start-3 row-start-2 md:col-start-3 md:row-start-1 min-w-0 flex flex-col justify-center items-start">
                    <div className="text-foreground dark:text-zinc-300 truncate text-sm">
                      {lipItem.installedVersion || "-"}
                    </div>
                    <div className="mt-1">
                      {lipState.canUpdate ? (
                        <Chip size="sm" variant="soft" color={"success"}>
                          <Chip.Label>
                            {t("mods.update_available", {
                              version: lipState.targetVersion,
                            })}
                          </Chip.Label>
                        </Chip>
                      ) : (
                        <Chip size="sm" variant="soft">
                          <Chip.Label>{t("mods.update_latest")}</Chip.Label>
                        </Chip>
                      )}
                    </div>
                  </div>

                  <div
                    className="col-start-3 row-start-1 md:col-start-4 md:row-start-1 flex items-center justify-end gap-2 pr-1"
                    onClick={(e) => e.stopPropagation()}
                  >
                    {lipState.canUpdate ? (
                      <Button
                        isIconOnly
                        size="sm"
                        onPress={() => void mp.handleUpdateLipGroup(lipItem)}
                        aria-label={t("mods.action_update") as string}
                        variant={"ghost"}
                        className={"text-brand-600 dark:text-brand-400"}
                      >
                        <LuDownload />
                      </Button>
                    ) : null}
                    <Switch
                      size="sm"
                      isSelected={lipItem.allEnabled}
                      aria-label={t("mods.toggle_label") as string}
                      onChange={(value: boolean) => {
                        void mp.toggleLipGroupEnabled(lipItem, value);
                      }}
                      className={"group"}
                    >
                      <Switch.Content>
                        <Switch.Control
                          className={"group-data-[selected]:bg-brand-500"}
                        >
                          <Switch.Thumb></Switch.Thumb>
                        </Switch.Control>
                        <span></span>
                      </Switch.Content>
                    </Switch>

                    <Dropdown>
                      <Tooltip>
                        <Button
                          isIconOnly
                          size="sm"
                          variant={"ghost"}
                          className={"text-muted"}
                          aria-label={t("audit.mods.more_actions", {
                            name: lipItem.packageName,
                          })}
                        >
                          <FaEllipsisVertical />
                        </Button>
                        <Tooltip.Content>
                          {t("audit.mods.more_actions", {
                            name: lipItem.packageName,
                          })}
                        </Tooltip.Content>
                      </Tooltip>
                      <Dropdown.Popover
                        className={COMPONENT_STYLES.dropdown.content}
                      >
                        <Dropdown.Menu aria-label="lip actions">
                          <Dropdown.Item
                            key="uninstall"
                            id={"uninstall"}
                            textValue={t("mods.action_uninstall")}
                            onAction={() =>
                              void mp.openDeleteForLipGroup(lipItem)
                            }
                            variant="danger"
                          >
                            {<FaTrash />}
                            <Label>{t("mods.action_uninstall")}</Label>
                            <Dropdown.ItemIndicator />
                          </Dropdown.Item>
                          {mp.isLipGroupPromotable(lipItem) ? (
                            <Dropdown.Item
                              key="promote"
                              id={"promote"}
                              textValue={t("mods.action_promote_install")}
                              onAction={() =>
                                void mp.handlePromoteLipGroup(lipItem)
                              }
                            >
                              {<LuDownload />}
                              <Label>{t("mods.action_promote_install")}</Label>
                              <Dropdown.ItemIndicator />
                            </Dropdown.Item>
                          ) : null}
                          <Dropdown.Item
                            key="update"
                            isDisabled={!lipState.canUpdate}
                            id={"update"}
                            textValue={t("mods.action_update")}
                            onAction={() =>
                              void mp.handleUpdateLipGroup(lipItem)
                            }
                          >
                            {<FaSync />}
                            <Label>{t("mods.action_update")}</Label>
                            <Dropdown.ItemIndicator />
                          </Dropdown.Item>
                          <Dropdown.Item
                            key="openLip"
                            id={"openLip"}
                            textValue={t("mods.lip_open_package")}
                            onAction={() =>
                              mp.openLIPPackageDetails(
                                lipItem.displayIdentifier || lipItem.identifier,
                              )
                            }
                          >
                            {<FaInfoCircle />}
                            <Label>{t("mods.lip_open_package")}</Label>
                            <Dropdown.ItemIndicator />
                          </Dropdown.Item>
                        </Dropdown.Menu>
                      </Dropdown.Popover>
                    </Dropdown>
                  </div>
                </div>
              );
            })
          )}
        </div>
      </div>

      <UnifiedModal
        size="wide"
        isOpen={mp.infoOpen}
        onOpenChange={mp.infoOnOpenChange}
        title={t("mods.details_title")}
        type="primary"
        icon={<FaPuzzlePiece className="w-6 h-6" />}
        showConfirmButton={false}
        showCancelButton
        cancelText={t("common.close")}
        onCancel={() => mp.infoOnClose()}
        footer={
          <>
            <ModalAction onPress={() => mp.infoOnClose()} variant="secondary">
              {t("common.cancel")}
            </ModalAction>
            <ModalAction
              onPress={() => {
                if (!mp.activeMod) return;
                mp.openEditForMod(mp.activeMod);
              }}
              isDisabled={!mp.activeMod}
              variant={"secondary"}
            >
              {t("common.edit")}
            </ModalAction>
            <ModalAction
              onPress={() => {
                if (!mp.activeMod) return;
                void mp.handleUpdateMod(mp.activeMod);
              }}
              isDisabled={
                !mp.activeMod ||
                mp.getModLIPState(mp.activeMod).sourceType !== "unique" ||
                !mp.getModLIPState(mp.activeMod).canUpdate
              }
              variant={"secondary"}
            >
              {t("mods.action_update")}
            </ModalAction>
            <ModalAction onPress={mp.delCfmOnOpen} variant={"danger"}>
              {t("mods.action_uninstall")}
            </ModalAction>
          </>
        }
      >
        {mp.activeMod ? (
          <div className="space-y-2 text-sm dark:text-zinc-200">
            <ModalDetails items={[
              { label: t("mods.field_name"), value: mp.activeMod.name, fullWidth: true },
              { label: t("mods.field_version"), value: mp.activeMod.version || "-", mono: true },
              { label: t("mods.field_type"), value: mp.activeMod.type || "-" },
              { label: t("mods.field_entry"), value: mp.activeMod.entry || "-", mono: true, fullWidth: true },
              ...(mp.activeMod.author ? [{ label: t("mods.field_author"), value: mp.activeMod.author }] : []),
              { label: t("mods.action_update"), value: (() => {
                const state = mp.getModLIPState(mp.activeMod);
                if (state.sourceType !== "unique") return t("mods.no_update_source");
                if (!state.canUpdate) return t("mods.update_latest");
                return t("mods.update_available", { version: state.targetVersion });
              })() },
            ]} />
            <div className="pt-2">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="text-muted dark:text-zinc-400">
                    {t("mods.toggle_label")}
                  </span>
                  <Chip
                    size="sm"
                    variant="soft"
                    color={
                      mp.enabledByFolder.get(mp.resolveModFolder(mp.activeMod!))
                        ? "success"
                        : "warning"
                    }
                  >
                    <Chip.Label>
                      {mp.enabledByFolder.get(
                        mp.resolveModFolder(mp.activeMod!),
                      )
                        ? (t("mods.toggle_on") as string)
                        : (t("mods.toggle_off") as string)}
                    </Chip.Label>
                  </Chip>
                </div>
                <Switch
                  isSelected={
                    !!mp.enabledByFolder.get(mp.resolveModFolder(mp.activeMod!))
                  }
                  aria-label={t("mods.toggle_label") as string}
                  onChange={(value: boolean) =>
                    mp.toggleModEnabled(
                      mp.resolveModFolder(mp.activeMod!),
                      value,
                    )
                  }
                  className={"group"}
                >
                  <Switch.Content>
                    <Switch.Control
                      className={"group-data-[selected]:bg-brand-500"}
                    >
                      <Switch.Thumb></Switch.Thumb>
                    </Switch.Control>
                    <span></span>
                  </Switch.Content>
                </Switch>
              </div>
              <div className="text-muted dark:text-zinc-400 text-xs mt-1">
                {mp.enabledByFolder.get(mp.resolveModFolder(mp.activeMod!))
                  ? (t("mods.toggle_desc_on") as string)
                  : (t("mods.toggle_desc_off") as string)}
              </div>
            </div>
          </div>
        ) : null}
      </UnifiedModal>

      <UnifiedModal
        size="wide"
        isOpen={mp.editOpen}
        onOpenChange={mp.editOnOpenChange}
        title={t("common.edit")}
        type="primary"
        icon={<FaPen className="w-5 h-5" />}

        showCancelButton
        confirmText={t("common.save")}
        cancelText={t("common.cancel")}
        onCancel={mp.editOnClose}
        onConfirm={() => void mp.handleSaveModEdit()}
        confirmButtonProps={{
          isPending: mp.editSaving,
          isDisabled: !mp.editFormValid,
        }}
        cancelButtonProps={{ isDisabled: mp.editSaving }}
      >
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <TextField
            isRequired={true}
            isInvalid={mp.editName.trim().length === 0}
            className={cn("group sm:col-span-2", COMPONENT_STYLES.input.mainWrapper)}
            value={mp.editName}
            onChange={mp.setEditName}
          >
            <Label className={COMPONENT_STYLES.input.label}>
              {t("mods.field_name") as string}
            </Label>
            <Input
              autoFocus
              className={cn(
                COMPONENT_STYLES.input.inputWrapper,
                COMPONENT_STYLES.input.input,
                "min-h-8 text-sm",
              )}
            />
          </TextField>
          <TextField
            className={cn("group", COMPONENT_STYLES.input.mainWrapper)}
            value={mp.editVersion}
            onChange={mp.setEditVersion}
          >
            <Label className={COMPONENT_STYLES.input.label}>
              {t("mods.field_version") as string}
            </Label>
            <Input
              className={cn(
                COMPONENT_STYLES.input.inputWrapper,
                COMPONENT_STYLES.input.input,
                "min-h-8 text-sm",
              )}
            />
          </TextField>
          <TextField
            className={cn("group", COMPONENT_STYLES.input.mainWrapper)}
            value={mp.editType}
            onChange={mp.setEditType}
          >
            <Label className={COMPONENT_STYLES.input.label}>
              {t("mods.field_type") as string}
            </Label>
            <Input
              className={cn(
                COMPONENT_STYLES.input.inputWrapper,
                COMPONENT_STYLES.input.input,
                "min-h-8 text-sm",
              )}
            />
          </TextField>
          <TextField
            className={cn("group sm:col-span-2", COMPONENT_STYLES.input.mainWrapper)}
            value={mp.editEntry}
            onChange={mp.setEditEntry}
          >
            <Label className={COMPONENT_STYLES.input.label}>
              {t("mods.field_entry") as string}
            </Label>
            <Input
              className={cn(
                COMPONENT_STYLES.input.inputWrapper,
                COMPONENT_STYLES.input.input,
                "min-h-8 text-sm",
              )}
            />
          </TextField>
          <TextField
            className={cn("group", COMPONENT_STYLES.input.mainWrapper)}
            value={mp.editAuthor}
            onChange={mp.setEditAuthor}
          >
            <Label className={COMPONENT_STYLES.input.label}>
              {t("mods.field_author") as string}
            </Label>
            <Input
              className={cn(
                COMPONENT_STYLES.input.inputWrapper,
                COMPONENT_STYLES.input.input,
                "min-h-8 text-sm",
              )}
            />
          </TextField>
        </div>
      </UnifiedModal>

      <DeleteConfirmModal
        isOpen={mp.delCfmOpen}
        onOpenChange={mp.delCfmOnOpenChange}
        onConfirm={mp.handleDeleteCurrentTarget}
        title={t("mods.action_uninstall")}
        description={t("mods.confirm_delete_body", {
          type: t("mods.action_uninstall"),
        })}
        itemName={mp.activeDeleteName}
        isPending={mp.deleting}
        confirmDisabled={mp.activeDeleteBlocked}
        warning={mp.activeDeleteWarning}
      />

      <UnifiedModal
        isOpen={mp.demotedWarningOpen}
        onOpenChange={mp.demotedWarningOnOpenChange}
        title={t("common.tip")}
        type="warning"
        confirmText={t("common.confirm")}
        showCancelButton={false}
        onConfirm={mp.closeDemotedWarning}
      >
        <ModalDescription className="whitespace-pre-wrap">
          {t("errors.ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY")}
        </ModalDescription>
        {mp.demotedWarningNames.length > 0 ? (
          <ModalNotice tone="warning" className="whitespace-pre-wrap break-all font-mono">
            {mp.demotedWarningNames.join("\n")}
          </ModalNotice>
        ) : null}
      </UnifiedModal>

      <UnifiedModal
        isOpen={mp.actionConfirmOpen}
        onOpenChange={mp.actionConfirmOnOpenChange}
        title={mp.actionConfirmTitle}
        type="primary"
        showCancelButton
        confirmText={t("common.confirm")}
        cancelText={t("common.cancel")}
        onCancel={mp.actionConfirmOnClose}
        onConfirm={() => void mp.confirmPendingAction()}
        confirmButtonProps={{ isPending: mp.actionConfirming }}
        cancelButtonProps={{ isDisabled: mp.actionConfirming }}
      >
        <ModalDescription className="whitespace-pre-wrap">
          {mp.actionConfirmBody}
        </ModalDescription>
      </UnifiedModal>

      <UnifiedModal
        isOpen={mp.batchUpdateOpen}
        onOpenChange={mp.batchUpdateOnOpenChange}
        title={t("mods.batch_update_title")}
        type="primary"
        showCancelButton
        confirmText={t("common.confirm")}
        cancelText={t("common.cancel")}
        onCancel={mp.batchUpdateOnClose}
        onConfirm={() => void mp.handleBatchUpdate()}
        confirmButtonProps={{ isPending: mp.batchUpdating }}
        cancelButtonProps={{ isDisabled: mp.batchUpdating }}
      >
        <ModalDescription className="whitespace-pre-wrap">
          {t("mods.batch_update_body", {
            selected: mp.selectedItems.length,
            updatable: mp.selectedUpdatableCount,
          })}
        </ModalDescription>
      </UnifiedModal>

      <UnifiedModal
        isOpen={mp.batchUninstallOpen}
        onOpenChange={mp.batchUninstallOnOpenChange}
        title={t("mods.batch_uninstall_title")}
        type="error"
        showCancelButton
        confirmText={t("common.confirm")}
        cancelText={t("common.cancel")}
        onCancel={mp.batchUninstallOnClose}
        onConfirm={() => void mp.handleBatchUninstall()}
        confirmButtonProps={{ isPending: mp.batchUninstalling, variant: "danger" }}
        cancelButtonProps={{ isDisabled: mp.batchUninstalling }}
      >
        <ModalDescription className="whitespace-pre-wrap">
          {t("mods.batch_uninstall_body", {
            count: mp.selectedItems.length,
          })}
        </ModalDescription>
      </UnifiedModal>
    </PageContainer>
  );
};

export default ModsPage;
