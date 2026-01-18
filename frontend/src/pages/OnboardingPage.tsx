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
  ModalContent,
  useDisclosure,
} from "@heroui/react";
import {
  BaseModal,
  BaseModalHeader,
  BaseModalBody,
  BaseModalFooter,
} from "@/components/BaseModal";
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
    onOpenChange: unsavedOnOpenChange,
  } = useDisclosure();

  React.useEffect(() => {
    GetLanguageNames().then((res: any) => setLangNames(res));

    const normalize = (lng: string) => {
      if (!lng) return "en_US";
      const lower = lng.toLowerCase();
      if (lower === "en-us" || lower === "en") return "en_US";
      if (lower === "zh-cn" || lower === "zh") return "zh_CN";
      return lng;
    };
    setSelectedLang(normalize(i18n.language));
    (async () => {
      try {
        if (hasBackend) {
          const br = await GetBaseRoot();
          const pick = (location?.state as any)?.baseRootPickResult;
          setBaseRoot(String(br || ""));
          setNewBaseRoot(
            typeof pick === "string" && pick.length > 0
              ? String(pick)
              : String(br || ""),
          );
          const id = await GetInstallerDir();
          setInstallerDir(String(id || ""));
          const vd = await GetVersionsDir();
          setVersionsDir(String(vd || ""));
        }
      } catch {}
    })();
  }, []);

  React.useEffect(() => {
    const v: string | undefined = (location?.state as any)?.baseRootPickResult;
    if (v && typeof v === "string" && v.length > 0) {
      setNewBaseRoot(v);
      setTimeout(() => {
        navigate(location.pathname, {
          replace: true,
          state: { ...(location.state as any), baseRootPickResult: undefined },
        });
      }, 0);
    }
  }, [location?.state]);

  React.useEffect(() => {
    const checkWritable = async () => {
      try {
        if (!newBaseRoot) {
          setBaseRootWritable(false);
          return;
        }
        const ok = await CanWriteToDir(newBaseRoot);
        setBaseRootWritable(Boolean(ok));
      } catch {
        setBaseRootWritable(false);
      }
    };
    checkWritable();
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
    <div className="fixed inset-0 z-40 flex items-center justify-center px-4 pb-4 pt-[84px] overflow-hidden bg-default-50 dark:bg-black">
      {/* Background Gradients */}
      <div className="fixed inset-0 -z-10 pointer-events-none overflow-hidden">
        <div className="absolute top-[-10%] left-[-10%] w-[40%] h-[40%] rounded-full bg-emerald-500/10 dark:bg-emerald-500/20 blur-[100px]" />
        <div className="absolute bottom-[-10%] right-[-10%] w-[40%] h-[40%] rounded-full bg-indigo-500/10 dark:bg-indigo-500/20 blur-[100px]" />
      </div>

      <motion.div
        initial={{ opacity: 0, scale: 0.95 }}
        animate={{ opacity: 1, scale: 1 }}
        transition={{ duration: 0.4, ease: "easeOut" }}
        className="w-full h-full"
      >
        <Card className="relative w-full h-full overflow-hidden border-none shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl">
          <div className="flex flex-col md:flex-row h-full">
            {/* Left Panel: Hero & Info */}
            <div className="w-full md:w-[35%] lg:w-[40%] bg-linear-to-br from-emerald-50/50 to-teal-50/50 dark:from-emerald-900/10 dark:to-teal-900/10 p-8 flex flex-col justify-center relative min-h-[400px] h-full">
              <div className="absolute top-0 left-0 w-full h-full overflow-hidden pointer-events-none">
                <div className="absolute -top-20 -left-20 w-60 h-60 bg-emerald-400/20 rounded-full blur-3xl" />
                <div className="absolute bottom-0 right-0 w-60 h-60 bg-teal-400/20 rounded-full blur-3xl" />
              </div>

              <div className="relative z-10">
                <div className="w-16 h-16 mb-6 rounded-2xl bg-linear-to-br from-emerald-500 to-teal-600 shadow-lg shadow-emerald-900/20 flex items-center justify-center text-white">
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
                <PageHeader
                  title={t("onboarding.title", {
                    defaultValue: "首次启动引导",
                  })}
                  description={t("onboarding.subtitle", {
                    defaultValue: "请先设置内容路径与语言",
                  })}
                  titleClassName="text-3xl lg:text-4xl"
                />
              </div>
            </div>

            {/* Right Panel: Settings */}
            <div className="w-full md:w-[65%] lg:w-[60%] p-6 flex flex-col h-full overflow-hidden">
              <div className="flex-1 flex flex-col justify-center space-y-8 pr-2">
                {/* Paths Section */}
                <motion.div
                  initial={{ opacity: 0, y: 20 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.4, delay: 0.1 }}
                  className="bg-default-50/50 dark:bg-zinc-800/30 border border-default-100 dark:border-white/5 rounded-3xl p-6"
                >
                  <div className="mb-6">
                    <SectionHeader
                      title={t("settingscard.body.paths.title", {
                        defaultValue: "内容路径",
                      })}
                      description={t("settingscard.body.paths.subtitle", {
                        defaultValue: "管理游戏数据存储位置",
                      })}
                    />
                  </div>

                  <div className="space-y-6">
                    <Input
                      labelPlacement="outside"
                      label={t("settingscard.body.paths.base_root", {
                        defaultValue: "根目录",
                      })}
                      placeholder={t("settingscard.body.paths.base_root", {
                        defaultValue: "根目录",
                      })}
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
                              ? t("settingscard.body.paths.unsaved", {
                                  defaultValue: "更改未保存",
                                })
                              : t("settingscard.body.paths.not_writable", {
                                  defaultValue: "目录不可写入",
                                })}
                          </span>
                        ) : null
                      }
                      endContent={
                        <Button
                          size="sm"
                          variant="flat"
                          className="bg-default-200/50 dark:bg-white/10 font-medium"
                          onPress={() => {
                            navigate("/filemanager", {
                              state: {
                                directoryPickMode: true,
                                returnTo: "/onboarding",
                                returnState: {},
                                title: t("settingscard.body.paths.title", {
                                  defaultValue: "内容路径",
                                }),
                                initialPath: newBaseRoot || baseRoot || "",
                              },
                            });
                          }}
                        >
                          {t("common.browse", { defaultValue: "选择..." })}
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
                          {t("settingscard.body.paths.apply", {
                            defaultValue: "应用",
                          })}
                        </Button>
                        <Button
                          size="sm"
                          variant="light"
                          radius="full"
                          className="text-default-500 hover:text-default-700"
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
                          {t("settingscard.body.paths.reset", {
                            defaultValue: "恢复默认",
                          })}
                        </Button>
                      </div>
                    </div>
                  </div>
                </motion.div>

                {/* Language Section */}
                <motion.div
                  initial={{ opacity: 0, y: 20 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.4, delay: 0.2 }}
                  className="bg-default-50/50 dark:bg-zinc-800/30 border border-default-100 dark:border-white/5 rounded-3xl p-6"
                >
                  <SectionHeader
                    title={t("settingscard.body.language.name", {
                      defaultValue: t("app.lang"),
                    })}
                    description={
                      langNames.find((l) => l.code === selectedLang)
                        ?.language || selectedLang
                    }
                    action={
                      <Dropdown>
                        <DropdownTrigger>
                          <Button
                            radius="full"
                            variant="flat"
                            className="bg-default-100 dark:bg-white/10 font-medium"
                          >
                            {t("settingscard.body.language.button", {
                              defaultValue: "更改",
                            })}
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
                            const arr = Array.from(
                              keys as unknown as Set<string>,
                            );
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
                            <DropdownItem
                              key={lang.code}
                              textValue={lang.language}
                            >
                              {lang.language}
                            </DropdownItem>
                          ))}
                        </DropdownMenu>
                      </Dropdown>
                    }
                  />
                </motion.div>
              </div>

              {/* Bottom Actions */}
              <div className="flex items-center justify-end gap-3 pt-6 mt-auto">
                <Button
                  variant="light"
                  radius="full"
                  onPress={requestFinish}
                  className="font-medium text-default-500"
                >
                  {t("onboarding.skip", { defaultValue: "跳过" })}
                </Button>
                <Button
                  color="primary"
                  radius="full"
                  className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20 px-8"
                  onPress={requestFinish}
                >
                  {t("onboarding.finish", { defaultValue: "完成" })}
                </Button>
              </div>
            </div>
          </div>
        </Card>
      </motion.div>

      <BaseModal
        size="md"
        isOpen={unsavedOpen}
        onOpenChange={unsavedOnOpenChange}
        hideCloseButton
      >
        <ModalContent>
          {(onClose) => (
            <>
              <BaseModalHeader className="text-warning-600">
                {t("onboarding.unsaved.title", { defaultValue: "未保存修改" })}
              </BaseModalHeader>
              <BaseModalBody>
                <div className="text-default-700 text-sm">
                  {t("onboarding.unsaved.body", {
                    defaultValue:
                      "您更改了内容路径但尚未保存。是否保存后完成引导？",
                  })}
                </div>
                {!baseRootWritable && (
                  <div className="text-tiny text-danger-500 mt-1">
                    {t("settingscard.body.paths.not_writable", {
                      defaultValue: "目录不可写入",
                    })}
                  </div>
                )}
              </BaseModalBody>
              <BaseModalFooter>
                <Button variant="light" onPress={onClose}>
                  {t("onboarding.unsaved.cancel", { defaultValue: "取消" })}
                </Button>
                <Button
                  color="primary"
                  isLoading={savingBaseRoot}
                  isDisabled={!newBaseRoot || !baseRootWritable}
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
                          onClose();
                          proceedHome();
                        }
                      }
                    } catch {}
                    setSavingBaseRoot(false);
                  }}
                >
                  {t("onboarding.unsaved.save", { defaultValue: "保存并完成" })}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>
    </div>
  );
}
