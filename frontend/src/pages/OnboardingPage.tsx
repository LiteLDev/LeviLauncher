import React from "react";
import { useTranslation } from "react-i18next";
import { motion } from "framer-motion";
import {
  Card,
  CardHeader,
  CardBody,
  Button,
  Input,
  Dropdown,
  DropdownTrigger,
  DropdownMenu,
  DropdownItem,
  Divider,
  useDisclosure,
} from "@heroui/react";
import { UnifiedModal } from "@/components/UnifiedModal";
import { PageHeader, SectionHeader } from "@/components/PageHeader";
import { useNavigate, useLocation } from "react-router-dom";
import {
  GetLanguageNames,
  GetBaseRoot,
  SetBaseRoot,
  ResetBaseRoot,
  GetInstallerDir,
  GetVersionsDir,
  CanWriteToDir,
} from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { normalizeLanguage } from "@/utils/i18nUtils";
import { Dialogs } from "@wailsio/runtime";

export default function OnboardingPage() {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const hasBackend = minecraft !== undefined;
  const [langNames, setLangNames] = React.useState<
    Array<{ language: string; code: string }>
  >([]);
  const [selectedLang, setSelectedLang] = React.useState<string>("en_US");
  const [baseRoot, setBaseRoot] = React.useState<string>("");
  const [newBaseRoot, setNewBaseRoot] = React.useState<string>("");
  const [installerDir, setInstallerDir] = React.useState<string>("");
  const [versionsDir, setVersionsDir] = React.useState<string>("");
  const [baseRootWritable, setBaseRootWritable] = React.useState<boolean>(true);
  const [savingBaseRoot, setSavingBaseRoot] = React.useState<boolean>(false);
  const {
    isOpen: unsavedOpen,
    onOpen: unsavedOnOpen,
    onClose: unsavedOnClose,
    onOpenChange: unsavedOnOpenChange,
  } = useDisclosure();

  React.useEffect(() => {
    GetLanguageNames().then((res: any) => setLangNames(res));

    setSelectedLang(normalizeLanguage(i18n.language));
    (async () => {
      try {
        if (hasBackend) {
          const br = await GetBaseRoot();
          setBaseRoot(String(br || ""));
          setNewBaseRoot(String(br || ""));
          const id = await GetInstallerDir();
          setInstallerDir(String(id || ""));
          const vd = await GetVersionsDir();
          setVersionsDir(String(vd || ""));
        }
      } catch {}
    })();
  }, []);

  React.useEffect(() => {
    setBaseRootWritable(true);
  }, [newBaseRoot]);

  const proceedHome = () => {
    try {
      localStorage.setItem("ll.onboarded", "1");
    } catch {}
    navigate("/", { replace: true });
  };

  const requestFinish = () => {
    if (newBaseRoot && newBaseRoot !== baseRoot && baseRootWritable) {
      unsavedOnOpen();
      return;
    }
    proceedHome();
  };

  return (
    <div className="relative w-full h-full flex items-center justify-center px-4 pb-4 overflow-hidden bg-default-50 dark:bg-black">
      <motion.div
        initial={{ opacity: 0, scale: 0.95 }}
        animate={{ opacity: 1, scale: 1 }}
        transition={{ duration: 0.4, ease: "easeOut" }}
        className="w-full max-w-2xl mx-auto flex flex-col justify-center py-6"
      >
        <div className="flex flex-row items-center justify-center gap-6 mb-6">
          <div className="w-16 h-16 rounded-3xl bg-linear-to-br from-emerald-500 to-teal-600 shadow-xl shadow-emerald-500/20 flex items-center justify-center text-white shrink-0">
            <svg
              className="w-8 h-8"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
              xmlns="http://www.w3.org/2000/svg"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M13 10V3L4 14h7v7l9-11h-7z"
              />
            </svg>
          </div>
          <div className="flex flex-col items-start text-left">
            <h1 className="text-3xl font-bold bg-clip-text text-transparent bg-gradient-to-r from-emerald-600 to-teal-600 dark:from-emerald-400 dark:to-teal-400 mb-1">
              {t("onboarding.title")}
            </h1>
            <p className="text-default-500 dark:text-zinc-400 text-base max-w-md">
              {t("onboarding.subtitle")}
            </p>
          </div>
        </div>

        <Card className="w-full border-none shadow-xl bg-white/60 dark:bg-zinc-900/60 backdrop-blur-xl rounded-[2rem] p-2">
          <CardBody className="p-4 space-y-4">
            <motion.div
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.4, delay: 0.1 }}
              className="bg-default-50/50 dark:bg-zinc-800/30 border border-default-100 dark:border-white/5 rounded-3xl p-4"
            >
              <div className="mb-3">
                <SectionHeader
                  title={t("settings.body.paths.title")}
                  description={t("settings.body.paths.subtitle")}
                />
              </div>

              <div className="space-y-3">
                <Input
                  labelPlacement="outside"
                  label={t("settings.body.paths.base_root")}
                  placeholder={t("settings.body.paths.base_root")}
                  value={newBaseRoot}
                  onValueChange={setNewBaseRoot}
                  variant="bordered"
                  classNames={{
                    inputWrapper:
                      "bg-default-50/50 dark:bg-black/20 border-default-200 dark:border-white/10 shadow-none",
                  }}
                  description={
                    newBaseRoot && newBaseRoot !== baseRoot ? (
                      <span
                        className={
                          baseRootWritable
                            ? "text-warning-500 font-medium"
                            : "text-danger-500 font-medium"
                        }
                      >
                        {baseRootWritable
                          ? t("settings.body.paths.unsaved")
                          : t("settings.body.paths.not_writable")}
                      </span>
                    ) : null
                  }
                  endContent={
                    <Button
                      size="sm"
                      variant="flat"
                      className="bg-default-200/50 dark:bg-white/10 font-medium"
                      onPress={async () => {
                        try {
                          const options: any = {
                            Title: t("settings.body.paths.title"),
                            CanChooseDirectories: true,
                            CanChooseFiles: false,
                            PromptForSingleSelection: true,
                          };
                          if (baseRoot) {
                            options.Directory = baseRoot;
                          }
                          const result = await Dialogs.OpenFile(options);
                          if (Array.isArray(result) && result.length > 0) {
                            setNewBaseRoot(result[0]);
                          } else if (typeof result === "string" && result) {
                            setNewBaseRoot(result);
                          }
                        } catch (e) {
                          console.error(e);
                        }
                      }}
                    >
                      {t("common.browse")}
                    </Button>
                  }
                />

                <div className="flex items-center justify-between">
                  <div className="flex gap-2">
                    <Button
                      size="sm"
                      color="primary"
                      radius="full"
                      className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
                      isDisabled={
                        !newBaseRoot ||
                        !baseRootWritable ||
                        newBaseRoot === baseRoot
                      }
                      isLoading={savingBaseRoot}
                      onPress={async () => {
                        setSavingBaseRoot(true);
                        try {
                          const ok = await CanWriteToDir(newBaseRoot);
                          if (!ok) {
                            setBaseRootWritable(false);
                          } else {
                            const err = await SetBaseRoot(newBaseRoot);
                            if (!err) {
                              const br = await GetBaseRoot();
                              setBaseRoot(String(br || ""));
                              const id = await GetInstallerDir();
                              setInstallerDir(String(id || ""));
                              const vd = await GetVersionsDir();
                              setVersionsDir(String(vd || ""));
                            }
                          }
                        } catch {}
                        setSavingBaseRoot(false);
                      }}
                    >
                      {t("settings.body.paths.apply")}
                    </Button>
                    <Button
                      size="sm"
                      variant="light"
                      radius="full"
                      className="text-default-500 dark:text-zinc-400 hover:text-default-700 dark:hover:text-zinc-200"
                      onPress={async () => {
                        try {
                          const err = await ResetBaseRoot();
                          if (!err) {
                            const br = await GetBaseRoot();
                            setBaseRoot(String(br || ""));
                            setNewBaseRoot(String(br || ""));
                            const id = await GetInstallerDir();
                            setInstallerDir(String(id || ""));
                            const vd = await GetVersionsDir();
                            setVersionsDir(String(vd || ""));
                            setBaseRootWritable(true);
                          }
                        } catch {}
                      }}
                    >
                      {t("settings.body.paths.reset")}
                    </Button>
                  </div>
                </div>
              </div>
            </motion.div>

            <motion.div
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.4, delay: 0.2 }}
              className="bg-default-50/50 dark:bg-zinc-800/30 border border-default-100 dark:border-white/5 rounded-3xl p-4"
            >
              <SectionHeader
                title={t("settings.body.language.name")}
                description={
                  langNames.find((l) => l.code === selectedLang)?.language ||
                  selectedLang
                }
                action={
                  <Dropdown>
                    <DropdownTrigger>
                      <Button
                        radius="full"
                        variant="flat"
                        className="bg-default-100 dark:bg-white/10 font-medium"
                      >
                        {t("settings.body.language.button")}
                      </Button>
                    </DropdownTrigger>
                    <DropdownMenu
                      aria-label="Language selection"
                      variant="flat"
                      disallowEmptySelection
                      selectionMode="single"
                      className="max-h-60 overflow-y-auto"
                      selectedKeys={new Set([selectedLang])}
                      onSelectionChange={(keys) => {
                        const arr = Array.from(keys as unknown as Set<string>);
                        const next = arr[0];
                        if (typeof next === "string" && next.length > 0) {
                          setSelectedLang(next);
                          Promise.resolve(i18n.changeLanguage(next)).then(
                            () => {
                              try {
                                localStorage.setItem("i18nextLng", next);
                              } catch {}
                            },
                          );
                        }
                      }}
                    >
                      {langNames.map((lang) => (
                        <DropdownItem key={lang.code} textValue={lang.language}>
                          {lang.language}
                        </DropdownItem>
                      ))}
                    </DropdownMenu>
                  </Dropdown>
                }
              />
            </motion.div>
          </CardBody>
        </Card>

        <div className="flex items-center justify-end gap-3 mt-4 w-full">
          <Button
            variant="light"
            radius="full"
            onPress={requestFinish}
            className="font-medium text-default-500 dark:text-zinc-400"
          >
            {t("onboarding.skip")}
          </Button>
          <Button
            color="primary"
            radius="full"
            className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20 px-8"
            onPress={requestFinish}
          >
            {t("onboarding.finish")}
          </Button>
        </div>
      </motion.div>

      <UnifiedModal
        size="md"
        isOpen={unsavedOpen}
        onOpenChange={unsavedOnOpenChange}
        type="warning"
        title={t("onboarding.unsaved.title")}
        cancelText={t("onboarding.unsaved.cancel")}
        confirmText={t("onboarding.unsaved.save")}
        showCancelButton
        confirmButtonProps={{
          isLoading: savingBaseRoot,
          isDisabled: !newBaseRoot || !baseRootWritable,
        }}
        onCancel={() => unsavedOnClose()}
        onConfirm={async () => {
          setSavingBaseRoot(true);
          try {
            const ok = await CanWriteToDir(newBaseRoot);
            if (!ok) {
              setBaseRootWritable(false);
            } else {
              const err = await SetBaseRoot(newBaseRoot);
              if (!err) {
                const br = await GetBaseRoot();
                setBaseRoot(String(br || ""));
                const id = await GetInstallerDir();
                setInstallerDir(String(id || ""));
                const vd = await GetVersionsDir();
                setVersionsDir(String(vd || ""));
                unsavedOnClose();
                proceedHome();
              }
            }
          } catch {}
          setSavingBaseRoot(false);
        }}
      >
        <div className="flex flex-col gap-2">
          <div className="text-default-700 dark:text-zinc-300 text-sm">
            {t("onboarding.unsaved.body")}
          </div>
          {!baseRootWritable && (
            <div className="text-tiny text-danger-500">
              {t("settings.body.paths.not_writable")}
            </div>
          )}
        </div>
      </UnifiedModal>
    </div>
  );
}
