import { Dialogs } from "@wailsio/runtime";
import { UnifiedModal } from "@/components/UnifiedModal";
import { DeleteConfirmModal } from "@/components/DeleteConfirmModal";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import {
  Button,
  Card,
  CardBody,
  Input,
  Switch,
  Chip,
  Progress,
  Textarea,
  Tabs,
  Tab,
  addToast,
} from "@heroui/react";
import { useTranslation } from "react-i18next";
import { motion } from "framer-motion";
import { FaWindows } from "react-icons/fa";
import { FiAlertTriangle, FiCheckCircle } from "react-icons/fi";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import {
  CreateDesktopShortcut,
  GetVersionLogoDataUrl,
  RemoveVersionLogo,
  SaveVersionLogoFromPath,
  UnregisterVersionByName,
} from "bindings/github.com/liteldev/LeviLauncher/versionservice";
import { PageHeader } from "@/components/PageHeader";
import LeviLaminaIcon from "@/assets/images/LeviLamina.png";
import { useVersionSettings } from "@/hooks/useVersionSettings";

export default function VersionSettingsPage() {
  const { t } = useTranslation();
  const vs = useVersionSettings();

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
                      className="bg-primary-600 hover:bg-primary-500 text-white font-bold shadow-lg shadow-primary-900/20"
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
                        <Button
                          size="sm"
                          variant="flat"
                          color="primary"
                          radius="full"
                          className="justify-start px-4 font-medium bg-primary-500/10 text-primary-600 dark:text-primary-400"
                          onPress={async () => {
                            try {
                              const err: string = await CreateDesktopShortcut(
                                vs.targetName,
                              );
                              if (err) {
                                vs.setError(String(err));
                              } else {
                                vs.setShortcutSuccessOpen(true);
                              }
                            } catch {
                              vs.setError("ERR_SHORTCUT_CREATE_FAILED");
                            }
                          }}
                          startContent={<FaWindows />}
                        >
                          {
                            t(
                              "launcherpage.shortcut.create_button",
                            ) as unknown as string
                          }
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
                  <div className="flex items-center justify-between p-2 rounded-xl hover:bg-default-100/50 transition-colors">
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
                  <div className="flex items-center justify-between p-2 rounded-xl hover:bg-default-100/50 transition-colors">
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
                  <div className="flex items-center justify-between p-4 rounded-xl border border-default-200 dark:border-default-100/10 bg-default-50 dark:bg-default-100/10">
                    <div className="flex items-center gap-4">
                      <div className="w-12 h-12 rounded-xl overflow-hidden">
                        <img
                          src={LeviLaminaIcon}
                          alt="LeviLamina"
                          className="w-full h-full object-cover"
                        />
                      </div>
                      <div>
                        <div className="text-medium font-bold text-foreground">
                          LeviLamina
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
                              onPress={vs.handleUninstallLL}
                              isDisabled={vs.installingLL}
                            >
                              {t("common.remove")}
                            </Button>
                          )}
                          <Button
                            color="success"
                            variant="flat"
                            className="bg-primary-500/10 text-primary-600 dark:text-primary-400 font-bold"
                            onPress={() => {
                              if (vs.isLLInstalled) {
                                addToast({
                                  title: t("common.tip"),
                                  description: t("common.feature_unavailable"),
                                  color: "warning",
                                });
                                return;
                              }
                              vs.handleInstallLeviLamina();
                            }}
                            isLoading={vs.installingLL}
                          >
                            {vs.isLLInstalled
                              ? t("common.update")
                              : t("downloadpage.install.levilamina_label")}
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
                  <div className="flex items-center justify-between p-2 rounded-xl hover:bg-default-100/50 transition-colors">
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
                  <div className="flex items-center justify-between p-2 rounded-xl hover:bg-default-100/50 transition-colors">
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
                <div className="flex flex-col gap-1">
                  <div className="text-medium font-bold text-default-900 dark:text-zinc-100">
                    {vs.isRegistered
                      ? t("versions.edit.unregister_button")
                      : t("common.delete")}
                  </div>
                  <div className="text-small text-default-500 dark:text-zinc-400 mb-4 max-w-lg">
                    {vs.isRegistered
                      ? t("versions.edit.unregister_hint")
                      : t("versions.edit.delete_hint")}
                  </div>
                  <div>
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
              )}
            </CardBody>
          </Card>
        </motion.div>
      </div>

      <UnifiedModal
        isOpen={vs.unregisterOpen}
        onOpenChange={(open) => {
          if (!open) vs.setUnregisterOpen(false);
        }}
        hideCloseButton
        isDismissable={false}
        type="warning"
        title={t("versions.edit.unregister_progress.title")}
        icon={<FiAlertTriangle className="w-6 h-6 text-warning-500" />}
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
        icon={<FiCheckCircle className="w-6 h-6 text-success-500" />}
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
        icon={<FiAlertTriangle className="w-6 h-6 text-warning-500" />}
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
              color="primary"
              radius="full"
              className="bg-primary-600 hover:bg-primary-500 text-white font-bold shadow-lg shadow-primary-900/20"
              onPress={() => {
                vs.setGdkMissingOpen(false);
                vs.navigate("/settings", { state: { tab: "components" } });
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
        isOpen={vs.errorOpen}
        onOpenChange={(open) => {
          if (!open) vs.setErrorOpen(false);
        }}
        hideCloseButton
        type="error"
        title={t("common.error")}
        icon={<FiAlertTriangle className="w-6 h-6 text-danger-500" />}
        onConfirm={() => {
          vs.setError("");
          vs.setErrorOpen(false);
        }}
        confirmText={t("common.ok")}
        showCancelButton={false}
      >
        <div className="p-4 rounded-2xl bg-danger-50 dark:bg-danger-500/10 border border-danger-100 dark:border-danger-500/20 text-danger-600 dark:text-danger-400 font-medium">
          <div className="text-medium wrap-break-word">
            {t(vs.getErrorKey(vs.error))}
          </div>
        </div>
      </UnifiedModal>

      <UnifiedModal
        isOpen={vs.shortcutSuccessOpen}
        onOpenChange={(open) => {
          if (!open) vs.setShortcutSuccessOpen(false);
        }}
        size="md"
        type="success"
        title={t("launcherpage.shortcut.success.title")}
        icon={<FiCheckCircle className="w-6 h-6 text-success-500" />}
        onConfirm={() => {
          vs.setShortcutSuccessOpen(false);
        }}
        confirmText={t("common.close")}
        showCancelButton={false}
      >
        <div className="text-medium font-medium text-default-600 dark:text-zinc-300">
          {t("launcherpage.shortcut.success.body")}
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
        icon={<FiCheckCircle className="w-6 h-6 text-success-500" />}
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

      <UnifiedModal
        size="md"
        isOpen={vs.rcOpen}
        onOpenChange={vs.rcOnOpenChange}
        type="warning"
        title={t("mods.rc_warning.title")}
        icon={<FiAlertTriangle className="w-6 h-6 text-warning-500" />}
        cancelText={t("common.cancel")}
        confirmText={t("common.continue")}
        showCancelButton
        onCancel={() => vs.rcOnClose()}
        onConfirm={() => {
          vs.rcOnClose();
          vs.proceedInstallLeviLamina();
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
