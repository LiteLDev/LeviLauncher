import React, { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Button,
  Card,
  CardBody,
  Checkbox,
  Chip,
  Input,
  Select,
  SelectItem,
  ModalContent,
} from "@heroui/react";
import { BaseModal, BaseModalHeader, BaseModalBody, BaseModalFooter } from "../components/BaseModal";
import { AnimatePresence, motion } from "framer-motion";
import { useLocation, useNavigate } from "react-router-dom";
import * as minecraft from "../../bindings/github.com/liteldev/LeviLauncher/minecraft";

export type FileManagerState = {
  allowedExt?: string[];
  multi?: boolean;
  title?: string;
  initialPath?: string;
  returnTo?: string;
  returnState?: any;
  directoryPickMode?: boolean;
};

type Entry = { name: string; path: string; isDir: boolean; size: number };

type SortKey = "name" | "kind";

type ViewMode = "grid" | "list";

type ScreenSize = "mobile" | "tablet" | "desktop";

const FileManagerPage: React.FC = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const fmState = (location.state as FileManagerState) || {};

  const savedState = (() => {
    try {
      return JSON.parse(localStorage.getItem("fm.state") || "{}");
    } catch {
      return {};
    }
  })() as Partial<{
    path: string;
    view: ViewMode;
    sortKey: SortKey;
    sortAsc: boolean;
    query: string;
    qaOpen: boolean;
    pinnedOpen: boolean;
  }>;

  const [drives, setDrives] = useState<string[]>([]);
  const [path, setPath] = useState<string>(fmState.initialPath || "");
  const [entries, setEntries] = useState<Entry[]>([]);
  const [selected, setSelected] = useState<Record<string, boolean>>({});
  const [query, setQuery] = useState(savedState.query || "");
  const [sortKey, setSortKey] = useState<SortKey>(
    savedState.sortKey && ["name", "kind"].includes(savedState.sortKey)
      ? (savedState.sortKey as SortKey)
      : "name"
  );
  const [sortAsc, setSortAsc] = useState(savedState.sortAsc ?? true);
  const [view, setView] = useState<ViewMode>(savedState.view || "list");
  const [screenSize, setScreenSize] = useState<ScreenSize>("desktop");
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [known, setKnown] = useState<{ name: string; path: string }[]>([]);
  const [driveStats, setDriveStats] = useState<
    Record<string, { total: number; free: number }>
  >({});
  const [pins, setPins] = useState<string[]>(() => {
    try {
      return JSON.parse(localStorage.getItem("fm.pins") || "[]");
    } catch {
      return [];
    }
  });
  const [qaOpen, setQaOpen] = useState(savedState.qaOpen ?? true);
  const [pinnedOpen, setPinnedOpen] = useState(savedState.pinnedOpen ?? true);
  const [navDir, setNavDir] = useState<1 | -1>(1);
  const [editingPath, setEditingPath] = useState(false);
  const [pathEditText, setPathEditText] = useState("");
  const contentRef = useRef<HTMLDivElement | null>(null);
  const [scrollTop, setScrollTop] = useState(0);

  const multi = fmState.multi !== false;
  const allowed = (fmState.allowedExt || []).map((s) => s.toLowerCase());
  const [dirWritable, setDirWritable] = useState<boolean>(true);
  const [canCreateHere, setCanCreateHere] = useState<boolean>(true);
  const [mkdirOpen, setMkdirOpen] = useState(false);
  const [mkdirName, setMkdirName] = useState("");
  const [creatingFolder, setCreatingFolder] = useState(false);

  useEffect(() => {
    const update = () => {
      const w = window.innerWidth;
      if (w < 640) setScreenSize("mobile");
      else if (w < 1024) setScreenSize("tablet");
      else setScreenSize("desktop");
    };
    update();
    window.addEventListener("resize", update);
    return () => window.removeEventListener("resize", update);
  }, []);

  useEffect(() => {
    if (screenSize !== "mobile" && drawerOpen) setDrawerOpen(false);
  }, [screenSize, drawerOpen]);

  const savePins = (list: string[]) => {
    setPins(list);
    try {
      localStorage.setItem("fm.pins", JSON.stringify(list));
    } catch {}
  };
  const togglePin = (p: string) => {
    const has = pins.includes(p);
    const next = has ? pins.filter((x) => x !== p) : [p, ...pins];
    savePins(next);
  };

  const getExt = (name: string) => {
    const m = /\.([^.]+)$/.exec(name.toLowerCase());
    return m ? m[1] : "";
  };

  const kindMeta = (e: Entry) => {
    if (e.isDir)
      return {
        label: t("file.types.folder"),
        color: "bg-primary/15 text-primary",
        tone: "bg-default-200",
        inner: "bg-primary",
        key: "folder",
      } as any;
    const ext = getExt(e.name);
    const map: Record<
      string,
      { label: string; color: string; tone: string; inner: string }
    > = {
      exe: {
        label: t("file.types.executable"),
        color: "bg-danger/15 text-danger",
        tone: "bg-default-200",
        inner: "bg-danger",
      },
      dll: {
        label: t("file.types.library"),
        color: "bg-warning/15 text-warning",
        tone: "bg-default-200",
        inner: "bg-warning",
      },
      zip: {
        label: t("file.types.archive"),
        color: "bg-success/15 text-success",
        tone: "bg-default-200",
        inner: "bg-success",
      },
      rar: {
        label: t("file.types.archive"),
        color: "bg-success/15 text-success",
        tone: "bg-default-200",
        inner: "bg-success",
      },
      "7z": {
        label: t("file.types.archive"),
        color: "bg-success/15 text-success",
        tone: "bg-default-200",
        inner: "bg-success",
      },
      png: {
        label: t("file.types.image"),
        color: "bg-secondary/15 text-secondary",
        tone: "bg-default-200",
        inner: "bg-secondary",
      },
      jpg: {
        label: t("file.types.image"),
        color: "bg-secondary/15 text-secondary",
        tone: "bg-default-200",
        inner: "bg-secondary",
      },
      jpeg: {
        label: t("file.types.image"),
        color: "bg-secondary/15 text-secondary",
        tone: "bg-default-200",
        inner: "bg-secondary",
      },
      gif: {
        label: t("file.types.image"),
        color: "bg-secondary/15 text-secondary",
        tone: "bg-default-200",
        inner: "bg-secondary",
      },
      txt: {
        label: t("file.types.text"),
        color: "bg-default-200/50 text-default-700",
        tone: "bg-default-200",
        inner: "bg-default-500",
      },
      json: {
        label: t("file.types.json"),
        color: "bg-default-200/50 text-default-700",
        tone: "bg-default-200",
        inner: "bg-default-500",
      },
      jar: {
        label: t("file.types.jar"),
        color: "bg-foreground/10 text-foreground",
        tone: "bg-default-200",
        inner: "bg-foreground/70",
      },
      mcpack: {
        label: t("file.types.mcpack"),
        color: "bg-foreground/10 text-foreground",
        tone: "bg-default-200",
        inner: "bg-foreground/70",
      },
      mcaddon: {
        label: t("file.types.mcaddon"),
        color: "bg-foreground/10 text-foreground",
        tone: "bg-default-200",
        inner: "bg-foreground/70",
      },
    };
    const meta = map[ext] || {
      label: ext ? ext.toUpperCase() : t("file.types.file"),
      color: "bg-default-200/50 text-default-700",
      tone: "bg-default-200",
      inner: "bg-default-500",
    };
    return { ...meta, key: meta.label.toLowerCase() } as {
      label: string;
      color: string;
      tone: string;
      inner: string;
      key: string;
    };
  };

  const isWindowsPath = (s: string) => {
    const v = (s || "").trim();
    return /^(?:[A-Za-z]:[\\\/]|[\\\/]{2})/.test(v);
  };

  const normalizeWindowsPath = (s: string) => {
    let v = (s || "").trim();
    if (
      (v.startsWith('"') && v.endsWith('"')) ||
      (v.startsWith("'") && v.endsWith("'"))
    ) {
      v = v.slice(1, -1);
    }
    v = v.replace(/\//g, "\\");
    if (/^[A-Za-z]:$/.test(v)) v = v + "\\";
    return v;
  };

  useEffect(() => {
    const init = async () => {
      try {
        const ds: string[] = (await minecraft?.ListDrives?.()) || [];
        setDrives(ds);
        try {
          const k = await minecraft?.ListKnownFolders?.();
          setKnown(Array.isArray(k) ? k : []);
        } catch {}
        const stats: Record<string, { total: number; free: number }> = {};
        await Promise.all(
          ds.map(async (d) => {
            try {
              const st = await minecraft?.GetDriveStats?.(d);
              if (st)
                stats[d] = {
                  total: Number(st.total || 0),
                  free: Number(st.free || 0),
                };
            } catch {}
          })
        );
        setDriveStats(stats);
        const start = fmState.initialPath || savedState.path || "";
        if (start) await loadDir(start);
      } catch {}
    };
    init();
  }, []);

  useEffect(() => {
    try {
      localStorage.setItem(
        "fm.state",
        JSON.stringify({
          path,
          view,
          sortKey,
          sortAsc,
          query,
          qaOpen,
          pinnedOpen,
        })
      );
    } catch {}
  }, [path, view, sortKey, sortAsc, query, qaOpen, pinnedOpen]);

  const loadDir = async (p: string) => {
    try {
      const list: any[] = (await minecraft?.ListDir?.(p)) || [];
      const flt = list.filter(
        (e) =>
          e?.isDir ||
          allowed.length === 0 ||
          allowed.some((ext) =>
            String(e?.name || "")
              .toLowerCase()
              .endsWith(ext)
          )
      );
      const prevDepth = (path || "")
        .replace(/\\+$/, "")
        .split(/\\/)
        .filter(Boolean).length;
      const nextDepth = (p || "")
        .replace(/\\+$/, "")
        .split(/\\/)
        .filter(Boolean).length;
      setNavDir(nextDepth >= prevDepth ? 1 : -1);
      setEntries(
        flt.map((e) => ({
          name: e.name,
          path: e.path,
          isDir: !!e.isDir,
          size: Number(e.size || 0),
        }))
      );
      setPath(p);
      setSelected({});
    } catch {}
  };

  const refreshCurrent = async () => {
    try {
      setRefreshing(true);
      const ds: string[] = (await minecraft?.ListDrives?.()) || [];
      setDrives(ds);
      try {
        const k = await minecraft?.ListKnownFolders?.();
        setKnown(Array.isArray(k) ? k : []);
      } catch {}
      const stats: Record<string, { total: number; free: number }> = {};
      await Promise.all(
        ds.map(async (d) => {
          try {
            const st = await minecraft?.GetDriveStats?.(d);
            if (st)
              stats[d] = {
                total: Number(st.total || 0),
                free: Number(st.free || 0),
              };
          } catch {}
        })
      );
      setDriveStats(stats);
      if (path) await loadDir(path);
    } finally {
      setRefreshing(false);
    }
  };

  const goUp = async () => {
    if (!path) return;
    const parts = path.replace(/\\+$/, "").split(/\\/).filter(Boolean);
    if (parts.length <= 1) {
      setPath("");
      setSelected({});
      return;
    }
    const parent =
      path.endsWith("\\") && parts.length === 1
        ? path
        : path.replace(/\\+$/, "").replace(/\\[^\\]+$/, "\\");
    await loadDir(parent);
  };

  const breadcrumb = useMemo(() => {
    const items: { name: string; full: string }[] = [];
    items.push({ name: t("filemanager.computer"), full: "" });
    if (!path) return items;
    const clean = path.replace(/\\+$/, "");
    const parts = clean.split(/\\/).filter(Boolean);
    let full = "";
    for (let i = 0; i < parts.length; i++) {
      const seg = parts[i];
      full =
        i === 0 ? seg + "\\" : full.replace(/\\+$/, "") + "\\" + seg + "\\";
      items.push({ name: i === 0 ? seg + "\\" : seg, full });
    }
    return items;
  }, [path]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    const list = entries
      .filter((e) => !q || e.name.toLowerCase().includes(q))
      .filter((e) => (fmState?.directoryPickMode ? e.isDir : true));
    const ordered = [...list].sort((a, b) => {
      let cmp = 0;
      if (sortKey === "name") cmp = a.name.localeCompare(b.name);
      else cmp = Number(a.isDir) - Number(b.isDir);
      return sortAsc ? cmp : -cmp;
    });
    return ordered;
  }, [entries, query, sortKey, sortAsc]);

  const rowHeight = screenSize === "mobile" ? 56 : 48;
  const virt = useMemo(() => {
    if (!path) return null;
    const count = filtered.length;
    const shouldVirtualize = count > 800;
    if (!shouldVirtualize) return null;
    const viewport = contentRef.current?.clientHeight || 0;
    const start = Math.max(0, Math.floor(scrollTop / rowHeight) - 8);
    const end = Math.min(
      count,
      Math.ceil((scrollTop + viewport) / rowHeight) + 8
    );
    const offsetY = start * rowHeight;
    return { start, end, offsetY, totalHeight: count * rowHeight };
  }, [filtered, path, scrollTop, rowHeight, screenSize]);

  const toggle = (p: string) => {
    if (multi) {
      setSelected((prev) => ({ ...prev, [p]: !prev[p] }));
    } else {
      setSelected({ [p]: true });
    }
  };

  useEffect(() => {
    const check = async () => {
      try {
        if (fmState?.directoryPickMode && path) {
          const ok = await minecraft?.CanWriteToDir?.(path);
          setDirWritable(Boolean(ok));
        } else {
          setDirWritable(true);
        }
        if (path) {
          const ok2 = await minecraft?.CanWriteToDir?.(path);
          setCanCreateHere(Boolean(ok2));
        } else {
          setCanCreateHere(false);
        }
      } catch {
        setDirWritable(false);
        setCanCreateHere(false);
      }
    };
    check();
  }, [fmState?.directoryPickMode, path]);

  const confirm = () => {
    const returnTo = fmState.returnTo || "/mods";
    if (fmState?.directoryPickMode) {
      navigate(returnTo, {
        state: {
          ...(fmState.returnState || {}),
          baseRootPickResult: path || "",
        },
        replace: true,
      });
      return;
    }
    const paths = Object.keys(selected).filter((k) => selected[k]);
    const out = multi ? paths : paths.length ? [paths[0]] : [];
    navigate(returnTo, {
      state: { ...(fmState.returnState || {}), fileManagerResult: out },
      replace: true,
    });
  };

  const renderRow = (e: Entry) => {
    const meta = kindMeta(e);
    const isSel = !!selected[e.path];
    const isPinned = pins.includes(e.path);
    return (
      <div
        key={e.path}
        className={`flex items-center gap-3 px-3 ${
          screenSize === "mobile" ? "h-14" : "h-12"
        } ${
          !e.isDir && isSel ? "bg-primary/10" : "hover:bg-default-100/40"
        } transition-colors`}
        onClick={() => {
          if (e.isDir) loadDir(e.path);
        }}
      >
        <div
          className={`rounded-lg flex items-center justify-center shrink-0 ${
            meta.tone
          } ${screenSize === "mobile" ? "w-8 h-8" : "w-7 h-7"}`}
        >
          <div
            className={`rounded ${meta.inner} ${
              screenSize === "mobile" ? "w-4 h-4" : "w-4 h-4"
            }`}
          ></div>
        </div>
        <div className="flex-1 min-w-0">
          <div
            className="font-medium leading-tight truncate text-sm"
            title={e.name}
          >
            {e.name}
          </div>
          <div className="flex items-center gap-2">
            {screenSize !== "mobile" && (
              <Chip
                size="sm"
                className={`text-tiny ${meta.color}`}
                variant="flat"
              >
                {meta.label}
              </Chip>
            )}
            {null}
          </div>
        </div>
        {!e.isDir && (
          <Checkbox
            isSelected={isSel}
            onValueChange={() => toggle(e.path)}
            size="sm"
            className="z-10"
            onClick={(ev) => ev.stopPropagation()}
          />
        )}
        {e.isDir && (
          <div className="flex items-center gap-1">
            <Button
              isIconOnly
              size="sm"
              variant="light"
              title={isPinned ? "Unpin" : "Pin"}
              onPress={() => {
                togglePin(e.path);
              }}
            >
              {isPinned ? "⭐" : "☆"}
            </Button>
            <div className="text-default-400 text-sm">→</div>
          </div>
        )}
      </div>
    );
  };

  const Drawer = () => (
    <div
      className={`fixed inset-0 z-50 transition-opacity ${
        drawerOpen ? "pointer-events-auto" : "pointer-events-none"
      }`}
    >
      <div
        className={`absolute inset-0 bg-black/50 transition-opacity duration-200 ${
          drawerOpen ? "opacity-100" : "opacity-0"
        }`}
        onClick={() => setDrawerOpen(false)}
      />
      <div
        className={`absolute left-0 top-0 h-full w-60 rounded-r-2xl bg-white/60 dark:bg-black/30 backdrop-blur-md border-r border-white/30 shadow-xl transition-transform duration-300 ${
          drawerOpen ? "translate-x-0" : "-translate-x-full"
        }`}
      >
        <Card className="h-full rounded-none bg-transparent shadow-none">
          <CardBody className="p-4 flex h-full flex-col gap-3">
            <div className="flex items-center gap-2">
              <div className="text-sm font-medium text-default-600">
                {t("filemanager.drives_title")}
              </div>
              <div className="ml-auto flex items-center gap-1.5">
                <Button
                  isIconOnly
                  size="sm"
                  variant="light"
                  onPress={refreshCurrent}
                  isDisabled={refreshing}
                  title={t("common.refresh")}
                >
                  {refreshing ? "…" : "⟳"}
                </Button>
                <Button
                  isIconOnly
                  size="sm"
                  variant="light"
                  onPress={() => setDrawerOpen(false)}
                  title={t("common.close")}
                >
                  ✕
                </Button>
              </div>
            </div>
            <div className="-m-2 -mt-1 flex-1 overflow-y-auto p-2 pretty-scrollbar gutter-stable">
              {/* Drives (Top) */}
              <div className="flex flex-col gap-1.5">
                {drives.map((d) => (
                  <Button
                    key={d}
                    size="sm"
                    className="w-full justify-between h-10"
                    variant={
                      path.toLowerCase().startsWith(d.toLowerCase())
                        ? "flat"
                        : "light"
                    }
                    color={
                      path.toLowerCase().startsWith(d.toLowerCase())
                        ? "primary"
                        : "default"
                    }
                    onPress={() => {
                      loadDir(d);
                      setDrawerOpen(false);
                    }}
                    title={d}
                  >
                    <span>💾 {d}</span>
                    <span className="text-tiny text-default-500">
                      {(() => {
                        const st = driveStats[d];
                        if (!st || !st.total) return "";
                        const used = st.total - st.free;
                        const pct = Math.round((used / st.total) * 100);
                        return `${pct}%`;
                      })()}
                    </span>
                  </Button>
                ))}
              </div>
              {/* Quick Access (Collapsible) */}
              {known.length > 0 && (
                <div className="mt-3">
                  <div className="flex items-center justify-between text-tiny text-default-500 mb-1">
                    <div>{t("filemanager.quick_access")}</div>
                    <Button
                      size="sm"
                      variant="light"
                      isIconOnly
                      onPress={() => setQaOpen((v) => !v)}
                    >
                      {qaOpen ? "▾" : "▸"}
                    </Button>
                  </div>
                  <motion.div
                    initial={false}
                    animate={{
                      height: qaOpen ? "auto" : 0,
                      opacity: qaOpen ? 1 : 0,
                    }}
                    transition={{ duration: 0.2 }}
                    className="flex flex-col gap-1.5 overflow-hidden"
                    style={{ pointerEvents: qaOpen ? "auto" : "none" }}
                    aria-hidden={!qaOpen}
                  >
                    {known.map((k) => (
                      <Button
                        key={k.path}
                        size="sm"
                        className="w-full justify-start h-9"
                        variant="light"
                        onPress={() => loadDir(k.path)}
                        title={k.path}
                      >
                        📁 {k.name}
                      </Button>
                    ))}
                  </motion.div>
                </div>
              )}
              {/* Pinned (Collapsible) */}
              {pins.length > 0 && (
                <div className="mt-3">
                  <div className="flex items-center justify-between text-tiny text-default-500 mb-1">
                    <div>{t("filemanager.pinned")}</div>
                    <Button
                      size="sm"
                      variant="light"
                      isIconOnly
                      onPress={() => setPinnedOpen((v) => !v)}
                    >
                      {pinnedOpen ? "▾" : "▸"}
                    </Button>
                  </div>
                  <motion.div
                    initial={false}
                    animate={{
                      height: pinnedOpen ? "auto" : 0,
                      opacity: pinnedOpen ? 1 : 0,
                    }}
                    transition={{ duration: 0.2 }}
                    className="flex flex-col gap-1.5 overflow-hidden"
                  >
                    {pins.map((p) => (
                      <Button
                        key={p}
                        size="sm"
                        className="w-full justify-between h-9"
                        variant="light"
                        onPress={() => loadDir(p)}
                        title={p}
                      >
                        <span className="truncate">📌 {p}</span>
                        <span
                          className="text-default-400"
                          onClick={(e) => {
                            e.stopPropagation();
                            togglePin(p);
                          }}
                        >
                          ✕
                        </span>
                      </Button>
                    ))}
                  </motion.div>
                </div>
              )}
            </div>
            <div className="border-t border-divider" />
            <div className="flex items-center gap-2">
              <Button
                size="sm"
                variant="flat"
                className="flex-1 h-10"
                onPress={goUp}
                title={t("filemanager.up")}
              >
                {"⬆️ "}
                {t("filemanager.up")}
              </Button>
              <Button
                size="sm"
                variant="flat"
                className="h-10"
                onPress={() => {
                  if (path) togglePin(path);
                }}
                title={t("filemanager.pin_current")}
              >
                📌
              </Button>
              <Button
                size="sm"
                variant="flat"
                className="h-10"
                onPress={refreshCurrent}
                isDisabled={refreshing}
                title={t("common.refresh")}
              >
                {refreshing ? t("filemanager.refreshing") : t("common.refresh")}
              </Button>
            </div>
            <div className="text-xs text-default-400 break-words min-h-[1.5rem] leading-tight">
              {path || t("filemanager.choose_drive")}
            </div>
          </CardBody>
        </Card>
      </div>
    </div>
  );

  const Sidebar = () => (
    <aside className="hidden sm:block w-60 shrink-0">
      <Card className="h-full rounded-2xl shadow-md bg-white/70 dark:bg-black/30 backdrop-blur-md border border-white/30">
        <CardBody className="p-4 flex h-full flex-col gap-3">
          <div className="flex items-center gap-2">
            <div className="text-sm font-medium text-default-600">
              {t("filemanager.drives_title")}
            </div>
            <div className="ml-auto flex items-center gap-1.5">
              <Button
                isIconOnly
                size="sm"
                variant="light"
                onPress={refreshCurrent}
                isDisabled={refreshing}
                title={t("common.refresh")}
              >
                {refreshing ? "…" : "⟳"}
              </Button>
            </div>
          </div>
          <div className="-m-2 -mt-1 flex-1 overflow-y-auto p-2 pretty-scrollbar gutter-stable">
            {/* Drives (Top) */}
            <div className="flex flex-col gap-1.5">
              {drives.map((d) => (
                <Button
                  key={d}
                  size="sm"
                  className="w-full justify-between h-10"
                  variant={
                    path.toLowerCase().startsWith(d.toLowerCase())
                      ? "flat"
                      : "light"
                  }
                  color={
                    path.toLowerCase().startsWith(d.toLowerCase())
                      ? "primary"
                      : "default"
                  }
                  onPress={() => loadDir(d)}
                  title={d}
                >
                  <span>💾 {d}</span>
                  <span className="text-tiny text-default-500">
                    {(() => {
                      const st = driveStats[d];
                      if (!st || !st.total) return "";
                      const used = st.total - st.free;
                      const pct = Math.round((used / st.total) * 100);
                      return `${pct}%`;
                    })()}
                  </span>
                </Button>
              ))}
            </div>
            {/* Quick Access (Collapsible) */}
            {known.length > 0 && (
              <div className="mt-3">
                <div className="flex items-center justify-between text-tiny text-default-500 mb-1">
                  <div>{t("filemanager.quick_access")}</div>
                  <Button
                    size="sm"
                    variant="light"
                    isIconOnly
                    onClick={() => setQaOpen((v) => !v)}
                  >
                    {qaOpen ? "▾" : "▸"}
                  </Button>
                </div>
                <motion.div
                  initial={false}
                  animate={{
                    height: qaOpen ? "auto" : 0,
                    opacity: qaOpen ? 1 : 0,
                  }}
                  transition={{ duration: 0.2 }}
                  className="flex flex-col gap-1.5 overflow-hidden"
                  style={{ pointerEvents: qaOpen ? "auto" : "none" }}
                  aria-hidden={!qaOpen}
                >
                  {known.map((k) => (
                    <Button
                      key={k.path}
                      size="sm"
                      className="w-full justify-start h-9"
                      variant="light"
                      onPress={() => loadDir(k.path)}
                      title={k.path}
                    >
                      📁 {k.name}
                    </Button>
                  ))}
                </motion.div>
              </div>
            )}
            {/* Pinned (Collapsible) */}
            {pins.length > 0 && (
              <div className="mt-3">
                <div className="flex items-center justify-between text-tiny text-default-500 mb-1">
                  <div>{t("filemanager.pinned")}</div>
                  <Button
                    size="sm"
                    variant="light"
                    isIconOnly
                    onClick={() => setPinnedOpen((v) => !v)}
                  >
                    {pinnedOpen ? "▾" : "▸"}
                  </Button>
                </div>
                <motion.div
                  initial={false}
                  animate={{
                    height: pinnedOpen ? "auto" : 0,
                    opacity: pinnedOpen ? 1 : 0,
                  }}
                  transition={{ duration: 0.2 }}
                  className="flex flex-col gap-1.5 overflow-hidden"
                >
                  {pins.map((p) => (
                    <Button
                      key={p}
                      size="sm"
                      className="w-full justify-between h-9"
                      variant="light"
                      onPress={() => loadDir(p)}
                      title={p}
                    >
                      <span className="truncate">📌 {p}</span>
                      <span
                        className="text-default-400"
                        onClick={(e) => {
                          e.stopPropagation();
                          togglePin(p);
                        }}
                      >
                        ✕
                      </span>
                    </Button>
                  ))}
                </motion.div>
              </div>
            )}
          </div>
          <div className="text-xs text-default-400 break-words min-h-[1.5rem] leading-tight">
            {path || t("filemanager.choose_drive")}
          </div>
        </CardBody>
      </Card>
    </aside>
  );

  return (
    <motion.div
      className="w-full h-full relative"
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.25 }}
    >
      {/* Mobile Drawer */}
      {screenSize === "mobile" && <Drawer />}
      <motion.div
        className="w-full h-[calc(94vh-1rem)] sm:h-[calc(94vh-2rem)] p-2 sm:p-4 flex items-stretch gap-3 sm:gap-4 overflow-hidden"
        initial={{ opacity: 0, y: 8 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.25, delay: 0.05 }}
      >
        {screenSize !== "mobile" && <Sidebar />}
        {/* Tablet Compact Sidebar */}
        {screenSize === "mobile" && <div className="hidden" />}
        <section className="flex-1 min-w-0 min-h-0 flex flex-col">
          {/* Header */}
          <div className="flex items-center gap-2 mb-3">
            {screenSize === "mobile" && (
              <Button
                size="sm"
                variant="flat"
                className="text-xs px-2 min-w-0 shrink-0"
                onPress={() => setDrawerOpen(true)}
              >
                ☰
              </Button>
            )}
            <div className="text-base sm:text-lg lg:text-xl font-semibold truncate">
              {fmState.title || t("filemanager.title")}
            </div>
            <div className="ml-auto flex items-center gap-1.5 sm:gap-2">
              {fmState?.directoryPickMode && path && !dirWritable ? (
                <Chip size="sm" color="danger" variant="flat">
                  {t("filemanager.dir_not_writable", {
                    defaultValue: "不可写入",
                  })}
                </Chip>
              ) : null}
              <Button
                size="sm"
                variant={fmState?.directoryPickMode ? "flat" : "light"}
                color={fmState?.directoryPickMode ? "primary" : undefined}
                onPress={() =>
                  navigate(fmState.returnTo || "/mods", {
                    state: fmState.returnState,
                  })
                }
                className={screenSize === "mobile" ? "min-w-0 px-2" : ""}
                isIconOnly={screenSize === "mobile"}
              >
                {screenSize === "mobile" ? "✕" : t("common.cancel")}
              </Button>
              <Button
                size="sm"
                color="primary"
                isDisabled={
                  fmState?.directoryPickMode
                    ? !path || !dirWritable
                    : Object.values(selected).filter(Boolean).length === 0
                }
                onPress={confirm}
                className={screenSize === "mobile" ? "min-w-0 px-2" : ""}
              >
                {screenSize === "mobile"
                  ? "✓"
                  : fmState?.directoryPickMode
                  ? t("filemanager.choose_this_dir", {
                      defaultValue: "选择此目录",
                    })
                  : t("common.ok")}
              </Button>
            </div>
          </div>

          {/* Search and Controls */}
          <div
            className={`flex items-center gap-2 ${
              screenSize === "mobile" ? "flex-col" : "flex-wrap"
            }`}
          >
            <Input
              size="sm"
              variant="bordered"
              value={query}
              onValueChange={setQuery}
              placeholder={t("filemanager.search_placeholder")}
              className={
                screenSize === "mobile" ? "w-full" : "flex-1 min-w-[180px]"
              }
              startContent={screenSize === "mobile" && "🔍"}
              onPaste={(e) => {
                const text = e.clipboardData.getData("text");
                if (isWindowsPath(text)) {
                  e.preventDefault();
                  const p = normalizeWindowsPath(text);
                  loadDir(p);
                  setQuery("");
                }
              }}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  const q = query.trim();
                  if (isWindowsPath(q)) {
                    loadDir(normalizeWindowsPath(q));
                    setQuery("");
                  }
                }
              }}
            />
            <div
              className={`flex items-center gap-2 ${
                screenSize === "mobile" ? "w-full justify-between" : ""
              }`}
            >
              <Select
                size="sm"
                selectedKeys={new Set([sortKey])}
                onSelectionChange={(keys) =>
                  setSortKey(Array.from(keys as Set<string>)[0] as SortKey)
                }
                className={screenSize === "mobile" ? "w-24" : "w-28 sm:w-36"}
                aria-label="Sort by"
              >
                <SelectItem key="name">{t("filemanager.sort.name")}</SelectItem>
                <SelectItem key="kind">{t("filemanager.sort.kind")}</SelectItem>
              </Select>
              <Button
                size="sm"
                variant="flat"
                onPress={() => setSortAsc((v) => !v)}
                className="min-w-0 px-2 sm:px-3"
              >
                {sortAsc ? "↑" : "↓"}
              </Button>
              <Button
                size="sm"
                color="primary"
                variant="flat"
                onPress={() => {
                  if (!path) return;
                  setMkdirName("");
                  setMkdirOpen(true);
                }}
                isDisabled={!path || !canCreateHere}
              >
                {t("filemanager.new_folder", { defaultValue: "新建文件夹" })}
              </Button>
            </div>
          </div>

          <div className="mt-2 flex items-center gap-1 py-1 flex-wrap overflow-x-auto">
            <Button
              size="sm"
              variant="light"
              className="text-xs px-2 min-w-0 shrink-0"
              onPress={() => {
                setEditingPath((v) => {
                  const next = !v;
                  if (!v) {
                    setPathEditText(path || "");
                  }
                  return next;
                });
              }}
              title={t("filemanager.edit_path") || "Edit path"}
            >
              ✎
            </Button>
            {editingPath ? (
              <>
                <Input
                  size="sm"
                  variant="bordered"
                  value={pathEditText}
                  onValueChange={setPathEditText}
                  className="flex-1 min-w-[200px] max-w-full"
                  autoFocus
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      const v = pathEditText.trim();
                      if (!v) {
                        setNavDir(-1);
                        setPath("");
                        setSelected({});
                        setEditingPath(false);
                        return;
                      }
                      if (isWindowsPath(v)) {
                        loadDir(normalizeWindowsPath(v));
                        setEditingPath(false);
                      }
                    } else if (e.key === "Escape") {
                      setEditingPath(false);
                    }
                  }}
                />
                <Button
                  size="sm"
                  color="primary"
                  variant="flat"
                  className="min-w-0 px-2"
                  onPress={() => {
                    const v = pathEditText.trim();
                    if (!v) {
                      setNavDir(-1);
                      setPath("");
                      setSelected({});
                      setEditingPath(false);
                      return;
                    }
                    if (isWindowsPath(v)) {
                      loadDir(normalizeWindowsPath(v));
                      setEditingPath(false);
                    }
                  }}
                >
                  {t("common.ok")}
                </Button>
                <Button
                  size="sm"
                  variant="light"
                  className="min-w-0 px-2"
                  onPress={() => setEditingPath(false)}
                >
                  {t("common.cancel")}
                </Button>
              </>
            ) : (
              <>
                {screenSize === "mobile" && (
                  <Button
                    size="sm"
                    variant="light"
                    className="text-xs px-2 min-w-0 shrink-0"
                    onPress={goUp}
                    isDisabled={
                      !path ||
                      path.replace(/\\+$/, "").split(/\\/).filter(Boolean)
                        .length <= 1
                    }
                  >
                    ⬆️
                  </Button>
                )}
                {breadcrumb.map((bc, idx) => (
                  <div
                    key={bc.full + idx}
                    className="flex items-center shrink-0"
                  >
                    <Button
                      size="sm"
                      variant="light"
                      className={`text-xs px-2 min-w-0 ${
                        screenSize === "mobile" ? "max-w-20" : "max-w-32"
                      }`}
                      onPress={() => {
                        if (bc.full === "") {
                          setNavDir(-1);
                          setPath("");
                          setSelected({});
                        } else {
                          loadDir(bc.full);
                        }
                      }}
                    >
                      {bc.name}
                    </Button>
                    {idx < breadcrumb.length - 1 && (
                      <div className="mx-0.5 text-default-400 select-none shrink-0">
                        ›
                      </div>
                    )}
                  </div>
                ))}
              </>
            )}
          </div>

          <div
            className="mt-3 flex-1 min-h-0 overflow-auto pr-1 pretty-scrollbar gutter-stable"
            ref={contentRef}
            onScroll={(e) =>
              setScrollTop((e.currentTarget as HTMLDivElement).scrollTop)
            }
          >
            <AnimatePresence mode="wait" initial={false}>
              <motion.div
                key={
                  (path || "root") +
                  "::" +
                  sortKey +
                  String(sortAsc) +
                  "::" +
                  query
                }
                initial={{ x: navDir * 40, opacity: 0 }}
                animate={{ x: 0, opacity: 1 }}
                exit={{ x: -navDir * 40, opacity: 0 }}
                transition={{ type: "tween", duration: 0.18 }}
                className="flex flex-col divide-y divide-default-200 rounded-medium overflow-hidden border border-default-200/60"
              >
                {(() => {
                  const drivesToShow = !path
                    ? drives.filter(
                        (d) =>
                          !query ||
                          d.toLowerCase().includes(query.toLowerCase())
                      )
                    : [];
                  const shouldShowEmpty = path
                    ? filtered.length === 0
                    : drivesToShow.length === 0;
                  return shouldShowEmpty ? (
                    <div className="py-10 text-center text-default-400">
                      {t("filemanager.empty")}
                    </div>
                  ) : null;
                })()}
                {(!path ? drives : [])
                  .filter(
                    (d) =>
                      !query || d.toLowerCase().includes(query.toLowerCase())
                  )
                  .map((d) => {
                    const active = path
                      .toLowerCase()
                      .startsWith(d.toLowerCase());
                    const st = driveStats[d];
                    const pct =
                      st && st.total
                        ? Math.round(((st.total - st.free) / st.total) * 100)
                        : undefined;
                    return (
                      <div
                        key={d}
                        className={`flex items-center gap-3 px-3 ${
                          screenSize === "mobile" ? "h-14" : "h-12"
                        } hover:bg-default-100/40 transition-colors`}
                        onClick={() => loadDir(d)}
                      >
                        <div
                          className={`rounded-lg flex items-center justify-center shrink-0 bg-default-200 ${
                            screenSize === "mobile" ? "w-8 h-8" : "w-7 h-7"
                          }`}
                        >
                          <div
                            className={`rounded bg-default-500 ${
                              screenSize === "mobile" ? "w-4 h-4" : "w-4 h-4"
                            }`}
                          ></div>
                        </div>
                        <div className="flex-1 min-w-0">
                          <div
                            className="font-medium leading-tight truncate text-sm"
                            title={d}
                          >
                            {d}
                          </div>
                          <div className="flex items-center gap-2">
                            <Chip
                              size="sm"
                              className={`text-tiny bg-foreground/10 text-foreground`}
                              variant="flat"
                            >
                              {t("filemanager.drive_label")}
                            </Chip>
                            {typeof pct === "number" && (
                              <span className="text-tiny text-default-500 whitespace-nowrap">
                                {pct}%
                              </span>
                            )}
                          </div>
                        </div>
                        <div className="text-default-400 text-sm">→</div>
                      </div>
                    );
                  })}
                {!!path &&
                  (virt ? (
                    <div
                      style={{ height: virt.totalHeight, position: "relative" }}
                    >
                      <div
                        style={{
                          position: "absolute",
                          top: virt.offsetY,
                          left: 0,
                          right: 0,
                        }}
                      >
                        {filtered.slice(virt.start, virt.end).map(renderRow)}
                      </div>
                    </div>
                  ) : (
                    filtered.map(renderRow)
                  ))}
              </motion.div>
            </AnimatePresence>
          </div>
        </section>
      </motion.div>
      <BaseModal
        size="sm"
        isOpen={mkdirOpen}
        onOpenChange={setMkdirOpen}
        hideCloseButton
      >
        <ModalContent>
          {(onClose) => (
            <>
              <BaseModalHeader className="text-primary-600">
                {t("filemanager.new_folder", { defaultValue: "新建文件夹" })}
              </BaseModalHeader>
              <BaseModalBody>
                <Input
                  size="sm"
                  variant="bordered"
                  value={mkdirName}
                  onValueChange={setMkdirName}
                  placeholder={
                    t("filemanager.new_folder_placeholder", {
                      defaultValue: "输入文件夹名称",
                    }) as string
                  }
                  autoFocus
                  onKeyDown={async (e) => {
                    if (e.key === "Enter") {
                      if (!path) return;
                      setCreatingFolder(true);
                      try {
                        const err = await (minecraft as any)?.CreateFolder?.(
                          path,
                          mkdirName.trim()
                        );
                        if (!err) {
                          await loadDir(path);
                          onClose();
                          setMkdirName("");
                        }
                      } finally {
                        setCreatingFolder(false);
                      }
                    }
                  }}
                />
              </BaseModalBody>
              <BaseModalFooter>
                <Button
                  variant="light"
                  onPress={onClose}
                  isDisabled={creatingFolder}
                >
                  {t("common.cancel", { defaultValue: "取消" })}
                </Button>
                <Button
                  color="primary"
                  isDisabled={!mkdirName.trim() || creatingFolder}
                  onPress={async () => {
                    if (!path) return;
                    setCreatingFolder(true);
                    try {
                      const err = await (minecraft as any)?.CreateFolder?.(
                        path,
                        mkdirName.trim()
                      );
                      if (!err) {
                        await loadDir(path);
                        onClose();
                        setMkdirName("");
                      }
                    } finally {
                      setCreatingFolder(false);
                    }
                  }}
                >
                  {t("common.confirm", { defaultValue: "确定" })}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>
    </motion.div>
  );
};

export default FileManagerPage;
