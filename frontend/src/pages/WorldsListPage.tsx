import React from "react";
import { useTranslation } from "react-i18next";
import {
  Button,
  Card,
  CardBody,
  CardHeader,
  Dropdown,
  DropdownItem,
  DropdownMenu,
  DropdownTrigger,
  Spinner,
  Tooltip,
  Input,
  Checkbox,
  Modal,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  useDisclosure,
} from "@heroui/react";
import { motion } from "framer-motion";
import { useNavigate } from "react-router-dom";
import {
  GetContentRoots,
  OpenPathDir,
} from "../../bindings/github.com/liteldev/LeviLauncher/minecraft";
import * as types from "../../bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import { readCurrentVersionName } from "../utils/currentVersion";
import { listPlayers } from "../utils/content";
import { listDirectories } from "../utils/fs";
import * as minecraft from "../../bindings/github.com/liteldev/LeviLauncher/minecraft";

export default function WorldsListPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
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
  type WorldItem = {
    name: string;
    path: string;
    levelName?: string;
    iconDataUrl?: string;
    size?: number;
    modTime?: number;
  };
  const [worldEntries, setWorldEntries] = React.useState<WorldItem[]>([]);
  const [backingUp, setBackingUp] = React.useState<string>("");
  const [backupDest, setBackupDest] = React.useState<string>("");
  const [query, setQuery] = React.useState<string>("");
  const [sortKey, setSortKey] = React.useState<"name" | "time">("name");
  const [sortAsc, setSortAsc] = React.useState<boolean>(true);
  const [selected, setSelected] = React.useState<Record<string, boolean>>({});
  const [selectMode, setSelectMode] = React.useState<boolean>(false);
  const [activeWorld, setActiveWorld] = React.useState<WorldItem | null>(null);
  const { isOpen: delCfmOpen, onOpen: delCfmOnOpen, onOpenChange: delCfmOnOpenChange } = useDisclosure();
  const { isOpen: delOpen, onOpen: delOnOpen, onOpenChange: delOnOpenChange } = useDisclosure();
  const { isOpen: delManyCfmOpen, onOpen: delManyCfmOnOpen, onOpenChange: delManyCfmOnOpenChange } = useDisclosure();
  const [resultSuccess, setResultSuccess] = React.useState<string[]>([]);
  const [resultFailed, setResultFailed] = React.useState<Array<{ name: string; err: string }>>([]);
  const [deletingOne, setDeletingOne] = React.useState<boolean>(false);
  const [deletingMany, setDeletingMany] = React.useState<boolean>(false);
  const scrollRef = React.useRef<HTMLDivElement | null>(null);
  const lastScrollTopRef = React.useRef<number>(0);
  const restorePendingRef = React.useRef<boolean>(false);

  const loadWorlds = async (player: string, r: types.ContentRoots) => {
    if (!hasBackend || !r?.usersRoot || !player) {
      setWorldEntries([]);
      return;
    }
    const wp = `${r.usersRoot}\\${player}\\games\\com.mojang\\minecraftWorlds`;
    try {
      const dirs = await listDirectories(wp);
      const basic = await Promise.all(
        dirs.map(async (d) => {
          let levelName = "";
          let iconDataUrl = "";
          try {
            if (
              minecraft &&
              typeof minecraft.GetWorldLevelName === "function"
            ) {
              levelName = await minecraft.GetWorldLevelName(d.path);
            }
            if (
              minecraft &&
              typeof minecraft.GetWorldIconDataUrl === "function"
            ) {
              iconDataUrl = await minecraft.GetWorldIconDataUrl(d.path);
            }
          } catch {}
          return { ...d, levelName, iconDataUrl } as WorldItem;
        })
      );
      setWorldEntries(basic);
      Promise.resolve()
        .then(async () => {
          const filled = await Promise.all(
            basic.map(async (w) => {
              let size = 0;
              let modTime = 0;
              try {
                if (typeof (minecraft as any).GetPathSize === "function") {
                  size = await (minecraft as any).GetPathSize(w.path);
                }
                if (typeof (minecraft as any).GetPathModTime === "function") {
                  modTime = await (minecraft as any).GetPathModTime(w.path);
                }
              } catch {}
              return { ...w, size, modTime } as WorldItem;
            })
          );
          setWorldEntries(filled);
        })
        .catch(() => {});
    } catch {
      setWorldEntries([]);
    }
  };

  const refreshAll = React.useCallback(async (silent?: boolean) => {
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
        setWorldEntries([]);
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
        const names = await listPlayers(safe.usersRoot);
        setPlayers(names);
        const nextPlayer = names[0] || "";
        setSelectedPlayer(nextPlayer);
        await loadWorlds(nextPlayer, safe);
      }
    } catch (e) {
      setError(
        t("contentpage.error_resolve_paths", {
          defaultValue: "无法解析内容路径。",
        }) as string
      );
    } finally {
      if (!silent) setLoading(false);
    }
  }, [hasBackend, t]);

  React.useEffect(() => {
    refreshAll();
  }, []);

  React.useLayoutEffect(() => {
    if (!restorePendingRef.current) return;
    requestAnimationFrame(() => {
      try {
        if (scrollRef.current) scrollRef.current.scrollTop = lastScrollTopRef.current; else window.scrollTo({ top: lastScrollTopRef.current });
      } catch {}
    });
    restorePendingRef.current = false;
  }, [worldEntries]);

  const onChangePlayer = async (player: string) => {
    setSelectedPlayer(player);
    await loadWorlds(player, roots);
  };

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
    <div ref={scrollRef} className="w-full h-full p-3 sm:p-4 lg:p-6 overflow-auto">
      <motion.div
        initial={{ opacity: 0, y: 8 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.25 }}
        className="rounded-2xl border border-default-200 bg-white/60 dark:bg-neutral-900/60 backdrop-blur-md p-5"
      >
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-bold">
            {t("contentpage.worlds", { defaultValue: "世界" })}
          </h1>
          <div className="flex items-center gap-2">
            <Button
              size="sm"
              variant="bordered"
              onPress={() => navigate("/content")}
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
                onPress={refreshAll}
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

        <Card className="rounded-2xl mt-4">
          <CardHeader className="flex items-center justify-between">
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
                      {t("contentpage.no_players", {
                        defaultValue: "暂无玩家",
                      })}
                    </DropdownItem>
                  )}
                </DropdownMenu>
              </Dropdown>
            </div>
            <div className="flex items-center gap-2">
              <Input
                size="sm"
                variant="bordered"
                placeholder={t("common.search", { defaultValue: "搜索" }) as string}
                value={query}
                onValueChange={setQuery}
                className="w-40 sm:w-56"
              />
              <Dropdown>
                <DropdownTrigger>
                  <Button size="sm" variant="flat" className="rounded-full">
                    {sortKey === "name"
                      ? (t("filemanager.sort.name", { defaultValue: "名称" }) as string)
                      : (t("contentpage.sort_time", { defaultValue: "时间" }) as string)}
                    {" / "}
                    {sortAsc
                      ? (t("contentpage.sort_asc", { defaultValue: "从上到下" }) as string)
                      : (t("contentpage.sort_desc", { defaultValue: "从下到上" }) as string)}
                  </Button>
                </DropdownTrigger>
                <DropdownMenu
                  aria-label={t("contentpage.sort_aria", { defaultValue: "排序" }) as string}
                  selectionMode="single"
                  onSelectionChange={(keys) => {
                    const k = Array.from(keys as unknown as Set<string>)[0] || "";
                    if (k === "name" || k === "time") setSortKey(k as any);
                  }}
                >
                  <DropdownItem key="name">{t("filemanager.sort.name", { defaultValue: "名称" }) as string}</DropdownItem>
                  <DropdownItem key="time">{t("contentpage.sort_time", { defaultValue: "时间" }) as string}</DropdownItem>
                </DropdownMenu>
              </Dropdown>
              <Button size="sm" variant="bordered" onPress={() => setSortAsc((v) => !v)}>
                {sortAsc
                  ? (t("contentpage.sort_asc", { defaultValue: "从上到下" }) as string)
                  : (t("contentpage.sort_desc", { defaultValue: "从下到上" }) as string)}
              </Button>
              <Button
                size="sm"
                variant="bordered"
                onPress={async () => {
                  if (!hasBackend || !roots.usersRoot || !selectedPlayer)
                    return;
                  const wp = `${roots.usersRoot}\\${selectedPlayer}\\games\\com.mojang\\minecraftWorlds`;
                  await OpenPathDir(wp);
                }}
                isDisabled={!roots.usersRoot || !selectedPlayer || !hasBackend}
              >
                {t("common.open", { defaultValue: "打开" })}
              </Button>
              <Button size="sm" variant="bordered" onPress={() => setSelectMode((v) => !v)}>
                {selectMode ? (t("common.cancel", { defaultValue: "取消选择" }) as string) : (t("common.select", { defaultValue: "选择" }) as string)}
              </Button>
              {selectMode ? (
                <Button
                  size="sm"
                  color="danger"
                  variant="bordered"
                  onPress={() => {
                    const paths = Object.keys(selected).filter((k) => selected[k]);
                    if (!paths.length) return;
                    delManyCfmOnOpen();
                  }}
                  isDisabled={!Object.keys(selected).some((k) => selected[k])}
                >
                  {t("common.delete", { defaultValue: "删除" })}
                </Button>
              ) : null}
            </div>
          </CardHeader>
          <CardBody>
            {loading ? (
              <div className="flex items-center gap-2">
                <Spinner size="sm" />{" "}
                <span className="text-default-500">
                  {t("common.loading", { defaultValue: "加载中" })}
                </span>
              </div>
            ) : (
              <div className="flex flex-col gap-2">
                {backupDest ? (
                  <div className="text-success text-sm">
                    {t("contentpage.backup_success", {
                      defaultValue: "备份成功",
                    })}
                    : <span className="underline break-all">{backupDest}</span>
                  </div>
                ) : null}
                {worldEntries.length ? (
                  (() => {
                    const q = query.trim().toLowerCase();
                    const filtered = worldEntries.filter((w) => {
                      const nm = (w.levelName || w.name || "").toLowerCase();
                      return q ? nm.includes(q) : true;
                    });
                    const sorted = filtered.sort((a, b) => {
                      if (sortKey === "name") {
                        const an = (a.levelName || a.name || "").toLowerCase();
                        const bn = (b.levelName || b.name || "").toLowerCase();
                        const res = an.localeCompare(bn);
                        return sortAsc ? res : -res;
                      } else {
                        const at = a.modTime || 0;
                        const bt = b.modTime || 0;
                        const res = at - bt;
                        return sortAsc ? res : -res;
                      }
                    });
                    return sorted.map((w) => (
                      <div
                        key={w.path}
                        className="flex items-center justify-between rounded-xl px-3 py-2 bg-default-100/50"
                      >
                        <div className="flex items-center gap-3 overflow-hidden">
                          {selectMode ? (
                          <Checkbox
                            size="sm"
                            isSelected={!!selected[w.path]}
                            onValueChange={() =>
                              setSelected((prev) => ({ ...prev, [w.path]: !prev[w.path] }))
                            }
                            className="shrink-0"
                          />
                          ) : null}
                          {w.iconDataUrl ? (
                            <img
                              src={w.iconDataUrl}
                              alt={w.levelName || w.name}
                              className="w-10 h-10 rounded-md object-cover flex-shrink-0"
                            />
                          ) : (
                            <div className="w-10 h-10 rounded-md bg-default-200 flex-shrink-0" />
                          )}
                          <div className="truncate">
                            <div className="font-medium truncate">
                              {w.levelName || w.name}
                            </div>
                            <div className="text-xs text-default-500 truncate">
                            {w.name}
                            </div>
                            <div className="text-xs text-default-500 truncate mt-0.5">
                              {`${t("filemanager.sort.size", { defaultValue: "大小" })}: ${formatBytes(w.size)} · ${t("contentpage.sort_time", { defaultValue: "时间" })}: ${formatDate(w.modTime)}`}
                            </div>
                          </div>
                        </div>
                        <div className="flex items-center gap-2">
                          <Button
                            size="sm"
                            variant="flat"
                            onPress={() => OpenPathDir(w.path)}
                            isDisabled={!hasBackend}
                          >
                            {t("common.open", { defaultValue: "打开" })}
                          </Button>
                          <Button
                            size="sm"
                            color="danger"
                            variant="flat"
                            onPress={() => { setActiveWorld(w); delCfmOnOpen(); }}
                            isDisabled={!hasBackend}
                          >
                            {t("common.delete", { defaultValue: "删除" })}
                          </Button>
                          <Button
                            size="sm"
                            color="primary"
                            variant="flat"
                            isLoading={backingUp === w.path}
                            isDisabled={!hasBackend}
                            onPress={async () => {
                              if (!minecraft) return;
                              setBackupDest("");
                              setBackingUp(w.path);
                              try {
                                let dest = "";
                                if (
                                  typeof minecraft.BackupWorldWithVersion ===
                                  "function"
                                ) {
                                  dest = await minecraft.BackupWorldWithVersion(
                                    w.path,
                                    currentVersionName
                                  );
                                } else if (
                                  typeof minecraft.BackupWorld === "function"
                                ) {
                                  dest = await minecraft.BackupWorld(w.path);
                                }
                                if (dest) setBackupDest(dest);
                              } catch {}
                              setBackingUp("");
                            }}
                          >
                            {t("contentpage.backup", { defaultValue: "备份" })}
                          </Button>
                        </div>
                      </div>
                    ));
                  })()
                ) : (
                  <div className="text-default-500">
                    {t("contentpage.no_worlds", { defaultValue: "暂无世界" })}
                  </div>
                )}
              </div>
            )}
          </CardBody>
        </Card>

        <Modal size="sm" isOpen={delCfmOpen} onOpenChange={delCfmOnOpenChange} hideCloseButton>
          <ModalContent>
            {(onClose) => (
              <>
                <ModalHeader className="text-danger">{t("mods.confirm_delete_title", { defaultValue: "确认删除" })}</ModalHeader>
                <ModalBody>
                  <div className="text-sm text-default-700 break-words whitespace-pre-wrap">{t("mods.confirm_delete_body", { defaultValue: "确定要删除此包吗？此操作不可撤销。" })}</div>
                  {activeWorld ? (<div className="mt-1 rounded-md bg-default-100/60 border border-default-200 px-3 py-2 text-default-800 text-sm break-words whitespace-pre-wrap">{activeWorld.levelName || activeWorld.name}</div>) : null}
                </ModalBody>
                <ModalFooter>
                  <Button variant="light" onPress={() => { onClose(); }}>{t("common.cancel", { defaultValue: "取消" })}</Button>
                  <Button color="danger" isLoading={deletingOne} onPress={async () => {
                    if (!activeWorld) { onClose(); return; }
                    const pos = scrollRef.current?.scrollTop ?? (document.scrollingElement as any)?.scrollTop ?? 0;
                    setDeletingOne(true);
                    lastScrollTopRef.current = pos;
                    restorePendingRef.current = true;
                    const err = await (minecraft as any)?.DeleteWorld?.(currentVersionName, activeWorld.path);
                    if (err) {
                      setResultSuccess([]);
                      setResultFailed([{ name: activeWorld.levelName || activeWorld.name || activeWorld.path, err }]);
                      delOnOpen();
                    } else {
                      await refreshAll(true);
                      setResultSuccess([activeWorld.levelName || activeWorld.name || activeWorld.path]);
                      setResultFailed([]);
                      delOnOpen();
                    }
                    setDeletingOne(false);
                    onClose();
                  }}>{t("common.confirm", { defaultValue: "确定" })}</Button>
                </ModalFooter>
              </>
            )}
          </ModalContent>
        </Modal>
        <Modal size="sm" isOpen={delManyCfmOpen} onOpenChange={delManyCfmOnOpenChange} hideCloseButton>
          <ModalContent>
            {(onClose) => (
              <>
                <ModalHeader className="text-danger">{t("mods.confirm_delete_title", { defaultValue: "确认删除" })}</ModalHeader>
                <ModalBody>
                  <div className="text-sm text-default-700 break-words whitespace-pre-wrap">{t("mods.confirm_delete_body", { defaultValue: "确定要删除此包吗？此操作不可撤销。" })}</div>
                  <div className="mt-1 rounded-md bg-default-100/60 border border-default-200 px-3 py-2 text-default-800 text-sm break-words whitespace-pre-wrap">{Object.keys(selected).filter((k) => selected[k]).map((p) => {
                    const it = worldEntries.find((w) => w.path === p);
                    return (it?.levelName || it?.name || p);
                  }).join("\n")}</div>
                </ModalBody>
                <ModalFooter>
                  <Button variant="light" onPress={() => { onClose(); }}>{t("common.cancel", { defaultValue: "取消" })}</Button>
                  <Button color="danger" isLoading={deletingMany} onPress={async () => {
                    setDeletingMany(true);
                    const pos = scrollRef.current?.scrollTop ?? (document.scrollingElement as any)?.scrollTop ?? 0;
                    lastScrollTopRef.current = pos;
                    restorePendingRef.current = true;
                    const paths = Object.keys(selected).filter((k) => selected[k]);
                    const ok: string[] = [];
                    const failed: Array<{ name: string; err: string }> = [];
                    for (const p of paths) {
                      const err = await (minecraft as any)?.DeleteWorld?.(currentVersionName, p);
                      const it = worldEntries.find((w) => w.path === p);
                      const nm = it?.levelName || it?.name || p;
                      if (err) failed.push({ name: nm, err }); else ok.push(nm);
                    }
                    setResultSuccess(ok);
                    setResultFailed(failed);
                    delOnOpen();
                    await refreshAll(true);
                    setDeletingMany(false);
                    onClose();
                  }}>{t("common.confirm", { defaultValue: "确定" })}</Button>
                </ModalFooter>
              </>
            )}
          </ModalContent>
        </Modal>
        <Modal size="md" isOpen={delOpen} onOpenChange={delOnOpenChange} hideCloseButton>
          <ModalContent>
            {(onClose) => (
              <>
                <ModalHeader className={`flex items-center gap-2 ${resultFailed.length ? "text-red-600" : "text-primary-600"}`}>
                  <span>{resultFailed.length ? t("mods.delete_summary_title_failed", { defaultValue: "删除失败" }) : t("mods.delete_summary_title_done", { defaultValue: "删除完成" })}</span>
                </ModalHeader>
                <ModalBody>
                  {resultSuccess.length ? (
                    <div className="mb-2">
                      <div className="text-sm font-semibold text-success">{t("mods.summary_deleted", { defaultValue: "已删除" })} ({resultSuccess.length})</div>
                      <div className="mt-1 rounded-md bg-success/5 border border-success/30 px-3 py-2 text-success-700 text-sm break-words whitespace-pre-wrap">{resultSuccess.join("\n")}</div>
                    </div>
                  ) : null}
                  {resultFailed.length ? (
                    <div>
                      <div className="text-sm font-semibold text-danger">{t("mods.summary_failed", { defaultValue: "失败" })} ({resultFailed.length})</div>
                      <div className="mt-1 rounded-md bg-danger/5 border border-danger/30 px-3 py-2 text-danger-700 text-sm break-words whitespace-pre-wrap">{resultFailed.map((it) => `${it.name} - ${it.err}`).join("\n")}</div>
                    </div>
                  ) : null}
                </ModalBody>
                <ModalFooter>
                  <Button color="primary" onPress={() => { setResultSuccess([]); setResultFailed([]); onClose(); }}>{t("common.confirm", { defaultValue: "确定" })}</Button>
                </ModalFooter>
              </>
            )}
          </ModalContent>
        </Modal>
      </motion.div>
    </div>
  );
}
