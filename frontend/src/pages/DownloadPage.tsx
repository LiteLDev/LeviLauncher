"use client";
import React, { useEffect, useMemo, useState, useRef } from "react";
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
  Modal,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  Progress,
  Spinner,
  Accordion,
  AccordionItem,
  useDisclosure,
  Card,
  CardBody,
} from "@heroui/react";
import { FaDownload, FaCopy, FaSync, FaTrash } from "react-icons/fa";
import { Events } from "@wailsio/runtime";
import { useVersionStatus } from "../utils/VersionStatusContext";
import { useTranslation } from "react-i18next";
import { motion } from "framer-motion";
import { useNavigate } from "react-router-dom";
import * as minecraft from "../../bindings/github.com/liteldev/LeviLauncher/minecraft";

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
    <motion.div
      className="px-4 sm:px-6 lg:px-8 py-6 w-full max-w-none"
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.25 }}
    >
      <Card className="rounded-2xl shadow-md bg-white/70 dark:bg-black/30 backdrop-blur-md border border-white/30">
        <CardBody className="p-0">
          <Table
            aria-label={
              t("downloadpage.table.aria_label", {
                defaultValue: "versions-table",
              }) as unknown as string
            }
            className="bg-transparent"
            classNames={{
              wrapper: "bg-transparent",
            }}
            topContent={
              <motion.div
                className="flex items-center justify-between gap-3 sm:gap-4 whitespace-nowrap overflow-x-auto px-4 pt-3"
                initial={{ opacity: 0, y: -6 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.25 }}
              >
                <div className="flex items-center gap-3 sm:gap-4 min-w-0 flex-1">
                  <Input
                    className="flex-1 min-w-[220px]"
                    placeholder={t("downloadpage.topcontent.input.placeholder")}
                    value={query}
                    onValueChange={setQuery}
                    radius="full"
                  />
                </div>
                <div className="flex items-center gap-3 sm:gap-4 shrink-0">
                  <Button
                    radius="full"
                    variant="bordered"
                    className="shrink-0"
                    startContent={<FaSync size={14} />}
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
                    variant="bordered"
                    className="shrink-0"
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
                        variant="bordered"
                        className="shrink-0"
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
                        variant="bordered"
                        className="shrink-0"
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
              </motion.div>
            }
            bottomContent={
              <div className="flex items-center justify-between px-4 py-3 gap-3">
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
                      <Dropdown>
                        <DropdownTrigger>
                          <Chip
                            color="success"
                            variant="flat"
                            className="cursor-pointer"
                          >
                            {t("downloadpage.status.downloaded", {
                              defaultValue: "Downloaded",
                            })}
                          </Chip>
                        </DropdownTrigger>
                        <DropdownMenu
                          aria-label="Downloaded actions"
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
                            {t("downloadpage.actions.delete_installer", {
                              defaultValue: "删除下载包",
                            })}
                          </DropdownItem>
                        </DropdownMenu>
                      </Dropdown>
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
                      className="inline-flex items-center gap-2 w-[120px] justify-end"
                    >
                      <Button
                        radius="full"
                        variant="ghost"
                        color="default"
                        size="sm"
                        startContent={<FaDownload size={14} />}
                        className="px-4 h-8 rounded-full font-medium transition-transform hover:-translate-y-0.5 w-full"
                        isDisabled={!hasStatus(item) && refreshing}
                        onPress={() => {
                          const urls = item.urls || [];
                          setMirrorUrls(urls);
                          setMirrorVersion(item.short);
                          setMirrorType(item.type);
                          setSelectedUrl(null);
                          const already = Boolean(isDownloaded(item));
                          setInstallMode(already);
                          if (already) {
                            navigate("/install", {
                              state: {
                                mirrorVersion: item.short,
                                mirrorType: item.type,
                                returnTo: "/download",
                              },
                            });
                          } else {
                            setCurrentDownloadingInfo(item.short, item.type);
                            onOpen();
                            startMirrorTests(urls);
                          }
                        }}
                      >
                        {!hasStatus(item) && refreshing
                          ? t("downloadpage.status.checking", {
                              defaultValue: "检查中…",
                            })
                          : isDownloaded(item)
                          ? t("downloadpage.mirror.install_button", {
                              defaultValue: "安装",
                            })
                          : t("downloadmodal.download_button")}
                      </Button>
                    </motion.div>
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardBody>
      </Card>

      <Modal
        isOpen={isOpen}
        onOpenChange={onOpenChange}
        size="2xl"
        scrollBehavior="inside"
      >
        <ModalContent className="max-w-[820px]">
          {(onClose) => (
            <>
              <ModalHeader className="flex flex-col gap-1">
                <h2 className="text-xl font-bold">
                  {t("downloadpage.mirror.title", {
                    defaultValue: "选择下载镜像",
                  })}
                </h2>
                <p className="text-small text-default-500">{mirrorVersion}</p>
                <div className="rounded-medium bg-content2 border border-default-200 px-3 py-1">
                  <div className="flex items-center justify-between gap-2">
                    <div className="text-small text-default-500">
                      {t("downloadpage.mirror.target", {
                        defaultValue: "下载目标",
                      })}
                    </div>
                    {(() => {
                      const target =
                        selectedUrl || (!testing ? bestMirror?.url : "");
                      if (!target)
                        return (
                          <div className="text-small">
                            {testing
                              ? t("downloadpage.mirror.testing", {
                                  defaultValue: "测速中…",
                                })
                              : t("downloadpage.mirror.unselected", {
                                  defaultValue: "未选择",
                                })}
                          </div>
                        );
                      const domain = labelFromUrl(target);
                      const fname = fileNameFromUrl(target);
                      return (
                        <div className="flex items-center gap-3 min-w-0">
                          <div className="text-small truncate max-w-[520px]">
                            {domain} · {fname}
                          </div>
                          <Button
                            size="sm"
                            variant="light"
                            className="h-6 px-2"
                            onPress={() =>
                              navigator.clipboard?.writeText(target)
                            }
                            startContent={<FaCopy size={12} />}
                          >
                            {t("downloadpage.mirror.copy_link", {
                              defaultValue: "复制链接",
                            })}
                          </Button>
                        </div>
                      );
                    })()}
                  </div>
                </div>
              </ModalHeader>
              <ModalBody>
                {mirrorUrls && mirrorUrls.length > 0 ? (
                  <div className="flex flex-col gap-4">
                    <div className="rounded-lg border border-default-200 p-3">
                      <div className="flex items-center gap-3 min-w-0">
                        <span className="font-medium">
                          {t("downloadpage.mirror.recommended", {
                            defaultValue: "推荐镜像",
                          })}
                        </span>
                        {testing ? (
                          <span className="flex items-center gap-2 text-small text-default-500">
                            <Spinner size="sm" />{" "}
                            {t("downloadpage.mirror.auto_testing", {
                              defaultValue: "自动测速中…",
                            })}
                          </span>
                        ) : null}
                      </div>
                      {bestMirror ? (
                        <div
                          className={`mt-2 flex items-center justify-between gap-2 rounded-md border p-2 transition-shadow hover:shadow-sm cursor-pointer ${
                            selectedUrl === bestMirror.url
                              ? "border-primary bg-primary/5"
                              : "border-default-200 hover:bg-content2"
                          }`}
                          onClick={() => setSelectedUrl(bestMirror.url)}
                        >
                          <div className="min-w-0">
                            <div className="font-medium text-small truncate">
                              {bestMirror.label}
                            </div>
                          </div>
                          <div className="flex items-center gap-2 shrink-0">
                            <Chip
                              color={bestMirror.ok ? "success" : "danger"}
                              variant="flat"
                              size="sm"
                            >
                              {typeof bestMirror.latencyMs === "number"
                                ? `${Math.round(bestMirror.latencyMs)}ms`
                                : "-"}
                            </Chip>
                          </div>
                        </div>
                      ) : (
                        <div className="text-small text-default-500 mt-2">
                          {t("downloadpage.mirror.no_recommended", {
                            defaultValue: "暂无推荐镜像",
                          })}
                        </div>
                      )}
                    </div>

                    <Accordion
                      variant="bordered"
                      selectionMode="multiple"
                      defaultSelectedKeys={[]}
                    >
                      <AccordionItem
                        key="list"
                        title={t("downloadpage.mirror.others", {
                          defaultValue: "其它镜像",
                        })}
                      >
                        <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
                          {mirrorResults.map((m, i) => (
                            <div
                              key={`mirror-${i}`}
                              className={`flex items-center justify-between gap-2 rounded-md border p-2 transition-shadow hover:shadow-sm cursor-pointer ${
                                selectedUrl === m.url
                                  ? "border-primary bg-primary/5"
                                  : "border-default-200 hover:bg-content2"
                              }`}
                              onClick={() => setSelectedUrl(m.url)}
                            >
                              <Chip
                                color="default"
                                variant="flat"
                                size="sm"
                                className="shrink-0"
                              >
                                {String.fromCharCode(65 + i)}
                              </Chip>
                              <span className="text-[10px] text-default-400 truncate flex-1 min-w-0 px-1">
                                {m.label}
                              </span>
                              <div className="flex items-center gap-2 shrink-0">
                                <Chip
                                  color={
                                    m.ok
                                      ? "success"
                                      : testing
                                      ? "warning"
                                      : "danger"
                                  }
                                  variant="flat"
                                  size="sm"
                                >
                                  {typeof m.latencyMs === "number"
                                    ? `${Math.round(m.latencyMs)}ms`
                                    : testing
                                    ? t("downloadpage.mirror.testing_short", {
                                        defaultValue: "测速中",
                                      })
                                    : t("downloadpage.mirror.failed", {
                                        defaultValue: "失败",
                                      })}
                                </Chip>
                              </div>
                            </div>
                          ))}
                        </div>
                      </AccordionItem>
                    </Accordion>
                  </div>
                ) : (
                  <p className="text-default-500">
                    {t("downloadpage.mirror.no_mirrors", {
                      defaultValue: "当前版本无可用镜像。",
                    })}
                  </p>
                )}
              </ModalBody>
              <ModalFooter>
                <Button color="danger" variant="light" onPress={onClose}>
                  {t("common.cancel")}
                </Button>
                <Button
                  variant="light"
                  onPress={() => {
                    startMirrorTests(mirrorUrls || []);
                  }}
                >
                  {t("downloadpage.mirror.retest", {
                    defaultValue: "重新测速",
                  })}
                </Button>
                <Button
                  color="primary"
                  radius="full"
                  isDisabled={!selectedUrl}
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
                    } else {
                      if (hasBackend && minecraft?.StartMsixvcDownload) {
                        setDlError("");
                        setDlProgress(null);
                        progressDisclosure.onOpen();
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
                        minecraft.StartMsixvcDownload(urlWithFilename);
                      } else {
                        window.open(selectedUrl, "_blank");
                      }
                    }
                    onClose?.();
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
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>

      {/* Delete confirm modal */}
      <Modal
        isOpen={deleteDisclosure.isOpen}
        onOpenChange={deleteDisclosure.onOpenChange}
        size="md"
      >
        <ModalContent>
          {(onClose) => (
            <>
              <ModalHeader className="flex items-center gap-2">
                <div className="w-6 h-6 rounded-full bg-red-500 flex items-center justify-center">
                  <svg
                    viewBox="0 0 24 24"
                    width="14"
                    height="14"
                    className="text-white"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="3"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <path d="M3 6h18" />
                    <path d="M8 6v-2a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
                    <path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" />
                  </svg>
                </div>
                <h2 className="text-lg font-semibold">
                  {t("downloadpage.delete.title", {
                    defaultValue: "确认删除下载包",
                  })}
                </h2>
              </ModalHeader>
              <ModalBody>
                <div className="text-small text-default-600">
                  {t("downloadpage.delete.body", {
                    defaultValue: "将删除安装包：",
                  })}
                  <span className="font-mono text-default-700">
                    {" "}
                    {deleteItem?.fileName?.toLowerCase()?.endsWith(".msixvc")
                      ? deleteItem?.fileName
                      : `${deleteItem?.fileName || ""}.msixvc`}
                  </span>
                </div>
                <div className="text-small text-default-500">
                  {t("downloadpage.delete.warning", {
                    defaultValue: "此操作不可恢复。",
                  })}
                </div>
                {deleteError ? (
                  <div className="text-small text-danger-500">
                    {trErr(deleteError)}
                  </div>
                ) : null}
              </ModalBody>
              <ModalFooter>
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
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>

      {/* Delete success modal */}
      <Modal
        isOpen={deleteSuccessDisclosure.isOpen}
        onOpenChange={deleteSuccessDisclosure.onOpenChange}
        size="md"
      >
        <ModalContent>
          {(onClose) => (
            <>
              <ModalHeader className="flex items-center gap-2">
                <div className="w-6 h-6 rounded-full bg-green-500 flex items-center justify-center">
                  <svg
                    viewBox="0 0 24 24"
                    width="14"
                    height="14"
                    className="text-white"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="3"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <path d="M20 6L9 17l-5-5" />
                  </svg>
                </div>
                <h2 className="text-lg font-semibold">
                  {t("downloadpage.delete.success_title", {
                    defaultValue: "删除完成",
                  })}
                </h2>
              </ModalHeader>
              <ModalBody>
                <div className="text-small text-default-600">
                  {t("downloadpage.delete.success_body", {
                    defaultValue: "已删除安装包：",
                  })}
                  <span className="font-mono text-default-700">
                    {" "}
                    {deleteSuccessMsg}
                  </span>
                </div>
              </ModalBody>
              <ModalFooter>
                <Button
                  color="primary"
                  onPress={() => {
                    onClose?.();
                  }}
                >
                  {t("common.ok", { defaultValue: "确定" })}
                </Button>
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>

      {/* Download success modal */}
      <Modal
        isOpen={downloadSuccessDisclosure.isOpen}
        onOpenChange={downloadSuccessDisclosure.onOpenChange}
        size="md"
      >
        <ModalContent>
          {(onClose) => (
            <>
              <ModalHeader className="flex items-center gap-2">
                <div className="w-6 h-6 rounded-full bg-green-500 flex items-center justify-center">
                  <svg
                    viewBox="0 0 24 24"
                    width="14"
                    height="14"
                    className="text-white"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="3"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <path d="M20 6L9 17l-5-5" />
                  </svg>
                </div>
                <h2 className="text-lg font-semibold">
                  {t("downloadpage.download.success_title", {
                    defaultValue: "下载完成",
                  })}
                </h2>
              </ModalHeader>
              <ModalBody>
                <div className="text-small text-default-600">
                  {t("downloadpage.download.success_body", {
                    defaultValue: "文件已下载：",
                  })}
                  <span className="font-mono text-default-700">
                    {" "}
                    {downloadSuccessMsg}
                  </span>
                </div>
              </ModalBody>
              <ModalFooter>
                <Button
                  color="primary"
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
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>

      <Modal
        isOpen={installSuccessDisclosure.isOpen}
        onOpenChange={installSuccessDisclosure.onOpenChange}
        size="md"
      >
        <ModalContent>
          {(onClose) => (
            <>
              <ModalHeader className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-full bg-green-500 flex items-center justify-center">
                  <svg
                    viewBox="0 0 24 24"
                    width="20"
                    height="20"
                    className="text-white"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="3"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <path d="M20 6L9 17l-5-5" />
                  </svg>
                </div>
                <div className="flex flex-col">
                  <h2 className="text-lg font-bold">
                    {t("downloadpage.install.success_title", {
                      defaultValue: "安装完成",
                    })}
                  </h2>
                  {installingVersion ? (
                    <div className="text-small text-default-500">
                      {t("downloadpage.install.version_label", {
                        defaultValue: "版本",
                      })}
                      : {installingVersion}
                    </div>
                  ) : null}
                </div>
              </ModalHeader>
              <ModalBody>
                <div className="flex flex-col gap-4">
                  <div className="text-small text-default-600">
                    {installSuccessMsg ||
                      (t("downloadpage.install.success", {
                        defaultValue: "版本已成功安装。",
                      }) as unknown as string)}
                  </div>
                  {installingTargetName ? (
                    <div className="rounded-medium bg-content2 border border-default-200 px-3 py-2 text-small text-default-600">
                      {t("downloadpage.install.target", {
                        defaultValue: "安装目标",
                      })}
                      :{" "}
                      <span className="font-mono text-default-700">
                        {installingTargetName}
                      </span>
                    </div>
                  ) : null}
                </div>
              </ModalBody>
              <ModalFooter>
                <Button
                  variant="light"
                  onPress={() => {
                    onClose?.();
                  }}
                >
                  {t("common.back", { defaultValue: "返回" })}
                </Button>
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>

      {/* Install error modal */}
      <Modal
        isOpen={installErrorDisclosure.isOpen}
        onOpenChange={installErrorDisclosure.onOpenChange}
        size="md"
      >
        <ModalContent>
          {(onClose) => (
            <>
              <ModalHeader className="flex items-center gap-2">
                <div className="w-6 h-6 rounded-full bg-red-500 flex items-center justify-center">
                  <svg
                    viewBox="0 0 24 24"
                    width="14"
                    height="14"
                    className="text-white"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="3"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <path d="M12 9v4" />
                    <path d="M12 17h.01" />
                  </svg>
                </div>
                <h2 className="text-lg font-semibold">
                  {t("downloadpage.progress.unknown_error", {
                    defaultValue: "未知错误",
                  })}
                </h2>
              </ModalHeader>
              <ModalBody>
                <div className="text-small text-default-600">
                  {trErr(installError)}
                </div>
              </ModalBody>
              <ModalFooter>
                <Button
                  color="primary"
                  onPress={() => {
                    onClose?.();
                  }}
                >
                  {t("common.ok", { defaultValue: "确定" })}
                </Button>
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>

      {/* Install progress modal (beautified) */}
      <Modal
        isOpen={installLoadingDisclosure.isOpen}
        onOpenChange={installLoadingDisclosure.onOpenChange}
        size="md"
        hideCloseButton
        isDismissable={false}
      >
        <ModalContent>
          {() => (
            <>
              <ModalHeader className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-full bg-content2 border border-default-200 flex items-center justify-center">
                  <Spinner size="sm" />
                </div>
                <div className="flex flex-col">
                  <h2 className="text-lg font-bold">
                    {t("downloadmodal.installing.title", {
                      defaultValue: "正在安装",
                    })}
                  </h2>
                  {installingVersion ? (
                    <div className="text-small text-default-500">
                      {t("downloadpage.install.version_label", {
                        defaultValue: "版本",
                      })}
                      : {installingVersion}
                    </div>
                  ) : null}
                </div>
              </ModalHeader>
              <ModalBody>
                <div className="flex flex-col gap-4">
                  <div className="text-small text-default-500">
                    {t("downloadpage.install.hint", {
                      defaultValue: "请稍候，正在卸载旧版本并注册安装包...",
                    })}
                  </div>
                  <div className="flex items-center gap-3">
                    <Progress
                      aria-label="install-progress"
                      isIndeterminate
                      value={undefined}
                      className="flex-1"
                    />
                  </div>
                  {typeof extractInfo?.bytes === "number" &&
                  extractInfo.bytes > 0 ? (
                    <div className="text-small text-default-500">
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
                    <div className="rounded-medium bg-content2 border border-default-200 px-3 py-2 text-small text-default-600">
                      {t("downloadpage.install.target", {
                        defaultValue: "安装目标",
                      })}
                      :{" "}
                      <span className="font-mono text-default-700">
                        {installingTargetName}
                      </span>
                    </div>
                  ) : null}
                  {extractError ? (
                    <div className="text-small text-danger-500">
                      {extractError}
                    </div>
                  ) : null}
                </div>
              </ModalBody>
            </>
          )}
        </ModalContent>
      </Modal>

      <Modal
        isOpen={progressDisclosure.isOpen}
        onOpenChange={progressDisclosure.onOpenChange}
        size="md"
        hideCloseButton={true}
        isDismissable={false}
      >
        <ModalContent>
          {(onClose) => (
            <>
              <ModalHeader className="flex flex-col gap-1">
                <h2 className="text-lg font-bold">
                  {t("downloadpage.progress.title", {
                    defaultValue: "下载进度",
                  })}
                </h2>
                <p className="text-small text-default-500 truncate">
                  {dlProgress?.dest ||
                    t("downloadpage.progress.dest_placeholder", {
                      defaultValue: "安装包目录下(msixvc)",
                    })}
                </p>
              </ModalHeader>
              <ModalBody>
                {dlError ? (
                  <div className="text-danger">
                    {t("downloadpage.progress.error_prefix", {
                      defaultValue: "错误：",
                    })}
                    {dlError}
                  </div>
                ) : (
                  <div className="flex flex-col gap-3">
                    <div className="h-2 w-full rounded bg-default-200 overflow-hidden">
                      {(() => {
                        const total = dlProgress?.total || 0;
                        const done = dlProgress?.downloaded || 0;
                        const pct =
                          total > 0
                            ? Math.min(100, Math.round((done / total) * 100))
                            : 0;
                        return (
                          <div
                            className="h-full bg-primary"
                            style={{ width: `${pct}%` }}
                          />
                        );
                      })()}
                    </div>
                    <div className="text-small text-default-500">
                      {(() => {
                        const total = dlProgress?.total || 0;
                        const done = dlProgress?.downloaded || 0;
                        const pct =
                          total > 0
                            ? Math.min(100, Math.round((done / total) * 100))
                            : 0;
                        const fmt = (n: number) =>
                          `${(n / (1024 * 1024)).toFixed(2)} MB`;
                        const fmtSpd = (bps: number) =>
                          `${(bps / (1024 * 1024)).toFixed(2)} MB/s`;
                        return `${fmt(done)} / ${fmt(
                          total
                        )} (${pct}%) · ${fmtSpd(dlSpeed || 0)}`;
                      })()}
                    </div>
                  </div>
                )}
              </ModalBody>
              <ModalFooter>
                <Button
                  color="danger"
                  variant="light"
                  isDisabled={
                    !hasBackend ||
                    dlStatus === "done" ||
                    dlStatus === "cancelled"
                  }
                  onPress={() => {
                    if (!hasBackend) return;
                    if (minecraft.CancelMsixvcDownload) {
                      minecraft.CancelMsixvcDownload();
                    }
                    onClose?.();
                  }}
                >
                  {t("common.cancel")}
                </Button>
                <Button
                  color="primary"
                  isDisabled={dlStatus !== "done"}
                  onPress={() => onClose?.()}
                >
                  {t("common.ok")}
                </Button>
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>
    </motion.div>
  );
};

export default DownloadPage;
