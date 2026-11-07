import {
  ModalDescription,
  ModalDetails,
  ModalNotice,
} from "@/components/ModalPrimitives";
import { Label, ListBox, Select, Spinner } from "@heroui/react";
import React, { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  LuCircleCheck,
  LuCircleDashed,
  LuCircleMinus,
  LuCircleX,
  LuDownload,
} from "react-icons/lu";
import {
  GetLIPPackageInstallStates,
  InstallLeviLamina,
  InstallLIPPackage,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import { GetVersionLogoDataUrl } from "bindings/github.com/liteldev/LeviLauncher/internal/app/versionservice";
import { useInstanceList } from "@/components/InstancePicker";
import { UnifiedModal } from "@/components/UnifiedModal";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import {
  appendTaskLog,
  finishActivityTask,
  notify,
  startActivityTask,
} from "@/utils/activity";
import { cn } from "@/utils/cn";
import {
  fetchLIPPackageDetail,
  isLeviLaminaVersionCompatible,
  resolveSupportedGameVersionsByLLRanges,
  type LIPPackageDetail,
  type LIPPackageFileInfo,
  type LIPPackageVariantDetail,
} from "@/utils/content";
import { useCurrentVersion } from "@/utils/CurrentVersionContext";
import { useLeviLamina } from "@/utils/LeviLaminaContext";
import {
  LipTaskProgress,
  useLipTaskConsole,
} from "@/utils/LipTaskConsoleContext";
import {
  normalizeIdentifierLookupKey,
  parseLIPPackageInstallState,
  type LIPPackageInstallState,
} from "@/utils/modIntelligenceResolver";
import { compareVersions } from "@/utils/version";

type ActionKind = "install" | "upgrade" | "downgrade" | "reinstall";

type StepStatus = "pending" | "running" | "success" | "error" | "skipped";

type InstallStep = {
  id: string;
  label: string;
  status: StepStatus;
  message?: string;
};

type Phase = "idle" | "running" | "success" | "error";

type LoadedState = LIPPackageInstallState & { loading: boolean };

type InstanceStates = { ll: LoadedState; pkg: LoadedState };

const LEVILAMINA_IDENTIFIER = "LiteLDev/LeviLamina";

const ACTION_LABEL_KEYS: Record<ActionKind, string> = {
  install: "lip.files.install",
  upgrade: "lip.files.upgrade",
  downgrade: "lip.files.downgrade",
  reinstall: "lip.files.reinstall",
};

const LOADING_STATE: LoadedState = {
  installed: false,
  explicitInstalled: false,
  installedVersion: "",
  error: "",
  loading: true,
};

const FALLBACK_LOGO =
  "https://raw.githubusercontent.com/LiteLDev/LeviLauncher/main/build/appicon.png";

// Instance logos, shared across dialog opens.
const logoCache = new Map<string, Promise<string>>();
const loadLogo = (name: string): Promise<string> => {
  let pending = logoCache.get(name);
  if (!pending) {
    pending = Promise.resolve(GetVersionLogoDataUrl(name))
      .then((url) => String(url || ""))
      .catch(() => "");
    logoCache.set(name, pending);
  }
  return pending;
};

const getErrorCode = (value: unknown): string => {
  const raw = String(
    value instanceof Error ? value.message : (value ?? ""),
  ).trim();
  if (!raw) return "";
  const match = raw.match(/ERR_[A-Z0-9_]+/);
  return match ? match[0] : raw;
};

const isClientVariant = (variant: LIPPackageVariantDetail): boolean =>
  String(variant.key || "")
    .trim()
    .toLowerCase()
    .includes("client");

/** Variant label shown to users, falling back to its key. */
export const getLIPVariantDisplayLabel = (
  variant: Pick<LIPPackageVariantDetail, "key" | "label"> | null | undefined,
  defaultLabel: string,
): string =>
  String(variant?.label || "").trim() ||
  String(variant?.key || "").trim() ||
  defaultLabel;

/** The variant installed when the user has not picked one. */
export const resolveDefaultLIPVariant = (
  pkg: LIPPackageDetail | null,
): LIPPackageVariantDetail | null => {
  const variants = (pkg?.variantDetails || []).filter(isClientVariant);
  if (variants.length === 0) return null;
  const preferred = String(pkg?.preferredVariantKey || "")
    .trim()
    .toLowerCase();
  return (
    (preferred &&
      variants.find(
        (variant) =>
          String(variant.key || "")
            .trim()
            .toLowerCase() === preferred,
      )) ||
    variants[0]
  );
};

type Props = {
  isOpen: boolean;
  onClose: () => void;
  /** Package to install; its detail is loaded when `pkg` is not given. */
  identifier: string;
  pkg?: LIPPackageDetail | null;
  /** Variant picked on the package page; defaults to the preferred one. */
  variant?: LIPPackageVariantDetail | null;
  /** Version picked by the user; the newest compatible one otherwise. */
  initialVersion?: string;
  onInstalled?: (instanceName: string) => void;
};

/**
 * Installs a lip package on an instance in one confirm. A missing or
 * incompatible LeviLamina is installed first in the same run, so the user
 * never has to leave the dialog to satisfy the prerequisite.
 */
export const LIPInstallDialog: React.FC<Props> = (props) => {
  // Mounted on first open only, so pages that never install pay nothing.
  const [used, setUsed] = useState(props.isOpen);
  useEffect(() => {
    if (props.isOpen) setUsed(true);
  }, [props.isOpen]);
  return used || props.isOpen ? <LIPInstallDialogBody {...props} /> : null;
};

const LIPInstallDialogBody: React.FC<Props> = ({
  isOpen,
  onClose,
  identifier,
  pkg: providedPkg,
  variant: providedVariant,
  initialVersion,
  onInstalled,
}) => {
  const { t } = useTranslation();
  const { runWithLipTask } = useLipTaskConsole();
  const { currentVersionName } = useCurrentVersion();
  const { instances, loading: instancesLoading } = useInstanceList();
  const {
    gameToLLVersions,
    getSupportedLLVersions,
    mappingAvailable,
    mappingInitialized,
    loading: mappingLoading,
    normalizeGameVersion,
  } = useLeviLamina();

  const [loadedPkg, setLoadedPkg] = useState<LIPPackageDetail | null>(null);
  const [pkgLoading, setPkgLoading] = useState(false);
  const [pkgError, setPkgError] = useState("");
  const [instanceName, setInstanceName] = useState("");
  const [version, setVersion] = useState("");
  const [versionTouched, setVersionTouched] = useState(false);
  const [llVersion, setLLVersion] = useState("");
  const [states, setStates] = useState<Record<string, InstanceStates>>({});
  const [logos, setLogos] = useState<Record<string, string>>({});
  const [phase, setPhase] = useState<Phase>("idle");
  const [steps, setSteps] = useState<InstallStep[]>([]);

  const pkg = providedPkg ?? loadedPkg;
  const running = phase === "running";
  const finished = phase === "success";

  const resolveErrorText = useCallback(
    (value: unknown) => {
      const code = getErrorCode(value);
      if (!code) return t("common.error");
      if (code === "ERR_LIP_INSTALL_FAILED") {
        return t("mods.err_lip_install_failed_suggestion");
      }
      for (const key of [`errors.${code}`, code]) {
        const translated = t(key);
        if (translated !== key) return translated;
      }
      return code;
    },
    [t],
  );

  // Fresh form each time the dialog opens; the finished run stays visible
  // until the user closes it.
  useEffect(() => {
    if (!isOpen) return;
    setPhase("idle");
    setSteps([]);
    setVersion(String(initialVersion || "").trim());
    setVersionTouched(Boolean(String(initialVersion || "").trim()));
    setLLVersion("");
    setStates({});
    setInstanceName("");
  }, [isOpen, initialVersion, identifier]);

  useEffect(() => {
    if (!isOpen || providedPkg || !identifier) return;
    let cancelled = false;
    setPkgLoading(true);
    setPkgError("");
    setLoadedPkg(null);
    fetchLIPPackageDetail(identifier)
      .then((detail) => {
        if (cancelled) return;
        setLoadedPkg(detail);
        if (!detail) setPkgError(t("common.load_failed"));
      })
      .catch(() => {
        if (!cancelled) setPkgError(t("common.load_failed"));
      })
      .finally(() => {
        if (!cancelled) setPkgLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [identifier, isOpen, providedPkg, t]);

  // Target defaults to the current instance.
  useEffect(() => {
    if (!isOpen || instanceName || instances.length === 0) return;
    const names = instances.map((item) => item.name);
    const current = String(currentVersionName || "").trim();
    setInstanceName(names.includes(current) ? current : names[0]);
  }, [currentVersionName, instanceName, instances, isOpen]);

  useEffect(() => {
    if (!isOpen || instances.length === 0) return;
    let cancelled = false;
    void Promise.all(
      instances.map(
        async (item) => [item.name, await loadLogo(item.name)] as const,
      ),
    ).then((entries) => {
      if (!cancelled) setLogos(Object.fromEntries(entries));
    });
    return () => {
      cancelled = true;
    };
  }, [instances, isOpen]);

  const variant = useMemo(
    () => providedVariant ?? resolveDefaultLIPVariant(pkg),
    [pkg, providedVariant],
  );
  const packageIdentifier = String(
    variant?.packageIdentifier || pkg?.identifier || identifier || "",
  ).trim();
  const packageName = pkg?.name || identifier;

  const files = useMemo<LIPPackageFileInfo[]>(
    () =>
      [...(variant?.files || pkg?.files || [])].sort((a, b) =>
        compareVersions(b.version, a.version),
      ),
    [pkg?.files, variant?.files],
  );

  const instance = instances.find((item) => item.name === instanceName);
  const rawGameVersion = String(instance?.gameVersion || "").trim();
  const gameVersion = normalizeGameVersion(rawGameVersion);

  const supportsGame = useCallback(
    (file: LIPPackageFileInfo) =>
      file.llDependencyRanges.length === 0 ||
      (mappingAvailable &&
        Boolean(gameVersion) &&
        resolveSupportedGameVersionsByLLRanges(
          file.llDependencyRanges,
          gameToLLVersions,
        ).includes(gameVersion)),
    [gameToLLVersions, gameVersion, mappingAvailable],
  );

  // Without an explicit pick, follow the newest version the target supports.
  useEffect(() => {
    if (!isOpen || files.length === 0) return;
    // initialVersion is checked too: on open, versionTouched is set in the
    // same commit and not visible here yet.
    if (versionTouched || String(initialVersion || "").trim()) return;
    const best = files.find(supportsGame) || files[0];
    setVersion(best.version);
  }, [files, initialVersion, isOpen, supportsGame, versionTouched]);

  const file = files.find((item) => item.version === version) || null;
  const requiresLL = Boolean(file && file.llDependencyRanges.length > 0);
  const gameCompatible = Boolean(file && supportsGame(file));

  const queryStates = useCallback(
    async (name: string, pkgId: string): Promise<InstanceStates> => {
      try {
        const raw = await GetLIPPackageInstallStates(name, [
          LEVILAMINA_IDENTIFIER,
          pkgId,
        ]);
        const byKey = new Map<string, LIPPackageInstallState>();
        for (const item of Array.isArray(raw) ? raw : []) {
          const key = normalizeIdentifierLookupKey(
            String((item as any)?.identifierKey ?? ""),
          );
          if (key && !byKey.has(key)) {
            byKey.set(key, parseLIPPackageInstallState((item as any)?.state));
          }
        }
        const pick = (id: string): LoadedState => ({
          ...(byKey.get(normalizeIdentifierLookupKey(id)) ?? {
            ...LOADING_STATE,
            error: "ERR_LIP_PACKAGE_QUERY_FAILED",
          }),
          loading: false,
        });
        return { ll: pick(LEVILAMINA_IDENTIFIER), pkg: pick(pkgId) };
      } catch (error) {
        const failed: LoadedState = {
          ...LOADING_STATE,
          loading: false,
          error: getErrorCode(error) || "ERR_LIP_PACKAGE_QUERY_FAILED",
        };
        return { ll: failed, pkg: failed };
      }
    },
    [],
  );

  // Asked from lip directly rather than the mod snapshot, which can still be
  // half-hydrated and would make an installed LeviLamina look missing.
  const reloadStates = useCallback(
    async (name: string, pkgId: string) => {
      setStates((prev) => ({
        ...prev,
        [name]: { ll: LOADING_STATE, pkg: LOADING_STATE },
      }));
      const next = await queryStates(name, pkgId);
      setStates((prev) => ({ ...prev, [name]: next }));
    },
    [queryStates],
  );

  useEffect(() => {
    if (!isOpen || !instanceName || !packageIdentifier) return;
    void reloadStates(instanceName, packageIdentifier);
  }, [instanceName, isOpen, packageIdentifier, reloadStates]);

  const instanceStates = instanceName ? states[instanceName] : undefined;
  const llState = instanceStates?.ll;
  const llLoading = requiresLL && (!llState || llState.loading);
  const installedLLVersion = String(llState?.installedVersion || "").trim();
  const llInstalled = Boolean(llState?.installed);
  const llCompatible =
    llInstalled &&
    Boolean(installedLLVersion) &&
    Boolean(file) &&
    isLeviLaminaVersionCompatible(installedLLVersion, file!.llDependencyRanges);
  // A failed check is not proof that LeviLamina is missing.
  const llCheckFailed =
    requiresLL && !llLoading && !llInstalled && Boolean(llState?.error);
  const needsLL = requiresLL && !llLoading && !llCheckFailed && !llCompatible;
  // Replacing a LeviLamina that is there can break other mods on the instance.
  const replacingLL = needsLL && llInstalled;
  const llDowngrade =
    replacingLL &&
    Boolean(installedLLVersion) &&
    compareVersions(llVersion, installedLLVersion) < 0;

  const llOptions = useMemo(() => {
    if (!file || !gameVersion || !mappingAvailable) return [];
    return getSupportedLLVersions(gameVersion).filter((candidate) =>
      isLeviLaminaVersionCompatible(candidate, file.llDependencyRanges),
    );
  }, [file, gameVersion, getSupportedLLVersions, mappingAvailable]);

  // Pre-select the newest stable LeviLamina that fits; the list is newest first.
  useEffect(() => {
    if (llOptions.includes(llVersion)) return;
    setLLVersion(
      llOptions.find((candidate) => !/rc/i.test(candidate)) ||
        llOptions[0] ||
        "",
    );
  }, [llOptions, llVersion]);

  const packageState = instanceStates?.pkg;
  const packageStateLoading = !packageState || packageState.loading;
  const installedPackageVersion = packageState?.installed
    ? String(packageState.installedVersion || "").trim()
    : "";

  const actionKind = useMemo<ActionKind>(() => {
    if (!version || packageStateLoading || !packageState?.installed) {
      return "install";
    }
    if (!installedPackageVersion) return "reinstall";
    const diff = compareVersions(version, installedPackageVersion);
    return diff > 0 ? "upgrade" : diff < 0 ? "downgrade" : "reinstall";
  }, [
    installedPackageVersion,
    packageState?.installed,
    packageStateLoading,
    version,
  ]);
  const actionLabel = t(ACTION_LABEL_KEYS[actionKind]);

  const mappingPending = requiresLL && (!mappingInitialized || mappingLoading);
  const blockedHint = (() => {
    if (!file) return "";
    if (requiresLL && mappingInitialized && !mappingAvailable) {
      return t("lip.files.mapping_unavailable_blocked_hint");
    }
    if (requiresLL && !mappingPending && !gameCompatible) {
      return t("lip.files.select_version_incompatible_hint", {
        version,
        gameVersion: gameVersion || t("contentpage.none"),
      });
    }
    if (llCheckFailed) {
      return t("lip_install.ll_check_failed", {
        error: resolveErrorText(llState?.error),
      });
    }
    if (needsLL && llOptions.length === 0) {
      return t("lip_install.ll_no_compatible", {
        gameVersion: gameVersion || t("contentpage.none"),
      });
    }
    return "";
  })();

  const canConfirm =
    !running &&
    Boolean(instanceName && file && packageIdentifier) &&
    !packageStateLoading &&
    !llLoading &&
    !mappingPending &&
    !blockedHint &&
    (!needsLL || Boolean(llVersion));

  const updateStep = (id: string, patch: Partial<InstallStep>) =>
    setSteps((prev) =>
      prev.map((step) => (step.id === id ? { ...step, ...patch } : step)),
    );

  const handleConfirm = async () => {
    if (finished) {
      onClose();
      return;
    }
    if (!canConfirm || !file) return;

    const target = instanceName;
    const installLLVersion = needsLL ? llVersion : "";
    const runKey = Date.now();
    const llStep: InstallStep | null = installLLVersion
      ? {
          id: `${runKey}-ll`,
          label: t("lip_install.step_ll", { version: installLLVersion }),
          status: "pending",
        }
      : null;
    const pkgStep: InstallStep = {
      id: `${runKey}-pkg`,
      label: t("lip_install.step_package", {
        action: actionLabel,
        name: packageName,
        version: file.version,
      }),
      status: "pending",
    };
    // Earlier attempts stay listed above the new one.
    setSteps((prev) => [...prev, ...(llStep ? [llStep] : []), pkgStep]);
    setPhase("running");

    const activityId = startActivityTask({
      title: t("lip_install.activity_title", {
        name: packageName,
        instance: target,
      }),
      source: "lip",
    });
    let activeStep: InstallStep = llStep || pkgStep;
    const total = llStep ? 2 : 1;

    try {
      await runWithLipTask(
        {
          action: actionKind,
          target,
          methods: ["Install", "Update"],
          feedbackMode: "inline",
        },
        async ({ addLog, setOuter }) => {
          if (llStep) {
            activeStep = llStep;
            updateStep(llStep.id, { status: "running" });
            appendTaskLog(activityId, llStep.label);
            addLog("info", llStep.label);
            setOuter(0, total, `LeviLamina ${installLLVersion}`);
            const err = await InstallLeviLamina(
              rawGameVersion,
              target,
              installLLVersion,
            );
            if (String(err || "").trim()) throw new Error(String(err));
            updateStep(llStep.id, { status: "success" });
            appendTaskLog(activityId, llStep.label, "success");
          }

          activeStep = pkgStep;
          updateStep(pkgStep.id, { status: "running" });
          appendTaskLog(activityId, pkgStep.label);
          addLog("info", `${actionLabel} ${packageIdentifier}@${file.version}`);
          if (llStep) setOuter(1, total, `${packageName} ${file.version}`);
          const err = await InstallLIPPackage(
            target,
            packageIdentifier,
            file.version,
          );
          if (String(err || "").trim()) throw new Error(String(err));
          updateStep(pkgStep.id, { status: "success" });
          appendTaskLog(activityId, pkgStep.label, "success");
        },
      );

      const summary = t("lip_install.success_summary", {
        name: packageName,
        version: file.version,
        instance: target,
      });
      finishActivityTask(activityId, "success", summary);
      setPhase("success");
      void reloadStates(target, packageIdentifier);
      onInstalled?.(target);
    } catch (error) {
      const message = resolveErrorText(error);
      const failedId = activeStep.id;
      setSteps((prev) =>
        prev.map((step) =>
          step.id === failedId
            ? { ...step, status: "error", message }
            : step.id === pkgStep.id && step.status === "pending"
              ? { ...step, status: "skipped" }
              : step,
        ),
      );
      appendTaskLog(activityId, `${activeStep.label}: ${message}`, "error");
      finishActivityTask(activityId, "error", message);
      notify({
        title: t("lip_install.failed_title", { name: packageName }),
        description: t("lip.files.action_failed", { error: message }),
        variant: "danger",
        source: "lip",
      });
      setPhase("error");
      // Retry stays disabled until this shows whether LeviLamina did install.
      void reloadStates(target, packageIdentifier);
    }
  };

  const close = () => {
    if (running) return;
    onClose();
  };

  const confirmText = finished
    ? t("common.done")
    : phase === "error"
      ? t("common.retry")
      : needsLL
        ? t("lip_install.confirm_with_ll", { action: actionLabel })
        : actionLabel;

  const formLocked = running || finished;
  const noInstances = !instancesLoading && instances.length === 0;

  return (
    <UnifiedModal
      size="wide"
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) close();
      }}
      type="primary"
      icon={<LuDownload size={24} />}
      title={t("lip_install.title", { name: packageName })}
      isDismissable={!running}
      isPending={running}
      showCancelButton={!finished}
      cancelButtonProps={{ isDisabled: running }}
      onCancel={close}
      onConfirm={() => void handleConfirm()}
      confirmText={confirmText}
      confirmButtonProps={{
        isDisabled: !finished && !canConfirm,
      }}
    >
      <div className="flex min-w-0 flex-col gap-4">
        {pkgLoading ? (
          <div className="flex items-center gap-2 text-sm text-muted">
            <Spinner size="sm" />
            {t("common.loading")}
          </div>
        ) : null}
        {pkgError ? <ModalNotice tone="danger">{pkgError}</ModalNotice> : null}
        {noInstances ? (
          <ModalNotice tone="warning">
            {t("instance_picker.none_installed")}
          </ModalNotice>
        ) : null}

        <div className="grid min-w-0 grid-cols-1 gap-4 sm:grid-cols-2">
          <Select
            className="min-w-0"
            placeholder={t("instance_picker.placeholder")}
            isDisabled={formLocked || instances.length === 0}
            value={instanceName || null}
            onChange={(key) => {
              const next = String(key ?? "");
              if (next) setInstanceName(next);
            }}
          >
            <Label>{t("lip_install.target_label")}</Label>
            <Select.Trigger
              className={cn(COMPONENT_STYLES.select.trigger, "min-h-8 text-sm")}
            >
              <Select.Value />
              <Select.Indicator />
            </Select.Trigger>
            <Select.Popover className={COMPONENT_STYLES.select.popoverContent}>
              <ListBox className={COMPONENT_STYLES.select.listbox}>
                {instances.map((item) => (
                  <ListBox.Item
                    key={item.name}
                    id={item.name}
                    textValue={item.name}
                  >
                    <div className="flex min-w-0 items-center gap-2">
                      <div className="flex size-8 shrink-0 items-center justify-center overflow-hidden rounded bg-surface-tertiary">
                        <img
                          src={logos[item.name] || FALLBACK_LOGO}
                          alt=""
                          className="size-full object-cover"
                          onError={(e) =>
                            (e.currentTarget.style.display = "none")
                          }
                        />
                      </div>
                      <div className="flex min-w-0 flex-col">
                        <span className="truncate text-sm">{item.name}</span>
                        <span className="truncate text-xs text-muted">
                          {normalizeGameVersion(item.gameVersion) ||
                            t("contentpage.none")}
                        </span>
                      </div>
                    </div>
                    <ListBox.ItemIndicator />
                  </ListBox.Item>
                ))}
              </ListBox>
            </Select.Popover>
          </Select>

          <Select
            className="min-w-0"
            placeholder={t("lip_install.version_label")}
            isDisabled={formLocked || files.length === 0}
            value={version || null}
            onChange={(key) => {
              const next = String(key ?? "");
              if (!next) return;
              setVersion(next);
              setVersionTouched(true);
            }}
          >
            <Label>{t("lip_install.version_label")}</Label>
            <Select.Trigger
              className={cn(COMPONENT_STYLES.select.trigger, "min-h-8 text-sm")}
            >
              <Select.Value />
              <Select.Indicator />
            </Select.Trigger>
            <Select.Popover className={COMPONENT_STYLES.select.popoverContent}>
              <ListBox className={COMPONENT_STYLES.select.listbox}>
                {files.map((item) => (
                  <ListBox.Item
                    key={item.version}
                    id={item.version}
                    textValue={item.version}
                  >
                    <div className="flex min-w-0 flex-col">
                      <span className="font-mono text-sm">{item.version}</span>
                      <span className="truncate text-xs text-muted">
                        {item.llDependencyRanges.length
                          ? `LeviLamina ${item.llDependencyRanges.join(", ")}`
                          : t("lip.files.game_versions_unrestricted")}
                      </span>
                    </div>
                    <ListBox.ItemIndicator />
                  </ListBox.Item>
                ))}
              </ListBox>
            </Select.Popover>
          </Select>
        </div>

        <ModalDetails
          items={[
            {
              label: t("lip.files.variant_label"),
              value: getLIPVariantDisplayLabel(
                variant,
                t("lip.files.variant_default"),
              ),
            },
            {
              label: t("lip.files.current_installed_version_label"),
              value:
                instanceName && packageStateLoading
                  ? t("lip.files.checking_current_installed_version")
                  : installedPackageVersion || t("contentpage.none"),
            },
            {
              label: t("lip.files.instance_game_version_label"),
              value: gameVersion || t("contentpage.none"),
              mono: true,
            },
            {
              label: "LeviLamina",
              value: !requiresLL
                ? t("lip.files.game_versions_unrestricted")
                : llLoading
                  ? t("lip.files.checking_ll_state")
                  : installedLLVersion || t("contentpage.none"),
              mono: requiresLL && !llLoading && Boolean(installedLLVersion),
            },
          ]}
        />

        {needsLL && llOptions.length > 0 && !formLocked ? (
          <ModalNotice
            tone={replacingLL ? "warning" : "info"}
            data-testid="lip-install-ll-notice"
          >
            <div className="flex min-w-0 flex-col gap-3">
              <span>
                {llInstalled
                  ? t("lip_install.ll_will_replace", {
                      installedVersion: installedLLVersion || "?",
                      version: llVersion,
                    })
                  : t("lip_install.ll_will_install", { version: llVersion })}
                {replacingLL ? (
                  <>
                    {" "}
                    {llDowngrade
                      ? `${t("lip_install.ll_downgrade_note")} `
                      : null}
                    {t("lip_install.ll_replace_warning", {
                      installedVersion: installedLLVersion,
                    })}
                  </>
                ) : null}
              </span>
              {llOptions.length > 1 ? (
                <Select
                  className="min-w-0 max-w-full sm:max-w-xs"
                  value={llVersion || null}
                  onChange={(key) => setLLVersion(String(key ?? ""))}
                >
                  <Label>{t("lip_install.ll_version_label")}</Label>
                  <Select.Trigger
                    className={cn(
                      COMPONENT_STYLES.select.trigger,
                      "min-h-8 text-sm",
                    )}
                  >
                    <Select.Value />
                    <Select.Indicator />
                  </Select.Trigger>
                  <Select.Popover
                    className={COMPONENT_STYLES.select.popoverContent}
                  >
                    <ListBox className={COMPONENT_STYLES.select.listbox}>
                      {llOptions.map((option) => (
                        <ListBox.Item
                          key={option}
                          id={option}
                          textValue={option}
                        >
                          <span className="font-mono text-sm">{option}</span>
                          <ListBox.ItemIndicator />
                        </ListBox.Item>
                      ))}
                    </ListBox>
                  </Select.Popover>
                </Select>
              ) : null}
              {/rc/i.test(llVersion) ? (
                <span className="text-amber-700 dark:text-amber-300">
                  {t("mods.rc_warning.body_2")}
                </span>
              ) : null}
            </div>
          </ModalNotice>
        ) : null}

        {blockedHint && !formLocked ? (
          <ModalNotice tone="warning">{blockedHint}</ModalNotice>
        ) : null}

        {steps.length > 0 ? (
          <ol
            className="flex min-w-0 flex-col gap-2"
            aria-label={t("lip_install.steps_label")}
            aria-live="polite"
          >
            {steps.map((step) => (
              <li
                key={step.id}
                data-status={step.status}
                className="flex min-w-0 items-start gap-2 text-sm"
              >
                <StepIcon status={step.status} />
                <div className="min-w-0 flex-1 [overflow-wrap:anywhere]">
                  <span
                    className={cn(
                      step.status === "skipped" && "text-muted",
                      step.status === "error" && "text-danger",
                    )}
                  >
                    {step.label}
                  </span>
                  <span className="text-muted">
                    {" · "}
                    {t(`lip_install.status_${step.status}`)}
                  </span>
                  {step.message ? (
                    <p className="mt-1 text-xs text-danger">{step.message}</p>
                  ) : null}
                </div>
              </li>
            ))}
          </ol>
        ) : null}

        {finished ? (
          <ModalDescription role="status">
            {t("lip_install.success_summary", {
              name: packageName,
              version,
              instance: instanceName,
            })}
          </ModalDescription>
        ) : null}

        {running ? <LipTaskProgress /> : null}
      </div>
    </UnifiedModal>
  );
};

const StepIcon: React.FC<{ status: StepStatus }> = ({ status }) => {
  const className = "mt-0.5 size-4 shrink-0";
  if (status === "running") return <Spinner size="sm" className={className} />;
  if (status === "success") {
    return (
      <LuCircleCheck
        aria-hidden="true"
        className={cn(className, "text-success")}
      />
    );
  }
  if (status === "error") {
    return (
      <LuCircleX aria-hidden="true" className={cn(className, "text-danger")} />
    );
  }
  if (status === "skipped") {
    return (
      <LuCircleMinus
        aria-hidden="true"
        className={cn(className, "text-muted")}
      />
    );
  }
  return (
    <LuCircleDashed
      aria-hidden="true"
      className={cn(className, "text-muted")}
    />
  );
};
