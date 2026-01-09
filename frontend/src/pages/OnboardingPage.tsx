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
  Modal,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  useDisclosure,
} from "@heroui/react";
import { useNavigate, useLocation } from "react-router-dom";
import {
  GetLanguageNames,
  GetBaseRoot,
  SetBaseRoot,
  ResetBaseRoot,
  GetInstallerDir,
  GetVersionsDir,
  CanWriteToDir,
} from "../../bindings/github.com/liteldev/LeviLauncher/minecraft";
import * as minecraft from "../../bindings/github.com/liteldev/LeviLauncher/minecraft";

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
    if (hasBackend) {
      GetLanguageNames().then((res: any) => setLangNames(res));
    } else {
      setLangNames([
        { language: "English", code: "en_US" },
        { language: "简体中文", code: "zh_CN" },
        { language: "繁体中文", code: "zh_HK" },
        { language: "日本語", code: "ja_JP" },
      ]);
    }
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
              : String(br || "")
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
    <div className="px-3 sm:px-5 lg:px-8 py-3 sm:py-4 lg:py-6 w-full max-w-none">
      <motion.div
        initial={{ opacity: 0, y: 8 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.25 }}
      >
        <Card className="rounded-3xl shadow-xl p-2 bg-white/60 dark:bg-black/30 backdrop-blur-md border border-white/30">
          <CardHeader className="flex flex-col items-start px-4 pb-0 pt-4">
            <div className="text-large font-semibold">
              {t("onboarding.title", { defaultValue: "首次启动引导" })}
            </div>
            <p className="text-small text-default-500">
              {t("onboarding.subtitle", {
                defaultValue: "请先设置内容路径与语言",
              })}
            </p>
          </CardHeader>
          <Divider className="my-2 bg-default-200/60 h-px" />
          <CardBody className="space-y-3">
            <motion.div
              initial={{ opacity: 0, y: 6 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.25, delay: 0.04 }}
              className="flex items-center justify-between"
            >
              <div className="min-w-0">
                <p className="font-medium">
                  {t("settingscard.body.paths.title", {
                    defaultValue: "内容路径",
                  })}
                </p>
                {newBaseRoot && newBaseRoot !== baseRoot ? (
                  <p
                    className={`text-tiny mt-1 ${
                      baseRootWritable ? "text-warning-500" : "text-danger-500"
                    } truncate`}
                    title={newBaseRoot}
                  >
                    {baseRootWritable
                      ? t("settingscard.body.paths.base_root", {
                          defaultValue: "根目录",
                        }) +
                        ": " +
                        newBaseRoot
                      : t("settingscard.body.paths.not_writable", {
                          defaultValue: "目录不可写入",
                        })}
                  </p>
                ) : null}
              </div>
              <div className="flex items-center gap-1.5 shrink-0">
                <Button
                  color="primary"
                  radius="full"
                  isDisabled={!newBaseRoot || !baseRootWritable}
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
                  {t("settingscard.body.paths.apply", { defaultValue: "应用" })}
                </Button>
                <Button
                  variant="light"
                  radius="full"
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
            </motion.div>
            <div className="mt-1">
              <div className="flex flex-col">
                <Input
                  label={
                    t("settingscard.body.paths.base_root", {
                      defaultValue: "根目录",
                    }) as string
                  }
                  value={newBaseRoot}
                  onValueChange={setNewBaseRoot}
                  endContent={
                    <Button
                      size="sm"
                      variant="flat"
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
                {!baseRootWritable ? (
                  <div className="text-tiny text-danger-500 mt-1">
                    {t("settingscard.body.paths.not_writable", {
                      defaultValue: "目录不可写入",
                    })}
                  </div>
                ) : null}
              </div>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
              <div className="text-tiny text-default-500">
                {t("settingscard.body.paths.installer", {
                  defaultValue: "安装器目录",
                })}
                : {installerDir || "-"}
              </div>
              <div className="text-tiny text-default-500">
                {t("settingscard.body.paths.versions", {
                  defaultValue: "版本目录",
                })}
                : {versionsDir || "-"}
              </div>
            </div>

            <Divider className="my-2 bg-default-200/60 h-px" />

            <motion.div
              initial={{ opacity: 0, y: 6 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.25, delay: 0.06 }}
              className="flex items-center justify-between"
            >
              <div>
                <p>
                  {t("settingscard.body.language.name", {
                    defaultValue: t("app.lang"),
                  })}
                </p>
                <p className="text-small text-default-500">
                  {langNames.find((l) => l.code === selectedLang)?.language ||
                    selectedLang}
                </p>
              </div>
              <Dropdown>
                <DropdownTrigger>
                  <Button radius="full" variant="bordered">
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
                    const arr = Array.from(keys as unknown as Set<string>);
                    const next = arr[0];
                    if (typeof next === "string" && next.length > 0) {
                      setSelectedLang(next);
                      Promise.resolve(i18n.changeLanguage(next)).then(() => {
                        try {
                          localStorage.setItem("i18nextLng", next);
                        } catch {}
                      });
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
            </motion.div>

            <Divider className="my-2 bg-default-200/60 h-px" />

            <div className="flex items-center justify-end gap-2">
              <Button variant="light" onPress={requestFinish}>
                {t("onboarding.skip", { defaultValue: "跳过" })}
              </Button>
              <Button color="primary" onPress={requestFinish}>
                {t("onboarding.finish", { defaultValue: "完成" })}
              </Button>
            </div>
          </CardBody>
        </Card>
      </motion.div>

      <Modal
        size="md"
        isOpen={unsavedOpen}
        onOpenChange={unsavedOnOpenChange}
        hideCloseButton
      >
        <ModalContent>
          {(onClose) => (
            <>
              <ModalHeader className="text-warning-600">
                {t("onboarding.unsaved.title", { defaultValue: "未保存修改" })}
              </ModalHeader>
              <ModalBody>
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
              </ModalBody>
              <ModalFooter>
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
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>
    </div>
  );
}
