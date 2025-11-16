import React, { useEffect, useMemo, useState } from "react";
import {
  Button,
  Input,
  Switch,
  Dropdown,
  DropdownTrigger,
  DropdownMenu,
  DropdownItem,
  Card,
  CardBody,
  CardHeader,
  Chip,
  Spinner,
  Progress,
} from "@heroui/react";
import { useLocation, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { FaChevronDown } from "react-icons/fa";
import { useVersionStatus } from "../utils/VersionStatusContext";
import { motion } from "framer-motion";
import * as minecraft from "../../bindings/github.com/liteldev/LeviLauncher/minecraft";

type ItemType = "Preview" | "Release";

export default function InstallPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation() as any;
  const { refreshAll } = useVersionStatus();

  const mirrorVersion: string = String(location?.state?.mirrorVersion || "");
  const mirrorType: ItemType = String(
    location?.state?.mirrorType || "Release"
  ) as ItemType;
  const returnTo: string = String(location?.state?.returnTo || "/download");

  const [installName, setInstallName] = useState<string>(mirrorVersion || "");
  const [installIsolation, setInstallIsolation] = useState<boolean>(true);
  const [inheritSource, setInheritSource] = useState<string>("");
  const [inheritMetas, setInheritMetas] = useState<any[]>([]);
  const [inheritCandidates, setInheritCandidates] = useState<string[]>([]);
  const [installError, setInstallError] = useState<string>("");
  const [installing, setInstalling] = useState<boolean>(false);
  const [installingVersion, setInstallingVersion] = useState<string>("");
  const [installingTargetName, setInstallingTargetName] = useState<string>("");
  const [resultMsg, setResultMsg] = useState<string>("");
  const [customInstallerPath, setCustomInstallerPath] = useState<string>("");
  const [installerDir, setInstallerDir] = useState<string>("");
  const [downloadResolved, setDownloadResolved] = useState<boolean>(false);

  useEffect(() => {
    const guardActive = installing && !resultMsg;
    try {
      (window as any).llNavLock = guardActive;
      window.dispatchEvent(
        new CustomEvent("ll-nav-lock-changed", {
          detail: { lock: guardActive },
        })
      );
    } catch {}
    const originalPush = window.history.pushState.bind(window.history);
    const originalReplace = window.history.replaceState.bind(window.history);

    if (guardActive) {
      (window.history as any).pushState = function (..._args: any[]) {
        return;
      } as any;
      (window.history as any).replaceState = function (..._args: any[]) {
        return;
      } as any;
    }

    return () => {
      (window.history as any).pushState = originalPush as any;
      (window.history as any).replaceState = originalReplace as any;
      try {
        (window as any).llNavLock = false;
        window.dispatchEvent(
          new CustomEvent("ll-nav-lock-changed", { detail: { lock: false } })
        );
      } catch {}
    };
  }, [installing, resultMsg]);

  useEffect(() => {
    const name = (installName || "").trim();
    if (!name) {
      setInstallError("ERR_NAME_REQUIRED");
      return;
    }
    let cancelled = false;
    const timer = setTimeout(async () => {
      try {
        const validate = minecraft?.ValidateVersionFolderName;
        if (typeof validate === "function") {
          const msg: string = await validate(name);
          if (!cancelled) setInstallError(msg || "");
        } else {
          if (!cancelled) setInstallError("");
        }
      } catch {
        if (!cancelled) setInstallError("");
      }
    }, 250);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [installName]);

  useEffect(() => {
    if (!installIsolation) {
      setInheritMetas([]);
      setInheritSource("");
      return;
    }
    try {
      const list = minecraft?.ListVersionMetas;
      if (typeof list === "function") {
        (async () => {
          try {
            const metas = await list();
            setInheritMetas(Array.isArray(metas) ? metas : []);
          } catch {
            setInheritMetas([]);
          }
        })();
      }
    } catch {
      setInheritMetas([]);
    }
  }, [installIsolation]);

  useEffect(() => {
    if (!installIsolation) {
      setInheritCandidates([]);
      return;
    }
    try {
      const listInh = minecraft?.ListInheritableVersionNames;
      if (typeof listInh === "function") {
        (async () => {
          try {
            const type = String(mirrorType || "Release").toLowerCase();
            const names: string[] = await listInh(type);
            setInheritCandidates(Array.isArray(names) ? names : []);
          } catch {
            setInheritCandidates([]);
          }
        })();
      }
    } catch {
      setInheritCandidates([]);
    }
  }, [installIsolation, mirrorType]);

  const inheritOptions = useMemo(() => {
    const type = String(mirrorType || "Release").toLowerCase();
    const allowed = new Set(
      (inheritCandidates || []).map((n) => String(n || ""))
    );
    return (inheritMetas || [])
      .filter(
        (m: any) =>
          Boolean(m?.enableIsolation) &&
          String(m?.type || "").toLowerCase() === type &&
          allowed.has(String(m?.name || ""))
      )
      .map((m: any) => ({
        key: String(m?.name || ""),
        label: `${m?.name || ""}${m?.gameVersion ? ` (${m.gameVersion})` : ""}`,
      }))
      .filter((x: any) => x.key);
  }, [inheritMetas, inheritCandidates, mirrorType]);

  const inheritLabel = useMemo(() => {
    const src = inheritSource || "none";
    if (src === "none")
      return t("downloadpage.install_folder.inherit_none") as unknown as string;
    if (src === "gdk")
      return t("downloadpage.install_folder.inherit_gdk") as unknown as string;
    return inheritOptions.find((o) => o.key === src)?.label || src;
  }, [inheritSource, inheritOptions]);

  useEffect(() => {
    try {
      const r = (location?.state as any)?.fileManagerResult;
      if (Array.isArray(r) && r.length > 0) {
        setCustomInstallerPath(String(r[0] || ""));
      }
    } catch {}
  }, [location?.state]);

  useEffect(() => {
    try {
      const getDir = minecraft?.GetInstallerDir;
      if (typeof getDir === "function") {
        (async () => {
          try {
            const d = await getDir();
            setInstallerDir(String(d || ""));
          } catch {}
        })();
      }
    } catch {}
  }, []);

  useEffect(() => {
    const checkResolved = async () => {
      if (!mirrorVersion) {
        setDownloadResolved(false);
        return;
      }
      try {
        const resolver = minecraft?.ResolveDownloadedMsixvc;
        if (typeof resolver === "function") {
          const name = await resolver(
            `${mirrorType || "Release"} ${mirrorVersion}`,
            String(mirrorType || "Release").toLowerCase()
          );
          setDownloadResolved(Boolean(name));
        } else {
          setDownloadResolved(false);
        }
      } catch {
        setDownloadResolved(false);
      }
    };
    checkResolved();
  }, [mirrorVersion, mirrorType]);

  const trErr = (msg: string): string => {
    const s = String(msg || "");
    if (!s) return "";
    if (s.startsWith("ERR_")) {
      const [code, ...restArr] = s.split(":");
      const codeTrim = code.trim();
      const rest = restArr.join(":").trim();
      const key = `errors.${codeTrim}`;
      const translated = t(key) as unknown as string;
      if (translated && translated !== key) {
        return rest ? `${translated} (${rest})` : translated;
      }
      return s;
    }
    return s;
  };

  const headerTitle = useMemo(() => {
    if (installing)
      return t("downloadmodal.installing.title", {
        defaultValue: "正在安装",
      }) as unknown as string;
    if (resultMsg)
      return t("downloadpage.install.success_title", {
        defaultValue: "安装完成",
      }) as unknown as string;
    return t("downloadpage.install_folder.confirm_title") as unknown as string;
  }, [installing, resultMsg, t]);

  const handleInstall = async () => {
    setInstallError("");
    setResultMsg("");
    const name = (installName || "").trim();
    if (!name) {
      setInstallError("ERR_NAME_REQUIRED");
      return;
    }
    try {
      const validate = minecraft?.ValidateVersionFolderName;
      if (typeof validate === "function") {
        const msg: string = await validate(name);
        if (msg) {
          setInstallError(msg);
          return;
        }
      }
    } catch {}

    try {
      const install = minecraft?.InstallExtractMsixvc;
      const saveMeta = minecraft?.SaveVersionMeta;
      const copyFromGDK = minecraft?.CopyVersionDataFromGDK;
      const copyFromVersion = minecraft?.CopyVersionDataFromVersion;
      const resolver = minecraft?.ResolveDownloadedMsixvc;
      const isPrev = (mirrorType || "Release") === "Preview";
      let fname = "";
      if (customInstallerPath && customInstallerPath.trim().length > 0) {
        fname = customInstallerPath.trim();
      } else if (typeof resolver === "function") {
        try {
          fname = await resolver(
            (mirrorType || "Release") +
              " " +
              (mirrorVersion || installName || ""),
            String(mirrorType || "Release").toLowerCase()
          );
        } catch {}
      }
      if (!fname) {
        setInstallError("ERR_MSIXVC_NOT_SPECIFIED");
        return;
      }

      setInstalling(true);
      setInstallingVersion(mirrorVersion || installName || "");
      try {
        const disp = fname?.toLowerCase().endsWith(".msixvc")
          ? fname
          : `${fname}.msixvc`;
        setInstallingTargetName(disp);
      } catch {
        setInstallingTargetName(fname);
      }

      if (typeof install === "function") {
        const err: string = await install(fname, name, isPrev);
        if (err) {
          setInstallError(err);
          setInstalling(false);
          return;
        }
      }

      if (typeof saveMeta === "function") {
        await saveMeta(
          name,
          mirrorVersion || name,
          String(mirrorType || "Release").toLowerCase(),
          installIsolation,
          false,
          false
        );
      }

      if (installIsolation && inheritSource) {
        try {
          let copyErr: string = "";
          if (inheritSource === "gdk") {
            if (typeof copyFromGDK === "function")
              copyErr = await copyFromGDK(isPrev, name);
          } else {
            if (typeof copyFromVersion === "function")
              copyErr = await copyFromVersion(inheritSource, name);
          }
          if (copyErr) {
            setInstallError(copyErr);
            setInstalling(false);
            return;
          }
        } catch (e: any) {
          setInstallError(String(e?.message || e || ""));
          setInstalling(false);
          return;
        }
      }

      try {
        await refreshAll();
      } catch {}
      setResultMsg(
        t("downloadpage.install.success", {
          defaultValue: "安装完成",
        }) as unknown as string
      );
      setInstalling(false);
    } catch (e: any) {
      setInstallError(String(e?.message || e || ""));
      setInstalling(false);
    }
  };

  return (
    <div className="w-full h-full max-w-[100vw] flex flex-col overflow-x-hidden gutter-stable overflow-auto no-scrollbar">
      <div className="px-3 sm:px-5 lg:px-8 py-3 sm:py-4 lg:py-6 w-full flex flex-col flex-1 min-h-0">
        <motion.div
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.25 }}
        >
          <Card className="rounded-3xl shadow-lg mb-4 mx-3 sm:mx-5 lg:mx-8 bg-white/60 dark:bg-black/30 backdrop-blur-md border border-white/30">
            <CardHeader className="flex flex-col gap-3 p-3 sm:p-4">
              <div className="flex w-full items-center justify-between gap-2">
                <div className="flex items-center gap-3">
                  <div className="text-2xl font-bold">{headerTitle}</div>
                  <div className="text-small text-default-500 flex items-center gap-2">
                    <Chip
                      size="sm"
                      variant="flat"
                      color={mirrorType === "Preview" ? "warning" : "primary"}
                    >
                      {mirrorType}
                    </Chip>
                    <span className="font-mono">{mirrorVersion}</span>
                  </div>
                </div>
              </div>
            </CardHeader>

            <CardBody className="flex flex-col gap-4 p-4">
              {installError ? (
                <div className="rounded-xl border border-danger bg-danger-50 px-3 py-2">
                  <div className="text-danger font-medium">
                    {t("common.error", { defaultValue: "错误" })}
                  </div>
                  <div className="text-small text-danger-600">
                    {trErr(installError)}
                  </div>
                </div>
              ) : null}

              {resultMsg && !installing ? (
                <div className="rounded-2xl border border-success bg-success-50 px-4 py-3 flex flex-col gap-3">
                  <div className="flex items-center gap-3">
                    <div className="w-10 h-10 rounded-full bg-success-500 flex items-center justify-center">
                      <svg
                        viewBox="0 0 24 24"
                        width="20"
                        height="20"
                        className="text-white"
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="3"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      >
                        <path d="M20 6L9 17l-5-5" />
                      </svg>
                    </div>
                    <div className="flex flex-col">
                      <div className="text-lg font-bold text-success-700">
                        {t("downloadpage.install.success_title", {
                          defaultValue: "安装完成",
                        })}
                      </div>
                      {installingVersion ? (
                        <div className="text-small text-success-600">
                          {t("downloadpage.install.version_label", {
                            defaultValue: "版本",
                          })}
                          : {installingVersion}
                        </div>
                      ) : null}
                    </div>
                  </div>
                  <div className="text-small text-success-700">{resultMsg}</div>
                  {installingTargetName ? (
                    <div className="rounded-medium bg-success-100 border border-success-200 px-3 py-2 text-small text-success-700">
                      {t("downloadpage.install.target", {
                        defaultValue: "安装目标",
                      })}
                      :{" "}
                      <span className="font-mono">{installingTargetName}</span>
                    </div>
                  ) : null}
                  <div className="mt-1 flex flex-wrap justify-end gap-2">
                    <Button variant="light" onPress={() => navigate(returnTo)}>
                      {t("common.back", { defaultValue: "返回" })}
                    </Button>
                  </div>
                </div>
              ) : null}

              {!installing && !resultMsg ? (
                <>
                  <Input
                    label={
                      t("downloadpage.install_folder.name_label", {
                        defaultValue: "版本名称",
                      }) as unknown as string
                    }
                    placeholder={
                      t("downloadpage.install_folder.placeholder_name", {
                        defaultValue: "请输入版本名称",
                      }) as unknown as string
                    }
                    value={installName}
                    onValueChange={setInstallName}
                    isInvalid={!!installError}
                    errorMessage={
                      installError ? trErr(installError) : undefined
                    }
                    variant="bordered"
                    size="sm"
                  />
                  {(!downloadResolved) ? (
                    <div className="flex items-center justify-between">
                      <div className="min-w-0">
                        <div className="text-small font-medium">
                          {t("downloadpage.install.custom_installer.label", {
                            defaultValue: "选择本地安装包 (.msixvc)",
                          })}
                        </div>
                        <div className="text-tiny text-default-500">
                          {customInstallerPath
                            ? customInstallerPath
                            : (t("downloadpage.install.custom_installer.hint", {
                                defaultValue:
                                  "默认使用安装器目录下已下载的文件",
                              }) as unknown as string)}
                        </div>
                      </div>
                      <Button
                        variant="bordered"
                        size="sm"
                        onPress={() => {
                          navigate("/filemanager", {
                            state: {
                              allowedExt: [".msixvc"],
                              multi: false,
                              title: t("downloadpage.customappx.modal.1.header", {
                                defaultValue: "版本信息",
                              }),
                              initialPath: installerDir || "",
                              returnTo: "/install",
                              returnState: {
                                mirrorVersion,
                                mirrorType,
                                returnTo,
                              },
                            },
                          });
                        }}
                      >
                        {t("common.browse", { defaultValue: "选择..." })}
                      </Button>
                    </div>
                  ) : null}
                  <div className="flex items-center justify-between">
                    <div className="min-w-0">
                      <div className="text-small font-medium">
                        {t("downloadpage.install_folder.enable_isolation")}
                      </div>
                      <div className="text-tiny text-default-500">
                        {t("downloadpage.install_folder.enable_isolation_desc")}
                      </div>
                    </div>
                    <Switch
                      isSelected={installIsolation}
                      onValueChange={setInstallIsolation}
                    />
                  </div>
                  {installIsolation ? (
                    <div className="flex items-center justify-between">
                      <div className="min-w-0">
                        <div className="text-small font-medium">
                          {t("downloadpage.install_folder.inherit_label")}
                        </div>
                        <div className="text-tiny text-default-500">
                          {t("downloadpage.install_folder.inherit_hint")}
                        </div>
                      </div>
                      <div className="shrink-0 min-w-[240px]">
                        <Dropdown closeOnSelect>
                          <DropdownTrigger>
                            <Button
                              variant="bordered"
                              size="sm"
                              className="w-full justify-between"
                              endContent={<FaChevronDown size={12} />}
                            >
                              {inheritLabel}
                            </Button>
                          </DropdownTrigger>
                          <DropdownMenu
                            aria-label="inherit-source-select"
                            selectionMode="single"
                            disallowEmptySelection
                            selectedKeys={new Set([inheritSource || "none"])}
                            className="max-h-64 overflow-y-auto min-w-[240px] no-scrollbar"
                            onSelectionChange={(keys) => {
                              const arr = Array.from(
                                keys as unknown as Set<string>
                              );
                              const k = String(arr[0] || "");
                              if (!k) return;
                              setInheritSource(k === "none" ? "" : k);
                            }}
                          >
                            <DropdownItem key="none">
                              {t("downloadpage.install_folder.inherit_none")}
                            </DropdownItem>
                            <DropdownItem key="gdk">
                              {t("downloadpage.install_folder.inherit_gdk")}
                            </DropdownItem>
                            {inheritOptions.map((opt) => (
                              <DropdownItem key={opt.key}>
                                {opt.label}
                              </DropdownItem>
                            ))}
                          </DropdownMenu>
                        </Dropdown>
                      </div>
                    </div>
                  ) : null}

                  <div className="flex justify-end gap-2">
                    <Button variant="light" onPress={() => navigate(returnTo)}>
                      {t("common.cancel")}
                    </Button>
                    <Button color="primary" onPress={handleInstall}>
                      {t(
                        "downloadpage.customappx.modal.1.footer.install_button",
                        { defaultValue: "安装" }
                      )}
                    </Button>
                  </div>
                </>
              ) : null}

              {installing ? (
                <div className="flex flex-col gap-3">
                  <div className="flex items-center gap-3">
                    <div className="w-10 h-10 rounded-full bg-content2 border border-default-200 flex items-center justify-center">
                      <Spinner size="sm" />
                    </div>
                    <div className="flex flex-col">
                      <h2 className="text-lg font-bold">
                        {t("downloadmodal.installing.title", {
                          defaultValue: "正在安装",
                        })}
                      </h2>
                      {installingVersion ? (
                        <div className="text-small text-default-500">
                          {t("downloadpage.install.version_label", {
                            defaultValue: "版本",
                          })}
                          : {installingVersion}
                        </div>
                      ) : null}
                    </div>
                  </div>
                  <div className="text-small text-default-500">
                    {t("downloadpage.install.hint", {
                      defaultValue: "请稍候，正在卸载旧版本并注册安装包...",
                    })}
                  </div>
                  <div className="text-small text-default-700">
                    {t("downloadpage.install.target", {
                      defaultValue: "安装目标",
                    })}
                    : <span className="font-mono">{installingTargetName}</span>
                  </div>
                  <Progress
                    aria-label="install-progress"
                    isIndeterminate
                    value={undefined}
                    className="flex-1"
                  />
                </div>
              ) : null}
            </CardBody>
          </Card>
        </motion.div>
      </div>
    </div>
  );
}
