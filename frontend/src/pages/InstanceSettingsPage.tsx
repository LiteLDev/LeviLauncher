import { Dialogs } from "@wailsio/runtime";
import {
  UnifiedModal,
  getUnifiedModalConfirmButtonProps,
} from "@/components/UnifiedModal";
import { DeleteConfirmModal } from "@/components/DeleteConfirmModal";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { ROUTES } from "@/constants/routes";
import { formatBytes } from "@/utils/formatting";
import {
  Button,
  Card,
  CardBody,
  Checkbox,
  Input,
  Select,
  SelectItem,
  Switch,
  Chip,
  Progress,
  Textarea,
  Tabs,
  Tab,
  Divider,
} from "@heroui/react";
import { useTranslation } from "react-i18next";
import { motion } from "framer-motion";
import { FiAlertTriangle, FiCheckCircle } from "react-icons/fi";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import {
  GetVersionLogoDataUrl,
  RemoveVersionLogo,
  SaveVersionLogoFromPath,
  UnregisterVersionByName,
} from "bindings/github.com/liteldev/LeviLauncher/versionservice";
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
  String(value || "").replace(/\\/g, "/").toLowerCase();

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

const getScopeDisplayLabel = (scopeKey: string, fallbackLabel: string): string =>
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
  const restoreConflictGroups = (() => {
    const scopeOrder = new Map(
      vs.restoreScopes.map((scope, index) => [scope.key, index]),
    );
    const groups = new Map<
      string,
      RestoreConflictScopeGroup & {
        categoryMap: Map<RestoreConflictCategoryKey, RestoreConflictCategoryGroup>;
      }
    >();

    for (const conflict of vs.restoreConflicts as RestoreConflict[]) {
      const groupKey = String(conflict.scopeKey || conflict.scopeLabel || "other").trim();
      const selectedChoice = vs.restoreConflictChoices[conflict.id];
      const categoryKey = resolveRestoreConflictCategoryKey(conflict);
      const categoryLabelKey = RESTORE_CONFLICT_CATEGORY_LABEL_KEYS[categoryKey];
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
          initial={{ y: -20, opacity: 0 }}
          animate={{ y: 0, opacity: 1 }}
          transition={{ duration: 0.4 }}
        >
          <Card className={LAYOUT.GLASS_CARD.BASE}>
            <CardBody className="p-6 w-full">
              <PageHeader
                title={t("versions.edit.title")}
                titleClassName="text-left pb-1"
                description={
                  <div className="mt-1 text-xs text-default-500 dark:text-zinc-400 truncate text-left">
                    {t("versions.edit.mc_version")}:{" "}
                    <span className="text-default-700 dark:text-zinc-200 font-medium">
                      {vs.loading ? (
                        <span className="inline-block h-4 w-24 rounded bg-default-200 animate-pulse" />
                      ) : (
                        vs.gameVersion ||
                        (t(
                          "launcherpage.version_select.unknown",
                        ) as unknown as string)
                      )}
                    </span>
                    <span className="mx-2 text-default-400">·</span>
                    {t("versions.info.name")}:{" "}
                    <span className="text-default-700 dark:text-zinc-200 font-medium">
                      {vs.targetName || "-"}
                    </span>
                    <span className="mx-2 text-default-400">·</span>
                    {vs.versionType === "preview" ? (
                      <Chip size="sm" variant="flat" color="warning">
                        {t("common.preview")}
                      </Chip>
                    ) : vs.versionType === "release" ? (
                      <span className="text-default-700 dark:text-zinc-300">
                        {t("common.release")}
                      </span>
                    ) : (
                      <Chip size="sm" variant="flat" color="secondary">
                        {vs.versionType || "-"}
                      </Chip>
                    )}
                  </div>
                }
                endContent={
                  <div className="hidden sm:flex items-center gap-3">
                    <Button
                      variant="light"
                      radius="full"
                      onPress={() => vs.navigate(vs.returnToPath)}
                      className="font-medium text-default-600 dark:text-zinc-300"
                    >
                      {t("common.cancel")}
                    </Button>
                    <Button
                      color="primary"
                      radius="full"
                      className="bg-primary-500 hover:bg-primary-500 text-white font-bold shadow-lg shadow-primary-900/20"
                      onPress={() => vs.onSave()}
                    >
                      {t("common.ok")}
                    </Button>
                  </div>
                }
              />
              <Tabs
                aria-label="Version Settings Tabs"
                selectedKey={vs.selectedTab}
                onSelectionChange={(k) => vs.setSelectedTab(k as string)}
                variant="solid"
                classNames={{
                  ...COMPONENT_STYLES.tabs,
                  base: "mt-4",
                }}
              >
                <Tab key="general" title={t("versions.edit.tabs.general")} />
                <Tab key="launch" title={t("versions.edit.tabs.launch")} />
                <Tab key="loader" title={t("versions.edit.tabs.loader")} />
                <Tab key="features" title={t("versions.edit.tabs.features")} />
                <Tab key="manage" title={t("versions.edit.tabs.manage")} />
              </Tabs>
            </CardBody>
          </Card>
        </motion.div>

        <motion.div
          key={vs.selectedTab}
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3 }}
        >
          <Card className={LAYOUT.GLASS_CARD.BASE}>
            <CardBody className="p-6">
              {vs.selectedTab === "general" && (
                <div className="flex flex-col gap-6">
                  <div>
                    <label className="text-small font-medium text-default-700 dark:text-zinc-200 mb-2 block">
                      {t("versions.edit.new_name")}
                    </label>
                    <Input
                      value={vs.newName}
                      onValueChange={(v) => {
                        vs.setNewName(v);
                        if (vs.error) vs.setError("");
                      }}
                      size="md"
                      variant="bordered"
                      radius="lg"
                      classNames={COMPONENT_STYLES.input}
                      isDisabled={vs.isRegistered || vs.loading}
                      placeholder={
                        t("versions.edit.placeholder") as unknown as string
                      }
                    />
                    <p className="text-tiny text-default-400 mt-2">
                      {t("versions.edit.hint")}
                    </p>
                  </div>
                  <div className="flex flex-col gap-3">
                    <div className="text-small font-medium text-default-700 dark:text-zinc-200">
                      {t("versions.logo.title")}
                    </div>
                    <div className="flex items-center gap-4">
                      <div
                        className="relative h-24 w-24 rounded-2xl overflow-hidden bg-default-100 flex items-center justify-center border border-default-200 cursor-pointer group transition-all hover:scale-105 hover:shadow-lg"
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
                        <div className="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 transition flex items-center justify-center text-white text-tiny font-medium backdrop-blur-[2px]">
                          {t("versions.logo.change")}
                        </div>
                      </div>
                      <div className="flex flex-col gap-2">
                        <Button
                          size="sm"
                          variant="flat"
                          color="danger"
                          radius="full"
                          className="px-4 font-medium"
                          onPress={async () => {
                            try {
                              const rm = RemoveVersionLogo as any;
                              if (typeof rm === "function") {
                                await rm(vs.targetName);
                              }
                            } catch {}
                            vs.setLogoDataUrl("");
                          }}
                        >
                          {t("versions.logo.clear")}
                        </Button>
                      </div>
                    </div>
                    <p className="text-tiny text-default-400">
                      {t("versions.logo.hint")}
                    </p>
                  </div>
                </div>
              )}
              {vs.selectedTab === "launch" && (
                <div className="flex flex-col gap-6">
                  <div className="flex items-center justify-between p-2 rounded-xl">
                    <div className="text-medium font-medium">
                      {t("versions.edit.enable_isolation")}
                    </div>
                    <Switch
                      size="md"
                      color="success"
                      isSelected={vs.enableIsolation}
                      onValueChange={vs.setEnableIsolation}
                      classNames={{
                        wrapper: "group-data-[selected=true]:bg-primary-500",
                      }}
                    />
                  </div>
                  <div className="flex items-center justify-between p-2 rounded-xl">
                    <div className="text-medium font-medium">
                      {t("versions.edit.enable_console")}
                    </div>
                    <Switch
                      size="md"
                      color="success"
                      isSelected={vs.enableConsole}
                      onValueChange={vs.setEnableConsole}
                      classNames={{
                        wrapper: "group-data-[selected=true]:bg-primary-500",
                      }}
                    />
                  </div>
                  <div className="flex flex-col gap-3">
                    <label className="text-small font-medium text-default-700 dark:text-zinc-200 block">
                      {t("versions.edit.launch_args")}
                    </label>
                    <Input
                      value={vs.launchArgs}
                      onValueChange={(v) => {
                        vs.setLaunchArgs(v);
                        if (vs.error) vs.setError("");
                      }}
                      size="md"
                      variant="flat"
                      radius="lg"
                      placeholder={
                        t(
                          "versions.edit.launch_args_placeholder",
                        ) as unknown as string
                      }
                      classNames={COMPONENT_STYLES.input}
                    />
                    <p className="text-tiny text-default-400">
                      {t("versions.edit.launch_args_hint")}
                    </p>
                  </div>
                  <div className="flex flex-col gap-3">
                    <label className="text-small font-medium text-default-700 dark:text-zinc-200 block">
                      {t("versions.edit.env_vars")}
                    </label>
                    <Textarea
                      value={vs.envVars}
                      onValueChange={(v) => {
                        vs.setEnvVars(v);
                        if (vs.error) vs.setError("");
                      }}
                      minRows={3}
                      variant="bordered"
                      radius="lg"
                      placeholder={
                        t(
                          "versions.edit.env_vars_placeholder",
                        ) as unknown as string
                      }
                      classNames={COMPONENT_STYLES.input}
                    />
                    <p className="text-tiny text-default-400">
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
                          <div className="text-medium font-bold text-foreground">
                            LeviLamina
                          </div>
                          <span
                            className={
                              vs.currentLLVersion
                                ? "text-small font-semibold text-default-500 dark:text-zinc-400"
                                : "text-small font-medium text-default-400 dark:text-zinc-500"
                            }
                          >
                            {currentLLVersionText}
                          </span>
                        </div>
                        <div className="text-small text-default-500 dark:text-zinc-400">
                          {t("downloadpage.install.levilamina_desc")}
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-3">
                      {vs.isLLSupported(vs.gameVersion) ? (
                        <>
                          {vs.isLLInstalled && (
                            <Button
                              color="danger"
                              variant="flat"
                              onPress={vs.openLLUninstallConfirm}
                              isDisabled={vs.installingLL || vs.uninstallingLL}
                              isLoading={vs.uninstallingLL}
                            >
                              {t("common.remove")}
                            </Button>
                          )}
                          <Button
                            color="success"
                            variant="flat"
                            className="bg-primary-500/10 text-primary-600 dark:text-primary-400 font-bold"
                            onPress={vs.openLeviLaminaVersionSelect}
                            isDisabled={
                              vs.installingLL ||
                              vs.uninstallingLL ||
                              vs.llSupportedVersions.length === 0
                            }
                            isLoading={vs.installingLL}
                          >
                            {t(vs.llInstallActionLabelKey)}
                          </Button>
                        </>
                      ) : (
                        <div className="text-small text-default-400 italic">
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
                    <div className="text-medium font-medium">
                      {t("versions.edit.enable_render_dragon")}
                    </div>
                    <Switch
                      size="md"
                      color="success"
                      isSelected={vs.enableRenderDragon}
                      onValueChange={vs.setEnableRenderDragon}
                      classNames={{
                        wrapper: "group-data-[selected=true]:bg-primary-500",
                      }}
                    />
                  </div>
                  <div className="flex items-center justify-between p-2 rounded-xl">
                    <div className="text-medium font-medium">
                      {t("versions.edit.enable_ctrl_r_reload_resources")}
                    </div>
                    <Switch
                      size="md"
                      color="success"
                      isSelected={vs.enableCtrlRReloadResources}
                      onValueChange={vs.setEnableCtrlRReloadResources}
                      classNames={{
                        wrapper: "group-data-[selected=true]:bg-primary-500",
                      }}
                    />
                  </div>
                  <div className="flex items-center justify-between p-2 rounded-xl">
                    <div className="text-medium font-medium">
                      {t("versions.edit.enable_editor_mode")}
                    </div>
                    <Switch
                      size="md"
                      color="success"
                      isSelected={vs.enableEditorMode}
                      onValueChange={vs.setEnableEditorMode}
                      classNames={{
                        wrapper: "group-data-[selected=true]:bg-primary-500",
                      }}
                    />
                  </div>
                </div>
              )}
              {vs.selectedTab === "manage" && (
                <div className="flex flex-col gap-6">
                  <section className="flex flex-col gap-3">
                    <div className="flex flex-col gap-1">
                      <p className="font-medium text-default-700 dark:text-zinc-200">
                        {t("versions.edit.backup.title")}
                      </p>
                      <p className="text-tiny text-default-500 dark:text-zinc-400 max-w-2xl">
                        {t("versions.edit.backup.hint", {
                          gameDataLabel: vs.backupGameDataLabel,
                        })}
                      </p>
                    </div>
                    <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
                      <div className="flex flex-col gap-1 max-w-2xl">
                        <p className="text-small text-default-600 dark:text-zinc-300 leading-7">
                          {t("versions.edit.backup.section_body")}
                        </p>
                      </div>
                      <div className="shrink-0 flex flex-wrap gap-3">
                        <Button
                          variant="flat"
                          radius="lg"
                          className="font-medium"
                          isDisabled={!vs.targetName || vs.loading}
                          isLoading={
                            vs.restoreInfoLoading || vs.restoringInstance
                          }
                          onPress={vs.openInstanceRestore}
                        >
                          {t("versions.edit.backup.restore.button")}
                        </Button>
                        <Button
                          color="primary"
                          radius="lg"
                          className="font-medium"
                          isDisabled={!vs.targetName || vs.loading}
                          isLoading={
                            vs.backupInfoLoading || vs.backingUpInstance
                          }
                          onPress={vs.openInstanceBackup}
                        >
                          {t("versions.edit.backup.button")}
                        </Button>
                      </div>
                    </div>
                  </section>

                  <Divider className="bg-default-200/50" />

                  <section className="flex flex-col gap-3">
                    <div className="flex flex-col gap-1">
                      <p className="font-medium text-default-700 dark:text-zinc-200">
                        {t("versions.edit.danger_title")}
                      </p>
                      <p className="text-tiny text-default-500 dark:text-zinc-400 max-w-2xl">
                        {t("versions.edit.danger_hint")}
                      </p>
                    </div>
                    <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
                      <div className="flex flex-col gap-2 max-w-2xl">
                        <p className="font-medium text-default-700 dark:text-zinc-200">
                          {vs.isRegistered
                            ? t("versions.edit.unregister_button")
                            : t("common.delete")}
                        </p>
                        <p className="text-small text-default-500 dark:text-zinc-400 leading-7">
                          {vs.isRegistered
                            ? t("versions.edit.unregister_hint")
                            : t("versions.edit.delete_hint")}
                        </p>
                      </div>
                      <div className="shrink-0">
                        {vs.isRegistered ? (
                          <Button
                            color="warning"
                            variant="flat"
                            radius="lg"
                            isDisabled={vs.loading}
                            className="font-medium"
                            onPress={async () => {
                              try {
                                const has = await (
                                  minecraft as any
                                )?.IsGDKInstalled?.();
                                if (!has) {
                                  vs.setUnregisterOpen(false);
                                  vs.setGdkMissingOpen(true);
                                  return;
                                }
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
                          >
                            {t("versions.edit.unregister_button")}
                          </Button>
                        ) : (
                          <Button
                            color="danger"
                            variant="flat"
                            radius="lg"
                            className="font-medium"
                            onPress={() => vs.setDeleteOpen(true)}
                          >
                            {t("common.delete")}
                          </Button>
                        )}
                      </div>
                    </div>
                  </section>
                </div>
              )}
            </CardBody>
          </Card>
        </motion.div>
      </div>

      <UnifiedModal
        isOpen={vs.backupOpen}
        onOpenChange={vs.onInstanceBackupOpenChange}
        size="lg"
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
          isLoading: vs.backingUpInstance,
        }}
      >
        <div className="space-y-4">
          <p className="text-small leading-6 text-default-500 dark:text-zinc-400">
            {t("versions.edit.backup.dialog_body")}
          </p>
          {vs.backupHasSharedScope ? (
            <div className="rounded-2xl border border-warning-200/70 dark:border-warning-500/30 bg-warning-50/80 dark:bg-warning-500/10 px-4 py-3 text-sm text-warning-700 dark:text-warning-300">
              {t("versions.edit.backup.shared_warning")}
            </div>
          ) : null}
          {vs.backupFullModeSelected ? (
            <div className="rounded-2xl border border-danger-200/70 dark:border-danger-500/30 bg-danger-50/80 dark:bg-danger-500/10 px-4 py-3 text-sm text-danger-700 dark:text-danger-300">
              {t("versions.edit.backup.mode.full.warning")}
            </div>
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
                      ? "border-primary-300 dark:border-primary-500/40 bg-primary-50/70 dark:bg-primary-500/10"
                      : "border-default-200/70 dark:border-white/10 bg-default-50/40 dark:bg-white/5"
                  }`}
                >
                  <div className="flex items-start gap-3">
                    <Checkbox
                      isSelected={isSelected}
                      isDisabled={!scope.selectable || vs.backingUpInstance}
                      onValueChange={(selected) => {
                        vs.setBackupScopeSelected(scope.key, selected);
                      }}
                    />
                    <div className="min-w-0 flex-1 space-y-3">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="text-medium font-semibold text-default-900 dark:text-zinc-100">
                          {displayLabel}
                        </span>
                        {isSelected && selectedMode?.key ? (
                          <Chip size="sm" variant="flat" color="primary">
                            {t(
                              `versions.edit.backup.mode.${selectedMode.key}.label`,
                            )}
                          </Chip>
                        ) : null}
                        {scope.shared ? (
                          <Chip size="sm" variant="flat" color="warning">
                            {t("versions.edit.backup.shared_tag")}
                          </Chip>
                        ) : null}
                        {!scope.exists ? (
                          <Chip size="sm" variant="flat" color="danger">
                            {t("versions.edit.backup.missing_tag")}
                          </Chip>
                        ) : null}
                        {scope.exists && displaySize === 0 ? (
                          <Chip size="sm" variant="flat">
                            {t("versions.edit.backup.empty_tag")}
                          </Chip>
                        ) : null}
                      </div>
                      {Array.isArray(scope.modes) && scope.modes.length > 0 ? (
                        <Select
                          size="sm"
                          variant="bordered"
                          radius="lg"
                          selectedKeys={
                            selectedMode?.key
                              ? new Set([selectedMode.key])
                              : new Set()
                          }
                          disallowEmptySelection
                          isDisabled={!isSelected || vs.backingUpInstance}
                          className="max-w-sm"
                          onSelectionChange={(keys) => {
                            const nextValue = Array.from(keys)[0];
                            if (typeof nextValue === "string") {
                              vs.setBackupScopeMode(scope.key, nextValue);
                            }
                          }}
                        >
                          {scope.modes.map((mode) => (
                            <SelectItem
                              key={mode.key}
                              textValue={
                                t(
                                  `versions.edit.backup.mode.${mode.key}.label`,
                                ) as string
                              }
                            >
                              {t(`versions.edit.backup.mode.${mode.key}.label`)}
                            </SelectItem>
                          ))}
                        </Select>
                      ) : null}
                      {isSelected && selectedMode?.warning ? (
                        <div className="rounded-2xl border border-danger-200/70 dark:border-danger-500/30 bg-danger-50/80 dark:bg-danger-500/10 px-3 py-2 text-tiny text-danger-700 dark:text-danger-300">
                          {t(selectedMode.warning)}
                        </div>
                      ) : null}
                      {isSelected &&
                      scope.key === "mods" &&
                      vs.backupLipPackageCount > 0 ? (
                        <div className="rounded-2xl border border-primary-200/70 dark:border-primary-500/30 bg-primary-50/80 dark:bg-primary-500/10 px-3 py-2 text-tiny text-primary-700 dark:text-primary-300">
                          {t("versions.edit.backup.lip_summary", {
                            count: vs.backupLipPackageCount,
                            summary: vs.backupLipPackageSummary || "-",
                          })}
                        </div>
                      ) : null}
                      <div className="space-y-2 text-tiny text-default-500 dark:text-zinc-400">
                        <div className="break-all">
                          {t("versions.edit.backup.path_label")}:{" "}
                          <span className="font-mono text-[11px] text-default-700 dark:text-zinc-300">
                            {displayPath}
                          </span>
                        </div>
                        <div>
                          {t("versions.edit.backup.size_label")}:{" "}
                          <span className="text-default-700 dark:text-zinc-300 font-medium">
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
            <div className="text-sm text-danger-500">
              {t("versions.edit.backup.no_scope_selected")}
            </div>
          ) : null}
        </div>
      </UnifiedModal>

      <UnifiedModal
        isOpen={vs.backingUpInstance}
        onOpenChange={() => {}}
        hideCloseButton
        isDismissable={false}
        type="primary"
        title={t("versions.edit.backup.progress_title")}
      >
        <div className="text-medium font-medium text-default-600 dark:text-zinc-300 mb-4">
          {t("versions.edit.backup.progress_body")}
        </div>
        <Progress
          size="sm"
          isIndeterminate
          aria-label="Backing up instance"
          classNames={{ indicator: "bg-primary-500" }}
        />
      </UnifiedModal>

      <UnifiedModal
        isOpen={vs.restoreOpen}
        onOpenChange={vs.onInstanceRestoreOpenChange}
        size="5xl"
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
          isLoading: vs.restoringInstance,
        }}
      >
        <div className="space-y-4">
          <p className="text-small leading-6 text-default-500 dark:text-zinc-400">
            {t("versions.edit.backup.restore.dialog_body")}
          </p>
          <div className="rounded-2xl bg-default-50/70 dark:bg-zinc-800/80 border border-default-200/70 dark:border-white/10 px-4 py-3">
            <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-start">
              <div className="min-w-0">
                <div className="text-tiny text-default-500 dark:text-zinc-400 mb-1">
                  {t("versions.edit.backup.archive_label")}
                </div>
                <div className="font-mono text-small break-all text-default-800 dark:text-zinc-100">
                  {vs.restoreArchiveInfo?.archiveName || "-"}
                </div>
              </div>
              <div className="rounded-2xl bg-default-100/55 dark:bg-white/5 border border-default-200/60 dark:border-white/10 px-3 py-2">
                <div className="text-tiny text-default-500 dark:text-zinc-400">
                  {t("versions.edit.backup.restore.created_at")}
                </div>
                <div className="text-sm font-medium text-default-700 dark:text-zinc-300">
                  {vs.restoreArchiveCreatedAtText || "-"}
                </div>
              </div>
            </div>
          </div>
          {vs.restoreHasHighRiskScope ? (
            <div className="rounded-2xl border border-danger-200/70 dark:border-danger-500/30 bg-danger-50/80 dark:bg-danger-500/10 px-4 py-3 text-sm text-danger-700 dark:text-danger-300">
              {t("versions.edit.backup.mode.full.warning")}
            </div>
          ) : null}
          <div className="rounded-2xl border border-default-200/70 dark:border-white/10 bg-default-50/25 dark:bg-white/5 px-4 py-4 space-y-3">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm font-semibold text-default-800 dark:text-zinc-100">
                {t("versions.edit.backup.restore.scope_section_title")}
              </span>
              <Chip size="sm" variant="flat">
                {vs.restoreScopes.length}
              </Chip>
            </div>
            <div className="grid gap-3 md:grid-cols-2">
              {vs.restoreScopes.map((scope) => {
                const isSelected = vs.selectedRestoreScopeSet.has(scope.key);
                const displayLabel = getScopeDisplayLabel(scope.key, scope.label);
                return (
                  <div
                    key={scope.key}
                    className={`rounded-2xl border p-4 transition-colors ${
                      isSelected
                        ? "border-primary-300 dark:border-primary-500/40 bg-primary-50/70 dark:bg-primary-500/10"
                        : "border-default-200/70 dark:border-white/10 bg-default-50/40 dark:bg-white/5"
                    }`}
                  >
                    <div className="flex items-start gap-3">
                      <Checkbox
                        isSelected={isSelected}
                        isDisabled={vs.restoringInstance}
                        onValueChange={(selected) => {
                          vs.setRestoreScopeSelected(scope.key, selected);
                        }}
                      />
                      <div className="min-w-0 flex-1 space-y-3">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="text-medium font-semibold text-default-900 dark:text-zinc-100">
                            {displayLabel}
                          </span>
                          {scope.mode ? (
                            <Chip size="sm" variant="flat" color="primary">
                              {t(`versions.edit.backup.mode.${scope.mode}.label`)}
                            </Chip>
                          ) : null}
                        </div>
                        {scope.key === "mods" &&
                        vs.restoreArchiveInfo?.modsLipPackages?.length ? (
                          <div className="rounded-xl border border-default-200/70 dark:border-white/10 bg-white/55 dark:bg-white/5 px-3 py-2.5">
                            <div className="flex flex-wrap items-center">
                              <span className="text-tiny leading-6 text-default-600 dark:text-zinc-300">
                                {t("versions.edit.backup.restore.lip_summary", {
                                  count: vs.restoreArchiveInfo.modsLipPackages.length,
                                })}
                              </span>
                            </div>
                          </div>
                        ) : null}
                        {Array.isArray(scope.warnings) &&
                        scope.warnings.length > 0 ? (
                          <div className="space-y-2">
                            {scope.warnings.map((warning) => (
                              <div
                                key={`${scope.key}-${warning}`}
                                className="rounded-2xl border border-danger-200/70 dark:border-danger-500/30 bg-danger-50/80 dark:bg-danger-500/10 px-3 py-2 text-tiny text-danger-700 dark:text-danger-300"
                              >
                                {t(warning)}
                              </div>
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
                <div className="rounded-2xl border border-default-200/70 dark:border-white/10 bg-default-50/60 dark:bg-white/5 px-4 py-3 text-sm text-default-600 dark:text-zinc-300">
                  {t("versions.edit.backup.restore.conflict_loading")}
                </div>
              ) : null}
              {!vs.restoreConflictLoading && vs.restoreConflicts.length > 0 ? (
                <>
                  <div className="rounded-2xl border border-warning-200/70 dark:border-warning-500/30 bg-warning-50/80 dark:bg-warning-500/10 px-4 py-3 text-sm text-warning-700 dark:text-warning-300">
                    {t("versions.edit.backup.restore.conflict_summary", {
                      count: vs.restoreConflicts.length,
                    })}
                  </div>
                  <div className="flex flex-col gap-3">
                    {restoreConflictGroups.map((group) => (
                      <div
                        key={group.key}
                        className="rounded-[28px] border border-default-200/70 dark:border-white/10 bg-default-50/40 dark:bg-white/5 p-4 sm:p-5 space-y-4"
                      >
                        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
                          <div className="min-w-0 space-y-2">
                            <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
                              <span className="text-base font-semibold text-default-900 dark:text-zinc-100">
                                {group.label}
                              </span>
                              <span className="text-sm font-medium text-default-700 dark:text-zinc-300">
                                {t(
                                  "versions.edit.backup.restore.conflict_group_count",
                                  { count: group.conflicts.length },
                                )}
                              </span>
                              {group.unresolvedCount > 0 ? (
                                <span className="text-sm text-default-500 dark:text-zinc-400">
                                  {t(
                                    "versions.edit.backup.restore.conflict_group_unresolved",
                                    { count: group.unresolvedCount },
                                  )}
                                </span>
                              ) : (
                                <span className="text-sm text-default-500 dark:text-zinc-400">
                                  {t(
                                    "versions.edit.backup.restore.conflict_group_resolved",
                                  )}
                                </span>
                              )}
                            </div>
                            <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-default-500 dark:text-zinc-400">
                              {group.categories.map((category, index) => (
                                <span
                                  key={`${group.key}-${category.key}`}
                                  className="contents"
                                >
                                  {index > 0 ? (
                                    <span className="text-default-300 dark:text-zinc-600">
                                      ·
                                    </span>
                                  ) : null}
                                  <span>
                                    {t(category.labelKey)} {category.conflicts.length}
                                  </span>
                                </span>
                              ))}
                            </div>
                          </div>
                          <div className="flex flex-wrap gap-2">
                            <Button
                              size="sm"
                              variant="bordered"
                              radius="full"
                              isDisabled={vs.restoringInstance}
                              className="min-w-[132px] border-default-300 bg-default-100 px-4 text-default-700 shadow-sm transition-colors hover:bg-default-200 dark:border-white/15 dark:bg-white/10 dark:text-zinc-300 dark:hover:bg-white/15"
                              onPress={() =>
                                vs.setRestoreConflictChoicesBulk(
                                  group.conflicts.map((conflict) => conflict.id),
                                  "backup",
                                )
                              }
                            >
                              {t(
                                "versions.edit.backup.restore.conflict_group_apply_backup",
                              )}
                            </Button>
                            <Button
                              size="sm"
                              variant="bordered"
                              radius="full"
                              isDisabled={vs.restoringInstance}
                              className="min-w-[132px] border-default-300 bg-default-100 px-4 text-default-700 shadow-sm transition-colors hover:bg-default-200 dark:border-white/15 dark:bg-white/10 dark:text-zinc-300 dark:hover:bg-white/15"
                              onPress={() =>
                                vs.setRestoreConflictChoicesBulk(
                                  group.conflicts.map((conflict) => conflict.id),
                                  "current",
                                )
                              }
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
                                <div className="flex flex-wrap items-center gap-2 text-sm text-default-500 dark:text-zinc-400">
                                  <span className="font-medium">
                                    {t(category.labelKey)}
                                  </span>
                                  <span className="text-default-300 dark:text-zinc-600">
                                    ·
                                  </span>
                                  <span>
                                    {category.conflicts.length}
                                  </span>
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
                                      className="rounded-2xl border border-default-200/70 dark:border-white/10 bg-white/70 dark:bg-zinc-900/20 p-4 space-y-3"
                                    >
                                      <div className="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
                                        <div className="min-w-0 space-y-2">
                                          <div className="flex flex-wrap items-center gap-2">
                                            <span className="text-sm font-semibold text-default-900 dark:text-zinc-100 break-all">
                                              {title}
                                            </span>
                                            <Chip
                                              size="sm"
                                              variant="flat"
                                              className="bg-default-100 text-default-700 dark:bg-white/10 dark:text-zinc-300"
                                            >
                                              {t(
                                                `versions.edit.backup.restore.identity_kind.${conflict.identityKind}`,
                                              )}
                                            </Chip>
                                            {conflict.sourceType !==
                                            conflict.targetType ? (
                                              <Chip size="sm" variant="flat">
                                                {t(
                                                  `versions.edit.backup.restore.conflict_type.${conflict.sourceType}`,
                                                )}
                                                {" -> "}
                                                {t(
                                                  `versions.edit.backup.restore.conflict_type.${conflict.targetType}`,
                                                )}
                                              </Chip>
                                            ) : null}
                                            {selectedChoice !== "backup" &&
                                            selectedChoice !== "current" ? (
                                              <Chip
                                                size="sm"
                                                variant="flat"
                                                className="bg-default-100 text-default-700 dark:bg-white/10 dark:text-zinc-300"
                                              >
                                                {t(
                                                  "versions.edit.backup.restore.conflict_choice_pending",
                                                )}
                                              </Chip>
                                            ) : null}
                                          </div>
                                          <div className="flex flex-wrap gap-2 text-xs text-default-500 dark:text-zinc-400">
                                            {Array.isArray(conflict.diffFields)
                                              ? conflict.diffFields
                                                  .slice(0, 3)
                                                  .map((field) => (
                                                    <Chip
                                                      key={`${conflict.id}-${field.key}`}
                                                      size="sm"
                                                      variant="flat"
                                                    >
                                                      {t(field.label)}
                                                    </Chip>
                                                  ))
                                              : null}
                                            {Array.isArray(conflict.diffFields) &&
                                            conflict.diffFields.length > 3 ? (
                                              <Chip size="sm" variant="flat">
                                                +{conflict.diffFields.length - 3}
                                              </Chip>
                                            ) : null}
                                          </div>
                                        </div>
                                        <div className="w-full xl:max-w-xs">
                                          <Select
                                            size="sm"
                                            variant="bordered"
                                            radius="lg"
                                            selectedKeys={
                                              selectedChoice
                                                ? new Set([selectedChoice])
                                                : new Set()
                                            }
                                            placeholder={
                                              t(
                                                "versions.edit.backup.restore.conflict_choice_placeholder",
                                              ) as string
                                            }
                                            isDisabled={vs.restoringInstance}
                                            onSelectionChange={(keys) => {
                                              const nextValue = Array.from(keys)[0];
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
                                            <SelectItem
                                              key="backup"
                                              textValue={
                                                t(
                                                  "versions.edit.backup.restore.conflict_choice_backup",
                                                ) as string
                                              }
                                            >
                                              {t(
                                                "versions.edit.backup.restore.conflict_choice_backup",
                                              )}
                                            </SelectItem>
                                            <SelectItem
                                              key="current"
                                              textValue={
                                                t(
                                                  "versions.edit.backup.restore.conflict_choice_current",
                                                ) as string
                                              }
                                            >
                                              {t(
                                                "versions.edit.backup.restore.conflict_choice_current",
                                              )}
                                            </SelectItem>
                                          </Select>
                                        </div>
                                      </div>
                                      {conflict.backupSummary ||
                                      conflict.currentSummary ? (
                                        <div className="grid gap-2 lg:grid-cols-2">
                                          <div className="rounded-2xl border border-default-200/70 dark:border-white/10 bg-default-50/70 dark:bg-white/5 px-3 py-3 space-y-2">
                                            <div className="text-[11px] uppercase tracking-wide text-default-500 dark:text-zinc-400">
                                              {t(
                                                "versions.edit.backup.restore.backup_side_label",
                                              )}
                                            </div>
                                            <div className="text-sm font-medium text-default-800 dark:text-zinc-100 break-all">
                                              {conflict.backupSummary || "-"}
                                            </div>
                                          </div>
                                          <div className="rounded-2xl border border-default-200/70 dark:border-white/10 bg-default-50/70 dark:bg-white/5 px-3 py-3 space-y-2">
                                            <div className="text-[11px] uppercase tracking-wide text-default-500 dark:text-zinc-400">
                                              {t(
                                                "versions.edit.backup.restore.current_side_label",
                                              )}
                                            </div>
                                            <div className="text-sm font-medium text-default-800 dark:text-zinc-100 break-all">
                                              {conflict.currentSummary || "-"}
                                            </div>
                                          </div>
                                        </div>
                                      ) : null}
                                      {canShowDetails ? (
                                        <details className="rounded-2xl border border-default-200/70 dark:border-white/10 bg-default-50/60 dark:bg-white/5 px-3 py-3">
                                          <summary className="cursor-pointer text-xs font-medium text-default-600 dark:text-zinc-300 select-none">
                                            {t(
                                              "versions.edit.backup.restore.conflict_details_toggle",
                                            )}
                                          </summary>
                                          <div className="mt-3 space-y-3">
                                            {conflict.identityKey ? (
                                              <div className="text-xs text-default-500 dark:text-zinc-400 break-all">
                                                {t(
                                                  "versions.edit.backup.restore.identity_key_label",
                                                )}
                                                :{" "}
                                                <span className="font-mono text-default-700 dark:text-zinc-300">
                                                  {conflict.identityKey}
                                                </span>
                                              </div>
                                            ) : null}
                                            {conflict.backupPath ||
                                            conflict.currentPath ? (
                                              <div className="grid gap-3 md:grid-cols-2">
                                                <div className="rounded-2xl border border-default-200/70 dark:border-white/10 bg-default-100/60 dark:bg-zinc-800/70 px-3 py-3 space-y-2">
                                                  <div className="text-tiny uppercase tracking-wide text-default-500 dark:text-zinc-400">
                                                    {t(
                                                      "versions.edit.backup.restore.backup_path_label",
                                                    )}
                                                  </div>
                                                  <div className="font-mono text-xs text-default-700 dark:text-zinc-300 break-all">
                                                    {conflict.backupPath || "-"}
                                                  </div>
                                                </div>
                                                <div className="rounded-2xl border border-default-200/70 dark:border-white/10 bg-default-100/60 dark:bg-zinc-800/70 px-3 py-3 space-y-2">
                                                  <div className="text-tiny uppercase tracking-wide text-default-500 dark:text-zinc-400">
                                                    {t(
                                                      "versions.edit.backup.restore.current_path_label",
                                                    )}
                                                  </div>
                                                  <div className="font-mono text-xs text-default-700 dark:text-zinc-300 break-all">
                                                    {conflict.currentPath || "-"}
                                                  </div>
                                                </div>
                                              </div>
                                            ) : null}
                                            {Array.isArray(conflict.diffFields) &&
                                            conflict.diffFields.length > 0 ? (
                                              <div className="space-y-2">
                                                <Divider />
                                                {conflict.diffFields.map(
                                                  (field) => (
                                                    <div
                                                      key={`${conflict.id}-${field.key}`}
                                                      className="rounded-2xl border border-default-200/70 dark:border-white/10 bg-default-50/40 dark:bg-white/5 px-3 py-3 space-y-2"
                                                    >
                                                      <div className="text-xs font-semibold text-default-700 dark:text-zinc-200">
                                                        {t(field.label)}
                                                      </div>
                                                      <div className="grid gap-2 md:grid-cols-2">
                                                        <div className="rounded-xl bg-default-100/80 dark:bg-white/5 px-3 py-2">
                                                          <div className="text-[11px] uppercase tracking-wide text-default-500 dark:text-zinc-400">
                                                            {t(
                                                              "versions.edit.backup.restore.backup_side_label",
                                                            )}
                                                          </div>
                                                          <div className="text-sm text-default-800 dark:text-zinc-100 break-all font-mono">
                                                            {field.backupValue || "-"}
                                                          </div>
                                                        </div>
                                                        <div className="rounded-xl bg-default-100/80 dark:bg-white/5 px-3 py-2">
                                                          <div className="text-[11px] uppercase tracking-wide text-default-500 dark:text-zinc-400">
                                                            {t(
                                                              "versions.edit.backup.restore.current_side_label",
                                                            )}
                                                          </div>
                                                          <div className="text-sm text-default-800 dark:text-zinc-100 break-all font-mono">
                                                            {field.currentValue || "-"}
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
                    <div className="text-sm text-danger-500">
                      {t("versions.edit.backup.restore.conflict_unresolved")}
                    </div>
                  ) : null}
                </>
              ) : null}
              {!vs.restoreConflictLoading &&
              vs.selectedRestoreScopes.length > 0 &&
              vs.restoreConflicts.length === 0 ? (
                <div className="rounded-2xl border border-success-200/70 dark:border-success-500/30 bg-success-50/80 dark:bg-success-500/10 px-4 py-3 text-sm text-success-700 dark:text-success-300">
                  {t("versions.edit.backup.restore.conflict_empty")}
                </div>
              ) : null}
            </div>
          ) : null}
          {vs.selectedRestoreScopes.length === 0 ? (
            <div className="text-sm text-danger-500">
              {t("versions.edit.backup.no_scope_selected")}
            </div>
          ) : null}
        </div>
      </UnifiedModal>

      <UnifiedModal
        isOpen={vs.restoringInstance}
        onOpenChange={() => {}}
        hideCloseButton
        isDismissable={false}
        type="primary"
        title={t("versions.edit.backup.restore.progress_title")}
      >
        <div className="space-y-4">
          <div className="space-y-1">
            <div className="text-medium font-medium text-default-700 dark:text-zinc-200">
              {vs.restoreProgressText || t("versions.edit.backup.restore.progress_body")}
            </div>
            {vs.restoreProgressStepText ? (
              <div className="text-sm text-default-500 dark:text-zinc-400">
                {vs.restoreProgressStepText}
              </div>
            ) : null}
          </div>
          <Progress
            size="sm"
            value={vs.restoreProgress ? vs.restoreProgressPercent : undefined}
            isIndeterminate={!vs.restoreProgress}
            showValueLabel={!!vs.restoreProgress}
            aria-label="Restoring instance backup"
            classNames={{ indicator: "bg-primary-500" }}
          />
        </div>
      </UnifiedModal>

      <UnifiedModal
        isOpen={vs.backupSuccessOpen}
        onOpenChange={(open) => {
          if (!open) vs.setBackupSuccessOpen(false);
        }}
        size="md"
        type="success"
        title={t("versions.edit.backup.success_title")}
        icon={<FiCheckCircle className="w-6 h-6" />}
        footer={
          <>
            <Button
              variant="light"
              radius="full"
              onPress={() => vs.setBackupSuccessOpen(false)}
            >
              {t("launcherpage.delete.complete.close_button")}
            </Button>
            <Button
              color="primary"
              radius="full"
              className="font-bold shadow-lg shadow-primary-500/20"
              onPress={async () => {
                await vs.openInstanceBackupDirectory();
                vs.setBackupSuccessOpen(false);
              }}
            >
              {t("versions.edit.backup.open_dir")}
            </Button>
          </>
        }
      >
        <div className="space-y-3 text-default-700 dark:text-zinc-300">
          <p className="text-medium font-medium">
            {t("versions.edit.backup.success_body")}
          </p>
          <div className="rounded-2xl bg-default-100/60 dark:bg-zinc-800/80 border border-default-200/60 dark:border-white/10 px-4 py-3">
            <div className="text-tiny text-default-500 dark:text-zinc-400 mb-1">
              {t("versions.edit.backup.archive_label")}
            </div>
            <div className="font-mono text-small break-all text-default-800 dark:text-zinc-100">
              {vs.backupArchiveName || "-"}
            </div>
          </div>
          <div className="rounded-2xl bg-default-100/60 dark:bg-zinc-800/80 border border-default-200/60 dark:border-white/10 px-4 py-3">
            <div className="text-tiny text-default-500 dark:text-zinc-400 mb-1">
              {t("versions.edit.backup.location_label")}
            </div>
            <div className="font-mono text-small break-all text-default-800 dark:text-zinc-100">
              {vs.backupResult?.backupDir || vs.backupInfo?.backupDir || "-"}
            </div>
          </div>
        </div>
      </UnifiedModal>

      <UnifiedModal
        isOpen={vs.restoreResultOpen}
        onOpenChange={(open) => {
          if (!open) vs.setRestoreResultOpen(false);
        }}
        size="lg"
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
          <Button
            variant="light"
            radius="full"
            onPress={() => vs.setRestoreResultOpen(false)}
          >
            {t("launcherpage.delete.complete.close_button")}
          </Button>
        }
      >
        <div className="space-y-3 text-default-700 dark:text-zinc-300">
          <p className="text-medium font-medium">
            {t(
              vs.restoreResult?.status === "success"
                ? "versions.edit.backup.restore.success_body"
                : vs.restoreResult?.status === "partial"
                  ? "versions.edit.backup.restore.partial_body"
                  : "versions.edit.backup.restore.failed_body",
            )}
          </p>
          {(vs.restoreResult?.scopeResults || []).map((scopeResult) => (
            <div
              key={`${scopeResult.key}-${scopeResult.mode}`}
              className="rounded-2xl bg-default-100/60 dark:bg-zinc-800/80 border border-default-200/60 dark:border-white/10 px-4 py-3 space-y-3"
            >
              <div className="flex flex-wrap items-center gap-2">
                <span className="font-semibold text-default-900 dark:text-zinc-100">
                  {scopeResult.label}
                </span>
                {scopeResult.mode ? (
                  <Chip size="sm" variant="flat" color="warning">
                    {t(`versions.edit.backup.mode.${scopeResult.mode}.label`)}
                  </Chip>
                ) : null}
                <Chip
                  size="sm"
                  variant="flat"
                  color={
                    scopeResult.status === "success"
                      ? "success"
                      : scopeResult.status === "partial"
                        ? "warning"
                        : "danger"
                  }
                >
                  {t(
                    `versions.edit.backup.restore.status.${scopeResult.status}`,
                  )}
                </Chip>
              </div>
              {scopeResult.errorCode ? (
                <div className="text-sm text-danger-500">
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
                      className="text-sm text-warning-600 dark:text-warning-300"
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
                      className="font-mono text-tiny break-all text-default-500 dark:text-zinc-400"
                    >
                      {detail}
                    </div>
                  ))}
                </div>
              ) : null}
            </div>
          ))}
        </div>
      </UnifiedModal>

      <UnifiedModal
        isOpen={vs.unregisterOpen}
        onOpenChange={(open) => {
          if (!open) vs.setUnregisterOpen(false);
        }}
        hideCloseButton
        isDismissable={false}
        type="warning"
        title={t("versions.edit.unregister_progress.title")}
        icon={<FiAlertTriangle className="w-6 h-6" />}
      >
        <div className="text-medium font-medium text-default-600 dark:text-zinc-300 mb-4">
          {t("versions.edit.unregister_progress.body")}
        </div>
        <Progress
          size="sm"
          isIndeterminate
          aria-label="Unregistering"
          classNames={{ indicator: "bg-warning-500" }}
        />
      </UnifiedModal>

      <UnifiedModal
        isOpen={vs.unregisterSuccessOpen}
        onOpenChange={(open) => {
          if (!open) vs.setUnregisterSuccessOpen(false);
        }}
        size="md"
        type="success"
        title={t("versions.edit.unregister_success.title")}
        icon={<FiCheckCircle className="w-6 h-6" />}
        onConfirm={() => {
          vs.setUnregisterSuccessOpen(false);
        }}
        confirmText={t("launcherpage.delete.complete.close_button")}
        showCancelButton={false}
      >
        <div className="text-medium font-medium text-default-600 dark:text-zinc-300">
          {t("versions.edit.unregister_success.body")}
        </div>
      </UnifiedModal>

      <UnifiedModal
        isOpen={vs.gdkMissingOpen}
        onOpenChange={(open) => {
          if (!open) vs.setGdkMissingOpen(false);
        }}
        type="warning"
        title={t("launcherpage.gdk_missing.title")}
        icon={<FiAlertTriangle className="w-6 h-6" />}
        footer={
          <>
            <Button
              variant="light"
              radius="full"
              onPress={() => {
                vs.setGdkMissingOpen(false);
              }}
            >
              {t("common.cancel")}
            </Button>
            <Button
              {...warningConfirmButtonProps}
              radius="full"
              onPress={() => {
                vs.setGdkMissingOpen(false);
                vs.navigate(ROUTES.settings, { state: { tab: "components" } });
              }}
            >
              {t("launcherpage.gdk_missing.go_settings")}
            </Button>
          </>
        }
      >
        <div className="text-medium font-medium text-default-600 dark:text-zinc-300">
          {t("launcherpage.gdk_missing.body")}
        </div>
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
          <Button
            {...warningConfirmButtonProps}
            radius="full"
            onPress={vs.openLipComponentsSettings}
          >
            {t("settings.lip.startup_prompt.open_settings_button")}
          </Button>
        }
      >
        <div className="text-medium font-medium text-default-600 dark:text-zinc-300">
          {t("lip.guard.description")}
        </div>
      </UnifiedModal>

      <UnifiedModal
        isOpen={vs.errorOpen}
        onOpenChange={(open) => {
          if (!open) vs.setErrorOpen(false);
        }}
        hideCloseButton
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
        <div className="text-medium font-medium text-danger-600 dark:text-danger-400 whitespace-pre-wrap break-words leading-7">
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
        size="md"
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
        <div className="text-medium font-medium text-default-600 dark:text-zinc-300">
          {t("launcherpage.delete.complete.content")}
          {vs.deleteSuccessMsg ? (
            <span className="font-mono text-default-700 dark:text-zinc-200 font-bold">
              {" "}
              {vs.deleteSuccessMsg}
            </span>
          ) : null}
        </div>
      </UnifiedModal>

      <UnifiedModal
        size="md"
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
        <div className="text-default-700 dark:text-zinc-300 text-sm">
          {t("versions.unsaved.body")}
        </div>
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
        <div className="text-sm text-default-700 dark:text-zinc-300 whitespace-pre-wrap">
          {t("errors.ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY")}
        </div>
        {vs.demotedWarningNames.length > 0 ? (
          <div className="mt-3 rounded-md bg-warning-50/70 dark:bg-warning-500/10 border border-warning-200/70 dark:border-warning-500/30 px-3 py-2 text-warning-700 dark:text-warning-300 text-sm whitespace-pre-wrap break-all font-mono">
            {vs.demotedWarningNames.join("\n")}
          </div>
        ) : null}
      </UnifiedModal>

      <UnifiedModal
        size="md"
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
          isLoading: vs.installingLL,
          isDisabled: !vs.selectedLLVersion || !vs.canInstallSelectedLLVersion,
        }}
      >
        <div className="space-y-3">
          <p className="text-small leading-6 text-default-500 dark:text-zinc-400">
            {t("versions.edit.loader.ll_select_guidance")}
          </p>
          <Select
            label={
              t("versions.edit.loader.ll_select_version") as unknown as string
            }
            placeholder={
              t(
                "versions.edit.loader.ll_select_placeholder",
              ) as unknown as string
            }
            classNames={COMPONENT_STYLES.select}
            selectedKeys={
              vs.selectedLLVersion
                ? new Set([vs.selectedLLVersion])
                : new Set([])
            }
            onSelectionChange={(keys) => {
              const selected = Array.from(keys as unknown as Set<string>)[0];
              vs.setSelectedLLVersion(String(selected || ""));
            }}
            isDisabled={vs.installingLL || vs.uninstallingLL}
          >
            {vs.llSupportedVersions.map((version: string) => (
              <SelectItem key={version} textValue={version} isDisabled={false}>
                {version}
              </SelectItem>
            ))}
          </Select>
          {vs.llInstallBlockedReasonKey ? (
            <p className="text-tiny text-warning-600 dark:text-warning-500">
              {t(vs.llInstallBlockedReasonKey)}
            </p>
          ) : null}
        </div>
      </UnifiedModal>

      <UnifiedModal
        size="md"
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
          isLoading: vs.installingLL,
          isDisabled: !vs.resolvedLLTargetVersion || vs.installingLL,
        }}
        cancelButtonProps={{
          isDisabled: vs.installingLL,
        }}
      >
        <div className="text-sm text-default-700 dark:text-zinc-300 whitespace-pre-wrap">
          {t("lip.package.confirm_install_body", {
            action: llInstallActionLabel,
            package: "LeviLamina",
            version: vs.resolvedLLTargetVersion || "-",
            instance: vs.targetName || "-",
          })}
        </div>
      </UnifiedModal>

      <UnifiedModal
        size="md"
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
        <div className="text-sm text-default-700 dark:text-zinc-300 space-y-2">
          <p>
            {t("mods.rc_warning.body_1", {
              version: vs.rcVersion,
            })}
          </p>
          <p className="font-semibold text-warning-700">
            {t("mods.rc_warning.body_2")}
          </p>
          <p>{t("mods.rc_warning.body_3")}</p>
        </div>
      </UnifiedModal>
    </PageContainer>
  );
}
