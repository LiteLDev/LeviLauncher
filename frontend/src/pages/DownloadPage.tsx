"use client";
import React, { useEffect, useMemo, useState, useRef } from "react";
import { UnifiedModal } from "@/components/UnifiedModal";
import { DeleteConfirmModal } from "@/components/DeleteConfirmModal";
import {
  Button,
  Chip,
  Dropdown,
  DropdownItem,
  DropdownMenu,
  DropdownTrigger,
  Input,
  Pagination,
  Table,
  TableHeader,
  TableColumn,
  TableBody,
  TableRow,
  TableCell,
  Progress,
  Spinner,
  useDisclosure,
  Card,
  CardBody,
  ButtonGroup,
  Tooltip,
  addToast,
} from "@heroui/react";
import {
  FaDownload,
  FaCopy,
  FaSync,
  FaTrash,
  FaBoxOpen,
  FaChevronDown,
  FaTimes,
  FaList,
  FaCircleNotch,
  FaCloudDownloadAlt,
  FaServer,
} from "react-icons/fa";
import { Events } from "@wailsio/runtime";
import { createPortal } from "react-dom";
import { useVersionStatus } from "@/utils/VersionStatusContext";
import { useLeviLamina } from "@/utils/LeviLaminaContext";
import { useTranslation } from "react-i18next";
import { motion, Variants } from "framer-motion";
import { useNavigate } from "react-router-dom";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { useDownloads } from "@/utils/DownloadsContext";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { cn } from "@/utils/cn";

type ItemType = "Preview" | "Release";

type VersionItem = {
  version: string;
  urls: string[];
  type: ItemType;
  short: string;
  timestamp?: number;
  md5?: string;
};

export const DownloadPage: React.FC = () => {
  const { t, i18n } = useTranslation();
  const [isAnimating, setIsAnimating] = useState(true);
  const navigate = useNavigate();
  const { startDownload, isDownloading } = useDownloads();
  const [items, setItems] = useState<VersionItem[]>([]);
  const {
    map: versionStatusMap,
    refreshAll,
    refreshOne,
    markDownloaded,
    setCurrentDownloadingInfo,
    refreshing,
  } = useVersionStatus();

  const [query, setQuery] = useState("");
  const [typeFilter, setTypeFilter] = useState<"all" | ItemType>("all");
  const [statusFilter, setStatusFilter] = useState<
    "all" | "downloaded" | "not_downloaded"
  >("all");
  const [llFilter, setLlFilter] = useState<"all" | "levilamina">("all");
  const [rowsPerPage, setRowsPerPage] = useState<number>(6);
  const [page, setPage] = useState<number>(1);

  const { isOpen, onOpen, onClose, onOpenChange } = useDisclosure();

  const [extractInfo, setExtractInfo] = useState<{
    files: number;
    bytes: number;
    dir: string;
    totalBytes?: number;
    currentFile?: string;
  } | null>(null);
  const [extractError, setExtractError] = useState<string>("");
  const [mirrorUrls, setMirrorUrls] = useState<string[]>([]);
  const [mirrorVersion, setMirrorVersion] = useState<string>("");
  const [mirrorResults, setMirrorResults] = useState<
    { url: string; label: string; latencyMs: number | null; ok: boolean }[]
  >([]);
  const initialStatusFetchedRef = useRef(false);
  const [testing, setTesting] = useState<boolean>(false);
  const [selectedUrl, setSelectedUrl] = useState<string | null>(null);
  const [installMode, setInstallMode] = useState<boolean>(false);
  const [mirrorType, setMirrorType] = useState<ItemType | null>(null);

  const mirrorVersionRef = useRef(mirrorVersion);
  const mirrorTypeRef = useRef(mirrorType);

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

  useEffect(() => {
    mirrorVersionRef.current = mirrorVersion;
  }, [mirrorVersion]);
  useEffect(() => {
    mirrorTypeRef.current = mirrorType;
  }, [mirrorType]);

  const [installError, setInstallError] = useState<string>("");
  const installLoadingDisclosure = useDisclosure();
  const installErrorDisclosure = useDisclosure();
  const [installingVersion, setInstallingVersion] = useState<string>("");
  const [installingTargetName, setInstallingTargetName] = useState<string>("");
  const deleteDisclosure = useDisclosure();
  const [deleteItem, setDeleteItem] = useState<{
    short: string;
    type: ItemType;
    fileName: string;
  } | null>(null);
  const [deleteError, setDeleteError] = useState<string>("");
  const [deleteLoading, setDeleteLoading] = useState<boolean>(false);

  const { isLLSupported, refreshLLDB } = useLeviLamina();
  const hasBackend = minecraft !== undefined;

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

  const isChinaUser = useMemo(() => {
    try {
      const tz = Intl.DateTimeFormat().resolvedOptions().timeZone || "";
      const lang = String(
        i18n?.language || navigator.language || "",
      ).toLowerCase();
      const langs = (navigator.languages || []).map((l) =>
        String(l).toLowerCase(),
      );
      const isTzCN = tz === "Asia/Shanghai" || tz === "Asia/Urumqi";
      const isLangCN =
        lang.startsWith("zh-cn") || langs.includes("zh-cn") || lang === "zh";
      return isTzCN || isLangCN;
    } catch {
      return false;
    }
  }, [i18n?.language]);

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
      if (hasBackend && minecraft?.TestMirrorLatencies) {
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

  useEffect(() => {
    const fetchData = async () => {
      try {
        let data: any;
        if (
          hasBackend &&
          typeof minecraft?.FetchHistoricalVersions === "function"
        ) {
          data = await minecraft.FetchHistoricalVersions(Boolean(isChinaUser));
        } else {
          data = { previewVersions: [], releaseVersions: [] };
        }
        const preview: VersionItem[] = (data.previewVersions || []).map(
          (v: any) => ({
            version: v.version,
            urls: normalizeUrls(v.urls ?? v.url),
            type: "Preview",
            short: String(v.version).replace(/^Preview\s*/, ""),
            timestamp: v.timestamp,
            md5: v.md5,
          }),
        );
        const release: VersionItem[] = (data.releaseVersions || []).map(
          (v: any) => ({
            version: v.version,
            urls: normalizeUrls(v.urls ?? v.url),
            type: "Release",
            short: String(v.version).replace(/^Release\s*/, ""),
            timestamp: v.timestamp,
            md5: v.md5,
          }),
        );
        const newItems = [...preview, ...release];
        setItems(newItems);
        try {
          (window as any).__llVersionItemsCache = newItems;
          localStorage.setItem("ll.version_items", JSON.stringify(newItems));
        } catch {}
      } catch (e) {
        console.error("Failed to fetch versions", e);
      }
    };
    try {
      const raw = localStorage.getItem("ll.version_items");
      const cached: VersionItem[] = raw ? JSON.parse(raw) : [];
      if (cached && Array.isArray(cached) && cached.length > 0) {
        setItems(cached);
      }
    } catch {}
    fetchData();
  }, []);

  const reloadAll = async () => {
    refreshLLDB();
    try {
      let data: any;
      if (
        hasBackend &&
        typeof minecraft?.FetchHistoricalVersions === "function"
      ) {
        data = await minecraft.FetchHistoricalVersions(Boolean(isChinaUser));
      } else {
        data = { previewVersions: [], releaseVersions: [] };
      }
      const preview: VersionItem[] = (data.previewVersions || []).map(
        (v: any) => ({
          version: v.version,
          urls: normalizeUrls(v.urls ?? v.url),
          type: "Preview",
          short: String(v.version).replace(/^Preview\s*/, ""),
          timestamp: v.timestamp,
          md5: v.md5,
        }),
      );
      const release: VersionItem[] = (data.releaseVersions || []).map(
        (v: any) => ({
          version: v.version,
          urls: normalizeUrls(v.urls ?? v.url),
          type: "Release",
          short: String(v.version).replace(/^Release\s*/, ""),
          timestamp: v.timestamp,
          md5: v.md5,
        }),
      );
      const newItems = [...preview, ...release];
      setItems(newItems);
      try {
        (window as any).__llVersionItemsCache = newItems;
        localStorage.setItem("ll.version_items", JSON.stringify(newItems));
      } catch {}
      try {
        await refreshAll(newItems as any);
      } catch {}
    } catch (e) {
      console.error("reloadAll failed", e);
    }
  };

  useEffect(() => {
    if (!hasBackend) return;
    if (!initialStatusFetchedRef.current && items.length > 0) {
      initialStatusFetchedRef.current = true;
      refreshAll(items as any);
    }
  }, [hasBackend, items]);

  const itemsWithStatus = useMemo(
    () =>
      items.map((it) => ({ ...it, _status: versionStatusMap.get(it.short) })),
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
            ? isLLSupported(it.short)
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

  const totalPages = Math.max(1, Math.ceil(filtered.length / rowsPerPage));
  const paged = filtered.slice((page - 1) * rowsPerPage, page * rowsPerPage);

  useEffect(() => {
    setPage(1);
  }, [query, typeFilter, statusFilter, llFilter]);

  useEffect(() => {
    const calcRows = () => {
      const rowH = 56;
      const reserve = 280;
      const computed = Math.max(
        4,
        Math.floor((window.innerHeight - reserve) / rowH),
      );
      setRowsPerPage(computed);
    };
    calcRows();
    window.addEventListener("resize", calcRows);
    return () => window.removeEventListener("resize", calcRows);
  }, []);

  useEffect(() => {
    setPage(1);
  }, [rowsPerPage]);

  const getVersionStatus = (it: VersionItem) => {
    return (
      versionStatusMap.get(it.short) || {
        version: it.short,
        isInstalled: false,
        isDownloaded: false,
        type: it.type.toLowerCase(),
      }
    );
  };
  const hasStatus = (it: VersionItem) => versionStatusMap.has(it.short);

  const isDownloaded = (it: VersionItem) => getVersionStatus(it).isDownloaded;
  const isInstalled = (it: VersionItem) => getVersionStatus(it).isInstalled;

  const cardVariants: Variants = {
    hidden: { opacity: 0, y: 20 },
    visible: (i: number) => ({
      opacity: 1,
      y: 0,
      transition: {
        delay: i * 0.1,
        duration: 0.4,
        ease: "easeOut",
      },
    }),
  };

  const rowVariants: Variants = {
    hidden: { opacity: 0, y: 10 },
    visible: (i: number) => ({
      opacity: 1,
      y: 0,
      transition: {
        delay: i * 0.05,
        duration: 0.3,
        ease: "easeOut",
      },
    }),
  };

  return (
    <>
      <PageContainer
        className={cn("relative", isAnimating && "overflow-hidden")}
        animate={false}
      >
        <motion.div
          initial={{ opacity: 0, y: -20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.4 }}
        >
          <Card className={cn("flex-none", LAYOUT.GLASS_CARD.BASE)}>
            <CardBody className="p-4">
              <div className="flex flex-col sm:flex-row gap-4 justify-between items-center">
                <div className="flex items-center gap-3 w-full sm:max-w-md">
                  <Input
                    isClearable
                    radius="full"
                    classNames={{
                      base: "max-w-full sm:max-w-[20rem] h-10",
                      mainWrapper: "h-full",
                      input: "text-small",
                      ...COMPONENT_STYLES.input,
                    }}
                    placeholder={t("downloadpage.topcontent.input.placeholder")}
                    value={query}
                    onValueChange={setQuery}
                    startContent={
                      <FaSync size={14} className="text-default-400" />
                    }
                    onClear={() => setQuery("")}
                  />
                </div>
                <div className="flex items-center gap-2 shrink-0 overflow-x-auto w-full sm:w-auto pb-1 sm:pb-0">
                  <Button
                    radius="full"
                    variant="flat"
                    className="bg-default-100/50 dark:bg-zinc-800/50 text-default-600 dark:text-zinc-200 font-medium hover:bg-default-200/50 dark:hover:bg-zinc-700/50 transition-colors"
                    startContent={
                      <FaSync
                        className={refreshing ? "animate-spin" : ""}
                        size={14}
                      />
                    }
                    isDisabled={items.length === 0}
                    isLoading={refreshing}
                    onPress={async () => {
                      await reloadAll();
                    }}
                  >
                    {t("common.refresh")}
                  </Button>
                  <Button
                    radius="full"
                    variant="flat"
                    className="bg-default-100/50 dark:bg-zinc-800/50 text-default-600 dark:text-zinc-200 font-medium hover:bg-default-200/50 dark:hover:bg-zinc-700/50 transition-colors"
                    onPress={() =>
                      navigate("/install", {
                        state: {
                          mirrorVersion: "",
                          mirrorType: "Release",
                          returnTo: "/download",
                        },
                      })
                    }
                  >
                    {t("downloadpage.customappx.button")}
                  </Button>
                  <Dropdown classNames={COMPONENT_STYLES.dropdown}>
                    <DropdownTrigger>
                      <Button
                        radius="full"
                        variant="flat"
                        className="bg-default-100/50 dark:bg-zinc-800/50 text-default-600 dark:text-zinc-200 font-medium shrink-0 hover:bg-default-200/50 dark:hover:bg-zinc-700/50 transition-colors"
                      >
                        {t("downloadpage.topcontent.types")}
                      </Button>
                    </DropdownTrigger>
                    <DropdownMenu
                      disallowEmptySelection
                      selectionMode="single"
                      selectedKeys={[typeFilter]}
                      onSelectionChange={(keys) => {
                        const k = Array.from(keys)[0] as "all" | ItemType;
                        if (k) setTypeFilter(k);
                      }}
                    >
                      <DropdownItem key="all">
                        {t("downloadpage.topcontent.types_all")}
                      </DropdownItem>
                      <DropdownItem key="Release">
                        {t("downloadpage.customappx.modal.1.body.select.item1")}
                      </DropdownItem>
                      <DropdownItem key="Preview">
                        {t("downloadpage.customappx.modal.1.body.select.item2")}
                      </DropdownItem>
                    </DropdownMenu>
                  </Dropdown>
                  <Dropdown classNames={COMPONENT_STYLES.dropdown}>
                    <DropdownTrigger>
                      <Button
                        radius="full"
                        variant="flat"
                        className="bg-default-100/50 dark:bg-zinc-800/50 text-default-600 dark:text-zinc-200 font-medium shrink-0 hover:bg-default-200/50 dark:hover:bg-zinc-700/50 transition-colors"
                      >
                        {t("downloadpage.topcontent.status")}
                      </Button>
                    </DropdownTrigger>
                    <DropdownMenu
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
                      <DropdownItem key="all">
                        {t("downloadpage.topcontent.status_all")}
                      </DropdownItem>
                      <DropdownItem key="downloaded">
                        {t("downloadpage.topcontent.status_downloaded")}
                      </DropdownItem>
                      <DropdownItem key="not_downloaded">
                        {t("downloadpage.topcontent.status_not_downloaded")}
                      </DropdownItem>
                    </DropdownMenu>
                  </Dropdown>
                  <Dropdown classNames={COMPONENT_STYLES.dropdown}>
                    <DropdownTrigger>
                      <Button
                        radius="full"
                        variant="flat"
                        className="bg-default-100/50 dark:bg-zinc-800/50 text-default-600 dark:text-zinc-200 font-medium shrink-0 hover:bg-default-200/50 dark:hover:bg-zinc-700/50 transition-colors"
                      >
                        {t("downloadpage.topcontent.loader")}
                      </Button>
                    </DropdownTrigger>
                    <DropdownMenu
                      disallowEmptySelection
                      selectionMode="single"
                      selectedKeys={new Set([llFilter])}
                      onSelectionChange={(keys) => {
                        const k = Array.from(keys)[0] as "all" | "levilamina";
                        if (k) setLlFilter(k);
                      }}
                    >
                      <DropdownItem key="all">
                        {t("downloadpage.topcontent.status_all")}
                      </DropdownItem>
                      <DropdownItem key="levilamina">LeviLamina</DropdownItem>
                    </DropdownMenu>
                  </Dropdown>

                  <div className="h-6 w-px bg-default-300 mx-1" />

                  <Tooltip content={t("download_manager.title")}>
                    <Button
                      ref={tasksButtonRef}
                      isIconOnly
                      radius="full"
                      variant={isDownloading ? "solid" : "flat"}
                      color={isDownloading ? "success" : "default"}
                      className={`transition-all ${
                        isDownloading
                          ? "bg-primary-500 text-white"
                          : "bg-default-100/50 dark:bg-zinc-800/50 text-default-600 dark:text-zinc-200 hover:bg-default-200/50 dark:hover:bg-zinc-700/50"
                      }`}
                      onPress={() => navigate("/tasks")}
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
                  </Tooltip>
                </div>
              </div>
            </CardBody>
          </Card>
        </motion.div>

        <motion.div
          custom={0}
          initial="hidden"
          animate="visible"
          variants={cardVariants}
          className="flex-1 min-h-0 flex flex-col"
          onAnimationComplete={() => setIsAnimating(false)}
        >
          <Card className={cn("flex-1 min-h-0", LAYOUT.GLASS_CARD.BASE)}>
            <CardBody className="p-0 flex flex-col h-full overflow-hidden relative">
              <Table
                aria-label={
                  t("downloadpage.table.aria_label") as unknown as string
                }
                className="h-full"
                removeWrapper
                isHeaderSticky
                radius="none"
                shadow="none"
                classNames={{
                  base: "h-full overflow-y-auto custom-scrollbar",
                  table: "min-w-full",
                  thead: "rounded-none",
                  th: "bg-transparent backdrop-blur-lg text-default-500 dark:text-zinc-400 font-semibold border-b border-default-200 dark:border-white/10 h-12 rounded-none",
                  td: "py-3 border-b border-default-100 dark:border-white/5 group-last:border-0",
                  tr: "group transition-colors hover:bg-default-50/50 dark:hover:bg-zinc-800/30 data-[selected=true]:bg-default-100",
                }}
              >
                <TableHeader>
                  <TableColumn key="version" width={180}>
                    {t("downloadpage.table.header.version")}
                  </TableColumn>
                  <TableColumn key="type" width={140}>
                    {t("downloadpage.table.header.type")}
                  </TableColumn>
                  <TableColumn key="status" width={160}>
                    {t("downloadpage.table.header.status")}
                  </TableColumn>
                  <TableColumn key="loader" width={160}>
                    {t("downloadpage.table.header.loader")}
                  </TableColumn>
                  <TableColumn key="actions" align="end">
                    {t("downloadpage.table.header.actions")}
                  </TableColumn>
                </TableHeader>
                <TableBody
                  emptyContent={
                    <div className="flex flex-col items-center justify-center h-40 text-default-400 gap-2">
                      <FaBoxOpen className="w-10 h-10 opacity-20" />
                      <p>{t("downloadpage.table.empty")}</p>
                    </div>
                  }
                >
                  {paged.map((item, index) => (
                    <TableRow key={`${item.type}-${item.short}`}>
                      <TableCell>
                        <motion.div
                          custom={index}
                          initial="hidden"
                          animate="visible"
                          variants={rowVariants}
                        >
                          <span className="text-default-900 dark:text-white text-small">
                            {item.short}
                          </span>
                        </motion.div>
                      </TableCell>
                      <TableCell>
                        <motion.div
                          custom={index}
                          initial="hidden"
                          animate="visible"
                          variants={rowVariants}
                        >
                          <Chip
                            size="sm"
                            color={
                              item.type === "Release" ? "warning" : "secondary"
                            }
                            variant="flat"
                            className="font-medium"
                          >
                            {item.type === "Release"
                              ? t("downloadpage.table.type.release")
                              : t("downloadpage.table.type.preview")}
                          </Chip>
                        </motion.div>
                      </TableCell>
                      <TableCell>
                        <motion.div
                          custom={index}
                          initial="hidden"
                          animate="visible"
                          variants={rowVariants}
                        >
                          {!hasStatus(item) && refreshing ? (
                            <div className="flex items-center gap-2 text-default-400">
                              <FaCircleNotch className="animate-spin text-xs" />
                              <span className="text-small">
                                {t("downloadpage.status.checking")}
                              </span>
                            </div>
                          ) : isDownloaded(item) ? (
                            <Chip
                              color="success"
                              variant="flat"
                              size="sm"
                              className="gap-1 px-2"
                            >
                              {t("downloadpage.status.downloaded")}
                            </Chip>
                          ) : (
                            <Chip
                              color="danger"
                              variant="flat"
                              size="sm"
                              className="gap-1 px-2"
                            >
                              {t("downloadpage.status.not_downloaded")}
                            </Chip>
                          )}
                        </motion.div>
                      </TableCell>
                      <TableCell>
                        <motion.div
                          custom={index}
                          initial="hidden"
                          animate="visible"
                          variants={rowVariants}
                        >
                          {isLLSupported(item.short) ? (
                            <div className="flex items-center gap-1.5 text-primary-600 dark:text-primary-400 bg-primary-100/50 dark:bg-primary-900/20 px-2 py-1 rounded-lg w-fit">
                              <span className="text-small">LeviLamina</span>
                            </div>
                          ) : (
                            <span className="text-default-300 dark:text-zinc-600 ml-2">
                              -
                            </span>
                          )}
                        </motion.div>
                      </TableCell>
                      <TableCell>
                        <motion.div
                          custom={index}
                          initial="hidden"
                          animate="visible"
                          variants={rowVariants}
                          className="flex justify-end"
                        >
                          {isDownloaded(item) ? (
                            <ButtonGroup
                              radius="full"
                              size="sm"
                              variant="flat"
                              className="bg-transparent"
                            >
                              <Button
                                className="px-2 h-8 font-medium text-default-700 dark:text-zinc-200 bg-default-100 dark:bg-zinc-700/50 w-[88px]"
                                startContent={<FaBoxOpen size={14} />}
                                onPress={() => {
                                  navigate("/install", {
                                    state: {
                                      mirrorVersion: item.short,
                                      mirrorType: item.type,
                                      returnTo: "/download",
                                      isLeviLaminaSupported: isLLSupported(
                                        item.short,
                                      ),
                                    },
                                  });
                                }}
                              >
                                {t("downloadpage.mirror.install_button")}
                              </Button>
                              <Dropdown classNames={COMPONENT_STYLES.dropdown}>
                                <DropdownTrigger>
                                  <Button
                                    isIconOnly
                                    className="h-8 min-w-8 w-8 px-0 bg-default-100 dark:bg-zinc-700/50"
                                  >
                                    <FaChevronDown size={12} />
                                  </Button>
                                </DropdownTrigger>
                                <DropdownMenu
                                  aria-label="Actions"
                                  onAction={async (key) => {
                                    if (String(key) !== "delete_msixvc") return;
                                    setDeleteError("");
                                    setDeleteLoading(false);
                                    let fname = "";
                                    try {
                                      if (
                                        hasBackend &&
                                        typeof minecraft?.ResolveDownloadedMsixvc ===
                                          "function"
                                      ) {
                                        fname =
                                          await minecraft.ResolveDownloadedMsixvc(
                                            `${item.type} ${item.short}`,
                                            String(item.type).toLowerCase(),
                                          );
                                      }
                                    } catch {}
                                    setDeleteItem({
                                      short: item.short,
                                      type: item.type,
                                      fileName:
                                        fname || `${item.type} ${item.short}`,
                                    });
                                    deleteDisclosure.onOpen();
                                  }}
                                >
                                  <DropdownItem
                                    key="delete_msixvc"
                                    color="danger"
                                    startContent={<FaTrash size={12} />}
                                  >
                                    {t("downloadpage.actions.delete_installer")}
                                  </DropdownItem>
                                </DropdownMenu>
                              </Dropdown>
                            </ButtonGroup>
                          ) : (
                            <Button
                              radius="full"
                              size="sm"
                              startContent={<FaDownload size={14} />}
                              className="px-0 h-8 font-medium bg-default-100 dark:bg-zinc-700/50 text-default-700 dark:text-zinc-200 hover:bg-default-200 dark:hover:bg-zinc-600 transition-all w-[120px]"
                              isDisabled={!hasStatus(item) && refreshing}
                              onPress={() => {
                                if (isDownloading) {
                                  addToast({
                                    title: t(
                                      "downloadpage.error.already_downloading",
                                    ),
                                    color: "danger",
                                  });
                                  return;
                                }
                                const urls = item.urls || [];
                                setMirrorUrls(urls);
                                setMirrorVersion(item.short);
                                setMirrorType(item.type);
                                setSelectedUrl(null);
                                setInstallMode(false);
                                setCurrentDownloadingInfo(
                                  item.short,
                                  item.type,
                                );
                                onOpen();
                                startMirrorTests(urls);
                              }}
                            >
                              {!hasStatus(item) && refreshing
                                ? t("downloadpage.status.checking")
                                : t("downloadmodal.download_button")}
                            </Button>
                          )}
                        </motion.div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>

              {/* Footer Pagination */}
              <div className="flex items-center justify-between px-4 py-3 border-t border-default-200 dark:border-white/10 bg-transparent shrink-0 z-10">
                <div className="text-small text-default-500 dark:text-zinc-400">
                  {t("downloadpage.bottomcontent.total", {
                    count: filtered.length,
                  })}
                </div>
                <Pagination
                  total={totalPages}
                  page={page}
                  onChange={setPage}
                  radius="full"
                  showControls
                  size="sm"
                  variant="light"
                  classNames={{
                    cursor:
                      "bg-primary-600 hover:bg-primary-500 text-white font-bold shadow-lg shadow-primary-900/20",
                  }}
                />
              </div>
            </CardBody>
          </Card>
        </motion.div>

        <UnifiedModal
          isOpen={isOpen}
          onOpenChange={onOpenChange}
          size="2xl"
          scrollBehavior="inside"
          type="primary"
          title={t("downloadpage.mirror.title")}
          icon={<FaCloudDownloadAlt size={24} className="text-primary-500" />}
          hideScrollbar={true}
          showConfirmButton={false}
          showCancelButton={false}
          footer={
            <div className="flex w-full justify-end gap-2">
              <Button
                className="font-medium text-default-500 dark:text-zinc-400 hover:text-default-700 dark:hover:text-zinc-200"
                variant="light"
                onPress={onClose}
              >
                {t("common.cancel")}
              </Button>
              <Button
                variant="flat"
                color="default"
                className="bg-default-100 dark:bg-white/10"
                startContent={
                  <FaSync className={testing ? "animate-spin" : ""} />
                }
                onPress={() => {
                  startMirrorTests(mirrorUrls || []);
                }}
              >
                {t("downloadpage.mirror.retest")}
              </Button>
              <Button
                className="font-bold text-white shadow-lg shadow-primary-900/20 bg-primary-600 hover:bg-primary-500 hover:scale-[1.02] active:scale-[0.98] transition-transform"
                radius="full"
                size="lg"
                isDisabled={!selectedUrl}
                startContent={installMode ? null : <FaDownload />}
                onPress={async (e) => {
                  if (!selectedUrl) return;
                  if (installMode) {
                    navigate("/install", {
                      state: {
                        mirrorVersion,
                        mirrorType,
                        returnTo: "/download",
                      },
                    });
                    onClose();
                  } else {
                    if (hasBackend) {
                      const desired = `${
                        mirrorType || "Release"
                      } ${mirrorVersion}.msixvc`;
                      const item = items.find(
                        (i) =>
                          i.short === mirrorVersion && i.type === mirrorType,
                      );
                      const md5sum = item?.md5;
                      const success = await startDownload(
                        selectedUrl,
                        desired,
                        md5sum,
                      );
                      if (success) {
                        triggerAnimation(e);
                        onClose();
                      }
                    } else {
                      window.open(selectedUrl, "_blank");
                      triggerAnimation(e);
                      onClose();
                    }
                  }
                }}
              >
                {installMode
                  ? t("downloadpage.mirror.install_selected")
                  : t("downloadpage.mirror.download_selected")}
              </Button>
            </div>
          }
        >
          <div className="flex flex-col gap-6">
            <div className="flex flex-col gap-4 p-4 bg-default-50 dark:bg-zinc-800/50 rounded-2xl border border-default-200/50 dark:border-white/5">
              <div className="flex items-center gap-2">
                <Chip
                  size="sm"
                  variant="flat"
                  color={mirrorType === "Preview" ? "secondary" : "warning"}
                  className="h-6"
                >
                  {mirrorType}
                </Chip>
                <span className="text-small font-mono text-default-600 dark:text-zinc-400">
                  {mirrorVersion}
                </span>
              </div>
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                <div className="flex items-center gap-2 text-small font-bold text-default-700 dark:text-zinc-300 shrink-0">
                  <div className="w-1 h-4 rounded-full bg-primary-500"></div>
                  {t("downloadpage.mirror.target")}
                </div>
                {(() => {
                  const target =
                    selectedUrl || (!testing ? bestMirror?.url : "");
                  if (!target)
                    return (
                      <div className="text-small text-default-400 dark:text-zinc-500 italic">
                        {testing
                          ? t("downloadpage.mirror.testing")
                          : t("downloadpage.mirror.unselected")}
                      </div>
                    );
                  const domain = labelFromUrl(target);
                  const fname = fileNameFromUrl(target);
                  return (
                    <div className="flex items-center gap-3 min-w-0 bg-white/50 dark:bg-black/20 rounded-xl px-3 py-1.5 border border-black/5 dark:border-white/5">
                      <div className="text-small truncate max-w-[400px] text-default-700 dark:text-zinc-300">
                        <span className="font-semibold text-primary-600 dark:text-primary-500">
                          {domain}
                        </span>
                        <span className="mx-1.5 opacity-30">|</span>
                        {fname}
                      </div>
                      <Button
                        size="sm"
                        variant="flat"
                        className="h-7 min-w-20 bg-default-200/50 dark:bg-white/10"
                        onPress={() => navigator.clipboard?.writeText(target)}
                        startContent={<FaCopy size={12} />}
                      >
                        {t("downloadpage.mirror.copy_link")}
                      </Button>
                    </div>
                  );
                })()}
              </div>
            </div>

            {mirrorUrls && mirrorUrls.length > 0 ? (
              <div className="flex flex-col gap-4">
                {/* Recommended Section */}
                <div>
                  <div className="flex items-center gap-3 mb-3 px-1">
                    <span className="text-sm font-bold text-default-800 dark:text-zinc-300 uppercase tracking-wider">
                      {t("downloadpage.mirror.recommended")}
                    </span>
                    {testing && (
                      <Chip
                        size="sm"
                        variant="flat"
                        color="success"
                        startContent={
                          <FaCircleNotch className="animate-spin" size={12} />
                        }
                      >
                        {t("downloadpage.mirror.auto_testing")}
                      </Chip>
                    )}
                  </div>

                  {bestMirror ? (
                    <div
                      className={`group relative overflow-hidden flex items-center justify-between gap-4 rounded-2xl border-2 p-4 transition-all cursor-pointer ${
                        selectedUrl === bestMirror.url
                          ? "border-primary-500 bg-primary-500/5 shadow-xl shadow-primary-500/10"
                          : "border-transparent bg-default-50 dark:bg-zinc-800/50 hover:bg-default-100 dark:hover:bg-zinc-800"
                      }`}
                      onClick={() => setSelectedUrl(bestMirror.url)}
                    >
                      <div className="flex items-center gap-4 min-w-0 z-10">
                        <div
                          className={`w-10 h-10 rounded-full flex items-center justify-center shrink-0 ${
                            selectedUrl === bestMirror.url
                              ? "bg-primary-500 text-white"
                              : "bg-default-200 dark:bg-zinc-700 text-default-500 dark:text-zinc-400"
                          }`}
                        >
                          <FaDownload size={16} />
                        </div>
                        <div className="flex flex-col gap-0.5 min-w-0">
                          <div className="font-bold text-medium text-default-900 dark:text-white truncate">
                            {bestMirror.label}
                          </div>
                          <div className="text-tiny text-default-500 dark:text-zinc-400 truncate font-mono opacity-70">
                            {bestMirror.url}
                          </div>
                        </div>
                      </div>
                      <div className="flex items-center gap-3 shrink-0 z-10">
                        <div className="flex flex-col items-end">
                          <span className="text-tiny text-default-500 dark:text-zinc-400 font-medium">
                            延迟
                          </span>
                          <span
                            className={`text-medium font-bold ${bestMirror.ok ? "text-primary-500" : "text-danger-500"}`}
                          >
                            {typeof bestMirror.latencyMs === "number"
                              ? `${Math.round(bestMirror.latencyMs)}ms`
                              : "-"}
                          </span>
                        </div>
                        {selectedUrl === bestMirror.url && (
                          <motion.div
                            layoutId="selected-check"
                            className="text-primary-500"
                          >
                            <div className="w-6 h-6 rounded-full bg-primary-500 flex items-center justify-center">
                              <svg
                                className="w-3.5 h-3.5 text-white"
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
                    <div className="p-8 rounded-2xl border border-dashed border-default-300 dark:border-zinc-700 flex flex-col items-center justify-center text-default-500 dark:text-zinc-400 gap-2">
                      <span>{t("downloadpage.mirror.no_recommended")}</span>
                    </div>
                  )}
                </div>

                {/* Others Section */}
                <div className="flex flex-col gap-2">
                  <div className="text-sm font-bold text-default-800 dark:text-zinc-300 uppercase tracking-wider px-1">
                    {t("downloadpage.mirror.others")}
                  </div>
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                    {mirrorResults.map((m, i) => (
                      <div
                        key={`mirror-${i}`}
                        className={`relative flex items-center justify-between gap-3 rounded-xl border p-2.5 transition-all cursor-pointer ${
                          selectedUrl === m.url
                            ? "border-primary-500/50 bg-primary-500/5"
                            : "border-default-200/50 dark:border-zinc-600 bg-white/50 dark:bg-zinc-700/30 hover:bg-default-100 dark:hover:bg-zinc-700 hover:border-default-300"
                        }`}
                        onClick={() => setSelectedUrl(m.url)}
                      >
                        <div className="flex items-center gap-3 min-w-0 overflow-hidden">
                          <div className="flex items-center justify-center w-6 h-6 rounded-full bg-default-100 dark:bg-zinc-600 text-tiny font-bold text-default-600 dark:text-zinc-100 shrink-0">
                            {String.fromCharCode(65 + i)}
                          </div>
                          <div className="flex flex-col min-w-0">
                            <span className="text-small font-bold text-default-800 dark:text-zinc-100 truncate">
                              {m.label}
                            </span>
                          </div>
                        </div>
                        <div className="flex items-center gap-2 shrink-0">
                          <span
                            className={`text-xs font-bold ${
                              typeof m.latencyMs === "number"
                                ? m.latencyMs < 100
                                  ? "text-primary-500"
                                  : m.latencyMs < 300
                                    ? "text-warning-500"
                                    : "text-danger-500"
                                : "text-default-400"
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
                          <div className="absolute inset-0 border-2 border-primary-500 rounded-xl pointer-events-none" />
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            ) : (
              <div className="flex flex-col items-center justify-center py-12 text-default-400 dark:text-zinc-500">
                <div className="w-16 h-16 rounded-full bg-default-100 flex items-center justify-center mb-4">
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
          onOpenChange={deleteDisclosure.onOpenChange}
          title={t("downloadpage.delete.title")}
          description={t("downloadpage.delete.body")}
          itemName={
            deleteItem?.fileName?.toLowerCase()?.endsWith(".msixvc")
              ? deleteItem?.fileName
              : `${deleteItem?.fileName || ""}.msixvc`
          }
          warning={t("downloadpage.delete.warning")}
          isPending={deleteLoading}
          error={deleteError ? trErr(deleteError) : null}
          onConfirm={async () => {
            if (!hasBackend) {
              setDeleteError("ERR_WRITE_TARGET");
              throw new Error("No backend");
            }
            setDeleteError("");
            setDeleteLoading(true);
            try {
              if (typeof minecraft?.DeleteDownloadedMsixvc !== "function") {
                setDeleteError("ERR_WRITE_TARGET");
                setDeleteLoading(false);
                throw new Error("Function not found");
              }
              const msg: string = await minecraft.DeleteDownloadedMsixvc(
                `${String(deleteItem?.type)} ${String(deleteItem?.short)}`,
                String(deleteItem?.type).toLowerCase(),
              );
              if (msg) {
                setDeleteError(msg);
                setDeleteLoading(false);
                throw new Error(msg);
              }
              setDeleteLoading(false);
              try {
                await refreshOne(
                  String(deleteItem?.short || ""),
                  String(deleteItem?.type || "release").toLowerCase(),
                );
              } catch {}
              try {
                const disp = deleteItem?.fileName
                  ?.toLowerCase()
                  ?.endsWith(".msixvc")
                  ? deleteItem?.fileName
                  : `${deleteItem?.fileName}.msixvc`;
                addToast({
                  title:
                    t("downloadpage.delete.success_body") + " " + (disp || ""),
                  color: "success",
                });
              } catch {
                addToast({
                  title:
                    t("downloadpage.delete.success_body") +
                    " " +
                    String(deleteItem?.fileName || ""),
                  color: "success",
                });
              }
            } catch (e: any) {
              setDeleteLoading(false);
              if (
                e.message !== "No backend" &&
                e.message !== "Function not found" &&
                e.message !== deleteError
              ) {
                if (!["No backend", "Function not found"].includes(e.message)) {
                  setDeleteError(String(e.message || e));
                }
              }
              throw e;
            }
          }}
        />

        {/* Install error modal */}
        <UnifiedModal
          isOpen={installErrorDisclosure.isOpen}
          onOpenChange={installErrorDisclosure.onOpenChange}
          type="error"
          title={t("downloadpage.progress.unknown_error")}
          confirmText={t("common.close")}
          onConfirm={installErrorDisclosure.onClose}
          showCancelButton={false}
        >
          <div className="text-medium text-default-700 dark:text-zinc-300 font-bold">
            {trErr(installError)}
          </div>
        </UnifiedModal>

        {/* Install progress modal */}
        <UnifiedModal
          isOpen={installLoadingDisclosure.isOpen}
          onOpenChange={installLoadingDisclosure.onOpenChange}
          type="primary"
          title={t("downloadpage.install.title")}
          icon={<Spinner size="lg" color="primary" />}
          hideCloseButton
          isDismissable={false}
          showConfirmButton={false}
          showCancelButton={false}
        >
          <div className="flex flex-col gap-6">
            <div className="text-medium text-default-700 dark:text-zinc-300 font-bold">
              {t("downloadpage.install.hint")}
            </div>
            <div className="flex items-center gap-3">
              <Progress
                aria-label="install-progress"
                isIndeterminate={!extractInfo?.totalBytes}
                value={
                  extractInfo?.totalBytes
                    ? (extractInfo.bytes / extractInfo.totalBytes) * 100
                    : 0
                }
                size="md"
                showValueLabel={!!extractInfo?.totalBytes}
                formatOptions={{ style: "percent" }}
                classNames={{
                  indicator: "bg-gradient-to-r from-primary-500 to-primary-400",
                  track: "bg-default-100",
                }}
                className="flex-1"
              />
            </div>
            {typeof extractInfo?.bytes === "number" && extractInfo.bytes > 0 ? (
              <div className="flex justify-between text-small text-default-600 dark:text-zinc-400 font-medium">
                <span>
                  {extractInfo.totalBytes
                    ? t("downloadpage.install.progress")
                    : t("downloadpage.install.estimated_size")}
                  :{" "}
                </span>
                <span className="font-mono">
                  {(() => {
                    const formatSize = (n: number) => {
                      const kb = 1024;
                      const mb = kb * 1024;
                      const gb = mb * 1024;
                      if (n >= gb) return (n / gb).toFixed(2) + " GB";
                      if (n >= mb) return (n / mb).toFixed(2) + " MB";
                      if (n >= kb) return (n / kb).toFixed(2) + " KB";
                      return n + " B";
                    };
                    const current = formatSize(extractInfo.bytes);
                    if (extractInfo.totalBytes) {
                      const percent = (
                        (extractInfo.bytes / extractInfo.totalBytes) *
                        100
                      ).toFixed(1);
                      return `${current} / ${formatSize(extractInfo.totalBytes)} (${percent}%)`;
                    }
                    return current;
                  })()}
                </span>
              </div>
            ) : null}
            {installingTargetName ? (
              <div className="p-3 bg-default-100/50 dark:bg-zinc-800/50 rounded-xl border border-default-200/50 dark:border-zinc-700/50 text-small text-default-600 dark:text-zinc-400 font-medium">
                {t("downloadpage.install.target")}:{" "}
                <span className="font-mono text-default-700 dark:text-zinc-200 font-bold">
                  {installingTargetName}
                </span>
              </div>
            ) : null}
          </div>
        </UnifiedModal>

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
              <div className="w-8 h-8 rounded-full bg-gradient-to-br from-primary-400 to-primary-600 text-white flex items-center justify-center">
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
