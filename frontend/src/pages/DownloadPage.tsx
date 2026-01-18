"use client";
import React, { useEffect, useMemo, useState, useRef } from "react";
import { BaseModal, BaseModalHeader, BaseModalBody, BaseModalFooter } from "@/components/BaseModal";
import { PageHeader } from "@/components/PageHeader";
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
  TableBody,
  TableCell,
  TableColumn,
  TableHeader,
  TableRow,
  ModalContent,
  Progress,
  Spinner,
  useDisclosure,
  Card,
  CardBody,
  CardHeader,
  ButtonGroup,
} from "@heroui/react";
import { FaDownload, FaCopy, FaSync, FaTrash, FaBoxOpen, FaChevronDown } from "react-icons/fa";
import { Events } from "@wailsio/runtime";
import { useVersionStatus } from "@/utils/VersionStatusContext";
import { useTranslation } from "react-i18next";
import { motion } from "framer-motion";
import { useNavigate } from "react-router-dom";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";

type ItemType = "Preview" | "Release";

type VersionItem = {
  version: string;
  urls: string[];
  type: ItemType;
  short: string;
  timestamp?: number;
};

export const DownloadPage: React.FC = () => {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
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
  const [rowsPerPage, setRowsPerPage] = useState<number>(6);
  const [page, setPage] = useState<number>(1);

  const { isOpen, onOpen, onOpenChange } = useDisclosure();
  const progressDisclosure = useDisclosure();
  const [dlProgress, setDlProgress] = useState<{
    downloaded: number;
    total: number;
    dest?: string;
  } | null>(null);
  const [dlSpeed, setDlSpeed] = useState<number>(0);
  const [dlStatus, setDlStatus] = useState<string>("");
  const [dlError, setDlError] = useState<string>("");
  const downloadSuccessDisclosure = useDisclosure();
  const [downloadSuccessMsg, setDownloadSuccessMsg] = useState<string>("");
  const [extractInfo, setExtractInfo] = useState<{
    files: number;
    bytes: number;
    dir: string;
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
  const [installError, setInstallError] = useState<string>("");
  const installLoadingDisclosure = useDisclosure();
  const installSuccessDisclosure = useDisclosure();
  const installErrorDisclosure = useDisclosure();
  const [installSuccessMsg, setInstallSuccessMsg] = useState<string>("");
  const [installingVersion, setInstallingVersion] = useState<string>("");
  const [installingTargetName, setInstallingTargetName] = useState<string>("");
  const deleteDisclosure = useDisclosure();
  const deleteSuccessDisclosure = useDisclosure();
  const [deleteItem, setDeleteItem] = useState<{
    short: string;
    type: ItemType;
    fileName: string;
  } | null>(null);
  const [deleteError, setDeleteError] = useState<string>("");
  const [deleteLoading, setDeleteLoading] = useState<boolean>(false);
  const [deleteSuccessMsg, setDeleteSuccessMsg] = useState<string>("");

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
      (m) => typeof m.latencyMs === "number"
    );
    if (measured.length === 0) return null;
    const okList = measured.filter((m) => m.ok);
    const list = (okList.length > 0 ? okList : measured).slice();
    list.sort(
      (a, b) =>
        (a.latencyMs ?? Number.MAX_SAFE_INTEGER) -
        (b.latencyMs ?? Number.MAX_SAFE_INTEGER)
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
        i18n?.language || navigator.language || ""
      ).toLowerCase();
      const langs = (navigator.languages || []).map((l) =>
        String(l).toLowerCase()
      );
      const isTzCN = tz === "Asia/Shanghai" || tz === "Asia/Urumqi";
      const isLangCN =
        lang.startsWith("zh-cn") || langs.includes("zh-cn") || lang === "zh";
      return isTzCN || isLangCN;
    } catch {
      return false;
    }
  }, [i18n?.language]);



  const startMirrorTests = async (urls: string[]) => {
    if (!urls || urls.length === 0) return;
    setMirrorResults(
      urls.map((u) => ({
        url: u,
        label: labelFromUrl(u),
        latencyMs: null,
        ok: false,
      }))
    );
    setTesting(true);
    try {
      if (hasBackend && minecraft?.TestMirrorLatencies) {
        const res = await minecraft.TestMirrorLatencies(urls, 7000);
        const byUrl = new Map<string, any>(
          (res || []).map((r: any) => [String(r.url), r])
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
          })
        );
      } else {
        setMirrorResults((prev) =>
          prev.map((mr) => ({ ...mr, latencyMs: null, ok: false }))
        );
      }
    } finally {
      setTesting(false);
    }
  };

  const hasBackend = minecraft !== undefined;
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
        Number(b.includes("xboxlive.cn")) - Number(a.includes("xboxlive.cn"))
    );
  };

  const fileNameFromUrl = (u: string): string => {
    try {
      const url = new URL(u);
      const segs = url.pathname.split("/").filter(Boolean);
      return (
        segs[segs.length - 1] ||
        (t("downloadpage.mirror.filename_fallback", {
          defaultValue: "(文件)",
        }) as unknown as string)
      );
    } catch {
      const segs = String(u).split("/").filter(Boolean);
      return (
        segs[segs.length - 1] ||
        (t("downloadpage.mirror.filename_fallback", {
          defaultValue: "(文件)",
        }) as unknown as string)
      );
    }
  };

  const dlOffsRef = useRef<(() => void)[]>([]);
  const extractActiveRef = useRef<boolean>(false);
  const dlLastRef = useRef<{ ts: number; bytes: number } | null>(null);
  const ensureDlSubscriptions = () => {
    if (!hasBackend) return;
    if (dlOffsRef.current.length > 0) return;
    const off1 = Events.On("msixvc_download_progress", (event) => {
      setDlProgress({
        downloaded: Number(event.data.Downloaded || 0),
        total: Number(event.data.Total || 0),
        dest: String(event.data.Dest || ""),
      });
      try {
        const now = Date.now();
        const bytes = Number(event.data.Downloaded || 0);
        const prev = dlLastRef.current;
        if (prev) {
          const dt = (now - prev.ts) / 1000;
          const db = bytes - prev.bytes;
          const spd = dt > 0 && db >= 0 ? db / dt : 0;
          setDlSpeed(spd);
        }
        dlLastRef.current = { ts: now, bytes };
      } catch {}
    });
    const off2 = Events.On("msixvc_download_status", (event) => {
      const s = String(event.data || "");
      setDlStatus(s);
      if (s === "started" || s === "resumed" || s === "cancelled") {
        setDlError("");
        dlLastRef.current = null;
        setDlSpeed(0);
      }
    });
    const off3 = Events.On("msixvc_download_error", (event) => {
      setDlError(
        event.data
          ? String(event.data)
          : (t("downloadpage.progress.unknown_error", {
              defaultValue: "未知错误",
            }) as unknown as string)
      );
    });
    const off4 = Events.On("msixvc_download_done", (event) => {
      setDlStatus("done");
      const d = String(event.data || "");
      setDlProgress((p) => ({
        downloaded: p?.total || 0,
        total: p?.total || 0,
        dest: d || String(p?.dest || ""),
      }));
      try {
        dlLastRef.current = null;
        setDlSpeed(0);
      } catch {}
      try {
        progressDisclosure.onClose();
      } catch {}
      setTimeout(() => {
        try {
          const fname = d
            ? d.split(/[\\/]/).pop() || ""
            : dlProgress?.dest || "";
          const show =
            fname ||
            (t("downloadpage.progress.title", {
              defaultValue: "下载进度",
            }) as unknown as string);
          setDownloadSuccessMsg(String(show));
          downloadSuccessDisclosure.onOpen();
        } catch {}
      }, 120);
    });
    const off5 = Events.On("extract.progress", (event) => {
      const payload = event?.data || {};
      const files = Number(payload?.files || 0);
      const bytes = Number(payload?.bytes || 0);
      const dir = String(payload?.dir || "");
      setExtractInfo({ files, bytes, dir });
      if (extractActiveRef.current && !installLoadingDisclosure.isOpen)
        installLoadingDisclosure.onOpen();
    });
    const off6 = Events.On("extract.error", (event) => {
      setExtractError(String(event?.data || ""));
      extractActiveRef.current = false;
    });
    const off7 = Events.On("extract.done", (_event) => {
      extractActiveRef.current = false;
      setTimeout(() => {
        setExtractInfo(null);
        setExtractError("");
        try {
          installLoadingDisclosure.onClose();
        } catch {}
        try {
          setInstallSuccessMsg(
            t("downloadpage.install.success", {
              defaultValue: "安装完成",
            }) as unknown as string
          );
          installSuccessDisclosure.onOpen();
        } catch {}
      }, 300);
    });
    dlOffsRef.current = [off1, off2, off3, off4, off5, off6, off7];
  };

  useEffect(() => {
    ensureDlSubscriptions();
    return () => {
      for (const off of dlOffsRef.current) {
        try {
          off();
        } catch {}
      }
      dlOffsRef.current = [];
    };
  }, [hasBackend]);

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
          // data = await fetchDbJson();
          data = { previewVersions: [], releaseVersions: [] };
        }
        const preview: VersionItem[] = (data.previewVersions || []).map(
          (v: any) => ({
            version: v.version,
            urls: normalizeUrls(v.urls ?? v.url),
            type: "Preview",
            short: String(v.version).replace(/^Preview\s*/, ""),
            timestamp: v.timestamp,
          })
        );
        const release: VersionItem[] = (data.releaseVersions || []).map(
          (v: any) => ({
            version: v.version,
            urls: normalizeUrls(v.urls ?? v.url),
            type: "Release",
            short: String(v.version).replace(/^Release\s*/, ""),
            timestamp: v.timestamp,
          })
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
        })
      );
      const release: VersionItem[] = (data.releaseVersions || []).map(
        (v: any) => ({
          version: v.version,
          urls: normalizeUrls(v.urls ?? v.url),
          type: "Release",
          short: String(v.version).replace(/^Release\s*/, ""),
          timestamp: v.timestamp,
        })
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
    [items, versionStatusMap]
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
          : !Boolean(it._status?.isDownloaded)
      )
      .filter((it) =>
        q
          ? it.short.toLowerCase().includes(q) ||
            it.version.toLowerCase().includes(q) ||
            it.type.toLowerCase().includes(q)
          : true
      )
      .sort((a, b) => {
        const ta = a.timestamp ?? 0;
        const tb = b.timestamp ?? 0;
        if (ta !== tb) return tb - ta;
        return compareVersionDesc(a.short, b.short);
      });
  }, [itemsWithStatus, query, typeFilter, statusFilter]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / rowsPerPage));
  const paged = filtered.slice((page - 1) * rowsPerPage, page * rowsPerPage);

  useEffect(() => {
    setPage(1);
  }, [query, typeFilter, statusFilter]);

  useEffect(() => {
    const calcRows = () => {
      const rowH = 56;
      const reserve = 300;
      const computed = Math.max(
        4,
        Math.floor((window.innerHeight - reserve) / rowH)
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

  return (
    <>
      <motion.div
        className="w-full max-w-full mx-auto p-4 h-full flex flex-col"
        initial={{ opacity: 0, y: 8 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.25 }}
      >
        <div className="flex flex-col h-full">
          <Card className="flex-1 min-h-0 border-none shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl">
            <CardBody className="p-0 flex flex-col h-full overflow-hidden">
              <div className="p-4 sm:p-6 pb-2 flex flex-col sm:flex-row gap-4 justify-between items-center border-b border-default-200 dark:border-white/10">
                <div className="flex items-center gap-3 w-full sm:max-w-md">
                    <Input
                      isClearable
                      radius="full"
                      classNames={{
                        base: "max-w-full sm:max-w-[20rem] h-10",
                        mainWrapper: "h-full",
                        input: "text-small",
                        inputWrapper: "h-full font-normal text-default-500 bg-default-400/20 dark:bg-default-500/20",
                      }}
                      placeholder={t("downloadpage.topcontent.input.placeholder")}
                      value={query}
                      onValueChange={setQuery}
                      startContent={<FaSync size={14} className="text-default-400" />}
                      onClear={() => setQuery("")}
                    />
                </div>
                <div className="flex items-center gap-2 shrink-0 overflow-x-auto w-full sm:w-auto pb-1 sm:pb-0">
                  <Button
                    radius="full"
                    variant="flat"
                    className="bg-default-100 dark:bg-zinc-800 text-default-600 dark:text-zinc-200 font-medium hover:bg-default-200 dark:hover:bg-zinc-700 transition-colors"
                    startContent={<FaSync className={refreshing ? "animate-spin" : ""} size={14} />}
                    isDisabled={items.length === 0}
                    isLoading={refreshing}
                    onPress={async () => {
                      await reloadAll();
                    }}
                  >
                    {t("common.refresh", { defaultValue: "刷新" })}
                  </Button>
                  <Button
                    radius="full"
                    variant="flat"
                    className="bg-default-100 dark:bg-zinc-800 text-default-600 dark:text-zinc-200 font-medium hover:bg-default-200 dark:hover:bg-zinc-700 transition-colors"
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
                    {t("downloadpage.customappx.button", {
                      defaultValue: "自定义",
                    })}
                  </Button>
                  <Dropdown>
                    <DropdownTrigger>
                      <Button
                        radius="full"
                        variant="flat"
                        className="bg-default-100 dark:bg-zinc-800 text-default-600 dark:text-zinc-200 font-medium shrink-0 hover:bg-default-200 dark:hover:bg-zinc-700 transition-colors"
                      >
                        {t("downloadpage.topcontent.types")}
                      </Button>
                    </DropdownTrigger>
                    <DropdownMenu
                      selectionMode="single"
                      selectedKeys={[typeFilter]}
                      onSelectionChange={(keys) => {
                        const k = Array.from(keys)[0] as "all" | ItemType;
                        setTypeFilter(k);
                      }}
                    >
                      <DropdownItem key="all">
                        {t("downloadpage.topcontent.types_all", {
                          defaultValue: "All",
                        })}
                      </DropdownItem>
                      <DropdownItem key="Release">
                        {t("downloadpage.customappx.modal.1.body.select.item1")}
                      </DropdownItem>
                      <DropdownItem key="Preview">
                        {t("downloadpage.customappx.modal.1.body.select.item2")}
                      </DropdownItem>
                    </DropdownMenu>
                  </Dropdown>
                  <Dropdown>
                    <DropdownTrigger>
                      <Button
                        radius="full"
                        variant="flat"
                        className="bg-default-100 dark:bg-zinc-800 text-default-600 dark:text-zinc-200 font-medium shrink-0 hover:bg-default-200 dark:hover:bg-zinc-700 transition-colors"
                      >
                        {t("downloadpage.topcontent.status", {
                          defaultValue: "状态",
                        })}
                      </Button>
                    </DropdownTrigger>
                    <DropdownMenu
                      selectionMode="single"
                      selectedKeys={[statusFilter]}
                      onSelectionChange={(keys) => {
                        const k = Array.from(keys)[0] as
                          | "all"
                          | "downloaded"
                          | "not_downloaded";
                        setStatusFilter(k);
                      }}
                    >
                      <DropdownItem key="all">
                        {t("downloadpage.topcontent.status_all", {
                          defaultValue: "全部",
                        })}
                      </DropdownItem>
                      <DropdownItem key="downloaded">
                        {t("downloadpage.topcontent.status_downloaded", {
                          defaultValue: "已下载",
                        })}
                      </DropdownItem>
                      <DropdownItem key="not_downloaded">
                        {t("downloadpage.topcontent.status_not_downloaded", {
                          defaultValue: "未下载",
                        })}
                      </DropdownItem>
                    </DropdownMenu>
                  </Dropdown>
                </div>
              </div>

              <Table
                aria-label={
                  t("downloadpage.table.aria_label", {
                    defaultValue: "versions-table",
                  }) as unknown as string
                }
                className="bg-transparent flex-1 overflow-auto min-h-0"
                removeWrapper
                classNames={{
                   th: "bg-transparent text-default-500 border-b border-default-200 dark:border-white/10",
                   td: "py-3 border-b border-default-100 dark:border-white/5 last:border-0",
                }}
                bottomContent={
                  <div className="flex items-center justify-between px-6 py-4 border-t border-default-200 dark:border-white/10 bg-transparent">
                    <div className="text-sm text-default-500">
                      {t("downloadpage.bottomcontent.total", {
                        count: filtered.length,
                      })}
                    </div>
                    <div className="flex items-center gap-3 ml-auto">
                      <Pagination
                        total={totalPages}
                        page={page}
                        onChange={setPage}
                        radius="full"
                        showControls
                        variant="light" 
                        classNames={{
                          cursor: "bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
                        }}
                      />
                    </div>
                  </div>
                }
              >
            <TableHeader>
              <TableColumn>
                {t("downloadpage.table.header.version")}
              </TableColumn>
              <TableColumn>{t("downloadpage.table.header.type")}</TableColumn>
              <TableColumn>{t("downloadpage.table.header.status")}</TableColumn>
              <TableColumn className="text-right">
                {t("downloadpage.table.header.actions")}
              </TableColumn>
            </TableHeader>
            <TableBody
              emptyContent={
                t("downloadpage.table.empty", {
                  defaultValue: "暂无数据",
                }) as unknown as string
              }
              items={paged}
            >
              {(item: VersionItem) => (
                <TableRow
                  key={`${item.type}-${item.short}`}
                  className="animate-fadeInUp"
                >
                  <TableCell>{item.short}</TableCell>
                  <TableCell>
                    <Chip
                      color={item.type === "Release" ? "warning" : "secondary"}
                      variant="flat"
                    >
                      {item.type === "Release"
                        ? t("downloadpage.table.type.release", {
                            defaultValue: "Release",
                          })
                        : t("downloadpage.table.type.preview", {
                            defaultValue: "Preview",
                          })}
                    </Chip>
                  </TableCell>
                  <TableCell>
                    {!hasStatus(item) && refreshing ? (
                      <Chip color="default" variant="flat">
                        {t("downloadpage.status.checking", {
                          defaultValue: "Checking…",
                        })}
                      </Chip>
                    ) : isDownloaded(item) ? (
                      <Chip
                        color="success"
                        variant="flat"
                      >
                        {t("downloadpage.status.downloaded", {
                          defaultValue: "Downloaded",
                        })}
                      </Chip>
                    ) : (
                      <Chip color="danger" variant="flat">
                        {t("downloadpage.status.not_downloaded", {
                          defaultValue: "Not downloaded",
                        })}
                      </Chip>
                    )}
                  </TableCell>
                  <TableCell className="text-right">
                    <motion.div
                      whileHover={{ scale: 1.03 }}
                      whileTap={{ scale: 0.97 }}
                      className="inline-flex items-center gap-2 min-w-[120px] justify-end"
                    >
                      {isDownloaded(item) ? (
                        <ButtonGroup radius="full" size="sm" variant="bordered" color="default" className="w-full">
                            <Button
                                className="px-3 h-8 font-bold flex-1"
                                startContent={<FaBoxOpen size={14} />}
                                onPress={() => {
                                  navigate("/install", {
                                    state: {
                                      mirrorVersion: item.short,
                                      mirrorType: item.type,
                                      returnTo: "/download",
                                    },
                                  });
                                }}
                            >
                                {t("downloadpage.mirror.install_button", { defaultValue: "安装" })}
                            </Button>
                            <Dropdown>
                                <DropdownTrigger>
                                    <Button isIconOnly className="h-8 min-w-8 w-8 px-0">
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
                                        typeof minecraft?.ResolveDownloadedMsixvc === "function"
                                      ) {
                                        fname = await minecraft.ResolveDownloadedMsixvc(
                                          `${item.type} ${item.short}`,
                                          String(item.type).toLowerCase()
                                        );
                                      }
                                    } catch {}
                                    setDeleteItem({
                                      short: item.short,
                                      type: item.type,
                                      fileName: fname || `${item.type} ${item.short}`,
                                    });
                                    deleteDisclosure.onOpen();
                                  }}
                                >
                                    <DropdownItem
                                      key="delete_msixvc"
                                      color="danger"
                                      startContent={<FaTrash size={12} />}
                                    >
                                      {t("downloadpage.actions.delete_installer", { defaultValue: "删除下载包" })}
                                    </DropdownItem>
                                </DropdownMenu>
                            </Dropdown>
                        </ButtonGroup>
                      ) : (
                          <Button
                            radius="full"
                            size="sm"
                            startContent={<FaDownload size={14} />}
                            className="px-4 h-8 w-full font-bold transition-transform hover:-translate-y-0.5"
                            variant="bordered"
                            color="default"
                            isDisabled={!hasStatus(item) && refreshing}
                            onPress={() => {
                              const urls = item.urls || [];
                              setMirrorUrls(urls);
                              setMirrorVersion(item.short);
                              setMirrorType(item.type);
                              setSelectedUrl(null);
                              setInstallMode(false);
                              setCurrentDownloadingInfo(item.short, item.type);
                              onOpen();
                              startMirrorTests(urls);
                            }}
                          >
                            {!hasStatus(item) && refreshing
                              ? t("downloadpage.status.checking", {
                                  defaultValue: "检查中…",
                                })
                              : t("downloadmodal.download_button")}
                          </Button>
                      )}
                    </motion.div>
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardBody>
      </Card>

      <BaseModal
        isOpen={isOpen}
        onOpenChange={onOpenChange}
        size="2xl"
        scrollBehavior="inside"
      >
        <ModalContent className="max-w-[820px] shadow-none">
          {(onClose) => (
            <>
              <BaseModalHeader className="flex flex-col gap-1 px-8 pt-6 pb-2">
                <div className="flex flex-col gap-2">
                  <motion.h2
                    className="text-3xl font-black tracking-tight text-emerald-600"
                    initial={{ opacity: 0, y: -10 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ duration: 0.3, ease: "easeOut" }}
                  >
                    {t("downloadpage.mirror.title", {
                      defaultValue: "选择下载镜像",
                    })}
                  </motion.h2>
                  <motion.div
                    className="flex items-center gap-2"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    transition={{ duration: 0.3, delay: 0.1 }}
                  >
                     <Chip size="sm" variant="flat" color={mirrorType === "Preview" ? "secondary" : "warning"} className="h-6">
                        {mirrorType}
                     </Chip>
                     <span className="text-small font-mono text-default-600 dark:text-zinc-400">{mirrorVersion}</span>
                  </motion.div>
                </div>
                
                <motion.div
                  className="mt-6 rounded-2xl bg-default-100/50 dark:bg-zinc-800 border border-default-200/50 dark:border-white/5 p-4"
                  initial={{ opacity: 0, y: 5 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.3, delay: 0.2 }}
                >
                  <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                    <div className="flex items-center gap-2 text-small font-bold text-default-700 dark:text-zinc-300 shrink-0">
                      <div className="w-1 h-4 rounded-full bg-emerald-500"></div>
                      {t("downloadpage.mirror.target", {
                        defaultValue: "下载目标",
                      })}
                    </div>
                    {(() => {
                      const target =
                        selectedUrl || (!testing ? bestMirror?.url : "");
                      if (!target)
                        return (
                          <div className="text-small text-default-400 dark:text-zinc-500 italic">
                            {testing
                              ? t("downloadpage.mirror.testing", {
                                  defaultValue: "正在测速...",
                                })
                              : t("downloadpage.mirror.unselected", {
                                  defaultValue: "请选择一个镜像",
                                })}
                          </div>
                        );
                      const domain = labelFromUrl(target);
                      const fname = fileNameFromUrl(target);
                      return (
                        <div className="flex items-center gap-3 min-w-0 bg-white/50 dark:bg-black/20 rounded-xl px-3 py-1.5 border border-black/5 dark:border-white/5">
                          <div className="text-small truncate max-w-[400px] text-default-700 dark:text-zinc-300">
                            <span className="font-semibold text-emerald-600 dark:text-emerald-500">{domain}</span>
                            <span className="mx-1.5 opacity-30">|</span>
                            {fname}
                          </div>
                          <Button
                            size="sm"
                            variant="flat"
                            className="h-7 min-w-20 bg-default-200/50 dark:bg-white/10"
                            onPress={() =>
                              navigator.clipboard?.writeText(target)
                            }
                            startContent={<FaCopy size={12} />}
                          >
                            {t("downloadpage.mirror.copy_link", {
                              defaultValue: "复制",
                            })}
                          </Button>
                        </div>
                      );
                    })()}
                  </div>
                </motion.div>
              </BaseModalHeader>
              <BaseModalBody className="[&::-webkit-scrollbar]:hidden">
                <motion.div
                  initial={{ opacity: 0, y: 10 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.3, delay: 0.25 }}
                >
                  {mirrorUrls && mirrorUrls.length > 0 ? (
                    <div className="flex flex-col gap-4">
                      {/* Recommended Section */}
                      <div>
                          <div className="flex items-center gap-3 mb-3 px-1">
                            <span className="text-sm font-bold text-default-800 dark:text-zinc-300 uppercase tracking-wider">
                              {t("downloadpage.mirror.recommended", {
                                defaultValue: "推荐镜像",
                              })}
                            </span>
                            {testing && (
                              <Chip size="sm" variant="flat" color="success" startContent={<Spinner size="sm" color="success" className="w-3 h-3" />}>
                                {t("downloadpage.mirror.auto_testing", { defaultValue: "测速中" })}
                              </Chip>
                            )}
                          </div>
                          
                          {bestMirror ? (
                            <div
                              className={`group relative overflow-hidden flex items-center justify-between gap-4 rounded-2xl border-2 p-4 transition-all cursor-pointer ${
                                selectedUrl === bestMirror.url
                                  ? "border-emerald-500 bg-emerald-500/5 shadow-xl shadow-emerald-500/10"
                                  : "border-transparent bg-default-50 dark:bg-zinc-800/50 hover:bg-default-100 dark:hover:bg-zinc-800"
                              }`}
                              onClick={() => setSelectedUrl(bestMirror.url)}
                            >
                              <div className="flex items-center gap-4 min-w-0 z-10">
                                <div className={`w-10 h-10 rounded-full flex items-center justify-center shrink-0 ${
                                    selectedUrl === bestMirror.url ? "bg-emerald-500 text-white" : "bg-default-200 text-default-500"
                                }`}>
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
                                    <span className="text-tiny text-default-500 font-medium">延迟</span>
                                    <span className={`text-medium font-bold ${bestMirror.ok ? "text-emerald-500" : "text-danger-500"}`}>
                                        {typeof bestMirror.latencyMs === "number" ? `${Math.round(bestMirror.latencyMs)}ms` : "-"}
                                    </span>
                                </div>
                                {selectedUrl === bestMirror.url && (
                                    <motion.div layoutId="selected-check" className="text-emerald-500">
                                        <div className="w-6 h-6 rounded-full bg-emerald-500 flex items-center justify-center">
                                            <svg className="w-3.5 h-3.5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="3" d="M5 13l4 4L19 7" /></svg>
                                        </div>
                                    </motion.div>
                                )}
                              </div>
                            </div>
                          ) : (
                            <div className="p-8 rounded-2xl border border-dashed border-default-300 flex flex-col items-center justify-center text-default-500 gap-2">
                                <span>暂无推荐镜像</span>
                            </div>
                          )}
                      </div>

                      {/* Others Section */}
                      <div className="flex flex-col gap-2">
                        <div className="text-sm font-bold text-default-800 dark:text-zinc-300 uppercase tracking-wider px-1">
                            {t("downloadpage.mirror.others", { defaultValue: "其它镜像" })}
                        </div>
                        <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                          {mirrorResults.map((m, i) => (
                            <div
                              key={`mirror-${i}`}
                              className={`relative flex items-center justify-between gap-3 rounded-xl border p-2.5 transition-all cursor-pointer ${
                                selectedUrl === m.url
                                  ? "border-emerald-500/50 bg-emerald-500/5"
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
                                <span className={`text-xs font-bold ${
                                    typeof m.latencyMs === 'number' 
                                        ? m.latencyMs < 100 ? 'text-emerald-500' : m.latencyMs < 300 ? 'text-warning-500' : 'text-danger-500'
                                        : 'text-default-400'
                                }`}>
                                  {typeof m.latencyMs === "number"
                                    ? `${Math.round(m.latencyMs)}ms`
                                    : testing
                                    ? "..."
                                    : "-"}
                                </span>
                              </div>
                              {selectedUrl === m.url && (
                                <div className="absolute inset-0 border-2 border-emerald-500 rounded-xl pointer-events-none" />
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
                      <p>
                        {t("downloadpage.mirror.no_mirrors", {
                          defaultValue: "当前版本无可用镜像。",
                        })}
                      </p>
                    </div>
                  )}
                </motion.div>
              </BaseModalBody>
              <BaseModalFooter>
                <Button 
                    className="font-medium text-default-500 hover:text-default-700"
                    variant="light" 
                    onPress={onClose}
                >
                  {t("common.cancel")}
                </Button>
                <Button
                  variant="flat"
                  color="default"
                  className="bg-default-100 dark:bg-white/10"
                  startContent={<FaSync className={testing ? "animate-spin" : ""} />}
                  onPress={() => {
                    startMirrorTests(mirrorUrls || []);
                  }}
                >
                  {t("downloadpage.mirror.retest", {
                    defaultValue: "重新测速",
                  })}
                </Button>
                <Button
                  className="font-bold text-white shadow-lg shadow-emerald-900/20 bg-emerald-600 hover:bg-emerald-500 hover:scale-[1.02] active:scale-[0.98] transition-transform"
                  radius="full"
                  size="lg"
                  isDisabled={!selectedUrl}
                  startContent={installMode ? null : <FaDownload />}
                  onPress={() => {
                    if (!selectedUrl) return;
                    ensureDlSubscriptions();
                    if (installMode) {
                      navigate("/install", {
                        state: {
                          mirrorVersion,
                          mirrorType,
                          returnTo: "/download",
                        },
                      });
                      onClose?.();
                    } else {
                      onClose?.();
                      if (hasBackend && minecraft?.StartMsixvcDownload) {
                        setDlError("");
                        setDlProgress(null);
                        
                        let urlWithFilename = selectedUrl;
                        try {
                          const u = new URL(selectedUrl);
                          const desired = `${
                            mirrorType || "Release"
                          } ${mirrorVersion}.msixvc`;
                          u.searchParams.set("filename", desired);
                          urlWithFilename = u.toString();
                        } catch {
                          const desired = `${
                            mirrorType || "Release"
                          } ${mirrorVersion}.msixvc`;
                          const sep = selectedUrl.includes("?") ? "&" : "?";
                          urlWithFilename = `${selectedUrl}${sep}filename=${encodeURIComponent(
                            desired
                          )}`;
                        }

                        setTimeout(() => {
                          progressDisclosure.onOpen();
                          minecraft.StartMsixvcDownload(urlWithFilename);
                        }, 200);
                      } else {
                        window.open(selectedUrl, "_blank");
                      }
                    }
                  }}
                >
                  {installMode
                    ? t("downloadpage.mirror.install_selected", {
                        defaultValue: "安装所选镜像",
                      })
                    : t("downloadpage.mirror.download_selected", {
                        defaultValue: "下载所选镜像",
                      })}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>

      {/* Delete confirm modal */}
      <BaseModal
        isOpen={deleteDisclosure.isOpen}
        onOpenChange={deleteDisclosure.onOpenChange}
        size="md"
      >
        <ModalContent className="shadow-none">
          {(onClose) => (
            <>
              <BaseModalHeader className="flex items-center gap-3 text-danger-600">
                <motion.div
                  initial={{ scale: 0 }}
                  animate={{ scale: 1 }}
                  transition={{ type: "spring", stiffness: 260, damping: 20 }}
                  className="w-12 h-12 rounded-full bg-red-500/10 border border-red-500/20 flex items-center justify-center shrink-0"
                >
                  <svg
                    viewBox="0 0 24 24"
                    width="24"
                    height="24"
                    className="text-red-500"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2.5"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <path d="M3 6h18" />
                    <path d="M8 6v-2a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
                    <path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" />
                  </svg>
                </motion.div>
                <motion.h2
                  className="text-xl font-bold text-default-900 dark:text-white"
                  initial={{ opacity: 0, x: -10 }}
                  animate={{ opacity: 1, x: 0 }}
                  transition={{ duration: 0.25 }}
                >
                  {t("downloadpage.delete.title", {
                    defaultValue: "确认删除下载包",
                  })}
                </motion.h2>
              </BaseModalHeader>
              <BaseModalBody>
                <motion.div
                  initial={{ opacity: 0, y: 10 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.25, delay: 0.1 }}
                >
                  <div className="text-medium text-default-700 dark:text-zinc-300 font-bold">
                    {t("downloadpage.delete.body", {
                      defaultValue: "将删除安装包：",
                    })}
                  </div>
                  <div className="mt-2 p-3 bg-default-100/50 dark:bg-zinc-800 rounded-xl border border-default-200/50">
                    <span className="font-mono text-default-800 dark:text-zinc-200 font-bold break-all text-small">
                      {deleteItem?.fileName?.toLowerCase()?.endsWith(".msixvc")
                        ? deleteItem?.fileName
                        : `${deleteItem?.fileName || ""}.msixvc`}
                    </span>
                  </div>
                  <div className="text-small text-danger-500 mt-3 font-bold flex items-center gap-2">
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" /></svg>
                    {t("downloadpage.delete.warning", {
                      defaultValue: "此操作不可恢复。",
                    })}
                  </div>
                  {deleteError ? (
                    <motion.div
                      initial={{ opacity: 0, height: 0 }}
                      animate={{ opacity: 1, height: "auto" }}
                      className="text-small text-white bg-danger-500/90 px-3 py-2 rounded-lg mt-3"
                    >
                      {trErr(deleteError)}
                    </motion.div>
                  ) : null}
                </motion.div>
              </BaseModalBody>
              <BaseModalFooter>
                <Button
                  variant="light"
                  color="default"
                  onPress={() => {
                    onClose?.();
                  }}
                >
                  {t("common.cancel", { defaultValue: "取消" })}
                </Button>
                <Button
                  color="danger"
                  className="bg-red-500 shadow-lg shadow-red-500/30 text-white font-bold"
                  radius="full"
                  isLoading={deleteLoading}
                  onPress={async () => {
                    if (!hasBackend) {
                      setDeleteError("ERR_WRITE_TARGET");
                      return;
                    }
                    setDeleteError("");
                    setDeleteLoading(true);
                    try {
                      if (
                        typeof minecraft?.DeleteDownloadedMsixvc !== "function"
                      ) {
                        setDeleteError("ERR_WRITE_TARGET");
                        setDeleteLoading(false);
                        return;
                      }
                      const msg: string =
                        await minecraft.DeleteDownloadedMsixvc(
                          `${String(deleteItem?.type)} ${String(
                            deleteItem?.short
                          )}`,
                          String(deleteItem?.type).toLowerCase()
                        );
                      if (msg) {
                        setDeleteError(msg);
                        setDeleteLoading(false);
                        return;
                      }
                      setDeleteLoading(false);
                      try {
                        onClose?.();
                      } catch {}
                      try {
                        await refreshOne(
                          String(deleteItem?.short || ""),
                          String(deleteItem?.type || "release").toLowerCase()
                        );
                      } catch {}
                      try {
                        const disp = deleteItem?.fileName
                          ?.toLowerCase()
                          ?.endsWith(".msixvc")
                          ? deleteItem?.fileName
                          : `${deleteItem?.fileName}.msixvc`;
                        setDeleteSuccessMsg(disp || "");
                      } catch {
                        setDeleteSuccessMsg(String(deleteItem?.fileName || ""));
                      }
                      deleteSuccessDisclosure.onOpen();
                    } catch (e: any) {
                      setDeleteError(String(e || ""));
                      setDeleteLoading(false);
                    }
                  }}
                >
                  {t("downloadpage.delete.confirm", { defaultValue: "删除" })}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>

      {/* Delete success modal */}
      <BaseModal
        isOpen={deleteSuccessDisclosure.isOpen}
        onOpenChange={deleteSuccessDisclosure.onOpenChange}
        size="md"
      >
        <ModalContent className="shadow-none">
          {(onClose) => (
            <>
              <BaseModalHeader className="flex items-center gap-3 px-8 pt-8 pb-4 text-success-600">
                <motion.div
                  initial={{ scale: 0 }}
                  animate={{ scale: 1 }}
                  transition={{ type: "spring", stiffness: 260, damping: 20 }}
                  className="w-10 h-10 rounded-full bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center shrink-0"
                >
                  <svg
                    viewBox="0 0 24 24"
                    width="20"
                    height="20"
                    className="text-emerald-500"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="3"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <path d="M20 6L9 17l-5-5" />
                  </svg>
                </motion.div>
                <motion.h2
                  className="text-xl font-bold text-default-900 dark:text-white"
                  initial={{ opacity: 0, x: -10 }}
                  animate={{ opacity: 1, x: 0 }}
                  transition={{ duration: 0.25 }}
                >
                  {t("downloadpage.delete.success_title", {
                    defaultValue: "删除完成",
                  })}
                </motion.h2>
              </BaseModalHeader>
              <BaseModalBody className="px-8 py-4">
                <motion.div
                  initial={{ opacity: 0, y: 10 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.25, delay: 0.1 }}
                  className="text-medium text-default-700 dark:text-zinc-300 font-bold"
                >
                  {t("downloadpage.delete.success_body", {
                    defaultValue: "已删除安装包：",
                  })}
                  <div className="mt-2 p-3 bg-default-100/50 rounded-xl border border-default-200/50 font-mono text-small text-default-800 dark:text-zinc-200 font-bold break-all">
                    {deleteSuccessMsg}
                  </div>
                </motion.div>
              </BaseModalBody>
              <BaseModalFooter className="px-8 pb-8">
                <Button
                  color="success"
                  className="bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
                  radius="full"
                  onPress={() => {
                    onClose?.();
                  }}
                >
                  {t("common.ok", { defaultValue: "确定" })}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>

      {/* Download success modal */}
      <BaseModal
        isOpen={downloadSuccessDisclosure.isOpen}
        onOpenChange={downloadSuccessDisclosure.onOpenChange}
        size="md"
      >
        <ModalContent className="shadow-none">
          {(onClose) => (
            <>
              <BaseModalHeader className="flex items-center gap-3 px-8 pt-8 pb-4 text-success-600">
                <div className="w-10 h-10 rounded-full bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center shrink-0">
                  <svg
                    viewBox="0 0 24 24"
                    width="20"
                    height="20"
                    className="text-emerald-500"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="3"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <path d="M20 6L9 17l-5-5" />
                  </svg>
                </div>
                <h2 className="text-xl font-bold text-default-900 dark:text-white">
                  {t("downloadpage.download.success_title", {
                    defaultValue: "下载完成",
                  })}
                </h2>
              </BaseModalHeader>
              <BaseModalBody className="px-8 py-4">
                <div className="text-medium text-default-700 dark:text-zinc-300 font-bold">
                  {t("downloadpage.download.success_body", {
                    defaultValue: "文件已下载：",
                  })}
                  <div className="mt-2 p-3 bg-default-100/50 dark:bg-zinc-800 rounded-xl border border-default-200/50 font-mono text-small text-default-800 dark:text-zinc-200 font-bold break-all">
                    {downloadSuccessMsg}
                  </div>
                </div>
              </BaseModalBody>
              <BaseModalFooter className="px-8 pb-8">
                <Button
                  color="primary"
                  className="bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
                  radius="full"
                  onPress={() => {
                    try {
                      if (mirrorVersion && mirrorType) {
                        markDownloaded(mirrorVersion, String(mirrorType));
                      }
                    } catch {}
                    onClose?.();
                  }}
                >
                  {t("common.ok", { defaultValue: "确定" })}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>

      {/* Install success modal */}
      <BaseModal
        isOpen={installSuccessDisclosure.isOpen}
        onOpenChange={installSuccessDisclosure.onOpenChange}
        size="md"
      >
        <ModalContent className="shadow-none">
          {(onClose) => (
            <>
              <BaseModalHeader className="flex items-center gap-4 px-8 pt-8 pb-4 text-success-600">
                <motion.div
                  initial={{ scale: 0 }}
                  animate={{ scale: 1 }}
                  transition={{ type: "spring", stiffness: 260, damping: 20 }}
                  className="w-12 h-12 rounded-full bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center shrink-0"
                >
                  <svg
                    viewBox="0 0 24 24"
                    width="24"
                    height="24"
                    className="text-emerald-500"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2.5"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <path d="M20 6L9 17l-5-5" />
                  </svg>
                </motion.div>
                <div className="flex flex-col">
                  <motion.h2
                    className="text-xl font-bold text-default-900 dark:text-white"
                    initial={{ opacity: 0, x: -10 }}
                    animate={{ opacity: 1, x: 0 }}
                    transition={{ duration: 0.25 }}
                  >
                    {t("downloadpage.install.success_title", {
                      defaultValue: "安装完成",
                    })}
                  </motion.h2>
                  {installingVersion ? (
                    <motion.div
                      className="text-small text-default-600 dark:text-zinc-400 font-medium"
                      initial={{ opacity: 0 }}
                      animate={{ opacity: 1 }}
                      transition={{ delay: 0.1 }}
                    >
                      {t("downloadpage.install.version_label", {
                        defaultValue: "版本",
                      })}
                      : {installingVersion}
                    </motion.div>
                  ) : null}
                </div>
              </BaseModalHeader>
              <BaseModalBody className="px-8 py-4">
                <motion.div
                  initial={{ opacity: 0, y: 10 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.25, delay: 0.1 }}
                  className="flex flex-col gap-4"
                >
                  <div className="text-medium text-default-700 dark:text-zinc-300 font-bold">
                    {installSuccessMsg ||
                      (t("downloadpage.install.success", {
                        defaultValue: "版本已成功安装。",
                      }) as unknown as string)}
                  </div>
                  {installingTargetName ? (
                    <div className="p-3 bg-default-100/50 dark:bg-zinc-800 rounded-xl border border-default-200/50 text-small text-default-600 dark:text-zinc-400 font-medium">
                      {t("downloadpage.install.target", {
                        defaultValue: "安装目标",
                      })}
                      :{" "}
                      <span className="font-mono text-default-800 dark:text-zinc-200 font-bold">
                        {installingTargetName}
                      </span>
                    </div>
                  ) : null}
                </motion.div>
              </BaseModalBody>
              <BaseModalFooter className="px-8 pb-8 pt-4">
                <Button
                  className="bg-default-100 text-default-700 font-bold hover:bg-default-200"
                  radius="full"
                  onPress={() => {
                    onClose?.();
                  }}
                >
                  {t("common.back", { defaultValue: "返回" })}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>

      {/* Install error modal */}
      <BaseModal
        isOpen={installErrorDisclosure.isOpen}
        onOpenChange={installErrorDisclosure.onOpenChange}
        size="md"
      >
        <ModalContent className="shadow-none">
          {(onClose) => (
            <>
              <BaseModalHeader className="flex items-center gap-4 text-danger-600">
                <motion.div
                  initial={{ scale: 0 }}
                  animate={{ scale: 1 }}
                  transition={{ type: "spring", stiffness: 260, damping: 20 }}
                  className="w-12 h-12 rounded-full bg-red-500/10 border border-red-500/20 flex items-center justify-center shrink-0"
                >
                  <svg
                    viewBox="0 0 24 24"
                    width="24"
                    height="24"
                    className="text-red-500"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2.5"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <path d="M12 9v4" />
                    <path d="M12 17h.01" />
                  </svg>
                </motion.div>
                <motion.h2
                  className="text-xl font-bold text-default-900 dark:text-white"
                  initial={{ opacity: 0, x: -10 }}
                  animate={{ opacity: 1, x: 0 }}
                  transition={{ duration: 0.25 }}
                >
                  {t("downloadpage.progress.unknown_error", {
                    defaultValue: "未知错误",
                  })}
                </motion.h2>
              </BaseModalHeader>
              <BaseModalBody>
                <motion.div
                  initial={{ opacity: 0, y: 10 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.25, delay: 0.1 }}
                  className="text-medium text-default-700 dark:text-zinc-300 font-bold"
                >
                  {trErr(installError)}
                </motion.div>
              </BaseModalBody>
              <BaseModalFooter>
                <Button
                  className="bg-default-100 text-default-700 font-bold hover:bg-default-200"
                  radius="full"
                  onPress={() => {
                    onClose?.();
                  }}
                >
                  {t("common.close", { defaultValue: "关闭" })}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>

      {/* Install progress modal */}
      <BaseModal
        isOpen={installLoadingDisclosure.isOpen}
        onOpenChange={installLoadingDisclosure.onOpenChange}
        size="md"
        hideCloseButton
        isDismissable={false}
      >
        <ModalContent className="shadow-none">
          {() => (
            <>
              <BaseModalHeader className="flex items-center gap-4 text-primary-600">
                <Spinner size="lg" color="primary" />
                <motion.h2
                  className="text-xl font-bold text-default-900 dark:text-white"
                  initial={{ opacity: 0, x: -10 }}
                  animate={{ opacity: 1, x: 0 }}
                  transition={{ duration: 0.25 }}
                >
                  {t("downloadpage.install.title", {
                    defaultValue: "正在安装",
                  })}
                </motion.h2>
              </BaseModalHeader>
              <BaseModalBody>
                <div className="flex flex-col gap-6">
                  <div className="text-medium text-default-700 dark:text-zinc-300 font-bold">
                    {t("downloadpage.install.hint", {
                      defaultValue: "请稍候，正在卸载旧版本并注册安装包...",
                    })}
                  </div>
                  <div className="flex items-center gap-3">
                    <Progress
                      aria-label="install-progress"
                      isIndeterminate
                      size="md"
                      classNames={{
                        indicator: "bg-linear-to-r from-emerald-500 to-teal-500",
                        track: "bg-default-100",
                      }}
                      className="flex-1"
                    />
                  </div>
                  {typeof extractInfo?.bytes === "number" &&
                  extractInfo.bytes > 0 ? (
                    <div className="text-small text-default-600 dark:text-zinc-400 font-medium">
                      {t("downloadpage.install.estimated_size", {
                        defaultValue: "已写入大小（估算）",
                      })}
                      :{" "}
                      {(() => {
                        const n = extractInfo?.bytes ?? 0;
                        const kb = 1024;
                        const mb = kb * 1024;
                        const gb = mb * 1024;
                        if (n >= gb) return (n / gb).toFixed(2) + " GB";
                        if (n >= mb) return (n / mb).toFixed(2) + " MB";
                        if (n >= kb) return (n / kb).toFixed(2) + " KB";
                        return n + " B";
                      })()}
                    </div>
                  ) : null}
                  {installingTargetName ? (
                    <div className="p-3 bg-default-100/50 rounded-xl border border-default-200/50 text-small text-default-600 dark:text-zinc-400 font-medium">
                      {t("downloadpage.install.target", {
                        defaultValue: "安装目标",
                      })}
                      :{" "}
                      <span className="font-mono text-default-700 dark:text-zinc-200 font-bold">
                        {installingTargetName}
                      </span>
                    </div>
                  ) : null}
                </div>
              </BaseModalBody>
            </>
          )}
        </ModalContent>
      </BaseModal>

      {/* Download progress modal */}
      <BaseModal
        isOpen={progressDisclosure.isOpen}
        onOpenChange={progressDisclosure.onOpenChange}
        size="md"
        hideCloseButton
        isDismissable={false}
      >
        <ModalContent className="shadow-none">
          {() => (
            <>
              <BaseModalHeader className="flex items-center gap-4 text-primary-600">
                <div className="w-12 h-12 rounded-full bg-emerald-500/10 dark:bg-emerald-500/20 border border-emerald-500/20 flex items-center justify-center shrink-0">
                  <FaDownload className="text-emerald-500 animate-pulse" size={20} />
                </div>
                <div className="flex flex-col">
                  <motion.h2
                    className="text-xl font-bold text-default-900 dark:text-white"
                    initial={{ opacity: 0, x: -10 }}
                    animate={{ opacity: 1, x: 0 }}
                    transition={{ duration: 0.25 }}
                  >
                    {t("downloadpage.progress.title", {
                      defaultValue: "下载进度",
                    })}
                  </motion.h2>
                </div>
              </BaseModalHeader>
              <BaseModalBody>
                <div className="flex flex-col gap-4">
                  {dlError ? (
                    <div className="p-4 rounded-xl bg-danger-500/10 border border-danger-500/20 text-danger-600 font-bold">
                      {trErr(dlError)}
                    </div>
                  ) : (
                    <div className="flex flex-col gap-3 p-5 rounded-2xl bg-default-50 dark:bg-zinc-800/40 border border-default-100 dark:border-white/5">
                      <div className="flex items-center justify-between text-small font-bold text-default-700 dark:text-zinc-200">
                        <span>
                          {dlProgress
                            ? `${(
                                (dlProgress.downloaded / dlProgress.total) *
                                100
                              ).toFixed(1)}%`
                            : "0%"}
                        </span>
                        <span className="font-mono opacity-80">
                          {dlSpeed < 1024 * 1024
                            ? `${(dlSpeed / 1024).toFixed(1)} KB/s`
                            : `${(dlSpeed / (1024 * 1024)).toFixed(1)} MB/s`}
                        </span>
                      </div>
                      <Progress
                        aria-label="download-progress"
                        value={
                          dlProgress
                            ? (dlProgress.downloaded / dlProgress.total) * 100
                            : 0
                        }
                        size="md"
                        classNames={{
                          indicator: "bg-linear-to-r from-emerald-500 to-teal-500 shadow-lg shadow-emerald-900/20",
                          track: "bg-default-200/50 dark:bg-white/5",
                        }}
                      />
                      <div className="flex justify-between text-tiny text-default-400 dark:text-zinc-500 font-mono">
                        <span>
                          {dlProgress
                            ? `${(
                                dlProgress.downloaded /
                                (1024 * 1024)
                              ).toFixed(2)} MB`
                            : "0 MB"}
                        </span>
                        <span>
                          {dlProgress
                            ? `${(dlProgress.total / (1024 * 1024)).toFixed(
                                2
                              )} MB`
                            : "0 MB"}
                        </span>
                      </div>
                    </div>
                  )}
                </div>
              </BaseModalBody>
              <BaseModalFooter>
                <Button
                  color="danger"
                  variant="flat"
                  onPress={() => {
                    if (hasBackend && minecraft?.CancelMsixvcDownload) {
                      minecraft.CancelMsixvcDownload();
                    }
                    setDlError("");
                    setDlProgress(null);
                    progressDisclosure.onClose();
                  }}
                  radius="full"
                >
                  {t("common.cancel", { defaultValue: "取消" })}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>
        </div>
      </motion.div>
    </>
  );
};
