import {
  Button,
  Card,
  Description,
  Dropdown,
  FieldError,
  InputGroup,
  Label,
  Separator,
  Spinner,
  TextField,
  useOverlayState,
} from "@heroui/react";

import { cn } from "@/utils/cn";

import React from "react";
import { useTranslation } from "react-i18next";

import { UnifiedModal } from "@/components/UnifiedModal";
import { PageHeader, SectionHeader } from "@/components/PageHeader";
import { PageContainer } from "@/components/PageContainer";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { LAYOUT } from "@/constants/layout";
import { ROUTES } from "@/constants/routes";
import { useNavigate } from "react-router-dom";
import {
  GetLanguageNames,
  GetBaseRoot,
  SetBaseRoot,
  ResetBaseRoot,
  CanWriteToDir,
} from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { normalizeLanguage } from "@/utils/i18nUtils";
import { Dialogs } from "@wailsio/runtime";
import { LuHardDrive, LuLanguages } from "react-icons/lu";

type OnboardingErrorKey = "common.load_failed" | "common.save_failed" | null;

export default function OnboardingPage() {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const hasBackend = minecraft !== undefined;
  const [langNames, setLangNames] = React.useState<
    Array<{ language: string; code: string }>
  >([]);
  const [selectedLang, setSelectedLang] = React.useState<string>("en_US");
  const [baseRoot, setBaseRoot] = React.useState<string>("");
  const [newBaseRoot, setNewBaseRoot] = React.useState<string>("");
  const [baseRootWritable, setBaseRootWritable] = React.useState<boolean>(true);
  const [savingBaseRoot, setSavingBaseRoot] = React.useState<boolean>(false);
  const [errorKey, setErrorKey] = React.useState<OnboardingErrorKey>(null);
  const {
    isOpen: unsavedOpen,
    open: unsavedOnOpen,
    close: unsavedOnClose,
    setOpen: unsavedOnOpenChange,
  } = useOverlayState();

  React.useEffect(() => {
    GetLanguageNames()
      .then((res: any) => setLangNames(res))
      .catch((error: unknown) => {
        console.error("Failed to load language names", error);
        setErrorKey("common.load_failed");
      });

    setSelectedLang(normalizeLanguage(i18n.language));
    (async () => {
      try {
        if (hasBackend) {
          const br = await GetBaseRoot();
          setBaseRoot(String(br || ""));
          setNewBaseRoot(String(br || ""));
        }
      } catch (error) {
        console.error("Failed to load base root", error);
        setErrorKey("common.load_failed");
      }
    })();
  }, []);

  React.useEffect(() => {
    setBaseRootWritable(true);
  }, [newBaseRoot]);

  const proceedHome = () => {
    try {
      localStorage.setItem("ll.onboarded", "1");
    } catch {}
    navigate(ROUTES.home, { replace: true });
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
          <Card.Content className="p-6">
            <PageHeader
              title={t("onboarding.title")}
              description={t("onboarding.subtitle")}
              startContent={
                <div className="w-16 h-16 rounded-2xl bg-brand-500/10 flex items-center justify-center text-brand-600 dark:text-brand-500 shrink-0">
                  <svg
                    className="w-10 h-10"
                    aria-hidden="true"
                    fill="none"
                    focusable="false"
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
                    onPress={requestFinish}
                    variant={"ghost"}
                    className={cn("rounded-full", "font-bold text-muted px-6")}
                  >
                    {t("onboarding.skip")}
                  </Button>
                  <Button
                    onPress={requestFinish}
                    variant={"primary"}
                    className={cn(
                      "rounded-full",
                      "font-black px-10 h-12 text-lg brand-primary-foreground shadow-lg shadow-brand-500/20",
                    )}
                  >
                    {t("onboarding.finish")}
                  </Button>
                </div>
              }
            />
          </Card.Content>
        </Card>

        {/* Content Card */}
        <Card className={LAYOUT.GLASS_CARD.BASE}>
          <Card.Content className="p-6 space-y-8">
            {errorKey ? (
              <p
                aria-atomic="true"
                className="text-sm font-medium text-rose-500"
                role="alert"
              >
                {t(errorKey)}
              </p>
            ) : null}
            <div className="space-y-4">
              <SectionHeader
                title={t("settings.body.paths.title")}
                description={t("settings.body.paths.subtitle")}
                icon={<LuHardDrive className="w-5 h-5" />}
                action={
                  <div className="flex items-center gap-2">
                    <Button
                      size="md"
                      onPress={async () => {
                        setErrorKey(null);
                        try {
                          const err = await ResetBaseRoot();
                          if (err) {
                            throw new Error(err);
                          }

                          const br = await GetBaseRoot();
                          setBaseRoot(String(br || ""));
                          setNewBaseRoot(String(br || ""));
                          setBaseRootWritable(true);
                        } catch (error) {
                          console.error("Failed to reset base root", error);
                          setErrorKey("common.save_failed");
                        }
                      }}
                      variant={"ghost"}
                      className={cn("rounded-full", "font-bold px-4")}
                    >
                      {t("settings.body.paths.reset")}
                    </Button>
                    <Button
                      size="md"
                      isDisabled={
                        !newBaseRoot ||
                        !baseRootWritable ||
                        newBaseRoot === baseRoot
                      }
                      onPress={async () => {
                        setErrorKey(null);
                        setSavingBaseRoot(true);
                        try {
                          const ok = await CanWriteToDir(newBaseRoot);
                          if (!ok) {
                            setBaseRootWritable(false);
                          } else {
                            const err = await SetBaseRoot(newBaseRoot);
                            if (err) {
                              throw new Error(err);
                            }

                            const br = await GetBaseRoot();
                            setBaseRoot(String(br || ""));
                          }
                        } catch (error) {
                          console.error("Failed to save base root", error);
                          setErrorKey("common.save_failed");
                        } finally {
                          setSavingBaseRoot(false);
                        }
                      }}
                      variant={"primary"}
                      isPending={savingBaseRoot}
                      className={cn("rounded-full", "font-bold px-6")}
                    >
                      {({ isPending }) => (
                        <>
                          <Spinner
                            size="sm"
                            color="current"
                            className={isPending ? "" : "hidden"}
                          />
                          {t("settings.body.paths.apply")}
                        </>
                      )}
                    </Button>
                  </div>
                }
              />

              <div className="space-y-4">
                <TextField
                  isInvalid={!baseRootWritable}
                  className={cn("group", COMPONENT_STYLES.input.mainWrapper)}
                  value={newBaseRoot}
                  onChange={setNewBaseRoot}
                >
                  <Label className={COMPONENT_STYLES.input.label}>
                    {t("settings.body.paths.base_root") as string}
                  </Label>
                  <InputGroup
                    className={cn(
                      COMPONENT_STYLES.input.inputWrapper,
                      COMPONENT_STYLES.input.innerWrapper,
                      "rounded-lg",
                    )}
                  >
                    <InputGroup.Input
                      placeholder={t("settings.body.paths.base_root")}
                      className={COMPONENT_STYLES.input.input}
                    />
                    <InputGroup.Suffix>
                      {
                        <Button
                          size="sm"
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
                            } catch (error) {
                              console.error(
                                "Failed to browse for base root",
                                error,
                              );
                              setErrorKey("common.load_failed");
                            }
                          }}
                          variant={"secondary"}
                          className={cn(
                            "rounded-full",
                            "bg-surface-tertiary/50 dark:bg-white/10 font-medium",
                          )}
                        >
                          {t("common.browse")}
                        </Button>
                      }
                    </InputGroup.Suffix>
                  </InputGroup>
                  <Description className={COMPONENT_STYLES.input.description}>
                    {baseRootWritable &&
                    newBaseRoot &&
                    newBaseRoot !== baseRoot ? (
                      <span className="text-amber-500 font-medium">
                        {t("settings.body.paths.unsaved")}
                      </span>
                    ) : null}
                  </Description>
                  <FieldError className={COMPONENT_STYLES.input.errorMessage}>
                    {!baseRootWritable
                      ? t("settings.body.paths.not_writable")
                      : undefined}
                  </FieldError>
                </TextField>
              </div>
            </div>
            <Separator className="opacity-50" />
            <div className="space-y-4">
              <SectionHeader
                title={t("settings.body.language.name")}
                description={
                  langNames.find((l) => l.code === selectedLang)?.language ||
                  selectedLang
                }
                icon={<LuLanguages className="w-5 h-5" />}
                action={
                  <Dropdown>
                    <Button
                      variant={"secondary"}
                      className={cn(
                        "rounded-full",
                        "bg-surface-tertiary/50 dark:bg-white/10 font-bold",
                      )}
                    >
                      {t("settings.body.language.button")}
                    </Button>
                    <Dropdown.Popover
                      className={COMPONENT_STYLES.dropdown.content}
                    >
                      <Dropdown.Menu
                        aria-label={t("settings.body.language.name")}
                        disallowEmptySelection
                        selectionMode="single"
                        className="max-h-60 overflow-y-auto custom-scrollbar"
                        selectedKeys={new Set([selectedLang])}
                        onSelectionChange={(keys) => {
                          const arr = Array.from(
                            keys as unknown as Set<string>,
                          );
                          const next = arr[0];
                          if (typeof next === "string" && next.length > 0) {
                            const previous = selectedLang;
                            setErrorKey(null);
                            setSelectedLang(next);
                            Promise.resolve(i18n.changeLanguage(next))
                              .then(() => {
                                try {
                                  localStorage.setItem("i18nextLng", next);
                                } catch {}
                              })
                              .catch((error: unknown) => {
                                console.error(
                                  "Failed to change language",
                                  error,
                                );
                                setSelectedLang(previous);
                                setErrorKey("common.load_failed");
                              });
                          }
                        }}
                      >
                        {langNames.map((lang) => (
                          <Dropdown.Item
                            key={lang.code}
                            id={lang.code}
                            textValue={lang.language}
                          >
                            <Label>{lang.language}</Label>
                            <Dropdown.ItemIndicator />
                          </Dropdown.Item>
                        ))}
                      </Dropdown.Menu>
                    </Dropdown.Popover>
                  </Dropdown>
                }
              />
            </div>
          </Card.Content>
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
          isPending: savingBaseRoot,
          isDisabled: !newBaseRoot || !baseRootWritable,
        }}
        onCancel={() => unsavedOnClose()}
        onConfirm={async () => {
          setErrorKey(null);
          setSavingBaseRoot(true);
          try {
            const ok = await CanWriteToDir(newBaseRoot);
            if (!ok) {
              setBaseRootWritable(false);
            } else {
              const err = await SetBaseRoot(newBaseRoot);
              if (err) {
                throw new Error(err);
              }

              const br = await GetBaseRoot();
              setBaseRoot(String(br || ""));
              unsavedOnClose();
              proceedHome();
            }
          } catch (error) {
            console.error("Failed to save base root before finishing", error);
            setErrorKey("common.save_failed");
          } finally {
            setSavingBaseRoot(false);
          }
        }}
      >
        <div className="flex flex-col gap-2">
          <div className="text-foreground dark:text-zinc-300 text-sm">
            {t("onboarding.unsaved.body")}
          </div>
          {!baseRootWritable && (
            <div className="text-xs text-rose-500">
              {t("settings.body.paths.not_writable")}
            </div>
          )}
        </div>
      </UnifiedModal>
    </PageContainer>
  );
}
