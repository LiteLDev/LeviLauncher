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
  addToast,
} from "@heroui/react";
import {
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
  FaTrash,
  FaPlus,
  FaTag,
} from "react-icons/fa";
import { useNavigate, useLocation } from "react-router-dom";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import {
  GetContentRoots,
  OpenPathDir,
} from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { readCurrentVersionName } from "@/utils/currentVersion";
import { motion } from "framer-motion";
import {
  getPlayerGamertagMap,
  listPlayers,
  resolvePlayerDisplayName,
} from "@/utils/content";
import { McText } from "@/utils/mcformat";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { cn } from "@/utils/cn";
import { COMPONENT_STYLES } from "@/constants/componentStyles";

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
    <div
      className={cn(
        COMPONENT_STYLES.listItem,
        "w-full p-5 flex gap-5 group cursor-pointer relative overflow-hidden",
      )}
    >
      <div className="relative shrink-0">
        <div className="w-24 h-24 sm:w-28 sm:h-28 rounded-2xl bg-default-100/50 flex items-center justify-center overflow-hidden shadow-sm group-hover:shadow-md transition-shadow">
          <FaServer className="text-4xl text-blue-500/80" />
        </div>
      </div>
      <div className="flex flex-col flex-1 min-w-0">
        <div className="flex justify-between items-start gap-2 mb-1">
          <div className="flex items-center gap-2 truncate">
            <h3
              className="text-lg font-bold text-default-900 dark:text-white truncate"
              title={server.name}
            >
              {server.name}
            </h3>
            {info?.status === "online" && (
              <Chip
                size="sm"
                variant="flat"
                color="success"
                className="bg-success-50 dark:bg-success-900/20"
              >
                {t("server.online")}
              </Chip>
            )}
            {info?.status !== "online" && !loading && (
              <Chip
                size="sm"
                variant="flat"
                color="danger"
                className="bg-danger-50 dark:bg-danger-900/20"
              >
                {t("server.offline")}
              </Chip>
            )}
          </div>
        </div>

        {info?.motd && (
          <div className="text-sm text-default-500 dark:text-zinc-400 line-clamp-1 w-full font-mono mb-1">
            <McText text={info.motd} />
          </div>
        )}

        <p className="text-sm text-default-400 dark:text-zinc-500 mb-3">
          {server.ip}:{server.port}
        </p>

        <div className="flex flex-wrap items-center gap-4 text-xs text-default-400 dark:text-zinc-500 mt-auto">
          {info?.status === "online" && (
            <>
              <div
                className="flex items-center gap-1.5 bg-default-100/50 dark:bg-zinc-800/50 px-2 py-1 rounded-lg"
                title={t("server.version")}
              >
                <FaTag className="text-default-400" />
                <span>
                  <McText text={info.version} />
                </span>
              </div>
              <div
                className="flex items-center gap-1.5 bg-default-100/50 dark:bg-zinc-800/50 px-2 py-1 rounded-lg"
                title={t("server.players")}
              >
                <FaGamepad className="text-default-400" />
                <span>
                  {info.online}/{info.max}
                </span>
              </div>
              <div
                className="flex items-center gap-1.5 bg-default-100/50 dark:bg-zinc-800/50 px-2 py-1 rounded-lg"
                title={t("server.delay")}
              >
                <FaSignal className={`text-${delayColor}-500`} />
                <span
                  className={`text-${delayColor}-600 dark:text-${delayColor}-400 font-medium`}
                >
                  {info.delay}ms
                </span>
              </div>
            </>
          )}
          <div
            className="flex items-center gap-1.5 bg-default-100/50 dark:bg-zinc-800/50 px-2 py-1 rounded-lg"
            title={t("common.date")}
          >
            <FaClock className="text-default-400" />
            <span>{new Date(server.timestamp * 1000).toLocaleString()}</span>
          </div>
        </div>
      </div>
    </div>
  );
});

export default function ServersPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const [selectedPlayer, setSelectedPlayer] = useState<string>(
    location.state?.player || "",
  );
  const [players, setPlayers] = useState<string[]>([]);
  const [playerGamertagMap, setPlayerGamertagMap] = useState<
    Record<string, string>
  >({});
  const [servers, setServers] = useState<Server[]>([]);
  const [loading, setLoading] = useState(false);
  const [roots, setRoots] = useState<any>({});
  const [query, setQuery] = useState("");
  const [sortKey, setSortKey] = useState<"name" | "time">("name");
  const [sortAsc, setSortAsc] = useState<boolean>(true);
  const currentVersionName =
    location.state?.versionName || readCurrentVersionName();

  const refreshAll = useCallback(async () => {
    setLoading(true);
    try {
      const r = await GetContentRoots(currentVersionName || "");
      setRoots(r);
      const srvList = await (minecraft as any)?.ListServers?.(
        currentVersionName || "",
        selectedPlayer,
      );
      setServers(srvList || []);
    } catch (err) {
      console.error(err);
      addToast({ description: String(err), color: "danger" });
    } finally {
      setLoading(false);
    }
  }, [currentVersionName, selectedPlayer]);

  const handleOpenFolder = async () => {
    if (!selectedPlayer) return;
    const r = await GetContentRoots(currentVersionName || "");
    if (r.usersRoot) {
      const path = `${r.usersRoot}\\${selectedPlayer}\\games\\com.mojang\\minecraftpe`;
      OpenPathDir(path);
    }
  };

  useEffect(() => {
    const fetchPlayers = async () => {
      try {
        const r = await GetContentRoots(currentVersionName || "");
        setRoots(r);
        if (r.usersRoot) {
          const pList = await listPlayers(r.usersRoot);
          setPlayers(pList);
          const map = await getPlayerGamertagMap(r.usersRoot);
          setPlayerGamertagMap(map);
        } else {
          setPlayers([]);
          setPlayerGamertagMap({});
        }
      } catch (e) {
        console.error("Failed to list players", e);
        setPlayers([]);
        setPlayerGamertagMap({});
      }
    };
    fetchPlayers();
    refreshAll();
  }, [currentVersionName, refreshAll]);

  const filteredServers = useMemo(() => {
    let list = [...servers];
    if (query.trim()) {
      const q = query.toLowerCase();
      list = list.filter((srv) => srv.name.toLowerCase().includes(q));
    }
    list.sort((a, b) => {
      if (sortKey === "name") {
        const nameA = a.name.toLowerCase();
        const nameB = b.name.toLowerCase();
        const res = nameA.localeCompare(nameB);
        return sortAsc ? res : -res;
      }
      const ta = Number(a.timestamp || 0);
      const tb = Number(b.timestamp || 0);
      const res = ta - tb;
      return sortAsc ? res : -res;
    });
    return list;
  }, [servers, query, sortKey, sortAsc]);

  return (
    <PageContainer>
      <Card className={LAYOUT.GLASS_CARD.BASE}>
        <CardBody className="p-6 flex flex-col gap-6">
          <PageHeader
            title={t("contentpage.servers")}
            endContent={
              <div className="flex items-center gap-2">
                <Dropdown classNames={COMPONENT_STYLES.dropdown}>
                  <DropdownTrigger>
                    <Button
                      className={cn(
                        COMPONENT_STYLES.dropdownTriggerButton,
                        "w-full sm:w-auto sm:min-w-[200px] text-default-600 dark:text-zinc-200 font-medium",
                      )}
                      isDisabled={!players.length}
                      startContent={<FaUser />}
                    >
                      {selectedPlayer
                        ? resolvePlayerDisplayName(
                            selectedPlayer,
                            playerGamertagMap,
                          )
                        : t("contentpage.select_player")}
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
                        <DropdownItem
                          key={p}
                          textValue={resolvePlayerDisplayName(
                            p,
                            playerGamertagMap,
                          )}
                        >
                          {resolvePlayerDisplayName(p, playerGamertagMap)}
                        </DropdownItem>
                      ))
                    ) : (
                      <DropdownItem key="none" isDisabled>
                        {t("contentpage.no_players")}
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
                  {t("common.open")}
                </Button>

                <Tooltip content={t("common.refresh") as string}>
                  <Button
                    isIconOnly
                    radius="full"
                    variant="light"
                    onPress={refreshAll}
                    isDisabled={loading}
                  >
                    <FaSync
                      className={loading ? "animate-spin" : ""}
                      size={18}
                    />
                  </Button>
                </Tooltip>
              </div>
            }
          />

          <div className="flex flex-col md:flex-row gap-4 items-end md:items-center justify-between">
            <Input
              placeholder={t("common.search_placeholder")}
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
              classNames={COMPONENT_STYLES.input}
            />

            <div className="flex items-center gap-3">
              <Dropdown classNames={COMPONENT_STYLES.dropdown}>
                <DropdownTrigger>
                  <Button
                    className={cn(
                      COMPONENT_STYLES.dropdownTriggerButton,
                      "min-w-[120px]",
                    )}
                    startContent={
                      sortAsc ? <FaSortAmountDown /> : <FaSortAmountUp />
                    }
                  >
                    {sortKey === "name"
                      ? t("filemanager.sort.name")
                      : t("contentpage.sort_time")}
                    {" / "}
                    {sortAsc
                      ? t("contentpage.sort_asc")
                      : t("contentpage.sort_desc")}
                  </Button>
                </DropdownTrigger>
                <DropdownMenu
                  selectionMode="single"
                  selectedKeys={
                    new Set([`${sortKey}-${sortAsc ? "asc" : "desc"}`])
                  }
                  onSelectionChange={(keys) => {
                    const val = Array.from(keys as unknown as Set<string>);
                    const item = val[0] || "name-asc";
                    const [k, order] = item.split("-");
                    setSortKey(k as "name" | "time");
                    setSortAsc(order === "asc");
                  }}
                >
                  <DropdownItem
                    key="name-asc"
                    startContent={<FaSortAmountDown />}
                  >
                    {t("filemanager.sort.name")} (A-Z)
                  </DropdownItem>
                  <DropdownItem
                    key="name-desc"
                    startContent={<FaSortAmountUp />}
                  >
                    {t("filemanager.sort.name")} (Z-A)
                  </DropdownItem>
                  <DropdownItem
                    key="time-asc"
                    startContent={<FaSortAmountDown />}
                  >
                    {t("contentpage.sort_time")} (Old-New)
                  </DropdownItem>
                  <DropdownItem
                    key="time-desc"
                    startContent={<FaSortAmountUp />}
                  >
                    {t("contentpage.sort_time")} (New-Old)
                  </DropdownItem>
                </DropdownMenu>
              </Dropdown>
            </div>
          </div>

          <div className="mt-2 text-default-500 dark:text-zinc-400 text-sm flex flex-wrap items-center gap-2">
            <span>{t("contentpage.current_version")}:</span>
            <span className="font-medium text-default-700 dark:text-zinc-200 bg-default-100 dark:bg-zinc-800 px-2 py-0.5 rounded-md">
              {currentVersionName || t("contentpage.none")}
            </span>
            <span className="text-default-300">|</span>
            <span>{t("contentpage.isolation")}:</span>
            <span
              className={`font-medium px-2 py-0.5 rounded-md ${
                roots.isIsolation
                  ? "bg-success-50 text-success-600 dark:bg-success-900/20 dark:text-success-400"
                  : "bg-default-100 dark:bg-zinc-800 text-default-700 dark:text-zinc-200"
              }`}
            >
              {roots.isIsolation ? t("common.yes") : t("common.no")}
            </span>
          </div>
        </CardBody>
      </Card>

      {loading ? (
        <div className="flex flex-col items-center justify-center py-20 gap-4">
          <Spinner size="lg" />
          <span className="text-default-500 dark:text-zinc-400">
            {t("common.loading")}
          </span>
        </div>
      ) : filteredServers.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-default-400">
          <FaBox className="text-6xl mb-4 opacity-20" />
          <p>{query ? t("common.no_results") : t("contentpage.no_items")}</p>
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
    </PageContainer>
  );
}
