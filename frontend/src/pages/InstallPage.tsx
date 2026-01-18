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
import { useVersionStatus } from "@/utils/VersionStatusContext";
import { motion } from "framer-motion";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { PageHeader } from "@/components/PageHeader";

type ItemType = "Preview" | "Release";

export default function InstallPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation() as any;
  const { refreshAll } = useVersionStatus();

  const mirrorVersion: string = String(location?.state?.mirrorVersion || "");
  const mirrorType: ItemType = String(
    location?.state?.mirrorType || "Release",
  ) as ItemType;
  const typeLabel: string = (mirrorType === "Preview"
    ? (t("common.preview") as unknown as string)
    : (t("common.release") as unknown as string)) as unknown as string;
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
        }),
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
          new CustomEvent("ll-nav-lock-changed", { detail: { lock: false } }),
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
      (inheritCandidates || []).map((n) => String(n || "")),
    );
    return (inheritMetas || [])
      .filter(
        (m: any) =>
          Boolean(m?.enableIsolation) &&
          String(m?.type || "").toLowerCase() === type &&
          allowed.has(String(m?.name || "")),
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

  const inheritMenuItems = useMemo(
    () => [
      {
        key: "none",
        label: t(
          "downloadpage.install_folder.inherit_none",
        ) as unknown as string,
      },
      {
        key: "gdk",
        label: t(
          "downloadpage.install_folder.inherit_gdk",
        ) as unknown as string,
      },
      ...inheritOptions,
    ],
    [inheritOptions, t],
  );

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
            String(mirrorType || "Release").toLowerCase(),
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

  const trErr = (msg: string, typeLabelOverride?: string): string => {
    const s = String(msg || "");
    if (!s) return "";
    if (s.startsWith("ERR_")) {
      const [code, ...restArr] = s.split(":");
      const codeTrim = code.trim();
      const rest = restArr.join(":").trim();
      const key = `errors.${codeTrim}`;
      const translated = t(key, {
        typeLabel: typeLabelOverride || typeLabel,
      }) as unknown as string;
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
            String(mirrorType || "Release").toLowerCase(),
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
          false,
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
        let cachedItems: { version: string; short: string; type: ItemType }[] =
          [];
        try {
          const raw = localStorage.getItem("ll.version_items");
          const parsed = raw ? JSON.parse(raw) : [];
          if (Array.isArray(parsed)) {
            cachedItems = parsed.map((it: any) => ({
              version: String(it?.version || it?.short || ""),
              short: String(it?.short || it?.version || ""),
              type: String(it?.type || "Release") as ItemType,
            }));
          }
        } catch {}
        const itemsToRefresh =
          cachedItems && cachedItems.length > 0
            ? cachedItems
            : [
                {
                  version: String(mirrorVersion || installName || ""),
                  short: String(mirrorVersion || installName || ""),
                  type: (mirrorType || "Release") as ItemType,
                },
              ];
        await refreshAll(itemsToRefresh as any);
      } catch {}
      setResultMsg(
        t("downloadpage.install.success", {
          defaultValue: "安装完成",
        }) as unknown as string,
      );
      setInstalling(false);
    } catch (e: any) {
      setInstallError(String(e?.message || e || ""));
      setInstalling(false);
    }
  };

  return (
    <motion.div
      className="w-full max-w-full mx-auto p-4 h-full flex flex-col"
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.25 }}
    >
      <div className="flex flex-col h-full">
        <Card className="flex-1 min-h-0 border-none shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl">
          <CardHeader className="p-4 sm:p-6 pb-2 block border-b border-default-200 dark:border-white/10">
            <PageHeader
              title={headerTitle}
              description={
                <div className="flex items-center gap-2">
                  <Chip
                    size="sm"
                    variant="flat"
                    color={mirrorType === "Preview" ? "warning" : "primary"}
                  >
                    {mirrorType === "Preview"
                      ? `${t("common.preview")} Minecraft`
                      : `${t("common.release")} Minecraft`}
                  </Chip>
                  <span className="font-mono">{mirrorVersion}</span>
                </div>
              }
            />
          </CardHeader>

          <CardBody className="flex flex-col gap-4 p-4 overflow-y-auto">
            {installError ? (
              <motion.div
                initial={{ opacity: 0, height: 0 }}
                animate={{ opacity: 1, height: "auto" }}
                className="rounded-xl border border-danger bg-danger-50 px-3 py-2"
              >
                <div className="text-danger font-medium">
                  {t("common.error", { defaultValue: "错误" })}
                </div>
                <div className="text-small text-danger-600">
                  {trErr(installError, typeLabel)}
                </div>
              </motion.div>
            ) : null}

            {resultMsg && !installing ? (
              <motion.div
                initial={{ opacity: 0, scale: 0.95 }}
                animate={{ opacity: 1, scale: 1 }}
                className="flex flex-col items-center justify-center h-full gap-4 py-4"
              >
                <div className="relative">
                  <motion.div
                    initial={{ scale: 0 }}
                    animate={{ scale: 1 }}
                    transition={{
                      type: "spring",
                      stiffness: 260,
                      damping: 20,
                      delay: 0.1,
                    }}
                    className="w-16 h-16 rounded-full bg-linear-to-br from-emerald-400 to-teal-600 flex items-center justify-center shadow-lg shadow-emerald-900/20"
                  >
                    <svg
                      viewBox="0 0 24 24"
                      width="32"
                      height="32"
                      className="text-white drop-shadow-md"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="3"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    >
                      <motion.path
                        initial={{ pathLength: 0 }}
                        animate={{ pathLength: 1 }}
                        transition={{ duration: 0.5, delay: 0.3 }}
                        d="M20 6L9 17l-5-5"
                      />
                    </svg>
                  </motion.div>
                </div>

                <div className="flex flex-col items-center gap-1 text-center">
                  <h2 className="text-2xl font-black bg-linear-to-br from-emerald-600 to-teal-600 dark:from-emerald-500 dark:to-teal-500 bg-clip-text text-transparent">
                    {t("downloadpage.install.success_title", {
                      defaultValue: "安装完成",
                    })}
                  </h2>
                  {installingVersion && (
                    <Chip
                      variant="flat"
                      color="success"
                      size="sm"
                      classNames={{ content: "font-bold" }}
                    >
                      {installingVersion}
                    </Chip>
                  )}
                </div>

                <div className="w-full max-w-lg mt-1">
                  {installingTargetName && (
                    <div className="rounded-xl bg-default-100/50 dark:bg-zinc-800/50 border border-default-200/50 dark:border-white/5 p-3 flex flex-col gap-1 items-center">
                      <span className="text-[10px] uppercase tracking-wider text-default-400 font-bold">
                        {t("downloadpage.install.target", {
                          defaultValue: "安装目标",
                        })}
                      </span>
                      <span className="font-mono text-xs text-default-600 dark:text-zinc-400 truncate w-full text-center">
                        {installingTargetName}
                      </span>
                    </div>
                  )}
                </div>

                <div className="mt-2">
                  <Button
                    className="font-bold text-white shadow-lg shadow-emerald-900/20 bg-emerald-600 hover:bg-emerald-500 px-8"
                    radius="full"
                    size="md"
                    onPress={() => navigate(returnTo)}
                  >
                    {t("common.back", { defaultValue: "返回" })}
                  </Button>
                </div>
              </motion.div>
            ) : null}

            {!installing && !resultMsg ? (
              <>
                <motion.div
                  initial={{ opacity: 0, y: 10 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ delay: 0.1 }}
                >
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
                      installError ? trErr(installError, typeLabel) : undefined
                    }
                    variant="bordered"
                    size="sm"
                  />
                </motion.div>
                {!downloadResolved ? (
                  <motion.div
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: 0.2 }}
                    className="flex items-center justify-between p-3 rounded-2xl bg-default-50/50 dark:bg-default-100/10 border border-default-100 dark:border-white/5"
                  >
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
                              defaultValue: "默认使用安装器目录下已下载的文件",
                            }) as unknown as string)}
                      </div>
                    </div>
                    <Button
                      variant="flat"
                      size="sm"
                      className="bg-default-100 dark:bg-white/10"
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
                  </motion.div>
                ) : null}

                <motion.div
                  initial={{ opacity: 0, y: 10 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ delay: 0.3 }}
                  className="flex items-center justify-between p-3 rounded-2xl bg-default-50/50 dark:bg-default-100/10 border border-default-100 dark:border-white/5"
                >
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
                    color="success"
                  />
                </motion.div>
                {installIsolation ? (
                  <motion.div
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: 0.4 }}
                    className="flex items-center justify-between p-3 rounded-2xl bg-default-50/50 dark:bg-default-100/10 border border-default-100 dark:border-white/5"
                  >
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
                            variant="flat"
                            size="sm"
                            className="bg-default-100 dark:bg-white/10 w-full justify-between"
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
                          items={inheritMenuItems}
                          onSelectionChange={(keys) => {
                            const arr = Array.from(
                              keys as unknown as Set<string>,
                            );
                            const k = String(arr[0] || "");
                            if (!k) return;
                            setInheritSource(k === "none" ? "" : k);
                          }}
                        >
                          {(item: { key: string; label: string }) => (
                            <DropdownItem key={item.key}>
                              {item.label}
                            </DropdownItem>
                          )}
                        </DropdownMenu>
                      </Dropdown>
                    </div>
                  </motion.div>
                ) : null}

                <motion.div
                  initial={{ opacity: 0, y: 10 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ delay: 0.5 }}
                  className="flex justify-end gap-2"
                >
                  <Button variant="light" onPress={() => navigate(returnTo)}>
                    {t("common.cancel")}
                  </Button>
                  <Button
                    className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
                    radius="full"
                    onPress={handleInstall}
                  >
                    {t(
                      "downloadpage.customappx.modal.1.footer.install_button",
                      { defaultValue: "安装" },
                    )}
                  </Button>
                </motion.div>
              </>
            ) : null}

            {installing ? (
              <motion.div
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                className="flex flex-col items-center justify-center h-full gap-4 py-6"
              >
                <div className="relative flex items-center justify-center">
                  <div className="absolute inset-0 bg-emerald-500/20 blur-xl rounded-full animate-pulse" />
                  <div className="w-16 h-16 rounded-full bg-default-50 dark:bg-zinc-800 border-4 border-default-100 dark:border-zinc-700 flex items-center justify-center relative z-10">
                    <Spinner
                      size="md"
                      color="success"
                      classNames={{ wrapper: "w-8 h-8" }}
                    />
                  </div>
                </div>

                <div className="flex flex-col items-center gap-1 text-center max-w-sm">
                  <h2 className="text-xl font-bold text-default-900 dark:text-white">
                    {t("downloadmodal.installing.title", {
                      defaultValue: "正在安装",
                    })}
                  </h2>
                  <p className="text-small text-default-500">
                    {t("downloadpage.install.hint", {
                      defaultValue: "请稍候，正在卸载旧版本并注册安装包...",
                    })}
                  </p>
                </div>

                <div className="w-full max-w-lg flex flex-col gap-2">
                  {installingVersion && (
                    <div className="flex items-center justify-between px-3 py-2 rounded-xl bg-default-100/50 dark:bg-zinc-800/50">
                      <span className="text-small font-medium text-default-500">
                        {t("downloadpage.install.version_label", {
                          defaultValue: "版本",
                        })}
                      </span>
                      <span className="text-small font-bold text-default-700 dark:text-zinc-300">
                        {installingVersion}
                      </span>
                    </div>
                  )}

                  {installingTargetName && (
                    <div className="flex flex-col gap-1 px-3 py-2 rounded-xl bg-default-100/50 dark:bg-zinc-800/50">
                      <span className="text-[10px] font-bold uppercase tracking-wider text-default-400">
                        {t("downloadpage.install.target", {
                          defaultValue: "安装目标",
                        })}
                      </span>
                      <span className="font-mono text-xs text-default-600 dark:text-zinc-400 truncate">
                        {installingTargetName}
                      </span>
                    </div>
                  )}

                  <div className="mt-1">
                    <Progress
                      aria-label="install-progress"
                      isIndeterminate
                      size="sm"
                      color="success"
                      classNames={{
                        indicator:
                          "bg-linear-to-r from-emerald-500 to-teal-500",
                        track:
                          "bg-default-200/50 dark:bg-zinc-700/50 border border-default-100 dark:border-white/5",
                      }}
                      className="w-full"
                    />
                  </div>
                </div>
              </motion.div>
            ) : null}
          </CardBody>
        </Card>
      </div>
    </motion.div>
  );
}
