import { MicrosoftAccountBar } from "@/components/MicrosoftAccountBar";
import { ModalAction, ModalPanel, ModalDescription } from "@/components/ModalPrimitives";
import {
  Button,
  ButtonGroup,
  Card,
  Chip,
  CloseButton,
  Dropdown,
  InputGroup,
  Label,
  ProgressBar,
  Spinner,
  Table,
  TextField,
  Tooltip,
  toast,
  useOverlayState,
} from "@heroui/react";
import { PagePagination } from "@/components/PagePagination";
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
import { motion, Variants } from "framer-motion";
import { useNavigate } from "react-router-dom";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import { useDownloads } from "@/utils/DownloadsContext";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { cn } from "@/utils/cn";
import { ROUTES } from "@/constants/routes";
import { useDownloadFilters } from "@/hooks/useDownloadFilters";
import { Clipboard } from "@wailsio/runtime";

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
  const navigate = useNavigate();
  const { startDownload, isDownloading } = useDownloads();
  const [items, setItems] = useState<VersionItem[]>([]);
  const [versionsLoading, setVersionsLoading] = useState(true);
  const [versionsError, setVersionsError] = useState(false);
  const {
    map: versionStatusMap,
    refreshAll,
    refreshOne,
    markDownloaded,
    setCurrentDownloadingInfo,
    refreshing,
  } = useVersionStatus();

  const [query, setQuery] = useState("");
  const {
    typeFilter,
    setTypeFilter,
    statusFilter,
    setStatusFilter,
    llFilter,
    setLlFilter,
  } = useDownloadFilters();
  const [rowsPerPage, setRowsPerPage] = useState<number>(6);
  const [page, setPage] = useState<number>(1);
  const tableAreaRef = useRef<HTMLDivElement>(null);

  const {
    isOpen,
    open: onOpen,
    close: onClose,
    setOpen: onOpenChange,
  } = useOverlayState();

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
  const installLoadingDisclosure = useOverlayState();
  const installErrorDisclosure = useOverlayState();
  const [installingVersion, setInstallingVersion] = useState<string>("");
  const [installingTargetName, setInstallingTargetName] = useState<string>("");
  const deleteDisclosure = useOverlayState();
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
      setVersionsLoading(true);
      setVersionsError(false);
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
        setVersionsError(true);
      } finally {
        setVersionsLoading(false);
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
    if (versionsLoading) return;
    setVersionsLoading(true);
    setVersionsError(false);
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
      setVersionsError(true);
    } finally {
      setVersionsLoading(false);
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
  const paged = useMemo(
    () => filtered.slice((page - 1) * rowsPerPage, page * rowsPerPage),
    [filtered, page, rowsPerPage],
  );

  useEffect(() => {
    setPage(1);
  }, [query, typeFilter, statusFilter, llFilter]);

  useLayoutEffect(() => {
    const area = tableAreaRef.current;
    if (!area) return;

    const calcRows = () => {
      const header = area.querySelector("thead");
      const rows = Array.from(area.querySelectorAll("tbody tr"));
      if (!header || paged.length === 0 || rows.length === 0) return;

      // Measure the space left after the toolbar, warning and pagination.
      // Actual row heights also account for font scaling and translated labels.
      const rowHeight = Math.max(
        ...rows.map((row) => row.getBoundingClientRect().height),
      );
      if (rowHeight <= 0) return;
      setRowsPerPage(
        Math.max(
          1,
          Math.floor(
            (area.clientHeight - header.getBoundingClientRect().height - 1) /
              rowHeight,
          ),
        ),
      );
    };
    calcRows();
    const observer = new ResizeObserver(calcRows);
    observer.observe(area);
    const table = area.querySelector("table");
    if (table) observer.observe(table);
    return () => observer.disconnect();
  }, [paged]);

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
        className="relative h-dvh min-h-0 overflow-hidden"
        animate={false}
      >
        <motion.div
          className="shrink-0"
          initial={{ opacity: 0, y: -20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.4 }}
        >
          <Card className={cn("flex-none gap-0 p-0", LAYOUT.GLASS_CARD.BASE)}>
            <Card.Content className="px-4 py-3">
              <MicrosoftAccountBar />
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
                        `transition-all ${
                          isDownloading
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
          custom={0}
          initial="hidden"
          animate="visible"
          variants={cardVariants}
          className="flex-1 min-h-0 flex flex-col"
        >
          <Card className={cn("flex-1 min-h-0 gap-0 p-0 overflow-hidden", LAYOUT.GLASS_CARD.BASE)}>
            <Card.Content className="p-0 flex flex-1 min-h-0 flex-col overflow-hidden relative">
              {versionsError && items.length > 0 && (
                <div role="alert" className="flex shrink-0 items-center justify-between gap-3 p-3 text-sm bg-warning/10 text-foreground">
                  <span>{t("audit.primary.download.stale")}</span>
                  <Button size="sm" variant="secondary" onPress={reloadAll} isDisabled={versionsLoading}>{t("download_manager.actions.retry")}</Button>
                </div>
              )}
              <Table
                ref={tableAreaRef}
                className="flex-1 min-h-0 overflow-hidden"
                variant="secondary"
              >
                <Table.ScrollContainer className="h-full overflow-hidden">
                  <Table.Content
                    aria-label={
                      t("downloadpage.table.aria_label") as unknown as string
                    }
                    className="min-w-full [&_td]:whitespace-nowrap"
                  >
                    <Table.Header
                      className={cn(
                        "sticky top-0 z-10",
                        COMPONENT_STYLES.tableSticky.thead,
                      )}
                    >
                      <Table.Column
                        className={cn(COMPONENT_STYLES.tableSticky.th)}
                        key="version"
                        width={180}
                        id={"version"}
                        isRowHeader
                      >
                        {t("downloadpage.table.header.version")}
                      </Table.Column>
                      <Table.Column
                        className={cn(COMPONENT_STYLES.tableSticky.th)}
                        key="type"
                        width={140}
                        id={"type"}
                      >
                        {t("downloadpage.table.header.type")}
                      </Table.Column>
                      <Table.Column
                        className={cn(COMPONENT_STYLES.tableSticky.th)}
                        key="status"
                        width={160}
                        id={"status"}
                      >
                        {t("downloadpage.table.header.status")}
                      </Table.Column>
                      <Table.Column
                        className={cn(COMPONENT_STYLES.tableSticky.th)}
                        key="loader"
                        width={160}
                        id={"loader"}
                      >
                        {t("downloadpage.table.header.loader")}
                      </Table.Column>
                      <Table.Column
                        key="actions"
                        id={"actions"}
                        className={cn(
                          COMPONENT_STYLES.tableSticky.th,
                          "text-right",
                        )}
                      >
                        {t("downloadpage.table.header.actions")}
                      </Table.Column>
                    </Table.Header>
                    <Table.Body
                      renderEmptyState={() => (
                        <div className="flex flex-col items-center justify-center h-40 text-muted gap-2">
                          {versionsLoading ? <Spinner /> : <FaBoxOpen className="w-10 h-10 opacity-20" />}
                          <p role={versionsError ? "alert" : "status"}>{t(versionsLoading ? "common.loading" : versionsError ? "audit.primary.download.failed" : items.length > 0 ? "audit.primary.download.no_matches" : "downloadpage.table.empty")}</p>
                          {!versionsLoading && (versionsError || items.length === 0 ? (
                            <Button size="sm" variant="secondary" onPress={reloadAll}>{t("download_manager.actions.retry")}</Button>
                          ) : (
                            <Button size="sm" variant="secondary" onPress={() => { setQuery(""); setTypeFilter("all"); setStatusFilter("all"); setLlFilter("all"); }}>{t("audit.primary.clear_filters")}</Button>
                          ))}
                        </div>
                      )}
                    >
                      {paged.map((item, index) => (
                        <Table.Row
                          className={cn(
                            "group transition-colors hover:bg-surface/50 dark:hover:bg-surface-secondary/30",
                          )}
                          key={`${item.type}-${item.short}`}
                          id={`${item.type}-${item.short}`}
                        >
                          <Table.Cell
                            className={cn(
                              "py-3 border-b border-border/50 dark:border-white/5 group-last:border-0",
                            )}
                          >
                            <motion.div
                              custom={index}
                              initial="hidden"
                              animate="visible"
                              variants={rowVariants}
                            >
                              <span className="text-foreground dark:text-white text-sm">
                                {item.short}
                              </span>
                            </motion.div>
                          </Table.Cell>
                          <Table.Cell
                            className={cn(
                              "py-3 border-b border-border/50 dark:border-white/5 group-last:border-0",
                            )}
                          >
                            <motion.div
                              custom={index}
                              initial="hidden"
                              animate="visible"
                              variants={rowVariants}
                            >
                              <Chip
                                size="sm"
                                variant="soft"
                                color={
                                  item.type === "Release" ? "success" : "warning"
                                }
                                className={"font-medium"}
                              >
                                <Chip.Label>
                                  {item.type === "Release"
                                    ? t("downloadpage.table.type.release")
                                    : t("downloadpage.table.type.preview")}
                                </Chip.Label>
                              </Chip>
                            </motion.div>
                          </Table.Cell>
                          <Table.Cell
                            className={cn(
                              "py-3 border-b border-border/50 dark:border-white/5 group-last:border-0",
                            )}
                          >
                            <motion.div
                              custom={index}
                              initial="hidden"
                              animate="visible"
                              variants={rowVariants}
                            >
                              {!hasStatus(item) && refreshing ? (
                                <div className="flex items-center gap-2 text-muted">
                                  <FaCircleNotch className="animate-spin text-xs" />
                                  <span className="text-sm">
                                    {t("downloadpage.status.checking")}
                                  </span>
                                </div>
                              ) : isDownloaded(item) ? (
                                <Chip
                                  size="sm"
                                  variant="soft"
                                  color={"success"}
                                  className={"gap-1 px-2"}
                                >
                                  <Chip.Label>
                                    {t("downloadpage.status.downloaded")}
                                  </Chip.Label>
                                </Chip>
                              ) : (
                                <Chip
                                  size="sm"
                                  variant="soft"
                                  color={"danger"}
                                  className={"gap-1 px-2"}
                                >
                                  <Chip.Label>
                                    {t("downloadpage.status.not_downloaded")}
                                  </Chip.Label>
                                </Chip>
                              )}
                            </motion.div>
                          </Table.Cell>
                          <Table.Cell
                            className={cn(
                              "py-3 border-b border-border/50 dark:border-white/5 group-last:border-0",
                            )}
                          >
                            <motion.div
                              custom={index}
                              initial="hidden"
                              animate="visible"
                              variants={rowVariants}
                            >
                              {isLLSupported(item.short) ? (
                                <div className="flex items-center gap-1.5 text-brand-600 dark:text-brand-400 bg-brand-100/50 dark:bg-brand-900/20 px-2 py-1 rounded-lg w-fit">
                                  <span className="text-sm">LeviLamina</span>
                                </div>
                              ) : (
                                <span className="text-muted dark:text-zinc-600 ml-2">
                                  -
                                </span>
                              )}
                            </motion.div>
                          </Table.Cell>
                          <Table.Cell
                            className={cn(
                              "py-3 border-b border-border/50 dark:border-white/5 group-last:border-0",
                            )}
                          >
                            <motion.div
                              custom={index}
                              initial="hidden"
                              animate="visible"
                              variants={rowVariants}
                              className="flex justify-end"
                            >
                              {isDownloaded(item) ? (
                                <ButtonGroup
                                  size="sm"
                                  variant={"secondary"}
                                  className={cn(
                                    "rounded-full",
                                    "bg-transparent",
                                  )}
                                >
                                  <Button
                                    onPress={() => {
                                      navigate(ROUTES.install, {
                                        state: {
                                          mirrorVersion: item.short,
                                          mirrorType: item.type,
                                          returnTo: ROUTES.download,
                                          isLeviLaminaSupported: isLLSupported(
                                            item.short,
                                          ),
                                        },
                                      });
                                    }}
                                    variant={"secondary"}
                                    className={
                                      "px-2 h-8 font-medium text-foreground dark:text-zinc-200 bg-surface-secondary dark:bg-surface-tertiary/50 w-[88px]"
                                    }
                                  >
                                    {<FaBoxOpen size={14} />}
                                    {t("downloadpage.mirror.install_button")}
                                  </Button>
                                  <Dropdown>
                                    <Button
                                      isIconOnly
                                      variant={"secondary"}
                                      className={
                                        "h-8 min-w-8 w-8 px-0 bg-surface-secondary dark:bg-surface-tertiary/50"
                                      }
                                    >
                                      <FaChevronDown size={12} />
                                    </Button>
                                    <Dropdown.Popover
                                      className={
                                        COMPONENT_STYLES.dropdown.content
                                      }
                                    >
                                      <Dropdown.Menu
                                        aria-label="Actions"
                                        onAction={async (key) => {
                                          if (String(key) !== "delete_msixvc")
                                            return;
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
                                                  String(
                                                    item.type,
                                                  ).toLowerCase(),
                                                );
                                            }
                                          } catch {}
                                          setDeleteItem({
                                            short: item.short,
                                            type: item.type,
                                            fileName:
                                              fname ||
                                              `${item.type} ${item.short}`,
                                          });
                                          deleteDisclosure.open();
                                        }}
                                      >
                                        <Dropdown.Item
                                          key="delete_msixvc"
                                          id={"delete_msixvc"}
                                          textValue={t(
                                            "downloadpage.actions.delete_installer",
                                          )}
                                          variant="danger"
                                        >
                                          {<FaTrash size={12} />}
                                          <Label>
                                            {t(
                                              "downloadpage.actions.delete_installer",
                                            )}
                                          </Label>
                                          <Dropdown.ItemIndicator />
                                        </Dropdown.Item>
                                      </Dropdown.Menu>
                                    </Dropdown.Popover>
                                  </Dropdown>
                                </ButtonGroup>
                              ) : (
                                <Button
                                  size="sm"
                                  isDisabled={!hasStatus(item) && refreshing}
                                  onPress={() => {
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
                                  variant={"secondary"}
                                  className={cn(
                                    "rounded-full",
                                    "px-0 h-8 font-medium bg-surface-secondary dark:bg-surface-tertiary/50 text-foreground dark:text-zinc-200 hover:bg-surface-tertiary dark:hover:bg-surface-quaternary transition-all w-[120px]",
                                  )}
                                >
                                  {<FaDownload size={14} />}
                                  {!hasStatus(item) && refreshing
                                    ? t("downloadpage.status.checking")
                                    : t("downloadmodal.download_button")}
                                </Button>
                              )}
                            </motion.div>
                          </Table.Cell>
                        </Table.Row>
                      ))}
                    </Table.Body>
                  </Table.Content>
                </Table.ScrollContainer>
              </Table>
              {/* Footer Pagination */}
              <div className="flex items-center justify-between px-4 py-3 border-t border-border dark:border-white/10 bg-transparent shrink-0 z-10">
                <div className="text-sm text-muted dark:text-zinc-400 shrink-0 whitespace-nowrap">
                  {t("downloadpage.bottomcontent.total", {
                    count: filtered.length,
                  })}
                </div>
                <PagePagination
                  size="sm"
                  pageCount={totalPages}
                  currentPage={page}
                  onPageChange={setPage}
                  pageClassName="rounded-full"
                />
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
            <div className="flex w-full justify-end gap-2">
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
                size="lg"
                isDisabled={!selectedUrl}
                onPress={async (e) => {
                  if (!selectedUrl) return;
                  if (installMode) {
                    navigate(ROUTES.install, {
                      state: {
                        mirrorVersion,
                        mirrorType,
                        returnTo: ROUTES.download,
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
                        { version: mirrorVersion, type: mirrorType || "Release", isLeviLaminaSupported: isLLSupported(mirrorVersion) },
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
                variant={"secondary"}
              >
                {installMode ? null : <FaDownload />}
                {installMode
                  ? t("downloadpage.mirror.install_selected")
                  : t("downloadpage.mirror.download_selected")}
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
                      <div className="text-sm truncate max-w-[400px] text-foreground dark:text-zinc-300">
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
                            toast.success(t("audit.mods.link_copied"));
                          } catch {
                            toast.danger(t("audit.mods.copy_failed"));
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
                      className={`group relative overflow-hidden flex items-center justify-between gap-4 rounded-2xl border-2 p-4 transition-all cursor-pointer ${
                        selectedUrl === bestMirror.url
                          ? "border-brand-500 bg-brand-500/5 shadow-xl shadow-brand-500/10"
                          : "border-transparent bg-surface dark:bg-surface-secondary/50 hover:bg-surface-secondary "
                      }`}
                      onClick={() => setSelectedUrl(bestMirror.url)}
                    >
                      <div className="flex items-center gap-4 min-w-0 z-10">
                        <div
                          className={`w-10 h-10 rounded-full flex items-center justify-center shrink-0 ${
                            selectedUrl === bestMirror.url
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
                            延迟
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
                        className={`relative flex items-center justify-between gap-3 rounded-xl border p-2.5 transition-all cursor-pointer ${
                          selectedUrl === m.url
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
                            className={`text-xs font-bold ${
                              typeof m.latencyMs === "number"
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
                toast(
                  t("downloadpage.delete.success_body") + " " + (disp || ""),
                  { variant: "success", timeout: 2000 },
                );
              } catch {
                toast(
                  t("downloadpage.delete.success_body") +
                    " " +
                    String(deleteItem?.fileName || ""),
                  { variant: "success", timeout: 2000 },
                );
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
          onOpenChange={installErrorDisclosure.setOpen}
          type="error"
          title={t("downloadpage.progress.unknown_error")}
          confirmText={t("common.close")}
          onConfirm={installErrorDisclosure.close}
          showCancelButton={false}
        >
          <ModalDescription>
            {trErr(installError)}
          </ModalDescription>
        </UnifiedModal>

        {/* Install progress modal */}
        <UnifiedModal
          isOpen={installLoadingDisclosure.isOpen}
          onOpenChange={installLoadingDisclosure.setOpen}
          type="primary"
          title={t("downloadpage.install.title")}
          icon={<Spinner size="lg" color={"accent"} />}
          isDismissable={false}
          showConfirmButton={false}
          showCancelButton={false}
        >
          <div className="flex flex-col gap-6">
            <ModalDescription>
              {t("downloadpage.install.hint")}
            </ModalDescription>
            <div className="flex items-center gap-3">
              <ProgressBar
                aria-label="install-progress"
                isIndeterminate={!extractInfo?.totalBytes}
                value={
                  extractInfo?.totalBytes
                    ? (extractInfo.bytes / extractInfo.totalBytes) * 100
                    : 0
                }
                size="md"
                formatOptions={{ style: "percent" }}
                className={"flex-1"}
              >
                {!!extractInfo?.totalBytes && <ProgressBar.Output />}
                <ProgressBar.Track className={"bg-surface-secondary"}>
                  <ProgressBar.Fill
                    className={"bg-brand-500"}
                  />
                </ProgressBar.Track>
              </ProgressBar>
            </div>
            {typeof extractInfo?.bytes === "number" && extractInfo.bytes > 0 ? (
              <div className="flex justify-between text-sm text-foreground dark:text-zinc-400 font-medium">
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
              <ModalPanel className="font-medium">
                {t("downloadpage.install.target")}:{" "}
                <span className="font-mono text-foreground dark:text-zinc-200 font-bold">
                  {installingTargetName}
                </span>
              </ModalPanel>
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
