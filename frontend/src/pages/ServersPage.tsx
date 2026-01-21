import React, { useState, useEffect, useCallback, useMemo } from "react";
import { useTranslation } from "react-i18next";
import { PageHeader } from "@/components/PageHeader";
import {
  Button,
  Card,
  CardBody,
  Dropdown,
  DropdownTrigger,
  DropdownMenu,
  DropdownItem,
  Input,
  Tooltip,
  Spinner,
  Chip,
} from "@heroui/react";
import {
  FaArrowLeft,
  FaServer,
  FaSync,
  FaUser,
  FaFolderOpen,
  FaFilter,
  FaTimes,
  FaBox,
  FaSortAmountDown,
  FaSortAmountUp,
  FaSignal,
  FaGamepad,
  FaClock,
} from "react-icons/fa";
import { useNavigate, useLocation } from "react-router-dom";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { GetContentRoots, OpenPathDir } from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { readCurrentVersionName } from "@/utils/currentVersion";
import { motion } from "framer-motion";
import {
  getPlayerGamertagMap,
  listPlayers,
  resolvePlayerDisplayName,
} from "@/utils/content";
import { toast } from "react-hot-toast";

interface Server {
  index: string;
  name: string;
  ip: string;
  port: string;
  timestamp: number;
}

interface MotdInfo {
  status: string;
  host: string;
  motd: string;
  agreement: number;
  version: string;
  online: number;
  max: number;
  level_name: string;
  gamemode: string;
  server_unique_id: string;
  delay: number;
}

const ServerRow = React.memo(({ server }: { server: Server }) => {
  const { t } = useTranslation();
  const [info, setInfo] = useState<MotdInfo | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let mounted = true;
    const fetchStatus = async () => {
      try {
        const host = `${server.ip}:${server.port}`;
        // Call the backend PingServer function
        const res = await (minecraft as any)?.PingServer?.(host);
        if (mounted) {
          setInfo(res);
          setLoading(false);
        }
      } catch (err) {
        if (mounted) setLoading(false);
      }
    };
    fetchStatus();
    return () => {
      mounted = false;
    };
  }, [server.ip, server.port]);

  const delayColor = useMemo(() => {
    if (!info || info.status !== "online") return "default";
    if (info.delay < 100) return "success";
    if (info.delay < 300) return "warning";
    return "danger";
  }, [info]);

  return (
    <Card className="border border-transparent hover:border-default-200 dark:hover:border-zinc-700 bg-white dark:bg-zinc-800 shadow-sm transition-all hover:shadow-md">
      <CardBody className="p-4">
        <div className="flex flex-col md:flex-row gap-4 items-start md:items-center">
          {/* Icon & Basic Info */}
          <div className="flex items-center gap-4 min-w-[200px] flex-1">
            <div className={`p-3 rounded-xl shrink-0 ${info?.status === "online" ? "bg-emerald-50 dark:bg-emerald-900/20 text-emerald-500" : "bg-default-100 dark:bg-zinc-800 text-default-400"}`}>
              <FaServer size={24} />
            </div>
            <div className="flex flex-col min-w-0">
              <h3 className="text-base font-bold truncate text-default-900">{server.name}</h3>
              <div className="flex items-center gap-2 text-xs text-default-500 font-mono">
                <span>{server.ip}:{server.port}</span>
              </div>
            </div>
          </div>

          {/* Status & MOTD */}
          <div className="flex-1 min-w-0 flex flex-col justify-center w-full md:w-auto">
            {loading ? (
               <div className="flex items-center gap-2 text-xs text-default-400">
                  <Spinner size="sm" color="default" />
                  <span>{t("common.loading", { defaultValue: "Ping..." })}</span>
               </div>
            ) : info?.status === "online" ? (
                <div className="flex flex-col gap-1">
                    <div className="text-sm text-default-700 truncate font-medium">
                        {info.motd || info.level_name || t("common.unknown", { defaultValue: "未知" })}
                    </div>
                    <div className="flex items-center gap-2 text-xs text-default-500">
                        <span className="bg-default-100 px-1.5 py-0.5 rounded text-default-600">{info.version}</span>
                        <span>{info.gamemode}</span>
                    </div>
                </div>
            ) : (
                <span className="text-sm text-default-400 italic">
                    {t("server.offline", { defaultValue: "离线 / 无法连接" })}
                </span>
            )}
          </div>

          {/* Stats (Players & Delay) */}
          <div className="flex items-center gap-4 md:justify-end w-full md:w-auto shrink-0 mt-2 md:mt-0 border-t md:border-t-0 border-default-100 pt-2 md:pt-0">
             {loading ? (
                 <div className="h-8 w-20 bg-default-100 rounded-lg animate-pulse" />
             ) : info?.status === "online" ? (
                 <>
                    <Tooltip content={t("server.players", { defaultValue: "在线玩家" })}>
                        <Chip size="sm" variant="flat" startContent={<FaUser size={10} />} className="gap-1 bg-indigo-50 dark:bg-indigo-900/20 text-indigo-500">
                            {info.online}/{info.max}
                        </Chip>
                    </Tooltip>
                    
                    <Tooltip content={t("server.delay", { defaultValue: "延迟" })}>
                        <Chip size="sm" variant="flat" color={delayColor} startContent={<FaSignal size={10} />} className="gap-1">
                            {info.delay}ms
                        </Chip>
                    </Tooltip>
                 </>
             ) : (
                 <Chip size="sm" variant="flat" color="default" className="text-default-400">
                    {t("server.offline", { defaultValue: "Offline" })}
                 </Chip>
             )}
          </div>
        </div>
      </CardBody>
    </Card>
  );
});

export default function ServersPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation() as any;
  const [loading, setLoading] = useState<boolean>(true);
  const [servers, setServers] = useState<Server[]>([]);
  const [currentVersionName, setCurrentVersionName] = useState<string>("");
  
  // Player & Roots State
  const [roots, setRoots] = useState<any>({});
  const [players, setPlayers] = useState<string[]>([]);
  const [selectedPlayer, setSelectedPlayer] = useState<string>("");
  const [playerGamertagMap, setPlayerGamertagMap] = useState<Record<string, string>>({});

  // Search & Sort State
  const [query, setQuery] = useState<string>("");
  const [sortKey, setSortKey] = useState<"name" | "time">("time");
  const [sortAsc, setSortAsc] = useState<boolean>(true);

  // Initialization & Player Loading
  useEffect(() => {
    const init = async () => {
      const name = readCurrentVersionName();
      setCurrentVersionName(name);
      
      try {
        const r = await GetContentRoots(name);
        setRoots(r || {});

        if (r && r.usersRoot) {
            const pList = await listPlayers(r.usersRoot);
            setPlayers(pList);

            // Determine initial player
            let targetPlayer = location.state?.player || "";
            let currentSelection = "";

            if (!targetPlayer && pList.length > 0) {
                targetPlayer = pList[0];
            }
            if (targetPlayer && pList.includes(targetPlayer)) {
                currentSelection = targetPlayer;
                setSelectedPlayer(targetPlayer);
            } else if (pList.length > 0) {
                currentSelection = pList[0];
                setSelectedPlayer(pList[0]);
            }

            (async () => {
              try {
                const map = await getPlayerGamertagMap(r.usersRoot);
                setPlayerGamertagMap(map);

                const tag = await (minecraft as any)?.GetLocalUserGamertag?.();
                if (tag) {
                  for (const p of pList) {
                    if (map[p] === tag) {
                      if (p !== currentSelection) {
                        setSelectedPlayer(p);
                      }
                      break;
                    }
                  }
                }
              } catch {}
            })();
            if (pList.length === 0) {
                setLoading(false);
            }
        } else {
            setPlayers([]);
            setPlayerGamertagMap({});
            setLoading(false);
        }
      } catch (e) {
        console.error("Failed to init servers page", e);
        setLoading(false);
      }
    };
    init();
  }, []);

  const refreshAll = useCallback(async () => {
    if (!selectedPlayer) return;
    setLoading(true);
    try {
        const v = readCurrentVersionName();
        const srvs = await (minecraft as any)?.ListServers?.(v, selectedPlayer);
        setServers(srvs || []);
    } catch (err) {
        console.error(err);
        toast.error(t("common.error_load", { defaultValue: "加载失败" }));
    } finally {
        setLoading(false);
    }
  }, [selectedPlayer, t]);

  useEffect(() => {
    if (selectedPlayer) {
        refreshAll();
    }
  }, [selectedPlayer, refreshAll]);

  // Filtering & Sorting
  const filteredServers = useMemo(() => {
    let list = [...servers];
    if (query.trim()) {
        const q = query.toLowerCase();
        list = list.filter(s => 
            s.name.toLowerCase().includes(q) || 
            s.ip.toLowerCase().includes(q)
        );
    }

    list.sort((a, b) => {
        if (sortKey === "name") {
            const res = a.name.localeCompare(b.name);
            return sortAsc ? res : -res;
        } else {
            const res = a.timestamp - b.timestamp;
            return sortAsc ? res : -res;
        }
    });

    return list;
  }, [servers, query, sortKey, sortAsc]);

  const handleOpenFolder = async () => {
    if (roots.usersRoot && selectedPlayer) {
        const path = `${roots.usersRoot}\\${selectedPlayer}\\games\\com.mojang\\minecraftpe`;
        await OpenPathDir(path);
    }
  };

  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.25 }}
      className="w-full max-w-full mx-auto p-4 h-full flex flex-col"
    >
      <Card className="flex-1 min-h-0 border-none shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl">
        <CardBody className="p-0 flex flex-col h-full overflow-hidden">
          {/* Header Section */}
          <div className="shrink-0 p-4 sm:p-6 pb-2 flex flex-col gap-4 border-b border-default-200 dark:border-white/10">
            <PageHeader
              title={t("contentpage.servers", { defaultValue: "服务器" })}
              startContent={
                <Button
                  isIconOnly
                  radius="full"
                  variant="light"
                  onPress={() =>
                    navigate("/content", { state: { player: selectedPlayer } })
                  }
                >
                  <FaArrowLeft size={20} />
                </Button>
              }
              endContent={
                <div className="flex items-center gap-2">
                  <Dropdown>
                    <DropdownTrigger>
                      <Button
                        radius="full"
                        variant="flat"
                        className="bg-default-100 dark:bg-zinc-800 text-default-600 dark:text-zinc-200 font-medium w-full sm:w-auto sm:min-w-[200px]"
                        isDisabled={!players.length}
                        startContent={<FaUser />}
                      >
                        {selectedPlayer
                          ? resolvePlayerDisplayName(selectedPlayer, playerGamertagMap)
                          : t("contentpage.select_player", { defaultValue: "选择玩家" })}
                      </Button>
                    </DropdownTrigger>
                    <DropdownMenu
                      selectionMode="single"
                      selectedKeys={new Set([selectedPlayer])}
                      onSelectionChange={(keys) => {
                        const arr = Array.from(keys as unknown as Set<string>);
                        const next = arr[0] || "";
                        if (typeof next === "string") setSelectedPlayer(next);
                      }}
                    >
                      {players.length ? (
                        players.map((p) => (
                          <DropdownItem key={p} textValue={resolvePlayerDisplayName(p, playerGamertagMap)}>
                            {resolvePlayerDisplayName(p, playerGamertagMap)}
                          </DropdownItem>
                        ))
                      ) : (
                        <DropdownItem key="none" isDisabled>
                          {t("contentpage.no_players", { defaultValue: "暂无玩家" })}
                        </DropdownItem>
                      )}
                    </DropdownMenu>
                  </Dropdown>

                  <Button
                    radius="full"
                    variant="flat"
                    startContent={<FaFolderOpen />}
                    onPress={handleOpenFolder}
                    isDisabled={!selectedPlayer}
                    className="bg-default-100 dark:bg-zinc-800 text-default-600 dark:text-zinc-200 font-medium"
                  >
                    {t("common.open", { defaultValue: "打开" })}
                  </Button>

                  <Tooltip content={t("common.refresh", { defaultValue: "刷新" }) as string}>
                    <Button
                      isIconOnly
                      radius="full"
                      variant="light"
                      onPress={refreshAll}
                      isDisabled={loading}
                    >
                      <FaSync className={loading ? "animate-spin" : ""} size={18} />
                    </Button>
                  </Tooltip>
                </div>
              }
            />

            {/* Toolbar */}
            <div className="flex flex-col md:flex-row gap-4 items-end md:items-center justify-between">
              <Input
                placeholder={t("common.search_placeholder", { defaultValue: "搜索..." })}
                value={query}
                onValueChange={setQuery}
                startContent={<FaFilter className="text-default-400" />}
                endContent={
                  query && (
                    <button onClick={() => setQuery("")}>
                      <FaTimes className="text-default-400 hover:text-default-600" />
                    </button>
                  )
                }
                radius="full"
                variant="flat"
                className="w-full md:max-w-xs"
                classNames={{
                    inputWrapper:
                      "bg-default-100 dark:bg-default-50/50 hover:bg-default-200/70 transition-colors group-data-[focus=true]:bg-white dark:group-data-[focus=true]:bg-zinc-900 shadow-sm",
                  }}
              />

              <div className="flex items-center gap-3">
                 <Dropdown>
                  <DropdownTrigger>
                    <Button
                      variant="flat"
                      radius="full"
                      startContent={sortAsc ? <FaSortAmountDown /> : <FaSortAmountUp />}
                      className="min-w-[120px]"
                    >
                      {sortKey === "name"
                        ? t("filemanager.sort.name", { defaultValue: "名称" })
                        : t("contentpage.sort_time", { defaultValue: "时间" })}
                      {" / "}
                      {sortAsc
                        ? t("contentpage.sort_asc", { defaultValue: "从上到下" })
                        : t("contentpage.sort_desc", { defaultValue: "从下到上" })}
                    </Button>
                  </DropdownTrigger>
                  <DropdownMenu
                    selectionMode="single"
                    selectedKeys={new Set([`${sortKey}-${sortAsc ? "asc" : "desc"}`])}
                    onSelectionChange={(keys) => {
                      const val = Array.from(keys)[0] as string;
                      const [k, order] = val.split("-");
                      setSortKey(k as "name" | "time");
                      setSortAsc(order === "asc");
                    }}
                  >
                    <DropdownItem key="name-asc" startContent={<FaSortAmountDown />}>
                      {t("filemanager.sort.name", { defaultValue: "名称" })} (A-Z)
                    </DropdownItem>
                    <DropdownItem key="name-desc" startContent={<FaSortAmountUp />}>
                      {t("filemanager.sort.name", { defaultValue: "名称" })} (Z-A)
                    </DropdownItem>
                    <DropdownItem key="time-asc" startContent={<FaSortAmountDown />}>
                      {t("contentpage.sort_time", { defaultValue: "时间" })} (Old-New)
                    </DropdownItem>
                    <DropdownItem key="time-desc" startContent={<FaSortAmountUp />}>
                      {t("contentpage.sort_time", { defaultValue: "时间" })} (New-Old)
                    </DropdownItem>
                  </DropdownMenu>
                </Dropdown>
              </div>
            </div>

            {/* Version Info */}
            <div className="mt-2 text-default-500 text-sm flex flex-wrap items-center gap-2">
              <span>{t("contentpage.current_version", { defaultValue: "当前版本" })}:</span>
              <span className="font-medium text-default-700 bg-default-100 px-2 py-0.5 rounded-md">
                {currentVersionName || t("contentpage.none", { defaultValue: "无" })}
              </span>
              <span className="text-default-300">|</span>
              <span>{t("contentpage.isolation", { defaultValue: "版本隔离" })}:</span>
              <span
                className={`font-medium px-2 py-0.5 rounded-md ${
                  roots.isIsolation
                    ? "bg-success-50 text-success-600 dark:bg-success-900/20 dark:text-success-400"
                    : "bg-default-100 text-default-700"
                }`}
              >
                {roots.isIsolation
                  ? t("common.yes", { defaultValue: "是" })
                  : t("common.no", { defaultValue: "否" })}
              </span>
            </div>
          </div>

          {/* List Content */}
          <div className="flex-1 overflow-y-auto p-4 sm:p-6 custom-scrollbar">
            {loading ? (
              <div className="flex flex-col items-center justify-center py-20 gap-4">
                <Spinner size="lg" />
                <span className="text-default-500">{t("common.loading", { defaultValue: "加载中" })}</span>
              </div>
            ) : filteredServers.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-20 text-default-400">
                <FaBox className="text-6xl mb-4 opacity-20" />
                <p>
                  {query
                    ? t("common.no_results", { defaultValue: "无搜索结果" })
                    : t("contentpage.no_items", { defaultValue: "没有找到项目" })}
                </p>
              </div>
            ) : (
              <div className="flex flex-col gap-3 pb-8">
                {filteredServers.map((srv, idx) => (
                  <motion.div
                    key={srv.index + "-" + idx}
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: idx * 0.05 }}
                  >
                    <ServerRow server={srv} />
                  </motion.div>
                ))}
              </div>
            )}
          </div>
        </CardBody>
      </Card>
    </motion.div>
  );
}
