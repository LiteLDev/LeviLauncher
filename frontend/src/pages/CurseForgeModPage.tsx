import { ModalAction, ModalDescription, ModalPanel, ModalNotice, ModalProgress } from "@/components/ModalPrimitives";
import {
  Button,
  Card,
  Chip,
  Label,
  Link,
  ListBox,
  ProgressBar,
  Select,
  Skeleton,
  Spinner,
  Table,
  Tabs,
  Tooltip,
} from "@heroui/react";

import { cn } from "@/utils/cn";

import React, { useEffect, useState, useRef } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useRouteTitle } from "@/hooks/useRouteTitle";
import { ProjectShareButton } from "@/components/ProjectShareButton";
import {
  GetCurseForgeModsByIDs,
  GetCurseForgeModDescription,
  GetCurseForgeModFiles,
  StartFileDownload,
  CancelFileDownload,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import {
  GetContentRoots,
  ImportMcpackPath,
  ImportMcpackPathWithPlayer,
  ImportMcaddonPath,
  ImportMcaddonPathWithPlayer,
  ImportMcworldPath,
  IsMcpackSkinPackPath,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/contentservice";
import {
  ListVersionMetasWithRegistered,
  GetVersionLogoDataUrl,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/versionservice";
import { GetLocalUserGamertag } from "bindings/github.com/liteldev/LeviLauncher/internal/app/userservice";
import { Events, Browser } from "@wailsio/runtime";
import { VersionMeta } from "bindings/github.com/liteldev/LeviLauncher/internal/versions/models";
import { File as ModFile } from "bindings/github.com/liteldev/LeviLauncher/internal/curseforge/client/types";
import {
  getPlayerGamertagMap,
  listPlayers,
  resolvePlayerDisplayName,
} from "@/utils/content";
import { readCurrentVersionName } from "@/utils/currentVersion";
import { compareVersions } from "@/utils/version";
import { shouldDisableAnimations } from "@/hooks/useAnimations";
import { UnifiedModal } from "@/components/UnifiedModal";
import { motion } from "framer-motion";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";

import { FiCheckCircle } from "react-icons/fi";
import {
  LuDownload,
  LuCalendar,
  LuFileDigit,
  LuGlobe,
  LuGithub,
  LuBug,
  LuGamepad2,
  LuUser,
} from "react-icons/lu";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import {
  formatNumber,
  formatDateStr,
  formatFileSize,
  sortGameVersions,
} from "@/utils/formatting";

const CurseForgeModPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();

  const [mod, setMod] = useState<any | null>(null);
  useRouteTitle(mod?.id === Number(id) ? mod?.name : undefined);
  const [description, setDescription] = useState<string>("");
  const [loadError, setLoadError] = useState(false);
  const [descriptionLoading, setDescriptionLoading] = useState(true);
  const [descriptionError, setDescriptionError] = useState(false);
  const [filesLoading, setFilesLoading] = useState(true);
  const [filesError, setFilesError] = useState(false);
  const [detailRetry, setDetailRetry] = useState(0);
  const [descriptionRetry, setDescriptionRetry] = useState(0);
  const [filesRetry, setFilesRetry] = useState(0);
  const [files, setFiles] = useState<ModFile[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedGameVersion, setSelectedGameVersion] = useState<string>("all");
  const [selectedTab, setSelectedTab] = useState<string>("description");
  const [isEntering, setIsEntering] = useState(true);
  const tabsRef = useRef<HTMLDivElement>(null);

  const [installModalOpen, setInstallModalOpen] = useState(false);
  const [installStep, setInstallStep] = useState<
    | "downloading"
    | "version_select"
    | "player_select"
    | "importing"
    | "success"
    | "error"
  >("downloading");
  const [installFile, setInstallFile] = useState<{
    name: string;
    path: string;
    type: string;
  } | null>(null);
  const [downloadProgress, setDownloadProgress] = useState<{
    downloaded: number;
    total: number;
  } | null>(null);
  const [availableVersions, setAvailableVersions] = useState<VersionMeta[]>([]);
  const [versionLogos, setVersionLogos] = useState<Record<string, string>>({});
  const [availablePlayers, setAvailablePlayers] = useState<string[]>([]);
  const [playerGamertagMap, setPlayerGamertagMap] = useState<
    Record<string, string>
  >({});
  const [selectedVersion, setSelectedVersion] = useState<string>("");
  const [selectedPlayer, setSelectedPlayer] = useState<string>("");
  const [installError, setInstallError] = useState<string>("");
  const [dupOpen, setDupOpen] = useState(false);
  const [dupName, setDupName] = useState<string>("");
  const dupResolveRef = useRef<((overwrite: boolean) => void) | null>(null);
  const isCancelling = useRef(false);
  const cleanupRef = useRef<() => void>(() => {});
  const downloadAttemptRef = useRef(0);

  useEffect(() => () => {
    downloadAttemptRef.current++;
    cleanupRef.current();
  }, []);

  React.useLayoutEffect(() => {
    setIsEntering(true);
    const reset = () => {
      try {
        window.scrollTo({ top: 0, left: 0, behavior: "auto" });
      } catch {}
      try {
        const el = document.scrollingElement as HTMLElement | null;
        if (el) el.scrollTop = 0;
      } catch {}
      try {
        document.documentElement.scrollTop = 0;
      } catch {}
      try {
        document.body.scrollTop = 0;
      } catch {}
      try {
        const root = document.getElementById("root");
        if (root) (root as HTMLElement).scrollTop = 0;
      } catch {}
    };
    reset();
    const raf = requestAnimationFrame(reset);
    const t0 = window.setTimeout(reset, 0);
    const t1 = window.setTimeout(reset, 120);
    const t2 = window.setTimeout(() => setIsEntering(false), 550);
    return () => {
      cancelAnimationFrame(raf);
      clearTimeout(t0);
      clearTimeout(t1);
      clearTimeout(t2);
    };
  }, [id]);

  const handleCancelDownload = async () => {
    isCancelling.current = true;
    downloadAttemptRef.current++;
    cleanupRef.current();
    try {
      await CancelFileDownload();
    } catch (e) {
      console.warn("Cancel failed", e);
    }
    setInstallModalOpen(false);
  };

  const handleInstall = async (file: ModFile) => {
    if (!file.downloadUrl) {
      alert(t("curseforge.no_download_url"));
      return;
    }

    setInstallModalOpen(true);
    setInstallStep("downloading");
    setInstallError("");
    setInstallFile(null);
    setDownloadProgress(null);
    isCancelling.current = false;

    cleanupRef.current();
    const attempt = ++downloadAttemptRef.current;
    const isCurrentAttempt = () => downloadAttemptRef.current === attempt;
    const subscriptions: (() => void)[] = [];
    const cleanup = () => {
      subscriptions.splice(0).forEach((off) => off());
    };
    cleanupRef.current = cleanup;

    try {
      subscriptions.push(Events.On("file.download.progress", (event) => {
        if (!isCurrentAttempt()) return;
        const data = event.data || {};
        setDownloadProgress({
          downloaded: Number(data.Downloaded || 0),
          total: Number(data.Total || 0),
        });
      }));

      subscriptions.push(Events.On("file.download.done", async (event) => {
        if (!isCurrentAttempt()) return;
        cleanup();
        const dest = event.data;
        try {
          let type = "unknown";
          const lowerName = file.fileName.toLowerCase();
          if (lowerName.endsWith(".mcworld")) type = "mcworld";
          else if (lowerName.endsWith(".mcaddon")) type = "mcaddon";
          else if (lowerName.endsWith(".mcpack")) {
            type = (await IsMcpackSkinPackPath(dest)) ? "skin_pack" : "mcpack";
          }

          if (!isCurrentAttempt()) return;
          setInstallFile({ name: file.fileName, path: dest, type });

          const metas = await ListVersionMetasWithRegistered();
          if (!isCurrentAttempt()) return;
          if (metas) {
            metas.sort((a, b) => {
              const cmp = compareVersions(
                a.gameVersion || "0",
                b.gameVersion || "0",
              );
              return -cmp;
            });
            setAvailableVersions(metas);

            const currentName = readCurrentVersionName();
            let defaultSelect = "";
            if (currentName && metas.some((m) => m.name === currentName)) {
              defaultSelect = currentName;
            } else if (metas.length > 0) {
              defaultSelect = metas[0].name;
            }
            setSelectedVersion(defaultSelect);

            const logoMap: Record<string, string> = {};
            await Promise.all(
              metas.map(async (m) => {
                try {
                  const url = await GetVersionLogoDataUrl(m.name);
                  if (url) logoMap[m.name] = url;
                } catch (e) {
                  console.warn("Failed to fetch logo for", m.name, e);
                }
              }),
            );
            if (!isCurrentAttempt()) return;
            setVersionLogos(logoMap);
          }

          setInstallStep("version_select");
        } catch (e: any) {
          if (!isCurrentAttempt()) return;
          setInstallError(e.message || "Detection failed");
          setInstallStep("error");
        }
      }));

      subscriptions.push(Events.On("file.download.error", (event) => {
        if (!isCurrentAttempt()) return;
        cleanup();
        if (isCancelling.current) return;
        const err = event.data;
        setInstallError(err || "Download failed");
        setInstallStep("error");
      }));

      // Register first: small downloads may finish before the binding resolves.
      await StartFileDownload(file.downloadUrl, file.fileName);
    } catch (e: any) {
      cleanup();
      if (!isCurrentAttempt()) return;
      if (isCancelling.current) return;
      setInstallError(e.message || "Download start failed");
      setInstallStep("error");
    }
  };

  const handleVersionSelectNext = async () => {
    if (!installFile) return;

    if (installStep === "player_select") {
      await executeImport();
      return;
    }

    let skipPlayerSelect = false;
    if (installFile.type === "skin_pack") {
      const targetMeta = availableVersions.find(
        (v) => v.name === selectedVersion,
      );
      if (
        targetMeta &&
        compareVersions(targetMeta.gameVersion || "0", "1.26.0") > 0
      ) {
        skipPlayerSelect = true;
      }
    }

    if (
      installFile.type === "mcworld" ||
      (installFile.type === "skin_pack" && !skipPlayerSelect)
    ) {
      setInstallStep("player_select");
      try {
        const roots = await GetContentRoots(selectedVersion);
        if (roots && roots.usersRoot) {
          const players = await listPlayers(roots.usersRoot);
          setAvailablePlayers(players);
          let defaultP = "";
          if (players.length > 0) {
            defaultP = players[0];
            setSelectedPlayer(defaultP);
          }

          (async () => {
            try {
              const map = await getPlayerGamertagMap(roots.usersRoot);
              setPlayerGamertagMap(map);

              const tag = await GetLocalUserGamertag();
              if (tag) {
                for (const p of players) {
                  if (map[p] === tag) {
                    if (p !== defaultP) setSelectedPlayer(p);
                    break;
                  }
                }
              }
            } catch {}
          })();
        } else {
          setPlayerGamertagMap({});
        }
      } catch (e) {
        console.error(e);
        setAvailablePlayers([]);
        setPlayerGamertagMap({});
      }
    } else {
      await executeImport();
    }
  };

  const executeImport = async () => {
    if (!installFile || !selectedVersion) return;

    setInstallStep("importing");
    setInstallError("");

    try {
      const { name, path, type } = installFile;
      const runImport = async (overwrite: boolean): Promise<string> => {
        if (type === "mcworld") {
          if (!selectedPlayer) throw new Error("No player selected");
          return String(
            await ImportMcworldPath(
              selectedVersion,
              selectedPlayer,
              path,
              overwrite,
            ),
          );
        }
        if (type === "mcaddon") {
          if (selectedPlayer) {
            return String(
              await ImportMcaddonPathWithPlayer(
                selectedVersion,
                selectedPlayer,
                path,
                overwrite,
              ),
            );
          }
          return String(
            await ImportMcaddonPath(selectedVersion, path, overwrite),
          );
        }
        if (selectedPlayer) {
          if (type === "skin_pack" && !selectedPlayer) {
            throw new Error("No player selected for skin pack");
          }
          if (type === "skin_pack") {
            return String(
              await ImportMcpackPathWithPlayer(
                selectedVersion,
                selectedPlayer,
                path,
                overwrite,
              ),
            );
          }
          return String(
            await ImportMcpackPath(selectedVersion, path, overwrite),
          );
        }
        return String(await ImportMcpackPath(selectedVersion, path, overwrite));
      };

      let err = await runImport(false);
      if (err) {
        if (
          String(err) === "ERR_DUPLICATE_FOLDER" ||
          String(err) === "ERR_DUPLICATE_UUID"
        ) {
          setDupName(name);
          await new Promise<void>((resolve) => {
            dupResolveRef.current = (overwrite) => {
              resolve();
              if (!overwrite) {
                err = "";
              }
            };
            setDupOpen(true);
          });
          if (!err) {
            setInstallModalOpen(false);
            return;
          }
          err = await runImport(true);
        }
      }

      if (err) {
        throw new Error(err);
      }

      setInstallStep("success");
    } catch (e: any) {
      setInstallError(e.message || "Import failed");
      setInstallStep("error");
    }
  };

  const gameVersions = React.useMemo(() => {
    if (!files || files.length === 0) return [];
    const versions = new Set<string>();
    files.forEach((file) => {
      file.gameVersions?.forEach((v) => {
        versions.add(v);
      });
    });
    return sortGameVersions(Array.from(versions));
  }, [files]);

  const filteredFiles = React.useMemo(() => {
    if (selectedGameVersion === "all") return files;
    return files.filter((file) =>
      file.gameVersions?.includes(selectedGameVersion),
    );
  }, [files, selectedGameVersion]);

  useEffect(() => {
    let cancelled = false;
    const modId = Number(id);
    setMod(null);
    setLoadError(false);
    setLoading(true);
    if (!Number.isSafeInteger(modId) || modId <= 0) {
      setLoading(false);
      return;
    }
    GetCurseForgeModsByIDs([modId])
      .then((result) => {
        if (!cancelled) setMod(result?.data?.[0] ?? null);
      })
      .catch(() => {
        if (!cancelled) setLoadError(true);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [id, detailRetry]);

  useEffect(() => {
    let cancelled = false;
    const modId = Number(id);
    setDescription("");
    setDescriptionError(false);
    setDescriptionLoading(true);
    if (!Number.isSafeInteger(modId) || modId <= 0) {
      setDescriptionLoading(false);
      return;
    }
    GetCurseForgeModDescription(modId)
      .then((result) => {
        if (!cancelled) setDescription(result?.data ?? "");
      })
      .catch(() => {
        if (!cancelled) setDescriptionError(true);
      })
      .finally(() => {
        if (!cancelled) setDescriptionLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [id, descriptionRetry]);

  useEffect(() => {
    let cancelled = false;
    const modId = Number(id);
    setFiles([]);
    setFilesError(false);
    setFilesLoading(true);
    if (!Number.isSafeInteger(modId) || modId <= 0) {
      setFilesLoading(false);
      return;
    }
    GetCurseForgeModFiles(modId)
      .then((result) => {
        if (!cancelled) setFiles(result?.data ?? []);
      })
      .catch(() => {
        if (!cancelled) setFilesError(true);
      })
      .finally(() => {
        if (!cancelled) setFilesLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [id, filesRetry]);

  const handleDescriptionClick = (e: React.MouseEvent<HTMLDivElement>) => {
    const target = (e.target as HTMLElement).closest("a");
    const href = target?.getAttribute("href");

    if (!target || !href) return;

    e.preventDefault();

    let url = href;

    if (url.includes("linkout?remoteUrl=")) {
      const match = url.match(/remoteUrl=(.+)$/);
      if (match?.[1]) {
        try {
          url = decodeURIComponent(match[1]);
        } catch (err) {
          console.warn("Failed to decode remoteUrl", err);
        }
      }
    } else if (url.startsWith("/")) {
      url = "https://www.curseforge.com" + url;
    }

    if (/^https?%3a/i.test(url)) {
      try {
        url = decodeURIComponent(url);
      } catch (e) {
        console.warn("Failed to decode encoded URL", e);
      }
    }

    if (!url || url.startsWith("#")) return;

    if (!/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(url)) {
      url = "https://" + url;
    }

    try {
      const parsedUrl = new URL(url);
      if (["http:", "https:"].includes(parsedUrl.protocol)) {
        Browser.OpenURL(parsedUrl.toString());
      } else {
        console.warn("Blocked opening non-http/https URL:", url);
      }
    } catch (e) {
      console.warn("Invalid URL:", url, e);
    }
  };

  if (loading) {
    return (
      <PageContainer animate={false}>
        <Card className={LAYOUT.GLASS_CARD.BASE}>
          <Card.Content className="p-6">
            <div className="flex flex-col md:flex-row gap-6">
              <div className="flex items-start gap-4 flex-1">
                <Skeleton className="w-24 h-24 rounded-2xl shrink-0" />
                <div className="flex flex-col gap-3 w-full max-w-lg">
                  <Skeleton className="h-8 w-3/4 rounded-lg" />
                  <div className="flex gap-2">
                    <Skeleton className="h-4 w-20 rounded-md" />
                    <Skeleton className="h-4 w-20 rounded-md" />
                  </div>
                  <div className="flex gap-2">
                    <Skeleton className="h-6 w-16 rounded-full" />
                    <Skeleton className="h-6 w-16 rounded-full" />
                  </div>
                </div>
              </div>
              <div className="flex flex-col gap-3 min-w-[200px] justify-center">
                <Skeleton className="h-12 w-full rounded-xl" />
                <div className="flex gap-2 justify-center">
                  <Skeleton className="h-10 w-10 rounded-lg" />
                  <Skeleton className="h-10 w-10 rounded-lg" />
                  <Skeleton className="h-10 w-10 rounded-lg" />
                </div>
              </div>
            </div>
          </Card.Content>
        </Card>

        <Card className={`${LAYOUT.GLASS_CARD.BASE} min-h-[300px]`}>
          <Card.Content className="p-6">
            <div className="flex gap-6 mb-6">
              <Skeleton className="h-8 w-24 rounded-lg" />
              <Skeleton className="h-8 w-24 rounded-lg" />
            </div>
            <div className="space-y-4">
              <Skeleton className="h-4 w-full rounded-md" />
              <Skeleton className="h-4 w-full rounded-md" />
              <Skeleton className="h-4 w-3/4 rounded-md" />
            </div>
          </Card.Content>
        </Card>
      </PageContainer>
    );
  }

  if (!mod) {
    return (
      <div className="w-full h-full min-h-0 flex flex-col p-4 sm:p-6 gap-4 items-center justify-center">
        <Card className="bg-surface/50 dark:bg-surface/40 launcher-material-blur rounded-4xl p-8">
          <Card.Content className="flex flex-col items-center gap-4">
            <p role={loadError ? "alert" : "status"} className="text-xl font-bold">
              {t(
                loadError
                  ? "audit.mods.detail_load_failed"
                  : "curseforge.mod_not_found",
              )}
            </p>
            {loadError && (
              <Button
                variant="primary"
                onPress={() => setDetailRetry((value) => value + 1)}
              >
                {t("common.retry")}
              </Button>
            )}
            <Button
              onPress={() => navigate(-1)}
              variant={"primary"}
              className={
                "bg-brand-500 hover:bg-brand-500 brand-primary-foreground font-bold shadow-lg shadow-brand-900/20"
              }
            >
              {t("curseforge.go_back")}
            </Button>
          </Card.Content>
        </Card>
      </div>
    );
  }

  return (
    <PageContainer
      animate={false}
      className={isEntering ? "overflow-y-hidden" : undefined}
    >
      {/* Header Card */}
      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={{ duration: 0.4 }}
      >
        <Card className={LAYOUT.GLASS_CARD.BASE}>
          <Card.Content className="p-6">
            <div className="flex flex-col md:flex-row gap-6">
              <div className="shrink-0">
                <img
                  src={mod.logo?.url}
                  alt={mod.name}
                  className={
                    "w-32 h-32 object-cover rounded-2xl shadow-lg bg-surface-secondary"
                  }
                />
              </div>

              <div className="flex flex-col grow gap-3">
                <h1 className="text-3xl sm:text-4xl font-black tracking-tight text-foreground dark:text-zinc-100 pb-1">
                  {mod.name}
                </h1>

                <div className="flex items-center gap-3 text-muted dark:text-zinc-400 text-sm flex-wrap">
                  <span className="flex items-center gap-1">
                    {t("curseforge.by")}
                    {mod.authors?.map((author: any, idx: number) => (
                      <React.Fragment key={author.id}>
                        <Link
                          onPress={() => Browser.OpenURL(author.url)}
                          className={cn(
                            "text-sm",
                            "text-accent hover:underline cursor-pointer",
                          )}
                        >
                          {author.name}
                        </Link>
                        {idx < (mod.authors?.length || 0) - 1 && ", "}
                      </React.Fragment>
                    ))}
                  </span>
                  <span className="w-1 h-1 rounded-full bg-surface-quaternary"></span>
                  <span className="flex items-center gap-1">
                    <LuCalendar size={14} />
                    {t("curseforge.updated_date", {
                      date: formatDateStr(mod.dateModified),
                    })}
                  </span>
                  <span className="w-1 h-1 rounded-full bg-surface-quaternary"></span>
                  <span className="flex items-center gap-1">
                    <LuDownload size={14} />
                    {t("curseforge.download_count", {
                      count: mod.downloadCount,
                    })}
                  </span>
                </div>

                <div className="flex flex-wrap gap-2 mt-1">
                  {mod.categories?.map((cat: any) => (
                    <Chip
                      key={cat.id}
                      size="sm"
                      variant="soft"
                      className={"pl-1"}
                    >
                      {cat.iconUrl ? (
                        <img src={cat.iconUrl} alt="" className={"w-4 h-4"} />
                      ) : undefined}
                      <Chip.Label>{cat.name}</Chip.Label>
                    </Chip>
                  ))}
                  <Chip size="sm" variant="secondary">
                    {<LuGamepad2 size={12} />}
                    <Chip.Label>ID: {mod.id}</Chip.Label>
                  </Chip>
                </div>

                <p className="text-foreground dark:text-zinc-300 mt-2 text-sm leading-relaxed max-w-4xl">
                  {mod.summary}
                </p>
              </div>

              {/* Actions */}
              <div className="flex flex-col gap-3 min-w-[240px] md:border-l md:border-border md:pl-8 justify-center">
                <Button
                  size="lg"
                  onPress={() => {
                    setSelectedTab("files");
                    setTimeout(() => {
                      tabsRef.current?.scrollIntoView({
                        behavior: shouldDisableAnimations() ? "auto" : "smooth",
                        block: "start",
                      });
                    }, 100);
                  }}
                  variant={"secondary"}
                  className={
                    "w-full font-semibold shadow-md shadow-brand-900/20 brand-primary-foreground bg-brand-500 hover:bg-brand-500"
                  }
                >
                  {<LuDownload size={20} />}
                  {t("curseforge.install_action")}
                </Button>
                <div className="flex gap-2 justify-center">
                  {mod.links?.websiteUrl && (
                    <Button
                      onPress={() => Browser.OpenURL(mod.links.websiteUrl)}
                      isIconOnly
                      aria-label={t("curseforge.website")}
                      variant={"secondary"}
                    >
                      <LuGlobe size={20} />
                    </Button>
                  )}
                  {mod.links?.sourceUrl && (
                    <Button
                      onPress={() => Browser.OpenURL(mod.links.sourceUrl)}
                      isIconOnly
                      aria-label={t("curseforge.source")}
                      variant={"secondary"}
                    >
                      <LuGithub size={20} />
                    </Button>
                  )}
                  {mod.links?.issuesUrl && (
                    <Button
                      onPress={() => Browser.OpenURL(mod.links.issuesUrl)}
                      isIconOnly
                      aria-label={t("curseforge.issues")}
                      variant={"secondary"}
                    >
                      <LuBug size={20} />
                    </Button>
                  )}
                  <ProjectShareButton url={mod.links?.websiteUrl} />
                </div>
              </div>
            </div>
          </Card.Content>
        </Card>
      </motion.div>

      {/* Content Card */}
      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={{ duration: 0.4, delay: 0.1 }}
      >
        <Card className={`${LAYOUT.GLASS_CARD.BASE} min-h-[500px]`}>
          <Card.Content className="p-6">
            <div ref={tabsRef} className="flex w-full flex-col scroll-mt-24">
              <Tabs
                selectedKey={selectedTab}
                onSelectionChange={(key) => setSelectedTab(key as string)}
                variant="secondary"
              >
                <Tabs.ListContainer>
                  <Tabs.List
                    aria-label={t("curseforge.mod_details_aria_label")}
                    className={
                      "gap-6 w-full relative rounded-none p-0 border-b border-border mb-6"
                    }
                  >
                    <Tabs.Tab
                      key="description"
                      id={"description"}
                      className={
                        "group-data-[selected]:text-brand-600 dark:group-data-[selected]:text-brand-500 font-bold"
                      }
                    >
                      {t("curseforge.mod_tabs.description")}
                      <Tabs.Indicator
                        className={
                          "w-full bg-accent h-[3px]"
                        }
                      />
                    </Tabs.Tab>
                    <Tabs.Tab
                      key="files"
                      id={"files"}
                      className={
                        "group-data-[selected]:text-brand-600 dark:group-data-[selected]:text-brand-500 font-bold"
                      }
                    >
                      {t("curseforge.mod_tabs.files")}
                      <Tabs.Indicator
                        className={
                          "w-full bg-accent h-[3px]"
                        }
                      />
                    </Tabs.Tab>
                  </Tabs.List>
                </Tabs.ListContainer>

                <Tabs.Panel id="description">
                  <div className="prose dark:prose-invert max-w-none prose-img:rounded-xl prose-img:mx-auto prose-a:text-brand-600 dark:prose-a:text-brand-500">
                    {descriptionLoading ? (
                      <div
                        role="status"
                        className="flex flex-col items-center justify-center py-12 text-muted gap-3"
                      >
                        <Spinner color="accent" />
                        <p>{t("curseforge.loading_description")}</p>
                      </div>
                    ) : descriptionError ? (
                      <div
                        role="alert"
                        className="flex flex-col items-center justify-center py-12 gap-3"
                      >
                        <p>{t("audit.mods.description_load_failed")}</p>
                        <Button
                          variant="secondary"
                          onPress={() => setDescriptionRetry((value) => value + 1)}
                        >
                          {t("common.retry")}
                        </Button>
                      </div>
                    ) : description.trim() ? (
                      <div
                        dangerouslySetInnerHTML={{ __html: description }}
                        onClick={handleDescriptionClick}
                      />
                    ) : (
                      <div
                        role="status"
                        className="flex flex-col items-center justify-center py-12 text-muted gap-3"
                      >
                        <p>{t("curseforge.no_description")}</p>
                      </div>
                    )}
                  </div>
                </Tabs.Panel>

                <Tabs.Panel id="files">
                  <div className="flex flex-col gap-4">
                    <div className="flex justify-between items-center mb-2">
                      <h3 className="text-lg font-semibold">
                        {t("curseforge.mod_files.title")}
                      </h3>
                      <div className="w-48">
                        <Select
                          value={Array.from([selectedGameVersion])[0] ?? null}
                          onChange={(e) =>
                            setSelectedGameVersion(String(e ?? "all"))
                          }
                        >
                          <Label>{t("curseforge.minecraft_version")}</Label>
                          <Select.Trigger
                            className={cn(
                              COMPONENT_STYLES.select.trigger,
                              "min-h-8 text-sm",
                            )}
                          >
                            <Select.Value />
                            <Select.Indicator />
                          </Select.Trigger>
                          <Select.Popover
                            className={COMPONENT_STYLES.select.popoverContent}
                          >
                            <ListBox
                              items={[
                                {
                                  key: "all",
                                  label: t("curseforge.all_versions"),
                                },
                                ...gameVersions.map((v) => ({
                                  key: v,
                                  label: v,
                                })),
                              ]}
                              className={COMPONENT_STYLES.select.listbox}
                            >
                              {(item) => (
                                <ListBox.Item
                                  key={item.key}
                                  id={item.key}
                                  textValue={item.label}
                                >
                                  <Label>{item.label}</Label>
                                  <ListBox.ItemIndicator />
                                </ListBox.Item>
                              )}
                            </ListBox>
                          </Select.Popover>
                        </Select>
                      </div>
                    </div>

                    {filesLoading ? (
                      <div
                        role="status"
                        className="flex flex-col items-center py-12 gap-3"
                      >
                        <Spinner color="accent" />
                        <p>{t("common.loading")}</p>
                      </div>
                    ) : filesError ? (
                      <div
                        role="alert"
                        className="flex flex-col items-center py-12 gap-3"
                      >
                        <p>{t("audit.mods.files_load_failed")}</p>
                        <Button
                          variant="secondary"
                          onPress={() => setFilesRetry((value) => value + 1)}
                        >
                          {t("common.retry")}
                        </Button>
                      </div>
                    ) : filteredFiles.length > 0 ? (
                      <Table variant="secondary">
                        <Table.ScrollContainer className="h-full">
                          <Table.Content
                            aria-label={t(
                              "curseforge.mod_files.table_aria_label",
                            )}
                          >
                            <Table.Header
                              className={cn(COMPONENT_STYLES.table.thead)}
                            >
                              <Table.Column
                                className={cn(COMPONENT_STYLES.table.th)}
                                isRowHeader
                              >
                                {t("curseforge.mod_files.columns.type")}
                              </Table.Column>
                              <Table.Column
                                className={cn(COMPONENT_STYLES.table.th)}
                              >
                                {t("curseforge.mod_files.columns.name")}
                              </Table.Column>
                              <Table.Column
                                className={cn(COMPONENT_STYLES.table.th)}
                              >
                                {t("curseforge.mod_files.columns.uploaded")}
                              </Table.Column>
                              <Table.Column
                                className={cn(COMPONENT_STYLES.table.th)}
                              >
                                {t("curseforge.mod_files.columns.size")}
                              </Table.Column>
                              <Table.Column
                                className={cn(COMPONENT_STYLES.table.th)}
                              >
                                {t("curseforge.minecraft_version")}
                              </Table.Column>
                              <Table.Column
                                className={cn(COMPONENT_STYLES.table.th)}
                              >
                                {t("curseforge.mod_files.columns.downloads")}
                              </Table.Column>
                              <Table.Column
                                className={cn(COMPONENT_STYLES.table.th)}
                              >
                                {t("curseforge.mod_files.columns.actions")}
                              </Table.Column>
                            </Table.Header>
                            <Table.Body>
                              {filteredFiles.map((file) => {
                                const sortedVersions = sortGameVersions(
                                  file.gameVersions,
                                );
                                return (
                                  <Table.Row
                                    className={cn(
                                      "group transition-colors hover:bg-surface/50 dark:hover:bg-surface-secondary/30",
                                    )}
                                    key={file.id}
                                    id={file.id}
                                  >
                                    <Table.Cell
                                      className={cn(
                                        "py-3 border-b border-border/50 dark:border-white/5 group-last:border-0",
                                      )}
                                    >
                                      <Chip
                                        size="sm"
                                        variant="soft"
                                        color={
                                          file.releaseType === 1
                                            ? "success"
                                            : file.releaseType === 2
                                              ? "accent"
                                              : "warning"
                                        }
                                        className={"capitalize"}
                                      >
                                        <Chip.Label>
                                          {file.releaseType === 1
                                            ? "R"
                                            : file.releaseType === 2
                                              ? "B"
                                              : "A"}
                                        </Chip.Label>
                                      </Chip>
                                    </Table.Cell>
                                    <Table.Cell
                                      className={cn(
                                        "py-3 border-b border-border/50 dark:border-white/5 group-last:border-0",
                                      )}
                                    >
                                      <span className="font-medium">
                                        {file.displayName}
                                      </span>
                                    </Table.Cell>
                                    <Table.Cell
                                      className={cn(
                                        "py-3 border-b border-border/50 dark:border-white/5 group-last:border-0",
                                      )}
                                    >
                                      <span className="text-muted dark:text-zinc-400">
                                        {formatDateStr(file.fileDate)}
                                      </span>
                                    </Table.Cell>
                                    <Table.Cell
                                      className={cn(
                                        "py-3 border-b border-border/50 dark:border-white/5 group-last:border-0",
                                      )}
                                    >
                                      <span className="text-muted dark:text-zinc-400">
                                        {formatFileSize(file.fileLength)}
                                      </span>
                                    </Table.Cell>
                                    <Table.Cell
                                      className={cn(
                                        "py-3 border-b border-border/50 dark:border-white/5 group-last:border-0",
                                      )}
                                    >
                                      <div className="flex items-center gap-1">
                                        {sortedVersions.length > 0 ? (
                                          <>
                                            <span className="text-foreground dark:text-zinc-300 bg-surface-secondary px-2 py-1 rounded text-xs">
                                              {sortedVersions[0]}
                                            </span>
                                            {sortedVersions.length > 1 && (
                                              <Tooltip>
                                                <span className="text-xs text-accent cursor-pointer">
                                                  +{sortedVersions.length - 1}
                                                </span>
                                                <Tooltip.Content>
                                                  {
                                                    <div className="flex flex-wrap gap-1 max-w-xs p-2">
                                                      {sortedVersions
                                                        .slice(1)
                                                        .map((v) => (
                                                          <span
                                                            key={v}
                                                            className="text-xs bg-surface dark:bg-surface-secondary text-muted dark:text-zinc-400 px-1.5 py-0.5 rounded border border-border dark:border-zinc-700"
                                                          >
                                                            {v}
                                                          </span>
                                                        ))}
                                                    </div>
                                                  }
                                                </Tooltip.Content>
                                              </Tooltip>
                                            )}
                                          </>
                                        ) : (
                                          <span className="text-muted">-</span>
                                        )}
                                      </div>
                                    </Table.Cell>
                                    <Table.Cell
                                      className={cn(
                                        "py-3 border-b border-border/50 dark:border-white/5 group-last:border-0",
                                      )}
                                    >
                                      <span className="text-muted dark:text-zinc-400">
                                        {formatNumber(file.downloadCount)}
                                      </span>
                                    </Table.Cell>
                                    <Table.Cell
                                      className={cn(
                                        "py-3 border-b border-border/50 dark:border-white/5 group-last:border-0",
                                      )}
                                    >
                                      <Tooltip>
                                        <Button
                                          isIconOnly
                                          aria-label={t("audit.mods.install_file", {
                                            name: file.displayName || file.fileName,
                                          })}
                                          size="sm"
                                          onPress={() => handleInstall(file)}
                                          variant={"ghost"}
                                          className={
                                            "text-muted dark:text-zinc-400 hover:text-accent"
                                          }
                                        >
                                          <LuDownload size={20} />
                                        </Button>
                                        <Tooltip.Content>
                                          {t("audit.mods.install_file", {
                                            name: file.displayName || file.fileName,
                                          })}
                                        </Tooltip.Content>
                                      </Tooltip>
                                    </Table.Cell>
                                  </Table.Row>
                                );
                              })}
                            </Table.Body>
                          </Table.Content>
                        </Table.ScrollContainer>
                      </Table>
                    ) : (
                      <div className="flex flex-col items-center justify-center py-12 text-muted border border-dashed border-border rounded-xl">
                        <LuFileDigit size={48} className="mb-4 opacity-50" />
                        <p className="text-lg font-medium">
                          {t("curseforge.no_files_found")}
                        </p>
                      </div>
                    )}
                  </div>
                </Tabs.Panel>
              </Tabs>
            </div>
          </Card.Content>
        </Card>
      </motion.div>

      <UnifiedModal
        size="wide"
        isOpen={installModalOpen}
        onOpenChange={(open) => {
          if (!open) {
            if (installStep === "downloading") {
              handleCancelDownload();
            } else {
              setInstallModalOpen(false);
            }
          }
        }}
        isDismissable={false}
        contentKey={installStep}
        type={installStep === "error" ? "error" : installStep === "success" ? "success" : "primary"}
        icon={
          installStep === "downloading" ? (
            <LuDownload size={24} />
          ) : installStep === "version_select" ? (
            <LuGamepad2 size={24} />
          ) : installStep === "player_select" ? (
            <LuUser size={24} />
          ) : installStep === "importing" ? (
            <LuFileDigit size={24} />
          ) : installStep === "success" ? (
            <FiCheckCircle size={24} />
          ) : undefined
        }
        title={
          <>
            {installStep === "downloading" &&
              t("curseforge.install.downloading_title")}
            {installStep === "version_select" &&
              t("curseforge.install.select_version_title")}
            {installStep === "player_select" &&
              t("curseforge.install.select_player_title")}
            {installStep === "importing" &&
              t("curseforge.install.importing_title")}
            {installStep === "success" && t("curseforge.install.success_title")}
            {installStep === "error" && t("curseforge.install.error_title")}
          </>
        }
        footer={
          <>
            {installStep === "downloading" && (
              <ModalAction onPress={handleCancelDownload} variant="secondary">
                {t("common.cancel")}
              </ModalAction>
            )}
            {(installStep === "version_select" ||
              installStep === "player_select") && (
              <>
                <ModalAction
                  onPress={() => setInstallModalOpen(false)}
                  variant="secondary"
                >
                  {t("common.cancel")}
                </ModalAction>
                <ModalAction
                  onPress={handleVersionSelectNext}
                  variant={"primary"}
                >
                  {t("curseforge.install.next")}
                </ModalAction>
              </>
            )}
            {(installStep === "success" || installStep === "error") && (
              <ModalAction
                onPress={() => setInstallModalOpen(false)}
                variant="primary"
              >
                {t("curseforge.install.close")}
              </ModalAction>
            )}
          </>
        }
      >
        {installStep === "downloading" && (
          <div className="flex flex-col gap-4 w-full">
            <ModalDescription>
              {t("curseforge.install.downloading_body")}
            </ModalDescription>
            {downloadProgress ? (
              <ProgressBar
                aria-label="Downloading..."
                value={
                  (downloadProgress.downloaded / downloadProgress.total) * 100
                }
                color={"accent"}
                className={"w-full"}
              >
                {true && <ProgressBar.Output />}
                <ProgressBar.Track>
                  <ProgressBar.Fill
                    className={"bg-brand-500 dark:bg-brand-500"}
                  />
                </ProgressBar.Track>
              </ProgressBar>
            ) : (
              <Spinner size="lg" color={"accent"} />
            )}
            {downloadProgress && (
              <ModalDescription>
                {formatFileSize(downloadProgress.downloaded)} /{" "}
                {formatFileSize(downloadProgress.total)}
              </ModalDescription>
            )}
          </div>
        )}

        {installStep === "version_select" && (
          <div className="flex flex-col gap-4">
            <ModalDescription>
              {t("curseforge.install.select_version_body")}
            </ModalDescription>
            <Select
              placeholder={t("curseforge.install.select_version_placeholder")}
              value={
                Array.from(selectedVersion ? [selectedVersion] : [])[0] ?? null
              }
              onChange={(e) => setSelectedVersion(String(e ?? ""))}
            >
              <Label>{t("curseforge.install.local_installation")}</Label>
              <Select.Trigger className={COMPONENT_STYLES.select.trigger}>
                <Select.Value />
                <Select.Indicator />
              </Select.Trigger>
              <Select.Popover
                className={COMPONENT_STYLES.select.popoverContent}
              >
                <ListBox
                  items={availableVersions}
                  className={COMPONENT_STYLES.select.listbox}
                >
                  {(ver) => (
                    <ListBox.Item
                      key={ver.name}
                      id={ver.name}
                      textValue={ver.name}
                    >
                      <Label>
                        <div className="flex gap-2 items-center">
                          <div className="w-8 h-8 rounded bg-surface-tertiary flex items-center justify-center overflow-hidden">
                            <img
                              src={
                                versionLogos[ver.name] ||
                                "https://raw.githubusercontent.com/LiteLDev/LeviLauncher/main/build/appicon.png"
                              }
                              alt="icon"
                              className="w-full h-full object-cover"
                              onError={(e) =>
                                (e.currentTarget.style.display = "none")
                              }
                            />
                          </div>
                          <div className="flex flex-col">
                            <span className="text-sm">{ver.name}</span>
                            <span className="text-xs text-muted">
                              {ver.gameVersion}
                            </span>
                          </div>
                          {ver.registered && (
                            <Chip
                              size="sm"
                              variant="soft"
                              color={"success"}
                              className={"ml-auto"}
                            >
                              <Chip.Label>
                                {t("curseforge.install.registered")}
                              </Chip.Label>
                            </Chip>
                          )}
                        </div>
                      </Label>
                      <ListBox.ItemIndicator />
                    </ListBox.Item>
                  )}
                </ListBox>
              </Select.Popover>
            </Select>
          </div>
        )}

        {installStep === "player_select" && (
          <div className="flex flex-col gap-4">
            <ModalDescription>
              {t("curseforge.install.select_player_body")}
            </ModalDescription>
            <Select
              placeholder={t("curseforge.install.select_player_placeholder")}
              value={
                Array.from(selectedPlayer ? [selectedPlayer] : [])[0] ?? null
              }
              onChange={(e) => setSelectedPlayer(String(e ?? ""))}
            >
              <Label>{t("curseforge.install.player_label")}</Label>
              <Select.Trigger className={COMPONENT_STYLES.select.trigger}>
                <Select.Value />
                <Select.Indicator />
              </Select.Trigger>
              <Select.Popover
                className={COMPONENT_STYLES.select.popoverContent}
              >
                <ListBox className={COMPONENT_STYLES.select.listbox}>
                  {availablePlayers.map((player) => (
                    <ListBox.Item
                      key={player}
                      id={player}
                      textValue={resolvePlayerDisplayName(
                        player,
                        playerGamertagMap,
                      )}
                    >
                      <Label>
                        {resolvePlayerDisplayName(player, playerGamertagMap)}
                      </Label>
                      <ListBox.ItemIndicator />
                    </ListBox.Item>
                  ))}
                </ListBox>
              </Select.Popover>
            </Select>
          </div>
        )}

        {installStep === "importing" && (
          <ModalProgress label={t("curseforge.install.importing_title")} description={t("curseforge.install.importing_body")} />
        )}

        {installStep === "success" && (
          <div className="flex flex-col gap-4">
            <div className="space-y-2">
              <ModalDescription>
                {t("curseforge.install.success_msg")}
              </ModalDescription>
              <ModalDescription className="mt-1">
                {t("curseforge.install.success_desc")}
              </ModalDescription>
            </div>
          </div>
        )}

        {installStep === "error" && (
          <div className="flex flex-col gap-4">
            <ModalDescription>
                {t("curseforge.install.failed_msg")}
            </ModalDescription>
              <ModalNotice role="alert" tone="danger" className="font-mono">
                {installError}
              </ModalNotice>
          </div>
        )}
      </UnifiedModal>
      <UnifiedModal
        size="standard"
        isOpen={dupOpen}
        onOpenChange={(open) => {
          if (!open) {
            setDupOpen(false);
          }
        }}
        type="warning"
        title={t("mods.overwrite_modal_title")}
        footer={
          <>
            <ModalAction
              onPress={() => {
                try {
                  if (dupResolveRef.current) dupResolveRef.current(false);
                } finally {
                  setDupOpen(false);
                }
              }}
              variant="secondary"
            >
              {t("common.cancel")}
            </ModalAction>
            <ModalAction
              onPress={() => {
                try {
                  if (dupResolveRef.current) dupResolveRef.current(true);
                } finally {
                  setDupOpen(false);
                }
              }}
              variant={"primary"}
            >
              {t("common.confirm")}
            </ModalAction>
          </>
        }
      >
        <ModalDescription>{t("mods.overwrite_modal_body")}</ModalDescription>
        {dupName ? <ModalPanel className="font-mono">{dupName}</ModalPanel> : null}
      </UnifiedModal>
    </PageContainer>
  );
};

export default CurseForgeModPage;
