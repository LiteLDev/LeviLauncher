import React from "react";
import { useNavigate, useLocation } from "react-router-dom";
import { useDisclosure, addToast } from "@heroui/react";
import { Call, Dialogs, Events } from "@wailsio/runtime";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { GetMods } from "bindings/github.com/liteldev/LeviLauncher/modsservice";
import {
  DeleteVersionFolder,
  GetVersionLogoDataUrl,
  GetVersionMeta,
  RenameVersionFolder,
  SaveVersionLogoDataUrl,
  SaveVersionMeta,
  ValidateVersionFolderName,
} from "bindings/github.com/liteldev/LeviLauncher/versionservice";
import { useLeviLamina } from "@/utils/LeviLaminaContext";
import {
  clearCurrentVersionName,
  readCurrentVersionName,
} from "@/utils/currentVersion";
import { useModIntelligence } from "@/utils/ModIntelligenceContext";
import { useLipTaskConsole } from "@/utils/LipTaskConsoleContext";
import { useTranslation } from "react-i18next";
import { ROUTES } from "@/constants/routes";
import {
  EXPERIMENTAL_FEATURES_EVENT_NAME,
  readExperimentalInstanceBackupEnabled,
} from "@/utils/experimentalFeatures";

type InstanceBackupScopeMode = {
  key: string;
  path: string;
  size: number;
  selectable: boolean;
  warning: string;
};

type InstanceBackupScope = {
  key: string;
  label: string;
  path: string;
  size: number;
  selectable: boolean;
  exists: boolean;
  shared: boolean;
  modes: InstanceBackupScopeMode[];
  defaultMode: string;
  warnings: string[];
};

type InstanceBackupInfo = {
  name: string;
  backupDir: string;
  scopes: InstanceBackupScope[];
  errorCode: string;
};

type InstanceBackupModsLIPPackage = {
  identifier: string;
  version: string;
  explicitInstalled: boolean;
  folders: string[];
};

type InstanceBackupRequest = {
  scopes: string[];
  scopeModes: Record<string, string>;
  modsLipPackages: InstanceBackupModsLIPPackage[];
};

type InstanceBackupResult = {
  archivePath: string;
  backupDir: string;
  includedScopes: string[];
  errorCode: string;
};

type InstanceBackupArchiveScope = {
  key: string;
  label: string;
  mode: string;
  warnings: string[];
};

type InstanceBackupArchiveInfo = {
  formatVersion: number;
  name: string;
  archivePath: string;
  archiveName: string;
  gameVersion: string;
  type: string;
  enableIsolation: boolean;
  createdAt: string;
  includedScopes: string[];
  scopeModes: Record<string, string>;
  scopes: InstanceBackupArchiveScope[];
  modsLipPackages: InstanceBackupModsLIPPackage[];
  rawModFolders: string[];
  bedrockWhitelistRoots: string[];
  errorCode: string;
};

type InstanceBackupRestoreRequest = {
  archivePath: string;
  scopes: string[];
  conflictResolutions: InstanceBackupRestoreResolution[];
};

type InstanceBackupRestoreResolution = {
  conflictId: string;
  choice: "backup" | "current";
};

type InstanceBackupRestoreConflict = {
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
  diffFields: InstanceBackupRestoreConflictDiffField[];
};

type InstanceBackupRestoreConflictDiffField = {
  key: string;
  label: string;
  backupValue: string;
  currentValue: string;
};

type InstanceBackupRestoreConflictInfo = {
  archivePath: string;
  includedScopes: string[];
  conflicts: InstanceBackupRestoreConflict[];
  errorCode: string;
};

type InstanceBackupRestoreScopeResult = {
  key: string;
  label: string;
  mode: string;
  status: "success" | "partial" | "failed";
  errorCode: string;
  warnings: string[];
  details: string[];
};

type InstanceBackupRestoreResult = {
  archivePath: string;
  status: "success" | "partial" | "failed";
  includedScopes: string[];
  scopeResults: InstanceBackupRestoreScopeResult[];
  errorCode: string;
};

type InstanceBackupRestoreProgress = {
  phase: string;
  currentStep: number;
  totalSteps: number;
  scopeKey: string;
  scopeLabel: string;
  ts: number;
};

const VERSION_SERVICE_NAME = "main.VersionService";
const LEVILAMINA_BACKUP_IDENTIFIER = "LiteLDev/LeviLamina";
const LEVILAMINA_MOD_NAME = "levilamina";
const EVENT_INSTANCE_BACKUP_RESTORE_PROGRESS =
  "instance_backup.restore.progress";

const asRecord = (value: unknown): Record<string, unknown> =>
  value && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};

const toStringArray = (value: unknown): string[] =>
  Array.isArray(value)
    ? value.map((item) => String(item ?? "").trim()).filter(Boolean)
    : [];

const isDialogCancelledError = (value: unknown): boolean => {
  const raw =
    typeof value === "string"
      ? value
      : value instanceof Error
        ? value.message
        : String(value || "");
  const normalized = raw.trim().toLowerCase();
  if (!normalized) return false;
  return (
    normalized.includes("cancelled by user") ||
    normalized.includes("canceled by user") ||
    normalized.includes("cancelled") ||
    normalized.includes("canceled")
  );
};

const toBackupScopeMode = (value: unknown): InstanceBackupScopeMode => {
  const record = asRecord(value);
  return {
    key: String(record.key ?? "").trim(),
    path: String(record.path ?? "").trim(),
    size: Number(record.size ?? 0) || 0,
    selectable: Boolean(record.selectable),
    warning: String(record.warning ?? "").trim(),
  };
};

const toBackupScope = (value: unknown): InstanceBackupScope => {
  const record = asRecord(value);
  return {
    key: String(record.key ?? "").trim(),
    label: String(record.label ?? "").trim(),
    path: String(record.path ?? "").trim(),
    size: Number(record.size ?? 0) || 0,
    selectable: Boolean(record.selectable),
    exists: Boolean(record.exists),
    shared: Boolean(record.shared),
    modes: Array.isArray(record.modes)
      ? record.modes.map(toBackupScopeMode).filter((item) => item.key)
      : [],
    defaultMode: String(record.defaultMode ?? "").trim(),
    warnings: toStringArray(record.warnings),
  };
};

const toInstanceBackupInfo = (value: unknown): InstanceBackupInfo => {
  const record = asRecord(value);
  return {
    name: String(record.name ?? "").trim(),
    backupDir: String(record.backupDir ?? "").trim(),
    scopes: Array.isArray(record.scopes)
      ? record.scopes.map(toBackupScope).filter((item) => item.key)
      : [],
    errorCode: String(record.errorCode ?? "").trim(),
  };
};

const toInstanceBackupModsLIPPackage = (
  value: unknown,
): InstanceBackupModsLIPPackage => {
  const record = asRecord(value);
  return {
    identifier: String(record.identifier ?? "").trim(),
    version: String(record.version ?? "").trim(),
    explicitInstalled: Boolean(record.explicitInstalled),
    folders: toStringArray(record.folders),
  };
};

const toInstanceBackupResult = (value: unknown): InstanceBackupResult => {
  const record = asRecord(value);
  return {
    archivePath: String(record.archivePath ?? "").trim(),
    backupDir: String(record.backupDir ?? "").trim(),
    includedScopes: toStringArray(record.includedScopes),
    errorCode: String(record.errorCode ?? "").trim(),
  };
};

const toInstanceBackupArchiveScope = (
  value: unknown,
): InstanceBackupArchiveScope => {
  const record = asRecord(value);
  return {
    key: String(record.key ?? "").trim(),
    label: String(record.label ?? "").trim(),
    mode: String(record.mode ?? "").trim(),
    warnings: toStringArray(record.warnings),
  };
};

const toStringMap = (value: unknown): Record<string, string> => {
  const record = asRecord(value);
  const output: Record<string, string> = {};
  Object.entries(record).forEach(([key, rawValue]) => {
    const normalizedKey = String(key || "").trim();
    const normalizedValue = String(rawValue ?? "").trim();
    if (!normalizedKey || !normalizedValue) return;
    output[normalizedKey] = normalizedValue;
  });
  return output;
};

const toInstanceBackupArchiveInfo = (
  value: unknown,
): InstanceBackupArchiveInfo => {
  const record = asRecord(value);
  return {
    formatVersion: Number(record.formatVersion ?? 0) || 0,
    name: String(record.name ?? "").trim(),
    archivePath: String(record.archivePath ?? "").trim(),
    archiveName: String(record.archiveName ?? "").trim(),
    gameVersion: String(record.gameVersion ?? "").trim(),
    type: String(record.type ?? "").trim(),
    enableIsolation: Boolean(record.enableIsolation),
    createdAt: String(record.createdAt ?? "").trim(),
    includedScopes: toStringArray(record.includedScopes),
    scopeModes: toStringMap(record.scopeModes),
    scopes: Array.isArray(record.scopes)
      ? record.scopes
          .map(toInstanceBackupArchiveScope)
          .filter((item) => item.key)
      : [],
    modsLipPackages: Array.isArray(record.modsLipPackages)
      ? record.modsLipPackages
          .map(toInstanceBackupModsLIPPackage)
          .filter((item) => item.identifier)
      : [],
    rawModFolders: toStringArray(record.rawModFolders),
    bedrockWhitelistRoots: toStringArray(record.bedrockWhitelistRoots),
    errorCode: String(record.errorCode ?? "").trim(),
  };
};

const toInstanceBackupRestoreScopeResult = (
  value: unknown,
): InstanceBackupRestoreScopeResult => {
  const record = asRecord(value);
  const rawStatus = String(record.status ?? "").trim();
  const status: InstanceBackupRestoreScopeResult["status"] =
    rawStatus === "success" || rawStatus === "partial" || rawStatus === "failed"
      ? rawStatus
      : "failed";
  return {
    key: String(record.key ?? "").trim(),
    label: String(record.label ?? "").trim(),
    mode: String(record.mode ?? "").trim(),
    status,
    errorCode: String(record.errorCode ?? "").trim(),
    warnings: toStringArray(record.warnings),
    details: toStringArray(record.details),
  };
};

const toInstanceBackupRestoreResult = (
  value: unknown,
): InstanceBackupRestoreResult => {
  const record = asRecord(value);
  const rawStatus = String(record.status ?? "").trim();
  const status: InstanceBackupRestoreResult["status"] =
    rawStatus === "success" || rawStatus === "partial" || rawStatus === "failed"
      ? rawStatus
      : "failed";
  return {
    archivePath: String(record.archivePath ?? "").trim(),
    status,
    includedScopes: toStringArray(record.includedScopes),
    scopeResults: Array.isArray(record.scopeResults)
      ? record.scopeResults
          .map(toInstanceBackupRestoreScopeResult)
          .filter((item) => item.key)
      : [],
    errorCode: String(record.errorCode ?? "").trim(),
  };
};

const toInstanceBackupRestoreProgress = (
  value: unknown,
): InstanceBackupRestoreProgress => {
  const record = asRecord(value);
  return {
    phase: String(record.phase ?? "").trim(),
    currentStep: Number(record.currentStep ?? 0) || 0,
    totalSteps: Number(record.totalSteps ?? 0) || 0,
    scopeKey: String(record.scopeKey ?? "").trim(),
    scopeLabel: String(record.scopeLabel ?? "").trim(),
    ts: Number(record.ts ?? 0) || 0,
  };
};

const toInstanceBackupRestoreConflict = (
  value: unknown,
): InstanceBackupRestoreConflict => {
  const record = asRecord(value);
  const sourceType =
    String(record.sourceType ?? "").trim() === "dir" ? "dir" : "file";
  const targetType =
    String(record.targetType ?? "").trim() === "dir" ? "dir" : "file";
  const identityKindRaw = String(record.identityKind ?? "").trim();
  const identityKind: InstanceBackupRestoreConflict["identityKind"] =
    identityKindRaw === "pack_uuid" ||
    identityKindRaw === "world_folder" ||
    identityKindRaw === "mod_folder" ||
    identityKindRaw === "file_path"
      ? identityKindRaw
      : "file_path";
  return {
    id: String(record.id ?? "").trim(),
    scopeKey: String(record.scopeKey ?? "").trim(),
    scopeLabel: String(record.scopeLabel ?? "").trim(),
    path: String(record.path ?? "").trim(),
    sourceType,
    targetType,
    identityKind,
    identityKey: String(record.identityKey ?? "").trim(),
    backupPath: String(record.backupPath ?? "").trim(),
    currentPath: String(record.currentPath ?? "").trim(),
    backupSummary: String(record.backupSummary ?? "").trim(),
    currentSummary: String(record.currentSummary ?? "").trim(),
    diffFields: Array.isArray(record.diffFields)
      ? record.diffFields
          .map((item) => {
            const fieldRecord = asRecord(item);
            return {
              key: String(fieldRecord.key ?? "").trim(),
              label: String(fieldRecord.label ?? "").trim(),
              backupValue: String(fieldRecord.backupValue ?? "").trim(),
              currentValue: String(fieldRecord.currentValue ?? "").trim(),
            } satisfies InstanceBackupRestoreConflictDiffField;
          })
          .filter((item) => item.key)
      : [],
  };
};

const toInstanceBackupRestoreConflictInfo = (
  value: unknown,
): InstanceBackupRestoreConflictInfo => {
  const record = asRecord(value);
  return {
    archivePath: String(record.archivePath ?? "").trim(),
    includedScopes: toStringArray(record.includedScopes),
    conflicts: Array.isArray(record.conflicts)
      ? record.conflicts
          .map(toInstanceBackupRestoreConflict)
          .filter((item) => item.id && item.path)
      : [],
    errorCode: String(record.errorCode ?? "").trim(),
  };
};

export const useInstanceSettings = () => {
  const { t } = useTranslation();
  const { runWithLipTask } = useLipTaskConsole();
  const {
    ensureInstanceHydrated,
    getInstanceSnapshot,
    refreshInstance,
    snapshotRevision,
  } = useModIntelligence();
  const navigate = useNavigate();
  const location = useLocation() as any;
  const initialName: string = String(location?.state?.name || "");
  const initialTab: string = String(location?.state?.tab || "general");
  const returnToPath: string = String(
    location?.state?.returnTo || ROUTES.instances,
  );
  const hasBackend = minecraft !== undefined;
  const ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY =
    "ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY";
  const ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS =
    "ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS";
  const ERR_INSTANCE_BACKUP_MODS_STATE_UNAVAILABLE =
    "ERR_INSTANCE_BACKUP_MODS_STATE_UNAVAILABLE";

  const callVersionService = React.useCallback(
    async <T>(method: string, ...args: unknown[]): Promise<T> => {
      return (await Call.ByName(
        `${VERSION_SERVICE_NAME}.${method}`,
        ...args,
      )) as T;
    },
    [],
  );

  const buildBackupLipPackagesFromSnapshot = React.useCallback(
    (
      snapshot: any,
      allMods: unknown,
    ): InstanceBackupModsLIPPackage[] | null => {
      const groups = Array.isArray(snapshot?.lipGroupItems)
        ? snapshot.lipGroupItems
        : [];
      const installStateMap =
        snapshot?.lipInstallStateByIdentifier instanceof Map
          ? snapshot.lipInstallStateByIdentifier
          : new Map<string, any>();
      const packages: InstanceBackupModsLIPPackage[] = [];
      const seen = new Set<string>();

      for (const group of groups) {
        const identifier = String(group?.identifier || "").trim();
        const identifierKey = String(
          group?.identifierKey || group?.identifier || "",
        ).trim();
        const lookupKey = identifierKey.toLowerCase();
        if (!identifier || !identifierKey || seen.has(lookupKey)) {
          continue;
        }

        const installState = installStateMap.get(identifierKey);
        if (!installState?.installed) {
          continue;
        }

        const folders = Array.from(
          new Set<string>(
            Array.isArray(group?.folders)
              ? group.folders
                  .map((item: unknown) => String(item || "").trim())
                  .filter(Boolean)
              : [],
          ),
        ).sort((a, b) =>
          a.localeCompare(b, undefined, { sensitivity: "base" }),
        );
        if (folders.length === 0) {
          continue;
        }

        let version = String(
          installState?.installedVersion || group?.installedVersion || "",
        ).trim();
        if (!version && Array.isArray(group?.mods)) {
          const versions = Array.from(
            new Set<string>(
              group.mods
                .map((mod: any) => String(mod?.version || "").trim())
                .filter(Boolean),
            ),
          );
          if (versions.length === 1) {
            version = versions[0];
          }
        }
        if (!version) {
          continue;
        }

        seen.add(lookupKey);
        packages.push({
          identifier,
          version,
          explicitInstalled: Boolean(installState?.explicitInstalled),
          folders,
        });
      }

      const leviLaminaState = snapshot?.llState;
      const leviLaminaLookupKey = LEVILAMINA_BACKUP_IDENTIFIER.toLowerCase();
      if (
        Boolean(leviLaminaState?.installed) &&
        !seen.has(leviLaminaLookupKey)
      ) {
        const leviLaminaMods = Array.isArray(allMods)
          ? allMods.filter((item) => {
              const record = asRecord(item);
              return (
                String(record.name ?? "")
                  .trim()
                  .toLowerCase() === LEVILAMINA_MOD_NAME
              );
            })
          : [];
        const folders = Array.from(
          new Set(
            leviLaminaMods
              .map((item) => {
                const record = asRecord(item);
                return String(record.folder ?? record.name ?? "").trim();
              })
              .filter(Boolean),
          ),
        ).sort((a, b) =>
          a.localeCompare(b, undefined, { sensitivity: "base" }),
        );
        const modVersions = Array.from(
          new Set(
            leviLaminaMods
              .map((item) => {
                const record = asRecord(item);
                return String(record.version ?? "").trim();
              })
              .filter(Boolean),
          ),
        );
        const version =
          String(leviLaminaState?.installedVersion || "").trim() ||
          (modVersions.length === 1 ? modVersions[0] : "");
        if (folders.length > 0 && version) {
          seen.add(leviLaminaLookupKey);
          packages.push({
            identifier: LEVILAMINA_BACKUP_IDENTIFIER,
            version,
            explicitInstalled: Boolean(leviLaminaState?.explicitInstalled),
            folders,
          });
        } else {
          console.warn(
            "LeviLamina is installed but could not be mapped into backup metadata",
            {
              folderCount: folders.length,
              version,
            },
          );
          return null;
        }
      }

      return packages.sort((a, b) =>
        a.identifier.localeCompare(b.identifier, undefined, {
          sensitivity: "base",
        }),
      );
    },
    [],
  );

  // Form state
  const [targetName, setTargetName] = React.useState<string>(initialName);
  const [selectedTab, setSelectedTab] = React.useState<string>(initialTab);
  const [newName, setNewName] = React.useState<string>(initialName);
  const [gameVersion, setGameVersion] = React.useState<string>("");
  const [versionType, setVersionType] = React.useState<string>("");
  const [isPreview, setIsPreview] = React.useState<boolean>(false);
  const [enableIsolation, setEnableIsolation] = React.useState<boolean>(false);
  const [enableConsole, setEnableConsole] = React.useState<boolean>(false);
  const [enableEditorMode, setEnableEditorMode] =
    React.useState<boolean>(false);
  const [enableRenderDragon, setEnableRenderDragon] =
    React.useState<boolean>(false);
  const [enableCtrlRReloadResources, setEnableCtrlRReloadResources] =
    React.useState<boolean>(false);
  const [envVars, setEnvVars] = React.useState<string>("");
  const [launchArgs, setLaunchArgs] = React.useState<string>("");
  const [isRegistered, setIsRegistered] = React.useState<boolean>(false);
  const [loading, setLoading] = React.useState<boolean>(true);
  const [error, setError] = React.useState<string>("");
  const [logoDataUrl, setLogoDataUrl] = React.useState<string>("");

  // Original values for unsaved changes detection
  const [originalIsolation, setOriginalIsolation] =
    React.useState<boolean>(false);
  const [originalConsole, setOriginalConsole] = React.useState<boolean>(false);
  const [originalEditorMode, setOriginalEditorMode] =
    React.useState<boolean>(false);
  const [originalRenderDragon, setOriginalRenderDragon] =
    React.useState<boolean>(false);
  const [originalCtrlRReloadResources, setOriginalCtrlRReloadResources] =
    React.useState<boolean>(false);
  const [originalEnvVars, setOriginalEnvVars] = React.useState<string>("");
  const [originalLaunchArgs, setOriginalLaunchArgs] =
    React.useState<string>("");

  // Modal states
  const [unregisterOpen, setUnregisterOpen] = React.useState<boolean>(false);
  const [unregisterSuccessOpen, setUnregisterSuccessOpen] =
    React.useState<boolean>(false);
  const [errorOpen, setErrorOpen] = React.useState<boolean>(false);
  const [deleteOpen, setDeleteOpen] = React.useState<boolean>(false);
  const [deleteSuccessOpen, setDeleteSuccessOpen] =
    React.useState<boolean>(false);
  const [deleting, setDeleting] = React.useState<boolean>(false);
  const [deleteSuccessMsg, setDeleteSuccessMsg] = React.useState<string>("");
  const [backupOpen, setBackupOpen] = React.useState<boolean>(false);
  const [backupInfoLoading, setBackupInfoLoading] =
    React.useState<boolean>(false);
  const [backupInfo, setBackupInfo] = React.useState<InstanceBackupInfo | null>(
    null,
  );
  const [selectedBackupScopes, setSelectedBackupScopes] = React.useState<
    string[]
  >([]);
  const [backupScopeModes, setBackupScopeModes] = React.useState<
    Record<string, string>
  >({});
  const [backupPreparedLipPackages, setBackupPreparedLipPackages] =
    React.useState<InstanceBackupModsLIPPackage[]>([]);
  const [backingUpInstance, setBackingUpInstance] =
    React.useState<boolean>(false);
  const [backupSuccessOpen, setBackupSuccessOpen] =
    React.useState<boolean>(false);
  const [backupResult, setBackupResult] =
    React.useState<InstanceBackupResult | null>(null);
  const [restoreOpen, setRestoreOpen] = React.useState<boolean>(false);
  const [restoreInfoLoading, setRestoreInfoLoading] =
    React.useState<boolean>(false);
  const [restoreArchiveInfo, setRestoreArchiveInfo] =
    React.useState<InstanceBackupArchiveInfo | null>(null);
  const [selectedRestoreScopes, setSelectedRestoreScopes] = React.useState<
    string[]
  >([]);
  const [restoreConflictLoading, setRestoreConflictLoading] =
    React.useState<boolean>(false);
  const [restoreConflictInfo, setRestoreConflictInfo] =
    React.useState<InstanceBackupRestoreConflictInfo | null>(null);
  const [restoreConflictChoices, setRestoreConflictChoices] = React.useState<
    Record<string, "backup" | "current">
  >({});
  const [restoringInstance, setRestoringInstance] =
    React.useState<boolean>(false);
  const [restoreProgress, setRestoreProgress] =
    React.useState<InstanceBackupRestoreProgress | null>(null);
  const [restoreResultOpen, setRestoreResultOpen] =
    React.useState<boolean>(false);
  const [restoreResult, setRestoreResult] =
    React.useState<InstanceBackupRestoreResult | null>(null);
  const [
    instanceBackupExperimentalEnabled,
    setInstanceBackupExperimentalEnabled,
  ] = React.useState<boolean>(() => readExperimentalInstanceBackupEnabled());

  // Unsaved changes modal
  const {
    isOpen: unsavedOpen,
    onOpen: unsavedOnOpen,
    onClose: unsavedOnClose,
    onOpenChange: unsavedOnOpenChange,
  } = useDisclosure();
  const [pendingNavPath, setPendingNavPath] = React.useState<string>("");

  // LeviLamina
  const {
    isLLSupported,
    getSupportedLLVersions,
    getLatestLLVersion,
    compareLLVersions,
  } = useLeviLamina();
  const [installingLL, setInstallingLL] = React.useState(false);
  const [uninstallingLL, setUninstallingLL] = React.useState(false);
  const [demotedWarningNames, setDemotedWarningNames] = React.useState<
    string[]
  >([]);
  const {
    isOpen: llVersionSelectOpen,
    onOpen: llVersionSelectOnOpen,
    onOpenChange: llVersionSelectOnOpenChange,
    onClose: llVersionSelectOnClose,
  } = useDisclosure();
  const {
    isOpen: llInstallConfirmOpen,
    onOpen: llInstallConfirmOnOpen,
    onClose: llInstallConfirmOnClose,
  } = useDisclosure();
  const {
    isOpen: llUninstallConfirmOpen,
    onOpen: llUninstallConfirmOnOpen,
    onClose: llUninstallConfirmOnClose,
  } = useDisclosure();
  const {
    isOpen: rcOpen,
    onOpen: rcOnOpen,
    onOpenChange: rcOnOpenChange,
    onClose: rcOnClose,
  } = useDisclosure();
  const {
    isOpen: demotedWarningOpen,
    onOpen: demotedWarningOnOpen,
    onClose: demotedWarningOnClose,
  } = useDisclosure();
  const [rcVersion, setRcVersion] = React.useState("");
  const [isLLInstalled, setIsLLInstalled] = React.useState(false);
  const [llExplicitInstalled, setLLExplicitInstalled] = React.useState(false);
  const [currentLLVersion, setCurrentLLVersion] = React.useState("");
  const [selectedLLVersion, setSelectedLLVersion] = React.useState("");
  const [lipMissingOpen, setLipMissingOpen] = React.useState(false);
  const [llSupportedVersions, setLLSupportedVersions] = React.useState<
    string[]
  >([]);

  const latestLLVersion = React.useMemo(
    () => llSupportedVersions[0] || "",
    [llSupportedVersions],
  );

  React.useEffect(() => {
    if (initialName) return;
    navigate(ROUTES.instances, { replace: true });
  }, [initialName, navigate]);

  React.useEffect(() => {
    const handleExperimentalFeaturesChange = () => {
      setInstanceBackupExperimentalEnabled(
        readExperimentalInstanceBackupEnabled(),
      );
    };
    window.addEventListener(
      EXPERIMENTAL_FEATURES_EVENT_NAME,
      handleExperimentalFeaturesChange,
    );
    return () =>
      window.removeEventListener(
        EXPERIMENTAL_FEATURES_EVENT_NAME,
        handleExperimentalFeaturesChange,
      );
  }, []);

  React.useEffect(() => {
    if (instanceBackupExperimentalEnabled) return;
    if (!backingUpInstance) {
      setBackupOpen(false);
    }
    if (!restoringInstance) {
      setRestoreOpen(false);
    }
  }, [backingUpInstance, instanceBackupExperimentalEnabled, restoringInstance]);

  const resolvedLLTargetVersion = React.useMemo(
    () =>
      String(selectedLLVersion || getLatestLLVersion(gameVersion) || "").trim(),
    [gameVersion, getLatestLLVersion, selectedLLVersion],
  );
  const llUninstallBlocked = React.useMemo(
    () => Boolean(isLLInstalled) && !Boolean(llExplicitInstalled),
    [isLLInstalled, llExplicitInstalled],
  );

  const isSemverLike = React.useCallback((version: string) => {
    const v = String(version || "").trim();
    return /^[vV]?\d+\.\d+\.\d+(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/.test(
      v,
    );
  }, []);

  const canInstallSelectedLLVersion = React.useMemo(
    () => Boolean(String(selectedLLVersion || "").trim()),
    [selectedLLVersion],
  );

  const llInstallBlockedReasonKey = React.useMemo(() => "", []);

  const llOnlyLatestSelectable = React.useMemo(() => false, []);

  const llInstallActionKind = React.useMemo<
    "install" | "upgrade" | "downgrade" | "reinstall"
  >(() => {
    const selected = String(resolvedLLTargetVersion || "").trim();
    if (!selected) return "install";
    if (!isLLInstalled) return "install";

    const current = String(currentLLVersion || "").trim();
    if (!current) return "reinstall";

    const cmp = compareLLVersions(selected, current);
    if (Number.isNaN(cmp)) return "reinstall";
    if (cmp > 0) return "upgrade";
    if (cmp < 0) return "downgrade";
    return "reinstall";
  }, [
    compareLLVersions,
    currentLLVersion,
    isLLInstalled,
    resolvedLLTargetVersion,
  ]);
  const llInstallActionLabelKey = React.useMemo(
    () => `lip.files.${llInstallActionKind}`,
    [llInstallActionKind],
  );
  const llInstallSuccessKey = React.useMemo(
    () => `lip.files.${llInstallActionKind}_success`,
    [llInstallActionKind],
  );

  // Load LL install status
  React.useEffect(() => {
    if (selectedTab !== "loader" || !targetName) return;
    let cancelled = false;
    const run = async () => {
      try {
        await ensureInstanceHydrated(targetName, {
          background: true,
          reason: "version-settings-loader-state",
        });
        if (cancelled) return;
        const snapshot = getInstanceSnapshot(targetName);
        setIsLLInstalled(Boolean(snapshot?.llState?.installed));
        setLLExplicitInstalled(Boolean(snapshot?.llState?.explicitInstalled));
        setCurrentLLVersion(
          String(snapshot?.llState?.installedVersion || "").trim(),
        );
      } catch {
        if (cancelled) return;
        setIsLLInstalled(false);
        setLLExplicitInstalled(false);
        setCurrentLLVersion("");
      }
    };
    void run();
    return () => {
      cancelled = true;
    };
  }, [
    ensureInstanceHydrated,
    getInstanceSnapshot,
    selectedTab,
    snapshotRevision,
    targetName,
    installingLL,
    uninstallingLL,
  ]);

  React.useEffect(() => {
    setLLSupportedVersions(getSupportedLLVersions(gameVersion));
  }, [gameVersion, getSupportedLLVersions]);

  React.useEffect(() => {
    setSelectedLLVersion((prev) => {
      if (llSupportedVersions.length === 0) {
        return "";
      }
      if (prev && llSupportedVersions.includes(prev)) {
        return prev;
      }
      if (
        isLLInstalled &&
        currentLLVersion &&
        llSupportedVersions.includes(currentLLVersion)
      ) {
        return currentLLVersion;
      }
      return latestLLVersion;
    });
  }, [llSupportedVersions, isLLInstalled, currentLLVersion, latestLLVersion]);

  // Load version metadata
  React.useEffect(() => {
    if (!hasBackend || !targetName) return;
    (async () => {
      try {
        const getMeta = GetVersionMeta as any;
        if (typeof getMeta === "function") {
          const meta: any = await getMeta(targetName);
          if (meta) {
            setGameVersion(String(meta?.gameVersion || ""));
            const type = String(meta?.type || "release").toLowerCase();
            setVersionType(type);
            setIsPreview(type === "preview");
            setEnableIsolation(!!meta?.enableIsolation);
            setOriginalIsolation(!!meta?.enableIsolation);
            setEnableConsole(!!meta?.enableConsole);
            setOriginalConsole(!!meta?.enableConsole);
            setEnableEditorMode(!!meta?.enableEditorMode);
            setOriginalEditorMode(!!meta?.enableEditorMode);
            setEnableRenderDragon(!!meta?.enableRenderDragon);
            setOriginalRenderDragon(!!meta?.enableRenderDragon);
            setEnableCtrlRReloadResources(!!meta?.enableCtrlRReloadResources);
            setOriginalCtrlRReloadResources(!!meta?.enableCtrlRReloadResources);
            setEnvVars(String(meta?.envVars || ""));
            setOriginalEnvVars(String(meta?.envVars || ""));
            setLaunchArgs(String(meta?.launchArgs || ""));
            setOriginalLaunchArgs(String(meta?.launchArgs || ""));
            setIsRegistered(Boolean(meta?.registered));
          }
        }
      } catch {}
      try {
        const getter = GetVersionLogoDataUrl as any;
        if (typeof getter === "function") {
          const u = await getter(targetName);
          setLogoDataUrl(String(u || ""));
        }
      } catch {
        setLogoDataUrl("");
      }
      setLoading(false);
    })();
  }, [hasBackend, targetName]);

  React.useEffect(() => {
    setErrorOpen(!!error);
  }, [error]);

  React.useEffect(() => {
    if (location.state && (location.state as any).tab) {
      setSelectedTab((location.state as any).tab);
    }
  }, [location.state]);

  // Unsaved changes navigation guard
  React.useEffect(() => {
    const handler = (ev: any) => {
      try {
        let targetPath = ev?.detail?.path;
        if (targetPath === -1) targetPath = "-1";
        targetPath = String(targetPath || "");
        const hasUnsaved =
          (newName && newName !== targetName) ||
          enableIsolation !== originalIsolation ||
          enableConsole !== originalConsole ||
          enableEditorMode !== originalEditorMode ||
          enableRenderDragon !== originalRenderDragon ||
          enableCtrlRReloadResources !== originalCtrlRReloadResources ||
          envVars !== originalEnvVars ||
          launchArgs !== originalLaunchArgs;

        if (!targetPath || targetPath === location.pathname) return;
        if (hasUnsaved) {
          setPendingNavPath(targetPath);
          unsavedOnOpen();
          return;
        }
        if (targetPath === "-1") {
          navigate(-1);
        } else {
          navigate(targetPath);
        }
      } catch {}
    };
    window.addEventListener("ll-try-nav", handler as any);
    return () => window.removeEventListener("ll-try-nav", handler as any);
  }, [
    newName,
    targetName,
    enableIsolation,
    originalIsolation,
    enableConsole,
    originalConsole,
    enableEditorMode,
    originalEditorMode,
    enableRenderDragon,
    originalRenderDragon,
    enableCtrlRReloadResources,
    originalCtrlRReloadResources,
    navigate,
    location.pathname,
    unsavedOnOpen,
  ]);

  // Error text helper
  const errorDefaults: Record<string, string> = {
    ERR_INVALID_NAME: "errors.ERR_INVALID_NAME",
    ERR_ICON_DECODE: "errors.ERR_ICON_DECODE",
    ERR_ICON_NOT_SQUARE: "errors.ERR_ICON_NOT_SQUARE",
  };
  const getErrorKey = (code: string) => {
    if (!code) return "";
    return errorDefaults[code] || `errors.${code}`;
  };

  const resolveToastText = React.useCallback(
    (value: string) => {
      const raw = String(value || "").trim();
      if (!raw) return "";

      const direct = t(raw);
      if (direct && direct !== raw) {
        return String(direct);
      }

      if (/^ERR_[A-Z0-9_]+$/.test(raw)) {
        const mapped = getErrorKey(raw);
        const translated = t(mapped);
        if (translated && translated !== mapped) {
          return String(translated);
        }
      }

      const errorsKey = `errors.${raw}`;
      const errorsTranslated = t(errorsKey);
      if (errorsTranslated && errorsTranslated !== errorsKey) {
        return String(errorsTranslated);
      }

      return raw;
    },
    [t],
  );

  const llUninstallWarning = React.useMemo(() => {
    if (!llUninstallBlocked) return "";
    return resolveToastText(ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS);
  }, [
    ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS,
    llUninstallBlocked,
    resolveToastText,
  ]);

  const openDemotedWarning = React.useCallback(
    (names: string[]) => {
      const uniqueNames = Array.from(
        new Set(names.map((item) => String(item || "").trim()).filter(Boolean)),
      );
      if (uniqueNames.length === 0) return;
      setDemotedWarningNames(uniqueNames);
      demotedWarningOnOpen();
    },
    [demotedWarningOnOpen],
  );

  const closeDemotedWarning = React.useCallback(() => {
    setDemotedWarningNames([]);
    demotedWarningOnClose();
  }, [demotedWarningOnClose]);

  const demotedWarningOnOpenChange = React.useCallback(
    (nextOpen: boolean) => {
      if (nextOpen) {
        demotedWarningOnOpen();
        return;
      }
      closeDemotedWarning();
    },
    [closeDemotedWarning, demotedWarningOnOpen],
  );

  const refreshLLStateForUninstall = React.useCallback(async () => {
    if (!targetName) {
      setIsLLInstalled(false);
      setLLExplicitInstalled(false);
      setCurrentLLVersion("");
      return null;
    }

    try {
      await ensureInstanceHydrated(targetName, {
        background: true,
        reason: "version-settings-loader-state",
      });
    } catch {}

    const snapshot = getInstanceSnapshot(targetName);
    setIsLLInstalled(Boolean(snapshot?.llState?.installed));
    setLLExplicitInstalled(Boolean(snapshot?.llState?.explicitInstalled));
    setCurrentLLVersion(
      String(snapshot?.llState?.installedVersion || "").trim(),
    );
    return snapshot?.llState || null;
  }, [ensureInstanceHydrated, getInstanceSnapshot, targetName]);

  const openLipComponentsSettings = React.useCallback(() => {
    setLipMissingOpen(false);
    navigate(ROUTES.settings, { state: { tab: "components" } });
  }, [navigate]);

  const ensureLipRuntimeAvailable = React.useCallback(async () => {
    try {
      const getLipStatus = (minecraft as any)?.GetLipStatus;
      if (typeof getLipStatus === "function") {
        const status = await getLipStatus();
        if (Boolean(status?.installed)) {
          return true;
        }
      } else {
        const isLipInstalled = await (minecraft as any)?.IsLipInstalled?.();
        if (Boolean(isLipInstalled)) {
          return true;
        }
      }
    } catch (error) {
      console.warn("Failed to check lip runtime status", error);
      return true;
    }

    llVersionSelectOnClose();
    llInstallConfirmOnClose();
    rcOnClose();
    setLipMissingOpen(true);
    return false;
  }, [llInstallConfirmOnClose, llVersionSelectOnClose, rcOnClose]);

  const backupGameDataLabel = React.useMemo(
    () => (isPreview ? "Minecraft Bedrock Preview" : "Minecraft Bedrock"),
    [isPreview],
  );

  const backupScopes = React.useMemo(
    () => (Array.isArray(backupInfo?.scopes) ? backupInfo.scopes : []),
    [backupInfo],
  );

  const restoreScopes = React.useMemo(
    () =>
      Array.isArray(restoreArchiveInfo?.scopes)
        ? restoreArchiveInfo.scopes
        : [],
    [restoreArchiveInfo],
  );

  const selectedBackupScopeSet = React.useMemo(
    () => new Set(selectedBackupScopes),
    [selectedBackupScopes],
  );

  const backupHasSharedScope = React.useMemo(
    () =>
      backupScopes.some(
        (scope) =>
          selectedBackupScopeSet.has(String(scope?.key || "").trim()) &&
          Boolean(scope?.shared),
      ),
    [backupScopes, selectedBackupScopeSet],
  );

  const selectedRestoreScopeSet = React.useMemo(
    () => new Set(selectedRestoreScopes),
    [selectedRestoreScopes],
  );

  const restoreConflicts = React.useMemo(
    () =>
      Array.isArray(restoreConflictInfo?.conflicts)
        ? restoreConflictInfo.conflicts
        : [],
    [restoreConflictInfo],
  );

  const restoreHasUnresolvedConflicts = React.useMemo(
    () =>
      restoreConflicts.some((conflict) => {
        const choice = restoreConflictChoices[conflict.id];
        return choice !== "backup" && choice !== "current";
      }),
    [restoreConflictChoices, restoreConflicts],
  );

  const backupArchiveName = React.useMemo(() => {
    const archivePath = String(backupResult?.archivePath || "").trim();
    if (!archivePath) return "";
    const parts = archivePath.split(/[/\\]+/);
    return parts[parts.length - 1] || archivePath;
  }, [backupResult]);

  const getBackupScopeMode = React.useCallback(
    (
      scope: InstanceBackupScope | null | undefined,
    ): InstanceBackupScopeMode | null => {
      if (!scope || !Array.isArray(scope.modes) || scope.modes.length === 0) {
        return null;
      }
      const selectedModeKey = String(
        backupScopeModes[scope.key] || scope.defaultMode || "",
      ).trim();
      return (
        scope.modes.find(
          (mode) => String(mode.key || "").trim() === selectedModeKey,
        ) ||
        scope.modes[0] ||
        null
      );
    },
    [backupScopeModes],
  );

  const backupFullModeSelected = React.useMemo(
    () =>
      backupScopes.some((scope) => {
        if (
          scope.key !== "gameData" ||
          !selectedBackupScopeSet.has(scope.key)
        ) {
          return false;
        }
        return getBackupScopeMode(scope)?.key === "full";
      }),
    [backupScopes, getBackupScopeMode, selectedBackupScopeSet],
  );

  const backupLipPackageCount = React.useMemo(
    () => backupPreparedLipPackages.length,
    [backupPreparedLipPackages],
  );

  const backupLipPackageSummary = React.useMemo(() => {
    const identifiers = backupPreparedLipPackages
      .map((item) => String(item.identifier || "").trim())
      .filter(Boolean);
    const preview = identifiers.slice(0, 2).join(", ");
    if (identifiers.length <= 2) return preview;
    return `${preview} +${identifiers.length - 2}`;
  }, [backupPreparedLipPackages]);

  const restoreHasHighRiskScope = React.useMemo(
    () =>
      restoreScopes.some(
        (scope) =>
          selectedRestoreScopeSet.has(scope.key) &&
          Array.isArray(scope.warnings) &&
          scope.warnings.length > 0,
      ),
    [restoreScopes, selectedRestoreScopeSet],
  );

  const restoreArchiveCreatedAtText = React.useMemo(() => {
    const raw = String(restoreArchiveInfo?.createdAt || "").trim();
    if (!raw) return "";
    const parsed = new Date(raw);
    if (Number.isNaN(parsed.getTime())) return raw;
    return parsed.toLocaleString();
  }, [restoreArchiveInfo]);

  const restoreProgressPercent = React.useMemo(() => {
    if (!restoreProgress || restoreProgress.totalSteps <= 0) return 0;
    return Math.max(
      0,
      Math.min(
        100,
        Math.round(
          (restoreProgress.currentStep / restoreProgress.totalSteps) * 100,
        ),
      ),
    );
  }, [restoreProgress]);

  const restoreProgressScopeLabel = React.useMemo(() => {
    const scopeKey = String(restoreProgress?.scopeKey || "").trim();
    if (scopeKey === "mods") return "Mods";
    return String(restoreProgress?.scopeLabel || "").trim();
  }, [restoreProgress]);

  const restoreProgressText = React.useMemo(() => {
    const phase = String(restoreProgress?.phase || "").trim();
    switch (phase) {
      case "preparing":
        return t("versions.edit.backup.restore.progress_preparing") as string;
      case "extracting":
        return t("versions.edit.backup.restore.progress_extracting") as string;
      case "restoring_scope":
        return t("versions.edit.backup.restore.progress_scope", {
          scope: restoreProgressScopeLabel || "-",
        }) as string;
      case "finalizing":
        return t("versions.edit.backup.restore.progress_finalizing") as string;
      default:
        return t("versions.edit.backup.restore.progress_body") as string;
    }
  }, [restoreProgress?.phase, restoreProgressScopeLabel, t]);

  const restoreProgressStepText = React.useMemo(() => {
    if (!restoreProgress || restoreProgress.totalSteps <= 0) return "";
    return t("versions.edit.backup.restore.progress_step", {
      current: Math.max(1, restoreProgress.currentStep),
      total: restoreProgress.totalSteps,
    }) as string;
  }, [restoreProgress, t]);

  const selectedRestoreScopesKey = React.useMemo(
    () => selectedRestoreScopes.join("\n"),
    [selectedRestoreScopes],
  );

  const restoreResultType = React.useMemo(() => {
    if (restoreResult?.status === "success") return "success" as const;
    if (restoreResult?.status === "partial") return "warning" as const;
    return "error" as const;
  }, [restoreResult]);

  const restoreResultTitleKey = React.useMemo(() => {
    if (restoreResult?.status === "success") {
      return "versions.edit.backup.restore.success_title";
    }
    if (restoreResult?.status === "partial") {
      return "versions.edit.backup.restore.partial_title";
    }
    return "versions.edit.backup.restore.failed_title";
  }, [restoreResult]);

  React.useEffect(() => {
    const off = Events.On(
      EVENT_INSTANCE_BACKUP_RESTORE_PROGRESS,
      (event: any) => {
        const payload = toInstanceBackupRestoreProgress(event?.data ?? event);
        if (!payload.phase) return;
        setRestoreProgress(payload);
      },
    );
    return () => off();
  }, []);

  React.useEffect(() => {
    if (!restoringInstance) {
      setRestoreProgress(null);
    }
  }, [restoringInstance]);

  React.useEffect(() => {
    if (!restoreOpen || !targetName || !restoreArchiveInfo) {
      setRestoreConflictLoading(false);
      return;
    }
    if (selectedRestoreScopes.length === 0) {
      setRestoreConflictLoading(false);
      setRestoreConflictInfo(null);
      setRestoreConflictChoices({});
      return;
    }

    let cancelled = false;
    setRestoreConflictLoading(true);
    void (async () => {
      try {
        const preview = toInstanceBackupRestoreConflictInfo(
          await callVersionService<unknown>(
            "PreviewInstanceBackupRestoreConflicts",
            targetName,
            {
              archivePath: restoreArchiveInfo.archivePath,
              scopes: selectedRestoreScopes,
              conflictResolutions: [],
            } satisfies InstanceBackupRestoreRequest,
          ),
        );
        if (cancelled) return;

        const errorCode = String(preview.errorCode || "").trim();
        if (errorCode) {
          setRestoreConflictInfo(null);
          setRestoreConflictChoices({});
          setError(errorCode);
          return;
        }

        setRestoreConflictInfo(preview);
        setRestoreConflictChoices((prev) => {
          const next: Record<string, "backup" | "current"> = {};
          for (const conflict of preview.conflicts) {
            const previousChoice = prev[conflict.id];
            if (previousChoice === "backup" || previousChoice === "current") {
              next[conflict.id] = previousChoice;
            }
          }
          return next;
        });
      } catch (e: any) {
        if (cancelled) return;
        setRestoreConflictInfo(null);
        setRestoreConflictChoices({});
        setError(String(e?.message || e || "ERR_INSTANCE_BACKUP_ARCHIVE_OPEN"));
      } finally {
        if (!cancelled) {
          setRestoreConflictLoading(false);
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [
    callVersionService,
    restoreArchiveInfo,
    restoreOpen,
    selectedRestoreScopes,
    selectedRestoreScopesKey,
    targetName,
  ]);

  const isBackupModsSnapshotReady = React.useCallback((snapshot: any) => {
    const snapshotStatus = String(snapshot?.status || "").trim();
    const lipSyncStatus = String(snapshot?.lipSyncStatus || "").trim();
    const snapshotError = String(snapshot?.error || "").trim();
    const lipSyncError = String(snapshot?.lipSyncError || "").trim();
    return (
      snapshot &&
      snapshotStatus === "ready" &&
      lipSyncStatus === "ready" &&
      !snapshotError &&
      !lipSyncError
    );
  }, []);

  const openInstanceBackup = React.useCallback(async () => {
    if (!instanceBackupExperimentalEnabled || !targetName) return false;
    setBackupInfoLoading(true);
    setBackupSuccessOpen(false);
    try {
      const info = toInstanceBackupInfo(
        await callVersionService<unknown>("GetInstanceBackupInfo", targetName),
      );
      const errorCode = String(info?.errorCode || "").trim();
      if (errorCode) {
        addToast({
          description: resolveToastText(errorCode),
          color: "danger",
        });
        return false;
      }
      const scopes = Array.isArray(info?.scopes) ? info.scopes : [];
      const modsScope = scopes.find(
        (scope) => scope?.key === "mods" && scope.selectable && scope.exists,
      );
      let snapshot: any = null;
      let allMods: unknown = [];
      if (modsScope) {
        try {
          await refreshInstance(targetName, "version-settings-backup-open");
        } catch (error) {
          console.warn(
            "Failed to refresh mod intelligence before instance backup",
            error,
          );
          setError(ERR_INSTANCE_BACKUP_MODS_STATE_UNAVAILABLE);
          return false;
        }
        try {
          allMods = await GetMods(targetName);
        } catch (error) {
          console.warn("Failed to read mods before instance backup", error);
          setError(ERR_INSTANCE_BACKUP_MODS_STATE_UNAVAILABLE);
          return false;
        }
        snapshot = getInstanceSnapshot(targetName);
        if (!isBackupModsSnapshotReady(snapshot)) {
          console.warn(
            "Instance backup requires a ready mod intelligence snapshot",
            {
              targetName,
              snapshotStatus: String(snapshot?.status || "").trim(),
              lipSyncStatus: String(snapshot?.lipSyncStatus || "").trim(),
              snapshotError: String(snapshot?.error || "").trim(),
              lipSyncError: String(snapshot?.lipSyncError || "").trim(),
            },
          );
          setError(ERR_INSTANCE_BACKUP_MODS_STATE_UNAVAILABLE);
          return false;
        }
      }
      const nextModes: Record<string, string> = {};
      scopes.forEach((scope) => {
        if (!Array.isArray(scope.modes) || scope.modes.length === 0) return;
        const fallback = String(
          scope.defaultMode || scope.modes[0]?.key || "",
        ).trim();
        if (fallback) {
          nextModes[scope.key] = fallback;
        }
      });
      setBackupInfo(info);
      setBackupResult(null);
      setRestoreResult(null);
      setBackupScopeModes(nextModes);
      const preparedLipPackages = modsScope
        ? buildBackupLipPackagesFromSnapshot(snapshot, allMods)
        : [];
      if (modsScope && !preparedLipPackages) {
        setError(ERR_INSTANCE_BACKUP_MODS_STATE_UNAVAILABLE);
        return false;
      }
      setBackupPreparedLipPackages(preparedLipPackages || []);
      setSelectedBackupScopes(
        scopes
          .filter(
            (scope) => Boolean(scope?.selectable) && Boolean(scope?.exists),
          )
          .map((scope) => String(scope?.key || "").trim())
          .filter(Boolean),
      );
      setBackupOpen(true);
      return true;
    } catch (e: any) {
      setError(String(e?.message || e || "ERR_INSTANCE_BACKUP_READ_SOURCE"));
      return false;
    } finally {
      setBackupInfoLoading(false);
    }
  }, [
    buildBackupLipPackagesFromSnapshot,
    callVersionService,
    ERR_INSTANCE_BACKUP_MODS_STATE_UNAVAILABLE,
    getInstanceSnapshot,
    isBackupModsSnapshotReady,
    refreshInstance,
    resolveToastText,
    setError,
    instanceBackupExperimentalEnabled,
    targetName,
  ]);

  const setBackupScopeSelected = React.useCallback(
    (scopeKey: string, selected: boolean) => {
      const normalizedKey = String(scopeKey || "").trim();
      if (!normalizedKey) return;
      setSelectedBackupScopes((prev) => {
        const next = new Set(prev);
        if (selected) {
          next.add(normalizedKey);
        } else {
          next.delete(normalizedKey);
        }
        const orderedKeys = backupScopes
          .map((scope) => String(scope?.key || "").trim())
          .filter(Boolean);
        return orderedKeys.filter((key) => next.has(key));
      });
    },
    [backupScopes],
  );

  const setBackupScopeMode = React.useCallback(
    (scopeKey: string, modeKey: string) => {
      const normalizedScopeKey = String(scopeKey || "").trim();
      const normalizedModeKey = String(modeKey || "").trim();
      if (!normalizedScopeKey) return;
      setBackupScopeModes((prev) => ({
        ...prev,
        [normalizedScopeKey]: normalizedModeKey,
      }));
    },
    [],
  );

  const closeInstanceBackup = React.useCallback(() => {
    if (backingUpInstance) return;
    setBackupOpen(false);
  }, [backingUpInstance]);

  const onInstanceBackupOpenChange = React.useCallback(
    (open: boolean) => {
      if (backingUpInstance) return;
      if (open && !instanceBackupExperimentalEnabled) {
        setBackupOpen(false);
        return;
      }
      setBackupOpen(open);
    },
    [backingUpInstance, instanceBackupExperimentalEnabled],
  );

  const openInstanceBackupDirectory = React.useCallback(async () => {
    const dir = String(
      backupResult?.backupDir || backupInfo?.backupDir || "",
    ).trim();
    if (!dir) return;
    try {
      await (minecraft as any)?.OpenPathDir?.(dir);
    } catch (error) {
      console.warn("Failed to open backup directory", error);
    }
  }, [backupInfo, backupResult]);

  const confirmInstanceBackup = React.useCallback(async () => {
    if (!instanceBackupExperimentalEnabled || !targetName) return false;
    if (selectedBackupScopes.length === 0) {
      return false;
    }

    setBackupOpen(false);
    setBackingUpInstance(true);
    try {
      const request: InstanceBackupRequest = {
        scopes: selectedBackupScopes,
        scopeModes: selectedBackupScopes.reduce<Record<string, string>>(
          (acc, scopeKey) => {
            const selectedScope = backupScopes.find(
              (scope) => scope.key === scopeKey,
            );
            if (
              !selectedScope ||
              !Array.isArray(selectedScope.modes) ||
              selectedScope.modes.length === 0
            ) {
              return acc;
            }
            const modeKey = String(
              backupScopeModes[scopeKey] || selectedScope.defaultMode || "",
            ).trim();
            if (modeKey) {
              acc[scopeKey] = modeKey;
            }
            return acc;
          },
          {},
        ),
        modsLipPackages: backupPreparedLipPackages,
      };
      const resolvedResult = toInstanceBackupResult(
        await callVersionService<unknown>(
          "BackupInstance",
          targetName,
          request,
        ),
      );
      const errorCode = String(resolvedResult?.errorCode || "").trim();
      if (errorCode) {
        setError(errorCode);
        return false;
      }
      setBackupResult(resolvedResult);
      setBackupSuccessOpen(true);
      addToast({
        title: t("versions.edit.backup.success_title") as string,
        color: "success",
      });
      return true;
    } catch (e: any) {
      setError(String(e?.message || e || "ERR_INSTANCE_BACKUP_WRITE_ARCHIVE"));
      return false;
    } finally {
      setBackingUpInstance(false);
    }
  }, [
    backupPreparedLipPackages,
    backupScopeModes,
    backupScopes,
    callVersionService,
    instanceBackupExperimentalEnabled,
    selectedBackupScopes,
    t,
    targetName,
  ]);

  const openInstanceRestore = React.useCallback(async () => {
    if (!instanceBackupExperimentalEnabled || !targetName) return false;
    try {
      const selection = await Dialogs.OpenFile({
        Title: t("versions.edit.backup.restore.file_picker_title"),
        Filters: [
          {
            DisplayName: "Zip Archives",
            Pattern: "*.zip",
          },
        ],
        AllowsMultipleSelection: false,
      });
      let archivePath = "";
      if (Array.isArray(selection) && selection.length > 0) {
        archivePath = String(selection[0] || "").trim();
      } else if (typeof selection === "string") {
        archivePath = String(selection || "").trim();
      }
      if (!archivePath) {
        return false;
      }

      setRestoreInfoLoading(true);
      setRestoreResultOpen(false);
      setRestoreConflictInfo(null);
      setRestoreConflictChoices({});
      setRestoreProgress(null);
      const info = toInstanceBackupArchiveInfo(
        await callVersionService<unknown>(
          "InspectInstanceBackupArchive",
          archivePath,
        ),
      );
      const errorCode = String(info.errorCode || "").trim();
      if (errorCode) {
        addToast({
          description: resolveToastText(errorCode),
          color: "danger",
        });
        return false;
      }
      setRestoreArchiveInfo(info);
      setSelectedRestoreScopes(info.includedScopes);
      setRestoreOpen(true);
      return true;
    } catch (e: any) {
      if (isDialogCancelledError(e)) {
        return false;
      }
      setError("ERR_INSTANCE_BACKUP_ARCHIVE_OPEN");
      return false;
    } finally {
      setRestoreInfoLoading(false);
    }
  }, [
    callVersionService,
    instanceBackupExperimentalEnabled,
    resolveToastText,
    t,
    targetName,
  ]);

  const setRestoreScopeSelected = React.useCallback(
    (scopeKey: string, selected: boolean) => {
      const normalizedKey = String(scopeKey || "").trim();
      if (!normalizedKey) return;
      setSelectedRestoreScopes((prev) => {
        const next = new Set(prev);
        if (selected) {
          next.add(normalizedKey);
        } else {
          next.delete(normalizedKey);
        }
        const orderedKeys = restoreScopes
          .map((scope) => String(scope?.key || "").trim())
          .filter(Boolean);
        return orderedKeys.filter((key) => next.has(key));
      });
    },
    [restoreScopes],
  );

  const setRestoreConflictChoice = React.useCallback(
    (conflictId: string, choice: "backup" | "current") => {
      const normalizedId = String(conflictId || "").trim();
      if (!normalizedId) return;
      setRestoreConflictChoices((prev) => ({
        ...prev,
        [normalizedId]: choice,
      }));
    },
    [],
  );

  const setRestoreConflictChoicesBulk = React.useCallback(
    (conflictIds: string[], choice: "backup" | "current") => {
      const normalizedIds = Array.from(
        new Set(
          conflictIds.map((id) => String(id || "").trim()).filter(Boolean),
        ),
      );
      if (normalizedIds.length === 0) return;
      setRestoreConflictChoices((prev) => {
        const next = { ...prev };
        normalizedIds.forEach((id) => {
          next[id] = choice;
        });
        return next;
      });
    },
    [],
  );

  const closeInstanceRestore = React.useCallback(() => {
    if (restoringInstance) return;
    setRestoreOpen(false);
  }, [restoringInstance]);

  const onInstanceRestoreOpenChange = React.useCallback(
    (open: boolean) => {
      if (restoringInstance) return;
      if (open && !instanceBackupExperimentalEnabled) {
        setRestoreOpen(false);
        return;
      }
      setRestoreOpen(open);
    },
    [instanceBackupExperimentalEnabled, restoringInstance],
  );

  const confirmInstanceRestore = React.useCallback(async () => {
    if (
      !instanceBackupExperimentalEnabled ||
      !targetName ||
      !restoreArchiveInfo
    ) {
      return false;
    }
    if (selectedRestoreScopes.length === 0) return false;
    if (restoreConflictLoading || restoreHasUnresolvedConflicts) return false;

    setRestoreProgress({
      phase: "preparing",
      currentStep: 1,
      totalSteps: selectedRestoreScopes.length + 3,
      scopeKey: "",
      scopeLabel: "",
      ts: Date.now(),
    });
    setRestoreOpen(false);
    setRestoringInstance(true);
    try {
      const conflictResolutions: InstanceBackupRestoreResolution[] =
        restoreConflicts
          .map((conflict) => {
            const choice = restoreConflictChoices[conflict.id];
            if (choice !== "backup" && choice !== "current") {
              return null;
            }
            return {
              conflictId: conflict.id,
              choice,
            };
          })
          .filter((item): item is InstanceBackupRestoreResolution =>
            Boolean(item),
          );
      const request: InstanceBackupRestoreRequest = {
        archivePath: restoreArchiveInfo.archivePath,
        scopes: selectedRestoreScopes,
        conflictResolutions,
      };
      const resolvedResult = toInstanceBackupRestoreResult(
        await callVersionService<unknown>(
          "RestoreInstanceBackup",
          targetName,
          request,
        ),
      );
      if (!resolvedResult.scopeResults.length && resolvedResult.errorCode) {
        setError(resolvedResult.errorCode);
        return false;
      }
      setRestoreResult(resolvedResult);
      setRestoreResultOpen(true);
      addToast({
        title: t(
          resolvedResult.status === "success"
            ? "versions.edit.backup.restore.success_title"
            : resolvedResult.status === "partial"
              ? "versions.edit.backup.restore.partial_title"
              : "versions.edit.backup.restore.failed_title",
        ) as string,
        color:
          resolvedResult.status === "success"
            ? "success"
            : resolvedResult.status === "partial"
              ? "warning"
              : "danger",
      });
      return resolvedResult.status === "success";
    } catch (e: any) {
      setError(
        String(e?.message || e || "ERR_INSTANCE_BACKUP_RESTORE_WRITE_TARGET"),
      );
      return false;
    } finally {
      setRestoringInstance(false);
    }
  }, [
    callVersionService,
    instanceBackupExperimentalEnabled,
    restoreConflictChoices,
    restoreConflictLoading,
    restoreConflicts,
    restoreHasUnresolvedConflicts,
    restoreArchiveInfo,
    selectedRestoreScopes,
    t,
    targetName,
  ]);

  // Save handler
  const onSave = React.useCallback(
    async (destPath?: string) => {
      if (!hasBackend || !targetName) {
        navigate(-1);
        return false;
      }
      const validate = ValidateVersionFolderName as any;
      const rename = RenameVersionFolder as any;
      const save = SaveVersionMeta as any;
      const saver = SaveVersionLogoDataUrl as any;

      const nn = (newName || "").trim();
      if (!nn) {
        setError("ERR_INVALID_NAME");
        return false;
      }
      const type = versionType || (isPreview ? "preview" : "release");

      if (nn !== targetName) {
        if (typeof validate === "function") {
          const msg: string = await validate(nn);
          if (msg) {
            setError(msg);
            return false;
          }
        }
        if (typeof rename === "function") {
          const err: string = await rename(targetName, nn);
          if (err) {
            setError(err);
            return false;
          }
        }
        setTargetName(nn);
      }

      if (typeof save === "function") {
        const err2: string = await save(
          nn,
          gameVersion,
          type,
          !!enableIsolation,
          !!enableConsole,
          !!enableEditorMode,
          !!enableRenderDragon,
          !!enableCtrlRReloadResources,
          launchArgs,
          envVars,
        );
        if (err2) {
          setError(err2);
          return false;
        }
      }
      try {
        if (typeof saver === "function" && logoDataUrl) {
          const e = await saver(nn, logoDataUrl);
          if (e) {
            setError(e);
            return false;
          }
        }
      } catch {}

      if (destPath === "-1") {
        navigate(-1);
      } else {
        navigate(typeof destPath === "string" ? destPath : returnToPath);
      }
      return true;
    },
    [
      hasBackend,
      targetName,
      newName,
      gameVersion,
      isPreview,
      enableIsolation,
      enableConsole,
      enableEditorMode,
      enableRenderDragon,
      enableCtrlRReloadResources,
      logoDataUrl,
      returnToPath,
      navigate,
      versionType,
      envVars,
      launchArgs,
    ],
  );

  // Delete handler
  const onDeleteConfirm = React.useCallback(async () => {
    if (!hasBackend || !targetName) {
      setDeleteOpen(false);
      return;
    }
    setDeleting(true);
    try {
      const del = DeleteVersionFolder as any;
      if (typeof del === "function") {
        const err: string = await del(targetName);
        if (err) {
          setError(String(err));
          setDeleting(false);
          setDeleteOpen(false);
          return;
        }
        setDeleteOpen(false);
        setDeleteSuccessMsg(targetName);
        try {
          const cur = readCurrentVersionName();
          if (cur === targetName) {
            clearCurrentVersionName("version-settings.delete");
          }
        } catch {}
        setDeleteSuccessOpen(true);
      }
    } catch {
      setError("ERR_DELETE_FAILED");
    } finally {
      setDeleting(false);
    }
  }, [hasBackend, targetName]);

  // LeviLamina install handlers
  const proceedInstallLeviLamina = React.useCallback(
    async (forcedVersion?: string) => {
      if (!hasBackend || !targetName || !gameVersion) return false;
      const installVersion = String(
        forcedVersion || resolvedLLTargetVersion || "",
      ).trim();
      if (!installVersion) {
        addToast({
          description: resolveToastText("ERR_LL_VERSION_UNSUPPORTED"),
          color: "danger",
        });
        return false;
      }
      setInstallingLL(true);
      try {
        const lipReady = await ensureLipRuntimeAvailable();
        if (!lipReady) {
          return false;
        }
        const installLL = (minecraft as any)?.InstallLeviLamina;
        if (typeof installLL === "function") {
          await runWithLipTask(
            {
              action: llInstallActionKind,
              target: targetName,
              methods: ["Install", "Update"],
              feedbackMode: "on_error",
            },
            async ({ addLog }) => {
              const err: string = await installLL(
                gameVersion,
                targetName,
                installVersion,
              );
              if (err) {
                throw new Error(String(err));
              }
            },
          );
          await refreshInstance(targetName, "version-settings-install-ll");
          setIsLLInstalled(true);
          setLLExplicitInstalled(true);
          setCurrentLLVersion(installVersion);
          addToast({
            title: t(llInstallSuccessKey) as string,
            color: "success",
          });
          return true;
        }
        return false;
      } catch (e: any) {
        const rawError = String(e?.message || e || "");
        let toastKeyOrMsg = rawError;
        if (rawError.includes("ERR_LIP_INSTALL_FAILED")) {
          toastKeyOrMsg = "mods.err_lip_install_failed_suggestion";
        }
        addToast({
          description: resolveToastText(toastKeyOrMsg),
          color: "danger",
        });
        return false;
      } finally {
        setInstallingLL(false);
      }
    },
    [
      gameVersion,
      hasBackend,
      isLLInstalled,
      llInstallActionKind,
      llInstallSuccessKey,
      ensureLipRuntimeAvailable,
      resolveToastText,
      resolvedLLTargetVersion,
      runWithLipTask,
      refreshInstance,
      t,
      targetName,
    ],
  );

  const handleInstallLeviLamina = React.useCallback(async () => {
    if (!hasBackend || !targetName || !gameVersion) return false;

    const targetLLVersion = resolvedLLTargetVersion;
    if (!targetLLVersion) {
      addToast({
        description: resolveToastText("ERR_LL_VERSION_UNSUPPORTED"),
        color: "danger",
      });
      return false;
    }
    if (isLLInstalled && !canInstallSelectedLLVersion) {
      return false;
    }

    if (targetLLVersion && targetLLVersion.includes("rc")) {
      setRcVersion(targetLLVersion);
      rcOnOpen();
      return true;
    }

    return await proceedInstallLeviLamina(targetLLVersion);
  }, [
    hasBackend,
    targetName,
    gameVersion,
    isLLInstalled,
    canInstallSelectedLLVersion,
    rcOnOpen,
    proceedInstallLeviLamina,
    resolvedLLTargetVersion,
    resolveToastText,
  ]);

  const openLeviLaminaVersionSelect = React.useCallback(async () => {
    const lipReady = await ensureLipRuntimeAvailable();
    if (!lipReady) {
      return;
    }
    llVersionSelectOnOpen();
  }, [ensureLipRuntimeAvailable, llVersionSelectOnOpen]);

  const openLLInstallConfirm = React.useCallback(() => {
    llInstallConfirmOnOpen();
  }, [llInstallConfirmOnOpen]);

  const closeLLInstallConfirm = React.useCallback(() => {
    if (installingLL) return;
    llInstallConfirmOnClose();
  }, [installingLL, llInstallConfirmOnClose]);

  const confirmLeviLaminaVersionSelect = React.useCallback(async () => {
    if (!resolvedLLTargetVersion) {
      addToast({
        description: resolveToastText("ERR_LL_VERSION_UNSUPPORTED"),
        color: "danger",
      });
      return false;
    }
    if (isLLInstalled && !canInstallSelectedLLVersion) {
      return false;
    }
    const lipReady = await ensureLipRuntimeAvailable();
    if (!lipReady) {
      return false;
    }
    llVersionSelectOnClose();
    llInstallConfirmOnOpen();
    return true;
  }, [
    canInstallSelectedLLVersion,
    ensureLipRuntimeAvailable,
    isLLInstalled,
    llInstallConfirmOnOpen,
    llVersionSelectOnClose,
    resolveToastText,
    resolvedLLTargetVersion,
  ]);

  const confirmInstallLeviLaminaAction = React.useCallback(async () => {
    const ok = await handleInstallLeviLamina();
    if (ok) {
      llInstallConfirmOnClose();
    }
    return ok;
  }, [handleInstallLeviLamina, llInstallConfirmOnClose]);

  const openLLUninstallConfirm = React.useCallback(async () => {
    const latestState = await refreshLLStateForUninstall();
    if (!latestState?.installed && !isLLInstalled) return;
    llUninstallConfirmOnOpen();
  }, [isLLInstalled, llUninstallConfirmOnOpen, refreshLLStateForUninstall]);

  const closeLLUninstallConfirm = React.useCallback(() => {
    if (uninstallingLL) return;
    llUninstallConfirmOnClose();
  }, [llUninstallConfirmOnClose, uninstallingLL]);

  const confirmUninstallLL = React.useCallback(async (): Promise<boolean> => {
    if (!targetName) return false;
    if (llUninstallBlocked) {
      addToast({
        description: resolveToastText(ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS),
        color: "danger",
      });
      return false;
    }
    setUninstallingLL(true);
    let demotedToDependency = false;
    try {
      await runWithLipTask(
        {
          action: "uninstall",
          target: targetName,
          methods: ["Uninstall"],
          feedbackMode: "on_error",
        },
        async ({ addLog }) => {
          addLog("info", t("mods.action_uninstall"));
          const err = await (minecraft as any)?.UninstallLeviLamina?.(
            targetName,
          );
          if (err === ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY) {
            demotedToDependency = true;
            addLog("warning", `LeviLamina: ${err}`);
            return;
          }
          if (err) {
            throw new Error(String(err));
          }
        },
      );
      await refreshInstance(targetName, "version-settings-uninstall-ll");
      if (demotedToDependency) {
        await refreshLLStateForUninstall();
        llUninstallConfirmOnClose();
        openDemotedWarning(["LeviLamina"]);
        return false;
      }
      setIsLLInstalled(false);
      setLLExplicitInstalled(false);
      setCurrentLLVersion("");
      addToast({ title: t("common.success") as string, color: "success" });
      return true;
    } catch (e) {
      const errCode = String((e as any)?.message || e || "").trim();
      if (
        errCode === ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY ||
        errCode === ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS
      ) {
        await refreshLLStateForUninstall();
      }
      addToast({
        description: resolveToastText(errCode),
        color: "danger",
      });
      return false;
    } finally {
      setUninstallingLL(false);
    }
  }, [
    ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY,
    ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS,
    llUninstallBlocked,
    llUninstallConfirmOnClose,
    openDemotedWarning,
    refreshLLStateForUninstall,
    refreshInstance,
    resolveToastText,
    runWithLipTask,
    t,
    targetName,
  ]);

  return {
    // Navigation
    navigate,
    returnToPath,

    // Form state
    targetName,
    selectedTab,
    setSelectedTab,
    newName,
    setNewName,
    gameVersion,
    versionType,
    isPreview,
    enableIsolation,
    setEnableIsolation,
    enableConsole,
    setEnableConsole,
    enableEditorMode,
    setEnableEditorMode,
    enableRenderDragon,
    setEnableRenderDragon,
    enableCtrlRReloadResources,
    setEnableCtrlRReloadResources,
    envVars,
    setEnvVars,
    launchArgs,
    setLaunchArgs,
    isRegistered,
    setIsRegistered,
    loading,
    error,
    setError,
    logoDataUrl,
    setLogoDataUrl,
    hasBackend,

    // Modal states
    unregisterOpen,
    setUnregisterOpen,
    unregisterSuccessOpen,
    setUnregisterSuccessOpen,
    errorOpen,
    setErrorOpen,
    deleteOpen,
    setDeleteOpen,
    deleteSuccessOpen,
    setDeleteSuccessOpen,
    deleting,
    deleteSuccessMsg,
    backupOpen,
    setBackupOpen,
    backupInfoLoading,
    backupInfo,
    backupScopes,
    backupHasSharedScope,
    selectedBackupScopes,
    selectedBackupScopeSet,
    backupScopeModes,
    backupPreparedLipPackages,
    backupLipPackageCount,
    backupLipPackageSummary,
    backupFullModeSelected,
    backingUpInstance,
    instanceBackupExperimentalEnabled,
    backupSuccessOpen,
    setBackupSuccessOpen,
    backupResult,
    backupArchiveName,
    backupGameDataLabel,
    getBackupScopeMode,
    restoreOpen,
    restoreInfoLoading,
    restoreArchiveInfo,
    restoreScopes,
    selectedRestoreScopes,
    selectedRestoreScopeSet,
    restoreConflictLoading,
    restoreConflictInfo,
    restoreConflicts,
    restoreConflictChoices,
    restoreHasUnresolvedConflicts,
    restoringInstance,
    restoreProgress,
    restoreProgressPercent,
    restoreProgressText,
    restoreProgressStepText,
    restoreResultOpen,
    setRestoreResultOpen,
    restoreResult,
    restoreHasHighRiskScope,
    restoreArchiveCreatedAtText,
    restoreResultType,
    restoreResultTitleKey,

    // Unsaved changes
    unsavedOpen,
    unsavedOnClose,
    unsavedOnOpenChange,
    pendingNavPath,

    // LeviLamina
    isLLSupported,
    llSupportedVersions,
    latestLLVersion,
    selectedLLVersion,
    setSelectedLLVersion,
    currentLLVersion,
    canInstallSelectedLLVersion,
    llInstallBlockedReasonKey,
    llOnlyLatestSelectable,
    llInstallActionKind,
    llInstallActionLabelKey,
    llInstallSuccessKey,
    llUninstallBlocked,
    llUninstallWarning,
    demotedWarningNames,
    installingLL,
    uninstallingLL,
    llInstallConfirmOpen,
    llUninstallConfirmOpen,
    llVersionSelectOpen,
    llVersionSelectOnOpenChange,
    llVersionSelectOnClose,
    rcOpen,
    rcOnOpenChange,
    rcOnClose,
    rcVersion,
    isLLInstalled,
    lipMissingOpen,
    setLipMissingOpen,
    resolvedLLTargetVersion,
    demotedWarningOpen,
    demotedWarningOnOpenChange,

    // Error helper
    getErrorKey,

    // Actions
    onSave,
    onDeleteConfirm,
    openInstanceBackup,
    closeInstanceBackup,
    onInstanceBackupOpenChange,
    setBackupScopeSelected,
    setBackupScopeMode,
    confirmInstanceBackup,
    openInstanceBackupDirectory,
    openInstanceRestore,
    closeInstanceRestore,
    onInstanceRestoreOpenChange,
    setRestoreScopeSelected,
    setRestoreConflictChoice,
    setRestoreConflictChoicesBulk,
    confirmInstanceRestore,
    proceedInstallLeviLamina,
    openLeviLaminaVersionSelect,
    confirmLeviLaminaVersionSelect,
    openLipComponentsSettings,
    openLLInstallConfirm,
    closeLLInstallConfirm,
    confirmInstallLeviLaminaAction,
    openLLUninstallConfirm,
    closeLLUninstallConfirm,
    closeDemotedWarning,
    confirmUninstallLL,
    handleInstallLeviLamina,
  };
};
