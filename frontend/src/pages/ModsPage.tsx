import React, { useRef } from "react";
import {
  Button,
  Card,
  CardBody,
  Chip,
  Input,
  Progress,
  Switch,
  Checkbox,
  Dropdown,
  DropdownTrigger,
  DropdownMenu,
  DropdownItem,
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
} from "react-icons/fa";
import { FiUploadCloud, FiAlertTriangle } from "react-icons/fi";
import { FileDropOverlay } from "@/components/FileDropOverlay";
import { useFileDrag } from "@/hooks/useFileDrag";
import { useModsPage } from "@/hooks/useModsPage";
import { PageHeader } from "@/components/PageHeader";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { cn } from "@/utils/cn";
import { COMPONENT_STYLES } from "@/constants/componentStyles";

export const ModsPage: React.FC = () => {
  const { t } = useTranslation();
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const isDragActive = useFileDrag(scrollRef as React.RefObject<HTMLElement>);
  const mp = useModsPage(t as any, scrollRef);

  return (
    <PageContainer
      ref={scrollRef}
      id="mods-drop-zone"
      {...({ "data-file-drop-target": true } as any)}
      className="relative"
    >
      {/* Drag Overlay */}
      <FileDropOverlay isDragActive={isDragActive} text={t("mods.drop_hint")} />

      <UnifiedModal
        isOpen={mp.importing && !mp.dllOpen}
        onOpenChange={() => {}}
        title={t("mods.importing_title")}
        type="primary"
        icon={<FiUploadCloud className="w-6 h-6 text-primary-500" />}
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
        icon={<FaPuzzlePiece className="w-6 h-6 text-primary-500" />}
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

      {/* Drag Overlay */}
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
                  className="bg-primary-600 hover:bg-primary-500 text-white shadow-lg shadow-primary-900/20"
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

      <div className="flex flex-col sm:flex-row items-center gap-3 px-2">
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

      <div className="flex-1 overflow-hidden flex flex-col">
        <div className="flex flex-row items-center px-5 py-2 text-sm text-default-500 dark:text-zinc-400 font-semibold">
          <div className="w-10 flex justify-center">
            <Checkbox
              size="sm"
              classNames={{ wrapper: "after:bg-primary-500" }}
              isSelected={
                mp.sortedItems.length > 0 &&
                mp.selectedKeys.size === mp.sortedItems.length
              }
              isIndeterminate={
                mp.selectedKeys.size > 0 && mp.selectedKeys.size < mp.sortedItems.length
              }
              onValueChange={mp.onSelectAll}
            />
          </div>
          {mp.selectedKeys.size > 0 ? (
            <div className="flex-1 flex items-center gap-3 animate-in fade-in slide-in-from-left-2 duration-200">
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
                onPress={mp.handleBatchRemove}
              >
                {t("common.remove")}
              </Button>
            </div>
          ) : (
            <>
              <div
                className="flex-1 cursor-pointer flex items-center gap-1 hover:text-default-700 dark:hover:text-zinc-300 transition-colors select-none"
                onClick={() => mp.handleSort("name")}
              >
                {t("mods.field_name")}{" "}
                {mp.sortConfig.key === "name" &&
                  (mp.sortConfig.direction === "asc" ? (
                    <FaChevronUp className="text-xs" />
                  ) : (
                    <FaChevronDown className="text-xs" />
                  ))}
              </div>
              <div className="flex-[0.6]">{t("common.version")}</div>
              <div className="flex items-center gap-6 text-xs">
                <button
                  className="flex items-center gap-1.5 hover:text-default-700 dark:hover:text-zinc-300 transition-colors"
                  onClick={() => mp.refreshAll()}
                  disabled={mp.loading}
                >
                  <FaSync className={mp.loading ? "animate-spin" : ""} />
                  {t("common.refresh")}
                </button>
              </div>
            </>
          )}
        </div>

        <div className="flex-1 overflow-y-auto p-2 custom-scrollbar flex flex-col gap-2">
          {!mp.currentVersionName ? (
            <div className="flex flex-col items-center justify-center h-full text-default-400 gap-2">
              <FiAlertTriangle className="w-8 h-8 opacity-50" />
              <p>{t("launcherpage.currentVersion_none")}</p>
            </div>
          ) : mp.sortedItems.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full text-default-400 gap-2">
              <FaPuzzlePiece className="w-8 h-8 opacity-50" />
              <p>{t("moddedcard.content.none")}</p>
            </div>
          ) : (
            mp.sortedItems.map((m, idx) => {
              return (
                <div
                  key={`${m.name}-${m.version}-${idx}`}
                  className={`flex flex-row items-center p-3 rounded-2xl bg-white/60 dark:bg-zinc-800/40 hover:bg-white/80 dark:hover:bg-zinc-800/80 border transition-all gap-4 ${
                    mp.selectedKeys.has(m.name)
                      ? "border-primary-500/50 dark:border-primary-500/30 bg-primary-50/50 dark:bg-primary-900/10"
                      : "border-white/40 dark:border-white/5"
                  }`}
                >
                  <div
                    className="w-10 flex justify-center"
                    onClick={(e) => e.stopPropagation()}
                  >
                    <Checkbox
                      size="sm"
                      color="success"
                      isSelected={mp.selectedKeys.has(m.name)}
                      onValueChange={() => mp.onSelectionChange(m.name)}
                    />
                  </div>

                  <div className="w-12 h-12 rounded-xl bg-default-100 dark:bg-zinc-900 flex items-center justify-center text-default-500 dark:text-zinc-400 shrink-0">
                    <FaPuzzlePiece className="w-6 h-6" />
                  </div>

                  <div className="flex-1 min-w-0 flex flex-col justify-center">
                    <div className="font-bold text-default-900 dark:text-zinc-100 truncate text-base">
                      {m.name}
                    </div>
                    <div className="text-xs text-default-500 dark:text-zinc-400 truncate">
                      by {m.author || "Unknown"}
                    </div>
                  </div>

                  <div className="flex-[0.6] min-w-0 flex flex-col justify-center">
                    <div className="text-default-700 dark:text-zinc-300 truncate text-sm">
                      {m.version || "-"}
                    </div>
                    <div className="text-xs text-default-500 dark:text-zinc-400 truncate font-mono opacity-70">
                      {m.entry || m.type}
                    </div>
                  </div>

                  <div
                    className="flex items-center gap-3 pr-2"
                    onClick={(e) => e.stopPropagation()}
                  >
                    <Switch
                      size="sm"
                      isSelected={!!mp.enabledByName.get(m.name)}
                      classNames={{
                        wrapper: "group-data-[selected=true]:bg-primary-500",
                      }}
                      onValueChange={(val: boolean) => mp.toggleModEnabled(m.name, val)}
                      aria-label={t("mods.toggle_label") as string}
                    />

                    <Button
                      isIconOnly
                      size="sm"
                      variant="light"
                      className="text-default-400 hover:text-danger transition-opacity"
                      onPress={() => {
                        mp.setActiveMod(m);
                        mp.delCfmOnOpen();
                      }}
                    >
                      <FaTrash />
                    </Button>

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
                      <DropdownMenu aria-label="Mod Actions">
                        <DropdownItem
                          key="folder"
                          startContent={<FaFolderOpen />}
                          onPress={() => mp.openModFolder(m)}
                        >
                          {t("common.open_folder")}
                        </DropdownItem>
                        <DropdownItem
                          key="details"
                          startContent={<FaInfoCircle />}
                          onPress={() => mp.openDetails(m)}
                        >
                          {t("common.details")}
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
        icon={<FaPuzzlePiece className="w-6 h-6 text-primary-500" />}
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
            <Button color="danger" onPress={mp.delCfmOnOpen}>
              {t("common.delete")}
            </Button>
          </>
        }
      >
        {mp.activeMod ? (
          <div className="space-y-2 text-sm dark:text-zinc-200">
            <div>
              <span className="text-default-500 dark:text-zinc-400">
                {t("mods.field_name")}：
              </span>
              {mp.activeMod.name}
            </div>
            <div>
              <span className="text-default-500 dark:text-zinc-400">
                {t("mods.field_version")}：
              </span>
              {mp.activeMod.version || "-"}
            </div>
            <div>
              <span className="text-default-500 dark:text-zinc-400">
                {t("mods.field_type")}：
              </span>
              {mp.activeMod.type || "-"}
            </div>
            <div>
              <span className="text-default-500 dark:text-zinc-400">
                {t("mods.field_entry")}：
              </span>
              {mp.activeMod.entry || "-"}
            </div>
            {mp.activeMod.author ? (
              <div>
                <span className="text-default-500 dark:text-zinc-400">
                  {t("mods.field_author")}：
                </span>
                {mp.activeMod.author}
              </div>
            ) : null}
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
                      mp.enabledByName.get(mp.activeMod.name) ? "success" : "warning"
                    }
                  >
                    {mp.enabledByName.get(mp.activeMod.name)
                      ? (t("mods.toggle_on") as string)
                      : (t("mods.toggle_off") as string)}
                  </Chip>
                </div>
                <Switch
                  isSelected={!!mp.enabledByName.get(mp.activeMod.name)}
                  classNames={{
                    wrapper: "group-data-[selected=true]:bg-primary-500",
                  }}
                  onValueChange={(val: boolean) => mp.toggleModEnabled(mp.activeMod!.name, val)}
                  aria-label={t("mods.toggle_label") as string}
                />
              </div>
              <div className="text-default-500 dark:text-zinc-400 text-xs mt-1">
                {mp.enabledByName.get(mp.activeMod.name)
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
        onConfirm={mp.handleDeleteMod}
        title={t("mods.confirm_delete_title")}
        description={t("mods.confirm_delete_body", {
          type: t("moddedcard.title"),
        })}
        itemName={mp.activeMod?.name}
        isPending={mp.deleting}
      />
    </PageContainer>
  );
};

export default ModsPage;
