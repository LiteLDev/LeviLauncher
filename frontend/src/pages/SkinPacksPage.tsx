import React from "react";
import { useTranslation } from "react-i18next";
import {
  Button,
  Chip,
  Image,
  Spinner,
  Tooltip,
  Dropdown,
  DropdownTrigger,
  DropdownMenu,
  DropdownItem,
  Modal,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  useDisclosure,
  Input,
  Checkbox,
} from "@heroui/react";
import { motion } from "framer-motion";
import { useNavigate, useLocation } from "react-router-dom";
import {
  GetContentRoots,
  ListPacksForVersion,
  OpenPathDir,
} from "../../bindings/github.com/liteldev/LeviLauncher/minecraft";
import * as types from "../../bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import { readCurrentVersionName } from "../utils/currentVersion";
import { listPlayers } from "../utils/content";
import * as minecraft from "../../bindings/github.com/liteldev/LeviLauncher/minecraft";
import { renderMcText } from "../utils/mcformat";

export default function SkinPacksPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const hasBackend = minecraft !== undefined;
  const [loading, setLoading] = React.useState<boolean>(true);
  const [error, setError] = React.useState<string>("");
  const [currentVersionName, setCurrentVersionName] =
    React.useState<string>("");
  const [roots, setRoots] = React.useState<types.ContentRoots>({
    base: "",
    usersRoot: "",
    resourcePacks: "",
    behaviorPacks: "",
    isIsolation: false,
    isPreview: false,
  });
  const [players, setPlayers] = React.useState<string[]>([]);
  const [selectedPlayer, setSelectedPlayer] = React.useState<string>("");
  const [packs, setPacks] = React.useState<any[]>([]);
  const [resultSuccess, setResultSuccess] = React.useState<string[]>([]);
  const [resultFailed, setResultFailed] = React.useState<
    Array<{ name: string; err: string }>
  >([]);
  const [activePack, setActivePack] = React.useState<any | null>(null);
  const {
    isOpen: delOpen,
    onOpen: delOnOpen,
    onOpenChange: delOnOpenChange,
  } = useDisclosure();
  const {
    isOpen: delCfmOpen,
    onOpen: delCfmOnOpen,
    onOpenChange: delCfmOnOpenChange,
  } = useDisclosure();
  const [query, setQuery] = React.useState<string>("");
  const [sortKey, setSortKey] = React.useState<"name" | "time">(() => {
    try {
      const saved = JSON.parse(
        localStorage.getItem("content.skin.sort") || "{}"
      );
      const k = saved?.sortKey;
      if (k === "name" || k === "time") return k;
    } catch {}
    return "name";
  });
  const [sortAsc, setSortAsc] = React.useState<boolean>(() => {
    try {
      const saved = JSON.parse(
        localStorage.getItem("content.skin.sort") || "{}"
      );
      const a = saved?.sortAsc;
      if (typeof a === "boolean") return a;
    } catch {}
    return true;
  });
  const [selected, setSelected] = React.useState<Record<string, boolean>>({});
  const {
    isOpen: delManyCfmOpen,
    onOpen: delManyCfmOnOpen,
    onOpenChange: delManyCfmOnOpenChange,
  } = useDisclosure();
  const [selectMode, setSelectMode] = React.useState<boolean>(false);
  const [deletingOne, setDeletingOne] = React.useState<boolean>(false);
  const [deletingMany, setDeletingMany] = React.useState<boolean>(false);
  const scrollRef = React.useRef<HTMLDivElement | null>(null);
  const lastScrollTopRef = React.useRef<number>(0);
  const restorePendingRef = React.useRef<boolean>(false);

  const refreshAll = React.useCallback(
    async (silent?: boolean, forcePlayer?: string) => {
      if (!silent) setLoading(true);
      setError("");
      const name = readCurrentVersionName();
      setCurrentVersionName(name);
      try {
        if (!hasBackend || !name) {
          setRoots({
            base: "",
            usersRoot: "",
            resourcePacks: "",
            behaviorPacks: "",
            isIsolation: false,
            isPreview: false,
          });
          setPlayers([]);
          setSelectedPlayer("");
          setPacks([]);
        } else {
          const r = await GetContentRoots(name);
          const safe = r || {
            base: "",
            usersRoot: "",
            resourcePacks: "",
            behaviorPacks: "",
            isIsolation: false,
            isPreview: false,
          };
          setRoots(safe);

          // Player handling
          let nextPlayer = forcePlayer;
          if (nextPlayer === undefined) {
             const names = await listPlayers(safe.usersRoot);
             setPlayers(names);
             const passedPlayer = location?.state?.player || "";
             nextPlayer = (selectedPlayer && names.includes(selectedPlayer))
                ? selectedPlayer
                : (names.includes(passedPlayer) ? passedPlayer : (names[0] || ""));
             setSelectedPlayer(nextPlayer);
          }

          const allPacks = await ListPacksForVersion(name, nextPlayer || "");
          
          const filtered = (allPacks || []).filter(
            (p) => p.manifest.pack_type === 7
          );

          const basic = await Promise.all(
            filtered.map(async (p) => {
              try {
                const info = await (minecraft as any)?.GetPackInfo?.(p.path);
                return { ...info, path: p.path };
              } catch {
                return {
                  name: p.manifest.name,
                  description: p.manifest.description,
                  version: p.manifest.identity.version
                    ? `${p.manifest.identity.version.major}.${p.manifest.identity.version.minor}.${p.manifest.identity.version.patch}`
                    : "",
                  minEngineVersion: "",
                  iconDataUrl: "",
                  path: p.path,
                };
              }
            })
          );
          const withTime = await Promise.all(
            basic.map(async (p: any) => {
              let modTime = 0;
              try {
                if (typeof (minecraft as any).GetPathModTime === "function") {
                  modTime = await (minecraft as any).GetPathModTime(p.path);
                }
              } catch {}
              return { ...p, modTime };
            })
          );
          setPacks(withTime);
          Promise.resolve()
            .then(async () => {
              const readCache = () => {
                try {
                  return JSON.parse(
                    localStorage.getItem("content.size.cache") || "{}"
                  );
                } catch {
                  return {};
                }
              };
              const writeCache = (c: any) => {
                try {
                  localStorage.setItem("content.size.cache", JSON.stringify(c));
                } catch {}
              };
              const cache = readCache();
              const limit = 4;
              const items = withTime.slice();
              for (let i = 0; i < items.length; i += limit) {
                const chunk = items.slice(i, i + limit);
                await Promise.all(
                  chunk.map(async (p: any) => {
                    const key = p.path;
                    const c = cache[key];
                    if (
                      c &&
                      typeof c.size === "number" &&
                      Number(c.modTime || 0) === Number(p.modTime || 0)
                    ) {
                      setPacks((prev) =>
                        prev.map((it: any) =>
                          it.path === key ? { ...it, size: c.size } : it
                        )
                      );
                    } else {
                      let size = 0;
                      try {
                        if (typeof (minecraft as any).GetPathSize === "function") {
                          size = await (minecraft as any).GetPathSize(key);
                        }
                      } catch {}
                      cache[key] = { modTime: p.modTime || 0, size };
                      setPacks((prev) =>
                        prev.map((it: any) =>
                          it.path === key ? { ...it, size } : it
                        )
                      );
                    }
                  })
                );
                writeCache(cache);
              }
            })
            .catch(() => {});
        }
      } catch (e: any) {
        setError(e.toString());
      } finally {
        if (!silent) setLoading(false);
      }
    },
    [hasBackend, location?.state?.player, selectedPlayer]
  );

  React.useEffect(() => {
    refreshAll();
  }, [refreshAll]);

  // Old loadSkinPacks removed

  React.useEffect(() => {
    try {
      localStorage.setItem(
        "content.skin.sort",
        JSON.stringify({ sortKey, sortAsc })
      );
    } catch {}
  }, [sortKey, sortAsc]);

  const onChangePlayer = async (player: string) => {
    setSelectedPlayer(player);
    await refreshAll(false, player);
  };

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
  }, [packs]);

  const formatBytes = (n?: number) => {
    const v = typeof n === "number" ? n : 0;
    if (v < 1024) return `${v} B`;
    const k = 1024;
    const sizes = ["KB", "MB", "GB", "TB"];
    let i = -1;
    let val = v;
    do {
      val /= k;
      i++;
    } while (val >= k && i < sizes.length - 1);
    return `${val.toFixed(val >= 100 ? 0 : val >= 10 ? 1 : 2)} ${sizes[i]}`;
  };

  const formatDate = (ts?: number) => {
    const v = typeof ts === "number" ? ts : 0;
    if (!v) return "";
    const d = new Date(v * 1000);
    return d.toLocaleString();
  };

  return (
    <div
      ref={scrollRef}
      className="w-full h-full p-3 sm:p-4 lg:p-6 overflow-auto"
    >
      <motion.div
        initial={{ opacity: 0, y: 8 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.25 }}
        className="rounded-2xl border border-default-200 bg-white/60 dark:bg-neutral-900/60 backdrop-blur-md p-5"
      >
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-bold">
            {t("contentpage.skin_packs", { defaultValue: "皮肤包" })}
          </h1>
          <div className="flex items-center gap-2">
            <Button
              size="sm"
              variant="bordered"
              onPress={() => navigate("/content", { state: { player: selectedPlayer } })}
            >
              {t("common.back", { defaultValue: "返回" })}
            </Button>
            <Tooltip
              content={
                t("common.refresh", {
                  defaultValue: "刷新",
                }) as unknown as string
              }
            >
              <Button
                size="sm"
                variant="bordered"
                onPress={() => refreshAll()}
                isDisabled={loading}
              >
                {t("common.refresh", { defaultValue: "刷新" })}
              </Button>
            </Tooltip>
          </div>
        </div>
        <div className="mt-2 text-default-500 text-sm">
          {t("contentpage.current_version", { defaultValue: "当前版本" })}:{" "}
          <span className="font-medium">
            {currentVersionName ||
              t("contentpage.none", { defaultValue: "无" })}
          </span>
          <span className="mx-2">·</span>
          {t("contentpage.isolation", { defaultValue: "版本隔离" })}:{" "}
          <span className="font-medium">
            {roots.isIsolation
              ? t("common.yes", { defaultValue: "是" })
              : t("common.no", { defaultValue: "否" })}
          </span>
        </div>

        <div className="rounded-xl border border-default-200 bg-white/50 dark:bg-neutral-800/40 shadow-sm backdrop-blur-sm px-3 py-2 mt-4 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Dropdown>
              <DropdownTrigger>
                <Button
                  size="sm"
                  variant="flat"
                  className="rounded-full"
                  isDisabled={!players.length}
                >
                  {selectedPlayer ||
                    t("contentpage.select_player", {
                      defaultValue: "选择玩家",
                    })}
                </Button>
              </DropdownTrigger>
              <DropdownMenu
                aria-label={
                  t("contentpage.players_aria", {
                    defaultValue: "Players",
                  }) as unknown as string
                }
                selectionMode="single"
                selectedKeys={new Set([selectedPlayer])}
                onSelectionChange={(keys) => {
                  const arr = Array.from(keys as unknown as Set<string>);
                  const next = arr[0] || "";
                  if (typeof next === "string") onChangePlayer(next);
                }}
              >
                {players.length ? (
                  players.map((p) => (
                    <DropdownItem key={p} textValue={p}>
                      {p}
                    </DropdownItem>
                  ))
                ) : (
                  <DropdownItem key="none" isDisabled>
                    {t("contentpage.no_players", { defaultValue: "暂无玩家" })}
                  </DropdownItem>
                )}
              </DropdownMenu>
            </Dropdown>
          </div>
          <div className="flex items-center gap-2">
            <Input
              size="sm"
              variant="bordered"
              placeholder={
                t("common.search", { defaultValue: "搜索" }) as string
              }
              value={query}
              onValueChange={setQuery}
              className="w-40 sm:w-56"
            />
            <Dropdown>
              <DropdownTrigger>
                <Button size="sm" variant="flat" className="rounded-full">
                  {sortKey === "name"
                    ? (t("filemanager.sort.name", {
                        defaultValue: "名称",
                      }) as string)
                    : (t("contentpage.sort_time", {
                        defaultValue: "时间",
                      }) as string)}
                  {" / "}
                  {sortAsc
                    ? (t("contentpage.sort_asc", {
                        defaultValue: "从上到下",
                      }) as string)
                    : (t("contentpage.sort_desc", {
                        defaultValue: "从下到上",
                      }) as string)}
                </Button>
              </DropdownTrigger>
              <DropdownMenu
                aria-label={
                  t("contentpage.sort_aria", { defaultValue: "排序" }) as string
                }
                selectionMode="single"
                onSelectionChange={(keys) => {
                  const k = Array.from(keys as unknown as Set<string>)[0] || "";
                  if (k === "name" || k === "time") setSortKey(k as any);
                }}
              >
                <DropdownItem key="name">
                  {
                    t("filemanager.sort.name", {
                      defaultValue: "名称",
                    }) as string
                  }
                </DropdownItem>
                <DropdownItem key="time">
                  {
                    t("contentpage.sort_time", {
                      defaultValue: "时间",
                    }) as string
                  }
                </DropdownItem>
              </DropdownMenu>
            </Dropdown>
            <Button
              size="sm"
              variant="bordered"
              onPress={() => setSortAsc((v) => !v)}
            >
              {sortAsc
                ? (t("contentpage.sort_asc", {
                    defaultValue: "从上到下",
                  }) as string)
                : (t("contentpage.sort_desc", {
                    defaultValue: "从下到上",
                  }) as string)}
            </Button>
            <Button
              size="sm"
              variant="bordered"
              onPress={async () => {
                if (!hasBackend || !roots.resourcePacks) return;
                let sp = roots.resourcePacks.replace(/resource_packs$/, "skin_packs");
                if (selectedPlayer && roots.usersRoot) {
                   sp = `${roots.usersRoot}\\${selectedPlayer}\\games\\com.mojang\\skin_packs`;
                }
                await OpenPathDir(sp);
              }}
              isDisabled={!roots.resourcePacks || !hasBackend}
            >
              {t("common.open", { defaultValue: "打开" })}
            </Button>
            <Button
              size="sm"
              variant="bordered"
              onPress={() => setSelectMode((v) => !v)}
            >
              {selectMode
                ? (t("common.cancel", { defaultValue: "取消选择" }) as string)
                : (t("common.select", { defaultValue: "选择" }) as string)}
            </Button>
            {selectMode ? (
              <Button
                size="sm"
                color="danger"
                variant="bordered"
                onPress={() => {
                  const has = packs.some((p: any) => selected[p.path]);
                  if (has) delManyCfmOnOpen();
                }}
                isDisabled={!packs.some((p: any) => selected[p.path])}
              >
                {t("common.delete", { defaultValue: "删除" })}
              </Button>
            ) : null}
          </div>
        </div>

        <div className="mt-3">
          {loading ? (
            <div className="flex items-center gap-2">
              <Spinner size="sm" />{" "}
              <span className="text-default-500">
                {t("common.loading", { defaultValue: "加载中" })}
              </span>
            </div>
          ) : (
            <div className="flex flex-col gap-2">
              {packs.length ? (
                <div className="grid gap-4 grid-cols-1 sm:grid-cols-2">
                  {(() => {
                    const q = query.trim().toLowerCase();
                    const filtered = packs.filter((p) => {
                      const nm = String(
                        p.name || p.path?.split("\\").pop() || ""
                      ).toLowerCase();
                      return q ? nm.includes(q) : true;
                    });
                    const sorted = filtered.sort((a: any, b: any) => {
                      if (sortKey === "name") {
                        const an = String(
                          a.name || a.path?.split("\\").pop() || ""
                        ).toLowerCase();
                        const bn = String(
                          b.name || b.path?.split("\\").pop() || ""
                        ).toLowerCase();
                        const res = an.localeCompare(bn);
                        return sortAsc ? res : -res;
                      } else {
                        const at = Number(a.modTime || 0);
                        const bt = Number(b.modTime || 0);
                        const res = at - bt;
                        return sortAsc ? res : -res;
                      }
                    });
                    return sorted.map((p: any, idx: number) => (
                      <div
                        key={`${p.path}-${idx}`}
                        className={`relative rounded-xl border p-3 transition-colors h-36 ${
                          selectMode ? "cursor-pointer" : "cursor-default"
                        } ${
                          selectMode && selected[p.path]
                            ? "border-primary-300 dark:border-primary-400 bg-primary/10 dark:bg-primary/15 shadow-sm"
                            : "border-default-200 bg-white/70 dark:bg-neutral-800/50 hover:bg-white/80 dark:hover:bg-neutral-800/60"
                        }`}
                        onClick={() => {
                          if (!selectMode) return;
                          setSelected((prev) => ({
                            ...prev,
                            [p.path]: !prev[p.path],
                          }));
                        }}
                      >
                        <div className="flex items-start h-full">
                          {selectMode ? (
                            <div
                              className="mr-2 shrink-0 flex items-center"
                              onClick={(e) => e.stopPropagation()}
                            >
                              <Checkbox
                                size="sm"
                                isSelected={!!selected[p.path]}
                                onValueChange={() =>
                                  setSelected((prev) => ({
                                    ...prev,
                                    [p.path]: !prev[p.path],
                                  }))
                                }
                              />
                            </div>
                          ) : null}
                          <div className="flex-1 min-w-0 flex flex-col pb-12 pr-3">
                            <div className="flex items-center justify-between">
                              <div className="font-semibold truncate">
                                {renderMcText(
                                  p.name || p.path.split("\\").pop()
                                )}
                              </div>
                              {p.version ? (
                                <Chip
                                  size="sm"
                                  variant="flat"
                                  className="shrink-0"
                                >
                                  {p.version}
                                </Chip>
                              ) : null}
                            </div>
                            <div className="flex-1 min-h-0 overflow-hidden">
                              <div className="text-small text-default-600 mt-1 overflow-hidden text-ellipsis whitespace-nowrap">
                                {renderMcText(p.description || "")}
                              </div>
                              <div className="text-tiny text-default-500 mt-1 overflow-hidden text-ellipsis whitespace-nowrap">
                                {p.minEngineVersion
                                  ? `${t("contentpage.min_engine_version", {
                                      defaultValue: "最小游戏版本",
                                    })}: ${p.minEngineVersion}`
                                  : "\u00A0"}
                              </div>
                              <div className="text-tiny text-default-500 mt-1 overflow-hidden text-ellipsis whitespace-nowrap">
                                {`${t("filemanager.sort.size", {
                                  defaultValue: "大小",
                                })}: ${formatBytes(p.size)} · ${t(
                                  "contentpage.sort_time",
                                  { defaultValue: "时间" }
                                )}: ${formatDate(p.modTime)}`}
                              </div>
                            </div>
                          </div>
                        </div>
                        <div
                          className="absolute right-3 bottom-3 flex items-center gap-2"
                          onClick={(e) => e.stopPropagation()}
                        >
                          <Button
                            size="sm"
                            variant="flat"
                            onPress={() => OpenPathDir(p.path)}
                          >
                            {t("common.open", { defaultValue: "打开" })}
                          </Button>
                          <Button
                            size="sm"
                            color="danger"
                            variant="flat"
                            onPress={() => {
                              setActivePack(p);
                              delCfmOnOpen();
                            }}
                          >
                            {t("common.delete", { defaultValue: "删除" })}
                          </Button>
                        </div>
                      </div>
                    ));
                  })()}
                </div>
              ) : (
                <div className="text-default-500">
                  {t("contentpage.no_skin_packs", {
                    defaultValue: "暂无皮肤包",
                  })}
                </div>
              )}
            </div>
          )}
        </div>

        <Modal
          size="sm"
          isOpen={delCfmOpen}
          onOpenChange={delCfmOnOpenChange}
          hideCloseButton
        >
          <ModalContent>
            {(onClose) => (
              <>
                <ModalHeader className="text-danger">
                  {t("mods.confirm_delete_title", { defaultValue: "确认删除" })}
                </ModalHeader>
                <ModalBody>
                  <div className="text-sm text-default-700 break-words whitespace-pre-wrap">
                    {t("mods.confirm_delete_body", {
                      type: t("contentpage.skin_packs"),
                      defaultValue: "确定要删除此包吗？此操作不可撤销。",
                    })}
                  </div>
                  {activePack ? (
                    <div className="mt-1 rounded-md bg-default-100/60 border border-default-200 px-3 py-2 text-default-800 text-sm break-words whitespace-pre-wrap">
                      {activePack.name || activePack.path}
                    </div>
                  ) : null}
                </ModalBody>
                <ModalFooter>
                  <Button
                    variant="light"
                    onPress={() => {
                      onClose();
                    }}
                  >
                    {t("common.cancel", { defaultValue: "取消" })}
                  </Button>
                  <Button
                    color="danger"
                    isLoading={deletingOne}
                    onPress={async () => {
                      if (!activePack) {
                        onClose();
                        return;
                      }
                      const pos =
                        scrollRef.current?.scrollTop ??
                        (document.scrollingElement as any)?.scrollTop ??
                        0;
                      setDeletingOne(true);
                      lastScrollTopRef.current = pos;
                      restorePendingRef.current = true;
                      const err = await (minecraft as any)?.DeletePack?.(
                        currentVersionName,
                        activePack.path
                      );
                      if (err) {
                        setResultSuccess([]);
                        setResultFailed([
                          { name: activePack.name || activePack.path, err },
                        ]);
                        delOnOpen();
                      } else {
                        await refreshAll(true);
                        requestAnimationFrame(() => {
                          try {
                            if (scrollRef.current)
                              scrollRef.current.scrollTop = pos;
                            else window.scrollTo({ top: pos });
                          } catch {}
                        });
                        setResultSuccess([activePack.name || activePack.path]);
                        setResultFailed([]);
                        delOnOpen();
                      }
                      setDeletingOne(false);
                      onClose();
                    }}
                  >
                    {t("common.confirm", { defaultValue: "确定" })}
                  </Button>
                </ModalFooter>
              </>
            )}
          </ModalContent>
        </Modal>
        <Modal
          size="sm"
          isOpen={delManyCfmOpen}
          onOpenChange={delManyCfmOnOpenChange}
          hideCloseButton
        >
          <ModalContent>
            {(onClose) => (
              <>
                <ModalHeader className="text-danger">
                  {t("mods.confirm_delete_title", { defaultValue: "确认删除" })}
                </ModalHeader>
                <ModalBody>
                  <div className="text-sm text-default-700 break-words whitespace-pre-wrap">
                    {t("mods.confirm_delete_body", {
                      type: t("contentpage.skin_packs"),
                      defaultValue: "确定要删除此包吗？此操作不可撤销。",
                    })}
                  </div>
                  <div className="mt-1 rounded-md bg-default-100/60 border border-default-200 px-3 py-2 text-default-800 text-sm break-words whitespace-pre-wrap">
                    {packs
                      .filter((p: any) => selected[p.path])
                      .map((p: any) => p.name || p.path)
                      .join("\n")}
                  </div>
                </ModalBody>
                <ModalFooter>
                  <Button
                    variant="light"
                    onPress={() => {
                      onClose();
                    }}
                  >
                    {t("common.cancel", { defaultValue: "取消" })}
                  </Button>
                  <Button
                    color="danger"
                    isLoading={deletingMany}
                    onPress={async () => {
                      setDeletingMany(true);
                      const paths = packs
                        .filter((p: any) => selected[p.path])
                        .map((p: any) => p.path);
                      const pos =
                        scrollRef.current?.scrollTop ??
                        (document.scrollingElement as any)?.scrollTop ??
                        0;
                      lastScrollTopRef.current = pos;
                      restorePendingRef.current = true;
                      const ok: string[] = [];
                      const failed: Array<{ name: string; err: string }> = [];
                      for (const p of paths) {
                        const err = await (minecraft as any)?.DeletePack?.(
                          currentVersionName,
                          p
                        );
                        const it = packs.find((x: any) => x.path === p);
                        const nm = it?.name || p;
                        if (err) failed.push({ name: nm, err });
                        else ok.push(nm);
                      }
                      setResultSuccess(ok);
                      setResultFailed(failed);
                      delOnOpen();
                      await refreshAll(true);
                      requestAnimationFrame(() => {
                        try {
                          if (scrollRef.current)
                            scrollRef.current.scrollTop = pos;
                          else window.scrollTo({ top: pos });
                        } catch {}
                      });
                      setDeletingMany(false);
                      onClose();
                    }}
                  >
                    {t("common.confirm", { defaultValue: "确定" })}
                  </Button>
                </ModalFooter>
              </>
            )}
          </ModalContent>
        </Modal>
        <Modal
          size="md"
          isOpen={delOpen}
          onOpenChange={delOnOpenChange}
          hideCloseButton
        >
          <ModalContent>
            {(onClose) => (
              <>
                <ModalHeader
                  className={`flex items-center gap-2 ${
                    resultFailed.length ? "text-red-600" : "text-primary-600"
                  }`}
                >
                  <span>
                    {resultFailed.length
                      ? t("mods.delete_summary_title_failed", {
                          defaultValue: "删除失败",
                        })
                      : t("mods.delete_summary_title_done", {
                          defaultValue: "删除完成",
                        })}
                  </span>
                </ModalHeader>
                <ModalBody>
                  {resultSuccess.length ? (
                    <div className="mb-2">
                      <div className="text-sm font-semibold text-success">
                        {t("mods.summary_deleted", { defaultValue: "已删除" })}{" "}
                        ({resultSuccess.length})
                      </div>
                      <div className="mt-1 rounded-md bg-success/5 border border-success/30 px-3 py-2 text-success-700 text-sm break-words whitespace-pre-wrap">
                        {resultSuccess.join("\n")}
                      </div>
                    </div>
                  ) : null}
                  {resultFailed.length ? (
                    <div>
                      <div className="text-sm font-semibold text-danger">
                        {t("mods.summary_failed", { defaultValue: "失败" })} (
                        {resultFailed.length})
                      </div>
                      <div className="mt-1 rounded-md bg-danger/5 border border-danger/30 px-3 py-2 text-danger-700 text-sm break-words whitespace-pre-wrap">
                        {resultFailed
                          .map((it) => `${it.name} - ${it.err}`)
                          .join("\n")}
                      </div>
                    </div>
                  ) : null}
                </ModalBody>
                <ModalFooter>
                  <Button
                    color="primary"
                    onPress={() => {
                      setResultSuccess([]);
                      setResultFailed([]);
                      onClose();
                    }}
                  >
                    {t("common.confirm", { defaultValue: "确定" })}
                  </Button>
                </ModalFooter>
              </>
            )}
          </ModalContent>
        </Modal>
      </motion.div>
    </div>
  );
}
