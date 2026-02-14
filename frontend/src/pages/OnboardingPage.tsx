import React from "react";
import { useTranslation } from "react-i18next";
import {
  Card,
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
import { PageContainer } from "@/components/PageContainer";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { LAYOUT } from "@/constants/layout";
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
import { LuHardDrive, LuLanguages } from "react-icons/lu";

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
    <PageContainer>
      <div className="flex flex-col gap-6">
        {/* Header Card */}
        <Card className={LAYOUT.GLASS_CARD.BASE}>
          <CardBody className="p-6">
            <PageHeader
              title={t("onboarding.title")}
              description={t("onboarding.subtitle")}
              startContent={
                <div className="w-16 h-16 rounded-2xl bg-primary-500/10 flex items-center justify-center text-primary-600 dark:text-primary-500 shrink-0">
                  <svg
                    className="w-10 h-10"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                    xmlns="http://www.w3.org/2000/svg"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2.5}
                      d="M13 10V3L4 14h7v7l9-11h-7z"
                    />
                  </svg>
                </div>
              }
              endContent={
                <div className="flex items-center gap-3">
                  <Button
                    variant="light"
                    radius="full"
                    onPress={requestFinish}
                    className="font-bold text-default-500 px-6"
                  >
                    {t("onboarding.skip")}
                  </Button>
                  <Button
                    color="primary"
                    radius="full"
                    className="font-black px-10 h-12 text-lg shadow-lg shadow-primary-500/20"
                    onPress={requestFinish}
                  >
                    {t("onboarding.finish")}
                  </Button>
                </div>
              }
            />
          </CardBody>
        </Card>

        {/* Content Card */}
        <Card className={LAYOUT.GLASS_CARD.BASE}>
          <CardBody className="p-6 space-y-8">
            <div className="space-y-4">
              <SectionHeader
                title={t("settings.body.paths.title")}
                description={t("settings.body.paths.subtitle")}
                icon={<LuHardDrive className="w-5 h-5" />}
                action={
                  <div className="flex items-center gap-2">
                    <Button
                      size="md"
                      variant="light"
                      radius="full"
                      className="font-bold px-4"
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
                    <Button
                      size="md"
                      color="primary"
                      radius="full"
                      className="font-bold px-6"
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
                  </div>
                }
              />

              <div className="space-y-4">
                <Input
                  label={t("settings.body.paths.base_root") as string}
                  placeholder={t("settings.body.paths.base_root")}
                  value={newBaseRoot}
                  onValueChange={setNewBaseRoot}
                  variant="bordered"
                  radius="lg"
                  classNames={COMPONENT_STYLES.input}
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
                      radius="full"
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
              </div>
            </div>

            <Divider className="opacity-50" />

            <div className="space-y-4">
              <SectionHeader
                title={t("settings.body.language.name")}
                description={
                  langNames.find((l) => l.code === selectedLang)?.language ||
                  selectedLang
                }
                icon={<LuLanguages className="w-5 h-5" />}
                action={
                  <Dropdown classNames={COMPONENT_STYLES.dropdown}>
                    <DropdownTrigger>
                      <Button
                        radius="full"
                        variant="flat"
                        className="bg-default-200/50 dark:bg-white/10 font-bold"
                      >
                        {t("settings.body.language.button")}
                      </Button>
                    </DropdownTrigger>
                    <DropdownMenu
                      aria-label="Language selection"
                      variant="flat"
                      disallowEmptySelection
                      selectionMode="single"
                      className="max-h-60 overflow-y-auto custom-scrollbar"
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
            </div>
          </CardBody>
        </Card>
      </div>

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
    </PageContainer>
  );
}
