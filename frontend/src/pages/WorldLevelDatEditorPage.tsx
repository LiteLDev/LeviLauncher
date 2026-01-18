import React from "react";
import { useTranslation } from "react-i18next";
import {
  Button,
  Card,
  CardBody,
  CardHeader,
  Input,
  Spinner,
  Switch,
  Select,
  SelectItem,
  Chip,
  Tooltip,
} from "@heroui/react";
import { motion, AnimatePresence } from "framer-motion";
import { useLocation, useNavigate } from "react-router-dom";
import {
  FaArrowLeft,
  FaSave,
  FaSync,
  FaPlus,
  FaTimes,
  FaSearch,
} from "react-icons/fa";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { PageHeader } from "@/components/PageHeader";

export default function WorldLevelDatEditorPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const loc = useLocation();
  const hasBackend = minecraft !== undefined;
  const sp = new URLSearchParams(String(loc?.search || ""));
  const worldPath = sp.get("path") || "";
  const [loading, setLoading] = React.useState<boolean>(true);
  const [error, setError] = React.useState<string>("");
  const [levelName, setLevelName] = React.useState<string>("");
  const [saving, setSaving] = React.useState<boolean>(false);
  const [typedVersion, setTypedVersion] = React.useState<number>(0);
  const [typedFields, setTypedFields] = React.useState<
    Array<{
      name: string;
      tag: string;
      valueString?: string;
      valueJSON?: string;
    }>
  >([]);
  const [compoundOpen, setCompoundOpen] = React.useState<
    Record<string, boolean>
  >({});
  const [compoundFields, setCompoundFields] = React.useState<
    Record<
      string,
      Array<{
        name: string;
        tag: string;
        valueString?: string;
        valueJSON?: string;
      }>
    >
  >({});
  const [topOrder, setTopOrder] = React.useState<string[]>([]);
  const [compoundOrders, setCompoundOrders] = React.useState<
    Record<string, string[]>
  >({});
  const [typedDrafts, setTypedDrafts] = React.useState<Record<string, string>>(
    {},
  );
  const [currentAddParent, setCurrentAddParent] = React.useState<string>("");
  const [newChildField, setNewChildField] = React.useState<{
    name: string;
    tag: string;
    value: string;
  }>({ name: "", tag: "string", value: "" });
  const [newTopField, setNewTopField] = React.useState<{
    name: string;
    tag: string;
    value: string;
  }>({ name: "", tag: "string", value: "" });
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

  const normTag = (s: any) => String(s || "").toLowerCase();
  const nameCmp = React.useCallback((a: string, b: string) => {
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
  }, []);
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
  const nonCompoundFields = React.useMemo(
    () => orderedTopFields.filter((f) => normTag(f.tag) !== "compound"),
    [orderedTopFields],
  );
  const compoundTopFields = React.useMemo(
    () => orderedTopFields.filter((f) => normTag(f.tag) === "compound"),
    [orderedTopFields],
  );

  const [filterText, setFilterText] = React.useState<string>("");
  const matchesName = (s: any) => {
    const q = String(filterText || "")
      .trim()
      .toLowerCase();
    if (!q) return true;
    return String(s || "")
      .toLowerCase()
      .includes(q);
  };

  const parseListJSON = (s: string): any[] => {
    try {
      const v = JSON.parse(String(s || ""));
      return Array.isArray(v) ? v : [];
    } catch {
      return [];
    }
  };
  const stringifyList = (arr: any[]): string => {
    try {
      return JSON.stringify(arr);
    } catch {
      return "[]";
    }
  };

  const enumOptions: Record<string, Array<{ value: string; label: string }>> = {
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
  const getEnumOpts = (name: string) => enumOptions[String(name || "")] || null;
  const tagOptions = [
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
  const [addTargetKey, setAddTargetKey] = React.useState<string>("root");
  const [newUnifiedField, setNewUnifiedField] = React.useState<{
    name: string;
    tag: string;
    value: string;
  }>({ name: "", tag: "string", value: "" });
  const [addOpen, setAddOpen] = React.useState<boolean>(false);
  const compoundTargetKeys = React.useMemo(() => {
    const names = typedFields
      .filter((f) => normTag(f.tag) === "compound")
      .map((f) => String(f.name || ""));
    const loaded = Object.keys(compoundFields || {});
    const set = new Set<string>(["root", ...names, ...loaded]);
    return Array.from(set);
  }, [typedFields, compoundFields]);

  const loadCompound = async (nameOrPath: string | string[]) => {
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
  };

  const addChildOnOpen = () => {
    const parent = String(currentAddParent || "").trim();
    if (!parent) return;
    setCompoundOpen((prev) => ({ ...prev, [parent]: true }));
  };

  const setCompoundFieldValue = (
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
  };

  const load = React.useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      if (!hasBackend || !worldPath) {
        setError(
          t("contentpage.error_resolve_paths", {
            defaultValue: "无法解析内容路径。",
          }) as string,
        );
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
      setError(t("common.load_failed", { defaultValue: "加载失败" }) as string);
    } finally {
      setLoading(false);
    }
  }, [hasBackend, worldPath]);

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

  React.useEffect(() => {
    load();
  }, [load]);

  const saveAll = async () => {
    if (!hasBackend || !worldPath) return;
    setSaving(true);
    setError("");
    try {
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
        setError(
          t("common.save_failed", { defaultValue: "保存失败" }) as string,
        );
      } else {
        navigate(-1);
      }
    } catch {
      setError(t("common.save_failed", { defaultValue: "保存失败" }) as string);
    } finally {
      setSaving(false);
    }
  };

  const setTypedFieldValueByName = (
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
  };

  const FieldBox = React.useMemo(() => {
    const chipColor = (tp: string): any => {
      const t = String(tp || "").toLowerCase();
      if (t === "compound") return "secondary";
      if (t === "list") return "warning";
      if (t === "string") return "primary";
      if (
        t === "byte" ||
        t === "short" ||
        t === "int" ||
        t === "long" ||
        t === "float" ||
        t === "double"
      )
        return "success";
      if (t === "add") return "primary";
      return "default";
    };
    return function Box({
      title,
      type,
      children,
      delay = 0,
    }: {
      title: string;
      type: string;
      children: React.ReactNode;
      delay?: number;
    }) {
      return (
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3, delay: delay * 0.03 }}
          className="group relative overflow-hidden rounded-2xl border border-default-200 dark:border-default-100/10 bg-white/50 dark:bg-zinc-900/50 p-4 transition-all hover:bg-default-100 dark:hover:bg-zinc-800/50 hover:shadow-lg"
        >
          <div className="flex items-center justify-between mb-3">
            <div
              className="text-sm font-semibold text-default-700 dark:text-default-300 truncate"
              title={title}
            >
              {title}
            </div>
            <Chip
              size="sm"
              variant="flat"
              color={chipColor(type)}
              className="h-6 min-w-12 justify-center font-mono text-xs uppercase"
            >
              {type}
            </Chip>
          </div>
          <div className="relative z-10">{children}</div>
        </motion.div>
      );
    };
  }, []);

  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.25 }}
      className="w-full max-w-full mx-auto p-4 h-full flex flex-col"
    >
      <Card className="flex-1 min-h-0 border-none shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl">
        <CardBody className="p-0 flex flex-col h-full overflow-hidden">
          <div className="shrink-0 p-4 sm:p-6 pb-2 flex flex-col gap-4 border-b border-default-200 dark:border-white/10">
            <PageHeader
              title={t("contentpage.world_leveldat_editor", {
                defaultValue: "Level.dat Editor",
              })}
              startContent={
                <Button
                  isIconOnly
                  radius="full"
                  variant="light"
                  onPress={() => navigate(-1)}
                >
                  <FaArrowLeft size={20} />
                </Button>
              }
              endContent={
                <>
                  <Input
                    size="sm"
                    radius="full"
                    variant="flat"
                    placeholder={
                      t("common.search", { defaultValue: "搜索" }) as string
                    }
                    value={filterText}
                    onValueChange={(v) => {
                      beforeUpdate();
                      setFilterText(v);
                    }}
                    isClearable
                    startContent={<FaSearch className="text-default-400" />}
                    className="w-48 sm:w-64"
                    classNames={{
                      inputWrapper:
                        "bg-default-100 dark:bg-default-50/20 group-data-[focus=true]:bg-default-200/50",
                    }}
                  />
                  <Tooltip
                    content={t("common.refresh", { defaultValue: "刷新" })}
                  >
                    <Button
                      isIconOnly
                      radius="full"
                      variant="flat"
                      onPress={load}
                      isLoading={loading}
                      className="bg-default-100 dark:bg-default-50/20 text-default-600"
                    >
                      <FaSync className={loading ? "animate-spin" : ""} />
                    </Button>
                  </Tooltip>
                  <Tooltip content={t("common.save", { defaultValue: "保存" })}>
                    <Button
                      isIconOnly
                      radius="full"
                      color="primary"
                      onPress={saveAll}
                      isLoading={saving}
                      isDisabled={!hasBackend || loading}
                      className="bg-linear-to-r from-emerald-500 to-teal-500 text-white shadow-lg shadow-emerald-900/20"
                    >
                      <FaSave className="w-4 h-4" />
                    </Button>
                  </Tooltip>
                </>
              }
            />

            {error && (
              <div className="w-full p-4 rounded-2xl bg-danger-50 dark:bg-danger-900/20 border border-danger-200 dark:border-danger-800/50 text-danger flex items-center gap-2">
                <FaTimes className="w-4 h-4" />
                <span>{error}</span>
              </div>
            )}
          </div>
          <div
            ref={scrollRef}
            className="flex-1 overflow-y-auto pretty-scrollbar p-4 sm:p-6 pt-0"
          >
            {loading ? (
              <div className="flex flex-col items-center justify-center h-64 gap-4">
                <Spinner size="lg" color="success" />
                <div className="text-default-400 animate-pulse">
                  {t("common.loading", { defaultValue: "加载中..." })}
                </div>
              </div>
            ) : (
              <div className="flex flex-col gap-8">
                {/* Basic Info Section */}
                <motion.div
                  initial={{ opacity: 0, x: -20 }}
                  animate={{ opacity: 1, x: 0 }}
                  transition={{ duration: 0.4, delay: 0.1 }}
                  className="p-6 rounded-2xl bg-white/50 dark:bg-zinc-900/50 border border-default-200 dark:border-default-100/10 backdrop-blur-md shadow-sm"
                >
                  <div className="flex items-center gap-4 mb-6">
                    <div className="w-1 h-6 rounded-full bg-linear-to-b from-emerald-500 to-teal-500 shadow-[0_0_10px_rgba(16,185,129,0.4)]" />
                    <h3 className="text-lg font-bold text-default-700 dark:text-default-300">
                      {t("contentpage.basic_info", {
                        defaultValue: "基础信息",
                      })}
                    </h3>
                  </div>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                    <Input
                      label={t("contentpage.level_name", {
                        defaultValue: "世界名称",
                      })}
                      labelPlacement="outside"
                      placeholder="My World"
                      value={levelName}
                      onValueChange={(v) => {
                        beforeUpdate();
                        setLevelName(v);
                      }}
                      variant="flat"
                      radius="lg"
                      classNames={{
                        inputWrapper:
                          "bg-default-100 dark:bg-default-50/20 group-data-[focus=true]:bg-default-200/50",
                      }}
                    />
                    <div className="flex flex-col gap-2">
                      <label className="text-sm text-default-600">
                        {t("contentpage.version", { defaultValue: "版本" })}
                      </label>
                      <div className="h-10 px-3 flex items-center rounded-lg bg-default-100 dark:bg-default-50/20 text-default-500 text-sm font-mono border border-transparent">
                        {typedVersion}
                      </div>
                    </div>
                  </div>
                </motion.div>
                <div role="separator" className="h-px bg-default-200/50 my-2" />
                {/* Add Field Section */}
                <div className="flex flex-col gap-4">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-4">
                      <div className="w-1 h-6 rounded-full bg-linear-to-b from-emerald-500 to-teal-500 shadow-[0_0_10px_rgba(16,185,129,0.4)]" />
                      <h3 className="text-lg font-bold text-default-700 dark:text-default-300">
                        {t("contentpage.add_field", {
                          defaultValue: "新增字段",
                        })}
                      </h3>
                    </div>
                    <Button
                      size="sm"
                      radius="full"
                      variant="flat"
                      className="bg-default-100 dark:bg-default-50/20 text-default-600"
                      onPress={() => setAddOpen((o) => !o)}
                      startContent={addOpen ? <FaTimes /> : <FaPlus />}
                    >
                      {addOpen
                        ? t("common.collapse", { defaultValue: "收起" })
                        : t("common.expand", { defaultValue: "展开" })}
                    </Button>
                  </div>

                  <AnimatePresence>
                    {addOpen && (
                      <motion.div
                        initial={{ height: 0, opacity: 0 }}
                        animate={{ height: "auto", opacity: 1 }}
                        exit={{ height: 0, opacity: 0 }}
                        transition={{ duration: 0.3 }}
                        className="overflow-hidden"
                      >
                        <div className="p-6 rounded-2xl bg-white/50 dark:bg-zinc-900/50 border border-default-200 dark:border-default-100/10 backdrop-blur-md shadow-sm transition-all">
                          <div className="grid grid-cols-1 sm:grid-cols-[140px_1fr_100px_1fr_80px] gap-4 items-end">
                            <Select
                              label="Target"
                              labelPlacement="outside"
                              size="sm"
                              radius="lg"
                              variant="flat"
                              selectedKeys={new Set([addTargetKey])}
                              onSelectionChange={(keys: any) => {
                                const v = String(Array.from(keys)[0] || "root");
                                setAddTargetKey(v);
                              }}
                              classNames={{
                                trigger: "bg-default-100 dark:bg-default-50/20",
                              }}
                            >
                              {compoundTargetKeys.map((o) => (
                                <SelectItem key={o}>{o}</SelectItem>
                              ))}
                            </Select>
                            <Input
                              label="Name"
                              labelPlacement="outside"
                              size="sm"
                              radius="lg"
                              variant="flat"
                              placeholder={
                                t("contentpage.field_name", {
                                  defaultValue: "名称",
                                }) as string
                              }
                              value={newUnifiedField.name}
                              onValueChange={(v) =>
                                setNewUnifiedField((prev) => ({
                                  ...prev,
                                  name: v,
                                }))
                              }
                              classNames={{
                                inputWrapper:
                                  "bg-default-100 dark:bg-default-50/20",
                              }}
                            />
                            <Select
                              label="Type"
                              labelPlacement="outside"
                              size="sm"
                              radius="lg"
                              variant="flat"
                              selectedKeys={new Set([newUnifiedField.tag])}
                              onSelectionChange={(keys: any) => {
                                const v = String(
                                  Array.from(keys)[0] || "string",
                                );
                                setNewUnifiedField((prev) => ({
                                  ...prev,
                                  tag: v,
                                }));
                              }}
                              classNames={{
                                trigger: "bg-default-100 dark:bg-default-50/20",
                              }}
                            >
                              {tagOptions.map((o) => (
                                <SelectItem key={o}>{o}</SelectItem>
                              ))}
                            </Select>
                            <Input
                              label="Value"
                              labelPlacement="outside"
                              size="sm"
                              radius="lg"
                              variant="flat"
                              placeholder={
                                t("contentpage.initial_value", {
                                  defaultValue: "初始值",
                                }) as string
                              }
                              value={newUnifiedField.value}
                              onValueChange={(v) =>
                                setNewUnifiedField((prev) => ({
                                  ...prev,
                                  value: v,
                                }))
                              }
                              classNames={{
                                inputWrapper:
                                  "bg-default-100 dark:bg-default-50/20",
                              }}
                            />
                            <Button
                              size="sm"
                              radius="lg"
                              className="bg-linear-to-r from-emerald-500 to-teal-500 text-white shadow-lg shadow-emerald-900/20"
                              onPress={() => {
                                const nm = String(
                                  newUnifiedField.name || "",
                                ).trim();
                                const tg = String(
                                  newUnifiedField.tag || "string",
                                ).trim();
                                if (!nm) return;
                                if (addTargetKey === "root") {
                                  setTypedFields((prev) => {
                                    if (
                                      prev.some(
                                        (f) => String(f.name || "") === nm,
                                      )
                                    )
                                      return prev;
                                    const it: any = { name: nm, tag: tg };
                                    if (
                                      tg === "string" ||
                                      tg === "byte" ||
                                      tg === "short" ||
                                      tg === "int" ||
                                      tg === "long" ||
                                      tg === "float" ||
                                      tg === "double"
                                    )
                                      it.valueString = String(
                                        newUnifiedField.value || "",
                                      );
                                    else
                                      it.valueJSON = String(
                                        newUnifiedField.value || "",
                                      );
                                    const next = prev.slice();
                                    next.push(it);
                                    return next;
                                  });
                                } else {
                                  setCompoundFields((prev) => {
                                    const list2 = prev[addTargetKey]
                                      ? prev[addTargetKey].slice()
                                      : [];
                                    if (
                                      list2.some(
                                        (x) => String(x.name || "") === nm,
                                      )
                                    )
                                      return prev;
                                    const it: any = { name: nm, tag: tg };
                                    if (
                                      tg === "string" ||
                                      tg === "byte" ||
                                      tg === "short" ||
                                      tg === "int" ||
                                      tg === "long" ||
                                      tg === "float" ||
                                      tg === "double"
                                    )
                                      it.valueString = String(
                                        newUnifiedField.value || "",
                                      );
                                    else
                                      it.valueJSON = String(
                                        newUnifiedField.value || "",
                                      );
                                    const next = {
                                      ...prev,
                                      [addTargetKey]: [...list2, it],
                                    };
                                    return next;
                                  });
                                  setCompoundOpen((prev) => ({
                                    ...prev,
                                    [addTargetKey]: true,
                                  }));
                                }
                                setNewUnifiedField({
                                  name: "",
                                  tag: "string",
                                  value: "",
                                });
                              }}
                            >
                              {t("common.add", { defaultValue: "添加" })}
                            </Button>
                          </div>
                        </div>
                      </motion.div>
                    )}
                  </AnimatePresence>
                </div>
                <div role="separator" className="h-px bg-default-200/50 my-2" />
                {(() => {
                  const acc: React.ReactNode[] = [];
                  const out: React.ReactNode[] = [];
                  orderedTopFields.forEach((f, i) => {
                    const k = f.name as any;
                    const tag = normTag(f.tag);
                    if (tag !== "compound") {
                      if (!matchesName(k)) return;
                      if (tag === "list") {
                        const dk = `tflist:${k}`;
                        const display =
                          typedDrafts[dk] ??
                          String((f as any).valueJSON || "[]");
                        const items = parseListJSON(display);
                        acc.push(
                          <FieldBox
                            key={`tf-${k}`}
                            title={k}
                            type={tag}
                            delay={i * 0.015}
                          >
                            <div className="flex flex-col gap-2">
                              <div className="flex gap-1 overflow-x-auto flex-nowrap pretty-scrollbar gutter-stable">
                                {items.map((it, idx) => (
                                  <Input
                                    key={`tf-${k}-li-${idx}`}
                                    aria-label={`${k} item ${idx}`}
                                    size="sm"
                                    variant="flat"
                                    radius="lg"
                                    classNames={{
                                      inputWrapper:
                                        "bg-default-100 dark:bg-default-50/20 group-data-[focus=true]:bg-default-200/50",
                                    }}
                                    className="w-12 shrink-0"
                                    value={String(it ?? "")}
                                    onValueChange={(v) => {
                                      const next = items.slice();
                                      next[idx] = v;
                                      beforeUpdate();
                                      setTypedDrafts((prev) => ({
                                        ...prev,
                                        [dk]: stringifyList(next),
                                      }));
                                    }}
                                    onBlur={() => {
                                      const val = String(
                                        typedDrafts[dk] ?? stringifyList(items),
                                      );
                                      setTypedFieldValueByName(String(k), {
                                        valueJSON: val,
                                      });
                                      setTypedDrafts((prev) => {
                                        const nn = { ...prev };
                                        delete nn[dk];
                                        return nn;
                                      });
                                    }}
                                  />
                                ))}
                              </div>
                            </div>
                          </FieldBox>,
                        );
                      } else if (tag === "byte") {
                        const isOn =
                          String((f as any).valueString || "0") !== "0";
                        acc.push(
                          <FieldBox
                            key={`tf-${k}`}
                            title={k}
                            type={tag}
                            delay={i * 0.015}
                          >
                            <div className="flex justify-end">
                              <Switch
                                size="sm"
                                color="success"
                                isSelected={isOn}
                                onValueChange={(c) => {
                                  setTypedFieldValueByName(String(k), {
                                    valueString: c ? "1" : "0",
                                  });
                                }}
                                thumbIcon={
                                  <span className="block w-2 h-2 bg-black rounded-full" />
                                }
                              />
                            </div>
                          </FieldBox>,
                        );
                      } else {
                        const opts = getEnumOpts(String(k));
                        if (opts) {
                          acc.push(
                            <FieldBox
                              key={`tf-${k}`}
                              title={k}
                              type={tag}
                              delay={i * 0.015}
                            >
                              <Select
                                aria-label={String(k)}
                                size="sm"
                                radius="lg"
                                variant="flat"
                                classNames={{
                                  trigger:
                                    "bg-default-100 dark:bg-default-50/20",
                                }}
                                selectedKeys={
                                  new Set([
                                    String((f as any).valueString || "0"),
                                  ])
                                }
                                onSelectionChange={(keys: any) => {
                                  const v = Array.from(keys)[0] || "0";
                                  setTypedFieldValueByName(String(k), {
                                    valueString: String(v),
                                  });
                                }}
                              >
                                {opts.map((o) => (
                                  <SelectItem key={o.value}>
                                    {o.label}
                                  </SelectItem>
                                ))}
                              </Select>
                            </FieldBox>,
                          );
                        } else {
                          const dk = `tf:${k}`;
                          const display =
                            typedDrafts[dk] ??
                            String((f as any).valueString || "");
                          acc.push(
                            <FieldBox
                              key={`tf-${k}`}
                              title={k}
                              type={tag}
                              delay={i * 0.015}
                            >
                              <Input
                                aria-label={String(k)}
                                size="sm"
                                variant="flat"
                                radius="lg"
                                classNames={{
                                  inputWrapper:
                                    "bg-default-100 dark:bg-default-50/20 group-data-[focus=true]:bg-default-200/50",
                                }}
                                value={display}
                                onValueChange={(v) => {
                                  beforeUpdate();
                                  setTypedDrafts((prev) => ({
                                    ...prev,
                                    [dk]: v,
                                  }));
                                }}
                                onBlur={() => {
                                  const val = String(typedDrafts[dk] ?? "");
                                  setTypedFieldValueByName(String(k), {
                                    valueString: val,
                                  });
                                  setTypedDrafts((prev) => {
                                    const next = { ...prev };
                                    delete next[dk];
                                    return next;
                                  });
                                }}
                              />
                            </FieldBox>,
                          );
                        }
                      }
                      return;
                    }
                    const pathKey = String(k);
                    const listRaw = compoundFields[pathKey] || [];
                    const ord = compoundOrders[pathKey] || [];
                    const list = (() => {
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
                    })();
                    const listShow = filterText
                      ? list.filter((sf) => matchesName(sf.name))
                      : list;
                    if (filterText && !matchesName(k) && listShow.length === 0)
                      return;
                    if (acc.length) {
                      out.push(
                        <div
                          key={`grid-before-${k}`}
                          className="grid grid-cols-[repeat(auto-fit,minmax(18rem,1fr))] gap-4"
                        >
                          {acc.splice(0, acc.length)}
                        </div>,
                      );
                    }
                    out.push(
                      <div key={`c-${k}`} className="mt-3">
                        <div className="flex items-center justify-between">
                          <div className="text-xs text-default-500">
                            {String(k)}
                          </div>
                          <div className="flex items-center gap-2">
                            <Button
                              size="sm"
                              radius="lg"
                              variant="flat"
                              className="bg-default-100 dark:bg-default-50/20 text-default-600"
                              onPress={() => {
                                if (!compoundOpen[pathKey]) {
                                  const hasLocal =
                                    (compoundFields[pathKey] || []).length > 0;
                                  if (hasLocal) {
                                    beforeUpdate();
                                    setCompoundOpen((p) => ({
                                      ...p,
                                      [pathKey]: true,
                                    }));
                                  } else {
                                    loadCompound(pathKey);
                                  }
                                } else {
                                  beforeUpdate();
                                  setCompoundOpen((p) => ({
                                    ...p,
                                    [pathKey]: !p[pathKey],
                                  }));
                                }
                              }}
                            >
                              {compoundOpen[pathKey]
                                ? t("common.collapse", { defaultValue: "收起" })
                                : t("common.expand", { defaultValue: "展开" })}
                            </Button>
                          </div>
                        </div>
                        <div
                          role="separator"
                          className="h-px bg-default-200 my-2"
                        />
                        {compoundOpen[pathKey] ? (
                          <div className="grid grid-cols-[repeat(auto-fit,minmax(16rem,1fr))] gap-4">
                            {listShow.map((sf, si) => {
                              const stag = normTag(sf.tag);
                              if (stag === "string") {
                                const opts = getEnumOpts(String(sf.name));
                                if (opts) {
                                  return (
                                    <FieldBox
                                      key={`c-${k}-${sf.name}`}
                                      title={String(sf.name)}
                                      type={stag}
                                      delay={si * 0.015}
                                    >
                                      <Select
                                        aria-label={String(sf.name)}
                                        size="sm"
                                        radius="lg"
                                        variant="flat"
                                        classNames={{
                                          trigger:
                                            "bg-default-100 dark:bg-default-50/20",
                                        }}
                                        selectedKeys={
                                          new Set([
                                            String(sf.valueString || "0"),
                                          ])
                                        }
                                        onSelectionChange={(keys: any) => {
                                          const v = Array.from(keys)[0] || "0";
                                          setCompoundFieldValue(pathKey, si, {
                                            valueString: String(v),
                                          });
                                        }}
                                      >
                                        {opts.map((o) => (
                                          <SelectItem key={o.value}>
                                            {o.label}
                                          </SelectItem>
                                        ))}
                                      </Select>
                                    </FieldBox>
                                  );
                                }
                                const dk = `cf:${k}:${String(sf.name)}`;
                                const display =
                                  typedDrafts[dk] ??
                                  String(sf.valueString || "");
                                return (
                                  <FieldBox
                                    key={`c-${k}-${sf.name}`}
                                    title={String(sf.name)}
                                    type={stag}
                                    delay={si * 0.015}
                                  >
                                    <Input
                                      aria-label={String(sf.name)}
                                      size="sm"
                                      variant="flat"
                                      radius="lg"
                                      classNames={{
                                        inputWrapper:
                                          "bg-default-100 dark:bg-default-50/20 group-data-[focus=true]:bg-default-200/50",
                                      }}
                                      value={display}
                                      onValueChange={(v) => {
                                        beforeUpdate();
                                        setTypedDrafts((prev) => ({
                                          ...prev,
                                          [dk]: v,
                                        }));
                                      }}
                                      onBlur={() => {
                                        const val = String(
                                          typedDrafts[dk] ?? "",
                                        );
                                        setCompoundFieldValue(pathKey, si, {
                                          valueString: val,
                                        });
                                        setTypedDrafts((prev) => {
                                          const next = { ...prev };
                                          delete next[dk];
                                          return next;
                                        });
                                      }}
                                    />
                                  </FieldBox>
                                );
                              }
                              if (stag === "list") {
                                const dk = `cflist:${k}:${String(sf.name)}`;
                                const display =
                                  typedDrafts[dk] ??
                                  String(sf.valueJSON || "[]");
                                const items = parseListJSON(display);
                                return (
                                  <FieldBox
                                    key={`c-${k}-${sf.name}`}
                                    title={String(sf.name)}
                                    type={stag}
                                    delay={si * 0.015}
                                  >
                                    <div className="flex flex-col gap-2">
                                      <div className="flex gap-1 overflow-x-auto flex-nowrap pretty-scrollbar gutter-stable">
                                        {items.map((it, idx) => (
                                          <Input
                                            key={`c-${k}-li-${idx}`}
                                            aria-label={`${sf.name} item ${idx}`}
                                            size="sm"
                                            variant="flat"
                                            radius="lg"
                                            classNames={{
                                              inputWrapper:
                                                "bg-default-100 dark:bg-default-50/20 group-data-[focus=true]:bg-default-200/50",
                                            }}
                                            className="w-12 shrink-0"
                                            value={String(it ?? "")}
                                            onValueChange={(v) => {
                                              const next = items.slice();
                                              next[idx] = v;
                                              beforeUpdate();
                                              setTypedDrafts((prev) => ({
                                                ...prev,
                                                [dk]: stringifyList(next),
                                              }));
                                            }}
                                            onBlur={() => {
                                              const val = String(
                                                typedDrafts[dk] ??
                                                  stringifyList(items),
                                              );
                                              setCompoundFieldValue(
                                                pathKey,
                                                si,
                                                { valueJSON: val },
                                              );
                                              setTypedDrafts((prev) => {
                                                const nn = { ...prev };
                                                delete nn[dk];
                                                return nn;
                                              });
                                            }}
                                          />
                                        ))}
                                      </div>
                                    </div>
                                  </FieldBox>
                                );
                              }
                              if (
                                stag === "byte" ||
                                stag === "short" ||
                                stag === "int" ||
                                stag === "long" ||
                                stag === "float" ||
                                stag === "double"
                              ) {
                                const isBoolLike =
                                  sf.name?.[0] >= "a" &&
                                  sf.name?.[0] <= "z" &&
                                  stag === "byte";
                                if (isBoolLike) {
                                  const isOn =
                                    String(sf.valueString || "0") !== "0";
                                  return (
                                    <FieldBox
                                      key={`c-${k}-${sf.name}`}
                                      title={String(sf.name)}
                                      type={stag}
                                      delay={si * 0.015}
                                    >
                                      <div className="flex justify-end">
                                        <Switch
                                          size="sm"
                                          color="success"
                                          isSelected={isOn}
                                          onValueChange={(c) => {
                                            setCompoundFieldValue(pathKey, si, {
                                              valueString: c ? "1" : "0",
                                            });
                                          }}
                                          thumbIcon={
                                            <span className="block w-2 h-2 bg-black rounded-full" />
                                          }
                                        />
                                      </div>
                                    </FieldBox>
                                  );
                                }
                                const dk = `cf:${k}:${String(sf.name)}`;
                                const display =
                                  typedDrafts[dk] ??
                                  String(sf.valueString || "");
                                return (
                                  <FieldBox
                                    key={`c-${k}-${sf.name}`}
                                    title={String(sf.name)}
                                    type={stag}
                                    delay={si * 0.015}
                                  >
                                    <Input
                                      aria-label={String(sf.name)}
                                      size="sm"
                                      variant="flat"
                                      radius="lg"
                                      classNames={{
                                        inputWrapper:
                                          "bg-default-100 dark:bg-default-50/20 group-data-[focus=true]:bg-default-200/50",
                                      }}
                                      value={display}
                                      onValueChange={(v) => {
                                        beforeUpdate();
                                        setTypedDrafts((prev) => ({
                                          ...prev,
                                          [dk]: v,
                                        }));
                                      }}
                                      onBlur={() => {
                                        const val = String(
                                          typedDrafts[dk] ?? "",
                                        );
                                        setCompoundFieldValue(pathKey, si, {
                                          valueString: val,
                                        });
                                        setTypedDrafts((prev) => {
                                          const next = { ...prev };
                                          delete next[dk];
                                          return next;
                                        });
                                      }}
                                    />
                                  </FieldBox>
                                );
                              }
                              return (
                                <FieldBox
                                  key={`c-${k}-${sf.name}`}
                                  title={String(sf.name)}
                                  type={stag}
                                  delay={si * 0.015}
                                >
                                  {(() => {
                                    const dk = `cfjson:${k}:${String(sf.name)}`;
                                    const display =
                                      typedDrafts[dk] ??
                                      String(sf.valueJSON || "");
                                    return (
                                      <>
                                        <Input
                                          aria-label={String(sf.name)}
                                          size="sm"
                                          variant="flat"
                                          radius="lg"
                                          classNames={{
                                            inputWrapper:
                                              "bg-default-100 dark:bg-default-50/20 group-data-[focus=true]:bg-default-200/50",
                                          }}
                                          value={display}
                                          onValueChange={(v) => {
                                            beforeUpdate();
                                            setTypedDrafts((prev) => ({
                                              ...prev,
                                              [dk]: v,
                                            }));
                                          }}
                                          onBlur={() => {
                                            const val = String(
                                              typedDrafts[dk] ?? "",
                                            );
                                            setCompoundFieldValue(pathKey, si, {
                                              valueJSON: val,
                                            });
                                            setTypedDrafts((prev) => {
                                              const next = { ...prev };
                                              delete next[dk];
                                              return next;
                                            });
                                          }}
                                        />
                                        <div className="mt-2">
                                          <Button
                                            size="sm"
                                            radius="lg"
                                            variant="flat"
                                            className="bg-default-100 dark:bg-default-50/20 text-default-600"
                                            onPress={() => {
                                              const segs = pathKey.split("/");
                                              const nextPath = [
                                                ...segs,
                                                String(sf.name || ""),
                                              ];
                                              loadCompound(nextPath);
                                            }}
                                          >
                                            {t("common.expand", {
                                              defaultValue: "展开",
                                            })}
                                          </Button>
                                        </div>
                                      </>
                                    );
                                  })()}
                                </FieldBox>
                              );
                            })}
                          </div>
                        ) : null}
                      </div>,
                    );
                  });
                  if (acc.length) {
                    out.push(
                      <div
                        key={`grid-last`}
                        className="grid grid-cols-[repeat(auto-fit,minmax(18rem,1fr))] gap-4"
                      >
                        {acc}
                      </div>,
                    );
                  }
                  return out;
                })()}
              </div>
            )}
          </div>
        </CardBody>
      </Card>
    </motion.div>
  );
}
