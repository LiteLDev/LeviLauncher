import { ModalAction, ModalPanel } from "@/components/ModalPrimitives";
import {
  Button,
  ButtonGroup,
  Card,
  Chip,
  CloseButton,
  Dropdown,
  InputGroup,
  Label,
  Spinner,
  TextField,
  Tooltip,
  useOverlayState,
} from "@heroui/react";
import React, { useEffect, useLayoutEffect, useMemo, useState, useRef } from "react";
import { UnifiedModal } from "@/components/UnifiedModal";
import { DeleteConfirmModal } from "@/components/DeleteConfirmModal";

("use client");

import {
  FaDownload,
  FaCopy,
  FaSync,
  FaSearch,
  FaTrash,
  FaBoxOpen,
  FaChevronDown, FaCircleNotch,
  FaCloudDownloadAlt
} from "react-icons/fa";
import { createPortal } from "react-dom";
import { useVersionStatus } from "@/utils/VersionStatusContext";
import { useLeviLamina } from "@/utils/LeviLaminaContext";
import { useTranslation } from "react-i18next";
import { motion } from "framer-motion";
import { useNavigate } from "react-router-dom";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import { isDownloadActive, useDownloads } from "@/utils/DownloadsContext";
import { notify } from "@/utils/activity";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { cn } from "@/utils/cn";
import { ROUTES } from "@/constants/routes";
import { useDownloadFilters } from "@/hooks/useDownloadFilters";
import { Clipboard } from "@wailsio/runtime";
import { normalizeVersionChannel, versionStatusKey, type PackageType, type VersionChannel } from "@/utils/packageType";

type ItemType = VersionChannel;

type VersionItem = {
  version: string;
  urls: string[];
  type: ItemType;
  short: string;
  timestamp?: number;
  md5?: string;
  packageType: PackageType;
  uuid?: string;
};

type DownloadIntent = "install" | "download";

// Rows rendered per step while scrolling.
const LIST_CHUNK = 60;

// Column template shared by the header and rows once the list is ~48rem wide.
const VERSION_ROW_COLUMNS =
  "@3xl:grid-cols-[minmax(8rem,1.4fr)_minmax(6rem,1fr)_minmax(7rem,1fr)_minmax(7rem,1fr)_minmax(10rem,auto)]";

// Kept across visits so returning from the install page restores the list.
const pageMemory = { query: "", scrollTop: 0, visibleCount: LIST_CHUNK };

export const DownloadPage: React.FC = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { startDownload, isDownloading, downloads } = useDownloads();
  const [items, setItems] = useState<VersionItem[]>([]);
  const [versionsLoading, setVersionsLoading] = useState(true);
  const [versionsError, setVersionsError] = useState(false);
  const [resolvingUWP, setResolvingUWP] = useState<Set<string>>(new Set());
  const {
    map: versionStatusMap,
    refreshAll,
    refreshOne,
    refreshing,
  } = useVersionStatus();

  const [query, setQuery] = useState(() => pageMemory.query);
  const {
    typeFilter,
    setTypeFilter,
    statusFilter,
    setStatusFilter,
    llFilter,
    setLlFilter,
    packageFilter,
    setPackageFilter,
  } = useDownloadFilters();
  const [visibleCount, setVisibleCount] = useState<number>(() =>
    Math.max(LIST_CHUNK, pageMemory.visibleCount),
  );
  const listScrollRef = useRef<HTMLDivElement>(null);
  const loadMoreRef = useRef<HTMLDivElement>(null);

  const {
    isOpen,
    open: onOpen,
    close: onClose,
    setOpen: onOpenChange,
  } = useOverlayState();

  const [mirrorUrls, setMirrorUrls] = useState<string[]>([]);
  const [mirrorVersion, setMirrorVersion] = useState<string>("");
  const [mirrorResults, setMirrorResults] = useState<
    { url: string; label: string; latencyMs: number | null; ok: boolean }[]
  >([]);
  const [testing, setTesting] = useState<boolean>(false);
  const [selectedUrl, setSelectedUrl] = useState<string | null>(null);
  const [mirrorType, setMirrorType] = useState<ItemType | null>(null);

  const tasksButtonRef = useRef<HTMLButtonElement>(null);
  const [flyingItems, setFlyingItems] = useState<
    {
      id: number;
      startX: number;
      startY: number;
      targetX: number;
      targetY: number;
    }[]
  >([]);

  const deleteDisclosure = useOverlayState();
  const [deleteItem, setDeleteItem] = useState<{
    short: string;
    type: ItemType;
    fileName: string;
    packageType: PackageType;
  } | null>(null);
  const [deleteError, setDeleteError] = useState<string>("");
  const [deleteLoading, setDeleteLoading] = useState<boolean>(false);

  const { isLLSupported, refreshLLDB } = useLeviLamina();

  const trErr = (msg: string, typeLabelOverride?: string): string => {
    const s = String(msg || "");
    if (!s) return "";
    if (s.startsWith("ERR_")) {
      const [code, ...restArr] = s.split(":");
      const codeTrim = code.trim();
      const rest = restArr.join(":").trim();
      const key = `errors.${codeTrim}`;
      const translated = t(key, {
        typeLabel:
          typeLabelOverride ||
          ((String(mirrorType || "Release") === "Preview"
            ? t("common.preview")
            : t("common.release")) as unknown as string),
      }) as unknown as string;
      if (translated && translated !== key) {
        return rest ? `${translated} (${rest})` : translated;
      }
      return s;
    }
    return s;
  };

  const bestMirror = useMemo(() => {
    if (!mirrorResults || mirrorResults.length === 0) return null;

    const measured = mirrorResults.filter(
      (m) => typeof m.latencyMs === "number",
    );
    if (measured.length === 0) return null;
    const okList = measured.filter((m) => m.ok);
    const list = (okList.length > 0 ? okList : measured).slice();
    list.sort(
      (a, b) =>
        (a.latencyMs ?? Number.MAX_SAFE_INTEGER) -
        (b.latencyMs ?? Number.MAX_SAFE_INTEGER),
    );
    return list[0] ?? null;
  }, [mirrorResults]);

  useEffect(() => {
    if (!testing && bestMirror && !selectedUrl) {
      setSelectedUrl(bestMirror.url);
    }
  }, [testing, bestMirror, selectedUrl]);

  const triggerAnimation = (e: any) => {
    try {
      const target = (e.currentTarget || e.target) as HTMLElement;
      const startRect = target?.getBoundingClientRect
        ? target.getBoundingClientRect()
        : null;
      const targetRect = tasksButtonRef.current?.getBoundingClientRect();

      if (startRect && targetRect) {
        const itemSize = 32;
        const startX = startRect.left + startRect.width / 2 - itemSize / 2;
        const startY = startRect.top + startRect.height / 2 - itemSize / 2;
        const targetX = targetRect.left + targetRect.width / 2 - itemSize / 2;
        const targetY = targetRect.top + targetRect.height / 2 - itemSize / 2;

        setFlyingItems((prev) => [
          ...prev,
          {
            id: Date.now(),
            startX,
            startY,
            targetX,
            targetY,
          },
        ]);
      }
    } catch (err) {
      console.error("Animation trigger failed", err);
    }
  };

  const startMirrorTests = async (urls: string[]) => {
    if (!urls || urls.length === 0) return;
    setMirrorResults(
      urls.map((u) => ({
        url: u,
        label: labelFromUrl(u),
        latencyMs: null,
        ok: false,
      })),
    );
    setTesting(true);
    try {
      if (minecraft?.TestMirrorLatencies) {
        const res = await minecraft.TestMirrorLatencies(urls, 7000);
        const byUrl = new Map<string, any>(
          (res || []).map((r: any) => [String(r.url), r]),
        );
        setMirrorResults((prev) =>
          prev.map((mr) => {
            const r = byUrl.get(mr.url);
            if (!r) return mr;
            return {
              ...mr,
              latencyMs: typeof r.latencyMs === "number" ? r.latencyMs : null,
              ok: Boolean(r.ok),
            };
          }),
        );
      } else {
        setMirrorResults((prev) =>
          prev.map((mr) => ({ ...mr, latencyMs: null, ok: false })),
        );
      }
    } finally {
      setTesting(false);
    }
  };

  const compareVersionDesc = (a: string, b: string) => {
    const pa = a.split(".").map((x) => parseInt(x, 10) || 0);
    const pb = b.split(".").map((x) => parseInt(x, 10) || 0);
    const len = Math.max(pa.length, pb.length);
    for (let i = 0; i < len; i++) {
      const va = pa[i] ?? 0;
      const vb = pb[i] ?? 0;
      if (va !== vb) return vb - va;
    }
    return 0;
  };

  const sanitizeUrl = (u: any): string =>
    String(u).trim().replace(/^`|`$/g, "");
  const labelFromUrl = (u: string): string => {
    try {
      const url = new URL(u);
      return url.hostname;
    } catch {
      return u;
    }
  };
  const normalizeUrls = (raw: any): string[] => {
    const arr = Array.isArray(raw) ? raw : raw ? [raw] : [];
    const cleaned = arr.map(sanitizeUrl).filter(Boolean);
    return cleaned.sort(
      (a, b) =>
        Number(b.includes("xboxlive.cn")) - Number(a.includes("xboxlive.cn")),
    );
  };

  const fileNameFromUrl = (u: string): string => {
    try {
      const url = new URL(u);
      const segs = url.pathname.split("/").filter(Boolean);
      return (
        segs[segs.length - 1] ||
        (t("downloadpage.mirror.filename_fallback") as unknown as string)
      );
    } catch {
      const segs = String(u).split("/").filter(Boolean);
      return (
        segs[segs.length - 1] ||
        (t("downloadpage.mirror.filename_fallback", {}) as unknown as string)
      );
    }
  };

  const fetchVersionItems = async (packageType = packageFilter): Promise<VersionItem[]> => {
    if (packageType === "all") {
      const catalogs = await Promise.all([fetchVersionItems("gdk"), fetchVersionItems("uwp")]);
      return catalogs.flat();
    }
    if (packageType === "uwp") {
      const data = await minecraft.FetchUWPVersions();
      return (data || []).map((v) => ({
        version: v.version, short: v.version,
        type: normalizeVersionChannel(v.type), packageType: "uwp", uuid: v.uuid, urls: [],
      }));
    }
    const preferCN =
      typeof minecraft?.IsMirrorEnabled === "function"
        ? await minecraft.IsMirrorEnabled()
        : false;
    const data = await minecraft.FetchHistoricalVersions(preferCN);
    return (["Preview", "Release"] as const).flatMap((type) =>
      ((type === "Preview" ? data.previewVersions : data.releaseVersions) || []).map((v: any) => ({
        version: v.version,
        short: String(v.version).replace(/^(?:Preview|Release)\s*/, ""),
        type, packageType: "gdk" as const, urls: normalizeUrls(v.urls ?? v.url), timestamp: v.timestamp, md5: v.md5,
      })),
    );
  };

  const cacheItems = (newItems: VersionItem[]) => {
    try {
      (window as any).__llVersionItemsCache = newItems;
      localStorage.setItem("ll.version_items", JSON.stringify(newItems));
      localStorage.setItem(`ll.version_items.${packageFilter}`, JSON.stringify(newItems));
    } catch { }
  };

  useEffect(() => {
    let cancelled = false;
    setVersionsLoading(true);
    setVersionsError(false);
    setItems([]);
    try {
      const cached = JSON.parse(localStorage.getItem(`ll.version_items.${packageFilter}`) || "[]");
      if (Array.isArray(cached)) setItems(cached);
    } catch { }
    void fetchVersionItems().then((newItems) => {
      if (cancelled) return;
      setItems(newItems);
      cacheItems(newItems);
      void refreshAll(newItems);
    }).catch((error) => {
      if (cancelled) return;
      console.error("Failed to fetch versions", error);
      setVersionsError(true);
    }).finally(() => {
      if (!cancelled) setVersionsLoading(false);
    });
    return () => { cancelled = true; };
  }, [packageFilter]);

  const reloadAll = async () => {
    if (versionsLoading) return;
    setVersionsLoading(true);
    setVersionsError(false);
    if (packageFilter !== "uwp") refreshLLDB();
    try {
      const newItems = await fetchVersionItems();
      setItems(newItems);
      cacheItems(newItems);
      await refreshAll(newItems);
    } catch (error) {
      console.error("reloadAll failed", error);
      setVersionsError(true);
    } finally {
      setVersionsLoading(false);
    }
  };

  const itemsWithStatus = useMemo(
    () =>
      items.map((it) => ({ ...it, _status: versionStatusMap.get(versionStatusKey(it.short, it.type, it.packageType)) })),
    [items, versionStatusMap],
  );

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return itemsWithStatus
      .filter((it) => (typeFilter === "all" ? true : it.type === typeFilter))
      .filter((it) =>
        statusFilter === "all"
          ? true
          : statusFilter === "downloaded"
            ? Boolean(it._status?.isDownloaded)
            : !Boolean(it._status?.isDownloaded),
      )
      .filter((it) =>
        llFilter === "all"
          ? true
          : llFilter === "levilamina"
            ? it.packageType === "gdk" && isLLSupported(it.short)
            : true,
      )
      .filter((it) =>
        q
          ? it.short.toLowerCase().includes(q) ||
          it.version.toLowerCase().includes(q) ||
          it.type.toLowerCase().includes(q)
          : true,
      )
      .sort((a, b) => {
        const ta = a.timestamp ?? 0;
        const tb = b.timestamp ?? 0;
        if (ta !== tb) return tb - ta;
        return compareVersionDesc(a.short, b.short);
      });
  }, [
    itemsWithStatus,
    query,
    typeFilter,
    statusFilter,
    llFilter,
    isLLSupported,
  ]);

  // The list scrolls and renders progressively instead of paging by
  // measured height, so small windows still reach every row.
  const visible = useMemo(
    () => filtered.slice(0, visibleCount),
    [filtered, visibleCount],
  );

  const filterKey = `${query}|${typeFilter}|${statusFilter}|${llFilter}|${packageFilter}`;
  const lastFilterKey = useRef(filterKey);
  useEffect(() => {
    if (lastFilterKey.current === filterKey) return;
    lastFilterKey.current = filterKey;
    setVisibleCount(LIST_CHUNK);
    listScrollRef.current?.scrollTo({ top: 0 });
  }, [filterKey]);

  useEffect(() => {
    pageMemory.query = query;
    pageMemory.visibleCount = visibleCount;
  }, [query, visibleCount]);

  // Restore the scroll position once rows exist (e.g. after returning from install).
  const scrollRestored = useRef(false);
  useLayoutEffect(() => {
    if (scrollRestored.current || visible.length === 0) return;
    scrollRestored.current = true;
    if (listScrollRef.current) listScrollRef.current.scrollTop = pageMemory.scrollTop;
  }, [visible.length]);

  useEffect(() => {
    const sentinel = loadMoreRef.current;
    const root = listScrollRef.current;
    if (!sentinel || !root || visibleCount >= filtered.length) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          setVisibleCount((count) => Math.min(filtered.length, count + LIST_CHUNK));
        }
      },
      { root, rootMargin: "0px 0px 400px 0px" },
    );
    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [visibleCount, filtered.length]);

  const activeDownloads = useMemo(() => {
    const byVersion = new Map<string, (typeof downloads)[number]>();
    downloads.forEach((dl) => {
      if (!dl.installer || !isDownloadActive(dl.status)) return;
      byVersion.set(
        versionStatusKey(dl.installer.version, normalizeVersionChannel(dl.installer.type), dl.installer.packageType || "gdk"),
        dl,
      );
    });
    return byVersion;
  }, [downloads]);
  const activeDownloadFor = (it: VersionItem) =>
    activeDownloads.get(versionStatusKey(it.short, it.type, it.packageType));

  // "Download & install": follow the download on the install page, where the
  // options can be set while it runs and installation continues afterwards.
  const followOnInstallPage = (dest: string, it: { short: string; type: ItemType; packageType: PackageType }) => {
    navigate(ROUTES.install, {
      state: {
        mirrorVersion: it.short,
        mirrorType: it.type,
        packageType: it.packageType,
        isLeviLaminaSupported: it.packageType === "gdk" && isLLSupported(it.short),
        pendingDownload: dest,
        returnTo: ROUTES.download,
      },
    });
  };

  const startUWPDownload = async (item: VersionItem, intent: DownloadIntent, e?: any) => {
    const key = versionStatusKey(item.short, item.type, item.packageType);
    setResolvingUWP((previous) => new Set(previous).add(key));
    try {
      const dest = await startDownload(`uwp:${item.uuid}`, `Minecraft-UWP-${item.type}-${item.short}.appx`, "", {
        version: item.short, type: item.type, packageType: "uwp", uuid: item.uuid, isLeviLaminaSupported: false,
      });
      if (!dest) return;
      if (intent === "install") followOnInstallPage(dest, item);
      else if (e) triggerAnimation(e);
    } finally {
      setResolvingUWP((previous) => { const next = new Set(previous); next.delete(key); return next; });
    }
  };

  const openMirrorPicker = (item: VersionItem) => {
    const urls = item.urls || [];
    setMirrorUrls(urls);
    setMirrorVersion(item.short);
    setMirrorType(item.type);
    setSelectedUrl(null);
    onOpen();
    startMirrorTests(urls);
  };

  const submitMirror = async (intent: DownloadIntent, e: any) => {
    if (!selectedUrl) return;
    const type = mirrorType || "Release";
    const desired = `${type} ${mirrorVersion}.msixvc`;
    const item = items.find(
      (i) => i.short === mirrorVersion && i.type === mirrorType && i.packageType === "gdk",
    );
    const dest = await startDownload(
      selectedUrl,
      desired,
      item?.md5,
      { version: mirrorVersion, type, isLeviLaminaSupported: isLLSupported(mirrorVersion) },
    );
    if (!dest) return;
    if (intent === "install") {
      onClose();
      followOnInstallPage(dest, { short: mirrorVersion, type, packageType: "gdk" });
    } else {
      triggerAnimation(e);
      onClose();
    }
  };

  const getVersionStatus = (it: VersionItem) => {
    return (
      versionStatusMap.get(versionStatusKey(it.short, it.type, it.packageType)) || {
        version: it.short,
        isInstalled: false,
        isDownloaded: false,
        type: it.type.toLowerCase(),
      }
    );
  };
  const hasStatus = (it: VersionItem) => versionStatusMap.has(versionStatusKey(it.short, it.type, it.packageType));

  const isDownloaded = (it: VersionItem) => getVersionStatus(it).isDownloaded;

  return (
    <>
      <PageContainer
        className={cn("relative", LAYOUT.CATALOG.PAGE)}
        animate={false}
      >
        <motion.div
          data-material-motion
          className="shrink-0"
          initial={{ opacity: 0, y: -20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.4 }}
        >
          <Card className={cn("flex-none gap-0 p-0", LAYOUT.GLASS_CARD.BASE)}>
            <Card.Content className="px-4 py-3">
              <div className="flex flex-wrap gap-3 items-center">
                <div className="flex min-w-0 flex-[1_1_14rem] items-center">
                  <TextField
                    aria-label={t("downloadpage.topcontent.input.placeholder")}
                    className={cn(
                      "group w-full min-w-0",
                      cn(COMPONENT_STYLES.input.mainWrapper, "h-full"),
                    )}
                    value={query}
                    onChange={setQuery}
                  >
                    <InputGroup
                      className={cn(
                        COMPONENT_STYLES.input.inputWrapper,
                        COMPONENT_STYLES.input.innerWrapper,
                        "rounded-full",
                      )}
                    >
                      <InputGroup.Prefix>
                        {<FaSearch size={14} className="text-muted" />}
                      </InputGroup.Prefix>
                      <InputGroup.Input
                        placeholder={t(
                          "downloadpage.topcontent.input.placeholder",
                        )}
                        className={cn(COMPONENT_STYLES.input.input, "text-sm")}
                      />
                      <InputGroup.Suffix>
                        {query && (
                          <CloseButton
                            aria-label={t("audit.primary.clear_search")}
                            onPress={() => setQuery("")}
                            className={COMPONENT_STYLES.input.clearButton}
                          />
                        )}
                      </InputGroup.Suffix>
                    </InputGroup>
                  </TextField>
                </div>
                <div className="flex min-w-0 flex-wrap items-center gap-2">
                  <Button
                    isDisabled={versionsLoading || refreshing}
                    onPress={async () => {
                      await reloadAll();
                    }}
                    variant={"secondary"}
                    isPending={versionsLoading || refreshing}
                    className={cn(
                      "rounded-full",
                      "bg-surface-secondary/50 text-foreground dark:text-zinc-200 font-medium hover:bg-surface-tertiary/50 transition-colors",
                    )}
                  >
                    {({ isPending }) => (
                      <>
                        <Spinner
                          size="sm"
                          color="current"
                          className={isPending ? "" : "hidden"}
                        />
                        {
                          <FaSync
                            className={refreshing ? "animate-spin" : ""}
                            size={14}
                          />
                        }
                        {t("common.refresh")}
                      </>
                    )}
                  </Button>
                  <Button
                    onPress={() =>
                      navigate(ROUTES.install, {
                        state: {
                          mirrorVersion: "",
                          mirrorType: "Release",
                          packageType: packageFilter === "uwp" ? "uwp" : "gdk",
                          returnTo: ROUTES.download,
                        },
                      })
                    }
                    variant={"secondary"}
                    className={cn(
                      "rounded-full",
                      "bg-surface-secondary/50 text-foreground dark:text-zinc-200 font-medium hover:bg-surface-tertiary/50 transition-colors",
                    )}
                  >
                    {t("audit.primary.local_install")}
                  </Button>
                  <Dropdown>
                    <Button variant="secondary" className="rounded-full" aria-label={t("uwp.package_type")} isDisabled={versionsLoading}>
                      {packageFilter === "all" ? t("downloadpage.topcontent.types_all") : packageFilter.toUpperCase()}
                    </Button>
                    <Dropdown.Popover className={COMPONENT_STYLES.dropdown.content}>
                      <Dropdown.Menu selectionMode="single" disallowEmptySelection selectedKeys={[packageFilter]}
                        onSelectionChange={(keys) => {
                          if (versionsLoading) return;
                          const next = Array.from(keys)[0];
                          if (next === "all" || next === "gdk" || next === "uwp") {
                            setPackageFilter(next);
                            if (next === "uwp") setLlFilter("all");
                            if (next === "gdk" && typeFilter === "Beta") setTypeFilter("Release");
                          }
                        }}>
                        <Dropdown.Item id="all" textValue={t("downloadpage.topcontent.types_all")}><Label>{t("downloadpage.topcontent.types_all")}</Label><Dropdown.ItemIndicator /></Dropdown.Item>
                        <Dropdown.Item id="gdk" textValue="GDK"><Label>GDK</Label><Dropdown.ItemIndicator /></Dropdown.Item>
                        <Dropdown.Item id="uwp" textValue="UWP"><Label>UWP</Label><Dropdown.ItemIndicator /></Dropdown.Item>
                      </Dropdown.Menu>
                    </Dropdown.Popover>
                  </Dropdown>
                  <Dropdown>
                    <Button
                      variant={"secondary"}
                      className={cn(
                        "rounded-full",
                        "bg-surface-secondary/50 text-foreground dark:text-zinc-200 font-medium shrink-0 hover:bg-surface-tertiary/50 transition-colors",
                      )}
                    >
                      {t("downloadpage.topcontent.types")}
                    </Button>
                    <Dropdown.Popover
                      className={COMPONENT_STYLES.dropdown.content}
                    >
                      <Dropdown.Menu
                        disallowEmptySelection
                        selectionMode="single"
                        selectedKeys={[typeFilter]}
                        onSelectionChange={(keys) => {
                          const k = Array.from(keys)[0] as "all" | ItemType;
                          if (k) setTypeFilter(k);
                        }}
                      >
                        <Dropdown.Item
                          key="all"
                          id={"all"}
                          textValue={t("downloadpage.topcontent.types_all")}
                        >
                          <Label>
                            {t("downloadpage.topcontent.types_all")}
                          </Label>
                          <Dropdown.ItemIndicator />
                        </Dropdown.Item>
                        <Dropdown.Item
                          key="Release"
                          id={"Release"}
                          textValue={t(
                            "downloadpage.customappx.modal.1.body.select.item1",
                          )}
                        >
                          <Label>
                            {t(
                              "downloadpage.customappx.modal.1.body.select.item1",
                            )}
                          </Label>
                          <Dropdown.ItemIndicator />
                        </Dropdown.Item>
                        <Dropdown.Item id="Beta" textValue={t("uwp.beta")} isDisabled={packageFilter === "gdk"}>
                          <Label>{t("uwp.beta")}</Label><Dropdown.ItemIndicator />
                        </Dropdown.Item>
                        <Dropdown.Item
                          key="Preview"
                          id={"Preview"}
                          textValue={t(
                            "downloadpage.customappx.modal.1.body.select.item2",
                          )}
                        >
                          <Label>
                            {t(
                              "downloadpage.customappx.modal.1.body.select.item2",
                            )}
                          </Label>
                          <Dropdown.ItemIndicator />
                        </Dropdown.Item>
                      </Dropdown.Menu>
                    </Dropdown.Popover>
                  </Dropdown>
                  <Dropdown>
                    <Button
                      variant={"secondary"}
                      className={cn(
                        "rounded-full",
                        "bg-surface-secondary/50 text-foreground dark:text-zinc-200 font-medium shrink-0 hover:bg-surface-tertiary/50 transition-colors",
                      )}
                    >
                      {t("downloadpage.topcontent.status")}
                    </Button>
                    <Dropdown.Popover
                      className={COMPONENT_STYLES.dropdown.content}
                    >
                      <Dropdown.Menu
                        disallowEmptySelection
                        selectionMode="single"
                        selectedKeys={[statusFilter]}
                        onSelectionChange={(keys) => {
                          const k = Array.from(keys)[0] as
                            | "all"
                            | "downloaded"
                            | "not_downloaded";
                          if (k) setStatusFilter(k);
                        }}
                      >
                        <Dropdown.Item
                          key="all"
                          id={"all"}
                          textValue={t("downloadpage.topcontent.status_all")}
                        >
                          <Label>
                            {t("downloadpage.topcontent.status_all")}
                          </Label>
                          <Dropdown.ItemIndicator />
                        </Dropdown.Item>
                        <Dropdown.Item
                          key="downloaded"
                          id={"downloaded"}
                          textValue={t(
                            "downloadpage.topcontent.status_downloaded",
                          )}
                        >
                          <Label>
                            {t("downloadpage.topcontent.status_downloaded")}
                          </Label>
                          <Dropdown.ItemIndicator />
                        </Dropdown.Item>
                        <Dropdown.Item
                          key="not_downloaded"
                          id={"not_downloaded"}
                          textValue={t(
                            "downloadpage.topcontent.status_not_downloaded",
                          )}
                        >
                          <Label>
                            {t("downloadpage.topcontent.status_not_downloaded")}
                          </Label>
                          <Dropdown.ItemIndicator />
                        </Dropdown.Item>
                      </Dropdown.Menu>
                    </Dropdown.Popover>
                  </Dropdown>
                  <Dropdown>
                    <Button
                      variant={"secondary"}
                      className={cn(
                        "rounded-full",
                        "bg-surface-secondary/50 text-foreground dark:text-zinc-200 font-medium shrink-0 hover:bg-surface-tertiary/50 transition-colors",
                      )}
                    >
                      {t("downloadpage.topcontent.loader")}
                    </Button>
                    <Dropdown.Popover
                      className={COMPONENT_STYLES.dropdown.content}
                    >
                      <Dropdown.Menu
                        disallowEmptySelection
                        selectionMode="single"
                        selectedKeys={new Set([llFilter])}
                        onSelectionChange={(keys) => {
                          const k = Array.from(keys)[0] as "all" | "levilamina";
                          if (k) setLlFilter(k);
                        }}
                      >
                        <Dropdown.Item
                          key="all"
                          id={"all"}
                          textValue={t("downloadpage.topcontent.status_all")}
                        >
                          <Label>
                            {t("downloadpage.topcontent.status_all")}
                          </Label>
                          <Dropdown.ItemIndicator />
                        </Dropdown.Item>
                        <Dropdown.Item
                          key="levilamina"
                          id={"levilamina"}
                          textValue={"LeviLamina"}
                          isDisabled={packageFilter === "uwp"}
                        >
                          <Label>LeviLamina</Label>
                          <Dropdown.ItemIndicator />
                        </Dropdown.Item>
                      </Dropdown.Menu>
                    </Dropdown.Popover>
                  </Dropdown>

                  <div className="h-6 w-px bg-surface-quaternary mx-1" />

                  <Tooltip>
                    <Button
                      ref={tasksButtonRef}
                      aria-label={t("download_manager.title")}
                      isIconOnly
                      onPress={() => navigate(ROUTES.downloadTasks)}
                      variant={isDownloading ? "primary" : "secondary"}
                      className={cn(
                        "rounded-full",
                        `transition-all ${isDownloading
                          ? "bg-brand-500 brand-primary-foreground"
                          : "bg-surface-secondary/50 text-foreground dark:text-zinc-200 hover:bg-surface-tertiary/50 "
                        }`,
                      )}
                    >
                      <motion.div
                        animate={isDownloading ? { y: [0, -2, 0] } : {}}
                        transition={{
                          repeat: Infinity,
                          duration: 1.5,
                          ease: "easeInOut",
                        }}
                      >
                        <FaCloudDownloadAlt size={20} />
                      </motion.div>
                      {isDownloading && (
                        <span className="absolute -top-1 -right-1 w-3 h-3 bg-red-500 rounded-full border-2 border-white dark:border-zinc-900 animate-pulse" />
                      )}
                    </Button>
                    <Tooltip.Content>
                      {t("download_manager.title")}
                    </Tooltip.Content>
                  </Tooltip>
                </div>
              </div>
            </Card.Content>
          </Card>
        </motion.div>

        <motion.div
          data-material-motion
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.4, ease: "easeOut" }}
          className="flex-1 min-h-0 flex flex-col"
        >
          <Card className={cn("flex-1 min-h-0 gap-0 p-0 overflow-hidden", LAYOUT.GLASS_CARD.BASE)}>
            <Card.Content className="p-0 flex flex-1 min-h-0 flex-col overflow-hidden relative">
              {versionsError && items.length > 0 && (
                <div role="alert" className="flex shrink-0 flex-wrap items-center justify-between gap-3 p-3 text-sm bg-warning/10 text-foreground">
                  <span>{t("audit.primary.download.stale")}</span>
                  <Button size="sm" variant="secondary" onPress={reloadAll} isDisabled={versionsLoading}>{t("download_manager.actions.retry")}</Button>
                </div>
              )}
              {/* Rows are a grid table on wide panels and stacked cards below ~48rem. */}
              <div
                ref={listScrollRef}
                onScroll={(e) => { pageMemory.scrollTop = e.currentTarget.scrollTop; }}
                className="@container flex-1 min-h-0 overflow-y-auto overflow-x-hidden"
              >
                <div
                  aria-hidden="true"
                  className={cn(
                    "hidden @3xl:grid sticky top-0 z-10 gap-3 px-4 py-2.5 text-xs font-semibold text-muted border-b border-border dark:border-white/10",
                    VERSION_ROW_COLUMNS,
                    COMPONENT_STYLES.tableSticky.thead,
                  )}
                >
                  <span>{t("downloadpage.table.header.version")}</span>
                  <span>{t("downloadpage.table.header.type")}</span>
                  <span>{t("downloadpage.table.header.status")}</span>
                  <span>{t("downloadpage.table.header.loader")}</span>
                  <span className="text-right">{t("downloadpage.table.header.actions")}</span>
                </div>
                {visible.length === 0 ? (
                  <div className="flex flex-col items-center justify-center py-10 text-muted gap-2">
                    {versionsLoading ? <Spinner /> : <FaBoxOpen className="w-10 h-10 opacity-20" />}
                    <p role={versionsError ? "alert" : "status"}>{t(versionsLoading ? "common.loading" : versionsError ? "audit.primary.download.failed" : items.length > 0 ? "audit.primary.download.no_matches" : "downloadpage.table.empty")}</p>
                    {!versionsLoading && (versionsError || items.length === 0 ? (
                      <Button size="sm" variant="secondary" onPress={reloadAll}>{t("download_manager.actions.retry")}</Button>
                    ) : (
                      <Button size="sm" variant="secondary" onPress={() => { setQuery(""); setTypeFilter("all"); setStatusFilter("all"); setLlFilter("all"); }}>{t("audit.primary.clear_filters")}</Button>
                    ))}
                  </div>
                ) : (
                  <ul
                    aria-label={t("downloadpage.table.aria_label") as unknown as string}
                    className="flex flex-col"
                  >
                    {visible.map((item) => {
                      const key = versionStatusKey(item.short, item.type, item.packageType);
                      const resolving = resolvingUWP.has(key);
                      const checking = !hasStatus(item) && refreshing;
                      const active = activeDownloadFor(item);
                      const activePercent =
                        active?.progress && active.progress.total > 0
                          ? Math.floor((active.progress.downloaded / active.progress.total) * 100)
                          : null;
                      return (
                        <li
                          key={key}
                          data-version-row
                          className={cn(
                            "grid items-center gap-x-3 gap-y-1.5 px-3 py-2.5 @3xl:px-4 border-b border-border/50 dark:border-white/5 last:border-0 transition-colors hover:bg-surface/50 dark:hover:bg-surface-secondary/30",
                            "grid-cols-[minmax(0,1fr)_auto]",
                            VERSION_ROW_COLUMNS,
                          )}
                        >
                          <span className="col-start-1 row-start-1 min-w-0 truncate text-sm font-medium @3xl:font-normal text-foreground dark:text-white">
                            {item.short}
                            {packageFilter === "all" && (
                              <span className="ml-2 text-xs text-muted font-mono">{item.packageType.toUpperCase()}</span>
                            )}
                          </span>
                          <div className="col-start-1 row-start-2 flex min-w-0 flex-wrap items-center gap-1.5 @3xl:contents">
                            <div className="@3xl:row-start-1">
                              <Chip
                                size="sm"
                                variant="soft"
                                color={item.type === "Release" ? "success" : "warning"}
                                className={"font-medium"}
                              >
                                <Chip.Label>
                                  {item.type === "Beta" ? t("uwp.beta") : item.type === "Release"
                                    ? t("downloadpage.table.type.release")
                                    : t("downloadpage.table.type.preview")}
                                </Chip.Label>
                              </Chip>
                            </div>
                            <div className="@3xl:row-start-1">
                              {checking ? (
                                <div className="flex items-center gap-2 text-muted">
                                  <FaCircleNotch className="animate-spin text-xs" />
                                  <span className="text-sm">
                                    {t("downloadpage.status.checking")}
                                  </span>
                                </div>
                              ) : isDownloaded(item) ? (
                                <Chip size="sm" variant="soft" color={"success"} className={"gap-1 px-2"}>
                                  <Chip.Label>{t("downloadpage.status.downloaded")}</Chip.Label>
                                </Chip>
                              ) : (
                                <Chip size="sm" variant="soft" color={"danger"} className={"gap-1 px-2"}>
                                  <Chip.Label>{t("downloadpage.status.not_downloaded")}</Chip.Label>
                                </Chip>
                              )}
                            </div>
                            <div className="@3xl:row-start-1">
                              {item.packageType === "gdk" && isLLSupported(item.short) ? (
                                <div className="flex items-center gap-1.5 text-brand-600 dark:text-brand-400 bg-brand-100/50 dark:bg-brand-900/20 px-2 py-0.5 rounded-lg w-fit">
                                  <span className="text-xs @3xl:text-sm">LeviLamina</span>
                                </div>
                              ) : (
                                <span className="hidden @3xl:inline text-muted dark:text-zinc-600 ml-2">-</span>
                              )}
                            </div>
                          </div>
                          <div className="col-start-2 row-start-1 row-span-2 @3xl:col-start-5 @3xl:row-span-1 flex justify-end">
                            {active ? (
                              <Button
                                size="sm"
                                variant="secondary"
                                className="rounded-full h-8 min-w-[7.5rem] font-medium"
                                onPress={() => followOnInstallPage(active.dest, item)}
                              >
                                <FaCircleNotch className="animate-spin" size={12} />
                                {activePercent === null
                                  ? t("common.downloading")
                                  : t("download_flow.row.downloading", { percent: activePercent })}
                              </Button>
                            ) : isDownloaded(item) ? (
                              <ButtonGroup
                                size="sm"
                                variant={"secondary"}
                                className={cn("rounded-full", "bg-transparent")}
                              >
                                <Button
                                  onPress={() => {
                                    navigate(ROUTES.install, {
                                      state: {
                                        mirrorVersion: item.short,
                                        mirrorType: item.type,
                                        packageType: item.packageType,
                                        returnTo: ROUTES.download,
                                        isLeviLaminaSupported: item.packageType === "gdk" && isLLSupported(item.short),
                                      },
                                    });
                                  }}
                                  variant={"secondary"}
                                  className={
                                    "px-3 h-8 font-medium text-foreground dark:text-zinc-200 bg-surface-secondary dark:bg-surface-tertiary/50 min-w-[5.5rem]"
                                  }
                                >
                                  {<FaBoxOpen size={14} />}
                                  {t("downloadpage.mirror.install_button")}
                                </Button>
                                <Dropdown>
                                  <Button
                                    isIconOnly
                                    aria-label={t("downloadpage.table.header.actions")}
                                    variant={"secondary"}
                                    className={
                                      "h-8 min-w-8 w-8 px-0 bg-surface-secondary dark:bg-surface-tertiary/50"
                                    }
                                  >
                                    <FaChevronDown size={12} />
                                  </Button>
                                  <Dropdown.Popover className={COMPONENT_STYLES.dropdown.content}>
                                    <Dropdown.Menu
                                      aria-label={t("downloadpage.table.header.actions")}
                                      onAction={async (actionKey) => {
                                        if (String(actionKey) !== "delete_msixvc") return;
                                        setDeleteError("");
                                        setDeleteLoading(false);
                                        let fname = "";
                                        try {
                                          if (typeof minecraft?.ResolveDownloadedMsixvc === "function") {
                                            fname = item.packageType === "uwp"
                                              ? await minecraft.ResolveDownloadedUWP(item.short, item.type.toLowerCase())
                                              : await minecraft.ResolveDownloadedMsixvc(
                                                `${item.type} ${item.short}`,
                                                String(item.type).toLowerCase(),
                                              );
                                          }
                                        } catch { }
                                        setDeleteItem({
                                          short: item.short,
                                          type: item.type,
                                          packageType: item.packageType,
                                          fileName: fname || `${item.type} ${item.short}`,
                                        });
                                        deleteDisclosure.open();
                                      }}
                                    >
                                      <Dropdown.Item
                                        key="delete_msixvc"
                                        id={"delete_msixvc"}
                                        textValue={t("downloadpage.actions.delete_installer")}
                                        variant="danger"
                                      >
                                        {<FaTrash size={12} />}
                                        <Label>{t("downloadpage.actions.delete_installer")}</Label>
                                        <Dropdown.ItemIndicator />
                                      </Dropdown.Item>
                                    </Dropdown.Menu>
                                  </Dropdown.Popover>
                                </Dropdown>
                              </ButtonGroup>
                            ) : (
                              <ButtonGroup
                                size="sm"
                                variant={"secondary"}
                                className={cn("rounded-full", "bg-transparent")}
                              >
                                <Button
                                  isDisabled={checking || resolving}
                                  isPending={resolving}
                                  aria-label={`${t("download_flow.download_install")} ${item.type} ${item.short} ${item.packageType.toUpperCase()}`}
                                  onPress={() => {
                                    if (item.packageType === "uwp") void startUWPDownload(item, "install");
                                    else openMirrorPicker(item);
                                  }}
                                  variant={"secondary"}
                                  className={
                                    "px-3 h-8 font-medium bg-surface-secondary dark:bg-surface-tertiary/50 text-foreground dark:text-zinc-200 hover:bg-surface-tertiary dark:hover:bg-surface-quaternary transition-all min-w-[7.5rem]"
                                  }
                                >
                                  {<FaDownload size={14} />}
                                  {resolving
                                    ? t("uwp.resolving_download")
                                    : checking
                                      ? t("downloadpage.status.checking")
                                      : t("download_flow.download_install")}
                                </Button>
                                {item.packageType === "uwp" && (
                                  <Dropdown>
                                    <Button
                                      isIconOnly
                                      isDisabled={checking || resolving}
                                      aria-label={`${t("download_flow.more_options")} ${item.short}`}
                                      variant={"secondary"}
                                      className={
                                        "h-8 min-w-8 w-8 px-0 bg-surface-secondary dark:bg-surface-tertiary/50"
                                      }
                                    >
                                      <FaChevronDown size={12} />
                                    </Button>
                                    <Dropdown.Popover className={COMPONENT_STYLES.dropdown.content}>
                                      <Dropdown.Menu
                                        aria-label={t("download_flow.more_options")}
                                        onAction={(actionKey) => {
                                          if (String(actionKey) === "download_only") {
                                            void startUWPDownload(item, "download");
                                          }
                                        }}
                                      >
                                        <Dropdown.Item id="download_only" textValue={t("download_flow.download_only")}>
                                          {<FaCloudDownloadAlt size={12} />}
                                          <Label>{t("download_flow.download_only")}</Label>
                                        </Dropdown.Item>
                                      </Dropdown.Menu>
                                    </Dropdown.Popover>
                                  </Dropdown>
                                )}
                              </ButtonGroup>
                            )}
                          </div>
                        </li>
                      );
                    })}
                  </ul>
                )}
                {visibleCount < filtered.length && (
                  <div ref={loadMoreRef} className="flex justify-center py-3">
                    <Button size="sm" variant="ghost" onPress={() => setVisibleCount((count) => count + LIST_CHUNK)}>
                      {t("download_flow.show_more", { count: filtered.length - visibleCount })}
                    </Button>
                  </div>
                )}
              </div>
              <div className="flex items-center justify-between gap-3 px-4 py-2 border-t border-border dark:border-white/10 bg-transparent shrink-0 z-10">
                <div className="text-sm text-muted dark:text-zinc-400 whitespace-nowrap">
                  {t("downloadpage.bottomcontent.total", {
                    count: filtered.length,
                  })}
                </div>
              </div>
            </Card.Content>
          </Card>
        </motion.div>

        <UnifiedModal
          isOpen={isOpen}
          onOpenChange={onOpenChange}
          size="wide"
          scrollBehavior="inside"
          type="primary"
          title={t("downloadpage.mirror.title")}
          icon={<FaCloudDownloadAlt size={24} />}
          hideScrollbar={true}
          showConfirmButton={false}
          showCancelButton={false}
          footer={
            <div className="flex w-full flex-wrap justify-end gap-2">
              <ModalAction
                onPress={onClose}
                variant="secondary"
              >
                {t("common.cancel")}
              </ModalAction>
              <ModalAction
                onPress={() => {
                  startMirrorTests(mirrorUrls || []);
                }}
                variant={"secondary"}
              >
                {<FaSync className={testing ? "animate-spin" : ""} />}
                {t("downloadpage.mirror.retest")}
              </ModalAction>
              <ModalAction
                isDisabled={!selectedUrl}
                onPress={(e) => submitMirror("download", e)}
                variant={"secondary"}
              >
                <FaCloudDownloadAlt />
                {t("download_flow.download_only")}
              </ModalAction>
              <ModalAction
                size="lg"
                isDisabled={!selectedUrl}
                onPress={(e) => submitMirror("install", e)}
                variant={"primary"}
              >
                <FaDownload />
                {t("download_flow.download_install")}
              </ModalAction>
            </div>
          }
        >
          <div className="flex flex-col gap-6">
            <ModalPanel className="flex flex-col gap-4">
              <div className="flex items-center gap-2">
                <Chip
                  size="sm"
                  variant="soft"
                  color={mirrorType === "Preview" ? "warning" : "success"}
                  className={"h-6"}
                >
                  <Chip.Label>{mirrorType}</Chip.Label>
                </Chip>
                <span className="text-sm font-mono text-foreground dark:text-zinc-400">
                  {mirrorVersion}
                </span>
              </div>
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                <div className="flex items-center gap-2 text-sm font-bold text-foreground dark:text-zinc-300 shrink-0">
                  <div className="w-1 h-4 rounded-full bg-brand-500"></div>
                  {t("downloadpage.mirror.target")}
                </div>
                {(() => {
                  const target =
                    selectedUrl || (!testing ? bestMirror?.url : "");
                  if (!target)
                    return (
                      <div className="text-sm text-muted dark:text-zinc-500 italic">
                        {testing
                          ? t("downloadpage.mirror.testing")
                          : t("downloadpage.mirror.unselected")}
                      </div>
                    );
                  const domain = labelFromUrl(target);
                  const fname = fileNameFromUrl(target);
                  return (
                    <div className="flex items-center gap-3 min-w-0 bg-surface/50 dark:bg-surface/20 rounded-xl px-3 py-1.5 border border-black/5 dark:border-white/5">
                      <div className="text-sm truncate min-w-0 max-w-[25rem] text-foreground dark:text-zinc-300">
                        <span className="font-semibold text-brand-600 dark:text-brand-500">
                          {domain}
                        </span>
                        <span className="mx-1.5 opacity-30">|</span>
                        {fname}
                      </div>
                      <Button
                        size="sm"
                        onPress={async () => {
                          try {
                            await Clipboard.SetText(target);
                            notify({ variant: "success", source: "downloads", title: t("audit.mods.link_copied") });
                          } catch {
                            notify({ variant: "danger", source: "downloads", title: t("audit.mods.copy_failed") });
                          }
                        }}
                        variant={"secondary"}
                        className={
                          "h-7 min-w-20 bg-surface-tertiary/50 dark:bg-surface/10"
                        }
                      >
                        {<FaCopy size={12} />}
                        {t("downloadpage.mirror.copy_link")}
                      </Button>
                    </div>
                  );
                })()}
              </div>
            </ModalPanel>

            {mirrorUrls && mirrorUrls.length > 0 ? (
              <div className="flex flex-col gap-4">
                {/* Recommended Section */}
                <div>
                  <div className="flex items-center gap-3 mb-3 px-1">
                    <span className="text-sm font-bold text-foreground dark:text-zinc-300 uppercase tracking-wider">
                      {t("downloadpage.mirror.recommended")}
                    </span>
                    {testing && (
                      <Chip size="sm" variant="soft" color={"accent"}>
                        {<FaCircleNotch className="animate-spin" size={12} />}
                        <Chip.Label>
                          {t("downloadpage.mirror.auto_testing")}
                        </Chip.Label>
                      </Chip>
                    )}
                  </div>

                  {bestMirror ? (
                    <div
                      className={`group relative overflow-hidden flex items-center justify-between gap-4 rounded-2xl border-2 p-4 transition-all cursor-pointer ${selectedUrl === bestMirror.url
                        ? "border-brand-500 bg-brand-500/5 shadow-xl shadow-brand-500/10"
                        : "border-transparent bg-surface dark:bg-surface-secondary/50 hover:bg-surface-secondary "
                        }`}
                      onClick={() => setSelectedUrl(bestMirror.url)}
                    >
                      <div className="flex items-center gap-4 min-w-0 z-10">
                        <div
                          className={`w-10 h-10 rounded-full flex items-center justify-center shrink-0 ${selectedUrl === bestMirror.url
                            ? "bg-brand-500 brand-primary-foreground"
                            : "bg-surface-tertiary text-muted dark:text-zinc-400"
                            }`}
                        >
                          <FaDownload size={16} />
                        </div>
                        <div className="flex flex-col gap-0.5 min-w-0">
                          <div className="font-bold text-base text-foreground dark:text-white truncate">
                            {bestMirror.label}
                          </div>
                          <div className="text-xs text-muted dark:text-zinc-400 truncate font-mono opacity-70">
                            {bestMirror.url}
                          </div>
                        </div>
                      </div>
                      <div className="flex items-center gap-3 shrink-0 z-10">
                        <div className="flex flex-col items-end">
                          <span className="text-xs text-muted dark:text-zinc-400 font-medium">
                            {t("download_flow.latency")}
                          </span>
                          <span
                            className={`text-base font-bold ${bestMirror.ok ? "text-brand-500" : "text-rose-500"}`}
                          >
                            {typeof bestMirror.latencyMs === "number"
                              ? `${Math.round(bestMirror.latencyMs)}ms`
                              : "-"}
                          </span>
                        </div>
                        {selectedUrl === bestMirror.url && (
                          <motion.div
                            layoutId="selected-check"
                            className="text-brand-500"
                          >
                            <div className="w-6 h-6 rounded-full bg-brand-500 flex items-center justify-center">
                              <svg
                                className="w-3.5 h-3.5 brand-primary-foreground"
                                fill="none"
                                viewBox="0 0 24 24"
                                stroke="currentColor"
                              >
                                <path
                                  strokeLinecap="round"
                                  strokeLinejoin="round"
                                  strokeWidth="3"
                                  d="M5 13l4 4L19 7"
                                />
                              </svg>
                            </div>
                          </motion.div>
                        )}
                      </div>
                    </div>
                  ) : (
                    <div className="p-8 rounded-2xl border border-dashed border-border dark:border-zinc-700 flex flex-col items-center justify-center text-muted dark:text-zinc-400 gap-2">
                      <span>{t("downloadpage.mirror.no_recommended")}</span>
                    </div>
                  )}
                </div>

                {/* Others Section */}
                <div className="flex flex-col gap-2">
                  <div className="text-sm font-bold text-foreground dark:text-zinc-300 uppercase tracking-wider px-1">
                    {t("downloadpage.mirror.others")}
                  </div>
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                    {mirrorResults.map((m, i) => (
                      <div
                        key={`mirror-${i}`}
                        className={`relative flex items-center justify-between gap-3 rounded-xl border p-2.5 transition-all cursor-pointer ${selectedUrl === m.url
                          ? "border-brand-500/50 bg-brand-500/5"
                          : "border-border/50 dark:border-zinc-600 bg-surface/50 dark:bg-surface-tertiary/30 hover:bg-surface-secondary dark:hover:bg-surface-tertiary hover:border-border"
                          }`}
                        onClick={() => setSelectedUrl(m.url)}
                      >
                        <div className="flex items-center gap-3 min-w-0 overflow-hidden">
                          <div className="flex items-center justify-center w-6 h-6 rounded-full bg-surface-secondary dark:bg-surface-quaternary text-xs font-bold text-foreground dark:text-zinc-100 shrink-0">
                            {String.fromCharCode(65 + i)}
                          </div>
                          <div className="flex flex-col min-w-0">
                            <span className="text-sm font-bold text-foreground dark:text-zinc-100 truncate">
                              {m.label}
                            </span>
                          </div>
                        </div>
                        <div className="flex items-center gap-2 shrink-0">
                          <span
                            className={`text-xs font-bold ${typeof m.latencyMs === "number"
                              ? m.latencyMs < 100
                                ? "text-brand-500"
                                : m.latencyMs < 300
                                  ? "text-amber-500"
                                  : "text-rose-500"
                              : "text-muted"
                              }`}
                          >
                            {typeof m.latencyMs === "number"
                              ? `${Math.round(m.latencyMs)}ms`
                              : testing
                                ? "..."
                                : "-"}
                          </span>
                        </div>
                        {selectedUrl === m.url && (
                          <div className="absolute inset-0 border-2 border-brand-500 rounded-xl pointer-events-none" />
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            ) : (
              <div className="flex flex-col items-center justify-center py-12 text-muted dark:text-zinc-500">
                <div className="w-16 h-16 rounded-full bg-surface-secondary flex items-center justify-center mb-4">
                  <FaDownload size={24} className="opacity-50" />
                </div>
                <p>{t("downloadpage.mirror.no_mirrors")}</p>
              </div>
            )}
          </div>
        </UnifiedModal>

        {/* Delete confirm modal */}
        <DeleteConfirmModal
          isOpen={deleteDisclosure.isOpen}
          onOpenChange={deleteDisclosure.setOpen}
          title={t("downloadpage.delete.title")}
          description={t("downloadpage.delete.body")}
          itemName={
            /\.(msixvc|appx|msix|appxbundle|msixbundle)$/i.test(deleteItem?.fileName || "")
              ? deleteItem?.fileName
              : `${deleteItem?.fileName || ""}.${deleteItem?.packageType === "uwp" ? "appx" : "msixvc"}`
          }
          warning={t("downloadpage.delete.warning")}
          isPending={deleteLoading}
          error={deleteError ? trErr(deleteError) : null}
          onConfirm={async () => {
            if (!deleteItem) return false;
            const { fileName, short, type, packageType } = deleteItem;
            setDeleteError("");
            setDeleteLoading(true);
            try {
              const msg = packageType === "uwp"
                ? await minecraft.DeleteDownloadedUWP(short, type.toLowerCase())
                : await minecraft.DeleteDownloadedMsixvc(`${type} ${short}`, type.toLowerCase());
              if (msg) throw new Error(msg);
              await refreshOne(short, type.toLowerCase(), packageType).catch(() => {});
              const displayName = /\.(msixvc|appx|msix|appxbundle|msixbundle)$/i.test(fileName)
                ? fileName
                : `${fileName}.${packageType === "uwp" ? "appx" : "msixvc"}`;
              notify({ variant: "success", source: "downloads", title: t("downloadpage.delete.success_body"), description: displayName });
            } catch (error) {
              setDeleteError(error instanceof Error ? error.message : String(error));
              throw error;
            } finally {
              setDeleteLoading(false);
            }
          }}
        />

        {createPortal(
          flyingItems.map((item) => (
            <motion.div
              key={item.id}
              initial={{
                x: item.startX,
                y: item.startY,
                scale: 0.5,
                opacity: 0,
                rotate: 0,
              }}
              animate={{
                x: [item.startX, item.targetX],
                y: [item.startY, item.targetY],
                scale: [1, 0.5],
                opacity: [1, 0],
                rotate: 360,
              }}
              transition={{ duration: 1.2, ease: "easeInOut" }}
              onAnimationComplete={() =>
                setFlyingItems((prev) => prev.filter((i) => i.id !== item.id))
              }
              style={{
                position: "fixed",
                top: 0,
                left: 0,
                zIndex: 99999,
                pointerEvents: "none",
              }}
            >
              <div className="w-8 h-8 rounded-full bg-brand-500 brand-primary-foreground flex items-center justify-center">
                <FaCloudDownloadAlt size={14} />
              </div>
            </motion.div>
          )),
          document.body,
        )}
      </PageContainer>
    </>
  );
};
