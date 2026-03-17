import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Call, Events } from "@wailsio/runtime";
import { GetMods, IsModEnabled } from "bindings/github.com/liteldev/LeviLauncher/modsservice";
import * as types from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import {
  fetchLIPPackagesIndex,
  fetchLIPSelfVariantRelations,
  type LIPPackageBasicInfo,
} from "@/utils/content";
import { useCurrentVersion } from "@/utils/CurrentVersionContext";
import {
  buildListItems,
  buildModLIPStateByFolder,
  buildSelfVariantCandidates,
  collectCandidateLIPIdentifiers,
  filterVisibleMods,
  parseLIPPackageInstallState,
  type CandidateLIPIdentifier,
  type LIPPackageInstallState,
  type LipGroupItem,
  type ModLIPState,
  type ModListItem,
} from "@/utils/modIntelligenceResolver";
import { useStartupInteractive } from "@/utils/startupState";

export type InstanceModSnapshot = {
  status: "idle" | "loading" | "ready" | "refreshing" | "error";
  lipSyncStatus: "idle" | "loading" | "ready" | "error";
  lipSyncError: string;
  modsInfo: types.ModInfo[];
  enabledByFolder: Map<string, boolean>;
  modLIPStateByFolder: Map<string, ModLIPState>;
  lipGroupItems: LipGroupItem[];
  normalItems: ModListItem[];
  lipInstallStateByIdentifier: Map<string, LIPPackageInstallState>;
  llState: LIPPackageInstallState;
  updatedAt: number;
  error?: string;
};

type ModIntelligenceContextValue = {
  lipSourceLoaded: boolean;
  snapshotRevision: number;
  ensureInstanceHydrated: (
    instanceName: string,
    opts?: { force?: boolean; background?: boolean; reason?: string },
  ) => Promise<void>;
  getInstanceSnapshot: (instanceName: string) => InstanceModSnapshot | null;
  ensurePackageInstallState: (
    instanceName: string,
    identifier: string,
  ) => Promise<LIPPackageInstallState>;
  invalidateInstance: (instanceName: string, reason?: string) => void;
  refreshInstance: (instanceName: string, reason?: string) => Promise<void>;
};

const ModIntelligenceContext = createContext<
  ModIntelligenceContextValue | undefined
>(undefined);

const INSTANCE_CACHE_TTL_MS = 30_000;
const PACKAGE_STATE_CACHE_TTL_MS = 30_000;
const LEVILAMINA_IDENTIFIER = "LiteLDev/LeviLamina";

const normalizeIdentifierLookupKey = (value: string): string =>
  String(value || "").trim().toLowerCase();

const emptyInstallState = (): LIPPackageInstallState => ({
  installed: false,
  explicitInstalled: false,
  installedVersion: "",
  error: "",
});

const parseErrorCode = (value: unknown): string => {
  const source =
    typeof value === "string"
      ? value
      : value instanceof Error
        ? value.message
        : String(value || "");
  const trimmed = source.trim();
  if (!trimmed) return "";
  const match = trimmed.match(/ERR_[A-Z0-9_]+/);
  return match ? match[0] : trimmed;
};

const findInstallStateKeyByLookupKey = (
  map: Map<string, LIPPackageInstallState>,
  lookupKey: string,
): string => {
  const normalizedLookupKey = normalizeIdentifierLookupKey(lookupKey);
  if (!normalizedLookupKey) return "";
  for (const key of map.keys()) {
    if (normalizeIdentifierLookupKey(key) === normalizedLookupKey) {
      return key;
    }
  }
  return "";
};

const findInstallStateByLookupKey = (
  map: Map<string, LIPPackageInstallState>,
  lookupKey: string,
): LIPPackageInstallState | null => {
  const key = findInstallStateKeyByLookupKey(map, lookupKey);
  return key ? map.get(key) || null : null;
};

const buildInitialSnapshot = (): InstanceModSnapshot => ({
  status: "idle",
  lipSyncStatus: "idle",
  lipSyncError: "",
  modsInfo: [],
  enabledByFolder: new Map(),
  modLIPStateByFolder: new Map(),
  lipGroupItems: [],
  normalItems: [],
  lipInstallStateByIdentifier: new Map(),
  llState: emptyInstallState(),
  updatedAt: 0,
});

type PackageStateCacheValue = {
  at: number;
  state: LIPPackageInstallState;
};

type ParsedInstallStateEntry = {
  identifierKey: string;
  lookupKey: string;
  state: LIPPackageInstallState;
};

const parseInstallStateEntries = (value: unknown): ParsedInstallStateEntry[] => {
  if (!Array.isArray(value)) return [];

  return value
    .map((item) => {
      if (!item || typeof item !== "object" || Array.isArray(item)) {
        return null;
      }
      const record = item as Record<string, unknown>;
      const identifierKey = String(record.identifierKey ?? "").trim();
      if (!identifierKey) return null;

      return {
        identifierKey,
        lookupKey: normalizeIdentifierLookupKey(identifierKey),
        state: parseLIPPackageInstallState(record.state),
      } satisfies ParsedInstallStateEntry;
    })
    .filter((item): item is ParsedInstallStateEntry => Boolean(item));
};

const buildFallbackSnapshot = (args: {
  base: InstanceModSnapshot;
  modsInfo: types.ModInfo[];
  enabledByFolder: Map<string, boolean>;
  lipSyncStatus: InstanceModSnapshot["lipSyncStatus"];
  lipSyncError?: string;
}): InstanceModSnapshot => {
  const {
    base,
    modsInfo,
    enabledByFolder,
    lipSyncStatus,
    lipSyncError = "",
  } = args;
  const { normalItems } = buildListItems({
    modsInfo,
    modLIPStateByFolder: new Map(),
    lipInstallStateByIdentifier: new Map(),
    enabledByFolder,
    sortDirection: "asc",
  });

  return {
    ...base,
    status: "ready",
    lipSyncStatus,
    lipSyncError,
    modsInfo,
    enabledByFolder,
    modLIPStateByFolder: new Map(),
    lipGroupItems: [],
    normalItems,
    lipInstallStateByIdentifier: new Map(),
    updatedAt: Date.now(),
    error: "",
  };
};

export const ModIntelligenceProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const { currentVersionName } = useCurrentVersion();
  const startupInteractive = useStartupInteractive();
  const [lipSourceLoaded, setLipSourceLoaded] = useState(false);
  const [snapshotRevision, setRevision] = useState(0);

  const snapshotsRef = useRef<Map<string, InstanceModSnapshot>>(new Map());
  const inFlightRef = useRef<Map<string, Promise<void>>>(new Map());
  const packageStateCacheRef = useRef<Map<string, PackageStateCacheValue>>(
    new Map(),
  );

  const lipPackagesByNameRef = useRef<Record<string, LIPPackageBasicInfo[]>>({});
  const lipPackageByIdentifierRef = useRef<Record<string, LIPPackageBasicInfo>>(
    {},
  );
  const selfVariantCandidatesRef = useRef<
    ReturnType<typeof buildSelfVariantCandidates>
  >([]);
  const lipSourceErrorRef = useRef("");

  const bumpRevision = useCallback(() => {
    setRevision((prev) => prev + 1);
  }, []);

  const setSnapshot = useCallback(
    (instanceName: string, next: InstanceModSnapshot) => {
      const key = String(instanceName || "").trim();
      if (!key) return;
      snapshotsRef.current.set(key, next);
      bumpRevision();
    },
    [bumpRevision],
  );

  const queryInstallStates = useCallback(
    async (
      instanceName: string,
      identifiers: CandidateLIPIdentifier[],
    ): Promise<Map<string, LIPPackageInstallState>> => {
      const normalizedName = String(instanceName || "").trim();
      if (!normalizedName || identifiers.length === 0) {
        return new Map();
      }

      const requestIdentifiers = identifiers
        .map((item) => String(item.identifierKey || item.identifier || "").trim())
        .filter(Boolean);

      if (requestIdentifiers.length === 0) {
        return new Map();
      }

      try {
        const raw = await Call.ByName(
          "main.Minecraft.GetLIPPackageInstallStates",
          normalizedName,
          requestIdentifiers,
        );
        const parsedEntries = parseInstallStateEntries(raw);
        const nextMap = new Map<string, LIPPackageInstallState>();

        for (const entry of parsedEntries) {
          if (!entry.identifierKey || nextMap.has(entry.identifierKey)) {
            continue;
          }
          nextMap.set(entry.identifierKey, entry.state);
          packageStateCacheRef.current.set(
            `${normalizedName}::${entry.lookupKey}`,
            { at: Date.now(), state: entry.state },
          );
        }

        for (const request of identifiers) {
          const identifierKey = String(
            request.identifierKey || request.identifier || "",
          ).trim();
          if (!identifierKey || nextMap.has(identifierKey)) continue;
          nextMap.set(identifierKey, {
            installed: false,
            explicitInstalled: false,
            installedVersion: "",
            error: "ERR_LIP_PACKAGE_QUERY_FAILED",
          });
        }

        return nextMap;
      } catch (error) {
        const errorCode = parseErrorCode(error) || "ERR_LIP_PACKAGE_QUERY_FAILED";
        const nextMap = new Map<string, LIPPackageInstallState>();

        for (const request of identifiers) {
          const identifierKey = String(
            request.identifierKey || request.identifier || "",
          ).trim();
          if (!identifierKey || nextMap.has(identifierKey)) continue;

          const state: LIPPackageInstallState = {
            installed: false,
            explicitInstalled: false,
            installedVersion: "",
            error: errorCode,
          };
          nextMap.set(identifierKey, state);
          packageStateCacheRef.current.set(
            `${normalizedName}::${normalizeIdentifierLookupKey(identifierKey)}`,
            { at: Date.now(), state },
          );
        }

        return nextMap;
      }
    },
    [],
  );

  const queryInstallState = useCallback(
    async (instanceName: string, identifier: string): Promise<LIPPackageInstallState> => {
      const normalizedIdentifier = String(identifier || "").trim();
      if (!String(instanceName || "").trim() || !normalizedIdentifier) {
        return emptyInstallState();
      }

      const result = await queryInstallStates(instanceName, [
        {
          identifier: normalizedIdentifier,
          identifierKey: normalizedIdentifier,
        },
      ]);
      return result.get(normalizedIdentifier) || {
        installed: false,
        explicitInstalled: false,
        installedVersion: "",
        error: "ERR_LIP_PACKAGE_QUERY_FAILED",
      };
    },
    [queryInstallStates],
  );

  const syncInstanceLipState = useCallback(
    async (instanceName: string, baseSnapshot: InstanceModSnapshot) => {
      const normalizedName = String(instanceName || "").trim();
      if (!normalizedName) return;

      const currentBaseSnapshot =
        snapshotsRef.current.get(normalizedName) || baseSnapshot;

      if (!lipSourceLoaded) {
        setSnapshot(
          normalizedName,
          buildFallbackSnapshot({
            base: currentBaseSnapshot,
            modsInfo: currentBaseSnapshot.modsInfo,
            enabledByFolder: currentBaseSnapshot.enabledByFolder,
            lipSyncStatus: "loading",
          }),
        );
        return;
      }

      const currentSnapshot = snapshotsRef.current.get(normalizedName) || baseSnapshot;

      if (lipSourceErrorRef.current) {
        const llState = await queryInstallState(normalizedName, LEVILAMINA_IDENTIFIER);
        setSnapshot(normalizedName, {
          ...buildFallbackSnapshot({
            base: currentSnapshot,
            modsInfo: currentSnapshot.modsInfo,
            enabledByFolder: currentSnapshot.enabledByFolder,
            lipSyncStatus: "error",
            lipSyncError: lipSourceErrorRef.current,
          }),
          llState,
        });
        return;
      }

      const modLIPStateByFolder = buildModLIPStateByFolder({
        modsInfo: currentSnapshot.modsInfo,
        lipSourceLoaded: true,
        selfVariantCandidates: selfVariantCandidatesRef.current,
        lipPackagesByName: lipPackagesByNameRef.current,
        lipPackageByIdentifier: lipPackageByIdentifierRef.current,
      });

      const candidateIdentifiers = collectCandidateLIPIdentifiers(modLIPStateByFolder);
      const llCandidate: CandidateLIPIdentifier = {
        identifier: LEVILAMINA_IDENTIFIER,
        identifierKey: LEVILAMINA_IDENTIFIER,
      };
      const allInstallStates = await queryInstallStates(normalizedName, [
        llCandidate,
        ...candidateIdentifiers,
      ]);
      const llState =
        allInstallStates.get(LEVILAMINA_IDENTIFIER) || emptyInstallState();
      const lipInstallStateByIdentifier = new Map<string, LIPPackageInstallState>();
      for (const item of candidateIdentifiers) {
        const state = allInstallStates.get(item.identifierKey);
        if (!state) continue;
        lipInstallStateByIdentifier.set(item.identifierKey, state);
      }

      const llBlockingError =
        Boolean(llState.error) && String(llState.error).trim() !== "ERR_LIP_NOT_INSTALLED";
      const hasBlockingError =
        llBlockingError ||
        Array.from(lipInstallStateByIdentifier.values()).some(
          (state) =>
            Boolean(state.error) &&
            String(state.error).trim() !== "ERR_LIP_NOT_INSTALLED",
        );

      if (hasBlockingError) {
        const firstError =
          (llBlockingError
            ? llState.error
            : Array.from(lipInstallStateByIdentifier.values()).find(
                (state) =>
                  Boolean(state.error) &&
                  String(state.error).trim() !== "ERR_LIP_NOT_INSTALLED",
              )?.error) || "ERR_LIP_PACKAGE_QUERY_FAILED";

        setSnapshot(normalizedName, {
          ...buildFallbackSnapshot({
            base: currentSnapshot,
            modsInfo: currentSnapshot.modsInfo,
            enabledByFolder: currentSnapshot.enabledByFolder,
            lipSyncStatus: "error",
            lipSyncError: String(firstError || "").trim(),
          }),
          llState,
        });
        return;
      }

      const { lipGroupItems, normalItems } = buildListItems({
        modsInfo: currentSnapshot.modsInfo,
        modLIPStateByFolder,
        lipInstallStateByIdentifier,
        enabledByFolder: currentSnapshot.enabledByFolder,
        sortDirection: "asc",
      });

      setSnapshot(normalizedName, {
        ...currentSnapshot,
        status: "ready",
        lipSyncStatus: "ready",
        lipSyncError: "",
        modLIPStateByFolder,
        lipGroupItems,
        normalItems,
        lipInstallStateByIdentifier,
        llState,
        updatedAt: Date.now(),
        error: "",
      });
    },
    [lipSourceLoaded, queryInstallState, queryInstallStates, setSnapshot],
  );

  useEffect(() => {
    if (!startupInteractive) return;
    let cancelled = false;

    const loadLIPSource = async () => {
      lipSourceErrorRef.current = "";
      try {
        const [packages, relations] = await Promise.all([
          fetchLIPPackagesIndex(),
          fetchLIPSelfVariantRelations(),
        ]);
        if (cancelled) return;

        const groupedByName: Record<string, LIPPackageBasicInfo[]> = {};
        const groupedByIdentifier: Record<string, LIPPackageBasicInfo> = {};

        for (const pkg of packages || []) {
          const normalizedName = String(pkg?.name || "").trim().toLowerCase();
          if (normalizedName) {
            if (!Array.isArray(groupedByName[normalizedName])) {
              groupedByName[normalizedName] = [];
            }
            groupedByName[normalizedName].push(pkg);
          }

          const identifierLookupKey = normalizeIdentifierLookupKey(
            String(pkg?.identifier || ""),
          );
          if (identifierLookupKey && !groupedByIdentifier[identifierLookupKey]) {
            groupedByIdentifier[identifierLookupKey] = pkg;
          }
        }

        lipPackagesByNameRef.current = groupedByName;
        lipPackageByIdentifierRef.current = groupedByIdentifier;
        selfVariantCandidatesRef.current = buildSelfVariantCandidates(
          Array.isArray(relations) ? relations : [],
        );
      } catch (error) {
        if (cancelled) return;
        lipSourceErrorRef.current =
          parseErrorCode(error) || "ERR_LIP_PACKAGE_QUERY_FAILED";
        lipPackagesByNameRef.current = {};
        lipPackageByIdentifierRef.current = {};
        selfVariantCandidatesRef.current = [];
      } finally {
        if (!cancelled) setLipSourceLoaded(true);
      }
    };

    void loadLIPSource();
    return () => {
      cancelled = true;
    };
  }, [startupInteractive]);

  const ensureInstanceHydrated = useCallback<
    ModIntelligenceContextValue["ensureInstanceHydrated"]
  >(
    async (instanceName, opts) => {
      const normalizedName = String(instanceName || "").trim();
      if (!normalizedName) return;

      const force = opts?.force === true;
      const now = Date.now();
      const existing = snapshotsRef.current.get(normalizedName);
      if (
        !force &&
        existing &&
        existing.status === "ready" &&
        now - existing.updatedAt < INSTANCE_CACHE_TTL_MS &&
        (!lipSourceLoaded ||
          existing.lipSyncStatus === "ready" ||
          existing.lipSyncStatus === "error")
      ) {
        return;
      }

      const pending = inFlightRef.current.get(normalizedName);
      if (pending) return pending;

      const run = (async () => {
        const previous = snapshotsRef.current.get(normalizedName) || buildInitialSnapshot();
        setSnapshot(normalizedName, {
          ...previous,
          status: previous.updatedAt > 0 ? "refreshing" : "loading",
          error: "",
        });

        try {
          const mods = filterVisibleMods(await GetMods(normalizedName));
          const enabledEntries = await Promise.all(
            mods.map(async (mod) => {
              const folder =
                String((mod as any)?.folder || "").trim() ||
                String(mod?.name || "").trim();
              if (!folder) return ["", false] as const;
              try {
                const enabled = await (IsModEnabled as any)?.(normalizedName, folder);
                return [folder, Boolean(enabled)] as const;
              } catch {
                return [folder, false] as const;
              }
            }),
          );

          const enabledByFolder = new Map<string, boolean>();
          for (const [folder, enabled] of enabledEntries) {
            if (!folder) continue;
            enabledByFolder.set(folder, enabled);
          }

          const localSnapshot = buildFallbackSnapshot({
            base: previous,
            modsInfo: mods,
            enabledByFolder,
            lipSyncStatus: "loading",
          });
          setSnapshot(normalizedName, localSnapshot);

          await syncInstanceLipState(normalizedName, localSnapshot);
        } catch (error) {
          const current = snapshotsRef.current.get(normalizedName) || previous;
          setSnapshot(normalizedName, {
            ...current,
            status: "error",
            error: parseErrorCode(error) || "ERR_MOD_INTELLIGENCE_REFRESH_FAILED",
            updatedAt: current.updatedAt || Date.now(),
          });
        } finally {
          inFlightRef.current.delete(normalizedName);
        }
      })();

      inFlightRef.current.set(normalizedName, run);
      return run;
    },
    [lipSourceLoaded, setSnapshot, syncInstanceLipState],
  );

  const refreshInstance = useCallback<ModIntelligenceContextValue["refreshInstance"]>(
    async (instanceName) => {
      await ensureInstanceHydrated(instanceName, {
        force: true,
        background: false,
      });
    },
    [ensureInstanceHydrated],
  );

  const invalidateInstance = useCallback<
    ModIntelligenceContextValue["invalidateInstance"]
  >(
    (instanceName, reason) => {
      const normalizedName = String(instanceName || "").trim();
      if (!normalizedName) return;
      const previous = snapshotsRef.current.get(normalizedName) || buildInitialSnapshot();
      setSnapshot(normalizedName, {
        ...previous,
        status: "idle",
        lipSyncStatus: "idle",
        lipSyncError: "",
        updatedAt: 0,
        error: reason ? String(reason) : "",
      });
    },
    [setSnapshot],
  );

  const getInstanceSnapshot = useCallback<
    ModIntelligenceContextValue["getInstanceSnapshot"]
  >((instanceName) => {
    const normalizedName = String(instanceName || "").trim();
    if (!normalizedName) return null;
    return snapshotsRef.current.get(normalizedName) || null;
  }, []);

  const ensurePackageInstallState = useCallback<
    ModIntelligenceContextValue["ensurePackageInstallState"]
  >(
    async (instanceName, identifier) => {
      const normalizedName = String(instanceName || "").trim();
      const rawIdentifier = String(identifier || "").trim();
      const lookupKey = normalizeIdentifierLookupKey(rawIdentifier);
      if (!normalizedName || !rawIdentifier || !lookupKey) {
        return emptyInstallState();
      }

      const snapshot = snapshotsRef.current.get(normalizedName);
      if (snapshot) {
        const fromSnapshot = findInstallStateByLookupKey(
          snapshot.lipInstallStateByIdentifier,
          lookupKey,
        );
        if (fromSnapshot) return fromSnapshot;
      }

      const cacheKey = `${normalizedName}::${lookupKey}`;
      const cached = packageStateCacheRef.current.get(cacheKey);
      if (cached && Date.now() - cached.at < PACKAGE_STATE_CACHE_TTL_MS) {
        return cached.state;
      }

      const state = await queryInstallState(normalizedName, rawIdentifier);
      packageStateCacheRef.current.set(cacheKey, { at: Date.now(), state });

      if (snapshot) {
        const nextInstallMap = new Map(snapshot.lipInstallStateByIdentifier);
        const existingKey = findInstallStateKeyByLookupKey(nextInstallMap, lookupKey);
        nextInstallMap.set(existingKey || rawIdentifier, state);
        setSnapshot(normalizedName, {
          ...snapshot,
          lipInstallStateByIdentifier: nextInstallMap,
        });
      }

      return state;
    },
    [queryInstallState, setSnapshot],
  );

  useEffect(() => {
    if (!currentVersionName) return;
    void ensureInstanceHydrated(currentVersionName, { background: true });
  }, [currentVersionName, lipSourceLoaded, ensureInstanceHydrated]);

  useEffect(() => {
    const off = Events.On("lip_task_finished", (event: unknown) => {
      const maybeEnvelope = event as { data?: unknown };
      const payload =
        maybeEnvelope && typeof maybeEnvelope === "object" && "data" in maybeEnvelope
          ? (maybeEnvelope.data as any)
          : (event as any);
      if (!payload || typeof payload !== "object") return;

      const success = Boolean((payload as any).success);
      if (!success) return;

      const target = String((payload as any).target || "").trim();
      if (target) {
        void refreshInstance(target, "lip-task-finished");
        return;
      }

      if (currentVersionName) {
        void refreshInstance(currentVersionName, "lip-task-finished-current");
      }
    });

    return () => {
      try {
        off && off();
      } catch {}
    };
  }, [currentVersionName, refreshInstance]);

  const value = useMemo<ModIntelligenceContextValue>(
    () => ({
      lipSourceLoaded,
      snapshotRevision,
      ensureInstanceHydrated,
      getInstanceSnapshot,
      ensurePackageInstallState,
      invalidateInstance,
      refreshInstance,
    }),
    [
      lipSourceLoaded,
      snapshotRevision,
      ensureInstanceHydrated,
      getInstanceSnapshot,
      ensurePackageInstallState,
      invalidateInstance,
      refreshInstance,
    ],
  );

  return (
    <ModIntelligenceContext.Provider value={value}>
      {children}
    </ModIntelligenceContext.Provider>
  );
};

export const useModIntelligence = (): ModIntelligenceContextValue => {
  const context = useContext(ModIntelligenceContext);
  if (!context) {
    throw new Error(
      "useModIntelligence must be used within ModIntelligenceProvider",
    );
  }
  return context;
};
