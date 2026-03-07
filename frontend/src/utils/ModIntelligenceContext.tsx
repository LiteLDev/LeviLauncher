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
  type CandidateLIPIdentifier,
  filterVisibleMods,
  parseLIPPackageInstallState,
  type LIPPackageInstallState,
  type LipGroupItem,
  type ModLIPState,
  type ModListItem,
} from "@/utils/modIntelligenceResolver";

export type InstanceModSnapshot = {
  status: "idle" | "loading" | "ready" | "refreshing" | "error";
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

const ModIntelligenceContext = createContext<ModIntelligenceContextValue | undefined>(
  undefined,
);

const INSTANCE_CACHE_TTL_MS = 30_000;
const PACKAGE_STATE_CACHE_TTL_MS = 30_000;
const LEVILAMINA_IDENTIFIER = "LiteLDev/LeviLamina";
const normalizeIdentifierKey = (value: string): string =>
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

const buildInitialSnapshot = (): InstanceModSnapshot => ({
  status: "idle",
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

export const ModIntelligenceProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const { currentVersionName } = useCurrentVersion();
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

  const queryInstallState = useCallback(
    async (instanceName: string, identifier: string): Promise<LIPPackageInstallState> => {
      const normalizedName = String(instanceName || "").trim();
      const normalizedIdentifier = String(identifier || "").trim();
      if (!normalizedName || !normalizedIdentifier) {
        return emptyInstallState();
      }

      try {
        const raw = await Call.ByName(
          "main.Minecraft.GetLIPPackageInstallState",
          normalizedName,
          normalizedIdentifier,
        );
        return parseLIPPackageInstallState(raw);
      } catch (error) {
        return {
          installed: false,
          explicitInstalled: false,
          installedVersion: "",
          error: parseErrorCode(error) || "ERR_LIP_PACKAGE_QUERY_FAILED",
        };
      }
    },
    [],
  );

  const resolveIdentifierForRequest = useCallback(
    (identifier: string, identifierKey?: string): string => {
      const rawIdentifier = String(identifier || "").trim();
      const normalizedIdentifier = normalizeIdentifierKey(
        identifierKey || rawIdentifier,
      );
      if (!normalizedIdentifier) return rawIdentifier;
      const fromIndex = String(
        lipPackageByIdentifierRef.current[normalizedIdentifier]?.identifier || "",
      ).trim();
      if (fromIndex) return fromIndex;
      return rawIdentifier;
    },
    [],
  );

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

  useEffect(() => {
    let cancelled = false;

    const loadLIPSource = async () => {
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

          const normalizedIdentifier = String(pkg?.identifier || "")
            .trim()
            .toLowerCase();
          if (normalizedIdentifier && !groupedByIdentifier[normalizedIdentifier]) {
            groupedByIdentifier[normalizedIdentifier] = pkg;
          }
        }

        lipPackagesByNameRef.current = groupedByName;
        lipPackageByIdentifierRef.current = groupedByIdentifier;
        selfVariantCandidatesRef.current = buildSelfVariantCandidates(
          Array.isArray(relations) ? relations : [],
        );
      } catch {
        if (cancelled) return;
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
  }, []);

  const ensureInstanceHydrated = useCallback<
    ModIntelligenceContextValue["ensureInstanceHydrated"]
  >(
    async (instanceName, opts) => {
      const normalizedName = String(instanceName || "").trim();
      if (!normalizedName) return;
      if (!lipSourceLoaded) return;

      const force = opts?.force === true;
      const now = Date.now();
      const existing = snapshotsRef.current.get(normalizedName);
      if (
        !force &&
        existing &&
        existing.status === "ready" &&
        now - existing.updatedAt < INSTANCE_CACHE_TTL_MS
      ) {
        return;
      }

      const pending = inFlightRef.current.get(normalizedName);
      if (pending) return pending;

      const run = (async () => {
        const prev = snapshotsRef.current.get(normalizedName) || buildInitialSnapshot();
        setSnapshot(normalizedName, {
          ...prev,
          status: prev.updatedAt > 0 ? "refreshing" : "loading",
          error: "",
        });

        try {
          const mods = filterVisibleMods(await GetMods(normalizedName));

          const enabledEntries = await Promise.all(
            mods.map(async (mod) => {
              const folder = String((mod as any)?.folder || "").trim() || String(mod?.name || "").trim();
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

          const modLIPStateByFolder = buildModLIPStateByFolder({
            modsInfo: mods,
            lipSourceLoaded: true,
            selfVariantCandidates: selfVariantCandidatesRef.current,
            lipPackagesByName: lipPackagesByNameRef.current,
            lipPackageByIdentifier: lipPackageByIdentifierRef.current,
          });

          const identifiers = collectCandidateLIPIdentifiers(modLIPStateByFolder);
          const installEntries = await Promise.all(
            identifiers.map(async (item: CandidateLIPIdentifier) => {
              const state = await queryInstallState(
                normalizedName,
                resolveIdentifierForRequest(item.identifier, item.identifierKey),
              );
              packageStateCacheRef.current.set(
                `${normalizedName}::${item.identifierKey}`,
                { at: Date.now(), state },
              );
              return [item.identifierKey, state] as const;
            }),
          );
          const lipInstallStateByIdentifier = new Map<string, LIPPackageInstallState>(
            installEntries,
          );

          const llState = await queryInstallState(normalizedName, LEVILAMINA_IDENTIFIER);

          const { lipGroupItems, normalItems } = buildListItems({
            modsInfo: mods,
            modLIPStateByFolder,
            lipInstallStateByIdentifier,
            enabledByFolder,
            sortDirection: "asc",
          });

          setSnapshot(normalizedName, {
            status: "ready",
            modsInfo: mods,
            enabledByFolder,
            modLIPStateByFolder,
            lipGroupItems,
            normalItems,
            lipInstallStateByIdentifier,
            llState,
            updatedAt: Date.now(),
            error: "",
          });
        } catch (error) {
          const previous = snapshotsRef.current.get(normalizedName) || buildInitialSnapshot();
          setSnapshot(normalizedName, {
            ...previous,
            status: "error",
            error: parseErrorCode(error) || "ERR_MOD_INTELLIGENCE_REFRESH_FAILED",
            updatedAt: previous.updatedAt || Date.now(),
          });
        } finally {
          inFlightRef.current.delete(normalizedName);
        }
      })();

      inFlightRef.current.set(normalizedName, run);
      return run;
    },
    [lipSourceLoaded, queryInstallState, resolveIdentifierForRequest, setSnapshot],
  );

  const refreshInstance = useCallback<ModIntelligenceContextValue["refreshInstance"]>(
    async (instanceName) => {
      await ensureInstanceHydrated(instanceName, { force: true, background: false });
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
      const normalizedIdentifier = normalizeIdentifierKey(rawIdentifier);
      if (!normalizedName || !normalizedIdentifier) return emptyInstallState();

      const snapshot = snapshotsRef.current.get(normalizedName);
      if (snapshot) {
        const fromSnapshot = snapshot.lipInstallStateByIdentifier.get(normalizedIdentifier);
        if (fromSnapshot) return fromSnapshot;
      }

      const cacheKey = `${normalizedName}::${normalizedIdentifier}`;
      const cached = packageStateCacheRef.current.get(cacheKey);
      if (cached && Date.now() - cached.at < PACKAGE_STATE_CACHE_TTL_MS) {
        return cached.state;
      }

      const state = await queryInstallState(
        normalizedName,
        resolveIdentifierForRequest(rawIdentifier, normalizedIdentifier),
      );
      packageStateCacheRef.current.set(cacheKey, { at: Date.now(), state });

      if (snapshot) {
        const nextInstallMap = new Map(snapshot.lipInstallStateByIdentifier);
        nextInstallMap.set(normalizedIdentifier, state);
        setSnapshot(normalizedName, {
          ...snapshot,
          lipInstallStateByIdentifier: nextInstallMap,
        });
      }

      return state;
    },
    [queryInstallState, resolveIdentifierForRequest, setSnapshot],
  );

  useEffect(() => {
    if (!currentVersionName || !lipSourceLoaded) return;
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
