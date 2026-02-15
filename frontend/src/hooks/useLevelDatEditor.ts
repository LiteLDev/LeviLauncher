import React from "react";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";

export interface TypedField {
  name: string;
  tag: string;
  valueString?: string;
  valueJSON?: string;
}

export const ENUM_OPTIONS: Record<
  string,
  Array<{ value: string; label: string }>
> = {
  XBLBroadcastIntent: [
    { value: "0", label: "NoMultiPlay" },
    { value: "1", label: "InviteOnly" },
    { value: "2", label: "FriendsOnly" },
    { value: "3", label: "FriendsOfFriends" },
    { value: "4", label: "Public" },
  ],
  GamePublishSetting: [
    { value: "0", label: "NoMultiPlay" },
    { value: "1", label: "InviteOnly" },
    { value: "2", label: "FriendsOnly" },
    { value: "3", label: "FriendsOfFriends" },
    { value: "4", label: "Public" },
  ],
  PlatformBroadcastIntent: [
    { value: "0", label: "NoMultiPlay" },
    { value: "1", label: "InviteOnly" },
    { value: "2", label: "FriendsOnly" },
    { value: "3", label: "FriendsOfFriends" },
    { value: "4", label: "Public" },
  ],
  WorldVersion: [
    { value: "0", label: "Pre_1_18" },
    { value: "1", label: "Post_1_18" },
  ],
  StorageVersion: [
    { value: "0", label: "Unknown" },
    { value: "1", label: "OldV1" },
    { value: "2", label: "OldV2" },
    { value: "3", label: "OldV3" },
    { value: "4", label: "LevelDB1" },
    { value: "5", label: "LevelDBSubChunks" },
    { value: "6", label: "LevelDBSubChunkRawZip" },
    { value: "7", label: "LevelDBPaletted1" },
    { value: "8", label: "LevelDBPalettedMultiBlockStorage" },
    { value: "9", label: "LevelDataUpgradedSeed" },
    { value: "10", label: "LevelDataStrictSize" },
  ],
  Generator: [
    { value: "0", label: "Legacy" },
    { value: "1", label: "Overworld" },
    { value: "2", label: "Flat" },
    { value: "3", label: "Nether" },
    { value: "4", label: "TheEnd" },
    { value: "5", label: "Void" },
  ],
  GeneratorType: [
    { value: "0", label: "Legacy" },
    { value: "1", label: "Overworld" },
    { value: "2", label: "Flat" },
    { value: "3", label: "Nether" },
    { value: "4", label: "TheEnd" },
    { value: "5", label: "Void" },
  ],
  GameType: [
    { value: "-1", label: "Undefined" },
    { value: "0", label: "Survival/WorldDefault" },
    { value: "1", label: "Creative" },
    { value: "2", label: "Adventure" },
    { value: "5", label: "Default" },
    { value: "6", label: "Spectator" },
  ],
  Difficulty: [
    { value: "0", label: "Peaceful" },
    { value: "1", label: "Easy" },
    { value: "2", label: "Normal" },
    { value: "3", label: "Hard" },
  ],
  editorWorldType: [
    { value: "0", label: "NonEditor" },
    { value: "1", label: "EditorProject" },
    { value: "2", label: "EditorTestLevel" },
    { value: "3", label: "EditorRealmsUpload" },
  ],
  eduOffer: [
    { value: "0", label: "None" },
    { value: "1", label: "RestOfWorld" },
    { value: "2", label: "China_Deprecated" },
  ],
  permissionsLevel: [
    { value: "0", label: "Any" },
    { value: "1", label: "GameDirectors" },
    { value: "2", label: "Admin" },
    { value: "3", label: "Host" },
    { value: "4", label: "Owner" },
    { value: "5", label: "Internal" },
  ],
  playerPermissionsLevel: [
    { value: "0", label: "Visitor" },
    { value: "1", label: "Member" },
    { value: "2", label: "Operator" },
    { value: "3", label: "Custom" },
  ],
  daylightCycle: [
    { value: "0", label: "Normal" },
    { value: "1", label: "AlwaysDay" },
    { value: "2", label: "LockTime" },
  ],
};

export const TAG_OPTIONS = [
  "string",
  "byte",
  "short",
  "int",
  "long",
  "float",
  "double",
  "list",
  "compound",
];

export const normTag = (s: any) => String(s || "").toLowerCase();

export const getEnumOpts = (name: string) =>
  ENUM_OPTIONS[String(name || "")] || null;

export const parseListJSON = (s: string): any[] => {
  try {
    const v = JSON.parse(String(s || ""));
    return Array.isArray(v) ? v : [];
  } catch {
    return [];
  }
};

export const stringifyList = (arr: any[]): string => {
  try {
    return JSON.stringify(arr);
  } catch {
    return "[]";
  }
};

const nameCmp = (a: string, b: string) => {
  const g = (s: string) => {
    const c = s.charCodeAt(0) || 0;
    if (c >= 65 && c <= 90) return 0;
    if (c >= 97 && c <= 122) return 1;
    return 2;
  };
  const ga = g(String(a || ""));
  const gb = g(String(b || ""));
  if (ga !== gb) return ga - gb;
  if (a < b) return -1;
  if (a > b) return 1;
  return 0;
};

const sanitizeJSON = (arr: Array<any>) =>
  arr.map((it) => {
    const tag = String(it?.tag || "").toLowerCase();
    if (tag === "list") {
      const v = String(it?.valueJSON || "").trim();
      if (!v) return { ...it, valueJSON: "[]" };
    } else if (tag === "compound") {
      const v = String(it?.valueJSON || "").trim();
      if (!v) return { ...it, valueJSON: "{}" };
    }
    return it;
  });

export const useLevelDatEditor = (worldPath: string) => {
  const hasBackend = minecraft !== undefined;
  const [loading, setLoading] = React.useState<boolean>(true);
  const [error, setError] = React.useState<string>("");
  const [levelName, setLevelName] = React.useState<string>("");
  const [saving, setSaving] = React.useState<boolean>(false);
  const [typedVersion, setTypedVersion] = React.useState<number>(0);
  const [typedFields, setTypedFields] = React.useState<TypedField[]>([]);
  const [compoundOpen, setCompoundOpen] = React.useState<
    Record<string, boolean>
  >({});
  const [compoundFields, setCompoundFields] = React.useState<
    Record<string, TypedField[]>
  >({});
  const [topOrder, setTopOrder] = React.useState<string[]>([]);
  const [compoundOrders, setCompoundOrders] = React.useState<
    Record<string, string[]>
  >({});
  const [typedDrafts, setTypedDrafts] = React.useState<Record<string, string>>(
    {},
  );
  const [filterText, setFilterText] = React.useState<string>("");

  // Add field state
  const [addTargetKey, setAddTargetKey] = React.useState<string>("root");
  const [newUnifiedField, setNewUnifiedField] = React.useState<{
    name: string;
    tag: string;
    value: string;
  }>({ name: "", tag: "string", value: "" });
  const [addOpen, setAddOpen] = React.useState<boolean>(false);

  // Scroll position management
  const scrollRef = React.useRef<HTMLDivElement | null>(null);
  const lastScrollTopRef = React.useRef<number>(0);
  const restorePendingRef = React.useRef<boolean>(false);

  const beforeUpdate = React.useCallback(() => {
    try {
      lastScrollTopRef.current = scrollRef.current
        ? scrollRef.current.scrollTop
        : window.scrollY || 0;
      restorePendingRef.current = true;
    } catch {}
  }, []);

  React.useLayoutEffect(() => {
    if (!restorePendingRef.current) return;
    requestAnimationFrame(() => {
      try {
        if (scrollRef.current)
          scrollRef.current.scrollTop = lastScrollTopRef.current;
        else window.scrollTo({ top: lastScrollTopRef.current });
      } catch {}
    });
    restorePendingRef.current = false;
  }, [typedFields, compoundFields, compoundOpen, filterText]);

  // Computed fields
  const orderedTopFields = React.useMemo(() => {
    if (!Array.isArray(topOrder) || topOrder.length === 0) {
      const arr = typedFields.slice();
      arr.sort((x, y) => nameCmp(String(x?.name || ""), String(y?.name || "")));
      return arr;
    }
    const pos: Record<string, number> = {};
    topOrder.forEach((n, i) => {
      pos[String(n)] = i;
    });
    const withPos = typedFields.map((f, i) => ({
      f,
      i,
      p: pos[String(f?.name || "")] ?? 100000 + i,
    }));
    withPos.sort((a, b) => a.p - b.p);
    return withPos.map((x) => x.f);
  }, [typedFields, topOrder]);

  const compoundTargetKeys = React.useMemo(() => {
    const names = typedFields
      .filter((f) => normTag(f.tag) === "compound")
      .map((f) => String(f.name || ""));
    const loaded = Object.keys(compoundFields || {});
    const set = new Set<string>(["root", ...names, ...loaded]);
    return Array.from(set);
  }, [typedFields, compoundFields]);

  const matchesName = React.useCallback(
    (s: any) => {
      const q = String(filterText || "")
        .trim()
        .toLowerCase();
      if (!q) return true;
      return String(s || "")
        .toLowerCase()
        .includes(q);
    },
    [filterText],
  );

  // Field operations
  const setTypedFieldValueByName = React.useCallback(
    (
      name: string,
      patch: Partial<{ valueString?: string; valueJSON?: string }>,
    ) => {
      beforeUpdate();
      setTypedFields((prev) => {
        const next = prev.slice();
        const idx = next.findIndex(
          (x: any) => String(x?.name || "") === String(name || ""),
        );
        if (idx >= 0) next[idx] = { ...next[idx], ...patch } as any;
        return next;
      });
    },
    [beforeUpdate],
  );

  const setCompoundFieldValue = React.useCallback(
    (
      parentPathKey: string,
      idx: number,
      patch: Partial<{ valueString?: string; valueJSON?: string }>,
    ) => {
      beforeUpdate();
      setCompoundFields((prev) => {
        const list = prev[parentPathKey] ? prev[parentPathKey].slice() : [];
        if (idx >= 0 && idx < list.length)
          list[idx] = { ...list[idx], ...patch } as any;
        return { ...prev, [parentPathKey]: list };
      });
    },
    [beforeUpdate],
  );

  const loadCompound = React.useCallback(
    async (nameOrPath: string | string[]) => {
      try {
        const path = Array.isArray(nameOrPath) ? nameOrPath : [nameOrPath];
        const key = path.join("/");
        const res = await (minecraft as any)?.ReadWorldLevelDatFieldsAt?.(
          worldPath,
          path,
        );
        const remote = Array.isArray(res?.fields) ? res.fields : [];
        setCompoundFields((prev) => {
          const local = prev[key] ? prev[key].slice() : [];
          const localMap = new Map<string, any>();
          for (const it of local) localMap.set(String(it?.name || ""), it);
          const used = new Set<string>();
          const merged: Array<any> = remote.map((r: any) => {
            const nm = String(r?.name || "");
            if (localMap.has(nm)) {
              used.add(nm);
              const lv = localMap.get(nm);
              return { ...r, ...lv };
            }
            return r;
          });
          for (const it of local) {
            const nm = String(it?.name || "");
            if (!used.has(nm)) merged.push(it);
          }
          return { ...prev, [key]: merged };
        });
        const ord = Array.isArray(res?.order) ? (res.order as string[]) : [];
        if (ord.length > 0)
          setCompoundOrders((prev) => ({ ...prev, [key]: ord }));
        setCompoundOpen((prev) => ({ ...prev, [key]: true }));
      } catch {}
    },
    [worldPath],
  );

  const addField = React.useCallback(() => {
    const nm = String(newUnifiedField.name || "").trim();
    const tg = String(newUnifiedField.tag || "string").trim();
    if (!nm) return;

    const isSimpleType =
      tg === "string" ||
      tg === "byte" ||
      tg === "short" ||
      tg === "int" ||
      tg === "long" ||
      tg === "float" ||
      tg === "double";

    if (addTargetKey === "root") {
      setTypedFields((prev) => {
        if (prev.some((f) => String(f.name || "") === nm)) return prev;
        const it: any = { name: nm, tag: tg };
        if (isSimpleType) it.valueString = String(newUnifiedField.value || "");
        else it.valueJSON = String(newUnifiedField.value || "");
        return [...prev, it];
      });
    } else {
      setCompoundFields((prev) => {
        const list2 = prev[addTargetKey] ? prev[addTargetKey].slice() : [];
        if (list2.some((x) => String(x.name || "") === nm)) return prev;
        const it: any = { name: nm, tag: tg };
        if (isSimpleType) it.valueString = String(newUnifiedField.value || "");
        else it.valueJSON = String(newUnifiedField.value || "");
        return { ...prev, [addTargetKey]: [...list2, it] };
      });
      setCompoundOpen((prev) => ({ ...prev, [addTargetKey]: true }));
    }
    setNewUnifiedField({ name: "", tag: "string", value: "" });
  }, [newUnifiedField, addTargetKey]);

  // Load data
  const load = React.useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      if (!hasBackend || !worldPath) {
        setError("contentpage.error_resolve_paths");
        return;
      }
      const res2 = await (minecraft as any)?.ReadWorldLevelDatFields?.(
        worldPath,
      );
      const v2 = Number(res2?.version || 0);
      const fields2 = Array.isArray(res2?.fields) ? res2.fields : [];
      setTypedVersion(v2);
      setTypedFields(fields2);
      const ord = Array.isArray(res2?.order) ? (res2.order as string[]) : [];
      setTopOrder(ord);
      try {
        const txt = await (minecraft as any)?.GetWorldLevelName?.(worldPath);
        setLevelName(String(txt || ""));
      } catch {}
    } catch {
      setError("common.load_failed");
    } finally {
      setLoading(false);
    }
  }, [hasBackend, worldPath]);

  React.useEffect(() => {
    load();
  }, [load]);

  // Save all
  const saveAll = React.useCallback(async () => {
    if (!hasBackend || !worldPath) return false;
    setSaving(true);
    setError("");
    try {
      // Flush drafts
      const drafts = { ...typedDrafts };
      if (Object.keys(drafts).length > 0) {
        Object.keys(drafts).forEach((dk) => {
          const val = String(drafts[dk] ?? "");
          if (dk.startsWith("tf:")) {
            const name = dk.slice(3);
            setTypedFieldValueByName(name, { valueString: val });
          } else if (dk.startsWith("cf:")) {
            const rest = dk.slice(3);
            const p = rest.split(":");
            if (p.length >= 2) {
              const parentPathKey = p[0];
              const childName = p.slice(1).join(":");
              setCompoundFields((prev) => {
                const list = prev[parentPathKey]
                  ? prev[parentPathKey].slice()
                  : [];
                const idx = list.findIndex(
                  (x) => String(x.name || "") === childName,
                );
                if (idx >= 0)
                  list[idx] = { ...list[idx], valueString: val } as any;
                return { ...prev, [parentPathKey]: list };
              });
            }
          } else if (dk.startsWith("cfjson:")) {
            const rest = dk.slice(7);
            const p = rest.split(":");
            if (p.length >= 2) {
              const parentPathKey = p[0];
              const childName = p.slice(1).join(":");
              setCompoundFields((prev) => {
                const list = prev[parentPathKey]
                  ? prev[parentPathKey].slice()
                  : [];
                const idx = list.findIndex(
                  (x) => String(x.name || "") === childName,
                );
                if (idx >= 0)
                  list[idx] = { ...list[idx], valueJSON: val } as any;
                return { ...prev, [parentPathKey]: list };
              });
            }
          } else if (dk.startsWith("tflist:")) {
            const name = dk.slice(7);
            setTypedFieldValueByName(name, { valueJSON: val });
          } else if (dk.startsWith("cflist:")) {
            const rest = dk.slice(7);
            const p = rest.split(":");
            if (p.length >= 2) {
              const parentPathKey = p[0];
              const childName = p.slice(1).join(":");
              setCompoundFields((prev) => {
                const list = prev[parentPathKey]
                  ? prev[parentPathKey].slice()
                  : [];
                const idx = list.findIndex(
                  (x) => String(x.name || "") === childName,
                );
                if (idx >= 0)
                  list[idx] = { ...list[idx], valueJSON: val } as any;
                return { ...prev, [parentPathKey]: list };
              });
            }
          }
        });
        setTypedDrafts({});
      }

      const typedFieldsSafe = sanitizeJSON(typedFields);
      const err2 = await (minecraft as any)?.SetWorldLevelName?.(
        worldPath,
        levelName,
      );
      const err3 = await (minecraft as any)?.WriteWorldLevelDatFields?.(
        worldPath,
        { version: typedVersion || 0, fields: typedFieldsSafe, levelName },
      );
      let err4 = "";
      const entries = Object.entries(compoundFields);
      for (const [pathKey, list] of entries) {
        const erx = await (minecraft as any)?.WriteWorldLevelDatFieldsAt?.(
          worldPath,
          {
            version: typedVersion || 0,
            path: pathKey.split("/"),
            fields: sanitizeJSON(list),
          },
        );
        if (erx) err4 = erx;
      }
      if (err2 || err3 || err4) {
        setError("common.save_failed");
        return false;
      }
      return true;
    } catch {
      setError("common.save_failed");
      return false;
    } finally {
      setSaving(false);
    }
  }, [
    hasBackend,
    worldPath,
    typedDrafts,
    typedFields,
    levelName,
    typedVersion,
    compoundFields,
    setTypedFieldValueByName,
  ]);

  const getOrderedCompoundChildren = React.useCallback(
    (pathKey: string) => {
      const listRaw = compoundFields[pathKey] || [];
      const ord = compoundOrders[pathKey] || [];
      if (!Array.isArray(ord) || ord.length === 0) {
        const tmp = listRaw.slice();
        tmp.sort((a, b) =>
          nameCmp(String(a?.name || ""), String(b?.name || "")),
        );
        return tmp;
      }
      const pos: Record<string, number> = {};
      ord.forEach((n, i2) => {
        pos[String(n)] = i2;
      });
      const withPos = listRaw.map((f2, i2) => ({
        f2,
        i2,
        p: pos[String(f2?.name || "")] ?? 100000 + i2,
      }));
      withPos.sort((a, b) => a.p - b.p);
      return withPos.map((x) => x.f2);
    },
    [compoundFields, compoundOrders],
  );

  return {
    // State
    loading,
    error,
    setError,
    levelName,
    setLevelName,
    saving,
    typedVersion,
    typedFields,
    compoundOpen,
    setCompoundOpen,
    compoundFields,
    typedDrafts,
    setTypedDrafts,
    filterText,
    setFilterText,
    addTargetKey,
    setAddTargetKey,
    newUnifiedField,
    setNewUnifiedField,
    addOpen,
    setAddOpen,
    scrollRef,
    hasBackend,

    // Computed
    orderedTopFields,
    compoundTargetKeys,

    // Operations
    beforeUpdate,
    matchesName,
    setTypedFieldValueByName,
    setCompoundFieldValue,
    loadCompound,
    addField,
    load,
    saveAll,
    getOrderedCompoundChildren,
  };
};
