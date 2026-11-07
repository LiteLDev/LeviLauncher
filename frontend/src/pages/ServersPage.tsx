import { resolveContentPath } from "@/utils/content";
import { openDirectory, showDirectoryOpenError } from "@/utils/explorer";
import { Button, Chip, Spinner } from "@heroui/react";

import React, { useState, useEffect, useCallback, useMemo } from "react";
import { useTranslation } from "react-i18next";
import { ModalNotice } from "@/components/ModalPrimitives";

import {
  FaServer,
  FaSync,
  FaUser,
  FaFolderOpen,
  FaBox,
  FaSignal,
  FaGamepad,
  FaClock,
  FaTag,
} from "react-icons/fa";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import { useCurrentVersion } from "@/utils/CurrentVersionContext";
import { motion } from "framer-motion";
import {
  ContentPrerequisite,
  ContentSearchField,
  ContentSortMenu,
  ContentSubpageHeader,
  useContentPlayers,
  useContentRoots,
} from "@/components/ContentSubpage";
import { InstancePicker } from "@/components/InstancePicker";
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
        COMPONENT_STYLES.contentListItem,
        "w-full h-full p-4 flex gap-4 group cursor-default relative overflow-hidden",
      )}
    >
      <div className="relative shrink-0">
        <div className="size-16 sm:size-20 rounded-2xl bg-surface-secondary/50 flex items-center justify-center overflow-hidden shadow-sm group-hover:shadow-md transition-shadow">
          <FaServer className="text-4xl text-blue-500/80" />
        </div>
      </div>
      <div className="flex flex-col flex-1 min-w-0">
        <div className="flex justify-between items-start gap-2 mb-1">
          <div className="flex min-w-0 items-center gap-2">
            <h3
              className="text-lg font-bold text-foreground dark:text-white truncate"
              title={server.name}
            >
              {server.name}
            </h3>
            {info?.status === "online" && (
              <Chip
                size="sm"
                variant="soft"
                color={"success"}
                className={"bg-green-50 dark:bg-green-900/20"}
              >
                <Chip.Label>{t("server.online")}</Chip.Label>
              </Chip>
            )}
            {info?.status !== "online" && !loading && (
              <Chip
                size="sm"
                variant="soft"
                color={"danger"}
                className={"bg-rose-50 dark:bg-rose-900/20"}
              >
                <Chip.Label>{t("server.offline")}</Chip.Label>
              </Chip>
            )}
          </div>
        </div>

        {info?.motd && (
          <div className="text-sm text-muted dark:text-zinc-400 line-clamp-1 w-full font-mono mb-1">
            <McText text={info.motd} />
          </div>
        )}

        <p className="text-sm text-muted dark:text-zinc-500 mb-3">
          {server.ip}:{server.port}
        </p>

        <div className="flex flex-wrap items-center gap-4 text-xs text-muted dark:text-zinc-500 mt-auto">
          {info?.status === "online" && (
            <>
              <div
                className="flex items-center gap-1.5 bg-surface-secondary/50 px-2 py-1 rounded-lg"
                title={t("server.version")}
              >
                <FaTag className="text-muted" />
                <span>
                  <McText text={info.version} />
                </span>
              </div>
              <div
                className="flex items-center gap-1.5 bg-surface-secondary/50 px-2 py-1 rounded-lg"
                title={t("server.players")}
              >
                <FaGamepad className="text-muted" />
                <span>
                  {info.online}/{info.max}
                </span>
              </div>
              <div
                className="flex items-center gap-1.5 bg-surface-secondary/50 px-2 py-1 rounded-lg"
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
            className="flex items-center gap-1.5 bg-surface-secondary/50 px-2 py-1 rounded-lg"
            title={t("common.date")}
          >
            <FaClock className="text-muted" />
            <span>{new Date(server.timestamp * 1000).toLocaleString()}</span>
          </div>
        </div>
      </div>
    </div>
  );
});

export default function ServersPage() {
  const { t } = useTranslation();
  const { currentVersionName } = useCurrentVersion();
  const [servers, setServers] = useState<Server[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState("");
  const loadGeneration = React.useRef(0);
  const rootsState = useContentRoots(currentVersionName);
  const { roots } = rootsState;
  const [query, setQuery] = useState("");
  const [sortKey, setSortKey] = useState<"name" | "time">("name");
  const [sortAsc, setSortAsc] = useState<boolean>(true);
  const isUWP = roots.packageType === "uwp";
  const playerState = useContentPlayers(
    rootsState.ready && !isUWP ? roots.usersRoot || "" : "",
    true,
  );
  const selectedPlayer = isUWP ? "" : playerState.selectedPlayer;
  const ready =
    rootsState.ready && !playerState.loading;

  useEffect(() => {
    setServers([]);
    setLoadError("");
    setLoading(false);
  }, [currentVersionName]);

  const refreshAll = useCallback(async () => {
    if (!ready) return;
    const generation = ++loadGeneration.current;
    setLoading(true);
    setLoadError("");
    try {
      if (!selectedPlayer && !isUWP) {
        setServers([]);
        return;
      }
      const srvList = await (minecraft as any)?.ListServers?.(
        currentVersionName,
        selectedPlayer,
      );
      if (generation === loadGeneration.current) setServers(srvList || []);
    } catch (err) {
      if (generation !== loadGeneration.current) return;
      console.error(err);
      setLoadError(String(err));
    } finally {
      if (generation === loadGeneration.current) setLoading(false);
    }
  }, [currentVersionName, isUWP, ready, selectedPlayer]);

  const handleOpenFolder = async () => {
    try {
      const path = resolveContentPath(roots, "minecraftpe", selectedPlayer);
      if (path) {
        await openDirectory(path);
      }
    } catch (error) {
      showDirectoryOpenError(error);
    }
  };

  useEffect(() => {
    if (!ready) return;
    setServers([]);
    void refreshAll();
    return () => { loadGeneration.current++; };
  }, [ready, refreshAll]);

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

  const error = rootsState.error || loadError || playerState.error;
  const busy = loading || (!!currentVersionName && !ready && !error);

  return (
    <PageContainer>
      <ContentSubpageHeader
        title={t("contentpage.servers")}
        description={t("contentpage.servers_read_only")}
        isIsolation={currentVersionName ? !!roots.isIsolation : undefined}
        player={
          currentVersionName && !isUWP
            ? {
                players: playerState.players,
                value: selectedPlayer,
                gamertagMap: playerState.gamertagMap,
                onChange: playerState.setSelectedPlayer,
              }
            : null
        }
        actions={[
          {
            key: "open",
            label: t("common.open_folder"),
            icon: <FaFolderOpen />,
            onPress: () => void handleOpenFolder(),
            isDisabled: !ready || (!selectedPlayer && !isUWP),
          },
          {
            key: "refresh",
            label: t("common.refresh"),
            icon: <FaSync className={loading ? "animate-spin" : ""} />,
            onPress: () => void refreshAll(),
            isDisabled: loading || !ready,
          },
        ]}
      >
        <ContentSearchField value={query} onChange={setQuery} />
        <ContentSortMenu
          sortKey={sortKey}
          sortAsc={sortAsc}
          onChange={(key, asc) => {
            setSortKey(key);
            setSortAsc(asc);
          }}
        />
      </ContentSubpageHeader>

      {error && (
        <ModalNotice role="alert" tone="danger">
          <p>{t("audit.usability.content_load_failed")}</p>
          <p className="mt-1 text-xs select-text">{error}</p>
          <Button className="mt-3" variant="secondary" size="sm" isDisabled={loading || playerState.loading} onPress={() => {
            if (playerState.error) playerState.reload();
            else if (rootsState.error) rootsState.retry();
            else void refreshAll();
          }}>{t("common.retry")}</Button>
        </ModalNotice>
      )}
      {!currentVersionName ? (
        <ContentPrerequisite
          icon={<FaServer />}
          title={t("content_sub.no_instance_title")}
          body={t("content_sub.no_instance_body")}
        >
          <InstancePicker />
        </ContentPrerequisite>
      ) : busy ? (
        <div className="flex flex-col items-center justify-center py-20 gap-4">
          <Spinner size="lg" />
          <span className="text-muted dark:text-zinc-400">
            {t("common.loading")}
          </span>
        </div>
      ) : error ? null : !selectedPlayer && !isUWP ? (
        <ContentPrerequisite
          icon={<FaUser />}
          title={t("contentpage.no_players")}
          body={t("audit.usability.choose_player")}
        />
      ) : filteredServers.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-muted">
          <FaBox className="text-6xl mb-4 opacity-20" />
          <p>{query ? t("common.no_results") : t("contentpage.no_items")}</p>
        </div>
      ) : (
        <div className={cn(LAYOUT.GRID.AUTO(30), "pb-8")}>
          {filteredServers.map((srv, idx) => (
            <motion.div
              key={srv.index + "-" + idx}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: Math.min(idx, 10) * 0.05 }}
              className="min-w-0"
            >
              <ServerRow server={srv} />
            </motion.div>
          ))}
        </div>
      )}
    </PageContainer>
  );
}
