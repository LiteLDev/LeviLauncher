import React, { useMemo, useRef } from "react";
import {
  Button,
  Card,
  CardBody,
  Chip,
  Input,
  Progress,
  Spinner,
  Switch,
  Checkbox,
  Dropdown,
  DropdownTrigger,
  DropdownMenu,
  DropdownItem,
  Tabs,
  Tab,
} from "@heroui/react";
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
        isOpen={mp.importing && !mp.dllOpen}
        title={t("mods.importing_title")}
        type="primary"
        icon={<FiUploadCloud className="w-6 h-6" />}
        hideCloseButton
        isDismissable={false}
        showConfirmButton={false}
      >
        <div className="py-1">
          <Progress isIndeterminate aria-label="importing" className="w-full" />
        </div>
        <div className="text-default-600 dark:text-zinc-300 text-sm mt-2">
          {t("mods.importing_body")}
        </div>
        {mp.currentFile ? (
          <div className="mt-2 rounded-md bg-default-100/60 dark:bg-zinc-800/60 border border-default-200 dark:border-zinc-700 px-3 py-2 text-default-800 dark:text-zinc-100 text-sm wrap-break-word whitespace-pre-wrap font-mono">
            {mp.currentFile}
          </div>
        ) : null}
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
        isOpen={mp.dllOpen}
        onOpenChange={mp.dllOnOpenChange}
        title={t("mods.dll_modal_title")}
        type="primary"
        icon={<FaPuzzlePiece className="w-6 h-6" />}
        hideCloseButton
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
      >
        <div className="flex flex-col gap-3">
          <Input
            label={t("mods.dll_name") as string}
            value={mp.dllName}
            onValueChange={mp.setDllName}
            autoFocus
            size="sm"
            classNames={COMPONENT_STYLES.input}
          />
          <Input
            label={t("mods.dll_type") as string}
            value={mp.dllType}
            onValueChange={mp.setDllType}
            size="sm"
            classNames={COMPONENT_STYLES.input}
          />
          <Input
            label={t("mods.dll_version") as string}
            value={mp.dllVersion}
            onValueChange={mp.setDllVersion}
            size="sm"
            classNames={COMPONENT_STYLES.input}
          />
        </div>
      </UnifiedModal>

      <UnifiedModal
        isOpen={mp.dupOpen}
        onOpenChange={mp.dupOnOpenChange}
        title={t("mods.overwrite_modal_title")}
        type="warning"
        hideCloseButton
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
        <div className="text-sm text-default-700 dark:text-zinc-300">
          {t("mods.overwrite_modal_body")}
        </div>
        {mp.dupNameRef.current ? (
          <div className="mt-2 rounded-md bg-default-100/60 dark:bg-zinc-800/60 border border-default-200 dark:border-zinc-700 px-3 py-2 text-default-800 dark:text-zinc-100 text-sm wrap-break-word whitespace-pre-wrap">
            {mp.dupNameRef.current}
          </div>
        ) : null}
      </UnifiedModal>

      <Card className={cn("shrink-0", LAYOUT.GLASS_CARD.BASE)}>
        <CardBody className="p-6 flex flex-col gap-6">
          <PageHeader
            title={t("moddedcard.title")}
            description={
              <div className="flex items-center gap-2">
                <span>{mp.currentVersionName || "No Version Selected"}</span>
                {mp.modsInfo.length > 0 && (
                  <Chip
                    size="sm"
                    variant="flat"
                    className="h-5 text-xs bg-default-100 dark:bg-zinc-800"
                  >
                    {mp.modsInfo.length}
                  </Chip>
                )}
              </div>
            }
            endContent={
              <>
                <Button
                  color="primary"
                  variant="shadow"
                  className="bg-primary-500 hover:bg-primary-500 text-white shadow-lg shadow-primary-900/20"
                  startContent={<FiUploadCloud />}
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
                >
                  {t("mods.import_button")}
                </Button>
                <Button
                  variant="flat"
                  className="bg-default-100 dark:bg-zinc-800"
                  onPress={mp.openFolder}
                >
                  {t("downloadmodal.open_folder")}
                </Button>
              </>
            }
          />
        </CardBody>
      </Card>

      <div className="flex flex-col gap-3 px-2">
        <div className="flex flex-col sm:flex-row items-center gap-3">
          <Input
            placeholder={t("common.search_placeholder")}
            value={mp.query}
            onValueChange={mp.setQuery}
            startContent={<FaFilter className="text-default-400" />}
            endContent={
              mp.query && (
                <button onClick={() => mp.setQuery("")}>
                  <FaTimes className="text-default-400 hover:text-default-600" />
                </button>
              )
            }
            radius="full"
            variant="flat"
            className="w-full sm:max-w-xs"
            classNames={COMPONENT_STYLES.input}
          />
          <div className="w-px h-6 bg-default-200 dark:bg-white/10 hidden sm:block" />
          <Checkbox
            size="sm"
            isSelected={mp.onlyEnabled}
            onValueChange={mp.setOnlyEnabled}
            classNames={{
              base: "m-0",
              label: "text-default-500 dark:text-zinc-400",
            }}
          >
            {t("mods.only_enabled") as string}
          </Checkbox>
        </div>

        <Tabs
          aria-label="Mods Tabs"
          selectedKey={mp.tabKey}
          onSelectionChange={(key) => mp.setTabKey(key as any)}
          classNames={COMPONENT_STYLES.tabs}
        >
          <Tab key="all" title={t("mods.tab_all") as string} />
          <Tab key="normal" title={t("mods.tab_normal") as string} />
          <Tab key="lip" title={t("mods.tab_lip") as string} />
        </Tabs>

        {(mp.lipInfoPending || mp.lipInfoWarning) && (
          <div
            className={cn(
              "rounded-2xl border px-4 py-3 flex items-start gap-3 backdrop-blur-md",
              mp.lipInfoWarning
                ? "border-warning-200/60 bg-warning-50/70 dark:border-warning-900/30 dark:bg-warning-900/10"
                : "border-primary-200/60 bg-primary-50/70 dark:border-primary-900/30 dark:bg-primary-900/10",
            )}
          >
            <div className="pt-0.5 shrink-0">
              {mp.lipInfoWarning ? (
                <FiAlertTriangle className="text-warning-600 dark:text-warning-400" />
              ) : (
                <Spinner size="sm" color="primary" />
              )}
            </div>
            <div className="min-w-0">
              <p
                className={cn(
                  "text-sm font-medium",
                  mp.lipInfoWarning
                    ? "text-warning-700 dark:text-warning-300"
                    : "text-primary-700 dark:text-primary-300",
                )}
              >
                {mp.lipInfoWarning
                  ? t("mods.lip_sync_error")
                  : t("mods.lip_sync_loading")}
              </p>
              {mp.lipInfoWarning && (
                <p className="mt-1 text-xs text-warning-700/80 dark:text-warning-300/80 font-mono break-all">
                  {mp.lipInfoWarning}
                </p>
              )}
            </div>
          </div>
        )}
      </div>

      <div className="flex-1 overflow-hidden flex flex-col">
        <div className="px-4 py-2 text-sm text-default-500 dark:text-zinc-400 font-semibold">
          {selectedCount > 0 ? (
            <div className="flex items-center gap-3 animate-in fade-in slide-in-from-left-2 duration-200">
              <div className="w-8 flex justify-center">
                <Checkbox
                  size="sm"
                  classNames={{ wrapper: "after:bg-primary-500" }}
                  isSelected={allSelected}
                  isIndeterminate={indeterminate}
                  onValueChange={mp.onSelectAll}
                />
              </div>
              <Chip size="sm" variant="flat" className="h-8 px-3">
                {selectedCountLabel}
              </Chip>
              <Button
                size="sm"
                color="primary"
                variant="flat"
                className="bg-primary-500/10 text-primary-600 dark:text-primary-400 h-8 min-w-0 px-3"
                startContent={<FaSync />}
                onPress={mp.openBatchUpdateConfirm}
              >
                {t("mods.action_update")}
              </Button>
              <Button
                size="sm"
                variant="flat"
                className="bg-default-100 dark:bg-zinc-800 h-8 min-w-0 px-3"
                startContent={<FaCheck />}
                onPress={mp.handleBatchEnable}
              >
                {t("common.enable")}
              </Button>
              <Button
                size="sm"
                variant="flat"
                className="bg-default-100 dark:bg-zinc-800 h-8 min-w-0 px-3"
                startContent={<FaBan />}
                onPress={mp.handleBatchDisable}
              >
                {t("common.disable")}
              </Button>
              <Button
                size="sm"
                color="danger"
                variant="flat"
                className="bg-danger-500/10 text-danger h-8 min-w-0 px-3"
                startContent={<FaTrash />}
                onPress={mp.openBatchUninstallConfirm}
              >
                {t("mods.action_uninstall")}
              </Button>
            </div>
          ) : (
            <div className={cn("grid items-center gap-x-3", listGridColumns)}>
              <div className="flex justify-center">
                <Checkbox
                  size="sm"
                  classNames={{ wrapper: "after:bg-primary-500" }}
                  isSelected={allSelected}
                  isIndeterminate={indeterminate}
                  onValueChange={mp.onSelectAll}
                />
              </div>
              <button
                type="button"
                className="min-w-0 cursor-pointer flex items-center gap-1 hover:text-default-700 dark:hover:text-zinc-300 transition-colors select-none text-left"
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
                  className="flex items-center gap-1.5 text-xs hover:text-default-700 dark:hover:text-zinc-300 transition-colors"
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
            <div className="flex flex-col items-center justify-center h-full text-default-400 gap-2">
              <FiAlertTriangle className="w-8 h-8 opacity-50" />
              <p>{t("launcherpage.currentVersion_none")}</p>
            </div>
          ) : mp.listHydrating ? (
            <div className="flex flex-col items-center justify-center h-full text-default-400 gap-3">
              <Spinner size="lg" color="primary" />
              <p>{t("common.loading")}</p>
            </div>
          ) : mp.visibleItems.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full text-default-400 gap-2">
              <FaPuzzlePiece className="w-8 h-8 opacity-50" />
              <p>{t("moddedcard.content.none")}</p>
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
                      "grid rounded-2xl border transition-all p-3 bg-white/60 dark:bg-zinc-800/40 hover:bg-white/80 dark:hover:bg-zinc-800/80",
                      listGridColumns,
                      "grid-rows-[auto_auto] md:grid-rows-1 gap-x-3 gap-y-2",
                      mp.selectedKeys.has(item.key)
                        ? "border-primary-500/50 dark:border-primary-500/30 bg-primary-50/50 dark:bg-primary-900/10"
                        : "border-white/40 dark:border-white/5",
                    )}
                  >
                    <div
                      className="row-span-2 md:row-span-1 flex justify-center pt-1 md:pt-0"
                      onClick={(e) => e.stopPropagation()}
                    >
                      <Checkbox
                        size="sm"
                        color="success"
                        isSelected={mp.selectedKeys.has(item.key)}
                        onValueChange={() => mp.onSelectionChange(item.key)}
                      />
                    </div>

                    <div className="col-start-2 row-span-2 md:row-span-1 min-w-0 flex items-start gap-3">
                      <div className="w-11 h-11 rounded-xl bg-default-100 dark:bg-zinc-900 flex items-center justify-center text-default-500 dark:text-zinc-400 shrink-0">
                        <FaPuzzlePiece className="w-5 h-5" />
                      </div>
                      <div className="min-w-0 flex flex-col justify-center">
                        <div className="font-bold text-default-900 dark:text-zinc-100 truncate text-base">
                          {mod.name}
                        </div>
                        <div className="text-xs text-default-500 dark:text-zinc-400 truncate">
                          by {mod.author || "Unknown"}
                        </div>
                        <div className="text-xs text-default-500 dark:text-zinc-400 truncate font-mono opacity-80 mt-0.5">
                          {mod.entry || mod.type}
                        </div>
                      </div>
                    </div>

                    <div className="col-start-3 row-start-2 md:col-start-3 md:row-start-1 min-w-0 flex flex-col justify-center items-start">
                      <div className="text-default-700 dark:text-zinc-300 truncate text-sm">
                        {mod.version || "-"}
                      </div>
                      {lipState.sourceType === "unique" ? (
                        <div className="mt-1">
                          {lipState.canUpdate ? (
                            <Chip size="sm" variant="flat" color="success">
                              {t("mods.update_available", {
                                version: lipState.targetVersion,
                              })}
                            </Chip>
                          ) : (
                            <Chip size="sm" variant="flat">
                              {t("mods.update_latest")}
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
                          variant="light"
                          className="text-primary-600 dark:text-primary-400"
                          onPress={() => void mp.handleUpdateMod(mod)}
                          aria-label={t("mods.action_update") as string}
                        >
                          <LuDownload />
                        </Button>
                      ) : null}
                      <Switch
                        size="sm"
                        isSelected={!!mp.enabledByFolder.get(folder)}
                        classNames={{
                          wrapper: "group-data-[selected=true]:bg-primary-500",
                        }}
                        onValueChange={(value: boolean) => {
                          mp.toggleModEnabled(folder, value);
                        }}
                        aria-label={t("mods.toggle_label") as string}
                      />

                      <Dropdown classNames={COMPONENT_STYLES.dropdown}>
                        <DropdownTrigger>
                          <Button
                            isIconOnly
                            size="sm"
                            variant="light"
                            className="text-default-400"
                          >
                            <FaEllipsisVertical />
                          </Button>
                        </DropdownTrigger>
                        <DropdownMenu aria-label="mod actions">
                          <DropdownItem
                            key="uninstall"
                            color="danger"
                            startContent={<FaTrash />}
                            onPress={() => mp.openDeleteForMod(mod)}
                          >
                            {t("mods.action_uninstall")}
                          </DropdownItem>
                          <DropdownItem
                            key="update"
                            startContent={<FaSync />}
                            onPress={() => void mp.handleUpdateMod(mod)}
                            isDisabled={
                              lipState.sourceType !== "unique" ||
                              !lipState.canUpdate
                            }
                          >
                            {t("mods.action_update")}
                          </DropdownItem>
                          <DropdownItem
                            key="folder"
                            startContent={<FaFolderOpen />}
                            onPress={() => mp.openModFolder(mod)}
                          >
                            {t("common.open_folder")}
                          </DropdownItem>
                          <DropdownItem
                            key="details"
                            startContent={<FaInfoCircle />}
                            onPress={() => mp.openDetails(mod)}
                          >
                            {t("common.details")}
                          </DropdownItem>
                        </DropdownMenu>
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
                    "grid rounded-2xl border transition-all p-3 bg-white/60 dark:bg-zinc-800/40 hover:bg-white/80 dark:hover:bg-zinc-800/80",
                    listGridColumns,
                    "grid-rows-[auto_auto] md:grid-rows-1 gap-x-3 gap-y-2",
                    mp.selectedKeys.has(item.key)
                      ? "border-primary-500/50 dark:border-primary-500/30 bg-primary-50/50 dark:bg-primary-900/10"
                      : "border-white/40 dark:border-white/5",
                  )}
                >
                  <div
                    className="row-span-2 md:row-span-1 flex justify-center pt-1 md:pt-0"
                    onClick={(e) => e.stopPropagation()}
                  >
                    <Checkbox
                      size="sm"
                      color="success"
                      isSelected={mp.selectedKeys.has(item.key)}
                      onValueChange={() => mp.onSelectionChange(item.key)}
                    />
                  </div>

                  <div className="col-start-2 row-span-2 md:row-span-1 min-w-0 flex items-start gap-3">
                    <div className="w-11 h-11 rounded-xl bg-default-100 dark:bg-zinc-900 flex items-center justify-center text-default-500 dark:text-zinc-400 shrink-0">
                      <FaBoxOpen className="w-5 h-5" />
                    </div>
                    <div className="min-w-0 flex flex-col justify-center">
                      <div className="font-bold text-default-900 dark:text-zinc-100 truncate text-base">
                        {lipItem.packageName}
                      </div>
                      <div className="text-xs text-default-500 dark:text-zinc-400 truncate font-mono">
                        {lipItem.displayIdentifier}
                      </div>
                      <div className="text-xs text-default-500 dark:text-zinc-400 truncate mt-0.5">
                        {childrenSummary}
                      </div>
                    </div>
                  </div>

                  <div className="col-start-3 row-start-2 md:col-start-3 md:row-start-1 min-w-0 flex flex-col justify-center items-start">
                    <div className="text-default-700 dark:text-zinc-300 truncate text-sm">
                      {lipItem.installedVersion || "-"}
                    </div>
                    <div className="mt-1">
                      {lipState.canUpdate ? (
                        <Chip size="sm" variant="flat" color="success">
                          {t("mods.update_available", {
                            version: lipState.targetVersion,
                          })}
                        </Chip>
                      ) : (
                        <Chip size="sm" variant="flat">
                          {t("mods.update_latest")}
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
                        variant="light"
                        className="text-primary-600 dark:text-primary-400"
                        onPress={() => void mp.handleUpdateLipGroup(lipItem)}
                        aria-label={t("mods.action_update") as string}
                      >
                        <LuDownload />
                      </Button>
                    ) : null}
                    <Switch
                      size="sm"
                      isSelected={lipItem.allEnabled}
                      classNames={{
                        wrapper: "group-data-[selected=true]:bg-primary-500",
                      }}
                      onValueChange={(value: boolean) => {
                        void mp.toggleLipGroupEnabled(lipItem, value);
                      }}
                      aria-label={t("mods.toggle_label") as string}
                    />

                    <Dropdown classNames={COMPONENT_STYLES.dropdown}>
                      <DropdownTrigger>
                        <Button
                          isIconOnly
                          size="sm"
                          variant="light"
                          className="text-default-400"
                        >
                          <FaEllipsisVertical />
                        </Button>
                      </DropdownTrigger>
                      <DropdownMenu aria-label="lip actions">
                        <DropdownItem
                          key="uninstall"
                          color="danger"
                          startContent={<FaTrash />}
                          onPress={() => void mp.openDeleteForLipGroup(lipItem)}
                        >
                          {t("mods.action_uninstall")}
                        </DropdownItem>
                        {mp.isLipGroupPromotable(lipItem) ? (
                          <DropdownItem
                            key="promote"
                            startContent={<LuDownload />}
                            onPress={() =>
                              void mp.handlePromoteLipGroup(lipItem)
                            }
                          >
                            {t("mods.action_promote_install")}
                          </DropdownItem>
                        ) : null}
                        <DropdownItem
                          key="update"
                          startContent={<FaSync />}
                          onPress={() => void mp.handleUpdateLipGroup(lipItem)}
                          isDisabled={!lipState.canUpdate}
                        >
                          {t("mods.action_update")}
                        </DropdownItem>
                        <DropdownItem
                          key="openLip"
                          startContent={<FaInfoCircle />}
                          onPress={() =>
                            mp.openLIPPackageDetails(
                              lipItem.displayIdentifier || lipItem.identifier,
                            )
                          }
                        >
                          {t("mods.lip_open_package")}
                        </DropdownItem>
                      </DropdownMenu>
                    </Dropdown>
                  </div>
                </div>
              );
            })
          )}
        </div>
      </div>

      <UnifiedModal
        isOpen={mp.infoOpen}
        onOpenChange={mp.infoOnOpenChange}
        title={t("mods.details_title")}
        type="primary"
        icon={<FaPuzzlePiece className="w-6 h-6" />}
        hideCloseButton
        showConfirmButton={false}
        showCancelButton
        cancelText={t("common.close")}
        onCancel={() => mp.infoOnClose()}
        footer={
          <>
            <Button variant="light" onPress={() => mp.infoOnClose()}>
              {t("common.cancel")}
            </Button>
            <Button
              color="primary"
              variant="flat"
              onPress={() => {
                if (!mp.activeMod) return;
                void mp.handleUpdateMod(mp.activeMod);
              }}
              isDisabled={
                !mp.activeMod ||
                mp.getModLIPState(mp.activeMod).sourceType !== "unique" ||
                !mp.getModLIPState(mp.activeMod).canUpdate
              }
            >
              {t("mods.action_update")}
            </Button>
            <Button color="danger" onPress={mp.delCfmOnOpen}>
              {t("mods.action_uninstall")}
            </Button>
          </>
        }
      >
        {mp.activeMod ? (
          <div className="space-y-2 text-sm dark:text-zinc-200">
            <div>
              <span className="text-default-500 dark:text-zinc-400">
                {t("mods.field_name")}:
              </span>
              {mp.activeMod.name}
            </div>
            <div>
              <span className="text-default-500 dark:text-zinc-400">
                {t("mods.field_version")}:
              </span>
              {mp.activeMod.version || "-"}
            </div>
            <div>
              <span className="text-default-500 dark:text-zinc-400">
                {t("mods.field_type")}:
              </span>
              {mp.activeMod.type || "-"}
            </div>
            <div>
              <span className="text-default-500 dark:text-zinc-400">
                {t("mods.field_entry")}:
              </span>
              {mp.activeMod.entry || "-"}
            </div>
            {mp.activeMod.author ? (
              <div>
                <span className="text-default-500 dark:text-zinc-400">
                  {t("mods.field_author")}:
                </span>
                {mp.activeMod.author}
              </div>
            ) : null}
            <div>
              <span className="text-default-500 dark:text-zinc-400">
                {t("mods.action_update")}:
              </span>
              <span className="ml-2">
                {(() => {
                  const state = mp.getModLIPState(mp.activeMod);
                  if (state.sourceType !== "unique") {
                    return t("mods.no_update_source");
                  }
                  if (!state.canUpdate) {
                    return t("mods.update_latest");
                  }
                  return t("mods.update_available", {
                    version: state.targetVersion,
                  });
                })()}
              </span>
            </div>
            <div className="pt-2">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="text-default-500 dark:text-zinc-400">
                    {t("mods.toggle_label")}
                  </span>
                  <Chip
                    size="sm"
                    variant="flat"
                    color={
                      mp.enabledByFolder.get(mp.resolveModFolder(mp.activeMod!))
                        ? "success"
                        : "warning"
                    }
                  >
                    {mp.enabledByFolder.get(mp.resolveModFolder(mp.activeMod!))
                      ? (t("mods.toggle_on") as string)
                      : (t("mods.toggle_off") as string)}
                  </Chip>
                </div>
                <Switch
                  isSelected={
                    !!mp.enabledByFolder.get(mp.resolveModFolder(mp.activeMod!))
                  }
                  classNames={{
                    wrapper: "group-data-[selected=true]:bg-primary-500",
                  }}
                  onValueChange={(value: boolean) =>
                    mp.toggleModEnabled(
                      mp.resolveModFolder(mp.activeMod!),
                      value,
                    )
                  }
                  aria-label={t("mods.toggle_label") as string}
                />
              </div>
              <div className="text-default-500 dark:text-zinc-400 text-xs mt-1">
                {mp.enabledByFolder.get(mp.resolveModFolder(mp.activeMod!))
                  ? (t("mods.toggle_desc_on") as string)
                  : (t("mods.toggle_desc_off") as string)}
              </div>
            </div>
          </div>
        ) : null}
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
        <div className="text-sm text-default-700 dark:text-zinc-300 whitespace-pre-wrap">
          {t("errors.ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY")}
        </div>
        {mp.demotedWarningNames.length > 0 ? (
          <div className="mt-3 rounded-md bg-warning-50/70 dark:bg-warning-500/10 border border-warning-200/70 dark:border-warning-500/30 px-3 py-2 text-warning-700 dark:text-warning-300 text-sm whitespace-pre-wrap break-all font-mono">
            {mp.demotedWarningNames.join("\n")}
          </div>
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
        confirmButtonProps={{ isLoading: mp.actionConfirming }}
        cancelButtonProps={{ isDisabled: mp.actionConfirming }}
      >
        <div className="text-sm text-default-700 dark:text-zinc-300 whitespace-pre-wrap">
          {mp.actionConfirmBody}
        </div>
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
        confirmButtonProps={{ isLoading: mp.batchUpdating }}
        cancelButtonProps={{ isDisabled: mp.batchUpdating }}
      >
        <div className="text-sm text-default-700 dark:text-zinc-300 whitespace-pre-wrap">
          {t("mods.batch_update_body", {
            selected: mp.selectedItems.length,
            updatable: mp.selectedUpdatableCount,
          })}
        </div>
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
        confirmButtonProps={{ isLoading: mp.batchUninstalling }}
        cancelButtonProps={{ isDisabled: mp.batchUninstalling }}
      >
        <div className="text-sm text-default-700 dark:text-zinc-300 whitespace-pre-wrap">
          {t("mods.batch_uninstall_body", {
            count: mp.selectedItems.length,
          })}
        </div>
      </UnifiedModal>
    </PageContainer>
  );
};

export default ModsPage;
