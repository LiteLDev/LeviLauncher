import { ModalDescription, ModalPanel, ModalAction, ModalProgress, ModalNotice } from "@/components/ModalPrimitives";
import {
  Button,
  Card,
  Checkbox,
  Chip,
  Input,
  Label,
  ListBox,
  ProgressBar,
  Select,
  Separator,
  Spinner,
  Switch,
  Tabs,
  TextArea,
  TextField,
} from "@heroui/react";

import { cn } from "@/utils/cn";

import { Dialogs } from "@wailsio/runtime";
import {
  UnifiedModal,
  getUnifiedModalConfirmButtonProps,
} from "@/components/UnifiedModal";
import { DeleteConfirmModal } from "@/components/DeleteConfirmModal";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { formatBytes } from "@/utils/formatting";

import { useTranslation } from "react-i18next";
import { motion } from "framer-motion";
import { FiAlertTriangle, FiCheckCircle } from "react-icons/fi";
import {
  GetVersionLogoDataUrl,
  RemoveVersionLogo,
  SaveVersionLogoFromPath,
  UnregisterVersionByName,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/versionservice";
import { PageHeader } from "@/components/PageHeader";
import LeviLaminaIcon from "@/assets/images/LeviLamina.png";
import { useInstanceSettings } from "@/hooks/useInstanceSettings";

type RestoreConflictDiffField = {
  key: string;
  label: string;
  backupValue: string;
  currentValue: string;
};

type RestoreConflict = {
  id: string;
  scopeKey: string;
  scopeLabel: string;
  path: string;
  sourceType: "file" | "dir";
  targetType: "file" | "dir";
  identityKind: "pack_uuid" | "world_folder" | "mod_folder" | "file_path";
  identityKey: string;
  backupPath: string;
  currentPath: string;
  backupSummary: string;
  currentSummary: string;
  diffFields: RestoreConflictDiffField[];
};

type RestoreConflictCategoryKey =
  | "mods"
  | "skin_packs"
  | "behavior_packs"
  | "resource_packs"
  | "worlds"
  | "dev_behavior_packs"
  | "dev_resource_packs"
  | "templates"
  | "files"
  | "other";

type RestoreConflictCategoryGroup = {
  key: RestoreConflictCategoryKey;
  labelKey: string;
  conflicts: RestoreConflict[];
  unresolvedCount: number;
};

type RestoreConflictScopeGroup = {
  key: string;
  label: string;
  conflicts: RestoreConflict[];
  unresolvedCount: number;
  categories: RestoreConflictCategoryGroup[];
};

const RESTORE_CONFLICT_CATEGORY_ORDER: RestoreConflictCategoryKey[] = [
  "mods",
  "skin_packs",
  "behavior_packs",
  "resource_packs",
  "worlds",
  "dev_behavior_packs",
  "dev_resource_packs",
  "templates",
  "files",
  "other",
];

const RESTORE_CONFLICT_CATEGORY_LABEL_KEYS: Record<
  RestoreConflictCategoryKey,
  string
> = {
  mods: "versions.edit.backup.restore.conflict_category.mods",
  skin_packs: "versions.edit.backup.restore.conflict_category.skin_packs",
  behavior_packs:
    "versions.edit.backup.restore.conflict_category.behavior_packs",
  resource_packs:
    "versions.edit.backup.restore.conflict_category.resource_packs",
  worlds: "versions.edit.backup.restore.conflict_category.worlds",
  dev_behavior_packs:
    "versions.edit.backup.restore.conflict_category.dev_behavior_packs",
  dev_resource_packs:
    "versions.edit.backup.restore.conflict_category.dev_resource_packs",
  templates: "versions.edit.backup.restore.conflict_category.templates",
  files: "versions.edit.backup.restore.conflict_category.files",
  other: "versions.edit.backup.restore.conflict_category.other",
};

const normalizeConflictPath = (value: string): string =>
  String(value || "")
    .replace(/\\/g, "/")
    .toLowerCase();

const resolveRestoreConflictCategoryKey = (
  conflict: RestoreConflict,
): RestoreConflictCategoryKey => {
  const paths = [conflict.path, conflict.backupPath, conflict.currentPath].map(
    normalizeConflictPath,
  );
  if (conflict.scopeKey === "mods") return "mods";
  if (paths.some((path) => path.includes("/skin_packs/"))) return "skin_packs";
  if (
    paths.some((path) => path.includes("/behavior_packs/")) ||
    paths.some((path) => path.includes("/development_behavior_packs/"))
  ) {
    return paths.some((path) => path.includes("/development_behavior_packs/"))
      ? "dev_behavior_packs"
      : "behavior_packs";
  }
  if (
    paths.some((path) => path.includes("/resource_packs/")) ||
    paths.some((path) => path.includes("/development_resource_packs/"))
  ) {
    return paths.some((path) => path.includes("/development_resource_packs/"))
      ? "dev_resource_packs"
      : "resource_packs";
  }
  if (
    paths.some((path) => path.includes("/minecraftworlds/")) ||
    paths.some((path) => path.includes("/worlds/"))
  ) {
    return "worlds";
  }
  if (paths.some((path) => path.includes("/world_templates/"))) {
    return "templates";
  }
  if (conflict.scopeKey === "gameData") return "files";
  return "other";
};

const resolveRestoreConflictTitle = (
  conflict: RestoreConflict,
  fallbackLabel: string,
): string => {
  const backupSummary = String(conflict.backupSummary || "").trim();
  const currentSummary = String(conflict.currentSummary || "").trim();
  if (backupSummary) return backupSummary;
  if (currentSummary) return currentSummary;
  const candidatePath = String(
    conflict.currentPath || conflict.backupPath || conflict.path || "",
  ).trim();
  const segments = candidatePath.split(/[\\/]+/).filter(Boolean);
  return segments[segments.length - 1] || fallbackLabel;
};

const hasRestoreConflictDetails = (conflict: RestoreConflict): boolean =>
  Boolean(
    String(conflict.identityKey || "").trim() ||
      String(conflict.backupPath || "").trim() ||
      String(conflict.currentPath || "").trim() ||
      (Array.isArray(conflict.diffFields) && conflict.diffFields.length > 0) ||
      conflict.sourceType !== conflict.targetType,
  );

const getScopeDisplayLabel = (
  scopeKey: string,
  fallbackLabel: string,
): string =>
  String(scopeKey || "").trim() === "mods"
    ? "Mods"
    : String(fallbackLabel || "").trim() || "-";

export default function InstanceSettingsPage() {
  const { t } = useTranslation();
  const vs = useInstanceSettings();
  const warningConfirmButtonProps =
    getUnifiedModalConfirmButtonProps("warning");
  const currentLLVersionText =
    vs.currentLLVersion ||
    (t("versions.edit.loader.ll_not_installed") as unknown as string);
  const llInstallActionLabel = t(vs.llInstallActionLabelKey) as string;
  const conflictItemFallbackLabel = t(
    "versions.edit.backup.restore.conflict_item_fallback",
  ) as string;
  const backupSettingsPath = `${t("settings.header.title")} > ${t("settings.tabs.others")}`;
  const backupExperimentalDisabledText = t(
    "versions.edit.backup.experimental_disabled",
    {
      settingsPath: backupSettingsPath,
    },
  ) as string;
  const restoreConflictGroups = (() => {
    const scopeOrder = new Map(
      vs.restoreScopes.map((scope, index) => [scope.key, index]),
    );
    const groups = new Map<
      string,
      RestoreConflictScopeGroup & {
        categoryMap: Map<
          RestoreConflictCategoryKey,
          RestoreConflictCategoryGroup
        >;
      }
    >();

    for (const conflict of vs.restoreConflicts as RestoreConflict[]) {
      const groupKey = String(
        conflict.scopeKey || conflict.scopeLabel || "other",
      ).trim();
      const selectedChoice = vs.restoreConflictChoices[conflict.id];
      const categoryKey = resolveRestoreConflictCategoryKey(conflict);
      const categoryLabelKey =
        RESTORE_CONFLICT_CATEGORY_LABEL_KEYS[categoryKey];
      let group = groups.get(groupKey);
      if (!group) {
        group = {
          key: groupKey,
          label: getScopeDisplayLabel(
            groupKey,
            String(conflict.scopeLabel || conflict.scopeKey || "-").trim(),
          ),
          conflicts: [],
          unresolvedCount: 0,
          categories: [],
          categoryMap: new Map(),
        };
        groups.set(groupKey, group);
      }
      group.conflicts.push(conflict);
      if (selectedChoice !== "backup" && selectedChoice !== "current") {
        group.unresolvedCount += 1;
      }

      let category = group.categoryMap.get(categoryKey);
      if (!category) {
        category = {
          key: categoryKey,
          labelKey: categoryLabelKey,
          conflicts: [],
          unresolvedCount: 0,
        };
        group.categoryMap.set(categoryKey, category);
      }
      category.conflicts.push(conflict);
      if (selectedChoice !== "backup" && selectedChoice !== "current") {
        category.unresolvedCount += 1;
      }
    }

    return Array.from(groups.values())
      .map((group) => ({
        key: group.key,
        label: group.label,
        conflicts: group.conflicts,
        unresolvedCount: group.unresolvedCount,
        categories: Array.from(group.categoryMap.values()).sort(
          (left, right) =>
            RESTORE_CONFLICT_CATEGORY_ORDER.indexOf(left.key) -
            RESTORE_CONFLICT_CATEGORY_ORDER.indexOf(right.key),
        ),
      }))
      .sort(
        (left, right) =>
          (scopeOrder.get(left.key) ?? Number.MAX_SAFE_INTEGER) -
          (scopeOrder.get(right.key) ?? Number.MAX_SAFE_INTEGER),
      );
  })();

  return (
    <PageContainer className="relative" animate={false}>
      <div className="flex flex-col gap-4">
        <motion.div
          data-material-motion
          initial={{ y: -20, opacity: 0 }}
          animate={{ y: 0, opacity: 1 }}
          transition={{ duration: 0.4 }}
        >
          <Card className={LAYOUT.GLASS_CARD.BASE}>
            <Card.Content className="p-6 w-full">
              <PageHeader
                title={t("versions.edit.title")}
                titleClassName="text-left pb-1"
                description={
                  <div className="mt-1 text-xs text-muted dark:text-zinc-400 truncate text-left">
                    {t("versions.edit.mc_version")}:{" "}
                    <span className="text-foreground dark:text-zinc-200 font-medium">
                      {vs.loading ? (
                        <span className="inline-block h-4 w-24 rounded bg-surface-tertiary animate-pulse" />
                      ) : (
                        vs.gameVersion ||
                        (t(
                          "launcherpage.version_select.unknown",
                        ) as unknown as string)
                      )}
                    </span>
                    <span className="mx-2 text-muted">·</span>
                    {t("versions.info.name")}:{" "}
                    <span className="text-foreground dark:text-zinc-200 font-medium">
                      {vs.targetName || "-"}
                    </span>
                    <span className="mx-2 text-muted">·</span>
                    {vs.versionType === "preview" ? (
                      <Chip size="sm" variant="soft" color={"warning"}>
                        <Chip.Label>{t("common.preview")}</Chip.Label>
                      </Chip>
                    ) : vs.versionType === "release" ? (
                      <span className="text-foreground dark:text-zinc-300">
                        {t("common.release")}
                      </span>
                    ) : (
                      <Chip size="sm" variant="soft" color={"accent"}>
                        <Chip.Label>{vs.versionType || "-"}</Chip.Label>
                      </Chip>
                    )}
                  </div>
                }
                endContent={
                  <div className="hidden sm:flex items-center gap-3">
                    <Button
                      onPress={() => vs.navigate(vs.returnToPath)}
                      variant={"ghost"}
                      className={cn(
                        "rounded-full",
                        "font-medium text-foreground dark:text-zinc-300",
                      )}
                    >
                      {t("common.cancel")}
                    </Button>
                    <Button
                      onPress={() => vs.onSave()}
                      variant={"primary"}
                      className={cn(
                        "rounded-full",
                        "bg-brand-500 hover:bg-brand-500 brand-primary-foreground font-bold shadow-lg shadow-brand-900/20",
                      )}
                    >
                      {t("common.ok")}
                    </Button>
                  </div>
                }
              />
              <Tabs
                selectedKey={vs.selectedTab}
                onSelectionChange={(k) => vs.setSelectedTab(k as string)}
                variant="primary"
                className={"mt-4"}
              >
                <Tabs.ListContainer>
                  <Tabs.List
                    aria-label={"Version Settings Tabs"}
                    className={COMPONENT_STYLES.tabs.tabList}
                  >
                    <Tabs.Tab key="general" id={"general"}>
                      {t("versions.edit.tabs.general")}
                      <Tabs.Indicator />
                    </Tabs.Tab>
                    <Tabs.Tab key="launch" id={"launch"}>
                      {t("versions.edit.tabs.launch")}
                      <Tabs.Indicator />
                    </Tabs.Tab>
                    <Tabs.Tab key="loader" id={"loader"}>
                      {t("versions.edit.tabs.loader")}
                      <Tabs.Indicator />
                    </Tabs.Tab>
                    <Tabs.Tab key="features" id={"features"}>
                      {t("versions.edit.tabs.features")}
                      <Tabs.Indicator />
                    </Tabs.Tab>
                    <Tabs.Tab key="manage" id={"manage"}>
                      {t("versions.edit.tabs.manage")}
                      <Tabs.Indicator />
                    </Tabs.Tab>
                  </Tabs.List>
                </Tabs.ListContainer>
              </Tabs>
            </Card.Content>
          </Card>
        </motion.div>

        <motion.div
          data-material-motion
          key={vs.selectedTab}
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3 }}
        >
          <Card className={LAYOUT.GLASS_CARD.BASE}>
            <Card.Content className="p-6">
              {vs.selectedTab === "general" && (
                <div className="flex flex-col gap-6">
                  <div>
                    <label className="text-sm font-medium text-foreground dark:text-zinc-200 mb-2 block">
                      {t("versions.edit.new_name")}
                    </label>
                    <TextField
                      aria-label={
                        t("versions.edit.placeholder") as unknown as string
                      }
                      isDisabled={vs.isRegistered || vs.loading}
                      className={cn(
                        "group",
                        COMPONENT_STYLES.input.mainWrapper,
                      )}
                      value={vs.newName}
                      onChange={(v) => {
                        vs.setNewName(v);
                        if (vs.error) vs.setError("");
                      }}
                    >
                      <Input
                        placeholder={
                          t("versions.edit.placeholder") as unknown as string
                        }
                        className={cn(
                          COMPONENT_STYLES.input.inputWrapper,
                          COMPONENT_STYLES.input.input,
                          "rounded-lg",
                        )}
                      />
                    </TextField>
                    <p className="text-xs text-muted mt-2">
                      {t("versions.edit.hint")}
                    </p>
                  </div>
                  <div className="flex flex-col gap-3">
                    <div className="text-sm font-medium text-foreground dark:text-zinc-200">
                      {t("versions.logo.title")}
                    </div>
                    <div className="flex items-center gap-4">
                      <div
                        className="relative h-24 w-24 rounded-2xl overflow-hidden bg-surface-secondary flex items-center justify-center border border-border cursor-pointer group transition-all hover:scale-105 hover:shadow-lg"
                        onClick={async () => {
                          try {
                            const paths = await Dialogs.OpenFile({
                              Title: t("versions.logo.title"),
                              Filters: [
                                {
                                  DisplayName: "Image Files",
                                  Pattern: "*.png;*.jpg;*.jpeg;*.gif;*.webp",
                                },
                              ],
                              AllowsMultipleSelection: false,
                            });
                            let path = "";
                            if (Array.isArray(paths) && paths.length > 0) {
                              path = paths[0];
                            } else if (typeof paths === "string" && paths) {
                              path = paths;
                            }

                            if (path) {
                              const saver = SaveVersionLogoFromPath as any;
                              const getter = GetVersionLogoDataUrl as any;
                              if (typeof saver === "function") {
                                saver(vs.targetName, path).then(
                                  (err: string) => {
                                    if (err) {
                                      vs.setError(
                                        String(err || "ERR_ICON_DECODE"),
                                      );
                                      return;
                                    }
                                    if (typeof getter === "function") {
                                      getter(vs.targetName).then((u: string) =>
                                        vs.setLogoDataUrl(String(u || "")),
                                      );
                                    }
                                  },
                                );
                              }
                            }
                          } catch (e) {
                            console.error(e);
                          }
                        }}
                        title={t("versions.logo.change") as string}
                      >
                        {vs.logoDataUrl ? (
                          <img
                            src={vs.logoDataUrl}
                            alt="logo"
                            className="h-full w-full object-cover"
                          />
                        ) : null}
                        <div className="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 transition flex items-center justify-center text-white text-xs font-medium backdrop-blur-[2px]">
                          {t("versions.logo.change")}
                        </div>
                      </div>
                      <div className="flex flex-col gap-2">
                        <Button
                          size="sm"
                          onPress={async () => {
                            try {
                              const rm = RemoveVersionLogo as any;
                              if (typeof rm === "function") {
                                await rm(vs.targetName);
                              }
                            } catch {}
                            vs.setLogoDataUrl("");
                          }}
                          variant={"danger-soft"}
                          className={cn("rounded-full", "px-4 font-medium")}
                        >
                          {t("versions.logo.clear")}
                        </Button>
                      </div>
                    </div>
                    <p className="text-xs text-muted">
                      {t("versions.logo.hint")}
                    </p>
                  </div>
                </div>
              )}
              {vs.selectedTab === "launch" && (
                <div className="flex flex-col gap-6">
                  <div className="flex items-center justify-between p-2 rounded-xl">
                    <div className="text-base font-medium">
                      {t("versions.edit.enable_isolation")}
                    </div>
                    <Switch
                      aria-label={t("versions.edit.enable_isolation")}
                      size="md"
                      isSelected={vs.enableIsolation}
                      onChange={vs.setEnableIsolation}
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
                  <div className="flex items-center justify-between p-2 rounded-xl">
                    <div className="text-base font-medium">
                      {t("versions.edit.enable_console")}
                    </div>
                    <Switch
                      aria-label={t("versions.edit.enable_console")}
                      size="md"
                      isSelected={vs.enableConsole}
                      onChange={vs.setEnableConsole}
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
                  <div className="flex flex-col gap-3">
                    <label className="text-sm font-medium text-foreground dark:text-zinc-200 block">
                      {t("versions.edit.launch_args")}
                    </label>
                    <TextField
                      aria-label={
                        t(
                          "versions.edit.launch_args_placeholder",
                        ) as unknown as string
                      }
                      className={cn(
                        "group",
                        COMPONENT_STYLES.input.mainWrapper,
                      )}
                      value={vs.launchArgs}
                      onChange={(v) => {
                        vs.setLaunchArgs(v);
                        if (vs.error) vs.setError("");
                      }}
                    >
                      <Input
                        placeholder={
                          t(
                            "versions.edit.launch_args_placeholder",
                          ) as unknown as string
                        }
                        className={cn(
                          COMPONENT_STYLES.input.inputWrapper,
                          COMPONENT_STYLES.input.input,
                          "rounded-lg",
                        )}
                      />
                    </TextField>
                    <p className="text-xs text-muted">
                      {t("versions.edit.launch_args_hint")}
                    </p>
                  </div>
                  <div className="flex flex-col gap-3">
                    <label className="text-sm font-medium text-foreground dark:text-zinc-200 block">
                      {t("versions.edit.env_vars")}
                    </label>
                    <TextField
                      aria-label={
                        t(
                          "versions.edit.env_vars_placeholder",
                        ) as unknown as string
                      }
                      className={cn(
                        "group",
                        COMPONENT_STYLES.input.mainWrapper,
                      )}
                      value={vs.envVars}
                      onChange={(v) => {
                        vs.setEnvVars(v);
                        if (vs.error) vs.setError("");
                      }}
                    >
                      <TextArea
                        placeholder={
                          t(
                            "versions.edit.env_vars_placeholder",
                          ) as unknown as string
                        }
                        rows={3}
                        className={cn(
                          COMPONENT_STYLES.input.inputWrapper,
                          COMPONENT_STYLES.input.input,
                          "rounded-lg",
                        )}
                      />
                    </TextField>
                    <p className="text-xs text-muted">
                      {t("versions.edit.env_vars_hint")}
                    </p>
                  </div>
                </div>
              )}
              {vs.selectedTab === "loader" && (
                <div className="flex flex-col gap-6">
                  <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                    <div className="flex items-center gap-4">
                      <div className="w-12 h-12 rounded-xl overflow-hidden">
                        <img
                          src={LeviLaminaIcon}
                          alt="LeviLamina"
                          className="w-full h-full object-cover"
                        />
                      </div>
                      <div>
                        <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
                          <div className="text-base font-bold text-foreground">
                            LeviLamina
                          </div>
                          <span
                            className={
                              vs.currentLLVersion
                                ? "text-sm font-semibold text-muted dark:text-zinc-400"
                                : "text-sm font-medium text-muted dark:text-zinc-500"
                            }
                          >
                            {currentLLVersionText}
                          </span>
                        </div>
                        <div className="text-sm text-muted dark:text-zinc-400">
                          {t("downloadpage.install.levilamina_desc")}
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-3">
                      {vs.isLLSupported(vs.gameVersion) ? (
                        <>
                          {vs.isLLInstalled && (
                            <Button
                              onPress={vs.openLLUninstallConfirm}
                              isDisabled={vs.installingLL || vs.uninstallingLL}
                              variant={"danger-soft"}
                              isPending={vs.uninstallingLL}
                            >
                              {({ isPending }) => (
                                <>
                                  <Spinner
                                    size="sm"
                                    color="current"
                                    className={isPending ? "" : "hidden"}
                                  />
                                  {t("common.remove")}
                                </>
                              )}
                            </Button>
                          )}
                          <Button
                            onPress={vs.openLeviLaminaVersionSelect}
                            isDisabled={
                              vs.installingLL ||
                              vs.uninstallingLL ||
                              vs.llSupportedVersions.length === 0
                            }
                            variant={"secondary"}
                            isPending={vs.installingLL}
                            className={
                              "bg-brand-500/10 text-brand-600 dark:text-brand-400 font-bold"
                            }
                          >
                            {({ isPending }) => (
                              <>
                                <Spinner
                                  size="sm"
                                  color="current"
                                  className={isPending ? "" : "hidden"}
                                />
                                {t(vs.llInstallActionLabelKey)}
                              </>
                            )}
                          </Button>
                        </>
                      ) : (
                        <div className="text-sm text-muted italic">
                          {t("downloadpage.install.levilamina_unsupported") ||
                            "Not Supported"}
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              )}
              {vs.selectedTab === "features" && (
                <div className="flex flex-col gap-5">
                  <div className="flex items-center justify-between p-2 rounded-xl">
                    <div className="text-base font-medium">
                      {t("versions.edit.enable_editor_mode")}
                    </div>
                    <Switch
                      aria-label={t("versions.edit.enable_editor_mode")}
                      size="md"
                      isSelected={vs.enableEditorMode}
                      onChange={vs.setEnableEditorMode}
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
                </div>
              )}
              {vs.selectedTab === "manage" && (
                <div className="flex flex-col gap-6">
                  <section className="flex flex-col gap-3">
                    <div className="flex flex-col gap-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <p className="font-medium text-foreground dark:text-zinc-200">
                          {t("versions.edit.backup.title")}
                        </p>
                        <Chip size="sm" variant="soft" color={"warning"}>
                          <Chip.Label>
                            {t("versions.edit.backup.experimental_badge")}
                          </Chip.Label>
                        </Chip>
                      </div>
                      <p className="text-xs text-muted dark:text-zinc-400 max-w-2xl">
                        {t("versions.edit.backup.hint", {
                          gameDataLabel: vs.backupGameDataLabel,
                        })}
                      </p>
                      {!vs.instanceBackupExperimentalEnabled ? (
                        <p className="text-xs text-amber-600 dark:text-amber-400 max-w-2xl">
                          {backupExperimentalDisabledText}
                        </p>
                      ) : null}
                    </div>
                    <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
                      <div className="flex flex-col gap-1 max-w-2xl">
                        <p className="text-sm text-foreground dark:text-zinc-300 leading-7">
                          {t("versions.edit.backup.section_body")}
                        </p>
                      </div>
                      <div className="shrink-0 flex flex-wrap gap-3">
                        <Button
                          isDisabled={
                            !vs.instanceBackupExperimentalEnabled ||
                            !vs.targetName ||
                            vs.loading
                          }
                          onPress={vs.openInstanceRestore}
                          variant={"secondary"}
                          isPending={
                            vs.restoreInfoLoading || vs.restoringInstance
                          }
                          className={cn("rounded-full", "font-medium")}
                        >
                          {({ isPending }) => (
                            <>
                              <Spinner
                                size="sm"
                                color="current"
                                className={isPending ? "" : "hidden"}
                              />
                              {t("versions.edit.backup.restore.button")}
                            </>
                          )}
                        </Button>
                        <Button
                          isDisabled={
                            !vs.instanceBackupExperimentalEnabled ||
                            !vs.targetName ||
                            vs.loading
                          }
                          onPress={vs.openInstanceBackup}
                          variant={"primary"}
                          isPending={
                            vs.backupInfoLoading || vs.backingUpInstance
                          }
                          className={cn("rounded-full", "font-medium")}
                        >
                          {({ isPending }) => (
                            <>
                              <Spinner
                                size="sm"
                                color="current"
                                className={isPending ? "" : "hidden"}
                              />
                              {t("versions.edit.backup.button")}
                            </>
                          )}
                        </Button>
                      </div>
                    </div>
                  </section>

                  <Separator className="bg-surface-tertiary/50" />

                  <section className="flex flex-col gap-3">
                    <div className="flex flex-col gap-1">
                      <p className="font-medium text-foreground dark:text-zinc-200">
                        {t("versions.edit.danger_title")}
                      </p>
                      <p className="text-xs text-muted dark:text-zinc-400 max-w-2xl">
                        {t("versions.edit.danger_hint")}
                      </p>
                    </div>
                    <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
                      <div className="flex flex-col gap-2 max-w-2xl">
                        <p className="font-medium text-foreground dark:text-zinc-200">
                          {vs.isRegistered
                            ? t("versions.edit.unregister_button")
                            : t("common.delete")}
                        </p>
                        <p className="text-sm text-muted dark:text-zinc-400 leading-7">
                          {vs.isRegistered
                            ? t("versions.edit.unregister_hint")
                            : t("versions.edit.delete_hint")}
                        </p>
                      </div>
                      <div className="shrink-0">
                        {vs.isRegistered ? (
                          <Button
                            isDisabled={vs.loading}
                            onPress={async () => {
                              try {
                                const fn = UnregisterVersionByName as any;
                                if (typeof fn === "function") {
                                  vs.setUnregisterOpen(true);
                                  const err: string = await fn(vs.targetName);
                                  vs.setUnregisterOpen(false);
                                  if (err) {
                                    vs.setError(String(err));
                                  } else {
                                    vs.setIsRegistered(false);
                                    vs.setUnregisterSuccessOpen(true);
                                  }
                                }
                              } catch {
                                vs.setUnregisterOpen(false);
                                vs.setError("ERR_UNREGISTER_FAILED");
                              }
                            }}
                            variant={"secondary"}
                            className={cn(
                              "rounded-full",
                              "bg-warning text-warning-foreground",
                              "font-medium",
                            )}
                          >
                            {t("versions.edit.unregister_button")}
                          </Button>
                        ) : (
                          <Button
                            onPress={() => vs.setDeleteOpen(true)}
                            variant={"danger-soft"}
                            className={cn("rounded-full", "font-medium")}
                          >
                            {t("common.delete")}
                          </Button>
                        )}
                      </div>
                    </div>
                  </section>
                </div>
              )}
            </Card.Content>
          </Card>
        </motion.div>
      </div>

      <UnifiedModal
        isOpen={vs.backupOpen}
        onOpenChange={vs.onInstanceBackupOpenChange}
        size="wide"
        type="primary"
        title={t("versions.edit.backup.dialog_title")}
        confirmText={t("versions.edit.backup.confirm")}
        cancelText={t("common.cancel")}
        showCancelButton
        onCancel={vs.closeInstanceBackup}
        onConfirm={vs.confirmInstanceBackup}
        confirmButtonProps={{
          isDisabled:
            vs.selectedBackupScopes.length === 0 || vs.backingUpInstance,
          isPending: vs.backingUpInstance,
        }}
      >
        <div className="space-y-4">
          <ModalDescription>
            {t("versions.edit.backup.dialog_body")}
          </ModalDescription>
          {vs.backupHasSharedScope ? (
            <ModalNotice tone="warning">
              {t("versions.edit.backup.shared_warning")}
            </ModalNotice>
          ) : null}
          {vs.backupFullModeSelected ? (
            <ModalNotice tone="danger">
              {t("versions.edit.backup.mode.full.warning")}
            </ModalNotice>
          ) : null}
          <div className="flex flex-col gap-3">
            {vs.backupScopes.map((scope) => {
              const isSelected = vs.selectedBackupScopeSet.has(scope.key);
              const selectedMode = vs.getBackupScopeMode(scope);
              const displayPath = selectedMode?.path || scope.path || "-";
              const displaySize = Number(selectedMode?.size ?? scope.size ?? 0);
              const displayLabel = getScopeDisplayLabel(scope.key, scope.label);
              return (
                <div
                  key={scope.key}
                  className={`rounded-2xl border p-4 transition-colors ${
                    isSelected
                      ? "border-brand-300 dark:border-brand-500/40 bg-brand-50/70 dark:bg-brand-500/10"
                      : "border-border/70 dark:border-white/10 bg-surface/40 dark:bg-surface/5"
                  }`}
                >
                  <div className="flex items-start gap-3">
                    <Checkbox
                      aria-label={getScopeDisplayLabel(scope.key, scope.label)}
                      isSelected={isSelected}
                      isDisabled={!scope.selectable || vs.backingUpInstance}
                      onChange={(selected) => {
                        vs.setBackupScopeSelected(scope.key, selected);
                      }}
                      className={"group"}
                    >
                      <Checkbox.Content>
                        <Checkbox.Control>
                          <Checkbox.Indicator />
                        </Checkbox.Control>
                        <span></span>
                      </Checkbox.Content>
                    </Checkbox>
                    <div className="min-w-0 flex-1 space-y-3">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="text-base font-semibold text-foreground dark:text-zinc-100">
                          {displayLabel}
                        </span>
                        {isSelected && selectedMode?.key ? (
                          <Chip size="sm" variant="soft" color={"accent"}>
                            <Chip.Label>
                              {t(
                                `versions.edit.backup.mode.${selectedMode.key}.label`,
                              )}
                            </Chip.Label>
                          </Chip>
                        ) : null}
                        {scope.shared ? (
                          <Chip size="sm" variant="soft" color={"warning"}>
                            <Chip.Label>
                              {t("versions.edit.backup.shared_tag")}
                            </Chip.Label>
                          </Chip>
                        ) : null}
                        {!scope.exists ? (
                          <Chip size="sm" variant="soft" color={"danger"}>
                            <Chip.Label>
                              {t("versions.edit.backup.missing_tag")}
                            </Chip.Label>
                          </Chip>
                        ) : null}
                        {scope.exists && displaySize === 0 ? (
                          <Chip size="sm" variant="soft">
                            <Chip.Label>
                              {t("versions.edit.backup.empty_tag")}
                            </Chip.Label>
                          </Chip>
                        ) : null}
                      </div>
                      {Array.isArray(scope.modes) && scope.modes.length > 0 ? (
                        <Select
                          isDisabled={!isSelected || vs.backingUpInstance}
                          value={selectedMode?.key ?? null}
                          onChange={(keys) => {
                            const nextValue = keys;
                            if (typeof nextValue === "string") {
                              vs.setBackupScopeMode(scope.key, nextValue);
                            }
                          }}
                          className={"max-w-sm"}
                        >
                          <Select.Trigger
                            className={cn("rounded-lg", "min-h-8 text-sm")}
                          >
                            <Select.Value />
                            <Select.Indicator />
                          </Select.Trigger>
                          <Select.Popover>
                            <ListBox>
                              {scope.modes.map((mode) => (
                                <ListBox.Item
                                  key={mode.key}
                                  id={mode.key}
                                  textValue={
                                    t(
                                      `versions.edit.backup.mode.${mode.key}.label`,
                                    ) as string
                                  }
                                >
                                  <Label>
                                    {t(
                                      `versions.edit.backup.mode.${mode.key}.label`,
                                    )}
                                  </Label>
                                  <ListBox.ItemIndicator />
                                </ListBox.Item>
                              ))}
                            </ListBox>
                          </Select.Popover>
                        </Select>
                      ) : null}
                      {isSelected && selectedMode?.warning ? (
                        <ModalNotice tone="danger">
                          {t(selectedMode.warning)}
                        </ModalNotice>
                      ) : null}
                      {isSelected &&
                      scope.key === "mods" &&
                      vs.backupLipPackageCount > 0 ? (
                        <div className="rounded-2xl border border-brand-200/70 dark:border-brand-500/30 bg-brand-50/80 dark:bg-brand-500/10 px-3 py-2 text-xs text-brand-700 dark:text-brand-300">
                          {t("versions.edit.backup.lip_summary", {
                            count: vs.backupLipPackageCount,
                            summary: vs.backupLipPackageSummary || "-",
                          })}
                        </div>
                      ) : null}
                      <div className="space-y-2 text-xs text-muted dark:text-zinc-400">
                        <div className="break-all">
                          {t("versions.edit.backup.path_label")}:{" "}
                          <span className="font-mono text-[11px] text-foreground dark:text-zinc-300">
                            {displayPath}
                          </span>
                        </div>
                        <div>
                          {t("versions.edit.backup.size_label")}:{" "}
                          <span className="text-foreground dark:text-zinc-300 font-medium">
                            {formatBytes(displaySize)}
                          </span>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
          {vs.selectedBackupScopes.length === 0 ? (
            <div className="text-sm text-rose-500">
              {t("versions.edit.backup.no_scope_selected")}
            </div>
          ) : null}
        </div>
      </UnifiedModal>

      <UnifiedModal
        isOpen={vs.backingUpInstance}
        isDismissable={false}
        type="primary"
        title={t("versions.edit.backup.progress_title")}
      >
        <ModalProgress label={t("versions.edit.backup.progress_title")} description={t("versions.edit.backup.progress_body")} />
      </UnifiedModal>

      <UnifiedModal
        isOpen={vs.restoreOpen}
        onOpenChange={vs.onInstanceRestoreOpenChange}
        size="detail"
        type="primary"
        title={t("versions.edit.backup.restore.dialog_title")}
        confirmText={t("versions.edit.backup.restore.confirm")}
        cancelText={t("common.cancel")}
        showCancelButton
        onCancel={vs.closeInstanceRestore}
        onConfirm={vs.confirmInstanceRestore}
        confirmButtonProps={{
          isDisabled:
            vs.selectedRestoreScopes.length === 0 ||
            vs.restoringInstance ||
            vs.restoreConflictLoading ||
            vs.restoreHasUnresolvedConflicts,
          isPending: vs.restoringInstance,
        }}
      >
        <div className="space-y-4">
          <ModalDescription>
            {t("versions.edit.backup.restore.dialog_body")}
          </ModalDescription>
          <ModalPanel>
            <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-start">
              <div className="min-w-0">
                <div className="text-xs text-muted dark:text-zinc-400 mb-1">
                  {t("versions.edit.backup.archive_label")}
                </div>
                <div className="font-mono text-sm break-all text-foreground dark:text-zinc-100">
                  {vs.restoreArchiveInfo?.archiveName || "-"}
                </div>
              </div>
              <ModalPanel>
                <div className="text-xs text-muted dark:text-zinc-400">
                  {t("versions.edit.backup.restore.created_at")}
                </div>
                <ModalDescription>
                  {vs.restoreArchiveCreatedAtText || "-"}
                </ModalDescription>
              </ModalPanel>
            </div>
          </ModalPanel>
          {vs.restoreHasHighRiskScope ? (
            <ModalNotice tone="danger">
              {t("versions.edit.backup.mode.full.warning")}
            </ModalNotice>
          ) : null}
          <div className="rounded-2xl border border-border/70 dark:border-white/10 bg-surface/25 dark:bg-surface/5 px-4 py-4 space-y-3">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm font-semibold text-foreground dark:text-zinc-100">
                {t("versions.edit.backup.restore.scope_section_title")}
              </span>
              <Chip size="sm" variant="soft">
                <Chip.Label>{vs.restoreScopes.length}</Chip.Label>
              </Chip>
            </div>
            <div className="grid gap-3 md:grid-cols-2">
              {vs.restoreScopes.map((scope) => {
                const isSelected = vs.selectedRestoreScopeSet.has(scope.key);
                const displayLabel = getScopeDisplayLabel(
                  scope.key,
                  scope.label,
                );
                return (
                  <div
                    key={scope.key}
                    className={`rounded-2xl border p-4 transition-colors ${
                      isSelected
                        ? "border-brand-300 dark:border-brand-500/40 bg-brand-50/70 dark:bg-brand-500/10"
                        : "border-border/70 dark:border-white/10 bg-surface/40 dark:bg-surface/5"
                    }`}
                  >
                    <div className="flex items-start gap-3">
                      <Checkbox
                        aria-label={getScopeDisplayLabel(
                          scope.key,
                          scope.label,
                        )}
                        isSelected={isSelected}
                        isDisabled={vs.restoringInstance}
                        onChange={(selected) => {
                          vs.setRestoreScopeSelected(scope.key, selected);
                        }}
                        className={"group"}
                      >
                        <Checkbox.Content>
                          <Checkbox.Control>
                            <Checkbox.Indicator />
                          </Checkbox.Control>
                          <span></span>
                        </Checkbox.Content>
                      </Checkbox>
                      <div className="min-w-0 flex-1 space-y-3">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="text-base font-semibold text-foreground dark:text-zinc-100">
                            {displayLabel}
                          </span>
                          {scope.mode ? (
                            <Chip size="sm" variant="soft" color={"accent"}>
                              <Chip.Label>
                                {t(
                                  `versions.edit.backup.mode.${scope.mode}.label`,
                                )}
                              </Chip.Label>
                            </Chip>
                          ) : null}
                        </div>
                        {scope.key === "mods" &&
                        vs.restoreArchiveInfo?.modsLipPackages?.length ? (
                          <div className="rounded-xl border border-border/70 dark:border-white/10 bg-surface/55 dark:bg-surface/5 px-3 py-2.5">
                            <div className="flex flex-wrap items-center">
                              <span className="text-xs leading-6 text-foreground dark:text-zinc-300">
                                {t("versions.edit.backup.restore.lip_summary", {
                                  count:
                                    vs.restoreArchiveInfo.modsLipPackages
                                      .length,
                                })}
                              </span>
                            </div>
                          </div>
                        ) : null}
                        {Array.isArray(scope.warnings) &&
                        scope.warnings.length > 0 ? (
                          <div className="space-y-2">
                            {scope.warnings.map((warning) => (
                              <ModalNotice
                                key={`${scope.key}-${warning}`}
                                tone="danger"
                              >
                                {t(warning)}
                              </ModalNotice>
                            ))}
                          </div>
                        ) : null}
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
          {vs.selectedRestoreScopes.length > 0 ? (
            <div className="space-y-3">
              {vs.restoreConflictLoading ? (
                <div className="rounded-2xl border border-border/70 dark:border-white/10 bg-surface/60 dark:bg-surface/5 px-4 py-3 text-sm text-foreground dark:text-zinc-300">
                  {t("versions.edit.backup.restore.conflict_loading")}
                </div>
              ) : null}
              {!vs.restoreConflictLoading && vs.restoreConflicts.length > 0 ? (
                <>
                  <ModalNotice tone="warning">
                    {t("versions.edit.backup.restore.conflict_summary", {
                      count: vs.restoreConflicts.length,
                    })}
                  </ModalNotice>
                  <div className="flex flex-col gap-3">
                    {restoreConflictGroups.map((group) => (
                      <div
                        key={group.key}
                        className="rounded-[28px] border border-border/70 dark:border-white/10 bg-surface/40 dark:bg-surface/5 p-4 sm:p-5 space-y-4"
                      >
                        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
                          <div className="min-w-0 space-y-2">
                            <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
                              <span className="text-base font-semibold text-foreground dark:text-zinc-100">
                                {group.label}
                              </span>
                              <span className="text-sm font-medium text-foreground dark:text-zinc-300">
                                {t(
                                  "versions.edit.backup.restore.conflict_group_count",
                                  { count: group.conflicts.length },
                                )}
                              </span>
                              {group.unresolvedCount > 0 ? (
                                <span className="text-sm text-muted dark:text-zinc-400">
                                  {t(
                                    "versions.edit.backup.restore.conflict_group_unresolved",
                                    { count: group.unresolvedCount },
                                  )}
                                </span>
                              ) : (
                                <span className="text-sm text-muted dark:text-zinc-400">
                                  {t(
                                    "versions.edit.backup.restore.conflict_group_resolved",
                                  )}
                                </span>
                              )}
                            </div>
                            <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-muted dark:text-zinc-400">
                              {group.categories.map((category, index) => (
                                <span
                                  key={`${group.key}-${category.key}`}
                                  className="contents"
                                >
                                  {index > 0 ? (
                                    <span className="text-muted dark:text-zinc-600">
                                      ·
                                    </span>
                                  ) : null}
                                  <span>
                                    {t(category.labelKey)}{" "}
                                    {category.conflicts.length}
                                  </span>
                                </span>
                              ))}
                            </div>
                          </div>
                          <div className="flex flex-wrap gap-2">
                            <Button
                              size="sm"
                              isDisabled={vs.restoringInstance}
                              onPress={() =>
                                vs.setRestoreConflictChoicesBulk(
                                  group.conflicts.map(
                                    (conflict) => conflict.id,
                                  ),
                                  "backup",
                                )
                              }
                              variant={"outline"}
                              className={cn(
                                "rounded-full",
                                "min-w-[132px] border-border bg-surface-secondary px-4 text-foreground shadow-sm transition-colors hover:bg-surface-tertiary dark:border-white/15 dark:bg-surface/10 dark:text-zinc-300 dark:hover:bg-surface/15",
                              )}
                            >
                              {t(
                                "versions.edit.backup.restore.conflict_group_apply_backup",
                              )}
                            </Button>
                            <Button
                              size="sm"
                              isDisabled={vs.restoringInstance}
                              onPress={() =>
                                vs.setRestoreConflictChoicesBulk(
                                  group.conflicts.map(
                                    (conflict) => conflict.id,
                                  ),
                                  "current",
                                )
                              }
                              variant={"outline"}
                              className={cn(
                                "rounded-full",
                                "min-w-[132px] border-border bg-surface-secondary px-4 text-foreground shadow-sm transition-colors hover:bg-surface-tertiary dark:border-white/15 dark:bg-surface/10 dark:text-zinc-300 dark:hover:bg-surface/15",
                              )}
                            >
                              {t(
                                "versions.edit.backup.restore.conflict_group_apply_current",
                              )}
                            </Button>
                          </div>
                        </div>
                        <div className="space-y-4">
                          {group.categories.map((category) => (
                            <div
                              key={`${group.key}-${category.key}`}
                              className="space-y-3"
                            >
                              {group.categories.length > 1 ? (
                                <div className="flex flex-wrap items-center gap-2 text-sm text-muted dark:text-zinc-400">
                                  <span className="font-medium">
                                    {t(category.labelKey)}
                                  </span>
                                  <span className="text-muted dark:text-zinc-600">
                                    ·
                                  </span>
                                  <span>{category.conflicts.length}</span>
                                </div>
                              ) : null}
                              <div className="grid gap-3">
                                {category.conflicts.map((conflict) => {
                                  const selectedChoice =
                                    vs.restoreConflictChoices[conflict.id];
                                  const title = resolveRestoreConflictTitle(
                                    conflict,
                                    conflictItemFallbackLabel,
                                  );
                                  const canShowDetails =
                                    hasRestoreConflictDetails(conflict);
                                  return (
                                    <div
                                      key={conflict.id}
                                      className="rounded-2xl border border-border/70 dark:border-white/10 bg-surface/70 dark:bg-surface/20 p-4 space-y-3"
                                    >
                                      <div className="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
                                        <div className="min-w-0 space-y-2">
                                          <div className="flex flex-wrap items-center gap-2">
                                            <span className="text-sm font-semibold text-foreground dark:text-zinc-100 break-all">
                                              {title}
                                            </span>
                                            <Chip
                                              size="sm"
                                              variant="soft"
                                              className={
                                                "bg-surface-secondary text-foreground dark:bg-surface/10 dark:text-zinc-300"
                                              }
                                            >
                                              <Chip.Label>
                                                {t(
                                                  `versions.edit.backup.restore.identity_kind.${conflict.identityKind}`,
                                                )}
                                              </Chip.Label>
                                            </Chip>
                                            {conflict.sourceType !==
                                            conflict.targetType ? (
                                              <Chip size="sm" variant="soft">
                                                <Chip.Label>
                                                  {t(
                                                    `versions.edit.backup.restore.conflict_type.${conflict.sourceType}`,
                                                  )}
                                                  {" -> "}
                                                  {t(
                                                    `versions.edit.backup.restore.conflict_type.${conflict.targetType}`,
                                                  )}
                                                </Chip.Label>
                                              </Chip>
                                            ) : null}
                                            {selectedChoice !== "backup" &&
                                            selectedChoice !== "current" ? (
                                              <Chip
                                                size="sm"
                                                variant="soft"
                                                className={
                                                  "bg-surface-secondary text-foreground dark:bg-surface/10 dark:text-zinc-300"
                                                }
                                              >
                                                <Chip.Label>
                                                  {t(
                                                    "versions.edit.backup.restore.conflict_choice_pending",
                                                  )}
                                                </Chip.Label>
                                              </Chip>
                                            ) : null}
                                          </div>
                                          <div className="flex flex-wrap gap-2 text-xs text-muted dark:text-zinc-400">
                                            {Array.isArray(conflict.diffFields)
                                              ? conflict.diffFields
                                                  .slice(0, 3)
                                                  .map((field) => (
                                                    <Chip
                                                      key={`${conflict.id}-${field.key}`}
                                                      size="sm"
                                                      variant="soft"
                                                    >
                                                      <Chip.Label>
                                                        {t(field.label)}
                                                      </Chip.Label>
                                                    </Chip>
                                                  ))
                                              : null}
                                            {Array.isArray(
                                              conflict.diffFields,
                                            ) &&
                                            conflict.diffFields.length > 3 ? (
                                              <Chip size="sm" variant="soft">
                                                <Chip.Label>
                                                  +
                                                  {conflict.diffFields.length -
                                                    3}
                                                </Chip.Label>
                                              </Chip>
                                            ) : null}
                                          </div>
                                        </div>
                                        <div className="w-full xl:max-w-xs">
                                          <Select
                                            placeholder={
                                              t(
                                                "versions.edit.backup.restore.conflict_choice_placeholder",
                                              ) as string
                                            }
                                            isDisabled={vs.restoringInstance}
                                            value={selectedChoice ?? null}
                                            onChange={(keys) => {
                                              const nextValue = keys;
                                              if (
                                                nextValue === "backup" ||
                                                nextValue === "current"
                                              ) {
                                                vs.setRestoreConflictChoice(
                                                  conflict.id,
                                                  nextValue,
                                                );
                                              }
                                            }}
                                          >
                                            <Select.Trigger
                                              className={cn(
                                                "rounded-lg",
                                                "min-h-8 text-sm",
                                              )}
                                            >
                                              <Select.Value />
                                              <Select.Indicator />
                                            </Select.Trigger>
                                            <Select.Popover>
                                              <ListBox>
                                                <ListBox.Item
                                                  key="backup"
                                                  id={"backup"}
                                                  textValue={
                                                    t(
                                                      "versions.edit.backup.restore.conflict_choice_backup",
                                                    ) as string
                                                  }
                                                >
                                                  <Label>
                                                    {t(
                                                      "versions.edit.backup.restore.conflict_choice_backup",
                                                    )}
                                                  </Label>
                                                  <ListBox.ItemIndicator />
                                                </ListBox.Item>
                                                <ListBox.Item
                                                  key="current"
                                                  id={"current"}
                                                  textValue={
                                                    t(
                                                      "versions.edit.backup.restore.conflict_choice_current",
                                                    ) as string
                                                  }
                                                >
                                                  <Label>
                                                    {t(
                                                      "versions.edit.backup.restore.conflict_choice_current",
                                                    )}
                                                  </Label>
                                                  <ListBox.ItemIndicator />
                                                </ListBox.Item>
                                              </ListBox>
                                            </Select.Popover>
                                          </Select>
                                        </div>
                                      </div>
                                      {conflict.backupSummary ||
                                      conflict.currentSummary ? (
                                        <div className="grid gap-2 lg:grid-cols-2">
                                          <div className="rounded-2xl border border-border/70 dark:border-white/10 bg-surface/70 dark:bg-surface/5 px-3 py-3 space-y-2">
                                            <div className="text-[11px] uppercase tracking-wide text-muted dark:text-zinc-400">
                                              {t(
                                                "versions.edit.backup.restore.backup_side_label",
                                              )}
                                            </div>
                                            <ModalDescription className="break-all">
                                              {conflict.backupSummary || "-"}
                                            </ModalDescription>
                                          </div>
                                          <div className="rounded-2xl border border-border/70 dark:border-white/10 bg-surface/70 dark:bg-surface/5 px-3 py-3 space-y-2">
                                            <div className="text-[11px] uppercase tracking-wide text-muted dark:text-zinc-400">
                                              {t(
                                                "versions.edit.backup.restore.current_side_label",
                                              )}
                                            </div>
                                            <ModalDescription className="break-all">
                                              {conflict.currentSummary || "-"}
                                            </ModalDescription>
                                          </div>
                                        </div>
                                      ) : null}
                                      {canShowDetails ? (
                                        <details className="rounded-2xl border border-border/70 dark:border-white/10 bg-surface/60 dark:bg-surface/5 px-3 py-3">
                                          <summary className="cursor-pointer text-xs font-medium text-foreground dark:text-zinc-300 select-none">
                                            {t(
                                              "versions.edit.backup.restore.conflict_details_toggle",
                                            )}
                                          </summary>
                                          <div className="mt-3 space-y-3">
                                            {conflict.identityKey ? (
                                              <div className="text-xs text-muted dark:text-zinc-400 break-all">
                                                {t(
                                                  "versions.edit.backup.restore.identity_key_label",
                                                )}
                                                :{" "}
                                                <span className="font-mono text-foreground dark:text-zinc-300">
                                                  {conflict.identityKey}
                                                </span>
                                              </div>
                                            ) : null}
                                            {conflict.backupPath ||
                                            conflict.currentPath ? (
                                              <div className="grid gap-3 md:grid-cols-2">
                                                <ModalPanel className="space-y-2">
                                                  <div className="text-xs uppercase tracking-wide text-muted dark:text-zinc-400">
                                                    {t(
                                                      "versions.edit.backup.restore.backup_path_label",
                                                    )}
                                                  </div>
                                                  <div className="font-mono text-xs text-foreground dark:text-zinc-300 break-all">
                                                    {conflict.backupPath || "-"}
                                                  </div>
                                                </ModalPanel>
                                                <ModalPanel className="space-y-2">
                                                  <div className="text-xs uppercase tracking-wide text-muted dark:text-zinc-400">
                                                    {t(
                                                      "versions.edit.backup.restore.current_path_label",
                                                    )}
                                                  </div>
                                                  <div className="font-mono text-xs text-foreground dark:text-zinc-300 break-all">
                                                    {conflict.currentPath ||
                                                      "-"}
                                                  </div>
                                                </ModalPanel>
                                              </div>
                                            ) : null}
                                            {Array.isArray(
                                              conflict.diffFields,
                                            ) &&
                                            conflict.diffFields.length > 0 ? (
                                              <div className="space-y-2">
                                                <Separator />
                                                {conflict.diffFields.map(
                                                  (field) => (
                                                    <div
                                                      key={`${conflict.id}-${field.key}`}
                                                      className="rounded-2xl border border-border/70 dark:border-white/10 bg-surface/40 dark:bg-surface/5 px-3 py-3 space-y-2"
                                                    >
                                                      <ModalDescription>
                                                        {t(field.label)}
                                                      </ModalDescription>
                                                      <div className="grid gap-2 md:grid-cols-2">
                                                        <div className="rounded-xl bg-surface-secondary/80 dark:bg-surface/5 px-3 py-2">
                                                          <div className="text-[11px] uppercase tracking-wide text-muted dark:text-zinc-400">
                                                            {t(
                                                              "versions.edit.backup.restore.backup_side_label",
                                                            )}
                                                          </div>
                                                          <div className="text-sm text-foreground dark:text-zinc-100 break-all font-mono">
                                                            {field.backupValue ||
                                                              "-"}
                                                          </div>
                                                        </div>
                                                        <div className="rounded-xl bg-surface-secondary/80 dark:bg-surface/5 px-3 py-2">
                                                          <div className="text-[11px] uppercase tracking-wide text-muted dark:text-zinc-400">
                                                            {t(
                                                              "versions.edit.backup.restore.current_side_label",
                                                            )}
                                                          </div>
                                                          <div className="text-sm text-foreground dark:text-zinc-100 break-all font-mono">
                                                            {field.currentValue ||
                                                              "-"}
                                                          </div>
                                                        </div>
                                                      </div>
                                                    </div>
                                                  ),
                                                )}
                                              </div>
                                            ) : null}
                                          </div>
                                        </details>
                                      ) : null}
                                    </div>
                                  );
                                })}
                              </div>
                            </div>
                          ))}
                        </div>
                      </div>
                    ))}
                  </div>
                  {vs.restoreHasUnresolvedConflicts ? (
                    <div className="text-sm text-rose-500">
                      {t("versions.edit.backup.restore.conflict_unresolved")}
                    </div>
                  ) : null}
                </>
              ) : null}
              {!vs.restoreConflictLoading &&
              vs.selectedRestoreScopes.length > 0 &&
              vs.restoreConflicts.length === 0 ? (
                <div className="rounded-2xl border border-green-200/70 dark:border-green-500/30 bg-green-50/80 dark:bg-green-500/10 px-4 py-3 text-sm text-green-700 dark:text-green-300">
                  {t("versions.edit.backup.restore.conflict_empty")}
                </div>
              ) : null}
            </div>
          ) : null}
          {vs.selectedRestoreScopes.length === 0 ? (
            <div className="text-sm text-rose-500">
              {t("versions.edit.backup.no_scope_selected")}
            </div>
          ) : null}
        </div>
      </UnifiedModal>

      <UnifiedModal
        isOpen={vs.restoringInstance}
        isDismissable={false}
        type="primary"
        title={t("versions.edit.backup.restore.progress_title")}
      >
        <ModalProgress
          label={t("versions.edit.backup.restore.progress_title")}
          description={vs.restoreProgressText || t("versions.edit.backup.restore.progress_body")}
          detail={vs.restoreProgressStepText}
          value={vs.restoreProgress ? vs.restoreProgressPercent : undefined}
        />
      </UnifiedModal>

      <UnifiedModal
        isOpen={vs.backupSuccessOpen}
        onOpenChange={(open) => {
          if (!open) vs.setBackupSuccessOpen(false);
        }}
        size="standard"
        type="success"
        title={t("versions.edit.backup.success_title")}
        icon={<FiCheckCircle className="w-6 h-6" />}
        footer={
          <>
            <ModalAction
              onPress={() => vs.setBackupSuccessOpen(false)}
              variant="secondary"
            >
              {t("launcherpage.delete.complete.close_button")}
            </ModalAction>
            <ModalAction
              onPress={async () => {
                await vs.openInstanceBackupDirectory();
                vs.setBackupSuccessOpen(false);
              }}
              variant={"primary"}
            >
              {t("versions.edit.backup.open_dir")}
            </ModalAction>
          </>
        }
      >
        <ModalDescription className="space-y-3">
          <ModalDescription>
            {t("versions.edit.backup.success_body")}
          </ModalDescription>
          <ModalPanel>
            <div className="text-xs text-muted dark:text-zinc-400 mb-1">
              {t("versions.edit.backup.archive_label")}
            </div>
            <div className="font-mono text-sm break-all text-foreground dark:text-zinc-100">
              {vs.backupArchiveName || "-"}
            </div>
          </ModalPanel>
          <ModalPanel>
            <div className="text-xs text-muted dark:text-zinc-400 mb-1">
              {t("versions.edit.backup.location_label")}
            </div>
            <div className="font-mono text-sm break-all text-foreground dark:text-zinc-100">
              {vs.backupResult?.backupDir || vs.backupInfo?.backupDir || "-"}
            </div>
          </ModalPanel>
        </ModalDescription>
      </UnifiedModal>

      <UnifiedModal
        isOpen={vs.restoreResultOpen}
        onOpenChange={(open) => {
          if (!open) vs.setRestoreResultOpen(false);
        }}
        size="wide"
        type={vs.restoreResultType}
        title={t(vs.restoreResultTitleKey)}
        icon={
          vs.restoreResultType === "success" ? (
            <FiCheckCircle className="w-6 h-6" />
          ) : (
            <FiAlertTriangle className="w-6 h-6" />
          )
        }
        footer={
          <ModalAction
            onPress={() => vs.setRestoreResultOpen(false)}
            variant="secondary"
          >
            {t("launcherpage.delete.complete.close_button")}
          </ModalAction>
        }
      >
        <ModalDescription className="space-y-3">
          <ModalDescription>
            {t(
              vs.restoreResult?.status === "success"
                ? "versions.edit.backup.restore.success_body"
                : vs.restoreResult?.status === "partial"
                  ? "versions.edit.backup.restore.partial_body"
                  : "versions.edit.backup.restore.failed_body",
            )}
          </ModalDescription>
          {(vs.restoreResult?.scopeResults || []).map((scopeResult) => (
            <ModalPanel
              key={`${scopeResult.key}-${scopeResult.mode}`}
              className="space-y-3"
            >
              <div className="flex flex-wrap items-center gap-2">
                <span className="font-semibold text-foreground dark:text-zinc-100">
                  {scopeResult.label}
                </span>
                {scopeResult.mode ? (
                  <Chip size="sm" variant="soft" color={"warning"}>
                    <Chip.Label>
                      {t(`versions.edit.backup.mode.${scopeResult.mode}.label`)}
                    </Chip.Label>
                  </Chip>
                ) : null}
                <Chip
                  size="sm"
                  variant="soft"
                  color={
                    scopeResult.status === "success"
                      ? "success"
                      : scopeResult.status === "partial"
                        ? "warning"
                        : "danger"
                  }
                >
                  <Chip.Label>
                    {t(
                      `versions.edit.backup.restore.status.${scopeResult.status}`,
                    )}
                  </Chip.Label>
                </Chip>
              </div>
              {scopeResult.errorCode ? (
                <div className="text-sm text-rose-500">
                  {vs.getErrorKey(scopeResult.errorCode)
                    ? t(vs.getErrorKey(scopeResult.errorCode))
                    : scopeResult.errorCode}
                </div>
              ) : null}
              {Array.isArray(scopeResult.warnings) &&
              scopeResult.warnings.length > 0 ? (
                <div className="space-y-2">
                  {scopeResult.warnings.map((warning) => (
                    <div
                      key={`${scopeResult.key}-${warning}`}
                      className="text-sm text-amber-600 dark:text-amber-300"
                    >
                      {warning.startsWith("ERR_")
                        ? t(vs.getErrorKey(warning))
                        : t(warning)}
                    </div>
                  ))}
                </div>
              ) : null}
              {Array.isArray(scopeResult.details) &&
              scopeResult.details.length > 0 ? (
                <div className="space-y-1">
                  {scopeResult.details.map((detail) => (
                    <div
                      key={`${scopeResult.key}-${detail}`}
                      className="font-mono text-xs break-all text-muted dark:text-zinc-400"
                    >
                      {detail}
                    </div>
                  ))}
                </div>
              ) : null}
            </ModalPanel>
          ))}
        </ModalDescription>
      </UnifiedModal>

      <UnifiedModal
        isOpen={vs.unregisterOpen}
        onOpenChange={(open) => {
          if (!open) vs.setUnregisterOpen(false);
        }}
        isDismissable={false}
        type="warning"
        title={t("versions.edit.unregister_progress.title")}
        icon={<FiAlertTriangle className="w-6 h-6" />}
      >
        <ModalDescription className="mb-4">
          {t("versions.edit.unregister_progress.body")}
        </ModalDescription>
        <ProgressBar size="sm" isIndeterminate aria-label="Unregistering">
          <ProgressBar.Track>
            <ProgressBar.Fill className={"bg-amber-500"} />
          </ProgressBar.Track>
        </ProgressBar>
      </UnifiedModal>

      <UnifiedModal
        isOpen={vs.unregisterSuccessOpen}
        onOpenChange={(open) => {
          if (!open) vs.setUnregisterSuccessOpen(false);
        }}
        size="standard"
        type="success"
        title={t("versions.edit.unregister_success.title")}
        icon={<FiCheckCircle className="w-6 h-6" />}
        onConfirm={() => {
          vs.setUnregisterSuccessOpen(false);
        }}
        confirmText={t("launcherpage.delete.complete.close_button")}
        showCancelButton={false}
      >
        <ModalDescription>
          {t("versions.edit.unregister_success.body")}
        </ModalDescription>
      </UnifiedModal>

      <UnifiedModal
        isOpen={vs.lipMissingOpen}
        onOpenChange={(open) => {
          if (!open) vs.setLipMissingOpen(false);
        }}
        type="warning"
        title={t("lip.guard.title")}
        isDismissable={false}
        footer={
          <ModalAction
            {...warningConfirmButtonProps}
            onPress={vs.openLipComponentsSettings}
            variant={"secondary"}
          >
            {t("settings.lip.startup_prompt.open_settings_button")}
          </ModalAction>
        }
      >
        <ModalDescription>
          {t("lip.guard.description")}
        </ModalDescription>
      </UnifiedModal>

      <UnifiedModal
        isOpen={vs.errorOpen}
        onOpenChange={(open) => {
          if (!open) vs.setErrorOpen(false);
        }}
        type="error"
        title={t("common.error")}
        icon={<FiAlertTriangle className="w-6 h-6" />}
        onConfirm={() => {
          vs.setError("");
          vs.setErrorOpen(false);
        }}
        confirmText={t("common.ok")}
        showCancelButton={false}
      >
        <div className="text-base font-medium text-rose-600 dark:text-rose-400 whitespace-pre-wrap break-words leading-7">
          {t(vs.getErrorKey(vs.error))}
        </div>
      </UnifiedModal>

      <DeleteConfirmModal
        isOpen={vs.deleteOpen}
        onOpenChange={vs.setDeleteOpen}
        onConfirm={vs.onDeleteConfirm}
        title={t("launcherpage.delete.confirm.title")}
        description={t("launcherpage.delete.confirm.content")}
        itemName={vs.targetName}
        isPending={vs.deleting}
      />

      <UnifiedModal
        isOpen={vs.deleteSuccessOpen}
        onOpenChange={(open) => {
          if (!open) vs.setDeleteSuccessOpen(open);
        }}
        size="standard"
        type="success"
        title={t("launcherpage.delete.complete.title")}
        icon={<FiCheckCircle className="w-6 h-6" />}
        onConfirm={() => {
          vs.setDeleteSuccessOpen(false);
          vs.navigate(vs.returnToPath);
        }}
        confirmText={t("launcherpage.delete.complete.close_button")}
        showCancelButton={false}
      >
        <ModalDescription>
          {t("launcherpage.delete.complete.content")}
          {vs.deleteSuccessMsg ? (
            <span className="font-mono text-foreground dark:text-zinc-200 font-bold">
              {" "}
              {vs.deleteSuccessMsg}
            </span>
          ) : null}
        </ModalDescription>
      </UnifiedModal>

      <UnifiedModal
        size="standard"
        isOpen={vs.unsavedOpen}
        onOpenChange={vs.unsavedOnOpenChange}
        type="warning"
        title={t("settings.unsaved.title")}
        cancelText={t("settings.unsaved.cancel")}
        confirmText={t("settings.unsaved.save")}
        showCancelButton
        onCancel={() => vs.unsavedOnClose()}
        onConfirm={async () => {
          const ok = await vs.onSave(vs.pendingNavPath);
          if (ok) {
            vs.unsavedOnClose();
          }
        }}
      >
        <ModalDescription>
          {t("versions.unsaved.body")}
        </ModalDescription>
      </UnifiedModal>

      <DeleteConfirmModal
        isOpen={vs.llUninstallConfirmOpen}
        onOpenChange={(open) => {
          if (open) {
            vs.openLLUninstallConfirm();
            return;
          }
          vs.closeLLUninstallConfirm();
        }}
        onConfirm={vs.confirmUninstallLL}
        title={t("versions.edit.loader.ll_remove_confirm_title")}
        description={t("versions.edit.loader.ll_remove_confirm_body")}
        itemName="LeviLamina"
        confirmDisabled={vs.llUninstallBlocked}
        warning={
          vs.llUninstallWarning ||
          t("versions.edit.loader.ll_remove_confirm_warning")
        }
        isPending={vs.uninstallingLL}
        confirmText={t("common.remove")}
      />

      <UnifiedModal
        isOpen={vs.demotedWarningOpen}
        onOpenChange={vs.demotedWarningOnOpenChange}
        title={t("common.tip")}
        type="warning"
        confirmText={t("common.confirm")}
        showCancelButton={false}
        onConfirm={vs.closeDemotedWarning}
      >
        <ModalDescription className="whitespace-pre-wrap">
          {t("errors.ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY")}
        </ModalDescription>
        {vs.demotedWarningNames.length > 0 ? (
          <ModalNotice tone="warning" className="whitespace-pre-wrap break-all font-mono">
            {vs.demotedWarningNames.join("\n")}
          </ModalNotice>
        ) : null}
      </UnifiedModal>

      <UnifiedModal
        size="standard"
        isOpen={vs.llVersionSelectOpen}
        onOpenChange={vs.llVersionSelectOnOpenChange}
        type="primary"
        title={t("versions.edit.loader.ll_select_version")}
        cancelText={t("common.cancel")}
        confirmText={t(vs.llInstallActionLabelKey)}
        showCancelButton
        onCancel={() => vs.llVersionSelectOnClose()}
        onConfirm={async () => {
          await vs.confirmLeviLaminaVersionSelect();
        }}
        confirmButtonProps={{
          isPending: vs.installingLL,
          isDisabled: !vs.selectedLLVersion || !vs.canInstallSelectedLLVersion,
        }}
      >
        <div className="space-y-3">
          <ModalDescription>
            {t("versions.edit.loader.ll_select_guidance")}
          </ModalDescription>
          <Select
            placeholder={
              t(
                "versions.edit.loader.ll_select_placeholder",
              ) as unknown as string
            }
            isDisabled={vs.installingLL || vs.uninstallingLL}
            value={
              Array.from(
                vs.selectedLLVersion
                  ? new Set([vs.selectedLLVersion])
                  : new Set([]),
              )[0] ?? null
            }
            onChange={(keys) => {
              const selected = keys;
              vs.setSelectedLLVersion(String(selected || ""));
            }}
          >
            <Label>
              {t("versions.edit.loader.ll_select_version") as unknown as string}
            </Label>
            <Select.Trigger className={COMPONENT_STYLES.select.trigger}>
              <Select.Value />
              <Select.Indicator />
            </Select.Trigger>
            <Select.Popover className={COMPONENT_STYLES.select.popoverContent}>
              <ListBox className={COMPONENT_STYLES.select.listbox}>
                {vs.llSupportedVersions.map((version: string) => (
                  <ListBox.Item
                    key={version}
                    isDisabled={false}
                    id={version}
                    textValue={version}
                  >
                    <Label>{version}</Label>
                    <ListBox.ItemIndicator />
                  </ListBox.Item>
                ))}
              </ListBox>
            </Select.Popover>
          </Select>
          {vs.llInstallBlockedReasonKey ? (
            <p className="text-xs text-amber-600 dark:text-amber-500">
              {t(vs.llInstallBlockedReasonKey)}
            </p>
          ) : null}
        </div>
      </UnifiedModal>

      <UnifiedModal
        size="standard"
        isOpen={vs.llInstallConfirmOpen}
        onOpenChange={(open) => {
          if (open) {
            vs.openLLInstallConfirm();
            return;
          }
          vs.closeLLInstallConfirm();
        }}
        type="primary"
        title={t("lip.package.confirm_install_title")}
        confirmText={t("common.confirm")}
        cancelText={t("common.cancel")}
        showCancelButton
        isDismissable={!vs.installingLL}
        onCancel={() => vs.closeLLInstallConfirm()}
        onConfirm={async () => {
          await vs.confirmInstallLeviLaminaAction();
        }}
        confirmButtonProps={{
          isPending: vs.installingLL,
          isDisabled: !vs.resolvedLLTargetVersion || vs.installingLL,
        }}
        cancelButtonProps={{
          isDisabled: vs.installingLL,
        }}
      >
        <ModalDescription className="whitespace-pre-wrap">
          {t("lip.package.confirm_install_body", {
            action: llInstallActionLabel,
            package: "LeviLamina",
            version: vs.resolvedLLTargetVersion || "-",
            instance: vs.targetName || "-",
          })}
        </ModalDescription>
      </UnifiedModal>

      <UnifiedModal
        size="standard"
        isOpen={vs.rcOpen}
        onOpenChange={vs.rcOnOpenChange}
        type="warning"
        title={t("mods.rc_warning.title")}
        icon={<FiAlertTriangle className="w-6 h-6" />}
        cancelText={t("common.cancel")}
        confirmText={t("common.continue")}
        showCancelButton
        onCancel={() => vs.rcOnClose()}
        onConfirm={() => {
          vs.rcOnClose();
          vs.proceedInstallLeviLamina(vs.selectedLLVersion);
        }}
      >
        <ModalDescription className="space-y-2">
          <p>
            {t("mods.rc_warning.body_1", {
              version: vs.rcVersion,
            })}
          </p>
          <p className="font-semibold text-amber-700 dark:text-amber-300">
            {t("mods.rc_warning.body_2")}
          </p>
          <p>{t("mods.rc_warning.body_3")}</p>
        </ModalDescription>
      </UnifiedModal>
    </PageContainer>
  );
}
